package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tobsai/fort/cloud/securebody"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

const groupTurnSendScope = "group.turn.send"

type collaborationPolicySnapshot struct {
	ID        string                      `json:"id"`
	Revision  string                      `json:"revision"`
	FortGroup *collaborationGroupTurnMeta `json:"fort_group,omitempty"`
}

type collaborationGroupTurnMeta struct {
	GroupID              string                               `json:"group_id"`
	Selection            conversation.GroupRecipientSelection `json:"selection"`
	Recipients           []conversation.GroupRecipient        `json:"recipients"`
	TargetIDs            []string                             `json:"target_ids"`
	CostLimitEvidenceID  string                               `json:"cost_limit_evidence_id,omitempty"`
	TokenLimitEvidenceID string                               `json:"token_limit_evidence_id,omitempty"`
}

func (store *Store) SendGroupTurn(ctx context.Context, command ledger.SendGroupTurnCommand) (ledger.GroupTurnRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	command.AccountID = accountID
	cipher, err := store.collaborationBodies()
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	var record ledger.GroupTurnRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, groupTurnSendScope,
			command.Envelope.IdempotencyKey, digest, "group_turn", command.Envelope.ID,
			command.Envelope.CreatedAt)
		if err != nil {
			return err
		}
		if !claimed {
			record, err = getPostgresGroupTurn(ctx, tx, cipher, accountID, command.Envelope.ID)
			return err
		}
		group, err := getPostgresGroup(ctx, tx, accountID, command.Envelope.GroupID)
		if err != nil {
			return err
		}
		if err := command.Validate(group.Group, group.Membership); err != nil {
			return err
		}
		positions := make(map[string]int, len(group.Membership.Members))
		for _, member := range group.Membership.Members {
			positions[member.AgentID] = member.Position
		}
		for _, recipient := range command.Envelope.Recipients {
			position, ok := positions[recipient.AgentID]
			if !ok {
				return fmt.Errorf("Group recipient %q is not a current stable-Agent member", recipient.AgentID)
			}
			if err := ensurePostgresCurrentGroupParticipant(ctx, tx, accountID, command.Envelope.GroupID,
				command.Envelope.ConversationID, position, recipient, command.Envelope.CreatedAt); err != nil {
				return err
			}
		}
		initial, err := command.Envelope.InitialTargets(group.Group, group.Membership)
		if err != nil {
			return err
		}
		body, err := cipher.seal(securebody.Scope{AccountID: accountID,
			RecordType: "group_turn_prompt", RecordID: command.Envelope.ID}, command.Body)
		if err != nil {
			return fmt.Errorf("encrypt Group Turn prompt: %w", err)
		}
		metadata := collaborationGroupTurnMeta{
			GroupID: command.Envelope.GroupID, Selection: command.Envelope.Selection,
			Recipients:           append([]conversation.GroupRecipient{}, command.Envelope.Recipients...),
			TargetIDs:            append([]string{}, command.TargetIDs...),
			CostLimitEvidenceID:  command.Envelope.CostLimitEvidenceID,
			TokenLimitEvidenceID: command.Envelope.TokenLimitEvidenceID,
		}
		cancellationJSON, err := json.Marshal(collaborationPolicySnapshot{
			ID: command.Envelope.CancellationPolicyID, Revision: command.Envelope.CancellationPolicyRevision,
		})
		if err != nil {
			return err
		}
		approvalJSON, err := json.Marshal(collaborationPolicySnapshot{
			ID: command.Envelope.ApprovalPolicyID, Revision: command.Envelope.ApprovalPolicyRevision,
			FortGroup: &metadata,
		})
		if err != nil {
			return err
		}
		var messageID int64
		if err := tx.queryRow(ctx, `insert into fort_private.conversation_message (
  account_id, conversation_id, turn_id, target_id, handoff_id, routine_run_id,
  message_kind, author_kind, author_id, author_agent_id,
  body_ciphertext, body_key_id, body_nonce, body_digest,
  body_plaintext_length, created_at
) values ($1,$2,$3,null,null,null,'human','human',$4,null,$5,$6,$7,$8,$9,$10)
returning message_id`, accountID, command.Envelope.ConversationID, command.Envelope.ID,
			command.HumanID, body.Ciphertext, body.KeyID, body.Nonce, body.Digest,
			body.PlaintextBytes, command.Envelope.CreatedAt.UTC()).scan(&messageID); err != nil {
			return fmt.Errorf("insert Group prompt message: %w", err)
		}
		messageIDs, err := loadPostgresGroupContextMessageIDs(ctx, tx, accountID,
			command.Envelope.ConversationID, messageID)
		if err != nil {
			return err
		}
		contextDigest, err := evidenceDigest(struct {
			Version          int     `json:"version"`
			ConversationID   string  `json:"conversation_id"`
			ThroughMessageID int64   `json:"through_message_id"`
			MessageIDs       []int64 `json:"message_ids"`
		}{Version: 1, ConversationID: command.Envelope.ConversationID,
			ThroughMessageID: messageID, MessageIDs: messageIDs})
		if err != nil {
			return err
		}
		contextRecordIDs := make([]string, 0, len(messageIDs))
		for _, frozenMessageID := range messageIDs {
			contextRecordIDs = append(contextRecordIDs, "message:"+fmt.Sprint(frozenMessageID))
		}
		command.Envelope.RootDelegationGrant.ContextRecordIDs = contextRecordIDs
		grantJSON, err := json.Marshal(command.Envelope.RootDelegationGrant)
		if err != nil {
			return err
		}
		grantDigest, err := evidenceDigest(command.Envelope.RootDelegationGrant)
		if err != nil {
			return err
		}
		if _, err := tx.exec(ctx, `insert into fort_private.delegation_grant (
  account_id, delegation_grant_id, source_kind, source_id, authority_grant,
  grant_digest, maximum_agent_messages, maximum_handoff_depth,
  hard_deadline, created_by, created_at
) values ($1,$2,'human_turn',$3,$4::jsonb,$5,$6,$7,$8,$9,$10)`, accountID,
			command.Envelope.RootDelegationGrant.ID, command.Envelope.ID, string(grantJSON), grantDigest,
			command.Envelope.MaxAgentMessages, command.Envelope.MaxHandoffDepth,
			command.Envelope.Deadline.UTC(), command.HumanID, command.Envelope.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Group delegation grant: %w", err)
		}
		if _, err := tx.exec(ctx, `insert into fort_private.context_manifest (
  account_id, context_manifest_id, purpose, manifest_digest, created_by, created_at
) values ($1,$2,'turn',$3,$4,$5)`, accountID, command.Envelope.ContextSnapshotID,
			contextDigest, command.HumanID, command.Envelope.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Group context manifest: %w", err)
		}
		for ordinal, frozenMessageID := range messageIDs {
			if _, err := tx.exec(ctx, `insert into fort_private.context_manifest_message (
  account_id, context_manifest_id, ordinal, message_id
) values ($1,$2,$3,$4)`, accountID, command.Envelope.ContextSnapshotID,
				ordinal, frozenMessageID); err != nil {
				return fmt.Errorf("insert Group context manifest message: %w", err)
			}
		}
		concurrency, err := postgresGroupConcurrency(command.Envelope.ConcurrencyPolicy)
		if err != nil {
			return err
		}
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_turn (
  account_id, turn_id, conversation_id, client_turn_id, idempotency_key,
  command_digest, kind, prompt_message_id, through_message_id,
  membership_revision_id, context_manifest_id, delegation_grant_id,
  concurrency_policy, cancellation_policy, approval_policy,
  maximum_agent_messages, maximum_handoff_depth,
  cost_limit_classification, token_limit_classification,
  cost_limit, token_limit, hard_deadline, state, created_at, updated_at
) values ($1,$2,$3,$4,$5,$6,'human_group',$7,$7,$8,$9,$10,$11,$12::jsonb,$13::jsonb,
  $14,$15,$16,$17,null,null,$18,'open',$19,$19)`, accountID, command.Envelope.ID,
			command.Envelope.ConversationID, command.Envelope.ClientTurnID,
			command.Envelope.IdempotencyKey, digest, messageID,
			command.Envelope.MembershipRevisionID, command.Envelope.ContextSnapshotID,
			command.Envelope.RootDelegationGrant.ID, concurrency, string(cancellationJSON),
			string(approvalJSON), command.Envelope.MaxAgentMessages, command.Envelope.MaxHandoffDepth,
			command.Envelope.CostLimitClass, command.Envelope.TokenLimitClass,
			command.Envelope.Deadline.UTC(), command.Envelope.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Group Turn: %w", err)
		}
		for index, target := range initial {
			targetID := command.TargetIDs[index]
			if _, err := tx.exec(ctx, `insert into fort_private.conversation_target (
  account_id, target_id, turn_id, conversation_id, agent_id,
  membership_revision_id, target_kind, origin_id, run_id, state,
  attempt_count, hard_deadline, cancellation_policy, created_at, updated_at
) values ($1,$2,$3,$4,$5,$6,'initial',$3,$2,'queued',0,$7,$8::jsonb,$9,$9)`,
				accountID, targetID, command.Envelope.ID, command.Envelope.ConversationID,
				target.AgentID, command.Envelope.MembershipRevisionID,
				command.Envelope.Deadline.UTC(), string(cancellationJSON), command.Envelope.CreatedAt.UTC()); err != nil {
				return fmt.Errorf("insert Group initial target: %w", err)
			}
			if _, err := tx.exec(ctx, `insert into fort_private.conversation_target_binding (
  account_id, target_id, conversation_id, agent_id, behavior_revision_id,
  binding_revision_id, participant_id, membership_revision_id, pinned_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, accountID, targetID,
				command.Envelope.ConversationID, target.AgentID, target.BehaviorRevisionID,
				target.BindingRevisionID, target.ParticipantID,
				command.Envelope.MembershipRevisionID, command.Envelope.CreatedAt.UTC()); err != nil {
				return fmt.Errorf("insert Group target binding: %w", err)
			}
		}
		if _, err := tx.exec(ctx, `update fort_private.conversation
set updated_at = $3 where account_id = $1 and conversation_id = $2`, accountID,
			command.Envelope.ConversationID, command.Envelope.CreatedAt.UTC()); err != nil {
			return err
		}
		record = groupTurnRecord(command, messageID, initial)
		return nil
	})
	return record, err
}

