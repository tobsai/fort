package engine

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobsai/fort/core/requestid"
	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/exec/fake"
)

const ruleset = `
version: 1
defaults:
  route: claude
rules:
  - id: dev-lane
    when:
      label: [dev, feature]
    route: codex
`

func newEngine(t *testing.T) (*Engine, *store.Store, *fake.Runtime) {
	t.Helper()
	rs, err := rules.Parse([]byte(ruleset))
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	rt := fake.New()
	e := New(router.New(rs), rt, st, t.TempDir())
	return e, st, rt
}

func TestSubmitPersistsIngressRequestIDBeforeDispatch(t *testing.T) {
	e, st, _ := newEngine(t)
	const id = "018f3f1c-7d3a-7c1d-a176-9c52c606c6e4"
	run, err := e.SubmitAndWait(requestid.With(context.Background(), id), task.Task{ID: "trace", Title: "trace"})
	if err != nil {
		t.Fatal(err)
	}
	events, err := st.Events(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 || events[0].Type != "ingress" || events[0].Data != `{"request_id":"`+id+`"}` {
		t.Fatalf("events=%+v", events)
	}
}

func TestSubmitRoutesPersistsAndRuns(t *testing.T) {
	e, st, rt := newEngine(t)
	run, err := e.SubmitAndWait(context.Background(), task.Task{ID: "t1", Title: "add feature", Body: "do the thing", Labels: []string{"feature"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if run.Agent != "codex" {
		t.Errorf("agent = %q, want codex", run.Agent)
	}
	if run.Status != "succeeded" {
		t.Errorf("status = %q, want succeeded", run.Status)
	}
	if run.MatchedRule != "dev-lane" {
		t.Errorf("matched rule = %q, want dev-lane", run.MatchedRule)
	}

	// route_decision persisted
	decs, _ := st.RouteDecisions("t1")
	if len(decs) != 1 || decs[0].Route != "codex" {
		t.Errorf("route decisions = %+v", decs)
	}

	// the dispatched spec carried the full task definition (title + body) as prompt
	disp := rt.Dispatched()
	if len(disp) != 1 || disp[0].Prompt != "add feature\n\ndo the thing" || disp[0].Agent != "codex" {
		t.Errorf("dispatched = %+v", disp)
	}

	// events streamed into the store (started ... exited)
	evs, _ := st.Events(run.ID)
	if len(evs) < 2 || evs[0].Type != "started" || evs[len(evs)-1].Type != "exited" {
		t.Errorf("events = %+v", evs)
	}
}

func TestSubmitDefaultsWhenNoRuleMatches(t *testing.T) {
	e, _, _ := newEngine(t)
	run, err := e.SubmitAndWait(context.Background(), task.Task{ID: "t2", Title: "chat", Labels: []string{"random"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if run.Agent != "claude" {
		t.Errorf("agent = %q, want claude (default)", run.Agent)
	}
}

// stubPlacer records the arguments it is asked to place and returns a fixed
// answer (or an error), standing in for core/machines.Registry.
type stubPlacer struct {
	machine  string
	err      error
	gotAgent string
	gotPin   string
}

func (p *stubPlacer) Place(agent, pin string) (string, error) {
	p.gotAgent, p.gotPin = agent, pin
	return p.machine, p.err
}

func TestPlacementRecordsMachineAndStampsSpec(t *testing.T) {
	e, _, rt := newEngine(t)
	p := &stubPlacer{machine: "macbook-pro"}
	e.UsePlacer(p)

	run, err := e.SubmitAndWait(context.Background(), task.Task{
		ID: "t4", Title: "build", Labels: []string{"feature"}, Machine: "macbook-pro",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	// Placement was asked about the routed agent + the task's machine pin.
	if p.gotAgent != "codex" || p.gotPin != "macbook-pro" {
		t.Errorf("place args = (%q,%q), want (codex,macbook-pro)", p.gotAgent, p.gotPin)
	}
	if run.Machine != "macbook-pro" {
		t.Errorf("run machine = %q, want macbook-pro", run.Machine)
	}
	disp := rt.Dispatched()
	if len(disp) != 1 || disp[0].Machine != "macbook-pro" {
		t.Errorf("dispatched spec machine = %+v", disp)
	}
}

func TestPlacementErrorBoardsFailedRun(t *testing.T) {
	e, st, rt := newEngine(t)
	e.UsePlacer(&stubPlacer{err: context.DeadlineExceeded})

	runID, err := e.Submit(context.Background(), task.Task{ID: "t5", Title: "x", Labels: []string{"feature"}})
	if err == nil {
		t.Fatal("expected placement error")
	}
	// No dispatch happened, but the failure is boarded for visibility.
	if len(rt.Dispatched()) != 0 {
		t.Errorf("expected no dispatch on placement failure")
	}
	got, gerr := st.GetRun(runID)
	if gerr != nil || got.Status != "failed" || got.Error == "" || got.ExitCode != -1 {
		t.Errorf("boarded run = %+v (err %v), want failed/-1 with error", got, gerr)
	}
}

func TestNoPlacerLeavesMachineEmpty(t *testing.T) {
	e, _, rt := newEngine(t) // no UsePlacer
	run, err := e.SubmitAndWait(context.Background(), task.Task{ID: "t6", Title: "x", Labels: []string{"feature"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if run.Machine != "" {
		t.Errorf("machine = %q, want empty (single-machine)", run.Machine)
	}
	if d := rt.Dispatched(); len(d) != 1 || d[0].Machine != "" {
		t.Errorf("dispatched spec machine = %+v, want empty", d)
	}
}

func TestSubmitCopiesBodyToRun(t *testing.T) {
	e, st, _ := newEngine(t)
	runID, err := e.Submit(context.Background(), task.Task{ID: "tb", Title: "title line", Body: "body md"})
	if err != nil {
		t.Fatal(err)
	}
	e.Wait(runID)
	r, _ := st.GetRun(runID)
	if r.Body != "body md" {
		t.Fatalf("run body = %q, want body md", r.Body)
	}
}

func TestPromptCombinesTitleAndBody(t *testing.T) {
	e, st, _ := newEngine(t)
	run, err := e.SubmitAndWait(context.Background(), task.Task{ID: "tp", Title: "do the thing", Body: "with details"})
	if err != nil {
		t.Fatal(err)
	}
	// The fake echoes the dispatched prompt as a message event; the agent must
	// see the full task definition (title line + body), not the body alone.
	evs, _ := st.Events(run.ID)
	var msg string
	for _, ev := range evs {
		if ev.Type == "message" {
			msg = ev.Data
		}
	}
	if !strings.Contains(msg, "do the thing") || !strings.Contains(msg, "with details") {
		t.Fatalf("prompt message = %q, want both title and body", msg)
	}
}

func TestBodylessTaskPromptIsTitleOnly(t *testing.T) {
	e, _, rt := newEngine(t)
	run, err := e.SubmitAndWait(context.Background(), task.Task{ID: "tt", Title: "fix it"})
	if err != nil {
		t.Fatal(err)
	}
	// No body: the prompt is exactly the title — no "\n\n" separator glued on.
	if d := rt.Dispatched(); len(d) != 1 || d[0].Prompt != "fix it" {
		t.Fatalf("dispatched = %+v, want prompt exactly %q", d, "fix it")
	}
	if run.Body != "" {
		t.Fatalf("run body = %q, want empty", run.Body)
	}
}

func TestSubmitFailureRecordsFailedStatus(t *testing.T) {
	e, _, rt := newEngine(t)
	rt.ExitCode = 1
	run, err := e.SubmitAndWait(context.Background(), task.Task{ID: "t3", Title: "x", Labels: []string{"feature"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if run.Status != "failed" || run.ExitCode != 1 {
		t.Errorf("run = %+v, want failed/1", run)
	}
}
