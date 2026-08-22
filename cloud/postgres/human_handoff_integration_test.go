package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestPostgresHumanHandoffGroupDirectAndCancellationIntegration(t *testing.T) {
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
	defer admin.Close()
	if _, err := admin.Exec(ctx, `insert into fort_private.fort_account(account_id,normalized_email)
values($1,$2)`, accountID, accountID+"@human-handoff.fort.test"); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := admin.Exec(ctx, `insert into fort_private.worker(
  account_id,worker_id,machine_id,display_name,identity_key_digest,enrollment_token_hash,state,enrolled_at
) values($1,$2,$2,'Human Handoff Worker',$3,$4,'enrolled',$5)`, accountID, workerID,
		strings.Repeat("b", 64), strings.Repeat("c", 64), now.Add(-time.Hour)); err != nil {
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
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewWithKeyRing(pool, accountID, collaborationTestKeyRing())
	if err != nil {
		pool.Close()
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	sender := integrationAgentCommand(accountID, workerID)
	recipient := integrationAgentCommand(accountID, workerID)
	recipient.ExecutionSource.GatewayID = "gateway:" + uuid.NewString()
	recipient.ExecutionSource.InstanceID = "instance:" + uuid.NewString()
	if _, err := store.CreateAgent(ctx, sender); err != nil {
		t.Fatalf("CreateAgent sender: %v", err)
	}
	if _, err := store.CreateAgent(ctx, recipient); err != nil {
		t.Fatalf("CreateAgent recipient: %v", err)
	}
	group := postgresGroupCommand(sender, recipient)
	group.Group.AccountID = accountID
	group.Group.CreatedAt = now.Add(-5 * time.Minute)
	group.Conversation.CreatedAt, group.Conversation.UpdatedAt = group.Group.CreatedAt, group.Group.CreatedAt
	group.Membership.CreatedAt = group.Group.CreatedAt
	if _, err := store.CreateGroup(ctx, group); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	send := postgresGroupSendCommand(group)
	send.AccountID = accountID
	// The Handoff recipient remains a current stable Group member, but was not
	// selected for this initial wave. Human Handoffs resolve current membership
	// instead of treating the turn's recipient snapshot as the Group roster.
	send.Envelope.Selection = conversation.GroupRecipientSelectionExplicit
	send.Envelope.Recipients = append([]conversation.GroupRecipient{}, send.Envelope.Recipients[:1]...)
	send.TargetIDs = append([]string{}, send.TargetIDs[:1]...)
	send.Envelope.CreatedAt = now.Add(-4 * time.Minute)
	send.Envelope.Deadline = now.Add(20 * time.Minute)
	groupTurn, err := store.SendGroupTurn(ctx, send)
	if err != nil {
		t.Fatalf("SendGroupTurn: %v", err)
	}
	sourceMessageID := strconv.FormatInt(groupTurn.Message.ID, 10)
	if !containsString(groupTurn.Envelope.RootDelegationGrant.ContextRecordIDs, "message:"+sourceMessageID) {
		t.Fatalf("Group root grant does not authorize frozen prompt: %+v", groupTurn.Envelope.RootDelegationGrant)
	}
	groupCommand := ledger.CreateHumanHandoffCommand{
		IdempotencyKey: "human-handoff:" + uuid.NewString(), AccountID: accountID,
		SourceConversationID: group.Conversation.ID, SourceMessageID: sourceMessageID,
		RecipientAgentID: recipient.Agent.ID, ContextMessageIDs: []string{sourceMessageID},
		RequestedResult: "Review the Group evidence.", ReplyToMessageID: sourceMessageID,
		HardDeadline: groupTurn.Envelope.Deadline, HandoffID: "handoff:" + uuid.NewString(),
		TargetID: "target:" + uuid.NewString(), RootDelegationGrantID: "grant:unused:" + uuid.NewString(),
		CreatedByID: "human:" + accountID, CreatedAt: now.Add(-3 * time.Minute),
	}
	groupHandoff, err := store.CreateHumanHandoff(ctx, groupCommand)
	if err != nil {
		t.Fatalf("CreateHumanHandoff Group: %v", err)
	}
	if groupHandoff.Handoff.GroupTurnID != groupTurn.Envelope.ID ||
		groupHandoff.Handoff.OutputConversationID != group.Conversation.ID ||
		groupHandoff.Handoff.RootDelegationGrant.ID != groupTurn.Envelope.RootDelegationGrant.ID ||
		groupHandoff.Target.ParticipantID != group.MemberBindings[1].ParticipantID || len(groupHandoff.Projections) != 0 {
		t.Fatalf("Group human Handoff = %+v", groupHandoff)
	}
	replay := groupCommand
	replay.HandoffID, replay.TargetID = "handoff:"+uuid.NewString(), "target:"+uuid.NewString()
	replay.RootDelegationGrantID, replay.CreatedByID = "grant:"+uuid.NewString(), "human:server-reformatted"
	replay.CreatedAt = groupCommand.CreatedAt.Add(time.Second)
	replayed, err := store.CreateHumanHandoff(ctx, replay)
	if err != nil || replayed.Handoff.ID != groupHandoff.Handoff.ID || replayed.Target.ID != groupHandoff.Target.ID {
		t.Fatalf("CreateHumanHandoff replay = %+v, %v", replayed, err)
	}
	conflict := replay
	conflict.RequestedResult = "A conflicting request."
	if _, err := store.CreateHumanHandoff(ctx, conflict); !errors.Is(err, ledger.ErrIdempotencyConflict) {
		t.Fatalf("CreateHumanHandoff conflict = %v", err)
	}
	queuedCancel := ledger.CancelHandoffCommand{IdempotencyKey: "cancel:" + uuid.NewString(), AccountID: accountID,
		HandoffID: groupHandoff.Handoff.ID, CanceledBy: "human:" + accountID, CanceledAt: now.Add(-2 * time.Minute)}
	canceledGroup, err := store.CancelHandoff(ctx, queuedCancel)
	if err != nil || canceledGroup.Handoff.State != conversation.HandoffCanceled ||
		canceledGroup.Target.State != conversation.TargetCanceled || canceledGroup.Cancellation == nil ||
		canceledGroup.Cancellation.State != ledger.HandoffCancellationCanceled ||
		canceledGroup.Cancellation.BindingRevisionID != groupHandoff.Target.BindingRevisionID {
		t.Fatalf("queued CancelHandoff = %+v, %v", canceledGroup, err)
	}

	directSend := postgresDirectAgentTurnCommand(sender, uuid.NewString())
	directSend.AccountID = accountID
	directSend.CreatedAt, directSend.HardDeadline = now.Add(-90*time.Second), now.Add(20*time.Minute)
	directTurn, err := store.SendAgentTurn(ctx, directSend)
	if err != nil {
		t.Fatalf("SendAgentTurn direct source: %v", err)
	}
	directMessageID := strconv.FormatInt(directTurn.Message.ID, 10)
	directCommand := ledger.CreateHumanHandoffCommand{
		IdempotencyKey: "human-handoff:" + uuid.NewString(), AccountID: accountID,
		SourceConversationID: sender.Home.ID, SourceMessageID: directMessageID,
		RecipientAgentID: recipient.Agent.ID, ContextMessageIDs: []string{directMessageID},
		RequestedResult: "Review the direct evidence.", ReplyToMessageID: directMessageID,
		HardDeadline: now.Add(15 * time.Minute), HandoffID: "handoff:" + uuid.NewString(),
		TargetID: "target:" + uuid.NewString(), RootDelegationGrantID: "grant:" + uuid.NewString(),
		CreatedByID: "human:" + accountID, CreatedAt: now.Add(-time.Minute),
	}
	directHandoff, err := store.CreateHumanHandoff(ctx, directCommand)
	if err != nil {
		t.Fatalf("CreateHumanHandoff direct: %v", err)
	}
	if directHandoff.Handoff.GroupTurnID != "" || directHandoff.Handoff.OutputConversationID != recipient.Home.ID ||
		directHandoff.Handoff.RootDelegationGrant.ID != directCommand.RootDelegationGrantID ||
		directHandoff.Target.BindingRevisionID != recipient.Binding.ID || len(directHandoff.Projections) != 1 {
		t.Fatalf("direct human Handoff = %+v", directHandoff)
	}

	capability := json.RawMessage(`{"frameworks":["codex"],"ready":true}`)
	capabilityHash := sha256.Sum256(capability)
	capabilityID := "capability:" + uuid.NewString()
	if _, err := store.RecordWorkerReadiness(ctx, controlapi.WorkerReadinessCommand{
		AccountID: accountID, WorkerID: workerID, MachineID: workerID,
		IdempotencyKey: "readiness:" + uuid.NewString(), CapabilityRevisionID: capabilityID,
		Revision: 1, CapabilityEvidence: capability, EvidenceDigest: hex.EncodeToString(capabilityHash[:]),
		ObservedAt: now.Add(-50 * time.Second),
	}); err != nil {
		t.Fatalf("RecordWorkerReadiness: %v", err)
	}
	claim := controlapi.WorkerClaimCommand{AccountID: accountID, WorkerID: workerID, MachineID: workerID,
		TargetID: directHandoff.Target.ID, ExecutionAttemptID: "attempt:" + uuid.NewString(),
		LeaseID: "lease:" + uuid.NewString(), IdempotencyKey: "claim:" + uuid.NewString(),
		CapabilityRevisionID: capabilityID, ClaimedAt: now.Add(-40 * time.Second), ExpiresAt: now.Add(80 * time.Second)}
	assignment, err := store.ClaimWorkerTarget(ctx, claim)
	if err != nil {
		t.Fatalf("ClaimWorkerTarget: %v", err)
	}
	start := ledger.StartHandoffCommand{AccountID: accountID, HandoffID: directHandoff.Handoff.ID,
		IdempotencyKey: "start:" + uuid.NewString(), AttemptID: claim.ExecutionAttemptID, LeaseID: claim.LeaseID,
		MachineID: workerID, FenceToken: strconv.FormatInt(assignment.FenceToken, 10),
		StartedAt: now.Add(-30 * time.Second), LeaseExpiresAt: claim.ExpiresAt}
	if _, err := store.StartHandoff(ctx, start); err != nil {
		t.Fatalf("StartHandoff: %v", err)
	}
	workingCancel := ledger.CancelHandoffCommand{IdempotencyKey: "cancel:" + uuid.NewString(), AccountID: accountID,
		HandoffID: directHandoff.Handoff.ID, CanceledBy: "human:" + accountID, CanceledAt: now}
	canceledDirect, err := store.CancelHandoff(ctx, workingCancel)
	if err != nil || canceledDirect.Handoff.State != conversation.HandoffCanceled ||
		canceledDirect.Target.State != conversation.TargetWorking || canceledDirect.Cancellation == nil ||
		canceledDirect.Cancellation.State != ledger.HandoffCancellationRequested ||
		canceledDirect.Cancellation.TargetID != directHandoff.Target.ID ||
		canceledDirect.Cancellation.BindingRevisionID != directHandoff.Target.BindingRevisionID {
		t.Fatalf("working CancelHandoff = %+v, %v", canceledDirect, err)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
