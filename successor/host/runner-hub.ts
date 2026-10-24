import { randomUUID } from 'node:crypto';
import type { IncomingMessage, ServerResponse } from 'node:http';
import type { ExecutionDispatch } from '../contracts/executor.ts';
import type { ChatInput, ChatResult, ChatRuntime } from '../runner/chat-runtime.ts';
import { terminal, validDispatch } from '../runner/executor-process.ts';

// Hub side of the runner channel (Spec 063). A runner holds one outbound POST to /v1/runner/connect;
// the hub streams lease, cancel and ping frames down it as NDJSON. The runner returns executor
// evidence for a lease as separate authenticated POSTs to /v1/runner/frames.
type HubOptions = { authenticate(token: string): string | undefined; pingMs?: number };
type Connection = { runnerId: string; response: ServerResponse; leases: Set<string>; timer: ReturnType<typeof setInterval> };
type Lease = { runnerId: string; connection: Connection; receive(frame: unknown): void; ended(): void; lost(): void };

const FRAMES_BYTES = 1024 * 1024;
const TOKEN = /^Bearer ([A-Za-z0-9._~-]{1,512})$/;
const OFFLINE = 'That Mac is not connected. Nothing was sent, and the turn was not moved to another machine.';

export class RunnerHub {
  private readonly options: HubOptions;
  private readonly connections = new Map<string, Connection>();
  private readonly leases = new Map<string, Lease>();
  private readonly waiters = new Map<string, Set<() => void>>();
  private closed = false;
  constructor(options: HubOptions) { this.options = options; }

  online(runnerId: string): boolean { return this.connections.has(runnerId); }

  // Resolves true once the runner is connected, or false when the wait or the signal ends first.
  async whenOnline(runnerId: string, waitMs: number, signal: AbortSignal): Promise<boolean> {
    if (this.online(runnerId)) return true;
    if (waitMs <= 0 || signal.aborted) return false;
    const done = Promise.withResolvers<boolean>(), waiters = this.waiters.get(runnerId) ?? new Set();
    const arrived = () => done.resolve(true), gone = () => done.resolve(false), timer = setTimeout(gone, waitMs);
    this.waiters.set(runnerId, waiters.add(arrived));
    signal.addEventListener('abort', gone, { once: true });
    try { return await done.promise; } finally {
      clearTimeout(timer); signal.removeEventListener('abort', gone);
      waiters.delete(arrived); if (!waiters.size) this.waiters.delete(runnerId);
    }
  }

  // Drops a runner's connection, for example after its enrollment is revoked. Its running leases
  // become unknown; whether it may reconnect is decided by authenticate().
  disconnect(runnerId: string): void {
    const connection = this.connections.get(runnerId);
    if (connection) this.drop(connection);
  }

  // Returns false for paths outside the runner channel so the caller can route them.
  handle(request: IncomingMessage, response: ServerResponse): boolean {
    const path = new URL(request.url ?? '/', 'http://hub').pathname;
    if (path === '/v1/runner/connect') { this.connect(request, response); return true; }
    if (path === '/v1/runner/frames') { this.frames(request, response); return true; }
    return false;
  }

  executor(runnerId: string): ChatRuntime {
    return { execute: (input, signal, dispatch) => this.execute(runnerId, input, signal, dispatch), dispose: async () => {} };
  }

  close(): void {
    this.closed = true;
    for (const connection of [...this.connections.values()]) this.drop(connection);
  }

  private runner(request: IncomingMessage): string | undefined {
    const match = TOKEN.exec(request.headers.authorization ?? '');
    return match ? this.options.authenticate(match[1]!) : undefined;
  }

  private connect(request: IncomingMessage, response: ServerResponse) {
    if (request.method !== 'POST') { response.writeHead(405).end(); return; }
    const runnerId = this.runner(request);
    if (!runnerId || this.closed) { response.writeHead(401).end(); return; }
    const previous = this.connections.get(runnerId);
    if (previous) this.drop(previous);
    response.writeHead(200, { 'content-type': 'application/x-ndjson', 'cache-control': 'no-store' });
    response.flushHeaders();
    const connection: Connection = { runnerId, response, leases: new Set(),
      timer: setInterval(() => { response.write('{"kind":"ping"}\n'); }, this.options.pingMs ?? 15_000) };
    this.connections.set(runnerId, connection);
    response.once('close', () => this.drop(connection));
    request.resume();
    for (const arrived of [...this.waiters.get(runnerId) ?? []]) arrived();
  }

  private drop(connection: Connection) {
    clearInterval(connection.timer);
    if (this.connections.get(connection.runnerId) === connection) this.connections.delete(connection.runnerId);
    for (const id of [...connection.leases]) this.leases.get(id)?.lost();
    if (!connection.response.writableEnded) connection.response.end();
  }

