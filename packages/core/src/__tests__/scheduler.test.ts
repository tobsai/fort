import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import Database from 'better-sqlite3';
import { SchedulerStore } from '../scheduler/store.js';
import { Scheduler } from '../scheduler/index.js';
import { TaskGraph } from '../task-graph/index.js';
import { ModuleBus } from '../module-bus/index.js';

function makeDb(): InstanceType<typeof Database> {
  const db = new (Database as any)(':memory:') as InstanceType<typeof Database>;
  (db as any).pragma('journal_mode = WAL');
  return db;
}

function makeStore(db: InstanceType<typeof Database>): SchedulerStore {
  const store = new SchedulerStore(db);
  store.initSchema();
  return store;
}

function makeScheduler(store?: SchedulerStore) {
  const bus = new ModuleBus();
  const taskGraph = new TaskGraph(bus);
  const scheduler = new Scheduler(bus, taskGraph, store);
  return { bus, taskGraph, scheduler };
}

// ─── SchedulerStore ─────────────────────────────────────────────────────────

describe('SchedulerStore', () => {
  let db: InstanceType<typeof Database>;
  let store: SchedulerStore;

  beforeEach(() => {
    db = makeDb();
    store = makeStore(db);
  });

  it('createSchedule persists to DB', () => {
    const s = store.createSchedule({
      name: 'Email check',
      agentId: 'agent-1',
      scheduleType: 'interval',
      scheduleValue: '30m',
      taskTitle: 'Check email',
    });
    expect(s.id).toBeTruthy();
    expect(s.name).toBe('Email check');
    expect(s.enabled).toBe(true);
    expect(s.runCount).toBe(0);

    const all = store.listSchedules();
    expect(all).toHaveLength(1);
    expect(all[0].id).toBe(s.id);
  });

  it('getSchedule returns null for unknown id', () => {
    expect(store.getSchedule('no-such-id')).toBeNull();
  });

  it('updateSchedule changes fields', () => {
    const s = store.createSchedule({
      name: 'Old name',
      agentId: 'agent-1',
      scheduleType: 'interval',
      scheduleValue: '1h',
      taskTitle: 'Do stuff',
    });
    const updated = store.updateSchedule(s.id, { name: 'New name' });
    expect(updated.name).toBe('New name');
    expect(updated.scheduleValue).toBe('1h'); // unchanged
  });

  it('deleteSchedule removes from DB', () => {
    const s = store.createSchedule({
      name: 'Temp',
      agentId: 'agent-1',
      scheduleType: 'interval',
      scheduleValue: '1d',
      taskTitle: 'Temp task',
    });
    store.deleteSchedule(s.id);
    expect(store.listSchedules()).toHaveLength(0);
  });

  it('setEnabled toggles enabled flag', () => {
    const s = store.createSchedule({
      name: 'Toggle me',
      agentId: 'agent-1',
      scheduleType: 'interval',
      scheduleValue: '6h',
      taskTitle: 'Toggle task',
    });
    store.setEnabled(s.id, false);
    expect(store.getSchedule(s.id)!.enabled).toBe(false);
    store.setEnabled(s.id, true);
    expect(store.getSchedule(s.id)!.enabled).toBe(true);
  });

  it('updateRunInfo increments run_count and sets timestamps', () => {
    const s = store.createSchedule({
      name: 'Run counter',
      agentId: 'agent-1',
      scheduleType: 'interval',
      scheduleValue: '30m',
      taskTitle: 'Count me',
    });
    const lastRun = new Date('2026-03-25T10:00:00Z');
    const nextRun = new Date('2026-03-25T10:30:00Z');
    store.updateRunInfo(s.id, lastRun, nextRun);

    const loaded = store.getSchedule(s.id)!;
    expect(loaded.runCount).toBe(1);
    expect(loaded.lastRunAt?.toISOString()).toBe(lastRun.toISOString());
    expect(loaded.nextRunAt?.toISOString()).toBe(nextRun.toISOString());
  });
});

// ─── Scheduler: interval parsing ────────────────────────────────────────────

