import { randomUUID } from 'node:crypto';
import type { IncomingMessage, ServerResponse } from 'node:http';
import { executionDigest, type ExecutionDispatch } from '../contracts/executor.ts';
import { parseAccountIdentity } from '../contracts/account.ts';
import type { AccountIdentity, AccountSignInOptions, AccountSignInResult, ModelCatalog } from '../contracts/v1.ts';
import type { ChatInput, ChatResult, ChatRuntime } from '../runner/chat-runtime.ts';
import { terminal, validDispatch } from '../runner/executor-process.ts';
import { parseExecutorModelFrame, validateExecutorBinding, type ExecutorBinding, type ExecutorModelRequest } from '../runner/executor-wire.ts';
import { parseExecutorAccountFrame, type ExecutorAccountRequest } from '../runner/executor-account-wire.ts';
import { chooseRunner, type RunPins, type RunnerPool } from './runner-pool.ts';

// Hub side of the runner channel (Spec 063). A runner holds one outbound POST to /v1/runner/connect;
// the hub streams lease, cancel and ping frames down it as NDJSON. The runner returns executor
// evidence for a lease as separate authenticated POSTs to /v1/runner/frames.
// nodeOf names the node a runner serves (its own ID unless it is in a pool); onOnline hears each connect.
type HubOptions = { authenticate(token: string): string | undefined; pingMs?: number; nodeOf?(runnerId: string): string; onOnline?(runnerId: string): void };
type Connection = { runnerId: string; response: ServerResponse; leases: Set<string>; timer: ReturnType<typeof setInterval> };
type Lease = { runnerId: string; connection: Connection; receive(frame: unknown): void; ended(): void; lost(): void };
// One request leased to a runner: the executor wire request, its cancel frame, and how its evidence settles.
type Exchange<T> = { request: object; cancel: object; deadline: number; receive(frame: unknown, settle: (value: T) => void): void; unknown(message: string): T };

