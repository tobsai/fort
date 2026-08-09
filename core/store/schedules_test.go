package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/scheduler"
)

func TestReadScheduleCatalogIsBoundedAndIncludesLatestOccurrence(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 8, 18, 0, 0, 0, time.UTC)
	for index, id := range []string{"active", "paused", "non-today"} {
		if err := s.CreateSchedule(scheduler.Definition{
			ID: id, Title: id, Kind: scheduler.KindCron, Expression: "0 0 9 * * *",
			FlowID: "flow-" + id, Timezone: "America/Chicago", Enabled: id != "paused",
			CreatedAt: now.Add(time.Duration(index) * time.Minute), UpdatedAt: now.Add(time.Duration(index) * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, occurrence := range []scheduler.Occurrence{
		{ID: "active:older", ScheduleID: "active", ScheduledFor: now.Add(-2 * time.Hour), State: scheduler.OccurrenceSucceeded, CreatedAt: now, UpdatedAt: now},
		{ID: "active:latest", ScheduleID: "active", RunID: "run-latest", ScheduledFor: now.Add(-time.Hour), State: scheduler.OccurrenceFailed, Error: "boom", CreatedAt: now, UpdatedAt: now},
	} {
		if err := s.UpsertScheduleOccurrence(occurrence); err != nil {
			t.Fatal(err)
		}
	}

	before := sqliteTotalChanges(t, s)
	rows, err := s.ReadScheduleCatalog(context.Background(), nil, 3)
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("catalog rows = %d, want all three definitions", len(rows))
	}
	var active ScheduleReadRow
	for _, row := range rows {
		if row.Definition.ID == "active" {
			active = row
		}
	}
	if active.LatestOccurrence == nil || active.LatestOccurrence.ID != "active:latest" || active.LatestOccurrence.RunID != "run-latest" || active.LatestOccurrence.Error != "boom" {
		t.Fatalf("active latest occurrence = %+v", active.LatestOccurrence)
	}
	if active.RelatedChannel != nil {
		t.Fatalf("catalog invented Channel link: %+v", active.RelatedChannel)
	}
	if got := sqliteTotalChanges(t, s); got != before {
		t.Fatalf("catalog read changed database: before=%d after=%d", before, got)
	}
	if err := s.UpsertPrimaryAgentSetting(primarySetting(now)); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePrimaryChannel(conversation.Conversation{ID: "channel-1", Title: "Morning brief", CreatedAt: now}, "participant-1"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateScheduleChannelLink(conversation.ScheduleChannelLink{ScheduleID: "active", ConversationID: "channel-1", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	before = sqliteTotalChanges(t, s)
	linkedRows, err := s.ReadScheduleCatalog(context.Background(), nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range linkedRows {
		if row.Definition.ID == "active" {
			if row.RelatedChannel == nil || row.RelatedChannel.ID != "channel-1" || row.RelatedChannel.Name != "Morning brief" {
				t.Fatalf("explicit related Channel = %+v", row.RelatedChannel)
			}
		} else if row.RelatedChannel != nil {
			t.Fatalf("schedule %q inherited invented Channel link %+v", row.Definition.ID, row.RelatedChannel)
		}
	}
	if got := sqliteTotalChanges(t, s); got != before {
		t.Fatalf("linked catalog read changed database: before=%d after=%d", before, got)
	}

	if _, err := s.ReadScheduleCatalog(context.Background(), nil, 2); !errors.Is(err, ErrScheduleCatalogLimit) {
		t.Fatalf("bounded catalog error = %v, want ErrScheduleCatalogLimit", err)
	}
	enabled := true
	activeRows, err := s.ReadScheduleCatalog(context.Background(), &enabled, 2)
	if err != nil || len(activeRows) != 2 {
		t.Fatalf("active catalog = %+v, err=%v", activeRows, err)
	}
	paused := false
	pausedRows, err := s.ReadScheduleCatalog(context.Background(), &paused, 1)
	if err != nil || len(pausedRows) != 1 || pausedRows[0].Definition.ID != "paused" {
		t.Fatalf("paused catalog = %+v, err=%v", pausedRows, err)
	}
}

func TestReadScheduleDetailAndOccurrenceCursorAreBoundedPureReads(t *testing.T) {
	s := openTemp(t)
	observedAt := time.Date(2026, 8, 8, 18, 0, 0, 0, time.UTC)
	definition := scheduler.Definition{
		ID: "all-days", Title: "All days", Kind: scheduler.KindCron, Expression: "0 0 * * * *",
		FlowID: "brief", Timezone: "America/Chicago", Enabled: true, CreatedAt: observedAt, UpdatedAt: observedAt,
	}
	if err := s.CreateSchedule(definition); err != nil {
		t.Fatal(err)
	}
	for offset := -12; offset <= 11; offset++ {
		at := observedAt.Add(time.Duration(offset) * time.Hour)
		if err := s.UpsertScheduleOccurrence(scheduler.Occurrence{
			ID: scheduler.OccurrenceID(definition.ID, at), ScheduleID: definition.ID,
			ScheduledFor: at, State: scheduler.OccurrenceScheduled, CreatedAt: observedAt, UpdatedAt: observedAt,
		}); err != nil {
			t.Fatal(err)
		}
	}
	fractional := observedAt.Add(100 * time.Millisecond)
	if err := s.UpsertScheduleOccurrence(scheduler.Occurrence{
		ID: "all-days:fractional", ScheduleID: definition.ID, ScheduledFor: fractional,
		State: scheduler.OccurrenceScheduled, CreatedAt: observedAt, UpdatedAt: observedAt,
	}); err != nil {
		t.Fatal(err)
	}

	before := sqliteTotalChanges(t, s)
	detail, err := s.ReadScheduleDetail(context.Background(), definition.ID, observedAt, 10)
	if err != nil {
		t.Fatalf("read detail: %v", err)
	}
	if len(detail.Upcoming) != 10 || !detail.Upcoming[0].ScheduledFor.Equal(observedAt) || !detail.Upcoming[1].ScheduledFor.Equal(fractional) || !detail.Upcoming[9].ScheduledFor.Equal(observedAt.Add(8*time.Hour)) {
		t.Fatalf("upcoming = %+v", detail.Upcoming)
	}
	if len(detail.Recent) != 10 || !detail.Recent[0].ScheduledFor.Equal(observedAt.Add(-time.Hour)) || !detail.Recent[9].ScheduledFor.Equal(observedAt.Add(-10*time.Hour)) {
		t.Fatalf("recent = %+v", detail.Recent)
	}
	if got := sqliteTotalChanges(t, s); got != before {
		t.Fatalf("detail read changed database: before=%d after=%d", before, got)
	}

	first, err := s.ReadScheduleOccurrences(context.Background(), definition.ID, 5, time.Time{}, "")
	if err != nil || len(first) != 5 {
		t.Fatalf("first occurrence page = %+v, err=%v", first, err)
	}
	last := first[len(first)-1]
	second, err := s.ReadScheduleOccurrences(context.Background(), definition.ID, 5, last.ScheduledFor, last.ID)
	if err != nil || len(second) != 5 {
		t.Fatalf("second occurrence page = %+v, err=%v", second, err)
	}
	if !second[0].ScheduledFor.Before(last.ScheduledFor) || second[0].ID == last.ID {
		t.Fatalf("exclusive cursor duplicated/skipped boundary: last=%+v second=%+v", last, second[0])
	}
	if _, err := s.ReadScheduleDetail(context.Background(), "missing", observedAt, 10); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing detail error = %v, want sql.ErrNoRows", err)
	}
	if _, err := s.ReadScheduleOccurrences(context.Background(), "missing", 5, time.Time{}, ""); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing occurrences error = %v, want sql.ErrNoRows", err)
	}
	if got := sqliteTotalChanges(t, s); got != before {
		t.Fatalf("occurrence reads changed database: before=%d after=%d", before, got)
	}
}

func sqliteTotalChanges(t *testing.T, s *Store) int64 {
	t.Helper()
	var changes int64
	if err := s.db.QueryRow(`SELECT total_changes()`).Scan(&changes); err != nil {
		t.Fatal(err)
	}
	return changes
}

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
