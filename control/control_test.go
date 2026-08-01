package control

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobsai/fort/core/engine"
	"github.com/tobsai/fort/core/flow"
	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/exec/fake"
)

type blockedRuntime struct {
	started chan struct{}
	release chan struct{}
}

func (r *blockedRuntime) Name() string { return "blocked" }

func (r *blockedRuntime) Dispatch(ctx context.Context, _ runtime.RunSpec) (runtime.Run, error) {
	close(r.started)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.release:
		return nil, errors.New("blocked runtime released")
	}
}

const rs = `
version: 1
defaults: { route: claude }
rules:
  - id: dev
    when: { label: [feature] }
    route: codex
`

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// QueueDispatcher (control-only) boards a task without any execution plane.
func TestQueueDispatcherBoardsTask(t *testing.T) {
	st := newStore(t)
	d := NewQueueDispatcher(st)
	ref, err := d.Submit(context.Background(), task.Task{ID: "t1", Title: "do a thing"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !ref.Queued || ref.RunID == "" {
		t.Fatalf("ref = %+v, want queued with a run id", ref)
	}
	run, err := st.GetRun(ref.RunID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if run.Status != "queued" || run.Title != "do a thing" {
		t.Errorf("run = %+v, want queued board card", run)
	}
}

// EngineDispatcher (execution plane present) routes + dispatches.
func TestEngineDispatcherRoutesAndRuns(t *testing.T) {
	st := newStore(t)
	parsed, _ := rules.Parse([]byte(rs))
	eng := engine.New(router.New(parsed), fake.New(), st, t.TempDir())
	d := NewEngineDispatcher(eng)
	ref, err := d.Submit(context.Background(), task.Task{ID: "t2", Title: "add feature", Labels: []string{"feature"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if ref.Queued {
		t.Errorf("ref should not be queued when routed: %+v", ref)
	}
	if ref.Route != "codex" {
		t.Errorf("route = %q, want codex", ref.Route)
	}
}

func TestEngineDispatcherAcceptsBeforeBlockedDispatchCompletes(t *testing.T) {
	st := newStore(t)
	parsed, _ := rules.Parse([]byte(rs))
	rt := &blockedRuntime{started: make(chan struct{}), release: make(chan struct{})}
	eng := engine.New(router.New(parsed), rt, st, t.TempDir())
	d := NewEngineDispatcher(eng)

	ref, err := d.Accept(context.Background(), task.Task{ID: "accepted", Title: "accepted"})
	if err != nil || ref.RunID == "" {
		t.Fatalf("accept = %+v, %v", ref, err)
	}
	if run, err := st.GetRun(ref.RunID); err != nil || run.Status != "running" {
		t.Fatalf("accepted run = %+v, %v", run, err)
	}
	select {
	case <-rt.started:
	case <-time.After(time.Second):
		t.Fatal("accepted dispatch did not start")
	}
	close(rt.release)
}

// FlowExecutor adapts graph.Executor to ui.FlowRunner (id-based).
func TestFlowExecutorRunsAndResumesByID(t *testing.T) {
	st := newStore(t)
	flows, err := flow.LoadDir("../flows")
	if err != nil {
		t.Fatalf("flows: %v", err)
	}
	fx := NewFlowExecutor(graph.NewExecutor(fake.New(), st), flows)

	res, err := fx.StartFlow(context.Background(), "ship-feature", "run1", "add search")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if res.State != "paused" || res.PausedNode != "plan_gate" {
		t.Fatalf("res = %+v, want paused at plan_gate", res)
	}
	if err := fx.Approve("run1", "plan_gate", ""); err != nil {
		t.Fatalf("approve: %v", err)
	}
	res, err = fx.ResumeFlow(context.Background(), "ship-feature", "run1")
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if res.State != "paused" || res.PausedNode != "merge_gate" {
		t.Errorf("res = %+v, want paused at merge_gate", res)
	}
}

func TestFlowExecutorUnknownFlow(t *testing.T) {
	fx := NewFlowExecutor(graph.NewExecutor(fake.New(), newStore(t)), nil)
	if _, err := fx.StartFlow(context.Background(), "nope", "r", ""); err == nil {
		t.Fatal("expected error for unknown flow id")
	}
}
