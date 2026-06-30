# Agent Ops — Implementation Backlog

*Created June 30, 2026 · **rev. 2** · Owner: Toby (solo) · Source of truth for building **Fort** — one project: orchestration + execution + interface, built native*

> **What this is:** the ticketed backlog for the system in `../mtree - Agent Ops - Build vs Buy Analysis & Spec.md`. **rev. 2** reflects the decision to **consolidate everything under Fort and go Fort-native** — no rented Multica runtime, no separate Postern app. Postern, Agent Portal, and Multica now appear only as **references** the work draws from.
>
> **Architecture:** one repo, three modules — `fort-core` (deterministic orchestration), `fort-exec` (native CLI execution), `fort-ui` (chat/board/gate-inbox/iOS). Phases 0–2 are detailed to ticket level; Phase 3 (`fort-ui`) and Phase 4 (hardening) are concrete but lighter.
>
> **Estimates** are rough solo ideal-days: **S** ≤ 0.5d · **M** 1–2d · **L** 3–5d · **XL** > 1 week or a spike.

---

## What changed in rev. 2

- **Execution went native.** The old "stand up Multica + drive it via `MulticaBackend`" tickets are gone. Phase 1 now builds `fort-exec`'s **`NativeRuntime`** (spawns the CLIs in PTYs). Multica is studied and partially **vendored under its Apache-2.0 license** (AO-003), not run.
- **Postern is no longer a component.** The old "port into Postern" tickets become "build `fort-ui`, **informed by** Agent Portal/Postern contracts."
- **The de-risk/cutover phase is gone** (nothing to cut over from — you're native from day one). Phase 4 is now hardening + gateway + distribution.

---

## How to use this backlog

- **Human view:** this README (board + index) + one file per phase.
- **Machine view:** [`backlog.yaml`](./backlog.yaml) — the same tickets, importable into a tracker or as a Fort flow seed. Canonical for ingestion.

### Routing labels (a built-in dogfood test)

Each ticket's label maps to the Fort routing rule it would hit (§6.3 of the spec) — so running this backlog *through Fort* would route each ticket to the right agent:

| Label | Routes to | Use for |
|---|---|---|
| `dev` | **Codex** | implementation, refactors, tests, packaging |
| `design` / `chat` | **Claude** | API/schema design, rulesets, decisions, this doc |
| `errand` | **OpenClaw** | provisioning, secrets, env setup |
| `research` / `spike` | **Hermes** | repo reading, reconnaissance, write-ups |
| `decision` | **you** (human) | choices only you should make |

---

## The board

### Now — Phase 0 · Reconnaissance & setup → [phase-0-foundation.md](./phase-0-foundation.md)
Stand up the Fort monorepo; validate each CLI runs headless and document how `fort-exec` will drive it; study Multica's daemon; extract `fort-ui` contracts from Agent Portal/Postern.
**Exit gate:** you can run all four CLIs headless and capture stdout — and know exactly how `fort-exec` will drive them.

### Next — Phase 1 · `fort-core` + `fort-exec` → [phase-1-core-exec.md](./phase-1-core-exec.md)
Deterministic router + the native `NativeRuntime` that spawns the CLIs and streams output. Inbox → route → native dispatch.
**Exit gate:** a new task auto-routes and runs **natively** (no Multica) to completion, ≥90% routing accuracy.

### Next — Phase 2 · `fort-core` task-graph + gates → [phase-2-task-graph.md](./phase-2-task-graph.md)
DAG engine (`task`/`gate`/`check`/`transform`), retries, scheduler. Port the game-starter pipeline.
**Exit gate:** one multi-agent flow runs unattended except at gates.

### Later — Phase 3 · `fort-ui` interface → [phase-3-interface.md](./phase-3-interface.md)
Chat + board + gate-inbox + live feed + iOS, built into Fort, informed by Agent Portal/Postern contracts.
**Exit gate:** run a full day entirely from `fort-ui`.

### Later — Phase 4 · Hardening, gateway, distribution → [phase-4-hardening.md](./phase-4-hardening.md)
Security baseline, optional gateway (budgets/failover), packaging (Homebrew).
**Exit gate:** a clean install runs the full system with spend caps enforced.

### Always — Cross-cutting → [cross-cutting.md](./cross-cutting.md)
Language/module decision, licensing.

---

## Index (all tickets)

| ID | Title | Phase | Type | Pri | Est | Labels | Depends on |
|----|-------|:----:|------|:--:|:--:|--------|-----------|
| AO-001 | Set up the Fort monorepo (core/exec/ui seams) | 0 | task | P0 | M | dev | AO-090 |
| AO-002 | Runtime recon: drive each CLI headless | 0 | spike | P0 | M | spike | — |
| AO-003 | Study Multica's Apache-2.0 daemon; decide what to vendor | 0 | spike | P0 | M | spike | — |
| AO-004 | Extract fort-ui contracts from Agent Portal/Postern | 0 | spike | P1 | M | spike | — |
| AO-005 | Provider keys + secrets baseline | 0 | task | P0 | S | errand | — |
| AO-006 | Runtime topology decision (local vs VPS) | 0 | decision | P1 | S | decision | AO-002 |
| AO-011 | Scaffold fort-core service | 1 | task | P0 | M | dev | AO-001 |
| AO-012 | Routing-rule YAML schema + parser | 1 | task | P0 | M | dev | AO-011 |
| AO-013 | Deterministic matcher engine | 1 | task | P0 | L | dev | AO-012 |
| AO-014 | Runtime interface + NativeRuntime (PTY executor) | 1 | task | P0 | XL | dev | AO-002, AO-003, AO-011 |
| AO-015 | Inbox watcher → router → native dispatch | 1 | task | P0 | M | dev | AO-013, AO-014 |
| AO-016 | State store + route_decision + event log | 1 | task | P1 | M | dev | AO-011 |
| AO-017 | Author v1 ruleset (the four lanes) | 1 | task | P0 | S | design | AO-013 |
| AO-018 | `fort` CLI (dry-run, runs, logs) | 1 | task | P1 | M | dev | AO-013, AO-014 |
| AO-021 | DAG model + executor | 2 | task | P0 | L | dev | AO-015, AO-016 |
| AO-022 | `check` node | 2 | task | P0 | M | dev | AO-021 |
| AO-023 | `gate` node (human approve/edit/reject) | 2 | task | P0 | M | dev | AO-021 |
| AO-024 | `transform` node | 2 | task | P1 | M | dev | AO-021 |
| AO-025 | Retry + escalation semantics | 2 | task | P1 | M | dev | AO-021 |
| AO-026 | Port game-starter pipeline as Flow #1 | 2 | task | P1 | M | dev | AO-022, AO-024 |
| AO-027 | Author "ship a feature" Flow #2 | 2 | task | P1 | M | design | AO-022, AO-023 |
| AO-028 | Scheduler (cron/once triggers) | 2 | task | P2 | M | dev | AO-021 |
| AO-031 | fort-ui ↔ fort-core event/command contract | 3 | task | P1 | M | design | AO-016 |
| AO-032 | Board module (live runs/nodes/agents) | 3 | task | P1 | M | dev | AO-031 |
| AO-033 | Live-feed transport (SSE/WS) | 3 | task | P1 | M | dev | AO-031 |
| AO-034 | Chat surface wired to fort-core | 3 | task | P1 | L | dev | AO-031 |
| AO-035 | Gate-inbox UI (approve/edit/reject → signal) | 3 | task | P1 | M | dev | AO-023, AO-031 |
| AO-036 | OpenClaw channel (informed by Agent Portal) | 3 | task | P2 | M | dev | AO-004 |
| AO-037 | iOS shell pointed at Fort | 3 | task | P2 | M | dev | AO-032 |
| AO-041 | Security baseline (sandbox, least-privilege, keys) | 4 | task | P1 | M | dev | AO-005, AO-014 |
| AO-042 | Optional gateway (keys/budgets/failover) | 4 | task | P2 | M | dev | AO-014 |
| AO-043 | Packaging & distribution (Homebrew tap) | 4 | task | P2 | M | dev | AO-018 |
| AO-090 | Decide Fort's language + module boundaries | X | decision | P0 | S | decision | — |
| AO-091 | Open-core / licensing decision | X | decision | P2 | S | decision | — |

**34 tickets** · Phase 0: 6 · Phase 1: 8 · Phase 2: 8 · Phase 3: 7 · Phase 4: 3 · Cross-cutting: 2. Routing split: Codex 24 · human-decision 3 · Hermes 3 · Claude 3 · OpenClaw 1.

---

## Critical path

`AO-090 → AO-001 → (AO-002, AO-003) → AO-014 → AO-015 → AO-021 → (AO-022, AO-023) → AO-027`

The single most leveraged ticket is **AO-014** (the native `NativeRuntime`) — it's the piece you're now building instead of renting, and Phase 2+ all sit on it. De-risk it with **AO-002** (CLI recon) and **AO-003** (borrow Multica's daemon internals) first.
