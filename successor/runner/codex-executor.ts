import { readdirSync, openSync, closeSync, fstatSync, readSync, realpathSync, constants } from 'node:fs';
import { isAbsolute, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';
import type { Readable, Writable } from 'node:stream';
import { CodexChat } from './codex-chat.ts';
import { launchGuardedCodexProcess } from './guarded-codex-process.ts';
import { serveExecutorWorker } from './executor-worker.ts';
import { ExecutorProcess } from './executor-process.ts';
import { privateCodexLaunchPlan } from './private-codex-plan.ts';
export { CODEX_RUNTIME } from './codex-descriptor.ts';
export { privateCodexLaunchPlan } from './private-codex-plan.ts';
export { signInCodexAccount } from './codex-account.ts';

type CodexExecutorOptions = {
  input: Readable; output: Writable; hostId: string;
  runtime: Omit<Parameters<typeof launchGuardedCodexProcess>[0], 'deadlineMs'>;
};
type CodexProcessOptions = Omit<ConstructorParameters<typeof ExecutorProcess>[0], 'lifecycle' | 'cooperativeCancellationMs' | 'discovery' | 'signIn'> & { guardian: string; hostId: string };
type PrivateCodexOptions = Parameters<typeof privateCodexLaunchPlan>[0] & { hostId: string };

function compiledWorker(): string {
  const worker = fileURLToPath(import.meta.url);
  if (process.platform !== 'darwin' || process.version !== 'v26.5.0' || !worker.endsWith('.js')) throw new Error('Use the compiled executor artifact with Node 26.5.0 on macOS');
  return worker;
}

function nodeConfiguration(): string {
  const path = fileURLToPath(new URL('../node-openssl.cnf', import.meta.url));
  if (realpathSync(path) !== path) throw new Error('Unsafe Node configuration');
  const descriptor = openSync(path, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const info = fstatSync(descriptor), data = Buffer.alloc(1025);
    if (!info.isFile() || info.nlink !== 1 || info.mode & 0o022 || info.size > 1024 ||
        data.subarray(0, readSync(descriptor, data)).toString('utf8') !== 'nodejs_conf = fort_node\n[fort_node]\n') throw new Error('Invalid Node configuration');
  } finally { closeSync(descriptor); }
  return path;
}

export function createPrivateCodexExecutorProcess(options: PrivateCodexOptions): ExecutorProcess {
  const worker = compiledWorker(), configuration = nodeConfiguration(), plan = privateCodexLaunchPlan(options);
  return createCodexExecutorProcess({ executable: process.execPath,
    args: ['--jitless', '--no-experimental-strip-types', '--no-warnings', '--openssl-config=' + configuration, worker,
      '--executable', plan.executable, '--guardian', plan.guardian, '--directory', options.directory, '--host', options.hostId],
    cwd: options.directory, env: {}, guardian: plan.guardian, hostId: options.hostId });
}

export function createCodexExecutorProcess({ guardian, hostId, ...process }: CodexProcessOptions): ExecutorProcess {
  return new ExecutorProcess({ ...process, lifecycle: { kind: 'macParentExit', guardian }, cooperativeCancellationMs: 3500,
    discovery: { provider: 'codex', runtimeVersion: '0.163.0', hostId }, signIn: { provider: 'codex', runtimeVersion: '0.163.0', hostId } });
}

export function serveCodexExecutor(options: CodexExecutorOptions): Promise<void> {
  const runtime = { ...options.runtime, args: [...options.runtime.args], env: { ...options.runtime.env } };
  function create(deadline: number): CodexChat {
    if (!isAbsolute(runtime.cwd) || runtime.cwd === '/' || normalize(runtime.cwd) !== runtime.cwd || !runtime.cwd.isWellFormed() || runtime.cwd.includes('\0')) throw new Error('Select an exact empty runtime workspace');
    if (readdirSync(runtime.cwd).length !== 0) throw new Error('Codex executor workspace must be empty');
    const owned = launchGuardedCodexProcess({ ...runtime, deadlineMs: deadline - Date.now() });
    return new CodexChat({ connection: owned.connection, workspace: runtime.cwd, runtimeVersion: '0.163.0' });
  }
  return serveExecutorWorker({ input: options.input, output: options.output,
    binding: { provider: 'codex', runtimeVersion: '0.163.0', hostId: options.hostId },
    createRuntime: ({ dispatch }) => create(dispatch.deadline),
    createDiscovery: ({ deadline }) => create(deadline),
    createSignIn: ({ deadline }) => create(deadline),
  });
}

if (import.meta.main) {
  try {
    compiledWorker();
    const { values } = parseArgs({ options: { executable: { type: 'string' }, guardian: { type: 'string' }, directory: { type: 'string' }, host: { type: 'string' } }, strict: true, allowPositionals: false });
    if (!values.executable || !values.guardian || !values.directory || !values.host) throw new Error('Missing private runtime configuration');
    await serveCodexExecutor({ input: process.stdin, output: process.stdout, hostId: values.host,
      runtime: privateCodexLaunchPlan({ executable: values.executable, guardian: values.guardian, directory: values.directory }) });
  } catch {
    process.stderr.write('Fort executor could not start or finish safely.\n');
    process.exitCode = 1;
  }
}
