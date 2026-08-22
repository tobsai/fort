package controlapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestGroupMutationHandlerMapsClosedRenameArchiveAndReopenActions(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	tests := []struct {
		name, body string
		check      func(*testing.T, *fakeGroupLifecycleRepository)
	}{
		{"rename", `{"idempotency_key":"group:rename:one","action":"rename","expected_title":"Launch","title":"Launch council"}`, func(t *testing.T, repository *fakeGroupLifecycleRepository) {
			if repository.rename.ExpectedTitle != "Launch" || repository.rename.Title != "Launch council" {
				t.Fatalf("rename = %+v", repository.rename)
			}
		}},
		{"archive", `{"idempotency_key":"group:archive:one","action":"archive"}`, func(t *testing.T, repository *fakeGroupLifecycleRepository) {
			if repository.state.ExpectedState != conversation.ConversationOpen || repository.state.State != conversation.ConversationArchived {
				t.Fatalf("archive = %+v", repository.state)
			}
		}},
		{"reopen", `{"idempotency_key":"group:reopen:one","action":"reopen"}`, func(t *testing.T, repository *fakeGroupLifecycleRepository) {
			if repository.state.ExpectedState != conversation.ConversationArchived || repository.state.State != conversation.ConversationOpen {
				t.Fatalf("reopen = %+v", repository.state)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newFakeGroupLifecycleRepository(now)
			verifier, token := serviceAuthorizationFixture(t, now, test.body, "owner.groups.mutate")
			request := httptest.NewRequest(http.MethodPatch, "/api/v2/groups/group:launch", strings.NewReader(test.body))
			request.SetPathValue("group_id", "group:launch")
			request.Header.Set(controlapi.ServiceAssertionHeader, token)
			recorder := httptest.NewRecorder()
			controlapi.RequireServiceAssertion(verifier, "owner.groups.mutate", controlapi.GroupMutationHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status/body = %d/%s", recorder.Code, recorder.Body.String())
			}
			test.check(t, repository)
			if repository.accountID() != testOwnerAccountID || repository.groupID() != "group:launch" ||
				repository.changedBy() != "human:"+testOwnerAccountID || repository.changedAt() != now {
				t.Fatalf("scope/audit = account %q Group %q actor %q time %v", repository.accountID(),
					repository.groupID(), repository.changedBy(), repository.changedAt())
			}
		})
	}
}

func TestGroupMembersHandlerResolvesExactCurrentPinsAndServerIdentity(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	body := `{"idempotency_key":"group:members:two","expected_membership_revision_id":"membership:launch:1","agent_ids":["agent:builder","agent:reviewer"]}`
	repository := newFakeGroupLifecycleRepository(now)
	verifier, token := serviceAuthorizationFixture(t, now, body, "owner.group_members.replace")
	request := httptest.NewRequest(http.MethodPost, "/api/v2/groups/group:launch/members", strings.NewReader(body))
	request.SetPathValue("group_id", "group:launch")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(verifier, "owner.group_members.replace", controlapi.GroupMembersHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request)

	command := repository.replace
	if recorder.Code != http.StatusOK || command.AccountID != testOwnerAccountID || command.GroupID != "group:launch" ||
		command.ExpectedMembershipRevisionID != "membership:launch:1" || command.Membership.ID == "" ||
		command.Membership.GroupID != command.GroupID || command.Membership.Revision != 2 || command.Membership.CreatedAt != now ||
		command.ChangedBy != "human:"+testOwnerAccountID || command.ChangedAt != now || len(command.Membership.Members) != 2 ||
		len(command.MemberBindings) != 2 {
		t.Fatalf("status/command = %d/%+v", recorder.Code, command)
	}
	if command.Membership.Members[0] != (conversation.GroupMember{AgentID: "agent:builder", Position: 0}) ||
		command.MemberBindings[0].ParticipantID != "participant:builder" ||
		command.MemberBindings[0].BehaviorRevisionID != "behavior:agent:builder" ||
		command.MemberBindings[0].BindingRevisionID != "binding:agent:builder" {
		t.Fatalf("retained member evidence = %+v / %+v", command.Membership.Members[0], command.MemberBindings[0])
	}
	if command.Membership.Members[1] != (conversation.GroupMember{AgentID: "agent:reviewer", Position: 1}) ||
		command.MemberBindings[1].ParticipantID == "" || command.MemberBindings[1].BehaviorRevisionID != "behavior:agent:reviewer" ||
		command.MemberBindings[1].BindingRevisionID != "binding:agent:reviewer" {
		t.Fatalf("new member evidence = %+v / %+v", command.Membership.Members[1], command.MemberBindings[1])
	}
}

func TestGroupLifecycleHandlersRejectOpenEndedExecutionFieldsAndMapConflicts(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	invalid := `{"idempotency_key":"group:members:two","expected_membership_revision_id":"membership:launch:1","agent_ids":["agent:builder","agent:reviewer"],"model":"forged"}`
	repository := newFakeGroupLifecycleRepository(now)
	verifier, token := serviceAuthorizationFixture(t, now, invalid, "owner.group_members.replace")
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(invalid))
	request.SetPathValue("group_id", "group:launch")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	controlapi.RequireServiceAssertion(verifier, "owner.group_members.replace", controlapi.GroupMembersHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || repository.replace.GroupID != "" {
		t.Fatalf("invalid status/command = %d/%+v", recorder.Code, repository.replace)
	}

	for name, repositoryErr := range map[string]error{
		"foreign": ledger.ErrNotFound, "active": ledger.ErrStateConflict, "stale": ledger.ErrRevisionConflict,
	} {
		t.Run(name, func(t *testing.T) {
			body := `{"idempotency_key":"group:archive:one","action":"archive"}`
			repository := newFakeGroupLifecycleRepository(now)
			repository.err = repositoryErr
			verifier, token := serviceAuthorizationFixture(t, now, body, "owner.groups.mutate")
			request := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(body))
			request.SetPathValue("group_id", "group:launch")
			request.Header.Set(controlapi.ServiceAssertionHeader, token)
			recorder := httptest.NewRecorder()
			controlapi.RequireServiceAssertion(verifier, "owner.groups.mutate", controlapi.GroupMutationHandler(repository, func() time.Time { return now })).ServeHTTP(recorder, request)
			want := http.StatusConflict
			if errors.Is(repositoryErr, ledger.ErrNotFound) {
				want = http.StatusNotFound
			}
			if recorder.Code != want {
				t.Fatalf("status/body = %d/%s, want %d", recorder.Code, recorder.Body.String(), want)
			}
		})
	}
}

