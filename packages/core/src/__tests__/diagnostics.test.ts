/**
 * Diagnostics tests — SPEC-013
 *
 * Covers: DBHealthCheck, StallDetector, SchedulerHealthCheck,
 *         ErrorLog, and /health HTTP endpoint status codes.
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Database from 'better-sqlite3';
import { mkdtempSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { ErrorLog } from '../diagnostics/error-log.js';
import { DBHealthCheck, StallDetector, SchedulerHealthCheck, MemoryHealthCheck } from '../diagnostics/checks.js';
import { TaskStore } from '../task-graph/task-store.js';
import { TaskGraph } from '../task-graph/index.js';
import { ModuleBus } from '../module-bus/index.js';
import { SchedulerStore } from '../scheduler/store.js';

function makeDb(): InstanceType<typeof Database> {
  const db = new (Database as any)(':memory:') as InstanceType<typeof Database>;
  (db as any).pragma('journal_mode = WAL');
  return db;
}

// ─── ErrorLog ─────────────────────────────────────────────────────────────────

describe('ErrorLog', () => {
  it('logError stores entry in ring buffer', () => {
    const log = new ErrorLog();
    log.logError('test-sub', new Error('oops'));
    const recent = log.getRecent();
    expect(recent).toHaveLength(1);
    expect(recent[0].subsystem).toBe('test-sub');
    expect(recent[0].message).toBe('oops');
  });

  it('getRecent returns newest-first order', () => {
    const log = new ErrorLog();
    log.logError('a', new Error('first'));
    log.logError('b', new Error('second'));
    log.logError('c', new Error('third'));
    const recent = log.getRecent();
    expect(recent[0].message).toBe('third');
    expect(recent[2].message).toBe('first');
  });

  it('ring buffer caps at 100 entries', () => {
    const log = new ErrorLog();
    for (let i = 0; i < 110; i++) {
      log.logError('sub', new Error(`err ${i}`));
    }
    expect(log.getRecent(200)).toHaveLength(100);
    // Newest entry is the last logged (109)
    expect(log.getRecent(1)[0].message).toBe('err 109');
    // Oldest entry should be 10 (entries 0-9 were evicted)
    const all = log.getRecent(100);
    expect(all[99].message).toBe('err 10');
  });

  it('getBySubsystem filters correctly', () => {
    const log = new ErrorLog();
    log.logError('alpha', new Error('a1'));
    log.logError('beta', new Error('b1'));
    log.logError('alpha', new Error('a2'));
    const alpha = log.getBySubsystem('alpha');
    expect(alpha).toHaveLength(2);
    expect(alpha.every((e) => e.subsystem === 'alpha')).toBe(true);
  });

  it('persists to DB and loads back', () => {
    const db = makeDb();
    const log = new ErrorLog(db);
    log.logError('persist-test', new Error('saved'), { key: 'val' });

    // Create a fresh log pointing at the same DB
    const log2 = new ErrorLog(db);
    log2.loadFromDb();
    const recent = log2.getRecent();
    expect(recent).toHaveLength(1);
    expect(recent[0].subsystem).toBe('persist-test');
    expect(recent[0].context).toEqual({ key: 'val' });
  });

  it('limit parameter is respected', () => {
    const log = new ErrorLog();
    for (let i = 0; i < 20; i++) {
      log.logError('x', new Error(`e${i}`));
    }
    expect(log.getRecent(5)).toHaveLength(5);
  });
});

// ─── DBHealthCheck ────────────────────────────────────────────────────────────

describe('DBHealthCheck', () => {
  let tmpDir: string;
  let tmpDbPath: string;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-'));
    tmpDbPath = join(tmpDir, 'test.db');
  });

  afterEach(() => {
    try { rmSync(tmpDir, { recursive: true, force: true }); } catch { /* ignore */ }
  });

  it('returns healthy on a valid WAL-mode DB', () => {
    const db = new (Database as any)(tmpDbPath) as InstanceType<typeof Database>;
    (db as any).pragma('journal_mode = WAL');
    const check = new DBHealthCheck(db, tmpDbPath);
    const result = check.diagnose();
    expect(result.module).toBe('database');
    const walCheck = result.checks.find((c) => c.name === 'WAL mode');
    expect(walCheck).toBeTruthy();
    expect(walCheck!.passed).toBe(true);
    db.close();
  });

  it('returns degraded when WAL mode is off', () => {
    const db = new (Database as any)(tmpDbPath) as InstanceType<typeof Database>;
    // Do NOT enable WAL — default 'delete' journal mode
    const check = new DBHealthCheck(db, tmpDbPath);
    const result = check.diagnose();
    const walCheck = result.checks.find((c) => c.name === 'WAL mode');
    expect(walCheck!.passed).toBe(false);
    expect(result.status).toBe('degraded');
    db.close();
  });
});

