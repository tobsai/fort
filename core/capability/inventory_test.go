package capability

import (
	"testing"
	"time"
)

func TestCodexPredicatePinsCurrentCapabilityRuntimeEffect(t *testing.T) {
	profile, ok := CatalogV2().profile("codex:gpt-5.5")
	if !ok {
		t.Fatal("missing Codex profile")
	}
	predicates := profilePredicateShapes(profile)
	want := "effect.codex.capability-0.149.0-alpha.4.1-780383c8.v4"
	if len(predicates) == 0 || len(predicates[0].RemedyEffectIDs) != 1 || predicates[0].RemedyEffectIDs[0] != want {
		t.Fatalf("Codex runtime effect = %+v, want %q", predicates, want)
	}
}

func TestSnapshotRevisionExcludesObservationTimes(t *testing.T) {
	a := readySnapshot()
	b := readySnapshot()
	a.ObservedAt = time.Unix(10, 0).UTC()
	b.ObservedAt = time.Unix(20, 0).UTC()
	a.Machines[0].ObservedAt = a.ObservedAt
	b.Machines[0].ObservedAt = b.ObservedAt

	ar, err := SnapshotRevision(a)
	if err != nil {
		t.Fatal(err)
	}
	br, err := SnapshotRevision(b)
	if err != nil {
		t.Fatal(err)
	}
	if ar != br {
		t.Fatalf("timestamp changed revision: %s != %s", ar, br)
	}

	b.Machines[0].Profiles[0].BindingRevision = "opaque:changed"
	br, err = SnapshotRevision(b)
	if err != nil {
		t.Fatal(err)
	}
	if ar == br {
		t.Fatal("binding revision change did not change snapshot revision")
	}
}

func TestSnapshotRevisionIncludesResolvedModel(t *testing.T) {
	a := readyDynamicSnapshot("gpt-5.6-sol")
	b := readyDynamicSnapshot("gpt-5.6-terra")

	ar, err := SnapshotRevision(a)
	if err != nil {
		t.Fatal(err)
	}
	br, err := SnapshotRevision(b)
	if err != nil {
		t.Fatal(err)
	}
	if ar == br {
		t.Fatal("resolved model change did not change snapshot revision")
	}
}

func TestNormalizeSnapshotRequiresResolvedModelForReadyDynamicProfile(t *testing.T) {
	snapshot := readyDynamicSnapshot("")
	if _, err := NormalizeSnapshot(snapshot); err == nil {
		t.Fatal("ready configured-default profile without resolved_model was accepted")
	}
}

func TestNormalizeSnapshotRejectsResolvedModelOnExplicitProfile(t *testing.T) {
	snapshot := readySnapshot()
	snapshot.Machines[0].Profiles[0].ResolvedModel = "gpt-5.5"
	if _, err := NormalizeSnapshot(snapshot); err == nil {
		t.Fatal("explicit-model profile accepted a dynamic resolved_model")
	}
}

func TestNormalizeSnapshotRejectsReadyOfferWithoutProof(t *testing.T) {
	s := readySnapshot()
	s.Machines[0].Profiles[0].BindingRevision = ""
	if _, err := NormalizeSnapshot(s); err == nil {
		t.Fatal("expected ready profile without revision to fail")
	}
}

func TestNormalizeSnapshotRejectsIncompleteCatalogPredicateVector(t *testing.T) {
	s := readySnapshot()
	s.Machines[0].Profiles[0].Predicates = s.Machines[0].Profiles[0].Predicates[:2]
	if _, err := NormalizeSnapshot(s); err == nil {
		t.Fatal("expected incomplete profile predicate vector to fail")
	}
}

func TestNormalizeSnapshotRejectsOutOfOrderCatalogPredicateVector(t *testing.T) {
	s := readySnapshot()
	predicates := s.Machines[0].Profiles[0].Predicates
	predicates[0], predicates[1] = predicates[1], predicates[0]
	if _, err := NormalizeSnapshot(s); err == nil {
		t.Fatal("expected out-of-order profile predicate vector to fail")
	}
}

func readySnapshot() Snapshot {
	profilePredicates := []Predicate{
		{ID: "predicate.codex.native-contract.v1", Resolution: ResolutionProbe, State: PredicateSatisfied, DependsOn: []string{}, RemedyEffectIDs: []string{codexCapabilityRuntimeEffectID}},
		{ID: "predicate.codex.authenticated-subject.v1", Resolution: ResolutionProbe, State: PredicateSatisfied, DependsOn: []string{"predicate.codex.native-contract.v1"}, RemedyEffectIDs: []string{"effect.codex.authenticated-subject.v1"}},
		{ID: "predicate.codex.model.codex:gpt-5.5.v1", Resolution: ResolutionProbe, State: PredicateSatisfied, DependsOn: []string{"predicate.codex.authenticated-subject.v1"}, RemedyEffectIDs: []string{"effect.codex.model-ready.codex:gpt-5.5.v1"}},
	}
	return Snapshot{
		CatalogVersion:        CatalogVersion,
		ProfileMappingVersion: ProfileMappingVersion,
		LocalMachine:          "laptop",
		Machines: []MachineInventory{{
			Name: "laptop", Local: true, RegistryRank: 0, Reachable: true,
			ProtocolVersion: ProtocolVersion, CatalogVersion: CatalogVersion, ProfileMappingVersion: ProfileMappingVersion,
			State: MachineReady, Reason: "",
			Profiles: []ProfileOffer{{
				ID: "codex:gpt-5.5", Agent: "codex", Adapter: "profile.codex.native",
				State: OfferReady, BindingRevision: "opaque:profile-laptop",
				Predicates: profilePredicates,
			}},
			Offers:          []LogicalOffer{},
			TextOnlyOptions: []TextOnlyOptionOffer{},
			Bindings: []ExecutionBindingOffer{{
				ID: "codex-native", Profile: "codex:gpt-5.5", Capabilities: []string{},
				State: OfferReady, BindingRevision: "opaque:binding-laptop",
				Predicates: []Predicate{{
					ID: "predicate.binding.codex-native.v1", Resolution: ResolutionDerived,
					State: PredicateSatisfied, DependsOn: []string{
						"predicate.codex.native-contract.v1",
						"predicate.codex.authenticated-subject.v1",
						"predicate.codex.model.codex:gpt-5.5.v1",
					}, RemedyEffectIDs: []string{},
				}},
			}},
		}},
	}
}

func readyDynamicSnapshot(model string) Snapshot {
	snapshot := readySnapshot()
	profile := &snapshot.Machines[0].Profiles[0]
	profile.ID = "codex:configured-default"
	profile.ResolvedModel = model
	profile.Predicates[2].ID = "predicate.codex.model.codex:configured-default.v1"
	profile.Predicates[2].RemedyEffectIDs = []string{"effect.codex.model-ready.codex:configured-default.v1"}
	binding := &snapshot.Machines[0].Bindings[0]
	binding.Profile = profile.ID
	binding.Predicates[0].DependsOn[2] = profile.Predicates[2].ID
	return snapshot
}
