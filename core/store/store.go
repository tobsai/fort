// Package store is Fort's SQLite state store (backlog AO-016, spec §6.6):
// run, node_run, route_decision, and an append-only event log. The event log
// is the source the fort-ui live feed replays from.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver, registered as "sqlite"
)

// Store wraps the SQLite database.
type Store struct{ db *sql.DB }

// RouteDecision is a persisted routing outcome.
type RouteDecision struct {
	ID          string
	TaskID      string
	Route       string
	MatchedRule string
	IsDefault   bool
	Reason      string
	CreatedAt   time.Time
}

// Run is a persisted execution (a routed task or a flow run).
type Run struct {
	ID          string
	Title       string
	Body        string // markdown body from a multiline compose (spec 031); "" if title-only
	Agent       string
	Profile     string // exact Fort-owned profile requested for a direct run
	Model       string // provider model derived from Profile; empty means configured default
	Status      string
	MatchedRule string
	Machine     string // resolved target host (spec 022); "" = local/single-machine
	FlowID      string
	ExitCode    int
	Error       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NodeRun is a persisted DAG node execution (Phase 2).
type NodeRun struct {
	ID        string // runID:nodeID
	RunID     string
	NodeID    string
	Type      string
	Status    string
	Input     string
	Output    string
	Attempts  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Event is one append-only event row.
type Event struct {
	ID        int64
	RunID     string
	NodeID    string // DAG step this event came from (spec 027); "" for run-level/single-run events
	Type      string
	Data      string
	Code      int
	CreatedAt time.Time
}

// Open opens (creating if needed) the database at path and applies migrations.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("store: mkdir %s: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db.SetMaxOpenConns(1) // serialize writes; SQLite single-writer
	if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: configure SQLite busy timeout: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	const schema = `
PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS route_decision (
  id TEXT PRIMARY KEY, task_id TEXT, route TEXT, matched_rule TEXT,
  is_default INTEGER, reason TEXT, created_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_route_decision_task ON route_decision(task_id);
CREATE TABLE IF NOT EXISTS run (
	  id TEXT PRIMARY KEY, title TEXT, body TEXT, agent TEXT, profile TEXT, model TEXT, status TEXT, matched_rule TEXT,
  machine TEXT, flow_id TEXT, exit_code INTEGER, error TEXT,
  created_at TEXT, updated_at TEXT
);
CREATE TABLE IF NOT EXISTS node_run (
  id TEXT PRIMARY KEY, run_id TEXT, node_id TEXT, type TEXT, status TEXT,
  input TEXT, output TEXT, attempts INTEGER, created_at TEXT, updated_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_node_run_run ON node_run(run_id);
CREATE TABLE IF NOT EXISTS event (
  id INTEGER PRIMARY KEY AUTOINCREMENT, run_id TEXT, node_id TEXT, type TEXT, data TEXT,
  code INTEGER, created_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_event_run ON event(run_id);
CREATE TABLE IF NOT EXISTS invite (
  code_hash TEXT PRIMARY KEY, created_at TEXT, expires_at TEXT, used_at TEXT
);
CREATE TABLE IF NOT EXISTS backlog_item (
  id TEXT PRIMARY KEY, title TEXT, body TEXT, agent TEXT, machine TEXT,
  labels TEXT, source TEXT, created_at TEXT
);
CREATE TABLE IF NOT EXISTS playbook_revision (
  id TEXT NOT NULL, revision INTEGER NOT NULL, data TEXT NOT NULL, created_at TEXT,
  PRIMARY KEY(id, revision)
);
CREATE INDEX IF NOT EXISTS idx_playbook_revision_latest ON playbook_revision(id, revision DESC);
CREATE TABLE IF NOT EXISTS project (
  id TEXT PRIMARY KEY, name TEXT NOT NULL COLLATE NOCASE, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_project_name_unique ON project(name COLLATE NOCASE);
CREATE TABLE IF NOT EXISTS primary_agent_setting (
  singleton INTEGER PRIMARY KEY CHECK(singleton=1),
  option_id TEXT NOT NULL,
  seat_id TEXT NOT NULL,
  profile TEXT NOT NULL CHECK(profile='codex-subscription:'||model),
  agent TEXT NOT NULL CHECK(agent='codex-subscription'),
  model TEXT NOT NULL CHECK(length(trim(model))>0),
  machine TEXT NOT NULL,
  display_name TEXT NOT NULL,
  authority TEXT NOT NULL CHECK(authority='chat_subscription_isolated_v1'),
  policy_id TEXT NOT NULL CHECK(policy_id='codex-subscription-chat-v1'),
  policy_revision TEXT NOT NULL CHECK(length(trim(policy_revision))>0),
  adapter_id TEXT NOT NULL CHECK(adapter_id='model.chat.text-only.codex-subscription'),
  adapter_revision TEXT NOT NULL CHECK(length(trim(adapter_revision))>0),
  codex_version TEXT NOT NULL,
  codex_executable_revision TEXT NOT NULL CHECK(length(codex_executable_revision)=64 AND codex_executable_revision NOT GLOB '*[^0-9a-f]*'),
  codex_schema_revision TEXT NOT NULL CHECK(length(codex_schema_revision)=64 AND codex_schema_revision NOT GLOB '*[^0-9a-f]*'),
  runtime_contract TEXT NOT NULL CHECK(runtime_contract='codex_subscription_exec_v1'),
  reasoning_effort TEXT NOT NULL CHECK(reasoning_effort='medium'),
  reasoning_context TEXT NOT NULL CHECK(reasoning_context='current_turn'),
  request_timeout_millis INTEGER NOT NULL CHECK(request_timeout_millis=120000),
  developer_instruction_revision TEXT NOT NULL,
  account_type TEXT NOT NULL CHECK(account_type='chatgpt'),
  account_plan TEXT NOT NULL CHECK(length(trim(account_plan))>0 AND account_plan=trim(account_plan)),
  thread_mode TEXT NOT NULL CHECK(thread_mode='ephemeral'),
  sandbox_mode TEXT NOT NULL CHECK(sandbox_mode='readOnly'),
  approval_policy TEXT NOT NULL CHECK(approval_policy='never'),
  workdir_mode TEXT NOT NULL CHECK(workdir_mode='empty_per_target'),
  dynamic_tools_mode TEXT NOT NULL CHECK(dynamic_tools_mode='none'),
  mcp_mode TEXT NOT NULL CHECK(mcp_mode='none'),
  command_policy TEXT NOT NULL CHECK(command_policy='deny_and_fail'),
  file_read_policy TEXT NOT NULL CHECK(file_read_policy='deny_and_fail'),
  isolation_revision TEXT NOT NULL CHECK(length(trim(isolation_revision))>0),
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS conversation (
  id TEXT PRIMARY KEY, project_id TEXT, title TEXT NOT NULL, state TEXT NOT NULL DEFAULT 'open',
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
  FOREIGN KEY(project_id) REFERENCES project(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_conversation_project ON conversation(project_id, updated_at DESC);
CREATE TABLE IF NOT EXISTS conversation_participant (
  id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL, seat_id TEXT NOT NULL,
  profile TEXT NOT NULL, agent TEXT NOT NULL, model TEXT, machine TEXT NOT NULL,
  display_name TEXT NOT NULL, position INTEGER NOT NULL, state TEXT NOT NULL DEFAULT 'active',
  created_at TEXT NOT NULL, removed_at TEXT,
  FOREIGN KEY(conversation_id) REFERENCES conversation(id) ON DELETE CASCADE,
  UNIQUE(conversation_id, seat_id)
);
CREATE INDEX IF NOT EXISTS idx_conversation_participant_conversation
  ON conversation_participant(conversation_id, position);
CREATE TABLE IF NOT EXISTS primary_channel (
  conversation_id TEXT PRIMARY KEY,
  participant_id TEXT NOT NULL UNIQUE,
  authority TEXT NOT NULL CHECK(authority='chat_subscription_isolated_v1'),
  policy_id TEXT NOT NULL CHECK(policy_id='codex-subscription-chat-v1'),
  policy_revision TEXT NOT NULL CHECK(length(trim(policy_revision))>0),
  adapter_id TEXT NOT NULL CHECK(adapter_id='model.chat.text-only.codex-subscription'),
  adapter_revision TEXT NOT NULL CHECK(length(trim(adapter_revision))>0),
  codex_version TEXT NOT NULL,
  codex_executable_revision TEXT NOT NULL CHECK(length(codex_executable_revision)=64 AND codex_executable_revision NOT GLOB '*[^0-9a-f]*'),
  codex_schema_revision TEXT NOT NULL CHECK(length(codex_schema_revision)=64 AND codex_schema_revision NOT GLOB '*[^0-9a-f]*'),
  runtime_contract TEXT NOT NULL CHECK(runtime_contract='codex_subscription_exec_v1'),
  reasoning_effort TEXT NOT NULL CHECK(reasoning_effort='medium'),
  reasoning_context TEXT NOT NULL CHECK(reasoning_context='current_turn'),
  request_timeout_millis INTEGER NOT NULL CHECK(request_timeout_millis=120000),
  developer_instruction_revision TEXT NOT NULL,
  account_type TEXT NOT NULL CHECK(account_type='chatgpt'),
  account_plan TEXT NOT NULL CHECK(length(trim(account_plan))>0 AND account_plan=trim(account_plan)),
  thread_mode TEXT NOT NULL CHECK(thread_mode='ephemeral'),
  sandbox_mode TEXT NOT NULL CHECK(sandbox_mode='readOnly'),
  approval_policy TEXT NOT NULL CHECK(approval_policy='never'),
  workdir_mode TEXT NOT NULL CHECK(workdir_mode='empty_per_target'),
  dynamic_tools_mode TEXT NOT NULL CHECK(dynamic_tools_mode='none'),
  mcp_mode TEXT NOT NULL CHECK(mcp_mode='none'),
  command_policy TEXT NOT NULL CHECK(command_policy='deny_and_fail'),
  file_read_policy TEXT NOT NULL CHECK(file_read_policy='deny_and_fail'),
  isolation_revision TEXT NOT NULL CHECK(length(trim(isolation_revision))>0),
  created_at TEXT NOT NULL,
  FOREIGN KEY(conversation_id) REFERENCES conversation(id) ON DELETE CASCADE,
  FOREIGN KEY(participant_id) REFERENCES conversation_participant(id)
);
CREATE TABLE IF NOT EXISTS primary_channel_pin (
  conversation_id TEXT PRIMARY KEY,
  pinned_at TEXT NOT NULL,
  FOREIGN KEY(conversation_id) REFERENCES primary_channel(conversation_id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS conversation_message (
  id INTEGER PRIMARY KEY AUTOINCREMENT, conversation_id TEXT NOT NULL, turn_id TEXT,
  target_id TEXT, author_kind TEXT NOT NULL, author_id TEXT NOT NULL,
  body TEXT NOT NULL, created_at TEXT NOT NULL,
  FOREIGN KEY(conversation_id) REFERENCES conversation(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_conversation_message_conversation
  ON conversation_message(conversation_id, id);
CREATE TABLE IF NOT EXISTS conversation_turn (
  id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL, client_turn_id TEXT NOT NULL,
  prompt_message_id INTEGER NOT NULL,
  through_message_id INTEGER NOT NULL, context_json TEXT, created_at TEXT NOT NULL,
  FOREIGN KEY(conversation_id) REFERENCES conversation(id) ON DELETE CASCADE,
  FOREIGN KEY(prompt_message_id) REFERENCES conversation_message(id)
);
CREATE INDEX IF NOT EXISTS idx_conversation_turn_conversation
  ON conversation_turn(conversation_id, created_at);
CREATE TABLE IF NOT EXISTS conversation_target (
  id TEXT PRIMARY KEY, turn_id TEXT NOT NULL, participant_id TEXT NOT NULL,
  run_id TEXT NOT NULL UNIQUE, attempt INTEGER NOT NULL DEFAULT 1, state TEXT NOT NULL,
  error_code TEXT, error TEXT,
  authority TEXT, policy_id TEXT, policy_revision TEXT,
  selected_adapter_id TEXT, selected_adapter_revision TEXT,
  selected_codex_version TEXT, selected_codex_executable_revision TEXT, selected_codex_schema_revision TEXT,
  runtime_contract TEXT, requested_model TEXT, reasoning_effort TEXT, reasoning_context TEXT,
  request_timeout_millis INTEGER, developer_instruction_revision TEXT,
  account_type TEXT, account_plan TEXT, thread_mode TEXT, sandbox_mode TEXT, approval_policy TEXT,
  workdir_mode TEXT, dynamic_tools_mode TEXT, mcp_mode TEXT, command_policy TEXT, file_read_policy TEXT,
  isolation_revision TEXT,
  observed_adapter_id TEXT, observed_adapter_revision TEXT,
  observed_codex_version TEXT, observed_codex_executable_revision TEXT, observed_codex_schema_revision TEXT,
  resolved_model TEXT, provider_thread_id TEXT, provider_terminal_status TEXT,
  usage_source TEXT, input_tokens INTEGER, cached_input_tokens INTEGER, output_tokens INTEGER, reasoning_tokens INTEGER,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
  FOREIGN KEY(turn_id) REFERENCES conversation_turn(id) ON DELETE CASCADE,
  FOREIGN KEY(participant_id) REFERENCES conversation_participant(id)
);
CREATE INDEX IF NOT EXISTS idx_conversation_target_turn ON conversation_target(turn_id);
CREATE INDEX IF NOT EXISTS idx_conversation_target_state ON conversation_target(state, updated_at DESC);
CREATE TABLE IF NOT EXISTS schedule (
  id TEXT PRIMARY KEY, title TEXT NOT NULL, kind TEXT NOT NULL, expression TEXT NOT NULL, flow_id TEXT NOT NULL,
  timezone TEXT NOT NULL, enabled INTEGER NOT NULL, next_fire_at TEXT, last_fire_at TEXT,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS schedule_channel_link (
  schedule_id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(schedule_id) REFERENCES schedule(id) ON DELETE CASCADE,
  FOREIGN KEY(conversation_id) REFERENCES primary_channel(conversation_id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS schedule_occurrence (
  id TEXT PRIMARY KEY, schedule_id TEXT NOT NULL, run_id TEXT,
  scheduled_for TEXT NOT NULL, state TEXT NOT NULL, error TEXT,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
  FOREIGN KEY(schedule_id) REFERENCES schedule(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_schedule_occurrence_day
  ON schedule_occurrence(scheduled_for, state);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	// Additive migrations for databases created before a column existed. Each is
	// idempotent (skipped when the column is already present).
	if err := s.addColumn("run", "machine", "TEXT"); err != nil {
		return fmt.Errorf("store: migrate run.machine: %w", err)
	}
	if err := s.addColumn("event", "node_id", "TEXT"); err != nil {
		return fmt.Errorf("store: migrate event.node_id: %w", err)
	}
	if err := s.addColumn("run", "body", "TEXT"); err != nil {
		return fmt.Errorf("store: migrate run.body: %w", err)
	}
	if err := s.addColumn("run", "profile", "TEXT"); err != nil {
		return fmt.Errorf("store: migrate run.profile: %w", err)
	}
	if err := s.addColumn("run", "model", "TEXT"); err != nil {
		return fmt.Errorf("store: migrate run.model: %w", err)
	}
	if err := s.addColumn("conversation", "state", "TEXT NOT NULL DEFAULT 'open'"); err != nil {
		return fmt.Errorf("store: migrate conversation.state: %w", err)
	}
	if err := s.addColumn("conversation_participant", "state", "TEXT NOT NULL DEFAULT 'active'"); err != nil {
		return fmt.Errorf("store: migrate conversation_participant.state: %w", err)
	}
	if err := s.addColumn("conversation_participant", "removed_at", "TEXT"); err != nil {
		return fmt.Errorf("store: migrate conversation_participant.removed_at: %w", err)
	}
	if err := s.addColumn("conversation_turn", "client_turn_id", "TEXT"); err != nil {
		return fmt.Errorf("store: migrate conversation_turn.client_turn_id: %w", err)
	}
	if err := s.addColumn("conversation_turn", "context_json", "TEXT"); err != nil {
		return fmt.Errorf("store: migrate conversation_turn.context_json: %w", err)
	}
	if err := s.addColumn("conversation_target", "attempt", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return fmt.Errorf("store: migrate conversation_target.attempt: %w", err)
	}
	if err := s.addColumn("conversation_target", "error_code", "TEXT"); err != nil {
		return fmt.Errorf("store: migrate conversation_target.error_code: %w", err)
	}
	primaryTargetColumns := []struct {
		name string
		typ  string
	}{
		{"authority", "TEXT"},
		{"policy_id", "TEXT"},
		{"policy_revision", "TEXT"},
		{"selected_adapter_id", "TEXT"},
		{"selected_adapter_revision", "TEXT"},
		{"selected_codex_version", "TEXT"},
		{"selected_codex_executable_revision", "TEXT"},
		{"selected_codex_schema_revision", "TEXT"},
		{"runtime_contract", "TEXT"},
		{"requested_model", "TEXT"},
		{"reasoning_effort", "TEXT"},
		{"reasoning_context", "TEXT"},
		{"request_timeout_millis", "INTEGER"},
		{"developer_instruction_revision", "TEXT"},
		{"account_type", "TEXT"},
		{"account_plan", "TEXT"},
		{"thread_mode", "TEXT"},
		{"sandbox_mode", "TEXT"},
		{"approval_policy", "TEXT"},
		{"workdir_mode", "TEXT"},
		{"dynamic_tools_mode", "TEXT"},
		{"mcp_mode", "TEXT"},
		{"command_policy", "TEXT"},
		{"file_read_policy", "TEXT"},
		{"isolation_revision", "TEXT"},
		{"observed_adapter_id", "TEXT"},
		{"observed_adapter_revision", "TEXT"},
		{"observed_codex_version", "TEXT"},
		{"observed_codex_executable_revision", "TEXT"},
		{"observed_codex_schema_revision", "TEXT"},
		{"resolved_model", "TEXT"},
		{"provider_thread_id", "TEXT"},
		{"provider_terminal_status", "TEXT"},
		{"usage_source", "TEXT"},
		{"input_tokens", "INTEGER"},
		{"cached_input_tokens", "INTEGER"},
		{"output_tokens", "INTEGER"},
		{"reasoning_tokens", "INTEGER"},
	}
	for _, column := range primaryTargetColumns {
		if err := s.addColumn("conversation_target", column.name, column.typ); err != nil {
			return fmt.Errorf("store: migrate conversation_target.%s: %w", column.name, err)
		}
	}
	if err := s.addColumn("schedule", "title", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("store: migrate schedule.title: %w", err)
	}
	if err := s.addColumn("schedule", "next_fire_at", "TEXT"); err != nil {
		return fmt.Errorf("store: migrate schedule.next_fire_at: %w", err)
	}
	if err := s.addColumn("schedule", "last_fire_at", "TEXT"); err != nil {
		return fmt.Errorf("store: migrate schedule.last_fire_at: %w", err)
	}
	if _, err := s.db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS idx_conversation_turn_client
  ON conversation_turn(conversation_id, client_turn_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_conversation_target_attempt
  ON conversation_target(turn_id, participant_id, attempt);
CREATE UNIQUE INDEX IF NOT EXISTS idx_primary_channel_active_target
  ON conversation_target(participant_id)
  WHERE authority='chat_subscription_isolated_v1' AND state IN ('queued','working');
CREATE UNIQUE INDEX IF NOT EXISTS idx_schedule_occurrence_unique
  ON schedule_occurrence(schedule_id, scheduled_for);
`); err != nil {
		return fmt.Errorf("store: migrate conversation indexes: %w", err)
	}
	if _, err := s.db.Exec(primaryChannelTriggers); err != nil {
		return fmt.Errorf("store: migrate primary Channel invariants: %w", err)
	}
	if err := s.migrateAgentChannels(); err != nil {
		return fmt.Errorf("store: migrate Agent Channels: %w", err)
	}
	return nil
}

