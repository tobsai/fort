package controlapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

// AgentLister is the read-only owner projection required by the v2 Agent
// roster. The account always comes from the verified gateway assertion.
type AgentLister interface {
	ListAgents(context.Context, string, conversation.AgentState) ([]ledger.AgentRecord, error)
}

func AgentsHandler(repository AgentLister) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		response.Header().Set("Content-Type", "application/json")
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, ok := AccountIDFromContext(request.Context())
		if !ok {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"code": "service_assertion_required"})
			return
		}
		query := request.URL.Query()
		for key := range query {
			if key != "state" || len(query[key]) != 1 {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_list_invalid"})
				return
			}
		}
		state := conversation.AgentState(query.Get("state"))
		if state == "" {
			state = conversation.AgentOpen
		}
		if state != conversation.AgentOpen && state != conversation.AgentArchived {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_list_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "agent_list_unavailable"})
			return
		}
		records, err := repository.ListAgents(request.Context(), accountID, state)
		if err != nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "agent_list_unavailable"})
			return
		}
		if records == nil {
			records = []ledger.AgentRecord{}
		}
		payload, err := json.Marshal(records)
		if err != nil || len(payload)+1 > MaximumFunctionBodyBytes {
			writeJSON(response, http.StatusBadGateway, map[string]string{"code": "response_limit"})
			return
		}
		payload = append(payload, '\n')
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(payload)
	})
}
