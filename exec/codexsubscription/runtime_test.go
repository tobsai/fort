package codexsubscription

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	coreruntime "github.com/tobsai/fort/core/runtime"
)

const (
	testCodexVersion = CodexVersion
	testAdapterRev   = AdapterRevision
	testDeveloperRev = DeveloperInstructionRevision
	testIsolationRev = IsolationRevision
)

var (
	testExecutableRev = CodexExecutableRevision
	testSchemaRev     = CodexSchemaRevision
)

type fakeResolver struct {
	held HeldExecutable
	err  error
}

func (r fakeResolver) ResolveCodex(context.Context) (HeldExecutable, error) {
	return r.held, r.err
}

type fakeStarter struct {
	mu       sync.Mutex
	requests []StartRequest
	process  Process
	err      error
	check    func(context.Context, StartRequest) error
}

func (s *fakeStarter) Start(ctx context.Context, request StartRequest) (Process, error) {
	s.mu.Lock()
	s.requests = append(s.requests, request)
	s.mu.Unlock()
	if s.check != nil {
		if err := s.check(ctx, request); err != nil {
			return nil, err
		}
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.process, nil
}

func (s *fakeStarter) captured() []StartRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]StartRequest(nil), s.requests...)
}

type fakeProcess struct {
	stdout io.Reader
	stderr io.Reader
	code   int
	err    error
	wait   <-chan struct{}

	mu     sync.Mutex
	killed bool
}

func (p *fakeProcess) Stdout() io.Reader { return p.stdout }
func (p *fakeProcess) Stderr() io.Reader { return p.stderr }
func (p *fakeProcess) Wait() (int, error) {
	if p.wait != nil {
		<-p.wait
	}
	return p.code, p.err
}
func (p *fakeProcess) KillProcessGroup() error {
	p.mu.Lock()
	p.killed = true
	p.mu.Unlock()
	return nil
}
func (p *fakeProcess) wasKilled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.killed
}

func heldExecutable() HeldExecutable {
	return HeldExecutable{
		Path:               "/held/codex",
		Version:            testCodexVersion,
		ExecutableRevision: testExecutableRev,
		SchemaRevision:     testSchemaRev,
		Environment:        []string{"CODEX_HOME=/private/auth", "PATH=/usr/bin"},
	}
}

func validSpec() coreruntime.RunSpec {
	return coreruntime.RunSpec{
		RunID:           "run-1",
		Profile:         "codex-subscription:gpt-5.6-sol",
		Agent:           "codex-subscription",
		Model:           "gpt-5.6-sol",
		Prompt:          "hello from Fort",
		Authority:       coreruntime.AuthorityChatSubscriptionIsolatedV1,
		RuntimeContract: coreruntime.RuntimeContractCodexSubscriptionExecV1,
		TextOnlyPolicy: &coreruntime.TextOnlyPolicy{
			PolicyID:                        coreruntime.PolicyCodexSubscriptionChatV1,
			PolicyRevision:                  PolicyRevision,
			Model:                           "gpt-5.6-sol",
			ReasoningEffort:                 coreruntime.ReasoningEffortMedium,
			ReasoningContext:                coreruntime.ReasoningContextCurrentTurn,
			RequestTimeoutMillis:            120000,
			DeveloperInstructionRevision:    testDeveloperRev,
			AccountType:                     coreruntime.AccountTypeChatGPT,
			AccountPlan:                     "pro",
			SelectedAdapterID:               coreruntime.AdapterCodexSubscription,
			SelectedAdapterRevision:         testAdapterRev,
			SelectedCodexVersion:            testCodexVersion,
			SelectedCodexExecutableRevision: testExecutableRev,
			SelectedCodexSchemaRevision:     testSchemaRev,
			ThreadMode:                      coreruntime.ThreadModeEphemeral,
			SandboxMode:                     coreruntime.SandboxModeReadOnly,
			ApprovalPolicy:                  coreruntime.ApprovalPolicyNever,
			WorkdirMode:                     coreruntime.WorkdirModeEmptyPerTarget,
			DynamicToolsMode:                coreruntime.ToolsModeNone,
			MCPMode:                         coreruntime.ToolsModeNone,
			CommandPolicy:                   coreruntime.ResourcePolicyDenyAndFail,
			FileReadPolicy:                  coreruntime.ResourcePolicyDenyAndFail,
			IsolationRevision:               testIsolationRev,
		},
	}
}

