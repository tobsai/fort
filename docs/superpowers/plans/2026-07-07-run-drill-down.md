# Run Drill-Down Drawer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a board run card open a detail drawer that shows a flow's executed DAG steps and each step's own live log (and, for a single run, just its live log).

**Architecture:** One additive data change — an `event.node_id` column stamped by the graph executor's `execTask` — gives per-step log attribution. Everything else is read-side: the existing `GET /api/runs/{id}` (run + nodes + events) and `/api/events` SSE gain `node_id`, and `ui/page.go` grows a clickable-card → drawer that filters events by the selected step. No new endpoints; the `ui` seam (no engine import) is unchanged.

**Tech Stack:** Go 1.22 (`core/store` SQLite via modernc, `core/graph` executor, `ui` net/http + served HTML/CSS/JS), `exec/fake` runtime for tests. Governing spec: `specs/027-run-drill-down.md`.

---

## File Structure

- `core/store/store.go` — `event.node_id` column (schema + idempotent migration), `Event.NodeID` field, `AppendEvent` insert, `queryEvents` select/scan. **Responsibility:** persist + return a per-event node id.
- `core/graph/executor.go` — stamp `NodeID: node.ID` on the events `execTask` appends. **Responsibility:** attribute a flow task step's events to that step.
- `ui/contract.go` — `Event.NodeID` wire field. **Responsibility:** expose the node id on the HTTP/SSE contract.
- `ui/server.go` — `toEvent` copies `NodeID` (so `/api/runs/{id}` and the `/api/events` SSE carry it). **Responsibility:** serialize it.
- `ui/page.go` — clickable run cards + the drawer (step list, per-step log filter, live reload). **Responsibility:** the drill-down UI.
- `README.md` — one line documenting the drawer.

---

## Task 1: `core/store` — `event.node_id` round-trip

**Files:**
- Modify: `core/store/store.go` (`Event` struct ~line 60; `event` CREATE TABLE ~line 110; `migrate` ~line 128; `AppendEvent` ~line 333; `queryEvents` ~line 353)
- Test: `core/store/store_test.go`

- [ ] **Step 1.1: Write the failing test** — append to `core/store/store_test.go`:

```go
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
```

- [ ] **Step 1.2: Run — expect FAIL** (compile error: `Event` has no field `NodeID`).

Run: `go test ./core/store/ -run TestEventNodeIDRoundTrip`
Expected: FAIL — `unknown field NodeID in struct literal`.

- [ ] **Step 1.3: Add the `NodeID` field** — in the `Event` struct (currently lines 60-68), add `NodeID` right after `RunID`:

```go
// Event is one append-only event row.
type Event struct {
	ID        int64
	RunID     string
	NodeID    string // DAG step this event came from (spec 027); "" for run-level/single-run events
	Type      string
	Data      string
	Code      int
	CreatedAt time.Time
}
```

- [ ] **Step 1.4: Add the column to the schema + migration** — in the `event` CREATE TABLE (lines 110-113), add `node_id TEXT`:

```go
CREATE TABLE IF NOT EXISTS event (
  id INTEGER PRIMARY KEY AUTOINCREMENT, run_id TEXT, node_id TEXT, type TEXT, data TEXT,
  code INTEGER, created_at TEXT
);
```

And in `migrate()` (after the `run.machine` additive migration at line 128-130), add the idempotent column for pre-existing databases:

```go
	if err := s.addColumn("event", "node_id", "TEXT"); err != nil {
		return fmt.Errorf("store: migrate event.node_id: %w", err)
	}
```

- [ ] **Step 1.5: Write + read the column** — update `AppendEvent` (lines 333-341) to insert `node_id`:

```go
func (s *Store) AppendEvent(e Event) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO event(run_id,node_id,type,data,code,created_at) VALUES(?,?,?,?,?,?)`,
		e.RunID, e.NodeID, e.Type, e.Data, e.Code, nowOr(e.CreatedAt))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
