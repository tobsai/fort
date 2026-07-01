// Package native is Fort's NativeRuntime (backlog AO-014): it spawns agent CLIs
// itself — no Multica — normalizes their stdout into runtime.RunEvents, injects
// stdin for Signal (human-in-the-loop), and tracks exit codes.
//
// The executor is provider-agnostic: a Provider maps an agent name to argv and
// an optional line parser. DefaultProviders() encodes the AO-002 recon contract
// for claude/codex/hermes (openclaw pending install).
package native

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/tobsai/fort/core/runtime"
)

// Provider describes how to launch one agent CLI headless.
type Provider struct {
	Name string
	// Command builds the argv for a run (argv[0] is the binary).
	Command func(spec runtime.RunSpec) []string
	// Parse optionally normalizes a stdout line into a message. When ok is
	// false the line is emitted as a raw EventStdout.
	Parse func(line string) (msg string, ok bool)
}

// Runtime is the native executor.
type Runtime struct {
	providers map[string]Provider
	workRoot  string
	// EnvAllow, when non-empty, restricts which host environment variables are
	// passed to spawned CLIs (least privilege, AO-041). Empty = pass the full
	// environment (the relaxed default; providers need their own auth keys).
	EnvAllow []string
}

// New builds a native runtime rooted at workRoot with the given providers.
func New(workRoot string, providers ...Provider) *Runtime {
	r := &Runtime{providers: map[string]Provider{}, workRoot: workRoot}
	for _, p := range providers {
		r.providers[p.Name] = p
	}
	return r
}

// Name implements runtime.Runtime.
func (r *Runtime) Name() string { return "native" }

func (r *Runtime) provider(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// scopedEnv builds the environment for a spawned CLI, honoring EnvAllow.
func (r *Runtime) scopedEnv(spec runtime.RunSpec) []string {
	if len(r.EnvAllow) == 0 {
		return append(os.Environ(), spec.Env...)
	}
	allow := make(map[string]bool, len(r.EnvAllow))
	for _, k := range r.EnvAllow {
		allow[k] = true
	}
	var out []string
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i > 0 && allow[kv[:i]] {
			out = append(out, kv)
		}
	}
	return append(out, spec.Env...)
}

// Dispatch launches spec via its provider.
func (r *Runtime) Dispatch(ctx context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	p, ok := r.providers[spec.Agent]
	if !ok {
		return nil, fmt.Errorf("native: no provider registered for agent %q", spec.Agent)
	}
	workdir := spec.Workdir
	if workdir == "" {
		workdir = r.workRoot
	}
	if workdir != "" {
		if err := os.MkdirAll(workdir, 0o755); err != nil {
			return nil, fmt.Errorf("native: workdir: %w", err)
		}
	}

	argv := p.Command(spec)
	if len(argv) == 0 {
		return nil, fmt.Errorf("native: provider %q produced empty argv", spec.Agent)
	}

	cctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cctx, argv[0], argv[1:]...)
	cmd.Dir = workdir
	cmd.Env = r.scopedEnv(spec)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, err
	}

	run := &nativeRun{
		spec:   spec,
		parse:  p.Parse,
		events: make(chan runtime.RunEvent, 64),
		done:   make(chan struct{}),
		stdin:  stdin,
		cancel: cancel,
		status: runtime.Status{State: runtime.StateRunning},
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("native: start %s: %w", argv[0], err)
	}
	go run.pump(cmd, stdout, stderr)
	return run, nil
}

type nativeRun struct {
	spec   runtime.RunSpec
	parse  func(string) (string, bool)
	events chan runtime.RunEvent
	done   chan struct{}
	stdin  io.WriteCloser
	cancel context.CancelFunc

	mu       sync.Mutex
	status   runtime.Status
	canceled bool
}

func (n *nativeRun) ID() string                      { return n.spec.RunID }
func (n *nativeRun) Stream() <-chan runtime.RunEvent { return n.events }

func (n *nativeRun) emit(t runtime.EventType, data string, code int) {
	n.events <- runtime.RunEvent{RunID: n.spec.RunID, Type: t, Time: time.Now(), Data: data, Code: code}
}

func (n *nativeRun) pump(cmd *exec.Cmd, stdout, stderr io.Reader) {
	defer close(n.events)
	defer close(n.done)
	defer n.cancel()

	n.emit(runtime.EventStarted, n.spec.Agent, 0)

	var wg sync.WaitGroup
	wg.Add(2)
	go n.scan(&wg, stdout, false)
	go n.scan(&wg, stderr, true)
	wg.Wait()

	err := cmd.Wait()
	code := exitCode(err)

	n.mu.Lock()
	canceled := n.canceled
	n.mu.Unlock()

	switch {
	case canceled:
		n.emit(runtime.EventExited, "", code)
		n.setStatus(runtime.StateCanceled, code, "canceled")
	case err == nil:
		n.emit(runtime.EventExited, "", 0)
		n.setStatus(runtime.StateSucceeded, 0, "")
	default:
		n.emit(runtime.EventExited, "", code)
		n.setStatus(runtime.StateFailed, code, err.Error())
	}
}

func (n *nativeRun) scan(wg *sync.WaitGroup, r io.Reader, isErr bool) {
	defer wg.Done()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case isErr:
			n.emit(runtime.EventStderr, line, 0)
		case n.parse != nil:
			if msg, ok := n.parse(line); ok {
				n.emit(runtime.EventMessage, msg, 0)
			} else {
				n.emit(runtime.EventStdout, line, 0)
			}
		default:
			n.emit(runtime.EventStdout, line, 0)
		}
	}
}

func (n *nativeRun) Signal(input string) error {
	_, err := io.WriteString(n.stdin, input+"\n")
	return err
}

func (n *nativeRun) Cancel() error {
	n.mu.Lock()
	n.canceled = true
	n.mu.Unlock()
	n.cancel()
	return nil
}

func (n *nativeRun) Status() runtime.Status {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.status
}

func (n *nativeRun) setStatus(state runtime.State, code int, errMsg string) {
	n.mu.Lock()
	n.status = runtime.Status{State: state, ExitCode: code, Err: errMsg}
	n.mu.Unlock()
}

func (n *nativeRun) Wait() runtime.Status {
	<-n.done
	return n.Status()
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}
