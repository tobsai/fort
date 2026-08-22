package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

// agentDirectChatSchema is additive. The legacy turn/target tables remain the
// local execution queue while these records preserve v2 stable-Agent pins and
// a reference-only frozen context manifest.
const agentDirectChatSchema = `
CREATE TABLE IF NOT EXISTS execution_source_config_observation (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  observation_id TEXT NOT NULL,
  account_id TEXT NOT NULL,
  execution_source_id TEXT NOT NULL,
  source_config_digest TEXT NOT NULL CHECK(length(source_config_digest)=64 AND source_config_digest NOT GLOB '*[^0-9a-f]*'),
  observed_by TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  UNIQUE(account_id,observation_id),
  FOREIGN KEY(execution_source_id) REFERENCES execution_source(id)
);
CREATE INDEX IF NOT EXISTS idx_execution_source_config_observation_latest
  ON execution_source_config_observation(account_id,execution_source_id,id DESC);

CREATE TABLE IF NOT EXISTS stable_agent_context_manifest (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  conversation_id TEXT NOT NULL,
  through_message_id INTEGER NOT NULL,
  manifest_digest TEXT NOT NULL CHECK(length(manifest_digest)=64 AND manifest_digest NOT GLOB '*[^0-9a-f]*'),
  created_by TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(conversation_id) REFERENCES conversation(id),
  FOREIGN KEY(through_message_id) REFERENCES conversation_message(id)
);
CREATE TABLE IF NOT EXISTS stable_agent_context_manifest_message (
  manifest_id TEXT NOT NULL,
  ordinal INTEGER NOT NULL CHECK(ordinal>=0 AND ordinal<256),
  message_id INTEGER NOT NULL,
  PRIMARY KEY(manifest_id,ordinal),
  UNIQUE(manifest_id,message_id),
  FOREIGN KEY(manifest_id) REFERENCES stable_agent_context_manifest(id),
  FOREIGN KEY(message_id) REFERENCES conversation_message(id)
);
CREATE TABLE IF NOT EXISTS stable_agent_direct_turn (
  turn_id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  agent_id TEXT NOT NULL,
  conversation_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  command_digest TEXT NOT NULL CHECK(length(command_digest)=64 AND command_digest NOT GLOB '*[^0-9a-f]*'),
  membership_revision_id TEXT NOT NULL,
  context_manifest_id TEXT NOT NULL UNIQUE,
  delegation_grant_id TEXT NOT NULL,
  behavior_revision_id TEXT NOT NULL,
  binding_revision_id TEXT NOT NULL,
  created_by TEXT NOT NULL,
  hard_deadline TEXT NOT NULL,
  state TEXT NOT NULL CHECK(state IN ('open','settled','needs_you','canceled')),
  created_at TEXT NOT NULL,
  UNIQUE(account_id,conversation_id,idempotency_key),
  FOREIGN KEY(turn_id) REFERENCES conversation_turn(id),
  FOREIGN KEY(agent_id,conversation_id) REFERENCES agent_conversation(agent_id,conversation_id),
  FOREIGN KEY(behavior_revision_id,agent_id) REFERENCES agent_behavior_revision(id,agent_id),
  FOREIGN KEY(binding_revision_id,agent_id) REFERENCES agent_binding_revision(id,agent_id),
  FOREIGN KEY(context_manifest_id) REFERENCES stable_agent_context_manifest(id)
);
CREATE TABLE IF NOT EXISTS stable_agent_direct_target_binding (
  target_id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  turn_id TEXT NOT NULL,
  conversation_id TEXT NOT NULL,
  agent_id TEXT NOT NULL,
  behavior_revision_id TEXT NOT NULL,
  binding_revision_id TEXT NOT NULL,
  participant_id TEXT NOT NULL,
  pinned_at TEXT NOT NULL,
  FOREIGN KEY(target_id) REFERENCES conversation_target(id),
  FOREIGN KEY(turn_id) REFERENCES stable_agent_direct_turn(turn_id),
  FOREIGN KEY(agent_id,binding_revision_id,conversation_id)
    REFERENCES stable_agent_participant_evidence(agent_id,binding_revision_id,conversation_id),
  FOREIGN KEY(participant_id) REFERENCES conversation_participant(id),
  UNIQUE(account_id,target_id,agent_id,behavior_revision_id,binding_revision_id,participant_id)
);

CREATE TRIGGER IF NOT EXISTS execution_source_config_observation_immutable_update
BEFORE UPDATE ON execution_source_config_observation BEGIN SELECT RAISE(ABORT,'stable_agent_direct_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS execution_source_config_observation_immutable_delete
BEFORE DELETE ON execution_source_config_observation BEGIN SELECT RAISE(ABORT,'stable_agent_direct_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_context_manifest_immutable_update
BEFORE UPDATE ON stable_agent_context_manifest BEGIN SELECT RAISE(ABORT,'stable_agent_direct_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_context_manifest_immutable_delete
BEFORE DELETE ON stable_agent_context_manifest BEGIN SELECT RAISE(ABORT,'stable_agent_direct_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_context_manifest_message_immutable_update
BEFORE UPDATE ON stable_agent_context_manifest_message BEGIN SELECT RAISE(ABORT,'stable_agent_direct_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_context_manifest_message_immutable_delete
BEFORE DELETE ON stable_agent_context_manifest_message BEGIN SELECT RAISE(ABORT,'stable_agent_direct_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_context_message_update_immutable
BEFORE UPDATE ON conversation_message
WHEN EXISTS (SELECT 1 FROM stable_agent_context_manifest_message WHERE message_id=OLD.id OR message_id=NEW.id)
BEGIN SELECT RAISE(ABORT,'stable_agent_direct_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_context_message_delete_immutable
BEFORE DELETE ON conversation_message
WHEN EXISTS (SELECT 1 FROM stable_agent_context_manifest_message WHERE message_id=OLD.id)
BEGIN SELECT RAISE(ABORT,'stable_agent_direct_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_direct_turn_immutable_identity
BEFORE UPDATE OF turn_id,account_id,agent_id,conversation_id,idempotency_key,command_digest,membership_revision_id,
context_manifest_id,delegation_grant_id,behavior_revision_id,binding_revision_id,created_by,hard_deadline,created_at
ON stable_agent_direct_turn BEGIN SELECT RAISE(ABORT,'stable_agent_direct_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_direct_turn_delete_immutable
BEFORE DELETE ON stable_agent_direct_turn BEGIN SELECT RAISE(ABORT,'stable_agent_direct_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_direct_target_binding_immutable_update
BEFORE UPDATE ON stable_agent_direct_target_binding BEGIN SELECT RAISE(ABORT,'stable_agent_direct_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_direct_target_binding_immutable_delete
BEFORE DELETE ON stable_agent_direct_target_binding BEGIN SELECT RAISE(ABORT,'stable_agent_direct_evidence_immutable'); END;
`

