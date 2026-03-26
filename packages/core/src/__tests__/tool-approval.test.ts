import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import Database from 'better-sqlite3';
import { ApprovalStore } from '../tools/approval-store.js';
import { ToolExecutor, ToolRejectedError } from '../tools/executor.js';
import { ModuleBus } from '../module-bus/index.js';
import type { FortTool, ToolResult } from '../tools/types.js';
import type { PermissionManager } from '../permissions/index.js';
import type { TokenTracker } from '../tokens/index.js';

// ── Helpers ──────────────────────────────────────────────────────────

function makeDb(): Database.Database {
  const db = new Database(':memory:');
  db.pragma('journal_mode = WAL');
  return db;
}

function makeStore(db: Database.Database): ApprovalStore {
  const store = new ApprovalStore(db);
  store.initSchema();
  return store;
}

function makeTool(tier: 1 | 2 | 3 | 4, name = 'test-tool'): FortTool {
  return {
    name,
    description: 'A test tool',
    inputSchema: {},
    tier,
    execute: async (_input: unknown): Promise<ToolResult> => ({
      success: true,
      output: 'executed',
    }),
  };
}

function makePermissions(): PermissionManager {
  return {} as unknown as PermissionManager;
}

function makeTokens(): TokenTracker {
  return {
    record: vi.fn().mockResolvedValue(undefined),
  } as unknown as TokenTracker;
}

function makeExecutor(store?: ApprovalStore): { executor: ToolExecutor; bus: ModuleBus } {
  const bus = new ModuleBus();
  const executor = new ToolExecutor(makePermissions(), bus, makeTokens());
  if (store) {
    executor.setApprovalStore(store);
  }
  return { executor, bus };
}

// ── ApprovalStore tests ───────────────────────────────────────────────

describe('ApprovalStore', () => {
  let db: Database.Database;
  let store: ApprovalStore;

  beforeEach(() => {
    db = makeDb();
    store = makeStore(db);
  });

  afterEach(() => {
    db.close();
  });

  it('create() writes a pending record and returns ApprovalRequest', () => {
    const req = store.create({
      taskId: 'task-1',
      agentId: 'agent-1',
      toolName: 'delete-file',
      toolTier: 3,
      parameters: { path: '/tmp/foo' },
    });

    expect(req.id).toBeTruthy();
    expect(req.status).toBe('pending');
    expect(req.toolName).toBe('delete-file');
    expect(req.toolTier).toBe(3);
    expect(req.parameters).toEqual({ path: '/tmp/foo' });
    expect(req.rejectionReason).toBeNull();
    expect(req.resolvedAt).toBeNull();
    expect(req.createdAt).toBeInstanceOf(Date);
  });

  it('getPending() returns only pending records ordered by createdAt', () => {
    store.create({ taskId: 't1', agentId: 'a1', toolName: 'tool-a', toolTier: 2, parameters: {} });
    const r2 = store.create({ taskId: 't2', agentId: 'a1', toolName: 'tool-b', toolTier: 3, parameters: {} });
    const r3 = store.create({ taskId: 't3', agentId: 'a1', toolName: 'tool-c', toolTier: 2, parameters: {} });

    // Resolve r2
    store.resolve(r2.id, true);

    const pending = store.getPending();
    expect(pending).toHaveLength(2);
    expect(pending.map((p) => p.toolName)).toEqual(['tool-a', 'tool-c']);
    expect(pending.find((p) => p.id === r3.id)).toBeTruthy();
  });

  it('resolve() with approved=true sets status to approved', () => {
    const req = store.create({ taskId: 't1', agentId: 'a1', toolName: 'write-file', toolTier: 2, parameters: {} });
    const resolved = store.resolve(req.id, true);

    expect(resolved.status).toBe('approved');
    expect(resolved.resolvedAt).toBeInstanceOf(Date);
    expect(resolved.rejectionReason).toBeNull();
    expect(store.getPending()).toHaveLength(0);
  });

  it('resolve() with approved=false sets status to rejected with reason', () => {
    const req = store.create({ taskId: 't1', agentId: 'a1', toolName: 'rm-rf', toolTier: 3, parameters: {} });
    const resolved = store.resolve(req.id, false, 'Too dangerous');

    expect(resolved.status).toBe('rejected');
    expect(resolved.rejectionReason).toBe('Too dangerous');
    expect(resolved.resolvedAt).toBeInstanceOf(Date);
  });

  it('resolve() throws when id not found', () => {
    expect(() => store.resolve('nonexistent-id', true)).toThrow('not found');
  });

  it('getForTask() returns all approvals for a task', () => {
    store.create({ taskId: 'task-x', agentId: 'a1', toolName: 'tool-1', toolTier: 2, parameters: {} });
    store.create({ taskId: 'task-x', agentId: 'a1', toolName: 'tool-2', toolTier: 3, parameters: {} });
    store.create({ taskId: 'task-y', agentId: 'a1', toolName: 'tool-3', toolTier: 2, parameters: {} });

    const forX = store.getForTask('task-x');
    expect(forX).toHaveLength(2);
    expect(forX.map((r) => r.toolName)).toEqual(['tool-1', 'tool-2']);

    const forY = store.getForTask('task-y');
    expect(forY).toHaveLength(1);
  });
});