```

Update the two `SELECT`s to include `node_id` — `Events` (line 345) and `EventsSince` (line 350):

```go
func (s *Store) Events(runID string) ([]Event, error) {
	return s.queryEvents(`SELECT id,run_id,node_id,type,data,code,created_at FROM event WHERE run_id=? ORDER BY id`, runID)
}

func (s *Store) EventsSince(cursor int64) ([]Event, error) {
	return s.queryEvents(`SELECT id,run_id,node_id,type,data,code,created_at FROM event WHERE id>? ORDER BY id`, cursor)
}
```

Update `queryEvents` (lines 353-370) to scan `node_id` NULL-safely (rows written before the migration are NULL):

```go
func (s *Store) queryEvents(q string, arg any) ([]Event, error) {
	rows, err := s.db.Query(q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var ts string
		var nodeID sql.NullString
		if err := rows.Scan(&e.ID, &e.RunID, &nodeID, &e.Type, &e.Data, &e.Code, &ts); err != nil {
			return nil, err
		}
		e.NodeID = nodeID.String
		e.CreatedAt = parseTime(ts)
		out = append(out, e)
	}
	return out, rows.Err()
}
```

(`database/sql` is already imported at line 7, so `sql.NullString` needs no new import.)

- [ ] **Step 1.6: Run — expect PASS**

Run: `go test ./core/store/ -run TestEventNodeIDRoundTrip -race`
Expected: PASS.

- [ ] **Step 1.7: Guard the whole package** (the shared `SELECT`/scan change touches every events reader).

Run: `go test ./core/store/ -race`
Expected: PASS (all existing store tests still green).

- [ ] **Step 1.8: Commit**

```bash
git add core/store/store.go core/store/store_test.go
git commit -m "feat(store): event.node_id column + Event.NodeID round-trip (spec 027)"
```

---

## Task 2: `core/graph` — stamp `NodeID` on a task step's events

**Files:**
- Modify: `core/graph/executor.go:194` (the `AppendEvent` inside `execTask`)
- Test: `core/graph/executor_test.go`

- [ ] **Step 2.1: Write the failing test** — append to `core/graph/executor_test.go`:

```go
func TestTaskEventsCarryNodeID(t *testing.T) {
	ex, st, _ := newExec(t)
	f := Flow{
		ID: "f-nid", Start: "k1",
		Nodes: []Node{{ID: "k1", Type: Task, Agent: "codex", Prompt: "do work"}},
	}
	if _, err := ex.Start(context.Background(), f, "run-nid", "payload"); err != nil {
		t.Fatalf("start: %v", err)
	}
	evs, _ := st.Events("run-nid")
	if len(evs) == 0 {
		t.Fatal("task node produced no events")
	}
	for _, e := range evs {
		if e.NodeID != "k1" {
			t.Errorf("event %q node_id=%q, want k1", e.Type, e.NodeID)
		}
	}
}

func TestTransformEventHasEmptyNodeID(t *testing.T) {
	ex, st, _ := newExec(t)
	f := Flow{ID: "f-tr", Start: "t0", Nodes: []Node{{ID: "t0", Type: Transform, Transform: &TransformSpec{Op: "upper"}}}}
	if _, err := ex.Start(context.Background(), f, "run-tr", "abc"); err != nil {
		t.Fatalf("start: %v", err)
	}
	evs, _ := st.Events("run-tr")
	for _, e := range evs {
		if e.Type == "transform" && e.NodeID != "" {
			t.Errorf("run-level transform event node_id=%q, want empty", e.NodeID)
		}
	}
}
```

- [ ] **Step 2.2: Run — expect FAIL** (`TestTaskEventsCarryNodeID` fails: events carry `""`, not `"k1"`).

Run: `go test ./core/graph/ -run 'TestTaskEventsCarryNodeID|TestTransformEventHasEmptyNodeID'`
Expected: `TestTaskEventsCarryNodeID` FAIL (`node_id="" want k1`); `TestTransformEventHasEmptyNodeID` PASS (already empty).

- [ ] **Step 2.3: Stamp the node id** — in `execTask`, change the append at line 194 to include `NodeID: node.ID`:

```go
			_, _ = e.store.AppendEvent(store.Event{RunID: runID, NodeID: node.ID, Type: string(ev.Type), Data: ev.Data, Code: ev.Code, CreatedAt: ev.Time})
