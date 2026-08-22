package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tobsai/fort/cloud/controlapi"
)

func TestCreateWorkerArtifactPinsActiveLeaseAndOutputManifest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 21, 0, 0, 0, time.UTC)
	command := testWorkerArtifactCreateCommand(now)
	tx := &fakeTransaction{}
	tx.queryRowHook = func(query string, arguments []any) row {
		if strings.Contains(query, "from fort_private.worker_lease") {
			assertArtifactLeaseQuery(t, query, arguments, command.TargetID, command.ExecutionAttemptID, command.LeaseID, command.WorkerID, command.MachineID, command.FenceToken)
			return fakeRow{values: []any{now.Add(time.Minute), "leased", "claimed"}}
		}
		return fakeRow{err: errors.New("unexpected artifact create query")}
	}
	tx.execHook = func(query string, arguments []any) (int64, error) {
		switch {
		case strings.Contains(query, "set_config('fort.account_id'"):
			return 1, nil
		case strings.Contains(query, "idempotency_record"):
			return 1, nil
		case strings.Contains(query, "insert into fort_private.artifact"):
			if !strings.Contains(query, "'output', 'uploading'") || len(arguments) != 9 ||
				arguments[0] != testAccountID || arguments[1] != command.ArtifactID ||
				arguments[2] != command.ExecutionAttemptID || arguments[3] != command.ExpectedChunkCount ||
				arguments[4] != command.ExpectedPlaintextLength || arguments[5] != int64(43) ||
				arguments[6] != command.LogicalDigest || arguments[7] != "test-key" || arguments[8] != now {
				return 0, errors.New("artifact manifest arguments are not exact")
			}
			return 1, nil
		default:
			return 0, errors.New("unexpected artifact create statement")
		}
	}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatal(err)
	}

	artifact, err := store.CreateWorkerArtifact(context.Background(), command)
	if err != nil {
		t.Fatalf("CreateWorkerArtifact: %v", err)
	}
	if artifact.ArtifactID != command.ArtifactID || artifact.Kind != "output" || artifact.State != "uploading" || !artifact.Created {
		t.Fatalf("artifact = %#v", artifact)
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("transaction commits/rollbacks = %d/%d", tx.commits, tx.rollbacks)
	}
}

func TestAppendWorkerArtifactChunkAllowsOutOfOrderAndExactReplayButRejectsChangedDuplicate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 21, 1, 0, 0, time.UTC)
	command := testWorkerArtifactChunkCommand(now)
	command.ChunkIndex = 1 // Index zero need not exist yet; resume supports out-of-order appends.
	manifestValues := []any{"attempt:1", "output", "uploading", 2, int64(11), int64(43), strings.Repeat("d", 64), "test-key", now.Add(-time.Minute), sql.NullTime{}}

	makeTx := func(existing row) *fakeTransaction {
		tx := &fakeTransaction{}
		tx.queryRowHook = func(query string, _ []any) row {
			switch {
			case strings.Contains(query, "from fort_private.worker_lease"):
				return fakeRow{values: []any{now.Add(time.Minute), "working", "working"}}
			case strings.Contains(query, "from fort_private.artifact\n"):
				return fakeRow{values: manifestValues}
			case strings.Contains(query, "from fort_private.artifact_chunk"):
				return existing
			default:
				return fakeRow{err: errors.New("unexpected artifact chunk query")}
			}
		}
		tx.execHook = func(query string, _ []any) (int64, error) {
			switch {
			case strings.Contains(query, "set_config"):
				return 1, nil
			case strings.Contains(query, "idempotency_record"):
				return 1, nil
			case strings.Contains(query, "insert into fort_private.artifact_chunk"):
				return 1, nil
			default:
				return 0, errors.New("unexpected artifact chunk statement")
			}
		}
		return tx
	}

	first := makeTx(fakeRow{err: pgx.ErrNoRows})
	exact := makeTx(fakeRow{values: []any{
		[]byte("server ciphertext"), len(command.Plaintext) + 16, len(command.Plaintext),
		"test-key", []byte("0123456789ab"), command.PlaintextDigest, now,
	}})
	changed := makeTx(fakeRow{values: []any{
		[]byte("changed ciphertext"), len("changed ciphertext"), len(command.Plaintext),
		"test-key", []byte("0123456789ab"), strings.Repeat("f", 64), now,
	}})
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{first, exact, changed}}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatal(err)
	}

	created, err := store.AppendWorkerArtifactChunk(context.Background(), command)
	if err != nil || !created.Created || created.ChunkIndex != 1 {
		t.Fatalf("first append = %#v, %v", created, err)
	}
	command.IdempotencyKey = "artifact:chunk:replay"
	replayed, err := store.AppendWorkerArtifactChunk(context.Background(), command)
	if err != nil || replayed.Created || replayed.CreatedAt != now {
		t.Fatalf("exact replay = %#v, %v", replayed, err)
	}
	command.IdempotencyKey = "artifact:chunk:changed"
	if _, err := store.AppendWorkerArtifactChunk(context.Background(), command); !errors.Is(err, controlapi.ErrWorkerIdempotencyConflict) {
		t.Fatalf("changed duplicate error = %v, want idempotency conflict", err)
	}
	if changed.commits != 0 || changed.rollbacks != 1 {
		t.Fatalf("changed duplicate transaction = commits %d rollbacks %d", changed.commits, changed.rollbacks)
	}
}

