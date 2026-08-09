package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	modernsqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func primarySetting(now time.Time) conversation.PrimaryAgentSetting {
	return conversation.PrimaryAgentSetting{
		OptionID: "primary-option:v1:studio",
		Seat: conversation.Seat{
			ID:          "seat:v1:studio",
			Profile:     "codex-subscription:gpt-5.6-sol",
			Agent:       "codex-subscription",
			Model:       "gpt-5.6-sol",
			Machine:     "studio",
			DisplayName: "Codex on Studio",
		},
		Authority: conversation.AuthorityChatSubscriptionIsolatedV1,
		Policy: conversation.SubscriptionPolicy{
			PolicyID:                     conversation.PolicyCodexSubscriptionChatV1,
			PolicyRevision:               "policy-revision-v1",
			AdapterID:                    conversation.AdapterCodexSubscription,
			AdapterRevision:              "codex-exec-adapter-v1",
			CodexVersion:                 "0.120.0",
			CodexExecutableRevision:      strings.Repeat("a", 64),
			CodexSchemaRevision:          strings.Repeat("b", 64),
			RuntimeContract:              conversation.RuntimeContractCodexSubscriptionExecV1,
			ReasoningEffort:              "medium",
			ReasoningContext:             "current_turn",
			RequestTimeoutMillis:         120_000,
			DeveloperInstructionRevision: "developer-instruction-v1",
			AccountType:                  conversation.AccountTypeChatGPT,
			AccountPlan:                  "plus",
			ThreadMode:                   conversation.ThreadModeEphemeral,
			SandboxMode:                  conversation.SandboxModeReadOnly,
			ApprovalPolicy:               conversation.ApprovalPolicyNever,
			WorkdirMode:                  conversation.WorkdirModeEmptyPerTarget,
			DynamicToolsMode:             conversation.ToolsModeNone,
			MCPMode:                      conversation.ToolsModeNone,
			CommandPolicy:                conversation.ResourcePolicyDenyAndFail,
			FileReadPolicy:               conversation.ResourcePolicyDenyAndFail,
			IsolationRevision:            "isolation-v1",
		},
		UpdatedAt: now,
	}
}

func targetAuthority(setting conversation.PrimaryAgentSetting) *conversation.TargetAuthority {
	return &conversation.TargetAuthority{
		Authority:      setting.Authority,
		Policy:         setting.Policy,
		RequestedModel: setting.Seat.Model,
	}
}

func TestPrimaryAgentSettingRoundTripsAndUpserts(t *testing.T) {
	s := openTemp(t)
	if _, err := s.GetPrimaryAgentSetting(); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing setting error = %v, want sql.ErrNoRows", err)
	}
	now := time.Date(2026, 8, 8, 12, 0, 0, 123, time.UTC)
	want := primarySetting(now)
	if err := s.UpsertPrimaryAgentSetting(want); err != nil {
		t.Fatalf("upsert setting: %v", err)
	}
	got, err := s.GetPrimaryAgentSetting()
	if err != nil {
		t.Fatalf("get setting: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("setting = %#v, want %#v", got, want)
	}

	want.OptionID = "primary-option:v1:mini"
	want.Seat.ID, want.Seat.Machine, want.Seat.DisplayName = "seat:v1:mini", "mini", "Codex on Mini"
	want.Policy.AccountPlan = "pro"
	want.UpdatedAt = now.Add(time.Minute)
	if err := s.UpsertPrimaryAgentSetting(want); err != nil {
		t.Fatalf("replace setting: %v", err)
	}
	got, err = s.GetPrimaryAgentSetting()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("replaced setting = %#v, %v, want %#v", got, err, want)
	}

	invalid := want
	invalid.Policy.SandboxMode = "workspaceWrite"
	if err := s.UpsertPrimaryAgentSetting(invalid); err == nil {
		t.Fatal("invalid subscription setting persisted")
	}
	got, _ = s.GetPrimaryAgentSetting()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("invalid upsert changed setting to %#v", got)
	}
	if err := s.ClearPrimaryAgentSetting(); err != nil {
		t.Fatalf("clear setting: %v", err)
	}
	if _, err := s.GetPrimaryAgentSetting(); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cleared setting error = %v, want sql.ErrNoRows", err)
	}
}

