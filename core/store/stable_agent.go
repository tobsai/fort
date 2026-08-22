package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

// stableAgentSchema is intentionally additive. The v1 agent_channel tables
// remain byte-for-byte compatible for rollback; stable_agent is the v2 Fort
// identity that survives binding changes.
const stableAgentSchema = `
CREATE TABLE IF NOT EXISTS execution_source (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  framework TEXT NOT NULL,
  instance_id TEXT NOT NULL,
  gateway_id TEXT NOT NULL,
  display_name TEXT NOT NULL,
  resource_sharing_json TEXT NOT NULL CHECK(json_valid(resource_sharing_json)),
  last_seen_at TEXT,
  UNIQUE(account_id,framework,instance_id,gateway_id)
);
CREATE TABLE IF NOT EXISTS source_agent (
  id TEXT PRIMARY KEY,
  execution_source_id TEXT NOT NULL,
  opaque_source_agent_id TEXT NOT NULL,
  display_name TEXT NOT NULL,
  last_seen_at TEXT,
  FOREIGN KEY(execution_source_id) REFERENCES execution_source(id),
  UNIQUE(execution_source_id,opaque_source_agent_id),
  UNIQUE(id,execution_source_id)
);
CREATE TABLE IF NOT EXISTS stable_agent (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  state TEXT NOT NULL CHECK(state IN ('open','archived')),
  current_profile_revision_id TEXT NOT NULL,
  current_behavior_revision_id TEXT NOT NULL,
  current_binding_revision_id TEXT NOT NULL,
  canonical_conversation_id TEXT NOT NULL UNIQUE,
  canonical_participant_id TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  FOREIGN KEY(canonical_conversation_id) REFERENCES conversation(id),
  FOREIGN KEY(canonical_participant_id) REFERENCES conversation_participant(id)
);
CREATE INDEX IF NOT EXISTS idx_stable_agent_account_state
  ON stable_agent(account_id,state,created_at,id);
CREATE TABLE IF NOT EXISTS agent_profile_revision (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  revision INTEGER NOT NULL CHECK(revision>0),
  name TEXT NOT NULL,
  title TEXT NOT NULL,
  avatar_url TEXT NOT NULL,
  hidden INTEGER NOT NULL CHECK(hidden IN (0,1)),
  pinned INTEGER NOT NULL CHECK(pinned IN (0,1)),
  sort_order INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(agent_id) REFERENCES stable_agent(id),
  UNIQUE(agent_id,revision),
  UNIQUE(id,agent_id)
);
CREATE TABLE IF NOT EXISTS agent_behavior_revision (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  revision INTEGER NOT NULL CHECK(revision>0),
  role TEXT NOT NULL,
  standing_instructions TEXT NOT NULL,
  enabled_skills_json TEXT NOT NULL CHECK(json_valid(enabled_skills_json) AND json_type(enabled_skills_json)='array'),
  enabled_tools_json TEXT NOT NULL CHECK(json_valid(enabled_tools_json) AND json_type(enabled_tools_json)='array'),
  prompt_material TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(agent_id) REFERENCES stable_agent(id),
  UNIQUE(agent_id,revision),
  UNIQUE(id,agent_id)
);
CREATE TABLE IF NOT EXISTS agent_binding_revision (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  revision INTEGER NOT NULL CHECK(revision>0),
  behavior_revision_id TEXT NOT NULL,
  execution_source_id TEXT NOT NULL,
  source_agent_id TEXT NOT NULL,
  seat_id TEXT NOT NULL,
  fort_profile TEXT NOT NULL,
  provider TEXT NOT NULL,
  requested_model TEXT NOT NULL,
  resolved_model TEXT NOT NULL,
  computer_id TEXT NOT NULL,
  cloud_runtime TEXT NOT NULL,
  adapter_id TEXT NOT NULL,
  adapter_revision TEXT NOT NULL,
  source_config_digest TEXT NOT NULL,
  authority_id TEXT NOT NULL,
  authority_revision TEXT NOT NULL,
  policy_id TEXT NOT NULL,
  policy_revision TEXT NOT NULL,
  session_behavior TEXT NOT NULL,
  memory_behavior TEXT NOT NULL,
  capability_evidence_json TEXT NOT NULL CHECK(json_valid(capability_evidence_json) AND json_type(capability_evidence_json)='array'),
  readiness_contract_id TEXT NOT NULL,
  readiness_contract_revision TEXT NOT NULL,
  supersedes_revision_id TEXT NOT NULL,
  activated_at TEXT NOT NULL,
  retired_at TEXT,
  FOREIGN KEY(agent_id) REFERENCES stable_agent(id),
  FOREIGN KEY(behavior_revision_id,agent_id) REFERENCES agent_behavior_revision(id,agent_id),
  FOREIGN KEY(execution_source_id) REFERENCES execution_source(id),
  FOREIGN KEY(source_agent_id,execution_source_id) REFERENCES source_agent(id,execution_source_id),
  UNIQUE(agent_id,revision),
  UNIQUE(id,agent_id)
);
CREATE TABLE IF NOT EXISTS agent_conversation (
  agent_id TEXT NOT NULL,
  conversation_id TEXT NOT NULL UNIQUE,
  kind TEXT NOT NULL CHECK(kind IN ('canonical','secondary')),
  created_at TEXT NOT NULL,
  PRIMARY KEY(agent_id,conversation_id),
  FOREIGN KEY(agent_id) REFERENCES stable_agent(id),
  FOREIGN KEY(conversation_id) REFERENCES conversation(id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_conversation_one_canonical
  ON agent_conversation(agent_id) WHERE kind='canonical';
CREATE TABLE IF NOT EXISTS stable_agent_participant_evidence (
  agent_id TEXT NOT NULL,
  binding_revision_id TEXT NOT NULL,
  conversation_id TEXT NOT NULL,
  participant_id TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  PRIMARY KEY(agent_id,binding_revision_id,conversation_id),
  FOREIGN KEY(agent_id) REFERENCES stable_agent(id),
  FOREIGN KEY(binding_revision_id,agent_id) REFERENCES agent_binding_revision(id,agent_id),
  FOREIGN KEY(conversation_id) REFERENCES conversation(id),
  FOREIGN KEY(participant_id) REFERENCES conversation_participant(id)
);
CREATE TABLE IF NOT EXISTS stable_agent_create_idempotency (
  account_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  command_digest TEXT NOT NULL CHECK(length(command_digest)=64 AND command_digest NOT GLOB '*[^0-9a-f]*'),
  agent_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY(account_id,idempotency_key),
  FOREIGN KEY(agent_id) REFERENCES stable_agent(id)
);
CREATE TABLE IF NOT EXISTS stable_agent_lifecycle_idempotency (
  account_id TEXT NOT NULL,
  scope TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  command_digest TEXT NOT NULL CHECK(length(command_digest)=64 AND command_digest NOT GLOB '*[^0-9a-f]*'),
  result_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY(account_id,scope,idempotency_key)
);
CREATE TABLE IF NOT EXISTS agent_profile_revision_acceptance (
  profile_revision_id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  accepted_by TEXT NOT NULL,
  accepted_at TEXT NOT NULL,
  FOREIGN KEY(profile_revision_id,agent_id) REFERENCES agent_profile_revision(id,agent_id)
);
CREATE TABLE IF NOT EXISTS agent_binding_transition (
  agent_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK(kind IN ('behavior','rebind')),
  previous_behavior_revision_id TEXT NOT NULL,
  successor_behavior_revision_id TEXT NOT NULL,
  previous_binding_revision_id TEXT NOT NULL,
  successor_binding_revision_id TEXT NOT NULL,
  preview_digest TEXT NOT NULL CHECK(length(preview_digest)=64 AND preview_digest NOT GLOB '*[^0-9a-f]*'),
  non_transferable_resources_json TEXT NOT NULL CHECK(json_valid(non_transferable_resources_json) AND json_type(non_transferable_resources_json)='array'),
  readiness_evidence_json TEXT NOT NULL CHECK(json_valid(readiness_evidence_json) AND json_type(readiness_evidence_json)='array'),
  authority_evidence_json TEXT NOT NULL CHECK(json_valid(authority_evidence_json) AND json_type(authority_evidence_json)='array'),
  accepted_by TEXT NOT NULL,
  accepted_at TEXT NOT NULL,
  PRIMARY KEY(agent_id,successor_binding_revision_id),
  UNIQUE(agent_id,previous_binding_revision_id),
  FOREIGN KEY(previous_behavior_revision_id,agent_id) REFERENCES agent_behavior_revision(id,agent_id),
  FOREIGN KEY(successor_behavior_revision_id,agent_id) REFERENCES agent_behavior_revision(id,agent_id),
  FOREIGN KEY(previous_binding_revision_id,agent_id) REFERENCES agent_binding_revision(id,agent_id),
  FOREIGN KEY(successor_binding_revision_id,agent_id) REFERENCES agent_binding_revision(id,agent_id)
);
CREATE TABLE IF NOT EXISTS stable_agent_lifecycle_event (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account_id TEXT NOT NULL,
  agent_id TEXT NOT NULL,
  type TEXT NOT NULL,
  metadata_json TEXT NOT NULL CHECK(json_valid(metadata_json) AND json_type(metadata_json)='object'),
  created_at TEXT NOT NULL,
  FOREIGN KEY(agent_id) REFERENCES stable_agent(id)
);
CREATE TABLE IF NOT EXISTS agent_conversation_pin_revision (
  account_id TEXT NOT NULL,
  agent_id TEXT NOT NULL,
  conversation_id TEXT NOT NULL,
  revision INTEGER NOT NULL CHECK(revision>0),
  pinned INTEGER NOT NULL CHECK(pinned IN (0,1)),
  changed_by TEXT NOT NULL,
  changed_at TEXT NOT NULL,
  PRIMARY KEY(account_id,agent_id,conversation_id,revision),
  FOREIGN KEY(agent_id,conversation_id) REFERENCES agent_conversation(agent_id,conversation_id)
);
CREATE INDEX IF NOT EXISTS idx_stable_agent_lifecycle_event
  ON stable_agent_lifecycle_event(account_id,agent_id,id);

CREATE TRIGGER IF NOT EXISTS execution_source_immutable_update
BEFORE UPDATE ON execution_source BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS execution_source_immutable_delete
BEFORE DELETE ON execution_source BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS source_agent_immutable_update
BEFORE UPDATE ON source_agent BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS source_agent_immutable_delete
BEFORE DELETE ON source_agent BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS agent_profile_revision_immutable_update
BEFORE UPDATE ON agent_profile_revision BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS agent_profile_revision_immutable_delete
BEFORE DELETE ON agent_profile_revision BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS agent_behavior_revision_immutable_update
BEFORE UPDATE ON agent_behavior_revision BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS agent_behavior_revision_immutable_delete
BEFORE DELETE ON agent_behavior_revision BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS agent_binding_revision_immutable_update
BEFORE UPDATE ON agent_binding_revision BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS agent_binding_revision_immutable_delete
BEFORE DELETE ON agent_binding_revision BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS agent_conversation_immutable_update
BEFORE UPDATE ON agent_conversation BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS agent_conversation_immutable_delete
BEFORE DELETE ON agent_conversation BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_insert_home_invariant
BEFORE INSERT ON stable_agent
WHEN NOT EXISTS (
  SELECT 1 FROM conversation home
  JOIN conversation_participant participant ON participant.conversation_id=home.id
  WHERE home.id=NEW.canonical_conversation_id
    AND home.state='open'
    AND participant.id=NEW.canonical_participant_id
    AND participant.state='active'
)
BEGIN SELECT RAISE(ABORT,'stable_agent_home_invariant'); END;
CREATE TRIGGER IF NOT EXISTS agent_conversation_insert_home_invariant
BEFORE INSERT ON agent_conversation
WHEN (NEW.kind='canonical' AND NOT EXISTS (
  SELECT 1 FROM stable_agent
  WHERE id=NEW.agent_id AND canonical_conversation_id=NEW.conversation_id
)) OR (NEW.kind='secondary' AND EXISTS (
  SELECT 1 FROM stable_agent
  WHERE id=NEW.agent_id AND canonical_conversation_id=NEW.conversation_id
))
BEGIN SELECT RAISE(ABORT,'stable_agent_home_invariant'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_participant_evidence_immutable_update
BEFORE UPDATE ON stable_agent_participant_evidence BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_participant_evidence_immutable_delete
BEFORE DELETE ON stable_agent_participant_evidence BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_participant_update_immutable
BEFORE UPDATE ON conversation_participant
WHEN EXISTS (SELECT 1 FROM stable_agent_participant_evidence WHERE participant_id=OLD.id OR participant_id=NEW.id)
BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_participant_delete_immutable
BEFORE DELETE ON conversation_participant
WHEN EXISTS (SELECT 1 FROM stable_agent_participant_evidence WHERE participant_id=OLD.id)
BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_identity_immutable
BEFORE UPDATE OF id,account_id,canonical_conversation_id,canonical_participant_id,created_at ON stable_agent
BEGIN SELECT RAISE(ABORT,'stable_agent_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_delete_immutable
BEFORE DELETE ON stable_agent BEGIN SELECT RAISE(ABORT,'stable_agent_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_home_must_remain_open
BEFORE UPDATE OF state ON conversation
WHEN NEW.state<>'open' AND EXISTS (
  SELECT 1 FROM stable_agent WHERE canonical_conversation_id=OLD.id AND state='open'
)
BEGIN SELECT RAISE(ABORT,'stable_agent_home_invariant'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_home_delete_immutable
BEFORE DELETE ON conversation
WHEN EXISTS (SELECT 1 FROM stable_agent WHERE canonical_conversation_id=OLD.id)
BEGIN SELECT RAISE(ABORT,'stable_agent_home_invariant'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_create_idempotency_immutable_update
BEFORE UPDATE ON stable_agent_create_idempotency BEGIN SELECT RAISE(ABORT,'stable_agent_idempotency_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_create_idempotency_immutable_delete
BEFORE DELETE ON stable_agent_create_idempotency BEGIN SELECT RAISE(ABORT,'stable_agent_idempotency_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_lifecycle_idempotency_immutable_update
BEFORE UPDATE ON stable_agent_lifecycle_idempotency BEGIN SELECT RAISE(ABORT,'stable_agent_idempotency_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_lifecycle_idempotency_immutable_delete
BEFORE DELETE ON stable_agent_lifecycle_idempotency BEGIN SELECT RAISE(ABORT,'stable_agent_idempotency_immutable'); END;
CREATE TRIGGER IF NOT EXISTS agent_profile_revision_acceptance_immutable_update
BEFORE UPDATE ON agent_profile_revision_acceptance BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS agent_profile_revision_acceptance_immutable_delete
BEFORE DELETE ON agent_profile_revision_acceptance BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS agent_binding_transition_immutable_update
BEFORE UPDATE ON agent_binding_transition BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS agent_binding_transition_immutable_delete
BEFORE DELETE ON agent_binding_transition BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_lifecycle_event_immutable_update
BEFORE UPDATE ON stable_agent_lifecycle_event BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_agent_lifecycle_event_immutable_delete
BEFORE DELETE ON stable_agent_lifecycle_event BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS agent_conversation_pin_revision_immutable_update
BEFORE UPDATE ON agent_conversation_pin_revision BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
CREATE TRIGGER IF NOT EXISTS agent_conversation_pin_revision_immutable_delete
BEFORE DELETE ON agent_conversation_pin_revision BEGIN SELECT RAISE(ABORT,'stable_agent_evidence_immutable'); END;
`

