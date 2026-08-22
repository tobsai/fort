package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestPostgresAgentRepositoryAndNonceClaimerIntegration(t *testing.T) {
	databaseURL := os.Getenv("FORT_TEST_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("FORT_TEST_POSTGRES_URL is not set")
	}
	ctx := context.Background()
	accountID := uuid.NewString()
	workerID := "worker:" + uuid.NewString()

	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	if _, err := admin.Exec(ctx, `insert into fort_private.fort_account (
  account_id, normalized_email
) values ($1, $2)`, accountID, accountID+"@fort.test"); err != nil {
		admin.Close()
		t.Fatalf("seed Fort account: %v", err)
	}
	if _, err := admin.Exec(ctx, `insert into fort_private.worker (
  account_id, worker_id, machine_id, display_name, identity_key_digest,
  enrollment_token_hash, state, enrolled_at
) values ($1, $2, $3, 'Integration Worker', $4, $5, 'enrolled', $6)`,
		accountID, workerID, workerID, strings.Repeat("b", 64), strings.Repeat("c", 64),
		time.Now().UTC()); err != nil {
		admin.Close()
		t.Fatalf("seed worker: %v", err)
	}
	if _, err := admin.Exec(ctx, `insert into fort_private.ledger_event (
  account_id, aggregate_kind, aggregate_id, event_type, event_metadata, created_at
) values ($1::uuid, 'account', ($1::uuid)::text, 'account.created', '{"source":"integration"}'::jsonb, $2)`,
		accountID, time.Now().UTC()); err != nil {
		admin.Close()
		t.Fatalf("seed ledger event: %v", err)
	}
	admin.Close()

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
	var rowsWithoutAccount int
	if err := runtimePool.QueryRow(ctx, `select count(*) from fort_private.stable_agent`).Scan(&rowsWithoutAccount); err != nil {
		runtimePool.Close()
		t.Fatalf("read without transaction account: %v", err)
	}
	if rowsWithoutAccount != 0 {
		runtimePool.Close()
		t.Fatalf("runtime role saw %d Agents without transaction account", rowsWithoutAccount)
	}
	store, err := New(runtimePool, accountID)
	if err != nil {
		runtimePool.Close()
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	command := integrationAgentCommand(accountID, workerID)
	created, err := store.CreateAgent(ctx, command)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if created.Agent.ID != command.Agent.ID || created.Home.ID != command.Home.ID {
		t.Fatalf("created Agent = %+v", created)
	}
	if _, err := store.CreateAgent(ctx, command); err != nil {
		t.Fatalf("idempotent CreateAgent replay: %v", err)
	}
	retrieved, err := store.GetAgent(ctx, accountID, command.Agent.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if retrieved.Agent.ID != command.Agent.ID || retrieved.Binding.ID != command.Binding.ID || retrieved.Participant.ID != command.Participant.ID {
		t.Fatalf("retrieved Agent = %+v", retrieved)
	}
	listed, err := store.ListAgents(ctx, accountID, conversation.AgentOpen)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(listed) != 1 || listed[0].Agent.ID != command.Agent.ID {
		t.Fatalf("listed Agents = %+v", listed)
	}
	page, err := store.ReadCursorPage(ctx, accountID, "cursor-0")
	if err != nil {
		t.Fatalf("ReadCursorPage: %v", err)
	}
	if len(page.Events) != 1 || page.Events[0].Kind != "account.created" || page.NextCursor == "cursor-0" {
		t.Fatalf("cursor page = %+v", page)
	}
	expiresAt := time.Now().UTC().Add(time.Minute)
	nonce := uuid.NewString()
	claimed, err := store.Claim(ctx, accountID, "integration-key", nonce, expiresAt)
	if err != nil || !claimed {
		t.Fatalf("first nonce claim = %v, %v", claimed, err)
	}
	claimed, err = store.Claim(ctx, accountID, "integration-key", nonce, expiresAt)
	if err != nil || claimed {
		t.Fatalf("replayed nonce claim = %v, %v", claimed, err)
	}
}

func integrationAgentCommand(accountID, workerID string) ledger.CreateAgentCommand {
	command := postgresAgentCommand()
	suffix := uuid.NewString()
	command.Agent.AccountID = accountID
	command.ExecutionSource.AccountID = accountID
	command.Agent.ID = "agent:" + suffix
	command.Profile.ID = "profile:" + suffix + ":1"
	command.Profile.AgentID = command.Agent.ID
	command.Behavior.ID = "behavior:" + suffix + ":1"
	command.Behavior.AgentID = command.Agent.ID
	command.Binding.ID = "binding:" + suffix + ":1"
	command.Binding.AgentID = command.Agent.ID
	command.Binding.BehaviorRevisionID = command.Behavior.ID
	command.Binding.ExecutionSourceID = "source:" + suffix
	command.Binding.SourceAgentID = "source-agent:" + suffix
	command.Binding.ComputerID = workerID
	command.ExecutionSource.ID = command.Binding.ExecutionSourceID
	command.SourceAgent.ID = command.Binding.SourceAgentID
	command.SourceAgent.ExecutionSourceID = command.ExecutionSource.ID
	command.Agent.CurrentProfileRevisionID = command.Profile.ID
	command.Agent.CurrentBehaviorRevisionID = command.Behavior.ID
	command.Agent.CurrentBindingRevisionID = command.Binding.ID
	command.Agent.CanonicalConversationID = "conversation:" + suffix + ":home"
	command.Home.ID = command.Agent.CanonicalConversationID
	command.Participant.ID = "participant:" + suffix + ":1"
	command.Participant.ConversationID = command.Home.ID
	command.Participant.Machine = workerID
	command.Link.AgentID = command.Agent.ID
	command.Link.ConversationID = command.Home.ID
	command.IdempotencyKey = "create:" + suffix
	return command
}
