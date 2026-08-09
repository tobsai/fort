package conversation

import (
	"strings"
	"testing"
	"time"
)

func validPrimaryAgentSetting() PrimaryAgentSetting {
	return PrimaryAgentSetting{
		OptionID: "primary-option:v1:test",
		Seat: Seat{
			ID:          "seat:v1:codex-studio",
			Profile:     "codex-subscription:gpt-5.6-sol",
			Agent:       "codex-subscription",
			Model:       "gpt-5.6-sol",
			Machine:     "studio",
			DisplayName: "Codex on Studio",
		},
		Authority: AuthorityChatSubscriptionIsolatedV1,
		Policy: SubscriptionPolicy{
			PolicyID:                     PolicyCodexSubscriptionChatV1,
			PolicyRevision:               "policy-revision-v1",
			AdapterID:                    AdapterCodexSubscription,
			AdapterRevision:              "codex-exec-adapter-v1",
			CodexVersion:                 "0.120.0",
			CodexExecutableRevision:      strings.Repeat("a", 64),
			CodexSchemaRevision:          strings.Repeat("b", 64),
			RuntimeContract:              RuntimeContractCodexSubscriptionExecV1,
			ReasoningEffort:              "medium",
			ReasoningContext:             "current_turn",
			RequestTimeoutMillis:         120_000,
			DeveloperInstructionRevision: "developer-instruction-v1",
			AccountType:                  AccountTypeChatGPT,
			AccountPlan:                  "plus",
			ThreadMode:                   ThreadModeEphemeral,
			SandboxMode:                  SandboxModeReadOnly,
			ApprovalPolicy:               ApprovalPolicyNever,
			WorkdirMode:                  WorkdirModeEmptyPerTarget,
			DynamicToolsMode:             ToolsModeNone,
			MCPMode:                      ToolsModeNone,
			CommandPolicy:                ResourcePolicyDenyAndFail,
			FileReadPolicy:               ResourcePolicyDenyAndFail,
			IsolationRevision:            "isolation-v1",
		},
		UpdatedAt: time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC),
	}
}

func TestPrimaryAgentSettingRequiresClosedSubscriptionAuthority(t *testing.T) {
	valid := validPrimaryAgentSetting()
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid setting: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*PrimaryAgentSetting)
	}{
		{"authority", func(v *PrimaryAgentSetting) { v.Authority = "legacy" }},
		{"policy", func(v *PrimaryAgentSetting) { v.Policy.PolicyID = "other" }},
		{"account type", func(v *PrimaryAgentSetting) { v.Policy.AccountType = "api" }},
		{"account plan", func(v *PrimaryAgentSetting) { v.Policy.AccountPlan = "  " }},
		{"executable revision uppercase", func(v *PrimaryAgentSetting) { v.Policy.CodexExecutableRevision = strings.Repeat("A", 64) }},
		{"schema revision length", func(v *PrimaryAgentSetting) { v.Policy.CodexSchemaRevision = "abc" }},
		{"runtime contract", func(v *PrimaryAgentSetting) { v.Policy.RuntimeContract = "codex_app_server" }},
		{"adapter", func(v *PrimaryAgentSetting) { v.Policy.AdapterID = "codex-exec" }},
		{"reasoning effort", func(v *PrimaryAgentSetting) { v.Policy.ReasoningEffort = "high" }},
		{"request timeout", func(v *PrimaryAgentSetting) { v.Policy.RequestTimeoutMillis = 60_000 }},
		{"thread mode", func(v *PrimaryAgentSetting) { v.Policy.ThreadMode = "resume" }},
		{"sandbox", func(v *PrimaryAgentSetting) { v.Policy.SandboxMode = "workspaceWrite" }},
		{"approval", func(v *PrimaryAgentSetting) { v.Policy.ApprovalPolicy = "on-request" }},
		{"workdir", func(v *PrimaryAgentSetting) { v.Policy.WorkdirMode = "repository" }},
		{"dynamic tools", func(v *PrimaryAgentSetting) { v.Policy.DynamicToolsMode = "allowed" }},
		{"mcp", func(v *PrimaryAgentSetting) { v.Policy.MCPMode = "configured" }},
		{"command policy", func(v *PrimaryAgentSetting) { v.Policy.CommandPolicy = "allow" }},
		{"file read policy", func(v *PrimaryAgentSetting) { v.Policy.FileReadPolicy = "allow" }},
		{"isolation revision", func(v *PrimaryAgentSetting) { v.Policy.IsolationRevision = "" }},
		{"seat model", func(v *PrimaryAgentSetting) { v.Seat.Model = "" }},
		{"seat provider", func(v *PrimaryAgentSetting) { v.Seat.Agent = "codex" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := valid
			tt.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("invalid setting accepted")
			}
		})
	}
}

func TestTargetAuthorityRequiresExactRequestedModel(t *testing.T) {
	setting := validPrimaryAgentSetting()
	authority := TargetAuthority{
		Authority:      setting.Authority,
		Policy:         setting.Policy,
		RequestedModel: setting.Seat.Model,
	}
	if err := authority.Validate(); err != nil {
		t.Fatalf("valid target authority: %v", err)
	}
	authority.RequestedModel = ""
	if err := authority.Validate(); err == nil {
		t.Fatal("target authority without an exact model accepted")
	}
}

func TestTargetReceiptRejectsInferredResolvedModelAndObservedDrift(t *testing.T) {
	setting := validPrimaryAgentSetting()
	authority := TargetAuthority{Authority: setting.Authority, Policy: setting.Policy, RequestedModel: setting.Seat.Model}
	receipt := TargetReceipt{
		ObservedAdapterID:               setting.Policy.AdapterID,
		ObservedAdapterRevision:         setting.Policy.AdapterRevision,
		ObservedCodexVersion:            setting.Policy.CodexVersion,
		ObservedCodexExecutableRevision: setting.Policy.CodexExecutableRevision,
		ObservedCodexSchemaRevision:     setting.Policy.CodexSchemaRevision,
		ProviderThreadID:                "thread-evidence",
		ProviderTerminalStatus:          "completed",
		UsageSource:                     "codex_exec_jsonl",
	}
	if err := receipt.ValidateFor(authority); err != nil {
		t.Fatalf("valid receipt: %v", err)
	}
	receipt.ResolvedModel = setting.Seat.Model
	if err := receipt.ValidateFor(authority); err == nil {
		t.Fatal("inferred resolved model accepted")
	}
	receipt.ResolvedModel = ""
	receipt.ObservedCodexSchemaRevision = strings.Repeat("c", 64)
	if err := receipt.ValidateFor(authority); err == nil {
		t.Fatal("observed schema drift accepted")
	}
}
