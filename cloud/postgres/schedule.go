package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

var (
	_ controlapi.ScheduleRepository = (*Store)(nil)
	_ controlapi.ScheduleRepository = (*SharedPool)(nil)
)

// WithScheduleTick holds an account-scoped Postgres transaction and a
// transaction-level advisory lock for the entire one-shot schedule tick.
func (store *Store) WithScheduleTick(
	ctx context.Context,
	accountID, schedulerID string,
	operation func(controlapi.ScheduleTickTransaction) error,
) (bool, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(schedulerID) == "" || operation == nil {
		return false, fmt.Errorf("Postgres schedule tick is incomplete")
	}

	acquired := false
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		if err := tx.queryRow(ctx, `select pg_try_advisory_xact_lock(hashtextextended($1, 0))`, accountID+"|"+schedulerID).scan(&acquired); err != nil {
			return fmt.Errorf("acquire Postgres schedule advisory lock: %w", err)
		}
		if !acquired {
			return nil
		}
		return operation(postgresScheduleTickTransaction{
			tx: tx, accountID: accountID,
			enqueueDue: func(ctx context.Context, tx transaction, occurrence controlapi.RoutineOccurrence) error {
				_, err := store.enqueueRoutineOccurrence(ctx, ledger.EnqueueRoutineOccurrenceCommand{
					AccountID: accountID, RoutineID: occurrence.RoutineID,
					RoutineRevisionID: occurrence.RoutineRevisionID, OccurrenceID: occurrence.OccurrenceID,
					RunID: "routine-run:" + occurrence.OccurrenceID, Kind: conversation.RoutineRunScheduled,
					ScheduledFor: occurrence.ScheduledFor, IdempotencyKey: occurrence.IdempotencyKey,
					ApprovalEvidenceID: "approval-evaluation:" + occurrence.OccurrenceID,
					CreatedAt:          occurrence.RecordedAt,
				}, tx)
				return err
			},
		})
	})
	return acquired, err
}

// WithScheduleTick implements the same contract on a shared warm-function
// pool without allowing the caller to retarget an account-bound Store.
func (pool *SharedPool) WithScheduleTick(
	ctx context.Context,
	accountID, schedulerID string,
	operation func(controlapi.ScheduleTickTransaction) error,
) (bool, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return false, err
	}
	return store.WithScheduleTick(ctx, accountID, schedulerID, operation)
}

type postgresScheduleTickTransaction struct {
	tx         transaction
	accountID  string
	enqueueDue func(context.Context, transaction, controlapi.RoutineOccurrence) error
}

