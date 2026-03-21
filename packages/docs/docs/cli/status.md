---
sidebar_position: 2
title: fort status
---

# fort status

Display a system overview including agents, tasks, memory, tools, and scheduler state.

## Usage

```
fort status [options]
```

## Options

| Option | Description |
|--------|-------------|
| `--json` | Output results as JSON |

## Output Sections

- **Agents** — Count of active and retired agents.
- **Tasks** — Total, active, and blocked task counts.
- **Memory** — Number of nodes and edges in the memory graph.
- **Tools** — Total registered tools.
- **Scheduler** — Number of configured routines and their next run times.

## Examples

```bash
# Show system overview
fort status

# Get machine-readable output for scripting
fort status --json

# Combine with watch for a live dashboard
watch -n 5 fort status
```

## Output

```
Fort Status
───────────
Agents:     4 active, 1 retired
Tasks:      128 total, 3 active, 1 blocked
Memory:     342 nodes, 1,204 edges
Tools:      17 registered
Scheduler:  5 routines (3 enabled)
```
