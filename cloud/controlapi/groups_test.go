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

func TestGroupsHandlerListsStableGroupsForOnlyTheSignedAccount(t *testing.T) {
	t.Parallel()
	provider := &fakeGroupLister{records: []ledger.GroupRecord{{
		Group:        conversation.GroupConversation{ID: "group:launch", AccountID: "4af424a4-d81a-47d5-a495-400868883b86"},
		Conversation: conversation.Conversation{ID: "conversation:launch", Title: "Product launch"},
	}}}
	verifier, token := serviceAuthorizationFixture(t, time.Now().UTC(), "", "owner.groups.list")
	handler := controlapi.RequireServiceAssertion(verifier, "owner.groups.list", controlapi.GroupsHandler(provider))
	request := httptest.NewRequest(http.MethodGet, "/api/v2/groups?state=open", nil)
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	request.Header.Set("X-Fort-Account-ID", "forged")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if provider.accountID != "4af424a4-d81a-47d5-a495-400868883b86" || provider.state != conversation.ConversationOpen {
		t.Fatalf("repository scope = %q/%q", provider.accountID, provider.state)
	}
	var records []ledger.GroupRecord
	if err := json.Unmarshal(recorder.Body.Bytes(), &records); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(records) != 1 || records[0].Group.ID != "group:launch" {
		t.Fatalf("records = %+v", records)
	}
}

func TestGroupsHandlerReturnsAllocatedEmptyListAndRejectsInvalidInputs(t *testing.T) {
	t.Parallel()
	provider := &fakeGroupLister{}
	verifier, token := serviceAuthorizationFixture(t, time.Now().UTC(), "", "owner.groups.list")
	handler := controlapi.RequireServiceAssertion(verifier, "owner.groups.list", controlapi.GroupsHandler(provider))
	request := httptest.NewRequest(http.MethodGet, "/api/v2/groups", nil)
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != "[]" {
		t.Fatalf("empty response = %d %q, want 200 []", recorder.Code, recorder.Body.String())
	}

	verifier, token = serviceAuthorizationFixture(t, time.Now().UTC(), "", "owner.groups.list")
	invalid := httptest.NewRequest(http.MethodGet, "/api/v2/groups?state=working", nil)
	invalid.Header.Set(controlapi.ServiceAssertionHeader, token)
	invalidRecorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(verifier, "owner.groups.list", controlapi.GroupsHandler(provider)).ServeHTTP(invalidRecorder, invalid)
	if invalidRecorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid state status = %d, want 400", invalidRecorder.Code)
	}
}

type fakeGroupLister struct {
	records   []ledger.GroupRecord
	accountID string
	state     conversation.ConversationState
}

func (provider *fakeGroupLister) ListGroups(_ context.Context, accountID string, state conversation.ConversationState) ([]ledger.GroupRecord, error) {
	provider.accountID = accountID
	provider.state = state
	return append([]ledger.GroupRecord{}, provider.records...), nil
}
