package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

const agentProfileAppendScope = "agent.profile.append"

const agentBehaviorAppendScope = "agent.behavior.append"

var _ ledger.AgentLifecycleRepository = (*Store)(nil)

func (store *Store) AppendAgentProfile(ctx context.Context, command ledger.AppendAgentProfileCommand) (ledger.AgentRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.AgentRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentRecord{}, err
	}

	var record ledger.AgentRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, agentProfileAppendScope,
			command.IdempotencyKey, digest, "agent_profile_revision", command.Revision.ID, command.Revision.CreatedAt)
		if err != nil {
			return err
		}
		if !claimed {
			record, err = getAgentRecord(ctx, tx, accountID, command.AgentID)
			return err
		}

		var currentID string
		var currentRevision int
		err = tx.queryRow(ctx, `select agent.current_profile_revision_id, profile.revision
from fort_private.stable_agent as agent
join fort_private.agent_profile_revision as profile
  on profile.account_id = agent.account_id
 and profile.agent_id = agent.agent_id
 and profile.profile_revision_id = agent.current_profile_revision_id
where agent.account_id = $1 and agent.agent_id = $2
for update of agent`, accountID, command.AgentID).scan(&currentID, &currentRevision)
		if isNoRows(err) {
			return fmt.Errorf("%w: Agent %q", ledger.ErrNotFound, command.AgentID)
		}
		if err != nil {
			return err
		}
		if currentID != command.ExpectedProfileRevisionID || command.Revision.Revision != currentRevision+1 {
			return fmt.Errorf("%w: Agent Profile Revision", ledger.ErrRevisionConflict)
		}
		if _, err := tx.exec(ctx, `insert into fort_private.agent_profile_revision (
  account_id, profile_revision_id, agent_id, revision, name, title,
  avatar_url, hidden, pinned, sort_order, created_by, created_at
) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			accountID, command.Revision.ID, command.Revision.AgentID, command.Revision.Revision,
			command.Revision.Name, command.Revision.Title, command.Revision.AvatarURL,
			command.Revision.Hidden, command.Revision.Pinned, command.Revision.SortOrder,
			command.AcceptedBy, command.Revision.CreatedAt.UTC()); err != nil {
			return err
		}
		affected, err := tx.exec(ctx, `update fort_private.stable_agent
set current_profile_revision_id = $1, updated_at = $2
where account_id = $3 and agent_id = $4 and current_profile_revision_id = $5`, command.Revision.ID,
			command.Revision.CreatedAt.UTC(), accountID, command.AgentID, command.ExpectedProfileRevisionID)
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("%w: Agent Profile Revision", ledger.ErrRevisionConflict)
		}
		metadata, err := json.Marshal(map[string]any{
			"previous_profile_revision_id": command.ExpectedProfileRevisionID,
			"profile_revision_id":          command.Revision.ID,
			"accepted_by":                  command.AcceptedBy,
		})
		if err != nil {
			return err
		}
		if _, err := tx.exec(ctx, `insert into fort_private.ledger_event (
  account_id, aggregate_kind, aggregate_id, event_type, event_metadata, created_at
) values ($1, 'stable_agent', $2, 'agent.profile.advanced', $3::jsonb, $4)`, accountID,
			command.AgentID, string(metadata), command.Revision.CreatedAt.UTC()); err != nil {
			return err
		}
		record, err = getAgentRecord(ctx, tx, accountID, command.AgentID)
		return err
	})
	return record, err
}

func (store *Store) AppendAgentBehavior(ctx context.Context, command ledger.AppendAgentBehaviorCommand) (ledger.AgentBindingAdvanceResult, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	command.AccountID = accountID
	command.Behavior.EnabledSkills = sortedCopy(command.Behavior.EnabledSkills)
	command.Behavior.EnabledTools = sortedCopy(command.Behavior.EnabledTools)
	command.Binding.CapabilityEvidence = sortedCopy(command.Binding.CapabilityEvidence)
	if err := command.Validate(); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}

	var result ledger.AgentBindingAdvanceResult
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, agentBehaviorAppendScope,
			command.IdempotencyKey, digest, "agent_binding_revision", command.Binding.ID, command.AcceptedAt)
		if err != nil {
			return err
		}
		if !claimed {
			result.Agent, err = getAgentRecord(ctx, tx, accountID, command.AgentID)
			if err != nil {
				return err
			}
			result.Transition, err = getAgentBindingTransition(ctx, tx, accountID, command.AgentID, command.Binding.ID)
			return err
		}

		current, err := getAgentRecord(ctx, tx, accountID, command.AgentID)
		if err != nil {
			return err
		}
		if current.Agent.State != "open" {
			return fmt.Errorf("%w: archived Agent", ledger.ErrStateConflict)
		}
		if current.Behavior.ID != command.ExpectedBehaviorRevisionID || current.Binding.ID != command.ExpectedBindingRevisionID ||
			command.Behavior.Revision != current.Behavior.Revision+1 || command.Binding.Revision != current.Binding.Revision+1 {
			return fmt.Errorf("%w: Agent Behavior or Binding Revision", ledger.ErrRevisionConflict)
		}
		if !samePostgresBindingExecution(current.Binding, command.Binding) {
			return fmt.Errorf("Behavior acceptance cannot change execution identity; use explicit Rebind")
		}
		if command.Participant.ConversationID != current.Home.ID || command.Participant.DisplayName != current.Profile.Name {
			return fmt.Errorf("Behavior participant evidence does not match Agent Home and profile")
		}

		skills, err := json.Marshal(command.Behavior.EnabledSkills)
		if err != nil {
			return err
		}
		tools, err := json.Marshal(command.Behavior.EnabledTools)
		if err != nil {
			return err
		}
		behaviorDigest, err := evidenceDigest(command.Behavior)
		if err != nil {
			return err
		}
		if _, err := tx.exec(ctx, `insert into fort_private.agent_behavior_revision (
  account_id, behavior_revision_id, agent_id, revision, role,
  standing_instructions, enabled_skills, enabled_tools, prompt_material,
  behavior_digest, created_by, created_at
) values ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9, $10, $11, $12)`, accountID,
			command.Behavior.ID, command.AgentID, command.Behavior.Revision, command.Behavior.Role,
			command.Behavior.StandingInstructions, string(skills), string(tools), command.Behavior.PromptMaterial,
			behaviorDigest, command.AcceptedBy, command.Behavior.CreatedAt.UTC()); err != nil {
			return err
		}
		capabilities, err := encodeCapabilityEvidence(command.Binding)
		if err != nil {
			return err
		}
		workerID, err := bindingWorker(command.Binding)
		if err != nil {
			return err
		}
		if _, err := tx.exec(ctx, `insert into fort_private.agent_binding_revision (
  account_id, binding_revision_id, agent_id, revision, behavior_revision_id,
  execution_source_id, source_agent_id, worker_id, seat_id, fort_profile,
  provider, requested_model, resolved_model, adapter_id, adapter_revision,
  source_config_digest, authority_id, authority_revision, policy_id,
  policy_revision, session_behavior, memory_behavior, capability_evidence,
  readiness_contract_id, readiness_contract_revision,
  supersedes_binding_revision_id, activated_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23::jsonb,$24,$25,$26,$27)`,
			accountID, command.Binding.ID, command.AgentID, command.Binding.Revision,
			command.Binding.BehaviorRevisionID, command.Binding.ExecutionSourceID, command.Binding.SourceAgentID,
			workerID, command.Binding.SeatID, command.Binding.FortProfile, command.Binding.Provider,
			command.Binding.RequestedModel, command.Binding.ResolvedModel, command.Binding.AdapterID,
			command.Binding.AdapterRevision, command.Binding.SourceConfigDigest, command.Binding.AuthorityID,
			command.Binding.AuthorityRevision, command.Binding.PolicyID, command.Binding.PolicyRevision,
			command.Binding.SessionBehavior, command.Binding.MemoryBehavior, capabilities,
			command.Binding.ReadinessContractID, command.Binding.ReadinessContractRevision,
			command.Binding.SupersedesRevisionID, command.Binding.ActivatedAt.UTC()); err != nil {
			return err
		}
		seat, authority, participantDigest, err := participantEvidence(ledger.CreateAgentCommand{
			Binding: command.Binding, Participant: command.Participant,
		})
		if err != nil {
			return err
		}
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_participant (
  account_id, participant_id, conversation_id, agent_id,
  behavior_revision_id, binding_revision_id, seat_snapshot,
  authority_snapshot, snapshot_digest, created_at
) values ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9,$10)`, accountID, command.Participant.ID,
			command.Participant.ConversationID, command.AgentID, command.Behavior.ID, command.Binding.ID,
			seat, authority, participantDigest, command.Participant.CreatedAt.UTC()); err != nil {
			return err
		}
		transition := ledger.AgentBindingTransition{
			AccountID: accountID, AgentID: command.AgentID, Kind: ledger.BindingTransitionBehavior,
			PreviousBehaviorRevisionID: current.Behavior.ID, SuccessorBehaviorRevisionID: command.Behavior.ID,
			PreviousBindingRevisionID: current.Binding.ID, SuccessorBindingRevisionID: command.Binding.ID,
			PreviewDigest: digest, NonTransferableResources: []ledger.RebindResource{},
			ReadinessEvidence: sortedCopy(command.ReadinessEvidence), AuthorityEvidence: sortedCopy(command.AuthorityEvidence),
			AcceptedBy: command.AcceptedBy, AcceptedAt: command.AcceptedAt,
		}
		if err := insertPostgresBindingTransition(ctx, tx, transition); err != nil {
			return err
		}
		affected, err := tx.exec(ctx, `update fort_private.stable_agent
set current_behavior_revision_id = $1, current_binding_revision_id = $2, updated_at = $3
where account_id = $4 and agent_id = $5
  and current_behavior_revision_id = $6 and current_binding_revision_id = $7`, command.Behavior.ID,
			command.Binding.ID, command.AcceptedAt.UTC(), accountID, command.AgentID,
			command.ExpectedBehaviorRevisionID, command.ExpectedBindingRevisionID)
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("%w: Agent Behavior or Binding Revision", ledger.ErrRevisionConflict)
		}
		if err := insertBindingAdvanceEvent(ctx, tx, transition); err != nil {
			return err
		}
		result.Agent, err = getAgentRecord(ctx, tx, accountID, command.AgentID)
		if err != nil {
			return err
		}
		result.Transition = transition
		return nil
	})
	return result, err
}

