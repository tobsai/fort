/**
 * read-file — Tier 1 (auto)
 *
 * Reads the contents of a file from an allowed directory.
 * Throws if the resolved path escapes any entry in the allowlist.
 */

import { readFileSync } from 'node:fs';
import { resolve, normalize } from 'node:path';
import type { FortTool, ToolResult } from '../types.js';

export interface ReadFileInput {
  path: string;
}

/** Directories that read-file is allowed to access. Override via FORT_READ_DIRS env var. */
function getAllowedDirs(): string[] {
  const env = process.env['FORT_READ_DIRS'];
  if (env) return env.split(':').map((d) => resolve(d));
  // Safe default: only the user's home directory subtree
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

export const readFileTool: FortTool = {
  name: 'read-file',
  description: 'Read the contents of a file from an allowed directory.',
  inputSchema: {
    type: 'object',
    properties: {
      path: { type: 'string', description: 'Absolute or relative path to the file to read.' },
    },
    required: ['path'],
  },
  tier: 1,

  async execute(input: unknown): Promise<ToolResult> {
    const { path } = input as ReadFileInput;
    if (!path || typeof path !== 'string') {
      return { success: false, output: '', error: 'Input must include a "path" string.' };
    }

    const allowedDirs = getAllowedDirs();

    try {
      assertAllowed(path, allowedDirs);
    } catch (err) {
      return { success: false, output: '', error: (err as Error).message };
    }

    try {
      const contents = readFileSync(resolve(path), 'utf-8');
      // Truncate at 50 KB per spec
      const MAX = 50 * 1024;
      const truncated = contents.length > MAX;
      const output = truncated ? contents.slice(0, MAX) + '\n[...truncated at 50 KB]' : contents;
      return { success: true, output, artifacts: truncated ? [{ truncated: true }] : undefined };
    } catch (err) {
      return { success: false, output: '', error: (err as Error).message };
    }
  },
};
