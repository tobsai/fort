---
slug: /
sidebar_position: 1
title: Welcome to Fort
---

# Welcome to Fort

Fort is a task-centric personal AI agent platform. Every interaction creates a task with clear ownership -- not a conversation log. Deterministic services handle orchestration, memory, scheduling, and reflection. Users create long-lived specialist agents, each shaped by a SOUL.md that defines its personality and goals.

## Key Concepts

- **Tasks, not chat** -- Every message creates a task. The portal shows a kanban board (To Do / In Progress / Done) with `assignedTo` marking whether the agent or user owns it.
- **Services vs Agents** -- Orchestrator, Memory, Scheduler, and Reflection are deterministic services (not agents). Only user-created specialists are agents.
- **SOUL.md** -- Each agent has a SOUL.md defining personality, goals, rules, voice, and boundaries. This gets injected into the LLM system prompt.
- **Portal wizard** -- Create agents through the web portal at `http://localhost:4077`. The wizard collects name, goals, and emoji, then generates the SOUL.md.
- **Deterministic by default** -- Services avoid LLM calls unless they hit ambiguity. Reflection is a periodic scan, not an event-driven agent.

## Architecture at a Glance

```
packages/
  core/         # TypeScript -- services, agents, task graph, memory, tools
  cli/          # TypeScript -- fort command-line interface
  swift-shell/  # Swift -- macOS menu bar app
  dashboard/    # Tauri + React -- portal and dashboard UI
```

Services live in `packages/core/src/services/`. Agents are user-created specialists stored in `.fort/data/agents/` with their own `identity.yaml`, `SOUL.md`, and `tools/` directory.

All modules communicate through a typed **Module Bus**. Every operation creates a **Task** in the task graph.

## Quick Links

- [Installation](./getting-started/installation) -- Get Fort running locally
- [Authentication](./getting-started/authentication) -- Connect to Claude
- [First Steps](./getting-started/first-steps) -- Verify your setup and explore core commands
