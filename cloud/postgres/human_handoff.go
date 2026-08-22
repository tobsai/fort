package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

const handoffCancelScope = "handoff.cancel"

func (store *Store) CreateHumanHandoff(ctx context.Context, command ledger.CreateHumanHandoffCommand) (ledger.HandoffRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	command.AccountID = accountID
	digest, err := command.Digest()
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	var recipient ledger.AgentRecord
	var groupTurn *ledger.GroupTurnRecord
	var groupRecipient *conversation.GroupRecipient
	var causal humanHandoffCausalEvidence
	var replayed *ledger.HandoffRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		var existingDigest, resultKind, resultID string
		err := tx.queryRow(ctx, `select command_digest,result_kind,result_id
from fort_private.idempotency_record
where account_id=$1 and scope=$2 and idempotency_key=$3`, accountID, handoffAcceptScope,
			command.IdempotencyKey).scan(&existingDigest, &resultKind, &resultID)
		if err == nil {
			if existingDigest != digest || resultKind != "handoff" {
				return fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.IdempotencyKey)
			}
			record, err := getPostgresHandoff(ctx, tx, cipher, accountID, resultID)
			if err != nil {
				return err
			}
			replayed = &record
			return nil
		}
		if !isNoRows(err) {
			return err
		}
		if err := command.Validate(); err != nil {
			return err
		}
		recipient, err = getAgentRecord(ctx, tx, accountID, command.RecipientAgentID)
		if err != nil {
			return err
		}
		sourceMessageID, err := canonicalPostgresMessageID(command.SourceMessageID)
		if err != nil {
			return err
		}
		source, derived, err := loadPostgresHumanHandoffSource(ctx, tx, cipher, accountID,
			command.SourceConversationID, sourceMessageID)
		if err != nil {
			return err
		}
		causal = derived
		groupTurnID := causal.groupTurnID
		if groupTurnID == "" && source.turnID != "" {
			var candidate string
			err := tx.queryRow(ctx, `select turn_id from fort_private.conversation_turn
where account_id=$1 and turn_id=$2 and kind='human_group'`, accountID, source.turnID).scan(&candidate)
			switch {
			case err == nil:
				groupTurnID = candidate
			case isNoRows(err):
			default:
				return err
			}
		}
		if groupTurnID != "" {
			loaded, err := getPostgresGroupTurn(ctx, tx, cipher, accountID, groupTurnID)
			if err != nil {
				return err
			}
			groupTurn = &loaded
			group, err := getPostgresGroup(ctx, tx, accountID, loaded.Envelope.GroupID)
			if err != nil {
				return err
			}
			position := -1
			for _, member := range group.Membership.Members {
				if member.AgentID == command.RecipientAgentID {
					position = member.Position
					break
				}
			}
			if position < 0 {
				return fmt.Errorf("%w: recipient Agent is not a current Group member", ledger.ErrNotFound)
			}
			current := conversation.GroupRecipient{
				AgentID: command.RecipientAgentID, BehaviorRevisionID: recipient.Behavior.ID,
				BindingRevisionID: recipient.Binding.ID,
				ParticipantID: postgresGroupParticipantID(accountID, loaded.Envelope.GroupID,
					command.RecipientAgentID, recipient.Binding.ID),
			}
			if err := ensurePostgresCurrentGroupParticipant(ctx, tx, accountID, loaded.Envelope.GroupID,
				loaded.Envelope.ConversationID, position, current, command.CreatedAt); err != nil {
				return err
			}
			groupRecipient = &current
		}
		return nil
	})
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	if replayed != nil {
		return *replayed, nil
	}
	if recipient.Agent.State != conversation.AgentOpen {
		return ledger.HandoffRecord{}, fmt.Errorf("%w: recipient Agent is not open", ledger.ErrStateConflict)
	}
	manifest := conversation.ContextManifest{References: make([]conversation.ContextReference, 0, len(command.ContextMessageIDs))}
	contextIDs := make([]string, 0, len(command.ContextMessageIDs))
	for _, messageID := range command.ContextMessageIDs {
		reference := conversation.ContextReference{Kind: conversation.ContextMessage, ID: messageID,
			AccountID: accountID, Immutable: true}
		manifest.References = append(manifest.References, reference)
		contextIDs = append(contextIDs, reference.Key())
	}
	root := conversation.AuthorityGrant{ID: command.RootDelegationGrantID,
		Permissions: []string{}, ContextRecordIDs: contextIDs}
	recipientBehaviorID := recipient.Behavior.ID
	recipientBindingID := recipient.Binding.ID
	participantID := recipient.Participant.ID
	outputConversationID := recipient.Home.ID
	policyID, policyRevision := recipient.Binding.PolicyID, recipient.Binding.PolicyRevision
	maxMessages, maxDepth := conversation.MaxGroupAgentMessages, conversation.MaxGroupHandoffDepth
	deadline := command.HardDeadline
	groupTurnID := ""
	requireCurrentRecipient := true
	inheritedRoot := false
	if causal.rootDelegationGrant != nil {
		if causal.deadline == nil || !command.HardDeadline.Equal(*causal.deadline) {
			return ledger.HandoffRecord{}, fmt.Errorf("%w: nested Handoff must preserve its parent deadline", ledger.ErrStateConflict)
		}
		root = cloneAuthorityGrant(*causal.rootDelegationGrant)
		maxMessages, maxDepth = causal.maxAgentMessages, causal.maxDepth
		deadline = *causal.deadline
		groupTurnID = causal.groupTurnID
		inheritedRoot = true
	}
	if groupTurn != nil {
		if !command.HardDeadline.Equal(groupTurn.Envelope.Deadline) {
			return ledger.HandoffRecord{}, fmt.Errorf("%w: Group Handoff must preserve its Group Turn deadline", ledger.ErrStateConflict)
		}
		if groupRecipient == nil {
			return ledger.HandoffRecord{}, fmt.Errorf("%w: recipient Agent is not a current Group member", ledger.ErrNotFound)
		}
		root = cloneAuthorityGrant(groupTurn.Envelope.RootDelegationGrant)
		recipientBehaviorID = groupRecipient.BehaviorRevisionID
		recipientBindingID = groupRecipient.BindingRevisionID
		participantID = groupRecipient.ParticipantID
		outputConversationID = groupTurn.Envelope.ConversationID
		maxMessages, maxDepth = groupTurn.Envelope.MaxAgentMessages, groupTurn.Envelope.MaxHandoffDepth
		deadline = groupTurn.Envelope.Deadline
		groupTurnID = groupTurn.Envelope.ID
		inheritedRoot = true
	} else if recipient.Home.State != conversation.ConversationOpen ||
		recipient.Agent.CurrentBehaviorRevisionID != recipient.Behavior.ID ||
		recipient.Agent.CurrentBindingRevisionID != recipient.Binding.ID ||
		recipient.Binding.BehaviorRevisionID != recipient.Behavior.ID ||
		recipient.Participant.ConversationID != recipient.Home.ID {
		return ledger.HandoffRecord{}, fmt.Errorf("%w: recipient Agent is not open with exact current Home evidence", ledger.ErrStateConflict)
	}
	if inheritedRoot {
		allowed := make(map[string]struct{}, len(root.ContextRecordIDs))
		for _, contextID := range root.ContextRecordIDs {
			allowed[contextID] = struct{}{}
		}
		for _, contextID := range contextIDs {
			if _, ok := allowed[contextID]; !ok {
				return ledger.HandoffRecord{}, fmt.Errorf("Handoff context reference %q is not authorized by its root grant", contextID)
			}
		}
	}
	handoffPolicy := conversation.AuthorityGrant{ID: "policy:handoff:human:1", Permissions: []string{}}
	recipientPolicy := conversation.AuthorityGrant{
		ID:          "policy:binding:" + policyID + ":" + policyRevision + ":" + recipientBindingID,
		Permissions: []string{},
	}
	authorityLayers := []conversation.AuthorityGrant{root}
	if causal.parentStageAuthority != nil {
		authorityLayers = append(authorityLayers, *causal.parentStageAuthority)
	}
	authorityLayers = append(authorityLayers, handoffPolicy, recipientPolicy)
	effective, err := conversation.ComputeEffectiveAuthority([]string{}, authorityLayers...)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	handoff := conversation.Handoff{
		ID: command.HandoffID, AccountID: accountID, IdempotencyKey: command.IdempotencyKey,
		State: conversation.HandoffQueued, CreatedByKind: conversation.HandoffActorHuman, CreatedByID: command.CreatedByID,
		SourceMessageID: command.SourceMessageID, SourceAgentID: causal.sourceAgentID,
		SourceBehaviorRevisionID: causal.sourceBehaviorID, SourceBindingRevisionID: causal.sourceBindingID,
		RecipientAgentID:            command.RecipientAgentID,
		RecipientBehaviorRevisionID: recipientBehaviorID, RecipientBindingRevisionID: recipientBindingID,
		SourceConversationID: command.SourceConversationID, OutputConversationID: outputConversationID,
		Context: manifest, RequestedResult: command.RequestedResult, ReplyToMessageID: command.ReplyToMessageID,
		RootDelegationGrant: root, ParentStageAuthority: causal.parentStageAuthority,
		HandoffPolicy: handoffPolicy, RecipientBindingPolicy: recipientPolicy,
		RequestedAuthority: []string{}, EffectiveAuthority: effective, BudgetClass: conversation.LimitUnknown,
		MaxAgentMessages: maxMessages, MaxDepth: maxDepth, GroupTurnID: groupTurnID,
		Depth: causal.depth, Deadline: deadline,
		AncestorAgentIDs: append([]string{}, causal.ancestorAgentIDs...), ParentHandoffID: causal.parentHandoffID,
		CreatedAt: command.CreatedAt,
	}
	projections := []string{}
	if handoff.SourceConversationID != handoff.OutputConversationID {
		projections = append(projections, handoff.SourceConversationID)
	}
	return store.AcceptHandoff(ctx, ledger.AcceptHandoffCommand{
		Handoff: handoff, TargetID: command.TargetID, ParticipantID: participantID,
		ProjectionConversationIDs: projections, RequireCurrentRecipient: requireCurrentRecipient,
	})
}