const primaryChannelTriggers = `
CREATE TRIGGER IF NOT EXISTS primary_channel_insert_invariant
BEFORE INSERT ON primary_channel
WHEN NOT EXISTS (
  SELECT 1
  FROM conversation_participant selected
  WHERE selected.id=NEW.participant_id
    AND selected.conversation_id=NEW.conversation_id
    AND selected.state='active'
    AND selected.agent='codex-subscription'
    AND selected.model IS NOT NULL AND length(trim(selected.model))>0
    AND selected.profile='codex-subscription:'||selected.model
    AND (SELECT COUNT(*) FROM conversation_participant active
         WHERE active.conversation_id=NEW.conversation_id AND active.state='active')=1
)
BEGIN
  SELECT RAISE(ABORT, 'primary_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS primary_channel_update_immutable
BEFORE UPDATE ON primary_channel
BEGIN
  SELECT RAISE(ABORT, 'primary_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS primary_channel_delete_immutable
BEFORE DELETE ON primary_channel
BEGIN
  SELECT RAISE(ABORT, 'primary_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS primary_channel_participant_insert_immutable
BEFORE INSERT ON conversation_participant
WHEN EXISTS (SELECT 1 FROM primary_channel WHERE conversation_id=NEW.conversation_id)
BEGIN
  SELECT RAISE(ABORT, 'primary_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS primary_channel_participant_update_immutable
BEFORE UPDATE ON conversation_participant
WHEN EXISTS (SELECT 1 FROM primary_channel WHERE conversation_id=OLD.conversation_id OR conversation_id=NEW.conversation_id)
BEGIN
  SELECT RAISE(ABORT, 'primary_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS primary_channel_participant_delete_immutable
BEFORE DELETE ON conversation_participant
WHEN EXISTS (SELECT 1 FROM primary_channel WHERE conversation_id=OLD.conversation_id)
BEGIN
  SELECT RAISE(ABORT, 'primary_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS primary_channel_conversation_delete_immutable
BEFORE DELETE ON conversation
WHEN EXISTS (SELECT 1 FROM primary_channel WHERE conversation_id=OLD.id)
BEGIN
  SELECT RAISE(ABORT, 'primary_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS primary_channel_target_insert_authority
BEFORE INSERT ON conversation_target
WHEN EXISTS (
  SELECT 1 FROM conversation_turn turn
  JOIN primary_channel channel ON channel.conversation_id=turn.conversation_id
  WHERE turn.id=NEW.turn_id
)
AND NOT EXISTS (
  SELECT 1 FROM conversation_turn turn
  JOIN primary_channel channel ON channel.conversation_id=turn.conversation_id
  JOIN conversation_participant participant ON participant.id=channel.participant_id
  WHERE turn.id=NEW.turn_id
    AND NEW.participant_id=channel.participant_id
    AND NEW.authority=channel.authority
    AND NEW.policy_id=channel.policy_id
    AND NEW.policy_revision=channel.policy_revision
    AND NEW.selected_adapter_id=channel.adapter_id
    AND length(trim(NEW.selected_adapter_revision))>0
    AND NEW.selected_codex_version=channel.codex_version
    AND NEW.selected_codex_executable_revision=channel.codex_executable_revision
    AND NEW.selected_codex_schema_revision=channel.codex_schema_revision
    AND NEW.runtime_contract=channel.runtime_contract
    AND NEW.requested_model=participant.model
    AND NEW.reasoning_effort=channel.reasoning_effort
    AND NEW.reasoning_context=channel.reasoning_context
    AND NEW.request_timeout_millis=channel.request_timeout_millis
    AND NEW.developer_instruction_revision=channel.developer_instruction_revision
    AND NEW.account_type=channel.account_type
    AND NEW.account_plan=channel.account_plan
    AND NEW.thread_mode=channel.thread_mode
    AND NEW.sandbox_mode=channel.sandbox_mode
    AND NEW.approval_policy=channel.approval_policy
    AND NEW.workdir_mode=channel.workdir_mode
    AND NEW.dynamic_tools_mode=channel.dynamic_tools_mode
    AND NEW.mcp_mode=channel.mcp_mode
    AND NEW.command_policy=channel.command_policy
    AND NEW.file_read_policy=channel.file_read_policy
    AND NEW.isolation_revision=channel.isolation_revision
)
BEGIN
  SELECT RAISE(ABORT, 'primary_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS primary_channel_target_single_flight
BEFORE INSERT ON conversation_target
WHEN EXISTS (
  SELECT 1
  FROM conversation_turn incoming_turn
  JOIN primary_channel channel ON channel.conversation_id=incoming_turn.conversation_id
  WHERE incoming_turn.id=NEW.turn_id
)
AND EXISTS (
  SELECT 1
  FROM conversation_target active_target
  JOIN conversation_turn active_turn ON active_turn.id=active_target.turn_id
  JOIN conversation_turn incoming_turn ON incoming_turn.id=NEW.turn_id
  WHERE active_turn.conversation_id=incoming_turn.conversation_id
    AND active_target.state IN ('queued','working')
)
BEGIN
  SELECT RAISE(ABORT, 'primary_channel_active_target');
END;

CREATE TRIGGER IF NOT EXISTS legacy_conversation_target_authority
BEFORE INSERT ON conversation_target
WHEN COALESCE(NEW.authority,'')<>''
AND NOT EXISTS (
  SELECT 1 FROM conversation_turn turn
  JOIN primary_channel channel ON channel.conversation_id=turn.conversation_id
  WHERE turn.id=NEW.turn_id
)
BEGIN
  SELECT RAISE(ABORT, 'primary_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS conversation_target_authority_immutable
BEFORE UPDATE OF
  turn_id,participant_id,authority,policy_id,policy_revision,selected_adapter_id,selected_adapter_revision,
  selected_codex_version,selected_codex_executable_revision,selected_codex_schema_revision,
  runtime_contract,requested_model,reasoning_effort,reasoning_context,request_timeout_millis,
  developer_instruction_revision,account_type,account_plan,thread_mode,sandbox_mode,
  approval_policy,workdir_mode,dynamic_tools_mode,mcp_mode,command_policy,file_read_policy,
  isolation_revision
ON conversation_target
WHEN OLD.turn_id IS NOT NEW.turn_id
  OR OLD.participant_id IS NOT NEW.participant_id
  OR OLD.authority IS NOT NEW.authority
  OR OLD.policy_id IS NOT NEW.policy_id
  OR OLD.policy_revision IS NOT NEW.policy_revision
  OR OLD.selected_adapter_id IS NOT NEW.selected_adapter_id
  OR OLD.selected_adapter_revision IS NOT NEW.selected_adapter_revision
  OR OLD.selected_codex_version IS NOT NEW.selected_codex_version
  OR OLD.selected_codex_executable_revision IS NOT NEW.selected_codex_executable_revision
  OR OLD.selected_codex_schema_revision IS NOT NEW.selected_codex_schema_revision
  OR OLD.runtime_contract IS NOT NEW.runtime_contract
  OR OLD.requested_model IS NOT NEW.requested_model
  OR OLD.reasoning_effort IS NOT NEW.reasoning_effort
  OR OLD.reasoning_context IS NOT NEW.reasoning_context
  OR OLD.request_timeout_millis IS NOT NEW.request_timeout_millis
  OR OLD.developer_instruction_revision IS NOT NEW.developer_instruction_revision
  OR OLD.account_type IS NOT NEW.account_type
  OR OLD.account_plan IS NOT NEW.account_plan
  OR OLD.thread_mode IS NOT NEW.thread_mode
  OR OLD.sandbox_mode IS NOT NEW.sandbox_mode
  OR OLD.approval_policy IS NOT NEW.approval_policy
  OR OLD.workdir_mode IS NOT NEW.workdir_mode
  OR OLD.dynamic_tools_mode IS NOT NEW.dynamic_tools_mode
  OR OLD.mcp_mode IS NOT NEW.mcp_mode
  OR OLD.command_policy IS NOT NEW.command_policy
  OR OLD.file_read_policy IS NOT NEW.file_read_policy
  OR OLD.isolation_revision IS NOT NEW.isolation_revision
BEGIN
  SELECT RAISE(ABORT, 'primary_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS conversation_target_receipt_insert_empty
BEFORE INSERT ON conversation_target
WHEN NEW.observed_adapter_id IS NOT NULL
  OR NEW.observed_adapter_revision IS NOT NULL
  OR NEW.observed_codex_version IS NOT NULL
  OR NEW.observed_codex_executable_revision IS NOT NULL
  OR NEW.observed_codex_schema_revision IS NOT NULL
  OR NEW.resolved_model IS NOT NULL
  OR NEW.provider_thread_id IS NOT NULL
  OR NEW.provider_terminal_status IS NOT NULL
  OR NEW.usage_source IS NOT NULL
  OR NEW.input_tokens IS NOT NULL
  OR NEW.cached_input_tokens IS NOT NULL
  OR NEW.output_tokens IS NOT NULL
  OR NEW.reasoning_tokens IS NOT NULL
BEGIN
  SELECT RAISE(ABORT, 'primary_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS conversation_target_receipt_terminal_only
BEFORE UPDATE OF
  observed_adapter_id,observed_adapter_revision,observed_codex_version,
  observed_codex_executable_revision,observed_codex_schema_revision,resolved_model,
  provider_thread_id,provider_terminal_status,usage_source,input_tokens,cached_input_tokens,
  output_tokens,reasoning_tokens
ON conversation_target
WHEN NEW.state NOT IN ('answered','failed','canceled')
  AND (OLD.observed_adapter_id IS NOT NEW.observed_adapter_id
    OR OLD.observed_adapter_revision IS NOT NEW.observed_adapter_revision
    OR OLD.observed_codex_version IS NOT NEW.observed_codex_version
    OR OLD.observed_codex_executable_revision IS NOT NEW.observed_codex_executable_revision
    OR OLD.observed_codex_schema_revision IS NOT NEW.observed_codex_schema_revision
    OR OLD.resolved_model IS NOT NEW.resolved_model
    OR OLD.provider_thread_id IS NOT NEW.provider_thread_id
    OR OLD.provider_terminal_status IS NOT NEW.provider_terminal_status
    OR OLD.usage_source IS NOT NEW.usage_source
    OR OLD.input_tokens IS NOT NEW.input_tokens
    OR OLD.cached_input_tokens IS NOT NEW.cached_input_tokens
    OR OLD.output_tokens IS NOT NEW.output_tokens
    OR OLD.reasoning_tokens IS NOT NEW.reasoning_tokens)
BEGIN
  SELECT RAISE(ABORT, 'primary_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS conversation_target_receipt_immutable
BEFORE UPDATE OF
  observed_adapter_id,observed_adapter_revision,observed_codex_version,
  observed_codex_executable_revision,observed_codex_schema_revision,resolved_model,
  provider_thread_id,provider_terminal_status,usage_source,input_tokens,cached_input_tokens,
  output_tokens,reasoning_tokens
ON conversation_target
WHEN OLD.provider_terminal_status IS NOT NULL
  AND (OLD.observed_adapter_id IS NOT NEW.observed_adapter_id
    OR OLD.observed_adapter_revision IS NOT NEW.observed_adapter_revision
    OR OLD.observed_codex_version IS NOT NEW.observed_codex_version
    OR OLD.observed_codex_executable_revision IS NOT NEW.observed_codex_executable_revision
    OR OLD.observed_codex_schema_revision IS NOT NEW.observed_codex_schema_revision
    OR OLD.resolved_model IS NOT NEW.resolved_model
    OR OLD.provider_thread_id IS NOT NEW.provider_thread_id
    OR OLD.provider_terminal_status IS NOT NEW.provider_terminal_status
    OR OLD.usage_source IS NOT NEW.usage_source
    OR OLD.input_tokens IS NOT NEW.input_tokens
    OR OLD.cached_input_tokens IS NOT NEW.cached_input_tokens
    OR OLD.output_tokens IS NOT NEW.output_tokens
    OR OLD.reasoning_tokens IS NOT NEW.reasoning_tokens)
BEGIN
  SELECT RAISE(ABORT, 'primary_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS primary_channel_target_terminal_receipt
BEFORE UPDATE OF state ON conversation_target
WHEN NEW.authority='chat_subscription_isolated_v1'
  AND NEW.state IN ('answered','failed','canceled')
  AND (COALESCE(NEW.observed_adapter_id,'')=''
    OR COALESCE(NEW.observed_adapter_revision,'')=''
    OR COALESCE(NEW.observed_codex_version,'')=''
    OR COALESCE(NEW.observed_codex_executable_revision,'')=''
    OR COALESCE(NEW.observed_codex_schema_revision,'')=''
    OR COALESCE(NEW.provider_terminal_status,'')=''
    OR COALESCE(NEW.usage_source,'')=''
    OR NEW.input_tokens IS NULL OR NEW.input_tokens<0
    OR NEW.cached_input_tokens IS NULL OR NEW.cached_input_tokens<0
    OR NEW.output_tokens IS NULL OR NEW.output_tokens<0
    OR NEW.reasoning_tokens IS NULL OR NEW.reasoning_tokens<0
    OR NEW.observed_adapter_id NOT IN (NEW.selected_adapter_id,'unknown')
    OR NEW.observed_adapter_revision NOT IN (NEW.selected_adapter_revision,'unknown')
    OR NEW.observed_codex_version NOT IN (NEW.selected_codex_version,'unknown')
    OR NEW.observed_codex_executable_revision NOT IN (NEW.selected_codex_executable_revision,'unknown')
    OR NEW.observed_codex_schema_revision NOT IN (NEW.selected_codex_schema_revision,'unknown')
    OR COALESCE(NEW.resolved_model,'') NOT IN ('','unknown')
    OR (NEW.provider_terminal_status='completed' AND NEW.observed_adapter_id<>NEW.selected_adapter_id)
    OR (NEW.provider_terminal_status='completed' AND NEW.observed_adapter_revision<>NEW.selected_adapter_revision)
    OR (NEW.provider_terminal_status='completed' AND NEW.observed_codex_version<>NEW.selected_codex_version)
    OR (NEW.provider_terminal_status='completed' AND NEW.observed_codex_executable_revision<>NEW.selected_codex_executable_revision)
    OR (NEW.provider_terminal_status='completed' AND NEW.observed_codex_schema_revision<>NEW.selected_codex_schema_revision)
    OR (NEW.provider_terminal_status='completed' AND NEW.usage_source<>'codex_exec_jsonl')
    OR (NEW.provider_terminal_status='completed' AND COALESCE(NEW.provider_thread_id,'')=''))
BEGIN
  SELECT RAISE(ABORT, 'primary_channel_invariant');
END;
`