func (store *Store) PreviewAgentRebind(ctx context.Context, command ledger.PreviewAgentRebindCommand) (ledger.AgentRebindPreview, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.AgentRebindPreview{}, err
	}
	command.AccountID = accountID
	command.ExecutionSource.AccountID = accountID
	command.Binding.CapabilityEvidence = sortedCopy(command.Binding.CapabilityEvidence)
	if err := command.Validate(); err != nil {
		return ledger.AgentRebindPreview{}, err
	}
	current, err := store.GetAgent(ctx, accountID, command.AgentID)
	if err != nil {
		return ledger.AgentRebindPreview{}, err
	}
	if current.Agent.State != conversation.AgentOpen {
		return ledger.AgentRebindPreview{}, fmt.Errorf("%w: archived Agent", ledger.ErrStateConflict)
	}
	if current.Binding.ID != command.ExpectedBindingRevisionID || command.Binding.Revision != current.Binding.Revision+1 {
		return ledger.AgentRebindPreview{}, fmt.Errorf("%w: Agent Binding Revision", ledger.ErrRevisionConflict)
	}
	if command.Binding.BehaviorRevisionID != current.Behavior.ID {
		return ledger.AgentRebindPreview{}, fmt.Errorf("Rebind must retain the current Behavior Revision")
	}
	if command.Participant.ConversationID != current.Home.ID || command.Participant.DisplayName != current.Profile.Name {
		return ledger.AgentRebindPreview{}, fmt.Errorf("Rebind participant evidence does not match Agent Home and profile")
	}
	preview := ledger.AgentRebindPreview{
		AccountID: accountID, AgentID: command.AgentID, CurrentBinding: current.Binding,
		CurrentExecutionSource: current.ExecutionSource, CurrentSourceAgent: current.SourceAgent,
		ProposedBinding: command.Binding, ProposedExecutionSource: command.ExecutionSource,
		ProposedSourceAgent: command.SourceAgent, Participant: command.Participant,
		NonTransferableResources: sortedPostgresRebindResources(command.NonTransferableResources),
		ReadinessEvidence:        sortedCopy(command.ReadinessEvidence), AuthorityEvidence: sortedCopy(command.AuthorityEvidence),
		GeneratedAt: command.GeneratedAt,
	}
	preview.Digest, err = preview.CalculateDigest()
	if err != nil {
		return ledger.AgentRebindPreview{}, err
	}
	return preview, preview.Validate()
}

