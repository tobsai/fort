package capability

import (
	"fmt"
	"math/bits"
	"sort"
)

type SolveState string

const (
	SolveReady          SolveState = "ready"
	SolveChoiceRequired SolveState = "choice_required"
	SolveBlocked        SolveState = "blocked"
)

type Placement string

const (
	PlacementSingle Placement = "single"
	PlacementSplit  Placement = "split"
)

type MappingEntry struct {
	StageID                    string            `json:"stage_id"`
	Machine                    string            `json:"machine"`
	Profile                    string            `json:"profile"`
	Binding                    string            `json:"binding"`
	ProfileBindingRevision     string            `json:"profile_binding_revision"`
	CapabilityBindingRevisions map[string]string `json:"capability_binding_revisions"`
	ExecutionBindingRevision   string            `json:"execution_binding_revision"`
}

type Handoff struct {
	Output   string `json:"output"`
	From     string `json:"from"`
	To       string `json:"to"`
	Format   string `json:"format"`
	MaxBytes int    `json:"max_bytes"`
}

type Alternative struct {
	Placement Placement      `json:"placement"`
	Mapping   []MappingEntry `json:"mapping"`
	Handoffs  []Handoff      `json:"handoffs"`
}

type SolveResult struct {
	State             SolveState         `json:"state"`
	Reason            Reason             `json:"reason,omitempty"`
	Placement         Placement          `json:"placement,omitempty"`
	Mapping           []MappingEntry     `json:"mapping,omitempty"`
	Handoffs          []Handoff          `json:"handoffs,omitempty"`
	ReadyAlternatives []Alternative      `json:"ready_alternatives"`
	SetupAlternatives []SetupAlternative `json:"setup_alternatives"`
}

type stageCandidate struct {
	machineIndex int
	mapping      MappingEntry
}

// Solve performs model-free placement from one frozen public snapshot.
func Solve(plan Plan, snapshot Snapshot, pin string) (SolveResult, error) {
	normalized, err := NormalizeSnapshot(snapshot)
	if err != nil {
		return SolveResult{}, err
	}
	if len(plan.Stages) < 1 || len(plan.Stages) > 16 {
		return SolveResult{}, fmt.Errorf("capability: solver received an invalid plan")
	}
	catalog := CatalogV1()
	candidates := make([][]stageCandidate, len(plan.Stages))
	for stageIndex, stage := range plan.Stages {
		for machineIndex, machine := range normalized.Machines {
			if pin != "" && machine.Name != pin {
				continue
			}
			if candidate, ok := candidateFor(catalog, stage, machine, machineIndex); ok {
				candidates[stageIndex] = append(candidates[stageIndex], candidate)
			}
		}
	}

	// A single ready host is the only automatic placement.
	allReady := true
	for _, stageCandidates := range candidates {
		if len(stageCandidates) == 0 {
			allReady = false
			break
		}
	}
	if allReady {
		for _, machine := range singleHostOrder(normalized.Machines) {
			mapping, ok := mappingOnMachine(candidates, machine)
			if !ok {
				continue
			}
			return SolveResult{
				State: SolveReady, Placement: PlacementSingle, Mapping: mapping,
				Handoffs: []Handoff{}, ReadyAlternatives: []Alternative{},
				SetupAlternatives: []SetupAlternative{},
			}, nil
		}
	}

	readyAlternatives := []Alternative{}
	if allReady && pin == "" {
		readyAlternatives = splitAlternatives(plan, normalized.Machines, candidates)
	}
	setupAlternatives, limitExceeded, err := solveSetupAlternatives(plan, normalized.Machines, candidates, pin)
	if err != nil {
		return SolveResult{}, err
	}
	if limitExceeded {
		return SolveResult{
			State: SolveBlocked, Reason: ReasonSolverLimitExceeded,
			ReadyAlternatives: []Alternative{}, SetupAlternatives: []SetupAlternative{},
		}, nil
	}
	if len(readyAlternatives) == 0 && len(setupAlternatives) == 0 {
		return SolveResult{
			State: SolveBlocked, Reason: ReasonNoExecutionPlane,
			ReadyAlternatives: []Alternative{}, SetupAlternatives: []SetupAlternative{},
		}, nil
	}
	if len(readyAlternatives) > 16 {
		readyAlternatives = readyAlternatives[:16]
	}
	return SolveResult{
		State: SolveChoiceRequired, ReadyAlternatives: readyAlternatives,
		SetupAlternatives: setupAlternatives,
	}, nil
}

