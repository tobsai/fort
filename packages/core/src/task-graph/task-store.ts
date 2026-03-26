/**
 * TaskStore — SQLite-backed persistence for the TaskGraph.
 *
 * Writes every task create/update to SQLite so the task history
 * survives server restarts. Hot-path reads come from the in-memory
 * Map in TaskGraph; this store is only queried for history lookups
 * and on startup hydration.
 */

import Database from 'better-sqlite3';
import type { Task, TaskStatus, TaskSource } from '../types.js';

export interface TaskQuery {
  status?: TaskStatus | TaskStatus[];
  assignedAgent?: string;
  since?: Date;
  limit?: number;
  offset?: number;
}

export class TaskStore {
  constructor(private db: InstanceType<typeof Database>) {}

  initSchema(): void {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS tasks (
        id TEXT PRIMARY KEY,
        short_id TEXT NOT NULL DEFAULT '',
        title TEXT NOT NULL,
        description TEXT,
        status TEXT NOT NULL DEFAULT 'pending',
        source TEXT,
        assigned_agent TEXT,
        assigned_to TEXT,
        parent_id TEXT,
        result TEXT,
        inability_reason TEXT,
        subtask_ids TEXT NOT NULL DEFAULT '[]',
        thread_id TEXT,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        completed_at TEXT,
        metadata TEXT
      );

      CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
      CREATE INDEX IF NOT EXISTS idx_tasks_agent ON tasks(assigned_agent);
      CREATE INDEX IF NOT EXISTS idx_tasks_created ON tasks(created_at);
      CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_id);
    `);

    // Add source_agent_id column if it doesn't exist (migration for existing DBs)
    try {
      this.db.exec('ALTER TABLE tasks ADD COLUMN source_agent_id TEXT');
    } catch {
      // Column already exists — ignore
    }
  }

  upsertTask(task: Task): void {
    (this.db.prepare(`
      INSERT OR REPLACE INTO tasks (
        id, short_id, title, description, status, source,
        assigned_agent, assigned_to, parent_id, source_agent_id, result, inability_reason,
        subtask_ids, thread_id, created_at, updated_at, completed_at, metadata
      ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `) as any).run(
      task.id,
      task.shortId,
      task.title,
      task.description ?? null,
      task.status,
      task.source,
      task.assignedAgent ?? null,
      task.assignedTo ?? null,
      task.parentId ?? null,
      task.sourceAgentId ?? null,
      task.result ?? null,
      (task.metadata?.statusReason as string) ?? null,
      JSON.stringify(task.subtaskIds ?? []),
      task.threadId ?? null,
      task.createdAt.toISOString(),
      task.updatedAt.toISOString(),
      task.completedAt?.toISOString() ?? null,
      JSON.stringify(task.metadata ?? {}),
    );
  }

  loadAll(): Task[] {
    const rows = (this.db.prepare('SELECT * FROM tasks ORDER BY created_at ASC') as any).all() as Record<string, unknown>[];
    return rows.map((row) => {
      const task = this.rowToTask(row);
      // Reset in_progress → pending (orphaned by restart)
      if (task.status === 'in_progress') {
        task.status = 'pending';
        task.updatedAt = new Date();
      }
      return task;
    });
  }

  queryTasks(query: TaskQuery): Task[] {
    const conditions: string[] = [];
    const params: unknown[] = [];

    if (query.status !== undefined) {
      if (Array.isArray(query.status)) {
        const placeholders = query.status.map(() => '?').join(', ');
        conditions.push(`status IN (${placeholders})`);
        params.push(...query.status);
      } else {
        conditions.push('status = ?');
        params.push(query.status);
      }
    }

    if (query.assignedAgent !== undefined) {
      conditions.push('assigned_agent = ?');
      params.push(query.assignedAgent);
    }

    if (query.since !== undefined) {
      conditions.push('created_at >= ?');
      params.push(query.since.toISOString());
    }

    const where = conditions.length > 0 ? `WHERE ${conditions.join(' AND ')}` : '';
    const limit = query.limit !== undefined ? `LIMIT ${query.limit}` : '';
    const offset = query.offset !== undefined ? `OFFSET ${query.offset}` : '';

    const sql = `SELECT * FROM tasks ${where} ORDER BY created_at DESC ${limit} ${offset}`.trim();
    const rows = (this.db.prepare(sql) as any).all(...params) as Record<string, unknown>[];
    return rows.map((row) => this.rowToTask(row));
  }

  getSubtasks(parentId: string): Task[] {
    const rows = (this.db.prepare('SELECT * FROM tasks WHERE parent_id = ? ORDER BY created_at ASC') as any).all(parentId) as Record<string, unknown>[];
    return rows.map((row) => this.rowToTask(row));
  }

  private rowToTask(row: Record<string, unknown>): Task {
    const metadata = this.parseJson(row['metadata'] as string, {});
    return {
      id: row['id'] as string,
      shortId: (row['short_id'] as string) || '',
      title: row['title'] as string,
      description: (row['description'] as string) ?? '',
      status: row['status'] as TaskStatus,
      source: row['source'] as TaskSource,
      assignedAgent: (row['assigned_agent'] as string | null) ?? null,
      assignedTo: (row['assigned_to'] as 'agent' | 'user' | null) ?? null,
      parentId: (row['parent_id'] as string | null) ?? null,
      sourceAgentId: (row['source_agent_id'] as string | null) ?? null,
      result: (row['result'] as string | null) ?? null,
      subtaskIds: this.parseJson(row['subtask_ids'] as string, []),
      threadId: (row['thread_id'] as string | null) ?? null,
      createdAt: new Date(row['created_at'] as string),
      updatedAt: new Date(row['updated_at'] as string),
      completedAt: row['completed_at'] ? new Date(row['completed_at'] as string) : null,
      metadata,
    };
  }

  private parseJson<T>(value: string | null | undefined, fallback: T): T {
    if (!value) return fallback;
    try {
      return JSON.parse(value) as T;
    } catch {
      return fallback;
    }
  }
}
