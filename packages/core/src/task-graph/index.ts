/**
 * Task Graph Engine — The Backbone
 *
 * Core invariant: every conversation creates a task.
 * Tasks are the atomic unit of transparency.
 */

import { v4 as uuid } from 'uuid';
import type { Task, TaskStatus, TaskSource, Thread, FortEvent } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { LLMClient } from '../llm/index.js';

export class TaskGraph {
  private tasks: Map<string, Task> = new Map();
  private threads: Map<string, Thread> = new Map();
  private bus: ModuleBus;
  private llm: LLMClient | null = null;
  private taskCounter = 0;

  constructor(bus: ModuleBus) {
    this.bus = bus;
  }

  setLLM(llm: LLMClient): void {
    this.llm = llm;
  }

  createTask(params: {
    title: string;
    description?: string;
    source: TaskSource;
    parentId?: string;
    assignedAgent?: string;
    threadId?: string;
    metadata?: Record<string, unknown>;
  }): Task {
    this.taskCounter++;
    const task: Task = {
      id: uuid(),
      shortId: `FORT-${String(this.taskCounter).padStart(3, '0')}`,
      parentId: params.parentId ?? null,
      title: params.title,
      description: params.description ?? '',
      status: 'created',
      source: params.source,
      assignedAgent: params.assignedAgent ?? null,
      createdAt: new Date(),
      updatedAt: new Date(),
      completedAt: null,
      result: null,
      assignedTo: params.assignedAgent ? 'agent' : null,
      metadata: params.metadata ?? {},
      subtaskIds: [],
      threadId: params.threadId ?? null,
    };

    this.tasks.set(task.id, task);

    // Link to parent
    if (task.parentId) {
      const parent = this.tasks.get(task.parentId);
      if (parent) {
        parent.subtaskIds.push(task.id);
        parent.updatedAt = new Date();
      }
    }

    this.bus.publish('task.created', 'task-graph', { task });
    return task;
  }

  updateStatus(taskId: string, status: TaskStatus, reason?: string, result?: string): Task {
    const task = this.getTask(taskId);
    const previousStatus = task.status;
    task.status = status;
    task.updatedAt = new Date();

    if (status === 'completed' || status === 'failed') {
      task.completedAt = new Date();
    }

    if (reason) {
      task.metadata.statusReason = reason;
    }

    if (result !== undefined) {
      task.result = result;
    }

    this.bus.publish('task.status_changed', 'task-graph', {
      task,
      previousStatus,
      newStatus: status,
      reason,
    });

    // Check if all subtasks of parent are done
    if ((status === 'completed' || status === 'failed') && task.parentId) {
      this.checkParentCompletion(task.parentId);
    }

    return task;
  }

  assignAgent(taskId: string, agentId: string): Task {
    const task = this.getTask(taskId);
    task.assignedAgent = agentId;
    task.updatedAt = new Date();
    this.bus.publish('task.assigned', 'task-graph', { task, agentId });
    return task;
  }

  /**
   * Convenience: complete a task with a result string.
   * Use this for system/internal completions that don't need review.
   */
  completeTask(taskId: string, result: string): Task {
    return this.updateStatus(taskId, 'completed', undefined, result);
  }

