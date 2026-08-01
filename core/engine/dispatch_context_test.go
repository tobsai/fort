package engine

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
)

// ctxProbeRuntime captures the context handed to Dispatch and returns a run
// that stays open until finished, so a test can observe whether that context
// outlives the submit call.
type ctxProbeRuntime struct {
	started chan struct{}
	gotCtx  context.Context
	run     *ctxProbeRun
}

func (r *ctxProbeRuntime) Name() string { return "ctxprobe" }

func (r *ctxProbeRuntime) Dispatch(ctx context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	r.gotCtx = ctx
	run := &ctxProbeRun{events: make(chan runtime.RunEvent), cancelCalled: make(chan struct{})}
	r.run = run
	close(r.started)
	// Model a real runtime: the run terminates when its own context is canceled.
	go func() {
		<-ctx.Done()
		run.finish(runtime.StateCanceled)
	}()
	return run, nil
}

type ctxProbeRun struct {
	events       chan runtime.RunEvent
	once         sync.Once
	cancelOnce   sync.Once
	cancelCalled chan struct{}
	state        runtime.State
}

func (r *ctxProbeRun) ID() string                      { return "probe" }
func (r *ctxProbeRun) Stream() <-chan runtime.RunEvent { return r.events }
func (r *ctxProbeRun) Signal(string) error             { return nil }
func (r *ctxProbeRun) Cancel() error {
	r.cancelOnce.Do(func() { close(r.cancelCalled) })
	r.finish(runtime.StateCanceled)
	return nil
}
func (r *ctxProbeRun) Status() runtime.Status { return runtime.Status{State: r.state} }
func (r *ctxProbeRun) Wait() runtime.Status   { return runtime.Status{State: r.state} }
func (r *ctxProbeRun) finish(s runtime.State) {
	r.once.Do(func() { r.state = s; close(r.events) })
}

type blockedDispatchRuntime struct {
	started chan struct{}
	release chan struct{}
}

func (r *blockedDispatchRuntime) Name() string { return "blocked-dispatch" }

func (r *blockedDispatchRuntime) Dispatch(ctx context.Context, _ runtime.RunSpec) (runtime.Run, error) {
	close(r.started)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.release:
		return nil, context.DeadlineExceeded
	}
}

// TestRunContextDetachedFromCaller pins the spec-024/board fix: a submitted run
// must execute on its own context, independent of the submit call's context. A
// board chat submit is fire-and-forget — its HTTP request context ends the
// instant the handler returns — so binding the run to it tore the run down
// (observed on a mesh dispatch as "remote stream error: context canceled").
func TestRunContextDetachedFromCaller(t *testing.T) {
	rs, err := rules.Parse([]byte(ruleset))
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	rt := &ctxProbeRuntime{started: make(chan struct{})}
	e := New(router.New(rs), rt, st, t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	runID, err := e.Submit(ctx, task.Task{ID: "t1", Title: "hello"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	<-rt.started
	// Always let the consume goroutine unwind, even if the test fails early.
	t.Cleanup(func() { rt.run.finish(runtime.StateSucceeded) })

	// The submit call's context ends (fire-and-forget board request returned).
	cancel()

	// The run's execution context must NOT be canceled with it. In the buggy
	// code gotCtx == the caller's ctx, so its Done() is ready immediately here.
	select {
	case <-rt.gotCtx.Done():
		t.Fatalf("run dispatched on the caller's context; canceled with the submit call: %v", rt.gotCtx.Err())
	case <-time.After(200 * time.Millisecond):
	}

	// Completion still flows through: finish the run, it lands as succeeded.
	rt.run.finish(runtime.StateSucceeded)
	e.wait(runID)
	if got, _ := st.GetRun(runID); got.Status != "succeeded" {
		t.Fatalf("run status = %q, want succeeded", got.Status)
	}
}

// TestSubmitCancelStopsBlockedDispatch covers the window before Dispatch has
// returned a runtime.Run. A provider contract probe or remote response-header
// wait can block there, so the caller's first Ctrl-C must cancel that dispatch
// even though successful asynchronous runs detach afterward.
func TestSubmitCancelStopsBlockedDispatch(t *testing.T) {
	rs, err := rules.Parse([]byte(ruleset))
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	rt := &blockedDispatchRuntime{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	e := New(router.New(rs), rt, st, t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := e.Submit(ctx, task.Task{ID: "blocked", Title: "blocked dispatch"})
		result <- err
	}()
	<-rt.started
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("submit error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		close(rt.release)
		t.Fatal("caller cancellation did not stop blocked Dispatch")
	}
}

// TestAcceptRefPersistsBeforeDispatchAndReturnsImmediately pins the HTTP
// acceptance seam: a provider preflight may block, but the caller must receive
// the durable run identity without waiting for Dispatch to return.
func TestAcceptRefPersistsBeforeDispatchAndReturnsImmediately(t *testing.T) {
	rs, err := rules.Parse([]byte(ruleset))
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	rt := &blockedDispatchRuntime{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	e := New(router.New(rs), rt, st, t.TempDir())

	type accepted struct {
		runID   string
		machine string
		err     error
	}
	result := make(chan accepted, 1)
	go func() {
		runID, machine, err := e.AcceptRef(context.Background(), task.Task{ID: "accepted", Title: "accepted"})
		result <- accepted{runID: runID, machine: machine, err: err}
	}()

	var got accepted
	select {
	case got = <-result:
	case <-time.After(250 * time.Millisecond):
		close(rt.release)
		t.Fatal("AcceptRef waited for blocked Dispatch")
	}
	if got.err != nil || got.runID == "" || got.machine != "" {
		t.Fatalf("accepted = %+v", got)
	}
	run, err := st.GetRun(got.runID)
	if err != nil || run.Status != "running" {
		t.Fatalf("durable run before dispatch = %+v, %v", run, err)
	}
	select {
	case <-rt.started:
	case <-time.After(time.Second):
		t.Fatal("accepted dispatch never started")
	}
	close(rt.release)
}

// TestSubmitAndWaitCancelStopsRun keeps the CLI semantics: a blocking
// SubmitAndWait whose caller cancels should still cancel the run (Ctrl-C on
// `fort task add`), not leave it detached and running.
func TestSubmitAndWaitCancelStopsRun(t *testing.T) {
	rs, err := rules.Parse([]byte(ruleset))
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	rt := &ctxProbeRuntime{started: make(chan struct{})}
	e := New(router.New(rs), rt, st, t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-rt.started
		cancel() // caller aborts while the run is still open
	}()

	run, err := e.SubmitAndWait(ctx, task.Task{ID: "t2", Title: "blocks"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if run.Status != "canceled" {
		t.Fatalf("status = %q, want canceled (caller cancel must reach the run)", run.Status)
	}
	select {
	case <-rt.run.cancelCalled:
	default:
		t.Fatal("caller cancellation did not invoke runtime.Run.Cancel")
	}
}
