package runtime

import (
	"fmt"
	"strings"
)

type AuthorityMode string
type ReasoningEffort string

const (
	AuthorityChatSubscriptionIsolatedV1    AuthorityMode = "chat_subscription_isolated_v1"
	PolicyCodexSubscriptionChatV1                        = "codex-subscription-chat-v1"
	RuntimeContractCodexSubscriptionExecV1               = "codex_subscription_exec_v1"
	AdapterCodexSubscription                             = "model.chat.text-only.codex-subscription"

	ReasoningEffortMedium       ReasoningEffort = "medium"
	ReasoningContextCurrentTurn                 = "current_turn"
	AccountTypeChatGPT                          = "chatgpt"
	ThreadModeEphemeral                         = "ephemeral"
	SandboxModeReadOnly                         = "readOnly"
	ApprovalPolicyNever                         = "never"
	WorkdirModeEmptyPerTarget                   = "empty_per_target"
	ToolsModeNone                               = "none"
	ResourcePolicyDenyAndFail                   = "deny_and_fail"
	UnknownProviderIdentity                     = "unknown"
	UsageSourceCodexExecJSONL                   = "codex_exec_jsonl"

	ErrorChatPolicyUnavailable  = "chat_policy_unavailable"
	ErrorChatAuthorityViolation = "chat_authority_violation"
	ErrorProviderResultUnknown  = "provider_result_unknown"
	ErrorProviderFailed         = "provider_failed"
)

// TextOnlyPolicy is the complete wire-visible subscription execution policy.
// Authentication material and process-private executable paths never enter it.
type TextOnlyPolicy struct {
	PolicyID                        string          `json:"policy_id"`
	PolicyRevision                  string          `json:"policy_revision"`
	Model                           string          `json:"model"`
	ReasoningEffort                 ReasoningEffort `json:"reasoning_effort"`
	ReasoningContext                string          `json:"reasoning_context"`
	RequestTimeoutMillis            int             `json:"request_timeout_millis"`
	DeveloperInstructionRevision    string          `json:"developer_instruction_revision"`
	AccountType                     string          `json:"account_type"`
	AccountPlan                     string          `json:"account_plan"`
	SelectedAdapterID               string          `json:"selected_adapter_id"`
	SelectedAdapterRevision         string          `json:"selected_adapter_revision"`
	SelectedCodexVersion            string          `json:"selected_codex_version"`
	SelectedCodexExecutableRevision string          `json:"selected_codex_executable_revision"`
	SelectedCodexSchemaRevision     string          `json:"selected_codex_schema_revision"`
	ThreadMode                      string          `json:"thread_mode"`
	SandboxMode                     string          `json:"sandbox_mode"`
	ApprovalPolicy                  string          `json:"approval_policy"`
	WorkdirMode                     string          `json:"workdir_mode"`
	DynamicToolsMode                string          `json:"dynamic_tools_mode"`
	MCPMode                         string          `json:"mcp_mode"`
	CommandPolicy                   string          `json:"command_policy"`
	FileReadPolicy                  string          `json:"file_read_policy"`
	IsolationRevision               string          `json:"isolation_revision"`
}

type ProviderUsage struct {
	InputTokens       int64 `json:"input_tokens"`
	CachedInputTokens int64 `json:"cached_input_tokens"`
	OutputTokens      int64 `json:"output_tokens"`
	ReasoningTokens   int64 `json:"reasoning_tokens"`
}

type ResponseMetadata struct {
	ProviderThreadID                string        `json:"provider_thread_id"`
	RequestedModel                  string        `json:"requested_model"`
	ResolvedModel                   string        `json:"resolved_model"`
	SelectedAdapterID               string        `json:"selected_adapter_id"`
	SelectedAdapterRevision         string        `json:"selected_adapter_revision"`
	SelectedCodexVersion            string        `json:"selected_codex_version"`
	SelectedCodexExecutableRevision string        `json:"selected_codex_executable_revision"`
	SelectedCodexSchemaRevision     string        `json:"selected_codex_schema_revision"`
	ObservedAdapterID               string        `json:"observed_adapter_id"`
	ObservedAdapterRevision         string        `json:"observed_adapter_revision"`
	ObservedCodexVersion            string        `json:"observed_codex_version"`
	ObservedCodexExecutableRevision string        `json:"observed_codex_executable_revision"`
	ObservedCodexSchemaRevision     string        `json:"observed_codex_schema_revision"`
	TerminalStatus                  string        `json:"terminal_status"`
	UsageSource                     string        `json:"usage_source"`
	Usage                           ProviderUsage `json:"usage"`
}

