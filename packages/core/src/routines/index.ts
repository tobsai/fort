/**
 * Routine Module — Scheduled, repeatable workflows for Fort
 *
 * Routines are deterministic flows triggered by the Scheduler.
 * They integrate with TaskGraph for transparency and the memory
 * graph for persistence. Examples:
 *   - "Daily briefing at 7 AM"
 *   - "Weekly project status summary on Fridays"
 *   - "Check email every 30 minutes and flag urgent items"
 */

import { v4 as uuid } from 'uuid';
import type { Routine, RoutineStep, RoutineExecution, DiagnosticResult } from '../types.js';
import type { MemoryManager } from '../memory/index.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { Scheduler } from '../scheduler/index.js';
import type { TaskGraph } from '../task-graph/index.js';

export class RoutineManager {
  private memory: MemoryManager;
  private bus: ModuleBus;
  private scheduler: Scheduler;
  private taskGraph: TaskGraph;
  private executions: Map<string, RoutineExecution[]> = new Map();

  constructor(
    memory: MemoryManager,
    bus: ModuleBus,
    scheduler: Scheduler,
    taskGraph: TaskGraph,
  ) {
    this.memory = memory;
    this.bus = bus;
    this.scheduler = scheduler;
    this.taskGraph = taskGraph;
  }

  /**
   * Create a routine, store it in the memory graph, and register with the Scheduler.
   */
  addRoutine(params: {
    name: string;
    description: string;
    schedule: string;
    steps: RoutineStep[];
    flowId?: string;
    source?: string;
    behaviors?: string[];
    enabled?: boolean;
  }): Routine {
    const id = uuid();
    const now = new Date().toISOString();
    const enabled = params.enabled ?? true;

    const routine: Routine = {
      id,
      name: params.name,
      description: params.description,
      schedule: params.schedule,
      flowId: params.flowId,
      steps: params.steps,
      enabled,
      createdAt: now,
      source: params.source ?? 'user',
      behaviors: params.behaviors,
    };

    // Store as memory graph node
    this.memory.createNode({
      type: 'routine',
      label: params.name,
      properties: {
        routineId: id,
        name: params.name,
        description: params.description,
        schedule: params.schedule,
        flowId: params.flowId,
        steps: params.steps,
        enabled,
        source: routine.source,
        behaviors: params.behaviors ?? [],
        createdAt: now,
      },
      source: routine.source,
    });

    // Register with the Scheduler
    if (enabled) {
      this.registerWithScheduler(routine);
    }

    this.bus.publish('routine.added', 'routines', { routine });
    return routine;
  }

  removeRoutine(id: string): boolean {
    const node = this.findRoutineNode(id);
    if (!node) return false;

    // Remove from scheduler
    try {
      this.scheduler.removeRoutine(id);
    } catch {
      // May not be registered if disabled
    }

    // Mark disabled in memory graph
    this.memory.updateNode(node.id, {
      properties: { ...node.properties, enabled: false, removed: true },
    });

    this.bus.publish('routine.removed', 'routines', { routineId: id });
    return true;
  }

  getRoutine(id: string): Routine | null {
    const node = this.findRoutineNode(id);
    if (!node) return null;
    return this.nodeToRoutine(node);
  }

  listRoutines(): Routine[] {
    const result = this.memory.search({ nodeType: 'routine' });
    return result.nodes
      .map((n) => this.nodeToRoutine(n))
      .filter((r) => !r.lastResult || r.lastResult !== 'skipped' || true) // include all
      .filter((r) => {
        const node = this.findRoutineNode(r.id);
        return node && !node.properties.removed;
      });
  }

  enableRoutine(id: string): boolean {
    const node = this.findRoutineNode(id);
    if (!node) return false;

    this.memory.updateNode(node.id, {
      properties: { ...node.properties, enabled: true },
    });

    const routine = this.nodeToRoutine(node);
    routine.enabled = true;
    this.registerWithScheduler(routine);

    this.bus.publish('routine.enabled', 'routines', { routineId: id });
    return true;
  }

  disableRoutine(id: string): boolean {
    const node = this.findRoutineNode(id);
    if (!node) return false;

    this.memory.updateNode(node.id, {
      properties: { ...node.properties, enabled: false },
    });

    try {
      this.scheduler.disableRoutine(id);
    } catch {
      // May not be registered
    }

    this.bus.publish('routine.disabled', 'routines', { routineId: id });
    return true;
  }

