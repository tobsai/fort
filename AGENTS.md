# Fort — Codex / Agent Directives

> This file mirrors `CLAUDE.md` for Codex and other coding agents. Keep the two in sync — when you change one, change the other.

## Project Overview
Fort is a self-improving personal AI agent platform. A ground-up replacement for OpenClaw combining long-lived specialist agents, graph-based memory, deterministic workflows, and macOS native integration.

## Architecture
- **Monorepo** with `packages/core` (TypeScript), `packages/cli` (TypeScript), `packages/swift-shell` (Swift), `packages/dashboard` (Tauri + React)
- **Three languages**: TypeScript (core logic), Swift (macOS native), Python (MemU memory sidecar)
- **Module Bus**: Event-driven backbone — all modules communicate via typed events
- **Task Graph**: Every conversation creates a task. Tasks are the atomic unit of transparency
- **Agent Registry**: Core agents (Orchestrator, Memory, Scheduler, Reflection) + specialist agents

## Core Design Principles

### Deterministic Tools
Fort uses tools that are **powered by AI but deterministic at runtime**. The LLM decides *when* to use a tool, but the tool itself is a bounded, testable function with predictable behavior — same input, same output.

Fort **never calls external tools directly**. Every capability goes through a Fort-owned tool that wraps the underlying industry tool and adds constraints. Example: Fort doesn't call Chrome MCP raw — it builds a `web-browse` tool that wraps Chrome MCP, adds an allowed-sites list, and exposes a predictable contract.

**Tool creation hierarchy:**
1. Check Fort's own ToolRegistry for an existing tool
2. If building new: use industry tools (npm packages, CLIs, APIs) as the engine
3. Fort owns the interface; industry tools provide the engine
4. **New tool specs require Toby's approval before implementation**

### Spec-Driven Development
All development follows: spec → approve → implement → verify → merge/rollback. No code lands without a spec. Specs live in `specs/` as machine-readable markdown.

## Behavioral Guidelines (Karpathy)
Guidelines to reduce common LLM coding mistakes. They bias toward caution over speed — for trivial tasks, use judgment.

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
1. **Spec-driven**: Before building any module, write a spec in `specs/`. Spec includes: goal, approach, affected files, test criteria, rollback plan. **Specs require approval before implementation begins.**
2. **Test-first**: Every module needs tests. Use Vitest for TypeScript, pytest for Python
3. **TypeScript strict return types**: `bus.subscribe()` callbacks must return `void`. Use `() => { array.push(x); }` not `() => array.push(x)` (the latter returns `number`, causing TS2322 in Docker builds)
3. **Tool Registry is sacred**: Before building anything new, search Fort's own tools first. Only build new if nothing fits. New tools wrap industry tools with deterministic constraints.
4. **Machine-readable everything**: Config has JSON Schema validation. Specs follow templates. Memory graph has defined schema
5. **Git discipline**: Feature branches for non-trivial changes. Every meaningful change committed
6. **Inspectability**: Every module exposes `diagnose()`. Every task has a log. Every decision has a rationale

## Key Patterns
- `ModuleBus` for all inter-module communication
- `TaskGraph` tracks every operation — never do work without creating a task
- `PermissionManager` enforces tiered action model (Tier 1: Auto, Tier 2: Draft, Tier 3: Approve, Tier 4: Never)
- `ToolRegistry` enforces reuse-before-build

## Tech Stack
- Runtime: Node.js / TypeScript
- Test: Vitest
- CLI: Commander.js
- DB: better-sqlite3
- Config: YAML + JSON Schema (ajv)
- Memory: MemU (Python sidecar) with SQLite fallback
- macOS: Swift/AppKit (menu bar) + Tauri (dashboard)

## File Naming
- Modules: `packages/core/src/<module>/index.ts`
- Tests: `packages/core/src/__tests__/<module>.test.ts`
- CLI commands: `packages/cli/src/commands/<command>.ts`
- Specs: `specs/<uuid>.md`
