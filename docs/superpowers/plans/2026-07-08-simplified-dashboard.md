# Simplified Dashboard (spec 031) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the 5-column kanban with a three-zone dashboard (Define · Ready · In progress) — multiline markdown compose with first-line-=-title, Start buttons instead of drag, inline gate approvals, and nested tool/subagent activity on live runs.

**Architecture:** T1 adds the one data change the compose needs: an additive `run.body` column (store → engine copy → wire), and the server-side first-line/rest split in `handleChat`. T2 rewrites the `boardHTML` layout in `ui/page.go` — keeping the header/machines bar/theme/md-region/agent-picker/drawer and replacing the board grid + single-line compose + drag JS with the three zones; the SSE handler upgrades from "poke refresh" to also accumulating a small per-run activity buffer so In-progress rows show live `tool`/`subagent` events (spec 030). T3 verifies, documents, and amends spec 031's non-goal line (which said "no new persistence" but its own test criteria require reading back a run's body — resolved in favor of the additive column).

**Tech Stack:** Go 1.26; `ui/page.go` Go raw-string constant (NO literal backticks — `BT`/`S0`/`S1` via `String.fromCharCode` only); existing goja tests must keep passing (do not touch the `/* md:start */…/* md:end */` region).

---

## File Structure

- `core/store/store.go` — `Run.Body` + `run.body` column (addColumn pattern) + insert/scan.
- `core/engine/engine.go` — `SubmitRef` copies `Body: t.Body` onto the run row.
- `ui/contract.go` — `RunSummary.Body` wire field.
- `ui/server.go` — `handleChat` first-line/rest split; `toRunSummary` carries Body (board + run detail).
- `ui/page.go` — the dashboard rewrite (T2).
- `specs/031-simplified-dashboard.md` — one-line non-goal amendment.
- `README.md` — Web bullet update.

---

### Task 1: run body — store, engine, wire, chat split

