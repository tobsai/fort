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
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestAgentsHandlerListsStableAgentsForOnlyTheSignedAccount(t *testing.T) {
	t.Parallel()

	provider := &fakeAgentRepositoryProvider{records: []ledger.AgentRecord{{
		Agent:   conversation.Agent{ID: "agent:researcher", AccountID: "4af424a4-d81a-47d5-a495-400868883b86"},
		Profile: conversation.AgentProfileRevision{Name: "Researcher"},
	}}}
	body := ""
	now := time.Now().UTC()
	verifier, token := serviceAuthorizationFixture(t, now, body, "owner.agents.list")
	handler := controlapi.RequireServiceAssertion(verifier, "owner.agents.list", controlapi.AgentsHandler(provider))
	request := httptest.NewRequest(http.MethodGet, "/api/v2/agents?state=open", nil)
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	request.Header.Set("X-Fort-Account-ID", "forged")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if provider.accountID != "4af424a4-d81a-47d5-a495-400868883b86" || provider.state != conversation.AgentOpen {
		t.Fatalf("repository scope = %q/%q", provider.accountID, provider.state)
	}
	var records []ledger.AgentRecord
	if err := json.Unmarshal(recorder.Body.Bytes(), &records); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(records) != 1 || records[0].Agent.ID != "agent:researcher" {
		t.Fatalf("records = %+v", records)
	}
}

func TestAgentsHandlerReturnsAllocatedEmptyListAndRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	provider := &fakeAgentRepositoryProvider{}
	now := time.Now().UTC()
	verifier, token := serviceAuthorizationFixture(t, now, "", "owner.agents.list")
	handler := controlapi.RequireServiceAssertion(verifier, "owner.agents.list", controlapi.AgentsHandler(provider))

	request := httptest.NewRequest(http.MethodGet, "/api/v2/agents", nil)
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != "[]" {
		t.Fatalf("empty list response = %d %q, want 200 []", recorder.Code, recorder.Body.String())
	}

	verifier, token = serviceAuthorizationFixture(t, now, "", "owner.agents.list")
	invalid := httptest.NewRequest(http.MethodGet, "/api/v2/agents?state=working", nil)
	invalid.Header.Set(controlapi.ServiceAssertionHeader, token)
	invalidRecorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(verifier, "owner.agents.list", controlapi.AgentsHandler(provider)).ServeHTTP(invalidRecorder, invalid)
	if invalidRecorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid state status = %d, want 400", invalidRecorder.Code)
	}
}

type fakeAgentRepositoryProvider struct {
	records   []ledger.AgentRecord
	accountID string
	state     conversation.AgentState
}

func (provider *fakeAgentRepositoryProvider) ListAgents(_ context.Context, accountID string, state conversation.AgentState) ([]ledger.AgentRecord, error) {
	provider.accountID = accountID
	provider.state = state
	return append([]ledger.AgentRecord{}, provider.records...), nil
}
