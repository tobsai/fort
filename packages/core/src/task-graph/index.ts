/**
 * Task Graph Engine — The Backbone
 *
 * Core invariant: every conversation creates a task.
 * Tasks are the atomic unit of transparency.
 */

import { v4 as uuid } from 'uuid';
import type { Task, TaskStatus, TaskSource, Thread, FortEvent } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';

export class TaskGraph {
  private tasks: Map<string, Task> = new Map();
  private threads: Map<string, Thread> = new Map();
  private bus: ModuleBus;

  constructor(bus: ModuleBus) {
    this.bus = bus;
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
    const task: Task = {
      id: uuid(),
      parentId: params.parentId ?? null,
      title: params.title,
      description: params.description ?? '',
      status: 'created',
      source: params.source,
      assignedAgent: params.assignedAgent ?? null,
      createdAt: new Date(),
      updatedAt: new Date(),
      completedAt: null,
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

  updateStatus(taskId: string, status: TaskStatus, reason?: string): Task {
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