var _ ledger.AgentDirectChatRepository = (*Store)(nil)
var _ ledger.ExecutionSourceConfigObservationRepository = (*Store)(nil)

func (s *Store) SendAgentTurn(ctx context.Context, command ledger.SendAgentTurnCommand) (ledger.AgentTurnDispatch, error) {
	if err := command.Validate(); err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	defer tx.Rollback()

	if resultID, replay, err := findSQLiteDirectIdempotency(ctx, tx, command.AccountID, "agent.turn.send", command.IdempotencyKey, digest); err != nil {
		return ledger.AgentTurnDispatch{}, err
	} else if replay {
		result, err := getSQLiteAgentTurnDispatch(ctx, tx, command.AccountID, command.AgentID, command.ConversationID, resultID)
		if err != nil {
			return ledger.AgentTurnDispatch{}, err
		}
		if err := tx.Commit(); err != nil {
			return ledger.AgentTurnDispatch{}, err
		}
		result.Created = false
		return result, nil
	}

	var agentState conversation.AgentState
	var itemState conversation.ConversationState
	err = tx.QueryRowContext(ctx, `SELECT agent.state,item.state
FROM stable_agent agent JOIN agent_conversation relation ON relation.agent_id=agent.id
JOIN conversation item ON item.id=relation.conversation_id
WHERE agent.account_id=? AND agent.id=? AND relation.conversation_id=?`, command.AccountID,
		command.AgentID, command.ConversationID).Scan(&agentState, &itemState)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.AgentTurnDispatch{}, fmt.Errorf("%w: Agent Conversation %q", ledger.ErrNotFound, command.ConversationID)
	}
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	if agentState != conversation.AgentOpen || itemState != conversation.ConversationOpen {
		return ledger.AgentTurnDispatch{}, fmt.Errorf("%w: Agent and Conversation must be open", ledger.ErrStateConflict)
	}
	current, err := getAgentRecord(ctx, tx, command.AccountID, command.AgentID)
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	if err := verifySQLiteSourceConfiguration(ctx, tx, command.AccountID, current.Binding); err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	participant, err := ensureSQLiteDirectParticipant(ctx, tx, current, command.ConversationID, command.CreatedAt)
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}

	createdAt := nowOr(command.CreatedAt)
	messageResult, err := tx.ExecContext(ctx, `INSERT INTO conversation_message(
conversation_id,turn_id,target_id,author_kind,author_id,body,created_at
) VALUES(?,?,NULL,?,?,?,?)`, command.ConversationID, command.TurnID, conversation.AuthorHuman,
		command.HumanID, command.Body, createdAt)
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	messageID, err := messageResult.LastInsertId()
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	messages, err := conversationMessagesQuery(tx, command.ConversationID)
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	messageIDs := make([]int64, 0, len(messages))
	for _, message := range messages {
		if message.ID <= messageID {
			messageIDs = append(messageIDs, message.ID)
		}
	}
	if len(messageIDs) > 256 {
		return ledger.AgentTurnDispatch{}, fmt.Errorf("Agent direct context exceeds 256 committed messages")
	}
	contextJSON, err := conversation.CompileContext(command.ConversationID, messageID, []conversation.Participant{participant}, messages)
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	manifestDigest, err := sqliteManifestDigest(command.ConversationID, messageID, messageIDs)
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_agent_context_manifest(
id,account_id,conversation_id,through_message_id,manifest_digest,created_by,created_at
) VALUES(?,?,?,?,?,?,?)`, command.ContextManifestID, command.AccountID, command.ConversationID,
		messageID, manifestDigest, command.CreatedBy, createdAt); err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	for ordinal, id := range messageIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO stable_agent_context_manifest_message(manifest_id,ordinal,message_id)
VALUES(?,?,?)`, command.ContextManifestID, ordinal, id); err != nil {
			return ledger.AgentTurnDispatch{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO conversation_turn(
id,conversation_id,client_turn_id,prompt_message_id,through_message_id,context_json,created_at
) VALUES(?,?,?,?,?,?,?)`, command.TurnID, command.ConversationID, command.ClientTurnID,
		messageID, messageID, contextJSON, createdAt); err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	membershipID := sqliteAgentMembershipID(command.AccountID, command.AgentID, command.ConversationID)
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_agent_direct_turn(
turn_id,account_id,agent_id,conversation_id,idempotency_key,command_digest,membership_revision_id,
context_manifest_id,delegation_grant_id,behavior_revision_id,binding_revision_id,created_by,hard_deadline,state,created_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,'open',?)`, command.TurnID, command.AccountID, command.AgentID,
		command.ConversationID, command.IdempotencyKey, digest, membershipID, command.ContextManifestID,
		command.DelegationGrantID, current.Behavior.ID, current.Binding.ID, command.CreatedBy,
		nowOr(command.HardDeadline), createdAt); err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	target := conversation.Target{ID: command.TargetID, TurnID: command.TurnID, ParticipantID: participant.ID,
		RunID: command.RunID, Attempt: 1, State: conversation.TargetQueued,
		CreatedAt: command.CreatedAt.UTC(), UpdatedAt: command.CreatedAt.UTC()}
	if err := insertConversationTarget(tx, target); err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_agent_direct_target_binding(
target_id,account_id,turn_id,conversation_id,agent_id,behavior_revision_id,binding_revision_id,participant_id,pinned_at
) VALUES(?,?,?,?,?,?,?,?,?)`, target.ID, command.AccountID, command.TurnID, command.ConversationID,
		command.AgentID, current.Behavior.ID, current.Binding.ID, participant.ID, createdAt); err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_agent_lifecycle_idempotency(
account_id,scope,idempotency_key,command_digest,result_id,created_at) VALUES(?,?,?,?,?,?)`,
		command.AccountID, "agent.turn.send", command.IdempotencyKey, digest, command.TurnID, createdAt); err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE conversation SET updated_at=? WHERE id=?`, createdAt, command.ConversationID); err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	if err := insertSQLiteAgentLifecycleEvent(ctx, tx, command.AccountID, command.AgentID, "agent.turn.queued",
		map[string]any{"conversation_id": command.ConversationID, "turn_id": command.TurnID,
			"target_id": command.TargetID, "run_id": command.RunID, "behavior_revision_id": current.Behavior.ID,
			"binding_revision_id": current.Binding.ID}, command.CreatedAt); err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	result, err := getSQLiteAgentTurnDispatch(ctx, tx, command.AccountID, command.AgentID, command.ConversationID, command.TurnID)
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	result.Created = true
	return result, nil
}

func (s *Store) ReadAgentConversation(ctx context.Context, accountID, agentID, conversationID string) (ledger.AgentConversationProjection, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(agentID) == "" || strings.TrimSpace(conversationID) == "" {
		return ledger.AgentConversationProjection{}, fmt.Errorf("Agent Conversation parent chain is required")
	}
	record, err := getAgentConversationRecord(ctx, s.db, accountID, agentID, conversationID)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.AgentConversationProjection{}, fmt.Errorf("%w: Agent Conversation %q", ledger.ErrNotFound, conversationID)
	}
	if err != nil {
		return ledger.AgentConversationProjection{}, err
	}
	projection := ledger.AgentConversationProjection{Conversation: record,
		Messages: make([]ledger.AgentConversationMessage, 0), Turns: make([]ledger.AgentConversationTurn, 0),
		Targets: make([]ledger.AgentConversationTarget, 0)}
	messageRows, err := s.db.QueryContext(ctx, `SELECT id,conversation_id,COALESCE(turn_id,''),COALESCE(target_id,''),