const FRAMES_BYTES = 1024 * 1024;
// The Mac refuses a model request due more than 5 seconds out; a second of margin covers clock skew.
const MODEL_WINDOW_MS = 4000;
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
    this.options.onOnline?.(runnerId);
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
    if (dispatch.binding.hostId !== (this.options.nodeOf?.(runnerId) ?? runnerId)) return { kind: 'notStarted', message: 'This turn is pinned to a different Mac. Nothing was sent.' };
    if (!validDispatch(input, dispatch)) return { kind: 'notStarted', message: 'Execution scope, input, policy or deadline was not confirmed. Nothing was sent.' };
    if (!this.connections.has(runnerId)) return { kind: 'notStarted', message: OFFLINE };
    return this.lease<ChatResult>(runnerId, signal, {
      request: { protocolVersion: 1, kind: 'dispatch', dispatch, input },
      cancel: { protocolVersion: 1, kind: 'cancel', dispatchDigest: dispatch.dispatchDigest, attemptId: dispatch.attempt.attemptId, generation: dispatch.attempt.generation },
      deadline: dispatch.deadline, receive: (frame, settle) => settle(terminal(frame, dispatch)),
      unknown: message => ({ kind: 'outcomeUnknown', message }),
    });
  }

  // Asks the Mac which models its signed-in account offers on this lane. Nothing is cached on the hub.
  // A pooled node's Mac is named by runnerId; otherwise the binding's hostId is the Mac.
  async models(binding: ExecutorBinding, signal: AbortSignal, account?: AccountIdentity, runnerId = binding.hostId): Promise<ModelCatalog> {
    if (!this.connections.has(runnerId) || signal.aborted) return { provider: binding.provider, state: 'unavailable', models: [], detail: 'That Mac is not connected, so its models cannot be checked. Your selection has not changed.' };
    const body = { ...(account ? { account: parseAccountIdentity(account) } : {}), protocolVersion: 1, kind: 'models', requestId: randomUUID(), binding: { ...binding }, deadline: Date.now() + MODEL_WINDOW_MS } as const;
    const request: ExecutorModelRequest = { ...body, requestDigest: executionDigest(body) };
    return this.lease<ModelCatalog>(runnerId, signal, { request, deadline: request.deadline,
      cancel: { protocolVersion: 1, kind: 'cancelModels', requestId: request.requestId, requestDigest: request.requestDigest },
      receive: (frame, settle) => settle(parseExecutorModelFrame(frame, request)),
      unknown: () => ({ provider: binding.provider, state: 'error', models: [], detail: 'The Mac did not confirm its models. Your selection has not changed.' }) });
  }

  // Confirms the account the Mac's runtime is signed in to. A browser sign-in, when one is needed,
  // happens on the Mac; the challenge is passed back so the owner can see it.
  async signIn(binding: ExecutorBinding, options: AccountSignInOptions, signal: AbortSignal, runnerId = binding.hostId): Promise<AccountSignInResult> {
    const { durationMs, onChallenge } = options;
    if (!this.connections.has(runnerId) || signal.aborted || !Number.isSafeInteger(durationMs) || durationMs < 1 || durationMs > 115_000) return { kind: 'unconfirmed' };
    const workDeadline = Date.now() + durationMs;
    const body = { protocolVersion: 1, kind: 'accountSignIn', requestId: randomUUID(), binding: { ...binding }, workDeadline, deadline: workDeadline + 5000 } as const;
    const request: ExecutorAccountRequest = { ...body, requestDigest: executionDigest(body) };
    let offered = false;
    return this.lease<AccountSignInResult>(runnerId, signal, { request, deadline: request.deadline,
      cancel: { protocolVersion: 1, kind: 'cancelAccount', requestId: request.requestId, requestDigest: request.requestDigest },
      receive: (frame, settle) => {
        const parsed = parseExecutorAccountFrame(frame, request);
        if (parsed.kind === 'result') { settle(parsed.value); return; }
        if (offered) throw new Error('A second account challenge arrived');
        offered = true; onChallenge({ ...parsed.value });
      },
      unknown: () => ({ kind: 'unconfirmed' }) });
  }

  private async lease<T>(runnerId: string, signal: AbortSignal, exchange: Exchange<T>): Promise<T> {
    const connection = this.connections.get(runnerId);
    if (!connection) return exchange.unknown(OFFLINE);
    const leaseId = randomUUID(), outcome = Promise.withResolvers<T>();
    const unknown = (message: string) => outcome.resolve(exchange.unknown(message));
    this.leases.set(leaseId, { runnerId, connection,
      receive: frame => { try { exchange.receive(frame, outcome.resolve); } catch { unknown('The Mac returned evidence that could not be confirmed. Earlier effects remain unknown.'); } },
      ended: () => unknown('The Mac ended this turn without a confirmed reply. Earlier effects remain unknown.'),
      lost: () => unknown('The Mac disconnected before a reply was confirmed. Earlier effects remain unknown.') });
    connection.leases.add(leaseId);
    const expiry = setTimeout(() => unknown('No confirmed reply arrived before the deadline. Earlier effects remain unknown.'), Math.max(1, exchange.deadline - Date.now() + 1000));
    const cancel = () => connection.response.writableEnded || connection.response.write(JSON.stringify({ kind: 'cancel', leaseId, cancel: exchange.cancel }) + '\n');
    signal.addEventListener('abort', cancel, { once: true });
    try {
      connection.response.write(JSON.stringify({ kind: 'lease', leaseId, request: exchange.request }) + '\n');
      return await outcome.promise;
    } finally {
      clearTimeout(expiry); signal.removeEventListener('abort', cancel);
      this.leases.delete(leaseId); connection.leases.delete(leaseId);
    }
  }
}

// Sends each turn to the runner its binding pins (binding.hostId), never elsewhere. When that Mac is
// offline the turn is refused at once, or after waiting up to waitMs for it to connect.
// With a binding, the lane also checks models and confirms the Mac's signed-in account through the
// channel, so the hub pins each turn to the account the Mac reports.
export function routeByNode(hub: RunnerHub, options: { waitMs?: (dispatch: ExecutionDispatch) => number; binding?: ExecutorBinding } = {}): ChatRuntime {
  const binding = options.binding && Object.freeze({ provider: options.binding.provider, runtimeVersion: options.binding.runtimeVersion, hostId: options.binding.hostId });
  if (binding) validateExecutorBinding(binding);
  return {
    ...(binding ? { models: (signal: AbortSignal, account?: AccountIdentity) => hub.models(binding, signal, account),
      signIn: (request: AccountSignInOptions, signal: AbortSignal) => hub.signIn(binding, request, signal) } : {}),
    async execute(input, signal, dispatch) {
      const runnerId = dispatch.binding.hostId;
      const wait = Math.min(options.waitMs?.(dispatch) ?? 0, dispatch.deadline - Date.now());
      if (!await hub.whenOnline(runnerId, wait, signal)) return { kind: 'notStarted', message: signal.aborted ? 'Canceled before dispatch. Nothing was sent.' : OFFLINE };
      return hub.executor(runnerId).execute(input, signal, dispatch);
    },
    dispose: async () => {},
  };
}

