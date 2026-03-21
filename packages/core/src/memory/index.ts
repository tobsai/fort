/**
 * Memory Module — Graph-based memory with MemU backend + local fallback
 *
 * Provides a unified interface over MemU (when available) or a local
 * SQLite-backed graph store as fallback. Adds the Memory Inspector
 * layer on top.
 */

import Database from 'better-sqlite3';
import { v4 as uuid } from 'uuid';
import { existsSync, mkdirSync } from 'node:fs';
import { dirname } from 'node:path';
import type { MemoryNode, MemoryEdge, MemoryQuery, MemorySearchResult, DiagnosticResult } from '../types.js';
import { MemUClient } from './memu-client.js';
import type { ModuleBus } from '../module-bus/index.js';

export class MemoryManager {
  private db: Database.Database;
  private memuClient: MemUClient | null;
  private bus: ModuleBus;
  private useMemU: boolean = false;

  constructor(dbPath: string, bus: ModuleBus, memuUrl?: string) {
    this.bus = bus;

    // Ensure directory exists
    const dir = dirname(dbPath);
    if (!existsSync(dir)) mkdirSync(dir, { recursive: true });

    this.db = new Database(dbPath);
    this.db.pragma('journal_mode = WAL');
    this.initializeSchema();

    this.memuClient = memuUrl ? new MemUClient({ baseUrl: memuUrl }) : null;
  }

