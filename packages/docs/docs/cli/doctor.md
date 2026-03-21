---
sidebar_position: 1
title: fort doctor
---

# fort doctor

Run health checks across all Fort modules and report their status.

## Usage

```
fort doctor [options]
```

## Options

| Option | Description |
|--------|-------------|
| `--json` | Output results as JSON |
| `--module <name>` | Check a specific module only |

Each module reports one of two statuses:

- **healthy** — Module is operational and passing all checks.
- **degraded** — Module is running but one or more checks failed.

## Checked Modules

The doctor command runs `diagnose()` on every registered module, including: ModuleBus, TaskGraph, AgentRegistry, MemoryManager, ToolRegistry, PermissionManager, Scheduler, and any loaded plugins.

## Examples

```bash
# Run full health check
fort doctor

# Check only the memory module
fort doctor --module memory

# Get machine-readable output
fort doctor --json
```

## Output

```
Module          Status
──────────────  ────────
ModuleBus       healthy
TaskGraph       healthy
MemoryManager   degraded
ToolRegistry    healthy
Scheduler       healthy
```

When a module is degraded, the doctor prints diagnostic details below the summary table.
