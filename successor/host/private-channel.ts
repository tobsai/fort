import { lstatSync, realpathSync } from 'node:fs';
import { request } from 'node:http';
import type { Readable, Writable } from 'node:stream';
import type { ServiceEndpoint } from '../service/openai.ts';
import type { OwnerSetupCancellation, OwnerSetupIdentity, OwnerSetupReceipt, OwnerSetupRequest, OwnerSetupState } from '../contracts/owner-setup.ts';
import { OwnerRecoveryIdentityRejected } from '../contracts/owner-recovery.ts';
import type {SecureNetworkControl} from '../contracts/secure-network.ts';

type PrivateEndpoint = Extract<ServiceEndpoint, { kind: 'privateSocket' }>;
type NativeRequest = { path: string; method: 'GET' | 'POST'; installationID: string; token?: string; body?: Buffer; timeout: number };
type RecoveryRequest = { installationID: string; deviceID: string; timeout: number };
type RecoveryProof = { installationId: string; deviceId: string; runtimeStopped: boolean; revoked: boolean };
type SetupRequest = { operation: 'begin'; timeout: number }
  | ({ operation: 'complete'; timeout: number } & OwnerSetupRequest)
  | ({ operation: 'cancel'; timeout: number } & OwnerSetupIdentity);
type NetworkRequest = {command:unknown;timeout:number};
type ChannelWork = { kind: 'http'; request: NativeRequest } | { kind: 'recovery'; request: RecoveryRequest } | { kind: 'setup'; request: SetupRequest } | {kind:'network';request:NetworkRequest};
const inputLimit = 2 * 1024 * 1024, bodyLimit = 8 * 1024 * 1024;
const protocolError = () => new Error('Private channel protocol or lifecycle was not confirmed');
function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw protocolError();
  return value as Record<string, unknown>;
}
function header(value: unknown): value is string { return typeof value === 'string' && value.length > 0 && value.length <= 4096 && /^[\x20-\x7e]+$/.test(value); }
function identifier(value: unknown): value is string { return typeof value === 'string' && value.length > 0 && value.length <= 128 && !/[^A-Za-z0-9_-]/.test(value); }
function deadline(value: unknown): value is number { return typeof value === 'number' && Number.isFinite(value) && value > 0 && value <= 30; }
function expiry(value: unknown): value is string { return typeof value === 'string' && value.length === 24 && Number.isFinite(Date.parse(value)) && new Date(value).toISOString() === value; }
function decodeNetwork(value:unknown):NetworkRequest {
  const input=object(value);
  if(Object.keys(input).length!==2 || Object.keys(input).some(key=>!['command','timeout'].includes(key)) || !deadline(input.timeout))throw protocolError();
  return {command:input.command,timeout:input.timeout};
}
function decodeRecovery(value: unknown): RecoveryRequest {
  const input = object(value);
  if (Object.keys(input).some(key => !['installationID', 'deviceID', 'timeout'].includes(key))
    || !identifier(input.installationID) || !identifier(input.deviceID)
    || typeof input.timeout !== 'number' || !Number.isFinite(input.timeout) || input.timeout <= 0 || input.timeout > 30) throw protocolError();
  return { installationID: input.installationID, deviceID: input.deviceID, timeout: input.timeout };
}
function decodeSetup(value: unknown): SetupRequest {
  const input = object(value);
  if (!deadline(input.timeout)) throw protocolError();
  const keys = input.operation === 'begin' ? ['operation', 'timeout'] : ['operation', 'timeout', 'installationId', 'setupId', 'deviceId', ...(input.operation === 'complete' ? ['deviceToken'] : [])];
  if (Object.keys(input).some(key => !keys.includes(key))) throw protocolError();
  if (input.operation === 'begin') return { operation: 'begin', timeout: input.timeout };
  if (!identifier(input.installationId) || !identifier(input.setupId) || !identifier(input.deviceId)) throw protocolError();
  const identity = { installationId: input.installationId, setupId: input.setupId, deviceId: input.deviceId };
  if (input.operation === 'cancel') return { operation: 'cancel', timeout: input.timeout, ...identity };
  if (input.operation !== 'complete' || typeof input.deviceToken !== 'string' || !/^[A-Za-z0-9_-]{43}$/.test(input.deviceToken)
    || Buffer.from(input.deviceToken, 'base64url').toString('base64url') !== input.deviceToken) throw protocolError();
  return { operation: 'complete', timeout: input.timeout, ...identity, deviceToken: input.deviceToken };
}
function setupCancellation(value: unknown, identity: OwnerSetupIdentity): OwnerSetupCancellation {
  const input = object(value);
  if (input.installationId !== identity.installationId || input.setupId !== identity.setupId || input.deviceId !== identity.deviceId) throw protocolError();
  if (input.outcome === 'notPaired') return { ...identity, outcome: 'notPaired' };
  if (input.outcome !== 'revoked' || input.runtimeStopped !== true) throw protocolError();
  return { ...identity, outcome: 'revoked', runtimeStopped: true };
}
function setupReceipt(value: unknown): OwnerSetupReceipt {
  const input = object(value);
  if (!identifier(input.installationId) || !identifier(input.deviceId) || (input.expiresAt !== null && !expiry(input.expiresAt))) throw protocolError();
  return { installationId: input.installationId, deviceId: input.deviceId, expiresAt: input.expiresAt };
}
function setupState(value: unknown): OwnerSetupState {
  const input = object(value);
  if (input.kind === 'paired') return { kind: 'paired', ...setupReceipt(input) };
  if (input.kind !== 'available' || !identifier(input.installationId) || !identifier(input.setupId) || !expiry(input.expiresAt)) throw protocolError();
  return { kind: 'available', installationId: input.installationId, setupId: input.setupId, expiresAt: input.expiresAt };
}
function decodeRequest(value: unknown): NativeRequest {
  const input = object(value);
  if (typeof input.path !== 'string' || !input.path.startsWith('/v1/') || input.path.length > 8192 || /[\x00-\x20\x7f#\\]/.test(input.path)
    || (input.method !== 'GET' && input.method !== 'POST') || !header(input.installationID)
    || (input.token !== undefined && input.token !== null && !header(input.token))
    || typeof input.timeout !== 'number' || !Number.isFinite(input.timeout) || input.timeout <= 0 || input.timeout > 30) throw protocolError();
  for (const segment of input.path.split('?')[0].split('/')) {
    const decoded = decodeURIComponent(segment);
    if (decoded === '.' || decoded === '..' || /[/\\]/.test(decoded)) throw protocolError();
  }
  let body: Buffer | undefined;
  if (input.body !== undefined && input.body !== null) {
    if (typeof input.body !== 'string') throw protocolError();
    body = Buffer.from(input.body, 'base64');
    if (body.toString('base64') !== input.body || input.method === 'GET') throw protocolError();
  }
  return { path: input.path, method: input.method, installationID: input.installationID,
    token: typeof input.token === 'string' ? input.token : undefined, body, timeout: input.timeout };
}

function forward(socket: string, input: NativeRequest, signal: AbortSignal): Promise<{ status: number; body: string }> {
  return new Promise((resolve, reject) => {
    const call = request({ socketPath: socket, path: input.path, method: input.method, signal, agent: false,
      headers: { 'Content-Type': 'application/json', 'X-Fort-Installation-Id': input.installationID,
        ...(input.token ? { Authorization: 'Bearer ' + input.token } : {}), ...(input.body ? { 'Content-Length': input.body.length } : {}) } }, response => {
      const parts: Buffer[] = []; let size = 0;
      response.on('data', (part: Buffer) => {
        size += part.length;
        if (size > bodyLimit) call.destroy(protocolError()); else parts.push(part);
      });
      response.once('error', reject);
      response.once('end', () => {
        if (!response.complete || !response.statusCode || size > bodyLimit) reject(protocolError());
        else resolve({ status: response.statusCode, body: Buffer.concat(parts).toString('base64') });
      });
    });
    call.once('error', reject); call.end(input.body);
  });
}

export async function servePrivateChannel(options: {
  endpoint: PrivateEndpoint; input: Readable; output: Writable; signal?: AbortSignal;
  networkControl?: SecureNetworkControl;
  ownerRecovery?: (identity: { installationId: string; deviceId: string }) => Promise<RecoveryProof>;
  ownerSetup?: {
    begin: () => OwnerSetupState | Promise<OwnerSetupState>;
    complete: (request: OwnerSetupRequest) => OwnerSetupReceipt | Promise<OwnerSetupReceipt>;
    cancel?: (identity: OwnerSetupIdentity) => OwnerSetupCancellation | Promise<OwnerSetupCancellation>;
  };
}): Promise<void> {
  const endpoint = { ...options.endpoint }, { input, output, signal, ownerRecovery } = options;
  const networkControl=options.networkControl?.execute.bind(options.networkControl);
  const closeNetwork=options.networkControl?.close.bind(options.networkControl);
  const ownerSetup = options.ownerSetup && { ...options.ownerSetup };
  if (endpoint.kind !== 'privateSocket' || !/^[A-Za-z0-9][A-Za-z0-9_-]{0,47}\.sock$/.test(endpoint.name)
    || realpathSync(endpoint.directory) !== realpathSync(process.cwd())) throw protocolError();
  const directory = lstatSync(endpoint.directory), socket = lstatSync(endpoint.name);
  if (!process.getuid || directory.uid !== process.getuid() || (directory.mode & 0o077) !== 0
    || !socket.isSocket() || socket.uid !== process.getuid() || (socket.mode & 0o077) !== 0) throw protocolError();
  const active = new Map<number, AbortController>(), workers = new Set<Promise<void>>();
  let closing = false, failure: Error | undefined, lastID = 0, pending = Buffer.alloc(0);
  const stop = (error?: Error) => {
    failure ??= error; closing = true;
    for (const controller of active.values()) controller.abort();
    input.destroy(); output.destroy();
  };
  const onFailure = () => stop(protocolError()), onAbort = () => stop();
  input.on('error', onFailure); output.on('error', onFailure); signal?.addEventListener('abort', onAbort, { once: true });
  const write = (value: unknown) => new Promise<void>((resolve, reject) => {
    if (closing) { reject(protocolError()); return; }
    const bytes = Buffer.from(JSON.stringify(value) + '\n');
    if (bytes.length > 12 * 1024 * 1024) { reject(protocolError()); return; }
    const timer = setTimeout(() => { stop(protocolError()); reject(protocolError()); }, 5000);
    output.write(bytes, error => { clearTimeout(timer); if (error) reject(protocolError()); else resolve(); });
  });
  const execute = async (id: number, work: ChannelWork, controller: AbortController) => {
    let timedOut = false;
    const timer = setTimeout(() => { timedOut = true; controller.abort(); }, Math.max(1, Math.ceil(work.request.timeout * 1000)));
    let response: { status: number; body: string } | undefined;
    try {
      controller.signal.throwIfAborted();
      if (work.kind === 'http') response = await forward(endpoint.name, work.request, controller.signal);
      else if(work.kind==='network') {
        if(!networkControl)throw protocolError();
        response={status:200,body:Buffer.from(JSON.stringify(await networkControl(work.request.command))).toString('base64')};
      }
      else if (work.kind === 'recovery') {
        if (!ownerRecovery) throw protocolError();
        const { installationID, deviceID } = work.request;
        const proof = await ownerRecovery({ installationId: installationID, deviceId: deviceID });
        if (proof.installationId !== installationID || proof.deviceId !== deviceID || proof.runtimeStopped !== true || proof.revoked !== true) throw protocolError();
        response = { status: 200, body: Buffer.from(JSON.stringify({ installationId: installationID, deviceId: deviceID, runtimeStopped: true, revoked: true })).toString('base64') };
      } else {
        if (!ownerSetup) throw protocolError();
        let proof: OwnerSetupState | OwnerSetupReceipt | OwnerSetupCancellation;
        if (work.request.operation === 'begin') proof = setupState(await ownerSetup.begin());
        else if (work.request.operation === 'cancel') {
          if (!ownerSetup.cancel) throw protocolError();
          const { installationId, setupId, deviceId } = work.request;
          const identity = { installationId, setupId, deviceId };
          proof = setupCancellation(await ownerSetup.cancel(identity), identity);
        }
        else {
          const { installationId, setupId, deviceId, deviceToken } = work.request;
          proof = setupReceipt(await ownerSetup.complete({ installationId, setupId, deviceId, deviceToken }));
          if (proof.installationId !== installationId || proof.deviceId !== deviceId) throw protocolError();
        }
        response = { status: 200, body: Buffer.from(JSON.stringify(proof)).toString('base64') };
      }
    } catch (error) {
      if (work.kind === 'recovery' && error instanceof OwnerRecoveryIdentityRejected) {
        response = { status: 409, body: Buffer.from(JSON.stringify({ installationId: work.request.installationID, deviceId: work.request.deviceID, outcome: 'identityRejected' })).toString('base64') };
      }
    }
    finally { clearTimeout(timer); }
    if (closing) return;
    if (controller.signal.aborted) response = undefined;
    await write(response ? { protocolVersion: 1, kind: 'response', id, response }
      : { protocolVersion: 1, kind: 'error', id, error: timedOut ? 'timeout' : controller.signal.aborted ? 'canceled' : 'unavailable' });
  };
  try {
    if (signal?.aborted) { stop(); return; }
    await write({ protocolVersion: 1, kind: 'ready' });
    for await (const chunk of input) {
      if (!Buffer.isBuffer(chunk)) throw protocolError();
      pending = Buffer.concat([pending, chunk]);
      for (let newline; (newline = pending.indexOf(10)) >= 0;) {
        if (newline > inputLimit) throw protocolError();
        const bytes = pending.subarray(0, newline); pending = pending.subarray(newline + 1);
        const text = bytes.toString('utf8'); if (!Buffer.from(text).equals(bytes)) throw protocolError();
        const frame = object(JSON.parse(text));
        if (frame.protocolVersion !== 1 || typeof frame.id !== 'number' || !Number.isSafeInteger(frame.id) || frame.id < 1) throw protocolError();
        if (frame.kind === 'cancel') {
          if (frame.id > lastID) throw protocolError();
          active.get(frame.id)?.abort(); continue;
        }
        if (frame.id <= lastID || active.size >= 8) throw protocolError();
        let work: ChannelWork;
        if (frame.kind === 'request') work = { kind: 'http', request: decodeRequest(frame.request) };
        else if(frame.kind==='networkControl' && networkControl)work={kind:'network',request:decodeNetwork(frame.network)};
        else if (frame.kind === 'ownerRecovery' && ownerRecovery) work = { kind: 'recovery', request: decodeRecovery(frame.recovery) };
        else if (frame.kind === 'ownerSetup' && ownerSetup) work = { kind: 'setup', request: decodeSetup(frame.setup) };
        else throw protocolError();
        const id = frame.id;
        lastID = id;
        const controller = new AbortController(); active.set(id, controller);
        const worker = execute(id, work, controller).catch(() => stop(protocolError())).finally(() => { active.delete(id); workers.delete(worker); });
        workers.add(worker);
      }
      if (pending.length > inputLimit) throw protocolError();
    }
    if (pending.length) throw protocolError();
  } catch { if (!closing) failure = protocolError(); }
  finally {
    closing = true;
    for (const controller of active.values()) controller.abort();
    const outcomes=await Promise.allSettled([Promise.resolve().then(()=>closeNetwork?.()),...workers]);
    if(outcomes.some(outcome=>outcome.status==='rejected'))failure=protocolError();
    input.off('error', onFailure); output.off('error', onFailure); signal?.removeEventListener('abort', onAbort);
  }
  if (failure) throw failure;
}
