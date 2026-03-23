/**
 * Token Tracker — LLM Usage Monitoring & Budget Enforcement
 *
 * Tracks every LLM call's token usage, calculates costs, enforces budgets,
 * and provides breakdowns by model, agent, and task. Subscribes to ModuleBus
 * 'llm.usage' events for automatic recording.
 */

import Database from 'better-sqlite3';
import { v4 as uuid } from 'uuid';
import { existsSync, mkdirSync } from 'node:fs';
import { dirname } from 'node:path';
import type { ModuleBus } from '../module-bus/index.js';
import type { TokenUsage, TokenBudget, TokenStats, DiagnosticResult } from '../types.js';

/** Per-model pricing in USD per token */
interface ModelPricing {
  inputPerToken: number;
  outputPerToken: number;
}

const DEFAULT_PRICING: Record<string, ModelPricing> = {
  'claude-sonnet-4-5-20250929': { inputPerToken: 3 / 1_000_000, outputPerToken: 15 / 1_000_000 },
  'claude-opus-4-6': { inputPerToken: 15 / 1_000_000, outputPerToken: 75 / 1_000_000 },
  'claude-haiku-4-5-20251001': { inputPerToken: 0.80 / 1_000_000, outputPerToken: 4 / 1_000_000 },
};

const DEFAULT_BUDGET: TokenBudget = {
  dailyLimit: 1_000_000,
  monthlyLimit: 20_000_000,
  perTaskLimit: 200_000,
  warningThreshold: 0.8,
};

export class TokenTracker {
  private db: Database.Database;
  private bus: ModuleBus | null;
  private budget: TokenBudget;
  private pricing: Record<string, ModelPricing>;
  private unsubscribe: (() => void) | null = null;

  constructor(
    dbPath: string,
    bus?: ModuleBus,
    budget?: Partial<TokenBudget>,
    pricing?: Record<string, ModelPricing>,
  ) {
    const dir = dirname(dbPath);
    if (!existsSync(dir)) mkdirSync(dir, { recursive: true });

    this.db = new Database(dbPath);
    this.db.pragma('journal_mode = WAL');
    this.initializeSchema();

    this.bus = bus ?? null;
    this.budget = { ...DEFAULT_BUDGET, ...budget };
    this.pricing = { ...DEFAULT_PRICING, ...pricing };

    if (this.bus) {
      this.unsubscribe = this.bus.subscribe('llm.usage', async (event) => {
        const payload = event.payload as Omit<TokenUsage, 'id'>;
        await this.record(payload);
      });
    }
  }

  private initializeSchema(): void {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS token_usage (
        id TEXT PRIMARY KEY,
        timestamp TEXT NOT NULL,
        model TEXT NOT NULL,
        input_tokens INTEGER NOT NULL,
        output_tokens INTEGER NOT NULL,
        total_tokens INTEGER NOT NULL,
        cost_usd REAL NOT NULL,
        task_id TEXT,
        agent_id TEXT,
        source TEXT NOT NULL
      );

      CREATE INDEX IF NOT EXISTS idx_token_usage_timestamp ON token_usage(timestamp);
      CREATE INDEX IF NOT EXISTS idx_token_usage_model ON token_usage(model);
      CREATE INDEX IF NOT EXISTS idx_token_usage_task_id ON token_usage(task_id);
      CREATE INDEX IF NOT EXISTS idx_token_usage_agent_id ON token_usage(agent_id);
    `);
  }

  /**
   * Calculate cost for a usage record, using the pricing table.
   * If costUsd is already provided and non-zero, it is used as-is.
   */
  private calculateCost(usage: Omit<TokenUsage, 'id'>): number {
    if (usage.costUsd > 0) return usage.costUsd;
    const pricing = this.pricing[usage.model];
    if (!pricing) return 0;
    return usage.inputTokens * pricing.inputPerToken + usage.outputTokens * pricing.outputPerToken;
  }

  async record(usage: Omit<TokenUsage, 'id'>): Promise<TokenUsage> {
    const id = uuid();
    const costUsd = this.calculateCost(usage);
    const totalTokens = usage.totalTokens || usage.inputTokens + usage.outputTokens;
    const timestamp = usage.timestamp instanceof Date ? usage.timestamp : new Date(usage.timestamp);

    this.db.prepare(`
      INSERT INTO token_usage (id, timestamp, model, input_tokens, output_tokens, total_tokens, cost_usd, task_id, agent_id, source)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run(
      id,
      timestamp.toISOString(),
      usage.model,
      usage.inputTokens,
      usage.outputTokens,
      totalTokens,
      costUsd,
      usage.taskId ?? null,
      usage.agentId ?? null,
      usage.source,
    );

    const recorded: TokenUsage = {
      ...usage,
      id,
      totalTokens,
      costUsd,
      timestamp,
    };

    // Check budgets and publish warnings
    if (this.bus) {
      const budgetStatus = this.checkBudget();
      for (const warning of budgetStatus.warnings) {
        await this.bus.publish('tokens.budget_warning', 'token-tracker', { message: warning });
      }
      if (!budgetStatus.withinBudget) {
        await this.bus.publish('tokens.budget_exceeded', 'token-tracker', {
          dailyRemaining: budgetStatus.dailyRemaining,
          monthlyRemaining: budgetStatus.monthlyRemaining,
        });
      }
    }

    return recorded;
  }

