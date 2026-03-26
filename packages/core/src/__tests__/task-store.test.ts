import { describe, it, expect, beforeEach } from 'vitest';
import Database from 'better-sqlite3';
import { TaskStore } from '../task-graph/task-store.js';
import { TaskGraph } from '../task-graph/index.js';
import { ModuleBus } from '../module-bus/index.js';
import type { Task } from '../types.js';

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
    assignedTo: null,
    parentId: null,
    sourceAgentId: null,
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

describe('TaskStore', () => {
  let db: InstanceType<typeof Database>;
  let store: TaskStore;

  beforeEach(() => {
    db = makeDb();
    store = makeStore(db);
  });

  it('upsertTask writes to DB', () => {
    const task = makeTask();
    store.upsertTask(task);

    const loaded = store.loadAll();
    expect(loaded).toHaveLength(1);
    expect(loaded[0].id).toBe('task-1');
    expect(loaded[0].title).toBe('Test task');
  });

  it('upsertTask updates existing row (replace semantics)', () => {
    const task = makeTask();
    store.upsertTask(task);

    const updated = { ...task, status: 'completed' as const, result: 'Done!' };
    store.upsertTask(updated);

    const loaded = store.loadAll();
    expect(loaded).toHaveLength(1);
    expect(loaded[0].status).toBe('completed');
    expect(loaded[0].result).toBe('Done!');
  });

  it('loadAll resets in_progress → pending', () => {
    const task = makeTask({ status: 'in_progress' });
    store.upsertTask(task);

    const loaded = store.loadAll();
    expect(loaded[0].status).toBe('pending');
  });

  it('loadAll preserves completed tasks as-is', () => {
    const task = makeTask({ status: 'completed', completedAt: new Date() });
    store.upsertTask(task);

    const loaded = store.loadAll();
    expect(loaded[0].status).toBe('completed');
  });

  it('queryTasks filters by status (string)', () => {
    store.upsertTask(makeTask({ id: 't1', status: 'created' }));
    store.upsertTask(makeTask({ id: 't2', status: 'completed' }));

    const results = store.queryTasks({ status: 'completed' });
    expect(results).toHaveLength(1);
    expect(results[0].id).toBe('t2');
  });

  it('queryTasks filters by status (array)', () => {
    store.upsertTask(makeTask({ id: 't1', status: 'created' }));
    store.upsertTask(makeTask({ id: 't2', status: 'completed' }));
    store.upsertTask(makeTask({ id: 't3', status: 'failed' }));

    const results = store.queryTasks({ status: ['completed', 'failed'] });
    expect(results).toHaveLength(2);
  });

  it('queryTasks filters by assignedAgent', () => {
    store.upsertTask(makeTask({ id: 't1', assignedAgent: 'agent-alpha' }));
    store.upsertTask(makeTask({ id: 't2', assignedAgent: 'agent-beta' }));

    const results = store.queryTasks({ assignedAgent: 'agent-alpha' });
    expect(results).toHaveLength(1);
    expect(results[0].id).toBe('t1');
  });

  it('queryTasks filters by since', () => {
    store.upsertTask(makeTask({ id: 't1', createdAt: new Date('2026-01-01T00:00:00Z'), updatedAt: new Date('2026-01-01') }));
    store.upsertTask(makeTask({ id: 't2', createdAt: new Date('2026-02-01T00:00:00Z'), updatedAt: new Date('2026-02-01') }));

    const results = store.queryTasks({ since: new Date('2026-01-15T00:00:00Z') });
    expect(results).toHaveLength(1);
    expect(results[0].id).toBe('t2');
  });

  it('queryTasks respects limit and offset', () => {
    for (let i = 1; i <= 5; i++) {
      store.upsertTask(makeTask({ id: `t${i}`, createdAt: new Date(`2026-01-0${i}T00:00:00Z`), updatedAt: new Date(`2026-01-0${i}`) }));
    }

    const page1 = store.queryTasks({ limit: 2, offset: 0 });
    const page2 = store.queryTasks({ limit: 2, offset: 2 });
    expect(page1).toHaveLength(2);
    expect(page2).toHaveLength(2);
    expect(page1[0].id).not.toBe(page2[0].id);
  });

  it('preserves metadata JSON round-trip', () => {
    const task = makeTask({ metadata: { key: 'value', num: 42, nested: { a: true } } });
    store.upsertTask(task);

    const loaded = store.loadAll();
    expect(loaded[0].metadata).toEqual({ key: 'value', num: 42, nested: { a: true } });
  });

  it('preserves subtaskIds JSON round-trip', () => {
    const task = makeTask({ subtaskIds: ['child-1', 'child-2'] });
    store.upsertTask(task);

    const loaded = store.loadAll();
    expect(loaded[0].subtaskIds).toEqual(['child-1', 'child-2']);
  });
});

