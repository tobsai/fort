import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import { TokenTracker } from '../tokens/index.js';
import { ModuleBus } from '../module-bus/index.js';

describe('TokenTracker', () => {
  let tmpDir: string;
  let tracker: TokenTracker;
  let bus: ModuleBus;

  function setup(budget?: Record<string, unknown>) {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-tokens-'));
    bus = new ModuleBus();
    tracker = new TokenTracker(join(tmpDir, 'tokens.db'), bus, budget as any);
    return tracker;
  }

  afterEach(() => {
    tracker?.close();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should record usage and return correct stats', async () => {
    setup();

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 1000,
      outputTokens: 500,
      totalTokens: 1500,
      costUsd: 0,
      source: 'test',
    });

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 2000,
      outputTokens: 1000,
      totalTokens: 3000,
      costUsd: 0,
      source: 'test',
    });

    const stats = tracker.getStats();
    expect(stats.today.calls).toBe(2);
    expect(stats.today.tokens).toBe(4500);
    expect(stats.allTime.calls).toBe(2);
    expect(stats.allTime.tokens).toBe(4500);
    // Cost should be auto-calculated from pricing table
    // 3000 input * $3/MTok + 1500 output * $15/MTok = $0.009 + $0.0225 = $0.0315
    expect(stats.today.cost).toBeCloseTo(0.0315, 4);
  });

  it('should calculate cost automatically from pricing table', async () => {
    setup();

    const usage = await tracker.record({
      timestamp: new Date(),
      model: 'claude-opus-4-6-20250311',
      inputTokens: 10000,
      outputTokens: 5000,
      totalTokens: 15000,
      costUsd: 0,
      source: 'test',
    });

    // 10000 * $15/MTok + 5000 * $75/MTok = $0.15 + $0.375 = $0.525
    expect(usage.costUsd).toBeCloseTo(0.525, 4);
  });

  it('should use provided costUsd when non-zero', async () => {
    setup();

    const usage = await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 1000,
      outputTokens: 500,
      totalTokens: 1500,
      costUsd: 0.99,
      source: 'test',
    });

    expect(usage.costUsd).toBe(0.99);
  });

  it('should track usage by model', async () => {
    setup();

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 1000,
      outputTokens: 500,
      totalTokens: 1500,
      costUsd: 0,
      source: 'test',
    });

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-haiku-4-5-20250315',
      inputTokens: 2000,
      outputTokens: 1000,
      totalTokens: 3000,
      costUsd: 0,
      source: 'test',
    });

    const stats = tracker.getStats();
    expect(stats.byModel['claude-sonnet-4-6-20250311'].calls).toBe(1);
    expect(stats.byModel['claude-sonnet-4-6-20250311'].tokens).toBe(1500);
    expect(stats.byModel['claude-haiku-4-5-20250315'].calls).toBe(1);
    expect(stats.byModel['claude-haiku-4-5-20250315'].tokens).toBe(3000);
  });

  it('should track usage by agent', async () => {
    setup();

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 1000,
      outputTokens: 500,
      totalTokens: 1500,
      costUsd: 0,
      agentId: 'orchestrator',
      source: 'test',
    });

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 3000,
      outputTokens: 1500,
      totalTokens: 4500,
      costUsd: 0,
      agentId: 'memory-agent',
      source: 'test',
    });

    const stats = tracker.getStats();
    expect(stats.byAgent['orchestrator'].tokens).toBe(1500);
    expect(stats.byAgent['memory-agent'].tokens).toBe(4500);

    const agentUsage = tracker.getAgentUsage('orchestrator');
    expect(agentUsage.tokens).toBe(1500);
    expect(agentUsage.calls).toBe(1);
  });

  it('should track usage by task', async () => {
    setup();

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 500,
      outputTokens: 200,
      totalTokens: 700,
      costUsd: 0,
      taskId: 'task-1',
      source: 'test',
    });

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 800,
      outputTokens: 300,
      totalTokens: 1100,
      costUsd: 0,
      taskId: 'task-1',
      source: 'test',
    });

    const taskUsage = tracker.getTaskUsage('task-1');
    expect(taskUsage.tokens).toBe(1800);
    expect(taskUsage.calls).toBe(2);
  });

  it('should check budget and report within limits', async () => {
    setup({ dailyLimit: 100_000, monthlyLimit: 1_000_000, perTaskLimit: 50_000, warningThreshold: 0.8 });

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 5000,
      outputTokens: 2000,
      totalTokens: 7000,
      costUsd: 0,
      source: 'test',
    });

    const budget = tracker.checkBudget();
    expect(budget.withinBudget).toBe(true);
    expect(budget.dailyRemaining).toBe(93_000);
    expect(budget.warnings).toHaveLength(0);
  });

  it('should warn when approaching budget limit', async () => {
    setup({ dailyLimit: 10_000, monthlyLimit: 1_000_000, perTaskLimit: 50_000, warningThreshold: 0.8 });

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 4500,
      outputTokens: 4500,
      totalTokens: 9000,
      costUsd: 0,
      source: 'test',
    });

    const budget = tracker.checkBudget();
    expect(budget.withinBudget).toBe(true);
    expect(budget.warnings.length).toBeGreaterThan(0);
    expect(budget.warnings[0]).toContain('90%');
  });

  it('should report budget exceeded', async () => {
    setup({ dailyLimit: 5000, monthlyLimit: 1_000_000, perTaskLimit: 50_000, warningThreshold: 0.8 });

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 3000,
      outputTokens: 3000,
      totalTokens: 6000,
      costUsd: 0,
      source: 'test',
    });

    const budget = tracker.checkBudget();
    expect(budget.withinBudget).toBe(false);
    expect(budget.dailyRemaining).toBe(0);
  });

  it('should publish budget warning events via ModuleBus', async () => {
    setup({ dailyLimit: 10_000, monthlyLimit: 1_000_000, perTaskLimit: 50_000, warningThreshold: 0.8 });

    const warnings: string[] = [];
    bus.subscribe('tokens.budget_warning', (event) => {
      warnings.push((event.payload as any).message);
    });

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 4500,
      outputTokens: 4500,
      totalTokens: 9000,
      costUsd: 0,
      source: 'test',
    });

    expect(warnings.length).toBeGreaterThan(0);
  });

  it('should publish budget exceeded events via ModuleBus', async () => {
    setup({ dailyLimit: 5000, monthlyLimit: 1_000_000, perTaskLimit: 50_000, warningThreshold: 0.8 });

    let exceeded = false;
    bus.subscribe('tokens.budget_exceeded', () => {
      exceeded = true;
    });

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 3000,
      outputTokens: 3000,
      totalTokens: 6000,
      costUsd: 0,
      source: 'test',
    });

    expect(exceeded).toBe(true);
  });

  it('should auto-record from llm.usage bus events', async () => {
    setup();

    await bus.publish('llm.usage', 'orchestrator', {
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 1000,
      outputTokens: 500,
      totalTokens: 1500,
      costUsd: 0,
      source: 'orchestrator',
    });

    const stats = tracker.getStats();
    expect(stats.allTime.calls).toBe(1);
    expect(stats.allTime.tokens).toBe(1500);
  });

  it('should return correct diagnose output', async () => {
    setup();

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 1000,
      outputTokens: 500,
      totalTokens: 1500,
      costUsd: 0,
      source: 'test',
    });

    const diag = tracker.diagnose();
    expect(diag.module).toBe('tokens');
    expect(diag.status).toBe('healthy');
    expect(diag.checks.length).toBeGreaterThanOrEqual(3);
    expect(diag.checks[0].name).toBe('Token database');
    expect(diag.checks[0].passed).toBe(true);
    expect(diag.checks[0].message).toContain('1 total LLM calls');
  });

  it('should report degraded status when budget exceeded in diagnose', async () => {
    setup({ dailyLimit: 1000, monthlyLimit: 1_000_000, perTaskLimit: 50_000, warningThreshold: 0.8 });

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 1000,
      outputTokens: 500,
      totalTokens: 1500,
      costUsd: 0,
      source: 'test',
    });

    const diag = tracker.diagnose();
    expect(diag.status).toBe('degraded');
  });

  it('should allow setting budget dynamically', () => {
    setup();

    tracker.setBudget({ dailyLimit: 50_000 });
    const budget = tracker.getBudget();
    expect(budget.dailyLimit).toBe(50_000);
    // Other defaults should remain
    expect(budget.monthlyLimit).toBe(20_000_000);
  });

  it('should get daily usage for a specific date', async () => {
    setup();

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 1000,
      outputTokens: 500,
      totalTokens: 1500,
      costUsd: 0,
      source: 'test',
    });

    const todayUsage = tracker.getDailyUsage();
    expect(todayUsage.tokens).toBe(1500);
    expect(todayUsage.calls).toBe(1);

    // A different date should have zero usage
    const otherDay = new Date(2020, 0, 1);
    const otherUsage = tracker.getDailyUsage(otherDay);
    expect(otherUsage.tokens).toBe(0);
    expect(otherUsage.calls).toBe(0);
  });

  it('should get monthly usage', async () => {
    setup();

    await tracker.record({
      timestamp: new Date(),
      model: 'claude-sonnet-4-6-20250311',
      inputTokens: 2000,
      outputTokens: 1000,
      totalTokens: 3000,
      costUsd: 0,
      source: 'test',
    });

    const monthUsage = tracker.getMonthlyUsage();
    expect(monthUsage.tokens).toBe(3000);
    expect(monthUsage.calls).toBe(1);
  });
});