  /**
   * Manually trigger a routine execution. Creates a task in TaskGraph
   * and runs each step sequentially.
   */
  async executeRoutine(id: string): Promise<RoutineExecution> {
    const routine = this.getRoutine(id);
    if (!routine) throw new Error(`Routine not found: ${id}`);

    const executionId = uuid();
    const startedAt = new Date().toISOString();

    // Create a task for transparency
    const task = this.taskGraph.createTask({
      title: `Routine: ${routine.name}`,
      description: routine.description,
      source: 'scheduled_routine',
      metadata: { routineId: id, executionId },
    });

    this.taskGraph.updateStatus(task.id, 'in_progress');

    const execution: RoutineExecution = {
      id: executionId,
      routineId: id,
      startedAt,
      result: 'success',
      taskId: task.id,
    };

    try {
      // Execute each step
      for (const step of routine.steps) {
        try {
          await this.bus.publish('routine.step_executing', 'routines', {
            routineId: id,
            executionId,
            step,
          });

          // Steps are executed via event — tools/integrations subscribe
          // For now, emit an action event that tool handlers can pick up
          await this.bus.publish(`routine.action.${step.action}`, 'routines', {
            routineId: id,
            executionId,
            stepId: step.id,
            params: step.params,
          });
        } catch (err) {
          const errorMsg = err instanceof Error ? err.message : String(err);
          if (step.onError === 'abort') {
            throw new Error(`Step '${step.action}' failed: ${errorMsg}`);
          }
          // 'skip' or 'retry' — for now, skip on error
          await this.bus.publish('routine.step_error', 'routines', {
            routineId: id,
            executionId,
            stepId: step.id,
            error: errorMsg,
          });
        }
      }

      execution.completedAt = new Date().toISOString();
      execution.result = 'success';
      this.taskGraph.updateStatus(task.id, 'completed');

      // Update the routine's last run info in memory
      this.updateRoutineLastRun(id, 'success');
    } catch (err) {
      execution.completedAt = new Date().toISOString();
      execution.result = 'failure';
      execution.error = err instanceof Error ? err.message : String(err);

      this.taskGraph.updateStatus(task.id, 'failed', execution.error);
      this.updateRoutineLastRun(id, 'failure');
    }

    // Store execution in history
    if (!this.executions.has(id)) {
      this.executions.set(id, []);
    }
    this.executions.get(id)!.push(execution);

    this.bus.publish('routine.executed', 'routines', { execution });
    return execution;
  }

  /**
   * Get execution history for a routine.
   */
  getHistory(id: string, limit: number = 20): RoutineExecution[] {
    const history = this.executions.get(id) ?? [];
    return history
      .sort((a, b) => b.startedAt.localeCompare(a.startedAt))
      .slice(0, limit);
  }

  diagnose(): DiagnosticResult {
    const routines = this.listRoutines();
    const enabled = routines.filter((r) => r.enabled);
    const failed = routines.filter((r) => r.lastResult === 'failure');

    const checks = [
      {
        name: 'Routine count',
        passed: true,
        message: `${routines.length} routines (${enabled.length} enabled)`,
      },
    ];

    if (failed.length > 0) {
      checks.push({
        name: 'Failed routines',
        passed: false,
        message: `${failed.length} routines failed recently: ${failed.map((r) => r.name).join(', ')}`,
      });
    } else {
      checks.push({
        name: 'Routine health',
        passed: true,
        message: 'No recent failures',
      });
    }

    return {
      module: 'routines',
      status: checks.every((c) => c.passed) ? 'healthy' : 'degraded',
      checks,
    };
  }

  // ─── Internal Helpers ───────────────────────────────────────────

  private findRoutineNode(routineId: string): ReturnType<MemoryManager['getNode']> {
    const result = this.memory.search({ nodeType: 'routine' });
    return result.nodes.find((n) => n.properties.routineId === routineId) ?? null;
  }

  private nodeToRoutine(node: NonNullable<ReturnType<MemoryManager['getNode']>>): Routine {
    return {
      id: (node.properties.routineId as string) ?? node.id,
      name: (node.properties.name as string) ?? node.label,
      description: (node.properties.description as string) ?? '',
      schedule: (node.properties.schedule as string) ?? '',
      flowId: node.properties.flowId as string | undefined,
      steps: (node.properties.steps as RoutineStep[]) ?? [],
      enabled: (node.properties.enabled as boolean) ?? true,
      lastRun: node.properties.lastRun as string | undefined,
      lastResult: node.properties.lastResult as Routine['lastResult'],
      createdAt: (node.properties.createdAt as string) ?? node.createdAt.toISOString(),
      source: node.source || (node.properties.source as string) || 'unknown',
      behaviors: (node.properties.behaviors as string[]) ?? undefined,
    };
  }

  private registerWithScheduler(routine: Routine): void {
    // The Scheduler.addRoutine generates its own ID, but we use the routine ID
    // by removing any existing registration first, then adding
    try {
      this.scheduler.removeRoutine(routine.id);
    } catch {
      // Not registered yet
    }

    this.scheduler.addRoutine({
      name: routine.name,
      description: routine.description,
      cronExpression: routine.schedule,
      handler: async () => {
        await this.executeRoutine(routine.id);
      },
      enabled: routine.enabled,
    });
  }

  private updateRoutineLastRun(id: string, result: 'success' | 'failure'): void {
    const node = this.findRoutineNode(id);
    if (node) {
      this.memory.updateNode(node.id, {
        properties: {
          ...node.properties,
          lastRun: new Date().toISOString(),
          lastResult: result,
        },
      });
    }
  }
}
