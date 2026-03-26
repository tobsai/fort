/**
 * Tool Registry — Reuse Before You Build
 *
 * Before Fort creates any new tool, it must search this registry.
 * The registry is the manifest of all deterministic tools, workflows,
 * and agent capabilities available in the system.
 */

import Database from 'better-sqlite3';
import { v4 as uuid } from 'uuid';
import { existsSync, mkdirSync } from 'node:fs';
import { dirname } from 'node:path';
import type { ToolDefinition, DiagnosticResult } from '../types.js';
import type { FortTool } from './types.js';

export type { FortTool, ToolResult, ToolCallLog } from './types.js';
export { ToolExecutor, ToolRejectedError } from './executor.js';
export { ApprovalStore } from './approval-store.js';
export type { ApprovalRequest, ApprovalStatus, CreateApprovalInput } from './approval-store.js';

export class ToolRegistry {
  private db: Database.Database;
  private liveTools: Map<string, FortTool> = new Map();

  constructor(dbPath: string) {
    const dir = dirname(dbPath);
    if (!existsSync(dir)) mkdirSync(dir, { recursive: true });

    this.db = new Database(dbPath);
    this.db.pragma('journal_mode = WAL');
    this.initializeSchema();
  }

  private initializeSchema(): void {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS tools (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        description TEXT NOT NULL,
        capabilities TEXT NOT NULL DEFAULT '[]',
        input_types TEXT NOT NULL DEFAULT '[]',
        output_types TEXT NOT NULL DEFAULT '[]',
        tags TEXT NOT NULL DEFAULT '[]',
        module TEXT NOT NULL,
        version TEXT NOT NULL,
        usage_count INTEGER NOT NULL DEFAULT 0,
        last_used_at TEXT,
        created_at TEXT NOT NULL
      );

      CREATE INDEX IF NOT EXISTS idx_tools_name ON tools(name);
      CREATE INDEX IF NOT EXISTS idx_tools_module ON tools(module);
    `);
  }

  /**
   * Register a live FortTool (executable) alongside metadata in the registry.
   * The tool's metadata is persisted to SQLite; the callable implementation is
   * stored in memory for the lifetime of this ToolRegistry instance.
   */
  registerTool(tool: FortTool): ToolDefinition {
    // Persist metadata to database
    const definition = this.register({
      name: tool.name,
      description: tool.description,
      capabilities: [],
      inputTypes: [],
      outputTypes: [],
      tags: [],
      module: 'runtime',
      version: '1.0.0',
    });

    // Store the live callable
    this.liveTools.set(tool.name, tool);

    return definition;
  }

  /**
   * Retrieve a registered live FortTool by name.
   * Returns null if the tool was registered as metadata-only (via register())
   * or has not been registered at all.
   */
  getLiveTool(name: string): FortTool | null {
    return this.liveTools.get(name) ?? null;
  }

  /**
   * List all live FortTool instances registered via registerTool().
   */
  listLiveTools(): FortTool[] {
    return Array.from(this.liveTools.values());
  }

  register(tool: Omit<ToolDefinition, 'id' | 'usageCount' | 'lastUsedAt'>): ToolDefinition {
    const id = uuid();
    const now = new Date().toISOString();

    this.db.prepare(`
      INSERT INTO tools (id, name, description, capabilities, input_types, output_types, tags, module, version, created_at)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run(
      id, tool.name, tool.description,
      JSON.stringify(tool.capabilities),
      JSON.stringify(tool.inputTypes),
      JSON.stringify(tool.outputTypes),
      JSON.stringify(tool.tags),
      tool.module, tool.version, now
    );

    return { ...tool, id, usageCount: 0, lastUsedAt: null };
  }

  get(toolId: string): ToolDefinition | null {
    const row = this.db.prepare('SELECT * FROM tools WHERE id = ?').get(toolId) as any;
    return row ? this.rowToTool(row) : null;
  }

  /**
   * Search the registry by capability, name, or description.
   * This is the core of the "reuse before build" principle.
   */
  search(query: string): ToolDefinition[] {
    // Split query into terms and require each term matches somewhere
    const terms = query.toLowerCase().split(/\s+/).filter(Boolean);
    if (terms.length === 0) return [];

    const conditions = terms.map(() =>
      `(LOWER(name) LIKE ? OR LOWER(description) LIKE ? OR LOWER(capabilities) LIKE ? OR LOWER(tags) LIKE ?)`
    ).join(' AND ');

    const params = terms.flatMap((t) => {
      const p = `%${t}%`;
      return [p, p, p, p];
    });

    const rows = this.db.prepare(`
      SELECT * FROM tools
      WHERE ${conditions}
      ORDER BY usage_count DESC
    `).all(...params) as any[];

    return rows.map(this.rowToTool);
  }

  /**
   * Check if existing tools can handle a capability before building new ones.
   * Returns matching tools ranked by relevance.
   */
  findByCapability(capability: string): ToolDefinition[] {
    const pattern = `%${capability.toLowerCase()}%`;
    const rows = this.db.prepare(`
      SELECT * FROM tools
      WHERE LOWER(capabilities) LIKE ?
         OR LOWER(description) LIKE ?
      ORDER BY usage_count DESC
    `).all(pattern, pattern) as any[];

    return rows.map(this.rowToTool);
  }

  recordUsage(toolId: string): void {
    this.db.prepare(`
      UPDATE tools SET usage_count = usage_count + 1, last_used_at = ? WHERE id = ?
    `).run(new Date().toISOString(), toolId);
  }

  list(): ToolDefinition[] {
    const rows = this.db.prepare('SELECT * FROM tools ORDER BY name').all() as any[];
    return rows.map(this.rowToTool);
  }

  unregister(toolId: string): boolean {
    const result = this.db.prepare('DELETE FROM tools WHERE id = ?').run(toolId);
    return result.changes > 0;
  }

  stats(): { totalTools: number; totalUsage: number; byModule: Record<string, number> } {
    const totalTools = (this.db.prepare('SELECT COUNT(*) as count FROM tools').get() as any).count;
    const totalUsage = (this.db.prepare('SELECT SUM(usage_count) as total FROM tools').get() as any).total ?? 0;
    const byModule: Record<string, number> = {};
    const rows = this.db.prepare('SELECT module, COUNT(*) as count FROM tools GROUP BY module').all() as any[];
    for (const row of rows) byModule[row.module] = row.count;
    return { totalTools, totalUsage, byModule };
  }

  diagnose(): DiagnosticResult {
    const checks = [];
    try {
      const s = this.stats();
      checks.push({ name: 'Tool registry', passed: true, message: `${s.totalTools} tools registered` });
    } catch (err) {
      checks.push({ name: 'Tool registry', passed: false, message: String(err) });
    }
    return {
      module: 'tools',
      status: checks.every((c) => c.passed) ? 'healthy' : 'unhealthy',
      checks,
    };
  }

  close(): void {
    this.db.close();
  }

  private rowToTool(row: any): ToolDefinition {
    return {
      id: row.id,
      name: row.name,
      description: row.description,
      capabilities: JSON.parse(row.capabilities),
      inputTypes: JSON.parse(row.input_types),
      outputTypes: JSON.parse(row.output_types),
      tags: JSON.parse(row.tags),
      module: row.module,
      version: row.version,
      usageCount: row.usage_count,
      lastUsedAt: row.last_used_at ? new Date(row.last_used_at) : null,
    };
  }
}
