package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

const (
	agentCreateScope = "agent.create"
	agentCreateActor = "fort-control:agent-create"
)

var _ ledger.AgentRepository = (*Store)(nil)

// CreateAgent commits the stable Agent, its current immutable revisions, its
// exact execution-source evidence, and its permanent Home in one transaction.
func (store *Store) CreateAgent(ctx context.Context, command ledger.CreateAgentCommand) (ledger.AgentRecord, error) {
	accountID, err := store.operationAccount(command.Agent.AccountID)
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	sourceAccountID, err := normalizeAccountID(command.ExecutionSource.AccountID)
	if err != nil || sourceAccountID != accountID {
		return ledger.AgentRecord{}, fmt.Errorf("Execution Source belongs to another account")
	}
	command.Agent.AccountID = accountID
	command.ExecutionSource.AccountID = accountID
	command = canonicalAgentCommand(command)
	if command.Home.ProjectID != "" {
		return ledger.AgentRecord{}, fmt.Errorf("cloud Agent Home does not support a local Project id")
	}
	if !command.Binding.RetiredAt.IsZero() {
		return ledger.AgentRecord{}, fmt.Errorf("new Agent Binding Revision cannot already be retired")
	}
	if err := command.Validate(); err != nil {
		return ledger.AgentRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentRecord{}, err
	}

	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimAgentIdempotency(ctx, tx, command, digest)
		if err != nil {
			return err
		}
		if !claimed {
			return nil
		}
		if err := insertAgentAggregate(ctx, tx, command, digest); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	return agentRecord(command), nil
}

func claimAgentIdempotency(ctx context.Context, tx transaction, command ledger.CreateAgentCommand, digest string) (bool, error) {
	affected, err := tx.exec(ctx, `insert into fort_private.idempotency_record (
  account_id, scope, idempotency_key, command_digest, result_kind,
  result_id, response_digest, created_at
) values ($1, $2, $3, $4, 'stable_agent', $5, $4, $6)
on conflict (account_id, scope, idempotency_key) do nothing`,
		command.Agent.AccountID, agentCreateScope, command.IdempotencyKey, digest,
		command.Agent.ID, command.Agent.CreatedAt.UTC())
	if err != nil {
		return false, fmt.Errorf("reserve Agent idempotency key: %w", err)
	}
	if affected == 1 {
		return true, nil
	}
	if affected != 0 {
		return false, fmt.Errorf("reserve Agent idempotency key affected %d rows", affected)
	}

	var existingDigest, resultKind, resultID string
	err = tx.queryRow(ctx, `select command_digest, result_kind, result_id
from fort_private.idempotency_record
where account_id = $1 and scope = $2 and idempotency_key = $3`,
		command.Agent.AccountID, agentCreateScope, command.IdempotencyKey,
	).scan(&existingDigest, &resultKind, &resultID)
	if err != nil {
		return false, fmt.Errorf("read Agent idempotency key: %w", err)
	}
	if existingDigest != digest || resultKind != "stable_agent" || resultID != command.Agent.ID {
		return false, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.IdempotencyKey)
	}
	return false, nil
}

