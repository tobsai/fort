package controlapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestRoutinesHandlerAllocatesFortCloudIdentityAndCurrentPins(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	input := `{"idempotency_key":"routine:create:weekly","trigger":"schedule","schedule":"0 0 9 * * 1","timezone":"America/Chicago","next_occurrence":"2026-08-24T14:00:00Z","input_source":"fort:conversation:research","freshness_seconds":86400,"expected_result":"Weekly brief","result_conversation_id":"conversation:home","approval_boundary":"before_external_side_effect","missing_input_behavior":"needs_you","retry_policy":"once","catch_up_policy":"skip","lateness_policy":"within_90s"}`
	repository := routineOwnerFixture()
	request := ownerRoutineRequest(t, now, http.MethodPost, input, "owner.routines.create")
	request.SetPathValue("agent_id", "agent:researcher")
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(request.verifier, "owner.routines.create",
		controlapi.RoutinesHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request.Request)

	command := repository.create
	if recorder.Code != http.StatusCreated || command.Routine.ID == "" || command.Revision.ID == "" ||
		command.Routine.AccountID != testOwnerAccountID || command.Routine.AgentID != "agent:researcher" ||
		command.Routine.CurrentRevisionID != command.Revision.ID || command.Routine.State != conversation.RoutineActive ||
		command.Routine.CreatedAt != now || command.Revision.CreatedAt != now || command.Revision.Revision != 1 ||
		command.Revision.BehaviorRevisionID != "behavior:current" || command.Revision.BindingRevisionID != "binding:current" ||
		command.Revision.Authority != conversation.RoutineAuthorityFortCloud ||
		command.Revision.Trigger != conversation.RoutineTriggerSchedule || command.Revision.ResultConversationID != "conversation:home" {
		t.Fatalf("status/command = %d/%+v", recorder.Code, command)
	}
}

func TestRoutinesHandlerRejectsExecutionIdentityAndUntruthfulEventSchedule(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	for _, input := range []string{
		`{"idempotency_key":"routine:create","trigger":"event","input_source":"fort:event:mail","freshness_seconds":60,"expected_result":"Digest","result_conversation_id":"conversation:home","approval_boundary":"none","missing_input_behavior":"skip","retry_policy":"once","catch_up_policy":"skip","lateness_policy":"none","provider":"openclaw"}`,
		`{"idempotency_key":"routine:create","trigger":"event","schedule":"0 9 * * *","input_source":"fort:event:mail","freshness_seconds":60,"expected_result":"Digest","result_conversation_id":"conversation:home","approval_boundary":"none","missing_input_behavior":"skip","retry_policy":"once","catch_up_policy":"skip","lateness_policy":"none"}`,
	} {
		repository := routineOwnerFixture()
		request := ownerRoutineRequest(t, now, http.MethodPost, input, "owner.routines.create")
		request.SetPathValue("agent_id", "agent:researcher")
		recorder := httptest.NewRecorder()
		controlapi.RequireServiceAssertion(request.verifier, "owner.routines.create",
			controlapi.RoutinesHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request.Request)
		if recorder.Code != http.StatusBadRequest || repository.create.Routine.ID != "" {
			t.Fatalf("input %s status/command = %d/%+v", input, recorder.Code, repository.create)
		}
	}
}

func TestRoutinesHandlerRejectsMalformedCronOrTimezoneBeforePersistence(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	for _, scheduleAndTimezone := range []string{
		`"schedule":"0 9 * * 1","timezone":"America/Chicago"`,
		`"schedule":"0 0 25 * * *","timezone":"America/Chicago"`,
		`"schedule":"0 0 9 * * 1","timezone":"Mars/Olympus"`,
	} {
		input := `{"idempotency_key":"routine:create:invalid","trigger":"schedule",` + scheduleAndTimezone +
			`,"next_occurrence":"2026-08-24T14:00:00Z","input_source":"fort:conversation:research","freshness_seconds":86400,"expected_result":"Weekly brief","result_conversation_id":"conversation:home","approval_boundary":"none","missing_input_behavior":"needs_you","retry_policy":"once","catch_up_policy":"skip","lateness_policy":"within_90s"}`
		repository := routineOwnerFixture()
		request := ownerRoutineRequest(t, now, http.MethodPost, input, "owner.routines.create")
		request.SetPathValue("agent_id", "agent:researcher")
		recorder := httptest.NewRecorder()
		controlapi.RequireServiceAssertion(request.verifier, "owner.routines.create",
			controlapi.RoutinesHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request.Request)
		if recorder.Code != http.StatusBadRequest || repository.create.Routine.ID != "" {
			t.Fatalf("input %s status/command = %d/%+v", input, recorder.Code, repository.create)
		}
	}
}

func TestRoutinesHandlerListVerifiesExactAgentParentBeforeListingChildren(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	repository := routineOwnerFixture()
	repository.agent.Agent.ID = "agent:foreign"
	request := ownerRoutineRequest(t, now, http.MethodGet, "", "owner.routines.list")
	request.SetPathValue("agent_id", "agent:researcher")
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(request.verifier, "owner.routines.list",
		controlapi.RoutinesHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request.Request)
	if recorder.Code != http.StatusNotFound || repository.listCalls != 0 {
		t.Fatalf("foreign parent status/list calls = %d/%d, want 404/0", recorder.Code, repository.listCalls)
	}
}

