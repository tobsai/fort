# SPEC-013: Diagnostics Dashboard

**Status:** Draft  
**Author:** Lewis  
**Date:** 2026-03-25  
**Priority:** MEDIUM — operational visibility  
**Depends on:** Spec 007 (Cost Tracking), Spec 008 (Scheduled Tasks)

---

## Problem

Fort runs as a persistent service but has no production health visibility. When something breaks — an agent stalls, memory grows, DB queries slow, a scheduled task silently fails — there is no single place to see system state. The existing `/health` endpoint returns `{"status":"ok"}` regardless of actual health.

## Goals

1. Rich health endpoint: each subsystem reports its own status (Green/Yellow/Red)
2. Dashboard Diagnostics page: real-time system metrics
3. Automatic stall detection: agents/tasks stuck > threshold emit a warning
4. DB metrics: file size, WAL size, slow queries (> 100ms)
5. Memory metrics: process heap usage, trend
6. Error log: last 100 errors across all subsystems

## Non-Goals (v1)

- External alerting (PagerDuty, Slack webhook)
- Prometheus/metrics export
- Multi-node monitoring

---

## Design

### FortDoctor (enhanced)

Fort already has a `FortDoctor` class that agents self-register with. Enhance it:

```typescript
interface HealthCheck {
  name: string;
  check(): Promise<DiagnosticResult>;
}

interface DiagnosticResult {
  status: 'healthy' | 'degraded' | 'unhealthy';
  message: string;
  details?: Record<string, unknown>;
  latencyMs?: number;
}
```

New health checks to add:
- **DBHealthCheck** — check DB file size, WAL mode on, run PRAGMA integrity_check (sampled 1/10 calls)
- **SchedulerHealthCheck** — any schedules that should have run but haven't in 2x their interval
- **StallDetector** — any tasks in_progress > 30 minutes without a log entry → degraded
- **MemoryHealthCheck** — process.memoryUsage().heapUsed > 500MB → degraded

### ErrorLog

File: `packages/core/src/diagnostics/error-log.ts`

In-memory ring buffer (last 100 errors), written to DB for persistence:

```typescript
export class ErrorLog {
  logError(subsystem: string, error: Error, context?: object): void
  getRecent(limit?: number): ErrorEntry[]
  getBySubsystem(subsystem: string): ErrorEntry[]
}
```

All existing try/catch blocks in Fort call `errorLog.logError()` rather than `console.error()`.

### Enhanced /health endpoint

```json
{
  "status": "healthy" | "degraded" | "unhealthy",
  "timestamp": "2026-03-25T22:00:00Z",
  "uptime": 3600,
  "checks": {
    "database": { "status": "healthy", "details": { "size_mb": 2.1, "wal_mb": 0.1 } },
    "scheduler": { "status": "healthy", "details": { "schedules": 3, "overdue": 0 } },
    "stall_detector": { "status": "degraded", "message": "Task 'xyz' stalled 45m" },
    "memory": { "status": "healthy", "details": { "heap_mb": 142 } }
  }
}
```

### WS Handlers

```
'diagnostics.health'   → full health check result (runs all checks)
'diagnostics.errors'   → recent error log entries
'diagnostics.metrics'  → point-in-time metrics (uptime, heap, DB size, task counts)
```

### Dashboard: Diagnostics Page

File: `packages/dashboard/src/pages/DiagnosticsPage.tsx`

Layout:
1. **Status bar** at top — overall status (Green/Yellow/Red pill)
2. **Check cards** — one per registered health check, show status + key detail
3. **Metrics row** — uptime, heap MB, DB size MB, tasks today, errors today
4. **Error log table** — subsystem, message, time ago, expand for stack trace

Auto-refreshes every 30 seconds via `diagnostics.metrics` WS call.

---

## Test Criteria

- DBHealthCheck returns healthy on clean DB
- StallDetector returns degraded when task stuck > 30m
- SchedulerHealthCheck returns degraded when overdue schedule detected
- ErrorLog.logError() stores entry, getRecent() returns in reverse-chronological order
- /health endpoint returns 200 when healthy, 503 when any check is 'unhealthy'
- All existing 587 tests still pass
