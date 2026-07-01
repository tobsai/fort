<p align="center">
  <img src="assets/fort-logo.png" alt="Fort" width="800" />
</p>

# Fort

**Deterministic agent orchestration — route work, run it natively, gate it at humans.**

Fort takes a task, routes it to the right agent by fixed rules (no model in the
routing path), runs it by spawning the agent CLIs itself, and sequences
multi-step work as a DAG that pauses at human gates. It runs as one native Go
binary and exposes a control plane you can drive from the web, iOS, macOS,
CarPlay, and Apple Watch.

> Built native per the [Agent Ops Backlog](./Agent%20Ops%20Backlog/) (rev. 2).
> The original TypeScript prototype was an experiment; this Go build is the
> delivered project. Governing spec: [`specs/021-fort-native.md`](./specs/021-fort-native.md).

## Two planes, two modes (one binary)

```sh
fort serve      # full plane: control + deterministic execution
fort control    # CONTROL PLANE ONLY — board, chat, scheduler, gate inbox, feed
                # no router/runtime/DAG, no agent CLIs needed; tasks are boarded
```

- **Control plane** — the human surfaces: Kanban board, chat, scheduler, gate
  inbox, live feed, and the client apps. Depends on nothing but the store.
- **Execution plane** (deterministic) — the router, the native runtime that
  spawns `claude`/`codex`/`hermes`/`openclaw`, and the DAG engine. Optional —
  plug it in for `fort serve`, leave it out for `fort control`.

## Architecture

One Go module, hard module seams (enforced by `core/arch_test.go`):

| Module | Role |
|---|---|
| `core/` | deterministic orchestration: rules, router, runtime interface, store, engine, graph, inbox, flow, scheduler, server |
| `exec/` | native execution: `NativeRuntime` (PTY-less CLI executor), `FakeRuntime`, `gateway` (budgets/tracing/failover) |
| `ui/` | control-plane HTTP/SSE API + web board; imports **none** of the execution components |
| `control/` | adapters wiring execution into the ui ports (or a queue-only dispatcher) |
| `rules/`, `flows/` | the routing ruleset and flow definitions (YAML) |
| `cmd/fort/` | the `fort` CLI |
| `ui/apple/` | FortKit Swift package + iOS / macOS / CarPlay / watch clients |

Design tenets: **routing is deterministic** (proven by tests, zero model calls);
**only `task` nodes invoke inference**; **every state change is an append-only
event** (the feed + board are derived, replayable).

## Quickstart

```sh
# install
brew install tobsai/tap/fort     # or: make build -> ./bin/fort (needs Go 1.22+)

fort control                     # control plane at http://127.0.0.1:4087/
# or, with the execution plane + agent CLIs installed:
fort serve
```

The binary embeds a default ruleset, so `fort serve` works from any directory
with no checked-out repo.

Then open the board at `/`, or drive the API:

```sh
fort route --dry-run --label bug "null deref"      # -> codex
fort task add --label research "read the repo"      # auto-route + run natively
fort flow run ship-feature --input "add search"     # DAG, pauses at gates
fort gate approve <run> plan_gate
```

`FORT_FAKE=1` runs a token-free fake runtime for demos/CI.

## Clients

All surfaces speak one HTTP/SSE contract ([`docs/notes/event-contract.md`](./docs/notes/event-contract.md)):

- **Web** — served at `GET /` (board, feed, chat, gate inbox).
- **Apple** — [`ui/apple`](./ui/apple): a shared **FortKit** package reused by iOS,
  macOS (menu bar), CarPlay, and watchOS (app + complication). `make apple-build`
  compiles them all. Deploy: [`docs/notes/testflight.md`](./docs/notes/testflight.md).

## Multi-machine ([spec 022](./specs/022-multi-machine-orchestration.md))

One control plane can orchestrate agents across several hosts (e.g. a Mac Mini +
a MacBook Pro). Point Fort at a machine registry and it routes each task to the
agent (deterministic, as always) and then to a **machine** that offers it — local
or remote — streaming the run back to the board you're watching. Remote execution
is just another `runtime.Runtime`, so the core is unchanged.

```sh
cp machines.example.yaml machines.yaml    # name + url + agents per host

# on each host that runs agents (expose on the LAN, share one token):
FORT_ADDR=0.0.0.0:4087 FORT_NODE_TOKEN=shared-secret fort serve

# on the host you drive (also knows the registry):
FORT_MACHINES=machines.yaml FORT_NODE_TOKEN=shared-secret \
  FORT_NODE_NAME=mac-mini FORT_ADDR=0.0.0.0:4087 fort serve
```

The board shows every host (with reachability) and tags each run with the machine
it ran on; chat and `fort task add --machine <host>` can pin a target. Placement
is deterministic: an explicit pin, else the local host if it offers the agent,
else the first host in the registry that does. Unset `FORT_MACHINES` ⇒ classic
single-machine mode. Inter-host `/api/exec` is bearer-token authenticated; keep it
on a trusted LAN.

## What needs you

- The execution plane spawns real agent CLIs (`claude`, `codex`, `hermes`,
  `openclaw`) with your provider keys — see `.env.example`. The `openclaw`
  invocation is a **best guess** ([spec 023](./specs/023-openclaw-provider.md))
  until the CLI is installed and probed; correct one line in
  `exec/native/providers.go` if it differs.
- CarPlay ships only with Apple's category-gated entitlement (see the TestFlight
  note); the code compiles and runs in the simulator regardless.

## Docs

`Agent Ops Backlog/` (the plan), `docs/notes/` (recon, decisions, threat model,
control-plane, event contract, distribution, TestFlight), `specs/` (specs).

## License

MIT © 2026 Tobias Gunn.
