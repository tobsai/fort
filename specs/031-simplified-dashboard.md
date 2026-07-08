# 031 — Simplified dashboard (replaces the kanban)

**Status:** design approved in brainstorm (Toby, 2026-07-08) — pending written-spec review.
**Governed by:** [021-fort-native](021-fort-native.md) · replaces the 5-column board ([025-board-ux](025-board-ux.md)); consumes markdown ([029](029-markdown-content.md)) and subagent events ([030](030-subagent-events.md)); keeps the drawer ([027](027-run-drill-down.md)).

## Goal
Replace the 5-column kanban with a focused three-zone dashboard that matches how
work actually flows: **Define → Ready → In progress**. Compose work in markdown
(with optional breakdown), see what's ready to start, and watch what's running —
including **subagent activity, shown as such**.

## Non-goals (v1 — YAGNI)
- **No new endpoints.** Reuses `/api/summary`, `/api/board`, `/api/backlog(+/…)`,
  `/api/breakdown` (026), `/api/gate`, `/api/chat`, `/api/runs/{id}` (027),
  `/api/events` SSE, `/api/machines`. This is a re-layout of the served page.
- **No kanban toggle.** The 5-column board is **removed** (recoverable via git),
  per the approved decision — not kept behind a switch.
- **No drag-and-drop.** "Start" buttons replace drag-to-dispatch (simpler, and
  works on touch/mobile-width).
- **No new persistence.** Ready = existing backlog + queued runs; nothing new
  stored.

## Approach

### Layout — three stacked zones (top → bottom)
1. **Define** — a **multiline markdown** compose (replacing the single-line
   input). Convention: **first line = title**, the rest = body (markdown). Agent
   + machine pickers (carried from 025). Three actions:
   - **Run** — dispatch now (`POST /api/chat`, title+body).
   - **Add to Ready** — `POST /api/backlog` (source `user`).
   - **Break down** — `POST /api/breakdown` (026); sub-tasks land in Ready.
   A live `md()` (029) preview of the body renders beside/below the editor.
2. **Ready** — backlog items **and** `queued` runs merged into one "ready to
   start" list, newest-first. Each item: title + a clamped `md()` body preview,
   agent/machine chips, a **Start ▸** button (backlog → `/api/backlog/{id}/
   dispatch`; a queued run is already dispatched, shown for awareness), and
   edit/delete for backlog items. Agent-authored items (source `agent`, from
   breakdown) are badged.
3. **In progress** — live `running`/`blocked` runs. Each run row shows title,
   agent, machine, status, and **nested activity** from its event stream:
   `message` lines plainly, `tool` events as dim activity (`🔧 …`), and
   `subagent` events **badged + indented** (`🤖 subagent · "…"`) — so it's
   obvious when a subagent is working (030). **Gate approvals are inline here**
   (approve/reject move from the old Blocked column). A collapsed **Recent**
   strip at the bottom lists `succeeded`/`failed`/`canceled` runs.

Clicking any run opens the **027 drawer** for full drill-down (now rendering
markdown bodies (029) and tool/subagent events (030)).

### Data flow
- `refresh()` continues to poll `/api/board` + `/api/backlog` + `/api/summary`
  on the 3s tick and the `/api/events` SSE, re-bucketing runs into the three
  zones instead of five columns. In-progress activity nesting reads each running
  run's recent events (from the SSE stream already consumed, filtered by
  `run_id`; a run row can lazy-load `/api/runs/{id}` when expanded, same as the
  drawer).
- Gate items (`/api/board` `gates`) render inline in **In progress** with the
  existing `decide()` approve/reject calls.

### Theming / accessibility
Light/dark themes and the focus-visible styling from 025 carry over. Buttons
replace drag, so the dashboard is usable at mobile width (feeds 028's web app and
032's Mac app which mirror this layout).

### Determinism / seams
Pure `ui` re-layout; imports nothing new. No model calls; routing/placement
unchanged. The removed kanban markup/CSS is deleted, not commented out.

### Failure handling
- Control-only mode (no execution plane): **Run**/**Break down** already 409 (025/
  026); the dashboard badges the plane and surfaces the 409 (unchanged behavior),
  Ready still works (backlog is control-plane).
- Empty zones show a quiet empty state, not blank.

## Architecture (respects the seams)
- **`ui/page.go`** — the dashboard rewrite: new three-zone HTML/CSS, a multiline
  markdown compose, Ready list with Start buttons, In-progress with nested
  activity + inline gates, Recent strip. Removes the 5-column board markup/CSS
  and the drag handlers. Reuses `md()` (029) and renders `tool`/`subagent`
  events (030).
- No `core`/`exec`/contract changes; no new endpoints.

## Decisions
- **D1 — three zones, replace kanban.** Define/Ready/In-progress mirrors the real
  lifecycle; the 5 columns are removed (git-recoverable) per Toby.
- **D2 — markdown multiline compose, first line = title.** One field, natural for
  notes; body is markdown (029).
- **D3 — Start buttons, no drag.** Simpler, touch-friendly, mobile-ready for the
  028 web app.
- **D4 — gates inline in In-progress.** The blocked state lives with the run it
  blocks; no separate column.
- **D5 — subagent activity is first-class.** `subagent` events (030) are badged
  and nested so "a subagent is working" is visible at a glance — the core ask.
- **D6 — reuse every endpoint.** No API surface change; lower risk, easy
  rollback.

## Affected files
- `ui/page.go` — the dashboard (replaces the kanban board section + drag JS;
  adds Define/Ready/In-progress rendering, Start actions, inline gates, activity
  nesting).
- `README.md` — update the **Web** bullet (dashboard, not kanban).
- `docs/notes/*` — note the layout change.

## Test criteria
- Bucketing: given a `/api/board` payload with runs across
  queued/running/blocked/done + gates + backlog items, the page places each in
  the right zone (Ready = backlog + queued; In progress = running + blocked;
  Recent = done); a fake-served-page assertion (grep/DOM) covers the zone ids.
- Compose: first line becomes the run/backlog **title**, remainder the **body**
  (verified by posting and reading back the stored title/body).
- Actions: **Start** on a Ready backlog item calls
  `/api/backlog/{id}/dispatch`; **Break down** calls `/api/breakdown`; **Run**
  calls `/api/chat` — asserted via the existing ui test harness (capturing
  dispatcher/stub).
- Gates: an approve/reject in In-progress posts `/api/gate` (409 surfaced in
  control-only).
- Activity: a running run whose events include a `subagent` event renders a
  badged subagent line (served-page assertion using a seeded event).
- Regression: markdown bodies render via `md()` (029) and are XSS-inert; the
  kanban markup is gone (`grep -c 'data-col='` → 0).
- `go build ./...` + `go test ./ui/...` green.

## Rollback
Frontend-only. `git revert` restores the 5-column board (029/030 rendering is
additive and independent). No data/endpoint/type changes to undo.