// RecoverExpiredWorkerLeases moves a bounded, locked set of elapsed leases and
// every dependent aggregate to an explicit human-actionable recovery state.
// Work is never silently requeued because doing so could duplicate provider
// execution after a disconnected worker has already crossed the side-effect
// boundary.
func (transaction postgresScheduleTickTransaction) RecoverExpiredWorkerLeases(
	ctx context.Context,
	observedAt time.Time,
	limit int,
) (int, error) {
	if observedAt.IsZero() || limit < 1 || limit > controlapi.MaximumExpiredWorkerLeaseRecoveries {
		return 0, fmt.Errorf("Postgres expired worker lease recovery scope is invalid")
	}
	var recovered int64
	err := transaction.tx.queryRow(ctx, `with candidates as materialized (
  select lease.lease_id, lease.execution_attempt_id, lease.target_id,
    target.target_kind, target.turn_id
  from fort_private.worker_lease as lease
  join fort_private.execution_attempt as attempt
    on attempt.account_id = lease.account_id
   and attempt.execution_attempt_id = lease.execution_attempt_id
  join fort_private.conversation_target as target
    on target.account_id = lease.account_id and target.target_id = lease.target_id
  where lease.account_id = $1 and lease.state = 'active' and lease.expires_at <= $2
    and attempt.state in ('leased','working','cancel_requested')
    and target.state in ('claimed','working','cancel_requested')
  order by lease.expires_at, lease.lease_id
  limit $3
  for update of lease skip locked
), expired_leases as (
  update fort_private.worker_lease as lease
  set state = 'expired', released_at = $2, updated_at = $2
  from candidates as candidate
  where lease.account_id = $1 and lease.lease_id = candidate.lease_id
    and lease.state = 'active' and lease.expires_at <= $2
  returning candidate.lease_id, candidate.execution_attempt_id,
    candidate.target_id, candidate.target_kind, candidate.turn_id
), expired_attempts as (
  update fort_private.execution_attempt as attempt
  set state = 'lease_expired', updated_at = $2
  from expired_leases as expired
  where attempt.account_id = $1
    and attempt.execution_attempt_id = expired.execution_attempt_id
    and attempt.state in ('leased','working','cancel_requested')
), expired_targets as (
  update fort_private.conversation_target as target
  set state = 'lease_expired', error_code = 'worker_lease_expired', updated_at = $2
  from expired_leases as expired
  where target.account_id = $1 and target.target_id = expired.target_id
    and target.state in ('claimed','working','cancel_requested')
), handoff_recovery as (
  update fort_private.handoff as handoff
  set state = 'needs_you', updated_at = $2
  from expired_leases as expired
  where expired.target_kind = 'handoff' and handoff.account_id = $1
    and handoff.target_id = expired.target_id and handoff.state in ('queued','working')
), routine_run_recovery as (
  update fort_private.routine_run as run
  set state = 'needs_you', execution_attempt_id = coalesce(run.execution_attempt_id, expired.execution_attempt_id),
    failure_code = 'worker_lease_expired',
    next_action = jsonb_build_object('kind','review_expired_worker_lease'), updated_at = $2
  from expired_leases as expired
  where expired.target_kind = 'routine' and run.account_id = $1
    and run.target_id = expired.target_id and run.state in ('queued','working')
  returning run.routine_occurrence_id
), routine_occurrence_recovery as (
  update fort_private.routine_occurrence as occurrence
  set state = 'missed_needs_attention', updated_at = $2
  from routine_run_recovery as run
  where occurrence.account_id = $1
    and occurrence.routine_occurrence_id = run.routine_occurrence_id
    and occurrence.state in ('queued','working')
), turn_recovery as (
  update fort_private.conversation_turn as turn
  set state = 'needs_you', updated_at = $2
  from expired_leases as expired
  where turn.account_id = $1 and turn.turn_id = expired.turn_id and turn.state = 'open'
), recovery_events as (
  insert into fort_private.ledger_event (
    account_id, aggregate_kind, aggregate_id, event_type, target_id,
    execution_attempt_id, event_metadata, created_at
  )
  select $1, 'worker_lease', expired.lease_id, 'worker_lease_expired',
    expired.target_id, expired.execution_attempt_id,
    jsonb_build_object('reason','heartbeat_elapsed','target_kind',expired.target_kind), $2
  from expired_leases as expired
)
select count(*) from expired_leases`, transaction.accountID, observedAt.UTC(), limit).scan(&recovered)
	if err != nil {
		return 0, fmt.Errorf("recover expired Postgres worker leases: %w", err)
	}
	return int(recovered), nil
}

