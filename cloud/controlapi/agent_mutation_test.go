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

func TestAgentMutationHandlerAppendsPresentationWithoutExecutionIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	repository := &fakeAgentMutationRepository{record: mutationAgentRecord(now.Add(-time.Hour))}
	body := `{"action":"profile","idempotency_key":"profile:rename:1","expected_profile_revision_id":"profile:researcher:1","profile":{"name":"Research Lead","title":"Primary-source research","avatar_url":"","hidden":false,"pinned":true,"sort_order":2}}`
	verifier, token := serviceAuthorizationFixture(t, now, body, "owner.agents.mutate")
	request := httptest.NewRequest(http.MethodPatch, "/api/v2/agents/agent:researcher", strings.NewReader(body))
	request.SetPathValue("agent_id", "agent:researcher")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()

	controlapi.RequireServiceAssertion(
		verifier, "owner.agents.mutate", controlapi.AgentMutationHandler(repository, func() time.Time { return now }),
	).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", recorder.Code, recorder.Body.String())
	}
	command := repository.profile
	if command.AccountID != agentMutationAccountID || command.AgentID != "agent:researcher" ||
		command.ExpectedProfileRevisionID != "profile:researcher:1" || command.Revision.Revision != 2 ||
		command.Revision.Name != "Research Lead" || command.Revision.ID == "" ||
		command.AcceptedBy != "human:"+agentMutationAccountID || !command.Revision.CreatedAt.Equal(now) {
		t.Fatalf("profile command = %+v", command)
	}
}

func TestAgentMutationHandlerAcceptsBehaviorByCopyingExactCurrentBinding(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	current := mutationAgentRecord(now.Add(-time.Hour))
	repository := &fakeAgentMutationRepository{record: current}
	body := `{"action":"behavior","idempotency_key":"behavior:instructions:1","expected_behavior_revision_id":"behavior:researcher:1","expected_binding_revision_id":"binding:researcher:1","behavior":{"role":"Researcher","standing_instructions":"Cite primary sources.","enabled_skills":["web"],"enabled_tools":["browser"],"prompt_material":"Be concise."}}`
	verifier, token := serviceAuthorizationFixture(t, now, body, "owner.agents.mutate")
	request := httptest.NewRequest(http.MethodPatch, "/api/v2/agents/agent:researcher", strings.NewReader(body))
	request.SetPathValue("agent_id", "agent:researcher")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()

	controlapi.RequireServiceAssertion(
		verifier, "owner.agents.mutate", controlapi.AgentMutationHandler(repository, func() time.Time { return now }),
	).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", recorder.Code, recorder.Body.String())
	}
	command := repository.behavior
	if command.Behavior.Revision != 2 || command.Binding.Revision != 2 || command.Binding.ID == current.Binding.ID ||
		command.Binding.BehaviorRevisionID != command.Behavior.ID || command.Binding.SupersedesRevisionID != current.Binding.ID ||
		command.Binding.ExecutionSourceID != current.Binding.ExecutionSourceID || command.Binding.SourceAgentID != current.Binding.SourceAgentID ||
		command.Binding.Provider != current.Binding.Provider || command.Binding.RequestedModel != current.Binding.RequestedModel ||
		command.Binding.ComputerID != current.Binding.ComputerID || command.Participant.SeatID != command.Binding.SeatID ||
		!command.AcceptedAt.Equal(now) {
		t.Fatalf("behavior command changed or lost exact execution evidence: %+v", command)
	}
}

func TestAgentMutationHandlerRejectsOpenEndedExecutionFields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	tests := []string{
		`{"action":"profile","idempotency_key":"profile:1","expected_profile_revision_id":"profile:researcher:1","profile":{"name":"Researcher","title":"","avatar_url":"","hidden":false,"pinned":false,"sort_order":0},"provider":"openclaw"}`,
		`{"action":"behavior","idempotency_key":"behavior:1","expected_behavior_revision_id":"behavior:researcher:1","expected_binding_revision_id":"binding:researcher:1","behavior":{"role":"Researcher","standing_instructions":"","enabled_skills":[],"enabled_tools":[],"prompt_material":""},"machine":"studio"}`,
	}
	for _, body := range tests {
		repository := &fakeAgentMutationRepository{record: mutationAgentRecord(now.Add(-time.Hour))}
		verifier, token := serviceAuthorizationFixture(t, now, body, "owner.agents.mutate")
		request := httptest.NewRequest(http.MethodPatch, "/api/v2/agents/agent:other", strings.NewReader(body))
		request.SetPathValue("agent_id", "agent:other")
		request.Header.Set(controlapi.ServiceAssertionHeader, token)
		recorder := httptest.NewRecorder()
		controlapi.RequireServiceAssertion(
			verifier, "owner.agents.mutate", controlapi.AgentMutationHandler(repository, func() time.Time { return now }),
		).ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest || repository.profile.AgentID != "" || repository.behavior.AgentID != "" {
			t.Fatalf("body %s => %d %s; repository reached", body, recorder.Code, recorder.Body.String())
		}
	}
}

func TestAgentMutationHandlerHidesForeignAgentParent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	repository := &fakeAgentMutationRepository{record: mutationAgentRecord(now.Add(-time.Hour))}
	body := `{"action":"profile","idempotency_key":"profile:1","expected_profile_revision_id":"profile:researcher:1","profile":{"name":"Researcher","title":"","avatar_url":"","hidden":false,"pinned":false,"sort_order":0}}`
	verifier, token := serviceAuthorizationFixture(t, now, body, "owner.agents.mutate")
	request := httptest.NewRequest(http.MethodPatch, "/api/v2/agents/agent:foreign", strings.NewReader(body))
	request.SetPathValue("agent_id", "agent:foreign")
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()

	controlapi.RequireServiceAssertion(
		verifier, "owner.agents.mutate", controlapi.AgentMutationHandler(repository, func() time.Time { return now }),
	).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound || repository.profile.AgentID != "" {
		t.Fatalf("status = %d, want 404; body=%s; repository write=%+v", recorder.Code, recorder.Body.String(), repository.profile)
	}
}