func (s *Store) migrateStableAgents() error {
	if _, err := s.db.Exec(stableAgentSchema); err != nil {
		return err
	}
	if _, err := s.db.Exec(agentDirectChatSchema); err != nil {
		return err
	}
	// These columns are additive for local ledgers created while the direct
	// chat contract was still under development. Historical observations remain
	// readable; all new observations carry explicit identity and actor evidence.
	if err := s.addColumn("execution_source_config_observation", "observation_id", "TEXT"); err != nil {
		return err
	}
	if err := s.addColumn("execution_source_config_observation", "observed_by", "TEXT"); err != nil {
		return err
	}
	_, err := s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_execution_source_config_observation_id
ON execution_source_config_observation(account_id,observation_id) WHERE observation_id IS NOT NULL`)
	return err
}

var _ ledger.AgentRepository = (*Store)(nil)
var _ ledger.AgentLifecycleRepository = (*Store)(nil)

func (s *Store) CreateAgent(ctx context.Context, command ledger.CreateAgentCommand) (ledger.AgentRecord, error) {
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	command = canonicalAgentCommand(command)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	defer tx.Rollback()

	var existingDigest, existingAgentID string
	err = tx.QueryRowContext(ctx, `SELECT command_digest,agent_id FROM stable_agent_create_idempotency
WHERE account_id=? AND idempotency_key=?`, command.Agent.AccountID, command.IdempotencyKey).Scan(&existingDigest, &existingAgentID)
	if err == nil {
		if existingDigest != digest {
			return ledger.AgentRecord{}, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.IdempotencyKey)
		}
		return getAgentRecord(ctx, tx, command.Agent.AccountID, existingAgentID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ledger.AgentRecord{}, err
	}
	if err := command.Validate(); err != nil {
		return ledger.AgentRecord{}, err
	}

	if err := ensureExecutionSource(ctx, tx, command.ExecutionSource); err != nil {
		return ledger.AgentRecord{}, err
	}
	if err := appendSQLiteSourceConfiguration(ctx, tx, "source-observation:create:"+command.Binding.ID,
		command.Agent.AccountID, command.Binding.ExecutionSourceID, command.Binding.SourceConfigDigest,
		"fort-control:agent-create", command.Binding.ActivatedAt); err != nil {
		return ledger.AgentRecord{}, err
	}
	if err := ensureSourceAgent(ctx, tx, command.SourceAgent); err != nil {
		return ledger.AgentRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO conversation(id,project_id,title,state,created_at,updated_at)
VALUES(?,?,?,?,?,?)`, command.Home.ID, nullableString(command.Home.ProjectID), command.Home.Title, command.Home.State,
		nowOr(command.Home.CreatedAt), nowOr(command.Home.UpdatedAt)); err != nil {
		return ledger.AgentRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO conversation_participant(
id,conversation_id,seat_id,profile,agent,model,machine,display_name,position,state,created_at,removed_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,NULL)`, command.Participant.ID, command.Participant.ConversationID,
		command.Participant.SeatID, command.Participant.Profile, command.Participant.Agent,
		nullableString(command.Participant.Model), command.Participant.Machine, command.Participant.DisplayName,
		command.Participant.Position, command.Participant.State, nowOr(command.Participant.CreatedAt)); err != nil {
		return ledger.AgentRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_agent(
id,account_id,state,current_profile_revision_id,current_behavior_revision_id,current_binding_revision_id,
canonical_conversation_id,canonical_participant_id,created_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		command.Agent.ID, command.Agent.AccountID, command.Agent.State, command.Agent.CurrentProfileRevisionID,
		command.Agent.CurrentBehaviorRevisionID, command.Agent.CurrentBindingRevisionID,
		command.Agent.CanonicalConversationID, command.Participant.ID, nowOr(command.Agent.CreatedAt)); err != nil {
		return ledger.AgentRecord{}, err
	}
	if err := insertProfileRevision(ctx, tx, command.Profile); err != nil {
		return ledger.AgentRecord{}, err
	}
	if err := insertBehaviorRevision(ctx, tx, command.Behavior); err != nil {
		return ledger.AgentRecord{}, err
	}
	if err := insertBindingRevision(ctx, tx, command.Binding); err != nil {
		return ledger.AgentRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO agent_conversation(agent_id,conversation_id,kind,created_at)
VALUES(?,?,?,?)`, command.Link.AgentID, command.Link.ConversationID, command.Link.Kind, nowOr(command.Link.CreatedAt)); err != nil {
		return ledger.AgentRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_agent_participant_evidence(
agent_id,binding_revision_id,conversation_id,participant_id,created_at) VALUES(?,?,?,?,?)`,
		command.Agent.ID, command.Binding.ID, command.Home.ID, command.Participant.ID, nowOr(command.Participant.CreatedAt)); err != nil {
		return ledger.AgentRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_agent_create_idempotency(
account_id,idempotency_key,command_digest,agent_id,created_at) VALUES(?,?,?,?,?)`,
		command.Agent.AccountID, command.IdempotencyKey, digest, command.Agent.ID, nowOr(command.Agent.CreatedAt)); err != nil {
		return ledger.AgentRecord{}, err
	}
	record, err := getAgentRecord(ctx, tx, command.Agent.AccountID, command.Agent.ID)
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.AgentRecord{}, err
	}
	return record, nil
}

