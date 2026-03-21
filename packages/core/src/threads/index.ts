/**
 * Thread Manager — Conversation Threading System
 *
 * All conversations support named threads — trackable conversation branches
 * that enable topic isolation. Threads and tasks are unified: every thread
 * is backed by a task, and every complex task can have a thread for ongoing
 * discussion.
 */

import Database from 'better-sqlite3';
import { v4 as uuid } from 'uuid';
import { existsSync, mkdirSync } from 'node:fs';
import { dirname } from 'node:path';
import type { ModuleBus } from '../module-bus/index.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { Thread, ThreadMessage, ThreadReference, DiagnosticResult } from '../types.js';

export interface CreateThreadOpts {
  name: string;
  description?: string;
  parentThreadId?: string;
  projectTag?: string;
  assignedAgent?: string;
}

export interface ListThreadsOpts {
  status?: Thread['status'];
  projectTag?: string;
  agentId?: string;
}

export interface GetMessagesOpts {
  limit?: number;
  before?: string;
}

export class ThreadManager {
  private db: Database.Database;
  private bus: ModuleBus;
  private taskGraph: TaskGraph;

  constructor(dbPath: string, bus: ModuleBus, taskGraph: TaskGraph) {
    const dir = dirname(dbPath);
    if (!existsSync(dir)) mkdirSync(dir, { recursive: true });

    this.db = new Database(dbPath);
    this.db.pragma('journal_mode = WAL');
    this.initializeSchema();

    this.bus = bus;
    this.taskGraph = taskGraph;
  }

