# Interactive Board Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement spec 025 — turn the served board into a work board: a polished kanban of runs, a persisted backlog with drag-to-dispatch, an agent picker, and light/dark themes.

**Architecture:** A new `backlog_item` SQLite table + CRUD in `core/store`; four `/api/backlog` endpoints in `ui/server.go` that reuse the existing store and `ui.Dispatcher` port (no new seam, no `ui`→engine import); a full rewrite of the single served-HTML board in `ui/page.go` (themed CSS variables, kanban columns, backlog lane, HTML5 drag-and-drop, agent picker). No `cmd/fort` changes — the backlog works in both `serve` (EngineDispatcher) and `control` (QueueDispatcher) modes because both a `Store` and a `Dispatcher` are already injected.

**Tech Stack:** Go 1.22 stdlib (`net/http`, `database/sql`, `modernc.org/sqlite`), the existing `ui` contract, vanilla HTML/CSS/JS served as a Go string.

**Ground rules (CLAUDE.md):** TDD the Go (store + endpoints): failing test → watch fail → minimal code → pass. `go test ./...` stays green after every task; `-race` on nothing new-concurrent here but run it on `./ui/...` and `./core/store/`. Respect seams: `ui` imports `core/store` + `core/task` (already does) but never engine/graph/router/native. The board JS is verified live (build + serve + screenshot), not unit-tested. Commit after each task on branch `feat/025-board-ux`.

**Spec:** `specs/025-board-ux.md` (approved). Read it first.

---

### Task 1: Store — `backlog_item` table + CRUD

**Files:**
- Modify: `core/store/store.go` (add one `CREATE TABLE` to the `migrate()` schema const)
- Create: `core/store/backlog.go`
- Create: `core/store/backlog_test.go`

- [ ] **Step 1.1: Write the failing test** — `core/store/backlog_test.go`:

```go
package store

import (
	"path/filepath"
	"testing"
)

func openBacklogTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestBacklogCRUD(t *testing.T) {
	s := openBacklogTest(t)
	if items, err := s.ListBacklog(); err != nil || len(items) != 0 {
		t.Fatalf("empty backlog: %v %v", items, err)
	}
	a := BacklogItem{ID: "b1", Title: "refactor loader", Body: "do it", Agent: "codex", Machine: "mini", Labels: []string{"dev"}, Source: "user"}
	if err := s.CreateBacklogItem(a); err != nil {
		t.Fatal(err)
	}
	b := BacklogItem{ID: "b2", Title: "write docs", Source: "agent"}
	if err := s.CreateBacklogItem(b); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListBacklog()
	if err != nil || len(items) != 2 {
		t.Fatalf("list: %v %v", items, err)
	}
	// newest first
	if items[0].ID != "b2" || items[1].ID != "b1" {
		t.Fatalf("order = %s,%s; want b2,b1", items[0].ID, items[1].ID)
	}
	got, err := s.GetBacklogItem("b1")
	if err != nil || got.Title != "refactor loader" || got.Agent != "codex" || got.Machine != "mini" || got.Source != "user" || len(got.Labels) != 1 || got.Labels[0] != "dev" {
		t.Fatalf("get b1 = %+v (%v)", got, err)
	}
	if err := s.DeleteBacklogItem("b1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetBacklogItem("b1"); err == nil {
		t.Fatal("b1 should be gone")
	}
	items, _ = s.ListBacklog()
	if len(items) != 1 || items[0].ID != "b2" {
		t.Fatalf("after delete = %+v", items)
	}
}
```

- [ ] **Step 1.2: Run — expect FAIL (undefined BacklogItem/CreateBacklogItem…)**

Run: `go test ./core/store/ -run TestBacklogCRUD -v`
Expected: build failure, `undefined: BacklogItem`.

- [ ] **Step 1.3: Add the table** — in `core/store/store.go`, inside the `migrate()` `schema` const, after the `invite` table's closing `);` and before the closing backtick, add:

```sql
CREATE TABLE IF NOT EXISTS backlog_item (
  id TEXT PRIMARY KEY, title TEXT, body TEXT, agent TEXT, machine TEXT,
  labels TEXT, source TEXT, created_at TEXT
);
```