func (s *Store) AppendAgentProfile(ctx context.Context, command ledger.AppendAgentProfileCommand) (ledger.AgentRecord, error) {
	if err := command.Validate(); err != nil {
		return ledger.AgentRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	defer tx.Rollback()

	claimed, err := claimAgentLifecycleIdempotency(ctx, tx, command.AccountID, "agent.profile.append",
		command.IdempotencyKey, digest, command.Revision.ID, command.Revision.CreatedAt)
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	if !claimed {
		return getAgentRecord(ctx, tx, command.AccountID, command.AgentID)
	}

	var currentID string
	var currentRevision int
	err = tx.QueryRowContext(ctx, `SELECT agent.current_profile_revision_id,profile.revision
FROM stable_agent agent JOIN agent_profile_revision profile
  ON profile.id=agent.current_profile_revision_id AND profile.agent_id=agent.id
WHERE agent.account_id=? AND agent.id=?`, command.AccountID, command.AgentID).Scan(&currentID, &currentRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.AgentRecord{}, fmt.Errorf("%w: Agent %q", ledger.ErrNotFound, command.AgentID)
	}
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	if currentID != command.ExpectedProfileRevisionID || command.Revision.Revision != currentRevision+1 {
		return ledger.AgentRecord{}, fmt.Errorf("%w: Agent Profile Revision", ledger.ErrRevisionConflict)
	}
	if err := insertProfileRevision(ctx, tx, command.Revision); err != nil {
		return ledger.AgentRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO agent_profile_revision_acceptance(
profile_revision_id,agent_id,accepted_by,accepted_at) VALUES(?,?,?,?)`, command.Revision.ID,
		command.AgentID, command.AcceptedBy, nowOr(command.Revision.CreatedAt)); err != nil {
		return ledger.AgentRecord{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE stable_agent SET current_profile_revision_id=?
WHERE account_id=? AND id=? AND current_profile_revision_id=?`, command.Revision.ID, command.AccountID,
		command.AgentID, command.ExpectedProfileRevisionID)
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	if changed != 1 {
		return ledger.AgentRecord{}, fmt.Errorf("%w: Agent Profile Revision", ledger.ErrRevisionConflict)
	}
	record, err := getAgentRecord(ctx, tx, command.AccountID, command.AgentID)
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.AgentRecord{}, err
	}
	return record, nil
}

