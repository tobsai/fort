package controlapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestHandoffCreateHandlerDerivesOwnerEvidenceAndRejectsExecutionIdentity(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	deadline := now.Add(20 * time.Minute)
	body := `{"idempotency_key":"handoff:create:one","source_conversation_id":"conversation:launch","source_message_id":"41","recipient_agent_id":"agent:builder","context_message_ids":["40","41"],"requested_result":"Verify the release evidence.","reply_to_message_id":"40","hard_deadline":"` + deadline.Format(time.RFC3339Nano) + `"}`
	repository := &fakeHumanHandoffRepository{}
	verifier, token := serviceAuthorizationFixture(t, now, body, "owner.handoffs.create")
	request := httptest.NewRequest(http.MethodPost, "/api/v2/handoffs", strings.NewReader(body))
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(
		verifier, "owner.handoffs.create", controlapi.HandoffCreateHandler(repository, func() time.Time { return now }),
	).ServeHTTP(recorder, request)

	command := repository.create
	if recorder.Code != http.StatusAccepted || command.AccountID != testOwnerAccountID ||
		command.IdempotencyKey != "handoff:create:one" || command.SourceConversationID != "conversation:launch" ||
		command.SourceMessageID != "41" || command.RecipientAgentID != "agent:builder" ||
		command.RequestedResult != "Verify the release evidence." || command.ReplyToMessageID != "40" ||
		!command.HardDeadline.Equal(deadline) || command.CreatedByID != "human:"+testOwnerAccountID ||
		!command.CreatedAt.Equal(now) || command.HandoffID == "" || command.TargetID == "" ||
		command.RootDelegationGrantID == "" || len(command.ContextMessageIDs) != 2 {
		t.Fatalf("status/create = %d/%+v", recorder.Code, command)
	}
	if command.HandoffID == command.TargetID || command.HandoffID == command.RootDelegationGrantID ||
		command.TargetID == command.RootDelegationGrantID {
		t.Fatalf("server-owned ids are not distinct: %+v", command)
	}

	for _, forbidden := range []string{"provider", "model", "machine_id", "binding_revision_id", "participant_id", "handoff_id", "created_at"} {
		forged := strings.TrimSuffix(body, "}") + `,"` + forbidden + `":"forged"}`
		verifier, token = serviceAuthorizationFixture(t, now, forged, "owner.handoffs.create")
		request = httptest.NewRequest(http.MethodPost, "/api/v2/handoffs", strings.NewReader(forged))
		request.Header.Set(controlapi.ServiceAssertionHeader, token)
		recorder = httptest.NewRecorder()
		controlapi.RequireServiceAssertion(
			verifier, "owner.handoffs.create", controlapi.HandoffCreateHandler(repository, func() time.Time { return now }),
		).ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("client field %q status = %d, want 400", forbidden, recorder.Code)
		}
	}
}

func TestHandoffListDetailAndCancelHandlersPreserveAccountScope(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	repository := &fakeHumanHandoffRepository{}

	verifier, token := serviceAuthorizationFixture(t, now, "", "owner.handoffs.list")
	request := httptest.NewRequest(http.MethodGet, "/api/v2/handoffs", nil)
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(verifier, "owner.handoffs.list", controlapi.HandoffsHandler(repository)).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "[]\n" || repository.accountID != testOwnerAccountID {
		t.Fatalf("list response/scope = %d %q / %q", recorder.Code, recorder.Body.String(), repository.accountID)
	}

	repository.record = ledger.HandoffRecord{Handoff: conversation.Handoff{ID: "handoff:one", AccountID: testOwnerAccountID}}
	verifier, token = serviceAuthorizationFixture(t, now, "", "owner.handoffs.read")
	request = httptest.NewRequest(http.MethodGet, "/api/v2/handoffs/handoff:one", nil)
	request.SetPathValue("handoff_id", "handoff:one")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder = httptest.NewRecorder()
	controlapi.RequireServiceAssertion(verifier, "owner.handoffs.read", controlapi.HandoffDetailHandler(repository)).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"id":"handoff:one"`) ||
		repository.accountID != testOwnerAccountID || repository.handoffID != "handoff:one" {
		t.Fatalf("detail response/scope = %d %q / %q %q", recorder.Code, recorder.Body.String(), repository.accountID, repository.handoffID)
	}

	cancelBody := `{"idempotency_key":"handoff:cancel:one"}`
	verifier, token = serviceAuthorizationFixture(t, now, cancelBody, "owner.handoffs.cancel")
	request = httptest.NewRequest(http.MethodPost, "/api/v2/handoffs/handoff:one/cancel", strings.NewReader(cancelBody))
	request.SetPathValue("handoff_id", "handoff:one")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder = httptest.NewRecorder()
	controlapi.RequireServiceAssertion(
		verifier, "owner.handoffs.cancel", controlapi.HandoffCancelHandler(repository, func() time.Time { return now }),
	).ServeHTTP(recorder, request)
	command := repository.cancel
	if recorder.Code != http.StatusAccepted || command.AccountID != testOwnerAccountID || command.HandoffID != "handoff:one" ||
		command.IdempotencyKey != "handoff:cancel:one" || command.CanceledBy != "human:"+testOwnerAccountID || !command.CanceledAt.Equal(now) {
		t.Fatalf("cancel response/command = %d %q / %+v", recorder.Code, recorder.Body.String(), command)
	}

	forged := `{"idempotency_key":"handoff:cancel:two","target_id":"target:replacement"}`
	verifier, token = serviceAuthorizationFixture(t, now, forged, "owner.handoffs.cancel")
	request = httptest.NewRequest(http.MethodPost, "/api/v2/handoffs/handoff:one/cancel", strings.NewReader(forged))
	request.SetPathValue("handoff_id", "handoff:one")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder = httptest.NewRecorder()
	controlapi.RequireServiceAssertion(
		verifier, "owner.handoffs.cancel", controlapi.HandoffCancelHandler(repository, func() time.Time { return now }),
	).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("replacement target status = %d, want 400", recorder.Code)
	}
}

type fakeHumanHandoffRepository struct {
	create               ledger.CreateHumanHandoffCommand
	cancel               ledger.CancelHandoffCommand
	record               ledger.HandoffRecord
	accountID, handoffID string
}

func (repository *fakeHumanHandoffRepository) CreateHumanHandoff(_ context.Context, command ledger.CreateHumanHandoffCommand) (ledger.HandoffRecord, error) {
	repository.create = command
	return ledger.HandoffRecord{Handoff: conversation.Handoff{ID: command.HandoffID, AccountID: command.AccountID}}, nil
}

func (repository *fakeHumanHandoffRepository) ListHandoffs(_ context.Context, accountID string) ([]ledger.HandoffRecord, error) {
	repository.accountID = accountID
	return nil, nil
}

func (repository *fakeHumanHandoffRepository) GetHandoff(_ context.Context, accountID, handoffID string) (ledger.HandoffRecord, error) {
	repository.accountID, repository.handoffID = accountID, handoffID
	return repository.record, nil
}

func (repository *fakeHumanHandoffRepository) CancelHandoff(_ context.Context, command ledger.CancelHandoffCommand) (ledger.HandoffRecord, error) {
	repository.cancel = command
	return ledger.HandoffRecord{Handoff: conversation.Handoff{ID: command.HandoffID, AccountID: command.AccountID}}, nil
}
