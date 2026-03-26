/**
 * ApprovalStore — SQLite persistence for tool approval requests
 *
 * When an agent requests a Tier 2 or Tier 3 tool, ToolExecutor creates
 * an ApprovalRequest here before blocking. Resolution arrives via
 * approval.respond WS message which calls resolve().
 */

import Database from 'better-sqlite3';
import { v4 as uuid } from 'uuid';

export type ApprovalStatus = 'pending' | 'approved' | 'rejected';

export interface ApprovalRequest {
  id: string;
  taskId: string;
  agentId: string;
  toolName: string;
  toolTier: number;
  parameters: unknown;
  status: ApprovalStatus;
  rejectionReason: string | null;
  createdAt: Date;
  resolvedAt: Date | null;
}

export interface CreateApprovalInput {
  taskId: string;
  agentId: string;
  toolName: string;
  toolTier: number;
  parameters: unknown;
}

export class ApprovalStore {
  private db: Database.Database;

  constructor(db: Database.Database) {
    this.db = db;
  }

  initSchema(): void {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS approval_requests (
        id TEXT PRIMARY KEY,
        task_id TEXT NOT NULL,
        agent_id TEXT NOT NULL,
        tool_name TEXT NOT NULL,
        tool_tier INTEGER NOT NULL,
        parameters TEXT NOT NULL,
        status TEXT DEFAULT 'pending',
        rejection_reason TEXT,
        created_at TEXT DEFAULT (datetime('now')),
        resolved_at TEXT
      );

      CREATE INDEX IF NOT EXISTS idx_approvals_status ON approval_requests(status);
      CREATE INDEX IF NOT EXISTS idx_approvals_task ON approval_requests(task_id);
    `);
  }

  create(input: CreateApprovalInput): ApprovalRequest {
    const id = uuid();
    const now = new Date().toISOString();

    this.db.prepare(`
      INSERT INTO approval_requests (id, task_id, agent_id, tool_name, tool_tier, parameters, status, created_at)
      VALUES (?, ?, ?, ?, ?, ?, 'pending', ?)
    `).run(
      id,
      input.taskId,
      input.agentId,
      input.toolName,
      input.toolTier,
      JSON.stringify(input.parameters),
      now,
    );

    return {
      id,
      taskId: input.taskId,
      agentId: input.agentId,
      toolName: input.toolName,
      toolTier: input.toolTier,
      parameters: input.parameters,
      status: 'pending',
      rejectionReason: null,
      createdAt: new Date(now),
      resolvedAt: null,
    };
  }

  resolve(id: string, approved: boolean, rejectionReason?: string): ApprovalRequest {
    const now = new Date().toISOString();
    const status: ApprovalStatus = approved ? 'approved' : 'rejected';

    const result = this.db.prepare(`
      UPDATE approval_requests
      SET status = ?, rejection_reason = ?, resolved_at = ?
      WHERE id = ?
    `).run(status, rejectionReason ?? null, now, id);

    if (result.changes === 0) {
      throw new Error(`ApprovalRequest not found: ${id}`);
    }

    const row = this.db.prepare('SELECT * FROM approval_requests WHERE id = ?').get(id) as any;
    return this.rowToApproval(row);
  }

  getPending(): ApprovalRequest[] {
    const rows = this.db.prepare(
      `SELECT * FROM approval_requests WHERE status = 'pending' ORDER BY created_at ASC`
    ).all() as any[];
    return rows.map((r) => this.rowToApproval(r));
  }

  getForTask(taskId: string): ApprovalRequest[] {
    const rows = this.db.prepare(
      `SELECT * FROM approval_requests WHERE task_id = ? ORDER BY created_at ASC`
    ).all(taskId) as any[];
    return rows.map((r) => this.rowToApproval(r));
  }

  private rowToApproval(row: any): ApprovalRequest {
    return {
      id: row.id,
      taskId: row.task_id,
      agentId: row.agent_id,
      toolName: row.tool_name,
      toolTier: row.tool_tier,
      parameters: JSON.parse(row.parameters),
      status: row.status as ApprovalStatus,
      rejectionReason: row.rejection_reason ?? null,
      createdAt: new Date(row.created_at),
      resolvedAt: row.resolved_at ? new Date(row.resolved_at) : null,
    };
  }
}
