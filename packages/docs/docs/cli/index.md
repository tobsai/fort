---
sidebar_position: 0
slug: /cli
title: CLI Reference
---

# Fort CLI Reference

The Fort CLI is the primary interface for managing your Fort agent platform.

## Installation

```bash
npm link --workspace=packages/cli
```

Once linked, all commands are available as `fort <command>`.

## Global Usage

```
fort <command> [subcommand] [options]
```

All commands support `--help` for inline usage information.

## Commands

| Command | Description |
|---------|-------------|
| [`doctor`](/cli/doctor) | Run health checks across all modules |
| [`status`](/cli/status) | Show system overview and module stats |
| [`llm`](/cli/llm) | Configure and interact with LLM providers |
| [`agents`](/cli/agents) | Manage specialist and core agents |
| [`tasks`](/cli/tasks) | Inspect the task graph |
| [`threads`](/cli/threads) | Manage conversation threads |
| [`memory`](/cli/memory) | Query and export the memory graph |
| [`tools`](/cli/tools) | Browse and search the tool registry |
| [`tokens`](/cli/tokens) | Monitor token usage and budgets |
| [`behaviors`](/cli/behaviors) | Manage agent behavior rules |
| [`routines`](/cli/routines) | Create and run scheduled routines |
| [`schedule`](/cli/schedule) | View scheduled routines and triggers |
| [`flags`](/cli/flags) | Manage feature flags and rollouts |
| [`plugins`](/cli/plugins) | Load and manage plugins |
| [`harness`](/cli/harness) | Run goal-driven improvement cycles |
| [`rewind`](/cli/rewind) | Snapshot and restore system state |
| [`introspect`](/cli/introspect) | Inspect internal modules and capabilities |

## Examples

```bash
# Check system health
fort doctor

# See what's running
fort status

# Ask the LLM a question with memory context
fort llm ask "summarize my recent tasks" --memory "tasks"
```
