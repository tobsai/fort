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

func TestAgentConversationCreateHandlerAcceptsOnlyTitleAndAllocatesControlIdentity(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	body := `{"idempotency_key":"conversation:new:one","title":"Market map"}`
	repository := &fakeAgentConversationLifecycleRepository{}
	verifier, token := serviceAuthorizationFixture(t, now, body, "owner.agent_conversations.create")
	request := httptest.NewRequest(http.MethodPost, "/api/v2/agents/agent:researcher/conversations", strings.NewReader(body))
	request.SetPathValue("agent_id", "agent:researcher")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(
		verifier,
		"owner.agent_conversations.create",
		controlapi.AgentConversationCreateHandler(repository, func() time.Time { return now }),
	).ServeHTTP(recorder, request)

	command := repository.create
	if recorder.Code != http.StatusCreated || command.IdempotencyKey != "conversation:new:one" ||
		command.AccountID != testOwnerAccountID || command.AgentID != "agent:researcher" ||
		command.Conversation.ID == "" || command.Conversation.Title != "Market map" ||
		command.Conversation.State != conversation.ConversationOpen || command.Conversation.CreatedAt != now ||
		command.Conversation.UpdatedAt != now || command.Conversation.ProjectID != "" ||
		command.Link.AgentID != command.AgentID || command.Link.ConversationID != command.Conversation.ID ||
		command.Link.Kind != conversation.AgentConversationSecondary || command.Link.CreatedAt != now ||
		command.CreatedBy != "human:"+testOwnerAccountID {
		t.Fatalf("status/command = %d/%+v", recorder.Code, command)
	}

	second := &fakeAgentConversationLifecycleRepository{}
	verifier, token = serviceAuthorizationFixture(t, now.Add(time.Hour), body, "owner.agent_conversations.create")
	request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	request.SetPathValue("agent_id", "agent:researcher")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder = httptest.NewRecorder()
	controlapi.RequireServiceAssertion(verifier, "owner.agent_conversations.create", controlapi.AgentConversationCreateHandler(second, func() time.Time {
		return now.Add(time.Hour)
	})).ServeHTTP(recorder, request)
	if second.create.Conversation.ID != command.Conversation.ID {
		t.Fatalf("same command allocated different IDs: %q != %q", second.create.Conversation.ID, command.Conversation.ID)
	}
}

func TestAgentConversationCreateHandlerRejectsExecutionIdentityAndMalformedTitles(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	for _, body := range []string{
		`{"idempotency_key":"conversation:new:one","title":"Market map","provider":"openclaw"}`,
		`{"idempotency_key":"conversation:new:one","title":" Market map "}`,
		`{"idempotency_key":"conversation:new:one","title":""}`,
	} {
		repository := &fakeAgentConversationLifecycleRepository{}
		verifier, token := serviceAuthorizationFixture(t, now, body, "owner.agent_conversations.create")
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		request.SetPathValue("agent_id", "agent:researcher")
		request.Header.Set(controlapi.ServiceAssertionHeader, token)
		recorder := httptest.NewRecorder()
		controlapi.RequireServiceAssertion(verifier, "owner.agent_conversations.create", controlapi.AgentConversationCreateHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest || repository.create.AgentID != "" {
			t.Fatalf("body %s status/command = %d/%+v", body, recorder.Code, repository.create)
		}
	}
}

func TestAgentConversationMutationHandlerMapsClosedActionsToLifecycleCommands(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		body  string
		check func(*testing.T, *fakeAgentConversationLifecycleRepository)
	}{
		{"rename", `{"idempotency_key":"mutate:rename","action":"rename","expected_title":"Market map","title":"Market landscape"}`, func(t *testing.T, repository *fakeAgentConversationLifecycleRepository) {
			if repository.rename.ExpectedTitle != "Market map" || repository.rename.Title != "Market landscape" {
				t.Fatalf("rename = %+v", repository.rename)
			}
		}},
		{"pin", `{"idempotency_key":"mutate:pin","action":"pin"}`, func(t *testing.T, repository *fakeAgentConversationLifecycleRepository) {
			if repository.pin.ExpectedPinned || !repository.pin.Pinned {
				t.Fatalf("pin = %+v", repository.pin)
			}
		}},
		{"unpin", `{"idempotency_key":"mutate:unpin","action":"unpin"}`, func(t *testing.T, repository *fakeAgentConversationLifecycleRepository) {
			if !repository.pin.ExpectedPinned || repository.pin.Pinned {
				t.Fatalf("unpin = %+v", repository.pin)
			}
		}},
		{"archive", `{"idempotency_key":"mutate:archive","action":"archive"}`, func(t *testing.T, repository *fakeAgentConversationLifecycleRepository) {
			if repository.state.ExpectedState != conversation.ConversationOpen || repository.state.State != conversation.ConversationArchived {
				t.Fatalf("archive = %+v", repository.state)
			}
		}},
		{"reopen", `{"idempotency_key":"mutate:reopen","action":"reopen"}`, func(t *testing.T, repository *fakeAgentConversationLifecycleRepository) {
			if repository.state.ExpectedState != conversation.ConversationArchived || repository.state.State != conversation.ConversationOpen {
				t.Fatalf("reopen = %+v", repository.state)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakeAgentConversationLifecycleRepository{}
			verifier, token := serviceAuthorizationFixture(t, now, test.body, "owner.agent_conversations.mutate")
			request := httptest.NewRequest(http.MethodPatch, "/api/v2/agents/agent:researcher/conversations/conversation:market", strings.NewReader(test.body))
			request.SetPathValue("agent_id", "agent:researcher")
			request.SetPathValue("conversation_id", "conversation:market")
			request.Header.Set(controlapi.ServiceAssertionHeader, token)
			recorder := httptest.NewRecorder()
			controlapi.RequireServiceAssertion(verifier, "owner.agent_conversations.mutate", controlapi.AgentConversationMutationHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status/body = %d/%s", recorder.Code, recorder.Body.String())
			}
			test.check(t, repository)
			for _, common := range []struct{ account, agent, child, actor string }{
				{repository.accountID(), repository.agentID(), repository.conversationID(), repository.changedBy()},
			} {
				if common.account != testOwnerAccountID || common.agent != "agent:researcher" || common.child != "conversation:market" || common.actor != "human:"+testOwnerAccountID {
					t.Fatalf("parent scope/actor = %+v", common)
				}
			}
		})
	}
}

func TestAgentConversationMutationHandlerRejectsOpenEndedFieldsAndMapsRepositoryConflicts(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	invalid := `{"idempotency_key":"mutate:one","action":"pin","machine_id":"worker:other"}`
	repository := &fakeAgentConversationLifecycleRepository{}
	verifier, token := serviceAuthorizationFixture(t, now, invalid, "owner.agent_conversations.mutate")
	request := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(invalid))
	request.SetPathValue("agent_id", "agent:researcher")
	request.SetPathValue("conversation_id", "conversation:market")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(verifier, "owner.agent_conversations.mutate", controlapi.AgentConversationMutationHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || repository.pin.AgentID != "" {
		t.Fatalf("invalid status/command = %d/%+v", recorder.Code, repository.pin)
	}

	for name, repositoryErr := range map[string]error{"foreign": ledger.ErrNotFound, "home": ledger.ErrStateConflict, "stale": ledger.ErrRevisionConflict} {
		t.Run(name, func(t *testing.T) {
			body := `{"idempotency_key":"mutate:pin","action":"pin"}`
			repository := &fakeAgentConversationLifecycleRepository{err: repositoryErr}
			verifier, token := serviceAuthorizationFixture(t, now, body, "owner.agent_conversations.mutate")
			request := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(body))
			request.SetPathValue("agent_id", "agent:researcher")
			request.SetPathValue("conversation_id", "conversation:market")
			request.Header.Set(controlapi.ServiceAssertionHeader, token)
			recorder := httptest.NewRecorder()
			controlapi.RequireServiceAssertion(verifier, "owner.agent_conversations.mutate", controlapi.AgentConversationMutationHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request)
			want := http.StatusConflict
			if repositoryErr == ledger.ErrNotFound {
				want = http.StatusNotFound
			}
			if recorder.Code != want {
				t.Fatalf("status = %d, want %d", recorder.Code, want)
			}
		})
	}
}

