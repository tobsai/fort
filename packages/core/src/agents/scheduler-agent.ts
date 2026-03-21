/**
 * Scheduler Agent — Owns all cron jobs, event triggers, and routine execution
 *
 * Core agent that manages the unified scheduler. Ensures routines run
 * transparently with full task tracking.
 */

import { BaseAgent } from './index.js';
import type { AgentConfig } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { Scheduler } from '../scheduler/index.js';

export class SchedulerAgent extends BaseAgent {
  private scheduler: Scheduler;

  constructor(bus: ModuleBus, taskGraph: TaskGraph, scheduler: Scheduler) {
    const config: AgentConfig = {
      id: 'scheduler-agent',
      name: 'Scheduler Agent',
      type: 'core',
      description: 'Owns all cron jobs, event triggers, and routine execution',
      capabilities: ['scheduling', 'routine_management', 'event_triggers'],
    };
    super(config, bus, taskGraph);
    this.scheduler = scheduler;
  }

  protected async onStart(): Promise<void> {
    // Listen for schedule requests
    this.bus.subscribe('scheduler.request', async (event) => {
      const req = event.payload as {
        action: string;
        params: Record<string, unknown>;
      };
      await this.handleScheduleRequest(req);
    });
  }

  protected async onStop(): Promise<void> {
    this.scheduler.shutdown();
  }

  protected async onTask(taskId: string): Promise<void> {
    const task = this.taskGraph.getTask(taskId);
    const text = task.description.toLowerCase();

    if (text.includes('list') || text.includes('show')) {
      const routines = this.scheduler.listRoutines();
      task.metadata.routines = routines.map(({ handler: _, ...r }) => r);
      this.taskGraph.updateStatus(taskId, 'completed');
    } else {
      this.taskGraph.updateStatus(taskId, 'completed', 'Acknowledged by scheduler');
    }
  }

  private async handleScheduleRequest(req: { action: string; params: Record<string, unknown> }): Promise<void> {
    switch (req.action) {
      case 'list':
        this.bus.publish('scheduler.response', 'scheduler-agent', {
          routines: this.scheduler.listRoutines().map(({ handler: _, ...r }) => r),
        });
        break;
      case 'enable':
        if (typeof req.params.routineId === 'string') {
          this.scheduler.enableRoutine(req.params.routineId);
        }
        break;
      case 'disable':
        if (typeof req.params.routineId === 'string') {
          this.scheduler.disableRoutine(req.params.routineId);
        }
        break;
    }
  }

  async handleMessage(fromAgentId: string, message: unknown): Promise<void> {
    const msg = message as { type: string; data?: unknown };
    if (msg.type === 'status') {
      await this.sendMessage(fromAgentId, {
        type: 'scheduler_status',
        routines: this.scheduler.listRoutines().length,
        triggers: this.scheduler.listTriggers().length,
      });
    }
  }
}
