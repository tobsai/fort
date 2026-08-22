package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
	coreworker "github.com/tobsai/fort/core/worker"
)

func TestPostgresRoutineRepositoryContract(t *testing.T) {
	t.Parallel()
	var repository ledger.RoutineRepository = (*Store)(nil)
	_ = repository
}

func TestPostgresRoutineTriggerMappingKeepsEventTruthful(t *testing.T) {
	t.Parallel()

	trigger, schedule, timezone := postgresRoutineTrigger(conversation.RoutineTriggerEvent, "", "")
	if trigger != "event" || schedule != postgresRoutineNotApplicable || timezone != postgresRoutineNotApplicable {
		t.Fatalf("event mapping = %q, %q, %q", trigger, schedule, timezone)
	}
	domainTrigger, domainSchedule, domainTimezone, err := domainRoutineTrigger(trigger, schedule, timezone)
	if err != nil {
		t.Fatalf("domainRoutineTrigger: %v", err)
	}
	if domainTrigger != conversation.RoutineTriggerEvent || domainSchedule != "" || domainTimezone != "" {
		t.Fatalf("event round trip = %q, %q, %q", domainTrigger, domainSchedule, domainTimezone)
	}

	trigger, schedule, timezone = postgresRoutineTrigger(conversation.RoutineTriggerSchedule, "0 9 * * *", "America/Chicago")
	if trigger != "cron" || schedule != "0 9 * * *" || timezone != "America/Chicago" {
		t.Fatalf("schedule mapping = %q, %q, %q", trigger, schedule, timezone)
	}
	if _, _, _, err := domainRoutineTrigger("once", schedule, timezone); err == nil {
		t.Fatal("first-release reader accepted unsupported once trigger")
	}
}

func TestRoutineStartPoliciesFailClosedBeforeProviderWork(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 16, 0, 0, 0, time.UTC)
	record := ledger.RoutineRecord{
		Routine: conversation.Routine{ID: "routine:daily", AccountID: testAccountID, AgentID: "agent:researcher",
			CurrentRevisionID: "routine-revision:daily:1", State: conversation.RoutineActive, CreatedAt: now.Add(-time.Hour)},
		CurrentRevision: conversation.RoutineRevision{ID: "routine-revision:daily:1", RoutineID: "routine:daily",
			Revision: 1, AgentID: "agent:researcher", BehaviorRevisionID: "behavior:researcher:1",
			BindingRevisionID: "binding:researcher:1", Authority: conversation.RoutineAuthorityFortCloud,
			Trigger: conversation.RoutineTriggerSchedule, Schedule: "0 * * * * *", Timezone: "UTC",
			NextOccurrence: now, InputSource: "none", FreshnessSeconds: 3600, ExpectedResult: "Daily brief",
			ResultConversationID: "conversation:results", ApprovalBoundary: conversation.RoutineApprovalNone,
			MissingInputBehavior: "needs_you", RetryPolicy: "once", CatchUpPolicy: "skip",
			LatenessPolicy: conversation.RoutineLatenessWithin90Seconds, CreatedAt: now.Add(-time.Hour)},
	}
	command := ledger.EnqueueRoutineOccurrenceCommand{AccountID: testAccountID, RoutineID: record.Routine.ID,
		RoutineRevisionID: record.CurrentRevision.ID, OccurrenceID: "occurrence:daily", RunID: "run:daily",
		Kind: conversation.RoutineRunScheduled, ScheduledFor: now, IdempotencyKey: "routine:daily@now",
		ApprovalEvidenceID: "approval-evaluation:daily", CreatedAt: now.Add(20 * time.Second)}

	queued, err := evaluatePostgresRoutineStart(context.Background(), &fakeTransaction{}, nil,
		testAccountID, record, command, "queued")
	if err != nil || queued.RunState != conversation.RoutineRunQueued || queued.TargetState != "queued" {
		t.Fatalf("fresh policy decision = %+v/%v", queued, err)
	}

	late := command
	late.CreatedAt = now.Add(91 * time.Second)
	decision, err := evaluatePostgresRoutineStart(context.Background(), &fakeTransaction{}, nil,
		testAccountID, record, late, "queued")
	if err != nil || decision.RunState != conversation.RoutineRunNeedsYou || decision.FailureCode != "routine_late" {
		t.Fatalf("late policy decision = %+v/%v", decision, err)
	}

	approval := record
	approval.CurrentRevision.ApprovalBoundary = conversation.RoutineApprovalBeforeExternalSideEffect
	decision, err = evaluatePostgresRoutineStart(context.Background(), &fakeTransaction{}, nil,
		testAccountID, approval, command, "queued")
	if err != nil || decision.RunState != conversation.RoutineRunNeedsYou || decision.NextAction != "approve_routine_run" {
		t.Fatalf("approval policy decision = %+v/%v", decision, err)
	}
}

