import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync, readFileSync, existsSync, writeFileSync } from 'node:fs';
import { writeFileTool } from '../../tools/builtins/write-file.js';

describe('write-file tool', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-write-file-'));
    process.env['FORT_WRITE_DIRS'] = tmpDir;
  });

  afterEach(() => {
    delete process.env['FORT_WRITE_DIRS'];
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('has correct metadata', () => {
    expect(writeFileTool.name).toBe('write-file');
    expect(writeFileTool.tier).toBe(2);
    expect(writeFileTool.description).toBeTruthy();
  });

  it('creates a new file and returns a diff preview', async () => {
    const filePath = join(tmpDir, 'new.txt');
    const result = await writeFileTool.execute({ path: filePath, content: 'Hello!' });

    expect(result.success).toBe(true);
    expect(existsSync(filePath)).toBe(true);
    expect(readFileSync(filePath, 'utf-8')).toBe('Hello!');
    expect(result.output).toMatch(/Created/i);
  });

  it('updates an existing file and shows diff', async () => {
    const filePath = join(tmpDir, 'existing.txt');
    writeFileSync(filePath, 'old content', 'utf-8');

    const result = await writeFileTool.execute({ path: filePath, content: 'new content' });

    expect(result.success).toBe(true);
    expect(readFileSync(filePath, 'utf-8')).toBe('new content');
    expect(result.output).toMatch(/Updated/i);
    // Diff should show removed old and added new lines
    expect(result.output).toContain('- old content');
    expect(result.output).toContain('+ new content');
  });

  it('rejects a path outside the allowed directory', async () => {
    const result = await writeFileTool.execute({ path: '/etc/evil.txt', content: 'bad' });

    expect(result.success).toBe(false);
    expect(result.error).toMatch(/outside the allowed/i);
  });

  it('returns error when path is missing', async () => {
    const result = await writeFileTool.execute({ content: 'hi' });
    expect(result.success).toBe(false);
    expect(result.error).toMatch(/path/i);
  });

  it('returns error when content is missing', async () => {
    const result = await writeFileTool.execute({ path: join(tmpDir, 'foo.txt') });
    expect(result.success).toBe(false);
    expect(result.error).toMatch(/content/i);
  });

  it('creates parent directories as needed', async () => {
    const filePath = join(tmpDir, 'a', 'b', 'c', 'deep.txt');
    const result = await writeFileTool.execute({ path: filePath, content: 'deep' });

    expect(result.success).toBe(true);
    expect(readFileSync(filePath, 'utf-8')).toBe('deep');
  });

  it('includes artifact with path, isNew, and diff', async () => {
    const filePath = join(tmpDir, 'artifact.txt');
    const result = await writeFileTool.execute({ path: filePath, content: 'data' });

    expect(result.artifacts).toHaveLength(1);
    const artifact = result.artifacts![0] as any;
    expect(artifact.path).toBe(filePath);
    expect(artifact.isNew).toBe(true);
    expect(typeof artifact.diff).toBe('string');
  });

  it('returns (no changes) when content is identical to existing file', async () => {
    const filePath = join(tmpDir, 'same.txt');
    writeFileSync(filePath, 'same content', 'utf-8');

    const result = await writeFileTool.execute({ path: filePath, content: 'same content' });

    expect(result.success).toBe(true);
    expect(result.output).toContain('(no changes)');
  });
});