func ensurePostgresCurrentGroupParticipant(ctx context.Context, tx transaction, accountID, groupID,
	conversationID string, position int, recipient conversation.GroupRecipient, createdAt time.Time) error {
	var agent ledger.AgentRecord
	var state string
	var currentBehaviorID, currentBindingID, workerID string
	err := tx.queryRow(ctx, `select agent.state, agent.current_behavior_revision_id,
  agent.current_binding_revision_id, behavior.behavior_revision_id,
  binding.binding_revision_id, binding.agent_id, binding.behavior_revision_id,
  binding.seat_id, binding.fort_profile, binding.provider, binding.requested_model,
  binding.worker_id, binding.authority_id, binding.authority_revision,
  binding.policy_id, binding.policy_revision, profile.name
from fort_private.stable_agent as agent
join fort_private.agent_behavior_revision as behavior
  on behavior.account_id = agent.account_id and behavior.agent_id = agent.agent_id
 and behavior.behavior_revision_id = agent.current_behavior_revision_id
join fort_private.agent_binding_revision as binding
  on binding.account_id = agent.account_id and binding.agent_id = agent.agent_id
 and binding.binding_revision_id = agent.current_binding_revision_id
 and binding.behavior_revision_id = behavior.behavior_revision_id
join fort_private.agent_profile_revision as profile
  on profile.account_id = agent.account_id and profile.agent_id = agent.agent_id
 and profile.profile_revision_id = agent.current_profile_revision_id
where agent.account_id = $1 and agent.agent_id = $2
for update of agent`, accountID, recipient.AgentID).scan(
		&state, &currentBehaviorID, &currentBindingID, &agent.Behavior.ID,
		&agent.Binding.ID, &agent.Binding.AgentID, &agent.Binding.BehaviorRevisionID,
		&agent.Binding.SeatID, &agent.Binding.FortProfile, &agent.Binding.Provider,
		&agent.Binding.RequestedModel, &workerID, &agent.Binding.AuthorityID,
		&agent.Binding.AuthorityRevision, &agent.Binding.PolicyID,
		&agent.Binding.PolicyRevision, &agent.Profile.Name,
	)
	if isNoRows(err) {
		return fmt.Errorf("%w: current Group Agent %q", ledger.ErrNotFound, recipient.AgentID)
	}
	if err != nil {
		return err
	}
	agent.Agent = conversation.Agent{ID: recipient.AgentID, AccountID: accountID,
		State: conversation.AgentState(state), CurrentBehaviorRevisionID: currentBehaviorID,
		CurrentBindingRevisionID: currentBindingID}
	agent.Behavior.AgentID = recipient.AgentID
	agent.Binding.ComputerID = workerID
	if agent.Agent.State != conversation.AgentOpen || currentBehaviorID != recipient.BehaviorRevisionID ||
		currentBindingID != recipient.BindingRevisionID || agent.Behavior.ID != recipient.BehaviorRevisionID ||
		agent.Binding.ID != recipient.BindingRevisionID || agent.Binding.AgentID != recipient.AgentID ||
		agent.Binding.BehaviorRevisionID != recipient.BehaviorRevisionID {
		return fmt.Errorf("%w: Group recipient %q current Behavior/Binding changed", ledger.ErrRevisionConflict, recipient.AgentID)
	}
	if recipient.ParticipantID != postgresGroupParticipantID(accountID, groupID, recipient.AgentID, recipient.BindingRevisionID) {
		return fmt.Errorf("Group recipient %q participant identity is not canonical", recipient.AgentID)
	}
	var participantID string
	err = tx.queryRow(ctx, `select participant_id from fort_private.conversation_participant
where account_id=$1 and conversation_id=$2 and agent_id=$3
  and behavior_revision_id=$4 and binding_revision_id=$5`, accountID, conversationID,
		recipient.AgentID, recipient.BehaviorRevisionID, recipient.BindingRevisionID).scan(&participantID)
	if err == nil {
		if participantID != recipient.ParticipantID {
			return fmt.Errorf("Group recipient %q participant evidence conflicts", recipient.AgentID)
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return insertPostgresGroupParticipant(ctx, tx, accountID, conversationID, position, agent, recipient, createdAt)
}

func postgresGroupParticipantID(accountID, groupID, agentID, bindingID string) string {
	payload, _ := json.Marshal([]string{accountID, groupID, agentID, bindingID})
	digest := sha256.Sum256(payload)
	return "participant:v2:" + hex.EncodeToString(digest[:16])
}

func loadPostgresGroupContextMessageIDs(ctx context.Context, tx transaction, accountID,
	conversationID string, throughMessageID int64) ([]int64, error) {
	var ids []int64
	err := tx.queryRow(ctx, `select coalesce(
  array_agg(message_id order by message_id), '{}'::bigint[]
)
from (
  select message_id
  from fort_private.conversation_message
  where account_id = $1 and conversation_id = $2 and message_id <= $3
  order by message_id
  limit 257
) as frozen_messages`, accountID, conversationID, throughMessageID).scan(&ids)
	if err != nil {
		return nil, fmt.Errorf("freeze Group context messages: %w", err)
	}
	if len(ids) > 256 {
		return nil, fmt.Errorf("Group context exceeds 256 committed messages")
	}
	if len(ids) == 0 || ids[len(ids)-1] != throughMessageID {
		return nil, fmt.Errorf("Group prompt is absent from frozen context")
	}
	return append([]int64(nil), ids...), nil
}

func (store *Store) ListGroupTurns(ctx context.Context, accountID, groupID string) ([]ledger.GroupTurnRecord, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return nil, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return nil, err
	}
	records := make([]ledger.GroupTurnRecord, 0)
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		group, err := getPostgresGroup(ctx, tx, accountID, groupID)
		if err != nil {
			return err
		}
		result, err := tx.query(ctx, `select turn_id
from fort_private.conversation_turn
where account_id = $1 and conversation_id = $2 and kind = 'human_group'
order by created_at, turn_id`, accountID, group.Group.ConversationID)
		if err != nil {
			return err
		}
		defer result.close()
		ids := make([]string, 0)
		for result.next() {
			var id string
			if err := result.scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		if err := result.errResult(); err != nil {
			return err
		}
		for _, id := range ids {
			record, err := getPostgresGroupTurn(ctx, tx, cipher, accountID, id)
			if err != nil {
				return err
			}
			records = append(records, record)
		}
		return nil
	})
	return records, err
}