func (store *Store) AcceptAgentRebind(ctx context.Context, command ledger.AcceptAgentRebindCommand) (ledger.AgentBindingAdvanceResult, error) {
	accountID, err := store.operationAccount(command.Preview.AccountID)
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	command.Preview.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	var result ledger.AgentBindingAdvanceResult
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, "agent.rebind.accept", command.IdempotencyKey,
			digest, "agent_binding_revision", command.Preview.ProposedBinding.ID, command.AcceptedAt)
		if err != nil {
			return err
		}
		if !claimed {
			result.Agent, err = getAgentRecord(ctx, tx, accountID, command.Preview.AgentID)
			if err != nil {
				return err
			}
			result.Transition, err = getAgentBindingTransition(ctx, tx, accountID, command.Preview.AgentID,
				command.Preview.ProposedBinding.ID)
			return err
		}
		current, err := getAgentRecord(ctx, tx, accountID, command.Preview.AgentID)
		if err != nil {
			return err
		}
		if current.Agent.State != conversation.AgentOpen {
			return fmt.Errorf("%w: archived Agent", ledger.ErrStateConflict)
		}
		if current.Binding.ID != command.Preview.CurrentBinding.ID ||
			!reflect.DeepEqual(canonicalPostgresBinding(current.Binding), canonicalPostgresBinding(command.Preview.CurrentBinding)) ||
			command.Preview.ProposedBinding.Revision != current.Binding.Revision+1 ||
			command.Preview.ProposedBinding.BehaviorRevisionID != current.Behavior.ID {
			return fmt.Errorf("%w: Agent Binding Revision", ledger.ErrRevisionConflict)
		}
		if command.Preview.Participant.ConversationID != current.Home.ID ||
			command.Preview.Participant.DisplayName != current.Profile.Name {
			return fmt.Errorf("Rebind participant evidence does not match Agent Home and profile")
		}
		evidence := ledger.CreateAgentCommand{
			Agent: current.Agent, Binding: command.Preview.ProposedBinding,
			ExecutionSource: command.Preview.ProposedExecutionSource, SourceAgent: command.Preview.ProposedSourceAgent,
		}
		if err := ensureExecutionSource(ctx, tx, evidence); err != nil {
			return err
		}
		if err := appendPostgresSourceConfigObservation(ctx, tx,
			"source-observation:rebind:"+command.Preview.ProposedBinding.ID, accountID,
			command.Preview.ProposedBinding.ExecutionSourceID,
			command.Preview.ProposedBinding.SourceConfigDigest, command.AcceptedBy,
			command.Preview.ProposedBinding.ActivatedAt); err != nil {
			return err
		}
		if err := ensureSourceAgent(ctx, tx, evidence); err != nil {
			return err
		}
		if err := insertLifecycleBinding(ctx, tx, accountID, command.Preview.ProposedBinding); err != nil {
			return err
		}
		if err := insertLifecycleParticipant(ctx, tx, accountID, command.Preview.AgentID, current.Behavior.ID,
			command.Preview.ProposedBinding, command.Preview.Participant); err != nil {
			return err
		}
		transition := ledger.AgentBindingTransition{
			AccountID: accountID, AgentID: command.Preview.AgentID, Kind: ledger.BindingTransitionRebind,
			PreviousBehaviorRevisionID: current.Behavior.ID, SuccessorBehaviorRevisionID: current.Behavior.ID,
			PreviousBindingRevisionID:  current.Binding.ID,
			SuccessorBindingRevisionID: command.Preview.ProposedBinding.ID, PreviewDigest: command.Preview.Digest,
			NonTransferableResources: sortedPostgresRebindResources(command.Preview.NonTransferableResources),
			ReadinessEvidence:        sortedCopy(command.Preview.ReadinessEvidence), AuthorityEvidence: sortedCopy(command.Preview.AuthorityEvidence),
			AcceptedBy: command.AcceptedBy, AcceptedAt: command.AcceptedAt,
		}
		if err := insertPostgresBindingTransition(ctx, tx, transition); err != nil {
			return err
		}
		affected, err := tx.exec(ctx, `update fort_private.stable_agent
set current_binding_revision_id = $1, updated_at = $2
where account_id = $3 and agent_id = $4
  and current_binding_revision_id = $5 and current_behavior_revision_id = $6`,
			command.Preview.ProposedBinding.ID, command.AcceptedAt.UTC(), accountID, command.Preview.AgentID,
			current.Binding.ID, current.Behavior.ID)
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("%w: Agent Binding Revision", ledger.ErrRevisionConflict)
		}
		if err := insertBindingAdvanceEvent(ctx, tx, transition); err != nil {
			return err
		}
		result.Agent, err = getAgentRecord(ctx, tx, accountID, command.Preview.AgentID)
		if err != nil {
			return err
		}
		result.Transition = transition
		return nil
	})
	return result, err
}