func TestRoutineMissingOrStaleInputUsesPersistedBehavior(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 16, 0, 0, 0, time.UTC)
	record := ledger.RoutineRecord{
		Routine: conversation.Routine{ID: "routine:daily", AccountID: testAccountID, AgentID: "agent:researcher",
			CurrentRevisionID: "routine-revision:daily:1", State: conversation.RoutineActive, CreatedAt: now.Add(-time.Hour)},
		CurrentRevision: conversation.RoutineRevision{ID: "routine-revision:daily:1", RoutineID: "routine:daily",
			Revision: 1, AgentID: "agent:researcher", BehaviorRevisionID: "behavior:researcher:1",
			BindingRevisionID: "binding:researcher:1", Authority: conversation.RoutineAuthorityFortCloud,
			Trigger: conversation.RoutineTriggerSchedule, Schedule: "0 * * * * *", Timezone: "UTC",
			NextOccurrence: now, InputSource: "fort:conversation:conversation:source", FreshnessSeconds: 3600,
			ExpectedResult: "Daily brief", ResultConversationID: "conversation:results",
			ApprovalBoundary: conversation.RoutineApprovalNone, MissingInputBehavior: "needs_you",
			RetryPolicy: "once", CatchUpPolicy: "skip", LatenessPolicy: conversation.RoutineLatenessWithin90Seconds,
			CreatedAt: now.Add(-time.Hour)},
	}
	command := ledger.EnqueueRoutineOccurrenceCommand{AccountID: testAccountID, RoutineID: record.Routine.ID,
		RoutineRevisionID: record.CurrentRevision.ID, OccurrenceID: "occurrence:daily", RunID: "run:daily",
		Kind: conversation.RoutineRunScheduled, ScheduledFor: now, IdempotencyKey: "routine:daily@now",
		ApprovalEvidenceID: "approval-evaluation:daily", CreatedAt: now.Add(20 * time.Second)}

	for behavior, wantState := range map[string]conversation.RoutineRunState{
		"needs_you": conversation.RoutineRunNeedsYou,
		"skip":      conversation.RoutineRunFailed,
		"fail":      conversation.RoutineRunFailed,
	} {
		candidate := record
		candidate.CurrentRevision.MissingInputBehavior = behavior
		tx := &fakeTransaction{queryRowHook: func(string, []any) row { return fakeRow{err: pgx.ErrNoRows} }}
		decision, err := evaluatePostgresRoutineStart(context.Background(), tx, nil,
			testAccountID, candidate, command, "queued")
		if err != nil || decision.RunState != wantState || decision.FailureCode == "" || decision.TargetState == "queued" {
			t.Fatalf("missing input %q decision = %+v/%v", behavior, decision, err)
		}
	}
}

