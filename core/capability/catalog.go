// Package capability owns Fort's closed capability catalog, strict plan
// contract, safe inventory projection, and deterministic placement solver.
//
// It is deliberately pure core code: discovery and execution adapters live
// under exec, while this package makes no runtime or model calls.
package capability

import "strings"

const (
	ProtocolVersion       = 1
	CatalogVersion        = 1
	ProfileMappingVersion = 1
)

// SelectionKind is the closed provider-selection strategy for a profile.
type SelectionKind string

const (
	SelectionConfiguredDefault SelectionKind = "configured_default"
	SelectionModel             SelectionKind = "model"
	SelectionProviderModel     SelectionKind = "provider_model"
	SelectionConfiguredAgent   SelectionKind = "configured_agent"
)

// ProfileSelection contains only provider-native selection data.
type ProfileSelection struct {
	Kind       SelectionKind `json:"kind"`
	ModelID    string        `json:"model_id,omitempty"`
	ProviderID string        `json:"provider_id,omitempty"`
	AgentID    string        `json:"agent_id,omitempty"`
}

// ProfileDefinition is one immutable execution-profile catalog row.
type ProfileDefinition struct {
	ID          string           `json:"id"`
	Agent       string           `json:"agent"`
	Adapter     string           `json:"adapter"`
	Selection   ProfileSelection `json:"selection"`
	DisplayName string           `json:"display_name"`
	legacy      []string
}

// CapabilityDefinition is one logical capability and its only v1 adapter.
type CapabilityDefinition struct {
	ID      string `json:"id"`
	Adapter string `json:"adapter"`
}

// BindingDefinition is one closed, tested stage runtime composition.
type BindingDefinition struct {
	ID                 string   `json:"id"`
	Agent              string   `json:"agent"`
	RuntimeContract    string   `json:"runtime_contract"`
	CapabilityIDs      []string `json:"capabilities"`
	CapabilityAdapters []string `json:"logical_adapters"`
}

// Catalog is the immutable input to validation and placement.
type Catalog struct {
	Version               int                    `json:"version"`
	ProfileMappingVersion int                    `json:"profile_mapping_version"`
	Profiles              []ProfileDefinition    `json:"profiles"`
	Capabilities          []CapabilityDefinition `json:"capabilities"`
	Bindings              []BindingDefinition    `json:"bindings"`
}

// CatalogV1 returns a fresh copy of the approved version-1 catalog.
func CatalogV1() Catalog {
	return Catalog{
		Version:               CatalogVersion,
		ProfileMappingVersion: ProfileMappingVersion,
		Profiles: []ProfileDefinition{
			{ID: "claude:configured-default", Agent: "claude", Adapter: "profile.claude.native", Selection: ProfileSelection{Kind: SelectionConfiguredDefault}, DisplayName: "Claude · configured default", legacy: []string{""}},
			{ID: "claude:sonnet", Agent: "claude", Adapter: "profile.claude.native", Selection: ProfileSelection{Kind: SelectionModel, ModelID: "sonnet"}, DisplayName: "Claude · Sonnet", legacy: []string{"Sonnet"}},
			{ID: "claude:opus", Agent: "claude", Adapter: "profile.claude.native", Selection: ProfileSelection{Kind: SelectionModel, ModelID: "opus"}, DisplayName: "Claude · Opus", legacy: []string{"Opus"}},
			{ID: "codex:configured-default", Agent: "codex", Adapter: "profile.codex.native", Selection: ProfileSelection{Kind: SelectionConfiguredDefault}, DisplayName: "Codex · configured default", legacy: []string{""}},
			{ID: "codex:gpt-5.5", Agent: "codex", Adapter: "profile.codex.native", Selection: ProfileSelection{Kind: SelectionModel, ModelID: "gpt-5.5"}, DisplayName: "Codex · GPT-5.5"},
			{ID: "codex:gpt-5.6-sol", Agent: "codex", Adapter: "profile.codex.native", Selection: ProfileSelection{Kind: SelectionModel, ModelID: "gpt-5.6-sol"}, DisplayName: "Codex · GPT-5.6 Sol", legacy: []string{"5.6 Sol"}},
			{ID: "hermes:configured-default", Agent: "hermes", Adapter: "profile.hermes.native", Selection: ProfileSelection{Kind: SelectionConfiguredDefault}, DisplayName: "Hermes · configured default", legacy: []string{""}},
			{ID: "hermes:openai-codex/gpt-5.6-sol", Agent: "hermes", Adapter: "profile.hermes.native", Selection: ProfileSelection{Kind: SelectionProviderModel, ProviderID: "openai-codex", ModelID: "gpt-5.6-sol"}, DisplayName: "Hermes · Codex GPT-5.6 Sol", legacy: []string{"Codex 5.6 Sol"}},
			{ID: "openclaw:main", Agent: "openclaw", Adapter: "profile.openclaw.main", Selection: ProfileSelection{Kind: SelectionConfiguredAgent, AgentID: "main"}, DisplayName: "OpenClaw · main", legacy: []string{"", "Fable"}},
		},
		Capabilities: []CapabilityDefinition{
			{ID: "email.gmail.read", Adapter: "email.gmail.read.himalaya-broker"},
			{ID: "database.supabase.inspect", Adapter: "database.supabase.inspect.codex-broker"},
		},
		Bindings: []BindingDefinition{
			{ID: "claude-native", Agent: "claude", RuntimeContract: "native-cli", CapabilityIDs: []string{}, CapabilityAdapters: []string{}},
			{ID: "codex-native", Agent: "codex", RuntimeContract: "native-cli", CapabilityIDs: []string{}, CapabilityAdapters: []string{}},
			{ID: "hermes-native", Agent: "hermes", RuntimeContract: "native-cli", CapabilityIDs: []string{}, CapabilityAdapters: []string{}},
			{ID: "openclaw-main", Agent: "openclaw", RuntimeContract: "openclaw-main", CapabilityIDs: []string{}, CapabilityAdapters: []string{}},
			{ID: "codex-appserver+gmail", Agent: "codex", RuntimeContract: "codex-appserver", CapabilityIDs: []string{"email.gmail.read"}, CapabilityAdapters: []string{"email.gmail.read.himalaya-broker"}},
			{ID: "codex-appserver+supabase", Agent: "codex", RuntimeContract: "codex-appserver", CapabilityIDs: []string{"database.supabase.inspect"}, CapabilityAdapters: []string{"database.supabase.inspect.codex-broker"}},
		},
	}
}

