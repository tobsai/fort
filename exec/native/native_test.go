package native

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/tobsai/fort/core/runtime"
)

// shProvider runs an arbitrary shell snippet (argv ignores the prompt).
func shProvider(name, script string) Provider {
	return Provider{
		Name:    name,
		Command: func(_ runtime.RunSpec) []string { return []string{"sh", "-c", script} },
	}
}

func collect(r runtime.Run) (evs []runtime.RunEvent, st runtime.Status) {
	for e := range r.Stream() {
		evs = append(evs, e)
	}
	return evs, r.Wait()
}

func lines(evs []runtime.RunEvent, typ runtime.EventType) []string {
	var out []string
	for _, e := range evs {
		if e.Type == typ {
			out = append(out, e.Data)
		}
	}
	return out
}

func TestExecStreamsStdoutAndExitsZero(t *testing.T) {
	rt := New(t.TempDir(), shProvider("echoer", "echo hello; echo world"))
	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r1", Agent: "echoer"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	evs, st := collect(run)
	if evs[0].Type != runtime.EventStarted {
		t.Errorf("first = %q, want started", evs[0].Type)
	}
	if got := lines(evs, runtime.EventStdout); len(got) != 2 || got[0] != "hello" || got[1] != "world" {
		t.Errorf("stdout = %v", got)
	}
	last := evs[len(evs)-1]
	if last.Type != runtime.EventExited || last.Code != 0 {
		t.Errorf("last = %+v, want exited 0", last)
	}
	if st.State != runtime.StateSucceeded {
		t.Errorf("state = %v, want succeeded", st.State)
	}
}

func TestExecNonZeroExitFails(t *testing.T) {
	rt := New(t.TempDir(), shProvider("failer", "exit 3"))
	run, _ := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r2", Agent: "failer"})
	_, st := collect(run)
	if st.State != runtime.StateFailed || st.ExitCode != 3 {
		t.Errorf("status = %+v, want failed/3", st)
	}
}

func TestProviderFatalOutputFailsEvenWhenProcessExitsZero(t *testing.T) {
	p := shProvider("false-success", "echo 'API call failed after 3 retries: timed out'")
	p.Failure = func(line string) (string, bool) {
		return line, strings.HasPrefix(line, "API call failed after ")
	}
	rt := New(t.TempDir(), p)
	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "fatal-output", Agent: p.Name})
	if err != nil {
		t.Fatal(err)
	}
	evs, st := collect(run)
	if st.State != runtime.StateFailed || st.ExitCode != 1 ||
		!strings.Contains(st.Err, "API call failed after 3 retries") {
		t.Fatalf("status = %+v, want failed/1 with the provider failure", st)
	}
	if got := lines(evs, runtime.EventError); len(got) != 1 ||
		!strings.Contains(got[0], "API call failed after 3 retries") {
		t.Fatalf("error events = %v, want the provider failure", got)
	}
	last := evs[len(evs)-1]
	if last.Type != runtime.EventExited || last.Code != 1 {
		t.Fatalf("last event = %+v, want exited/1", last)
	}
}

func TestExecParseNormalizesMessages(t *testing.T) {
	p := shProvider("claudish", `echo 'MSG:thinking'; echo 'plain line'`)
	p.Parse = func(line string) (string, bool) {
		if s, ok := strings.CutPrefix(line, "MSG:"); ok {
			return s, true
		}
		return "", false
	}
	rt := New(t.TempDir(), p)
	run, _ := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r3", Agent: "claudish"})
	evs, _ := collect(run)
	if msgs := lines(evs, runtime.EventMessage); len(msgs) != 1 || msgs[0] != "thinking" {
		t.Errorf("messages = %v, want [thinking]", msgs)
	}
	if std := lines(evs, runtime.EventStdout); len(std) != 1 || std[0] != "plain line" {
		t.Errorf("stdout = %v, want [plain line]", std)
	}
}

