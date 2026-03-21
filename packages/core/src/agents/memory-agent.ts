/**
 * Memory Agent — Manages the knowledge graph
 *
 * Handles consolidation, detects contradictions, and manages
 * the graph on behalf of other agents.
 */

import { BaseAgent } from './index.js';
import type { AgentConfig } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { MemoryManager } from '../memory/index.js';

export class MemoryAgent extends BaseAgent {
  private memory: MemoryManager;

  constructor(bus: ModuleBus, taskGraph: TaskGraph, memory: MemoryManager) {
    const config: AgentConfig = {
      id: 'memory-agent',
      name: 'Memory Agent',
      type: 'core',
      description: 'Manages the knowledge graph, handles consolidation, detects contradictions',
      capabilities: ['memory_read', 'memory_write', 'memory_search', 'consolidation', 'contradiction_detection'],
    };
    super(config, bus, taskGraph);
    this.memory = memory;
  }

  protected async onStart(): Promise<void> {
    // Listen for memory-related requests from other agents
    this.bus.subscribe('memory.request', async (event) => {
      const req = event.payload as { action: string; params: any; replyTo: string };
      await this.handleMemoryRequest(req);
    });

    // Listen for task completions to extract memory
    this.bus.subscribe('task.status_changed', async (event) => {
      const { task, newStatus } = event.payload as any;
      if (newStatus === 'completed') {
        await this.extractMemoryFromTask(task);
      }
    });
  }

  protected async onStop(): Promise<void> {}

  protected async onTask(taskId: string): Promise<void> {
    const task = this.taskGraph.getTask(taskId);
    const text = task.description.toLowerCase();

    if (text.includes('search') || text.includes('find')) {
      const results = this.memory.search({ text: task.description });
      task.metadata.results = results;
      this.taskGraph.updateStatus(taskId, 'completed');
    } else if (text.includes('remember') || text.includes('store')) {
      this.memory.createNode({
        type: 'fact',
        label: task.description,
        source: `task:${taskId}`,
      });
      this.taskGraph.updateStatus(taskId, 'completed');
    } else if (text.includes('forget') || text.includes('delete')) {
      // Search and delete matching nodes
      const results = this.memory.search({ text: task.description, limit: 5 });
      for (const node of results.nodes) {
        this.memory.deleteNode(node.id);
      }
      this.taskGraph.updateStatus(taskId, 'completed');
    } else {
      this.taskGraph.updateStatus(taskId, 'completed', 'No memory action needed');
    }
  }

  private async handleMemoryRequest(req: { action: string; params: any; replyTo: string }): Promise<void> {
    let result: unknown;

    switch (req.action) {
      case 'search':
        result = this.memory.search(req.params);
        break;
      case 'create_node':
        result = this.memory.createNode(req.params);
        break;
      case 'traverse':
        result = this.memory.traverse(req.params.nodeId, req.params.depth);
        break;
      case 'stats':
        result = this.memory.stats();
        break;
      default:
        result = { error: `Unknown memory action: ${req.action}` };
    }

    await this.bus.publish('memory.response', 'memory-agent', {
      replyTo: req.replyTo,
      result,
    });
  }

  private async extractMemoryFromTask(task: any): Promise<void> {
    // Phase 1: Basic extraction — store task completion as a fact
    // Phase 2+: LLM-based extraction of entities, relationships, behaviors
    if (task.source === 'user_chat' && task.description.length > 20) {
      this.memory.createNode({
        type: 'fact',
        label: `Task completed: ${task.title}`,
        properties: {
          taskId: task.id,
          source: task.source,
          completedAt: new Date().toISOString(),
        },
        source: `task:${task.id}`,
      });
    }
  }

  async handleMessage(fromAgentId: string, message: unknown): Promise<void> {
    const msg = message as { type: string; query?: string };
    if (msg.type === 'search' && msg.query) {
      const results = this.memory.search({ text: msg.query });
      await this.sendMessage(fromAgentId, { type: 'search_results', results });
    }
  }
}
