---
sidebar_position: 3
title: First Steps
---

# First Steps

With Fort installed and authenticated, run through these commands to verify everything works.

## Health Check

```bash
fort doctor
```

Runs diagnostics across all modules -- checks SQLite connectivity, config validation, agent availability, and module bus health. Fix any issues it reports before continuing.

## System Overview

```bash
fort status
```

Displays the current state of Fort: active agents, task graph stats, memory status, and module bus activity.

## Test LLM Connectivity

```bash
fort llm ask "Hello"
```

Sends a prompt to Claude and prints the response. Confirms your authentication is working end-to-end.

## Explore Core Agents

```bash
fort agents list
```

Lists the four core agents and any registered specialists:

- **Orchestrator** — Routes tasks to the right agent
- **Memory** — Manages the knowledge graph
- **Scheduler** — Handles timed and recurring tasks
- **Reflection** — Reviews past decisions for self-improvement

## Check Memory

```bash
fort memory stats
```

Shows memory graph statistics: node count, edge count, and storage backend (SQLite or MemU).

## Token Usage

```bash
fort tokens
```

Displays cumulative token usage across sessions, broken down by model and task.

## Next Steps

- [Agents Guide](../guides/agents) — Learn how agents work and how to create specialists
- [LLM Guide](../guides/llm) — Configure models, context windows, and fallback behavior
