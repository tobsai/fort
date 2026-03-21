---
sidebar_position: 17
title: fort introspect
---

# fort introspect

Inspect Fort's internal module graph, capabilities, and event system. Useful for debugging and understanding system structure.

## Usage

```
fort introspect [subcommand] [options]
```

When called without a subcommand, prints a summary of all modules and their status.

## Subcommands

### modules

List all registered modules with their dependencies and health status.

```
fort introspect modules [--json]
```

### capabilities

List all capabilities exposed by the system, grouped by providing module.

```
fort introspect capabilities [--json]
```

### events

List all event types on the module bus, including subscriber counts and recent activity.

```
fort introspect events [--json]
```

### search

Search across modules, capabilities, and events by keyword.

```
fort introspect search <query> [--json]
```

### export

Export the full introspection data for external analysis.

```
fort introspect export [--format json|markdown]
```

| Option | Description |
|--------|-------------|
| `--format` | Output format: `json` or `markdown`. Default: `json` |

## Examples

```bash
# Quick system overview
fort introspect

# Find everything related to scheduling
fort introspect search "schedule"

# Export full system map as markdown
fort introspect export --format markdown > system-map.md
```
