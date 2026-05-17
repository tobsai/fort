/**
 * GoalsService — manages structured goals per agent.
 *
 * Goals exist as first-class DB objects (not prose in SOUL.md). They
 * are created during the hatch, edited via CLI/API, used as context
 * by every user-facing chat (LLMClient.buildSystemPrompt), tagged on
 * tasks (TaskGraph), and reviewed by the Reflection service.
 */

import { v4 as uuid } from 'uuid';
import type { ModuleBus } from '../module-bus/index.js';
import type { Goal, GoalStatus, GoalSource } from '../types.js';
import type { GoalsStore } from './goals-store.js';

export interface CreateGoalInput {
  agentId: string;
  title: string;
  description?: string | null;
  source?: GoalSource;
}

export interface UpdateGoalInput {
  title?: string;
  description?: string | null;
  status?: GoalStatus;
}

export class GoalsService {
  constructor(private store: GoalsStore, private bus: ModuleBus) {}

  create(input: CreateGoalInput): Goal {
    const now = new Date();
    const goal: Goal = {
      id: uuid(),
      agentId: input.agentId,
      title: input.title.trim(),
      description: input.description?.trim() ?? null,
      status: 'active',
      source: input.source ?? 'user',
      createdAt: now,
      updatedAt: now,
      lastActivityAt: now,
      lastNudgeAt: null,
    };
    this.store.upsert(goal);
    void this.bus.publish('goal:created', 'goals', goal);
    return goal;
  }

  get(id: string): Goal | null {
    return this.store.get(id);
  }

  /**
   * List goals for an agent. Defaults to active goals — the set that
   * gets injected into the agent's system prompt.
   */
  listForAgent(agentId: string, status: GoalStatus | GoalStatus[] = 'active'): Goal[] {
    return this.store.query({ agentId, status });
  }

  listAll(agentId: string): Goal[] {
    return this.store.query({ agentId });
  }

  update(id: string, patch: UpdateGoalInput): Goal | null {
    const current = this.store.get(id);
    if (!current) return null;
    const updated: Goal = {
      ...current,
      title: patch.title?.trim() ?? current.title,
      description:
        patch.description === undefined ? current.description : patch.description?.trim() ?? null,
      status: patch.status ?? current.status,
      updatedAt: new Date(),
    };
    this.store.upsert(updated);
    void this.bus.publish('goal:updated', 'goals', updated);
    return updated;
  }

  /** Mark goal achieved. Convenience for `update(id, { status: 'achieved' })`. */
  achieve(id: string): Goal | null {
    return this.update(id, { status: 'achieved' });
  }

  /** Mark a task event against this goal (advances `last_activity_at`). */
  recordActivity(goalId: string, when: Date = new Date()): void {
    this.store.touch(goalId, when);
  }

  /** Record that the Reflection service nudged this goal (cooldown clock). */
  markNudged(goalId: string, when: Date = new Date()): void {
    this.store.markNudged(goalId, when);
  }

  delete(id: string): boolean {
    const ok = this.store.delete(id);
    if (ok) void this.bus.publish('goal:deleted', 'goals', { id });
    return ok;
  }
}
