package conversation

import (
	"fmt"
	"strings"
	"time"
)

const (
	AuthorityChatSubscriptionIsolatedV1    = "chat_subscription_isolated_v1"
	PolicyCodexSubscriptionChatV1          = "codex-subscription-chat-v1"
	RuntimeContractCodexSubscriptionExecV1 = "codex_subscription_exec_v1"
	AdapterCodexSubscription               = "model.chat.text-only.codex-subscription"
	AccountTypeChatGPT                     = "chatgpt"
	ThreadModeEphemeral                    = "ephemeral"
	SandboxModeReadOnly                    = "readOnly"
	ApprovalPolicyNever                    = "never"
	WorkdirModeEmptyPerTarget              = "empty_per_target"
	ToolsModeNone                          = "none"
	ResourcePolicyDenyAndFail              = "deny_and_fail"
	UnknownProviderIdentity                = "unknown"
	UsageSourceCodexExecJSONL              = "codex_exec_jsonl"
)

// SubscriptionPolicy is the immutable, non-secret execution boundary for one
// Primary Channel. It records what Fort selected; readiness remains a live
// projection and is deliberately not part of this snapshot.
type SubscriptionPolicy struct {
	PolicyID                     string `json:"policy_id"`
	PolicyRevision               string `json:"policy_revision"`
	AdapterID                    string `json:"adapter_id"`
	AdapterRevision              string `json:"adapter_revision"`
	CodexVersion                 string `json:"codex_version"`
	CodexExecutableRevision      string `json:"codex_executable_revision"`
	CodexSchemaRevision          string `json:"codex_schema_revision"`
	RuntimeContract              string `json:"runtime_contract"`
	ReasoningEffort              string `json:"reasoning_effort"`
	ReasoningContext             string `json:"reasoning_context"`
	RequestTimeoutMillis         int    `json:"request_timeout_millis"`
	DeveloperInstructionRevision string `json:"developer_instruction_revision"`
	AccountType                  string `json:"account_type"`
	AccountPlan                  string `json:"account_plan"`
	ThreadMode                   string `json:"thread_mode"`
	SandboxMode                  string `json:"sandbox_mode"`
	ApprovalPolicy               string `json:"approval_policy"`
	WorkdirMode                  string `json:"workdir_mode"`
	DynamicToolsMode             string `json:"dynamic_tools_mode"`
	MCPMode                      string `json:"mcp_mode"`
	CommandPolicy                string `json:"command_policy"`
	FileReadPolicy               string `json:"file_read_policy"`
	IsolationRevision            string `json:"isolation_revision"`
}

// PrimaryAgentSetting is the singleton selection used only for Channels
// created after it is written. Existing Channels retain their own snapshot.
type PrimaryAgentSetting struct {
	OptionID  string             `json:"option_id"`
	Seat      Seat               `json:"seat"`
	Authority string             `json:"authority"`
	Policy    SubscriptionPolicy `json:"policy"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// PrimaryChannel marks one canonical conversation as a private Channel and
// binds it to exactly one immutable participant and subscription policy.
type PrimaryChannel struct {
	ConversationID string             `json:"conversation_id"`
	ParticipantID  string             `json:"participant_id"`
	Authority      string             `json:"authority"`
	Policy         SubscriptionPolicy `json:"policy"`
	CreatedAt      time.Time          `json:"created_at"`
}

type PrimaryChannelSummary struct {
	Conversation Conversation   `json:"conversation"`
	Participant  Participant    `json:"participant"`
	Identity     PrimaryChannel `json:"primary_identity"`
	Pinned       bool           `json:"pinned"`
	PinnedAt     time.Time      `json:"pinned_at,omitempty"`
}

// TargetAuthority records the exact authority selected before one target is
// committed. Legacy targets have no TargetAuthority.
type TargetAuthority struct {
	Authority      string             `json:"authority"`
	Policy         SubscriptionPolicy `json:"policy"`
	RequestedModel string             `json:"requested_model"`
}

// TargetReceipt is provider-observed terminal metadata. It is separate from
// the immutable selected authority and may be absent for legacy targets.
type TargetReceipt struct {
	ObservedAdapterID               string `json:"observed_adapter_id,omitempty"`
	ObservedAdapterRevision         string `json:"observed_adapter_revision,omitempty"`
	ObservedCodexVersion            string `json:"observed_codex_version,omitempty"`
	ObservedCodexExecutableRevision string `json:"observed_codex_executable_revision,omitempty"`
	ObservedCodexSchemaRevision     string `json:"observed_codex_schema_revision,omitempty"`
	ResolvedModel                   string `json:"resolved_model,omitempty"`
	ProviderThreadID                string `json:"provider_thread_id,omitempty"`
	ProviderTerminalStatus          string `json:"provider_terminal_status,omitempty"`
	UsageSource                     string `json:"usage_source,omitempty"`
	InputTokens                     int64  `json:"input_tokens,omitempty"`
	CachedInputTokens               int64  `json:"cached_input_tokens,omitempty"`
	OutputTokens                    int64  `json:"output_tokens,omitempty"`
	ReasoningTokens                 int64  `json:"reasoning_tokens,omitempty"`
}

type ScheduleChannelLink struct {
	ScheduleID     string    `json:"schedule_id"`
	ConversationID string    `json:"conversation_id"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s PrimaryAgentSetting) Validate() error {
	if strings.TrimSpace(s.OptionID) == "" {
		return fmt.Errorf("primary option id is required")
	}
	if strings.TrimSpace(s.Seat.ID) == "" || strings.TrimSpace(s.Seat.Profile) == "" ||
		strings.TrimSpace(s.Seat.Agent) == "" || strings.TrimSpace(s.Seat.Model) == "" ||
		strings.TrimSpace(s.Seat.Machine) == "" || strings.TrimSpace(s.Seat.DisplayName) == "" {
		return fmt.Errorf("primary option requires an exact seat, profile, agent, model, machine, and display name")
	}
	if s.Seat.Agent != "codex-subscription" || s.Seat.Profile != "codex-subscription:"+s.Seat.Model {
		return fmt.Errorf("primary option requires an exact Codex subscription profile")
	}
	return validateSubscriptionPolicy(s.Authority, s.Policy)
}