func (store *Store) CreateSecondaryConversation(ctx context.Context, command ledger.CreateSecondaryConversationCommand) (ledger.AgentConversationRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	command.AccountID = accountID
	if command.Conversation.ProjectID != "" {
		return ledger.AgentConversationRecord{}, fmt.Errorf("cloud Agent Conversation does not support a local Project id")
	}
	if err := command.Validate(); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	var record ledger.AgentConversationRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, "agent.conversation.create",
			command.IdempotencyKey, digest, "conversation", command.Conversation.ID, command.Conversation.CreatedAt)
		if err != nil {
			return err
		}
		if !claimed {
			record, err = getPostgresAgentConversation(ctx, tx, accountID, command.AgentID, command.Conversation.ID)
			return err
		}
		var agentState, canonicalID string
		err = tx.queryRow(ctx, `select state, canonical_conversation_id
from fort_private.stable_agent where account_id = $1 and agent_id = $2
for update`, accountID, command.AgentID).scan(&agentState, &canonicalID)
		if isNoRows(err) {
			return fmt.Errorf("%w: Agent %q", ledger.ErrNotFound, command.AgentID)
		}
		if err != nil {
			return err
		}
		if agentState != string(conversation.AgentOpen) {
			return fmt.Errorf("%w: archived Agent", ledger.ErrStateConflict)
		}
		if canonicalID == command.Conversation.ID {
			return fmt.Errorf("secondary Conversation cannot replace Home")
		}
		membershipID := homeMembershipID(accountID, command.AgentID, command.Conversation.ID)
		if _, err := tx.exec(ctx, `insert into fort_private.conversation (
  account_id, conversation_id, kind, title, state, current_membership_revision_id, created_at, updated_at
) values ($1,$2,'agent',$3,$4,$5,$6,$7)`, accountID, command.Conversation.ID,
			command.Conversation.Title, command.Conversation.State, membershipID,
			command.Conversation.CreatedAt.UTC(), command.Conversation.UpdatedAt.UTC()); err != nil {
			return err
		}
		if _, err := tx.exec(ctx, `insert into fort_private.agent_conversation (
  account_id, agent_id, conversation_id, kind, created_at
) values ($1,$2,$3,'secondary',$4)`, accountID, command.AgentID, command.Conversation.ID,
			command.Link.CreatedAt.UTC()); err != nil {
			return err
		}
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_membership_revision (
  account_id, membership_revision_id, conversation_id, revision, command_digest, created_by, created_at
) values ($1,$2,$3,1,$4,$5,$6)`, accountID, membershipID, command.Conversation.ID, digest,
			command.CreatedBy, command.Conversation.CreatedAt.UTC()); err != nil {
			return err
		}
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_member_revision (
  account_id, membership_revision_id, conversation_id, agent_id, position, added_by, created_at
) values ($1,$2,$3,$4,0,$5,$6)`, accountID, membershipID, command.Conversation.ID,
			command.AgentID, command.CreatedBy, command.Conversation.CreatedAt.UTC()); err != nil {
			return err
		}
		if err := insertAgentConversationEvent(ctx, tx, accountID, command.AgentID, "agent.conversation.created",
			map[string]any{"conversation_id": command.Conversation.ID, "kind": command.Link.Kind, "created_by": command.CreatedBy},
			command.Conversation.CreatedAt); err != nil {
			return err
		}
		record, err = getPostgresAgentConversation(ctx, tx, accountID, command.AgentID, command.Conversation.ID)
		return err
	})
	return record, err
}