func candidateFor(catalog Catalog, stage Stage, machine MachineInventory, machineIndex int) (stageCandidate, bool) {
	if !machine.Reachable || machine.ProtocolVersion != ProtocolVersion ||
		machine.CatalogVersion != CatalogVersion ||
		machine.ProfileMappingVersion != ProfileMappingVersion {
		return stageCandidate{}, false
	}
	var profile *ProfileOffer
	for i := range machine.Profiles {
		if machine.Profiles[i].ID == stage.Profile && machine.Profiles[i].State == OfferReady {
			profile = &machine.Profiles[i]
			break
		}
	}
	if profile == nil {
		return stageCandidate{}, false
	}
	var binding *ExecutionBindingOffer
	for i := range machine.Bindings {
		offer := &machine.Bindings[i]
		if offer.Profile == stage.Profile && offer.State == OfferReady &&
			equalStrings(offer.Capabilities, stage.Requires) {
			definition, ok := catalog.bindingFor(stage.Profile, stage.Requires)
			if ok && definition.ID == offer.ID {
				binding = offer
				break
			}
		}
	}
	if binding == nil {
		return stageCandidate{}, false
	}
	logicalRevisions := make(map[string]string, len(stage.Requires))
	for _, requirement := range stage.Requires {
		found := false
		for _, offer := range machine.Offers {
			if offer.ID == requirement && offer.State == OfferReady &&
				containsString(offer.AvailableThrough, binding.ID) {
				logicalRevisions[requirement] = offer.BindingRevision
				found = true
				break
			}
		}
		if !found {
			return stageCandidate{}, false
		}
	}
	return stageCandidate{
		machineIndex: machineIndex,
		mapping: MappingEntry{
			StageID: stage.ID, Machine: machine.Name, Profile: stage.Profile,
			Binding: binding.ID, ProfileBindingRevision: profile.BindingRevision,
			CapabilityBindingRevisions: logicalRevisions,
			ExecutionBindingRevision:   binding.BindingRevision,
		},
	}, true
}

func singleHostOrder(machines []MachineInventory) []int {
	order := make([]int, len(machines))
	for i := range machines {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := machines[order[i]], machines[order[j]]
		if a.Local != b.Local {
			return a.Local
		}
		if a.RegistryRank != b.RegistryRank {
			return a.RegistryRank < b.RegistryRank
		}
		return a.Name < b.Name
	})
	return order
}

