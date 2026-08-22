package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

const (
	groupCreateScope = "group.create"
	groupCreateActor = "fort-control:group-create"
)

var _ ledger.CollaborationRepository = (*Store)(nil)

// CreateGroup commits one distinct stable Group identity, its Conversation,
// immutable membership revision, and exact participant evidence atomically.
func (store *Store) CreateGroup(ctx context.Context, command ledger.CreateGroupCommand) (ledger.GroupRecord, error) {
	accountID, err := store.operationAccount(command.Group.AccountID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	command.Group.AccountID = accountID
	if command.Conversation.ProjectID != "" {
		return ledger.GroupRecord{}, fmt.Errorf("cloud Group Conversation does not support a local Project id")
	}
	if err := command.Validate(); err != nil {
		return ledger.GroupRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.GroupRecord{}, err
	}

	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, groupCreateScope,
			command.IdempotencyKey, digest, "group", command.Group.ID, command.Group.CreatedAt)
		if err != nil || !claimed {
			return err
		}
		return insertGroupAggregate(ctx, tx, command, digest)
	})
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	return groupRecord(command), nil
}

func insertGroupAggregate(ctx context.Context, tx transaction, command ledger.CreateGroupCommand, digest string) error {
	if _, err := tx.exec(ctx, `insert into fort_private.conversation (
  account_id, conversation_id, kind, title, state,
  current_membership_revision_id, created_at, updated_at
) values ($1,$2,'group',$3,$4,$5,$6,$7)`, command.Group.AccountID,
		command.Conversation.ID, command.Conversation.Title, command.Conversation.State,
		command.Membership.ID, command.Conversation.CreatedAt.UTC(), command.Conversation.UpdatedAt.UTC()); err != nil {
		return fmt.Errorf("insert Group Conversation: %w", err)
	}
	if _, err := tx.exec(ctx, `insert into fort_private.group_conversation (
  account_id, group_id, conversation_id, created_at
) values ($1,$2,$3,$4)`, command.Group.AccountID, command.Group.ID,
		command.Group.ConversationID, command.Group.CreatedAt.UTC()); err != nil {
		return fmt.Errorf("insert Group identity: %w", err)
	}
	if _, err := tx.exec(ctx, `insert into fort_private.conversation_membership_revision (
  account_id, membership_revision_id, conversation_id, revision,
  command_digest, created_by, created_at
) values ($1,$2,$3,$4,$5,$6,$7)`, command.Group.AccountID, command.Membership.ID,
		command.Group.ConversationID, command.Membership.Revision, digest, groupCreateActor,
		command.Membership.CreatedAt.UTC()); err != nil {
		return fmt.Errorf("insert Group Membership Revision: %w", err)
	}

	for position, member := range command.Membership.Members {
		binding := command.MemberBindings[position]
		agent, err := getAgentRecord(ctx, tx, command.Group.AccountID, member.AgentID)
		if err != nil {
			return err
		}
		if agent.Agent.State != conversation.AgentOpen ||
			agent.Agent.CurrentBehaviorRevisionID != binding.BehaviorRevisionID ||
			agent.Agent.CurrentBindingRevisionID != binding.BindingRevisionID {
			return fmt.Errorf("Group member %q binding evidence is not current", member.AgentID)
		}
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_member_revision (
  account_id, membership_revision_id, conversation_id, agent_id,
  position, added_by, created_at
) values ($1,$2,$3,$4,$5,$6,$7)`, command.Group.AccountID, command.Membership.ID,
			command.Group.ConversationID, member.AgentID, member.Position, groupCreateActor,
			command.Membership.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Group member: %w", err)
		}

		machine := agent.Binding.ComputerID
		if machine == "" {
			machine = agent.Binding.CloudRuntime
		}
		participant := conversation.Participant{
			ID: binding.ParticipantID, ConversationID: command.Group.ConversationID,
			SeatID: agent.Binding.SeatID, Profile: agent.Binding.FortProfile,
			Agent: agent.Binding.Provider, Model: agent.Binding.RequestedModel,
			Machine: machine, DisplayName: agent.Profile.Name, Position: member.Position,
			State: conversation.ParticipantActive, CreatedAt: command.Group.CreatedAt,
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
) values ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9,$10)`, command.Group.AccountID,
			binding.ParticipantID, command.Group.ConversationID, binding.AgentID,
			binding.BehaviorRevisionID, binding.BindingRevisionID, seat, authority,
			snapshotDigest, command.Group.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Group participant evidence: %w", err)
		}
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_member_binding (
  account_id, membership_revision_id, conversation_id, agent_id,
  behavior_revision_id, binding_revision_id, participant_id, pinned_at
) values ($1,$2,$3,$4,$5,$6,$7,$8)`, command.Group.AccountID, command.Membership.ID,
			command.Group.ConversationID, binding.AgentID, binding.BehaviorRevisionID,
			binding.BindingRevisionID, binding.ParticipantID, command.Membership.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert Group member binding evidence: %w", err)
		}
	}
	return nil
}

func (store *Store) GetGroup(ctx context.Context, accountID, groupID string) (ledger.GroupRecord, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if strings.TrimSpace(groupID) == "" {
		return ledger.GroupRecord{}, fmt.Errorf("Group id is required")
	}
	var record ledger.GroupRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		var err error
		record, err = getPostgresGroup(ctx, tx, accountID, groupID)
		return err
	})
	return record, err
}

func (store *Store) ListGroups(ctx context.Context, accountID string, state conversation.ConversationState) ([]ledger.GroupRecord, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return nil, err
	}
	if state == "" {
		state = conversation.ConversationOpen
	}
	if state != conversation.ConversationOpen && state != conversation.ConversationArchived {
		return nil, fmt.Errorf("Group state must be open or archived")
	}
	records := make([]ledger.GroupRecord, 0)
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		result, err := tx.query(ctx, `select identity.group_id
from fort_private.group_conversation as identity
join fort_private.conversation as conversation
  on conversation.account_id = identity.account_id
 and conversation.conversation_id = identity.conversation_id
where identity.account_id = $1 and conversation.state = $2
order by identity.created_at, identity.group_id`, accountID, state)
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
			record, err := getPostgresGroup(ctx, tx, accountID, id)
			if err != nil {
				return err
			}
			records = append(records, record)
		}
		return nil
	})
	return records, err
}

func getPostgresGroup(ctx context.Context, tx transaction, accountID, groupID string) (ledger.GroupRecord, error) {
	var record ledger.GroupRecord
	err := tx.queryRow(ctx, `select identity.group_id, identity.account_id,
  identity.conversation_id, conversation.state,
  conversation.current_membership_revision_id, identity.created_at,
  conversation.title, conversation.created_at, conversation.updated_at,
  membership.membership_revision_id, membership.revision, membership.created_at
from fort_private.group_conversation as identity
join fort_private.conversation as conversation
  on conversation.account_id = identity.account_id
 and conversation.conversation_id = identity.conversation_id
 and conversation.kind = 'group'
join fort_private.conversation_membership_revision as membership
  on membership.account_id = conversation.account_id
 and membership.conversation_id = conversation.conversation_id
 and membership.membership_revision_id = conversation.current_membership_revision_id
where identity.account_id = $1 and identity.group_id = $2`, accountID, groupID).scan(
		&record.Group.ID, &record.Group.AccountID, &record.Group.ConversationID,
		&record.Group.State, &record.Group.CurrentMembershipRevisionID, &record.Group.CreatedAt,
		&record.Conversation.Title, &record.Conversation.CreatedAt, &record.Conversation.UpdatedAt,
		&record.Membership.ID, &record.Membership.Revision, &record.Membership.CreatedAt,
	)
	if isNoRows(err) {
		return ledger.GroupRecord{}, fmt.Errorf("%w: Group %q", ledger.ErrNotFound, groupID)
	}
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	record.Conversation.ID = record.Group.ConversationID
	record.Conversation.State = record.Group.State
	record.Membership.GroupID = record.Group.ID
	record.Membership.Members = make([]conversation.GroupMember, 0)
	record.MemberBindings = make([]conversation.GroupRecipient, 0)
	result, err := tx.query(ctx, `select member.agent_id, member.position,
  binding.behavior_revision_id, binding.binding_revision_id,
  binding.participant_id
from fort_private.conversation_member_revision as member
join fort_private.conversation_member_binding as binding
  on binding.account_id = member.account_id
 and binding.membership_revision_id = member.membership_revision_id
 and binding.conversation_id = member.conversation_id
 and binding.agent_id = member.agent_id
where member.account_id = $1 and member.conversation_id = $2
  and member.membership_revision_id = $3
order by member.position, member.agent_id`, accountID, record.Group.ConversationID,
		record.Membership.ID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	defer result.close()
	for result.next() {
		var member conversation.GroupMember
		var binding conversation.GroupRecipient
		if err := result.scan(&member.AgentID, &member.Position, &binding.BehaviorRevisionID,
			&binding.BindingRevisionID, &binding.ParticipantID); err != nil {
			return ledger.GroupRecord{}, err
		}
		binding.AgentID = member.AgentID
		record.Membership.Members = append(record.Membership.Members, member)
		record.MemberBindings = append(record.MemberBindings, binding)
	}
	if err := result.errResult(); err != nil {
		return ledger.GroupRecord{}, err
	}
	return record, nil
}

func groupRecord(command ledger.CreateGroupCommand) ledger.GroupRecord {
	command.Membership.Members = append([]conversation.GroupMember{}, command.Membership.Members...)
	command.MemberBindings = append([]conversation.GroupRecipient{}, command.MemberBindings...)
	return ledger.GroupRecord{Group: command.Group, Conversation: command.Conversation,
		Membership: command.Membership, MemberBindings: command.MemberBindings}
}
