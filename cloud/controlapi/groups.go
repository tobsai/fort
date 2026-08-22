package controlapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

// GroupLister is the minimal owner projection for the stable Group roster.
// Account identity always comes from a verified gateway assertion.
type GroupLister interface {
	ListGroups(context.Context, string, conversation.ConversationState) ([]ledger.GroupRecord, error)
}

func GroupsHandler(repository GroupLister) http.Handler {
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
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_list_invalid"})
				return
			}
		}
		state := conversation.ConversationState(query.Get("state"))
		if state == "" {
			state = conversation.ConversationOpen
		}
		if state != conversation.ConversationOpen && state != conversation.ConversationArchived {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_list_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "group_list_unavailable"})
			return
		}
		records, err := repository.ListGroups(request.Context(), accountID, state)
		if err != nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "group_list_unavailable"})
			return
		}
		if records == nil {
			records = []ledger.GroupRecord{}
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
