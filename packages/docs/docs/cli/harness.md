---
sidebar_position: 15
title: fort harness
---

# fort harness

Run goal-driven self-improvement cycles. The harness proposes changes, runs them through approval, and manages rollback if needed.

## Usage

```
fort harness <subcommand> [options]
```

## Subcommands

### start

Begin a new improvement cycle with a stated goal.

```
fort harness start --goal <g> [options]
```

| Option | Description |
|--------|-------------|
| `--goal <g>` | The improvement goal (required) |
| `--approach <a>` | Suggested approach or strategy |
| `--files <f>` | Comma-separated list of files in scope |

### approve

Approve a pending cycle after reviewing its proposed changes.

```
fort harness approve <cycleId>
```

### status

View the status of improvement cycles.

```
fort harness status [cycleId] [--json]
```

Without a `cycleId`, lists all cycles. With one, shows full details including proposed changes and test results.

### rollback

Roll back an applied cycle and restore the previous state.

```
fort harness rollback <cycleId> --reason <r>
```

### gc

Garbage-collect old cycle artifacts.

```
fort harness gc [--clean] [--json]
```

| Option | Description |
|--------|-------------|
| `--clean` | Actually delete artifacts (default is dry run) |
| `--json` | Output as JSON |

## Examples

```bash
# Start an improvement cycle
fort harness start --goal "Reduce memory search latency" \
  --approach "Add caching layer" --files "packages/core/src/memory/index.ts"

# Review and approve the proposed changes
fort harness status cycle-abc123
fort harness approve cycle-abc123

# Roll back if something goes wrong
fort harness rollback cycle-abc123 --reason "Increased latency in production"
```
