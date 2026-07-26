package capability

import (
	"reflect"
	"testing"
)

func TestCatalogV1IsClosedAndOrdered(t *testing.T) {
	c := CatalogV1()

	if c.Version != 1 || c.ProfileMappingVersion != 1 {
		t.Fatalf("versions = %d/%d, want 1/1", c.Version, c.ProfileMappingVersion)
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
	if want := []string{"email.gmail.read", "database.supabase.inspect"}; !reflect.DeepEqual(gotCapabilities, want) {
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
	}
	if !reflect.DeepEqual(gotBindings, wantBindings) {
		t.Fatalf("bindings = %#v, want %#v", gotBindings, wantBindings)
	}
}

func TestCatalogV1MapsOnlyApprovedLegacyLabels(t *testing.T) {
	c := CatalogV1()
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

func TestCatalogV1LowersFirstClassGPT55Profile(t *testing.T) {
	agent, model, ok := CatalogV1().RuntimeSelection("codex:gpt-5.5")
	if !ok || agent != "codex" || model != "gpt-5.5" {
		t.Fatalf("RuntimeSelection = %q/%q,%v", agent, model, ok)
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
