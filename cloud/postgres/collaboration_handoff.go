package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tobsai/fort/cloud/securebody"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

const handoffAcceptScope = "handoff.accept"

type collaborationHandoffMeta struct {
	Version           int                  `json:"version"`
	Handoff           conversation.Handoff `json:"handoff"`
	ParticipantID     string               `json:"participant_id"`
	TargetTurnID      string               `json:"target_turn_id"`
	ContextManifestID string               `json:"context_manifest_id"`
}

func (store *Store) AcceptHandoff(ctx context.Context, command ledger.AcceptHandoffCommand) (ledger.HandoffRecord, error) {
	accountID, err := store.operationAccount(command.Handoff.AccountID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	command.Handoff.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.HandoffRecord{}, err
	}
	if command.Handoff.CreatedByKind == conversation.HandoffActorAgent &&
		command.Handoff.CreatedByID != command.Handoff.SourceAgentID {
		return ledger.HandoffRecord{}, fmt.Errorf("Agent-initiated Handoff creation actor must be its source Agent")
	}
	if command.Handoff.CreatedByKind == conversation.HandoffActorHuman &&
		(command.Handoff.StructuredEmitterID != "" || command.Handoff.EmitterRequest != nil) {
		return ledger.HandoffRecord{}, fmt.Errorf("human-initiated Handoff cannot claim structured emitter evidence")
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
		claimed, acceptedID, err := claimHandoffAcceptance(ctx, tx, accountID,
			command.Handoff.IdempotencyKey, digest, command.Handoff.ID, command.Handoff.CreatedAt)
		if err != nil {
			return err
		}
		if !claimed {
			record, err = getPostgresHandoff(ctx, tx, cipher, accountID, acceptedID)
			return err
		}
		evidence, err := validatePostgresHandoffEvidence(ctx, tx, cipher, command)
		if err != nil {
			return err
		}
		requested, err := cipher.seal(securebody.Scope{AccountID: accountID,
			RecordType: "handoff_requested_result", RecordID: command.Handoff.ID},
			command.Handoff.RequestedResult)
		if err != nil {
			return fmt.Errorf("encrypt Handoff requested result: %w", err)
		}
		contextID := handoffContextManifestID(command.Handoff.ID)
		contextDigest, err := evidenceDigest(command.Handoff.Context)
		if err != nil {
			return err
		}
		if _, err := tx.exec(ctx, `insert into fort_private.context_manifest (
  account_id, context_manifest_id, purpose, manifest_digest, created_by, created_at
) values ($1,$2,'handoff',$3,$4,$5)`, accountID, contextID, contextDigest,
			command.Handoff.CreatedByID, command.Handoff.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Handoff context manifest: %w", err)
		}
		for index, reference := range command.Handoff.Context.References {
			switch reference.Kind {
			case conversation.ContextMessage:
				messageID, err := canonicalPostgresMessageID(reference.ID)
				if err != nil {
					return err
				}
				if _, err := tx.exec(ctx, `insert into fort_private.context_manifest_message (
  account_id, context_manifest_id, ordinal, message_id
) values ($1,$2,$3,$4)`, accountID, contextID, index, messageID); err != nil {
					return fmt.Errorf("insert Handoff context message: %w", err)
				}
			case conversation.ContextArtifact, conversation.ContextOutputArtifact:
				if _, err := tx.exec(ctx, `insert into fort_private.context_manifest_artifact (
  account_id, context_manifest_id, ordinal, artifact_id
) values ($1,$2,$3,$4)`, accountID, contextID, index, reference.ID); err != nil {
					return fmt.Errorf("insert Handoff context artifact: %w", err)
				}
			}
		}
		if err := ensurePostgresDelegationGrant(ctx, tx, command.Handoff); err != nil {
			return err
		}
		if err := persistPostgresHandoffApproval(ctx, tx, cipher, command.Handoff); err != nil {
			return err
		}
		sourceExecutionAttemptID, err := validatePostgresEmitterReceipt(
			ctx, tx, command.Handoff, digest, evidence.sourceMessageID,
		)
		if err != nil {
			return err
		}
		redacted := command.Handoff
		redacted.RequestedResult = ""
		meta := collaborationHandoffMeta{Version: 1, Handoff: redacted,
			ParticipantID: command.ParticipantID, TargetTurnID: handoffTurnID(command.Handoff.ID),
			ContextManifestID: contextID}
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		effectiveJSON, err := json.Marshal(command.Handoff.EffectiveAuthority)
		if err != nil {
			return err
		}
		effectiveDigest, err := evidenceDigest(command.Handoff.EffectiveAuthority)
		if err != nil {
			return err
		}
		cancellationJSON, _ := json.Marshal(collaborationPolicySnapshot{ID: "handoff", Revision: "1"})
		approvalJSON, _ := json.Marshal(collaborationPolicySnapshot{ID: "handoff-approval", Revision: "1"})
		turnID := handoffTurnID(command.Handoff.ID)
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_turn (
  account_id, turn_id, conversation_id, client_turn_id, idempotency_key,
  command_digest, kind, prompt_message_id, through_message_id,
  membership_revision_id, context_manifest_id, delegation_grant_id,
  concurrency_policy, cancellation_policy, approval_policy,
  maximum_agent_messages, maximum_handoff_depth,
  cost_limit_classification, token_limit_classification,
  cost_limit, token_limit, hard_deadline, state, created_at, updated_at
) values ($1,$2,$3,$4,$5,$6,'handoff',$7,$7,$8,$9,$10,'serial',$11::jsonb,$12::jsonb,
  $13,$14,$15,'unknown',null,null,$16,'open',$17,$17)`, accountID, turnID,
			command.Handoff.OutputConversationID, command.Handoff.ID, command.Handoff.IdempotencyKey,
			digest, evidence.sourceMessageID, evidence.outputMembershipID, contextID,
			command.Handoff.RootDelegationGrant.ID, string(cancellationJSON), string(approvalJSON),
			command.Handoff.MaxAgentMessages, command.Handoff.MaxDepth, command.Handoff.BudgetClass,
			command.Handoff.Deadline.UTC(), command.Handoff.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Handoff Turn: %w", err)
		}
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_target (
  account_id, target_id, turn_id, conversation_id, agent_id,
  membership_revision_id, target_kind, origin_id, run_id, state,
  attempt_count, hard_deadline, cancellation_policy, created_at, updated_at
) values ($1,$2,$3,$4,$5,$6,'handoff',$7,$2,'queued',0,$8,$9::jsonb,$10,$10)`,
			accountID, command.TargetID, turnID, command.Handoff.OutputConversationID,
			command.Handoff.RecipientAgentID, evidence.outputMembershipID, command.Handoff.ID,
			command.Handoff.Deadline.UTC(), string(cancellationJSON), command.Handoff.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Handoff target: %w", err)
		}
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_target_binding (
  account_id, target_id, conversation_id, agent_id, behavior_revision_id,
  binding_revision_id, participant_id, membership_revision_id, pinned_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, accountID, command.TargetID,
			command.Handoff.OutputConversationID, command.Handoff.RecipientAgentID,
			command.Handoff.RecipientBehaviorRevisionID, command.Handoff.RecipientBindingRevisionID,
			command.ParticipantID, evidence.outputMembershipID, command.Handoff.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Handoff target binding: %w", err)
		}
		if _, err := tx.exec(ctx, `insert into fort_private.handoff (
  account_id, handoff_id, idempotency_key, command_digest, state,
  creation_actor_kind, creation_actor_id, emitter_receipt_id,
  source_execution_attempt_id, source_turn_id, source_message_id,
  source_agent_id, source_behavior_revision_id,
  source_binding_revision_id, recipient_agent_id,
  recipient_behavior_revision_id, recipient_binding_revision_id,
  source_conversation_id, output_conversation_id, context_manifest_id,
  requested_result_ciphertext, requested_result_key_id, requested_result_nonce,
  requested_result_digest, delegation_grant_id, handoff_policy,
  effective_authority, effective_authority_digest, approval_required,
  approval_receipt_id, budget_classification, parent_handoff_id,
  group_turn_id, depth, hard_deadline, target_id, created_at, updated_at
) values ($1,$2,$3,$4,'queued',$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,
  $20,$21,$22,$23,$24,$25::jsonb,$26::jsonb,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36,$36)`,
			accountID, command.Handoff.ID, command.Handoff.IdempotencyKey, digest,
			command.Handoff.CreatedByKind, command.Handoff.CreatedByID,
			nilIfBlank(command.Handoff.StructuredEmitterID), nilIfBlank(sourceExecutionAttemptID),
			nilIfBlank(evidence.sourceTurnID), evidence.sourceMessageID,
			nilIfBlank(command.Handoff.SourceAgentID),
			nilIfBlank(command.Handoff.SourceBehaviorRevisionID), nilIfBlank(command.Handoff.SourceBindingRevisionID),
			command.Handoff.RecipientAgentID, command.Handoff.RecipientBehaviorRevisionID,
			command.Handoff.RecipientBindingRevisionID, command.Handoff.SourceConversationID,
			command.Handoff.OutputConversationID, contextID, requested.Ciphertext, requested.KeyID,
			requested.Nonce, requested.Digest, command.Handoff.RootDelegationGrant.ID,
			string(metaJSON), string(effectiveJSON), effectiveDigest, command.Handoff.ApprovalRequired,
			handoffApprovalReceiptID(command.Handoff), command.Handoff.BudgetClass,
			nilIfBlank(command.Handoff.ParentHandoffID), nilIfBlank(command.Handoff.GroupTurnID),
			command.Handoff.Depth, command.Handoff.Deadline.UTC(), command.TargetID,
			command.Handoff.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Handoff: %w", err)
		}
		projectionIDs := append([]string{}, command.ProjectionConversationIDs...)
		sort.Strings(projectionIDs)
		for _, conversationID := range projectionIDs {
			kind := "source"
			if command.Handoff.GroupTurnID != "" {
				kind = "group"
			}
			if _, err := tx.exec(ctx, `insert into fort_private.handoff_projection (
  account_id, handoff_id, conversation_id, projection_kind, projected_at
) values ($1,$2,$3,$4,$5)`, accountID, command.Handoff.ID, conversationID,
				kind, command.Handoff.CreatedAt.UTC()); err != nil {
				return fmt.Errorf("insert Handoff projection: %w", err)
			}
		}
		record = acceptedHandoffRecord(command)
		return nil
	})
	return record, err
}