func successJSONL(message string) string {
	return strings.Join([]string{
		`{"type":"thread.started","thread_id":"thread-1"}`,
		`{"type":"item.completed","item":{"id":"item_0","type":"error","message":"Code Mode is unavailable because code-mode host is disabled. Code mode will fail closed; enable ` + "`features.code_mode_host`" + ` and install ` + "`codex-code-mode-host`" + `."}}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"id":"reason-1","type":"reasoning","text":"private"}}`,
		`{"type":"item.completed","item":{"id":"message-1","type":"agent_message","text":` + quoteJSON(message) + `}}`,
		`{"type":"turn.completed","usage":{"input_tokens":12,"cached_input_tokens":3,"output_tokens":5,"reasoning_tokens":2}}`,
	}, "\n") + "\n"
}

func liveSuccessJSONL(message string) string {
	return strings.Join([]string{
		`{"type":"thread.started","thread_id":"thread-1"}`,
		`{"type":"item.completed","item":{"id":"item_0","type":"error","message":"Code Mode is unavailable because code-mode host is disabled. Code mode will fail closed; enable ` + "`features.code_mode_host`" + ` and install ` + "`codex-code-mode-host`" + `."}}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"id":"message-1","type":"agent_message","text":` + quoteJSON(message) + `}}`,
		`{"type":"turn.completed","usage":{"input_tokens":12,"cached_input_tokens":3,"output_tokens":5,"reasoning_output_tokens":2}}`,
	}, "\n") + "\n"
}

