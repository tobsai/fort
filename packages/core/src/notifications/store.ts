/**
 * NotificationStore — SQLite persistence for in-app notifications
 */

import Database from 'better-sqlite3';
import { v4 as uuid } from 'uuid';

export type NotificationType =
  | 'task.completed'
  | 'task.failed'
  | 'approval.required'
  | 'agent.started'
  | 'agent.stopped';

export interface Notification {
  id: string;
  type: NotificationType;
  title: string;
  body: string | null;
  entityType: string | null;
  entityId: string | null;
  read: boolean;
  createdAt: Date;
}

export interface CreateNotificationInput {
  type: NotificationType;
  title: string;
  body?: string;
  entityType?: string;
  entityId?: string;
}

export class NotificationStore {
  private db: Database.Database;

  constructor(db: Database.Database) {
    this.db = db;
  }

  initSchema(): void {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS notifications (
        id TEXT PRIMARY KEY,
        type TEXT NOT NULL,
        title TEXT NOT NULL,
        body TEXT,
        entity_type TEXT,
        entity_id TEXT,
        read INTEGER DEFAULT 0,
        created_at TEXT DEFAULT (datetime('now'))
      );

      CREATE INDEX IF NOT EXISTS idx_notifications_read ON notifications(read);
      CREATE INDEX IF NOT EXISTS idx_notifications_created ON notifications(created_at);
    `);
  }

  create(input: CreateNotificationInput): Notification {
    const id = uuid();
    const now = new Date().toISOString();

    this.db.prepare(`
      INSERT INTO notifications (id, type, title, body, entity_type, entity_id, read, created_at)
      VALUES (?, ?, ?, ?, ?, ?, 0, ?)
    `).run(
      id,
      input.type,
      input.title,
      input.body ?? null,
      input.entityType ?? null,
      input.entityId ?? null,
      now,
    );

    return {
      id,
      type: input.type,
      title: input.title,
      body: input.body ?? null,
      entityType: input.entityType ?? null,
      entityId: input.entityId ?? null,
      read: false,
      createdAt: new Date(now),
    };
  }

  markRead(id: string): void {
    this.db.prepare(`UPDATE notifications SET read = 1 WHERE id = ?`).run(id);
  }

  markAllRead(): void {
    this.db.prepare(`UPDATE notifications SET read = 1`).run();
  }

  list(options?: { unreadOnly?: boolean; limit?: number }): Notification[] {
    let sql = 'SELECT * FROM notifications';
    const params: unknown[] = [];

    if (options?.unreadOnly) {
      sql += ' WHERE read = 0';
    }

    sql += ' ORDER BY created_at DESC';

    if (options?.limit) {
      sql += ' LIMIT ?';
      params.push(options.limit);
    }

    const rows = this.db.prepare(sql).all(...params) as any[];
    return rows.map((r) => this.rowToNotification(r));
  }

  getUnreadCount(): number {
    const row = this.db.prepare(`SELECT COUNT(*) as count FROM notifications WHERE read = 0`).get() as any;
    return row.count;
  }

  private rowToNotification(row: any): Notification {
    return {
      id: row.id,
      type: row.type as NotificationType,
      title: row.title,
      body: row.body ?? null,
      entityType: row.entity_type ?? null,
      entityId: row.entity_id ?? null,
      read: row.read === 1,
      createdAt: new Date(row.created_at),
    };
  }
}
