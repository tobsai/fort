import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync, writeFileSync, mkdirSync } from 'node:fs';
import { listFilesTool } from '../../tools/builtins/list-files.js';

describe('list-files tool', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-list-files-'));
    process.env['FORT_READ_DIRS'] = tmpDir;
  });

  afterEach(() => {
    delete process.env['FORT_READ_DIRS'];
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('has correct metadata', () => {
    expect(listFilesTool.name).toBe('list-files');
    expect(listFilesTool.tier).toBe(1);
    expect(listFilesTool.description).toBeTruthy();
  });

  it('lists files and directories in an allowed directory', async () => {
    writeFileSync(join(tmpDir, 'file.txt'), 'hi', 'utf-8');
    mkdirSync(join(tmpDir, 'subdir'));

    const result = await listFilesTool.execute({ path: tmpDir });

    expect(result.success).toBe(true);
    expect(result.output).toContain('file.txt');
    expect(result.output).toContain('subdir/');
  });

  it('appends / suffix to directories', async () => {
    mkdirSync(join(tmpDir, 'mydir'));

    const result = await listFilesTool.execute({ path: tmpDir });

    expect(result.success).toBe(true);
    expect(result.output).toContain('mydir/');
  });

  it('returns (empty directory) for an empty directory', async () => {
    const emptyDir = join(tmpDir, 'empty');
    mkdirSync(emptyDir);

    const result = await listFilesTool.execute({ path: emptyDir });

    expect(result.success).toBe(true);
    expect(result.output).toBe('(empty directory)');
  });

  it('rejects a path outside the allowed directory', async () => {
    const result = await listFilesTool.execute({ path: '/etc' });

    expect(result.success).toBe(false);
    expect(result.error).toMatch(/outside the allowed/i);
  });

  it('returns error for a non-existent path', async () => {
    const result = await listFilesTool.execute({ path: join(tmpDir, 'nope') });

    expect(result.success).toBe(false);
    expect(result.error).toMatch(/does not exist/i);
  });

  it('returns error when path is missing', async () => {
    const result = await listFilesTool.execute({});
    expect(result.success).toBe(false);
    expect(result.error).toMatch(/path/i);
  });

  it('returns file entries as artifacts', async () => {
    writeFileSync(join(tmpDir, 'a.txt'), 'a', 'utf-8');

    const result = await listFilesTool.execute({ path: tmpDir });

    expect(result.success).toBe(true);
    expect(Array.isArray(result.artifacts)).toBe(true);
    const entry = (result.artifacts as any[]).find((e: any) => e.name === 'a.txt');
    expect(entry).toBeDefined();
    expect(entry.type).toBe('file');
    expect(typeof entry.size).toBe('number');
  });

  it('lists the allowed root directory itself', async () => {
    writeFileSync(join(tmpDir, 'root.txt'), 'root', 'utf-8');

    const result = await listFilesTool.execute({ path: tmpDir });

    expect(result.success).toBe(true);
    expect(result.output).toContain('root.txt');
  });
});
