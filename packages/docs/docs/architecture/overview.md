---
sidebar_position: 1
title: Architecture Overview
---

# Architecture Overview

Fort is a task-centric AI agent platform. It is not a chat wrapper. Every interaction creates a task with clear ownership and status. The architecture separates deterministic **services** from user-created **agents**.

## Packages

- **`packages/core`** -- TypeScript. Services, agents, task graph, memory, tools, and all core modules.
- **`packages/cli`** -- TypeScript. Commander.js CLI exposing every core capability.
- **`packages/swift-shell`** -- Swift/AppKit. macOS menu bar app connected over WebSocket IPC.
- **`packages/dashboard`** -- Tauri + React. Portal and dashboard at `http://localhost:4077`.

## Services vs Agents

### Services Layer

Services are deterministic infrastructure in `packages/core/src/services/`. They avoid LLM calls unless they hit ambiguity.

| Service | Role |
|---------|------|
| **Orchestrator** | Creates a task for every chat message, routes it to the correct agent. Does not answer questions itself. |
| **Memory** | Manages the SQLite-backed knowledge graph. Stores, links, and retrieves knowledge. |
| **Scheduler** | Executes routines on cron schedules, triggers time-based tasks. |
| **Reflection** | Periodically scans completed chat tasks for missed action items, creates follow-up tasks. Not event-driven. |

### Agent Layer

Agents are user-created specialists. Each has a **SOUL.md** that defines personality, goals, rules, voice, and boundaries. The SOUL.md is injected into the LLM system prompt at runtime.

Users create agents through the portal wizard at `http://localhost:4077` or via `fort agents create`. Each agent gets a directory with `identity.yaml`, `SOUL.md`, and `tools/`.

Chat goes to the user's default agent (or a selected long-lived agent). The orchestrator service creates the task and routes it.

## Design Principles

1. **Every interaction is a task.** The TaskGraph tracks every message as a first-class object with `result`, `assignedTo` ('agent' or 'user'), and status (To Do / In Progress / Done).
2. **Deterministic by default.** Services run deterministic logic first and escalate to an LLM only for ambiguity. Reflection is a periodic scan, not a reactive agent.
3. **Spec-driven development.** Before building any module, write a spec in `specs/`. Specs define goal, approach, affected files, test criteria, and rollback plan.
4. **Tool registry enforces reuse.** The ToolRegistry is checked before building anything new.
5. **Inspectability everywhere.** Every module exposes `diagnose()`. Every task has a log. `fort doctor` rolls up all diagnostics.

## Architecture Layers

```
+------------------------------------------------------+
|                   Native Clients                      |
|   Swift Menu Bar (AppKit)  |  Portal (Tauri+React)   |
+--------------+------------------+---------------------+
               |   WebSocket IPC  |
+--------------+------------------+---------------------+
|                     Fort Core                         |
|                                                       |
|  +-----------------------------------------------+   |
|  |               ModuleBus (pub/sub)              |   |
|  +-----------------------------------------------+   |
|                         |                             |
|  === Services (deterministic) ===                     |
|  +--------------+  +--------------+                   |
|  | Orchestrator |  | Reflection   |                   |
|  +--------------+  +--------------+                   |
|  +--------------+  +--------------+                   |
|  | Memory       |  | Scheduler    |                   |
|  +--------------+  +--------------+                   |
|                                                       |
|  === Core Modules ===                                 |
|  +-----------+  +---------------+  +--------------+   |
|  | TaskGraph |  | AgentRegistry |  | ToolRegistry |   |
|  +-----------+  +---------------+  +--------------+   |
|  +-----------+  +---------------+  +--------------+   |
|  | Threads   |  | Routines      |  | TokenTracker |   |
|  +-----------+  +---------------+  +--------------+   |
|  +-----------+  +---------------+  +--------------+   |
|  | Rewind    |  | Plugins       |  | LLMClient    |   |
|  +-----------+  +---------------+  +--------------+   |
|                                                       |
|  === Agents (user-created specialists) ===            |
|  +----------------------------------------------+    |
|  | Each agent: identity.yaml + SOUL.md + tools/  |    |
|  +----------------------------------------------+    |
+------------------------------------------------------+
|                   Storage Layer                        |
|          SQLite (better-sqlite3) per module            |
+------------------------------------------------------+
```

## The Fort Class

The `Fort` class in `packages/core/src/fort.ts` is the wiring point. It instantiates every module, connects them to the bus, initializes the four services, and registers all diagnostic providers with `FortDoctor`.

```typescript
const fort = new Fort({
  dataDir: '~/.fort',
  specsDir: './specs',
  memuUrl: 'http://localhost:8400',
});

await fort.start();          // initialize services, load agents, start IPC
const reply = await fort.chat('summarize my open tasks');
await fort.stop();           // graceful shutdown
```

## Module Communication

Modules never import each other directly. All communication flows through the [ModuleBus](./module-bus). A module publishes a typed event; any other module that cares subscribes to that event type.

## Diagnostics

Every module that implements `diagnose()` is registered with `FortDoctor`. Running `fort doctor` executes all providers and returns a roll-up of health checks:

```typescript
const results = await fort.runDoctor();
// Each result: { module, status: 'healthy'|'degraded'|'unhealthy', checks[] }
```
