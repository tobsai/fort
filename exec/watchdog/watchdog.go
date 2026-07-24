// Package watchdog bounds silent runtime invocations. It decorates any
// runtime.Runtime, so the same policy applies to local and remote providers.
package watchdog

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tobsai/fort/core/runtime"
)

// Runtime cancels an invocation that emits no events for idleTimeout.
type Runtime struct {
	under       runtime.Runtime
	idleTimeout time.Duration
}

// New wraps under with a per-invocation silence deadline. A non-positive
// timeout disables the deadline while retaining the runtime wrapper.
func New(under runtime.Runtime, idleTimeout time.Duration) *Runtime {
	return &Runtime{under: under, idleTimeout: idleTimeout}
}

// Name implements runtime.Runtime.
func (r *Runtime) Name() string { return "watchdog(" + r.under.Name() + ")" }

// Dispatch implements runtime.Runtime.
func (r *Runtime) Dispatch(ctx context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	inner, err := r.under.Dispatch(ctx, spec)
	if err != nil {
		return nil, err
	}
	run := &watchedRun{
		inner:       inner,
		idleTimeout: r.idleTimeout,
		events:      make(chan runtime.RunEvent, 64),
		done:        make(chan struct{}),
		status:      runtime.Status{State: runtime.StateRunning},
	}
	go run.watch()
	return run, nil
}

type watchedRun struct {
	inner       runtime.Run
	idleTimeout time.Duration
	events      chan runtime.RunEvent
	done        chan struct{}

	mu     sync.Mutex
	status runtime.Status
}

func (r *watchedRun) ID() string                      { return r.inner.ID() }
func (r *watchedRun) Stream() <-chan runtime.RunEvent { return r.events }
func (r *watchedRun) Signal(input string) error       { return r.inner.Signal(input) }
func (r *watchedRun) Cancel() error                   { return r.inner.Cancel() }

func (r *watchedRun) Status() runtime.Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func (r *watchedRun) Wait() runtime.Status {
	<-r.done
	return r.Status()
}

func (r *watchedRun) watch() {
	defer close(r.events)
	defer close(r.done)

	stream := r.inner.Stream()
	var (
		timer     *time.Timer
		timeoutCh <-chan time.Time
		timedOut  bool
		timeout   string
	)
	if r.idleTimeout > 0 {
		timer = time.NewTimer(r.idleTimeout)
		timeoutCh = timer.C
		defer timer.Stop()
	}

	for {
		select {
		case event, ok := <-stream:
			if !ok {
				status := r.inner.Wait()
				if timedOut {
					status = runtime.Status{
						State:    runtime.StateFailed,
						ExitCode: -1,
						Err:      timeout,
					}
				}
				r.setStatus(status)
				return
			}
			r.events <- event
			if timer != nil && !timedOut {
				resetTimer(timer, r.idleTimeout)
			}

		case <-timeoutCh:
			timedOut = true
			timeoutCh = nil
			timeout = fmt.Sprintf("runtime: no events for %s", r.idleTimeout)
			r.events <- runtime.RunEvent{
				RunID: r.ID(),
				Type:  runtime.EventError,
				Time:  time.Now(),
				Data:  timeout,
				Code:  -1,
			}
			_ = r.inner.Cancel()
		}
	}
}

func (r *watchedRun) setStatus(status runtime.Status) {
	r.mu.Lock()
	r.status = status
	r.mu.Unlock()
}

func resetTimer(timer *time.Timer, timeout time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(timeout)
}
