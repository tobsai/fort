# SPEC-012: Agent Memory Persistence

**Status:** Draft  
**Author:** Lewis  
**Date:** 2026-03-25  
**Priority:** HIGH — agents need context across sessions  
**Depends on:** Spec 004 (Agent Management), Spec 003 (Conversation Persistence)

---

## Problem

Agents currently have no persistent memory across sessions. When a conversation thread ends or Fort restarts, everything the agent learned is gone. An agent helping with a long-running project should accumulate knowledge — decisions made, things tried, preferences learned — and surface that context automatically in new conversations.

## Goals

1. Each agent has a personal memory store (key-value + semantic search)
2. Agents can write to memory during task execution via a `remember` tool
3. Agents can read from memory via a `recall` tool (keyword + recency)
4. Memory persists to SQLite, survives Fort restart
5. Dashboard shows each agent's memory entries (read-only for now)
6. Memory entries are categorized: `fact`, `decision`, `preference`, `observation`

## Non-Goals (v1)

- Vector/semantic embedding search (keyword + recency only for now)
- Cross-agent shared memory
- Memory expiry/forgetting
- Memory size limits / eviction

---

## Design

### Data Model

```sql
CREATE TABLE agent_memories (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  category TEXT NOT NULL,     -- 'fact' | 'decision' | 'preference' | 'observation'
  content TEXT NOT NULL,      -- the memory text
  tags TEXT,                  -- JSON array of string tags
  task_id TEXT,               -- task context when memory was created (optional)
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX idx_memories_agent ON agent_memories(agent_id);
CREATE INDEX idx_memories_category ON agent_memories(agent_id, category);
CREATE VIRTUAL TABLE agent_memories_fts USING fts5(content, tags, agent_id UNINDEXED);
```

### MemoryStore

File: `packages/core/src/memory/agent-memory-store.ts`

```typescript
export class AgentMemoryStore {
  constructor(db: Database.Database) {}
  initSchema(): void
  remember(agentId: string, entry: MemoryEntry): AgentMemory
  recall(agentId: string, query: string, options?: { limit?: number; category?: MemoryCategory }): AgentMemory[]
  list(agentId: string, options?: { category?: MemoryCategory; limit?: number }): AgentMemory[]
  delete(id: string): void
  clear(agentId: string): void
}
```

`recall()` uses FTS5 MATCH for keyword search, falls back to LIKE if no FTS match.

### remember Tool (Tier 1)

Built-in tool available to all agents:

```typescript
{
  name: 'remember',
  tier: 1,
  description: 'Store a piece of information in your persistent memory',
  parameters: {
    content: string,           // the memory text
    category: MemoryCategory,  // 'fact' | 'decision' | 'preference' | 'observation'
    tags: string[],            // optional tags for retrieval
  }
}
```

### recall Tool (Tier 1)

```typescript
{
  name: 'recall',
  tier: 1,
  description: 'Search your persistent memory for relevant information',
  parameters: {
    query: string,             // keyword query
    category?: MemoryCategory, // optional filter
    limit?: number,            // max results (default 10)
  }
}
```

### Auto-inject memory context

When an agent starts a new task, the system automatically runs `recall(query: taskTitle, limit: 5)` and prepends the results as a system message:

```
[Memory] Relevant context from your memory:
- [decision] Decided to use PostgreSQL for all new services (2026-03-15)
- [fact] User prefers clean, minimal UIs over feature-heavy dashboards
- [observation] Railway custom domains require Cloudflare proxy OFF
```

This happens in the Orchestrator before the agent sees the task.

### Fort.ts wiring

- Shared `AgentMemoryStore` initialized with same DB
- Passed to ToolExecutor (remember/recall tools)
- Passed to Orchestrator (auto-inject on task start)

### WS Handlers

```
'agent.memories'        → { agentId, category? } → list memories
'agent.memory.delete'   → { id } → delete a memory
'agent.memory.clear'    → { agentId } → clear all memories for agent
```

### Dashboard: Agent Memory View

In the Agent detail panel (from Spec 004, AgentsPage.tsx):
- New tab "Memory" alongside existing tabs
- List of memories grouped by category
- Each memory: category badge, content, tags, date
- Delete button per entry
- "Clear All" button with confirmation

---

## Test Criteria

- AgentMemoryStore.remember() writes to DB
- AgentMemoryStore.recall() matches by keyword
- recall() respects category filter
- Auto-inject prepends memory context before task starts
- remember/recall tools available in ToolExecutor
- All existing 587 tests still pass

---

## Notes

This is the foundation for long-running agent intelligence. Version 2 will add embeddings (Ollama nomic-embed-text) for semantic recall. Version 1 is keyword-only for simplicity.
