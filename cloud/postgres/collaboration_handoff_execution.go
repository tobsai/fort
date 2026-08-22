package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/tobsai/fort/cloud/securebody"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

const (
	handoffStartScope    = "handoff.start"
	handoffCompleteScope = "handoff.complete"
)

func (store *Store) StartHandoff(ctx context.Context, command ledger.StartHandoffCommand) (ledger.HandoffRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.HandoffRecord{}, err
	}
	fence, err := canonicalPostgresFence(command.FenceToken)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	var record ledger.HandoffRecord
	var needsYou error
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, handoffStartScope,
			command.IdempotencyKey, digest, "handoff_attempt", command.AttemptID, command.StartedAt)
		if err != nil {
			return err
		}
		record, err = getPostgresHandoff(ctx, tx, cipher, accountID, command.HandoffID)
		if err != nil {
			return err
		}
		if !claimed {
			if record.Handoff.State == conversation.HandoffNeedsYou {
				needsYou = conversation.ErrHandoffNeedsYou
			}
			return nil
		}
		if record.Handoff.State != conversation.HandoffQueued || record.Target.State != conversation.TargetQueued {
			return fmt.Errorf("Handoff %q is not startable from state %q", command.HandoffID, record.Handoff.State)
		}
		var attemptNumber int
		var attemptState, targetState, leaseState string
		var claimedAt, expiresAt time.Time
		err = tx.queryRow(ctx, `select attempt.attempt_number, attempt.state,
  target.state, lease.state, lease.claimed_at, lease.expires_at
from fort_private.handoff as handoff
join fort_private.conversation_target as target
  on target.account_id = handoff.account_id and target.target_id = handoff.target_id
join fort_private.conversation_target_binding as binding
  on binding.account_id = target.account_id and binding.target_id = target.target_id
join fort_private.execution_attempt as attempt
  on attempt.account_id = target.account_id and attempt.target_id = target.target_id
 and attempt.agent_id = binding.agent_id
 and attempt.behavior_revision_id = binding.behavior_revision_id
 and attempt.binding_revision_id = binding.binding_revision_id
 and attempt.participant_id = binding.participant_id
join fort_private.worker_lease as lease
  on lease.account_id = attempt.account_id
 and lease.execution_attempt_id = attempt.execution_attempt_id
 and lease.target_id = target.target_id
join fort_private.worker as worker
  on worker.account_id = lease.account_id and worker.worker_id = lease.worker_id
where handoff.account_id = $1 and handoff.handoff_id = $2
  and attempt.execution_attempt_id = $3 and lease.lease_id = $4
  and lease.fence_token = $5 and worker.machine_id = $6
for update of handoff, target, attempt, lease`, accountID, command.HandoffID,
			command.AttemptID, command.LeaseID, fence, command.MachineID).scan(
			&attemptNumber, &attemptState, &targetState, &leaseState, &claimedAt, &expiresAt)
		if isNoRows(err) {
			return fmt.Errorf("Handoff start lease, fence, machine, attempt, target, or binding evidence is stale")
		}
		if err != nil {
			return err
		}
		if attemptState != "leased" || targetState != "claimed" || leaseState != "active" ||
			!expiresAt.Equal(command.LeaseExpiresAt.UTC()) || command.StartedAt.Before(claimedAt) ||
			!command.StartedAt.Before(expiresAt) {
			return fmt.Errorf("Handoff start requires the exact active unexpired claimed lease")
		}
		agentMessages, err := postgresHandoffAgentMessageCount(ctx, tx, accountID, record.Handoff.GroupTurnID)
		if err != nil {
			return err
		}
		if err := record.Handoff.CanStart(command.StartedAt, agentMessages); err != nil {
			if !errors.Is(err, conversation.ErrHandoffNeedsYou) {
				return err
			}
			needsYou = err
			if _, updateErr := tx.exec(ctx, `update fort_private.handoff
set state = 'needs_you', updated_at = $3
where account_id = $1 and handoff_id = $2 and state = 'queued'`, accountID,
				command.HandoffID, command.StartedAt.UTC()); updateErr != nil {
				return updateErr
			}
			if _, updateErr := tx.exec(ctx, `update fort_private.conversation_target
set state = 'needs_you', error_code = 'handoff_bound_exhausted', updated_at = $3
where account_id = $1 and target_id = $2 and state = 'claimed'`, accountID,
				record.Target.ID, command.StartedAt.UTC()); updateErr != nil {
				return updateErr
			}
			if _, updateErr := tx.exec(ctx, `update fort_private.execution_attempt
set state = 'needs_you', updated_at = $3
where account_id = $1 and execution_attempt_id = $2 and state = 'leased'`, accountID,
				command.AttemptID, command.StartedAt.UTC()); updateErr != nil {
				return updateErr
			}
			if _, updateErr := tx.exec(ctx, `update fort_private.worker_lease
set state = 'revoked', released_at = $3, updated_at = $3
where account_id = $1 and lease_id = $2 and state = 'active'`, accountID,
				command.LeaseID, command.StartedAt.UTC()); updateErr != nil {
				return updateErr
			}
			if _, updateErr := tx.exec(ctx, `update fort_private.conversation_turn
set state = 'needs_you', updated_at = $3
where account_id = $1 and turn_id = $2 and state = 'open'`, accountID,
				handoffTurnID(command.HandoffID), command.StartedAt.UTC()); updateErr != nil {
				return updateErr
			}
			record.Handoff.State = conversation.HandoffNeedsYou
			for index := range record.Projections {
				record.Projections[index].State = conversation.HandoffNeedsYou
			}
			return nil
		}
		if _, err := tx.exec(ctx, `insert into fort_private.handoff_attempt (
  account_id, handoff_id, execution_attempt_id, attempt_number,
  recipient_agent_id, recipient_behavior_revision_id,
  recipient_binding_revision_id, created_at
) values ($1,$2,$3,$4,$5,$6,$7,$8)`, accountID, command.HandoffID,
			command.AttemptID, attemptNumber, record.Handoff.RecipientAgentID,
			record.Handoff.RecipientBehaviorRevisionID, record.Handoff.RecipientBindingRevisionID,
			command.StartedAt.UTC()); err != nil {
			return fmt.Errorf("insert Handoff attempt mapping: %w", err)
		}
		if affected, err := tx.exec(ctx, `update fort_private.handoff
set state = 'working', updated_at = $3
where account_id = $1 and handoff_id = $2 and state = 'queued'`, accountID,
			command.HandoffID, command.StartedAt.UTC()); err != nil || affected != 1 {
			return changedRowsError("start Handoff", affected, err)
		}
		if affected, err := tx.exec(ctx, `update fort_private.conversation_target
set state = 'working', updated_at = $3
where account_id = $1 and target_id = $2 and state = 'claimed'`, accountID,
			record.Target.ID, command.StartedAt.UTC()); err != nil || affected != 1 {
			return changedRowsError("start Handoff target", affected, err)
		}
		if affected, err := tx.exec(ctx, `update fort_private.execution_attempt
set state = 'working', started_at = $3, updated_at = $3
where account_id = $1 and execution_attempt_id = $2 and state = 'leased'`, accountID,
			command.AttemptID, command.StartedAt.UTC()); err != nil || affected != 1 {
			return changedRowsError("start Handoff execution attempt", affected, err)
		}
		record.Handoff.State = conversation.HandoffWorking
		record.Target.State = conversation.TargetWorking
		record.Attempt = &ledger.HandoffAttemptRecord{ID: command.AttemptID,
			HandoffID: command.HandoffID, LeaseID: command.LeaseID, MachineID: command.MachineID,
			FenceToken: command.FenceToken, State: ledger.HandoffAttemptWorking,
			StartedAt: command.StartedAt, LeaseExpiresAt: command.LeaseExpiresAt}
		for index := range record.Projections {
			record.Projections[index].State = conversation.HandoffWorking
		}
		return nil
	})
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	if needsYou != nil {
		return record, needsYou
	}
	return record, nil
}

