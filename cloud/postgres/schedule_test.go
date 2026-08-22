package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tobsai/fort/cloud/controlapi"
)

func TestWithScheduleTickUsesAccountTransactionAndAdvisoryLock(t *testing.T) {
	t.Parallel()

	tx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		if strings.Contains(sql, "pg_try_advisory_xact_lock") {
			return fakeRow{values: []any{true}}
		}
		return fakeRow{err: errors.New("unexpected query")}
	}}
	database := &fakeDatabase{transactions: []transaction{tx}}
	store, err := newStore(database, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	called := false

	acquired, err := store.WithScheduleTick(context.Background(), testAccountID, "fort-cloud", func(controlapi.ScheduleTickTransaction) error {
		called = true
		return nil
	})
	if err != nil || !acquired || !called {
		t.Fatalf("WithScheduleTick acquired/called/error = %t/%t/%v", acquired, called, err)
	}
	if len(tx.execs) != 1 || !strings.Contains(tx.execs[0].sql, "set_config('fort.account_id'") {
		t.Fatalf("first transaction statement = %+v", tx.execs)
	}
	if len(tx.queries) != 1 || !strings.Contains(tx.queries[0].sql, "pg_try_advisory_xact_lock") {
		t.Fatalf("advisory lock query = %+v", tx.queries)
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("transaction lifecycle = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func TestWithScheduleTickSkipsOverlappingInvocation(t *testing.T) {
	t.Parallel()

	tx := &fakeTransaction{queryRowHook: func(string, []any) row { return fakeRow{values: []any{false}} }}
	database := &fakeDatabase{transactions: []transaction{tx}}
	store, err := newStore(database, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	called := false

	acquired, err := store.WithScheduleTick(context.Background(), testAccountID, "fort-cloud", func(controlapi.ScheduleTickTransaction) error {
		called = true
		return nil
	})
	if err != nil || acquired || called {
		t.Fatalf("overlap acquired/called/error = %t/%t/%v", acquired, called, err)
	}
	if tx.commits != 1 {
		t.Fatalf("overlap transaction commits = %d, want 1", tx.commits)
	}
}

func TestPostgresScheduleTransactionReadsLockedWatermarkAndActiveCronRoutines(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	tx := &fakeTransaction{
		queryRowHook: func(sql string, _ []any) row {
			if !strings.Contains(sql, "for update") {
				return fakeRow{err: errors.New("watermark was not row locked")}
			}
			return fakeRow{values: []any{now, "tick-before"}}
		},
		queryRows: &fakeRows{values: [][]any{{"routine-1", "revision-1", "0 * * * * *", "UTC", now}}},
	}
	transaction := postgresScheduleTickTransaction{tx: tx, accountID: testAccountID}

	watermark, exists, err := transaction.Watermark(context.Background(), "fort-cloud")
	if err != nil || !exists || !watermark.Equal(now) {
		t.Fatalf("Watermark = %s/%t/%v", watermark, exists, err)
	}
	routines, err := transaction.ActiveRoutineSchedules(context.Background(), 17)
	if err != nil || len(routines) != 1 {
		t.Fatalf("ActiveRoutineSchedules = %+v/%v", routines, err)
	}
	if len(tx.queries) != 2 || !strings.Contains(tx.queries[1].sql, "trigger_kind = 'cron'") ||
		!strings.Contains(tx.queries[1].sql, "routine.state = 'active'") ||
		!strings.Contains(tx.queries[1].sql, "routine.authority = 'fort_cloud'") ||
		!strings.Contains(tx.queries[1].sql, "for share of routine") {
		t.Fatalf("routine query = %+v", tx.queries)
	}
	if tx.queries[1].args[len(tx.queries[1].args)-1] != 17 {
		t.Fatalf("routine limit args = %#v", tx.queries[1].args)
	}
}

func TestPostgresScheduleTransactionRecoversExpiredLeasesAndDependentAggregates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	tx := &fakeTransaction{queryRowHook: func(sql string, arguments []any) row {
		for _, required := range []string{
			"for update of lease skip locked",
			"update fort_private.worker_lease",
			"update fort_private.execution_attempt",
			"update fort_private.conversation_target",
			"update fort_private.handoff",
			"update fort_private.routine_run",
			"update fort_private.routine_occurrence",
			"update fort_private.conversation_turn",
			"insert into fort_private.ledger_event",
		} {
			if !strings.Contains(strings.ToLower(sql), required) {
				return fakeRow{err: errors.New("lease recovery omitted " + required)}
			}
		}
		if len(arguments) != 3 || arguments[0] != testAccountID || arguments[1] != now || arguments[2] != 17 {
			return fakeRow{err: errors.New("lease recovery scope is not exact")}
		}
		return fakeRow{values: []any{int64(2)}}
	}}
	transaction := postgresScheduleTickTransaction{tx: tx, accountID: testAccountID}

	recovered, err := transaction.RecoverExpiredWorkerLeases(context.Background(), now, 17)
	if err != nil || recovered != 2 {
		t.Fatalf("RecoverExpiredWorkerLeases = %d/%v, want 2/nil", recovered, err)
	}
}

func TestPostgresScheduleTransactionExpiresLateRoutineRunsWithOccurrenceEvidence(t *testing.T) {
	t.Parallel()

	due := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	observedAt := due.Add(2 * time.Minute)
	tx := &fakeTransaction{
		queryRows: &fakeRows{values: [][]any{{
			"occurrence-1", "routine-1", "revision-1", due, "routine-1@2026-08-21T20:30:00Z",
		}}},
		queryRowHook: func(statement string, _ []any) row {
			if !strings.Contains(statement, "from fort_private.routine_run as run") {
				return fakeRow{err: errors.New("unexpected late Routine query")}
			}
			return fakeRow{values: []any{"run-1", "target-1", "turn-1"}}
		},
	}
	transaction := postgresScheduleTickTransaction{tx: tx, accountID: testAccountID}

	expired, err := transaction.ExpireLateRoutineRuns(context.Background(), observedAt, 17)
	if err != nil || expired != 1 {
		t.Fatalf("ExpireLateRoutineRuns = %d/%v, want 1/nil", expired, err)
	}
	var occurrenceEvent bool
	for _, statement := range tx.execs {
		if strings.Contains(statement.sql, "insert into fort_private.ledger_event") &&
			containsArgument(statement.args, "routine.occurrence.state_changed") &&
			containsArgument(statement.args, "occurrence-1") {
			occurrenceEvent = true
		}
	}
	if !occurrenceEvent {
		t.Fatalf("late Routine writes omitted occurrence evidence: %+v", tx.execs)
	}
}

func TestPostgresScheduleTransactionAppliesExactOccurrenceIdempotently(t *testing.T) {
	t.Parallel()

	due := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	now := due.Add(20 * time.Second)
	inserted := true
	tx := &fakeTransaction{
		execHook: func(sql string, _ []any) (int64, error) {
			if strings.Contains(sql, "insert into fort_private.routine_occurrence") {
				if inserted {
					inserted = false
					return 1, nil
				}
				return 0, nil
			}
			if strings.Contains(sql, "update fort_private.routine_occurrence") {
				return 0, nil
			}
			if strings.Contains(sql, "insert into fort_private.ledger_event") {
				return 1, nil
			}
			return 0, errors.New("unexpected exec")
		},
		queryRowHook: func(sql string, _ []any) row {
			if !strings.Contains(sql, "routine_occurrence") {
				return fakeRow{err: errors.New("unexpected query")}
			}
			return fakeRow{values: []any{"occurrence-1", "revision-1", "routine-1@2026-08-21T20:30:00Z", string(controlapi.OccurrenceQueued)}}
		},
	}
	transaction := postgresScheduleTickTransaction{tx: tx, accountID: testAccountID}
	occurrence := controlapi.RoutineOccurrence{
		OccurrenceID: "occurrence-1", RoutineID: "routine-1", RoutineRevisionID: "revision-1",
		ScheduledFor: due, State: controlapi.OccurrenceQueued,
		IdempotencyKey: "routine-1@2026-08-21T20:30:00Z", RecordedAt: now,
	}

	changed, err := transaction.ApplyOccurrence(context.Background(), occurrence)
	if err != nil || !changed {
		t.Fatalf("first ApplyOccurrence = %t/%v", changed, err)
	}
	changed, err = transaction.ApplyOccurrence(context.Background(), occurrence)
	if err != nil || changed {
		t.Fatalf("replay ApplyOccurrence = %t/%v, want unchanged", changed, err)
	}
	if len(tx.execs) < 3 || !strings.Contains(tx.execs[0].sql, "on conflict") ||
		len(tx.execs[1].args) < 3 || tx.execs[1].args[2] != "routine.occurrence.materialized" {
		t.Fatalf("occurrence insert statements = %+v", tx.execs)
	}
}

func TestPostgresScheduleTransactionRejectsConflictingOccurrenceIdentity(t *testing.T) {
	t.Parallel()

	due := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	tx := &fakeTransaction{
		execHook: func(string, []any) (int64, error) { return 0, nil },
		queryRowHook: func(string, []any) row {
			return fakeRow{values: []any{"other-occurrence", "revision-1", "same-key", string(controlapi.OccurrenceQueued)}}
		},
	}
	transaction := postgresScheduleTickTransaction{tx: tx, accountID: testAccountID}

	_, err := transaction.ApplyOccurrence(context.Background(), controlapi.RoutineOccurrence{
		OccurrenceID: "occurrence-1", RoutineID: "routine-1", RoutineRevisionID: "revision-1",
		ScheduledFor: due, State: controlapi.OccurrenceQueued, IdempotencyKey: "same-key", RecordedAt: due,
	})
	if err == nil || !strings.Contains(err.Error(), "identity conflict") {
		t.Fatalf("conflicting ApplyOccurrence error = %v", err)
	}
}

func TestPostgresScheduleTransactionMakesPersistedFutureOccurrenceClaimableOnlyWhenDue(t *testing.T) {
	t.Parallel()

	due := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	execs := 0
	tx := &fakeTransaction{
		execHook: func(sql string, _ []any) (int64, error) {
			execs++
			switch {
			case strings.Contains(sql, "insert into fort_private.routine_occurrence"):
				return 0, nil
			case strings.Contains(sql, "update fort_private.routine_occurrence"):
				if !strings.Contains(sql, "state = 'scheduled'") || !strings.Contains(sql, "state = 'queued'") {
					return 0, errors.New("occurrence transition did not fence scheduled state")
				}
				return 1, nil
			case strings.Contains(sql, "insert into fort_private.ledger_event"):
				return 1, nil
			default:
				return 0, errors.New("unexpected exec")
			}
		},
		queryRowHook: func(string, []any) row {
			return fakeRow{values: []any{"occurrence-1", "revision-1", "same-key", string(controlapi.OccurrenceScheduled)}}
		},
	}
	transaction := postgresScheduleTickTransaction{tx: tx, accountID: testAccountID}

	changed, err := transaction.ApplyOccurrence(context.Background(), controlapi.RoutineOccurrence{
		OccurrenceID: "occurrence-1", RoutineID: "routine-1", RoutineRevisionID: "revision-1",
		ScheduledFor: due, State: controlapi.OccurrenceQueued, IdempotencyKey: "same-key", RecordedAt: due,
	})
	if err != nil || !changed {
		t.Fatalf("due transition changed/error = %t/%v", changed, err)
	}
	if execs != 3 || tx.execs[2].args[2] != "routine.occurrence.state_changed" {
		t.Fatalf("transition statements = %+v", tx.execs)
	}
}

func TestPostgresScheduleTransactionExpiresUnclaimedQueuedOccurrenceAfterLatenessBound(t *testing.T) {
	t.Parallel()

	due := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	tx := &fakeTransaction{
		execHook: func(sql string, _ []any) (int64, error) {
			switch {
			case strings.Contains(sql, "insert into fort_private.routine_occurrence"):
				return 0, nil
			case strings.Contains(sql, "update fort_private.routine_occurrence"):
				return 1, nil
			case strings.Contains(sql, "insert into fort_private.ledger_event"):
				return 1, nil
			default:
				return 0, errors.New("unexpected exec")
			}
		},
		queryRowHook: func(string, []any) row {
			return fakeRow{values: []any{"occurrence-1", "revision-1", "same-key", string(controlapi.OccurrenceQueued)}}
		},
	}
	transaction := postgresScheduleTickTransaction{tx: tx, accountID: testAccountID}

	changed, err := transaction.ApplyOccurrence(context.Background(), controlapi.RoutineOccurrence{
		OccurrenceID: "occurrence-1", RoutineID: "routine-1", RoutineRevisionID: "revision-1",
		ScheduledFor: due, State: controlapi.OccurrenceMissedNeedsAttention,
		IdempotencyKey: "same-key", RecordedAt: due.Add(91 * time.Second),
	})
	if err != nil || !changed {
		t.Fatalf("late queued transition changed/error = %t/%v", changed, err)
	}
}

func TestPostgresScheduleTransactionWatermarkIsMonotonic(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	tx := &fakeTransaction{execHook: func(sql string, _ []any) (int64, error) {
		if !strings.Contains(sql, "last_success_at < excluded.last_success_at") {
			return 0, errors.New("watermark update lacks monotonic predicate")
		}
		return 1, nil
	}}
	transaction := postgresScheduleTickTransaction{tx: tx, accountID: testAccountID}

	if err := transaction.SaveWatermark(context.Background(), "fort-cloud", "tick-1", now); err != nil {
		t.Fatalf("SaveWatermark: %v", err)
	}
	tx.execHook = func(string, []any) (int64, error) { return 0, nil }
	if err := transaction.SaveWatermark(context.Background(), "fort-cloud", "tick-duplicate", now); !errors.Is(err, controlapi.ErrScheduleClockNotMonotonic) {
		t.Fatalf("non-monotonic SaveWatermark error = %v", err)
	}
}

func TestPostgresScheduleTransactionMissingWatermark(t *testing.T) {
	t.Parallel()

	tx := &fakeTransaction{queryRowHook: func(string, []any) row { return fakeRow{err: pgx.ErrNoRows} }}
	transaction := postgresScheduleTickTransaction{tx: tx, accountID: testAccountID}
	watermark, exists, err := transaction.Watermark(context.Background(), "fort-cloud")
	if err != nil || exists || !watermark.IsZero() {
		t.Fatalf("missing Watermark = %s/%t/%v", watermark, exists, err)
	}
}