func mappingOnMachine(candidates [][]stageCandidate, machineIndex int) ([]MappingEntry, bool) {
	mapping := make([]MappingEntry, len(candidates))
	for stageIndex, stageCandidates := range candidates {
		found := false
		for _, candidate := range stageCandidates {
			if candidate.machineIndex == machineIndex {
				mapping[stageIndex] = candidate.mapping
				found = true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	return mapping, true
}

type partialAssignment struct {
	mapping     []MappingEntry
	machineBits uint32
	handoffs    int
	nonLocal    int
	ranks       []int
	reserved    int
}

func splitAlternatives(plan Plan, machines []MachineInventory, candidates [][]stageCandidate) []Alternative {
	var alternatives []Alternative
	for cardinality := 2; cardinality <= len(machines); cardinality++ {
		for subset := uint32(1); subset < uint32(1)<<len(machines); subset++ {
			if bits.OnesCount32(subset) != cardinality {
				continue
			}
			assignment, ok := bestInSubset(plan, machines, candidates, subset)
			if !ok || bits.OnesCount32(assignment.machineBits) != cardinality {
				continue
			}
			alternatives = append(alternatives, Alternative{
				Placement: PlacementSplit, Mapping: assignment.mapping,
				Handoffs: handoffsFor(plan, assignment.mapping),
			})
		}
		if len(alternatives) > 0 {
			break
		}
	}
	sort.Slice(alternatives, func(i, j int) bool {
		return alternativeLess(alternatives[i], alternatives[j], machines)
	})
	return alternatives
}

func bestInSubset(plan Plan, machines []MachineInventory, candidates [][]stageCandidate, subset uint32) (partialAssignment, bool) {
	states := map[int]partialAssignment{}
	for _, candidate := range candidates[0] {
		if subset&(uint32(1)<<candidate.machineIndex) == 0 {
			continue
		}
		machine := machines[candidate.machineIndex]
		states[candidate.machineIndex] = partialAssignment{
			mapping: []MappingEntry{candidate.mapping}, machineBits: uint32(1) << candidate.machineIndex,
			nonLocal: boolInt(!machine.Local), ranks: []int{machine.RegistryRank},
		}
	}
	for stageIndex := 1; stageIndex < len(plan.Stages); stageIndex++ {
		next := map[int]partialAssignment{}
		for lastMachine, state := range states {
			for _, candidate := range candidates[stageIndex] {
				if subset&(uint32(1)<<candidate.machineIndex) == 0 {
					continue
				}
				machine := machines[candidate.machineIndex]
				candidateState := partialAssignment{
					mapping:     append(append([]MappingEntry(nil), state.mapping...), candidate.mapping),
					machineBits: state.machineBits | uint32(1)<<candidate.machineIndex,
					handoffs:    state.handoffs, nonLocal: state.nonLocal + boolInt(!machine.Local),
					ranks:    append(append([]int(nil), state.ranks...), machine.RegistryRank),
					reserved: state.reserved,
				}
				if lastMachine != candidate.machineIndex {
					candidateState.handoffs++
					candidateState.reserved += plan.Stages[stageIndex-1].MaxOutputBytes
				}
				if candidateState.reserved > MaxHandoffReservedBytes {
					continue
				}
				current, exists := next[candidate.machineIndex]
				if !exists || assignmentLess(candidateState, current) {
					next[candidate.machineIndex] = candidateState
				}
			}
		}
		states = next
	}
	var best partialAssignment
	found := false
	for _, state := range states {
		if !found || assignmentLess(state, best) {
			best, found = state, true
		}
	}
	return best, found
}

func assignmentLess(a, b partialAssignment) bool {
	if bits.OnesCount32(a.machineBits) != bits.OnesCount32(b.machineBits) {
		return bits.OnesCount32(a.machineBits) < bits.OnesCount32(b.machineBits)
	}
	if a.handoffs != b.handoffs {
		return a.handoffs < b.handoffs
	}
	if a.nonLocal != b.nonLocal {
		return a.nonLocal < b.nonLocal
	}
	for i := range a.ranks {
		if a.ranks[i] != b.ranks[i] {
			return a.ranks[i] < b.ranks[i]
		}
	}
	return false
}

func alternativeLess(a, b Alternative, machines []MachineInventory) bool {
	ma, mb := metricsFor(a, machines), metricsFor(b, machines)
	return assignmentLess(ma, mb)
}

func metricsFor(alternative Alternative, machines []MachineInventory) partialAssignment {
	index := make(map[string]int, len(machines))
	for i, machine := range machines {
		index[machine.Name] = i
	}
	var metrics partialAssignment
	last := -1
	for _, mapping := range alternative.Mapping {
		i := index[mapping.Machine]
		metrics.machineBits |= uint32(1) << i
		metrics.ranks = append(metrics.ranks, machines[i].RegistryRank)
		metrics.nonLocal += boolInt(!machines[i].Local)
		if last >= 0 && last != i {
			metrics.handoffs++
		}
		last = i
	}
	return metrics
}

func handoffsFor(plan Plan, mapping []MappingEntry) []Handoff {
	handoffs := []Handoff{}
	for i := 1; i < len(mapping); i++ {
		if mapping[i-1].Machine == mapping[i].Machine {
			continue
		}
		previous := plan.Stages[i-1]
		handoffs = append(handoffs, Handoff{
			Output: previous.Output, From: mapping[i-1].Machine, To: mapping[i].Machine,
			Format: previous.OutputFormat, MaxBytes: previous.MaxOutputBytes,
		})
	}
	return handoffs
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