author_kind,author_id,body,created_at FROM conversation_message WHERE conversation_id=? ORDER BY id`, conversationID)
	if err != nil {
		return ledger.AgentConversationProjection{}, err
	}
	for messageRows.Next() {
		var message ledger.AgentConversationMessage
		var created string
		if err := messageRows.Scan(&message.ID, &message.ConversationID, &message.TurnID, &message.TargetID,
			&message.AuthorKind, &message.AuthorID, &message.Body, &created); err != nil {
			messageRows.Close()
			return ledger.AgentConversationProjection{}, err
		}
		message.CreatedAt = parseTime(created)
		projection.Messages = append(projection.Messages, message)
	}
	if err := messageRows.Close(); err != nil {
		return ledger.AgentConversationProjection{}, err
	}
	turnRows, err := s.db.QueryContext(ctx, `SELECT turn.id,turn.conversation_id,turn.client_turn_id,
turn.prompt_message_id,turn.through_message_id,direct.membership_revision_id,direct.context_manifest_id,
direct.state,direct.created_at FROM stable_agent_direct_turn direct JOIN conversation_turn turn ON turn.id=direct.turn_id
WHERE direct.account_id=? AND direct.agent_id=? AND direct.conversation_id=? ORDER BY turn.prompt_message_id,turn.id`,
		accountID, agentID, conversationID)
	if err != nil {
		return ledger.AgentConversationProjection{}, err
	}
	for turnRows.Next() {
		var turn ledger.AgentConversationTurn
		var created string
		if err := turnRows.Scan(&turn.ID, &turn.ConversationID, &turn.ClientTurnID, &turn.PromptMessageID,
			&turn.ThroughMessageID, &turn.MembershipRevisionID, &turn.ContextManifestID, &turn.State, &created); err != nil {
			turnRows.Close()
			return ledger.AgentConversationProjection{}, err
		}
		turn.CreatedAt = parseTime(created)
		projection.Turns = append(projection.Turns, turn)
	}
	if err := turnRows.Close(); err != nil {
		return ledger.AgentConversationProjection{}, err
	}
	targetRows, err := s.db.QueryContext(ctx, `SELECT target.id,target.turn_id,direct.conversation_id,pin.agent_id,
