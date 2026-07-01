# Control plane vs execution plane

Fort separates the human-facing **control plane** (board, chat, scheduler, gate
inbox, live feed — and the client surfaces: web, iOS, Mac, CarPlay, watch) from
the deterministic **execution plane** (router + native runtime + DAG engine).
You can run the control plane on its own — no router, no native runtime, no DAG,
and no agent CLIs installed.

## Two run modes (one binary)

```sh
fort serve      # full plane: control + execution (routes + runs natively)
fort control    # CONTROL PLANE ONLY: board, chat, scheduler, gate inbox, feed
```

`fort control` needs nothing but the `fort` binary itself — no `claude`/`codex`/
`hermes`/`openclaw`, no ruleset, no flows. Submitted tasks are **boarded as
`queued`**; an execution plane can pick them up later (run `fort serve` against
the same `FORT_DB`).

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

All surfaces speak the same HTTP/SSE contract
([`event-contract.md`](./event-contract.md)); the Apple clients share one Swift
package, **FortKit** (`ui/apple/FortKit`).

| Surface | Path | Consumes |
|---|---|---|
| Web | served at `GET /` | board + summary + SSE feed + chat + gates |
| iOS | `ui/apple/iOS` | board, chat, gates, feed (FortKit) |
| macOS | `ui/apple/macOS` | menu-bar summary + gate quick-approve (FortKit) |
| CarPlay | `ui/apple/CarPlay` | `CPListTemplate` gates + status, glanceable (FortKit) |
| watch | `ui/apple/watch` | glance counts + gate approve + complication (FortKit) |

`GET /api/summary` exists specifically for the constrained surfaces (watch
complication, CarPlay lists): a small counts payload + the pending gates, with an
`execution` flag so a client can show whether an execution plane is attached.
