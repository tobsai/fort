import { describe, it, expect, beforeEach } from 'vitest';
import Database from 'better-sqlite3';
import { UsageStore } from '../usage/store.js';
import type { UsageEvent } from '../usage/store.js';

function makeDb(): InstanceType<typeof Database> {
  const db = new (Database as any)(':memory:') as InstanceType<typeof Database>;
  (db as any).pragma('journal_mode = WAL');
  return db;
}

function makeStore(db: InstanceType<typeof Database>): UsageStore {
  const store = new UsageStore(db);
  store.initSchema();
  store.seedDefaultPricing();
  return store;
}

function makeEvent(overrides: Partial<UsageEvent> = {}): UsageEvent {
  return {
    taskId: 'task-1',
    agentId: 'agent-1',
    model: 'claude-sonnet-4-5-20250929',
    inputTokens: 1000,
    outputTokens: 500,
    cacheReadTokens: 0,
    cacheWriteTokens: 0,
    ...overrides,
  };
}

describe('UsageStore', () => {
  let db: InstanceType<typeof Database>;
  let store: UsageStore;

  beforeEach(() => {
    db = makeDb();
    store = makeStore(db);
  });

  // ─── Schema ───────────────────────────────────────────────────────────────

  it('creates usage_events and model_pricing tables', () => {
    const tables = (db as any).prepare(
      "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"
    ).all().map((r: any) => r.name) as string[];
    expect(tables).toContain('usage_events');
    expect(tables).toContain('model_pricing');
  });

  it('seeds default pricing rows', () => {
    const row = (db as any).prepare("SELECT * FROM model_pricing WHERE model = 'claude-sonnet-4-5-20250929'").get() as any;
    expect(row).toBeTruthy();
    expect(row.input_per_million).toBe(3.00);
    expect(row.output_per_million).toBe(15.00);
  });

  // ─── recordUsage ──────────────────────────────────────────────────────────

  it('recordUsage writes a row to usage_events', () => {
    store.recordUsage(makeEvent());
    const rows = (db as any).prepare('SELECT * FROM usage_events').all() as any[];
    expect(rows).toHaveLength(1);
    expect(rows[0].agent_id).toBe('agent-1');
    expect(rows[0].input_tokens).toBe(1000);
    expect(rows[0].output_tokens).toBe(500);
  });

  it('recordUsage calculates total_tokens via generated column', () => {
    store.recordUsage(makeEvent({ inputTokens: 100, outputTokens: 200, cacheReadTokens: 50, cacheWriteTokens: 25 }));
    const row = (db as any).prepare('SELECT total_tokens FROM usage_events').get() as any;
    expect(row.total_tokens).toBe(375);
  });

  it('recordUsage calculates estimated_cost_usd correctly', () => {
    // sonnet: $3/M input, $15/M output
    // 1000 input + 500 output = 1000*3/1M + 500*15/1M = 0.003 + 0.0075 = 0.0105
    store.recordUsage(makeEvent({ inputTokens: 1000, outputTokens: 500, model: 'claude-sonnet-4-5-20250929' }));
    const row = (db as any).prepare('SELECT estimated_cost_usd FROM usage_events').get() as any;
    expect(row.estimated_cost_usd).toBeCloseTo(0.0105, 6);
  });

  it('recordUsage sets estimated_cost_usd to null for unknown model', () => {
    store.recordUsage(makeEvent({ model: 'unknown-model-xyz' }));
    const row = (db as any).prepare('SELECT estimated_cost_usd FROM usage_events').get() as any;
    expect(row.estimated_cost_usd).toBeNull();
  });

  it('recordUsage stores cache tokens', () => {
    store.recordUsage(makeEvent({ cacheReadTokens: 200, cacheWriteTokens: 100 }));
    const row = (db as any).prepare('SELECT cache_read_tokens, cache_write_tokens FROM usage_events').get() as any;
    expect(row.cache_read_tokens).toBe(200);
    expect(row.cache_write_tokens).toBe(100);
  });

  // ─── queryUsage ───────────────────────────────────────────────────────────

  it('queryUsage returns all records when no filters', () => {
    store.recordUsage(makeEvent({ agentId: 'agent-1' }));
    store.recordUsage(makeEvent({ agentId: 'agent-2' }));
    const records = store.queryUsage();
    expect(records).toHaveLength(2);
  });

  it('queryUsage filters by agentId', () => {
    store.recordUsage(makeEvent({ agentId: 'agent-1' }));
    store.recordUsage(makeEvent({ agentId: 'agent-2' }));
    const records = store.queryUsage({ agentId: 'agent-1' });
    expect(records).toHaveLength(1);
    expect(records[0].agentId).toBe('agent-1');
  });

  it('queryUsage filters by taskId', () => {
    store.recordUsage(makeEvent({ taskId: 'task-A' }));
    store.recordUsage(makeEvent({ taskId: 'task-B' }));
    const records = store.queryUsage({ taskId: 'task-A' });
    expect(records).toHaveLength(1);
    expect(records[0].taskId).toBe('task-A');
  });

  it('queryUsage respects limit', () => {
    for (let i = 0; i < 5; i++) {
      store.recordUsage(makeEvent({ taskId: `task-${i}` }));
    }
    const records = store.queryUsage({ limit: 3 });
    expect(records).toHaveLength(3);
  });

  // ─── getSummary ───────────────────────────────────────────────────────────

  it('getSummary returns correct aggregates', () => {
    store.recordUsage(makeEvent({ inputTokens: 100, outputTokens: 200 }));
    store.recordUsage(makeEvent({ inputTokens: 300, outputTokens: 400 }));
    const summary = store.getSummary('month');
    expect(summary.totalInputTokens).toBe(400);
    expect(summary.totalOutputTokens).toBe(600);
    expect(summary.totalTokens).toBe(1000);
    expect(summary.eventCount).toBe(2);
    expect(summary.period).toBe('month');
  });

  it('getSummary filters by agentId', () => {
    store.recordUsage(makeEvent({ agentId: 'agent-1', inputTokens: 100, outputTokens: 100 }));
    store.recordUsage(makeEvent({ agentId: 'agent-2', inputTokens: 500, outputTokens: 500 }));
    const summary = store.getSummary('month', 'agent-1');
    expect(summary.eventCount).toBe(1);
    expect(summary.totalInputTokens).toBe(100);
  });

  it('getSummary returns zeros when no data in period', () => {
    const summary = store.getSummary('day');
    expect(summary.totalTokens).toBe(0);
    expect(summary.estimatedCostUsd).toBe(0);
    expect(summary.eventCount).toBe(0);
  });

  // ─── getByAgent ───────────────────────────────────────────────────────────

  it('getByAgent returns agents ranked by total tokens', () => {
    store.recordUsage(makeEvent({ agentId: 'cheap', inputTokens: 100, outputTokens: 100 }));
    store.recordUsage(makeEvent({ agentId: 'heavy', inputTokens: 5000, outputTokens: 5000 }));
    const byAgent = store.getByAgent('month');
    expect(byAgent[0].agentId).toBe('heavy');
    expect(byAgent[1].agentId).toBe('cheap');
  });

  it('getByAgent returns empty array when no data', () => {
    const byAgent = store.getByAgent('day');
    expect(byAgent).toHaveLength(0);
  });

  // ─── getTotals ────────────────────────────────────────────────────────────

  it('getTotals returns all-time aggregates', () => {
    store.recordUsage(makeEvent({ agentId: 'a1', model: 'claude-sonnet-4-5-20250929', inputTokens: 100, outputTokens: 200 }));
    store.recordUsage(makeEvent({ agentId: 'a2', model: 'claude-opus-4-6', inputTokens: 300, outputTokens: 400 }));
    const totals = store.getTotals();
    expect(totals.totalInputTokens).toBe(400);
    expect(totals.totalOutputTokens).toBe(600);
    expect(totals.eventCount).toBe(2);
    expect(totals.uniqueAgents).toBe(2);
    expect(totals.uniqueModels).toBe(2);
  });

  it('getTotals returns zeros when store is empty', () => {
    const totals = store.getTotals();
    expect(totals.totalTokens).toBe(0);
    expect(totals.eventCount).toBe(0);
    expect(totals.uniqueAgents).toBe(0);
  });

  // ─── Model Pricing ────────────────────────────────────────────────────────

  it('getPricing returns null for unknown model', () => {
    expect(store.getPricing('not-a-real-model')).toBeNull();
  });

  it('getPricing returns correct rates for known model', () => {
    const pricing = store.getPricing('claude-opus-4-6');
    expect(pricing).not.toBeNull();
    expect(pricing!.inputPerMillion).toBe(15.00);
    expect(pricing!.outputPerMillion).toBe(75.00);
  });

  it('calculateCost handles cache tokens', () => {
    const pricing = store.getPricing('claude-sonnet-4-5-20250929')!;
    const cost = store.calculateCost(
      { taskId: 't', agentId: 'a', model: 'claude-sonnet-4-5-20250929', inputTokens: 0, outputTokens: 0, cacheReadTokens: 1_000_000, cacheWriteTokens: 0 },
      pricing,
    );
    // cache read: 1M * 0.30/M = 0.30
    expect(cost).toBeCloseTo(0.30, 6);
  });
});
