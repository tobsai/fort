import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import Database from 'better-sqlite3';
import {
  scoreGoal,
  scoreGoals,
  DEFAULT_GOAL_SCORE_CONFIG,
} from '../services/reflection-scoring.js';
import { GoalsStore } from '../services/goals-store.js';
import { GoalsService } from '../services/goals.js';
import { ReflectionService } from '../services/reflection.js';
import { ModuleBus } from '../module-bus/index.js';
import { TaskGraph, TaskStore } from '../task-graph/index.js';
import { LLMClient } from '../llm/index.js';
import { TokenTracker } from '../tokens/index.js';
import { BehaviorManager } from '../behaviors/index.js';
import { MemoryManager } from '../memory/index.js';
import type { Goal, Task } from '../types.js';

const day = 86400 * 1000;
const hour = 60 * 60 * 1000;

function makeGoal(overrides: Partial<Goal> = {}): Goal {
  const now = new Date();
  return {
    id: 'g1',
    agentId: 'a1',
    title: 'Test goal',
    description: null,
    status: 'active',
    source: 'user',
    createdAt: now,
    updatedAt: now,
    lastActivityAt: now,
    lastNudgeAt: null,
    ...overrides,
  };
}

describe('scoreGoal', () => {
  it('flags stale goals past the threshold', () => {
    const now = new Date('2026-05-16T12:00:00Z');
    const goal = makeGoal({
      lastActivityAt: new Date(now.getTime() - 10 * day),
    });
    const score = scoreGoal(goal, [], now);
    expect(score.flag).toBe('stale');
    expect(score.staleDays).toBe(10);
  });

  it('does not flag healthy goals', () => {
    const now = new Date('2026-05-16T12:00:00Z');
    const goal = makeGoal({
      lastActivityAt: new Date(now.getTime() - 2 * day),
    });
    expect(scoreGoal(goal, [], now).flag).toBeNull();
  });

  it('flags goals with failed tasks as blocked', () => {
    const now = new Date('2026-05-16T12:00:00Z');
    const goal = makeGoal({
      lastActivityAt: new Date(now.getTime() - 1 * day),
    });
    const tasks: Task[] = [
      {
        id: 't1', shortId: 'T-1', parentId: null, title: 't', description: '',
        status: 'failed', source: 'agent_delegation', assignedAgent: null,
        sourceAgentId: null, createdAt: now, updatedAt: now, completedAt: null,
        result: null, assignedTo: null, metadata: {}, subtaskIds: [], threadId: null,
        goalId: 'g1',
      },
    ];
    expect(scoreGoal(goal, tasks, now).flag).toBe('blocked');
  });

  it('flags `created` tasks older than the blocker threshold', () => {
    const now = new Date('2026-05-16T12:00:00Z');
    const goal = makeGoal({
      lastActivityAt: new Date(now.getTime() - 1 * day),
    });
    const tasks: Task[] = [
      {
        id: 't1', shortId: 'T-1', parentId: null, title: 't', description: '',
        status: 'created', source: 'user_chat', assignedAgent: null,
        sourceAgentId: null,
        createdAt: new Date(now.getTime() - 5 * day),
        updatedAt: now, completedAt: null,
        result: null, assignedTo: null, metadata: {}, subtaskIds: [], threadId: null,
        goalId: 'g1',
      },
    ];
    expect(scoreGoal(goal, tasks, now).flag).toBe('blocked');
  });

  it('respects cooldown — does not flag if nudged within window', () => {
    const now = new Date('2026-05-16T12:00:00Z');
    const goal = makeGoal({
      lastActivityAt: new Date(now.getTime() - 10 * day),
      lastNudgeAt: new Date(now.getTime() - 24 * hour), // 24h < default 48h
    });
    expect(scoreGoal(goal, [], now).flag).toBeNull();
  });

  it('flags again once cooldown elapses', () => {
    const now = new Date('2026-05-16T12:00:00Z');
    const goal = makeGoal({
      lastActivityAt: new Date(now.getTime() - 10 * day),
      lastNudgeAt: new Date(now.getTime() - 60 * hour),
    });
    expect(scoreGoal(goal, [], now).flag).toBe('stale');
  });

  it('skips non-active goals (paused/achieved/abandoned)', () => {
    const now = new Date('2026-05-16T12:00:00Z');
    expect(scoreGoal(makeGoal({ status: 'paused' }), [], now).flag).toBeNull();
    expect(scoreGoal(makeGoal({ status: 'achieved' }), [], now).flag).toBeNull();
    expect(scoreGoal(makeGoal({ status: 'abandoned' }), [], now).flag).toBeNull();
  });
});

