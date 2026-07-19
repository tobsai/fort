# Fort Dashboard Redesign Implementation Plan (spec 033)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the web board with the six-view delegation-model dashboard from `design_handoff_fort_dashboard_redesign/`, plus the four small backend additions it needs (gate-decision events + reject notes, board timestamps/checkpoints, backlog reassign, metrics rollups).

**Architecture:** `ui/page.go` stays one Go raw-string HTML const served at `GET /` over the existing HTTP/SSE API; `ui` keeps its ports discipline (imports only `core/store` + `core/task`). Backend additions are additive JSON + one new event type; nothing removes or renames existing fields.

**Tech Stack:** Go 1.26 stdlib, SQLite via existing store, vanilla HTML/CSS/JS, goja (tests only).

## Global Constraints

- `boardHTML` is a Go **raw string**: NO literal backtick anywhere in the page (JS uses `String.fromCharCode(96)`; write JS without template literals).
- The `/* md:start */ … /* md:end */` region must survive **verbatim** (copy from current `ui/page.go:160-200`): `ui/md_test.go` extracts it by `strings.Index(boardHTML, "/* md:start")` / `"/* md:end */"` and runs it under goja. Keep it self-contained; keep it the ONLY innerHTML source for user/agent-authored bodies.
- `/api/summary` and `/api/board` raw bodies must never contain the substring `null` (ui_test.go:218). Every new slice field: initialize `[]`, every new struct field: value types or omitempty-safe.
- Additive JSON only; never remove/rename keys (FortKit + gateway compat).
- Amber (#e0a458) never animates. Progress is never an agent-estimated %.
- Where README.md and the mockup disagree, the **mockup** wins. Known cases (from full extraction): on-amber text `#12100a`, on-blue `#07101f`; working ring is a circle (static rings are rounded squares); not-started bar segments filled `#212938`; idle schedule blocks border `1px #1a212e` text `#4a5262`; sheen highlight `#e4efff`; slipping card border `#3a3020`; 1c has a 4th checkpoint-dot state (outlined `#e0a458` = awaiting sign-off); "That's everything —" line renders below pending cards too.
- Sigil fill = the project's **status color** (not brass — the brass in the README code comment is wrong).
- The user's real daemon runs on **127.0.0.1:4087 under launchd — never pkill fort, never bind 4087**; demo server uses :4099, kill by saved PID.
- Vocabulary on ALL user-facing copy: assignment (run/task), sign-off / "Needs you" (gate), Up next / Start (backlog / dispatch), Draft a plan (breakdown), project plan · checkpoint (flow / node), agent toolbox shown at assignment only.
- Agent display names: `claude`→Claude Code, `codex`→Codex, `hermes`→Hermes, `openclaw`→OpenClaw; flow runs display under the agent of their latest `started` event, else "Fort".

---

### Task 1: Spec + plan docs

Done when this file and `specs/033-dashboard-redesign.md` exist on the branch.

- [x] Write both docs
- [ ] **Commit** `docs(spec): 033 dashboard redesign — spec + implementation plan`

---

### Task 2: Gate decisions become events; reject carries a note

**Files:**
- Modify: `core/graph/executor.go` (decideGate ~:56-81, Reject signature)
- Modify: `ui/ports.go` (FlowRunner.Reject), `ui/contract.go` (GateDecision.Note), `ui/server.go` (handleGate ~:174-208)
- Modify: `control/control.go` (FlowExecutor.Reject ~:106-113)
- Modify: existing callers in tests that use `Reject(run, node)`
- Test: `core/graph/executor_test.go`, `ui/ui_test.go`

**Interfaces:**
- Produces: `graph.Executor.Reject(runID, nodeID, note string) error`; event rows `{Type:"gate", RunID, NodeID, Data:"{\"decision\":\"approved|rejected\",\"note\":\"…\"}"}` appended on every decision (approve note = the edit text); `ui.FlowRunner.Reject(runID, nodeID, note string) error`; `ui.GateDecision.Note string \`json:"note,omitempty"\``.

- [ ] **Step 1: failing test (graph)** — in `core/graph/executor_test.go`:

```go
func TestGateDecisionsAppendEvents(t *testing.T) {
	x, st := newTestExecutor(t) // follow the file's existing helper pattern
	runID := "r1"
	startFlowToGate(t, x, runID) // reuse existing helper that pauses at plan_gate
	if err := x.Reject(runID, "plan_gate", "tighten the scope"); err != nil {
		t.Fatal(err)
	}
	if err := x.Approve(runID, "plan_gate", "ship it smaller"); err != nil {
		t.Fatal(err)
	}
	evs, _ := st.Events(runID)
	var gates []store.Event
	for _, e := range evs {
		if e.Type == "gate" {
			gates = append(gates, e)
		}
	}
	if len(gates) != 2 {
		t.Fatalf("want 2 gate events, got %d", len(gates))
	}
	if gates[0].NodeID != "plan_gate" || !strings.Contains(gates[0].Data, `"rejected"`) || !strings.Contains(gates[0].Data, "tighten the scope") {
		t.Fatalf("bad reject event: %+v", gates[0])
	}
	if !strings.Contains(gates[1].Data, `"approved"`) || !strings.Contains(gates[1].Data, "ship it smaller") {
		t.Fatalf("bad approve event: %+v", gates[1])
	}
}
```

(Adapt helper names to what the file actually has — write the minimal local helpers if none fit.)

- [ ] **Step 2:** `go test ./core/graph/ -run TestGateDecisionsAppendEvents` → FAIL (Reject arity / no gate events)
- [ ] **Step 3: implement** in `executor.go`:

```go
func (e *Executor) Approve(runID, nodeID, edited string) error {
	return e.decideGate(runID, nodeID, "approved", edited)
}

// Reject records a rejection; note is the human's redirect note ("" = none).
func (e *Executor) Reject(runID, nodeID, note string) error {
	return e.decideGate(runID, nodeID, "rejected", note)
}

func (e *Executor) decideGate(runID, nodeID, status, text string) error {
	// … existing upsert logic unchanged …
	// append-only decision record (spec 033): survives the node_run upsert.
	data, _ := json.Marshal(map[string]string{"decision": status, "note": text})
	_, _ = e.store.AppendEvent(store.Event{RunID: runID, NodeID: nodeID, Type: "gate", Data: string(data)})
	return nil // keep the existing error flow — append after the successful upsert
}
```

Then ripple the `Reject` arity: `ui/ports.go` `Reject(runID, nodeID, note string) error`; `control/control.go` forwards `note`; `ui/server.go` `case "reject": err = s.d.Runner.Reject(dec.RunID, dec.NodeID, dec.Note)`; `ui/contract.go` adds `Note string \`json:"note,omitempty"\`` to GateDecision; fix any test fakes implementing FlowRunner.

- [ ] **Step 4: failing test (ui)** — in `ui/ui_test.go` add `TestGateRejectRecordsNote`: POST `/api/gate` `{"run_id":…,"node_id":"plan_gate","decision":"reject","note":"smaller please"}` on the full-UI harness, then assert `GET /api/runs/{id}` events contain a `gate` event whose data contains `smaller please`.
- [ ] **Step 5:** `go test ./core/graph/ ./ui/ ./control/` → PASS (whole packages, catches ripples)
- [ ] **Step 6: Commit** `feat(core): gate decisions are append-only events; reject carries a note (spec 033)`

---

### Task 3: Board payload — timestamps, gate since, checkpoint summaries

**Files:**
- Modify: `core/store/store.go` (+`AllNodeRuns`), `ui/contract.go`, `ui/ports.go` (+`Plan`), `ui/server.go` (handleBoard/handleSummary), `control/control.go` (FlowExecutor.Plan)
- Test: `core/store/store_test.go`, `ui/ui_test.go`

**Interfaces:**
- Produces:
  - `store.AllNodeRuns() ([]NodeRun, error)` — all rows ordered by run_id, created_at.
  - `ui.FlowNode{ID, Type string}`; `ui.FlowRunner.Plan(flowID string) []FlowNode` (unknown id ⇒ nil).
  - `ui.RunSummary` += `CreatedAt string \`json:"created_at"\``, `UpdatedAt string \`json:"updated_at"\`` (RFC3339, always set), `Checkpoints *CheckpointSummary \`json:"checkpoints,omitempty"\``.
  - `ui.CheckpointSummary{Total, Accepted, Waiting, Rejected, Done int}` — gates only: Total = gate nodes in the plan (fallback: gate node_runs seen), Accepted = approved, Waiting = waiting, Rejected = rejected, Done = non-gate nodes succeeded (for "in progress" inference client-side).
  - `ui.GateItem` += `Since string \`json:"since,omitempty"\`` (gate node_run CreatedAt, RFC3339).

- [ ] **Step 1: failing store test** `TestAllNodeRuns` — upsert node runs across two run ids, assert both returned grouped/ordered.
- [ ] **Step 2:** run → FAIL. **Step 3:** implement (`SELECT … FROM node_run ORDER BY run_id, created_at`). **Step 4:** PASS.
- [ ] **Step 5: failing ui test** `TestBoardCarriesTimestampsAndCheckpoints`: full harness, start `ship X` flow (pauses at plan_gate); `GET /api/board` → the flow run has non-empty `created_at`/`updated_at`, `checkpoints.total==3` (ship-feature gates: plan_gate, merge_gate, escalate), `checkpoints.waiting==1`; gates[0].since non-empty. Also assert raw body still has no `"null"`.
- [ ] **Step 6:** implement: `control.FlowExecutor.Plan` walks its flow map → `[]ui.FlowNode`; `handleBoard` fetches `AllNodeRuns()` once, buckets by run, merges with `Plan(run.FlowID)` when Runner non-nil. Timestamps via `r.CreatedAt.UTC().Format(time.RFC3339)`.
- [ ] **Step 7:** `go test ./ui/ ./core/store/ ./control/` → PASS. **Step 8: Commit** `feat(ui): board carries run timestamps, gate since, checkpoint summaries (spec 033)`

---

### Task 4: PATCH /api/backlog/{id} — reassign agent

**Files:** `core/store/backlog.go`, `ui/server.go`, `ui/contract.go`; tests in `core/store`, `ui/backlog_test.go`.

**Interfaces:** `store.UpdateBacklogAgent(id, agent string) error` (missing id ⇒ error); `PATCH /api/backlog/{id}` body `{"agent":"codex"}` → 200 updated `BacklogItem` (404 unknown id; empty agent allowed = clear pin).

- [ ] **Step 1: failing tests** — store: create item, `UpdateBacklogAgent(id,"codex")`, Get shows codex; unknown id errors. ui: `TestBacklogReassign` POST item → PATCH agent → GET list shows new agent; PATCH unknown id → 404.
- [ ] **Step 2:** FAIL. **Step 3:** implement (`UPDATE backlog_item SET agent=? WHERE id=?`, check RowsAffected; route `mux.HandleFunc("PATCH /api/backlog/{id}", …)`). **Step 4:** PASS. **Step 5: Commit** `feat(ui): backlog items can be reassigned (PATCH /api/backlog/{id}) (spec 033)`

---

### Task 5: GET /api/metrics — per-agent scorecards

**Files:** Create `ui/metrics.go`, `ui/metrics_test.go`; modify `ui/server.go` (route), `ui/contract.go` (types).

**Interfaces (exact wire types in `ui/contract.go`):**

```go
type AgentMetrics struct {
	Agent          string    `json:"agent"`
	Assignments    int       `json:"assignments"`
	Decided        int       `json:"decided"`         // sign-offs that reached a decision
	FirstPass      int       `json:"first_pass"`      // approved, no prior reject, empty note
	FirstPassPct   float64   `json:"first_pass_pct"`  // 0 when Decided==0
	Redirects      int       `json:"redirects"`       // rejects + approves-with-edits
	RedirectsPer   float64   `json:"redirects_per_assignment"`
	CostUSD        float64   `json:"cost_usd"`         // parsed claude result lines; 0 = unknown
	CostPerAccept  float64   `json:"cost_per_accepted"`// 0 = unknown
	CostKnown      bool      `json:"cost_known"`
	Trend          string    `json:"trend"`            // improving | steady | slipping
	Spark          []float64 `json:"spark"`            // 7 buckets, first-pass % carried forward
	Best           []string  `json:"best"`             // matched_rule lanes, ≥3 runs, top success
	Weak           []string  `json:"weak"`
}
type MetricsResponse struct {
	WindowDays  int            `json:"window_days"`
	Assignments int            `json:"assignments"`
	Agents      []AgentMetrics `json:"agents"`
	Lanes       []string       `json:"lanes"` // distinct matched_rule values (filter options)
}
```

**Algorithm (pin):** window = `?days` clamp 1..365 default 30, cutoff = now−days.
Scan `ListRuns()` (window-filter) + all events per windowed run (`Events(runID)`).
- *Assignment* = a non-flow run (agent has no `flow:` prefix) in window, attributed to its agent; plus each distinct `(run,node)` with a `started` event where NodeID≠"" (flow task nodes), attributed to that event's Data.
- *Decisions*: `gate` events; attribute each to the Data (agent) of the latest `started` event in the same run with smaller event ID (skip if none). First-pass per distinct `(run,node)`: final decision approved, no earlier rejected for that pair, and empty note on the approving event. Redirect = every rejected event + every approved event with non-empty note.
- *Cost*: `stdout` events whose Data parses as JSON with `"type":"result"` and numeric `total_cost_usd`; attribute like decisions (or to the run's agent for non-flow runs). CostKnown = any parsed.
- *Trend*: split window at midpoint; halves' first-pass pct; Δ≥+5 improving, ≤−5 slipping, else steady; steady if either half decided<3.
- *Spark*: 7 equal time buckets; bucket value = first-pass pct of decisions in bucket, carry previous value forward when empty (start 0).
- *Best/Weak*: non-flow runs grouped by MatchedRule (skip ""); lanes with ≥3 terminal runs: score = succeeded/(succeeded+failed); best = up to 2 top with score ≥ .7; weak = up to 1 with score < .6. Labels humanize `-`/`_` → space.
- Sort agents by Assignments desc. Slices always non-nil (`[]float64{}`, `[]string{}`).

- [ ] **Step 1: failing test** `ui/metrics_test.go` — build a real `store.Store` fixture in a temp dir: two agents; claude: 3 runs (2 succeeded lane `lane-feature`, 1 failed), a flow run with `started(node)` events + `gate` events (reject w/ note then approve, and one clean approve on a second gate) + one stdout result row `{"type":"result","total_cost_usd":1.5}`; codex: 2 runs. Assert: claude Assignments (3 + node executions), Decided 2, FirstPass 1, FirstPassPct 50, Redirects 2, CostKnown true, CostUSD 1.5, spark length 7, response.Lanes contains `lane-feature`, raw body contains no `null`.
- [ ] **Step 2:** FAIL. **Step 3:** implement `ui/metrics.go` (pure function `computeMetrics(runs []store.Run, events func(string) ([]store.Event, error), now time.Time, days int) MetricsResponse` + thin handler) so the test can inject `now`. **Step 4:** PASS + `go test ./ui/`. **Step 5: Commit** `feat(ui): /api/metrics — per-agent scorecard rollups from the event log (spec 033)`

---

### Task 6: Page foundation (tokens, nav, sigil, md, drawer)

**Files:** rewrite `ui/page.go`.

The remaining tasks are one growing artifact; each task ends with: `go test ./ui/` green (md region intact) + a visual check against the mockup section on the seeded demo server (`FORT_FAKE=1 … FORT_ADDR=127.0.0.1:4099`, seeding runbook in scratchpad notes / plan appendix).

**Structure of the new const (in order):**
1. `<head>`: theme bootstrap script (keep current one verbatim), Google Fonts preconnect + `Instrument+Sans:wght@400;500;600;700` + `Spline+Sans+Mono:wght@400;500;600`, `<style>` tokens.
2. Design tokens (dark = source of truth from README §Design Tokens + mockup):

```css
:root{
  --bg:#0b0e14;--panel:#12161f;--line:#1a212e;--line2:#212938;--raise:#26314a;--outline:#303848;
  --fg:#e8ebf2;--body:#b8bfce;--mut:#8b93a5;--faint:#687183;--dis:#4a5262;
  --brass:#c9a35c;--brass2:#dcb877;--work:#6fa8ff;--need:#e0a458;--ok:#57b98a;--bad:#d96a6a;
  --queued:#2a3650;--sched:#56617a;--seg0:#212938;--sheen:#e4efff;--slip-border:#3a3020;
  --on-brass:#0b0e14;--on-amber:#12100a;--on-blue:#07101f;--on-green:#07120c;
  --font:'Instrument Sans',system-ui,sans-serif;--mono:'Spline Sans Mono',ui-monospace,Menlo,monospace;
}
:root[data-theme=light]{
  --bg:#f4f5f7;--panel:#ffffff;--line:#e4e7ec;--line2:#d8dce3;--raise:#c6cfdd;--outline:#b6bfcc;
  --fg:#1b2230;--body:#3a4658;--mut:#68738a;--faint:#8b94a7;--dis:#aab2c0;
  --brass:#9a7b2e;--brass2:#7a5f16;--work:#2b6fd0;--need:#b07d10;--ok:#2f8f5a;--bad:#c23b34;
  --queued:#dde5f2;--sched:#98a2b8;--seg0:#e4e7ec;--sheen:#f3f7ff;--slip-border:#e2d2b0;
  --on-brass:#ffffff;--on-amber:#231a04;--on-blue:#0d1b33;--on-green:#06251ા6;
}
```
   (Fix the typo above when transcribing: light `--on-green:#062516`.) Base body: `font:14px/1.5 var(--font)`; mono only for data. Keyframes exactly: `spinrace{to{transform:rotate(360deg)}}`, `sheen{to{background-position:-220% 0}}`, `dotpulse{0%,100%{opacity:1}50%{opacity:.35}}` + `@media (prefers-reduced-motion: reduce){*{animation:none!important}}`.
3. Top bar (padding 14px 22px, border-bottom 1px var(--line)): brass `FORT` wordmark (mono 700 15px, ls .22em) · nav chips `Deck·Projects·Assign·Performance·Week·Today` (13px; active = brass tint pill) · "N need you" pill (only when N>0) · spacer · machine dots (7px, mono 12px) · plane badge when control-only (`control only`, mono 10px outline) · `Give direction` button (bg var(--brass), on-brass, 600 13px, radius 8) → Assign view · theme toggle (existing ◐ button).
4. Six `<section class="view" id="v-deck|v-projects|v-assign|v-perf|v-week|v-today">`; router `showView(name)` toggles `hidden`, stores `localStorage['fort-view']`, per-view lazy render functions run on each `refresh()`.
5. **md region**: paste current `ui/page.go` lines 160–200 EXACTLY (from `/* md:start` through `/* md:end */`).
6. Sigil (verbatim algorithm from the mockup, DOM string not React):

```js
function sigil(name,size,color){
  var h=2166136261;
  for(var i=0;i<name.length;i++){h^=name.charCodeAt(i);h=Math.imul(h,16777619)>>>0;}
  function rnd(){h^=h<<13;h^=h>>>17;h^=h<<5;h>>>=0;return h/4294967296;}
  var cells=[]; for(var x=0;x<3;x++)for(var y=0;y<5;y++){if(rnd()>0.55){cells.push([x,y]);if(x<2)cells.push([4-x,y]);}}
  var u=size/5, r='';
  for(var i=0;i<cells.length;i++){r+='<rect x="'+(cells[i][0]*u+u*0.04)+'" y="'+(cells[i][1]*u+u*0.04)+'" width="'+(u*0.88)+'" height="'+(u*0.88)+'" rx="'+(u*0.2)+'" fill="'+color+'"/>';}
  return '<svg width="'+size+'" height="'+size+'" viewBox="0 0 '+size+' '+size+'" aria-hidden="true" style="display:block">'+r+'</svg>';
}
function ringWrap(name,size,state){ /* state: working|need|ok|idle */
  var col={working:'#6fa8ff',need:'#e0a458',ok:'#57b98a',idle:'#303848'}[state];
  var fill={working:'#6fa8ff',need:'#e0a458',ok:'#57b98a',idle:'#56617a'}[state];
  if(state==='working') return '<span class="ring ring-work" style="border-color:'+col+'"><span class="race"></span>'+sigil(name,size,fill)+'</span>';
  return '<span class="ring" style="border-color:'+col+';border-radius:'+(size>36?10:8)+'px;padding:'+(size>36?5:4)+'px">'+sigil(name,size,fill)+'</span>';
}
```
CSS: `.ring{display:inline-block;border:2px solid;line-height:0}` `.ring-work{position:relative;border-radius:50%;padding:8px}` `.race{position:absolute;inset:-2px;border-radius:50%;background:conic-gradient(rgba(255,255,255,0) 0 70%,#fff 82%,rgba(255,255,255,0) 94%);-webkit-mask:radial-gradient(farthest-side,transparent calc(100% - 4px),#000 calc(100% - 3px));mask:radial-gradient(farthest-side,transparent calc(100% - 4px),#000 calc(100% - 3px));animation:spinrace 2.2s linear infinite}`.
7. State + data layer: keep current polling (`refresh()` every 3s + SSE-triggered), add `/api/metrics` fetch (60s + on view open), keep activity buffers (spec 030) and prune logic; shared derived model `model = {runs, gates, backlog, machines, metrics}` + helpers: `dispName(agent)`, `runAgent(run)` (flow attribution: latest started event agent, tracked from SSE + run detail fetch fallback → cache), `elapsed(ts)`, `ago(ts)`, `projectState(run)` (need|working|ok|idle|failed), `esc`, `md`.
8. Drawer: keep current markup/behavior (`openDrawer`, steps, per-node log filter) restyled with tokens; ADD a gate action bar when the run has a waiting gate: Approve (green) / `Request changes…` (reveals a textarea + Send, posts `{decision:"reject", note}`) — reuse from Deck.

- [ ] Steps: write foundation with all six views stubbed (`<div class="empty">`), wire router/theme/SSE; `go test ./ui/` (md tests) → PASS; launch seeded demo; screenshot header/nav both themes; **Commit** `feat(ui): dashboard redesign foundation — tokens, nav, sigils, drawer (spec 033)`

---

### Task 7: Deck view (1a) — mockup lines 251–309

Left pane (flex 1.5): label `NEEDS YOU` (12px 600 uppercase ls .09em, --need). Cards (bg panel, border 1px raise, border-left 3px --need, radius 10, padding 16 18):
- One per waiting gate: title 16/600 = `Sign off on the plan` (node id humanized: strip `_gate`, `plan`→"the plan", `merge`→"the merge", `escalate`→`An assignment needs direction`; fallback `Sign off: {id}`); right mono 11.5 `ago(gate.since)`; body 13.5/1.55 --body: `{dispName} is waiting on ` + `<strong class="proj">` run title `</strong>` + gate input first line (via md → text, ≤140 chars); actions: `Approve` (bg --ok, on-green) / `Request changes…` (outline --outline, reveals note textarea + `Send` → POST gate reject with note) / `View the plan` (text-only --mut → drawer).
- One per failed run (last 48h): border-left --bad; title `{title} hit a wall`; body = run error excerpt; single action `View what happened` → drawer.
- Terminator line 12.5 --faint below cards: with items: `That's everything else — {k} agents are working and don't need you.`; empty: `That's everything — {k} agents are working and don't need you.` (k = working agents; if k==0 and nothing pending: `All quiet — nothing needs you.`).

Right pane (flex 1, border-left 1px --line): `PROJECTS` label (--mut) + rows (sigil 30 ringWrap + name 14/600 + caption 12 --mut): flow runs newest-first + running/blocked plain runs + backlog briefs (idle ring, caption `Not started · brief drafted`), cap 6. Captions: flows `"{a} of {t} checkpoints accepted · {w} awaiting sign-off"` / `"{a} of {t} accepted · {dispName} {verbing}"`; plain running `"{dispName} on it · {elapsed}"`. Click → drawer (runs) / Assign prefill (briefs).
`CREW` label (margin-top 10) + rows (8px dot: working = --work + dotpulse; waiting = --need static; idle = --outline): `<strong>{dispName}</strong>` + activity 13px --mut (latest SSE buffer line as plain text, else `working — {title} · {elapsed}` / `waiting on your sign-off` / `idle`).

Header pill: `{n} need you` (n = gates + recent failed).

- [ ] Render + actions wired; refresh-safe (no focus loss while typing a note: skip re-render of an open note editor); `go test ./ui/`; screenshots vs mockup; **Commit** `feat(ui): Deck view — needs-you inbox, projects, crew (spec 033)`

---

### Task 8: Projects view (1b) — mockup lines 311–375

Header center: `{p} projects · {w} agents working` (13px --mut); right `＋ New brief` outlined brass → Assign. Grid 2×2 (gap 16, padding 22). Card (panel, radius 12, padding 20, gap 14): border by state — need: 1px raise; working/delivered: 1px --line; failed: 1px --bad at 40%; brief: 1px **dashed** --outline. Header: ringWrap 42 · name 17/600 · sub 12.5 --mut (`{agents} · on {machine}` / `{dispName} · finished {ago}` / `Brief drafted · no one assigned` in --faint) · pill right (need/working/delivered/failed tints at 13% alpha).
Checkpoint bar (flows only): equal-flex 8px segments radius 4 — per gate: approved `--ok`, waiting `--need`, rejected `--bad`; if run running append one `--work` segment; pad to plan total with `--seg0`. Caption 12.5 --mut with colored inline spans: `“{a} accepted · 1 awaiting your sign-off · {n} not started”` pattern; all-done: `All {t} checkpoints accepted`. Plain runs: no bar; caption `Direct assignment — no checkpoints`.
Activity sentence 13.5/1.5 --body: latest buffer/message line, else status-derived. Brief cards: quoted first line of body in curly quotes, --faint.
One CTA max: need → `Review the plan` (filled --need, on-amber) → drawer; working → `Watch the work` (outline) → drawer; brief → `Assign an agent` (outline brass) → Assign prefill; delivered/failed → none (card click opens drawer).

- [ ] Render + CTAs; tests; screenshots; **Commit** `feat(ui): Projects view — sigil cards with honest checkpoint bars (spec 033)`

---

### Task 9: Assign view (1c) — mockup lines 377–427

Left (flex 1, padding 24, border-right): `Give direction` 18/600; textarea (panel bg, raise border, radius 10, min-height 110, 14.5px) placeholder `Describe the outcome you want — like briefing an employee.` + md preview below (reuse pattern); `ASSIGN TO` chips — `Fort decides` (selected default: 1.5px brass border, brass tint) + agent chips (from union(machines, runs, metrics)); when machines>1 a second mono chip row `on: any machine | {names}`; toggle `Propose a plan first — I'll sign off before work starts` (34×20 pill, ON default; knob animates; OFF = outline track); submit `Hand it off` (brass, 600 14.5, radius 9, padding 11 22).
Submit logic: prefill-mode (from briefs) → PATCH agent (if chip picked) + dispatch that backlog id; else toggle ON → POST `/api/breakdown` {text, agent, machine} (409 → alert `Drafting a plan needs the execution plane — start fort serve.`); toggle OFF → POST `/api/chat`. Secondary text action under submit: `or add to Up next` → POST `/api/backlog`.
Right (flex 1.2): `THE ROSTER` label; per-agent cards (panel, radius 10, padding 14 16, border-left 3px status): name 15/600 + status pill (`waiting on you` amber tint / `working · 12m` blue tint / `idle` line2 bg) + machine mono right; assignment sentence 13.5 --body; checkpoint dots 9px (gap 5): filled --ok accepted · filled --work + dotpulse current · outlined 1.5px --need awaiting sign-off · outlined 1.5px --outline future; trailing `checkpoints` 11.5 --faint. Idle: dimmed name + `Assign work` outlined brass (chip-selects that agent, focuses textarea).

- [ ] Render + all submit paths (incl. control-only 409s); tests; screenshots; **Commit** `feat(ui): Assign view — give direction, plan-first toggle, roster (spec 033)`

---

### Task 10: Performance view (2a) — mockup lines 96–169

Header: `Crew performance` · `last {days} days · {n} assignments` · right filter select `All task types ▾` (options = metrics.Lanes; filtering client-side re-requests `/api/metrics?lane=` — NO: filter is client-side over per-lane data we don't have per agent ⇒ implement server param `?lane=` filtering assignments/best-weak only; keep simple: re-fetch with `&lane=`). Grid 2×2. Card (panel, 1px --line, radius 12, padding 18 20; slipping: border --slip-border): name 16/600 · trend right (`▲ improving` --ok 600 / `→ steady` --mut / `▼ slipping` --bad 600; NO method chip — deferred, spec 033). Metric row (gap 22): mono 600 22px numerals — first-pass % (trend-colored) with `·{decided} signed off` sample sub-label, redirects/assignment, `$x.xx` or `—` per accepted checkpoint; labels 11.5 --faint (`first-pass accepted` / `redirects / assignment` / `per accepted checkpoint`); sparkline `<svg width=90 height=34>` polyline stroke-width 2 trend color, points from spark buckets mapped y=30−(v/100)*26.
Chips row 11.5: `best at: {lane}` green tint (+ second lane), `weak: {lane}` line2/--faint; omit when empty. Footer 12.5 --mut: improving `First-pass acceptance up {Δ} pts across the window.` · slipping `First-pass acceptance down {Δ} pts — steer earlier or split the work smaller.` · steady `Holding steady on {decided} sign-offs.` · no data `No sign-offs in this window yet — numbers appear after your first accepts.`
Empty state (no agents): centered `No assignments in the last {days} days.`

- [ ] Extend metrics endpoint with optional `?lane=`; render; tests; screenshots; **Commit** `feat(ui): Performance view — honest scorecards over /api/metrics (spec 033)`

---

### Task 11: Week view (2b) — mockup lines 171–210

Header: `The week ahead` · mono `{Jul 20 – 26}` (current Mon–Sun) · legend right (10×10 radius-3 swatches): active now --work / up next --queued / waiting `#e0a458`. Grid `130px repeat(7,1fr)` gap `8px 6px`; day headers mono 600 11px, today `{DAY} ·today` brass. One row per agent (idle names --mut).
Blocks 36px radius 7 (shared classes from foundation): active (sheen gradient, on-blue, 600) spans creation-day→today, label = run title; waiting `⏸ on you` span 1 at gate-since day (600, on-amber) followed by queued block `{title} — once approved` spanning 2; up-next (backlog with agent): --queued blocks placed after today sequentially span 2, label title, `draggable=true`; idle rows: `open capacity — assign work` block (border 1px --line, --dis, italic, centered, spans remaining week from today) → Assign.
Drag: dragstart stores item id; agent rows are drop targets (highlight row on dragover); drop → `PATCH /api/backlog/{id}` `{agent}` + refresh. Recurring/scheduled blocks: none (deferred — no scheduler listing; keep the dashed style defined in CSS for future use).
Footer caption 12.5 --faint: `"Up next" blocks are your Ready queue, ordered — drag between rows to reassign.`

- [ ] Render + drag; tests; screenshots; **Commit** `feat(ui): Week view — per-agent schedule with drag reassign (spec 033)`

---

### Task 12: Today view (3a) — mockup lines 38–83

Header: `Today` · mono `{Tue Jul 21} · now {11:40}` · right day summary 12.5 --mut (derived: `{g} sign-offs waiting · {m} more expected today` / `{g} sign-offs expected before {h}pm · evening is clear` when none after 17:00 / `nothing needs you yet — the crew is heads-down`).
Grid `130px repeat(12,1fr)` gap `8px 4px`, hours 8–19 headers mono 600 10.5px (12-hour labels `8 9 10 11 12 1 …`), current hour brass. NOW line: absolute 2px --bad, `left:calc(130px + (100% - 152px - 130px) * {frac} + 22px)` with `{frac}=(now−8:00)/12h` clamped 0..1, mono 10px `NOW` label above; hidden outside 8:00–20:00.
Row 1 `You` (brass, 700): per waiting gate → solid amber span-1 `⚑ sign-off` at max(8:00, since); per prediction (running run with ETA in window, ≥2 duration samples) → dashed amber span-1 `~ review` at ETA hour. Agent rows: active block from max(8:00, created) to clamp(ETA, +30min, 20:00) with inline copy `{title} → sign-off ~{h}{am|pm}` (flows with pending future gate) or `{title} → done ~{h}` (plain; no ETA when samples<2 → span to now+1h, no arrow copy); queued `then: {next backlog title}` span 3 after; waiting-on-you span-2 `⏸ waiting on you` at since + queued `{title} — starts on your approval` span 5; idle span-rest `idle — assign work` → Assign.
ETA (pin): per-agent μ = mean(updated−created) of terminal non-flow runs (30d, clamp 10s..8h, ≥2 samples); remaining = max(μ−elapsed, 5min); ETA=now+remaining.
Footer caption: `Your row is derived, not planned: solid amber = a sign-off already waiting; dashed amber = a checkpoint an agent is on pace to reach. Answer them early and the crew's afternoon compresses left.`

- [ ] Render + ETA math (unit-testable pure JS kept small; verify by seeding runs with backdated created_at via sqlite3 on the DEMO db only); tests; screenshots; **Commit** `feat(ui): Today view — hour grid with derived You row (spec 033)`

---

### Task 13: Light theme + control-only + polish

- [ ] Sweep every view in light theme (screenshots) — adjust only light token values, never structure. Contrast: body text ≥ 4.5:1 on panels.
- [ ] Control-only (`fort control` harness): plane badge shows `control only`; Approve/Request-changes/Draft-a-plan surface 409 messages; Start still boards; Performance shows the no-data state.
- [ ] Focus rings on ALL interactive elements (`:focus-visible` outline 2px brass), hover lifts (border → --raise), cursor:pointer everywhere clickable, Escape closes drawer/note editors, buttons are real `<button>`s.
- [ ] `go test ./...` full; **Commit** `feat(ui): light theme, control-only degradation, a11y polish (spec 033)`

---

### Task 14: Verification & review

- [ ] `go test ./...` and `go test -race ./ui/ ./core/graph/ ./core/store/`
- [ ] Fresh seeded demo (runbook): screenshot all 6 views dark + light; compare against mockups side-by-side; fix fidelity gaps (px-level: paddings, radii, colors, copy).
- [ ] Gateway sanity: `curl -s 127.0.0.1:4099/ | head` renders static header/nav markup without JS (iframe srcDoc case shows a labeled shell, not a blank page).
- [ ] Adversarial review: dispatch reviewer subagents (correctness / XSS via innerHTML paths / seam discipline `go list -deps ./ui` shows no engine/graph/router/native/exec imports) and fix confirmed findings.
- [ ] Final: clean `git log` on branch; report to Toby (no merge/push — held for approval per repo discipline).

## Self-review notes

- Every README screen requirement maps to Tasks 6–12; deferrals (method chips, scheduler recurring blocks, non-claude cost) are documented in spec 033 and rendered as honest absence, not fabricated data.
- Type consistency: `FlowNode`, `CheckpointSummary`, `AgentMetrics` names used consistently above; `Reject(runID, nodeID, note string)` everywhere.
- No placeholder steps: backend tasks carry full code/tests; frontend tasks pin exact styles/copy to mockup line ranges in-repo (`design_handoff_fort_dashboard_redesign/Fort Redesign.dc.html`).
