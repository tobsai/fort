# 027 — Run drill-down drawer (DAG steps + per-step live log)

**Status:** design approved in brainstorm (Toby, 2026-07-07) — pending written-spec review.
**New capability — approved before implementation** (adds an `event.node_id` column + a board drawer).
**Governed by:** [021-fort-native](021-fort-native.md) · builds on the board ([025-board-ux](025-board-ux.md)).

## Goal
Make a run's work inspectable from the board. A run card becomes clickable; it
opens a **detail drawer** (side panel). For a **flow** run the drawer lists the
run's DAG **steps** (each `node_run` with its type + status); selecting a step
shows **that step's own live log**. For a **single** run (a plain `task`/chat/
breakdown run, which has no DAG) the drawer just streams the run's live log. The
drawer live-updates and closes back to the board.

## Non-goals (v1 — YAGNI)
- **No single-agent internal activity.** We do NOT parse an agent CLI's
  stream-json to reconstruct the tool-calls / Task-subagents *inside* one agent
  run (the "option A" tree). That needs new stream parsing and is declined for
  v1; the agent's text output remains the log.
- **No branching-graph reconstruction.** The step view is the *ordered list* of
  executed `node_run` rows; fanout branches appear as sibling steps. We do not
  reconstruct the flow's edge/branch structure into a nested graph.
- **No gate actions in the drawer.** Approve/reject stays in the existing Blocked
  column (already built, spec 025); the drawer shows a gate step read-only.
- **No new persistence** beyond the one additive `event.node_id` column.
- **No new streaming endpoint** — the drawer rides the existing `/api/events` SSE
  and the existing 3s `/api/board` poll.

## Approach

### Trigger
- A board run card gets a click handler → opens the drawer for that run id. The
  card markup gains an affordance (cursor/hover); the compose bar and columns are
  unchanged.

### The one data change — per-step log isolation
Today a flow's steps all write events under the flow's run id with **no step
marker** (`core/graph/executor.go` `execTask` appends `store.Event{RunID: runID,
…}` with no node), so their logs interleave and can't be separated. Fix:
- **`core/store`**: add a nullable `node_id` column to the `event` table via the
  existing idempotent `addColumn("event", "node_id", "TEXT")` helper (the same
  pattern used for `run.machine`); carry `NodeID` on the `Event` struct, write it
  in `AppendEvent`, and select it in the events query. Empty for single-run
  events and run-level flow events.
- **`core/graph/executor.go`**: when `execTask` appends a step's stream events,
  stamp `NodeID: node.ID`. That one field is the entire attribution mechanism.
  (The standalone-run path in `core/engine` is unchanged — its events keep an
  empty `node_id`.)

### The drawer (read-side, no new endpoint)
- Data source is the existing **`GET /api/runs/{id}`**, which already returns
  `{run, nodes, events}`. We add `node_id` to the `Event` **wire type** so the
  client can group events by step.
- Rendering: `nodes` → the ordered **step list** (node id, type, status,
  attempts); `events` → the log. Selecting a step filters the log to events whose
  `node_id` matches; a **single run** (no `nodes`) shows all its events with no
  step list.
- **Live updates ride existing rails:** the board already opens
  `EventSource('/api/events?since=0')` and ticks every 3s. The SSE payload gains
  `node_id`; while the drawer is open the client tails that stream — appending
  events that match the open run id (and, when a step is selected, its node id)
  to the log — for instant log lines, and re-fetches `GET /api/runs/{id}` on the
  existing 3s tick to refresh step statuses and backfill any missed events. No
  new server streaming.

### Determinism
`node_id` is descriptive metadata written on the existing append path; it adds
**zero** model calls and does not touch routing, placement, or the DAG's control
flow. The determinism invariants are unaffected.

### Failure handling
- A run with no events yet → drawer shows the step list (if a flow) or an empty
  log with a "waiting…" placeholder.
- A step that never ran (flow blocked/failed upstream) → it simply has no
  `node_run` row yet and no events; it isn't shown until it starts (consistent
  with today's `NodeRuns` which only returns executed nodes).
