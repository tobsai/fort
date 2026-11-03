import { spawn, spawnSync } from 'node:child_process';
import { chmodSync, lstatSync, mkdirSync, readdirSync, readFileSync, realpathSync } from 'node:fs';
import { isAbsolute, join, normalize } from 'node:path';
import type { RuntimeDescriptor } from '../contracts/v1.ts';
import { ClaudeCodeChat, claudeCodeLaunch, claudeCodeRuntime, processEnding, type ClaudeCodeLaunch, type ClaudeCodeProcess } from './claude-code-chat.ts';
import { SETPRIV } from './linux-codex-process.ts';

// Claude on the cloud hub itself (Spec 062 and Spec 063, revision 2026-11-03). The official Claude Code CLI ships in the hub
// image at one exact version, and runs with the same switches and the same isolated environment the Mac uses: no tools,
// no settings, no MCP, no session files, and only a claude.ai subscription sign-in. Its home, workspace and temporary
// directories are private folders on the hub's volume. The hub has no browser, so the sign-in uses Claude's manual page,
// whose code the owner pastes into Fort's web page.
export const CLOUD_CLAUDE_ROOT = '/opt/fort/claude';
const VERSION = /^\d+\.\d+\.\d+$/;
const PARTS = ['home', 'workspace', 'tmp'] as const;
const exact = (value: string) => typeof value === 'string' && value.isWellFormed() && !value.includes('\0') && isAbsolute(value) && value !== '/' && normalize(value) === value && Buffer.byteLength(value) <= 4096;

// The version the image installed, from the one line the build wrote beside the binary.
export function installedCloudClaude(root = CLOUD_CLAUDE_ROOT): { version: string; executable: string } | undefined {
  const marker = lstatSync(join(root, 'version'), { throwIfNoEntry: false });
  if (!marker?.isFile()) return undefined;
  const version = readFileSync(join(root, 'version'), 'utf8').trim();
  if (!VERSION.test(version)) throw new Error('The hub image names no exact Claude Code version');
  const executable = join(root, version), binary = lstatSync(executable, { throwIfNoEntry: false });
  if (!binary?.isFile() || (binary.mode & 0o022) !== 0 || (binary.mode & 0o111) === 0) throw new Error('The hub image has no usable Claude Code binary');
  return { version, executable };
}

export function prepareCloudClaudeDirectory(directory: string): string {
  if (!exact(directory)) throw new Error('Use an exact absolute directory for Claude on the hub');
  for (const path of [directory, ...PARTS.map(part => join(directory, part))]) {
    mkdirSync(path, { recursive: true, mode: 0o700 });
    const info = lstatSync(path);
    if (!info.isDirectory() || info.uid !== process.getuid?.() || realpathSync(path) !== path) throw new Error('Claude directories on the hub must be canonical and owned by the hub');
    if ((info.mode & 0o777) !== 0o700) chmodSync(path, 0o700);
  }
  // Claude Code never gets a file to read or a place to write but its own home: the workspace stays empty.
  if (readdirSync(join(directory, 'workspace')).length !== 0) throw new Error('The Claude workspace on the hub must be empty');
  return directory;
}

// True once Claude is signed in on the hub. On Linux, Claude Code keeps its claude.ai sign-in in its home as a private file.
// Only the file's presence and privacy are read, never its contents; every turn still asks Claude Code which account it is.
export function cloudClaudeSignedIn(directory: string): boolean {
  const file = lstatSync(join(directory, 'home', '.claude', '.credentials.json'), { throwIfNoEntry: false });
  return !!file && file.isFile() && file.nlink === 1 && (file.mode & 0o077) === 0 && file.size > 0 && file.size <= 128 * 1024 && file.uid === process.getuid?.();
}

// Linux has no Fort guardian. setpriv has the kernel end Claude Code if the hub dies first, and Claude Code runs in its own
// process group, so ending a turn ends everything it started.
export function linuxClaudeLaunch(setpriv = SETPRIV): (plan: ClaudeCodeLaunch) => ClaudeCodeProcess {
  if (!exact(setpriv)) throw new Error('Select the exact setpriv');
  return plan => {
    const child = spawn(setpriv, ['--pdeathsig', 'KILL', '--', plan.executable, ...plan.args], { cwd: plan.cwd, env: plan.env, detached: true, stdio: ['pipe', 'pipe', 'pipe'] });
    child.on('error', () => child.stdout.destroy());
    return { stdin: child.stdin, stdout: child.stdout, ended: processEnding(child), kill: () => {
      if (child.pid === undefined || child.exitCode !== null || child.signalCode !== null) return;
      try { process.kill(-child.pid, 'SIGTERM'); } catch {}
    } };
  };
}

export type CloudClaude = { runtime: ClaudeCodeChat; provider: RuntimeDescriptor; signedIn(): boolean };

export function createCloudClaude(options: { executable: string; version: string; directory: string; hostId: string; report?: (line: string) => void; setpriv?: string }): CloudClaude {
  if (process.platform !== 'linux') throw new Error('Claude on the hub runs on Linux only');
  if (!exact(options.executable) || !VERSION.test(options.version)) throw new Error('Select the exact Claude Code binary and version');
  const directory = prepareCloudClaudeDirectory(options.directory);
  const home = join(directory, 'home'), workspace = join(directory, 'workspace'), runtimeDirectory = join(directory, 'tmp');
  const plan = claudeCodeLaunch({ executable: options.executable, workspace, home, runtimeDirectory });
  const answer = spawnSync(options.executable, ['--version'], { cwd: workspace, env: plan.env, encoding: 'utf8', timeout: 10_000, maxBuffer: 4096, stdio: ['ignore', 'pipe', 'ignore'] });
  if (answer.status !== 0 || !new RegExp(`^${options.version.replaceAll('.', '\\.')} \\(Claude Code\\)\\n?$`).test(answer.stdout)) throw new Error('The hub’s Claude Code version was not confirmed');
  const runtime = new ClaudeCodeChat({ executable: options.executable, workspace, home, runtimeDirectory, runtimeVersion: options.version, pasteCode: true,
    launch: linuxClaudeLaunch(options.setpriv), ...(options.report ? { report: options.report } : {}) });
  return { runtime, provider: { ...claudeCodeRuntime(options.version), hostId: options.hostId }, signedIn: () => cloudClaudeSignedIn(directory) };
}