func TestPostgresRoutineParentRejectsConversationNotLinkedToOwningAgent(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 14, 0, 0, 0, time.UTC)
	tx := &fakeTransaction{}
	tx.queryRowHook = func(statement string, arguments []any) row {
		if !strings.Contains(statement, "join fort_private.agent_conversation as relation") ||
			len(arguments) != 3 || arguments[0] != testAccountID || arguments[1] != "agent:researcher" ||
			arguments[2] != "conversation:builder-home" {
			return fakeRow{err: errors.New("Routine parent query was not scoped through the owning Agent")}
		}
		return fakeRow{err: pgx.ErrNoRows}
	}
	command := ledger.CreateRoutineCommand{
		IdempotencyKey: "routine:create:foreign-result",
		Routine: conversation.Routine{ID: "routine:daily", AccountID: testAccountID, AgentID: "agent:researcher",
			CurrentRevisionID: "routine-revision:daily:1", State: conversation.RoutineActive, CreatedAt: now},
		Revision: conversation.RoutineRevision{ID: "routine-revision:daily:1", RoutineID: "routine:daily", Revision: 1,
			AgentID: "agent:researcher", BehaviorRevisionID: "behavior:researcher:1",
			BindingRevisionID: "binding:researcher:1", Authority: conversation.RoutineAuthorityFortCloud,
			Trigger: conversation.RoutineTriggerSchedule, Schedule: "0 0 9 * * *", Timezone: "America/Chicago",
			NextOccurrence: now.Add(time.Hour), InputSource: "agent-home", FreshnessSeconds: 3600,
			ExpectedResult: "Daily brief", ResultConversationID: "conversation:builder-home",
			ApprovalBoundary: "none", MissingInputBehavior: "needs_you", RetryPolicy: "none",
			CatchUpPolicy: "skip", LatenessPolicy: "within_90s", CreatedAt: now},
	}
	if err := validatePostgresRoutineParents(context.Background(), tx, testAccountID, command); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("validatePostgresRoutineParents error = %v, want %v", err, ledger.ErrNotFound)
	}
}

func TestEnqueuePausedRoutineDoesNotConsumeIdempotencyKey(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC)
	tx := &fakeTransaction{}
	tx.queryRowHook = func(statement string, _ []any) row {
		switch {
		case strings.Contains(statement, "from fort_private.idempotency_record"):
			return fakeRow{err: pgx.ErrNoRows}
		case strings.Contains(statement, "from fort_private.routine where"):
			return fakeRow{values: []any{
				"routine:daily", testAccountID, "agent:researcher", "routine-revision:daily:1",
				"paused_needs_revalidation", now.Add(-time.Hour),
			}}
		case strings.Contains(statement, "from fort_private.routine_revision"):
			return fakeRow{values: []any{
				"routine-revision:daily:1", "routine:daily", 1, "agent:researcher",
				"behavior:researcher:1", "binding:researcher:1", "cron", "0 9 * * *",
				"America/Chicago", sql.NullTime{Time: now.Add(time.Hour), Valid: true},
				`{"value":"agent-home"}`, `{"seconds":3600}`, "Daily brief", "conversation:results",
				`{"value":"none"}`, "needs_you", `{"value":"none"}`, `{"value":"skip"}`,
				`{"value":"skip"}`, `{"behavior_revision_id":"behavior:researcher:1","binding_revision_id":"binding:researcher:1"}`,
				now.Add(-time.Hour),
			}}
		case strings.Contains(statement, "from fort_private.routine_import_receipt"):
			return fakeRow{err: pgx.ErrNoRows}
		default:
			return fakeRow{err: errors.New("unexpected Routine query")}
		}
	}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID,
		collaborationTestKeyRing())
	if err != nil {
		t.Fatalf("newStoreWithKeyRing: %v", err)
	}
	_, err = store.EnqueueRoutineOccurrence(context.Background(), ledger.EnqueueRoutineOccurrenceCommand{
		AccountID: testAccountID, RoutineID: "routine:daily", RoutineRevisionID: "routine-revision:daily:1",
		OccurrenceID: "occurrence:daily:test:1", RunID: "run:daily:test:1", Kind: conversation.RoutineRunTest,
		ScheduledFor: now, IdempotencyKey: "routine:test:daily:1", ApprovalEvidenceID: "approval:daily:test:1",
		CreatedAt: now,
	})
	if !errors.Is(err, ledger.ErrRoutineNeedsRevalidation) {
		t.Fatalf("EnqueueRoutineOccurrence error = %v, want %v", err, ledger.ErrRoutineNeedsRevalidation)
	}
	for _, statement := range tx.execs {
		if strings.Contains(statement.sql, "fort_private.idempotency_record") {
			t.Fatalf("paused Routine consumed idempotency key: %q", statement.sql)
		}
	}
}

