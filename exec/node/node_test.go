package node

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/exec/fake"
)

// TestTokenBecomesLiveWithoutRestart is the regression guard for spec 024: the
// first `mesh invite` mints the mesh token inside the running daemon, so the
// node server must observe it without a restart. The token is read per-request
// via a func() string rather than captured once at construction.
func TestTokenBecomesLiveWithoutRestart(t *testing.T) {
	tok := ""
	srv := New(fake.New(), func() string { return tok })
	mux := http.NewServeMux()
	srv.Register(mux)
	req := httptest.NewRequest("POST", "/api/exec", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer later")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("empty token: %d, want 403", rec.Code)
	}
	tok = "later" // first `mesh invite` minted the token — no restart
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/exec", strings.NewReader("{}")))
	// wrong/missing header still 401
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no header: %d, want 401", rec.Code)
	}

	// A request WITH the now-valid token must pass auth and reach handleExec.
	req = httptest.NewRequest("POST", "/api/exec", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer later")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authed: %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Fatalf("content-type = %q, want application/x-ndjson", ct)
	}
}

type cancelOnlyRuntime struct {
	run        *cancelOnlyRun
	dispatched chan struct{}
}

func (r *cancelOnlyRuntime) Name() string { return "cancel-only" }

func (r *cancelOnlyRuntime) Dispatch(context.Context, runtime.RunSpec) (runtime.Run, error) {
	r.run = &cancelOnlyRun{
		events:   make(chan runtime.RunEvent),
		canceled: make(chan struct{}),
	}
	close(r.dispatched)
	return r.run, nil
}

type cancelOnlyRun struct {
	events   chan runtime.RunEvent
	canceled chan struct{}
	once     sync.Once
}

func (r *cancelOnlyRun) ID() string                      { return "silent" }
func (r *cancelOnlyRun) Stream() <-chan runtime.RunEvent { return r.events }
func (r *cancelOnlyRun) Signal(string) error             { return nil }
func (r *cancelOnlyRun) Cancel() error {
	r.once.Do(func() {
		close(r.canceled)
		close(r.events)
	})
	return nil
}
func (r *cancelOnlyRun) Status() runtime.Status {
	select {
	case <-r.canceled:
		return runtime.Status{State: runtime.StateCanceled}
	default:
		return runtime.Status{State: runtime.StateRunning}
	}
}
func (r *cancelOnlyRun) Wait() runtime.Status { return r.Status() }

func TestRequestContextCancelStopsSilentRun(t *testing.T) {
	rt := &cancelOnlyRuntime{dispatched: make(chan struct{})}
	srv := New(rt, func() string { return "token" })
	mux := http.NewServeMux()
	srv.Register(mux)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("POST", "/api/exec", strings.NewReader(`{"run_id":"silent","agent":"test"}`)).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer token")
	done := make(chan struct{})
	go func() {
		mux.ServeHTTP(httptest.NewRecorder(), req)
		close(done)
	}()
	<-rt.dispatched
	t.Cleanup(func() { _ = rt.run.Cancel() })
	cancel()

	select {
	case <-rt.run.canceled:
	case <-time.After(time.Second):
		t.Fatal("request cancellation did not invoke runtime.Run.Cancel")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("node handler did not return after canceling a silent run")
	}
}

type countingCancelRuntime struct {
	run        *countingCancelRun
	dispatched chan struct{}
}

func (r *countingCancelRuntime) Name() string { return "counting-cancel" }

func (r *countingCancelRuntime) Dispatch(context.Context, runtime.RunSpec) (runtime.Run, error) {
	r.run = &countingCancelRun{
		events:   make(chan runtime.RunEvent),
		canceled: make(chan struct{}),
	}
	close(r.dispatched)
	return r.run, nil
}

type countingCancelRun struct {
	events   chan runtime.RunEvent
	canceled chan struct{}
	once     sync.Once

	mu          sync.Mutex
	cancelCalls int
}

