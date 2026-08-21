package store

import (
	"reflect"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
)

func TestPrimaryAgentChannelMigrationIsPreviewedDeterministicAndIdempotent(t *testing.T) {
	t.Parallel()

	s := openTemp(t)
	now := time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC)
	setting := primarySetting(now)
	if err := s.UpsertPrimaryAgentSetting(setting); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(conversation.Conversation{ID: "legacy-a", Title: "Alpha", CreatedAt: now}, "participant-a"); err != nil {
		t.Fatal(err)
	}
	pinnedAt := now.Add(10 * time.Minute)
	if err := s.SetPrimaryChannelPinned("legacy-a", true, pinnedAt); err != nil {
		t.Fatal(err)
	}

	sameIdentity := setting
	sameIdentity.Seat.DisplayName = "Renamed presentation"
	sameIdentity.UpdatedAt = now.Add(time.Minute)
	if err := s.UpsertPrimaryAgentSetting(sameIdentity); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(conversation.Conversation{ID: "legacy-b", Title: "Beta", CreatedAt: now.Add(time.Minute)}, "participant-b"); err != nil {
		t.Fatal(err)
	}

	differentPolicy := sameIdentity
	differentPolicy.Policy.PolicyRevision = "policy-revision-v2"
	differentPolicy.UpdatedAt = now.Add(2 * time.Minute)
	if err := s.UpsertPrimaryAgentSetting(differentPolicy); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(conversation.Conversation{ID: "legacy-c", Title: "Gamma", CreatedAt: now.Add(2 * time.Minute)}, "participant-c"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateConversation(conversation.Conversation{ID: "legacy-shared", Title: "Shared", CreatedAt: now}, []conversation.Participant{
		{ID: "shared-a", ConversationID: "legacy-shared", SeatID: "seat-a", Profile: "codex:a", Agent: "codex", Machine: "studio", DisplayName: "A", CreatedAt: now},
		{ID: "shared-b", ConversationID: "legacy-shared", SeatID: "seat-b", Profile: "claude:b", Agent: "claude", Machine: "mini", DisplayName: "B", Position: 1, CreatedAt: now},
	}); err != nil {
		t.Fatal(err)
	}

	legacyBefore, err := s.ListPrimaryChannels("all")
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewPrimaryAgentChannelMigration()
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	secondPreview, err := s.PreviewPrimaryAgentChannelMigration()
	if err != nil || !reflect.DeepEqual(secondPreview, preview) {
		t.Fatalf("preview changed: first=%+v second=%+v err=%v", preview, secondPreview, err)
	}
	if len(preview.Channels) != 2 || len(preview.Conversations) != 3 || len(preview.Pins) != 1 || len(preview.Skipped) != 0 {
		t.Fatalf("preview counts = channels:%d conversations:%d pins:%d skipped:%d", len(preview.Channels), len(preview.Conversations), len(preview.Pins), len(preview.Skipped))
	}
	links := map[string]string{}
	for _, link := range preview.Conversations {
		links[link.ConversationID] = link.AgentChannelID
	}
	if links["legacy-a"] == "" || links["legacy-a"] != links["legacy-b"] || links["legacy-c"] == links["legacy-a"] {
		t.Fatalf("migration grouping = %+v", links)
	}
	if preview.Pins[0].ConversationID != "legacy-a" || !preview.Pins[0].PinnedAt.Equal(pinnedAt) {
		t.Fatalf("migrated pin = %+v", preview.Pins[0])
	}
	if channels, err := s.ListAgentChannels("all"); err != nil || len(channels) != 0 {
		t.Fatalf("preview wrote Agent Channels: %+v, %v", channels, err)
	}

	applied, err := s.MigratePrimaryAgentChannels()
	if err != nil || !reflect.DeepEqual(applied, preview) {
		t.Fatalf("apply = %+v, %v; preview=%+v", applied, err, preview)
	}
	restartPreview, err := s.PreviewPrimaryAgentChannelMigration()
	if err != nil || len(restartPreview.Pins) != 0 || !reflect.DeepEqual(restartPreview.Channels, preview.Channels) || !reflect.DeepEqual(restartPreview.Conversations, preview.Conversations) {
		t.Fatalf("restart preview = %+v, %v", restartPreview, err)
	}
	if reapplied, err := s.MigratePrimaryAgentChannels(); err != nil || !reflect.DeepEqual(reapplied, restartPreview) {
		t.Fatalf("reapply = %+v, %v; preview=%+v", reapplied, err, restartPreview)
	}
	channels, err := s.ListAgentChannels("all")
	if err != nil || len(channels) != 2 {
		t.Fatalf("migrated Channels = %+v, %v", channels, err)
	}
	for conversationID, channelID := range links {
		if owned, err := s.AgentConversationOwned(channelID, conversationID); err != nil || !owned {
			t.Fatalf("ownership %s -> %s = %v, %v", conversationID, channelID, owned, err)
		}
	}
	if owned, err := s.AgentConversationOwned(links["legacy-a"], "legacy-shared"); err != nil || owned {
		t.Fatalf("legacy shared conversation was assigned: %v, %v", owned, err)
	}
	legacyAfter, err := s.ListPrimaryChannels("all")
	if err != nil || !reflect.DeepEqual(legacyAfter, legacyBefore) {
		t.Fatalf("legacy Primary rows changed:\nbefore=%+v\nafter=%+v\nerr=%v", legacyBefore, legacyAfter, err)
	}
}

func TestPrimaryAgentChannelMigrationProjectsLegacyPinOnlyWhenOwnershipIsFirstLinked(t *testing.T) {
	t.Parallel()

	s := openTemp(t)
	now := time.Date(2026, 8, 20, 17, 0, 0, 0, time.UTC)
	if err := s.UpsertPrimaryAgentSetting(primarySetting(now)); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(conversation.Conversation{ID: "legacy-pin", Title: "Pinned", CreatedAt: now}, "participant-pin"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPrimaryChannelPinned("legacy-pin", true, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	first, err := s.MigratePrimaryAgentChannels()
	if err != nil || len(first.Pins) != 1 || len(first.Conversations) != 1 {
		t.Fatalf("first migration = %+v, %v", first, err)
	}
	channelID := first.Conversations[0].AgentChannelID
	if err := s.SetAgentConversationPinned(channelID, "legacy-pin", false, time.Time{}); err != nil {
		t.Fatal(err)
	}

	second, err := s.MigratePrimaryAgentChannels()
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Pins) != 0 {
		t.Fatalf("restart migration reprojected legacy pin after Agent unpin: %+v", second.Pins)
	}
	detail, err := s.GetAgentChannel(channelID)
	if err != nil || len(detail.Conversations) != 1 || detail.Conversations[0].Pinned {
		t.Fatalf("Agent unpin was not restart-stable: %+v, %v", detail.Conversations, err)
	}
}
