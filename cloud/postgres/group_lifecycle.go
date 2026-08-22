package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

const (
	groupRenameScope         = "group.rename"
	groupStateScope          = "group.state"
	groupMembersReplaceScope = "group.members.replace"
)

func (store *Store) RenameGroup(ctx context.Context, command ledger.RenameGroupCommand) (ledger.GroupRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.GroupRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	var record ledger.GroupRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, groupRenameScope,
			command.IdempotencyKey, digest, "group", command.GroupID, command.ChangedAt)
		if err != nil {
			return err
		}
		if !claimed {
			record, err = getPostgresGroup(ctx, tx, accountID, command.GroupID)
			return err
		}
		var conversationID, currentTitle string
		err = tx.queryRow(ctx, `select identity.conversation_id, conversation.title
from fort_private.group_conversation as identity
join fort_private.conversation as conversation
  on conversation.account_id = identity.account_id
 and conversation.conversation_id = identity.conversation_id
where identity.account_id = $1 and identity.group_id = $2
for update of conversation`, accountID, command.GroupID).scan(&conversationID, &currentTitle)
		if isNoRows(err) {
			return fmt.Errorf("%w: Group %q", ledger.ErrNotFound, command.GroupID)
		}
		if err != nil {
			return err
		}
		if currentTitle != command.ExpectedTitle {
			return fmt.Errorf("%w: Group title", ledger.ErrRevisionConflict)
		}
		active, err := postgresGroupHasActiveWork(ctx, tx, accountID, command.GroupID)
		if err != nil {
			return err
		}
		if active {
			return fmt.Errorf("%w: Group has an active Turn, target, or Handoff", ledger.ErrStateConflict)
		}
		affected, err := tx.exec(ctx, `update fort_private.conversation
set title = $1, updated_at = $2
where account_id = $3 and conversation_id = $4 and title = $5`, command.Title,
			command.ChangedAt.UTC(), accountID, conversationID, command.ExpectedTitle)
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("%w: Group title", ledger.ErrRevisionConflict)
		}
		if err := insertPostgresGroupLifecycleEvent(ctx, tx, accountID, command.GroupID, "group.renamed",
			map[string]any{"previous_title": command.ExpectedTitle, "title": command.Title, "changed_by": command.ChangedBy},
			command.ChangedAt); err != nil {
			return err
		}
		record, err = getPostgresGroup(ctx, tx, accountID, command.GroupID)
		return err
	})
	return record, err
}

func (store *Store) SetGroupState(ctx context.Context, command ledger.SetGroupStateCommand) (ledger.GroupRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.GroupRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	var record ledger.GroupRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, groupStateScope,
			command.IdempotencyKey, digest, "group", command.GroupID, command.ChangedAt)
		if err != nil {
			return err
		}
		if !claimed {
			record, err = getPostgresGroup(ctx, tx, accountID, command.GroupID)
			return err
		}
		var conversationID, currentState string
		err = tx.queryRow(ctx, `select identity.conversation_id, conversation.state
from fort_private.group_conversation as identity
join fort_private.conversation as conversation
  on conversation.account_id = identity.account_id
 and conversation.conversation_id = identity.conversation_id
where identity.account_id = $1 and identity.group_id = $2
for update of conversation`, accountID, command.GroupID).scan(&conversationID, &currentState)
		if isNoRows(err) {
			return fmt.Errorf("%w: Group %q", ledger.ErrNotFound, command.GroupID)
		}
		if err != nil {
			return err
		}
		if currentState != string(command.ExpectedState) {
			return fmt.Errorf("%w: Group state", ledger.ErrStateConflict)
		}
		active, err := postgresGroupHasActiveWork(ctx, tx, accountID, command.GroupID)
		if err != nil {
			return err
		}
		if active {
			return fmt.Errorf("%w: Group has an active Turn, target, or Handoff", ledger.ErrStateConflict)
		}
		affected, err := tx.exec(ctx, `update fort_private.conversation
set state = $1, updated_at = $2
where account_id = $3 and conversation_id = $4 and state = $5`, command.State,
			command.ChangedAt.UTC(), accountID, conversationID, command.ExpectedState)
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("%w: Group state", ledger.ErrStateConflict)
		}
		if err := insertPostgresGroupLifecycleEvent(ctx, tx, accountID, command.GroupID, "group.state_changed",
			map[string]any{"previous_state": command.ExpectedState, "state": command.State, "changed_by": command.ChangedBy},
			command.ChangedAt); err != nil {
			return err
		}
		record, err = getPostgresGroup(ctx, tx, accountID, command.GroupID)
		return err
	})
	return record, err
}

