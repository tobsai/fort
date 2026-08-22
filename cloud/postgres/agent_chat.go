package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tobsai/fort/cloud/securebody"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

const (
	agentTurnSendScope          = "agent.turn.send"
	agentTargetRetryScope       = "agent.target.retry"
	agentTargetCancelScope      = "agent.target.cancel"
	executionSourceObserveScope = "execution_source.config.observe"
)

var _ ledger.AgentDirectChatRepository = (*Store)(nil)
var _ ledger.ExecutionSourceConfigObservationRepository = (*Store)(nil)

type postgresAgentDirectParent struct {
	agentState, conversationState, membershipRevisionID      string
	behaviorRevisionID, bindingRevisionID                    string
	executionSourceID, bindingSourceDigest, sourceDigest     string
	seatID, fortProfile, provider, requestedModel            string
	workerID                                                 string
	authorityID, authorityRevision, policyID, policyRevision string
	displayName, workerState                                 string
}

func (store *Store) ObserveExecutionSourceConfig(ctx context.Context, command ledger.ObserveExecutionSourceConfigCommand) (ledger.ExecutionSourceConfigObservation, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	var observation ledger.ExecutionSourceConfigObservation
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		resultID, claimed, err := claimAgentChatIdempotency(ctx, tx, accountID, executionSourceObserveScope,
			command.IdempotencyKey, digest, "execution_source_config_observation", command.ObservationID,
			command.ObservedAt)
		if err != nil {
			return err
		}
		if claimed {
			affected, err := tx.exec(ctx, `insert into fort_private.execution_source_config_observation (
  account_id,observation_id,execution_source_id,source_config_digest,observed_by,observed_at
) select $1,$2,source.execution_source_id,$4,$5,$6
from fort_private.execution_source as source
where source.account_id=$1 and source.execution_source_id=$3`, accountID, resultID,
				command.ExecutionSourceID, command.SourceConfigDigest, command.ObservedBy, command.ObservedAt.UTC())
			if err != nil {
				return fmt.Errorf("append Execution Source configuration observation: %w", err)
			}
			if affected != 1 {
				return fmt.Errorf("%w: Execution Source %q", ledger.ErrNotFound, command.ExecutionSourceID)
			}
		}
		observation, err = getPostgresSourceConfigObservation(ctx, tx, accountID, resultID)
		return err
	})
	return observation, err
}

func (store *Store) LatestExecutionSourceConfigObservation(ctx context.Context, accountID, executionSourceID string) (ledger.ExecutionSourceConfigObservation, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return ledger.ExecutionSourceConfigObservation{}, err
	}
	if strings.TrimSpace(executionSourceID) == "" || strings.TrimSpace(executionSourceID) != executionSourceID {
		return ledger.ExecutionSourceConfigObservation{}, fmt.Errorf("Execution Source id is required and must be canonical")
	}
	var observation ledger.ExecutionSourceConfigObservation
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		observation, err = latestPostgresSourceConfigObservation(ctx, tx, accountID, executionSourceID)
		return err
	})
	return observation, err
}

