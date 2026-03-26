/**
 * Fort health checks for SPEC-013.
 *
 * Each check implements DiagnosticsProvider and can be registered
 * with FortDoctor via fortDoctor.register().
 */

import { statSync } from 'node:fs';
import Database from 'better-sqlite3';
import type { DiagnosticsProvider } from './index.js';
import type { DiagnosticResult } from '../types.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { SchedulerStore } from '../scheduler/store.js';

// ─── DBHealthCheck ────────────────────────────────────────────────────────────

export class DBHealthCheck implements DiagnosticsProvider {
  private callCount = 0;

  constructor(
    private db: InstanceType<typeof Database>,
    private dbPath: string,
  ) {}

  diagnose(): DiagnosticResult {
    this.callCount++;
    const checks: DiagnosticResult['checks'] = [];

    // WAL mode check
    let walMode = 'unknown';
    try {
      const row = (this.db as any).pragma('journal_mode', { simple: true });
      walMode = String(row);
      checks.push({
        name: 'WAL mode',
        passed: walMode === 'wal',
        message: walMode === 'wal' ? 'WAL mode enabled' : `Journal mode: ${walMode}`,
      });
    } catch (err) {
      checks.push({ name: 'WAL mode', passed: false, message: `Failed: ${err}` });
    }

    // File size check
    let sizeMb = 0;
    let walMb = 0;
    try {
      const stat = statSync(this.dbPath);
      sizeMb = Math.round((stat.size / 1024 / 1024) * 100) / 100;
      try {
        const walStat = statSync(this.dbPath + '-wal');
        walMb = Math.round((walStat.size / 1024 / 1024) * 100) / 100;
      } catch {
        // WAL file may not exist yet — fine
      }
      checks.push({
        name: 'DB file size',
        passed: true,
        message: `DB: ${sizeMb} MB, WAL: ${walMb} MB`,
        details: { size_mb: sizeMb, wal_mb: walMb },
      });
    } catch (err) {
      checks.push({ name: 'DB file size', passed: false, message: `stat failed: ${err}` });
    }

    // Integrity check — sampled 1/10 calls
    if (this.callCount % 10 === 1) {
      try {
        const result = (this.db as any).pragma('integrity_check', { simple: true });
        const ok = result === 'ok';
        checks.push({
          name: 'Integrity check',
          passed: ok,
          message: ok ? 'Integrity OK' : `Integrity issue: ${result}`,
        });
      } catch (err) {
        checks.push({ name: 'Integrity check', passed: false, message: `Failed: ${err}` });
      }
    }

    const allPassed = checks.every((c) => c.passed);
    return {
      module: 'database',
      status: allPassed ? 'healthy' : 'degraded',
      checks,
    };
  }
}

// ─── StallDetector ────────────────────────────────────────────────────────────

const STALL_THRESHOLD_MS = 30 * 60 * 1000; // 30 minutes

export class StallDetector implements DiagnosticsProvider {
  constructor(private taskGraph: TaskGraph) {}

  diagnose(): DiagnosticResult {
    const stalled = this.taskGraph.getStaleTasks(STALL_THRESHOLD_MS);

    if (stalled.length === 0) {
      return {
        module: 'stall_detector',
        status: 'healthy',
        checks: [{ name: 'Stalled tasks', passed: true, message: 'No stalled tasks' }],
      };
    }

    return {
      module: 'stall_detector',
      status: 'degraded',
      checks: stalled.map((t) => {
        const ageMs = Date.now() - t.updatedAt.getTime();
        const ageMins = Math.round(ageMs / 60_000);
        return {
          name: `Task: ${t.shortId || t.id}`,
          passed: false,
          message: `Task '${t.title}' stalled ${ageMins}m (status: ${t.status})`,
          details: { taskId: t.id, ageMins },
        };
      }),
    };
  }
}

// ─── SchedulerHealthCheck ─────────────────────────────────────────────────────

function parseIntervalMs(value: string): number | null {
  const m = value.match(/^(\d+)(m|h|d)$/);
  if (!m) return null;
  const n = parseInt(m[1], 10);
  switch (m[2]) {
    case 'm': return n * 60_000;
    case 'h': return n * 3_600_000;
    case 'd': return n * 86_400_000;
    default: return null;
  }
}

export class SchedulerHealthCheck implements DiagnosticsProvider {
  constructor(private store: SchedulerStore) {}

  diagnose(): DiagnosticResult {
    const schedules = this.store.listSchedules();
    const enabled = schedules.filter((s) => s.enabled);
    const overdue: string[] = [];
    const now = Date.now();

    for (const s of enabled) {
      if (s.scheduleType !== 'interval') continue;
      const intervalMs = parseIntervalMs(s.scheduleValue);
      if (!intervalMs) continue;

      const lastRun = s.lastRunAt?.getTime() ?? s.createdAt.getTime();
      const elapsed = now - lastRun;
      if (elapsed > 2 * intervalMs) {
        const ageMins = Math.round(elapsed / 60_000);
        overdue.push(`'${s.name}' overdue by ${ageMins}m`);
      }
    }

    if (overdue.length === 0) {
      return {
        module: 'scheduler',
        status: 'healthy',
        checks: [{
          name: 'Schedules',
          passed: true,
          message: `${enabled.length} enabled, 0 overdue`,
          details: { total: schedules.length, enabled: enabled.length, overdue: 0 },
        }],
      };
    }

    return {
      module: 'scheduler',
      status: 'degraded',
      checks: [{
        name: 'Overdue schedules',
        passed: false,
        message: overdue.join('; '),
        details: { total: schedules.length, enabled: enabled.length, overdue: overdue.length },
      }],
    };
  }
}

// ─── MemoryHealthCheck ────────────────────────────────────────────────────────

const HEAP_WARN_BYTES = 500 * 1024 * 1024; // 500 MB

export class MemoryHealthCheck implements DiagnosticsProvider {
  diagnose(): DiagnosticResult {
    const { heapUsed, heapTotal, rss } = process.memoryUsage();
    const heapMb = Math.round(heapUsed / 1024 / 1024);
    const rssMb = Math.round(rss / 1024 / 1024);
    const degraded = heapUsed > HEAP_WARN_BYTES;

    return {
      module: 'memory',
      status: degraded ? 'degraded' : 'healthy',
      checks: [{
        name: 'Heap usage',
        passed: !degraded,
        message: degraded
          ? `Heap ${heapMb} MB exceeds 500 MB threshold`
          : `Heap ${heapMb} MB (${Math.round((heapUsed / heapTotal) * 100)}% of total)`,
        details: { heap_mb: heapMb, rss_mb: rssMb },
      }],
    };
  }
}
