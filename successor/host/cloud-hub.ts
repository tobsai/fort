import { mkdirSync } from 'node:fs';
import { createServer, request as forward, type IncomingMessage, type ServerResponse } from 'node:http';
import { isAbsolute, join } from 'node:path';
import { createHash, timingSafeEqual } from 'node:crypto';
import { DatabaseSync } from 'node:sqlite';
import type { RuntimeDescriptor } from '../contracts/v1.ts';
import { createApplication } from '../service/openai.ts';
import { RunnerEnrollmentError, runnerEnrollment } from '../service/runner-enrollment.ts';
import { RunnerHub, routeByNode } from './runner-hub.ts';

// Headless hub entry point (Spec 063, step 3). One process serves plain HTTP on $PORT behind the
// host's TLS proxy: the runner channel, runner enrollment, a one-time owner setup, and every other
// /v1 route passed to the Fort core on loopback. Forwarded headers are dropped, never trusted.
export type CloudHubOptions = {
  dataDirectory: string; port: number; host?: string; provider: RuntimeDescriptor;
  // SHA-256 (hex) of the owner's one-time setup code. The hub holds only the digest.
  setupCodeDigest?: string;
  offlineWaitMs?: number;
};

const BODY_BYTES = 64 * 1024, PROXY_BYTES = 1024 * 1024;
const PASSED = ['authorization', 'content-type', 'content-length', 'x-fort-installation-id', 'origin'];

class HubError extends Error {
  readonly status: number; readonly code: string;
  constructor(status: number, code: string, message: string) { super(message); this.status = status; this.code = code; }
}
function fail(status: number, code: string, message: string): never { throw new HubError(status, code, message); }

async function body(request: IncomingMessage): Promise<unknown> {
  if (request.headers['content-type'] !== 'application/json') fail(415, 'invalidBody', 'Send JSON.');
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of request as AsyncIterable<Buffer>) { size += chunk.length; if (size > BODY_BYTES) fail(413, 'bodyTooLarge', 'The request is too large.'); chunks.push(chunk); }
  try { return JSON.parse(Buffer.concat(chunks).toString('utf8')); } catch { return fail(400, 'invalidBody', 'Send valid JSON.'); }
}

