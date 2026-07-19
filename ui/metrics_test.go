package ui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/ui"
)

// seedMetricsFixture builds a small but full history: three claude lane runs,
// a flow run whose spec task (claude) had its gate rejected-then-approved and
// whose implement task (codex) passed its gate first-try, plus one parsed-cost
// stdout row for the claude node.
func seedMetricsFixture(t *testing.T, st *store.Store) {
	t.Helper()
	n := time.Now().UTC()
	day := 24 * time.Hour
	for i, id := range []string{"r1", "r2", "r3"} {
		if err := st.CreateRun(store.Run{ID: id, Title: "feature " + id, Agent: "claude",
			MatchedRule: "lane-feature", Status: "succeeded", CreatedAt: n.Add(-time.Duration(i+2) * day)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.CreateRun(store.Run{ID: "rf", Title: "Ship a Feature", Agent: "flow:ship-feature",
		FlowID: "ship-feature", Status: "blocked", CreatedAt: n.Add(-1 * day)}); err != nil {
		t.Fatal(err)
	}
	evs := []store.Event{
		{RunID: "rf", NodeID: "spec", Type: "started", Data: "claude", CreatedAt: n.Add(-25 * time.Hour)},
		{RunID: "rf", NodeID: "spec", Type: "stdout", Data: `{"type":"result","total_cost_usd":1.5}`, CreatedAt: n.Add(-24 * time.Hour)},
		{RunID: "rf", NodeID: "plan_gate", Type: "gate", Data: `{"decision":"rejected","note":"tighten"}`, CreatedAt: n.Add(-23 * time.Hour)},
		{RunID: "rf", NodeID: "plan_gate", Type: "gate", Data: `{"decision":"approved","note":""}`, CreatedAt: n.Add(-22 * time.Hour)},
		{RunID: "rf", NodeID: "implement", Type: "started", Data: "codex", CreatedAt: n.Add(-21 * time.Hour)},
		{RunID: "rf", NodeID: "merge_gate", Type: "gate", Data: `{"decision":"approved","note":""}`, CreatedAt: n.Add(-20 * time.Hour)},
	}
	for _, e := range evs {
		if _, err := st.AppendEvent(e); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMetricsRollups(t *testing.T) {
	s, st := newControlUI(t)
	seedMetricsFixture(t, st)

	rec := do(t, s, "GET", "/api/metrics", nil)
	if rec.Code != 200 {
		t.Fatalf("code %d: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "null") {
		t.Fatalf("metrics body contains null: %s", rec.Body)
	}
	m := decode[ui.MetricsResponse](t, rec)
	if m.WindowDays != 30 {
		t.Errorf("window = %d, want 30", m.WindowDays)
	}
	if len(m.Agents) != 2 || m.Agents[0].Agent != "claude" {
		t.Fatalf("agents = %+v, want claude first then codex", m.Agents)
	}
	cl, cx := m.Agents[0], m.Agents[1]

	// claude: 3 lane runs + the spec node execution
	if cl.Assignments != 4 {
		t.Errorf("claude assignments = %d, want 4", cl.Assignments)
	}
	// plan_gate: rejected then approved => decided 1, first-pass 0, accepted 1, 1 redirect
	if cl.Decided != 1 || cl.FirstPass != 0 || cl.FirstPassPct != 0 || cl.Accepted != 1 || cl.Redirects != 1 {
		t.Errorf("claude gate stats = %+v", cl)
	}
	if !cl.CostKnown || cl.CostUSD != 1.5 || cl.CostPerAccept != 1.5 {
		t.Errorf("claude cost = %+v", cl)
	}
	if len(cl.Spark) != 7 {
		t.Errorf("spark len = %d, want 7", len(cl.Spark))
	}
	joined := strings.Join(cl.Best, "|")
	if !strings.Contains(joined, "lane feature") {
		t.Errorf("claude best = %v, want lane feature", cl.Best)
	}

	// codex: one node execution, clean first-pass approval, no cost data
	if cx.Agent != "codex" || cx.Assignments != 1 || cx.Decided != 1 || cx.FirstPass != 1 || cx.FirstPassPct != 100 {
		t.Errorf("codex = %+v", cx)
	}
	if cx.CostKnown || cx.CostPerAccept != 0 {
		t.Errorf("codex cost should be unknown: %+v", cx)
	}

	if m.Assignments != 5 {
		t.Errorf("total assignments = %d, want 5", m.Assignments)
	}
	if !contains(m.Lanes, "lane-feature") {
		t.Errorf("lanes = %v, want lane-feature", m.Lanes)
	}
}

func TestMetricsLaneFilter(t *testing.T) {
	s, st := newControlUI(t)
	seedMetricsFixture(t, st)
	m := decode[ui.MetricsResponse](t, do(t, s, "GET", "/api/metrics?lane=lane-feature", nil))
	// flow-node work carries no lane, so only the three claude lane runs remain
	if len(m.Agents) != 1 || m.Agents[0].Agent != "claude" || m.Agents[0].Assignments != 3 || m.Agents[0].Decided != 0 {
		t.Fatalf("filtered = %+v, want claude with 3 assignments and no sign-off data", m.Agents)
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
