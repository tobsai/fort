import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, stat } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createHash, generateKeyPairSync, randomBytes, randomUUID, sign } from 'node:crypto';
import type { RuntimeDescriptor } from '../contracts/v1.ts';
import { startCloudHub } from './cloud-hub.ts';

// Spec 066: with the Google gate on, a browser gets nothing of Fort's until the owner signs in with
// Google, and the owner's browser credential is useless without that session. Machines keep their tokens.
const secret = () => randomBytes(32).toString('base64url');
const origin = 'https://fort.example.com', clientId = '1234-abc.apps.googleusercontent.com';
const codex: RuntimeDescriptor = { provider: 'codex', name: 'ChatGPT', runtimeVersion: '0.163.0', hostId: 'tobys-mac', session: 'stateless', capabilities: ['plainText'] };
const setupCode = secret(), setupCodeDigest = createHash('sha256').update(setupCode).digest('hex');
const google = generateKeyPairSync('rsa', { modulusLength: 2048 }), stranger = generateKeyPairSync('rsa', { modulusLength: 2048 });
const jwk = { ...google.publicKey.export({ format: 'jwk' }), kid: 'k1', alg: 'RS256', use: 'sig' };
const b64 = (value: object) => Buffer.from(JSON.stringify(value)).toString('base64url');

function idToken(claims: Record<string, unknown>, key = google.privateKey, kid = 'k1') {
  const now = Math.floor(Date.now() / 1000);
  const body = b64({ alg: 'RS256', kid, typ: 'JWT' }) + '.' + b64({ iss: 'https://accounts.google.com', aud: clientId, sub: '1001', email: 'toby@example.com', email_verified: true, iat: now, exp: now + 3600, ...claims });
  return body + '.' + sign('RSA-SHA256', Buffer.from(body), key).toString('base64url');
}

async function fixture(t: TestContext) {
  const dataDirectory = await mkdtemp(join(tmpdir(), 'fort-cloud-google-'));
  const hub = await startCloudHub({ dataDirectory, host: '127.0.0.1', port: 0, provider: codex, setupCodeDigest, publicOrigin: origin,
    google: { clientId, emails: ['Toby@example.com'], keys: async () => [jwk] } });
  t.after(async () => { await hub.close(); await rm(dataDirectory, { recursive: true, force: true }); });
  const owner = { deviceId: randomUUID(), deviceToken: secret() };
  let cookie = '';
  const call = async (path: string, init: { body?: unknown; headers?: Record<string, string>; method?: string } = {}) => {
    const response = await fetch(hub.url + path, { method: init.method ?? (init.body === undefined ? 'GET' : 'POST'),
      headers: { ...(init.body === undefined ? {} : { 'content-type': 'application/json' }), ...init.headers }, body: init.body === undefined ? undefined : JSON.stringify(init.body) });
    const text = await response.text();
    let value: Record<string, any> = {}; try { value = JSON.parse(text); } catch { /* a page */ }
    return { status: response.status, body: value, text, headers: response.headers };
  };
  // What the owner's browser sends: its origin, its Google session, and its device credential.
  const browser = (path: string, body?: unknown, extra: Record<string, string> = {}) => call(path, { body,
    headers: { origin, 'sec-fetch-site': 'same-origin', ...(cookie ? { cookie } : {}), authorization: 'Bearer ' + owner.deviceToken, 'x-fort-installation-id': hub.installation.installationId, ...extra } });
  const nonce = async () => (await call('/auth/session')).body.nonce as string;
  const signIn = async (claims: Record<string, unknown> = {}, key = google.privateKey) => {
    const result = await call('/auth/google', { body: { credential: idToken({ nonce: await nonce(), ...claims }, key) }, headers: { origin } });
    if (result.status === 200) cookie = result.headers.get('set-cookie')!.split(';')[0]!;
    return result;
  };
  return { hub, dataDirectory, owner, call, browser, nonce, signIn, idToken, cookie: () => cookie, setCookie: (value: string) => { cookie = value; } };
}

test('before Google sign-in a browser gets only the sign-in page, and nothing else answers', async t => {
  const h = await fixture(t);
  assert.equal((await h.call('/')).status, 200);
  const signin = await h.call('/signin.js');
  assert.equal(signin.status, 200);
  assert.match(signin.headers.get('content-security-policy')!, /script-src 'self' https:\/\/accounts\.google\.com\/gsi\/client;/);
  assert.equal((await h.call('/app.js')).status, 401, 'the app itself waits for sign-in');
  const session = await h.call('/auth/session');
  assert.deepEqual([session.body.google, session.body.signedIn, session.body.clientId], [true, false, clientId]);
  for (const path of ['/v1/installation', '/v1/agents', '/v1/runners']) assert.equal((await h.browser(path)).body.error.code, 'signInRequired', path);
  assert.equal((await h.browser('/v1/hub/setup', { setupCode, ...h.owner })).body.error.code, 'signInRequired', 'claiming the hub needs Google too');
  assert.equal((await h.call('/v1/hub/setup', { body: { setupCode, ...h.owner } })).body.error.code, 'signInRequired', 'even without a browser');
});

