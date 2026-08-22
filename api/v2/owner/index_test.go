package handler

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/cloud/securebody"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestOwnerEndpointDispatchesOnlyExactRewrittenResources(t *testing.T) {
	t.Parallel()

	key := []byte("0123456789abcdef0123456789abcdef")
	values := map[string]string{
		"DATABASE_URL":                     "postgresql://runtime.test/fort?sslmode=require",
		"FORT_CONTROL_ASSERTION_KEYS_JSON": `{"service-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
		"FORT_BODY_ACTIVE_KID":             "body-2026-08",
		"FORT_BODY_KEYS_JSON":              `{"body-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
		"FORT_AUTHORITY_MODE":              controlapi.CloudWriteAuthorityMode,
		"FORT_AUTHORITY_EPOCH":             "9",
	}
	store := &fakeOwnerStore{claimed: make(map[string]struct{})}
	opens := 0
	handler := newOwnerEndpoint(func(key string) string { return values[key] }, func(context.Context, string, securebody.KeyRing) (ownerControlStore, error) {
		opens++
		return store, nil
	})

	tests := []struct {
		name, target, routeClass, want string
	}{
		{"agent", "/api/v2/owner?resource=agent&agent_id=agent%3Aresearcher", "owner.agents.read", `"id":"agent:researcher"`},
		{"conversations", "/api/v2/owner?resource=agent_conversations&agent_id=agent%3Aresearcher", "owner.agent_conversations.list", `"conversation_id":"conversation:home"`},
		{"canonical", "/api/v2/owner?resource=agent_canonical_conversation&agent_id=agent%3Aresearcher", "owner.agent_conversations.canonical", `"kind":"canonical"`},
		{"groups", "/api/v2/owner?resource=groups&state=open", "owner.groups.list", `"id":"group:launch"`},
		{"group detail", "/api/v2/owner?resource=group_detail&group_id=group%3Alaunch", "owner.groups.read", `"turns":[]`},
		{"routines", "/api/v2/owner?resource=routines&agent_id=agent%3Aresearcher", "owner.routines.list", `"id":"routine:daily"`},
		{"routine runs", "/api/v2/owner?resource=routine_runs&agent_id=agent%3Aresearcher&routine_id=routine%3Adaily", "owner.routines.runs", `"id":"routine-run:daily:1"`},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			request.Header.Set(controlapi.ServiceAssertionHeader, ownerAssertion(t, key, test.routeClass, "owner-endpoint-nonce-000000000000000"+string(rune('a'+index))))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), test.want) {
				t.Fatalf("response = %d %q, want body containing %q", recorder.Code, recorder.Body.String(), test.want)
			}
		})
	}
	if opens != 1 || store.accountID != ownerAccountID || store.agentID != "agent:researcher" {
		t.Fatalf("opens/scope = %d %q/%q", opens, store.accountID, store.agentID)
	}

	for _, target := range []string{
		"/api/v2/owner",
		"/api/v2/owner?resource=unknown",
		"/api/v2/owner?resource=groups&extra=value",
		"/api/v2/owner?resource=agent&agent_id=",
	} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("invalid target %q status = %d, want 404", target, recorder.Code)
		}
	}
}

func TestOwnerEndpointRejectsWrongMethodBeforeDatabase(t *testing.T) {
	t.Parallel()
	opens := 0
	handler := newOwnerEndpoint(func(string) string { return "" }, func(context.Context, string, securebody.KeyRing) (ownerControlStore, error) {
		opens++
		return nil, nil
	})
	request := httptest.NewRequest(http.MethodPut, "/api/v2/owner?resource=groups", strings.NewReader("{}"))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed || opens != 0 {
		t.Fatalf("status/opens = %d/%d, want 405/0", recorder.Code, opens)
	}
}

