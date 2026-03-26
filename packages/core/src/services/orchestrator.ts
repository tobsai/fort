/**
 * Orchestrator Service — Deterministic task routing
 *
 * Not an agent. Routes chat messages and tasks to the appropriate
 * specialist agent. Every interaction creates a task.
 */

import type { Task, TaskSource } from '../types.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { AgentRegistry } from '../agents/index.js';
import type { ModuleBus } from '../module-bus/index.js';

export class OrchestratorService {
  private taskGraph: TaskGraph;
  private agents: AgentRegistry;
  private bus: ModuleBus;

  constructor(taskGraph: TaskGraph, agents: AgentRegistry, bus: ModuleBus) {
    this.taskGraph = taskGraph;
    this.agents = agents;
    this.bus = bus;
  }

  /**
   * Route a delegated task to its assigned agent.
   * Validates that the target agent exists and is running, then dispatches.
   * Used for programmatic delegation outside of the delegate-to-agent tool.
   */
  async routeTask(taskId: string): Promise<void> {
    const task = this.taskGraph.getTask(taskId);
    if (!task.assignedAgent) {
      throw new Error(`Task ${taskId} has no assigned agent`);
    }

    const agent = this.agents.get(task.assignedAgent);
    if (!agent) {
      this.taskGraph.updateStatus(taskId, 'failed', `Target agent "${task.assignedAgent}" not found`);
      throw new Error(`Agent not found: ${task.assignedAgent}`);
    }
    if (agent.status === 'stopped' || agent.status === 'error') {
      this.taskGraph.updateStatus(taskId, 'failed', `Target agent "${task.assignedAgent}" is ${agent.status}`);
      throw new Error(`Agent "${task.assignedAgent}" is ${agent.status}`);
    }

    try {
      await agent.handleTask(taskId);
    } catch (err) {
      const errMsg = err instanceof Error ? err.message : String(err);
      this.taskGraph.updateStatus(taskId, 'failed', errMsg);
      this.bus.publish('task.failed', 'orchestrator', { taskId, error: errMsg, agentId: task.assignedAgent });
      throw err;
    }
  }

  /**
   * Route a chat message to an agent. Creates a task and dispatches it.
   * If no agentId provided, routes to the default agent.
   */
  async routeChat(
    message: string,
    source: TaskSource = 'user_chat',
    agentId?: string,
    modelTier?: string,
  ): Promise<Task> {
    // Find target agent
    const targetId = agentId ?? this.findDefaultAgentId();

    if (!targetId) {
      throw new Error('No agent available. Create an agent first via the portal.');
    }

    const agent = this.agents.get(targetId);
    if (!agent) {
      throw new Error(`Agent not found: ${targetId}`);
    }

    // Create task — every chat creates a task
    const metadata: Record<string, unknown> = { type: 'chat', agentId: targetId };
    if (modelTier) metadata.modelTier = modelTier;

    const task = this.taskGraph.createTask({
      title: message.slice(0, 100),
      description: message,
      source,
      assignedAgent: targetId,
      metadata,
    });

    this.bus.publish('task.received', 'orchestrator', {
      taskId: task.id,
      message,
      agentId: targetId,
      source,
    });

    // Dispatch to agent with failure handling
    try {
      await agent.handleTask(task.id);
    } catch (err) {
      const errMsg = err instanceof Error ? err.message : String(err);
      this.taskGraph.updateStatus(task.id, 'failed', errMsg);
      this.bus.publish('task.failed', 'orchestrator', {
        taskId: task.id,
        error: errMsg,
        agentId: targetId,
      });
    }

    return task;
  }

  /**
   * Create a user-owned task (not assigned to an agent).
   */
  createUserTask(title: string, description?: string): Task {
    const task = this.taskGraph.createTask({
      title,
      description,
      source: 'user_chat',
    });
    // Mark as user-owned
    task.assignedTo = 'user';
    return task;
  }

  /**
   * Check for stale tasks and publish warnings.
   */
  checkStaleTasks(): void {
    const staleTasks = this.taskGraph.getStaleTasks();
    for (const task of staleTasks) {
      this.bus.publish('task.stale', 'orchestrator', {
        taskId: task.id,
        title: task.title,
        assignedAgent: task.assignedAgent,
      });
    }
  }

  /**
   * Find the default agent ID.
   */
  findDefaultAgentId(): string | null {
    const agents = this.agents.listInfo();
    // Look for isDefault on specialist agents
    for (const agent of agents) {
      const specialist = this.agents.get(agent.config.id);
      if (specialist && 'identity' in specialist) {
        const identity = (specialist as any).identity;
        if (identity?.isDefault) return agent.config.id;
      }
    }
    // Fallback: first running specialist
    const running = agents.filter((a) => a.status === 'running');
    return running.length > 0 ? running[0].config.id : null;
  }
}
