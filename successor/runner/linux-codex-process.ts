import { isAbsolute, normalize } from 'node:path';
import { launchOwnedProcess } from './owned-process.ts';

// The cloud hub's Codex launcher (Spec 063, revision 2026-11-03). Linux has no Fort guardian, so the kernel takes its
// place: setpriv asks for SIGKILL when the executor that started Codex dies, and the owned-process group still bounds
// every other exit. Same bounds as the Mac's guarded launch, so a cloud turn gets no more room than a Mac turn.
export const SETPRIV = '/usr/bin/setpriv';
type LinuxCodexOptions = {
  executable: string;
  args: readonly string[];
  cwd: string;
  env: Readonly<Record<string, string>>;
  deadlineMs: number;
  frameLimit?: number;
  // The process that asks for the parent-death signal. Only tests replace it.
  setpriv?: string;
};

const text = (value: string) => typeof value === 'string' && value.isWellFormed() && !value.includes('\0');
const path = (value: string) => text(value) && isAbsolute(value) && value !== '/' && normalize(value) === value && Buffer.byteLength(value) <= 4096;

export function linuxCodexCommand(options: Pick<LinuxCodexOptions, 'executable' | 'args' | 'setpriv'>): { executable: string; args: string[] } {
  return { executable: options.setpriv ?? SETPRIV, args: ['--pdeathsig', 'KILL', '--', options.executable, ...options.args] };
}

export function launchLinuxCodexProcess(options: LinuxCodexOptions) {
  if (process.platform !== 'linux') throw new Error('The cloud Codex launch runs on Linux only');
  const setpriv = options.setpriv ?? SETPRIV;
  if (![setpriv, options.executable, options.cwd].every(path)) throw new Error('Select exact normalized launcher, runtime and working-directory paths');
  if (!Number.isInteger(options.deadlineMs) || options.deadlineMs < 1 || options.deadlineMs > 120_000) throw new Error('Select an explicit runtime deadline of at most 120 seconds');
  const args = [...options.args], entries = Object.entries(options.env);
  if (args.length > 256 || !args.every(text) || args.reduce((sum, argument) => sum + Buffer.byteLength(argument) + 1, 0) > 65_536) throw new Error('Runtime arguments exceed their bounds');
  if (entries.length > 128 || !entries.every(([key, value]) => text(key) && key.length > 0 && !key.includes('=') && text(value))
      || entries.reduce((sum, [key, value]) => sum + Buffer.byteLength(key) + Buffer.byteLength(value) + 2, 0) > 65_536) throw new Error('Runtime environment exceeds its bounds');
  const command = linuxCodexCommand({ executable: options.executable, args, setpriv });
  return launchOwnedProcess({ executable: command.executable, args: command.args, cwd: options.cwd, env: Object.fromEntries(entries),
    deadlineMs: options.deadlineMs, terminationGraceMs: 250, killGraceMs: 750, ...(options.frameLimit === undefined ? {} : { frameLimit: options.frameLimit }) });
}
