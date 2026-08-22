package controlapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestAgentDetailHandlerUsesVerifiedAccountAndExactPathAgent(t *testing.T) {
	t.Parallel()
	repository := &fakeAgentDetailRepository{agent: ledger.AgentRecord{Agent: conversation.Agent{ID: "agent:researcher"}}}
	verifier, token := serviceAuthorizationFixture(t, time.Now().UTC(), "", "owner.agents.read")
	request := httptest.NewRequest(http.MethodGet, "/api/v2/agents/agent:researcher", nil)
	request.SetPathValue("agent_id", "agent:researcher")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(verifier, "owner.agents.read", controlapi.AgentDetailHandler(repository)).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || repository.accountID != "4af424a4-d81a-47d5-a495-400868883b86" || repository.agentID != "agent:researcher" {
		t.Fatalf("response/scope = %d %q/%q", recorder.Code, repository.accountID, repository.agentID)
	}
}

func TestAgentConversationsHandlerVerifiesParentAndReturnsHomeFirst(t *testing.T) {
	t.Parallel()
	repository := &fakeAgentDetailRepository{conversations: []ledger.AgentConversationRecord{
		{Conversation: conversation.Conversation{ID: "conversation:home"}, Link: conversation.AgentConversation{AgentID: "agent:researcher", Kind: conversation.AgentConversationCanonical}},
		{Conversation: conversation.Conversation{ID: "conversation:pinned"}, Link: conversation.AgentConversation{AgentID: "agent:researcher", Kind: conversation.AgentConversationSecondary}},
	}}
	verifier, token := serviceAuthorizationFixture(t, time.Now().UTC(), "", "owner.agent_conversations.list")
	request := httptest.NewRequest(http.MethodGet, "/api/v2/agents/agent:researcher/conversations", nil)
	request.SetPathValue("agent_id", "agent:researcher")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(verifier, "owner.agent_conversations.list", controlapi.AgentConversationsHandler(repository)).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || repository.agentID != "agent:researcher" {
		t.Fatalf("response/parent = %d %q", recorder.Code, repository.agentID)
	}
	var records []ledger.AgentConversationRecord
	if err := json.Unmarshal(recorder.Body.Bytes(), &records); err != nil || len(records) != 2 || records[0].Link.Kind != conversation.AgentConversationCanonical {
		t.Fatalf("records = %+v, err=%v", records, err)
	}
}

func TestAgentCanonicalConversationHandlerReturnsOnlyPermanentHome(t *testing.T) {
	t.Parallel()
	repository := &fakeAgentDetailRepository{conversations: []ledger.AgentConversationRecord{
		{Conversation: conversation.Conversation{ID: "conversation:home"}, Link: conversation.AgentConversation{AgentID: "agent:researcher", Kind: conversation.AgentConversationCanonical}},
		{Conversation: conversation.Conversation{ID: "conversation:other"}, Link: conversation.AgentConversation{AgentID: "agent:researcher", Kind: conversation.AgentConversationSecondary}},
	}}
	verifier, token := serviceAuthorizationFixture(t, time.Now().UTC(), "", "owner.agent_conversations.canonical")
	request := httptest.NewRequest(http.MethodGet, "/api/v2/agents/agent:researcher/conversations/canonical", nil)
	request.SetPathValue("agent_id", "agent:researcher")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(
		verifier,
		"owner.agent_conversations.canonical",
		controlapi.AgentCanonicalConversationHandler(repository),
	).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var record ledger.AgentConversationRecord
	if err := json.Unmarshal(recorder.Body.Bytes(), &record); err != nil || record.Link.Kind != conversation.AgentConversationCanonical {
		t.Fatalf("record = %+v, err=%v", record, err)
	}
}

func TestAgentChildHandlersUse404ForForeignChildrenAnd409ForWrongState(t *testing.T) {
	t.Parallel()
	for name, handler := range map[string]http.Handler{
		"detail":        controlapi.AgentDetailHandler(&fakeAgentDetailRepository{err: ledger.ErrNotFound}),
		"conversations": controlapi.AgentConversationsHandler(&fakeAgentDetailRepository{err: ledger.ErrNotFound}),
	} {
		t.Run(name, func(t *testing.T) {
			verifier, token := serviceAuthorizationFixture(t, time.Now().UTC(), "", "owner.agent_child.read")
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.SetPathValue("agent_id", "agent:foreign")
			request.Header.Set(controlapi.ServiceAssertionHeader, token)
			recorder := httptest.NewRecorder()
			controlapi.RequireServiceAssertion(verifier, "owner.agent_child.read", handler).ServeHTTP(recorder, request)
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", recorder.Code)
			}
		})
	}

	verifier, token := serviceAuthorizationFixture(t, time.Now().UTC(), "", "owner.agents.read")
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.SetPathValue("agent_id", "agent:archived")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(verifier, "owner.agents.read", controlapi.AgentDetailHandler(&fakeAgentDetailRepository{err: ledger.ErrStateConflict})).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("wrong-state status = %d, want 409", recorder.Code)
	}
}

type fakeAgentDetailRepository struct {
	agent         ledger.AgentRecord
	conversations []ledger.AgentConversationRecord
	err           error
	accountID     string
	agentID       string
}

func (repository *fakeAgentDetailRepository) GetAgent(_ context.Context, accountID, agentID string) (ledger.AgentRecord, error) {
	repository.accountID, repository.agentID = accountID, agentID
	return repository.agent, repository.err
}

func (repository *fakeAgentDetailRepository) ListAgentConversations(_ context.Context, accountID, agentID string) ([]ledger.AgentConversationRecord, error) {
	repository.accountID, repository.agentID = accountID, agentID
	if repository.err != nil {
		return nil, repository.err
	}
	return append([]ledger.AgentConversationRecord{}, repository.conversations...), nil
}
