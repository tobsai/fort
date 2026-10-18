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
type CodexProcessOptions = Omit<ConstructorParameters<typeof ExecutorProcess>[0], 'lifecycle' | 'cooperativeCancellationMs'> & { guardian: string };

export function createCodexExecutorProcess({ guardian, ...process }: CodexProcessOptions): ExecutorProcess {
  return new ExecutorProcess({ ...process, lifecycle: { kind: 'macParentExit', guardian }, cooperativeCancellationMs: 3500 });
}

export function serveCodexExecutor(options: CodexExecutorOptions): Promise<void> {
  const runtime = { ...options.runtime, args: [...options.runtime.args], env: { ...options.runtime.env } };
  return serveExecutorWorker({ input: options.input, output: options.output,
    binding: { provider: 'codex', runtimeVersion: '0.157.1', hostId: options.hostId },
    createRuntime({ dispatch }) {
      if (!isAbsolute(runtime.cwd) || runtime.cwd === '/' || normalize(runtime.cwd) !== runtime.cwd || !runtime.cwd.isWellFormed() || runtime.cwd.includes('\0')) throw new Error('Select an exact empty runtime workspace');
      if (readdirSync(runtime.cwd).length !== 0) throw new Error('Codex executor workspace must be empty');
      const owned = launchGuardedCodexProcess({ ...runtime, deadlineMs: dispatch.deadline - Date.now() });
      return new CodexChat({ connection: owned.connection, workspace: runtime.cwd, runtimeVersion: '0.157.1' });
    },
  });
}