pin.behavior_revision_id,pin.binding_revision_id,pin.participant_id,target.run_id,target.state,target.attempt,
target.created_at,target.updated_at FROM stable_agent_direct_target_binding pin
JOIN stable_agent_direct_turn direct ON direct.turn_id=pin.turn_id
JOIN conversation_turn turn_record ON turn_record.id=direct.turn_id
JOIN conversation_target target ON target.id=pin.target_id
WHERE pin.account_id=? AND pin.agent_id=? AND direct.conversation_id=? ORDER BY turn_record.prompt_message_id,target.id`,
		accountID, agentID, conversationID)
	if err != nil {
		return ledger.AgentConversationProjection{}, err
	}
	for targetRows.Next() {
		var target ledger.AgentConversationTarget
		var created, updated string
		var legacyAttempt int
		if err := targetRows.Scan(&target.ID, &target.TurnID, &target.ConversationID, &target.AgentID,
			&target.BehaviorRevisionID, &target.BindingRevisionID, &target.ParticipantID, &target.RunID,
			&target.State, &legacyAttempt, &created, &updated); err != nil {
			targetRows.Close()
			return ledger.AgentConversationProjection{}, err
		}
		target.AttemptCount = max(0, legacyAttempt-1)
		target.CreatedAt, target.UpdatedAt = parseTime(created), parseTime(updated)
		projection.Targets = append(projection.Targets, target)
	}
	if err := targetRows.Close(); err != nil {
		return ledger.AgentConversationProjection{}, err
	}
	return projection, nil
}

func (s *Store) RetryAgentTarget(ctx context.Context, command ledger.RetryAgentTargetCommand) (ledger.AgentConversationTarget, error) {
	if err := command.Validate(); err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	defer tx.Rollback()
	if _, replay, err := findSQLiteDirectIdempotency(ctx, tx, command.AccountID, "agent.target.retry", command.IdempotencyKey, digest); err != nil {
		return ledger.AgentConversationTarget{}, err
	} else if replay {
		target, err := getSQLiteAgentTarget(ctx, tx, command.AccountID, command.AgentID, command.ConversationID, command.TargetID)
		if err != nil {
			return ledger.AgentConversationTarget{}, err
		}
		if err := tx.Commit(); err != nil {
			return ledger.AgentConversationTarget{}, err
		}
		return target, nil
	}
	target, err := getSQLiteAgentTarget(ctx, tx, command.AccountID, command.AgentID, command.ConversationID, command.TargetID)
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	if err := requireSQLiteAgentConversationOpen(ctx, tx, command.AccountID, command.AgentID, command.ConversationID); err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	if target.State != string(conversation.TargetFailed) && target.State != string(conversation.TargetCanceled) && target.State != "needs_you" {
		return ledger.AgentConversationTarget{}, fmt.Errorf("%w: only terminal Agent Targets can be retried", ledger.ErrStateConflict)
	}
	var binding conversation.AgentBindingRevision
	binding, err = scanBindingRevision(tx.QueryRowContext(ctx, `SELECT id,agent_id,revision,behavior_revision_id,