func claimHandoffAcceptance(ctx context.Context, tx transaction, accountID, key, digest, proposedID string,
	createdAt time.Time) (bool, string, error) {
	affected, err := tx.exec(ctx, `insert into fort_private.idempotency_record (
  account_id,scope,idempotency_key,command_digest,result_kind,result_id,response_digest,created_at
) values ($1,$2,$3,$4,'handoff',$5,$4,$6)
on conflict (account_id,scope,idempotency_key) do nothing`, accountID, handoffAcceptScope, key,
		digest, proposedID, createdAt.UTC())
	if err != nil {
		return false, "", err
	}
	if affected == 1 {
		return true, proposedID, nil
	}
	if affected != 0 {
		return false, "", fmt.Errorf("reserve Handoff idempotency key affected %d rows", affected)
	}
	var existingDigest, resultKind, resultID string
	if err := tx.queryRow(ctx, `select command_digest,result_kind,result_id
from fort_private.idempotency_record
where account_id=$1 and scope=$2 and idempotency_key=$3`, accountID, handoffAcceptScope, key).scan(
		&existingDigest, &resultKind, &resultID); err != nil {
		return false, "", err
	}
	if existingDigest != digest || resultKind != "handoff" {
		return false, "", fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, key)
	}
	return false, resultID, nil
}

