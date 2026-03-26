/**
 * AgentMemoryStore — Persistent per-agent memory with FTS5 keyword search
 *
 * Each agent accumulates memories across sessions. Memories are categorized
 * (fact / decision / preference / observation) and searchable via FTS5.
 *
 * Schema lives in the shared tasks.db (passed in as Database instance).
 */

import { v4 as uuid } from 'uuid';
import type Database from 'better-sqlite3';

export type MemoryCategory = 'fact' | 'decision' | 'preference' | 'observation';

export interface MemoryEntry {
  category: MemoryCategory;
  content: string;
  tags?: string[];
  taskId?: string;
}

export interface AgentMemory {
  id: string;
  agentId: string;
  category: MemoryCategory;
  content: string;
  tags: string[];
  taskId: string | null;
  createdAt: string;
  updatedAt: string;
}

export class AgentMemoryStore {
  private db: Database.Database;

  constructor(db: Database.Database) {
    this.db = db;
  }

  initSchema(): void {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS agent_memories (
        id TEXT PRIMARY KEY,
        agent_id TEXT NOT NULL,
        category TEXT NOT NULL,
        content TEXT NOT NULL,
        tags TEXT NOT NULL DEFAULT '[]',
        task_id TEXT,
        created_at TEXT DEFAULT (datetime('now')),
        updated_at TEXT DEFAULT (datetime('now'))
      );

      CREATE INDEX IF NOT EXISTS idx_memories_agent ON agent_memories(agent_id);
      CREATE INDEX IF NOT EXISTS idx_memories_category ON agent_memories(agent_id, category);

      CREATE VIRTUAL TABLE IF NOT EXISTS agent_memories_fts USING fts5(
        content,
        tags,
        agent_id UNINDEXED,
        content='agent_memories',
        content_rowid='rowid'
      );

      CREATE TRIGGER IF NOT EXISTS agent_memories_ai AFTER INSERT ON agent_memories BEGIN
        INSERT INTO agent_memories_fts(rowid, content, tags, agent_id)
          VALUES (new.rowid, new.content, new.tags, new.agent_id);
      END;

      CREATE TRIGGER IF NOT EXISTS agent_memories_ad AFTER DELETE ON agent_memories BEGIN
        INSERT INTO agent_memories_fts(agent_memories_fts, rowid, content, tags, agent_id)
          VALUES ('delete', old.rowid, old.content, old.tags, old.agent_id);
      END;

      CREATE TRIGGER IF NOT EXISTS agent_memories_au AFTER UPDATE ON agent_memories BEGIN
        INSERT INTO agent_memories_fts(agent_memories_fts, rowid, content, tags, agent_id)
          VALUES ('delete', old.rowid, old.content, old.tags, old.agent_id);
        INSERT INTO agent_memories_fts(rowid, content, tags, agent_id)
          VALUES (new.rowid, new.content, new.tags, new.agent_id);
      END;
    `);
  }

  remember(agentId: string, entry: MemoryEntry): AgentMemory {
    const id = uuid();
    const now = new Date().toISOString();
    const tags = entry.tags ?? [];
    const tagsJson = JSON.stringify(tags);

    this.db.prepare(`
      INSERT INTO agent_memories (id, agent_id, category, content, tags, task_id, created_at, updated_at)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `).run(id, agentId, entry.category, entry.content, tagsJson, entry.taskId ?? null, now, now);

    return {
      id,
      agentId,
      category: entry.category,
      content: entry.content,
      tags,
      taskId: entry.taskId ?? null,
      createdAt: now,
      updatedAt: now,
    };
  }

  recall(
    agentId: string,
    query: string,
    options?: { limit?: number; category?: MemoryCategory },
  ): AgentMemory[] {
    const limit = options?.limit ?? 10;

    // Build FTS5 query: tokenize into individual terms joined with OR so that
    // a multi-word task title (e.g. "Tell me about PostgreSQL") matches memories
    // containing any of those keywords.
    const terms = query
      .toLowerCase()
      .replace(/[^\w\s]/g, ' ')
      .split(/\s+/)
      .filter((t) => t.length > 2)
      .map((t) => `"${t}"`)
      .join(' OR ');

    // Try FTS5 MATCH first
    if (terms.length > 0) {
      try {
        const categoryClause = options?.category ? 'AND m.category = ?' : '';
        const ftsParams: unknown[] = options?.category
          ? [terms, agentId, options.category, limit]
          : [terms, agentId, limit];

        const rows = this.db.prepare(`
          SELECT m.*
          FROM agent_memories m
          JOIN agent_memories_fts fts ON m.rowid = fts.rowid
          WHERE agent_memories_fts MATCH ?
            AND m.agent_id = ?
            ${categoryClause}
          ORDER BY m.created_at DESC
          LIMIT ?
        `).all(...ftsParams) as any[];

        if (rows.length > 0) {
          return rows.map(this.rowToMemory);
        }
      } catch {
        // FTS error — fall through to LIKE fallback
      }
    }

    // LIKE fallback: search each meaningful term individually, union results
    const meaningfulTerms = query
      .split(/\s+/)
      .filter((t) => t.length > 2)
      .slice(0, 5);

    if (meaningfulTerms.length === 0) {
      return [];
    }

    const likeConditions = meaningfulTerms
      .map(() => '(content LIKE ? OR tags LIKE ?)')
      .join(' OR ');
    const likeValues = meaningfulTerms.flatMap((t) => {
      const p = `%${t}%`;
      return [p, p];
    });

    const categoryClause = options?.category ? 'AND category = ?' : '';
    const params: unknown[] = options?.category
      ? [agentId, ...likeValues, options.category, limit]
      : [agentId, ...likeValues, limit];

    const rows = this.db.prepare(`
      SELECT *
      FROM agent_memories
      WHERE agent_id = ?
        AND (${likeConditions})
        ${categoryClause}
      ORDER BY created_at DESC
      LIMIT ?
    `).all(...params) as any[];

    return rows.map(this.rowToMemory);
  }

  list(
    agentId: string,
    options?: { category?: MemoryCategory; limit?: number },
  ): AgentMemory[] {
    const limit = options?.limit ?? 100;

    if (options?.category) {
      const rows = this.db.prepare(`
        SELECT * FROM agent_memories
        WHERE agent_id = ? AND category = ?
        ORDER BY created_at DESC
        LIMIT ?
      `).all(agentId, options.category, limit) as any[];
      return rows.map(this.rowToMemory);
    }

    const rows = this.db.prepare(`
      SELECT * FROM agent_memories
      WHERE agent_id = ?
      ORDER BY created_at DESC
      LIMIT ?
    `).all(agentId, limit) as any[];

    return rows.map(this.rowToMemory);
  }

  delete(id: string): void {
    this.db.prepare('DELETE FROM agent_memories WHERE id = ?').run(id);
  }

  clear(agentId: string): void {
    this.db.prepare('DELETE FROM agent_memories WHERE agent_id = ?').run(agentId);
  }

  private rowToMemory(row: any): AgentMemory {
    return {
      id: row.id,
      agentId: row.agent_id,
      category: row.category as MemoryCategory,
      content: row.content,
      tags: JSON.parse(row.tags ?? '[]'),
      taskId: row.task_id ?? null,
      createdAt: row.created_at,
      updatedAt: row.updated_at,
    };
  }
}