execution_source_id,source_agent_id,seat_id,fort_profile,provider,requested_model,resolved_model,computer_id,
cloud_runtime,adapter_id,adapter_revision,source_config_digest,authority_id,authority_revision,policy_id,
policy_revision,session_behavior,memory_behavior,capability_evidence_json,readiness_contract_id,
readiness_contract_revision,supersedes_revision_id,activated_at,retired_at FROM agent_binding_revision
WHERE id=? AND agent_id=?`, target.BindingRevisionID, command.AgentID))
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	if err := verifySQLiteSourceConfiguration(ctx, tx, command.AccountID, binding); err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	updated := nowOr(command.RetriedAt)
	result, err := tx.ExecContext(ctx, `UPDATE conversation_target SET state='queued',attempt=attempt+1,error_code=NULL,error=NULL,updated_at=?
WHERE id=? AND state IN ('failed','canceled','needs_you')`, updated, command.TargetID)
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	if changed, err := result.RowsAffected(); err != nil || changed != 1 {
		if err != nil {
			return ledger.AgentConversationTarget{}, err
		}
		return ledger.AgentConversationTarget{}, fmt.Errorf("%w: Agent Target retry race", ledger.ErrStateConflict)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_agent_lifecycle_idempotency(
account_id,scope,idempotency_key,command_digest,result_id,created_at) VALUES(?,?,?,?,?,?)`, command.AccountID,
		"agent.target.retry", command.IdempotencyKey, digest, command.TargetID, updated); err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	if err := insertSQLiteAgentLifecycleEvent(ctx, tx, command.AccountID, command.AgentID, "agent.target.retried",
		map[string]any{"conversation_id": command.ConversationID, "target_id": command.TargetID,
			"behavior_revision_id": target.BehaviorRevisionID, "binding_revision_id": target.BindingRevisionID,
			"retried_by": command.RetriedBy}, command.RetriedAt); err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	target, err = getSQLiteAgentTarget(ctx, tx, command.AccountID, command.AgentID, command.ConversationID, command.TargetID)
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	return target, nil
}

