---
sidebar_position: 3
title: Memory
---

# Memory

Fort stores knowledge in a **graph-based memory system** backed by SQLite. The graph consists of typed nodes connected by edges, enabling rich relational queries across everything Fort learns.

## Node Types

Every piece of stored knowledge is a node with a type:

| Type | Purpose | Example |
|---|---|---|
| `person` | People Fort interacts with or knows about | "Alice — engineering lead" |
| `project` | Projects and workstreams | "Fort — AI agent platform" |
| `preference` | User preferences and settings | "Prefers dark mode terminals" |
| `fact` | Factual knowledge | "Company fiscal year starts April 1" |
| `decision` | Recorded decisions with rationale | "Chose SQLite over Postgres for portability" |
| `behavior` | Behavioral rules for agents | "Always summarize emails before acting" |
| `routine` | Scheduled routine definitions | "Morning standup summary at 9am" |
| `entity` | Generic entities that don't fit other types | "AWS us-east-1 production cluster" |

## Edges

Edges connect nodes to form a knowledge graph. Every edge has a relationship label:

```
[person: Alice] --manages--> [project: Fort]
[decision: Use SQLite] --relates_to--> [project: Fort]
[preference: Dark mode] --owned_by--> [person: User]
```

Edges are bidirectional for traversal but stored with a direction for semantics.

## Storing Memory

Memory is created automatically during task execution — the Memory agent extracts knowledge from conversations and stores it. You can also add nodes directly:

```bash
fort memory add --type fact --content "Deploy window is Tuesday 2-4pm UTC"
```

Link nodes with edges:

```bash
fort memory link <node-id-1> <node-id-2> --relation "relates_to"
```

## Searching and Inspecting

Full-text search across all memory:

```bash
fort memory search "deploy window"
```

```
ID    TYPE   CONTENT                              CREATED
m-42  fact   Deploy window is Tuesday 2-4pm UTC   2026-03-18
m-19  fact   Staging deploys run on Monday         2026-03-12
```

Filter by node type:

```bash
fort memory inspect --type fact
```

```
ID    CONTENT                                      EDGES
m-42  Deploy window is Tuesday 2-4pm UTC           2
m-19  Staging deploys run on Monday                1
m-08  Company fiscal year starts April 1           0
```

Inspect a single node and its edges:

```bash
fort memory inspect m-42
```

```
Node: m-42
Type: fact
Content: Deploy window is Tuesday 2-4pm UTC
Created: 2026-03-18T10:30:00Z
Edges:
  --relates_to--> [project: Fort] (m-03)
  --created_by--> [person: User] (m-01)
```

## Stats and Export

View memory statistics:

```bash
fort memory stats
```

```
Total Nodes: 347
Total Edges: 512
By Type:
  fact: 89
  person: 42
  project: 18
  preference: 31
  decision: 27
  behavior: 44
  routine: 12
  entity: 84
Storage: 2.4 MB
```

Export the full graph for backup or analysis:

```bash
fort memory export --format json > memory-backup.json
fort memory export --format csv > memory-backup.csv
```

## Agent Memory Partitions

Each agent gets its own memory partition. When the Email Triager stores a fact, it goes into the `email-triager` partition. Partitions prevent cross-contamination — one agent's knowledge does not leak into another's context unless explicitly shared.

The Memory agent can bridge partitions when cross-referencing is needed:

```bash
fort memory search "deploy" --partition code-reviewer
```

## MemU Sidecar Fallback

Fort can optionally connect to a **MemU** instance — a Python-based external memory sidecar — via HTTP. When MemU is available, Fort uses it for advanced semantic search and embedding-based retrieval. When MemU is unavailable, Fort falls back to the local SQLite graph with text-based search.

Configure MemU in `.fort/config.yaml`:

```yaml
memory:
  backend: sqlite          # or "memu"
  memu_url: http://localhost:8420
  memu_timeout_ms: 5000
  sqlite_path: .fort/data/memory.db
```

The fallback is automatic — if MemU stops responding, Fort switches to SQLite without interruption.