type fakeGroupLifecycleRepository struct {
	group   ledger.GroupRecord
	agents  map[string]ledger.AgentRecord
	rename  ledger.RenameGroupCommand
	state   ledger.SetGroupStateCommand
	replace ledger.ReplaceGroupMembersCommand
	err     error
}

func newFakeGroupLifecycleRepository(now time.Time) *fakeGroupLifecycleRepository {
	repository := &fakeGroupLifecycleRepository{group: fakeGroupRecord(now), agents: make(map[string]ledger.AgentRecord)}
	for _, id := range []string{"agent:research", "agent:builder", "agent:reviewer"} {
		repository.agents[id] = ledger.AgentRecord{
			Agent: conversation.Agent{ID: id, AccountID: testOwnerAccountID, State: conversation.AgentOpen,
				CurrentBehaviorRevisionID: "behavior:" + id, CurrentBindingRevisionID: "binding:" + id},
			Behavior: conversation.AgentBehaviorRevision{ID: "behavior:" + id, AgentID: id},
			Binding:  conversation.AgentBindingRevision{ID: "binding:" + id, AgentID: id, BehaviorRevisionID: "behavior:" + id},
		}
	}
	return repository
}

func (repository *fakeGroupLifecycleRepository) GetGroup(_ context.Context, accountID, groupID string) (ledger.GroupRecord, error) {
	if repository.err != nil {
		return ledger.GroupRecord{}, repository.err
	}
	if accountID != testOwnerAccountID || groupID != repository.group.Group.ID {
		return ledger.GroupRecord{}, ledger.ErrNotFound
	}
	return repository.group, nil
}

func (repository *fakeGroupLifecycleRepository) GetAgent(_ context.Context, accountID, agentID string) (ledger.AgentRecord, error) {
	if repository.err != nil {
		return ledger.AgentRecord{}, repository.err
	}
	record, ok := repository.agents[agentID]
	if accountID != testOwnerAccountID || !ok {
		return ledger.AgentRecord{}, ledger.ErrNotFound
	}
	return record, nil
}

func (repository *fakeGroupLifecycleRepository) RenameGroup(_ context.Context, command ledger.RenameGroupCommand) (ledger.GroupRecord, error) {
	repository.rename = command
	record := repository.group
	record.Conversation.Title = command.Title
	return record, repository.err
}

func (repository *fakeGroupLifecycleRepository) SetGroupState(_ context.Context, command ledger.SetGroupStateCommand) (ledger.GroupRecord, error) {
	repository.state = command
	record := repository.group
	record.Group.State, record.Conversation.State = command.State, command.State
	return record, repository.err
}

func (repository *fakeGroupLifecycleRepository) ReplaceGroupMembers(_ context.Context, command ledger.ReplaceGroupMembersCommand) (ledger.GroupRecord, error) {
	repository.replace = command
	record := repository.group
	record.Group.CurrentMembershipRevisionID = command.Membership.ID
	record.Membership, record.MemberBindings = command.Membership, command.MemberBindings
	return record, repository.err
}

func (repository *fakeGroupLifecycleRepository) accountID() string {
	if repository.rename.AccountID != "" {
		return repository.rename.AccountID
	}
	return repository.state.AccountID
}

func (repository *fakeGroupLifecycleRepository) groupID() string {
	if repository.rename.GroupID != "" {
		return repository.rename.GroupID
	}
	return repository.state.GroupID
}

func (repository *fakeGroupLifecycleRepository) changedBy() string {
	if repository.rename.ChangedBy != "" {
		return repository.rename.ChangedBy
	}
	return repository.state.ChangedBy
}

func (repository *fakeGroupLifecycleRepository) changedAt() time.Time {
	if !repository.rename.ChangedAt.IsZero() {
		return repository.rename.ChangedAt
	}
	return repository.state.ChangedAt
}
