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

func TestGroupCreateHandlerResolvesCurrentAgentEvidenceServerSide(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	body := `{"idempotency_key":"group:create:one","title":"Launch review","agent_ids":["agent:research","agent:builder"]}`
	repository := newFakeGroupCommandRepository()
	verifier, token := serviceAuthorizationFixture(t, now, body, "owner.groups.create")
	request := httptest.NewRequest(http.MethodPost, "/api/v2/groups", strings.NewReader(body))
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(
		verifier, "owner.groups.create", controlapi.GroupCreateHandler(repository, func() time.Time { return now }),
	).ServeHTTP(recorder, request)

	command := repository.create
	if recorder.Code != http.StatusCreated || command.Group.AccountID != testOwnerAccountID ||
		command.Conversation.Title != "Launch review" || command.Group.ID == "" || command.Conversation.ID == "" ||
		command.Membership.ID == "" || command.Group.CurrentMembershipRevisionID != command.Membership.ID ||
		len(command.Membership.Members) != 2 || len(command.MemberBindings) != 2 {
		t.Fatalf("status/create = %d/%+v", recorder.Code, command)
	}
	for index, agentID := range []string{"agent:research", "agent:builder"} {
		binding := command.MemberBindings[index]
		if command.Membership.Members[index] != (conversation.GroupMember{AgentID: agentID, Position: index}) ||
			binding.AgentID != agentID || binding.BehaviorRevisionID != "behavior:"+agentID ||
			binding.BindingRevisionID != "binding:"+agentID || binding.ParticipantID == "" {
			t.Fatalf("member %d = %+v / %+v", index, command.Membership.Members[index], binding)
		}
	}

	unknown := `{"idempotency_key":"group:create:two","title":"Bad","agent_ids":["agent:research","agent:unknown"],"provider":"forged"}`
	verifier, token = serviceAuthorizationFixture(t, now, unknown, "owner.groups.create")
	request = httptest.NewRequest(http.MethodPost, "/api/v2/groups", strings.NewReader(unknown))
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder = httptest.NewRecorder()
	controlapi.RequireServiceAssertion(verifier, "owner.groups.create", controlapi.GroupCreateHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("client-selected execution identity status = %d, want 400", recorder.Code)
	}
}