func appendPostgresSourceConfigObservation(ctx context.Context, tx transaction, observationID, accountID,
	executionSourceID, sourceConfigDigest, observedBy string, observedAt time.Time) error {
	affected, err := tx.exec(ctx, `insert into fort_private.execution_source_config_observation (
  account_id,observation_id,execution_source_id,source_config_digest,observed_by,observed_at
) values ($1,$2,$3,$4,$5,$6)`, accountID, observationID, executionSourceID, sourceConfigDigest,
		observedBy, observedAt.UTC())
	if err != nil {
		return fmt.Errorf("append Execution Source configuration observation: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("append Execution Source configuration observation affected %d rows", affected)
	}
	return nil
}

func getPostgresSourceConfigObservation(ctx context.Context, tx transaction, accountID,
	observationID string) (ledger.ExecutionSourceConfigObservation, error) {
	var observation ledger.ExecutionSourceConfigObservation
	err := tx.queryRow(ctx, `select observation_id,observation_sequence,account_id::text,
  execution_source_id,source_config_digest,observed_by,observed_at
from fort_private.execution_source_config_observation
where account_id=$1 and observation_id=$2`, accountID, observationID).scan(&observation.ID,
		&observation.Sequence, &observation.AccountID, &observation.ExecutionSourceID,
		&observation.SourceConfigDigest, &observation.ObservedBy, &observation.ObservedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ledger.ExecutionSourceConfigObservation{}, fmt.Errorf("%w: Execution Source configuration observation %q",
			ledger.ErrNotFound, observationID)
	}
	return observation, err
}

func latestPostgresSourceConfigObservation(ctx context.Context, tx transaction, accountID,
	executionSourceID string) (ledger.ExecutionSourceConfigObservation, error) {
	var observation ledger.ExecutionSourceConfigObservation
	err := tx.queryRow(ctx, `select observation_id,observation_sequence,account_id::text,
  execution_source_id,source_config_digest,observed_by,observed_at
from fort_private.execution_source_config_observation
where account_id=$1 and execution_source_id=$2
order by observation_sequence desc limit 1`, accountID, executionSourceID).scan(&observation.ID,
		&observation.Sequence, &observation.AccountID, &observation.ExecutionSourceID,
		&observation.SourceConfigDigest, &observation.ObservedBy, &observation.ObservedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ledger.ExecutionSourceConfigObservation{}, fmt.Errorf("%w: Execution Source configuration observation",
			ledger.ErrNotFound)
	}
	return observation, err
}

func (store *Store) SendAgentTurn(ctx context.Context, command ledger.SendAgentTurnCommand) (ledger.AgentTurnDispatch, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	var result ledger.AgentTurnDispatch
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		resultID, claimed, err := claimAgentChatIdempotency(ctx, tx, accountID, agentTurnSendScope,
			command.IdempotencyKey, digest, "conversation_turn", command.TurnID, command.CreatedAt)
		if err != nil {
			return err
		}
		if !claimed {
			result, err = getPostgresAgentTurnDispatch(ctx, tx, cipher, accountID, command.AgentID,
				command.ConversationID, resultID)
			return err
		}
		parent, err := loadPostgresAgentDirectParent(ctx, tx, accountID, command.AgentID, command.ConversationID)
		if err != nil {
			return err
		}
		if parent.agentState != string(conversation.AgentOpen) || parent.conversationState != string(conversation.ConversationOpen) {
			return fmt.Errorf("%w: Agent and Conversation must be open", ledger.ErrStateConflict)
		}
		if parent.bindingSourceDigest != parent.sourceDigest {
			return fmt.Errorf("%w: Agent Binding source configuration drift", ledger.ErrSourceDrift)
		}
		if parent.workerState != "enrolled" {
			return fmt.Errorf("%w: accepted Agent source is unavailable", ledger.ErrStateConflict)
		}
		participantID, err := ensurePostgresAgentDirectParticipant(ctx, tx, accountID, command, parent)
		if err != nil {
			return err
		}
		encrypted, err := cipher.seal(securebody.Scope{AccountID: accountID,
			RecordType: "conversation_message", RecordID: command.TurnID}, command.Body)
		if err != nil {
			return err
		}
		var messageID int64
		if err := tx.queryRow(ctx, `insert into fort_private.conversation_message (
  account_id, conversation_id, turn_id, message_kind, author_kind, author_id,
  body_ciphertext, body_key_id, body_nonce, body_digest, body_plaintext_length, created_at
) values ($1,$2,$3,'human','human',$4,$5,$6,$7,$8,$9,$10)
returning message_id`, accountID, command.ConversationID, command.TurnID, command.HumanID,
			encrypted.Ciphertext, encrypted.KeyID, encrypted.Nonce, encrypted.Digest, encrypted.PlaintextBytes,
			command.CreatedAt.UTC()).scan(&messageID); err != nil {
			return fmt.Errorf("insert Agent prompt message: %w", err)
		}
		messageIDs, err := loadPostgresAgentContextMessageIDs(ctx, tx, accountID, command.ConversationID, messageID)
		if err != nil {
			return err
		}
		manifestDigest, err := postgresAgentManifestDigest(command.ConversationID, messageID, messageIDs)
		if err != nil {
			return err
		}
		if _, err := tx.exec(ctx, `insert into fort_private.context_manifest (
  account_id, context_manifest_id, purpose, manifest_digest, created_by, created_at
) values ($1,$2,'turn',$3,$4,$5)`, accountID, command.ContextManifestID, manifestDigest,
			command.CreatedBy, command.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Agent Context Manifest: %w", err)
		}
		for ordinal, id := range messageIDs {
			if _, err := tx.exec(ctx, `insert into fort_private.context_manifest_message (
  account_id, context_manifest_id, ordinal, message_id
) values ($1,$2,$3,$4)`, accountID, command.ContextManifestID, ordinal, id); err != nil {
				return fmt.Errorf("insert Agent Context Manifest message: %w", err)
			}
		}
		contextRecordIDs := make([]string, 0, len(messageIDs))
		for _, contextMessageID := range messageIDs {
			contextRecordIDs = append(contextRecordIDs, fmt.Sprintf("message:%d", contextMessageID))
		}
		grant, err := json.Marshal(conversation.AuthorityGrant{
			ID: command.DelegationGrantID, Permissions: []string{}, ContextRecordIDs: contextRecordIDs,
		})
		if err != nil {
			return fmt.Errorf("encode Agent Delegation Grant: %w", err)
		}
		grantDigest := sha256.Sum256(grant)
		if _, err := tx.exec(ctx, `insert into fort_private.delegation_grant (
  account_id, delegation_grant_id, source_kind, source_id, authority_grant,
  grant_digest, maximum_agent_messages, maximum_handoff_depth, hard_deadline, created_by, created_at
) values ($1,$2,'human_turn',$3,$4::jsonb,$5,10,3,$6,$7,$8)`, accountID,
			command.DelegationGrantID, command.TurnID, string(grant), hex.EncodeToString(grantDigest[:]),
			command.HardDeadline.UTC(), command.CreatedBy, command.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Agent Delegation Grant: %w", err)
		}
		cancellationPolicy := `{"kind":"human_or_deadline","revision":"1"}`
		approvalPolicy := `{"kind":"explicit","revision":"1"}`
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_turn (
  account_id, turn_id, conversation_id, client_turn_id, idempotency_key, command_digest, kind,
  prompt_message_id, through_message_id, membership_revision_id, context_manifest_id, delegation_grant_id,
  concurrency_policy, cancellation_policy, approval_policy, maximum_agent_messages, maximum_handoff_depth,
  cost_limit_classification, token_limit_classification, hard_deadline, state, created_at, updated_at
) values ($1,$2,$3,$4,$5,$6,'human_direct',$7,$7,$8,$9,$10,'serial',$11::jsonb,$12::jsonb,
  10,3,'unknown','unknown',$13,'open',$14,$14)`, accountID, command.TurnID, command.ConversationID,
			command.ClientTurnID, command.IdempotencyKey, digest, messageID, parent.membershipRevisionID,
			command.ContextManifestID, command.DelegationGrantID, cancellationPolicy, approvalPolicy,
			command.HardDeadline.UTC(), command.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Agent Turn: %w", err)
		}
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_target (
  account_id, target_id, turn_id, conversation_id, agent_id, membership_revision_id,
  target_kind, origin_id, run_id, state, attempt_count, hard_deadline, cancellation_policy, created_at, updated_at
) values ($1,$2,$3,$4,$5,$6,'initial',$3,$7,'queued',0,$8,$9::jsonb,$10,$10)`, accountID,
			command.TargetID, command.TurnID, command.ConversationID, command.AgentID,
			parent.membershipRevisionID, command.RunID, command.HardDeadline.UTC(), cancellationPolicy,
			command.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Agent Target: %w", err)
		}
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_target_binding (
  account_id, target_id, conversation_id, agent_id, behavior_revision_id,
  binding_revision_id, participant_id, membership_revision_id, pinned_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, accountID, command.TargetID, command.ConversationID,
			command.AgentID, parent.behaviorRevisionID, parent.bindingRevisionID, participantID,
			parent.membershipRevisionID, command.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Agent Target Binding: %w", err)
		}
		metadata, _ := json.Marshal(map[string]string{"conversation_id": command.ConversationID,
			"run_id": command.RunID, "behavior_revision_id": parent.behaviorRevisionID,
			"binding_revision_id": parent.bindingRevisionID})
		if _, err := tx.exec(ctx, `insert into fort_private.ledger_event (
  account_id, aggregate_kind, aggregate_id, event_type, turn_id, target_id, event_metadata, created_at
) values ($1,'conversation',$2,'agent.turn.queued',$3,$4,$5::jsonb,$6)`, accountID,
			command.ConversationID, command.TurnID, command.TargetID, string(metadata), command.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("append Agent Turn event: %w", err)
		}
		result = ledger.AgentTurnDispatch{
			Message: ledger.AgentConversationMessage{ID: messageID, ConversationID: command.ConversationID,
				TurnID: command.TurnID, AuthorKind: conversation.AuthorHuman, AuthorID: command.HumanID,
				Body: command.Body, CreatedAt: command.CreatedAt.UTC()},
			Turn: ledger.AgentConversationTurn{ID: command.TurnID, ConversationID: command.ConversationID,
				ClientTurnID: command.ClientTurnID, PromptMessageID: messageID, ThroughMessageID: messageID,
				MembershipRevisionID: parent.membershipRevisionID, ContextManifestID: command.ContextManifestID,
				State: "open", CreatedAt: command.CreatedAt.UTC()},
			Context: ledger.AgentContextManifest{ID: command.ContextManifestID, ConversationID: command.ConversationID,
				ThroughMessageID: messageID, MessageIDs: messageIDs, Digest: manifestDigest, CreatedAt: command.CreatedAt.UTC()},
			Target: ledger.AgentConversationTarget{ID: command.TargetID, TurnID: command.TurnID,
				ConversationID: command.ConversationID, AgentID: command.AgentID,
				BehaviorRevisionID: parent.behaviorRevisionID, BindingRevisionID: parent.bindingRevisionID,
				ParticipantID: participantID, RunID: command.RunID, State: "queued",
				CreatedAt: command.CreatedAt.UTC(), UpdatedAt: command.CreatedAt.UTC()},
			Created: true,
		}
		return nil
	})
	return result, err
}