**Files:**
- Modify: `core/store/store.go` (Run struct ~line 32; run CREATE TABLE ~line 100; migrate ~line 128; CreateRun; scanRun/run SELECTs)
- Modify: `core/engine/engine.go` (`SubmitRef`'s `CreateRun` call, ~line 142)
- Modify: `ui/contract.go` (RunSummary ~line 28), `ui/server.go` (`handleChat`; the RunSummary constructions in `handleBoard` + `handleRunDetail`)
- Tests: `core/store/store_test.go`, `core/engine/engine_test.go` (or the existing engine test file), `ui/ui_test.go`

- [ ] **Step 1.1: Failing store test** — append to `core/store/store_test.go`:

```go
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
```

Run: `go test ./core/store/ -run TestRunBodyRoundTrip` → FAIL (no field `Body`).

- [ ] **Step 1.2: Implement store** — `Run` struct gains `Body string` after `Title`; the `run` CREATE TABLE gains `body TEXT` (after `title`); `migrate()` gains `s.addColumn("run", "body", "TEXT")` next to the other additive migrations; `CreateRun`'s INSERT adds the `body` column + `r.Body` value; every `SELECT` that scans runs (find them: `grep -n 'FROM run' core/store/store.go`) adds `body` in the same position, scanned via `sql.NullString` into `Body` (NULL-safe, mirroring `machine`/`node_id`). Keep column order identical across INSERT/SELECT/Scan.

Run: `go test ./core/store/ -race` → all PASS.

- [ ] **Step 1.3: Failing engine test** — append to the engine test file (locate: `ls core/engine/*_test.go`; use its existing helpers/fake pattern):

```go
func TestSubmitCopiesBodyToRun(t *testing.T) {
	// reuse the file's existing engine+fake+store constructor helper
	e, st := newTestEngine(t) // ADAPT to the file's actual helper name/signature
	runID, err := e.Submit(context.Background(), task.Task{ID: "tb", Title: "title line", Body: "body md"})
	if err != nil {
		t.Fatal(err)
	}
	e.Wait(runID)
	r, _ := st.GetRun(runID)
	if r.Body != "body md" {
		t.Fatalf("run body = %q, want body md", r.Body)
	}
}
```

(If no such helper exists, build engine.New(router.New(parsed-rules), fake.New(), store, t.TempDir()) inline following the file's existing tests.) Run → FAIL (`body = ""`).

- [ ] **Step 1.4: Implement engine copy** — in `core/engine/engine.go` `SubmitRef`, the `store.CreateRun(store.Run{...})` literal gains `Body: t.Body,` (and the placement-failure `CreateRun` a few lines above may stay body-less). Run → PASS. `go test ./core/engine/ -race` → all PASS.

- [ ] **Step 1.5: Failing ui test** — append to `ui/ui_test.go`:

```go
func TestChatSplitsTitleAndBody(t *testing.T) {
	s, _ := newControlUI(t)
	cd := &capturingDispatcher{}
	// ADAPT: construct the server the way other tests wire a capturing dispatcher
	// (see the existing chat tests in this file) so cd receives the task.
	_ = s
	rec := do(t, uiServerWith(t, cd), "POST", "/api/chat", ui.ChatRequest{Text: "fix the header\n# Details\n- step one"})
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
	if cd.last.Title != "fix the header" || cd.last.Body != "# Details\n- step one" {
		t.Fatalf("title=%q body=%q", cd.last.Title, cd.last.Body)
	}
}

func TestChatSingleLineHasNoBody(t *testing.T) {
	cd := &capturingDispatcher{}
	rec := do(t, uiServerWith(t, cd), "POST", "/api/chat", ui.ChatRequest{Text: "just a title"})
	if rec.Code != 200 || cd.last.Title != "just a title" || cd.last.Body != "" {
		t.Fatalf("code=%d title=%q body=%q", rec.Code, cd.last.Title, cd.last.Body)
	}
}
```

ADAPT the construction to this file's real pattern — read the existing chat test (`grep -n 'api/chat' ui/ui_test.go`) and mirror it exactly; the assertion (Title=first line, Body=rest-trimmed) is the fixed requirement. If `capturingDispatcher.last` lacks a Body field passthrough, it already captures the whole `task.Task` — use it. Run → FAIL.

- [ ] **Step 1.6: Implement the split + wire field** — in `ui/server.go` `handleChat`, where the task's title is set from the text (find: `grep -n 'func (s \*Server) handleChat' ui/server.go` and read the task-construction block): split ONCE at the first newline —

```go
	title, body := req.Text, ""
	if i := strings.IndexByte(req.Text, '\n'); i >= 0 {
		title, body = strings.TrimSpace(req.Text[:i]), strings.TrimSpace(req.Text[i+1:])
	}
```

use `title` everywhere the old code used the full text as the task title (INCLUDING the `matchFlow` input, so "ship X" flow matching keys on the first line) and set `Body: body` on the task. In `ui/contract.go`, `RunSummary` gains `Body string \`json:"body,omitempty"\`` (after Title). In `ui/server.go`, every `RunSummary{...}` construction (`handleBoard`, `handleRunDetail`) gains `Body: r.Body` / `Body: run.Body`. Run → PASS; `go test ./ui/` → all PASS.

- [ ] **Step 1.7: Full gates + commit**

```bash
go test ./... && go vet ./core/store/ ./core/engine/ ./ui/
git add core/store/store.go core/store/store_test.go core/engine/engine.go core/engine/*_test.go ui/contract.go ui/server.go ui/ui_test.go
git commit -m "feat(core,ui): run.body column + chat first-line/rest split (spec 031)"
```

---

### Task 2: the dashboard rewrite (`ui/page.go`)

**Files:**
- Modify: `ui/page.go` ONLY. Preserve UNTOUCHED: the `<head>` (theme init), header markup, machines bar, the whole `/* md:start */…/* md:end */` region, `toggleTheme`, `agentsFor`/`syncAgentOptions`, `edgeFor`, `stepIcon`, `openDrawer`/`closeDrawer`/`loadDrawer`/`renderSteps`/`selectStep`, `dispatchItem`, `decide`, the SSE/`setInterval` skeleton, drawer markup + drawer CSS, `.mdbody` CSS. NO literal backticks.

- [ ] **Step 2.1: Replace the board + compose markup** — delete the `<div class="board" id="board">…</div>` block and the `<div class="compose">…</div>` block; in their place:

```html
<div class="dash">
  <section>
    <div class="zonehead"><span>Define</span></div>
    <textarea id="msg" rows="3" placeholder="first line = title; the rest is the body (markdown)…"></textarea>
    <div id="preview" class="mdbody preview" hidden></div>
    <div class="define-actions">
      <select id="machine" title="target machine"><option value="">any machine</option></select>
      <select id="agent" title="agent"><option value="">auto agent</option></select>
      <span class="grow"></span>
      <button onclick="addToReady()">Add to Ready</button>
      <button onclick="breakdownTask()">Break down</button>
      <button class="run" onclick="runNow()">Run ▸</button>
    </div>
  </section>
  <section>
    <div class="zonehead"><span>Ready</span><span class="n" id="n-ready">0</span></div>
    <div class="zone" id="z-ready"></div>
  </section>
  <section>
    <div class="zonehead"><span>In progress</span><span class="n" id="n-progress">0</span></div>
    <div class="zone" id="z-progress"></div>
    <details class="recent"><summary>Recent <span class="n" id="n-recent">0</span></summary><div class="zone" id="z-recent"></div></details>
  </section>
</div>
```

- [ ] **Step 2.2: Replace the board CSS** — delete the rules for `.board`, `.col`, `.colhead`, `.col.running .colhead`, `.col.blocked .colhead`, `.col-body`, `.col.drop .col-body`, `.card.item` (drag cursors), and `.compose*`; add:

```css
  .dash{max-width:900px;margin:0 auto;padding:14px 18px;display:flex;flex-direction:column;gap:20px}
  .zonehead{display:flex;align-items:center;gap:8px;font-size:11px;letter-spacing:.05em;color:var(--mut);text-transform:uppercase;margin-bottom:9px}
  .zonehead .n{background:var(--line2);border-radius:20px;padding:0 6px;min-width:18px;text-align:center}
  .zone{display:flex;flex-direction:column;gap:9px}
  textarea#msg{width:100%;resize:vertical;min-height:64px;background:var(--panel);color:var(--fg);border:1px solid var(--line);border-radius:8px;padding:9px 11px;font:inherit;font-size:12.5px}
  .preview{border:1px dashed var(--line);border-radius:8px;padding:8px 11px;max-height:140px;overflow:auto}
  .define-actions{display:flex;gap:8px;margin-top:8px;align-items:center}
  .define-actions select{background:var(--panel);color:var(--fg);border:1px solid var(--line);border-radius:7px;padding:7px 9px;font:inherit;font-size:12px;cursor:pointer}
  .define-actions button{border-radius:7px;padding:7px 13px;font:inherit;font-size:12px;cursor:pointer;border:1px solid var(--line);background:var(--panel);color:var(--fg2)}
  .define-actions button.run{border-color:var(--brass);color:var(--brass2);background:transparent}
  .define-actions button:hover{background:var(--line2)}
  .startbtn{font-size:10.5px;padding:1px 9px;border-radius:5px;background:transparent;border:1px solid var(--brass);color:var(--brass2);cursor:pointer;margin-top:7px}
  .queuedtag{font-size:10px;color:var(--mut);border:1px solid var(--line);border-radius:5px;padding:0 6px;margin-top:7px;display:inline-block}
  .activity{margin-top:7px;display:flex;flex-direction:column;gap:2px;font-size:11px;color:var(--fg2)}
  .activity .a-tool{color:var(--mut)}
  .activity .a-sub{color:var(--block);padding-left:14px}
  .activity .a-msg{color:var(--fg2)}
  .recent summary{cursor:pointer;font-size:11px;letter-spacing:.05em;color:var(--mut);text-transform:uppercase;margin:4px 0 9px}
  .recent .n{background:var(--line2);border-radius:20px;padding:0 6px}
```

(`.card`, `.meta`, `.e-*` edges, `.gateact`, `.runbtn`, `.empty`, `.mdbody`, drawer CSS all remain.)

- [ ] **Step 2.3: Replace the rendering + actions JS** — remove `backlogCard`, `fill`, the whole `// ---- drag` block (`onDrag` + the `['queued',…].forEach` listeners), `runNow`/`addToBacklog`/`breakdownTask` (replaced below), and the board section of `refresh()`. `runCard` stays (used by Recent + queued awareness). `gateCard` is deleted (gates render inline). Add:

```js
// ---- live activity: per-run buffer fed by the SSE stream (spec 030) ----
const ACT_MAX=6;
let actByRun={};
function trackEvent(e){
  if(!e||!e.run_id)return;
  if(e.type!=='tool'&&e.type!=='subagent'&&e.type!=='message')return;
  const buf=actByRun[e.run_id]||(actByRun[e.run_id]=[]);
  buf.push(e); if(buf.length>ACT_MAX)buf.shift();
}
function activityLine(e){
  if(e.type==='tool'){
    let d={}; try{d=JSON.parse(e.data||'{}')}catch(err){}
    return '<div class="a-tool">🔧 '+esc(d.name||'tool')+(d.summary?' · '+esc(d.summary):'')+'</div>';
  }
  if(e.type==='subagent'){
    let d={}; try{d=JSON.parse(e.data||'{}')}catch(err){}
    return '<div class="a-sub">🤖 subagent'+(d.agent?' ('+esc(d.agent)+')':'')+(d.description?' · '+esc(d.description):'')+'</div>';
  }
  const t=(e.data||'').split('\n')[0];
  return t?'<div class="a-msg">💬 '+esc(t.length>120?t.slice(0,119)+'…':t)+'</div>':'';
}

// ---- zone renderers ----
function readyItem(b){
  return '<div class="card item e-neutral" tabindex="0" data-id="'+b.id+'">'+
    '<div class="title">'+esc(b.title)+'</div>'+
    (b.body?'<div class="mdbody">'+md(b.body)+'</div>':'')+
    '<div class="meta"><span class="src '+(b.source==='agent'?'agent':'')+'"></span>'+
    (b.agent?'<span class="ag">'+esc(b.agent)+'</span>':'')+(b.machine?'<span class="mc">'+esc(b.machine)+'</span>':'')+'</div>'+
    '<button class="startbtn" onclick="dispatchItem(\''+b.id+'\')">Start ▸</button></div>';
}
function queuedItem(r){
  return '<div class="card run-card e-neutral" tabindex="0" onclick="openDrawer(\''+r.id+'\')" onkeydown="if(event.key===\'Enter\')openDrawer(\''+r.id+'\')">'+
    '<div class="title">'+esc(r.title||r.id)+'</div>'+
    '<div class="meta"><span class="ag">'+esc(r.agent)+'</span>'+(r.machine?'<span class="mc">'+esc(r.machine)+'</span>':'')+'</div>'+
    '<span class="queuedtag">queued</span></div>';
}
function progressItem(r,gates){
  const g=gates.filter(x=>x.run_id===r.id);
  const acts=(actByRun[r.id]||[]).map(activityLine).join('');
  return '<div class="card run-card '+edgeFor(r.status)+'" tabindex="0" onclick="openDrawer(\''+r.id+'\')" onkeydown="if(event.key===\'Enter\')openDrawer(\''+r.id+'\')">'+
    '<div class="title">'+esc(r.title||r.id)+'</div>'+
    (r.body?'<div class="mdbody">'+md(r.body)+'</div>':'')+
    '<div class="meta"><span class="ag">'+esc(r.agent)+'</span>'+(r.machine?'<span class="mc">'+esc(r.machine)+'</span>':'')+
    '<span class="mc">'+esc(r.status)+'</span></div>'+
    (acts?'<div class="activity">'+acts+'</div>':'')+
    g.map(x=>'<div class="gateact"><span class="mc">gate · '+esc(x.node_id)+'</span>'+
      '<button class="ok" onclick="event.stopPropagation();decide(\''+x.run_id+'\',\''+esc(x.node_id)+'\',\'approve\')">approve</button>'+
      '<button class="no" onclick="event.stopPropagation();decide(\''+x.run_id+'\',\''+esc(x.node_id)+'\',\'reject\')">reject</button></div>').join('')+
    '</div>';
}
function zone(id,html,nid,count){$('#'+id).innerHTML=html||'<div class="empty">—</div>';$('#'+nid).textContent=count;}
```

And the new `refresh()` board section (replacing the old `fill(...)` calls; the summary/machines part of `refresh()` stays as-is):

```js
  const b=await (await fetch('/api/board')).json();
  const runs=b.runs||[], gates=b.gates||[];
  const items=await (await fetch('/api/backlog')).json()||[];
  const queued=runs.filter(r=>r.status==='queued');
  const live=runs.filter(r=>r.status==='running'||r.status==='blocked');
  const done=runs.filter(r=>r.status==='succeeded'||r.status==='failed'||r.status==='canceled');
  zone('z-ready',items.map(readyItem).join('')+queued.map(queuedItem).join(''),'n-ready',items.length+queued.length);
  zone('z-progress',live.map(r=>progressItem(r,gates)).join(''),'n-progress',live.length);
  zone('z-recent',done.map(runCard).join(''),'n-recent',done.length);
  if(dwRun) loadDrawer();
```

And the new actions + compose helpers:

```js
function splitMsg(){
  const t=$('#msg').value; const i=t.indexOf('\n');
  return i<0?{title:t.trim(),body:''}:{title:t.slice(0,i).trim(),body:t.slice(i+1).trim()};
}
async function runNow(){
  const el=$('#msg'); if(!el.value.trim())return; const text=el.value; el.value=''; renderPreview();
  await fetch('/api/chat',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({text,machine:$('#machine').value,agent:$('#agent').value})});
  refresh();
}
async function addToReady(){
  const {title,body}=splitMsg(); if(!title)return; $('#msg').value=''; renderPreview();
  await fetch('/api/backlog',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({title,body,machine:$('#machine').value,agent:$('#agent').value})});
  refresh();
}
async function breakdownTask(){
  const el=$('#msg'); if(!el.value.trim())return; const text=el.value; el.value=''; renderPreview();
  const r=await fetch('/api/breakdown',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({text,machine:$('#machine').value,agent:$('#agent').value})});
  if(r.status===409)alert('Breakdown needs an execution plane — start fort serve.');
  refresh();
}
function renderPreview(){
  const {title,body}=splitMsg(); const pv=$('#preview');
  if(!body){pv.hidden=true;pv.innerHTML='';return;}
  pv.hidden=false; pv.innerHTML='<strong>'+esc(title)+'</strong>'+md(body);
}
$('#msg').addEventListener('input',renderPreview);
$('#msg').addEventListener('keydown',e=>{if((e.metaKey||e.ctrlKey)&&e.key==='Enter')runNow();});
```

And upgrade the SSE handler (replace `es.onmessage=()=>refresh();`):

```js
es.onmessage=ev=>{try{trackEvent(JSON.parse(ev.data))}catch(err){} refresh();};
```

- [ ] **Step 2.4: Drawer upgrades** — in `loadDrawer`, after the `dw-sub` line, add a body render (add `<div class="mdbody" id="dw-body"></div>` to the drawer-head markup, under the title/sub div):

```js
  $('#dw-body').innerHTML=d.run.body?md(d.run.body):'';
```

In `renderLog`, replace the event-line construction so tool/subagent events render typed (reusing `activityLine`):

```js
  log.innerHTML=evs.map(e=>{
    if(e.type==='tool'||e.type==='subagent')return '<div class="ev">'+activityLine(e)+'</div>';
    return '<div class="ev"><span class="k">'+esc(e.type)+'</span> '+esc(e.data||'')+'</div>';
  }).join('');
```

- [ ] **Step 2.5: Update the top-of-file Go comment** (`boardHTML` doc, lines 3–10) to describe the dashboard (three zones, markdown compose, inline gates, activity nesting; kanban removed per spec 031).

- [ ] **Step 2.6: Build + test + serve-grep**

```bash
go build ./... || exit 1
go test ./ui/ -race   # md goja tests must still pass untouched
go build -o "$SCRATCH/fort-31" ./cmd/fort 2>/dev/null || go build -o /tmp/fort-31 ./cmd/fort
FORT_FAKE=1 FORT_DB=/tmp/dash31/fort.db FORT_ADDR=127.0.0.1:4088 /tmp/fort-31 serve >/tmp/dash31.log 2>&1 &
for i in $(seq 1 20); do curl -s 127.0.0.1:4088/api/summary >/dev/null 2>&1 && break; sleep 0.25; done
P=$(curl -s 127.0.0.1:4088/)
echo -n "kanban gone (data-col count 0): "; echo "$P" | grep -c 'data-col=' || true
echo -n "zones present: "; echo "$P" | grep -c 'id="z-ready"\|id="z-progress"\|id="z-recent"'
echo -n "textarea compose: "; echo "$P" | grep -c '<textarea id="msg"'
echo -n "md intact: "; echo "$P" | grep -c 'function md('
echo -n "drawer intact: "; echo "$P" | grep -c 'id="drawer"'
# functional: multiline chat -> run title is first line + body stored
curl -s -X POST 127.0.0.1:4088/api/chat -H 'content-type: application/json' -d '{"text":"dash title\n# body md"}' >/dev/null; sleep 1
curl -s 127.0.0.1:4088/api/board | grep -c '"title":"dash title"'
kill %1 2>/dev/null; rm -rf /tmp/dash31 /tmp/fort-31
```

Expected: 0 / 3 / 1 / 1 / 1 / 1.

- [ ] **Step 2.7: Commit**

```bash
git add ui/page.go
git commit -m "feat(ui): three-zone dashboard replaces the kanban (spec 031)"
```

---

### Task 3: spec amendment + docs + full gates

- [ ] **Step 3.1: Amend spec 031's non-goal** — in `specs/031-simplified-dashboard.md`, replace the line `- **No new persistence.** Reuses …` bullet's first sentence with: `- **No new persistence beyond one additive column.** Runs gain a nullable `body` (the compose split must be readable back — its own test criteria require it); everything else reuses existing endpoints/tables.` (Keep the rest of the bullet.)

- [ ] **Step 3.2: README** — the Web bullet: replace the kanban description sentence (`a kanban board (Backlog · Queued · Running · Blocked · Done) with a backlog you drag onto the board to dispatch, an agent/machine picker on the compose bar, gate approvals inline in Blocked, and a light/dark theme toggle.`) with: `a three-zone dashboard (Define · Ready · In progress) — markdown compose with breakdown, Start buttons on ready work, live runs with nested tool/subagent activity and inline gate approvals, plus a light/dark theme toggle.` Keep the drawer + markdown sentences that follow.

- [ ] **Step 3.3: Full gates**

```bash
go test ./... && go test -race ./core/store/ ./core/engine/ ./ui/... && go vet ./...
go list -deps ./ui | grep -E 'core/engine|core/graph|core/router|exec/native' || echo "seam clean"
```

Expected: green + `seam clean`.

- [ ] **Step 3.4: Commit**

```bash
git add specs/031-simplified-dashboard.md README.md
git commit -m "docs(spec 031): amend non-goal for run.body; README dashboard bullet"
```

---

## Self-review

**Spec coverage:** three zones + markdown compose + first-line-title (T1 server split + T2 textarea/splitMsg); Run/Add-to-Ready/Break-down actions (T2 2.3); Ready = backlog + queued with Start buttons, agent-badged (readyItem/queuedItem); In-progress = running+blocked with nested `tool`/`subagent` activity via the SSE buffer (trackEvent/activityLine) and inline gates (progressItem + decide with stopPropagation); Recent collapsed (details); kanban + drag deleted (2.1–2.3, verified by `data-col` grep 0); drawer kept and upgraded (body md + typed events, 2.4 — delivering 029's deferred drawer site); themes/machines/picker untouched; no new endpoints (all fetches are pre-existing routes); control-only 409 behavior preserved (breakdown/gate alerts unchanged). Spec inconsistency resolved + amended (T3.1).
**Placeholder scan:** two deliberate ADAPT notes (engine/ui test helper names — the implementer must mirror the real helpers in those files; assertions are fully specified). No TBDs.
**Type consistency:** `Run.Body`/`RunSummary.Body`/`r.body` chain matches T1↔T2 (progressItem reads `r.body`); `trackEvent` field names match the SSE Event wire type (`run_id`,`type`,`data` — 027); gate fields `run_id`/`node_id` match GateItem; `readyItem` reuses `.mdbody`/`md()` from 029.
