package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

// collaborationSchema is additive: the legacy Conversation, Turn, Target,
// Agent Channel, and stable-Agent tables remain intact for rollback.
const collaborationSchema = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_stable_agent_account_id
  ON stable_agent(account_id,id);

CREATE TABLE IF NOT EXISTS stable_group (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  conversation_id TEXT NOT NULL UNIQUE,
  state TEXT NOT NULL CHECK(state IN ('open','archived')),
  current_membership_revision_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(account_id,id),
  FOREIGN KEY(conversation_id) REFERENCES conversation(id)
);
CREATE INDEX IF NOT EXISTS idx_stable_group_account_state
  ON stable_group(account_id,state,created_at,id);

CREATE TABLE IF NOT EXISTS group_membership_revision (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  revision INTEGER NOT NULL CHECK(revision>0),
  created_at TEXT NOT NULL,
  UNIQUE(account_id,group_id,revision),
  UNIQUE(account_id,group_id,id),
  FOREIGN KEY(account_id,group_id) REFERENCES stable_group(account_id,id)
);

CREATE TABLE IF NOT EXISTS group_member_revision (
  account_id TEXT NOT NULL,
  membership_revision_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  agent_id TEXT NOT NULL,
  position INTEGER NOT NULL CHECK(position>=0 AND position<6),
  PRIMARY KEY(account_id,membership_revision_id,agent_id),
  UNIQUE(account_id,membership_revision_id,position),
  UNIQUE(account_id,membership_revision_id,group_id,agent_id),
  FOREIGN KEY(account_id,group_id,membership_revision_id)
    REFERENCES group_membership_revision(account_id,group_id,id),
  FOREIGN KEY(account_id,agent_id) REFERENCES stable_agent(account_id,id)
);

CREATE TABLE IF NOT EXISTS group_member_binding (
  account_id TEXT NOT NULL,
  membership_revision_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  conversation_id TEXT NOT NULL,
  agent_id TEXT NOT NULL,
  behavior_revision_id TEXT NOT NULL,
  binding_revision_id TEXT NOT NULL,
  participant_id TEXT NOT NULL,
  PRIMARY KEY(account_id,membership_revision_id,agent_id),
  UNIQUE(account_id,membership_revision_id,agent_id,behavior_revision_id,binding_revision_id,participant_id),
  FOREIGN KEY(account_id,membership_revision_id,group_id,agent_id)
    REFERENCES group_member_revision(account_id,membership_revision_id,group_id,agent_id),
  FOREIGN KEY(behavior_revision_id,agent_id) REFERENCES agent_behavior_revision(id,agent_id),
  FOREIGN KEY(binding_revision_id,agent_id) REFERENCES agent_binding_revision(id,agent_id),
  FOREIGN KEY(agent_id,binding_revision_id,conversation_id)
    REFERENCES stable_agent_participant_evidence(agent_id,binding_revision_id,conversation_id),
  FOREIGN KEY(participant_id) REFERENCES conversation_participant(id),
  FOREIGN KEY(conversation_id) REFERENCES conversation(id)
);

CREATE TABLE IF NOT EXISTS stable_group_create_idempotency (
  account_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  command_digest TEXT NOT NULL CHECK(length(command_digest)=64 AND command_digest NOT GLOB '*[^0-9a-f]*'),
  group_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY(account_id,idempotency_key),
  FOREIGN KEY(account_id,group_id) REFERENCES stable_group(account_id,id)
);

CREATE TABLE IF NOT EXISTS stable_group_lifecycle_event (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  type TEXT NOT NULL,
  metadata_json TEXT NOT NULL CHECK(json_valid(metadata_json)),
  created_at TEXT NOT NULL,
  FOREIGN KEY(account_id,group_id) REFERENCES stable_group(account_id,id)
);
CREATE INDEX IF NOT EXISTS idx_stable_group_lifecycle_event
  ON stable_group_lifecycle_event(account_id,group_id,id);

CREATE TABLE IF NOT EXISTS stable_group_turn (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  conversation_id TEXT NOT NULL,
  client_turn_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  command_digest TEXT NOT NULL CHECK(length(command_digest)=64 AND command_digest NOT GLOB '*[^0-9a-f]*'),
  membership_revision_id TEXT NOT NULL,
  selection TEXT NOT NULL CHECK(selection IN ('explicit','everyone')),
  context_snapshot_id TEXT NOT NULL,
  delegation_grant_json TEXT NOT NULL DEFAULT '{"id":"legacy:group-turn","permissions":[],"context_record_ids":[]}' CHECK(json_valid(delegation_grant_json)),
  concurrency_policy TEXT NOT NULL CHECK(concurrency_policy IN ('sequential','concurrent')),
  cancellation_policy_id TEXT NOT NULL DEFAULT 'group-cancel:human-or-deadline',
  cancellation_policy_revision TEXT NOT NULL DEFAULT '1',
  approval_policy_id TEXT NOT NULL DEFAULT 'group-approval:explicit',
  approval_policy_revision TEXT NOT NULL DEFAULT '1',
  max_agent_messages INTEGER NOT NULL CHECK(max_agent_messages=10),
  max_handoff_depth INTEGER NOT NULL CHECK(max_handoff_depth=3),
  cost_limit_class TEXT NOT NULL CHECK(cost_limit_class IN ('hard','unknown')),
  cost_limit_evidence_id TEXT,
  token_limit_class TEXT NOT NULL CHECK(token_limit_class IN ('hard','unknown')),
  token_limit_evidence_id TEXT,
  deadline TEXT NOT NULL,
  prompt_message_id INTEGER NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  UNIQUE(account_id,id),
  UNIQUE(account_id,idempotency_key),
  UNIQUE(account_id,group_id,client_turn_id),
  UNIQUE(account_id,id,membership_revision_id),
  FOREIGN KEY(account_id,group_id) REFERENCES stable_group(account_id,id),
  FOREIGN KEY(account_id,group_id,membership_revision_id)
    REFERENCES group_membership_revision(account_id,group_id,id),
  FOREIGN KEY(conversation_id) REFERENCES conversation(id),
  FOREIGN KEY(prompt_message_id) REFERENCES conversation_message(id),
  CHECK((cost_limit_class='hard' AND cost_limit_evidence_id IS NOT NULL) OR cost_limit_class='unknown'),
  CHECK((token_limit_class='hard' AND token_limit_evidence_id IS NOT NULL) OR token_limit_class='unknown')
);
CREATE INDEX IF NOT EXISTS idx_stable_group_turn_group
  ON stable_group_turn(account_id,group_id,created_at,id);

CREATE TABLE IF NOT EXISTS stable_group_turn_recipient (
  account_id TEXT NOT NULL,
  turn_id TEXT NOT NULL,
  membership_revision_id TEXT NOT NULL,
  position INTEGER NOT NULL CHECK(position>=0 AND position<6),
  agent_id TEXT NOT NULL,
  behavior_revision_id TEXT NOT NULL,
  binding_revision_id TEXT NOT NULL,
  participant_id TEXT NOT NULL,
  PRIMARY KEY(account_id,turn_id,agent_id),
  UNIQUE(account_id,turn_id,position),
  UNIQUE(account_id,turn_id,agent_id,behavior_revision_id,binding_revision_id,participant_id),
  FOREIGN KEY(account_id,turn_id,membership_revision_id)
    REFERENCES stable_group_turn(account_id,id,membership_revision_id),
  FOREIGN KEY(account_id,membership_revision_id,agent_id,behavior_revision_id,binding_revision_id,participant_id)
    REFERENCES group_member_binding(account_id,membership_revision_id,agent_id,behavior_revision_id,binding_revision_id,participant_id)
);

CREATE TABLE IF NOT EXISTS stable_group_initial_target (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  turn_id TEXT NOT NULL,
  wave INTEGER NOT NULL CHECK(wave=0),
  agent_id TEXT NOT NULL,
  behavior_revision_id TEXT NOT NULL,
  binding_revision_id TEXT NOT NULL,
  participant_id TEXT NOT NULL,
  state TEXT NOT NULL CHECK(state IN ('queued','working','answered','failed','canceled')),
  created_at TEXT NOT NULL,
  UNIQUE(account_id,turn_id,agent_id),
  FOREIGN KEY(account_id,turn_id,agent_id,behavior_revision_id,binding_revision_id,participant_id)
    REFERENCES stable_group_turn_recipient(account_id,turn_id,agent_id,behavior_revision_id,binding_revision_id,participant_id)
);

CREATE TABLE IF NOT EXISTS stable_handoff (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  command_digest TEXT NOT NULL CHECK(length(command_digest)=64 AND command_digest NOT GLOB '*[^0-9a-f]*'),
  state TEXT NOT NULL CHECK(state IN ('queued','needs_you','working','completed','failed','canceled')),
  group_turn_id TEXT,
  source_message_id INTEGER NOT NULL,
  source_agent_id TEXT,
  recipient_agent_id TEXT NOT NULL,
  recipient_behavior_revision_id TEXT NOT NULL,
  recipient_binding_revision_id TEXT NOT NULL,
  source_conversation_id TEXT NOT NULL,
  output_conversation_id TEXT NOT NULL,
  context_json TEXT NOT NULL CHECK(json_valid(context_json)),
  effective_authority_json TEXT NOT NULL CHECK(json_valid(effective_authority_json)),
  command_json TEXT NOT NULL CHECK(json_valid(command_json)),
  created_at TEXT NOT NULL,
  completed_at TEXT,
  UNIQUE(account_id,id),
  UNIQUE(account_id,idempotency_key),
  FOREIGN KEY(source_message_id) REFERENCES conversation_message(id),
  FOREIGN KEY(recipient_agent_id) REFERENCES stable_agent(id),
  FOREIGN KEY(recipient_behavior_revision_id,recipient_agent_id)
    REFERENCES agent_behavior_revision(id,agent_id),
  FOREIGN KEY(recipient_binding_revision_id,recipient_agent_id)
    REFERENCES agent_binding_revision(id,agent_id),
  FOREIGN KEY(source_conversation_id) REFERENCES conversation(id),
  FOREIGN KEY(output_conversation_id) REFERENCES conversation(id),
  FOREIGN KEY(account_id,group_turn_id) REFERENCES stable_group_turn(account_id,id)
);
CREATE INDEX IF NOT EXISTS idx_stable_handoff_account_created
  ON stable_handoff(account_id,created_at,id);

