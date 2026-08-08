package capability

import (
	"fmt"
	"sort"
	"strings"
)

// NormalizeSnapshot validates and orders a complete public snapshot. It never
// fills missing proof data or converts static machine claims into readiness.
func NormalizeSnapshot(snapshot Snapshot) (Snapshot, error) {
	if snapshot.CatalogVersion != CatalogVersion ||
		snapshot.ProfileMappingVersion != ProfileMappingVersion {
		return Snapshot{}, fmt.Errorf("capability: unsupported snapshot version %d/%d", snapshot.CatalogVersion, snapshot.ProfileMappingVersion)
	}
	if snapshot.LocalMachine == "" {
		return Snapshot{}, fmt.Errorf("capability: local_machine is required")
	}
	if len(snapshot.Machines) == 0 || len(snapshot.Machines) > 16 {
		return Snapshot{}, fmt.Errorf("capability: machines must contain 1 to 16 rows")
	}

	catalog := CatalogV2()
	out := snapshot
	out.Machines = append([]MachineInventory(nil), snapshot.Machines...)
	names := map[string]bool{}
	ranks := map[int]bool{}
	localCount := 0
	for i := range out.Machines {
		machine := &out.Machines[i]
		if machine.Name == "" || names[machine.Name] {
			return Snapshot{}, fmt.Errorf("capability: machine names must be non-empty and unique")
		}
		names[machine.Name] = true
		if machine.RegistryRank < 0 || ranks[machine.RegistryRank] {
			return Snapshot{}, fmt.Errorf("capability: registry ranks must be non-negative and unique")
		}
		ranks[machine.RegistryRank] = true
		if machine.Local {
			localCount++
			if machine.Name != snapshot.LocalMachine || machine.RegistryRank != 0 {
				return Snapshot{}, fmt.Errorf("capability: local machine identity/rank mismatch")
			}
		}
		if len(machine.Profiles) > 64 || len(machine.Offers) > 64 || len(machine.Bindings) > 128 {
			return Snapshot{}, fmt.Errorf("capability: machine offer bounds exceeded")
		}
		if machine.Profiles == nil || machine.Offers == nil || machine.Bindings == nil {
			return Snapshot{}, fmt.Errorf("capability: machine %q offer arrays must be non-null", machine.Name)
		}
		if err := normalizeMachine(machine, catalog); err != nil {
			return Snapshot{}, fmt.Errorf("capability: machine %q: %w", machine.Name, err)
		}
	}
	if localCount != 1 {
		return Snapshot{}, fmt.Errorf("capability: exactly one local machine is required")
	}
	sort.Slice(out.Machines, func(i, j int) bool {
		if out.Machines[i].RegistryRank != out.Machines[j].RegistryRank {
			return out.Machines[i].RegistryRank < out.Machines[j].RegistryRank
		}
		return out.Machines[i].Name < out.Machines[j].Name
	})
	return out, nil
}