func insertAgentAggregate(ctx context.Context, tx transaction, command ledger.CreateAgentCommand, digest string) error {
	if err := ensureExecutionSource(ctx, tx, command); err != nil {
		return err
	}
	if err := appendPostgresSourceConfigObservation(ctx, tx, "source-observation:create:"+command.Binding.ID,
		command.Agent.AccountID, command.Binding.ExecutionSourceID, command.Binding.SourceConfigDigest,
		agentCreateActor, command.Binding.ActivatedAt); err != nil {
		return err
	}
	if err := ensureSourceAgent(ctx, tx, command); err != nil {
		return err
	}

	membershipID := homeMembershipID(command.Agent.AccountID, command.Agent.ID, command.Home.ID)
	if _, err := tx.exec(ctx, `insert into fort_private.conversation (
  account_id, conversation_id, kind, title, state,
  current_membership_revision_id, created_at, updated_at
) values ($1, $2, 'agent', $3, $4, $5, $6, $7)`,
		command.Agent.AccountID, command.Home.ID, command.Home.Title, command.Home.State,
		membershipID, command.Home.CreatedAt.UTC(), command.Home.UpdatedAt.UTC()); err != nil {
		return fmt.Errorf("insert Agent Home: %w", err)
	}
	if _, err := tx.exec(ctx, `insert into fort_private.stable_agent (
  account_id, agent_id, state, current_profile_revision_id,
  current_behavior_revision_id, current_binding_revision_id,
  canonical_conversation_id, created_at, updated_at
) values ($1, $2, $3, $4, $5, $6, $7, $8, $8)`,
		command.Agent.AccountID, command.Agent.ID, command.Agent.State,
		command.Agent.CurrentProfileRevisionID, command.Agent.CurrentBehaviorRevisionID,
		command.Agent.CurrentBindingRevisionID, command.Agent.CanonicalConversationID,
		command.Agent.CreatedAt.UTC()); err != nil {
		return fmt.Errorf("insert stable Agent: %w", err)
	}
	if _, err := tx.exec(ctx, `insert into fort_private.agent_profile_revision (
  account_id, profile_revision_id, agent_id, revision, name, title,
  avatar_url, hidden, pinned, sort_order, created_by, created_at
) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		command.Agent.AccountID, command.Profile.ID, command.Profile.AgentID,
		command.Profile.Revision, command.Profile.Name, command.Profile.Title,
		command.Profile.AvatarURL, command.Profile.Hidden, command.Profile.Pinned,
		command.Profile.SortOrder, agentCreateActor, command.Profile.CreatedAt.UTC()); err != nil {
		return fmt.Errorf("insert Agent Profile Revision: %w", err)
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
) values ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9, $10, $11, $12)`,
		command.Agent.AccountID, command.Behavior.ID, command.Behavior.AgentID,
		command.Behavior.Revision, command.Behavior.Role, command.Behavior.StandingInstructions,
		string(skills), string(tools), command.Behavior.PromptMaterial, behaviorDigest,
		agentCreateActor, command.Behavior.CreatedAt.UTC()); err != nil {
		return fmt.Errorf("insert Agent Behavior Revision: %w", err)
	}

	capabilityEvidence, err := encodeCapabilityEvidence(command.Binding)
	if err != nil {
		return err
	}
	workerID, _ := bindingWorker(command.Binding)
	if _, err := tx.exec(ctx, `insert into fort_private.agent_binding_revision (
  account_id, binding_revision_id, agent_id, revision, behavior_revision_id,
  execution_source_id, source_agent_id, worker_id, seat_id, fort_profile,
  provider, requested_model, resolved_model, adapter_id, adapter_revision,
  source_config_digest, authority_id, authority_revision, policy_id,
  policy_revision, session_behavior, memory_behavior, capability_evidence,
  readiness_contract_id, readiness_contract_revision,
  supersedes_binding_revision_id, activated_at
) values (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
  $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
  $21, $22, $23::jsonb, $24, $25, nullif($26, ''), $27
)`, command.Agent.AccountID, command.Binding.ID, command.Binding.AgentID,
		command.Binding.Revision, command.Binding.BehaviorRevisionID,
		command.Binding.ExecutionSourceID, command.Binding.SourceAgentID, workerID,
		command.Binding.SeatID, command.Binding.FortProfile, command.Binding.Provider,
		command.Binding.RequestedModel, command.Binding.ResolvedModel,
		command.Binding.AdapterID, command.Binding.AdapterRevision,
		command.Binding.SourceConfigDigest, command.Binding.AuthorityID,
		command.Binding.AuthorityRevision, command.Binding.PolicyID,
		command.Binding.PolicyRevision, command.Binding.SessionBehavior,
		command.Binding.MemoryBehavior, capabilityEvidence,
		command.Binding.ReadinessContractID, command.Binding.ReadinessContractRevision,
		command.Binding.SupersedesRevisionID, command.Binding.ActivatedAt.UTC()); err != nil {
		return fmt.Errorf("insert Agent Binding Revision: %w", err)
	}
	if _, err := tx.exec(ctx, `insert into fort_private.agent_conversation (
  account_id, agent_id, conversation_id, kind, created_at
) values ($1, $2, $3, $4, $5)`, command.Agent.AccountID, command.Link.AgentID,
		command.Link.ConversationID, command.Link.Kind, command.Link.CreatedAt.UTC()); err != nil {
		return fmt.Errorf("insert Agent Conversation link: %w", err)
	}
	if _, err := tx.exec(ctx, `insert into fort_private.conversation_membership_revision (
  account_id, membership_revision_id, conversation_id, revision,
  command_digest, created_by, created_at
) values ($1, $2, $3, 1, $4, $5, $6)`, command.Agent.AccountID, membershipID,
		command.Home.ID, digest, agentCreateActor, command.Home.CreatedAt.UTC()); err != nil {
		return fmt.Errorf("insert Agent Home membership: %w", err)
	}
	if _, err := tx.exec(ctx, `insert into fort_private.conversation_member_revision (
  account_id, membership_revision_id, conversation_id, agent_id,
  position, added_by, created_at
) values ($1, $2, $3, $4, 0, $5, $6)`, command.Agent.AccountID, membershipID,
		command.Home.ID, command.Agent.ID, agentCreateActor, command.Home.CreatedAt.UTC()); err != nil {
		return fmt.Errorf("insert Agent Home member: %w", err)
	}

	seatSnapshot, authoritySnapshot, snapshotDigest, err := participantEvidence(command)
	if err != nil {
		return err
	}
	if _, err := tx.exec(ctx, `insert into fort_private.conversation_participant (
  account_id, participant_id, conversation_id, agent_id,
  behavior_revision_id, binding_revision_id, seat_snapshot,
  authority_snapshot, snapshot_digest, created_at
) values ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9, $10)`,
		command.Agent.AccountID, command.Participant.ID, command.Participant.ConversationID,
		command.Agent.ID, command.Behavior.ID, command.Binding.ID, seatSnapshot,
		authoritySnapshot, snapshotDigest, command.Participant.CreatedAt.UTC()); err != nil {
		return fmt.Errorf("insert Agent Home participant evidence: %w", err)
	}
	return nil
}

