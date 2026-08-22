# Fort — Claude Code Directives

## Project Overview
Fort is a durable chat service for stable named Agents across frameworks and
computers. Every Agent has one permanent Home Conversation; Groups use explicit
frozen recipients; additional Agent-to-Agent work is a bounded durable Handoff;
Agent-owned Routines report to an exact Conversation. Under that surface, Fort
routes by fixed rules (no model in the routing path), runs exact approved agent
CLI bindings natively, and sequences DAG work through human gates.

> This Go build (`fort-native`) is the delivered project, built per the
> `Agent Ops Backlog/` (rev. 2). The earlier TypeScript prototype was an
> experiment and has been removed (recover from git history if ever needed).
> Governing native spec: `specs/021-fort-native.md`. Specs `047` and `048`
> govern the Vercel/Supabase control plane and stable-Agent product model.

## Architecture
One Go module (`github.com/tobsai/fort`), hard module seams enforced by
`core/arch_test.go`:
- `core/` — deterministic orchestration: `rules`, `router`, `runtime` (the
  executor interface), `store` (SQLite), `engine`, `graph` (DAG), `inbox`,
  `flow`, `scheduler`, `server`, `config`.
- `cloud/` + `api/` — stateless cloud-control contracts and bounded Vercel Go
  Function entrypoints; never native execution or permanent loops.
- `supabase/` — private Postgres ledger migrations and database contract tests.
- `gateway/` — authenticated Next.js application tier and bounded Node SSE.
- `exec/` — native execution: `native.NativeRuntime` (spawns CLIs), `fake`
  (tests), `gateway` (budgets/tracing/failover). Implements `core/runtime.Runtime`.
- `ui/` — control-plane HTTP/SSE API + web board. Talks to the rest of Fort only
  through ports (`Dispatcher`, `FlowRunner`); imports **none** of the
  execution/deterministic packages.
- `control/` — adapters that plug execution into the ui ports (or a queue-only
  dispatcher for control-only mode).
- `cmd/fort/` — the CLI (composition root; the only place that imports a concrete
  runtime).
- `rules/`, `flows/` — YAML ruleset + flow definitions. `ui/apple/` — FortKit +
  the Apple clients.

**Two planes:** the **control plane** (board, chat, scheduler, gate inbox, feed,
clients) runs with no execution components (`fort control`); the **execution
plane** (router + native runtime + DAG) plugs in for `fort serve`.

## Core Design Principles

### Deterministic by default
Routing reads only deterministic signals on a task — **zero model calls in the
routing path** (asserted in tests). Inference happens **only** at `task` nodes.
Every state change is an append-only `event`; the board + feed are derived and
replayable.

### Fort owns the interface; industry tools are the engine
Fort never drives an external tool raw. Each agent CLI is reached through a
`NativeRuntime` **provider** that fixes its argv, normalizes its output into
`RunEvent`s, and adds constraints (scoped workdir, env allowlist, gateway spend
caps). New capabilities wrap an industry CLI/library behind a bounded, testable
Fort-owned contract. **New capability specs require Toby's approval before
implementation.**

### Spec-Driven Development
Development follows: spec → approve → implement → verify → merge/rollback. Specs
live in `specs/` as machine-readable markdown (goal, approach, affected files,
test criteria, rollback).

## Behavioral Guidelines (Karpathy)
Guidelines to reduce common LLM coding mistakes. They bias toward caution over
speed — for trivial tasks, use judgment.

### 1. Think Before Coding
**Don't assume. Don't hide confusion. Surface tradeoffs.** Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them — don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

### 2. Simplicity First
**Minimum code that solves the problem. Nothing speculative.**
- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.
- Ask: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

### 3. Surgical Changes
**Touch only what you must. Clean up only your own mess.** When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it — don't delete it.
- Remove imports/variables/functions that YOUR changes made unused; leave pre-existing dead code unless asked.
- The test: every changed line should trace directly to the user's request.

### 4. Goal-Driven Execution
**Define success criteria. Loop until verified.** Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan (`step → verify: check`). Strong success criteria let you loop independently; weak criteria ("make it work") require constant clarification.

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

## Development Rules
1. **Spec-driven**: Before building a module, write a spec in `specs/`. Specs require approval before implementation.
2. **Test-first (TDD)**: `go test ./...` must stay green. Write the failing test first, watch it fail, then write minimal code to pass. Run the race detector (`go test -race ./...`) on anything concurrent.
3. **Determinism is asserted**: the routing path takes zero model calls; only `task` nodes invoke the `Runtime`. Add tests that hold these invariants.
4. **Respect the seams**: `core` must not import `ui` or a concrete `exec` package (only `runtime.Runtime`); `ui` must not import `engine`/`graph`/`router`/`native`. `core/arch_test.go` and `go list -deps` enforce this.
5. **Git discipline**: feature branches for non-trivial work; every meaningful change committed. Commit/push only when asked.
6. **Inspectability**: structured logs; the append-only `event` log + `route_decision` make every dispatch and decision traceable and replayable.

## Key Patterns
- `runtime.Runtime` interface is the only path from `core` to execution; `cmd/fort` injects the concrete `exec/native` (or `exec/fake`) runtime.
- Deterministic router: ordered YAML rules, first-match-wins + default; matchers on label/path/repo/@agent/size/time with any/all.
- DAG executor: `task`/`gate`/`check`/`transform`/`fanout`/`fanin`, conditional edges, retry→escalate, resumable from persisted `node_run` state.
- Control-plane ports: `ui.Dispatcher` + `ui.FlowRunner`; the `control` package supplies real (engine) or queue-only adapters.

## Tech Stack
- Language: Go 1.22+ (single toolchain for core/exec/ui).
- Test: standard `testing` + `go test -race`.
- DB: SQLite via `modernc.org/sqlite` (pure Go, no cgo).
- Config/flows/rules: YAML (`gopkg.in/yaml.v3`).
- CLI: stdlib `flag`. Cron: `robfig/cron/v3`. Globs: `bmatcuk/doublestar`.
- Clients: Swift / SwiftUI (FortKit) for iOS / macOS / CarPlay / watch; a served HTML board for web.

## File Naming
- Packages: `core/<module>/`, `exec/<impl>/`, `ui/`, `control/`.
- Tests: `*_test.go` beside the code (external `<pkg>_test` package when it must import adapters).
- CLI: `cmd/fort/<command>.go`.
- Rules/flows: `rules/*.yaml`, `flows/*.yaml`. Specs: `specs/<NNN>-<slug>.md`.