func loadPostgresAgentDirectParent(ctx context.Context, tx transaction, accountID, agentID, conversationID string) (postgresAgentDirectParent, error) {
	var parent postgresAgentDirectParent
	err := tx.queryRow(ctx, `select agent.state, item.state, item.current_membership_revision_id,
  agent.current_behavior_revision_id, agent.current_binding_revision_id,
  binding.execution_source_id, binding.source_config_digest, coalesce(observed.source_config_digest,''),
  binding.seat_id, binding.fort_profile, binding.provider, binding.requested_model, binding.worker_id,
  binding.authority_id, binding.authority_revision, binding.policy_id, binding.policy_revision,
  profile.name, worker.state
from fort_private.stable_agent as agent
join fort_private.agent_conversation as relation
  on relation.account_id=agent.account_id and relation.agent_id=agent.agent_id
join fort_private.conversation as item
  on item.account_id=relation.account_id and item.conversation_id=relation.conversation_id
join fort_private.conversation_member_revision as member
  on member.account_id=item.account_id and member.conversation_id=item.conversation_id
 and member.membership_revision_id=item.current_membership_revision_id and member.agent_id=agent.agent_id
join fort_private.agent_binding_revision as binding
  on binding.account_id=agent.account_id and binding.agent_id=agent.agent_id
 and binding.binding_revision_id=agent.current_binding_revision_id
join fort_private.execution_source as source
  on source.account_id=binding.account_id and source.execution_source_id=binding.execution_source_id
left join lateral (
  select observation.source_config_digest
  from fort_private.execution_source_config_observation as observation
  where observation.account_id=binding.account_id
    and observation.execution_source_id=binding.execution_source_id
  order by observation.observation_sequence desc
  limit 1
) as observed on true
join fort_private.worker as worker
  on worker.account_id=binding.account_id and worker.worker_id=binding.worker_id
join fort_private.agent_profile_revision as profile
  on profile.account_id=agent.account_id and profile.agent_id=agent.agent_id
 and profile.profile_revision_id=agent.current_profile_revision_id
where agent.account_id=$1 and agent.agent_id=$2 and relation.conversation_id=$3
for update of agent,item`, accountID, agentID, conversationID).scan(
		&parent.agentState, &parent.conversationState, &parent.membershipRevisionID,
		&parent.behaviorRevisionID, &parent.bindingRevisionID, &parent.executionSourceID,
		&parent.bindingSourceDigest, &parent.sourceDigest, &parent.seatID, &parent.fortProfile,
		&parent.provider, &parent.requestedModel, &parent.workerID, &parent.authorityID,
		&parent.authorityRevision, &parent.policyID, &parent.policyRevision, &parent.displayName,
		&parent.workerState)
	if errors.Is(err, pgx.ErrNoRows) {
		return postgresAgentDirectParent{}, fmt.Errorf("%w: Agent Conversation %q", ledger.ErrNotFound, conversationID)
	}
	return parent, err
}