func (transaction postgresScheduleTickTransaction) ExpireLateRoutineRuns(
	ctx context.Context,
	observedAt time.Time,
	limit int,
) (int, error) {
	if observedAt.IsZero() || limit < 1 || limit > controlapi.MaximumLateRoutineRunExpirations {
		return 0, fmt.Errorf("Postgres late Routine run recovery scope is invalid")
	}
	rows, err := transaction.tx.query(ctx, `select occurrence.routine_occurrence_id,occurrence.routine_id,
  occurrence.routine_revision_id,occurrence.scheduled_for,occurrence.idempotency_key
from fort_private.routine_occurrence as occurrence
join fort_private.routine_run as run
  on run.account_id=occurrence.account_id and run.routine_occurrence_id=occurrence.routine_occurrence_id
where occurrence.account_id=$1 and occurrence.state='queued' and run.state='queued'
  and occurrence.scheduled_for < $2
order by occurrence.scheduled_for,occurrence.routine_occurrence_id
limit $3 for update of occurrence skip locked`, transaction.accountID,
		observedAt.UTC().Add(-controlapi.MaximumScheduleLateness), limit)
	if err != nil {
		return 0, fmt.Errorf("select late Postgres Routine runs: %w", err)
	}
	occurrences := make([]controlapi.RoutineOccurrence, 0)
	for rows.next() {
		var occurrence controlapi.RoutineOccurrence
		if err := rows.scan(&occurrence.OccurrenceID, &occurrence.RoutineID, &occurrence.RoutineRevisionID,
			&occurrence.ScheduledFor, &occurrence.IdempotencyKey); err != nil {
			rows.close()
			return 0, err
		}
		occurrence.State = controlapi.OccurrenceMissedNeedsAttention
		occurrence.RecordedAt = observedAt.UTC()
		occurrences = append(occurrences, occurrence)
	}
	if err := rows.errResult(); err != nil {
		rows.close()
		return 0, err
	}
	rows.close()
	for _, occurrence := range occurrences {
		affected, err := transaction.tx.exec(ctx, `update fort_private.routine_occurrence
set state='missed_needs_attention',updated_at=$1
where account_id=$2 and routine_occurrence_id=$3 and state='queued'`, observedAt.UTC(),
			transaction.accountID, occurrence.OccurrenceID)
		if err != nil || affected != 1 {
			return 0, changedRowsError("expire late Routine occurrence", affected, err)
		}
		if err := transaction.recordOccurrenceEvent(ctx, occurrence,
			"routine.occurrence.state_changed"); err != nil {
			return 0, err
		}
		if err := markPostgresRoutineOccurrenceLate(ctx, transaction.tx, transaction.accountID,
			occurrence.OccurrenceID, observedAt); err != nil {
			return 0, err
		}
	}
	return len(occurrences), nil
}

func (transaction postgresScheduleTickTransaction) Watermark(ctx context.Context, schedulerID string) (time.Time, bool, error) {
	var watermark time.Time
	var priorTickID string
	err := transaction.tx.queryRow(ctx, `select last_success_at, last_tick_id
from fort_private.schedule_tick_watermark
where account_id = $1 and scheduler_id = $2
for update`, transaction.accountID, schedulerID).scan(&watermark, &priorTickID)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("read Postgres schedule watermark: %w", err)
	}
	return watermark.UTC(), true, nil
}

func (transaction postgresScheduleTickTransaction) ActiveRoutineSchedules(ctx context.Context, limit int) ([]controlapi.RoutineSchedule, error) {
	if limit < 1 {
		return nil, fmt.Errorf("Postgres Routine schedule limit must be positive")
	}
	result, err := transaction.tx.query(ctx, `select routine.routine_id, revision.routine_revision_id,
  revision.schedule_expression, revision.timezone,
  greatest(coalesce(revision.next_occurrence_at, revision.created_at), revision.created_at)
from fort_private.routine as routine
join fort_private.routine_revision as revision
  on revision.account_id = routine.account_id
 and revision.routine_id = routine.routine_id
 and revision.routine_revision_id = routine.current_revision_id
where routine.account_id = $1
  and routine.authority = 'fort_cloud'
  and routine.state = 'active'
  and revision.trigger_kind = 'cron'
order by routine.routine_id
limit $2
for share of routine`, transaction.accountID, limit)
	if err != nil {
		return nil, fmt.Errorf("list Postgres Routine schedules: %w", err)
	}
	defer result.close()

	routines := make([]controlapi.RoutineSchedule, 0)
	for result.next() {
		var routine controlapi.RoutineSchedule
		if err := result.scan(&routine.RoutineID, &routine.RoutineRevisionID, &routine.Expression, &routine.Timezone, &routine.StartsAt); err != nil {
			return nil, fmt.Errorf("scan Postgres Routine schedule: %w", err)
		}
		routines = append(routines, routine)
	}
	if err := result.errResult(); err != nil {
		return nil, fmt.Errorf("iterate Postgres Routine schedules: %w", err)
	}
	return routines, nil
}

