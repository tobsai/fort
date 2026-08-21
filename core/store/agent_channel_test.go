package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
)

func TestAgentChannelSchemaInstallsConcurrentlyWithoutDroppingInvariant(t *testing.T) {
	if strings.Contains(agentChannelSchema, "DROP TRIGGER") {
		t.Fatal("Agent Channel schema drops a live invariant during concurrent Store.Open")
	}

	path := filepath.Join(t.TempDir(), "fort.db")
	const count = 6
	start := make(chan struct{})
	results := make(chan struct {
		store *Store
		err   error
	}, count)
	var wait sync.WaitGroup
	for range count {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			opened, err := Open(path)
			results <- struct {
				store *Store
				err   error
			}{store: opened, err: err}
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	for result := range results {
		if result.err != nil {
			t.Errorf("concurrent Store.Open: %v", result.err)
			continue
		}
		if err := result.store.Close(); err != nil {
			t.Errorf("close concurrent Store: %v", err)
		}
	}
}

func TestCreateAgentChannelRejectsInvalidIdentityWithoutPersisting(t *testing.T) {
	t.Parallel()

	s := openTemp(t)
	now := time.Date(2026, 8, 20, 11, 0, 0, 0, time.UTC)
	binding := agentBindingFromPrimarySetting(primarySetting(now))
	channel := conversation.AgentChannel{
		ID: "agent-channel:v1:not-the-binding", Name: "Codex", State: conversation.AgentChannelOpen,
		OptionID: "agent-option:v1:codex", Binding: binding, CreatedAt: now,
	}
	if err := s.CreateAgentChannel(channel); err == nil {
		t.Fatal("Agent Channel with mismatched identity was persisted")
	}
	if _, err := s.GetAgentChannel(channel.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("invalid Channel lookup error = %v, want sql.ErrNoRows", err)
	}
}

func TestAgentChannelOwnsMultipleConversations(t *testing.T) {
	t.Parallel()

	s := openTemp(t)
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	setting := primarySetting(now)
	binding := agentBindingFromPrimarySetting(setting)
	channelID, err := conversation.AgentChannelID(binding)
	if err != nil {
		t.Fatalf("derive Agent Channel ID: %v", err)
	}
	channel := conversation.AgentChannel{
		ID:        channelID,
		Name:      "Codex",
		State:     conversation.AgentChannelOpen,
		OptionID:  setting.OptionID,
		Binding:   binding,
		CreatedAt: now,
	}
	if err := s.CreateAgentChannel(channel); err != nil {
		t.Fatalf("create Agent Channel: %v", err)
	}

	for index, item := range []struct {
		id    string
		title string
	}{
		{id: "conversation-1", title: "First"},
		{id: "conversation-2", title: "Second"},
	} {
		createdAt := now.Add(time.Duration(index+1) * time.Minute)
		if err := s.CreateAgentChannelConversation(channel.ID, conversation.Conversation{
			ID:        item.id,
			Title:     item.title,
			State:     conversation.ConversationOpen,
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		}, "participant-"+item.id); err != nil {
			t.Fatalf("create %s: %v", item.id, err)
		}
	}

	detail, err := s.GetAgentChannel(channel.ID)
	if err != nil {
		t.Fatalf("get Agent Channel: %v", err)
	}
	if !reflect.DeepEqual(detail.Channel, channel) {
		t.Fatalf("Channel = %#v, want %#v", detail.Channel, channel)
	}
	if got := agentConversationIDs(detail.Conversations); !reflect.DeepEqual(got, []string{"conversation-2", "conversation-1"}) {
		t.Fatalf("Conversations = %v, want newest first", got)
	}
	for _, child := range detail.Conversations {
		if child.Conversation.ID == channel.ID || child.Participant.SeatID != channel.Binding.Seat.ID ||
			child.Participant.Agent != channel.Binding.Seat.Agent || child.Participant.Model != channel.Binding.Seat.Model ||
			child.Participant.Machine != channel.Binding.Seat.Machine ||
			!reflect.DeepEqual(detail.Channel.Binding, channel.Binding) {
			t.Fatalf("child did not retain the exact parent identity: %+v", child)
		}
	}
}

func TestArchivedAgentChannelCannotCreateConversationAtStoreBoundary(t *testing.T) {
	t.Parallel()

	s := openTemp(t)
	now := time.Date(2026, 8, 20, 12, 30, 0, 0, time.UTC)
	setting := primarySetting(now)
	channel := createAgentChannelForTest(t, s, "Codex", setting.OptionID, agentBindingFromPrimarySetting(setting), now)
	if err := s.SetAgentChannelState(channel.ID, conversation.AgentChannelArchived); err != nil {
		t.Fatal(err)
	}
	child := conversation.Conversation{
		ID: "archived-parent-child", Title: "Must not exist", State: conversation.ConversationOpen,
		CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute),
	}

	if err := s.CreateAgentChannelConversation(channel.ID, child, "archived-parent-participant"); !errors.Is(err, ErrAgentChannelState) {
		t.Fatalf("archived Agent Channel child error = %v, want ErrAgentChannelState", err)
	}
	if _, err := s.GetConversation(child.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("rejected child creation left a Conversation: %v", err)
	}
}

