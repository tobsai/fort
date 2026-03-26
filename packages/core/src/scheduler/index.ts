/**
 * Scheduler Module — Unified cron + event triggers
 *
 * Replaces OpenClaw's conflicting heartbeat/cron system with a
 * unified, transparent scheduler. Single execution queue with
 * deduplication, priority ordering, and conflict detection.
 *
 * SPEC-008: Adds DB-backed ScheduledTask API with cron/interval support.
 */

import * as cron from 'node-cron';
import { Cron } from 'croner';
import { v4 as uuid } from 'uuid';
import type { ModuleBus } from '../module-bus/index.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { DiagnosticResult } from '../types.js';
import { SchedulerStore, type ScheduleConfig, type ScheduledTask } from './store.js';

export { SchedulerStore, type ScheduleConfig, type ScheduledTask } from './store.js';

// ─── Existing in-process routine API (used by RoutineManager) ──────────────

export interface ScheduledRoutine {
  id: string;
  name: string;
  description: string;
  cronExpression: string;
  handler: () => Promise<void>;
  enabled: boolean;
  lastRunAt: Date | null;
  lastResult: 'success' | 'failure' | null;
  lastError: string | null;
  runCount: number;
  failCount: number;
  createdAt: Date;
}

export interface EventTrigger {
  id: string;
  name: string;
  eventType: string;
  handler: (payload: unknown) => Promise<void>;
  enabled: boolean;
}

// ─── Scheduler ─────────────────────────────────────────────────────────────

export class Scheduler {
  // In-process routines (addRoutine / RoutineManager)
  private routines: Map<string, ScheduledRoutine> = new Map();
  private cronJobs: Map<string, cron.ScheduledTask> = new Map();
  private triggers: Map<string, EventTrigger> = new Map();
  private executing: Set<string> = new Set();

  // DB-backed scheduled tasks (SPEC-008)
  private store: SchedulerStore | null = null;
  private intervalHandle: ReturnType<typeof setInterval> | null = null;

  private bus: ModuleBus;
  private taskGraph: TaskGraph;

  constructor(bus: ModuleBus, taskGraph: TaskGraph, store?: SchedulerStore) {
    this.bus = bus;
    this.taskGraph = taskGraph;
    this.store = store ?? null;
  }

  // ─── SPEC-008: Lifecycle ────────────────────────────────────────────────

  /**
   * Start the check-every-minute loop for DB-backed scheduled tasks.
   * Also initialises nextRunAt for any schedule that doesn't have one yet.
   */
  start(): void {
    if (!this.store) return;
    if (this.intervalHandle) return; // already running

    // Initialise nextRunAt for schedules that lack one
    const schedules = this.store.listSchedules();
    for (const s of schedules) {
      if (s.enabled && !s.nextRunAt) {
        const next = this.calculateNextRun(s);
        this.store.setNextRunAt(s.id, next);
      }
    }

    this.intervalHandle = setInterval(() => {
      this.checkAndRunDue();
    }, 60_000);

    // Run an immediate check in case something was due during downtime
    this.checkAndRunDue();
  }

  /**
   * Stop the check loop (does not stop in-process cron jobs).
   */
  stop(): void {
    if (this.intervalHandle) {
      clearInterval(this.intervalHandle);
      this.intervalHandle = null;
    }
  }

  // ─── SPEC-008: CRUD API ─────────────────────────────────────────────────

  createSchedule(config: ScheduleConfig): ScheduledTask {
    if (!this.store) throw new Error('SchedulerStore not configured');
    const schedule = this.store.createSchedule(config);
    const next = this.calculateNextRun(schedule);
    this.store.setNextRunAt(schedule.id, next);
    const updated = this.store.getSchedule(schedule.id)!;
    this.bus.publish('scheduler.schedule_created', 'scheduler', { schedule: updated });
    return updated;
  }

  updateSchedule(id: string, updates: Partial<ScheduleConfig>): ScheduledTask {
    if (!this.store) throw new Error('SchedulerStore not configured');
    const updated = this.store.updateSchedule(id, updates);
    // Recalculate next run if schedule expression changed
    if (updates.scheduleType !== undefined || updates.scheduleValue !== undefined) {
      const next = this.calculateNextRun(updated);
      this.store.setNextRunAt(id, next);
    }
    return this.store.getSchedule(id)!;
  }

  deleteSchedule(id: string): void {
    if (!this.store) throw new Error('SchedulerStore not configured');
    this.store.deleteSchedule(id);
    this.bus.publish('scheduler.schedule_deleted', 'scheduler', { id });
  }