type postgresHandoffEvidence struct {
	sourceMessageID    int64
	sourceTurnID       string
	outputMembershipID string
}

func validatePostgresHandoffEvidence(ctx context.Context, tx transaction, cipher collaborationBodyCipher,
	command ledger.AcceptHandoffCommand) (postgresHandoffEvidence, error) {
	handoff := command.Handoff
	for _, conversationID := range []string{handoff.SourceConversationID, handoff.OutputConversationID} {
		var found string
		if err := tx.queryRow(ctx, `select conversation_id from fort_private.conversation
where account_id = $1 and conversation_id = $2`, handoff.AccountID, conversationID).scan(&found); err != nil {
			if isNoRows(err) {
				return postgresHandoffEvidence{}, fmt.Errorf("%w: Conversation %q", ledger.ErrNotFound, conversationID)
			}
			return postgresHandoffEvidence{}, err
		}
	}
	sourceMessageID, err := canonicalPostgresMessageID(handoff.SourceMessageID)
	if err != nil {
		return postgresHandoffEvidence{}, err
	}
	var messageConversationID, sourceTurnID string
	if err := tx.queryRow(ctx, `select conversation_id, coalesce(turn_id, '')
from fort_private.conversation_message
where account_id = $1 and message_id = $2`, handoff.AccountID, sourceMessageID).scan(
		&messageConversationID, &sourceTurnID); err != nil {
		if isNoRows(err) {
			return postgresHandoffEvidence{}, fmt.Errorf("%w: source message %q", ledger.ErrNotFound, handoff.SourceMessageID)
		}
		return postgresHandoffEvidence{}, err
	}
	if messageConversationID != handoff.SourceConversationID {
		return postgresHandoffEvidence{}, fmt.Errorf("Handoff source message does not belong to its persisted source Conversation")
	}
	var outputMembershipID string
	if err := tx.queryRow(ctx, `select conversation.current_membership_revision_id
from fort_private.conversation_participant as participant
join fort_private.conversation as conversation
  on conversation.account_id = participant.account_id
 and conversation.conversation_id = participant.conversation_id
where participant.account_id = $1 and participant.conversation_id = $2
  and participant.agent_id = $3 and participant.behavior_revision_id = $4
  and participant.binding_revision_id = $5 and participant.participant_id = $6`,
		handoff.AccountID, handoff.OutputConversationID, handoff.RecipientAgentID,
		handoff.RecipientBehaviorRevisionID, handoff.RecipientBindingRevisionID,
		command.ParticipantID).scan(&outputMembershipID); err != nil {
		if isNoRows(err) {
			return postgresHandoffEvidence{}, fmt.Errorf("Handoff recipient target does not match persisted Agent, revision, and participant evidence")
		}
		return postgresHandoffEvidence{}, err
	}
	if command.RequireCurrentRecipient {
		var state conversation.AgentState
		var currentBehaviorID, currentBindingID string
		if err := tx.queryRow(ctx, `select state,current_behavior_revision_id,current_binding_revision_id
from fort_private.stable_agent where account_id=$1 and agent_id=$2`, handoff.AccountID,
			handoff.RecipientAgentID).scan(&state, &currentBehaviorID, &currentBindingID); err != nil {
			if isNoRows(err) {
				return postgresHandoffEvidence{}, fmt.Errorf("%w: recipient Agent %q", ledger.ErrNotFound, handoff.RecipientAgentID)
			}
			return postgresHandoffEvidence{}, err
		}
		if state != conversation.AgentOpen || currentBehaviorID != handoff.RecipientBehaviorRevisionID ||
			currentBindingID != handoff.RecipientBindingRevisionID {
			return postgresHandoffEvidence{}, fmt.Errorf("%w: Handoff recipient revision evidence is no longer current", ledger.ErrRevisionConflict)
		}
	}
	if handoff.SourceAgentID != "" {
		var found string
		if err := tx.queryRow(ctx, `select agent.agent_id
from fort_private.stable_agent as agent
join fort_private.agent_behavior_revision as behavior
  on behavior.account_id = agent.account_id and behavior.agent_id = agent.agent_id
join fort_private.agent_binding_revision as binding
  on binding.account_id = agent.account_id and binding.agent_id = agent.agent_id
 and binding.behavior_revision_id = behavior.behavior_revision_id
where agent.account_id = $1 and agent.agent_id = $2
  and behavior.behavior_revision_id = $3 and binding.binding_revision_id = $4`,
			handoff.AccountID, handoff.SourceAgentID, handoff.SourceBehaviorRevisionID,
			handoff.SourceBindingRevisionID).scan(&found); err != nil {
			return postgresHandoffEvidence{}, fmt.Errorf("Handoff source Agent revision evidence was not found: %w", err)
		}
	}
	if handoff.GroupTurnID != "" {
		groupTurn, err := getPostgresGroupTurn(ctx, tx, cipher, handoff.AccountID, handoff.GroupTurnID)
		if err != nil {
			return postgresHandoffEvidence{}, err
		}
		if groupTurn.Envelope.ConversationID != handoff.SourceConversationID ||
			!reflect.DeepEqual(groupTurn.Envelope.RootDelegationGrant, handoff.RootDelegationGrant) ||
			groupTurn.Envelope.MaxAgentMessages != handoff.MaxAgentMessages ||
			groupTurn.Envelope.MaxHandoffDepth != handoff.MaxDepth ||
			!groupTurn.Envelope.Deadline.Equal(handoff.Deadline) {
			return postgresHandoffEvidence{}, fmt.Errorf("Handoff does not preserve its Group Turn delegation and limits")
		}
	}
	if handoff.ParentHandoffID != "" {
		parent, err := getPostgresHandoff(ctx, tx, cipher, handoff.AccountID, handoff.ParentHandoffID)
		if err != nil {
			return postgresHandoffEvidence{}, err
		}
		if parent.Handoff.State != conversation.HandoffCompleted || parent.Result == nil ||
			handoff.Depth != parent.Handoff.Depth+1 || handoff.GroupTurnID != parent.Handoff.GroupTurnID ||
			handoff.SourceAgentID != parent.Handoff.RecipientAgentID ||
			handoff.SourceBehaviorRevisionID != parent.Handoff.RecipientBehaviorRevisionID ||
			handoff.SourceBindingRevisionID != parent.Handoff.RecipientBindingRevisionID ||
			handoff.SourceMessageID != parent.Result.MessageID ||
			!reflect.DeepEqual(handoff.RootDelegationGrant, parent.Handoff.RootDelegationGrant) ||
			handoff.ParentStageAuthority == nil ||
			!reflect.DeepEqual(*handoff.ParentStageAuthority, parent.Handoff.EffectiveAuthority) {
			return postgresHandoffEvidence{}, fmt.Errorf("nested Handoff does not preserve its parent causal chain and authority")
		}
	}
	for _, reference := range handoff.Context.References {
		switch reference.Kind {
		case conversation.ContextMessage:
			messageID, err := canonicalPostgresMessageID(reference.ID)
			if err != nil {
				return postgresHandoffEvidence{}, err
			}
			var found int64
			if err := tx.queryRow(ctx, `select message_id from fort_private.conversation_message
where account_id = $1 and message_id = $2`, handoff.AccountID, messageID).scan(&found); err != nil {
				return postgresHandoffEvidence{}, fmt.Errorf("%w: context message %q", ledger.ErrNotFound, reference.ID)
			}
		case conversation.ContextArtifact, conversation.ContextOutputArtifact:
			var state, digest string
			var plaintextLength int64
			if err := tx.queryRow(ctx, `select state, logical_digest, expected_plaintext_length
from fort_private.artifact where account_id = $1 and artifact_id = $2`,
				handoff.AccountID, reference.ID).scan(&state, &digest, &plaintextLength); err != nil ||
				state != "finalized" || digest != reference.Digest || plaintextLength != reference.Size {
				return postgresHandoffEvidence{}, fmt.Errorf("Handoff artifact %q lacks exact finalized ledger evidence", reference.Key())
			}
		}
	}
	for _, conversationID := range command.ProjectionConversationIDs {
		var found string
		if err := tx.queryRow(ctx, `select conversation_id from fort_private.conversation
where account_id = $1 and conversation_id = $2`, handoff.AccountID, conversationID).scan(&found); err != nil {
			return postgresHandoffEvidence{}, fmt.Errorf("%w: projection Conversation %q", ledger.ErrNotFound, conversationID)
		}
	}
	return postgresHandoffEvidence{sourceMessageID: sourceMessageID,
		sourceTurnID: sourceTurnID, outputMembershipID: outputMembershipID}, nil
}