func ensureExecutionSource(ctx context.Context, tx transaction, command ledger.CreateAgentCommand) error {
	sharing, err := json.Marshal(command.ExecutionSource.ResourceSharing)
	if err != nil {
		return err
	}
	workerID, err := bindingWorker(command.Binding)
	if err != nil {
		return err
	}
	discoveredAt := command.ExecutionSource.LastSeenAt.UTC()
	affected, err := tx.exec(ctx, `insert into fort_private.execution_source (
  account_id, execution_source_id, worker_id, framework_family, gateway_id,
  instance_id, display_name, resource_sharing, source_config_digest, discovered_at
) values ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10)
on conflict (account_id, execution_source_id) do nothing`, command.Agent.AccountID,
		command.ExecutionSource.ID, workerID, command.ExecutionSource.Framework,
		command.ExecutionSource.GatewayID, command.ExecutionSource.InstanceID,
		command.ExecutionSource.DisplayName, string(sharing), command.Binding.SourceConfigDigest,
		discoveredAt)
	if err != nil {
		return fmt.Errorf("insert Execution Source: %w", err)
	}
	if affected == 1 {
		return nil
	}
	if affected != 0 {
		return fmt.Errorf("insert Execution Source affected %d rows", affected)
	}

	var existing conversation.ExecutionSource
	var existingWorker, existingSharing, existingDigest string
	err = tx.queryRow(ctx, `select execution_source_id, account_id::text, worker_id,
  framework_family, instance_id, gateway_id, display_name,
  resource_sharing::text, source_config_digest, discovered_at
from fort_private.execution_source
where account_id = $1 and execution_source_id = $2`, command.Agent.AccountID,
		command.ExecutionSource.ID).scan(&existing.ID, &existing.AccountID, &existingWorker,
		&existing.Framework, &existing.InstanceID, &existing.GatewayID, &existing.DisplayName,
		&existingSharing, &existingDigest, &existing.LastSeenAt)
	if err != nil {
		return fmt.Errorf("read Execution Source: %w", err)
	}
	if err := json.Unmarshal([]byte(existingSharing), &existing.ResourceSharing); err != nil {
		return fmt.Errorf("decode Execution Source resource sharing: %w", err)
	}
	if !reflect.DeepEqual(existing, command.ExecutionSource) || existingWorker != workerID || existingDigest != command.Binding.SourceConfigDigest {
		return fmt.Errorf("Execution Source %q conflicts with immutable evidence", command.ExecutionSource.ID)
	}
	return nil
}