func TestOwnerRouteSeparatesAgentListCreateReadMutationAndRebind(t *testing.T) {
	t.Parallel()

	list := httptest.NewRequest(http.MethodGet, "/api/v2/owner?resource=agents&state=open", nil)
	route, method, forwarded, ok := ownerRoute(list)
	if !ok || route != "agents" || method != http.MethodGet || forwarded.URL.RawQuery != "state=open" {
		t.Fatalf("GET Agents route = %q/%q/%q/%v", route, method, forwarded.URL.RawQuery, ok)
	}

	create := httptest.NewRequest(http.MethodPost, "/api/v2/owner?resource=agents", nil)
	route, method, forwarded, ok = ownerRoute(create)
	if !ok || route != "agent_create" || method != http.MethodPost || forwarded.URL.RawQuery != "" {
		t.Fatalf("POST Agents route = %q/%q/%q/%v", route, method, forwarded.URL.RawQuery, ok)
	}

	read := httptest.NewRequest(http.MethodGet, "/api/v2/owner?resource=agent&agent_id=agent%3Aresearcher", nil)
	route, method, forwarded, ok = ownerRoute(read)
	if !ok || route != "agent" || method != http.MethodGet || forwarded.PathValue("agent_id") != "agent:researcher" {
		t.Fatalf("GET Agent route = %q/%q/%v", route, method, ok)
	}

	mutate := httptest.NewRequest(http.MethodPatch, "/api/v2/owner?resource=agent&agent_id=agent%3Aresearcher", nil)
	route, method, forwarded, ok = ownerRoute(mutate)
	if !ok || route != "agent_mutation" || method != http.MethodPatch || forwarded.PathValue("agent_id") != "agent:researcher" {
		t.Fatalf("PATCH Agent route = %q/%q/%v", route, method, ok)
	}

	rebind := httptest.NewRequest(http.MethodPost, "/api/v2/owner?resource=agent_rebind&agent_id=agent%3Aresearcher", nil)
	route, method, forwarded, ok = ownerRoute(rebind)
	if !ok || route != "agent_rebind" || method != http.MethodPost || forwarded.PathValue("agent_id") != "agent:researcher" {
		t.Fatalf("POST Agent Rebind route = %q/%q/%v", route, method, ok)
	}
}

func TestOwnerRouteSeparatesGroupReadMutationAndMembershipReplacement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method, target, route, wantMethod string
	}{
		{http.MethodGet, "/api/v2/owner?resource=group_detail&group_id=group%3Alaunch", "group_detail", http.MethodGet},
		{http.MethodPatch, "/api/v2/owner?resource=group_detail&group_id=group%3Alaunch", "group_mutation", http.MethodPatch},
		{http.MethodPost, "/api/v2/owner?resource=group_members&group_id=group%3Alaunch", "group_members", http.MethodPost},
	}
	for _, test := range tests {
		request := httptest.NewRequest(test.method, test.target, nil)
		route, method, forwarded, ok := ownerRoute(request)
		if !ok || route != test.route || method != test.wantMethod || forwarded.PathValue("group_id") != "group:launch" {
			t.Fatalf("%s %s route = %q/%q/%v", test.method, test.target, route, method, ok)
		}
	}
}

func TestOwnerRouteAcceptsOnlyExactRoutineResourcesAndMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method, target, route, wantMethod, agentID, routineID string
	}{
		{http.MethodGet, "/api/v2/owner?resource=routines&agent_id=agent%3Aresearcher", "routines_list", http.MethodGet, "agent:researcher", ""},
		{http.MethodPost, "/api/v2/owner?resource=routines&agent_id=agent%3Aresearcher", "routines_create", http.MethodPost, "agent:researcher", ""},
		{http.MethodPatch, "/api/v2/owner?resource=routine_detail&agent_id=agent%3Aresearcher&routine_id=routine%3Adaily", "routine_mutation", http.MethodPatch, "agent:researcher", "routine:daily"},
		{http.MethodPost, "/api/v2/owner?resource=routine_test&agent_id=agent%3Aresearcher&routine_id=routine%3Adaily", "routine_test", http.MethodPost, "agent:researcher", "routine:daily"},
		{http.MethodGet, "/api/v2/owner?resource=routine_runs&agent_id=agent%3Aresearcher&routine_id=routine%3Adaily", "routine_runs", http.MethodGet, "agent:researcher", "routine:daily"},
	}
	for _, test := range tests {
		request := httptest.NewRequest(test.method, test.target, nil)
		route, method, forwarded, ok := ownerRoute(request)
		if !ok || route != test.route || method != test.wantMethod || forwarded.URL.RawQuery != "" ||
			forwarded.PathValue("agent_id") != test.agentID || forwarded.PathValue("routine_id") != test.routineID {
			t.Fatalf("route(%s %s) = %q/%q/%v (%q, %q), want %q/%q/true (%q, %q)",
				test.method, test.target, route, method, ok, forwardedPathValue(forwarded, "agent_id"),
				forwardedPathValue(forwarded, "routine_id"), test.route, test.wantMethod, test.agentID, test.routineID)
		}
	}

	for _, target := range []string{
		"/api/v2/owner?resource=routines&agent_id=agent%3Aresearcher&provider=openai",
		"/api/v2/owner?resource=routine_detail&agent_id=agent%3Aresearcher",
		"/api/v2/owner?resource=routine_test&agent_id=agent%3Aresearcher&routine_id=",
	} {
		if _, _, _, ok := ownerRoute(httptest.NewRequest(http.MethodGet, target, nil)); ok {
			t.Fatalf("ownerRoute(%q) accepted an inexact Routine resource", target)
		}
	}
}

