package capability

import (
	"fmt"
	"math/bits"
	"sort"
	"strings"
)

type setupStageCandidate struct {
	machineIndex int
	machineRank  int
	stageID      string
	profile      string
	binding      string
	deficits     []Deficit
	operations   []remedyOperation
}

func setupCandidateFor(catalog Catalog, stage Stage, machine MachineInventory, machineIndex int) (setupStageCandidate, bool, error) {
	if !machine.Reachable || machine.ProtocolVersion != ProtocolVersion ||
		machine.CatalogVersion != CatalogVersion ||
		machine.ProfileMappingVersion != ProfileMappingVersion {
		return setupStageCandidate{}, false, nil
	}
	profileDefinition, ok := catalog.profile(stage.Profile)
	if !ok {
		return setupStageCandidate{}, false, nil
	}
	var profile *ProfileOffer
	for i := range machine.Profiles {
		if machine.Profiles[i].ID == stage.Profile {
			profile = &machine.Profiles[i]
			break
		}
	}
	if profile == nil {
		return setupStageCandidate{}, false, nil
	}
	bindingDefinition, ok := catalog.bindingFor(stage.Profile, stage.Requires)
	if !ok {
		return setupStageCandidate{}, false, nil
	}
	var binding *ExecutionBindingOffer
	for i := range machine.Bindings {
		if machine.Bindings[i].ID == bindingDefinition.ID &&
			machine.Bindings[i].Profile == stage.Profile &&
			equalStrings(machine.Bindings[i].Capabilities, stage.Requires) {
			binding = &machine.Bindings[i]
			break
		}
	}
	if binding == nil {
		return setupStageCandidate{}, false, nil
	}

	instances := make([]predicateInstance, 0, 16)
	for _, predicate := range profile.Predicates {
		instances = append(instances, predicateInstance{
			Kind: "profile", ID: profile.ID, MatchTarget: profileDefinition.Adapter, Predicate: predicate,
		})
	}
	for _, requirement := range stage.Requires {
		var logicalOffer *LogicalOffer
		for i := range machine.Offers {
			if machine.Offers[i].ID == requirement {
				logicalOffer = &machine.Offers[i]
				break
			}
		}
		if logicalOffer == nil {
			return setupStageCandidate{}, false, nil
		}
		logicalDefinition, _ := catalog.capability(requirement)
		for _, predicate := range logicalOffer.Predicates {
			instances = append(instances, predicateInstance{
				Kind: "capability", ID: logicalOffer.ID, MatchTarget: logicalDefinition.Adapter, Predicate: predicate,
			})
		}
	}
	for _, predicate := range binding.Predicates {
		instances = append(instances, predicateInstance{
			Kind: "binding", ID: binding.ID, MatchTarget: binding.ID, Predicate: predicate,
		})
	}

	operations, deficits, ok, err := setupClosure(machine.Name, instances)
	if err != nil || !ok || len(operations) == 0 || len(operations) > 8 || len(deficits) > 64 {
		return setupStageCandidate{}, false, err
	}
	return setupStageCandidate{
		machineIndex: machineIndex, machineRank: machine.RegistryRank,
		stageID: stage.ID, profile: stage.Profile, binding: binding.ID,
		deficits: deficits, operations: operations,
	}, true, nil
}

