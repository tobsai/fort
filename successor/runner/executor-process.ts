import { spawn } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { isAbsolute } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { EXECUTION_LIMITS, TEXT_POLICY, executionDigest, sealDispatch, type ExecutionDispatch } from '../contracts/executor.ts';
import type { ModelCatalog, RuntimeReadiness } from '../contracts/v1.ts';
import type { ChatInput, ChatResult, ChatRuntime } from './chat-runtime.ts';
import { MODEL_EVIDENCE_BYTES, parseExecutorModelFrame, validateExecutorBinding, type ExecutorBinding, type ExecutorModelRequest } from './executor-wire.ts';

type ProcessOptions = {
  executable: string; args: string[]; cwd: string; env: Record<string, string>;
  lifecycle: { kind: 'processGroup' } | { kind: 'macParentExit'; guardian: string };
  cooperativeCancellationMs?: number;
  discovery?: ExecutorBinding;
};
type ExitReport = { pid: number | null; groupAbsent: boolean; forced: boolean; code: number | null; signal: NodeJS.Signals | null };
type Exchange<T> = {
  request: object; cancel: object; deadline: number; evidenceBytes: number;
  parse(value: unknown): T;
  canComplete(): boolean;
  failure(spawnFailed: boolean, absent: boolean): T;
};

function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Invalid executor object');
  return value as Record<string, unknown>;
}

function terminal(value: unknown, dispatch: ExecutionDispatch): ChatResult {
  if (Date.now() >= dispatch.deadline) throw new Error('Expired evidence');
  const frame = object(value);
  if (frame.protocolVersion !== 1 || frame.kind !== 'terminal' || frame.sequence !== 1 || frame.dispatchDigest !== dispatch.dispatchDigest || frame.attemptId !== dispatch.attempt.attemptId || frame.generation !== dispatch.attempt.generation) throw new Error('Uncorrelated evidence');
  const result = object(frame.result);
  if (result.kind === 'completed') {
    if (typeof result.text !== 'string' || !result.text || !result.text.isWellFormed() || Buffer.byteLength(result.text) > dispatch.limits.replyBytes || result.effectiveModel !== null) throw new Error('Invalid reply');
    return { kind: 'completed', text: result.text, effectiveModel: null };
  }
  if (result.kind !== 'notStarted' && result.kind !== 'failed' && result.kind !== 'outcomeUnknown' && result.kind !== 'canceled') throw new Error('Invalid terminal');
  if (typeof result.message !== 'string' || !result.message.trim() || !result.message.isWellFormed() || Buffer.byteLength(result.message) > 16_384) throw new Error('Invalid result detail');
  return { kind: result.kind, message: result.message };
}

function validDispatch(input: ChatInput, dispatch: ExecutionDispatch): boolean {
  const { dispatchDigest, ...body } = dispatch;
  return dispatch.protocolVersion === 1 && sealDispatch(body).dispatchDigest === dispatchDigest
    && executionDigest(dispatch.policy) === executionDigest(TEXT_POLICY) && dispatch.policyDigest === executionDigest(TEXT_POLICY)
    && executionDigest(dispatch.limits) === executionDigest(EXECUTION_LIMITS)
    && executionDigest(input) === dispatch.inputDigest && executionDigest(input.configuration) === dispatch.configurationDigest
    && executionDigest(input.history) === dispatch.historyDigest && executionDigest(input.attempt) === executionDigest(dispatch.attempt)
    && input.attempt.runId === input.runId && input.attempt.bindingRevisionId === input.configuration.bindingRevisionId
    && input.attempt.selectedModel === input.configuration.selectedModel && dispatch.binding.bindingRevisionId === input.attempt.bindingRevisionId
    && dispatch.binding.runtimeVersion === input.attempt.runtimeVersion && dispatch.binding.selectedModel === input.attempt.selectedModel
    && dispatch.binding.enabled && Number.isSafeInteger(dispatch.deadline) && dispatch.deadline > Date.now()
    && dispatch.deadline <= Date.now() + 120_000 && Buffer.byteLength(JSON.stringify(input)) <= dispatch.limits.inputBytes;
}

export class ExecutorProcess implements ChatRuntime {
  private readonly options: ProcessOptions;
  private active: { cancel(): void; done: Promise<unknown> } | undefined;
  private stopped = false;
  private blocked = false;
  private report: ExitReport | undefined;
  readonly models?: (signal: AbortSignal) => Promise<ModelCatalog>;