type fakeAgentConversationLifecycleRepository struct {
	create ledger.CreateSecondaryConversationCommand
	rename ledger.RenameAgentConversationCommand
	pin    ledger.SetAgentConversationPinCommand
	state  ledger.SetAgentConversationStateCommand
	err    error
}

func (repository *fakeAgentConversationLifecycleRepository) CreateSecondaryConversation(_ context.Context, command ledger.CreateSecondaryConversationCommand) (ledger.AgentConversationRecord, error) {
	repository.create = command
	return lifecycleRecord(command.AgentID, command.Conversation.ID, command.Conversation.Title), repository.err
}

func (repository *fakeAgentConversationLifecycleRepository) RenameAgentConversation(_ context.Context, command ledger.RenameAgentConversationCommand) (ledger.AgentConversationRecord, error) {
	repository.rename = command
	return lifecycleRecord(command.AgentID, command.ConversationID, command.Title), repository.err
}

func (repository *fakeAgentConversationLifecycleRepository) SetAgentConversationPin(_ context.Context, command ledger.SetAgentConversationPinCommand) (ledger.AgentConversationRecord, error) {
	repository.pin = command
	record := lifecycleRecord(command.AgentID, command.ConversationID, "Market map")
	record.Pinned = command.Pinned
	return record, repository.err
}

func (repository *fakeAgentConversationLifecycleRepository) SetAgentConversationState(_ context.Context, command ledger.SetAgentConversationStateCommand) (ledger.AgentConversationRecord, error) {
	repository.state = command
	record := lifecycleRecord(command.AgentID, command.ConversationID, "Market map")
	record.Conversation.State = command.State
	return record, repository.err
}

func (repository *fakeAgentConversationLifecycleRepository) accountID() string {
	if repository.rename.AccountID != "" {
		return repository.rename.AccountID
	}
	if repository.pin.AccountID != "" {
		return repository.pin.AccountID
	}
	return repository.state.AccountID
}
func (repository *fakeAgentConversationLifecycleRepository) agentID() string {
	if repository.rename.AgentID != "" {
		return repository.rename.AgentID
	}
	if repository.pin.AgentID != "" {
		return repository.pin.AgentID
	}
	return repository.state.AgentID
}
func (repository *fakeAgentConversationLifecycleRepository) conversationID() string {
	if repository.rename.ConversationID != "" {
		return repository.rename.ConversationID
	}
	if repository.pin.ConversationID != "" {
		return repository.pin.ConversationID
	}
	return repository.state.ConversationID
}
func (repository *fakeAgentConversationLifecycleRepository) changedBy() string {
	if repository.rename.ChangedBy != "" {
		return repository.rename.ChangedBy
	}
	if repository.pin.ChangedBy != "" {
		return repository.pin.ChangedBy
	}
	return repository.state.ChangedBy
}

func lifecycleRecord(agentID, conversationID, title string) ledger.AgentConversationRecord {
	return ledger.AgentConversationRecord{
		Conversation: conversation.Conversation{ID: conversationID, Title: title, State: conversation.ConversationOpen},
		Link:         conversation.AgentConversation{AgentID: agentID, ConversationID: conversationID, Kind: conversation.AgentConversationSecondary},
	}
}
