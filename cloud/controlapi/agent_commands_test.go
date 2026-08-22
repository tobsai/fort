package controlapi_test

import (
	"context"
	"encoding/base64"
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

func TestAgentCreateHandlerResolvesOpaqueEligibleOptionServerSide(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	repository := &fakeAgentLifecycleRepository{}
	resolver := &fakeAgentOptionResolver{option: eligibleAgentOption(now.Add(-time.Minute))}
	body := `{"idempotency_key":"agent:create:researcher","option_id":"eligible:hermes-mini:researcher","profile":{"name":"Researcher","title":"Primary-source research","avatar_url":"","hidden":false,"pinned":true,"sort_order":1},"behavior":{"role":"Researcher","standing_instructions":"Cite primary sources.","enabled_skills":["research"],"enabled_tools":["web"],"prompt_material":"Be concise."}}`
	verifier, token := serviceAuthorizationFixture(t, now, body, "owner.agents.create")
	request := httptest.NewRequest(http.MethodPost, "/api/v2/agents", strings.NewReader(body))
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()

	controlapi.RequireServiceAssertion(
		verifier, "owner.agents.create", controlapi.AgentCreateHandler(repository, resolver, func() time.Time { return now }),
	).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", recorder.Code, recorder.Body.String())
	}
	command := repository.created
	if resolver.accountID != agentMutationAccountID || resolver.optionID != "eligible:hermes-mini:researcher" ||
		command.Agent.AccountID != agentMutationAccountID || command.Agent.ID == "" || command.Agent.State != conversation.AgentOpen ||
		command.Profile.Name != "Researcher" || command.Behavior.Role != "Researcher" ||
		command.Binding.Provider != "hermes" || command.Binding.ComputerID != "computer:mini" ||
		command.Binding.ExecutionSourceID != resolver.option.ExecutionSource.ID || command.Binding.SourceAgentID != resolver.option.SourceAgent.ID ||
		command.Binding.AuthorityID != "authority:chat" || command.Home.ID != command.Agent.CanonicalConversationID ||
		command.Participant.SeatID != command.Binding.SeatID || command.Link.Kind != conversation.AgentConversationCanonical ||
		!command.Agent.CreatedAt.Equal(now) || !command.Binding.ActivatedAt.Equal(now) {
		t.Fatalf("server-resolved create command = %+v", command)
	}
}