describe('Scheduler.calculateNextRun — interval parsing', () => {
  it('30m → 30 minutes from now', () => {
    const db = makeDb();
    const store = makeStore(db);
    const { scheduler } = makeScheduler(store);
    const before = Date.now();
    const schedule = store.createSchedule({
      name: 'Thirty',
      agentId: 'a',
      scheduleType: 'interval',
      scheduleValue: '30m',
      taskTitle: 't',
    });
    const next = scheduler.calculateNextRun(schedule);
    const diff = next.getTime() - before;
    expect(diff).toBeGreaterThanOrEqual(29 * 60_000);
    expect(diff).toBeLessThanOrEqual(31 * 60_000);
  });

  it('1h → 60 minutes from now', () => {
    const db = makeDb();
    const store = makeStore(db);
    const { scheduler } = makeScheduler(store);
    const before = Date.now();
    const schedule = store.createSchedule({
      name: 'Hour',
      agentId: 'a',
      scheduleType: 'interval',
      scheduleValue: '1h',
      taskTitle: 't',
    });
    const next = scheduler.calculateNextRun(schedule);
    const diff = next.getTime() - before;
    expect(diff).toBeGreaterThanOrEqual(59 * 60_000);
    expect(diff).toBeLessThanOrEqual(61 * 60_000);
  });

  it('6h → 360 minutes from now', () => {
    const db = makeDb();
    const store = makeStore(db);
    const { scheduler } = makeScheduler(store);
    const before = Date.now();
    const schedule = store.createSchedule({
      name: 'Six hours',
      agentId: 'a',
      scheduleType: 'interval',
      scheduleValue: '6h',
      taskTitle: 't',
    });
    const next = scheduler.calculateNextRun(schedule);
    const diff = next.getTime() - before;
    expect(diff).toBeGreaterThanOrEqual(359 * 60_000);
    expect(diff).toBeLessThanOrEqual(361 * 60_000);
  });

  it('1d → 1440 minutes from now', () => {
    const db = makeDb();
    const store = makeStore(db);
    const { scheduler } = makeScheduler(store);
    const before = Date.now();
    const schedule = store.createSchedule({
      name: 'Day',
      agentId: 'a',
      scheduleType: 'interval',
      scheduleValue: '1d',
      taskTitle: 't',
    });
    const next = scheduler.calculateNextRun(schedule);
    const diff = next.getTime() - before;
    expect(diff).toBeGreaterThanOrEqual(1439 * 60_000);
    expect(diff).toBeLessThanOrEqual(1441 * 60_000);
  });
});

// ─── Scheduler: cron nextRun ─────────────────────────────────────────────────

describe('Scheduler.calculateNextRun — cron', () => {
  it('returns a future date for a standard cron expression', () => {
    const db = makeDb();
    const store = makeStore(db);
    const { scheduler } = makeScheduler(store);
    const schedule = store.createSchedule({
      name: 'Daily',
      agentId: 'a',
      scheduleType: 'cron',
      scheduleValue: '0 7 * * *',
      taskTitle: 'Morning task',
    });
    const next = scheduler.calculateNextRun(schedule);
    expect(next.getTime()).toBeGreaterThan(Date.now());
  });
});

// ─── Scheduler: createSchedule sets nextRunAt ───────────────────────────────

describe('Scheduler.createSchedule', () => {
  it('sets nextRunAt on creation', () => {
    const db = makeDb();
    const store = makeStore(db);
    const { scheduler } = makeScheduler(store);

    const s = scheduler.createSchedule({
      name: 'Test',
      agentId: 'a',
      scheduleType: 'interval',
      scheduleValue: '30m',
      taskTitle: 'Test task',
    });
    expect(s.nextRunAt).not.toBeNull();
    expect(s.nextRunAt!.getTime()).toBeGreaterThan(Date.now());
  });
});

// ─── Scheduler: checkAndRunDue ───────────────────────────────────────────────

describe('Scheduler.checkAndRunDue', () => {
  it('runs due schedules and creates Task records', () => {
    const db = makeDb();
    const store = makeStore(db);
    const bus = new ModuleBus();
    const taskGraph = new TaskGraph(bus);
    const scheduler = new Scheduler(bus, taskGraph, store);

    // Create a schedule with a next_run_at in the past
    const s = store.createSchedule({
      name: 'Due now',
      agentId: 'agent-1',
      scheduleType: 'interval',
      scheduleValue: '30m',
      taskTitle: 'Run me',
    });
    // Force nextRunAt into the past
    store.setNextRunAt(s.id, new Date(Date.now() - 1000));

    const tasksBefore = taskGraph.getAllTasks().length;
    scheduler.checkAndRunDue();
    const tasksAfter = taskGraph.getAllTasks().length;
    expect(tasksAfter).toBe(tasksBefore + 1);

    const created = taskGraph.getAllTasks()[0];
    expect(created.title).toBe('Run me');
    expect(created.source).toBe('scheduled_routine');
  });

  it('does not run disabled (paused) schedules', () => {
    const db = makeDb();
    const store = makeStore(db);
    const bus = new ModuleBus();
    const taskGraph = new TaskGraph(bus);
    const scheduler = new Scheduler(bus, taskGraph, store);

    const s = store.createSchedule({
      name: 'Paused',
      agentId: 'agent-1',
      scheduleType: 'interval',
      scheduleValue: '30m',
      taskTitle: 'Paused task',
    });
    store.setNextRunAt(s.id, new Date(Date.now() - 1000));
    store.setEnabled(s.id, false);

    const tasksBefore = taskGraph.getAllTasks().length;
    scheduler.checkAndRunDue();
    expect(taskGraph.getAllTasks().length).toBe(tasksBefore);
  });

  it('updates last_run_at and recalculates next_run_at after run', () => {
    const db = makeDb();
    const store = makeStore(db);
    const bus = new ModuleBus();
    const taskGraph = new TaskGraph(bus);
    const scheduler = new Scheduler(bus, taskGraph, store);

    const s = store.createSchedule({
      name: 'After run',
      agentId: 'agent-1',
      scheduleType: 'interval',
      scheduleValue: '30m',
      taskTitle: 'After run task',
    });
    const pastTime = new Date(Date.now() - 5000);
    store.setNextRunAt(s.id, pastTime);

    scheduler.checkAndRunDue();

    const updated = store.getSchedule(s.id)!;
    expect(updated.lastRunAt).not.toBeNull();
    expect(updated.runCount).toBe(1);
    // nextRunAt should be ~30m from now
    expect(updated.nextRunAt!.getTime()).toBeGreaterThan(Date.now());
  });
});

