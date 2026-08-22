package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

// routineSchema is additive. The legacy schedule and schedule_occurrence
// tables remain the v1 scheduler authority until the approved cutover.
const routineSchema = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_stable_agent_account_id
  ON stable_agent(account_id,id);

CREATE TABLE IF NOT EXISTS routine (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  agent_id TEXT NOT NULL,
  authority TEXT NOT NULL CHECK(authority='fort_cloud'),
  state TEXT NOT NULL CHECK(state IN ('active','paused','archived')),
  pause_reason TEXT NOT NULL DEFAULT '' CHECK(pause_reason IN ('','needs_revalidation')),
  current_revision_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(account_id,id),
  UNIQUE(account_id,agent_id,id),
  FOREIGN KEY(account_id,agent_id) REFERENCES stable_agent(account_id,id),
  CHECK((state='paused' AND pause_reason IN ('','needs_revalidation')) OR
        (state<>'paused' AND pause_reason=''))
);
CREATE INDEX IF NOT EXISTS idx_routine_agent_state
  ON routine(account_id,agent_id,state,created_at,id);

CREATE TABLE IF NOT EXISTS routine_revision (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  routine_id TEXT NOT NULL,
  agent_id TEXT NOT NULL,
  revision INTEGER NOT NULL CHECK(revision>0),
  behavior_revision_id TEXT NOT NULL,
  binding_revision_id TEXT NOT NULL,
  authority TEXT NOT NULL CHECK(authority='fort_cloud'),
  trigger_kind TEXT NOT NULL CHECK(trigger_kind IN ('schedule','event')),
  schedule_expression TEXT NOT NULL,
  timezone TEXT NOT NULL,
  next_occurrence_at TEXT,
  input_source TEXT NOT NULL,
  freshness_seconds INTEGER NOT NULL CHECK(freshness_seconds>0),
  expected_result TEXT NOT NULL,
  result_conversation_id TEXT NOT NULL,
  approval_boundary TEXT NOT NULL,
  missing_input_behavior TEXT NOT NULL,
  retry_policy TEXT NOT NULL,
  catch_up_policy TEXT NOT NULL,
  lateness_policy TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(account_id,id),
  UNIQUE(account_id,routine_id,id),
  UNIQUE(account_id,routine_id,revision),
  UNIQUE(account_id,routine_id,agent_id,id),
  UNIQUE(account_id,routine_id,id,agent_id,behavior_revision_id,binding_revision_id,result_conversation_id),
  UNIQUE(id,agent_id),
  FOREIGN KEY(account_id,agent_id,routine_id) REFERENCES routine(account_id,agent_id,id),
  FOREIGN KEY(behavior_revision_id,agent_id) REFERENCES agent_behavior_revision(id,agent_id),
  FOREIGN KEY(binding_revision_id,agent_id) REFERENCES agent_binding_revision(id,agent_id),
  FOREIGN KEY(result_conversation_id) REFERENCES conversation(id),
  CHECK((trigger_kind='schedule' AND length(trim(schedule_expression))>0 AND
         length(trim(timezone))>0 AND next_occurrence_at IS NOT NULL) OR
        (trigger_kind='event'))
);

CREATE TABLE IF NOT EXISTS routine_create_idempotency (
  account_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  command_digest TEXT NOT NULL CHECK(length(command_digest)=64 AND command_digest NOT GLOB '*[^0-9a-f]*'),
  routine_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY(account_id,idempotency_key),
  FOREIGN KEY(account_id,routine_id) REFERENCES routine(account_id,id)
);

CREATE TABLE IF NOT EXISTS routine_revalidate_idempotency (
  account_id TEXT NOT NULL,
  routine_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  command_digest TEXT NOT NULL CHECK(length(command_digest)=64 AND command_digest NOT GLOB '*[^0-9a-f]*'),
  routine_revision_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY(account_id,routine_id,idempotency_key),
  FOREIGN KEY(account_id,routine_id) REFERENCES routine(account_id,id),
  FOREIGN KEY(account_id,routine_revision_id) REFERENCES routine_revision(account_id,id)
);

CREATE TABLE IF NOT EXISTS source_routine_projection (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  execution_source_id TEXT NOT NULL,
  opaque_source_routine_id TEXT NOT NULL,
  projection_revision INTEGER NOT NULL CHECK(projection_revision>0),
  authority TEXT NOT NULL CHECK(authority='source_native'),
  schedule_snapshot TEXT NOT NULL CHECK(json_valid(schedule_snapshot) AND json_type(schedule_snapshot)='object'),
  projection_digest TEXT NOT NULL CHECK(length(projection_digest)=64 AND projection_digest NOT GLOB '*[^0-9a-f]*'),
  last_occurrence_at TEXT,
  next_occurrence_at TEXT,
  observed_at TEXT NOT NULL,
  UNIQUE(account_id,id),
  UNIQUE(account_id,execution_source_id,opaque_source_routine_id,projection_revision),
  FOREIGN KEY(execution_source_id) REFERENCES execution_source(id)
);
CREATE INDEX IF NOT EXISTS idx_source_routine_projection_source
  ON source_routine_projection(account_id,execution_source_id,observed_at,id);

CREATE TABLE IF NOT EXISTS routine_import_receipt (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  source_routine_projection_id TEXT NOT NULL,
  routine_id TEXT NOT NULL,
  routine_revision_id TEXT NOT NULL,
  source_disabled_at TEXT NOT NULL,
  exact_last_source_occurrence_at TEXT,
  exact_next_source_occurrence_at TEXT,
  fencing_receipt_ciphertext BLOB NOT NULL CHECK(length(fencing_receipt_ciphertext)>0),
  fencing_receipt_key_id TEXT NOT NULL,
  fencing_receipt_nonce BLOB NOT NULL CHECK(length(fencing_receipt_nonce)>=12),
  fencing_receipt_digest TEXT NOT NULL CHECK(length(fencing_receipt_digest)=64 AND fencing_receipt_digest NOT GLOB '*[^0-9a-f]*'),
  imported_at TEXT NOT NULL,
  UNIQUE(account_id,id),
  UNIQUE(account_id,source_routine_projection_id),
  UNIQUE(account_id,routine_id,routine_revision_id),
  FOREIGN KEY(account_id,source_routine_projection_id) REFERENCES source_routine_projection(account_id,id),
  FOREIGN KEY(account_id,routine_id) REFERENCES routine(account_id,id),
  FOREIGN KEY(account_id,routine_revision_id) REFERENCES routine_revision(account_id,id)
);