func ensurePostgresAgentDirectParticipant(ctx context.Context, tx transaction, accountID string, command ledger.SendAgentTurnCommand, parent postgresAgentDirectParent) (string, error) {
	var participantID string
	err := tx.queryRow(ctx, `select participant_id from fort_private.conversation_participant
where account_id=$1 and conversation_id=$2 and agent_id=$3
  and behavior_revision_id=$4 and binding_revision_id=$5`, accountID, command.ConversationID,
		command.AgentID, parent.behaviorRevisionID, parent.bindingRevisionID).scan(&participantID)
	if err == nil {
		return participantID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	participantID = postgresDirectParticipantID(accountID, command.AgentID, command.ConversationID, parent.bindingRevisionID)
	participant := conversation.Participant{ID: participantID, ConversationID: command.ConversationID,
		SeatID: parent.seatID, Profile: parent.fortProfile, Agent: parent.provider,
		Model: parent.requestedModel, Machine: parent.workerID, DisplayName: parent.displayName,
		Position: 0, State: conversation.ParticipantActive, CreatedAt: command.CreatedAt.UTC()}
	binding := conversation.AgentBindingRevision{AuthorityID: parent.authorityID,
		AuthorityRevision: parent.authorityRevision, PolicyID: parent.policyID, PolicyRevision: parent.policyRevision}
	seat, authority, snapshotDigest, err := participantEvidence(ledger.CreateAgentCommand{Binding: binding, Participant: participant})
	if err != nil {
		return "", err
	}
	if _, err := tx.exec(ctx, `insert into fort_private.conversation_participant (
  account_id, participant_id, conversation_id, agent_id, behavior_revision_id, binding_revision_id,
  seat_snapshot, authority_snapshot, snapshot_digest, created_at
) values ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9,$10)`, accountID, participantID,
		command.ConversationID, command.AgentID, parent.behaviorRevisionID, parent.bindingRevisionID,
		seat, authority, snapshotDigest, command.CreatedAt.UTC()); err != nil {
		return "", fmt.Errorf("insert Agent participant evidence: %w", err)
	}
	return participantID, nil
}

func loadPostgresAgentContextMessageIDs(ctx context.Context, tx transaction, accountID, conversationID string, through int64) ([]int64, error) {
	rows, err := tx.query(ctx, `select message_id from fort_private.conversation_message
where account_id=$1 and conversation_id=$2 and message_id <= $3 order by message_id limit 257`,
		accountID, conversationID, through)
	if err != nil {
		return nil, err
	}
	defer rows.close()
	ids := make([]int64, 0)
	for rows.next() {
		var id int64
		if err := rows.scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.errResult(); err != nil {
		return nil, err
	}
	if len(ids) > 256 {
		return nil, fmt.Errorf("Agent direct context exceeds 256 committed messages")
	}
	if len(ids) == 0 || ids[len(ids)-1] != through {
		return nil, fmt.Errorf("Agent direct prompt is absent from frozen context")
	}
	return ids, nil
}

func claimAgentChatIdempotency(ctx context.Context, tx transaction, accountID, scope, key, digest, resultKind, proposedResultID string, createdAt time.Time) (string, bool, error) {
	affected, err := tx.exec(ctx, `insert into fort_private.idempotency_record (
  account_id, scope, idempotency_key, command_digest, result_kind, result_id, response_digest, created_at
) values ($1,$2,$3,$4,$5,$6,$4,$7)
on conflict (account_id,scope,idempotency_key) do nothing`, accountID, scope, key, digest,
		resultKind, proposedResultID, createdAt.UTC())
	if err != nil {
		return "", false, err
	}
	if affected == 1 {
		return proposedResultID, true, nil
	}
	if affected != 0 {
		return "", false, fmt.Errorf("reserve Agent chat idempotency affected %d rows", affected)
	}
	var existingDigest, existingKind, existingID string
	if err := tx.queryRow(ctx, `select command_digest,result_kind,result_id
from fort_private.idempotency_record where account_id=$1 and scope=$2 and idempotency_key=$3`,
		accountID, scope, key).scan(&existingDigest, &existingKind, &existingID); err != nil {
		return "", false, err
	}
	if existingDigest != digest || existingKind != resultKind {
		return "", false, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, key)
	}
	return existingID, false, nil
}

func postgresAgentManifestDigest(conversationID string, through int64, messageIDs []int64) (string, error) {
	payload, err := json.Marshal(struct {
		Version          int     `json:"version"`
		ConversationID   string  `json:"conversation_id"`
		ThroughMessageID int64   `json:"through_message_id"`
		MessageIDs       []int64 `json:"message_ids"`
	}{1, conversationID, through, messageIDs})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func postgresDirectParticipantID(accountID, agentID, conversationID, bindingID string) string {
	digest := sha256.Sum256([]byte(accountID + "\x00" + agentID + "\x00" + conversationID + "\x00" + bindingID))
	return "participant:agent:" + hex.EncodeToString(digest[:])
}

func getPostgresAgentTurnDispatch(ctx context.Context, tx transaction, cipher collaborationBodyCipher, accountID, agentID, conversationID, turnID string) (ledger.AgentTurnDispatch, error) {
	var result ledger.AgentTurnDispatch
	var body collaborationEncryptedBody
	err := tx.queryRow(ctx, `select message.message_id,message.conversation_id,coalesce(message.turn_id,''),
  coalesce(message.target_id,''),message.author_kind,message.author_id,coalesce(message.author_agent_id,''),
  message.body_ciphertext,message.body_key_id,message.body_nonce,message.body_digest,message.body_plaintext_length,message.created_at,
  turn.turn_id,turn.conversation_id,turn.client_turn_id,turn.prompt_message_id,turn.through_message_id,
  turn.membership_revision_id,turn.context_manifest_id,turn.state,turn.created_at,
  manifest.manifest_digest,manifest.created_at,
  target.target_id,target.turn_id,target.conversation_id,target.agent_id,pin.behavior_revision_id,
  pin.binding_revision_id,pin.participant_id,target.run_id,target.state,target.attempt_count,target.created_at,target.updated_at
from fort_private.conversation_turn as turn
join fort_private.conversation_message as message
  on message.account_id=turn.account_id and message.message_id=turn.prompt_message_id
join fort_private.context_manifest as manifest
  on manifest.account_id=turn.account_id and manifest.context_manifest_id=turn.context_manifest_id
join fort_private.conversation_target as target
  on target.account_id=turn.account_id and target.turn_id=turn.turn_id and target.target_kind='initial'
join fort_private.conversation_target_binding as pin
  on pin.account_id=target.account_id and pin.target_id=target.target_id
where turn.account_id=$1 and turn.conversation_id=$2 and target.agent_id=$3 and turn.turn_id=$4`,
		accountID, conversationID, agentID, turnID).scan(&result.Message.ID, &result.Message.ConversationID,
		&result.Message.TurnID, &result.Message.TargetID, &result.Message.AuthorKind, &result.Message.AuthorID,
		&result.Message.AuthorAgentID, &body.Ciphertext, &body.KeyID, &body.Nonce, &body.Digest,
		&body.PlaintextBytes, &result.Message.CreatedAt, &result.Turn.ID, &result.Turn.ConversationID,
		&result.Turn.ClientTurnID, &result.Turn.PromptMessageID, &result.Turn.ThroughMessageID,
		&result.Turn.MembershipRevisionID, &result.Turn.ContextManifestID, &result.Turn.State,
		&result.Turn.CreatedAt, &result.Context.Digest, &result.Context.CreatedAt, &result.Target.ID,
		&result.Target.TurnID, &result.Target.ConversationID, &result.Target.AgentID,
		&result.Target.BehaviorRevisionID, &result.Target.BindingRevisionID, &result.Target.ParticipantID,
		&result.Target.RunID, &result.Target.State, &result.Target.AttemptCount, &result.Target.CreatedAt,
		&result.Target.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ledger.AgentTurnDispatch{}, fmt.Errorf("%w: Agent Turn %q", ledger.ErrNotFound, turnID)
	}
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	result.Context.ID, result.Context.ConversationID = result.Turn.ContextManifestID, conversationID
	result.Context.ThroughMessageID = result.Turn.ThroughMessageID
	bodyText, err := cipher.open(securebody.Scope{AccountID: accountID, RecordType: "conversation_message",
		RecordID: result.Message.TurnID}, body)
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	result.Message.Body = bodyText
	result.Context.MessageIDs, err = loadPostgresManifestMessageIDs(ctx, tx, accountID, result.Context.ID)
	return result, err
}

func loadPostgresManifestMessageIDs(ctx context.Context, tx transaction, accountID, manifestID string) ([]int64, error) {
	rows, err := tx.query(ctx, `select message_id from fort_private.context_manifest_message
where account_id=$1 and context_manifest_id=$2 order by ordinal`, accountID, manifestID)
	if err != nil {
		return nil, err
	}
	defer rows.close()
	ids := make([]int64, 0)
	for rows.next() {
		var id int64
		if err := rows.scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.errResult()
}

// ReadAgentConversation returns the durable client projection and decrypts
// message bodies only after the full account -> Agent -> Conversation chain is
// established inside the account-scoped transaction.
func (store *Store) ReadAgentConversation(ctx context.Context, accountID, agentID, conversationID string) (ledger.AgentConversationProjection, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return ledger.AgentConversationProjection{}, err
	}
	if strings.TrimSpace(agentID) == "" || strings.TrimSpace(conversationID) == "" {
		return ledger.AgentConversationProjection{}, fmt.Errorf("Agent Conversation parent chain is required")
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return ledger.AgentConversationProjection{}, err
	}
	projection := ledger.AgentConversationProjection{Messages: make([]ledger.AgentConversationMessage, 0),
		Turns: make([]ledger.AgentConversationTurn, 0), Targets: make([]ledger.AgentConversationTarget, 0)}
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		projection.Conversation, err = getPostgresAgentConversation(ctx, tx, accountID, agentID, conversationID)
		if err != nil {
			return err
		}
		messageRows, err := tx.query(ctx, `select message_id,conversation_id,coalesce(turn_id,''),coalesce(target_id,''),
  coalesce(handoff_id,''),coalesce(routine_run_id,''),message_kind,
  author_kind,author_id,coalesce(author_agent_id,''),body_ciphertext,body_key_id,body_nonce,
  body_digest,body_plaintext_length,created_at
from fort_private.conversation_message where account_id=$1 and conversation_id=$2 order by message_id`, accountID, conversationID)
		if err != nil {
			return err
		}
		for messageRows.next() {
			var message ledger.AgentConversationMessage
			var handoffID, routineRunID, messageKind string
			var body collaborationEncryptedBody
			if err := messageRows.scan(&message.ID, &message.ConversationID, &message.TurnID, &message.TargetID,
				&handoffID, &routineRunID, &messageKind, &message.AuthorKind, &message.AuthorID,
				&message.AuthorAgentID, &body.Ciphertext, &body.KeyID,
				&body.Nonce, &body.Digest, &body.PlaintextBytes, &message.CreatedAt); err != nil {
				messageRows.close()
				return err
			}
			recordType, recordID, err := agentConversationMessageEncryptionScope(messageKind, message,
				handoffID, routineRunID)
			if err != nil {
				messageRows.close()
				return err
			}
			message.Body, err = cipher.open(securebody.Scope{AccountID: accountID,
				RecordType: recordType, RecordID: recordID}, body)
			if err != nil {
				messageRows.close()
				return err
			}
			projection.Messages = append(projection.Messages, message)
		}
		if err := messageRows.errResult(); err != nil {
			messageRows.close()
			return err
		}
		messageRows.close()
		turnRows, err := tx.query(ctx, `select turn_id,conversation_id,client_turn_id,prompt_message_id,
  through_message_id,membership_revision_id,context_manifest_id,state,created_at
from fort_private.conversation_turn where account_id=$1 and conversation_id=$2 and kind='human_direct'
order by prompt_message_id,turn_id`, accountID, conversationID)
		if err != nil {
			return err
		}
		for turnRows.next() {
			var turn ledger.AgentConversationTurn
			if err := turnRows.scan(&turn.ID, &turn.ConversationID, &turn.ClientTurnID, &turn.PromptMessageID,
				&turn.ThroughMessageID, &turn.MembershipRevisionID, &turn.ContextManifestID, &turn.State,
				&turn.CreatedAt); err != nil {
				turnRows.close()
				return err
			}
			projection.Turns = append(projection.Turns, turn)
		}
		if err := turnRows.errResult(); err != nil {
			turnRows.close()
			return err
		}
		turnRows.close()
		targetRows, err := tx.query(ctx, `select target.target_id,target.turn_id,target.conversation_id,target.agent_id,
  pin.behavior_revision_id,pin.binding_revision_id,pin.participant_id,target.run_id,target.state,
  target.attempt_count,target.created_at,target.updated_at
from fort_private.conversation_target as target
join fort_private.conversation_target_binding as pin
  on pin.account_id=target.account_id and pin.target_id=target.target_id
join fort_private.conversation_turn as turn
  on turn.account_id=target.account_id and turn.turn_id=target.turn_id
where target.account_id=$1 and target.conversation_id=$2 and target.agent_id=$3 and turn.kind='human_direct'
order by turn.prompt_message_id,target.target_id`, accountID, conversationID, agentID)
		if err != nil {
			return err
		}
		for targetRows.next() {
			var target ledger.AgentConversationTarget
			if err := targetRows.scan(&target.ID, &target.TurnID, &target.ConversationID, &target.AgentID,
				&target.BehaviorRevisionID, &target.BindingRevisionID, &target.ParticipantID, &target.RunID,
				&target.State, &target.AttemptCount, &target.CreatedAt, &target.UpdatedAt); err != nil {
				targetRows.close()
				return err
			}
			projection.Targets = append(projection.Targets, target)
		}
		if err := targetRows.errResult(); err != nil {
			targetRows.close()
			return err
		}
		targetRows.close()
		return nil
	})
	return projection, err
}