func normalizeMachine(machine *MachineInventory, catalog Catalog) error {
	switch machine.State {
	case MachineReady, MachinePartial, MachineUnknown:
	default:
		return fmt.Errorf("unknown machine state %q", machine.State)
	}
	if machine.Reason != "" && FirstReason(machine.Reason) == "" {
		return fmt.Errorf("unknown machine reason %q", machine.Reason)
	}

	profiles := append([]ProfileOffer{}, machine.Profiles...)
	profileKeys := map[string]bool{}
	for _, offer := range profiles {
		definition, ok := catalog.profile(offer.ID)
		if !ok || offer.Agent != definition.Agent || offer.Adapter != definition.Adapter {
			return fmt.Errorf("profile offer %q is not cataloged", offer.ID)
		}
		if profileKeys[offer.ID] {
			return fmt.Errorf("duplicate profile offer %q", offer.ID)
		}
		profileKeys[offer.ID] = true
		if err := validateOffer(offer.State, offer.Reason, offer.BindingRevision, offer.Predicates); err != nil {
			return fmt.Errorf("profile %q: %w", offer.ID, err)
		}
		if definition.RequiresResolvedModel() {
			if offer.ResolvedModel != strings.TrimSpace(offer.ResolvedModel) {
				return fmt.Errorf("profile %q has an invalid resolved_model", offer.ID)
			}
			if offer.State == OfferReady && offer.ResolvedModel == "" {
				return fmt.Errorf("profile %q requires resolved_model when ready", offer.ID)
			}
		} else if offer.ResolvedModel != "" {
			return fmt.Errorf("profile %q cannot carry resolved_model", offer.ID)
		}
		if err := validatePredicateShapes(offer.Predicates, profilePredicateShapes(definition)); err != nil {
			return fmt.Errorf("profile %q: %w", offer.ID, err)
		}
	}
	sort.Slice(profiles, func(i, j int) bool {
		return catalog.profileRank(profiles[i].ID) < catalog.profileRank(profiles[j].ID)
	})
	machine.Profiles = profiles

	offers := append([]LogicalOffer{}, machine.Offers...)
	offerKeys := map[string]bool{}
	for i := range offers {
		offer := &offers[i]
		definition, ok := catalog.capability(offer.ID)
		if !ok || offer.Adapter != definition.Adapter {
			return fmt.Errorf("logical offer %q is not cataloged", offer.ID)
		}
		if offerKeys[offer.ID] {
			return fmt.Errorf("duplicate logical offer %q", offer.ID)
		}
		offerKeys[offer.ID] = true
		if !uniqueStrings(offer.AvailableThrough) {
			return fmt.Errorf("logical offer %q repeats available_through", offer.ID)
		}
		for _, bindingID := range offer.AvailableThrough {
			if catalog.bindingRank(bindingID) == len(catalog.Bindings) {
				return fmt.Errorf("logical offer %q names unknown binding %q", offer.ID, bindingID)
			}
		}
		sort.Slice(offer.AvailableThrough, func(i, j int) bool {
			return catalog.bindingRank(offer.AvailableThrough[i]) < catalog.bindingRank(offer.AvailableThrough[j])
		})
		if err := validateOffer(offer.State, offer.Reason, offer.BindingRevision, offer.Predicates); err != nil {
			return fmt.Errorf("logical offer %q: %w", offer.ID, err)
		}
		if err := validatePredicateShapes(offer.Predicates, logicalPredicateShapes(definition)); err != nil {
			return fmt.Errorf("logical offer %q: %w", offer.ID, err)
		}
	}
	sort.Slice(offers, func(i, j int) bool {
		return catalog.capabilityRank(offers[i].ID) < catalog.capabilityRank(offers[j].ID)
	})
	machine.Offers = offers

	bindings := append([]ExecutionBindingOffer{}, machine.Bindings...)
	bindingKeys := map[string]bool{}
	for i := range bindings {
		binding := &bindings[i]
		if _, ok := catalog.profile(binding.Profile); !ok {
			return fmt.Errorf("binding %q names unknown profile %q", binding.ID, binding.Profile)
		}
		definition, ok := catalog.bindingFor(binding.Profile, binding.Capabilities)
		if !ok || definition.ID != binding.ID {
			return fmt.Errorf("binding offer %q is not cataloged for %q", binding.ID, binding.Profile)
		}
		key := binding.ID + "\x00" + binding.Profile
		if bindingKeys[key] {
			return fmt.Errorf("duplicate binding offer %q/%q", binding.ID, binding.Profile)
		}
		bindingKeys[key] = true
		if err := validateOffer(binding.State, binding.Reason, binding.BindingRevision, binding.Predicates); err != nil {
			return fmt.Errorf("binding %q: %w", binding.ID, err)
		}
		profile, _ := catalog.profile(binding.Profile)
		if err := validatePredicateShapes(binding.Predicates, bindingPredicateShapes(catalog, definition, profile)); err != nil {
			return fmt.Errorf("binding %q: %w", binding.ID, err)
		}
	}
	sort.Slice(bindings, func(i, j int) bool {
		if catalog.bindingRank(bindings[i].ID) != catalog.bindingRank(bindings[j].ID) {
			return catalog.bindingRank(bindings[i].ID) < catalog.bindingRank(bindings[j].ID)
		}
		return catalog.profileRank(bindings[i].Profile) < catalog.profileRank(bindings[j].Profile)
	})
	machine.Bindings = bindings
	return nil
}

