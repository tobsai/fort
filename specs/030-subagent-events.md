# 030 — Subagent + tool events (parse the agent stream)

**Status:** design approved in brainstorm (Toby, 2026-07-08) — pending written-spec review.
**New capability — approved before implementation** (adds typed run events for tool/subagent activity).
**Governed by:** [021-fort-native](021-fort-native.md) · builds on run drill-down ([027-run-drill-down](027-run-drill-down.md)); this is the deferred "option A" from 027's brainstorm.

## Goal
Make "an agent spawned a subagent / called a tool" a **fact the UI can show**.
The native `claude` provider currently downsamples its rich `stream-json` to a
single `message` event type (text only) and drops everything else as raw
`stdout`. This spec parses the structured stream into new typed events —
`tool` and `subagent` — so the dashboard (031) and drawer (027) can badge and
nest that activity.

## Non-goals (v1 — YAGNI)
- **No recursion into a subagent's own internals.** claude's parent stream
  surfaces a Task-tool `tool_use` and its returned `tool_result`, not the
  subagent's private step-by-step stream. v1 shows the spawn + its result, not a
  full nested tree inside the subagent. (Documented limitation.)
- **No new providers.** Only `claude` gains structured parsing in v1 (it has the
  richest stream). `codex`/`hermes`/`openclaw` keep today's text behavior; the UI
  degrades gracefully (no tool/subagent rows for them).
- **No model calls added.** Pure parsing of already-emitted CLI output.
- **No change to routing/placement.** Determinism invariants untouched.

## Approach

### New event types (`core/runtime`)
Add two `EventType` constants alongside the existing six:
- `EventTool` (`"tool"`) — the agent invoked a tool. `Data` = a compact JSON
  `{"name":<tool>,"summary":<short input desc>}` (or a normalized string; the
  normalizer decides). Example: a `Read` of a file, a `Bash` command.
- `EventSubagent` (`"subagent"`) — the agent spawned a sub-task (claude's `Task`
  tool). `Data` = `{"description":<task desc>,"agent":<subagent type>}`.

These persist through the existing append-only `event` path unchanged (they are
just new `type` string values; `store.Event` already carries `type`/`data`, and
`node_id` from 027 still attributes them to a flow step).

### Provider parsing (`exec/native`)
The `claude` provider's line parser is upgraded from "extract text field" to a
**structured stream-json classifier**:
- `type:"assistant"` message containing a `tool_use` block → `EventTool` (or
  `EventSubagent` when the tool name is `Task`), carrying name + a short input
  summary.
- `type:"user"` message containing a `tool_result` block → optionally a terminal
  marker on the matching tool event (v1: emit a lightweight `tool` event of the
  result completing; keep it simple).
- assistant **text** → `EventMessage` (unchanged).
- the terminal `type:"result"` line → still surfaced as today (used by 026's
  planner extraction — **must not regress**).
- unknown/other lines → raw `stdout` (unchanged fallback).

This is additive: everything that produced a `message` or `result` before still
does. The classifier is a Fort-owned contract over claude's output (the
"industry tool is the engine, Fort owns the interface" tenet).

### UI consumption (thin, mostly 031/027)
- The SSE/`toEvent` path already serializes any event type; `tool`/`subagent`
  events flow to clients with no contract change.
- **Drawer (027)** and **dashboard (031)** render a `tool` event as a dim
  activity line (e.g. `🔧 Read core/foo.go`) and a `subagent` event as a badged,
  indented line (e.g. `🤖 subagent · "write a repro test"`), distinct from plain
  messages. The visual treatment lands in 031; 030 delivers the data + the event
  contract.

### Determinism
Parsing CLI stdout adds **zero** inference and no routing decisions. The
classifier is deterministic string/JSON handling. Assert: no new
`runtime.Runtime.Dispatch`/router calls; the provider still spawns exactly the
same CLI argv.

### Failure handling
- Malformed/partial JSON line → falls through to `stdout` (never panics; the
  parser is total, mirroring 026's robust extraction lessons).
- A `tool_use` with no recognizable name → `EventTool` with an empty/summary
  name rather than a drop (never silently lose activity).
- `--include-partial-messages` duplicate/partial frames → deduped by only
  classifying complete assistant/user message objects (partials ignored for
  tool/subagent typing), so no doubled tool events.

## Architecture (respects the seams)
- **`core/runtime/runtime.go`** — `EventTool`, `EventSubagent` constants (+ doc).
- **`exec/native/providers.go`** — the `claude` structured classifier
  (`tool_use`/`tool_result`/`Task` → typed events); other providers unchanged.
- **`exec/native/native.go`** — the scan loop emits the classifier's typed
  events (it already emits normalized events; this widens the set).
- **`ui/`** — no contract change (event `type`/`data` already flow); rendering is
  in 031/027.
- **`docs/notes/runtime-recon.md`** — document the claude stream-json → event map.

## Decisions
- **D1 — parse, don't infer.** Structured classification of existing output; no
  model calls, determinism intact.
- **D2 — claude-only in v1.** Richest stream; others degrade gracefully and are a
  documented follow-on.
- **D3 — spawn + result, not deep recursion.** Show that a subagent ran and its
  outcome; the subagent's private internals aren't in the parent stream.
- **D4 — additive event types.** `message`/`result` behavior preserved so 026's
  planner extraction and existing logs don't regress; new types are opt-in for
  the UI.
- **D5 — total parser.** Any unrecognized/partial line degrades to `stdout`;
  never panic, never silently drop recognized activity.

## Affected files
- `core/runtime/runtime.go` — new event-type constants (+ `core/runtime` doc).
- `exec/native/providers.go` — claude stream-json classifier (+ tests).
- `exec/native/native.go` — emit typed events from the scan loop.
- `docs/notes/runtime-recon.md` — the event map.

## Test criteria
- Classifier (unit, table-driven over canned claude stream-json lines):
  a `tool_use` line → one `EventTool` with the tool name; a `Task` `tool_use` →
  `EventSubagent` with the description; an assistant **text** line → `EventMessage`
  (unchanged); a `type:"result"` line → still the terminal result (regression
  guard for 026); a malformed line → `stdout` (no panic).
- No-doubling: a stream with `--include-partial-messages` partial frames plus the
  final message yields exactly one tool/subagent event per real invocation.
- Determinism guard: `exec/native` adds no router/engine import and no `Dispatch`
  call in the classifier; the spawned argv is unchanged (existing provider tests
  stay green).
- End-to-end (fake not applicable — real claude stream): an integration test
  fixture (recorded stream-json) drives the classifier and asserts the event
  sequence; `go test ./exec/native/... -race` green.

## Rollback
Additive. Revert the classifier (the provider falls back to text extraction) and
drop the two event-type constants. Persisted `tool`/`subagent` rows become inert
unknown types the UI ignores — no migration. `message`/`result` behavior is
unchanged throughout, so nothing downstream (planner, logs) breaks.