func TestAgentConversationPinsAreOwnedIdempotentAndOrdered(t *testing.T) {
	t.Parallel()

	s := openTemp(t)
	now := time.Date(2026, 8, 20, 13, 0, 0, 0, time.UTC)
	setting := primarySetting(now)
	channel := createAgentChannelForTest(t, s, "Codex", setting.OptionID, agentBindingFromPrimarySetting(setting), now)
	for index, id := range []string{"a", "b"} {
		createdAt := now.Add(time.Duration(index+1) * time.Minute)
		if err := s.CreateAgentChannelConversation(channel.ID, conversation.Conversation{
			ID: id, Title: id, State: conversation.ConversationOpen, CreatedAt: createdAt, UpdatedAt: createdAt,
		}, "participant-"+id); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}

	foreignSetting := setting
	foreignSetting.OptionID = "primary-option:v1:mini"
	foreignSetting.Seat.ID = "seat:v1:mini"
	foreignSetting.Seat.Machine = "mini"
	foreignSetting.Seat.DisplayName = "Codex on Mini"
	foreign := createAgentChannelForTest(t, s, "Codex on Mini", foreignSetting.OptionID, agentBindingFromPrimarySetting(foreignSetting), now)
	if err := s.CreateAgentChannelConversation(foreign.ID, conversation.Conversation{
		ID: "foreign", Title: "Foreign", State: conversation.ConversationOpen,
		CreatedAt: now.Add(3 * time.Minute), UpdatedAt: now.Add(3 * time.Minute),
	}, "participant-foreign"); err != nil {
		t.Fatalf("create foreign conversation: %v", err)
	}

	before, err := s.GetConversation("b")
	if err != nil {
		t.Fatal(err)
	}
	pinnedAt := now.Add(10 * time.Minute)
	if err := s.SetAgentConversationPinned(channel.ID, "a", true, pinnedAt); err != nil {
		t.Fatalf("pin a: %v", err)
	}
	if err := s.SetAgentConversationPinned(channel.ID, "b", true, pinnedAt); err != nil {
		t.Fatalf("pin b: %v", err)
	}
	if err := s.SetAgentConversationPinned(channel.ID, "b", true, now.Add(30*time.Minute)); err != nil {
		t.Fatalf("repeat pin b: %v", err)
	}
	if err := s.SetAgentConversationPinned(foreign.ID, "a", true, pinnedAt); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("foreign parent pin error = %v, want sql.ErrNoRows", err)
	}

	detail, err := s.GetAgentChannel(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := agentConversationIDs(detail.Conversations); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("pinned ordering = %v, want stable ID tie-breaker", got)
	}
	for _, item := range detail.Conversations {
		if !item.Pinned || !item.PinnedAt.Equal(pinnedAt) {
			t.Fatalf("idempotent pin = %+v", item)
		}
	}
	after, err := s.GetConversation("b")
	if err != nil {
		t.Fatal(err)
	}
	if !after.Conversation.UpdatedAt.Equal(before.Conversation.UpdatedAt) {
		t.Fatalf("pin changed conversation activity: before=%s after=%s", before.Conversation.UpdatedAt, after.Conversation.UpdatedAt)
	}

	if err := s.SetAgentConversationPinned(channel.ID, "a", false, time.Time{}); err != nil {
		t.Fatalf("unpin a: %v", err)
	}
	detail, err = s.GetAgentChannel(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := agentConversationIDs(detail.Conversations); !reflect.DeepEqual(got, []string{"b", "a"}) || detail.Conversations[1].Pinned {
		t.Fatalf("unpin projection = %+v", detail.Conversations)
	}
}