func setupClosure(machine string, instances []predicateInstance) ([]remedyOperation, []Deficit, bool, error) {
	satisfied := make(map[string]bool, len(instances))
	for _, instance := range instances {
		if instance.Predicate.State == PredicateSatisfied {
			satisfied[instance.Predicate.ID] = true
		}
	}
	operations := map[string]*remedyOperation{}
	deficits := map[string]Deficit{}

	for passes := 0; passes < 64; passes++ {
		if len(satisfied) == len(instances) {
			orderedDeficits := make([]Deficit, 0, len(deficits))
			for _, instance := range instances {
				for _, deficit := range deficits {
					if deficit.Kind == instance.Kind && deficit.ID == instance.ID &&
						deficit.PredicateID == instance.Predicate.ID {
						orderedDeficits = append(orderedDeficits, deficit)
						break
					}
				}
			}
			orderedOperations := make([]remedyOperation, 0, len(operations))
			for _, operation := range operations {
				orderedOperations = append(orderedOperations, *operation)
			}
			sort.Slice(orderedOperations, func(i, j int) bool {
				return operationCatalogLess(orderedOperations[i], orderedOperations[j])
			})
			return orderedOperations, orderedDeficits, true, nil
		}
		progress := false
		for i, instance := range instances {
			if satisfied[instance.Predicate.ID] || !dependenciesSatisfied(instance.Predicate.DependsOn, satisfied) {
				continue
			}
			if instance.Predicate.Resolution == ResolutionDerived {
				satisfied[instance.Predicate.ID] = true
				progress = true
				continue
			}
			matched, ok := remedyFor(instance)
			if !ok {
				return nil, nil, false, nil
			}
			opID, err := operationID(machine, matched.EffectID, matched.PostconditionID)
			if err != nil {
				return nil, nil, false, err
			}
			operation := operations[opID]
			if operation == nil {
				operation = &remedyOperation{
					ID: opID, Machine: machine, TemplateID: matched.TemplateID,
					EffectID: matched.EffectID, PostconditionID: matched.PostconditionID,
					EffectSummary: matched.EffectSummary,
				}
				operations[opID] = operation
			}
			// One effect covers every required target predicate in its exact
			// postcondition set on this machine, even if a downstream row is
			// still visibly conditional/blocked.
			for j, covered := range instances {
				if satisfied[covered.Predicate.ID] || !containsString(covered.Predicate.RemedyEffectIDs, matched.EffectID) {
					continue
				}
				coveredRemedy, ok := remedyFor(covered)
				if !ok || coveredRemedy.EffectID != matched.EffectID ||
					coveredRemedy.PostconditionID != matched.PostconditionID {
					return nil, nil, false, nil
				}
				deficit, err := deficitFor(machine, covered, coveredRemedy)
				if err != nil {
					return nil, nil, false, err
				}
				if _, exists := deficits[deficit.DeficitID]; !exists {
					deficits[deficit.DeficitID] = deficit
					operation.Deficits = append(operation.Deficits, deficit)
				}
				satisfied[instances[j].Predicate.ID] = true
			}
			// The initiating row must always be covered; protect against a
			// malformed predicate vector even though normalization checked it.
			if !satisfied[instances[i].Predicate.ID] {
				return nil, nil, false, nil
			}
			progress = true
			break
		}
		if !progress {
			return nil, nil, false, nil
		}
	}
	return nil, nil, false, fmt.Errorf("capability: setup fixed point exceeded its bound")
}

func dependenciesSatisfied(dependencies []string, satisfied map[string]bool) bool {
	for _, dependency := range dependencies {
		if !satisfied[dependency] {
			return false
		}
	}
	return true
}

func operationCatalogLess(a, b remedyOperation) bool {
	if effectRank(a.EffectID) != effectRank(b.EffectID) {
		return effectRank(a.EffectID) < effectRank(b.EffectID)
	}
	if a.PostconditionID != b.PostconditionID {
		return a.PostconditionID < b.PostconditionID
	}
	return a.ID < b.ID
}

func effectRank(effect string) int {
	order := []string{
		codexCapabilityRuntimeEffectID,
		"effect.codex.authenticated-subject.v1",
		"effect.codex.model-ready.",
		"effect.claude-2.1.207.v1",
		"effect.claude.authenticated-subject.v1",
		"effect.hermes-0.15.1.v1",
		"effect.hermes.provider-model.",
		"effect.openclaw-2026.7.1-2.v1",
		"effect.openclaw.main-ready.v1",
		"effect.himalaya-1.2.0-preview.v1",
		"effect.gmail.selected-imap-read.v1",
		"effect.supabase.selected-project-readonly.v1",
	}
	for i, prefix := range order {
		if effect == prefix || strings.HasPrefix(effect, prefix) {
			return i
		}
	}
	return len(order)
}

type setupDPKey struct {
	last        int
	machineBits uint32
	opBits      uint64
}

type setupAssignment struct {
	stages      []setupStageCandidate
	machineBits uint32
	opBits      uint64
	handoffs    int
	nonLocal    int
	ranks       []int
}

