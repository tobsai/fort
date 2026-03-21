---
sidebar_position: 1
title: Agents
---

# Agents

Fort runs on a registry of **core agents** and **specialist agents**. Core agents handle orchestration, memory, scheduling, and reflection. Specialist agents are purpose-built for specific domains and hatched on demand.

## Core Agents

Four agents ship with every Fort installation:

| Agent | Role |
|---|---|
| **Orchestrator** | Routes tasks to the right agent, manages thread lifecycle |
| **Memory** | Manages the memory graph — storing, linking, and retrieving knowledge |
| **Scheduler** | Executes routines on cron schedules, triggers time-based tasks |
| **Reflection** | Reviews completed tasks, extracts lessons, updates behaviors |

Core agents cannot be retired or forked. They are always present in the registry.

## Hatching Specialist Agents

Create new specialist agents with the Hatchery:

```bash
fort agents hatch \
  --name "Email Triager" \
  --description "Categorizes and prioritizes incoming email" \
  --capabilities "email_read,email_classify,email_draft" \
  --behaviors "Always flag messages from VIPs,Never auto-delete"
```

Each flag maps to the agent's YAML definition:

- `--name` — Human-readable agent name
- `--description` — What the agent does (injected into LLM context)
- `--capabilities` — Comma-separated list of registered tool names the agent can use
- `--behaviors` — Comma-separated behavioral rules applied during execution

### Hatching from a YAML File

For complex agents, define the full spec in YAML and hatch from file:

```yaml
# agents/code-reviewer.yaml
name: Code Reviewer
description: Reviews pull requests for style, bugs, and security issues
capabilities:
  - git_diff
  - code_search
  - comment_create
behaviors:
  - Never approve PRs that add dependencies without justification
  - Flag any use of eval() or dynamic code execution
  - Check for test coverage on new public functions
memory_partition: code-review
```

```bash
fort agents hatch --file agents/code-reviewer.yaml
```

## Agent Lifecycle

Agents move through a defined lifecycle:

```
hatch → active → retired → revived (back to active)
                ↑
          fork (creates new agent from existing)
```

**Fork** an agent to create a variant:

```bash
fort agents fork "Email Triager" --name "Slack Triager" \
  --capabilities "slack_read,slack_classify,slack_draft"
```

The forked agent inherits behaviors and description from the parent but gets its own memory partition.

**Retire** an agent when it is no longer needed:

```bash
fort agents retire "Email Triager"
```

Retired agents keep their data but stop receiving tasks. **Revive** them later:

```bash
fort agents revive "Email Triager"
```

## Storage

Agent definitions persist as YAML files in `.fort/data/agents/`. Each agent gets its own memory partition in the graph, so knowledge stays isolated unless explicitly shared.

```
.fort/data/agents/
├── orchestrator.yaml
├── memory.yaml
├── scheduler.yaml
├── reflection.yaml
├── email-triager.yaml
└── code-reviewer.yaml
```

## CLI Reference

List all registered agents with their status:

```bash
fort agents list
```

```
NAME              TYPE        STATUS   CAPABILITIES
Orchestrator      core        active   task_route,thread_manage
Memory            core        active   memory_store,memory_search
Scheduler         core        active   routine_exec,cron_manage
Reflection        core        active   task_review,behavior_update
Email Triager     specialist  active   email_read,email_classify
Code Reviewer     specialist  retired  git_diff,code_search
```

View the full identity (description, behaviors, capabilities) of an agent:

```bash
fort agents identities
```

Deep-inspect a single agent including memory stats and recent task history:

```bash
fort agents inspect "Email Triager"
```

```
Agent: Email Triager
Status: active
Memory Partition: email-triager (142 nodes, 87 edges)
Capabilities: email_read, email_classify, email_draft
Behaviors:
  - Always flag messages from VIPs
  - Never auto-delete
Recent Tasks: 23 (last 7 days)
Last Active: 2026-03-21T09:14:00Z
```

## How Agents Receive Work

The Orchestrator examines each incoming task, matches it against agent capabilities, and routes it. If no specialist matches, the Orchestrator handles it directly. After completion, the Reflection agent reviews the outcome and may update behaviors or suggest new specialist agents.
