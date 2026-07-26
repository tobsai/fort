package capability

import (
	"strings"
	"testing"
)

func TestDirectionDigestPreservesExactUTF8AndEnforcesBound(t *testing.T) {
	a, err := DirectionDigest("e\u0301")
	if err != nil {
		t.Fatal(err)
	}
	b, err := DirectionDigest("é")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("direction digest normalized distinct UTF-8 inputs")
	}
	if _, err := DirectionDigest(strings.Repeat("x", MaxDirectionBytes+1)); err == nil {
		t.Fatal("expected oversized direction to fail")
	}
}

func TestPlanRevisionBindsSemanticsButNotInventory(t *testing.T) {
	plan, err := DecodeGeneratedPlan([]byte(validGeneratedPlan), CatalogV1(), nil)
	if err != nil {
		t.Fatal(err)
	}
	directionDigest, err := DirectionDigest("diagnose the incident")
	if err != nil {
		t.Fatal(err)
	}
	identity := PlanIdentity{
		PlanID: "plan-1", RunID: "run-1",
		Source:          PlanSource{Kind: "generated", PlaybookID: "default", PlaybookRevision: 7},
		DirectionDigest: directionDigest,
		Constraints: IngressConstraints{
			PermittedProfiles: []string{"codex:gpt-5.5"},
			DeliveryMode:      "assignment", SignoffRequired: true,
		},
		Plan: plan, CatalogVersion: 1, ProfileMappingVersion: 1,
	}
	first, err := PlanRevision(identity)
	if err != nil {
		t.Fatal(err)
	}
	identity.Plan.Stages[0].Prompt += " changed"
	second, err := PlanRevision(identity)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("changed prompt retained plan revision")
	}
}

func TestRelevantRevisionIgnoresTimestampsAndUnrelatedOffers(t *testing.T) {
	plan, err := DecodeGeneratedPlan([]byte(`{"stages":[{
		"id":"answer","order":1,"title":"Answer","prompt":"Answer.",
		"profile":"codex:gpt-5.5","requires":[],"input_from":[],
		"output":"answer","output_format":"text","max_output_bytes":1024
	}]}`), CatalogV1(), nil)
	if err != nil {
		t.Fatal(err)
	}
	a := readySnapshot()
	b := readySnapshot()
	b.ObservedAt = b.ObservedAt.AddDate(1, 0, 0)
	b.Machines[0].Profiles = append(b.Machines[0].Profiles, readyClaudeProfile())
	b.Machines[0].Bindings = append(b.Machines[0].Bindings, readyClaudeBinding())

	_, ar, err := RelevantInventory(plan, a)
	if err != nil {
		t.Fatal(err)
	}
	_, br, err := RelevantInventory(plan, b)
	if err != nil {
		t.Fatal(err)
	}
	if ar != br {
		t.Fatalf("unrelated inventory changed relevant revision: %s != %s", ar, br)
	}
}

func TestPlacementProofAndOptionIDsAreStable(t *testing.T) {
	plan, err := DecodeGeneratedPlan([]byte(validGeneratedPlan), CatalogV1(), nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := splitSnapshot()
	result, err := Solve(plan, snapshot, "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := BuildPlacementProof("plan-revision", plan, snapshot, result, "", "capdec_abcdefghijklmnopqrstuv", 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildPlacementProof("plan-revision", plan, snapshot, result, "", "capdec_abcdefghijklmnopqrstuv", 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.ChoiceRevision != second.ChoiceRevision || len(first.Options) == 0 {
		t.Fatalf("proofs differ: %#v %#v", first, second)
	}
	for i := range first.Options {
		if first.Options[i].ID == "" || first.Options[i].ID != second.Options[i].ID {
			t.Fatalf("option %d IDs are not stable", i)
		}
	}
}

func TestInputContractRevisionChangesWithConsumer(t *testing.T) {
	contract := InputContract{
		RunID: "run-1", PlanID: "plan-1", PlanRevision: "sha256:plan",
		ChoiceRevision: "sha256:choice", SourceStageID: "read",
		ConsumerStageID: "diagnose", SourceMachine: "mini", TargetMachine: "laptop",
		Output: "incident_evidence", Format: "text", MaxBytes: 1024,
	}
	first, err := InputContractRevision(contract)
	if err != nil {
		t.Fatal(err)
	}
	contract.ConsumerStageID = "other"
	second, err := InputContractRevision(contract)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("consumer change retained input contract revision")
	}
}

func readyClaudeProfile() ProfileOffer {
	return ProfileOffer{
		ID: "claude:configured-default", Agent: "claude", Adapter: "profile.claude.native",
		State: OfferReady, BindingRevision: "opaque:claude-profile",
		Predicates: []Predicate{
			{ID: "predicate.claude.native-contract.v1", Resolution: ResolutionProbe, State: PredicateSatisfied, DependsOn: []string{}, RemedyEffectIDs: []string{"effect.claude-2.1.207.v1"}},
			{ID: "predicate.claude.authenticated-subject.v1", Resolution: ResolutionProbe, State: PredicateSatisfied, DependsOn: []string{"predicate.claude.native-contract.v1"}, RemedyEffectIDs: []string{"effect.claude.authenticated-subject.v1"}},
		},
	}
}

func readyClaudeBinding() ExecutionBindingOffer {
	return ExecutionBindingOffer{
		ID: "claude-native", Profile: "claude:configured-default", Capabilities: []string{},
		State: OfferReady, BindingRevision: "opaque:claude-binding",
		Predicates: []Predicate{{
			ID: "predicate.binding.claude-native.v1", Resolution: ResolutionDerived,
			State:           PredicateSatisfied,
			DependsOn:       []string{"predicate.claude.native-contract.v1", "predicate.claude.authenticated-subject.v1"},
			RemedyEffectIDs: []string{},
		}},
	}
}