func TestAdvanceRoutineWorkingPinsExecutionAttemptInRunRow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 16, 0, 0, 0, time.UTC)
	expiresAt := now.Add(5 * time.Minute)
	runReads := 0
	tx := &fakeTransaction{}
	tx.queryRowHook = func(statement string, _ []any) row {
		switch {
		case strings.Contains(statement, "from fort_private.routine_run as run") && strings.Contains(statement, "left join fort_private.conversation_message"):
			runReads++
			if runReads > 1 {
				return fakeRow{err: errors.New("stop after Routine update")}
			}
			return fakeRow{values: []any{
				"run:daily:test:1", "occurrence:daily:test:1", "routine:daily", "routine-revision:daily:1",
				"agent:researcher", "behavior:researcher:1", "binding:researcher:1", "conversation:results",
				string(conversation.RoutineRunQueued), sql.NullString{}, sql.NullString{}, now.Add(-time.Minute),
				"occurrence:daily:test:1", testAccountID, "routine:daily", "routine-revision:daily:1", true,
				"queued", now.Add(-time.Minute), "routine:test:daily:1", now.Add(-time.Minute), now.Add(-time.Minute),
				sql.NullInt64{}, []byte{}, sql.NullString{}, []byte{}, sql.NullString{}, sql.NullInt64{},
			}}
		case strings.Contains(statement, "join fort_private.worker_lease as lease"):
			return fakeRow{values: []any{"active", expiresAt}}
		default:
			return fakeRow{err: errors.New("unexpected Routine working query")}
		}
	}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID,
		collaborationTestKeyRing())
	if err != nil {
		t.Fatalf("newStoreWithKeyRing: %v", err)
	}
	_, err = store.AdvanceRoutineRun(context.Background(), ledger.AdvanceRoutineRunCommand{
		AccountID: testAccountID, RunID: "run:daily:test:1", IdempotencyKey: "routine:run:working:1",
		FromState: conversation.RoutineRunQueued, ToState: conversation.RoutineRunWorking,
		AttemptID: "attempt:daily:test:1", LeaseID: "lease:daily:test:1", LeaseExpiresAt: expiresAt,
		Activity: "worker lease claimed", OccurredAt: now,
	})
	if err == nil || !strings.Contains(err.Error(), "stop after Routine update") {
		t.Fatalf("AdvanceRoutineRun error = %v, want final-read stop", err)
	}
	for _, statement := range tx.execs {
		if strings.Contains(statement.sql, "update fort_private.routine_run") {
			if !strings.Contains(statement.sql, "execution_attempt_id") {
				t.Fatalf("Routine working update did not persist its exact execution attempt: %q", statement.sql)
			}
			return
		}
	}
	t.Fatal("AdvanceRoutineRun did not update the Routine run")
}