- Backfill: existing `event` rows created before the migration have `node_id`
  NULL/empty, so their logs show under the run but not attributed to a step —
  acceptable (historical runs predate attribution).

## Architecture (respects the seams)
- **`core/store/store.go`** — `event.node_id` column (idempotent migration),
  `Event.NodeID`, `AppendEvent` insert, events `SELECT`.
- **`core/graph/executor.go`** — stamp `NodeID: node.ID` on `execTask`'s event
  appends.
- **`ui/contract.go`** — `NodeID string \`json:"node_id,omitempty"\`` on the
  `Event` wire type.
- **`ui/server.go`** — `toEvent` carries `node_id`; the `/api/events` SSE line
  includes it. `handleRunDetail` is unchanged (already returns events, now
  tagged).
- **`ui/page.go`** — clickable cards; the drawer (step list + per-step log
  filter + live wiring); a single run renders log-only. `ui` still imports **no**
  engine/graph/router — the drawer uses only existing endpoints.
- **`README.md`** — a line on the drill-down drawer.

## Decisions
- **D1 — DAG steps, not agent internals.** Chosen in brainstorm (option B over A/
  C). The `node_run` data already exists; parsing an agent's internal
  tool/subagent stream (A) is deferred as a separate, larger capability.
- **D2 — detail drawer surface.** Chosen over inline-expand (cramped kanban
  columns) and a dedicated page (loses the at-a-glance board).
- **D3 — per-step log isolation via `event.node_id`.** Chosen over a shared
  run-level log so selecting a step shows only that step's activity. One additive
  column + one executor stamp.
- **D4 — reuse existing SSE + 3s poll.** No new streaming endpoint; the drawer
  filters the stream the board already consumes.
- **D5 — ordered step list of *executed* steps, not a branching graph.** v1
  renders the `node_run` rows in order as they run; the list grows as the flow
  progresses. Fanout branches are siblings. It does **not** pre-render pending
  future steps (that needs the flow definition, not just `node_run`) nor
  reconstruct branch nesting — both are possible later enhancements. (The
  brainstorm mockup greyed-out future steps for illustration; v1 shows only what
  has run so far.)
- **D6 — the drawer works for any run.** Flow → step list + per-step log; single
  run → log only. It is not flow-only (most board runs are single runs).
- **D7 — gates read-only in the drawer.** Approve/reject stays in the Blocked
  column (spec 025); no duplicate action surface.

## Affected files
- `core/store/store.go` — `event.node_id` (migration + struct + AppendEvent +
  query) (+ `core/store/store_test.go`).
- `core/graph/executor.go` — stamp `NodeID` in `execTask` (+ executor test).
- `ui/contract.go` — `Event.NodeID` wire field.
- `ui/server.go` — `toEvent` + SSE include `node_id`.
- `ui/page.go` — clickable cards + drawer + per-step log filter + live wiring.
- `README.md` — document the drill-down drawer.

## Test criteria
- `core/store`: `AppendEvent` + the events query round-trip `node_id`; an event
  written with no node has empty `node_id`.
- `core/graph`: a fake-runtime flow whose task node emits events → those events
  carry `node_id == node.ID`; a non-task node's run-level events carry empty
  `node_id`.
- `core/engine` (guard): a standalone task run's events have empty `node_id`
  (the engine path is unchanged).
- `ui`: `GET /api/runs/{id}` returns events with `node_id`; the `/api/events` SSE
  line includes `node_id`; grouping by `node_id` yields per-step logs.
- Seam: `ui` imports no engine/graph (`go list -deps`); `go test ./...` +
  `-race` on `core/store` + `core/graph` green.
- Frontend: the served page (`GET /`) contains the drawer markup and a clickable
  card handler (smoke `grep`); the drawer is verified live on the board.

## Rollback
Additive. Revert the frontend (drawer + clickable cards), the `Event.NodeID` wire
field, and the executor stamp. The `event.node_id` column is nullable and
harmless if left in place (no data migration to undo); it can be dropped later if
desired. With the drawer reverted the board is byte-for-byte 025/026 behavior.
