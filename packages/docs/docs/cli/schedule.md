---
sidebar_position: 12
title: fort schedule
---

# fort schedule

View all scheduled routines and event triggers in one place.

## Usage

```
fort schedule list [options]
```

## Options

| Option | Description |
|--------|-------------|
| `--json` | Output as JSON |

## Output

The schedule command displays a unified view of:

- **Routines** — Cron-scheduled workflows with their next run time and enabled state.
- **Event triggers** — Module bus events that automatically spawn tasks or invoke agents.

Each entry shows:

| Field | Description |
|-------|-------------|
| Name | Routine or trigger name |
| Schedule | Cron expression or event pattern |
| Next Run | Timestamp of the next scheduled execution |
| Status | `enabled` or `disabled` |
| Last Run | Timestamp of the most recent execution |

## Examples

```bash
# View all scheduled items
fort schedule list

# Export schedule for external monitoring
fort schedule list --json

# Combine with grep to find specific routines
fort schedule list --json | jq '.[] | select(.status == "enabled")'
```
