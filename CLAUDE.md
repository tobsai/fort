# Fort — Claude Code Directives

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