```

(Only `execTask` is changed. Transform/gate/check/fanout events stay node-less by design — per spec D3/D5, only task steps carry a per-step log.)

- [ ] **Step 2.4: Run — expect PASS**

Run: `go test ./core/graph/ -run 'TestTaskEventsCarryNodeID|TestTransformEventHasEmptyNodeID' -race`
Expected: both PASS.

- [ ] **Step 2.5: Guard the package**

Run: `go test ./core/graph/ -race`
Expected: PASS.

- [ ] **Step 2.6: Commit**

```bash
git add core/graph/executor.go core/graph/executor_test.go
git commit -m "feat(graph): stamp event.node_id from execTask (spec 027)"
```

---

## Task 3: `ui` contract + server — carry `node_id` on the wire

**Files:**
- Modify: `ui/contract.go` (`Event` type ~line 18); `ui/server.go` (`toEvent` ~line 405)
- Test: `ui/ui_test.go`

- [ ] **Step 3.1: Write the failing test** — append to `ui/ui_test.go`:

```go
func TestRunDetailIncludesNodeID(t *testing.T) {
	s, st := newControlUI(t)
	_ = st.CreateRun(store.Run{ID: "rd1", Agent: "codex", Status: "running"})
	_, _ = st.AppendEvent(store.Event{RunID: "rd1", NodeID: "implement", Type: "message", Data: "hi"})
	_, _ = st.AppendEvent(store.Event{RunID: "rd1", Type: "stdout", Data: "raw"})

	rec := do(t, s, "GET", "/api/runs/rd1", nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	d := decode[ui.RunDetail](t, rec)
	if len(d.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(d.Events))
	}
	if d.Events[0].NodeID != "implement" || d.Events[1].NodeID != "" {
		t.Fatalf("node_ids = %q,%q want implement,\"\"", d.Events[0].NodeID, d.Events[1].NodeID)
	}
}
```

- [ ] **Step 3.2: Run — expect FAIL** (compile error: `ui.Event` has no field `NodeID`).

Run: `go test ./ui/ -run TestRunDetailIncludesNodeID`
Expected: FAIL — `d.Events[0].NodeID undefined`.

- [ ] **Step 3.3: Add the wire field** — in `ui/contract.go`, add `NodeID` to `Event` (lines 18-25):

```go
// Event is the wire form of one append-only event-log row (the live-feed unit).
type Event struct {
	ID     int64  `json:"id"`
	RunID  string `json:"run_id"`
	NodeID string `json:"node_id,omitempty"`
	Type   string `json:"type"`
	Data   string `json:"data,omitempty"`
	Code   int    `json:"code,omitempty"`
	Time   string `json:"time"`
}
```

- [ ] **Step 3.4: Copy it in `toEvent`** — in `ui/server.go` (line 405-407):

```go
func toEvent(e store.Event) Event {
	return Event{ID: e.ID, RunID: e.RunID, NodeID: e.NodeID, Type: e.Type, Data: e.Data, Code: e.Code, Time: e.CreatedAt.Format(time.RFC3339)}
}
```

(`toEvent` feeds both `handleRunDetail` and the `/api/events` SSE, so both carry `node_id` with this one change.)

- [ ] **Step 3.5: Run — expect PASS**

Run: `go test ./ui/ -run TestRunDetailIncludesNodeID`
Expected: PASS.

- [ ] **Step 3.6: Guard the package + seam**

Run: `go test ./ui/ && (go list -deps ./ui | grep -E 'core/engine|core/graph|core/router|exec/native' || echo "seam clean")`
Expected: PASS, then `seam clean`.

- [ ] **Step 3.7: Commit**

```bash
git add ui/contract.go ui/server.go ui/ui_test.go
git commit -m "feat(ui): carry node_id on Event (run detail + SSE) (spec 027)"
```

---

## Task 4: `ui/page.go` — clickable cards + drill-down drawer

**Files:**
- Modify: `ui/page.go` (CSS in the `<style>` block; drawer markup before `<script>`; `runCard`; drawer JS + `refresh`)

This is a served-HTML task (no Go unit test framework for the page). Verify by building, serving with the fake runtime, and grepping the served page; the human/controller does the live click check.

- [ ] **Step 4.1: Add drawer CSS** — in the `<style>` block, immediately before the closing `</style>` (line 77), add:

```css
  .run-card{cursor:pointer}
  .drawer[hidden]{display:none}
  .drawer{position:fixed;inset:0;z-index:50}
  .drawer-scrim{position:absolute;inset:0;background:rgba(0,0,0,.42)}
  .drawer-panel{position:absolute;top:0;right:0;height:100%;width:min(560px,92vw);background:var(--panel);border-left:1px solid var(--line);display:flex;flex-direction:column;box-shadow:-10px 0 30px rgba(0,0,0,.35)}
  .drawer-head{display:flex;justify-content:space-between;align-items:flex-start;gap:10px;padding:14px 16px;border-bottom:1px solid var(--line2)}
  .drawer-title{font-size:13px;color:var(--fg)}
  .drawer-sub{font-size:11px;color:var(--mut);margin-top:3px}
  .drawer-steps{padding:8px 10px;border-bottom:1px solid var(--line2);max-height:38%;overflow:auto;display:flex;flex-direction:column;gap:3px}
  .step{display:flex;align-items:center;gap:8px;padding:5px 8px;border-radius:6px;cursor:pointer;border:1px solid transparent}
  .step:hover{background:var(--line2)}
  .step.sel{background:var(--line2);border-color:var(--line)}
  .step .st{font-size:11px;width:14px;text-align:center;color:var(--mut)}
  .step .nm{font-size:12px;color:var(--fg)}
  .step .ty{font-size:10.5px;color:var(--mut);margin-left:auto}
  .step.s-succeeded .st{color:var(--ok)} .step.s-running .st{color:var(--run)} .step.s-failed .st{color:var(--fail)} .step.s-waiting .st{color:var(--block)}
  .drawer-log{flex:1;overflow:auto;padding:10px 14px;font-size:11.5px;line-height:1.55;white-space:pre-wrap;color:var(--fg2)}
  .drawer-log .ev{padding:1px 0}
  .drawer-log .ev .k{color:var(--mut)}
