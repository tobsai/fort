# AO-031 · fort-ui ↔ fort-core event/command contract

The interface fort-ui subscribes to (events) and calls back (commands). The
authoritative Go types live in [`ui/contract.go`](../../ui/contract.go); this is
the human-readable summary. Informed by the Agent Portal SSE/WS model
(`interface-contracts.md`), reimplemented natively over fort-core's append-only
event log.

## Events (subscribe) — `GET /api/events[?since=N]`

Server-Sent Events. Each append-only `event` row is one frame:

```
id: <int>
event: <type>
data: {"id":<int>,"run_id":"<id>","type":"<type>","data":"<text>","code":<int>,"time":"<rfc3339>"}
```

`?since=N` replays every event after cursor `N` (so **a run is replayable** from
`since=0`). Event `type` values come from the executor/runtime:
`started`, `stdout`, `stderr`, `message`, `exited`, `transform`.

A single run is also fetchable whole (replay without streaming):
`GET /api/runs/{id}` → `{run, nodes[], events[]}`.

## Commands (call back)

| Method & path | Body | Result |
|---|---|---|
| `GET /api/board` | — | `{runs[], gates[]}` — live board (AO-032) |
| `GET /api/summary` | — | `{total,running,queued,blocked,succeeded,failed,execution,gates[]}` — glanceable (watch/CarPlay) |
| `GET /api/runs/{id}` | — | `{run, nodes[], events[]}` — replayable |
| `GET /api/gates` | — | `[{run_id,node_id,input}]` — gate inbox |
| `POST /api/gate` | `{run_id,node_id,decision:"approve"\|"reject",edit?}` | `{state,paused_node?}` (AO-035) · **409** in control-only mode |
| `POST /api/chat` | `{text,agent?}` | `{kind:"task"\|"flow",run_id,route?,queued?,flow_id?,paused?}` (AO-034) |
| `POST /api/openclaw` | `{from,text}` | `{kind:"task",run_id,route?,queued?}` (AO-036) |

### Control-only mode
When Fort runs as a control plane with no execution plane (`fort control`), the
same contract holds with graceful degradation: `Summary.execution=false`,
`POST /api/chat` returns a boarded task (`queued:true`, no `route`, `"ship X"`
does not start a flow), and `POST /api/gate` returns **HTTP 409**. All read
endpoints work. See [`control-plane.md`](./control-plane.md).

### Chat → flow templates (deterministic, not an LLM planner)
`POST /api/chat` with text matching a template trigger instantiates the flow:
`"ship <X>"` → the `ship-feature` flow with input `<X>`. Anything else becomes a
routed task via the deterministic router. The mapping is a static table
(`matchFlow`), never a model call.

### Replayability
Because every state change is an append-only `event`, a client can reconstruct a
run's full history by reading `GET /api/runs/{id}` or by streaming
`GET /api/events?since=0`. The board is derived state (`run`/`node_run`),
rebuildable from the log.

## iOS (AO-037)
The iOS shell (`ui/ios/`) consumes the same `GET /api/board` + `POST /api/gate`
contract; `Codable` structs mirror `ui/contract.go`.
