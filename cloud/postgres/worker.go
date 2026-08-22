package postgres

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/cloud/securebody"
	"github.com/tobsai/fort/core/conversation"
	coreworker "github.com/tobsai/fort/core/worker"
)

const (
	workerReadinessScope = "worker.readiness"
	workerClaimScope     = "worker.claim"
	workerClaimNextScope = "worker.claim_next"
)

func (store *Store) MachineCredential(ctx context.Context, accountID, workerID, machineID string) (controlapi.MachineCredential, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return controlapi.MachineCredential{}, err
	}
	if !validWorkerIdentifier(workerID) || !validWorkerIdentifier(machineID) {
		return controlapi.MachineCredential{}, fmt.Errorf("worker and machine ids are required")
	}

	var credential controlapi.MachineCredential
	var state string
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		return tx.queryRow(ctx, `select account_id::text, worker_id, machine_id, enrollment_token_hash, state
from fort_private.worker
where account_id = $1 and worker_id = $2 and machine_id = $3`, accountID, workerID, machineID).scan(
			&credential.AccountID, &credential.WorkerID, &credential.MachineID, &credential.TokenHash, &state,
		)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return controlapi.MachineCredential{}, controlapi.ErrWorkerNotFound
	}
	if err != nil {
		return controlapi.MachineCredential{}, err
	}
	credential.State = controlapi.MachineCredentialState(state)
	if credential.State != controlapi.MachineCredentialEnrolled && credential.State != controlapi.MachineCredentialOffline && credential.State != controlapi.MachineCredentialRevoked {
		return controlapi.MachineCredential{}, fmt.Errorf("worker credential state is invalid")
	}
	return credential, nil
}

func (store *Store) RecordWorkerReadiness(ctx context.Context, command controlapi.WorkerReadinessCommand) (controlapi.WorkerReadinessResult, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerReadinessResult{}, err
	}
	if err := validateWorkerReadiness(command); err != nil {
		return controlapi.WorkerReadinessResult{}, err
	}
	commandDigest, err := workerReadinessDigest(command)
	if err != nil {
		return controlapi.WorkerReadinessResult{}, err
	}

	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimWorkerIdempotency(ctx, tx, accountID, workerReadinessScope, command.IdempotencyKey,
			commandDigest, "worker_capability_revision", command.CapabilityRevisionID, command.ObservedAt)
		if err != nil {
			return err
		}
		if claimed {
			if _, err := tx.exec(ctx, `insert into fort_private.worker_capability_revision (
  account_id, capability_revision_id, worker_id, revision,
  capability_evidence, evidence_digest, observed_at
) values ($1, $2, $3, $4, $5::jsonb, $6, $7)`, accountID, command.CapabilityRevisionID,
				command.WorkerID, command.Revision, string(command.CapabilityEvidence), command.EvidenceDigest,
				command.ObservedAt.UTC()); err != nil {
				return fmt.Errorf("insert worker capability revision: %w", err)
			}
		}
		affected, err := tx.exec(ctx, `update fort_private.worker
set state = 'enrolled', last_seen_at = greatest(coalesce(last_seen_at, $1), $1), updated_at = $1
where account_id = $2 and worker_id = $3 and machine_id = $4 and state <> 'revoked'`,
			command.ObservedAt.UTC(), accountID, command.WorkerID, command.MachineID)
		if err != nil {
			return fmt.Errorf("update worker readiness: %w", err)
		}
		if affected != 1 {
			return controlapi.ErrWorkerRevoked
		}
		return nil
	})
	if err != nil {
		return controlapi.WorkerReadinessResult{}, err
	}
	return controlapi.WorkerReadinessResult{
		Status: "ready", CapabilityRevisionID: command.CapabilityRevisionID, ObservedAt: command.ObservedAt.UTC(),
	}, nil
}

func (store *Store) ClaimWorkerTarget(ctx context.Context, command controlapi.WorkerClaimCommand) (controlapi.WorkerAssignment, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerAssignment{}, err
	}
	if err := validateWorkerClaim(command); err != nil {
		return controlapi.WorkerAssignment{}, err
	}
	commandDigest, err := workerClaimDigest(command)
	if err != nil {
		return controlapi.WorkerAssignment{}, err
	}
	return store.claimWorkerTarget(ctx, accountID, command, workerClaimScope, commandDigest, loadClaimCandidate)
}

func (store *Store) ClaimNextWorkerTarget(ctx context.Context, next controlapi.WorkerClaimNextCommand) (controlapi.WorkerAssignment, error) {
	accountID, err := store.operationAccount(next.AccountID)
	if err != nil {
		return controlapi.WorkerAssignment{}, err
	}
	if err := validateWorkerClaimNext(next); err != nil {
		return controlapi.WorkerAssignment{}, err
	}
	commandDigest, err := workerClaimNextDigest(next)
	if err != nil {
		return controlapi.WorkerAssignment{}, err
	}
	command := controlapi.WorkerClaimCommand{
		AccountID: accountID, WorkerID: next.WorkerID, MachineID: next.MachineID,
		ExecutionAttemptID: next.ExecutionAttemptID, LeaseID: next.LeaseID,
		IdempotencyKey: next.IdempotencyKey, CapabilityRevisionID: next.CapabilityRevisionID,
		ClaimedAt: next.ClaimedAt, ExpiresAt: next.ExpiresAt,
	}
	loader := func(ctx context.Context, tx transaction, command controlapi.WorkerClaimCommand) (workerClaimCandidate, error) {
		candidate, replayed, err := loadReplayedNextClaim(ctx, tx, command, commandDigest)
		if err != nil || replayed {
			return candidate, err
		}
		return loadNextClaimCandidate(ctx, tx, command)
	}
	return store.claimWorkerTarget(ctx, accountID, command, workerClaimNextScope, commandDigest, loader)
}

type workerClaimLoader func(context.Context, transaction, controlapi.WorkerClaimCommand) (workerClaimCandidate, error)

func (store *Store) claimWorkerTarget(ctx context.Context, accountID string, command controlapi.WorkerClaimCommand,
	scope, commandDigest string, load workerClaimLoader) (controlapi.WorkerAssignment, error) {

	var assignment controlapi.WorkerAssignment
	var rejected error
	err := store.withTransaction(ctx, accountID, func(tx transaction) error {
		row, err := load(ctx, tx, command)
		if errors.Is(err, pgx.ErrNoRows) {
			return controlapi.ErrWorkerNoCompatibleTarget
		}
		if err != nil {
			return err
		}
		command.TargetID = row.targetID
		if row.replayed {
			assignment, err = store.prepareWorkerAssignment(command, row)
			if err != nil {
				return err
			}
			assignment.FenceToken = row.replayFence
			assignment.ClaimedAt = row.replayClaimedAt
			assignment.ExpiresAt = row.replayExpiresAt
			return nil
		}

		if row.state != "queued" {
			assignment, err = store.prepareWorkerAssignment(command, row)
			if err != nil {
				return err
			}
			fence, claimedAt, expiresAt, replayErr := replayWorkerClaim(ctx, tx, command, commandDigest)
			if replayErr != nil {
				return replayErr
			}
			assignment.FenceToken, assignment.ClaimedAt, assignment.ExpiresAt = fence, claimedAt, expiresAt
			return nil
		}
		if !command.ClaimedAt.Before(row.hardDeadline) {
			if _, err := tx.exec(ctx, `update fort_private.conversation_target
set state = 'needs_you', error_code = 'hard_deadline_elapsed', updated_at = $1
where account_id = $2 and target_id = $3 and state = 'queued'`, command.ClaimedAt.UTC(), accountID, command.TargetID); err != nil {
				return err
			}
			rejected = controlapi.ErrWorkerNoCompatibleTarget
			return nil
		}
		if row.targetKind == "routine" {
			if !row.scheduledFor.Valid || !row.occurrenceState.Valid || row.occurrenceState.String != "queued" {
				return controlapi.ErrWorkerNoCompatibleTarget
			}
			due := row.scheduledFor.Time.UTC()
			if command.ClaimedAt.Before(due) {
				return controlapi.ErrWorkerNoCompatibleTarget
			}
			if command.ClaimedAt.Sub(due) > controlapi.MaximumScheduleLateness {
				if _, err := tx.exec(ctx, `update fort_private.routine_occurrence
set state = 'missed_needs_attention', updated_at = $1
where account_id = $2 and routine_occurrence_id = $3 and state = 'queued'`,
					command.ClaimedAt.UTC(), accountID, row.routineOccurrenceID.String); err != nil {
					return err
				}
				if _, err := tx.exec(ctx, `update fort_private.routine_run
set state = 'needs_you', failure_code = 'routine_late',
    next_action = '{"kind":"review_missed_occurrence"}'::jsonb, updated_at = $1
where account_id = $2 and routine_occurrence_id = $3 and state = 'queued'`,
					command.ClaimedAt.UTC(), accountID, row.routineOccurrenceID.String); err != nil {
					return err
				}
				if _, err := tx.exec(ctx, `update fort_private.conversation_target
set state = 'needs_you', error_code = 'routine_late', updated_at = $1
where account_id = $2 and target_id = $3 and state = 'queued'`,
					command.ClaimedAt.UTC(), accountID, command.TargetID); err != nil {
					return err
				}
				rejected = controlapi.ErrWorkerNoCompatibleTarget
				return nil
			}
		}
		assignment, err = store.prepareWorkerAssignment(command, row)
		if err != nil {
			return err
		}

		claimed, err := claimWorkerIdempotency(ctx, tx, accountID, scope, command.IdempotencyKey,
			commandDigest, "worker_lease", command.LeaseID, command.ClaimedAt)
		if err != nil {
			return err
		}
		if !claimed {
			return fmt.Errorf("worker claim replay did not find an existing target lease")
		}
		attemptNumber := row.attemptCount + 1
		if _, err := tx.exec(ctx, `insert into fort_private.execution_attempt (
  account_id, execution_attempt_id, target_id, attempt_number,
  agent_id, behavior_revision_id, binding_revision_id, participant_id,
  worker_id, worker_capability_revision_id, state, created_at, updated_at
) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'leased', $11, $11)`,
			accountID, command.ExecutionAttemptID, command.TargetID, attemptNumber,
			row.agentID, row.behaviorRevisionID, row.bindingRevisionID, row.participantID,
			command.WorkerID, command.CapabilityRevisionID, command.ClaimedAt.UTC()); err != nil {
			return fmt.Errorf("insert execution attempt: %w", err)
		}
		if err := tx.queryRow(ctx, `insert into fort_private.worker_lease (
  account_id, lease_id, worker_id, execution_attempt_id, target_id,
  agent_id, behavior_revision_id, binding_revision_id, state,
  claimed_at, heartbeat_at, expires_at, updated_at
) values ($1, $2, $3, $4, $5, $6, $7, $8, 'active', $9, $9, $10, $9)
returning fence_token`, accountID, command.LeaseID, command.WorkerID, command.ExecutionAttemptID,
			command.TargetID, row.agentID, row.behaviorRevisionID, row.bindingRevisionID,
			command.ClaimedAt.UTC(), command.ExpiresAt.UTC()).scan(&assignment.FenceToken); err != nil {
			return fmt.Errorf("insert worker lease: %w", err)
		}
		affected, err := tx.exec(ctx, `update fort_private.conversation_target
set state = 'claimed', attempt_count = $1, updated_at = $2
where account_id = $3 and target_id = $4 and state = 'queued'`, attemptNumber, command.ClaimedAt.UTC(), accountID, command.TargetID)
		if err != nil {
			return err
		}
		if affected != 1 {
			return controlapi.ErrWorkerNoCompatibleTarget
		}
		eventMetadata, err := json.Marshal(struct {
			LeaseID string `json:"lease_id"`
		}{LeaseID: command.LeaseID})
		if err != nil {
			return fmt.Errorf("encode worker claim event: %w", err)
		}
		if _, err := tx.exec(ctx, `insert into fort_private.ledger_event (
  account_id, aggregate_kind, aggregate_id, event_type, target_id,
  execution_attempt_id, worker_id, event_metadata, created_at
) values ($1, 'target', $2, 'worker_target_claimed', $2, $3, $4, $5::jsonb, $6)`,
			accountID, command.TargetID, command.ExecutionAttemptID, command.WorkerID,
			string(eventMetadata), command.ClaimedAt.UTC()); err != nil {
			return fmt.Errorf("append worker claim event: %w", err)
		}
		return nil
	})
	if err != nil {
		return controlapi.WorkerAssignment{}, err
	}
	if rejected != nil {
		return controlapi.WorkerAssignment{}, rejected
	}
	return assignment, nil
}

