package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tobsai/fort/core/canonicaljson"
	coreruntime "github.com/tobsai/fort/core/runtime"
)

const agentSourceInventorySchema = `
CREATE TABLE IF NOT EXISTS agent_source_inventory_observation (
  sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  account_id TEXT NOT NULL CHECK(length(trim(account_id))>0 AND account_id=trim(account_id)),
  execution_source_id TEXT NOT NULL CHECK(length(trim(execution_source_id))>0 AND execution_source_id=trim(execution_source_id)),
  observation_kind TEXT NOT NULL CHECK(observation_kind IN ('success','failure')),
  observed_at TEXT NOT NULL,
  snapshot_json TEXT,
  snapshot_sha256 TEXT,
  failure_reason TEXT,
  CHECK(
    (observation_kind='success' AND snapshot_json IS NOT NULL AND json_valid(snapshot_json)
      AND length(snapshot_sha256)=64 AND snapshot_sha256 NOT GLOB '*[^0-9a-f]*' AND failure_reason IS NULL)
    OR
    (observation_kind='failure' AND snapshot_json IS NULL AND snapshot_sha256 IS NULL
      AND failure_reason='source_inventory_unavailable')
  )
);
CREATE INDEX IF NOT EXISTS idx_agent_source_inventory_latest
  ON agent_source_inventory_observation(account_id,execution_source_id,sequence DESC);
CREATE TRIGGER IF NOT EXISTS agent_source_inventory_observation_update_immutable
BEFORE UPDATE ON agent_source_inventory_observation
BEGIN
  SELECT RAISE(ABORT, 'agent_source_inventory_observation_immutable');
END;
CREATE TRIGGER IF NOT EXISTS agent_source_inventory_observation_delete_immutable
BEFORE DELETE ON agent_source_inventory_observation
BEGIN
  SELECT RAISE(ABORT, 'agent_source_inventory_observation_immutable');
END;
`

func (s *Store) migrateAgentSourceInventory() error {
	_, err := s.db.Exec(agentSourceInventorySchema)
	return err
}

func (s *Store) AppendAgentSourceInventorySuccess(ctx context.Context, snapshot coreruntime.AgentSourceInventorySnapshot) error {
	if err := snapshot.Validate(); err != nil {
		return fmt.Errorf("store: validate Agent Source inventory success: %w", err)
	}
	encoded, err := canonicaljson.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("store: encode Agent Source inventory success: %w", err)
	}
	digest := sha256.Sum256(encoded)
	_, err = s.db.ExecContext(ctx, `INSERT INTO agent_source_inventory_observation(
account_id,execution_source_id,observation_kind,observed_at,snapshot_json,snapshot_sha256,failure_reason)
VALUES(?,?, 'success', ?,?,?,NULL)`, snapshot.ExecutionSource.AccountID, snapshot.ExecutionSource.ID,
		nowOr(snapshot.ObservedAt), string(encoded), hex.EncodeToString(digest[:]))
	if err != nil {
		return fmt.Errorf("store: append Agent Source inventory success: %w", err)
	}
	return nil
}