func (c PrimaryChannel) Validate() error {
	if strings.TrimSpace(c.ConversationID) == "" || strings.TrimSpace(c.ParticipantID) == "" {
		return fmt.Errorf("primary Channel conversation and participant are required")
	}
	return validateSubscriptionPolicy(c.Authority, c.Policy)
}

func (p SubscriptionPolicy) Validate(authority string) error {
	return validateSubscriptionPolicy(authority, p)
}

func (a TargetAuthority) Validate() error {
	if strings.TrimSpace(a.RequestedModel) == "" {
		return fmt.Errorf("subscription target requested model is required")
	}
	return validateSubscriptionPolicy(a.Authority, a.Policy)
}

func (r TargetReceipt) ValidateFor(authority TargetAuthority) error {
	if err := authority.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.ProviderTerminalStatus) == "" || strings.TrimSpace(r.UsageSource) == "" {
		return fmt.Errorf("subscription target receipt needs terminal status and usage source")
	}
	if r.InputTokens < 0 || r.CachedInputTokens < 0 || r.OutputTokens < 0 || r.ReasoningTokens < 0 {
		return fmt.Errorf("subscription target receipt token counts cannot be negative")
	}
	if r.ResolvedModel != "" && r.ResolvedModel != "unknown" {
		return fmt.Errorf("codex exec does not expose a resolved model")
	}
	selected := authority.Policy
	observed := []struct {
		name, got, want string
	}{
		{"adapter", r.ObservedAdapterID, selected.AdapterID},
		{"adapter revision", r.ObservedAdapterRevision, selected.AdapterRevision},
		{"Codex version", r.ObservedCodexVersion, selected.CodexVersion},
		{"Codex executable revision", r.ObservedCodexExecutableRevision, selected.CodexExecutableRevision},
		{"Codex schema revision", r.ObservedCodexSchemaRevision, selected.CodexSchemaRevision},
	}
	for _, identity := range observed {
		if identity.got != identity.want && identity.got != UnknownProviderIdentity {
			return fmt.Errorf("observed %s %q does not match selected authority", identity.name, identity.got)
		}
	}
	if r.ProviderTerminalStatus == "completed" {
		for _, identity := range observed {
			if identity.got != identity.want {
				return fmt.Errorf("completed receipt has unknown %s", identity.name)
			}
		}
		if strings.TrimSpace(r.ProviderThreadID) == "" {
			return fmt.Errorf("completed receipt needs ephemeral thread evidence")
		}
		if r.UsageSource != UsageSourceCodexExecJSONL {
			return fmt.Errorf("completed receipt needs Codex JSONL usage evidence")
		}
	}
	return nil
}

func validateSubscriptionPolicy(authority string, policy SubscriptionPolicy) error {
	if authority != AuthorityChatSubscriptionIsolatedV1 {
		return fmt.Errorf("unsupported primary authority %q", authority)
	}
	if policy.PolicyID != PolicyCodexSubscriptionChatV1 || strings.TrimSpace(policy.PolicyRevision) == "" {
		return fmt.Errorf("invalid subscription chat policy")
	}
	if policy.AdapterID != AdapterCodexSubscription || strings.TrimSpace(policy.AdapterRevision) == "" || strings.TrimSpace(policy.CodexVersion) == "" {
		return fmt.Errorf("subscription adapter identity is required")
	}
	if !isLowerSHA256(policy.CodexExecutableRevision) || !isLowerSHA256(policy.CodexSchemaRevision) {
		return fmt.Errorf("codex executable and schema revisions must be lowercase SHA-256")
	}
	if policy.RuntimeContract != RuntimeContractCodexSubscriptionExecV1 {
		return fmt.Errorf("unsupported subscription runtime contract %q", policy.RuntimeContract)
	}
	if policy.ReasoningEffort != "medium" || policy.ReasoningContext != "current_turn" {
		return fmt.Errorf("invalid subscription reasoning policy")
	}
	if policy.RequestTimeoutMillis != 120_000 || strings.TrimSpace(policy.DeveloperInstructionRevision) == "" {
		return fmt.Errorf("invalid subscription request policy")
	}
	if policy.AccountType != AccountTypeChatGPT || strings.TrimSpace(policy.AccountPlan) == "" || strings.TrimSpace(policy.AccountPlan) != policy.AccountPlan {
		return fmt.Errorf("a normalized ChatGPT subscription plan is required")
	}
	if policy.ThreadMode != ThreadModeEphemeral || policy.SandboxMode != SandboxModeReadOnly || policy.ApprovalPolicy != ApprovalPolicyNever ||
		policy.WorkdirMode != WorkdirModeEmptyPerTarget || policy.DynamicToolsMode != ToolsModeNone || policy.MCPMode != ToolsModeNone ||
		policy.CommandPolicy != ResourcePolicyDenyAndFail || policy.FileReadPolicy != ResourcePolicyDenyAndFail {
		return fmt.Errorf("invalid subscription isolation policy")
	}
	if strings.TrimSpace(policy.IsolationRevision) == "" {
		return fmt.Errorf("subscription isolation revision is required")
	}
	return nil
}

func isLowerSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}