  /**
   * LLM-reviewed completion — asks a fast-tier model whether the agent
   * actually addressed the task before marking it complete.
   * Falls through to completeTask() if no LLM is configured or on error.
   */
  async reviewCompletion(taskId: string, result: string): Promise<Task> {
    const task = this.getTask(taskId);

    // ── Deterministic pre-check ──────────────────────────────────────
    // If the response clearly states inability, reject without burning an LLM call.
    const inabilityReason = this.detectInability(result);
    if (inabilityReason) {
      this.bus.publish('task.review_completed', 'task-graph', {
        task,
        approved: false,
        reason: inabilityReason,
      });
      return this.updateStatus(taskId, 'needs_review', inabilityReason, result);
    }

    // ── LLM review gate ──────────────────────────────────────────────
    if (!this.llm || !this.llm.isConfigured) {
      return this.completeTask(taskId, result);
    }

    try {
      const response = await this.llm.ask(
        `You are reviewing whether an agent has completed a task.

Task: "${task.title}"
Description: "${task.description}"
Agent's result: "${result}"

Has the agent ACTUALLY completed what was requested? Consider:
- Did the agent perform the requested action or deliver the requested outcome?
- If the agent said it CANNOT do something, that is NOT completion — reject it.
- If the agent only offered alternatives instead of doing what was asked, reject it.
- Explaining limitations, apologizing, or suggesting workarounds is NOT completion.
- Only approve if the user's original request was fulfilled.

Respond with JSON only: {"approved": true, "reason": "brief explanation"} or {"approved": false, "reason": "brief explanation"}`,
        {
          model: 'fast',
          taskId: task.id,
          system: 'You are a strict task completion reviewer. Respond with JSON only. A task is only complete if the requested action was actually performed. Declining, explaining inability, or offering alternatives is NOT completion.',
        },
      );

      const decision = JSON.parse(response);

      this.bus.publish('task.review_completed', 'task-graph', {
        task,
        approved: decision.approved,
        reason: decision.reason,
      });

      if (decision.approved) {
        return this.completeTask(taskId, result);
      } else {
        return this.updateStatus(taskId, 'needs_review', decision.reason, result);
      }
    } catch {
      // LLM review failed — complete only if the response passes the deterministic check.
      // The deterministic check already ran above so if we're here, the response has no
      // obvious inability signals. Safe to complete.
      return this.completeTask(taskId, result);
    }
  }

