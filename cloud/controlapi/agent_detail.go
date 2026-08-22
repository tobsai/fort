package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

type AgentReader interface {
	GetAgent(context.Context, string, string) (ledger.AgentRecord, error)
}

type AgentConversationReader interface {
	ListAgentConversations(context.Context, string, string) ([]ledger.AgentConversationRecord, error)
}

func AgentDetailHandler(repository AgentReader) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, agentID, ok := ownerAgentPath(request)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_read_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "agent_read_unavailable"})
			return
		}
		record, err := repository.GetAgent(request.Context(), accountID, agentID)
		if err != nil {
			writeOwnerRepositoryError(response, err, "agent_read_unavailable")
			return
		}
		writeBoundedOwnerJSON(response, record)
	})
}

func AgentConversationsHandler(repository AgentConversationReader) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, agentID, ok := ownerAgentPath(request)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_conversation_list_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "agent_conversation_list_unavailable"})
			return
		}
		records, err := repository.ListAgentConversations(request.Context(), accountID, agentID)
		if err != nil {
			writeOwnerRepositoryError(response, err, "agent_conversation_list_unavailable")
			return
		}
		if records == nil {
			records = []ledger.AgentConversationRecord{}
		}
		writeBoundedOwnerJSON(response, records)
	})
}

func AgentCanonicalConversationHandler(repository AgentConversationReader) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, agentID, ok := ownerAgentPath(request)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_canonical_conversation_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "agent_canonical_conversation_unavailable"})
			return
		}
		records, err := repository.ListAgentConversations(request.Context(), accountID, agentID)
		if err != nil {
			writeOwnerRepositoryError(response, err, "agent_canonical_conversation_unavailable")
			return
		}
		for _, record := range records {
			if record.Link.Kind == conversation.AgentConversationCanonical {
				writeBoundedOwnerJSON(response, record)
				return
			}
		}
		writeJSON(response, http.StatusNotFound, map[string]string{"code": "not_found"})
	})
}

func ownerAgentPath(request *http.Request) (string, string, bool) {
	accountID, ok := AccountIDFromContext(request.Context())
	agentID := strings.TrimSpace(request.PathValue("agent_id"))
	return accountID, agentID, ok && agentID != "" && len([]byte(agentID)) <= 512 && len(request.URL.Query()) == 0
}

func prepareOwnerJSON(response http.ResponseWriter) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "application/json")
}

func writeBoundedOwnerJSON(response http.ResponseWriter, value any) {
	payload, err := json.Marshal(value)
	if err != nil || len(payload)+1 > MaximumFunctionBodyBytes {
		writeJSON(response, http.StatusBadGateway, map[string]string{"code": "response_limit"})
		return
	}
	payload = append(payload, '\n')
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(payload)
}

func writeOwnerRepositoryError(response http.ResponseWriter, err error, unavailableCode string) {
	switch {
	case errors.Is(err, ledger.ErrNotFound):
		writeJSON(response, http.StatusNotFound, map[string]string{"code": "not_found"})
	case errors.Is(err, ledger.ErrStateConflict), errors.Is(err, ledger.ErrRevisionConflict),
		errors.Is(err, ledger.ErrIdempotencyConflict), errors.Is(err, ledger.ErrRoutineNeedsRevalidation),
		errors.Is(err, ledger.ErrRoutineRunTerminal):
		writeJSON(response, http.StatusConflict, map[string]string{"code": "state_conflict"})
	default:
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": unavailableCode})
	}
}
