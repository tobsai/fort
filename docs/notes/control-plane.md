# Control plane vs execution plane

Fort separates durable **control-plane state and APIs** from the deterministic
**execution plane** (router + native runtime + DAG engine). The Phase 1 shipping
clients present only private Channels, Scheduled reads, Needs You, and Settings.
The older board/chat/gate/feed surface is an explicit off-mode administration
and rollback contract, not a sibling Phase 1 destination.

## Two run modes (one binary)

```sh
fort serve      # full plane; FORT_PRIMARY_CHANNELS selects Primary or off mode
fort control    # legacy off-mode administration without an execution plane
```

`fort control` needs nothing but the `fort` binary itself — no `claude`/`codex`/
`hermes`/`openclaw`, no ruleset, no flows. It exists for legacy administration
and rollback; it is not the Phase 1 client experience.

## How the seam works

The `ui` module talks to the rest of Fort only through two ports
(`ui/ports.go`) and imports **none** of `engine`/`graph`/`router`/`rules`/
`native` (verified: `go list -deps ./ui`):

- `Dispatcher.Submit(task) -> RunRef` — accept a task.
- `FlowRunner` — run/approve/resume flows (nil in control-only mode).

`cmd/fort` wires the concrete adapters (`control` package):

| Mode | Dispatcher | FlowRunner |
|---|---|---|
| `fort serve` (full) | `EngineDispatcher` (routes + runs natively) | `FlowExecutor` (DAG engine) |
| `fort control` (control-only) | `QueueDispatcher` (boards as `queued`) | `nil` |

### Graceful degradation with no execution plane
- `POST /api/chat` → boards a **queued** task (`{kind:"task",queued:true}`).
  `"ship X"` does **not** run a flow (no engine) — it's boarded too.
- `POST /api/gate` → **HTTP 409** (nothing to resume without the engine).
- `GET /api/board` · `/api/summary` · `/api/events` · `/api/runs/{id}` → work
  fully (they read the store).

## Client surfaces

The shipping clients share the typed Primary HTTP/SSE contract in Spec 044 and
one Swift package, **FortKit** (`ui/apple/FortKit`). The separate legacy
off-mode API is documented in [`event-contract.md`](./event-contract.md).

| Surface | Path | Consumes |
|---|---|---|
| Web | served at `GET /` | private Channels, Scheduled, Needs you, Settings |
| iOS | `ui/apple/iOS` | private Channels over the authenticated relay (FortKit) |
| macOS | `ui/apple/macOS` | private Channels plus a bounded menu-bar glance (FortKit) |