  constructor(options: ProcessOptions) {
    if (!isAbsolute(options.executable) || !isAbsolute(options.cwd)) throw new Error('Select exact executor and working-directory paths');
    if (!options.lifecycle || !['processGroup', 'macParentExit'].includes(options.lifecycle.kind)) throw new Error('Select an explicit executor lifecycle');
    if (options.lifecycle.kind === 'macParentExit' && (process.platform !== 'darwin' || !isAbsolute(options.lifecycle.guardian) || options.lifecycle.guardian.includes('\0'))) throw new Error('Select an exact Mac guardian on macOS');
    const allowance = options.cooperativeCancellationMs ?? 250;
    if (!Number.isInteger(allowance) || allowance < 250 || allowance > 3500) throw new Error('Select a bounded cooperative cancellation allowance');
    this.options = { ...options, args: [...options.args], env: { ...options.env }, lifecycle: { ...options.lifecycle } };
    if (options.discovery) {
      validateExecutorBinding(options.discovery);
      const binding = Object.freeze({ ...options.discovery });
      this.models = signal => this.discover(binding, signal);
    }
  }

  get lastExit(): Readonly<ExitReport> | undefined { return this.report; }

  readiness(): RuntimeReadiness {
    return { state: this.stopped ? 'stopped' : this.blocked ? 'unavailable' : this.active ? 'busy' : 'ready', canRestart: this.stopped && !this.blocked && !this.active,
      detail: this.blocked ? 'Executor cleanup is unconfirmed. New dispatch is blocked.' : this.stopped ? 'Executor stopped.' : this.active ? 'An executor operation is active.' : 'Executor available.' };
  }

  async dispose(): Promise<void> {
    this.stopped = true;
    this.active?.cancel();
    await this.active?.done;
    if (this.blocked) throw new Error('Executor cleanup was not confirmed');
  }

  async restart(signal: AbortSignal): Promise<void> {
    signal.throwIfAborted();
    if (!this.stopped || this.blocked || this.active) throw new Error('Executor resume requires confirmed shutdown');
    this.stopped = false;
  }

  async execute(input: ChatInput, signal: AbortSignal, dispatch: ExecutionDispatch): Promise<ChatResult> {
    if (this.stopped || this.blocked || this.active || signal.aborted) return { kind: 'notStarted', message: 'Executor is stopped, busy or canceled. Nothing was sent.' };
    if (!validDispatch(input, dispatch)) return { kind: 'notStarted', message: 'Execution scope, input, policy or deadline was not confirmed. Nothing was sent.' };
    return this.exchange(signal, {
      request: { protocolVersion: 1, kind: 'dispatch', dispatch, input },
      cancel: { protocolVersion: 1, kind: 'cancel', dispatchDigest: dispatch.dispatchDigest, attemptId: dispatch.attempt.attemptId, generation: dispatch.attempt.generation },
      deadline: dispatch.deadline, evidenceBytes: dispatch.limits.evidenceBytes, parse: value => terminal(value, dispatch),
      canComplete: () => Date.now() < dispatch.deadline,
      failure: (spawnFailed, absent) => spawnFailed ? { kind: 'notStarted', message: 'The exact executor could not start. Nothing was sent.' }
        : { kind: 'outcomeUnknown', message: absent ? 'Executor ended without valid terminal evidence and clean exit. Earlier effects remain unknown.' : 'Executor cleanup is unconfirmed. Earlier effects remain unknown.' },
    });
  }

  private async discover(binding: ExecutorBinding, signal: AbortSignal): Promise<ModelCatalog> {
    if (this.stopped || this.blocked || this.active || signal.aborted) return { provider: binding.provider, state: 'unavailable', models: [], detail: 'Model discovery is stopped, busy, blocked or canceled. Your selection has not changed.' };
    const body = { protocolVersion: 1, kind: 'models', requestId: randomUUID(), binding, deadline: Date.now() + 5000 } as const;
    const request: ExecutorModelRequest = { ...body, requestDigest: executionDigest(body) };
    return this.exchange(signal, { request, deadline: request.deadline, evidenceBytes: MODEL_EVIDENCE_BYTES,
      cancel: { protocolVersion: 1, kind: 'cancelModels', requestId: request.requestId, requestDigest: request.requestDigest },
      parse(value) { signal.throwIfAborted(); return parseExecutorModelFrame(value, request); },
      canComplete: () => !this.stopped && !signal.aborted && Date.now() < request.deadline,
      failure: (spawnFailed, absent) => ({ provider: binding.provider, state: !absent ? 'error' : spawnFailed || signal.aborted || this.stopped ? 'unavailable' : 'error', models: [],
        detail: absent ? 'Model discovery did not finish with valid evidence and clean exit. Your selection has not changed.' : 'Executor cleanup is unconfirmed. Model discovery and new dispatch are blocked.' }),
    });
  }

