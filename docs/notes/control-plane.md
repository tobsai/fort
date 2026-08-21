# Control plane vs execution plane

Fort separates durable **control-plane state and APIs** from the deterministic
**execution plane** (router + native runtime + DAG engine). Shipping clients
present Agent Channels as top-level destinations, nested durable Conversations,
Needs You, and Settings. Primary Channels are the explicit one-flag rollback;
the older board/chat/gate/feed surface remains off-mode administration.

## Two run modes (one binary)

```sh
fort serve      # full plane; product flags select Agent, Primary, or off mode
fort control    # legacy off-mode administration without an execution plane
```

`fort control` needs nothing but the `fort` binary itself — no `claude`/`codex`/
`hermes`/`openclaw`, no ruleset, no flows. It exists for legacy administration
and rollback; it is not the Agent Channels client experience.

Agent Channels are enabled only with `FORT_AGENT_CHANNELS=primary`, and that
value is valid only while `FORT_PRIMARY_CHANNELS=preview|primary`. The normal
rollback changes only `FORT_AGENT_CHANNELS` to `off`; it does not downgrade the
binary or rewrite conversations.

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

The shipping clients share the typed Agent Channels HTTP/SSE contract in Spec
046 and one Swift package, **FortKit** (`ui/apple/FortKit`). The Spec 044 Primary
contract stays mounted for rollback. The separate legacy off-mode API is
documented in [`event-contract.md`](./event-contract.md).

| Surface | Path | Consumes |
|---|---|---|
| Web | served at `GET /` | Agent Channels, nested Conversations, Needs You, Settings |
| iOS | `ui/apple/iOS` | Agent Channels over the authenticated relay (FortKit) |
| macOS | `ui/apple/macOS` | Agent Channels plus a bounded menu-bar glance (FortKit) |
