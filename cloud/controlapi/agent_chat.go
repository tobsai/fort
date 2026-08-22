package controlapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tobsai/fort/core/ledger"
)

const maximumDirectTurnWindow = 24 * time.Hour

type AgentDirectChatRepository interface {
	ReadAgentConversationRepository
	SendAgentTurnRepository
	RetryAgentTargetRepository
	CancelAgentTargetRepository
}

type ReadAgentConversationRepository interface {
	ReadAgentConversation(ctx context.Context, accountID, agentID, conversationID string) (ledger.AgentConversationProjection, error)
}

type SendAgentTurnRepository interface {
	SendAgentTurn(ctx context.Context, command ledger.SendAgentTurnCommand) (ledger.AgentTurnDispatch, error)
}

type RetryAgentTargetRepository interface {
	RetryAgentTarget(ctx context.Context, command ledger.RetryAgentTargetCommand) (ledger.AgentConversationTarget, error)
}

type CancelAgentTargetRepository interface {
	CancelAgentTarget(ctx context.Context, command ledger.CancelAgentTargetCommand) (ledger.AgentConversationTarget, error)
}

// AgentConversationProjectionHandler returns the complete durable projection
// for one Conversation only after verifying its stable Agent parent.
func AgentConversationProjectionHandler(repository ReadAgentConversationRepository) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, agentID, conversationID, ok := ownerAgentConversationPath(request)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_conversation_read_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "agent_conversation_read_unavailable"})
			return
		}
		projection, err := repository.ReadAgentConversation(request.Context(), accountID, agentID, conversationID)
		if err != nil {
			writeOwnerRepositoryError(response, err, "agent_conversation_read_unavailable")
			return
		}
		if projection.Messages == nil {
			projection.Messages = []ledger.AgentConversationMessage{}
		}
		if projection.Turns == nil {
			projection.Turns = []ledger.AgentConversationTurn{}
		}
		if projection.Targets == nil {
			projection.Targets = []ledger.AgentConversationTarget{}
		}
		writeBoundedOwnerJSON(response, projection)
	})
}

type agentTurnRequest struct {
	IdempotencyKey string    `json:"idempotency_key"`
	ClientTurnID   string    `json:"client_turn_id"`
	Text           string    `json:"text"`
	HardDeadline   time.Time `json:"hard_deadline"`
}

// AgentConversationTurnsHandler creates one direct human message, frozen
// context, Turn, and exact current-Agent target in the repository transaction.
func AgentConversationTurnsHandler(repository SendAgentTurnRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, agentID, conversationID, ok := ownerAgentConversationPath(request)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_turn_invalid"})
			return
		}
		var input agentTurnRequest
		if err := decodeStrictOwnerJSON(response, request, &input); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_turn_invalid"})
			return
		}
		now := time.Now().UTC()
		if clock != nil {
			now = clock().UTC()
		}
		deadline := input.HardDeadline.UTC()
		if !deadline.After(now) || deadline.After(now.Add(maximumDirectTurnWindow)) {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_turn_invalid"})
			return
		}
		seed := []string{accountID, agentID, conversationID, input.IdempotencyKey}
		actorID := "human:" + accountID
		command := ledger.SendAgentTurnCommand{
			IdempotencyKey: input.IdempotencyKey,
			AccountID:      accountID, AgentID: agentID, ConversationID: conversationID,
			TurnID:            ownerCommandID("turn", seed...),
			ClientTurnID:      input.ClientTurnID,
			ContextManifestID: ownerCommandID("context", seed...),
			DelegationGrantID: ownerCommandID("grant", seed...),
			TargetID:          ownerCommandID("target", seed...),
			RunID:             ownerCommandID("run", seed...),
			HumanID:           actorID,
			Body:              input.Text,
			CreatedBy:         actorID,
			CreatedAt:         now,
			HardDeadline:      deadline,
		}
		if err := command.Validate(); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_turn_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "agent_turn_unavailable"})
			return
		}
		dispatch, err := repository.SendAgentTurn(request.Context(), command)
		if err != nil {
			writeOwnerRepositoryError(response, err, "agent_turn_unavailable")
			return
		}
		writeBoundedOwnerJSONStatus(response, http.StatusAccepted, dispatch)
	})
}

type agentTargetRetryRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
}

