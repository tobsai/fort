package capability

import (
	"reflect"
	"testing"
)

func TestCatalogV2IsClosedAndOrdered(t *testing.T) {
	c := CatalogV2()

	if ProtocolVersion != 3 || c.Version != 3 || c.ProfileMappingVersion != 4 {
		t.Fatalf("versions = protocol:%d catalog:%d mapping:%d, want 3/3/4", ProtocolVersion, c.Version, c.ProfileMappingVersion)
	}
	gotProfiles := make([]string, 0, len(c.Profiles))
	for _, p := range c.Profiles {
		gotProfiles = append(gotProfiles, p.ID)
	}
	wantProfiles := []string{
		"claude:configured-default",
		"claude:sonnet",
		"claude:opus",
		"codex:configured-default",
		"codex:gpt-5.5",
		"codex:gpt-5.6-sol",
		"codex:gpt-5.6-terra",
		"codex:gpt-5.6-luna",
		"codex-subscription:gpt-5.6-sol",
		"hermes:configured-default",
		"hermes:openai-codex/gpt-5.6-sol",
		"openclaw:main",
	}
	if !reflect.DeepEqual(gotProfiles, wantProfiles) {
		t.Fatalf("profiles = %#v, want %#v", gotProfiles, wantProfiles)
	}

	gotCapabilities := make([]string, 0, len(c.Capabilities))
	for _, logical := range c.Capabilities {
		gotCapabilities = append(gotCapabilities, logical.ID)
	}
	if want := []string{"email.gmail.read", "database.supabase.inspect", "model.chat.text-only"}; !reflect.DeepEqual(gotCapabilities, want) {
		t.Fatalf("capabilities = %#v, want %#v", gotCapabilities, want)
	}

	gotBindings := make([]string, 0, len(c.Bindings))
	for _, binding := range c.Bindings {
		gotBindings = append(gotBindings, binding.ID)
	}
	wantBindings := []string{
		"claude-native",
		"codex-native",
		"hermes-native",
		"openclaw-main",
		"codex-appserver+gmail",
		"codex-appserver+supabase",
		"codex-subscription-chat",
	}
	if !reflect.DeepEqual(gotBindings, wantBindings) {
		t.Fatalf("bindings = %#v, want %#v", gotBindings, wantBindings)
	}
}

func TestCatalogV2LowersSubscriptionProfileWithoutChangingProviderIdentity(t *testing.T) {
	catalog := CatalogV2()
	agent, model, ok := catalog.RuntimeSelection("codex-subscription:gpt-5.6-sol")
	if !ok || agent != "codex-subscription" || model != "gpt-5.6-sol" {
		t.Fatalf("RuntimeSelection = %q/%q,%v", agent, model, ok)
	}
	profile, ok := catalog.profile("codex-subscription:gpt-5.6-sol")
	if !ok || profile.Adapter != "profile.codex-subscription.isolated" || profile.DisplayName != "Codex · GPT-5.6 Sol (subscription)" {
		t.Fatalf("profile = %#v,%v", profile, ok)
	}
	binding, ok := catalog.bindingFor(profile.ID, []string{"model.chat.text-only"})
	if !ok || binding.ID != "codex-subscription-chat" || binding.RuntimeContract != "codex_subscription_exec_v1" {
		t.Fatalf("binding = %#v,%v", binding, ok)
	}
}

