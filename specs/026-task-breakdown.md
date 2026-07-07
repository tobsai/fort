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
  strongest at decomposition). The run is built with `task.Agent = <planner>`.
  **Precondition (D7):** forcing that agent relies on an `@agent` passthrough
  rule existing for it in the active ruleset — `rules/v1.yaml` ships these for
  `claude`/`codex`/`hermes`/`openclaw` (the router treats `Task.Agent` as a
  *matcher*, not an override). With a custom ruleset that lacks a passthrough
  rule for `FORT_PLANNER`, the planner routes per the rules instead. (This is the
  same latent assumption that backs 025's "agent picker forces the agent".)
- Prompt: the goal wrapped in a fixed planner instruction asking for a short
  (≈3–8) list of concrete, independently-runnable sub-tasks, output as **only** a
  JSON array of `{"title": string, "agent"?: string, "machine"?: string}`, with
  an explicit "no prose, no markdown code fences" instruction (extraction strips
  fences defensively anyway — see below).
- The run appears on the board (title `breakdown: <goal>`, agent = the planner)
  and moves Queued → Running → Done like any run.

### Output → backlog (asynchronous, on completion)
`POST /api/breakdown` returns the planner **run id immediately**; the sub-tasks
appear in the backlog when that run finishes:
1. `control.Planner.Breakdown` dispatches the planner run via `engine.SubmitRef`
   (returns the run id) and spawns a completion goroutine (bound to the server's
   context; see the crash-window note under Failure handling).
2. The goroutine `engine.Wait(runID)` (new exported wait — blocks until the run's
   events are fully persisted), then checks `store.GetRun(runID).Status`. **Only
   a `succeeded` run proceeds**; a failed/canceled planner creates no items.
3. It recovers the planner's **final answer text from a single authoritative
   source**, not by concatenating normalized `message` events (claude runs with
   `--include-partial-messages`, so each partial delta *and* the terminal line
   both become `message` events — concatenating them doubles the array into
   `[…][…]`, which is unparseable). Instead, from `store.Events(runID)` it takes
   the raw `stdout` events and finds claude's terminal stream-json result line
   (`{"type":"result","result":"…"}` — Fort stores it as a raw stdout event
   because the message normalizer has no `"result"` key), `json.Unmarshal`s that
   line, and reads its `result` string (correct escape handling; avoids the lossy
   normalized-message path). Fallback for providers that emit plain final text
   (e.g. hermes): the **last** `message` event's data.
4. From that final text it extracts the plan by scanning for every **top-level**
   JSON array that decodes as `[]SubTask` and is *plan-shaped* (a valid empty
   array, or one with ≥1 object bearing a non-empty `title`), and accepts a
   result **only when exactly one** such array exists. A `json.Decoder` respects
   string literals (brackets/fences inside a title are content), an enclosing
   JSON object is consumed whole (so an array nested as an object *field* — e.g.
   a `{"decision":…,"example":[…]}` refusal — is never mistaken for the plan),
   and each decoded array's span is skipped so its inner brackets can't be
   recounted. **Zero** plan arrays → raw fallback; **two or more** (an
   illustrative example beside the real plan, whether it leads or trails; a
   doubled array; an object-wrapped array) → ambiguous → raw fallback: never
   guess which is the plan (D6: no false success). A titleless array is not
   plan-shaped, so it is unparsed, not a valid-empty plan.
5. For each valid sub-task it calls `store.CreateBacklogItem` with
   `source="agent"`, the title, and any suggested agent/machine.
On the next board refresh the items are in the Backlog.

### Determinism
Exactly **one** inference call — the planner run — which is generation, permitted
at a task node. Routing and placement of both the planner run and the resulting
sub-tasks stay model-free. The decomposition is the only model call; everything
downstream (scheduling by drag, routing each sub-task) is deterministic.

### Failure handling
- Planner run fails / is canceled: no backlog items; the failed `breakdown:
  <goal>` run is visible in Done — retry.
- **Valid but empty** plan (`[]`): success with **zero items** (the planner
  legitimately decided no breakdown is needed) — logged at info, no raw fallback.
  This is deliberately distinct from garbage output.
- **Unparseable** output (no balanced array, or the parsed value isn't an
  array-of-objects-with-title): create a **single** backlog item titled
  `breakdown (unparsed): <goal>` whose body is the raw final text, plus a logged
  warning — never silently dropped; you hand-edit.
- **Crash window (known limitation, v1):** the completion goroutine is the one
  non-replayable step. If `fort serve` stops in the gap between the planner run
  finishing and the goroutine writing items, the sub-tasks are not written (the
  run still shows Done) — re-run the breakdown. The goroutine uses the server's
  context and is abandoned on shutdown. A startup reconcile (re-extract from the
  persisted events of a finished `breakdown:` run that produced no items) is
  possible since extraction reads only durable state, but is **deferred** to keep
  v1 focused.

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
- **D6 — never silently drop, but distinguish empty from garbage.** A valid empty
  plan is success with zero items; only unparseable output becomes one
  raw-output backlog item (+ logged warning). Extraction reads a single
  authoritative result line (claude's stream-json `result`), not concatenated
  normalized messages, and accepts a plan **only when exactly one top-level
  plan-shaped array exists** (see step 4): any ambiguity — an illustrative
  example array beside the real plan, a doubled array, or an array nested inside
  a JSON object — falls to the raw fallback rather than guessing, and a titleless
  array is unparsed, not empty. So a mis-extracted substring can't be a false
  success. (Hardened across three adversarial-review rounds.)
- **D7 — planner-agent forcing is a ruleset precondition.** Forcing the planner
  agent depends on an `@agent` passthrough rule in the active ruleset;
  `rules/v1.yaml` ships them for the four known agents. Documented, and asserted
  by a default-ruleset test. (Shared with 025's agent picker; not an
  engine-level guarantee.)

## Affected files
- `ui/ports.go` — `Planner` port + `SubTask` type.
- `ui/server.go` — `POST /api/breakdown` handler + registration.
- `ui/contract.go` — `BreakdownRequest` + `BreakdownResult` wire types.
- `control/planner.go` (new) — `Planner` impl: prompt, dispatch, `Wait` +
  status check, extract the plan from claude's terminal `result` stdout line
  (small line struct decoded with `encoding/json`; fence-strip + balanced-array
  + shape-validate helpers), create backlog items (+ `control/planner_test.go`).
- `core/engine/engine.go` — exported `Wait(runID)`.
- `cmd/fort/wire.go`, `cmd/fort/main.go` — `FORT_PLANNER` config, `Planner`
  injection (serve only), `fort task breakdown` command + usage.
- `ui/page.go` — the Break down button.
- `docs/notes/*` / `README.md` — document breakdown + `FORT_PLANNER`.

## Test criteria
- `core/engine`: `Wait(runID)` blocks then returns after a run's events are
  persisted (fake runtime, in-flight path); **and** returns immediately for an
  already-finished run (complete a fake run, then call Wait) and for an unknown
  id — pinning the guarantee against `consume`'s defer ordering.
- Plan extraction (a `control` helper, unit-tested directly): a claude-shaped
  terminal `{"type":"result","result":"[…]"}` stdout line yields the sub-tasks;
  a ```json-fenced result with leading prose is stripped and parsed; a valid
  empty array `[]` yields **zero** items (not a raw fallback); prose containing a
  stray non-object bracketed list, or a non-array, yields the unparsed fallback;
  the doubled `[…][…]` shape (proving we don't concatenate partials) is handled
  by reading the single result line.
- `control/planner_test.go` (fake runtime whose canned output is a claude-style
  result line): `Breakdown` dispatches a run, and after it completes the store
  has one `source="agent"` backlog item per sub-task with the right
  titles/agents; unparseable output → exactly one `breakdown (unparsed): …` item;
  empty array → zero items; a **failed** planner run → zero items (status checked
  after `Wait`).
- Default-ruleset agent forcing (D7): a router/engine test asserts a task with
  `Agent=claude` under `rules/v1.yaml` routes to `claude` (the passthrough rule
  the planner depends on exists).
- `ui`: `POST /api/breakdown` 400s on blank text; 409 when `Planner` is nil
  (control-only); returns `{run_id}` with a stub planner.
- Determinism guard: exactly one agent dispatch per breakdown; no model call in
  the extraction/backlog-creation path.
- `go test ./...` + `-race` on `control` + `core/engine` green; seams intact
  (`ui` imports no engine; `control` may).

## Rollback
Additive. Revert the commits; drop the `Planner` wiring. With no `Planner`
injected the endpoint 409s and the board button is inert — byte-for-byte 025
behavior. No data migration (backlog items created by breakdown are ordinary
backlog rows).
