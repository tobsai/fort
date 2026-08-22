package capability

import (
	"fmt"
	"strings"
)

const codexCapabilityRuntimeEffectID = "effect.codex.capability-0.149.0-alpha.4.1-780383c8.v4"
const codexSubscriptionEffectID = "effect.codex-subscription-0.149.0-alpha.4.1.v2"

// PredicateTemplate is the immutable catalog portion of a predicate row. A
// node probe supplies only state/reason; it cannot alter dependencies or
// remedy semantics.
type PredicateTemplate struct {
	ID              string              `json:"id"`
	Resolution      PredicateResolution `json:"resolution"`
	DependsOn       []string            `json:"depends_on"`
	RemedyEffectIDs []string            `json:"remedy_effect_ids"`
}

func profilePredicateShapes(profile ProfileDefinition) []PredicateTemplate {
	switch profile.Agent {
	case "codex-subscription":
		return []PredicateTemplate{{
			ID: "predicate.codex-subscription.closed-contract.v1", Resolution: ResolutionProbe,
			DependsOn: []string{}, RemedyEffectIDs: []string{codexSubscriptionEffectID},
		}}
	case "codex":
		native := "predicate.codex.native-contract.v1"
		auth := "predicate.codex.authenticated-subject.v1"
		return []PredicateTemplate{
			{ID: native, Resolution: ResolutionProbe, DependsOn: []string{}, RemedyEffectIDs: []string{codexCapabilityRuntimeEffectID}},
			{ID: auth, Resolution: ResolutionProbe, DependsOn: []string{native}, RemedyEffectIDs: []string{"effect.codex.authenticated-subject.v1"}},
			{
				ID: "predicate.codex.model." + profile.ID + ".v1", Resolution: ResolutionProbe,
				DependsOn: []string{auth}, RemedyEffectIDs: []string{"effect.codex.model-ready." + profile.ID + ".v1"},
			},
		}
	case "claude":
		native := "predicate.claude.native-contract.v1"
		return []PredicateTemplate{
			{ID: native, Resolution: ResolutionProbe, DependsOn: []string{}, RemedyEffectIDs: []string{"effect.claude-2.1.207.v1"}},
			{
				ID: "predicate.claude.authenticated-subject.v1", Resolution: ResolutionProbe,
				DependsOn: []string{native}, RemedyEffectIDs: []string{"effect.claude.authenticated-subject.v1"},
			},
		}
	case "hermes":
		native := "predicate.hermes.native-contract.v1"
		return []PredicateTemplate{
			{ID: native, Resolution: ResolutionProbe, DependsOn: []string{}, RemedyEffectIDs: []string{"effect.hermes-0.15.1.v1"}},
			{
				ID: "predicate.hermes.provider-model." + profile.ID + ".v1", Resolution: ResolutionProbe,
				DependsOn: []string{native}, RemedyEffectIDs: []string{"effect.hermes.provider-model." + profile.ID + ".v1"},
			},
		}
	case "openclaw":
		native := "predicate.openclaw.native-contract.v1"
		return []PredicateTemplate{
			{ID: native, Resolution: ResolutionProbe, DependsOn: []string{}, RemedyEffectIDs: []string{"effect.openclaw-2026.7.1-2.v1"}},
			{
				ID: "predicate.openclaw.main-ready.v1", Resolution: ResolutionProbe,
				DependsOn: []string{native}, RemedyEffectIDs: []string{"effect.openclaw.main-ready.v1"},
			},
		}
	default:
		return nil
	}
}