  private async exchange<T>(signal: AbortSignal, protocol: Exchange<T>): Promise<T> {
    const { executable, args, lifecycle } = this.options;
    const child = spawn(lifecycle.kind === 'macParentExit' ? lifecycle.guardian : executable,
      lifecycle.kind === 'macParentExit' ? [String(process.pid), executable, ...args] : args,
      { cwd: this.options.cwd, env: this.options.env, detached: true, stdio: ['pipe', 'pipe', 'pipe'] });
    let bytes = 0, frames = 0, pending = Buffer.alloc(0), result: T | undefined;
    let invalid = false, forced = false, closed = false, finishing = false, groupGone = false, spawnFailed = false;
    let cancelTimer: ReturnType<typeof setTimeout> | undefined, killTimer: ReturnType<typeof setTimeout> | undefined, finishTimer: ReturnType<typeof setTimeout> | undefined;
    const outcome = Promise.withResolvers<T>();
    const groupPresent = () => {
      if (!child.pid || groupGone) return false;
      try { process.kill(-child.pid, 0); return true; }
      catch (error) { if (error instanceof Error && 'code' in error && error.code === 'ESRCH') { groupGone = true; return false; } return true; }
    };
    const signalGroup = (value: NodeJS.Signals) => {
      if (!child.pid || !groupPresent()) return;
      try { process.kill(-child.pid, value); }
      catch (error) { if (error instanceof Error && 'code' in error && error.code === 'ESRCH') groupGone = true; }
    };
    const waitAbsent = async (milliseconds: number) => {
      const until = performance.now() + milliseconds;
      while ((!closed || groupPresent()) && performance.now() < until) await delay(10);
      return closed && !groupPresent();
    };
    const finish = async () => {
      if (finishing) return;
      finishing = true;
      clearTimeout(deadline); clearTimeout(cancelTimer); clearTimeout(killTimer); clearTimeout(finishTimer);
      signal.removeEventListener('abort', cancel);
      if (child.pid && (!closed || groupPresent())) {
        forced = true; signalGroup('SIGTERM');
        if (!await waitAbsent(250)) { signalGroup('SIGKILL'); await waitAbsent(750); }
      }
      const absent = (!child.pid || closed) && !groupPresent();
      this.blocked = !absent;
      this.report = { pid: child.pid ?? null, groupAbsent: absent, forced, code: child.exitCode, signal: child.signalCode };
      child.stdin.destroy(); child.stdout.destroy(); child.stderr.destroy();
      outcome.resolve(!spawnFailed && !invalid && !forced && absent && child.exitCode === 0 && pending.length === 0 && result !== undefined && protocol.canComplete() ? result : protocol.failure(spawnFailed, absent));
    };
    const forceStop = () => {
      if (finishing || forced) return;
      forced = true; signalGroup('SIGTERM');
      killTimer = setTimeout(() => signalGroup('SIGKILL'), 250);
      finishTimer = setTimeout(() => { void finish(); }, 1_000);
    };
    const fail = () => { invalid = true; forceStop(); };
    const cancel = () => {
      if (finishing || cancelTimer || forced) return;
      child.stdin.write(JSON.stringify(protocol.cancel) + '\n', error => { if (error) fail(); });
      const allowance = Math.min(this.options.cooperativeCancellationMs ?? 250, Math.max(0, protocol.deadline - Date.now()) + 250);
      cancelTimer = setTimeout(forceStop, allowance);
    };
    const deadline = setTimeout(() => { invalid = true; cancel(); }, Math.max(1, protocol.deadline - Date.now()));
    child.on('error', () => { spawnFailed = !child.pid; fail(); });
    child.once('close', () => { closed = true; void finish(); });
    child.stdin.on('error', fail); child.stdout.on('error', fail); child.stderr.on('error', fail);
    child.stderr.on('data', (chunk: Buffer) => { bytes += chunk.length; if (bytes > protocol.evidenceBytes) fail(); });
    child.stdout.on('data', (chunk: Buffer) => {
      if (finishing || invalid) return;
      bytes += chunk.length;
      if (bytes > protocol.evidenceBytes) { fail(); return; }
      pending = Buffer.concat([pending, chunk]);
      let newline: number;
      while ((newline = pending.indexOf(10)) >= 0) {
        const line = pending.subarray(0, newline); pending = pending.subarray(newline + 1);
        try {
          if (++frames > 8) throw new Error('Too many evidence frames');
          const next = protocol.parse(JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(line)));
          if (result && executionDigest(next) !== executionDigest(result)) throw new Error('Conflicting terminal');
          result = next;
        } catch { fail(); return; }
      }
    });
    const operation = { cancel, done: outcome.promise };
    this.active = operation;
    signal.addEventListener('abort', cancel, { once: true });
    child.stdin.write(JSON.stringify(protocol.request) + '\n', error => { if (error) fail(); });
    if (signal.aborted) cancel();
    try { return await outcome.promise; }
    finally { if (this.active === operation) this.active = undefined; }
  }
}
