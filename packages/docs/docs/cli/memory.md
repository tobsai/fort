---
sidebar_position: 7
title: fort memory
---

# fort memory

Query, search, and export the memory graph. Fort stores knowledge as nodes and edges in a SQLite-backed graph with optional MemU sidecar integration.

## Usage

```
fort memory <subcommand> [options]
```

## Subcommands

### inspect

Browse memory nodes with optional type filtering.

```
fort memory inspect [--type <t>] [--limit <n>] [--json]
```

| Option | Description |
|--------|-------------|
| `--type <t>` | Filter by node type (e.g., `fact`, `decision`, `entity`) |
| `--limit <n>` | Max nodes to return. Default: 20 |
| `--json` | Output as JSON |

### search

Semantic search across the memory graph.

```
fort memory search <query> [--json]
```

### history

View recent memory mutations.

```
fort memory history [--since <date>] [--limit <n>]
```

### export

Export the full memory graph.

```
fort memory export [--format json]
```

### stats

Show memory graph statistics: node counts by type, edge counts, and storage size.

```
fort memory stats
```

## Examples

```bash
# Search for everything related to authentication
fort memory search "authentication flow"

# Inspect recent decisions
fort memory inspect --type decision --limit 10

# Export graph for backup
fort memory export --format json > memory-backup.json
```