func TestRoutineMutationAndTestHandlersDeriveSuccessorAndOccurrence(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 21, 0, 0, 0, time.UTC)
	repository := routineOwnerFixture()
	repository.routine.Routine.State = conversation.RoutinePaused
	repository.routine.PauseReason = ledger.RoutinePauseNeedsRevalidation

	mutation := `{"idempotency_key":"routine:revalidate:2","action":"revalidate"}`
	request := ownerRoutineRequest(t, now, http.MethodPatch, mutation, "owner.routines.mutate")
	request.SetPathValue("agent_id", "agent:researcher")
	request.SetPathValue("routine_id", "routine:weekly")
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(request.verifier, "owner.routines.mutate",
		controlapi.RoutineMutationHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request.Request)
	if recorder.Code != http.StatusOK || repository.revalidate.Revision.Revision != 2 ||
		repository.revalidate.Revision.ID == repository.routine.CurrentRevision.ID ||
		repository.revalidate.Revision.BehaviorRevisionID != "behavior:current" ||
		repository.revalidate.Revision.BindingRevisionID != "binding:current" ||
		repository.revalidate.Revision.ExpectedResult != repository.routine.CurrentRevision.ExpectedResult ||
		repository.revalidate.Revision.CreatedAt != now {
		t.Fatalf("revalidation status/command = %d/%+v", recorder.Code, repository.revalidate)
	}

	testBody := `{"idempotency_key":"routine:test:one"}`
	request = ownerRoutineRequest(t, now, http.MethodPost, testBody, "owner.routines.test")
	request.SetPathValue("agent_id", "agent:researcher")
	request.SetPathValue("routine_id", "routine:weekly")
	recorder = httptest.NewRecorder()
	controlapi.RequireServiceAssertion(request.verifier, "owner.routines.test",
		controlapi.RoutineTestHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request.Request)
	command := repository.enqueue
	if recorder.Code != http.StatusAccepted || command.AccountID != testOwnerAccountID ||
		command.RoutineID != "routine:weekly" || command.RoutineRevisionID != repository.routine.CurrentRevision.ID ||
		command.Kind != conversation.RoutineRunTest || command.OccurrenceID == "" || command.RunID == "" ||
		command.ApprovalEvidenceID == "" || command.ScheduledFor != now || command.CreatedAt != now {
		t.Fatalf("test status/command = %d/%+v", recorder.Code, command)
	}
}

func TestRoutineRunsHandlerReturnsExactOwnerRoutineHistory(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 21, 30, 0, 0, time.UTC)
	repository := routineOwnerFixture()
	repository.runs = []ledger.RoutineRunRecord{{
		Occurrence: ledger.RoutineOccurrence{ID: "occurrence:weekly:1", AccountID: testOwnerAccountID,
			RoutineID: "routine:weekly", RoutineRevisionID: repository.routine.CurrentRevision.ID,
			Kind: conversation.RoutineRunScheduled, State: conversation.RoutineRunSucceeded,
			ScheduledFor: now.Add(-time.Minute), IdempotencyKey: "routine:weekly@one",
			ApprovalEvidenceID: "approval:weekly:1", CreatedAt: now.Add(-time.Minute), UpdatedAt: now},
		Run: conversation.RoutineRun{ID: "run:weekly:1", RoutineID: "routine:weekly",
			RoutineRevisionID: repository.routine.CurrentRevision.ID, AgentID: "agent:researcher",
			BehaviorRevisionID: "behavior:old", BindingRevisionID: "binding:old",
			OccurrenceID: "occurrence:weekly:1", Kind: conversation.RoutineRunScheduled,
			State: conversation.RoutineRunSucceeded, NormalizedResult: "Weekly brief", CreatedAt: now.Add(-time.Minute)},
		ResultConversationID: "conversation:home",
		Activities: []ledger.RoutineRunActivity{{Sequence: 1, State: conversation.RoutineRunSucceeded,
			Activity: "worker completed Routine run", CreatedAt: now}},
	}}
	request := ownerRoutineRequest(t, now, http.MethodGet, "", "owner.routines.runs")
	request.SetPathValue("agent_id", "agent:researcher")
	request.SetPathValue("routine_id", "routine:weekly")
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(request.verifier, "owner.routines.runs",
		controlapi.RoutineRunsHandler(repository)).ServeHTTP(recorder, request.Request)
	if recorder.Code != http.StatusOK || repository.runListCalls != 1 ||
		!strings.Contains(recorder.Body.String(), `"id":"run:weekly:1"`) ||
		!strings.Contains(recorder.Body.String(), `"normalized_result":"Weekly brief"`) {
		t.Fatalf("Routine history status/calls/body = %d/%d/%s", recorder.Code,
			repository.runListCalls, recorder.Body.String())
	}
}