export async function startCloudHub(options: CloudHubOptions) {
  const { dataDirectory, provider, offlineWaitMs = 0 } = options;
  if (!isAbsolute(dataDirectory)) throw new Error('Use an absolute data directory');
  if (options.setupCodeDigest !== undefined && !/^[a-f0-9]{64}$/.test(options.setupCodeDigest)) throw new Error('Use a SHA-256 hex digest for the setup code');
  if (!Number.isSafeInteger(offlineWaitMs) || offlineWaitMs < 0 || offlineWaitMs > 600_000) throw new Error('Use an offline wait of at most ten minutes');
  mkdirSync(dataDirectory, { recursive: true, mode: 0o700 });
  const runners = new RunnerHub({ authenticate: token => enrollment.authenticate(token) });
  const app = createApplication({ dataDirectory, provider, runtime: routeByNode(runners, { waitMs: () => offlineWaitMs }), autoRun: true, ownerPairingPolicy: { expiresAfterDays: null } });
  let db: DatabaseSync | undefined;
  try {
    db = new DatabaseSync(join(dataDirectory, 'runners.sqlite'));
    db.exec('PRAGMA locking_mode=EXCLUSIVE; BEGIN EXCLUSIVE; COMMIT; PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL;');
  } catch (error) { db?.close(); await app.close(); throw error; }
  const ledger = db;
  const enrollment = runnerEnrollment(ledger, { revoked: runnerId => runners.disconnect(runnerId) });
  try { await app.start({ listener: { kind: 'loopback' } }); }
  catch (error) { ledger.close(); await app.close().catch(() => {}); throw error; }
  const core = new URL(app.baseURL);

  const owner = (request: IncomingMessage) => {
    const token = /^Bearer ([A-Za-z0-9_-]{43})$/.exec(request.headers.authorization ?? '')?.[1];
    if (request.headers['x-fort-installation-id'] !== app.installation.installationId || !token || !app.ownerDevice(token)) fail(401, 'unauthorized', 'Use the owning Mac’s credential.');
  };
  async function setup(request: IncomingMessage) {
    if (!options.setupCodeDigest) fail(404, 'notFound', 'Setup is not offered.');
    const value = await body(request) as Record<string, unknown>;
    if (!value || typeof value !== 'object' || Object.keys(value).some(key => !['setupCode', 'deviceId', 'deviceToken'].includes(key))) fail(400, 'invalidSetup', 'Use the exact setup fields.');
    const presented = createHash('sha256').update(typeof value.setupCode === 'string' ? value.setupCode : '').digest();
    if (!timingSafeEqual(presented, Buffer.from(options.setupCodeDigest, 'hex'))) fail(401, 'setupDenied', 'This setup code does not match.');
    const offer = app.beginOwnerSetup();
    if (offer.kind !== 'available') fail(409, 'alreadyPaired', 'This hub already has an owner.');
    return app.completeOwnerSetup({ installationId: offer.installationId, setupId: offer.setupId, deviceId: value.deviceId as string, deviceToken: value.deviceToken as string });
  }

  function proxy(request: IncomingMessage, response: ServerResponse) {
    const headers: Record<string, string> = {};
    for (const name of PASSED) { const value = request.headers[name]; if (typeof value === 'string') headers[name] = value; }
    if (Number(headers['content-length'] ?? 0) > PROXY_BYTES || request.headers['transfer-encoding']) { response.writeHead(413, { connection: 'close' }).end(); request.resume(); return; }
    const call = forward({ host: core.hostname, port: core.port, path: request.url, method: request.method, headers, agent: false }, reply => {
      const passed: Record<string, string> = { 'cache-control': 'no-store' };
      if (typeof reply.headers['content-type'] === 'string') passed['content-type'] = reply.headers['content-type'];
      response.writeHead(reply.statusCode ?? 502, passed);
      reply.pipe(response);
    });
    call.once('error', () => { if (!response.headersSent) response.writeHead(502).end(); else response.destroy(); });
    response.once('close', () => call.destroy());
    request.pipe(call);
  }

  const server = createServer({ maxHeaderSize: 16 * 1024 }, (request, response) => {
    if (runners.handle(request, response)) return;
    const path = new URL(request.url ?? '/', 'http://hub').pathname;
    const local = async (): Promise<[number, unknown] | undefined> => {
      if (request.method === 'POST' && path === '/v1/hub/setup') return [201, await setup(request)];
      if (request.method === 'POST' && path === '/v1/runners/join') return [201, enrollment.join(await body(request))];
      if (request.method === 'GET' && path === '/v1/runners') { owner(request); return [200, { runners: enrollment.list().map(runner => ({ ...runner, online: runners.online(runner.runnerId) })) }]; }
      if (request.method === 'POST' && path === '/v1/runners/invitation') { owner(request); return [201, enrollment.invite(await body(request))]; }
      if (request.method === 'POST' && path === '/v1/runners/revoke') {
        owner(request);
        const value = await body(request) as { runnerId?: unknown };
        if (typeof value?.runnerId !== 'string' || !enrollment.revoke(value.runnerId)) fail(404, 'notFound', 'No enrolled Mac has this ID.');
        return [200, { runnerId: value.runnerId, revoked: true }];
      }
      return undefined;
    };
    void local().then(result => {
      if (!result) { proxy(request, response); return; }
      response.writeHead(result[0], { 'content-type': 'application/json', 'cache-control': 'no-store' }).end(JSON.stringify(result[1]));
    }, (error: unknown) => {
      const known = error instanceof HubError || error instanceof RunnerEnrollmentError ? error : undefined;
      const status = known?.status ?? (error && typeof error === 'object' && 'status' in error && typeof error.status === 'number' ? error.status : 500);
      const code = known?.code ?? (error && typeof error === 'object' && 'code' in error && typeof error.code === 'string' ? error.code : 'hubError');
      if (!response.headersSent) response.writeHead(status, { 'content-type': 'application/json', 'cache-control': 'no-store', connection: 'close' })
        .end(JSON.stringify({ error: { code, message: known?.message ?? 'The request could not be completed.' } }));
      request.resume();
    });
  });
  server.maxConnections = 128;
  server.headersTimeout = 10_000;
  server.requestTimeout = 0; // The runner channel and event replay are long-lived; pings keep them honest.
  try { await new Promise<void>((resolve, reject) => { server.once('error', reject); server.listen(options.port, options.host ?? '0.0.0.0', () => { server.off('error', reject); resolve(); }); }); }
  catch (error) { runners.close(); await app.close().catch(() => {}); ledger.close(); throw error; }
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Missing listener');
  let closing: Promise<void> | undefined;
  return {
    url: `http://${address.family === 'IPv6' ? `[${address.address}]` : address.address}:${address.port}`,
    installation: app.installation,
    close(): Promise<void> {
      closing ??= (async () => {
        runners.close();
        server.closeAllConnections();
        await new Promise(resolve => server.close(resolve));
        try { await app.close(); } finally { ledger.close(); }
      })();
      return closing;
    },
  };
}

// Container entry: node host/cloud-hub.ts. Configuration comes only from the environment.
if (import.meta.main) {
  const env = process.env, port = Number(env.PORT ?? 8080);
  const required = (name: string) => { const value = env[name]; if (!value) throw new Error(`Set ${name}`); return value; };
  const provider: RuntimeDescriptor = { provider: required('FORT_PROVIDER'), name: env.FORT_PROVIDER_NAME ?? required('FORT_PROVIDER'),
    runtimeVersion: required('FORT_RUNTIME_VERSION'), hostId: required('FORT_NODE'), session: 'stateless', capabilities: ['plainText', 'cancel'] };
  const hub = await startCloudHub({ dataDirectory: required('FORT_DATA'), port, provider,
    ...(env.FORT_SETUP_CODE_SHA256 ? { setupCodeDigest: env.FORT_SETUP_CODE_SHA256 } : {}),
    offlineWaitMs: Number(env.FORT_OFFLINE_WAIT_MS ?? 0) });
  process.stdout.write(`Fort hub listening on port ${port}\n`);
  const stop = () => { void hub.close().finally(() => process.exit(0)); };
  process.once('SIGTERM', stop); process.once('SIGINT', stop);
}