func quoteJSON(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func newTestRuntime(t *testing.T, starter *fakeStarter) *Runtime {
	t.Helper()
	result, err := New(Options{
		WorkRoot: t.TempDir(),
		Resolver: fakeResolver{held: heldExecutable()},
		Starter:  starter,
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func collect(run coreruntime.Run) ([]coreruntime.RunEvent, coreruntime.Status) {
	var events []coreruntime.RunEvent
	for event := range run.Stream() {
		events = append(events, event)
	}
	return events, run.Wait()
}

func messages(events []coreruntime.RunEvent) []coreruntime.RunEvent {
	var result []coreruntime.RunEvent
	for _, event := range events {
		if event.Type == coreruntime.EventMessage {
			result = append(result, event)
		}
	}
	return result
}

func errorCode(events []coreruntime.RunEvent) string {
	for _, event := range events {
		if event.Type == coreruntime.EventError {
			return event.ErrorCode
		}
	}
	return ""
}

func TestRuntimeUsesExactEphemeralInvocationAndValidatedJSONL(t *testing.T) {
	process := &fakeProcess{stdout: strings.NewReader(successJSONL("answer")), stderr: strings.NewReader("")}
	starter := &fakeStarter{process: process}
	starter.check = func(ctx context.Context, request StartRequest) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) < 119*time.Second || time.Until(deadline) > 121*time.Second {
			return errors.New("target deadline is not 120 seconds")
		}
		info, err := os.Stat(request.Workdir)
		if err != nil || info.Mode().Perm() != 0o700 {
			return errors.New("workdir is not mode 0700")
		}
		entries, err := os.ReadDir(request.Workdir)
		if err != nil || len(entries) != 0 {
			return errors.New("workdir is not empty")
		}
		return nil
	}
	runtime := newTestRuntime(t, starter)
	run, err := runtime.Dispatch(context.Background(), validSpec())
	if err != nil {
		t.Fatal(err)
	}
	events, status := collect(run)
	if status.State != coreruntime.StateSucceeded {
		t.Fatalf("status = %#v", status)
	}
	gotMessages := messages(events)
	if len(gotMessages) != 1 || gotMessages[0].Data != "answer" || gotMessages[0].Response == nil {
		t.Fatalf("messages = %#v", gotMessages)
	}
	metadata := gotMessages[0].Response
	if err := metadata.Validate(); err != nil {
		t.Fatalf("metadata: %v (%#v)", err, metadata)
	}
	if metadata.ProviderThreadID != "thread-1" || metadata.RequestedModel != "gpt-5.6-sol" ||
		metadata.ResolvedModel != coreruntime.UnknownProviderIdentity || metadata.Usage.InputTokens != 12 ||
		metadata.Usage.CachedInputTokens != 3 || metadata.Usage.OutputTokens != 5 || metadata.Usage.ReasoningTokens != 2 {
		t.Fatalf("metadata = %#v", metadata)
	}

	requests := starter.captured()
	if len(requests) != 1 {
		t.Fatalf("requests = %d", len(requests))
	}
	wantArgs := []string{
		"exec", "hello from Fort", "--json", "--sandbox", "read-only",
		"--skip-git-repo-check", "--ephemeral", "--ignore-user-config", "--ignore-rules", "--strict-config",
		"-C", requests[0].Workdir, "--model", "gpt-5.6-sol",
		"-c", `approval_policy="never"`,
		"-c", `developer_instructions="` + DeveloperInstruction + `"`,
		"-c", `model_reasoning_effort="medium"`,
		"-c", `web_search="disabled"`,
		"-c", "tools.update_plan.enabled=false",
		"-c", "tools.experimental_request_user_input.enabled=false",
		"-c", "skills.include_instructions=false",
		"-c", "skills.bundled.enabled=false",
		"-c", "include_apps_instructions=false",
		"-c", "include_collaboration_mode_instructions=false",
		"-c", "include_environment_context=false",
		"-c", "include_permissions_instructions=false",
		"--disable", "shell_tool", "--disable", "unified_exec", "--disable", "apps",
		"--disable", "browser_use", "--disable", "computer_use", "--disable", "image_generation",
		"--disable", "in_app_browser", "--disable", "multi_agent", "--disable", "memories",
		"--disable", "plugins", "--disable", "skill_search", "--disable", "workspace_dependencies",
		"--disable", "code_mode", "--disable", "code_mode_host",
		"--disable", "code_mode_only", "--disable", "code_mode_buffered_exec",
	}
	if requests[0].Executable != "/held/codex" || !reflect.DeepEqual(requests[0].Args, wantArgs) ||
		!reflect.DeepEqual(requests[0].Environment, heldExecutable().Environment) || !requests[0].StdinNull {
		t.Fatalf("request = %#v", requests[0])
	}
	if _, err := os.Stat(requests[0].Workdir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("workdir was not removed: %v", err)
	}
}

func TestRuntimeAcceptsOnlyPinnedFailClosedCodeModeDiagnostic(t *testing.T) {
	process := &fakeProcess{stdout: strings.NewReader(liveSuccessJSONL("answer")), stderr: strings.NewReader("")}
	runtime := newTestRuntime(t, &fakeStarter{process: process})
	run, err := runtime.Dispatch(context.Background(), validSpec())
	if err != nil {
		t.Fatal(err)
	}
	events, status := collect(run)
	gotMessages := messages(events)
	if status.State != coreruntime.StateSucceeded || len(gotMessages) != 1 || gotMessages[0].Data != "answer" || gotMessages[0].Response == nil {
		t.Fatalf("status=%#v events=%#v", status, events)
	}
	if gotMessages[0].Response.Usage.ReasoningTokens != 2 {
		t.Fatalf("response metadata = %#v", gotMessages[0].Response)
	}

	for _, replacement := range []string{
		"Code Mode is unavailable because code-mode host was not found.",
		"Code Mode is unavailable because code-mode host is disabled. Code mode will continue.",
	} {
		t.Run(replacement, func(t *testing.T) {
			stream := strings.Replace(liveSuccessJSONL("discard"),
				"Code Mode is unavailable because code-mode host is disabled. Code mode will fail closed; enable `features.code_mode_host` and install `codex-code-mode-host`.", replacement, 1)
			process := &fakeProcess{stdout: strings.NewReader(stream), stderr: strings.NewReader("")}
			runtime := newTestRuntime(t, &fakeStarter{process: process})
			run, err := runtime.Dispatch(context.Background(), validSpec())
			if err != nil {
				t.Fatal(err)
			}
			events, status := collect(run)
			if status.State != coreruntime.StateFailed || len(messages(events)) != 0 {
				t.Fatalf("status=%#v events=%#v", status, events)
			}
		})
	}
}

func TestRuntimeUsesFreshWorkdirAndThreadForEveryTarget(t *testing.T) {
	starter := &fakeStarter{process: &fakeProcess{stdout: strings.NewReader(successJSONL("one")), stderr: strings.NewReader("")}}
	runtime := newTestRuntime(t, starter)
	first, err := runtime.Dispatch(context.Background(), validSpec())
	if err != nil {
		t.Fatal(err)
	}
	collect(first)
	starter.process = &fakeProcess{stdout: strings.NewReader(strings.ReplaceAll(successJSONL("two"), "thread-1", "thread-2")), stderr: strings.NewReader("")}
	secondSpec := validSpec()
	secondSpec.RunID = "run-2"
	second, err := runtime.Dispatch(context.Background(), secondSpec)
	if err != nil {
		t.Fatal(err)
	}
	collect(second)
	requests := starter.captured()
	if len(requests) != 2 || requests[0].Workdir == requests[1].Workdir {
		t.Fatalf("workdirs = %#v", requests)
	}
	for _, request := range requests {
		for _, arg := range request.Args {
			if arg == "resume" || arg == "fork" || arg == "review" {
				t.Fatalf("session continuation argument escaped: %q", arg)
			}
		}
	}
}

func TestRuntimeRejectsEveryActiveJSONLItemWithoutMessage(t *testing.T) {
	for _, itemType := range []string{
		"command_execution", "file_read", "file_change", "mcp_tool_call", "collab_tool_call",
		"web_search", "todo_list", "dynamic_tool_call", "unknown_active",
	} {
		t.Run(itemType, func(t *testing.T) {
			stream := strings.Join([]string{
				`{"type":"thread.started","thread_id":"thread-1"}`,
				`{"type":"turn.started"}`,
				`{"type":"item.started","item":{"id":"active-1","type":"` + itemType + `"}}`,
				`{"type":"item.completed","item":{"id":"message-1","type":"agent_message","text":"false success"}}`,
				`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}`,
			}, "\n") + "\n"
			process := &fakeProcess{stdout: strings.NewReader(stream), stderr: strings.NewReader("")}
			runtime := newTestRuntime(t, &fakeStarter{process: process})
			run, err := runtime.Dispatch(context.Background(), validSpec())
			if err != nil {
				t.Fatal(err)
			}
			events, status := collect(run)
			if status.State != coreruntime.StateFailed || len(messages(events)) != 0 ||
				errorCode(events) != coreruntime.ErrorChatAuthorityViolation {
				t.Fatalf("status=%#v events=%#v", status, events)
			}
			if !process.wasKilled() {
				t.Fatal("authority violation did not terminate the process group")
			}
		})
	}
}

func TestRuntimeFailsClosedForInvalidStreamsAndProcessExit(t *testing.T) {
	duplicateMessage := strings.Replace(successJSONL("answer"),
		`{"type":"turn.completed"`, `{"type":"item.completed","item":{"id":"message-2","type":"agent_message","text":"second"}}\n{"type":"turn.completed"`, 1)
	tests := []struct {
		name   string
		stream string
		code   int
		err    error
	}{
		{"malformed", "not-json\n", 0, nil},
		{"error item", `{"type":"thread.started","thread_id":"t"}` + "\n" + `{"type":"turn.started"}` + "\n" + `{"type":"item.completed","item":{"type":"error","message":"no"}}` + "\n", 0, nil},
		{"turn failed", `{"type":"thread.started","thread_id":"t"}` + "\n" + `{"type":"turn.started"}` + "\n" + `{"type":"turn.failed"}` + "\n", 0, nil},
		{"duplicate thread", `{"type":"thread.started","thread_id":"t"}` + "\n" + `{"type":"thread.started","thread_id":"other"}` + "\n", 0, nil},
		{"duplicate turn", `{"type":"thread.started","thread_id":"t"}` + "\n" + `{"type":"turn.started"}` + "\n" + `{"type":"turn.started"}` + "\n", 0, nil},
		{"duplicate message", duplicateMessage, 0, nil},
		{"blank message", successJSONL("   "), 0, nil},
		{"duplicate completion", successJSONL("answer") + `{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}` + "\n", 0, nil},
		{"cached usage exceeds input", strings.Replace(successJSONL("answer"), `"input_tokens":12,"cached_input_tokens":3`, `"input_tokens":1,"cached_input_tokens":2`, 1), 0, nil},
		{"zero output usage with message", strings.Replace(successJSONL("answer"), `"output_tokens":5`, `"output_tokens":0`, 1), 0, nil},
		{"reasoning usage exceeds output", strings.Replace(successJSONL("answer"), `"output_tokens":5,"reasoning_tokens":2`, `"output_tokens":1,"reasoning_tokens":2`, 1), 0, nil},
		{"nonzero after success shape", successJSONL("must discard"), 7, errors.New("exit status 7")},
		{"missing completion", `{"type":"thread.started","thread_id":"t"}` + "\n" + `{"type":"turn.started"}` + "\n", 0, nil},
		{"unknown event", `{"type":"thread.started","thread_id":"t"}` + "\n" + `{"type":"future.event"}` + "\n", 0, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			process := &fakeProcess{stdout: strings.NewReader(test.stream), stderr: strings.NewReader("bounded stderr"), code: test.code, err: test.err}
			runtime := newTestRuntime(t, &fakeStarter{process: process})
			run, err := runtime.Dispatch(context.Background(), validSpec())
			if err != nil {
				t.Fatal(err)
			}
			events, status := collect(run)
			if status.State != coreruntime.StateFailed || len(messages(events)) != 0 || errorCode(events) == "" {
				t.Fatalf("status=%#v events=%#v", status, events)
			}
		})
	}
}

func TestRuntimeEnforcesOutputBounds(t *testing.T) {
	tooManyEvents := `{"type":"thread.started","thread_id":"t"}` + "\n" +
		`{"type":"turn.started"}` + "\n" +
		strings.Repeat(`{"type":"item.completed","item":{"type":"reasoning"}}`+"\n", maxJSONLEvents)
	tooMuchStdout := `{"type":"thread.started","thread_id":"t"}` + "\n" +
		`{"type":"turn.started"}` + "\n" +
		strings.Repeat(`{"type":"item.completed","item":{"type":"reasoning","text":"`+strings.Repeat("x", 4096)+`"}}`+"\n", 300)
	tests := []struct {
		name   string
		stdout string
		stderr string
	}{
		{"line", strings.Repeat("x", maxJSONLLineBytes+1) + "\n", ""},
		{"stdout", tooMuchStdout, ""},
		{"event count", tooManyEvents, ""},
		{"stderr", "", strings.Repeat("x", maxStderrBytes+1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			process := &fakeProcess{stdout: strings.NewReader(test.stdout), stderr: strings.NewReader(test.stderr)}
			runtime := newTestRuntime(t, &fakeStarter{process: process})
			run, err := runtime.Dispatch(context.Background(), validSpec())
			if err != nil {
				t.Fatal(err)
			}
			events, status := collect(run)
			if status.State != coreruntime.StateFailed || len(messages(events)) != 0 {
				t.Fatalf("status=%#v events=%#v", status, events)
			}
		})
	}
}

func TestRuntimeRejectsExecutableAndAdapterDriftBeforeStart(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*coreruntime.RunSpec, *HeldExecutable)
	}{
		{"executable", func(_ *coreruntime.RunSpec, held *HeldExecutable) {
			held.ExecutableRevision = strings.Repeat("c", 64)
		}},
		{"schema", func(_ *coreruntime.RunSpec, held *HeldExecutable) {
			held.SchemaRevision = strings.Repeat("c", 64)
		}},
		{"version", func(_ *coreruntime.RunSpec, held *HeldExecutable) { held.Version = "other" }},
		{"policy", func(spec *coreruntime.RunSpec, _ *HeldExecutable) {
			spec.TextOnlyPolicy.PolicyRevision = strings.Repeat("c", 64)
		}},
		{"adapter", func(spec *coreruntime.RunSpec, _ *HeldExecutable) {
			spec.TextOnlyPolicy.SelectedAdapterRevision = strings.Repeat("c", 64)
		}},
		{"developer", func(spec *coreruntime.RunSpec, _ *HeldExecutable) {
			spec.TextOnlyPolicy.DeveloperInstructionRevision = strings.Repeat("c", 64)
		}},
		{"isolation", func(spec *coreruntime.RunSpec, _ *HeldExecutable) {
			spec.TextOnlyPolicy.IsolationRevision = strings.Repeat("c", 64)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			held := heldExecutable()
			starter := &fakeStarter{process: &fakeProcess{stdout: strings.NewReader(successJSONL("x")), stderr: strings.NewReader("")}}
			options := Options{
				WorkRoot: t.TempDir(), Resolver: fakeResolver{held: held}, Starter: starter,
			}
			spec := validSpec()
			test.mutate(&spec, &held)
			options.Resolver = fakeResolver{held: held}
			runtime, err := New(options)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.Dispatch(context.Background(), spec); err == nil {
				t.Fatal("drifted dispatch accepted")
			}
			if len(starter.captured()) != 0 {
				t.Fatal("drift started a process")
			}
		})
	}
}

