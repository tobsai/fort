import { existsSync, mkdirSync, readFileSync, renameSync, rmSync, writeFileSync } from 'node:fs';
import { createServer, request as forward, type IncomingMessage, type ServerResponse } from 'node:http';
import { isAbsolute, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash, timingSafeEqual } from 'node:crypto';
import { DatabaseSync } from 'node:sqlite';
import type { RuntimeDescriptor } from '../contracts/v1.ts';
import { createApplication } from '../service/openai.ts';
import { RunnerEnrollmentError, runnerEnrollment } from '../service/runner-enrollment.ts';
import { RunnerHub, routeByNode, routePool } from './runner-hub.ts';
import { parseRunnerPool, runPins, type RunnerPool } from './runner-pool.ts';
import { importLedger, inspectLedgerSnapshot } from '../service/import-ledger.ts';
import { SignInError, googleSignIn, type GoogleSignInOptions } from './google-sign-in.ts';
import { LOCAL_CONTEXT } from '../modules/local-context.ts';

// Headless hub entry point (Spec 063, step 3). One process serves plain HTTP on $PORT behind the
// host's TLS proxy: the runner channel, runner enrollment, a one-time owner setup, and every other
// /v1 route passed to the Fort core on loopback. Forwarded headers are dropped, never trusted.
export type CloudHubOptions = {
  dataDirectory: string; port: number; host?: string; provider: RuntimeDescriptor;
  // More runtimes on the same hub (Spec 064), for example Claude beside ChatGPT. Each lane is routed to
  // the Mac its hostId names, and checks models and accounts on that Mac.
  runtimes?: RuntimeDescriptor[];
  // The hub's public https origin. Only pages from this origin (the web client the hub serves) may call
  // the API from a browser; without it, browser requests are refused as before.
  publicOrigin?: string;
  // SHA-256 (hex) of the owner's one-time setup code. The hub holds only the digest.
  setupCodeDigest?: string;
  // Puts every browser path behind Google sign-in for the named emails (Spec 066). Needs publicOrigin.
  google?: GoogleSignInOptions;
  offlineWaitMs?: number;
  // Several Macs serving this hub's node (Spec 063, revision 2026-10-28): runner IDs, primary first.
  // Each run goes to the first connected one and stays there; with none connected, runs wait queued.
  pool?: readonly string[];
  // How long queued work may wait with no Mac connected before onQueueAlert is called. Default 15 minutes.
  queueAlertMs?: number;
  onQueueAlert?: (alert: QueueAlert) => void;
  // Called once a Mac's ledger snapshot is staged. The hub applies it the next time it starts, so the
  // container entry restarts the hub in place.
  onImportStaged?: () => void;
};

export type QueueAlert = { waiting: number; waitingSince: string; runners: readonly string[] };

const BODY_BYTES = 64 * 1024, PROXY_BYTES = 1024 * 1024, SNAPSHOT_BYTES = 256 * 1024 * 1024, HANDOFF_BYTES = 32 * 1024 * 1024;
const PENDING = 'ledger-import.sqlite', RESULT = 'ledger-import.json';
// The web client: a few static files, served with a policy that allows only this origin's own scripts.
const WEB = new Map([['/', ['index.html', 'text/html; charset=utf-8']], ['/signin.js', ['signin.js', 'text/javascript; charset=utf-8']], ['/app.js', ['app.js', 'text/javascript; charset=utf-8']], ['/transcript.js', ['transcript.js', 'text/javascript; charset=utf-8']],
  ['/app.css', ['app.css', 'text/css; charset=utf-8']], ['/icon.svg', ['icon.svg', 'image/svg+xml']], ['/manifest.webmanifest', ['manifest.webmanifest', 'application/manifest+json']]]);
const WEB_HEADERS = { 'cache-control': 'no-cache', 'x-content-type-options': 'nosniff', 'referrer-policy': 'no-referrer', 'x-frame-options': 'DENY',
  'content-security-policy': "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; manifest-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'" };
