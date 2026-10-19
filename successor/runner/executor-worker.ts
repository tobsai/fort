import type { Readable, Writable } from 'node:stream';
import type { ChatRuntime } from './chat-runtime.ts';
import { DISPATCH_FRAME_BYTES, MODEL_FRAME_BYTES, parseExecutorModelRequest, parseExecutorModelCatalog, parseExecutorRequest, parseExecutorResult, validateExecutorBinding, validateExecutorCancel, validateExecutorModelCancel, type ExecutorBinding, type ExecutorRequest, type ExecutorModelRequest } from './executor-wire.ts';
import { ACCOUNT_FRAME_BYTES, parseExecutorAccountRequest, parseAccountChallenge, parseAccountSignInResult, validateExecutorAccountCancel, type ExecutorAccountRequest } from './executor-account-wire.ts';

type WorkerOptions = {
  input: Readable; output: Writable; binding: ExecutorBinding;
  createRuntime(request: ExecutorRequest, signal: AbortSignal): ChatRuntime;
  createDiscovery?(request: ExecutorModelRequest, signal: AbortSignal): ChatRuntime;
  createSignIn?(request: ExecutorAccountRequest, signal: AbortSignal): ChatRuntime;
};
type Operation = {
  deadline: number; requestBytes: number; canceledIsFailure: boolean;
  create(): ChatRuntime;
  execute(runtime: ChatRuntime): Promise<unknown>;
  terminal(value: unknown): object;
  cancel(value: unknown): void;
};

