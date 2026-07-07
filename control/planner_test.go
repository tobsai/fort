package control

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobsai/fort/core/engine"
	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/exec/fake"
)

const plannerRules = "version: 1\ndefaults:\n  route: claude\nrules:\n  - id: explicit-claude\n    when:\n      agent: claude\n    route: claude\n  - id: explicit-codex\n    when:\n      agent: codex\n    route: codex\n"

func planStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// resultEvent is a claude-style terminal stream-json result line (stored raw).
func resultEvent(payload string) store.Event {
	return store.Event{Type: "stdout", Data: `{"type":"result","subtype":"success","result":` + payload + `}`}
}

// resultEventRaw builds a result line whose "result" is the given raw text
// (JSON-escaped for us), so tests can use literal backticks/quotes freely.
func resultEventRaw(result string) store.Event {
	b, _ := json.Marshal(struct {
		Type   string `json:"type"`
		Result string `json:"result"`
	}{Type: "result", Result: result})
	return store.Event{Type: "stdout", Data: string(b)}
}

func TestParsePlanFromClaudeResultLine(t *testing.T) {
	// payload is a JSON string whose value is the plan array
	evs := []store.Event{
		{Type: "stdout", Data: `{"type":"system"}`},
		resultEvent(`"[{\"title\":\"write tests\",\"agent\":\"codex\"},{\"title\":\"ship it\"}]"`),
	}
	subs, ok := parsePlan(evs)
	if !ok || len(subs) != 2 || subs[0].Title != "write tests" || subs[0].Agent != "codex" || subs[1].Title != "ship it" {
		t.Fatalf("parse = %+v ok=%v", subs, ok)
	}
}

func TestParsePlanStripsFenceAndProse(t *testing.T) {
	evs := []store.Event{resultEvent("\"Here is the plan:\\n```json\\n[{\\\"title\\\":\\\"a\\\"}]\\n```\"")}
	subs, ok := parsePlan(evs)
	if !ok || len(subs) != 1 || subs[0].Title != "a" {
		t.Fatalf("fenced parse = %+v ok=%v", subs, ok)
	}
}

func TestParsePlanEmptyArrayIsValidZero(t *testing.T) {
	subs, ok := parsePlan([]store.Event{resultEvent(`"[]"`)})
	if !ok || len(subs) != 0 {
		t.Fatalf("empty array should be ok with 0 items; got %+v ok=%v", subs, ok)
	}
}

func TestParsePlanGarbageIsUnparsed(t *testing.T) {
	// prose with a stray non-object bracketed list -> not array-of-objects -> unparsed
	if _, ok := parsePlan([]store.Event{resultEvent(`"sorry, I can't. see [1,2,3]"`)}); ok {
		t.Fatal("non-object array should be unparsed (ok=false)")
	}
	// no bracket at all -> unparsed
	if _, ok := parsePlan([]store.Event{resultEvent(`"just prose"`)}); ok {
		t.Fatal("no array should be unparsed")
	}
	// doubled arrays (proving we don't accept a concatenation) -> unparsed
	if _, ok := parsePlan([]store.Event{resultEvent(`"[{\"title\":\"a\"}][{\"title\":\"a\"}]"`)}); ok {
		t.Fatal("doubled array should be unparsed")
	}
}

func TestParsePlanIgnoresTrailingFencedExample(t *testing.T) {
	// The model emitted the real array, then an illustrative fenced example. The
	// example must NOT replace the real plan (spec-026 robustness: critical).
	raw := "[{\"title\":\"write tests\"},{\"title\":\"ship it\"}]\nExample format: ```[{\"title\":\"...\"}]```"
	subs, ok := parsePlan([]store.Event{resultEventRaw(raw)})
	if !ok || len(subs) != 2 || subs[0].Title != "write tests" || subs[1].Title != "ship it" {
		t.Fatalf("trailing fenced example must not clobber the plan; got %+v ok=%v", subs, ok)
	}
}

func TestParsePlanTitleWithCodeFence(t *testing.T) {
	// A sub-task title legitimately contains a ```json fence; backticks inside a
	// JSON string are content, not structure (spec-026 robustness: important).
	raw := "[{\"title\":\"emit ```json output\"},{\"title\":\"ship it\"}]"
	subs, ok := parsePlan([]store.Event{resultEventRaw(raw)})
	if !ok || len(subs) != 2 || subs[0].Title != "emit ```json output" || subs[1].Title != "ship it" {
		t.Fatalf("a triple-backtick in a title must not break parsing; got %+v ok=%v", subs, ok)
	}
}

func TestParsePlanPrefersPlanBearingResultLine(t *testing.T) {
	// Two result lines (e.g. a gateway failover retry): the plan-bearing one must
	// win over a later prose result, not be clobbered by last-wins (important).
	evs := []store.Event{
		resultEventRaw(`[{"title":"a"},{"title":"b"}]`),
		resultEventRaw("Summary: I created a 2-step plan."),
	}
	subs, ok := parsePlan(evs)
	if !ok || len(subs) != 2 || subs[0].Title != "a" || subs[1].Title != "b" {
		t.Fatalf("a later prose result must not clobber the plan; got %+v ok=%v", subs, ok)
	}
}

