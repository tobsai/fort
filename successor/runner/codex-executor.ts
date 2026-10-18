import { readdirSync } from 'node:fs';
import { isAbsolute, normalize } from 'node:path';
import type { Readable, Writable } from 'node:stream';
import { CodexChat } from './codex-chat.ts';
import { launchGuardedCodexProcess } from './guarded-codex-process.ts';
import { serveExecutorWorker } from './executor-worker.ts';
import { ExecutorProcess } from './executor-process.ts';

type CodexExecutorOptions = {
  input: Readable; output: Writable; hostId: string;
  runtime: Omit<Parameters<typeof launchGuardedCodexProcess>[0], 'deadlineMs'>;
};
type CodexProcessOptions = Omit<ConstructorParameters<typeof ExecutorProcess>[0], 'lifecycle' | 'cooperativeCancellationMs' | 'discovery'> & { guardian: string; hostId: string };

export function createCodexExecutorProcess({ guardian, hostId, ...process }: CodexProcessOptions): ExecutorProcess {
  return new ExecutorProcess({ ...process, lifecycle: { kind: 'macParentExit', guardian }, cooperativeCancellationMs: 3500,
    discovery: { provider: 'codex', runtimeVersion: '0.157.1', hostId } });
}

export function serveCodexExecutor(options: CodexExecutorOptions): Promise<void> {
  const runtime = { ...options.runtime, args: [...options.runtime.args], env: { ...options.runtime.env } };
  function create(deadline: number): CodexChat {
    if (!isAbsolute(runtime.cwd) || runtime.cwd === '/' || normalize(runtime.cwd) !== runtime.cwd || !runtime.cwd.isWellFormed() || runtime.cwd.includes('\0')) throw new Error('Select an exact empty runtime workspace');
    if (readdirSync(runtime.cwd).length !== 0) throw new Error('Codex executor workspace must be empty');
    const owned = launchGuardedCodexProcess({ ...runtime, deadlineMs: deadline - Date.now() });
    return new CodexChat({ connection: owned.connection, workspace: runtime.cwd, runtimeVersion: '0.157.1' });
  }
  return serveExecutorWorker({ input: options.input, output: options.output,
    binding: { provider: 'codex', runtimeVersion: '0.157.1', hostId: options.hostId },
    createRuntime: ({ dispatch }) => create(dispatch.deadline),
    createDiscovery: ({ deadline }) => create(deadline),
  });
}
