package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/cloud/postgres"
	"github.com/tobsai/fort/cloud/securebody"
)

type ownerControlStore interface {
	controlapi.NonceClaimer
	controlapi.AgentLister
	controlapi.AgentReader
	controlapi.AgentLifecycleCommandRepository
	controlapi.AgentMutationRepository
	controlapi.AgentConversationReader
	controlapi.AgentConversationCreateRepository
	controlapi.GroupLister
	controlapi.GroupCreateRepository
	controlapi.GroupDetailRepository
	controlapi.GroupTurnRepository
	controlapi.GroupMutationRepository
	controlapi.GroupMembersRepository
	controlapi.HumanHandoffRepository
	controlapi.RoutineOwnerRepository
	Close() error
}

type ownerControlStoreOpener func(context.Context, string, securebody.KeyRing) (ownerControlStore, error)

var productionOwnerEndpoint = newOwnerEndpointWithAgentLifecycle(
	os.Getenv,
	func(ctx context.Context, databaseURL string, ring securebody.KeyRing) (ownerControlStore, error) {
		return postgres.OpenSharedPoolWithKeyRing(ctx, databaseURL, ring)
	},
	controlapi.NoEligibleAgentOptions(),
	productionRebindAcceptanceTokens(os.Getenv),
)

func productionRebindAcceptanceTokens(getenv func(string) string) controlapi.RebindAcceptanceTokens {
	tokens, err := controlapi.HMACRebindAcceptanceTokensFromEnvironment(getenv)
	if err != nil {
		return nil
	}
	return tokens
}

// Handler is the bounded Vercel Go Function behind semantic parameterized
// owner reads. vercel.json rewrites preserve the public /api/v2 resource URL
// and pass only the resolved resource identity to this dispatcher.
func Handler(response http.ResponseWriter, request *http.Request) {
	productionOwnerEndpoint.ServeHTTP(response, request)
}

func newOwnerEndpoint(getenv func(string) string, open ownerControlStoreOpener) http.Handler {
	return newOwnerEndpointWithAgentLifecycle(getenv, open, controlapi.NoEligibleAgentOptions(), nil)
}

func newOwnerEndpointWithAgentLifecycle(
	getenv func(string) string,
	open ownerControlStoreOpener,
	resolver controlapi.AgentOptionResolver,
	tokens controlapi.RebindAcceptanceTokens,
) http.Handler {
	state := &ownerEndpointState{getenv: getenv, open: open, agentOptions: resolver, rebindTokens: tokens}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		route, method, forwarded, ok := ownerRoute(request)
		if !ok {
			writeOwnerEndpointError(response, http.StatusNotFound, "not_found")
			return
		}
		if request.Method != method {
			response.Header().Set("Allow", method)
			writeOwnerEndpointError(response, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		if method != http.MethodGet && !controlapi.CloudWriteAuthorityActive(state.getenv) {
			writeOwnerEndpointError(response, http.StatusConflict, "write_authority_inactive")
			return
		}
		handlers, err := state.load(request.Context())
		if err != nil {
			writeOwnerEndpointError(response, http.StatusServiceUnavailable, "owner_read_unavailable")
			return
		}
		handlers[route].ServeHTTP(response, forwarded)
	})
}

type ownerEndpointState struct {
	mu           sync.Mutex
	getenv       func(string) string
	open         ownerControlStoreOpener
	agentOptions controlapi.AgentOptionResolver
	rebindTokens controlapi.RebindAcceptanceTokens
	handlers     map[string]http.Handler
}