func ensurePostgresDelegationGrant(ctx context.Context, tx transaction, handoff conversation.Handoff) error {
	payload, err := json.Marshal(handoff.RootDelegationGrant)
	if err != nil {
		return err
	}
	digest, err := evidenceDigest(handoff.RootDelegationGrant)
	if err != nil {
		return err
	}
	sourceKind := "direct_handoff"
	sourceID := handoff.ID
	if handoff.GroupTurnID != "" {
		sourceKind = "human_turn"
		sourceID = handoff.GroupTurnID
	}
	affected, err := tx.exec(ctx, `insert into fort_private.delegation_grant (
  account_id, delegation_grant_id, source_kind, source_id, authority_grant,
  grant_digest, maximum_agent_messages, maximum_handoff_depth,
  hard_deadline, created_by, created_at
) values ($1,$2,$3,$4,$5::jsonb,$6,$7,$8,$9,$10,$11)
on conflict (account_id, delegation_grant_id) do nothing`, handoff.AccountID,
		handoff.RootDelegationGrant.ID, sourceKind, sourceID, string(payload), digest,
		handoff.MaxAgentMessages, handoff.MaxDepth, handoff.Deadline.UTC(),
		handoff.CreatedByID, handoff.CreatedAt.UTC())
	if err != nil {
		return err
	}
	if affected == 1 {
		return nil
	}
	var existingJSON, existingDigest string
	var maxMessages, maxDepth int
	var deadline time.Time
	if err := tx.queryRow(ctx, `select authority_grant::text, grant_digest,
  maximum_agent_messages, maximum_handoff_depth, hard_deadline
from fort_private.delegation_grant
where account_id = $1 and delegation_grant_id = $2`, handoff.AccountID,
		handoff.RootDelegationGrant.ID).scan(&existingJSON, &existingDigest, &maxMessages, &maxDepth, &deadline); err != nil {
		return err
	}
	if existingDigest != digest || maxMessages != handoff.MaxAgentMessages || maxDepth != handoff.MaxDepth ||
		!deadline.Equal(handoff.Deadline) {
		return fmt.Errorf("Handoff root delegation grant conflicts with persisted evidence")
	}
	var existing conversation.AuthorityGrant
	if err := json.Unmarshal([]byte(existingJSON), &existing); err != nil || !reflect.DeepEqual(existing, handoff.RootDelegationGrant) {
		return fmt.Errorf("Handoff root delegation grant conflicts with persisted evidence")
	}
	return nil
}

