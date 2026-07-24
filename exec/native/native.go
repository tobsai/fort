// Package native is Fort's NativeRuntime (backlog AO-014): it spawns agent CLIs
// itself — no Multica — normalizes their stdout into runtime.RunEvents, injects
// stdin for Signal (human-in-the-loop), and tracks exit codes.
//
// The executor is provider-agnostic: a Provider maps an agent name to argv and
// an optional line parser. DefaultProviders() encodes the AO-002 recon contract
// for claude/codex/hermes/openclaw (including the verified OpenClaw contract in
// spec 023).
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
	// Probe is a token-free command that proves the provider's non-interactive
	// entry point still exists in the installed CLI. Dispatch fails closed when
	// it exits non-zero, preventing a removed subcommand from accepting work.
	Probe []string
	// Command builds the argv for a run (argv[0] is the binary).
	Command func(spec runtime.RunSpec) []string
	// Failure recognizes a terminal provider error written to stdout. Some
	// agent CLIs report retry exhaustion or API rejection but still exit zero;
	// those lines must fail the run rather than produce a false success.
	Failure func(line string) (message string, ok bool)
	// Parse optionally normalizes a stdout line into a message. When ok is
	// false the line is emitted as a raw EventStdout.
	Parse func(line string) (msg string, ok bool)
	// Classify optionally turns a stdout line into typed events (spec 030). A
	// line may yield several (text + tool_use blocks). When set it supersedes
	// Parse; ok=false falls through to a raw EventStdout.
	Classify func(line string) ([]Classified, bool)
	// Interactive opts the provider into a stdin PIPE so Signal can inject input
	// mid-run (HITL). Default false: the child gets /dev/null and so an immediate
	// EOF. This matters — a CLI that drains stdin (e.g. `codex exec`, which
	// prints "Reading additional input from stdin...") hangs forever on an open
	// pipe that is never written to nor closed. None of the shipped agent CLIs
	// read their prompt from stdin; they take it as argv.
	Interactive bool
}

