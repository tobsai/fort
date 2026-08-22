package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/cloud/securebody"
)

func TestReadWorkerContextPageRevalidatesPinnedLeaseAndDecryptsOrderedItems(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 23, 45, 0, 0, time.UTC)
	manifestID := "manifest:turn:1"
	manifestDigest := strings.Repeat("d", 64)
	directBody := sealWorkerContextTestBody(t, securebody.Scope{
		AccountID: testAccountID, RecordType: "conversation_message", RecordID: "turn:direct",
	}, "Direct context")
	groupBody := sealWorkerContextTestBody(t, securebody.Scope{
		AccountID: testAccountID, RecordType: "group_turn_prompt", RecordID: "turn:group",
	}, "Group context")
	tx := workerContextTransaction(now, manifestID, manifestDigest, &fakeRows{values: [][]any{
		workerContextMessageRow(0, 11, "conversation:direct", "turn:direct", "", "", "", "human", "human_direct", "human", "human:1", "", now.Add(-3*time.Minute), directBody),
		workerContextArtifactRow(1, "artifact:context:1", "context", "attempt:source", 2, 4096, 4128, strings.Repeat("e", 64), now.Add(-2*time.Minute), now.Add(-time.Minute)),
		workerContextMessageRow(2, 12, "conversation:group", "turn:group", "", "", "", "human", "human_group", "human", "human:1", "", now.Add(-time.Minute), groupBody),
	}})
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatalf("newStoreWithKeyRing: %v", err)
	}
	command := workerContextCommand(now)
	page, err := store.ReadWorkerContextPage(context.Background(), command)
	if err != nil {
		t.Fatalf("ReadWorkerContextPage: %v", err)
	}
	if page.ContextManifestID != manifestID || page.ManifestDigest != manifestDigest || page.NextCursor != "" || len(page.Items) != 3 {
		t.Fatalf("context page header/items = %#v", page)
	}
	if page.Items[0].Kind != controlapi.WorkerContextMessageKind || page.Items[0].Message == nil || page.Items[0].Message.Body != "Direct context" ||
		page.Items[1].Kind != controlapi.WorkerContextArtifactKind || page.Items[1].Artifact == nil || page.Items[1].Artifact.ArtifactID != "artifact:context:1" ||
		page.Items[2].Message == nil || page.Items[2].Message.Body != "Group context" {
		t.Fatalf("ordered context items = %#v", page.Items)
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("marshal page: %v", err)
	}
	if len(encoded)+1 > controlapi.MaximumWorkerContextPageEncodedBytes {
		t.Fatalf("encoded page = %d, exceeds %d", len(encoded)+1, controlapi.MaximumWorkerContextPageEncodedBytes)
	}
	for _, forbidden := range []string{"ciphertext", "key_id", "nonce", "chunks", "encryption_key"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("page exposed %q: %s", forbidden, encoded)
		}
	}
	if len(tx.queries) != 2 {
		t.Fatalf("context queries = %d, want lease + manifest page", len(tx.queries))
	}
	leaseQuery := tx.queries[0]
	for _, fragment := range []string{"lease.lease_id = $2", "lease.execution_attempt_id = $3", "lease.target_id = $4", "lease.worker_id = $5", "lease.fence_token = $6", "machine.machine_id = $7", "turn.context_manifest_id"} {
		if !strings.Contains(leaseQuery.sql, fragment) {
			t.Fatalf("lease query missing %q:\n%s", fragment, leaseQuery.sql)
		}
	}
	if got := leaseQuery.args; len(got) != 7 || got[0] != testAccountID || got[1] != "lease:1" || got[2] != "attempt:1" ||
		got[3] != "target:1" || got[4] != "worker:mac-studio" || got[5] != int64(41) || got[6] != "machine:mac-studio" {
		t.Fatalf("lease query args = %#v", got)
	}
	pageQuery := tx.queries[1]
	for _, fragment := range []string{"context_manifest_message", "context_manifest_artifact", "artifact.state = 'finalized'", "body_envelope_version", "order by ordinal, item_rank", "limit $5"} {
		if !strings.Contains(pageQuery.sql, fragment) {
			t.Fatalf("page query missing %q:\n%s", fragment, pageQuery.sql)
		}
	}
	if got := pageQuery.args; len(got) != 5 || got[0] != testAccountID || got[1] != manifestID || got[2] != -1 || got[3] != -1 || got[4] != controlapi.MaximumWorkerContextPageItems+1 {
		t.Fatalf("page query args = %#v", got)
	}
}

