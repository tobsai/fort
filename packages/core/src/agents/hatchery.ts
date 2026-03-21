/**
 * Agent Hatchery — Create, persist, fork, and retire specialist agents
 *
 * The hatchery manages the full lifecycle of specialist agents:
 * - hatch: Create a new specialist from an identity definition
 * - retire: Deactivate an agent (preserving its memory and history)
 * - fork: Clone an agent with modifications (new identity, shared lineage)
 * - revive: Bring a retired agent back to active status
 *
 * Identity files are persisted as YAML in the agents directory.
 */

import { readFileSync, writeFileSync, readdirSync, existsSync, mkdirSync, renameSync } from 'node:fs';
import { join, basename } from 'node:path';
import { parse as parseYaml, stringify as stringifyYaml } from 'yaml';
import { v4 as uuid } from 'uuid';
import type { SpecialistIdentity } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { MemoryManager } from '../memory/index.js';
import type { AgentRegistry } from './index.js';
import { SpecialistAgent } from './specialist.js';

export class AgentHatchery {
  private agentsDir: string;
  private bus: ModuleBus;
  private taskGraph: TaskGraph;
  private memory: MemoryManager;
  private registry: AgentRegistry;

  constructor(
    agentsDir: string,
    bus: ModuleBus,
    taskGraph: TaskGraph,
    memory: MemoryManager,
    registry: AgentRegistry,
  ) {
    this.agentsDir = agentsDir;
    this.bus = bus;
    this.taskGraph = taskGraph;
    this.memory = memory;
    this.registry = registry;

    if (!existsSync(agentsDir)) {
      mkdirSync(agentsDir, { recursive: true });
    }
  }

  /**
   * Hatch a new specialist agent from parameters or identity definition.
   */
  hatch(params: {
    name: string;
    description: string;
    capabilities?: string[];
    behaviors?: string[];
    eventSubscriptions?: string[];
    createdBy?: string;
  }): SpecialistAgent {
    const id = this.slugify(params.name);

    if (this.registry.get(id)) {
      throw new Error(`Agent with id "${id}" already exists. Choose a different name.`);
    }

    const identity: SpecialistIdentity = {
      id,
      name: params.name,
      description: params.description,
      capabilities: params.capabilities ?? [],
      memoryPartition: id,
      behaviors: params.behaviors ?? [],
      eventSubscriptions: params.eventSubscriptions ?? [],
      createdAt: new Date().toISOString(),
      createdBy: params.createdBy ?? 'user',
      status: 'active',
    };

    // Persist identity file
    this.saveIdentity(identity);

    // Create and register the agent
    const agent = new SpecialistAgent(identity, this.bus, this.taskGraph, this.memory);
    this.registry.register(agent);

    // Store hatch event in memory
    this.memory.createNode({
      type: 'fact',
      label: `Hatched specialist: ${identity.name}`,
      properties: {
        agentId: identity.id,
        event: 'agent_hatched',
        capabilities: identity.capabilities,
        behaviors: identity.behaviors,
      },
      source: 'hatchery',
    });

    this.bus.publish('agent.hatched', 'hatchery', { identity });

    return agent;
  }

  /**
   * Hatch from a YAML identity file.
   */
  hatchFromFile(filePath: string): SpecialistAgent {
    const raw = readFileSync(filePath, 'utf-8');
    const identity = parseYaml(raw) as SpecialistIdentity;

    if (!identity.id || !identity.name) {
      throw new Error('Identity file must have at least "id" and "name" fields');
    }

    // Apply defaults
    identity.capabilities ??= [];
    identity.memoryPartition ??= identity.id;
    identity.behaviors ??= [];
    identity.eventSubscriptions ??= [];
    identity.createdAt ??= new Date().toISOString();
    identity.createdBy ??= 'file';
    identity.status ??= 'active';

    if (this.registry.get(identity.id)) {
      throw new Error(`Agent with id "${identity.id}" already exists`);
    }

    this.saveIdentity(identity);

    const agent = new SpecialistAgent(identity, this.bus, this.taskGraph, this.memory);
    this.registry.register(agent);

    this.bus.publish('agent.hatched', 'hatchery', { identity });
    return agent;
  }