const agentMutationAccountID = "4af424a4-d81a-47d5-a495-400868883b86"

type fakeAgentMutationRepository struct {
	record   ledger.AgentRecord
	profile  ledger.AppendAgentProfileCommand
	behavior ledger.AppendAgentBehaviorCommand
}

func (repository *fakeAgentMutationRepository) GetAgent(_ context.Context, accountID, agentID string) (ledger.AgentRecord, error) {
	if accountID != repository.record.Agent.AccountID || agentID != repository.record.Agent.ID {
		return ledger.AgentRecord{}, ledger.ErrNotFound
	}
	return repository.record, nil
}

func (repository *fakeAgentMutationRepository) AppendAgentProfile(_ context.Context, command ledger.AppendAgentProfileCommand) (ledger.AgentRecord, error) {
	repository.profile = command
	repository.record.Agent.CurrentProfileRevisionID = command.Revision.ID
	repository.record.Profile = command.Revision
	return repository.record, nil
}

func (repository *fakeAgentMutationRepository) AppendAgentBehavior(_ context.Context, command ledger.AppendAgentBehaviorCommand) (ledger.AgentBindingAdvanceResult, error) {
	repository.behavior = command
	return ledger.AgentBindingAdvanceResult{Agent: repository.record}, nil
}

func mutationAgentRecord(createdAt time.Time) ledger.AgentRecord {
	return ledger.AgentRecord{
		Agent: conversation.Agent{
			ID: "agent:researcher", AccountID: agentMutationAccountID, State: conversation.AgentOpen,
			CurrentProfileRevisionID: "profile:researcher:1", CurrentBehaviorRevisionID: "behavior:researcher:1",
			CurrentBindingRevisionID: "binding:researcher:1", CanonicalConversationID: "conversation:researcher:home", CreatedAt: createdAt,
		},
		Profile: conversation.AgentProfileRevision{
			ID: "profile:researcher:1", AgentID: "agent:researcher", Revision: 1, Name: "Researcher", CreatedAt: createdAt,
		},
		Behavior: conversation.AgentBehaviorRevision{
			ID: "behavior:researcher:1", AgentID: "agent:researcher", Revision: 1, Role: "Researcher",
			EnabledSkills: []string{"web"}, EnabledTools: []string{"browser"}, CreatedAt: createdAt,
		},
		Binding: conversation.AgentBindingRevision{
			ID: "binding:researcher:1", AgentID: "agent:researcher", Revision: 1,
			BehaviorRevisionID: "behavior:researcher:1", ExecutionSourceID: "source:studio", SourceAgentID: "source-agent:researcher",
			SeatID: "seat:researcher:1", FortProfile: "openclaw:researcher", Provider: "openclaw",
			RequestedModel: "gpt-5.6", ResolvedModel: "gpt-5.6", ComputerID: "studio",
			AdapterID: "adapter:openclaw", AdapterRevision: "1", SourceConfigDigest: strings.Repeat("a", 64),
			AuthorityID: "authority:chat", AuthorityRevision: "1", PolicyID: "policy:chat", PolicyRevision: "1",
			SessionBehavior: "profile_scoped", MemoryBehavior: "profile_scoped", CapabilityEvidence: []string{"text"},
			ReadinessContractID: "readiness:chat", ReadinessContractRevision: "1", ActivatedAt: createdAt,
		},
		ExecutionSource: conversation.ExecutionSource{
			ID: "source:studio", AccountID: agentMutationAccountID, Framework: "openclaw", InstanceID: "studio",
			GatewayID: "gateway:studio", DisplayName: "OpenClaw · Studio",
			ResourceSharing: conversation.ResourceSharingDisclosure{
				ProviderCredentials: conversation.ResourceMachineShared, Filesystem: conversation.ResourceMachineShared,
				BrowserSessions: conversation.ResourceMachineShared, FrameworkSessions: conversation.ResourceProfileScoped,
				SourceMemory: conversation.ResourceProfileScoped, ToolConfiguration: conversation.ResourceProfileScoped,
			},
		},
		SourceAgent: conversation.SourceAgent{
			ID: "source-agent:researcher", ExecutionSourceID: "source:studio", OpaqueSourceAgentID: "researcher", DisplayName: "Researcher",
		},
		Home: conversation.Conversation{
			ID: "conversation:researcher:home", Title: "Researcher", State: conversation.ConversationOpen,
			CreatedAt: createdAt, UpdatedAt: createdAt,
		},
		Participant: conversation.Participant{
			ID: "participant:researcher:1", ConversationID: "conversation:researcher:home", SeatID: "seat:researcher:1",
			Profile: "openclaw:researcher", Agent: "openclaw", Model: "gpt-5.6", Machine: "studio",
			DisplayName: "Researcher", Position: 0, State: conversation.ParticipantActive, CreatedAt: createdAt,
		},
		Link: conversation.AgentConversation{
			AgentID: "agent:researcher", ConversationID: "conversation:researcher:home",
			Kind: conversation.AgentConversationCanonical, CreatedAt: createdAt,
		},
	}
}
