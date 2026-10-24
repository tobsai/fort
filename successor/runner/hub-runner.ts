import { PassThrough } from 'node:stream';
import type { ChatRuntime } from './chat-runtime.ts';
import { serveExecutorWorker } from './executor-worker.ts';
import { DISPATCH_FRAME_BYTES, type ExecutorBinding } from './executor-wire.ts';

// Runner side of the runner channel (Spec 063). Holds one outbound connection to the hub and runs
// each leased dispatch through the same executor worker that guards local turns. Nothing listens
// on this machine. When the connection ends, every running lease is stopped; nothing is replayed.
type Fetch = (url: string, init?: RequestInit) => Promise<Response>;
type RunnerOptions = { url: string; token: string; binding: ExecutorBinding; createRuntime(): ChatRuntime; signal: AbortSignal; fetch?: Fetch;
  // The hub pings every 15 seconds; a connection silent for longer than this is treated as dead.
  idleMs?: number };
export class HubRefused extends Error { constructor() { super('The hub refused this runner'); } }

export async function connectToHub(options: RunnerOptions): Promise<void> {
  const hub = new URL(options.url);
  if (hub.protocol !== 'https:' && !(hub.protocol === 'http:' && ['127.0.0.1', '[::1]', 'localhost'].includes(hub.hostname))) throw new Error('Connect to the hub over HTTPS');
  const fetcher = options.fetch ?? fetch, authorization = `Bearer ${options.token}`;
  const silent = new AbortController(), signal = AbortSignal.any([options.signal, silent.signal]), idleMs = options.idleMs ?? 45_000;
  let idle = setTimeout(() => silent.abort(), idleMs);
  const heard = () => { clearTimeout(idle); idle = setTimeout(() => silent.abort(), idleMs); };
  try {
    const response = await fetcher(options.url + '/v1/runner/connect', { method: 'POST', headers: { authorization }, signal });
    if (response.status === 401 || response.status === 403) { await response.body?.cancel(); throw new HubRefused(); }
    if (response.status !== 200 || !response.body) { await response.body?.cancel(); throw new Error('The hub did not open the runner channel'); }
    heard();
    const leases = new Map<string, PassThrough>();
    const post = (row: object) => fetcher(options.url + '/v1/runner/frames', { method: 'POST', signal: options.signal,
      headers: { authorization, 'content-type': 'application/x-ndjson' }, body: JSON.stringify(row) + '\n' })
      .then(reply => reply.body?.cancel()).catch(() => {});

    function lease(leaseId: string, request: unknown) {
      if (leases.has(leaseId)) return;
      const input = new PassThrough(), output = new PassThrough();
      leases.set(leaseId, input);
      // Frames for one lease are posted in order, and the final "ended" row only after all of them.
      let pending = '', sent: Promise<unknown> = Promise.resolve();
      const send = (row: object) => { sent = sent.then(() => post(row)); };
      output.setEncoding('utf8');
      output.on('data', (text: string) => {
        pending += text;
        for (let newline = pending.indexOf('\n'); newline >= 0; newline = pending.indexOf('\n')) {
          const line = pending.slice(0, newline); pending = pending.slice(newline + 1);
          try { send({ leaseId, frame: JSON.parse(line) }); } catch {}
        }
      });
      void serveExecutorWorker({ input, output, binding: options.binding, createRuntime: () => options.createRuntime() })
        .catch(() => {}).finally(() => { leases.delete(leaseId); send({ leaseId, ended: true }); });
      input.write(JSON.stringify(request) + '\n');
    }

    function receive(line: string) {
      const frame = JSON.parse(line) as { kind?: unknown; leaseId?: unknown; request?: unknown; cancel?: unknown };
      if (typeof frame.leaseId !== 'string') return;
      if (frame.kind === 'lease') lease(frame.leaseId, frame.request);
      else if (frame.kind === 'cancel') leases.get(frame.leaseId)?.write(JSON.stringify(frame.cancel) + '\n');
    }

    let pending = '';
    try {
      for await (const chunk of response.body.pipeThrough(new TextDecoderStream())) {
        heard(); pending += chunk;
        if (pending.length > DISPATCH_FRAME_BYTES + 4096) throw new Error('Hub frame exceeds its bound');
        for (let newline = pending.indexOf('\n'); newline >= 0; newline = pending.indexOf('\n')) {
          const line = pending.slice(0, newline); pending = pending.slice(newline + 1);
          if (line) receive(line);
        }
      }
    } catch (error) { if (!options.signal.aborted) throw silent.signal.aborted ? new Error('The hub connection went silent') : error; }
    finally { for (const input of leases.values()) input.destroy(); }
  } finally { clearTimeout(idle); }
}

export type RunnerStatus = { state: 'connecting' | 'connected' | 'waiting' | 'refused' | 'stopped'; retryMs?: number };

// Keeps one runner connected until the signal ends. Network failures retry with capped, jittered
// backoff; a refused token stops for good so a revoked Mac does not keep knocking.
export async function keepConnected(options: RunnerOptions & { minRetryMs?: number; maxRetryMs?: number; onStatus?(status: RunnerStatus): void }): Promise<RunnerStatus> {
  const minRetryMs = options.minRetryMs ?? 1000, maxRetryMs = options.maxRetryMs ?? 60_000;
  let retryMs = minRetryMs;
  const report = (status: RunnerStatus) => { options.onStatus?.(status); return status; };
  while (!options.signal.aborted) {
    report({ state: 'connecting' });
    const opened = Date.now();
    try {
      await connectToHub({ ...options, fetch: async (url, init) => {
        const reply = await (options.fetch ?? fetch)(url, init);
        if (url.endsWith('/v1/runner/connect') && reply.status === 200) report({ state: 'connected' });
        return reply;
      } });
    } catch (error) { if (error instanceof HubRefused) return report({ state: 'refused' }); }
    if (options.signal.aborted) break;
    if (Date.now() - opened > maxRetryMs) retryMs = minRetryMs;
    const wait = Math.round(retryMs * (0.5 + Math.random() / 2));
    report({ state: 'waiting', retryMs: wait });
    await new Promise<void>(resolve => {
      const timer = setTimeout(done, wait);
      function done() { clearTimeout(timer); options.signal.removeEventListener('abort', done); resolve(); }
      options.signal.addEventListener('abort', done, { once: true });
    });
    retryMs = Math.min(maxRetryMs, retryMs * 2);
  }
  return report({ state: 'stopped' });
}