func (store *Store) ReplaceGroupMembers(ctx context.Context, command ledger.ReplaceGroupMembersCommand) (ledger.GroupRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.GroupRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	var record ledger.GroupRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, groupMembersReplaceScope,
			command.IdempotencyKey, digest, "group", command.GroupID, command.ChangedAt)
		if err != nil {
			return err
		}
		if !claimed {
			record, err = getPostgresGroup(ctx, tx, accountID, command.GroupID)
			return err
		}
		var conversationID, currentState, currentMembershipID string
		err = tx.queryRow(ctx, `select identity.conversation_id, conversation.state,
  conversation.current_membership_revision_id
from fort_private.group_conversation as identity
join fort_private.conversation as conversation
  on conversation.account_id = identity.account_id
 and conversation.conversation_id = identity.conversation_id
where identity.account_id = $1 and identity.group_id = $2
for update of conversation`, accountID, command.GroupID).scan(&conversationID, &currentState, &currentMembershipID)
		if isNoRows(err) {
			return fmt.Errorf("%w: Group %q", ledger.ErrNotFound, command.GroupID)
		}
		if err != nil {
			return err
		}
		if currentState != string(conversation.ConversationOpen) {
			return fmt.Errorf("%w: archived Group", ledger.ErrStateConflict)
		}
		current, err := getPostgresGroup(ctx, tx, accountID, command.GroupID)
		if err != nil {
			return err
		}
		if currentMembershipID != command.ExpectedMembershipRevisionID || current.Membership.ID != currentMembershipID ||
			command.Membership.Revision != current.Membership.Revision+1 {
			return fmt.Errorf("%w: Group Membership Revision", ledger.ErrRevisionConflict)
		}
		active, err := postgresGroupHasActiveWork(ctx, tx, accountID, command.GroupID)
		if err != nil {
			return err
		}
		if active {
			return fmt.Errorf("%w: Group has an active Turn, target, or Handoff", ledger.ErrStateConflict)
		}
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_membership_revision (
  account_id, membership_revision_id, conversation_id, revision,
  command_digest, created_by, created_at
) values ($1,$2,$3,$4,$5,$6,$7)`, accountID, command.Membership.ID, conversationID,
			command.Membership.Revision, digest, command.ChangedBy, command.Membership.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert successor Group Membership Revision: %w", err)
		}
		for position, member := range command.Membership.Members {
			binding := command.MemberBindings[position]
			agent, err := getAgentRecord(ctx, tx, accountID, member.AgentID)
			if err != nil {
				return err
			}
			if agent.Agent.State != conversation.AgentOpen || agent.Agent.CurrentBehaviorRevisionID != binding.BehaviorRevisionID ||
				agent.Agent.CurrentBindingRevisionID != binding.BindingRevisionID || agent.Behavior.ID != binding.BehaviorRevisionID ||
				agent.Binding.ID != binding.BindingRevisionID {
				return fmt.Errorf("Group member %q binding evidence is not current", member.AgentID)
			}
			if _, err := tx.exec(ctx, `insert into fort_private.conversation_member_revision (
  account_id, membership_revision_id, conversation_id, agent_id,
  position, added_by, created_at
) values ($1,$2,$3,$4,$5,$6,$7)`, accountID, command.Membership.ID, conversationID,
				member.AgentID, member.Position, command.ChangedBy, command.Membership.CreatedAt.UTC()); err != nil {
				return fmt.Errorf("insert successor Group member: %w", err)
			}
			var existingBehaviorID, existingParticipantID string
			err = tx.queryRow(ctx, `select behavior_revision_id, participant_id
from fort_private.conversation_participant
where account_id = $1 and conversation_id = $2 and agent_id = $3 and binding_revision_id = $4`,
				accountID, conversationID, member.AgentID, binding.BindingRevisionID).scan(
				&existingBehaviorID, &existingParticipantID)
			if err == nil {
				if existingBehaviorID != binding.BehaviorRevisionID || existingParticipantID != binding.ParticipantID {
					return fmt.Errorf("Group member %q participant evidence does not match its current Binding", member.AgentID)
				}
			} else if isNoRows(err) {
				if err := insertPostgresGroupParticipant(ctx, tx, accountID, conversationID, member.Position,
					agent, binding, command.ChangedAt); err != nil {
					return err
				}
			} else {
				return err
			}
			if _, err := tx.exec(ctx, `insert into fort_private.conversation_member_binding (
  account_id, membership_revision_id, conversation_id, agent_id,
  behavior_revision_id, binding_revision_id, participant_id, pinned_at
) values ($1,$2,$3,$4,$5,$6,$7,$8)`, accountID, command.Membership.ID, conversationID,
				member.AgentID, binding.BehaviorRevisionID, binding.BindingRevisionID,
				binding.ParticipantID, command.Membership.CreatedAt.UTC()); err != nil {
				return fmt.Errorf("insert successor Group member binding evidence: %w", err)
			}
		}
		affected, err := tx.exec(ctx, `update fort_private.conversation
