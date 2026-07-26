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
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/runtime"
)

// Server serves the inter-Fort execution endpoint over an injected runtime.
type Server struct {
	rt    runtime.Runtime
	token func() string
	caps  CapabilityRegistry

	mu   sync.Mutex
	runs map[string]*runHandle // in-flight runs, for signal/cancel
}

// CapabilityRegistry is the bounded discovery seam mounted only by a
// capability-aware node. Its absence intentionally leaves the new routes 404,
// so a coordinator can distinguish an old node from an empty inventory.
type CapabilityRegistry interface {
	Current() corecap.NodeInventory
	Refresh(context.Context, corecap.RecheckRequest) (corecap.NodeInventory, error)
}

type runHandle struct {
	run runtime.Run

	once      sync.Once
	cancelErr error
}

func (h *runHandle) cancel() error {
	h.once.Do(func() {
		if h.run.Status().Terminal() {
			return
		}
		h.cancelErr = h.run.Cancel()
	})
	return h.cancelErr
}

func (h *runHandle) complete() {
	h.once.Do(func() {})
}

// New builds a node server over rt. token is read fresh on every request, so
// the endpoint accepts requests as soon as a token exists — even if it is
// minted after the server is already mounted (e.g. a mesh invite writing the
// token into a running daemon). An empty token disables the endpoint (403).
func New(rt runtime.Runtime, token func() string) *Server {
	return &Server{rt: rt, token: token, runs: map[string]*runHandle{}}
}

// UseCapabilities enables the versioned mesh capability endpoints. Call it
// before Register.
func (s *Server) UseCapabilities(registry CapabilityRegistry) {
	s.caps = registry
}

// Register mounts the exec routes onto mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/exec", s.handleExec)
	mux.HandleFunc("POST /api/exec/{id}/signal", s.handleSignal)
	mux.HandleFunc("POST /api/exec/{id}/cancel", s.handleCancel)
	// Always own the versioned capability paths so the UI's GET / fallback
	// cannot disguise an old node as a successful HTML response.
	mux.HandleFunc("GET /api/node/capabilities", s.handleCapabilities)
	mux.HandleFunc("POST /api/node/capabilities/recheck", s.handleCapabilityRecheck)
}

// authed reports whether the request carries the shared bearer token.
func (s *Server) authed(w http.ResponseWriter, r *http.Request) bool {
	tok := s.token()
	if tok == "" {
		http.Error(w, "node: exec endpoint disabled (no FORT_NODE_TOKEN)", http.StatusForbidden)
		return false
	}
	got := []byte(r.Header.Get("Authorization"))
	want := []byte("Bearer " + tok)
	if subtle.ConstantTimeCompare(got, want) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (s *Server) track(id string, run runtime.Run) *runHandle {
	handle := &runHandle{run: run}
	s.mu.Lock()
	s.runs[id] = handle
	s.mu.Unlock()
	return handle
}

func (s *Server) untrack(id string) {
	s.mu.Lock()
	delete(s.runs, id)
	s.mu.Unlock()
}

func (s *Server) lookup(id string) (*runHandle, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	handle, ok := s.runs[id]
	return handle, ok
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
	// The caller's placement label is spent; this node always runs locally, and
	// in its own workspace — never a workdir path dictated by the caller.
	spec.Machine = ""
	spec.Workdir = ""

	w.Header().Set("Content-Type", "application/x-ndjson")
	enc := json.NewEncoder(w)

	run, err := s.rt.Dispatch(r.Context(), spec)
	if err != nil {
		_ = enc.Encode(runtime.RunEvent{RunID: spec.RunID, Type: runtime.EventError, Time: time.Now(), Data: err.Error()})
		flusher.Flush()
		return
	}
	handle := s.track(spec.RunID, run)
	requestDone := make(chan struct{})
	defer func() {
		handle.complete()
		close(requestDone)
		s.untrack(spec.RunID)
	}()
	go func() {
		select {
		case <-r.Context().Done():
			// A silent provider may never emit another frame, so waiting for an
			// Encode error is insufficient to notice a vanished client. Invoke
			// the runtime contract explicitly so native process groups die too.
			_ = handle.cancel()
		case <-requestDone:
		}
	}()

	for ev := range run.Stream() {
		if err := enc.Encode(ev); err != nil {
			// Client vanished mid-stream. Cancel the local work, then keep
			// draining the stream to completion: the runtime's event channel is
			// buffered, so a consumer that simply stops reading wedges the
			// producer goroutines on a full-channel send (Cancel kills the
			// process but cannot unblock a goroutine parked on a send). Draining
			// lets them finish and the run terminate instead of leaking.
			_ = handle.cancel()
			go drain(run)
			return
		}
		flusher.Flush()
	}
}

// drain discards any remaining events so a runtime's producer goroutines unblock
// and the run reaches a terminal state after the consumer has gone away.
func drain(run runtime.Run) {
	for range run.Stream() {
	}
}

// handleSignal injects HITL input into a tracked run.
func (s *Server) handleSignal(w http.ResponseWriter, r *http.Request) {
	if !s.authed(w, r) {
		return
	}
	handle, ok := s.lookup(r.PathValue("id"))
	if !ok {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err := handle.run.Signal(string(body)); err != nil {
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
	handle, ok := s.lookup(r.PathValue("id"))
	if !ok {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	_ = handle.cancel()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	if s.caps == nil {
		http.NotFound(w, r)
		return
	}
	if !s.authed(w, r) {
		return
	}
	writeBoundedJSON(w, s.caps.Current())
}

func (s *Server) handleCapabilityRecheck(w http.ResponseWriter, r *http.Request) {
	if s.caps == nil {
		http.NotFound(w, r)
		return
	}
	if !s.authed(w, r) {
		return
	}
	var request corecap.RecheckRequest
	if err := decodeStrictBounded(r.Body, 64*1024, &request); err != nil {
		http.Error(w, "bad capability recheck request", http.StatusBadRequest)
		return
	}
	inventory, err := s.caps.Refresh(r.Context(), request)
	if err != nil {
		http.Error(w, "bad capability recheck request", http.StatusBadRequest)
		return
	}
	writeBoundedJSON(w, inventory)
}

func decodeStrictBounded(body io.Reader, maximum int64, target any) error {
	limited := io.LimitReader(body, maximum+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if int64(len(raw)) > maximum {
		return fmt.Errorf("request exceeds %d bytes", maximum)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("request contains trailing JSON")
	}
	return nil
}

func writeBoundedJSON(w http.ResponseWriter, value any) {
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) > 512*1024 {
		http.Error(w, "capability response unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}