```

- [ ] **Step 4.2: Add the drawer markup** — insert immediately before the `<script>` tag (line 104), after the `.compose` block's closing `</div>` (line 103):

```html
<div id="drawer" class="drawer" hidden>
  <div class="drawer-scrim" onclick="closeDrawer()"></div>
  <aside class="drawer-panel" role="dialog" aria-label="run detail">
    <div class="drawer-head">
      <div><div class="drawer-title" id="dw-title">—</div><div class="drawer-sub" id="dw-sub"></div></div>
      <button class="iconbtn" onclick="closeDrawer()" aria-label="close">✕</button>
    </div>
    <div class="drawer-steps" id="dw-steps"></div>
    <div class="drawer-log" id="dw-log"></div>
  </aside>
</div>
```

- [ ] **Step 4.3: Make run cards clickable** — replace `runCard` (lines 130-135) with:

```js
function runCard(r){
  const done=(r.status==='succeeded'||r.status==='failed'||r.status==='canceled');
  return '<div class="card run-card '+(done?'done ':'')+edgeFor(r.status)+'" tabindex="0" onclick="openDrawer(\''+r.id+'\')" onkeydown="if(event.key===\'Enter\')openDrawer(\''+r.id+'\')">'+
    '<div class="title">'+esc(r.title||r.id)+'</div>'+
    '<div class="meta"><span class="ag">'+esc(r.agent)+'</span>'+(r.machine?'<span class="mc">'+esc(r.machine)+'</span>':'')+'</div></div>';
}
```

(Run ids are server-generated UUIDs, so they are safe to embed in the handler string. `gateCard`/`backlogCard` are unchanged — only run cards open the drawer.)

- [ ] **Step 4.4: Add the drawer logic** — insert this block right after `runCard`/`gateCard`/`backlogCard` (after line 148, before `function fill`):

```js
// ---- run drill-down drawer (spec 027) ----
let dwRun=null, dwNode=null, dwNodes=[], dwEvents=[];
function stepIcon(s){return s==='succeeded'?'✓':s==='failed'?'✕':s==='running'?'▸':s==='waiting'?'⏸':'▫';}
async function openDrawer(runID){ dwRun=runID; dwNode=null; $('#drawer').hidden=false; await loadDrawer(); }
function closeDrawer(){ dwRun=null; dwNode=null; $('#drawer').hidden=true; }
async function loadDrawer(){
  if(!dwRun) return;
  const d=await (await fetch('/api/runs/'+encodeURIComponent(dwRun))).json();
  dwNodes=d.nodes||[]; dwEvents=d.events||[];
  $('#dw-title').textContent=d.run.title||d.run.id;
  $('#dw-sub').textContent=[d.run.agent,d.run.status,d.run.machine].filter(Boolean).join(' · ');
  renderSteps(); renderLog();
}
function renderSteps(){
  const el=$('#dw-steps');
  if(!dwNodes.length){ el.style.display='none'; return; }
  el.style.display='flex';
  el.innerHTML=dwNodes.map(n=>
    '<div class="step s-'+esc(n.status)+(n.node_id===dwNode?' sel':'')+'" onclick="selectStep(\''+n.node_id+'\')">'+
    '<span class="st">'+stepIcon(n.status)+'</span><span class="nm">'+esc(n.node_id)+'</span>'+
    '<span class="ty">'+esc(n.type)+'</span></div>').join('');
}
function selectStep(nodeID){ dwNode=(dwNode===nodeID?null:nodeID); renderSteps(); renderLog(); }
function renderLog(){
  const evs=dwEvents.filter(e=>!dwNode||e.node_id===dwNode);
  const log=$('#dw-log');
  if(!evs.length){ log.innerHTML='<div class="empty">waiting…</div>'; return; }
  log.innerHTML=evs.map(e=>'<div class="ev"><span class="k">'+esc(e.type)+'</span> '+esc(e.data||'')+'</div>').join('');
  log.scrollTop=log.scrollHeight;
}
document.addEventListener('keydown',e=>{if(e.key==='Escape')closeDrawer();});
```

- [ ] **Step 4.5: Keep the drawer live** — at the end of `refresh()`, right before its closing brace (after line 180 `fill('c-backlog',…)`), add:

```js
  if(dwRun) loadDrawer();