func TestAgentChannelOwnershipRejectsSameSeatWithDifferentPrimaryAuthority(t *testing.T) {
	t.Parallel()
	s := openTemp(t)
	now := time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC)
	first := primarySetting(now)
	if err := s.UpsertPrimaryAgentSetting(first); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(conversation.Conversation{ID: "policy-one", Title: "One", CreatedAt: now, UpdatedAt: now}, "participant-one"); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Policy.PolicyRevision = "different-policy-revision"
	second.UpdatedAt = now.Add(time.Second)
	if err := s.UpsertPrimaryAgentSetting(second); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(conversation.Conversation{ID: "policy-two", Title: "Two", CreatedAt: now, UpdatedAt: now}, "participant-two"); err != nil {
		t.Fatal(err)
	}
	binding := agentBindingFromPrimarySetting(first)
	channelID, err := conversation.AgentChannelID(binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateAgentChannel(conversation.AgentChannel{
		ID: channelID, Name: "Exact policy one", State: conversation.AgentChannelOpen,
		OptionID: "option-one", Binding: binding, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO agent_channel_conversation(agent_channel_id,conversation_id,created_at) VALUES(?,?,?)`, channelID, "policy-two", nowOr(now)); err == nil {
		t.Fatal("same-seat Conversation with different Primary authority was linked to the wrong Agent Channel")
	}
}

func TestCreateAgentChannelConversationTurnIsAtomicAndIdempotent(t *testing.T) {
	t.Parallel()

	s := openTemp(t)
	now := time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)
	setting := primarySetting(now)
	channel := createAgentChannelForTest(t, s, "Codex", setting.OptionID, agentBindingFromPrimarySetting(setting), now)
	params := CreateAgentChannelConversationTurnParams{
		ChannelID: channel.ID,
		Conversation: conversation.Conversation{
			ID: "conversation-first", Title: "First send", State: conversation.ConversationOpen,
			CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute),
		},
		ParticipantID: "participant-first", TurnID: "turn-first", ClientTurnID: "client-first",
		TargetID: "target-first", RunID: "run-first", HumanID: "human", Body: "hello",
		Authority: targetAuthority(setting), CreatedAt: now.Add(time.Minute),
	}
	firstTurn, firstTargets, firstContext, err := s.CreateAgentChannelConversationTurn(params)
	if err != nil {
		t.Fatalf("first Send: %v", err)
	}
	if !firstTurn.Created || len(firstTargets) != 1 || firstTargets[0].State != conversation.TargetQueued || firstContext == "" {
		t.Fatalf("first Send result = turn=%+v targets=%+v context=%q", firstTurn, firstTargets, firstContext)
	}

	replay := params
	replay.TurnID, replay.TargetID, replay.RunID = "turn-replay", "target-replay", "run-replay"
	replayedTurn, replayedTargets, replayedContext, err := s.CreateAgentChannelConversationTurn(replay)
	if err != nil {
		t.Fatalf("replayed first Send: %v", err)
	}
	if replayedTurn.ID != firstTurn.ID || replayedTurn.Created || !reflect.DeepEqual(replayedTargets, firstTargets) || replayedContext != firstContext {
		t.Fatalf("replay = turn=%+v targets=%+v context=%q", replayedTurn, replayedTargets, replayedContext)
	}
	detail, err := s.GetConversation(params.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Messages) != 1 || len(detail.Turns) != 1 || len(detail.Targets) != 1 {
		t.Fatalf("idempotent durable rows = %+v", detail)
	}

	rejected := params
	rejected.Conversation.ID = "conversation-rejected"
	rejected.ParticipantID, rejected.TurnID, rejected.ClientTurnID = "participant-rejected", "turn-rejected", "client-rejected"
	rejected.TargetID, rejected.RunID = "target-rejected", "run-rejected"
	rejected.Authority = targetAuthority(setting)
	rejected.Authority.Policy.PolicyRevision = "drifted"
	if _, _, _, err := s.CreateAgentChannelConversationTurn(rejected); err == nil {
		t.Fatal("first Send with drifted authority succeeded")
	}
	if _, err := s.GetConversation(rejected.Conversation.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("rejected first Send left a conversation: %v", err)
	}
	channelDetail, err := s.GetAgentChannel(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := agentConversationIDs(channelDetail.Conversations); !reflect.DeepEqual(got, []string{params.Conversation.ID}) {
		t.Fatalf("rejected first Send left ownership: %v", got)
	}
}

func TestAgentChannelFirstTurnRejectsTargetOutsideImmutableAdapterBinding(t *testing.T) {
	t.Parallel()

	s := openTemp(t)
	now := time.Date(2026, 8, 20, 14, 30, 0, 0, time.UTC)
	setting := primarySetting(now)
	channel := createAgentChannelForTest(t, s, "Codex", setting.OptionID, agentBindingFromPrimarySetting(setting), now)
	driftedAuthority := targetAuthority(setting)
	driftedAuthority.Policy.AdapterRevision = "different-adapter-revision"
	params := CreateAgentChannelConversationTurnParams{
		ChannelID: channel.ID,
		Conversation: conversation.Conversation{
			ID: "conversation-adapter-drift", Title: "Adapter drift", State: conversation.ConversationOpen,
			CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute),
		},
		ParticipantID: "participant-adapter-drift", TurnID: "turn-adapter-drift", ClientTurnID: "client-adapter-drift",
		TargetID: "target-adapter-drift", RunID: "run-adapter-drift", HumanID: "human", Body: "do not substitute",
		Authority: driftedAuthority, CreatedAt: now.Add(time.Minute),
	}

	if _, _, _, err := s.CreateAgentChannelConversationTurn(params); err == nil {
		t.Fatal("first Send persisted a target outside the Agent Channel adapter binding")
	}
	if _, err := s.GetConversation(params.Conversation.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("rejected first Send left a Conversation: %v", err)
	}
}

func TestAgentConversationTurnRejectsTargetOutsideImmutableAdapterBinding(t *testing.T) {
	t.Parallel()

	s := openTemp(t)
	now := time.Date(2026, 8, 20, 14, 40, 0, 0, time.UTC)
	setting := primarySetting(now)
	channel := createAgentChannelForTest(t, s, "Codex", setting.OptionID, agentBindingFromPrimarySetting(setting), now)
	child := conversation.Conversation{
		ID: "conversation-existing-adapter-drift", Title: "Existing adapter drift", State: conversation.ConversationOpen,
		CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute),
	}
	participantID := "participant-existing-adapter-drift"
	if err := s.CreateAgentChannelConversation(channel.ID, child, participantID); err != nil {
		t.Fatal(err)
	}
	driftedAuthority := targetAuthority(setting)
	driftedAuthority.Policy.AdapterRevision = "different-adapter-revision"

	if _, _, _, err := s.CreateConversationTurn(CreateConversationTurnParams{
		TurnID: "turn-existing-adapter-drift", ClientTurnID: "client-existing-adapter-drift",
		ConversationID: child.ID, AgentChannelID: channel.ID, HumanID: "human", Body: "do not substitute",
		Targets: []ConversationTurnTarget{{
			ID: "target-existing-adapter-drift", ParticipantID: participantID, RunID: "run-existing-adapter-drift", Authority: driftedAuthority,
		}},
		CreatedAt: now.Add(2 * time.Minute), PrimarySingleFlight: true,
	}); err == nil {
		t.Fatal("existing Agent Conversation persisted a target outside its Agent Channel adapter binding")
	}
	detail, err := s.GetConversation(child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Messages) != 0 || len(detail.Turns) != 0 || len(detail.Targets) != 0 {
		t.Fatalf("rejected existing turn left durable rows: %+v", detail)
	}
}

func TestAgentConversationRetryRejectsTargetOutsideImmutableAdapterBinding(t *testing.T) {
	t.Parallel()

	s := openTemp(t)
	now := time.Date(2026, 8, 20, 14, 50, 0, 0, time.UTC)
	setting := primarySetting(now)
	channel := createAgentChannelForTest(t, s, "Codex", setting.OptionID, agentBindingFromPrimarySetting(setting), now)
	child := conversation.Conversation{
		ID: "conversation-retry-adapter-drift", Title: "Retry adapter drift", State: conversation.ConversationOpen,
		CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute),
	}
	participantID := "participant-retry-adapter-drift"
	if err := s.CreateAgentChannelConversation(channel.ID, child, participantID); err != nil {
		t.Fatal(err)
	}
	_, targets, _, err := s.CreateConversationTurn(CreateConversationTurnParams{
		TurnID: "turn-retry-adapter-drift", ClientTurnID: "client-retry-adapter-drift",
		ConversationID: child.ID, AgentChannelID: channel.ID, HumanID: "human", Body: "fail once",
		Targets: []ConversationTurnTarget{{
			ID: "target-retry-adapter-drift", ParticipantID: participantID, RunID: "run-retry-adapter-drift", Authority: targetAuthority(setting),
		}},
		CreatedAt: now.Add(2 * time.Minute), PrimarySingleFlight: true,
	})
	if err != nil || len(targets) != 1 {
		t.Fatalf("create original target = %+v, %v", targets, err)
	}
	failureReceipt := conversation.TargetReceipt{
		ObservedAdapterID: "unknown", ObservedAdapterRevision: "unknown", ObservedCodexVersion: "unknown",
		ObservedCodexExecutableRevision: "unknown", ObservedCodexSchemaRevision: "unknown",
		ProviderTerminalStatus: "provider_failed", UsageSource: "unknown",
	}
	if changed, err := s.TransitionConversationTargetWithReceipt(targets[0].ID, conversation.TargetQueued, conversation.TargetFailed, "provider_failed", "failed", failureReceipt); err != nil || !changed {
		t.Fatalf("fail original target = %v, %v", changed, err)
	}

	if _, err := s.RetryAgentConversationTargetWithAdapterRevision(
		channel.ID,
		targets[0].ID,
		"target-retry-adapter-drift-2",
		"run-retry-adapter-drift-2",
		"different-adapter-revision",
		now.Add(3*time.Minute),
	); err == nil {
		t.Fatal("Agent retry persisted a target outside its Agent Channel adapter binding")
	}
	detail, err := s.GetConversation(child.ID)
	if err != nil || len(detail.Targets) != 1 {
		t.Fatalf("rejected retry changed durable targets = %+v, %v", detail.Targets, err)
	}
}

func TestAgentChannelListPresentationAndOwnershipAreBounded(t *testing.T) {
	t.Parallel()

	s := openTemp(t)
	empty, err := s.ListAgentChannels("open")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty list = %#v, %v", empty, err)
	}
	now := time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC)
	setting := primarySetting(now)
	first := createAgentChannelForTest(t, s, "Codex", setting.OptionID, agentBindingFromPrimarySetting(setting), now)
	secondSetting := setting
	secondSetting.OptionID = "primary-option:v1:mini"
	secondSetting.Seat.ID, secondSetting.Seat.Machine = "seat:v1:mini", "mini"
	second := createAgentChannelForTest(t, s, "Codex Mini", secondSetting.OptionID, agentBindingFromPrimarySetting(secondSetting), now.Add(time.Minute))
	if err := s.CreateAgentChannelConversation(first.ID, conversation.Conversation{
		ID: "owned", Title: "Owned", State: conversation.ConversationOpen,
		CreatedAt: now.Add(2 * time.Minute), UpdatedAt: now.Add(2 * time.Minute),
	}, "participant-owned"); err != nil {
		t.Fatal(err)
	}
	if owned, err := s.AgentConversationOwned(first.ID, "owned"); err != nil || !owned {
		t.Fatalf("correct ownership = %v, %v", owned, err)
	}
	if owned, err := s.AgentConversationOwned(second.ID, "owned"); err != nil || owned {
		t.Fatalf("foreign ownership = %v, %v", owned, err)
	}

	if err := s.RenameAgentChannel(first.ID, "Personal Codex"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := s.SetAgentChannelState(first.ID, conversation.AgentChannelArchived); err != nil {
		t.Fatalf("archive: %v", err)
	}
	changed, err := s.GetAgentChannel(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Channel.Name != "Personal Codex" || changed.Channel.State != conversation.AgentChannelArchived ||
		changed.Channel.ID != first.ID || !reflect.DeepEqual(changed.Channel.Binding, first.Binding) {
		t.Fatalf("presentation mutation changed identity: before=%+v after=%+v", first, changed.Channel)
	}

	open, err := s.ListAgentChannels("open")
	if err != nil || !reflect.DeepEqual(agentChannelIDs(open), []string{second.ID}) {
		t.Fatalf("open Channels = %v, %v", agentChannelIDs(open), err)
	}
	archived, err := s.ListAgentChannels("archived")
	if err != nil || !reflect.DeepEqual(agentChannelIDs(archived), []string{first.ID}) {
		t.Fatalf("archived Channels = %v, %v", agentChannelIDs(archived), err)
	}
	all, err := s.ListAgentChannels("all")
	if err != nil || !reflect.DeepEqual(agentChannelIDs(all), []string{first.ID, second.ID}) {
		t.Fatalf("all Channels = %v, %v", agentChannelIDs(all), err)
	}
	if _, err := s.ListAgentChannels("deleted"); err == nil {
		t.Fatal("invalid Agent Channel state filter accepted")
	}
}

func TestAgentChannelIdentityAndOwnershipAreDatabaseImmutable(t *testing.T) {
	t.Parallel()

	s := openTemp(t)
	now := time.Date(2026, 8, 20, 15, 30, 0, 0, time.UTC)
	setting := primarySetting(now)
	first := createAgentChannelForTest(t, s, "First", setting.OptionID, agentBindingFromPrimarySetting(setting), now)
	secondSetting := setting
	secondSetting.OptionID = "primary-option:v1:second"
	secondSetting.Seat.ID, secondSetting.Seat.Machine = "seat:v1:second", "second"
	second := createAgentChannelForTest(t, s, "Second", secondSetting.OptionID, agentBindingFromPrimarySetting(secondSetting), now.Add(time.Minute))
	if err := s.CreateAgentChannelConversation(first.ID, conversation.Conversation{
		ID: "owned-immutable", Title: "Owned", State: conversation.ConversationOpen,
		CreatedAt: now.Add(2 * time.Minute), UpdatedAt: now.Add(2 * time.Minute),
	}, "participant-immutable"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE agent_channel SET binding_json='{}' WHERE id=?`, first.ID); err == nil {
		t.Fatal("database rewrote Agent Channel identity")
	}
	if _, err := s.db.Exec(`UPDATE agent_channel_conversation SET agent_channel_id=? WHERE conversation_id='owned-immutable'`, second.ID); err == nil {
		t.Fatal("database moved a Conversation to another Agent Channel")
	}
	if owned, err := s.AgentConversationOwned(first.ID, "owned-immutable"); err != nil || !owned {
		t.Fatalf("ownership changed after rejected writes: %v, %v", owned, err)
	}
}

func agentChannelIDs(items []conversation.AgentChannelDetail) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Channel.ID)
	}
	return ids
}

