// Package engine wires the deterministic router to native execution and the
// state store (backlog AO-015): a submitted task auto-routes, is persisted with
// its matched rule, and runs natively with zero manual assignment. Run events
// stream into the append-only event log.
package engine

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
)

// Engine routes and dispatches tasks.
type Engine struct {
	router   *router.Router
	rt       runtime.Runtime
	store    *store.Store
	workRoot string
	newID    func() string

	mu      sync.Mutex
	waiters map[string]chan struct{} // runID -> done
}

// New builds an engine.
func New(r *router.Router, rt runtime.Runtime, st *store.Store, workRoot string) *Engine {
	return &Engine{
		router:   r,
		rt:       rt,
		store:    st,
		workRoot: workRoot,
		newID:    uuid.NewString,
		waiters:  map[string]chan struct{}{},
	}
}

func (e *Engine) track(runID string, done chan struct{}) {
	e.mu.Lock()
	e.waiters[runID] = done
	e.mu.Unlock()
}

// wait blocks until the run's consume goroutine has finished persisting.
func (e *Engine) wait(runID string) {
	e.mu.Lock()
	done := e.waiters[runID]
	e.mu.Unlock()
	if done != nil {
		<-done
	}
}

// Route returns the routing decision for a task without dispatching it
// (powers `fort route --dry-run`).
func (e *Engine) Route(t task.Task) router.RouteDecision { return e.router.Route(t) }

// Submit routes and dispatches a task. It persists the route decision and a run
// row, starts native execution, and streams events into the store on a
// goroutine. It returns the run id immediately.
func (e *Engine) Submit(ctx context.Context, t task.Task) (string, error) {
	dec := e.router.Route(t)
	_ = e.store.SaveRouteDecision(store.RouteDecision{
		ID:          e.newID(),
		TaskID:      t.ID,
		Route:       dec.Route,
		MatchedRule: dec.MatchedRule,
		IsDefault:   dec.Default,
		Reason:      dec.Reason,
	})

	runID := e.newID()
	title := t.Title
	if title == "" {
		title = t.ID
	}
	if err := e.store.CreateRun(store.Run{
		ID:          runID,
		Title:       title,
		Agent:       dec.Route,
		Status:      "running",
		MatchedRule: dec.MatchedRule,
	}); err != nil {
		return "", err
	}

	spec := runtime.RunSpec{
		RunID:   runID,
		Agent:   dec.Route,
		Prompt:  prompt(t),
		Workdir: filepath.Join(e.workRoot, runID),
	}
	run, err := e.rt.Dispatch(ctx, spec)
	if err != nil {
		_ = e.store.UpdateRunStatus(runID, "failed", -1, err.Error())
		return runID, err
	}

	done := make(chan struct{})
	go e.consume(run, runID, done)
	// stash the done channel so SubmitAndWait can block on it
	e.track(runID, done)
	return runID, nil
}

// SubmitAndWait submits a task and blocks until its run terminates, returning
// the final run row.
func (e *Engine) SubmitAndWait(ctx context.Context, t task.Task) (store.Run, error) {
	runID, err := e.Submit(ctx, t)
	if err != nil {
		return store.Run{}, err
	}
	e.wait(runID)
	return e.store.GetRun(runID)
}

func (e *Engine) consume(run runtime.Run, runID string, done chan struct{}) {
	defer close(done)
	for ev := range run.Stream() {
		_, _ = e.store.AppendEvent(store.Event{
			RunID: runID,
			Type:  string(ev.Type),
			Data:  ev.Data,
			Code:  ev.Code,
			CreatedAt: ev.Time,
		})
	}
	st := run.Wait()
	_ = e.store.UpdateRunStatus(runID, statusString(st.State), st.ExitCode, st.Err)
}

func statusString(s runtime.State) string {
	switch s {
	case runtime.StateSucceeded:
		return "succeeded"
	case runtime.StateFailed:
		return "failed"
	case runtime.StateCanceled:
		return "canceled"
	default:
		return "running"
	}
}

func prompt(t task.Task) string {
	if t.Body != "" {
		return t.Body
	}
	return t.Title
}
