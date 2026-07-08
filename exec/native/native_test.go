package native

import (
	"context"
	"strings"
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

func TestSignalInjectsStdin(t *testing.T) {
	rt := New(t.TempDir(), shProvider("reader", "read x; echo got:$x"))
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
func TestDefaultProvidersArgv(t *testing.T) {
	rt := New(t.TempDir(), DefaultProviders()...)
	cases := map[string][]string{
		"codex":    {"codex", "exec"},
		"claude":   {"claude", "-p"},
		"hermes":   {"hermes", "--oneshot"},
		"openclaw": {"openclaw", "run"}, // best-guess argv, spec 023
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
