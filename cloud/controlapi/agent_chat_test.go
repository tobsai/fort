package controlapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/ledger"
)

func TestAgentConversationProjectionHandlerScopesTheFullParentChain(t *testing.T) {
	t.Parallel()
	repository := &fakeAgentChatRepository{projection: ledger.AgentConversationProjection{
		Messages: []ledger.AgentConversationMessage{}, Turns: []ledger.AgentConversationTurn{}, Targets: []ledger.AgentConversationTarget{},
	}}
	verifier, token := serviceAuthorizationFixture(t, time.Now().UTC(), "", "owner.agent_conversations.read")
	request := httptest.NewRequest(http.MethodGet, "/api/v2/agents/agent:researcher/conversations/conversation:home", nil)
	request.SetPathValue("agent_id", "agent:researcher")
	request.SetPathValue("conversation_id", "conversation:home")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(
		verifier,
		"owner.agent_conversations.read",
		controlapi.AgentConversationProjectionHandler(repository),
	).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || repository.accountID != testOwnerAccountID ||
		repository.agentID != "agent:researcher" || repository.conversationID != "conversation:home" {
		t.Fatalf("status/scope = %d %q/%q/%q", recorder.Code, repository.accountID, repository.agentID, repository.conversationID)
	}
	var projection ledger.AgentConversationProjection
	if err := json.Unmarshal(recorder.Body.Bytes(), &projection); err != nil || projection.Messages == nil || projection.Turns == nil || projection.Targets == nil {
		t.Fatalf("projection = %+v, err=%v", projection, err)
	}
}

func TestAgentConversationTurnsHandlerBuildsOnlyServerOwnedExecutionIdentity(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 16, 30, 0, 0, time.UTC)
	deadline := now.Add(10 * time.Minute)
	body := `{"idempotency_key":"send:one","client_turn_id":"client-turn:one","text":"Compare the evidence.","hard_deadline":"` + deadline.Format(time.RFC3339Nano) + `"}`
	repository := &fakeAgentChatRepository{dispatch: ledger.AgentTurnDispatch{Created: true}}
	verifier, token := serviceAuthorizationFixture(t, now, body, "owner.agent_turns.send")
	request := httptest.NewRequest(http.MethodPost, "/api/v2/agents/agent:researcher/conversations/conversation:home/turns", strings.NewReader(body))
	request.SetPathValue("agent_id", "agent:researcher")
	request.SetPathValue("conversation_id", "conversation:home")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(
		verifier,
		"owner.agent_turns.send",
		controlapi.AgentConversationTurnsHandler(repository, func() time.Time { return now }),
	).ServeHTTP(recorder, request)

	command := repository.send
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", recorder.Code, recorder.Body.String())
	}
	if command.AccountID != testOwnerAccountID || command.AgentID != "agent:researcher" ||
		command.ConversationID != "conversation:home" || command.IdempotencyKey != "send:one" ||
		command.ClientTurnID != "client-turn:one" || command.Body != "Compare the evidence." ||
		command.CreatedAt != now || command.HardDeadline != deadline || command.HumanID != "human:"+testOwnerAccountID ||
		command.TurnID == "" || command.ContextManifestID == "" || command.DelegationGrantID == "" ||
		command.TargetID == "" || command.RunID == "" {
		t.Fatalf("server command = %+v", command)
	}
	if command.CreatedBy != command.HumanID {
		t.Fatalf("created_by = %q, want authenticated human %q", command.CreatedBy, command.HumanID)
	}

	second := &fakeAgentChatRepository{dispatch: ledger.AgentTurnDispatch{Created: false}}
	verifier, token = serviceAuthorizationFixture(t, now, body, "owner.agent_turns.send")
	request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	request.SetPathValue("agent_id", "agent:researcher")
	request.SetPathValue("conversation_id", "conversation:home")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder = httptest.NewRecorder()
	controlapi.RequireServiceAssertion(verifier, "owner.agent_turns.send", controlapi.AgentConversationTurnsHandler(second, func() time.Time { return now })).ServeHTTP(recorder, request)
	if second.send.TurnID != command.TurnID || second.send.TargetID != command.TargetID || second.send.RunID != command.RunID {
		t.Fatalf("same command did not derive stable internal ids: first=%+v second=%+v", command, second.send)
	}
}

func TestAgentConversationTurnsHandlerRejectsUnknownIdentityAndInvalidDeadline(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 16, 30, 0, 0, time.UTC)
	tests := []string{
		`{"idempotency_key":"send:one","client_turn_id":"turn:one","text":"hello","hard_deadline":"2026-08-21T16:40:00Z","provider":"openclaw"}`,
		`{"idempotency_key":"send:one","client_turn_id":"turn:one","text":"hello","hard_deadline":"2026-08-21T16:29:59Z"}`,
		`{"idempotency_key":"send:one","client_turn_id":"turn:one","text":"hello","hard_deadline":"2026-08-22T16:30:01Z"}`,
	}
	for _, body := range tests {
		repository := &fakeAgentChatRepository{}
		verifier, token := serviceAuthorizationFixture(t, now, body, "owner.agent_turns.send")
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		request.SetPathValue("agent_id", "agent:researcher")
		request.SetPathValue("conversation_id", "conversation:home")
		request.Header.Set(controlapi.ServiceAssertionHeader, token)
		recorder := httptest.NewRecorder()
		controlapi.RequireServiceAssertion(verifier, "owner.agent_turns.send", controlapi.AgentConversationTurnsHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest || repository.send.AgentID != "" {
			t.Fatalf("body %s status/command = %d/%+v, want rejected before repository", body, recorder.Code, repository.send)
		}
	}
}

