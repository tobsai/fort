// Package control provides the adapters that plug Fort's deterministic
// components into the control-plane ports (ui.Dispatcher, ui.FlowRunner).
//
// This is the composition seam for "control plane, optionally with execution":
//   - EngineDispatcher / FlowExecutor wrap the real router + DAG engine (full mode).
//   - QueueDispatcher boards tasks with no execution plane at all (control-only mode).
//
// cmd/fort picks which adapters to wire; the ui module only ever sees the ports.
package control

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tobsai/fort/core/engine"
	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/ui"
)

// EngineDispatcher routes + dispatches via the deterministic engine.
type EngineDispatcher struct{ e *engine.Engine }

// NewEngineDispatcher adapts an engine to ui.Dispatcher.
func NewEngineDispatcher(e *engine.Engine) EngineDispatcher { return EngineDispatcher{e: e} }

// Submit routes the task and starts native execution.
func (d EngineDispatcher) Submit(ctx context.Context, t task.Task) (ui.RunRef, error) {
	dec := d.e.Route(t)
	runID, err := d.e.Submit(ctx, t)
	if err != nil {
		return ui.RunRef{}, err
	}
	return ui.RunRef{RunID: runID, Route: dec.Route}, nil
}

// QueueDispatcher boards a task as a "queued" run with no execution plane.
type QueueDispatcher struct {
	s     *store.Store
	newID func() string
}

// NewQueueDispatcher adapts a store to ui.Dispatcher for control-only mode.
func NewQueueDispatcher(s *store.Store) QueueDispatcher {
	return QueueDispatcher{s: s, newID: uuid.NewString}
}

// Submit records the task on the board as queued; it is never dispatched.
func (d QueueDispatcher) Submit(_ context.Context, t task.Task) (ui.RunRef, error) {
	runID := d.newID()
	title := t.Title
	if title == "" {
		title = t.ID
	}
	if err := d.s.CreateRun(store.Run{ID: runID, Title: title, Agent: "unassigned", Status: "queued"}); err != nil {
		return ui.RunRef{}, err
	}
	return ui.RunRef{RunID: runID, Queued: true}, nil
}

// FlowExecutor adapts the DAG executor to ui.FlowRunner (id-based).
type FlowExecutor struct {
	x     *graph.Executor
	flows map[string]graph.Flow
}

// NewFlowExecutor adapts a graph executor + flow set to ui.FlowRunner.
func NewFlowExecutor(x *graph.Executor, flows []graph.Flow) FlowExecutor {
	m := make(map[string]graph.Flow, len(flows))
	for _, f := range flows {
		m[f.ID] = f
	}
	return FlowExecutor{x: x, flows: m}
}

func (f FlowExecutor) lookup(id string) (graph.Flow, error) {
	fl, ok := f.flows[id]
	if !ok {
		return graph.Flow{}, fmt.Errorf("control: unknown flow %q", id)
	}
	return fl, nil
}

// StartFlow starts the named flow.
func (f FlowExecutor) StartFlow(ctx context.Context, flowID, runID, payload string) (ui.RunResult, error) {
	fl, err := f.lookup(flowID)
	if err != nil {
		return ui.RunResult{}, err
	}
	r, err := f.x.Start(ctx, fl, runID, payload)
	return ui.RunResult{State: r.State, PausedNode: r.PausedNode}, err
}

// ResumeFlow resumes the named flow.
func (f FlowExecutor) ResumeFlow(ctx context.Context, flowID, runID string) (ui.RunResult, error) {
	fl, err := f.lookup(flowID)
	if err != nil {
		return ui.RunResult{}, err
	}
	r, err := f.x.Resume(ctx, fl, runID)
	return ui.RunResult{State: r.State, PausedNode: r.PausedNode}, err
}

// Approve records a gate approval.
func (f FlowExecutor) Approve(runID, nodeID, edit string) error {
	return f.x.Approve(runID, nodeID, edit)
}

// Reject records a gate rejection.
func (f FlowExecutor) Reject(runID, nodeID string) error {
	return f.x.Reject(runID, nodeID)
}
