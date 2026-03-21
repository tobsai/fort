---
sidebar_position: 11
title: Feature Flags
description: SQLite-backed feature flags with bake periods and automatic rollback
---

# Feature Flags

Fort uses feature flags to safely roll out new capabilities. Flags are backed by SQLite and include a bake period mechanism that automatically promotes or rolls back features based on health checks.

## Lifecycle

Every feature flag moves through a defined status lifecycle:

```
disabled → baking → stable
                  ↘ rolled_back
```

- **disabled** — Flag is off. The feature is not active.
- **baking** — Flag is enabled and the bake timer is running. Health checks run periodically.
- **stable** — Bake period completed successfully. The feature is permanently on.
- **rolled_back** — Health checks failed during baking. The feature was automatically disabled.

## Creating a Flag

```bash
fort flags create \
  --name "gmail-batch-send" \
  --description "Enable batch email sending via Gmail integration" \
  --bake-period 86400000
```

The `--bake-period` is in milliseconds. Common values:

| Duration | Milliseconds |
|----------|-------------|
| 1 hour | 3600000 |
| 12 hours | 43200000 |
| 1 day | 86400000 |
| 3 days | 259200000 |
| 1 week | 604800000 |

A newly created flag starts in `disabled` status.

## Enabling a Flag

```bash
fort flags enable gmail-batch-send
```

This transitions the flag to `baking` and starts the bake timer. From this point, health checks determine the flag's fate.

## Health Checks

While a flag is baking, Fort runs periodic health checks. These verify that the feature is not causing errors, performance degradation, or unexpected behavior.

Run checks manually:

```bash
fort flags check
```

This evaluates all currently baking flags. If a flag's health checks pass and the bake period has elapsed, the flag is promoted to `stable`. If checks fail at any point during baking, the flag is automatically rolled back.

## Querying Flags

List all flags:

```bash
fort flags list
```

Filter by status:

```bash
fort flags list --status baking
fort flags list --status stable
fort flags list --status rolled_back
```

Check a specific flag:

```bash
fort flags get gmail-batch-send
# Name:        gmail-batch-send
# Status:      baking
# Created:     2026-03-20T10:00:00Z
# Enabled:     2026-03-20T10:05:00Z
# Bake period: 86400000ms (24h)
# Remaining:   62100000ms (17.25h)
```

## Manual Promotion and Rollback

If you are confident in a feature, promote it early:

```bash
fort flags promote gmail-batch-send
# gmail-batch-send promoted to stable
```

If something goes wrong, roll back immediately:

```bash
fort flags rollback gmail-batch-send --reason "Sending duplicate drafts"
# gmail-batch-send rolled back
```

The `--reason` is stored in the flag record for future reference.

## Using Flags in Code

Check a flag's state programmatically:

```typescript
import { FlagManager } from '@fort/core';

const flags = new FlagManager(db);

if (flags.isEnabled('gmail-batch-send')) {
  await sendBatch(messages);
} else {
  for (const msg of messages) {
    await sendSingle(msg);
  }
}
```

The `isEnabled` method returns `true` only for flags in `baking` or `stable` status.

## Integration with Self-Coding

When the harness self-coding system merges a new feature, it automatically creates a feature flag for it. This means every self-built feature goes through the bake period process before being considered stable. See the [Self-Coding guide](./self-coding.md) for details.

## Events

The flag system publishes events on the ModuleBus:

- `flag:created` — A new flag was registered.
- `flag:enabled` — A flag entered baking.
- `flag:promoted` — A flag was promoted to stable.
- `flag:rolledback` — A flag was rolled back.
- `flag:check` — Health checks ran with results.

## Storage

Flags are stored in the Fort SQLite database alongside memory and tool data. Each flag record includes creation time, enable time, bake period, status, promotion/rollback timestamps, and the rollback reason if applicable.
