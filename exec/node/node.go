// Package node exposes a Fort's local runtime over HTTP so another Fort — the
// control plane — can dispatch runs to this machine (spec 022). It depends only
// on core/runtime (the interface); cmd/fort injects the concrete native runtime.
//
// The wire protocol is deliberately small:
//
//	POST /api/exec              body: RunSpec JSON
//	                           -> application/x-ndjson: one RunEvent per line,
//	                              flushed live, until a terminal exited/error, EOF.
//	POST /api/exec/{id}/signal  body: raw input  -> HITL stdin injection
//	POST /api/exec/{id}/cancel                    -> cancel a running remote run
//
// Every route requires "Authorization: Bearer <token>" (constant-time compared):
// the endpoint runs arbitrary agent CLIs, so it is never left unauthenticated.
package node

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/tobsai/fort/core/runtime"
)

// Server serves the inter-Fort execution endpoint over an injected runtime.
type Server struct {
	rt    runtime.Runtime
	token string

	mu   sync.Mutex
	runs map[string]runtime.Run // in-flight runs, for signal/cancel
}

// New builds a node server over rt. token must be non-empty for the endpoint to
// accept any request.
func New(rt runtime.Runtime, token string) *Server {
	return &Server{rt: rt, token: token, runs: map[string]runtime.Run{}}
}

// Register mounts the exec routes onto mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/exec", s.handleExec)
	mux.HandleFunc("POST /api/exec/{id}/signal", s.handleSignal)
	mux.HandleFunc("POST /api/exec/{id}/cancel", s.handleCancel)
}

// authed reports whether the request carries the shared bearer token.
func (s *Server) authed(w http.ResponseWriter, r *http.Request) bool {
	if s.token == "" {
		http.Error(w, "node: exec endpoint disabled (no FORT_NODE_TOKEN)", http.StatusForbidden)
		return false
	}
	got := []byte(r.Header.Get("Authorization"))
	want := []byte("Bearer " + s.token)
	if subtle.ConstantTimeCompare(got, want) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (s *Server) track(id string, run runtime.Run) {
	s.mu.Lock()
	s.runs[id] = run
	s.mu.Unlock()
}

func (s *Server) untrack(id string) {
	s.mu.Lock()
	delete(s.runs, id)
	s.mu.Unlock()
}

func (s *Server) lookup(id string) (runtime.Run, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[id]
	return run, ok
}

// handleExec dispatches a RunSpec on the local runtime and streams its events
// back as newline-delimited JSON. The request context is the run's context, so a
// client disconnect terminates the local run.
func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	if !s.authed(w, r) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	var spec runtime.RunSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		http.Error(w, "bad run spec", http.StatusBadRequest)
		return
	}
	// The caller's placement label is spent; this node always runs locally.
	spec.Machine = ""

	w.Header().Set("Content-Type", "application/x-ndjson")
	enc := json.NewEncoder(w)

	run, err := s.rt.Dispatch(r.Context(), spec)
	if err != nil {
		_ = enc.Encode(runtime.RunEvent{RunID: spec.RunID, Type: runtime.EventError, Time: time.Now(), Data: err.Error()})
		flusher.Flush()
		return
	}
	s.track(spec.RunID, run)
	defer s.untrack(spec.RunID)

	for ev := range run.Stream() {
		if err := enc.Encode(ev); err != nil {
			// Client vanished mid-stream — stop and cancel the local work.
			_ = run.Cancel()
			return
		}
		flusher.Flush()
	}
}

// handleSignal injects HITL input into a tracked run.
func (s *Server) handleSignal(w http.ResponseWriter, r *http.Request) {
	if !s.authed(w, r) {
		return
	}
	run, ok := s.lookup(r.PathValue("id"))
	if !ok {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err := run.Signal(string(body)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleCancel cancels a tracked run.
func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	if !s.authed(w, r) {
		return
	}
	run, ok := s.lookup(r.PathValue("id"))
	if !ok {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	_ = run.Cancel()
	w.WriteHeader(http.StatusNoContent)
}