func (s *Store) CancelAgentTarget(ctx context.Context, command ledger.CancelAgentTargetCommand) (ledger.AgentConversationTarget, error) {
	if err := command.Validate(); err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	defer tx.Rollback()
	if _, replay, err := findSQLiteDirectIdempotency(ctx, tx, command.AccountID, "agent.target.cancel", command.IdempotencyKey, digest); err != nil {
		return ledger.AgentConversationTarget{}, err
	} else if replay {
		target, err := getSQLiteAgentTarget(ctx, tx, command.AccountID, command.AgentID, command.ConversationID, command.TargetID)
		if err != nil {
			return ledger.AgentConversationTarget{}, err
		}
		if err := tx.Commit(); err != nil {
			return ledger.AgentConversationTarget{}, err
		}
		return target, nil
	}
	target, err := getSQLiteAgentTarget(ctx, tx, command.AccountID, command.AgentID, command.ConversationID, command.TargetID)
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	nextState := ""
	switch target.State {
	case "queued", "lease_expired":
		nextState = "canceled"
	case "claimed", "working":
		nextState = "cancel_requested"
	default:
		return ledger.AgentConversationTarget{}, fmt.Errorf("%w: Agent Target cannot be canceled from %s", ledger.ErrStateConflict, target.State)
	}
	updated := nowOr(command.CanceledAt)
	result, err := tx.ExecContext(ctx, `UPDATE conversation_target SET state=?,updated_at=? WHERE id=? AND state=?`,
		nextState, updated, command.TargetID, target.State)
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	if changed, err := result.RowsAffected(); err != nil || changed != 1 {
		if err != nil {
			return ledger.AgentConversationTarget{}, err
		}
		return ledger.AgentConversationTarget{}, fmt.Errorf("%w: Agent Target cancellation race", ledger.ErrStateConflict)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_agent_lifecycle_idempotency(
account_id,scope,idempotency_key,command_digest,result_id,created_at) VALUES(?,?,?,?,?,?)`, command.AccountID,
		"agent.target.cancel", command.IdempotencyKey, digest, command.TargetID, updated); err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	eventType := "agent.target.cancel_requested"
	if nextState == "canceled" {
		eventType = "agent.target.canceled"
	}
	if err := insertSQLiteAgentLifecycleEvent(ctx, tx, command.AccountID, command.AgentID, eventType,
		map[string]any{"conversation_id": command.ConversationID, "target_id": command.TargetID,
			"state": nextState, "canceled_by": command.CanceledBy,
			"behavior_revision_id": target.BehaviorRevisionID, "binding_revision_id": target.BindingRevisionID},
		command.CanceledAt); err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	target, err = getSQLiteAgentTarget(ctx, tx, command.AccountID, command.AgentID, command.ConversationID, command.TargetID)
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	return target, nil
}

func (s *Store) ObserveExecutionSourceConfig(ctx context.Context, command ledger.ObserveExecutionSourceConfigCommand) (ledger.ExecutionSourceConfigObservation, error) {
	if err := command.Validate(); err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	defer tx.Rollback()

	if resultID, replay, err := findSQLiteDirectIdempotency(ctx, tx, command.AccountID,
		"execution_source.config.observe", command.IdempotencyKey, digest); err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	} else if replay {
		observation, err := getSQLiteSourceConfigObservation(ctx, tx, command.AccountID, resultID)
		if err != nil {
			return ledger.ExecutionSourceConfigObservation{}, err
		}
		if err := tx.Commit(); err != nil {
			return ledger.ExecutionSourceConfigObservation{}, err
		}
		return observation, nil
	}

	var sourceExists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM execution_source WHERE account_id=? AND id=?`,
		command.AccountID, command.ExecutionSourceID).Scan(&sourceExists); err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	if sourceExists != 1 {
		return ledger.ExecutionSourceConfigObservation{}, fmt.Errorf("%w: Execution Source %q", ledger.ErrNotFound,
			command.ExecutionSourceID)
	}
	if err := appendSQLiteSourceConfiguration(ctx, tx, command.ObservationID, command.AccountID,
		command.ExecutionSourceID, command.SourceConfigDigest, command.ObservedBy, command.ObservedAt); err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_agent_lifecycle_idempotency(
account_id,scope,idempotency_key,command_digest,result_id,created_at) VALUES(?,?,?,?,?,?)`, command.AccountID,
		"execution_source.config.observe", command.IdempotencyKey, digest, command.ObservationID,
		nowOr(command.ObservedAt)); err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	observation, err := getSQLiteSourceConfigObservation(ctx, tx, command.AccountID, command.ObservationID)
	if err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	return observation, nil
}

func (s *Store) LatestExecutionSourceConfigObservation(ctx context.Context, accountID, executionSourceID string) (ledger.ExecutionSourceConfigObservation, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(accountID) != accountID ||
		strings.TrimSpace(executionSourceID) == "" || strings.TrimSpace(executionSourceID) != executionSourceID {
		return ledger.ExecutionSourceConfigObservation{}, fmt.Errorf("account and Execution Source ids are required and must be canonical")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	defer tx.Rollback()
	observation, err := latestSQLiteSourceConfigObservation(ctx, tx, accountID, executionSourceID)
	if err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	return observation, nil
}

func appendSQLiteSourceConfiguration(ctx context.Context, tx *sql.Tx, observationID, accountID, sourceID,
	digest, observedBy string, observedAt time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO execution_source_config_observation(
observation_id,account_id,execution_source_id,source_config_digest,observed_by,observed_at)
VALUES(?,?,?,?,?,?)`, observationID, accountID, sourceID, digest, observedBy, nowOr(observedAt))
	return err
}

func getSQLiteSourceConfigObservation(ctx context.Context, tx *sql.Tx, accountID, observationID string) (ledger.ExecutionSourceConfigObservation, error) {
	var observation ledger.ExecutionSourceConfigObservation
	var observedAt string
	err := tx.QueryRowContext(ctx, `SELECT observation_id,id,account_id,execution_source_id,
source_config_digest,observed_by,observed_at FROM execution_source_config_observation
WHERE account_id=? AND observation_id=?`, accountID, observationID).Scan(&observation.ID, &observation.Sequence,
		&observation.AccountID, &observation.ExecutionSourceID, &observation.SourceConfigDigest,
		&observation.ObservedBy, &observedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.ExecutionSourceConfigObservation{}, fmt.Errorf("%w: Execution Source configuration observation %q",
			ledger.ErrNotFound, observationID)
	}
	if err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	observation.ObservedAt = parseTime(observedAt)
	return observation, nil
}

