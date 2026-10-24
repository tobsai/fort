import { PassThrough } from 'node:stream';
import type { ChatRuntime } from './chat-runtime.ts';
import { serveExecutorWorker } from './executor-worker.ts';
import { DISPATCH_FRAME_BYTES, type ExecutorBinding } from './executor-wire.ts';

// Runner side of the runner channel (Spec 063). Holds one outbound connection to the hub and runs
// each leased dispatch through the same executor worker that guards local turns. Nothing listens
// on this machine. When the connection ends, every running lease is stopped; nothing is replayed.
type Fetch = (url: string, init?: RequestInit) => Promise<Response>;
type RunnerOptions = { url: string; token: string; binding: ExecutorBinding; createRuntime(): ChatRuntime; signal: AbortSignal; fetch?: Fetch };

export async function connectToHub(options: RunnerOptions): Promise<void> {
  const hub = new URL(options.url);
  if (hub.protocol !== 'https:' && !(hub.protocol === 'http:' && ['127.0.0.1', '[::1]', 'localhost'].includes(hub.hostname))) throw new Error('Connect to the hub over HTTPS');
  const fetcher = options.fetch ?? fetch, authorization = `Bearer ${options.token}`;
  const response = await fetcher(options.url + '/v1/runner/connect', { method: 'POST', headers: { authorization }, signal: options.signal });
  if (response.status !== 200 || !response.body) { await response.body?.cancel(); throw new Error('The hub refused this runner'); }
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
      pending += chunk;
      if (pending.length > DISPATCH_FRAME_BYTES + 4096) throw new Error('Hub frame exceeds its bound');
      for (let newline = pending.indexOf('\n'); newline >= 0; newline = pending.indexOf('\n')) {
        const line = pending.slice(0, newline); pending = pending.slice(newline + 1);
        if (line) receive(line);
      }
    }
  } catch (error) { if (!options.signal.aborted) throw error; }
  finally { for (const input of leases.values()) input.destroy(); }
}
