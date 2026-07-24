package remote

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tobsai/fort/core/engine"
	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/exec/fake"
	"github.com/tobsai/fort/exec/node"
)

// nodeServer wires a runtime behind the node exec endpoint on an httptest
// server and returns its base URL.
func nodeServer(t *testing.T, rt runtime.Runtime, token string) string {
	t.Helper()
	mux := http.NewServeMux()
	node.New(rt, func() string { return token }).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func drain(run runtime.Run) []runtime.RunEvent {
	var evs []runtime.RunEvent
	for ev := range run.Stream() {
		evs = append(evs, ev)
	}
	return evs
}

func types(evs []runtime.RunEvent) []runtime.EventType {
	out := make([]runtime.EventType, len(evs))
	for i, e := range evs {
		out[i] = e.Type
	}
	return out
}

func TestRoundTripStreamsEventsAndStatus(t *testing.T) {
	f := fake.New()
	base := nodeServer(t, f, "s3cr3t")
	rt := New("macbook-pro", base, "s3cr3t")

	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r1", Agent: "codex", Prompt: "hello"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	evs := drain(run)
	st := run.Wait()

	// fake emits started, message(prompt), exited(0).
	if len(evs) < 3 {
		t.Fatalf("events = %v, want >=3", types(evs))
	}
	if evs[0].Type != runtime.EventStarted || evs[1].Type != runtime.EventMessage || evs[1].Data != "hello" {
		t.Fatalf("stream = %+v", evs)
	}
	last := evs[len(evs)-1]
	if last.Type != runtime.EventExited || last.Code != 0 {
		t.Fatalf("terminal event = %+v", last)
	}
	if st.State != runtime.StateSucceeded {
		t.Fatalf("status = %+v, want succeeded", st)
	}
	// The RunID is preserved end to end.
	if evs[0].RunID != "r1" {
		t.Fatalf("run id not preserved: %q", evs[0].RunID)
	}
}

func TestNonZeroExitFails(t *testing.T) {
	f := fake.New()
	f.ExitCode = 2
	base := nodeServer(t, f, "tok")
	rt := New("mini", base, "tok")

	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r2", Agent: "hermes", Prompt: "x"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	drain(run)
	st := run.Wait()
	if st.State != runtime.StateFailed || st.ExitCode != 2 {
		t.Fatalf("status = %+v, want failed/2", st)
	}
}

type fatalRuntime struct{}

func (fatalRuntime) Name() string { return "fatal" }

func (fatalRuntime) Dispatch(_ context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	events := make(chan runtime.RunEvent, 2)
	events <- runtime.RunEvent{RunID: spec.RunID, Type: runtime.EventError, Time: time.Now(), Data: "provider exhausted retries"}
	events <- runtime.RunEvent{RunID: spec.RunID, Type: runtime.EventExited, Time: time.Now(), Code: 7}
	close(events)
	return &fatalRun{id: spec.RunID, events: events}, nil
}

type fatalRun struct {
	id     string
	events chan runtime.RunEvent
}

func (r *fatalRun) ID() string                      { return r.id }
func (r *fatalRun) Stream() <-chan runtime.RunEvent { return r.events }
func (r *fatalRun) Signal(string) error             { return nil }
func (r *fatalRun) Cancel() error                   { return nil }
func (r *fatalRun) Status() runtime.Status {
	return runtime.Status{State: runtime.StateFailed, ExitCode: 7, Err: "provider exhausted retries"}
}
func (r *fatalRun) Wait() runtime.Status { return r.Status() }

func TestFatalErrorAndExitPersistThroughNodeRemoteAndEngine(t *testing.T) {
	const fatal = "provider exhausted retries"
	base := nodeServer(t, fatalRuntime{}, "tok")
	rt := New("mini", base, "tok")

	rs, err := rules.Parse([]byte("version: 1\ndefaults:\n  route: hermes\n"))
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	e := engine.New(router.New(rs), rt, st, t.TempDir())

	got, err := e.SubmitAndWait(context.Background(), task.Task{ID: "fatal-remote", Title: "diagnose"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got.Status != "failed" || got.ExitCode != 7 || got.Error != fatal {
		t.Fatalf("persisted run = %+v, want failed/7 with error %q", got, fatal)
	}
}

func TestAuthRejectsBadToken(t *testing.T) {
	base := nodeServer(t, fake.New(), "right")
	rt := New("mini", base, "wrong")
	if _, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r3", Agent: "codex"}); err == nil {
		t.Fatal("expected dispatch to fail with a bad token")
	}
}

func TestDisabledWhenNoToken(t *testing.T) {
	base := nodeServer(t, fake.New(), "") // node with no token = endpoint disabled
	rt := New("mini", base, "anything")
	if _, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r4", Agent: "codex"}); err == nil {
		t.Fatal("expected dispatch to fail when the node has no token configured")
	}
}

func TestCancelStopsRun(t *testing.T) {
	f := fake.New()
	f.Block = true // hold the run open after started+message
	base := nodeServer(t, f, "tok")
	rt := New("mini", base, "tok")

	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r5", Agent: "codex", Prompt: "wait"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	// Read the first two events, then cancel.
	<-run.Stream() // started
	<-run.Stream() // message
	if err := run.Cancel(); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	// Drain whatever remains and confirm terminal-canceled.
	for range run.Stream() {
	}
	st := run.Wait()
	if st.State != runtime.StateCanceled {
		t.Fatalf("status = %+v, want canceled", st)
	}
}

func TestCancelAfterTerminalEventIsIdempotentAndPreservesResult(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseStream := func() { releaseOnce.Do(func() { close(release) }) }
	var cancelRequests atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/exec", func(w http.ResponseWriter, r *http.Request) {
		enc := json.NewEncoder(w)
		_ = enc.Encode(runtime.RunEvent{RunID: "terminal", Type: runtime.EventStarted, Time: time.Now()})
		_ = enc.Encode(runtime.RunEvent{RunID: "terminal", Type: runtime.EventExited, Time: time.Now()})
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	mux.HandleFunc("POST /api/exec/terminal/cancel", func(w http.ResponseWriter, _ *http.Request) {
		cancelRequests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Cleanup(releaseStream)

	run, err := New("mini", srv.URL, "tok").Dispatch(
		context.Background(),
		runtime.RunSpec{RunID: "terminal", Agent: "codex"},
	)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	for ev := range run.Stream() {
		if ev.Type == runtime.EventExited {
			break
		}
	}

	if err := run.Cancel(); err != nil {
		t.Fatalf("first cancel: %v", err)
	}
	if err := run.Cancel(); err != nil {
		t.Fatalf("second cancel: %v", err)
	}
	if got := cancelRequests.Load(); got != 0 {
		t.Fatalf("remote cancel requests = %d, want 0 after terminal event", got)
	}

	releaseStream()
	for range run.Stream() {
	}
	if st := run.Wait(); st.State != runtime.StateSucceeded || st.ExitCode != 0 || st.Err != "" {
		t.Fatalf("status = %+v, want succeeded/0 after late repeated cancel", st)
	}
}

func TestSignalReachesNode(t *testing.T) {
	f := fake.New()
	f.Block = true
	base := nodeServer(t, f, "tok")
	rt := New("mini", base, "tok")

	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r6", Agent: "codex", Prompt: "hi"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	<-run.Stream() // started
	<-run.Stream() // message
	if err := run.Signal("approve\n"); err != nil {
		t.Fatalf("signal: %v", err)
	}
	// Poll for the injected signal to land on the node's run.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if sigs := f.Signals("r6"); len(sigs) == 1 && sigs[0] == "approve\n" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("signal not observed on node: %v", f.Signals("r6"))
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = run.Cancel()
	for range run.Stream() {
	}
}