func TestSubscriptionCatalogUsesOneClosedProbeAndDerivedAdapterComposition(t *testing.T) {
	catalog := CatalogV2()
	profile, ok := catalog.ProfilePredicateTemplates("codex-subscription:gpt-5.6-sol")
	if !ok || !reflect.DeepEqual(profile, []PredicateTemplate{{
		ID: "predicate.codex-subscription.closed-contract.v1", Resolution: ResolutionProbe,
		DependsOn: []string{}, RemedyEffectIDs: []string{"effect.codex-subscription-0.149.0-alpha.4.1.v2"},
	}}) {
		t.Fatalf("profile predicates = %#v,%v", profile, ok)
	}
	logical, ok := catalog.LogicalPredicateTemplates("model.chat.text-only")
	if !ok || !reflect.DeepEqual(logical, []PredicateTemplate{{
		ID: "predicate.codex-subscription.text-only-adapter.v1", Resolution: ResolutionDerived,
		DependsOn: []string{}, RemedyEffectIDs: []string{},
	}}) {
		t.Fatalf("logical predicates = %#v,%v", logical, ok)
	}
	binding, ok := catalog.BindingPredicateTemplates(
		"codex-subscription:gpt-5.6-sol", "codex-subscription-chat", []string{"model.chat.text-only"},
	)
	if !ok || len(binding) != 1 || binding[0].Resolution != ResolutionDerived ||
		!reflect.DeepEqual(binding[0].DependsOn, []string{
			"predicate.codex-subscription.closed-contract.v1",
			"predicate.codex-subscription.text-only-adapter.v1",
		}) {
		t.Fatalf("binding predicates = %#v,%v", binding, ok)
	}
}

func TestCatalogV2MapsOnlyApprovedLegacyLabels(t *testing.T) {
	c := CatalogV2()
	tests := []struct {
		agent string
		label string
		want  string
		ok    bool
	}{
		{"claude", "", "claude:configured-default", true},
		{"claude", "Sonnet", "claude:sonnet", true},
		{"claude", "Opus", "claude:opus", true},
		{"codex", "", "codex:configured-default", true},
		{"codex", "5.6 Sol", "codex:gpt-5.6-sol", true},
		{"hermes", "Codex 5.6 Sol", "hermes:openai-codex/gpt-5.6-sol", true},
		{"openclaw", "", "openclaw:main", true},
		{"openclaw", "Fable", "openclaw:main", true},
		{"codex", "GPT 5.5", "", false},
		{"codex", "gpt-5.5", "", false},
		{"unknown", "", "", false},
	}
	for _, tt := range tests {
		got, ok := c.MapLegacyProfile(tt.agent, tt.label)
		if got != tt.want || ok != tt.ok {
			t.Errorf("MapLegacyProfile(%q,%q) = %q,%v; want %q,%v", tt.agent, tt.label, got, ok, tt.want, tt.ok)
		}
	}
}

func TestCatalogV2LowersFirstClassGPT55Profile(t *testing.T) {
	agent, model, ok := CatalogV2().RuntimeSelection("codex:gpt-5.5")
	if !ok || agent != "codex" || model != "gpt-5.5" {
		t.Fatalf("RuntimeSelection = %q/%q,%v", agent, model, ok)
	}
}

func TestCatalogV2LowersApprovedGPT56ProfilesExactly(t *testing.T) {
	tests := []struct {
		id      string
		model   string
		display string
	}{
		{id: "codex:gpt-5.6-terra", model: "gpt-5.6-terra", display: "Codex · GPT-5.6 Terra"},
		{id: "codex:gpt-5.6-luna", model: "gpt-5.6-luna", display: "Codex · GPT-5.6 Luna"},
	}
	catalog := CatalogV2()
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			agent, model, ok := catalog.RuntimeSelection(test.id)
			if !ok || agent != "codex" || model != test.model {
				t.Fatalf("RuntimeSelection = %q/%q,%v", agent, model, ok)
			}
			definition, ok := catalog.profile(test.id)
			if !ok || definition.DisplayName != test.display {
				t.Fatalf("profile = %#v,%v", definition, ok)
			}
		})
	}
}

func TestReasonPrecedenceIsTotal(t *testing.T) {
	reasons := []Reason{
		ReasonCapabilityDrift,
		ReasonAuthRequired,
		ReasonUnsupportedPlatform,
		ReasonProbeFailed,
	}
	if got := FirstReason(reasons...); got != ReasonUnsupportedPlatform {
		t.Fatalf("FirstReason = %q, want %q", got, ReasonUnsupportedPlatform)
	}
	if got := FirstReason(ReasonProbeFailed, ReasonAuthRequired); got != ReasonAuthRequired {
		t.Fatalf("FirstReason = %q, want %q", got, ReasonAuthRequired)
	}
}
