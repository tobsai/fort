---
sidebar_position: 4
title: Threads
---

# Threads

Threads are Fort's conversation primitive. Every interaction — whether a quick question or a multi-day project — lives in a thread. Threads are backed by the TaskGraph, giving full traceability from conversation to execution.

## Creating Threads

Start a new thread from the CLI:

```bash
fort threads create --name "Refactor auth module"
```

```
Thread created: t-0047
Name: Refactor auth module
Status: active
Agent: Orchestrator (default)
```

Assign a thread to a specific agent and tag it with a project:

```bash
fort threads create \
  --name "Review Q2 metrics" \
  --agent "Data Analyst" \
  --project analytics
```

## Thread Lifecycle

Threads move through four states:

```
active → paused → active (resumed)
   ↓
resolved
```

**Pause** a thread to set it aside:

```bash
fort threads pause t-0047
```

**Resume** when ready to continue:

```bash
fort threads resume t-0047
```

**Resolve** when the work is complete:

```bash
fort threads resolve t-0047 --summary "Auth module refactored, tests passing"
```

Resolved threads are archived but remain searchable.

## Viewing Threads

List active threads:

```bash
fort threads list
```

```
ID       NAME                    STATUS   AGENT          PROJECT     CREATED
t-0047   Refactor auth module    active   Orchestrator   core        2026-03-20
t-0045   Review Q2 metrics       active   Data Analyst   analytics   2026-03-19
t-0041   Fix email parser        resolved Email Triager  email       2026-03-17
```

Show full thread history including messages and task references:

```bash
fort threads show t-0047
```

```
Thread: t-0047 — Refactor auth module
Status: active
Agent: Orchestrator
Project: core
Messages: 12
Tasks: 3

[2026-03-20 09:00] user: We need to refactor the auth module to use JWT
[2026-03-20 09:01] orchestrator: I'll break this into subtasks...
  → Task tk-112: Audit current auth implementation
  → Task tk-113: Design JWT token flow
  → Task tk-114: Implement and test
[2026-03-20 09:15] orchestrator: Audit complete. Current impl uses session cookies...
```

## Forking Threads

Fork a thread to branch off a conversation while preserving the original message history:

```bash
fort threads fork t-0047 --name "JWT refresh token strategy"
```

```
Forked thread: t-0048
Parent: t-0047 (Refactor auth module)
Messages copied: 12
```

The forked thread starts with the full message history of the parent. Both threads continue independently from that point.

## Cross-References

Threads can reference each other. When a thread mentions work from another thread, Fort creates a cross-reference link:

```bash
fort threads link t-0047 t-0048 --relation "spawned"
```

View cross-references:

```bash
fort threads refs t-0047
```

```
RELATION   THREAD   NAME
spawned    t-0048   JWT refresh token strategy
related    t-0041   Fix email parser
```

## Searching Threads

Full-text search across all thread messages:

```bash
fort threads search "JWT token"
```

```
t-0047  [2026-03-20 09:00] "...refactor the auth module to use JWT"
t-0048  [2026-03-20 10:30] "...JWT refresh token rotation every 24h..."
```

Filter by project or status:

```bash
fort threads search "deploy" --project core --status active
```

## TaskGraph Integration

Every thread is backed by a task in the TaskGraph. When an agent works on a thread, it creates subtasks that are children of the thread's root task. This means every action taken in a thread is logged, traceable, and auditable.

```
Thread t-0047 (root task tk-110)
├── tk-112: Audit current auth implementation (completed)
├── tk-113: Design JWT token flow (completed)
└── tk-114: Implement and test (in_progress)
```

Use `fort tasks inspect tk-112` to drill into any individual task's execution log.
