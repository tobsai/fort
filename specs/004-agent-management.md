# SPEC-004: Agent Management UI

**Status:** Draft  
**Author:** Lewis + Marty  
**Date:** 2026-03-25  
**Depends on:** Spec 001 (Chat), Spec 002 (Tools), Spec 003 (Conversation Persistence)

---

## Problem

Agents are currently created via YAML identity files on disk (`agents/` directory). There is no way to create, edit, or manage agents from the dashboard. The "Agents" page exists but is read-only.

For Fort to be usable by non-technical users (like Damien), agents need to be creatable and configurable from the web UI.

## Goals

1. Create new specialist agents from the dashboard
2. View agent details (status, soul, capabilities, recent tasks)
3. Edit agent configuration (name, description, model, tools)
4. Start/stop agents
5. Delete agents (soft-delete, preserve history)

## Non-Goals (v1)

- Agent marketplace / templates (future)
- Multi-user agent permissions
- Agent-to-agent delegation config
- Custom tool authoring from UI (tools are code-only for now)

---

## Design

### Backend: WebSocket Handlers

Add to `packages/core/src/server/index.ts`:

```
'agents.list'    → List all registered agents with status
'agent.get'      → Get agent details by ID
'agent.create'   → Create a new specialist agent from UI input
'agent.update'   → Update agent config (name, description, model, soul)
'agent.start'    → Start a stopped agent
'agent.stop'     → Stop a running agent
'agent.delete'   → Soft-delete (mark deleted, stop, remove from routing)
```

### Agent Creation Flow

1. User fills form: name, description, model preference, tool selection, optional SOUL.md text
2. Server generates a unique ID and creates the identity object
3. SpecialistAgent is instantiated and started
4. Agent appears in agent list and is routable for chat
5. Identity is persisted to SQLite (NOT disk YAML) for portability

### Data Model Addition

Add to ThreadManager's SQLite or a new `agents` table:

```sql
CREATE TABLE agents (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  type TEXT DEFAULT 'specialist',
  model_preference TEXT,
  soul TEXT,            -- SOUL.md content
  capabilities TEXT,    -- JSON array
  tools TEXT,           -- JSON array of tool names
  event_subscriptions TEXT, -- JSON array
  memory_partition TEXT,
  status TEXT DEFAULT 'active',  -- active, stopped, deleted
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);
```

### Dashboard: AgentsPage.tsx

Replace the current read-only list with:

1. **Agent List** — Polaris-style card grid showing each agent with:
   - Name, description, status badge (running/stopped)
   - Task count (completed/in-progress/failed)
   - Last active timestamp
   - Quick actions: Start/Stop, Edit, Delete

2. **Create Agent Modal** — form with:
   - Name (required)
   - Description (required)
   - Model preference (dropdown: default, fast, powerful)
   - Available tools (checkbox list from ToolRegistry)
   - SOUL.md (textarea, optional — sets personality)

3. **Agent Detail Panel** — slide-out or page:
   - Full config (editable)
   - Recent tasks list
   - Memory snapshot (nodes created by this agent)
   - Chat shortcut (navigate to chat with this agent selected)

---

## Phases

### Phase 1: Agent Persistence
- Add `agents` table to SQLite
- Migrate existing YAML-loaded agents to DB on first boot
- Fort loads agents from DB instead of (or in addition to) disk

### Phase 2: WebSocket Handlers
- Implement all 7 WS handlers
- Wire create/update into SpecialistAgent lifecycle

### Phase 3: Dashboard UI
- Agent list with status badges
- Create agent modal
- Start/stop/delete actions

### Phase 4: Agent Detail + Edit
- Detail panel with config editing
- Recent tasks display
- SOUL.md editor

---

## Test Criteria

- Create agent via WS → agent appears in list, is routable for chat
- Update agent name → reflected in list and chat
- Stop agent → status changes, chat routing skips it
- Start stopped agent → routable again
- Delete agent → removed from list, history preserved
- DB persistence → agents survive server restart
- Existing YAML agents migrated to DB on first boot

---

## Rollback

Agent creation is additive. If the feature breaks:
1. Remove WS handlers
2. Fall back to YAML loading (existing code path)
3. Agents created via UI are orphaned but data is preserved in SQLite
