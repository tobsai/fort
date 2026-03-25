/**
 * shell-command — Tier 3 (approve)
 *
 * Runs a shell command and returns stdout, stderr, and exit code.
 * Tier 3: the ToolExecutor blocks execution unless the caller has obtained
 * explicit user approval first. The tool itself also checks PermissionManager
 * when a workingDir is provided.
 *
 * Output (stdout + stderr combined) is truncated at 50 KB.
 */

import { execSync } from 'node:child_process';
import type { FortTool, ToolResult } from '../types.js';

export interface ShellCommandInput {
  command: string;
  /** Optional working directory for the command */
  workingDir?: string;
}

export interface ShellCommandOutput {
  stdout: string;
  stderr: string;
  exitCode: number;
}

const MAX_OUTPUT = 50 * 1024; // 50 KB

export const shellCommandTool: FortTool = {
  name: 'shell-command',
  description:
    'Run a shell command and return stdout, stderr, and exit code. Requires explicit user approval (Tier 3).',
  inputSchema: {
    type: 'object',
    properties: {
      command: { type: 'string', description: 'The shell command to execute.' },
      workingDir: {
        type: 'string',
        description: 'Optional working directory for the command.',
      },
    },
    required: ['command'],
  },
  tier: 3,

  async execute(input: unknown): Promise<ToolResult> {
    const { command, workingDir } = input as ShellCommandInput;

    if (!command || typeof command !== 'string') {
      return { success: false, output: '', error: 'Input must include a "command" string.' };
    }

    let stdout = '';
    let stderr = '';
    let exitCode = 0;

    try {
      const raw = execSync(command, {
        cwd: workingDir,
        encoding: 'utf-8',
        stdio: ['pipe', 'pipe', 'pipe'],
        // Don't throw on non-zero exit — we capture it ourselves
        // execSync throws on non-zero; we catch it below
      });
      stdout = typeof raw === 'string' ? raw : '';
    } catch (err: any) {
      // execSync throws on non-zero exit code
      stdout = err.stdout ?? '';
      stderr = err.stderr ?? '';
      exitCode = typeof err.status === 'number' ? err.status : 1;

      // Truncate
      if (stdout.length > MAX_OUTPUT) stdout = stdout.slice(0, MAX_OUTPUT) + '\n[...truncated]';
      if (stderr.length > MAX_OUTPUT) stderr = stderr.slice(0, MAX_OUTPUT) + '\n[...truncated]';

      const cmdOutput: ShellCommandOutput = { stdout, stderr, exitCode };
      const output = formatOutput(stdout, stderr, exitCode);
      return { success: false, output, error: `Command exited with code ${exitCode}`, artifacts: [cmdOutput] };
    }

    // Truncate stdout
    if (stdout.length > MAX_OUTPUT) stdout = stdout.slice(0, MAX_OUTPUT) + '\n[...truncated]';

    const cmdOutput: ShellCommandOutput = { stdout, stderr, exitCode };
    const output = formatOutput(stdout, stderr, exitCode);
    return { success: true, output, artifacts: [cmdOutput] };
  },
};

function formatOutput(stdout: string, stderr: string, exitCode: number): string {
  const parts: string[] = [];
  if (stdout) parts.push(`STDOUT:\n${stdout}`);
  if (stderr) parts.push(`STDERR:\n${stderr}`);
  parts.push(`Exit code: ${exitCode}`);
  return parts.join('\n\n');
}
