package conversation

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type AgentState string

const (
	AgentOpen     AgentState = "open"
	AgentArchived AgentState = "archived"
)

// Agent is the stable Fort identity. Its current revision links may advance,
// while its ID and canonical Conversation remain unchanged.
type Agent struct {
	ID                        string     `json:"id"`
	AccountID                 string     `json:"account_id"`
	State                     AgentState `json:"state"`
	CurrentProfileRevisionID  string     `json:"current_profile_revision_id"`
	CurrentBehaviorRevisionID string     `json:"current_behavior_revision_id"`
	CurrentBindingRevisionID  string     `json:"current_binding_revision_id"`
	CanonicalConversationID   string     `json:"canonical_conversation_id"`
	CreatedAt                 time.Time  `json:"created_at"`
}

type AgentProfileRevision struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Revision  int       `json:"revision"`
	Name      string    `json:"name"`
	Title     string    `json:"title,omitempty"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Hidden    bool      `json:"hidden"`
	Pinned    bool      `json:"pinned"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

type AgentBehaviorRevision struct {
	ID                   string    `json:"id"`
	AgentID              string    `json:"agent_id"`
	Revision             int       `json:"revision"`
	Role                 string    `json:"role"`
	StandingInstructions string    `json:"standing_instructions,omitempty"`
	EnabledSkills        []string  `json:"enabled_skills"`
	EnabledTools         []string  `json:"enabled_tools"`
	PromptMaterial       string    `json:"prompt_material,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
}

type ResourceSharingScope string

const (
	ResourceProfileScoped ResourceSharingScope = "profile_scoped"
	ResourceMachineShared ResourceSharingScope = "machine_shared"
	ResourceAccountShared ResourceSharingScope = "account_shared"
	ResourceUnknown       ResourceSharingScope = "unknown"
)

type ResourceSharingDisclosure struct {
	ProviderCredentials ResourceSharingScope `json:"provider_credentials"`
	Filesystem          ResourceSharingScope `json:"filesystem"`
	BrowserSessions     ResourceSharingScope `json:"browser_sessions"`
	FrameworkSessions   ResourceSharingScope `json:"framework_sessions"`
	SourceMemory        ResourceSharingScope `json:"source_memory"`
	ToolConfiguration   ResourceSharingScope `json:"tool_configuration"`
}

type ExecutionSource struct {
	ID              string                    `json:"id"`
	AccountID       string                    `json:"account_id"`
	Framework       string                    `json:"framework"`
	InstanceID      string                    `json:"instance_id"`
	GatewayID       string                    `json:"gateway_id"`
	DisplayName     string                    `json:"display_name"`
	ResourceSharing ResourceSharingDisclosure `json:"resource_sharing"`
	LastSeenAt      time.Time                 `json:"last_seen_at,omitempty"`
}

type SourceAgent struct {
	ID                  string    `json:"id"`
	ExecutionSourceID   string    `json:"execution_source_id"`
	OpaqueSourceAgentID string    `json:"opaque_source_agent_id"`
	DisplayName         string    `json:"display_name"`
	LastSeenAt          time.Time `json:"last_seen_at,omitempty"`
}

// SourceAgentIdentity is deliberately source-qualified. Display names are not
// part of identity and cannot merge two framework-native Agents.
type SourceAgentIdentity struct {
	ExecutionSourceID   string
	OpaqueSourceAgentID string
}

type AgentBindingRevision struct {
	ID                        string    `json:"id"`
	AgentID                   string    `json:"agent_id"`
	Revision                  int       `json:"revision"`
	BehaviorRevisionID        string    `json:"behavior_revision_id"`
	ExecutionSourceID         string    `json:"execution_source_id"`
	SourceAgentID             string    `json:"source_agent_id"`
	SeatID                    string    `json:"seat_id"`
	FortProfile               string    `json:"fort_profile"`
	Provider                  string    `json:"provider"`
	RequestedModel            string    `json:"requested_model"`
	ResolvedModel             string    `json:"resolved_model"`
	ComputerID                string    `json:"computer_id,omitempty"`
	CloudRuntime              string    `json:"cloud_runtime,omitempty"`
	AdapterID                 string    `json:"adapter_id"`
	AdapterRevision           string    `json:"adapter_revision"`
	SourceConfigDigest        string    `json:"source_config_digest"`
	AuthorityID               string    `json:"authority_id"`
	AuthorityRevision         string    `json:"authority_revision"`
	PolicyID                  string    `json:"policy_id"`
	PolicyRevision            string    `json:"policy_revision"`
	SessionBehavior           string    `json:"session_behavior"`
	MemoryBehavior            string    `json:"memory_behavior"`
	CapabilityEvidence        []string  `json:"capability_evidence"`
	ReadinessContractID       string    `json:"readiness_contract_id"`
	ReadinessContractRevision string    `json:"readiness_contract_revision"`
	SupersedesRevisionID      string    `json:"supersedes_revision_id,omitempty"`
	ActivatedAt               time.Time `json:"activated_at"`
	RetiredAt                 time.Time `json:"retired_at,omitempty"`
}

type AgentConversationKind string

const (
	AgentConversationCanonical AgentConversationKind = "canonical"
	AgentConversationSecondary AgentConversationKind = "secondary"
)

type AgentConversation struct {
	AgentID        string                `json:"agent_id"`
	ConversationID string                `json:"conversation_id"`
	Kind           AgentConversationKind `json:"kind"`
	CreatedAt      time.Time             `json:"created_at"`
}

func (a Agent) Validate() error {
	if err := requireStrings("Agent", map[string]string{
		"id": a.ID, "account id": a.AccountID, "current profile revision id": a.CurrentProfileRevisionID,
		"current behavior revision id": a.CurrentBehaviorRevisionID, "current binding revision id": a.CurrentBindingRevisionID,
		"canonical Conversation id": a.CanonicalConversationID,
	}); err != nil {
		return err
	}
	if a.State != AgentOpen && a.State != AgentArchived {
		return fmt.Errorf("Agent state must be open or archived")
	}
	if a.CreatedAt.IsZero() {
		return fmt.Errorf("Agent creation time is required")
	}
	return nil
}

func (r AgentProfileRevision) Validate() error {
	if err := requireStrings("Agent Profile Revision", map[string]string{"id": r.ID, "Agent id": r.AgentID, "name": r.Name}); err != nil {
		return err
	}
	if r.Revision < 1 || r.CreatedAt.IsZero() {
		return fmt.Errorf("Agent Profile Revision number and creation time are required")
	}
	if len([]byte(strings.TrimSpace(r.Name))) > 120 {
		return fmt.Errorf("Agent Profile Revision name exceeds 120 UTF-8 bytes")
	}
	return nil
}

func (r AgentBehaviorRevision) Validate() error {
	if err := requireStrings("Agent Behavior Revision", map[string]string{"id": r.ID, "Agent id": r.AgentID, "role": r.Role}); err != nil {
		return err
	}
	if r.Revision < 1 || r.CreatedAt.IsZero() {
		return fmt.Errorf("Agent Behavior Revision number and creation time are required")
	}
	if err := requireUniqueValues("enabled skill", r.EnabledSkills); err != nil {
		return err
	}
	return requireUniqueValues("enabled tool", r.EnabledTools)
}

func (s ExecutionSource) Validate() error {
	if err := requireStrings("Execution Source", map[string]string{
		"id": s.ID, "account id": s.AccountID, "framework": s.Framework, "instance id": s.InstanceID,
		"gateway id": s.GatewayID, "display name": s.DisplayName,
	}); err != nil {
		return err
	}
	sharing := []ResourceSharingScope{
		s.ResourceSharing.ProviderCredentials, s.ResourceSharing.Filesystem,
		s.ResourceSharing.BrowserSessions, s.ResourceSharing.FrameworkSessions,
		s.ResourceSharing.SourceMemory, s.ResourceSharing.ToolConfiguration,
	}
	for _, scope := range sharing {
		if !scope.Valid() {
			return fmt.Errorf("Execution Source has invalid resource-sharing scope %q", scope)
		}
	}
	return nil
}

func (s ResourceSharingScope) Valid() bool {
	return s == ResourceProfileScoped || s == ResourceMachineShared || s == ResourceAccountShared || s == ResourceUnknown
}

func (a SourceAgent) Validate() error {
	return requireStrings("Source Agent", map[string]string{
		"id": a.ID, "Execution Source id": a.ExecutionSourceID,
		"opaque source Agent id": a.OpaqueSourceAgentID, "display name": a.DisplayName,
	})
}

func (a SourceAgent) Identity() (SourceAgentIdentity, error) {
	if err := a.Validate(); err != nil {
		return SourceAgentIdentity{}, err
	}
	return SourceAgentIdentity{
		ExecutionSourceID: a.ExecutionSourceID, OpaqueSourceAgentID: a.OpaqueSourceAgentID,
	}, nil
}

func (r AgentBindingRevision) Validate() error {
	if err := requireStrings("Agent Binding Revision", map[string]string{
		"id": r.ID, "Agent id": r.AgentID, "Behavior Revision id": r.BehaviorRevisionID,
		"Execution Source id": r.ExecutionSourceID, "Source Agent id": r.SourceAgentID, "seat id": r.SeatID,
		"Fort profile": r.FortProfile, "provider": r.Provider, "requested model": r.RequestedModel,
		"resolved model": r.ResolvedModel, "adapter id": r.AdapterID, "adapter revision": r.AdapterRevision,
		"authority id": r.AuthorityID, "authority revision": r.AuthorityRevision, "policy id": r.PolicyID,
		"policy revision": r.PolicyRevision, "session behavior": r.SessionBehavior, "memory behavior": r.MemoryBehavior,
		"readiness contract id": r.ReadinessContractID, "readiness contract revision": r.ReadinessContractRevision,
	}); err != nil {
		return err
	}
	if r.Revision < 1 || r.ActivatedAt.IsZero() {
		return fmt.Errorf("Agent Binding Revision number and activation time are required")
	}
	if (strings.TrimSpace(r.ComputerID) == "") == (strings.TrimSpace(r.CloudRuntime) == "") {
		return fmt.Errorf("Agent Binding Revision requires exactly one computer or cloud runtime")
	}
	if !isLowerSHA256(r.SourceConfigDigest) {
		return fmt.Errorf("Agent Binding Revision source configuration digest must be lowercase SHA-256")
	}
	if len(r.CapabilityEvidence) == 0 {
		return fmt.Errorf("Agent Binding Revision capability evidence is required")
	}
	return requireUniqueValues("capability evidence", r.CapabilityEvidence)
}

func (c AgentConversation) Validate() error {
	if err := requireStrings("Agent Conversation", map[string]string{"Agent id": c.AgentID, "Conversation id": c.ConversationID}); err != nil {
		return err
	}
	if c.Kind != AgentConversationCanonical && c.Kind != AgentConversationSecondary {
		return fmt.Errorf("Agent Conversation kind must be canonical or secondary")
	}
	if c.CreatedAt.IsZero() {
		return fmt.Errorf("Agent Conversation creation time is required")
	}
	return nil
}

// ValidateAgentRevisionSet checks the public aggregate boundary used when an
// Agent and its currently selected revisions are committed together.
func ValidateAgentRevisionSet(agent Agent, profile AgentProfileRevision, behavior AgentBehaviorRevision, binding AgentBindingRevision, conversations []AgentConversation) error {
	if err := agent.Validate(); err != nil {
		return err
	}
	if err := profile.Validate(); err != nil {
		return err
	}
	if err := behavior.Validate(); err != nil {
		return err
	}
	if err := binding.Validate(); err != nil {
		return err
	}
	if profile.AgentID != agent.ID || behavior.AgentID != agent.ID || binding.AgentID != agent.ID {
		return fmt.Errorf("current revisions must belong to the stable Agent")
	}
	if profile.ID != agent.CurrentProfileRevisionID || behavior.ID != agent.CurrentBehaviorRevisionID ||
		binding.ID != agent.CurrentBindingRevisionID || binding.BehaviorRevisionID != behavior.ID {
		return fmt.Errorf("Agent current revision links must match the supplied revisions")
	}
	canonical := 0
	for _, link := range conversations {
		if err := link.Validate(); err != nil {
			return err
		}
		if link.AgentID != agent.ID {
			return fmt.Errorf("Agent Conversation belongs to another Agent")
		}
		if link.Kind == AgentConversationCanonical {
			canonical++
			if link.ConversationID != agent.CanonicalConversationID {
				return fmt.Errorf("Agent canonical Conversation link does not match Home")
			}
		}
	}
	if canonical != 1 {
		return fmt.Errorf("Agent requires exactly one canonical Conversation")
	}
	return nil
}

func requireStrings(subject string, fields map[string]string) error {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := fields[name]
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s %s is required", subject, name)
		}
	}
	return nil
}

func requireUniqueValues(subject string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			return fmt.Errorf("%s cannot be blank", subject)
		}
		if normalized != value {
			return fmt.Errorf("%s %q is not normalized", subject, value)
		}
		if _, exists := seen[normalized]; exists {
			return fmt.Errorf("%s %q is duplicated", subject, value)
		}
		seen[normalized] = struct{}{}
	}
	return nil
}
