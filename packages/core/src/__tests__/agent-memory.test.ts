import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import Database from 'better-sqlite3';
import { AgentMemoryStore } from '../memory/agent-memory-store.js';
import { ModuleBus } from '../module-bus/index.js';
import { TaskGraph } from '../task-graph/index.js';
import { OrchestratorService } from '../services/orchestrator.js';
import { AgentRegistry, BaseAgent } from '../agents/index.js';
import type { AgentConfig, AgentType } from '../types.js';

describe('AgentMemoryStore', () => {
  let tmpDir: string;
  let db: InstanceType<typeof Database>;
  let store: AgentMemoryStore;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-agent-memory-'));
    db = new (Database as any)(join(tmpDir, 'test.db'));
    store = new AgentMemoryStore(db);
    store.initSchema();
  });

  afterEach(() => {
    db.close();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('remember() writes to DB', () => {
    const memory = store.remember('agent-1', {
      category: 'fact',
      content: 'The sky is blue',
      tags: ['color', 'sky'],
    });

    expect(memory.id).toBeDefined();
    expect(memory.agentId).toBe('agent-1');
    expect(memory.category).toBe('fact');
    expect(memory.content).toBe('The sky is blue');
    expect(memory.tags).toEqual(['color', 'sky']);
    expect(memory.taskId).toBeNull();
    expect(memory.createdAt).toBeDefined();
  });

  it('remember() stores taskId when provided', () => {
    const memory = store.remember('agent-1', {
      category: 'decision',
      content: 'Use PostgreSQL for all new services',
      taskId: 'task-abc',
    });

    expect(memory.taskId).toBe('task-abc');
  });

  it('list() returns memories for an agent', () => {
    store.remember('agent-1', { category: 'fact', content: 'Fact one' });
    store.remember('agent-1', { category: 'decision', content: 'Decision one' });
    store.remember('agent-2', { category: 'fact', content: 'Other agent fact' });

    const list = store.list('agent-1');
    expect(list).toHaveLength(2);
    expect(list.every((m) => m.agentId === 'agent-1')).toBe(true);
  });

  it('list() respects category filter', () => {
    store.remember('agent-1', { category: 'fact', content: 'A fact' });
    store.remember('agent-1', { category: 'decision', content: 'A decision' });
    store.remember('agent-1', { category: 'preference', content: 'A preference' });

    const facts = store.list('agent-1', { category: 'fact' });
    expect(facts).toHaveLength(1);
    expect(facts[0].category).toBe('fact');
  });

  it('recall() matches by keyword via LIKE fallback', () => {
    store.remember('agent-1', { category: 'fact', content: 'PostgreSQL is the preferred database' });
    store.remember('agent-1', { category: 'fact', content: 'Redis is used for caching' });
    store.remember('agent-1', { category: 'fact', content: 'Node.js is the runtime' });

    const results = store.recall('agent-1', 'PostgreSQL');
    expect(results.length).toBeGreaterThanOrEqual(1);
    expect(results.some((m) => m.content.includes('PostgreSQL'))).toBe(true);
  });

  it('recall() respects category filter', () => {
    store.remember('agent-1', { category: 'fact', content: 'database fact' });
    store.remember('agent-1', { category: 'decision', content: 'database decision' });

    const results = store.recall('agent-1', 'database', { category: 'fact' });
    expect(results.every((m) => m.category === 'fact')).toBe(true);
  });

  it('recall() returns empty array when no match', () => {
    store.remember('agent-1', { category: 'fact', content: 'something completely unrelated' });

    const results = store.recall('agent-1', 'xyznomatch12345unique');
    expect(results).toHaveLength(0);
  });

  it('recall() does not return memories for other agents', () => {
    store.remember('agent-1', { category: 'fact', content: 'shared keyword test' });
    store.remember('agent-2', { category: 'fact', content: 'shared keyword test too' });

    const results = store.recall('agent-1', 'keyword');
    expect(results.every((m) => m.agentId === 'agent-1')).toBe(true);
  });

  it('delete() removes entry', () => {
    const m = store.remember('agent-1', { category: 'fact', content: 'to be deleted' });
    expect(store.list('agent-1')).toHaveLength(1);

    store.delete(m.id);
    expect(store.list('agent-1')).toHaveLength(0);
  });

  it('clear() removes all entries for an agent', () => {
    store.remember('agent-1', { category: 'fact', content: 'fact 1' });
    store.remember('agent-1', { category: 'fact', content: 'fact 2' });
    store.remember('agent-2', { category: 'fact', content: 'other agent' });

    store.clear('agent-1');
    expect(store.list('agent-1')).toHaveLength(0);
    // agent-2 memories unaffected
    expect(store.list('agent-2')).toHaveLength(1);
  });
});

/** Minimal concrete agent for testing — captures the task ID passed to onTask */
class TestAgent extends BaseAgent {
  capturedTaskId: string | null = null;

  constructor(id: string, bus: ModuleBus, taskGraph: TaskGraph) {
    const config: AgentConfig = {
      id,
      name: 'Test Agent',
      description: '',
      type: 'specialist',
      capabilities: [],
    };
    super(config, bus, taskGraph);
  }

  protected async onStart(): Promise<void> {}
  protected async onStop(): Promise<void> {}
  protected async onTask(taskId: string): Promise<void> {
    this.capturedTaskId = taskId;
  }
}

describe('OrchestratorService — auto-inject memory context', () => {
  let tmpDir: string;
  let db: InstanceType<typeof Database>;
  let agentMemory: AgentMemoryStore;
  let bus: ModuleBus;
  let taskGraph: TaskGraph;
  let orchestrator: OrchestratorService;
  let agents: AgentRegistry;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-orch-memory-'));
    db = new (Database as any)(join(tmpDir, 'test.db'));
    agentMemory = new AgentMemoryStore(db);
    agentMemory.initSchema();
    bus = new ModuleBus();
    taskGraph = new TaskGraph(bus);
    agents = new AgentRegistry(bus);
    orchestrator = new OrchestratorService(taskGraph, agents, bus);
    orchestrator.setAgentMemoryStore(agentMemory);
  });

  afterEach(() => {
    db.close();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('auto-inject prepends memory context in task metadata when relevant memories exist', async () => {
    const agentId = 'agent-orch-1';

    // Pre-populate memory
    agentMemory.remember(agentId, {
      category: 'decision',
      content: 'Use PostgreSQL for all new services',
    });

    const agent = new TestAgent(agentId, bus, taskGraph);
    await agent.start();
    agents.register(agent);

    await orchestrator.routeChat('Tell me about PostgreSQL databases', 'user_chat', agentId);

    expect(agent.capturedTaskId).toBeDefined();
    const task = taskGraph.getTask(agent.capturedTaskId!);
    expect(task.metadata.memoryContext).toBeDefined();
    expect(task.metadata.memoryContext as string).toContain('[Memory] Relevant context');
    expect(task.metadata.memoryContext as string).toContain('PostgreSQL');
  });

  it('does not inject memoryContext when no relevant memories exist', async () => {
    const agentId = 'agent-orch-2';

    const agent = new TestAgent(agentId, bus, taskGraph);
    await agent.start();
    agents.register(agent);

    await orchestrator.routeChat('Hello world', 'user_chat', agentId);

    const task = taskGraph.getTask(agent.capturedTaskId!);
    expect(task.metadata.memoryContext).toBeUndefined();
  });
});