func TestWorkerHeartbeatMarksExactRoutineRunWorkingOnce(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 17, 0, 0, 0, time.UTC)
	expiresAt := now.Add(5 * time.Minute)
	tx := &fakeTransaction{queryRowHook: func(statement string, arguments []any) row {
		if !strings.Contains(statement, "from fort_private.routine_run as run") ||
			!strings.Contains(statement, "for update of run,occurrence") {
			return fakeRow{err: errors.New("unexpected Routine worker heartbeat query")}
		}
		return fakeRow{values: []any{
			"run:daily:1", "occurrence:daily:1", string(conversation.RoutineRunQueued), "queued",
			sql.NullString{}, sql.NullString{String: "attempt:daily:1", Valid: true},
			sql.NullString{String: "lease:daily:1", Valid: true}, sql.NullString{String: "active", Valid: true},
			sql.NullTime{Time: expiresAt, Valid: true},
		}}
	}}
	err := markPostgresRoutineRunWorking(context.Background(), tx, testAccountID,
		"target:routine:daily:1", "attempt:daily:1", "lease:daily:1", expiresAt, now)
	if err != nil {
		t.Fatalf("markPostgresRoutineRunWorking: %v", err)
	}

	var runUpdates, occurrenceUpdates, activities int
	for _, statement := range tx.execs {
		switch {
		case strings.Contains(statement.sql, "update fort_private.routine_run"):
			runUpdates++
			if !containsArgument(statement.args, "attempt:daily:1") || !containsArgument(statement.args, "run:daily:1") {
				t.Fatalf("Routine working update arguments = %#v", statement.args)
			}
		case strings.Contains(statement.sql, "update fort_private.routine_occurrence"):
			occurrenceUpdates++
		case strings.Contains(statement.sql, "insert into fort_private.ledger_event"):
			activities++
			if !containsArgument(statement.args, "run:daily:1") || !containsArgument(statement.args, "target:routine:daily:1") {
				t.Fatalf("Routine working activity arguments = %#v", statement.args)
			}
		}
	}
	if runUpdates != 1 || occurrenceUpdates != 1 || activities != 1 {
		t.Fatalf("Routine working writes = run %d, occurrence %d, activity %d", runUpdates, occurrenceUpdates, activities)
	}
}

func TestWorkerHeartbeatRejectsRoutineLeaseExpiryMismatch(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 17, 0, 0, 0, time.UTC)
	storedExpiry := now.Add(5 * time.Minute)
	tx := &fakeTransaction{queryRowHook: func(string, []any) row {
		return fakeRow{values: []any{
			"run:daily:1", "occurrence:daily:1", string(conversation.RoutineRunQueued), "queued",
			sql.NullString{}, sql.NullString{String: "attempt:daily:1", Valid: true},
			sql.NullString{String: "lease:daily:1", Valid: true}, sql.NullString{String: "active", Valid: true},
			sql.NullTime{Time: storedExpiry, Valid: true},
		}}
	}}
	err := markPostgresRoutineRunWorking(context.Background(), tx, testAccountID,
		"target:routine:daily:1", "attempt:daily:1", "lease:daily:1", storedExpiry.Add(time.Second), now)
	if !errors.Is(err, controlapi.ErrWorkerStaleLease) {
		t.Fatalf("markPostgresRoutineRunWorking error = %v, want stale lease", err)
	}
	if len(tx.execs) != 0 {
		t.Fatalf("lease mismatch wrote %d statements", len(tx.execs))
	}
}

