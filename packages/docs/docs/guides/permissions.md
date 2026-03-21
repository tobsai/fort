---
sidebar_position: 10
title: Permissions
description: Tiered action model for controlling what Fort can do autonomously
---

# Permissions

Fort uses a four-tier permission model to control how actions are executed. Every action Fort takes is classified into a tier that determines the level of human involvement required.

## The Four Tiers

| Tier | Name | Behavior |
|------|------|----------|
| **1** | Auto | Executes immediately with no approval. Safe, read-only, or low-risk actions. |
| **2** | Draft | Creates a draft for human review before execution. |
| **3** | Approve | Requires explicit approval before Fort will execute. |
| **4** | Never | Blocked entirely. Fort will refuse to perform the action. |

### Tier 1 — Auto

Actions that carry no meaningful risk. Examples: reading files, listing tasks, querying memory, running diagnostics.

### Tier 2 — Draft

Actions where you want to review the output before it goes live. Fort prepares the action and waits for your sign-off. Examples: drafting an email, composing an iMessage, writing a file.

### Tier 3 — Approve

High-impact actions that require explicit confirmation. Fort describes what it intends to do and waits for a yes/no. Examples: sending email, submitting forms, creating calendar events, running shell commands.

### Tier 4 — Never

Permanently blocked actions. Fort will not attempt these under any circumstances. Examples: deleting databases, running destructive shell commands, accessing forbidden paths.

## Configuration

Permissions are defined in YAML at `.fort/data/permissions.yaml`:

```yaml
tiers:
  file_read: 1
  file_write: 2
  email_draft: 2
  email_send: 3
  calendar_create: 3
  shell_execute: 3
  database_delete: 4
  system_modify: 4

commands:
  allowed:
    - git status
    - git diff
    - git log
    - npm test
    - npx vitest
  denied:
    - rm -rf
    - sudo
    - mkfs
    - dd if=
```

## Folder-Level Permissions

Override tiers for specific directories:

```yaml
folder_overrides:
  /Users/me/Documents/source:
    file_write: 1    # auto-approve writes in your source dir
    shell_execute: 2  # draft shell commands here

  /Users/me/.ssh:
    file_read: 3      # require approval to read SSH keys
    file_write: 4     # never write to .ssh
```

Folder overrides take precedence over the top-level tier assignments. Fort resolves permissions by matching the most specific path first.

## System-Level Permissions

Some permissions apply globally regardless of folder context:

```yaml
system:
  network_access: 2
  browser_navigation: 3
  process_spawn: 3
  credential_access: 4
```

## CLI Usage

Check the effective permission for an action:

```bash
fort permissions check email_send
# Tier 3 — Approve (requires explicit approval)
```

List all configured permissions:

```bash
fort permissions list
```

Override a permission temporarily (current session only):

```bash
fort permissions override email_send --tier 2
# email_send downgraded to Draft for this session
```

View the resolved permission for an action in a specific folder:

```bash
fort permissions check file_write --path /Users/me/.ssh
# Tier 4 — Never (blocked by folder override)
```

## How It Works

1. An agent or integration requests an action (e.g., `email_send`).
2. The `PermissionManager` looks up the tier for that action.
3. Folder overrides are checked if a path is involved.
4. The action is routed based on tier: execute, draft, prompt, or reject.
5. Every permission check is logged to the task graph for auditability.

## Defaults

Fort ships with conservative defaults. All shell commands start at Tier 3, all network actions at Tier 2, and destructive operations at Tier 4. You can relax these as you build trust with the system.

```bash
# Reset permissions to defaults
fort permissions reset
```

The permission system publishes `permission:check` and `permission:denied` events on the ModuleBus, so other modules can react to access decisions.