export function serveExecutorWorker({ input, output, binding, createRuntime, createDiscovery, createSignIn }: WorkerOptions): Promise<void> {
  validateExecutorBinding(binding);
  const selected = { ...binding }, done = Promise.withResolvers<void>(), failed = Promise.withResolvers<never>();
  void failed.promise.catch(() => {});
  const controller = new AbortController();
  let pending = Buffer.alloc(0), total = 0, frames = 0, request: Operation | undefined;
  let stopped = false, invalid = false;
  let timer = setTimeout(() => fail(), 5000);
  const fail = () => {
    if (stopped || invalid) return;
    invalid = true; controller.abort();
    failed.reject(new Error('Executor channel ended without confirmed completion'));
    if (!request) finish(new Error('Executor dispatch was not confirmed'));
  };
  const outputClosed = () => { if (!output.writableFinished) fail(); };
  function finish(error?: unknown) {
    if (stopped) return;
    stopped = true; clearTimeout(timer);
    input.off('data', receive); input.off('end', fail); input.off('close', fail); input.off('error', fail);
    output.off('error', fail); output.off('close', outputClosed);
    input.destroy();
    if (error !== undefined) { output.destroy(); done.reject(new Error('Executor completion was not confirmed')); }
    else done.resolve();
  }
  function operation(value: unknown): Operation {
    if (value && typeof value === 'object' && 'kind' in value && value.kind === 'accountSignIn') {
      const accepted = parseExecutorAccountRequest(value, selected);
      if (!createSignIn) throw new Error('Account sign-in is not configured');
      return { deadline: accepted.deadline, requestBytes: ACCOUNT_FRAME_BYTES, canceledIsFailure: false,
        create: () => createSignIn(accepted, controller.signal),
        execute: async runtime => {
          if (!runtime.signIn || Date.now() >= accepted.workDeadline) throw new Error('Account sign-in is unavailable');
          let offered = false;
          const durationMs = accepted.workDeadline - Date.now();
          const workTimer = setTimeout(() => controller.abort(), Math.max(1, durationMs));
          try {
            return await runtime.signIn({ durationMs, onChallenge: value => {
              const challenge = parseAccountChallenge(value);
              if (offered || stopped || invalid || controller.signal.aborted || Date.now() >= accepted.workDeadline) throw new Error('Account challenge is no longer available');
              offered = true;
              output.write(JSON.stringify({ protocolVersion: 1, kind: 'accountChallenge', sequence: 0,
                requestId: accepted.requestId, requestDigest: accepted.requestDigest, challenge }) + '\n');
            } }, controller.signal);
          } finally { clearTimeout(workTimer); }
        },
        terminal: result => ({ protocolVersion: 1, kind: 'accountResult', sequence: 1,
          requestId: accepted.requestId, requestDigest: accepted.requestDigest, result: parseAccountSignInResult(result) }),
        cancel: value => validateExecutorAccountCancel(value, accepted),
      };
    }
    if (value && typeof value === 'object' && 'kind' in value && value.kind === 'models') {
      const accepted = parseExecutorModelRequest(value, selected);
      if (!createDiscovery) throw new Error('Model discovery is not configured');
      return { deadline: accepted.deadline, requestBytes: MODEL_FRAME_BYTES, canceledIsFailure: true,
        create: () => createDiscovery(accepted, controller.signal),
        execute: async runtime => {
          if (!runtime.models) throw new Error('Model discovery is not supported');
          return runtime.models(controller.signal, accepted.account);
        },
        terminal: result => ({ protocolVersion: 1, kind: 'catalog', sequence: 1,
          requestId: accepted.requestId, requestDigest: accepted.requestDigest, result: parseExecutorModelCatalog(result, selected.provider) }),
        cancel: value => validateExecutorModelCancel(value, accepted),
      };
    }
    const accepted = parseExecutorRequest(value, selected);
    return { deadline: accepted.dispatch.deadline, requestBytes: DISPATCH_FRAME_BYTES, canceledIsFailure: false,
      create: () => createRuntime(accepted, controller.signal),
      execute: runtime => runtime.execute(accepted.input, controller.signal, accepted.dispatch),
      terminal: result => ({ protocolVersion: 1, kind: 'terminal', sequence: 1, dispatchDigest: accepted.dispatch.dispatchDigest,
        attemptId: accepted.dispatch.attempt.attemptId, generation: accepted.dispatch.attempt.generation, result: parseExecutorResult(result) }),
      cancel: value => validateExecutorCancel(value, accepted.dispatch),
    };
  }
  async function run(accepted: Operation) {
    let runtime: ChatRuntime | undefined;
    try {
      clearTimeout(timer);
      timer = setTimeout(fail, Math.max(1, accepted.deadline - Date.now()));
      let frame;
      try {
        runtime = accepted.create();
        frame = accepted.terminal(await Promise.race([accepted.execute(runtime), failed.promise]));
      } finally {
        if (runtime) {
          let timeout: ReturnType<typeof setTimeout> | undefined;
          try {
            await Promise.race([runtime.dispose(), new Promise<never>((_, reject) => { timeout = setTimeout(() => reject(new Error('Runtime cleanup exceeded its bound')), 1500); })]);
          } finally { clearTimeout(timeout); }
        }
      }
      if (invalid || pending.length !== 0 || Date.now() >= accepted.deadline || (accepted.canceledIsFailure && controller.signal.aborted)) throw new Error('Executor deadline or channel failed');
      await Promise.race([new Promise<void>((resolve, reject) => output.end(JSON.stringify(frame) + '\n', (error?: Error | null) => error ? reject(error) : resolve())), failed.promise]);
      finish();
    } catch (error) { finish(error); }
  }
  function receive(chunk: Buffer) {
    if (stopped || invalid) return;
    total += chunk.byteLength;
    if (total > DISPATCH_FRAME_BYTES + 2048) { fail(); return; }
    pending = Buffer.concat([pending, chunk]);
    let newline: number;
    while (!invalid && (newline = pending.indexOf(10)) >= 0) {
      const line = pending.subarray(0, newline); pending = pending.subarray(newline + 1);
      try {
        if (++frames > 2 || line.length > (request ? 2048 : DISPATCH_FRAME_BYTES)) throw new Error('Executor frame exceeds its bound');
        const value: unknown = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(line));
        if (!request) {
          const accepted = operation(value);
          if (line.length > accepted.requestBytes || total > accepted.requestBytes + 2048) throw new Error('Executor request exceeds its bound');
          request = accepted; void run(request);
        }
        else { request.cancel(value); controller.abort(); }
      } catch { fail(); return; }
    }
    if (pending.length > (request ? 2048 : DISPATCH_FRAME_BYTES) || total > (request?.requestBytes ?? DISPATCH_FRAME_BYTES) + 2048) fail();
  }
  input.on('data', receive); input.once('end', fail); input.once('close', fail); input.once('error', fail);
  output.once('error', fail); output.once('close', outputClosed);
  if (input.destroyed || output.destroyed) fail();
  return done.promise;
}