func logicalPredicateShapes(logical CapabilityDefinition) []PredicateTemplate {
	switch logical.ID {
	case "model.chat.text-only":
		return []PredicateTemplate{{
			ID: "predicate.codex-subscription.text-only-adapter.v1", Resolution: ResolutionDerived,
			DependsOn: []string{}, RemedyEffectIDs: []string{},
		}}
	case "email.gmail.read":
		preview := "predicate.himalaya.preview-contract.v1"
		return []PredicateTemplate{
			{ID: preview, Resolution: ResolutionProbe, DependsOn: []string{}, RemedyEffectIDs: []string{"effect.himalaya-1.2.0-preview.v1"}},
			{
				ID: "predicate.gmail.selected-imap-preview-read.v1", Resolution: ResolutionProbe,
				DependsOn: []string{preview}, RemedyEffectIDs: []string{"effect.gmail.selected-imap-read.v1"},
			},
		}
	case "database.supabase.inspect":
		runtime := "predicate.codex.capability-runtime.v1"
		return []PredicateTemplate{
			{ID: runtime, Resolution: ResolutionProbe, DependsOn: []string{}, RemedyEffectIDs: []string{codexCapabilityRuntimeEffectID}},
			{
				ID: "predicate.supabase.selected-project-readonly.v1", Resolution: ResolutionProbe,
				DependsOn: []string{runtime}, RemedyEffectIDs: []string{"effect.supabase.selected-project-readonly.v1"},
			},
		}
	default:
		return nil
	}
}

func bindingPredicateShapes(catalog Catalog, binding BindingDefinition, profile ProfileDefinition) []PredicateTemplate {
	depends := make([]string, 0, 32)
	for _, shape := range profilePredicateShapes(profile) {
		depends = append(depends, shape.ID)
	}
	for _, capabilityID := range binding.CapabilityIDs {
		logical, _ := catalog.capability(capabilityID)
		for _, shape := range logicalPredicateShapes(logical) {
			depends = append(depends, shape.ID)
		}
	}
	shape := PredicateTemplate{
		ID:              "predicate.binding." + binding.ID + ".v1",
		DependsOn:       depends,
		RemedyEffectIDs: []string{},
		Resolution:      ResolutionDerived,
	}
	if strings.HasPrefix(binding.ID, "codex-appserver+") {
		shape.Resolution = ResolutionProbe
		shape.RemedyEffectIDs = []string{codexCapabilityRuntimeEffectID}
	}
	return []PredicateTemplate{shape}
}

func validatePredicateShapes(predicates []Predicate, expected []PredicateTemplate) error {
	if len(predicates) != len(expected) {
		return fmt.Errorf("predicate vector has %d rows, want %d", len(predicates), len(expected))
	}
	for i, shape := range expected {
		predicate := predicates[i]
		if predicate.ID != shape.ID || predicate.Resolution != shape.Resolution ||
			!equalStrings(predicate.DependsOn, shape.DependsOn) ||
			!equalStrings(predicate.RemedyEffectIDs, shape.RemedyEffectIDs) {
			return fmt.Errorf("predicate row %d does not match the catalog", i)
		}
	}
	return nil
}

// ProfilePredicateTemplates returns a defensive copy of the exact predicate
// graph for a closed profile ID.
func (c Catalog) ProfilePredicateTemplates(profileID string) ([]PredicateTemplate, bool) {
	profile, ok := c.profile(profileID)
	if !ok {
		return nil, false
	}
	return clonePredicateTemplates(profilePredicateShapes(profile)), true
}

// LogicalPredicateTemplates returns the exact graph for a closed capability.
func (c Catalog) LogicalPredicateTemplates(capabilityID string) ([]PredicateTemplate, bool) {
	logical, ok := c.capability(capabilityID)
	if !ok {
		return nil, false
	}
	return clonePredicateTemplates(logicalPredicateShapes(logical)), true
}

// BindingPredicateTemplates returns the intrinsic graph for one exact
// profile/binding composition.
func (c Catalog) BindingPredicateTemplates(profileID, bindingID string, capabilities []string) ([]PredicateTemplate, bool) {
	profile, ok := c.profile(profileID)
	if !ok {
		return nil, false
	}
	binding, ok := c.bindingFor(profileID, capabilities)
	if !ok || binding.ID != bindingID {
		return nil, false
	}
	return clonePredicateTemplates(bindingPredicateShapes(c, binding, profile)), true
}

func clonePredicateTemplates(in []PredicateTemplate) []PredicateTemplate {
	out := make([]PredicateTemplate, len(in))
	for i, template := range in {
		out[i] = template
		out[i].DependsOn = append([]string{}, template.DependsOn...)
		out[i].RemedyEffectIDs = append([]string{}, template.RemedyEffectIDs...)
	}
	return out
}
