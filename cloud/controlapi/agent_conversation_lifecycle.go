package controlapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

type AgentConversationCreateRepository interface {
	CreateSecondaryConversation(context.Context, ledger.CreateSecondaryConversationCommand) (ledger.AgentConversationRecord, error)
}

type AgentConversationMutationRepository interface {
	RenameAgentConversation(context.Context, ledger.RenameAgentConversationCommand) (ledger.AgentConversationRecord, error)
	SetAgentConversationState(context.Context, ledger.SetAgentConversationStateCommand) (ledger.AgentConversationRecord, error)
	SetAgentConversationPin(context.Context, ledger.SetAgentConversationPinCommand) (ledger.AgentConversationRecord, error)
}

type agentConversationCreateRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	Title          string `json:"title"`
}

// AgentConversationCreateHandler creates only a secondary Conversation. The
// verified account, stable Agent parent, identifiers, state, actor, and times
// are all allocated by the control plane rather than accepted from a client.
func AgentConversationCreateHandler(repository AgentConversationCreateRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "agent_conversation_create_method_not_allowed"})
			return
		}
		accountID, agentID, ok := ownerAgentPath(request)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_conversation_create_invalid"})
			return
		}
		var input agentConversationCreateRequest
		if err := decodeStrictOwnerJSON(response, request, &input); err != nil ||
			input.Title != strings.TrimSpace(input.Title) || input.Title == "" || len([]byte(input.Title)) > 512 {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_conversation_create_invalid"})
			return
		}
		now := time.Now().UTC()
		if clock != nil {
			now = clock().UTC()
		}
		conversationID := ownerCommandID("conversation", accountID, agentID, input.IdempotencyKey)
		actorID := "human:" + accountID
		command := ledger.CreateSecondaryConversationCommand{
			IdempotencyKey: input.IdempotencyKey,
			AccountID:      accountID,
			AgentID:        agentID,
			Conversation: conversation.Conversation{
				ID: conversationID, Title: input.Title, State: conversation.ConversationOpen,
				CreatedAt: now, UpdatedAt: now,
			},
			Link: conversation.AgentConversation{
				AgentID: agentID, ConversationID: conversationID,
				Kind: conversation.AgentConversationSecondary, CreatedAt: now,
			},
			CreatedBy: actorID,
		}
		if err := command.Validate(); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_conversation_create_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "agent_conversation_create_unavailable"})
			return
		}
		record, err := repository.CreateSecondaryConversation(request.Context(), command)
		if err != nil {
			writeOwnerRepositoryError(response, err, "agent_conversation_create_unavailable")
			return
		}
		writeBoundedOwnerJSONStatus(response, http.StatusCreated, record)
	})
}

type agentConversationMutationRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	Action         string `json:"action"`
	ExpectedTitle  string `json:"expected_title,omitempty"`
	Title          string `json:"title,omitempty"`
}

// AgentConversationMutationHandler exposes one closed mutation vocabulary for
// secondary Conversations. Repositories enforce the canonical Home guard and
// the complete account -> Agent -> Conversation parent chain.
func AgentConversationMutationHandler(repository AgentConversationMutationRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPatch {
			response.Header().Set("Allow", http.MethodPatch)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "agent_conversation_mutation_method_not_allowed"})
			return
		}
		accountID, agentID, conversationID, ok := ownerAgentConversationPath(request)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_conversation_mutation_invalid"})
			return
		}
		var input agentConversationMutationRequest
		if err := decodeStrictOwnerJSON(response, request, &input); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_conversation_mutation_invalid"})
			return
		}
		now := time.Now().UTC()
		if clock != nil {
			now = clock().UTC()
		}
		actorID := "human:" + accountID
		var record ledger.AgentConversationRecord
		var err error
		switch input.Action {
		case "rename":
			command := ledger.RenameAgentConversationCommand{
				IdempotencyKey: input.IdempotencyKey, AccountID: accountID, AgentID: agentID,
				ConversationID: conversationID, ExpectedTitle: input.ExpectedTitle, Title: input.Title,
				ChangedBy: actorID, ChangedAt: now,
			}
			if command.Validate() != nil {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_conversation_mutation_invalid"})
				return
			}
			if repository != nil {
				record, err = repository.RenameAgentConversation(request.Context(), command)
			}
		case "pin", "unpin":
			if input.ExpectedTitle != "" || input.Title != "" {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_conversation_mutation_invalid"})
				return
			}
			pinned := input.Action == "pin"
			command := ledger.SetAgentConversationPinCommand{
				IdempotencyKey: input.IdempotencyKey, AccountID: accountID, AgentID: agentID,
				ConversationID: conversationID, ExpectedPinned: !pinned, Pinned: pinned,
				ChangedBy: actorID, ChangedAt: now,
			}
			if command.Validate() != nil {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_conversation_mutation_invalid"})
				return
			}
			if repository != nil {
				record, err = repository.SetAgentConversationPin(request.Context(), command)
			}
		case "archive", "reopen":
			if input.ExpectedTitle != "" || input.Title != "" {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_conversation_mutation_invalid"})
				return
			}
			expected, desired := conversation.ConversationOpen, conversation.ConversationArchived
			if input.Action == "reopen" {
				expected, desired = desired, expected
			}
			command := ledger.SetAgentConversationStateCommand{
				IdempotencyKey: input.IdempotencyKey, AccountID: accountID, AgentID: agentID,
				ConversationID: conversationID, ExpectedState: expected, State: desired,
				ChangedBy: actorID, ChangedAt: now,
			}
			if command.Validate() != nil {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_conversation_mutation_invalid"})
				return
			}
			if repository != nil {
				record, err = repository.SetAgentConversationState(request.Context(), command)
			}
		default:
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_conversation_mutation_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "agent_conversation_mutation_unavailable"})
			return
		}
		if err != nil {
			writeOwnerRepositoryError(response, err, "agent_conversation_mutation_unavailable")
			return
		}
		writeBoundedOwnerJSON(response, record)
	})
}
