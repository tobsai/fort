package store_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
	_ "modernc.org/sqlite"
)

func TestAgentSourceInventoryLatestPreservesSuccessAcrossFailureAndRecovers(t *testing.T) {
	repository, path := openInventoryRepository(t)
	ctx := context.Background()
	first := inventorySnapshot("account:one", "source:hermes", "bot:research", time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC))
	if err := repository.AppendAgentSourceInventorySuccess(ctx, first); err != nil {
		t.Fatalf("append success: %v", err)
	}
	if err := repository.Close(); err != nil {
		t.Fatalf("close after success: %v", err)
	}
	repository, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen after success: %v", err)
	}
	t.Cleanup(func() { _ = repository.Close() })

	got, err := repository.LatestAgentSourceInventory(ctx, "account:one", "source:hermes")
	if err != nil {
		t.Fatalf("latest success: %v", err)
	}
	if !got.HasSnapshot || got.Freshness != runtime.AgentSourceInventoryCurrent || got.LastAttemptAt != first.ObservedAt || got.Snapshot.Agents[0].SourceAgent.OpaqueSourceAgentID != "bot:research" {
		t.Fatalf("latest success = %+v", got)
	}

	failureAt := first.ObservedAt.Add(time.Minute)
	if err := repository.AppendAgentSourceInventoryFailure(ctx, runtime.AgentSourceInventoryFailure{
		AccountID: "account:one", ExecutionSourceID: "source:hermes", ObservedAt: failureAt,
		Reason: runtime.AgentSourceInventoryUnavailable,
	}); err != nil {
		t.Fatalf("append failure: %v", err)
	}
	got, err = repository.LatestAgentSourceInventory(ctx, "account:one", "source:hermes")
	if err != nil {
		t.Fatalf("latest stale: %v", err)
	}
	if !got.HasSnapshot || got.Freshness != runtime.AgentSourceInventoryStale || got.Snapshot.ObservedAt != first.ObservedAt || got.SourceLastSeenAt != first.ExecutionSource.LastSeenAt || got.LastAttemptAt != failureAt || got.FailureReason != runtime.AgentSourceInventoryUnavailable || got.FailureObservedAt != failureAt {
		t.Fatalf("latest stale = %+v", got)
	}

	recovered := inventorySnapshot("account:one", "source:hermes", "", first.ObservedAt.Add(2*time.Minute))
	if err := repository.AppendAgentSourceInventorySuccess(ctx, recovered); err != nil {
		t.Fatalf("append empty recovery: %v", err)
	}
	got, err = repository.LatestAgentSourceInventory(ctx, "account:one", "source:hermes")
	if err != nil {
		t.Fatalf("latest recovery: %v", err)
	}
	if !got.HasSnapshot || got.Freshness != runtime.AgentSourceInventoryCurrent || len(got.Snapshot.Agents) != 0 || got.LastAttemptAt != recovered.ObservedAt || got.FailureReason != "" || !got.FailureObservedAt.IsZero() {
		t.Fatalf("latest recovery = %+v", got)
	}
}

