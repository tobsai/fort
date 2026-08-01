package control

import (
	"context"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/scheduler"
	"github.com/tobsai/fort/core/store"
)

type todayScheduleSource struct {
	definitions []scheduler.Definition
}

func (s todayScheduleSource) MaterializeDay(context.Context, time.Time, *time.Location) error {
	return nil
}
func (s todayScheduleSource) Definitions(context.Context) ([]scheduler.Definition, error) {
	return append([]scheduler.Definition(nil), s.definitions...), nil
}

type activeTargets map[string]bool

func (a activeTargets) ConversationTargetActive(id string) bool { return a[id] }

func TestTodayUsesDurableTargetsAndScheduleOccurrences(t *testing.T) {
	st := newStore(t)
	now := time.Date(2026, 7, 31, 15, 0, 0, 0, time.UTC)
	conversationItem := conversation.Conversation{ID: "c1", Title: "Shared thread", CreatedAt: now, UpdatedAt: now}
	participant := conversation.Participant{ID: "p1", ConversationID: "c1", SeatID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", DisplayName: "Codex on Studio", CreatedAt: now}
	if err := st.CreateConversation(conversationItem, []conversation.Participant{participant}); err != nil {
		t.Fatal(err)
	}
	_, targets, _, err := st.CreateConversationTurn(store.CreateConversationTurnParams{TurnID: "turn-1", ConversationID: "c1", HumanID: "human", Body: "hello", Targets: []store.ConversationTurnTarget{{ID: "target-1", ParticipantID: "p1", RunID: "run-1"}}, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateRun(store.Run{ID: "run-1", Title: "Shared thread", Agent: "codex", Status: "queued", Machine: "studio", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	definition := scheduler.Definition{ID: "daily", Kind: scheduler.KindCron, Expression: "0 0 9 * * *", FlowID: "brief", Timezone: "America/Chicago", Enabled: true, CreatedAt: now, UpdatedAt: now}
	if err := st.CreateSchedule(definition); err != nil {
		t.Fatal(err)
	}
	occurrence := scheduler.Occurrence{ID: scheduler.OccurrenceID("daily", now), ScheduleID: "daily", ScheduledFor: now, State: scheduler.OccurrenceScheduled, CreatedAt: now, UpdatedAt: now}
	if err := st.UpsertScheduleOccurrence(occurrence); err != nil {
		t.Fatal(err)
	}
	service := NewTodayService(st, todayScheduleSource{definitions: []scheduler.Definition{definition}}, activeTargets{})
	view, err := service.Today(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatalf("today: %v", err)
	}
	if len(view.InProgress) != 1 || view.InProgress[0].TargetID != targets[0].ID || view.InProgress[0].Machine != "studio" {
		t.Fatalf("in progress = %+v", view.InProgress)
	}
	if len(view.Scheduled) != 1 || view.Scheduled[0].FlowID != "brief" || !view.Scheduled[0].ScheduledFor.Equal(now) {
		t.Fatalf("scheduled = %+v", view.Scheduled)
	}
}

func TestTodayOmitsWorkingTargetWithoutLiveRunAndStaleLegacyRun(t *testing.T) {
	st := newStore(t)
	now := time.Now().UTC()
	item := conversation.Conversation{ID: "c1", Title: "Thread", CreatedAt: now, UpdatedAt: now}
	participant := conversation.Participant{ID: "p1", ConversationID: "c1", SeatID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", CreatedAt: now}
	if err := st.CreateConversation(item, []conversation.Participant{participant}); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := st.CreateConversationTurn(store.CreateConversationTurnParams{TurnID: "turn-1", ConversationID: "c1", HumanID: "human", Body: "hello", Targets: []store.ConversationTurnTarget{{ID: "target-1", ParticipantID: "p1", RunID: "run-1"}}, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.TransitionConversationTarget("target-1", conversation.TargetQueued, conversation.TargetWorking, ""); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateRun(store.Run{ID: "run-1", Title: "Thread", Agent: "codex", Status: "running", Machine: "studio", CreatedAt: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateRun(store.Run{ID: "legacy-stale", Title: "Old run", Agent: "codex", Status: "running", CreatedAt: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	service := NewTodayService(st, nil, activeTargets{})
	view, err := service.Today(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.InProgress) != 0 {
		t.Fatalf("stale work presented as live: %+v", view.InProgress)
	}
}
