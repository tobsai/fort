/**
 * SPEC-010: Agent Handoff & Inter-Agent Messaging — Tests
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import Database from 'better-sqlite3';
import { ModuleBus } from '../module-bus/index.js';
import { TaskGraph, TaskStore } from '../task-graph/index.js';
import { AgentRegistry, BaseAgent } from '../agents/index.js';
import { createDelegateTool } from '../tools/delegate-tool.js';
import type { AgentConfig, Task } from '../types.js';

// ─── Helpers ────────────────────────────────────────────────────────────────

function makeDb(): InstanceType<typeof Database> {
  const db = new (Database as any)(':memory:') as InstanceType<typeof Database>;
  (db as any).pragma('journal_mode = WAL');
  return db;
}

function makeStore(db: InstanceType<typeof Database>): TaskStore {
  const store = new TaskStore(db);
  store.initSchema();
  return store;
}

function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    id: 'task-1',
    shortId: 'FORT-001',
    title: 'Test task',
    description: 'A test task',
    status: 'created',
    source: 'user_chat',
    assignedAgent: null,
    sourceAgentId: null,
    assignedTo: null,
    parentId: null,
    result: null,
    subtaskIds: [],
    threadId: null,
    createdAt: new Date('2026-01-01T10:00:00Z'),
    updatedAt: new Date('2026-01-01T10:00:00Z'),
    completedAt: null,
    metadata: {},
    ...overrides,
  };
}

/** A simple stub agent that immediately completes any task with a fixed result. */
class StubAgent extends BaseAgent {
  private result: string;
  private shouldFail: boolean;

  constructor(
    config: AgentConfig,
    bus: ModuleBus,
    taskGraph: TaskGraph,
    result = 'stub result',
    shouldFail = false,
  ) {
    super(config, bus, taskGraph);
    this.result = result;
    this.shouldFail = shouldFail;
  }

  protected async onStart(): Promise<void> {}
  protected async onStop(): Promise<void> {}

  protected async onTask(taskId: string): Promise<void> {
    if (this.shouldFail) {
      throw new Error('stub agent failure');
    }
    this.taskGraph.completeTask(taskId, this.result);
  }
}

function makeRegistry(bus: ModuleBus, taskGraph: TaskGraph): AgentRegistry {
  return new AgentRegistry(bus);
}

function makeAgent(
  id: string,
  bus: ModuleBus,
  taskGraph: TaskGraph,
  result = 'agent result',
  shouldFail = false,
): StubAgent {
  const config: AgentConfig = {
    id,
    name: `Agent ${id}`,
    type: 'specialist',
    description: 'Test agent',
    capabilities: [],
  };
  return new StubAgent(config, bus, taskGraph, result, shouldFail);
}

// ─── TaskStore tests ─────────────────────────────────────────────────────────

describe('TaskStore — sourceAgentId', () => {
  let db: InstanceType<typeof Database>;
  let store: TaskStore;

  beforeEach(() => {
    db = makeDb();
    store = makeStore(db);
  });

  it('persists and loads sourceAgentId', () => {
    const task = makeTask({ id: 'task-src', sourceAgentId: 'agent-alpha' });
    store.upsertTask(task);
    const loaded = store.loadAll();
    expect(loaded[0].sourceAgentId).toBe('agent-alpha');
  });

  it('stores null sourceAgentId for non-delegated tasks', () => {
    const task = makeTask({ id: 'task-no-src' });
    store.upsertTask(task);
    const loaded = store.loadAll();
    expect(loaded[0].sourceAgentId).toBeNull();
  });

  it('getSubtasks returns children by parentId', () => {
    const parent = makeTask({ id: 'parent-1' });
    const child1 = makeTask({ id: 'child-1', parentId: 'parent-1' });
    const child2 = makeTask({ id: 'child-2', parentId: 'parent-1' });
    const other = makeTask({ id: 'other-1' });

    store.upsertTask(parent);
    store.upsertTask(child1);
    store.upsertTask(child2);
    store.upsertTask(other);

    const subs = store.getSubtasks('parent-1');
    expect(subs).toHaveLength(2);
    expect(subs.map((t) => t.id).sort()).toEqual(['child-1', 'child-2']);
  });

  it('getSubtasks returns empty array when no children', () => {
    const task = makeTask({ id: 'lone-task' });
    store.upsertTask(task);
    expect(store.getSubtasks('lone-task')).toHaveLength(0);
  });
});