func (r *countingCancelRun) ID() string                      { return "counting" }
func (r *countingCancelRun) Stream() <-chan runtime.RunEvent { return r.events }
func (r *countingCancelRun) Signal(string) error             { return nil }
func (r *countingCancelRun) Cancel() error {
	r.mu.Lock()
	r.cancelCalls++
	r.mu.Unlock()
	r.once.Do(func() { close(r.canceled) })
	return nil
}
func (r *countingCancelRun) Status() runtime.Status {
	select {
	case <-r.canceled:
		return runtime.Status{State: runtime.StateCanceled}
	default:
		return runtime.Status{State: runtime.StateRunning}
	}
}
func (r *countingCancelRun) Wait() runtime.Status { return r.Status() }
func (r *countingCancelRun) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cancelCalls
}

func waitTracked(t *testing.T, srv *Server, id string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		if _, ok := srv.lookup(id); ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %q was not tracked", id)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestRequestAndEndpointCancellationInvokeRunCancelOnce(t *testing.T) {
	rt := &countingCancelRuntime{dispatched: make(chan struct{})}
	srv := New(rt, func() string { return "token" })
	mux := http.NewServeMux()
	srv.Register(mux)

	ctx, cancelRequest := context.WithCancel(context.Background())
	req := httptest.NewRequest("POST", "/api/exec", strings.NewReader(`{"RunID":"counting","Agent":"test"}`)).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer token")
	requestDone := make(chan struct{})
	go func() {
		mux.ServeHTTP(httptest.NewRecorder(), req)
		close(requestDone)
	}()
	<-rt.dispatched
	waitTracked(t, srv, "counting")

	cancelRequest()
	select {
	case <-rt.run.canceled:
	case <-time.After(time.Second):
		t.Fatal("request cancellation did not reach the run")
	}

	cancelReq := httptest.NewRequest("POST", "/api/exec/counting/cancel", nil)
	cancelReq.Header.Set("Authorization", "Bearer token")
	cancelRec := httptest.NewRecorder()
	mux.ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusNoContent {
		t.Fatalf("cancel endpoint status = %d, want 204", cancelRec.Code)
	}
	if got := rt.run.calls(); got != 1 {
		t.Fatalf("Run.Cancel calls = %d, want 1 across request and endpoint cancellation", got)
	}

	close(rt.run.events)
	select {
	case <-requestDone:
	case <-time.After(time.Second):
		t.Fatal("node handler did not return after the run stream closed")
	}
}

type terminalRuntime struct {
	run        *terminalRun
	dispatched chan struct{}
}

func (r *terminalRuntime) Name() string { return "terminal" }

func (r *terminalRuntime) Dispatch(context.Context, runtime.RunSpec) (runtime.Run, error) {
	r.run = &terminalRun{events: make(chan runtime.RunEvent)}
	close(r.dispatched)
	return r.run, nil
}

type terminalRun struct {
	events chan runtime.RunEvent

	mu          sync.Mutex
	cancelCalls int
}

func (r *terminalRun) ID() string                      { return "terminal" }
func (r *terminalRun) Stream() <-chan runtime.RunEvent { return r.events }
func (r *terminalRun) Signal(string) error             { return nil }
func (r *terminalRun) Cancel() error {
	r.mu.Lock()
	r.cancelCalls++
	r.mu.Unlock()
	return nil
}
func (r *terminalRun) Status() runtime.Status {
	return runtime.Status{State: runtime.StateSucceeded}
}
func (r *terminalRun) Wait() runtime.Status { return r.Status() }
func (r *terminalRun) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cancelCalls
}