func (transaction postgresScheduleTickTransaction) ApplyOccurrence(ctx context.Context, occurrence controlapi.RoutineOccurrence) (bool, error) {
	if occurrence.OccurrenceID == "" || occurrence.RoutineID == "" || occurrence.RoutineRevisionID == "" ||
		occurrence.IdempotencyKey == "" || occurrence.ScheduledFor.IsZero() || occurrence.RecordedAt.IsZero() {
		return false, fmt.Errorf("Postgres Routine occurrence is incomplete")
	}
	if occurrence.State != controlapi.OccurrenceScheduled && occurrence.State != controlapi.OccurrenceQueued &&
		occurrence.State != controlapi.OccurrenceMissedNeedsAttention {
		return false, fmt.Errorf("Postgres Routine occurrence state is invalid")
	}

	inserted, err := transaction.tx.exec(ctx, `insert into fort_private.routine_occurrence(
  account_id, routine_occurrence_id, routine_id, routine_revision_id,
  scheduled_for, is_test, state, idempotency_key, created_at, updated_at
) values ($1, $2, $3, $4, $5, false, $6, $7, $8, $8)
on conflict do nothing`,
		transaction.accountID, occurrence.OccurrenceID, occurrence.RoutineID,
		occurrence.RoutineRevisionID, occurrence.ScheduledFor.UTC(), occurrence.State,
		occurrence.IdempotencyKey, occurrence.RecordedAt.UTC())
	if err != nil {
		return false, fmt.Errorf("insert Postgres Routine occurrence: %w", err)
	}

	var existingID, existingRevision, existingKey, existingState string
	err = transaction.tx.queryRow(ctx, `select routine_occurrence_id, routine_revision_id, idempotency_key, state
from fort_private.routine_occurrence
where account_id = $1 and routine_id = $2 and scheduled_for = $3`,
		transaction.accountID, occurrence.RoutineID, occurrence.ScheduledFor.UTC()).scan(
		&existingID, &existingRevision, &existingKey, &existingState,
	)
	if err != nil {
		return false, fmt.Errorf("verify Postgres Routine occurrence: %w", err)
	}
	if existingID != occurrence.OccurrenceID || existingRevision != occurrence.RoutineRevisionID || existingKey != occurrence.IdempotencyKey {
		return false, fmt.Errorf("Postgres Routine occurrence identity conflict")
	}
	if inserted == 1 {
		if err := transaction.recordOccurrenceEvent(ctx, occurrence, "routine.occurrence.materialized"); err != nil {
			return false, err
		}
		if err := transaction.enqueueDueOccurrence(ctx, occurrence); err != nil {
			return false, err
		}
		return true, nil
	}
	if inserted != 0 {
		return false, fmt.Errorf("Postgres Routine occurrence insert affected %d rows", inserted)
	}

	existing := controlapi.RoutineOccurrenceState(existingState)
	canAdvance := (existing == controlapi.OccurrenceScheduled && occurrence.State != controlapi.OccurrenceScheduled) ||
		(existing == controlapi.OccurrenceQueued && occurrence.State == controlapi.OccurrenceMissedNeedsAttention)
	if !canAdvance {
		if err := transaction.enqueueDueOccurrence(ctx, occurrence); err != nil {
			return false, err
		}
		return false, nil
	}
	updated, err := transaction.tx.exec(ctx, `update fort_private.routine_occurrence
set state = $4, updated_at = $5
where account_id = $1 and routine_id = $2 and scheduled_for = $3
	and routine_occurrence_id = $6
	and (
	  (state = 'scheduled' and $4::text in ('queued', 'missed_needs_attention'))
	  or (state = 'queued' and $4::text = 'missed_needs_attention')
	)`,
		transaction.accountID, occurrence.RoutineID, occurrence.ScheduledFor.UTC(),
		occurrence.State, occurrence.RecordedAt.UTC(), occurrence.OccurrenceID)
	if err != nil {
		return false, fmt.Errorf("advance Postgres Routine occurrence: %w", err)
	}
	if updated > 1 {
		return false, fmt.Errorf("Postgres Routine occurrence update affected %d rows", updated)
	}
	if updated == 1 {
		if err := transaction.recordOccurrenceEvent(ctx, occurrence, "routine.occurrence.state_changed"); err != nil {
			return false, err
		}
		if err := transaction.enqueueDueOccurrence(ctx, occurrence); err != nil {
			return false, err
		}
	}
	return updated == 1, nil
}

