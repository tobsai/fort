package controlapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

type RoutineOwnerRepository interface {
	AgentReader
	ListRoutines(context.Context, string, string) ([]ledger.RoutineRecord, error)
	CreateRoutine(context.Context, ledger.CreateRoutineCommand) (ledger.RoutineRecord, error)
	GetRoutine(context.Context, string, string) (ledger.RoutineRecord, error)
	ListRoutineRuns(context.Context, string, string) ([]ledger.RoutineRunRecord, error)
	RevalidateRoutine(context.Context, ledger.RevalidateRoutineCommand) (ledger.RoutineRecord, error)
	EnqueueRoutineOccurrence(context.Context, ledger.EnqueueRoutineOccurrenceCommand) (ledger.RoutineRunRecord, error)
}

type routineCreateRequest struct {
	IdempotencyKey       string                      `json:"idempotency_key"`
	Trigger              conversation.RoutineTrigger `json:"trigger"`
	Schedule             string                      `json:"schedule,omitempty"`
	Timezone             string                      `json:"timezone,omitempty"`
	NextOccurrence       time.Time                   `json:"next_occurrence,omitempty"`
	InputSource          string                      `json:"input_source"`
	FreshnessSeconds     int64                       `json:"freshness_seconds"`
	ExpectedResult       string                      `json:"expected_result"`
	ResultConversationID string                      `json:"result_conversation_id"`
	ApprovalBoundary     string                      `json:"approval_boundary"`
	MissingInputBehavior string                      `json:"missing_input_behavior"`
	RetryPolicy          string                      `json:"retry_policy"`
	CatchUpPolicy        string                      `json:"catch_up_policy"`
	LatenessPolicy       string                      `json:"lateness_policy"`
}

// RoutinesHandler lists or creates Agent-owned fort_cloud Routines. The
// stable Agent parent and current Behavior/Binding pins are always resolved by
// the server; the client supplies only Routine semantics and idempotency.
func RoutinesHandler(repository RoutineOwnerRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		accountID, agentID, ok := ownerAgentPath(request)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "routine_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "routine_unavailable"})
			return
		}
		switch request.Method {
		case http.MethodGet:
			agent, err := repository.GetAgent(request.Context(), accountID, agentID)
			if err != nil {
				writeOwnerRepositoryError(response, err, "routine_list_unavailable")
				return
			}
			if agent.Agent.AccountID != accountID || agent.Agent.ID != agentID {
				writeJSON(response, http.StatusNotFound, map[string]string{"code": "not_found"})
				return
			}
			records, err := repository.ListRoutines(request.Context(), accountID, agentID)
			if err != nil {
				writeOwnerRepositoryError(response, err, "routine_list_unavailable")
				return
			}
			if records == nil {
				records = []ledger.RoutineRecord{}
			}
			writeBoundedOwnerJSON(response, records)
		case http.MethodPost:
			var input routineCreateRequest
			if decodeStrictOwnerJSON(response, request, &input) != nil || !validRoutineCreateInput(input) {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "routine_create_invalid"})
				return
			}
			agent, err := repository.GetAgent(request.Context(), accountID, agentID)
			if err != nil {
				writeOwnerRepositoryError(response, err, "routine_create_unavailable")
				return
			}
			if agent.Agent.AccountID != accountID || agent.Agent.ID != agentID {
				writeJSON(response, http.StatusNotFound, map[string]string{"code": "not_found"})
				return
			}
			if agent.Agent.State != conversation.AgentOpen {
				writeJSON(response, http.StatusConflict, map[string]string{"code": "state_conflict"})
				return
			}
			now := ownerClock(clock)
			routineID := ownerCommandID("routine", accountID, agentID, input.IdempotencyKey)
			revisionID := ownerCommandID("routine-revision", accountID, agentID, input.IdempotencyKey, "1")
			command := ledger.CreateRoutineCommand{
				IdempotencyKey: input.IdempotencyKey,
				Routine: conversation.Routine{ID: routineID, AccountID: accountID, AgentID: agentID,
					CurrentRevisionID: revisionID, State: conversation.RoutineActive, CreatedAt: now},
				Revision: conversation.RoutineRevision{ID: revisionID, RoutineID: routineID, Revision: 1,
					AgentID: agentID, BehaviorRevisionID: agent.Agent.CurrentBehaviorRevisionID,
					BindingRevisionID: agent.Agent.CurrentBindingRevisionID,
					Authority:         conversation.RoutineAuthorityFortCloud, Trigger: input.Trigger,
					Schedule: input.Schedule, Timezone: input.Timezone, NextOccurrence: input.NextOccurrence.UTC(),
					InputSource: input.InputSource, FreshnessSeconds: input.FreshnessSeconds,
					ExpectedResult: input.ExpectedResult, ResultConversationID: input.ResultConversationID,
					ApprovalBoundary: input.ApprovalBoundary, MissingInputBehavior: input.MissingInputBehavior,
					RetryPolicy: input.RetryPolicy, CatchUpPolicy: input.CatchUpPolicy,
					LatenessPolicy: input.LatenessPolicy, CreatedAt: now},
			}
			if err := command.Validate(); err != nil {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "routine_create_invalid"})
				return
			}
			record, err := repository.CreateRoutine(request.Context(), command)
			if err != nil {
				writeOwnerRepositoryError(response, err, "routine_create_unavailable")
				return
			}
			writeBoundedOwnerJSONStatus(response, http.StatusCreated, record)
		default:
			response.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
		}
	})
}