CREATE TABLE IF NOT EXISTS routine_occurrence (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  routine_id TEXT NOT NULL,
  routine_revision_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK(kind IN ('scheduled','test')),
  state TEXT NOT NULL CHECK(state IN ('queued','working','needs_you','succeeded','failed','canceled')),
  scheduled_for TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  command_digest TEXT NOT NULL CHECK(length(command_digest)=64 AND command_digest NOT GLOB '*[^0-9a-f]*'),
  approval_evidence_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(account_id,id),
  UNIQUE(account_id,routine_id,scheduled_for),
  UNIQUE(account_id,routine_id,idempotency_key),
  UNIQUE(account_id,routine_id,id,routine_revision_id),
  FOREIGN KEY(account_id,routine_id) REFERENCES routine(account_id,id),
  FOREIGN KEY(account_id,routine_id,routine_revision_id)
    REFERENCES routine_revision(account_id,routine_id,id)
);
CREATE INDEX IF NOT EXISTS idx_routine_occurrence_history
  ON routine_occurrence(account_id,routine_id,created_at,id);

CREATE TABLE IF NOT EXISTS routine_run (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  occurrence_id TEXT NOT NULL,
  routine_id TEXT NOT NULL,
  routine_revision_id TEXT NOT NULL,
  agent_id TEXT NOT NULL,
  behavior_revision_id TEXT NOT NULL,
  binding_revision_id TEXT NOT NULL,
  result_conversation_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK(kind IN ('scheduled','test')),
  state TEXT NOT NULL CHECK(state IN ('queued','working','needs_you','succeeded','failed','canceled')),
  current_attempt_id TEXT,
  current_lease_id TEXT,
  lease_expires_at TEXT,
  normalized_result TEXT,
  result_message_id INTEGER,
  failure_code TEXT,
  next_action TEXT,
  terminal_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(account_id,id),
  UNIQUE(account_id,occurrence_id),
  FOREIGN KEY(account_id,routine_id,occurrence_id,routine_revision_id)
    REFERENCES routine_occurrence(account_id,routine_id,id,routine_revision_id),
  FOREIGN KEY(account_id,routine_id,routine_revision_id,agent_id,behavior_revision_id,binding_revision_id,result_conversation_id)
    REFERENCES routine_revision(account_id,routine_id,id,agent_id,behavior_revision_id,binding_revision_id,result_conversation_id),
  FOREIGN KEY(result_conversation_id) REFERENCES conversation(id),
  FOREIGN KEY(result_message_id) REFERENCES conversation_message(id),
  CHECK((current_attempt_id IS NULL AND current_lease_id IS NULL AND lease_expires_at IS NULL) OR
        (length(trim(current_attempt_id))>0 AND length(trim(current_lease_id))>0 AND lease_expires_at IS NOT NULL)),
  CHECK((state='succeeded' AND normalized_result IS NOT NULL AND result_message_id IS NOT NULL AND terminal_at IS NOT NULL) OR
        (state IN ('failed','canceled') AND normalized_result IS NULL AND result_message_id IS NULL AND terminal_at IS NOT NULL) OR
        (state NOT IN ('succeeded','failed','canceled') AND normalized_result IS NULL AND result_message_id IS NULL AND terminal_at IS NULL))
);
CREATE INDEX IF NOT EXISTS idx_routine_run_history
  ON routine_run(account_id,routine_id,created_at,id);