func (store *Store) HeartbeatWorkerLease(ctx context.Context, command controlapi.WorkerLeaseHeartbeatCommand) (controlapi.WorkerLeaseHeartbeatResult, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerLeaseHeartbeatResult{}, err
	}
	if err := validateWorkerLeaseHeartbeat(command); err != nil {
		return controlapi.WorkerLeaseHeartbeatResult{}, err
	}
	digest, err := workerLeaseHeartbeatDigest(command)
	if err != nil {
		return controlapi.WorkerLeaseHeartbeatResult{}, err
	}
	result := controlapi.WorkerLeaseHeartbeatResult{
		TargetID: command.TargetID, ExecutionAttemptID: command.ExecutionAttemptID,
		LeaseID: command.LeaseID, FenceToken: command.FenceToken,
	}
	var rejected error
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		var leaseState string
		var currentExpiry time.Time
		var attemptState, targetState, targetKind, originID, turnID string
		var attemptNumber int
		var authorAgentID, behaviorID, bindingID string
		err := tx.queryRow(ctx, `select lease.state, lease.expires_at, attempt.state, target.state,
  target.target_kind,target.origin_id,target.turn_id,attempt.attempt_number,
  pinned.agent_id,pinned.behavior_revision_id,pinned.binding_revision_id
from fort_private.worker_lease as lease
join fort_private.execution_attempt as attempt
  on attempt.account_id = lease.account_id and attempt.execution_attempt_id = lease.execution_attempt_id
join fort_private.conversation_target as target
  on target.account_id = lease.account_id and target.target_id = lease.target_id
join fort_private.conversation_target_binding as pinned
  on pinned.account_id=target.account_id and pinned.target_id=target.target_id
join fort_private.worker as machine
  on machine.account_id = lease.account_id and machine.worker_id = lease.worker_id
where lease.account_id = $1 and lease.lease_id = $2 and lease.execution_attempt_id = $3
  and lease.target_id = $4 and lease.worker_id = $5 and lease.fence_token = $6
  and machine.machine_id = $7 and machine.state <> 'revoked'
  and $8 < target.hard_deadline
for update of lease`, accountID, command.LeaseID, command.ExecutionAttemptID, command.TargetID,
			command.WorkerID, command.FenceToken, command.MachineID, command.ObservedAt.UTC()).scan(
			&leaseState, &currentExpiry, &attemptState, &targetState, &targetKind, &originID, &turnID,
			&attemptNumber, &authorAgentID, &behaviorID, &bindingID)
		if errors.Is(err, pgx.ErrNoRows) {
			return controlapi.ErrWorkerStaleLease
		}
		if err != nil {
			return err
		}
		if leaseState != "active" {
			return controlapi.ErrWorkerStaleLease
		}
		if !command.ObservedAt.Before(currentExpiry) {
			if _, err := tx.exec(ctx, `update fort_private.worker_lease
set state = 'expired', released_at = $1, updated_at = $1
where account_id = $2 and lease_id = $3 and execution_attempt_id = $4
  and worker_id = $5 and fence_token = $6 and state = 'active'`, command.ObservedAt.UTC(), accountID,
				command.LeaseID, command.ExecutionAttemptID, command.WorkerID, command.FenceToken); err != nil {
				return err
			}
			if _, err := tx.exec(ctx, `update fort_private.execution_attempt
set state = 'lease_expired', updated_at = $1
where account_id = $2 and execution_attempt_id = $3 and worker_id = $4
  and state in ('leased', 'working', 'cancel_requested')`, command.ObservedAt.UTC(), accountID,
				command.ExecutionAttemptID, command.WorkerID); err != nil {
				return err
			}
			if _, err := tx.exec(ctx, `update fort_private.conversation_target
set state = 'lease_expired', error_code = 'worker_lease_expired', updated_at = $1
where account_id = $2 and target_id = $3
  and state in ('claimed', 'working', 'cancel_requested')`, command.ObservedAt.UTC(), accountID, command.TargetID); err != nil {
				return err
			}
			rejected = controlapi.ErrWorkerStaleLease
			return nil
		}

		result.Directive = coreworker.DirectiveContinue
		if attemptState == "cancel_requested" || targetState == "cancel_requested" {
			result.Directive = coreworker.DirectiveCancel
		}
		claimed, err := claimWorkerIdempotency(ctx, tx, accountID, "worker.lease_heartbeat", command.IdempotencyKey,
			digest, "worker_lease", command.LeaseID, command.ObservedAt)
		if err != nil {
			return err
		}
		if !claimed {
			result.ExpiresAt = currentExpiry.UTC()
			return nil
		}
		if result.Directive == coreworker.DirectiveContinue && targetKind == "handoff" {
			stopped, err := startWorkerHandoffAggregate(ctx, tx, accountID, workerAggregateEvidence{
				targetID: command.TargetID, originID: originID, turnID: turnID,
				attemptID: command.ExecutionAttemptID, attemptNumber: attemptNumber, leaseID: command.LeaseID,
				authorAgentID: authorAgentID, behaviorID: behaviorID, bindingID: bindingID,
			}, command.ObservedAt)
			if err != nil {
				return err
			}
			if stopped {
				result.Directive = coreworker.DirectiveCancel
				result.ExpiresAt = currentExpiry.UTC()
				rejected = controlapi.ErrWorkerStaleLease
				return nil
			}
		} else if result.Directive == coreworker.DirectiveContinue && targetKind == "routine" {
			if err := markPostgresRoutineRunWorking(ctx, tx, accountID, command.TargetID,
				command.ExecutionAttemptID, command.LeaseID, currentExpiry.UTC(), command.ObservedAt.UTC()); err != nil {
				return err
			}
		}
		newExpiry := command.ExtendUntil.UTC()
		if currentExpiry.After(newExpiry) {
			newExpiry = currentExpiry.UTC()
		}
		affected, err := tx.exec(ctx, `update fort_private.worker_lease
set heartbeat_at = $1, expires_at = $2, updated_at = $1
where account_id = $3 and lease_id = $4 and execution_attempt_id = $5
  and worker_id = $6 and fence_token = $7 and state = 'active'`, command.ObservedAt.UTC(), newExpiry,
			accountID, command.LeaseID, command.ExecutionAttemptID, command.WorkerID, command.FenceToken)
		if err != nil {
			return err
		}
		if affected != 1 {
			return controlapi.ErrWorkerStaleLease
		}
		if result.Directive == coreworker.DirectiveContinue {
			if _, err := tx.exec(ctx, `update fort_private.execution_attempt
set state = 'working', started_at = coalesce(started_at, $1), updated_at = $1
where account_id = $2 and execution_attempt_id = $3 and worker_id = $4 and state in ('leased', 'working')`,
				command.ObservedAt.UTC(), accountID, command.ExecutionAttemptID, command.WorkerID); err != nil {
				return err
			}
			if _, err := tx.exec(ctx, `update fort_private.conversation_target
set state = 'working', updated_at = $1
where account_id = $2 and target_id = $3 and state in ('claimed', 'working')`,
				command.ObservedAt.UTC(), accountID, command.TargetID); err != nil {
				return err
			}
		}
		if _, err := tx.exec(ctx, `update fort_private.worker
set last_seen_at = greatest(coalesce(last_seen_at, $1), $1), updated_at = $1
where account_id = $2 and worker_id = $3 and machine_id = $4 and state <> 'revoked'`,
			command.ObservedAt.UTC(), accountID, command.WorkerID, command.MachineID); err != nil {
			return err
		}
		result.ExpiresAt = newExpiry
		return nil
	})
	if err != nil {
		return controlapi.WorkerLeaseHeartbeatResult{}, err
	}
	if rejected != nil {
		return controlapi.WorkerLeaseHeartbeatResult{}, rejected
	}
	return result, nil
}

