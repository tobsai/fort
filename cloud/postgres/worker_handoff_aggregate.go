package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/conversation"
	coreworker "github.com/tobsai/fort/core/worker"
)

type workerAggregateEvidence struct {
	targetID       string
	originID       string
	runID          string
	conversationID string
	turnID         string
	attemptID      string
	attemptNumber  int
	leaseID        string
	authorAgentID  string
	behaviorID     string
	bindingID      string
}

// startWorkerHandoffAggregate binds the already-fenced execution attempt to
// its Handoff before a provider starts. The generic target/attempt Working
// transition remains in HeartbeatWorkerLease's same transaction.
func startWorkerHandoffAggregate(ctx context.Context, tx transaction, accountID string,
	evidence workerAggregateEvidence, observedAt time.Time) (bool, error) {
	var state string
	var depth int
	var groupTurnID string
	err := tx.queryRow(ctx, `select handoff.state,handoff.depth,coalesce(handoff.group_turn_id,'')
from fort_private.handoff as handoff
join fort_private.conversation_target as target
  on target.account_id=handoff.account_id and target.target_id=handoff.target_id
join fort_private.conversation_target_binding as binding
  on binding.account_id=target.account_id and binding.target_id=target.target_id
where handoff.account_id=$1 and handoff.handoff_id=$2 and handoff.target_id=$3
  and binding.agent_id=$4 and binding.behavior_revision_id=$5 and binding.binding_revision_id=$6
for update of handoff`, accountID, evidence.originID, evidence.targetID, evidence.authorAgentID,
		evidence.behaviorID, evidence.bindingID).scan(&state, &depth, &groupTurnID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, controlapi.ErrWorkerStaleLease
	}
	if err != nil {
		return false, err
	}
	if state == "working" {
		var found string
		if err := tx.queryRow(ctx, `select execution_attempt_id from fort_private.handoff_attempt
where account_id=$1 and handoff_id=$2 and execution_attempt_id=$3`, accountID,
			evidence.originID, evidence.attemptID).scan(&found); err != nil {
			if isNoRows(err) {
				return false, controlapi.ErrWorkerStaleLease
			}
			return false, err
		}
		return false, nil
	}
	if state != "queued" {
		return false, controlapi.ErrWorkerStaleLease
	}
	agentMessages, err := postgresHandoffAgentMessageCount(ctx, tx, accountID, groupTurnID)
	if err != nil {
		return false, err
	}
	if depth > conversation.MaxGroupHandoffDepth || agentMessages >= conversation.MaxGroupAgentMessages {
		if err := markWorkerHandoffNeedsYou(ctx, tx, accountID, evidence, observedAt); err != nil {
			return false, err
		}
		return true, nil
	}
	if _, err := tx.exec(ctx, `insert into fort_private.handoff_attempt (
  account_id,handoff_id,execution_attempt_id,attempt_number,
  recipient_agent_id,recipient_behavior_revision_id,recipient_binding_revision_id,created_at
) values ($1,$2,$3,$4,$5,$6,$7,$8)`, accountID, evidence.originID, evidence.attemptID,
		evidence.attemptNumber, evidence.authorAgentID, evidence.behaviorID, evidence.bindingID,
		observedAt.UTC()); err != nil {
		return false, fmt.Errorf("insert worker Handoff attempt mapping: %w", err)
	}
	if affected, err := tx.exec(ctx, `update fort_private.handoff
set state='working',updated_at=$3
where account_id=$1 and handoff_id=$2 and state='queued'`, accountID, evidence.originID,
		observedAt.UTC()); err != nil || affected != 1 {
		return false, changedRowsError("start worker Handoff", affected, err)
	}
	return false, nil
}

func markWorkerHandoffNeedsYou(ctx context.Context, tx transaction, accountID string,
	evidence workerAggregateEvidence, observedAt time.Time) error {
	updates := []struct {
		query string
		args  []any
	}{
		{`update fort_private.handoff set state='needs_you',updated_at=$3
where account_id=$1 and handoff_id=$2 and state='queued'`, []any{accountID, evidence.originID, observedAt.UTC()}},
		{`update fort_private.conversation_target set state='needs_you',error_code='handoff_bound_exhausted',updated_at=$3
where account_id=$1 and target_id=$2 and state='claimed'`, []any{accountID, evidence.targetID, observedAt.UTC()}},
		{`update fort_private.execution_attempt set state='needs_you',updated_at=$3
where account_id=$1 and execution_attempt_id=$2 and state='leased'`, []any{accountID, evidence.attemptID, observedAt.UTC()}},
		{`update fort_private.worker_lease set state='revoked',released_at=$3,updated_at=$3
where account_id=$1 and lease_id=$2 and state='active'`, []any{accountID, evidence.leaseID, observedAt.UTC()}},
		{`update fort_private.conversation_turn set state='needs_you',updated_at=$3
where account_id=$1 and turn_id=$2 and state='open'`, []any{accountID, evidence.turnID, observedAt.UTC()}},
	}
	for _, update := range updates {
		if affected, err := tx.exec(ctx, update.query, update.args...); err != nil || affected != 1 {
			return changedRowsError("mark bounded Handoff Needs You", affected, err)
		}
	}
	return nil
}

