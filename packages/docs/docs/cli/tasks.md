---
sidebar_position: 5
title: fort tasks
---

# fort tasks

Inspect the task graph. Every operation in Fort creates a task, making this the primary audit trail.

## Usage

```
fort tasks [taskId] [options]
```

When called with a `taskId`, displays full details for that task. Without an ID, lists tasks matching the filters.

## Options

| Option | Description |
|--------|-------------|
| `--all` | Include completed and failed tasks |
| `--agent <name>` | Filter by agent name |
| `--status <s>` | Filter by status: `pending`, `active`, `completed`, `failed`, `blocked` |
| `--search <q>` | Full-text search across task descriptions |
| `--json` | Output results as JSON |

## Task Details

When inspecting a single task, the output includes:

- **ID** and parent task (if any)
- **Agent** that owns the task
- **Status** and timestamps (created, started, completed)
- **Log** of all actions and decisions with rationale
- **Children** tasks spawned by this task

## Examples

```bash
# List active tasks
fort tasks --status active

# Inspect a specific task
fort tasks task-abc123

# Search for tasks related to memory
fort tasks --search "memory migration" --all
```
