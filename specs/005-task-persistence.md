# SPEC-005: Task Graph Persistence

**Status:** Draft  
**Author:** Lewis + Marty  
**Date:** 2026-03-25  
**Priority:** HIGH — architectural risk (Marty review finding #3)  
**Depends on:** Spec 001, Spec 003

---

## Problem

`TaskGraph` stores all tasks in a `Map<string, Task>` in memory. Every server restart loses all task history — completed work, failed tasks, in-progress items — everything. This breaks the core design principle: *"tasks are the atomic unit of transparency."* If tasks don't survive restarts, the system has no durable audit trail.

## Goals

1. All tasks persist to SQLite
2. Tasks survive server restarts
3. Task history is queryable (filter by status, agent, date range)
4. Existing TaskGraph API unchanged (no breaking changes)
5. Performance: reads from in-memory cache, writes to DB asynchronously

## Non-Goals (v1)

- Task archiving / expiry (future)
- Cross-node task sync
- Full-text search on task content

---

## Design

### Storage Layer

Add task persistence to the same SQLite DB used by ThreadManager (to avoid multiple DB files).

```sql
CREATE TABLE IF NOT EXISTS tasks (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  description TEXT,
  status TEXT NOT NULL DEFAULT 'pending',
  source TEXT,
  assigned_agent TEXT,
  parent_id TEXT,
  result TEXT,
  inability_reason TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  completed_at TEXT,
  metadata TEXT   -- JSON blob for future extensibility
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_agent ON tasks(assigned_agent);
CREATE INDEX IF NOT EXISTS idx_tasks_created ON tasks(created_at);
CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_id);
```

### TaskGraph Changes

**Constructor:** Accept optional `db: Database` parameter. If provided, persistence is enabled.

**`createTask()`:** Write to DB synchronously (tasks must not be lost even if the process crashes immediately after creation).

**`updateStatus()`:** Write to DB synchronously (status changes are the primary audit trail).

**`getTasks()` / `getTask()`:** Read from in-memory cache (no DB round-trip for hot paths).

**`loadFromDb()`:** Called on startup — hydrates the in-memory Map from DB. Tasks in `in_progress` state at load time are reset to `pending` (they were orphaned by a restart).

**New method `queryTasks()`:**
```typescript
interface TaskQuery {
  status?: TaskStatus | TaskStatus[];
  assignedAgent?: string;
  since?: Date;
  limit?: number;
  offset?: number;
}
queryTasks(query: TaskQuery): Task[]
```
Queries the DB directly (not the cache) for historical lookups.

### Integration

Wire up in `Fort.ts`:
```typescript
// Fort constructor
this.taskGraph = new TaskGraph(bus, db);  // pass the shared DB instance
await this.taskGraph.loadFromDb();
```

### Dashboard: Task History

Add a **Tasks** tab to the dashboard showing:
- All tasks (paginated, 50/page)
- Filters: status, agent, date range
- Each row: title, agent, status, duration, created_at
- Click to expand: description, result, inability reason, subtasks

New WS handler `tasks.query` accepting `TaskQuery`.

---

## Migration

On first boot with this feature:
1. In-memory tasks from current session are already in the Map
2. Going forward, all new tasks write to DB
3. No historical data to migrate (none was persisted before)

---

## Performance Notes

- SQLite with WAL mode handles ~50k inserts/sec — not a bottleneck
- In-memory cache means `getTask()` / `getTasks()` remain O(1)
- `queryTasks()` goes to DB and is paginated — fine for dashboard use

---

## Test Criteria

- `createTask()` writes to DB
- `updateStatus()` writes to DB  
- Tasks survive TaskGraph restart (hydrate from DB)
- `in_progress` tasks reset to `pending` on reload
- `queryTasks()` returns correct results for status/agent/date filters
- No regression on existing 480 tests
- Performance: 1000 task creates in < 100ms

---

## Rollback

If DB causes issues:
1. Pass `undefined` as db parameter → falls back to pure in-memory (existing behavior)
2. Feature flag: `FORT_TASK_PERSISTENCE=false` disables DB writes