// TestNonInteractiveProviderGetsEOFOnStdin pins the hang fix. Fort used to hand
// every child an open stdin pipe (for Signal) and never close it, so a CLI that
// drains stdin blocked forever. Live symptom: `codex exec` printed "Reading
// additional input from stdin..." and the run never terminated. A provider that
// does not opt into Interactive must get /dev/null (immediate EOF).
func TestNonInteractiveProviderGetsEOFOnStdin(t *testing.T) {
	// `cat` reads stdin to EOF; with an open pipe it would hang until the test
	// deadline, with /dev/null it exits at once.
	rt := New(t.TempDir(), shProvider("drainer", "cat; echo drained"))
	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "eof1", Agent: "drainer"})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan runtime.Status, 1)
	go func() { _, st := collect(run); done <- st }()
	select {
	case st := <-done:
		if st.State != runtime.StateSucceeded {
			t.Fatalf("state = %v, want succeeded", st.State)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run hung: child never saw EOF on stdin")
	}
}

// TestSignalOnNonInteractiveErrors: Signal is meaningful only for a provider that
// asked for a stdin pipe; otherwise it must fail loudly rather than silently drop.
func TestSignalOnNonInteractiveErrors(t *testing.T) {
	rt := New(t.TempDir(), shProvider("quiet", "sleep 0.2"))
	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "sig0", Agent: "quiet"})
	if err != nil {
		t.Fatal(err)
	}
	if err := run.Signal("yo"); err == nil {
		t.Error("Signal on a non-interactive provider must return an error")
	}
	collect(run)
}

func TestSignalInjectsStdin(t *testing.T) {
	p := shProvider("reader", "read x; echo got:$x")
	p.Interactive = true // opt in to a stdin pipe (HITL)
	rt := New(t.TempDir(), p)
	run, _ := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r4", Agent: "reader"})
	// Give the process a moment to reach `read`, then inject.
	time.Sleep(50 * time.Millisecond)
	if err := run.Signal("yo"); err != nil {
		t.Fatalf("signal: %v", err)
	}
	evs, st := collect(run)
	if got := lines(evs, runtime.EventStdout); len(got) != 1 || got[0] != "got:yo" {
		t.Errorf("stdout = %v, want [got:yo]", got)
	}
	if st.State != runtime.StateSucceeded {
		t.Errorf("state = %v", st.State)
	}
}

func TestCancelTerminatesRun(t *testing.T) {
	rt := New(t.TempDir(), shProvider("sleeper", "sleep 30"))
	run, _ := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r5", Agent: "sleeper"})
	time.Sleep(50 * time.Millisecond)
	if err := run.Cancel(); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	st := run.Wait()
	if st.State != runtime.StateCanceled {
		t.Errorf("state = %v, want canceled", st.State)
	}
}

