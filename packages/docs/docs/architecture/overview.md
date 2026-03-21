---
sidebar_position: 1
title: Architecture Overview
---

# Architecture Overview

Fort is a polyglot AI agent platform built as a monorepo with four packages:

- **`packages/core`** -- TypeScript. The brain: module bus, task graph, agents, memory, permissions, tools, scheduler, and 15+ more modules.
- **`packages/cli`** -- TypeScript. Commander.js CLI that exposes every core capability as a terminal command.
- **`packages/swift-shell`** -- Swift/AppKit. macOS menu bar app connected to core over WebSocket IPC.
- **`packages/dashboard`** -- Tauri + React. Visual dashboard for tasks, agents, memory, and diagnostics.

## Design Principles

1. **Every conversation is a task.** The TaskGraph tracks every user interaction as a first-class object with status, assignment, and parent/child relationships.
2. **Deterministic by default.** LLM calls happen only when needed. Routines, flows, and scheduled work run deterministic steps first and escalate to an LLM only for ambiguity.
3. **Spec-driven development.** Before building any module, write a spec in `specs/`. Specs define goal, approach, affected files, test criteria, and rollback plan.
4. **Tool registry enforces reuse.** The ToolRegistry is checked before building anything new. If an existing tool handles the capability, you use it.
5. **Inspectability everywhere.** Every module exposes `diagnose()`. Every task has a log. Every decision has a rationale. `fort doctor` rolls up all diagnostics in one command.

## Architecture Layers

```
+------------------------------------------------------+
|                   Native Clients                      |
|   Swift Menu Bar (AppKit)  |  Dashboard (Tauri+React) |
+--------------+------------------+---------------------+
               |   WebSocket IPC  |
               |  ws://localhost:4001/shell              |
+--------------+------------------+---------------------+
|                     Fort Core                         |
|                                                       |
|  +-----------------------------------------------+   |
|  |               ModuleBus (pub/sub)              |   |
|  +-----------------------------------------------+   |
|                         |                             |
|  +-----------+  +---------------+  +--------------+   |
|  | TaskGraph |  | AgentRegistry |  | MemoryManager|   |
|  +-----------+  +---------------+  +--------------+   |
|                                                       |
|  +-----------+  +---------------+  +--------------+   |
|  | Scheduler |  | ToolRegistry  |  | Permissions  |   |
|  +-----------+  +---------------+  +--------------+   |
|                                                       |
|  +-----------+  +---------------+  +--------------+   |
|  | Threads   |  | Routines      |  | TokenTracker |   |
|  +-----------+  +---------------+  +--------------+   |
|                                                       |
|  +-----------+  +---------------+  +--------------+   |
|  | Rewind    |  | Plugins       |  | LLMClient    |   |
|  +-----------+  +---------------+  +--------------+   |
+------------------------------------------------------+
|                   Storage Layer                        |
|          SQLite (better-sqlite3) per module            |
+------------------------------------------------------+
```

## The Fort Class

The `Fort` class in `packages/core/src/fort.ts` is the wiring point. It instantiates every module, connects them to the bus, registers the four core agents (Orchestrator, Memory, Scheduler, Reflection), and registers all diagnostic providers with `FortDoctor`.

```typescript
const fort = new Fort({
  dataDir: '~/.fort',
  specsDir: './specs',
  memuUrl: 'http://localhost:8400',
});

await fort.start();          // initialize memory, load agents, start IPC
const reply = await fort.chat('summarize my open tasks');
await fort.stop();           // graceful shutdown of all modules
```

Startup is resilient -- IPC port conflicts do not block `fort.start()`. The bus emits `fort.started` and `fort.stopped` lifecycle events so every module can react to the application state.

## Module Communication

Modules never import each other directly. All communication flows through the [ModuleBus](./module-bus). A module publishes a typed event; any other module that cares subscribes to that event type. This keeps the dependency graph flat and makes it straightforward to add, remove, or replace modules without cascading changes.

## Diagnostics

Every module that implements `diagnose()` is registered with `FortDoctor`. Running `fort doctor` (or sending a `run_doctor` action over IPC) executes all providers and returns a roll-up of health checks:

```typescript
const results = await fort.runDoctor();
// Each result: { module, status: 'healthy'|'degraded'|'unhealthy', checks[] }
```

There are currently 18 diagnostic providers covering agents, tasks, memory, tools, tokens, threads, IPC, and more.