// MapLegacyProfile maps only a cataloged agent/label pair. Display names are
// intentionally not accepted as provider identities.
func (c Catalog) MapLegacyProfile(agent, label string) (string, bool) {
	for _, profile := range c.Profiles {
		if profile.Agent != agent {
			continue
		}
		for _, accepted := range profile.legacy {
			if label == accepted {
				return profile.ID, true
			}
		}
	}
	return "", false
}

// RuntimeSelection resolves one closed execution profile into the exact
// provider key and provider-native model selector carried to the runtime. The
// profile ID remains the authoritative identity; these fields are derived
// execution data, not another lookup surface.
func (c Catalog) RuntimeSelection(id string) (agent, model string, ok bool) {
	profile, ok := c.profile(id)
	if !ok || profile.Agent == "" {
		return "", "", false
	}
	switch profile.Selection.Kind {
	case SelectionConfiguredDefault:
		return profile.Agent, "", true
	case SelectionModel:
		if profile.Selection.ModelID == "" {
			return "", "", false
		}
		return profile.Agent, profile.Selection.ModelID, true
	case SelectionProviderModel:
		if profile.Selection.ProviderID == "" || profile.Selection.ModelID == "" {
			return "", "", false
		}
		return profile.Agent, profile.Selection.ProviderID + "/" + profile.Selection.ModelID, true
	case SelectionConfiguredAgent:
		if profile.Selection.AgentID == "" {
			return "", "", false
		}
		return profile.Agent, "", true
	default:
		return "", "", false
	}
}

func (c Catalog) profile(id string) (ProfileDefinition, bool) {
	for _, profile := range c.Profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return ProfileDefinition{}, false
}

func (c Catalog) capability(id string) (CapabilityDefinition, bool) {
	for _, logical := range c.Capabilities {
		if logical.ID == id {
			return logical, true
		}
	}
	return CapabilityDefinition{}, false
}

func (c Catalog) bindingFor(profileID string, requirements []string) (BindingDefinition, bool) {
	profile, ok := c.profile(profileID)
	if !ok {
		return BindingDefinition{}, false
	}
	for _, binding := range c.Bindings {
		if binding.Agent == profile.Agent && equalStrings(binding.CapabilityIDs, requirements) {
			return binding, true
		}
	}
	return BindingDefinition{}, false
}

func (c Catalog) profileRank(id string) int {
	for i, profile := range c.Profiles {
		if profile.ID == id {
			return i
		}
	}
	return len(c.Profiles)
}

func (c Catalog) capabilityRank(id string) int {
	for i, logical := range c.Capabilities {
		if logical.ID == id {
			return i
		}
	}
	return len(c.Capabilities)
}

func (c Catalog) bindingRank(id string) int {
	for i, binding := range c.Bindings {
		if binding.ID == id {
			return i
		}
	}
	return len(c.Bindings)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validOpaqueRevision(value string) bool {
	return strings.HasPrefix(value, "opaque:") && len(value) > len("opaque:")
}