// ─── TaskGraph tests ─────────────────────────────────────────────────────────

describe('TaskGraph — sourceAgentId in createTask', () => {
  it('stores sourceAgentId on the created task', () => {
    const bus = new ModuleBus();
    const graph = new TaskGraph(bus);

    const task = graph.createTask({
      title: 'Delegated task',
      source: 'agent_delegation',
      sourceAgentId: 'orchestrator-agent',
      assignedAgent: 'specialist-agent',
    });

    expect(task.sourceAgentId).toBe('orchestrator-agent');
  });

  it('sourceAgentId defaults to null when not provided', () => {
    const bus = new ModuleBus();
    const graph = new TaskGraph(bus);

    const task = graph.createTask({ title: 'User task', source: 'user_chat' });
    expect(task.sourceAgentId).toBeNull();
  });

  it('getSubtasksFromStore returns children from DB', () => {
    const db = makeDb();
    const store = makeStore(db);
    const bus = new ModuleBus();
    const graph = new TaskGraph(bus, store);

    const parent = graph.createTask({ title: 'Parent', source: 'user_chat' });
    graph.createTask({ title: 'Child A', source: 'agent_delegation', parentId: parent.id });
    graph.createTask({ title: 'Child B', source: 'agent_delegation', parentId: parent.id });

    const subs = graph.getSubtasksFromStore(parent.id);
    expect(subs).toHaveLength(2);
  });
});

// ─── delegate-to-agent tool ───────────────────────────────────────────────────