CREATE TABLE IF NOT EXISTS routine_run_activity (
  sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  account_id TEXT NOT NULL,
  run_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  command_digest TEXT NOT NULL CHECK(length(command_digest)=64 AND command_digest NOT GLOB '*[^0-9a-f]*'),
  state TEXT NOT NULL CHECK(state IN ('queued','working','needs_you','succeeded','failed','canceled')),
  attempt_id TEXT,
  lease_id TEXT,
  lease_expires_at TEXT,
  activity TEXT NOT NULL,
  failure_code TEXT,
  next_action TEXT,
  created_at TEXT NOT NULL,
  UNIQUE(account_id,run_id,idempotency_key),
  FOREIGN KEY(account_id,run_id) REFERENCES routine_run(account_id,id),
  CHECK((attempt_id IS NULL AND lease_id IS NULL AND lease_expires_at IS NULL) OR
        (length(trim(attempt_id))>0 AND length(trim(lease_id))>0 AND lease_expires_at IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS routine_result (
  account_id TEXT NOT NULL,
  run_id TEXT NOT NULL,
  conversation_id TEXT NOT NULL,
  message_id INTEGER NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  PRIMARY KEY(account_id,run_id),
  FOREIGN KEY(account_id,run_id) REFERENCES routine_run(account_id,id),
  FOREIGN KEY(conversation_id) REFERENCES conversation(id),
  FOREIGN KEY(message_id) REFERENCES conversation_message(id)
);

CREATE TRIGGER IF NOT EXISTS routine_pause_on_agent_revision_change
AFTER UPDATE OF current_behavior_revision_id,current_binding_revision_id ON stable_agent
WHEN OLD.current_behavior_revision_id<>NEW.current_behavior_revision_id
  OR OLD.current_binding_revision_id<>NEW.current_binding_revision_id
BEGIN
  UPDATE routine
  SET state='paused',pause_reason='needs_revalidation',
      updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')
  WHERE account_id=NEW.account_id AND agent_id=NEW.id AND state='active';
END;

CREATE TRIGGER IF NOT EXISTS routine_identity_immutable
BEFORE UPDATE OF id,account_id,agent_id,authority,created_at ON routine
BEGIN SELECT RAISE(ABORT,'routine_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_delete_immutable
BEFORE DELETE ON routine BEGIN SELECT RAISE(ABORT,'routine_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_revision_immutable_update
BEFORE UPDATE ON routine_revision BEGIN SELECT RAISE(ABORT,'routine_revision_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_revision_immutable_delete
BEFORE DELETE ON routine_revision BEGIN SELECT RAISE(ABORT,'routine_revision_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_create_idempotency_immutable_update
BEFORE UPDATE ON routine_create_idempotency BEGIN SELECT RAISE(ABORT,'routine_idempotency_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_create_idempotency_immutable_delete
BEFORE DELETE ON routine_create_idempotency BEGIN SELECT RAISE(ABORT,'routine_idempotency_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_revalidate_idempotency_immutable_update
BEFORE UPDATE ON routine_revalidate_idempotency BEGIN SELECT RAISE(ABORT,'routine_idempotency_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_revalidate_idempotency_immutable_delete
BEFORE DELETE ON routine_revalidate_idempotency BEGIN SELECT RAISE(ABORT,'routine_idempotency_immutable'); END;
CREATE TRIGGER IF NOT EXISTS source_routine_projection_immutable_update
BEFORE UPDATE ON source_routine_projection BEGIN SELECT RAISE(ABORT,'source_routine_projection_immutable'); END;
CREATE TRIGGER IF NOT EXISTS source_routine_projection_immutable_delete
BEFORE DELETE ON source_routine_projection BEGIN SELECT RAISE(ABORT,'source_routine_projection_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_import_receipt_immutable_update
BEFORE UPDATE ON routine_import_receipt BEGIN SELECT RAISE(ABORT,'routine_import_receipt_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_import_receipt_immutable_delete
BEFORE DELETE ON routine_import_receipt BEGIN SELECT RAISE(ABORT,'routine_import_receipt_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_occurrence_identity_immutable
BEFORE UPDATE OF id,account_id,routine_id,routine_revision_id,kind,scheduled_for,idempotency_key,
command_digest,approval_evidence_id,created_at ON routine_occurrence
BEGIN SELECT RAISE(ABORT,'routine_occurrence_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_occurrence_terminal
BEFORE UPDATE OF state ON routine_occurrence WHEN OLD.state IN ('succeeded','failed','canceled')
BEGIN SELECT RAISE(ABORT,'routine_occurrence_terminal'); END;
CREATE TRIGGER IF NOT EXISTS routine_occurrence_delete_immutable
BEFORE DELETE ON routine_occurrence BEGIN SELECT RAISE(ABORT,'routine_occurrence_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_run_identity_immutable
BEFORE UPDATE OF id,account_id,occurrence_id,routine_id,routine_revision_id,agent_id,behavior_revision_id,
binding_revision_id,result_conversation_id,kind,created_at ON routine_run
BEGIN SELECT RAISE(ABORT,'routine_run_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_run_terminal
BEFORE UPDATE ON routine_run WHEN OLD.state IN ('succeeded','failed','canceled')
BEGIN SELECT RAISE(ABORT,'routine_run_terminal'); END;
CREATE TRIGGER IF NOT EXISTS routine_run_delete_immutable
BEFORE DELETE ON routine_run BEGIN SELECT RAISE(ABORT,'routine_run_identity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_run_activity_immutable_update
BEFORE UPDATE ON routine_run_activity BEGIN SELECT RAISE(ABORT,'routine_run_activity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_run_activity_immutable_delete
BEFORE DELETE ON routine_run_activity BEGIN SELECT RAISE(ABORT,'routine_run_activity_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_result_immutable_update
BEFORE UPDATE ON routine_result BEGIN SELECT RAISE(ABORT,'routine_result_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_result_immutable_delete
BEFORE DELETE ON routine_result BEGIN SELECT RAISE(ABORT,'routine_result_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_result_message_immutable_update
BEFORE UPDATE ON conversation_message
WHEN EXISTS(SELECT 1 FROM routine_result WHERE message_id=OLD.id)
BEGIN SELECT RAISE(ABORT,'routine_result_immutable'); END;
CREATE TRIGGER IF NOT EXISTS routine_result_message_immutable_delete
BEFORE DELETE ON conversation_message
WHEN EXISTS(SELECT 1 FROM routine_result WHERE message_id=OLD.id)
BEGIN SELECT RAISE(ABORT,'routine_result_immutable'); END;
`

func (s *Store) migrateRoutines() error {
	_, err := s.db.Exec(routineSchema)
	return err
}

var _ ledger.RoutineRepository = (*Store)(nil)

func (s *Store) RecordSourceRoutineProjection(ctx context.Context, projection ledger.SourceRoutineProjection) (ledger.SourceRoutineProjection, error) {
	if err := projection.Validate(); err != nil {
		return ledger.SourceRoutineProjection{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.SourceRoutineProjection{}, err
	}
	defer tx.Rollback()

	existing, err := getSourceRoutineProjection(ctx, tx, projection.AccountID, projection.ID)
	if err == nil {
		equal, compareErr := canonicalEqual(existing, projection)
		if compareErr != nil {
			return ledger.SourceRoutineProjection{}, compareErr
		}
		if !equal {
			return ledger.SourceRoutineProjection{}, fmt.Errorf("%w: source Routine projection %q", ledger.ErrIdempotencyConflict, projection.ID)
		}
		return existing, nil
	}
	if !errors.Is(err, ledger.ErrNotFound) {
		return ledger.SourceRoutineProjection{}, err
	}
	var sourceExists int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM execution_source WHERE id=? AND account_id=?)`,
		projection.ExecutionSourceID, projection.AccountID).Scan(&sourceExists); err != nil {
		return ledger.SourceRoutineProjection{}, err
	}
	if sourceExists != 1 {
		return ledger.SourceRoutineProjection{}, fmt.Errorf("source Routine projection Execution Source belongs to another account")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO source_routine_projection(
id,account_id,execution_source_id,opaque_source_routine_id,projection_revision,authority,schedule_snapshot,
projection_digest,last_occurrence_at,next_occurrence_at,observed_at) VALUES(?,?,?,?,?,'source_native',?,?,?,?,?)`,
		projection.ID, projection.AccountID, projection.ExecutionSourceID, projection.OpaqueSourceRoutineID,
		projection.ProjectionRevision, string(projection.ScheduleSnapshot), projection.ProjectionDigest,
		nullableTime(projection.LastOccurrenceAt), nullableTime(projection.NextOccurrenceAt), nowOr(projection.ObservedAt)); err != nil {
		return ledger.SourceRoutineProjection{}, err
	}
	stored, err := getSourceRoutineProjection(ctx, tx, projection.AccountID, projection.ID)
	if err != nil {
		return ledger.SourceRoutineProjection{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.SourceRoutineProjection{}, err
	}
	return stored, nil
}

func (s *Store) ListSourceRoutineProjections(ctx context.Context, accountID, executionSourceID string) ([]ledger.SourceRoutineProjection, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(executionSourceID) == "" {
		return nil, fmt.Errorf("source Routine projection account and Execution Source ids are required")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,account_id,execution_source_id,opaque_source_routine_id,
projection_revision,schedule_snapshot,projection_digest,last_occurrence_at,next_occurrence_at,observed_at
FROM source_routine_projection WHERE account_id=? AND execution_source_id=? ORDER BY observed_at,id`,
		accountID, executionSourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ledger.SourceRoutineProjection{}
	for rows.Next() {
		item, err := scanSourceRoutineProjection(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func getSourceRoutineProjection(ctx context.Context, queryer agentQueryer, accountID, projectionID string) (ledger.SourceRoutineProjection, error) {
	projection, err := scanSourceRoutineProjection(queryer.QueryRowContext(ctx, `SELECT id,account_id,execution_source_id,
opaque_source_routine_id,projection_revision,schedule_snapshot,projection_digest,last_occurrence_at,
next_occurrence_at,observed_at FROM source_routine_projection WHERE account_id=? AND id=?`, accountID, projectionID))
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.SourceRoutineProjection{}, fmt.Errorf("%w: source Routine projection %q", ledger.ErrNotFound, projectionID)
	}
	return projection, err
}

func scanSourceRoutineProjection(row scanner) (ledger.SourceRoutineProjection, error) {
	var projection ledger.SourceRoutineProjection
	var scheduleSnapshot, observedAt string
	var lastOccurrence, nextOccurrence sql.NullString
	if err := row.Scan(&projection.ID, &projection.AccountID, &projection.ExecutionSourceID,
		&projection.OpaqueSourceRoutineID, &projection.ProjectionRevision, &scheduleSnapshot,
		&projection.ProjectionDigest, &lastOccurrence, &nextOccurrence, &observedAt); err != nil {
		return ledger.SourceRoutineProjection{}, err
	}
	projection.ScheduleSnapshot = []byte(scheduleSnapshot)
	projection.LastOccurrenceAt = parseTime(lastOccurrence.String)
	projection.NextOccurrenceAt = parseTime(nextOccurrence.String)
	projection.ObservedAt = parseTime(observedAt)
	return projection, nil
}

func (s *Store) CreateRoutine(ctx context.Context, command ledger.CreateRoutineCommand) (ledger.RoutineRecord, error) {
	digest, err := command.Digest()
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	defer tx.Rollback()

	var existingDigest, existingRoutineID string
	err = tx.QueryRowContext(ctx, `SELECT command_digest,routine_id FROM routine_create_idempotency
WHERE account_id=? AND idempotency_key=?`, command.Routine.AccountID, command.IdempotencyKey).Scan(
		&existingDigest, &existingRoutineID)
	if err == nil {
		if existingDigest != digest {
			return ledger.RoutineRecord{}, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.IdempotencyKey)
		}
		return getRoutineRecord(ctx, tx, command.Routine.AccountID, existingRoutineID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ledger.RoutineRecord{}, err
	}
	if err := insertRoutineRows(ctx, tx, command, digest); err != nil {
		return ledger.RoutineRecord{}, err
	}
	record, err := getRoutineRecord(ctx, tx, command.Routine.AccountID, command.Routine.ID)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.RoutineRecord{}, err
	}
	return record, nil
}

func insertRoutineRows(ctx context.Context, tx *sql.Tx, command ledger.CreateRoutineCommand, digest string) error {
	if err := command.Validate(); err != nil {
		return err
	}
	var agentExists int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
  SELECT 1 FROM stable_agent WHERE account_id=? AND id=?
    AND current_behavior_revision_id=? AND current_binding_revision_id=?
)`, command.Routine.AccountID, command.Routine.AgentID, command.Revision.BehaviorRevisionID,
		command.Revision.BindingRevisionID).Scan(&agentExists); err != nil {
		return err
	}
	if agentExists != 1 {
		return fmt.Errorf("Routine must pin the owning Agent's current Behavior and Binding Revisions")
	}
	resultConversationExists, err := routineResultConversationExists(ctx, tx, command.Routine.AccountID,
		command.Routine.AgentID, command.Revision.ResultConversationID)
	if err != nil {
		return err
	}
	if !resultConversationExists {
		return fmt.Errorf("Routine result Conversation does not exist")
	}

	createdAt := nowOr(command.Routine.CreatedAt)
	if _, err := tx.ExecContext(ctx, `INSERT INTO routine(
id,account_id,agent_id,authority,state,pause_reason,current_revision_id,created_at,updated_at
) VALUES(?,?,?,?,?,'',?,?,?)`, command.Routine.ID, command.Routine.AccountID, command.Routine.AgentID,
		command.Revision.Authority, command.Routine.State, command.Routine.CurrentRevisionID, createdAt, createdAt); err != nil {
		return err
	}
	if err := insertRoutineRevision(ctx, tx, command.Routine.AccountID, command.Revision); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO routine_create_idempotency(
account_id,idempotency_key,command_digest,routine_id,created_at) VALUES(?,?,?,?,?)`,
		command.Routine.AccountID, command.IdempotencyKey, digest, command.Routine.ID, createdAt); err != nil {
		return err
	}
	return nil
}

func (s *Store) ImportSourceRoutine(ctx context.Context, command ledger.ImportRoutineCommand) (ledger.RoutineRecord, error) {
	digest, err := command.Digest()
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	defer tx.Rollback()

	var existingDigest, existingRoutineID string
	err = tx.QueryRowContext(ctx, `SELECT command_digest,routine_id FROM routine_create_idempotency
WHERE account_id=? AND idempotency_key=?`, command.Create.Routine.AccountID, command.Create.IdempotencyKey).Scan(
		&existingDigest, &existingRoutineID)
	if err == nil {
		if existingDigest != digest {
			return ledger.RoutineRecord{}, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.Create.IdempotencyKey)
		}
		return getRoutineRecord(ctx, tx, command.Create.Routine.AccountID, existingRoutineID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ledger.RoutineRecord{}, err
	}
	projection, err := getSourceRoutineProjection(ctx, tx, command.Create.Routine.AccountID,
		command.Receipt.SourceRoutineProjectionID)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	if err := command.Validate(projection); err != nil {
		return ledger.RoutineRecord{}, err
	}
	if err := insertRoutineRows(ctx, tx, command.Create, digest); err != nil {
		return ledger.RoutineRecord{}, err
	}
	receipt := command.Receipt
	if _, err := tx.ExecContext(ctx, `INSERT INTO routine_import_receipt(
id,account_id,source_routine_projection_id,routine_id,routine_revision_id,source_disabled_at,
exact_last_source_occurrence_at,exact_next_source_occurrence_at,fencing_receipt_ciphertext,
fencing_receipt_key_id,fencing_receipt_nonce,fencing_receipt_digest,imported_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, receipt.ID, receipt.AccountID, receipt.SourceRoutineProjectionID,
		receipt.RoutineID, receipt.RoutineRevisionID, nowOr(receipt.SourceDisabledAt),
		nullableTime(receipt.ExactLastSourceOccurrenceAt), nullableTime(receipt.ExactNextSourceOccurrenceAt),
		receipt.FencingReceiptCiphertext, receipt.FencingReceiptKeyID, receipt.FencingReceiptNonce,
		receipt.FencingReceiptDigest, nowOr(receipt.ImportedAt)); err != nil {
		return ledger.RoutineRecord{}, err
	}
	record, err := getRoutineRecord(ctx, tx, command.Create.Routine.AccountID, command.Create.Routine.ID)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.RoutineRecord{}, err
	}
	return record, nil
}

func (s *Store) RevalidateRoutine(ctx context.Context, command ledger.RevalidateRoutineCommand) (ledger.RoutineRecord, error) {
	digest, err := command.Digest()
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	defer tx.Rollback()

	var existingDigest, existingRevisionID string
	err = tx.QueryRowContext(ctx, `SELECT command_digest,routine_revision_id
FROM routine_revalidate_idempotency WHERE account_id=? AND routine_id=? AND idempotency_key=?`,
		command.AccountID, command.RoutineID, command.IdempotencyKey).Scan(&existingDigest, &existingRevisionID)
	if err == nil {
		if existingDigest != digest || existingRevisionID != command.Revision.ID {
			return ledger.RoutineRecord{}, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.IdempotencyKey)
		}
		return getRoutineRecord(ctx, tx, command.AccountID, command.RoutineID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ledger.RoutineRecord{}, err
	}
	record, err := getRoutineRecord(ctx, tx, command.AccountID, command.RoutineID)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	if err := command.Validate(record); err != nil {
		return ledger.RoutineRecord{}, err
	}
	var agentCurrent int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM stable_agent
WHERE account_id=? AND id=? AND current_behavior_revision_id=? AND current_binding_revision_id=?)`,
		command.AccountID, record.Routine.AgentID, command.Revision.BehaviorRevisionID,
		command.Revision.BindingRevisionID).Scan(&agentCurrent); err != nil {
		return ledger.RoutineRecord{}, err
	}
	if agentCurrent != 1 {
		return ledger.RoutineRecord{}, fmt.Errorf("Routine revalidation must pin the Agent's current Behavior and Binding Revisions")
	}
	resultConversationExists, err := routineResultConversationExists(ctx, tx, command.AccountID,
		record.Routine.AgentID, command.Revision.ResultConversationID)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	if !resultConversationExists {
		return ledger.RoutineRecord{}, fmt.Errorf("Routine result Conversation does not exist")
	}
	if err := insertRoutineRevision(ctx, tx, command.AccountID, command.Revision); err != nil {
		return ledger.RoutineRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE routine SET current_revision_id=?,state='active',pause_reason='',updated_at=?
WHERE account_id=? AND id=? AND state='paused' AND pause_reason='needs_revalidation'`, command.Revision.ID,
		nowOr(command.Revision.CreatedAt), command.AccountID, command.RoutineID); err != nil {
		return ledger.RoutineRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO routine_revalidate_idempotency(
account_id,routine_id,idempotency_key,command_digest,routine_revision_id,created_at) VALUES(?,?,?,?,?,?)`,
		command.AccountID, command.RoutineID, command.IdempotencyKey, digest, command.Revision.ID,
		nowOr(command.Revision.CreatedAt)); err != nil {
		return ledger.RoutineRecord{}, err
	}
	record, err = getRoutineRecord(ctx, tx, command.AccountID, command.RoutineID)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.RoutineRecord{}, err
	}
	return record, nil
}

func (s *Store) EnqueueRoutineOccurrence(ctx context.Context, command ledger.EnqueueRoutineOccurrenceCommand) (ledger.RoutineRunRecord, error) {
	digest, err := command.Digest()
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	defer tx.Rollback()

	var existingDigest, existingOccurrenceID, existingRunID string
	err = tx.QueryRowContext(ctx, `SELECT occurrence.command_digest,occurrence.id,run.id
FROM routine_occurrence occurrence JOIN routine_run run
  ON run.account_id=occurrence.account_id AND run.occurrence_id=occurrence.id
WHERE occurrence.account_id=? AND occurrence.routine_id=? AND occurrence.idempotency_key=?`,
		command.AccountID, command.RoutineID, command.IdempotencyKey).Scan(
		&existingDigest, &existingOccurrenceID, &existingRunID)
	if err == nil {
		if existingDigest != digest {
			return ledger.RoutineRunRecord{}, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.IdempotencyKey)
		}
		return getRoutineRunRecord(ctx, tx, command.AccountID, existingRunID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ledger.RoutineRunRecord{}, err
	}
	if err := command.Validate(); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	routine, err := getRoutineRecord(ctx, tx, command.AccountID, command.RoutineID)
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	if routine.PauseReason == ledger.RoutinePauseNeedsRevalidation {
		return ledger.RoutineRunRecord{}, ledger.ErrRoutineNeedsRevalidation
	}
	if routine.Routine.State != conversation.RoutineActive || routine.PauseReason != "" {
		return ledger.RoutineRunRecord{}, fmt.Errorf("Routine is not active")
	}
	if routine.CurrentRevision.ID != command.RoutineRevisionID {
		return ledger.RoutineRunRecord{}, fmt.Errorf("Routine occurrence must pin the current Routine Revision")
	}
	var currentBehaviorRevisionID, currentBindingRevisionID string
	if err := tx.QueryRowContext(ctx, `SELECT current_behavior_revision_id,current_binding_revision_id
FROM stable_agent WHERE account_id=? AND id=?`, command.AccountID, routine.Routine.AgentID).Scan(
		&currentBehaviorRevisionID, &currentBindingRevisionID); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	if routine.CurrentRevision.BehaviorRevisionID != currentBehaviorRevisionID ||
		routine.CurrentRevision.BindingRevisionID != currentBindingRevisionID {
		if _, err := tx.ExecContext(ctx, `UPDATE routine SET state='paused',pause_reason='needs_revalidation',updated_at=?
WHERE account_id=? AND id=? AND state='active'`, nowOr(command.CreatedAt), command.AccountID, command.RoutineID); err != nil {
			return ledger.RoutineRunRecord{}, err
		}
		if err := tx.Commit(); err != nil {
			return ledger.RoutineRunRecord{}, err
		}
		return ledger.RoutineRunRecord{}, ledger.ErrRoutineNeedsRevalidation
	}
	var scheduledCollision int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM routine_occurrence
WHERE account_id=? AND routine_id=? AND scheduled_for=?)`, command.AccountID, command.RoutineID,
		nowOr(command.ScheduledFor)).Scan(&scheduledCollision); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	if scheduledCollision == 1 {
		return ledger.RoutineRunRecord{}, fmt.Errorf("%w: Routine occurrence at %s", ledger.ErrIdempotencyConflict,
			command.ScheduledFor.UTC().Format(time.RFC3339Nano))
	}

	createdAt := nowOr(command.CreatedAt)
	if _, err := tx.ExecContext(ctx, `INSERT INTO routine_occurrence(
id,account_id,routine_id,routine_revision_id,kind,state,scheduled_for,idempotency_key,command_digest,
approval_evidence_id,created_at,updated_at) VALUES(?,?,?,?,?,'queued',?,?,?,?,?,?)`, command.OccurrenceID,
		command.AccountID, command.RoutineID, command.RoutineRevisionID, command.Kind,
		nowOr(command.ScheduledFor), command.IdempotencyKey, digest, command.ApprovalEvidenceID, createdAt, createdAt); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO routine_run(
id,account_id,occurrence_id,routine_id,routine_revision_id,agent_id,behavior_revision_id,binding_revision_id,
result_conversation_id,kind,state,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,'queued',?,?)`,
		command.RunID, command.AccountID, command.OccurrenceID, command.RoutineID, command.RoutineRevisionID,
		routine.Routine.AgentID, routine.CurrentRevision.BehaviorRevisionID, routine.CurrentRevision.BindingRevisionID,
		routine.CurrentRevision.ResultConversationID, command.Kind, createdAt, createdAt); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO routine_run_activity(
account_id,run_id,idempotency_key,command_digest,state,activity,created_at
) VALUES(?,?,?,?,?,'occurrence queued',?)`, command.AccountID, command.RunID, command.IdempotencyKey,
		digest, conversation.RoutineRunQueued, createdAt); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	record, err := getRoutineRunRecord(ctx, tx, command.AccountID, command.RunID)
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	return record, nil
}

func (s *Store) AdvanceRoutineRun(ctx context.Context, command ledger.AdvanceRoutineRunCommand) (ledger.RoutineRunRecord, error) {
	digest, err := command.Digest()
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	defer tx.Rollback()

	var existingDigest string
	err = tx.QueryRowContext(ctx, `SELECT command_digest FROM routine_run_activity
WHERE account_id=? AND run_id=? AND idempotency_key=?`, command.AccountID, command.RunID,
		command.IdempotencyKey).Scan(&existingDigest)
	if err == nil {
		if existingDigest != digest {
			return ledger.RoutineRunRecord{}, fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.IdempotencyKey)
		}
		return getRoutineRunRecord(ctx, tx, command.AccountID, command.RunID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ledger.RoutineRunRecord{}, err
	}
	if err := command.Validate(); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	record, err := getRoutineRunRecord(ctx, tx, command.AccountID, command.RunID)
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	if routineRunTerminal(record.Run.State) {
		return ledger.RoutineRunRecord{}, ledger.ErrRoutineRunTerminal
	}
	if record.Run.State != command.FromState {
		return ledger.RoutineRunRecord{}, fmt.Errorf("Routine run state is %q, not %q", record.Run.State, command.FromState)
	}
	if command.FromState == conversation.RoutineRunWorking &&
		(record.AttemptID != command.AttemptID || record.LeaseID != command.LeaseID ||
			!record.LeaseExpiresAt.Equal(command.LeaseExpiresAt)) {
		return ledger.RoutineRunRecord{}, fmt.Errorf("Routine run activity lease does not match the exact working attempt")
	}

	attemptID, leaseID, leaseExpiresAt := record.AttemptID, record.LeaseID, record.LeaseExpiresAt
	if command.ToState == conversation.RoutineRunWorking {
		attemptID, leaseID, leaseExpiresAt = command.AttemptID, command.LeaseID, command.LeaseExpiresAt
	}
	failureCode, nextAction := command.FailureCode, command.NextAction
	if command.ToState == conversation.RoutineRunQueued {
		failureCode, nextAction = "", ""
	}
	var resultMessageID any
	var resultMessageIDString string
	var terminalAt any
	if routineRunTerminal(command.ToState) {
		terminalAt = nowOr(command.OccurredAt)
	}
	if command.ToState == conversation.RoutineRunSucceeded {
		result, err := tx.ExecContext(ctx, `INSERT INTO conversation_message(
conversation_id,turn_id,target_id,author_kind,author_id,body,created_at) VALUES(?,NULL,NULL,'agent',?,?,?)`,
			record.ResultConversationID, record.Run.AgentID, command.NormalizedResult, nowOr(command.OccurredAt))
		if err != nil {
			return ledger.RoutineRunRecord{}, err
		}
		messageID, err := result.LastInsertId()
		if err != nil {
			return ledger.RoutineRunRecord{}, err
		}
		resultMessageID = messageID
		resultMessageIDString = strconv.FormatInt(messageID, 10)
	}
	update, err := tx.ExecContext(ctx, `UPDATE routine_run SET state=?,current_attempt_id=?,current_lease_id=?,
lease_expires_at=?,normalized_result=?,result_message_id=?,failure_code=?,next_action=?,terminal_at=?,updated_at=?
WHERE account_id=? AND id=? AND state=?`, command.ToState, nullableString(attemptID), nullableString(leaseID),
		nullableTime(leaseExpiresAt), nullableString(command.NormalizedResult), resultMessageID,
		nullableString(failureCode), nullableString(nextAction), terminalAt, nowOr(command.OccurredAt),
		command.AccountID, command.RunID, command.FromState)
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	changed, err := update.RowsAffected()
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	if changed != 1 {
		return ledger.RoutineRunRecord{}, fmt.Errorf("Routine run state changed concurrently")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE routine_occurrence SET state=?,updated_at=?
WHERE account_id=? AND id=? AND state=?`, command.ToState, nowOr(command.OccurredAt), command.AccountID,
		record.Occurrence.ID, command.FromState); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO routine_run_activity(
account_id,run_id,idempotency_key,command_digest,state,attempt_id,lease_id,lease_expires_at,
activity,failure_code,next_action,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, command.AccountID,
		command.RunID, command.IdempotencyKey, digest, command.ToState, nullableString(command.AttemptID),
		nullableString(command.LeaseID), nullableTime(command.LeaseExpiresAt), command.Activity,
		nullableString(command.FailureCode), nullableString(command.NextAction), nowOr(command.OccurredAt)); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	if command.ToState == conversation.RoutineRunSucceeded {
		messageID, _ := strconv.ParseInt(resultMessageIDString, 10, 64)
		if _, err := tx.ExecContext(ctx, `INSERT INTO routine_result(account_id,run_id,conversation_id,message_id,created_at)
VALUES(?,?,?,?,?)`, command.AccountID, command.RunID, record.ResultConversationID, messageID,
			nowOr(command.OccurredAt)); err != nil {
			return ledger.RoutineRunRecord{}, err
		}
	}
	record, err = getRoutineRunRecord(ctx, tx, command.AccountID, command.RunID)
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	return record, nil
}

func (s *Store) GetRoutineRun(ctx context.Context, accountID, runID string) (ledger.RoutineRunRecord, error) {
	return getRoutineRunRecord(ctx, s.db, accountID, runID)
}

func (s *Store) ListRoutineRuns(ctx context.Context, accountID, routineID string) ([]ledger.RoutineRunRecord, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(routineID) == "" {
		return nil, fmt.Errorf("Routine run account and Routine ids are required")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM routine_run WHERE account_id=? AND routine_id=?
ORDER BY created_at DESC,id DESC`, accountID, routineID)
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
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := make([]ledger.RoutineRunRecord, 0, len(ids))
	for _, id := range ids {
		item, err := getRoutineRunRecord(ctx, s.db, accountID, id)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

type routineQueryer interface {
	agentQueryer
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func getRoutineRunRecord(ctx context.Context, queryer routineQueryer, accountID, runID string) (ledger.RoutineRunRecord, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(runID) == "" {
		return ledger.RoutineRunRecord{}, fmt.Errorf("Routine run account id and id are required")
	}
	var record ledger.RoutineRunRecord
	var createdAt, updatedAt string
	var attemptID, leaseID, leaseExpiresAt, normalizedResult, failureCode, nextAction sql.NullString
	var resultMessageID sql.NullInt64
	err := queryer.QueryRowContext(ctx, `SELECT id,occurrence_id,routine_id,routine_revision_id,agent_id,
behavior_revision_id,binding_revision_id,result_conversation_id,kind,state,current_attempt_id,current_lease_id,
lease_expires_at,normalized_result,result_message_id,failure_code,next_action,created_at,updated_at
FROM routine_run WHERE account_id=? AND id=?`, accountID, runID).Scan(&record.Run.ID, &record.Run.OccurrenceID,
		&record.Run.RoutineID, &record.Run.RoutineRevisionID, &record.Run.AgentID,
		&record.Run.BehaviorRevisionID, &record.Run.BindingRevisionID, &record.ResultConversationID,
		&record.Run.Kind, &record.Run.State, &attemptID, &leaseID, &leaseExpiresAt, &normalizedResult,
		&resultMessageID, &failureCode, &nextAction, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.RoutineRunRecord{}, fmt.Errorf("%w: Routine run %q", ledger.ErrNotFound, runID)
	}
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	record.Run.NormalizedResult = normalizedResult.String
	if resultMessageID.Valid {
		record.Run.ResultMessageID = strconv.FormatInt(resultMessageID.Int64, 10)
	}
	record.Run.CreatedAt = parseTime(createdAt)
	record.AttemptID, record.LeaseID = attemptID.String, leaseID.String
	record.LeaseExpiresAt = parseTime(leaseExpiresAt.String)
	record.FailureCode, record.NextAction = failureCode.String, nextAction.String

	var occurrenceCreated, occurrenceUpdated, scheduledFor string
	if err := queryer.QueryRowContext(ctx, `SELECT id,account_id,routine_id,routine_revision_id,kind,state,
scheduled_for,idempotency_key,approval_evidence_id,created_at,updated_at FROM routine_occurrence
WHERE account_id=? AND id=?`, accountID, record.Run.OccurrenceID).Scan(&record.Occurrence.ID,
		&record.Occurrence.AccountID, &record.Occurrence.RoutineID, &record.Occurrence.RoutineRevisionID,
		&record.Occurrence.Kind, &record.Occurrence.State, &scheduledFor, &record.Occurrence.IdempotencyKey,
		&record.Occurrence.ApprovalEvidenceID, &occurrenceCreated, &occurrenceUpdated); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	record.Occurrence.ScheduledFor = parseTime(scheduledFor)
	record.Occurrence.CreatedAt = parseTime(occurrenceCreated)
	record.Occurrence.UpdatedAt = parseTime(occurrenceUpdated)

	rows, err := queryer.QueryContext(ctx, `SELECT sequence,state,attempt_id,lease_id,lease_expires_at,
activity,failure_code,next_action,created_at FROM routine_run_activity
WHERE account_id=? AND run_id=? ORDER BY sequence`, accountID, runID)
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	record.Activities = []ledger.RoutineRunActivity{}
	for rows.Next() {
		var activity ledger.RoutineRunActivity
		var activityAttempt, activityLease, activityLeaseExpiry, activityFailure, activityNext sql.NullString
		var activityCreated string
		if err := rows.Scan(&activity.Sequence, &activity.State, &activityAttempt, &activityLease,
			&activityLeaseExpiry, &activity.Activity, &activityFailure, &activityNext, &activityCreated); err != nil {
			rows.Close()
			return ledger.RoutineRunRecord{}, err
		}
		activity.AttemptID, activity.LeaseID = activityAttempt.String, activityLease.String
		activity.LeaseExpiresAt = parseTime(activityLeaseExpiry.String)
		activity.FailureCode, activity.NextAction = activityFailure.String, activityNext.String
		activity.CreatedAt = parseTime(activityCreated)
		record.Activities = append(record.Activities, activity)
	}
	if err := rows.Close(); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	if err := rows.Err(); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	return record, nil
}

func routineRunTerminal(state conversation.RoutineRunState) bool {
	return state == conversation.RoutineRunSucceeded || state == conversation.RoutineRunFailed ||
		state == conversation.RoutineRunCanceled
}

func routineResultConversationExists(ctx context.Context, queryer agentQueryer, accountID, agentID, conversationID string) (bool, error) {
	var exists int
	err := queryer.QueryRowContext(ctx, `SELECT EXISTS(
  SELECT 1 FROM agent_conversation link
  JOIN stable_agent agent ON agent.id=link.agent_id
  JOIN conversation transcript ON transcript.id=link.conversation_id
  WHERE agent.account_id=? AND agent.id=? AND link.agent_id=?
    AND link.conversation_id=? AND transcript.state='open'
)`, accountID, agentID, agentID, conversationID).Scan(&exists)
	return exists == 1, err
}

func insertRoutineRevision(ctx context.Context, tx *sql.Tx, accountID string, revision conversation.RoutineRevision) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO routine_revision(
id,account_id,routine_id,agent_id,revision,behavior_revision_id,binding_revision_id,authority,trigger_kind,
schedule_expression,timezone,next_occurrence_at,input_source,freshness_seconds,expected_result,
result_conversation_id,approval_boundary,missing_input_behavior,retry_policy,catch_up_policy,lateness_policy,created_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, revision.ID, accountID, revision.RoutineID,
		revision.AgentID, revision.Revision, revision.BehaviorRevisionID, revision.BindingRevisionID,
		revision.Authority, revision.Trigger, revision.Schedule, revision.Timezone, nullableTime(revision.NextOccurrence),
		revision.InputSource, revision.FreshnessSeconds, revision.ExpectedResult, revision.ResultConversationID,
		revision.ApprovalBoundary, revision.MissingInputBehavior, revision.RetryPolicy, revision.CatchUpPolicy,
		revision.LatenessPolicy, nowOr(revision.CreatedAt))
	return err
}

func (s *Store) GetRoutine(ctx context.Context, accountID, routineID string) (ledger.RoutineRecord, error) {
	return getRoutineRecord(ctx, s.db, accountID, routineID)
}

func (s *Store) ListRoutines(ctx context.Context, accountID, agentID string) ([]ledger.RoutineRecord, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(agentID) == "" {
		return nil, fmt.Errorf("Routine account and Agent ids are required")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM routine WHERE account_id=? AND agent_id=?
ORDER BY created_at,id`, accountID, agentID)
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
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := make([]ledger.RoutineRecord, 0, len(ids))
	for _, id := range ids {
		item, err := getRoutineRecord(ctx, s.db, accountID, id)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func getRoutineRecord(ctx context.Context, queryer agentQueryer, accountID, routineID string) (ledger.RoutineRecord, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(routineID) == "" {
		return ledger.RoutineRecord{}, fmt.Errorf("Routine account id and id are required")
	}
	var record ledger.RoutineRecord
	var routineCreated string
	err := queryer.QueryRowContext(ctx, `SELECT id,account_id,agent_id,current_revision_id,state,pause_reason,created_at
FROM routine WHERE account_id=? AND id=?`, accountID, routineID).Scan(&record.Routine.ID,
		&record.Routine.AccountID, &record.Routine.AgentID, &record.Routine.CurrentRevisionID,
		&record.Routine.State, &record.PauseReason, &routineCreated)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.RoutineRecord{}, fmt.Errorf("%w: Routine %q", ledger.ErrNotFound, routineID)
	}
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	record.Routine.CreatedAt = parseTime(routineCreated)
	revision, err := scanRoutineRevision(queryer.QueryRowContext(ctx, `SELECT id,routine_id,revision,agent_id,
behavior_revision_id,binding_revision_id,authority,trigger_kind,schedule_expression,timezone,next_occurrence_at,
input_source,freshness_seconds,expected_result,result_conversation_id,approval_boundary,missing_input_behavior,
retry_policy,catch_up_policy,lateness_policy,created_at FROM routine_revision
WHERE account_id=? AND id=? AND routine_id=?`, accountID, record.Routine.CurrentRevisionID, record.Routine.ID))
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	record.CurrentRevision = revision
	receipt, err := scanRoutineImportReceipt(queryer.QueryRowContext(ctx, `SELECT id,account_id,
source_routine_projection_id,routine_id,routine_revision_id,source_disabled_at,
exact_last_source_occurrence_at,exact_next_source_occurrence_at,fencing_receipt_ciphertext,
fencing_receipt_key_id,fencing_receipt_nonce,fencing_receipt_digest,imported_at
FROM routine_import_receipt WHERE account_id=? AND routine_id=?`, accountID, record.Routine.ID))
	if err == nil {
		record.ImportReceipt = &receipt
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ledger.RoutineRecord{}, err
	}
	return record, nil
}

func scanRoutineRevision(row scanner) (conversation.RoutineRevision, error) {
	var revision conversation.RoutineRevision
	var nextOccurrence sql.NullString
	var createdAt string
	err := row.Scan(&revision.ID, &revision.RoutineID, &revision.Revision, &revision.AgentID,
		&revision.BehaviorRevisionID, &revision.BindingRevisionID, &revision.Authority, &revision.Trigger,
		&revision.Schedule, &revision.Timezone, &nextOccurrence, &revision.InputSource,
		&revision.FreshnessSeconds, &revision.ExpectedResult, &revision.ResultConversationID,
		&revision.ApprovalBoundary, &revision.MissingInputBehavior, &revision.RetryPolicy,
		&revision.CatchUpPolicy, &revision.LatenessPolicy, &createdAt)
	if err != nil {
		return conversation.RoutineRevision{}, err
	}
	revision.NextOccurrence = parseTime(nextOccurrence.String)
	revision.CreatedAt = parseTime(createdAt)
	return revision, nil
}

func scanRoutineImportReceipt(row scanner) (ledger.RoutineImportReceipt, error) {
	var receipt ledger.RoutineImportReceipt
	var disabledAt, importedAt string
	var lastOccurrence, nextOccurrence sql.NullString
	if err := row.Scan(&receipt.ID, &receipt.AccountID, &receipt.SourceRoutineProjectionID,
		&receipt.RoutineID, &receipt.RoutineRevisionID, &disabledAt, &lastOccurrence, &nextOccurrence,
		&receipt.FencingReceiptCiphertext, &receipt.FencingReceiptKeyID, &receipt.FencingReceiptNonce,
		&receipt.FencingReceiptDigest, &importedAt); err != nil {
		return ledger.RoutineImportReceipt{}, err
	}
	receipt.SourceDisabledAt = parseTime(disabledAt)
	receipt.ExactLastSourceOccurrenceAt = parseTime(lastOccurrence.String)
	receipt.ExactNextSourceOccurrenceAt = parseTime(nextOccurrence.String)
	receipt.ImportedAt = parseTime(importedAt)
	return receipt, nil
}
