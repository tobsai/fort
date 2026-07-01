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

// Placer chooses the machine that will run an agent (spec 022). Like the router
// it is a pure, deterministic function of its inputs — zero model calls — so the
// full dispatch path stays model-free. core/machines.Registry implements it;
// cmd/fort injects it in multi-machine mode.
type Placer interface {
	Place(agent, pin string) (machine string, err error)
}

// Engine routes and dispatches tasks.
type Engine struct {
	router   *router.Router
	rt       runtime.Runtime
	store    *store.Store
	workRoot string
	newID    func() string
	placer   Placer // nil => single-machine (no placement)

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

// UsePlacer enables deterministic machine placement (spec 022). With no placer
// the engine dispatches to the local runtime exactly as before.
func (e *Engine) UsePlacer(p Placer) { e.placer = p }

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

// Submit routes and dispatches a task, returning the run id immediately. It
// satisfies inbox.Submitter; callers that need the resolved machine use
// SubmitRef.
func (e *Engine) Submit(ctx context.Context, t task.Task) (string, error) {
	runID, _, err := e.SubmitRef(ctx, t)
	return runID, err
}

// SubmitRef is Submit that also returns the resolved machine (spec 022). It
// persists the route decision and a run row, starts native execution, and
// streams events into the store on a goroutine.
func (e *Engine) SubmitRef(ctx context.Context, t task.Task) (string, string, error) {
	dec := e.router.Route(t)
	_ = e.store.SaveRouteDecision(store.RouteDecision{
		ID:          e.newID(),
		TaskID:      t.ID,
		Route:       dec.Route,
		MatchedRule: dec.MatchedRule,
		IsDefault:   dec.Default,
		Reason:      dec.Reason,
	})

	title := t.Title
	if title == "" {
		title = t.ID
	}

	// Deterministic machine placement (spec 022). A nil placer keeps the
	// original single-machine behavior (machine stays "").
	machine := ""
	if e.placer != nil {
		m, perr := e.placer.Place(dec.Route, t.Machine)
		if perr != nil {
			// Board the placement failure so it is visible, then surface it.
			failID := e.newID()
			_ = e.store.CreateRun(store.Run{
				ID: failID, Title: title, Agent: dec.Route, Status: "failed",
				MatchedRule: dec.MatchedRule, Error: perr.Error(),
			})
			return failID, "", perr
		}
		machine = m
	}

	runID := e.newID()
	if err := e.store.CreateRun(store.Run{
		ID:          runID,
		Title:       title,
		Agent:       dec.Route,
		Status:      "running",
		MatchedRule: dec.MatchedRule,
		Machine:     machine,
	}); err != nil {
		return "", "", err
	}

	spec := runtime.RunSpec{
		RunID:   runID,
		Agent:   dec.Route,
		Prompt:  prompt(t),
		Workdir: filepath.Join(e.workRoot, runID),
		Machine: machine,
	}
	run, err := e.rt.Dispatch(ctx, spec)
	if err != nil {
		_ = e.store.UpdateRunStatus(runID, "failed", -1, err.Error())
		return runID, machine, err
	}

	done := make(chan struct{})
	go e.consume(run, runID, done)
	// stash the done channel so SubmitAndWait can block on it
	e.track(runID, done)
	return runID, machine, nil
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
			RunID:     runID,
			Type:      string(ev.Type),
			Data:      ev.Data,
			Code:      ev.Code,
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
