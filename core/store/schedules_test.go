package store

import (
	"testing"
	"time"

	"github.com/tobsai/fort/core/scheduler"
)

func TestDurableSchedulesAndOccurrencesRoundTrip(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC)
	definition := scheduler.Definition{ID: "daily", Kind: scheduler.KindCron, Expression: "0 0 9 * * *", FlowID: "brief", Timezone: "America/Chicago", Enabled: true, CreatedAt: now, UpdatedAt: now}
	if err := s.CreateSchedule(definition); err != nil {
		t.Fatalf("create schedule: %v", err)
	}
	occurrence := scheduler.Occurrence{ID: "daily:1785506400", ScheduleID: definition.ID, ScheduledFor: now, State: scheduler.OccurrenceScheduled, CreatedAt: now, UpdatedAt: now}
	if err := s.UpsertScheduleOccurrence(occurrence); err != nil {
		t.Fatalf("upsert occurrence: %v", err)
	}
	definitions, err := s.ListSchedules()
	if err != nil || len(definitions) != 1 || definitions[0].Timezone != "America/Chicago" {
		t.Fatalf("definitions = %+v, %v", definitions, err)
	}
	items, err := s.ScheduleOccurrencesBetween(now.Add(-time.Minute), now.Add(time.Minute))
	if err != nil || len(items) != 1 || items[0].State != scheduler.OccurrenceScheduled {
		t.Fatalf("occurrences = %+v, %v", items, err)
	}
	changed, err := s.TransitionScheduleOccurrence(occurrence.ID, scheduler.OccurrenceScheduled, scheduler.OccurrenceRunning, "run-1", "")
	if err != nil || !changed {
		t.Fatalf("transition: changed=%v err=%v", changed, err)
	}
}

func TestScheduledOccurrenceTracksBlockedAndResumedRunTruthfully(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 3, 3, 0, 0, 0, time.UTC)
	definition := scheduler.Definition{
		ID: "gated", Kind: scheduler.KindOnce, Expression: now.Format(time.RFC3339),
		FlowID: "ship-feature", Timezone: "UTC", Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateSchedule(definition); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(Run{ID: "schedule-gated", Title: "Gated flow", Agent: "flow:ship-feature", Status: "running", FlowID: "ship-feature", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertScheduleOccurrence(scheduler.Occurrence{
		ID: "gated:1", ScheduleID: definition.ID, RunID: "schedule-gated",
		ScheduledFor: now, State: scheduler.OccurrenceRunning, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateRunStatus("schedule-gated", "blocked", 0, ""); err != nil {
		t.Fatal(err)
	}
	items, err := s.ScheduleOccurrencesBetween(now.Add(-time.Minute), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].State != scheduler.OccurrenceFired {
		t.Fatalf("blocked occurrence = %+v, want fired rather than completed", items)
	}

	if err := s.UpdateRunStatus("schedule-gated", "succeeded", 0, ""); err != nil {
		t.Fatal(err)
	}
	items, err = s.ScheduleOccurrencesBetween(now.Add(-time.Minute), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].State != scheduler.OccurrenceSucceeded {
		t.Fatalf("resumed occurrence = %+v, want succeeded", items)
	}
}

func TestTransitionRunStatusClaimsOnceAndSynchronizesOccurrence(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 3, 3, 0, 0, 0, time.UTC)
	definition := scheduler.Definition{
		ID: "resume-claim", Kind: scheduler.KindOnce, Expression: now.Format(time.RFC3339),
		FlowID: "gated", Timezone: "UTC", Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateSchedule(definition); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(Run{ID: "claimed-run", Title: "Gated", Agent: "flow:gated", Status: "blocked", FlowID: "gated", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertScheduleOccurrence(scheduler.Occurrence{
		ID: "resume-claim:1", ScheduleID: definition.ID, RunID: "claimed-run",
		ScheduledFor: now, State: scheduler.OccurrenceFired, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	changed, err := s.TransitionRunStatus("claimed-run", definition.FlowID, "blocked", "running", 0, "")
	if err != nil || !changed {
		t.Fatalf("first resume claim: changed=%v err=%v", changed, err)
	}
	changed, err = s.TransitionRunStatus("claimed-run", definition.FlowID, "blocked", "running", 0, "")
	if err != nil || changed {
		t.Fatalf("duplicate resume claim: changed=%v err=%v", changed, err)
	}
	items, err := s.ScheduleOccurrencesBetween(now.Add(-time.Minute), now.Add(time.Minute))
	if err != nil || len(items) != 1 || items[0].State != scheduler.OccurrenceRunning {
		t.Fatalf("claimed occurrence = %+v err=%v, want running", items, err)
	}

	changed, err = s.TransitionRunStatus("claimed-run", definition.FlowID, "running", "blocked", 0, "")
	if err != nil || !changed {
		t.Fatalf("release resume claim: changed=%v err=%v", changed, err)
	}
	items, err = s.ScheduleOccurrencesBetween(now.Add(-time.Minute), now.Add(time.Minute))
	if err != nil || len(items) != 1 || items[0].State != scheduler.OccurrenceFired {
		t.Fatalf("released occurrence = %+v err=%v, want fired", items, err)
	}
}