func validateOffer(state OfferState, reason Reason, revision string, predicates []Predicate) error {
	switch state {
	case OfferReady:
		if reason != "" || !validOpaqueRevision(revision) {
			return fmt.Errorf("ready offer requires empty reason and opaque revision")
		}
	case OfferSetupRequired, OfferUnavailable, OfferUnknown:
		if revision != "" || FirstReason(reason) == "" {
			return fmt.Errorf("non-ready offer requires closed reason and empty revision")
		}
	default:
		return fmt.Errorf("unknown offer state %q", state)
	}
	if predicates == nil || len(predicates) == 0 || len(predicates) > 4 {
		return fmt.Errorf("predicate vector must contain 1 to 4 rows")
	}
	ids := map[string]bool{}
	for _, predicate := range predicates {
		if predicate.ID == "" || ids[predicate.ID] {
			return fmt.Errorf("predicate ids must be non-empty and unique")
		}
		ids[predicate.ID] = true
		if predicate.DependsOn == nil || predicate.RemedyEffectIDs == nil ||
			len(predicate.DependsOn) > 32 || len(predicate.RemedyEffectIDs) > 2 ||
			!uniqueStrings(predicate.DependsOn) || !uniqueStrings(predicate.RemedyEffectIDs) {
			return fmt.Errorf("invalid predicate dependency/effect vector")
		}
		switch predicate.Resolution {
		case ResolutionProbe, ResolutionDerived:
		default:
			return fmt.Errorf("unknown predicate resolution %q", predicate.Resolution)
		}
		switch predicate.State {
		case PredicateSatisfied:
			if predicate.Reason != "" {
				return fmt.Errorf("satisfied predicate has a reason")
			}
		case PredicateUnsatisfied:
			if FirstReason(predicate.Reason) == "" {
				return fmt.Errorf("unsatisfied predicate lacks a closed reason")
			}
		case PredicateBlocked:
			if predicate.Resolution == ResolutionProbe && FirstReason(predicate.Reason) == "" {
				return fmt.Errorf("blocked probe predicate lacks a closed reason")
			}
			if predicate.Resolution == ResolutionDerived && predicate.Reason != "" {
				return fmt.Errorf("blocked derived predicate has a reason")
			}
		default:
			return fmt.Errorf("unknown predicate state %q", predicate.State)
		}
	}
	return nil
}

// SnapshotRevision hashes the timestamp-free, secret-free normalized solver
// projection.
func SnapshotRevision(snapshot Snapshot) (string, error) {
	normalized, err := NormalizeSnapshot(snapshot)
	if err != nil {
		return "", err
	}
	projection := struct {
		CatalogVersion        int                `json:"catalog_version"`
		ProfileMappingVersion int                `json:"profile_mapping_version"`
		LocalMachine          string             `json:"local_machine"`
		Machines              []MachineInventory `json:"machines"`
	}{
		CatalogVersion: normalized.CatalogVersion, ProfileMappingVersion: normalized.ProfileMappingVersion,
		LocalMachine: normalized.LocalMachine, Machines: normalized.Machines,
	}
	for i := range projection.Machines {
		projection.Machines[i].ObservedAt = projection.Machines[i].ObservedAt.UTC()
	}
	// Use a dedicated wire projection to omit timestamps rather than depending
	// on zero-time JSON encoding.
	type machineProjection struct {
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
	machines := make([]machineProjection, len(normalized.Machines))
	for i, machine := range normalized.Machines {
		machines[i] = machineProjection{
			Name: machine.Name, Local: machine.Local, RegistryRank: machine.RegistryRank,
			Reachable: machine.Reachable, ProtocolVersion: machine.ProtocolVersion,
			CatalogVersion: machine.CatalogVersion, ProfileMappingVersion: machine.ProfileMappingVersion,
			State: machine.State, Reason: machine.Reason, Profiles: machine.Profiles,
			Offers: machine.Offers, Bindings: machine.Bindings,
		}
	}
	return controlHash("fort.capability-inventory.v1", struct {
		CatalogVersion        int                 `json:"catalog_version"`
		ProfileMappingVersion int                 `json:"profile_mapping_version"`
		LocalMachine          string              `json:"local_machine"`
		Machines              []machineProjection `json:"machines"`
	}{
		CatalogVersion: normalized.CatalogVersion, ProfileMappingVersion: normalized.ProfileMappingVersion,
		LocalMachine: normalized.LocalMachine, Machines: machines,
	})
}