// With the Google gate, the page may also load Google's sign-in button, which needs the page's origin as referrer.
const GSI = 'https://accounts.google.com/gsi/';
const GOOGLE_WEB_HEADERS = { ...WEB_HEADERS, 'referrer-policy': 'strict-origin-when-cross-origin',
  'content-security-policy': `default-src 'none'; script-src 'self' ${GSI}client; style-src 'self' ${GSI}style; frame-src ${GSI}; img-src 'self' data:; connect-src 'self' ${GSI}; manifest-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'` };
// Without a Google session only these pages load: enough to show the sign-in button, nothing of Fort's.
const PUBLIC_PAGES = new Set(['/', '/signin.js', '/app.css', '/icon.svg', '/manifest.webmanifest']);
// Machine paths keep their own credentials (runner token, one-time invitation codes) and never see a browser.
const MACHINE_PATHS = new Set(['/v1/runners/join', '/v1/runners/import', '/v1/runners/agents', '/v1/runners/client', '/v1/devices/join', '/v1/pairing/leave']);
const PASSED = ['authorization', 'content-type', 'content-length', 'x-fort-installation-id'];

class HubError extends Error {
  readonly status: number; readonly code: string;
  constructor(status: number, code: string, message: string) { super(message); this.status = status; this.code = code; }
}
function fail(status: number, code: string, message: string): never { throw new HubError(status, code, message); }

async function body(request: IncomingMessage, limit = BODY_BYTES): Promise<unknown> {
  if (request.headers['content-type'] !== 'application/json') fail(415, 'invalidBody', 'Send JSON.');
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of request as AsyncIterable<Buffer>) { size += chunk.length; if (size > limit) fail(413, 'bodyTooLarge', 'The request is too large.'); chunks.push(chunk); }
  try { return JSON.parse(Buffer.concat(chunks).toString('utf8')); } catch { return fail(400, 'invalidBody', 'Send valid JSON.'); }
}

// A staged Mac ledger is applied before the core opens its ledger. A refusal is recorded and the hub
// starts as it was; it never stops the hub from starting.
function applyStagedImport(dataDirectory: string, hostId: string) {
  const pending = join(dataDirectory, PENDING);
  if (!existsSync(pending)) return;
  let result: unknown;
  try { result = { state: 'applied', ...importLedger({ source: pending, ledger: join(dataDirectory, 'openai-candidate-b.sqlite'), hostId }) }; }
  catch (error) { result = { state: 'refused', message: error instanceof Error ? error.message.slice(0, 300) : 'The import could not be applied.' }; }
  writeFileSync(join(dataDirectory, RESULT), JSON.stringify(result), { mode: 0o600 });
  rmSync(pending, { force: true });
}

function bareOrigin(value: string): string {
  const url = new URL(value);
  if (url.origin !== value.replace(/\/$/, '') || (url.protocol !== 'https:' && !['127.0.0.1', 'localhost'].includes(url.hostname))) throw new Error('Use the hub’s bare https origin as its public address');
  return url.origin;
}

