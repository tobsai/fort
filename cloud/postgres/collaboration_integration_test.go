package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
	coreworker "github.com/tobsai/fort/core/worker"
)

// This test runs only when a local Supabase URL is supplied. It deliberately
// exercises the public repositories through the fort_gateway role; the admin
// pool is used only for isolated account setup and final invariant counts.
func TestPostgresCollaborationRepositoryIntegration(t *testing.T) {
	databaseURL := os.Getenv("FORT_TEST_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("FORT_TEST_POSTGRES_URL is not set")
	}
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
	if _, err := admin.Exec(ctx, `insert into fort_private.fort_account (
  account_id, normalized_email
) values ($1,$2)`, accountID, accountID+"@fort.test"); err != nil {
		t.Fatalf("seed Fort account: %v", err)
	}
	if _, err := admin.Exec(ctx, `insert into fort_private.worker (
  account_id, worker_id, machine_id, display_name, identity_key_digest,
  enrollment_token_hash, state, enrolled_at
) values ($1,$2,$2,'Collaboration Worker',$3,$4,'enrolled',$5)`, accountID,
		workerID, strings.Repeat("b", 64), strings.Repeat("c", 64), now.Add(-10*time.Minute)); err != nil {
		t.Fatalf("seed worker: %v", err)
	}

	config, err := SupavisorTransactionConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse runtime config: %v", err)
	}
	config.AfterConnect = func(ctx context.Context, connection *pgx.Conn) error {
		_, err := connection.Exec(ctx, "set role fort_gateway")
		return err
	}
	runtimePool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("open runtime pool: %v", err)
	}
	store, err := NewWithKeyRing(runtimePool, accountID, collaborationTestKeyRing())
	if err != nil {
		runtimePool.Close()
		t.Fatalf("NewWithKeyRing: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	researcher := integrationAgentCommand(accountID, workerID)
	builder := integrationAgentCommand(accountID, workerID)
	builder.ExecutionSource.GatewayID = "gateway:" + uuid.NewString()
	builder.ExecutionSource.InstanceID = "instance:" + uuid.NewString()
	if _, err := store.CreateAgent(ctx, researcher); err != nil {
		t.Fatalf("CreateAgent researcher: %v", err)
	}
	if _, err := store.CreateAgent(ctx, builder); err != nil {
		t.Fatalf("CreateAgent builder: %v", err)
	}

	groupCommand := postgresGroupCommand(researcher, builder)
	groupCommand.Group.AccountID = accountID
	groupCommand.Group.CreatedAt = now.Add(-4 * time.Minute)
	groupCommand.Conversation.CreatedAt = groupCommand.Group.CreatedAt
	groupCommand.Conversation.UpdatedAt = groupCommand.Group.CreatedAt
	groupCommand.Membership.CreatedAt = groupCommand.Group.CreatedAt
	createdGroup, err := store.CreateGroup(ctx, groupCommand)
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	replayedGroup, err := store.CreateGroup(ctx, groupCommand)
	if err != nil || replayedGroup.Group.ID != createdGroup.Group.ID {
		t.Fatalf("CreateGroup replay = %+v, %v", replayedGroup, err)
	}
	conflictingGroup := groupCommand
	conflictingGroup.Conversation.Title = "Conflicting title"
	if _, err := store.CreateGroup(ctx, conflictingGroup); !errors.Is(err, ledger.ErrIdempotencyConflict) {
		t.Fatalf("CreateGroup conflict = %v", err)
	}
	groups, err := store.ListGroups(ctx, accountID, conversation.ConversationOpen)
	if err != nil || len(groups) != 1 || groups[0].Group.ID != groupCommand.Group.ID {
		t.Fatalf("ListGroups = %+v, %v", groups, err)
	}

	sendCommand := postgresGroupSendCommand(groupCommand)
	sendCommand.AccountID = accountID
	sendCommand.Envelope.CreatedAt = now.Add(-3 * time.Minute)
	sendCommand.Envelope.Deadline = now.Add(10 * time.Minute)
	sent, err := store.SendGroupTurn(ctx, sendCommand)
	if err != nil {
		t.Fatalf("SendGroupTurn: %v", err)
	}
	replayedTurn, err := store.SendGroupTurn(ctx, sendCommand)
	if err != nil || replayedTurn.Message.ID != sent.Message.ID || replayedTurn.Message.Body != sendCommand.Body {
		t.Fatalf("SendGroupTurn replay = %+v, %v", replayedTurn, err)
	}
	conflictingSend := sendCommand
	conflictingSend.Body = "A conflicting prompt"
	if _, err := store.SendGroupTurn(ctx, conflictingSend); !errors.Is(err, ledger.ErrIdempotencyConflict) {
		t.Fatalf("SendGroupTurn conflict = %v", err)
	}
	turns, err := store.ListGroupTurns(ctx, accountID, groupCommand.Group.ID)
	if err != nil || len(turns) != 1 || len(turns[0].InitialTargets) != len(sendCommand.Envelope.Recipients) {
		t.Fatalf("ListGroupTurns = %+v, %v", turns, err)
	}

	handoffCommand := postgresHandoffCommand(t, sent, groupCommand, builder)
	handoffCommand.Handoff.AccountID = accountID
	sourceMessageID := strconv.FormatInt(sent.Message.ID, 10)
	handoffCommand.Handoff.SourceMessageID = sourceMessageID
	handoffCommand.Handoff.Context = conversation.ContextManifest{References: []conversation.ContextReference{{
		Kind: conversation.ContextMessage, ID: sourceMessageID, AccountID: accountID, Immutable: true,
	}}}
	handoffCommand.Handoff.RootDelegationGrant.ID = "grant:handoff:" + uuid.NewString()
	handoffCommand.Handoff.RootDelegationGrant.ContextRecordIDs = []string{"message:" + sourceMessageID}
	handoffCommand.Handoff.CreatedAt = now.Add(-2 * time.Minute)
	handoffCommand.Handoff.Deadline = now.Add(8 * time.Minute)
	handoffCommand.Handoff.EffectiveAuthority, err = conversation.ComputeEffectiveAuthority(
		handoffCommand.Handoff.RequestedAuthority,
		handoffCommand.Handoff.RootDelegationGrant,
		handoffCommand.Handoff.HandoffPolicy,
		handoffCommand.Handoff.RecipientBindingPolicy,
	)
	if err != nil {
		t.Fatalf("compute Handoff authority: %v", err)
	}
	accepted, err := store.AcceptHandoff(ctx, handoffCommand)
	if err != nil {
		t.Fatalf("AcceptHandoff: %v", err)
	}
	replayedHandoff, err := store.AcceptHandoff(ctx, handoffCommand)
	if err != nil || replayedHandoff.Handoff.ID != accepted.Handoff.ID ||
		replayedHandoff.Handoff.RequestedResult != handoffCommand.Handoff.RequestedResult {
		t.Fatalf("AcceptHandoff replay = %+v, %v", replayedHandoff, err)
	}
	conflictingHandoff := handoffCommand
	conflictingHandoff.Handoff.RequestedResult = "A conflicting requested result"
	if _, err := store.AcceptHandoff(ctx, conflictingHandoff); !errors.Is(err, ledger.ErrIdempotencyConflict) {
		t.Fatalf("AcceptHandoff conflict = %v", err)
	}

	capabilityEvidence := json.RawMessage(`{"frameworks":["codex"],"ready":true}`)
	capabilityDigest := sha256.Sum256(capabilityEvidence)
	capabilityID := "capability:" + uuid.NewString()
	readinessAt := now.Add(-90 * time.Second)
	if _, err := store.RecordWorkerReadiness(ctx, controlapi.WorkerReadinessCommand{
		AccountID: accountID, WorkerID: workerID, MachineID: workerID,
		IdempotencyKey: "readiness:" + uuid.NewString(), CapabilityRevisionID: capabilityID,
		Revision: 1, CapabilityEvidence: capabilityEvidence,
		EvidenceDigest: hex.EncodeToString(capabilityDigest[:]), ObservedAt: readinessAt,
	}); err != nil {
		t.Fatalf("RecordWorkerReadiness: %v", err)
	}
	claimAt := now.Add(-time.Minute)
	groupClaim := controlapi.WorkerClaimCommand{
		AccountID: accountID, WorkerID: workerID, MachineID: workerID,
		TargetID: sent.InitialTargets[0].ID, ExecutionAttemptID: "attempt:group:" + uuid.NewString(),
		LeaseID: "lease:group:" + uuid.NewString(), IdempotencyKey: "claim:group:" + uuid.NewString(),
		CapabilityRevisionID: capabilityID, ClaimedAt: claimAt, ExpiresAt: claimAt.Add(2 * time.Minute),
	}
	groupAssignment, err := store.ClaimWorkerTarget(ctx, groupClaim)
	if err != nil {
		t.Fatalf("ClaimWorkerTarget Group target: %v", err)
	}
	if groupAssignment.ContextManifestID != sendCommand.Envelope.ContextSnapshotID {
		t.Fatalf("Group worker assignment context = %q, want only frozen manifest %q",
			groupAssignment.ContextManifestID, sendCommand.Envelope.ContextSnapshotID)
	}
	groupBody := "Research found the exact Group evidence."
	groupBodyHash := sha256.Sum256([]byte(groupBody))
	groupBodyDigest := hex.EncodeToString(groupBodyHash[:])
	groupArtifactID := "artifact:group-output:" + uuid.NewString()
	groupArtifactCiphertext := []byte("opaque encrypted Group output")
	if _, err := admin.Exec(ctx, `insert into fort_private.artifact(
  account_id,artifact_id,execution_attempt_id,kind,state,expected_chunk_count,
  expected_plaintext_length,expected_encoded_length,logical_digest,encryption_key_id,created_at
) values($1,$2,$3,'output','uploading',1,$4,$5,$6,'test-key',$7)`, accountID, groupArtifactID,
		groupAssignment.ExecutionAttemptID, len(groupBody), len(groupArtifactCiphertext), groupBodyDigest,
		claimAt.Add(2*time.Second)); err != nil {
		t.Fatalf("create Group output artifact: %v", err)
	}
	if _, err := admin.Exec(ctx, `insert into fort_private.artifact_chunk(
  account_id,artifact_id,chunk_index,ciphertext,encoded_length,plaintext_length,
  encryption_key_id,nonce,authenticated_digest,created_at
) values($1,$2,0,$3,$4,$5,'test-key',$6,$7,$8)`, accountID, groupArtifactID,
		groupArtifactCiphertext, len(groupArtifactCiphertext), len(groupBody), []byte("0123456789ab"),
		groupBodyDigest, claimAt.Add(2*time.Second)); err != nil {
		t.Fatalf("append Group output artifact chunk: %v", err)
	}
	if _, err := admin.Exec(ctx, `update fort_private.artifact
set state='finalized',finalized_at=$3 where account_id=$1 and artifact_id=$2`, accountID,
		groupArtifactID, claimAt.Add(2*time.Second)); err != nil {
		t.Fatalf("finalize Group output artifact: %v", err)
	}
	groupTerminal, err := store.CommitWorkerTerminal(ctx, controlapi.WorkerTerminalCommand{
		AccountID: accountID, WorkerID: workerID, MachineID: workerID,
		TargetID: groupClaim.TargetID, ExecutionAttemptID: groupAssignment.ExecutionAttemptID,
		LeaseID: groupAssignment.LeaseID, FenceToken: groupAssignment.FenceToken,
		TerminalReceiptID: "receipt:group:" + uuid.NewString(), IdempotencyKey: "terminal:group:" + uuid.NewString(),
		Status:                 coreworker.TerminalCompleted,
		ReceiptPlaintext:       json.RawMessage(`{"status":"completed","exit_code":0}`),
		Output:                 controlapi.WorkerOutputReference{ArtifactID: groupArtifactID, Digest: groupBodyDigest},
		OutputMessagePlaintext: &groupBody,
		CommittedAt:            claimAt.Add(3 * time.Second),
	})
	if err != nil || groupTerminal.MessageID == 0 {
		t.Fatalf("CommitWorkerTerminal Group target = %+v, %v", groupTerminal, err)
	}
	groupMessages, err := store.ListGroupMessages(ctx, accountID, groupCommand.Group.ID)
	if err != nil || len(groupMessages) != 2 || groupMessages[1].Body != groupBody ||
		groupMessages[1].AuthorAgentID != researcher.Agent.ID || groupMessages[1].TargetID != groupClaim.TargetID {
		t.Fatalf("ListGroupMessages after worker terminal = %+v, %v", groupMessages, err)
	}
	claimCommand := controlapi.WorkerClaimCommand{
		AccountID: accountID, WorkerID: workerID, MachineID: workerID,
		TargetID: handoffCommand.TargetID, ExecutionAttemptID: "attempt:" + uuid.NewString(),
		LeaseID: "lease:" + uuid.NewString(), IdempotencyKey: "claim:" + uuid.NewString(),
		CapabilityRevisionID: capabilityID, ClaimedAt: claimAt, ExpiresAt: claimAt.Add(2 * time.Minute),
	}
	assignment, err := store.ClaimWorkerTarget(ctx, claimCommand)
	if err != nil {
		t.Fatalf("ClaimWorkerTarget: %v", err)
	}
	if assignment.Pins.AgentID != builder.Agent.ID || assignment.Pins.BehaviorRevisionID != builder.Behavior.ID ||
		assignment.Pins.BindingRevisionID != builder.Binding.ID || assignment.FenceToken < 1 ||
		assignment.ContextManifestID != handoffContextManifestID(handoffCommand.Handoff.ID) {
		t.Fatalf("worker assignment = %+v", assignment)
	}

	startCommand := ledger.StartHandoffCommand{
		AccountID: accountID, HandoffID: handoffCommand.Handoff.ID,
		IdempotencyKey: "start:" + uuid.NewString(), AttemptID: claimCommand.ExecutionAttemptID,
		LeaseID: claimCommand.LeaseID, MachineID: workerID,
		FenceToken: strconv.FormatInt(assignment.FenceToken, 10),
		StartedAt:  claimAt.Add(time.Second), LeaseExpiresAt: claimCommand.ExpiresAt,
	}
	startResults := make([]ledger.HandoffRecord, 2)
	startErrors := make([]error, 2)
	var startWait sync.WaitGroup
	for index := range startResults {
		startWait.Add(1)
		go func(index int) {
			defer startWait.Done()
			startResults[index], startErrors[index] = store.StartHandoff(ctx, startCommand)
		}(index)
	}
	startWait.Wait()
	for index, err := range startErrors {
		if err != nil || startResults[index].Attempt == nil ||
			startResults[index].Attempt.ID != claimCommand.ExecutionAttemptID {
			t.Fatalf("concurrent StartHandoff %d = %+v, %v", index, startResults[index], err)
		}
	}

	staleComplete := ledger.CompleteHandoffCommand{
		AccountID: accountID, HandoffID: handoffCommand.Handoff.ID,
		IdempotencyKey: "complete-stale:" + uuid.NewString(), AuthorAgentID: builder.Agent.ID,
		AttemptID: claimCommand.ExecutionAttemptID, LeaseID: claimCommand.LeaseID,
		FenceToken:        strconv.FormatInt(assignment.FenceToken+1, 10),
		TerminalReceiptID: "receipt-stale:" + uuid.NewString(), Body: "stale",
		CreatedAt: startCommand.StartedAt.Add(time.Second),
	}
	if _, err := store.CompleteHandoff(ctx, staleComplete); err == nil {
		t.Fatal("CompleteHandoff accepted a stale fence")
	}
	atExpiry := staleComplete
	atExpiry.IdempotencyKey = "complete-expired:" + uuid.NewString()
	atExpiry.FenceToken = startCommand.FenceToken
	atExpiry.TerminalReceiptID = "receipt-expired:" + uuid.NewString()
	atExpiry.CreatedAt = startCommand.LeaseExpiresAt
	if _, err := store.CompleteHandoff(ctx, atExpiry); err == nil {
		t.Fatal("CompleteHandoff accepted a result at the exact lease expiry")
	}

	completeCommand := atExpiry
	completeCommand.IdempotencyKey = "complete:" + uuid.NewString()
	completeCommand.TerminalReceiptID = "receipt:" + uuid.NewString()
	completeCommand.Body = "The launch evidence is ready."
	completeCommand.CreatedAt = startCommand.StartedAt.Add(2 * time.Second)
	completeResults := make([]ledger.HandoffRecord, 2)
	completeErrors := make([]error, 2)
	var completeWait sync.WaitGroup
	for index := range completeResults {
		completeWait.Add(1)
		go func(index int) {
			defer completeWait.Done()
			completeResults[index], completeErrors[index] = store.CompleteHandoff(ctx, completeCommand)
		}(index)
	}
	completeWait.Wait()
	for index, err := range completeErrors {
		if err != nil || completeResults[index].Result == nil ||
			completeResults[index].Result.Body != completeCommand.Body ||
			completeResults[index].Attempt == nil ||
			completeResults[index].Attempt.TerminalReceiptID != completeCommand.TerminalReceiptID {
			t.Fatalf("concurrent CompleteHandoff %d = %+v, %v", index, completeResults[index], err)
		}
	}

	secondCompletion := completeCommand
	secondCompletion.IdempotencyKey = "complete-second:" + uuid.NewString()
	secondCompletion.TerminalReceiptID = "receipt-second:" + uuid.NewString()
	if _, err := store.CompleteHandoff(ctx, secondCompletion); !errors.Is(err, ledger.ErrAlreadyCompleted) {
		t.Fatalf("second Handoff completion = %v", err)
	}
	finalRecord, err := store.GetHandoff(ctx, accountID, handoffCommand.Handoff.ID)
	if err != nil || finalRecord.Handoff.State != conversation.HandoffCompleted ||
		finalRecord.Result == nil || finalRecord.Result.Body != completeCommand.Body || len(finalRecord.Projections) != 1 ||
		finalRecord.Projections[0].AuthoritativeMessageID != finalRecord.Result.MessageID {
		t.Fatalf("GetHandoff final = %+v, %v", finalRecord, err)
	}
	handoffs, err := store.ListHandoffs(ctx, accountID)
	if err != nil || len(handoffs) != 1 || handoffs[0].Handoff.ID != finalRecord.Handoff.ID {
		t.Fatalf("ListHandoffs = %+v, %v", handoffs, err)
	}

	deadlineCommand := handoffCommand
	deadlineCommand.Handoff.ID = "handoff:deadline:" + uuid.NewString()
	deadlineCommand.Handoff.IdempotencyKey = "handoff-deadline:" + uuid.NewString()
	deadlineCommand.TargetID = "target:deadline:" + uuid.NewString()
	deadlineCommand.Handoff.RootDelegationGrant.ID = "grant:deadline:" + uuid.NewString()
	deadlineCommand.Handoff.CreatedAt = now.Add(-time.Minute)
	deadlineCommand.Handoff.Deadline = now.Add(time.Minute)
	deadlineCommand.Handoff.EffectiveAuthority, err = conversation.ComputeEffectiveAuthority(
		deadlineCommand.Handoff.RequestedAuthority,
		deadlineCommand.Handoff.RootDelegationGrant,
		deadlineCommand.Handoff.HandoffPolicy,
		deadlineCommand.Handoff.RecipientBindingPolicy,
	)
	if err != nil {
		t.Fatalf("compute deadline Handoff authority: %v", err)
	}
	if _, err := store.AcceptHandoff(ctx, deadlineCommand); err != nil {
		t.Fatalf("AcceptHandoff deadline fixture: %v", err)
	}
	deadlineClaim := controlapi.WorkerClaimCommand{
		AccountID: accountID, WorkerID: workerID, MachineID: workerID,
		TargetID: deadlineCommand.TargetID, ExecutionAttemptID: "attempt:deadline:" + uuid.NewString(),
		LeaseID: "lease:deadline:" + uuid.NewString(), IdempotencyKey: "claim:deadline:" + uuid.NewString(),
		CapabilityRevisionID: capabilityID, ClaimedAt: deadlineCommand.Handoff.Deadline.Add(-10 * time.Second),
		ExpiresAt: deadlineCommand.Handoff.Deadline.Add(time.Minute),
	}
	deadlineAssignment, err := store.ClaimWorkerTarget(ctx, deadlineClaim)
	if err != nil {
		t.Fatalf("ClaimWorkerTarget deadline fixture: %v", err)
	}
	deadlineStart := ledger.StartHandoffCommand{
		AccountID: accountID, HandoffID: deadlineCommand.Handoff.ID,
		IdempotencyKey: "start:deadline:" + uuid.NewString(), AttemptID: deadlineClaim.ExecutionAttemptID,
		LeaseID: deadlineClaim.LeaseID, MachineID: workerID,
		FenceToken: strconv.FormatInt(deadlineAssignment.FenceToken, 10),
		StartedAt:  deadlineCommand.Handoff.Deadline, LeaseExpiresAt: deadlineClaim.ExpiresAt,
	}
	needsYou, err := store.StartHandoff(ctx, deadlineStart)
	if !errors.Is(err, conversation.ErrHandoffNeedsYou) || needsYou.Handoff.State != conversation.HandoffNeedsYou {
		t.Fatalf("deadline StartHandoff = %+v, %v", needsYou, err)
	}
	if _, err := store.StartHandoff(ctx, deadlineStart); !errors.Is(err, conversation.ErrHandoffNeedsYou) {
		t.Fatalf("deadline StartHandoff replay = %v", err)
	}
	var handoffState, targetState, attemptState, leaseState, turnState string
	if err := admin.QueryRow(ctx, `select handoff.state, target.state, attempt.state, lease.state, turn.state
from fort_private.handoff as handoff
join fort_private.conversation_target as target
  on target.account_id = handoff.account_id and target.target_id = handoff.target_id
join fort_private.execution_attempt as attempt
  on attempt.account_id = target.account_id and attempt.target_id = target.target_id
join fort_private.worker_lease as lease
  on lease.account_id = attempt.account_id and lease.execution_attempt_id = attempt.execution_attempt_id
join fort_private.conversation_turn as turn
  on turn.account_id = target.account_id and turn.turn_id = target.turn_id
where handoff.account_id = $1 and handoff.handoff_id = $2`, accountID,
		deadlineCommand.Handoff.ID).Scan(&handoffState, &targetState, &attemptState, &leaseState, &turnState); err != nil {
		t.Fatalf("read deadline Handoff states: %v", err)
	}
	if handoffState != "needs_you" || targetState != "needs_you" || attemptState != "needs_you" ||
		leaseState != "revoked" || turnState != "needs_you" {
		t.Fatalf("deadline Handoff states = %q %q %q %q %q", handoffState, targetState,
			attemptState, leaseState, turnState)
	}

	var initialTargets, attempts, results, projections, manifestMessages int
	var manifestDigest string
	if err := admin.QueryRow(ctx, `select
  (select count(*) from fort_private.conversation_target where account_id = $1 and turn_id = $2 and target_kind = 'initial'),
  (select count(*) from fort_private.handoff_attempt where account_id = $1 and handoff_id = $3),
  (select count(*) from fort_private.conversation_message where account_id = $1 and handoff_id = $3 and message_kind = 'handoff_result'),
	(select count(*) from fort_private.handoff_projection where account_id = $1 and handoff_id = $3),
	(select count(*) from fort_private.context_manifest_message where account_id = $1 and context_manifest_id = $4),
	(select manifest_digest from fort_private.context_manifest where account_id = $1 and context_manifest_id = $4)`,
		accountID, sendCommand.Envelope.ID, handoffCommand.Handoff.ID,
		sendCommand.Envelope.ContextSnapshotID).Scan(
		&initialTargets, &attempts, &results, &projections, &manifestMessages, &manifestDigest); err != nil {
		t.Fatalf("read collaboration invariant counts: %v", err)
	}
	wantManifestDigest, err := evidenceDigest(struct {
		Version          int     `json:"version"`
		ConversationID   string  `json:"conversation_id"`
		ThroughMessageID int64   `json:"through_message_id"`
		MessageIDs       []int64 `json:"message_ids"`
	}{1, sendCommand.Envelope.ConversationID, sent.Message.ID, []int64{sent.Message.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if initialTargets != len(sendCommand.Envelope.Recipients) || attempts != 1 || results != 1 ||
		projections != 1 || manifestMessages != 1 || manifestDigest != wantManifestDigest {
		t.Fatalf("collaboration counts = targets %d attempts %d results %d projections %d manifest messages %d digest %q",
			initialTargets, attempts, results, projections, manifestMessages, manifestDigest)
	}
}
