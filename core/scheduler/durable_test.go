package scheduler

import (
	"testing"
	"time"
)

func TestOccurrencesForDayUsesScheduleTimezone(t *testing.T) {
	definition := Definition{ID: "morning", Kind: KindCron, Expression: "0 0 9 * * *", FlowID: "brief", Timezone: "America/Chicago", Enabled: true}
	day := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	occurrences, err := OccurrencesForDay(definition, day, time.UTC)
	if err != nil {
		t.Fatalf("occurrences: %v", err)
	}
	if len(occurrences) != 1 {
		t.Fatalf("occurrences = %+v", occurrences)
	}
	want := time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC)
	if !occurrences[0].ScheduledFor.Equal(want) {
		t.Fatalf("scheduled for %s, want %s", occurrences[0].ScheduledFor, want)
	}
}

func TestOccurrencesForDayIncludesOneShotOnlyOnItsDay(t *testing.T) {
	definition := Definition{ID: "once", Kind: KindOnce, Expression: "2026-07-31T16:30:00-05:00", FlowID: "ship", Timezone: "America/Chicago", Enabled: true}
	items, err := OccurrencesForDay(definition, time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC), time.UTC)
	if err != nil || len(items) != 1 {
		t.Fatalf("today = %+v, %v", items, err)
	}
	tomorrow, err := OccurrencesForDay(definition, time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC), time.UTC)
	if err != nil || len(tomorrow) != 0 {
		t.Fatalf("tomorrow = %+v, %v", tomorrow, err)
	}
}
