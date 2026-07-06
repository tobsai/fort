# 026 — Task breakdown (agents decompose a goal into backlog sub-tasks)

**Status:** design approved in brainstorm (Toby, 2026-07-06) — pending written-spec review.
**New capability — approved before implementation** (adds a planning model call + a new port).
**Governed by:** [021-fort-native](021-fort-native.md) · builds on the backlog ([025-board-ux](025-board-ux.md)).

## Goal
Turn a one-line goal into a reviewed set of sub-tasks. You ask Fort to **break
down** a goal; a planner agent decomposes it into concrete sub-tasks that land in
the **backlog** (`source=agent`) for you to curate — edit, discard, and drag the
ones you want onto the board to run. The backlog is the plan-approval surface.

## Non-goals (v1 — YAGNI)
- No dependency edges / sequencing between sub-tasks — each is an independent
  backlog item you schedule freely (the hybrid/DAG models were considered and
  declined in brainstorm).
- No auto-dispatch of the sub-tasks — they wait in the backlog until you drag
  them (that is the whole point of 025's backlog).
- No iterative re-planning / plan editing UI beyond the existing backlog CRUD.
- No breakdown in control-only mode — planning is a real agent run, so it needs
  an execution plane (like gates, it 409s without one).

## Approach

### Trigger
- **Board:** a **Break down** action on the compose bar (beside Run / Add to
  backlog). It posts the prompt input to `POST /api/breakdown`.
- **CLI:** `fort task breakdown "<goal>"` — a thin client of the running daemon
  (posts to `/api/breakdown` on the local loopback, like `fort mesh invite`),
  prints the planner run id. Requires `fort serve` running.
- Request body: `{"text": "<goal>", "agent": "", "machine": ""}` — optional
  `agent`/`machine` override where the planner runs.

### The planner (a visible run)
The planner is a normal Fort run so you can watch it and read its output:
- Agent = the request's `agent` if set, else `FORT_PLANNER` (default `claude` —
  strongest at decomposition). Placed like any task (on the mesh, wherever claude
  is offered).
- Prompt: the goal wrapped in a fixed planner instruction asking for a short
  (≈3–8) list of concrete, independently-runnable sub-tasks, output as **only** a
  JSON array of `{"title": string, "agent"?: string, "machine"?: string}`, no
  prose.
- The run appears on the board (title `breakdown: <goal>`, agent = the planner)
  and moves Queued → Running → Done like any run.

### Output → backlog (asynchronous, on completion)
`POST /api/breakdown` returns the planner **run id immediately**; the sub-tasks
appear in the backlog when that run finishes:
1. `control.Planner.Breakdown` dispatches the planner run via `engine.SubmitRef`
   (returns the run id) and spawns a goroutine.
2. The goroutine `engine.Wait(runID)` (new exported wait — blocks until the run's
   events are fully persisted), then `store.Events(runID)`.
3. It extracts the plan: concatenate the run's `message` events (Fort's
   normalized assistant output; fall back to `stdout` events), find the JSON
   array (first `[` … last `]`), `json.Unmarshal` into `[]SubTask`.
4. For each sub-task it calls `store.CreateBacklogItem` with `source="agent"`,
   the sub-task title, and any suggested agent/machine.
On the next board refresh the items are in the Backlog.

### Determinism
Exactly **one** inference call — the planner run — which is generation, permitted
at a task node. Routing and placement of both the planner run and the resulting
sub-tasks stay model-free. The decomposition is the only model call; everything
downstream (scheduling by drag, routing each sub-task) is deterministic.

### Failure handling
- Planner run fails / is canceled: no backlog items created; the failed
  `breakdown: <goal>` run is visible in Done (you see it failed) — retry.
- Output has no parseable JSON array: create a **single** backlog item titled
  `breakdown (unparsed): <goal>` whose body is the raw planner output, and log a
  warning — so the work is never silently dropped and you can hand-edit.
- Zero sub-tasks parsed: same single-item fallback.

