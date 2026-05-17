/**
 * Reflection Service — Deterministic chat review
 *
 * Not an agent. Periodically reviews recent chat tasks to ensure
 * no action items were missed. Creates tasks for anything that
 * slipped through.
 */

import type { Task, Goal } from '../types.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { LLMClient } from '../llm/index.js';
import type { DiagnosticResult } from '../types.js';
import type { GoalsService } from './goals.js';
import { scoreGoals, DEFAULT_GOAL_SCORE_CONFIG, type GoalScore, type GoalScoreConfig } from './reflection-scoring.js';

export interface ReflectionResult {
  reviewedTasks: number;
  newTasks: Task[];
  summary: string;
}

export type GoalReflectionAction =
  | { type: 'nudge'; goalId: string; message: string }
  | { type: 'draft_task'; goalId: string; title: string; description: string }
  | { type: 'skip'; goalId: string };

export interface GoalReflectionResult {
  reviewedGoals: number;
  actions: GoalReflectionAction[];
  summary: string;
}

export interface ReflectionConfig {
  /** Master switch. `fort reflection off` flips this to false. */
  enabled: boolean;
  /** Scoring thresholds — see GoalScoreConfig. */
  scoring: GoalScoreConfig;
}

export const DEFAULT_REFLECTION_CONFIG: ReflectionConfig = {
  enabled: true,
  scoring: DEFAULT_GOAL_SCORE_CONFIG,
};

export class ReflectionService {
  private taskGraph: TaskGraph;
  private bus: ModuleBus;
  private llm: LLMClient;
  private goals: GoalsService | null = null;
  private lastReviewAt: Date | null = null;
  private lastGoalReviewAt: Date | null = null;
  private config: ReflectionConfig = { ...DEFAULT_REFLECTION_CONFIG };

  constructor(taskGraph: TaskGraph, bus: ModuleBus, llm: LLMClient) {
    this.taskGraph = taskGraph;
    this.bus = bus;
    this.llm = llm;
  }

  /** Wire goals so reviewGoals() can run. Called from Fort during init. */
  setGoals(goals: GoalsService): void {
    this.goals = goals;
  }

  /** Toggle the reflection loop. `fort reflection off` calls this with false. */
  setEnabled(enabled: boolean): void {
    this.config.enabled = enabled;
  }

  getConfig(): ReflectionConfig {
    return { ...this.config };
  }

  /**
   * Review recent chat tasks for missed action items.
   * This is a deterministic pass — it scans completed tasks and
   * checks if any mentioned actions weren't turned into tasks.
   */
  async reviewChats(since?: Date): Promise<ReflectionResult> {
    const cutoff = since ?? this.lastReviewAt ?? new Date(Date.now() - 30 * 60 * 1000);
    this.lastReviewAt = new Date();

    // Get all completed chat tasks since cutoff
    const allTasks = this.taskGraph.getAllTasks();
    const chatTasks = allTasks.filter((t) =>
      t.metadata.type === 'chat' &&
      t.status === 'completed' &&
      t.result &&
      t.completedAt &&
      t.completedAt >= cutoff
    );

    if (chatTasks.length === 0) {
      return {
        reviewedTasks: 0,
        newTasks: [],
        summary: 'No recent chat tasks to review.',
      };
    }

    // If LLM is not configured, do a basic pattern-based review
    if (!this.llm.isConfigured) {
      return this.basicReview(chatTasks);
    }

    // Use LLM to analyze chats for missed tasks
    return this.llmReview(chatTasks);
  }

