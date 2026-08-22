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
	"github.com/tobsai/fort/cloud/controlapi"
)

func TestPostgresScheduleTickIntegration(t *testing.T) {
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
	defer admin.Close()
	now := time.Now().UTC().Truncate(time.Minute)
	if _, err := admin.Exec(ctx, `insert into fort_private.fort_account(account_id, normalized_email)
values ($1, $2)`, accountID, accountID+"@cron.fort.test"); err != nil {
		t.Fatalf("seed Fort account: %v", err)
	}
	if _, err := admin.Exec(ctx, `insert into fort_private.worker(
  account_id, worker_id, machine_id, display_name, identity_key_digest,
  enrollment_token_hash, state, enrolled_at
) values ($1, $2, $2, 'Cron Integration Worker', $3, $4, 'enrolled', $5)`,
		accountID, workerID, strings.Repeat("b", 64), strings.Repeat("c", 64), now); err != nil {
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
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = store.Close() }()

	command := integrationAgentCommand(accountID, workerID)
	if _, err := store.CreateAgent(ctx, command); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	routineID := "routine:" + uuid.NewString()
	revisionID := routineID + ":revision:1"
	seed, err := admin.Begin(ctx)
	if err != nil {
		t.Fatalf("begin Routine seed: %v", err)
	}
	defer func() { _ = seed.Rollback(ctx) }()
	if _, err := seed.Exec(ctx, `insert into fort_private.routine(
  account_id, routine_id, agent_id, authority, state, current_revision_id, created_at, updated_at
) values ($1, $2, $3, 'fort_cloud', 'active', $4, $5, $5)`,
		accountID, routineID, command.Agent.ID, revisionID, now.Add(-time.Minute)); err != nil {
		t.Fatalf("seed Routine: %v", err)
	}
	if _, err := seed.Exec(ctx, `insert into fort_private.routine_revision(
  account_id, routine_revision_id, routine_id, agent_id, revision,
  behavior_revision_id, binding_revision_id, trigger_kind, schedule_expression,
  timezone, next_occurrence_at, input_source, freshness_policy, expected_result,
  result_conversation_id, approval_policy, missing_input_policy, retry_policy,
  catch_up_policy, lateness_policy, binding_compatibility, command_digest, created_at
) values (
  $1, $2, $3, $4, 1, $5, $6, 'cron', '0 * * * * *', 'UTC', $7,
	  '{"value":"none"}'::jsonb, '{"seconds":3600}'::jsonb, 'one normalized message', $8,
	  '{"value":"none"}'::jsonb, 'needs_you', '{"value":"once"}'::jsonb,
	  '{"value":"skip"}'::jsonb, '{"value":"within_90s"}'::jsonb,
	  jsonb_build_object('behavior_revision_id',$5::text,'binding_revision_id',$6::text), $9, $10
)`, accountID, revisionID, routineID, command.Agent.ID, command.Behavior.ID,
		command.Binding.ID, now, command.Home.ID, strings.Repeat("d", 64), now.Add(-time.Minute)); err != nil {
		t.Fatalf("seed Routine revision: %v", err)
	}
	if err := seed.Commit(ctx); err != nil {
		t.Fatalf("commit Routine seed: %v", err)
	}

	clock := now
	tickNumber := 0
	service := controlapi.ScheduleTickService{
		Repository: store,
		Clock:      func() time.Time { return clock },
		TickIDs: func() string {
			tickNumber++
			return "integration-tick-" + time.Unix(int64(tickNumber), 0).UTC().Format("150405")
		},
	}
	first, err := service.Tick(ctx, accountID, "fort-cloud")
	if err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	if first.OccurrencesChanged != 2 {
		t.Fatalf("first Tick changed %d occurrences, want due and look-ahead", first.OccurrencesChanged)
	}
	clock = now.Add(time.Second)
	second, err := service.Tick(ctx, accountID, "fort-cloud")
	if err != nil {
		t.Fatalf("replay Tick: %v", err)
	}
	if second.OccurrencesChanged != 0 {
		t.Fatalf("replay Tick changed %d occurrences, want zero", second.OccurrencesChanged)
	}
	clock = now.Add(time.Minute)
	dueTick, err := service.Tick(ctx, accountID, "fort-cloud")
	if err != nil {
		t.Fatalf("due Tick: %v", err)
	}
	if dueTick.OccurrencesChanged != 2 {
		t.Fatalf("due Tick changed %d occurrences, want one queue transition and one look-ahead", dueTick.OccurrencesChanged)
	}
	clock = now.Add(3 * time.Minute)
	third, err := service.Tick(ctx, accountID, "fort-cloud")
	if err != nil {
		t.Fatalf("late Tick: %v", err)
	}
	if third.OccurrencesChanged != 3 || third.LateRoutineRunsExpired != 2 {
		t.Fatalf("late Tick changed/expired = %d/%d, want three schedule changes and two late run expirations",
			third.OccurrencesChanged, third.LateRoutineRunsExpired)
	}

	var count, queued, scheduled, missed, eventCount int
	if err := admin.QueryRow(ctx, `select count(*),
  count(*) filter (where state = 'queued'),
  count(*) filter (where state = 'scheduled'),
  count(*) filter (where state = 'missed_needs_attention')
from fort_private.routine_occurrence
where account_id = $1 and routine_id = $2`, accountID, routineID).Scan(&count, &queued, &scheduled, &missed); err != nil {
		t.Fatalf("read occurrences: %v", err)
	}
	if count != 5 || queued != 2 || scheduled != 1 || missed != 2 {
		t.Fatalf("occurrence count/queued/scheduled/missed = %d/%d/%d/%d, want 5/2/1/2", count, queued, scheduled, missed)
	}
	if err := admin.QueryRow(ctx, `select count(*) from fort_private.ledger_event
where account_id = $1 and aggregate_kind = 'routine_occurrence'`, accountID).Scan(&eventCount); err != nil {
		t.Fatalf("read occurrence events: %v", err)
	}
	if eventCount != 9 {
		t.Fatalf("occurrence event count = %d, want 9", eventCount)
	}
	var runs, turns, targets, needsYouRuns int
	if err := admin.QueryRow(ctx, `select
  (select count(*) from fort_private.routine_run where account_id=$1 and routine_id=$2),
  (select count(*) from fort_private.conversation_turn where account_id=$1 and kind='routine'),
  (select count(*) from fort_private.conversation_target where account_id=$1 and target_kind='routine'),
  (select count(*) from fort_private.routine_run where account_id=$1 and routine_id=$2 and state='needs_you')`,
		accountID, routineID).Scan(&runs, &turns, &targets, &needsYouRuns); err != nil {
		t.Fatalf("read Routine execution rows: %v", err)
	}
	if runs != 4 || turns != 4 || targets != 4 || needsYouRuns != 2 {
		t.Fatalf("run/turn/target/needs-you counts = %d/%d/%d/%d, want 4/4/4/2",
			runs, turns, targets, needsYouRuns)
	}
}
