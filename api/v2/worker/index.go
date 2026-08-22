package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/cloud/postgres"
)

type workerControlStore interface {
	controlapi.WorkerRepository
	Close() error
}

type workerControlStoreOpener func(context.Context, string) (workerControlStore, error)

var productionWorkerEndpoint = newWorkerEndpoint(os.Getenv, func(ctx context.Context, databaseURL string) (workerControlStore, error) {
	ring, err := postgres.BodyKeyRingFromEnvironment(os.Getenv)
	if err != nil {
		return nil, err
	}
	return postgres.OpenSharedPoolWithKeyRing(ctx, databaseURL, ring)
}, time.Now)

// Handler is the bounded Vercel Go Function entrypoint for machine-only
// /api/v2/worker commands. It never accepts owner assertions or Cron secrets.
func Handler(response http.ResponseWriter, request *http.Request) {
	productionWorkerEndpoint.ServeHTTP(response, request)
}

func newWorkerEndpoint(getenv func(string) string, open workerControlStoreOpener, clock func() time.Time) http.Handler {
	state := &workerEndpointState{getenv: getenv, open: open, clock: clock}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeWorkerEndpointError(response, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		if !controlapi.CloudWriteAuthorityActive(state.getenv) {
			writeWorkerEndpointError(response, http.StatusConflict, "write_authority_inactive")
			return
		}
		handler, err := state.load(request.Context())
		if err != nil {
			writeWorkerEndpointError(response, http.StatusServiceUnavailable, "worker_unavailable")
			return
		}
		handler.ServeHTTP(response, request)
	})
}

type workerEndpointState struct {
	mu                 sync.Mutex
	getenv             func(string) string
	open               workerControlStoreOpener
	clock              func() time.Time
	initializedHandler http.Handler
}

func (state *workerEndpointState) load(ctx context.Context) (http.Handler, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.initializedHandler != nil {
		return state.initializedHandler, nil
	}
	if state.getenv == nil || state.open == nil || state.clock == nil {
		return nil, controlapi.ErrAssertionConfiguration
	}
	databaseURL := strings.TrimSpace(state.getenv("DATABASE_URL"))
	if databaseURL == "" {
		return nil, controlapi.ErrAssertionConfiguration
	}
	if _, err := postgres.BodyKeyRingFromEnvironment(state.getenv); err != nil {
		return nil, controlapi.ErrAssertionConfiguration
	}
	store, err := state.open(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	state.initializedHandler = controlapi.WorkerHandler(store, state.clock)
	return state.initializedHandler, nil
}

func writeWorkerEndpointError(response http.ResponseWriter, status int, code string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]string{"code": code})
}