func (state *ownerEndpointState) load(ctx context.Context) (map[string]http.Handler, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.handlers != nil {
		return state.handlers, nil
	}
	if state.getenv == nil || state.open == nil {
		return nil, controlapi.ErrAssertionConfiguration
	}
	databaseURL := strings.TrimSpace(state.getenv("DATABASE_URL"))
	if databaseURL == "" || strings.TrimSpace(state.getenv("FORT_CONTROL_ASSERTION_KEYS_JSON")) == "" {
		return nil, controlapi.ErrAssertionConfiguration
	}
	ring, err := postgres.BodyKeyRingFromEnvironment(state.getenv)
	if err != nil {
		return nil, err
	}
	store, err := state.open(ctx, databaseURL, ring)
	if err != nil {
		return nil, err
	}
	verifier, err := controlapi.ServiceAssertionVerifierFromEnvironment(state.getenv, store)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	state.handlers = map[string]http.Handler{
		"agents": controlapi.RequireServiceAssertion(
			verifier, "owner.agents.list", controlapi.AgentsHandler(store),
		),
		"agent_create": controlapi.RequireServiceAssertion(
			verifier, "owner.agents.create", controlapi.AgentCreateHandler(store, state.agentOptions, nil),
		),
		"agent": controlapi.RequireServiceAssertion(
			verifier, "owner.agents.read", controlapi.AgentDetailHandler(store),
		),
		"agent_mutation": controlapi.RequireServiceAssertion(
			verifier, "owner.agents.mutate", controlapi.AgentMutationHandler(store, nil),
		),
		"agent_rebind": controlapi.RequireServiceAssertion(
			verifier, "owner.agents.rebind", controlapi.AgentRebindHandler(store, state.agentOptions, state.rebindTokens, nil),
		),
		"agent_conversations": controlapi.RequireServiceAssertion(
			verifier, "owner.agent_conversations.list", controlapi.AgentConversationsHandler(store),
		),
		"agent_conversation_create": controlapi.RequireServiceAssertion(
			verifier, "owner.agent_conversations.create", controlapi.AgentConversationCreateHandler(store, nil),
		),
		"agent_canonical_conversation": controlapi.RequireServiceAssertion(
			verifier, "owner.agent_conversations.canonical", controlapi.AgentCanonicalConversationHandler(store),
		),
		"groups": controlapi.RequireServiceAssertion(
			verifier, "owner.groups.list", controlapi.GroupsHandler(store),
		),
		"group_create": controlapi.RequireServiceAssertion(
			verifier, "owner.groups.create", controlapi.GroupCreateHandler(store, nil),
		),
		"group_detail": controlapi.RequireServiceAssertion(
			verifier, "owner.groups.read", controlapi.GroupDetailHandler(store),
		),
		"group_mutation": controlapi.RequireServiceAssertion(
			verifier, "owner.groups.mutate", controlapi.GroupMutationHandler(store, nil),
		),
		"group_members": controlapi.RequireServiceAssertion(
			verifier, "owner.group_members.replace", controlapi.GroupMembersHandler(store, nil),
		),
		"group_turns": controlapi.RequireServiceAssertion(
			verifier, "owner.group_turns.send", controlapi.GroupTurnsHandler(store, nil),
		),
		"handoffs": controlapi.RequireServiceAssertion(
			verifier, "owner.handoffs.list", controlapi.HandoffsHandler(store),
		),
		"handoff_create": controlapi.RequireServiceAssertion(
			verifier, "owner.handoffs.create", controlapi.HandoffCreateHandler(store, nil),
		),
		"handoff_detail": controlapi.RequireServiceAssertion(
			verifier, "owner.handoffs.read", controlapi.HandoffDetailHandler(store),
		),
		"handoff_cancel": controlapi.RequireServiceAssertion(
			verifier, "owner.handoffs.cancel", controlapi.HandoffCancelHandler(store, nil),
		),
		"routines_list": controlapi.RequireServiceAssertion(
			verifier, "owner.routines.list", controlapi.RoutinesHandler(store, nil),
		),
		"routines_create": controlapi.RequireServiceAssertion(
			verifier, "owner.routines.create", controlapi.RoutinesHandler(store, nil),
		),
		"routine_mutation": controlapi.RequireServiceAssertion(
			verifier, "owner.routines.mutate", controlapi.RoutineMutationHandler(store, nil),
		),
		"routine_test": controlapi.RequireServiceAssertion(
			verifier, "owner.routines.test", controlapi.RoutineTestHandler(store, nil),
		),
		"routine_runs": controlapi.RequireServiceAssertion(
			verifier, "owner.routines.runs", controlapi.RoutineRunsHandler(store),
		),
	}
	return state.handlers, nil
}