// ─── StallDetector ────────────────────────────────────────────────────────────

describe('StallDetector', () => {
  it('returns healthy when no tasks are in progress', () => {
    const bus = new ModuleBus();
    const taskGraph = new TaskGraph(bus);
    const detector = new StallDetector(taskGraph);
    expect(detector.diagnose().status).toBe('healthy');
  });

  it('returns degraded when a task is stalled > 30min', () => {
    const bus = new ModuleBus();
    const taskGraph = new TaskGraph(bus);
    const detector = new StallDetector(taskGraph);

    // Inject a stale task directly into the internal map
    const staleTask = {
      id: 'stale-1',
      shortId: 'S1',
      title: 'Stale task',
      description: '',
      status: 'in_progress' as const,
      source: 'test' as const,
      assignedAgent: null,
      assignedTo: null,
      parentId: null,
      result: null,
      subtaskIds: [],
      threadId: null,
      metadata: {},
      createdAt: new Date(Date.now() - 35 * 60_000),
      updatedAt: new Date(Date.now() - 35 * 60_000), // 35 min ago
      completedAt: null,
    };
    (taskGraph as any).tasks.set(staleTask.id, staleTask);

    const result = detector.diagnose();
    expect(result.status).toBe('degraded');
    expect(result.checks[0].passed).toBe(false);
    expect(result.checks[0].message).toContain('stalled');
  });

  it('does not flag tasks stalled < 30min', () => {
    const bus = new ModuleBus();
    const taskGraph = new TaskGraph(bus);
    const detector = new StallDetector(taskGraph);

    const recentTask = {
      id: 'recent-1',
      shortId: 'R1',
      title: 'Recent task',
      description: '',
      status: 'in_progress' as const,
      source: 'test' as const,
      assignedAgent: null,
      assignedTo: null,
      parentId: null,
      result: null,
      subtaskIds: [],
      threadId: null,
      metadata: {},
      createdAt: new Date(Date.now() - 5 * 60_000),
      updatedAt: new Date(Date.now() - 5 * 60_000), // 5 min ago
      completedAt: null,
    };
    (taskGraph as any).tasks.set(recentTask.id, recentTask);

    expect(detector.diagnose().status).toBe('healthy');
  });
});

// ─── SchedulerHealthCheck ─────────────────────────────────────────────────────

describe('SchedulerHealthCheck', () => {
  let db: InstanceType<typeof Database>;
  let store: SchedulerStore;

  beforeEach(() => {
    db = makeDb();
    store = new SchedulerStore(db);
    store.initSchema();
  });

  it('returns healthy when no schedules exist', () => {
    const check = new SchedulerHealthCheck(store);
    expect(check.diagnose().status).toBe('healthy');
  });

  it('returns healthy for a schedule that ran on time', () => {
    const s = store.createSchedule({
      name: 'On-time',
      agentId: 'a1',
      scheduleType: 'interval',
      scheduleValue: '1h',
      taskTitle: 'Check',
    });
    // Update run info to 30min ago (within 2x interval of 60min)
    store.updateRunInfo(s.id, new Date(Date.now() - 30 * 60_000), new Date());

    const check = new SchedulerHealthCheck(store);
    expect(check.diagnose().status).toBe('healthy');
  });

  it('returns degraded when an interval schedule is overdue', () => {
    const s = store.createSchedule({
      name: 'Overdue',
      agentId: 'a1',
      scheduleType: 'interval',
      scheduleValue: '30m',
      taskTitle: 'Check',
    });
    // Simulate last run 3 hours ago (well past 2x 30min threshold)
    store.updateRunInfo(s.id, new Date(Date.now() - 3 * 60 * 60_000), new Date());

    const check = new SchedulerHealthCheck(store);
    const result = check.diagnose();
    expect(result.status).toBe('degraded');
    expect(result.checks[0].message).toContain('Overdue');
  });

  it('ignores disabled schedules', () => {
    const s = store.createSchedule({
      name: 'Paused',
      agentId: 'a1',
      scheduleType: 'interval',
      scheduleValue: '30m',
      taskTitle: 'Check',
    });
    store.updateRunInfo(s.id, new Date(Date.now() - 3 * 60 * 60_000), new Date());
    store.setEnabled(s.id, false);

    const check = new SchedulerHealthCheck(store);
    expect(check.diagnose().status).toBe('healthy');
  });
});

// ─── MemoryHealthCheck ────────────────────────────────────────────────────────

describe('MemoryHealthCheck', () => {
  it('returns healthy when heap is normal', () => {
    const check = new MemoryHealthCheck();
    const result = check.diagnose();
    // Under normal test conditions heap will be well under 500 MB
    expect(result.module).toBe('memory');
    expect(result.status).toBe('healthy');
    expect(result.checks[0].passed).toBe(true);
  });
});