func (store *Store) ListAgentConversations(ctx context.Context, accountID, agentID string) ([]ledger.AgentConversationRecord, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(agentID) == "" {
		return nil, fmt.Errorf("Agent id is required")
	}
	records := make([]ledger.AgentConversationRecord, 0)
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		var exists int
		if err := tx.queryRow(ctx, `select 1 from fort_private.stable_agent
where account_id = $1 and agent_id = $2`, accountID, agentID).scan(&exists); isNoRows(err) {
			return fmt.Errorf("%w: Agent %q", ledger.ErrNotFound, agentID)
		} else if err != nil {
			return err
		}
		result, err := tx.query(ctx, `select item.conversation_id, item.title, item.state, item.created_at, item.updated_at,
  relation.agent_id, relation.conversation_id, relation.kind, relation.created_at,
  coalesce(pin.pinned, false), case when pin.pinned then pin.changed_at else null end
from fort_private.agent_conversation as relation
join fort_private.conversation as item
  on item.account_id = relation.account_id and item.conversation_id = relation.conversation_id
left join lateral (
  select revision.pinned, revision.changed_at
  from fort_private.agent_conversation_pin as revision
  where revision.account_id = relation.account_id and revision.agent_id = relation.agent_id
    and revision.conversation_id = relation.conversation_id
  order by revision.revision desc limit 1
) as pin on true
where relation.account_id = $1 and relation.agent_id = $2
order by case relation.kind when 'canonical' then 0 else 1 end,
  coalesce(pin.pinned, false) desc, item.updated_at desc, relation.conversation_id`, accountID, agentID)
		if err != nil {
			return err
		}
		defer result.close()
		for result.next() {
			record, err := scanPostgresAgentConversation(result)
			if err != nil {
				return err
			}
			records = append(records, record)
		}
		return result.errResult()
	})
	return records, err
}

func (store *Store) RenameAgentConversation(ctx context.Context, command ledger.RenameAgentConversationCommand) (ledger.AgentConversationRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	var record ledger.AgentConversationRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, "agent.conversation.rename",
			command.IdempotencyKey, digest, "conversation", command.ConversationID, command.ChangedAt)
		if err != nil {
			return err
		}
		if !claimed {
			record, err = getPostgresAgentConversation(ctx, tx, accountID, command.AgentID, command.ConversationID)
			return err
		}
		var kind, currentTitle string
		err = tx.queryRow(ctx, `select relation.kind, item.title
from fort_private.stable_agent as agent
join fort_private.agent_conversation as relation
  on relation.account_id = agent.account_id and relation.agent_id = agent.agent_id
join fort_private.conversation as item
  on item.account_id = relation.account_id and item.conversation_id = relation.conversation_id
where agent.account_id = $1 and agent.agent_id = $2 and item.conversation_id = $3
for update of item`, accountID, command.AgentID, command.ConversationID).scan(&kind, &currentTitle)
		if isNoRows(err) {
			return fmt.Errorf("%w: Agent Conversation %q", ledger.ErrNotFound, command.ConversationID)
		}
		if err != nil {
			return err
		}
		if kind == string(conversation.AgentConversationCanonical) {
			return fmt.Errorf("%w: Home cannot be renamed", ledger.ErrStateConflict)
		}
		if currentTitle != command.ExpectedTitle {
			return fmt.Errorf("%w: Conversation title", ledger.ErrRevisionConflict)
		}
		affected, err := tx.exec(ctx, `update fort_private.conversation set title = $1, updated_at = $2
where account_id = $3 and conversation_id = $4 and title = $5`, command.Title, command.ChangedAt.UTC(),
			accountID, command.ConversationID, command.ExpectedTitle)
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("%w: Conversation title", ledger.ErrRevisionConflict)
		}
		if err := insertAgentConversationEvent(ctx, tx, accountID, command.AgentID, "agent.conversation.renamed",
			map[string]any{"conversation_id": command.ConversationID, "previous_title": command.ExpectedTitle,
				"title": command.Title, "changed_by": command.ChangedBy}, command.ChangedAt); err != nil {
			return err
		}
		record, err = getPostgresAgentConversation(ctx, tx, accountID, command.AgentID, command.ConversationID)
		return err
	})
	return record, err
}