// ─── TaskGraph integration with TaskStore ────────────────────────────────────

describe('TaskGraph + TaskStore persistence', () => {
  function makeGraphWithStore() {
    const db = makeDb();
    const store = makeStore(db);
    const bus = new ModuleBus();
    const graph = new TaskGraph(bus, store);
    return { db, store, bus, graph };
  }

  it('createTask writes to DB', () => {
    const { store, graph } = makeGraphWithStore();
    graph.createTask({ title: 'Persist me', source: 'user_chat' });

    const loaded = store.loadAll();
    expect(loaded).toHaveLength(1);
    expect(loaded[0].title).toBe('Persist me');
  });

  it('updateStatus writes to DB', () => {
    const { store, graph } = makeGraphWithStore();
    const task = graph.createTask({ title: 'Status test', source: 'user_chat' });
    graph.updateStatus(task.id, 'in_progress');

    const loaded = store.queryTasks({ status: 'in_progress' });
    expect(loaded).toHaveLength(1);
  });

  it('tasks survive TaskGraph restart (hydration from DB)', () => {
    const db = makeDb();
    const store1 = makeStore(db);
    const bus1 = new ModuleBus();
    const graph1 = new TaskGraph(bus1, store1);

    graph1.createTask({ title: 'Task A', source: 'user_chat' });
    graph1.createTask({ title: 'Task B', source: 'user_chat' });

    // Simulate restart: new graph, same DB
    const store2 = new TaskStore(db);
    store2.initSchema();
    const bus2 = new ModuleBus();
    const graph2 = new TaskGraph(bus2, store2);
    graph2.loadFromStore();

    expect(graph2.getAllTasks()).toHaveLength(2);
    const titles = graph2.getAllTasks().map((t) => t.title);
    expect(titles).toContain('Task A');
    expect(titles).toContain('Task B');
  });

  it('in_progress tasks reset to pending on restart', () => {
    const db = makeDb();
    const store1 = makeStore(db);
    const bus1 = new ModuleBus();
    const graph1 = new TaskGraph(bus1, store1);

    const task = graph1.createTask({ title: 'In flight', source: 'user_chat' });
    graph1.updateStatus(task.id, 'in_progress');

    // Restart
    const store2 = new TaskStore(db);
    store2.initSchema();
    const bus2 = new ModuleBus();
    const graph2 = new TaskGraph(bus2, store2);
    graph2.loadFromStore();

    const rehydrated = graph2.getTask(task.id);
    expect(rehydrated.status).toBe('pending');
  });

  it('taskCounter restored after loadFromStore (no shortId collisions)', () => {
    const db = makeDb();
    const store1 = makeStore(db);
    const bus1 = new ModuleBus();
    const graph1 = new TaskGraph(bus1, store1);

    graph1.createTask({ title: 'First', source: 'user_chat' });
    graph1.createTask({ title: 'Second', source: 'user_chat' });

    const store2 = new TaskStore(db);
    store2.initSchema();
    const bus2 = new ModuleBus();
    const graph2 = new TaskGraph(bus2, store2);
    graph2.loadFromStore();

    const newTask = graph2.createTask({ title: 'Third', source: 'user_chat' });
    expect(newTask.shortId).toBe('FORT-003');
  });

  it('queryTasksFromStore delegates to store', () => {
    const { graph } = makeGraphWithStore();
    const t1 = graph.createTask({ title: 'Alpha', source: 'user_chat' });
    graph.updateStatus(t1.id, 'completed');
    graph.createTask({ title: 'Beta', source: 'user_chat' });

    const completed = graph.queryTasksFromStore({ status: 'completed' });
    expect(completed).toHaveLength(1);
    expect(completed[0].title).toBe('Alpha');
  });

  it('creates 1000 tasks in under 500ms', () => {
    const { graph } = makeGraphWithStore();
    const start = Date.now();
    for (let i = 0; i < 1000; i++) {
      graph.createTask({ title: `Task ${i}`, source: 'user_chat' });
    }
    const elapsed = Date.now() - start;
    expect(elapsed).toBeLessThan(500);
  });
});
