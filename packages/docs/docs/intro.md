---
slug: /
sidebar_position: 1
title: Welcome to Fort
---

# Welcome to Fort

Fort is a self-improving personal AI agent platform. It combines long-lived specialist agents, graph-based memory, deterministic workflows, and macOS native integration into a single local-first system.

## Key Features

- **Specialist Agents** — Orchestrator, Memory, Scheduler, and Reflection agents work together, with support for custom specialists
- **Graph Memory** — SQLite-backed knowledge graph with optional MemU sidecar for semantic search
- **Deterministic Workflows** — Task graph tracks every operation; every decision has a rationale
- **macOS Native** — Swift menu bar app and Tauri + React dashboard
- **Polyglot** — TypeScript core, Swift shell, Python memory sidecar

## Architecture at a Glance

```
packages/
  core/       # TypeScript — module bus, task graph, agents, memory, permissions
  cli/        # TypeScript — fort command-line interface
  swift-shell/# Swift — macOS menu bar app
  dashboard/  # Tauri + React — local dashboard UI
```

All modules communicate through a typed **Module Bus**. Every operation creates a **Task** in the task graph, making the system fully inspectable.

## Quick Links

- [Installation](./getting-started/installation) — Get Fort running locally
- [Authentication](./getting-started/authentication) — Connect to Claude
- [First Steps](./getting-started/first-steps) — Verify your setup and explore core commands