func TestGetWorkerArtifactStatusExposesMissingChunkIndexesWithoutCiphertext(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 21, 2, 0, 0, time.UTC)
	command := controlapi.WorkerArtifactStatusCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		IdempotencyKey: "artifact:status:1", ArtifactID: "artifact:output:1", ObservedAt: now,
	}
	tx := &fakeTransaction{queryRows: &fakeRows{values: [][]any{
		{0, 24, 5, "key:1", strings.Repeat("a", 64), now.Add(-time.Second)},
		{2, 25, 6, "key:1", strings.Repeat("b", 64), now},
	}}}
	tx.queryRowHook = func(query string, _ []any) row {
		if strings.Contains(query, "from fort_private.worker_lease") {
			return fakeRow{values: []any{now.Add(time.Minute), "working", "working"}}
		}
		if strings.Contains(query, "from fort_private.artifact\n") {
			return fakeRow{values: []any{"attempt:1", "output", "uploading", 3, int64(16), int64(73), strings.Repeat("d", 64), "key:1", now.Add(-time.Minute), sql.NullTime{}}}
		}
		return fakeRow{err: errors.New("unexpected status query")}
	}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatal(err)
	}

	artifact, err := store.GetWorkerArtifactStatus(context.Background(), command)
	if err != nil {
		t.Fatalf("GetWorkerArtifactStatus: %v", err)
	}
	if len(artifact.Chunks) != 2 || artifact.Chunks[0].ChunkIndex != 0 || artifact.Chunks[1].ChunkIndex != 2 {
		t.Fatalf("status chunks = %#v, want indexes 0 and 2", artifact.Chunks)
	}
}

