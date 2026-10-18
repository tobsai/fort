import { isAbsolute, normalize } from 'node:path';
import { launchOwnedProcess } from './owned-process.ts';

type GuardedCodexOptions = {
  guardian: string;
  executable: string;
  args: readonly string[];
  cwd: string;
  env: Readonly<Record<string, string>>;
  deadlineMs: number;
};

function text(value: string): boolean {
  return typeof value === 'string' && value.isWellFormed() && !value.includes('\0');
}
function path(value: string): boolean {
  return text(value) && isAbsolute(value) && value !== '/' && normalize(value) === value && Buffer.byteLength(value) <= 4096;
}

export function launchGuardedCodexProcess(options: GuardedCodexOptions) {
  if (process.platform !== 'darwin') throw new Error('Guarded Codex process requires macOS');
  if (![options.guardian, options.executable, options.cwd].every(path)) throw new Error('Select exact normalized guardian, runtime and working-directory paths');
  if (!Number.isInteger(options.deadlineMs) || options.deadlineMs < 1 || options.deadlineMs > 120_000) throw new Error('Select an explicit runtime deadline of at most 120 seconds');
  const args = [...options.args], entries = Object.entries(options.env);
  if (args.length > 256 || !args.every(text) || args.reduce((sum, argument) => sum + Buffer.byteLength(argument) + 1, 0) > 65_536) throw new Error('Runtime arguments exceed their bounds');
  if (entries.length > 128 || !entries.every(([key, value]) => text(key) && key.length > 0 && !key.includes('=') && text(value))
      || entries.reduce((sum, [key, value]) => sum + Buffer.byteLength(key) + Buffer.byteLength(value) + 2, 0) > 65_536) throw new Error('Runtime environment exceeds its bounds');
  return launchOwnedProcess({ executable: options.guardian, args: [String(process.pid), options.executable, ...args], cwd: options.cwd,
    env: Object.fromEntries(entries), deadlineMs: options.deadlineMs, terminationGraceMs: 250, killGraceMs: 750 });
}
