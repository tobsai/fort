---
sidebar_position: 4
title: fort agents
---

# fort agents

Manage specialist agents. Services (Orchestrator, Memory, Scheduler, Reflection) are not managed through this command -- they are deterministic infrastructure.

## Usage

```
fort agents <subcommand> [options]
```

## Subcommands

### list

List all specialist agents.

```
fort agents list [--json]
```

### inspect

Show detailed information about a specialist agent, including SOUL.md contents, task history, and memory stats.

```
fort agents inspect <id> [--json]
```

### create

Create a new specialist agent. This generates an agent directory with `identity.yaml`, `SOUL.md`, and `tools/`.

The recommended way to create agents is through the portal wizard at `http://localhost:4077`, which collects name, goals, and emoji interactively.

```
fort agents create --name <n> --goals <g> [options]
```

| Option | Description |
|--------|-------------|
| `--name <n>` | Agent name (required) |
| `--goals <g>` | What the agent should accomplish (required) |
| `--emoji <e>` | Visual identifier for the portal |

### fork

Clone an existing agent with a new name. The forked agent gets its own directory and a copy of the parent's SOUL.md.

```
fort agents fork <id> --name <n>
```

### retire

Deactivate an agent. Retired agents keep their data but stop receiving tasks.

```
fort agents retire <id> [--reason <r>]
```

### revive

Reactivate a previously retired agent.

```
fort agents revive <id>
```

## Examples

```bash
# List all specialist agents
fort agents list

# Create a specialist via CLI
fort agents create --name "Research Agent" --goals "Deep research and source synthesis"

# Fork an agent to create a variant
fort agents fork research-agent --name "Academic Researcher"

# Retire an agent
fort agents retire research-agent --reason "Replaced by Academic Researcher"

# Revive a retired agent
fort agents revive research-agent

# Inspect an agent
fort agents inspect research-agent
```