func TestAgentSourceInventoryLatestUsesAppendSequenceAndScopesAccountAndSource(t *testing.T) {
	repository, _ := openInventoryRepository(t)
	ctx := context.Background()
	newerClock := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	if err := repository.AppendAgentSourceInventorySuccess(ctx, inventorySnapshot("account:one", "source:hermes", "bot:one", newerClock)); err != nil {
		t.Fatal(err)
	}
	// The later append wins even if its observer clock is earlier.
	if err := repository.AppendAgentSourceInventoryFailure(ctx, runtime.AgentSourceInventoryFailure{
		AccountID: "account:one", ExecutionSourceID: "source:hermes", ObservedAt: newerClock.Add(-time.Hour),
		Reason: runtime.AgentSourceInventoryUnavailable,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.AppendAgentSourceInventorySuccess(ctx, inventorySnapshot("account:two", "source:hermes", "bot:two", newerClock)); err != nil {
		t.Fatal(err)
	}
	if err := repository.AppendAgentSourceInventorySuccess(ctx, inventorySnapshot("account:one", "source:other", "bot:other", newerClock)); err != nil {
		t.Fatal(err)
	}

	got, err := repository.LatestAgentSourceInventory(ctx, "account:one", "source:hermes")
	if err != nil {
		t.Fatal(err)
	}
	if got.Freshness != runtime.AgentSourceInventoryStale || got.LastAttemptAt != newerClock.Add(-time.Hour) || got.Snapshot.Agents[0].SourceAgent.OpaqueSourceAgentID != "bot:one" {
		t.Fatalf("scoped latest = %+v", got)
	}
	absent, err := repository.LatestAgentSourceInventory(ctx, "account:missing", "source:hermes")
	if err != nil || absent.HasSnapshot || absent.Freshness != "" || !absent.LastAttemptAt.IsZero() {
		t.Fatalf("absent = %+v, %v", absent, err)
	}
}

func TestAgentSourceInventoryLatestAcceptsEquivalentNonUTCObservationInstant(t *testing.T) {
	repository, _ := openInventoryRepository(t)
	observedAt := time.Date(2026, 8, 22, 12, 0, 0, 0, time.FixedZone("CDT", -5*60*60))
	snapshot := inventorySnapshot("account:one", "source:hermes", "bot:one", observedAt)
	if err := repository.AppendAgentSourceInventorySuccess(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}

	latest, err := repository.LatestAgentSourceInventory(context.Background(), "account:one", "source:hermes")
	if err != nil {
		t.Fatal(err)
	}
	if !latest.HasSnapshot || !latest.Snapshot.ObservedAt.Equal(observedAt) || !latest.LastAttemptAt.Equal(observedAt) {
		t.Fatalf("non-UTC observation instant = %+v", latest)
	}
}

func TestAgentSourceInventoryReopenPreservesRFC8785UnicodeSeparators(t *testing.T) {
	repository, path := openInventoryRepository(t)
	snapshot := inventorySnapshot(
		"account:one",
		"source:hermes",
		"bot:one",
		time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC),
	)
	wantDisplayName := "Literal \\u2028 and \\u003c beside actual \u2028\u2029"
	snapshot.Agents[0].SourceAgent.DisplayName = wantDisplayName
	if err := repository.AppendAgentSourceInventorySuccess(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	latest, err := reopened.LatestAgentSourceInventory(
		context.Background(),
		"account:one",
		"source:hermes",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !latest.HasSnapshot || latest.Snapshot.Agents[0].SourceAgent.DisplayName != wantDisplayName {
		t.Fatalf("reopened display name = %q, want %q", latest.Snapshot.Agents[0].SourceAgent.DisplayName, wantDisplayName)
	}
}

func TestAgentSourceInventoryRejectsInvalidWritesAndTamperedEvidence(t *testing.T) {
	repository, path := openInventoryRepository(t)
	ctx := context.Background()
	invalid := inventorySnapshot("account:one", "source:hermes", "bot:one", time.Now().UTC())
	invalid.Agents = nil
	if err := repository.AppendAgentSourceInventorySuccess(ctx, invalid); err == nil {
		t.Fatal("invalid snapshot was appended")
	}
	if err := repository.AppendAgentSourceInventoryFailure(ctx, runtime.AgentSourceInventoryFailure{
		AccountID: "account:one", ExecutionSourceID: "source:hermes", ObservedAt: time.Now().UTC(), Reason: "transport_error",
	}); err == nil {
		t.Fatal("open-ended failure reason was appended")
	}

	valid := inventorySnapshot("account:one", "source:hermes", "bot:one", time.Now().UTC())
	valid.Agents[0].SourceAgent.DisplayName = "Bot <&>"
	if err := repository.AppendAgentSourceInventorySuccess(ctx, valid); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE agent_source_inventory_observation SET snapshot_sha256=? WHERE sequence=1`, strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "agent_source_inventory_observation_immutable") {
		t.Fatalf("update error = %v", err)
	}
	if _, err := db.Exec(`DELETE FROM agent_source_inventory_observation WHERE sequence=1`); err == nil || !strings.Contains(err.Error(), "agent_source_inventory_observation_immutable") {
		t.Fatalf("delete error = %v", err)
	}
	var snapshotJSON, snapshotDigest string
	if err := db.QueryRow(`SELECT snapshot_json,snapshot_sha256 FROM agent_source_inventory_observation WHERE sequence=1`).Scan(&snapshotJSON, &snapshotDigest); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(snapshotJSON))
	if snapshotDigest != hex.EncodeToString(digest[:]) {
		t.Fatalf("snapshot digest = %q, want digest of canonical JSON", snapshotDigest)
	}
	if !strings.HasPrefix(snapshotJSON, `{"agents":`) || strings.Contains(snapshotJSON, `\u003c`) || strings.Contains(snapshotJSON, `\u0026`) {
		t.Fatalf("snapshot JSON is not RFC 8785 canonical JSON: %s", snapshotJSON)
	}
	if _, err := db.Exec(`DROP TRIGGER agent_source_inventory_observation_update_immutable`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE agent_source_inventory_observation SET snapshot_sha256=? WHERE sequence=1`, strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.LatestAgentSourceInventory(ctx, "account:one", "source:hermes"); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("tampered read error = %v", err)
	}
}

func TestAgentSourceInventoryLatestRejectsCorruptObservationRows(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*testing.T, *sql.DB, runtime.AgentSourceInventorySnapshot)
	}{
		{name: "unknown observation kind", mutate: func(t *testing.T, db *sql.DB, _ runtime.AgentSourceInventorySnapshot) {
			_, err := db.Exec(`UPDATE agent_source_inventory_observation SET observation_kind='unknown' WHERE sequence=1`)
			if err != nil {
				t.Fatal(err)
			}
		}},
		{name: "failure carries snapshot payload", mutate: func(t *testing.T, db *sql.DB, _ runtime.AgentSourceInventorySnapshot) {
			_, err := db.Exec(`UPDATE agent_source_inventory_observation SET observation_kind='failure',failure_reason='source_inventory_unavailable' WHERE sequence=1`)
			if err != nil {
				t.Fatal(err)
			}
		}},
		{name: "failure reason is open ended", mutate: func(t *testing.T, db *sql.DB, _ runtime.AgentSourceInventorySnapshot) {
			_, err := db.Exec(`UPDATE agent_source_inventory_observation SET observation_kind='failure',snapshot_json=NULL,snapshot_sha256=NULL,failure_reason='private_error' WHERE sequence=1`)
			if err != nil {
				t.Fatal(err)
			}
		}},
		{name: "success observation time differs from snapshot", mutate: func(t *testing.T, db *sql.DB, snapshot runtime.AgentSourceInventorySnapshot) {
			_, err := db.Exec(`UPDATE agent_source_inventory_observation SET observed_at=? WHERE sequence=1`, nowOrTest(snapshot.ObservedAt.Add(time.Minute)))
			if err != nil {
				t.Fatal(err)
			}
		}},
		{name: "snapshot bytes are not canonical", mutate: func(t *testing.T, db *sql.DB, snapshot runtime.AgentSourceInventorySnapshot) {
			noncanonical, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(noncanonical)
			_, err = db.Exec(`UPDATE agent_source_inventory_observation SET snapshot_json=?,snapshot_sha256=? WHERE sequence=1`,
				string(noncanonical), hex.EncodeToString(digest[:]))
			if err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository, path := openInventoryRepository(t)
			snapshot := inventorySnapshot("account:one", "source:hermes", "bot:one", time.Date(2026, 8, 22, 17, 0, 0, 0, time.UTC))
			if err := repository.AppendAgentSourceInventorySuccess(context.Background(), snapshot); err != nil {
				t.Fatal(err)
			}
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(`PRAGMA ignore_check_constraints=ON`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`DROP TRIGGER agent_source_inventory_observation_update_immutable`); err != nil {
				t.Fatal(err)
			}
			test.mutate(t, db, snapshot)

			if _, err := repository.LatestAgentSourceInventory(context.Background(), "account:one", "source:hermes"); err == nil {
				t.Fatal("corrupt observation row was accepted")
			}
		})
	}
}

func nowOrTest(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func openInventoryRepository(t *testing.T) (*store.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fort.db")
	repository, err := store.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	return repository, path
}

func inventorySnapshot(accountID, sourceID, opaqueAgentID string, observedAt time.Time) runtime.AgentSourceInventorySnapshot {
	agents := make([]runtime.SourceAgentInventory, 0, 1)
	if opaqueAgentID != "" {
		agents = append(agents, runtime.SourceAgentInventory{
			SourceAgent:  conversation.SourceAgent{ID: sourceID + ":" + opaqueAgentID, ExecutionSourceID: sourceID, OpaqueSourceAgentID: opaqueAgentID, DisplayName: opaqueAgentID, LastSeenAt: observedAt},
			Capabilities: []string{},
			Readiness:    runtime.SourceAgentReadiness{Ready: false, ContractID: "source-agent.inventory.hermes-bot.v1", ContractRevision: strings.Repeat("a", 64), Evidence: []string{"profile_roster"}},
		})
	}
	return runtime.AgentSourceInventorySnapshot{
		ExecutionSource: conversation.ExecutionSource{
			ID: sourceID, AccountID: accountID, Framework: "hermes", InstanceID: "instance:one", GatewayID: "gateway:one", DisplayName: "Hermes",
			ResourceSharing: conversation.ResourceSharingDisclosure{
				ProviderCredentials: conversation.ResourceProfileScoped, Filesystem: conversation.ResourceProfileScoped,
				BrowserSessions: conversation.ResourceProfileScoped, FrameworkSessions: conversation.ResourceProfileScoped,
				SourceMemory: conversation.ResourceProfileScoped, ToolConfiguration: conversation.ResourceProfileScoped,
			}, LastSeenAt: observedAt,
		},
		Agents: agents, ObservedAt: observedAt,
	}
}
