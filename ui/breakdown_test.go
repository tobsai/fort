package ui_test

import (
	"context"
	"testing"

	"github.com/tobsai/fort/ui"
)

type stubPlanner struct{ lastGoal, lastAgent string }

func (p *stubPlanner) Breakdown(_ context.Context, goal, agent, machine string) (string, error) {
	p.lastGoal, p.lastAgent = goal, agent
	return "planner-run-1", nil
}

func TestBreakdownReturnsRunID(t *testing.T) {
	st := openStore(t)
	sp := &stubPlanner{}
	s := ui.New(ui.Deps{Dispatcher: &capturingDispatcher{}, Store: st, Planner: sp})
	rec := do(t, s, "POST", "/api/breakdown", ui.BreakdownRequest{Text: "build a thing", Agent: "codex"})
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	res := decode[ui.BreakdownResult](t, rec)
	if res.RunID != "planner-run-1" || sp.lastGoal != "build a thing" || sp.lastAgent != "codex" {
		t.Fatalf("res=%+v planner=%+v", res, sp)
	}
}

func TestBreakdownRequiresText(t *testing.T) {
	st := openStore(t)
	s := ui.New(ui.Deps{Dispatcher: &capturingDispatcher{}, Store: st, Planner: &stubPlanner{}})
	if do(t, s, "POST", "/api/breakdown", ui.BreakdownRequest{Text: "  "}).Code != 400 {
		t.Fatal("blank text should 400")
	}
}

func TestBreakdown409WithoutPlanner(t *testing.T) {
	s, _ := newControlUI(t) // no Planner wired
	if do(t, s, "POST", "/api/breakdown", ui.BreakdownRequest{Text: "x"}).Code != 409 {
		t.Fatal("no planner should 409")
	}
}
