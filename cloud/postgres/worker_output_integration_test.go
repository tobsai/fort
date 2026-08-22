package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/ledger"
	coreworker "github.com/tobsai/fort/core/worker"
)

func TestPostgresWorkerOutputBecomesExactlyOneAuthoritativeAgentMessage(t *testing.T) {
	databaseURL := integrationDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	accountID := uuid.NewString()
	workerID := "worker:" + uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	t.Cleanup(admin.Close)
	if _, err := admin.Exec(ctx, `insert into fort_private.fort_account(account_id,normalized_email)
values($1,$2)`, accountID, accountID+"@worker-output.fort.test"); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := admin.Exec(ctx, `insert into fort_private.worker(
  account_id,worker_id,machine_id,display_name,identity_key_digest,enrollment_token_hash,state,enrolled_at
) values($1,$2,$2,'Output Worker',$3,$4,'enrolled',$5)`, accountID, workerID,
		shaString("output-worker"), shaString("output-token"), now); err != nil {
		t.Fatalf("seed worker: %v", err)
	}

	config, err := SupavisorTransactionConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.AfterConnect = func(ctx context.Context, connection *pgx.Conn) error {
		_, err := connection.Exec(ctx, "set role fort_gateway")
		return err
	}
	runtimePool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("open runtime pool: %v", err)
	}
	ring := collaborationTestKeyRing()
	store, err := NewWithKeyRing(runtimePool, accountID, ring)
	if err != nil {
		runtimePool.Close()
		t.Fatalf("NewWithKeyRing: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	agent := integrationAgentCommand(accountID, workerID)
	if _, err := store.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	suffix := uuid.NewString()
	turn := postgresDirectAgentTurnCommand(agent, suffix)
	turn.AccountID = accountID
	turn.CreatedAt, turn.HardDeadline = now.Add(time.Second), now.Add(10*time.Minute)
	if _, err := store.SendAgentTurn(ctx, turn); err != nil {
		t.Fatalf("SendAgentTurn: %v", err)
	}
	evidence := json.RawMessage(`{"frameworks":["openclaw"],"ready":true}`)
	evidenceDigest := sha256.Sum256(evidence)
	capabilityID := "capability:" + suffix
	if _, err := store.RecordWorkerReadiness(ctx, controlapi.WorkerReadinessCommand{
		AccountID: accountID, WorkerID: workerID, MachineID: workerID,
		IdempotencyKey: "readiness:" + suffix, CapabilityRevisionID: capabilityID, Revision: 1,
		CapabilityEvidence: evidence, EvidenceDigest: hex.EncodeToString(evidenceDigest[:]),
		ObservedAt: now.Add(2 * time.Second),
	}); err != nil {
		t.Fatalf("RecordWorkerReadiness: %v", err)
	}
	assignment, err := store.ClaimWorkerTarget(ctx, controlapi.WorkerClaimCommand{
		AccountID: accountID, WorkerID: workerID, MachineID: workerID,
		TargetID: turn.TargetID, ExecutionAttemptID: "attempt:" + suffix, LeaseID: "lease:" + suffix,
		IdempotencyKey: "claim:" + suffix, CapabilityRevisionID: capabilityID,
		ClaimedAt: now.Add(3 * time.Second), ExpiresAt: now.Add(3*time.Second + controlapi.DefaultWorkerLease),
	})
	if err != nil {
		t.Fatalf("ClaimWorkerTarget: %v", err)
	}

	body := "A persisted response from the exact OpenClaw Agent target."
	bodyHash := sha256.Sum256([]byte(body))
	bodyDigest := hex.EncodeToString(bodyHash[:])
	artifactID := "artifact:output:" + suffix
	chunkPlaintexts := [][]byte{[]byte(body[:len(body)/2]), []byte(body[len(body)/2:])}
	artifact, err := store.CreateWorkerArtifact(ctx, controlapi.WorkerArtifactCreateCommand{
		AccountID: accountID, WorkerID: workerID, MachineID: workerID,
		TargetID: turn.TargetID, ExecutionAttemptID: assignment.ExecutionAttemptID,
		LeaseID: assignment.LeaseID, FenceToken: assignment.FenceToken,
		IdempotencyKey: "artifact:create:" + suffix, ArtifactID: artifactID,
		ExpectedChunkCount: 2, ExpectedPlaintextLength: int64(len(body)),
		LogicalDigest: bodyDigest, CreatedAt: now.Add(4 * time.Second),
	})
	if err != nil || !artifact.Created || artifact.Kind != "output" || artifact.State != "uploading" {
		t.Fatalf("CreateWorkerArtifact = %+v, %v", artifact, err)
	}
	chunkCommand := func(index int, idempotencyKey string) controlapi.WorkerArtifactChunkCommand {
		digest := sha256.Sum256(chunkPlaintexts[index])
		return controlapi.WorkerArtifactChunkCommand{
			AccountID: accountID, WorkerID: workerID, MachineID: workerID,
			TargetID: turn.TargetID, ExecutionAttemptID: assignment.ExecutionAttemptID,
			LeaseID: assignment.LeaseID, FenceToken: assignment.FenceToken,
			IdempotencyKey: idempotencyKey, ArtifactID: artifactID, ChunkIndex: index,
			Plaintext: chunkPlaintexts[index], PlaintextDigest: hex.EncodeToString(digest[:]),
			CreatedAt: now.Add(time.Duration(5+index) * time.Second),
		}
	}
	secondChunk := chunkCommand(1, "artifact:chunk:1:"+suffix)
	appended, err := store.AppendWorkerArtifactChunk(ctx, secondChunk)
	if err != nil || !appended.Created || appended.ChunkIndex != 1 {
		t.Fatalf("append out-of-order chunk = %+v, %v", appended, err)
	}
	status, err := store.GetWorkerArtifactStatus(ctx, controlapi.WorkerArtifactStatusCommand{
		AccountID: accountID, WorkerID: workerID, MachineID: workerID, TargetID: turn.TargetID,
		ExecutionAttemptID: assignment.ExecutionAttemptID, LeaseID: assignment.LeaseID,
		FenceToken: assignment.FenceToken, IdempotencyKey: "artifact:status:partial:" + suffix,
		ArtifactID: artifactID, ObservedAt: now.Add(6 * time.Second),
	})
	if err != nil || len(status.Chunks) != 1 || status.Chunks[0].ChunkIndex != 1 {
		t.Fatalf("interrupted artifact status = %+v, %v", status, err)
	}
	replayedChunk, err := store.AppendWorkerArtifactChunk(ctx, secondChunk)
	if err != nil || replayedChunk.Created {
		t.Fatalf("exact chunk replay = %+v, %v", replayedChunk, err)
	}
	changedChunk := secondChunk
	changedChunk.IdempotencyKey = "artifact:chunk:changed:" + suffix
	changedChunk.Plaintext = append([]byte(nil), secondChunk.Plaintext...)
	changedChunk.Plaintext[0] ^= 1
	changedDigest := sha256.Sum256(changedChunk.Plaintext)
	changedChunk.PlaintextDigest = hex.EncodeToString(changedDigest[:])
	if _, err := store.AppendWorkerArtifactChunk(ctx, changedChunk); !errors.Is(err, controlapi.ErrWorkerIdempotencyConflict) {
		t.Fatalf("changed chunk replay = %v, want idempotency conflict", err)
	}
	if _, err := store.AppendWorkerArtifactChunk(ctx, chunkCommand(0, "artifact:chunk:0:"+suffix)); err != nil {
		t.Fatalf("append first chunk after resume: %v", err)
	}
	finalized, err := store.FinalizeWorkerArtifact(ctx, controlapi.WorkerArtifactFinalizeCommand{
		AccountID: accountID, WorkerID: workerID, MachineID: workerID, TargetID: turn.TargetID,
		ExecutionAttemptID: assignment.ExecutionAttemptID, LeaseID: assignment.LeaseID,
		FenceToken: assignment.FenceToken, IdempotencyKey: "artifact:finalize:" + suffix,
		ArtifactID: artifactID, FinalizedAt: now.Add(7 * time.Second),
	})
	if err != nil || !finalized.Created || finalized.State != "finalized" {
		t.Fatalf("FinalizeWorkerArtifact = %+v, %v", finalized, err)
	}

	terminal := controlapi.WorkerTerminalCommand{
		AccountID: accountID, WorkerID: workerID, MachineID: workerID,
		TargetID: turn.TargetID, ExecutionAttemptID: assignment.ExecutionAttemptID,
		LeaseID: assignment.LeaseID, FenceToken: assignment.FenceToken,
		TerminalReceiptID: "receipt:" + suffix, IdempotencyKey: "terminal:" + suffix,
		Status:                 coreworker.TerminalCompleted,
		ReceiptPlaintext:       json.RawMessage(`{"status":"completed","exit_code":0}`),
		Output:                 controlapi.WorkerOutputReference{ArtifactID: artifactID, Digest: bodyDigest},
		OutputMessagePlaintext: &body,
		CommittedAt:            now.Add(8 * time.Second),
	}
	committed, err := store.CommitWorkerTerminal(ctx, terminal)
	if err != nil {
		t.Fatalf("CommitWorkerTerminal: %v", err)
	}
	if !committed.Created || committed.MessageID == 0 {
		t.Fatalf("terminal result = %+v", committed)
	}
	replayed, err := store.CommitWorkerTerminal(ctx, terminal)
	if err != nil || replayed.Created || replayed.MessageID != committed.MessageID {
		t.Fatalf("terminal replay = %+v, %v", replayed, err)
	}
	conflict := terminal
	conflict.ReceiptPlaintext = json.RawMessage(`{"status":"completed","exit_code":0,"changed":true}`)
	if _, err := store.CommitWorkerTerminal(ctx, conflict); !errors.Is(err, controlapi.ErrWorkerIdempotencyConflict) {
		t.Fatalf("conflicting terminal replay = %v, want idempotency conflict", err)
	}

	projection, err := store.ReadAgentConversation(ctx, accountID, agent.Agent.ID, agent.Home.ID)
	if err != nil {
		t.Fatalf("ReadAgentConversation: %v", err)
	}
	if len(projection.Messages) != 2 || projection.Messages[1].Body != body ||
		projection.Messages[1].AuthorAgentID != agent.Agent.ID || projection.Messages[1].TargetID != turn.TargetID ||
		len(projection.Targets) != 1 || projection.Targets[0].State != "succeeded" {
		t.Fatalf("direct Agent projection = %+v", projection)
	}
	var messageCount int
	if err := admin.QueryRow(ctx, `select count(*) from fort_private.conversation_message
where account_id=$1 and target_id=$2 and message_kind='agent'`, accountID, turn.TargetID).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 1 {
		t.Fatalf("authoritative Agent message count = %d, want 1", messageCount)
	}
}

var _ ledger.AgentDirectChatRepository = (*Store)(nil)