func (s *Store) AppendAgentSourceInventoryFailure(ctx context.Context, failure coreruntime.AgentSourceInventoryFailure) error {
	if err := failure.Validate(); err != nil {
		return fmt.Errorf("store: validate Agent Source inventory failure: %w", err)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_source_inventory_observation(
account_id,execution_source_id,observation_kind,observed_at,snapshot_json,snapshot_sha256,failure_reason)
VALUES(?,?, 'failure', ?,NULL,NULL,?)`, failure.AccountID, failure.ExecutionSourceID,
		nowOr(failure.ObservedAt), string(failure.Reason))
	if err != nil {
		return fmt.Errorf("store: append Agent Source inventory failure: %w", err)
	}
	return nil
}

func (s *Store) LatestAgentSourceInventory(ctx context.Context, accountID, executionSourceID string) (coreruntime.LatestAgentSourceInventory, error) {
	if strings.TrimSpace(accountID) == "" || accountID != strings.TrimSpace(accountID) ||
		strings.TrimSpace(executionSourceID) == "" || executionSourceID != strings.TrimSpace(executionSourceID) {
		return coreruntime.LatestAgentSourceInventory{}, fmt.Errorf("store: Agent Source inventory account and Execution Source ids are required and normalized")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT observation_kind,observed_at,snapshot_json,snapshot_sha256,failure_reason
FROM agent_source_inventory_observation
WHERE account_id=? AND execution_source_id=?
ORDER BY sequence DESC`, accountID, executionSourceID)
	if err != nil {
		return coreruntime.LatestAgentSourceInventory{}, fmt.Errorf("store: query latest Agent Source inventory: %w", err)
	}
	defer rows.Close()

	var latest coreruntime.LatestAgentSourceInventory
	first := true
	for rows.Next() {
		var kind, observedAt string
		var snapshotJSON, snapshotDigest, failureReason *string
		if err := rows.Scan(&kind, &observedAt, &snapshotJSON, &snapshotDigest, &failureReason); err != nil {
			return coreruntime.LatestAgentSourceInventory{}, fmt.Errorf("store: scan Agent Source inventory: %w", err)
		}
		observed := parseTime(observedAt)
		if observed.IsZero() {
			return coreruntime.LatestAgentSourceInventory{}, fmt.Errorf("store: Agent Source inventory observation time is invalid")
		}
		switch kind {
		case "success":
			if snapshotJSON == nil || snapshotDigest == nil || failureReason != nil {
				return coreruntime.LatestAgentSourceInventory{}, fmt.Errorf("store: Agent Source inventory success evidence is invalid")
			}
		case "failure":
			if snapshotJSON != nil || snapshotDigest != nil || failureReason == nil ||
				coreruntime.AgentSourceInventoryFailureReason(*failureReason) != coreruntime.AgentSourceInventoryUnavailable {
				return coreruntime.LatestAgentSourceInventory{}, fmt.Errorf("store: Agent Source inventory failure evidence is invalid")
			}
		default:
			return coreruntime.LatestAgentSourceInventory{}, fmt.Errorf("store: Agent Source inventory observation kind is invalid")
		}
		if first {
			latest.LastAttemptAt = observed
			first = false
			if kind == "failure" {
				latest.Freshness = coreruntime.AgentSourceInventoryStale
				latest.FailureObservedAt = observed
				latest.FailureReason = coreruntime.AgentSourceInventoryFailureReason(valueOrEmpty(failureReason))
			} else {
				latest.Freshness = coreruntime.AgentSourceInventoryCurrent
			}
		}
		if kind != "success" {
			continue
		}
		digest := sha256.Sum256([]byte(*snapshotJSON))
		if hex.EncodeToString(digest[:]) != *snapshotDigest {
			return coreruntime.LatestAgentSourceInventory{}, fmt.Errorf("store: Agent Source inventory snapshot digest mismatch")
		}
		var snapshot coreruntime.AgentSourceInventorySnapshot
		if err := json.Unmarshal([]byte(*snapshotJSON), &snapshot); err != nil {
			return coreruntime.LatestAgentSourceInventory{}, fmt.Errorf("store: decode Agent Source inventory snapshot: %w", err)
		}
		canonical, err := canonicaljson.Marshal(snapshot)
		if err != nil || !bytes.Equal(canonical, []byte(*snapshotJSON)) {
			return coreruntime.LatestAgentSourceInventory{}, fmt.Errorf("store: Agent Source inventory snapshot is not canonical")
		}
		if err := snapshot.Validate(); err != nil {
			return coreruntime.LatestAgentSourceInventory{}, fmt.Errorf("store: validate persisted Agent Source inventory snapshot: %w", err)
		}
		if !snapshot.ObservedAt.Equal(observed) {
			return coreruntime.LatestAgentSourceInventory{}, fmt.Errorf("store: Agent Source inventory observation time mismatch")
		}
		if snapshot.ExecutionSource.AccountID != accountID || snapshot.ExecutionSource.ID != executionSourceID {
			return coreruntime.LatestAgentSourceInventory{}, fmt.Errorf("store: Agent Source inventory snapshot scope mismatch")
		}
		latest.Snapshot = snapshot
		latest.HasSnapshot = true
		latest.SourceLastSeenAt = latest.Snapshot.ExecutionSource.LastSeenAt
		break
	}
	if err := rows.Err(); err != nil {
		return coreruntime.LatestAgentSourceInventory{}, fmt.Errorf("store: iterate Agent Source inventory: %w", err)
	}
	return latest, nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

var _ coreruntime.AgentSourceInventoryRepository = (*Store)(nil)