func solveSetupAlternatives(plan Plan, machines []MachineInventory, ready [][]stageCandidate, pin string) ([]SetupAlternative, bool, error) {
	stageCandidates := make([][]setupStageCandidate, len(plan.Stages))
	operationUniverse := map[string]remedyOperation{}
	for stageIndex, stage := range plan.Stages {
		for _, candidate := range ready[stageIndex] {
			stageCandidates[stageIndex] = append(stageCandidates[stageIndex], setupStageCandidate{
				machineIndex: candidate.machineIndex, machineRank: machines[candidate.machineIndex].RegistryRank,
				stageID: stage.ID, profile: stage.Profile, binding: candidate.mapping.Binding,
				deficits: []Deficit{}, operations: []remedyOperation{},
			})
		}
		for machineIndex, machine := range machines {
			if pin != "" && machine.Name != pin {
				continue
			}
			candidate, ok, err := setupCandidateFor(CatalogV2(), stage, machine, machineIndex)
			if err != nil {
				return nil, false, err
			}
			if !ok {
				continue
			}
			stageCandidates[stageIndex] = append(stageCandidates[stageIndex], candidate)
			for _, operation := range candidate.operations {
				if existing, ok := operationUniverse[operation.ID]; ok {
					operation = mergeOperation(existing, operation)
				}
				operationUniverse[operation.ID] = operation
			}
		}
		if len(stageCandidates[stageIndex]) == 0 {
			return []SetupAlternative{}, false, nil
		}
		sort.Slice(stageCandidates[stageIndex], func(i, j int) bool {
			a, b := stageCandidates[stageIndex][i], stageCandidates[stageIndex][j]
			if a.machineRank != b.machineRank {
				return a.machineRank < b.machineRank
			}
			return a.binding < b.binding
		})
	}
	if len(operationUniverse) == 0 {
		return []SetupAlternative{}, false, nil
	}
	operations := make([]remedyOperation, 0, len(operationUniverse))
	for _, operation := range operationUniverse {
		operations = append(operations, operation)
	}
	sort.Slice(operations, func(i, j int) bool {
		mi, mj := machineRankByName(machines, operations[i].Machine), machineRankByName(machines, operations[j].Machine)
		if mi != mj {
			return mi < mj
		}
		return operationCatalogLess(operations[i], operations[j])
	})
	if len(operations) > 64 {
		return nil, true, nil
	}
	opIndex := make(map[string]uint, len(operations))
	for i, operation := range operations {
		opIndex[operation.ID] = uint(i)
	}

	states := map[setupDPKey]setupAssignment{}
	transitions := uint64(0)
	for _, candidate := range stageCandidates[0] {
		opBits := operationBits(candidate.operations, opIndex)
		if bits.OnesCount64(opBits) > 8 {
			continue
		}
		machine := machines[candidate.machineIndex]
		key := setupDPKey{last: candidate.machineIndex, machineBits: uint32(1) << candidate.machineIndex, opBits: opBits}
		states[key] = setupAssignment{
			stages: []setupStageCandidate{candidate}, machineBits: key.machineBits,
			opBits: opBits, nonLocal: boolInt(!machine.Local), ranks: []int{machine.RegistryRank},
		}
	}
	for stageIndex := 1; stageIndex < len(stageCandidates); stageIndex++ {
		next := map[setupDPKey]setupAssignment{}
		for key, state := range states {
			for _, candidate := range stageCandidates[stageIndex] {
				transitions++
				if transitions > 300_000_000 {
					return nil, true, nil
				}
				opBits := state.opBits | operationBits(candidate.operations, opIndex)
				if bits.OnesCount64(opBits) > 8 {
					continue
				}
				machine := machines[candidate.machineIndex]
				candidateState := setupAssignment{
					stages:      append(append([]setupStageCandidate(nil), state.stages...), candidate),
					machineBits: state.machineBits | uint32(1)<<candidate.machineIndex,
					opBits:      opBits, handoffs: state.handoffs + boolInt(key.last != candidate.machineIndex),
					nonLocal: state.nonLocal + boolInt(!machine.Local),
					ranks:    append(append([]int(nil), state.ranks...), machine.RegistryRank),
				}
				nextKey := setupDPKey{last: candidate.machineIndex, machineBits: candidateState.machineBits, opBits: opBits}
				if current, exists := next[nextKey]; !exists || setupAssignmentLess(candidateState, current) {
					next[nextKey] = candidateState
				}
			}
		}
		states = next
	}

	assignments := make([]setupAssignment, 0, len(states))
	for _, state := range states {
		if state.opBits != 0 {
			assignments = append(assignments, state)
		}
	}
	sort.Slice(assignments, func(i, j int) bool { return setupAssignmentLess(assignments[i], assignments[j]) })
	alternatives := make([]SetupAlternative, 0, 16)
	semanticSeen := map[string]bool{}
	for _, assignment := range assignments {
		alternative, err := setupAlternativeFor(assignment, machines, operations, opIndex)
		if err != nil {
			return nil, false, err
		}
		semantic, err := controlHash("fort.setup-alternative.v1", struct {
			Deficits []Deficit                  `json:"deficits"`
			Mapping  []HypotheticalMappingEntry `json:"mapping"`
		}{alternative.Deficits, alternative.HypotheticalMapping})
		if err != nil {
			return nil, false, err
		}
		if semanticSeen[semantic] {
			continue
		}
		semanticSeen[semantic] = true
		alternatives = append(alternatives, alternative)
		if len(alternatives) == 16 {
			break
		}
	}
	return alternatives, false, nil
}

