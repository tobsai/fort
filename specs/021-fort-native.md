# 021 — Fort-native (Go): deterministic orchestration + native execution

**Status:** implemented · **Source of truth:** [`../Agent Ops Backlog/`](../Agent%20Ops%20Backlog/) (rev. 2)
**Supersedes (where they conflict):** 001, 004, 006, 010, 017, 020 — see "Reconciliation".

> Governing spec for the **fort-native** rebuild. Per the Agent Ops Backlog
> (rev. 2) and the AO-090 decision (recorded in `docs/notes/decisions.md`), Fort
> is rebuilt **native, in Go, as one repo with three modules**. The existing
> TypeScript Fort under `packages/` is retained as **legacy/parallel**. Where the
> backlog disagrees with the legacy TS specs, **the backlog wins** and this spec
> records it.

## Goal
A deterministic router + native executor + DAG/gate engine + interface, built
native: a task auto-routes (no model in the routing path) and runs by spawning
the agent CLIs (`claude`/`codex`/`openclaw`/`hermes`) directly, with flows that
gate on humans and retry deterministically.

## Approach (as built)
- **Language/layout (AO-090):** Go. Modules `core/` (orchestration), `exec/`
  (native execution), `ui/` (interface), plus `rules/`, `flows/`, `cmd/fort/`,
  `docs/notes/`. Seam enforced by `core/arch_test.go` (core ↛ ui/exec).
- **Routing (AO-012/013/017):** YAML ruleset, first-match-wins, deterministic
  matchers (label/path/repo/@agent/size/time + any/all). **Zero model calls.**
- **Execution (AO-014):** `runtime.Runtime` interface; `exec/native` spawns CLIs
  and normalizes stdout → events; `exec/fake` for tests; `exec/gateway` adds
  budgets/tracing/failover. The composed execution runtime cancels and fails an
  invocation after 10 minutes with no emitted event, preventing a silent
  provider or descendant tool from holding a run forever.
- **State (AO-016):** SQLite `run`/`node_run`/`route_decision`/append-only
  `event`. At daemon startup, direct-task rows left `running` by an earlier
  daemon lifetime are atomically failed with an interruption event. Flow rows
  retain their durable node state for explicit resume.
- **Graph (AO-021–028):** node types task/gate/check/transform/fanout/fanin,
  conditional edges, retry→escalate, resumable; cron/once scheduler.
- **Interface (AO-031–037):** event/command contract, SSE live-feed, board,
  chat (templates, not an LLM planner), gate inbox, OpenClaw channel, iOS shell.
- **Hardening/dist (AO-041–043):** scoped workdir + env allowlist, threat model,
  spend caps, GoReleaser + Homebrew tap.

## Affected files
`core/**`, `exec/**`, `ui/**`, `rules/v1.yaml`, `flows/*.yaml`, `cmd/fort/**`,
`.goreleaser.yaml`, `Makefile`, `Formula/fort.rb`, `docs/notes/**`.

## Test criteria (all implemented, `go test ./...`)
- Routing is deterministic and ≥90% accurate on a sample (100%): `core/router`.
- Only `task` nodes invoke the runtime; flows resume after restart: `core/graph`.
- `ship-feature` runs unattended except at two gates: `core/flow`.
- Native runtime streams a real binary's output; signal/cancel work: `exec/native`.
- Silent runtime invocations are canceled and failed; activity resets the
  silence deadline.
- Startup reconciles interrupted direct-task `running` rows exactly once
  without changing resumable flow rows.
- Per-flow spend cap enforced; UI contract round-trips: `exec/gateway`, `ui`.

## Reconciliation (backlog vs legacy TS specs)
| Legacy spec | Conflict | Resolution (backlog wins) |
|---|---|---|
| 001 chat-mvp, 010 agent-handoff | LLM-based orchestration/handoff/triage | fort-native routing is **deterministic**, no model in the routing path |
| 004 agent-management | internal user-created LLM agents | fort-native "agents" are **external CLIs** dispatched by `NativeRuntime` |
| 006 llm-provider-config | in-process LLM client | providers are the **CLIs**; keys per-CLI via env/gateway (`.env.example`) |
| 017 openclaw-import | OpenClaw as one-shot import | fort-native adds a **live inbound channel** (AO-036, `POST /api/openclaw`) |
| 020 gated-model-choice | model-choice gate over WS | generalized to the **gate node** + gate-inbox contract (AO-023/031/035) |

Specs 001–020 describe the **retired v1 TypeScript prototype**, which has been
removed from the tree (recover from git history if needed). They are kept as
design history and do not govern fort-native.

## Rollback
fort-native is a self-contained Go module at the repo root. Rollback = revert the
`feat(fort-native)` / `feat(control-plane)` commits (or the merge). The retired v1
TypeScript prototype remains fully recoverable from git history (the commit that
removed it).
