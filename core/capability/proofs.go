package capability

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"sort"
	"unicode/utf8"
)

// DirectionDigest validates and hashes the exact original UTF-8 bytes without
// Unicode normalization.
func DirectionDigest(direction string) (string, error) {
	if !utf8.ValidString(direction) || len([]byte(direction)) > MaxDirectionBytes {
		return "", fmt.Errorf("capability: direction must be valid UTF-8 and at most %d bytes", MaxDirectionBytes)
	}
	sum := sha256.Sum256([]byte(direction))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

type PlanSource struct {
	Kind             string `json:"kind"`
	PlaybookID       string `json:"playbook_id"`
	PlaybookRevision int    `json:"playbook_revision"`
	SourceDigest     string `json:"source_digest,omitempty"`
}

type IngressConstraints struct {
	PermittedProfiles      []string `json:"permitted_profiles"`
	PlannerProfileOverride string   `json:"planner_profile_override"`
	PlannerMachinePin      string   `json:"planner_machine_pin"`
	SubstantiveMachinePin  string   `json:"substantive_machine_pin"`
	DeliveryMode           string   `json:"delivery_mode"`
	SignoffRequired        bool     `json:"signoff_required"`
}

type PlanIdentity struct {
	PlanID                string
	RunID                 string
	Source                PlanSource
	DirectionDigest       string
	Constraints           IngressConstraints
	Plan                  Plan
	CatalogVersion        int
	ProfileMappingVersion int
}

// PlanRevision binds immutable plan semantics using length-delimited canonical
// fields. Inventory and placement facts are intentionally not inputs.
func PlanRevision(identity PlanIdentity) (string, error) {
	if identity.PlanID == "" || identity.RunID == "" ||
		identity.CatalogVersion != CatalogVersion ||
		identity.ProfileMappingVersion != ProfileMappingVersion {
		return "", fmt.Errorf("capability: invalid plan identity")
	}
	switch identity.Source.Kind {
	case "generated":
		if identity.Source.SourceDigest != "" {
			return "", fmt.Errorf("capability: generated source forbids source_digest")
		}
	case "static":
		if identity.Source.SourceDigest == "" {
			return "", fmt.Errorf("capability: static source requires source_digest")
		}
	default:
		return "", fmt.Errorf("capability: invalid plan source kind %q", identity.Source.Kind)
	}
	source, err := canonicalJSON(identity.Source)
	if err != nil {
		return "", err
	}
	constraints, err := canonicalJSON(identity.Constraints)
	if err != nil {
		return "", err
	}
	plan, err := canonicalJSON(identity.Plan)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	hash.Write([]byte("fort.capability-plan.v1"))
	hash.Write([]byte{0})
	writeLengthDelimited(hash, []byte(identity.PlanID))
	writeLengthDelimited(hash, []byte(identity.RunID))
	writeLengthDelimited(hash, source)
	writeLengthDelimited(hash, []byte(identity.DirectionDigest))
	writeLengthDelimited(hash, constraints)
	writeLengthDelimited(hash, plan)
	writeLengthDelimited(hash, uint64Bytes(uint64(identity.CatalogVersion)))
	writeLengthDelimited(hash, uint64Bytes(uint64(identity.ProfileMappingVersion)))
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func writeLengthDelimited(destination hash.Hash, value []byte) {
	destination.Write(uint64Bytes(uint64(len(value))))
	destination.Write(value)
}

func uint64Bytes(value uint64) []byte {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	return encoded[:]
}

type BindingScope struct {
	Profile      string   `json:"profile"`
	Capabilities []string `json:"capabilities"`
}

type RelevantScope struct {
	Profiles     []string       `json:"profiles"`
	Capabilities []string       `json:"capabilities"`
	Bindings     []BindingScope `json:"bindings"`
}

type RelevantMachine struct {
	Name                  string                  `json:"name"`
	Local                 bool                    `json:"local"`
	RegistryRank          int                     `json:"registry_rank"`
	Reachable             bool                    `json:"reachable"`
	ProtocolVersion       int                     `json:"protocol_version"`
	CatalogVersion        int                     `json:"catalog_version"`
	ProfileMappingVersion int                     `json:"profile_mapping_version"`
	State                 MachineState            `json:"state"`
	Reason                Reason                  `json:"reason"`
	Profiles              []ProfileOffer          `json:"profiles"`
	Offers                []LogicalOffer          `json:"offers"`
	Bindings              []ExecutionBindingOffer `json:"bindings"`
}

type RelevantProjection struct {
	CatalogVersion        int               `json:"catalog_version"`
	ProfileMappingVersion int               `json:"profile_mapping_version"`
	LocalMachine          string            `json:"local_machine"`
	Scope                 RelevantScope     `json:"scope"`
	Machines              []RelevantMachine `json:"machines"`
}

// RelevantInventory projects only plan-affecting offers and excludes
// observations, URLs, and unrelated capabilities.
func RelevantInventory(plan Plan, snapshot Snapshot) (RelevantProjection, string, error) {
	normalized, err := NormalizeSnapshot(snapshot)
	if err != nil {
		return RelevantProjection{}, "", err
	}
	catalog := CatalogV1()
	profileSet, capabilitySet, bindingSet := map[string]bool{}, map[string]bool{}, map[string]bool{}
	scope := RelevantScope{Profiles: []string{}, Capabilities: []string{}, Bindings: []BindingScope{}}
	for _, stage := range plan.Stages {
		profileSet[stage.Profile] = true
		for _, requirement := range stage.Requires {
			capabilitySet[requirement] = true
		}
		key, err := controlHash("fort.binding-scope.v1", BindingScope{Profile: stage.Profile, Capabilities: stage.Requires})
		if err != nil {
			return RelevantProjection{}, "", err
		}
		if !bindingSet[key] {
			scope.Bindings = append(scope.Bindings, BindingScope{
				Profile: stage.Profile, Capabilities: append([]string(nil), stage.Requires...),
			})
			bindingSet[key] = true
		}
	}
	for _, profile := range catalog.Profiles {
		if profileSet[profile.ID] {
			scope.Profiles = append(scope.Profiles, profile.ID)
		}
	}
	for _, logical := range catalog.Capabilities {
		if capabilitySet[logical.ID] {
			scope.Capabilities = append(scope.Capabilities, logical.ID)
		}
	}

	projection := RelevantProjection{
		CatalogVersion: normalized.CatalogVersion, ProfileMappingVersion: normalized.ProfileMappingVersion,
		LocalMachine: normalized.LocalMachine, Scope: scope,
		Machines: make([]RelevantMachine, len(normalized.Machines)),
	}
	for i, machine := range normalized.Machines {
		row := RelevantMachine{
			Name: machine.Name, Local: machine.Local, RegistryRank: machine.RegistryRank,
			Reachable: machine.Reachable, ProtocolVersion: machine.ProtocolVersion,
			CatalogVersion: machine.CatalogVersion, ProfileMappingVersion: machine.ProfileMappingVersion,
			State: machine.State, Reason: machine.Reason,
			Profiles: []ProfileOffer{}, Offers: []LogicalOffer{}, Bindings: []ExecutionBindingOffer{},
		}
		for _, offer := range machine.Profiles {
			if profileSet[offer.ID] {
				row.Profiles = append(row.Profiles, offer)
			}
		}
		for _, offer := range machine.Offers {
			if capabilitySet[offer.ID] {
				row.Offers = append(row.Offers, offer)
			}
		}
		for _, offer := range machine.Bindings {
			for _, binding := range scope.Bindings {
				if offer.Profile == binding.Profile && equalStrings(offer.Capabilities, binding.Capabilities) {
					row.Bindings = append(row.Bindings, offer)
					break
				}
			}
		}
		projection.Machines[i] = row
	}
	revision, err := controlHash("fort.relevant-inventory.v1", projection)
	return projection, revision, err
}

type CandidateReady struct {
	Machine                    string            `json:"machine"`
	Profile                    string            `json:"profile"`
	Binding                    string            `json:"binding"`
	ProfileBindingRevision     string            `json:"profile_binding_revision"`
	CapabilityBindingRevisions map[string]string `json:"capability_binding_revisions"`
	ExecutionBindingRevision   string            `json:"execution_binding_revision"`
}

type CandidateSetup struct {
	Machine            string   `json:"machine"`
	Profile            string   `json:"profile"`
	Binding            string   `json:"binding"`
	RemedyOperationIDs []string `json:"remedy_operation_ids"`
	TargetDeficitIDs   []string `json:"target_deficit_ids"`
}

type CandidateSet struct {
	StageID string           `json:"stage_id"`
	Ready   []CandidateReady `json:"ready"`
	Setup   []CandidateSetup `json:"setup"`
}

type Option struct {
	ID                       string                     `json:"id"`
	Role                     string                     `json:"role"`
	Label                    string                     `json:"label"`
	Profile                  string                     `json:"profile,omitempty"`
	Machine                  string                     `json:"machine,omitempty"`
	ProfileBindingRevision   string                     `json:"profile_binding_revision,omitempty"`
	ExecutionBindingRevision string                     `json:"execution_binding_revision,omitempty"`
	Placement                Placement                  `json:"placement,omitempty"`
	Mapping                  []MappingEntry             `json:"mapping,omitempty"`
	Handoffs                 []Handoff                  `json:"handoffs,omitempty"`
	Mode                     string                     `json:"mode,omitempty"`
	Deficits                 []Deficit                  `json:"deficits,omitempty"`
	InstructionBundleRef     *InstructionBundleRef      `json:"instruction_bundle_ref,omitempty"`
	HypotheticalMapping      []HypotheticalMappingEntry `json:"hypothetical_mapping,omitempty"`
	EffectSummary            string                     `json:"effect_summary,omitempty"`
}

type semanticOption struct {
	Role                     string                     `json:"role"`
	Label                    string                     `json:"label"`
	Profile                  string                     `json:"profile,omitempty"`
	Machine                  string                     `json:"machine,omitempty"`
	ProfileBindingRevision   string                     `json:"profile_binding_revision,omitempty"`
	ExecutionBindingRevision string                     `json:"execution_binding_revision,omitempty"`
	Placement                Placement                  `json:"placement,omitempty"`
	Mapping                  []MappingEntry             `json:"mapping,omitempty"`
	Handoffs                 []Handoff                  `json:"handoffs,omitempty"`
	Mode                     string                     `json:"mode,omitempty"`
	Deficits                 []Deficit                  `json:"deficits,omitempty"`
	InstructionBundleRef     *InstructionBundleRef      `json:"instruction_bundle_ref,omitempty"`
	HypotheticalMapping      []HypotheticalMappingEntry `json:"hypothetical_mapping,omitempty"`
	EffectSummary            string                     `json:"effect_summary,omitempty"`
}

func (option Option) semantic() semanticOption {
	return semanticOption{
		Role: option.Role, Label: option.Label, Profile: option.Profile, Machine: option.Machine,
		ProfileBindingRevision:   option.ProfileBindingRevision,
		ExecutionBindingRevision: option.ExecutionBindingRevision,
		Placement:                option.Placement, Mapping: option.Mapping, Handoffs: option.Handoffs,
		Mode: option.Mode, Deficits: option.Deficits, InstructionBundleRef: option.InstructionBundleRef,
		HypotheticalMapping: option.HypotheticalMapping, EffectSummary: option.EffectSummary,
	}
}

type PlacementProof struct {
	RelevantProjection RelevantProjection  `json:"relevant_projection"`
	RelevantRevision   string              `json:"relevant_revision"`
	CandidateSets      []CandidateSet      `json:"candidate_sets"`
	ReadyAlternatives  []Alternative       `json:"ready_alternatives"`
	SetupAlternatives  []SetupAlternative  `json:"setup_alternatives"`
	InstructionBundles []InstructionBundle `json:"instruction_bundles"`
	Options            []Option            `json:"options"`
	ChoiceRevision     string              `json:"choice_revision"`
}

// BuildPlacementProof freezes the timestamp-free candidate, alternative,
// instruction, and semantic option projection used by a placement decision.
func BuildPlacementProof(planRevision string, plan Plan, snapshot Snapshot, result SolveResult, pin, decisionID string, decisionVersion int) (PlacementProof, error) {
	normalized, err := NormalizeSnapshot(snapshot)
	if err != nil {
		return PlacementProof{}, err
	}
	relevant, relevantRevision, err := RelevantInventory(plan, normalized)
	if err != nil {
		return PlacementProof{}, err
	}
	candidates, err := candidateProjection(plan, normalized, pin)
	if err != nil {
		return PlacementProof{}, err
	}
	readyAlternatives := append([]Alternative(nil), result.ReadyAlternatives...)
	if result.State == SolveReady {
		readyAlternatives = []Alternative{{
			Placement: result.Placement, Mapping: result.Mapping, Handoffs: result.Handoffs,
		}}
	}
	setupAlternatives := append([]SetupAlternative(nil), result.SetupAlternatives...)
	bundles := make([]InstructionBundle, len(setupAlternatives))
	options := make([]Option, 0, len(readyAlternatives)+len(setupAlternatives)+1)
	for _, alternative := range readyAlternatives {
		options = append(options, Option{
			Role: "run_mapping", Label: mappingLabel(alternative),
			Placement: alternative.Placement, Mapping: alternative.Mapping, Handoffs: alternative.Handoffs,
		})
	}
	for i, alternative := range setupAlternatives {
		bundles[i] = alternative.InstructionBundle
		ref := alternative.InstructionBundleRef
		options = append(options, Option{
			Role: "setup", Label: "Show setup instructions", Mode: "instructions",
			Deficits: alternative.Deficits, InstructionBundleRef: &ref,
			HypotheticalMapping: alternative.HypotheticalMapping, EffectSummary: alternative.EffectSummary,
		})
	}
	options = append(options, Option{Role: "cancel", Label: "Cancel"})
	semantic := make([]semanticOption, len(options))
	for i := range options {
		semantic[i] = options[i].semantic()
	}
	setupProjection := make([]struct {
		Mode                 string                     `json:"mode"`
		Deficits             []Deficit                  `json:"deficits"`
		InstructionBundleRef InstructionBundleRef       `json:"instruction_bundle_ref"`
		HypotheticalMapping  []HypotheticalMappingEntry `json:"hypothetical_mapping"`
		EffectSummary        string                     `json:"effect_summary"`
	}, len(setupAlternatives))
	for i, alternative := range setupAlternatives {
		setupProjection[i] = struct {
			Mode                 string                     `json:"mode"`
			Deficits             []Deficit                  `json:"deficits"`
			InstructionBundleRef InstructionBundleRef       `json:"instruction_bundle_ref"`
			HypotheticalMapping  []HypotheticalMappingEntry `json:"hypothetical_mapping"`
			EffectSummary        string                     `json:"effect_summary"`
		}{
			Mode: alternative.Mode, Deficits: alternative.Deficits,
			InstructionBundleRef: alternative.InstructionBundleRef,
			HypotheticalMapping:  alternative.HypotheticalMapping,
			EffectSummary:        alternative.EffectSummary,
		}
	}
	choiceInput := struct {
		PlanRevision       string              `json:"plan_revision"`
		RelevantRevision   string              `json:"relevant_revision"`
		CandidateSets      []CandidateSet      `json:"candidate_sets"`
		ReadyAlternatives  []Alternative       `json:"ready_alternatives"`
		SetupAlternatives  any                 `json:"setup_alternatives"`
		InstructionBundles []InstructionBundle `json:"instruction_bundles"`
		SemanticOptions    []semanticOption    `json:"semantic_options"`
	}{
		PlanRevision: planRevision, RelevantRevision: relevantRevision,
		CandidateSets: candidates, ReadyAlternatives: readyAlternatives,
		SetupAlternatives: setupProjection, InstructionBundles: bundles,
		SemanticOptions: semantic,
	}
	choiceRevision, err := controlHash("fort.choice-revision.v1", choiceInput)
	if err != nil {
		return PlacementProof{}, err
	}
	for i := range options {
		var id string
		if options[i].Role == "run_mapping" || options[i].Role == "setup" {
			id, err = shortContentID("fort.placement-option.v1", "option_", struct {
				PlanRevision   string         `json:"plan_revision"`
				SemanticOption semanticOption `json:"semantic_option"`
			}{planRevision, options[i].semantic()})
		} else {
			id, err = shortContentID("fort.control-option.v1", "option_", struct {
				DecisionID      string `json:"decision_id"`
				DecisionVersion int    `json:"decision_version"`
				Role            string `json:"role"`
			}{decisionID, decisionVersion, options[i].Role})
		}
		if err != nil {
			return PlacementProof{}, err
		}
		options[i].ID = id
	}
	return PlacementProof{
		RelevantProjection: relevant, RelevantRevision: relevantRevision,
		CandidateSets: candidates, ReadyAlternatives: readyAlternatives,
		SetupAlternatives: setupAlternatives, InstructionBundles: bundles,
		Options: options, ChoiceRevision: choiceRevision,
	}, nil
}

func candidateProjection(plan Plan, snapshot Snapshot, pin string) ([]CandidateSet, error) {
	catalog := CatalogV1()
	out := make([]CandidateSet, len(plan.Stages))
	for stageIndex, stage := range plan.Stages {
		row := CandidateSet{StageID: stage.ID, Ready: []CandidateReady{}, Setup: []CandidateSetup{}}
		for machineIndex, machine := range snapshot.Machines {
			if pin != "" && machine.Name != pin {
				continue
			}
			if candidate, ok := candidateFor(catalog, stage, machine, machineIndex); ok {
				row.Ready = append(row.Ready, CandidateReady{
					Machine: candidate.mapping.Machine, Profile: candidate.mapping.Profile,
					Binding:                    candidate.mapping.Binding,
					ProfileBindingRevision:     candidate.mapping.ProfileBindingRevision,
					CapabilityBindingRevisions: candidate.mapping.CapabilityBindingRevisions,
					ExecutionBindingRevision:   candidate.mapping.ExecutionBindingRevision,
				})
			}
			if candidate, ok, err := setupCandidateFor(catalog, stage, machine, machineIndex); err != nil {
				return nil, err
			} else if ok {
				operationIDs := make([]string, len(candidate.operations))
				for i, operation := range candidate.operations {
					operationIDs[i] = operation.ID
				}
				deficitIDs := make([]string, len(candidate.deficits))
				for i, deficit := range candidate.deficits {
					deficitIDs[i] = deficit.DeficitID
				}
				row.Setup = append(row.Setup, CandidateSetup{
					Machine: machine.Name, Profile: stage.Profile, Binding: candidate.binding,
					RemedyOperationIDs: operationIDs, TargetDeficitIDs: deficitIDs,
				})
			}
		}
		sort.Slice(row.Ready, func(i, j int) bool {
			ri, rj := machineRankByName(snapshot.Machines, row.Ready[i].Machine), machineRankByName(snapshot.Machines, row.Ready[j].Machine)
			if ri != rj {
				return ri < rj
			}
			if catalog.bindingRank(row.Ready[i].Binding) != catalog.bindingRank(row.Ready[j].Binding) {
				return catalog.bindingRank(row.Ready[i].Binding) < catalog.bindingRank(row.Ready[j].Binding)
			}
			return catalog.profileRank(row.Ready[i].Profile) < catalog.profileRank(row.Ready[j].Profile)
		})
		sort.Slice(row.Setup, func(i, j int) bool {
			ri, rj := machineRankByName(snapshot.Machines, row.Setup[i].Machine), machineRankByName(snapshot.Machines, row.Setup[j].Machine)
			if ri != rj {
				return ri < rj
			}
			return row.Setup[i].Binding < row.Setup[j].Binding
		})
		out[stageIndex] = row
	}
	return out, nil
}

func mappingLabel(alternative Alternative) string {
	if alternative.Placement == PlacementSingle && len(alternative.Mapping) > 0 {
		return "Run on " + alternative.Mapping[0].Machine
	}
	return "Run with the disclosed split"
}

type InputContract struct {
	RunID           string `json:"run_id"`
	PlanID          string `json:"plan_id"`
	PlanRevision    string `json:"plan_revision"`
	ChoiceRevision  string `json:"choice_revision"`
	SourceStageID   string `json:"source_stage_id"`
	ConsumerStageID string `json:"consumer_stage_id"`
	SourceMachine   string `json:"source_machine"`
	TargetMachine   string `json:"target_machine"`
	Output          string `json:"output"`
	Format          string `json:"format"`
	MaxBytes        int    `json:"max_bytes"`
}

func InputContractRevision(contract InputContract) (string, error) {
	if contract.RunID == "" || contract.PlanID == "" || contract.SourceStageID == "" ||
		contract.ConsumerStageID == "" || contract.SourceMachine == "" || contract.TargetMachine == "" ||
		(contract.Format != "text" && contract.Format != "json") ||
		contract.MaxBytes < 1 || contract.MaxBytes > MaxStageOutputBytes {
		return "", fmt.Errorf("capability: invalid input contract")
	}
	return controlHash("fort.input-contract.v1", contract)
}

func fullContentID(domain, prefix string, value any) (string, error) {
	sum, err := rawControlHash(domain, value)
	if err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(sum[:]), nil
}
