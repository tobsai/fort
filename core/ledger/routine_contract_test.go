package ledger_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
	"github.com/tobsai/fort/core/store"
)

func TestSQLiteRoutineLedgerCreatesAgentOwnedFortCloudRoutineIdempotently(t *testing.T) {
	repository := openRoutineRepository(t)
	agent := stableAgentCommandForCollaboration(t, "researcher", "Researcher")
	if _, err := repository.CreateAgent(context.Background(), agent); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	command := routineCommand(agent)
	created, err := repository.CreateRoutine(context.Background(), command)
	if err != nil {
		t.Fatalf("CreateRoutine: %v", err)
	}
	if created.Routine.ID != command.Routine.ID || created.CurrentRevision.ID != command.Revision.ID ||
		created.CurrentRevision.AgentID != agent.Agent.ID ||
		created.CurrentRevision.BehaviorRevisionID != agent.Behavior.ID ||
		created.CurrentRevision.BindingRevisionID != agent.Binding.ID ||
		created.CurrentRevision.ResultConversationID != agent.Home.ID {
		t.Fatalf("created Routine lost exact owner or execution pins: %+v", created)
	}

	replayed, err := repository.CreateRoutine(context.Background(), command)
	if err != nil {
		t.Fatalf("replay CreateRoutine: %v", err)
	}
	if replayed.Routine.ID != created.Routine.ID || replayed.CurrentRevision.ID != created.CurrentRevision.ID {
		t.Fatalf("replayed Routine changed identity: first %+v replay %+v", created, replayed)
	}
	conflict := command
	conflict.Revision.ExpectedResult = "A conflicting result under the same key"
	if _, err := repository.CreateRoutine(context.Background(), conflict); !errors.Is(err, ledger.ErrIdempotencyConflict) {
		t.Fatalf("conflicting Routine create error = %v, want %v", err, ledger.ErrIdempotencyConflict)
	}
	foreignAgent := stableAgentCommandForCollaboration(t, "foreign", "Foreign")
	foreignAgent.Agent.AccountID = "account:foreign"
	foreignAgent.ExecutionSource.ID = "source:foreign"
	foreignAgent.ExecutionSource.AccountID = foreignAgent.Agent.AccountID
	foreignAgent.Binding.ExecutionSourceID = foreignAgent.ExecutionSource.ID
	foreignAgent.SourceAgent.ExecutionSourceID = foreignAgent.ExecutionSource.ID
	if _, err := repository.CreateAgent(context.Background(), foreignAgent); err != nil {
		t.Fatalf("CreateAgent foreign: %v", err)
	}
	foreignResult := command
	foreignResult.IdempotencyKey = "create-routine:foreign-result"
	foreignResult.Routine.ID = "routine:foreign-result"
	foreignResult.Routine.CurrentRevisionID = "routine-revision:foreign-result:1"
	foreignResult.Revision.ID = foreignResult.Routine.CurrentRevisionID
	foreignResult.Revision.RoutineID = foreignResult.Routine.ID
	foreignResult.Revision.ResultConversationID = foreignAgent.Home.ID
	if _, err := repository.CreateRoutine(context.Background(), foreignResult); err == nil {
		t.Fatal("CreateRoutine accepted a result Conversation from another account")
	}
	sibling := stableAgentCommandForCollaboration(t, "builder", "Builder")
	if _, err := repository.CreateAgent(context.Background(), sibling); err != nil {
		t.Fatalf("CreateAgent sibling: %v", err)
	}
	foreignOwnerResult := command
	foreignOwnerResult.IdempotencyKey = "create-routine:foreign-owner-result"
	foreignOwnerResult.Routine.ID = "routine:foreign-owner-result"
	foreignOwnerResult.Routine.CurrentRevisionID = "routine-revision:foreign-owner-result:1"
	foreignOwnerResult.Revision.ID = foreignOwnerResult.Routine.CurrentRevisionID
	foreignOwnerResult.Revision.RoutineID = foreignOwnerResult.Routine.ID
	foreignOwnerResult.Revision.ResultConversationID = sibling.Home.ID
	if _, err := repository.CreateRoutine(context.Background(), foreignOwnerResult); err == nil {
		t.Fatal("CreateRoutine accepted a result Conversation owned by another Agent")
	}

	routines, err := repository.ListRoutines(context.Background(), agent.Agent.AccountID, agent.Agent.ID)
	if err != nil {
		t.Fatalf("ListRoutines: %v", err)
	}
	if len(routines) != 1 || routines[0].Routine.ID != command.Routine.ID {
		t.Fatalf("listed Routines = %+v", routines)
	}
	empty, err := repository.ListRoutines(context.Background(), "account:foreign", agent.Agent.ID)
	if err != nil {
		t.Fatalf("ListRoutines foreign account: %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("foreign-account Routines = %#v, want non-nil []", empty)
	}
	if _, err := repository.GetRoutine(context.Background(), "account:foreign", command.Routine.ID); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("foreign-account GetRoutine error = %v, want %v", err, ledger.ErrNotFound)
	}
}