  /**
   * Deterministic check for inability signals in agent responses.
   * Returns a reason string if the response indicates the agent couldn't
   * fulfill the request, or null if no clear inability signal is found.
   */
  private detectInability(result: string): string | null {
    // If the response contains a valid, parseable tool-building proposal, it's proactive — not inability
    const toolJsonMatch = result.match(/\{[\s\S]*?"needsTool"\s*:\s*true[\s\S]*?\}/);
    if (toolJsonMatch) {
      try {
        const parsed = JSON.parse(toolJsonMatch[0]);
        if (parsed.needsTool && parsed.toolName) {
          return null; // Valid tool proposal — skip inability detection
        }
      } catch {
        // Malformed JSON — don't skip, continue with inability detection
      }
    }

    // Normalize smart quotes/apostrophes to ASCII before matching
    const lower = result.toLowerCase().replace(/[\u2018\u2019\u2032]/g, "'");

    // Phrases that clearly indicate the agent could not perform the task
    const inabilityPatterns = [
      /i('m| am) (unable|not able) to/,
      /i (can'?t|cannot) (access|browse|check|connect|fetch|open|pull|read|reach|retrieve|search|send|view|visit)/,
      /i (don'?t|do not) (currently )?have (the )?(direct )?(access|ability|capability|capacity|integration)/,
      /i (lack|currently lack) (direct )?(access|integration|capability|ability)/,
      /beyond my (current )?(capabilities|abilities)/,
      /outside (of )?my (current )?(capabilities|abilities|scope)/,
      /i('m| am) not (currently )?(able|capable|equipped)/,
      /unfortunately,? i (can'?t|cannot|am unable)/,
      /i must (inform|let) you that i (can'?t|cannot|don'?t|currently lack)/,
      /my capabilities are limited to/,
      /i('m| am) afraid i (can'?t|cannot|don'?t|must)/,
      /i don'?t have direct access/,
    ];

    for (const pattern of inabilityPatterns) {
      if (pattern.test(lower)) {
        return 'Agent indicated it cannot perform the requested action';
      }
    }

    return null;
  }

  decompose(parentId: string, subtasks: Array<{
    title: string;
    description?: string;
    assignedAgent?: string;
  }>): Task[] {
    const parent = this.getTask(parentId);
    this.updateStatus(parentId, 'in_progress');

    return subtasks.map((sub) =>
      this.createTask({
        title: sub.title,
        description: sub.description,
        source: 'agent_delegation',
        parentId,
        assignedAgent: sub.assignedAgent,
        threadId: parent.threadId ?? undefined,
      })
    );
  }

  getTask(taskId: string): Task {
    const task = this.tasks.get(taskId);
    if (!task) throw new Error(`Task not found: ${taskId}`);
    return task;
  }

  getSubtasks(taskId: string): Task[] {
    const task = this.getTask(taskId);
    return task.subtaskIds.map((id) => this.getTask(id));
  }

  // ─── Thread Management ──────────────────────────────────────────

  createThread(params: {
    name: string;
    taskId: string;
    parentThreadId?: string;
  }): Thread {
    const thread: Thread = {
      id: uuid(),
      name: params.name,
      description: '',
      taskId: params.taskId,
      parentThreadId: params.parentThreadId ?? null,
      projectTag: null,
      assignedAgent: null,
      status: 'active',
      createdAt: new Date(),
      lastActiveAt: new Date(),
    };

    this.threads.set(thread.id, thread);

    // Link task to thread
    const task = this.getTask(params.taskId);
    task.threadId = thread.id;

    this.bus.publish('thread.created', 'task-graph', { thread });
    return thread;
  }

  getThread(threadId: string): Thread {
    const thread = this.threads.get(threadId);
    if (!thread) throw new Error(`Thread not found: ${threadId}`);
    return thread;
  }

  updateThreadStatus(threadId: string, status: Thread['status']): Thread {
    const thread = this.getThread(threadId);
    thread.status = status;
    thread.lastActiveAt = new Date();
    this.bus.publish('thread.status_changed', 'task-graph', { thread, status });
    return thread;
  }

  // ─── Queries ────────────────────────────────────────────────────

  queryTasks(filter: {
    status?: TaskStatus;
    assignedAgent?: string;
    source?: TaskSource;
    search?: string;
    parentId?: string | null;
  } = {}): Task[] {
    let results = Array.from(this.tasks.values());

    if (filter.status) {
      results = results.filter((t) => t.status === filter.status);
    }
    if (filter.assignedAgent) {
      results = results.filter((t) => t.assignedAgent === filter.assignedAgent);
    }
    if (filter.source) {
      results = results.filter((t) => t.source === filter.source);
    }
    if (filter.search) {
      const query = filter.search.toLowerCase();
      results = results.filter(
        (t) =>
          t.title.toLowerCase().includes(query) ||
          t.description.toLowerCase().includes(query)
      );
    }
    if (filter.parentId !== undefined) {
      results = results.filter((t) => t.parentId === filter.parentId);
    }

    return results.sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime());
  }

  getActiveTasks(): Task[] {
    return this.queryTasks({ status: 'in_progress' });
  }

  getBlockedTasks(): Task[] {
    return this.queryTasks({ status: 'blocked' });
  }

  getStaleTasks(olderThanMs: number = 30 * 60 * 1000): Task[] {
    const cutoff = new Date(Date.now() - olderThanMs);
    return Array.from(this.tasks.values()).filter(
      (t) =>
        (t.status === 'in_progress' || t.status === 'blocked') &&
        t.updatedAt < cutoff
    );
  }

  getAllTasks(): Task[] {
    return Array.from(this.tasks.values()).sort(
      (a, b) => b.createdAt.getTime() - a.createdAt.getTime()
    );
  }

  getTaskCount(): number {
    return this.tasks.size;
  }

  // ─── Internal ───────────────────────────────────────────────────

  private checkParentCompletion(parentId: string): void {
    const parent = this.tasks.get(parentId);
    if (!parent) return;

    const subtasks = this.getSubtasks(parentId);
    const allDone = subtasks.every(
      (t) => t.status === 'completed' || t.status === 'failed'
    );

    if (allDone) {
      const anyFailed = subtasks.some((t) => t.status === 'failed');
      this.updateStatus(
        parentId,
        anyFailed ? 'needs_review' : 'completed',
        anyFailed ? 'Some subtasks failed' : 'All subtasks completed'
      );
    }
  }
}
