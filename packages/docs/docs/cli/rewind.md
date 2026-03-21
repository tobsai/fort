---
sidebar_position: 16
title: fort rewind
---

# fort rewind

Snapshot and restore system state. Rewind captures configuration, memory, and module state so you can safely experiment and roll back.

## Usage

```
fort rewind [options]
```

## Subcommands

### list

List available snapshots.

```
fort rewind [--limit <n>] [--json]
```

| Option | Description |
|--------|-------------|
| `--limit <n>` | Max snapshots to show. Default: 10 |
| `--json` | Output as JSON |

### create

Create a new snapshot of the current system state.

```
fort rewind --create [label]
```

An optional label helps identify the snapshot later. If omitted, a timestamp-based label is generated.

### preview

Preview what would change if you restored to a specific snapshot.

```
fort rewind --preview <id>
```

Shows a diff between the current state and the snapshot without making changes.

### restore

Restore the system to a previous snapshot.

```
fort rewind --to <id> [options]
```

| Option | Description |
|--------|-------------|
| `--config-only` | Only restore configuration files |
| `--memory-only` | Only restore the memory graph |

## Examples

```bash
# Create a snapshot before a risky change
fort rewind --create "before-memu-migration"

# Preview what restoring would change
fort rewind --preview snap-abc123

# Restore only config, keep current memory
fort rewind --to snap-abc123 --config-only
```
