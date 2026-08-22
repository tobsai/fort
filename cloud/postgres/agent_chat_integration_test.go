package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tobsai/fort/core/ledger"
)

func TestPostgresAgentDirectChatIntegration(t *testing.T) {
	databaseURL := integrationDatabaseURL(t)
	ctx := context.Background()
	accountID := uuid.NewString()
	workerID := "worker:" + uuid.NewString()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	defer admin.Close()
	now := time.Now().UTC().Truncate(time.Millisecond)
	if _, err := admin.Exec(ctx, `insert into fort_private.fort_account(account_id,normalized_email)
values($1,$2)`, accountID, accountID+"@direct-chat.fort.test"); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := admin.Exec(ctx, `insert into fort_private.worker(
  account_id,worker_id,machine_id,display_name,identity_key_digest,enrollment_token_hash,state,enrolled_at
) values($1,$2,$2,'Direct Chat Worker',$3,$4,'enrolled',$5)`, accountID, workerID,
		shaString("direct-worker"), shaString("direct-token"), now); err != nil {
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
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("open runtime pool: %v", err)
	}
	store, err := NewWithKeyRing(pool, accountID, collaborationTestKeyRing())
	if err != nil {
		pool.Close()
		t.Fatalf("NewWithKeyRing: %v", err)
	}
	defer func() { _ = store.Close() }()
	agent := integrationAgentCommand(accountID, workerID)
	if _, err := store.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	suffix := uuid.NewString()
	command := postgresDirectAgentTurnCommand(agent, suffix)
	command.AccountID = accountID
	command.CreatedAt, command.HardDeadline = now.Add(time.Second), now.Add(10*time.Minute)
	created, err := store.SendAgentTurn(ctx, command)
	if err != nil {
		t.Fatalf("SendAgentTurn: %v", err)
	}
	if !created.Created || created.Message.Body != command.Body || created.Target.BindingRevisionID != agent.Binding.ID {
		t.Fatalf("created direct turn = %+v", created)
	}
	replay := command
	replay.TurnID, replay.ContextManifestID, replay.DelegationGrantID = "turn:"+uuid.NewString(), "context:"+uuid.NewString(), "grant:"+uuid.NewString()
	replay.TargetID, replay.RunID, replay.CreatedAt = "target:"+uuid.NewString(), "run:"+uuid.NewString(), command.CreatedAt.Add(time.Second)
	replayed, err := store.SendAgentTurn(ctx, replay)
	if err != nil {
		t.Fatalf("SendAgentTurn replay: %v", err)
	}
	if replayed.Created || replayed.Turn.ID != created.Turn.ID || replayed.Message.ID != created.Message.ID ||
		replayed.Target.ID != created.Target.ID || replayed.Message.Body != command.Body {
		t.Fatalf("replayed direct turn = %+v", replayed)
	}
	projection, err := store.ReadAgentConversation(ctx, accountID, agent.Agent.ID, agent.Home.ID)
	if err != nil {
		t.Fatalf("ReadAgentConversation: %v", err)
	}
	if len(projection.Messages) != 1 || len(projection.Turns) != 1 || len(projection.Targets) != 1 ||
		projection.Messages[0].Body != command.Body || projection.Targets[0].BindingRevisionID != agent.Binding.ID {
		t.Fatalf("direct projection = %+v", projection)
	}
	canceled, err := store.CancelAgentTarget(ctx, ledger.CancelAgentTargetCommand{
		IdempotencyKey: "cancel:" + suffix, AccountID: accountID, AgentID: agent.Agent.ID,
		ConversationID: agent.Home.ID, TargetID: created.Target.ID, CanceledBy: "human:toby",
		CanceledAt: command.CreatedAt.Add(2 * time.Second),
	})
	if err != nil || canceled.State != "canceled" {
		t.Fatalf("CancelAgentTarget = %+v, %v", canceled, err)
	}
	retried, err := store.RetryAgentTarget(ctx, ledger.RetryAgentTargetCommand{
		IdempotencyKey: "retry:" + suffix, AccountID: accountID, AgentID: agent.Agent.ID,
		ConversationID: agent.Home.ID, TargetID: created.Target.ID, RetriedBy: "human:toby",
		RetriedAt: command.CreatedAt.Add(3 * time.Second),
	})
	if err != nil || retried.State != "queued" || retried.BindingRevisionID != created.Target.BindingRevisionID {
		t.Fatalf("RetryAgentTarget = %+v, %v", retried, err)
	}
	drift, err := store.ObserveExecutionSourceConfig(ctx, ledger.ObserveExecutionSourceConfigCommand{
		IdempotencyKey: "observe-drift:" + suffix, ObservationID: "source-observation:" + uuid.NewString(),
		AccountID: accountID, ExecutionSourceID: agent.Binding.ExecutionSourceID,
		SourceConfigDigest: strings.Repeat("b", 64), ObservedBy: workerID,
		ObservedAt: command.CreatedAt.Add(4 * time.Second),
	})
	if err != nil || drift.SourceConfigDigest == agent.Binding.SourceConfigDigest {
		t.Fatalf("ObserveExecutionSourceConfig drift = %+v, %v", drift, err)
	}
	driftedSend := postgresDirectAgentTurnCommand(agent, "drift-"+suffix)
	driftedSend.AccountID = accountID
	driftedSend.CreatedAt, driftedSend.HardDeadline = command.CreatedAt.Add(5*time.Second), command.HardDeadline
	if _, err := store.SendAgentTurn(ctx, driftedSend); !errors.Is(err, ledger.ErrSourceDrift) {
		t.Fatalf("SendAgentTurn after later observed drift = %v, want source drift", err)
	}
	projection, err = store.ReadAgentConversation(ctx, accountID, agent.Agent.ID, agent.Home.ID)
	if err != nil || len(projection.Messages) != 1 || len(projection.Turns) != 1 || len(projection.Targets) != 1 {
		t.Fatalf("drift dispatch changed projection = %+v, %v", projection, err)
	}
}

func integrationDatabaseURL(t *testing.T) string {
	t.Helper()
	value := os.Getenv("FORT_TEST_POSTGRES_URL")
	if value == "" {
		t.Skip("FORT_TEST_POSTGRES_URL is not set")
	}
	return value
}

func shaString(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