func TestRoutineTestHandlerMapsRevalidationPauseToConflict(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 21, 0, 0, 0, time.UTC)
	repository := routineOwnerFixture()
	repository.err = ledger.ErrRoutineNeedsRevalidation
	body := `{"idempotency_key":"routine:test:one"}`
	request := ownerRoutineRequest(t, now, http.MethodPost, body, "owner.routines.test")
	request.SetPathValue("agent_id", "agent:researcher")
	request.SetPathValue("routine_id", "routine:weekly")
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(request.verifier, "owner.routines.test",
		controlapi.RoutineTestHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request.Request)
	if recorder.Code != http.StatusConflict || !errors.Is(repository.err, ledger.ErrRoutineNeedsRevalidation) {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
}

type routineRequest struct {
	*http.Request
	verifier controlapi.ServiceAssertionVerifier
}

func ownerRoutineRequest(t *testing.T, now time.Time, method, body, scope string) routineRequest {
	t.Helper()
	verifier, token := serviceAuthorizationFixture(t, now, body, scope)
	request := httptest.NewRequest(method, "/", strings.NewReader(body))
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	return routineRequest{Request: request, verifier: verifier}
}

type fakeRoutineOwnerRepository struct {
	agent        ledger.AgentRecord
	routine      ledger.RoutineRecord
	create       ledger.CreateRoutineCommand
	revalidate   ledger.RevalidateRoutineCommand
	enqueue      ledger.EnqueueRoutineOccurrenceCommand
	err          error
	listCalls    int
	runs         []ledger.RoutineRunRecord
	runListCalls int
}

func routineOwnerFixture() *fakeRoutineOwnerRepository {
	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	revision := conversation.RoutineRevision{
		ID: "routine-revision:weekly:1", RoutineID: "routine:weekly", Revision: 1,
		AgentID: "agent:researcher", BehaviorRevisionID: "behavior:old", BindingRevisionID: "binding:old",
		Authority: conversation.RoutineAuthorityFortCloud, Trigger: conversation.RoutineTriggerSchedule,
		Schedule: "0 0 9 * * 1", Timezone: "America/Chicago", NextOccurrence: now.Add(7 * 24 * time.Hour),
		InputSource: "fort:conversation:research", FreshnessSeconds: 86400, ExpectedResult: "Weekly brief",
		ResultConversationID: "conversation:home", ApprovalBoundary: "before_external_side_effect",
		MissingInputBehavior: "needs_you", RetryPolicy: "once", CatchUpPolicy: "skip",
		LatenessPolicy: "within_90s", CreatedAt: now,
	}
	return &fakeRoutineOwnerRepository{
		agent: ledger.AgentRecord{Agent: conversation.Agent{ID: "agent:researcher", AccountID: testOwnerAccountID,
			State: conversation.AgentOpen, CurrentBehaviorRevisionID: "behavior:current",
			CurrentBindingRevisionID: "binding:current"}},
		routine: ledger.RoutineRecord{Routine: conversation.Routine{ID: "routine:weekly", AccountID: testOwnerAccountID,
			AgentID: "agent:researcher", CurrentRevisionID: revision.ID, State: conversation.RoutineActive,
			CreatedAt: now}, CurrentRevision: revision},
	}
}

func (repository *fakeRoutineOwnerRepository) GetAgent(context.Context, string, string) (ledger.AgentRecord, error) {
	return repository.agent, nil
}
func (repository *fakeRoutineOwnerRepository) ListRoutines(context.Context, string, string) ([]ledger.RoutineRecord, error) {
	repository.listCalls++
	return []ledger.RoutineRecord{}, nil
}
func (repository *fakeRoutineOwnerRepository) CreateRoutine(_ context.Context, command ledger.CreateRoutineCommand) (ledger.RoutineRecord, error) {
	repository.create = command
	return ledger.RoutineRecord{Routine: command.Routine, CurrentRevision: command.Revision}, nil
}
func (repository *fakeRoutineOwnerRepository) GetRoutine(context.Context, string, string) (ledger.RoutineRecord, error) {
	return repository.routine, nil
}
func (repository *fakeRoutineOwnerRepository) RevalidateRoutine(_ context.Context, command ledger.RevalidateRoutineCommand) (ledger.RoutineRecord, error) {
	repository.revalidate = command
	result := repository.routine
	result.Routine.State = conversation.RoutineActive
	result.Routine.CurrentRevisionID = command.Revision.ID
	result.CurrentRevision = command.Revision
	result.PauseReason = ""
	return result, nil
}
func (repository *fakeRoutineOwnerRepository) EnqueueRoutineOccurrence(_ context.Context, command ledger.EnqueueRoutineOccurrenceCommand) (ledger.RoutineRunRecord, error) {
	repository.enqueue = command
	return ledger.RoutineRunRecord{Occurrence: ledger.RoutineOccurrence{ID: command.OccurrenceID},
		Run: conversation.RoutineRun{ID: command.RunID}}, repository.err
}

func (repository *fakeRoutineOwnerRepository) ListRoutineRuns(context.Context, string, string) ([]ledger.RoutineRunRecord, error) {
	repository.runListCalls++
	return repository.runs, repository.err
}