func TestReadWorkerContextPageUsesStableManifestBoundCursorAndOneMiBLimit(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	manifestID := "manifest:large"
	manifestDigest := strings.Repeat("f", 64)
	firstText, secondText := strings.Repeat("a", 600_000), strings.Repeat("b", 600_000)
	firstBody := sealWorkerContextTestBody(t, securebody.Scope{AccountID: testAccountID, RecordType: "conversation_message", RecordID: "target:1"}, firstText)
	secondBody := sealWorkerContextTestBody(t, securebody.Scope{AccountID: testAccountID, RecordType: "conversation_message", RecordID: "target:2"}, secondText)
	firstTx := workerContextTransaction(now, manifestID, manifestDigest, &fakeRows{values: [][]any{
		workerContextMessageRow(0, 21, "conversation:1", "turn:1", "target:1", "", "", "agent", "human_direct", "agent", "agent:1", "agent:1", now, firstBody),
		workerContextMessageRow(1, 22, "conversation:1", "turn:1", "target:2", "", "", "agent", "human_direct", "agent", "agent:2", "agent:2", now, secondBody),
	}})
	secondTx := workerContextTransaction(now.Add(time.Second), manifestID, manifestDigest, &fakeRows{values: [][]any{
		workerContextMessageRow(1, 22, "conversation:1", "turn:1", "target:2", "", "", "agent", "human_direct", "agent", "agent:2", "agent:2", now, secondBody),
	}})
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{firstTx, secondTx}}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatalf("newStoreWithKeyRing: %v", err)
	}
	firstPage, err := store.ReadWorkerContextPage(context.Background(), workerContextCommand(now))
	if err != nil {
		t.Fatalf("first ReadWorkerContextPage: %v", err)
	}
	firstEncoded, _ := json.Marshal(firstPage)
	if len(firstPage.Items) != 1 || firstPage.Items[0].Message.Body != firstText || firstPage.NextCursor == "" ||
		len(firstEncoded)+1 > controlapi.MaximumWorkerContextPageEncodedBytes {
		t.Fatalf("first page count/cursor/bytes = %d/%q/%d", len(firstPage.Items), firstPage.NextCursor, len(firstEncoded)+1)
	}
	secondCommand := workerContextCommand(now.Add(time.Second))
	secondCommand.Cursor = firstPage.NextCursor
	secondPage, err := store.ReadWorkerContextPage(context.Background(), secondCommand)
	if err != nil {
		t.Fatalf("second ReadWorkerContextPage: %v", err)
	}
	if len(secondPage.Items) != 1 || secondPage.Items[0].Message.Body != secondText || secondPage.NextCursor != "" {
		t.Fatalf("second page = %#v", secondPage)
	}
	if args := secondTx.queries[1].args; args[2] != 0 || args[3] != 0 {
		t.Fatalf("second page cursor position args = %#v, want ordinal/rank 0/0", args)
	}

	changed := workerContextCommand(now.Add(2 * time.Second))
	changed.Cursor = firstPage.NextCursor
	changed.TargetID = "target:other"
	changedTx := workerContextTransaction(now.Add(2*time.Second), manifestID, manifestDigest, &fakeRows{})
	changedStore, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{changedTx}}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatalf("new changed store: %v", err)
	}
	if _, err := changedStore.ReadWorkerContextPage(context.Background(), changed); !errors.Is(err, controlapi.ErrWorkerRequestInvalid) {
		t.Fatalf("cursor rebound to another target error = %v, want request invalid", err)
	}
	if len(changedTx.queries) != 1 {
		t.Fatalf("rebound cursor executed %d queries, want lease revalidation only", len(changedTx.queries))
	}
}