  /**
   * Basic pattern-based review (no LLM needed).
   * Looks for common action indicators in chat results.
   */
  private basicReview(chatTasks: Task[]): ReflectionResult {
    const newTasks: Task[] = [];
    const actionPatterns = [
      /(?:i(?:'ll| will)|let me|going to|need to|should|must|have to|don't forget to)\s+(.+?)(?:\.|$)/gi,
      /(?:remind|schedule|follow up|check back|circle back)\s+(.+?)(?:\.|$)/gi,
      /(?:todo|action item|next step)[:\s]+(.+?)(?:\.|$)/gi,
    ];

    for (const task of chatTasks) {
      const text = `${task.description} ${task.result ?? ''}`;

      for (const pattern of actionPatterns) {
        pattern.lastIndex = 0;
        let match;
        while ((match = pattern.exec(text)) !== null) {
          const actionText = match[1].trim();
          if (actionText.length > 10 && actionText.length < 200) {
            // Check if a similar task already exists
            const existing = this.taskGraph.getAllTasks().find((t) =>
              t.title.toLowerCase().includes(actionText.toLowerCase().slice(0, 30))
            );
            if (!existing) {
              const newTask = this.taskGraph.createTask({
                title: actionText.slice(0, 100),
                description: `Detected from chat: "${task.title}"`,
                source: 'reflection',
                metadata: { detectedFrom: task.id, type: 'action_item' },
              });
              newTask.assignedTo = 'user';
              newTasks.push(newTask);
            }
          }
        }
      }
    }

    const summary = newTasks.length > 0
      ? `Found ${newTasks.length} potential action item${newTasks.length > 1 ? 's' : ''} in ${chatTasks.length} recent chat${chatTasks.length > 1 ? 's' : ''}.`
      : `Reviewed ${chatTasks.length} recent chat${chatTasks.length > 1 ? 's' : ''}. No missed action items detected.`;

    this.bus.publish('reflection.completed', 'reflection', {
      reviewedTasks: chatTasks.length,
      newTaskCount: newTasks.length,
      summary,
    });

    return { reviewedTasks: chatTasks.length, newTasks, summary };
  }

  /**
   * LLM-powered review — asks Claude to find missed tasks.
   */
  private async llmReview(chatTasks: Task[]): Promise<ReflectionResult> {
    // Build a summary of recent chats
    const chatSummary = chatTasks.map((t) =>
      `[${t.assignedAgent ?? 'unknown'}] User: ${t.description}\nAgent: ${t.result ?? '(no response)'}`
    ).join('\n---\n');

    const existingTasks = this.taskGraph.getAllTasks()
      .filter((t) => t.status !== 'completed' && t.status !== 'failed')
      .map((t) => `- ${t.title} (${t.status}, assigned to ${t.assignedAgent ?? 'user'})`)
      .join('\n');

    try {
      const response = await this.llm.ask(
        `Review these recent conversations and identify any action items, commitments, or follow-ups that were mentioned but do NOT already exist as tasks.

RECENT CHATS:
${chatSummary}

EXISTING OPEN TASKS:
${existingTasks || '(none)'}

For each missed item, output a JSON array of objects with "title" and "assignedTo" ("agent" or "user") fields. If nothing was missed, output an empty array [].
Only output the JSON array, nothing else.`,
        {
          model: 'fast',
          system: 'You are a task extraction system. Identify action items from conversations. Be conservative — only flag clear commitments or requests, not vague mentions.',
        },
      );

      // Parse LLM response
      const newTasks: Task[] = [];
      try {
        const items = JSON.parse(response);
        if (Array.isArray(items)) {
          for (const item of items) {
            if (item.title && typeof item.title === 'string') {
              const task = this.taskGraph.createTask({
                title: item.title.slice(0, 100),
                description: 'Detected by reflection review',
                source: 'reflection',
                metadata: { type: 'action_item', detectedBy: 'llm' },
              });
              task.assignedTo = item.assignedTo === 'agent' ? 'agent' : 'user';
              newTasks.push(task);
            }
          }
        }
      } catch {
        // LLM response wasn't valid JSON — no tasks extracted
      }

      const summary = newTasks.length > 0
        ? `Found ${newTasks.length} missed action item${newTasks.length > 1 ? 's' : ''} in ${chatTasks.length} recent chat${chatTasks.length > 1 ? 's' : ''}.`
        : `Reviewed ${chatTasks.length} recent chat${chatTasks.length > 1 ? 's' : ''}. No missed items.`;

      this.bus.publish('reflection.completed', 'reflection', {
        reviewedTasks: chatTasks.length,
        newTaskCount: newTasks.length,
        summary,
      });

      return { reviewedTasks: chatTasks.length, newTasks, summary };
    } catch (err) {
      // LLM call failed — fall back to basic review
      return this.basicReview(chatTasks);
    }
  }

  /**
   * Goal review pass. For each active goal, score it; for flagged goals,
   * either nudge the user in chat or draft a next-step task for approval.
   * Bounded proactivity — never executes externally without user say-so.
   *
   * `agentIds`: list of agents to review. Reviews every agent that has
   * at least one active goal when omitted.
   */
  async reviewGoals(agentIds?: string[]): Promise<GoalReflectionResult> {
    if (!this.config.enabled) {
      return { reviewedGoals: 0, actions: [], summary: 'Reflection is off (fort reflection on to enable).' };
    }
    if (!this.goals) {
      return { reviewedGoals: 0, actions: [], summary: 'Goals service not wired — nothing to review.' };
    }

    this.lastGoalReviewAt = new Date();

    // Gather active goals across the target agent set
    const allActiveGoals: Goal[] = [];
    const targets = agentIds ?? this.discoverAgentIdsWithGoals();
    for (const agentId of targets) {
      allActiveGoals.push(...this.goals.listForAgent(agentId, 'active'));
    }

    if (allActiveGoals.length === 0) {
      return { reviewedGoals: 0, actions: [], summary: 'No active goals to review.' };
    }

    // Bucket tasks by goalId for scoring
    const tasksByGoal = new Map<string, Task[]>();
    for (const t of this.taskGraph.getAllTasks()) {
      if (t.goalId) {
        const arr = tasksByGoal.get(t.goalId) ?? [];
        arr.push(t);
        tasksByGoal.set(t.goalId, arr);
      }
    }

    const scores = scoreGoals(allActiveGoals, tasksByGoal, this.lastGoalReviewAt, this.config.scoring);

    const goalsById = new Map(allActiveGoals.map((g) => [g.id, g]));
    const actions: GoalReflectionAction[] = [];

    for (const score of scores) {
      if (!score.flag) continue;
      const goal = goalsById.get(score.goalId);
      if (!goal) continue;

      const action = await this.decideGoalAction(goal, score, tasksByGoal.get(goal.id) ?? []);
      actions.push(action);

      // Apply action
      if (action.type === 'nudge') {
        const task = this.taskGraph.createTask({
          title: action.message.slice(0, 100),
          description: action.message,
          source: 'reflection',
          assignedAgent: goal.agentId,
          metadata: {
            type: 'chat',
            kind: 'reflection_nudge',
            goalId: goal.id,
          },
        });
        task.assignedTo = 'user';
        this.goals.markNudged(goal.id, this.lastGoalReviewAt);
        void this.bus.publish('reflection.goal_nudged', 'reflection', { goalId: goal.id, taskId: task.id });
      } else if (action.type === 'draft_task') {
        const task = this.taskGraph.createTask({
          title: action.title,
          description: action.description,
          source: 'reflection',
          assignedAgent: goal.agentId,
          metadata: {
            kind: 'reflection_draft',
            goalId: goal.id,
            requiresApproval: true,
          },
        });
        task.assignedTo = 'user';
        task.goalId = goal.id;
        this.goals.markNudged(goal.id, this.lastGoalReviewAt);
        void this.bus.publish('reflection.task_drafted', 'reflection', { goalId: goal.id, taskId: task.id });
      }
    }

    const flagged = actions.filter((a) => a.type !== 'skip').length;
    const summary = `Reviewed ${allActiveGoals.length} active goal${allActiveGoals.length === 1 ? '' : 's'}; ${flagged} flagged.`;
    void this.bus.publish('reflection.goals_completed', 'reflection', { reviewedGoals: allActiveGoals.length, flagged });
    return { reviewedGoals: allActiveGoals.length, actions, summary };
  }

  /**
   * Decide what to do about a flagged goal. Uses one cheap LLM call;
   * falls back to a heuristic nudge when no LLM is available.
   */
  private async decideGoalAction(
    goal: Goal,
    score: GoalScore,
    goalTasks: Task[],
  ): Promise<GoalReflectionAction> {
    const fallbackMessage = score.flag === 'stale'
      ? `Goal "${goal.title}" hasn't had activity in ${score.staleDays ?? '?'} days — still active, or should we shelve it?`
      : `Goal "${goal.title}" has ${score.blockerCount} blocked or stalled task${score.blockerCount === 1 ? '' : 's'} — want me to look at unblocking them?`;

    if (!this.llm.isConfigured) {
      return { type: 'nudge', goalId: goal.id, message: fallbackMessage };
    }

    const taskSummary = goalTasks.slice(-5).map((t) => `- [${t.status}] ${t.title}`).join('\n') || '(none)';
    try {
      const raw = await this.llm.ask(
        `A user goal is being reviewed.

Goal: "${goal.title}"
${goal.description ? `Description: ${goal.description}\n` : ''}Flag: ${score.flag} (${score.flag === 'stale' ? `${score.staleDays}d since activity` : `${score.blockerCount} blockers`})
Recent tagged tasks:
${taskSummary}

Decide one action. Respond with JSON only:
{"action": "nudge", "message": "<short chat message to send to the user>"}
or
{"action": "draft_task", "title": "<short title>", "description": "<concrete next step>"}
or
{"action": "skip", "reason": "<why this isn't worth surfacing>"}

Prefer "draft_task" when there's an obvious next step you'd be happy to attempt yourself; prefer "nudge" when the user needs to decide direction; choose "skip" when the flag is a false alarm (e.g., goal is naturally slow-moving).`,
        {
          model: 'fast',
          system: 'You are the Reflection planner for a personal AI agent. Be terse, concrete, and avoid nagging. Respond with JSON only.',
          injectAgentContext: false,
        },
      );

      const parsed = JSON.parse(raw) as { action?: string; message?: string; title?: string; description?: string };
      if (parsed.action === 'nudge' && typeof parsed.message === 'string' && parsed.message.trim()) {
        return { type: 'nudge', goalId: goal.id, message: parsed.message.trim() };
      }
      if (parsed.action === 'draft_task' && typeof parsed.title === 'string' && parsed.title.trim()) {
        return {
          type: 'draft_task',
          goalId: goal.id,
          title: parsed.title.trim(),
          description: (parsed.description ?? '').trim() || parsed.title.trim(),
        };
      }
      if (parsed.action === 'skip') {
        return { type: 'skip', goalId: goal.id };
      }
    } catch {
      // Fall through to heuristic nudge below.
    }
    return { type: 'nudge', goalId: goal.id, message: fallbackMessage };
  }

  private discoverAgentIdsWithGoals(): string[] {
    if (!this.goals) return [];
    // No agent registry here — use task graph as a proxy: any agent that
    // has appeared as the assignedAgent of a goal-tagged task. Not
    // exhaustive; the caller can pass agentIds explicitly for precision.
    const set = new Set<string>();
    for (const t of this.taskGraph.getAllTasks()) {
      if (t.goalId && t.assignedAgent) set.add(t.assignedAgent);
    }
    return Array.from(set);
  }

  diagnose(): DiagnosticResult {
    return {
      module: 'reflection',
      status: 'healthy',
      checks: [
        {
          name: 'Last review',
          passed: true,
          message: this.lastReviewAt
            ? `Last reviewed at ${this.lastReviewAt.toISOString()}`
            : 'No reviews yet',
        },
        {
          name: 'LLM available',
          passed: this.llm.isConfigured,
          message: this.llm.isConfigured
            ? 'LLM-powered review enabled'
            : 'Using basic pattern matching (no LLM)',
        },
      ],
    };
  }
}