func agentConversationMessageEncryptionScope(messageKind string, message ledger.AgentConversationMessage,
	handoffID, routineRunID string) (string, string, error) {
	switch messageKind {
	case "human", "system":
		if message.TurnID != "" {
			return "conversation_message", message.TurnID, nil
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
	return "", "", fmt.Errorf("Agent Conversation message %d has unsupported or incomplete encryption scope", message.ID)
}

type postgresAgentTargetMutation struct {
	target        ledger.AgentConversationTarget
	agentState    string
	itemState     string
	bindingDigest string
	sourceDigest  string
	workerState   string
}

// RetryAgentTarget requeues the same durable Target. Its Target Binding row is
// immutable, so a retry after an Agent rebind still uses the original exact
// Behavior, Binding, participant, run, and source rather than current identity.
func (store *Store) RetryAgentTarget(ctx context.Context, command ledger.RetryAgentTargetCommand) (ledger.AgentConversationTarget, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	var target ledger.AgentConversationTarget
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		_, claimed, err := claimAgentChatIdempotency(ctx, tx, accountID, agentTargetRetryScope,
			command.IdempotencyKey, digest, "conversation_target", command.TargetID, command.RetriedAt)
		if err != nil {
			return err
		}
		current, err := loadPostgresAgentTargetMutation(ctx, tx, accountID, command.AgentID,
			command.ConversationID, command.TargetID)
		if err != nil {
			return err
		}
		if !claimed {
			target = current.target
			return nil
		}
		if current.agentState != string(conversation.AgentOpen) || current.itemState != string(conversation.ConversationOpen) {
			return fmt.Errorf("%w: Agent and Conversation must be open", ledger.ErrStateConflict)
		}
		if current.bindingDigest != current.sourceDigest {
			return fmt.Errorf("%w: original Agent Target source configuration drift", ledger.ErrSourceDrift)
		}
		if current.workerState != "enrolled" {
			return fmt.Errorf("%w: original Agent Target source is unavailable", ledger.ErrStateConflict)
		}
		switch current.target.State {
		case "failed", "canceled", "needs_you", "lease_expired":
		default:
			return fmt.Errorf("%w: only terminal Agent Targets can be retried", ledger.ErrStateConflict)
		}
		affected, err := tx.exec(ctx, `update fort_private.conversation_target
set state='queued', error_code=null, error_ciphertext=null, error_key_id=null,
    error_nonce=null, error_digest=null, updated_at=$1
where account_id=$2 and target_id=$3 and state in ('failed','canceled','needs_you','lease_expired')`,
			command.RetriedAt.UTC(), accountID, command.TargetID)
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("%w: Agent Target retry race", ledger.ErrStateConflict)
		}
		metadata, _ := json.Marshal(map[string]string{"conversation_id": command.ConversationID,
			"retried_by": command.RetriedBy, "behavior_revision_id": current.target.BehaviorRevisionID,
			"binding_revision_id": current.target.BindingRevisionID})
		if _, err := tx.exec(ctx, `insert into fort_private.ledger_event (
  account_id,aggregate_kind,aggregate_id,event_type,turn_id,target_id,event_metadata,created_at
) values ($1,'target',$2,'agent.target.retried',$3,$2,$4::jsonb,$5)`, accountID,
			command.TargetID, current.target.TurnID, string(metadata), command.RetriedAt.UTC()); err != nil {
			return err
		}
		target = current.target
		target.State = "queued"
		target.UpdatedAt = command.RetriedAt.UTC()
		return nil
	})
	return target, err
}

