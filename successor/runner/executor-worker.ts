import type { Readable, Writable } from 'node:stream';
import type { ChatRuntime } from './chat-runtime.ts';
import { DISPATCH_FRAME_BYTES, parseExecutorRequest, parseExecutorResult, validateExecutorBinding, validateExecutorCancel, type ExecutorBinding, type ExecutorRequest } from './executor-wire.ts';

type WorkerOptions = {
  input: Readable; output: Writable; binding: ExecutorBinding;
  createRuntime(request: ExecutorRequest, signal: AbortSignal): ChatRuntime;
};

export function serveExecutorWorker({ input, output, binding, createRuntime }: WorkerOptions): Promise<void> {
  validateExecutorBinding(binding);
  const selected = { ...binding }, done = Promise.withResolvers<void>(), failed = Promise.withResolvers<never>();
  void failed.promise.catch(() => {});
  const controller = new AbortController();
  let pending = Buffer.alloc(0), total = 0, frames = 0, request: ExecutorRequest | undefined;
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
  async function run(accepted: ExecutorRequest) {
    let runtime: ChatRuntime | undefined;
    try {
      clearTimeout(timer);
      timer = setTimeout(fail, Math.max(1, accepted.dispatch.deadline - Date.now()));
      let result;
      try {
        runtime = createRuntime(accepted, controller.signal);
        result = parseExecutorResult(await Promise.race([runtime.execute(accepted.input, controller.signal, accepted.dispatch), failed.promise]));
      } finally {
        if (runtime) {
          let timeout: ReturnType<typeof setTimeout> | undefined;
          try {
            await Promise.race([runtime.dispose(), new Promise<never>((_, reject) => { timeout = setTimeout(() => reject(new Error('Runtime cleanup exceeded its bound')), 1500); })]);
          } finally { clearTimeout(timeout); }
        }
      }
      if (invalid || pending.length !== 0 || Date.now() >= accepted.dispatch.deadline) throw new Error('Executor deadline or channel failed');
      const frame = { protocolVersion: 1, kind: 'terminal', sequence: 1, dispatchDigest: accepted.dispatch.dispatchDigest,
        attemptId: accepted.dispatch.attempt.attemptId, generation: accepted.dispatch.attempt.generation, result };
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
        if (!request) { request = parseExecutorRequest(value, selected); void run(request); }
        else { validateExecutorCancel(value, request.dispatch); controller.abort(); }
      } catch { fail(); return; }
    }
    if (pending.length > (request ? 2048 : DISPATCH_FRAME_BYTES)) fail();
  }
  input.on('data', receive); input.once('end', fail); input.once('close', fail); input.once('error', fail);
  output.once('error', fail); output.once('close', outputClosed);
  if (input.destroyed || output.destroyed) fail();
  return done.promise;
}