func commitWorkerHandoffAggregate(ctx context.Context, tx transaction, accountID string,
	evidence workerAggregateEvidence, command controlapi.WorkerTerminalCommand,
	message collaborationEncryptedBody) (int64, error) {
	var handoffState, groupTurnID string
	err := tx.queryRow(ctx, `select handoff.state,coalesce(handoff.group_turn_id,'')
from fort_private.handoff as handoff
join fort_private.conversation_target_binding as binding
  on binding.account_id=handoff.account_id and binding.target_id=handoff.target_id
where handoff.account_id=$1 and handoff.handoff_id=$2 and handoff.target_id=$3
  and binding.agent_id=$4 and binding.behavior_revision_id=$5 and binding.binding_revision_id=$6
for update of handoff`, accountID, evidence.originID, evidence.targetID, evidence.authorAgentID,
		evidence.behaviorID, evidence.bindingID).scan(&handoffState, &groupTurnID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, controlapi.ErrWorkerStaleLease
	}
	if err != nil {
		return 0, err
	}
	if command.Status == coreworker.TerminalCompleted && handoffState != "working" {
		return 0, controlapi.ErrWorkerStaleLease
	}
	if command.Status == coreworker.TerminalFailed && handoffState != "queued" && handoffState != "working" {
		return 0, controlapi.ErrWorkerStaleLease
	}
	if command.Status == coreworker.TerminalCanceled && handoffState != "canceled" {
		return 0, controlapi.ErrWorkerStaleLease
	}
	if handoffState == "queued" {
		if _, err := tx.exec(ctx, `insert into fort_private.handoff_attempt (
  account_id,handoff_id,execution_attempt_id,attempt_number,
  recipient_agent_id,recipient_behavior_revision_id,recipient_binding_revision_id,created_at
) values ($1,$2,$3,$4,$5,$6,$7,$8)`, accountID, evidence.originID, evidence.attemptID,
			evidence.attemptNumber, evidence.authorAgentID, evidence.behaviorID, evidence.bindingID,
			command.CommittedAt.UTC()); err != nil {
			return 0, fmt.Errorf("insert terminal Handoff attempt mapping: %w", err)
		}
	}
	var messageID int64
	if command.Status == coreworker.TerminalCompleted {
		if err := tx.queryRow(ctx, `insert into fort_private.conversation_message (
  account_id,conversation_id,turn_id,target_id,handoff_id,routine_run_id,
  message_kind,author_kind,author_id,author_agent_id,
  body_ciphertext,body_key_id,body_nonce,body_digest,body_plaintext_length,created_at
) values ($1,$2,$3,$4,$5,null,'handoff_result','agent',$6,$6,$7,$8,$9,$10,$11,$12)
returning message_id`, accountID, evidence.conversationID, evidence.turnID, evidence.targetID,
			evidence.originID, evidence.authorAgentID, message.Ciphertext, message.KeyID, message.Nonce,
			message.Digest, message.PlaintextBytes, command.CommittedAt.UTC()).scan(&messageID); err != nil {
			return 0, fmt.Errorf("insert authoritative worker Handoff result: %w", err)
		}
	}
	childTurnState, eventType := "needs_you", "handoff.failed"
	switch command.Status {
	case coreworker.TerminalCompleted:
		if affected, err := tx.exec(ctx, `update fort_private.handoff
set state='succeeded',terminal_at=$3,updated_at=$3
where account_id=$1 and handoff_id=$2 and state='working'`, accountID, evidence.originID,
			command.CommittedAt.UTC()); err != nil || affected != 1 {
			return 0, changedRowsError("complete worker Handoff", affected, err)
		}
		childTurnState, eventType = "settled", "handoff.completed"
	case coreworker.TerminalFailed:
		if affected, err := tx.exec(ctx, `update fort_private.handoff
set state='failed',terminal_at=$3,updated_at=$3
where account_id=$1 and handoff_id=$2 and state in ('queued','working')`, accountID,
			evidence.originID, command.CommittedAt.UTC()); err != nil || affected != 1 {
			return 0, changedRowsError("fail worker Handoff", affected, err)
		}
	case coreworker.TerminalCanceled:
		childTurnState, eventType = "canceled", "handoff.canceled"
	}
	if _, err := tx.exec(ctx, `update fort_private.conversation_turn
set state=$3,updated_at=$4 where account_id=$1 and turn_id=$2 and state in ('open','needs_you')`,
		accountID, evidence.turnID, childTurnState, command.CommittedAt.UTC()); err != nil {
		return 0, err
	}
	if groupTurnID != "" {
		if command.Status == coreworker.TerminalFailed {
			if _, err := tx.exec(ctx, `update fort_private.conversation_turn
set state='needs_you',updated_at=$3 where account_id=$1 and turn_id=$2 and state='open'`,
				accountID, groupTurnID, command.CommittedAt.UTC()); err != nil {
				return 0, err
			}
		} else if err := settleConversationTurnIfTerminal(ctx, tx, accountID, groupTurnID, command.CommittedAt); err != nil {
			return 0, err
		}
	}
	metadata, err := json.Marshal(map[string]any{
		"status": command.Status, "conversation_message_id": messageID,
		"output_artifact_id": command.Output.ArtifactID,
	})
	if err != nil {
		return 0, err
	}
	if _, err := tx.exec(ctx, `insert into fort_private.ledger_event (
  account_id,aggregate_kind,aggregate_id,event_type,turn_id,target_id,event_metadata,created_at
) values ($1,'handoff',$2,$3,$4,$5,$6::jsonb,$7)`, accountID, evidence.originID,
		eventType, evidence.turnID, evidence.targetID, string(metadata), command.CommittedAt.UTC()); err != nil {
		return 0, err
	}
	return messageID, nil
}
