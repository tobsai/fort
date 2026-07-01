# 022 — Multi-machine orchestration (control plane for many hosts)

**Status:** proposed · **New capability — requires Toby's approval before merge.**
**Governed by:** [021-fort-native](021-fort-native.md) · **Source of truth:** `Agent Ops Backlog/`

> Author's note: designed and implemented on a feature branch under Toby's
> standing "take your best guess" authorization while he was away. Every
> non-obvious decision is recorded under **Decisions** so it can be corrected on
> review. Nothing is pushed, merged, or released until Toby approves.

## Goal
Make Fort aware of more than one machine (initially a **Mac Mini** and a
**MacBook Pro**) and let a single control plane orchestrate agent runs across
them. From either machine you can watch the board, chat, and gate; a task routed
to an agent runs on whichever machine offers that agent — local or remote —
while the append-only event log, board, and feed stay authoritative on the
control plane. Determinism is preserved: **zero model calls** decide either the
agent (routing) or the machine (placement).

## Non-goals (v1 — YAGNI)
- No auto-discovery (mDNS/Bonjour): the machine roster is static config.
- No distributed DAG: flow (`graph.Executor`) task nodes run on the local node.
  Cross-machine flows are a later spec.
- No cross-machine failover/load-balancing beyond deterministic placement.
- No board authentication: the board stays open on the bind address; only the
  inter-Fort execution endpoint is authenticated. (See Decision D5.)

## Approach (as built)

### Topology — hub-and-spoke over the existing runtime seam
One Fort is the **control plane** (board/chat/gates/store/scheduler + UI). Every
machine that can run agents is an **execution node** exposing an authenticated
execution endpoint. A machine is typically both (a `fort serve` is control plane
+ local node). The control plane you look at can be either machine, or you can
run one on each — they are independent control planes that both target both
nodes.

The keystone is that `core/runtime.Runtime` already abstracts "dispatch a run and
stream its events." Remote execution is **just another `Runtime`**:

- `exec/remote.Runtime` implements `runtime.Runtime` by POSTing a `RunSpec` to a
  node's `/api/exec` and streaming the node's `RunEvent`s back (ndjson).
- `exec/cluster.Runtime` implements `runtime.Runtime` as a composite: it
  dispatches by `RunSpec.Machine` — local name → the local `native.Runtime`;
  any other name → that machine's `remote.Runtime`.
- `exec/node.Server` is the node side: an authenticated HTTP surface that runs a
  received `RunSpec` on the local `native.Runtime` and streams events out.

`core/engine` is unchanged in shape: it still calls `rt.Dispatch(spec)` and drains
`run.Stream()`. The composite `cluster.Runtime` is wired in at `cmd/fort` (the
composition root), so the core → exec seam (`core/arch_test.go`) is untouched.

### Machine registry — static, deterministic
`core/machines` (pure data + loader, no ui/exec imports) parses `machines.yaml`:

```yaml
version: 1
machines:
  - name: mac-mini
    url: http://mac-mini.local:4087
    agents: [claude, codex, hermes, openclaw]
  - name: macbook-pro
    url: http://macbook-pro.local:4087
    agents: [claude, codex]
```

- `name` — stable machine identity. The local machine is the entry whose `name`
  equals `FORT_NODE_NAME` (default: the OS hostname).
- `url` — base URL of that machine's Fort HTTP API (must be LAN-reachable; set
  `FORT_ADDR=0.0.0.0:4087` on a node so peers can reach it).
- `agents` — which providers that machine can run (placement input).

Absent `FORT_MACHINES`, Fort is single-machine and behaves **exactly as today**.

### Placement — deterministic, in core
After the router picks the agent (unchanged), `core/machines.Registry.Place`
picks the machine, as a pure function:

1. If the task pins a machine (`Task.Machine`), it must exist and offer the
   agent, else the run fails with a clear error.
2. Else prefer the **local** machine if it offers the agent.
3. Else the first machine in registry order that offers the agent.
4. Else fail: "no machine offers agent X".

`core/engine` gains an optional `Placer` (nil in single-machine mode). When set,
it computes the machine, records it on the run, and stamps `RunSpec.Machine`.
Placement is asserted model-free (`TestPlacementIsDeterministic`), mirroring
`TestRoutingIsDeterministic`.

### Inter-Fort execution protocol (`/api/exec`)
- `POST /api/exec` — `Authorization: Bearer <FORT_NODE_TOKEN>`, body = `RunSpec`
  JSON. Response: `application/x-ndjson`, one `RunEvent` JSON object per line,
  flushed live, until the run terminates (final `exited`/`error` event), then EOF.
  Client disconnect (context cancel) cancels the local run.
- `POST /api/exec/{id}/signal` — inject HITL stdin into a running remote run.
- `POST /api/exec/{id}/cancel` — cancel a running remote run.

`exec/remote.Runtime` returns a `Run` whose `Stream()` yields the decoded events;
`Wait()`/`Status()` derive the terminal state from the last `exited`/`error`
event (or `canceled` on `Cancel()`, or `failed` if the stream ends with no
terminal event).