// ListGroupMessages returns the ordered, decrypted Group transcript through
// the account -> Group -> Conversation parent chain. Agent messages retain the
// exact target link that pins their Behavior and Binding revisions.
func (store *Store) ListGroupMessages(ctx context.Context, accountID, groupID string) ([]ledger.AgentConversationMessage, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return nil, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return nil, err
	}
	messages := make([]ledger.AgentConversationMessage, 0)
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		var conversationID string
		if err := tx.queryRow(ctx, `select conversation_id
from fort_private.group_conversation
where account_id = $1 and group_id = $2`, accountID, groupID).scan(&conversationID); err != nil {
			if isNoRows(err) {
				return fmt.Errorf("%w: Group %q", ledger.ErrNotFound, groupID)
			}
			return err
		}
		rows, err := tx.query(ctx, `select message_id, conversation_id, coalesce(turn_id,''),
  coalesce(target_id,''), coalesce(handoff_id,''), coalesce(routine_run_id,''),
  message_kind, author_kind, author_id, coalesce(author_agent_id,''),
  body_ciphertext, body_key_id, body_nonce, body_digest, body_plaintext_length, created_at
from fort_private.conversation_message
where account_id = $1 and conversation_id = $2
order by message_id`, accountID, conversationID)
		if err != nil {
			return err
		}
		defer rows.close()
		for rows.next() {
			var message ledger.AgentConversationMessage
			var messageKind, handoffID, routineRunID string
			var encrypted collaborationEncryptedBody
			if err := rows.scan(&message.ID, &message.ConversationID, &message.TurnID,
				&message.TargetID, &handoffID, &routineRunID, &messageKind,
				&message.AuthorKind, &message.AuthorID, &message.AuthorAgentID,
				&encrypted.Ciphertext, &encrypted.KeyID, &encrypted.Nonce,
				&encrypted.Digest, &encrypted.PlaintextBytes, &message.CreatedAt); err != nil {
				return err
			}
			recordType, recordID, err := groupMessageEncryptionScope(messageKind, message, handoffID, routineRunID)
			if err != nil {
				return err
			}
			message.Body, err = cipher.open(securebody.Scope{AccountID: accountID,
				RecordType: recordType, RecordID: recordID}, encrypted)
			if err != nil {
				return fmt.Errorf("decrypt Group message %d: %w", message.ID, err)
			}
			messages = append(messages, message)
		}
		return rows.errResult()
	})
	return messages, err
}

