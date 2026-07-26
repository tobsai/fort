package watchdog

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tobsai/fort/core/runtime"
)

type stubRuntime struct {
	run runtime.Run
}

func (s stubRuntime) Name() string { return "stub" }

func (s stubRuntime) Dispatch(context.Context, runtime.RunSpec) (runtime.Run, error) {
	return s.run, nil
}

type stubRun struct {
	events chan runtime.RunEvent
	done   chan struct{}

	mu          sync.Mutex
	status      runtime.Status
	cancelCalls int
	once        sync.Once
}

func newStubRun() *stubRun {
	return &stubRun{
		events: make(chan runtime.RunEvent, 8),
		done:   make(chan struct{}),
		status: runtime.Status{State: runtime.StateRunning},
	}
}

func (r *stubRun) ID() string                      { return "run-1" }
func (r *stubRun) Stream() <-chan runtime.RunEvent { return r.events }
func (r *stubRun) Signal(string) error             { return nil }
func (r *stubRun) Status() runtime.Status          { r.mu.Lock(); defer r.mu.Unlock(); return r.status }
func (r *stubRun) Wait() runtime.Status            { <-r.done; return r.Status() }
func (r *stubRun) finish(status runtime.Status) {
	r.once.Do(func() { r.mu.Lock(); r.status = status; r.mu.Unlock(); close(r.events); close(r.done) })
}
func (r *stubRun) cancellationCount() int { r.mu.Lock(); defer r.mu.Unlock(); return r.cancelCalls }
func (r *stubRun) Cancel() error {
	r.mu.Lock()
	r.cancelCalls++
	r.mu.Unlock()
	r.finish(runtime.Status{State: runtime.StateCanceled, ExitCode: -1, Err: "canceled"})
	return nil
}

func TestSilentRunIsCanceledAndFailed(t *testing.T) {
	inner := newStubRun()
	rt := New(stubRuntime{run: inner}, 25*time.Millisecond)

	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "run-1", Agent: "hermes"})
	if err != nil {
		t.Fatal(err)
	}
	var events []runtime.RunEvent
	for event := range run.Stream() {
		events = append(events, event)
	}

	status := run.Wait()
	if status.State != runtime.StateFailed || !strings.Contains(status.Err, "no events for") {
		t.Fatalf("status = %+v, want failed silence timeout", status)
	}
	if inner.cancellationCount() != 1 {
		t.Fatalf("cancel calls = %d, want 1", inner.cancellationCount())
	}
	if len(events) != 1 || events[0].Type != runtime.EventError ||
		!strings.Contains(events[0].Data, "no events for") {
		t.Fatalf("events = %+v, want one timeout error", events)
	}
}

func TestActivityResetsSilenceDeadline(t *testing.T) {
	inner := newStubRun()
	rt := New(stubRuntime{run: inner}, 40*time.Millisecond)

	go func() {
		for range 3 {
			time.Sleep(20 * time.Millisecond)
			inner.events <- runtime.RunEvent{RunID: inner.ID(), Type: runtime.EventMessage, Data: "progress"}
		}
		inner.finish(runtime.Status{State: runtime.StateSucceeded})
	}()

	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "run-1", Agent: "hermes"})
	if err != nil {
		t.Fatal(err)
	}
	var messages int
	for event := range run.Stream() {
		if event.Type == runtime.EventMessage {
			messages++
		}
	}

	if status := run.Wait(); status.State != runtime.StateSucceeded {
		t.Fatalf("status = %+v, want succeeded", status)
	}
	if messages != 3 {
		t.Fatalf("messages = %d, want 3", messages)
	}
	if inner.cancellationCount() != 0 {
		t.Fatalf("cancel calls = %d, want 0", inner.cancellationCount())
	}
}
