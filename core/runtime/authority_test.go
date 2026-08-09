package runtime_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tobsai/fort/core/runtime"
)

func validSubscriptionSpec() runtime.RunSpec {
	return runtime.RunSpec{
		RunID:           "run-1",
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

func TestSubscriptionRunSpecWireCarriesAuthorityButNeverPrivateExpectedRevision(t *testing.T) {
	spec := validSubscriptionSpec()
	spec.ExpectedPolicyRevision = "private-hub-preflight"
	wire, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(wire, []byte(`"profile":"codex-subscription:gpt-5.6-sol"`)) ||
		!bytes.Contains(wire, []byte(`"text_only_policy"`)) || bytes.Contains(wire, []byte("private-hub-preflight")) {
		t.Fatalf("subscription wire = %s", wire)
	}
	var decoded runtime.RunSpec
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Profile != spec.Profile || decoded.Authority != spec.Authority || decoded.TextOnlyPolicy == nil ||
		decoded.ExpectedPolicyRevision != "" {
		t.Fatalf("decoded wire = %#v", decoded)
	}
	if err := decoded.ValidateAuthority(); err != nil {
		t.Fatalf("remote authority no longer validates: %v", err)
	}
}

func TestLegacyRunSpecWireDoesNotCarryControlPlaneProfile(t *testing.T) {
	wire, err := json.Marshal(runtime.RunSpec{
		RunID: "legacy", Agent: "codex", Profile: "codex:gpt-5.6-sol",
	})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(wire, []byte(`"profile"`)) {
		t.Fatalf("legacy profile crossed execution wire: %s", wire)
	}

	var decoded runtime.RunSpec
	if err := json.Unmarshal([]byte(`{"RunID":"legacy","Agent":"codex","profile":"codex:gpt-5.6-sol"}`), &decoded); err == nil {
		t.Fatal("legacy execution wire accepted a control-plane profile")
	}
}

func TestRunSpecWireRejectsUnknownAndTrailingValues(t *testing.T) {
	spec := validSubscriptionSpec()
	wire, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	unknown := append(append([]byte(nil), wire[:len(wire)-1]...), []byte(`,"private_expected_policy":"do-not-cross"}`)...)
	var decoded runtime.RunSpec
	if err := json.Unmarshal(unknown, &decoded); err == nil {
		t.Fatal("subscription execution wire accepted an unknown field")
	}
	if err := json.Unmarshal(append(wire, []byte(" {}")...), &decoded); err == nil {
		t.Fatal("subscription execution wire accepted a trailing JSON value")
	}
}

func TestRunSpecWireAcceptsEstablishedRunIDAliasButRejectsConflict(t *testing.T) {
	var decoded runtime.RunSpec
	if err := json.Unmarshal([]byte(`{"run_id":"legacy-run","agent":"codex"}`), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.RunID != "legacy-run" || decoded.Agent != "codex" {
		t.Fatalf("decoded alias = %#v", decoded)
	}
	if err := json.Unmarshal([]byte(`{"RunID":"one","run_id":"two","Agent":"codex"}`), &decoded); err == nil {
		t.Fatal("conflicting run id aliases were accepted")
	}
}

func TestRunSpecAuthorityValidation(t *testing.T) {
	valid := validSubscriptionSpec()
	if err := valid.ValidateAuthority(); err != nil {
		t.Fatalf("valid subscription authority: %v", err)
	}

	legacy := runtime.RunSpec{RunID: "legacy", Agent: "codex"}
	if err := legacy.ValidateAuthority(); err != nil {
		t.Fatalf("valid legacy authority: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*runtime.RunSpec)
	}{
		{"unknown authority", func(v *runtime.RunSpec) { v.Authority = "unknown" }},
		{"wrong runtime", func(v *runtime.RunSpec) { v.RuntimeContract = "other" }},
		{"wrong agent", func(v *runtime.RunSpec) { v.Agent = "codex" }},
		{"wrong profile", func(v *runtime.RunSpec) { v.Profile = "codex:gpt-5.6-sol" }},
		{"model drift", func(v *runtime.RunSpec) { v.Model = "gpt-5.6-terra" }},
		{"missing policy", func(v *runtime.RunSpec) { v.TextOnlyPolicy = nil }},
		{"policy id", func(v *runtime.RunSpec) { v.TextOnlyPolicy.PolicyID = "other" }},
		{"policy revision", func(v *runtime.RunSpec) { v.TextOnlyPolicy.PolicyRevision = "other" }},
		{"reasoning effort", func(v *runtime.RunSpec) { v.TextOnlyPolicy.ReasoningEffort = "high" }},
		{"timeout", func(v *runtime.RunSpec) { v.TextOnlyPolicy.RequestTimeoutMillis = 1 }},
		{"developer revision", func(v *runtime.RunSpec) { v.TextOnlyPolicy.DeveloperInstructionRevision = "other" }},
		{"account type", func(v *runtime.RunSpec) { v.TextOnlyPolicy.AccountType = "apiKey" }},
		{"empty plan", func(v *runtime.RunSpec) { v.TextOnlyPolicy.AccountPlan = "" }},
		{"unknown plan", func(v *runtime.RunSpec) { v.TextOnlyPolicy.AccountPlan = "future-secret-plan" }},
		{"adapter revision", func(v *runtime.RunSpec) { v.TextOnlyPolicy.SelectedAdapterRevision = "other" }},
		{"executable revision", func(v *runtime.RunSpec) { v.TextOnlyPolicy.SelectedCodexExecutableRevision = strings.Repeat("A", 64) }},
		{"schema revision", func(v *runtime.RunSpec) { v.TextOnlyPolicy.SelectedCodexSchemaRevision = "short" }},
		{"sandbox", func(v *runtime.RunSpec) { v.TextOnlyPolicy.SandboxMode = "workspaceWrite" }},
		{"approval", func(v *runtime.RunSpec) { v.TextOnlyPolicy.ApprovalPolicy = "on-request" }},
		{"isolation revision", func(v *runtime.RunSpec) { v.TextOnlyPolicy.IsolationRevision = "other" }},
		{"workdir", func(v *runtime.RunSpec) { v.Workdir = "/tmp/user" }},
		{"env", func(v *runtime.RunSpec) { v.Env = []string{"SECRET=x"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := validSubscriptionSpec()
			policyCopy := *candidate.TextOnlyPolicy
			candidate.TextOnlyPolicy = &policyCopy
			test.mutate(&candidate)
			if err := candidate.ValidateAuthority(); err == nil {
				t.Fatal("expected validation failure")
			}
		})
	}
}

func TestLegacyAuthorityRejectsSubscriptionFields(t *testing.T) {
	for _, mutate := range []func(*runtime.RunSpec){
		func(v *runtime.RunSpec) { v.RuntimeContract = runtime.RuntimeContractCodexSubscriptionExecV1 },
		func(v *runtime.RunSpec) { v.TextOnlyPolicy = validSubscriptionSpec().TextOnlyPolicy },
		func(v *runtime.RunSpec) { v.ExpectedPolicyRevision = "private" },
	} {
		spec := runtime.RunSpec{RunID: "legacy", Agent: "codex"}
		mutate(&spec)
		if err := spec.ValidateAuthority(); err == nil {
			t.Fatal("legacy authority accepted subscription-only fields")
		}
	}
}

func TestResponseMetadataValidation(t *testing.T) {
	metadata := runtime.ResponseMetadata{
		ProviderThreadID:                "thread-1",
		RequestedModel:                  "gpt-5.6-sol",
		ResolvedModel:                   runtime.UnknownProviderIdentity,
		SelectedAdapterID:               runtime.AdapterCodexSubscription,
		SelectedAdapterRevision:         strings.Repeat("c", 64),
		SelectedCodexVersion:            "codex-cli 0.147.0-alpha.6.5",
		SelectedCodexExecutableRevision: strings.Repeat("a", 64),
		SelectedCodexSchemaRevision:     strings.Repeat("b", 64),
		ObservedAdapterID:               runtime.AdapterCodexSubscription,
		ObservedAdapterRevision:         strings.Repeat("c", 64),
		ObservedCodexVersion:            "codex-cli 0.147.0-alpha.6.5",
		ObservedCodexExecutableRevision: strings.Repeat("a", 64),
		ObservedCodexSchemaRevision:     strings.Repeat("b", 64),
		TerminalStatus:                  "completed",
		UsageSource:                     "codex_exec_jsonl",
		Usage:                           runtime.ProviderUsage{InputTokens: 3, CachedInputTokens: 1, OutputTokens: 2},
	}
	if err := metadata.Validate(); err != nil {
		t.Fatalf("valid response metadata: %v", err)
	}
	metadata.Usage.OutputTokens = -1
	if err := metadata.Validate(); err == nil {
		t.Fatal("negative usage accepted")
	}
}