  private initializeSchema(): void {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS nodes (
        id TEXT PRIMARY KEY,
        type TEXT NOT NULL,
        label TEXT NOT NULL,
        properties TEXT NOT NULL DEFAULT '{}',
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        source TEXT NOT NULL DEFAULT ''
      );

      CREATE TABLE IF NOT EXISTS edges (
        id TEXT PRIMARY KEY,
        source_id TEXT NOT NULL,
        target_id TEXT NOT NULL,
        type TEXT NOT NULL,
        properties TEXT NOT NULL DEFAULT '{}',
        created_at TEXT NOT NULL,
        FOREIGN KEY (source_id) REFERENCES nodes(id) ON DELETE CASCADE,
        FOREIGN KEY (target_id) REFERENCES nodes(id) ON DELETE CASCADE
      );

      CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type);
      CREATE INDEX IF NOT EXISTS idx_nodes_label ON nodes(label);
      CREATE INDEX IF NOT EXISTS idx_edges_source ON edges(source_id);
      CREATE INDEX IF NOT EXISTS idx_edges_target ON edges(target_id);
      CREATE INDEX IF NOT EXISTS idx_edges_type ON edges(type);
    `);
  }

  async initialize(): Promise<void> {
    if (this.memuClient) {
      this.useMemU = await this.memuClient.ping();
    }
  }

  // ─── Node Operations ───────────────────────────────────────────

  createNode(params: {
    type: MemoryNode['type'];
    label: string;
    properties?: Record<string, unknown>;
    source?: string;
  }): MemoryNode {
    const node: MemoryNode = {
      id: uuid(),
      type: params.type,
      label: params.label,
      properties: params.properties ?? {},
      createdAt: new Date(),
      updatedAt: new Date(),
      source: params.source ?? '',
    };

    this.db.prepare(`
      INSERT INTO nodes (id, type, label, properties, created_at, updated_at, source)
      VALUES (?, ?, ?, ?, ?, ?, ?)
    `).run(
      node.id, node.type, node.label,
      JSON.stringify(node.properties),
      node.createdAt.toISOString(),
      node.updatedAt.toISOString(),
      node.source
    );

    this.bus.publish('memory.node_created', 'memory', { node });
    return node;
  }

  getNode(nodeId: string): MemoryNode | null {
    const row = this.db.prepare('SELECT * FROM nodes WHERE id = ?').get(nodeId) as any;
    return row ? this.rowToNode(row) : null;
  }

  updateNode(nodeId: string, updates: { label?: string; properties?: Record<string, unknown> }): MemoryNode | null {
    const node = this.getNode(nodeId);
    if (!node) return null;

    if (updates.label !== undefined) node.label = updates.label;
    if (updates.properties !== undefined) node.properties = { ...node.properties, ...updates.properties };
    node.updatedAt = new Date();

    this.db.prepare(`
      UPDATE nodes SET label = ?, properties = ?, updated_at = ? WHERE id = ?
    `).run(node.label, JSON.stringify(node.properties), node.updatedAt.toISOString(), nodeId);

    this.bus.publish('memory.node_updated', 'memory', { node });
    return node;
  }

  deleteNode(nodeId: string): boolean {
    const result = this.db.prepare('DELETE FROM nodes WHERE id = ?').run(nodeId);
    if (result.changes > 0) {
      // Cascade delete edges
      this.db.prepare('DELETE FROM edges WHERE source_id = ? OR target_id = ?').run(nodeId, nodeId);
      this.bus.publish('memory.node_deleted', 'memory', { nodeId });
      return true;
    }
    return false;
  }

  // ─── Edge Operations ───────────────────────────────────────────

  createEdge(params: {
    sourceId: string;
    targetId: string;
    type: string;
    properties?: Record<string, unknown>;
  }): MemoryEdge {
    const edge: MemoryEdge = {
      id: uuid(),
      sourceId: params.sourceId,
      targetId: params.targetId,
      type: params.type,
      properties: params.properties ?? {},
      createdAt: new Date(),
    };

    this.db.prepare(`
      INSERT INTO edges (id, source_id, target_id, type, properties, created_at)
      VALUES (?, ?, ?, ?, ?, ?)
    `).run(
      edge.id, edge.sourceId, edge.targetId, edge.type,
      JSON.stringify(edge.properties),
      edge.createdAt.toISOString()
    );

    this.bus.publish('memory.edge_created', 'memory', { edge });
    return edge;
  }

  getEdgesFrom(nodeId: string): MemoryEdge[] {
    const rows = this.db.prepare('SELECT * FROM edges WHERE source_id = ?').all(nodeId) as any[];
    return rows.map(this.rowToEdge);
  }

  getEdgesTo(nodeId: string): MemoryEdge[] {
    const rows = this.db.prepare('SELECT * FROM edges WHERE target_id = ?').all(nodeId) as any[];
    return rows.map(this.rowToEdge);
  }

  // ─── Search ────────────────────────────────────────────────────

  search(query: MemoryQuery): MemorySearchResult {
    let sql = 'SELECT * FROM nodes WHERE 1=1';
    const params: unknown[] = [];

    if (query.nodeType) {
      sql += ' AND type = ?';
      params.push(query.nodeType);
    }

    if (query.text) {
      sql += ' AND (label LIKE ? OR properties LIKE ?)';
      const pattern = `%${query.text}%`;
      params.push(pattern, pattern);
    }

    sql += ' ORDER BY updated_at DESC';

    if (query.limit) {
      sql += ' LIMIT ?';
      params.push(query.limit);
    }

    const nodes = (this.db.prepare(sql).all(...params) as any[]).map(this.rowToNode);

    // Gather edges between found nodes
    const nodeIds = new Set(nodes.map((n) => n.id));
    const edges: MemoryEdge[] = [];
    for (const node of nodes) {
      const outgoing = this.getEdgesFrom(node.id);
      for (const edge of outgoing) {
        if (nodeIds.has(edge.targetId)) {
          edges.push(edge);
        }
      }
    }

    return { nodes, edges };
  }

  traverse(startNodeId: string, depth: number = 2): MemorySearchResult {
    const visited = new Set<string>();
    const nodes: MemoryNode[] = [];
    const edges: MemoryEdge[] = [];

    const visit = (nodeId: string, currentDepth: number) => {
      if (currentDepth > depth || visited.has(nodeId)) return;
      visited.add(nodeId);

      const node = this.getNode(nodeId);
      if (!node) return;
      nodes.push(node);

      const outgoing = this.getEdgesFrom(nodeId);
      const incoming = this.getEdgesTo(nodeId);

      for (const edge of [...outgoing, ...incoming]) {
        edges.push(edge);
        const nextId = edge.sourceId === nodeId ? edge.targetId : edge.sourceId;
        visit(nextId, currentDepth + 1);
      }
    };

    visit(startNodeId, 0);
    return { nodes, edges };
  }

  // ─── Stats & Export ────────────────────────────────────────────

  stats(): { nodeCount: number; edgeCount: number; nodeTypes: Record<string, number>; edgeTypes: Record<string, number> } {
    const nodeCount = (this.db.prepare('SELECT COUNT(*) as count FROM nodes').get() as any).count;
    const edgeCount = (this.db.prepare('SELECT COUNT(*) as count FROM edges').get() as any).count;

    const nodeTypes: Record<string, number> = {};
    const ntRows = this.db.prepare('SELECT type, COUNT(*) as count FROM nodes GROUP BY type').all() as any[];
    for (const row of ntRows) nodeTypes[row.type] = row.count;

    const edgeTypes: Record<string, number> = {};
    const etRows = this.db.prepare('SELECT type, COUNT(*) as count FROM edges GROUP BY type').all() as any[];
    for (const row of etRows) edgeTypes[row.type] = row.count;

    return { nodeCount, edgeCount, nodeTypes, edgeTypes };
  }

  exportGraph(): { nodes: MemoryNode[]; edges: MemoryEdge[] } {
    const nodes = (this.db.prepare('SELECT * FROM nodes').all() as any[]).map(this.rowToNode);
    const edges = (this.db.prepare('SELECT * FROM edges').all() as any[]).map(this.rowToEdge);
    return { nodes, edges };
  }

  async diagnose(): Promise<DiagnosticResult> {
    const checks = [];

    // SQLite check
    try {
      this.db.prepare('SELECT 1').get();
      checks.push({ name: 'SQLite connection', passed: true, message: 'Connected' });
    } catch (err) {
      checks.push({ name: 'SQLite connection', passed: false, message: String(err) });
    }

    // MemU check
    if (this.memuClient) {
      const reachable = await this.memuClient.ping();
      checks.push({
        name: 'MemU sidecar',
        passed: reachable,
        message: reachable ? 'Connected' : 'MemU sidecar unreachable',
      });
    } else {
      checks.push({
        name: 'MemU sidecar',
        passed: true,
        message: 'Not configured (using local SQLite)',
      });
    }

    // Stats check
    const s = this.stats();
    checks.push({
      name: 'Memory stats',
      passed: true,
      message: `${s.nodeCount} nodes, ${s.edgeCount} edges`,
      details: s,
    });

    const allPassed = checks.every((c) => c.passed);
    return {
      module: 'memory',
      status: allPassed ? 'healthy' : 'degraded',
      checks,
    };
  }

  close(): void {
    this.db.close();
  }

  // ─── Row Mappers ───────────────────────────────────────────────

  private rowToNode(row: any): MemoryNode {
    return {
      id: row.id,
      type: row.type,
      label: row.label,
      properties: JSON.parse(row.properties),
      createdAt: new Date(row.created_at),
      updatedAt: new Date(row.updated_at),
      source: row.source,
    };
  }

  private rowToEdge(row: any): MemoryEdge {
    return {
      id: row.id,
      sourceId: row.source_id,
      targetId: row.target_id,
      type: row.type,
      properties: JSON.parse(row.properties),
      createdAt: new Date(row.created_at),
    };
  }
}

export { MemUClient } from './memu-client.js';