func TestReadWorkerContextPageRejectsOversizeSingleItemAndStaleLease(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 22, 0, 30, 0, 0, time.UTC)
	manifestID := "manifest:oversize"
	manifestDigest := strings.Repeat("a", 64)
	body := sealWorkerContextTestBody(t, securebody.Scope{AccountID: testAccountID, RecordType: "conversation_message", RecordID: "target:large"}, strings.Repeat("x", controlapi.MaximumWorkerContextPageEncodedBytes))
	largeTx := workerContextTransaction(now, manifestID, manifestDigest, &fakeRows{values: [][]any{
		workerContextMessageRow(0, 31, "conversation:1", "turn:1", "target:large", "", "", "agent", "human_direct", "agent", "agent:1", "agent:1", now, body),
	}})
	largeStore, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{largeTx}}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatalf("new large store: %v", err)
	}
	if _, err := largeStore.ReadWorkerContextPage(context.Background(), workerContextCommand(now)); !errors.Is(err, controlapi.ErrWorkerContextPageLimit) {
		t.Fatalf("oversize item error = %v, want page limit", err)
	}

	staleTx := &fakeTransaction{queryRowHook: func(string, []any) row { return fakeRow{err: pgx.ErrNoRows} }}
	staleStore, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{staleTx}}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatalf("new stale store: %v", err)
	}
	if _, err := staleStore.ReadWorkerContextPage(context.Background(), workerContextCommand(now)); !errors.Is(err, controlapi.ErrWorkerStaleLease) {
		t.Fatalf("stale lease error = %v, want stale lease", err)
	}
	if len(staleTx.queries) != 1 {
		t.Fatalf("stale lease executed %d queries, want one", len(staleTx.queries))
	}
}

func workerContextCommand(observedAt time.Time) controlapi.WorkerContextPageCommand {
	return controlapi.WorkerContextPageCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		IdempotencyKey: "context-page:1", ObservedAt: observedAt,
	}
}

func workerContextTransaction(now time.Time, manifestID, manifestDigest string, contextRows rows) *fakeTransaction {
	return &fakeTransaction{
		queryRows: contextRows,
		queryRowHook: func(query string, _ []any) row {
			if strings.Contains(query, "from fort_private.worker_lease") {
				return fakeRow{values: []any{manifestID, manifestDigest, now.Add(time.Minute), "working", "working"}}
			}
			return fakeRow{err: errors.New("unexpected query row")}
		},
	}
}

func sealWorkerContextTestBody(t *testing.T, scope securebody.Scope, plaintext string) collaborationEncryptedBody {
	t.Helper()
	body, err := (secureCollaborationBodyCipher{ring: collaborationTestKeyRing()}).seal(scope, plaintext)
	if err != nil {
		t.Fatalf("seal context body: %v", err)
	}
	return body
}

func workerContextMessageRow(ordinal int, messageID int64, conversationID, turnID, targetID, handoffID, routineRunID,
	messageKind, turnKind, authorKind, authorID, authorAgentID string, createdAt time.Time, body collaborationEncryptedBody) []any {
	metadata, _ := json.Marshal(map[string]any{
		"message_id": messageID, "conversation_id": conversationID, "turn_id": turnID, "target_id": targetID,
		"handoff_id": handoffID, "routine_run_id": routineRunID, "message_kind": messageKind,
		"turn_kind": turnKind, "author_kind": authorKind, "author_id": authorID, "author_agent_id": authorAgentID,
	})
	return []any{ordinal, 0, controlapi.WorkerContextMessageKind, string(metadata), createdAt, time.Time{},
		body.Ciphertext, int64(body.Version), body.KeyID, body.Nonce, body.Digest, int64(body.PlaintextBytes)}
}

func workerContextArtifactRow(ordinal int, artifactID, kind, attemptID string, chunks int, plaintextLength,
	encodedLength int64, digest string, createdAt, finalizedAt time.Time) []any {
	metadata, _ := json.Marshal(map[string]any{
		"artifact_id": artifactID, "kind": kind, "execution_attempt_id": attemptID,
		"expected_chunk_count": chunks, "expected_plaintext_length": plaintextLength,
		"expected_encoded_length": encodedLength, "logical_digest": digest,
	})
	return []any{ordinal, 1, controlapi.WorkerContextArtifactKind, string(metadata), createdAt, finalizedAt,
		[]byte(nil), int64(0), "", []byte(nil), "", int64(0)}
}
