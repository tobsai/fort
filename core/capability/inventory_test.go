package capability

import (
	"testing"
	"time"
)

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
		{ID: "predicate.codex.native-contract.v1", Resolution: ResolutionProbe, State: PredicateSatisfied, DependsOn: []string{}, RemedyEffectIDs: []string{"effect.codex.capability-0.146.0-alpha.3.1-3db500cc.v2"}},
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
			Offers: []LogicalOffer{},
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
