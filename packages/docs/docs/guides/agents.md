---
sidebar_position: 1
title: Agents
---

# Agents

In Fort, **agents** are user-created specialists. The four core functions (orchestration, memory, scheduling, reflection) are handled by deterministic **services**, not agents. See [Architecture Overview](../architecture/overview) for details on services.

## Creating an Agent

The recommended way to create an agent is through the **portal wizard** at `http://localhost:4077`.

The wizard collects:

- **Name** -- human-readable agent name
- **Goals** -- what the agent should accomplish
- **Emoji** -- visual identifier for the portal kanban board

This generates a **SOUL.md** that defines the agent's personality, goals, rules, voice, and boundaries. The SOUL.md is injected into the LLM system prompt whenever the agent handles a task.

### Creating via CLI

```bash
fort agents create --name "Research Agent" --goals "Deep research and source synthesis"
```

### Creating from a file

For full control, write the SOUL.md and identity.yaml directly:

```
.fort/data/agents/research-agent/
  identity.yaml
  SOUL.md
  tools/
```

## SOUL.md

Each agent's SOUL.md is the core of its identity. It contains:

- **Personality** -- tone, style, approach
- **Goals** -- what the agent is trying to achieve
- **Rules** -- hard constraints on behavior
- **Voice** -- how the agent communicates
- **Boundaries** -- what the agent will not do

The SOUL.md is injected into the LLM system prompt at runtime. Edit it directly to fine-tune agent behavior.

## How Agents Receive Work

Chat messages go to the user's **default agent** (or a selected long-lived agent). The orchestrator service creates a task and routes it -- it does not answer questions itself.

Each task has:

- `result` -- the outcome
- `assignedTo` -- `'agent'` or `'user'`
- Status tracking on the portal kanban board

## Agent Lifecycle

```
create --> active --> retired --> revived (back to active)
                 |
           fork (creates new agent from existing)
```

**Create** a new agent:

```bash
fort agents create --name "Email Triager" --goals "Categorize and prioritize incoming email"
```

**Fork** an agent to create a variant:

```bash
fort agents fork email-triager --name "Slack Triager"
```

The forked agent inherits the parent's SOUL.md as a starting point but gets its own directory and can be independently edited.

**Retire** an agent when it is no longer needed:

```bash
fort agents retire email-triager
```

Retired agents keep their data but stop receiving tasks.

**Revive** a retired agent:

```bash
fort agents revive email-triager
```

## Storage

Each agent gets its own directory:

```
.fort/data/agents/
├── research-agent/
│   ├── identity.yaml
│   ├── SOUL.md
│   └── tools/
├── email-triager/
│   ├── identity.yaml
│   ├── SOUL.md
│   └── tools/
└── slack-triager/
    ├── identity.yaml
    ├── SOUL.md
    └── tools/
```

## CLI Reference

```bash
# List all specialist agents
fort agents list

# Create a specialist
fort agents create --name "Code Reviewer" --goals "Review PRs for style, bugs, security"

# Fork an agent
fort agents fork code-reviewer --name "Security Reviewer"

# Retire an agent
fort agents retire code-reviewer

# Revive a retired agent
fort agents revive code-reviewer

# Deep-inspect an agent
fort agents inspect code-reviewer
```