func (s *Store) AppendAgentBehavior(ctx context.Context, command ledger.AppendAgentBehaviorCommand) (ledger.AgentBindingAdvanceResult, error) {
	if err := command.Validate(); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	command.Behavior.EnabledSkills = sortedStrings(command.Behavior.EnabledSkills)
	command.Behavior.EnabledTools = sortedStrings(command.Behavior.EnabledTools)
	command.Binding.CapabilityEvidence = sortedStrings(command.Binding.CapabilityEvidence)
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	defer tx.Rollback()
	claimed, err := claimAgentLifecycleIdempotency(ctx, tx, command.AccountID, "agent.behavior.append",
		command.IdempotencyKey, digest, command.Binding.ID, command.AcceptedAt)
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	if !claimed {
		record, err := getAgentRecord(ctx, tx, command.AccountID, command.AgentID)
		if err != nil {
			return ledger.AgentBindingAdvanceResult{}, err
		}
		transition, err := getBindingTransition(ctx, tx, command.AccountID, command.AgentID, command.Binding.ID)
		return ledger.AgentBindingAdvanceResult{Agent: record, Transition: transition}, err
	}

	current, err := getAgentRecord(ctx, tx, command.AccountID, command.AgentID)
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	if current.Agent.State != conversation.AgentOpen {
		return ledger.AgentBindingAdvanceResult{}, fmt.Errorf("%w: archived Agent", ledger.ErrStateConflict)
	}
	if current.Behavior.ID != command.ExpectedBehaviorRevisionID || current.Binding.ID != command.ExpectedBindingRevisionID ||
		command.Behavior.Revision != current.Behavior.Revision+1 || command.Binding.Revision != current.Binding.Revision+1 {
		return ledger.AgentBindingAdvanceResult{}, fmt.Errorf("%w: Agent Behavior or Binding Revision", ledger.ErrRevisionConflict)
	}
	if !sameBindingExecution(current.Binding, command.Binding) {
		return ledger.AgentBindingAdvanceResult{}, fmt.Errorf("Behavior acceptance cannot change execution identity; use explicit Rebind")
	}
	if command.Participant.ConversationID != current.Home.ID || command.Participant.DisplayName != current.Profile.Name {
		return ledger.AgentBindingAdvanceResult{}, fmt.Errorf("Behavior participant evidence does not match Agent Home and profile")
	}
	if err := insertBehaviorRevision(ctx, tx, command.Behavior); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	if err := insertBindingRevision(ctx, tx, command.Binding); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	if err := insertSuccessorParticipant(ctx, tx, command.AgentID, command.Binding.ID, command.Participant); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	transition := ledger.AgentBindingTransition{
		AccountID: command.AccountID, AgentID: command.AgentID, Kind: ledger.BindingTransitionBehavior,
		PreviousBehaviorRevisionID: current.Behavior.ID, SuccessorBehaviorRevisionID: command.Behavior.ID,
		PreviousBindingRevisionID: current.Binding.ID, SuccessorBindingRevisionID: command.Binding.ID,
		PreviewDigest: digest, NonTransferableResources: []ledger.RebindResource{},
		ReadinessEvidence: sortedStrings(command.ReadinessEvidence), AuthorityEvidence: sortedStrings(command.AuthorityEvidence),
		AcceptedBy: command.AcceptedBy, AcceptedAt: command.AcceptedAt,
	}
	if err := insertBindingTransition(ctx, tx, transition); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	if err := insertSQLiteBindingAdvanceEvent(ctx, tx, transition); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE stable_agent
SET current_behavior_revision_id=?,current_binding_revision_id=?
WHERE account_id=? AND id=? AND current_behavior_revision_id=? AND current_binding_revision_id=?`,
		command.Behavior.ID, command.Binding.ID, command.AccountID, command.AgentID,
		command.ExpectedBehaviorRevisionID, command.ExpectedBindingRevisionID)
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	if changed != 1 {
		return ledger.AgentBindingAdvanceResult{}, fmt.Errorf("%w: Agent Behavior or Binding Revision", ledger.ErrRevisionConflict)
	}
	record, err := getAgentRecord(ctx, tx, command.AccountID, command.AgentID)
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	return ledger.AgentBindingAdvanceResult{Agent: record, Transition: transition}, nil
}

func (s *Store) PreviewAgentRebind(ctx context.Context, command ledger.PreviewAgentRebindCommand) (ledger.AgentRebindPreview, error) {
	if err := command.Validate(); err != nil {
		return ledger.AgentRebindPreview{}, err
	}
	current, err := s.GetAgent(ctx, command.AccountID, command.AgentID)
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
		AccountID: command.AccountID, AgentID: command.AgentID,
		CurrentBinding: current.Binding, CurrentExecutionSource: current.ExecutionSource, CurrentSourceAgent: current.SourceAgent,
		ProposedBinding: command.Binding, ProposedExecutionSource: command.ExecutionSource, ProposedSourceAgent: command.SourceAgent,
		Participant: command.Participant, NonTransferableResources: sortedRebindResources(command.NonTransferableResources),
		ReadinessEvidence: sortedStrings(command.ReadinessEvidence), AuthorityEvidence: sortedStrings(command.AuthorityEvidence),
		GeneratedAt: command.GeneratedAt,
	}
	preview.Digest, err = preview.CalculateDigest()
	if err != nil {
		return ledger.AgentRebindPreview{}, err
	}
	return preview, preview.Validate()
}

func (s *Store) AcceptAgentRebind(ctx context.Context, command ledger.AcceptAgentRebindCommand) (ledger.AgentBindingAdvanceResult, error) {
	if err := command.Validate(); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	defer tx.Rollback()
	claimed, err := claimAgentLifecycleIdempotency(ctx, tx, command.Preview.AccountID, "agent.rebind.accept",
		command.IdempotencyKey, digest, command.Preview.ProposedBinding.ID, command.AcceptedAt)
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	if !claimed {
		record, err := getAgentRecord(ctx, tx, command.Preview.AccountID, command.Preview.AgentID)
		if err != nil {
			return ledger.AgentBindingAdvanceResult{}, err
		}
		transition, err := getBindingTransition(ctx, tx, command.Preview.AccountID, command.Preview.AgentID,
			command.Preview.ProposedBinding.ID)
		return ledger.AgentBindingAdvanceResult{Agent: record, Transition: transition}, err
	}
	current, err := getAgentRecord(ctx, tx, command.Preview.AccountID, command.Preview.AgentID)
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	if current.Agent.State != conversation.AgentOpen {
		return ledger.AgentBindingAdvanceResult{}, fmt.Errorf("%w: archived Agent", ledger.ErrStateConflict)
	}
	equalBinding, err := canonicalEqual(canonicalBinding(current.Binding), canonicalBinding(command.Preview.CurrentBinding))
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	if current.Binding.ID != command.Preview.CurrentBinding.ID || !equalBinding ||
		command.Preview.ProposedBinding.Revision != current.Binding.Revision+1 ||
		command.Preview.ProposedBinding.BehaviorRevisionID != current.Behavior.ID {
		return ledger.AgentBindingAdvanceResult{}, fmt.Errorf("%w: Agent Binding Revision", ledger.ErrRevisionConflict)
	}
	if command.Preview.Participant.ConversationID != current.Home.ID ||
		command.Preview.Participant.DisplayName != current.Profile.Name {
		return ledger.AgentBindingAdvanceResult{}, fmt.Errorf("Rebind participant evidence does not match Agent Home and profile")
	}
	if err := ensureExecutionSource(ctx, tx, command.Preview.ProposedExecutionSource); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	if err := appendSQLiteSourceConfiguration(ctx, tx, "source-observation:rebind:"+command.Preview.ProposedBinding.ID,
		command.Preview.AccountID, command.Preview.ProposedBinding.ExecutionSourceID,
		command.Preview.ProposedBinding.SourceConfigDigest, command.AcceptedBy,
		command.Preview.ProposedBinding.ActivatedAt); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	if err := ensureSourceAgent(ctx, tx, command.Preview.ProposedSourceAgent); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	proposedBinding := canonicalBinding(command.Preview.ProposedBinding)
	if err := insertBindingRevision(ctx, tx, proposedBinding); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	if err := insertSuccessorParticipant(ctx, tx, command.Preview.AgentID, proposedBinding.ID, command.Preview.Participant); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	transition := ledger.AgentBindingTransition{
		AccountID: command.Preview.AccountID, AgentID: command.Preview.AgentID, Kind: ledger.BindingTransitionRebind,
		PreviousBehaviorRevisionID: current.Behavior.ID, SuccessorBehaviorRevisionID: current.Behavior.ID,
		PreviousBindingRevisionID: current.Binding.ID, SuccessorBindingRevisionID: proposedBinding.ID,
		PreviewDigest:            command.Preview.Digest,
		NonTransferableResources: sortedRebindResources(command.Preview.NonTransferableResources),
		ReadinessEvidence:        sortedStrings(command.Preview.ReadinessEvidence), AuthorityEvidence: sortedStrings(command.Preview.AuthorityEvidence),
		AcceptedBy: command.AcceptedBy, AcceptedAt: command.AcceptedAt,
	}
	if err := insertBindingTransition(ctx, tx, transition); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE stable_agent SET current_binding_revision_id=?
WHERE account_id=? AND id=? AND current_binding_revision_id=? AND current_behavior_revision_id=?`, proposedBinding.ID,
		command.Preview.AccountID, command.Preview.AgentID, current.Binding.ID, current.Behavior.ID)
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	if changed != 1 {
		return ledger.AgentBindingAdvanceResult{}, fmt.Errorf("%w: Agent Binding Revision", ledger.ErrRevisionConflict)
	}
	if err := insertSQLiteBindingAdvanceEvent(ctx, tx, transition); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	record, err := getAgentRecord(ctx, tx, command.Preview.AccountID, command.Preview.AgentID)
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	return ledger.AgentBindingAdvanceResult{Agent: record, Transition: transition}, nil
}

func (s *Store) CreateSecondaryConversation(ctx context.Context, command ledger.CreateSecondaryConversationCommand) (ledger.AgentConversationRecord, error) {
	if err := command.Validate(); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	defer tx.Rollback()
	claimed, err := claimAgentLifecycleIdempotency(ctx, tx, command.AccountID, "agent.conversation.create",
		command.IdempotencyKey, digest, command.Conversation.ID, command.Conversation.CreatedAt)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if !claimed {
		return getAgentConversationRecord(ctx, tx, command.AccountID, command.AgentID, command.Conversation.ID)
	}
	var state conversation.AgentState
	var canonicalID string
	if err := tx.QueryRowContext(ctx, `SELECT state,canonical_conversation_id FROM stable_agent
WHERE account_id=? AND id=?`, command.AccountID, command.AgentID).Scan(&state, &canonicalID); errors.Is(err, sql.ErrNoRows) {
		return ledger.AgentConversationRecord{}, fmt.Errorf("%w: Agent %q", ledger.ErrNotFound, command.AgentID)
	} else if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if state != conversation.AgentOpen {
		return ledger.AgentConversationRecord{}, fmt.Errorf("%w: archived Agent", ledger.ErrStateConflict)
	}
	if command.Conversation.ID == canonicalID {
		return ledger.AgentConversationRecord{}, fmt.Errorf("secondary Conversation cannot replace Home")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO conversation(id,project_id,title,state,created_at,updated_at)
VALUES(?,?,?,?,?,?)`, command.Conversation.ID, nullableString(command.Conversation.ProjectID), command.Conversation.Title,
		command.Conversation.State, nowOr(command.Conversation.CreatedAt), nowOr(command.Conversation.UpdatedAt)); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO agent_conversation(agent_id,conversation_id,kind,created_at)
VALUES(?,?,?,?)`, command.AgentID, command.Conversation.ID, conversation.AgentConversationSecondary,
		nowOr(command.Link.CreatedAt)); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if err := insertSQLiteAgentLifecycleEvent(ctx, tx, command.AccountID, command.AgentID, "agent.conversation.created",
		map[string]any{"conversation_id": command.Conversation.ID, "kind": command.Link.Kind, "created_by": command.CreatedBy},
		command.Conversation.CreatedAt); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	record, err := getAgentConversationRecord(ctx, tx, command.AccountID, command.AgentID, command.Conversation.ID)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	return record, nil
}