func forwardedPathValue(request *http.Request, key string) string {
	if request == nil {
		return ""
	}
	return request.PathValue(key)
}

func TestOwnerEndpointDispatchesGroupCreateAndTurnCommands(t *testing.T) {
	t.Parallel()
	key := []byte("0123456789abcdef0123456789abcdef")
	values := map[string]string{
		"DATABASE_URL":                     "postgresql://runtime.test/fort?sslmode=require",
		"FORT_CONTROL_ASSERTION_KEYS_JSON": `{"service-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
		"FORT_BODY_ACTIVE_KID":             "body-2026-08",
		"FORT_BODY_KEYS_JSON":              `{"body-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
		"FORT_AUTHORITY_MODE":              controlapi.CloudWriteAuthorityMode,
		"FORT_AUTHORITY_EPOCH":             "9",
	}
	store := &fakeOwnerStore{claimed: make(map[string]struct{})}
	handler := newOwnerEndpoint(func(key string) string { return values[key] }, func(context.Context, string, securebody.KeyRing) (ownerControlStore, error) {
		return store, nil
	})

	createBody := `{"idempotency_key":"group:create:one","title":"Launch","agent_ids":["agent:researcher","agent:builder"]}`
	create := httptest.NewRequest(http.MethodPost, "/api/v2/owner?resource=groups", strings.NewReader(createBody))
	create.Header.Set(controlapi.ServiceAssertionHeader, ownerAssertionBody(t, key, "owner.groups.create", createBody, "owner-group-create-nonce-0000000001"))
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("Group create = %d %q", created.Code, created.Body.String())
	}

	deadline := time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339Nano)
	turnBody := `{"idempotency_key":"group:send:one","client_turn_id":"client:one","text":"Compare.","selection":"everyone","recipient_agent_ids":["agent:researcher","agent:builder"],"concurrency_policy":"concurrent","hard_deadline":"` + deadline + `"}`
	turn := httptest.NewRequest(http.MethodPost, "/api/v2/owner?resource=group_turns&group_id=group%3Alaunch", strings.NewReader(turnBody))
	turn.Header.Set(controlapi.ServiceAssertionHeader, ownerAssertionBody(t, key, "owner.group_turns.send", turnBody, "owner-group-turn-nonce-000000000001"))
	sent := httptest.NewRecorder()
	handler.ServeHTTP(sent, turn)
	if sent.Code != http.StatusAccepted {
		t.Fatalf("Group turn = %d %q", sent.Code, sent.Body.String())
	}
}

