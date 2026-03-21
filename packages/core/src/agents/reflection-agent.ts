/**
 * Reflection Agent — Periodic self-review, pattern detection, proactive suggestions
 *
 * On a configurable schedule, pauses to reflect on task completions,
 * routine health, user patterns, and memory quality.
 */

import { BaseAgent } from './index.js';
import type { AgentConfig } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { MemoryManager } from '../memory/index.js';

export interface ReflectionInsight {
  type: 'suggestion' | 'observation' | 'warning';
  title: string;
  description: string;
  actionable: boolean;
  timestamp: Date;
}

export class ReflectionAgent extends BaseAgent {
  private memory: MemoryManager;
  private insights: ReflectionInsight[] = [];

  constructor(bus: ModuleBus, taskGraph: TaskGraph, memory: MemoryManager) {
    const config: AgentConfig = {
      id: 'reflection-agent',
      name: 'Reflection Agent',
      type: 'core',
      description: 'Periodic self-review, pattern detection, proactive suggestions',
      capabilities: ['reflection', 'pattern_detection', 'suggestions'],
    };
    super(config, bus, taskGraph);
    this.memory = memory;
  }

  protected async onStart(): Promise<void> {
    this.bus.subscribe('scheduler.reflect', async () => {
      await this.reflect();
    });
  }

  protected async onStop(): Promise<void> {}

  protected async onTask(taskId: string): Promise<void> {
    await this.reflect();
    this.taskGraph.updateStatus(taskId, 'completed');
  }

  async reflect(): Promise<ReflectionInsight[]> {
    const newInsights: ReflectionInsight[] = [];

    // Check for stale tasks
    const staleTasks = this.taskGraph.getStaleTasks();
    if (staleTasks.length > 0) {
      newInsights.push({
        type: 'warning',
        title: 'Stale tasks detected',
        description: `${staleTasks.length} tasks have been in progress or blocked for too long: ${staleTasks.map((t) => t.title).join(', ')}`,
        actionable: true,
        timestamp: new Date(),
      });
    }

    // Check task completion patterns
    const allTasks = this.taskGraph.getAllTasks();
    const completedTasks = allTasks.filter((t) => t.status === 'completed');
    const failedTasks = allTasks.filter((t) => t.status === 'failed');

    if (failedTasks.length > 0 && failedTasks.length / Math.max(allTasks.length, 1) > 0.2) {
      newInsights.push({
        type: 'warning',
        title: 'High failure rate',
        description: `${failedTasks.length} out of ${allTasks.length} tasks have failed. Consider investigating common failure patterns.`,
        actionable: true,
        timestamp: new Date(),
      });
    }

    // Check memory health
    const memStats = this.memory.stats();
    if (memStats.nodeCount === 0) {
      newInsights.push({
        type: 'observation',
        title: 'Empty memory graph',
        description: 'No memories stored yet. Memory will grow as you interact with Fort.',
        actionable: false,
        timestamp: new Date(),
      });
    }

    // Store insights
    this.insights.push(...newInsights);

    // Publish insights as events
    for (const insight of newInsights) {
      await this.bus.publish('reflection.insight', 'reflection-agent', insight);
    }

    return newInsights;
  }

  getInsights(limit?: number): ReflectionInsight[] {
    const sorted = this.insights.sort(
      (a, b) => b.timestamp.getTime() - a.timestamp.getTime()
    );
    return limit ? sorted.slice(0, limit) : sorted;
  }

  async handleMessage(fromAgentId: string, message: unknown): Promise<void> {
    const msg = message as { type: string };
    if (msg.type === 'reflect') {
      const insights = await this.reflect();
      await this.sendMessage(fromAgentId, { type: 'insights', insights });
    }
  }
}
