---
sidebar_position: 11
title: fort routines
---

# fort routines

Create and manage scheduled routines. Routines are repeatable multi-step workflows that run on a cron schedule or on demand.

## Usage

```
fort routines <subcommand> [options]
```

## Subcommands

### list

List all registered routines.

```
fort routines list [--json]
```

### add

Create a new routine.

```
fort routines add --name <n> --schedule <cron> [options]
```

| Option | Description |
|--------|-------------|
| `--name <n>` | Routine name (required) |
| `--schedule <cron>` | Cron expression for scheduling (required) |
| `--description <d>` | Human-readable description |
| `--steps <json>` | JSON array of step definitions |
| `--source <s>` | Origin (e.g., `user`, `reflection`) |

### run

Manually trigger a routine immediately, outside its schedule.

```
fort routines run <id>
```

### history

View past executions of a routine.

```
fort routines history <id> [--limit <n>]
```

### enable / disable

Toggle whether a routine runs on its schedule.

```
fort routines enable <id>
fort routines disable <id>
```

## Examples

```bash
# Add a daily standup summary routine
fort routines add --name "daily-summary" --schedule "0 9 * * 1-5" \
  --description "Summarize yesterday's tasks each weekday morning"

# Manually trigger a routine
fort routines run routine-daily-summary

# View the last 5 runs
fort routines history routine-daily-summary --limit 5
```