func loadPostgresAgentTargetMutation(ctx context.Context, tx transaction, accountID, agentID, conversationID, targetID string) (postgresAgentTargetMutation, error) {
	var row postgresAgentTargetMutation
	err := tx.queryRow(ctx, `select target.target_id,target.turn_id,target.conversation_id,target.agent_id,
  pin.behavior_revision_id,pin.binding_revision_id,pin.participant_id,target.run_id,target.state,
  target.attempt_count,target.created_at,target.updated_at,agent.state,item.state,
  binding.source_config_digest,coalesce(observed.source_config_digest,''),worker.state
from fort_private.conversation_target as target
join fort_private.conversation_target_binding as pin
  on pin.account_id=target.account_id and pin.target_id=target.target_id
join fort_private.stable_agent as agent
  on agent.account_id=target.account_id and agent.agent_id=target.agent_id
join fort_private.agent_conversation as relation
  on relation.account_id=agent.account_id and relation.agent_id=agent.agent_id
 and relation.conversation_id=target.conversation_id
join fort_private.conversation as item
  on item.account_id=relation.account_id and item.conversation_id=relation.conversation_id
join fort_private.agent_binding_revision as binding
  on binding.account_id=pin.account_id and binding.agent_id=pin.agent_id
 and binding.binding_revision_id=pin.binding_revision_id
join fort_private.execution_source as source
  on source.account_id=binding.account_id and source.execution_source_id=binding.execution_source_id
left join lateral (
  select observation.source_config_digest
  from fort_private.execution_source_config_observation as observation
  where observation.account_id=binding.account_id
    and observation.execution_source_id=binding.execution_source_id
  order by observation.observation_sequence desc
  limit 1
) as observed on true
join fort_private.worker as worker
  on worker.account_id=binding.account_id and worker.worker_id=binding.worker_id
where target.account_id=$1 and target.agent_id=$2 and target.conversation_id=$3 and target.target_id=$4
for update of target`, accountID, agentID, conversationID, targetID).scan(&row.target.ID, &row.target.TurnID,
		&row.target.ConversationID, &row.target.AgentID, &row.target.BehaviorRevisionID,
		&row.target.BindingRevisionID, &row.target.ParticipantID, &row.target.RunID, &row.target.State,
		&row.target.AttemptCount, &row.target.CreatedAt, &row.target.UpdatedAt, &row.agentState,
		&row.itemState, &row.bindingDigest, &row.sourceDigest, &row.workerState)
	if errors.Is(err, pgx.ErrNoRows) {
		return postgresAgentTargetMutation{}, fmt.Errorf("%w: Agent Target %q", ledger.ErrNotFound, targetID)
	}
	return row, err
}

