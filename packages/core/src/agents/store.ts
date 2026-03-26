/**
 * AgentStore — SQLite-backed persistence for specialist agent metadata.
 *
 * Source of truth for agent configuration. Supports soft-delete (preserve history)
 * and on-first-boot migration from existing YAML identity files.
 */

import Database from 'better-sqlite3';
import { readFileSync, readdirSync, existsSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { parse as parseYaml } from 'yaml';
import type { SpecialistIdentity, DiagnosticResult } from '../types.js';

export interface AgentRecord {
  id: string;
  name: string;
  description: string;
  type: string;
  modelPreference: string | null;
  soul: string | null;
  capabilities: string[];
  tools: string[];
  eventSubscriptions: string[];
  memoryPartition: string;
  status: 'active' | 'retired';
  emoji: string | null;
  isDefault: boolean;
  deletedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface CreateAgentParams {
  id: string;
  name: string;
  description?: string;
  type?: string;
  modelPreference?: string;
  soul?: string;
  capabilities?: string[];
  tools?: string[];
  eventSubscriptions?: string[];
  memoryPartition?: string;
  emoji?: string;
  isDefault?: boolean;
}

export interface UpdateAgentParams {
  name?: string;
  description?: string;
  modelPreference?: string | null;
  soul?: string | null;
  capabilities?: string[];
  tools?: string[];
  eventSubscriptions?: string[];
  emoji?: string | null;
  isDefault?: boolean;
  status?: 'active' | 'retired';
}

export class AgentStore {
  private db: InstanceType<typeof Database>;

  constructor(dbPath: string, agentsDir?: string) {
    this.db = new (Database as any)(dbPath) as InstanceType<typeof Database>;
    (this.db as any).pragma('journal_mode = WAL');
    this.initialize();
    if (agentsDir && this.count() === 0) {
      this.migrateFromYaml(agentsDir);
    }
  }

  private initialize(): void {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS agents (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        description TEXT NOT NULL DEFAULT '',
        type TEXT NOT NULL DEFAULT 'specialist',
        model_preference TEXT,
        soul TEXT,
        capabilities TEXT NOT NULL DEFAULT '[]',
        tools TEXT NOT NULL DEFAULT '[]',
        event_subscriptions TEXT NOT NULL DEFAULT '[]',
        memory_partition TEXT NOT NULL DEFAULT '',
        status TEXT NOT NULL DEFAULT 'active',
        emoji TEXT,
        is_default INTEGER NOT NULL DEFAULT 0,
        deleted_at TEXT,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL
      )
    `);
  }

  private migrateFromYaml(agentsDir: string): void {
    if (!existsSync(agentsDir)) return;

    try {
      const entries = readdirSync(agentsDir);
      for (const entry of entries) {
        const entryPath = join(agentsDir, entry);
        let identityPath: string;
        let agentDir: string | null = null;

        if (statSync(entryPath).isDirectory()) {
          identityPath = join(entryPath, 'identity.yaml');
          agentDir = entryPath;
        } else if (entry.endsWith('.yaml')) {
          identityPath = entryPath;
        } else {
          continue;
        }

        if (!existsSync(identityPath)) continue;

        try {
          const identity = parseYaml(readFileSync(identityPath, 'utf-8')) as SpecialistIdentity;
          if (!identity?.id || !identity?.name) continue;

          // Read SOUL.md if in directory-based layout
          let soul: string | null = null;
          if (agentDir) {
            const soulPath = join(agentDir, 'SOUL.md');
            if (existsSync(soulPath)) {
              soul = readFileSync(soulPath, 'utf-8');
            }
          }

          // Don't re-insert if already exists
          if (this.get(identity.id)) continue;

          this.create({
            id: identity.id,
            name: identity.name,
            description: identity.description ?? '',
            type: 'specialist',
            capabilities: identity.capabilities ?? [],
            eventSubscriptions: identity.eventSubscriptions ?? [],
            memoryPartition: identity.memoryPartition ?? identity.id,
            emoji: identity.emoji,
            isDefault: identity.isDefault ?? false,
            soul,
            status: identity.status === 'retired' ? 'retired' : 'active',
          } as CreateAgentParams & { status?: 'active' | 'retired' });
        } catch {
          // Skip bad identity files
        }
      }
    } catch {
      // Migration failed — non-fatal
    }
  }

  create(params: CreateAgentParams & { status?: 'active' | 'retired' }): AgentRecord {
    const now = new Date().toISOString();
    (this.db.prepare(`
      INSERT INTO agents (id, name, description, type, model_preference, soul, capabilities, tools,
        event_subscriptions, memory_partition, status, emoji, is_default, created_at, updated_at)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `) as any).run(
      params.id,
      params.name,
      params.description ?? '',
      params.type ?? 'specialist',
      params.modelPreference ?? null,
      params.soul ?? null,
      JSON.stringify(params.capabilities ?? []),
      JSON.stringify(params.tools ?? []),
      JSON.stringify(params.eventSubscriptions ?? []),
      params.memoryPartition ?? params.id,
      params.status ?? 'active',
      params.emoji ?? null,
      params.isDefault ? 1 : 0,
      now,
      now,
    );
    return this.get(params.id)!;
  }

  update(id: string, params: UpdateAgentParams): AgentRecord {
    if (!this.get(id)) throw new Error(`Agent not found: ${id}`);

    const now = new Date().toISOString();
    const fields: string[] = ['updated_at = ?'];
    const values: unknown[] = [now];

    if (params.name !== undefined) { fields.push('name = ?'); values.push(params.name); }
    if (params.description !== undefined) { fields.push('description = ?'); values.push(params.description); }
    if (params.modelPreference !== undefined) { fields.push('model_preference = ?'); values.push(params.modelPreference); }
    if (params.soul !== undefined) { fields.push('soul = ?'); values.push(params.soul); }
    if (params.capabilities !== undefined) { fields.push('capabilities = ?'); values.push(JSON.stringify(params.capabilities)); }
    if (params.tools !== undefined) { fields.push('tools = ?'); values.push(JSON.stringify(params.tools)); }
    if (params.eventSubscriptions !== undefined) { fields.push('event_subscriptions = ?'); values.push(JSON.stringify(params.eventSubscriptions)); }
    if (params.emoji !== undefined) { fields.push('emoji = ?'); values.push(params.emoji); }
    if (params.isDefault !== undefined) { fields.push('is_default = ?'); values.push(params.isDefault ? 1 : 0); }
    if (params.status !== undefined) { fields.push('status = ?'); values.push(params.status); }

    values.push(id);
    (this.db.prepare(`UPDATE agents SET ${fields.join(', ')} WHERE id = ?`) as any).run(...values);

    return this.get(id)!;
  }

  softDelete(id: string): void {
    const now = new Date().toISOString();
    (this.db.prepare(
      `UPDATE agents SET deleted_at = ?, status = 'retired', updated_at = ? WHERE id = ?`,
    ) as any).run(now, now, id);
  }

  get(id: string): AgentRecord | null {
    const row = (this.db.prepare('SELECT * FROM agents WHERE id = ?') as any).get(id);
    return row ? this.rowToRecord(row) : null;
  }

  list(options: { includeDeleted?: boolean } = {}): AgentRecord[] {
    const sql = options.includeDeleted
      ? 'SELECT * FROM agents ORDER BY created_at ASC'
      : 'SELECT * FROM agents WHERE deleted_at IS NULL ORDER BY created_at ASC';
    const rows = (this.db.prepare(sql) as any).all() as unknown[];
    return rows.map((r) => this.rowToRecord(r as Record<string, unknown>));
  }

  private count(): number {
    const result = (this.db.prepare('SELECT COUNT(*) as count FROM agents') as any).get() as { count: number };
    return result.count;
  }

  private rowToRecord(row: Record<string, unknown>): AgentRecord {
    return {
      id: row['id'] as string,
      name: row['name'] as string,
      description: row['description'] as string,
      type: row['type'] as string,
      modelPreference: (row['model_preference'] as string | null) ?? null,
      soul: (row['soul'] as string | null) ?? null,
      capabilities: JSON.parse((row['capabilities'] as string) || '[]'),
      tools: JSON.parse((row['tools'] as string) || '[]'),
      eventSubscriptions: JSON.parse((row['event_subscriptions'] as string) || '[]'),
      memoryPartition: row['memory_partition'] as string,
      status: row['status'] as 'active' | 'retired',
      emoji: (row['emoji'] as string | null) ?? null,
      isDefault: row['is_default'] === 1,
      deletedAt: (row['deleted_at'] as string | null) ?? null,
      createdAt: row['created_at'] as string,
      updatedAt: row['updated_at'] as string,
    };
  }

  close(): void {
    this.db.close();
  }

  diagnose(): DiagnosticResult {
    const total = (this.db.prepare(
      'SELECT COUNT(*) as count FROM agents WHERE deleted_at IS NULL',
    ) as any).get() as { count: number };
    const active = (this.db.prepare(
      "SELECT COUNT(*) as count FROM agents WHERE status = 'active' AND deleted_at IS NULL",
    ) as any).get() as { count: number };

    return {
      module: 'agent-store',
      status: 'healthy',
      checks: [
        {
          name: 'Agent database',
          passed: true,
          message: `${total.count} agents total (${active.count} active)`,
        },
      ],
    };
  }
}
