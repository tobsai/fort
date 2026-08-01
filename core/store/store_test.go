package store

import (
	"database/sql"
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

func TestRunMachineRoundTrip(t *testing.T) {
	s := openTemp(t)
	if err := s.CreateRun(Run{ID: "r1", Title: "t", Agent: "codex", Status: "running", Machine: "macbook-pro"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.GetRun("r1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Machine != "macbook-pro" {
		t.Errorf("machine = %q, want macbook-pro", got.Machine)
	}
}

func TestRunProfileAndModelRoundTrip(t *testing.T) {
	s := openTemp(t)
	want := Run{
		ID: "profiled", Title: "t", Agent: "codex", Status: "running",
		Profile: "codex:gpt-5.6-sol", Model: "gpt-5.6-sol",
	}
	if err := s.CreateRun(want); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.GetRun(want.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Profile != want.Profile || got.Model != want.Model {
		t.Fatalf("profile/model = %q/%q, want %q/%q", got.Profile, got.Model, want.Profile, want.Model)
	}
}

// TestMachineColumnMigratesOldDB proves the additive migration is idempotent:
// a DB whose run table predates the machine column gains it on Open, and rows
// written before the column read back with an empty machine.
func TestMachineColumnMigratesOldDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")

	// Simulate a pre-022 database: run table without the machine column.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE run (
	  id TEXT PRIMARY KEY, title TEXT, agent TEXT, status TEXT, matched_rule TEXT,
	  flow_id TEXT, exit_code INTEGER, error TEXT, created_at TEXT, updated_at TEXT
	);
	INSERT INTO run(id,title,agent,status,matched_rule,flow_id,exit_code,error,created_at,updated_at)
	VALUES('legacy','old run','claude','succeeded','','',0,'','2020-01-01T00:00:00Z','2020-01-01T00:00:00Z');`)
	if err != nil {
		t.Fatalf("seed old db: %v", err)
	}
	db.Close()

	// Open through the store: migration must add the column without dropping the row.
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open (migrate): %v", err)
	}
	defer s.Close()

	got, err := s.GetRun("legacy")
	if err != nil {
		t.Fatalf("get legacy: %v", err)
	}
	if got.Title != "old run" || got.Machine != "" || got.Profile != "" || got.Model != "" {
		t.Errorf("legacy row = %+v, want title 'old run' + empty machine/profile/model", got)
	}
	// A second Open must be a no-op (idempotent) and still succeed.
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	s2.Close()
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

func TestDecideWaitingGateIsSingleFlight(t *testing.T) {
	s := openTemp(t)
	if err := s.CreateRun(Run{ID: "single-flight", Status: "blocked"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertNodeRun(NodeRun{
		ID: "single-flight:plan", RunID: "single-flight", NodeID: "plan", Type: "gate",
		Status: "waiting", Input: "draft",
	}); err != nil {
		t.Fatal(err)
	}

	accepted, err := s.DecideWaitingGate("single-flight:plan", "approved", "approved draft")
	if err != nil || !accepted {
		t.Fatalf("first decision = %v, %v", accepted, err)
	}
	accepted, err = s.DecideWaitingGate("single-flight:plan", "approved", "duplicate")
	if err != nil || accepted {
		t.Fatalf("duplicate decision = %v, %v", accepted, err)
	}
	nodes, err := s.NodeRuns("single-flight")
	if err != nil || len(nodes) != 1 || nodes[0].Status != "approved" || nodes[0].Output != "approved draft" {
		t.Fatalf("node after decisions = %+v, %v", nodes, err)
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

func TestRunBodyRoundTrip(t *testing.T) {
	s := openTemp(t)
	if err := s.CreateRun(Run{ID: "rb1", Title: "t", Body: "# body\nline two", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetRun("rb1")
	if err != nil || got.Body != "# body\nline two" {
		t.Fatalf("body = %q err=%v", got.Body, err)
	}
	// runs created without a body read back empty (NULL-safe)
	_ = s.CreateRun(Run{ID: "rb2", Title: "t2", Status: "queued"})
	if got2, _ := s.GetRun("rb2"); got2.Body != "" {
		t.Fatalf("empty body = %q", got2.Body)
	}
}

func TestEventNodeIDRoundTrip(t *testing.T) {
	s := openTemp(t)
	if err := s.CreateRun(Run{ID: "r1", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendEvent(Event{RunID: "r1", NodeID: "implement", Type: "message", Data: "hi"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendEvent(Event{RunID: "r1", Type: "stdout", Data: "raw"}); err != nil {
		t.Fatal(err)
	}
	evs, err := s.Events("r1")
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("events = %d, want 2", len(evs))
	}
	if evs[0].NodeID != "implement" {
		t.Errorf("evs[0].NodeID = %q, want implement", evs[0].NodeID)
	}
	if evs[1].NodeID != "" {
		t.Errorf("evs[1].NodeID = %q, want empty (node-less event)", evs[1].NodeID)
	}
}

func TestAllNodeRuns(t *testing.T) {
	s := openTemp(t)
	for _, n := range []NodeRun{
		{ID: "r2:b", RunID: "r2", NodeID: "b", Type: "gate", Status: "waiting"},
		{ID: "r1:a", RunID: "r1", NodeID: "a", Type: "task", Status: "succeeded"},
		{ID: "r1:g", RunID: "r1", NodeID: "g", Type: "gate", Status: "approved"},
	} {
		if err := s.UpsertNodeRun(n); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	got, err := s.AllNodeRuns()
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	// grouped by run: both r1 rows must be adjacent
	var order []string
	for _, n := range got {
		order = append(order, n.RunID)
	}
	if !(order[0] == order[1] || order[1] == order[2]) {
		t.Errorf("rows not grouped by run: %v", order)
	}
}