  pauseSchedule(id: string): void {
    if (!this.store) throw new Error('SchedulerStore not configured');
    this.store.setEnabled(id, false);
    this.bus.publish('scheduler.schedule_paused', 'scheduler', { id });
  }

  resumeSchedule(id: string): void {
    if (!this.store) throw new Error('SchedulerStore not configured');
    this.store.setEnabled(id, true);
    const schedule = this.store.getSchedule(id);
    if (schedule) {
      const next = this.calculateNextRun(schedule);
      this.store.setNextRunAt(id, next);
    }
    this.bus.publish('scheduler.schedule_resumed', 'scheduler', { id });
  }

  listSchedules(): ScheduledTask[] {
    if (!this.store) return [];
    return this.store.listSchedules();
  }

  getSchedule(id: string): ScheduledTask | null {
    if (!this.store) return null;
    return this.store.getSchedule(id);
  }

  // ─── SPEC-008: Internal check + run ────────────────────────────────────

  /** Called every 60s — runs all schedules whose nextRunAt is in the past. */
  checkAndRunDue(): void {
    if (!this.store) return;
    const now = new Date();
    const schedules = this.store.listSchedules();
    for (const s of schedules) {
      if (!s.enabled) continue;
      if (!s.nextRunAt) continue;
      if (s.nextRunAt <= now) {
        this.runSchedule(s);
      }
    }
  }

  /** Trigger a schedule immediately (for testing / run-now). */
  runNow(id: string): void {
    if (!this.store) throw new Error('SchedulerStore not configured');
    const schedule = this.store.getSchedule(id);
    if (!schedule) throw new Error(`Schedule not found: ${id}`);
    this.runSchedule(schedule);
  }

  private runSchedule(schedule: ScheduledTask): void {
    const task = this.taskGraph.createTask({
      title: schedule.taskTitle,
      description: schedule.taskDescription,
      source: 'scheduled_routine',
      assignedAgent: schedule.agentId,
      metadata: { scheduleId: schedule.id, scheduleName: schedule.name },
    });

    const next = this.calculateNextRun(schedule);
    this.store!.updateRunInfo(schedule.id, new Date(), next);

    this.bus.publish('scheduler.schedule_fired', 'scheduler', {
      scheduleId: schedule.id,
      scheduleName: schedule.name,
      taskId: task.id,
      nextRunAt: next,
    });
  }

  /** Calculate the next Date a schedule should run. */
  calculateNextRun(schedule: ScheduledTask): Date {
    if (schedule.scheduleType === 'interval') {
      return this.calculateIntervalNext(schedule.scheduleValue);
    }
    return this.calculateCronNext(schedule.scheduleValue);
  }

  private calculateIntervalNext(value: string): Date {
    const now = Date.now();
    const match = value.match(/^(\d+)(m|h|d)$/);
    if (!match) throw new Error(`Invalid interval value: ${value}`);
    const n = parseInt(match[1], 10);
    const unit = match[2];
    let ms: number;
    if (unit === 'm') {
      ms = n * 60_000;
    } else if (unit === 'h') {
      ms = n * 3_600_000;
    } else {
      ms = n * 86_400_000;
    }
    return new Date(now + ms);
  }

  private calculateCronNext(expr: string): Date {
    const job = new Cron(expr);
    const next = job.nextRun();
    if (!next) throw new Error(`Could not calculate next run for cron: ${expr}`);
    return next;
  }

  // ─── Existing in-process routine API ───────────────────────────────────

  addRoutine(params: {
    name: string;
    description?: string;
    cronExpression: string;
    handler: () => Promise<void>;
    enabled?: boolean;
  }): ScheduledRoutine {
    if (!cron.validate(params.cronExpression)) {
      throw new Error(`Invalid cron expression: ${params.cronExpression}`);
    }

    const routine: ScheduledRoutine = {
      id: uuid(),
      name: params.name,
      description: params.description ?? '',
      cronExpression: params.cronExpression,
      handler: params.handler,
      enabled: params.enabled ?? true,
      lastRunAt: null,
      lastResult: null,
      lastError: null,
      runCount: 0,
      failCount: 0,
      createdAt: new Date(),
    };

    this.routines.set(routine.id, routine);

    if (routine.enabled) {
      this.startCronJob(routine);
    }

    this.bus.publish('scheduler.routine_added', 'scheduler', { routine: { id: routine.id, name: routine.name } });
    return routine;
  }

