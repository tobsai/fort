/**
 * SubscriptionQuotaStore — SQLite snapshot of the latest known subscription
 * quota state for each LLM provider. Populated from rate-limit headers on
 * every OpenAI/Codex response so the dashboard and CLI can show "remaining
 * queries / resets in X" without making a separate query.
 *
 * The store keeps one row per provider id (e.g. 'openai'); writes are
 * upserts.
 */

import Database from 'better-sqlite3';

export interface QuotaSnapshot {
  providerId: string;
  /** Plan tier (e.g. "plus", "pro", "team") if the backend reports one. */
  planType: string | null;
  /** Units remaining in the primary rate-limit window. */
  remaining: number | null;
  /** Units consumed in the primary window. */
  used: number | null;
  /** Total units in the primary window. */
  limit: number | null;
  /** Human-readable label for the primary window (e.g. "5h"). */
  windowLabel: string | null;
  /** ISO datetime when the primary window resets. */
  resetAt: string | null;
  rawHeaders: Record<string, string>;
  updatedAt: string;
}

export class SubscriptionQuotaStore {
  private db: InstanceType<typeof Database>;

  constructor(dbPath: string) {
    this.db = new (Database as any)(dbPath) as InstanceType<typeof Database>;
    (this.db as any).pragma('journal_mode = WAL');
    this.initSchema();
  }

  initSchema(): void {
    const schema = `
      CREATE TABLE IF NOT EXISTS subscription_quota (
        provider_id   TEXT PRIMARY KEY,
        plan_type     TEXT,
        remaining     INTEGER,
        used          INTEGER,
        limit_total   INTEGER,
        window_label  TEXT,
        reset_at      TEXT,
        raw_headers   TEXT,
        updated_at    TEXT DEFAULT (datetime('now'))
      )
    `;
    (this.db as any)['exec'](schema);
    // Add plan_type column to pre-existing tables (idempotent).
    try {
      (this.db as any)['exec']("ALTER TABLE subscription_quota ADD COLUMN plan_type TEXT");
    } catch {
      // Column already exists — fine.
    }
  }

  set(snapshot: Omit<QuotaSnapshot, 'updatedAt'>): QuotaSnapshot {
    const now = new Date().toISOString();
    (this.db.prepare(`
      INSERT INTO subscription_quota
        (provider_id, plan_type, remaining, used, limit_total, window_label, reset_at, raw_headers, updated_at)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
      ON CONFLICT(provider_id) DO UPDATE SET
        plan_type    = excluded.plan_type,
        remaining    = excluded.remaining,
        used         = excluded.used,
        limit_total  = excluded.limit_total,
        window_label = excluded.window_label,
        reset_at     = excluded.reset_at,
        raw_headers  = excluded.raw_headers,
        updated_at   = excluded.updated_at
    `) as any).run(
      snapshot.providerId,
      snapshot.planType,
      snapshot.remaining,
      snapshot.used,
      snapshot.limit,
      snapshot.windowLabel,
      snapshot.resetAt,
      JSON.stringify(snapshot.rawHeaders),
      now,
    );
    return { ...snapshot, updatedAt: now };
  }

  get(providerId: string): QuotaSnapshot | null {
    const row = (this.db.prepare(
      'SELECT * FROM subscription_quota WHERE provider_id = ?',
    ) as any).get(providerId) as Record<string, unknown> | undefined;
    if (!row) return null;
    return this.rowToSnapshot(row);
  }

  list(): QuotaSnapshot[] {
    const rows = (this.db.prepare(
      'SELECT * FROM subscription_quota ORDER BY updated_at DESC',
    ) as any).all() as unknown[];
    return rows.map((r) => this.rowToSnapshot(r as Record<string, unknown>));
  }

  close(): void {
    this.db.close();
  }

  private rowToSnapshot(row: Record<string, unknown>): QuotaSnapshot {
    let rawHeaders: Record<string, string> = {};
    try {
      rawHeaders = JSON.parse((row['raw_headers'] as string) ?? '{}');
    } catch {
      rawHeaders = {};
    }
    return {
      providerId:  row['provider_id']  as string,
      planType:    (row['plan_type']   as string | null) ?? null,
      remaining:   (row['remaining']   as number | null) ?? null,
      used:        (row['used']        as number | null) ?? null,
      limit:       (row['limit_total'] as number | null) ?? null,
      windowLabel: (row['window_label'] as string | null) ?? null,
      resetAt:     (row['reset_at']    as string | null) ?? null,
      rawHeaders,
      updatedAt:   row['updated_at']   as string,
    };
  }
}