func TestCancelKillsDetachedDescendantAfterBothStreamsClose(t *testing.T) {
	stateDir := t.TempDir()
	childPIDPath := filepath.Join(stateDir, "child.pid")
	script := fmt.Sprintf(
		`(exec >/dev/null 2>&1; sleep 30) & child=$!; printf '%%s\n' "$child" > %q; exec >/dev/null 2>&1; sleep 30`,
		childPIDPath,
	)
	rt := New(t.TempDir(), shProvider("detached-descendant", script))
	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "detached", Agent: "detached-descendant"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	native := run.(*nativeRun)

	var childPID int
	deadline := time.Now().Add(2 * time.Second)
	for childPID == 0 {
		data, readErr := os.ReadFile(childPIDPath)
		if readErr == nil {
			childPID, err = strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Fatalf("parse child pid %q: %v", data, err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("descendant pid was not published: %v", readErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() { _ = syscall.Kill(childPID, syscall.SIGKILL) })

	native.mu.Lock()
	leaderPGID := native.pgid
	native.mu.Unlock()
	if got, getErr := syscall.Getpgid(childPID); getErr != nil || got != leaderPGID {
		t.Fatalf("descendant pgid = %d, %v; want leader group %d", got, getErr, leaderPGID)
	}

	select {
	case <-native.streamsEOF:
	case <-time.After(2 * time.Second):
		t.Fatal("stdout/stderr did not both reach EOF after the leader redirected them")
	}
	if err := run.Cancel(); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if st := run.Wait(); st.State != runtime.StateCanceled {
		t.Fatalf("status = %+v, want canceled", st)
	}

	deadline = time.Now().Add(2 * time.Second)
	for {
		err := syscall.Kill(childPID, 0)
		if err == syscall.ESRCH {
			break
		}
		if err != nil {
			t.Fatalf("probe descendant %d: %v", childPID, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("detached descendant %d survived cancellation after stream EOF", childPID)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCancelIsIdempotent(t *testing.T) {
	var calls atomic.Int32
	run := &nativeRun{
		cancel: func() { calls.Add(1) },
		pgid:   -1,
		status: runtime.Status{State: runtime.StateRunning},
	}

	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := run.Cancel(); err != nil {
				t.Errorf("Cancel: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := calls.Load(); got != 1 {
		t.Fatalf("underlying cancel calls = %d, want 1", got)
	}
}

func TestCancelAfterTerminalStatusIsNoOp(t *testing.T) {
	var calls atomic.Int32
	run := &nativeRun{
		cancel: func() { calls.Add(1) },
		pgid:   -1,
		status: runtime.Status{State: runtime.StateSucceeded},
	}

	if err := run.Cancel(); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("underlying cancel calls = %d, want 0 for terminal run", got)
	}
	if run.canceled {
		t.Fatal("terminal run was marked canceled")
	}
}

type waitFunc func() error

func (f waitFunc) Wait() error { return f() }

type leaderExitWatcherFunc struct {
	wait  func() error
	close func() error
}

func (w leaderExitWatcherFunc) Wait() error  { return w.wait() }
func (w leaderExitWatcherFunc) Close() error { return w.close() }

func TestPumpCleansProcessGroupBeforeReapingLeader(t *testing.T) {
	leaderExitEntered := make(chan struct{})
	releaseLeaderExit := make(chan struct{})
	waitEntered := make(chan struct{})
	releaseWait := make(chan struct{})
	var killCalls atomic.Int32
	run := &nativeRun{
		spec:       runtime.RunSpec{RunID: "cleanup-before-reap", Agent: "test"},
		events:     make(chan runtime.RunEvent, 4),
		done:       make(chan struct{}),
		streamsEOF: make(chan struct{}),
		cancel:     func() {},
		leaderExit: leaderExitWatcherFunc{
			wait: func() error {
				close(leaderExitEntered)
				<-releaseLeaderExit
				return nil
			},
			close: func() error { return nil },
		},
		pgid: 4242,
		killProcessGroup: func(int) {
			killCalls.Add(1)
		},
		status: runtime.Status{State: runtime.StateRunning},
	}
	waiter := waitFunc(func() error {
		close(waitEntered)
		<-releaseWait
		return nil
	})

	go run.pump(waiter, strings.NewReader(""), strings.NewReader(""))
	select {
	case <-leaderExitEntered:
	case <-time.After(time.Second):
		t.Fatal("pump did not start the non-reaping leader-exit watch")
	}
	select {
	case <-run.streamsEOF:
	case <-time.After(time.Second):
		t.Fatal("empty output streams did not reach EOF")
	}
	select {
	case <-waitEntered:
		t.Fatal("cmd.Wait began before leader exit was observed")
	default:
	}

	close(releaseLeaderExit)
	select {
	case <-waitEntered:
	case <-time.After(time.Second):
		t.Fatal("pump did not proceed to cmd.Wait after leader exit")
	}
	if got := killCalls.Load(); got != 1 {
		t.Fatalf("process-group cleanup calls = %d, want 1 before cmd.Wait", got)
	}

	close(releaseWait)
	for range run.Stream() {
	}
	if st := run.Wait(); st.State != runtime.StateSucceeded {
		t.Fatalf("status = %+v, want succeeded", st)
	}
}

func TestUnknownAgentErrors(t *testing.T) {
	rt := New(t.TempDir())
	_, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "r6", Agent: "nope"})
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestImplementsRuntime(t *testing.T) {
	var _ runtime.Runtime = New(t.TempDir())
}

// The built-in provider argv must match the AO-002 recon contract.
// TestCodexArgvMatchesInstalledCLI pins the codex contract against the real CLI
// (catalog-v2 codex-cli 0.146.0-alpha.9.2, verified 2026-08-03): `codex exec` REJECTS
// --ask-for-approval ("unexpected argument"), and Fort runs agents in a scratch
// workdir that is not a git repo, so --skip-git-repo-check is required.
func TestCodexArgvMatchesInstalledCLI(t *testing.T) {
	var codex Provider
	for _, p := range DefaultProviders() {
		if p.Name == "codex" {
			codex = p
		}
	}
	argv := strings.Join(codex.Command(runtime.RunSpec{Prompt: "x"}), " ")
	for _, want := range []string{"codex exec", "--json", "--sandbox workspace-write", "--skip-git-repo-check"} {
		if !strings.Contains(argv, want) {
			t.Errorf("codex argv missing %q: %s", want, argv)
		}
	}
	if strings.Contains(argv, "--ask-for-approval") {
		t.Errorf("codex exec rejects --ask-for-approval; argv: %s", argv)
	}
}

func TestDefaultProvidersArgv(t *testing.T) {
	rt := New(t.TempDir(), DefaultProviders()...)
	cases := map[string][]string{
		"codex":    {"codex", "exec"},
		"claude":   {"claude", "-p"},
		"hermes":   {"hermes", "--oneshot"},
		"openclaw": {"openclaw", "agent"},
	}
	for agent, wantPrefix := range cases {
		p, ok := rt.provider(agent)
		if !ok {
			t.Fatalf("provider %q not registered", agent)
		}
		argv := p.Command(runtime.RunSpec{Prompt: "do x"})
		for i, w := range wantPrefix {
			if i >= len(argv) || argv[i] != w {
				t.Errorf("%s argv = %v, want prefix %v", agent, argv, wantPrefix)
				break
			}
		}
		// the prompt must appear somewhere in argv
		if !contains(argv, "do x") {
			t.Errorf("%s argv = %v, missing prompt", agent, argv)
		}
	}
}

func TestOpenClawArgvMatchesInstalledCLIContract(t *testing.T) {
	var p Provider
	for _, candidate := range DefaultProviders() {
		if candidate.Name == "openclaw" {
			p = candidate
			break
		}
	}
	got := p.Command(runtime.RunSpec{RunID: "parent-run:design:2", Prompt: "do x"})
	want := []string{
		"openclaw", "agent", "--local", "--agent", "main",
		"--session-id", "parent-run:design:2", "--message", "do x",
		"--thinking", "off", "--timeout", "60", "--json",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("openclaw argv = %v, want %v", got, want)
	}
	if !slices.Equal(p.Probe, []string{"openclaw", "agent", "--help"}) {
		t.Fatalf("openclaw probe = %v, want agent --help contract probe", p.Probe)
	}
}

func TestOpenClawParserExtractsPrettyPrintedJSONPayload(t *testing.T) {
	line := `      "text": "openclaw-local-ok",`
	got, ok := jsonTextParser(line)
	if !ok || got != "openclaw-local-ok" {
		t.Fatalf("jsonTextParser(%q) = %q, %v; want openclaw-local-ok, true", line, got, ok)
	}
}

func TestDefaultProvidersDeclareNonInteractiveContractProbes(t *testing.T) {
	for _, p := range DefaultProviders() {
		if len(p.Probe) < 2 || p.Probe[0] != p.Name {
			t.Errorf("%s probe = %v, want a provider-specific help command", p.Name, p.Probe)
		}
		if p.Probe[len(p.Probe)-1] != "--help" {
			t.Errorf("%s probe = %v, want trailing --help", p.Name, p.Probe)
		}
	}
}

func TestDispatchFailsClosedWhenProviderCommandContractDrifts(t *testing.T) {
	p := Provider{
		Name:  "drifted",
		Probe: []string{"sh", "-c", "echo removed-subcommand >&2; exit 2"},
		Command: func(_ runtime.RunSpec) []string {
			return []string{"sh", "-c", "echo task-should-not-run"}
		},
	}
	rt := New(t.TempDir(), p)
	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "drift", Agent: "drifted"})
	if err == nil || run != nil {
		t.Fatalf("Dispatch = (%v, %v), want preflight failure", run, err)
	}
	for _, want := range []string{`provider "drifted" command contract`, "removed-subcommand"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Dispatch error = %q, want %q", err, want)
		}
	}
}

func TestVerifiedProvidersPassModelOverride(t *testing.T) {
	for _, agent := range []string{"claude", "codex", "hermes"} {
		t.Run(agent, func(t *testing.T) {
			var p Provider
			for _, candidate := range DefaultProviders() {
				if candidate.Name == agent {
					p = candidate
					break
				}
			}
			argv := p.Command(runtime.RunSpec{Prompt: "do x", Model: "model-under-test"})
			if !adjacent(argv, "--model", "model-under-test") {
				t.Fatalf("%s argv = %v, want --model model-under-test", agent, argv)
			}
		})
	}
}

func TestProvidersNormalizeDesignModelLabelsAtCLIContract(t *testing.T) {
	for _, tc := range []struct {
		agent string
		label string
		want  string
	}{
		{agent: "claude", label: "Sonnet", want: "sonnet"},
		{agent: "claude", label: "Opus", want: "opus"},
		{agent: "codex", label: "5.6 Sol", want: "gpt-5.6-sol"},
		{agent: "hermes", label: "Codex 5.6 Sol", want: "openai-codex/gpt-5.6-sol"},
	} {
		t.Run(tc.agent, func(t *testing.T) {
			var p Provider
			for _, candidate := range DefaultProviders() {
				if candidate.Name == tc.agent {
					p = candidate
					break
				}
			}
			argv := p.Command(runtime.RunSpec{Prompt: "do x", Model: tc.label})
			if !adjacent(argv, "--model", tc.want) {
				t.Fatalf("%s argv = %v, want normalized --model %s", tc.agent, argv, tc.want)
			}
		})
	}
}

func TestOpenClawDoesNotInventModelFlag(t *testing.T) {
	var p Provider
	for _, candidate := range DefaultProviders() {
		if candidate.Name == "openclaw" {
			p = candidate
			break
		}
	}
	argv := p.Command(runtime.RunSpec{Prompt: "do x", Model: "Fable"})
	if contains(argv, "--model") || contains(argv, "Fable") {
		t.Fatalf("openclaw model label is not a verified CLI model id: %v", argv)
	}
}

func TestBuiltInProvidersRecognizeObservedTerminalFailures(t *testing.T) {
	cases := []struct {
		agent string
		line  string
		want  string
	}{
		{
			agent: "hermes",
			line:  "API call failed after 3 retries: Non-streaming API call timed out after 90s",
			want:  "API call failed after 3 retries",
		},
		{
			agent: "codex",
			line:  `{"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The requested model requires a newer version of Codex."}}`,
			want:  "The requested model requires a newer version of Codex.",
		},
	}
	providers := map[string]Provider{}
	for _, p := range DefaultProviders() {
		providers[p.Name] = p
	}
	for _, tc := range cases {
		t.Run(tc.agent, func(t *testing.T) {
			p := providers[tc.agent]
			if p.Failure == nil {
				t.Fatalf("%s has no terminal failure recognizer", tc.agent)
			}
			got, ok := p.Failure(tc.line)
			if !ok || !strings.Contains(got, tc.want) {
				t.Fatalf("Failure(%q) = %q, %v; want text containing %q", tc.line, got, ok, tc.want)
			}
		})
	}
}

func adjacent(ss []string, a, b string) bool {
	for i := 0; i+1 < len(ss); i++ {
		if ss[i] == a && ss[i+1] == b {
			return true
		}
	}
	return false
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func TestClassifyEmitsTypedEvents(t *testing.T) {
	// A provider with Classify turns one stdout line into N typed events;
	// unclassified lines fall through to raw stdout.
	p := Provider{
		Name:    "clsfy",
		Command: func(_ runtime.RunSpec) []string { return []string{"sh", "-c", `printf 'KNOWN\nnoise\n'`} },
		Classify: func(line string) ([]Classified, bool) {
			if line == "KNOWN" {
				return []Classified{
					{Type: runtime.EventTool, Data: `{"name":"Read"}`},
					{Type: runtime.EventSubagent, Data: `{"description":"sub"}`},
				}, true
			}
			return nil, false
		},
	}
	rt := New(t.TempDir(), p)
	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "c1", Agent: "clsfy"})
	if err != nil {
		t.Fatal(err)
	}
	var got []runtime.RunEvent
	for ev := range run.Stream() {
		got = append(got, ev)
	}
	var tools, subs, stdouts int
	for _, e := range got {
		switch e.Type {
		case runtime.EventTool:
			tools++
		case runtime.EventSubagent:
			subs++
		case runtime.EventStdout:
			if e.Data == "noise" {
				stdouts++
			}
		}
	}
	if tools != 1 || subs != 1 || stdouts != 1 {
		t.Fatalf("tools=%d subs=%d stdouts=%d (want 1,1,1); events=%+v", tools, subs, stdouts, got)
	}
}