func groupMessageEncryptionScope(messageKind string, message ledger.AgentConversationMessage, handoffID,
	routineRunID string) (string, string, error) {
	switch messageKind {
	case "human":
		if message.TurnID != "" {
			return "group_turn_prompt", message.TurnID, nil
		}
	case "agent":
		if message.TargetID != "" {
			return "conversation_message", message.TargetID, nil
		}
	case "handoff_result":
		if handoffID != "" {
			return "handoff_result", handoffID, nil
		}
	case "routine_result":
		if routineRunID != "" {
			return "routine_result", routineRunID, nil
		}
	}
	return "", "", fmt.Errorf("Group message %d has unsupported or incomplete encryption scope", message.ID)
}

func getPostgresGroupTurn(ctx context.Context, tx transaction, cipher collaborationBodyCipher,
	accountID, turnID string) (ledger.GroupTurnRecord, error) {
	var record ledger.GroupTurnRecord
	var concurrency string
	var cancellationJSON, approvalJSON, grantJSON string
	var throughMessageID int64
	var manifestDigest string
	var encrypted collaborationEncryptedBody
	err := tx.queryRow(ctx, `select turn.turn_id, turn.conversation_id, turn.client_turn_id,
  turn.idempotency_key, turn.membership_revision_id, turn.context_manifest_id,
  turn.through_message_id, manifest.manifest_digest,
  turn.concurrency_policy, turn.cancellation_policy::text, turn.approval_policy::text,
  turn.maximum_agent_messages, turn.maximum_handoff_depth,
  turn.cost_limit_classification, turn.token_limit_classification,
  turn.hard_deadline, turn.created_at,
  delegation.authority_grant::text,
  message.message_id, message.author_id, message.body_ciphertext,
  message.body_key_id, message.body_nonce, message.body_digest,
  message.body_plaintext_length, message.created_at
from fort_private.conversation_turn as turn
join fort_private.delegation_grant as delegation
  on delegation.account_id = turn.account_id and delegation.delegation_grant_id = turn.delegation_grant_id
join fort_private.context_manifest as manifest
  on manifest.account_id = turn.account_id and manifest.context_manifest_id = turn.context_manifest_id
join fort_private.conversation_message as message
  on message.account_id = turn.account_id and message.message_id = turn.prompt_message_id
where turn.account_id = $1 and turn.turn_id = $2 and turn.kind = 'human_group'`, accountID, turnID).scan(
		&record.Envelope.ID, &record.Envelope.ConversationID, &record.Envelope.ClientTurnID,
		&record.Envelope.IdempotencyKey, &record.Envelope.MembershipRevisionID,
		&record.Envelope.ContextSnapshotID, &throughMessageID, &manifestDigest,
		&concurrency, &cancellationJSON, &approvalJSON,
		&record.Envelope.MaxAgentMessages, &record.Envelope.MaxHandoffDepth,
		&record.Envelope.CostLimitClass, &record.Envelope.TokenLimitClass,
		&record.Envelope.Deadline, &record.Envelope.CreatedAt, &grantJSON,
		&record.Message.ID, &record.Message.AuthorID, &encrypted.Ciphertext,
		&encrypted.KeyID, &encrypted.Nonce, &encrypted.Digest,
		&encrypted.PlaintextBytes, &record.Message.CreatedAt)
	if isNoRows(err) {
		return ledger.GroupTurnRecord{}, fmt.Errorf("%w: Group Turn %q", ledger.ErrNotFound, turnID)
	}
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	if err := json.Unmarshal([]byte(grantJSON), &record.Envelope.RootDelegationGrant); err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	var cancellation, approval collaborationPolicySnapshot
	if err := json.Unmarshal([]byte(cancellationJSON), &cancellation); err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	if err := json.Unmarshal([]byte(approvalJSON), &approval); err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	if approval.FortGroup == nil {
		return ledger.GroupTurnRecord{}, fmt.Errorf("persisted Group Turn metadata is missing")
	}
	meta := approval.FortGroup
	record.Envelope.GroupID = meta.GroupID
	record.Envelope.Selection = meta.Selection
	record.Envelope.CancellationPolicyID = cancellation.ID
	record.Envelope.CancellationPolicyRevision = cancellation.Revision
	record.Envelope.ApprovalPolicyID = approval.ID
	record.Envelope.ApprovalPolicyRevision = approval.Revision
	record.Envelope.CostLimitEvidenceID = meta.CostLimitEvidenceID
	record.Envelope.TokenLimitEvidenceID = meta.TokenLimitEvidenceID
	record.Envelope.ConcurrencyPolicy, err = domainGroupConcurrency(concurrency)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	record.Message.ConversationID = record.Envelope.ConversationID
	record.Message.TurnID = record.Envelope.ID
	record.Message.AuthorKind = conversation.AuthorHuman
	record.Message.Body, err = cipher.open(securebody.Scope{AccountID: accountID,
		RecordType: "group_turn_prompt", RecordID: record.Envelope.ID}, encrypted)
	if err != nil {
		return ledger.GroupTurnRecord{}, fmt.Errorf("decrypt Group Turn prompt: %w", err)
	}
	frozenMessageIDs, err := loadPostgresGroupManifestMessageIDs(ctx, tx, accountID,
		record.Envelope.ContextSnapshotID)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	selectedMessageIDs, err := loadPostgresGroupContextMessageIDs(ctx, tx, accountID,
		record.Envelope.ConversationID, throughMessageID)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	wantManifestDigest, err := evidenceDigest(struct {
		Version          int     `json:"version"`
		ConversationID   string  `json:"conversation_id"`
		ThroughMessageID int64   `json:"through_message_id"`
		MessageIDs       []int64 `json:"message_ids"`
	}{1, record.Envelope.ConversationID, throughMessageID, frozenMessageIDs})
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	if throughMessageID != record.Message.ID || !reflect.DeepEqual(frozenMessageIDs, selectedMessageIDs) ||
		manifestDigest != wantManifestDigest {
		return ledger.GroupTurnRecord{}, fmt.Errorf("persisted Group context manifest is incomplete or inconsistent")
	}
	record.Recipients = append([]conversation.GroupRecipient{}, meta.Recipients...)
	record.Envelope.Recipients = append([]conversation.GroupRecipient{}, meta.Recipients...)
	record.InitialTargets = make([]ledger.InitialTargetRecord, 0, len(meta.TargetIDs))
	result, err := tx.query(ctx, `select target.target_id, target.agent_id,
  binding.behavior_revision_id, binding.binding_revision_id,
  binding.participant_id, target.state, target.created_at
from fort_private.conversation_target as target
join fort_private.conversation_target_binding as binding
  on binding.account_id = target.account_id and binding.target_id = target.target_id
where target.account_id = $1 and target.turn_id = $2 and target.target_kind = 'initial'`, accountID, turnID)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	defer result.close()
	byID := make(map[string]ledger.InitialTargetRecord, len(meta.TargetIDs))
	for result.next() {
		var target ledger.InitialTargetRecord
		var state string
		if err := result.scan(&target.ID, &target.AgentID, &target.BehaviorRevisionID,
			&target.BindingRevisionID, &target.ParticipantID, &state, &target.CreatedAt); err != nil {
			return ledger.GroupTurnRecord{}, err
		}
		target.GroupTurnID = turnID
		target.Wave = 0
		target.State, err = domainTargetState(state)
		if err != nil {
			return ledger.GroupTurnRecord{}, err
		}
		byID[target.ID] = target
	}
	if err := result.errResult(); err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	for index, id := range meta.TargetIDs {
		target, ok := byID[id]
		if !ok || index >= len(meta.Recipients) || target.AgentID != meta.Recipients[index].AgentID ||
			target.BehaviorRevisionID != meta.Recipients[index].BehaviorRevisionID ||
			target.BindingRevisionID != meta.Recipients[index].BindingRevisionID ||
			target.ParticipantID != meta.Recipients[index].ParticipantID {
			return ledger.GroupTurnRecord{}, fmt.Errorf("Group Turn frozen target evidence is incomplete or inconsistent")
		}
		record.InitialTargets = append(record.InitialTargets, target)
	}
	if len(byID) != len(meta.TargetIDs) {
		return ledger.GroupTurnRecord{}, fmt.Errorf("Group Turn contains a later or duplicate fan-out wave")
	}
	return record, nil
}