  private startCronJob(routine: ScheduledRoutine): void {
    const job = cron.schedule(routine.cronExpression, async () => {
      await this.executeRoutine(routine.id);
    });
    this.cronJobs.set(routine.id, job);
  }

  async executeRoutine(routineId: string): Promise<void> {
    const routine = this.routines.get(routineId);
    if (!routine) throw new Error(`Routine not found: ${routineId}`);

    // Deduplication: skip if already executing
    if (this.executing.has(routineId)) return;
    this.executing.add(routineId);

    // Create a task for transparency
    const task = this.taskGraph.createTask({
      title: `Routine: ${routine.name}`,
      description: routine.description,
      source: 'scheduled_routine',
      metadata: { routineId },
    });

    try {
      this.taskGraph.updateStatus(task.id, 'in_progress');
      await routine.handler();

      routine.lastRunAt = new Date();
      routine.lastResult = 'success';
      routine.lastError = null;
      routine.runCount++;

      this.taskGraph.updateStatus(task.id, 'completed');
      this.bus.publish('scheduler.routine_completed', 'scheduler', {
        routineId, name: routine.name,
      });
    } catch (err) {
      routine.lastRunAt = new Date();
      routine.lastResult = 'failure';
      routine.lastError = err instanceof Error ? err.message : String(err);
      routine.failCount++;

      this.taskGraph.updateStatus(task.id, 'failed', routine.lastError);
      this.bus.publish('scheduler.routine_failed', 'scheduler', {
        routineId, name: routine.name, error: routine.lastError,
      });
    } finally {
      this.executing.delete(routineId);
    }
  }

  addTrigger(params: {
    name: string;
    eventType: string;
    handler: (payload: unknown) => Promise<void>;
  }): EventTrigger {
    const trigger: EventTrigger = {
      id: uuid(),
      name: params.name,
      eventType: params.eventType,
      handler: params.handler,
      enabled: true,
    };

    this.triggers.set(trigger.id, trigger);

    this.bus.subscribe(params.eventType, async (event) => {
      if (!trigger.enabled) return;
      await trigger.handler(event.payload);
    });

    return trigger;
  }

  enableRoutine(routineId: string): void {
    const routine = this.routines.get(routineId);
    if (!routine) throw new Error(`Routine not found: ${routineId}`);
    routine.enabled = true;
    if (!this.cronJobs.has(routineId)) {
      this.startCronJob(routine);
    }
  }

  disableRoutine(routineId: string): void {
    const routine = this.routines.get(routineId);
    if (!routine) throw new Error(`Routine not found: ${routineId}`);
    routine.enabled = false;
    const job = this.cronJobs.get(routineId);
    if (job) {
      job.stop();
      this.cronJobs.delete(routineId);
    }
  }

  removeRoutine(routineId: string): void {
    this.disableRoutine(routineId);
    this.routines.delete(routineId);
  }

  listRoutines(): ScheduledRoutine[] {
    return Array.from(this.routines.values());
  }

  listTriggers(): EventTrigger[] {
    return Array.from(this.triggers.values());
  }

  getRoutine(routineId: string): ScheduledRoutine | undefined {
    return this.routines.get(routineId);
  }

  diagnose(): DiagnosticResult {
    const checks = [];

    const routines = this.listRoutines();
    const enabledCount = routines.filter((r) => r.enabled).length;
    const failedRecently = routines.filter((r) => r.lastResult === 'failure');

    const dbSchedules = this.store ? this.store.listSchedules() : [];
    const activeDbSchedules = dbSchedules.filter((s) => s.enabled).length;

    checks.push({
      name: 'Scheduler status',
      passed: true,
      message: `${routines.length} routines (${enabledCount} enabled), ${dbSchedules.length} DB schedules (${activeDbSchedules} active)`,
    });

    if (failedRecently.length > 0) {
      checks.push({
        name: 'Failed routines',
        passed: false,
        message: `${failedRecently.length} routines failed recently: ${failedRecently.map((r) => r.name).join(', ')}`,
      });
    } else {
      checks.push({
        name: 'Routine health',
        passed: true,
        message: 'No recent failures',
      });
    }

    return {
      module: 'scheduler',
      status: checks.every((c) => c.passed) ? 'healthy' : 'degraded',
      checks,
    };
  }

  /** Stops all cron jobs and the DB check loop. */
  shutdown(): void {
    this.stop();
    for (const job of this.cronJobs.values()) {
      job.stop();
    }
    this.cronJobs.clear();
  }
}
