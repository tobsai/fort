import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import Database from 'better-sqlite3';
import { GoalsStore } from '../services/goals-store.js';
import { GoalsService } from '../services/goals.js';
import { ModuleBus } from '../module-bus/index.js';
import type { Goal } from '../types.js';

describe('GoalsService + GoalsStore', () => {
  let tmpDir: string;
  let db: InstanceType<typeof Database>;
  let bus: ModuleBus;
  let store: GoalsStore;
  let service: GoalsService;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-goals-'));
    db = new (Database as any)(join(tmpDir, 'goals.db'));
    (db as any).pragma('journal_mode = WAL');
    store = new GoalsStore(db);
    store.initSchema();
    bus = new ModuleBus();
    service = new GoalsService(store, bus);
  });

  afterEach(() => {
    (db as any).close?.();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('creates a goal with defaults', () => {
    const goal = service.create({ agentId: 'agent-1', title: 'Ship Fort v1' });
    expect(goal.id).toBeTruthy();
    expect(goal.title).toBe('Ship Fort v1');
    expect(goal.status).toBe('active');
    expect(goal.source).toBe('user');
    expect(goal.lastActivityAt).toBeInstanceOf(Date);
  });

  it('lists active goals for an agent by default', () => {
    service.create({ agentId: 'a', title: 'one' });
    service.create({ agentId: 'a', title: 'two' });
    service.create({ agentId: 'b', title: 'other agent' });
    const active = service.listForAgent('a');
    expect(active.map((g) => g.title).sort()).toEqual(['one', 'two']);
  });

  it('listForAgent excludes non-active goals when status defaults to active', () => {
    const g = service.create({ agentId: 'a', title: 'one' });
    service.update(g.id, { status: 'paused' });
    expect(service.listForAgent('a')).toHaveLength(0);
    expect(service.listAll('a')).toHaveLength(1);
  });

  it('update merges fields and bumps updatedAt', async () => {
    const g = service.create({ agentId: 'a', title: 'one' });
    await new Promise((r) => setTimeout(r, 5));
    const updated = service.update(g.id, { description: 'longer text' });
    expect(updated?.description).toBe('longer text');
    expect(updated?.title).toBe('one'); // preserved
    expect(updated?.updatedAt.getTime()).toBeGreaterThan(g.updatedAt.getTime());
  });

  it('achieve marks goal as achieved', () => {
    const g = service.create({ agentId: 'a', title: 'one' });
    const done = service.achieve(g.id);
    expect(done?.status).toBe('achieved');
    expect(service.listForAgent('a')).toHaveLength(0);
  });

  it('delete removes a goal', () => {
    const g = service.create({ agentId: 'a', title: 'one' });
    expect(service.delete(g.id)).toBe(true);
    expect(service.get(g.id)).toBeNull();
    expect(service.delete(g.id)).toBe(false); // second delete is no-op
  });

  it('publishes bus events on lifecycle changes', async () => {
    const events: Array<{ type: string; payload: unknown }> = [];
    bus.subscribe('goal:created', (e) => { events.push({ type: e.type, payload: e.payload }); });
    bus.subscribe('goal:updated', (e) => { events.push({ type: e.type, payload: e.payload }); });
    bus.subscribe('goal:deleted', (e) => { events.push({ type: e.type, payload: e.payload }); });

    const g = service.create({ agentId: 'a', title: 'one' });
    service.update(g.id, { title: 'two' });
    service.delete(g.id);

    // ModuleBus publish is async; give it a tick
    await new Promise((r) => setTimeout(r, 5));
    expect(events.map((e) => e.type)).toEqual(['goal:created', 'goal:updated', 'goal:deleted']);
  });

  it('recordActivity advances lastActivityAt', async () => {
    const g = service.create({ agentId: 'a', title: 'one' });
    const first = g.lastActivityAt!.getTime();
    await new Promise((r) => setTimeout(r, 10));
    service.recordActivity(g.id);
    const refreshed = service.get(g.id)!;
    expect(refreshed.lastActivityAt!.getTime()).toBeGreaterThan(first);
  });

  it('persists across new store instances on the same db', () => {
    service.create({ agentId: 'a', title: 'persisted' });
    const store2 = new GoalsStore(db);
    store2.initSchema(); // idempotent
    const service2 = new GoalsService(store2, new ModuleBus());
    expect(service2.listForAgent('a').map((g) => g.title)).toEqual(['persisted']);
  });
});
