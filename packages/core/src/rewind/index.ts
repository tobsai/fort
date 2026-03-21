/**
 * Rewind Manager — Backup & Restore for Fort State
 *
 * Creates timestamped snapshots of Fort state (memory DB, config files,
 * agent identities) and supports full or partial restore. Uses SQLite
 * to track snapshot metadata and change detection.
 */

import Database from 'better-sqlite3';
import { v4 as uuid } from 'uuid';
import { existsSync, mkdirSync, copyFileSync, readdirSync, statSync, readFileSync, writeFileSync, rmSync } from 'node:fs';
import { dirname, join, basename, relative } from 'node:path';
import { createHash } from 'node:crypto';
import type { ModuleBus } from '../module-bus/index.js';
import type { Snapshot, SnapshotStats, RewindPreview, RewindPreviewChange, DiagnosticResult } from '../types.js';

export class RewindManager {
  private db: Database.Database;
  private bus: ModuleBus | null;
  private dataDir: string;
  private snapshotsDir: string;

  constructor(dbPath: string, dataDir: string, bus?: ModuleBus) {
    const dir = dirname(dbPath);
    if (!existsSync(dir)) mkdirSync(dir, { recursive: true });

    this.db = new Database(dbPath);
    this.db.pragma('journal_mode = WAL');
    this.initializeSchema();

    this.dataDir = dataDir;
    this.snapshotsDir = join(dataDir, 'snapshots');
    if (!existsSync(this.snapshotsDir)) mkdirSync(this.snapshotsDir, { recursive: true });

    this.bus = bus ?? null;
  }

