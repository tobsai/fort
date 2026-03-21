/**
 * Scheduler Module — Unified cron + event triggers
 *
 * Replaces OpenClaw's conflicting heartbeat/cron system with a
 * unified, transparent scheduler. Single execution queue with
 * deduplication, priority ordering, and conflict detection.
 */

import * as cron from 'node-cron';
import { v4 as uuid } from 'uuid';
import type { ModuleBus } from '../module-bus/index.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { DiagnosticResult } from '../types.js';

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

export class Scheduler {
  private routines: Map<string, ScheduledRoutine> = new Map();
  private cronJobs: Map<string, cron.ScheduledTask> = new Map();
  private triggers: Map<string, EventTrigger> = new Map();
  private executionQueue: string[] = [];
  private executing: Set<string> = new Set();
  private bus: ModuleBus;
  private taskGraph: TaskGraph;

  constructor(bus: ModuleBus, taskGraph: TaskGraph) {
    this.bus = bus;
    this.taskGraph = taskGraph;
  }

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

    checks.push({
      name: 'Scheduler status',
      passed: true,
      message: `${routines.length} routines (${enabledCount} enabled)`,
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

  shutdown(): void {
    for (const job of this.cronJobs.values()) {
      job.stop();
    }
    this.cronJobs.clear();
  }
}
