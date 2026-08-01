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
