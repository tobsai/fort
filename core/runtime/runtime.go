// Package runtime defines the execution seam (backlog AO-014, spec §6.2).
//
// core depends only on these interfaces; the concrete executors live in the
// exec module (exec/native spawns real CLIs, exec/fake powers unit tests) and
// are wired in by cmd/fort. This is the only path from core to execution.
package runtime

import (
	"context"
	"time"
)

// RunSpec describes one agent invocation.
type RunSpec struct {
	RunID   string   // caller-assigned id, echoed back on the Run
	Agent   string   // provider key: claude | codex | openclaw | hermes
	Model   string   // optional provider-specific model override; empty uses provider default
	Prompt  string   // the task body handed to the CLI
	Workdir string   // scoped working directory
	Env     []string // additional KEY=VALUE pairs
	// Machine is the resolved target host (spec 022). Empty means "here": the
	// local runtime handles it. A non-local name routes the spec to that
	// machine's runtime (exec/cluster). Providers ignore this field.
	Machine string
}

// EventType enumerates normalized run events streamed from any runtime.
type EventType string

const (
	EventStarted EventType = "started" // process spawned
	EventStdout  EventType = "stdout"  // raw stdout line
	EventStderr  EventType = "stderr"  // raw stderr line
	EventMessage EventType = "message" // normalized assistant/output message
	EventExited  EventType = "exited"  // process exited (Code set)
	EventError   EventType = "error"   // runtime-level error (Data set)
	// EventTool: the agent invoked a tool (spec 030). Data is a compact JSON
	// object {"name":..., "summary":...}.
	EventTool EventType = "tool"
	// EventSubagent: the agent spawned a sub-task (spec 030; claude's Task
	// tool). Data is {"description":..., "agent":...}.
	EventSubagent EventType = "subagent"
)

// RunEvent is one normalized event in a run's stream.
type RunEvent struct {
	RunID string    `json:"run_id"`
	Type  EventType `json:"type"`
	Time  time.Time `json:"time"`
	Data  string    `json:"data,omitempty"`
	Code  int       `json:"code,omitempty"` // exit code for EventExited
}

// State is a run's lifecycle state.
type State string

const (
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
	StateCanceled  State = "canceled"
)

// Status is a run's terminal-or-current status.
type Status struct {
	State    State  `json:"state"`
	ExitCode int    `json:"exit_code"`
	Err      string `json:"err,omitempty"`
}

// Terminal reports whether the run has finished.
func (s Status) Terminal() bool {
	return s.State == StateSucceeded || s.State == StateFailed || s.State == StateCanceled
}

// Run is a handle to a dispatched invocation: stream (events), status, and
// signal (stdin injection for human-in-the-loop) + cancel.
type Run interface {
	ID() string
	// Stream returns the event channel; it is closed when the run terminates.
	Stream() <-chan RunEvent
	// Signal injects input into a running interactive task (HITL).
	Signal(input string) error
	// Cancel terminates the run.
	Cancel() error
	// Status returns the current status.
	Status() Status
	// Wait blocks until the run terminates and returns the terminal status.
	Wait() Status
}

// Runtime spawns agent invocations. Implementations must be safe for
// concurrent Dispatch calls.
type Runtime interface {
	// Dispatch launches spec and returns a Run handle. The returned Run begins
	// streaming immediately; the caller drains Stream() and/or calls Wait().
	Dispatch(ctx context.Context, spec RunSpec) (Run, error)
	// Name identifies the runtime implementation (for diagnostics).
	Name() string
}