func TestSQLiteRoutineLedgerImportsOnlyExactFencedSourceProjection(t *testing.T) {
	repository := openRoutineRepository(t)
	agent := stableAgentCommandForCollaboration(t, "researcher", "Researcher")
	if _, err := repository.CreateAgent(context.Background(), agent); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	projection := sourceRoutineProjection(agent)
	recorded, err := repository.RecordSourceRoutineProjection(context.Background(), projection)
	if err != nil {
		t.Fatalf("RecordSourceRoutineProjection: %v", err)
	}
	if recorded.ID != projection.ID || recorded.ExecutionSourceID != agent.ExecutionSource.ID ||
		recorded.OpaqueSourceRoutineID != projection.OpaqueSourceRoutineID {
		t.Fatalf("source-native projection = %+v", recorded)
	}
	if _, err := repository.RecordSourceRoutineProjection(context.Background(), projection); err != nil {
		t.Fatalf("replay source projection: %v", err)
	}
	conflictingProjection := projection
	conflictingProjection.ScheduleSnapshot = []byte(`{"cron":"0 8 * * 1"}`)
	conflictingProjection.ProjectionDigest = digestBytes(conflictingProjection.ScheduleSnapshot)
	if _, err := repository.RecordSourceRoutineProjection(context.Background(), conflictingProjection); !errors.Is(err, ledger.ErrIdempotencyConflict) {
		t.Fatalf("conflicting immutable projection error = %v, want %v", err, ledger.ErrIdempotencyConflict)
	}

	create := routineCommand(agent)
	create.IdempotencyKey = "import-routine:weekly-research"
	create.Routine.ID = "routine:imported-weekly-research"
	create.Routine.CurrentRevisionID = "routine-revision:imported-weekly-research:1"
	create.Revision.ID = create.Routine.CurrentRevisionID
	create.Revision.RoutineID = create.Routine.ID
	create.Revision.NextOccurrence = projection.NextOccurrenceAt
	receipt := ledger.RoutineImportReceipt{
		ID: "routine-import:weekly-research", AccountID: agent.Agent.AccountID,
		SourceRoutineProjectionID: projection.ID, RoutineID: create.Routine.ID,
		RoutineRevisionID: create.Revision.ID, SourceDisabledAt: projection.ObservedAt.Add(time.Minute),
		ExactLastSourceOccurrenceAt: projection.LastOccurrenceAt,
		ExactNextSourceOccurrenceAt: projection.NextOccurrenceAt,
		FencingReceiptCiphertext:    []byte("encrypted-source-disable-receipt"),
		FencingReceiptKeyID:         "key:local:1", FencingReceiptNonce: []byte("123456789012"),
		ImportedAt: projection.ObservedAt.Add(2 * time.Minute),
	}
	receipt.FencingReceiptDigest = digestBytes(receipt.FencingReceiptCiphertext)

	invalid := ledger.ImportRoutineCommand{Create: create, Receipt: receipt}
	invalid.Receipt.ExactNextSourceOccurrenceAt = projection.NextOccurrenceAt.Add(time.Hour)
	if _, err := repository.ImportSourceRoutine(context.Background(), invalid); err == nil {
		t.Fatal("ImportSourceRoutine accepted fencing evidence for another occurrence")
	}
	lateFence := ledger.ImportRoutineCommand{Create: create, Receipt: receipt}
	lateFence.Receipt.SourceDisabledAt = projection.NextOccurrenceAt
	lateFence.Receipt.ImportedAt = projection.NextOccurrenceAt.Add(time.Minute)
	if _, err := repository.ImportSourceRoutine(context.Background(), lateFence); err == nil {
		t.Fatal("ImportSourceRoutine accepted a source disable at or after its next occurrence")
	}
	if _, err := repository.GetRoutine(context.Background(), agent.Agent.AccountID, create.Routine.ID); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("failed import persisted a partial Routine: %v", err)
	}

	imported, err := repository.ImportSourceRoutine(context.Background(), ledger.ImportRoutineCommand{Create: create, Receipt: receipt})
	if err != nil {
		t.Fatalf("ImportSourceRoutine: %v", err)
	}
	if imported.ImportReceipt == nil || imported.ImportReceipt.ID != receipt.ID ||
		imported.CurrentRevision.NextOccurrence != projection.NextOccurrenceAt ||
		imported.CurrentRevision.Authority != conversation.RoutineAuthorityFortCloud {
		t.Fatalf("imported Routine lost fencing or fort_cloud authority evidence: %+v", imported)
	}

	projections, err := repository.ListSourceRoutineProjections(context.Background(), agent.Agent.AccountID, agent.ExecutionSource.ID)
	if err != nil {
		t.Fatalf("ListSourceRoutineProjections: %v", err)
	}
	if len(projections) != 1 || projections[0].ID != projection.ID {
		t.Fatalf("source projections = %+v", projections)
	}
	empty, err := repository.ListSourceRoutineProjections(context.Background(), "account:foreign", agent.ExecutionSource.ID)
	if err != nil {
		t.Fatalf("foreign ListSourceRoutineProjections: %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("foreign source projections = %#v, want non-nil []", empty)
	}
}