func mergeOperation(a, b remedyOperation) remedyOperation {
	seen := map[string]bool{}
	for _, deficit := range a.Deficits {
		seen[deficit.DeficitID] = true
	}
	for _, deficit := range b.Deficits {
		if !seen[deficit.DeficitID] {
			a.Deficits = append(a.Deficits, deficit)
			seen[deficit.DeficitID] = true
		}
	}
	return a
}

func operationBits(operations []remedyOperation, index map[string]uint) uint64 {
	var value uint64
	for _, operation := range operations {
		value |= uint64(1) << index[operation.ID]
	}
	return value
}

func setupAssignmentLess(a, b setupAssignment) bool {
	if bits.OnesCount64(a.opBits) != bits.OnesCount64(b.opBits) {
		return bits.OnesCount64(a.opBits) < bits.OnesCount64(b.opBits)
	}
	if setupMachineCount(a) != setupMachineCount(b) {
		return setupMachineCount(a) < setupMachineCount(b)
	}
	if bits.OnesCount32(a.machineBits) != bits.OnesCount32(b.machineBits) {
		return bits.OnesCount32(a.machineBits) < bits.OnesCount32(b.machineBits)
	}
	if a.handoffs != b.handoffs {
		return a.handoffs < b.handoffs
	}
	if a.nonLocal != b.nonLocal {
		return a.nonLocal < b.nonLocal
	}
	for i := 0; i < len(a.ranks) && i < len(b.ranks); i++ {
		if a.ranks[i] != b.ranks[i] {
			return a.ranks[i] < b.ranks[i]
		}
	}
	for i := 0; i < len(a.stages) && i < len(b.stages); i++ {
		if a.stages[i].profile != b.stages[i].profile {
			return a.stages[i].profile < b.stages[i].profile
		}
		if a.stages[i].binding != b.stages[i].binding {
			return a.stages[i].binding < b.stages[i].binding
		}
	}
	return false
}

func setupMachineCount(assignment setupAssignment) int {
	machines := map[string]bool{}
	for _, stage := range assignment.stages {
		if len(stage.operations) > 0 {
			for _, operation := range stage.operations {
				machines[operation.Machine] = true
			}
		}
	}
	return len(machines)
}

func setupAlternativeFor(assignment setupAssignment, machines []MachineInventory, universe []remedyOperation, opIndex map[string]uint) (SetupAlternative, error) {
	operations := make([]remedyOperation, 0, 8)
	for _, operation := range universe {
		bit := uint64(1) << opIndex[operation.ID]
		if assignment.opBits&bit != 0 {
			operations = append(operations, operation)
		}
	}
	deficits := []Deficit{}
	deficitSeen := map[string]bool{}
	for _, stage := range assignment.stages {
		for _, deficit := range stage.deficits {
			if !deficitSeen[deficit.DeficitID] {
				deficits = append(deficits, deficit)
				deficitSeen[deficit.DeficitID] = true
			}
		}
	}
	if len(deficits) == 0 || len(deficits) > 64 {
		return SetupAlternative{}, fmt.Errorf("capability: invalid setup deficit count")
	}
	// Rebuild selected operations with precisely the option-wide deficit
	// coverage, preserving the operation universe's causal identity.
	for i := range operations {
		operations[i].Deficits = operations[i].Deficits[:0]
		for _, deficit := range deficits {
			if deficit.Machine == operations[i].Machine &&
				deficit.RemedyEffectID == operations[i].EffectID &&
				deficit.PostconditionID == operations[i].PostconditionID {
				operations[i].Deficits = append(operations[i].Deficits, deficit)
			}
		}
	}
	bundle, err := bundleFor(operations)
	if err != nil {
		return SetupAlternative{}, err
	}
	mapping := make([]HypotheticalMappingEntry, len(assignment.stages))
	for i, stage := range assignment.stages {
		stageDeficits := make([]string, 0, len(stage.deficits))
		for _, deficit := range stage.deficits {
			stageDeficits = append(stageDeficits, deficit.DeficitID)
		}
		mapping[i] = HypotheticalMappingEntry{
			StageID: stage.stageID, Machine: machines[stage.machineIndex].Name,
			Profile: stage.profile, Binding: stage.binding, Deficits: stageDeficits,
		}
	}
	return SetupAlternative{
		Mode: "instructions", Deficits: deficits,
		InstructionBundleRef: InstructionBundleRef{ID: bundle.ID, Version: bundle.Version},
		HypotheticalMapping:  mapping, EffectSummary: bundle.EffectSummary,
		InstructionBundle: bundle,
	}, nil
}

func machineRankByName(machines []MachineInventory, name string) int {
	for _, machine := range machines {
		if machine.Name == name {
			return machine.RegistryRank
		}
	}
	return len(machines)
}