// addColumn adds col to table if it is not already present (idempotent).
func (s *Store) addColumn(table, col, typ string) error {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid, notnull, pk int
			name, ctype      string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, col) {
			return nil // already present
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, col, typ))
	return err
}

func nowOr(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

// SaveRouteDecision persists a routing decision.
func (s *Store) SaveRouteDecision(d RouteDecision) error {
	_, err := s.db.Exec(
		`INSERT INTO route_decision(id,task_id,route,matched_rule,is_default,reason,created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		d.ID, d.TaskID, d.Route, d.MatchedRule, boolToInt(d.IsDefault), d.Reason, nowOr(d.CreatedAt))
	return err
}

// RouteDecisions returns the decisions recorded for a task, oldest first.
func (s *Store) RouteDecisions(taskID string) ([]RouteDecision, error) {
	rows, err := s.db.Query(
		`SELECT id,task_id,route,matched_rule,is_default,reason,created_at
		 FROM route_decision WHERE task_id=? ORDER BY created_at`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RouteDecision
	for rows.Next() {
		var d RouteDecision
		var isDef int
		var ts string
		if err := rows.Scan(&d.ID, &d.TaskID, &d.Route, &d.MatchedRule, &isDef, &d.Reason, &ts); err != nil {
			return nil, err
		}
		d.IsDefault = isDef != 0
		d.CreatedAt = parseTime(ts)
		out = append(out, d)
	}
	return out, rows.Err()
}

// CreateRun inserts a new run.
func (s *Store) CreateRun(r Run) error {
	now := nowOr(r.CreatedAt)
	_, err := s.db.Exec(
		`INSERT INTO run(id,title,body,agent,profile,model,status,matched_rule,machine,flow_id,exit_code,error,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Title, r.Body, r.Agent, r.Profile, r.Model, r.Status, r.MatchedRule, r.Machine, r.FlowID, r.ExitCode, r.Error, now, now)
	return err
}

// UpdateRunStatus updates a run and keeps any linked schedule occurrence truthful.
func (s *Store) UpdateRunStatus(id, status string, exitCode int, errMsg string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := nowOr(time.Time{})
	result, err := tx.Exec(
		`UPDATE run SET status=?, exit_code=?, error=?, updated_at=? WHERE id=?`,
		status, exitCode, errMsg, now, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed > 0 {
		if err := updateScheduleOccurrenceForRun(tx, id, status, errMsg, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// TransitionRunStatus atomically changes one flow run from an expected state.
// A waiting human gate is never a resumable claim, and linked schedule state is
// updated in the same transaction only for the caller that wins the transition.
func (s *Store) TransitionRunStatus(id, flowID, from, to string, exitCode int, errMsg string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	now := nowOr(time.Time{})
	result, err := tx.Exec(
		`UPDATE run SET status=?, exit_code=?, error=?, updated_at=?
		 WHERE id=? AND flow_id=? AND status=?
		   AND NOT EXISTS (
		     SELECT 1 FROM node_run WHERE node_run.run_id=? AND node_run.status='waiting'
		   )`,
		to, exitCode, errMsg, now, id, flowID, from, id,
	)
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if changed == 1 {
		if err := updateScheduleOccurrenceForRun(tx, id, to, errMsg, now); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return changed == 1, nil
}

// FailInterruptedDirectRuns reconciles direct tasks left running by an earlier
// daemon lifetime. Flow runs are intentionally excluded: their durable
// node_run state is the input to graph.Resume after a restart.
func (s *Store) FailInterruptedDirectRuns(reason string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(
		`SELECT id FROM run
		 WHERE status='running' AND (flow_id IS NULL OR flow_id='')`,
	)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	now := nowOr(time.Time{})
	for _, id := range ids {
		if _, err := tx.Exec(
			`UPDATE run SET status='failed', exit_code=-1, error=?, updated_at=?
			 WHERE id=? AND status='running'`,
			reason, now, id,
		); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(
			`INSERT INTO event(run_id,node_id,type,data,code,created_at)
			 VALUES(?,NULL,'error',?,-1,?)`,
			id, reason, now,
		); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(ids), nil
}

// GetRun returns a run by id.
func (s *Store) GetRun(id string) (Run, error) {
	row := s.db.QueryRow(
		`SELECT id,title,body,agent,profile,model,status,matched_rule,machine,flow_id,exit_code,error,created_at,updated_at
		 FROM run WHERE id=?`, id)
	return scanRun(row)
}

// ListRuns returns all runs, newest first.
func (s *Store) ListRuns() ([]Run, error) {
	rows, err := s.db.Query(
		`SELECT id,title,body,agent,profile,model,status,matched_rule,machine,flow_id,exit_code,error,created_at,updated_at
		 FROM run ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRun(row scanner) (Run, error) {
	var r Run
	var created, updated string
	var body, profile, model, machine sql.NullString
	err := row.Scan(&r.ID, &r.Title, &body, &r.Agent, &profile, &model, &r.Status, &r.MatchedRule, &machine, &r.FlowID,
		&r.ExitCode, &r.Error, &created, &updated)
	if err != nil {
		return Run{}, err
	}
	r.Body = body.String       // NULL (pre-migration rows) -> ""
	r.Profile = profile.String // NULL (pre-migration rows) -> ""
	r.Model = model.String     // NULL (pre-migration rows) -> ""
	r.Machine = machine.String // NULL (pre-migration rows) -> ""
	r.CreatedAt = parseTime(created)
	r.UpdatedAt = parseTime(updated)
	return r, nil
}

// UpsertNodeRun inserts or updates a node run (keyed by id = runID:nodeID).
func (s *Store) UpsertNodeRun(n NodeRun) error {
	now := nowOr(time.Time{})
	_, err := s.db.Exec(
		`INSERT INTO node_run(id,run_id,node_id,type,status,input,output,attempts,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET status=excluded.status, input=excluded.input,
		   output=excluded.output, attempts=excluded.attempts, updated_at=excluded.updated_at`,
		n.ID, n.RunID, n.NodeID, n.Type, n.Status, n.Input, n.Output, n.Attempts, now, now)
	return err
}

// DecideWaitingGate atomically changes one waiting gate to a terminal decision.
// The false result means another caller already decided or reset that gate.
func (s *Store) DecideWaitingGate(id, status, output string) (bool, error) {
	result, err := s.db.Exec(
		`UPDATE node_run SET status=?, output=?, updated_at=? WHERE id=? AND status='waiting'`,
		status, output, nowOr(time.Time{}), id,
	)
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	return changed == 1, err
}

// NodeRuns returns the node runs for a run, in creation order.
func (s *Store) NodeRuns(runID string) ([]NodeRun, error) {
	rows, err := s.db.Query(
		`SELECT id,run_id,node_id,type,status,input,output,attempts,created_at,updated_at
		 FROM node_run WHERE run_id=? ORDER BY created_at`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeRun
	for rows.Next() {
		var n NodeRun
		var created, updated string
		if err := rows.Scan(&n.ID, &n.RunID, &n.NodeID, &n.Type, &n.Status,
			&n.Input, &n.Output, &n.Attempts, &created, &updated); err != nil {
			return nil, err
		}
		n.CreatedAt = parseTime(created)
		n.UpdatedAt = parseTime(updated)
		out = append(out, n)
	}
	return out, rows.Err()
}

// AllNodeRuns returns every node_run row grouped by run (the board's
// checkpoint-summary source, spec 033).
func (s *Store) AllNodeRuns() ([]NodeRun, error) {
	rows, err := s.db.Query(
		`SELECT id,run_id,node_id,type,status,input,output,attempts,created_at,updated_at
		 FROM node_run ORDER BY run_id, created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeRun
	for rows.Next() {
		var n NodeRun
		var created, updated string
		if err := rows.Scan(&n.ID, &n.RunID, &n.NodeID, &n.Type, &n.Status,
			&n.Input, &n.Output, &n.Attempts, &created, &updated); err != nil {
			return nil, err
		}
		n.CreatedAt = parseTime(created)
		n.UpdatedAt = parseTime(updated)
		out = append(out, n)
	}
	return out, rows.Err()
}

// WaitingGates returns every gate node currently awaiting a human decision,
// across all runs (the gate-inbox source).
func (s *Store) WaitingGates() ([]NodeRun, error) {
	rows, err := s.db.Query(
		`SELECT id,run_id,node_id,type,status,input,output,attempts,created_at,updated_at
		 FROM node_run WHERE type='gate' AND status='waiting' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeRun
	for rows.Next() {
		var n NodeRun
		var created, updated string
		if err := rows.Scan(&n.ID, &n.RunID, &n.NodeID, &n.Type, &n.Status,
			&n.Input, &n.Output, &n.Attempts, &created, &updated); err != nil {
			return nil, err
		}
		n.CreatedAt = parseTime(created)
		n.UpdatedAt = parseTime(updated)
		out = append(out, n)
	}
	return out, rows.Err()
}

// AppendEvent appends an event (append-only) and returns its id.
func (s *Store) AppendEvent(e Event) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO event(run_id,node_id,type,data,code,created_at) VALUES(?,?,?,?,?,?)`,
		e.RunID, e.NodeID, e.Type, e.Data, e.Code, nowOr(e.CreatedAt))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Events returns all events for a run, in insertion order.
func (s *Store) Events(runID string) ([]Event, error) {
	return s.queryEvents(`SELECT id,run_id,node_id,type,data,code,created_at FROM event WHERE run_id=? ORDER BY id`, runID)
}

// EventsSince returns events with id greater than the cursor (the UI feed tail).
func (s *Store) EventsSince(cursor int64) ([]Event, error) {
	return s.queryEvents(`SELECT id,run_id,node_id,type,data,code,created_at FROM event WHERE id>? ORDER BY id`, cursor)
}

func (s *Store) queryEvents(q string, arg any) ([]Event, error) {
	rows, err := s.db.Query(q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var ts string
		var nodeID sql.NullString
		if err := rows.Scan(&e.ID, &e.RunID, &nodeID, &e.Type, &e.Data, &e.Code, &ts); err != nil {
			return nil, err
		}
		e.NodeID = nodeID.String
		e.CreatedAt = parseTime(ts)
		out = append(out, e)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
