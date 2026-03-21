---
sidebar_position: 5
title: Behaviors
---

# Behaviors

Behaviors are rules injected into LLM prompts to shape how Fort's agents respond. They act as persistent instructions — once set, a behavior applies automatically whenever its context matches.

## How Behaviors Work

When Fort prepares an LLM call, it collects all behaviors matching the current context and injects them into the system prompt. The LLM sees them as first-class instructions alongside the agent's description.

```
System Prompt:
  [Agent description]
  [Matched behaviors]     ← injected here
  [Memory context]
  [User message]
```

This means behaviors shape every response without the user needing to repeat instructions.

## Adding Behaviors

Add a behavior with a context and priority:

```bash
fort behaviors add \
  --rule "Always include estimated completion time in task summaries" \
  --context tasks \
  --priority 7
```

- `--rule` — The instruction text injected into prompts
- `--context` — When this behavior applies (e.g., `email`, `code`, `tasks`, `general`)
- `--priority` — Importance from 1 (lowest) to 10 (highest); higher priority behaviors are listed first

### Context Matching

Behaviors only fire when the context matches the current operation:

| Context | When it applies |
|---|---|
| `general` | Every LLM call regardless of task type |
| `email` | Email triage, drafting, classification |
| `code` | Code review, generation, refactoring |
| `tasks` | Task planning, status updates, summaries |
| `memory` | Memory extraction and organization |
| `schedule` | Routine execution and scheduling decisions |

A behavior with context `general` applies everywhere. A behavior with context `email` only applies when an email-related agent or task is active.

## Listing Behaviors

List all behaviors:

```bash
fort behaviors list
```

```
ID    PRIORITY  CONTEXT   RULE
b-01  10        general   Never share API keys or credentials in responses
b-02  8         email     Always flag messages from VIPs with high priority
b-03  7         tasks     Include estimated completion time in task summaries
b-04  6         code      Suggest tests for any new public function
b-05  5         general   Prefer concise responses unless detail is requested
```

Filter by context:

```bash
fort behaviors list --context email
```

```
ID    PRIORITY  RULE
b-02  8         Always flag messages from VIPs with high priority
```

## Removing Behaviors

Remove a behavior by its ID:

```bash
fort behaviors remove b-03
```

## Priority and Ordering

When multiple behaviors match a context, they are sorted by priority (highest first) and injected in that order. This matters because LLMs tend to weight earlier instructions more heavily.

For critical rules, use priority 9-10:

```bash
fort behaviors add \
  --rule "Never auto-send emails without explicit user approval" \
  --context email \
  --priority 10
```

For nice-to-haves, use priority 1-3:

```bash
fort behaviors add \
  --rule "Use emoji in casual thread summaries" \
  --context general \
  --priority 2
```

## Storage

Behaviors are stored as nodes in the memory graph with type `behavior`. This means they are searchable, linkable, and exportable like any other memory node.

```bash
fort memory inspect --type behavior
```

```
ID    CONTENT                                               EDGES
b-01  Never share API keys or credentials in responses      1
b-02  Always flag messages from VIPs with high priority     2
b-03  Include estimated completion time in task summaries    1
```

Because behaviors live in the memory graph, the Reflection agent can analyze them — identifying redundant rules, suggesting new ones based on observed patterns, or flagging conflicting behaviors.

## Attaching Behaviors to Agents

When you hatch a specialist agent, its `--behaviors` flag creates behavior nodes scoped to that agent's partition:

```bash
fort agents hatch \
  --name "PR Reviewer" \
  --description "Reviews pull requests" \
  --capabilities "git_diff,code_search" \
  --behaviors "Flag any TODO comments,Require tests for new exports"
```

These behaviors only apply when the PR Reviewer agent is active. They do not leak into other agents' prompts.
