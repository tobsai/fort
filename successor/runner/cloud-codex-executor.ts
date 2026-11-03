import { chmodSync, lstatSync, mkdirSync, realpathSync } from 'node:fs';
import { isAbsolute, join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';
import { ExecutorProcess } from './executor-process.ts';
import { serveCodexExecutor } from './codex-executor.ts';
import { privateCodexLaunchPlan } from './private-codex-plan.ts';
import { launchLinuxCodexProcess, SETPRIV } from './linux-codex-process.ts';
import { CODEX_RUNTIME } from './codex-descriptor.ts';

// ChatGPT on the cloud hub itself (Spec 063, revision 2026-11-03). The same Codex 0.163.0 controls the Mac uses (no shell,
// no files, no network tools, ChatGPT sign-in only), in private directories on the hub's volume, behind the same executor
// worker. The worker is its own process group, so a stuck turn is ended with everything it started.
export type CloudCodexOptions = { executable: string; directory: string; hostId: string };

const exact = (value: string) => typeof value === 'string' && value.isWellFormed() && !value.includes('\0') && isAbsolute(value) && value !== '/' && normalize(value) === value && Buffer.byteLength(value) <= 4096;
const PARTS = ['home', 'state', 'workspace', 'tmp'] as const;

// Makes the four private directories the launch plan checks: owned by this user, mode 0700, nothing ambient inside.
export function prepareCloudCodexDirectory(directory: string): string {
  if (!exact(directory)) throw new Error('Use an exact absolute directory for the cloud runner');
  for (const path of [directory, ...PARTS.map(part => join(directory, part))]) {
    mkdirSync(path, { recursive: true, mode: 0o700 });
    const info = lstatSync(path);
    if (!info.isDirectory() || info.uid !== process.getuid?.() || realpathSync(path) !== path) throw new Error('Cloud runner directories must be canonical and owned by the hub');
    if ((info.mode & 0o777) !== 0o700) chmodSync(path, 0o700);
  }
  return directory;
}

// True once ChatGPT is signed in on the hub: Codex keeps its sign-in in its own state directory, as a private file.
// Only the file's presence and privacy are read, never its contents.
export function cloudCodexSignedIn(directory: string): boolean {
  const auth = lstatSync(join(directory, 'state', 'auth.json'), { throwIfNoEntry: false });
  return !!auth && auth.isFile() && auth.nlink === 1 && (auth.mode & 0o077) === 0 && auth.size > 0 && auth.size <= 128 * 1024 && auth.uid === process.getuid?.();
}

export function createCloudCodexExecutorProcess(options: CloudCodexOptions): ExecutorProcess {
  if (!exact(options.executable)) throw new Error('Use the exact Codex executable');
  prepareCloudCodexDirectory(options.directory);
  const worker = fileURLToPath(import.meta.url), binding = { provider: CODEX_RUNTIME.provider, runtimeVersion: CODEX_RUNTIME.runtimeVersion, hostId: options.hostId };
  return new ExecutorProcess({ executable: process.execPath, args: ['--no-warnings', worker, '--executable', options.executable, '--directory', options.directory, '--host', options.hostId],
    cwd: options.directory, env: {}, lifecycle: { kind: 'processGroup' }, cooperativeCancellationMs: 3500, discovery: binding, signIn: binding, portrait: binding });
}

if (import.meta.main) {
  try {
    if (process.platform !== 'linux') throw new Error('The cloud executor runs on Linux only');
    const { values } = parseArgs({ options: { executable: { type: 'string' }, directory: { type: 'string' }, host: { type: 'string' } }, strict: true, allowPositionals: false });
    if (!values.executable || !values.directory || !values.host) throw new Error('Missing cloud runtime configuration');
    prepareCloudCodexDirectory(values.directory);
    const plan = (profile: 'chat' | 'portrait') => privateCodexLaunchPlan({ executable: values.executable!, guardian: SETPRIV, directory: values.directory!, profile });
    await serveCodexExecutor({ input: process.stdin, output: process.stdout, hostId: values.host, runtime: plan('chat'), portraitRuntime: plan('portrait'),
      launch: options => launchLinuxCodexProcess(options) });
  } catch {
    process.stderr.write('Fort cloud executor could not start or finish safely.\n');
    process.exitCode = 1;
  }
}