func latestSQLiteSourceConfigObservation(ctx context.Context, tx *sql.Tx, accountID, executionSourceID string) (ledger.ExecutionSourceConfigObservation, error) {
	var observation ledger.ExecutionSourceConfigObservation
	var observedAt string
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(observation_id,'source-observation:legacy:'||id),id,
account_id,execution_source_id,source_config_digest,COALESCE(observed_by,'fort-control:legacy'),observed_at
FROM execution_source_config_observation WHERE account_id=? AND execution_source_id=? ORDER BY id DESC LIMIT 1`,
		accountID, executionSourceID).Scan(&observation.ID, &observation.Sequence, &observation.AccountID,
		&observation.ExecutionSourceID, &observation.SourceConfigDigest, &observation.ObservedBy, &observedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.ExecutionSourceConfigObservation{}, fmt.Errorf("%w: Execution Source configuration observation",
			ledger.ErrNotFound)
	}
	if err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	observation.ObservedAt = parseTime(observedAt)
	return observation, nil
}

func verifySQLiteSourceConfiguration(ctx context.Context, tx *sql.Tx, accountID string, binding conversation.AgentBindingRevision) error {
	observation, err := latestSQLiteSourceConfigObservation(ctx, tx, accountID, binding.ExecutionSourceID)
	if errors.Is(err, ledger.ErrNotFound) || (err == nil && observation.SourceConfigDigest != binding.SourceConfigDigest) {
		return fmt.Errorf("%w: Agent Binding source configuration drift", ledger.ErrSourceDrift)
	}
	return err
}

func requireSQLiteAgentConversationOpen(ctx context.Context, tx *sql.Tx, accountID, agentID, conversationID string) error {
	var agentState conversation.AgentState
	var itemState conversation.ConversationState
	err := tx.QueryRowContext(ctx, `SELECT agent.state,item.state
FROM stable_agent agent JOIN agent_conversation relation ON relation.agent_id=agent.id
JOIN conversation item ON item.id=relation.conversation_id
WHERE agent.account_id=? AND agent.id=? AND relation.conversation_id=?`, accountID, agentID, conversationID).Scan(
		&agentState, &itemState)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: Agent Conversation %q", ledger.ErrNotFound, conversationID)
	}
	if err != nil {
		return err
	}
	if agentState != conversation.AgentOpen || itemState != conversation.ConversationOpen {
		return fmt.Errorf("%w: Agent and Conversation must be open", ledger.ErrStateConflict)
	}
	return nil
}

func findSQLiteDirectIdempotency(ctx context.Context, tx *sql.Tx, accountID, scope, key, digest string) (string, bool, error) {
	var existingDigest, resultID string
	err := tx.QueryRowContext(ctx, `SELECT command_digest,result_id FROM stable_agent_lifecycle_idempotency
WHERE account_id=? AND scope=? AND idempotency_key=?`, accountID, scope, key).Scan(&existingDigest, &resultID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if existingDigest != digest {
		return "", false, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, key)
	}
	return resultID, true, nil
}

func ensureSQLiteDirectParticipant(ctx context.Context, tx *sql.Tx, current ledger.AgentRecord, conversationID string, createdAt time.Time) (conversation.Participant, error) {
	row := tx.QueryRowContext(ctx, `SELECT participant.id,participant.conversation_id,participant.seat_id,
participant.profile,participant.agent,participant.model,participant.machine,participant.display_name,
participant.position,participant.state,participant.created_at,participant.removed_at
FROM stable_agent_participant_evidence evidence JOIN conversation_participant participant ON participant.id=evidence.participant_id
WHERE evidence.agent_id=? AND evidence.binding_revision_id=? AND evidence.conversation_id=?`,
		current.Agent.ID, current.Binding.ID, conversationID)
	participant, err := scanStableAgentParticipant(row)
	if err == nil {
		return participant, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return conversation.Participant{}, err
	}
	machine := current.Binding.ComputerID
	if machine == "" {
		machine = current.Binding.CloudRuntime
	}
	participant = conversation.Participant{
		ID:             sqliteDirectParticipantID(current.Agent.AccountID, current.Agent.ID, conversationID, current.Binding.ID),
		ConversationID: conversationID, SeatID: current.Binding.SeatID, Profile: current.Binding.FortProfile,
		Agent: current.Binding.Provider, Model: current.Binding.RequestedModel, Machine: machine,
		DisplayName: current.Profile.Name, Position: 0, State: conversation.ParticipantActive, CreatedAt: createdAt.UTC(),
	}
	if err := insertSuccessorParticipant(ctx, tx, current.Agent.ID, current.Binding.ID, participant); err != nil {
		return conversation.Participant{}, err
	}
	return participant, nil
}

func getSQLiteAgentTurnDispatch(ctx context.Context, tx *sql.Tx, accountID, agentID, conversationID, turnID string) (ledger.AgentTurnDispatch, error) {
	var result ledger.AgentTurnDispatch
	var messageCreated, turnCreated, targetCreated, targetUpdated, manifestCreated string
	var legacyAttempt int
	err := tx.QueryRowContext(ctx, `SELECT message.id,message.conversation_id,COALESCE(message.turn_id,''),
COALESCE(message.target_id,''),message.author_kind,message.author_id,message.body,message.created_at,
turn.id,turn.conversation_id,turn.client_turn_id,turn.prompt_message_id,turn.through_message_id,
direct.membership_revision_id,direct.context_manifest_id,direct.state,direct.created_at,
manifest.id,manifest.conversation_id,manifest.through_message_id,manifest.manifest_digest,manifest.created_at,
target.id,target.turn_id,direct.conversation_id,pin.agent_id,pin.behavior_revision_id,pin.binding_revision_id,
pin.participant_id,target.run_id,target.state,target.attempt,target.created_at,target.updated_at
FROM stable_agent_direct_turn direct JOIN conversation_turn turn ON turn.id=direct.turn_id
JOIN conversation_message message ON message.id=turn.prompt_message_id
JOIN stable_agent_context_manifest manifest ON manifest.id=direct.context_manifest_id
JOIN stable_agent_direct_target_binding pin ON pin.turn_id=direct.turn_id
JOIN conversation_target target ON target.id=pin.target_id
WHERE direct.account_id=? AND direct.agent_id=? AND direct.conversation_id=? AND direct.turn_id=?`,
		accountID, agentID, conversationID, turnID).Scan(
		&result.Message.ID, &result.Message.ConversationID, &result.Message.TurnID, &result.Message.TargetID,
		&result.Message.AuthorKind, &result.Message.AuthorID, &result.Message.Body, &messageCreated,
		&result.Turn.ID, &result.Turn.ConversationID, &result.Turn.ClientTurnID, &result.Turn.PromptMessageID,
		&result.Turn.ThroughMessageID, &result.Turn.MembershipRevisionID, &result.Turn.ContextManifestID,
		&result.Turn.State, &turnCreated, &result.Context.ID, &result.Context.ConversationID,
		&result.Context.ThroughMessageID, &result.Context.Digest, &manifestCreated,
		&result.Target.ID, &result.Target.TurnID, &result.Target.ConversationID, &result.Target.AgentID,
		&result.Target.BehaviorRevisionID, &result.Target.BindingRevisionID, &result.Target.ParticipantID,
		&result.Target.RunID, &result.Target.State, &legacyAttempt, &targetCreated, &targetUpdated)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.AgentTurnDispatch{}, fmt.Errorf("%w: Agent Turn %q", ledger.ErrNotFound, turnID)
	}
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	result.Message.CreatedAt, result.Turn.CreatedAt = parseTime(messageCreated), parseTime(turnCreated)
	result.Context.CreatedAt = parseTime(manifestCreated)
	result.Target.AttemptCount = max(0, legacyAttempt-1)
	result.Target.CreatedAt, result.Target.UpdatedAt = parseTime(targetCreated), parseTime(targetUpdated)
	rows, err := tx.QueryContext(ctx, `SELECT message_id FROM stable_agent_context_manifest_message
WHERE manifest_id=? ORDER BY ordinal`, result.Context.ID)
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	result.Context.MessageIDs = make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return ledger.AgentTurnDispatch{}, err
		}
		result.Context.MessageIDs = append(result.Context.MessageIDs, id)
	}
	if err := rows.Close(); err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	return result, nil
}

func getSQLiteAgentTarget(ctx context.Context, tx *sql.Tx, accountID, agentID, conversationID, targetID string) (ledger.AgentConversationTarget, error) {
	var target ledger.AgentConversationTarget
	var created, updated string
	var legacyAttempt int
	err := tx.QueryRowContext(ctx, `SELECT target.id,target.turn_id,direct.conversation_id,pin.agent_id,
pin.behavior_revision_id,pin.binding_revision_id,pin.participant_id,target.run_id,target.state,target.attempt,
target.created_at,target.updated_at FROM stable_agent_direct_target_binding pin
JOIN stable_agent_direct_turn direct ON direct.turn_id=pin.turn_id
JOIN stable_agent agent ON agent.id=pin.agent_id AND agent.account_id=pin.account_id
JOIN agent_conversation relation ON relation.agent_id=agent.id AND relation.conversation_id=direct.conversation_id
JOIN conversation_target target ON target.id=pin.target_id
WHERE pin.account_id=? AND pin.agent_id=? AND direct.conversation_id=? AND pin.target_id=?`, accountID, agentID,
		conversationID, targetID).Scan(&target.ID, &target.TurnID, &target.ConversationID, &target.AgentID,
		&target.BehaviorRevisionID, &target.BindingRevisionID, &target.ParticipantID, &target.RunID,
		&target.State, &legacyAttempt, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.AgentConversationTarget{}, fmt.Errorf("%w: Agent Target %q", ledger.ErrNotFound, targetID)
	}
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	target.AttemptCount = max(0, legacyAttempt-1)
	target.CreatedAt, target.UpdatedAt = parseTime(created), parseTime(updated)
	return target, nil
}

func sqliteManifestDigest(conversationID string, throughMessageID int64, messageIDs []int64) (string, error) {
	payload, err := json.Marshal(struct {
		Version          int     `json:"version"`
		ConversationID   string  `json:"conversation_id"`
		ThroughMessageID int64   `json:"through_message_id"`
		MessageIDs       []int64 `json:"message_ids"`
	}{1, conversationID, throughMessageID, messageIDs})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func sqliteAgentMembershipID(accountID, agentID, conversationID string) string {
	digest := sha256.Sum256([]byte(accountID + "\x00" + agentID + "\x00" + conversationID))
	return "membership:agent:" + hex.EncodeToString(digest[:])
}

func sqliteDirectParticipantID(accountID, agentID, conversationID, bindingID string) string {
	digest := sha256.Sum256([]byte(accountID + "\x00" + agentID + "\x00" + conversationID + "\x00" + bindingID))
	return "participant:agent:" + hex.EncodeToString(digest[:])
}
