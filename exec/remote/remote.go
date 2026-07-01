// Package remote dispatches runs to another Fort over HTTP (spec 022). It
// implements core/runtime.Runtime: Dispatch POSTs a RunSpec to the target node's
// /api/exec and streams the node's RunEvents back as a runtime.Run, so remote
// execution is indistinguishable from local to core/engine. It is the client
// side of the exec/node protocol.
package remote

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tobsai/fort/core/runtime"
)

// Runtime is a runtime.Runtime backed by a remote Fort node.
type Runtime struct {
	name   string // target machine name (diagnostics + Run routing)
	base   string // base URL, e.g. http://macbook-pro.local:4087
	token  string
	client *http.Client
}

// New builds a remote runtime targeting machineName at baseURL, authenticating
// with token.
func New(machineName, baseURL, token string) *Runtime {
	return &Runtime{
		name:   machineName,
		base:   strings.TrimRight(baseURL, "/"),
		token:  token,
		client: &http.Client{}, // no overall timeout: runs stream for their lifetime
	}
}

// Name implements runtime.Runtime.
func (r *Runtime) Name() string { return "remote:" + r.name }

// Dispatch posts spec to the node and returns a streaming Run.
func (r *Runtime) Dispatch(ctx context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	body, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	rctx, cancel := context.WithCancel(ctx)
	req, err := http.NewRequestWithContext(rctx, http.MethodPost, r.base+"/api/exec", bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.token)

	resp, err := r.client.Do(req)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("remote %s: %w", r.name, err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("remote %s: exec returned %s: %s", r.name, resp.Status, strings.TrimSpace(string(b)))
	}

	run := &remoteRun{
		id:     spec.RunID,
		rt:     r,
		body:   resp.Body,
		cancel: cancel,
		events: make(chan runtime.RunEvent, 64),
		done:   make(chan struct{}),
		status: runtime.Status{State: runtime.StateRunning},
	}
	go run.pump()
	return run, nil
}

type remoteRun struct {
	id     string
	rt     *Runtime
	body   io.ReadCloser
	cancel context.CancelFunc

	events chan runtime.RunEvent
	done   chan struct{}

	mu       sync.Mutex
	status   runtime.Status
	canceled bool
}

func (rr *remoteRun) ID() string                      { return rr.id }
func (rr *remoteRun) Stream() <-chan runtime.RunEvent { return rr.events }

// pump decodes the node's ndjson event stream, forwards events, and derives the
// terminal status from the last exited/error frame.
func (rr *remoteRun) pump() {
	defer close(rr.events)
	defer close(rr.done)
	defer rr.body.Close()
	defer rr.cancel()

	sc := bufio.NewScanner(rr.body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	sawTerminal := false
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var ev runtime.RunEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue // skip a malformed frame rather than kill the stream
		}
		switch ev.Type {
		case runtime.EventExited:
			sawTerminal = true
			if ev.Code == 0 {
				rr.setStatus(runtime.StateSucceeded, 0, "")
			} else {
				rr.setStatus(runtime.StateFailed, ev.Code, "")
			}
		case runtime.EventError:
			sawTerminal = true
			rr.setStatus(runtime.StateFailed, -1, ev.Data)
		}
		rr.events <- ev
	}

	rr.mu.Lock()
	canceled := rr.canceled
	rr.mu.Unlock()
	switch {
	case canceled:
		rr.setStatus(runtime.StateCanceled, -1, "canceled")
	case !sawTerminal:
		// The connection dropped before the run reported a terminal event.
		msg := "remote stream ended before completion"
		if err := sc.Err(); err != nil {
			msg = fmt.Sprintf("remote stream error: %v", err)
		}
		rr.setStatus(runtime.StateFailed, -1, msg)
		rr.events <- runtime.RunEvent{RunID: rr.id, Type: runtime.EventError, Time: time.Now(), Data: msg}
	}
}

func (rr *remoteRun) setStatus(state runtime.State, code int, errMsg string) {
	rr.mu.Lock()
	// A prior Cancel wins over a late natural terminal state.
	if rr.canceled && state != runtime.StateCanceled {
		state, code, errMsg = runtime.StateCanceled, -1, "canceled"
	}
	rr.status = runtime.Status{State: state, ExitCode: code, Err: errMsg}
	rr.mu.Unlock()
}

// Signal forwards HITL input to the remote run.
func (rr *remoteRun) Signal(input string) error {
	return rr.post("/api/exec/"+rr.id+"/signal", strings.NewReader(input))
}

// Cancel stops the remote run: it asks the node to cancel (best-effort), then
// tears down the streaming connection.
func (rr *remoteRun) Cancel() error {
	rr.mu.Lock()
	rr.canceled = true
	rr.mu.Unlock()
	err := rr.post("/api/exec/"+rr.id+"/cancel", nil)
	rr.cancel() // drop the stream connection -> node's request context cancels too
	return err
}

func (rr *remoteRun) post(path string, body io.Reader) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rr.rt.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+rr.rt.token)
	resp, err := rr.rt.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (rr *remoteRun) Status() runtime.Status {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.status
}

func (rr *remoteRun) Wait() runtime.Status {
	<-rr.done
	return rr.Status()
}