func TestParsePlanFallsBackToLastMessage(t *testing.T) {
	// no result line (e.g. hermes) -> last message event is the final text
	evs := []store.Event{
		{Type: "message", Data: "thinking..."},
		{Type: "message", Data: `[{"title":"from message"}]`},
	}
	subs, ok := parsePlan(evs)
	if !ok || len(subs) != 1 || subs[0].Title != "from message" {
		t.Fatalf("message fallback = %+v ok=%v", subs, ok)
	}
}

func newPlanner(t *testing.T) (Planner, *engine.Engine, *store.Store) {
	t.Helper()
	rs, err := rules.Parse([]byte(plannerRules))
	if err != nil {
		t.Fatal(err)
	}
	st := planStore(t)
	eng := engine.New(router.New(rs), fake.New(), st, t.TempDir())
	return NewPlanner(eng, st, "claude"), eng, st
}

func TestIngestCreatesBacklogItems(t *testing.T) {
	p, _, st := newPlanner(t)
	if err := st.CreateRun(store.Run{ID: "r1", Title: "breakdown: x", Agent: "claude", Status: "succeeded"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AppendEvent(store.Event{RunID: "r1", Type: "stdout", Data: `{"type":"result","result":"[{\"title\":\"one\"},{\"title\":\"two\"}]"}`}); err != nil {
		t.Fatal(err)
	}
	p.ingest("r1", "x")
	items, _ := st.ListBacklog()
	if len(items) != 2 || items[0].Source != "agent" {
		t.Fatalf("items = %+v", items)
	}
}

func TestIngestUnparsedFallback(t *testing.T) {
	p, _, st := newPlanner(t)
	st.CreateRun(store.Run{ID: "r2", Title: "breakdown: y", Status: "succeeded"})
	st.AppendEvent(store.Event{RunID: "r2", Type: "stdout", Data: `{"type":"result","result":"i cannot"}`})
	p.ingest("r2", "y")
	items, _ := st.ListBacklog()
	if len(items) != 1 || items[0].Title != "breakdown (unparsed): y" {
		t.Fatalf("unparsed fallback = %+v", items)
	}
}

func TestIngestSkipsFailedRun(t *testing.T) {
	p, _, st := newPlanner(t)
	st.CreateRun(store.Run{ID: "r3", Title: "breakdown: z", Status: "failed"})
	st.AppendEvent(store.Event{RunID: "r3", Type: "stdout", Data: `{"type":"result","result":"[{\"title\":\"x\"}]"}`})
	p.ingest("r3", "z")
	if items, _ := st.ListBacklog(); len(items) != 0 {
		t.Fatalf("failed run should create no items; got %+v", items)
	}
}

func TestBreakdownDispatchesVisibleRun(t *testing.T) {
	rs, err := rules.Parse([]byte(plannerRules))
	if err != nil {
		t.Fatal(err)
	}
	st := planStore(t)
	rt := fake.New()
	rt.Stdout = []string{`{"type":"result","result":"[{\"title\":\"one\",\"agent\":\"codex\"},{\"title\":\"two\"}]"}`}
	eng := engine.New(router.New(rs), rt, st, t.TempDir())
	p := NewPlanner(eng, st, "claude")

	runID, err := p.Breakdown(context.Background(), "build a thing", "", "")
	if err != nil || runID == "" {
		t.Fatalf("breakdown: id=%q err=%v", runID, err)
	}
	p.e.Wait(runID)
	// let the ingest goroutine finish after Wait
	deadline := time.Now().Add(2 * time.Second)
	for {
		if items, _ := st.ListBacklog(); len(items) == 2 {
			break
		}
		if time.Now().After(deadline) {
			items, _ := st.ListBacklog()
			t.Fatalf("expected 2 sub-tasks in backlog; got %+v", items)
		}
		time.Sleep(10 * time.Millisecond)
	}
	run, err := st.GetRun(runID)
	if err != nil || run.Title != "breakdown: build a thing" || run.Agent != "claude" {
		t.Fatalf("planner run = %+v err=%v", run, err)
	}
	items, _ := st.ListBacklog()
	// ListBacklog is newest-first; both created ~same instant, so assert by set
	titles := map[string]string{items[0].Title: items[0].Agent, items[1].Title: items[1].Agent}
	if _, ok := titles["two"]; !ok || titles["one"] != "codex" {
		t.Fatalf("sub-tasks = %+v", items)
	}
	if items[0].Source != "agent" {
		t.Fatalf("source = %q, want agent", items[0].Source)
	}
}

// TestDefaultRulesetForcesPlannerAgent guards spec-026 D7: forcing the planner
// agent depends on an @agent passthrough rule in the active ruleset. This pins
// that the shipped rules/v1.yaml routes an explicit claude task to claude, so
// FORT_PLANNER=claude actually runs on claude.
func TestDefaultRulesetForcesPlannerAgent(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "rules", "v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	rs, err := rules.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	eng := engine.New(router.New(rs), fake.New(), planStore(t), t.TempDir())
	if dec := eng.Route(task.Task{ID: "t", Title: "x", Agent: "claude"}); dec.Route != "claude" {
		t.Fatalf("rules/v1.yaml must force Agent=claude -> claude (planner precondition); got %q", dec.Route)
	}
}