### Machine awareness in the control plane (board + chat)
- `store.Run` gains a `machine` column; `ui.RunSummary` gains `machine`. The
  board tags each run with its machine.
- `GET /api/machines` returns the roster (name, agents, local, reachable). A
  small control-side poller pings each node's `/health` to maintain `reachable`,
  surfaced through a `ui.MachineLister` port (nil ⇒ single-machine ⇒ `[]`).
- `ChatRequest` and `fort task add`/`fort route` gain an optional `machine`
  (pin). The board shows the roster and lets chat target a machine.

### Distribution (brew readiness — folds in WS4)
So `brew install fort` works without a checked-out repo:
- Embed the default ruleset (`rules/v1.yaml`) via `go:embed`; `buildApp` falls
  back to it when `FORT_RULES` points at a missing default path.
- Tolerate a missing flows directory (serve with zero flows instead of erroring).
- Ship `machines.example.yaml`; document all new `FORT_*` vars in `.env.example`.

## Decisions (best-guess; correct on review)
- **D1 — Hub-and-spoke, not peer mesh or shared DB.** The `runtime.Runtime` seam
  makes remote execution a drop-in; a mesh/gossip layer or a shared network DB
  would be far more code and violate local-first + simplicity for two machines.
- **D2 — Static `machines.yaml`, not mDNS.** Deterministic and inspectable; two
  known Macs don't need discovery. mDNS is a clean future add.
- **D3 — Placement lives in core and is deterministic.** Choosing a machine is
  part of routing's spirit (no model), so it belongs beside the router and is
  asserted model-free.
- **D4 — ndjson, not SSE, for `/api/exec`.** Simpler to produce/consume in Go for
  a Fort→Fort channel; the browser-facing `/api/events` stays SSE.
- **D5 — Bearer token + LAN trust; board stays unauthenticated.** Pragmatic for a
  home LAN with two Macs. The exec endpoint runs arbitrary agent CLIs, so it is
  always authenticated; exposing the board is the operator's choice via
  `FORT_ADDR`. Board auth is a future add if Fort leaves the LAN.
- **D6 — `fort serve` default is unchanged (single-machine).** Multi-machine is
  opt-in via `FORT_MACHINES`; with it unset, no registry, no placement, no exec
  endpoint — byte-for-byte today's behavior.
- **D7 — Flows run on the local node in v1.** Threading machine placement through
  `graph.Node` is deferred to keep scope bounded.

## Affected files
- New: `core/machines/{machines.go,machines_test.go}`,
  `exec/remote/{remote.go,remote_test.go}`,
  `exec/cluster/{cluster.go,cluster_test.go}`,
  `exec/node/{node.go,node_test.go}`, `machines.example.yaml`,
  `specs/022-multi-machine-orchestration.md`.
- Changed: `core/runtime/runtime.go` (+`RunSpec.Machine`),
  `core/task/task.go` (+`Task.Machine`), `core/engine/engine.go` (+placer),
  `core/store/store.go` (+`machine` column, idempotent migration),
  `ui/contract.go` (+`RunSummary.Machine`, `MachineStatus`, `ChatRequest.Machine`),
  `ui/ports.go` (+`MachineLister`), `ui/server.go` (+`/api/machines`, machine on
  cards, chat pin), `ui/page.go` (roster panel + machine on cards + selector),
  `control/control.go` (roster + poller adapter, machine pass-through),
  `cmd/fort/{wire.go,main.go,control.go,flow.go}` (registry/cluster/node wiring,
  `--machine` flag, embedded-rules + missing-flows fallback),
  `core/config/config.go` (+`NodeName`,`MachinesPath`,`NodeToken`),
  `.env.example`, `.goreleaser.yaml` (extra_files if needed), `README.md`.

## Test criteria (`go test ./...`, `-race` on the streaming paths)
- `core/machines`: placement is deterministic and model-free; pin/local-pref/
  order/no-offer branches; registry parse + `Local`.
- `exec/remote` + `exec/node`: a real round-trip (node server backed by
  `exec/fake`) streams `started…message…exited` back through `remote.Runtime`;
  auth rejects a missing/bad token; `Cancel` stops the run; `Signal` reaches the
  node.
- `exec/cluster`: dispatches local vs remote by `RunSpec.Machine`; unknown
  machine errors.
- `core/engine`: with a placer, the run records its machine and stamps the spec;
  without a placer, behavior is unchanged.
- `core/store`: `machine` round-trips; migration is idempotent on an old DB.
- `ui`: `/api/machines` shape; board card carries `machine`; chat pin flows to
  the dispatcher. Existing `ui` contract tests stay green.
- Backward-compat: full single-machine suite stays green with `FORT_MACHINES`
  unset.

## Rollback
All new code is additive and gated by `FORT_MACHINES`. Rollback = revert the
`feat(multi-machine)` commits (or the merge); with the env var unset the feature
is inert, so a partial rollback (leaving code, disabling config) is also safe.
The `machine` column is nullable/append-only and harmless if left in place.