  /**
   * Retire an agent — stops it and marks identity as retired.
   * Memory and history are preserved.
   */
  retire(agentId: string, reason?: string): void {
    const agent = this.registry.get(agentId);
    if (!agent) throw new Error(`Agent not found: ${agentId}`);
    if (agent.config.type !== 'specialist') {
      throw new Error(`Cannot retire core agent: ${agentId}`);
    }

    const specialist = agent as SpecialistAgent;
    const identity = specialist.identity;

    // Update identity status
    identity.status = 'retired';
    this.saveIdentity(identity);

    // Unregister (stops the agent)
    this.registry.unregister(agentId);

    // Record retirement
    this.memory.createNode({
      type: 'fact',
      label: `Retired specialist: ${identity.name}`,
      properties: {
        agentId: identity.id,
        event: 'agent_retired',
        reason: reason ?? 'User requested',
      },
      source: 'hatchery',
    });

    this.bus.publish('agent.retired', 'hatchery', { agentId, reason });
  }

  /**
   * Fork an agent — create a new specialist based on an existing one.
   * The new agent gets its own identity and memory partition but
   * inherits behaviors and capabilities from the parent.
   */
  fork(sourceAgentId: string, overrides: {
    name: string;
    description?: string;
    capabilities?: string[];
    behaviors?: string[];
  }): SpecialistAgent {
    const identity = this.loadIdentity(sourceAgentId);
    if (!identity) throw new Error(`Agent identity not found: ${sourceAgentId}`);

    return this.hatch({
      name: overrides.name,
      description: overrides.description ?? identity.description,
      capabilities: overrides.capabilities ?? [...identity.capabilities],
      behaviors: overrides.behaviors ?? [...identity.behaviors],
      eventSubscriptions: [...identity.eventSubscriptions],
      createdBy: `forked:${sourceAgentId}`,
    });
  }

  /**
   * Revive a retired agent — reload from identity file and re-register.
   */
  revive(agentId: string): SpecialistAgent {
    const identity = this.loadIdentity(agentId);
    if (!identity) throw new Error(`Agent identity not found: ${agentId}`);

    if (this.registry.get(agentId)) {
      throw new Error(`Agent "${agentId}" is already active`);
    }

    identity.status = 'active';
    this.saveIdentity(identity);

    const agent = new SpecialistAgent(identity, this.bus, this.taskGraph, this.memory);
    this.registry.register(agent);

    this.bus.publish('agent.revived', 'hatchery', { identity });
    return agent;
  }

  /**
   * Load all persisted specialist identities and start active ones.
   */
  async loadAll(): Promise<SpecialistAgent[]> {
    const agents: SpecialistAgent[] = [];

    if (!existsSync(this.agentsDir)) return agents;

    const files = readdirSync(this.agentsDir).filter((f) => f.endsWith('.yaml'));

    for (const file of files) {
      const identity = this.loadIdentityFromFile(join(this.agentsDir, file));
      if (!identity || identity.status !== 'active') continue;

      if (this.registry.get(identity.id)) continue; // already loaded

      const agent = new SpecialistAgent(identity, this.bus, this.taskGraph, this.memory);
      this.registry.register(agent);
      agents.push(agent);
    }

    return agents;
  }

  /**
   * List all known specialist identities (active and retired).
   */
  listIdentities(): SpecialistIdentity[] {
    if (!existsSync(this.agentsDir)) return [];

    const files = readdirSync(this.agentsDir).filter((f) => f.endsWith('.yaml'));
    const identities: SpecialistIdentity[] = [];

    for (const file of files) {
      const identity = this.loadIdentityFromFile(join(this.agentsDir, file));
      if (identity) identities.push(identity);
    }

    return identities.sort((a, b) => a.name.localeCompare(b.name));
  }

  // ─── Persistence ──────────────────────────────────────────────

  private saveIdentity(identity: SpecialistIdentity): void {
    const filePath = join(this.agentsDir, `${identity.id}.yaml`);
    writeFileSync(filePath, stringifyYaml(identity), 'utf-8');
  }

  private loadIdentity(agentId: string): SpecialistIdentity | null {
    return this.loadIdentityFromFile(join(this.agentsDir, `${agentId}.yaml`));
  }

  private loadIdentityFromFile(filePath: string): SpecialistIdentity | null {
    if (!existsSync(filePath)) return null;
    try {
      return parseYaml(readFileSync(filePath, 'utf-8')) as SpecialistIdentity;
    } catch {
      return null;
    }
  }

  private slugify(name: string): string {
    return name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '');
  }
}