func ensureSourceAgent(ctx context.Context, tx transaction, command ledger.CreateAgentCommand) error {
	inventoryDigest, err := evidenceDigest(command.SourceAgent)
	if err != nil {
		return err
	}
	discoveredAt := command.SourceAgent.LastSeenAt.UTC()
	affected, err := tx.exec(ctx, `insert into fort_private.source_agent (
  account_id, source_agent_id, execution_source_id, opaque_source_agent_id,
  display_name, inventory_digest, discovered_at
) values ($1, $2, $3, $4, $5, $6, $7)
on conflict (account_id, source_agent_id) do nothing`, command.Agent.AccountID,
		command.SourceAgent.ID, command.SourceAgent.ExecutionSourceID,
		command.SourceAgent.OpaqueSourceAgentID, command.SourceAgent.DisplayName,
		inventoryDigest, discoveredAt)
	if err != nil {
		return fmt.Errorf("insert Source Agent: %w", err)
	}
	if affected == 1 {
		return nil
	}
	if affected != 0 {
		return fmt.Errorf("insert Source Agent affected %d rows", affected)
	}

	var existing conversation.SourceAgent
	var existingDigest string
	err = tx.queryRow(ctx, `select source_agent_id, execution_source_id,
  opaque_source_agent_id, display_name, inventory_digest, discovered_at
from fort_private.source_agent
where account_id = $1 and source_agent_id = $2`, command.Agent.AccountID,
		command.SourceAgent.ID).scan(&existing.ID, &existing.ExecutionSourceID,
		&existing.OpaqueSourceAgentID, &existing.DisplayName, &existingDigest,
		&existing.LastSeenAt)
	if err != nil {
		return fmt.Errorf("read Source Agent: %w", err)
	}
	if !reflect.DeepEqual(existing, command.SourceAgent) || existingDigest != inventoryDigest {
		return fmt.Errorf("Source Agent %q conflicts with immutable evidence", command.SourceAgent.ID)
	}
	return nil
}

type capabilityEvidenceEnvelope struct {
	Values       []string `json:"values"`
	LocationKind string   `json:"location_kind"`
}

func encodeCapabilityEvidence(binding conversation.AgentBindingRevision) (string, error) {
	values := append([]string{}, binding.CapabilityEvidence...)
	sort.Strings(values)
	locationKind := "computer"
	if binding.ComputerID == "" {
		locationKind = "cloud"
	}
	payload, err := json.Marshal(capabilityEvidenceEnvelope{Values: values, LocationKind: locationKind})
	return string(payload), err
}

type participantSeatSnapshot struct {
	SeatID      string                        `json:"seat_id"`
	Profile     string                        `json:"profile"`
	Agent       string                        `json:"agent"`
	Model       string                        `json:"model"`
	Machine     string                        `json:"machine"`
	DisplayName string                        `json:"display_name"`
	Position    int                           `json:"position"`
	State       conversation.ParticipantState `json:"state"`
	RemovedAt   time.Time                     `json:"removed_at,omitempty"`
}

type participantAuthoritySnapshot struct {
	AuthorityID       string `json:"authority_id"`
	AuthorityRevision string `json:"authority_revision"`
	PolicyID          string `json:"policy_id"`
	PolicyRevision    string `json:"policy_revision"`
}

func participantEvidence(command ledger.CreateAgentCommand) (string, string, string, error) {
	seat := participantSeatSnapshot{
		SeatID: command.Participant.SeatID, Profile: command.Participant.Profile,
		Agent: command.Participant.Agent, Model: command.Participant.Model,
		Machine: command.Participant.Machine, DisplayName: command.Participant.DisplayName,
		Position: command.Participant.Position, State: command.Participant.State,
		RemovedAt: command.Participant.RemovedAt,
	}
	authority := participantAuthoritySnapshot{
		AuthorityID: command.Binding.AuthorityID, AuthorityRevision: command.Binding.AuthorityRevision,
		PolicyID: command.Binding.PolicyID, PolicyRevision: command.Binding.PolicyRevision,
	}
	seatJSON, err := json.Marshal(seat)
	if err != nil {
		return "", "", "", err
	}
	authorityJSON, err := json.Marshal(authority)
	if err != nil {
		return "", "", "", err
	}
	digest, err := evidenceDigest(command.Participant)
	if err != nil {
		return "", "", "", err
	}
	return string(seatJSON), string(authorityJSON), digest, nil
}

func bindingWorker(binding conversation.AgentBindingRevision) (string, error) {
	if binding.ComputerID != "" && binding.CloudRuntime == "" {
		return binding.ComputerID, nil
	}
	if binding.CloudRuntime != "" && binding.ComputerID == "" {
		return binding.CloudRuntime, nil
	}
	return "", fmt.Errorf("Agent Binding Revision requires exactly one worker location")
}