```

(The board already calls `refresh()` on every SSE event and every 3s, so an open drawer re-fetches `/api/runs/{id}` on the same cadence — step statuses and log stay current with no new endpoint.)

- [ ] **Step 4.6: Build + serve + grep the page**

```bash
go build ./... || exit 1
go build -o /tmp/fort-27 ./cmd/fort
mkdir -p /tmp/dd27
FORT_FAKE=1 FORT_DB=/tmp/dd27/fort.db FORT_ADDR=127.0.0.1:4091 /tmp/fort-27 serve >/tmp/dd27.log 2>&1 &
for i in $(seq 1 20); do curl -s 127.0.0.1:4091/api/summary >/dev/null 2>&1 && break; sleep 0.25; done
echo -n "drawer markup: "; curl -s 127.0.0.1:4091/ | grep -c 'id="drawer"'
echo -n "openDrawer fn: "; curl -s 127.0.0.1:4091/ | grep -c 'async function openDrawer'
echo -n "card opens drawer: "; curl -s 127.0.0.1:4091/ | grep -c "onclick=\"openDrawer"
kill %1 2>/dev/null; rm -rf /tmp/dd27 /tmp/fort-27
```

Expected: each count is `1` (drawer markup present, `openDrawer` defined, run cards wired). The live click-through (open a run, select a step, watch the log) is verified by the human/controller.

- [ ] **Step 4.7: Commit**

```bash
git add ui/page.go
git commit -m "feat(ui): run drill-down drawer — clickable cards, step list, per-step live log (spec 027)"
```

---

## Task 5: Docs + full verification + final review

**Files:**
- Modify: `README.md` (the Web bullet)

- [ ] **Step 5.1: Document the drawer** — in `README.md`, extend the **Web** bullet (the "served at `GET /`" line) by appending this sentence to it:

```
  Click any run to open a detail drawer: a flow shows its DAG steps and each
  step's own live log; a single run shows its live log.