  getStats(): TokenStats {
    const now = new Date();
    const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate()).toISOString();
    const monthStart = new Date(now.getFullYear(), now.getMonth(), 1).toISOString();

    const today = this.aggregateUsage('timestamp >= ?', [todayStart]);
    const thisMonth = this.aggregateUsage('timestamp >= ?', [monthStart]);
    const allTime = this.aggregateUsage('1=1', []);

    const byModel = this.groupedUsage('model');
    const byAgent = this.groupedUsage('agent_id');

    return { today, thisMonth, allTime, byModel, byAgent };
  }

  getDailyUsage(date?: Date): { tokens: number; cost: number; calls: number } {
    const d = date ?? new Date();
    const dayStart = new Date(d.getFullYear(), d.getMonth(), d.getDate()).toISOString();
    const dayEnd = new Date(d.getFullYear(), d.getMonth(), d.getDate() + 1).toISOString();
    return this.aggregateUsage('timestamp >= ? AND timestamp < ?', [dayStart, dayEnd]);
  }

  getMonthlyUsage(month?: Date): { tokens: number; cost: number; calls: number } {
    const d = month ?? new Date();
    const monthStart = new Date(d.getFullYear(), d.getMonth(), 1).toISOString();
    const monthEnd = new Date(d.getFullYear(), d.getMonth() + 1, 1).toISOString();
    return this.aggregateUsage('timestamp >= ? AND timestamp < ?', [monthStart, monthEnd]);
  }

  getTaskUsage(taskId: string): { tokens: number; cost: number; calls: number } {
    return this.aggregateUsage('task_id = ?', [taskId]);
  }

  getAgentUsage(agentId: string): { tokens: number; cost: number; calls: number } {
    return this.aggregateUsage('agent_id = ?', [agentId]);
  }

  checkBudget(): {
    withinBudget: boolean;
    dailyRemaining: number;
    monthlyRemaining: number;
    warnings: string[];
  } {
    const daily = this.getDailyUsage();
    const monthly = this.getMonthlyUsage();
    const warnings: string[] = [];

    const dailyRemaining = this.budget.dailyLimit - daily.tokens;
    const monthlyRemaining = this.budget.monthlyLimit - monthly.tokens;

    const dailyRatio = daily.tokens / this.budget.dailyLimit;
    const monthlyRatio = monthly.tokens / this.budget.monthlyLimit;

    if (dailyRatio >= this.budget.warningThreshold && dailyRatio < 1) {
      warnings.push(`Daily token usage at ${(dailyRatio * 100).toFixed(0)}% of limit (${daily.tokens.toLocaleString()} / ${this.budget.dailyLimit.toLocaleString()})`);
    }
    if (monthlyRatio >= this.budget.warningThreshold && monthlyRatio < 1) {
      warnings.push(`Monthly token usage at ${(monthlyRatio * 100).toFixed(0)}% of limit (${monthly.tokens.toLocaleString()} / ${this.budget.monthlyLimit.toLocaleString()})`);
    }
    if (dailyRatio >= 1) {
      warnings.push(`Daily token limit exceeded (${daily.tokens.toLocaleString()} / ${this.budget.dailyLimit.toLocaleString()})`);
    }
    if (monthlyRatio >= 1) {
      warnings.push(`Monthly token limit exceeded (${monthly.tokens.toLocaleString()} / ${this.budget.monthlyLimit.toLocaleString()})`);
    }

    return {
      withinBudget: dailyRemaining >= 0 && monthlyRemaining >= 0,
      dailyRemaining: Math.max(0, dailyRemaining),
      monthlyRemaining: Math.max(0, monthlyRemaining),
      warnings,
    };
  }

  getBudget(): TokenBudget {
    return { ...this.budget };
  }

  setBudget(budget: Partial<TokenBudget>): void {
    this.budget = { ...this.budget, ...budget };
  }

  diagnose(): DiagnosticResult {
    const checks = [];

    try {
      const stats = this.getStats();
      checks.push({
        name: 'Token database',
        passed: true,
        message: `${stats.allTime.calls} total LLM calls recorded`,
      });

      const budgetStatus = this.checkBudget();
      checks.push({
        name: 'Budget status',
        passed: budgetStatus.withinBudget,
        message: budgetStatus.withinBudget
          ? `Within budget (daily: ${budgetStatus.dailyRemaining.toLocaleString()} remaining, monthly: ${budgetStatus.monthlyRemaining.toLocaleString()} remaining)`
          : `Budget exceeded! Daily remaining: ${budgetStatus.dailyRemaining.toLocaleString()}, Monthly remaining: ${budgetStatus.monthlyRemaining.toLocaleString()}`,
        details: {
          dailyRemaining: budgetStatus.dailyRemaining,
          monthlyRemaining: budgetStatus.monthlyRemaining,
          warnings: budgetStatus.warnings,
        },
      });

      checks.push({
        name: 'Today usage',
        passed: true,
        message: `${stats.today.tokens.toLocaleString()} tokens, $${stats.today.cost.toFixed(4)} cost, ${stats.today.calls} calls`,
      });
    } catch (err) {
      checks.push({
        name: 'Token database',
        passed: false,
        message: `Error: ${err instanceof Error ? err.message : String(err)}`,
      });
    }

    return {
      module: 'tokens',
      status: checks.every((c) => c.passed) ? 'healthy' : 'degraded',
      checks,
    };
  }

  close(): void {
    if (this.unsubscribe) {
      this.unsubscribe();
      this.unsubscribe = null;
    }
    this.db.close();
  }

  private aggregateUsage(where: string, params: unknown[]): { tokens: number; cost: number; calls: number } {
    const row = this.db.prepare(`
      SELECT
        COALESCE(SUM(total_tokens), 0) as tokens,
        COALESCE(SUM(cost_usd), 0) as cost,
        COUNT(*) as calls
      FROM token_usage
      WHERE ${where}
    `).get(...params) as any;

    return {
      tokens: row.tokens,
      cost: row.cost,
      calls: row.calls,
    };
  }

  private groupedUsage(column: string): Record<string, { tokens: number; cost: number; calls: number }> {
    const rows = this.db.prepare(`
      SELECT
        ${column} as group_key,
        COALESCE(SUM(total_tokens), 0) as tokens,
        COALESCE(SUM(cost_usd), 0) as cost,
        COUNT(*) as calls
      FROM token_usage
      WHERE ${column} IS NOT NULL
      GROUP BY ${column}
    `).all() as any[];

    const result: Record<string, { tokens: number; cost: number; calls: number }> = {};
    for (const row of rows) {
      result[row.group_key] = { tokens: row.tokens, cost: row.cost, calls: row.calls };
    }
    return result;
  }
}
