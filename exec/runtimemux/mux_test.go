package runtimemux_test

import (
	"context"
	"strings"
	"testing"

	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/exec/runtimemux"
)

type countingRuntime struct {
	name  string
	calls int
	spec  runtime.RunSpec
}

func (r *countingRuntime) Dispatch(_ context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	r.calls++
	r.spec = spec
	return nil, nil
}

func (r *countingRuntime) Name() string { return r.name }

func subscriptionSpec() runtime.RunSpec {
	return runtime.RunSpec{
		RunID:           "subscription",
		Profile:         "codex-subscription:gpt-5.6-sol",
		Agent:           "codex-subscription",
		Model:           "gpt-5.6-sol",
		Prompt:          "hello",
		Authority:       runtime.AuthorityChatSubscriptionIsolatedV1,
		RuntimeContract: runtime.RuntimeContractCodexSubscriptionExecV1,
		TextOnlyPolicy: &runtime.TextOnlyPolicy{
			PolicyID:                        runtime.PolicyCodexSubscriptionChatV1,
			PolicyRevision:                  strings.Repeat("c", 64),
			Model:                           "gpt-5.6-sol",
			ReasoningEffort:                 runtime.ReasoningEffortMedium,
			ReasoningContext:                runtime.ReasoningContextCurrentTurn,
			RequestTimeoutMillis:            120000,
			DeveloperInstructionRevision:    strings.Repeat("d", 64),
			AccountType:                     runtime.AccountTypeChatGPT,
			AccountPlan:                     "pro",
			SelectedAdapterID:               runtime.AdapterCodexSubscription,
			SelectedAdapterRevision:         strings.Repeat("e", 64),
			SelectedCodexVersion:            "codex-cli 0.147.0-alpha.6.5",
			SelectedCodexExecutableRevision: strings.Repeat("a", 64),
			SelectedCodexSchemaRevision:     strings.Repeat("b", 64),
			ThreadMode:                      runtime.ThreadModeEphemeral,
			SandboxMode:                     runtime.SandboxModeReadOnly,
			ApprovalPolicy:                  runtime.ApprovalPolicyNever,
			WorkdirMode:                     runtime.WorkdirModeEmptyPerTarget,
			DynamicToolsMode:                runtime.ToolsModeNone,
			MCPMode:                         runtime.ToolsModeNone,
			CommandPolicy:                   runtime.ResourcePolicyDenyAndFail,
			FileReadPolicy:                  runtime.ResourcePolicyDenyAndFail,
			IsolationRevision:               strings.Repeat("f", 64),
		},
	}
}

func TestMuxRoutesOnlyExactAuthorityProviderCombinations(t *testing.T) {
	tests := []struct {
		name             string
		spec             runtime.RunSpec
		wantLegacyCalls  int
		wantPrimaryCalls int
		wantErr          bool
	}{
		{name: "legacy", spec: runtime.RunSpec{RunID: "legacy", Agent: "codex"}, wantLegacyCalls: 1},
		{name: "subscription", spec: subscriptionSpec(), wantPrimaryCalls: 1},
		{name: "empty authority subscription agent", spec: runtime.RunSpec{RunID: "bad", Agent: "codex-subscription"}, wantErr: true},
		{name: "empty authority subscription profile", spec: runtime.RunSpec{RunID: "bad", Agent: "codex", Profile: "codex-subscription:gpt-5.6-sol"}, wantErr: true},
		{name: "subscription authority legacy agent", spec: func() runtime.RunSpec { v := subscriptionSpec(); v.Agent = "codex"; return v }(), wantErr: true},
		{name: "unknown authority", spec: func() runtime.RunSpec { v := subscriptionSpec(); v.Authority = "future"; return v }(), wantErr: true},
		{name: "legacy with subscription contract", spec: runtime.RunSpec{RunID: "bad", Agent: "codex", RuntimeContract: runtime.RuntimeContractCodexSubscriptionExecV1}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			legacy := &countingRuntime{name: "legacy"}
			primary := &countingRuntime{name: "primary"}
			mux := runtimemux.New(legacy, primary)
			_, err := mux.Dispatch(context.Background(), test.spec)
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, test.wantErr)
			}
			if legacy.calls != test.wantLegacyCalls || primary.calls != test.wantPrimaryCalls {
				t.Fatalf("dispatch calls legacy=%d primary=%d", legacy.calls, primary.calls)
			}
		})
	}
}

func TestMuxFailsClosedWhenSelectedRuntimeIsMissing(t *testing.T) {
	legacy := &countingRuntime{name: "legacy"}
	if _, err := runtimemux.New(legacy, nil).Dispatch(context.Background(), subscriptionSpec()); err == nil {
		t.Fatal("missing subscription runtime accepted")
	}
	if legacy.calls != 0 {
		t.Fatal("subscription request crossed into legacy runtime")
	}

	primary := &countingRuntime{name: "primary"}
	if _, err := runtimemux.New(nil, primary).Dispatch(context.Background(), runtime.RunSpec{Agent: "codex"}); err == nil {
		t.Fatal("missing legacy runtime accepted")
	}
	if primary.calls != 0 {
		t.Fatal("legacy request crossed into subscription runtime")
	}
}

func TestMuxMissingSubscriptionRuntimeReturnsClosedRecoveryCode(t *testing.T) {
	legacy := &countingRuntime{name: "legacy"}
	_, err := runtimemux.New(legacy, nil).Dispatch(context.Background(), subscriptionSpec())
	if err == nil || !strings.Contains(err.Error(), runtime.ErrorChatPolicyUnavailable) {
		t.Fatalf("error = %v, want %q", err, runtime.ErrorChatPolicyUnavailable)
	}
	if legacy.calls != 0 {
		t.Fatal("subscription request crossed into legacy runtime")
	}
}