func (store *Store) SetAgentConversationState(ctx context.Context, command ledger.SetAgentConversationStateCommand) (ledger.AgentConversationRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	var record ledger.AgentConversationRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, "agent.conversation.state",
			command.IdempotencyKey, digest, "conversation", command.ConversationID, command.ChangedAt)
		if err != nil {
			return err
		}
		if !claimed {
			record, err = getPostgresAgentConversation(ctx, tx, accountID, command.AgentID, command.ConversationID)
			return err
		}
		var agentState, kind, currentState string
		err = tx.queryRow(ctx, `select agent.state, relation.kind, item.state
from fort_private.stable_agent as agent
join fort_private.agent_conversation as relation
  on relation.account_id = agent.account_id and relation.agent_id = agent.agent_id
join fort_private.conversation as item
  on item.account_id = relation.account_id and item.conversation_id = relation.conversation_id
where agent.account_id = $1 and agent.agent_id = $2 and item.conversation_id = $3
for update of item`, accountID, command.AgentID, command.ConversationID).scan(&agentState, &kind, &currentState)
		if isNoRows(err) {
			return fmt.Errorf("%w: Agent Conversation %q", ledger.ErrNotFound, command.ConversationID)
		}
		if err != nil {
			return err
		}
		if currentState != string(command.ExpectedState) {
			return fmt.Errorf("%w: Conversation state", ledger.ErrStateConflict)
		}
		if kind == string(conversation.AgentConversationCanonical) && agentState == string(conversation.AgentOpen) &&
			command.State != conversation.ConversationOpen {
			return fmt.Errorf("%w: open Agent Home cannot be archived", ledger.ErrStateConflict)
		}
		affected, err := tx.exec(ctx, `update fort_private.conversation set state = $1, updated_at = $2
where account_id = $3 and conversation_id = $4 and state = $5`, command.State, command.ChangedAt.UTC(),
			accountID, command.ConversationID, command.ExpectedState)
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("%w: Conversation state", ledger.ErrStateConflict)
		}
		if err := insertAgentConversationEvent(ctx, tx, accountID, command.AgentID, "agent.conversation.state_changed",
			map[string]any{"conversation_id": command.ConversationID, "previous_state": command.ExpectedState,
				"state": command.State, "changed_by": command.ChangedBy}, command.ChangedAt); err != nil {
			return err
		}
		record, err = getPostgresAgentConversation(ctx, tx, accountID, command.AgentID, command.ConversationID)
		return err
	})
	return record, err
}

func (store *Store) SetAgentConversationPin(ctx context.Context, command ledger.SetAgentConversationPinCommand) (ledger.AgentConversationRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	var record ledger.AgentConversationRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, "agent.conversation.pin",
			command.IdempotencyKey, digest, "conversation", command.ConversationID, command.ChangedAt)
		if err != nil {
			return err
		}
		if !claimed {
			record, err = getPostgresAgentConversation(ctx, tx, accountID, command.AgentID, command.ConversationID)
			return err
		}
		var kind string
		var pinned bool
		var revision int
		err = tx.queryRow(ctx, `select relation.kind,
  coalesce((select pin.pinned from fort_private.agent_conversation_pin as pin
    where pin.account_id = relation.account_id and pin.agent_id = relation.agent_id
      and pin.conversation_id = relation.conversation_id
    order by pin.revision desc limit 1), false),
  coalesce((select max(pin.revision) from fort_private.agent_conversation_pin as pin
    where pin.account_id = relation.account_id and pin.agent_id = relation.agent_id
      and pin.conversation_id = relation.conversation_id), 0)
from fort_private.agent_conversation as relation
where relation.account_id = $1 and relation.agent_id = $2 and relation.conversation_id = $3
for update of relation`, accountID, command.AgentID, command.ConversationID).scan(&kind, &pinned, &revision)
		if isNoRows(err) {
			return fmt.Errorf("%w: Agent Conversation %q", ledger.ErrNotFound, command.ConversationID)
		}
		if err != nil {
			return err
		}
		if kind == string(conversation.AgentConversationCanonical) {
			return fmt.Errorf("%w: Home is permanently unpinned", ledger.ErrStateConflict)
		}
		if pinned != command.ExpectedPinned {
			return fmt.Errorf("%w: Conversation pin state", ledger.ErrStateConflict)
		}
		if _, err := tx.exec(ctx, `insert into fort_private.agent_conversation_pin (
  account_id, agent_id, conversation_id, revision, pinned, changed_by, changed_at
) values ($1,$2,$3,$4,$5,$6,$7)`, accountID, command.AgentID, command.ConversationID, revision+1,
			command.Pinned, command.ChangedBy, command.ChangedAt.UTC()); err != nil {
			return err
		}
		if err := insertAgentConversationEvent(ctx, tx, accountID, command.AgentID, "agent.conversation.pin_changed",
			map[string]any{"conversation_id": command.ConversationID, "previous_pinned": command.ExpectedPinned,
				"pinned": command.Pinned, "changed_by": command.ChangedBy}, command.ChangedAt); err != nil {
			return err
		}
		record, err = getPostgresAgentConversation(ctx, tx, accountID, command.AgentID, command.ConversationID)
		return err
	})
	return record, err
}

func claimLifecycleIdempotency(ctx context.Context, tx transaction, accountID, scope, key, digest, resultKind, resultID string, createdAt time.Time) (bool, error) {
	affected, err := tx.exec(ctx, `insert into fort_private.idempotency_record (
  account_id, scope, idempotency_key, command_digest, result_kind,
  result_id, response_digest, created_at
) values ($1, $2, $3, $4, $5, $6, $4, $7)
on conflict (account_id, scope, idempotency_key) do nothing`, accountID, scope, key, digest,
		resultKind, resultID, createdAt.UTC())
	if err != nil {
		return false, err
	}
	if affected == 1 {
		return true, nil
	}
	if affected != 0 {
		return false, fmt.Errorf("reserve lifecycle idempotency key affected %d rows", affected)
	}
	var existingDigest, existingKind, existingID string
	if err := tx.queryRow(ctx, `select command_digest, result_kind, result_id
from fort_private.idempotency_record
where account_id = $1 and scope = $2 and idempotency_key = $3`, accountID, scope, key).scan(
		&existingDigest, &existingKind, &existingID); err != nil {
		return false, err
	}
	if existingDigest != digest || existingKind != resultKind || existingID != resultID {
		return false, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, key)
	}
	return false, nil
}