func persistPostgresHandoffApproval(ctx context.Context, tx transaction, cipher collaborationBodyCipher,
	handoff conversation.Handoff) error {
	if handoff.ApprovalReceipt == nil {
		return nil
	}
	payload, err := json.Marshal(handoff.ApprovalReceipt)
	if err != nil {
		return err
	}
	receipt, err := cipher.seal(securebody.Scope{AccountID: handoff.AccountID,
		RecordType: "handoff_approval_receipt", RecordID: handoff.ApprovalReceipt.ID}, string(payload))
	if err != nil {
		return err
	}
	_, err = tx.exec(ctx, `insert into fort_private.approval_receipt (
  account_id, approval_receipt_id, subject_kind, subject_id, decision,
  receipt_ciphertext, receipt_key_id, receipt_nonce,
  receipt_digest, decided_by, decided_at
) values ($1,$2,'handoff',$3,'approved',$4,$5,$6,$7,$8,$9)`,
		handoff.AccountID, handoff.ApprovalReceipt.ID, handoff.ID,
		receipt.Ciphertext, receipt.KeyID, receipt.Nonce, receipt.Digest,
		handoff.CreatedByID, handoff.CreatedAt.UTC())
	return err
}

func validatePostgresEmitterReceipt(ctx context.Context, tx transaction, handoff conversation.Handoff,
	commandDigest string, sourceMessageID int64) (string, error) {
	if handoff.CreatedByKind != conversation.HandoffActorAgent {
		return "", nil
	}
	var found, sourceExecutionAttemptID, observedDigest string
	if err := tx.queryRow(ctx, `select receipt.emitter_receipt_id,
  receipt.source_execution_attempt_id, receipt.structured_command_digest
from fort_private.handoff_emitter_receipt as receipt
join fort_private.execution_attempt as source_attempt
  on source_attempt.account_id = receipt.account_id
 and source_attempt.execution_attempt_id = receipt.source_execution_attempt_id
join fort_private.conversation_message as source_message
  on source_message.account_id = source_attempt.account_id
 and source_message.message_id = $6
 and source_message.target_id = source_attempt.target_id
 and source_message.author_kind = 'agent'
 and source_message.author_agent_id = receipt.source_agent_id
where receipt.account_id = $1 and receipt.emitter_receipt_id = $2
  and receipt.source_agent_id = $3 and receipt.source_behavior_revision_id = $4
  and receipt.source_binding_revision_id = $5`,
		handoff.AccountID, handoff.StructuredEmitterID, handoff.SourceAgentID,
		handoff.SourceBehaviorRevisionID, handoff.SourceBindingRevisionID, sourceMessageID).scan(
		&found, &sourceExecutionAttemptID, &observedDigest); err != nil {
		return "", fmt.Errorf("Agent-initiated Handoff lacks exact structured emitter receipt: %w", err)
	}
	if found != handoff.StructuredEmitterID || strings.TrimSpace(sourceExecutionAttemptID) == "" ||
		observedDigest != commandDigest {
		return "", fmt.Errorf("Agent-initiated Handoff structured emitter receipt does not match its exact command")
	}
	return sourceExecutionAttemptID, nil
}

