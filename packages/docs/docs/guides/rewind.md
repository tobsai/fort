---
sidebar_position: 14
title: Rewind
description: Snapshot-based backup and restore for Fort state
---

# Rewind

Rewind is Fort's snapshot-based backup system. It captures the state of your memory database, configuration, and agent identities so you can restore to any previous point.

## What Gets Captured

Each snapshot includes:

- **Memory DB** — The full SQLite memory graph (entities, relationships, observations).
- **Config** — All YAML and JSON configuration files from `.fort/`.
- **Agent identities** — Agent registry state, personality definitions, and capability mappings.

Snapshots are stored as timestamped archives in `.fort/data/snapshots/`.

## Creating Snapshots

### Manual Snapshots

Create a snapshot with a descriptive label:

```bash
fort rewind --create "before migration"
# Snapshot created: snap-20260321-143022
# Label: before migration
# Size: 2.4 MB
```

### Automatic Snapshots

Fort automatically creates snapshots when it detects meaningful changes. Change detection uses SHA-256 hashing — Fort computes a hash of the memory DB, config files, and agent state after each operation. If the hash differs from the last snapshot, a new one is created.

Auto-snapshots are labeled with the operation that triggered them:

```
snap-20260321-090000  auto: harness merge h-a1b2c3d4
snap-20260320-163000  auto: memory graph update (47 entities)
snap-20260320-100000  auto: config change permissions.yaml
```

## Listing Snapshots

```bash
fort rewind --limit 10
# ID                    Label                              Date                 Size
# snap-20260321-143022  before migration                   2026-03-21 14:30:22  2.4 MB
# snap-20260321-090000  auto: harness merge h-a1b2c3d4     2026-03-21 09:00:00  2.3 MB
# snap-20260320-163000  auto: memory graph update          2026-03-20 16:30:00  2.3 MB
# snap-20260320-100000  auto: config change                2026-03-20 10:00:00  2.1 MB
# ...
```

Without `--limit`, Fort shows the 20 most recent snapshots.

## Previewing a Restore

Before restoring, preview what would change:

```bash
fort rewind --preview snap-20260320-163000
# Restore preview for snap-20260320-163000
# ─────────────────────────────────────────
# Memory DB:
#   Current entities: 312
#   Snapshot entities: 289
#   Difference: -23 entities would be removed
#
# Config:
#   Modified: permissions.yaml (3 lines changed)
#   Added: integrations/brave.yaml (new file in current, would be removed)
#
# Agents:
#   No changes
```

This is a read-only operation. Nothing is modified.

## Restoring

Full restore:

```bash
fort rewind snap-20260320-163000
# Restoring from snap-20260320-163000...
# [1/3] Memory DB restored (289 entities)
# [2/3] Config restored (4 files)
# [3/3] Agent identities restored
# Done. Pre-restore snapshot created: snap-20260321-143500
```

Fort automatically creates a snapshot of the current state before restoring, so you can always undo a restore.

### Partial Restore

Restore only configuration:

```bash
fort rewind snap-20260320-163000 --config-only
# Restoring config from snap-20260320-163000...
# Config restored (4 files)
```

Restore only the memory database:

```bash
fort rewind snap-20260320-163000 --memory-only
# Restoring memory DB from snap-20260320-163000...
# Memory DB restored (289 entities)
```

Partial restores also create a pre-restore snapshot.

## Snapshot Details

Inspect a specific snapshot:

```bash
fort rewind --info snap-20260320-163000
# ID:       snap-20260320-163000
# Label:    auto: memory graph update
# Created:  2026-03-20T16:30:00Z
# Size:     2.3 MB
# Hash:     sha256:a1b2c3d4e5f6...
# Contents:
#   memory.db    2.1 MB  (289 entities, 1204 relationships)
#   config/      48 KB   (7 files)
#   agents/      12 KB   (4 agents)
```

## Cleanup

Remove old snapshots:

```bash
fort rewind --prune --older-than 30d
# Pruned 12 snapshots older than 30 days
# Freed 28.4 MB
```

Keep at least N snapshots regardless of age:

```bash
fort rewind --prune --older-than 14d --keep-min 10
```

## Events

The rewind system publishes on the ModuleBus:

- `rewind:snapshot:created` — A new snapshot was taken (manual or auto).
- `rewind:restore:started` — A restore operation began.
- `rewind:restore:completed` — A restore finished successfully.
- `rewind:pruned` — Old snapshots were removed.
