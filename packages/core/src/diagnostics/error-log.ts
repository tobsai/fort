/**
 * ErrorLog — in-memory ring buffer + SQLite persistence for Fort errors.
 *
 * Keeps the last 100 errors in memory for fast access.
 * All entries are also persisted to the DB so they survive restarts.
 */

import Database from 'better-sqlite3';
import { v4 as uuid } from 'uuid';

export interface ErrorEntry {
  id: string;
  subsystem: string;
  message: string;
  stack: string | null;
  context: Record<string, unknown> | null;
  createdAt: Date;
}

const RING_SIZE = 100;

export class ErrorLog {
  private ring: ErrorEntry[] = [];
  private db: InstanceType<typeof Database> | null = null;

  constructor(db?: InstanceType<typeof Database>) {
    if (db) {
      this.db = db;
      this.initSchema();
    }
  }

  private initSchema(): void {
    (this.db as any).exec(`
      CREATE TABLE IF NOT EXISTS error_log (
        id TEXT PRIMARY KEY,
        subsystem TEXT NOT NULL,
        message TEXT NOT NULL,
        stack TEXT,
        context TEXT,
        created_at TEXT NOT NULL
      );
      CREATE INDEX IF NOT EXISTS idx_error_log_subsystem ON error_log(subsystem);
      CREATE INDEX IF NOT EXISTS idx_error_log_created ON error_log(created_at);
    `);
  }

  logError(subsystem: string, error: Error | string, context?: Record<string, unknown>): void {
    const entry: ErrorEntry = {
      id: uuid(),
      subsystem,
      message: error instanceof Error ? error.message : String(error),
      stack: error instanceof Error ? (error.stack ?? null) : null,
      context: context ?? null,
      createdAt: new Date(),
    };

    // Ring buffer — newest at front
    this.ring.unshift(entry);
    if (this.ring.length > RING_SIZE) {
      this.ring.pop();
    }

    // Persist to DB
    if (this.db) {
      try {
        (this.db as any).prepare(`
          INSERT INTO error_log (id, subsystem, message, stack, context, created_at)
          VALUES (?, ?, ?, ?, ?, ?)
        `).run(
          entry.id,
          entry.subsystem,
          entry.message,
          entry.stack,
          entry.context ? JSON.stringify(entry.context) : null,
          entry.createdAt.toISOString(),
        );
      } catch {
        // Non-fatal: ring buffer still holds the entry
      }
    }
  }

  getRecent(limit: number = 50): ErrorEntry[] {
    return this.ring.slice(0, limit);
  }

  getBySubsystem(subsystem: string): ErrorEntry[] {
    return this.ring.filter((e) => e.subsystem === subsystem);
  }

  /** Load recent entries from DB into the ring (call once at startup). */
  loadFromDb(): void {
    if (!this.db) return;
    try {
      const rows = (this.db as any).prepare(
        `SELECT * FROM error_log ORDER BY created_at DESC LIMIT ${RING_SIZE}`
      ).all() as Record<string, unknown>[];
      this.ring = rows.map((row) => ({
        id: row['id'] as string,
        subsystem: row['subsystem'] as string,
        message: row['message'] as string,
        stack: (row['stack'] as string | null) ?? null,
        context: row['context'] ? JSON.parse(row['context'] as string) : null,
        createdAt: new Date(row['created_at'] as string),
      }));
    } catch {
      // Non-fatal
    }
  }
}