func TestRuntimeCancellationKillsProcessGroupAndEmitsNoMessage(t *testing.T) {
	wait := make(chan struct{})
	process := &fakeProcess{stdout: strings.NewReader(""), stderr: strings.NewReader(""), wait: wait}
	starter := &fakeStarter{process: process}
	runtime := newTestRuntime(t, starter)
	ctx, cancel := context.WithCancel(context.Background())
	run, err := runtime.Dispatch(ctx, validSpec())
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	deadline := time.Now().Add(time.Second)
	for !process.wasKilled() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	close(wait)
	events, status := collect(run)
	if !process.wasKilled() || status.State != coreruntime.StateCanceled || len(messages(events)) != 0 {
		t.Fatalf("killed=%v status=%#v events=%#v", process.wasKilled(), status, events)
	}
}

func TestNewRequiresClosedProductionInputs(t *testing.T) {
	valid := Options{
		WorkRoot: t.TempDir(), Resolver: fakeResolver{held: heldExecutable()}, Starter: &fakeStarter{},
	}
	tests := []func(*Options){
		func(v *Options) { v.WorkRoot = "" },
		func(v *Options) { v.Resolver = nil },
	}
	for _, mutate := range tests {
		candidate := valid
		mutate(&candidate)
		if _, err := New(candidate); err == nil {
			t.Fatal("invalid options accepted")
		}
	}
}