// ─── Scheduler: pause / resume ──────────────────────────────────────────────

describe('Scheduler: pause and resume', () => {
  it('pauseSchedule sets enabled=false', () => {
    const db = makeDb();
    const store = makeStore(db);
    const { scheduler } = makeScheduler(store);

    const s = scheduler.createSchedule({
      name: 'Pausable',
      agentId: 'a',
      scheduleType: 'interval',
      scheduleValue: '1h',
      taskTitle: 'Pause test',
    });
    scheduler.pauseSchedule(s.id);
    expect(store.getSchedule(s.id)!.enabled).toBe(false);
  });

  it('resumeSchedule sets enabled=true and recalculates nextRunAt', () => {
    const db = makeDb();
    const store = makeStore(db);
    const { scheduler } = makeScheduler(store);

    const s = scheduler.createSchedule({
      name: 'Resumable',
      agentId: 'a',
      scheduleType: 'interval',
      scheduleValue: '1h',
      taskTitle: 'Resume test',
    });
    scheduler.pauseSchedule(s.id);
    // Null out nextRunAt to test it gets recalculated
    store.setNextRunAt(s.id, new Date(0));
    scheduler.resumeSchedule(s.id);

    const updated = store.getSchedule(s.id)!;
    expect(updated.enabled).toBe(true);
    expect(updated.nextRunAt!.getTime()).toBeGreaterThan(Date.now());
  });

  it('paused schedule not triggered by checkAndRunDue', () => {
    const db = makeDb();
    const store = makeStore(db);
    const bus = new ModuleBus();
    const taskGraph = new TaskGraph(bus);
    const scheduler = new Scheduler(bus, taskGraph, store);

    const s = scheduler.createSchedule({
      name: 'No fire',
      agentId: 'a',
      scheduleType: 'interval',
      scheduleValue: '30m',
      taskTitle: 'Should not run',
    });
    store.setNextRunAt(s.id, new Date(Date.now() - 1000));
    scheduler.pauseSchedule(s.id);

    scheduler.checkAndRunDue();
    expect(taskGraph.getAllTasks().length).toBe(0);
  });
});

// ─── Scheduler: runNow ──────────────────────────────────────────────────────

describe('Scheduler.runNow', () => {
  it('triggers a schedule immediately regardless of nextRunAt', () => {
    const db = makeDb();
    const store = makeStore(db);
    const bus = new ModuleBus();
    const taskGraph = new TaskGraph(bus);
    const scheduler = new Scheduler(bus, taskGraph, store);

    const s = scheduler.createSchedule({
      name: 'Run now',
      agentId: 'agent-1',
      scheduleType: 'interval',
      scheduleValue: '1d',
      taskTitle: 'Immediate task',
    });

    scheduler.runNow(s.id);
    const tasks = taskGraph.getAllTasks();
    expect(tasks).toHaveLength(1);
    expect(tasks[0].title).toBe('Immediate task');
  });

  it('throws for unknown id', () => {
    const db = makeDb();
    const store = makeStore(db);
    const { scheduler } = makeScheduler(store);
    expect(() => scheduler.runNow('ghost')).toThrow('Schedule not found: ghost');
  });
});

// ─── Scheduler: persist across restart ─────────────────────────────────────

describe('Scheduler: persist across restart', () => {
  it('schedules survive restart — loadFromStore and due runs caught up', () => {
    const db = makeDb();
    const store1 = makeStore(db);

    const bus1 = new ModuleBus();
    const tg1 = new TaskGraph(bus1);
    const sched1 = new Scheduler(bus1, tg1, store1);

    sched1.createSchedule({
      name: 'Persistent',
      agentId: 'a',
      scheduleType: 'interval',
      scheduleValue: '30m',
      taskTitle: 'Persist check',
    });

    // Simulate downtime: force nextRunAt into the past
    const all = store1.listSchedules();
    store1.setNextRunAt(all[0].id, new Date(Date.now() - 5000));

    // Restart: new scheduler, same DB
    const store2 = new SchedulerStore(db);
    store2.initSchema();
    const bus2 = new ModuleBus();
    const tg2 = new TaskGraph(bus2);
    const sched2 = new Scheduler(bus2, tg2, store2);

    // start() catches up due runs
    sched2.start();
    sched2.stop(); // stop the interval so test ends cleanly

    expect(tg2.getAllTasks().length).toBe(1);
    expect(store2.listSchedules()[0].runCount).toBe(1);
  });
});
