---
sidebar_position: 4
title: fort agents
---

# fort agents

Manage core and specialist agents in the agent registry.

## Usage

```
fort agents <subcommand> [options]
```

## Subcommands

### list

List all registered agents.

```
fort agents list [--json]
```

### inspect

Show detailed information about a specific agent.

```
fort agents inspect <id> [--json]
```

### hatch

Create a new specialist agent.

```
fort agents hatch --name <n> --description <d> [options]
```

| Option | Description |
|--------|-------------|
| `--name <n>` | Agent name (required) |
| `--description <d>` | Agent description (required) |
| `--capabilities <c>` | Comma-separated capability list |
| `--behaviors <b>` | Comma-separated behavior rules |
| `--events <e>` | Comma-separated events to subscribe to |
| `--from <file>` | Load agent definition from a JSON or YAML file |

### retire

Deactivate an agent. Retired agents keep their history but stop receiving events.

```
fort agents retire <id> [--reason <r>]
```

### fork

Clone an existing agent with a new name.

```
fort agents fork <id> --name <n> [--description <d>]
```

### revive

Reactivate a previously retired agent.

```
fort agents revive <id>
```

### identities

List all agent identities and their roles.

```
fort agents identities [--json]
```

## Examples

```bash
# List all agents
fort agents list

# Create a code-review specialist
fort agents hatch --name "reviewer" --description "Reviews pull requests" \
  --capabilities "code-analysis,git" --events "task.created"

# Fork an agent to experiment with new behaviors
fort agents fork orchestrator-01 --name "orchestrator-v2"
```
