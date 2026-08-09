// Package codexsubscription implements the isolated, subscription-backed
// Primary Channel execution contract. It is deliberately separate from the
// ordinary tool-capable native Codex provider.
package codexsubscription

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	coreruntime "github.com/tobsai/fort/core/runtime"
)

const targetTimeout = TargetTimeoutMillis * time.Millisecond

type HeldExecutable struct {
	Path               string
	Version            string
	ExecutableRevision string
	SchemaRevision     string
	Environment        []string
}

// Resolver returns immutable, capability-authorized Codex bytes and the
// schema identity verified against those same bytes.
type Resolver interface {
	ResolveCodex(context.Context) (HeldExecutable, error)
}

type Options struct {
	WorkRoot string
	Resolver Resolver
	Starter  Starter
}

type Runtime struct {
	workRoot string
	resolver Resolver
	starter  Starter
}

func New(options Options) (*Runtime, error) {
	if !filepath.IsAbs(options.WorkRoot) || options.Resolver == nil {
		return nil, fmt.Errorf("codex subscription: invalid options")
	}
	starter := options.Starter
	if starter == nil {
		starter = osStarter{}
	}
	return &Runtime{workRoot: options.WorkRoot, resolver: options.Resolver, starter: starter}, nil
}

func (r *Runtime) Name() string { return "codex-subscription" }

func (r *Runtime) Dispatch(parent context.Context, spec coreruntime.RunSpec) (coreruntime.Run, error) {
	if r == nil || r.resolver == nil || r.starter == nil || parent == nil {
		return nil, policyUnavailable()
	}
	if err := spec.ValidateAuthority(); err != nil || strings.TrimSpace(spec.RunID) == "" ||
		spec.Prompt == "" || len(spec.Prompt) > maxPromptBytes || strings.ContainsRune(spec.Prompt, '\x00') {
		return nil, policyUnavailable()
	}
	policy := spec.TextOnlyPolicy
	if policy.PolicyRevision != PolicyRevision ||
		policy.SelectedAdapterRevision != AdapterRevision ||
		policy.DeveloperInstructionRevision != DeveloperInstructionRevision ||
		policy.IsolationRevision != IsolationRevision ||
		policy.SelectedCodexVersion != CodexVersion ||
		policy.SelectedCodexExecutableRevision != CodexExecutableRevision ||
		policy.SelectedCodexSchemaRevision != CodexSchemaRevision {
		return nil, policyUnavailable()
	}

	ctx, cancel := context.WithTimeout(parent, targetTimeout)
	if err := ctx.Err(); err != nil {
		cancel()
		return nil, policyUnavailable()
	}
	held, err := r.resolver.ResolveCodex(ctx)
	if err != nil || !filepath.IsAbs(held.Path) || held.Version != policy.SelectedCodexVersion ||
		held.ExecutableRevision != policy.SelectedCodexExecutableRevision ||
		held.SchemaRevision != policy.SelectedCodexSchemaRevision {
		cancel()
		return nil, policyUnavailable()
	}
	if err := os.MkdirAll(r.workRoot, 0o700); err != nil {
		cancel()
		return nil, policyUnavailable()
	}
	workdir, err := os.MkdirTemp(r.workRoot, "fort-primary-")
	if err != nil {
		cancel()
		return nil, policyUnavailable()
	}
	cleanup := func() { _ = os.RemoveAll(workdir) }
	if err := os.Chmod(workdir, 0o700); err != nil {
		cleanup()
		cancel()
		return nil, policyUnavailable()
	}
	entries, err := os.ReadDir(workdir)
	if err != nil || len(entries) != 0 {
		cleanup()
		cancel()
		return nil, policyUnavailable()
	}
	request := StartRequest{
		Executable: held.Path,
		Args:       ExecArguments(spec.Prompt, spec.Model, workdir),
		Workdir:    workdir, Environment: append([]string(nil), held.Environment...), StdinNull: true,
	}
	process, err := r.starter.Start(ctx, request)
	if err != nil || process == nil {
		cleanup()
		cancel()
		return nil, fmt.Errorf("codex subscription: process start failed")
	}
	run := &subscriptionRun{
		spec: spec, held: held,
		process: process, events: make(chan coreruntime.RunEvent, 8), done: make(chan struct{}),
		ctx: ctx, cancel: cancel, cleanup: cleanup,
		status: coreruntime.Status{State: coreruntime.StateRunning},
	}
	run.emit(coreruntime.RunEvent{RunID: spec.RunID, Type: coreruntime.EventStarted, Time: time.Now().UTC()})
	go run.execute()
	return run, nil
}

func policyUnavailable() error {
	return fmt.Errorf("codex subscription: %s", coreruntime.ErrorChatPolicyUnavailable)
}

type subscriptionRun struct {
	spec    coreruntime.RunSpec
	held    HeldExecutable
	process Process
	events  chan coreruntime.RunEvent
	done    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	cleanup func()

	mu       sync.RWMutex
	status   coreruntime.Status
	killOnce sync.Once
	endOnce  sync.Once
}

func (r *subscriptionRun) ID() string                          { return r.spec.RunID }
func (r *subscriptionRun) Stream() <-chan coreruntime.RunEvent { return r.events }
func (r *subscriptionRun) Signal(string) error {
	return fmt.Errorf("codex subscription: input unsupported")
}