func samePostgresBindingExecution(current, successor conversation.AgentBindingRevision) bool {
	current.ID, successor.ID = "", ""
	current.Revision, successor.Revision = 0, 0
	current.BehaviorRevisionID, successor.BehaviorRevisionID = "", ""
	current.SeatID, successor.SeatID = "", ""
	current.SupersedesRevisionID, successor.SupersedesRevisionID = "", ""
	current.ActivatedAt, successor.ActivatedAt = time.Time{}, time.Time{}
	current.RetiredAt, successor.RetiredAt = time.Time{}, time.Time{}
	current.CapabilityEvidence = sortedCopy(current.CapabilityEvidence)
	successor.CapabilityEvidence = sortedCopy(successor.CapabilityEvidence)
	return reflect.DeepEqual(current, successor)
}

func canonicalPostgresBinding(binding conversation.AgentBindingRevision) conversation.AgentBindingRevision {
	binding.CapabilityEvidence = sortedCopy(binding.CapabilityEvidence)
	return binding
}

func sortedPostgresRebindResources(values []ledger.RebindResource) []ledger.RebindResource {
	result := append([]ledger.RebindResource{}, values...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func insertLifecycleBinding(ctx context.Context, tx transaction, accountID string, binding conversation.AgentBindingRevision) error {
	capabilities, err := encodeCapabilityEvidence(binding)
	if err != nil {
		return err
	}
	workerID, err := bindingWorker(binding)
	if err != nil {
		return err
	}
	_, err = tx.exec(ctx, `insert into fort_private.agent_binding_revision (
  account_id, binding_revision_id, agent_id, revision, behavior_revision_id,
  execution_source_id, source_agent_id, worker_id, seat_id, fort_profile,
  provider, requested_model, resolved_model, adapter_id, adapter_revision,
  source_config_digest, authority_id, authority_revision, policy_id,
  policy_revision, session_behavior, memory_behavior, capability_evidence,
  readiness_contract_id, readiness_contract_revision,
  supersedes_binding_revision_id, activated_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23::jsonb,$24,$25,$26,$27)`,
		accountID, binding.ID, binding.AgentID, binding.Revision, binding.BehaviorRevisionID,
		binding.ExecutionSourceID, binding.SourceAgentID, workerID, binding.SeatID, binding.FortProfile,
		binding.Provider, binding.RequestedModel, binding.ResolvedModel, binding.AdapterID, binding.AdapterRevision,
		binding.SourceConfigDigest, binding.AuthorityID, binding.AuthorityRevision, binding.PolicyID,
		binding.PolicyRevision, binding.SessionBehavior, binding.MemoryBehavior, capabilities,
		binding.ReadinessContractID, binding.ReadinessContractRevision, binding.SupersedesRevisionID,
		binding.ActivatedAt.UTC())
	return err
}

func insertLifecycleParticipant(ctx context.Context, tx transaction, accountID, agentID, behaviorID string, binding conversation.AgentBindingRevision, participant conversation.Participant) error {
	seat, authority, participantDigest, err := participantEvidence(ledger.CreateAgentCommand{
		Binding: binding, Participant: participant,
	})
	if err != nil {
		return err
	}
	_, err = tx.exec(ctx, `insert into fort_private.conversation_participant (
  account_id, participant_id, conversation_id, agent_id,
  behavior_revision_id, binding_revision_id, seat_snapshot,
  authority_snapshot, snapshot_digest, created_at
) values ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9,$10)`, accountID, participant.ID,
		participant.ConversationID, agentID, behaviorID, binding.ID, seat, authority,
		participantDigest, participant.CreatedAt.UTC())
	return err
}

func insertPostgresBindingTransition(ctx context.Context, tx transaction, transition ledger.AgentBindingTransition) error {
	resources, err := json.Marshal(transition.NonTransferableResources)
	if err != nil {
		return err
	}
	readiness, err := json.Marshal(sortedCopy(transition.ReadinessEvidence))
	if err != nil {
		return err
	}
	authority, err := json.Marshal(sortedCopy(transition.AuthorityEvidence))
	if err != nil {
		return err
	}
	_, err = tx.exec(ctx, `insert into fort_private.agent_binding_transition (
  account_id, agent_id, kind, previous_behavior_revision_id,
  successor_behavior_revision_id, previous_binding_revision_id,
  successor_binding_revision_id, preview_digest,
  non_transferable_resources, readiness_evidence, authority_evidence,
  accepted_by, accepted_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10::jsonb,$11::jsonb,$12,$13)`, transition.AccountID,
		transition.AgentID, transition.Kind, transition.PreviousBehaviorRevisionID,
		transition.SuccessorBehaviorRevisionID, transition.PreviousBindingRevisionID,
		transition.SuccessorBindingRevisionID, transition.PreviewDigest, string(resources), string(readiness),
		string(authority), transition.AcceptedBy, transition.AcceptedAt.UTC())
	return err
}

func getAgentBindingTransition(ctx context.Context, tx transaction, accountID, agentID, successorBindingID string) (ledger.AgentBindingTransition, error) {
	var transition ledger.AgentBindingTransition
	var resources, readiness, authority string
	err := tx.queryRow(ctx, `select agent_id, kind, previous_behavior_revision_id,
  successor_behavior_revision_id, previous_binding_revision_id,
  successor_binding_revision_id, preview_digest,
  non_transferable_resources::text, readiness_evidence::text,
  authority_evidence::text, accepted_by, accepted_at
from fort_private.agent_binding_transition
where account_id = $1 and agent_id = $2 and successor_binding_revision_id = $3`, accountID, agentID,
		successorBindingID).scan(&transition.AgentID, &transition.Kind, &transition.PreviousBehaviorRevisionID,
		&transition.SuccessorBehaviorRevisionID, &transition.PreviousBindingRevisionID,
		&transition.SuccessorBindingRevisionID, &transition.PreviewDigest, &resources, &readiness,
		&authority, &transition.AcceptedBy, &transition.AcceptedAt)
	if err != nil {
		return ledger.AgentBindingTransition{}, err
	}
	transition.AccountID = accountID
	if err := json.Unmarshal([]byte(resources), &transition.NonTransferableResources); err != nil {
		return ledger.AgentBindingTransition{}, err
	}
	if err := json.Unmarshal([]byte(readiness), &transition.ReadinessEvidence); err != nil {
		return ledger.AgentBindingTransition{}, err
	}
	if err := json.Unmarshal([]byte(authority), &transition.AuthorityEvidence); err != nil {
		return ledger.AgentBindingTransition{}, err
	}
	return transition, nil
}

func insertBindingAdvanceEvent(ctx context.Context, tx transaction, transition ledger.AgentBindingTransition) error {
	metadata, err := json.Marshal(map[string]any{
		"kind":                           transition.Kind,
		"previous_behavior_revision_id":  transition.PreviousBehaviorRevisionID,
		"successor_behavior_revision_id": transition.SuccessorBehaviorRevisionID,
		"previous_binding_revision_id":   transition.PreviousBindingRevisionID,
		"successor_binding_revision_id":  transition.SuccessorBindingRevisionID,
		"preview_digest":                 transition.PreviewDigest,
		"accepted_by":                    transition.AcceptedBy,
	})
	if err != nil {
		return err
	}
	_, err = tx.exec(ctx, `insert into fort_private.ledger_event (
  account_id, aggregate_kind, aggregate_id, event_type, event_metadata, created_at
) values ($1, 'stable_agent', $2, 'agent.binding.advanced', $3::jsonb, $4)`, transition.AccountID,
		transition.AgentID, string(metadata), transition.AcceptedAt.UTC())
	return err
}

type postgresConversationScanner interface {
	scan(...any) error
}

func getPostgresAgentConversation(ctx context.Context, tx transaction, accountID, agentID, conversationID string) (ledger.AgentConversationRecord, error) {
	record, err := scanPostgresAgentConversation(tx.queryRow(ctx, `select item.conversation_id, item.title, item.state,
  item.created_at, item.updated_at, relation.agent_id, relation.conversation_id,
  relation.kind, relation.created_at, coalesce(pin.pinned, false),
  case when pin.pinned then pin.changed_at else null end
from fort_private.agent_conversation as relation
join fort_private.conversation as item
  on item.account_id = relation.account_id and item.conversation_id = relation.conversation_id
left join lateral (
  select revision.pinned, revision.changed_at
  from fort_private.agent_conversation_pin as revision
  where revision.account_id = relation.account_id and revision.agent_id = relation.agent_id
    and revision.conversation_id = relation.conversation_id
  order by revision.revision desc limit 1
) as pin on true
where relation.account_id = $1 and relation.agent_id = $2 and relation.conversation_id = $3`, accountID, agentID,
		conversationID))
	if isNoRows(err) {
		return ledger.AgentConversationRecord{}, fmt.Errorf("%w: Agent Conversation %q", ledger.ErrNotFound, conversationID)
	}
	return record, err
}

func scanPostgresAgentConversation(scanner postgresConversationScanner) (ledger.AgentConversationRecord, error) {
	var record ledger.AgentConversationRecord
	var state, kind string
	var pinnedAt *time.Time
	err := scanner.scan(&record.Conversation.ID, &record.Conversation.Title, &state,
		&record.Conversation.CreatedAt, &record.Conversation.UpdatedAt, &record.Link.AgentID,
		&record.Link.ConversationID, &kind, &record.Link.CreatedAt, &record.Pinned, &pinnedAt)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	record.Conversation.State = conversation.ConversationState(state)
	record.Link.Kind = conversation.AgentConversationKind(kind)
	if pinnedAt != nil {
		record.PinnedAt = *pinnedAt
	}
	return record, nil
}

func insertAgentConversationEvent(ctx context.Context, tx transaction, accountID, agentID, eventType string, metadata any, createdAt time.Time) error {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.exec(ctx, `insert into fort_private.ledger_event (
  account_id, aggregate_kind, aggregate_id, event_type, event_metadata, created_at
) values ($1, 'stable_agent', $2, $3, $4::jsonb, $5)`, accountID, agentID, eventType,
		string(payload), createdAt.UTC())
	return err
}
