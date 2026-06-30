package fake

import (
	"context"
	"testing"

	"github.com/tobsai/fort/core/runtime"
)

func drain(r runtime.Run) []runtime.RunEvent {
	var evs []runtime.RunEvent
	for e := range r.Stream() {
		evs = append(evs, e)
	}
	return evs
}

func TestDispatchEchoesAndSucceeds(t *testing.T) {
	rt := New()
	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r1", Agent: "codex", Prompt: "build it"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if run.ID() != "r1" {
		t.Errorf("id = %q, want r1", run.ID())
	}
	evs := drain(run)
	if len(evs) < 2 {
		t.Fatalf("want >=2 events, got %d: %+v", len(evs), evs)
	}
	if evs[0].Type != runtime.EventStarted {
		t.Errorf("first event = %q, want started", evs[0].Type)
	}
	last := evs[len(evs)-1]
	if last.Type != runtime.EventExited || last.Code != 0 {
		t.Errorf("last event = %+v, want exited code 0", last)
	}
	// default behavior echoes the prompt as a message event.
	foundMsg := false
	for _, e := range evs {
		if e.Type == runtime.EventMessage && e.Data == "build it" {
			foundMsg = true
		}
	}
	if !foundMsg {
		t.Errorf("expected echoed message event with prompt, got %+v", evs)
	}
	st := run.Wait()
	if st.State != runtime.StateSucceeded || st.ExitCode != 0 {
		t.Errorf("status = %+v, want succeeded/0", st)
	}
}

func TestDispatchFailureExitCode(t *testing.T) {
	rt := New()
	rt.ExitCode = 2
	run, _ := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r2", Agent: "codex", Prompt: "x"})
	st := run.Wait()
	if st.State != runtime.StateFailed || st.ExitCode != 2 {
		t.Errorf("status = %+v, want failed/2", st)
	}
}

func TestSignalRecorded(t *testing.T) {
	rt := New()
	rt.Block = true // hold open until canceled/signaled so we can inject
	run, _ := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r3", Prompt: "interactive"})
	if err := run.Signal("approve"); err != nil {
		t.Fatalf("signal: %v", err)
	}
	run.Cancel()
	run.Wait()
	if got := rt.Signals("r3"); len(got) != 1 || got[0] != "approve" {
		t.Errorf("signals = %v, want [approve]", got)
	}
}

func TestCancel(t *testing.T) {
	rt := New()
	rt.Block = true
	run, _ := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r4", Prompt: "long"})
	if err := run.Cancel(); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	st := run.Wait()
	if st.State != runtime.StateCanceled {
		t.Errorf("status = %+v, want canceled", st)
	}
}

func TestImplementsRuntimeInterface(t *testing.T) {
	var _ runtime.Runtime = New()
}
