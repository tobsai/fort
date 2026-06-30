package engine

import (
	"context"
	"path/filepath"
	"testing"

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

	// the dispatched spec carried the task body as prompt
	disp := rt.Dispatched()
	if len(disp) != 1 || disp[0].Prompt != "do the thing" || disp[0].Agent != "codex" {
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