func (s *Store) ListAgentConversations(ctx context.Context, accountID, agentID string) ([]ledger.AgentConversationRecord, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(agentID) == "" {
		return nil, fmt.Errorf("Agent account id and id are required")
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM stable_agent WHERE account_id=? AND id=?`, accountID, agentID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: Agent %q", ledger.ErrNotFound, agentID)
	} else if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT item.id,item.project_id,item.title,item.state,item.created_at,item.updated_at,
link.agent_id,link.conversation_id,link.kind,link.created_at,
COALESCE((SELECT pin.pinned FROM agent_conversation_pin_revision pin
  WHERE pin.account_id=? AND pin.agent_id=link.agent_id AND pin.conversation_id=link.conversation_id
  ORDER BY pin.revision DESC LIMIT 1),0) AS pinned,
(SELECT CASE WHEN pin.pinned=1 THEN pin.changed_at ELSE NULL END FROM agent_conversation_pin_revision pin
  WHERE pin.account_id=? AND pin.agent_id=link.agent_id AND pin.conversation_id=link.conversation_id
  ORDER BY pin.revision DESC LIMIT 1) AS pinned_at
FROM agent_conversation link JOIN conversation item ON item.id=link.conversation_id
WHERE link.agent_id=? ORDER BY CASE link.kind WHEN 'canonical' THEN 0 ELSE 1 END,pinned DESC,item.updated_at DESC,link.conversation_id`, accountID, accountID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]ledger.AgentConversationRecord, 0)
	for rows.Next() {
		record, err := scanAgentConversationRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) RenameAgentConversation(ctx context.Context, command ledger.RenameAgentConversationCommand) (ledger.AgentConversationRecord, error) {
	if err := command.Validate(); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	defer tx.Rollback()
	claimed, err := claimAgentLifecycleIdempotency(ctx, tx, command.AccountID, "agent.conversation.rename",
		command.IdempotencyKey, digest, command.ConversationID, command.ChangedAt)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if !claimed {
		return getAgentConversationRecord(ctx, tx, command.AccountID, command.AgentID, command.ConversationID)
	}
	var kind conversation.AgentConversationKind
	var currentTitle string
	err = tx.QueryRowContext(ctx, `SELECT link.kind,item.title
FROM stable_agent agent JOIN agent_conversation link ON link.agent_id=agent.id
JOIN conversation item ON item.id=link.conversation_id
WHERE agent.account_id=? AND agent.id=? AND item.id=?`, command.AccountID, command.AgentID,
		command.ConversationID).Scan(&kind, &currentTitle)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.AgentConversationRecord{}, fmt.Errorf("%w: Agent Conversation %q", ledger.ErrNotFound, command.ConversationID)
	}
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if kind == conversation.AgentConversationCanonical {
		return ledger.AgentConversationRecord{}, fmt.Errorf("%w: Home cannot be renamed", ledger.ErrStateConflict)
	}
	if currentTitle != command.ExpectedTitle {
		return ledger.AgentConversationRecord{}, fmt.Errorf("%w: Conversation title", ledger.ErrRevisionConflict)
	}
	result, err := tx.ExecContext(ctx, `UPDATE conversation SET title=?,updated_at=? WHERE id=? AND title=?`,
		command.Title, nowOr(command.ChangedAt), command.ConversationID, command.ExpectedTitle)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if changed != 1 {
		return ledger.AgentConversationRecord{}, fmt.Errorf("%w: Conversation title", ledger.ErrRevisionConflict)
	}
	if err := insertSQLiteAgentLifecycleEvent(ctx, tx, command.AccountID, command.AgentID, "agent.conversation.renamed",
		map[string]any{"conversation_id": command.ConversationID, "previous_title": command.ExpectedTitle,
			"title": command.Title, "changed_by": command.ChangedBy}, command.ChangedAt); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	record, err := getAgentConversationRecord(ctx, tx, command.AccountID, command.AgentID, command.ConversationID)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	return record, nil
}

func (s *Store) SetAgentConversationState(ctx context.Context, command ledger.SetAgentConversationStateCommand) (ledger.AgentConversationRecord, error) {
	if err := command.Validate(); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	defer tx.Rollback()
	claimed, err := claimAgentLifecycleIdempotency(ctx, tx, command.AccountID, "agent.conversation.state",
		command.IdempotencyKey, digest, command.ConversationID, command.ChangedAt)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if !claimed {
		return getAgentConversationRecord(ctx, tx, command.AccountID, command.AgentID, command.ConversationID)
	}
	var agentState conversation.AgentState
	var kind conversation.AgentConversationKind
	var currentState conversation.ConversationState
	err = tx.QueryRowContext(ctx, `SELECT agent.state,link.kind,item.state
FROM stable_agent agent JOIN agent_conversation link ON link.agent_id=agent.id
JOIN conversation item ON item.id=link.conversation_id
WHERE agent.account_id=? AND agent.id=? AND item.id=?`, command.AccountID, command.AgentID,
		command.ConversationID).Scan(&agentState, &kind, &currentState)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.AgentConversationRecord{}, fmt.Errorf("%w: Agent Conversation %q", ledger.ErrNotFound, command.ConversationID)
	}
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if currentState != command.ExpectedState {
		return ledger.AgentConversationRecord{}, fmt.Errorf("%w: Conversation state", ledger.ErrStateConflict)
	}
	if kind == conversation.AgentConversationCanonical && agentState == conversation.AgentOpen && command.State != conversation.ConversationOpen {
		return ledger.AgentConversationRecord{}, fmt.Errorf("%w: open Agent Home cannot be archived", ledger.ErrStateConflict)
	}
	result, err := tx.ExecContext(ctx, `UPDATE conversation SET state=?,updated_at=? WHERE id=? AND state=?`, command.State,
		nowOr(command.ChangedAt), command.ConversationID, command.ExpectedState)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if changed != 1 {
		return ledger.AgentConversationRecord{}, fmt.Errorf("%w: Conversation state", ledger.ErrStateConflict)
	}
	if err := insertSQLiteAgentLifecycleEvent(ctx, tx, command.AccountID, command.AgentID, "agent.conversation.state_changed",
		map[string]any{"conversation_id": command.ConversationID, "previous_state": command.ExpectedState,
			"state": command.State, "changed_by": command.ChangedBy}, command.ChangedAt); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	record, err := getAgentConversationRecord(ctx, tx, command.AccountID, command.AgentID, command.ConversationID)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	return record, nil
}

func (s *Store) SetAgentConversationPin(ctx context.Context, command ledger.SetAgentConversationPinCommand) (ledger.AgentConversationRecord, error) {
	if err := command.Validate(); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	defer tx.Rollback()
	claimed, err := claimAgentLifecycleIdempotency(ctx, tx, command.AccountID, "agent.conversation.pin",
		command.IdempotencyKey, digest, command.ConversationID, command.ChangedAt)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if !claimed {
		return getAgentConversationRecord(ctx, tx, command.AccountID, command.AgentID, command.ConversationID)
	}
	var kind conversation.AgentConversationKind
	var currentPinned int
	var currentRevision int
	err = tx.QueryRowContext(ctx, `SELECT link.kind,
COALESCE((SELECT pin.pinned FROM agent_conversation_pin_revision pin
  WHERE pin.account_id=? AND pin.agent_id=link.agent_id AND pin.conversation_id=link.conversation_id
  ORDER BY pin.revision DESC LIMIT 1),0),
COALESCE((SELECT MAX(pin.revision) FROM agent_conversation_pin_revision pin
  WHERE pin.account_id=? AND pin.agent_id=link.agent_id AND pin.conversation_id=link.conversation_id),0)
FROM stable_agent agent JOIN agent_conversation link ON link.agent_id=agent.id
WHERE agent.account_id=? AND agent.id=? AND link.conversation_id=?`, command.AccountID, command.AccountID,
		command.AccountID, command.AgentID, command.ConversationID).Scan(&kind, &currentPinned, &currentRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.AgentConversationRecord{}, fmt.Errorf("%w: Agent Conversation %q", ledger.ErrNotFound, command.ConversationID)
	}
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if kind == conversation.AgentConversationCanonical {
		return ledger.AgentConversationRecord{}, fmt.Errorf("%w: Home is permanently unpinned", ledger.ErrStateConflict)
	}
	if (currentPinned != 0) != command.ExpectedPinned {
		return ledger.AgentConversationRecord{}, fmt.Errorf("%w: Conversation pin state", ledger.ErrStateConflict)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO agent_conversation_pin_revision(
account_id,agent_id,conversation_id,revision,pinned,changed_by,changed_at) VALUES(?,?,?,?,?,?,?)`,
		command.AccountID, command.AgentID, command.ConversationID, currentRevision+1, boolToInt(command.Pinned),
		command.ChangedBy, nowOr(command.ChangedAt)); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if err := insertSQLiteAgentLifecycleEvent(ctx, tx, command.AccountID, command.AgentID, "agent.conversation.pin_changed",
		map[string]any{"conversation_id": command.ConversationID, "previous_pinned": command.ExpectedPinned,
			"pinned": command.Pinned, "changed_by": command.ChangedBy}, command.ChangedAt); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	record, err := getAgentConversationRecord(ctx, tx, command.AccountID, command.AgentID, command.ConversationID)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	return record, nil
}

func canonicalBinding(binding conversation.AgentBindingRevision) conversation.AgentBindingRevision {
	binding.CapabilityEvidence = sortedStrings(binding.CapabilityEvidence)
	return binding
}

func sortedRebindResources(values []ledger.RebindResource) []ledger.RebindResource {
	out := append([]ledger.RebindResource{}, values...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func insertSQLiteBindingAdvanceEvent(ctx context.Context, tx *sql.Tx, transition ledger.AgentBindingTransition) error {
	metadata, err := json.Marshal(map[string]any{
		"kind": transition.Kind, "previous_behavior_revision_id": transition.PreviousBehaviorRevisionID,
		"successor_behavior_revision_id": transition.SuccessorBehaviorRevisionID,
		"previous_binding_revision_id":   transition.PreviousBindingRevisionID,
		"successor_binding_revision_id":  transition.SuccessorBindingRevisionID,
		"preview_digest":                 transition.PreviewDigest, "accepted_by": transition.AcceptedBy,
	})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO stable_agent_lifecycle_event(
account_id,agent_id,type,metadata_json,created_at) VALUES(?,?,?,?,?)`, transition.AccountID, transition.AgentID,
		"agent.binding.advanced", string(metadata), nowOr(transition.AcceptedAt))
	return err
}

