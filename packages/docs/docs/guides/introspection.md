---
sidebar_position: 15
title: Introspection
description: Machine-readable system documentation and diagnostics
---

# Introspection

Fort is designed to be fully inspectable. The introspection system provides machine-readable documentation of every module, capability, and event in the system.

## System Profile

Get a high-level overview:

```bash
fort introspect
# Fort v0.1.0
# ──────────────────────────
# Uptime:        4d 12h 33m
# Modules:       14 loaded, 0 degraded
# Agents:        4 core + 2 specialist
# Tasks:         1,247 total (12 active)
# Memory:        312 entities, 1,204 relationships
# Tools:         38 registered
# Flags:         3 stable, 1 baking
# Plugins:       2 loaded
```

## Modules

List all loaded modules with their health status:

```bash
fort introspect modules
# Module           Status    Health    Last Check
# ──────────────────────────────────────────────────
# ModuleBus        loaded    healthy   2s ago
# TaskGraph        loaded    healthy   2s ago
# MemoryManager    loaded    healthy   5s ago
# AgentRegistry    loaded    healthy   2s ago
# ToolRegistry     loaded    healthy   2s ago
# PermissionMgr    loaded    healthy   2s ago
# Scheduler        loaded    healthy   3s ago
# FlagManager      loaded    healthy   2s ago
# PluginManager    loaded    healthy   4s ago
# Gmail            loaded    healthy   10s ago
# Calendar         loaded    healthy   10s ago
# iMessage         loaded    degraded  15s ago
# BraveSearch      loaded    healthy   8s ago
# Browser          loaded    healthy   12s ago
```

Every module exposes a `diagnose()` method that returns structured health information. A module reports `healthy`, `degraded`, or `unhealthy`.

Get details on a specific module:

```bash
fort introspect modules MemoryManager
# Module: MemoryManager
# Status: loaded
# Health: healthy
# Backend: sqlite (.fort/data/memory.db)
# MemU sidecar: not connected (using SQLite fallback)
# Entities: 312
# Relationships: 1,204
# Last write: 2026-03-21T14:22:10Z
# Events published: memory:entity:created, memory:entity:updated, memory:query
# Events subscribed: task:completed, agent:observation
```

## Capabilities

Get a flat list of everything Fort can do:

```bash
fort introspect capabilities
# file:read
# file:write
# email:list
# email:read
# email:draft
# email:send
# calendar:list
# calendar:create
# calendar:update
# memory:query
# memory:store
# task:create
# task:update
# tool:register
# tool:search
# shell:execute
# browser:navigate
# browser:extract
# search:web
# ...
```

Each capability maps to one or more tools in the tool registry and has an associated permission tier.

## Event Catalog

See all events flowing through the ModuleBus:

```bash
fort introspect events
# Event                      Publishers         Subscribers
# ──────────────────────────────────────────────────────────
# task:created               TaskGraph          Orchestrator, Scheduler
# task:completed             TaskGraph          Memory, Reflection
# memory:entity:created      MemoryManager      Reflection
# agent:observation          AgentRegistry      MemoryManager
# permission:check           PermissionMgr      TaskGraph
# permission:denied          PermissionMgr      TaskGraph, Orchestrator
# flag:enabled               FlagManager        Scheduler
# flag:rolledback            FlagManager        Orchestrator
# plugin:loaded              PluginManager      AgentRegistry
# email:received             Gmail              Orchestrator
# calendar:event:created     Calendar           Scheduler
# harness:merged             Harness            FlagManager
# ...
```

## Search

Find modules, capabilities, events, and tools matching a query:

```bash
fort introspect search "email"
# Modules:
#   Gmail — Email integration via Google Gmail API
#
# Capabilities:
#   email:list, email:read, email:draft, email:send
#
# Events:
#   email:received, email:drafted, email:sent
#
# Tools:
#   gmail-list-messages, gmail-read-message, gmail-draft, gmail-send
```

## Export

Generate a full system documentation export:

```bash
fort introspect export --format markdown
# Exported to .fort/exports/introspection-20260321.md
```

Supported formats:

| Format | Description |
|--------|-------------|
| `markdown` | Human-readable Markdown document |
| `json` | Machine-readable JSON for tooling |
| `yaml` | YAML format for config-style consumption |

The export includes every module, capability, event, tool, agent, and their relationships. This is useful for feeding Fort's own documentation back into its context.

```bash
fort introspect export --format json | jq '.modules | length'
# 14
```

## Programmatic Access

From TypeScript, use the introspection API directly:

```typescript
import { Introspector } from '@fort/core';

const intro = new Introspector(bus, registry);

const profile = intro.getSystemProfile();
const modules = intro.listModules();
const events = intro.getEventCatalog();
const results = intro.search('email');
```

All introspection data is read-only and reflects the live state of the system.