func createAgentChannelForTest(t *testing.T, s *Store, name, optionID string, binding conversation.AgentBinding, createdAt time.Time) conversation.AgentChannel {
	t.Helper()
	id, err := conversation.AgentChannelID(binding)
	if err != nil {
		t.Fatalf("derive Agent Channel ID: %v", err)
	}
	channel := conversation.AgentChannel{
		ID: id, Name: name, State: conversation.AgentChannelOpen, OptionID: optionID,
		Binding: binding, CreatedAt: createdAt,
	}
	if err := s.CreateAgentChannel(channel); err != nil {
		t.Fatalf("create Agent Channel: %v", err)
	}
	return channel
}

func agentBindingFromPrimarySetting(setting conversation.PrimaryAgentSetting) conversation.AgentBinding {
	policy := setting.Policy
	return conversation.AgentBinding{
		Seat: conversation.AgentSeatIdentity{
			ID: setting.Seat.ID, Profile: setting.Seat.Profile, Agent: setting.Seat.Agent,
			Model: setting.Seat.Model, Machine: setting.Seat.Machine,
		},
		Authority: conversation.AgentAuthoritySnapshot{
			RequestedModel:  setting.Seat.Model,
			ResolvedModel:   conversation.UnknownProviderIdentity,
			Authority:       setting.Authority,
			PolicyID:        policy.PolicyID,
			PolicyRevision:  policy.PolicyRevision,
			AdapterID:       policy.AdapterID,
			AdapterRevision: policy.AdapterRevision,
			RuntimeContract: policy.RuntimeContract,
			SessionMode:     policy.ThreadMode,
			MemoryMode:      conversation.AgentMemoryEphemeral,
			ExecutionPolicy: map[string]string{
				"account_type": policy.AccountType, "account_plan": policy.AccountPlan,
				"reasoning_effort": policy.ReasoningEffort, "reasoning_context": policy.ReasoningContext,
				"sandbox_mode": policy.SandboxMode, "approval_policy": policy.ApprovalPolicy,
				"workdir_mode": policy.WorkdirMode, "dynamic_tools_mode": policy.DynamicToolsMode,
				"mcp_mode": policy.MCPMode, "command_policy": policy.CommandPolicy,
				"file_read_policy": policy.FileReadPolicy, "isolation_revision": policy.IsolationRevision,
				"codex_version":                  policy.CodexVersion,
				"codex_executable_revision":      policy.CodexExecutableRevision,
				"codex_schema_revision":          policy.CodexSchemaRevision,
				"developer_instruction_revision": policy.DeveloperInstructionRevision,
				"request_timeout_millis":         strconv.Itoa(policy.RequestTimeoutMillis),
			},
		},
	}
}

func agentConversationIDs(items []conversation.AgentConversationSummary) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Conversation.ID)
	}
	return ids
}