func insertSQLiteAgentLifecycleEvent(ctx context.Context, tx *sql.Tx, accountID, agentID, eventType string, metadata any, createdAt time.Time) error {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO stable_agent_lifecycle_event(
account_id,agent_id,type,metadata_json,created_at) VALUES(?,?,?,?,?)`, accountID, agentID, eventType,
		string(payload), nowOr(createdAt))
	return err
}

type agentConversationScanner interface {
	Scan(...any) error
}

func getAgentConversationRecord(ctx context.Context, queryer agentQueryer, accountID, agentID, conversationID string) (ledger.AgentConversationRecord, error) {
	record, err := scanAgentConversationRecord(queryer.QueryRowContext(ctx, `SELECT item.id,item.project_id,item.title,item.state,item.created_at,item.updated_at,
link.agent_id,link.conversation_id,link.kind,link.created_at,
COALESCE((SELECT pin.pinned FROM agent_conversation_pin_revision pin
  WHERE pin.account_id=? AND pin.agent_id=link.agent_id AND pin.conversation_id=link.conversation_id
  ORDER BY pin.revision DESC LIMIT 1),0),
(SELECT CASE WHEN pin.pinned=1 THEN pin.changed_at ELSE NULL END FROM agent_conversation_pin_revision pin
  WHERE pin.account_id=? AND pin.agent_id=link.agent_id AND pin.conversation_id=link.conversation_id
  ORDER BY pin.revision DESC LIMIT 1)
FROM stable_agent agent JOIN agent_conversation link ON link.agent_id=agent.id
JOIN conversation item ON item.id=link.conversation_id
WHERE agent.account_id=? AND agent.id=? AND item.id=?`, accountID, accountID, accountID, agentID, conversationID))
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.AgentConversationRecord{}, fmt.Errorf("%w: Agent Conversation %q", ledger.ErrNotFound, conversationID)
	}
	return record, err
}

func scanAgentConversationRecord(scanner agentConversationScanner) (ledger.AgentConversationRecord, error) {
	var record ledger.AgentConversationRecord
	var projectID sql.NullString
	var created, updated, linked string
	var pinned int
	var pinnedAt sql.NullString
	err := scanner.Scan(&record.Conversation.ID, &projectID, &record.Conversation.Title, &record.Conversation.State,
		&created, &updated, &record.Link.AgentID, &record.Link.ConversationID, &record.Link.Kind, &linked, &pinned, &pinnedAt)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	record.Conversation.ProjectID = projectID.String
	record.Conversation.CreatedAt, record.Conversation.UpdatedAt = parseTime(created), parseTime(updated)
	record.Link.CreatedAt = parseTime(linked)
	record.Pinned, record.PinnedAt = pinned != 0, parseTime(pinnedAt.String)
	return record, nil
}

func sameBindingExecution(current, successor conversation.AgentBindingRevision) bool {
	current.ID, successor.ID = "", ""
	current.Revision, successor.Revision = 0, 0
	current.BehaviorRevisionID, successor.BehaviorRevisionID = "", ""
	current.SeatID, successor.SeatID = "", ""
	current.SupersedesRevisionID, successor.SupersedesRevisionID = "", ""
	current.ActivatedAt, successor.ActivatedAt = time.Time{}, time.Time{}
	current.RetiredAt, successor.RetiredAt = time.Time{}, time.Time{}
	current.CapabilityEvidence = sortedStrings(current.CapabilityEvidence)
	successor.CapabilityEvidence = sortedStrings(successor.CapabilityEvidence)
	equal, err := canonicalEqual(current, successor)
	return err == nil && equal
}

func insertSuccessorParticipant(ctx context.Context, tx *sql.Tx, agentID, bindingID string, participant conversation.Participant) error {
	existing, err := scanStableAgentParticipant(tx.QueryRowContext(ctx, `SELECT id,conversation_id,seat_id,profile,
agent,model,machine,display_name,position,state,created_at,removed_at FROM conversation_participant
WHERE conversation_id=? AND seat_id=?`, participant.ConversationID, participant.SeatID))
	if err == nil {
		equal, compareErr := canonicalEqual(existing, participant)
		if compareErr != nil {
			return compareErr
		}
		if !equal {
			return fmt.Errorf("successor Binding seat conflicts with immutable participant evidence")
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO conversation_participant(
id,conversation_id,seat_id,profile,agent,model,machine,display_name,position,state,created_at,removed_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,NULL)`, participant.ID, participant.ConversationID, participant.SeatID,
		participant.Profile, participant.Agent, nullableString(participant.Model), participant.Machine,
		participant.DisplayName, participant.Position, participant.State, nowOr(participant.CreatedAt)); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO stable_agent_participant_evidence(
agent_id,binding_revision_id,conversation_id,participant_id,created_at) VALUES(?,?,?,?,?)`, agentID,
		bindingID, participant.ConversationID, participant.ID, nowOr(participant.CreatedAt))
	return err
}

func insertBindingTransition(ctx context.Context, tx *sql.Tx, transition ledger.AgentBindingTransition) error {
	resources, err := json.Marshal(transition.NonTransferableResources)
	if err != nil {
		return err
	}
	readiness, err := marshalCanonicalStrings(transition.ReadinessEvidence)
	if err != nil {
		return err
	}
	authority, err := marshalCanonicalStrings(transition.AuthorityEvidence)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO agent_binding_transition(
agent_id,kind,previous_behavior_revision_id,successor_behavior_revision_id,
previous_binding_revision_id,successor_binding_revision_id,preview_digest,
non_transferable_resources_json,readiness_evidence_json,authority_evidence_json,accepted_by,accepted_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, transition.AgentID, transition.Kind, transition.PreviousBehaviorRevisionID,
		transition.SuccessorBehaviorRevisionID, transition.PreviousBindingRevisionID,
		transition.SuccessorBindingRevisionID, transition.PreviewDigest, string(resources), readiness, authority,
		transition.AcceptedBy, nowOr(transition.AcceptedAt))
	return err
}

func getBindingTransition(ctx context.Context, tx *sql.Tx, accountID, agentID, successorBindingID string) (ledger.AgentBindingTransition, error) {
	var transition ledger.AgentBindingTransition
	var resources, readiness, authority, acceptedAt string
	err := tx.QueryRowContext(ctx, `SELECT transition.agent_id,transition.kind,