func TestAgentTargetRetryHandlerKeepsTheExactTargetAndParents(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 16, 30, 0, 0, time.UTC)
	body := `{"idempotency_key":"retry:one"}`
	repository := &fakeAgentChatRepository{target: ledger.AgentConversationTarget{ID: "target:one"}}
	verifier, token := serviceAuthorizationFixture(t, now, body, "owner.agent_targets.retry")
	request := httptest.NewRequest(http.MethodPost, "/api/v2/agents/agent:researcher/conversations/conversation:home/targets/target:one/retry", strings.NewReader(body))
	request.SetPathValue("agent_id", "agent:researcher")
	request.SetPathValue("conversation_id", "conversation:home")
	request.SetPathValue("target_id", "target:one")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(
		verifier,
		"owner.agent_targets.retry",
		controlapi.AgentTargetRetryHandler(repository, func() time.Time { return now }),
	).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted || repository.retry.AccountID != testOwnerAccountID ||
		repository.retry.AgentID != "agent:researcher" || repository.retry.ConversationID != "conversation:home" ||
		repository.retry.TargetID != "target:one" || repository.retry.RetriedBy != "human:"+testOwnerAccountID ||
		repository.retry.RetriedAt != now {
		t.Fatalf("status/retry = %d/%+v", recorder.Code, repository.retry)
	}
}

func TestAgentTargetCancelHandlerKeepsTheExactTargetAndParents(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 16, 30, 0, 0, time.UTC)
	body := `{"idempotency_key":"cancel:one"}`
	repository := &fakeAgentChatRepository{target: ledger.AgentConversationTarget{ID: "target:one"}}
	verifier, token := serviceAuthorizationFixture(t, now, body, "owner.agent_targets.cancel")
	request := httptest.NewRequest(http.MethodPost, "/api/v2/agents/agent:researcher/conversations/conversation:home/targets/target:one/cancel", strings.NewReader(body))
	request.SetPathValue("agent_id", "agent:researcher")
	request.SetPathValue("conversation_id", "conversation:home")
	request.SetPathValue("target_id", "target:one")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(
		verifier,
		"owner.agent_targets.cancel",
		controlapi.AgentTargetCancelHandler(repository, func() time.Time { return now }),
	).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted || repository.cancel.AccountID != testOwnerAccountID ||
		repository.cancel.AgentID != "agent:researcher" || repository.cancel.ConversationID != "conversation:home" ||
		repository.cancel.TargetID != "target:one" || repository.cancel.CanceledBy != "human:"+testOwnerAccountID ||
		repository.cancel.CanceledAt != now {
		t.Fatalf("status/cancel = %d/%+v", recorder.Code, repository.cancel)
	}
}

func TestAgentChatHandlersMapForeignChildrenAndWrongState(t *testing.T) {
	t.Parallel()
	for name, err := range map[string]error{"foreign": ledger.ErrNotFound, "wrong-state": ledger.ErrStateConflict} {
		t.Run(name, func(t *testing.T) {
			repository := &fakeAgentChatRepository{err: err}
			verifier, token := serviceAuthorizationFixture(t, time.Now().UTC(), "", "owner.agent_conversations.read")
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.SetPathValue("agent_id", "agent:researcher")
			request.SetPathValue("conversation_id", "conversation:child")
			request.Header.Set(controlapi.ServiceAssertionHeader, token)
			recorder := httptest.NewRecorder()
			controlapi.RequireServiceAssertion(verifier, "owner.agent_conversations.read", controlapi.AgentConversationProjectionHandler(repository)).ServeHTTP(recorder, request)
			want := http.StatusNotFound
			if err == ledger.ErrStateConflict {
				want = http.StatusConflict
			}
			if recorder.Code != want {
				t.Fatalf("status = %d, want %d", recorder.Code, want)
			}
		})
	}
}

const testOwnerAccountID = "4af424a4-d81a-47d5-a495-400868883b86"

type fakeAgentChatRepository struct {
	projection     ledger.AgentConversationProjection
	dispatch       ledger.AgentTurnDispatch
	target         ledger.AgentConversationTarget
	err            error
	accountID      string
	agentID        string
	conversationID string
	send           ledger.SendAgentTurnCommand
	retry          ledger.RetryAgentTargetCommand
	cancel         ledger.CancelAgentTargetCommand
}

func (repository *fakeAgentChatRepository) ReadAgentConversation(_ context.Context, accountID, agentID, conversationID string) (ledger.AgentConversationProjection, error) {
	repository.accountID, repository.agentID, repository.conversationID = accountID, agentID, conversationID
	return repository.projection, repository.err
}

func (repository *fakeAgentChatRepository) SendAgentTurn(_ context.Context, command ledger.SendAgentTurnCommand) (ledger.AgentTurnDispatch, error) {
	repository.send = command
	return repository.dispatch, repository.err
}

func (repository *fakeAgentChatRepository) RetryAgentTarget(_ context.Context, command ledger.RetryAgentTargetCommand) (ledger.AgentConversationTarget, error) {
	repository.retry = command
	return repository.target, repository.err
}

func (repository *fakeAgentChatRepository) CancelAgentTarget(_ context.Context, command ledger.CancelAgentTargetCommand) (ledger.AgentConversationTarget, error) {
	repository.cancel = command
	return repository.target, repository.err
}
