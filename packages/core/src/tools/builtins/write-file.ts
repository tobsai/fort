/**
 * write-file — Tier 2 (draft)
 *
 * Writes or creates a file and returns a diff-style preview of the change.
 * The caller is responsible for the draft UX; the executor passes Tier 2 through.
 */

import { readFileSync, writeFileSync, existsSync, mkdirSync } from 'node:fs';
import { resolve, normalize, dirname } from 'node:path';
import type { FortTool, ToolResult } from '../types.js';

export interface WriteFileInput {
  path: string;
  content: string;
}

function getAllowedDirs(): string[] {
  const env = process.env['FORT_WRITE_DIRS'] ?? process.env['FORT_READ_DIRS'];
  if (env) return env.split(':').map((d) => resolve(d));
  return [resolve(process.env['HOME'] ?? '/tmp')];
}

function assertAllowed(filePath: string, allowedDirs: string[]): void {
  const resolved = resolve(normalize(filePath));
  const allowed = allowedDirs.some((dir) => resolved.startsWith(dir + '/') || resolved === dir);
  if (!allowed) {
    throw new Error(
      `Path "${filePath}" is outside the allowed directories: ${allowedDirs.join(', ')}`,
    );
  }
}

/** Produce a simple +/- unified-style diff preview (line-level, no context). */
function diffPreview(before: string, after: string): string {
  const beforeLines = before === '' ? [] : before.split('\n');
  const afterLines = after.split('\n');

  const removed = beforeLines.filter((l) => !afterLines.includes(l)).map((l) => `- ${l}`);
  const added = afterLines.filter((l) => !beforeLines.includes(l)).map((l) => `+ ${l}`);

  if (removed.length === 0 && added.length === 0) return '(no changes)';
  return [...removed, ...added].join('\n');
}

export const writeFileTool: FortTool = {
  name: 'write-file',
  description: 'Write or create a file. Returns a diff preview of the change (Tier 2 — draft).',
  inputSchema: {
    type: 'object',
    properties: {
      path: { type: 'string', description: 'Absolute or relative path to the file to write.' },
      content: { type: 'string', description: 'Full content to write to the file.' },
    },
    required: ['path', 'content'],
  },
  tier: 2,

  async execute(input: unknown): Promise<ToolResult> {
    const { path, content } = input as WriteFileInput;
    if (!path || typeof path !== 'string') {
      return { success: false, output: '', error: 'Input must include a "path" string.' };
    }
    if (typeof content !== 'string') {
      return { success: false, output: '', error: 'Input must include a "content" string.' };
    }

    const allowedDirs = getAllowedDirs();

    try {
      assertAllowed(path, allowedDirs);
    } catch (err) {
      return { success: false, output: '', error: (err as Error).message };
    }

    const resolved = resolve(path);
    const before = existsSync(resolved) ? readFileSync(resolved, 'utf-8') : '';
    const isNew = !existsSync(resolved);

    try {
      mkdirSync(dirname(resolved), { recursive: true });
      writeFileSync(resolved, content, 'utf-8');
    } catch (err) {
      return { success: false, output: '', error: (err as Error).message };
    }

    const diff = diffPreview(before, content);
    const action = isNew ? 'Created' : 'Updated';
    const output = `${action} ${resolved}\n\n${diff}`;

    return {
      success: true,
      output,
      artifacts: [{ path: resolved, isNew, diff }],
    };
  },
};