func TestAgentCreateHandlerRejectsRawExecutionComponentsAndNoEligibleOptions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	for _, suffix := range []string{`,"provider":"openclaw"}`, `,"machine":"studio"}`, `,"authority_id":"authority:any"}`} {
		body := `{"idempotency_key":"agent:create:researcher","option_id":"eligible:one","profile":{"name":"Researcher","title":"","avatar_url":"","hidden":false,"pinned":false,"sort_order":0},"behavior":{"role":"Researcher","standing_instructions":"","enabled_skills":[],"enabled_tools":[],"prompt_material":""}` + suffix
		repository := &fakeAgentLifecycleRepository{}
		resolver := &fakeAgentOptionResolver{option: eligibleAgentOption(now)}
		request := httptest.NewRequest(http.MethodPost, "/api/v2/agents", strings.NewReader(body))
		recorder := httptest.NewRecorder()
		serveSignedAgentCommand(t, now, body, "owner.agents.create",
			controlapi.AgentCreateHandler(repository, resolver, func() time.Time { return now }), recorder, request)
		if recorder.Code != http.StatusBadRequest || repository.created.Agent.ID != "" || resolver.optionID != "" {
			t.Fatalf("raw execution body accepted: status=%d body=%s command=%+v", recorder.Code, recorder.Body.String(), repository.created)
		}
	}

	body := `{"idempotency_key":"agent:create:researcher","option_id":"eligible:none","profile":{"name":"Researcher","title":"","avatar_url":"","hidden":false,"pinned":false,"sort_order":0},"behavior":{"role":"Researcher","standing_instructions":"","enabled_skills":[],"enabled_tools":[],"prompt_material":""}}`
	request := httptest.NewRequest(http.MethodPost, "/api/v2/agents", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	serveSignedAgentCommand(t, now, body, "owner.agents.create",
		controlapi.AgentCreateHandler(&fakeAgentLifecycleRepository{}, controlapi.NoEligibleAgentOptions(), func() time.Time { return now }), recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("no eligible options status = %d, want 409; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAgentRebindHandlerRequiresSignedPreviewThenReResolvesOptionOnAccept(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	repository := &fakeAgentLifecycleRepository{record: mutationAgentRecord(now.Add(-time.Hour))}
	resolver := &fakeAgentOptionResolver{option: eligibleAgentOption(now.Add(-time.Minute))}
	tokens, err := controlapi.NewHMACRebindAcceptanceTokens([]byte("0123456789abcdef0123456789abcdef"), 2*time.Minute)
	if err != nil {
		t.Fatalf("new token signer: %v", err)
	}
	handler := controlapi.AgentRebindHandler(repository, resolver, tokens, func() time.Time { return now })

	previewBody := `{"action":"preview","option_id":"eligible:hermes-mini:researcher","expected_binding_revision_id":"binding:researcher:1"}`
	previewRequest := httptest.NewRequest(http.MethodPost, "/api/v2/agents/agent:researcher/rebind", strings.NewReader(previewBody))
	previewRequest.SetPathValue("agent_id", "agent:researcher")
	previewRecorder := httptest.NewRecorder()
	serveSignedAgentCommand(t, now, previewBody, "owner.agents.rebind", handler, previewRecorder, previewRequest)
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("preview status = %d; body=%s", previewRecorder.Code, previewRecorder.Body.String())
	}
	var preview struct {
		Preview         ledger.AgentRebindPreview `json:"preview"`
		AcceptanceToken string                    `json:"acceptance_token"`
		ExpiresAt       time.Time                 `json:"expires_at"`
	}
	if err := json.Unmarshal(previewRecorder.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if preview.AcceptanceToken == "" || preview.Preview.CurrentBinding.ID != "binding:researcher:1" ||
		preview.Preview.ProposedBinding.Provider != "hermes" || preview.Preview.ProposedBinding.ComputerID != "computer:mini" ||
		len(preview.Preview.NonTransferableResources) != 3 || !preview.ExpiresAt.Equal(now.Add(2*time.Minute)) {
		t.Fatalf("preview omitted bound old/new evidence: %+v", preview)
	}
	if strings.Join(preview.Preview.ProposedBinding.CapabilityEvidence, ",") != "text,tools" ||
		strings.Join(preview.Preview.ReadinessEvidence, ",") != "ready:hermes-mini,ready:source-agent" ||
		strings.Join(preview.Preview.AuthorityEvidence, ",") != "authority:revalidated,authority:source" ||
		preview.Preview.NonTransferableResources[0] != ledger.RebindResourceFiles {
		t.Fatalf("preview option evidence was not canonicalized: %+v", preview.Preview)
	}
	expiredBody, err := json.Marshal(map[string]string{
		"action": "accept", "idempotency_key": "agent:rebind:expired", "acceptance_token": preview.AcceptanceToken,
	})
	if err != nil {
		t.Fatalf("encode expired accept: %v", err)
	}
	expiredAt := now.Add(2 * time.Minute)
	expiredRequest := httptest.NewRequest(http.MethodPost, "/api/v2/agents/agent:researcher/rebind", strings.NewReader(string(expiredBody)))
	expiredRequest.SetPathValue("agent_id", "agent:researcher")
	expiredRecorder := httptest.NewRecorder()
	serveSignedAgentCommand(t, expiredAt, string(expiredBody), "owner.agents.rebind",
		controlapi.AgentRebindHandler(repository, resolver, tokens, func() time.Time { return expiredAt }), expiredRecorder, expiredRequest)
	if expiredRecorder.Code != http.StatusBadRequest || repository.accepted.Preview.AgentID != "" {
		t.Fatalf("expired token status/write = %d/%+v", expiredRecorder.Code, repository.accepted)
	}

	acceptBody, err := json.Marshal(map[string]string{
		"action": "accept", "idempotency_key": "agent:rebind:researcher:2", "acceptance_token": preview.AcceptanceToken,
	})
	if err != nil {
		t.Fatalf("encode accept: %v", err)
	}
	acceptRequest := httptest.NewRequest(http.MethodPost, "/api/v2/agents/agent:researcher/rebind", strings.NewReader(string(acceptBody)))
	acceptRequest.SetPathValue("agent_id", "agent:researcher")
	acceptRecorder := httptest.NewRecorder()
	serveSignedAgentCommand(t, now, string(acceptBody), "owner.agents.rebind", handler, acceptRecorder, acceptRequest)
	if acceptRecorder.Code != http.StatusAccepted {
		t.Fatalf("accept status = %d; body=%s", acceptRecorder.Code, acceptRecorder.Body.String())
	}
	if resolver.calls != 2 || repository.accepted.Preview.Digest != preview.Preview.Digest ||
		repository.accepted.Preview.ProposedBinding.Provider != "hermes" ||
		repository.accepted.AcceptedBy != "human:"+agentMutationAccountID || !repository.accepted.AcceptedAt.Equal(now) {
		t.Fatalf("accept did not re-resolve exact signed preview: resolver=%d command=%+v", resolver.calls, repository.accepted)
	}
}

func TestRebindAcceptanceTokenEnvironmentFailsClosedAndUsesDedicatedKey(t *testing.T) {
	t.Parallel()

	if _, err := controlapi.HMACRebindAcceptanceTokensFromEnvironment(func(string) string { return "" }); err == nil {
		t.Fatal("missing dedicated Rebind token key was accepted")
	}
	key := []byte("0123456789abcdef0123456789abcdef")
	values := map[string]string{
		"FORT_REBIND_ACCEPTANCE_KEY_B64URL":  base64.RawURLEncoding.EncodeToString(key),
		"FORT_REBIND_ACCEPTANCE_TTL_SECONDS": "90",
	}
	if _, err := controlapi.HMACRebindAcceptanceTokensFromEnvironment(func(name string) string { return values[name] }); err != nil {
		t.Fatalf("valid dedicated Rebind token configuration: %v", err)
	}
}

func TestAgentRebindHandlerRejectsForgedPreviewTamperedTokenAndForeignParent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	tokens, err := controlapi.NewHMACRebindAcceptanceTokens([]byte("0123456789abcdef0123456789abcdef"), time.Minute)
	if err != nil {
		t.Fatalf("new token signer: %v", err)
	}
	tests := []struct {
		name, agentID, body string
		want                int
	}{
		{"client preview", "agent:researcher", `{"action":"accept","idempotency_key":"rebind:1","acceptance_token":"opaque","preview":{"proposed_binding":{"provider":"openclaw"}}}`, http.StatusBadRequest},
		{"tampered token", "agent:researcher", `{"action":"accept","idempotency_key":"rebind:1","acceptance_token":"eyJmb3JnZWQiOnRydWV9.invalid"}`, http.StatusBadRequest},
		{"foreign parent", "agent:foreign", `{"action":"preview","option_id":"eligible:hermes-mini:researcher","expected_binding_revision_id":"binding:researcher:1"}`, http.StatusNotFound},
	}
	for _, test := range tests {
		repository := &fakeAgentLifecycleRepository{record: mutationAgentRecord(now.Add(-time.Hour))}
		resolver := &fakeAgentOptionResolver{option: eligibleAgentOption(now)}
		request := httptest.NewRequest(http.MethodPost, "/api/v2/agents/"+test.agentID+"/rebind", strings.NewReader(test.body))
		request.SetPathValue("agent_id", test.agentID)
		recorder := httptest.NewRecorder()
		serveSignedAgentCommand(t, now, test.body, "owner.agents.rebind",
			controlapi.AgentRebindHandler(repository, resolver, tokens, func() time.Time { return now }), recorder, request)
		if recorder.Code != test.want || repository.accepted.Preview.AgentID != "" {
			t.Fatalf("%s status/write = %d/%+v; body=%s", test.name, recorder.Code, repository.accepted, recorder.Body.String())
		}
	}
}

type fakeAgentLifecycleRepository struct {
	record   ledger.AgentRecord
	created  ledger.CreateAgentCommand
	preview  ledger.PreviewAgentRebindCommand
	accepted ledger.AcceptAgentRebindCommand
}

func (repository *fakeAgentLifecycleRepository) CreateAgent(_ context.Context, command ledger.CreateAgentCommand) (ledger.AgentRecord, error) {
	repository.created = command
	return ledger.AgentRecord{Agent: command.Agent, Profile: command.Profile, Behavior: command.Behavior, Binding: command.Binding,
		ExecutionSource: command.ExecutionSource, SourceAgent: command.SourceAgent, Home: command.Home, Participant: command.Participant, Link: command.Link}, nil
}

func (repository *fakeAgentLifecycleRepository) GetAgent(_ context.Context, accountID, agentID string) (ledger.AgentRecord, error) {
	if repository.record.Agent.ID == "" || repository.record.Agent.AccountID != accountID || repository.record.Agent.ID != agentID {
		return ledger.AgentRecord{}, ledger.ErrNotFound
	}
	return repository.record, nil
}

func (repository *fakeAgentLifecycleRepository) PreviewAgentRebind(_ context.Context, command ledger.PreviewAgentRebindCommand) (ledger.AgentRebindPreview, error) {
	repository.preview = command
	preview := ledger.AgentRebindPreview{
		AccountID: command.AccountID, AgentID: command.AgentID, CurrentBinding: repository.record.Binding,
		CurrentExecutionSource: repository.record.ExecutionSource, CurrentSourceAgent: repository.record.SourceAgent,
		ProposedBinding: command.Binding, ProposedExecutionSource: command.ExecutionSource, ProposedSourceAgent: command.SourceAgent,
		Participant: command.Participant, NonTransferableResources: command.NonTransferableResources,
		ReadinessEvidence: command.ReadinessEvidence, AuthorityEvidence: command.AuthorityEvidence, GeneratedAt: command.GeneratedAt,
	}
	var err error
	preview.Digest, err = preview.CalculateDigest()
	return preview, err
}

func (repository *fakeAgentLifecycleRepository) AcceptAgentRebind(_ context.Context, command ledger.AcceptAgentRebindCommand) (ledger.AgentBindingAdvanceResult, error) {
	repository.accepted = command
	repository.record.Binding = command.Preview.ProposedBinding
	repository.record.Agent.CurrentBindingRevisionID = command.Preview.ProposedBinding.ID
	return ledger.AgentBindingAdvanceResult{Agent: repository.record}, nil
}

type fakeAgentOptionResolver struct {
	option              controlapi.EligibleAgentOption
	accountID, optionID string
	calls               int
}

func (resolver *fakeAgentOptionResolver) ResolveEligibleAgentOption(_ context.Context, accountID, optionID string) (controlapi.EligibleAgentOption, error) {
	resolver.calls++
	resolver.accountID, resolver.optionID = accountID, optionID
	return resolver.option, nil
}

func eligibleAgentOption(observedAt time.Time) controlapi.EligibleAgentOption {
	return controlapi.EligibleAgentOption{
		ID: "eligible:hermes-mini:researcher",
		ExecutionSource: conversation.ExecutionSource{
			ID: "source:mini", AccountID: agentMutationAccountID, Framework: "hermes", InstanceID: "instance:mini",
			GatewayID: "gateway:mini", DisplayName: "Hermes · Mini", LastSeenAt: observedAt,
			ResourceSharing: conversation.ResourceSharingDisclosure{
				ProviderCredentials: conversation.ResourceMachineShared, Filesystem: conversation.ResourceMachineShared,
				BrowserSessions: conversation.ResourceMachineShared, FrameworkSessions: conversation.ResourceProfileScoped,
				SourceMemory: conversation.ResourceProfileScoped, ToolConfiguration: conversation.ResourceProfileScoped,
			},
		},
		SourceAgent: conversation.SourceAgent{
			ID: "source-agent:mini:researcher", ExecutionSourceID: "source:mini", OpaqueSourceAgentID: "researcher",
			DisplayName: "Researcher · Mini", LastSeenAt: observedAt,
		},
		Binding: conversation.AgentBindingRevision{
			ExecutionSourceID: "source:mini", SourceAgentID: "source-agent:mini:researcher",
			FortProfile: "hermes:researcher", Provider: "hermes", RequestedModel: "hermes-main", ResolvedModel: "hermes-main",
			ComputerID: "computer:mini", AdapterID: "model.chat.hermes", AdapterRevision: "adapter:2",
			SourceConfigDigest: strings.Repeat("b", 64), AuthorityID: "authority:chat", AuthorityRevision: "authority:2",
			PolicyID: "policy:chat", PolicyRevision: "policy:2", SessionBehavior: "profile_scoped", MemoryBehavior: "profile_scoped",
			CapabilityEvidence: []string{"tools", "text"}, ReadinessContractID: "readiness:chat", ReadinessContractRevision: "readiness:2",
		},
		NonTransferableResources: []ledger.RebindResource{
			ledger.RebindResourceSourceMemory, ledger.RebindResourceSessions, ledger.RebindResourceFiles,
		},
		ReadinessEvidence: []string{"ready:source-agent", "ready:hermes-mini"},
		AuthorityEvidence: []string{"authority:source", "authority:revalidated"},
	}
}

func serveSignedAgentCommand(
	t *testing.T,
	now time.Time,
	body, routeClass string,
	handler http.Handler,
	recorder *httptest.ResponseRecorder,
	request *http.Request,
) {
	t.Helper()
	verifier, token := serviceAuthorizationFixture(t, now, body, routeClass)
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	controlapi.RequireServiceAssertion(verifier, routeClass, handler).ServeHTTP(recorder, request)
}
