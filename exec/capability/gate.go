package capability

import (
	"context"
	"fmt"

	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/runtime"
)

// MachineRefresher is the narrow capability-control seam needed immediately
// before provider dispatch. It refreshes only the machine placement already
// selected by the deterministic core.
type MachineRefresher interface {
	RefreshMachine(context.Context, string, corecap.RefreshMode, []string) (corecap.MachineInventory, error)
}

// ProfilePreflightError is safe to persist and render. It contains only closed
// catalog identifiers, the public machine name, and a closed reason code.
type ProfilePreflightError struct {
	ProfileID string
	Machine   string
	Reason    corecap.Reason
}

func (e *ProfilePreflightError) Error() string {
	if e.ProfileID == "" {
		return "capability preflight blocked: reason=" + string(e.Reason)
	}
	return fmt.Sprintf("capability preflight blocked: profile=%s machine=%s reason=%s", e.ProfileID, e.Machine, e.Reason)
}

// ProfileGate blocks legacy dispatch unless its exact cataloged execution
// profile is ready on the already-selected machine. It never substitutes a
// model or changes placement.
type ProfileGate struct {
	next      runtime.Runtime
	refresher MachineRefresher
	catalog   corecap.Catalog
}

func NewProfileGate(next runtime.Runtime, refresher MachineRefresher) *ProfileGate {
	return &ProfileGate{next: next, refresher: refresher, catalog: corecap.CatalogV2()}
}

func (g *ProfileGate) Name() string { return "profile-gate(" + g.next.Name() + ")" }

func (g *ProfileGate) Dispatch(ctx context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	profileID := spec.Profile
	if profileID != "" {
		agent, model, ok := g.catalog.RuntimeSelection(profileID)
		if !ok || spec.Agent != agent || spec.Model != model {
			return nil, &ProfilePreflightError{ProfileID: profileID, Reason: corecap.ReasonProfileUnmapped}
		}
	} else {
		var ok bool
		profileID, ok = g.catalog.MapLegacyProfile(spec.Agent, spec.Model)
		if !ok {
			return nil, &ProfilePreflightError{Reason: corecap.ReasonProfileUnmapped}
		}
	}
	definition, ok := g.profile(profileID)
	if !ok {
		return nil, &ProfilePreflightError{ProfileID: profileID, Reason: corecap.ReasonProfileUnmapped}
	}
	machine, err := g.refresher.RefreshMachine(ctx, spec.Machine, corecap.RefreshPlanning, []string{definition.Adapter})
	if err != nil {
		return nil, &ProfilePreflightError{ProfileID: profileID, Machine: spec.Machine, Reason: corecap.ReasonProbeFailed}
	}
	target := spec.Machine
	if machine.Name != "" {
		target = machine.Name
	}
	if !machine.Reachable {
		return nil, &ProfilePreflightError{ProfileID: profileID, Machine: target, Reason: corecap.ReasonUnavailable}
	}
	for _, offer := range machine.Profiles {
		if offer.ID != profileID {
			continue
		}
		if offer.State != corecap.OfferReady {
			reason := offer.Reason
			if corecap.FirstReason(reason) == "" {
				reason = corecap.ReasonCommandContractChanged
			}
			return nil, &ProfilePreflightError{ProfileID: profileID, Machine: target, Reason: reason}
		}
		if offer.BindingRevision == "" {
			return nil, &ProfilePreflightError{ProfileID: profileID, Machine: target, Reason: corecap.ReasonCommandContractChanged}
		}
		// Profile is a Fort control-plane identity, not part of the legacy node
		// execution protocol. Lower it to the catalog's exact provider selector
		// only after readiness passed, then keep it out of /api/exec.
		agent, model, ok := g.catalog.RuntimeSelection(profileID)
		if !ok {
			return nil, &ProfilePreflightError{ProfileID: profileID, Machine: target, Reason: corecap.ReasonProfileUnmapped}
		}
		spec.Profile = ""
		spec.Agent = agent
		spec.Model = model
		return g.next.Dispatch(ctx, spec)
	}
	reason := machine.Reason
	if corecap.FirstReason(reason) == "" {
		reason = corecap.ReasonProfileUnmapped
	}
	return nil, &ProfilePreflightError{ProfileID: profileID, Machine: target, Reason: reason}
}

func (g *ProfileGate) profile(id string) (corecap.ProfileDefinition, bool) {
	for _, profile := range g.catalog.Profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return corecap.ProfileDefinition{}, false
}