func TestRuntimeRejectsOversizedPromptBeforeStart(t *testing.T) {
	starter := &fakeStarter{process: &fakeProcess{}}
	runtime := newTestRuntime(t, starter)
	spec := validSpec()
	spec.Prompt = strings.Repeat("x", maxPromptBytes+1)
	if _, err := runtime.Dispatch(context.Background(), spec); err == nil {
		t.Fatal("oversized prompt accepted")
	}
	if len(starter.captured()) != 0 {
		t.Fatal("oversized prompt started a process")
	}
}

func TestRuntimeRejectsNonEmptyWorkRootTargetDirectory(t *testing.T) {
	// The adapter owns a new child directory; an unrelated file in the parent
	// is allowed and must never be used as the process cwd.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "unrelated"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	starter := &fakeStarter{process: &fakeProcess{stdout: strings.NewReader(successJSONL("ok")), stderr: strings.NewReader("")}}
	runtime, err := New(Options{
		WorkRoot: root, Resolver: fakeResolver{held: heldExecutable()}, Starter: starter,
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := runtime.Dispatch(context.Background(), validSpec())
	if err != nil {
		t.Fatal(err)
	}
	collect(run)
	request := starter.captured()[0]
	if filepath.Clean(request.Workdir) == filepath.Clean(root) {
		t.Fatal("parent work root was used directly")
	}
}