describe('ReflectionService.reviewGoals', () => {
  let tmpDir: string;
  let db: InstanceType<typeof Database>;
  let bus: ModuleBus;
  let memory: MemoryManager;
  let tokens: TokenTracker;
  let llm: LLMClient;
  let taskGraph: TaskGraph;
  let goals: GoalsService;
  let reflection: ReflectionService;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-reflect-'));
    bus = new ModuleBus();
    db = new (Database as any)(join(tmpDir, 'fort.db'));
    const taskStore = new TaskStore(db);
    taskStore.initSchema();
    taskGraph = new TaskGraph(bus, taskStore);
    memory = new MemoryManager(join(tmpDir, 'memory.db'), bus);
    tokens = new TokenTracker(join(tmpDir, 'tokens.db'), bus);
    const behaviors = new BehaviorManager(memory, bus);
    vi.spyOn(LLMClient, 'readEnvFile').mockReturnValue(null);
    vi.spyOn(LLMClient, 'readOpenAIEnvFile').mockReturnValue(null);
    vi.spyOn(LLMClient, 'readCodexOpenAIToken').mockReturnValue(null);
    vi.spyOn(LLMClient, 'readKeychainToken').mockReturnValue(null);
    llm = new LLMClient({}, bus, tokens, behaviors, memory);
    const goalsStore = new GoalsStore(db);
    goalsStore.initSchema();
    goals = new GoalsService(goalsStore, bus);
    reflection = new ReflectionService(taskGraph, bus, llm);
    reflection.setGoals(goals);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    tokens.close();
    memory.close();
    (db as any).close?.();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('returns "no active goals" summary when nothing to review', async () => {
    const result = await reflection.reviewGoals(['agent-1']);
    expect(result.reviewedGoals).toBe(0);
    expect(result.summary).toMatch(/No active goals/);
  });

  it('produces a nudge action for a stale goal and creates a chat task', async () => {
    const goal = goals.create({ agentId: 'agent-1', title: 'Ship Fort v1' });
    // Backdate lastActivityAt to make the goal stale.
    const store = (goals as any).store as GoalsStore;
    const stale = { ...goal, lastActivityAt: new Date(Date.now() - 10 * day) };
    store.upsert(stale);

    const result = await reflection.reviewGoals(['agent-1']);
    expect(result.reviewedGoals).toBe(1);
    expect(result.actions).toHaveLength(1);
    expect(result.actions[0].type).toBe('nudge');

    // A chat task was created against this goal
    const tasks = taskGraph.getAllTasks();
    const nudgeTask = tasks.find((t) => t.metadata.kind === 'reflection_nudge');
    expect(nudgeTask).toBeDefined();
    expect(nudgeTask?.metadata.goalId).toBe(goal.id);
    expect(nudgeTask?.assignedTo).toBe('user');
  });

  it('honors cooldown — second pass within window produces no new nudge', async () => {
    const goal = goals.create({ agentId: 'agent-1', title: 'Ship Fort v1' });
    const store = (goals as any).store as GoalsStore;
    store.upsert({ ...goal, lastActivityAt: new Date(Date.now() - 10 * day) });

    const first = await reflection.reviewGoals(['agent-1']);
    expect(first.actions.filter((a) => a.type !== 'skip')).toHaveLength(1);

    const second = await reflection.reviewGoals(['agent-1']);
    expect(second.actions.filter((a) => a.type !== 'skip')).toHaveLength(0);
  });

  it('returns "Reflection is off" when disabled', async () => {
    reflection.setEnabled(false);
    const result = await reflection.reviewGoals(['agent-1']);
    expect(result.summary).toMatch(/Reflection is off/);
    expect(result.reviewedGoals).toBe(0);
  });
});
