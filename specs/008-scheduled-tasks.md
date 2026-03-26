# SPEC-008: Scheduled Tasks

**Status:** Draft  
**Author:** Lewis  
**Date:** 2026-03-25  
**Priority:** MEDIUM  
**Depends on:** Spec 005 (Task Persistence)

---

## Problem

Agents can only work when explicitly triggered via chat or the dashboard. There is no way to schedule recurring work (e.g., "check email every 30 minutes," "run a daily summary at 7 AM"). Without scheduling, Fort is reactive-only.

## Goals

1. Define one-off and recurring scheduled tasks for any agent
2. Cron-style scheduling (`0 7 * * *`) and interval-based (`every 30m`)
3. Schedules persist across restarts
4. Dashboard: view, create, pause, delete scheduled tasks
5. Scheduled task runs create real Task records (visible in task history from Spec 005)

## Non-Goals (v1)

- Time-zone per schedule (all times are UTC)
- Schedule dependencies (A triggers B)
- External trigger webhooks

---

## Design

### Data Model

```sql
CREATE TABLE scheduled_tasks (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  agent_id TEXT NOT NULL,
  schedule_type TEXT NOT NULL,   -- 'cron' | 'interval'
  schedule_value TEXT NOT NULL,  -- cron expr OR '30m', '1h', '6h', '1d'
  task_title TEXT NOT NULL,      -- title for created task
  task_description TEXT,         -- description for created task
  enabled INTEGER DEFAULT 1,
  last_run_at TEXT,
  next_run_at TEXT,
  run_count INTEGER DEFAULT 0,
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);
```

### Scheduler Service

File: `packages/core/src/scheduler/index.ts`

```typescript
export class Scheduler {
  constructor(store: SchedulerStore, taskGraph: TaskGraph, bus: ModuleBus) {}
  
  start(): void    // Initialize timer loop (check every minute)
  stop(): void     // Clear timers
  
  createSchedule(config: ScheduleConfig): ScheduledTask
  updateSchedule(id: string, updates: Partial<ScheduleConfig>): ScheduledTask
  deleteSchedule(id: string): void
  pauseSchedule(id: string): void
  resumeSchedule(id: string): void
  listSchedules(): ScheduledTask[]
  getSchedule(id: string): ScheduledTask | null
  
  private checkAndRunDue(): void  // Called every minute
  private runSchedule(schedule: ScheduledTask): void
  private calculateNextRun(schedule: ScheduledTask): Date
}
```

### Schedule Config

```typescript
interface ScheduleConfig {
  name: string;
  description?: string;
  agentId: string;
  scheduleType: 'cron' | 'interval';
  scheduleValue: string;  // '0 7 * * *' or '30m'
  taskTitle: string;
  taskDescription?: string;
}
```

### Interval Parsing

Simple interval parser:
- `30m` → 30 minutes
- `1h` → 60 minutes  
- `6h` → 360 minutes
- `1d` → 1440 minutes

### Cron Parsing

Use `croner` or `node-cron` library for cron expression parsing. Small, well-tested, no native deps.

### Execution Flow

1. Scheduler checks due tasks every minute
2. For each due schedule, creates a Task via TaskGraph (title from schedule config)
3. Publishes `task.created` on the bus (agent picks it up per normal flow)
4. Updates `last_run_at`, calculates and stores `next_run_at`, increments `run_count`
5. Task appears in task history with `source: 'scheduled'`

### WebSocket Handlers

```
'schedules.list'     → list all scheduled tasks with status
'schedule.create'    → create new schedule
'schedule.update'    → update schedule
'schedule.delete'    → delete schedule
'schedule.pause'     → pause (enabled=false, preserves schedule)
'schedule.resume'    → resume
'schedule.run_now'   → trigger immediately (for testing)
```

### Dashboard: Schedules Page

New **Schedules** page:
1. List of schedules — name, agent, schedule expression, last run, next run, run count
2. Status badge (active/paused)
3. Create Schedule form:
   - Name, description
   - Agent selector (from agents list)
   - Schedule type: Cron / Interval tabs
   - Expression builder (cron) or dropdown (interval: 15m/30m/1h/6h/12h/1d/1w)
   - Task title and description template
4. Actions: Pause, Resume, Run Now, Delete

---

## Test Criteria

- Create interval schedule → next_run_at calculated correctly
- Create cron schedule → next_run_at calculated correctly  
- checkAndRunDue() → due schedules create Task records
- After run → last_run_at updated, next_run_at recalculated
- Pause → schedule not triggered
- Resume → schedule triggers again
- Persist across restart → schedules loaded from DB, due runs caught up
- All existing tests still pass

---

## Rollback

Scheduler is an optional module. If not initialized in Fort.start(), no schedules run. Feature flag: `FORT_SCHEDULER_ENABLED=false`.
