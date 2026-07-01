package remote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/exec/fake"
	"github.com/tobsai/fort/exec/node"
)

// nodeServer wires a fake runtime behind the node exec endpoint on an httptest
// server and returns the base URL plus the underlying fake (for assertions).
func nodeServer(t *testing.T, f *fake.Runtime, token string) string {
	t.Helper()
	mux := http.NewServeMux()
	node.New(f, token).Register(mux)
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