  private frames(request: IncomingMessage, response: ServerResponse) {
    if (request.method !== 'POST') { response.writeHead(405).end(); return; }
    const runnerId = this.runner(request);
    if (!runnerId) { response.writeHead(401).end(); request.resume(); return; }
    if (Number(request.headers['content-length'] ?? 0) > FRAMES_BYTES) { response.writeHead(413, { connection: 'close' }).end(); request.resume(); return; }
    const chunks: Buffer[] = [];
    let size = 0;
    request.on('data', (chunk: Buffer) => { size += chunk.length; if (size <= FRAMES_BYTES) chunks.push(chunk); });
    request.once('end', () => {
      if (size > FRAMES_BYTES) { response.writeHead(413).end(); return; }
      let accepted = true;
      try {
        for (const line of new TextDecoder('utf-8', { fatal: true }).decode(Buffer.concat(chunks)).split('\n')) {
          if (!line) continue;
          const row = JSON.parse(line) as { leaseId?: unknown; frame?: unknown; ended?: unknown };
          const lease = typeof row.leaseId === 'string' ? this.leases.get(row.leaseId) : undefined;
          if (!lease || lease.runnerId !== runnerId) { accepted = false; continue; }
          if (row.ended === true) lease.ended(); else lease.receive(row.frame);
        }
      } catch { response.writeHead(400).end(); return; }
      response.writeHead(accepted ? 204 : 409).end();
    });
  }

  private async execute(runnerId: string, input: ChatInput, signal: AbortSignal, dispatch: ExecutionDispatch): Promise<ChatResult> {
    if (signal.aborted) return { kind: 'notStarted', message: 'Canceled before dispatch. Nothing was sent.' };
    if (dispatch.binding.hostId !== runnerId) return { kind: 'notStarted', message: 'This turn is pinned to a different Mac. Nothing was sent.' };
    if (!validDispatch(input, dispatch)) return { kind: 'notStarted', message: 'Execution scope, input, policy or deadline was not confirmed. Nothing was sent.' };
    const connection = this.connections.get(runnerId);
    if (!connection) return { kind: 'notStarted', message: OFFLINE };
    const leaseId = randomUUID(), outcome = Promise.withResolvers<ChatResult>();
    const unknown = (message: string) => outcome.resolve({ kind: 'outcomeUnknown', message });
    this.leases.set(leaseId, { runnerId, connection,
      receive: frame => { try { outcome.resolve(terminal(frame, dispatch)); } catch { unknown('The Mac returned evidence that could not be confirmed. Earlier effects remain unknown.'); } },
      ended: () => unknown('The Mac ended this turn without a confirmed reply. Earlier effects remain unknown.'),
      lost: () => unknown('The Mac disconnected before a reply was confirmed. Earlier effects remain unknown.') });
    connection.leases.add(leaseId);
    const expiry = setTimeout(() => unknown('No confirmed reply arrived before the deadline. Earlier effects remain unknown.'), Math.max(1, dispatch.deadline - Date.now() + 1000));
    const cancel = () => connection.response.writableEnded || connection.response.write(JSON.stringify({ kind: 'cancel', leaseId,
      cancel: { protocolVersion: 1, kind: 'cancel', dispatchDigest: dispatch.dispatchDigest, attemptId: dispatch.attempt.attemptId, generation: dispatch.attempt.generation } }) + '\n');
    signal.addEventListener('abort', cancel, { once: true });
    try {
      connection.response.write(JSON.stringify({ kind: 'lease', leaseId, request: { protocolVersion: 1, kind: 'dispatch', dispatch, input } }) + '\n');
      return await outcome.promise;
    } finally {
      clearTimeout(expiry); signal.removeEventListener('abort', cancel);
      this.leases.delete(leaseId); connection.leases.delete(leaseId);
    }
  }
}

// Sends each turn to the runner its binding pins (binding.hostId), never elsewhere. When that Mac is
// offline the turn is refused at once, or after waiting up to waitMs for it to connect.
export function routeByNode(hub: RunnerHub, options: { waitMs?: (dispatch: ExecutionDispatch) => number } = {}): ChatRuntime {
  return {
    async execute(input, signal, dispatch) {
      const runnerId = dispatch.binding.hostId;
      const wait = Math.min(options.waitMs?.(dispatch) ?? 0, dispatch.deadline - Date.now());
      if (!await hub.whenOnline(runnerId, wait, signal)) return { kind: 'notStarted', message: signal.aborted ? 'Canceled before dispatch. Nothing was sent.' : OFFLINE };
      return hub.executor(runnerId).execute(input, signal, dispatch);
    },
    dispose: async () => {},
  };
}
