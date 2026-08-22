package controlapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/tobsai/fort/core/ledger"
)

type HandoffCreateRepository interface {
	CreateHumanHandoff(context.Context, ledger.CreateHumanHandoffCommand) (ledger.HandoffRecord, error)
}

type HandoffListRepository interface {
	ListHandoffs(context.Context, string) ([]ledger.HandoffRecord, error)
}

type HandoffDetailRepository interface {
	GetHandoff(context.Context, string, string) (ledger.HandoffRecord, error)
}

type HandoffCancelRepository interface {
	CancelHandoff(context.Context, ledger.CancelHandoffCommand) (ledger.HandoffRecord, error)
}

// HumanHandoffRepository is the bounded owner-facing persistence contract.
// Execution identity and immutable revision evidence are resolved below this
// seam; handlers accept only human intent and an idempotency key.
type HumanHandoffRepository interface {
	HandoffCreateRepository
	HandoffListRepository
	HandoffDetailRepository
	HandoffCancelRepository
}

type handoffCreateRequest struct {
	IdempotencyKey       string    `json:"idempotency_key"`
	SourceConversationID string    `json:"source_conversation_id"`
	SourceMessageID      string    `json:"source_message_id"`
	RecipientAgentID     string    `json:"recipient_agent_id"`
	ContextMessageIDs    []string  `json:"context_message_ids"`
	RequestedResult      string    `json:"requested_result"`
	ReplyToMessageID     string    `json:"reply_to_message_id,omitempty"`
	HardDeadline         time.Time `json:"hard_deadline"`
}

func HandoffCreateHandler(repository HandoffCreateRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, ok := AccountIDFromContext(request.Context())
		if !ok || len(request.URL.Query()) != 0 {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "handoff_create_invalid"})
			return
		}
		var input handoffCreateRequest
		if decodeStrictOwnerJSON(response, request, &input) != nil || !validHandoffCreateInput(input) {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "handoff_create_invalid"})
			return
		}
		now := ownerClock(clock)
		deadline := input.HardDeadline.UTC()
		if !deadline.After(now) || deadline.After(now.Add(maximumDirectTurnWindow)) {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "handoff_create_invalid"})
			return
		}
		seed := []string{accountID, input.IdempotencyKey}
		command := ledger.CreateHumanHandoffCommand{
			IdempotencyKey:        input.IdempotencyKey,
			AccountID:             accountID,
			SourceConversationID:  input.SourceConversationID,
			SourceMessageID:       input.SourceMessageID,
			RecipientAgentID:      input.RecipientAgentID,
			ContextMessageIDs:     append([]string{}, input.ContextMessageIDs...),
			RequestedResult:       input.RequestedResult,
			ReplyToMessageID:      input.ReplyToMessageID,
			HardDeadline:          deadline,
			HandoffID:             ownerCommandID("handoff", seed...),
			TargetID:              ownerCommandID("target", seed...),
			RootDelegationGrantID: ownerCommandID("grant", seed...),
			CreatedByID:           "human:" + accountID,
			CreatedAt:             now,
		}
		if err := command.Validate(); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "handoff_create_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "handoff_create_unavailable"})
			return
		}
		record, err := repository.CreateHumanHandoff(request.Context(), command)
		if err != nil {
			writeOwnerRepositoryError(response, err, "handoff_create_unavailable")
			return
		}
		writeBoundedOwnerJSONStatus(response, http.StatusAccepted, record)
	})
}

func validHandoffCreateInput(input handoffCreateRequest) bool {
	if !ownerPathIdentity(input.SourceConversationID) || !ownerPathIdentity(input.SourceMessageID) ||
		!ownerPathIdentity(input.RecipientAgentID) || strings.TrimSpace(input.IdempotencyKey) == "" ||
		strings.TrimSpace(input.RequestedResult) == "" {
		return false
	}
	if input.ReplyToMessageID != "" && !ownerPathIdentity(input.ReplyToMessageID) {
		return false
	}
	for _, messageID := range input.ContextMessageIDs {
		if !ownerPathIdentity(messageID) || strings.TrimSpace(messageID) != messageID {
			return false
		}
	}
	return true
}

func HandoffsHandler(repository HandoffListRepository) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, ok := AccountIDFromContext(request.Context())
		if !ok || len(request.URL.Query()) != 0 {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "handoff_list_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "handoff_list_unavailable"})
			return
		}
		records, err := repository.ListHandoffs(request.Context(), accountID)
		if err != nil {
			writeOwnerRepositoryError(response, err, "handoff_list_unavailable")
			return
		}
		if records == nil {
			records = []ledger.HandoffRecord{}
		}
		writeBoundedOwnerJSON(response, records)
	})
}

func HandoffDetailHandler(repository HandoffDetailRepository) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, handoffID, ok := ownerHandoffPath(request)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "handoff_read_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "handoff_read_unavailable"})
			return
		}
		record, err := repository.GetHandoff(request.Context(), accountID, handoffID)
		if err != nil {
			writeOwnerRepositoryError(response, err, "handoff_read_unavailable")
			return
		}
		writeBoundedOwnerJSON(response, record)
	})
}

type handoffCancelRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
}

func HandoffCancelHandler(repository HandoffCancelRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, handoffID, ok := ownerHandoffPath(request)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "handoff_cancel_invalid"})
			return
		}
		var input handoffCancelRequest
		if decodeStrictOwnerJSON(response, request, &input) != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "handoff_cancel_invalid"})
			return
		}
		command := ledger.CancelHandoffCommand{
			IdempotencyKey: input.IdempotencyKey,
			AccountID:      accountID,
			HandoffID:      handoffID,
			CanceledBy:     "human:" + accountID,
			CanceledAt:     ownerClock(clock),
		}
		if err := command.Validate(); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "handoff_cancel_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "handoff_cancel_unavailable"})
			return
		}
		record, err := repository.CancelHandoff(request.Context(), command)
		if err != nil {
			writeOwnerRepositoryError(response, err, "handoff_cancel_unavailable")
			return
		}
		writeBoundedOwnerJSONStatus(response, http.StatusAccepted, record)
	})
}

func ownerHandoffPath(request *http.Request) (string, string, bool) {
	accountID, ok := AccountIDFromContext(request.Context())
	handoffID := strings.TrimSpace(request.PathValue("handoff_id"))
	return accountID, handoffID, ok && ownerPathIdentity(handoffID) && len(request.URL.Query()) == 0
}
