/**
 * Agent Factory — Create, persist, fork, and retire specialist agents
 *
 * Each agent gets its own directory:
 *   {agentsDir}/{agent-id}/
 *     identity.yaml   — Config (name, status, metadata)
 *     SOUL.md          — Personality, rules, voice, boundaries
 *     tools/           — Agent-specific tools (future)
 *
 * Backward compatible: loads legacy flat {agent-id}.yaml files too.
 */

import { readFileSync, writeFileSync, readdirSync, existsSync, mkdirSync, copyFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { parse as parseYaml, stringify as stringifyYaml } from 'yaml';
import type { SpecialistIdentity } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { MemoryManager } from '../memory/index.js';
import type { LLMClient } from '../llm/index.js';
import type { ToolRegistry } from '../tools/index.js';
import type { AgentRegistry } from './index.js';
import { SpecialistAgent } from './specialist.js';

export class AgentFactory {
  private agentsDir: string;
  private bus: ModuleBus;
  private taskGraph: TaskGraph;
  private memory: MemoryManager;
  private registry: AgentRegistry;
  private llm: LLMClient | null = null;
  private toolRegistry: ToolRegistry | null = null;

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
   * Attach an LLM client. Agents created after this call will use it.
   */
  setLLM(llm: LLMClient): void {
    this.llm = llm;
  }

  /**
   * Attach a tool registry. Agents created after this call can check available tools.
   */
  setToolRegistry(tools: ToolRegistry): void {
    this.toolRegistry = tools;
  }

  /**
   * Create a new specialist agent. Only name is required.
   */
  create(params: {
    name: string;
    description?: string;
    capabilities?: string[];
    behaviors?: string[];
    eventSubscriptions?: string[];
    createdBy?: string;
    emoji?: string;
    avatar?: string;
    goals?: string;
  }): SpecialistAgent {
    const id = this.slugify(params.name);

    if (this.registry.get(id)) {
      throw new Error(`Agent with id "${id}" already exists. Choose a different name.`);
    }

    const description = params.description ?? 'A Fort specialist agent.';

    const identity: SpecialistIdentity = {
      id,
      name: params.name,
      description,
      capabilities: params.capabilities ?? [],
      memoryPartition: id,
      behaviors: params.behaviors ?? [],
      eventSubscriptions: params.eventSubscriptions ?? [],
      createdAt: new Date().toISOString(),
      createdBy: params.createdBy ?? 'user',
      status: 'active',
      soulPath: 'SOUL.md',
      emoji: params.emoji,
      avatar: params.avatar,
    };

    // Create agent directory structure
    const agentDir = this.getAgentDir(id);
    mkdirSync(agentDir, { recursive: true });
    mkdirSync(join(agentDir, 'tools'), { recursive: true });

    // Write identity and SOUL.md
    this.saveIdentity(identity);
    writeFileSync(
      join(agentDir, 'SOUL.md'),
      this.generateDefaultSoul(params.name, description, params.goals),
      'utf-8',
    );

    // Create and register the agent
    const agent = new SpecialistAgent(identity, this.bus, this.taskGraph, this.memory, agentDir);
    if (this.llm) agent.setLLM(this.llm);
    if (this.toolRegistry) agent.setToolRegistry(this.toolRegistry);
    this.registry.register(agent);

    // Store create event in memory
    this.memory.createNode({
      type: 'fact',
      label: `Created specialist: ${identity.name}`,
      properties: {
        agentId: identity.id,
        event: 'agent_created',
      },
      source: 'agent-factory',
    });

    this.bus.publish('agent.created', 'agent-factory', { identity });

    return agent;
  }

  /**
   * Create from a YAML identity file.
   */
  createFromFile(filePath: string): SpecialistAgent {
    const raw = readFileSync(filePath, 'utf-8');
    const identity = parseYaml(raw) as SpecialistIdentity;

    if (!identity.id || !identity.name) {
      throw new Error('Identity file must have at least "id" and "name" fields');
    }

    // Apply defaults
    identity.description ??= 'A Fort specialist agent.';
    identity.capabilities ??= [];
    identity.memoryPartition ??= identity.id;
    identity.behaviors ??= [];
    identity.eventSubscriptions ??= [];
    identity.createdAt ??= new Date().toISOString();
    identity.createdBy ??= 'file';
    identity.status ??= 'active';
    identity.soulPath ??= 'SOUL.md';

    if (this.registry.get(identity.id)) {
      throw new Error(`Agent with id "${identity.id}" already exists`);
    }

    // Create directory structure
    const agentDir = this.getAgentDir(identity.id);
    mkdirSync(agentDir, { recursive: true });
    mkdirSync(join(agentDir, 'tools'), { recursive: true });

    this.saveIdentity(identity);

    // Create SOUL.md if it doesn't exist
    const soulPath = join(agentDir, 'SOUL.md');
    if (!existsSync(soulPath)) {
      writeFileSync(soulPath, this.generateDefaultSoul(identity.name, identity.description), 'utf-8');
    }

    const agent = new SpecialistAgent(identity, this.bus, this.taskGraph, this.memory, agentDir);
    if (this.llm) agent.setLLM(this.llm);
    if (this.toolRegistry) agent.setToolRegistry(this.toolRegistry);
    this.registry.register(agent);

    this.bus.publish('agent.created', 'agent-factory', { identity });
    return agent;
  }

  /**
   * Retire an agent — stops it and marks identity as retired.
   * Memory, history, and SOUL.md are all preserved.
   */
  retire(agentId: string, reason?: string): void {
    const agent = this.registry.get(agentId);
    if (!agent) throw new Error(`Agent not found: ${agentId}`);
    if (agent.config.type !== 'specialist') {
      throw new Error(`Cannot retire core agent: ${agentId}`);
    }

    const specialist = agent as SpecialistAgent;
    const identity = specialist.identity;

    identity.status = 'retired';
    this.saveIdentity(identity);

    this.registry.unregister(agentId);

    this.memory.createNode({
      type: 'fact',
      label: `Retired specialist: ${identity.name}`,
      properties: {
        agentId: identity.id,
        event: 'agent_retired',
        reason: reason ?? 'User requested',
      },
      source: 'agent-factory',
    });

    this.bus.publish('agent.retired', 'agent-factory', { agentId, reason });
  }

  /**
   * Fork an agent — create a new specialist based on an existing one.
   * The new agent inherits the parent's SOUL.md as a starting point.
   */
  fork(sourceAgentId: string, overrides: {
    name: string;
    description?: string;
  }): SpecialistAgent {
    const identity = this.loadIdentity(sourceAgentId);
    if (!identity) throw new Error(`Agent identity not found: ${sourceAgentId}`);

    const agent = this.create({
      name: overrides.name,
      description: overrides.description ?? identity.description,
      capabilities: [...identity.capabilities],
      behaviors: [...identity.behaviors],
      eventSubscriptions: [...identity.eventSubscriptions],
      createdBy: `forked:${sourceAgentId}`,
    });

    // Copy parent's SOUL.md to the forked agent
    const parentSoulPath = join(this.getAgentDir(sourceAgentId), 'SOUL.md');
    const forkSoulPath = join(agent.agentDir, 'SOUL.md');
    if (existsSync(parentSoulPath)) {
      copyFileSync(parentSoulPath, forkSoulPath);
    }

    return agent;
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

    const agentDir = this.getAgentDir(agentId);
    const agent = new SpecialistAgent(identity, this.bus, this.taskGraph, this.memory, agentDir);
    if (this.llm) agent.setLLM(this.llm);
    if (this.toolRegistry) agent.setToolRegistry(this.toolRegistry);
    this.registry.register(agent);

    this.bus.publish('agent.revived', 'agent-factory', { identity });
    return agent;
  }

  /**
   * Load all persisted specialist identities and start active ones.
   * Supports both directory-based and legacy flat-file agents.
   */
  async loadAll(): Promise<SpecialistAgent[]> {
    const agents: SpecialistAgent[] = [];

    if (!existsSync(this.agentsDir)) return agents;

    const entries = readdirSync(this.agentsDir);

    for (const entry of entries) {
      const entryPath = join(this.agentsDir, entry);
      let identity: SpecialistIdentity | null = null;
      let agentDir: string;

      if (statSync(entryPath).isDirectory()) {
        // Directory-based agent: {id}/identity.yaml
        const identityPath = join(entryPath, 'identity.yaml');
        identity = this.loadIdentityFromFile(identityPath);
        agentDir = entryPath;
      } else if (entry.endsWith('.yaml')) {
        // Legacy flat-file agent: {id}.yaml
        identity = this.loadIdentityFromFile(entryPath);
        agentDir = this.agentsDir;
      } else {
        continue;
      }

      if (!identity || identity.status !== 'active') continue;
      if (this.registry.get(identity.id)) continue;

      const agent = new SpecialistAgent(identity, this.bus, this.taskGraph, this.memory, agentDir);
      if (this.llm) agent.setLLM(this.llm);
      if (this.toolRegistry) agent.setToolRegistry(this.toolRegistry);
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

    const entries = readdirSync(this.agentsDir);
    const identities: SpecialistIdentity[] = [];

    for (const entry of entries) {
      const entryPath = join(this.agentsDir, entry);
      let identity: SpecialistIdentity | null = null;

      if (statSync(entryPath).isDirectory()) {
        identity = this.loadIdentityFromFile(join(entryPath, 'identity.yaml'));
      } else if (entry.endsWith('.yaml')) {
        identity = this.loadIdentityFromFile(entryPath);
      }

      if (identity) identities.push(identity);
    }

    return identities.sort((a, b) => a.name.localeCompare(b.name));
  }

  /**
   * Get the SOUL.md contents for an agent.
   */
  getSoul(agentId: string): string | null {
    const soulPath = join(this.getAgentDir(agentId), 'SOUL.md');
    if (!existsSync(soulPath)) return null;
    return readFileSync(soulPath, 'utf-8');
  }

  /**
   * Get the directory path for an agent.
   */
  getAgentDir(agentId: string): string {
    return join(this.agentsDir, agentId);
  }

  // ─── Persistence ──────────────────────────────────────────────

  private saveIdentity(identity: SpecialistIdentity): void {
    const agentDir = this.getAgentDir(identity.id);
    // Directory-based: write to {id}/identity.yaml
    if (existsSync(agentDir) && statSync(agentDir).isDirectory()) {
      writeFileSync(join(agentDir, 'identity.yaml'), stringifyYaml(identity), 'utf-8');
    } else {
      // Legacy fallback: flat file
      writeFileSync(join(this.agentsDir, `${identity.id}.yaml`), stringifyYaml(identity), 'utf-8');
    }
  }

  private loadIdentity(agentId: string): SpecialistIdentity | null {
    // Try directory-based first
    const dirPath = join(this.getAgentDir(agentId), 'identity.yaml');
    const result = this.loadIdentityFromFile(dirPath);
    if (result) return result;

    // Fall back to legacy flat file
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

  private generateDefaultSoul(name: string, description: string, goals?: string): string {
    const goalsSection = goals
      ? `## Goals\n${goals}\n`
      : '## Goals\n<!-- What should this agent help you accomplish? -->\n';

    return `# ${name}

${description}

${goalsSection}
## Personality
<!-- How should this agent communicate? What's its tone and style? -->

## Rules
<!-- What rules must this agent always follow? -->

## Voice
<!-- Formal? Casual? Technical? Friendly? -->

## Boundaries
<!-- What should this agent refuse to do or stay away from? -->

## Knowledge
<!-- What domain knowledge does this agent specialize in? -->
`;
  }

  private slugify(name: string): string {
    return name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '');
  }
}
