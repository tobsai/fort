---
sidebar_position: 6
title: fort threads
---

# fort threads

Manage conversation threads. Threads are long-lived conversations that can span multiple tasks and agents.

## Usage

```
fort threads <subcommand> [options]
```

## Subcommands

### list

List threads, optionally filtered by project or agent.

```
fort threads [--all] [--project <tag>] [--agent <id>] [--json]
```

### create

Start a new conversation thread.

```
fort threads create --name <n> [options]
```

| Option | Description |
|--------|-------------|
| `--name <n>` | Thread name (required) |
| `--description <d>` | Thread description |
| `--project <p>` | Project tag for grouping |
| `--agent <a>` | Assign to a specific agent |
| `--parent <id>` | Parent thread ID for nesting |

### show

Display messages in a thread.

```
fort threads show <id> [--limit <n>] [--json]
```

### pause / resume / resolve

Change thread lifecycle state.

```
fort threads pause <id>
fort threads resume <id>
fort threads resolve <id>
```

### fork

Branch a thread from a specific message.

```
fort threads fork <id> --name <n> [--from-message <id>]
```

### search

Full-text search across all thread messages.

```
fort threads search <query> [--json]
```

## Examples

```bash
# Create a thread for a feature discussion
fort threads create --name "auth-redesign" --project backend

# View recent messages
fort threads show thread-abc --limit 20

# Fork a thread to explore an alternative approach
fort threads fork thread-abc --name "auth-redesign-alt" --from-message msg-42
```