func TestSQLiteRoutineLedgerRunsScheduledAndTestOccurrencesThroughOneEvidencePath(t *testing.T) {
	repository := openRoutineRepository(t)
	agent := stableAgentCommandForCollaboration(t, "researcher", "Researcher")
	if _, err := repository.CreateAgent(context.Background(), agent); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	create := routineCommand(agent)
	if _, err := repository.CreateRoutine(context.Background(), create); err != nil {
		t.Fatalf("CreateRoutine: %v", err)
	}

	scheduled := occurrenceCommand(create, conversation.RoutineRunScheduled, "scheduled", create.Revision.NextOccurrence)
	queued, err := repository.EnqueueRoutineOccurrence(context.Background(), scheduled)
	if err != nil {
		t.Fatalf("EnqueueRoutineOccurrence scheduled: %v", err)
	}
	if queued.Occurrence.ID != scheduled.OccurrenceID || queued.Occurrence.ApprovalEvidenceID != scheduled.ApprovalEvidenceID ||
		queued.Run.RoutineRevisionID != create.Revision.ID || queued.Run.AgentID != agent.Agent.ID ||
		queued.Run.BehaviorRevisionID != agent.Behavior.ID || queued.Run.BindingRevisionID != agent.Binding.ID ||
		queued.ResultConversationID != agent.Home.ID || queued.Run.State != conversation.RoutineRunQueued ||
		len(queued.Activities) != 1 || queued.Activities[0].State != conversation.RoutineRunQueued {
		t.Fatalf("queued scheduled occurrence lost pins or evidence: %+v", queued)
	}
	replayed, err := repository.EnqueueRoutineOccurrence(context.Background(), scheduled)
	if err != nil {
		t.Fatalf("replay scheduled occurrence: %v", err)
	}
	if replayed.Run.ID != queued.Run.ID || len(replayed.Activities) != 1 {
		t.Fatalf("replayed occurrence duplicated work: first %+v replay %+v", queued, replayed)
	}
	conflict := scheduled
	conflict.ApprovalEvidenceID = "approval-evaluation:conflict"
	if _, err := repository.EnqueueRoutineOccurrence(context.Background(), conflict); !errors.Is(err, ledger.ErrIdempotencyConflict) {
		t.Fatalf("conflicting occurrence replay error = %v, want %v", err, ledger.ErrIdempotencyConflict)
	}

	needsYou, err := repository.AdvanceRoutineRun(context.Background(), ledger.AdvanceRoutineRunCommand{
		AccountID: agent.Agent.AccountID, RunID: queued.Run.ID, IdempotencyKey: "routine-run:scheduled:needs-you",
		FromState: conversation.RoutineRunQueued, ToState: conversation.RoutineRunNeedsYou,
		Activity: "input freshness check failed", FailureCode: "input_stale",
		NextAction: "Refresh the selected input before retrying.", OccurredAt: scheduled.CreatedAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("AdvanceRoutineRun needs_you: %v", err)
	}
	if needsYou.Run.State != conversation.RoutineRunNeedsYou || needsYou.FailureCode != "input_stale" ||
		needsYou.NextAction == "" || len(needsYou.Activities) != 2 {
		t.Fatalf("needs-you run evidence = %+v", needsYou)
	}

	testOccurrence := occurrenceCommand(create, conversation.RoutineRunTest, "test", scheduled.ScheduledFor.Add(time.Minute))
	testQueued, err := repository.EnqueueRoutineOccurrence(context.Background(), testOccurrence)
	if err != nil {
		t.Fatalf("EnqueueRoutineOccurrence test: %v", err)
	}
	workingAt := testOccurrence.CreatedAt.Add(time.Minute)
	workingCommand := ledger.AdvanceRoutineRunCommand{
		AccountID: agent.Agent.AccountID, RunID: testQueued.Run.ID, IdempotencyKey: "routine-run:test:working",
		FromState: conversation.RoutineRunQueued, ToState: conversation.RoutineRunWorking,
		AttemptID: "attempt:routine:test:1", LeaseID: "lease:routine:test:1",
		LeaseExpiresAt: workingAt.Add(5 * time.Minute), Activity: "worker lease claimed", OccurredAt: workingAt,
	}
	working, err := repository.AdvanceRoutineRun(context.Background(), workingCommand)
	if err != nil {
		t.Fatalf("AdvanceRoutineRun working: %v", err)
	}
	if working.Run.Kind != conversation.RoutineRunTest || working.AttemptID != workingCommand.AttemptID ||
		working.LeaseID != workingCommand.LeaseID || working.LeaseExpiresAt != workingCommand.LeaseExpiresAt {
		t.Fatalf("test Routine did not use normal lease path: %+v", working)
	}
	workingReplay, err := repository.AdvanceRoutineRun(context.Background(), workingCommand)
	if err != nil {
		t.Fatalf("replay working activity: %v", err)
	}
	if len(workingReplay.Activities) != len(working.Activities) {
		t.Fatalf("replayed activity duplicated history: first %+v replay %+v", working.Activities, workingReplay.Activities)
	}

	succeeded, err := repository.AdvanceRoutineRun(context.Background(), ledger.AdvanceRoutineRunCommand{
		AccountID: agent.Agent.AccountID, RunID: testQueued.Run.ID, IdempotencyKey: "routine-run:test:succeeded",
		FromState: conversation.RoutineRunWorking, ToState: conversation.RoutineRunSucceeded,
		AttemptID: workingCommand.AttemptID, LeaseID: workingCommand.LeaseID,
		LeaseExpiresAt: workingCommand.LeaseExpiresAt, Activity: "normalized result committed",
		NormalizedResult: "Primary-source weekly brief.", OccurredAt: workingAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("AdvanceRoutineRun succeeded: %v", err)
	}
	if succeeded.Run.NormalizedResult != "Primary-source weekly brief." || succeeded.Run.ResultMessageID == "" ||
		succeeded.ResultConversationID != agent.Home.ID || len(succeeded.Activities) != 3 {
		t.Fatalf("successful Routine result = %+v", succeeded)
	}
	if _, err := repository.AdvanceRoutineRun(context.Background(), ledger.AdvanceRoutineRunCommand{
		AccountID: agent.Agent.AccountID, RunID: testQueued.Run.ID, IdempotencyKey: "routine-run:test:second-terminal",
		FromState: conversation.RoutineRunWorking, ToState: conversation.RoutineRunFailed,
		AttemptID: workingCommand.AttemptID, LeaseID: workingCommand.LeaseID, LeaseExpiresAt: workingCommand.LeaseExpiresAt,
		Activity: "conflicting terminal", FailureCode: "late_failure", OccurredAt: workingAt.Add(2 * time.Minute),
	}); !errors.Is(err, ledger.ErrRoutineRunTerminal) {
		t.Fatalf("second terminal transition error = %v, want %v", err, ledger.ErrRoutineRunTerminal)
	}

	runs, err := repository.ListRoutineRuns(context.Background(), agent.Agent.AccountID, create.Routine.ID)
	if err != nil {
		t.Fatalf("ListRoutineRuns: %v", err)
	}
	if len(runs) != 2 || runs[0].Run.ID != testOccurrence.RunID || runs[1].Run.ID != scheduled.RunID ||
		runs[0].Run.Kind != conversation.RoutineRunTest || runs[0].Run.State != conversation.RoutineRunSucceeded {
		t.Fatalf("Routine run history = %+v", runs)
	}
}

func TestSQLiteRoutineLedgerPausesOnAgentRevisionChangeUntilExplicitRevalidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fort.db")
	opened, err := store.Open(path)
	if err != nil {
		t.Fatalf("open Store: %v", err)
	}
	t.Cleanup(func() { _ = opened.Close() })
	var repository ledger.RoutineRepository = opened

	agent := stableAgentCommandForCollaboration(t, "researcher", "Researcher")
	if _, err := repository.CreateAgent(context.Background(), agent); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	create := routineCommand(agent)
	if _, err := repository.CreateRoutine(context.Background(), create); err != nil {
		t.Fatalf("CreateRoutine: %v", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open revision fixture: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`UPDATE routine_revision SET expected_result='mutated' WHERE id=?`, create.Revision.ID); err == nil {
		t.Fatal("immutable Routine Revision accepted an in-place mutation")
	}
	behaviorID, bindingID := installAcceptedAgentRevisionFixture(t, db, agent, create.Routine.CreatedAt.Add(time.Hour))
	pausedOnChange, err := repository.GetRoutine(context.Background(), agent.Agent.AccountID, create.Routine.ID)
	if err != nil {
		t.Fatalf("GetRoutine after Agent revision change: %v", err)
	}
	if pausedOnChange.Routine.State != conversation.RoutinePaused ||
		pausedOnChange.PauseReason != ledger.RoutinePauseNeedsRevalidation {
		t.Fatalf("Agent revision change did not immediately pause Routine: %+v", pausedOnChange)
	}

	stale := occurrenceCommand(create, conversation.RoutineRunScheduled, "stale", create.Revision.NextOccurrence)
	if _, err := repository.EnqueueRoutineOccurrence(context.Background(), stale); !errors.Is(err, ledger.ErrRoutineNeedsRevalidation) {
		t.Fatalf("stale Routine occurrence error = %v, want %v", err, ledger.ErrRoutineNeedsRevalidation)
	}
	paused, err := repository.GetRoutine(context.Background(), agent.Agent.AccountID, create.Routine.ID)
	if err != nil {
		t.Fatalf("GetRoutine paused: %v", err)
	}
	if paused.Routine.State != conversation.RoutinePaused || paused.PauseReason != ledger.RoutinePauseNeedsRevalidation {
		t.Fatalf("stale Routine state = %+v, want durable revalidation pause", paused)
	}
	runs, err := repository.ListRoutineRuns(context.Background(), agent.Agent.AccountID, create.Routine.ID)
	if err != nil {
		t.Fatalf("ListRoutineRuns after stale enqueue: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("stale Routine dispatched work before revalidation: %+v", runs)
	}

	revision := create.Revision
	revision.ID = "routine-revision:weekly-research:2"
	revision.Revision = 2
	revision.BehaviorRevisionID = behaviorID
	revision.BindingRevisionID = bindingID
	revision.NextOccurrence = create.Revision.NextOccurrence.Add(7 * 24 * time.Hour)
	revision.CreatedAt = create.Routine.CreatedAt.Add(2 * time.Hour)
	revalidated, err := repository.RevalidateRoutine(context.Background(), ledger.RevalidateRoutineCommand{
		AccountID: agent.Agent.AccountID, RoutineID: create.Routine.ID,
		IdempotencyKey: "revalidate-routine:weekly-research:2", Revision: revision,
	})
	if err != nil {
		t.Fatalf("RevalidateRoutine: %v", err)
	}
	if revalidated.Routine.State != conversation.RoutineActive || revalidated.PauseReason != "" ||
		revalidated.CurrentRevision.ID != revision.ID || revalidated.CurrentRevision.BehaviorRevisionID != behaviorID ||
		revalidated.CurrentRevision.BindingRevisionID != bindingID {
		t.Fatalf("revalidated Routine = %+v", revalidated)
	}

	ready := occurrenceCommand(create, conversation.RoutineRunScheduled, "revalidated", revision.NextOccurrence)
	ready.RoutineRevisionID = revision.ID
	queued, err := repository.EnqueueRoutineOccurrence(context.Background(), ready)
	if err != nil {
		t.Fatalf("enqueue revalidated occurrence: %v", err)
	}
	if queued.Run.BehaviorRevisionID != behaviorID || queued.Run.BindingRevisionID != bindingID {
		t.Fatalf("revalidated occurrence pins = %+v", queued.Run)
	}
}

func openRoutineRepository(t *testing.T) ledger.RoutineRepository {
	t.Helper()
	repository, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatalf("open Store: %v", err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	return repository
}

func routineCommand(agent ledger.CreateAgentCommand) ledger.CreateRoutineCommand {
	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	routine := conversation.Routine{
		ID: "routine:weekly-research", AccountID: agent.Agent.AccountID, AgentID: agent.Agent.ID,
		CurrentRevisionID: "routine-revision:weekly-research:1", State: conversation.RoutineActive, CreatedAt: now,
	}
	return ledger.CreateRoutineCommand{
		IdempotencyKey: "create-routine:weekly-research",
		Routine:        routine,
		Revision: conversation.RoutineRevision{
			ID: routine.CurrentRevisionID, RoutineID: routine.ID, Revision: 1, AgentID: routine.AgentID,
			BehaviorRevisionID: agent.Behavior.ID, BindingRevisionID: agent.Binding.ID,
			Authority: conversation.RoutineAuthorityFortCloud, Trigger: conversation.RoutineTriggerSchedule,
			Schedule: "0 0 9 * * 1", Timezone: "America/Chicago", NextOccurrence: now.Add(7 * 24 * time.Hour),
			InputSource: "fort:conversation:research", FreshnessSeconds: 86_400,
			ExpectedResult: "Weekly research brief", ResultConversationID: agent.Home.ID,
			ApprovalBoundary: "before_external_side_effect", MissingInputBehavior: "needs_you",
			RetryPolicy: "once", CatchUpPolicy: "skip", LatenessPolicy: "within_90s", CreatedAt: now,
		},
	}
}

func sourceRoutineProjection(agent ledger.CreateAgentCommand) ledger.SourceRoutineProjection {
	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	snapshot := []byte(`{"cron":"0 9 * * 1","timezone":"America/Chicago"}`)
	return ledger.SourceRoutineProjection{
		ID: "source-routine-projection:weekly-research:1", AccountID: agent.Agent.AccountID,
		ExecutionSourceID: agent.ExecutionSource.ID, OpaqueSourceRoutineID: "weekly-research", ProjectionRevision: 1,
		ScheduleSnapshot: snapshot, ProjectionDigest: digestBytes(snapshot),
		LastOccurrenceAt: now.Add(-7 * 24 * time.Hour), NextOccurrenceAt: now.Add(7 * 24 * time.Hour), ObservedAt: now,
	}
}

func digestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func occurrenceCommand(create ledger.CreateRoutineCommand, kind conversation.RoutineRunKind, slug string, scheduledFor time.Time) ledger.EnqueueRoutineOccurrenceCommand {
	createdAt := scheduledFor.Add(-time.Minute)
	return ledger.EnqueueRoutineOccurrenceCommand{
		AccountID: create.Routine.AccountID, RoutineID: create.Routine.ID,
		RoutineRevisionID: create.Revision.ID, OccurrenceID: "routine-occurrence:" + slug,
		RunID: "routine-run:" + slug, Kind: kind, ScheduledFor: scheduledFor,
		IdempotencyKey: "routine-occurrence:" + slug, ApprovalEvidenceID: "approval-evaluation:" + slug,
		CreatedAt: createdAt,
	}
}

func installAcceptedAgentRevisionFixture(t *testing.T, db *sql.DB, agent ledger.CreateAgentCommand, activatedAt time.Time) (string, string) {
	t.Helper()
	behaviorID := "behavior:researcher:2"
	bindingID := "binding:researcher:2"
	if _, err := db.Exec(`INSERT INTO agent_behavior_revision(
id,agent_id,revision,role,standing_instructions,enabled_skills_json,enabled_tools_json,prompt_material,created_at
) SELECT ?,agent_id,2,role,standing_instructions,enabled_skills_json,enabled_tools_json,prompt_material,?
FROM agent_behavior_revision WHERE id=?`, behaviorID, activatedAt.UTC().Format(time.RFC3339Nano), agent.Behavior.ID); err != nil {
		t.Fatalf("insert successor Behavior Revision: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO agent_binding_revision(
id,agent_id,revision,behavior_revision_id,execution_source_id,source_agent_id,seat_id,fort_profile,provider,
requested_model,resolved_model,computer_id,cloud_runtime,adapter_id,adapter_revision,source_config_digest,
authority_id,authority_revision,policy_id,policy_revision,session_behavior,memory_behavior,
capability_evidence_json,readiness_contract_id,readiness_contract_revision,supersedes_revision_id,
activated_at,retired_at
) SELECT ?,agent_id,2,?,execution_source_id,source_agent_id,seat_id,fort_profile,provider,
requested_model,resolved_model,computer_id,cloud_runtime,adapter_id,adapter_revision,source_config_digest,
authority_id,authority_revision,policy_id,policy_revision,session_behavior,memory_behavior,
capability_evidence_json,readiness_contract_id,readiness_contract_revision,id,?,NULL
FROM agent_binding_revision WHERE id=?`, bindingID, behaviorID, activatedAt.UTC().Format(time.RFC3339Nano), agent.Binding.ID); err != nil {
		t.Fatalf("insert successor Binding Revision: %v", err)
	}
	if _, err := db.Exec(`UPDATE stable_agent SET current_behavior_revision_id=?,current_binding_revision_id=?
WHERE account_id=? AND id=?`, behaviorID, bindingID, agent.Agent.AccountID, agent.Agent.ID); err != nil {
		t.Fatalf("advance stable Agent revision fixture: %v", err)
	}
	return behaviorID, bindingID
}