// Classified is one typed event extracted from a provider stdout line.
type Classified struct {
	Type runtime.EventType
	Data string
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
	if err := checkProvider(ctx, p, r.scopedEnv(spec)); err != nil {
		return nil, err
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
	setProcGroup(cmd) // own process group so Cancel can kill the whole tree

	// Only an Interactive provider gets a stdin pipe. Otherwise cmd.Stdin stays
	// nil, so os/exec hands the child /dev/null and it sees EOF immediately —
	// a CLI that drains stdin would otherwise block forever on a pipe nobody
	// ever writes to or closes.
	var stdin io.WriteCloser
	if p.Interactive {
		var err error
		stdin, err = cmd.StdinPipe()
		if err != nil {
			cancel()
			return nil, err
		}
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
		spec:             spec,
		parse:            p.Parse,
		classify:         p.Classify,
		failure:          p.Failure,
		events:           make(chan runtime.RunEvent, 64),
		done:             make(chan struct{}),
		streamsEOF:       make(chan struct{}),
		stdin:            stdin,
		cancel:           cancel,
		killProcessGroup: killProcGroup,
		status:           runtime.Status{State: runtime.StateRunning},
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("native: start %s: %w", argv[0], err)
	}
	pgid := cmd.Process.Pid // == the new process group's ID (Setpgid)
	exitWatcher, err := newLeaderExitWatcher(pgid)
	if err != nil {
		// The child has not been reaped, so its group identity cannot have been
		// reused. Fail closed and clean it up rather than run without a safe
		// non-reaping lifecycle boundary.
		killProcGroup(pgid)
		cancel()
		_ = cmd.Wait()
		return nil, fmt.Errorf("native: watch process exit: %w", err)
	}
	run.pgid = pgid
	run.leaderExit = exitWatcher
	go run.pump(cmd, stdout, stderr)
	return run, nil
}

const providerProbeTimeout = 5 * time.Second

// CheckProvider runs a provider's token-free CLI contract probe. It is public
// so mesh enrollment advertises only providers whose installed command surface
// is compatible with this Fort binary.
func CheckProvider(ctx context.Context, p Provider) error {
	return checkProvider(ctx, p, nil)
}

func checkProvider(ctx context.Context, p Provider, env []string) error {
	if len(p.Probe) == 0 {
		return nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, providerProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, p.Probe[0], p.Probe[1:]...)
	if env != nil {
		cmd.Env = env
	}
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	detail := strings.TrimSpace(string(output))
	if len(detail) > 1_024 {
		detail = detail[:1_024]
	}
	if detail != "" {
		return fmt.Errorf("native: provider %q command contract unavailable: %s: %w", p.Name, detail, err)
	}
	return fmt.Errorf("native: provider %q command contract unavailable: %w", p.Name, err)
}

type commandWaiter interface {
	Wait() error
}

type leaderExitWatcher interface {
	Wait() error
	Close() error
}

type nativeRun struct {
	spec       runtime.RunSpec
	parse      func(string) (string, bool)
	classify   func(string) ([]Classified, bool)
	failure    func(string) (string, bool)
	events     chan runtime.RunEvent
	done       chan struct{}
	streamsEOF chan struct{}
	stdin      io.WriteCloser
	cancel     context.CancelFunc
	leaderExit leaderExitWatcher
	// pgid is a protected ownership token for signaling the process group.
	// It remains live until the non-reaping watcher confirms leader exit, then
	// pump cleans remaining descendants and retires it before calling Wait.
	pgid             int
	killProcessGroup func(int)

	mu       sync.Mutex
	status   runtime.Status
	canceled bool
	fatal    string
}

func (n *nativeRun) ID() string                      { return n.spec.RunID }
func (n *nativeRun) Stream() <-chan runtime.RunEvent { return n.events }

func (n *nativeRun) emit(t runtime.EventType, data string, code int) {
	n.events <- runtime.RunEvent{RunID: n.spec.RunID, Type: t, Time: time.Now(), Data: data, Code: code}
}

func (n *nativeRun) pump(cmd commandWaiter, stdout, stderr io.Reader) {
	defer close(n.events)
	defer close(n.done)
	defer n.cancel()

	n.emit(runtime.EventStarted, n.spec.Agent, 0)

	var wg sync.WaitGroup
	wg.Add(2)
	go n.scan(&wg, stdout, false)
	go n.scan(&wg, stderr, true)
	go func() {
		wg.Wait()
		close(n.streamsEOF)
	}()

	watchErr := n.leaderExit.Wait()
	_ = n.leaderExit.Close()

	// The watcher does not reap: the exited leader still pins its PID/PGID, so
	// the numeric group identity cannot be reused. Clean any remaining
	// descendants while holding the same lock used by Cancel, then retire the
	// PGID before draining pipes and allowing cmd.Wait to reap the leader.
	n.mu.Lock()
	if n.pgid > 0 {
		n.killProcessGroup(n.pgid)
	}
	n.pgid = 0
	if watchErr != nil && n.fatal == "" {
		n.fatal = fmt.Sprintf("native: watch process exit: %v", watchErr)
	}
	n.mu.Unlock()

	<-n.streamsEOF
	err := cmd.Wait()
	code := exitCode(err)

	n.mu.Lock()
	canceled := n.canceled
	fatal := n.fatal
	switch {
	case canceled:
		n.status = runtime.Status{State: runtime.StateCanceled, ExitCode: code, Err: "canceled"}
	case fatal != "":
		if code == 0 {
			code = 1
		}
		n.status = runtime.Status{State: runtime.StateFailed, ExitCode: code, Err: fatal}
	case err == nil:
		n.status = runtime.Status{State: runtime.StateSucceeded}
	default:
		n.status = runtime.Status{State: runtime.StateFailed, ExitCode: code, Err: err.Error()}
	}
	n.mu.Unlock()

	switch {
	case canceled:
		n.emit(runtime.EventExited, "", code)
	case fatal != "":
		n.emit(runtime.EventError, fatal, 0)
		n.emit(runtime.EventExited, "", code)
	case err == nil:
		n.emit(runtime.EventExited, "", 0)
	default:
		n.emit(runtime.EventExited, "", code)
	}
}

func (n *nativeRun) scan(wg *sync.WaitGroup, r io.Reader, isErr bool) {
	defer wg.Done()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !isErr && n.failure != nil {
			if message, ok := n.failure(line); ok {
				n.mu.Lock()
				if n.fatal == "" {
					n.fatal = message
				}
				n.mu.Unlock()
			}
		}
		switch {
		case isErr:
			n.emit(runtime.EventStderr, line, 0)
		case n.classify != nil:
			if evs, ok := n.classify(line); ok && len(evs) > 0 {
				for _, ce := range evs {
					n.emit(ce.Type, ce.Data, 0)
				}
			} else {
				n.emit(runtime.EventStdout, line, 0)
			}
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
	if n.stdin == nil {
		return fmt.Errorf("native: provider %q is not interactive; it accepts no stdin", n.spec.Agent)
	}
	_, err := io.WriteString(n.stdin, input+"\n")
	return err
}

func (n *nativeRun) Cancel() error {
	n.mu.Lock()
	if n.status.Terminal() || n.canceled {
		n.mu.Unlock()
		return nil
	}
	n.canceled = true
	pgid := n.pgid
	cancel := n.cancel
	// Kill the whole process group first: a grandchild the CLI backgrounded may
	// hold the stdout/stderr pipes open, which would block the scanner
	// goroutines on Scan() and stall teardown. Cancelling the context alone only
	// SIGKILLs the direct child. The signal remains under mu so pump cannot
	// observe leader exit, clean descendants, and retire the group between this
	// ownership decision and kill. Pump retains the PGID for final cleanup.
	if pgid > 0 {
		kill := n.killProcessGroup
		if kill == nil {
			kill = killProcGroup
		}
		kill(pgid)
	}
	cancel()
	n.mu.Unlock()
	return nil
}

func (n *nativeRun) Status() runtime.Status {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.status
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