export async function startCloudHub(options: CloudHubOptions) {
  const { dataDirectory, provider, offlineWaitMs = 0 } = options;
  if (!isAbsolute(dataDirectory)) throw new Error('Use an absolute data directory');
  const publicOrigin = options.publicOrigin === undefined ? undefined : bareOrigin(options.publicOrigin);
  const web = new Map([...WEB].map(([path, [file, type]]) => [path, { type: type!, data: readFileSync(fileURLToPath(new URL('../web/' + file, import.meta.url))) }]));
  if (options.google && publicOrigin === undefined) throw new Error('Google sign-in needs the hub’s public origin');
  if (options.setupCodeDigest !== undefined && !/^[a-f0-9]{64}$/.test(options.setupCodeDigest)) throw new Error('Use a SHA-256 hex digest for the setup code');
  if (!Number.isSafeInteger(offlineWaitMs) || offlineWaitMs < 0 || offlineWaitMs > 600_000) throw new Error('Use an offline wait of at most ten minutes');
  mkdirSync(dataDirectory, { recursive: true, mode: 0o700 });
  applyStagedImport(dataDirectory, provider.hostId);
  const google = options.google ? googleSignIn(dataDirectory, options.google) : undefined;
  const pageHeaders = google ? GOOGLE_WEB_HEADERS : WEB_HEADERS;
  const queueAlertMs = options.queueAlertMs ?? 15 * 60_000;
  if (!Number.isSafeInteger(queueAlertMs) || queueAlertMs < 1000) throw new Error('Use a queue alert of at least a second');
  const pool: RunnerPool | undefined = options.pool === undefined ? undefined : { node: provider.hostId, runners: [...options.pool] };
  if (pool && (!pool.runners.length || new Set(pool.runners).size !== pool.runners.length || (options.runtimes ?? []).some(descriptor => descriptor.hostId !== pool.node))) throw new Error('Use distinct pool runners and one node for every runtime');
  let db: DatabaseSync | undefined;
  try {
    db = new DatabaseSync(join(dataDirectory, 'runners.sqlite'));
    db.exec('PRAGMA locking_mode=EXCLUSIVE; BEGIN EXCLUSIVE; COMMIT; PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL;');
  } catch (error) { db?.close(); throw error; }
  const ledger = db;
  // Removing a Mac also ends its client access to this hub.
  const enrollment = runnerEnrollment(ledger, { revoked: runnerId => { runners.disconnect(runnerId); app.removeRunnerClient(runnerId); } });
  const pins = pool ? runPins(ledger) : undefined;
  const nodeOf = (runnerId: string) => pool?.runners.includes(runnerId) ? pool.node : runnerId;
  // A Mac connecting may unblock queued work, so the core looks again.
  const pump = () => { void (async () => { while (await app.dispatchNext()) { /* one turn at a time */ } })().catch(() => {}); };
  const runners = new RunnerHub({ authenticate: token => enrollment.authenticate(token), nodeOf, ...(pool ? { onOnline: () => queueMicrotask(pump) } : {}) });
  const names = (runnerId: string) => enrollment.list().find(runner => runner.runnerId === runnerId)?.runnerName;
  // Queued work with no Mac connected: when it started waiting, and whether the owner has been alerted.
  let waitingSince: number | undefined, alerted = false;
  const queue = () => {
    let waiting = 0;
    try { waiting = app.queuedRuns(); } catch { /* the core is closing */ }
    const stranded = pool !== undefined && waiting > 0 && !pool.runners.some(id => runners.online(id));
    if (!stranded) { waitingSince = undefined; alerted = false; }
    else waitingSince ??= Date.now();
    return { waiting, ...(waitingSince === undefined ? {} : { waitingSince: new Date(waitingSince).toISOString() }), alert: waitingSince !== undefined && Date.now() - waitingSince >= queueAlertMs };
  };
  const waited = () => { const state = queue(); return state.alert && state.waitingSince ? `${state.waiting === 1 ? 'One run has' : state.waiting + ' runs have'} waited since ${state.waitingSince}.` : undefined; };
  const lane = (descriptor: RuntimeDescriptor) => pool && pins
    ? routePool(runners, pool, { binding: descriptor, pins, waitMs: () => offlineWaitMs, names, detail: waited })
    : routeByNode(runners, { waitMs: () => offlineWaitMs, binding: descriptor });
  let app: ReturnType<typeof createApplication>;
  try {
    // Agents keep their skills and memory on the hub: the context compiler reads only the ledger, never a Mac's files.
    app = createApplication({ dataDirectory, provider, context: LOCAL_CONTEXT, runtime: lane(provider), runtimes: (options.runtimes ?? []).map(descriptor => ({ provider: descriptor, runtime: lane(descriptor) })),
      autoRun: true, ownerPairingPolicy: { expiresAfterDays: null } });
  } catch (error) { ledger.close(); throw error; }
  const watch = pool ? setInterval(() => {
    const state = queue();
    if (state.alert && !alerted && state.waitingSince) { alerted = true; options.onQueueAlert?.({ waiting: state.waiting, waitingSince: state.waitingSince, runners: pool.runners }); }
  }, Math.min(30_000, queueAlertMs)) : undefined;
  watch?.unref();
  try { await app.start({ listener: { kind: 'loopback' } }); }
  catch (error) { clearInterval(watch); ledger.close(); await app.close().catch(() => {}); throw error; }
  const core = new URL(app.baseURL);

  const owner = (request: IncomingMessage) => {
    const token = /^Bearer ([A-Za-z0-9_-]{43})$/.exec(request.headers.authorization ?? '')?.[1];
    if (request.headers['x-fort-installation-id'] !== app.installation.installationId || !token || !app.ownerDevice(token)) fail(401, 'unauthorized', 'Use the owning Mac’s credential.');
  };
  // The enrolled Mac this hub serves sends its ledger snapshot with its runner token.
  const thisMac = (request: IncomingMessage) => {
    const token = /^Bearer ([A-Za-z0-9_-]{43})$/.exec(request.headers.authorization ?? '')?.[1];
    const runnerId = token ? enrollment.authenticate(token) : undefined;
    if (!runnerId) fail(401, 'unauthorized', 'Use this Mac’s runner token.');
    if (nodeOf(runnerId) !== provider.hostId) fail(403, 'wrongMac', 'Only the Mac this hub runs on can import its ledger.');
    return runnerId;
  };
  // Spec 063, revision 2026-10-31: a synced Mac hands its Agents to the hub, which owns them from then on.
  const runtimesServed = () => [provider, ...(options.runtimes ?? [])].map(descriptor => ({ provider: descriptor.provider, runtimeVersion: descriptor.runtimeVersion }));
  async function handoff(request: IncomingMessage) {
    thisMac(request);
    try { return app.adoptHandoff(await body(request, HANDOFF_BYTES)); }
    catch (error) {
      if (error instanceof HubError || (error && typeof error === 'object' && 'status' in error)) throw error;
      fail(422, 'handoffRefused', error instanceof Error ? error.message.slice(0, 300) : 'The handoff was refused.');
    }
  }
  async function stageImport(request: IncomingMessage) {
    thisMac(request);
    if (request.headers['content-type'] !== 'application/vnd.sqlite3') fail(415, 'invalidBody', 'Send the ledger snapshot as application/vnd.sqlite3.');
    if (existsSync(join(dataDirectory, RESULT)) && (JSON.parse(readFileSync(join(dataDirectory, RESULT), 'utf8')) as { state?: unknown }).state === 'applied') fail(409, 'alreadyImported', 'This hub already holds an imported ledger.');
    const chunks: Buffer[] = [];
    let size = 0;
    for await (const chunk of request as AsyncIterable<Buffer>) { size += chunk.length; if (size > SNAPSHOT_BYTES) fail(413, 'bodyTooLarge', 'The snapshot is too large.'); chunks.push(chunk); }
    const partial = join(dataDirectory, PENDING + '.partial');
    writeFileSync(partial, Buffer.concat(chunks), { mode: 0o600 });
    try { const inspected = inspectLedgerSnapshot(partial); renameSync(partial, join(dataDirectory, PENDING)); rmSync(join(dataDirectory, RESULT), { force: true }); return { state: 'staged', ...inspected }; }
    catch (error) { rmSync(partial, { force: true }); fail(422, 'importRefused', error instanceof Error ? error.message.slice(0, 300) : 'The snapshot was refused.'); }
  }
  function importStatus(request: IncomingMessage) {
    thisMac(request);
    if (existsSync(join(dataDirectory, PENDING))) return { state: 'staged' };
    return existsSync(join(dataDirectory, RESULT)) ? JSON.parse(readFileSync(join(dataDirectory, RESULT), 'utf8')) as unknown : { state: 'none' };
  }
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

  const answer = (response: ServerResponse, status: number, value: unknown, extra: Record<string, string> = {}) =>
    response.writeHead(status, { 'content-type': 'application/json', 'cache-control': 'no-store', ...extra }).end(JSON.stringify(value));
  // Google sign-in routes. A browser always names its origin on a POST, so these accept only the hub's own page.
  async function auth(request: IncomingMessage, response: ServerResponse, path: string): Promise<void> {
    // The page asks first: whether Google guards this hub, and the hub's one public address, so a
    // browser on an older address can carry its credential over.
    const where = publicOrigin ? { publicOrigin } : {};
    if (!google && request.method === 'GET' && path === '/auth/session') return void answer(response, 200, { google: false, ...where });
    if (!google) fail(404, 'notFound', 'Google sign-in is not enabled.');
    if (request.method === 'GET' && path === '/auth/session') {
      const session = signedIn(request);
      return void answer(response, 200, { google: true, ...where, clientId: google.clientId, ...(google.iosClientId ? { iosClientId: google.iosClientId } : {}), nonce: google.nonce(), signedIn: !!session, ...(session ? { email: session.email } : {}) });
    }
    // The iPhone app is no browser: it never names an origin, and gets its session value to keep.
    if (request.method === 'POST' && path === '/auth/google/native' && request.headers.origin === undefined && request.headers['sec-fetch-site'] === undefined) {
      const value = await body(request) as { credential?: unknown };
      const { session, value: token } = await google.signInNative(value?.credential);
      return void answer(response, 200, { signedIn: true, email: session.email, session: token, expiresAt: new Date(session.expiresAt).toISOString() });
    }
    if (request.method !== 'POST' || request.headers.origin !== publicOrigin || path === '/auth/google/native') fail(403, 'originDenied', 'Sign in from this hub’s own page.');
    if (path === '/auth/google') {
      const value = await body(request) as { credential?: unknown };
      const { session, cookie } = await google.signIn(value?.credential);
      return void answer(response, 200, { signedIn: true, email: session.email }, { 'set-cookie': cookie });
    }
    if (path === '/auth/signout') { request.resume(); return void answer(response, 200, { signedIn: false }, { 'set-cookie': google.signOutCookie }); }
    fail(404, 'notFound', 'No such sign-in route.');
  }
  // With the Google gate on, anything a browser can reach, and anything the owner's browser credential
  // can do, needs a Google session. Paired Macs keep their runner tokens; paired iPhones need Google
  // too once the iOS client is configured, and keep only their device token until then.
  const signedIn = (request: IncomingMessage) => google?.session(request.headers.cookie, request.headers['x-fort-session']);
  const needsGoogle = (request: IncomingMessage, path: string): boolean => {
    if (!google || path.startsWith('/v1/runner/') || MACHINE_PATHS.has(path) || signedIn(request)) return false;
    if (google.iosClientId) return true;
    if (request.headers.origin !== undefined || request.headers['sec-fetch-site'] !== undefined || request.headers.cookie !== undefined) return true;
    const token = /^Bearer ([A-Za-z0-9_-]{43})$/.exec(request.headers.authorization ?? '')?.[1];
    return !token || app.ownerDevice(token) !== undefined;
  };
  // An enrolled Mac's own client credential stands in for Google: the owner signed in to invite that Mac, and removing
  // the Mac ends it. Browsers never get one.
  const runnerClient = (request: IncomingMessage): boolean => {
    if (request.headers.origin !== undefined || request.headers['sec-fetch-site'] !== undefined || request.headers.cookie !== undefined) return false;
    const token = /^Bearer ([A-Za-z0-9_-]{43})$/.exec(request.headers.authorization ?? '')?.[1];
    const runnerId = token ? app.runnerClientOf(token) : undefined;
    return runnerId !== undefined && enrollment.list().some(runner => runner.runnerId === runnerId);
  };

  const server = createServer({ maxHeaderSize: 16 * 1024 }, (request, response) => {
    const path = new URL(request.url ?? '/', 'http://hub').pathname;
    if (path.startsWith('/auth/')) {
      void auth(request, response, path).catch((error: unknown) => {
        const known = error instanceof HubError || error instanceof SignInError ? error : undefined;
        if (!response.headersSent) answer(response, known?.status ?? 500, { error: { code: known?.code ?? 'hubError', message: known?.message ?? 'Sign-in could not be completed.' } }, { connection: 'close' });
        request.resume();
      });
      return;
    }
    const page = request.method === 'GET' || request.method === 'HEAD' ? web.get(path) : undefined;
    if (page && (!google || PUBLIC_PAGES.has(path) || signedIn(request))) {
      response.writeHead(200, { ...pageHeaders, 'content-type': page.type, 'content-length': page.data.length }).end(request.method === 'HEAD' ? undefined : page.data); return;
    }
    if (page || (needsGoogle(request, path) && !runnerClient(request))) {
      answer(response, 401, { error: { code: 'signInRequired', message: 'Sign in with Google first.' } }, { connection: 'close' });
      request.resume(); return;
    }
    // A browser names its page's origin. Only the hub's own web client may call the API, and the
    // origin is removed before the core sees the request, so the core's own browser refusal still
    // holds for every other origin.
    if (request.headers.origin !== undefined) {
      if (publicOrigin === undefined || request.headers.origin !== publicOrigin || path.startsWith('/v1/runner/')) {
        response.writeHead(403, { 'content-type': 'application/json', 'cache-control': 'no-store', connection: 'close' })
          .end(JSON.stringify({ error: { code: 'originDenied', message: 'Browser origins other than this hub are not enabled.' } }));
        request.resume(); return;
      }
      delete request.headers.origin;
    }
    if (runners.handle(request, response)) return;
    const local = async (): Promise<[number, unknown] | undefined> => {
      if (request.method === 'POST' && path === '/v1/hub/setup') return [201, await setup(request)];
      // The reply names the node the Mac serves, so a pooled Mac offers its lanes under the pool's ID.
      if (request.method === 'POST' && path === '/v1/runners/join') { const joined = enrollment.join(await body(request)); return [201, { ...joined, node: nodeOf(joined.runnerId) }]; }
      if (request.method === 'POST' && path === '/v1/runners/import') { const staged = await stageImport(request); response.once('finish', () => options.onImportStaged?.()); return [202, staged]; }
      if (request.method === 'GET' && path === '/v1/runners/import') return [200, importStatus(request)];
      if (request.method === 'GET' && path === '/v1/runners/agents') { thisMac(request); return [200, { node: provider.hostId, runtimes: runtimesServed() }]; }
      if (request.method === 'POST' && path === '/v1/runners/agents') return [200, await handoff(request)];
      if (request.method === 'POST' && path === '/v1/runners/client') { const runnerId = thisMac(request); request.resume(); return [201, app.issueRunnerClient(runnerId)]; }
      if (request.method === 'GET' && path === '/v1/runners') { owner(request); return [200, { runners: enrollment.list().map(runner => ({ ...runner, ...runners.status(runner.runnerId) })),
        ...(pool ? { pool: { node: pool.node, runners: pool.runners, queue: queue() } } : {}) }]; }
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
  catch (error) { clearInterval(watch); runners.close(); await app.close().catch(() => {}); ledger.close(); throw error; }
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Missing listener');
  let closing: Promise<void> | undefined;
  return {
    url: `http://${address.family === 'IPv6' ? `[${address.address}]` : address.address}:${address.port}`,
    installation: app.installation,
    close(): Promise<void> {
      closing ??= (async () => {
        clearInterval(watch);
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
  // FORT_RUNTIMES lists every runtime the Mac at FORT_NODE serves, primary first, as JSON:
  // [{"provider":"codex","name":"ChatGPT","runtimeVersion":"0.163.0","capabilities":["plainText","modelCatalog","cancel"]}, ...]
  const node = required('FORT_NODE');
  const listed = env.FORT_RUNTIMES ? JSON.parse(env.FORT_RUNTIMES) as unknown : [{ provider: required('FORT_PROVIDER'), name: env.FORT_PROVIDER_NAME, runtimeVersion: required('FORT_RUNTIME_VERSION'), capabilities: ['plainText', 'cancel'] }];
  if (!Array.isArray(listed) || listed.length === 0) throw new Error('Set FORT_RUNTIMES to a list of runtimes');
  const [provider, ...runtimes] = listed.map((row: { provider?: unknown; name?: unknown; runtimeVersion?: unknown; capabilities?: unknown }): RuntimeDescriptor => {
    if (typeof row?.provider !== 'string' || typeof row.runtimeVersion !== 'string' || (row.name !== undefined && typeof row.name !== 'string') || !Array.isArray(row.capabilities)) throw new Error('Set each runtime’s provider, runtimeVersion and capabilities');
    return { provider: row.provider, name: (row.name as string | undefined) ?? row.provider, runtimeVersion: row.runtimeVersion, hostId: node, session: 'stateless', capabilities: row.capabilities as RuntimeDescriptor['capabilities'] };
  });
  const start = () => startCloudHub({ dataDirectory: required('FORT_DATA'), port, provider: provider!, runtimes, onImportStaged: () => { void restart(); },
    ...(env.FORT_PUBLIC_ORIGIN ? { publicOrigin: env.FORT_PUBLIC_ORIGIN } : {}),
    ...(env.FORT_SETUP_CODE_SHA256 ? { setupCodeDigest: env.FORT_SETUP_CODE_SHA256 } : {}),
    // FORT_GOOGLE_CLIENT_ID (a public web client ID) and FORT_OWNER_EMAIL (comma-separated) turn on the Google gate.
    ...(env.FORT_GOOGLE_CLIENT_ID || env.FORT_OWNER_EMAIL ? { google: { clientId: required('FORT_GOOGLE_CLIENT_ID'), emails: required('FORT_OWNER_EMAIL').split(','),
      ...(env.FORT_GOOGLE_IOS_CLIENT_ID ? { iosClientId: env.FORT_GOOGLE_IOS_CLIENT_ID } : {}) } } : {}),
    // FORT_RUNNER_POOL lists the Macs that serve FORT_NODE, primary first, for example "toby-mini,toby-laptop".
    ...(env.FORT_RUNNER_POOL ? { pool: parseRunnerPool(node, env.FORT_RUNNER_POOL).runners,
      queueAlertMs: Number(env.FORT_QUEUE_ALERT_MINUTES ?? 15) * 60_000,
      onQueueAlert: (alert: QueueAlert) => process.stdout.write(`Fort hub: ${alert.waiting} queued run(s) have waited since ${alert.waitingSince}; none of ${alert.runners.join(', ')} is connected\n`) } : {}),
    offlineWaitMs: Number(env.FORT_OFFLINE_WAIT_MS ?? 0) });
  let hub = await start();
  // A staged ledger import is applied on start, so the hub closes and starts again in this process.
  async function restart() { await hub.close(); hub = await start(); process.stdout.write('Fort hub restarted to apply a ledger import\n'); }
  process.stdout.write(`Fort hub listening on port ${port}\n`);
  const stop = () => { void hub.close().finally(() => process.exit(0)); };
  process.once('SIGTERM', stop); process.once('SIGINT', stop);
}
