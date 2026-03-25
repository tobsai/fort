import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync, writeFileSync, mkdirSync } from 'node:fs';
import { readFileTool } from '../../tools/builtins/read-file.js';

describe('read-file tool', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-read-file-'));
    process.env['FORT_READ_DIRS'] = tmpDir;
  });

  afterEach(() => {
    delete process.env['FORT_READ_DIRS'];
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('has correct metadata', () => {
    expect(readFileTool.name).toBe('read-file');
    expect(readFileTool.tier).toBe(1);
    expect(readFileTool.description).toBeTruthy();
    expect(readFileTool.inputSchema).toMatchObject({ type: 'object' });
  });

  it('reads an allowed file and returns its contents', async () => {
    const filePath = join(tmpDir, 'hello.txt');
    writeFileSync(filePath, 'Hello, Fort!', 'utf-8');

    const result = await readFileTool.execute({ path: filePath });

    expect(result.success).toBe(true);
    expect(result.output).toBe('Hello, Fort!');
    expect(result.error).toBeUndefined();
  });

  it('rejects a path outside the allowed directory', async () => {
    const result = await readFileTool.execute({ path: '/etc/passwd' });

    expect(result.success).toBe(false);
    expect(result.error).toMatch(/outside the allowed/i);
  });

  it('returns error when file does not exist', async () => {
    const result = await readFileTool.execute({ path: join(tmpDir, 'nonexistent.txt') });

    expect(result.success).toBe(false);
    expect(result.error).toBeTruthy();
  });

  it('returns error when path is missing', async () => {
    const result = await readFileTool.execute({});
    expect(result.success).toBe(false);
    expect(result.error).toMatch(/path/i);
  });

  it('truncates files larger than 50 KB', async () => {
    const filePath = join(tmpDir, 'big.txt');
    // Write slightly over 50 KB
    writeFileSync(filePath, 'x'.repeat(52 * 1024), 'utf-8');

    const result = await readFileTool.execute({ path: filePath });

    expect(result.success).toBe(true);
    expect(result.output).toContain('[...truncated at 50 KB]');
    expect(result.artifacts).toEqual([{ truncated: true }]);
  });

  it('does not set truncated artifact for small files', async () => {
    const filePath = join(tmpDir, 'small.txt');
    writeFileSync(filePath, 'small content', 'utf-8');

    const result = await readFileTool.execute({ path: filePath });

    expect(result.success).toBe(true);
    expect(result.artifacts).toBeUndefined();
  });

  it('rejects path traversal attempts', async () => {
    const result = await readFileTool.execute({ path: join(tmpDir, '../../etc/passwd') });

    expect(result.success).toBe(false);
    expect(result.error).toMatch(/outside the allowed/i);
  });

  it('reads files in subdirectories of allowed dir', async () => {
    const subDir = join(tmpDir, 'sub');
    mkdirSync(subDir);
    const filePath = join(subDir, 'nested.txt');
    writeFileSync(filePath, 'nested content', 'utf-8');

    const result = await readFileTool.execute({ path: filePath });

    expect(result.success).toBe(true);
    expect(result.output).toBe('nested content');
  });
});
