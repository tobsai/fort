import { isAbsolute, normalize } from 'node:path';
import { parseArgs } from 'node:util';
import type { Readable, Writable } from 'node:stream';
import { OpenAIPlanChat, OPENAI_PLAN_RUNTIME } from './openai-plan-chat.ts';
import { serveExecutorWorker } from './executor-worker.ts';
import { ExecutorProcess } from './executor-process.ts';
export { OPENAI_PLAN_RUNTIME } from './openai-plan-chat.ts';

// The ChatGPT plan route as an owned executor child: one fresh runtime per turn, model list or sign-in,
// reading the credential from the exact private state directory. No Codex runtime is involved.
type Fetch = (url: string, init?: RequestInit) => Promise<Response>;
type PlanExecutorOptions = { input: Readable; output: Writable; hostId: string; stateDirectory: string; fetch?: Fetch };
type PlanProcessOptions = Omit<ConstructorParameters<typeof ExecutorProcess>[0], 'cooperativeCancellationMs' | 'discovery' | 'signIn'> & { hostId: string };

const binding = (hostId: string) => ({ provider: OPENAI_PLAN_RUNTIME.provider, runtimeVersion: OPENAI_PLAN_RUNTIME.runtimeVersion, hostId });

export function createOpenAIPlanExecutorProcess({ hostId, ...process }: PlanProcessOptions): ExecutorProcess {
  return new ExecutorProcess({ ...process, cooperativeCancellationMs: 3500, discovery: binding(hostId), signIn: binding(hostId) });
}

export function serveOpenAIPlanExecutor(options: PlanExecutorOptions): Promise<void> {
  const directory = options.stateDirectory;
  if (!isAbsolute(directory) || directory === '/' || normalize(directory) !== directory || !directory.isWellFormed() || directory.includes('\0')) throw new Error('Select the exact private state directory');
  const create = () => new OpenAIPlanChat({ stateDirectory: directory, ...(options.fetch ? { fetch: options.fetch } : {}) });
  return serveExecutorWorker({ input: options.input, output: options.output, binding: binding(options.hostId),
    createRuntime: create, createDiscovery: create, createSignIn: create });
}

if (import.meta.main) {
  try {
    const { values } = parseArgs({ options: { state: { type: 'string' }, host: { type: 'string' } }, strict: true, allowPositionals: false });
    if (!values.state || !values.host) throw new Error('Missing private runtime configuration');
    await serveOpenAIPlanExecutor({ input: process.stdin, output: process.stdout, hostId: values.host, stateDirectory: values.state });
  } catch {
    process.stderr.write('Fort executor could not start or finish safely.\n');
    process.exitCode = 1;
  }
}
