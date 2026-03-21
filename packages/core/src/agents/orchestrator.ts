/**
 * Orchestrator Agent — The central reasoning engine
 *
 * Receives user input, decomposes it into a task graph, and delegates
 * to long-lived agents or modules. This is the first core agent.
 */

import { BaseAgent } from './index.js';
import type { AgentConfig } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { AgentRegistry } from './index.js';

export class OrchestratorAgent extends BaseAgent {
  private registry: AgentRegistry | null = null;

  constructor(bus: ModuleBus, taskGraph: TaskGraph) {
    const config: AgentConfig = {
      id: 'orchestrator',
      name: 'Orchestrator',
      type: 'core',
      description: 'Routes tasks, decomposes goals, coordinates agents',
      capabilities: ['task_routing', 'task_decomposition', 'agent_coordination'],
    };
    super(config, bus, taskGraph);
  }

  setRegistry(registry: AgentRegistry): void {
    this.registry = registry;
  }

  protected async onStart(): Promise<void> {
    // Subscribe to new user input
    this.bus.subscribe('user.input', async (event) => {
      const { message, source } = event.payload as { message: string; source: string };
      await this.handleUserInput(message, source);
    });

    // Subscribe to stale task checks
    this.bus.subscribe('scheduler.stale_check', async () => {
      await this.checkStaleTasks();
    });
  }

  protected async onStop(): Promise<void> {
    // Cleanup subscriptions handled by bus clear
  }

  protected async onTask(taskId: string): Promise<void> {
    const task = this.taskGraph.getTask(taskId);

    // For now, simple routing based on task metadata
    // In Phase 2+, this will use LLM reasoning for complex decomposition
    const agentId = this.routeTask(task.title, task.description);

    if (agentId && agentId !== this.config.id) {
      this.taskGraph.assignAgent(taskId, agentId);
      const agent = this.registry?.get(agentId);
      if (agent) {
        await agent.handleTask(taskId);
      }
    } else {
      // Handle directly — simple tasks
      this.taskGraph.updateStatus(taskId, 'completed', 'Handled by orchestrator');
    }
  }

  async handleUserInput(message: string, source: string = 'user_chat'): Promise<string> {
    // Core invariant: every conversation creates a task
    const task = this.taskGraph.createTask({
      title: message.slice(0, 100),
      description: message,
      source: source as any,
      assignedAgent: 'orchestrator',
    });

    this.bus.publish('task.received', 'orchestrator', {
      taskId: task.id,
      message,
      source,
    });

    // Route and handle
    await this.handleTask(task.id);
    return task.id;
  }

  private routeTask(title: string, description: string): string | null {
    const text = `${title} ${description}`.toLowerCase();

    // Simple keyword-based routing for Phase 1
    // Phase 2+ will use LLM reasoning
    if (text.includes('memory') || text.includes('remember') || text.includes('forget')) {
      return 'memory-agent';
    }
    if (text.includes('schedule') || text.includes('routine') || text.includes('cron')) {
      return 'scheduler-agent';
    }
    if (text.includes('reflect') || text.includes('improve') || text.includes('suggest')) {
      return 'reflection-agent';
    }

    // Default: handle in orchestrator
    return null;
  }

  private async checkStaleTasks(): Promise<void> {
    const staleTasks = this.taskGraph.getStaleTasks();
    for (const task of staleTasks) {
      this.bus.publish('task.stale', 'orchestrator', {
        taskId: task.id,
        title: task.title,
        assignedAgent: task.assignedAgent,
        staleSince: task.updatedAt,
      });
    }
  }

  async handleMessage(fromAgentId: string, message: unknown): Promise<void> {
    const msg = message as { type: string; data: unknown };
    if (msg.type === 'task_complete') {
      // Agent reporting completion
      this.bus.publish('agent.task_reported', 'orchestrator', {
        from: fromAgentId,
        data: msg.data,
      });
    }
  }
}
