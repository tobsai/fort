# Task Breakdown Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement spec 026 — a "break down" action that runs a planner agent and turns its output into backlog sub-tasks you curate and drag to run.

**Architecture:** A new `ui.Planner` port (nil in control-only → the endpoint 409s), implemented by `control.Planner` using the engine (dispatch a visible planner run) and the store (read the run's output, create backlog items). The risky part — recovering a clean JSON plan from claude's stream-json output — is a pure, unit-tested function that reads the single terminal `result` line (a raw stdout event) rather than concatenating normalized message events. A small exported `engine.Wait(runID)` lets `control` block on the run.

**Tech Stack:** Go 1.22 stdlib (`net/http`, `encoding/json`, `log/slog`), the existing engine/store/ui seams, vanilla JS for the board button.

**Ground rules (CLAUDE.md):** TDD every Go task; `go test ./...` stays green after each; `-race` on `control` + `core/engine`. Seams: `ui` imports no engine (only the port + store + task); `control` may import engine + store. The one model call is the planner run — routing/placement stay model-free. Commit after each task on branch `feat/026-task-breakdown`.

**Spec:** `specs/026-task-breakdown.md` (approved, hardened after adversarial review). Read it first — especially the extraction (single `result` line, not concatenation), empty-vs-unparsed, the D7 ruleset precondition, and the crash-window limitation.

---

### Task 1: `engine.Wait` — exported wait

**Files:**
- Modify: `core/engine/engine.go` (add exported `Wait`)
- Test: `core/engine/wait_test.go` (new)

- [ ] **Step 1.1: Write the failing test** — `core/engine/wait_test.go`:

```go
package engine

import (
	"context"
	"testing"
	"time"

	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/exec/fake"
	"path/filepath"
)

func waitEngine(t *testing.T) (*Engine, *store.Store) {
	t.Helper()
	rs, err := rules.Parse([]byte(ruleset))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(router.New(rs), fake.New(), st, t.TempDir()), st
}

func TestWaitBlocksThenReturnsAfterPersist(t *testing.T) {
	e, st := waitEngine(t)
	runID, _, err := e.SubmitRef(context.Background(), task.Task{ID: "t1", Title: "x"})
	if err != nil {
		t.Fatal(err)
	}
	e.Wait(runID) // blocks until consume() has persisted + closed done
	got, err := st.GetRun(runID)
	if err != nil || got.Status != "succeeded" {
		t.Fatalf("after Wait: status=%q err=%v", got.Status, err)
	}
	// events fully persisted by the time Wait returned
	evs, _ := st.Events(runID)
	if len(evs) == 0 {
		t.Fatal("no events persisted after Wait")
	}
}

func TestWaitReturnsImmediatelyForFinishedOrUnknown(t *testing.T) {
	e, _ := waitEngine(t)
	// unknown id: immediate
	done := make(chan struct{})
	go func() { e.Wait("nope"); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Wait on unknown id did not return")
	}
	// finished run: complete it, then Wait returns immediately
	runID, _, _ := e.SubmitRef(context.Background(), task.Task{ID: "t2", Title: "y"})
	e.Wait(runID) // first wait blocks to completion
	done2 := make(chan struct{})
	go func() { e.Wait(runID); close(done2) }() // second wait: run finished/untracked
	select {
	case <-done2:
	case <-time.After(time.Second):
		t.Fatal("Wait on finished run did not return immediately")
	}
}
```

- [ ] **Step 1.2: Run — expect FAIL** (`e.Wait undefined`).

Run: `go test ./core/engine/ -run TestWait -v`
Expected: build failure `e.Wait undefined`.

- [ ] **Step 1.3: Implement** — in `core/engine/engine.go`, right after the unexported `wait` method:

```go
// Wait blocks until the run's events are fully persisted, then returns. It
// returns immediately for an unknown or already-finished run. Exposed so
// control.Planner can block on a planner run without polling (spec 026).
func (e *Engine) Wait(runID string) { e.wait(runID) }
```

- [ ] **Step 1.4: Run — expect PASS**

Run: `go test ./core/engine/ -race -run TestWait -v`
Expected: PASS.

- [ ] **Step 1.5: Commit**

```bash
git add core/engine/engine.go core/engine/wait_test.go
git commit -m "feat(engine): exported Wait(runID) for planner completion (spec 026)"
```

---

### Task 2: `ui.Planner` port + contract + Deps field

**Files:**
- Modify: `ui/ports.go` (add `Planner` interface)
- Modify: `ui/server.go` (add `Planner` to `Deps`)
- Modify: `ui/contract.go` (`BreakdownRequest` + `BreakdownResult`)

- [ ] **Step 2.1: Add the port** — append to `ui/ports.go`:

```go
// Planner decomposes a goal into backlog sub-tasks by running a planner agent
// (spec 026). It is nil in control-only mode (planning needs an execution
// plane); the /api/breakdown endpoint 409s when it is nil. Breakdown returns the
// planner run's id immediately; the sub-tasks land in the backlog asynchronously
// when that run completes.
type Planner interface {
	Breakdown(ctx context.Context, goal, agent, machine string) (runID string, err error)
}
```

- [ ] **Step 2.2: Add the Deps field** — in `ui/server.go`, in the `Deps` struct, after `Machines`:

```go
	Planner Planner // nil in control-only mode (spec 026)
```

- [ ] **Step 2.3: Add contract types** — append to `ui/contract.go`:

```go
// BreakdownRequest is the command body for POST /api/breakdown.
type BreakdownRequest struct {
	Text    string `json:"text"`
	Agent   string `json:"agent,omitempty"`
	Machine string `json:"machine,omitempty"`
}

// BreakdownResult is the response for POST /api/breakdown: the visible planner
// run's id. Sub-tasks appear in the backlog when that run completes.
type BreakdownResult struct {
	RunID string `json:"run_id"`
}
```

- [ ] **Step 2.4: Build**

Run: `go build ./ui/... && go vet ./ui/...`
Expected: clean (nothing consumes these yet).

- [ ] **Step 2.5: Commit**

```bash
git add ui/ports.go ui/server.go ui/contract.go
git commit -m "feat(ui): Planner port + breakdown contract types (spec 026)"
```

---

### Task 3: `control.Planner` — prompt, extraction, ingest, Breakdown

**Files:**
- Create: `control/planner.go`
- Create: `control/planner_test.go`

- [ ] **Step 3.1: Write the failing tests** — `control/planner_test.go`. These cover the extraction robustness (the review's blocker area) plus ingest and orchestration:

```go
package control

import (
	"context"
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
	p, _, st := newPlanner(t)
	runID, err := p.Breakdown(context.Background(), "build a thing", "", "")
	if err != nil || runID == "" {
		t.Fatalf("breakdown: id=%q err=%v", runID, err)
	}
	// wait for the planner run + its async ingest
	p.e.Wait(runID)
	time.Sleep(50 * time.Millisecond) // let the ingest goroutine finish after Wait
	run, err := st.GetRun(runID)
	if err != nil || run.Title != "breakdown: build a thing" || run.Agent != "claude" {
		t.Fatalf("planner run = %+v err=%v", run, err)
	}
	// the fake emits no plan, so ingest lands the unparsed fallback (proves the
	// full dispatch->Wait->ingest wiring runs)
	items, _ := st.ListBacklog()
	if len(items) != 1 || items[0].Title != "breakdown (unparsed): build a thing" {
		t.Fatalf("expected one unparsed item from fake output; got %+v", items)
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
```

(Add `"os"` and `"path/filepath"` to this test file's imports — `filepath` is already used by `planStore`.)

- [ ] **Step 3.2: Run — expect FAIL** (`undefined: NewPlanner`, `parsePlan`).

Run: `go test ./control/ -run 'ParsePlan|Ingest|Breakdown' -v`
Expected: build failure.

- [ ] **Step 3.3: Implement** — create `control/planner.go`:

```go
package control

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tobsai/fort/core/engine"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/ui"
)

// Planner implements ui.Planner: it runs a planner agent to decompose a goal
// into backlog sub-tasks (spec 026). The planner is a normal, visible run;
// Breakdown returns its id immediately and the sub-tasks are created when the
// run completes.
type Planner struct {
	e            *engine.Engine
	s            *store.Store
	defaultAgent string
	log          *slog.Logger
}

// NewPlanner adapts the engine + store to ui.Planner. defaultAgent is the
// planner agent used when a breakdown request doesn't specify one (FORT_PLANNER;
// falls back to "claude").
func NewPlanner(e *engine.Engine, s *store.Store, defaultAgent string) Planner {
	if defaultAgent == "" {
		defaultAgent = "claude"
	}
	return Planner{e: e, s: s, defaultAgent: defaultAgent, log: slog.Default()}
}

type subTask struct {
	Title   string `json:"title"`
	Agent   string `json:"agent"`
	Machine string `json:"machine"`
}

func plannerPrompt(goal string) string {
	return "Break the following work into a short list (3 to 8) of concrete, " +
		"independently-runnable sub-tasks. Output ONLY a JSON array; each element " +
		"an object with a \"title\" (string) and optional \"agent\" and \"machine\" " +
		"strings. No prose, no explanation, no markdown code fences. If the work " +
		"needs no breakdown, output []. Work:\n" + goal
}

// Breakdown dispatches the planner run and returns its id; a goroutine ingests
// its output into the backlog once it completes. NOTE (spec 026 crash window):
// if the process stops between the run finishing and ingest writing items, the
// sub-tasks are not written — re-run the breakdown.
func (p Planner) Breakdown(ctx context.Context, goal, agent, machine string) (string, error) {
	ag := agent
	if ag == "" {
		ag = p.defaultAgent
	}
	t := task.Task{
		ID: uuid.NewString(), Title: "breakdown: " + goal, Body: plannerPrompt(goal),
		Agent: ag, Machine: machine, CreatedAt: time.Now(),
	}
	runID, _, err := p.e.SubmitRef(ctx, t)
	if err != nil {
		return "", err
	}
	go func() {
		p.e.Wait(runID)
		p.ingest(runID, goal)
	}()
	return runID, nil
}

// ingest reads a completed planner run's output and creates backlog items.
func (p Planner) ingest(runID, goal string) {
	run, err := p.s.GetRun(runID)
	if err != nil || run.Status != "succeeded" {
		return // failed/canceled planner -> no items
	}
	evs, err := p.s.Events(runID)
	if err != nil {
		p.log.Error("planner: read events", "run", runID, "err", err)
		return
	}
	subs, ok := parsePlan(evs)
	if !ok {
		p.log.Warn("planner: unparseable output", "run", runID)
		_ = p.s.CreateBacklogItem(store.BacklogItem{
			ID: uuid.NewString(), Title: "breakdown (unparsed): " + goal,
			Body: finalText(evs), Source: "agent", CreatedAt: time.Now(),
		})
		return
	}
	for _, st := range subs {
		_ = p.s.CreateBacklogItem(store.BacklogItem{
			ID: uuid.NewString(), Title: st.Title, Agent: st.Agent, Machine: st.Machine,
			Source: "agent", CreatedAt: time.Now(),
		})
	}
}

// finalText recovers the planner's final answer from a single authoritative
// source: claude's terminal stream-json result line (stored as a raw stdout
// event); falling back to the last normalized message event for plain-text
// providers. It deliberately does NOT concatenate message events (claude emits
// partial deltas AND a terminal line, which would double the array).
func finalText(evs []store.Event) string {
	var result, lastMsg string
	for _, e := range evs {
		switch e.Type {
		case "stdout":
			var line struct {
				Type   string `json:"type"`
				Result string `json:"result"`
			}
			if json.Unmarshal([]byte(e.Data), &line) == nil && line.Type == "result" {
				result = line.Result
			}
		case "message":
			lastMsg = e.Data
		}
	}
	if result != "" {
		return result
	}
	return lastMsg
}

// parsePlan extracts the sub-task list from a run's events. Returns ok=false for
// unparseable output (caller writes the raw fallback); ok=true with zero items
// for a valid empty plan.
func parsePlan(evs []store.Event) ([]subTask, bool) {
	arr, ok := extractArray(finalText(evs))
	if !ok {
		return nil, false
	}
	var raws []subTask
	if json.Unmarshal([]byte(arr), &raws) != nil {
		return nil, false // not an array-of-objects (e.g. [1,2,3]) -> unparsed
	}
	var out []subTask
	for _, r := range raws {
		if strings.TrimSpace(r.Title) == "" {
			continue // shape-validate: every sub-task needs a title
		}
		out = append(out, r)
	}
	return out, true // empty array -> (nil, true): valid, zero items
}

// extractArray strips an optional ```json fence and returns the outermost
// balanced [...] span, or ok=false if there is none.
func extractArray(text string) (string, bool) {
	s := strings.TrimSpace(text)
	if i := strings.Index(s, "```"); i >= 0 {
		s = s[i+3:]
		s = strings.TrimPrefix(strings.TrimSpace(s), "json")
		if j := strings.LastIndex(s, "```"); j >= 0 {
			s = s[:j]
		}
	}
	a := strings.Index(s, "[")
	b := strings.LastIndex(s, "]")
	if a < 0 || b <= a {
		return "", false
	}
	return s[a : b+1], true
}
```

Note the test accesses `p.e` (unexported field) — that works because `control/planner_test.go` is `package control` (internal test). Keep it internal (not `control_test`).

- [ ] **Step 3.4: Run — expect PASS**

Run: `go test ./control/ -race -run 'ParsePlan|Ingest|Breakdown' -v`
Expected: PASS (all extraction cases, ingest, and the fake-run orchestration).

- [ ] **Step 3.5: Full package + commit**

Run: `go test ./control/...`
```bash
git add control/planner.go control/planner_test.go
git commit -m "feat(control): Planner — decompose a goal into backlog sub-tasks (spec 026)"
```

---

### Task 4: `POST /api/breakdown` endpoint

**Files:**
- Modify: `ui/server.go` (register route + handler)
- Test: `ui/breakdown_test.go` (new)

- [ ] **Step 4.1: Write the failing test** — `ui/breakdown_test.go`:

```go
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
	s := ui.New(ui.Deps{Dispatcher: newCapturing(), Store: st, Planner: sp})
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
	s := ui.New(ui.Deps{Dispatcher: newCapturing(), Store: st, Planner: &stubPlanner{}})
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
```

Add a `newCapturing()` helper if one isn't already present — check `ui/ui_test.go` for `capturingDispatcher`; if it exists, use `&capturingDispatcher{}` directly in place of `newCapturing()` and delete the helper reference. (The existing `capturingDispatcher` at ui_test.go satisfies `ui.Dispatcher`.)

- [ ] **Step 4.2: Run — expect FAIL** (404 route / undefined types).

Run: `go test ./ui/ -run TestBreakdown -v`
Expected: build failure or 404.

- [ ] **Step 4.3: Register + implement** — in `ui/server.go` `Register`, after the backlog routes:

```go
	mux.HandleFunc("POST /api/breakdown", s.handleBreakdown)
```

And the handler:

```go
func (s *Server) handleBreakdown(w http.ResponseWriter, r *http.Request) {
	if s.d.Planner == nil {
		http.Error(w, "no execution plane: breakdown needs the engine", http.StatusConflict)
		return
	}
	var req BreakdownRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}
	runID, err := s.d.Planner.Breakdown(r.Context(), req.Text, req.Agent, req.Machine)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, BreakdownResult{RunID: runID})
}
```

- [ ] **Step 4.4: Run — expect PASS**

Run: `go test ./ui/ -run TestBreakdown -v && go test ./ui/...`
Expected: PASS.

- [ ] **Step 4.5: Commit**

```bash
git add ui/server.go ui/breakdown_test.go
git commit -m "feat(ui): POST /api/breakdown endpoint (spec 026)"
```

---

### Task 5: cmd/fort wiring — planner injection + CLI

**Files:**
- Modify: `cmd/fort/main.go` (inject `Planner` in serve; `fort task breakdown` command + usage)

- [ ] **Step 5.1: Inject the planner (serve only)** — in `cmd/fort/main.go` `cmdServe`, where `deps := ui.Deps{...}` is built (the block that sets Dispatcher/Runner/Store/FlowIDs), add after it:

```go
	planner := os.Getenv("FORT_PLANNER")
	if planner == "" {
		planner = "claude"
	}
	deps.Planner = control.NewPlanner(a.engine, a.store, planner)
```

(`a.engine` and `a.store` are already in scope in `cmdServe`; `control` and `os` are already imported.)

- [ ] **Step 5.2: Add the CLI command** — in `cmd/fort/main.go` `main()` switch, add a case (the `task` case already dispatches to `cmdTask`; extend `cmdTask` to handle `breakdown`). In `cmdTask`, after the `add` handling, add:

```go
	if args[0] == "breakdown" {
		goal := strings.TrimSpace(strings.Join(args[1:], " "))
		if goal == "" {
			return fmt.Errorf("usage: fort task breakdown \"<goal>\"")
		}
		cfg := config.Load(os.Getenv)
		_, port, _ := net.SplitHostPort(cfg.Addr)
		body, _ := json.Marshal(map[string]string{"text": goal})
		resp, err := http.Post("http://127.0.0.1:"+port+"/api/breakdown", "application/json", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("fort serve is not running on this machine — start it first (breakdown runs in the daemon)")
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("breakdown failed: %s", strings.TrimSpace(string(b)))
		}
		var res struct {
			RunID string `json:"run_id"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&res)
		fmt.Printf("planner run %s started — sub-tasks will appear in the backlog\n", res.RunID)
		return nil
	}
```

Add imports to `cmd/fort/main.go` as needed: `bytes`, `encoding/json`, `io`, `net`, `net/http`, `github.com/tobsai/fort/core/config` (some already present — only add the missing ones; run `goimports`/build to confirm). Update the `usage` const's task line:

```
  fort task add [taskflags]        route + dispatch a task natively
  fort task breakdown "<goal>"     plan a goal into backlog sub-tasks (needs fort serve)
```

- [ ] **Step 5.3: Build + smoke**

Run: `go build ./... && go vet ./...`
Then:
```bash
go build -o /tmp/fort-26 ./cmd/fort
FORT_FAKE=1 FORT_DB=/tmp/bd26/fort.db FORT_ADDR=127.0.0.1:4094 /tmp/fort-26 serve >/tmp/bd26.log 2>&1 &
sleep 2
FORT_DB=/tmp/bd26/fort.db FORT_ADDR=127.0.0.1:4094 /tmp/fort-26 task breakdown "build a login page"
sleep 1
curl -s 127.0.0.1:4094/api/backlog | python3 -c "import sys,json; print('backlog:',[b['title'] for b in json.load(sys.stdin)])"
kill %1; rm -rf /tmp/bd26
```
Expected: the CLI prints a planner run id; the backlog gets one `breakdown (unparsed): build a login page` item (fake emits no plan). This proves the endpoint + planner + CLI wire end to end. Capture the output.

- [ ] **Step 5.4: Commit**

```bash
git add cmd/fort/main.go
git commit -m "feat(cmd): planner injection + fort task breakdown (spec 026)"
```

---

### Task 6: Board "Break down" button

**Files:**
- Modify: `ui/page.go` (compose bar button + JS action)

- [ ] **Step 6.1: Add the button** — in `ui/page.go` `boardHTML`, in the compose bar, after the `Add to backlog` button:

```html
<button onclick="breakdownTask()">Break down</button>
```

- [ ] **Step 6.2: Add the JS action** — in the `<script>`, next to `addToBacklog`:

```js
async function breakdownTask(){
  const el=$('#msg'),text=el.value.trim();if(!text)return;el.value='';
  const r=await fetch('/api/breakdown',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({text,machine:$('#machine').value,agent:$('#agent').value})});
  if(r.status===409)alert('Breakdown needs an execution plane — start fort serve.');
  refresh();
}
```

- [ ] **Step 6.3: Build + verify served page**

Run: `go build ./... `
```bash
go build -o /tmp/fort-26 ./cmd/fort
FORT_FAKE=1 FORT_DB=/tmp/bd26b/fort.db FORT_ADDR=127.0.0.1:4093 /tmp/fort-26 serve >/tmp/bd26b.log 2>&1 &
sleep 2
curl -s 127.0.0.1:4093/ | grep -c 'Break down'   # expect 1
kill %1; rm -rf /tmp/bd26b
```
Expected: `Break down` present once. (The controller does the visual/live click verification.)

- [ ] **Step 6.4: Commit**

```bash
git add ui/page.go
git commit -m "feat(ui): Break down button on the board compose bar (spec 026)"
```

---

### Task 7: Docs + full verification

**Files:**
- Modify: `README.md` (mention breakdown + FORT_PLANNER)

- [ ] **Step 7.1: Docs** — add a short line near the web-board bullet or the env section:

```
Break a goal into backlog sub-tasks with the board's "Break down" button or
`fort task breakdown "<goal>"` — a planner agent (FORT_PLANNER, default claude)
decomposes it into items you curate and drag to run.
```

- [ ] **Step 7.2: Full gates**

Run: `go test ./... && go test -race ./control/ ./core/engine/ ./ui/... && go vet ./...`
Expected: all green.

- [ ] **Step 7.3: Seam check** — `ui` must not import engine:

Run: `go list -deps ./ui | grep -E 'core/engine|core/graph|core/router|exec/native' || echo "seam clean"`
Expected: `seam clean`.

- [ ] **Step 7.4: Determinism guard** — the planner path makes exactly one dispatch and no model call during extraction:

Run: `go list -deps ./control | grep -E 'exec/native' || echo "no direct native in control"` (control talks to the engine, which owns the runtime; the extraction is pure). Confirm `control/planner.go` has no `Dispatch`/runtime call outside `SubmitRef`.

- [ ] **Step 7.5: Commit**

```bash
git add README.md
git commit -m "docs: task breakdown (fort task breakdown, FORT_PLANNER) (spec 026)"
```

---

## Self-review checklist (run after Task 7)

- **Spec coverage:** trigger (board button T6 + CLI T5 + endpoint T4); planner as a visible run with FORT_PLANNER default (T3 Breakdown, T5 injection); single-source extraction from claude's result line, NOT concatenation (T3 `finalText`/`parsePlan`, tested by the doubled-array case); empty-vs-unparsed (T3 tests); status-check-after-Wait skips failed runs (T3 `ingest` + test); crash-window documented in code comment + spec; `ui.Planner` port nil→409 (T2/T4); determinism = one dispatch (T7.4). D7 ruleset precondition: the planner uses `task.Agent`; the extraction/creation path has no model call.
- **Type consistency:** `ui.Planner.Breakdown(ctx, goal, agent, machine) (string, error)` matches control's method and the stub in T4; `BreakdownRequest{Text,Agent,Machine}` / `BreakdownResult{RunID}` consistent across T2/T4/T5/T6; `store.BacklogItem` fields (Source="agent") match Task-1's store; `subTask` is control-internal (not exposed).
- **Placeholder scan:** every step has complete code.
- **Missing-test note:** the happy-path "real claude plan → items" at the Breakdown level is covered at the `parsePlan`/`ingest` layer (canned claude result line) rather than through the fake runtime (which can't emit a plan) — this is intentional and called out in T3.