func TestWorkerTerminalCompletesRoutineWithOneAuthoritativeResult(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 17, 5, 0, 0, time.UTC)
	resultMessages := 0
	tx := &fakeTransaction{queryRowHook: func(statement string, arguments []any) row {
		switch {
		case strings.Contains(statement, "from fort_private.routine_run as run") &&
			strings.Contains(statement, "for update of run,occurrence"):
			return fakeRow{values: []any{
				"run:daily:1", "occurrence:daily:1", "conversation:results", "agent:researcher",
				string(conversation.RoutineRunWorking), "working",
				sql.NullString{String: "attempt:daily:1", Valid: true},
				sql.NullString{String: "attempt:daily:1", Valid: true},
				sql.NullString{String: "lease:daily:1", Valid: true}, sql.NullString{String: "active", Valid: true},
				int64(0),
			}}
		case strings.Contains(statement, "insert into fort_private.conversation_message"):
			resultMessages++
			if !containsArgument(arguments, "run:daily:1") ||
				!containsArgument(arguments, "conversation:results") {
				return fakeRow{err: fmt.Errorf("Routine result arguments = %#v", arguments)}
			}
			return fakeRow{values: []any{int64(81)}}
		default:
			return fakeRow{err: errors.New("unexpected Routine worker terminal query")}
		}
	}}
	store, err := newStoreWithKeyRing(&fakeDatabase{}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatalf("newStoreWithKeyRing: %v", err)
	}
	err = store.commitPostgresRoutineRunTerminal(context.Background(), tx, testAccountID,
		"target:routine:daily:1", "attempt:daily:1", "lease:daily:1", coreworker.TerminalCompleted,
		"The normalized daily brief.", now)
	if err != nil {
		t.Fatalf("commitPostgresRoutineRunTerminal: %v", err)
	}

	var runUpdates, occurrenceUpdates, turnUpdates, activities int
	for _, statement := range tx.execs {
		switch {
		case strings.Contains(statement.sql, "update fort_private.routine_run"):
			runUpdates++
			if !containsArgument(statement.args, string(conversation.RoutineRunSucceeded)) ||
				!containsArgument(statement.args, "attempt:daily:1") {
				t.Fatalf("Routine terminal update arguments = %#v", statement.args)
			}
		case strings.Contains(statement.sql, "update fort_private.routine_occurrence"):
			occurrenceUpdates++
		case strings.Contains(statement.sql, "update fort_private.conversation_turn"):
			turnUpdates++
			if !containsArgument(statement.args, "settled") ||
				!containsArgument(statement.args, routineTurnID("run:daily:1")) {
				t.Fatalf("Routine successful turn update arguments = %#v", statement.args)
			}
		case strings.Contains(statement.sql, "insert into fort_private.ledger_event"):
			activities++
		}
	}
	if resultMessages != 1 || runUpdates != 1 || occurrenceUpdates != 1 || turnUpdates != 1 || activities != 1 {
		t.Fatalf("Routine terminal writes = messages %d, run %d, occurrence %d, turn %d, activity %d",
			resultMessages, runUpdates, occurrenceUpdates, turnUpdates, activities)
	}
}

func TestWorkerTerminalFailureDoesNotCreateRoutineResult(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 17, 5, 0, 0, time.UTC)
	tx := &fakeTransaction{queryRowHook: func(statement string, _ []any) row {
		if !strings.Contains(statement, "from fort_private.routine_run as run") {
			return fakeRow{err: errors.New("unexpected Routine worker terminal query")}
		}
		return fakeRow{values: []any{
			"run:daily:1", "occurrence:daily:1", "conversation:results", "agent:researcher",
			string(conversation.RoutineRunQueued), "queued", sql.NullString{},
			sql.NullString{String: "attempt:daily:1", Valid: true},
			sql.NullString{String: "lease:daily:1", Valid: true}, sql.NullString{String: "active", Valid: true},
			int64(0),
		}}
	}}
	store, err := newStoreWithKeyRing(&fakeDatabase{}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatalf("newStoreWithKeyRing: %v", err)
	}
	err = store.commitPostgresRoutineRunTerminal(context.Background(), tx, testAccountID,
		"target:routine:daily:1", "attempt:daily:1", "lease:daily:1", coreworker.TerminalFailed, "", now)
	if err != nil {
		t.Fatalf("commitPostgresRoutineRunTerminal: %v", err)
	}
	var failedTurnUpdated bool
	for _, statement := range tx.execs {
		if strings.Contains(statement.sql, "insert into fort_private.conversation_message") {
			t.Fatalf("failed Routine created a result: %q", statement.sql)
		}
		if strings.Contains(statement.sql, "update fort_private.conversation_turn") &&
			containsArgument(statement.args, "needs_you") &&
			containsArgument(statement.args, routineTurnID("run:daily:1")) {
			failedTurnUpdated = true
		}
	}
	if !failedTurnUpdated {
		t.Fatalf("failed Routine did not move its turn to needs_you: %+v", tx.execs)
	}
}
