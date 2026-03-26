/**
 * SchedulerStore — SQLite persistence for scheduled tasks (SPEC-008)
 */

import { v4 as uuid } from 'uuid';
import Database from 'better-sqlite3';

export interface ScheduleConfig {
  name: string;
  description?: string;
  agentId: string;
  scheduleType: 'cron' | 'interval';
  scheduleValue: string;  // cron expr OR '30m', '1h', '6h', '1d'
  taskTitle: string;
  taskDescription?: string;
}

export interface ScheduledTask {
  id: string;
  name: string;
  description: string;
  agentId: string;
  scheduleType: 'cron' | 'interval';
  scheduleValue: string;
  taskTitle: string;
  taskDescription: string;
  enabled: boolean;
  lastRunAt: Date | null;
  nextRunAt: Date | null;
  runCount: number;
  createdAt: Date;
  updatedAt: Date;
}

export class SchedulerStore {
  private db: InstanceType<typeof Database>;

  constructor(db: InstanceType<typeof Database>) {
    this.db = db;
  }

  initSchema(): void {
    (this.db as any).exec(`
      CREATE TABLE IF NOT EXISTS scheduled_tasks (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        description TEXT DEFAULT '',
        agent_id TEXT NOT NULL,
        schedule_type TEXT NOT NULL,
        schedule_value TEXT NOT NULL,
        task_title TEXT NOT NULL,
        task_description TEXT DEFAULT '',
        enabled INTEGER DEFAULT 1,
        last_run_at TEXT,
        next_run_at TEXT,
        run_count INTEGER DEFAULT 0,
        created_at TEXT DEFAULT (datetime('now')),
        updated_at TEXT DEFAULT (datetime('now'))
      )
    `);
  }

  createSchedule(config: ScheduleConfig): ScheduledTask {
    const id = uuid();
    const now = new Date();
    (this.db as any).prepare(`
      INSERT INTO scheduled_tasks
        (id, name, description, agent_id, schedule_type, schedule_value, task_title, task_description, enabled, run_count, created_at, updated_at)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, 0, ?, ?)
    `).run(
      id,
      config.name,
      config.description ?? '',
      config.agentId,
      config.scheduleType,
      config.scheduleValue,
      config.taskTitle,
      config.taskDescription ?? '',
      now.toISOString(),
      now.toISOString(),
    );
    return this.getSchedule(id)!;
  }

  updateSchedule(id: string, updates: Partial<ScheduleConfig>): ScheduledTask {
    const fields: string[] = [];
    const values: unknown[] = [];
    if (updates.name !== undefined) { fields.push('name = ?'); values.push(updates.name); }
    if (updates.description !== undefined) { fields.push('description = ?'); values.push(updates.description); }
    if (updates.agentId !== undefined) { fields.push('agent_id = ?'); values.push(updates.agentId); }
    if (updates.scheduleType !== undefined) { fields.push('schedule_type = ?'); values.push(updates.scheduleType); }
    if (updates.scheduleValue !== undefined) { fields.push('schedule_value = ?'); values.push(updates.scheduleValue); }
    if (updates.taskTitle !== undefined) { fields.push('task_title = ?'); values.push(updates.taskTitle); }
    if (updates.taskDescription !== undefined) { fields.push('task_description = ?'); values.push(updates.taskDescription); }
    fields.push('updated_at = ?');
    values.push(new Date().toISOString());
    values.push(id);
    (this.db as any).prepare(`UPDATE scheduled_tasks SET ${fields.join(', ')} WHERE id = ?`).run(...values);
    return this.getSchedule(id)!;
  }

  deleteSchedule(id: string): void {
    (this.db as any).prepare('DELETE FROM scheduled_tasks WHERE id = ?').run(id);
  }

  getSchedule(id: string): ScheduledTask | null {
    const row = (this.db as any).prepare('SELECT * FROM scheduled_tasks WHERE id = ?').get(id) as any;
    if (!row) return null;
    return this.rowToSchedule(row);
  }

  listSchedules(): ScheduledTask[] {
    const rows = (this.db as any).prepare('SELECT * FROM scheduled_tasks ORDER BY created_at ASC').all() as any[];
    return rows.map((r) => this.rowToSchedule(r));
  }

  updateRunInfo(id: string, lastRunAt: Date, nextRunAt: Date): void {
    (this.db as any).prepare(`
      UPDATE scheduled_tasks
      SET last_run_at = ?, next_run_at = ?, run_count = run_count + 1, updated_at = ?
      WHERE id = ?
    `).run(lastRunAt.toISOString(), nextRunAt.toISOString(), new Date().toISOString(), id);
  }

  setEnabled(id: string, enabled: boolean): void {
    (this.db as any).prepare('UPDATE scheduled_tasks SET enabled = ?, updated_at = ? WHERE id = ?')
      .run(enabled ? 1 : 0, new Date().toISOString(), id);
  }

  setNextRunAt(id: string, nextRunAt: Date): void {
    (this.db as any).prepare('UPDATE scheduled_tasks SET next_run_at = ?, updated_at = ? WHERE id = ?')
      .run(nextRunAt.toISOString(), new Date().toISOString(), id);
  }

  private rowToSchedule(row: any): ScheduledTask {
    return {
      id: row.id,
      name: row.name,
      description: row.description ?? '',
      agentId: row.agent_id,
      scheduleType: row.schedule_type as 'cron' | 'interval',
      scheduleValue: row.schedule_value,
      taskTitle: row.task_title,
      taskDescription: row.task_description ?? '',
      enabled: Boolean(row.enabled),
      lastRunAt: row.last_run_at ? new Date(row.last_run_at) : null,
      nextRunAt: row.next_run_at ? new Date(row.next_run_at) : null,
      runCount: row.run_count,
      createdAt: new Date(row.created_at),
      updatedAt: new Date(row.updated_at),
    };
  }
}