CREATE TABLE IF NOT EXISTS stable_handoff_target (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  handoff_id TEXT NOT NULL,
  conversation_id TEXT NOT NULL,
  agent_id TEXT NOT NULL,
  behavior_revision_id TEXT NOT NULL,
  binding_revision_id TEXT NOT NULL,
  participant_id TEXT NOT NULL,
  state TEXT NOT NULL CHECK(state IN ('queued','working','answered','failed','canceled')),
  created_at TEXT NOT NULL,
  UNIQUE(account_id,handoff_id),
  FOREIGN KEY(account_id,handoff_id) REFERENCES stable_handoff(account_id,id),
  FOREIGN KEY(behavior_revision_id,agent_id) REFERENCES agent_behavior_revision(id,agent_id),
  FOREIGN KEY(binding_revision_id,agent_id) REFERENCES agent_binding_revision(id,agent_id),
  FOREIGN KEY(agent_id,binding_revision_id,conversation_id)
    REFERENCES stable_agent_participant_evidence(agent_id,binding_revision_id,conversation_id),
  FOREIGN KEY(participant_id) REFERENCES conversation_participant(id),
  FOREIGN KEY(conversation_id) REFERENCES conversation(id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_stable_handoff_target_cancellation_evidence
  ON stable_handoff_target(account_id,handoff_id,id,agent_id,behavior_revision_id,binding_revision_id,participant_id);

CREATE TABLE IF NOT EXISTS stable_handoff_cancellation (
  account_id TEXT NOT NULL,
  handoff_id TEXT NOT NULL,
  target_id TEXT NOT NULL,
  agent_id TEXT NOT NULL,
  behavior_revision_id TEXT NOT NULL,
  binding_revision_id TEXT NOT NULL,
  participant_id TEXT NOT NULL,
  state TEXT NOT NULL CHECK(state IN ('requested','canceled')),
  requested_by TEXT NOT NULL,
  requested_at TEXT NOT NULL,
  PRIMARY KEY(account_id,handoff_id),
  FOREIGN KEY(account_id,handoff_id,target_id,agent_id,behavior_revision_id,binding_revision_id,participant_id)
    REFERENCES stable_handoff_target(account_id,handoff_id,id,agent_id,behavior_revision_id,binding_revision_id,participant_id)
);

CREATE TABLE IF NOT EXISTS stable_handoff_projection (
  account_id TEXT NOT NULL,
  handoff_id TEXT NOT NULL,
  conversation_id TEXT NOT NULL,
  output_conversation_id TEXT NOT NULL,
  authoritative_message_id INTEGER,
  state TEXT NOT NULL CHECK(state IN ('queued','needs_you','working','completed','failed','canceled')),
  projected_at TEXT NOT NULL,
  PRIMARY KEY(account_id,handoff_id,conversation_id),
  FOREIGN KEY(account_id,handoff_id) REFERENCES stable_handoff(account_id,id),
  FOREIGN KEY(conversation_id) REFERENCES conversation(id),
  FOREIGN KEY(output_conversation_id) REFERENCES conversation(id),
  FOREIGN KEY(authoritative_message_id) REFERENCES conversation_message(id),
  CHECK(conversation_id<>output_conversation_id),
  CHECK((state='completed' AND authoritative_message_id IS NOT NULL) OR
        (state<>'completed' AND authoritative_message_id IS NULL))
);

CREATE TABLE IF NOT EXISTS stable_handoff_attempt (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  handoff_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  command_digest TEXT NOT NULL CHECK(length(command_digest)=64 AND command_digest NOT GLOB '*[^0-9a-f]*'),
  lease_id TEXT NOT NULL,
  machine_id TEXT NOT NULL,
  fence_token TEXT NOT NULL,
  state TEXT NOT NULL CHECK(state IN ('working','completed','failed','canceled')),
  started_at TEXT NOT NULL,
  lease_expires_at TEXT NOT NULL,
  terminal_receipt_id TEXT UNIQUE,
  completed_at TEXT,
  UNIQUE(account_id,id),
  UNIQUE(account_id,idempotency_key),
  UNIQUE(account_id,handoff_id,lease_id),
  UNIQUE(account_id,handoff_id,id,lease_id,fence_token),
  FOREIGN KEY(account_id,handoff_id) REFERENCES stable_handoff(account_id,id),
  CHECK((state='working' AND terminal_receipt_id IS NULL AND completed_at IS NULL) OR
        (state<>'working' AND terminal_receipt_id IS NOT NULL AND completed_at IS NOT NULL)),
  CHECK(lease_expires_at>started_at)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_stable_handoff_one_working_attempt
  ON stable_handoff_attempt(account_id,handoff_id) WHERE state='working';

CREATE TABLE IF NOT EXISTS stable_handoff_completion (
  account_id TEXT NOT NULL,
  handoff_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  command_digest TEXT NOT NULL CHECK(length(command_digest)=64 AND command_digest NOT GLOB '*[^0-9a-f]*'),
  message_id INTEGER NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  PRIMARY KEY(account_id,handoff_id),
  UNIQUE(account_id,idempotency_key),
  FOREIGN KEY(account_id,handoff_id) REFERENCES stable_handoff(account_id,id),
  FOREIGN KEY(message_id) REFERENCES conversation_message(id)
);

CREATE TRIGGER IF NOT EXISTS group_membership_revision_immutable_update
BEFORE UPDATE ON group_membership_revision BEGIN SELECT RAISE(ABORT,'group_membership_immutable'); END;
CREATE TRIGGER IF NOT EXISTS group_membership_revision_immutable_delete
BEFORE DELETE ON group_membership_revision BEGIN SELECT RAISE(ABORT,'group_membership_immutable'); END;
CREATE TRIGGER IF NOT EXISTS group_member_revision_immutable_update
BEFORE UPDATE ON group_member_revision BEGIN SELECT RAISE(ABORT,'group_membership_immutable'); END;
CREATE TRIGGER IF NOT EXISTS group_member_revision_immutable_delete
BEFORE DELETE ON group_member_revision BEGIN SELECT RAISE(ABORT,'group_membership_immutable'); END;
CREATE TRIGGER IF NOT EXISTS group_member_binding_immutable_update
BEFORE UPDATE ON group_member_binding BEGIN SELECT RAISE(ABORT,'group_membership_immutable'); END;
CREATE TRIGGER IF NOT EXISTS group_member_binding_immutable_delete
BEFORE DELETE ON group_member_binding BEGIN SELECT RAISE(ABORT,'group_membership_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_group_identity_immutable
BEFORE UPDATE OF id,account_id,conversation_id,created_at ON stable_group
BEGIN SELECT RAISE(ABORT,'stable_group_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_group_delete_immutable
BEFORE DELETE ON stable_group BEGIN SELECT RAISE(ABORT,'stable_group_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_group_create_idempotency_immutable_update
BEFORE UPDATE ON stable_group_create_idempotency BEGIN SELECT RAISE(ABORT,'stable_group_idempotency_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_group_create_idempotency_immutable_delete
BEFORE DELETE ON stable_group_create_idempotency BEGIN SELECT RAISE(ABORT,'stable_group_idempotency_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_group_lifecycle_event_immutable_update
BEFORE UPDATE ON stable_group_lifecycle_event BEGIN SELECT RAISE(ABORT,'stable_group_event_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_group_lifecycle_event_immutable_delete
BEFORE DELETE ON stable_group_lifecycle_event BEGIN SELECT RAISE(ABORT,'stable_group_event_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_group_turn_identity_immutable
BEFORE UPDATE ON stable_group_turn BEGIN SELECT RAISE(ABORT,'stable_group_turn_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_group_turn_delete_immutable
BEFORE DELETE ON stable_group_turn BEGIN SELECT RAISE(ABORT,'stable_group_turn_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_group_turn_recipient_immutable_update
BEFORE UPDATE ON stable_group_turn_recipient BEGIN SELECT RAISE(ABORT,'stable_group_turn_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_group_turn_recipient_immutable_delete
BEFORE DELETE ON stable_group_turn_recipient BEGIN SELECT RAISE(ABORT,'stable_group_turn_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_group_initial_target_identity_immutable
BEFORE UPDATE OF id,account_id,turn_id,wave,agent_id,behavior_revision_id,binding_revision_id,participant_id,created_at
ON stable_group_initial_target BEGIN SELECT RAISE(ABORT,'stable_group_target_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_group_initial_target_delete_immutable
BEFORE DELETE ON stable_group_initial_target BEGIN SELECT RAISE(ABORT,'stable_group_target_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_handoff_identity_immutable
BEFORE UPDATE OF id,account_id,idempotency_key,command_digest,group_turn_id,source_message_id,source_agent_id,
recipient_agent_id,recipient_behavior_revision_id,recipient_binding_revision_id,source_conversation_id,
output_conversation_id,context_json,effective_authority_json,command_json,created_at ON stable_handoff
BEGIN SELECT RAISE(ABORT,'stable_handoff_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_handoff_delete_immutable
BEFORE DELETE ON stable_handoff BEGIN SELECT RAISE(ABORT,'stable_handoff_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_handoff_target_identity_immutable
BEFORE UPDATE OF id,account_id,handoff_id,conversation_id,agent_id,behavior_revision_id,binding_revision_id,
participant_id,created_at ON stable_handoff_target
BEGIN SELECT RAISE(ABORT,'stable_handoff_target_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_handoff_target_delete_immutable
BEFORE DELETE ON stable_handoff_target BEGIN SELECT RAISE(ABORT,'stable_handoff_target_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_handoff_cancellation_immutable_update
BEFORE UPDATE ON stable_handoff_cancellation BEGIN SELECT RAISE(ABORT,'stable_handoff_cancellation_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_handoff_cancellation_immutable_delete
BEFORE DELETE ON stable_handoff_cancellation BEGIN SELECT RAISE(ABORT,'stable_handoff_cancellation_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_handoff_projection_identity_immutable
BEFORE UPDATE OF account_id,handoff_id,conversation_id,output_conversation_id,projected_at ON stable_handoff_projection
BEGIN SELECT RAISE(ABORT,'stable_handoff_projection_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_handoff_projection_delete_immutable
BEFORE DELETE ON stable_handoff_projection BEGIN SELECT RAISE(ABORT,'stable_handoff_projection_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_handoff_attempt_identity_immutable
BEFORE UPDATE OF id,account_id,handoff_id,idempotency_key,command_digest,lease_id,machine_id,fence_token,started_at,lease_expires_at
ON stable_handoff_attempt BEGIN SELECT RAISE(ABORT,'stable_handoff_attempt_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_handoff_attempt_delete_immutable
BEFORE DELETE ON stable_handoff_attempt BEGIN SELECT RAISE(ABORT,'stable_handoff_attempt_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_handoff_completion_immutable_update
BEFORE UPDATE ON stable_handoff_completion BEGIN SELECT RAISE(ABORT,'stable_handoff_completion_immutable'); END;
CREATE TRIGGER IF NOT EXISTS stable_handoff_completion_immutable_delete
BEFORE DELETE ON stable_handoff_completion BEGIN SELECT RAISE(ABORT,'stable_handoff_completion_immutable'); END;
`

func (s *Store) migrateCollaboration() error {
	if _, err := s.db.Exec(collaborationSchema); err != nil {
		return err
	}
	columns := []struct {
		name string
		typ  string
	}{
		{"cancellation_policy_id", "TEXT NOT NULL DEFAULT 'group-cancel:human-or-deadline'"},
		{"cancellation_policy_revision", "TEXT NOT NULL DEFAULT '1'"},
		{"approval_policy_id", "TEXT NOT NULL DEFAULT 'group-approval:explicit'"},
		{"approval_policy_revision", "TEXT NOT NULL DEFAULT '1'"},
		{"delegation_grant_json", `TEXT NOT NULL DEFAULT '{"id":"legacy:group-turn","permissions":[],"context_record_ids":[]}'`},
	}
	for _, column := range columns {
		if err := s.addColumn("stable_group_turn", column.name, column.typ); err != nil {
			return fmt.Errorf("store: migrate stable_group_turn.%s: %w", column.name, err)
		}
	}
	return nil
}

var _ ledger.CollaborationRepository = (*Store)(nil)

func (s *Store) CreateGroup(ctx context.Context, command ledger.CreateGroupCommand) (ledger.GroupRecord, error) {
	digest, err := command.Digest()
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	defer tx.Rollback()

	var existingDigest, existingGroupID string
	err = tx.QueryRowContext(ctx, `SELECT command_digest,group_id FROM stable_group_create_idempotency
WHERE account_id=? AND idempotency_key=?`, command.Group.AccountID, command.IdempotencyKey).Scan(&existingDigest, &existingGroupID)
	if err == nil {
		if existingDigest != digest {
			return ledger.GroupRecord{}, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.IdempotencyKey)
		}
		return getGroupRecord(ctx, tx, command.Group.AccountID, existingGroupID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ledger.GroupRecord{}, err
	}
	if err := command.Validate(); err != nil {
		return ledger.GroupRecord{}, err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO conversation(id,project_id,title,state,created_at,updated_at)
VALUES(?,?,?,?,?,?)`, command.Conversation.ID, nullableString(command.Conversation.ProjectID), command.Conversation.Title,
		command.Conversation.State, nowOr(command.Conversation.CreatedAt), nowOr(command.Conversation.UpdatedAt)); err != nil {
		return ledger.GroupRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_group(id,account_id,conversation_id,state,current_membership_revision_id,created_at)
VALUES(?,?,?,?,?,?)`, command.Group.ID, command.Group.AccountID, command.Group.ConversationID, command.Group.State,
		command.Group.CurrentMembershipRevisionID, nowOr(command.Group.CreatedAt)); err != nil {
		return ledger.GroupRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO group_membership_revision(id,account_id,group_id,revision,created_at)
VALUES(?,?,?,?,?)`, command.Membership.ID, command.Group.AccountID, command.Group.ID, command.Membership.Revision,
		nowOr(command.Membership.CreatedAt)); err != nil {
		return ledger.GroupRecord{}, err
	}
	for position, member := range command.Membership.Members {
		binding := command.MemberBindings[position]
		participant, err := groupParticipantForBinding(ctx, tx, command.Group.AccountID, command.Group.ConversationID, position, binding, command.Group.CreatedAt)
		if err != nil {
			return ledger.GroupRecord{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO group_member_revision(account_id,membership_revision_id,group_id,agent_id,position)
VALUES(?,?,?,?,?)`, command.Group.AccountID, command.Membership.ID, command.Group.ID, member.AgentID, member.Position); err != nil {
			return ledger.GroupRecord{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO conversation_participant(
id,conversation_id,seat_id,profile,agent,model,machine,display_name,position,state,created_at,removed_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,NULL)`, participant.ID, participant.ConversationID, participant.SeatID, participant.Profile,
			participant.Agent, nullableString(participant.Model), participant.Machine, participant.DisplayName,
			participant.Position, participant.State, nowOr(participant.CreatedAt)); err != nil {
			return ledger.GroupRecord{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO stable_agent_participant_evidence(
agent_id,binding_revision_id,conversation_id,participant_id,created_at) VALUES(?,?,?,?,?)`, binding.AgentID,
			binding.BindingRevisionID, command.Group.ConversationID, binding.ParticipantID, nowOr(command.Group.CreatedAt)); err != nil {
			return ledger.GroupRecord{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO group_member_binding(
account_id,membership_revision_id,group_id,conversation_id,agent_id,behavior_revision_id,binding_revision_id,participant_id
) VALUES(?,?,?,?,?,?,?,?)`, command.Group.AccountID, command.Membership.ID, command.Group.ID, command.Group.ConversationID,
			binding.AgentID, binding.BehaviorRevisionID, binding.BindingRevisionID, binding.ParticipantID); err != nil {
			return ledger.GroupRecord{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_group_create_idempotency(
account_id,idempotency_key,command_digest,group_id,created_at) VALUES(?,?,?,?,?)`, command.Group.AccountID,
		command.IdempotencyKey, digest, command.Group.ID, nowOr(command.Group.CreatedAt)); err != nil {
		return ledger.GroupRecord{}, err
	}
	record, err := getGroupRecord(ctx, tx, command.Group.AccountID, command.Group.ID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.GroupRecord{}, err
	}
	return record, nil
}

func (s *Store) GetGroup(ctx context.Context, accountID, groupID string) (ledger.GroupRecord, error) {
	return getGroupRecord(ctx, s.db, accountID, groupID)
}

func (s *Store) ListGroups(ctx context.Context, accountID string, state conversation.ConversationState) ([]ledger.GroupRecord, error) {
	if strings.TrimSpace(accountID) == "" {
		return nil, fmt.Errorf("Group account id is required")
	}
	if state == "" {
		state = conversation.ConversationOpen
	}
	if state != conversation.ConversationOpen && state != conversation.ConversationArchived {
		return nil, fmt.Errorf("Group state must be open or archived")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM stable_group WHERE account_id=? AND state=? ORDER BY created_at,id`, accountID, state)
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
	records := make([]ledger.GroupRecord, 0, len(ids))
	for _, id := range ids {
		record, err := getGroupRecord(ctx, s.db, accountID, id)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func (s *Store) RenameGroup(ctx context.Context, command ledger.RenameGroupCommand) (ledger.GroupRecord, error) {
	if err := command.Validate(); err != nil {
		return ledger.GroupRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	defer tx.Rollback()
	claimed, err := claimAgentLifecycleIdempotency(ctx, tx, command.AccountID, "group.rename",
		command.IdempotencyKey, digest, command.GroupID, command.ChangedAt)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if !claimed {
		return getGroupRecord(ctx, tx, command.AccountID, command.GroupID)
	}
	var conversationID, currentTitle string
	err = tx.QueryRowContext(ctx, `SELECT group_item.conversation_id,conversation.title
FROM stable_group group_item JOIN conversation ON conversation.id=group_item.conversation_id
WHERE group_item.account_id=? AND group_item.id=?`, command.AccountID, command.GroupID).Scan(&conversationID, &currentTitle)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.GroupRecord{}, fmt.Errorf("%w: Group %q", ledger.ErrNotFound, command.GroupID)
	}
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if currentTitle != command.ExpectedTitle {
		return ledger.GroupRecord{}, fmt.Errorf("%w: Group title", ledger.ErrRevisionConflict)
	}
	active, err := sqliteGroupHasActiveWork(ctx, tx, command.AccountID, command.GroupID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if active {
		return ledger.GroupRecord{}, fmt.Errorf("%w: Group has an active Turn, target, or Handoff", ledger.ErrStateConflict)
	}
	result, err := tx.ExecContext(ctx, `UPDATE conversation SET title=?,updated_at=? WHERE id=? AND title=?`,
		command.Title, nowOr(command.ChangedAt), conversationID, command.ExpectedTitle)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return ledger.GroupRecord{}, err
	} else if changed != 1 {
		return ledger.GroupRecord{}, fmt.Errorf("%w: Group title", ledger.ErrRevisionConflict)
	}
	if err := insertSQLiteGroupLifecycleEvent(ctx, tx, command.AccountID, command.GroupID, "group.renamed",
		map[string]any{"previous_title": command.ExpectedTitle, "title": command.Title, "changed_by": command.ChangedBy},
		command.ChangedAt); err != nil {
		return ledger.GroupRecord{}, err
	}
	record, err := getGroupRecord(ctx, tx, command.AccountID, command.GroupID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.GroupRecord{}, err
	}
	return record, nil
}

func (s *Store) SetGroupState(ctx context.Context, command ledger.SetGroupStateCommand) (ledger.GroupRecord, error) {
	if err := command.Validate(); err != nil {
		return ledger.GroupRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	defer tx.Rollback()
	claimed, err := claimAgentLifecycleIdempotency(ctx, tx, command.AccountID, "group.state",
		command.IdempotencyKey, digest, command.GroupID, command.ChangedAt)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if !claimed {
		return getGroupRecord(ctx, tx, command.AccountID, command.GroupID)
	}
	var conversationID string
	var groupState, conversationState conversation.ConversationState
	err = tx.QueryRowContext(ctx, `SELECT group_item.conversation_id,group_item.state,conversation.state
FROM stable_group group_item JOIN conversation ON conversation.id=group_item.conversation_id
WHERE group_item.account_id=? AND group_item.id=?`, command.AccountID, command.GroupID).Scan(
		&conversationID, &groupState, &conversationState)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.GroupRecord{}, fmt.Errorf("%w: Group %q", ledger.ErrNotFound, command.GroupID)
	}
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if groupState != conversationState || groupState != command.ExpectedState {
		return ledger.GroupRecord{}, fmt.Errorf("%w: Group state", ledger.ErrStateConflict)
	}
	active, err := sqliteGroupHasActiveWork(ctx, tx, command.AccountID, command.GroupID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if active {
		return ledger.GroupRecord{}, fmt.Errorf("%w: Group has an active Turn, target, or Handoff", ledger.ErrStateConflict)
	}
	result, err := tx.ExecContext(ctx, `UPDATE stable_group SET state=? WHERE account_id=? AND id=? AND state=?`,
		command.State, command.AccountID, command.GroupID, command.ExpectedState)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return ledger.GroupRecord{}, err
	} else if changed != 1 {
		return ledger.GroupRecord{}, fmt.Errorf("%w: Group state", ledger.ErrStateConflict)
	}
	result, err = tx.ExecContext(ctx, `UPDATE conversation SET state=?,updated_at=? WHERE id=? AND state=?`,
		command.State, nowOr(command.ChangedAt), conversationID, command.ExpectedState)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return ledger.GroupRecord{}, err
	} else if changed != 1 {
		return ledger.GroupRecord{}, fmt.Errorf("%w: Group Conversation state", ledger.ErrStateConflict)
	}
	if err := insertSQLiteGroupLifecycleEvent(ctx, tx, command.AccountID, command.GroupID, "group.state_changed",
		map[string]any{"previous_state": command.ExpectedState, "state": command.State, "changed_by": command.ChangedBy},
		command.ChangedAt); err != nil {
		return ledger.GroupRecord{}, err
	}
	record, err := getGroupRecord(ctx, tx, command.AccountID, command.GroupID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.GroupRecord{}, err
	}
	return record, nil
}

func (s *Store) ReplaceGroupMembers(ctx context.Context, command ledger.ReplaceGroupMembersCommand) (ledger.GroupRecord, error) {
	if err := command.Validate(); err != nil {
		return ledger.GroupRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	defer tx.Rollback()
	claimed, err := claimAgentLifecycleIdempotency(ctx, tx, command.AccountID, "group.members.replace",
		command.IdempotencyKey, digest, command.GroupID, command.ChangedAt)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if !claimed {
		return getGroupRecord(ctx, tx, command.AccountID, command.GroupID)
	}
	current, err := getGroupRecord(ctx, tx, command.AccountID, command.GroupID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if current.Group.State != conversation.ConversationOpen {
		return ledger.GroupRecord{}, fmt.Errorf("%w: archived Group", ledger.ErrStateConflict)
	}
	if current.Membership.ID != command.ExpectedMembershipRevisionID || command.Membership.Revision != current.Membership.Revision+1 {
		return ledger.GroupRecord{}, fmt.Errorf("%w: Group Membership Revision", ledger.ErrRevisionConflict)
	}
	active, err := sqliteGroupHasActiveWork(ctx, tx, command.AccountID, command.GroupID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if active {
		return ledger.GroupRecord{}, fmt.Errorf("%w: Group has an active Turn, target, or Handoff", ledger.ErrStateConflict)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO group_membership_revision(id,account_id,group_id,revision,created_at)
VALUES(?,?,?,?,?)`, command.Membership.ID, command.AccountID, command.GroupID, command.Membership.Revision,
		nowOr(command.Membership.CreatedAt)); err != nil {
		return ledger.GroupRecord{}, err
	}
	for position, member := range command.Membership.Members {
		binding := command.MemberBindings[position]
		participant, err := groupParticipantForBinding(ctx, tx, command.AccountID, current.Group.ConversationID,
			member.Position, binding, command.ChangedAt)
		if err != nil {
			return ledger.GroupRecord{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO group_member_revision(account_id,membership_revision_id,group_id,agent_id,position)
VALUES(?,?,?,?,?)`, command.AccountID, command.Membership.ID, command.GroupID, member.AgentID, member.Position); err != nil {
			return ledger.GroupRecord{}, err
		}
		var existingParticipantID string
		err = tx.QueryRowContext(ctx, `SELECT participant_id FROM stable_agent_participant_evidence
WHERE agent_id=? AND binding_revision_id=? AND conversation_id=?`, member.AgentID, binding.BindingRevisionID,
			current.Group.ConversationID).Scan(&existingParticipantID)
		switch {
		case err == nil && existingParticipantID != binding.ParticipantID:
			return ledger.GroupRecord{}, fmt.Errorf("Group member %q participant evidence does not match its current Binding", member.AgentID)
		case err == nil:
		case errors.Is(err, sql.ErrNoRows):
			if _, err := tx.ExecContext(ctx, `INSERT INTO conversation_participant(
id,conversation_id,seat_id,profile,agent,model,machine,display_name,position,state,created_at,removed_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,NULL)`, participant.ID, participant.ConversationID, participant.SeatID, participant.Profile,
				participant.Agent, nullableString(participant.Model), participant.Machine, participant.DisplayName,
				participant.Position, participant.State, nowOr(participant.CreatedAt)); err != nil {
				return ledger.GroupRecord{}, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO stable_agent_participant_evidence(
agent_id,binding_revision_id,conversation_id,participant_id,created_at) VALUES(?,?,?,?,?)`, member.AgentID,
				binding.BindingRevisionID, current.Group.ConversationID, binding.ParticipantID, nowOr(command.ChangedAt)); err != nil {
				return ledger.GroupRecord{}, err
			}
		default:
			return ledger.GroupRecord{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO group_member_binding(
account_id,membership_revision_id,group_id,conversation_id,agent_id,behavior_revision_id,binding_revision_id,participant_id
) VALUES(?,?,?,?,?,?,?,?)`, command.AccountID, command.Membership.ID, command.GroupID, current.Group.ConversationID,
			binding.AgentID, binding.BehaviorRevisionID, binding.BindingRevisionID, binding.ParticipantID); err != nil {
			return ledger.GroupRecord{}, err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE stable_group SET current_membership_revision_id=?
WHERE account_id=? AND id=? AND current_membership_revision_id=? AND state='open'`, command.Membership.ID,
		command.AccountID, command.GroupID, command.ExpectedMembershipRevisionID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return ledger.GroupRecord{}, err
	} else if changed != 1 {
		return ledger.GroupRecord{}, fmt.Errorf("%w: Group Membership Revision", ledger.ErrRevisionConflict)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE conversation SET updated_at=? WHERE id=?`,
		nowOr(command.ChangedAt), current.Group.ConversationID); err != nil {
		return ledger.GroupRecord{}, err
	}
	agentIDs := make([]string, 0, len(command.Membership.Members))
	for _, member := range command.Membership.Members {
		agentIDs = append(agentIDs, member.AgentID)
	}
	if err := insertSQLiteGroupLifecycleEvent(ctx, tx, command.AccountID, command.GroupID, "group.members_replaced",
		map[string]any{"previous_membership_revision_id": command.ExpectedMembershipRevisionID,
			"membership_revision_id": command.Membership.ID, "agent_ids": agentIDs, "changed_by": command.ChangedBy},
		command.ChangedAt); err != nil {
		return ledger.GroupRecord{}, err
	}
	record, err := getGroupRecord(ctx, tx, command.AccountID, command.GroupID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.GroupRecord{}, err
	}
	return record, nil
}

func sqliteGroupHasActiveWork(ctx context.Context, tx *sql.Tx, accountID, groupID string) (bool, error) {
	var active int
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(
  SELECT 1 FROM stable_group_turn turn_item
  WHERE turn_item.account_id=? AND turn_item.group_id=? AND (
    EXISTS (SELECT 1 FROM stable_group_initial_target target
      WHERE target.account_id=turn_item.account_id AND target.turn_id=turn_item.id
        AND target.state IN ('queued','working'))
    OR EXISTS (SELECT 1 FROM stable_handoff handoff
      WHERE handoff.account_id=turn_item.account_id AND handoff.group_turn_id=turn_item.id
        AND handoff.state IN ('queued','needs_you','working'))
  )
)`, accountID, groupID).Scan(&active)
	return active != 0, err
}

func insertSQLiteGroupLifecycleEvent(ctx context.Context, tx *sql.Tx, accountID, groupID, eventType string,
	metadata any, createdAt time.Time) error {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO stable_group_lifecycle_event(
account_id,group_id,type,metadata_json,created_at) VALUES(?,?,?,?,?)`, accountID, groupID, eventType,
		string(payload), nowOr(createdAt))
	return err
}

func (s *Store) SendGroupTurn(ctx context.Context, command ledger.SendGroupTurnCommand) (ledger.GroupTurnRecord, error) {
	digest, err := command.Digest()
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	defer tx.Rollback()

	var existingID, existingDigest string
	err = tx.QueryRowContext(ctx, `SELECT id,command_digest FROM stable_group_turn WHERE account_id=? AND idempotency_key=?`,
		command.AccountID, command.Envelope.IdempotencyKey).Scan(&existingID, &existingDigest)
	if err == nil {
		if existingDigest != digest {
			return ledger.GroupTurnRecord{}, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.Envelope.IdempotencyKey)
		}
		return getGroupTurnRecord(ctx, tx, command.AccountID, existingID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ledger.GroupTurnRecord{}, err
	}
	group, err := getGroupRecord(ctx, tx, command.AccountID, command.Envelope.GroupID)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	if err := command.Validate(group.Group, group.Membership); err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	bindings := make(map[string]conversation.GroupRecipient, len(group.MemberBindings))
	for _, binding := range group.MemberBindings {
		bindings[binding.AgentID] = binding
	}
	for _, recipient := range command.Envelope.Recipients {
		persisted, exists := bindings[recipient.AgentID]
		if !exists || persisted != recipient {
			return ledger.GroupTurnRecord{}, fmt.Errorf("Group recipient %q does not match persisted membership binding evidence", recipient.AgentID)
		}
	}
	targets, err := command.Envelope.InitialTargets(group.Group, group.Membership)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	created := nowOr(command.Envelope.CreatedAt)
	result, err := tx.ExecContext(ctx, `INSERT INTO conversation_message(
conversation_id,turn_id,target_id,author_kind,author_id,body,created_at) VALUES(?,?,NULL,?,?,?,?)`,
		command.Envelope.ConversationID, command.Envelope.ID, conversation.AuthorHuman, command.HumanID, command.Body, created)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	messageID, err := result.LastInsertId()
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	contextRows, err := tx.QueryContext(ctx, `SELECT id FROM conversation_message
WHERE conversation_id=? AND id<=? ORDER BY id LIMIT 257`, command.Envelope.ConversationID, messageID)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	contextRecordIDs := make([]string, 0)
	for contextRows.Next() {
		var frozenMessageID int64
		if err := contextRows.Scan(&frozenMessageID); err != nil {
			contextRows.Close()
			return ledger.GroupTurnRecord{}, err
		}
		contextRecordIDs = append(contextRecordIDs, "message:"+strconv.FormatInt(frozenMessageID, 10))
	}
	if err := contextRows.Err(); err != nil {
		contextRows.Close()
		return ledger.GroupTurnRecord{}, err
	}
	if err := contextRows.Close(); err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	if len(contextRecordIDs) == 0 || len(contextRecordIDs) > 256 ||
		contextRecordIDs[len(contextRecordIDs)-1] != "message:"+strconv.FormatInt(messageID, 10) {
		return ledger.GroupTurnRecord{}, fmt.Errorf("Group context snapshot is incomplete")
	}
	command.Envelope.RootDelegationGrant.ContextRecordIDs = contextRecordIDs
	delegationGrantJSON, err := json.Marshal(command.Envelope.RootDelegationGrant)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_group_turn(
id,account_id,group_id,conversation_id,client_turn_id,idempotency_key,command_digest,membership_revision_id,
selection,context_snapshot_id,delegation_grant_json,concurrency_policy,cancellation_policy_id,cancellation_policy_revision,
approval_policy_id,approval_policy_revision,max_agent_messages,max_handoff_depth,cost_limit_class,
cost_limit_evidence_id,token_limit_class,token_limit_evidence_id,deadline,prompt_message_id,created_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, command.Envelope.ID, command.AccountID, command.Envelope.GroupID,
		command.Envelope.ConversationID, command.Envelope.ClientTurnID, command.Envelope.IdempotencyKey, digest,
		command.Envelope.MembershipRevisionID, command.Envelope.Selection, command.Envelope.ContextSnapshotID,
		string(delegationGrantJSON), command.Envelope.ConcurrencyPolicy,
		command.Envelope.CancellationPolicyID, command.Envelope.CancellationPolicyRevision,
		command.Envelope.ApprovalPolicyID, command.Envelope.ApprovalPolicyRevision,
		command.Envelope.MaxAgentMessages, command.Envelope.MaxHandoffDepth,
		command.Envelope.CostLimitClass, nullableString(command.Envelope.CostLimitEvidenceID), command.Envelope.TokenLimitClass,
		nullableString(command.Envelope.TokenLimitEvidenceID), nowOr(command.Envelope.Deadline), messageID, created); err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	for position, recipient := range command.Envelope.Recipients {
		if _, err := tx.ExecContext(ctx, `INSERT INTO stable_group_turn_recipient(
account_id,turn_id,membership_revision_id,position,agent_id,behavior_revision_id,binding_revision_id,participant_id
) VALUES(?,?,?,?,?,?,?,?)`, command.AccountID, command.Envelope.ID, command.Envelope.MembershipRevisionID, position,
			recipient.AgentID, recipient.BehaviorRevisionID, recipient.BindingRevisionID, recipient.ParticipantID); err != nil {
			return ledger.GroupTurnRecord{}, err
		}
		target := targets[position]
		if _, err := tx.ExecContext(ctx, `INSERT INTO stable_group_initial_target(
id,account_id,turn_id,wave,agent_id,behavior_revision_id,binding_revision_id,participant_id,state,created_at
) VALUES(?,?,?,?,?,?,?,?,?,?)`, command.TargetIDs[position], command.AccountID, command.Envelope.ID, target.Wave,
			target.AgentID, target.BehaviorRevisionID, target.BindingRevisionID, target.ParticipantID,
			conversation.TargetQueued, created); err != nil {
			return ledger.GroupTurnRecord{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE conversation SET updated_at=? WHERE id=?`, created, command.Envelope.ConversationID); err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	record, err := getGroupTurnRecord(ctx, tx, command.AccountID, command.Envelope.ID)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	return record, nil
}

func (s *Store) ListGroupTurns(ctx context.Context, accountID, groupID string) ([]ledger.GroupTurnRecord, error) {
	if _, err := getGroupRecord(ctx, s.db, accountID, groupID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM stable_group_turn WHERE account_id=? AND group_id=? ORDER BY created_at,id`, accountID, groupID)
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
	records := make([]ledger.GroupTurnRecord, 0, len(ids))
	for _, id := range ids {
		record, err := getGroupTurnRecord(ctx, s.db, accountID, id)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func (s *Store) CreateHumanHandoff(ctx context.Context, command ledger.CreateHumanHandoffCommand) (ledger.HandoffRecord, error) {
	digest, err := command.Digest()
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	var existingID, existingDigest string
	err = s.db.QueryRowContext(ctx, `SELECT id,command_digest FROM stable_handoff
WHERE account_id=? AND idempotency_key=?`, command.AccountID, command.IdempotencyKey).Scan(&existingID, &existingDigest)
	if err == nil {
		if existingDigest != digest {
			return ledger.HandoffRecord{}, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.IdempotencyKey)
		}
		return s.GetHandoff(ctx, command.AccountID, existingID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ledger.HandoffRecord{}, err
	}
	if err := command.Validate(); err != nil {
		return ledger.HandoffRecord{}, err
	}
	recipient, err := s.GetAgent(ctx, command.AccountID, command.RecipientAgentID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	if recipient.Agent.State != conversation.AgentOpen {
		return ledger.HandoffRecord{}, fmt.Errorf("%w: recipient Agent is not open", ledger.ErrStateConflict)
	}
	manifest := conversation.ContextManifest{References: make([]conversation.ContextReference, 0, len(command.ContextMessageIDs))}
	contextIDs := make([]string, 0, len(command.ContextMessageIDs))
	for _, messageID := range command.ContextMessageIDs {
		reference := conversation.ContextReference{Kind: conversation.ContextMessage, ID: messageID,
			AccountID: command.AccountID, Immutable: true}
		manifest.References = append(manifest.References, reference)
		contextIDs = append(contextIDs, reference.Key())
	}
	root := conversation.AuthorityGrant{ID: command.RootDelegationGrantID, Permissions: []string{}, ContextRecordIDs: contextIDs}
	recipientBehaviorID := recipient.Behavior.ID
	recipientBindingID := recipient.Binding.ID
	participantID := recipient.Participant.ID
	outputConversationID := recipient.Home.ID
	policyID, policyRevision := recipient.Binding.PolicyID, recipient.Binding.PolicyRevision
	maxMessages, maxDepth := conversation.MaxGroupAgentMessages, conversation.MaxGroupHandoffDepth
	deadline := command.HardDeadline
	groupTurnID := ""
	requireCurrentRecipient := true
	sourceMessageID, err := canonicalMessageID(command.SourceMessageID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	err = s.db.QueryRowContext(ctx, `SELECT turn.id
FROM conversation_message message JOIN stable_group_turn turn ON turn.id=message.turn_id
WHERE message.id=? AND message.conversation_id=? AND turn.account_id=?`, sourceMessageID,
		command.SourceConversationID, command.AccountID).Scan(&groupTurnID)
	if err == nil {
		groupTurn, err := getGroupTurnRecord(ctx, s.db, command.AccountID, groupTurnID)
		if err != nil {
			return ledger.HandoffRecord{}, err
		}
		if !command.HardDeadline.Equal(groupTurn.Envelope.Deadline) {
			return ledger.HandoffRecord{}, fmt.Errorf("%w: Group Handoff must preserve its Group Turn deadline", ledger.ErrStateConflict)
		}
		var groupRecipient *conversation.GroupRecipient
		for index := range groupTurn.Recipients {
			if groupTurn.Recipients[index].AgentID == command.RecipientAgentID {
				binding := groupTurn.Recipients[index]
				groupRecipient = &binding
				break
			}
		}
		if groupRecipient == nil {
			return ledger.HandoffRecord{}, fmt.Errorf("%w: recipient Agent is not in the persisted Group Turn", ledger.ErrNotFound)
		}
		allowed := make(map[string]struct{}, len(groupTurn.Envelope.RootDelegationGrant.ContextRecordIDs))
		for _, contextID := range groupTurn.Envelope.RootDelegationGrant.ContextRecordIDs {
			allowed[contextID] = struct{}{}
		}
		for _, contextID := range contextIDs {
			if _, ok := allowed[contextID]; !ok {
				return ledger.HandoffRecord{}, fmt.Errorf("Handoff context reference %q is not authorized by its Group Turn root grant", contextID)
			}
		}
		root = groupTurn.Envelope.RootDelegationGrant
		recipientBehaviorID = groupRecipient.BehaviorRevisionID
		recipientBindingID = groupRecipient.BindingRevisionID
		participantID = groupRecipient.ParticipantID
		outputConversationID = groupTurn.Envelope.ConversationID
		maxMessages, maxDepth = groupTurn.Envelope.MaxAgentMessages, groupTurn.Envelope.MaxHandoffDepth
		deadline = groupTurn.Envelope.Deadline
		requireCurrentRecipient = false
		if err := s.db.QueryRowContext(ctx, `SELECT policy_id,policy_revision FROM agent_binding_revision
WHERE id=? AND agent_id=?`, recipientBindingID, command.RecipientAgentID).Scan(&policyID, &policyRevision); err != nil {
			return ledger.HandoffRecord{}, err
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ledger.HandoffRecord{}, err
	} else if recipient.Home.State != conversation.ConversationOpen ||
		recipient.Agent.CurrentBehaviorRevisionID != recipient.Behavior.ID ||
		recipient.Agent.CurrentBindingRevisionID != recipient.Binding.ID ||
		recipient.Binding.BehaviorRevisionID != recipient.Behavior.ID ||
		recipient.Participant.ConversationID != recipient.Home.ID {
		return ledger.HandoffRecord{}, fmt.Errorf("%w: recipient Agent is not open with exact current Home evidence", ledger.ErrStateConflict)
	}
	handoffPolicy := conversation.AuthorityGrant{ID: "policy:handoff:human:1", Permissions: []string{}}
	recipientPolicy := conversation.AuthorityGrant{
		ID:          "policy:binding:" + policyID + ":" + policyRevision + ":" + recipientBindingID,
		Permissions: []string{},
	}
	effective, err := conversation.ComputeEffectiveAuthority([]string{}, root, handoffPolicy, recipientPolicy)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	handoff := conversation.Handoff{
		ID: command.HandoffID, AccountID: command.AccountID, IdempotencyKey: command.IdempotencyKey,
		State: conversation.HandoffQueued, CreatedByKind: conversation.HandoffActorHuman, CreatedByID: command.CreatedByID,
		SourceMessageID: command.SourceMessageID, RecipientAgentID: command.RecipientAgentID,
		RecipientBehaviorRevisionID: recipientBehaviorID, RecipientBindingRevisionID: recipientBindingID,
		SourceConversationID: command.SourceConversationID, OutputConversationID: outputConversationID,
		Context: manifest, RequestedResult: command.RequestedResult, ReplyToMessageID: command.ReplyToMessageID,
		RootDelegationGrant: root, HandoffPolicy: handoffPolicy, RecipientBindingPolicy: recipientPolicy,
		RequestedAuthority: []string{}, EffectiveAuthority: effective, BudgetClass: conversation.LimitUnknown,
		MaxAgentMessages: maxMessages, MaxDepth: maxDepth, GroupTurnID: groupTurnID,
		Depth: 1, Deadline: deadline, AncestorAgentIDs: []string{}, CreatedAt: command.CreatedAt,
	}
	projections := []string{}
	if handoff.SourceConversationID != handoff.OutputConversationID {
		projections = append(projections, handoff.SourceConversationID)
	}
	return s.AcceptHandoff(ctx, ledger.AcceptHandoffCommand{
		Handoff: handoff, TargetID: command.TargetID, ParticipantID: participantID,
		ProjectionConversationIDs: projections, RequireCurrentRecipient: requireCurrentRecipient,
	})
}

func (s *Store) AcceptHandoff(ctx context.Context, command ledger.AcceptHandoffCommand) (ledger.HandoffRecord, error) {
	digest, err := command.Digest()
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	defer tx.Rollback()

	var existingID, existingDigest string
	err = tx.QueryRowContext(ctx, `SELECT id,command_digest FROM stable_handoff WHERE account_id=? AND idempotency_key=?`,
		command.Handoff.AccountID, command.Handoff.IdempotencyKey).Scan(&existingID, &existingDigest)
	if err == nil {
		if existingDigest != digest {
			return ledger.HandoffRecord{}, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.Handoff.IdempotencyKey)
		}
		return getHandoffRecord(ctx, tx, command.Handoff.AccountID, existingID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ledger.HandoffRecord{}, err
	}
	if err := command.Validate(); err != nil {
		return ledger.HandoffRecord{}, err
	}
	sourceMessageID, err := canonicalMessageID(command.Handoff.SourceMessageID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	if err := validateHandoffEvidence(ctx, tx, command, sourceMessageID); err != nil {
		return ledger.HandoffRecord{}, err
	}
	handoffJSON, err := json.Marshal(command.Handoff)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	contextJSON, err := json.Marshal(command.Handoff.Context)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	authorityJSON, err := json.Marshal(command.Handoff.EffectiveAuthority)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	created := nowOr(command.Handoff.CreatedAt)
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_handoff(
id,account_id,idempotency_key,command_digest,state,group_turn_id,source_message_id,source_agent_id,
recipient_agent_id,recipient_behavior_revision_id,recipient_binding_revision_id,source_conversation_id,
output_conversation_id,context_json,effective_authority_json,command_json,created_at,completed_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,NULL)`, command.Handoff.ID, command.Handoff.AccountID,
		command.Handoff.IdempotencyKey, digest, command.Handoff.State, nullableString(command.Handoff.GroupTurnID),
		sourceMessageID, nullableString(command.Handoff.SourceAgentID), command.Handoff.RecipientAgentID,
		command.Handoff.RecipientBehaviorRevisionID, command.Handoff.RecipientBindingRevisionID,
		command.Handoff.SourceConversationID, command.Handoff.OutputConversationID, string(contextJSON), string(authorityJSON),
		string(handoffJSON), created); err != nil {
		return ledger.HandoffRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_handoff_target(
id,account_id,handoff_id,conversation_id,agent_id,behavior_revision_id,binding_revision_id,participant_id,state,created_at
) VALUES(?,?,?,?,?,?,?,?,?,?)`, command.TargetID, command.Handoff.AccountID, command.Handoff.ID,
		command.Handoff.OutputConversationID, command.Handoff.RecipientAgentID, command.Handoff.RecipientBehaviorRevisionID,
		command.Handoff.RecipientBindingRevisionID, command.ParticipantID, conversation.TargetQueued, created); err != nil {
		return ledger.HandoffRecord{}, err
	}
	projectionIDs := append([]string{}, command.ProjectionConversationIDs...)
	sort.Strings(projectionIDs)
	for _, conversationID := range projectionIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO stable_handoff_projection(
account_id,handoff_id,conversation_id,output_conversation_id,authoritative_message_id,state,projected_at
) VALUES(?,?,?,?,NULL,?,?)`, command.Handoff.AccountID, command.Handoff.ID, conversationID,
			command.Handoff.OutputConversationID, command.Handoff.State, created); err != nil {
			return ledger.HandoffRecord{}, err
		}
	}
	record, err := getHandoffRecord(ctx, tx, command.Handoff.AccountID, command.Handoff.ID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.HandoffRecord{}, err
	}
	return record, nil
}

func (s *Store) GetHandoff(ctx context.Context, accountID, handoffID string) (ledger.HandoffRecord, error) {
	return getHandoffRecord(ctx, s.db, accountID, handoffID)
}

func (s *Store) ListHandoffs(ctx context.Context, accountID string) ([]ledger.HandoffRecord, error) {
	if strings.TrimSpace(accountID) == "" {
		return nil, fmt.Errorf("Handoff account id is required")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM stable_handoff WHERE account_id=? ORDER BY created_at,id`, accountID)
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
	records := make([]ledger.HandoffRecord, 0, len(ids))
	for _, id := range ids {
		record, err := getHandoffRecord(ctx, s.db, accountID, id)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func (s *Store) CancelHandoff(ctx context.Context, command ledger.CancelHandoffCommand) (ledger.HandoffRecord, error) {
	if err := command.Validate(); err != nil {
		return ledger.HandoffRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	defer tx.Rollback()
	if resultID, replay, err := findSQLiteDirectIdempotency(ctx, tx, command.AccountID,
		"handoff.cancel", command.IdempotencyKey, digest); err != nil {
		return ledger.HandoffRecord{}, err
	} else if replay {
		if resultID != command.HandoffID {
			return ledger.HandoffRecord{}, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.IdempotencyKey)
		}
		record, err := getHandoffRecord(ctx, tx, command.AccountID, command.HandoffID)
		if err != nil {
			return ledger.HandoffRecord{}, err
		}
		if err := tx.Commit(); err != nil {
			return ledger.HandoffRecord{}, err
		}
		return record, nil
	}
	record, err := getHandoffRecord(ctx, tx, command.AccountID, command.HandoffID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	if record.Handoff.State != conversation.HandoffQueued && record.Handoff.State != conversation.HandoffNeedsYou &&
		record.Handoff.State != conversation.HandoffWorking {
		return ledger.HandoffRecord{}, fmt.Errorf("%w: Handoff cannot be canceled from %s", ledger.ErrStateConflict, record.Handoff.State)
	}
	cancellationState := ledger.HandoffCancellationRequested
	if record.Target.State == conversation.TargetQueued {
		if affected, err := tx.ExecContext(ctx, `UPDATE stable_handoff_target SET state=?
WHERE account_id=? AND handoff_id=? AND id=? AND state=?`, conversation.TargetCanceled, command.AccountID,
			command.HandoffID, record.Target.ID, conversation.TargetQueued); err != nil {
			return ledger.HandoffRecord{}, err
		} else if changed, err := affected.RowsAffected(); err != nil || changed != 1 {
			return ledger.HandoffRecord{}, fmt.Errorf("%w: Handoff target cancellation race", ledger.ErrStateConflict)
		}
		cancellationState = ledger.HandoffCancellationCanceled
	} else if record.Target.State != conversation.TargetWorking {
		return ledger.HandoffRecord{}, fmt.Errorf("%w: Handoff target cannot be canceled from %s", ledger.ErrStateConflict, record.Target.State)
	}
	requestedAt := nowOr(command.CanceledAt)
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_handoff_cancellation(
account_id,handoff_id,target_id,agent_id,behavior_revision_id,binding_revision_id,participant_id,state,requested_by,requested_at
) VALUES(?,?,?,?,?,?,?,?,?,?)`, command.AccountID, command.HandoffID, record.Target.ID, record.Target.AgentID,
		record.Target.BehaviorRevisionID, record.Target.BindingRevisionID, record.Target.ParticipantID,
		cancellationState, command.CanceledBy, requestedAt); err != nil {
		return ledger.HandoffRecord{}, err
	}
	if affected, err := tx.ExecContext(ctx, `UPDATE stable_handoff SET state=?,completed_at=?
WHERE account_id=? AND id=? AND state=?`, conversation.HandoffCanceled, requestedAt, command.AccountID,
		command.HandoffID, record.Handoff.State); err != nil {
		return ledger.HandoffRecord{}, err
	} else if changed, err := affected.RowsAffected(); err != nil || changed != 1 {
		return ledger.HandoffRecord{}, fmt.Errorf("%w: Handoff cancellation race", ledger.ErrStateConflict)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE stable_handoff_projection SET state=?
WHERE account_id=? AND handoff_id=?`, conversation.HandoffCanceled, command.AccountID, command.HandoffID); err != nil {
		return ledger.HandoffRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_agent_lifecycle_idempotency(
account_id,scope,idempotency_key,command_digest,result_id,created_at) VALUES(?,?,?,?,?,?)`, command.AccountID,
		"handoff.cancel", command.IdempotencyKey, digest, command.HandoffID, requestedAt); err != nil {
		return ledger.HandoffRecord{}, err
	}
	eventType := "handoff.cancel_requested"
	if cancellationState == ledger.HandoffCancellationCanceled {
		eventType = "handoff.canceled"
	}
	if err := insertSQLiteAgentLifecycleEvent(ctx, tx, command.AccountID, record.Handoff.RecipientAgentID,
		eventType, map[string]any{
			"handoff_id": command.HandoffID, "target_id": record.Target.ID,
			"behavior_revision_id": record.Target.BehaviorRevisionID,
			"binding_revision_id":  record.Target.BindingRevisionID,
			"participant_id":       record.Target.ParticipantID, "canceled_by": command.CanceledBy,
		}, command.CanceledAt); err != nil {
		return ledger.HandoffRecord{}, err
	}
	canceled, err := getHandoffRecord(ctx, tx, command.AccountID, command.HandoffID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.HandoffRecord{}, err
	}
	return canceled, nil
}

func (s *Store) StartHandoff(ctx context.Context, command ledger.StartHandoffCommand) (ledger.HandoffRecord, error) {
	if err := command.Validate(); err != nil {
		return ledger.HandoffRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	defer tx.Rollback()

	var existingHandoffID, existingAttemptID, existingDigest string
	err = tx.QueryRowContext(ctx, `SELECT handoff_id,id,command_digest FROM stable_handoff_attempt
WHERE account_id=? AND idempotency_key=?`, command.AccountID, command.IdempotencyKey).Scan(
		&existingHandoffID, &existingAttemptID, &existingDigest)
	if err == nil {
		if existingHandoffID != command.HandoffID || existingAttemptID != command.AttemptID || existingDigest != digest {
			return ledger.HandoffRecord{}, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.IdempotencyKey)
		}
		return getHandoffRecord(ctx, tx, command.AccountID, command.HandoffID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ledger.HandoffRecord{}, err
	}

	record, err := getHandoffRecord(ctx, tx, command.AccountID, command.HandoffID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	if record.Handoff.State != conversation.HandoffQueued || record.Target.State != conversation.TargetQueued {
		return ledger.HandoffRecord{}, fmt.Errorf("Handoff %q is not startable from state %q", command.HandoffID, record.Handoff.State)
	}
	var computerID, cloudRuntime string
	if err := tx.QueryRowContext(ctx, `SELECT computer_id,cloud_runtime FROM agent_binding_revision
WHERE id=? AND agent_id=?`, record.Handoff.RecipientBindingRevisionID, record.Handoff.RecipientAgentID).Scan(
		&computerID, &cloudRuntime); err != nil {
		return ledger.HandoffRecord{}, fmt.Errorf("load Handoff Binding Revision location: %w", err)
	}
	expectedMachine := computerID
	if expectedMachine == "" {
		expectedMachine = cloudRuntime
	}
	if command.MachineID != expectedMachine {
		return ledger.HandoffRecord{}, fmt.Errorf("Handoff machine does not match its pinned Binding Revision")
	}
	agentMessages, err := groupAgentMessageCount(ctx, tx, command.AccountID, record.Handoff.GroupTurnID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	if err := record.Handoff.CanStart(command.StartedAt, agentMessages); err != nil {
		if errors.Is(err, conversation.ErrHandoffNeedsYou) {
			if _, updateErr := tx.ExecContext(ctx, `UPDATE stable_handoff SET state=? WHERE account_id=? AND id=? AND state=?`,
				conversation.HandoffNeedsYou, command.AccountID, command.HandoffID, conversation.HandoffQueued); updateErr != nil {
				return ledger.HandoffRecord{}, updateErr
			}
			if _, updateErr := tx.ExecContext(ctx, `UPDATE stable_handoff_projection SET state=?
WHERE account_id=? AND handoff_id=? AND state=?`, conversation.HandoffNeedsYou, command.AccountID,
				command.HandoffID, conversation.HandoffQueued); updateErr != nil {
				return ledger.HandoffRecord{}, updateErr
			}
			if commitErr := tx.Commit(); commitErr != nil {
				return ledger.HandoffRecord{}, commitErr
			}
		}
		return ledger.HandoffRecord{}, err
	}
	started := nowOr(command.StartedAt)
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_handoff_attempt(
id,account_id,handoff_id,idempotency_key,command_digest,lease_id,machine_id,fence_token,state,
started_at,lease_expires_at,terminal_receipt_id,completed_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,NULL,NULL)`, command.AttemptID, command.AccountID, command.HandoffID,
		command.IdempotencyKey, digest, command.LeaseID, command.MachineID, command.FenceToken,
		ledger.HandoffAttemptWorking, started, nowOr(command.LeaseExpiresAt)); err != nil {
		return ledger.HandoffRecord{}, err
	}
	if result, err := tx.ExecContext(ctx, `UPDATE stable_handoff SET state=? WHERE account_id=? AND id=? AND state=?`,
		conversation.HandoffWorking, command.AccountID, command.HandoffID, conversation.HandoffQueued); err != nil {
		return ledger.HandoffRecord{}, err
	} else if changed, err := result.RowsAffected(); err != nil || changed != 1 {
		return ledger.HandoffRecord{}, fmt.Errorf("Handoff %q start lost its state precondition", command.HandoffID)
	}
	if result, err := tx.ExecContext(ctx, `UPDATE stable_handoff_target SET state=? WHERE account_id=? AND handoff_id=? AND state=?`,
		conversation.TargetWorking, command.AccountID, command.HandoffID, conversation.TargetQueued); err != nil {
		return ledger.HandoffRecord{}, err
	} else if changed, err := result.RowsAffected(); err != nil || changed != 1 {
		return ledger.HandoffRecord{}, fmt.Errorf("Handoff %q target start lost its state precondition", command.HandoffID)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE stable_handoff_projection SET state=? WHERE account_id=? AND handoff_id=? AND state=?`,
		conversation.HandoffWorking, command.AccountID, command.HandoffID, conversation.HandoffQueued); err != nil {
		return ledger.HandoffRecord{}, err
	}
	startedRecord, err := getHandoffRecord(ctx, tx, command.AccountID, command.HandoffID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.HandoffRecord{}, err
	}
	return startedRecord, nil
}

func (s *Store) CompleteHandoff(ctx context.Context, command ledger.CompleteHandoffCommand) (ledger.HandoffRecord, error) {
	if err := command.Validate(); err != nil {
		return ledger.HandoffRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	defer tx.Rollback()

	var existingHandoffID, existingDigest string
	err = tx.QueryRowContext(ctx, `SELECT handoff_id,command_digest FROM stable_handoff_completion
WHERE account_id=? AND idempotency_key=?`, command.AccountID, command.IdempotencyKey).Scan(&existingHandoffID, &existingDigest)
	if err == nil {
		if existingHandoffID != command.HandoffID || existingDigest != digest {
			return ledger.HandoffRecord{}, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.IdempotencyKey)
		}
		return getHandoffRecord(ctx, tx, command.AccountID, command.HandoffID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ledger.HandoffRecord{}, err
	}
	record, err := getHandoffRecord(ctx, tx, command.AccountID, command.HandoffID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	if record.Result != nil || record.Handoff.State == conversation.HandoffCompleted {
		return ledger.HandoffRecord{}, fmt.Errorf("%w: Handoff %q", ledger.ErrAlreadyCompleted, command.HandoffID)
	}
	if record.Handoff.State != conversation.HandoffWorking || record.Target.State != conversation.TargetWorking {
		return ledger.HandoffRecord{}, fmt.Errorf("Handoff %q is not completable from state %q", command.HandoffID, record.Handoff.State)
	}
	if record.Attempt == nil || record.Attempt.State != ledger.HandoffAttemptWorking ||
		record.Attempt.ID != command.AttemptID || record.Attempt.LeaseID != command.LeaseID ||
		record.Attempt.FenceToken != command.FenceToken {
		return ledger.HandoffRecord{}, fmt.Errorf("Handoff completion attempt or lease fence is stale")
	}
	if command.AuthorAgentID != record.Handoff.RecipientAgentID {
		return ledger.HandoffRecord{}, fmt.Errorf("Handoff result author must be the persisted recipient Agent")
	}
	if command.CreatedAt.Before(record.Attempt.StartedAt) || !command.CreatedAt.Before(record.Attempt.LeaseExpiresAt) {
		return ledger.HandoffRecord{}, fmt.Errorf("Handoff completion is outside its active lease")
	}
	created := nowOr(command.CreatedAt)
	messageResult, err := tx.ExecContext(ctx, `INSERT INTO conversation_message(
conversation_id,turn_id,target_id,author_kind,author_id,body,created_at) VALUES(?,?,?,?,?,?,?)`,
		record.Handoff.OutputConversationID, nullableString(record.Handoff.GroupTurnID), record.Target.ID,
		conversation.AuthorAssistant, command.AuthorAgentID, command.Body, created)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	messageID, err := messageResult.LastInsertId()
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	completedHandoff := record.Handoff
	completedHandoff.State = conversation.HandoffCompleted
	result := conversation.HandoffResult{
		HandoffID: command.HandoffID, OutputConversationID: completedHandoff.OutputConversationID,
		MessageID: strconv.FormatInt(messageID, 10), Body: command.Body,
	}
	if err := result.ValidateFor(completedHandoff); err != nil {
		return ledger.HandoffRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stable_handoff_completion(
account_id,handoff_id,idempotency_key,command_digest,message_id,created_at) VALUES(?,?,?,?,?,?)`,
		command.AccountID, command.HandoffID, command.IdempotencyKey, digest, messageID, created); err != nil {
		return ledger.HandoffRecord{}, err
	}
	if result, err := tx.ExecContext(ctx, `UPDATE stable_handoff_attempt SET state=?,terminal_receipt_id=?,completed_at=?
WHERE account_id=? AND handoff_id=? AND id=? AND lease_id=? AND fence_token=? AND state=?`,
		ledger.HandoffAttemptCompleted, command.TerminalReceiptID, created, command.AccountID, command.HandoffID,
		command.AttemptID, command.LeaseID, command.FenceToken, ledger.HandoffAttemptWorking); err != nil {
		return ledger.HandoffRecord{}, err
	} else if changed, err := result.RowsAffected(); err != nil || changed != 1 {
		return ledger.HandoffRecord{}, fmt.Errorf("Handoff %q terminal receipt lost its lease fence", command.HandoffID)
	}
	if result, err := tx.ExecContext(ctx, `UPDATE stable_handoff SET state=?,completed_at=? WHERE account_id=? AND id=? AND state=?`,
		conversation.HandoffCompleted, created, command.AccountID, command.HandoffID, conversation.HandoffWorking); err != nil {
		return ledger.HandoffRecord{}, err
	} else if changed, err := result.RowsAffected(); err != nil || changed != 1 {
		return ledger.HandoffRecord{}, fmt.Errorf("Handoff %q completion lost its state precondition", command.HandoffID)
	}
	if result, err := tx.ExecContext(ctx, `UPDATE stable_handoff_target SET state=? WHERE account_id=? AND handoff_id=? AND state=?`,
		conversation.TargetAnswered, command.AccountID, command.HandoffID, conversation.TargetWorking); err != nil {
		return ledger.HandoffRecord{}, err
	} else if changed, err := result.RowsAffected(); err != nil || changed != 1 {
		return ledger.HandoffRecord{}, fmt.Errorf("Handoff %q target completion lost its state precondition", command.HandoffID)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE stable_handoff_projection SET state=?,authoritative_message_id=?
WHERE account_id=? AND handoff_id=?`, conversation.HandoffCompleted, messageID, command.AccountID, command.HandoffID); err != nil {
		return ledger.HandoffRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE conversation SET updated_at=? WHERE id=?`, created, completedHandoff.OutputConversationID); err != nil {
		return ledger.HandoffRecord{}, err
	}
	completed, err := getHandoffRecord(ctx, tx, command.AccountID, command.HandoffID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.HandoffRecord{}, err
	}
	return completed, nil
}

type collaborationQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getGroupRecord(ctx context.Context, queryer collaborationQueryer, accountID, groupID string) (ledger.GroupRecord, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(groupID) == "" {
		return ledger.GroupRecord{}, fmt.Errorf("Group account id and id are required")
	}
	var record ledger.GroupRecord
	var groupCreated, conversationCreated, conversationUpdated string
	var projectID sql.NullString
	err := queryer.QueryRowContext(ctx, `SELECT g.id,g.account_id,g.conversation_id,g.state,g.current_membership_revision_id,g.created_at,
c.project_id,c.title,c.state,c.created_at,c.updated_at
FROM stable_group g JOIN conversation c ON c.id=g.conversation_id WHERE g.account_id=? AND g.id=?`, accountID, groupID).Scan(
		&record.Group.ID, &record.Group.AccountID, &record.Group.ConversationID, &record.Group.State,
		&record.Group.CurrentMembershipRevisionID, &groupCreated, &projectID, &record.Conversation.Title,
		&record.Conversation.State, &conversationCreated, &conversationUpdated)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.GroupRecord{}, fmt.Errorf("%w: Group %q", ledger.ErrNotFound, groupID)
	}
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	record.Group.CreatedAt = parseTime(groupCreated)
	record.Conversation.ID = record.Group.ConversationID
	record.Conversation.ProjectID = projectID.String
	record.Conversation.CreatedAt = parseTime(conversationCreated)
	record.Conversation.UpdatedAt = parseTime(conversationUpdated)
	var membershipCreated string
	err = queryer.QueryRowContext(ctx, `SELECT id,group_id,revision,created_at FROM group_membership_revision
WHERE account_id=? AND group_id=? AND id=?`, accountID, groupID, record.Group.CurrentMembershipRevisionID).Scan(
		&record.Membership.ID, &record.Membership.GroupID, &record.Membership.Revision, &membershipCreated)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	record.Membership.CreatedAt = parseTime(membershipCreated)
	record.Membership.Members = []conversation.GroupMember{}
	record.MemberBindings = []conversation.GroupRecipient{}
	rows, err := queryer.QueryContext(ctx, `SELECT m.agent_id,m.position,b.behavior_revision_id,b.binding_revision_id,b.participant_id
FROM group_member_revision m JOIN group_member_binding b
  ON b.account_id=m.account_id AND b.membership_revision_id=m.membership_revision_id AND b.agent_id=m.agent_id
WHERE m.account_id=? AND m.membership_revision_id=? ORDER BY m.position,m.agent_id`, accountID, record.Membership.ID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	for rows.Next() {
		var member conversation.GroupMember
		var binding conversation.GroupRecipient
		if err := rows.Scan(&member.AgentID, &member.Position, &binding.BehaviorRevisionID, &binding.BindingRevisionID, &binding.ParticipantID); err != nil {
			rows.Close()
			return ledger.GroupRecord{}, err
		}
		binding.AgentID = member.AgentID
		record.Membership.Members = append(record.Membership.Members, member)
		record.MemberBindings = append(record.MemberBindings, binding)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return ledger.GroupRecord{}, err
	}
	if err := rows.Close(); err != nil {
		return ledger.GroupRecord{}, err
	}
	return record, nil
}

func groupParticipantForBinding(ctx context.Context, queryer collaborationQueryer, accountID, conversationID string, position int,
	binding conversation.GroupRecipient, createdAt time.Time) (conversation.Participant, error) {
	var currentBehaviorID, currentBindingID, seatID, profile, provider, requestedModel, computerID, cloudRuntime, displayName string
	err := queryer.QueryRowContext(ctx, `SELECT a.current_behavior_revision_id,a.current_binding_revision_id,
b.seat_id,b.fort_profile,b.provider,b.requested_model,b.computer_id,b.cloud_runtime,p.name
FROM stable_agent a
JOIN agent_binding_revision b ON b.id=a.current_binding_revision_id AND b.agent_id=a.id
JOIN agent_profile_revision p ON p.id=a.current_profile_revision_id AND p.agent_id=a.id
WHERE a.account_id=? AND a.id=? AND a.state='open'`, accountID, binding.AgentID).Scan(
		&currentBehaviorID, &currentBindingID, &seatID, &profile, &provider, &requestedModel, &computerID, &cloudRuntime, &displayName)
	if errors.Is(err, sql.ErrNoRows) {
		return conversation.Participant{}, fmt.Errorf("%w: open Agent %q", ledger.ErrNotFound, binding.AgentID)
	}
	if err != nil {
		return conversation.Participant{}, err
	}
	if currentBehaviorID != binding.BehaviorRevisionID || currentBindingID != binding.BindingRevisionID {
		return conversation.Participant{}, fmt.Errorf("Group member %q binding evidence is not current", binding.AgentID)
	}
	machine := computerID
	if machine == "" {
		machine = cloudRuntime
	}
	return conversation.Participant{
		ID: binding.ParticipantID, ConversationID: conversationID, SeatID: seatID, Profile: profile,
		Agent: provider, Model: requestedModel, Machine: machine, DisplayName: displayName,
		Position: position, State: conversation.ParticipantActive, CreatedAt: createdAt,
	}, nil
}

func getGroupTurnRecord(ctx context.Context, queryer collaborationQueryer, accountID, turnID string) (ledger.GroupTurnRecord, error) {
	var record ledger.GroupTurnRecord
	var created, deadline, delegationGrantJSON string
	var costEvidence, tokenEvidence sql.NullString
	var messageID int64
	err := queryer.QueryRowContext(ctx, `SELECT id,group_id,conversation_id,client_turn_id,idempotency_key,membership_revision_id,
selection,context_snapshot_id,delegation_grant_json,concurrency_policy,cancellation_policy_id,cancellation_policy_revision,
approval_policy_id,approval_policy_revision,max_agent_messages,max_handoff_depth,cost_limit_class,
cost_limit_evidence_id,token_limit_class,token_limit_evidence_id,deadline,prompt_message_id,created_at
FROM stable_group_turn WHERE account_id=? AND id=?`, accountID, turnID).Scan(
		&record.Envelope.ID, &record.Envelope.GroupID, &record.Envelope.ConversationID, &record.Envelope.ClientTurnID,
		&record.Envelope.IdempotencyKey, &record.Envelope.MembershipRevisionID, &record.Envelope.Selection,
		&record.Envelope.ContextSnapshotID, &delegationGrantJSON, &record.Envelope.ConcurrencyPolicy,
		&record.Envelope.CancellationPolicyID, &record.Envelope.CancellationPolicyRevision,
		&record.Envelope.ApprovalPolicyID, &record.Envelope.ApprovalPolicyRevision, &record.Envelope.MaxAgentMessages,
		&record.Envelope.MaxHandoffDepth, &record.Envelope.CostLimitClass, &costEvidence, &record.Envelope.TokenLimitClass,
		&tokenEvidence, &deadline, &messageID, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.GroupTurnRecord{}, fmt.Errorf("%w: Group Turn %q", ledger.ErrNotFound, turnID)
	}
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	if err := json.Unmarshal([]byte(delegationGrantJSON), &record.Envelope.RootDelegationGrant); err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	record.Envelope.CostLimitEvidenceID = costEvidence.String
	record.Envelope.TokenLimitEvidenceID = tokenEvidence.String
	record.Envelope.Deadline = parseTime(deadline)
	record.Envelope.CreatedAt = parseTime(created)
	var messageCreated string
	var messageTurnID, targetID sql.NullString
	err = queryer.QueryRowContext(ctx, `SELECT id,conversation_id,turn_id,target_id,author_kind,author_id,body,created_at
FROM conversation_message WHERE id=? AND conversation_id=?`, messageID, record.Envelope.ConversationID).Scan(
		&record.Message.ID, &record.Message.ConversationID, &messageTurnID, &targetID, &record.Message.AuthorKind,
		&record.Message.AuthorID, &record.Message.Body, &messageCreated)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	record.Message.TurnID = messageTurnID.String
	record.Message.TargetID = targetID.String
	record.Message.CreatedAt = parseTime(messageCreated)
	record.Recipients = []conversation.GroupRecipient{}
	rows, err := queryer.QueryContext(ctx, `SELECT agent_id,behavior_revision_id,binding_revision_id,participant_id
FROM stable_group_turn_recipient WHERE account_id=? AND turn_id=? ORDER BY position,agent_id`, accountID, turnID)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	for rows.Next() {
		var recipient conversation.GroupRecipient
		if err := rows.Scan(&recipient.AgentID, &recipient.BehaviorRevisionID, &recipient.BindingRevisionID, &recipient.ParticipantID); err != nil {
			rows.Close()
			return ledger.GroupTurnRecord{}, err
		}
		record.Recipients = append(record.Recipients, recipient)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return ledger.GroupTurnRecord{}, err
	}
	if err := rows.Close(); err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	record.Envelope.Recipients = append([]conversation.GroupRecipient{}, record.Recipients...)
	record.InitialTargets = []ledger.InitialTargetRecord{}
	rows, err = queryer.QueryContext(ctx, `SELECT t.id,t.turn_id,t.wave,t.agent_id,t.behavior_revision_id,t.binding_revision_id,
t.participant_id,t.state,t.created_at
FROM stable_group_initial_target t JOIN stable_group_turn_recipient r
  ON r.account_id=t.account_id AND r.turn_id=t.turn_id AND r.agent_id=t.agent_id
WHERE t.account_id=? AND t.turn_id=? ORDER BY r.position,t.id`, accountID, turnID)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	for rows.Next() {
		var target ledger.InitialTargetRecord
		var targetCreated string
		if err := rows.Scan(&target.ID, &target.GroupTurnID, &target.Wave, &target.AgentID, &target.BehaviorRevisionID,
			&target.BindingRevisionID, &target.ParticipantID, &target.State, &targetCreated); err != nil {
			rows.Close()
			return ledger.GroupTurnRecord{}, err
		}
		target.CreatedAt = parseTime(targetCreated)
		record.InitialTargets = append(record.InitialTargets, target)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return ledger.GroupTurnRecord{}, err
	}
	if err := rows.Close(); err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	return record, nil
}

func validateHandoffEvidence(ctx context.Context, queryer collaborationQueryer, command ledger.AcceptHandoffCommand, sourceMessageID int64) error {
	handoff := command.Handoff
	if ok, err := conversationBelongsToAccount(ctx, queryer, handoff.AccountID, handoff.SourceConversationID); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("%w: source Conversation %q", ledger.ErrNotFound, handoff.SourceConversationID)
	}
	if ok, err := conversationBelongsToAccount(ctx, queryer, handoff.AccountID, handoff.OutputConversationID); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("%w: output Conversation %q", ledger.ErrNotFound, handoff.OutputConversationID)
	}
	var messageConversationID string
	if err := queryer.QueryRowContext(ctx, `SELECT conversation_id FROM conversation_message WHERE id=?`, sourceMessageID).Scan(&messageConversationID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: source message %q", ledger.ErrNotFound, handoff.SourceMessageID)
		}
		return err
	}
	if messageConversationID != handoff.SourceConversationID {
		return fmt.Errorf("Handoff source message does not belong to its persisted source Conversation")
	}
	var exists int
	err := queryer.QueryRowContext(ctx, `SELECT 1
FROM stable_agent a
JOIN agent_behavior_revision behavior ON behavior.id=? AND behavior.agent_id=a.id
JOIN agent_binding_revision binding ON binding.id=? AND binding.agent_id=a.id AND binding.behavior_revision_id=behavior.id
JOIN stable_agent_participant_evidence evidence
  ON evidence.agent_id=a.id AND evidence.binding_revision_id=binding.id
WHERE a.account_id=? AND a.id=? AND evidence.conversation_id=? AND evidence.participant_id=?`,
		handoff.RecipientBehaviorRevisionID, handoff.RecipientBindingRevisionID, handoff.AccountID,
		handoff.RecipientAgentID, handoff.OutputConversationID, command.ParticipantID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("Handoff recipient target does not match persisted Agent, revision, and participant evidence")
	}
	if err != nil {
		return err
	}
	if command.RequireCurrentRecipient {
		var state conversation.AgentState
		var currentBehaviorID, currentBindingID string
		err := queryer.QueryRowContext(ctx, `SELECT state,current_behavior_revision_id,current_binding_revision_id
FROM stable_agent WHERE account_id=? AND id=?`, handoff.AccountID, handoff.RecipientAgentID).Scan(
			&state, &currentBehaviorID, &currentBindingID)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: recipient Agent %q", ledger.ErrNotFound, handoff.RecipientAgentID)
		}
		if err != nil {
			return err
		}
		if state != conversation.AgentOpen || currentBehaviorID != handoff.RecipientBehaviorRevisionID ||
			currentBindingID != handoff.RecipientBindingRevisionID {
			return fmt.Errorf("%w: Handoff recipient revision evidence is no longer current", ledger.ErrRevisionConflict)
		}
	}
	if handoff.SourceAgentID != "" {
		if strings.TrimSpace(handoff.SourceBehaviorRevisionID) == "" || strings.TrimSpace(handoff.SourceBindingRevisionID) == "" {
			return fmt.Errorf("attributed Handoff source Agent requires Behavior and Binding Revision ids")
		}
		err = queryer.QueryRowContext(ctx, `SELECT 1
FROM stable_agent a
JOIN agent_behavior_revision behavior ON behavior.id=? AND behavior.agent_id=a.id
JOIN agent_binding_revision binding ON binding.id=? AND binding.agent_id=a.id AND binding.behavior_revision_id=behavior.id
WHERE a.account_id=? AND a.id=?`, handoff.SourceBehaviorRevisionID, handoff.SourceBindingRevisionID,
			handoff.AccountID, handoff.SourceAgentID).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("Handoff source Agent revision evidence was not found in the account")
		}
		if err != nil {
			return err
		}
	} else if handoff.SourceBehaviorRevisionID != "" || handoff.SourceBindingRevisionID != "" {
		return fmt.Errorf("Handoff source revisions require an attributed source Agent")
	}
	if handoff.GroupTurnID != "" {
		groupTurn, err := getGroupTurnRecord(ctx, queryer, handoff.AccountID, handoff.GroupTurnID)
		if err != nil {
			return err
		}
		if groupTurn.Envelope.ConversationID != handoff.SourceConversationID {
			return fmt.Errorf("Handoff Group Turn belongs to another Conversation")
		}
		if !reflect.DeepEqual(groupTurn.Envelope.RootDelegationGrant, handoff.RootDelegationGrant) {
			return fmt.Errorf("Handoff root delegation does not match its persisted Group Turn")
		}
		if groupTurn.Envelope.MaxAgentMessages != handoff.MaxAgentMessages ||
			groupTurn.Envelope.MaxHandoffDepth != handoff.MaxDepth ||
			!groupTurn.Envelope.Deadline.Equal(handoff.Deadline) {
			return fmt.Errorf("Handoff limits do not match its persisted Group Turn")
		}
	}
	if handoff.ParentHandoffID != "" {
		parent, err := getHandoffRecord(ctx, queryer, handoff.AccountID, handoff.ParentHandoffID)
		if err != nil {
			return err
		}
		if parent.Handoff.State != conversation.HandoffCompleted || parent.Result == nil {
			return fmt.Errorf("parent Handoff must have one completed authoritative result")
		}
		if handoff.Depth != parent.Handoff.Depth+1 || handoff.GroupTurnID != parent.Handoff.GroupTurnID ||
			handoff.SourceAgentID != parent.Handoff.RecipientAgentID ||
			handoff.SourceBehaviorRevisionID != parent.Handoff.RecipientBehaviorRevisionID ||
			handoff.SourceBindingRevisionID != parent.Handoff.RecipientBindingRevisionID ||
			handoff.SourceMessageID != parent.Result.MessageID ||
			!reflect.DeepEqual(handoff.RootDelegationGrant, parent.Handoff.RootDelegationGrant) ||
			handoff.MaxAgentMessages != parent.Handoff.MaxAgentMessages || handoff.MaxDepth != parent.Handoff.MaxDepth ||
			!handoff.Deadline.Equal(parent.Handoff.Deadline) {
			return fmt.Errorf("nested Handoff does not preserve its parent causal chain and limits")
		}
		if handoff.ParentStageAuthority == nil ||
			!reflect.DeepEqual(*handoff.ParentStageAuthority, parent.Handoff.EffectiveAuthority) {
			return fmt.Errorf("nested Handoff source-stage authority does not match its parent")
		}
	}
	for _, reference := range handoff.Context.References {
		if reference.Kind != conversation.ContextMessage {
			return fmt.Errorf("Handoff artifact %q has no finalized local ledger evidence", reference.Key())
		}
		messageID, err := canonicalMessageID(reference.ID)
		if err != nil {
			return err
		}
		var conversationID string
		if err := queryer.QueryRowContext(ctx, `SELECT conversation_id FROM conversation_message WHERE id=?`, messageID).Scan(&conversationID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: context message %q", ledger.ErrNotFound, reference.ID)
			}
			return err
		}
		if ok, err := conversationBelongsToAccount(ctx, queryer, handoff.AccountID, conversationID); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("Handoff context message %q belongs to another account", reference.ID)
		}
	}
	for _, conversationID := range command.ProjectionConversationIDs {
		if ok, err := conversationBelongsToAccount(ctx, queryer, handoff.AccountID, conversationID); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("%w: projection Conversation %q", ledger.ErrNotFound, conversationID)
		}
	}
	return nil
}

func getHandoffRecord(ctx context.Context, queryer collaborationQueryer, accountID, handoffID string) (ledger.HandoffRecord, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(handoffID) == "" {
		return ledger.HandoffRecord{}, fmt.Errorf("Handoff account id and id are required")
	}
	var record ledger.HandoffRecord
	var commandJSON, contextJSON, authorityJSON string
	var state conversation.HandoffState
	err := queryer.QueryRowContext(ctx, `SELECT command_json,context_json,effective_authority_json,state
FROM stable_handoff WHERE account_id=? AND id=?`, accountID, handoffID).Scan(&commandJSON, &contextJSON, &authorityJSON, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.HandoffRecord{}, fmt.Errorf("%w: Handoff %q", ledger.ErrNotFound, handoffID)
	}
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	if err := json.Unmarshal([]byte(commandJSON), &record.Handoff); err != nil {
		return ledger.HandoffRecord{}, err
	}
	if err := json.Unmarshal([]byte(contextJSON), &record.Handoff.Context); err != nil {
		return ledger.HandoffRecord{}, err
	}
	if err := json.Unmarshal([]byte(authorityJSON), &record.Handoff.EffectiveAuthority); err != nil {
		return ledger.HandoffRecord{}, err
	}
	record.Handoff.State = state
	var targetCreated string
	err = queryer.QueryRowContext(ctx, `SELECT id,handoff_id,conversation_id,agent_id,behavior_revision_id,binding_revision_id,
participant_id,state,created_at FROM stable_handoff_target WHERE account_id=? AND handoff_id=?`, accountID, handoffID).Scan(
		&record.Target.ID, &record.Target.HandoffID, &record.Target.ConversationID, &record.Target.AgentID,
		&record.Target.BehaviorRevisionID, &record.Target.BindingRevisionID, &record.Target.ParticipantID,
		&record.Target.State, &targetCreated)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	record.Target.CreatedAt = parseTime(targetCreated)
	var cancellation ledger.HandoffCancellationRecord
	var cancellationRequestedAt string
	err = queryer.QueryRowContext(ctx, `SELECT handoff_id,target_id,agent_id,behavior_revision_id,binding_revision_id,
participant_id,state,requested_by,requested_at FROM stable_handoff_cancellation
WHERE account_id=? AND handoff_id=?`, accountID, handoffID).Scan(
		&cancellation.HandoffID, &cancellation.TargetID, &cancellation.AgentID,
		&cancellation.BehaviorRevisionID, &cancellation.BindingRevisionID,
		&cancellation.ParticipantID, &cancellation.State, &cancellation.RequestedBy,
		&cancellationRequestedAt)
	if err == nil {
		cancellation.RequestedAt = parseTime(cancellationRequestedAt)
		if cancellation.TargetID != record.Target.ID || cancellation.AgentID != record.Target.AgentID ||
			cancellation.BehaviorRevisionID != record.Target.BehaviorRevisionID ||
			cancellation.BindingRevisionID != record.Target.BindingRevisionID ||
			cancellation.ParticipantID != record.Target.ParticipantID {
			return ledger.HandoffRecord{}, fmt.Errorf("persisted Handoff cancellation conflicts with target evidence")
		}
		record.Cancellation = &cancellation
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ledger.HandoffRecord{}, err
	}
	record.Projections = []conversation.HandoffProjection{}
	rows, err := queryer.QueryContext(ctx, `SELECT conversation_id,output_conversation_id,authoritative_message_id,state
FROM stable_handoff_projection WHERE account_id=? AND handoff_id=? ORDER BY conversation_id`, accountID, handoffID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	for rows.Next() {
		var projection conversation.HandoffProjection
		var messageID sql.NullInt64
		if err := rows.Scan(&projection.ConversationID, &projection.OutputConversationID, &messageID, &projection.State); err != nil {
			rows.Close()
			return ledger.HandoffRecord{}, err
		}
		projection.HandoffID = handoffID
		if messageID.Valid {
			projection.AuthoritativeMessageID = strconv.FormatInt(messageID.Int64, 10)
		}
		record.Projections = append(record.Projections, projection)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return ledger.HandoffRecord{}, err
	}
	if err := rows.Close(); err != nil {
		return ledger.HandoffRecord{}, err
	}
	var attempt ledger.HandoffAttemptRecord
	var attemptStarted, leaseExpires string
	var terminalReceipt, attemptCompleted sql.NullString
	err = queryer.QueryRowContext(ctx, `SELECT id,handoff_id,lease_id,machine_id,fence_token,state,
started_at,lease_expires_at,terminal_receipt_id,completed_at
FROM stable_handoff_attempt WHERE account_id=? AND handoff_id=? ORDER BY started_at DESC,id DESC LIMIT 1`,
		accountID, handoffID).Scan(&attempt.ID, &attempt.HandoffID, &attempt.LeaseID, &attempt.MachineID,
		&attempt.FenceToken, &attempt.State, &attemptStarted, &leaseExpires, &terminalReceipt, &attemptCompleted)
	if err == nil {
		attempt.StartedAt = parseTime(attemptStarted)
		attempt.LeaseExpiresAt = parseTime(leaseExpires)
		attempt.TerminalReceiptID = terminalReceipt.String
		if attemptCompleted.Valid {
			attempt.CompletedAt = parseTime(attemptCompleted.String)
		}
		record.Attempt = &attempt
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ledger.HandoffRecord{}, err
	}
	var resultMessageID int64
	var resultConversationID, resultBody string
	err = queryer.QueryRowContext(ctx, `SELECT completion.message_id,message.conversation_id,message.body
FROM stable_handoff_completion completion
JOIN conversation_message message ON message.id=completion.message_id
WHERE completion.account_id=? AND completion.handoff_id=?`, accountID, handoffID).Scan(
		&resultMessageID, &resultConversationID, &resultBody)
	if err == nil {
		record.Result = &conversation.HandoffResult{
			HandoffID: handoffID, OutputConversationID: resultConversationID,
			MessageID: strconv.FormatInt(resultMessageID, 10), Body: resultBody,
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ledger.HandoffRecord{}, err
	}
	return record, nil
}

func conversationBelongsToAccount(ctx context.Context, queryer collaborationQueryer, accountID, conversationID string) (bool, error) {
	var belongs int
	err := queryer.QueryRowContext(ctx, `SELECT CASE WHEN
EXISTS(SELECT 1 FROM stable_group WHERE account_id=? AND conversation_id=?) OR
EXISTS(SELECT 1 FROM agent_conversation link JOIN stable_agent agent ON agent.id=link.agent_id
       WHERE agent.account_id=? AND link.conversation_id=?)
THEN 1 ELSE 0 END`, accountID, conversationID, accountID, conversationID).Scan(&belongs)
	return belongs == 1, err
}

func groupAgentMessageCount(ctx context.Context, queryer collaborationQueryer, accountID, groupTurnID string) (int, error) {
	if groupTurnID == "" {
		return 0, nil
	}
	var count int
	err := queryer.QueryRowContext(ctx, `SELECT count(*) FROM conversation_message
WHERE author_kind=? AND (
  turn_id=? OR turn_id IN (
    SELECT id FROM stable_handoff WHERE account_id=? AND group_turn_id=?
  )
)`, conversation.AuthorAssistant, groupTurnID, accountID, groupTurnID).Scan(&count)
	return count, err
}

func canonicalMessageID(value string) (int64, error) {
	messageID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || messageID <= 0 || strconv.FormatInt(messageID, 10) != value {
		return 0, fmt.Errorf("Handoff message id %q is not a canonical Fort message id", value)
	}
	return messageID, nil
}