```

- [ ] **Step 5.2: Full gates**

Run: `go test ./... && go test -race ./core/store/ ./core/graph/ ./ui/... && go vet ./...`
Expected: all green.

- [ ] **Step 5.3: Seam check** — `ui` must not import the execution packages:

Run: `go list -deps ./ui | grep -E 'core/engine|core/graph|core/router|exec/native' || echo "seam clean"`
Expected: `seam clean`.

- [ ] **Step 5.4: Determinism guard** — the change adds no model calls (node_id is descriptive metadata on the append path):

Run: `git diff main -- core/ | grep -nE 'Dispatch|Route|rt\.' || echo "no new dispatch/route in core"`
Expected: `no new dispatch/route in core` (the only `core` diff is the store column + the `execTask` append field).

- [ ] **Step 5.5: Commit**

```bash
git add README.md
git commit -m "docs: run drill-down drawer (spec 027)"
```

- [ ] **Step 5.6: Final whole-branch review** — dispatch an adversarial review of the full 027 diff (store migration correctness incl. NULL-safe scan of pre-migration rows; executor stamping only task events; the drawer's per-step filter + live-reload; XSS on `esc()` of node id / event data in the drawer; the `ui` seam). Fix any critical/important findings before merge.

---

## Self-review

**Spec coverage (each spec section → task):**
- Goal (drawer: flow steps + per-step log; single run: log) → Task 4 (`openDrawer`/`renderSteps`/`renderLog`, non-flow hides steps).
- Data change `event.node_id` → Task 1; executor stamp → Task 2.
- No new endpoint (reuse `/api/runs/{id}` + `/api/events` SSE) → Task 3 (`toEvent` carries `node_id`) + Task 4 (`loadDrawer` fetches `/api/runs/{id}`, `refresh` re-loads live).
- D5 executed-steps-only → Task 4 renders `d.nodes` (`NodeRuns` only returns executed steps); no future-step pre-render.
- D6 works for any run → Task 4 `renderSteps` hides the step list when `dwNodes` is empty (single run → log only).
- D7 gates read-only → Task 4 shows a gate step's status only; no approve/reject in the drawer (unchanged Blocked column).
- Determinism / seam → Task 3 Step 3.6 + Task 5 Steps 5.3–5.4.
- Tests (store round-trip, executor stamp + empty-for-non-task, run-detail node_id, single-run empty) → Tasks 1.1, 2.1, 3.1.
- Rollback (additive) → covered by the commit boundaries (revert Task 4/3/2; column is harmless).

**Placeholder scan:** none — every code step shows complete code and every run step shows the exact command + expected output.

**Type consistency:** `Event.NodeID` (store, Go) ↔ `Event.NodeID`/`json:"node_id"` (ui wire) ↔ `e.node_id` (JS) — consistent. `NodeSummary` fields used in `renderSteps` (`node_id`, `type`, `status`) match `ui/contract.go`. `node.ID` (graph `Node`) used in the executor stamp is the same id surfaced as `NodeRun.NodeID` → `NodeSummary.node_id` the drawer filters on.