func TestOwnerEndpointDispatchesSecondaryConversationCreate(t *testing.T) {
	t.Parallel()
	key := []byte("0123456789abcdef0123456789abcdef")
	values := map[string]string{
		"DATABASE_URL":                     "postgresql://runtime.test/fort?sslmode=require",
		"FORT_CONTROL_ASSERTION_KEYS_JSON": `{"service-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
		"FORT_BODY_ACTIVE_KID":             "body-2026-08",
		"FORT_BODY_KEYS_JSON":              `{"body-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
		"FORT_AUTHORITY_MODE":              controlapi.CloudWriteAuthorityMode,
		"FORT_AUTHORITY_EPOCH":             "9",
	}
	store := &fakeOwnerStore{claimed: make(map[string]struct{})}
	handler := newOwnerEndpoint(func(key string) string { return values[key] }, func(context.Context, string, securebody.KeyRing) (ownerControlStore, error) {
		return store, nil
	})
	body := `{"idempotency_key":"conversation:create:one","title":"Market map"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v2/owner?resource=agent_conversations&agent_id=agent%3Aresearcher", strings.NewReader(body))
	request.Header.Set(controlapi.ServiceAssertionHeader, ownerAssertionBody(t, key, "owner.agent_conversations.create", body, "owner-conversation-create-nonce-00001"))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated || store.createdConversation.AgentID != "agent:researcher" ||
		store.createdConversation.AccountID != ownerAccountID || store.createdConversation.Conversation.Title != "Market map" {
		t.Fatalf("status/command = %d/%+v; body=%s", recorder.Code, store.createdConversation, recorder.Body.String())
	}
}

func TestOwnerEndpointDispatchesAgentCreateThroughInjectedClosedOptionInventory(t *testing.T) {
	t.Parallel()
	key := []byte("0123456789abcdef0123456789abcdef")
	values := map[string]string{
		"DATABASE_URL":                     "postgresql://runtime.test/fort?sslmode=require",
		"FORT_CONTROL_ASSERTION_KEYS_JSON": `{"service-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
		"FORT_BODY_ACTIVE_KID":             "body-2026-08",
		"FORT_BODY_KEYS_JSON":              `{"body-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
		"FORT_AUTHORITY_MODE":              controlapi.CloudWriteAuthorityMode,
		"FORT_AUTHORITY_EPOCH":             "9",
	}
	store := &fakeOwnerStore{claimed: make(map[string]struct{})}
	resolver := &fakeOwnerOptionResolver{option: ownerEligibleAgentOption()}
	handler := newOwnerEndpointWithAgentLifecycle(
		func(name string) string { return values[name] },
		func(context.Context, string, securebody.KeyRing) (ownerControlStore, error) { return store, nil },
		resolver,
		nil,
	)
	body := `{"idempotency_key":"agent:create:one","option_id":"eligible:hermes-mini","profile":{"name":"Researcher","title":"","avatar_url":"","hidden":false,"pinned":true,"sort_order":1},"behavior":{"role":"Researcher","standing_instructions":"Cite sources.","enabled_skills":[],"enabled_tools":[],"prompt_material":""}}`
	request := httptest.NewRequest(http.MethodPost, "/api/v2/owner?resource=agents", strings.NewReader(body))
	request.Header.Set(controlapi.ServiceAssertionHeader, ownerAssertionBody(t, key, "owner.agents.create", body, "owner-agent-create-nonce-000000001"))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated || store.createdAgent.Agent.AccountID != ownerAccountID ||
		store.createdAgent.Binding.Provider != "hermes" || resolver.accountID != ownerAccountID || resolver.optionID != "eligible:hermes-mini" {
		t.Fatalf("status/create/resolution = %d/%+v/%q/%q; body=%s", recorder.Code, store.createdAgent, resolver.accountID, resolver.optionID, recorder.Body.String())
	}
}

func TestOwnerEndpointDispatchesClosedAgentProfileMutation(t *testing.T) {
	t.Parallel()
	key := []byte("0123456789abcdef0123456789abcdef")
	values := map[string]string{
		"DATABASE_URL":                     "postgresql://runtime.test/fort?sslmode=require",
		"FORT_CONTROL_ASSERTION_KEYS_JSON": `{"service-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
		"FORT_BODY_ACTIVE_KID":             "body-2026-08",
		"FORT_BODY_KEYS_JSON":              `{"body-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
		"FORT_AUTHORITY_MODE":              controlapi.CloudWriteAuthorityMode,
		"FORT_AUTHORITY_EPOCH":             "9",
	}
	store := &fakeOwnerStore{claimed: make(map[string]struct{})}
	handler := newOwnerEndpoint(func(key string) string { return values[key] }, func(context.Context, string, securebody.KeyRing) (ownerControlStore, error) {
		return store, nil
	})
	body := `{"action":"profile","idempotency_key":"agent:profile:2","expected_profile_revision_id":"profile:agent:researcher","profile":{"name":"Research Lead","title":"Primary-source research","avatar_url":"","hidden":false,"pinned":true,"sort_order":1}}`
	request := httptest.NewRequest(http.MethodPatch, "/api/v2/owner?resource=agent&agent_id=agent%3Aresearcher", strings.NewReader(body))
	request.Header.Set(controlapi.ServiceAssertionHeader, ownerAssertionBody(t, key, "owner.agents.mutate", body, "owner-agent-mutation-nonce-0000001"))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted || store.mutatedProfile.AgentID != "agent:researcher" ||
		store.mutatedProfile.AccountID != ownerAccountID || store.mutatedProfile.Revision.Name != "Research Lead" {
		t.Fatalf("status/command = %d/%+v; body=%s", recorder.Code, store.mutatedProfile, recorder.Body.String())
	}
}

const ownerAccountID = "4af424a4-d81a-47d5-a495-400868883b86"

type fakeOwnerStore struct {
	claimed             map[string]struct{}
	accountID           string
	agentID             string
	mutatedProfile      ledger.AppendAgentProfileCommand
	mutatedBehavior     ledger.AppendAgentBehaviorCommand
	createdAgent        ledger.CreateAgentCommand
	previewedRebind     ledger.PreviewAgentRebindCommand
	acceptedRebind      ledger.AcceptAgentRebindCommand
	renamedGroup        ledger.RenameGroupCommand
	groupState          ledger.SetGroupStateCommand
	replacedMembers     ledger.ReplaceGroupMembersCommand
	createdConversation ledger.CreateSecondaryConversationCommand
	createdHandoff      ledger.CreateHumanHandoffCommand
	canceledHandoff     ledger.CancelHandoffCommand
}

type fakeOwnerOptionResolver struct {
	option              controlapi.EligibleAgentOption
	accountID, optionID string
}

func (resolver *fakeOwnerOptionResolver) ResolveEligibleAgentOption(_ context.Context, accountID, optionID string) (controlapi.EligibleAgentOption, error) {
	resolver.accountID, resolver.optionID = accountID, optionID
	return resolver.option, nil
}

func ownerEligibleAgentOption() controlapi.EligibleAgentOption {
	return controlapi.EligibleAgentOption{
		ID: "eligible:hermes-mini",
		ExecutionSource: conversation.ExecutionSource{
			ID: "source:mini", AccountID: ownerAccountID, Framework: "hermes", InstanceID: "instance:mini",
			GatewayID: "gateway:mini", DisplayName: "Hermes · Mini",
			ResourceSharing: conversation.ResourceSharingDisclosure{
				ProviderCredentials: conversation.ResourceMachineShared, Filesystem: conversation.ResourceMachineShared,
				BrowserSessions: conversation.ResourceMachineShared, FrameworkSessions: conversation.ResourceProfileScoped,
				SourceMemory: conversation.ResourceProfileScoped, ToolConfiguration: conversation.ResourceProfileScoped,
			},
		},
		SourceAgent: conversation.SourceAgent{
			ID: "source-agent:mini:researcher", ExecutionSourceID: "source:mini", OpaqueSourceAgentID: "researcher", DisplayName: "Researcher · Mini",
		},
		Binding: conversation.AgentBindingRevision{
			ExecutionSourceID: "source:mini", SourceAgentID: "source-agent:mini:researcher", FortProfile: "hermes:researcher",
			Provider: "hermes", RequestedModel: "hermes-main", ResolvedModel: "hermes-main", ComputerID: "computer:mini",
			AdapterID: "model.chat.hermes", AdapterRevision: "2", SourceConfigDigest: strings.Repeat("b", 64),
			AuthorityID: "authority:chat", AuthorityRevision: "2", PolicyID: "policy:chat", PolicyRevision: "2",
			SessionBehavior: "profile_scoped", MemoryBehavior: "profile_scoped", CapabilityEvidence: []string{"text"},
			ReadinessContractID: "readiness:chat", ReadinessContractRevision: "2",
		},
		NonTransferableResources: []ledger.RebindResource{}, ReadinessEvidence: []string{"ready:hermes-mini"},
		AuthorityEvidence: []string{"authority:revalidated"},
	}
}

func (store *fakeOwnerStore) ListAgents(_ context.Context, accountID string, _ conversation.AgentState) ([]ledger.AgentRecord, error) {
	store.accountID = accountID
	return []ledger.AgentRecord{}, nil
}

func (store *fakeOwnerStore) CreateAgent(_ context.Context, command ledger.CreateAgentCommand) (ledger.AgentRecord, error) {
	store.createdAgent = command
	return ledger.AgentRecord{Agent: command.Agent, Profile: command.Profile, Behavior: command.Behavior, Binding: command.Binding,
		ExecutionSource: command.ExecutionSource, SourceAgent: command.SourceAgent, Home: command.Home, Participant: command.Participant, Link: command.Link}, nil
}

func (store *fakeOwnerStore) PreviewAgentRebind(_ context.Context, command ledger.PreviewAgentRebindCommand) (ledger.AgentRebindPreview, error) {
	store.previewedRebind = command
	current, err := store.GetAgent(context.Background(), command.AccountID, command.AgentID)
	if err != nil {
		return ledger.AgentRebindPreview{}, err
	}
	preview := ledger.AgentRebindPreview{
		AccountID: command.AccountID, AgentID: command.AgentID, CurrentBinding: current.Binding,
		CurrentExecutionSource: current.ExecutionSource, CurrentSourceAgent: current.SourceAgent,
		ProposedBinding: command.Binding, ProposedExecutionSource: command.ExecutionSource, ProposedSourceAgent: command.SourceAgent,
		Participant: command.Participant, NonTransferableResources: command.NonTransferableResources,
		ReadinessEvidence: command.ReadinessEvidence, AuthorityEvidence: command.AuthorityEvidence, GeneratedAt: command.GeneratedAt,
	}
	preview.Digest, err = preview.CalculateDigest()
	return preview, err
}

func (store *fakeOwnerStore) AcceptAgentRebind(_ context.Context, command ledger.AcceptAgentRebindCommand) (ledger.AgentBindingAdvanceResult, error) {
	store.acceptedRebind = command
	return ledger.AgentBindingAdvanceResult{}, nil
}

func (store *fakeOwnerStore) CreateSecondaryConversation(_ context.Context, command ledger.CreateSecondaryConversationCommand) (ledger.AgentConversationRecord, error) {
	store.createdConversation = command
	return ledger.AgentConversationRecord{Conversation: command.Conversation, Link: command.Link}, nil
}

func (store *fakeOwnerStore) Claim(_ context.Context, accountID, keyID, nonce string, _ time.Time) (bool, error) {
	claim := accountID + ":" + keyID + ":" + nonce
	if _, found := store.claimed[claim]; found {
		return false, nil
	}
	store.claimed[claim] = struct{}{}
	return true, nil
}

func (store *fakeOwnerStore) GetAgent(_ context.Context, accountID, agentID string) (ledger.AgentRecord, error) {
	store.accountID, store.agentID = accountID, agentID
	return ledger.AgentRecord{
		Agent: conversation.Agent{ID: agentID, AccountID: accountID, State: conversation.AgentOpen,
			CurrentProfileRevisionID:  "profile:" + agentID,
			CurrentBehaviorRevisionID: "behavior:" + agentID, CurrentBindingRevisionID: "binding:" + agentID},
		Profile:  conversation.AgentProfileRevision{ID: "profile:" + agentID, AgentID: agentID, Revision: 1, Name: "Researcher", CreatedAt: time.Now().UTC()},
		Behavior: conversation.AgentBehaviorRevision{ID: "behavior:" + agentID, AgentID: agentID},
		Binding: conversation.AgentBindingRevision{ID: "binding:" + agentID, AgentID: agentID,
			BehaviorRevisionID: "behavior:" + agentID},
	}, nil
}

func (store *fakeOwnerStore) AppendAgentProfile(_ context.Context, command ledger.AppendAgentProfileCommand) (ledger.AgentRecord, error) {
	store.mutatedProfile = command
	return ledger.AgentRecord{
		Agent:   conversation.Agent{ID: command.AgentID, AccountID: command.AccountID, State: conversation.AgentOpen, CurrentProfileRevisionID: command.Revision.ID},
		Profile: command.Revision,
	}, nil
}

func (store *fakeOwnerStore) AppendAgentBehavior(_ context.Context, command ledger.AppendAgentBehaviorCommand) (ledger.AgentBindingAdvanceResult, error) {
	store.mutatedBehavior = command
	return ledger.AgentBindingAdvanceResult{Agent: ledger.AgentRecord{
		Agent: conversation.Agent{ID: command.AgentID, AccountID: command.AccountID, State: conversation.AgentOpen,
			CurrentBehaviorRevisionID: command.Behavior.ID, CurrentBindingRevisionID: command.Binding.ID},
		Behavior: command.Behavior,
		Binding:  command.Binding,
	}}, nil
}

func (store *fakeOwnerStore) ListAgentConversations(_ context.Context, accountID, agentID string) ([]ledger.AgentConversationRecord, error) {
	store.accountID, store.agentID = accountID, agentID
	return []ledger.AgentConversationRecord{{
		Conversation: conversation.Conversation{ID: "conversation:home", Title: "Home", State: conversation.ConversationOpen},
		Link:         conversation.AgentConversation{AgentID: agentID, ConversationID: "conversation:home", Kind: conversation.AgentConversationCanonical},
	}}, nil
}

func (store *fakeOwnerStore) ListGroups(_ context.Context, accountID string, _ conversation.ConversationState) ([]ledger.GroupRecord, error) {
	store.accountID = accountID
	return []ledger.GroupRecord{{Group: conversation.GroupConversation{ID: "group:launch", AccountID: accountID}}}, nil
}

func (store *fakeOwnerStore) CreateGroup(_ context.Context, command ledger.CreateGroupCommand) (ledger.GroupRecord, error) {
	return ledger.GroupRecord{Group: command.Group, Conversation: command.Conversation, Membership: command.Membership, MemberBindings: command.MemberBindings}, nil
}

func (store *fakeOwnerStore) GetGroup(_ context.Context, accountID, groupID string) (ledger.GroupRecord, error) {
	store.accountID = accountID
	return ledger.GroupRecord{
		Group: conversation.GroupConversation{ID: groupID, AccountID: accountID, ConversationID: "conversation:launch", State: conversation.ConversationOpen,
			CurrentMembershipRevisionID: "membership:launch:1", CreatedAt: time.Now().UTC()},
		Conversation: conversation.Conversation{ID: "conversation:launch", Title: "Launch", State: conversation.ConversationOpen,
			CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		Membership: conversation.GroupMembershipRevision{ID: "membership:launch:1", GroupID: groupID, Revision: 1,
			Members: []conversation.GroupMember{{AgentID: "agent:researcher", Position: 0}, {AgentID: "agent:builder", Position: 1}}, CreatedAt: time.Now().UTC()},
		MemberBindings: []conversation.GroupRecipient{
			{AgentID: "agent:researcher", BehaviorRevisionID: "behavior:agent:researcher", BindingRevisionID: "binding:agent:researcher", ParticipantID: "participant:researcher"},
			{AgentID: "agent:builder", BehaviorRevisionID: "behavior:agent:builder", BindingRevisionID: "binding:agent:builder", ParticipantID: "participant:builder"},
		},
	}, nil
}

func (store *fakeOwnerStore) SendGroupTurn(_ context.Context, command ledger.SendGroupTurnCommand) (ledger.GroupTurnRecord, error) {
	return ledger.GroupTurnRecord{Envelope: command.Envelope}, nil
}

func (store *fakeOwnerStore) RenameGroup(_ context.Context, command ledger.RenameGroupCommand) (ledger.GroupRecord, error) {
	store.renamedGroup = command
	record, err := store.GetGroup(context.Background(), command.AccountID, command.GroupID)
	record.Conversation.Title = command.Title
	return record, err
}

func (store *fakeOwnerStore) SetGroupState(_ context.Context, command ledger.SetGroupStateCommand) (ledger.GroupRecord, error) {
	store.groupState = command
	record, err := store.GetGroup(context.Background(), command.AccountID, command.GroupID)
	record.Group.State, record.Conversation.State = command.State, command.State
	return record, err
}

func (store *fakeOwnerStore) ReplaceGroupMembers(_ context.Context, command ledger.ReplaceGroupMembersCommand) (ledger.GroupRecord, error) {
	store.replacedMembers = command
	record, err := store.GetGroup(context.Background(), command.AccountID, command.GroupID)
	record.Group.CurrentMembershipRevisionID = command.Membership.ID
	record.Membership, record.MemberBindings = command.Membership, command.MemberBindings
	return record, err
}

func (store *fakeOwnerStore) ListGroupTurns(context.Context, string, string) ([]ledger.GroupTurnRecord, error) {
	return []ledger.GroupTurnRecord{}, nil
}

func (store *fakeOwnerStore) ListGroupMessages(context.Context, string, string) ([]ledger.AgentConversationMessage, error) {
	return []ledger.AgentConversationMessage{}, nil
}

func (store *fakeOwnerStore) CreateHumanHandoff(_ context.Context, command ledger.CreateHumanHandoffCommand) (ledger.HandoffRecord, error) {
	store.createdHandoff = command
	return ledger.HandoffRecord{Handoff: conversation.Handoff{ID: command.HandoffID, AccountID: command.AccountID}}, nil
}

func (store *fakeOwnerStore) ListHandoffs(_ context.Context, accountID string) ([]ledger.HandoffRecord, error) {
	store.accountID = accountID
	return nil, nil
}

func (store *fakeOwnerStore) GetHandoff(_ context.Context, accountID, handoffID string) (ledger.HandoffRecord, error) {
	store.accountID = accountID
	return ledger.HandoffRecord{Handoff: conversation.Handoff{ID: handoffID, AccountID: accountID}}, nil
}

func (store *fakeOwnerStore) CancelHandoff(_ context.Context, command ledger.CancelHandoffCommand) (ledger.HandoffRecord, error) {
	store.canceledHandoff = command
	return ledger.HandoffRecord{Handoff: conversation.Handoff{ID: command.HandoffID, AccountID: command.AccountID}}, nil
}

func (store *fakeOwnerStore) ListRoutines(_ context.Context, accountID, agentID string) ([]ledger.RoutineRecord, error) {
	store.accountID, store.agentID = accountID, agentID
	return []ledger.RoutineRecord{ownerRoutineFixture(accountID, agentID, "routine:daily")}, nil
}

func (store *fakeOwnerStore) CreateRoutine(_ context.Context, command ledger.CreateRoutineCommand) (ledger.RoutineRecord, error) {
	store.accountID, store.agentID = command.Routine.AccountID, command.Routine.AgentID
	return ledger.RoutineRecord{Routine: command.Routine, CurrentRevision: command.Revision}, nil
}

func (store *fakeOwnerStore) GetRoutine(_ context.Context, accountID, routineID string) (ledger.RoutineRecord, error) {
	return ownerRoutineFixture(accountID, "agent:researcher", routineID), nil
}

func (store *fakeOwnerStore) RevalidateRoutine(_ context.Context, command ledger.RevalidateRoutineCommand) (ledger.RoutineRecord, error) {
	record := ownerRoutineFixture(command.AccountID, "agent:researcher", command.RoutineID)
	record.Routine.CurrentRevisionID = command.Revision.ID
	record.CurrentRevision = command.Revision
	return record, nil
}

func (store *fakeOwnerStore) EnqueueRoutineOccurrence(_ context.Context, command ledger.EnqueueRoutineOccurrenceCommand) (ledger.RoutineRunRecord, error) {
	return ledger.RoutineRunRecord{
		Occurrence: ledger.RoutineOccurrence{ID: command.OccurrenceID, AccountID: command.AccountID,
			RoutineID: command.RoutineID, RoutineRevisionID: command.RoutineRevisionID, Kind: command.Kind,
			State: conversation.RoutineRunQueued, ScheduledFor: command.ScheduledFor,
			IdempotencyKey: command.IdempotencyKey, ApprovalEvidenceID: command.ApprovalEvidenceID,
			CreatedAt: command.CreatedAt, UpdatedAt: command.CreatedAt},
		Run: conversation.RoutineRun{ID: command.RunID, RoutineID: command.RoutineID,
			RoutineRevisionID: command.RoutineRevisionID, AgentID: "agent:researcher",
			BehaviorRevisionID: "behavior:agent:researcher", BindingRevisionID: "binding:agent:researcher",
			OccurrenceID: command.OccurrenceID, Kind: command.Kind, State: conversation.RoutineRunQueued,
			CreatedAt: command.CreatedAt},
		ResultConversationID: "conversation:results",
		Activities:           []ledger.RoutineRunActivity{},
	}, nil
}

func (store *fakeOwnerStore) ListRoutineRuns(_ context.Context, accountID, routineID string) ([]ledger.RoutineRunRecord, error) {
	now := time.Date(2026, 8, 21, 13, 0, 0, 0, time.UTC)
	return []ledger.RoutineRunRecord{{
		Occurrence: ledger.RoutineOccurrence{ID: "routine-occurrence:daily:1", AccountID: accountID,
			RoutineID: routineID, RoutineRevisionID: "routine-revision:" + routineID,
			Kind: conversation.RoutineRunScheduled, State: conversation.RoutineRunSucceeded,
			ScheduledFor: now, IdempotencyKey: "routine:daily@1", ApprovalEvidenceID: "approval:daily:1",
			CreatedAt: now, UpdatedAt: now},
		Run: conversation.RoutineRun{ID: "routine-run:daily:1", RoutineID: routineID,
			RoutineRevisionID: "routine-revision:" + routineID, AgentID: "agent:researcher",
			BehaviorRevisionID: "behavior:agent:researcher", BindingRevisionID: "binding:agent:researcher",
			OccurrenceID: "routine-occurrence:daily:1", Kind: conversation.RoutineRunScheduled,
			State: conversation.RoutineRunSucceeded, NormalizedResult: "Daily brief", CreatedAt: now},
		ResultConversationID: "conversation:results", Activities: []ledger.RoutineRunActivity{},
	}}, nil
}

func ownerRoutineFixture(accountID, agentID, routineID string) ledger.RoutineRecord {
	createdAt := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	revisionID := "routine-revision:" + routineID
	return ledger.RoutineRecord{
		Routine: conversation.Routine{ID: routineID, AccountID: accountID, AgentID: agentID,
			CurrentRevisionID: revisionID, State: conversation.RoutineActive, CreatedAt: createdAt},
		CurrentRevision: conversation.RoutineRevision{ID: revisionID, RoutineID: routineID, Revision: 1,
			AgentID: agentID, BehaviorRevisionID: "behavior:" + agentID, BindingRevisionID: "binding:" + agentID,
			Authority: conversation.RoutineAuthorityFortCloud, Trigger: conversation.RoutineTriggerSchedule,
			Schedule: "0 0 9 * * *", Timezone: "America/Chicago", NextOccurrence: createdAt.Add(24 * time.Hour),
			InputSource: "agent-home", FreshnessSeconds: 3600, ExpectedResult: "Daily brief",
			ResultConversationID: "conversation:results", ApprovalBoundary: "none",
			MissingInputBehavior: "needs_you", RetryPolicy: "none", CatchUpPolicy: "skip",
			LatenessPolicy: "within_90s", CreatedAt: createdAt},
	}
}

func (store *fakeOwnerStore) Close() error { return nil }

func ownerAssertion(t *testing.T, key []byte, routeClass, nonce string) string {
	return ownerAssertionBody(t, key, routeClass, "", nonce)
}

func ownerAssertionBody(t *testing.T, key []byte, routeClass, body, nonce string) string {
	t.Helper()
	now := time.Now().UTC()
	digest := sha256.Sum256([]byte(body))
	token, err := controlapi.IssueServiceAssertion(key, controlapi.ServiceAssertion{
		KeyID: "service-2026-08", AccountID: ownerAccountID, RouteClass: routeClass,
		Audience: "fort-control", RequestDigest: hex.EncodeToString(digest[:]),
		IssuedAt: now.Add(-time.Second), ExpiresAt: now.Add(30 * time.Second), Nonce: nonce,
	})
	if err != nil {
		t.Fatalf("issue assertion: %v", err)
	}
	return token
}