func (transaction postgresScheduleTickTransaction) enqueueDueOccurrence(ctx context.Context,
	occurrence controlapi.RoutineOccurrence) error {
	if occurrence.State == controlapi.OccurrenceScheduled || transaction.enqueueDue == nil {
		return nil
	}
	if occurrence.State == controlapi.OccurrenceMissedNeedsAttention {
		if err := markPostgresRoutineOccurrenceLate(ctx, transaction.tx, transaction.accountID,
			occurrence.OccurrenceID, occurrence.RecordedAt); err != nil {
			return err
		}
	}
	return transaction.enqueueDue(ctx, transaction.tx, occurrence)
}

func (transaction postgresScheduleTickTransaction) recordOccurrenceEvent(
	ctx context.Context,
	occurrence controlapi.RoutineOccurrence,
	eventType string,
) error {
	changed, err := transaction.tx.exec(ctx, `insert into fort_private.ledger_event(
  account_id, aggregate_kind, aggregate_id, event_type, event_metadata, created_at
) values (
  $1, 'routine_occurrence', $2, $3,
  jsonb_build_object(
    'routine_id', $4::text,
    'routine_revision_id', $5::text,
    'scheduled_for', $6::timestamptz,
    'state', $7::text
  ), $8
)`, transaction.accountID, occurrence.OccurrenceID, eventType, occurrence.RoutineID,
		occurrence.RoutineRevisionID, occurrence.ScheduledFor.UTC(), occurrence.State,
		occurrence.RecordedAt.UTC())
	if err != nil {
		return fmt.Errorf("append Postgres Routine occurrence event: %w", err)
	}
	if changed != 1 {
		return fmt.Errorf("Postgres Routine occurrence event affected %d rows", changed)
	}
	return nil
}

func (transaction postgresScheduleTickTransaction) SaveWatermark(
	ctx context.Context,
	schedulerID, tickID string,
	watermark time.Time,
) error {
	changed, err := transaction.tx.exec(ctx, `insert into fort_private.schedule_tick_watermark(
  account_id, scheduler_id, last_success_at, last_tick_id, updated_at
) values ($1, $2, $3, $4, $3)
on conflict (account_id, scheduler_id) do update
set last_success_at = excluded.last_success_at,
    last_tick_id = excluded.last_tick_id,
    updated_at = excluded.updated_at
where fort_private.schedule_tick_watermark.last_success_at < excluded.last_success_at`,
		transaction.accountID, schedulerID, watermark.UTC(), tickID)
	if err != nil {
		return fmt.Errorf("save Postgres schedule watermark: %w", err)
	}
	if changed != 1 {
		return controlapi.ErrScheduleClockNotMonotonic
	}
	return nil
}