func loadPostgresGroupManifestMessageIDs(ctx context.Context, tx transaction, accountID,
	manifestID string) ([]int64, error) {
	var ids []int64
	err := tx.queryRow(ctx, `select coalesce(
  array_agg(message_id order by ordinal), '{}'::bigint[]
)
from fort_private.context_manifest_message
where account_id = $1 and context_manifest_id = $2`, accountID, manifestID).scan(&ids)
	if err != nil {
		return nil, fmt.Errorf("load frozen Group context manifest: %w", err)
	}
	return append([]int64(nil), ids...), nil
}

func groupTurnRecord(command ledger.SendGroupTurnCommand, messageID int64,
	targets []conversation.GroupInitialTarget) ledger.GroupTurnRecord {
	record := ledger.GroupTurnRecord{
		Message: conversation.Message{ID: messageID, ConversationID: command.Envelope.ConversationID,
			TurnID: command.Envelope.ID, AuthorKind: conversation.AuthorHuman,
			AuthorID: command.HumanID, Body: command.Body, CreatedAt: command.Envelope.CreatedAt},
		Envelope:       command.Envelope,
		Recipients:     append([]conversation.GroupRecipient{}, command.Envelope.Recipients...),
		InitialTargets: make([]ledger.InitialTargetRecord, 0, len(targets)),
	}
	for index, target := range targets {
		record.InitialTargets = append(record.InitialTargets, ledger.InitialTargetRecord{
			ID: command.TargetIDs[index], GroupInitialTarget: target,
			State: conversation.TargetQueued, CreatedAt: command.Envelope.CreatedAt,
		})
	}
	return record
}

func postgresGroupConcurrency(policy conversation.GroupConcurrencyPolicy) (string, error) {
	switch policy {
	case conversation.GroupSequential:
		return "serial", nil
	case conversation.GroupConcurrent:
		return "parallel", nil
	default:
		return "", fmt.Errorf("Group Turn concurrency policy is invalid")
	}
}

func domainGroupConcurrency(policy string) (conversation.GroupConcurrencyPolicy, error) {
	switch policy {
	case "serial":
		return conversation.GroupSequential, nil
	case "parallel":
		return conversation.GroupConcurrent, nil
	default:
		return "", fmt.Errorf("persisted Group Turn concurrency policy %q is invalid", policy)
	}
}

func domainTargetState(state string) (conversation.TargetState, error) {
	switch state {
	case "queued", "claimed", "lease_expired":
		return conversation.TargetQueued, nil
	case "working", "needs_you", "cancel_requested":
		return conversation.TargetWorking, nil
	case "succeeded":
		return conversation.TargetAnswered, nil
	case "failed":
		return conversation.TargetFailed, nil
	case "canceled":
		return conversation.TargetCanceled, nil
	default:
		return "", fmt.Errorf("persisted target state %q is invalid", state)
	}
}