test('the owner signs in with Google, claims the hub and uses it; the credential alone gets nothing', async t => {
  const h = await fixture(t);
  const result = await h.signIn();
  assert.equal(result.status, 200);
  assert.equal(result.body.email, 'toby@example.com');
  assert.match(result.headers.get('set-cookie')!, /^__Host-fort_session=[^;]+; Path=\/; HttpOnly; Secure; SameSite=Strict; Max-Age=\d+$/);
  assert.equal((await h.call('/auth/session', { headers: { cookie: h.cookie() } })).body.signedIn, true);
  assert.equal((await h.call('/app.js', { headers: { cookie: h.cookie() } })).status, 200);
  assert.equal((await h.browser('/v1/hub/setup', { setupCode, ...h.owner })).status, 201);
  assert.equal((await h.browser('/v1/agents')).status, 200);
  assert.equal((await h.browser('/v1/runners')).status, 200);
  // A copied credential, from a script or another browser, is refused without the session.
  const copied = { authorization: 'Bearer ' + h.owner.deviceToken, 'x-fort-installation-id': h.hub.installation.installationId };
  assert.equal((await h.call('/v1/agents', { headers: copied })).body.error.code, 'signInRequired');
  assert.equal((await h.call('/v1/agents', { headers: { ...copied, cookie: h.cookie() + 'x' } })).body.error.code, 'signInRequired', 'a tampered cookie is no session');
  // Signing out ends it.
  const out = await h.call('/auth/signout', { method: 'POST', headers: { origin } });
  assert.match(out.headers.get('set-cookie')!, /Max-Age=0/);
  assert.equal((await stat(join(h.dataDirectory, 'web-session.key'))).mode & 0o777, 0o600);
});

test('other accounts, forged, stale, replayed or foreign sign-ins are refused', async t => {
  const h = await fixture(t);
  assert.equal((await h.signIn({ email: 'someone@example.com' })).status, 401);
  assert.equal((await h.signIn({ email_verified: false })).status, 401);
  assert.equal((await h.signIn({ aud: 'other.apps.googleusercontent.com' })).status, 401);
  assert.equal((await h.signIn({ iss: 'https://evil.example.com' })).status, 401);
  assert.equal((await h.signIn({ exp: Math.floor(Date.now() / 1000) - 600 })).status, 401);
  assert.equal((await h.signIn({}, stranger.privateKey)).status, 401, 'not signed by Google');
  assert.equal((await h.signIn({ nonce: 'made-up' })).status, 401, 'a nonce the hub never issued');
  const replay = { credential: h.idToken({ nonce: await h.nonce() }) };
  assert.equal((await h.call('/auth/google', { body: replay, headers: { origin } })).status, 200);
  assert.equal((await h.call('/auth/google', { body: replay, headers: { origin } })).status, 401, 'a sign-in works once');
  assert.equal((await h.call('/auth/google', { body: { credential: h.idToken({ nonce: await h.nonce() }) }, headers: { origin: 'https://evil.example.com' } })).status, 403);
  assert.equal((await h.call('/auth/google', { body: { credential: h.idToken({ nonce: await h.nonce() }) } })).status, 403, 'only from the hub’s own page');
  // The first Google account in is pinned: the same email under another Google account ID is refused.
  assert.equal((await h.signIn({ sub: '2002' })).status, 401);
});

test('paired machines keep their own credentials and never meet the Google gate', async t => {
  const h = await fixture(t);
  await h.signIn();
  assert.equal((await h.browser('/v1/hub/setup', { setupCode, ...h.owner })).status, 201);
  const invitation = { invitationId: randomUUID(), code: secret() };
  assert.equal((await h.browser('/v1/runners/invitation', invitation)).status, 201);
  const joined = await h.call('/v1/runners/join', { body: { ...invitation, runnerId: 'tobys-mac', runnerToken: secret(), runnerName: 'Toby’s Mac' } });
  assert.equal(joined.status, 201, 'the Mac joins with its one-time invitation, no Google');
  assert.notEqual((await h.call('/v1/runner/connect', { method: 'POST', headers: { authorization: 'Bearer ' + secret() } })).body?.error?.code, 'signInRequired');
  // A non-owner bearer (a paired iPhone) goes on to the core, which checks it as before.
  const phone = await h.call('/v1/agents', { headers: { authorization: 'Bearer ' + secret(), 'x-fort-installation-id': h.hub.installation.installationId } });
  assert.equal(phone.status, 401);
  assert.notEqual(phone.body.error.code, 'signInRequired');
});

test('the gate needs a public origin and a Google web client ID', async t => {
  const dataDirectory = await mkdtemp(join(tmpdir(), 'fort-cloud-google-'));
  t.after(() => rm(dataDirectory, { recursive: true, force: true }));
  await assert.rejects(startCloudHub({ dataDirectory, host: '127.0.0.1', port: 0, provider: codex, google: { clientId, emails: ['toby@example.com'] } }), /public origin/);
  await assert.rejects(startCloudHub({ dataDirectory, host: '127.0.0.1', port: 0, provider: codex, publicOrigin: origin, google: { clientId: 'nope', emails: ['toby@example.com'] } }), /client ID/);
});
