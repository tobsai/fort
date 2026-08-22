<p align="center">
  <img src="assets/fort-logo.png" alt="Fort" width="800" />
</p>

# Fort

**A durable chat service for agents — across frameworks, conversations, and computers.**

In Fort, each Channel is a stable named **Agent** with one permanent Home
Conversation and optional pinned secondary Conversations. Groups put two to
six exact Agents in one durable chat. Additional Agent-to-Agent work happens
only through explicit, attributed, bounded **Handoffs**; it is never inferred
from prose or silently rerouted. Agent-owned Routines return their result to an
exact Conversation.

Under that product surface, Fort retains deterministic orchestration: routing
uses fixed rules (no model in the routing path), every execution pins an exact
behavior, framework profile, model, adapter, authority, and computer revision,
and multi-step work pauses at human gates. Native workers run local agent CLIs;
the approved cloud architecture uses Vercel for the stateless application tier
and Supabase Postgres for the durable ledger.

> Built native per the [Agent Ops Backlog](./Agent%20Ops%20Backlog/) (rev. 2).
> The original TypeScript prototype was an experiment; this Go build is the
> delivered project. The native foundation remains governed by
> [`specs/021-fort-native.md`](./specs/021-fort-native.md); the cloud Agent,
> Group, Handoff, and Routine model is governed by
> [`specs/047-vercel-supabase-cloud-control-plane.md`](./specs/047-vercel-supabase-cloud-control-plane.md)
> and [`specs/048-stable-agents-group-chats-and-handoffs.md`](./specs/048-stable-agents-group-chats-and-handoffs.md).

## Two planes, two modes (one binary)

```sh
fort serve      # full plane: control + deterministic execution
fort control    # LEGACY ADMIN / ROLLBACK CONTROL PLANE ONLY
                # no router/runtime/DAG or agent CLIs needed
```

- **Control plane** — the typed Primary Channels surface plus the durable
  scheduler, state APIs, and explicit off-mode legacy administration. Depends
  on nothing but the store.
- **Execution plane** (deterministic) — the router, the native runtime that
  spawns `claude`/`codex`/`hermes`/`openclaw`, and the DAG engine. Optional —
  plug it in for `fort serve`, leave it out for `fort control`.

## Architecture

One Go module, hard module seams (enforced by `core/arch_test.go`):

| Module | Role |
|---|---|
| `core/` | deterministic orchestration: rules, router, runtime interface, store, engine, graph, inbox, flow, scheduler, server |
| `cloud/` | stateless cloud-control contracts, signed service trust, and application-encrypted bodies |
| `api/` | bounded Vercel Go Function entrypoints; never starts a native runtime or permanent loop |
| `exec/` | native execution: `NativeRuntime` (PTY-less CLI executor), `FakeRuntime`, `gateway` (budgets/tracing/failover) |
| `ui/` | Primary Channels HTTP/SSE + Web, with off-mode legacy administration; imports **none** of the execution components |
| `control/` | adapters wiring execution into the ui ports (or a queue-only dispatcher) |
| `gateway/` | authenticated Next.js gateway and bounded reconnecting Node SSE transport |
| `supabase/` | private Postgres ledger migrations and database contract tests |
| `rules/`, `flows/` | the routing ruleset and flow definitions (YAML) |
| `cmd/fort/` | the `fort` CLI |
| `ui/apple/` | Phase 1 FortKit package + explicit iPhone and Mac clients |

Design tenets: **routing is deterministic** (proven by tests, zero model calls);
**only `task` nodes invoke inference**; **every state change is an append-only
event** (the feed + board are derived, replayable).

## Quickstart

```sh
# install
brew install tobsai/tap/fort     # or: make build -> ./bin/fort (needs Go 1.22+)

fort control                                  # explicit legacy admin/rollback
# Phase 1 preview, with the execution plane + required readiness inputs:
FORT_PRIMARY_CHANNELS=preview fort serve
```

The binary embeds a default ruleset, so `fort serve` works from any directory
with no checked-out repo.

Then open `/` for Private Channels. Setting `FORT_PRIMARY_CHANNELS=off` (the
default) intentionally restores the legacy admin surface instead. The CLI
continues to expose deterministic orchestration commands:

```sh
fort route --dry-run --label bug "null deref"      # -> codex
fort task add --label research "read the repo"      # auto-route + run natively
fort task breakdown "add search"                    # planner -> backlog sub-tasks
fort flow run ship-feature --input "add search"     # DAG, pauses at gates
fort gate approve <run> plan_gate
```