set current_membership_revision_id = $1, updated_at = $2
where account_id = $3 and conversation_id = $4
  and current_membership_revision_id = $5 and state = 'open'`, command.Membership.ID,
			command.ChangedAt.UTC(), accountID, conversationID, command.ExpectedMembershipRevisionID)
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("%w: Group Membership Revision", ledger.ErrRevisionConflict)
		}
		agentIDs := make([]string, 0, len(command.Membership.Members))
		for _, member := range command.Membership.Members {
			agentIDs = append(agentIDs, member.AgentID)
		}
		if err := insertPostgresGroupLifecycleEvent(ctx, tx, accountID, command.GroupID, "group.members_replaced",
			map[string]any{"previous_membership_revision_id": command.ExpectedMembershipRevisionID,
				"membership_revision_id": command.Membership.ID, "agent_ids": agentIDs, "changed_by": command.ChangedBy},
			command.ChangedAt); err != nil {
			return err
		}
		record, err = getPostgresGroup(ctx, tx, accountID, command.GroupID)
		return err
	})
	return record, err
}

func insertPostgresGroupParticipant(ctx context.Context, tx transaction, accountID, conversationID string, position int,
	agent ledger.AgentRecord, binding conversation.GroupRecipient, createdAt time.Time) error {
	machine := agent.Binding.ComputerID
	if machine == "" {
		machine = agent.Binding.CloudRuntime
	}
	participant := conversation.Participant{
		ID: binding.ParticipantID, ConversationID: conversationID, SeatID: agent.Binding.SeatID,
		Profile: agent.Binding.FortProfile, Agent: agent.Binding.Provider, Model: agent.Binding.RequestedModel,
		Machine: machine, DisplayName: agent.Profile.Name, Position: position,
		State: conversation.ParticipantActive, CreatedAt: createdAt,
	}
	seat, authority, snapshotDigest, err := participantEvidence(ledger.CreateAgentCommand{
		Binding: agent.Binding, Participant: participant,
	})
	if err != nil {
		return err
	}
	if _, err := tx.exec(ctx, `insert into fort_private.conversation_participant (
  account_id, participant_id, conversation_id, agent_id,
  behavior_revision_id, binding_revision_id, seat_snapshot,
  authority_snapshot, snapshot_digest, created_at
) values ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9,$10)`, accountID,
		binding.ParticipantID, conversationID, binding.AgentID, binding.BehaviorRevisionID,
		binding.BindingRevisionID, seat, authority, snapshotDigest, createdAt.UTC()); err != nil {
		return fmt.Errorf("insert Group participant evidence: %w", err)
	}
	return nil
}

func postgresGroupHasActiveWork(ctx context.Context, tx transaction, accountID, groupID string) (bool, error) {
	var active bool
	err := tx.queryRow(ctx, `select exists (
  select 1
  from fort_private.group_conversation as group_item
  join fort_private.conversation_turn as turn_item
    on turn_item.account_id = group_item.account_id
   and turn_item.conversation_id = group_item.conversation_id
  where group_item.account_id = $1 and group_item.group_id = $2
    and turn_item.kind = 'human_group'
    and (
      turn_item.state in ('open','needs_you')
      or exists (
        select 1 from fort_private.conversation_target as target
        where target.account_id = turn_item.account_id and target.turn_id = turn_item.turn_id
          and target.target_kind = 'initial'
          and target.state in ('queued','claimed','working','needs_you','cancel_requested','lease_expired')
      )
      or exists (
        select 1 from fort_private.handoff as handoff
        where handoff.account_id = turn_item.account_id and handoff.group_turn_id = turn_item.turn_id
          and handoff.state in ('requested','needs_approval','queued','working','needs_you')
      )
    )
)`, accountID, groupID).scan(&active)
	return active, err
}

func insertPostgresGroupLifecycleEvent(ctx context.Context, tx transaction, accountID, groupID, eventType string,
	metadata any, createdAt time.Time) error {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	if strings.TrimSpace(eventType) == "" {
		return fmt.Errorf("Group lifecycle event type is required")
	}
	_, err = tx.exec(ctx, `insert into fort_private.ledger_event (
  account_id, aggregate_kind, aggregate_id, event_type, event_metadata, created_at
) values ($1, 'group', $2, $3, $4::jsonb, $5)`, accountID, groupID, eventType,
		string(payload), createdAt.UTC())
	return err
}
