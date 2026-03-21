---
sidebar_position: 7
title: Flows
---

# Flows

Flows are deterministic workflows defined in YAML. Unlike free-form LLM conversations, flows execute a fixed sequence of steps with predictable behavior, branching logic, and error handling. Use flows when you need repeatable, auditable processes.

## Defining a Flow

Flows are YAML files with a list of typed steps:

```yaml
# flows/deploy-check.yaml
name: Deploy Readiness Check
description: Verify all conditions before deploying to production
steps:
  - id: run-tests
    type: action
    tool: test_runner
    params:
      suite: all

  - id: check-coverage
    type: condition
    expression: "steps.run-tests.output.coverage >= 80"
    then: build-artifact
    else: notify-low-coverage

  - id: notify-low-coverage
    type: notify
    event: flow:coverage-warning
    data:
      coverage: "{{ steps.run-tests.output.coverage }}"
    terminal: true

  - id: build-artifact
    type: action
    tool: build_tool
    params:
      target: production

  - id: review-changes
    type: llm
    prompt: "Review these changes and flag any risks: {{ steps.build-artifact.output.changelog }}"
    model: standard

  - id: deploy
    type: action
    tool: deploy_tool
    params:
      environment: production
      artifact: "{{ steps.build-artifact.output.path }}"
```

## Step Types

Flows support six step types:

### action

Calls a registered tool from the ToolRegistry:

```yaml
- id: send-email
  type: action
  tool: email_send
  params:
    to: "{{ context.recipient }}"
    subject: "Weekly Report"
    body: "{{ steps.generate-report.output.text }}"
```

### condition

Branches based on a JavaScript expression:

```yaml
- id: check-status
  type: condition
  expression: "steps.fetch-data.output.status === 'ok'"
  then: process-data
  else: handle-error
```

### transform

Transforms data between steps using JavaScript expressions:

```yaml
- id: format-results
  type: transform
  expression: |
    steps.fetch-data.output.items
      .filter(item => item.active)
      .map(item => ({ name: item.name, score: item.score }))
```

### llm

Sends a prompt to Claude for reasoning. Uses real Claude when API is configured, falls back to emitting a placeholder `llm:fallback` event on the ModuleBus when not:

```yaml
- id: analyze
  type: llm
  prompt: "Categorize this support ticket: {{ context.ticket_body }}"
  model: fast
```

### parallel

Runs multiple branches concurrently and waits for all to complete:

```yaml
- id: gather-data
  type: parallel
  branches:
    - id: fetch-metrics
      type: action
      tool: metrics_api
      params:
        range: 7d
    - id: fetch-alerts
      type: action
      tool: alerts_api
      params:
        severity: critical
```

### notify

Emits an event on the ModuleBus:

```yaml
- id: alert-team
  type: notify
  event: flow:deploy-complete
  data:
    version: "{{ steps.build-artifact.output.version }}"
    environment: production
```

Set `terminal: true` to end the flow after this step.

## Context Passing

Every step can read from two sources:

- `context` — Input data passed when the flow is triggered
- `steps.<id>.output` — Output from a previously completed step

Use `{{ }}` template syntax to interpolate values:

```yaml
params:
  message: "Build {{ steps.build.output.version }} deployed to {{ context.env }}"
```

## Error Policies

Each step can define what happens on failure:

```yaml
- id: risky-step
  type: action
  tool: external_api
  params:
    endpoint: /unstable
  on_error: retry    # retry, skip, or abort
  max_retries: 3
```

| Policy | Behavior |
|---|---|
| `abort` | Stop the entire flow (default) |
| `skip` | Log the error and continue to the next step |
| `retry` | Retry the step up to `max_retries` times |

## Running Flows

Execute a flow from the CLI:

```bash
fort flows run flows/deploy-check.yaml \
  --context '{"environment": "production"}'
```

Each flow execution creates a task in the TaskGraph with one subtask per step, giving full visibility into what happened.

List available flows:

```bash
fort flows list
```

Inspect the last execution of a flow:

```bash
fort flows history deploy-check
```

```
RUN   STATUS    STEPS   DURATION   STARTED
#12   success   6/6     34s        2026-03-21 14:00
#11   failed    3/6     8s         2026-03-20 14:00
#10   success   6/6     29s        2026-03-19 14:00
```