  private initializeSchema(): void {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS snapshots (
        id TEXT PRIMARY KEY,
        label TEXT,
        trigger TEXT NOT NULL DEFAULT 'manual',
        created_at TEXT NOT NULL,
        file_count INTEGER NOT NULL DEFAULT 0,
        total_bytes INTEGER NOT NULL DEFAULT 0,
        files_hash TEXT NOT NULL DEFAULT '',
        metadata TEXT NOT NULL DEFAULT '{}'
      );

      CREATE INDEX IF NOT EXISTS idx_snapshots_created_at ON snapshots(created_at);
    `);
  }

  // ─── Snapshot Creation ────────────────────────────────────────────

  createSnapshot(label?: string, trigger: string = 'manual'): Snapshot {
    const id = uuid();
    const createdAt = new Date().toISOString();
    const snapshotDir = join(this.snapshotsDir, id);
    mkdirSync(snapshotDir, { recursive: true });

    // Gather files to snapshot
    const files = this.gatherSnapshotFiles();
    let totalBytes = 0;

    for (const file of files) {
      const srcPath = join(this.dataDir, file);
      if (!existsSync(srcPath)) continue;

      const destDir = join(snapshotDir, dirname(file));
      if (!existsSync(destDir)) mkdirSync(destDir, { recursive: true });

      copyFileSync(srcPath, join(snapshotDir, file));
      const stat = statSync(srcPath);
      totalBytes += stat.size;
    }

    const filesHash = this.computeFilesHash(files);

    const stmt = this.db.prepare(`
      INSERT INTO snapshots (id, label, trigger, created_at, file_count, total_bytes, files_hash, metadata)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `);
    stmt.run(
      id,
      label ?? null,
      trigger,
      createdAt,
      files.length,
      totalBytes,
      filesHash,
      JSON.stringify({ files }),
    );

    const snapshot = this.getSnapshot(id)!;

    this.bus?.publish('rewind.snapshot_created', 'rewind', {
      snapshotId: id,
      label: label ?? null,
      trigger,
      fileCount: files.length,
      totalBytes,
    });

    return snapshot;
  }

  // ─── Listing ──────────────────────────────────────────────────────

  listSnapshots(opts?: { limit?: number; since?: Date }): Snapshot[] {
    let sql = 'SELECT * FROM snapshots';
    const params: unknown[] = [];

    if (opts?.since) {
      sql += ' WHERE created_at >= ?';
      params.push(opts.since.toISOString());
    }

    sql += ' ORDER BY created_at DESC';

    if (opts?.limit) {
      sql += ' LIMIT ?';
      params.push(opts.limit);
    }

    const rows = this.db.prepare(sql).all(...params) as SnapshotRow[];
    return rows.map(rowToSnapshot);
  }

  getSnapshot(id: string): Snapshot | null {
    const row = this.db.prepare('SELECT * FROM snapshots WHERE id = ?').get(id) as SnapshotRow | undefined;
    return row ? rowToSnapshot(row) : null;
  }

  // ─── Preview ──────────────────────────────────────────────────────

  previewRestore(snapshotId: string): RewindPreview {
    const snapshot = this.getSnapshot(snapshotId);
    if (!snapshot) throw new Error(`Snapshot not found: ${snapshotId}`);

    const snapshotDir = join(this.snapshotsDir, snapshotId);
    if (!existsSync(snapshotDir)) throw new Error(`Snapshot directory missing: ${snapshotId}`);

    const changes: RewindPreviewChange[] = [];
    const snapshotFiles = this.getSnapshotFileList(snapshotId);
    const currentFiles = this.gatherSnapshotFiles();

    // Files in snapshot
    for (const file of snapshotFiles) {
      const snapshotPath = join(snapshotDir, file);
      const currentPath = join(this.dataDir, file);

      if (!existsSync(currentPath)) {
        changes.push({ file, type: 'added', details: 'File will be restored (not in current state)' });
      } else {
        const snapshotHash = hashFile(snapshotPath);
        const currentHash = hashFile(currentPath);
        if (snapshotHash !== currentHash) {
          changes.push({ file, type: 'modified', details: 'File differs from snapshot' });
        }
      }
    }

    // Files in current state but not in snapshot
    for (const file of currentFiles) {
      if (!snapshotFiles.includes(file)) {
        changes.push({ file, type: 'removed', details: 'File exists now but not in snapshot' });
      }
    }

    const preview: RewindPreview = {
      snapshotId,
      snapshot,
      changes,
      hasChanges: changes.length > 0,
    };

    this.bus?.publish('rewind.preview', 'rewind', { snapshotId, changeCount: changes.length });

    return preview;
  }

  // ─── Restore ──────────────────────────────────────────────────────

  restore(snapshotId: string, opts?: { configOnly?: boolean; memoryOnly?: boolean }): RewindPreview {
    const snapshot = this.getSnapshot(snapshotId);
    if (!snapshot) throw new Error(`Snapshot not found: ${snapshotId}`);

    const snapshotDir = join(this.snapshotsDir, snapshotId);
    if (!existsSync(snapshotDir)) throw new Error(`Snapshot directory missing: ${snapshotId}`);

    const preview = this.previewRestore(snapshotId);
    const snapshotFiles = this.getSnapshotFileList(snapshotId);

    for (const file of snapshotFiles) {
      // Apply filters
      if (opts?.configOnly && !this.isConfigFile(file)) continue;
      if (opts?.memoryOnly && !this.isMemoryFile(file)) continue;

      const snapshotPath = join(snapshotDir, file);
      const destPath = join(this.dataDir, file);

      if (!existsSync(snapshotPath)) continue;

      const destDir = dirname(destPath);
      if (!existsSync(destDir)) mkdirSync(destDir, { recursive: true });

      copyFileSync(snapshotPath, destPath);
    }

    this.bus?.publish('rewind.restored', 'rewind', {
      snapshotId,
      label: snapshot.label,
      configOnly: opts?.configOnly ?? false,
      memoryOnly: opts?.memoryOnly ?? false,
      changeCount: preview.changes.length,
    });

    return preview;
  }

  // ─── Auto-snapshot ────────────────────────────────────────────────

  autoSnapshot(): Snapshot | null {
    const lastSnapshot = this.listSnapshots({ limit: 1 })[0];
    const currentHash = this.computeFilesHash(this.gatherSnapshotFiles());

    if (lastSnapshot && lastSnapshot.filesHash === currentHash) {
      return null; // No meaningful changes
    }

    return this.createSnapshot(undefined, 'auto');
  }

  // ─── Storage Info ─────────────────────────────────────────────────

  getSnapshotSize(): number {
    return dirSize(this.snapshotsDir);
  }

  // ─── Diagnostics ──────────────────────────────────────────────────

  diagnose(): DiagnosticResult {
    const snapshots = this.listSnapshots();
    const totalSize = this.getSnapshotSize();
    const latestSnapshot = snapshots[0] ?? null;

    const checks = [
      {
        name: 'Snapshot database',
        passed: true,
        message: `${snapshots.length} snapshots stored`,
      },
      {
        name: 'Snapshot storage',
        passed: totalSize < 500 * 1024 * 1024, // warn over 500MB
        message: `${formatBytes(totalSize)} used`,
      },
      {
        name: 'Latest snapshot',
        passed: latestSnapshot !== null,
        message: latestSnapshot
          ? `${latestSnapshot.label ?? 'unlabeled'} at ${latestSnapshot.createdAt}`
          : 'No snapshots yet',
      },
      {
        name: 'Snapshots directory',
        passed: existsSync(this.snapshotsDir),
        message: existsSync(this.snapshotsDir) ? 'Directory exists' : 'Directory missing',
      },
    ];

    const status = checks.every((c) => c.passed) ? 'healthy' : 'degraded';

    return { module: 'rewind', status, checks };
  }

  // ─── Cleanup ──────────────────────────────────────────────────────

  close(): void {
    this.db.close();
  }

  // ─── Private Helpers ──────────────────────────────────────────────

  private gatherSnapshotFiles(): string[] {
    const files: string[] = [];
    const candidates = [
      'memory.db',
      'tools.db',
      'tokens.db',
      'flags.db',
      'permissions.yaml',
    ];

    for (const file of candidates) {
      if (existsSync(join(this.dataDir, file))) {
        files.push(file);
      }
    }

    // Also include agent identity files
    const agentsDir = join(this.dataDir, 'agents');
    if (existsSync(agentsDir)) {
      try {
        const agentFiles = readdirSync(agentsDir).filter((f) => f.endsWith('.json'));
        for (const f of agentFiles) {
          files.push(join('agents', f));
        }
      } catch {
        // Ignore read errors
      }
    }

    return files;
  }

  private getSnapshotFileList(snapshotId: string): string[] {
    const row = this.db.prepare('SELECT metadata FROM snapshots WHERE id = ?').get(snapshotId) as { metadata: string } | undefined;
    if (!row) return [];
    try {
      const meta = JSON.parse(row.metadata);
      return meta.files ?? [];
    } catch {
      return [];
    }
  }

  private computeFilesHash(files: string[]): string {
    const hash = createHash('sha256');
    for (const file of files.sort()) {
      const filePath = join(this.dataDir, file);
      if (existsSync(filePath)) {
        hash.update(file);
        hash.update(readFileSync(filePath));
      }
    }
    return hash.digest('hex');
  }

  private isConfigFile(file: string): boolean {
    return file.endsWith('.yaml') || file.endsWith('.yml') || file.endsWith('.json');
  }

  private isMemoryFile(file: string): boolean {
    return file === 'memory.db';
  }
}

// ─── Row Mapping ──────────────────────────────────────────────────

interface SnapshotRow {
  id: string;
  label: string | null;
  trigger: string;
  created_at: string;
  file_count: number;
  total_bytes: number;
  files_hash: string;
  metadata: string;
}

function rowToSnapshot(row: SnapshotRow): Snapshot {
  return {
    id: row.id,
    label: row.label ?? undefined,
    trigger: row.trigger,
    createdAt: row.created_at,
    fileCount: row.file_count,
    totalBytes: row.total_bytes,
    filesHash: row.files_hash,
    metadata: JSON.parse(row.metadata),
  };
}

// ─── Utilities ────────────────────────────────────────────────────

function hashFile(filePath: string): string {
  return createHash('sha256').update(readFileSync(filePath)).digest('hex');
}

function dirSize(dir: string): number {
  if (!existsSync(dir)) return 0;
  let total = 0;
  try {
    const entries = readdirSync(dir, { withFileTypes: true });
    for (const entry of entries) {
      const fullPath = join(dir, entry.name);
      if (entry.isDirectory()) {
        total += dirSize(fullPath);
      } else {
        total += statSync(fullPath).size;
      }
    }
  } catch {
    // Ignore errors
  }
  return total;
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}