func TestWorkerArtifactStatusRejectsExpiredExactLeaseBeforeReadingManifest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 21, 2, 30, 0, time.UTC)
	command := controlapi.WorkerArtifactStatusCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		IdempotencyKey: "artifact:status:expired", ArtifactID: "artifact:output:1", ObservedAt: now,
	}
	tx := &fakeTransaction{queryRowHook: func(query string, _ []any) row {
		if !strings.Contains(query, "from fort_private.worker_lease") {
			return fakeRow{err: errors.New("expired lease reached artifact manifest")}
		}
		return fakeRow{values: []any{now, "working", "working"}}
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.GetWorkerArtifactStatus(context.Background(), command); !errors.Is(err, controlapi.ErrWorkerStaleLease) {
		t.Fatalf("expired artifact status error = %v, want stale lease", err)
	}
	if tx.commits != 0 || tx.rollbacks != 1 || len(tx.queries) != 1 {
		t.Fatalf("expired lease transaction = commits %d rollbacks %d queries %d", tx.commits, tx.rollbacks, len(tx.queries))
	}
}

func TestFinalizeWorkerArtifactReliesOnDatabaseCompletenessTrigger(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 21, 3, 0, 0, time.UTC)
	command := controlapi.WorkerArtifactFinalizeCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		IdempotencyKey: "artifact:finalize:1", ArtifactID: "artifact:output:1", FinalizedAt: now,
	}
	logicalDigest := sha256.Sum256([]byte("hello world"))
	cipher := secureCollaborationBodyCipher{ring: collaborationTestKeyRing()}
	first, err := cipher.seal(workerArtifactBodyScope(testAccountID, command.ArtifactID, 0), "hello ")
	if err != nil {
		t.Fatal(err)
	}
	second, err := cipher.seal(workerArtifactBodyScope(testAccountID, command.ArtifactID, 1), "world")
	if err != nil {
		t.Fatal(err)
	}
	tx := &fakeTransaction{queryRows: &fakeRows{values: [][]any{
		{0, first.Ciphertext, len(first.Ciphertext), first.PlaintextBytes, first.KeyID, first.Nonce, first.Digest},
		{1, second.Ciphertext, len(second.Ciphertext), second.PlaintextBytes, second.KeyID, second.Nonce, second.Digest},
	}}}
	tx.queryRowHook = func(query string, _ []any) row {
		if strings.Contains(query, "from fort_private.worker_lease") {
			return fakeRow{values: []any{now.Add(time.Minute), "working", "working"}}
		}
		if strings.Contains(query, "from fort_private.artifact\n") {
			return fakeRow{values: []any{"attempt:1", "output", "uploading", 2, int64(11), int64(43), hex.EncodeToString(logicalDigest[:]), "test-key", now.Add(-time.Minute), sql.NullTime{}}}
		}
		return fakeRow{err: errors.New("unexpected finalize query")}
	}
	tx.execHook = func(query string, _ []any) (int64, error) {
		switch {
		case strings.Contains(query, "set_config"), strings.Contains(query, "idempotency_record"):
			return 1, nil
		case strings.Contains(query, "update fort_private.artifact"):
			if !strings.Contains(query, "state = 'finalized'") {
				return 0, errors.New("finalize did not use artifact state transition")
			}
			return 0, &pgconn.PgError{Code: "23514", Message: "artifact_incomplete"}
		default:
			return 0, errors.New("unexpected finalize statement")
		}
	}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.FinalizeWorkerArtifact(context.Background(), command); !errors.Is(err, controlapi.ErrWorkerArtifactIncomplete) {
		t.Fatalf("FinalizeWorkerArtifact error = %v, want artifact incomplete", err)
	}
	if tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("incomplete finalize transaction = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func TestFinalizeWorkerArtifactRejectsOrderedPlaintextDigestMismatchBeforeStateTransition(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 21, 4, 0, 0, time.UTC)
	command := controlapi.WorkerArtifactFinalizeCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		IdempotencyKey: "artifact:finalize:digest-mismatch", ArtifactID: "artifact:output:1", FinalizedAt: now,
	}
	expected := sha256.Sum256([]byte("hello world"))
	cipher := secureCollaborationBodyCipher{ring: collaborationTestKeyRing()}
	first, err := cipher.seal(workerArtifactBodyScope(testAccountID, command.ArtifactID, 0), "world")
	if err != nil {
		t.Fatal(err)
	}
	second, err := cipher.seal(workerArtifactBodyScope(testAccountID, command.ArtifactID, 1), "hello ")
	if err != nil {
		t.Fatal(err)
	}
	tx := &fakeTransaction{queryRows: &fakeRows{values: [][]any{
		{0, first.Ciphertext, len(first.Ciphertext), first.PlaintextBytes, first.KeyID, first.Nonce, first.Digest},
		{1, second.Ciphertext, len(second.Ciphertext), second.PlaintextBytes, second.KeyID, second.Nonce, second.Digest},
	}}}
	tx.queryRowHook = func(query string, _ []any) row {
		switch {
		case strings.Contains(query, "from fort_private.worker_lease"):
			return fakeRow{values: []any{now.Add(time.Minute), "working", "working"}}
		case strings.Contains(query, "from fort_private.artifact\n"):
			return fakeRow{values: []any{"attempt:1", "output", "uploading", 2, int64(11), int64(43), hex.EncodeToString(expected[:]), "test-key", now.Add(-time.Minute), sql.NullTime{}}}
		default:
			return fakeRow{err: errors.New("unexpected finalize digest query")}
		}
	}
	tx.execHook = func(query string, _ []any) (int64, error) {
		switch {
		case strings.Contains(query, "set_config"), strings.Contains(query, "idempotency_record"):
			return 1, nil
		case strings.Contains(query, "update fort_private.artifact"):
			return 1, nil
		default:
			return 0, errors.New("unexpected finalize digest statement")
		}
	}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.FinalizeWorkerArtifact(context.Background(), command); !errors.Is(err, controlapi.ErrWorkerArtifactIncomplete) {
		t.Fatalf("FinalizeWorkerArtifact error = %v, want artifact incomplete", err)
	}
	for _, statement := range tx.execs {
		if strings.Contains(statement.sql, "update fort_private.artifact") {
			t.Fatal("digest mismatch reached artifact state transition")
		}
	}
}

func testWorkerArtifactCreateCommand(now time.Time) controlapi.WorkerArtifactCreateCommand {
	return controlapi.WorkerArtifactCreateCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		IdempotencyKey: "artifact:create:1", ArtifactID: "artifact:output:1", ExpectedChunkCount: 2,
		ExpectedPlaintextLength: 11, LogicalDigest: strings.Repeat("d", 64), CreatedAt: now,
	}
}

func testWorkerArtifactChunkCommand(now time.Time) controlapi.WorkerArtifactChunkCommand {
	plaintext := []byte("hello")
	digest := sha256.Sum256(plaintext)
	return controlapi.WorkerArtifactChunkCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		IdempotencyKey: "artifact:chunk:1", ArtifactID: "artifact:output:1", ChunkIndex: 0,
		Plaintext: plaintext, PlaintextDigest: hex.EncodeToString(digest[:]), CreatedAt: now,
	}
}

func assertArtifactLeaseQuery(t *testing.T, query string, arguments []any, targetID, attemptID, leaseID, workerID, machineID string, fence int64) {
	t.Helper()
	if !strings.Contains(query, "machine.machine_id = $7") || !strings.Contains(query, "machine.state <> 'revoked'") ||
		!strings.Contains(query, "for update of lease") || len(arguments) != 7 || arguments[0] != testAccountID ||
		arguments[1] != leaseID || arguments[2] != attemptID || arguments[3] != targetID || arguments[4] != workerID ||
		arguments[5] != fence || arguments[6] != machineID {
		t.Fatalf("artifact lease query/arguments are not exact: %s %#v", query, arguments)
	}
}