transition.previous_behavior_revision_id,transition.successor_behavior_revision_id,
transition.previous_binding_revision_id,transition.successor_binding_revision_id,transition.preview_digest,
transition.non_transferable_resources_json,transition.readiness_evidence_json,
transition.authority_evidence_json,transition.accepted_by,transition.accepted_at
FROM agent_binding_transition transition JOIN stable_agent agent ON agent.id=transition.agent_id
WHERE agent.account_id=? AND transition.agent_id=? AND transition.successor_binding_revision_id=?`, accountID,
		agentID, successorBindingID).Scan(&transition.AgentID, &transition.Kind,
		&transition.PreviousBehaviorRevisionID, &transition.SuccessorBehaviorRevisionID,
		&transition.PreviousBindingRevisionID, &transition.SuccessorBindingRevisionID, &transition.PreviewDigest,
		&resources, &readiness, &authority, &transition.AcceptedBy, &acceptedAt)
	if err != nil {
		return ledger.AgentBindingTransition{}, err
	}
	transition.AccountID = accountID
	transition.AcceptedAt = parseTime(acceptedAt)
	if err := json.Unmarshal([]byte(resources), &transition.NonTransferableResources); err != nil {
		return ledger.AgentBindingTransition{}, err
	}
	if err := unmarshalStringList(readiness, &transition.ReadinessEvidence); err != nil {
		return ledger.AgentBindingTransition{}, err
	}
	if err := unmarshalStringList(authority, &transition.AuthorityEvidence); err != nil {
		return ledger.AgentBindingTransition{}, err
	}
	return transition, nil
}

func claimAgentLifecycleIdempotency(ctx context.Context, tx *sql.Tx, accountID, scope, key, digest, resultID string, createdAt time.Time) (bool, error) {
	result, err := tx.ExecContext(ctx, `INSERT INTO stable_agent_lifecycle_idempotency(
account_id,scope,idempotency_key,command_digest,result_id,created_at) VALUES(?,?,?,?,?,?)
ON CONFLICT(account_id,scope,idempotency_key) DO NOTHING`, accountID, scope, key, digest, resultID, nowOr(createdAt))
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if changed == 1 {
		return true, nil
	}
	var existingDigest, existingResultID string
	if err := tx.QueryRowContext(ctx, `SELECT command_digest,result_id FROM stable_agent_lifecycle_idempotency
WHERE account_id=? AND scope=? AND idempotency_key=?`, accountID, scope, key).Scan(&existingDigest, &existingResultID); err != nil {
		return false, err
	}
	if existingDigest != digest || existingResultID != resultID {
		return false, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, key)
	}
	return false, nil
}

func (s *Store) GetAgent(ctx context.Context, accountID, agentID string) (ledger.AgentRecord, error) {
	return getAgentRecord(ctx, s.db, accountID, agentID)
}

func (s *Store) ListAgents(ctx context.Context, accountID string, state conversation.AgentState) ([]ledger.AgentRecord, error) {
	if strings.TrimSpace(accountID) == "" {
		return nil, fmt.Errorf("Agent account id is required")
	}
	if state == "" {
		state = conversation.AgentOpen
	}
	if state != conversation.AgentOpen && state != conversation.AgentArchived {
		return nil, fmt.Errorf("Agent state must be open or archived")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM stable_agent
WHERE account_id=? AND state=? ORDER BY created_at,id`, accountID, state)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	records := make([]ledger.AgentRecord, 0, len(ids))
	for _, id := range ids {
		record, err := getAgentRecord(ctx, s.db, accountID, id)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

type agentQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getAgentRecord(ctx context.Context, queryer agentQueryer, accountID, agentID string) (ledger.AgentRecord, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(agentID) == "" {
		return ledger.AgentRecord{}, fmt.Errorf("Agent account id and id are required")
	}
	var record ledger.AgentRecord
	var agentCreated string
	err := queryer.QueryRowContext(ctx, `SELECT id,account_id,state,current_profile_revision_id,
current_behavior_revision_id,current_binding_revision_id,canonical_conversation_id,created_at
FROM stable_agent WHERE account_id=? AND id=?`, accountID, agentID).Scan(
		&record.Agent.ID, &record.Agent.AccountID, &record.Agent.State, &record.Agent.CurrentProfileRevisionID,
		&record.Agent.CurrentBehaviorRevisionID, &record.Agent.CurrentBindingRevisionID,
		&record.Agent.CanonicalConversationID, &agentCreated)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.AgentRecord{}, fmt.Errorf("%w: Agent %q", ledger.ErrNotFound, agentID)
	}
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	record.Agent.CreatedAt = parseTime(agentCreated)
	if record.Profile, err = scanProfileRevision(queryer.QueryRowContext(ctx, `SELECT id,agent_id,revision,name,title,
avatar_url,hidden,pinned,sort_order,created_at FROM agent_profile_revision WHERE id=? AND agent_id=?`,
		record.Agent.CurrentProfileRevisionID, record.Agent.ID)); err != nil {
		return ledger.AgentRecord{}, err
	}
	if record.Behavior, err = scanBehaviorRevision(queryer.QueryRowContext(ctx, `SELECT id,agent_id,revision,role,
standing_instructions,enabled_skills_json,enabled_tools_json,prompt_material,created_at
FROM agent_behavior_revision WHERE id=? AND agent_id=?`, record.Agent.CurrentBehaviorRevisionID, record.Agent.ID)); err != nil {
		return ledger.AgentRecord{}, err
	}
	if record.Binding, err = scanBindingRevision(queryer.QueryRowContext(ctx, `SELECT id,agent_id,revision,
behavior_revision_id,execution_source_id,source_agent_id,seat_id,fort_profile,provider,requested_model,
resolved_model,computer_id,cloud_runtime,adapter_id,adapter_revision,source_config_digest,authority_id,
authority_revision,policy_id,policy_revision,session_behavior,memory_behavior,capability_evidence_json,
readiness_contract_id,readiness_contract_revision,supersedes_revision_id,activated_at,retired_at
FROM agent_binding_revision WHERE id=? AND agent_id=?`, record.Agent.CurrentBindingRevisionID, record.Agent.ID)); err != nil {
		return ledger.AgentRecord{}, err
	}
	if record.ExecutionSource, err = scanExecutionSource(queryer.QueryRowContext(ctx, `SELECT id,account_id,framework,
instance_id,gateway_id,display_name,resource_sharing_json,last_seen_at FROM execution_source WHERE id=?`,
		record.Binding.ExecutionSourceID)); err != nil {
		return ledger.AgentRecord{}, err
	}
	if record.ExecutionSource.AccountID != accountID {
		return ledger.AgentRecord{}, fmt.Errorf("stable Agent source belongs to another account")
	}
	if record.SourceAgent, err = scanSourceAgent(queryer.QueryRowContext(ctx, `SELECT id,execution_source_id,
opaque_source_agent_id,display_name,last_seen_at FROM source_agent WHERE id=? AND execution_source_id=?`,
		record.Binding.SourceAgentID, record.Binding.ExecutionSourceID)); err != nil {
		return ledger.AgentRecord{}, err
	}
	if record.Home, err = scanStableAgentHome(queryer.QueryRowContext(ctx, `SELECT id,project_id,title,state,created_at,updated_at
FROM conversation WHERE id=?`, record.Agent.CanonicalConversationID)); err != nil {
		return ledger.AgentRecord{}, err
	}
	if record.Participant, err = scanStableAgentParticipant(queryer.QueryRowContext(ctx, `SELECT p.id,p.conversation_id,
p.seat_id,p.profile,p.agent,p.model,p.machine,p.display_name,p.position,p.state,p.created_at,p.removed_at
FROM conversation_participant p WHERE p.conversation_id=? AND p.seat_id=?`, record.Home.ID,
		record.Binding.SeatID)); err != nil {
		return ledger.AgentRecord{}, err
	}
	var linkCreated string
	if err := queryer.QueryRowContext(ctx, `SELECT agent_id,conversation_id,kind,created_at FROM agent_conversation
WHERE agent_id=? AND conversation_id=? AND kind='canonical'`, record.Agent.ID, record.Home.ID).Scan(
		&record.Link.AgentID, &record.Link.ConversationID, &record.Link.Kind, &linkCreated); err != nil {
		return ledger.AgentRecord{}, err
	}
	record.Link.CreatedAt = parseTime(linkCreated)
	return record, nil
}

func ensureExecutionSource(ctx context.Context, tx *sql.Tx, source conversation.ExecutionSource) error {
	existing, err := scanExecutionSource(tx.QueryRowContext(ctx, `SELECT id,account_id,framework,instance_id,gateway_id,
display_name,resource_sharing_json,last_seen_at FROM execution_source WHERE id=?`, source.ID))
	if err == nil {
		equal, compareErr := canonicalEqual(existing, source)
		if compareErr != nil {
			return compareErr
		}
		if !equal {
			return fmt.Errorf("Execution Source %q conflicts with immutable evidence", source.ID)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	sharing, err := json.Marshal(source.ResourceSharing)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO execution_source(id,account_id,framework,instance_id,gateway_id,
display_name,resource_sharing_json,last_seen_at) VALUES(?,?,?,?,?,?,?,?)`, source.ID, source.AccountID,
		source.Framework, source.InstanceID, source.GatewayID, source.DisplayName, string(sharing), nullableTime(source.LastSeenAt))
	return err
}

func ensureSourceAgent(ctx context.Context, tx *sql.Tx, sourceAgent conversation.SourceAgent) error {
	existing, err := scanSourceAgent(tx.QueryRowContext(ctx, `SELECT id,execution_source_id,opaque_source_agent_id,
display_name,last_seen_at FROM source_agent WHERE id=?`, sourceAgent.ID))
	if err == nil {
		equal, compareErr := canonicalEqual(existing, sourceAgent)
		if compareErr != nil {
			return compareErr
		}
		if !equal {
			return fmt.Errorf("Source Agent %q conflicts with immutable evidence", sourceAgent.ID)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO source_agent(id,execution_source_id,opaque_source_agent_id,
display_name,last_seen_at) VALUES(?,?,?,?,?)`, sourceAgent.ID, sourceAgent.ExecutionSourceID,
		sourceAgent.OpaqueSourceAgentID, sourceAgent.DisplayName, nullableTime(sourceAgent.LastSeenAt))
	return err
}

func insertProfileRevision(ctx context.Context, tx *sql.Tx, revision conversation.AgentProfileRevision) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO agent_profile_revision(id,agent_id,revision,name,title,avatar_url,
hidden,pinned,sort_order,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, revision.ID, revision.AgentID, revision.Revision,
		revision.Name, revision.Title, revision.AvatarURL, boolToInt(revision.Hidden), boolToInt(revision.Pinned),
		revision.SortOrder, nowOr(revision.CreatedAt))
	return err
}

