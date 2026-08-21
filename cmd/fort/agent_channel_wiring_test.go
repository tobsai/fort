package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/ui"
)

func TestWireChannelProductsRejectsAgentCutoverWithoutPrimaryRollbackSurface(t *testing.T) {
	st := openChannelProductStore(t)
	deps := ui.Deps{}
	mode := ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsOff,
		AgentChannels:   ui.AgentChannelsPrimary,
	}

	products, err := wireChannelProducts(&deps, st, nil, nil, mode)
	if err == nil || !strings.Contains(err.Error(), "rollback") {
		t.Fatalf("Agent cutover without rollback error = %v", err)
	}
	if products.Primary != nil || deps.Primary != nil || deps.AgentChannels != nil {
		t.Fatalf("rejected cutover published services: products=%+v deps=%+v", products, deps)
	}
}

func TestWireChannelProductsMigratesLegacyPrimaryRowsBeforePublishingAgentPort(t *testing.T) {
	st := openChannelProductStore(t)
	now := time.Date(2026, 8, 20, 18, 0, 0, 0, time.UTC)
	if err := st.UpsertPrimaryAgentSetting(legacyPrimarySetting(now)); err != nil {
		t.Fatal(err)
	}
	if err := st.CreatePrimaryChannel(conversation.Conversation{
		ID: "legacy-channel", Title: "Legacy chat", CreatedAt: now, UpdatedAt: now,
	}, "legacy-participant"); err != nil {
		t.Fatal(err)
	}

	deps := ui.Deps{}
	previewedBeforeApply := false
	products, err := wireChannelProducts(&deps, st, nil, nil, ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsPreview,
		AgentChannels:   ui.AgentChannelsPrimary,
	}, func(report store.AgentChannelMigrationReport) error {
		persisted, listErr := st.ListAgentChannels("all")
		if listErr != nil {
			return listErr
		}
		if len(persisted) != 0 {
			return errors.New("migration applied before its preview was published")
		}
		previewedBeforeApply = len(report.Channels) == 1 && len(report.Conversations) == 1
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(products.Close)

	if products.Migration == nil || len(products.Migration.Conversations) != 1 {
		t.Fatalf("migration report = %+v", products.Migration)
	}
	if !previewedBeforeApply {
		t.Fatal("migration preview was not published before cutover")
	}
	channels, err := st.ListAgentChannels("all")
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || len(channels[0].Conversations) != 1 || channels[0].Conversations[0].Conversation.ID != "legacy-channel" {
		t.Fatalf("migrated Agent Channels = %+v", channels)
	}
	if deps.AgentChannels == nil {
		t.Fatal("Agent Channel port was not published after successful migration")
	}
}

func TestWireChannelProductsAcceptsMigrationThatConvergedAfterPublishedPreview(t *testing.T) {
	st := openChannelProductStore(t)
	now := time.Date(2026, 8, 20, 18, 30, 0, 0, time.UTC)
	if err := st.UpsertPrimaryAgentSetting(legacyPrimarySetting(now)); err != nil {
		t.Fatal(err)
	}
	if err := st.CreatePrimaryChannel(conversation.Conversation{
		ID: "concurrent-legacy", Title: "Concurrent legacy chat", CreatedAt: now, UpdatedAt: now,
	}, "concurrent-participant"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetPrimaryChannelPinned("concurrent-legacy", true, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	deps := ui.Deps{}
	products, err := wireChannelProducts(&deps, st, nil, nil, ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsPreview,
		AgentChannels:   ui.AgentChannelsPrimary,
	}, func(preview store.AgentChannelMigrationReport) error {
		if len(preview.Pins) != 1 {
			return errors.New("published preview did not include legacy pin")
		}
		_, migrateErr := st.MigratePrimaryAgentChannels()
		return migrateErr
	})
	if err != nil {
		t.Fatalf("concurrently converged migration was rejected: %v", err)
	}
	t.Cleanup(products.Close)
	if products.AgentChannels == nil || deps.AgentChannels == nil || products.Migration == nil {
		t.Fatalf("converged cutover did not publish Agent Channels: products=%+v deps=%+v", products, deps)
	}
	if len(products.Migration.Pins) != 0 {
		t.Fatalf("second migration reprojected pins: %+v", products.Migration.Pins)
	}
}

func TestWireChannelProductsLeavesCompatibilityDefaultUnchanged(t *testing.T) {
	st := openChannelProductStore(t)
	deps := ui.Deps{}
	products, err := wireChannelProducts(&deps, st, nil, nil, ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsOff,
		AgentChannels:   ui.AgentChannelsOff,
	})
	if err != nil {
		t.Fatal(err)
	}
	if products.Primary != nil || products.AgentChannels != nil || products.Migration != nil || deps.Primary != nil || deps.AgentChannels != nil {
		t.Fatalf("off/default wiring changed product services: products=%+v deps=%+v", products, deps)
	}
}

func TestWireChannelProductsPreservesPrimaryOnlyComposition(t *testing.T) {
	st := openChannelProductStore(t)
	deps := ui.Deps{}
	products, err := wireChannelProducts(&deps, st, nil, nil, ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsPreview,
		AgentChannels:   ui.AgentChannelsOff,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(products.Close)
	if products.Primary == nil || deps.Primary != products.Primary {
		t.Fatal("Primary-only mode did not publish its established product port")
	}
	if products.AgentChannels != nil || products.Migration != nil || deps.AgentChannels != nil {
		t.Fatalf("Primary-only mode unexpectedly enabled Agent Channels: products=%+v", products)
	}
}

func openChannelProductStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func legacyPrimarySetting(now time.Time) conversation.PrimaryAgentSetting {
	return conversation.PrimaryAgentSetting{
		OptionID: "primary-option:v1:legacy",
		Seat: conversation.Seat{
			ID: "seat:v1:legacy", Profile: "codex-subscription:gpt-5.6-sol",
			Agent: "codex-subscription", Model: "gpt-5.6-sol", Machine: "studio",
			DisplayName: "Codex on Studio",
		},
		Authority: conversation.AuthorityChatSubscriptionIsolatedV1,
		Policy: conversation.SubscriptionPolicy{
			PolicyID: conversation.PolicyCodexSubscriptionChatV1, PolicyRevision: "policy-revision-v1",
			AdapterID: conversation.AdapterCodexSubscription, AdapterRevision: "codex-exec-adapter-v1",
			CodexVersion: "0.120.0", CodexExecutableRevision: strings.Repeat("a", 64),
			CodexSchemaRevision: strings.Repeat("b", 64), RuntimeContract: conversation.RuntimeContractCodexSubscriptionExecV1,
			ReasoningEffort: "medium", ReasoningContext: "current_turn", RequestTimeoutMillis: 120_000,
			DeveloperInstructionRevision: "developer-instruction-v1", AccountType: conversation.AccountTypeChatGPT,
			AccountPlan: "plus", ThreadMode: conversation.ThreadModeEphemeral, SandboxMode: conversation.SandboxModeReadOnly,
			ApprovalPolicy: conversation.ApprovalPolicyNever, WorkdirMode: conversation.WorkdirModeEmptyPerTarget,
			DynamicToolsMode: conversation.ToolsModeNone, MCPMode: conversation.ToolsModeNone,
			CommandPolicy: conversation.ResourcePolicyDenyAndFail, FileReadPolicy: conversation.ResourcePolicyDenyAndFail,
			IsolationRevision: "isolation-v1",
		},
		UpdatedAt: now,
	}
}
