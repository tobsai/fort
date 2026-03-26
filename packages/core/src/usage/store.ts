/**
 * UsageStore — SQLite persistence for LLM token usage events
 *
 * Tracks per-task, per-agent token counts and estimated costs.
 * Schema: usage_events + model_pricing tables.
 */

import { randomUUID } from 'node:crypto';
import type Database from 'better-sqlite3';

// ─── Types ──────────────────────────────────────────────────────────────────

export interface UsageEvent {
  taskId: string;
  agentId: string;
  model: string;
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens?: number;
  cacheWriteTokens?: number;
}

export interface UsageRecord extends UsageEvent {
  id: string;
  totalTokens: number;
  estimatedCostUsd: number | null;
  createdAt: string;
}

export interface ModelPricing {
  model: string;
  inputPerMillion: number;
  outputPerMillion: number;
  cacheReadPerMillion: number;
  cacheWritePerMillion: number;
}

export interface UsageQuery {
  agentId?: string;
  taskId?: string;
  since?: string;
  until?: string;
  model?: string;
  limit?: number;
  offset?: number;
}

export interface UsageSummary {
  period: 'day' | 'week' | 'month';
  agentId?: string;
  totalInputTokens: number;
  totalOutputTokens: number;
  totalCacheReadTokens: number;
  totalCacheWriteTokens: number;
  totalTokens: number;
  estimatedCostUsd: number;
  eventCount: number;
}

export interface AgentUsage {
  agentId: string;
  totalTokens: number;
  totalInputTokens: number;
  totalOutputTokens: number;
  estimatedCostUsd: number;
  eventCount: number;
}

export interface UsageTotals {
  totalInputTokens: number;
  totalOutputTokens: number;
  totalCacheReadTokens: number;
  totalCacheWriteTokens: number;
  totalTokens: number;
  estimatedCostUsd: number;
  eventCount: number;
  uniqueAgents: number;
  uniqueModels: number;
}

// ─── Default Pricing (per 1M tokens) ────────────────────────────────────────

const DEFAULT_PRICING: ModelPricing[] = [
  { model: 'claude-opus-4-5', inputPerMillion: 15.00, outputPerMillion: 75.00, cacheReadPerMillion: 1.50, cacheWritePerMillion: 18.75 },
  { model: 'claude-sonnet-4-5', inputPerMillion: 3.00, outputPerMillion: 15.00, cacheReadPerMillion: 0.30, cacheWritePerMillion: 3.75 },
  { model: 'claude-haiku-4-5', inputPerMillion: 0.80, outputPerMillion: 4.00, cacheReadPerMillion: 0.08, cacheWritePerMillion: 1.00 },
  // Versioned model IDs used internally
  { model: 'claude-opus-4-6', inputPerMillion: 15.00, outputPerMillion: 75.00, cacheReadPerMillion: 1.50, cacheWritePerMillion: 18.75 },
  { model: 'claude-sonnet-4-5-20250929', inputPerMillion: 3.00, outputPerMillion: 15.00, cacheReadPerMillion: 0.30, cacheWritePerMillion: 3.75 },
  { model: 'claude-haiku-4-5-20251001', inputPerMillion: 0.80, outputPerMillion: 4.00, cacheReadPerMillion: 0.08, cacheWritePerMillion: 1.00 },
  { model: 'gpt-4o', inputPerMillion: 5.00, outputPerMillion: 15.00, cacheReadPerMillion: 0, cacheWritePerMillion: 0 },
  { model: 'gpt-4o-mini', inputPerMillion: 0.15, outputPerMillion: 0.60, cacheReadPerMillion: 0, cacheWritePerMillion: 0 },
  { model: 'llama-3.3-70b-versatile', inputPerMillion: 0.59, outputPerMillion: 0.79, cacheReadPerMillion: 0, cacheWritePerMillion: 0 },
];

// ─── UsageStore ──────────────────────────────────────────────────────────────

export class UsageStore {
  private db: InstanceType<typeof Database>;

  constructor(db: InstanceType<typeof Database>) {
    this.db = db;
  }