describe('delegate-to-agent tool', () => {
  let bus: ModuleBus;
  let graph: TaskGraph;
  let registry: AgentRegistry;

  beforeEach(() => {
    bus = new ModuleBus();
    graph = new TaskGraph(bus);
    registry = new AgentRegistry(bus);
  });

  it('creates subtask with correct parentId and sourceAgentId', async () => {
    const agent = makeAgent('target-agent', bus, graph, 'result from target');
    await agent.start();
    registry.register(agent);

    const parentTask = graph.createTask({
      title: 'Parent task',
      source: 'user_chat',
      assignedAgent: 'caller-agent',
    });

    const tool = createDelegateTool(graph, bus, registry);
    const result = await tool.execute(
      {
        agentId: 'target-agent',
        taskTitle: 'Delegated subtask',
        taskDescription: 'Do some work',
        expectation: 'Return a summary',
      },
      { taskId: parentTask.id, agentId: 'caller-agent' },
    );

    expect(result.success).toBe(true);
    expect(result.output).toBe('result from target');

    // Find the subtask that was created
    const subtasks = graph.getSubtasks(parentTask.id);
    expect(subtasks).toHaveLength(1);
    expect(subtasks[0].parentId).toBe(parentTask.id);
    expect(subtasks[0].sourceAgentId).toBe('caller-agent');
    expect(subtasks[0].assignedAgent).toBe('target-agent');
    expect(subtasks[0].source).toBe('agent_delegation');
  });

  it('returns error when target agent does not exist', async () => {
    const tool = createDelegateTool(graph, bus, registry);
    const result = await tool.execute({
      agentId: 'nonexistent-agent',
      taskTitle: 'Task',
      taskDescription: 'desc',
      expectation: 'something',
    });

    expect(result.success).toBe(false);
    expect(result.error).toMatch(/not found/);
  });

  it('returns error when target agent is stopped', async () => {
    const agent = makeAgent('stopped-agent', bus, graph);
    // Do NOT start the agent — status is 'stopped'
    registry.register(agent);

    const tool = createDelegateTool(graph, bus, registry);
    const result = await tool.execute({
      agentId: 'stopped-agent',
      taskTitle: 'Task',
      taskDescription: 'desc',
      expectation: 'something',
    });

    expect(result.success).toBe(false);
    expect(result.error).toMatch(/stopped/);
  });

  it('returns error for invalid parameters', async () => {
    const tool = createDelegateTool(graph, bus, registry);
    const result = await tool.execute({ agentId: 'x' }); // missing required fields
    expect(result.success).toBe(false);
    expect(result.error).toMatch(/invalid parameters/);
  });

  it('detects circular delegation — same agent in chain', async () => {
    const agentA = makeAgent('agent-a', bus, graph);
    await agentA.start();
    registry.register(agentA);

    // Create a chain: task-root (assigned to agent-a) → task-mid (sourceAgentId: agent-b)
    const taskRoot = graph.createTask({
      title: 'Root',
      source: 'user_chat',
      assignedAgent: 'agent-a',
    });
    const taskMid = graph.createTask({
      title: 'Mid',
      source: 'agent_delegation',
      parentId: taskRoot.id,
      assignedAgent: 'agent-b',
      sourceAgentId: 'agent-a',
    });

    const tool = createDelegateTool(graph, bus, registry);
    // Try to delegate back to agent-a from taskMid — cycle!
    const result = await tool.execute(
      {
        agentId: 'agent-a',
        taskTitle: 'Cycle task',
        taskDescription: 'desc',
        expectation: 'something',
      },
      { taskId: taskMid.id, agentId: 'agent-b' },
    );

    expect(result.success).toBe(false);
    expect(result.error).toMatch(/circular delegation/i);
  });

  it('enforces max delegation depth of 5', async () => {
    const agent = makeAgent('deep-agent', bus, graph);
    await agent.start();
    registry.register(agent);

    // Build a chain of 5 tasks (root + 4 children)
    let currentId: string | undefined;
    for (let i = 0; i < 5; i++) {
      const t = graph.createTask({
        title: `Level ${i}`,
        source: i === 0 ? 'user_chat' : 'agent_delegation',
        parentId: currentId,
        assignedAgent: `agent-level-${i}`,
      });
      currentId = t.id;
    }

    const tool = createDelegateTool(graph, bus, registry);
    // Try to create a 6th level (depth > 5)
    const result = await tool.execute(
      {
        agentId: 'deep-agent',
        taskTitle: 'Too deep',
        taskDescription: 'desc',
        expectation: 'something',
      },
      { taskId: currentId!, agentId: 'agent-level-4' },
    );

    expect(result.success).toBe(false);
    expect(result.error).toMatch(/too deep/i);
  });

  it('returns error when subtask fails', async () => {
    const failingAgent = makeAgent('failing-agent', bus, graph, '', true);
    await failingAgent.start();
    registry.register(failingAgent);

    const parentTask = graph.createTask({ title: 'Parent', source: 'user_chat' });
    const tool = createDelegateTool(graph, bus, registry);

    const result = await tool.execute(
      {
        agentId: 'failing-agent',
        taskTitle: 'Will fail',
        taskDescription: 'desc',
        expectation: 'something',
      },
      { taskId: parentTask.id, agentId: 'caller' },
    );

    expect(result.success).toBe(false);
  });

  it('tool has correct name and tier', () => {
    const tool = createDelegateTool(graph, bus, registry);
    expect(tool.name).toBe('delegate-to-agent');
    expect(tool.tier).toBe(1);
  });
});

// ─── tasks.query subtasks (integration) ──────────────────────────────────────

describe('TaskGraph.getSubtasksFromStore', () => {
  it('returns empty array when no store configured', () => {
    const bus = new ModuleBus();
    const graph = new TaskGraph(bus); // no store
    expect(graph.getSubtasksFromStore('any-id')).toEqual([]);
  });

  it('returns subtasks from store for a given parent', () => {
    const db = makeDb();
    const store = makeStore(db);
    const bus = new ModuleBus();
    const graph = new TaskGraph(bus, store);

    const parent = graph.createTask({ title: 'Parent', source: 'user_chat' });
    const child = graph.createTask({
      title: 'Child',
      source: 'agent_delegation',
      parentId: parent.id,
      sourceAgentId: 'caller-agent',
    });

    const subs = graph.getSubtasksFromStore(parent.id);
    expect(subs).toHaveLength(1);
    expect(subs[0].id).toBe(child.id);
    expect(subs[0].sourceAgentId).toBe('caller-agent');
  });
});
