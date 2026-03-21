import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync, writeFileSync, mkdirSync, existsSync, readFileSync } from 'node:fs';
import { RewindManager } from '../rewind/index.js';
import { ModuleBus } from '../module-bus/index.js';

describe('RewindManager', () => {
  let tmpDir: string;
  let dataDir: string;
  let manager: RewindManager;
  let bus: ModuleBus;

  function setup() {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-rewind-'));
    dataDir = join(tmpDir, 'data');
    mkdirSync(dataDir, { recursive: true });
    bus = new ModuleBus();
    manager = new RewindManager(join(dataDir, 'rewind.db'), dataDir, bus);
    return manager;
  }

  function seedDataFiles() {
    writeFileSync(join(dataDir, 'memory.db'), 'fake-memory-data');
    writeFileSync(join(dataDir, 'permissions.yaml'), 'defaults:\n  commandLine: deny\n');
  }

  function seedAgentFiles() {
    const agentsDir = join(dataDir, 'agents');
    mkdirSync(agentsDir, { recursive: true });
    writeFileSync(join(agentsDir, 'agent-1.json'), JSON.stringify({ id: 'agent-1', name: 'Test Agent' }));
  }

  afterEach(() => {
    manager?.close();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should create a snapshot with metadata', () => {
    setup();
    seedDataFiles();

    const snapshot = manager.createSnapshot('initial backup');

    expect(snapshot.id).toBeTruthy();
    expect(snapshot.label).toBe('initial backup');
    expect(snapshot.trigger).toBe('manual');
    expect(snapshot.fileCount).toBeGreaterThan(0);
    expect(snapshot.totalBytes).toBeGreaterThan(0);
    expect(snapshot.createdAt).toBeTruthy();
    expect(snapshot.filesHash).toBeTruthy();
  });

  it('should create a snapshot without a label', () => {
    setup();
    seedDataFiles();

    const snapshot = manager.createSnapshot();

    expect(snapshot.id).toBeTruthy();
    expect(snapshot.label).toBeUndefined();
    expect(snapshot.trigger).toBe('manual');
  });

  it('should include agent identity files in snapshots', () => {
    setup();
    seedDataFiles();
    seedAgentFiles();

    const snapshot = manager.createSnapshot('with agents');

    expect(snapshot.fileCount).toBeGreaterThanOrEqual(3); // memory.db, permissions.yaml, agent-1.json
    expect(snapshot.metadata).toHaveProperty('files');
    const files = snapshot.metadata.files as string[];
    expect(files.some((f: string) => f.includes('agent-1.json'))).toBe(true);
  });

  it('should list snapshots with time filtering', () => {
    setup();
    seedDataFiles();

    manager.createSnapshot('first');
    manager.createSnapshot('second');
    manager.createSnapshot('third');

    const all = manager.listSnapshots();
    expect(all.length).toBe(3);
    // Most recent first
    expect(all[0].label).toBe('third');
    expect(all[2].label).toBe('first');

    const limited = manager.listSnapshots({ limit: 2 });
    expect(limited.length).toBe(2);

    const future = manager.listSnapshots({ since: new Date(Date.now() + 100000) });
    expect(future.length).toBe(0);
  });

  it('should preview showing changes', () => {
    setup();
    seedDataFiles();

    const snapshot = manager.createSnapshot('before change');

    // Modify a file
    writeFileSync(join(dataDir, 'memory.db'), 'modified-memory-data');

    const preview = manager.previewRestore(snapshot.id);

    expect(preview.snapshotId).toBe(snapshot.id);
    expect(preview.hasChanges).toBe(true);
    expect(preview.changes.length).toBeGreaterThan(0);

    const memChange = preview.changes.find((c) => c.file === 'memory.db');
    expect(memChange).toBeTruthy();
    expect(memChange!.type).toBe('modified');
  });

  it('should preview with no changes when state is same', () => {
    setup();
    seedDataFiles();

    const snapshot = manager.createSnapshot('current');

    const preview = manager.previewRestore(snapshot.id);

    expect(preview.hasChanges).toBe(false);
    expect(preview.changes.length).toBe(0);
  });

  it('should detect added files in preview (file removed since snapshot)', () => {
    setup();
    seedDataFiles();
    writeFileSync(join(dataDir, 'tools.db'), 'fake-tools-data');

    const snapshot = manager.createSnapshot('with tools');

    // Remove the tools.db file
    rmSync(join(dataDir, 'tools.db'));

    const preview = manager.previewRestore(snapshot.id);
    const addedChange = preview.changes.find((c) => c.file === 'tools.db');
    expect(addedChange).toBeTruthy();
    expect(addedChange!.type).toBe('added');
  });

  it('should perform a full restore', () => {
    setup();
    seedDataFiles();

    const snapshot = manager.createSnapshot('before changes');

    // Modify files
    writeFileSync(join(dataDir, 'memory.db'), 'COMPLETELY-DIFFERENT-DATA');
    writeFileSync(join(dataDir, 'permissions.yaml'), 'defaults:\n  commandLine: allow\n');

    const result = manager.restore(snapshot.id);

    expect(result.hasChanges).toBe(true);

    // Verify files were restored
    const memoryContent = readFileSync(join(dataDir, 'memory.db'), 'utf-8');
    expect(memoryContent).toBe('fake-memory-data');

    const permContent = readFileSync(join(dataDir, 'permissions.yaml'), 'utf-8');
    expect(permContent).toBe('defaults:\n  commandLine: deny\n');
  });

  it('should perform config-only partial restore', () => {
    setup();
    seedDataFiles();

    const snapshot = manager.createSnapshot('config baseline');

    // Modify both config and memory
    writeFileSync(join(dataDir, 'memory.db'), 'MODIFIED-MEMORY');
    writeFileSync(join(dataDir, 'permissions.yaml'), 'modified-config');

    manager.restore(snapshot.id, { configOnly: true });

    // Config should be restored
    const permContent = readFileSync(join(dataDir, 'permissions.yaml'), 'utf-8');
    expect(permContent).toBe('defaults:\n  commandLine: deny\n');

    // Memory should NOT be restored (not a config file)
    const memoryContent = readFileSync(join(dataDir, 'memory.db'), 'utf-8');
    expect(memoryContent).toBe('MODIFIED-MEMORY');
  });

  it('should perform memory-only partial restore', () => {
    setup();
    seedDataFiles();

    const snapshot = manager.createSnapshot('memory baseline');

    // Modify both config and memory
    writeFileSync(join(dataDir, 'memory.db'), 'MODIFIED-MEMORY');
    writeFileSync(join(dataDir, 'permissions.yaml'), 'modified-config');

    manager.restore(snapshot.id, { memoryOnly: true });

    // Memory should be restored
    const memoryContent = readFileSync(join(dataDir, 'memory.db'), 'utf-8');
    expect(memoryContent).toBe('fake-memory-data');

    // Config should NOT be restored
    const permContent = readFileSync(join(dataDir, 'permissions.yaml'), 'utf-8');
    expect(permContent).toBe('modified-config');
  });

  it('should skip auto-snapshot when no changes detected', () => {
    setup();
    seedDataFiles();

    const first = manager.autoSnapshot();
    expect(first).not.toBeNull();
    expect(first!.trigger).toBe('auto');

    const second = manager.autoSnapshot();
    expect(second).toBeNull(); // No changes since last snapshot
  });

  it('should create auto-snapshot when changes detected', () => {
    setup();
    seedDataFiles();

    const first = manager.autoSnapshot();
    expect(first).not.toBeNull();

    // Modify a file
    writeFileSync(join(dataDir, 'memory.db'), 'changed-memory');

    const second = manager.autoSnapshot();
    expect(second).not.toBeNull();
    expect(second!.trigger).toBe('auto');
  });

  it('should return snapshot storage size', () => {
    setup();
    seedDataFiles();

    manager.createSnapshot('size test');

    const size = manager.getSnapshotSize();
    expect(size).toBeGreaterThan(0);
  });

  it('should return correct diagnostics', () => {
    setup();
    seedDataFiles();

    manager.createSnapshot('diagnostic test');

    const diag = manager.diagnose();
    expect(diag.module).toBe('rewind');
    expect(diag.status).toBe('healthy');
    expect(diag.checks.length).toBe(4);

    const dbCheck = diag.checks.find((c) => c.name === 'Snapshot database');
    expect(dbCheck).toBeTruthy();
    expect(dbCheck!.passed).toBe(true);
    expect(dbCheck!.message).toContain('1 snapshots');

    const latestCheck = diag.checks.find((c) => c.name === 'Latest snapshot');
    expect(latestCheck).toBeTruthy();
    expect(latestCheck!.passed).toBe(true);
  });

  it('should report no snapshots in diagnostics', () => {
    setup();

    const diag = manager.diagnose();
    expect(diag.module).toBe('rewind');

    const latestCheck = diag.checks.find((c) => c.name === 'Latest snapshot');
    expect(latestCheck).toBeTruthy();
    expect(latestCheck!.passed).toBe(false);
    expect(latestCheck!.message).toContain('No snapshots');
  });

  it('should publish bus events on snapshot creation', async () => {
    setup();
    seedDataFiles();

    const events: string[] = [];
    bus.subscribe('rewind.snapshot_created', () => { events.push('created'); });

    manager.createSnapshot('event test');
    await new Promise((r) => setTimeout(r, 10));

    expect(events).toContain('created');
  });

  it('should publish bus events on restore', async () => {
    setup();
    seedDataFiles();

    const events: string[] = [];
    bus.subscribe('rewind.restored', () => { events.push('restored'); });

    const snapshot = manager.createSnapshot('restore event test');
    writeFileSync(join(dataDir, 'memory.db'), 'changed');

    manager.restore(snapshot.id);
    await new Promise((r) => setTimeout(r, 10));

    expect(events).toContain('restored');
  });

  it('should publish bus events on preview', async () => {
    setup();
    seedDataFiles();

    const events: string[] = [];
    bus.subscribe('rewind.preview', () => { events.push('preview'); });

    const snapshot = manager.createSnapshot('preview event test');

    manager.previewRestore(snapshot.id);
    await new Promise((r) => setTimeout(r, 10));

    expect(events).toContain('preview');
  });

  it('should throw on preview/restore with nonexistent snapshot', () => {
    setup();

    expect(() => manager.previewRestore('nonexistent')).toThrow('Snapshot not found');
    expect(() => manager.restore('nonexistent')).toThrow('Snapshot not found');
  });

  it('should retrieve a snapshot by id', () => {
    setup();
    seedDataFiles();

    const created = manager.createSnapshot('get test');
    const retrieved = manager.getSnapshot(created.id);

    expect(retrieved).not.toBeNull();
    expect(retrieved!.id).toBe(created.id);
    expect(retrieved!.label).toBe('get test');
  });

  it('should return null for nonexistent snapshot id', () => {
    setup();

    const result = manager.getSnapshot('nonexistent');
    expect(result).toBeNull();
  });
});