  initSchema(): void {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS usage_events (
        id TEXT PRIMARY KEY,
        task_id TEXT NOT NULL,
        agent_id TEXT NOT NULL,
        model TEXT NOT NULL,
        input_tokens INTEGER DEFAULT 0,
        output_tokens INTEGER DEFAULT 0,
        cache_read_tokens INTEGER DEFAULT 0,
        cache_write_tokens INTEGER DEFAULT 0,
        total_tokens INTEGER GENERATED ALWAYS AS (input_tokens + output_tokens + cache_read_tokens + cache_write_tokens) STORED,
        estimated_cost_usd REAL,
        created_at TEXT DEFAULT (datetime('now'))
      );

      CREATE INDEX IF NOT EXISTS idx_usage_agent ON usage_events(agent_id);
      CREATE INDEX IF NOT EXISTS idx_usage_task ON usage_events(task_id);
      CREATE INDEX IF NOT EXISTS idx_usage_date ON usage_events(created_at);

      CREATE TABLE IF NOT EXISTS model_pricing (
        model TEXT PRIMARY KEY,
        input_per_million REAL NOT NULL,
        output_per_million REAL NOT NULL,
        cache_read_per_million REAL DEFAULT 0,
        cache_write_per_million REAL DEFAULT 0,
        updated_at TEXT DEFAULT (datetime('now'))
      );
    `);
  }

  seedDefaultPricing(): void {
    const stmt = this.db.prepare(`
      INSERT OR IGNORE INTO model_pricing
        (model, input_per_million, output_per_million, cache_read_per_million, cache_write_per_million)
      VALUES (?, ?, ?, ?, ?)
    `);
    for (const p of DEFAULT_PRICING) {
      stmt.run(p.model, p.inputPerMillion, p.outputPerMillion, p.cacheReadPerMillion, p.cacheWritePerMillion);
    }
  }

  getPricing(model: string): ModelPricing | null {
    const row = (this.db as any).prepare('SELECT * FROM model_pricing WHERE model = ?').get(model) as any;
    if (!row) return null;
    return {
      model: row.model,
      inputPerMillion: row.input_per_million,
      outputPerMillion: row.output_per_million,
      cacheReadPerMillion: row.cache_read_per_million,
      cacheWritePerMillion: row.cache_write_per_million,
    };
  }

  calculateCost(event: UsageEvent, pricing: ModelPricing): number {
    const { inputTokens, outputTokens, cacheReadTokens = 0, cacheWriteTokens = 0 } = event;
    return (
      (inputTokens * pricing.inputPerMillion +
        outputTokens * pricing.outputPerMillion +
        cacheReadTokens * pricing.cacheReadPerMillion +
        cacheWriteTokens * pricing.cacheWritePerMillion) /
      1_000_000
    );
  }

  recordUsage(event: UsageEvent): void {
    const pricing = this.getPricing(event.model);
    const cost = pricing ? this.calculateCost(event, pricing) : null;

    (this.db as any).prepare(`
      INSERT INTO usage_events
        (id, task_id, agent_id, model, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, estimated_cost_usd)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run(
      randomUUID(),
      event.taskId,
      event.agentId,
      event.model,
      event.inputTokens,
      event.outputTokens,
      event.cacheReadTokens ?? 0,
      event.cacheWriteTokens ?? 0,
      cost,
    );
  }

  queryUsage(query: UsageQuery = {}): UsageRecord[] {
    const conditions: string[] = [];
    const params: unknown[] = [];

    if (query.agentId) { conditions.push('agent_id = ?'); params.push(query.agentId); }
    if (query.taskId) { conditions.push('task_id = ?'); params.push(query.taskId); }
    if (query.model) { conditions.push('model = ?'); params.push(query.model); }
    if (query.since) { conditions.push('created_at >= ?'); params.push(query.since); }
    if (query.until) { conditions.push('created_at <= ?'); params.push(query.until); }

    const where = conditions.length > 0 ? `WHERE ${conditions.join(' AND ')}` : '';
    const limit = query.limit ?? 100;
    const offset = query.offset ?? 0;

    const rows = (this.db as any).prepare(`
      SELECT id, task_id, agent_id, model, input_tokens, output_tokens,
             cache_read_tokens, cache_write_tokens, total_tokens, estimated_cost_usd, created_at
      FROM usage_events
      ${where}
      ORDER BY created_at DESC
      LIMIT ? OFFSET ?
    `).all(...params, limit, offset) as any[];

    return rows.map(this.rowToRecord);
  }

