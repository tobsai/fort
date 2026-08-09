package capability

import (
	"strings"
	"testing"
)

func validTextOnlyOptionOffer() TextOnlyOptionOffer {
	machine := "studio.local"
	profile := "codex-subscription:gpt-5.6-sol"
	model := "gpt-5.6-sol"
	return TextOnlyOptionOffer{
		OfferVersion: 1, MachineID: machine,
		SeatID:   TextOnlySeatID(profile, machine, model),
		AgentKey: "codex-subscription", ProfileID: profile,
		RequestedModel: model, ResolvedModel: "unknown",
		AccountType: "chatgpt", AccountPlan: "pro",
		PolicyID: "codex-subscription-chat-v1", PolicyRevision: strings.Repeat("a", 64),
		RuntimeContract: "codex_subscription_exec_v1",
		ReasoningEffort: "medium", ReasoningContext: "current_turn",
		RequestTimeoutMillis: 120_000, DeveloperInstructionRevision: strings.Repeat("b", 64),
		AdapterID: "model.chat.text-only.codex-subscription", AdapterRevision: strings.Repeat("c", 64),
		CodexVersion:            "codex-cli 0.147.0-alpha.6.5",
		CodexExecutableRevision: strings.Repeat("d", 64), CodexSchemaRevision: strings.Repeat("e", 64),
		ThreadMode: "ephemeral", SandboxMode: "readOnly", ApprovalPolicy: "never",
		WorkdirMode: "empty_per_target", DynamicToolsMode: "none", MCPMode: "none",
		CommandPolicy: "deny_and_fail", FileReadPolicy: "deny_and_fail",
		IsolationRevision: strings.Repeat("f", 64),
	}
}

func TestTextOnlyOptionOfferNormalizesToStableServerComputedID(t *testing.T) {
	offer := validTextOnlyOptionOffer()
	normalized, id, err := NormalizeTextOnlyOptionOffer(offer, offer.MachineID)
	if err != nil {
		t.Fatal(err)
	}
	if normalized != offer || !strings.HasPrefix(id, "primary-option:v1:") || len(id) != len("primary-option:v1:")+64 {
		t.Fatalf("normalized=%+v id=%q", normalized, id)
	}
	offer.AccountPlan = "plus"
	_, changedID, err := NormalizeTextOnlyOptionOffer(offer, offer.MachineID)
	if err != nil {
		t.Fatal(err)
	}
	if changedID == id {
		t.Fatal("account plan change retained option id")
	}
}

func TestTextOnlyOptionOfferRejectsIncompleteOrCrossMachineAuthority(t *testing.T) {
	valid := validTextOnlyOptionOffer()
	tests := []struct {
		name   string
		mutate func(*TextOnlyOptionOffer)
	}{
		{"offer version", func(v *TextOnlyOptionOffer) { v.OfferVersion = 2 }},
		{"machine", func(v *TextOnlyOptionOffer) { v.MachineID = "other" }},
		{"seat", func(v *TextOnlyOptionOffer) { v.SeatID = "seat:v1:caller" }},
		{"agent", func(v *TextOnlyOptionOffer) { v.AgentKey = "codex" }},
		{"profile", func(v *TextOnlyOptionOffer) { v.ProfileID = "codex:gpt-5.6-sol" }},
		{"model", func(v *TextOnlyOptionOffer) { v.RequestedModel = "gpt-5.6-terra" }},
		{"resolved model", func(v *TextOnlyOptionOffer) { v.ResolvedModel = "gpt-5.6-sol" }},
		{"account type", func(v *TextOnlyOptionOffer) { v.AccountType = "apiKey" }},
		{"unknown plan", func(v *TextOnlyOptionOffer) { v.AccountPlan = "mystery" }},
		{"policy", func(v *TextOnlyOptionOffer) { v.PolicyID = "other" }},
		{"policy revision", func(v *TextOnlyOptionOffer) { v.PolicyRevision = strings.Repeat("A", 64) }},
		{"runtime", func(v *TextOnlyOptionOffer) { v.RuntimeContract = "native-cli" }},
		{"reasoning", func(v *TextOnlyOptionOffer) { v.ReasoningEffort = "high" }},
		{"timeout", func(v *TextOnlyOptionOffer) { v.RequestTimeoutMillis = 1 }},
		{"adapter", func(v *TextOnlyOptionOffer) { v.AdapterID = "profile.codex.native" }},
		{"codex version", func(v *TextOnlyOptionOffer) { v.CodexVersion = "" }},
		{"executable revision", func(v *TextOnlyOptionOffer) { v.CodexExecutableRevision = "short" }},
		{"schema revision", func(v *TextOnlyOptionOffer) { v.CodexSchemaRevision = "short" }},
		{"thread", func(v *TextOnlyOptionOffer) { v.ThreadMode = "resume" }},
		{"sandbox", func(v *TextOnlyOptionOffer) { v.SandboxMode = "workspaceWrite" }},
		{"approval", func(v *TextOnlyOptionOffer) { v.ApprovalPolicy = "on-request" }},
		{"workdir", func(v *TextOnlyOptionOffer) { v.WorkdirMode = "repository" }},
		{"tools", func(v *TextOnlyOptionOffer) { v.DynamicToolsMode = "available" }},
		{"mcp", func(v *TextOnlyOptionOffer) { v.MCPMode = "configured" }},
		{"command", func(v *TextOnlyOptionOffer) { v.CommandPolicy = "allow" }},
		{"file read", func(v *TextOnlyOptionOffer) { v.FileReadPolicy = "allow" }},
		{"isolation", func(v *TextOnlyOptionOffer) { v.IsolationRevision = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if _, _, err := NormalizeTextOnlyOptionOffer(candidate, valid.MachineID); err == nil {
				t.Fatal("invalid offer accepted")
			}
		})
	}
}

func TestSnapshotNormalizesBoundedTextOnlyOptionsAndRejectsDuplicates(t *testing.T) {
	snapshot := readySnapshot()
	offer := validTextOnlyOptionOffer()
	offer.MachineID = snapshot.Machines[0].Name
	offer.SeatID = TextOnlySeatID(offer.ProfileID, offer.MachineID, offer.RequestedModel)
	snapshot.Machines[0].TextOnlyOptions = []TextOnlyOptionOffer{offer}
	normalized, err := NormalizeSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(normalized.Machines[0].TextOnlyOptions) != 1 || normalized.Machines[0].TextOnlyOptions[0] != offer {
		t.Fatalf("options = %#v", normalized.Machines[0].TextOnlyOptions)
	}

	snapshot.Machines[0].TextOnlyOptions = append(snapshot.Machines[0].TextOnlyOptions, offer)
	if _, err := NormalizeSnapshot(snapshot); err == nil {
		t.Fatal("duplicate text-only option accepted")
	}
}

func TestSnapshotRequiresNonNullTextOnlyOptionVector(t *testing.T) {
	snapshot := readySnapshot()
	snapshot.Machines[0].TextOnlyOptions = nil
	if _, err := NormalizeSnapshot(snapshot); err == nil {
		t.Fatal("nil text-only option vector accepted")
	}
}
