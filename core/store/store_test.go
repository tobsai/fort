package store

import (
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRouteDecisionRoundTrip(t *testing.T) {
	s := openTemp(t)
	d := RouteDecision{ID: "d1", TaskID: "t1", Route: "codex", MatchedRule: "dev-lane", Reason: "matched"}
	if err := s.SaveRouteDecision(d); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.RouteDecisions("t1")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 1 || got[0].Route != "codex" || got[0].MatchedRule != "dev-lane" {
		t.Errorf("got %+v", got)
	}
}

func TestRunLifecycle(t *testing.T) {
	s := openTemp(t)
	r := Run{ID: "run1", Title: "ship it", Agent: "codex", Status: "running", MatchedRule: "dev-lane"}
	if err := s.CreateRun(r); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.UpdateRunStatus("run1", "succeeded", 0, ""); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := s.GetRun("run1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "succeeded" || got.Agent != "codex" {
		t.Errorf("got %+v", got)
	}
	runs, err := s.ListRuns()
	if err != nil || len(runs) != 1 {
		t.Fatalf("list: %v / %d", err, len(runs))
	}
}

func TestEventsAppendOnlyAndOrdered(t *testing.T) {
	s := openTemp(t)
	_ = s.CreateRun(Run{ID: "run1", Agent: "codex", Status: "running"})
	id1, err := s.AppendEvent(Event{RunID: "run1", Type: "started", Data: "codex"})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	id2, _ := s.AppendEvent(Event{RunID: "run1", Type: "message", Data: "hello"})
	id3, _ := s.AppendEvent(Event{RunID: "run1", Type: "exited", Code: 0})
	if !(id1 < id2 && id2 < id3) {
		t.Errorf("event ids not increasing: %d %d %d", id1, id2, id3)
	}
	evs, err := s.Events("run1")
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	if len(evs) != 3 || evs[0].Type != "started" || evs[2].Type != "exited" {
		t.Errorf("events = %+v", evs)
	}
	// EventsSince powers the UI feed (events after a cursor).
	since, _ := s.EventsSince(id1)
	if len(since) != 2 || since[0].Type != "message" {
		t.Errorf("EventsSince(%d) = %+v", id1, since)
	}
}

func TestWaitingGates(t *testing.T) {
	s := openTemp(t)
	_ = s.CreateRun(Run{ID: "run1", Status: "blocked", FlowID: "ship"})
	_ = s.UpsertNodeRun(NodeRun{ID: "run1:g1", RunID: "run1", NodeID: "g1", Type: "gate", Status: "waiting"})
	_ = s.UpsertNodeRun(NodeRun{ID: "run1:t1", RunID: "run1", NodeID: "t1", Type: "task", Status: "succeeded"})

	gates, err := s.WaitingGates()
	if err != nil {
		t.Fatalf("waiting gates: %v", err)
	}
	if len(gates) != 1 || gates[0].NodeID != "g1" {
		t.Fatalf("waiting gates = %+v, want [g1]", gates)
	}
}

func TestNodeRunUpsert(t *testing.T) {
	s := openTemp(t)
	_ = s.CreateRun(Run{ID: "run1", Agent: "codex", Status: "running"})
	n := NodeRun{ID: "run1:spec", RunID: "run1", NodeID: "spec", Type: "task", Status: "running"}
	if err := s.UpsertNodeRun(n); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	n.Status = "succeeded"
	n.Output = "done"
	if err := s.UpsertNodeRun(n); err != nil {
		t.Fatalf("upsert2: %v", err)
	}
	got, err := s.NodeRuns("run1")
	if err != nil || len(got) != 1 {
		t.Fatalf("noderuns: %v / %d", err, len(got))
	}
	if got[0].Status != "succeeded" || got[0].Output != "done" {
		t.Errorf("node = %+v", got[0])
	}
}
