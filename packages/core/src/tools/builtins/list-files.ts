/**
 * list-files — Tier 1 (auto)
 *
 * Lists the contents of a directory (files and subdirectories).
 * Restricted to directories within the allowed list.
 */

import { readdirSync, statSync, existsSync } from 'node:fs';
import { resolve, normalize, join } from 'node:path';
import type { FortTool, ToolResult } from '../types.js';

export interface ListFilesInput {
  path: string;
}

export interface FileEntry {
  name: string;
  type: 'file' | 'directory' | 'symlink' | 'other';
  size?: number;
}

function getAllowedDirs(): string[] {
  const env = process.env['FORT_READ_DIRS'];
  if (env) return env.split(':').map((d) => resolve(d));
  return [resolve(process.env['HOME'] ?? '/tmp')];
}

function assertAllowed(dirPath: string, allowedDirs: string[]): void {
  const resolved = resolve(normalize(dirPath));
  const allowed = allowedDirs.some(
    (dir) => resolved.startsWith(dir + '/') || resolved === dir || dir.startsWith(resolved + '/'),
  );
  if (!allowed) {
    throw new Error(
      `Path "${dirPath}" is outside the allowed directories: ${allowedDirs.join(', ')}`,
    );
  }
}

export const listFilesTool: FortTool = {
  name: 'list-files',
  description: 'List the contents of a directory (files and subdirectories).',
  inputSchema: {
    type: 'object',
    properties: {
      path: { type: 'string', description: 'Absolute or relative path to the directory to list.' },
    },
    required: ['path'],
  },
  tier: 1,

  async execute(input: unknown): Promise<ToolResult> {
    const { path } = input as ListFilesInput;
    if (!path || typeof path !== 'string') {
      return { success: false, output: '', error: 'Input must include a "path" string.' };
    }

    const allowedDirs = getAllowedDirs();

    try {
      assertAllowed(path, allowedDirs);
    } catch (err) {
      return { success: false, output: '', error: (err as Error).message };
    }

    const resolved = resolve(path);

    if (!existsSync(resolved)) {
      return { success: false, output: '', error: `Path does not exist: ${resolved}` };
    }

    let entries: FileEntry[];
    try {
      const names = readdirSync(resolved);
      entries = names.map((name) => {
        const full = join(resolved, name);
        try {
          const stat = statSync(full);
          if (stat.isDirectory()) return { name, type: 'directory' as const };
          if (stat.isSymbolicLink()) return { name, type: 'symlink' as const, size: stat.size };
          if (stat.isFile()) return { name, type: 'file' as const, size: stat.size };
          return { name, type: 'other' as const };
        } catch {
          return { name, type: 'other' as const };
        }
      });
    } catch (err) {
      return { success: false, output: '', error: (err as Error).message };
    }

    const lines = entries.map((e) => {
      const suffix = e.type === 'directory' ? '/' : '';
      const size = e.size !== undefined ? ` (${e.size} bytes)` : '';
      return `${e.name}${suffix}${size}`;
    });

    const output = lines.length > 0 ? lines.join('\n') : '(empty directory)';
    return { success: true, output, artifacts: entries };
  },
};