  private initializeSchema(): void {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS threads (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        description TEXT NOT NULL DEFAULT '',
        task_id TEXT NOT NULL,
        parent_thread_id TEXT,
        project_tag TEXT,
        assigned_agent TEXT,
        status TEXT NOT NULL DEFAULT 'active',
        created_at TEXT NOT NULL,
        last_active_at TEXT NOT NULL
      );

      CREATE TABLE IF NOT EXISTS thread_messages (
        id TEXT PRIMARY KEY,
        thread_id TEXT NOT NULL,
        role TEXT NOT NULL,
        content TEXT NOT NULL,
        agent_id TEXT,
        created_at TEXT NOT NULL,
        FOREIGN KEY (thread_id) REFERENCES threads(id)
      );

      CREATE TABLE IF NOT EXISTS thread_references (
        id TEXT PRIMARY KEY,
        from_thread_id TEXT NOT NULL,
        to_thread_id TEXT NOT NULL,
        note TEXT NOT NULL DEFAULT '',
        created_at TEXT NOT NULL,
        FOREIGN KEY (from_thread_id) REFERENCES threads(id),
        FOREIGN KEY (to_thread_id) REFERENCES threads(id)
      );

      CREATE INDEX IF NOT EXISTS idx_thread_messages_thread_id ON thread_messages(thread_id);
      CREATE INDEX IF NOT EXISTS idx_thread_messages_created_at ON thread_messages(created_at);
      CREATE INDEX IF NOT EXISTS idx_threads_status ON threads(status);
      CREATE INDEX IF NOT EXISTS idx_threads_project_tag ON threads(project_tag);
      CREATE INDEX IF NOT EXISTS idx_threads_assigned_agent ON threads(assigned_agent);
      CREATE INDEX IF NOT EXISTS idx_thread_references_from ON thread_references(from_thread_id);
      CREATE INDEX IF NOT EXISTS idx_thread_references_to ON thread_references(to_thread_id);
    `);
  }

  createThread(opts: CreateThreadOpts): Thread {
    const id = uuid();
    const now = new Date();

    // Create backing task in TaskGraph
    const task = this.taskGraph.createTask({
      title: `Thread: ${opts.name}`,
      description: opts.description ?? '',
      source: 'user_chat',
      assignedAgent: opts.assignedAgent,
      threadId: id,
      metadata: {
        threadId: id,
        projectTag: opts.projectTag ?? null,
      },
    });

    // Update task status to in_progress
    this.taskGraph.updateStatus(task.id, 'in_progress');

    this.db.prepare(`
      INSERT INTO threads (id, name, description, task_id, parent_thread_id, project_tag, assigned_agent, status, created_at, last_active_at)
      VALUES (?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)
    `).run(
      id,
      opts.name,
      opts.description ?? '',
      task.id,
      opts.parentThreadId ?? null,
      opts.projectTag ?? null,
      opts.assignedAgent ?? null,
      now.toISOString(),
      now.toISOString(),
    );

    const thread = this.rowToThread({
      id,
      name: opts.name,
      description: opts.description ?? '',
      task_id: task.id,
      parent_thread_id: opts.parentThreadId ?? null,
      project_tag: opts.projectTag ?? null,
      assigned_agent: opts.assignedAgent ?? null,
      status: 'active',
      created_at: now.toISOString(),
      last_active_at: now.toISOString(),
    });

    this.bus.publish('thread.created', 'thread-manager', { thread });
    return thread;
  }

  addMessage(threadId: string, message: {
    role: 'user' | 'agent' | 'system';
    content: string;
    agentId?: string;
  }): ThreadMessage {
    // Verify thread exists
    this.getThread(threadId);

    const id = uuid();
    const now = new Date();

    this.db.prepare(`
      INSERT INTO thread_messages (id, thread_id, role, content, agent_id, created_at)
      VALUES (?, ?, ?, ?, ?, ?)
    `).run(id, threadId, message.role, message.content, message.agentId ?? null, now.toISOString());

    // Update last_active_at
    this.db.prepare(`UPDATE threads SET last_active_at = ? WHERE id = ?`).run(now.toISOString(), threadId);

    const msg: ThreadMessage = {
      id,
      threadId,
      role: message.role,
      content: message.content,
      agentId: message.agentId ?? null,
      createdAt: now,
    };

    this.bus.publish('thread.message', 'thread-manager', { threadId, message: msg });
    return msg;
  }

  getThread(threadId: string): Thread {
    const row = this.db.prepare(`SELECT * FROM threads WHERE id = ?`).get(threadId) as any;
    if (!row) throw new Error(`Thread not found: ${threadId}`);
    return this.rowToThread(row);
  }

  listThreads(opts?: ListThreadsOpts): Thread[] {
    let sql = 'SELECT * FROM threads WHERE 1=1';
    const params: unknown[] = [];

    if (opts?.status) {
      sql += ' AND status = ?';
      params.push(opts.status);
    }
    if (opts?.projectTag) {
      sql += ' AND project_tag = ?';
      params.push(opts.projectTag);
    }
    if (opts?.agentId) {
      sql += ' AND assigned_agent = ?';
      params.push(opts.agentId);
    }

    sql += ' ORDER BY last_active_at DESC';

    const rows = this.db.prepare(sql).all(...params) as any[];
    return rows.map((r) => this.rowToThread(r));
  }

  pauseThread(threadId: string): Thread {
    const thread = this.getThread(threadId);

    // Set task to blocked
    this.taskGraph.updateStatus(thread.taskId, 'blocked', 'Thread paused');

    const now = new Date();
    this.db.prepare(`UPDATE threads SET status = 'paused', last_active_at = ? WHERE id = ?`).run(now.toISOString(), threadId);

    const updated = this.getThread(threadId);
    this.bus.publish('thread.paused', 'thread-manager', { thread: updated });
    return updated;
  }

  resumeThread(threadId: string): Thread {
    const thread = this.getThread(threadId);

    // Re-open task
    this.taskGraph.updateStatus(thread.taskId, 'in_progress', 'Thread resumed');

    const now = new Date();
    this.db.prepare(`UPDATE threads SET status = 'active', last_active_at = ? WHERE id = ?`).run(now.toISOString(), threadId);

    const updated = this.getThread(threadId);
    this.bus.publish('thread.resumed', 'thread-manager', { thread: updated });
    return updated;
  }

  resolveThread(threadId: string): Thread {
    const thread = this.getThread(threadId);

    // Complete the task
    this.taskGraph.updateStatus(thread.taskId, 'completed', 'Thread resolved');

    const now = new Date();
    this.db.prepare(`UPDATE threads SET status = 'resolved', last_active_at = ? WHERE id = ?`).run(now.toISOString(), threadId);

    const updated = this.getThread(threadId);
    this.bus.publish('thread.resolved', 'thread-manager', { thread: updated });
    return updated;
  }

  forkThread(threadId: string, opts: { name: string; fromMessageId?: string }): Thread {
    const parent = this.getThread(threadId);

    // Create new thread with parent lineage
    const forked = this.createThread({
      name: opts.name,
      parentThreadId: threadId,
      projectTag: parent.projectTag ?? undefined,
      assignedAgent: parent.assignedAgent ?? undefined,
    });

    // If fromMessageId specified, copy messages up to and including that message
    if (opts.fromMessageId) {
      const sourceMsg = this.db.prepare(`SELECT rowid, * FROM thread_messages WHERE id = ? AND thread_id = ?`).get(opts.fromMessageId, threadId) as any;
      if (sourceMsg) {
        const messages = this.db.prepare(`
          SELECT * FROM thread_messages
          WHERE thread_id = ? AND rowid <= ?
          ORDER BY rowid ASC
        `).all(threadId, sourceMsg.rowid) as any[];

        for (const msg of messages) {
          const newId = uuid();
          this.db.prepare(`
            INSERT INTO thread_messages (id, thread_id, role, content, agent_id, created_at)
            VALUES (?, ?, ?, ?, ?, ?)
          `).run(newId, forked.id, msg.role, msg.content, msg.agent_id, msg.created_at);
        }
      }
    }

    this.bus.publish('thread.forked', 'thread-manager', {
      parentThreadId: threadId,
      forkedThread: forked,
      fromMessageId: opts.fromMessageId ?? null,
    });

    return forked;
  }

  getMessages(threadId: string, opts?: GetMessagesOpts): ThreadMessage[] {
    // Verify thread exists
    this.getThread(threadId);

    let sql = 'SELECT * FROM thread_messages WHERE thread_id = ?';
    const params: unknown[] = [threadId];

    if (opts?.before) {
      const beforeMsg = this.db.prepare(`SELECT created_at FROM thread_messages WHERE id = ?`).get(opts.before) as any;
      if (beforeMsg) {
        sql += ' AND created_at < ?';
        params.push(beforeMsg.created_at);
      }
    }

    sql += ' ORDER BY created_at ASC';

    if (opts?.limit) {
      sql += ' LIMIT ?';
      params.push(opts.limit);
    }

    const rows = this.db.prepare(sql).all(...params) as any[];
    return rows.map((r) => this.rowToMessage(r));
  }

  searchThreads(query: string): Thread[] {
    const like = `%${query}%`;

    // Search in thread names, descriptions, and message content
    const rows = this.db.prepare(`
      SELECT DISTINCT t.* FROM threads t
      LEFT JOIN thread_messages m ON m.thread_id = t.id
      WHERE t.name LIKE ? OR t.description LIKE ? OR m.content LIKE ?
      ORDER BY t.last_active_at DESC
    `).all(like, like, like) as any[];

    return rows.map((r) => this.rowToThread(r));
  }

  crossReference(fromThreadId: string, toThreadId: string, note?: string): ThreadReference {
    // Verify both threads exist
    this.getThread(fromThreadId);
    this.getThread(toThreadId);

    const id = uuid();
    const now = new Date();

    this.db.prepare(`
      INSERT INTO thread_references (id, from_thread_id, to_thread_id, note, created_at)
      VALUES (?, ?, ?, ?, ?)
    `).run(id, fromThreadId, toThreadId, note ?? '', now.toISOString());

    return {
      id,
      fromThreadId,
      toThreadId,
      note: note ?? '',
      createdAt: now,
    };
  }

  getReferences(threadId: string): ThreadReference[] {
    const rows = this.db.prepare(`
      SELECT * FROM thread_references
      WHERE from_thread_id = ? OR to_thread_id = ?
      ORDER BY created_at DESC
    `).all(threadId, threadId) as any[];

    return rows.map((r) => ({
      id: r.id,
      fromThreadId: r.from_thread_id,
      toThreadId: r.to_thread_id,
      note: r.note,
      createdAt: new Date(r.created_at),
    }));
  }

  diagnose(): DiagnosticResult {
    const checks = [];

    try {
      const allThreads = this.listThreads();
      const active = allThreads.filter((t) => t.status === 'active');
      const paused = allThreads.filter((t) => t.status === 'paused');
      const resolved = allThreads.filter((t) => t.status === 'resolved');

      checks.push({
        name: 'Thread database',
        passed: true,
        message: `${allThreads.length} total threads`,
      });

      checks.push({
        name: 'Active threads',
        passed: true,
        message: `${active.length} active, ${paused.length} paused, ${resolved.length} resolved`,
      });

      const msgCount = (this.db.prepare(`SELECT COUNT(*) as count FROM thread_messages`).get() as any).count;
      checks.push({
        name: 'Messages',
        passed: true,
        message: `${msgCount} total messages`,
      });

      const refCount = (this.db.prepare(`SELECT COUNT(*) as count FROM thread_references`).get() as any).count;
      checks.push({
        name: 'Cross-references',
        passed: true,
        message: `${refCount} cross-references`,
      });
    } catch (err) {
      checks.push({
        name: 'Thread database',
        passed: false,
        message: `Error: ${err instanceof Error ? err.message : String(err)}`,
      });
    }

    return {
      module: 'threads',
      status: checks.every((c) => c.passed) ? 'healthy' : 'degraded',
      checks,
    };
  }

  close(): void {
    this.db.close();
  }

  private rowToThread(row: any): Thread {
    return {
      id: row.id,
      name: row.name,
      description: row.description,
      taskId: row.task_id,
      parentThreadId: row.parent_thread_id,
      projectTag: row.project_tag,
      assignedAgent: row.assigned_agent,
      status: row.status,
      createdAt: new Date(row.created_at),
      lastActiveAt: new Date(row.last_active_at),
    };
  }

  private rowToMessage(row: any): ThreadMessage {
    return {
      id: row.id,
      threadId: row.thread_id,
      role: row.role,
      content: row.content,
      agentId: row.agent_id,
      createdAt: new Date(row.created_at),
    };
  }
}
