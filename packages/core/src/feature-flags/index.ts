/**
 * Feature Flags — Safe Self-Modification for Fort
 *
 * When Fort modifies its own code or config, changes are applied behind
 * a feature flag. Fort monitors for degradation during a "bake period".
 * If problems detected: automatic rollback, flag disabled.
 * If stable: flag removed, change is permanent.
 */

import Database from 'better-sqlite3';
import { v4 as uuid } from 'uuid';
import { existsSync, mkdirSync } from 'node:fs';
import { dirname } from 'node:path';
import type { ModuleBus } from '../module-bus/index.js';
import type { FeatureFlag, FlagCheck, DiagnosticResult } from '../types.js';

const DEFAULT_BAKE_PERIOD_MS = 24 * 60 * 60 * 1000; // 24 hours

export class FeatureFlagManager {
  private db: Database.Database;
  private bus: ModuleBus;

  constructor(dbPath: string, bus: ModuleBus) {
    const dir = dirname(dbPath);
    if (!existsSync(dir)) mkdirSync(dir, { recursive: true });

    this.db = new Database(dbPath);
    this.db.pragma('journal_mode = WAL');
    this.bus = bus;
    this.initializeSchema();
  }

  private initializeSchema(): void {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS feature_flags (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        description TEXT NOT NULL,
        enabled INTEGER NOT NULL DEFAULT 0,
        status TEXT NOT NULL DEFAULT 'disabled',
        created_at TEXT NOT NULL,
        enabled_at TEXT,
        bake_period_ms INTEGER NOT NULL,
        bake_started_at TEXT,
        rollback_reason TEXT,
        spec_id TEXT,
        metadata TEXT NOT NULL DEFAULT '{}'
      );

      CREATE TABLE IF NOT EXISTS flag_checks (
        id TEXT PRIMARY KEY,
        flag_id TEXT NOT NULL,
        timestamp TEXT NOT NULL,
        healthy INTEGER NOT NULL,
        message TEXT NOT NULL,
        FOREIGN KEY (flag_id) REFERENCES feature_flags(id)
      );

      CREATE INDEX IF NOT EXISTS idx_flags_status ON feature_flags(status);
      CREATE INDEX IF NOT EXISTS idx_flag_checks_flag_id ON flag_checks(flag_id);
    `);
  }

  createFlag(
    name: string,
    description: string,
    opts?: {
      bakePeriodMs?: number;
      specId?: string;
      metadata?: Record<string, unknown>;
    },
  ): FeatureFlag {
    const id = uuid();
    const now = new Date().toISOString();

    this.db.prepare(`
      INSERT INTO feature_flags (id, name, description, enabled, status, created_at, bake_period_ms, spec_id, metadata)
      VALUES (?, ?, ?, 0, 'disabled', ?, ?, ?, ?)
    `).run(
      id,
      name,
      description,
      now,
      opts?.bakePeriodMs ?? DEFAULT_BAKE_PERIOD_MS,
      opts?.specId ?? null,
      JSON.stringify(opts?.metadata ?? {}),
    );

    const flag = this.getFlag(id)!;

    this.bus.publish('flag.created', 'feature-flags', { flag });

    return flag;
  }

  enableFlag(id: string): FeatureFlag {
    const flag = this.getFlag(id);
    if (!flag) throw new Error(`Flag not found: ${id}`);

    const now = new Date().toISOString();

    this.db.prepare(`
      UPDATE feature_flags
      SET enabled = 1, status = 'baking', enabled_at = ?, bake_started_at = ?
      WHERE id = ?
    `).run(now, now, id);

    const updated = this.getFlag(id)!;
    this.bus.publish('flag.enabled', 'feature-flags', { flag: updated });

    return updated;
  }

  disableFlag(id: string, reason?: string): FeatureFlag {
    const flag = this.getFlag(id);
    if (!flag) throw new Error(`Flag not found: ${id}`);

    this.db.prepare(`
      UPDATE feature_flags
      SET enabled = 0, status = 'disabled', rollback_reason = ?
      WHERE id = ?
    `).run(reason ?? null, id);

    const updated = this.getFlag(id)!;
    this.bus.publish('flag.disabled', 'feature-flags', { flag: updated, reason });

    return updated;
  }

  isEnabled(id: string): boolean {
    const row = this.db.prepare('SELECT enabled FROM feature_flags WHERE id = ?').get(id) as
      | { enabled: number }
      | undefined;
    return row?.enabled === 1;
  }

  rollback(id: string, reason: string): FeatureFlag {
    const flag = this.getFlag(id);
    if (!flag) throw new Error(`Flag not found: ${id}`);

    this.db.prepare(`
      UPDATE feature_flags
      SET enabled = 0, status = 'rolled_back', rollback_reason = ?
      WHERE id = ?
    `).run(reason, id);

    const updated = this.getFlag(id)!;
    this.bus.publish('flag.rolled_back', 'feature-flags', { flag: updated, reason });

    return updated;
  }

  promote(id: string): FeatureFlag {
    const flag = this.getFlag(id);
    if (!flag) throw new Error(`Flag not found: ${id}`);

    this.db.prepare(`
      UPDATE feature_flags
      SET status = 'stable'
      WHERE id = ?
    `).run(id);

    const updated = this.getFlag(id)!;
    this.bus.publish('flag.promoted', 'feature-flags', { flag: updated });

    return updated;
  }

  recordCheck(flagId: string, healthy: boolean, message: string): FlagCheck {
    const flag = this.getFlag(flagId);
    if (!flag) throw new Error(`Flag not found: ${flagId}`);

    const id = uuid();
    const now = new Date().toISOString();

    this.db.prepare(`
      INSERT INTO flag_checks (id, flag_id, timestamp, healthy, message)
      VALUES (?, ?, ?, ?, ?)
    `).run(id, flagId, now, healthy ? 1 : 0, message);

    return { flagId, timestamp: now, healthy, message };
  }

  getFlag(id: string): FeatureFlag | null {
    const row = this.db
      .prepare('SELECT * FROM feature_flags WHERE id = ?')
      .get(id) as Record<string, unknown> | undefined;
    if (!row) return null;
    return this.rowToFlag(row);
  }

  listFlags(status?: string): FeatureFlag[] {
    let rows: Record<string, unknown>[];
    if (status) {
      rows = this.db
        .prepare('SELECT * FROM feature_flags WHERE status = ? ORDER BY created_at DESC')
        .all(status) as Record<string, unknown>[];
    } else {
      rows = this.db
        .prepare('SELECT * FROM feature_flags ORDER BY created_at DESC')
        .all() as Record<string, unknown>[];
    }
    return rows.map((r) => this.rowToFlag(r));
  }

  getChecks(flagId: string): FlagCheck[] {
    const rows = this.db
      .prepare('SELECT * FROM flag_checks WHERE flag_id = ? ORDER BY timestamp ASC')
      .all(flagId) as Record<string, unknown>[];
    return rows.map((r) => ({
      flagId: r.flag_id as string,
      timestamp: r.timestamp as string,
      healthy: (r.healthy as number) === 1,
      message: r.message as string,
    }));
  }

  checkBakePeriods(): { promoted: FeatureFlag[]; rolledBack: FeatureFlag[] } {
    const bakingFlags = this.listFlags('baking');
    const now = Date.now();
    const promoted: FeatureFlag[] = [];
    const rolledBack: FeatureFlag[] = [];

    for (const flag of bakingFlags) {
      const checks = this.getChecks(flag.id);
      const hasUnhealthy = checks.some((c) => !c.healthy);

      if (hasUnhealthy) {
        const unhealthyCheck = checks.find((c) => !c.healthy)!;
        const rolled = this.rollback(flag.id, `Unhealthy check: ${unhealthyCheck.message}`);
        rolledBack.push(rolled);
        continue;
      }

      const bakeStart = flag.bakeStartedAt ? new Date(flag.bakeStartedAt).getTime() : now;
      const elapsed = now - bakeStart;

      if (elapsed >= flag.bakePeriodMs) {
        const prom = this.promote(flag.id);
        promoted.push(prom);
      }
    }

    return { promoted, rolledBack };
  }

  diagnose(): DiagnosticResult {
    const all = this.listFlags();
    const baking = all.filter((f) => f.status === 'baking');
    const rolledBack = all.filter((f) => f.status === 'rolled_back');
    const stable = all.filter((f) => f.status === 'stable');
    const disabled = all.filter((f) => f.status === 'disabled');

    const checks = [
      {
        name: 'Flag count',
        passed: true,
        message: `${all.length} total flags`,
      },
      {
        name: 'Baking flags',
        passed: true,
        message: `${baking.length} flags currently baking`,
      },
      {
        name: 'Rolled back flags',
        passed: rolledBack.length === 0,
        message: rolledBack.length > 0
          ? `${rolledBack.length} flags rolled back: ${rolledBack.map((f) => f.name).join(', ')}`
          : 'No rolled back flags',
      },
      {
        name: 'Stable flags',
        passed: true,
        message: `${stable.length} flags stable`,
      },
    ];

    return {
      module: 'feature-flags',
      status: rolledBack.length > 0 ? 'degraded' : 'healthy',
      checks,
    };
  }

  close(): void {
    this.db.close();
  }

  private rowToFlag(row: Record<string, unknown>): FeatureFlag {
    return {
      id: row.id as string,
      name: row.name as string,
      description: row.description as string,
      enabled: (row.enabled as number) === 1,
      status: row.status as FeatureFlag['status'],
      createdAt: row.created_at as string,
      enabledAt: (row.enabled_at as string) ?? undefined,
      bakePeriodMs: row.bake_period_ms as number,
      bakeStartedAt: (row.bake_started_at as string) ?? undefined,
      rollbackReason: (row.rollback_reason as string) ?? undefined,
      specId: (row.spec_id as string) ?? undefined,
      metadata: JSON.parse((row.metadata as string) || '{}'),
    };
  }
}
