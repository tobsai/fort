package capability

import (
	"reflect"
	"testing"
)

func TestSolveAutomaticallySelectsEligibleLocalSingleHost(t *testing.T) {
	plan, err := DecodeGeneratedPlan([]byte(`{"stages":[{
		"id":"answer","order":1,"title":"Answer","prompt":"Answer safely.",
		"profile":"codex:gpt-5.5","requires":[],"input_from":[],
		"output":"answer","output_format":"text","max_output_bytes":1024
	}]}`), CatalogV1(), nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := twoMachineNoCapabilitySnapshot()

	result, err := Solve(plan, snapshot, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.State != SolveReady || result.Placement != PlacementSingle {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Mapping) != 1 || result.Mapping[0].Machine != "laptop" {
		t.Fatalf("mapping = %#v, want laptop", result.Mapping)
	}
}

func TestSolveDisclosesExactSplitWhenNoSingleHostCanRunPlan(t *testing.T) {
	plan, err := DecodeGeneratedPlan([]byte(validGeneratedPlan), CatalogV1(), nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := splitSnapshot()

	result, err := Solve(plan, snapshot, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.State != SolveChoiceRequired || len(result.ReadyAlternatives) == 0 {
		t.Fatalf("result = %#v", result)
	}
	alt := result.ReadyAlternatives[0]
	if alt.Placement != PlacementSplit || len(alt.Mapping) != 2 {
		t.Fatalf("alternative = %#v", alt)
	}
	if alt.Mapping[0].Machine != "mini" || alt.Mapping[1].Machine != "laptop" {
		t.Fatalf("mapping = %#v", alt.Mapping)
	}
	if len(alt.Handoffs) != 1 || alt.Handoffs[0].Output != "incident_evidence" ||
		alt.Handoffs[0].From != "mini" || alt.Handoffs[0].To != "laptop" {
		t.Fatalf("handoffs = %#v", alt.Handoffs)
	}
}

func TestSolvePinNeverFallsBackOrSplits(t *testing.T) {
	plan, err := DecodeGeneratedPlan([]byte(validGeneratedPlan), CatalogV1(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Solve(plan, splitSnapshot(), "mini")
	if err != nil {
		t.Fatal(err)
	}
	if result.State != SolveBlocked || result.Reason != ReasonNoExecutionPlane {
		t.Fatalf("result = %#v", result)
	}
}

func TestSolveOffersInstructionsOnlySetupToSplit(t *testing.T) {
	plan, err := DecodeGeneratedPlan([]byte(validGeneratedPlan), CatalogV1(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Solve(plan, setupToSplitSnapshot(), "")
	if err != nil {
		t.Fatal(err)
	}
	if result.State != SolveChoiceRequired || len(result.SetupAlternatives) == 0 {
		t.Fatalf("result = %#v", result)
	}
	alt := result.SetupAlternatives[0]
	if alt.Mode != "instructions" || len(alt.InstructionBundle.Instructions) == 0 {
		t.Fatalf("setup alternative = %#v", alt)
	}
	if len(alt.HypotheticalMapping) != 2 ||
		alt.HypotheticalMapping[0].Machine != "mini" ||
		alt.HypotheticalMapping[1].Machine != "laptop" {
		t.Fatalf("hypothetical mapping = %#v", alt.HypotheticalMapping)
	}
	for _, instruction := range alt.InstructionBundle.Instructions {
		if instruction.ID == "" || instruction.OperationID == "" || len(instruction.Covers) == 0 {
			t.Fatalf("incomplete instruction = %#v", instruction)
		}
	}
}

func TestSetupDeduplicatesSharedCodexUpdateEffect(t *testing.T) {
	plan, err := DecodeGeneratedPlan([]byte(`{"stages":[{
		"id":"inspect","order":1,"title":"Inspect","prompt":"Inspect Supabase.",
		"profile":"codex:gpt-5.5","requires":["database.supabase.inspect"],"input_from":[],
		"output":"inspection","output_format":"text","max_output_bytes":1024
	}]}`), CatalogV1(), nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := sharedCodexSetupSnapshot()
	result, err := Solve(plan, snapshot, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SetupAlternatives) == 0 {
		t.Fatalf("result = %#v", result)
	}
	var update *Instruction
	for i := range result.SetupAlternatives[0].InstructionBundle.Instructions {
		instruction := &result.SetupAlternatives[0].InstructionBundle.Instructions[i]
		if instruction.RemedyEffectID == "effect.codex.capability-0.143.0-e0ee3ce1.v1" {
			if update != nil {
				t.Fatal("Codex update effect produced more than one instruction")
			}
			update = instruction
		}
	}
	if update == nil || len(update.Covers) < 3 {
		t.Fatalf("shared update instruction = %#v", update)
	}
}

func TestSetupIDsAndRankingAreDeterministic(t *testing.T) {
	plan, err := DecodeGeneratedPlan([]byte(validGeneratedPlan), CatalogV1(), nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := setupToSplitSnapshot()
	first, err := Solve(plan, snapshot, "")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		got, err := Solve(plan, snapshot, "")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("iteration %d produced a different result", i)
		}
	}
}

func twoMachineNoCapabilitySnapshot() Snapshot {
	base := readySnapshot()
	peer := base.Machines[0]
	peer.Name, peer.Local, peer.RegistryRank = "mini", false, 1
	peer.Profiles[0].BindingRevision = "opaque:profile-mini"
	peer.Bindings[0].BindingRevision = "opaque:binding-mini"
	base.Machines = append(base.Machines, peer)
	return base
}

func splitSnapshot() Snapshot {
	base := twoMachineNoCapabilitySnapshot()
	laptop := &base.Machines[0]
	mini := &base.Machines[1]

	laptop.Offers = []LogicalOffer{readyLogicalSupabase()}
	laptop.Bindings = []ExecutionBindingOffer{readyBinding("codex-appserver+supabase", "database.supabase.inspect", "laptop")}
	mini.Offers = []LogicalOffer{readyLogicalGmail()}
	mini.Bindings = []ExecutionBindingOffer{readyBinding("codex-appserver+gmail", "email.gmail.read", "mini")}
	return base
}

func setupToSplitSnapshot() Snapshot {
	base := splitSnapshot()
	laptop := &base.Machines[0]
	laptop.State = MachinePartial
	laptop.Reason = ReasonAuthRequired
	laptop.Offers = []LogicalOffer{{
		ID: "database.supabase.inspect", Adapter: "database.supabase.inspect.codex-broker",
		State: OfferSetupRequired, Reason: ReasonAuthRequired, BindingRevision: "",
		AvailableThrough: []string{"codex-appserver+supabase"},
		Predicates: []Predicate{
			{ID: "predicate.codex.capability-runtime.v1", Resolution: ResolutionProbe, State: PredicateSatisfied, DependsOn: []string{}, RemedyEffectIDs: []string{"effect.codex.capability-0.143.0-e0ee3ce1.v1"}},
			{ID: "predicate.supabase.selected-project-readonly.v1", Resolution: ResolutionProbe, State: PredicateUnsatisfied, Reason: ReasonAuthRequired, DependsOn: []string{"predicate.codex.capability-runtime.v1"}, RemedyEffectIDs: []string{"effect.supabase.selected-project-readonly.v1"}},
		},
	}}
	laptop.Bindings = []ExecutionBindingOffer{notReadySupabaseBinding(ReasonAuthRequired)}
	return base
}

func sharedCodexSetupSnapshot() Snapshot {
	base := readySnapshot()
	machine := &base.Machines[0]
	machine.State = MachinePartial
	machine.Reason = ReasonIncompatibleVersion
	machine.Profiles[0] = ProfileOffer{
		ID: "codex:gpt-5.5", Agent: "codex", Adapter: "profile.codex.native",
		State: OfferSetupRequired, Reason: ReasonIncompatibleVersion,
		BindingRevision: "",
		Predicates: []Predicate{
			{ID: "predicate.codex.native-contract.v1", Resolution: ResolutionProbe, State: PredicateUnsatisfied, Reason: ReasonIncompatibleVersion, DependsOn: []string{}, RemedyEffectIDs: []string{"effect.codex.capability-0.143.0-e0ee3ce1.v1"}},
			{ID: "predicate.codex.authenticated-subject.v1", Resolution: ResolutionProbe, State: PredicateBlocked, Reason: ReasonAuthRequired, DependsOn: []string{"predicate.codex.native-contract.v1"}, RemedyEffectIDs: []string{"effect.codex.authenticated-subject.v1"}},
			{ID: "predicate.codex.model.codex:gpt-5.5.v1", Resolution: ResolutionProbe, State: PredicateBlocked, Reason: ReasonModelUnavailable, DependsOn: []string{"predicate.codex.authenticated-subject.v1"}, RemedyEffectIDs: []string{"effect.codex.model-ready.codex:gpt-5.5.v1"}},
		},
	}
	machine.Offers = []LogicalOffer{{
		ID: "database.supabase.inspect", Adapter: "database.supabase.inspect.codex-broker",
		State: OfferSetupRequired, Reason: ReasonIncompatibleVersion, BindingRevision: "",
		AvailableThrough: []string{"codex-appserver+supabase"},
		Predicates: []Predicate{
			{ID: "predicate.codex.capability-runtime.v1", Resolution: ResolutionProbe, State: PredicateUnsatisfied, Reason: ReasonIncompatibleVersion, DependsOn: []string{}, RemedyEffectIDs: []string{"effect.codex.capability-0.143.0-e0ee3ce1.v1"}},
			{ID: "predicate.supabase.selected-project-readonly.v1", Resolution: ResolutionProbe, State: PredicateBlocked, Reason: ReasonAuthRequired, DependsOn: []string{"predicate.codex.capability-runtime.v1"}, RemedyEffectIDs: []string{"effect.supabase.selected-project-readonly.v1"}},
		},
	}}
	machine.Bindings = []ExecutionBindingOffer{notReadySupabaseBinding(ReasonIncompatibleVersion)}
	return base
}

func notReadySupabaseBinding(reason Reason) ExecutionBindingOffer {
	return ExecutionBindingOffer{
		ID: "codex-appserver+supabase", Profile: "codex:gpt-5.5",
		Capabilities: []string{"database.supabase.inspect"},
		State:        OfferSetupRequired, Reason: reason, BindingRevision: "",
		Predicates: []Predicate{{
			ID:         "predicate.binding.codex-appserver+supabase.v1",
			Resolution: ResolutionProbe, State: PredicateBlocked,
			Reason: ReasonIncompatibleVersion,
			DependsOn: []string{
				"predicate.codex.native-contract.v1",
				"predicate.codex.authenticated-subject.v1",
				"predicate.codex.model.codex:gpt-5.5.v1",
				"predicate.codex.capability-runtime.v1",
				"predicate.supabase.selected-project-readonly.v1",
			},
			RemedyEffectIDs: []string{"effect.codex.capability-0.143.0-e0ee3ce1.v1"},
		}},
	}
}

func readyLogicalGmail() LogicalOffer {
	return LogicalOffer{
		ID: "email.gmail.read", Adapter: "email.gmail.read.himalaya-broker",
		State: OfferReady, BindingRevision: "opaque:gmail",
		AvailableThrough: []string{"codex-appserver+gmail"},
		Predicates: []Predicate{
			{ID: "predicate.himalaya.preview-contract.v1", Resolution: ResolutionProbe, State: PredicateSatisfied, DependsOn: []string{}, RemedyEffectIDs: []string{"effect.himalaya-1.2.0-preview.v1"}},
			{ID: "predicate.gmail.selected-imap-preview-read.v1", Resolution: ResolutionProbe, State: PredicateSatisfied, DependsOn: []string{"predicate.himalaya.preview-contract.v1"}, RemedyEffectIDs: []string{"effect.gmail.selected-imap-read.v1"}},
		},
	}
}

func readyLogicalSupabase() LogicalOffer {
	return LogicalOffer{
		ID: "database.supabase.inspect", Adapter: "database.supabase.inspect.codex-broker",
		State: OfferReady, BindingRevision: "opaque:supabase",
		AvailableThrough: []string{"codex-appserver+supabase"},
		Predicates: []Predicate{
			{ID: "predicate.codex.capability-runtime.v1", Resolution: ResolutionProbe, State: PredicateSatisfied, DependsOn: []string{}, RemedyEffectIDs: []string{"effect.codex.capability-0.143.0-e0ee3ce1.v1"}},
			{ID: "predicate.supabase.selected-project-readonly.v1", Resolution: ResolutionProbe, State: PredicateSatisfied, DependsOn: []string{"predicate.codex.capability-runtime.v1"}, RemedyEffectIDs: []string{"effect.supabase.selected-project-readonly.v1"}},
		},
	}
}

func readyBinding(id, capabilityID, machine string) ExecutionBindingOffer {
	depends := []string{
		"predicate.codex.native-contract.v1",
		"predicate.codex.authenticated-subject.v1",
		"predicate.codex.model.codex:gpt-5.5.v1",
	}
	switch capabilityID {
	case "email.gmail.read":
		depends = append(depends,
			"predicate.himalaya.preview-contract.v1",
			"predicate.gmail.selected-imap-preview-read.v1")
	case "database.supabase.inspect":
		depends = append(depends,
			"predicate.codex.capability-runtime.v1",
			"predicate.supabase.selected-project-readonly.v1")
	}
	return ExecutionBindingOffer{
		ID: id, Profile: "codex:gpt-5.5", Capabilities: []string{capabilityID},
		State: OfferReady, BindingRevision: "opaque:" + id + ":" + machine,
		Predicates: []Predicate{{
			ID: "predicate.binding." + id + ".v1", Resolution: ResolutionProbe,
			State: PredicateSatisfied, DependsOn: depends,
			RemedyEffectIDs: []string{"effect.codex.capability-0.143.0-e0ee3ce1.v1"},
		}},
	}
}