func insertBehaviorRevision(ctx context.Context, tx *sql.Tx, revision conversation.AgentBehaviorRevision) error {
	skills, err := marshalCanonicalStrings(revision.EnabledSkills)
	if err != nil {
		return err
	}
	tools, err := marshalCanonicalStrings(revision.EnabledTools)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO agent_behavior_revision(id,agent_id,revision,role,standing_instructions,
enabled_skills_json,enabled_tools_json,prompt_material,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, revision.ID,
		revision.AgentID, revision.Revision, revision.Role, revision.StandingInstructions, skills, tools,
		revision.PromptMaterial, nowOr(revision.CreatedAt))
	return err
}

func insertBindingRevision(ctx context.Context, tx *sql.Tx, revision conversation.AgentBindingRevision) error {
	capabilities, err := marshalCanonicalStrings(revision.CapabilityEvidence)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO agent_binding_revision(id,agent_id,revision,behavior_revision_id,
execution_source_id,source_agent_id,seat_id,fort_profile,provider,requested_model,resolved_model,computer_id,
cloud_runtime,adapter_id,adapter_revision,source_config_digest,authority_id,authority_revision,policy_id,
policy_revision,session_behavior,memory_behavior,capability_evidence_json,readiness_contract_id,
readiness_contract_revision,supersedes_revision_id,activated_at,retired_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, revision.ID, revision.AgentID, revision.Revision,
		revision.BehaviorRevisionID, revision.ExecutionSourceID, revision.SourceAgentID, revision.SeatID,
		revision.FortProfile, revision.Provider, revision.RequestedModel, revision.ResolvedModel, revision.ComputerID,
		revision.CloudRuntime, revision.AdapterID, revision.AdapterRevision, revision.SourceConfigDigest,
		revision.AuthorityID, revision.AuthorityRevision, revision.PolicyID, revision.PolicyRevision,
		revision.SessionBehavior, revision.MemoryBehavior, capabilities, revision.ReadinessContractID,
		revision.ReadinessContractRevision, revision.SupersedesRevisionID, nowOr(revision.ActivatedAt),
		nullableTime(revision.RetiredAt))
	return err
}

func canonicalAgentCommand(command ledger.CreateAgentCommand) ledger.CreateAgentCommand {
	command.Behavior.EnabledSkills = sortedStrings(command.Behavior.EnabledSkills)
	command.Behavior.EnabledTools = sortedStrings(command.Behavior.EnabledTools)
	command.Binding.CapabilityEvidence = sortedStrings(command.Binding.CapabilityEvidence)
	return command
}

func sortedStrings(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}

func marshalCanonicalStrings(values []string) (string, error) {
	payload, err := json.Marshal(sortedStrings(values))
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func canonicalEqual(left, right any) (bool, error) {
	a, err := json.Marshal(left)
	if err != nil {
		return false, err
	}
	b, err := json.Marshal(right)
	if err != nil {
		return false, err
	}
	return string(a) == string(b), nil
}

func scanProfileRevision(row *sql.Row) (conversation.AgentProfileRevision, error) {
	var revision conversation.AgentProfileRevision
	var hidden, pinned int
	var created string
	err := row.Scan(&revision.ID, &revision.AgentID, &revision.Revision, &revision.Name, &revision.Title,
		&revision.AvatarURL, &hidden, &pinned, &revision.SortOrder, &created)
	revision.Hidden, revision.Pinned, revision.CreatedAt = hidden != 0, pinned != 0, parseTime(created)
	return revision, err
}

func scanBehaviorRevision(row *sql.Row) (conversation.AgentBehaviorRevision, error) {
	var revision conversation.AgentBehaviorRevision
	var skills, tools, created string
	err := row.Scan(&revision.ID, &revision.AgentID, &revision.Revision, &revision.Role,
		&revision.StandingInstructions, &skills, &tools, &revision.PromptMaterial, &created)
	if err != nil {
		return conversation.AgentBehaviorRevision{}, err
	}
	if err := unmarshalStringList(skills, &revision.EnabledSkills); err != nil {
		return conversation.AgentBehaviorRevision{}, err
	}
	if err := unmarshalStringList(tools, &revision.EnabledTools); err != nil {
		return conversation.AgentBehaviorRevision{}, err
	}
	revision.CreatedAt = parseTime(created)
	return revision, nil
}

func scanBindingRevision(row *sql.Row) (conversation.AgentBindingRevision, error) {
	var revision conversation.AgentBindingRevision
	var capabilities, activated string
	var retired sql.NullString
	err := row.Scan(&revision.ID, &revision.AgentID, &revision.Revision, &revision.BehaviorRevisionID,
		&revision.ExecutionSourceID, &revision.SourceAgentID, &revision.SeatID, &revision.FortProfile,
		&revision.Provider, &revision.RequestedModel, &revision.ResolvedModel, &revision.ComputerID,
		&revision.CloudRuntime, &revision.AdapterID, &revision.AdapterRevision, &revision.SourceConfigDigest,
		&revision.AuthorityID, &revision.AuthorityRevision, &revision.PolicyID, &revision.PolicyRevision,
		&revision.SessionBehavior, &revision.MemoryBehavior, &capabilities, &revision.ReadinessContractID,
		&revision.ReadinessContractRevision, &revision.SupersedesRevisionID, &activated, &retired)
	if err != nil {
		return conversation.AgentBindingRevision{}, err
	}
	if err := unmarshalStringList(capabilities, &revision.CapabilityEvidence); err != nil {
		return conversation.AgentBindingRevision{}, err
	}
	revision.ActivatedAt, revision.RetiredAt = parseTime(activated), parseTime(retired.String)
	return revision, nil
}

func scanExecutionSource(row *sql.Row) (conversation.ExecutionSource, error) {
	var source conversation.ExecutionSource
	var sharing string
	var lastSeen sql.NullString
	err := row.Scan(&source.ID, &source.AccountID, &source.Framework, &source.InstanceID, &source.GatewayID,
		&source.DisplayName, &sharing, &lastSeen)
	if err != nil {
		return conversation.ExecutionSource{}, err
	}
	if err := json.Unmarshal([]byte(sharing), &source.ResourceSharing); err != nil {
		return conversation.ExecutionSource{}, err
	}
	source.LastSeenAt = parseTime(lastSeen.String)
	return source, nil
}

func scanSourceAgent(row *sql.Row) (conversation.SourceAgent, error) {
	var sourceAgent conversation.SourceAgent
	var lastSeen sql.NullString
	err := row.Scan(&sourceAgent.ID, &sourceAgent.ExecutionSourceID, &sourceAgent.OpaqueSourceAgentID,
		&sourceAgent.DisplayName, &lastSeen)
	sourceAgent.LastSeenAt = parseTime(lastSeen.String)
	return sourceAgent, err
}

func scanStableAgentHome(row *sql.Row) (conversation.Conversation, error) {
	var home conversation.Conversation
	var projectID sql.NullString
	var created, updated string
	err := row.Scan(&home.ID, &projectID, &home.Title, &home.State, &created, &updated)
	home.ProjectID, home.CreatedAt, home.UpdatedAt = projectID.String, parseTime(created), parseTime(updated)
	return home, err
}

func scanStableAgentParticipant(row *sql.Row) (conversation.Participant, error) {
	var participant conversation.Participant
	var model, removed sql.NullString
	var created string
	err := row.Scan(&participant.ID, &participant.ConversationID, &participant.SeatID, &participant.Profile,
		&participant.Agent, &model, &participant.Machine, &participant.DisplayName, &participant.Position,
		&participant.State, &created, &removed)
	participant.Model, participant.CreatedAt, participant.RemovedAt = model.String, parseTime(created), parseTime(removed.String)
	return participant, err
}

func unmarshalStringList(raw string, destination *[]string) error {
	if err := json.Unmarshal([]byte(raw), destination); err != nil {
		return err
	}
	if *destination == nil {
		*destination = []string{}
	}
	return nil
}
