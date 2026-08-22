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

const cursorRouteClass = "owner.events.read"

type eventStore interface {
	controlapi.NonceClaimer
	controlapi.CursorReader
	Close() error
}

type eventStoreOpener func(context.Context, string) (eventStore, error)

var productionCursorEndpoint = newCursorEndpoint(os.Getenv, func(ctx context.Context, databaseURL string) (eventStore, error) {
	return postgres.OpenSharedPool(ctx, databaseURL)
})

// Handler is the bounded Vercel Go Function entrypoint for
// /api/v2/events/cursor. Streaming remains in the gateway's Node.js route.
func Handler(response http.ResponseWriter, request *http.Request) {
	productionCursorEndpoint.ServeHTTP(response, request)
}

func newCursorEndpoint(getenv func(string) string, open eventStoreOpener) http.Handler {
	state := &cursorEndpointState{getenv: getenv, open: open}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeCursorError(response, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		handler, err := state.handler(request.Context())
		if err != nil {
			writeCursorError(response, http.StatusServiceUnavailable, "cursor_unavailable")
			return
		}
		handler.ServeHTTP(response, request)
	})
}

type cursorEndpointState struct {
	mu                 sync.Mutex
	getenv             func(string) string
	open               eventStoreOpener
	initializedHandler http.Handler
}

func (state *cursorEndpointState) handler(ctx context.Context) (http.Handler, error) {
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
		cursorRouteClass,
		controlapi.CursorHandler(store),
	)
	return state.initializedHandler, nil
}

func writeCursorError(response http.ResponseWriter, status int, code string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]string{"code": code})
}
