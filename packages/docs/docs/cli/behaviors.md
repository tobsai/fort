---
sidebar_position: 10
title: fort behaviors
---

# fort behaviors

Manage behavior rules that shape how agents respond. Behaviors are context-scoped rules injected into agent prompts at runtime.

## Usage

```
fort behaviors <subcommand> [options]
```

## Subcommands

### list

List all registered behavior rules, optionally filtered by context.

```
fort behaviors list [--context <c>] [--json]
```

| Option | Description |
|--------|-------------|
| `--context <c>` | Filter by context (e.g., `code-review`, `planning`, `global`) |
| `--json` | Output as JSON |

### add

Add a new behavior rule.

```
fort behaviors add --rule <r> --context <c> [options]
```

| Option | Description |
|--------|-------------|
| `--rule <r>` | The behavior rule text (required) |
| `--context <c>` | Context scope for the rule (required) |
| `--priority <n>` | Priority level (higher runs first). Default: 0 |
| `--source <s>` | Origin of the rule (e.g., `user`, `reflection`) |

### remove

Remove a behavior rule by ID.

```
fort behaviors remove <id>
```

## Examples

```bash
# List all global behaviors
fort behaviors list --context global

# Add a code review rule
fort behaviors add --rule "Always check for error handling" \
  --context code-review --priority 10

# Remove an outdated rule
fort behaviors remove behavior-abc123
```