func ownerRoute(request *http.Request) (string, string, *http.Request, bool) {
	query := request.URL.Query()
	resources := query["resource"]
	if len(resources) != 1 {
		return "", "", nil, false
	}
	resource := resources[0]
	forwarded := request.Clone(request.Context())
	forwarded.URL = cloneOwnerURL(request.URL)
	switch resource {
	case "agents":
		if request.Method == http.MethodPost {
			if len(query) != 1 {
				return "", "", nil, false
			}
			forwarded.URL.RawQuery = ""
			return "agent_create", http.MethodPost, forwarded, true
		}
		for key, values := range query {
			if key == "resource" {
				continue
			}
			if key != "state" || len(values) != 1 {
				return "", "", nil, false
			}
		}
		forwardedQuery := forwarded.URL.Query()
		forwardedQuery.Del("resource")
		forwarded.URL.RawQuery = forwardedQuery.Encode()
		return "agents", http.MethodGet, forwarded, true
	case "agent", "agent_canonical_conversation":
		if len(query) != 2 || len(query["agent_id"]) != 1 {
			return "", "", nil, false
		}
		agentID := strings.TrimSpace(query.Get("agent_id"))
		if agentID == "" || len([]byte(agentID)) > 512 || strings.ContainsAny(agentID, "\r\n\x00") {
			return "", "", nil, false
		}
		forwarded.SetPathValue("agent_id", agentID)
		forwarded.URL.RawQuery = ""
		if resource == "agent" && request.Method == http.MethodPatch {
			return "agent_mutation", http.MethodPatch, forwarded, true
		}
		return resource, http.MethodGet, forwarded, true
	case "agent_rebind":
		if len(query) != 2 || len(query["agent_id"]) != 1 {
			return "", "", nil, false
		}
		agentID := strings.TrimSpace(query.Get("agent_id"))
		if agentID == "" || len([]byte(agentID)) > 512 || strings.ContainsAny(agentID, "\r\n\x00") {
			return "", "", nil, false
		}
		forwarded.SetPathValue("agent_id", agentID)
		forwarded.URL.RawQuery = ""
		return "agent_rebind", http.MethodPost, forwarded, true
	case "agent_conversations":
		if len(query) != 2 || len(query["agent_id"]) != 1 {
			return "", "", nil, false
		}
		agentID := strings.TrimSpace(query.Get("agent_id"))
		if agentID == "" || len([]byte(agentID)) > 512 || strings.ContainsAny(agentID, "\r\n\x00") {
			return "", "", nil, false
		}
		forwarded.SetPathValue("agent_id", agentID)
		forwarded.URL.RawQuery = ""
		if request.Method == http.MethodPost {
			return "agent_conversation_create", http.MethodPost, forwarded, true
		}
		return resource, http.MethodGet, forwarded, true
	case "groups":
		if request.Method == http.MethodPost {
			if len(query) != 1 {
				return "", "", nil, false
			}
			forwarded.URL.RawQuery = ""
			return "group_create", http.MethodPost, forwarded, true
		}
		for key, values := range query {
			if key == "resource" {
				continue
			}
			if key != "state" || len(values) != 1 {
				return "", "", nil, false
			}
		}
		forwardedQuery := forwarded.URL.Query()
		forwardedQuery.Del("resource")
		forwarded.URL.RawQuery = forwardedQuery.Encode()
		return resource, http.MethodGet, forwarded, true
	case "group_detail", "group_turns", "group_members":
		if len(query) != 2 || len(query["group_id"]) != 1 {
			return "", "", nil, false
		}
		groupID := strings.TrimSpace(query.Get("group_id"))
		if groupID == "" || len([]byte(groupID)) > 512 || strings.ContainsAny(groupID, "\r\n\x00") {
			return "", "", nil, false
		}
		forwarded.SetPathValue("group_id", groupID)
		forwarded.URL.RawQuery = ""
		method := http.MethodGet
		if resource == "group_detail" && request.Method == http.MethodPatch {
			return "group_mutation", http.MethodPatch, forwarded, true
		}
		if resource == "group_turns" || resource == "group_members" {
			method = http.MethodPost
		}
		return resource, method, forwarded, true
	case "handoffs":
		if len(query) != 1 {
			return "", "", nil, false
		}
		forwarded.URL.RawQuery = ""
		if request.Method == http.MethodPost {
			return "handoff_create", http.MethodPost, forwarded, true
		}
		return resource, http.MethodGet, forwarded, true
	case "handoff_detail", "handoff_cancel":
		if len(query) != 2 || len(query["handoff_id"]) != 1 {
			return "", "", nil, false
		}
		handoffID := strings.TrimSpace(query.Get("handoff_id"))
		if handoffID == "" || len([]byte(handoffID)) > 512 || strings.ContainsAny(handoffID, "\r\n\x00") {
			return "", "", nil, false
		}
		forwarded.SetPathValue("handoff_id", handoffID)
		forwarded.URL.RawQuery = ""
		method := http.MethodGet
		if resource == "handoff_cancel" {
			method = http.MethodPost
		}
		return resource, method, forwarded, true
	case "routines":
		if len(query) != 2 || len(query["agent_id"]) != 1 {
			return "", "", nil, false
		}
		agentID := strings.TrimSpace(query.Get("agent_id"))
		if agentID == "" || len([]byte(agentID)) > 512 || strings.ContainsAny(agentID, "\r\n\x00") {
			return "", "", nil, false
		}
		forwarded.SetPathValue("agent_id", agentID)
		forwarded.URL.RawQuery = ""
		if request.Method == http.MethodPost {
			return "routines_create", http.MethodPost, forwarded, true
		}
		return "routines_list", http.MethodGet, forwarded, true
	case "routine_detail", "routine_test", "routine_runs":
		if len(query) != 3 || len(query["agent_id"]) != 1 || len(query["routine_id"]) != 1 {
			return "", "", nil, false
		}
		agentID := strings.TrimSpace(query.Get("agent_id"))
		routineID := strings.TrimSpace(query.Get("routine_id"))
		if agentID == "" || routineID == "" || len([]byte(agentID)) > 512 || len([]byte(routineID)) > 512 ||
			strings.ContainsAny(agentID, "\r\n\x00") || strings.ContainsAny(routineID, "\r\n\x00") {
			return "", "", nil, false
		}
		forwarded.SetPathValue("agent_id", agentID)
		forwarded.SetPathValue("routine_id", routineID)
		forwarded.URL.RawQuery = ""
		if resource == "routine_test" {
			return "routine_test", http.MethodPost, forwarded, true
		}
		if resource == "routine_runs" {
			return "routine_runs", http.MethodGet, forwarded, true
		}
		return "routine_mutation", http.MethodPatch, forwarded, true
	default:
		return "", "", nil, false
	}
}

func cloneOwnerURL(source *url.URL) *url.URL {
	if source == nil {
		return &url.URL{}
	}
	clone := *source
	return &clone
}

func writeOwnerEndpointError(response http.ResponseWriter, status int, code string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]string{"code": code})
}
