# Fort — Codex / Agent Directives

> The authoritative directives live in [`CLAUDE.md`](./CLAUDE.md) and apply
> equally to Codex and any other coding agent. Read it. This file restates the
> load-bearing rules so they're not missed.

## What Fort is
A durable chat service for stable named Agents across frameworks and computers,
with canonical Home Conversations, secondary Conversations, multi-Agent Groups,
bounded durable Handoffs, and Agent-owned Routines. Execution remains
deterministic Go orchestration: route by fixed rules (no model in the routing
path), run exact approved agent CLI bindings natively, and pause DAGs at human
gates. Specs `047` and `048` govern the cloud product model;
`specs/021-fort-native.md` remains the native execution foundation.

## Non-negotiables
- **Test-first (TDD).** Write the failing `go test` first, watch it fail, then
  the minimal code to pass. Keep `go test ./...` green; `-race` for concurrency.
- **Determinism is asserted.** Zero model calls in the routing path; only `task`
  DAG nodes invoke the `Runtime`. Preserve those invariants with tests.
- **Respect the seams.** `core` must not import `ui` or a concrete `exec`
  package (only `runtime.Runtime`); `ui` must not import
  `engine`/`graph`/`router`/`native`. Enforced by `core/arch_test.go` +
  `go list -deps`.
- **Fort owns the interface.** Reach an industry CLI/library only through a
  bounded, testable Fort contract (e.g. a `NativeRuntime` provider). New
  capability specs require Toby's approval before implementation.
- **Spec-driven, surgical, simple.** Spec in `specs/` first; touch only what the
  task requires; minimum code that solves it. See CLAUDE.md's behavioral
  guidelines.

## Layout
`core/` (domain, rules, runtime iface, ledger/store, engine, graph, scheduler) ·
`cloud/` + `api/` (stateless Vercel control) · `supabase/` (private Postgres
ledger) · `gateway/` (authenticated web + bounded SSE) · `exec/` (native, fake,
gateway) · `ui/` + `ui/apple/` (local/Apple clients) · `control/` (ports) ·
`cmd/fort/` (CLI) · `rules/` + `flows/` (YAML).
