/**
 * Specialist Agent — Data-driven agent created from an identity file
 *
 * Unlike core agents which are coded in TypeScript, specialist agents
 * are defined entirely by their identity YAML. They have their own
 * memory partition, behavioral rules, and event subscriptions.
 *
 * This is Fort's equivalent of OpenClaw's "hatch" process.
 */

import { BaseAgent } from './index.js';
import type { AgentConfig, SpecialistIdentity } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { MemoryManager } from '../memory/index.js';

export class SpecialistAgent extends BaseAgent {
  readonly identity: SpecialistIdentity;
  private memory: MemoryManager;
  private unsubscribers: Array<() => void> = [];

  constructor(
    identity: SpecialistIdentity,
    bus: ModuleBus,
    taskGraph: TaskGraph,
    memory: MemoryManager,
  ) {
    const config: AgentConfig = {
      id: identity.id,
      name: identity.name,
      type: 'specialist',
      description: identity.description,
      capabilities: identity.capabilities,
      memoryPartition: identity.memoryPartition,
    };
    super(config, bus, taskGraph);
    this.identity = identity;
    this.memory = memory;
  }

  protected async onStart(): Promise<void> {
    // Subscribe to configured events
    for (const eventType of this.identity.eventSubscriptions) {
      const unsub = this.bus.subscribe(eventType, async (event) => {
        // Create a task for every event this specialist handles
        const task = this.taskGraph.createTask({
          title: `[${this.identity.name}] Handle ${eventType}`,
          description: JSON.stringify(event.payload),
          source: 'agent_delegation',
          assignedAgent: this.config.id,
        });
        await this.handleTask(task.id);
      });
      this.unsubscribers.push(unsub);
    }

    // Store agent activation in memory
    this.memory.createNode({
      type: 'fact',
      label: `${this.identity.name} started`,
      properties: {
        agentId: this.identity.id,
        event: 'agent_started',
        timestamp: new Date().toISOString(),
      },
      source: `agent:${this.identity.id}`,
    });
  }

  protected async onStop(): Promise<void> {
    for (const unsub of this.unsubscribers) {
      unsub();
    }
    this.unsubscribers = [];
  }

  protected async onTask(taskId: string): Promise<void> {
    const task = this.taskGraph.getTask(taskId);

    // Store the task interaction in this agent's memory partition
    this.memory.createNode({
      type: 'fact',
      label: `Handled: ${task.title}`,
      properties: {
        taskId: task.id,
        partition: this.identity.memoryPartition,
        behaviors: this.identity.behaviors,
      },
      source: `agent:${this.identity.id}`,
    });

    // Phase 1: Mark complete — in Phase 2+ this will route to LLM
    // with the agent's behaviors as system prompt context
    this.taskGraph.updateStatus(taskId, 'completed',
      `Handled by ${this.identity.name}`
    );
  }

  /**
   * Get this agent's behavioral rules (used as system prompt context in Phase 2+)
   */
  getBehaviors(): string[] {
    return [...this.identity.behaviors];
  }

  /**
   * Get memories scoped to this agent's partition
   */
  getMemories(limit?: number) {
    return this.memory.search({
      text: `agent:${this.identity.id}`,
      limit,
    });
  }

  async handleMessage(fromAgentId: string, message: unknown): Promise<void> {
    const msg = message as { type: string; data?: unknown };

    if (msg.type === 'query') {
      // Return this agent's memories and status
      const memories = this.getMemories(10);
      await this.sendMessage(fromAgentId, {
        type: 'query_response',
        agent: this.identity.name,
        behaviors: this.identity.behaviors,
        memories,
      });
    }
  }
}