// ── ToolExecutor approval integration tests ──────────────────────────

describe('ToolExecutor — Tier 1 bypass', () => {
  it('executes Tier 1 tool immediately without creating approval', async () => {
    const db = makeDb();
    const store = makeStore(db);
    const { executor } = makeExecutor(store);
    const tool = makeTool(1);

    const result = await executor.execute(tool, {});

    expect(result.success).toBe(true);
    expect(result.output).toBe('executed');
    expect(store.getPending()).toHaveLength(0);

    db.close();
  });
});

describe('ToolExecutor — Tier 4 deny', () => {
  it('denies Tier 4 tools always', async () => {
    const db = makeDb();
    const store = makeStore(db);
    const { executor } = makeExecutor(store);
    const tool = makeTool(4);

    const result = await executor.execute(tool, {});

    expect(result.success).toBe(false);
    expect(result.error).toContain('tier 4');
    expect(store.getPending()).toHaveLength(0);

    db.close();
  });
});

describe('ToolExecutor — Tier 2/3 approval flow', () => {
  let db: Database.Database;
  let store: ApprovalStore;
  let executor: ToolExecutor;
  let bus: ModuleBus;

  beforeEach(() => {
    db = makeDb();
    store = makeStore(db);
    ({ executor, bus } = makeExecutor(store));
  });

  afterEach(() => {
    db.close();
  });

  it('creates an ApprovalRequest for Tier 2 tool and blocks', async () => {
    const tool = makeTool(2, 'write-draft');
    const events: unknown[] = [];

    bus.subscribe('approval.required', (e) => { events.push(e.payload); });

    const executePromise = executor.execute(tool, { content: 'hello' }, { taskId: 't1', agentId: 'a1' });

    // Give the event loop a tick to publish the event
    await new Promise((r) => setTimeout(r, 10));

    expect(store.getPending()).toHaveLength(1);
    const pending = store.getPending()[0];
    expect(pending.toolName).toBe('write-draft');
    expect(pending.status).toBe('pending');
    expect(events).toHaveLength(1);
    expect((events[0] as any).toolName).toBe('write-draft');

    // Resolve it
    store.resolve(pending.id, true);
    executor.resolveApproval(pending.id, true);

    const result = await executePromise;
    expect(result.success).toBe(true);
    expect(result.output).toBe('executed');
  });

  it('creates an ApprovalRequest for Tier 3 tool and blocks', async () => {
    const tool = makeTool(3, 'delete-file');
    const events: unknown[] = [];

    bus.subscribe('approval.required', (e) => { events.push(e.payload); });

    const executePromise = executor.execute(tool, { path: '/tmp/x' }, { taskId: 't2', agentId: 'a2' });

    await new Promise((r) => setTimeout(r, 10));

    expect(store.getPending()).toHaveLength(1);
    const pending = store.getPending()[0];
    expect(pending.toolTier).toBe(3);
    expect((events[0] as any).tier).toBe(3);

    store.resolve(pending.id, true);
    executor.resolveApproval(pending.id, true);

    const result = await executePromise;
    expect(result.success).toBe(true);
  });

  it('rejection throws ToolRejectedError and returns failed ToolResult', async () => {
    const tool = makeTool(3, 'dangerous-op');

    const executePromise = executor.execute(tool, {}, { taskId: 't3', agentId: 'a3' });

    await new Promise((r) => setTimeout(r, 10));

    const pending = store.getPending()[0];
    store.resolve(pending.id, false, 'Not allowed');
    executor.resolveApproval(pending.id, false, 'Not allowed');

    const result = await executePromise;
    expect(result.success).toBe(false);
    expect(result.error).toContain('rejected');
    expect(result.error).toContain('Not allowed');
  });

  it('timeout rejects automatically', async () => {
    // Patch the timeout on the executor's private awaitApproval
    // We do this by using fake timers
    vi.useFakeTimers();

    const tool = makeTool(2, 'slow-approval');

    const executePromise = executor.execute(tool, {}, { taskId: 't4', agentId: 'a4' });

    await Promise.resolve(); // flush microtasks

    // Fast-forward 10 minutes + 1s
    vi.advanceTimersByTime(601_000);

    const result = await executePromise;
    expect(result.success).toBe(false);
    expect(result.error).toContain('timed out');

    vi.useRealTimers();
  });

  it('Tier 3 without approvalStore falls back to denial', async () => {
    const { executor: execNoStore } = makeExecutor(); // no store
    const tool = makeTool(3, 'no-store-tool');

    const result = await execNoStore.execute(tool, {});
    expect(result.success).toBe(false);
    expect(result.error).toContain('tier 3');
  });
});

describe('ToolRejectedError', () => {
  it('is an instance of Error with name ToolRejectedError', () => {
    const err = new ToolRejectedError('my-tool', 'bad idea');
    expect(err).toBeInstanceOf(Error);
    expect(err.name).toBe('ToolRejectedError');
    expect(err.message).toContain('my-tool');
    expect(err.message).toContain('bad idea');
  });
});
