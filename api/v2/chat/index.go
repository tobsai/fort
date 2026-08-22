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
)

type chatControlStore interface {
	controlapi.NonceClaimer
	controlapi.AgentDirectChatRepository
	controlapi.AgentConversationMutationRepository
	Close() error
}

type chatControlStoreOpener func(context.Context, string) (chatControlStore, error)

var productionChatEndpoint = newChatEndpoint(os.Getenv, func(ctx context.Context, databaseURL string) (chatControlStore, error) {
	ring, err := postgres.BodyKeyRingFromEnvironment(os.Getenv)
	if err != nil {
		return nil, err
	}
	return postgres.OpenSharedPoolWithKeyRing(ctx, databaseURL, ring)
})

// Handler is the bounded Vercel Go Function behind direct stable-Agent chat
// rewrites. The public parent chain is reconstructed into PathValue fields;
// account identity comes only from the signed service assertion.
func Handler(response http.ResponseWriter, request *http.Request) {
	productionChatEndpoint.ServeHTTP(response, request)
}

func newChatEndpoint(getenv func(string) string, open chatControlStoreOpener) http.Handler {
	state := &chatEndpointState{getenv: getenv, open: open}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		route, forwarded, ok := rewrittenChatRoute(request)
		if !ok {
			writeChatEndpointError(response, http.StatusNotFound, "not_found")
			return
		}
		if route != "conversation" && !controlapi.CloudWriteAuthorityActive(state.getenv) {
			writeChatEndpointError(response, http.StatusConflict, "write_authority_inactive")
			return
		}
		handlers, err := state.load(request.Context())
		if err != nil {
			writeChatEndpointError(response, http.StatusServiceUnavailable, "agent_chat_unavailable")
			return
		}
		handlers[route].ServeHTTP(response, forwarded)
	})
}

type chatEndpointState struct {
	mu       sync.Mutex
	getenv   func(string) string
	open     chatControlStoreOpener
	handlers map[string]http.Handler
}

func (state *chatEndpointState) load(ctx context.Context) (map[string]http.Handler, error) {
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
	if _, err := postgres.BodyKeyRingFromEnvironment(state.getenv); err != nil {
		return nil, controlapi.ErrAssertionConfiguration
	}
	store, err := state.open(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	verifier, err := controlapi.ServiceAssertionVerifierFromEnvironment(state.getenv, store)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	state.handlers = map[string]http.Handler{
		"conversation": controlapi.RequireServiceAssertion(
			verifier, "owner.agent_conversations.read", controlapi.AgentConversationProjectionHandler(store),
		),
		"conversation_mutation": controlapi.RequireServiceAssertion(
			verifier, "owner.agent_conversations.mutate", controlapi.AgentConversationMutationHandler(store, nil),
		),
		"turns": controlapi.RequireServiceAssertion(
			verifier, "owner.agent_turns.send", controlapi.AgentConversationTurnsHandler(store, nil),
		),
		"retry": controlapi.RequireServiceAssertion(
			verifier, "owner.agent_targets.retry", controlapi.AgentTargetRetryHandler(store, nil),
		),
		"cancel": controlapi.RequireServiceAssertion(
			verifier, "owner.agent_targets.cancel", controlapi.AgentTargetCancelHandler(store, nil),
		),
	}
	return state.handlers, nil
}

func rewrittenChatRoute(request *http.Request) (string, *http.Request, bool) {
	query := request.URL.Query()
	resources := query["resource"]
	if len(resources) != 1 {
		return "", nil, false
	}
	resource := resources[0]
	route := resource
	expectedMethod := http.MethodPost
	expectedKeys := 3
	if resource == "conversation" {
		if request.Method == http.MethodPatch {
			route = "conversation_mutation"
			expectedMethod = http.MethodPatch
		} else {
			expectedMethod = http.MethodGet
		}
	} else if resource == "retry" || resource == "cancel" {
		expectedKeys = 4
	} else if resource != "turns" {
		return "", nil, false
	}
	if request.Method != expectedMethod || len(query) != expectedKeys ||
		len(query["agent_id"]) != 1 || len(query["conversation_id"]) != 1 {
		return "", nil, false
	}
	agentID := strings.TrimSpace(query.Get("agent_id"))
	conversationID := strings.TrimSpace(query.Get("conversation_id"))
	if !chatPathIdentity(agentID) || !chatPathIdentity(conversationID) {
		return "", nil, false
	}
	forwarded := request.Clone(request.Context())
	forwarded.URL = cloneChatURL(request.URL)
	forwarded.URL.RawQuery = ""
	forwarded.SetPathValue("agent_id", agentID)
	forwarded.SetPathValue("conversation_id", conversationID)
	if expectedKeys == 4 {
		if len(query["target_id"]) != 1 {
			return "", nil, false
		}
		targetID := strings.TrimSpace(query.Get("target_id"))
		if !chatPathIdentity(targetID) {
			return "", nil, false
		}
		forwarded.SetPathValue("target_id", targetID)
	}
	return route, forwarded, true
}

func chatPathIdentity(value string) bool {
	return value != "" && len([]byte(value)) <= 512 && !strings.ContainsAny(value, "\r\n\x00")
}

func cloneChatURL(source *url.URL) *url.URL {
	if source == nil {
		return &url.URL{}
	}
	clone := *source
	return &clone
}

func writeChatEndpointError(response http.ResponseWriter, status int, code string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]string{"code": code})
}