### Architecture (respects the seams)
- **`ui.Planner` port** (`ui/ports.go`), nil in control-only mode:
  ```go
  type Planner interface {
      Breakdown(ctx context.Context, goal, agent, machine string) (runID string, err error)
  }
  type SubTask struct { Title, Agent, Machine string }
  ```
- **`control.Planner`** (`control/planner.go`) implements it using the engine
  (dispatch + `Wait`) and the store (read events, create backlog items). The
  planner prompt + JSON extraction live here.
- **`core/engine`**: add exported `Wait(runID string)` (thin wrapper over the
  existing unexported `wait`) so `control` can block on the run without polling.
- **`ui/server.go`**: `POST /api/breakdown` handler — decode `{text, agent,
  machine}`, 400 on blank text, 409 if `Planner == nil`, else
  `Planner.Breakdown(...)` → `{run_id}`.
- **`cmd/fort`**: read `FORT_PLANNER` (default `claude`); construct
  `control.Planner` with the engine + store + default planner agent; inject as
  `ui.Deps.Planner` (only in `serve`). Add the `fort task breakdown` CLI.
- **`ui/page.go`**: a **Break down** button in the compose bar → `POST
  /api/breakdown {text, machine, agent}` → refresh.

## Decisions
- **D1 — output is backlog items, not a DAG.** Chosen in brainstorm: reuses 025's
  backlog as the plan-review surface; no sequencing. Simpler, and the curation
  (edit/discard/drag) *is* the "approve the plan" step.
- **D2 — planner is a visible run.** Fort-idiomatic (everything is a run/event);
  you watch it work and can inspect its output. `POST /api/breakdown` returns the
  run id immediately and the backlog fills asynchronously on completion.
- **D3 — default planner `claude`, configurable.** `FORT_PLANNER` env / request
  `agent` override. Claude is strongest at decomposition.
- **D4 — one model call, at a task node.** Determinism preserved: routing +
  placement stay model-free; only the planner run infers.
- **D5 — needs an execution plane.** Breakdown 409s in control-only mode, like
  gates — planning is a real agent invocation.
- **D6 — never silently drop.** Unparseable/empty output becomes one raw-output
  backlog item you can edit, plus a logged warning.

## Affected files
- `ui/ports.go` — `Planner` port + `SubTask` type.
- `ui/server.go` — `POST /api/breakdown` handler + registration.
- `ui/contract.go` — `BreakdownRequest` + `BreakdownResult` wire types.
- `control/planner.go` (new) — `Planner` impl: prompt, dispatch, wait, extract,
  create backlog items (+ `control/planner_test.go`).
- `core/engine/engine.go` — exported `Wait(runID)`.
- `cmd/fort/wire.go`, `cmd/fort/main.go` — `FORT_PLANNER` config, `Planner`
  injection (serve only), `fort task breakdown` command + usage.
- `ui/page.go` — the Break down button.
- `docs/notes/*` / `README.md` — document breakdown + `FORT_PLANNER`.

## Test criteria
- `core/engine`: `Wait(runID)` returns after a run's events are persisted (fake
  runtime); returns immediately for an unknown/finished run.
- `control/planner_test.go` (fake runtime whose output is a canned JSON plan):
  `Breakdown` dispatches a run, and after it completes the store has one backlog
  item per sub-task with `source="agent"` and the right titles/agents; an
  unparseable output yields exactly one `breakdown (unparsed): …` item; a failed
  planner run yields zero items.
- `ui`: `POST /api/breakdown` 400s on blank text; 409 when `Planner` is nil
  (control-only); returns `{run_id}` with a stub planner. `/api/backlog` then
  lists the created items.
- Determinism guard: `control/planner.go` makes exactly one agent dispatch;
  no model call in the backlog-creation path.
- `go test ./...` + `-race` on `control` + `core/engine` green; seams intact
  (`ui` imports no engine; `control` may).

## Rollback
Additive. Revert the commits; drop the `Planner` wiring. With no `Planner`
injected the endpoint 409s and the board button is inert — byte-for-byte 025
behavior. No data migration (backlog items created by breakdown are ordinary
backlog rows).