func homeMembershipID(accountID, agentID, conversationID string) string {
	digest := sha256.Sum256([]byte(accountID + "\x00" + agentID + "\x00" + conversationID))
	return "membership:agent-home:" + hex.EncodeToString(digest[:])
}

func evidenceDigest(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func canonicalAgentCommand(command ledger.CreateAgentCommand) ledger.CreateAgentCommand {
	command.Behavior.EnabledSkills = sortedCopy(command.Behavior.EnabledSkills)
	command.Behavior.EnabledTools = sortedCopy(command.Behavior.EnabledTools)
	command.Binding.CapabilityEvidence = sortedCopy(command.Binding.CapabilityEvidence)
	return command
}

func sortedCopy(values []string) []string {
	result := append([]string{}, values...)
	sort.Strings(result)
	return result
}

func agentRecord(command ledger.CreateAgentCommand) ledger.AgentRecord {
	return ledger.AgentRecord{
		Agent: command.Agent, Profile: command.Profile, Behavior: command.Behavior,
		Binding: command.Binding, ExecutionSource: command.ExecutionSource,
		SourceAgent: command.SourceAgent, Home: command.Home,
		Participant: command.Participant, Link: command.Link,
	}
}

func getAgentRecord(ctx context.Context, tx transaction, accountID, agentID string) (ledger.AgentRecord, error) {
	var record ledger.AgentRecord
	var agentState, homeState, linkKind string
	var skillsJSON, toolsJSON, behaviorDigest string
	var workerID, capabilityJSON string
	var sharingJSON, sourceConfigDigest string
	var inventoryDigest string
	var seatJSON, authorityJSON, participantDigest string

	err := tx.queryRow(ctx, `select
  agent.agent_id, agent.account_id::text, agent.state,
  agent.current_profile_revision_id, agent.current_behavior_revision_id,
  agent.current_binding_revision_id, agent.canonical_conversation_id,
  agent.created_at,
  profile.profile_revision_id, profile.agent_id, profile.revision,
  profile.name, profile.title, profile.avatar_url, profile.hidden,
  profile.pinned, profile.sort_order, profile.created_at,
  behavior.behavior_revision_id, behavior.agent_id, behavior.revision,
  behavior.role, behavior.standing_instructions, behavior.enabled_skills::text,
  behavior.enabled_tools::text, behavior.prompt_material,
  behavior.behavior_digest, behavior.created_at,
  binding.binding_revision_id, binding.agent_id, binding.revision,
  binding.behavior_revision_id, binding.execution_source_id,
  binding.source_agent_id, binding.worker_id, binding.seat_id,
  binding.fort_profile, binding.provider, binding.requested_model,
  binding.resolved_model, binding.adapter_id, binding.adapter_revision,
  binding.source_config_digest, binding.authority_id,
  binding.authority_revision, binding.policy_id, binding.policy_revision,
  binding.session_behavior, binding.memory_behavior,
  binding.capability_evidence::text, binding.readiness_contract_id,
  binding.readiness_contract_revision,
  coalesce(binding.supersedes_binding_revision_id, ''), binding.activated_at,
  source.execution_source_id, source.account_id::text, source.worker_id,
  source.framework_family, source.instance_id, source.gateway_id,
  source.display_name, source.resource_sharing::text,
  source.source_config_digest, source.discovered_at,
  source_agent.source_agent_id, source_agent.execution_source_id,
  source_agent.opaque_source_agent_id, source_agent.display_name,
  source_agent.inventory_digest, source_agent.discovered_at,
  home.conversation_id, home.title, home.state, home.created_at, home.updated_at,
  participant.participant_id, participant.conversation_id,
  participant.seat_snapshot::text, participant.authority_snapshot::text,
  participant.snapshot_digest, participant.created_at,
  relation.agent_id, relation.conversation_id, relation.kind, relation.created_at
from fort_private.stable_agent as agent
join fort_private.agent_profile_revision as profile
  on profile.account_id = agent.account_id
 and profile.agent_id = agent.agent_id
 and profile.profile_revision_id = agent.current_profile_revision_id
join fort_private.agent_behavior_revision as behavior
  on behavior.account_id = agent.account_id
 and behavior.agent_id = agent.agent_id
 and behavior.behavior_revision_id = agent.current_behavior_revision_id
join fort_private.agent_binding_revision as binding
  on binding.account_id = agent.account_id
 and binding.agent_id = agent.agent_id
 and binding.binding_revision_id = agent.current_binding_revision_id
join fort_private.execution_source as source
  on source.account_id = agent.account_id
 and source.execution_source_id = binding.execution_source_id
join fort_private.source_agent as source_agent
  on source_agent.account_id = agent.account_id
 and source_agent.execution_source_id = source.execution_source_id
 and source_agent.source_agent_id = binding.source_agent_id
join fort_private.conversation as home
  on home.account_id = agent.account_id
 and home.conversation_id = agent.canonical_conversation_id
join fort_private.agent_conversation as relation
  on relation.account_id = agent.account_id
 and relation.agent_id = agent.agent_id
 and relation.conversation_id = home.conversation_id
 and relation.kind = 'canonical'
join fort_private.conversation_participant as participant
  on participant.account_id = agent.account_id
 and participant.conversation_id = home.conversation_id
 and participant.agent_id = agent.agent_id
 and participant.behavior_revision_id = behavior.behavior_revision_id
 and participant.binding_revision_id = binding.binding_revision_id
where agent.account_id = $1 and agent.agent_id = $2`, accountID, agentID).scan(
		&record.Agent.ID, &record.Agent.AccountID, &agentState,
		&record.Agent.CurrentProfileRevisionID, &record.Agent.CurrentBehaviorRevisionID,
		&record.Agent.CurrentBindingRevisionID, &record.Agent.CanonicalConversationID,
		&record.Agent.CreatedAt,
		&record.Profile.ID, &record.Profile.AgentID, &record.Profile.Revision,
		&record.Profile.Name, &record.Profile.Title, &record.Profile.AvatarURL,
		&record.Profile.Hidden, &record.Profile.Pinned, &record.Profile.SortOrder,
		&record.Profile.CreatedAt,
		&record.Behavior.ID, &record.Behavior.AgentID, &record.Behavior.Revision,
		&record.Behavior.Role, &record.Behavior.StandingInstructions, &skillsJSON,
		&toolsJSON, &record.Behavior.PromptMaterial, &behaviorDigest,
		&record.Behavior.CreatedAt,
		&record.Binding.ID, &record.Binding.AgentID, &record.Binding.Revision,
		&record.Binding.BehaviorRevisionID, &record.Binding.ExecutionSourceID,
		&record.Binding.SourceAgentID, &workerID, &record.Binding.SeatID,
		&record.Binding.FortProfile, &record.Binding.Provider,
		&record.Binding.RequestedModel, &record.Binding.ResolvedModel,
		&record.Binding.AdapterID, &record.Binding.AdapterRevision,
		&record.Binding.SourceConfigDigest, &record.Binding.AuthorityID,
		&record.Binding.AuthorityRevision, &record.Binding.PolicyID,
		&record.Binding.PolicyRevision, &record.Binding.SessionBehavior,
		&record.Binding.MemoryBehavior, &capabilityJSON,
		&record.Binding.ReadinessContractID, &record.Binding.ReadinessContractRevision,
		&record.Binding.SupersedesRevisionID, &record.Binding.ActivatedAt,
		&record.ExecutionSource.ID, &record.ExecutionSource.AccountID, &workerID,
		&record.ExecutionSource.Framework, &record.ExecutionSource.InstanceID,
		&record.ExecutionSource.GatewayID, &record.ExecutionSource.DisplayName,
		&sharingJSON, &sourceConfigDigest, &record.ExecutionSource.LastSeenAt,
		&record.SourceAgent.ID, &record.SourceAgent.ExecutionSourceID,
		&record.SourceAgent.OpaqueSourceAgentID, &record.SourceAgent.DisplayName,
		&inventoryDigest, &record.SourceAgent.LastSeenAt,
		&record.Home.ID, &record.Home.Title, &homeState,
		&record.Home.CreatedAt, &record.Home.UpdatedAt,
		&record.Participant.ID, &record.Participant.ConversationID,
		&seatJSON, &authorityJSON, &participantDigest, &record.Participant.CreatedAt,
		&record.Link.AgentID, &record.Link.ConversationID, &linkKind,
		&record.Link.CreatedAt,
	)
	if err != nil {
		if isNoRows(err) {
			return ledger.AgentRecord{}, fmt.Errorf("%w: Agent %q", ledger.ErrNotFound, agentID)
		}
		return ledger.AgentRecord{}, fmt.Errorf("read stable Agent: %w", err)
	}

	record.Agent.State = conversation.AgentState(agentState)
	record.Home.State = conversation.ConversationState(homeState)
	record.Link.Kind = conversation.AgentConversationKind(linkKind)
	if err := json.Unmarshal([]byte(skillsJSON), &record.Behavior.EnabledSkills); err != nil {
		return ledger.AgentRecord{}, fmt.Errorf("decode Agent skills: %w", err)
	}
	if err := json.Unmarshal([]byte(toolsJSON), &record.Behavior.EnabledTools); err != nil {
		return ledger.AgentRecord{}, fmt.Errorf("decode Agent tools: %w", err)
	}
	var capabilities capabilityEvidenceEnvelope
	if err := json.Unmarshal([]byte(capabilityJSON), &capabilities); err != nil {
		return ledger.AgentRecord{}, fmt.Errorf("decode Agent capabilities: %w", err)
	}
	record.Binding.CapabilityEvidence = capabilities.Values
	switch capabilities.LocationKind {
	case "computer":
		record.Binding.ComputerID = workerID
	case "cloud":
		record.Binding.CloudRuntime = workerID
	default:
		return ledger.AgentRecord{}, fmt.Errorf("Agent Binding Revision has unknown location kind %q", capabilities.LocationKind)
	}
	if err := json.Unmarshal([]byte(sharingJSON), &record.ExecutionSource.ResourceSharing); err != nil {
		return ledger.AgentRecord{}, fmt.Errorf("decode Execution Source resource sharing: %w", err)
	}
	var seat participantSeatSnapshot
	if err := json.Unmarshal([]byte(seatJSON), &seat); err != nil {
		return ledger.AgentRecord{}, fmt.Errorf("decode Agent participant seat: %w", err)
	}
	record.Participant.SeatID = seat.SeatID
	record.Participant.Profile = seat.Profile
	record.Participant.Agent = seat.Agent
	record.Participant.Model = seat.Model
	record.Participant.Machine = seat.Machine
	record.Participant.DisplayName = seat.DisplayName
	record.Participant.Position = seat.Position
	record.Participant.State = seat.State
	record.Participant.RemovedAt = seat.RemovedAt

	var authority participantAuthoritySnapshot
	if err := json.Unmarshal([]byte(authorityJSON), &authority); err != nil {
		return ledger.AgentRecord{}, fmt.Errorf("decode Agent participant authority: %w", err)
	}
	if authority.AuthorityID != record.Binding.AuthorityID ||
		authority.AuthorityRevision != record.Binding.AuthorityRevision ||
		authority.PolicyID != record.Binding.PolicyID ||
		authority.PolicyRevision != record.Binding.PolicyRevision {
		return ledger.AgentRecord{}, fmt.Errorf("Agent participant authority evidence conflicts with Binding Revision")
	}
	if sourceConfigDigest != record.Binding.SourceConfigDigest {
		return ledger.AgentRecord{}, fmt.Errorf("Execution Source configuration conflicts with Binding Revision")
	}
	if digest, digestErr := evidenceDigest(record.Behavior); digestErr != nil || digest != behaviorDigest {
		return ledger.AgentRecord{}, fmt.Errorf("Agent Behavior Revision digest mismatch")
	}
	if digest, digestErr := evidenceDigest(record.SourceAgent); digestErr != nil || digest != inventoryDigest {
		return ledger.AgentRecord{}, fmt.Errorf("Source Agent inventory digest mismatch")
	}
	if digest, digestErr := evidenceDigest(record.Participant); digestErr != nil || digest != participantDigest {
		return ledger.AgentRecord{}, fmt.Errorf("Agent participant snapshot digest mismatch")
	}
	validation := ledger.CreateAgentCommand{
		IdempotencyKey: "read-agent", Agent: record.Agent, Profile: record.Profile,
		Behavior: record.Behavior, Binding: record.Binding,
		ExecutionSource: record.ExecutionSource, SourceAgent: record.SourceAgent,
		Home: record.Home, Participant: record.Participant, Link: record.Link,
	}
	// Participant display name is immutable evidence from Binding activation;
	// later presentation-only Profile Revisions must not rewrite it.
	validation.Profile.Name = record.Participant.DisplayName
	if err := validation.Validate(); err != nil {
		return ledger.AgentRecord{}, fmt.Errorf("invalid persisted Agent evidence: %w", err)
	}
	return record, nil
}
