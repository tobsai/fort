---
sidebar_position: 9
title: fort tokens
---

# fort tokens

Monitor token usage across agents and models, and configure spending budgets.

## Usage

```
fort tokens <subcommand> [options]
```

## Subcommands

### stats

Display token usage statistics.

```
fort tokens stats [--by-agent] [--by-model] [--json]
```

| Option | Description |
|--------|-------------|
| `--by-agent` | Break down usage per agent |
| `--by-model` | Break down usage per model tier |
| `--json` | Output as JSON |

### budget

Show current budget configuration and remaining allowances.

```
fort tokens budget [--json]
```

### budget-set

Configure token spending limits.

```
fort tokens budget-set [options]
```

| Option | Description |
|--------|-------------|
| `--daily <n>` | Max tokens per day |
| `--monthly <n>` | Max tokens per month |
| `--per-task <n>` | Max tokens per individual task |
| `--warning <f>` | Warning threshold as a fraction (e.g., `0.8` for 80%) |

## Examples

```bash
# View usage broken down by agent
fort tokens stats --by-agent

# Set a daily budget with early warning
fort tokens budget-set --daily 500000 --warning 0.75

# Check remaining budget
fort tokens budget
```
