# 025 — Interactive board: kanban, backlog, drag-to-dispatch, agent picker, themes

**Status:** design approved in brainstorm (Toby, 2026-07-06) — pending written-spec review.
**New capability — approved before implementation** (backlog + drag-to-dispatch change the dispatch flow).
**Governed by:** [021-fort-native](021-fort-native.md) · precedes breakdown (026) and subagent-tree (027).

## Goal
Turn the served board from a passive run list into a **work board**:
1. A **kanban** — runs grouped into status columns (polished, light/dark).
2. A **backlog** — a lane of pending items you (or, later, agents) create and hold
   until you're ready.
3. **Drag-to-dispatch** — drag a backlog item onto the board to run it.
4. An **agent picker** — choose which agent runs a task.

Guiding principle: **simplicity of controls and feedback**. You control *when*
work starts (drag from the backlog); the board stays truthful about *what is
happening* (run cards sit in the column matching their real state).

## Non-goals (v1 — deferred to later specs / YAGNI)
- **Not** the task breakdown itself — agents *filling* the backlog is spec 026.
- **Not** the subagent tree — drilling into a run's children is spec 027.
- No reordering within the backlog (drag is backlog→board to dispatch, not sorting).
- No dragging run cards between status columns — a run's column reflects real
  state, never a manual move (see D1).
- No backlog scheduling/priorities/due dates. A flat, newest-first list.

## Approach

### Data model — backlog items (`core/store`)
A backlog item is a task that has not been dispatched. New table:

```sql
CREATE TABLE backlog_item (
  id TEXT PRIMARY KEY, title TEXT, body TEXT,
  agent TEXT, machine TEXT,      -- optional forced agent / pinned machine
  labels TEXT,                    -- json array
  source TEXT,                    -- "user" | "agent"
  created_at TEXT
);
```

Store methods (mirroring the existing run methods): `CreateBacklogItem`,
`ListBacklog` (newest first), `GetBacklogItem`, `DeleteBacklogItem`. Additive
migration; single-machine and existing behavior are untouched when the backlog is
empty.

### Endpoints (`ui/server.go`)
The `ui` package already reads the store directly (board data) and dispatches
through the injected `ui.Dispatcher` port — the backlog reuses both, so no new
architectural seam:
- `GET /api/backlog` — list pending items.
- `POST /api/backlog` — create one `{title, body?, agent?, machine?, labels?}`
  with `source:"user"` (agents use the same endpoint with `source:"agent"` in
  spec 026).
- `POST /api/backlog/{id}/dispatch` — build a `task.Task` from the item (plus any
  `{machine, agent}` overrides from the drop), call `Dispatcher.Submit`, then
  `DeleteBacklogItem`. Returns the new run ref. This is the drag's landing.
- `DELETE /api/backlog/{id}` — discard a pending item.

`Dispatcher.Submit` already runs the task on a detached context (v0.6.1), so a
drag-dispatched run behaves like any other.

### Board (`ui/page.go`)
**Layout:** top bar (brass `FORT` wordmark + plane pill · live counts · machine
roster · theme toggle) · a **Backlog** lane on the left · the four status columns
(**Queued · Running · Blocked · Done**) · a full-width compose row at the bottom.
The separate gate-inbox panel folds into **Blocked** (one "needs you" surface).

**Kanban:** group `board.runs` by status into the four columns. A run is a card
(title · agent · machine tag · 2px left accent in the column/outcome colour) —
no loud badges. **Done** is one column; outcome shown by the accent (green
succeeded / red failed / neutral canceled), newest first. **Blocked** cards carry
inline approve/reject posting to `/api/gate`.

**Backlog + drag-to-dispatch:**
- The Backlog lane lists pending items (title · optional agent/machine chips ·
  a source dot). Each item is `draggable`.
- Drop target: the board area (the status columns). Dropping an item calls
  `POST /api/backlog/{id}/dispatch`; the item leaves the backlog and appears as a
  run in Queued/Running on the next board refresh. Which column you drop on does
  not change routing — dispatch uses the item's own agent/machine (or the
  deterministic defaults); you set those on the item when you create it, not by
  aiming the drop.
