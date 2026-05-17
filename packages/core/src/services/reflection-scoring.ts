/**
 * Pure scoring functions for the Reflection service's goal pass.
 *
 * Kept separate from `reflection.ts` so they're trivially unit-testable
 * without an LLM, a task graph, or a clock.
 */

import type { Goal, Task } from '../types.js';

export interface GoalScoreConfig {
  /** Days since last_activity_at before flagging staleness. Default 7. */
  staleThresholdDays: number;
  /** Days a `created` task can sit before counting as a blocker. Default 3. */
  blockerCreatedDays: number;
  /** Hours since last_nudge_at before reflecting on this goal again. Default 48. */
  cooldownHours: number;
}

export const DEFAULT_GOAL_SCORE_CONFIG: GoalScoreConfig = {
  staleThresholdDays: 7,
  blockerCreatedDays: 3,
  cooldownHours: 48,
};

export interface GoalScore {
  goalId: string;
  flag: 'stale' | 'blocked' | null;
  staleDays: number | null;
  blockerCount: number;
}

/**
 * Score a single goal. Returns the `flag` that explains why we'd want
 * to act, or null if the goal is healthy enough to skip.
 *
 * Skips silently if:
 *   - Goal is not `active` (paused/achieved/abandoned never get nudged).
 *   - A nudge was sent within the cooldown window.
 */
export function scoreGoal(
  goal: Goal,
  goalTasks: Task[],
  now: Date,
  config: GoalScoreConfig = DEFAULT_GOAL_SCORE_CONFIG,
): GoalScore {
  const base: GoalScore = { goalId: goal.id, flag: null, staleDays: null, blockerCount: 0 };

  if (goal.status !== 'active') return base;

  // Cooldown: respect last_nudge_at
  if (goal.lastNudgeAt) {
    const hoursSinceNudge = (now.getTime() - goal.lastNudgeAt.getTime()) / (60 * 60 * 1000);
    if (hoursSinceNudge < config.cooldownHours) return base;
  }

  // Blocker pass: count failed tasks + created tasks older than threshold
  const createdCutoff = now.getTime() - config.blockerCreatedDays * 86400 * 1000;
  const blockers = goalTasks.filter(
    (t) =>
      t.status === 'failed' ||
      (t.status === 'created' && t.createdAt.getTime() <= createdCutoff),
  );
  base.blockerCount = blockers.length;
  if (blockers.length > 0) {
    return { ...base, flag: 'blocked' };
  }

  // Staleness pass
  const reference = goal.lastActivityAt ?? goal.createdAt;
  const staleDays = (now.getTime() - reference.getTime()) / (86400 * 1000);
  base.staleDays = Math.floor(staleDays);
  if (staleDays >= config.staleThresholdDays) {
    return { ...base, flag: 'stale' };
  }

  return base;
}

/** Score a batch of goals against their tasks. Pure. */
export function scoreGoals(
  goals: Goal[],
  tasksByGoalId: Map<string, Task[]>,
  now: Date,
  config: GoalScoreConfig = DEFAULT_GOAL_SCORE_CONFIG,
): GoalScore[] {
  return goals.map((g) => scoreGoal(g, tasksByGoalId.get(g.id) ?? [], now, config));
}