func TestGroupDetailAndTurnHandlersPreserveFrozenMembershipAndOneWave(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	repository := newFakeGroupCommandRepository()
	repository.group = fakeGroupRecord(now)
	// Group membership is stable-Agent identity only. A later accepted Rebind
	// must be selected by a future turn without rewriting the membership.
	rebound := repository.agents["agent:research"]
	rebound.Agent.CurrentBehaviorRevisionID = "behavior:agent:research:2"
	rebound.Agent.CurrentBindingRevisionID = "binding:agent:research:2"
	rebound.Behavior.ID = rebound.Agent.CurrentBehaviorRevisionID
	rebound.Binding.ID = rebound.Agent.CurrentBindingRevisionID
	rebound.Binding.BehaviorRevisionID = rebound.Behavior.ID
	repository.agents["agent:research"] = rebound
	repository.turns = []ledger.GroupTurnRecord{}
	repository.messages = []ledger.AgentConversationMessage{{
		ID: 73, ConversationID: repository.group.Group.ConversationID,
		TurnID: "group-turn:one", TargetID: "target:research", AuthorKind: conversation.AuthorAssistant,
		AuthorID: "agent:research", AuthorAgentID: "agent:research", Body: "The attributed result.", CreatedAt: now,
	}}

	verifier, token := serviceAuthorizationFixture(t, now, "", "owner.groups.read")
	request := httptest.NewRequest(http.MethodGet, "/api/v2/groups/group:launch", nil)
	request.SetPathValue("group_id", "group:launch")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(verifier, "owner.groups.read", controlapi.GroupDetailHandler(repository)).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"turns":[]`) ||
		!strings.Contains(recorder.Body.String(), `"messages":[{"id":73`) ||
		!strings.Contains(recorder.Body.String(), `"author_agent_id":"agent:research"`) ||
		repository.accountID != testOwnerAccountID || repository.groupID != "group:launch" {
		t.Fatalf("detail response/scope = %d %q / %q %q", recorder.Code, recorder.Body.String(), repository.accountID, repository.groupID)
	}

	deadline := now.Add(10 * time.Minute)
	body := `{"idempotency_key":"group:send:one","client_turn_id":"client:one","text":"Compare evidence.","selection":"everyone","recipient_agent_ids":["agent:research","agent:builder"],"concurrency_policy":"concurrent","hard_deadline":"` + deadline.Format(time.RFC3339Nano) + `"}`
	verifier, token = serviceAuthorizationFixture(t, now, body, "owner.group_turns.send")
	request = httptest.NewRequest(http.MethodPost, "/api/v2/groups/group:launch/turns", strings.NewReader(body))
	request.SetPathValue("group_id", "group:launch")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder = httptest.NewRecorder()
	controlapi.RequireServiceAssertion(
		verifier, "owner.group_turns.send", controlapi.GroupTurnsHandler(repository, func() time.Time { return now }),
	).ServeHTTP(recorder, request)

	command := repository.send
	if recorder.Code != http.StatusAccepted || command.AccountID != testOwnerAccountID || command.HumanID != "human:"+testOwnerAccountID ||
		command.Envelope.GroupID != repository.group.Group.ID || command.Envelope.MembershipRevisionID != repository.group.Membership.ID ||
		command.Envelope.MaxAgentMessages != conversation.MaxGroupAgentMessages ||
		command.Envelope.MaxHandoffDepth != conversation.MaxGroupHandoffDepth ||
		command.Envelope.Deadline != deadline || len(command.Envelope.Recipients) != 2 || len(command.TargetIDs) != 2 {
		t.Fatalf("status/send = %d/%+v", recorder.Code, command)
	}
	if got := command.Envelope.Recipients[0]; got.AgentID != "agent:research" ||
		got.BehaviorRevisionID != "behavior:agent:research:2" || got.BindingRevisionID != "binding:agent:research:2" ||
		got.ParticipantID == repository.group.MemberBindings[0].ParticipantID || command.TargetIDs[0] == "" {
		t.Fatalf("rebound recipient/target = %+v/%q", got, command.TargetIDs[0])
	}
	if got := command.Envelope.Recipients[1]; got.AgentID != "agent:builder" ||
		got.BehaviorRevisionID != "behavior:agent:builder" || got.BindingRevisionID != "binding:agent:builder" ||
		command.TargetIDs[1] == "" {
		t.Fatalf("unchanged recipient/target = %+v/%q", got, command.TargetIDs[1])
	}
}

func TestGroupCommandHandlersFailClosedWhenRepositoryIsUnavailable(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	tests := []struct {
		name, routeClass, body string
		handler                http.Handler
		pathValue              bool
	}{
		{
			name: "create", routeClass: "owner.groups.create",
			body:    `{"idempotency_key":"group:create:one","title":"Launch","agent_ids":["agent:research","agent:builder"]}`,
			handler: controlapi.GroupCreateHandler(nil, func() time.Time { return now }),
		},
		{
			name: "turn", routeClass: "owner.group_turns.send",
			body:      `{"idempotency_key":"group:send:one","client_turn_id":"client:one","text":"Compare.","selection":"everyone","recipient_agent_ids":["agent:research","agent:builder"],"concurrency_policy":"concurrent","hard_deadline":"` + now.Add(10*time.Minute).Format(time.RFC3339Nano) + `"}`,
			handler:   controlapi.GroupTurnsHandler(nil, func() time.Time { return now }),
			pathValue: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifier, token := serviceAuthorizationFixture(t, now, test.body, test.routeClass)
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))
			if test.pathValue {
				request.SetPathValue("group_id", "group:launch")
			}
			request.Header.Set(controlapi.ServiceAssertionHeader, token)
			recorder := httptest.NewRecorder()
			controlapi.RequireServiceAssertion(verifier, test.routeClass, test.handler).ServeHTTP(recorder, request)
			if recorder.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503; body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

type fakeGroupCommandRepository struct {
	agents             map[string]ledger.AgentRecord
	group              ledger.GroupRecord
	turns              []ledger.GroupTurnRecord
	messages           []ledger.AgentConversationMessage
	create             ledger.CreateGroupCommand
	send               ledger.SendGroupTurnCommand
	accountID, groupID string
}

func newFakeGroupCommandRepository() *fakeGroupCommandRepository {
	agents := make(map[string]ledger.AgentRecord)
	for _, id := range []string{"agent:research", "agent:builder"} {
		agents[id] = ledger.AgentRecord{
			Agent: conversation.Agent{ID: id, AccountID: testOwnerAccountID, State: conversation.AgentOpen,
				CurrentBehaviorRevisionID: "behavior:" + id, CurrentBindingRevisionID: "binding:" + id},
			Behavior: conversation.AgentBehaviorRevision{ID: "behavior:" + id, AgentID: id},
			Binding:  conversation.AgentBindingRevision{ID: "binding:" + id, AgentID: id, BehaviorRevisionID: "behavior:" + id},
		}
	}
	return &fakeGroupCommandRepository{agents: agents}
}

func fakeGroupRecord(now time.Time) ledger.GroupRecord {
	members := []conversation.GroupMember{{AgentID: "agent:research", Position: 0}, {AgentID: "agent:builder", Position: 1}}
	bindings := []conversation.GroupRecipient{
		{AgentID: "agent:research", BehaviorRevisionID: "behavior:agent:research", BindingRevisionID: "binding:agent:research", ParticipantID: "participant:research"},
		{AgentID: "agent:builder", BehaviorRevisionID: "behavior:agent:builder", BindingRevisionID: "binding:agent:builder", ParticipantID: "participant:builder"},
	}
	return ledger.GroupRecord{
		Group: conversation.GroupConversation{ID: "group:launch", AccountID: testOwnerAccountID,
			ConversationID: "conversation:launch", State: conversation.ConversationOpen,
			CurrentMembershipRevisionID: "membership:launch:1", CreatedAt: now},
		Conversation:   conversation.Conversation{ID: "conversation:launch", Title: "Launch", State: conversation.ConversationOpen, CreatedAt: now, UpdatedAt: now},
		Membership:     conversation.GroupMembershipRevision{ID: "membership:launch:1", GroupID: "group:launch", Revision: 1, Members: members, CreatedAt: now},
		MemberBindings: bindings,
	}
}

func (repository *fakeGroupCommandRepository) GetAgent(_ context.Context, accountID, agentID string) (ledger.AgentRecord, error) {
	repository.accountID = accountID
	return repository.agents[agentID], nil
}
func (repository *fakeGroupCommandRepository) CreateGroup(_ context.Context, command ledger.CreateGroupCommand) (ledger.GroupRecord, error) {
	repository.create = command
	return ledger.GroupRecord{Group: command.Group, Conversation: command.Conversation, Membership: command.Membership, MemberBindings: command.MemberBindings}, nil
}
func (repository *fakeGroupCommandRepository) GetGroup(_ context.Context, accountID, groupID string) (ledger.GroupRecord, error) {
	repository.accountID, repository.groupID = accountID, groupID
	return repository.group, nil
}
func (repository *fakeGroupCommandRepository) ListGroupTurns(_ context.Context, accountID, groupID string) ([]ledger.GroupTurnRecord, error) {
	repository.accountID, repository.groupID = accountID, groupID
	return append([]ledger.GroupTurnRecord{}, repository.turns...), nil
}
func (repository *fakeGroupCommandRepository) ListGroupMessages(_ context.Context, accountID, groupID string) ([]ledger.AgentConversationMessage, error) {
	repository.accountID, repository.groupID = accountID, groupID
	return append([]ledger.AgentConversationMessage{}, repository.messages...), nil
}
func (repository *fakeGroupCommandRepository) SendGroupTurn(_ context.Context, command ledger.SendGroupTurnCommand) (ledger.GroupTurnRecord, error) {
	repository.send = command
	return ledger.GroupTurnRecord{Envelope: command.Envelope}, nil
}