func TestCreatePrimaryChannelIsAtomicAndSnapshotsSetting(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 8, 13, 0, 0, 0, time.UTC)
	item := conversation.Conversation{ID: "channel-1", Title: "Private", CreatedAt: now, UpdatedAt: now}
	if err := s.CreatePrimaryChannel(item, "participant-1"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("create without setting error = %v, want sql.ErrNoRows", err)
	}
	if _, err := s.GetConversation(item.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("setting-less create left a conversation: %v", err)
	}

	setting := primarySetting(now)
	if err := s.UpsertPrimaryAgentSetting(setting); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(item, "participant-1"); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	detail, err := s.GetConversation(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.PrimaryChannel == nil || len(detail.Participants) != 1 {
		t.Fatalf("primary detail = %+v", detail)
	}
	participant := detail.Participants[0]
	if participant.ID != "participant-1" || participant.SeatID != setting.Seat.ID || participant.Profile != setting.Seat.Profile ||
		participant.Agent != setting.Seat.Agent || participant.Model != setting.Seat.Model || participant.Machine != setting.Seat.Machine {
		t.Fatalf("participant snapshot = %+v, setting = %+v", participant, setting.Seat)
	}
	if detail.PrimaryChannel.Authority != setting.Authority || !reflect.DeepEqual(detail.PrimaryChannel.Policy, setting.Policy) {
		t.Fatalf("primary identity = %+v, setting = %+v", detail.PrimaryChannel, setting)
	}

	replacement := setting
	replacement.OptionID = "primary-option:v1:replacement"
	replacement.Seat.ID, replacement.Seat.Model, replacement.Seat.Machine = "seat:v1:replacement", "gpt-5.6-sol-2", "mini"
	replacement.Seat.Profile = "codex-subscription:" + replacement.Seat.Model
	replacement.UpdatedAt = now.Add(time.Minute)
	if err := s.UpsertPrimaryAgentSetting(replacement); err != nil {
		t.Fatal(err)
	}
	after, err := s.GetConversation(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after.PrimaryChannel, detail.PrimaryChannel) || !reflect.DeepEqual(after.Participants, detail.Participants) {
		t.Fatalf("setting change mutated existing channel:\nbefore=%+v/%+v\nafter=%+v/%+v", detail.PrimaryChannel, detail.Participants, after.PrimaryChannel, after.Participants)
	}

	if err := s.CreateConversation(conversation.Conversation{ID: "legacy", Title: "Legacy", CreatedAt: now}, []conversation.Participant{{
		ID: "duplicate-participant", ConversationID: "legacy", SeatID: "legacy-seat", Profile: "codex", Agent: "codex", Machine: "local", DisplayName: "Legacy", CreatedAt: now,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(conversation.Conversation{ID: "rolled-back", Title: "Rollback", CreatedAt: now}, "duplicate-participant"); err == nil {
		t.Fatal("duplicate participant id unexpectedly created channel")
	}
	if _, err := s.GetConversation("rolled-back"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("failed atomic create left conversation: %v", err)
	}
}

func TestPrimaryChannelIdentityIsImmutableButPresentationCanChange(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 8, 14, 0, 0, 0, time.UTC)
	setting := primarySetting(now)
	if err := s.UpsertPrimaryAgentSetting(setting); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(conversation.Conversation{ID: "channel-1", Title: "Original", CreatedAt: now}, "participant-1"); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameConversation("channel-1", "Renamed"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := s.SetConversationState("channel-1", conversation.ConversationArchived); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, err := s.db.Exec(`UPDATE primary_channel SET policy_revision='drift' WHERE conversation_id='channel-1'`); err == nil {
		t.Fatal("primary marker update succeeded")
	}
	if _, err := s.db.Exec(`DELETE FROM primary_channel WHERE conversation_id='channel-1'`); err == nil {
		t.Fatal("primary marker delete succeeded")
	}
	if err := s.AddConversationParticipant(conversation.Participant{
		ID: "participant-2", ConversationID: "channel-1", SeatID: "other", Profile: "other", Agent: "other", Machine: "mini", DisplayName: "Other", CreatedAt: now,
	}); err == nil {
		t.Fatal("participant insert into primary Channel succeeded")
	}
	if err := s.RemoveConversationParticipant("channel-1", "participant-1", now); err == nil {
		t.Fatal("participant removal from primary Channel succeeded")
	}
	if err := s.DeleteConversation("channel-1"); err == nil {
		t.Fatal("primary Channel deletion succeeded")
	}
	detail, err := s.GetConversation("channel-1")
	if err != nil || detail.Conversation.Title != "Renamed" || detail.Conversation.State != conversation.ConversationArchived || len(detail.Participants) != 1 {
		t.Fatalf("channel after rejected identity mutations = %+v, %v", detail, err)
	}
}

func TestPrimaryMarkerInsertRequiresTheSoleExactActiveParticipant(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 8, 14, 30, 0, 0, time.UTC)
	setting := primarySetting(now)
	participant := func(id, channel string, position int) conversation.Participant {
		return conversation.Participant{
			ID: id, ConversationID: channel, SeatID: setting.Seat.ID + ":" + id,
			Profile: setting.Seat.Profile, Agent: setting.Seat.Agent, Model: setting.Seat.Model,
			Machine: setting.Seat.Machine, DisplayName: setting.Seat.DisplayName,
			Position: position, State: conversation.ParticipantActive, CreatedAt: now,
		}
	}
	if err := s.CreateConversation(conversation.Conversation{ID: "one", Title: "One", CreatedAt: now}, []conversation.Participant{participant("one-p1", "one", 0)}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateConversation(conversation.Conversation{ID: "other", Title: "Other", CreatedAt: now}, []conversation.Participant{participant("other-p1", "other", 0)}); err != nil {
		t.Fatal(err)
	}
	if err := insertPrimaryMarkerForTest(s, "one", "other-p1", setting, now); err == nil {
		t.Fatal("marker accepted a participant from another conversation")
	}
	if err := s.AddConversationParticipant(participant("one-p2", "one", 1)); err != nil {
		t.Fatal(err)
	}
	if err := insertPrimaryMarkerForTest(s, "one", "one-p1", setting, now); err == nil {
		t.Fatal("marker accepted a conversation with two active participants")
	}
	if err := s.RemoveConversationParticipant("one", "one-p2", now); err != nil {
		t.Fatal(err)
	}
	if err := insertPrimaryMarkerForTest(s, "one", "one-p1", setting, now); err != nil {
		t.Fatalf("marker rejected sole exact active participant: %v", err)
	}
	detail, err := s.GetConversation("one")
	if err != nil || detail.PrimaryChannel == nil || len(detail.Participants) != 2 {
		t.Fatalf("marked legacy detail = %+v, %v", detail, err)
	}
}

func TestPrimaryChannelPinOrderingAndStateFilters(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	setting := primarySetting(now)
	if err := s.UpsertPrimaryAgentSetting(setting); err != nil {
		t.Fatal(err)
	}
	for index, id := range []string{"old", "middle", "new"} {
		created := now.Add(time.Duration(index) * time.Minute)
		if err := s.CreatePrimaryChannel(conversation.Conversation{ID: id, Title: id, CreatedAt: created}, "participant-"+id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AppendConversationMessage(conversation.Message{ConversationID: "old", AuthorKind: conversation.AuthorHuman, AuthorID: "human", Body: "latest activity", CreatedAt: now.Add(10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPrimaryChannelPinned("middle", true, now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPrimaryChannelPinned("middle", true, now.Add(20*time.Minute)); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListPrimaryChannels("")
	if err != nil {
		t.Fatal(err)
	}
	if got := channelIDs(items); !reflect.DeepEqual(got, []string{"middle", "old", "new"}) {
		t.Fatalf("open ordering = %v", got)
	}
	if !items[0].Pinned || !items[0].PinnedAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("idempotent pin = %+v", items[0])
	}

	if err := s.SetConversationState("new", conversation.ConversationArchived); err != nil {
		t.Fatal(err)
	}
	archived, err := s.ListPrimaryChannels("archived")
	if err != nil || !reflect.DeepEqual(channelIDs(archived), []string{"new"}) {
		t.Fatalf("archived = %+v, %v", archived, err)
	}
	all, err := s.ListPrimaryChannels("all")
	if err != nil || len(all) != 3 {
		t.Fatalf("all = %+v, %v", all, err)
	}
	if _, err := s.ListPrimaryChannels("deleted"); err == nil {
		t.Fatal("invalid state filter accepted")
	}
	if err := s.SetPrimaryChannelPinned("middle", false, time.Time{}); err != nil {
		t.Fatal(err)
	}
	items, _ = s.ListPrimaryChannels("open")
	if got := channelIDs(items); !reflect.DeepEqual(got, []string{"old", "middle"}) {
		t.Fatalf("unpin ordering = %v", got)
	}
}

func TestPrimaryTargetAuthorityIsRequiredMatchedAndImmutable(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 8, 16, 0, 0, 0, time.UTC)
	setting := primarySetting(now)
	if err := s.UpsertPrimaryAgentSetting(setting); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(conversation.Conversation{ID: "channel", Title: "Private", CreatedAt: now}, "participant"); err != nil {
		t.Fatal(err)
	}
	base := CreateConversationTurnParams{
		TurnID: "missing-authority", ConversationID: "channel", HumanID: "human", Body: "must remain atomic",
		Targets: []ConversationTurnTarget{{ID: "target-missing", ParticipantID: "participant", RunID: "run-missing"}}, CreatedAt: now,
	}
	if _, _, _, err := s.CreateConversationTurn(base); err == nil {
		t.Fatal("primary target without authority succeeded")
	}
	detail, _ := s.GetConversation("channel")
	if len(detail.Messages) != 0 || len(detail.Turns) != 0 || len(detail.Targets) != 0 {
		t.Fatalf("rejected authority left partial rows: %+v", detail)
	}

	drift := *targetAuthority(setting)
	drift.Policy.PolicyRevision = "policy-drift"
	base.TurnID, base.Targets[0].ID, base.Targets[0].RunID, base.Targets[0].Authority = "drift", "target-drift", "run-drift", &drift
	if _, _, _, err := s.CreateConversationTurn(base); err == nil {
		t.Fatal("drifted primary target authority succeeded")
	}

	wantAuthority := targetAuthority(setting)
	base.TurnID, base.Targets[0].ID, base.Targets[0].RunID, base.Targets[0].Authority = "turn-ok", "target-ok", "run-ok", wantAuthority
	_, targets, _, err := s.CreateConversationTurn(base)
	if err != nil {
		t.Fatalf("create authorized turn: %v", err)
	}
	if len(targets) != 1 || !reflect.DeepEqual(targets[0].Authority, wantAuthority) {
		t.Fatalf("target authority = %+v, want %+v", targets, wantAuthority)
	}
	if _, err := s.db.Exec(`UPDATE conversation_target SET selected_codex_version='drift' WHERE id='target-ok'`); err == nil {
		t.Fatal("selected target authority update succeeded")
	}
	detail, err = s.GetConversation("channel")
	if err != nil || len(detail.Targets) != 1 || !reflect.DeepEqual(detail.Targets[0].Authority, wantAuthority) {
		t.Fatalf("persisted target authority = %+v, %v", detail.Targets, err)
	}
}

func TestPrimaryChannelConcurrentSameClientTurnReturnsDurableWinner(t *testing.T) {
	first, second, setting, now := openPrimaryStorePair(t)
	params := func(suffix string) CreateConversationTurnParams {
		return CreateConversationTurnParams{
			TurnID: "turn-" + suffix, ClientTurnID: "same-client-turn", ConversationID: "channel",
			HumanID: "human", Body: "exactly once", PrimarySingleFlight: true,
			Targets: []ConversationTurnTarget{{
				ID: "target-" + suffix, ParticipantID: "participant", RunID: "run-" + suffix,
				Authority: targetAuthority(setting),
			}},
			CreatedAt: now,
		}
	}

	type outcome struct {
		turn    conversation.Turn
		targets []conversation.Target
		err     error
	}
	start := make(chan struct{})
	results := make(chan outcome, 2)
	var wait sync.WaitGroup
	for _, call := range []struct {
		store  *Store
		params CreateConversationTurnParams
	}{{first, params("first")}, {second, params("second")}} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			turn, targets, _, err := call.store.CreateConversationTurn(call.params)
			results <- outcome{turn: turn, targets: targets, err: err}
		}()
	}
	close(start)
	wait.Wait()
	close(results)

	var outcomes []outcome
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent idempotent turn: %v", result.err)
		}
		outcomes = append(outcomes, result)
	}
	if len(outcomes) != 2 || outcomes[0].turn.ID != outcomes[1].turn.ID {
		t.Fatalf("outcomes did not return one winner: %+v", outcomes)
	}
	created := 0
	for _, result := range outcomes {
		if result.turn.Created {
			created++
		}
		if len(result.targets) != 1 || result.targets[0].TurnID != result.turn.ID {
			t.Fatalf("idempotent targets = %+v for turn %+v", result.targets, result.turn)
		}
	}
	if created != 1 {
		t.Fatalf("created outcomes = %d, want exactly one", created)
	}
	detail, err := first.GetConversation("channel")
	if err != nil || len(detail.Messages) != 1 || len(detail.Turns) != 1 || len(detail.Targets) != 1 {
		t.Fatalf("durable winner detail = %+v, %v", detail, err)
	}
}

func TestPrimaryChannelConcurrentDifferentClientTurnsPermitOneActiveTarget(t *testing.T) {
	first, second, setting, now := openPrimaryStorePair(t)
	params := func(suffix string) CreateConversationTurnParams {
		return CreateConversationTurnParams{
			TurnID: "turn-" + suffix, ClientTurnID: "client-" + suffix, ConversationID: "channel",
			HumanID: "human", Body: "body " + suffix, PrimarySingleFlight: true,
			Targets: []ConversationTurnTarget{{
				ID: "target-" + suffix, ParticipantID: "participant", RunID: "run-" + suffix,
				Authority: targetAuthority(setting),
			}},
			CreatedAt: now,
		}
	}

	start := make(chan struct{})
	errorsOut := make(chan error, 2)
	var wait sync.WaitGroup
	for _, call := range []struct {
		store  *Store
		params CreateConversationTurnParams
	}{{first, params("first")}, {second, params("second")}} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, _, _, err := call.store.CreateConversationTurn(call.params)
			errorsOut <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errorsOut)

	succeeded, active := 0, 0
	for err := range errorsOut {
		if err == nil {
			succeeded++
			continue
		}
		var bounded *conversation.BoundedError
		if errors.As(err, &bounded) && bounded.Code == conversation.ErrorConversationActive && errors.Is(err, conversation.ErrConversationActive) {
			active++
			continue
		}
		t.Fatalf("losing turn error = %v, want conversation_active", err)
	}
	if succeeded != 1 || active != 1 {
		t.Fatalf("outcomes: success=%d conversation_active=%d", succeeded, active)
	}
	detail, err := first.GetConversation("channel")
	if err != nil || len(detail.Messages) != 1 || len(detail.Turns) != 1 || len(detail.Targets) != 1 {
		t.Fatalf("single-flight detail = %+v, %v", detail, err)
	}
}

func TestPrimarySingleFlightTriggerProtectsOldWritersAndLeavesLegacyConversationsAlone(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 8, 16, 20, 0, 0, time.UTC)
	setting := primarySetting(now)
	if err := s.UpsertPrimaryAgentSetting(setting); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(conversation.Conversation{ID: "channel", Title: "Private", CreatedAt: now}, "participant"); err != nil {
		t.Fatal(err)
	}
	primaryParams := func(suffix string) CreateConversationTurnParams {
		return CreateConversationTurnParams{
			TurnID: "primary-turn-" + suffix, ClientTurnID: "primary-client-" + suffix, ConversationID: "channel",
			HumanID: "human", Body: suffix,
			Targets:   []ConversationTurnTarget{{ID: "primary-target-" + suffix, ParticipantID: "participant", RunID: "primary-run-" + suffix, Authority: targetAuthority(setting)}},
			CreatedAt: now,
		}
	}
	if _, _, _, err := s.CreateConversationTurn(primaryParams("one")); err != nil {
		t.Fatal(err)
	}
	assertActive := func(suffix string) {
		t.Helper()
		if _, _, _, err := s.CreateConversationTurn(primaryParams(suffix)); err == nil {
			t.Fatal("old writer bypassed primary Channel single-flight trigger")
		} else {
			var bounded *conversation.BoundedError
			if !errors.As(err, &bounded) || bounded.Code != conversation.ErrorConversationActive || !errors.Is(err, conversation.ErrConversationActive) {
				t.Fatalf("trigger error = %v, want conversation_active", err)
			}
		}
	}
	assertActive("queued-blocked")
	if changed, err := s.TransitionConversationTarget("primary-target-one", conversation.TargetQueued, conversation.TargetWorking, ""); err != nil || !changed {
		t.Fatalf("mark first target working: changed=%v err=%v", changed, err)
	}
	assertActive("working-blocked")
	failureReceipt := conversation.TargetReceipt{
		ObservedAdapterID: "unknown", ObservedAdapterRevision: "unknown", ObservedCodexVersion: "unknown",
		ObservedCodexExecutableRevision: "unknown", ObservedCodexSchemaRevision: "unknown",
		ProviderTerminalStatus: "test_failed", UsageSource: "unknown",
	}
	if changed, err := s.TransitionConversationTargetWithReceipt(
		"primary-target-one", conversation.TargetWorking, conversation.TargetFailed, "test_failed", "terminal",
		failureReceipt,
	); err != nil || !changed {
		t.Fatalf("release active slot: changed=%v err=%v", changed, err)
	}
	if _, _, _, err := s.CreateConversationTurn(primaryParams("after-terminal")); err != nil {
		t.Fatalf("terminal target did not release active slot: %v", err)
	}
	if _, err := s.RetryConversationTargetWithAdapterRevision(
		"primary-target-one", "primary-target-retry-blocked", "primary-run-retry-blocked", "adapter-revision-v2", now.Add(time.Minute),
	); err == nil {
		t.Fatal("retry created a second active Primary target")
	} else {
		var bounded *conversation.BoundedError
		if !errors.As(err, &bounded) || bounded.Code != conversation.ErrorConversationActive {
			t.Fatalf("blocked retry error = %v, want conversation_active", err)
		}
	}
	if _, err := s.db.Exec(`UPDATE conversation_target SET state='queued' WHERE id='primary-target-one'`); err == nil {
		t.Fatal("database update created a second active Primary target")
	} else {
		var sqliteErr *modernsqlite.Error
		if !errors.As(err, &sqliteErr) || sqliteErr.Code()&0xff != sqlite3.SQLITE_CONSTRAINT {
			t.Fatalf("direct update error = %v, want SQLite constraint", err)
		}
	}
	primaryDetail, _ := s.GetConversation("channel")
	targetStates := map[string]conversation.TargetState{}
	for _, target := range primaryDetail.Targets {
		targetStates[target.ID] = target.State
	}
	if len(primaryDetail.Messages) != 2 || len(primaryDetail.Turns) != 2 || len(primaryDetail.Targets) != 2 ||
		targetStates["primary-target-one"] != conversation.TargetFailed || targetStates["primary-target-after-terminal"] != conversation.TargetQueued {
		t.Fatalf("single-flight target history = %+v", primaryDetail)
	}
	if _, _, _, err := s.CreateConversationTurn(primaryParams("still-blocked")); err == nil {
		t.Fatal("active slot was lost after rejected direct update")
	} else {
		var bounded *conversation.BoundedError
		if !errors.As(err, &bounded) || bounded.Code != conversation.ErrorConversationActive {
			t.Fatalf("trigger error = %v, want conversation_active", err)
		}
	}

	legacy := conversation.Conversation{ID: "legacy", Title: "Shared", CreatedAt: now}
	legacyParticipant := conversation.Participant{ID: "legacy-participant", ConversationID: legacy.ID, SeatID: "legacy-seat", Profile: "codex", Agent: "codex", Machine: "local", DisplayName: "Legacy", CreatedAt: now}
	if err := s.CreateConversation(legacy, []conversation.Participant{legacyParticipant}); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"one", "two"} {
		if _, _, _, err := s.CreateConversationTurn(CreateConversationTurnParams{
			TurnID: "legacy-turn-" + suffix, ClientTurnID: "legacy-client-" + suffix, ConversationID: legacy.ID,
			HumanID: "human", Body: suffix,
			Targets:   []ConversationTurnTarget{{ID: "legacy-target-" + suffix, ParticipantID: legacyParticipant.ID, RunID: "legacy-run-" + suffix}},
			CreatedAt: now,
		}); err != nil {
			t.Fatalf("legacy active turn %s: %v", suffix, err)
		}
	}
	legacyDetail, _ := s.GetConversation(legacy.ID)
	if len(legacyDetail.Messages) != 2 || len(legacyDetail.Turns) != 2 || len(legacyDetail.Targets) != 2 {
		t.Fatalf("legacy conversation was constrained: %+v", legacyDetail)
	}
}