// Sends each turn of a pooled node (Spec 063, revision 2026-10-28) to one of the pool's Macs: the first
// connected one in order of preference, or the Mac the run is already pinned to. The lane reports itself
// unavailable while no Mac in the pool is connected, so the core keeps runs queued in its ledger instead
// of starting them; the hub wakes the core when a Mac connects. Model and account checks go to the first
// connected Mac. Each Mac still refuses a turn for an account it is not signed in to.
export function routePool(hub: RunnerHub, pool: RunnerPool, options: { binding: ExecutorBinding; pins: RunPins; waitMs?: () => number; names?: (runnerId: string) => string | undefined; detail?: () => string | undefined }): ChatRuntime {
  const binding = Object.freeze({ provider: options.binding.provider, runtimeVersion: options.binding.runtimeVersion, hostId: options.binding.hostId });
  validateExecutorBinding(binding);
  if (binding.hostId !== pool.node) throw new Error('A pooled lane must use the pool’s node ID');
  const online = (runnerId: string) => hub.online(runnerId);
  const name = (runnerId: string) => options.names?.(runnerId) ?? runnerId;
  // Resolves once some Mac can take the run, or undefined when the wait or the signal ends first.
  async function available(pinned: string | undefined, waitMs: number, signal: AbortSignal): Promise<string | undefined> {
    const end = Date.now() + Math.max(0, waitMs);
    for (;;) {
      const chosen = chooseRunner(pool, online, pinned);
      if (chosen !== undefined || signal.aborted || Date.now() >= end) return chosen;
      const candidates = pinned === undefined ? pool.runners : [pinned];
      const step = new AbortController(), stop = () => step.abort();
      signal.addEventListener('abort', stop, { once: true });
      try { await Promise.race(candidates.map(id => hub.whenOnline(id, end - Date.now(), step.signal))); }
      finally { step.abort(); signal.removeEventListener('abort', stop); }
    }
  }
  return {
    models: (signal: AbortSignal, account?: AccountIdentity) => {
      const runnerId = chooseRunner(pool, online);
      return runnerId === undefined ? hub.models(binding, signal, account, pool.runners[0]) : hub.models(binding, signal, account, runnerId);
    },
    signIn: (request: AccountSignInOptions, signal: AbortSignal) => hub.signIn(binding, request, signal, chooseRunner(pool, online) ?? pool.runners[0]),
    readiness: () => {
      const connected = pool.runners.filter(online);
      const extra = options.detail?.();
      return connected.length
        ? { state: 'ready', canRestart: false, detail: `Runs go to ${name(connected[0]!)}.` }
        : { state: 'unavailable', canRestart: false, detail: (extra ? extra + ' ' : '') + `Waiting for ${pool.runners.map(name).join(' or ')} to connect. Queued work starts when one does.` };
    },
    async execute(input, signal, dispatch) {
      if (dispatch.binding.hostId !== pool.node) return { kind: 'notStarted', message: 'This turn is pinned to a different Mac. Nothing was sent.' };
      const pinned = options.pins.get(dispatch.attempt.runId);
      const wait = Math.min(options.waitMs?.() ?? 0, dispatch.deadline - Date.now());
      const runnerId = await available(pinned, wait, signal);
      if (runnerId === undefined) return { kind: 'notStarted', message: signal.aborted ? 'Canceled before dispatch. Nothing was sent.'
        : pinned !== undefined ? `This run started on ${name(pinned)}, which is not connected. Nothing was sent, and the run was not moved to another Mac.` : OFFLINE };
      options.pins.pin(dispatch.attempt.runId, runnerId);
      return hub.executor(runnerId).execute(input, signal, dispatch);
    },
    dispose: async () => {},
  };
}
