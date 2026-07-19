# Spec 033 — Web Dashboard Redesign (delegation model)

**Status:** approved-by-instruction (Toby: "Implement @design_handoff_fort_dashboard_redesign from Claude Design")
**Design source:** `design_handoff_fort_dashboard_redesign/` — `README.md` (spec) + `Fort Redesign.dc.html` (high-fidelity mockups; where the two disagree, the mockup wins — verified discrepancies listed in the plan).

## Goal

Replace the web board (`ui/page.go`) with the redesigned control plane: the landing
surface answers "what needs me?", projects carry generated sigils with a status-ring
grammar, progress is human-accepted checkpoints (never agent-estimated %), agents get
performance scorecards, and Week/Today schedule views show upcoming work per agent —
including predicted moments the human is needed. Same API-first architecture: the page
stays a single Go-served HTML const over the existing HTTP/SSE surface; `ui` keeps
importing only `core/store` + `core/task` and its ports.

## Approach

One page, six views behind a shared top-bar nav (Deck default): **Deck** (1a Command
Deck), **Projects** (1b Project Rooms), **Assign** (1c The Roster), **Performance**
(2a Scorecards), **Week** (2b), **Today** (3a). Vocabulary shifts to human words
(assignment / sign-off / Up next / Start / Draft a plan / checkpoint) over the same API.

Small, honest backend additions (all derivable/replayable, no model calls):

1. **Gate decisions become events** — `decideGate` appends an `event` row
   (`type:"gate"`, data JSON `{decision, note}`) so decision history survives the
   node_run upsert; `Reject` gains a `note` argument end-to-end
   (`ui.GateDecision.Note` → `ui.FlowRunner.Reject` → `control` → `graph.Executor`).
   This powers "Request changes… with a note" and first-pass-acceptance metrics.
2. **Board payload additions** (additive; FortKit ignores unknown keys):
   `RunSummary.created_at/updated_at`, `GateItem.since`, and a per-run `checkpoints`
   summary `{total, accepted, waiting, rejected, done}` computed from a new
   `store.AllNodeRuns()` plus a new `FlowRunner.Plan(flowID)` port method exposing the
   flow's node list (nil Runner ⇒ executed-nodes-only totals).
3. **`PATCH /api/backlog/{id}`** `{agent}` (+ `store.UpdateBacklogAgent`) — Week-view
   drag-to-reassign and Assign-an-agent on briefs.
4. **`GET /api/metrics?days=30`** — per-agent rollups computed in `ui` from runs +
   events: assignments, first-pass accepted % with sample counts, redirects per
   assignment, cost per accepted checkpoint (parsed from claude stdout `result` JSON
   where present), trend, 7-bucket sparkline, best/weak lanes by `matched_rule`.
   Flow-node work is attributed to agents via `started` events.

**Explicit deferrals** (need their own specs; UI degrades gracefully without them):
method versioning (`method vN` chips, promote/variant actions — no data source exists),
scheduler recurring blocks in Week/Today (`fort serve` doesn't own a scheduler and
entries aren't listable — spec 008 follow-up), non-claude cost (shown as "—").

## Affected files

- `ui/page.go` — full rewrite of `boardHTML` (keeps the tested `/* md:start */…/* md:end */` region verbatim; Go raw string ⇒ no literal backticks).
- `ui/contract.go` — additive fields (`RunSummary.CreatedAt/UpdatedAt`, `GateItem.Since`, `RunSummary.Checkpoints`, `GateDecision.Note`) + metrics types.
- `ui/ports.go` — `FlowRunner.Reject(runID, nodeID, note string)`, `FlowRunner.Plan(flowID string) []FlowNode`.
- `ui/server.go` — new routes (`PATCH /api/backlog/{id}`, `GET /api/metrics`), board enrichment.
- `ui/metrics.go` (new) — rollup computation.
- `core/store/store.go` — `AllNodeRuns()`; `core/store/backlog.go` — `UpdateBacklogAgent`.
- `core/graph/executor.go` — gate event append, `Reject` note.
- `control/control.go` — adapter updates (`Reject` note, `Plan`).
- Tests beside each (`core/graph`, `core/store`, `ui`).

## Test criteria

- `go test ./...` green; `go test -race ./ui/... ./core/graph/...` green.
- Routing path still makes zero model calls (untouched); only additive JSON — raw
  `/api/summary` + `/api/board` bodies still contain no `null` (existing guard test).
- goja md tests still pass against the new page.
- New tests: gate event append + reject note; board checkpoints/timestamps; backlog
  PATCH; metrics rollups (fixture store).
- Visual: all six views match the mockups (dark), light theme legible, control-only
  mode surfaces 409s, seeded FORT_FAKE demo screenshots reviewed.

## Rollback

Single revert of the feature branch merge. No schema migrations (only a new event
*type* and additive JSON); old clients ignore the additions, and the old page never
read them.