  getSummary(period: 'day' | 'week' | 'month', agentId?: string): UsageSummary {
    const since = this.periodToSince(period);
    const conditions: string[] = ['created_at >= ?'];
    const params: unknown[] = [since];

    if (agentId) { conditions.push('agent_id = ?'); params.push(agentId); }

    const where = `WHERE ${conditions.join(' AND ')}`;

    const row = (this.db as any).prepare(`
      SELECT
        COALESCE(SUM(input_tokens), 0) AS total_input,
        COALESCE(SUM(output_tokens), 0) AS total_output,
        COALESCE(SUM(cache_read_tokens), 0) AS total_cache_read,
        COALESCE(SUM(cache_write_tokens), 0) AS total_cache_write,
        COALESCE(SUM(total_tokens), 0) AS total_tokens,
        COALESCE(SUM(estimated_cost_usd), 0) AS total_cost,
        COUNT(*) AS event_count
      FROM usage_events
      ${where}
    `).get(...params) as any;

    return {
      period,
      agentId,
      totalInputTokens: row.total_input,
      totalOutputTokens: row.total_output,
      totalCacheReadTokens: row.total_cache_read,
      totalCacheWriteTokens: row.total_cache_write,
      totalTokens: row.total_tokens,
      estimatedCostUsd: row.total_cost,
      eventCount: row.event_count,
    };
  }

  getByAgent(period: 'day' | 'week' | 'month'): AgentUsage[] {
    const since = this.periodToSince(period);

    const rows = (this.db as any).prepare(`
      SELECT
        agent_id,
        COALESCE(SUM(total_tokens), 0) AS total_tokens,
        COALESCE(SUM(input_tokens), 0) AS total_input,
        COALESCE(SUM(output_tokens), 0) AS total_output,
        COALESCE(SUM(estimated_cost_usd), 0) AS total_cost,
        COUNT(*) AS event_count
      FROM usage_events
      WHERE created_at >= ?
      GROUP BY agent_id
      ORDER BY total_tokens DESC
    `).all(since) as any[];

    return rows.map((r) => ({
      agentId: r.agent_id,
      totalTokens: r.total_tokens,
      totalInputTokens: r.total_input,
      totalOutputTokens: r.total_output,
      estimatedCostUsd: r.total_cost,
      eventCount: r.event_count,
    }));
  }

  getTotals(): UsageTotals {
    const row = (this.db as any).prepare(`
      SELECT
        COALESCE(SUM(input_tokens), 0) AS total_input,
        COALESCE(SUM(output_tokens), 0) AS total_output,
        COALESCE(SUM(cache_read_tokens), 0) AS total_cache_read,
        COALESCE(SUM(cache_write_tokens), 0) AS total_cache_write,
        COALESCE(SUM(total_tokens), 0) AS total_tokens,
        COALESCE(SUM(estimated_cost_usd), 0) AS total_cost,
        COUNT(*) AS event_count,
        COUNT(DISTINCT agent_id) AS unique_agents,
        COUNT(DISTINCT model) AS unique_models
      FROM usage_events
    `).get() as any;

    return {
      totalInputTokens: row.total_input,
      totalOutputTokens: row.total_output,
      totalCacheReadTokens: row.total_cache_read,
      totalCacheWriteTokens: row.total_cache_write,
      totalTokens: row.total_tokens,
      estimatedCostUsd: row.total_cost,
      eventCount: row.event_count,
      uniqueAgents: row.unique_agents,
      uniqueModels: row.unique_models,
    };
  }

  // ─── Private ──────────────────────────────────────────────────────────────

  private rowToRecord(row: any): UsageRecord {
    return {
      id: row.id,
      taskId: row.task_id,
      agentId: row.agent_id,
      model: row.model,
      inputTokens: row.input_tokens,
      outputTokens: row.output_tokens,
      cacheReadTokens: row.cache_read_tokens,
      cacheWriteTokens: row.cache_write_tokens,
      totalTokens: row.total_tokens,
      estimatedCostUsd: row.estimated_cost_usd,
      createdAt: row.created_at,
    };
  }

  private periodToSince(period: 'day' | 'week' | 'month'): string {
    const now = new Date();
    switch (period) {
      case 'day':
        now.setDate(now.getDate() - 1);
        break;
      case 'week':
        now.setDate(now.getDate() - 7);
        break;
      case 'month':
        now.setMonth(now.getMonth() - 1);
        break;
    }
    return now.toISOString().replace('T', ' ').slice(0, 19);
  }
}