func acceptedHandoffRecord(command ledger.AcceptHandoffCommand) ledger.HandoffRecord {
	projectionIDs := append([]string{}, command.ProjectionConversationIDs...)
	sort.Strings(projectionIDs)
	projections := make([]conversation.HandoffProjection, 0, len(projectionIDs))
	for _, conversationID := range projectionIDs {
		projections = append(projections, conversation.HandoffProjection{
			HandoffID: command.Handoff.ID, ConversationID: conversationID,
			OutputConversationID: command.Handoff.OutputConversationID,
			State:                command.Handoff.State,
		})
	}
	return ledger.HandoffRecord{Handoff: command.Handoff, Target: ledger.HandoffTargetRecord{
		ID: command.TargetID, HandoffID: command.Handoff.ID,
		ConversationID:     command.Handoff.OutputConversationID,
		AgentID:            command.Handoff.RecipientAgentID,
		BehaviorRevisionID: command.Handoff.RecipientBehaviorRevisionID,
		BindingRevisionID:  command.Handoff.RecipientBindingRevisionID,
		ParticipantID:      command.ParticipantID, State: conversation.TargetQueued,
		CreatedAt: command.Handoff.CreatedAt,
	}, Projections: projections}
}

func canonicalPostgresMessageID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != value {
		return 0, fmt.Errorf("Handoff message id %q is not a canonical Fort message id", value)
	}
	return id, nil
}

func handoffContextManifestID(handoffID string) string { return "context:handoff:" + handoffID }
func handoffTurnID(handoffID string) string            { return "turn:handoff:" + handoffID }

func handoffApprovalReceiptID(handoff conversation.Handoff) any {
	if handoff.ApprovalReceipt == nil {
		return nil
	}
	return handoff.ApprovalReceipt.ID
}

func nilIfBlank(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func (store *Store) GetHandoff(ctx context.Context, accountID, handoffID string) (ledger.HandoffRecord, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	if strings.TrimSpace(handoffID) == "" {
		return ledger.HandoffRecord{}, fmt.Errorf("Handoff id is required")
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	var record ledger.HandoffRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		var err error
		record, err = getPostgresHandoff(ctx, tx, cipher, accountID, handoffID)
		return err
	})
	return record, err
}

func (store *Store) ListHandoffs(ctx context.Context, accountID string) ([]ledger.HandoffRecord, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return nil, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return nil, err
	}
	records := make([]ledger.HandoffRecord, 0)
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		result, err := tx.query(ctx, `select handoff_id from fort_private.handoff
where account_id = $1 order by created_at, handoff_id`, accountID)
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
			record, err := getPostgresHandoff(ctx, tx, cipher, accountID, id)
			if err != nil {
				return err
			}
			records = append(records, record)
		}
		return nil
	})
	return records, err
}

