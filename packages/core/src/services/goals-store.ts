/**
 * GoalsStore — SQLite-backed persistence for structured Goals.
 *
 * Goals are first-class objects that tasks tag against and that the
 * Reflection service periodically reviews. The agent reads active
 * goals via this store on every user-facing chat (see LLMClient
 * buildSystemPrompt) so responses feel personally addressed.
 */

import Database from 'better-sqlite3';
import type { Goal, GoalStatus, GoalSource } from '../types.js';

export interface GoalQuery {
  agentId?: string;
  status?: GoalStatus | GoalStatus[];
  limit?: number;
}

const CREATE_TABLE_SQL = `CREATE TABLE IF NOT EXISTS goals (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT,
  status TEXT NOT NULL,
  source TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  last_activity_at TEXT,
  last_nudge_at TEXT
)`;

const CREATE_INDEX_SQL = `CREATE INDEX IF NOT EXISTS idx_goals_agent_status ON goals(agent_id, status)`;

// Migration for goals tables created before last_nudge_at existed.
const MIGRATE_LAST_NUDGE_SQL = `ALTER TABLE goals ADD COLUMN last_nudge_at TEXT`;

export class GoalsStore {
  constructor(private db: InstanceType<typeof Database>) {}

  initSchema(): void {
    (this.db.prepare(CREATE_TABLE_SQL) as any).run();
    (this.db.prepare(CREATE_INDEX_SQL) as any).run();
    // Best-effort migration for pre-existing tables.
    try {
      (this.db.prepare(MIGRATE_LAST_NUDGE_SQL) as any).run();
    } catch {
      // Column already exists — ignore.
    }
  }

  upsert(goal: Goal): void {
    (this.db.prepare(
      `INSERT OR REPLACE INTO goals
        (id, agent_id, title, description, status, source, created_at, updated_at, last_activity_at, last_nudge_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    ) as any).run(
      goal.id,
      goal.agentId,
      goal.title,
      goal.description ?? null,
      goal.status,
      goal.source,
      goal.createdAt.toISOString(),
      goal.updatedAt.toISOString(),
      goal.lastActivityAt?.toISOString() ?? null,
      goal.lastNudgeAt?.toISOString() ?? null,
    );
  }

  /** Mark a nudge fired for this goal (used by Reflection for cooldown). */
  markNudged(id: string, when: Date = new Date()): void {
    (this.db.prepare('UPDATE goals SET last_nudge_at = ? WHERE id = ?') as any).run(
      when.toISOString(),
      id,
    );
  }

  get(id: string): Goal | null {
    const row = (this.db.prepare('SELECT * FROM goals WHERE id = ?') as any).get(id) as
      | Record<string, unknown>
      | undefined;
    return row ? this.rowToGoal(row) : null;
  }

  query(q: GoalQuery): Goal[] {
    const conditions: string[] = [];
    const params: unknown[] = [];

    if (q.agentId !== undefined) {
      conditions.push('agent_id = ?');
      params.push(q.agentId);
    }
    if (q.status !== undefined) {
      if (Array.isArray(q.status)) {
        const placeholders = q.status.map(() => '?').join(', ');
        conditions.push(`status IN (${placeholders})`);
        params.push(...q.status);
      } else {
        conditions.push('status = ?');
        params.push(q.status);
      }
    }

    const where = conditions.length ? `WHERE ${conditions.join(' AND ')}` : '';
    const limit = q.limit !== undefined ? `LIMIT ${q.limit}` : '';
    const sql = `SELECT * FROM goals ${where} ORDER BY updated_at DESC ${limit}`.trim();
    const rows = (this.db.prepare(sql) as any).all(...params) as Record<string, unknown>[];
    return rows.map((r) => this.rowToGoal(r));
  }

  delete(id: string): boolean {
    const result = (this.db.prepare('DELETE FROM goals WHERE id = ?') as any).run(id) as {
      changes: number;
    };
    return result.changes > 0;
  }

  touch(id: string, when: Date = new Date()): void {
    (this.db.prepare('UPDATE goals SET last_activity_at = ?, updated_at = ? WHERE id = ?') as any).run(
      when.toISOString(),
      when.toISOString(),
      id,
    );
  }

  private rowToGoal(row: Record<string, unknown>): Goal {
    return {
      id: row['id'] as string,
      agentId: row['agent_id'] as string,
      title: row['title'] as string,
      description: (row['description'] as string | null) ?? null,
      status: row['status'] as GoalStatus,
      source: row['source'] as GoalSource,
      createdAt: new Date(row['created_at'] as string),
      updatedAt: new Date(row['updated_at'] as string),
      lastActivityAt: row['last_activity_at']
        ? new Date(row['last_activity_at'] as string)
        : null,
      lastNudgeAt: row['last_nudge_at']
        ? new Date(row['last_nudge_at'] as string)
        : null,
    };
  }
}