func TestRequestCancellationDoesNotCancelTerminalRun(t *testing.T) {
	rt := &terminalRuntime{dispatched: make(chan struct{})}
	srv := New(rt, func() string { return "token" })
	mux := http.NewServeMux()
	srv.Register(mux)

	ctx, cancelRequest := context.WithCancel(context.Background())
	req := httptest.NewRequest("POST", "/api/exec", strings.NewReader(`{"RunID":"terminal","Agent":"test"}`)).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer token")
	requestDone := make(chan struct{})
	go func() {
		mux.ServeHTTP(httptest.NewRecorder(), req)
		close(requestDone)
	}()
	<-rt.dispatched
	waitTracked(t, srv, "terminal")

	cancelRequest()
	cancelReq := httptest.NewRequest("POST", "/api/exec/terminal/cancel", nil)
	cancelReq.Header.Set("Authorization", "Bearer token")
	cancelRec := httptest.NewRecorder()
	mux.ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusNoContent {
		t.Fatalf("cancel endpoint status = %d, want 204", cancelRec.Code)
	}
	if got := rt.run.calls(); got != 0 {
		t.Fatalf("Run.Cancel calls = %d, want 0 after terminal status", got)
	}

	close(rt.run.events)
	select {
	case <-requestDone:
	case <-time.After(time.Second):
		t.Fatal("node handler did not return after the terminal stream closed")
	}
}

type fakeCapabilityRegistry struct {
	current       corecap.NodeInventory
	refreshResult corecap.NodeInventory
	refreshCalls  int
}

func (f *fakeCapabilityRegistry) Current() corecap.NodeInventory { return f.current }
func (f *fakeCapabilityRegistry) Refresh(_ context.Context, request corecap.RecheckRequest) (corecap.NodeInventory, error) {
	f.refreshCalls++
	if request.ProtocolVersion != 1 {
		return corecap.NodeInventory{}, context.Canceled
	}
	return f.refreshResult, nil
}

func TestCapabilityRoutesAreAbsentUnlessRegistryIsWired(t *testing.T) {
	srv := New(fake.New(), func() string { return "token" })
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("dashboard"))
	})
	srv.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/node/capabilities", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want old-node 404", rec.Code)
	}
}

func TestCapabilityInventoryAndRecheckRequireMeshAuth(t *testing.T) {
	inventory := corecap.NodeInventory{
		ProtocolVersion: 1, CatalogVersion: 1, ProfileMappingVersion: 1,
		NodeID: "node-1", ObservedAt: time.Unix(1, 0).UTC(),
		State: corecap.MachinePartial, Reason: corecap.ReasonAuthRequired,
		Profiles: []corecap.ProfileOffer{}, Offers: []corecap.LogicalOffer{},
		Bindings: []corecap.ExecutionBindingOffer{},
	}
	registry := &fakeCapabilityRegistry{current: inventory, refreshResult: inventory}
	srv := New(fake.New(), func() string { return "token" })
	srv.UseCapabilities(registry)
	mux := http.NewServeMux()
	srv.Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/node/capabilities", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated GET = %d, want 401", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/node/capabilities", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d: %s", rec.Code, rec.Body.String())
	}
	var got corecap.NodeInventory
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.NodeID != "node-1" || got.Profiles == nil || got.Offers == nil || got.Bindings == nil {
		t.Fatalf("inventory = %#v", got)
	}

	body := `{"protocol_version":1,"request_id":"018f3f1c-7d3a-7c1d-a176-9c52c606c6e4","mode":"planning","max_age_seconds":60,"adapters":["profile.codex.native"]}`
	req = httptest.NewRequest(http.MethodPost, "/api/node/capabilities/recheck", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || registry.refreshCalls != 1 {
		t.Fatalf("POST = %d calls=%d body=%s", rec.Code, registry.refreshCalls, rec.Body.String())
	}
}

func TestCapabilityRecheckRejectsUnknownFieldsBeforeProbe(t *testing.T) {
	registry := &fakeCapabilityRegistry{}
	srv := New(fake.New(), func() string { return "token" })
	srv.UseCapabilities(registry)
	mux := http.NewServeMux()
	srv.Register(mux)

	body := `{"protocol_version":1,"request_id":"018f3f1c-7d3a-7c1d-a176-9c52c606c6e4","mode":"planning","max_age_seconds":60,"adapters":["profile.codex.native"],"shell":"please"}`
	req := httptest.NewRequest(http.MethodPost, "/api/node/capabilities/recheck", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || registry.refreshCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, registry.refreshCalls, rec.Body.String())
	}
}