- [ ] **Step 1.4: Implement** — create `core/store/backlog.go`:

```go
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// BacklogItem is a task queued for later dispatch (spec 025). It becomes a run
// only when dispatched (dragged onto the board / the Run action).
type BacklogItem struct {
	ID        string
	Title     string
	Body      string
	Agent     string   // optional forced agent
	Machine   string   // optional pinned host
	Labels    []string
	Source    string   // "user" | "agent"
	CreatedAt time.Time
}

// CreateBacklogItem inserts a pending item.
func (s *Store) CreateBacklogItem(b BacklogItem) error {
	labels, _ := json.Marshal(b.Labels)
	_, err := s.db.Exec(
		`INSERT INTO backlog_item(id,title,body,agent,machine,labels,source,created_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		b.ID, b.Title, b.Body, b.Agent, b.Machine, string(labels), b.Source, nowOr(b.CreatedAt))
	if err != nil {
		return fmt.Errorf("store: create backlog item: %w", err)
	}
	return nil
}

// ListBacklog returns pending items, newest first.
func (s *Store) ListBacklog() ([]BacklogItem, error) {
	rows, err := s.db.Query(
		`SELECT id,title,body,agent,machine,labels,source,created_at
		 FROM backlog_item ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BacklogItem
	for rows.Next() {
		b, err := scanBacklog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetBacklogItem returns one item by id.
func (s *Store) GetBacklogItem(id string) (BacklogItem, error) {
	row := s.db.QueryRow(
		`SELECT id,title,body,agent,machine,labels,source,created_at
		 FROM backlog_item WHERE id=?`, id)
	return scanBacklog(row)
}

// DeleteBacklogItem removes an item (called after it is dispatched or discarded).
func (s *Store) DeleteBacklogItem(id string) error {
	_, err := s.db.Exec(`DELETE FROM backlog_item WHERE id=?`, id)
	return err
}

func scanBacklog(row scanner) (BacklogItem, error) {
	var b BacklogItem
	var created string
	var labels sql.NullString
	if err := row.Scan(&b.ID, &b.Title, &b.Body, &b.Agent, &b.Machine, &labels, &b.Source, &created); err != nil {
		return BacklogItem{}, err
	}
	if labels.Valid && labels.String != "" {
		_ = json.Unmarshal([]byte(labels.String), &b.Labels)
	}
	b.CreatedAt = parseTime(created)
	return b, nil
}
```

(`scanner`, `nowOr`, `parseTime` already exist in `store.go`.)

- [ ] **Step 1.5: Run — expect PASS**

Run: `go test ./core/store/ -v`
Expected: PASS (existing + `TestBacklogCRUD`).

- [ ] **Step 1.6: Commit**

```bash
git add core/store/store.go core/store/backlog.go core/store/backlog_test.go
git commit -m "feat(store): backlog_item table + CRUD (spec 025)"
```

---

### Task 2: ui contract + `/api/backlog` endpoints

**Files:**
- Modify: `ui/contract.go` (add `BacklogItem`, `BacklogRequest`)
- Modify: `ui/server.go` (register routes + four handlers)
- Create: `ui/backlog_test.go`

- [ ] **Step 2.1: Write the failing test** — `ui/backlog_test.go`:

```go
package ui_test

import (
	"testing"

	"github.com/tobsai/fort/ui"
)

func TestBacklogAddListDispatchDelete(t *testing.T) {
	s, st := newFullUI(t)

	// add
	add := do(t, s, "POST", "/api/backlog", ui.BacklogRequest{Title: "queue me", Agent: "codex", Machine: "mini"})
	if add.Code != 200 {
		t.Fatalf("add status = %d", add.Code)
	}
	item := decode[ui.BacklogItem](t, add)
	if item.ID == "" || item.Title != "queue me" || item.Source != "user" {
		t.Fatalf("added = %+v", item)
	}

	// list
	list := decode[[]ui.BacklogItem](t, do(t, s, "GET", "/api/backlog", nil))
	if len(list) != 1 || list[0].ID != item.ID {
		t.Fatalf("list = %+v", list)
	}

	// dispatch -> a run exists, item is gone
	disp := do(t, s, "POST", "/api/backlog/"+item.ID+"/dispatch", nil)
	if disp.Code != 200 {
		t.Fatalf("dispatch status = %d", disp.Code)
	}
	res := decode[ui.ChatResult](t, disp)
	if res.RunID == "" {
		t.Fatalf("dispatch produced no run: %+v", res)
	}
	if _, err := st.GetRun(res.RunID); err != nil {
		t.Fatalf("run not persisted: %v", err)
	}
	if remaining := decode[[]ui.BacklogItem](t, do(t, s, "GET", "/api/backlog", nil)); len(remaining) != 0 {
		t.Fatalf("backlog not emptied after dispatch: %+v", remaining)
	}
}

func TestBacklogDeleteDiscards(t *testing.T) {
	s, _ := newControlUI(t)
	item := decode[ui.BacklogItem](t, do(t, s, "POST", "/api/backlog", ui.BacklogRequest{Title: "discard me"}))
	if do(t, s, "DELETE", "/api/backlog/"+item.ID, nil).Code != 200 {
		t.Fatal("delete failed")
	}
	if list := decode[[]ui.BacklogItem](t, do(t, s, "GET", "/api/backlog", nil)); len(list) != 0 {
		t.Fatalf("still present: %+v", list)
	}
}

func TestBacklogAddRequiresTitle(t *testing.T) {
	s, _ := newControlUI(t)
	if do(t, s, "POST", "/api/backlog", ui.BacklogRequest{Title: "   "}).Code != 400 {
		t.Fatal("blank title should 400")
	}
}
```

- [ ] **Step 2.2: Run — expect FAIL (undefined ui.BacklogItem / 404 routes)**

Run: `go test ./ui/ -run TestBacklog -v`
Expected: build failure `undefined: ui.BacklogRequest`.

- [ ] **Step 2.3: Add contract types** — append to `ui/contract.go`:

```go
// BacklogItem is a pending task queued on the board (spec 025).
type BacklogItem struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Body    string   `json:"body,omitempty"`
	Agent   string   `json:"agent,omitempty"`
	Machine string   `json:"machine,omitempty"`
	Labels  []string `json:"labels,omitempty"`
	Source  string   `json:"source"` // "user" | "agent"
}

// BacklogRequest is the command body for POST /api/backlog.
type BacklogRequest struct {
	Title   string   `json:"title"`
	Body    string   `json:"body,omitempty"`
	Agent   string   `json:"agent,omitempty"`
	Machine string   `json:"machine,omitempty"`
	Labels  []string `json:"labels,omitempty"`
	Source  string   `json:"source,omitempty"` // defaults to "user"
}
```

- [ ] **Step 2.4: Register routes** — in `ui/server.go` `Register`, after the `POST /api/chat` line add:

```go
	mux.HandleFunc("GET /api/backlog", s.handleBacklogList)
	mux.HandleFunc("POST /api/backlog", s.handleBacklogAdd)
	mux.HandleFunc("POST /api/backlog/{id}/dispatch", s.handleBacklogDispatch)
	mux.HandleFunc("DELETE /api/backlog/{id}", s.handleBacklogDelete)
```

- [ ] **Step 2.5: Implement handlers** — add to `ui/server.go` (needs no new imports; `uuid`, `task`, `store`, `strings`, `time`, `json` already imported):

```go
func (s *Server) handleBacklogList(w http.ResponseWriter, _ *http.Request) {
	items, err := s.d.Store.ListBacklog()
	if err != nil {
		httpError(w, err)
		return
	}
	out := []BacklogItem{}
	for _, b := range items {
		out = append(out, toBacklogItem(b))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleBacklogAdd(w http.ResponseWriter, r *http.Request) {
	var req BacklogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Title) == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	src := req.Source
	if src == "" {
		src = "user"
	}
	b := store.BacklogItem{
		ID: uuid.NewString(), Title: req.Title, Body: req.Body,
		Agent: req.Agent, Machine: req.Machine, Labels: req.Labels,
		Source: src, CreatedAt: time.Now(),
	}
	if err := s.d.Store.CreateBacklogItem(b); err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toBacklogItem(b))
}

func (s *Server) handleBacklogDispatch(w http.ResponseWriter, r *http.Request) {
	b, err := s.d.Store.GetBacklogItem(r.PathValue("id"))
	if err != nil {
		http.Error(w, "backlog item not found", http.StatusNotFound)
		return
	}
	t := task.Task{
		ID: uuid.NewString(), Title: b.Title, Body: b.Body,
		Labels: b.Labels, Agent: b.Agent, Machine: b.Machine, CreatedAt: time.Now(),
	}
	if t.Body == "" {
		t.Body = b.Title
	}
	ref, err := s.d.Dispatcher.Submit(r.Context(), t)
	if err != nil {
		httpError(w, err)
		return
	}
	_ = s.d.Store.DeleteBacklogItem(b.ID)
	writeJSON(w, http.StatusOK, ChatResult{Kind: "task", RunID: ref.RunID, Route: ref.Route, Machine: ref.Machine, Queued: ref.Queued})
}

func (s *Server) handleBacklogDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.d.Store.DeleteBacklogItem(r.PathValue("id")); err != nil {
		httpError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func toBacklogItem(b store.BacklogItem) BacklogItem {
	return BacklogItem{ID: b.ID, Title: b.Title, Body: b.Body, Agent: b.Agent, Machine: b.Machine, Labels: b.Labels, Source: b.Source}
}
```

- [ ] **Step 2.6: Run — expect PASS**

Run: `go test ./ui/ -run TestBacklog -v && go test ./ui/...`
Expected: PASS.

- [ ] **Step 2.7: Commit**

```bash
git add ui/contract.go ui/server.go ui/backlog_test.go
git commit -m "feat(ui): /api/backlog list/add/dispatch/delete (spec 025)"
```

---

### Task 3: Board front-end — kanban, backlog lane, drag-to-dispatch, agent picker, themes

**Files:**
- Modify: `ui/page.go` — replace the entire `boardHTML` const with the new board below.

This is a single served HTML string (JS is verified live, not unit-tested). Replace the whole `const boardHTML = ` … `` ` `` value with:

- [ ] **Step 3.1: Replace `boardHTML`** with the complete new page:

```go
const boardHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>Fort — Command Center</title>
<style>
  :root{
    --bg:#0b0e14;--panel:#12161f;--card:#12161f;--line:#212938;--line2:#1a212e;
    --fg:#e6eaf2;--fg2:#b9c0ce;--mut:#5a6373;--brass:#c8a45c;--brass2:#e3c785;
    --run:#e0a93b;--block:#5b9bf0;--ok:#4fb477;--fail:#e0655b;
    --run-bg:#241c0e;--block-bg:#111c2e;
  }
  :root[data-theme=light]{
    --bg:#f4f5f7;--panel:#ffffff;--card:#ffffff;--line:#e2e5ea;--line2:#edeff2;
    --fg:#1b2230;--fg2:#3a4658;--mut:#8b94a7;--brass:#9a7b2e;--brass2:#7a5f16;
    --run:#b07d10;--block:#2b6fd0;--ok:#2f8f5a;--fail:#c23b34;
    --run-bg:#f7efdd;--block-bg:#e7f0fc;
  }
  *{box-sizing:border-box}
  body{margin:0;background:var(--bg);color:var(--fg);font:13px/1.5 ui-monospace,Menlo,Consolas,monospace;transition:background .15s,color .15s}
  header{display:flex;align-items:center;gap:12px;padding:12px 18px;border-bottom:1px solid var(--line2)}
  header h1{margin:0;font-size:14px;letter-spacing:.18em;color:var(--brass2);text-transform:uppercase}
  .plane{font-size:10px;letter-spacing:.08em;text-transform:uppercase;border:1px solid var(--line);border-radius:6px;padding:1px 7px;color:var(--brass)}
  .grow{flex:1}
  .counts{display:flex;gap:12px;font-size:12px;color:var(--mut)}
  .counts b{color:var(--fg2)}
  .iconbtn{background:transparent;border:1px solid var(--line);border-radius:7px;color:var(--fg2);padding:5px 9px;cursor:pointer;font:inherit}
  .iconbtn:hover{background:var(--line2)}
  .machines{display:flex;gap:14px;padding:8px 18px;font-size:11.5px;color:var(--mut);border-bottom:1px solid var(--line2);flex-wrap:wrap}
  .dot{display:inline-block;width:6px;height:6px;border-radius:50%;margin-right:5px;vertical-align:middle}
  .dot.up{background:var(--ok)}.dot.down{background:var(--fail)}
  .board{display:grid;grid-template-columns:0.9fr 1fr 1fr 1fr 1fr;gap:11px;padding:14px 18px;align-items:start}
  .col{display:flex;flex-direction:column;min-width:0}
  .colhead{display:flex;justify-content:space-between;align-items:center;font-size:11px;letter-spacing:.05em;color:var(--mut);margin-bottom:9px;text-transform:uppercase}
  .colhead .n{background:var(--line2);border-radius:20px;padding:0 6px;min-width:18px;text-align:center}
  .col.running .colhead{color:var(--run)} .col.running .n{background:var(--run-bg);color:var(--run)}
  .col.blocked .colhead{color:var(--block)} .col.blocked .n{background:var(--block-bg);color:var(--block)}
  .col-body{display:flex;flex-direction:column;gap:9px;min-height:60px;border-radius:8px;padding:2px;transition:background .12s}
  .col.drop .col-body{background:var(--line2);outline:1px dashed var(--brass)}
  .card{background:var(--card);border:1px solid var(--line);border-left:2px solid var(--edge,#3b4557);border-radius:8px;padding:9px 10px}
  .card .title{font-size:12.5px;line-height:1.4;margin-bottom:7px;color:var(--fg)}
  .card.done .title{color:var(--fg2)}
  .meta{display:flex;align-items:center;gap:7px}
  .meta .ag{font-size:10.5px;color:var(--brass2)}
  .meta .mc{font-size:10.5px;color:var(--mut)}
  .e-running{--edge:var(--run)} .e-blocked{--edge:var(--block)} .e-ok{--edge:var(--ok)} .e-fail{--edge:var(--fail)} .e-neutral{--edge:#3b4557}
  .card.item{cursor:grab}
  .card.item:active{cursor:grabbing}
  .card.item .src{width:6px;height:6px;border-radius:50%;background:var(--mut);display:inline-block}
  .card.item .src.agent{background:var(--brass)}
  .gateact{display:flex;gap:6px;margin-top:8px}
  .gateact button{font-size:10.5px;padding:1px 8px;border-radius:5px;background:transparent;border:1px solid var(--line);color:var(--fg2);cursor:pointer}
  .gateact button.ok{color:var(--ok);border-color:var(--ok)}
  .gateact button.no{color:var(--fail);border-color:var(--fail)}
  .runbtn{margin-top:7px;font-size:10.5px;padding:1px 9px;border-radius:5px;background:transparent;border:1px solid var(--brass);color:var(--brass2);cursor:pointer}
  .empty{color:var(--mut);font-size:11.5px;padding:8px 4px}
  .compose{display:flex;gap:8px;padding:12px 18px;border-top:1px solid var(--line2)}
  .compose select,.compose input{background:var(--panel);color:var(--fg);border:1px solid var(--line);border-radius:7px;padding:7px 9px;font:inherit;font-size:12px}
  .compose input{flex:1}
  .compose select{cursor:pointer}
  .compose button{border-radius:7px;padding:7px 13px;font:inherit;font-size:12px;cursor:pointer;border:1px solid var(--line);background:var(--panel);color:var(--fg2)}
  .compose button.run{border-color:#3a3320;color:var(--brass2);background:transparent}
  .compose button:hover{background:var(--line2)}
  a:focus-visible,button:focus-visible,select:focus-visible,input:focus-visible,.card.item:focus-visible{outline:2px solid var(--brass);outline-offset:1px}
</style>
</head>
<body>
<header>
  <h1>Fort</h1>
  <span class="plane" id="plane">…</span>
  <span class="grow"></span>
  <span class="counts" id="counts"></span>
  <button class="iconbtn" id="theme" title="toggle theme" onclick="toggleTheme()">◐</button>
  <span class="counts" id="clock"></span>
</header>
<div class="machines" id="machines" style="display:none"></div>
<div class="board" id="board">
  <div class="col" data-col="backlog"><div class="colhead"><span>Backlog</span><span class="n" id="n-backlog">0</span></div><div class="col-body" id="c-backlog"></div></div>
  <div class="col" data-col="queued"><div class="colhead"><span>Queued</span><span class="n" id="n-queued">0</span></div><div class="col-body" id="c-queued"></div></div>
  <div class="col running" data-col="running"><div class="colhead"><span>Running</span><span class="n" id="n-running">0</span></div><div class="col-body" id="c-running"></div></div>
  <div class="col blocked" data-col="blocked"><div class="colhead"><span>Blocked</span><span class="n" id="n-blocked">0</span></div><div class="col-body" id="c-blocked"></div></div>
  <div class="col" data-col="done"><div class="colhead"><span>Done</span><span class="n" id="n-done">0</span></div><div class="col-body" id="c-done"></div></div>
</div>
<div class="compose">
  <select id="machine" title="target machine"><option value="">any machine</option></select>
  <select id="agent" title="agent"><option value="">auto agent</option></select>
  <input id="msg" placeholder="describe a task…" onkeydown="if(event.key==='Enter')runNow()"/>
  <button class="run" onclick="runNow()">Run</button>
  <button onclick="addToBacklog()">Add to backlog</button>
</div>
<script>
const $=s=>document.querySelector(s);
const esc=s=>(s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;');
let hasExec=true, machines=[];

// ---- theme ----
(function initTheme(){
  const saved=localStorage.getItem('fort-theme');
  const t=saved|| (matchMedia('(prefers-color-scheme: light)').matches?'light':'dark');
  document.documentElement.setAttribute('data-theme',t);
})();
function toggleTheme(){
  const cur=document.documentElement.getAttribute('data-theme')==='light'?'dark':'light';
  document.documentElement.setAttribute('data-theme',cur);
  localStorage.setItem('fort-theme',cur);
}

// ---- agent picker: union of agents, filtered by chosen machine ----
function agentsFor(machineName){
  let set=new Set();
  machines.forEach(m=>{ if(!machineName||m.name===machineName)(m.agents||[]).forEach(a=>set.add(a)); });
  return [...set].sort();
}
function syncAgentOptions(){
  const asel=$('#agent'), cur=asel.value, opts=agentsFor($('#machine').value);
  asel.innerHTML='<option value="">auto agent</option>'+opts.map(a=>'<option value="'+a+'">'+a+'</option>').join('');
  asel.value=opts.includes(cur)?cur:'';
}

// ---- rendering ----
function edgeFor(status){return status==='succeeded'?'e-ok':status==='failed'?'e-fail':status==='running'?'e-running':status==='blocked'?'e-blocked':'e-neutral';}
function runCard(r){
  const done=(r.status==='succeeded'||r.status==='failed'||r.status==='canceled');
  return '<div class="card '+(done?'done ':'')+edgeFor(r.status)+'">'+
    '<div class="title">'+esc(r.title||r.id)+'</div>'+
    '<div class="meta"><span class="ag">'+esc(r.agent)+'</span>'+(r.machine?'<span class="mc">'+esc(r.machine)+'</span>':'')+'</div></div>';
}
function gateCard(g){
  return '<div class="card e-blocked"><div class="title">'+esc(g.node_id)+'</div>'+
    '<div class="meta"><span class="mc">gate · '+esc(g.run_id.slice(0,8))+'</span></div>'+
    '<div class="gateact"><button class="ok" onclick="decide(\''+g.run_id+'\',\''+g.node_id+'\',\'approve\')">approve</button>'+
    '<button class="no" onclick="decide(\''+g.run_id+'\',\''+g.node_id+'\',\'reject\')">reject</button></div></div>';
}
function backlogCard(b){
  return '<div class="card item e-neutral" draggable="true" tabindex="0" data-id="'+b.id+'" ondragstart="onDrag(event,\''+b.id+'\')">'+
    '<div class="title">'+esc(b.title)+'</div>'+
    '<div class="meta"><span class="src '+(b.source==='agent'?'agent':'')+'"></span>'+
    (b.agent?'<span class="ag">'+esc(b.agent)+'</span>':'')+(b.machine?'<span class="mc">'+esc(b.machine)+'</span>':'')+'</div>'+
    '<button class="runbtn" onclick="dispatchItem(\''+b.id+'\')">run ▸</button></div>';
}
function fill(id,html,count){$('#'+id).innerHTML=html||'<div class="empty">—</div>';$('#n-'+id.slice(2)).textContent=count;}

async function refresh(){
  const sum=await (await fetch('/api/summary')).json();
  hasExec=sum.execution;
  $('#plane').textContent=hasExec?'full plane':'control only';
  $('#counts').innerHTML=['running','blocked','done'].map(k=>{
    const v=k==='done'?(sum.succeeded+sum.failed):(sum[k]||0);
    return '<span>'+k+' <b>'+v+'</b></span>';
  }).join('');
  machines=await (await fetch('/api/machines')).json()||[];
  const mbar=$('#machines');
  if(machines.length){
    mbar.style.display='flex';
    mbar.innerHTML=machines.map(m=>'<span><span class="dot '+(m.reachable?'up':'down')+'"></span>'+esc(m.name)+(m.local?' (local)':'')+' <b style="color:var(--fg2)">'+(m.agents||[]).join(', ')+'</b></span>').join('');
    const sel=$('#machine'),cur=sel.value;
    sel.innerHTML='<option value="">any machine</option>'+machines.map(m=>'<option value="'+m.name+'">'+m.name+'</option>').join('');
    sel.value=cur;
  }else mbar.style.display='none';
  syncAgentOptions();

  const b=await (await fetch('/api/board')).json();
  const runs=b.runs||[], gates=b.gates||[];
  const by=s=>runs.filter(r=>r.status===s);
  const done=runs.filter(r=>r.status==='succeeded'||r.status==='failed'||r.status==='canceled');
  fill('c-queued',by('queued').map(runCard).join(''),by('queued').length);
  fill('c-running',by('running').map(runCard).join(''),by('running').length);
  fill('c-blocked',(gates.map(gateCard).join('')||by('blocked').map(runCard).join('')),gates.length||by('blocked').length);
  fill('c-done',done.map(runCard).join(''),done.length);

  const items=await (await fetch('/api/backlog')).json()||[];
  fill('c-backlog',items.map(backlogCard).join(''),items.length);
}

// ---- actions ----
async function runNow(){
  const el=$('#msg'),text=el.value.trim();if(!text)return;el.value='';
  await fetch('/api/chat',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({text,machine:$('#machine').value,agent:$('#agent').value})});
  refresh();
}
async function addToBacklog(){
  const el=$('#msg'),text=el.value.trim();if(!text)return;el.value='';
  await fetch('/api/backlog',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({title:text,machine:$('#machine').value,agent:$('#agent').value})});
  refresh();
}
async function dispatchItem(id){
  await fetch('/api/backlog/'+id+'/dispatch',{method:'POST'});
  refresh();
}
async function decide(run,node,decision){
  const r=await fetch('/api/gate',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({run_id:run,node_id:node,decision})});
  if(r.status===409)alert('No execution plane — start fort serve to act on gates.');
  refresh();
}

// ---- drag: backlog item -> board dispatches it ----
function onDrag(e,id){e.dataTransfer.setData('text/plain',id);e.dataTransfer.effectAllowed='move';}
['queued','running','blocked','done'].forEach(c=>{
  const col=document.querySelector('[data-col='+c+']');
  col.addEventListener('dragover',e=>{e.preventDefault();col.classList.add('drop');});
  col.addEventListener('dragleave',()=>col.classList.remove('drop'));
  col.addEventListener('drop',e=>{e.preventDefault();col.classList.remove('drop');const id=e.dataTransfer.getData('text/plain');if(id)dispatchItem(id);});
});
$('#machine').addEventListener('change',syncAgentOptions);

const es=new EventSource('/api/events?since=0');
es.onmessage=()=>refresh();
setInterval(()=>{$('#clock').textContent=new Date().toLocaleTimeString()},1000);
setInterval(refresh,3000);
refresh();
</script>
</body>
</html>`
```

- [ ] **Step 3.2: Build**

Run: `go build ./... && go vet ./ui/...`
Expected: clean (Go string compiles; no server-side changes).

- [ ] **Step 3.3: Serve and verify live (both themes)**

```bash
go build -o /tmp/fort-25 ./cmd/fort
FORT_FAKE=1 FORT_DB=/tmp/board25/fort.db FORT_ADDR=127.0.0.1:4097 /tmp/fort-25 serve &
```
Then drive the board at `http://127.0.0.1:4097/` (screenshot in both themes). Verify:
- five columns render (Backlog · Queued · Running · Blocked · Done); theme toggle flips and persists across reload; the machine/agent selects populate.
- `curl -s -XPOST 127.0.0.1:4097/api/backlog -d '{"title":"drag me","agent":"codex"}'` then confirm the item shows in Backlog; dragging it onto the board (or its `run ▸` button) dispatches it — item leaves Backlog, a run appears in Running/Done.
- picking an agent, then a machine, filters the agent list; "auto agent" clears it.
- Kill the server (`kill %1`), remove `/tmp/board25`.

Capture the two screenshots and the drag result in the task report.

- [ ] **Step 3.4: Commit**

```bash
git add ui/page.go
git commit -m "feat(ui): interactive board — kanban, backlog lane, drag-to-dispatch, agent picker, light/dark (spec 025)"
```

---

### Task 4: Full verification + gates

**Files:** none (verification only)

- [ ] **Step 4.1: Full suite**

Run: `go test ./... && go test -race ./ui/... ./core/store/ && go vet ./...`
Expected: all green.

- [ ] **Step 4.2: Confirm no seam violation** — `ui` must not import engine/graph/router/native:

Run: `go list -deps ./ui | grep -E 'core/(engine|graph|router)|exec/native' || echo "seam clean"`
Expected: `seam clean` (ui uses only store + task + the injected ports).

- [ ] **Step 4.3: Control-only mode still works** (backlog boards via QueueDispatcher):

```bash
go build -o /tmp/fort-25 ./cmd/fort
FORT_DB=/tmp/board25c/fort.db FORT_ADDR=127.0.0.1:4098 /tmp/fort-25 control &
curl -s -XPOST 127.0.0.1:4098/api/backlog -d '{"title":"queued item"}' ; echo
curl -s 127.0.0.1:4098/api/backlog ; echo
kill %1; rm -rf /tmp/board25c
```
Expected: item created and listed (dispatch in control mode boards it as queued via the QueueDispatcher).

---

### Task 5: Docs

**Files:**
- Modify: `README.md` (the board bullet under the web surface, if present) + `ui/apple/README.md` note only if it describes the board layout.

- [ ] **Step 5.1:** Update the one-line board description to mention the kanban + backlog + agent picker + themes. Keep it factual and short.

- [ ] **Step 5.2: Commit**

```bash
git add README.md
git commit -m "docs: interactive board (kanban, backlog, agent picker, themes)"
```

---

## Self-review checklist (run after Task 5)

- **Spec coverage:** kanban (T3), agent picker + filtering (T3), light/dark + persisted toggle (T3), backlog table + CRUD (T1), backlog endpoints + dispatch-empties-item (T2), drag-to-dispatch + Run fallback (T3), Blocked subsumes gate inbox (T3 gate cards in Blocked column), polish verified live both themes (T3.3). Non-goals honored: no run-card dragging (only backlog items are `draggable`), no backlog reordering, no subagent tree (that's spec 027).
- **Type consistency:** `BacklogItem`/`BacklogRequest` fields match between `core/store` (Go struct), `ui/contract.go` (wire), and the JS (`b.id/title/agent/machine/source`). `toBacklogItem` bridges store→wire. Dispatch reuses `ChatResult` (same shape the board already decodes).
- **Placeholder scan:** every code step has complete code; the front-end is the full `boardHTML`, not a sketch.
- **Seam:** no new `ui` imports beyond `store`+`task` (already present); `go list -deps` guard in T4.2.