`FORT_FAKE=1` runs a token-free fake runtime for demos/CI.

Break a goal into backlog sub-tasks with `fort task breakdown "<goal>"` (or the
explicit off-mode legacy admin board). A planner agent (`FORT_PLANNER`, default
`claude`) decomposes it into `source=agent` items for deterministic dispatch.
It needs the execution plane (`fort serve`); control-only mode returns 409.

## Clients

The shipping clients share the typed Spec 044 Primary HTTP/SSE contract. The
older orchestration contract remains documented only for off-mode rollback and
administration in [`docs/notes/event-contract.md`](./docs/notes/event-contract.md).

- **Web** — served at `GET /` in `preview` or `primary`: private Channels,
  canonical transcripts, a text-only composer, read-only Scheduled, Needs you,
  and Settings/Recheck in the three device-local themes.
- **Apple** — [`ui/apple`](./ui/apple): the same Phase 1 contract in native
  iPhone and Mac clients through one closed **FortKit** package. The iPhone
  archive contains no Watch, complication, or CarPlay scene. `make apple-build`
  compiles both shipping targets. Deploy:
  [`docs/notes/testflight.md`](./docs/notes/testflight.md).

## Multi-machine ([spec 022](./specs/022-multi-machine-orchestration.md), [spec 024](./specs/024-mesh-enrollment.md))

One control plane can orchestrate agents across several hosts (e.g. a Mac Mini +
a MacBook Pro). Fort routes each task to the agent (deterministic, as always)
and then to a **machine** that offers it — local or remote — streaming the run
back to the board you're watching. Remote execution is just another
`runtime.Runtime`, so the core is unchanged.

The easy path is **`fort mesh`**: it mints and distributes the shared token and
manages the registry for you — no file edits.

```sh
# hub (laptop)
fort serve &
fort mesh invite            # prints: fort mesh join http://100.x.y.z:4087 --code XXXX-XXXX

# new machine (paste the printed line)
fort mesh join http://100.x.y.z:4087 --code XXXX-XXXX
fort serve
```

`fort mesh invite` mints the durable mesh token on first use (the hub then also
accepts inbound mesh exec) and prints a paste-ready `fort mesh join` line good
for one use within its TTL (`--ttl`, default 15m, capped at 1h). `fort mesh
join` probes `$PATH` for agent CLIs (or takes `--agents a,b`), registers with
the hub, and writes this machine's identity. `fort mesh remove <name>` drops a
machine from the registry — see the token-rotation runbook in
[`docs/notes/threat-model.md`](./docs/notes/threat-model.md) for what it does
*not* do.

The off-mode admin board shows every host and tags each run with its machine;
legacy chat and `fort task add --machine <host>` can pin a target. Placement
is deterministic: an explicit pin, else the local host if it offers the agent,
else the first host in the registry that does. Inter-host `/api/exec` is
bearer-token authenticated; keep it on a trusted LAN.

### Manual / hand-managed alternative

You can still hand-manage the registry and token instead of `fort mesh`:

```sh
cp machines.example.yaml machines.yaml    # name + url + agents per host

# on each host that runs agents (expose on the LAN, share one token):
FORT_ADDR=0.0.0.0:4087 FORT_NODE_TOKEN=shared-secret fort serve

# on the host you drive (also knows the registry):
FORT_MACHINES=machines.yaml FORT_NODE_TOKEN=shared-secret \
  FORT_NODE_NAME=mac-mini FORT_ADDR=0.0.0.0:4087 fort serve
```

Unset `FORT_MACHINES` ⇒ classic single-machine mode. When `FORT_MACHINES` is
set, `fort mesh` refuses to write to it (it only manages its own file).

## Provider boundary

- Discovering an installed framework does not authorize it for production.
  Every real runtime family keeps its own reviewed identity, readiness,
  lifecycle, authority, and terminal-normalization contract. Fake runtimes can
  prove deterministic state transitions but cannot satisfy cross-framework
  production acceptance.
- Provider credentials, CLI OAuth state, workspace files, browser sessions,
  and source-managed memory remain on the enrolled worker unless a later
  approved contract says otherwise.

## Docs

`Agent Ops Backlog/` (the plan), `docs/notes/` (recon, decisions, threat model,
control-plane, event contract, distribution, TestFlight), `specs/` (specs).

## License

MIT © 2026 Tobias Gunn.