// AgentTargetRetryHandler requeues the same exact target. The repository
// retains its original Behavior and Binding Revision pins.
func AgentTargetRetryHandler(repository RetryAgentTargetRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, agentID, conversationID, targetID, ok := ownerAgentTargetPath(request)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_target_retry_invalid"})
			return
		}
		var input agentTargetRetryRequest
		if err := decodeStrictOwnerJSON(response, request, &input); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_target_retry_invalid"})
			return
		}
		now := time.Now().UTC()
		if clock != nil {
			now = clock().UTC()
		}
		command := ledger.RetryAgentTargetCommand{
			IdempotencyKey: input.IdempotencyKey,
			AccountID:      accountID, AgentID: agentID, ConversationID: conversationID,
			TargetID: targetID, RetriedBy: "human:" + accountID, RetriedAt: now,
		}
		if err := command.Validate(); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_target_retry_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "agent_target_retry_unavailable"})
			return
		}
		target, err := repository.RetryAgentTarget(request.Context(), command)
		if err != nil {
			writeOwnerRepositoryError(response, err, "agent_target_retry_unavailable")
			return
		}
		writeBoundedOwnerJSONStatus(response, http.StatusAccepted, target)
	})
}

// AgentTargetCancelHandler durably requests cancellation for the exact target.
// The repository preserves its parent chain and accepted revision pins.
func AgentTargetCancelHandler(repository CancelAgentTargetRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, agentID, conversationID, targetID, ok := ownerAgentTargetPath(request)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_target_cancel_invalid"})
			return
		}
		var input agentTargetRetryRequest
		if err := decodeStrictOwnerJSON(response, request, &input); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_target_cancel_invalid"})
			return
		}
		now := time.Now().UTC()
		if clock != nil {
			now = clock().UTC()
		}
		command := ledger.CancelAgentTargetCommand{
			IdempotencyKey: input.IdempotencyKey,
			AccountID:      accountID, AgentID: agentID, ConversationID: conversationID,
			TargetID: targetID, CanceledBy: "human:" + accountID, CanceledAt: now,
		}
		if err := command.Validate(); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_target_cancel_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "agent_target_cancel_unavailable"})
			return
		}
		target, err := repository.CancelAgentTarget(request.Context(), command)
		if err != nil {
			writeOwnerRepositoryError(response, err, "agent_target_cancel_unavailable")
			return
		}
		writeBoundedOwnerJSONStatus(response, http.StatusAccepted, target)
	})
}

func ownerAgentConversationPath(request *http.Request) (string, string, string, bool) {
	accountID, ok := AccountIDFromContext(request.Context())
	agentID := strings.TrimSpace(request.PathValue("agent_id"))
	conversationID := strings.TrimSpace(request.PathValue("conversation_id"))
	valid := ok && ownerPathIdentity(agentID) && ownerPathIdentity(conversationID) && len(request.URL.Query()) == 0
	return accountID, agentID, conversationID, valid
}

func ownerAgentTargetPath(request *http.Request) (string, string, string, string, bool) {
	accountID, agentID, conversationID, ok := ownerAgentConversationPath(request)
	targetID := strings.TrimSpace(request.PathValue("target_id"))
	return accountID, agentID, conversationID, targetID, ok && ownerPathIdentity(targetID)
}

func ownerPathIdentity(value string) bool {
	return value != "" && len([]byte(value)) <= 512 && !strings.ContainsAny(value, "\r\n\x00")
}

func decodeStrictOwnerJSON(response http.ResponseWriter, request *http.Request, destination any) error {
	request.Body = http.MaxBytesReader(response, request.Body, MaximumFunctionBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("request contains trailing JSON")
	}
	return nil
}

func ownerCommandID(kind string, components ...string) string {
	payload, _ := json.Marshal(components)
	digest := sha256.Sum256(payload)
	return kind + ":v2:" + hex.EncodeToString(digest[:16])
}

func writeBoundedOwnerJSONStatus(response http.ResponseWriter, status int, value any) {
	payload, err := json.Marshal(value)
	if err != nil || len(payload)+1 > MaximumFunctionBodyBytes {
		writeJSON(response, http.StatusBadGateway, map[string]string{"code": "response_limit"})
		return
	}
	payload = append(payload, '\n')
	response.WriteHeader(status)
	_, _ = response.Write(payload)
}
