---
sidebar_position: 13
title: Self-Coding
description: Harness self-coding cycle for autonomous feature development
---

# Self-Coding

Fort can build features for itself through the harness self-coding cycle. This is a structured, auditable process that creates specs, writes code, runs tests, reviews its own work, and manages rollback through feature flags.

## The Cycle

When you start a harness run, Fort executes these steps in order:

1. **Spec** — Write a spec document in `specs/` defining the goal, approach, affected files, test criteria, and rollback plan.
2. **Branch** — Create a feature branch from `main`.
3. **Tool Check** — Search the tool registry for existing tools that handle the capability. Build only if nothing exists.
4. **Implement** — Write the code, following the spec.
5. **Test** — Run the test suite. All tests must pass.
6. **Self-Review** — Review the git diff. Check for missed edge cases, style violations, and spec adherence.
7. **Merge** — Merge the feature branch to `main`.
8. **Flag** — Create a feature flag for the new capability with a bake period.

## Starting a Harness Run

```bash
fort harness start --goal "Add CSV export to task graph"
```

Fort creates a task for the run and begins the cycle. You can watch progress:

```bash
fort tasks get <task-id>
```

Each step in the cycle is logged as a sub-task, so you have full visibility into what happened.

### With Options

```bash
fort harness start \
  --goal "Add retry logic to Gmail integration" \
  --bake-period 259200000 \
  --branch "feat/gmail-retry"
```

## Tool Registry Enforcement

Before building anything new, the harness checks the tool registry:

```bash
fort tools search "csv export"
```

If an existing tool handles the capability, the harness reuses it. This prevents duplicate implementations and keeps the codebase lean. The tool check is logged in the task graph so you can see what was found.

## Self-Review

After implementation and tests pass, Fort reviews its own diff:

```bash
git diff main...HEAD
```

The self-review checks for:

- Spec adherence — Does the code match what the spec described?
- Test coverage — Are the test criteria from the spec satisfied?
- Edge cases — Are error paths handled?
- Style consistency — Does the code follow project patterns?
- Security — Are there any obvious vulnerabilities?

If the review finds issues, Fort iterates: fix, test, review again.

## Feature Flags on Merge

When the harness merges a feature, it automatically creates a feature flag:

```bash
fort flags get csv-export
# Status: baking
# Bake period: 86400000ms
```

This means every self-built feature goes through the [Feature Flags](./feature-flags.md) bake period before being considered stable. If health checks fail during baking, the flag is rolled back automatically.

## Rollback

If a harness-built feature causes problems, roll it back:

```bash
fort harness rollback <harness-id> --reason "CSV export corrupts Unicode characters"
```

This:
- Rolls back the associated feature flag.
- Records the rollback reason in the task graph.
- Keeps the branch and code intact for later analysis.

## Garbage Collector

Over time, harness runs accumulate artifacts. The garbage collector cleans up:

```bash
fort harness gc
```

It finds and reports:

- **Stale specs** — Specs with no corresponding implementation or branch.
- **Unused tools** — Tools registered but never invoked.
- **Dead flags** — Feature flags that were rolled back and never retried.
- **Orphaned tasks** — Tasks with no parent or broken references.

Preview what would be cleaned:

```bash
fort harness gc --dry-run
# Stale specs: 3
# Unused tools: 1
# Dead flags: 2
# Orphaned tasks: 5
# Run without --dry-run to clean up
```

## Viewing History

List all harness runs:

```bash
fort harness list
# ID          Goal                              Status    Date
# h-a1b2c3d4  Add CSV export to task graph       merged    2026-03-18
# h-e5f6g7h8  Add retry logic to Gmail           baking    2026-03-20
# h-i9j0k1l2  Refactor scheduler queue           rolled_back  2026-03-15
```

Get details on a specific run:

```bash
fort harness get h-a1b2c3d4
```

This shows the full timeline: spec creation, branch, implementation, test results, review notes, merge, and flag status.

## How It Connects

The harness ties together several Fort systems:

- **Specs** define what to build (`specs/`)
- **Tool Registry** enforces reuse before build
- **Task Graph** tracks every step
- **Feature Flags** gate the rollout
- **ModuleBus** publishes `harness:started`, `harness:merged`, `harness:rolledback` events