func (store *Store) AcknowledgeWorkerCancellation(ctx context.Context, command controlapi.WorkerCancellationAckCommand) (controlapi.WorkerCancellationAck, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerCancellationAck{}, err
	}
	if err := validateWorkerCancellationAck(command); err != nil {
		return controlapi.WorkerCancellationAck{}, err
	}
	digest, err := workerCancellationAckDigest(command)
	if err != nil {
		return controlapi.WorkerCancellationAck{}, err
	}
	result := controlapi.WorkerCancellationAck{
		AcknowledgementID: command.AcknowledgementID, TargetID: command.TargetID,
		ExecutionAttemptID: command.ExecutionAttemptID, LeaseID: command.LeaseID,
		FenceToken: command.FenceToken, AcknowledgedAt: command.AcknowledgedAt.UTC(),
	}
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		var expiresAt time.Time
		var attemptState, targetState string
		err := tx.queryRow(ctx, `select lease.expires_at, attempt.state, target.state
from fort_private.worker_lease as lease
join fort_private.execution_attempt as attempt
  on attempt.account_id = lease.account_id and attempt.execution_attempt_id = lease.execution_attempt_id
join fort_private.conversation_target as target
  on target.account_id = lease.account_id and target.target_id = lease.target_id
join fort_private.worker as machine
  on machine.account_id = lease.account_id and machine.worker_id = lease.worker_id
where lease.account_id = $1 and lease.lease_id = $2 and lease.execution_attempt_id = $3
  and lease.target_id = $4 and lease.worker_id = $5 and lease.fence_token = $6
  and machine.machine_id = $7 and machine.state <> 'revoked' and lease.state = 'active'
for update of lease`, accountID, command.LeaseID, command.ExecutionAttemptID, command.TargetID,
			command.WorkerID, command.FenceToken, command.MachineID).scan(&expiresAt, &attemptState, &targetState)
		if errors.Is(err, pgx.ErrNoRows) {
			return controlapi.ErrWorkerStaleLease
		}
		if err != nil {
			return err
		}
		if !command.AcknowledgedAt.Before(expiresAt) || attemptState != "cancel_requested" || targetState != "cancel_requested" {
			return controlapi.ErrWorkerStaleLease
		}
		claimed, err := claimWorkerIdempotency(ctx, tx, accountID, "worker.cancel_ack", command.IdempotencyKey,
			digest, "worker_cancellation_ack", command.AcknowledgementID, command.AcknowledgedAt)
		if err != nil {
			return err
		}
		if !claimed {
			var fence int64
			if err := tx.queryRow(ctx, `select cancellation_ack_id, target_id, execution_attempt_id,
  lease_id, fence_token, acknowledged_at
from fort_private.worker_cancellation_ack
where account_id = $1 and worker_id = $2 and idempotency_key = $3`, accountID, command.WorkerID,
				command.IdempotencyKey).scan(&result.AcknowledgementID, &result.TargetID, &result.ExecutionAttemptID,
				&result.LeaseID, &fence, &result.AcknowledgedAt); err != nil {
				return err
			}
			result.FenceToken = fence
			return nil
		}
		if _, err := tx.exec(ctx, `insert into fort_private.worker_cancellation_ack (
  account_id, cancellation_ack_id, target_id, execution_attempt_id,
  lease_id, fence_token, worker_id, machine_id, idempotency_key, acknowledged_at
) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, accountID,
			command.AcknowledgementID, command.TargetID, command.ExecutionAttemptID,
			command.LeaseID, command.FenceToken, command.WorkerID, command.MachineID,
			command.IdempotencyKey, command.AcknowledgedAt.UTC()); err != nil {
			return fmt.Errorf("insert worker cancellation acknowledgement: %w", err)
		}
		return nil
	})
	return result, err
}

func (store *Store) CommitWorkerTerminal(ctx context.Context, command controlapi.WorkerTerminalCommand) (controlapi.WorkerTerminalResult, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerTerminalResult{}, err
	}
	if err := validateWorkerTerminal(command); err != nil {
		return controlapi.WorkerTerminalResult{}, err
	}
	digest, err := workerTerminalDigest(command)
	if err != nil {
		return controlapi.WorkerTerminalResult{}, err
	}
	result := controlapi.WorkerTerminalResult{
		TargetID: command.TargetID, ExecutionAttemptID: command.ExecutionAttemptID,
		LeaseID: command.LeaseID, FenceToken: command.FenceToken,
		TerminalReceiptID: command.TerminalReceiptID, Status: command.Status,
		Output: command.Output, CommittedAt: command.CommittedAt.UTC(),
	}
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		var leaseState string
		var expiresAt time.Time
		var attemptState, targetState string
		var targetKind, conversationID, turnID, originID, runID, authorAgentID, behaviorID, bindingID string
		var attemptNumber int
		var cancellationAcked bool
		var terminalAt sql.NullTime
		var terminalReceiptID sql.NullString
		err := tx.queryRow(ctx, `select lease.state, lease.expires_at, attempt.state, target.state,
  exists (
    select 1 from fort_private.worker_cancellation_ack as ack
    where ack.account_id = lease.account_id
      and ack.execution_attempt_id = lease.execution_attempt_id
      and ack.lease_id = lease.lease_id and ack.fence_token = lease.fence_token
	  ), attempt.terminal_at, attempt.terminal_receipt_id,
	  target.target_kind, target.conversation_id, target.turn_id, target.origin_id, target.run_id,
	  attempt.attempt_number,pinned.agent_id,pinned.behavior_revision_id,pinned.binding_revision_id
from fort_private.worker_lease as lease
join fort_private.execution_attempt as attempt
  on attempt.account_id = lease.account_id and attempt.execution_attempt_id = lease.execution_attempt_id
join fort_private.conversation_target as target
  on target.account_id = lease.account_id and target.target_id = lease.target_id
join fort_private.conversation_target_binding as pinned
  on pinned.account_id = target.account_id and pinned.target_id = target.target_id
join fort_private.worker as machine
  on machine.account_id = lease.account_id and machine.worker_id = lease.worker_id
where lease.account_id = $1 and lease.lease_id = $2 and lease.execution_attempt_id = $3
  and lease.target_id = $4 and lease.worker_id = $5 and lease.fence_token = $6
  and machine.machine_id = $7 and machine.state <> 'revoked'
  and (lease.state <> 'active' or $8 < target.hard_deadline)
for update of lease`, accountID, command.LeaseID, command.ExecutionAttemptID, command.TargetID,
			command.WorkerID, command.FenceToken, command.MachineID, command.CommittedAt.UTC()).scan(&leaseState, &expiresAt, &attemptState,
			&targetState, &cancellationAcked, &terminalAt, &terminalReceiptID,
			&targetKind, &conversationID, &turnID, &originID, &runID, &attemptNumber,
			&authorAgentID, &behaviorID, &bindingID)
		if errors.Is(err, pgx.ErrNoRows) {
			return controlapi.ErrWorkerStaleLease
		}
		if err != nil {
			return err
		}
		if leaseState != "active" {
			if !terminalAt.Valid || !terminalReceiptID.Valid || terminalReceiptID.String != command.TerminalReceiptID ||
				!terminalStateMatches(command.Status, attemptState) {
				return controlapi.ErrWorkerStaleLease
			}
			if err := checkWorkerIdempotency(ctx, tx, accountID, "worker.terminal", command.IdempotencyKey,
				digest, "terminal_receipt", command.TerminalReceiptID); err != nil {
				return err
			}
			if command.Status == coreworker.TerminalCompleted {
				messageQuery, aggregateID, messageKind := `select message_id
from fort_private.conversation_message
where account_id = $1 and target_id = $2 and message_kind = 'agent'
	and author_kind = 'agent' and author_agent_id = $3`, "", "Agent"
				switch targetKind {
				case "handoff":
					messageQuery = `select message_id from fort_private.conversation_message
where account_id=$1 and target_id=$2 and author_agent_id=$3
  and message_kind='handoff_result' and handoff_id=$4`
					aggregateID, messageKind = originID, "Handoff"
				case "routine":
					messageQuery = `select message_id from fort_private.conversation_message
where account_id=$1 and target_id=$2 and author_agent_id=$3
  and message_kind='routine_result' and routine_run_id=$4`
					aggregateID, messageKind = runID, "Routine"
				}
				var messageErr error
				if aggregateID == "" {
					messageErr = tx.queryRow(ctx, messageQuery, accountID, command.TargetID, authorAgentID).scan(&result.MessageID)
				} else {
					messageErr = tx.queryRow(ctx, messageQuery, accountID, command.TargetID, authorAgentID, aggregateID).scan(&result.MessageID)
				}
				if messageErr != nil {
					if errors.Is(messageErr, pgx.ErrNoRows) {
						return fmt.Errorf("completed worker terminal is missing its authoritative %s message", messageKind)
					}
					return messageErr
				}
			}
			result.CommittedAt, result.Created = terminalAt.Time.UTC(), false
			return nil
		}
		if !command.CommittedAt.Before(expiresAt) {
			return controlapi.ErrWorkerStaleLease
		}
		if command.Status == coreworker.TerminalCanceled {
			if attemptState != "cancel_requested" || targetState != "cancel_requested" || !cancellationAcked {
				return controlapi.ErrWorkerStaleLease
			}
		} else if attemptState == "cancel_requested" || targetState == "cancel_requested" {
			return controlapi.ErrWorkerStaleLease
		} else if (attemptState != "leased" && attemptState != "working") || (targetState != "claimed" && targetState != "working") {
			return controlapi.ErrWorkerStaleLease
		}
		var artifactDigest string
		var artifactPlaintextLength int64
		if err := tx.queryRow(ctx, `select logical_digest,expected_plaintext_length
from fort_private.artifact
where account_id = $1 and artifact_id = $2 and execution_attempt_id = $3
  and kind = 'output' and state = 'finalized'`, accountID, command.Output.ArtifactID,
			command.ExecutionAttemptID).scan(&artifactDigest, &artifactPlaintextLength); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("terminal output artifact is not finalized")
			}
			return err
		}
		if artifactDigest != command.Output.Digest {
			return fmt.Errorf("terminal output digest does not match finalized artifact")
		}
		var message collaborationEncryptedBody
		var outputBody string
		if command.Status == coreworker.TerminalCompleted {
			outputBody, err = workerOutputMessageBody(command, artifactPlaintextLength)
			if err != nil {
				return err
			}
			switch targetKind {
			case "initial":
				message, err = store.prepareWorkerOutputMessage(accountID, command, artifactPlaintextLength)
			case "handoff":
				message, err = store.prepareWorkerOutputMessageForScope(accountID, command,
					artifactPlaintextLength, "handoff_result", originID)
			}
			if err != nil {
				return err
			}
		}
		receipt, err := store.prepareWorkerTerminalReceipt(accountID, command)
		if err != nil {
			return err
		}
		command.Receipt = controlapi.EncryptedEnvelope{
			Ciphertext: receipt.Ciphertext, KeyID: receipt.KeyID, Nonce: receipt.Nonce,
			Digest: receipt.Digest, PlaintextLength: receipt.PlaintextBytes,
		}
		claimed, err := claimWorkerIdempotency(ctx, tx, accountID, "worker.terminal", command.IdempotencyKey,
			digest, "terminal_receipt", command.TerminalReceiptID, command.CommittedAt)
		if err != nil {
			return err
		}
		if !claimed {
			return fmt.Errorf("worker terminal replay has an active lease")
		}
		aggregate := workerAggregateEvidence{
			targetID: command.TargetID, originID: originID, runID: runID,
			conversationID: conversationID, turnID: turnID, attemptID: command.ExecutionAttemptID,
			attemptNumber: attemptNumber, leaseID: command.LeaseID, authorAgentID: authorAgentID,
			behaviorID: behaviorID, bindingID: bindingID,
		}
		if command.Status == coreworker.TerminalCompleted && targetKind == "initial" {
			if err := tx.queryRow(ctx, `insert into fort_private.conversation_message (
  account_id, conversation_id, turn_id, target_id, message_kind, author_kind,
  author_id, author_agent_id, body_ciphertext, body_key_id, body_nonce,
  body_digest, body_plaintext_length, created_at
) values ($1,$2,$3,$4,'agent','agent',$5,$5,$6,$7,$8,$9,$10,$11)
returning message_id`, accountID, conversationID, turnID, command.TargetID, authorAgentID,
				message.Ciphertext, message.KeyID, message.Nonce, message.Digest,
				message.PlaintextBytes, command.CommittedAt.UTC()).scan(&result.MessageID); err != nil {
				return fmt.Errorf("insert authoritative Agent message: %w", err)
			}
		} else if targetKind == "handoff" {
			result.MessageID, err = commitWorkerHandoffAggregate(ctx, tx, accountID, aggregate, command, message)
			if err != nil {
				return err
			}
		} else if targetKind == "routine" {
			if err := store.commitPostgresRoutineRunTerminal(ctx, tx, accountID, command.TargetID,
				command.ExecutionAttemptID, command.LeaseID, command.Status, outputBody,
				command.CommittedAt.UTC()); err != nil {
				return err
			}
			if command.Status == coreworker.TerminalCompleted {
				if err := tx.queryRow(ctx, `select message_id from fort_private.conversation_message
where account_id=$1 and target_id=$2 and routine_run_id=$3 and message_kind='routine_result'`,
					accountID, command.TargetID, runID).scan(&result.MessageID); err != nil {
					return fmt.Errorf("load authoritative Routine result message: %w", err)
				}
			}
		}

		databaseState := workerTerminalDatabaseState(command.Status)
		attemptSQL := `update fort_private.execution_attempt
set state = '` + databaseState + `', provider_terminal_status = '` + databaseState + `',
    terminal_receipt_id = $1, terminal_receipt_ciphertext = $2, terminal_receipt_key_id = $3,
    terminal_receipt_nonce = $4, terminal_receipt_digest = $5,
    terminal_at = $6, updated_at = $6
where account_id = $7 and execution_attempt_id = $8 and target_id = $9
  and worker_id = $10 and terminal_receipt_id is null and terminal_receipt_ciphertext is null`
		affected, err := tx.exec(ctx, attemptSQL, command.TerminalReceiptID, command.Receipt.Ciphertext,
			command.Receipt.KeyID, command.Receipt.Nonce, command.Receipt.Digest, command.CommittedAt.UTC(),
			accountID, command.ExecutionAttemptID, command.TargetID, command.WorkerID)
		if err != nil {
			return err
		}
		if affected != 1 {
			return controlapi.ErrWorkerStaleLease
		}
		targetSQL := `update fort_private.conversation_target
set state = '` + databaseState + `', updated_at = $1
where account_id = $2 and target_id = $3`
		if affected, err := tx.exec(ctx, targetSQL, command.CommittedAt.UTC(), accountID, command.TargetID); err != nil {
			return err
		} else if affected != 1 {
			return controlapi.ErrWorkerStaleLease
		}
		if affected, err := tx.exec(ctx, `update fort_private.worker_lease
set state = 'released', released_at = $1, updated_at = $1
where account_id = $2 and lease_id = $3 and execution_attempt_id = $4
  and target_id = $5 and worker_id = $6 and fence_token = $7 and state = 'active'`,
			command.CommittedAt.UTC(), accountID, command.LeaseID, command.ExecutionAttemptID,
			command.TargetID, command.WorkerID, command.FenceToken); err != nil {
			return err
		} else if affected != 1 {
			return controlapi.ErrWorkerStaleLease
		}
		if result.MessageID != 0 {
			if affected, err := tx.exec(ctx, `update fort_private.conversation
set updated_at = $3 where account_id = $1 and conversation_id = $2`, accountID,
				conversationID, command.CommittedAt.UTC()); err != nil {
				return err
			} else if affected != 1 {
				return fmt.Errorf("authoritative Agent message Conversation does not exist")
			}
		}
		if targetKind == "initial" {
			if err := settleConversationTurnIfTerminal(ctx, tx, accountID, turnID, command.CommittedAt); err != nil {
				return err
			}
		}
		eventMetadata, err := json.Marshal(struct {
			OutputArtifactID    string `json:"output_artifact_id"`
			TerminalReceiptID   string `json:"terminal_receipt_id"`
			ConversationMessage int64  `json:"conversation_message_id,omitempty"`
		}{OutputArtifactID: command.Output.ArtifactID, TerminalReceiptID: command.TerminalReceiptID,
			ConversationMessage: result.MessageID})
		if err != nil {
			return fmt.Errorf("encode worker terminal event: %w", err)
		}
		if _, err := tx.exec(ctx, `insert into fort_private.ledger_event (
  account_id, aggregate_kind, aggregate_id, event_type, target_id,
  execution_attempt_id, worker_id, event_metadata, created_at
) values ($1, 'execution_attempt', $2, 'worker_terminal_committed', $3, $2, $4, $5::jsonb, $6)`,
			accountID, command.ExecutionAttemptID, command.TargetID, command.WorkerID,
			string(eventMetadata),
			command.CommittedAt.UTC()); err != nil {
			return err
		}
		result.Created = true
		return nil
	})
	return result, err
}

func validateWorkerTerminal(command controlapi.WorkerTerminalCommand) error {
	if !validWorkerIdentifier(command.WorkerID) || !validWorkerIdentifier(command.MachineID) ||
		!validWorkerIdentifier(command.TargetID) || !validWorkerIdentifier(command.ExecutionAttemptID) ||
		!validWorkerIdentifier(command.LeaseID) || !validWorkerIdentifier(command.TerminalReceiptID) ||
		!validWorkerIdentifier(command.IdempotencyKey) ||
		command.FenceToken < 1 || command.CommittedAt.IsZero() ||
		(command.Status != coreworker.TerminalCompleted && command.Status != coreworker.TerminalFailed && command.Status != coreworker.TerminalCanceled) ||
		!validWorkerTerminalReceipt(command.ReceiptPlaintext) || !validWorkerIdentifier(command.Output.ArtifactID) || !lowerWorkerDigest(command.Output.Digest) ||
		!validWorkerOutputMessage(command.Status, command.Output.Digest, command.OutputMessagePlaintext) {
		return fmt.Errorf("%w: worker terminal command is invalid", controlapi.ErrWorkerRequestInvalid)
	}
	return nil
}

func validWorkerTerminalReceipt(receipt json.RawMessage) bool {
	if len(receipt) == 0 || len(receipt) > 64<<10 {
		return false
	}
	var object map[string]any
	return json.Unmarshal(receipt, &object) == nil && object != nil
}

func validWorkerOutputMessage(status coreworker.TerminalStatus, outputDigest string, message *string) bool {
	if status != coreworker.TerminalCompleted {
		return message == nil
	}
	if message == nil {
		return true
	}
	if len([]byte(*message)) > controlapi.MaximumArtifactChunkPlaintextBytes {
		return false
	}
	digest := sha256.Sum256([]byte(*message))
	return hex.EncodeToString(digest[:]) == outputDigest
}

const workerOutputArtifactMessageType = "fort.output_artifact"

type workerOutputArtifactMessage struct {
	Type            string `json:"type"`
	ArtifactID      string `json:"artifact_id"`
	Digest          string `json:"digest"`
	PlaintextLength int64  `json:"plaintext_length"`
}

func (store *Store) prepareWorkerOutputMessage(accountID string, command controlapi.WorkerTerminalCommand,
	artifactPlaintextLength int64) (collaborationEncryptedBody, error) {
	return store.prepareWorkerOutputMessageForScope(accountID, command, artifactPlaintextLength,
		"conversation_message", command.TargetID)
}

func workerOutputMessageBody(command controlapi.WorkerTerminalCommand, artifactPlaintextLength int64) (string, error) {
	if artifactPlaintextLength <= controlapi.MaximumArtifactChunkPlaintextBytes {
		if command.OutputMessagePlaintext == nil {
			return "", fmt.Errorf("%w: completed output at or below the inline limit requires its plaintext message", controlapi.ErrWorkerRequestInvalid)
		}
		return *command.OutputMessagePlaintext, nil
	} else {
		if command.OutputMessagePlaintext != nil {
			return "", fmt.Errorf("%w: completed output above the inline limit must use its artifact reference", controlapi.ErrWorkerRequestInvalid)
		}
		reference, err := json.Marshal(workerOutputArtifactMessage{
			Type: workerOutputArtifactMessageType, ArtifactID: command.Output.ArtifactID,
			Digest: command.Output.Digest, PlaintextLength: artifactPlaintextLength,
		})
		if err != nil {
			return "", err
		}
		return string(reference), nil
	}
}

func (store *Store) prepareWorkerOutputMessageForScope(accountID string, command controlapi.WorkerTerminalCommand,
	artifactPlaintextLength int64, recordType, recordID string) (collaborationEncryptedBody, error) {
	body, err := workerOutputMessageBody(command, artifactPlaintextLength)
	if err != nil {
		return collaborationEncryptedBody{}, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return collaborationEncryptedBody{}, err
	}
	persisted, err := cipher.seal(securebody.Scope{AccountID: accountID,
		RecordType: recordType, RecordID: recordID}, body)
	if err != nil {
		return collaborationEncryptedBody{}, fmt.Errorf("encrypt authoritative Agent message: %w", err)
	}
	if artifactPlaintextLength <= controlapi.MaximumArtifactChunkPlaintextBytes &&
		(persisted.Digest != command.Output.Digest || int64(persisted.PlaintextBytes) != artifactPlaintextLength) {
		return collaborationEncryptedBody{}, fmt.Errorf("%w: worker output message does not match finalized artifact", controlapi.ErrWorkerRequestInvalid)
	}
	return persisted, nil
}

func (store *Store) prepareWorkerTerminalReceipt(accountID string, command controlapi.WorkerTerminalCommand) (collaborationEncryptedBody, error) {
	cipher, err := store.collaborationBodies()
	if err != nil {
		return collaborationEncryptedBody{}, err
	}
	return cipher.seal(securebody.Scope{
		AccountID: accountID, RecordType: "worker_terminal_receipt",
		RecordID: controlapi.WorkerOutputMessageRecordID(command.TargetID, command.ExecutionAttemptID, command.FenceToken),
	}, string(command.ReceiptPlaintext))
}

func validWorkerEnvelope(envelope controlapi.EncryptedEnvelope) bool {
	return len(envelope.Ciphertext) > 0 && len(envelope.Ciphertext) <= 4<<20 &&
		validWorkerIdentifier(envelope.KeyID) && len(envelope.Nonce) >= 12 && len(envelope.Nonce) <= 64 &&
		lowerWorkerDigest(envelope.Digest) && envelope.PlaintextLength >= 0 && envelope.PlaintextLength <= 2<<20
}

func workerTerminalDatabaseState(status coreworker.TerminalStatus) string {
	switch status {
	case coreworker.TerminalCompleted:
		return "succeeded"
	case coreworker.TerminalFailed:
		return "failed"
	default:
		return "canceled"
	}
}

func terminalStateMatches(status coreworker.TerminalStatus, state string) bool {
	return workerTerminalDatabaseState(status) == state
}

func workerTerminalDigest(command controlapi.WorkerTerminalCommand) (string, error) {
	return workerCommandDigest(struct {
		AccountID, WorkerID, MachineID, TargetID, ExecutionAttemptID, LeaseID string
		FenceToken                                                            int64
		TerminalReceiptID, IdempotencyKey                                     string
		Status                                                                coreworker.TerminalStatus
		Receipt                                                               json.RawMessage
		Output                                                                controlapi.WorkerOutputReference
		OutputMessage                                                         *string
	}{command.AccountID, command.WorkerID, command.MachineID, command.TargetID, command.ExecutionAttemptID,
		command.LeaseID, command.FenceToken, command.TerminalReceiptID, command.IdempotencyKey,
		command.Status, command.ReceiptPlaintext, command.Output, command.OutputMessagePlaintext})
}

func checkWorkerIdempotency(ctx context.Context, tx transaction, accountID, scope, key, digest, resultKind, resultID string) error {
	var existingDigest, existingKind, existingID string
	if err := tx.queryRow(ctx, `select command_digest, result_kind, result_id
from fort_private.idempotency_record
where account_id = $1 and scope = $2 and idempotency_key = $3`, accountID, scope, key).scan(
		&existingDigest, &existingKind, &existingID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return controlapi.ErrWorkerStaleLease
		}
		return err
	}
	if existingDigest != digest || existingKind != resultKind || existingID != resultID {
		return controlapi.ErrWorkerIdempotencyConflict
	}
	return nil
}

func validateWorkerCancellationAck(command controlapi.WorkerCancellationAckCommand) error {
	if !validWorkerIdentifier(command.WorkerID) || !validWorkerIdentifier(command.MachineID) ||
		!validWorkerIdentifier(command.TargetID) || !validWorkerIdentifier(command.ExecutionAttemptID) ||
		!validWorkerIdentifier(command.LeaseID) || !validWorkerIdentifier(command.AcknowledgementID) ||
		!validWorkerIdentifier(command.IdempotencyKey) || command.FenceToken < 1 || command.AcknowledgedAt.IsZero() {
		return fmt.Errorf("worker cancellation acknowledgement command is invalid")
	}
	return nil
}

func workerCancellationAckDigest(command controlapi.WorkerCancellationAckCommand) (string, error) {
	return workerCommandDigest(struct {
		AccountID, WorkerID, MachineID, TargetID, ExecutionAttemptID, LeaseID string
		FenceToken                                                            int64
		AcknowledgementID, IdempotencyKey                                     string
	}{command.AccountID, command.WorkerID, command.MachineID, command.TargetID, command.ExecutionAttemptID,
		command.LeaseID, command.FenceToken, command.AcknowledgementID, command.IdempotencyKey})
}

func validateWorkerLeaseHeartbeat(command controlapi.WorkerLeaseHeartbeatCommand) error {
	if !validWorkerIdentifier(command.WorkerID) || !validWorkerIdentifier(command.MachineID) ||
		!validWorkerIdentifier(command.TargetID) || !validWorkerIdentifier(command.ExecutionAttemptID) ||
		!validWorkerIdentifier(command.LeaseID) || !validWorkerIdentifier(command.IdempotencyKey) ||
		command.FenceToken < 1 || command.ObservedAt.IsZero() || !command.ExtendUntil.After(command.ObservedAt) ||
		command.ExtendUntil.Sub(command.ObservedAt) > controlapi.DefaultWorkerLease {
		return fmt.Errorf("worker lease heartbeat command is invalid")
	}
	return nil
}

func workerLeaseHeartbeatDigest(command controlapi.WorkerLeaseHeartbeatCommand) (string, error) {
	return workerCommandDigest(struct {
		AccountID, WorkerID, MachineID, TargetID, ExecutionAttemptID, LeaseID, IdempotencyKey string
		FenceToken                                                                            int64
	}{command.AccountID, command.WorkerID, command.MachineID, command.TargetID, command.ExecutionAttemptID,
		command.LeaseID, command.IdempotencyKey, command.FenceToken})
}

type workerClaimCandidate struct {
	targetID                 string
	state                    string
	attemptCount             int64
	hardDeadline             time.Time
	targetKind               string
	originID                 string
	outputConversationID     string
	turnID                   string
	turnKind                 string
	agentID                  string
	behaviorRevisionID       string
	bindingRevisionID        string
	participantID            string
	seatID                   string
	participantAuthorityJSON []byte
	delegationGrantJSON      []byte
	delegationGrantID        string
	contextManifestID        string
	executionSourceID        string
	sourceAgentID            string
	opaqueSourceAgentID      string
	fortProfile              string
	provider                 string
	requestedModel           string
	resolvedModel            string
	adapterID                string
	adapterRevision          string
	sourceConfigDigest       string
	authorityID              string
	authorityRevision        string
	policyID                 string
	policyRevision           string
	sessionBehavior          string
	memoryBehavior           string
	capabilityEvidenceJSON   []byte
	readinessContractID      string
	readinessContractRev     string
	computerID               string
	promptCiphertext         []byte
	promptKeyID              string
	promptNonce              []byte
	promptDigest             string
	promptPlaintextSize      int64
	scheduledFor             sql.NullTime
	occurrenceState          sql.NullString
	routineOccurrenceID      sql.NullString
	replayed                 bool
	replayFence              int64
	replayClaimedAt          time.Time
	replayExpiresAt          time.Time
}

func loadClaimCandidate(ctx context.Context, tx transaction, command controlapi.WorkerClaimCommand) (workerClaimCandidate, error) {
	var candidate workerClaimCandidate
	err := tx.queryRow(ctx, `select
  target.target_id, target.state, target.attempt_count, target.hard_deadline, target.target_kind,
  target.origin_id,
  case
    when target.target_kind = 'handoff' then handoff.output_conversation_id
    when target.target_kind = 'routine' then run.result_conversation_id
    else target.conversation_id
  end as output_conversation_id,
  turn.turn_id, turn.kind,
  pinned.agent_id, pinned.behavior_revision_id, pinned.binding_revision_id,
  pinned.participant_id, binding.seat_id, participant.authority_snapshot,
  authority_grant.authority_grant, authority_grant.delegation_grant_id,
  turn.context_manifest_id, binding.execution_source_id, binding.source_agent_id,
  source_agent.opaque_source_agent_id, binding.fort_profile, binding.provider,
  binding.requested_model, binding.resolved_model, binding.adapter_id,
  binding.adapter_revision, binding.source_config_digest, binding.authority_id,
  binding.authority_revision, binding.policy_id, binding.policy_revision,
  binding.session_behavior, binding.memory_behavior, binding.capability_evidence,
  binding.readiness_contract_id, binding.readiness_contract_revision,
  machine.machine_id,
  case when target.target_kind = 'handoff' then handoff.requested_result_ciphertext else prompt.body_ciphertext end,
  case when target.target_kind = 'handoff' then handoff.requested_result_key_id else prompt.body_key_id end,
  case when target.target_kind = 'handoff' then handoff.requested_result_nonce else prompt.body_nonce end,
  case when target.target_kind = 'handoff' then handoff.requested_result_digest else prompt.body_digest end,
  case when target.target_kind = 'handoff' then -1 else prompt.body_plaintext_length end,
  occurrence.scheduled_for, occurrence.state, run.routine_occurrence_id
from fort_private.conversation_target as target
join fort_private.conversation_target_binding as pinned
  on pinned.account_id = target.account_id and pinned.target_id = target.target_id
join fort_private.agent_binding_revision as binding
  on binding.account_id = pinned.account_id and binding.agent_id = pinned.agent_id
 and binding.binding_revision_id = pinned.binding_revision_id
 and binding.worker_id = $2
join fort_private.worker as machine
  on machine.account_id = binding.account_id and machine.worker_id = binding.worker_id
 and machine.machine_id = $5 and machine.state = 'enrolled'
join fort_private.worker_capability_revision as capability
  on capability.account_id = machine.account_id and capability.worker_id = machine.worker_id
 and capability.capability_revision_id = $3
join fort_private.execution_source as source
  on source.account_id = binding.account_id and source.execution_source_id = binding.execution_source_id
 and source.worker_id = binding.worker_id
join fort_private.source_agent as source_agent
  on source_agent.account_id = binding.account_id and source_agent.execution_source_id = binding.execution_source_id
 and source_agent.source_agent_id = binding.source_agent_id
join fort_private.conversation_participant as participant
  on participant.account_id = pinned.account_id and participant.participant_id = pinned.participant_id
join fort_private.conversation_turn as turn
  on turn.account_id = target.account_id and turn.turn_id = target.turn_id
join fort_private.delegation_grant as authority_grant
  on authority_grant.account_id = turn.account_id
 and authority_grant.delegation_grant_id = turn.delegation_grant_id
join fort_private.conversation_message as prompt
  on prompt.account_id = turn.account_id and prompt.message_id = turn.prompt_message_id
left join fort_private.handoff as handoff
  on handoff.account_id = target.account_id and handoff.target_id = target.target_id
left join fort_private.routine_run as run
  on run.account_id = target.account_id and run.target_id = target.target_id
left join fort_private.routine_occurrence as occurrence
  on occurrence.account_id = run.account_id and occurrence.routine_occurrence_id = run.routine_occurrence_id
where target.account_id = $1 and target.target_id = $4
  and coalesce((select observation.source_config_digest
    from fort_private.execution_source_config_observation as observation
    where observation.account_id = binding.account_id
      and observation.execution_source_id = binding.execution_source_id
    order by observation.observation_sequence desc limit 1), source.source_config_digest) = binding.source_config_digest
for update of target`, command.AccountID, command.WorkerID, command.CapabilityRevisionID, command.TargetID, command.MachineID).scan(
		&candidate.targetID, &candidate.state, &candidate.attemptCount, &candidate.hardDeadline, &candidate.targetKind,
		&candidate.originID, &candidate.outputConversationID, &candidate.turnID, &candidate.turnKind,
		&candidate.agentID, &candidate.behaviorRevisionID, &candidate.bindingRevisionID,
		&candidate.participantID, &candidate.seatID, &candidate.participantAuthorityJSON,
		&candidate.delegationGrantJSON, &candidate.delegationGrantID,
		&candidate.contextManifestID, &candidate.executionSourceID, &candidate.sourceAgentID,
		&candidate.opaqueSourceAgentID, &candidate.fortProfile, &candidate.provider,
		&candidate.requestedModel, &candidate.resolvedModel, &candidate.adapterID,
		&candidate.adapterRevision, &candidate.sourceConfigDigest, &candidate.authorityID,
		&candidate.authorityRevision, &candidate.policyID, &candidate.policyRevision,
		&candidate.sessionBehavior, &candidate.memoryBehavior, &candidate.capabilityEvidenceJSON,
		&candidate.readinessContractID, &candidate.readinessContractRev, &candidate.computerID,
		&candidate.promptCiphertext, &candidate.promptKeyID,
		&candidate.promptNonce, &candidate.promptDigest, &candidate.promptPlaintextSize,
		&candidate.scheduledFor, &candidate.occurrenceState, &candidate.routineOccurrenceID,
	)
	return candidate, err
}

func loadNextClaimCandidate(ctx context.Context, tx transaction, command controlapi.WorkerClaimCommand) (workerClaimCandidate, error) {
	var targetID string
	err := tx.queryRow(ctx, `select target.target_id
from fort_private.conversation_target as target
join fort_private.conversation_target_binding as pinned
  on pinned.account_id = target.account_id and pinned.target_id = target.target_id
join fort_private.agent_binding_revision as binding
  on binding.account_id = pinned.account_id and binding.agent_id = pinned.agent_id
 and binding.binding_revision_id = pinned.binding_revision_id and binding.worker_id = $2
join fort_private.worker as machine
  on machine.account_id = binding.account_id and machine.worker_id = binding.worker_id
 and machine.machine_id = $4 and machine.state = 'enrolled'
join fort_private.worker_capability_revision as capability
  on capability.account_id = machine.account_id and capability.worker_id = machine.worker_id
 and capability.capability_revision_id = $3
join fort_private.execution_source as source
  on source.account_id = binding.account_id and source.execution_source_id = binding.execution_source_id
 and source.worker_id = binding.worker_id
left join fort_private.routine_run as run
  on run.account_id = target.account_id and run.target_id = target.target_id
left join fort_private.routine_occurrence as occurrence
  on occurrence.account_id = run.account_id and occurrence.routine_occurrence_id = run.routine_occurrence_id
where target.account_id = $1 and target.state = 'queued'
  and (target.target_kind <> 'routine' or (occurrence.state = 'queued' and occurrence.scheduled_for <= $5))
  and coalesce((select observation.source_config_digest
    from fort_private.execution_source_config_observation as observation
    where observation.account_id = binding.account_id
      and observation.execution_source_id = binding.execution_source_id
    order by observation.observation_sequence desc limit 1), source.source_config_digest) = binding.source_config_digest
order by target.created_at, target.target_id
limit 1 for update of target skip locked`, command.AccountID, command.WorkerID,
		command.CapabilityRevisionID, command.MachineID, command.ClaimedAt.UTC()).scan(&targetID)
	if err != nil {
		return workerClaimCandidate{}, err
	}
	command.TargetID = targetID
	return loadClaimCandidate(ctx, tx, command)
}

func loadReplayedNextClaim(ctx context.Context, tx transaction, command controlapi.WorkerClaimCommand,
	commandDigest string) (workerClaimCandidate, bool, error) {
	var existingDigest, resultKind, resultID string
	err := tx.queryRow(ctx, `select command_digest, result_kind, result_id
from fort_private.idempotency_record
where account_id = $1 and scope = $2 and idempotency_key = $3`, command.AccountID,
		workerClaimNextScope, command.IdempotencyKey).scan(&existingDigest, &resultKind, &resultID)
	if errors.Is(err, pgx.ErrNoRows) {
		return workerClaimCandidate{}, false, nil
	}
	if err != nil {
		return workerClaimCandidate{}, false, err
	}
	if existingDigest != commandDigest || resultKind != "worker_lease" || resultID != command.LeaseID {
		return workerClaimCandidate{}, true, controlapi.ErrWorkerIdempotencyConflict
	}
	var targetID string
	var fence int64
	var claimedAt, expiresAt time.Time
	err = tx.queryRow(ctx, `select lease.target_id, lease.fence_token, lease.claimed_at, lease.expires_at
from fort_private.worker_lease as lease
join fort_private.execution_attempt as attempt
  on attempt.account_id = lease.account_id and attempt.execution_attempt_id = lease.execution_attempt_id
where lease.account_id = $1 and lease.lease_id = $2 and lease.execution_attempt_id = $3
  and lease.worker_id = $4 and attempt.worker_capability_revision_id = $5`, command.AccountID,
		command.LeaseID, command.ExecutionAttemptID, command.WorkerID, command.CapabilityRevisionID).scan(
		&targetID, &fence, &claimedAt, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return workerClaimCandidate{}, true, controlapi.ErrWorkerStaleLease
	}
	if err != nil {
		return workerClaimCandidate{}, true, err
	}
	command.TargetID = targetID
	candidate, err := loadClaimCandidate(ctx, tx, command)
	if err != nil {
		return workerClaimCandidate{}, true, err
	}
	candidate.replayed, candidate.replayFence = true, fence
	candidate.replayClaimedAt, candidate.replayExpiresAt = claimedAt.UTC(), expiresAt.UTC()
	return candidate, true, nil
}

func (candidate workerClaimCandidate) assignment(command controlapi.WorkerClaimCommand) (controlapi.WorkerAssignment, error) {
	var participantAuthority struct {
		AuthorityID       string `json:"authority_id"`
		AuthorityRevision string `json:"authority_revision"`
		PolicyID          string `json:"policy_id"`
		PolicyRevision    string `json:"policy_revision"`
	}
	var grant conversation.AuthorityGrant
	if err := json.Unmarshal(candidate.participantAuthorityJSON, &participantAuthority); err != nil ||
		!validWorkerIdentifier(participantAuthority.AuthorityID) || !validWorkerIdentifier(participantAuthority.AuthorityRevision) ||
		!validWorkerIdentifier(participantAuthority.PolicyID) || !validWorkerIdentifier(participantAuthority.PolicyRevision) {
		return controlapi.WorkerAssignment{}, fmt.Errorf("target participant authority evidence is invalid")
	}
	if candidate.authorityID != participantAuthority.AuthorityID || candidate.authorityRevision != participantAuthority.AuthorityRevision ||
		candidate.policyID != participantAuthority.PolicyID || candidate.policyRevision != participantAuthority.PolicyRevision {
		return controlapi.WorkerAssignment{}, fmt.Errorf("target Binding authority does not match its immutable participant snapshot")
	}
	if err := json.Unmarshal(candidate.delegationGrantJSON, &grant); err != nil || grant.Validate() != nil ||
		!validWorkerIdentifier(candidate.delegationGrantID) {
		return controlapi.WorkerAssignment{}, fmt.Errorf("target effective authority snapshot is invalid")
	}
	authority := coreworker.AuthoritySnapshot{
		ID: grant.ID, Revision: participantAuthority.AuthorityRevision,
		Permissions:      append([]string(nil), grant.Permissions...),
		ContextRecordIDs: append([]string(nil), grant.ContextRecordIDs...),
	}
	invalidPromptSize := candidate.promptPlaintextSize < 0 || candidate.promptPlaintextSize > int64(2<<20)
	if candidate.targetKind == "handoff" {
		invalidPromptSize = candidate.promptPlaintextSize != -1
	}
	if invalidPromptSize ||
		!validWorkerIdentifier(candidate.promptKeyID) || len(candidate.promptNonce) < 12 || !lowerWorkerDigest(candidate.promptDigest) {
		return controlapi.WorkerAssignment{}, fmt.Errorf("target prompt envelope is invalid")
	}
	requiredExecutionIDs := []string{
		candidate.executionSourceID, candidate.sourceAgentID, candidate.opaqueSourceAgentID,
		candidate.fortProfile, candidate.provider, candidate.requestedModel, candidate.resolvedModel,
		candidate.adapterID, candidate.adapterRevision, candidate.authorityID, candidate.authorityRevision,
		candidate.policyID, candidate.policyRevision, candidate.sessionBehavior, candidate.memoryBehavior,
		candidate.readinessContractID, candidate.readinessContractRev, candidate.computerID,
	}
	for _, value := range requiredExecutionIDs {
		if !validWorkerIdentifier(value) {
			return controlapi.WorkerAssignment{}, fmt.Errorf("target execution Binding is incomplete")
		}
	}
	if !lowerWorkerDigest(candidate.sourceConfigDigest) || candidate.computerID != command.MachineID {
		return controlapi.WorkerAssignment{}, fmt.Errorf("target execution Binding does not match the exact machine")
	}
	workdir, err := workerBindingWorkdir(candidate.capabilityEvidenceJSON)
	if err != nil {
		return controlapi.WorkerAssignment{}, fmt.Errorf("target execution Binding capability evidence is invalid")
	}
	outputMessageKind := ""
	switch candidate.targetKind {
	case "initial":
		outputMessageKind = "agent"
	case "handoff":
		outputMessageKind = "handoff_result"
	case "routine":
		outputMessageKind = "routine_result"
	}
	if outputMessageKind == "" || !validWorkerIdentifier(candidate.outputConversationID) ||
		!validWorkerIdentifier(candidate.agentID) {
		return controlapi.WorkerAssignment{}, fmt.Errorf("target output contract is incomplete")
	}
	return controlapi.WorkerAssignment{
		TargetID: candidate.targetID, TargetKind: candidate.targetKind, OriginID: candidate.originID,
		ExecutionAttemptID: command.ExecutionAttemptID,
		LeaseID:            command.LeaseID, WorkerID: command.WorkerID, MachineID: command.MachineID,
		CapabilityRevisionID: command.CapabilityRevisionID,
		Pins: coreworker.ExecutionPins{
			AgentID: candidate.agentID, BehaviorRevisionID: candidate.behaviorRevisionID,
			BindingRevisionID: candidate.bindingRevisionID, SeatID: candidate.seatID,
			EffectiveAuthoritySnapshot: authority,
		},
		Execution: controlapi.WorkerExecutionBinding{
			ExecutionSourceID: candidate.executionSourceID, SourceAgentID: candidate.sourceAgentID,
			OpaqueSourceAgentID: candidate.opaqueSourceAgentID, FortProfile: candidate.fortProfile,
			Provider: candidate.provider, RequestedModel: candidate.requestedModel, ResolvedModel: candidate.resolvedModel,
			AdapterID: candidate.adapterID, AdapterRevision: candidate.adapterRevision,
			SourceConfigDigest: candidate.sourceConfigDigest, AuthorityID: candidate.authorityID,
			AuthorityRevision: candidate.authorityRevision, PolicyID: candidate.policyID,
			PolicyRevision: candidate.policyRevision, SessionBehavior: candidate.sessionBehavior,
			MemoryBehavior:            candidate.memoryBehavior,
			CapabilityEvidence:        append(json.RawMessage(nil), candidate.capabilityEvidenceJSON...),
			ReadinessContractID:       candidate.readinessContractID,
			ReadinessContractRevision: candidate.readinessContractRev, Workdir: workdir,
			ComputerID: candidate.computerID,
		},
		ContextManifestID:    candidate.contextManifestID,
		OutputConversationID: candidate.outputConversationID,
		OutputMessageKind:    outputMessageKind, OutputAuthorAgentID: candidate.agentID,
		MaximumOutputPlaintextBytes: int64(controlapi.MaximumArtifactPlaintextBytes),
		InlineOutputPlaintextBytes:  int64(controlapi.MaximumArtifactChunkPlaintextBytes),
		PromptEnvelope: controlapi.EncryptedEnvelope{
			Ciphertext: append([]byte(nil), candidate.promptCiphertext...), KeyID: candidate.promptKeyID,
			Nonce: append([]byte(nil), candidate.promptNonce...), Digest: candidate.promptDigest,
			PlaintextLength: int(candidate.promptPlaintextSize),
		},
		ClaimedAt: command.ClaimedAt.UTC(), ExpiresAt: command.ExpiresAt.UTC(), HardDeadline: candidate.hardDeadline.UTC(),
	}, nil
}

func workerBindingWorkdir(evidenceJSON []byte) (string, error) {
	var evidence struct {
		Values       []string `json:"values"`
		LocationKind string   `json:"location_kind"`
	}
	if err := json.Unmarshal(evidenceJSON, &evidence); err != nil || evidence.LocationKind != "computer" {
		return "", fmt.Errorf("worker Binding capability evidence is invalid")
	}
	const prefix = "workdir="
	workdir := ""
	for _, value := range evidence.Values {
		if !strings.HasPrefix(value, prefix) {
			continue
		}
		if workdir != "" {
			return "", fmt.Errorf("worker Binding has multiple workdirs")
		}
		workdir = strings.TrimPrefix(value, prefix)
	}
	if workdir == "" || len(workdir) > 4096 || !filepath.IsAbs(workdir) || filepath.Clean(workdir) != workdir || workdir == string(filepath.Separator) {
		return "", fmt.Errorf("worker Binding workdir is not one canonical absolute path")
	}
	return workdir, nil
}

func (store *Store) prepareWorkerAssignment(command controlapi.WorkerClaimCommand, candidate workerClaimCandidate) (controlapi.WorkerAssignment, error) {
	assignment, err := candidate.assignment(command)
	if err != nil {
		return controlapi.WorkerAssignment{}, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return controlapi.WorkerAssignment{}, err
	}
	recordType, recordID := "conversation_message", candidate.turnID
	switch {
	case candidate.targetKind == "handoff":
		recordType, recordID = "handoff_requested_result", candidate.originID
	case candidate.turnKind == "human_group":
		recordType, recordID = "group_turn_prompt", candidate.turnID
	}
	if !validWorkerIdentifier(recordID) {
		return controlapi.WorkerAssignment{}, fmt.Errorf("target prompt scope is invalid")
	}
	assignment.Prompt, err = cipher.open(securebody.Scope{
		AccountID: command.AccountID, RecordType: recordType, RecordID: recordID,
	}, collaborationEncryptedBody{
		Ciphertext: assignment.PromptEnvelope.Ciphertext, KeyID: assignment.PromptEnvelope.KeyID,
		Nonce: assignment.PromptEnvelope.Nonce, Digest: assignment.PromptEnvelope.Digest,
		PlaintextBytes: assignment.PromptEnvelope.PlaintextLength,
	})
	if err != nil {
		return controlapi.WorkerAssignment{}, fmt.Errorf("decrypt exact worker prompt: %w", err)
	}
	assignment.PromptEnvelope = controlapi.EncryptedEnvelope{}
	return assignment, nil
}

func replayWorkerClaim(ctx context.Context, tx transaction, command controlapi.WorkerClaimCommand, commandDigest string) (int64, time.Time, time.Time, error) {
	var existingDigest, resultKind, resultID string
	if err := tx.queryRow(ctx, `select command_digest, result_kind, result_id
from fort_private.idempotency_record
where account_id = $1 and scope = $2 and idempotency_key = $3`, command.AccountID, workerClaimScope, command.IdempotencyKey).scan(
		&existingDigest, &resultKind, &resultID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, time.Time{}, time.Time{}, controlapi.ErrWorkerNoCompatibleTarget
		}
		return 0, time.Time{}, time.Time{}, err
	}
	if existingDigest != commandDigest || resultKind != "worker_lease" || resultID != command.LeaseID {
		return 0, time.Time{}, time.Time{}, controlapi.ErrWorkerIdempotencyConflict
	}
	var fence int64
	var claimedAt, expiresAt time.Time
	err := tx.queryRow(ctx, `select lease.fence_token, lease.claimed_at, lease.expires_at
from fort_private.worker_lease as lease
join fort_private.execution_attempt as attempt
  on attempt.account_id = lease.account_id and attempt.execution_attempt_id = lease.execution_attempt_id
where lease.account_id = $1 and lease.lease_id = $2 and lease.target_id = $3
  and lease.execution_attempt_id = $4 and lease.worker_id = $5
  and attempt.worker_capability_revision_id = $6`, command.AccountID, command.LeaseID, command.TargetID,
		command.ExecutionAttemptID, command.WorkerID, command.CapabilityRevisionID).scan(&fence, &claimedAt, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, time.Time{}, time.Time{}, controlapi.ErrWorkerStaleLease
	}
	return fence, claimedAt, expiresAt, err
}

func validateWorkerClaim(command controlapi.WorkerClaimCommand) error {
	if !validWorkerIdentifier(command.WorkerID) || !validWorkerIdentifier(command.MachineID) ||
		!validWorkerIdentifier(command.TargetID) || !validWorkerIdentifier(command.ExecutionAttemptID) ||
		!validWorkerIdentifier(command.LeaseID) || !validWorkerIdentifier(command.IdempotencyKey) ||
		!validWorkerIdentifier(command.CapabilityRevisionID) || command.ClaimedAt.IsZero() ||
		!command.ExpiresAt.After(command.ClaimedAt) || command.ExpiresAt.Sub(command.ClaimedAt) > controlapi.DefaultWorkerLease {
		return fmt.Errorf("worker claim command is invalid")
	}
	return nil
}

func validateWorkerClaimNext(command controlapi.WorkerClaimNextCommand) error {
	if !validWorkerIdentifier(command.WorkerID) || !validWorkerIdentifier(command.MachineID) ||
		!validWorkerIdentifier(command.ExecutionAttemptID) || !validWorkerIdentifier(command.LeaseID) ||
		!validWorkerIdentifier(command.IdempotencyKey) || !validWorkerIdentifier(command.CapabilityRevisionID) ||
		command.ClaimedAt.IsZero() || !command.ExpiresAt.After(command.ClaimedAt) ||
		command.ExpiresAt.Sub(command.ClaimedAt) > controlapi.DefaultWorkerLease {
		return fmt.Errorf("worker claim-next command is invalid")
	}
	return nil
}

func workerClaimNextDigest(command controlapi.WorkerClaimNextCommand) (string, error) {
	return workerCommandDigest(struct {
		AccountID, WorkerID, MachineID, ExecutionAttemptID, LeaseID, IdempotencyKey, CapabilityRevisionID string
	}{command.AccountID, command.WorkerID, command.MachineID, command.ExecutionAttemptID,
		command.LeaseID, command.IdempotencyKey, command.CapabilityRevisionID})
}

func validateWorkerReadiness(command controlapi.WorkerReadinessCommand) error {
	if !validWorkerIdentifier(command.WorkerID) || !validWorkerIdentifier(command.MachineID) ||
		!validWorkerIdentifier(command.IdempotencyKey) || !validWorkerIdentifier(command.CapabilityRevisionID) ||
		command.Revision < 1 || command.ObservedAt.IsZero() || len(command.CapabilityEvidence) == 0 || len(command.CapabilityEvidence) > 64<<10 ||
		!lowerWorkerDigest(command.EvidenceDigest) {
		return fmt.Errorf("worker readiness command is invalid")
	}
	var object map[string]any
	if err := json.Unmarshal(command.CapabilityEvidence, &object); err != nil || object == nil {
		return fmt.Errorf("worker capability evidence must be a JSON object")
	}
	digest := sha256.Sum256(command.CapabilityEvidence)
	if subtle.ConstantTimeCompare([]byte(command.EvidenceDigest), []byte(hex.EncodeToString(digest[:]))) != 1 {
		return fmt.Errorf("worker capability evidence digest does not match")
	}
	return nil
}

func workerReadinessDigest(command controlapi.WorkerReadinessCommand) (string, error) {
	return workerCommandDigest(struct {
		AccountID, WorkerID, MachineID, IdempotencyKey, CapabilityRevisionID string
		Revision                                                             int
		CapabilityEvidence                                                   json.RawMessage
		EvidenceDigest                                                       string
	}{command.AccountID, command.WorkerID, command.MachineID, command.IdempotencyKey,
		command.CapabilityRevisionID, command.Revision, command.CapabilityEvidence, command.EvidenceDigest})
}

func workerClaimDigest(command controlapi.WorkerClaimCommand) (string, error) {
	return workerCommandDigest(struct {
		AccountID, WorkerID, MachineID, TargetID, ExecutionAttemptID, LeaseID string
		IdempotencyKey, CapabilityRevisionID                                  string
	}{command.AccountID, command.WorkerID, command.MachineID, command.TargetID, command.ExecutionAttemptID,
		command.LeaseID, command.IdempotencyKey, command.CapabilityRevisionID})
}

func claimWorkerIdempotency(
	ctx context.Context,
	tx transaction,
	accountID, scope, idempotencyKey, commandDigest, resultKind, resultID string,
	createdAt time.Time,
) (bool, error) {
	affected, err := tx.exec(ctx, `insert into fort_private.idempotency_record (
  account_id, scope, idempotency_key, command_digest, result_kind,
  result_id, response_digest, created_at
) values ($1, $2, $3, $4, $5, $6, $4, $7)
on conflict (account_id, scope, idempotency_key) do nothing`, accountID, scope, idempotencyKey,
		commandDigest, resultKind, resultID, createdAt.UTC())
	if err != nil {
		return false, fmt.Errorf("reserve worker idempotency key: %w", err)
	}
	if affected == 1 {
		return true, nil
	}
	if affected != 0 {
		return false, fmt.Errorf("reserve worker idempotency key affected %d rows", affected)
	}

	var existingDigest, existingKind, existingID string
	if err := tx.queryRow(ctx, `select command_digest, result_kind, result_id
from fort_private.idempotency_record
where account_id = $1 and scope = $2 and idempotency_key = $3`, accountID, scope, idempotencyKey).scan(
		&existingDigest, &existingKind, &existingID); err != nil {
		return false, err
	}
	if existingDigest != commandDigest || existingKind != resultKind || existingID != resultID {
		return false, controlapi.ErrWorkerIdempotencyConflict
	}
	return false, nil
}

func workerCommandDigest(command any) (string, error) {
	encoded, err := json.Marshal(command)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validWorkerIdentifier(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 256 && !strings.ContainsAny(value, "\r\n\t ")
}

func lowerWorkerDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}
