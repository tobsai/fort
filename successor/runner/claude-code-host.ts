import { spawn, spawnSync } from 'node:child_process';
import { lstatSync, mkdirSync, readFileSync, readdirSync, realpathSync, renameSync, writeFileSync } from 'node:fs';
import { userInfo } from 'node:os';
import { isAbsolute, join, normalize } from 'node:path';
import type { RuntimeDescriptor } from '../contracts/v1.ts';
import { ClaudeCodeChat, claudeCodeLaunch, claudeCodeRuntime, type ClaudeCodeLaunch, type ClaudeCodeProcess } from './claude-code-chat.ts';

// Claude-only host mode (Spec 062). The owner's choice is <data>/ClaudeCode/selection.json, written by the app.
// Fort runs the owner's installed Claude Code in place, read-only (the helper's one sandbox exception is
// ~/.local/share/claude/), and never downloads, copies or changes it. The App Sandbox will not launch a
// binary placed in Fort's own container, so copying is not an option.
export const CLAUDE_CODE_DIRECTORY = 'ClaudeCode';
type HostOptions = { directory: string; guardian: string; temporary: string; installation?: string };
export type ClaudeCodeHost = { runtime: ClaudeCodeChat; provider: RuntimeDescriptor; dataDirectory: string };

const VERSION = /^\d+\.\d+\.\d+$/;
const exactPath = (path: string) => typeof path === 'string' && isAbsolute(path) && path !== '/' && normalize(path) === path && path.isWellFormed() && !path.includes('\0') && Buffer.byteLength(path) <= 4096;

export function selectedClaudeCode(dataDirectory: string): string | undefined {
  const directory = join(dataDirectory, CLAUDE_CODE_DIRECTORY);
  return lstatSync(join(directory, 'selection.json'), { throwIfNoEntry: false })?.isFile() ? directory : undefined;
}

function ensurePrivate(path: string): string {
  if (!lstatSync(path, { throwIfNoEntry: false })) mkdirSync(path, { mode: 0o700 });
  const info = lstatSync(path);
  if (!info.isDirectory() || info.uid !== process.getuid?.() || (info.mode & 0o777) !== 0o700 || realpathSync(path) !== path) throw new Error('Claude Code directories must be canonical and private');
  return path;
}

const newer = (left: string, right: string) => {
  const a = left.split('.').map(Number), b = right.split('.').map(Number);
  for (let i = 0; i < 3; i++) if (a[i] !== b[i]) return a[i]! > b[i]!;
  return false;
};

// The pinned version stays fixed across restarts so stored bindings keep matching. It is chosen once, as the newest
// installed version, and changes only when the owner chooses Claude again (the app removes pinned.json).
function pinnedVersion(directory: string, installation: string): string {
  const pinned = join(directory, 'pinned.json');
  if (lstatSync(pinned, { throwIfNoEntry: false })) {
    const row: unknown = JSON.parse(readFileSync(pinned, 'utf8'));
    if (!row || typeof row !== 'object' || Object.keys(row).join() !== 'version' || typeof (row as { version: unknown }).version !== 'string' || !VERSION.test((row as { version: string }).version)) throw new Error('Pinned Claude Code version is invalid');
    return (row as { version: string }).version;
  }
  const versions = readdirSync(installation).filter(name => VERSION.test(name) && lstatSync(join(installation, name)).isFile());
  const newest = versions.reduce<string | undefined>((best, name) => best === undefined || newer(name, best) ? name : best, undefined);
  if (!newest) throw new Error('No installed Claude Code version was found');
  writeFileSync(pinned + '.partial', JSON.stringify({ version: newest }) + '\n', { mode: 0o600, flag: 'w' });
  renameSync(pinned + '.partial', pinned);
  return newest;
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

export function createClaudeCodeHost({ directory, guardian, temporary, installation = join(userInfo().homedir, '.local/share/claude/versions') }: HostOptions): ClaudeCodeHost {
  if (![directory, guardian, temporary, installation].every(exactPath) || !process.getuid) throw new Error('Select exact Claude Code host paths');
  const home = join(directory, 'home'), workspace = join(directory, 'workspace');
  claudeCodeLaunch({ executable: join(installation, '0.0.0'), workspace, home, runtimeDirectory: temporary });
  ensurePrivate(directory);
  const runtimeVersion = pinnedVersion(directory, installation), executable = join(installation, runtimeVersion);
  const binary = lstatSync(executable, { throwIfNoEntry: false });
  if (!binary) throw new Error('The pinned Claude Code version is no longer installed. Choose Claude again to use the current version.');
  // The owner's own install: a regular file only their account can change, never a link.
  if (!binary.isFile() || binary.uid !== process.getuid() || (binary.mode & 0o022) !== 0 || (binary.mode & 0o100) === 0) throw new Error('The installed Claude Code binary is not owner-only');
  for (const path of [home, workspace, temporary]) ensurePrivate(path);
  const dataDirectory = ensurePrivate(join(directory, 'data'));
  if (readdirSync(workspace).length !== 0) throw new Error('The Claude Code workspace must be empty');
  const plan = claudeCodeLaunch({ executable, workspace, home, runtimeDirectory: temporary });
  const version = spawnSync(executable, ['--version'], { cwd: workspace, env: plan.env, encoding: 'utf8', timeout: 10_000, maxBuffer: 4096, stdio: ['ignore', 'pipe', 'ignore'] });
  if (version.status !== 0 || !new RegExp(`^${runtimeVersion.replaceAll('.', '\\.')} \\(Claude Code\\)\\n?$`).test(version.stdout)) throw new Error('The installed Claude Code version was not confirmed');
  const runtime = new ClaudeCodeChat({ executable, workspace, home, runtimeDirectory: temporary, runtimeVersion, launch: guardedClaudeLaunch(guardian) });
  return { runtime, provider: claudeCodeRuntime(runtimeVersion), dataDirectory };
}
