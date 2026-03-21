---
sidebar_position: 13
title: fort flags
---

# fort flags

Manage feature flags for safe, incremental rollouts. Flags support bake periods, spec linkage, and promotion/rollback workflows.

## Usage

```
fort flags <subcommand> [options]
```

## Subcommands

### list

List all feature flags.

```
fort flags list [--status <s>] [--json]
```

| Option | Description |
|--------|-------------|
| `--status <s>` | Filter by status: `enabled`, `disabled`, `baking`, `promoted`, `rolled-back` |
| `--json` | Output as JSON |

### create

Create a new feature flag.

```
fort flags create --name <n> --description <d> [options]
```

| Option | Description |
|--------|-------------|
| `--name <n>` | Flag name (required) |
| `--description <d>` | What the flag controls (required) |
| `--bake-period <ms>` | Bake period in milliseconds before auto-promotion |
| `--spec-id <id>` | Link to the spec that introduced this flag |

### enable / disable

Toggle a flag on or off.

```
fort flags enable <id> [--reason <r>]
fort flags disable <id> [--reason <r>]
```

### promote / rollback

Promote a baked flag to permanent, or roll it back.

```
fort flags promote <id> [--reason <r>]
fort flags rollback <id> [--reason <r>]
```

### check

Evaluate all flags and report their current effective state.

```
fort flags check [--json]
```

## Examples

```bash
# Create a flag for a new memory backend
fort flags create --name "memu-v2" --description "Use MemU v2 API" \
  --bake-period 86400000 --spec-id spec-abc123

# Enable the flag and start baking
fort flags enable flag-memu-v2 --reason "Testing in dev"

# Check all flag states
fort flags check --json
```
