// Package fake is an in-memory runtime.Runtime for fast, deterministic unit
// tests (backlog AO-014: "Interface is mockable — a FakeRuntime powers fast
// unit tests"). It spawns no processes.
package fake

import (
	"context"
	"sync"
	"time"

	"github.com/tobsai/fort/core/runtime"
)

// Runtime is a scriptable fake executor.
type Runtime struct {
	// ExitCode is the exit code reported for non-canceled runs (default 0).
	ExitCode int
	// Block, when true, holds the run open (after the started event) until it
	// is canceled or signaled — useful for exercising Signal/Cancel.
	Block bool
	// Stdout, when non-empty, is emitted as raw stdout lines (EventStdout) before
	// the terminal exited event — lets a test feed a canned provider stream (e.g.
	// claude's stream-json result line). Empty leaves behavior unchanged.
	Stdout []string

	mu      sync.Mutex
	signals map[string][]string // runID -> injected inputs
	specs   []runtime.RunSpec
}

// New returns a fake runtime.
func New() *Runtime {
	return &Runtime{signals: map[string][]string{}}
}

// Name implements runtime.Runtime.
func (r *Runtime) Name() string { return "fake" }

// Dispatch implements runtime.Runtime.
func (r *Runtime) Dispatch(ctx context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	r.mu.Lock()
	r.specs = append(r.specs, spec)
	// Snapshot the config fields under the lock so execute (a goroutine) reads
	// its own frozen copy, never racing a concurrent write to r.Block/Stdout/
	// ExitCode.
	run := &fakeRun{
		parent:   r,
		spec:     spec,
		events:   make(chan runtime.RunEvent, 16),
		done:     make(chan struct{}),
		sigCh:    make(chan struct{}, 1),
		status:   runtime.Status{State: runtime.StateRunning},
		block:    r.Block,
		stdout:   append([]string(nil), r.Stdout...),
		exitCode: r.ExitCode,
	}
	r.mu.Unlock()

	go run.execute(ctx)
	return run, nil
}

// Signals returns the inputs injected into a run via Signal.
func (r *Runtime) Signals(runID string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.signals[runID]))
	copy(out, r.signals[runID])
	return out
}

// Dispatched returns the specs dispatched so far.
func (r *Runtime) Dispatched() []runtime.RunSpec {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]runtime.RunSpec, len(r.specs))
	copy(out, r.specs)
	return out
}

type fakeRun struct {
	parent *Runtime
	spec   runtime.RunSpec
	events chan runtime.RunEvent

	// config snapshot taken at Dispatch (under parent.mu); execute reads these,
	// not parent.Block/Stdout/ExitCode, so it never races a concurrent write.
	block    bool
	stdout   []string
	exitCode int

	mu       sync.Mutex
	status   runtime.Status
	canceled bool

	done  chan struct{}
	sigCh chan struct{}
	once  sync.Once
}

func (f *fakeRun) ID() string                    { return f.spec.RunID }
func (f *fakeRun) Stream() <-chan runtime.RunEvent { return f.events }

func (f *fakeRun) emit(t runtime.EventType, data string, code int) {
	f.events <- runtime.RunEvent{RunID: f.spec.RunID, Type: t, Time: time.Now(), Data: data, Code: code}
}

func (f *fakeRun) execute(ctx context.Context) {
	defer close(f.events)
	defer close(f.done)

	f.emit(runtime.EventStarted, f.spec.Agent, 0)
	f.emit(runtime.EventMessage, f.spec.Prompt, 0)

	if f.block {
		select {
		case <-ctx.Done():
			f.finish(runtime.StateCanceled, 0, ctx.Err().Error())
			return
		case <-f.sigCh:
			f.finish(runtime.StateCanceled, 0, "canceled")
			return
		}
	}

	select {
	case <-ctx.Done():
		f.finish(runtime.StateCanceled, 0, ctx.Err().Error())
		return
	default:
	}

	for _, line := range f.stdout {
		f.emit(runtime.EventStdout, line, 0)
	}

	code := f.exitCode
	if code == 0 {
		f.emit(runtime.EventExited, "", 0)
		f.finish(runtime.StateSucceeded, 0, "")
	} else {
		f.emit(runtime.EventExited, "", code)
		f.finish(runtime.StateFailed, code, "")
	}
}

func (f *fakeRun) finish(state runtime.State, code int, errMsg string) {
	f.mu.Lock()
	if f.canceled && state != runtime.StateCanceled {
		state, code = runtime.StateCanceled, 0
	}
	f.status = runtime.Status{State: state, ExitCode: code, Err: errMsg}
	f.mu.Unlock()
}

func (f *fakeRun) Signal(input string) error {
	f.parent.mu.Lock()
	f.parent.signals[f.spec.RunID] = append(f.parent.signals[f.spec.RunID], input)
	f.parent.mu.Unlock()
	return nil
}

func (f *fakeRun) Cancel() error {
	f.mu.Lock()
	f.canceled = true
	f.mu.Unlock()
	f.once.Do(func() { close(f.sigCh) })
	return nil
}

func (f *fakeRun) Status() runtime.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status
}

func (f *fakeRun) Wait() runtime.Status {
	<-f.done
	return f.Status()
}