func getPostgresHandoff(ctx context.Context, tx transaction, cipher collaborationBodyCipher,
	accountID, handoffID string) (ledger.HandoffRecord, error) {
	var record ledger.HandoffRecord
	var metaJSON, persistedState, effectiveJSON, targetState string
	var requested collaborationEncryptedBody
	requested.PlaintextBytes = -1
	err := tx.queryRow(ctx, `select handoff.handoff_policy::text, handoff.state,
  handoff.requested_result_ciphertext, handoff.requested_result_key_id,
  handoff.requested_result_nonce, handoff.requested_result_digest,
  handoff.effective_authority::text,
  target.target_id, target.conversation_id, target.agent_id,
  binding.behavior_revision_id, binding.binding_revision_id,
  binding.participant_id, target.state, target.created_at
from fort_private.handoff as handoff
join fort_private.conversation_target as target
  on target.account_id = handoff.account_id and target.target_id = handoff.target_id
join fort_private.conversation_target_binding as binding
  on binding.account_id = target.account_id and binding.target_id = target.target_id
where handoff.account_id = $1 and handoff.handoff_id = $2`, accountID, handoffID).scan(
		&metaJSON, &persistedState, &requested.Ciphertext, &requested.KeyID,
		&requested.Nonce, &requested.Digest, &effectiveJSON,
		&record.Target.ID, &record.Target.ConversationID, &record.Target.AgentID,
		&record.Target.BehaviorRevisionID, &record.Target.BindingRevisionID,
		&record.Target.ParticipantID, &targetState, &record.Target.CreatedAt)
	if isNoRows(err) {
		return ledger.HandoffRecord{}, fmt.Errorf("%w: Handoff %q", ledger.ErrNotFound, handoffID)
	}
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	var meta collaborationHandoffMeta
	if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil || meta.Version != 1 {
		return ledger.HandoffRecord{}, fmt.Errorf("decode Handoff metadata: %w", err)
	}
	record.Handoff = meta.Handoff
	if record.Handoff.ID != handoffID || record.Handoff.AccountID != accountID ||
		record.Target.ID == "" || record.Target.ConversationID != record.Handoff.OutputConversationID ||
		record.Target.AgentID != record.Handoff.RecipientAgentID ||
		record.Target.BehaviorRevisionID != record.Handoff.RecipientBehaviorRevisionID ||
		record.Target.BindingRevisionID != record.Handoff.RecipientBindingRevisionID ||
		record.Target.ParticipantID != meta.ParticipantID {
		return ledger.HandoffRecord{}, fmt.Errorf("persisted Handoff identity or target evidence conflicts with metadata")
	}
	record.Handoff.State, err = domainHandoffState(persistedState)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	var effective conversation.AuthorityGrant
	if err := json.Unmarshal([]byte(effectiveJSON), &effective); err != nil ||
		!reflect.DeepEqual(effective, record.Handoff.EffectiveAuthority) {
		return ledger.HandoffRecord{}, fmt.Errorf("persisted Handoff effective authority conflicts with metadata")
	}
	record.Handoff.RequestedResult, err = cipher.open(securebody.Scope{AccountID: accountID,
		RecordType: "handoff_requested_result", RecordID: handoffID}, requested)
	if err != nil {
		return ledger.HandoffRecord{}, fmt.Errorf("decrypt Handoff requested result: %w", err)
	}
	record.Target.HandoffID = handoffID
	record.Target.State, err = domainTargetState(targetState)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	if record.Handoff.State == conversation.HandoffCanceled {
		var eventType, metadataJSON string
		var requestedAt time.Time
		err := tx.queryRow(ctx, `select event_type,event_metadata::text,created_at
from fort_private.ledger_event
where account_id=$1 and aggregate_kind='handoff' and aggregate_id=$2
  and event_type in ('handoff.cancel_requested','handoff.canceled')
order by event_id desc limit 1`, accountID, handoffID).scan(&eventType, &metadataJSON, &requestedAt)
		if err == nil {
			var metadata map[string]string
			if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
				return ledger.HandoffRecord{}, fmt.Errorf("decode Handoff cancellation metadata: %w", err)
			}
			state := ledger.HandoffCancellationRequested
			if eventType == "handoff.canceled" {
				state = ledger.HandoffCancellationCanceled
			}
			cancellation := ledger.HandoffCancellationRecord{
				HandoffID: handoffID, TargetID: record.Target.ID, AgentID: metadata["agent_id"],
				BehaviorRevisionID: metadata["behavior_revision_id"], BindingRevisionID: metadata["binding_revision_id"],
				ParticipantID: metadata["participant_id"], State: state,
				RequestedBy: metadata["canceled_by"], RequestedAt: requestedAt,
			}
			if cancellation.AgentID != record.Target.AgentID ||
				cancellation.BehaviorRevisionID != record.Target.BehaviorRevisionID ||
				cancellation.BindingRevisionID != record.Target.BindingRevisionID ||
				cancellation.ParticipantID != record.Target.ParticipantID {
				return ledger.HandoffRecord{}, fmt.Errorf("persisted Handoff cancellation conflicts with target evidence")
			}
			record.Cancellation = &cancellation
		} else if !isNoRows(err) {
			return ledger.HandoffRecord{}, err
		}
	}
	var resultMessageID int64
	var resultBody collaborationEncryptedBody
	var resultConversation string
	err = tx.queryRow(ctx, `select message_id, conversation_id, body_ciphertext,
  body_key_id, body_nonce, body_digest, body_plaintext_length
from fort_private.conversation_message
where account_id = $1 and handoff_id = $2 and message_kind = 'handoff_result'`,
		accountID, handoffID).scan(&resultMessageID, &resultConversation,
		&resultBody.Ciphertext, &resultBody.KeyID, &resultBody.Nonce,
		&resultBody.Digest, &resultBody.PlaintextBytes)
	if err == nil {
		body, err := cipher.open(securebody.Scope{AccountID: accountID,
			RecordType: "handoff_result", RecordID: handoffID}, resultBody)
		if err != nil {
			return ledger.HandoffRecord{}, fmt.Errorf("decrypt Handoff result: %w", err)
		}
		record.Result = &conversation.HandoffResult{HandoffID: handoffID,
			OutputConversationID: resultConversation, MessageID: strconv.FormatInt(resultMessageID, 10), Body: body}
	} else if !isNoRows(err) {
		return ledger.HandoffRecord{}, err
	}
	record.Projections = make([]conversation.HandoffProjection, 0)
	projections, err := tx.query(ctx, `select conversation_id
from fort_private.handoff_projection
where account_id = $1 and handoff_id = $2
order by conversation_id`, accountID, handoffID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	defer projections.close()
	for projections.next() {
		var projection conversation.HandoffProjection
		if err := projections.scan(&projection.ConversationID); err != nil {
			return ledger.HandoffRecord{}, err
		}
		projection.HandoffID = handoffID
		projection.OutputConversationID = record.Handoff.OutputConversationID
		projection.State = record.Handoff.State
		if record.Result != nil {
			projection.AuthoritativeMessageID = record.Result.MessageID
		}
		record.Projections = append(record.Projections, projection)
	}
	if err := projections.errResult(); err != nil {
		return ledger.HandoffRecord{}, err
	}
	var attempt ledger.HandoffAttemptRecord
	var fence int64
	var attemptState string
	var receipt sql.NullString
	var completed sql.NullTime
	err = tx.queryRow(ctx, `select attempt.execution_attempt_id, mapping.handoff_id,
  lease.lease_id, worker.machine_id, lease.fence_token, attempt.state,
  attempt.started_at, lease.expires_at, attempt.terminal_receipt_id,
  attempt.terminal_at
from fort_private.handoff_attempt as mapping
join fort_private.execution_attempt as attempt
  on attempt.account_id = mapping.account_id
 and attempt.execution_attempt_id = mapping.execution_attempt_id
join fort_private.worker_lease as lease
  on lease.account_id = attempt.account_id
 and lease.execution_attempt_id = attempt.execution_attempt_id
join fort_private.worker as worker
  on worker.account_id = lease.account_id and worker.worker_id = lease.worker_id
where mapping.account_id = $1 and mapping.handoff_id = $2
order by mapping.attempt_number desc limit 1`, accountID, handoffID).scan(
		&attempt.ID, &attempt.HandoffID, &attempt.LeaseID, &attempt.MachineID,
		&fence, &attemptState, &attempt.StartedAt, &attempt.LeaseExpiresAt,
		&receipt, &completed)
	if err == nil {
		attempt.FenceToken = strconv.FormatInt(fence, 10)
		attempt.State, err = domainHandoffAttemptState(attemptState)
		if err != nil {
			return ledger.HandoffRecord{}, err
		}
		attempt.TerminalReceiptID = receipt.String
		if completed.Valid {
			attempt.CompletedAt = completed.Time
		}
		record.Attempt = &attempt
	} else if !isNoRows(err) {
		return ledger.HandoffRecord{}, err
	}
	if err := record.Handoff.Validate(); err != nil {
		return ledger.HandoffRecord{}, fmt.Errorf("invalid persisted Handoff: %w", err)
	}
	if record.Result != nil {
		if err := record.Result.ValidateFor(record.Handoff); err != nil {
			return ledger.HandoffRecord{}, fmt.Errorf("invalid persisted Handoff result: %w", err)
		}
	}
	return record, nil
}

func domainHandoffState(state string) (conversation.HandoffState, error) {
	switch state {
	case "queued":
		return conversation.HandoffQueued, nil
	case "needs_you", "needs_approval":
		return conversation.HandoffNeedsYou, nil
	case "working", "requested":
		return conversation.HandoffWorking, nil
	case "succeeded":
		return conversation.HandoffCompleted, nil
	case "failed":
		return conversation.HandoffFailed, nil
	case "canceled":
		return conversation.HandoffCanceled, nil
	default:
		return "", fmt.Errorf("persisted Handoff state %q is invalid", state)
	}
}

func domainHandoffAttemptState(state string) (ledger.HandoffAttemptState, error) {
	switch state {
	case "queued", "leased", "working", "needs_you", "cancel_requested":
		return ledger.HandoffAttemptWorking, nil
	case "succeeded":
		return ledger.HandoffAttemptCompleted, nil
	case "failed", "lease_expired":
		return ledger.HandoffAttemptFailed, nil
	case "canceled":
		return ledger.HandoffAttemptCanceled, nil
	default:
		return "", fmt.Errorf("persisted Handoff attempt state %q is invalid", state)
	}
}