func (store *Store) CompleteHandoff(ctx context.Context, command ledger.CompleteHandoffCommand) (ledger.HandoffRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.HandoffRecord{}, err
	}
	fence, err := canonicalPostgresFence(command.FenceToken)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	var record ledger.HandoffRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, handoffCompleteScope,
			command.IdempotencyKey, digest, "handoff", command.HandoffID, command.CreatedAt)
		if err != nil {
			return err
		}
		record, err = getPostgresHandoff(ctx, tx, cipher, accountID, command.HandoffID)
		if err != nil {
			return err
		}
		if !claimed {
			return nil
		}
		if record.Handoff.State == conversation.HandoffCompleted || record.Result != nil {
			return fmt.Errorf("%w: Handoff %q", ledger.ErrAlreadyCompleted, command.HandoffID)
		}
		if record.Handoff.State != conversation.HandoffWorking || record.Target.State != conversation.TargetWorking ||
			record.Attempt == nil || command.AuthorAgentID != record.Handoff.RecipientAgentID {
			return fmt.Errorf("Handoff %q does not have one attributable working attempt", command.HandoffID)
		}
		var attemptState, targetState, leaseState string
		var startedAt, expiresAt, hardDeadline time.Time
		err = tx.queryRow(ctx, `select attempt.state, target.state, lease.state,
  attempt.started_at, lease.expires_at, handoff.hard_deadline
from fort_private.handoff_attempt as mapping
join fort_private.handoff as handoff
  on handoff.account_id = mapping.account_id and handoff.handoff_id = mapping.handoff_id
join fort_private.execution_attempt as attempt
  on attempt.account_id = mapping.account_id
 and attempt.execution_attempt_id = mapping.execution_attempt_id
join fort_private.conversation_target as target
  on target.account_id = attempt.account_id and target.target_id = attempt.target_id
join fort_private.worker_lease as lease
  on lease.account_id = attempt.account_id
 and lease.execution_attempt_id = attempt.execution_attempt_id
where mapping.account_id = $1 and mapping.handoff_id = $2
  and mapping.execution_attempt_id = $3 and lease.lease_id = $4
  and lease.fence_token = $5 and handoff.target_id = target.target_id
for update of handoff, attempt, target, lease`, accountID, command.HandoffID,
			command.AttemptID, command.LeaseID, fence).scan(
			&attemptState, &targetState, &leaseState, &startedAt, &expiresAt, &hardDeadline)
		if isNoRows(err) {
			return fmt.Errorf("Handoff completion attempt, lease, or fence evidence is stale")
		}
		if err != nil {
			return err
		}
		if attemptState != "working" || targetState != "working" || leaseState != "active" ||
			!handoffCompletionWithinBounds(command.CreatedAt, startedAt, expiresAt, hardDeadline) {
			return fmt.Errorf("Handoff completion requires the exact active working lease before its hard deadline")
		}
		result, err := cipher.seal(securebody.Scope{AccountID: accountID,
			RecordType: "handoff_result", RecordID: command.HandoffID}, command.Body)
		if err != nil {
			return fmt.Errorf("encrypt Handoff result: %w", err)
		}
		receipt, err := cipher.seal(securebody.Scope{AccountID: accountID,
			RecordType: "handoff_terminal_receipt", RecordID: command.AttemptID}, command.TerminalReceiptID)
		if err != nil {
			return fmt.Errorf("encrypt Handoff terminal receipt: %w", err)
		}
		var messageID int64
		if err := tx.queryRow(ctx, `insert into fort_private.conversation_message (
  account_id, conversation_id, turn_id, target_id, handoff_id, routine_run_id,
  message_kind, author_kind, author_id, author_agent_id,
  body_ciphertext, body_key_id, body_nonce, body_digest,
  body_plaintext_length, created_at
) values ($1,$2,$3,$4,$5,null,'handoff_result','agent',$6,$6,$7,$8,$9,$10,$11,$12)
returning message_id`, accountID, record.Handoff.OutputConversationID,
			handoffTurnID(command.HandoffID), record.Target.ID, command.HandoffID,
			command.AuthorAgentID, result.Ciphertext, result.KeyID, result.Nonce,
			result.Digest, result.PlaintextBytes, command.CreatedAt.UTC()).scan(&messageID); err != nil {
			return fmt.Errorf("insert authoritative Handoff result: %w", err)
		}
		if affected, err := tx.exec(ctx, `update fort_private.execution_attempt
set state = 'succeeded', provider_terminal_status = 'succeeded',
    terminal_receipt_id = $3, terminal_receipt_ciphertext = $4,
    terminal_receipt_key_id = $5, terminal_receipt_nonce = $6,
    terminal_receipt_digest = $7, terminal_at = $8, updated_at = $8
where account_id = $1 and execution_attempt_id = $2 and state = 'working'`,
			accountID, command.AttemptID, command.TerminalReceiptID, receipt.Ciphertext,
			receipt.KeyID, receipt.Nonce, receipt.Digest, command.CreatedAt.UTC()); err != nil || affected != 1 {
			return changedRowsError("complete Handoff execution attempt", affected, err)
		}
		if affected, err := tx.exec(ctx, `update fort_private.conversation_target
set state = 'succeeded', updated_at = $3
where account_id = $1 and target_id = $2 and state = 'working'`, accountID,
			record.Target.ID, command.CreatedAt.UTC()); err != nil || affected != 1 {
			return changedRowsError("complete Handoff target", affected, err)
		}
		if affected, err := tx.exec(ctx, `update fort_private.worker_lease
set state = 'released', released_at = $3, updated_at = $3
where account_id = $1 and lease_id = $2 and state = 'active'`, accountID,
			command.LeaseID, command.CreatedAt.UTC()); err != nil || affected != 1 {
			return changedRowsError("release Handoff lease", affected, err)
		}
		if affected, err := tx.exec(ctx, `update fort_private.handoff
set state = 'succeeded', terminal_at = $3, updated_at = $3
where account_id = $1 and handoff_id = $2 and state = 'working'`, accountID,
			command.HandoffID, command.CreatedAt.UTC()); err != nil || affected != 1 {
			return changedRowsError("complete Handoff", affected, err)
		}
		if _, err := tx.exec(ctx, `update fort_private.conversation_turn
set state = 'settled', updated_at = $3
where account_id = $1 and turn_id = $2 and state = 'open'`, accountID,
			handoffTurnID(command.HandoffID), command.CreatedAt.UTC()); err != nil {
			return err
		}
		if record.Handoff.GroupTurnID != "" {
			if err := settleConversationTurnIfTerminal(ctx, tx, accountID, record.Handoff.GroupTurnID, command.CreatedAt); err != nil {
				return err
			}
		}
		if _, err := tx.exec(ctx, `update fort_private.conversation
set updated_at = $3 where account_id = $1 and conversation_id = $2`, accountID,
			record.Handoff.OutputConversationID, command.CreatedAt.UTC()); err != nil {
			return err
		}
		record.Handoff.State = conversation.HandoffCompleted
		record.Target.State = conversation.TargetAnswered
		record.Result = &conversation.HandoffResult{HandoffID: command.HandoffID,
			OutputConversationID: record.Handoff.OutputConversationID,
			MessageID:            strconv.FormatInt(messageID, 10), Body: command.Body}
		record.Attempt.State = ledger.HandoffAttemptCompleted
		record.Attempt.TerminalReceiptID = command.TerminalReceiptID
		record.Attempt.CompletedAt = command.CreatedAt
		for index := range record.Projections {
			record.Projections[index].State = conversation.HandoffCompleted
			record.Projections[index].AuthoritativeMessageID = record.Result.MessageID
		}
		return nil
	})
	return record, err
}

func canonicalPostgresFence(value string) (int64, error) {
	fence, err := strconv.ParseInt(value, 10, 64)
	if err != nil || fence <= 0 || strconv.FormatInt(fence, 10) != value {
		return 0, fmt.Errorf("Handoff fence token must be canonical positive decimal text")
	}
	return fence, nil
}

func handoffCompletionWithinBounds(completedAt, startedAt, leaseExpiry, hardDeadline time.Time) bool {
	return !completedAt.Before(startedAt) && completedAt.Before(leaseExpiry) && completedAt.Before(hardDeadline)
}

func postgresHandoffAgentMessageCount(ctx context.Context, tx transaction, accountID, groupTurnID string) (int, error) {
	if groupTurnID == "" {
		return 0, nil
	}
	var count int
	err := tx.queryRow(ctx, `select count(*)
from fort_private.conversation_message
where account_id = $1 and author_kind = 'agent'
  and (turn_id = $2 or handoff_id in (
    select handoff_id from fort_private.handoff
    where account_id = $1 and group_turn_id = $2
  ))`, accountID, groupTurnID).scan(&count)
	return count, err
}

func changedRowsError(operation string, affected int64, err error) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("%s changed %d rows, want 1", operation, affected)
}