func (r *subscriptionRun) Cancel() error {
	r.mu.RLock()
	terminal := r.status.Terminal()
	r.mu.RUnlock()
	if terminal {
		return nil
	}
	r.cancel()
	r.kill()
	return nil
}

func (r *subscriptionRun) Status() coreruntime.Status {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

func (r *subscriptionRun) Wait() coreruntime.Status {
	<-r.done
	return r.Status()
}

func (r *subscriptionRun) emit(event coreruntime.RunEvent) { r.events <- event }

func (r *subscriptionRun) kill() {
	r.killOnce.Do(func() { _ = r.process.KillProcessGroup() })
}

type waitResult struct {
	code int
	err  error
}

func (r *subscriptionRun) execute() {
	stdoutCh := make(chan parseResult, 1)
	stderrCh := make(chan error, 1)
	waitCh := make(chan waitResult, 1)
	go func() { stdoutCh <- parseJSONL(r.process.Stdout()) }()
	go func() { stderrCh <- drainBounded(r.process.Stderr(), maxStderrBytes) }()
	go func() {
		code, err := r.process.Wait()
		waitCh <- waitResult{code: code, err: err}
	}()

	var parsed parseResult
	var stderrErr error
	var waited waitResult
	var gotStdout, gotStderr, gotWait bool
	failureCode := ""
	canceled := false
	ctxDone := r.ctx.Done()
	for !gotStdout || !gotStderr || !gotWait {
		select {
		case result := <-stdoutCh:
			if !gotStdout {
				parsed, gotStdout = result, true
				if result.err != nil && failureCode == "" {
					failureCode = failureCodeOf(result.err)
					r.kill()
				}
			}
		case err := <-stderrCh:
			if !gotStderr {
				stderrErr, gotStderr = err, true
				if err != nil && failureCode == "" {
					failureCode = coreruntime.ErrorProviderFailed
					r.kill()
				}
			}
		case result := <-waitCh:
			if !gotWait {
				waited, gotWait = result, true
			}
		case <-ctxDone:
			ctxDone = nil
			if errors.Is(r.ctx.Err(), context.DeadlineExceeded) {
				if failureCode == "" {
					failureCode = coreruntime.ErrorProviderResultUnknown
				}
			} else {
				canceled = true
			}
			r.kill()
		}
	}

	if canceled {
		r.finish(coreruntime.Status{State: coreruntime.StateCanceled, ExitCode: waited.code}, "canceled", waited.code, nil)
		return
	}
	if failureCode == "" && (stderrErr != nil || waited.err != nil || waited.code != 0 || parsed.err != nil) {
		failureCode = coreruntime.ErrorProviderFailed
	}
	if failureCode != "" {
		r.finish(coreruntime.Status{State: coreruntime.StateFailed, ExitCode: waited.code, Err: failureCode}, failureCode, waited.code, nil)
		return
	}
	metadata := &coreruntime.ResponseMetadata{
		ProviderThreadID: parsed.threadID, RequestedModel: r.spec.Model,
		ResolvedModel:     coreruntime.UnknownProviderIdentity,
		SelectedAdapterID: coreruntime.AdapterCodexSubscription, SelectedAdapterRevision: AdapterRevision,
		SelectedCodexVersion:            r.held.Version,
		SelectedCodexExecutableRevision: r.held.ExecutableRevision,
		SelectedCodexSchemaRevision:     r.held.SchemaRevision,
		ObservedAdapterID:               coreruntime.AdapterCodexSubscription, ObservedAdapterRevision: AdapterRevision,
		ObservedCodexVersion:            r.held.Version,
		ObservedCodexExecutableRevision: r.held.ExecutableRevision,
		ObservedCodexSchemaRevision:     r.held.SchemaRevision,
		TerminalStatus:                  "completed", UsageSource: coreruntime.UsageSourceCodexExecJSONL, Usage: parsed.usage,
	}
	if err := metadata.Validate(); err != nil {
		r.finish(coreruntime.Status{State: coreruntime.StateFailed, ExitCode: waited.code, Err: coreruntime.ErrorProviderFailed}, coreruntime.ErrorProviderFailed, waited.code, nil)
		return
	}
	r.emit(coreruntime.RunEvent{
		RunID: r.spec.RunID, Type: coreruntime.EventMessage, Time: time.Now().UTC(),
		Data: parsed.message, Response: metadata,
	})
	r.finish(coreruntime.Status{State: coreruntime.StateSucceeded, ExitCode: 0}, "", 0, nil)
}

func failureCodeOf(err error) string {
	var failure *streamFailure
	if errors.As(err, &failure) && failure.code != "" {
		return failure.code
	}
	return coreruntime.ErrorProviderFailed
}

func (r *subscriptionRun) finish(status coreruntime.Status, errorCode string, exitCode int, response *coreruntime.ResponseMetadata) {
	r.endOnce.Do(func() {
		r.mu.Lock()
		r.status = status
		r.mu.Unlock()
		if errorCode != "" {
			r.emit(coreruntime.RunEvent{
				RunID: r.spec.RunID, Type: coreruntime.EventError, Time: time.Now().UTC(),
				Data: errorCode, ErrorCode: errorCode, Response: response,
			})
		}
		r.emit(coreruntime.RunEvent{RunID: r.spec.RunID, Type: coreruntime.EventExited, Time: time.Now().UTC(), Code: exitCode})
		r.cancel()
		if r.cleanup != nil {
			r.cleanup()
		}
		close(r.events)
		close(r.done)
	})
}
