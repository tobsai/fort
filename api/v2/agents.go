package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/cloud/postgres"
)

const agentsListRouteClass = "owner.agents.list"

type agentControlStore interface {
	controlapi.NonceClaimer
	controlapi.AgentLister
	Close() error
}

type agentControlStoreOpener func(context.Context, string) (agentControlStore, error)

var productionAgentsEndpoint = newAgentsEndpoint(os.Getenv, func(ctx context.Context, databaseURL string) (agentControlStore, error) {
	return postgres.OpenSharedPool(ctx, databaseURL)
})

// Handler is the Vercel Go Function entrypoint for GET /api/v2/agents.
func Handler(response http.ResponseWriter, request *http.Request) {
	productionAgentsEndpoint.ServeHTTP(response, request)
}

func newAgentsEndpoint(getenv func(string) string, open agentControlStoreOpener) http.Handler {
	state := &agentsEndpointState{getenv: getenv, open: open}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeAgentsError(response, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		handler, err := state.load(request.Context())
		if err != nil {
			writeAgentsError(response, http.StatusServiceUnavailable, "agent_list_unavailable")
			return
		}
		handler.ServeHTTP(response, request)
	})
}

type agentsEndpointState struct {
	mu                 sync.Mutex
	getenv             func(string) string
	open               agentControlStoreOpener
	initializedHandler http.Handler
}

func (state *agentsEndpointState) load(ctx context.Context) (http.Handler, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.initializedHandler != nil {
		return state.initializedHandler, nil
	}
	if state.getenv == nil || state.open == nil {
		return nil, controlapi.ErrAssertionConfiguration
	}
	databaseURL := strings.TrimSpace(state.getenv("DATABASE_URL"))
	if databaseURL == "" || strings.TrimSpace(state.getenv("FORT_CONTROL_ASSERTION_KEYS_JSON")) == "" {
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
	state.initializedHandler = controlapi.RequireServiceAssertion(
		verifier,
		agentsListRouteClass,
		controlapi.AgentsHandler(store),
	)
	return state.initializedHandler, nil
}

func writeAgentsError(response http.ResponseWriter, status int, code string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]string{"code": code})
}