func (store *Store) CancelHandoff(ctx context.Context, command ledger.CancelHandoffCommand) (ledger.HandoffRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.HandoffRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	var record ledger.HandoffRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, handoffCancelScope,
			command.IdempotencyKey, digest, "handoff", command.HandoffID, command.CanceledAt)
		if err != nil {
			return err
		}
		record, err = getPostgresHandoff(ctx, tx, cipher, accountID, command.HandoffID)
		if err != nil || !claimed {
			return err
		}
		if record.Handoff.State != conversation.HandoffQueued && record.Handoff.State != conversation.HandoffNeedsYou &&
			record.Handoff.State != conversation.HandoffWorking {
			return fmt.Errorf("%w: Handoff cannot be canceled from %s", ledger.ErrStateConflict, record.Handoff.State)
		}
		var targetState string
		if err := tx.queryRow(ctx, `select target.state
from fort_private.handoff as handoff
join fort_private.conversation_target as target
  on target.account_id=handoff.account_id and target.target_id=handoff.target_id
join fort_private.conversation_target_binding as binding
  on binding.account_id=target.account_id and binding.target_id=target.target_id
where handoff.account_id=$1 and handoff.handoff_id=$2 and target.target_id=$3
  and binding.agent_id=$4 and binding.behavior_revision_id=$5
  and binding.binding_revision_id=$6 and binding.participant_id=$7
for update of handoff,target`, accountID, command.HandoffID, record.Target.ID, record.Target.AgentID,
			record.Target.BehaviorRevisionID, record.Target.BindingRevisionID, record.Target.ParticipantID).scan(&targetState); err != nil {
			return fmt.Errorf("lock exact Handoff cancellation target: %w", err)
		}
		cancellationState := ledger.HandoffCancellationRequested
		nextTargetState := "cancel_requested"
		switch targetState {
		case "queued", "lease_expired", "needs_you":
			nextTargetState = "canceled"
			cancellationState = ledger.HandoffCancellationCanceled
		case "claimed", "working":
			if affected, err := tx.exec(ctx, `update fort_private.execution_attempt
set state='cancel_requested',updated_at=$3
where account_id=$1 and target_id=$2 and state in ('leased','working')`, accountID,
				record.Target.ID, command.CanceledAt.UTC()); err != nil {
				return err
			} else if affected != 1 {
				return fmt.Errorf("%w: exact working Handoff attempt was not cancellable", ledger.ErrStateConflict)
			}
		default:
			return fmt.Errorf("%w: Handoff target cannot be canceled from %s", ledger.ErrStateConflict, targetState)
		}
		if affected, err := tx.exec(ctx, `update fort_private.conversation_target
set state=$3,updated_at=$4 where account_id=$1 and target_id=$2 and state=$5`, accountID,
			record.Target.ID, nextTargetState, command.CanceledAt.UTC(), targetState); err != nil || affected != 1 {
			return changedRowsError("cancel exact Handoff target", affected, err)
		}
		if affected, err := tx.exec(ctx, `update fort_private.handoff
set state='canceled',canceled_at=$3,terminal_at=$3,updated_at=$3
where account_id=$1 and handoff_id=$2 and state=$4`, accountID, command.HandoffID,
			command.CanceledAt.UTC(), postgresHandoffState(record.Handoff.State)); err != nil || affected != 1 {
			return changedRowsError("cancel Handoff", affected, err)
		}
		if _, err := tx.exec(ctx, `update fort_private.conversation_turn
set state='canceled',updated_at=$3 where account_id=$1 and turn_id=$2 and state in ('open','needs_you')`,
			accountID, handoffTurnID(command.HandoffID), command.CanceledAt.UTC()); err != nil {
			return err
		}
		metadata, err := json.Marshal(map[string]string{
			"canceled_by": command.CanceledBy, "cancellation_state": string(cancellationState),
			"agent_id": record.Target.AgentID, "behavior_revision_id": record.Target.BehaviorRevisionID,
			"binding_revision_id": record.Target.BindingRevisionID, "participant_id": record.Target.ParticipantID,
		})
		if err != nil {
			return err
		}
		eventType := "handoff.cancel_requested"
		if cancellationState == ledger.HandoffCancellationCanceled {
			eventType = "handoff.canceled"
		}
		if _, err := tx.exec(ctx, `insert into fort_private.ledger_event (
  account_id,aggregate_kind,aggregate_id,event_type,turn_id,target_id,event_metadata,created_at
) values ($1,'handoff',$2,$3,$4,$5,$6::jsonb,$7)`, accountID, command.HandoffID,
			eventType, handoffTurnID(command.HandoffID), record.Target.ID, string(metadata), command.CanceledAt.UTC()); err != nil {
			return err
		}
		record, err = getPostgresHandoff(ctx, tx, cipher, accountID, command.HandoffID)
		return err
	})
	return record, err
}

func postgresHandoffState(state conversation.HandoffState) string {
	switch state {
	case conversation.HandoffNeedsYou:
		return "needs_you"
	case conversation.HandoffWorking:
		return "working"
	default:
		return "queued"
	}
}
