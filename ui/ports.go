package ui

import (
	"context"

	"github.com/tobsai/fort/core/task"
)

// The ui module talks to the rest of Fort only through these ports. This is
// what lets the control plane (board, chat, scheduler, all client surfaces) run
// WITHOUT the deterministic components: ui imports neither the router, the
// native runtime, nor the DAG engine — only core/store and core/task. Concrete
// adapters live in package control and are wired in by cmd/fort.

// RunRef identifies the run a submitted task produced.
type RunRef struct {
	RunID  string `json:"run_id"`
	Route  string `json:"route,omitempty"`  // agent, when an execution plane routed it
	Queued bool   `json:"queued,omitempty"` // true when only boarded (no execution plane)
}

// Dispatcher accepts a task. With an execution plane it routes + dispatches;
// in control-only mode it simply boards the task (Queued=true).
type Dispatcher interface {
	Submit(ctx context.Context, t task.Task) (RunRef, error)
}

// RunResult is a flow run's state after a Start/Resume.
type RunResult struct {
	State      string `json:"state"`
	PausedNode string `json:"paused_node,omitempty"`
}

// FlowRunner runs flows by id. It is nil in control-only mode (no DAG engine);
// chat "ship X" then degrades to a boarded task and gate actions return 409.
type FlowRunner interface {
	StartFlow(ctx context.Context, flowID, runID, payload string) (RunResult, error)
	Approve(runID, nodeID, edit string) error
	Reject(runID, nodeID string) error
	ResumeFlow(ctx context.Context, flowID, runID string) (RunResult, error)
}