func (store *Store) CancelAgentTarget(ctx context.Context, command ledger.CancelAgentTargetCommand) (ledger.AgentConversationTarget, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	var target ledger.AgentConversationTarget
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		_, claimed, err := claimAgentChatIdempotency(ctx, tx, accountID, agentTargetCancelScope,
			command.IdempotencyKey, digest, "conversation_target", command.TargetID, command.CanceledAt)
		if err != nil {
			return err
		}
		current, err := loadPostgresAgentTargetMutation(ctx, tx, accountID, command.AgentID,
			command.ConversationID, command.TargetID)
		if err != nil {
			return err
		}
		if !claimed {
			target = current.target
			return nil
		}
		nextState := ""
		switch current.target.State {
		case "queued", "lease_expired":
			nextState = "canceled"
		case "claimed", "working":
			nextState = "cancel_requested"
		default:
			return fmt.Errorf("%w: Agent Target cannot be canceled from %s", ledger.ErrStateConflict,
				current.target.State)
		}
		affected, err := tx.exec(ctx, `update fort_private.conversation_target set state=$1,updated_at=$2
where account_id=$3 and target_id=$4 and state=$5`, nextState, command.CanceledAt.UTC(), accountID,
			command.TargetID, current.target.State)
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("%w: Agent Target cancellation race", ledger.ErrStateConflict)
		}
		metadata, _ := json.Marshal(map[string]string{"conversation_id": command.ConversationID,
			"canceled_by": command.CanceledBy, "state": nextState,
			"behavior_revision_id": current.target.BehaviorRevisionID,
			"binding_revision_id":  current.target.BindingRevisionID})
		eventType := "agent.target.cancel_requested"
		if nextState == "canceled" {
			eventType = "agent.target.canceled"
		}
		if _, err := tx.exec(ctx, `insert into fort_private.ledger_event (
  account_id,aggregate_kind,aggregate_id,event_type,turn_id,target_id,event_metadata,created_at
) values ($1,'target',$2,$3,$4,$2,$5::jsonb,$6)`, accountID,
			command.TargetID, eventType, current.target.TurnID, string(metadata), command.CanceledAt.UTC()); err != nil {
			return err
		}
		target = current.target
		target.State, target.UpdatedAt = nextState, command.CanceledAt.UTC()
		return nil
	})
	return target, err
}