func (s RunSpec) ValidateAuthority() error {
	if s.Authority == "" {
		if s.RuntimeContract != "" || s.TextOnlyPolicy != nil || s.ExpectedPolicyRevision != "" {
			return fmt.Errorf("runtime: legacy authority contains subscription fields")
		}
		return nil
	}
	if s.Authority != AuthorityChatSubscriptionIsolatedV1 ||
		s.RuntimeContract != RuntimeContractCodexSubscriptionExecV1 ||
		s.Agent != "codex-subscription" || s.TextOnlyPolicy == nil {
		return fmt.Errorf("runtime: unsupported authority contract")
	}
	p := s.TextOnlyPolicy
	if s.Profile != "codex-subscription:"+s.Model || s.Model == "" || p.Model != s.Model {
		return fmt.Errorf("runtime: subscription identity mismatch")
	}
	if s.Workdir != "" || len(s.Env) != 0 {
		return fmt.Errorf("runtime: subscription workdir and environment are adapter-owned")
	}
	if p.PolicyID != PolicyCodexSubscriptionChatV1 || !lowerSHA256(p.PolicyRevision) ||
		p.ReasoningEffort != ReasoningEffortMedium || p.ReasoningContext != ReasoningContextCurrentTurn ||
		p.RequestTimeoutMillis != 120000 || !lowerSHA256(p.DeveloperInstructionRevision) ||
		p.AccountType != AccountTypeChatGPT || !knownChatGPTPlan(p.AccountPlan) ||
		p.SelectedAdapterID != AdapterCodexSubscription || !lowerSHA256(p.SelectedAdapterRevision) ||
		strings.TrimSpace(p.SelectedCodexVersion) == "" ||
		!lowerSHA256(p.SelectedCodexExecutableRevision) || !lowerSHA256(p.SelectedCodexSchemaRevision) ||
		p.ThreadMode != ThreadModeEphemeral || p.SandboxMode != SandboxModeReadOnly ||
		p.ApprovalPolicy != ApprovalPolicyNever || p.WorkdirMode != WorkdirModeEmptyPerTarget ||
		p.DynamicToolsMode != ToolsModeNone || p.MCPMode != ToolsModeNone ||
		p.CommandPolicy != ResourcePolicyDenyAndFail || p.FileReadPolicy != ResourcePolicyDenyAndFail ||
		!lowerSHA256(p.IsolationRevision) {
		return fmt.Errorf("runtime: invalid subscription policy")
	}
	if s.ExpectedPolicyRevision != "" && s.ExpectedPolicyRevision != p.PolicyRevision {
		return fmt.Errorf("runtime: expected policy revision mismatch")
	}
	return nil
}

func (m ResponseMetadata) Validate() error {
	if strings.TrimSpace(m.ProviderThreadID) == "" || strings.TrimSpace(m.RequestedModel) == "" ||
		m.ResolvedModel != UnknownProviderIdentity || m.SelectedAdapterID != AdapterCodexSubscription ||
		!lowerSHA256(m.SelectedAdapterRevision) || strings.TrimSpace(m.SelectedCodexVersion) == "" ||
		!lowerSHA256(m.SelectedCodexExecutableRevision) || !lowerSHA256(m.SelectedCodexSchemaRevision) ||
		m.ObservedAdapterID != m.SelectedAdapterID || m.ObservedAdapterRevision != m.SelectedAdapterRevision ||
		m.ObservedCodexVersion != m.SelectedCodexVersion ||
		m.ObservedCodexExecutableRevision != m.SelectedCodexExecutableRevision ||
		m.ObservedCodexSchemaRevision != m.SelectedCodexSchemaRevision ||
		m.TerminalStatus != "completed" || m.UsageSource != UsageSourceCodexExecJSONL ||
		m.Usage.InputTokens < 0 || m.Usage.CachedInputTokens < 0 ||
		m.Usage.OutputTokens <= 0 || m.Usage.ReasoningTokens < 0 ||
		m.Usage.CachedInputTokens > m.Usage.InputTokens || m.Usage.ReasoningTokens > m.Usage.OutputTokens {
		return fmt.Errorf("runtime: invalid response metadata")
	}
	return nil
}

func lowerSHA256(value string) bool {
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

func knownChatGPTPlan(value string) bool {
	switch value {
	case "free", "go", "plus", "pro", "prolite", "team",
		"self_serve_business_prolite", "self_serve_business_usage_based", "business",
		"ent26", "enterprise_cbp_automation", "enterprise_cbp_usage_based",
		"enterprise", "edu":
		return true
	default:
		return false
	}
}