// RoutineRunsHandler exposes the durable attempt, lease, failure, next-action,
// and result history for one exact Agent-owned Routine.
func RoutineRunsHandler(repository RoutineOwnerRepository) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, agentID, routineID, ok := ownerAgentRoutinePath(request)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "routine_runs_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "routine_runs_unavailable"})
			return
		}
		if _, _, ok := loadOwnerRoutineParents(response, request, repository, accountID, agentID, routineID,
			"routine_runs_unavailable"); !ok {
			return
		}
		runs, err := repository.ListRoutineRuns(request.Context(), accountID, routineID)
		if err != nil {
			writeOwnerRepositoryError(response, err, "routine_runs_unavailable")
			return
		}
		if runs == nil {
			runs = []ledger.RoutineRunRecord{}
		}
		for _, run := range runs {
			if run.Occurrence.AccountID != accountID || run.Occurrence.RoutineID != routineID ||
				run.Run.RoutineID != routineID || run.Run.AgentID != agentID {
				writeJSON(response, http.StatusBadGateway, map[string]string{"code": "routine_runs_unavailable"})
				return
			}
		}
		writeBoundedOwnerJSON(response, runs)
	})
}

func validRoutineCreateInput(input routineCreateRequest) bool {
	if !ownerIntentString(input.IdempotencyKey, 512) || !ownerIntentString(input.InputSource, 4096) ||
		!ownerIntentString(input.ExpectedResult, 4096) || !ownerPathIdentity(input.ResultConversationID) ||
		!ownerIntentString(input.ApprovalBoundary, 512) || !ownerIntentString(input.RetryPolicy, 512) ||
		!ownerIntentString(input.CatchUpPolicy, 512) || !ownerIntentString(input.LatenessPolicy, 512) ||
		input.FreshnessSeconds <= 0 || input.FreshnessSeconds > 365*24*60*60 {
		return false
	}
	switch input.MissingInputBehavior {
	case "skip", "needs_you", "fail":
	default:
		return false
	}
	switch input.Trigger {
	case conversation.RoutineTriggerSchedule:
		return ownerIntentString(input.Schedule, 512) && ownerIntentString(input.Timezone, 128) &&
			!input.NextOccurrence.IsZero()
	case conversation.RoutineTriggerEvent:
		return input.Schedule == "" && input.Timezone == "" && input.NextOccurrence.IsZero()
	default:
		return false
	}
}

type routineMutationRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	Action         string `json:"action"`
}

func RoutineMutationHandler(repository RoutineOwnerRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPatch {
			response.Header().Set("Allow", http.MethodPatch)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, agentID, routineID, ok := ownerAgentRoutinePath(request)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "routine_mutation_invalid"})
			return
		}
		var input routineMutationRequest
		if decodeStrictOwnerJSON(response, request, &input) != nil || input.Action != "revalidate" ||
			!ownerIntentString(input.IdempotencyKey, 512) {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "routine_mutation_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "routine_mutation_unavailable"})
			return
		}
		record, agent, ok := loadOwnerRoutineParents(response, request, repository, accountID, agentID, routineID,
			"routine_mutation_unavailable")
		if !ok {
			return
		}
		now := ownerClock(clock)
		successor := record.CurrentRevision
		successor.ID = ownerCommandID("routine-revision", accountID, agentID, routineID, input.IdempotencyKey)
		successor.Revision++
		successor.BehaviorRevisionID = agent.Agent.CurrentBehaviorRevisionID
		successor.BindingRevisionID = agent.Agent.CurrentBindingRevisionID
		successor.Authority = conversation.RoutineAuthorityFortCloud
		successor.CreatedAt = now
		command := ledger.RevalidateRoutineCommand{AccountID: accountID, RoutineID: routineID,
			IdempotencyKey: input.IdempotencyKey, Revision: successor}
		if err := command.Validate(record); err != nil {
			writeJSON(response, http.StatusConflict, map[string]string{"code": "state_conflict"})
			return
		}
		result, err := repository.RevalidateRoutine(request.Context(), command)
		if err != nil {
			writeOwnerRepositoryError(response, err, "routine_mutation_unavailable")
			return
		}
		writeBoundedOwnerJSON(response, result)
	})
}

type routineTestRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
}

func RoutineTestHandler(repository RoutineOwnerRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, agentID, routineID, ok := ownerAgentRoutinePath(request)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "routine_test_invalid"})
			return
		}
		var input routineTestRequest
		if decodeStrictOwnerJSON(response, request, &input) != nil || !ownerIntentString(input.IdempotencyKey, 512) {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "routine_test_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "routine_test_unavailable"})
			return
		}
		record, _, ok := loadOwnerRoutineParents(response, request, repository, accountID, agentID, routineID,
			"routine_test_unavailable")
		if !ok {
			return
		}
		now := ownerClock(clock)
		command := ledger.EnqueueRoutineOccurrenceCommand{
			AccountID: accountID, RoutineID: routineID, RoutineRevisionID: record.CurrentRevision.ID,
			OccurrenceID: ownerCommandID("routine-occurrence", accountID, agentID, routineID, input.IdempotencyKey),
			RunID:        ownerCommandID("routine-run", accountID, agentID, routineID, input.IdempotencyKey),
			Kind:         conversation.RoutineRunTest, ScheduledFor: now, IdempotencyKey: input.IdempotencyKey,
			ApprovalEvidenceID: ownerCommandID("approval-evaluation", accountID, agentID, routineID, input.IdempotencyKey),
			CreatedAt:          now,
		}
		if err := command.Validate(); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "routine_test_invalid"})
			return
		}
		run, err := repository.EnqueueRoutineOccurrence(request.Context(), command)
		if err != nil {
			writeOwnerRepositoryError(response, err, "routine_test_unavailable")
			return
		}
		writeBoundedOwnerJSONStatus(response, http.StatusAccepted, run)
	})
}

func ownerAgentRoutinePath(request *http.Request) (string, string, string, bool) {
	accountID, ok := AccountIDFromContext(request.Context())
	agentID := strings.TrimSpace(request.PathValue("agent_id"))
	routineID := strings.TrimSpace(request.PathValue("routine_id"))
	return accountID, agentID, routineID, ok && ownerPathIdentity(agentID) && ownerPathIdentity(routineID) &&
		len(request.URL.Query()) == 0
}

func loadOwnerRoutineParents(response http.ResponseWriter, request *http.Request, repository RoutineOwnerRepository,
	accountID, agentID, routineID, unavailableCode string) (ledger.RoutineRecord, ledger.AgentRecord, bool) {
	record, err := repository.GetRoutine(request.Context(), accountID, routineID)
	if err != nil {
		writeOwnerRepositoryError(response, err, unavailableCode)
		return ledger.RoutineRecord{}, ledger.AgentRecord{}, false
	}
	if record.Routine.AccountID != accountID || record.Routine.AgentID != agentID || record.Routine.ID != routineID {
		writeJSON(response, http.StatusNotFound, map[string]string{"code": "not_found"})
		return ledger.RoutineRecord{}, ledger.AgentRecord{}, false
	}
	agent, err := repository.GetAgent(request.Context(), accountID, agentID)
	if err != nil {
		writeOwnerRepositoryError(response, err, unavailableCode)
		return ledger.RoutineRecord{}, ledger.AgentRecord{}, false
	}
	if agent.Agent.AccountID != accountID || agent.Agent.ID != agentID {
		writeJSON(response, http.StatusNotFound, map[string]string{"code": "not_found"})
		return ledger.RoutineRecord{}, ledger.AgentRecord{}, false
	}
	return record, agent, true
}

func ownerIntentString(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && value == strings.TrimSpace(value) && len([]byte(value)) <= maximum &&
		!strings.ContainsAny(value, "\r\n\x00")
}
