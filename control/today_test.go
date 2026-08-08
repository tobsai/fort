package control

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/scheduler"
	"github.com/tobsai/fort/core/store"
)

func TestTodayProjectionDoesNotMaterializeScheduleRows(t *testing.T) {
	st := newStore(t)
	now := time.Date(2026, 8, 2, 15, 0, 0, 0, time.UTC)
	if err := st.CreateSchedule(scheduler.Definition{
		ID: "unmaterialized", Kind: scheduler.KindOnce, Expression: "2026-08-02T16:00:00Z",
		FlowID: "brief", Timezone: "UTC", Enabled: true, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	service := NewTodayService(st, activeTargets{})

	view, err := service.Today(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	occurrences, err := st.ScheduleOccurrencesBetween(now.Truncate(24*time.Hour), now.Truncate(24*time.Hour).Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(occurrences) != 0 {
		t.Fatalf("GET Today materialized schedule rows: %+v", occurrences)
	}
	if view.Scheduled == nil {
		t.Fatal("read-only empty Today projection returned a nil scheduled array")
	}
}

type activeTargets map[string]bool

func (a activeTargets) ConversationTargetActive(id string) bool { return a[id] }

type scheduledResumeRuntime struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newScheduledResumeRuntime() *scheduledResumeRuntime {
	return &scheduledResumeRuntime{started: make(chan struct{}), release: make(chan struct{})}
}

func (r *scheduledResumeRuntime) Name() string { return "scheduled-resume" }

func (r *scheduledResumeRuntime) Dispatch(_ context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	events := make(chan runtime.RunEvent)
	done := make(chan struct{})
	go func() {
		events <- runtime.RunEvent{RunID: spec.RunID, Type: runtime.EventStarted, Time: time.Now().UTC(), Data: spec.Agent}
		close(r.started)
		<-r.release
		events <- runtime.RunEvent{RunID: spec.RunID, Type: runtime.EventExited, Time: time.Now().UTC()}
		close(events)
		close(done)
	}()
	return &scheduledResumeRun{id: spec.RunID, events: events, done: done, release: r.unblock}, nil
}

func (r *scheduledResumeRuntime) unblock() { r.once.Do(func() { close(r.release) }) }

type scheduledResumeRun struct {
	id      string
	events  <-chan runtime.RunEvent
	done    <-chan struct{}
	release func()
}

func (r *scheduledResumeRun) ID() string                      { return r.id }
func (r *scheduledResumeRun) Stream() <-chan runtime.RunEvent { return r.events }
func (r *scheduledResumeRun) Signal(string) error             { return nil }
func (r *scheduledResumeRun) Cancel() error                   { r.release(); return nil }
func (r *scheduledResumeRun) Status() runtime.Status {
	select {
	case <-r.done:
		return runtime.Status{State: runtime.StateSucceeded}
	default:
		return runtime.Status{State: runtime.StateRunning}
	}
}
func (r *scheduledResumeRun) Wait() runtime.Status {
	<-r.done
	return runtime.Status{State: runtime.StateSucceeded}
}

func TestTodayShowsResumedScheduledWorkInProgress(t *testing.T) {
	st := newStore(t)
	now := time.Now().UTC()
	definition := scheduler.Definition{
		ID: "scheduled-resume", Kind: scheduler.KindOnce, Expression: now.Format(time.RFC3339),
		FlowID: "gated-work", Title: "Gated work", Timezone: "UTC", Enabled: true,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := st.CreateSchedule(definition); err != nil {
		t.Fatal(err)
	}
	runID := "schedule-" + scheduler.OccurrenceID(definition.ID, now)
	if err := st.UpsertScheduleOccurrence(scheduler.Occurrence{
		ID: scheduler.OccurrenceID(definition.ID, now), ScheduleID: definition.ID, RunID: runID,
		ScheduledFor: now, State: scheduler.OccurrenceRunning, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	rt := newScheduledResumeRuntime()
	executor := graph.NewExecutor(rt, st)
	t.Cleanup(func() {
		rt.unblock()
		executor.Wait()
	})
	flow := graph.Flow{ID: definition.FlowID, Name: definition.Title, Start: "approval", Nodes: []graph.Node{
		{ID: "approval", Type: graph.Gate, Edges: []graph.Edge{{On: graph.OutApprove, To: "work"}}},
		{ID: "work", Type: graph.Task, Agent: "codex"},
	}}
	result, err := executor.Start(context.Background(), flow, runID, "payload")
	if err != nil || result.State != "paused" {
		t.Fatalf("start = %+v err=%v, want paused", result, err)
	}
	service := NewTodayService(st, activeTargets{})
	if err := executor.Approve(runID, "approval", ""); err != nil {
		t.Fatal(err)
	}
	if err := executor.ResumeAsync(context.Background(), flow, runID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-rt.started:
	case <-time.After(time.Second):
		t.Fatal("resumed task never started")
	}

	var startedPersisted bool
	deadline := time.Now().Add(time.Second)
	for !startedPersisted && time.Now().Before(deadline) {
		events, eventsErr := st.Events(runID)
		if eventsErr != nil {
			t.Fatal(eventsErr)
		}
		for _, event := range events {
			startedPersisted = startedPersisted || event.Type == string(runtime.EventStarted)
		}
		if !startedPersisted {
			time.Sleep(time.Millisecond)
		}
	}
	if !startedPersisted {
		t.Fatal("resumed task activity was not persisted")
	}

	run, err := st.GetRun(runID)
	if err != nil || run.Status != "running" {
		t.Fatalf("resumed run = %+v err=%v, want running", run, err)
	}
	view, err := service.Today(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.InProgress) != 1 || view.InProgress[0].RunID != runID {
		t.Fatalf("resumed work missing from In progress: %+v", view.InProgress)
	}
	if len(view.Scheduled) != 1 || view.Scheduled[0].State != scheduler.OccurrenceRunning {
		t.Fatalf("resumed schedule = %+v, want running", view.Scheduled)
	}

	rt.unblock()
	executor.Wait()
	run, err = st.GetRun(runID)
	if err != nil || run.Status != "succeeded" {
		t.Fatalf("completed run = %+v err=%v, want succeeded", run, err)
	}
	view, err = service.Today(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Scheduled) != 1 || view.Scheduled[0].State != scheduler.OccurrenceSucceeded {
		t.Fatalf("completed schedule = %+v, want succeeded", view.Scheduled)
	}
}

func TestTodayUsesDurableTargetsAndScheduleOccurrences(t *testing.T) {
	st := newStore(t)
	now := time.Date(2026, 7, 31, 15, 0, 0, 0, time.UTC)
	conversationItem := conversation.Conversation{ID: "c1", Title: "Shared thread", CreatedAt: now, UpdatedAt: now}
	participant := conversation.Participant{ID: "p1", ConversationID: "c1", SeatID: "codex@studio", Profile: "codex", Agent: "codex", Model: "gpt-5.6-sol", Machine: "studio", DisplayName: "Codex on Studio", CreatedAt: now}
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
	service := NewTodayService(st, activeTargets{})
	view, err := service.Today(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatalf("today: %v", err)
	}
	if len(view.InProgress) != 1 || view.InProgress[0].TargetID != targets[0].ID || view.InProgress[0].Machine != "studio" || view.InProgress[0].Model != "gpt-5.6-sol" {
		t.Fatalf("in progress = %+v", view.InProgress)
	}
	if len(view.Scheduled) != 1 || view.Scheduled[0].FlowID != "brief" || !view.Scheduled[0].ScheduledFor.Equal(now) {
		t.Fatalf("scheduled = %+v", view.Scheduled)
	}
}

func TestTodayPreservesExactModelForLegacyQueuedRun(t *testing.T) {
	st := newStore(t)
	now := time.Now().UTC()
	if err := st.CreateRun(store.Run{
		ID: "legacy-queued", Title: "Legacy task", Agent: "claude", Profile: "claude:opus",
		Model: "claude-opus-4-1", Status: "queued", Machine: "mini", CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	view, err := NewTodayService(st, activeTargets{}).Today(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.InProgress) != 1 || view.InProgress[0].Model != "claude-opus-4-1" {
		t.Fatalf("in progress = %+v", view.InProgress)
	}
}

func TestTodayShowsOneReadableNextRowPerSchedule(t *testing.T) {
	st := newStore(t)
	now := time.Date(2026, 7, 31, 15, 30, 0, 0, time.UTC)
	definition := scheduler.Definition{
		ID: "hourly", Kind: scheduler.KindCron, Expression: "0 0 * * * *", FlowID: "check",
		Title: "Hourly check", Timezone: "UTC", Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := st.CreateSchedule(definition); err != nil {
		t.Fatal(err)
	}
	for _, scheduledFor := range []time.Time{now.Add(-time.Hour), now.Add(30 * time.Minute), now.Add(90 * time.Minute)} {
		if err := st.UpsertScheduleOccurrence(scheduler.Occurrence{
			ID: scheduler.OccurrenceID(definition.ID, scheduledFor), ScheduleID: definition.ID,
			ScheduledFor: scheduledFor, State: scheduler.OccurrenceScheduled, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	view, err := NewTodayService(st, activeTargets{}).Today(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Scheduled) != 1 {
		t.Fatalf("scheduled rows = %+v, want one per schedule", view.Scheduled)
	}
	if !view.Scheduled[0].ScheduledFor.Equal(now.Add(30*time.Minute)) || view.Scheduled[0].Recurrence != "Every hour" {
		t.Fatalf("scheduled row = %+v", view.Scheduled[0])
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
	service := NewTodayService(st, activeTargets{})
	view, err := service.Today(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.InProgress) != 0 {
		t.Fatalf("stale work presented as live: %+v", view.InProgress)
	}
}