func TestPrimaryTargetRetryCopiesAuthorityIntoFreshAttempt(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 8, 16, 30, 0, 0, time.UTC)
	setting := primarySetting(now)
	if err := s.UpsertPrimaryAgentSetting(setting); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(conversation.Conversation{ID: "channel", Title: "Private", CreatedAt: now}, "participant"); err != nil {
		t.Fatal(err)
	}
	_, targets, _, err := s.CreateConversationTurn(CreateConversationTurnParams{
		TurnID: "turn", ConversationID: "channel", HumanID: "human", Body: "try",
		Targets: []ConversationTurnTarget{{ID: "target-1", ParticipantID: "participant", RunID: "run-1", Authority: targetAuthority(setting)}}, CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	failureReceipt := conversation.TargetReceipt{
		ObservedAdapterID: "unknown", ObservedAdapterRevision: "unknown", ObservedCodexVersion: "unknown",
		ObservedCodexExecutableRevision: "unknown", ObservedCodexSchemaRevision: "unknown",
		ProviderTerminalStatus: "seat_unready", UsageSource: "unknown",
	}
	if changed, err := s.TransitionConversationTargetWithReceipt(targets[0].ID, conversation.TargetQueued, conversation.TargetFailed, "seat_unready", "retry", failureReceipt); err != nil || !changed {
		t.Fatalf("fail original: changed=%v err=%v", changed, err)
	}
	retry, err := s.RetryConversationTargetWithAdapterRevision("target-1", "target-2", "run-2", "codex-exec-adapter-v2", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	wantAuthority := *targets[0].Authority
	wantAuthority.Policy.AdapterRevision = "codex-exec-adapter-v2"
	if retry.Target.Attempt != 2 || retry.Target.State != conversation.TargetQueued || !reflect.DeepEqual(retry.Target.Authority, &wantAuthority) {
		t.Fatalf("retry target = %+v, original = %+v", retry.Target, targets[0])
	}
	if retry.Target.Receipt != nil {
		t.Fatalf("retry resumed provider metadata: %+v", retry.Target.Receipt)
	}
	drifted := retry.Target
	drifted.ID, drifted.RunID, drifted.Attempt = "target-drift", "run-drift", 3
	drifted.Authority = cloneTargetAuthority(drifted.Authority)
	drifted.Authority.Policy.CodexSchemaRevision = strings.Repeat("c", 64)
	if err := insertConversationTarget(s.db, drifted); err == nil {
		t.Fatal("retry target drifted the Channel-pinned Codex schema revision")
	}
}

func TestPrimaryAnswerPersistsOneImmutableReceiptAtomically(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 8, 16, 45, 0, 0, time.UTC)
	setting := primarySetting(now)
	if err := s.UpsertPrimaryAgentSetting(setting); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(conversation.Conversation{ID: "channel", Title: "Private", CreatedAt: now}, "participant"); err != nil {
		t.Fatal(err)
	}
	turn, _, _, err := s.CreateConversationTurn(CreateConversationTurnParams{
		TurnID: "turn", ConversationID: "channel", HumanID: "human", Body: "hello",
		Targets: []ConversationTurnTarget{{ID: "target", ParticipantID: "participant", RunID: "run", Authority: targetAuthority(setting)}}, CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := s.TransitionConversationTarget("target", conversation.TargetQueued, conversation.TargetWorking, ""); err != nil || !changed {
		t.Fatalf("working: changed=%v err=%v", changed, err)
	}
	message := conversation.Message{
		ConversationID: "channel", TurnID: turn.ID, TargetID: "target",
		AuthorKind: conversation.AuthorAssistant, AuthorID: "participant", Body: "answer", CreatedAt: now.Add(time.Minute),
	}
	if changed, err := s.AnswerConversationTarget("target", message); err == nil || changed {
		t.Fatalf("subscription answer without receipt: changed=%v err=%v", changed, err)
	}
	if _, err := s.db.Exec(`UPDATE conversation_target SET state='answered' WHERE id='target'`); err == nil {
		t.Fatal("database accepted a terminal primary target without a receipt")
	}
	receipt := conversation.TargetReceipt{
		ObservedAdapterID:               setting.Policy.AdapterID,
		ObservedAdapterRevision:         setting.Policy.AdapterRevision,
		ObservedCodexVersion:            setting.Policy.CodexVersion,
		ObservedCodexExecutableRevision: setting.Policy.CodexExecutableRevision,
		ObservedCodexSchemaRevision:     setting.Policy.CodexSchemaRevision,
		ProviderThreadID:                "ephemeral-thread-evidence",
		ProviderTerminalStatus:          "completed",
		UsageSource:                     "codex_exec_jsonl",
		InputTokens:                     12,
		CachedInputTokens:               3,
		OutputTokens:                    7,
		ReasoningTokens:                 2,
	}
	changed, err := s.AnswerConversationTargetWithReceipt("target", message, receipt)
	if err != nil || !changed {
		t.Fatalf("answer with receipt: changed=%v err=%v", changed, err)
	}
	detail, err := s.GetConversation("channel")
	if err != nil || len(detail.Targets) != 1 || !reflect.DeepEqual(detail.Targets[0].Receipt, &receipt) || len(detail.Messages) != 2 {
		t.Fatalf("terminal detail = %+v, %v", detail, err)
	}
	if _, err := s.db.Exec(`UPDATE conversation_target SET observed_adapter_revision='rewrite' WHERE id='target'`); err == nil {
		t.Fatal("terminal receipt rewrite succeeded")
	}
}

func TestPrimaryInterruptedTargetGetsTruthfulUnknownReceipt(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 8, 16, 55, 0, 0, time.UTC)
	setting := primarySetting(now)
	if err := s.UpsertPrimaryAgentSetting(setting); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(conversation.Conversation{ID: "channel", Title: "Private", CreatedAt: now}, "participant"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := s.CreateConversationTurn(CreateConversationTurnParams{
		TurnID: "turn", ConversationID: "channel", HumanID: "human", Body: "hello",
		Targets: []ConversationTurnTarget{{ID: "target", ParticipantID: "participant", RunID: "run", Authority: targetAuthority(setting)}}, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	count, err := s.FailInterruptedConversationTargets("daemon stopped")
	if err != nil || count != 1 {
		t.Fatalf("reconcile count=%d err=%v", count, err)
	}
	detail, err := s.GetConversation("channel")
	if err != nil || len(detail.Targets) != 1 {
		t.Fatalf("detail = %+v, %v", detail, err)
	}
	target := detail.Targets[0]
	if target.State != conversation.TargetFailed || target.ErrorCode != "daemon_interrupted" || target.Receipt == nil ||
		target.Receipt.ProviderTerminalStatus != "daemon_interrupted" || target.Receipt.ObservedCodexExecutableRevision != "unknown" {
		t.Fatalf("interrupted target = %+v", target)
	}
}

func TestLegacyConversationAndTargetRemainUnmarked(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 8, 17, 0, 0, 0, time.UTC)
	item := conversation.Conversation{ID: "legacy", Title: "Shared", CreatedAt: now}
	participant := conversation.Participant{ID: "legacy-participant", ConversationID: item.ID, SeatID: "legacy-seat", Profile: "codex", Agent: "codex", Machine: "local", DisplayName: "Legacy", CreatedAt: now}
	if err := s.CreateConversation(item, []conversation.Participant{participant}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := s.CreateConversationTurn(CreateConversationTurnParams{
		TurnID: "legacy-turn", ConversationID: item.ID, HumanID: "human", Body: "legacy works",
		Targets: []ConversationTurnTarget{{ID: "legacy-target", ParticipantID: participant.ID, RunID: "legacy-run"}}, CreatedAt: now,
	}); err != nil {
		t.Fatalf("legacy turn: %v", err)
	}
	detail, err := s.GetConversation(item.ID)
	if err != nil || detail.PrimaryChannel != nil || len(detail.Targets) != 1 || detail.Targets[0].Authority != nil {
		t.Fatalf("legacy detail = %+v, %v", detail, err)
	}
	items, err := s.ListPrimaryChannels("all")
	if err != nil || len(items) != 0 {
		t.Fatalf("legacy conversation leaked into Channels: %+v, %v", items, err)
	}
}

func TestPrimaryChannelsWithSameSeatHaveDisjointContext(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 8, 18, 0, 0, 0, time.UTC)
	setting := primarySetting(now)
	if err := s.UpsertPrimaryAgentSetting(setting); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ channel, participant, body string }{
		{"channel-a", "participant-a", "only alpha"},
		{"channel-b", "participant-b", "only beta"},
	} {
		if err := s.CreatePrimaryChannel(conversation.Conversation{ID: item.channel, Title: item.channel, CreatedAt: now}, item.participant); err != nil {
			t.Fatal(err)
		}
	}
	_, _, alpha, err := s.CreateConversationTurn(CreateConversationTurnParams{
		TurnID: "turn-a", ConversationID: "channel-a", HumanID: "human", Body: "only alpha",
		Targets: []ConversationTurnTarget{{ID: "target-a", ParticipantID: "participant-a", RunID: "run-a", Authority: targetAuthority(setting)}}, CreatedAt: now, PrimarySingleFlight: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, beta, err := s.CreateConversationTurn(CreateConversationTurnParams{
		TurnID: "turn-b", ConversationID: "channel-b", HumanID: "human", Body: "only beta",
		Targets: []ConversationTurnTarget{{ID: "target-b", ParticipantID: "participant-b", RunID: "run-b", Authority: targetAuthority(setting)}}, CreatedAt: now, PrimarySingleFlight: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"channel-b", "participant-b", "only beta"} {
		if strings.Contains(alpha, forbidden) {
			t.Fatalf("alpha context contains %q: %s", forbidden, alpha)
		}
	}
	for _, forbidden := range []string{"channel-a", "participant-a", "only alpha"} {
		if strings.Contains(beta, forbidden) {
			t.Fatalf("beta context contains %q: %s", forbidden, beta)
		}
	}
}

func TestScheduleChannelLinkRoundTripsWithoutChangingScheduleTarget(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 8, 19, 0, 0, 0, time.UTC)
	setting := primarySetting(now)
	if err := s.UpsertPrimaryAgentSetting(setting); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(conversation.Conversation{ID: "channel", Title: "Private", CreatedAt: now}, "participant"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO schedule(id,title,kind,expression,flow_id,timezone,enabled,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		"schedule", "Nightly", "cron", "0 2 * * *", "actual-flow", "America/Chicago", 1, nowOr(now), nowOr(now)); err != nil {
		t.Fatal(err)
	}
	want := conversation.ScheduleChannelLink{ScheduleID: "schedule", ConversationID: "channel", CreatedAt: now}
	if err := s.CreateScheduleChannelLink(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetScheduleChannelLink("schedule")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("link = %+v, %v, want %+v", got, err, want)
	}
	var flowID string
	if err := s.db.QueryRow(`SELECT flow_id FROM schedule WHERE id='schedule'`).Scan(&flowID); err != nil || flowID != "actual-flow" {
		t.Fatalf("schedule flow = %q, %v", flowID, err)
	}
}

func TestPrimaryColumnsMigrateWithoutRewritingLegacyTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE conversation_target (
id TEXT PRIMARY KEY, turn_id TEXT NOT NULL, participant_id TEXT NOT NULL,
run_id TEXT NOT NULL UNIQUE, attempt INTEGER NOT NULL DEFAULT 1, state TEXT NOT NULL,
error_code TEXT, error TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
INSERT INTO conversation_target(id,turn_id,participant_id,run_id,attempt,state,error_code,error,created_at,updated_at)
VALUES('legacy-target','legacy-turn','legacy-participant','legacy-run',1,'answered','','','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatalf("migrate legacy db: %v", err)
	}
	defer s.Close()
	var id string
	var authority sql.NullString
	if err := s.db.QueryRow(`SELECT id,authority FROM conversation_target WHERE id='legacy-target'`).Scan(&id, &authority); err != nil {
		t.Fatal(err)
	}
	if id != "legacy-target" || authority.Valid {
		t.Fatalf("legacy target rewritten: id=%q authority=%+v", id, authority)
	}
}

func openPrimaryStorePair(t *testing.T) (*Store, *Store, conversation.PrimaryAgentSetting, time.Time) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fort.db")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	now := time.Date(2026, 8, 8, 16, 10, 0, 0, time.UTC)
	setting := primarySetting(now)
	if err := first.UpsertPrimaryAgentSetting(setting); err != nil {
		t.Fatal(err)
	}
	if err := first.CreatePrimaryChannel(conversation.Conversation{ID: "channel", Title: "Private", CreatedAt: now}, "participant"); err != nil {
		t.Fatal(err)
	}
	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	return first, second, setting, now
}

func channelIDs(items []conversation.PrimaryChannelSummary) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Conversation.ID)
	}
	return out
}

func insertPrimaryMarkerForTest(s *Store, channelID, participantID string, setting conversation.PrimaryAgentSetting, createdAt time.Time) error {
	values := []any{channelID, participantID, setting.Authority}
	values = append(values, subscriptionPolicyValues(setting.Policy)...)
	values = append(values, nowOr(createdAt))
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(values)), ",")
	_, err := s.db.Exec(`INSERT INTO primary_channel(conversation_id,participant_id,authority,`+subscriptionPolicyColumns+`,created_at) VALUES(`+placeholders+`)`, values...)
	return err
}
