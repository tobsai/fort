/**
 * Agent Framework — Registry, Lifecycle, and Inter-Agent Messaging
 *
 * Fort maintains persistent specialist agents that accumulate expertise.
 * Core agents ship with the system. Specialist agents are created for domains.
 */

import type { AgentConfig, AgentInfo, AgentStatus, AgentType } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { TaskGraph } from '../task-graph/index.js';

export abstract class BaseAgent {
  readonly config: AgentConfig;
  protected bus: ModuleBus;
  protected taskGraph: TaskGraph;
  private _status: AgentStatus = 'stopped';
  private _currentTaskId: string | null = null;
  private _startedAt: Date = new Date();
  private _taskCount: number = 0;
  private _errorCount: number = 0;

  constructor(config: AgentConfig, bus: ModuleBus, taskGraph: TaskGraph) {
    this.config = config;
    this.bus = bus;
    this.taskGraph = taskGraph;
  }

  get status(): AgentStatus {
    return this._status;
  }

  get info(): AgentInfo {
    return {
      config: this.config,
      status: this._status,
      currentTaskId: this._currentTaskId,
      startedAt: this._startedAt,
      taskCount: this._taskCount,
      errorCount: this._errorCount,
    };
  }

  async start(): Promise<void> {
    this._status = 'running';
    this._startedAt = new Date();
    await this.onStart();
    this.bus.publish('agent.started', this.config.id, { agent: this.config });
  }

  async stop(): Promise<void> {
    this._status = 'stopped';
    await this.onStop();
    this.bus.publish('agent.stopped', this.config.id, { agent: this.config });
  }

  async pause(): Promise<void> {
    this._status = 'paused';
    this.bus.publish('agent.paused', this.config.id, { agent: this.config });
  }

  async resume(): Promise<void> {
    this._status = 'running';
    this.bus.publish('agent.resumed', this.config.id, { agent: this.config });
  }

  async handleTask(taskId: string): Promise<void> {
    this._currentTaskId = taskId;
    this._taskCount++;

    try {
      this.taskGraph.updateStatus(taskId, 'in_progress');
      await this.onTask(taskId);
    } catch (err) {
      this._errorCount++;
      this._status = 'error';
      const message = err instanceof Error ? err.message : String(err);
      this.taskGraph.updateStatus(taskId, 'failed', message);
      this.bus.publish('agent.error', this.config.id, {
        agent: this.config,
        taskId,
        error: message,
      });
    } finally {
      this._currentTaskId = null;
    }
  }

  async sendMessage(targetAgentId: string, message: unknown): Promise<void> {
    await this.bus.publish('agent.message', this.config.id, {
      from: this.config.id,
      to: targetAgentId,
      message,
    });
  }

  protected abstract onStart(): Promise<void>;
  protected abstract onStop(): Promise<void>;
  protected abstract onTask(taskId: string): Promise<void>;
}

export class AgentRegistry {
  private agents: Map<string, BaseAgent> = new Map();
  private bus: ModuleBus;

  constructor(bus: ModuleBus) {
    this.bus = bus;

    // Route inter-agent messages
    this.bus.subscribe('agent.message', async (event) => {
      const payload = event.payload as { from: string; to: string; message: unknown };
      const target = this.agents.get(payload.to);
      if (target) {
        await target.handleMessage?.(payload.from, payload.message);
      }
    });
  }

  register(agent: BaseAgent): void {
    if (this.agents.has(agent.config.id)) {
      throw new Error(`Agent already registered: ${agent.config.id}`);
    }
    this.agents.set(agent.config.id, agent);
    this.bus.publish('agent.registered', 'agent-registry', { agent: agent.config });
  }

  unregister(agentId: string): void {
    const agent = this.agents.get(agentId);
    if (agent) {
      if (agent.status === 'running') {
        agent.stop();
      }
      this.agents.delete(agentId);
      this.bus.publish('agent.unregistered', 'agent-registry', { agentId });
    }
  }

  get(agentId: string): BaseAgent | undefined {
    return this.agents.get(agentId);
  }

  getAll(): BaseAgent[] {
    return Array.from(this.agents.values());
  }

  getByType(type: AgentType): BaseAgent[] {
    return this.getAll().filter((a) => a.config.type === type);
  }

  getRunning(): BaseAgent[] {
    return this.getAll().filter((a) => a.status === 'running');
  }

  async startAll(): Promise<void> {
    for (const agent of this.agents.values()) {
      if (agent.status !== 'running') {
        await agent.start();
      }
    }
  }

  async stopAll(): Promise<void> {
    for (const agent of this.agents.values()) {
      if (agent.status === 'running') {
        await agent.stop();
      }
    }
  }

  listInfo(): AgentInfo[] {
    return this.getAll().map((a) => a.info);
  }
}

// Extend BaseAgent interface for message handling
declare module './index.js' {
  interface BaseAgent {
    handleMessage?(fromAgentId: string, message: unknown): Promise<void>;
  }
}