- Keyboard/fallback: each backlog item also has a plain **Run** button (drag is
  the affordance, not the only path — accessibility + touch).

**Compose (the controls):** machine `<select>` · agent `<select>` · prompt input ·
two actions — **Run** (dispatch now, today's behavior) and **Add to backlog**
(creates a pending item). Minimal and unambiguous.

**Agent picker:** "auto agent" (default `""`, unchanged routing) plus the agents
the mesh offers (union of `/api/machines` `agents`). Selecting one forces it
(matches `--agent`); the list filters to the selected machine's agents so an
impossible combo can't be chosen; an invalidated choice resets to "auto agent".

**Light / dark:** all colours become CSS variables under `:root` (dark) and
`:root[data-theme="light"]`. Default follows `prefers-color-scheme`; one small
sun/moon toggle flips `data-theme` and persists to `localStorage`. Both palettes
keep the command-center identity (brass accent, semantic status colours tuned per
mode).

### Polish (explicit acceptance criterion)
Verified live in **both** themes: one palette per theme (all via CSS variables),
an 8px spacing grid, a tight type scale (~11–13px), status by column + left
accent (not badges), real hover/focus-visible states on cards, backlog items,
selects, input, buttons, a clear drag affordance (cursor + drop-target
highlight), subtle transitions, WCAG-AA text contrast, fully keyboard-operable.

## Decisions
- **D1 — drag dispatches backlog items; runs are not draggable.** You control when
  work starts by dragging a pending item onto the board. A run card's column is
  its real state and never a manual move — the board stays truthful. Reconciles
  the drag UX you want with Fort's determinism.
- **D2 — one Done column**, outcome via left-accent colour (Toby's choice).
- **D3 — agent picker forces the agent**; "auto agent" preserves rules routing;
  machine-aware filtering prevents invalid pins.
- **D4 — theme = OS default + persisted toggle** (`localStorage`, one icon).
- **D5 — backlog reuses existing seams.** CRUD via store methods called from
  `ui/server.go` (as board reads already do); dispatch via the `ui.Dispatcher`
  port. No new port, no `ui`→engine import.
- **D6 — Blocked subsumes the gate inbox** — one surface for "needs you".
- **D7 — backlog is persisted and durable.** Items survive restarts (SQLite),
  so a plan you queue up is still there tomorrow; `source` distinguishes yours
  from agent-created (spec 026).

## Affected files
- `core/store/` — `backlog_item` table + CRUD methods (+ a `backlog_test.go`).
- `ui/server.go` — `/api/backlog` list/create/dispatch/delete handlers.
- `ui/contract.go` — `BacklogItem` wire type.
- `ui/page.go` — the whole board: themed CSS variables + toggle, kanban grouping,
  Backlog lane, drag-to-dispatch, agent picker + filtering, gate actions on
  Blocked cards, Run / Add-to-backlog compose.
- `ui/ui_test.go` — endpoint contract tests (create→list→dispatch produces a run
  and empties the item; `/api/chat` honours `agent`; `/api/board` exposes status).

## Test criteria
- `go test ./core/store/ ./ui/...` green:
  - backlog CRUD round-trips; `ListBacklog` newest-first.
  - `POST /api/backlog` then `GET /api/backlog` returns the item; `dispatch`
    creates a run (via a fake dispatcher) and removes the item; `DELETE` discards.
  - `/api/chat` with `{agent:"codex"}` dispatches a codex run; `/api/board`
    carries `status`; `/api/machines` carries `agents`.
- Live board (both themes): runs land in the correct column; a gate-blocked run
  appears in Blocked and approve/reject resolves it; **dragging a backlog item
  onto the board dispatches it** (item disappears, run appears); the Run button is
  an equivalent path; the agent picker forces/filters correctly; theme follows the
  OS and the toggle persists across reload.
- `go test ./...` and `go vet ./...` stay green.

## Rollback
Additive. Revert the commit(s) to restore the current board; drop the
`backlog_item` table (auto-created; dropping the DB also suffices). No migration
of existing runs either way.
