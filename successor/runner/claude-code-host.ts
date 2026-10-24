import { spawn, spawnSync } from 'node:child_process';
import { lstatSync, mkdirSync, readdirSync, realpathSync } from 'node:fs';
import { isAbsolute, join, normalize } from 'node:path';
import type { RuntimeDescriptor } from '../contracts/v1.ts';
import { ClaudeCodeChat, claudeCodeLaunch, claudeCodeRuntime, type ClaudeCodeLaunch, type ClaudeCodeProcess } from './claude-code-chat.ts';

// Claude-only host mode (Spec 062): the owner copied their own `claude` binary into
// <data>/ClaudeCode/claude. Its presence is the explicit choice; Fort never downloads or modifies it.
export const CLAUDE_CODE_DIRECTORY = 'ClaudeCode';
type HostOptions = { directory: string; guardian: string; temporary: string };
export type ClaudeCodeHost = { runtime: ClaudeCodeChat; provider: RuntimeDescriptor; dataDirectory: string };

const exactPath = (path: string) => typeof path === 'string' && isAbsolute(path) && path !== '/' && normalize(path) === path && path.isWellFormed() && !path.includes('\0') && Buffer.byteLength(path) <= 4096;

export function selectedClaudeCode(dataDirectory: string): string | undefined {
  const directory = join(dataDirectory, CLAUDE_CODE_DIRECTORY);
  return lstatSync(join(directory, 'claude'), { throwIfNoEntry: false }) ? directory : undefined;
}

function ensurePrivate(path: string): string {
  if (!lstatSync(path, { throwIfNoEntry: false })) mkdirSync(path, { mode: 0o700 });
  const info = lstatSync(path);
  if (!info.isDirectory() || info.uid !== process.getuid?.() || (info.mode & 0o777) !== 0o700 || realpathSync(path) !== path) throw new Error('Claude Code directories must be canonical and private');
  return path;
}

// The guardian ends Claude Code's whole process group if this host exits first.
export function guardedClaudeLaunch(guardian: string): (plan: ClaudeCodeLaunch) => ClaudeCodeProcess {
  if (!exactPath(guardian)) throw new Error('Select an exact guardian');
  return plan => {
    const child = spawn(guardian, [String(process.pid), plan.executable, ...plan.args], { cwd: plan.cwd, env: plan.env, detached: true, stdio: ['pipe', 'pipe', 'ignore'] });
    child.on('error', () => child.stdout.destroy());
    return { stdin: child.stdin, stdout: child.stdout, kill: () => {
      if (child.pid === undefined || child.exitCode !== null || child.signalCode !== null) return;
      try { process.kill(-child.pid, 'SIGTERM'); } catch {}
    } };
  };
}

export function createClaudeCodeHost({ directory, guardian, temporary }: HostOptions): ClaudeCodeHost {
  if (![directory, guardian, temporary].every(exactPath) || !process.getuid) throw new Error('Select exact Claude Code host paths');
  const executable = join(directory, 'claude'), home = join(directory, 'home'), workspace = join(directory, 'workspace');
  const plan = claudeCodeLaunch({ executable, workspace, home, runtimeDirectory: temporary });
  ensurePrivate(directory);
  const binary = lstatSync(executable);
  // A regular file the owner's account owns and only it can change; never a link to an installation that updates itself.
  if (!binary.isFile() || binary.nlink !== 1 || binary.uid !== process.getuid() || (binary.mode & 0o022) !== 0 || (binary.mode & 0o100) === 0) throw new Error('The copied Claude Code binary is not private');
  for (const path of [home, workspace, temporary]) ensurePrivate(path);
  const dataDirectory = ensurePrivate(join(directory, 'data'));
  if (readdirSync(workspace).length !== 0) throw new Error('The Claude Code workspace must be empty');
  const version = spawnSync(executable, ['--version'], { cwd: workspace, env: plan.env, encoding: 'utf8', timeout: 10_000, maxBuffer: 4096, stdio: ['ignore', 'pipe', 'ignore'] });
  const match = version.status === 0 ? /^(\d+\.\d+\.\d+) \(Claude Code\)\n?$/.exec(version.stdout) : null;
  if (!match) throw new Error('The copied Claude Code version was not confirmed');
  const runtimeVersion = match[1]!;
  const runtime = new ClaudeCodeChat({ executable, workspace, home, runtimeDirectory: temporary, runtimeVersion, launch: guardedClaudeLaunch(guardian) });
  return { runtime, provider: claudeCodeRuntime(runtimeVersion), dataDirectory };
}
