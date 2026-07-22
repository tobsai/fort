package ui_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/ui"
)

func TestQuickAnswerIsHistoryOnly(t *testing.T) {
	s, st := newControlUI(t)
	now := time.Now().UTC()
	runs := []store.Run{
		{
			ID: "answer-run", Title: "What changed?", Agent: "flow:playbook:quick-answer",
			FlowID: "playbook:quick-answer:1:question:gate0:answer", Status: "succeeded", CreatedAt: now,
		},
		{
			ID: "assignment-run", Title: "Build the thing", Agent: "flow:playbook:standard-delivery",
			FlowID: "playbook:standard-delivery:1:feature:gate1:assignment", Status: "succeeded", CreatedAt: now.Add(-time.Minute),
		},
	}
	for _, run := range runs {
		if err := st.CreateRun(run); err != nil {
			t.Fatal(err)
		}
	}
	for _, event := range []store.Event{
		{RunID: "answer-run", NodeID: "stage-1", Type: "started", Data: "hermes", CreatedAt: now},
		{RunID: "assignment-run", NodeID: "stage-1", Type: "started", Data: "codex", CreatedAt: now},
	} {
		if _, err := st.AppendEvent(event); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.UpsertNodeRun(store.NodeRun{
		ID: "answer-run:plan-gate", RunID: "answer-run", NodeID: "plan-gate", Type: "gate", Status: "waiting",
	}); err != nil {
		t.Fatal(err)
	}

	board := decode[ui.Board](t, do(t, s, http.MethodGet, "/api/board", nil))
	if len(board.Runs) != 1 || board.Runs[0].ID != "assignment-run" {
		t.Fatalf("board runs = %+v, want only assignment-run", board.Runs)
	}
	if len(board.Gates) != 0 {
		t.Fatalf("board gates = %+v, answer history must not leak a checkpoint", board.Gates)
	}
	summary := decode[ui.Summary](t, do(t, s, http.MethodGet, "/api/summary", nil))
	if summary.Total != 1 || summary.Succeeded != 1 {
		t.Fatalf("summary = %+v, want only assignment run counted", summary)
	}
	if len(summary.Gates) != 0 {
		t.Fatalf("summary gates = %+v, answer history must not leak a checkpoint", summary.Gates)
	}
	gates := decode[[]ui.GateItem](t, do(t, s, http.MethodGet, "/api/gates", nil))
	if len(gates) != 0 {
		t.Fatalf("gate inbox = %+v, answer history must not leak a checkpoint", gates)
	}
	metrics := decode[ui.MetricsResponse](t, do(t, s, http.MethodGet, "/api/metrics", nil))
	if metrics.Assignments != 1 || len(metrics.Agents) != 1 || metrics.Agents[0].Agent != "codex" {
		t.Fatalf("metrics = %+v, want only codex assignment", metrics)
	}

	detail := do(t, s, http.MethodGet, "/api/runs/answer-run", nil)
	if detail.Code != http.StatusOK {
		t.Fatalf("answer history status = %d, want 200", detail.Code)
	}
	got := decode[ui.RunDetail](t, detail)
	if got.Run.ID != "answer-run" {
		t.Fatalf("answer history run = %+v", got.Run)
	}
}
