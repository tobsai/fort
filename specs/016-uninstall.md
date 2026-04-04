# SPEC-016: Uninstall Command

**Status:** Implemented  
**Author:** Toby  
**Date:** 2026-04-04  
**Priority:** MEDIUM — Users need a clean way to fully remove Fort  
**Depends on:** SPEC-001 (CLI)

---

## Problem

Fort has `fort reset` for wiping data, but no way to fully uninstall itself — remove the global CLI symlink, npm packages, and optionally all user data. Users must manually figure out `npm unlink`, find the right directories, and clean up.

## Goals

1. `fort uninstall` — interactive command that removes Fort from the system
2. Removes the npm global symlink (`fort` binary)
3. Optionally removes `~/.fort/` (all data, agents, config, API keys)
4. Stops any running Fort services before removal
5. Dry-run by default (requires `--yes` to actually delete, matching `reset` pattern)

## Approach

Create `packages/cli/src/commands/uninstall.ts` with a single command:

```
fort uninstall          # Show what would be removed (dry run)
fort uninstall --yes    # Actually uninstall
fort uninstall --keep-data --yes  # Remove CLI but keep ~/.fort/
```

### Steps performed:
1. Stop running services (ports 4001, 4077)
2. Show what will be removed:
   - npm global link (`fort` binary)
   - `~/.fort/` directory (unless `--keep-data`)
3. If `--yes`: execute removal
4. Print post-uninstall message

### What gets removed:
- **Always**: npm global symlink via `npm unlink -g @fort/cli` (or package name)
- **Unless `--keep-data`**: `~/.fort/` directory (data, agents, .env, databases)

### What does NOT get removed:
- The source repository itself (user's local clone)
- Any Docker images/containers (user's responsibility)

## Affected Files

- `packages/cli/src/commands/uninstall.ts` (new)
- `packages/cli/src/index.ts` (register command)

## Test Criteria

- `fort uninstall` without `--yes` shows dry-run output, removes nothing
- `fort uninstall --yes` stops services, unlinks CLI, removes `~/.fort/`
- `fort uninstall --keep-data --yes` unlinks CLI but preserves `~/.fort/`

## Rollback Plan

Revert the two files. User can re-install with `npm link --workspace=packages/cli` and `fort init`.
