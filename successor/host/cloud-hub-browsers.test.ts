import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createHash, generateKeyPairSync, randomBytes, randomUUID, sign } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import type { RuntimeDescriptor } from '../contracts/v1.ts';
import { startCloudHub } from './cloud-hub.ts';

// Spec 066, revision 2026-11-02: with the Google gate on, any browser that signs in with Google as the owner
// gets its own key. No setup code, nothing carried over from the owner browser, and each one can be removed.
const secret = () => randomBytes(32).toString('base64url');
const origin = 'https://fort.example.com', clientId = '1234-abc.apps.googleusercontent.com';
const codex: RuntimeDescriptor = { provider: 'codex', name: 'ChatGPT', runtimeVersion: '0.163.0', hostId: 'tobys-mac', session: 'stateless', capabilities: ['plainText'] };
const setupCode = secret(), setupCodeDigest = createHash('sha256').update(setupCode).digest('hex');
const google = generateKeyPairSync('rsa', { modulusLength: 2048 });
const jwk = { ...google.publicKey.export({ format: 'jwk' }), kid: 'k1', alg: 'RS256', use: 'sig' };
const b64 = (value: object) => Buffer.from(JSON.stringify(value)).toString('base64url');
function idToken(claims: Record<string, unknown>) {
  const now = Math.floor(Date.now() / 1000);
  const body = b64({ alg: 'RS256', kid: 'k1', typ: 'JWT' }) + '.' + b64({ iss: 'https://accounts.google.com', aud: clientId, sub: '1001', email: 'toby@example.com', email_verified: true, iat: now, exp: now + 3600, ...claims });
  return body + '.' + sign('RSA-SHA256', Buffer.from(body), google.privateKey).toString('base64url');
}

type Reply = { status: number; body: Record<string, any> };
async function fixture(t: TestContext, { gate = true } = {}) {
  const dataDirectory = await mkdtemp(join(tmpdir(), 'fort-cloud-browsers-'));
  const hub = await startCloudHub({ dataDirectory, host: '127.0.0.1', port: 0, provider: codex, setupCodeDigest, publicOrigin: origin,
    ...(gate ? { google: { clientId, emails: ['toby@example.com'], keys: async () => [jwk] } } : {}) });
  t.after(async () => { await hub.close(); await rm(dataDirectory, { recursive: true, force: true }); });
  const call = async (path: string, init: { body?: unknown; headers?: Record<string, string> } = {}): Promise<Reply & { cookie?: string }> => {
    const response = await fetch(hub.url + path, { method: init.body === undefined ? 'GET' : 'POST',
      headers: { ...(init.body === undefined ? {} : { 'content-type': 'application/json' }), ...init.headers }, body: init.body === undefined ? undefined : JSON.stringify(init.body) });
    const text = await response.text();
    let body: Record<string, any> = {}; try { body = JSON.parse(text); } catch { /* a page */ }
    const cookie = response.headers.get('set-cookie')?.split(';')[0];
    return { status: response.status, body, ...(cookie ? { cookie } : {}) };
  };
  // A browser: its own Google session and, once it has one, its own key.
  const browser = () => {
    const self = { cookie: '', token: '', installationId: '' };
    const send = (path: string, body?: unknown, extra: Record<string, string> = {}) => call(path, { body, headers: { origin, 'sec-fetch-site': 'same-origin',
      ...(self.cookie ? { cookie: self.cookie } : {}), ...(self.token ? { authorization: 'Bearer ' + self.token } : {}), 'x-fort-installation-id': self.installationId, ...extra } });
    const signIn = async () => {
      const nonce = (await call('/auth/session')).body.nonce as string;
      const result = await call('/auth/google', { body: { credential: idToken({ nonce }) }, headers: { origin } });
      assert.equal(result.status, 200); self.cookie = result.cookie!;
    };
    const join = async (name: unknown = 'Safari on iPhone') => {
      const result = await send('/v1/hub/browsers', { name });
      if (result.status === 201) { self.token = result.body.token; self.installationId = result.body.installationId; }
      return result;
    };
    return Object.assign(self, { send, signIn, join });
  };
  const owner = browser();
  const claim = async () => {
    await owner.signIn();
    const device = { deviceId: randomUUID(), deviceToken: secret() };
    const result = await owner.send('/v1/hub/setup', { setupCode, ...device });
    assert.equal(result.status, 201);
    owner.token = device.deviceToken; owner.installationId = result.body.installationId;
  };
  return { hub, call, browser, owner, claim };
}

test('a second browser signs in with Google and gets its own key; the owner browser keeps working', async t => {
  const h = await fixture(t);
  await h.claim();
  const safari = h.browser();
  await safari.signIn();
  const joined = await safari.join();
  assert.equal(joined.status, 201);
  assert.equal(joined.body.installationId, h.hub.installation.installationId);
  assert.match(joined.body.token, /^[A-Za-z0-9_-]{43}$/);
  assert.notEqual(joined.body.token, h.owner.token, 'nothing is carried over from the owner');
  for (const path of ['/v1/installation', '/v1/agents', '/v1/conversations', '/v1/runners']) assert.equal((await safari.send(path)).status, 200, path);
  const mine = await safari.send('/v1/hub/browsers');
  assert.equal(mine.body.you, 'browser');
  assert.deepEqual(mine.body.browsers.map((entry: any) => [entry.name, entry.email, entry.current]), [['Safari on iPhone', 'toby@example.com', true]]);
  assert.equal(mine.body.browsers[0].hash, undefined, 'keys never leave the hub');
  // The owner browser is untouched, and sees the new browser.
  for (const path of ['/v1/installation', '/v1/agents', '/v1/runners']) assert.equal((await h.owner.send(path)).status, 200, path);
  const theirs = await h.owner.send('/v1/hub/browsers');
  assert.deepEqual([theirs.body.google, theirs.body.you, theirs.body.browsers.map((entry: any) => entry.current)], [true, 'owner', [false]]);
  // A third browser gets a third key.
  const laptop = h.browser(); await laptop.signIn();
  assert.equal((await laptop.join('Firefox on Mac')).status, 201);
  assert.equal((await h.owner.send('/v1/hub/browsers')).body.browsers.length, 2);
});

test('an added browser stays out of what only the owner does', async t => {
  const h = await fixture(t);
  await h.claim();
  const safari = h.browser(); await safari.signIn(); await safari.join();
  assert.equal((await safari.send('/v1/account/sign-in?provider=codex')).body.error.code, 'hostOnly');
  assert.equal((await safari.send('/v1/pairing/invitation', { invitationId: randomUUID(), code: secret() })).status >= 400, true);
  assert.equal((await safari.send('/v1/hub/setup', { setupCode, deviceId: randomUUID(), deviceToken: secret() })).body.error.code, 'alreadyPaired');
});

test('a key comes only from a fresh Google sign-in on the hub’s own page, and never without Google', async t => {
  const h = await fixture(t);
  const early = h.browser(); await early.signIn();
  assert.equal((await early.join()).body.error.code, 'notClaimed', 'an unclaimed hub still takes its setup code first');
  await h.claim();
  const safari = h.browser();
  assert.equal((await safari.join()).body.error.code, 'signInRequired', 'no Google session, no key');
  await safari.signIn();
  assert.equal((await call(h, '/v1/hub/browsers', { name: 'x' }, { cookie: safari.cookie })).body.error.code, 'originDenied', 'a script without the page’s origin');
  assert.equal((await call(h, '/v1/hub/browsers', { name: 'x' }, { cookie: safari.cookie, origin: 'https://evil.example.com' })).body.error.code, 'originDenied');
  assert.equal((await safari.send('/v1/hub/browsers', ['x'])).status, 400);
  assert.equal((await safari.send('/v1/hub/browsers', { name: 'x', admin: true })).body.error.code, 'invalidBrowser');
  // A session cookie that is days old does not mint keys: the page signs in again first.
  const real = Date.now;
  Date.now = () => real() + 11 * 60_000;
  try { assert.equal((await safari.join()).body.error.code, 'signInAgain'); } finally { Date.now = real; }
  // Names are plain and short; a missing one is just "Browser".
  assert.equal((await safari.join('Chrome\u0000 on\nMac' + 'x'.repeat(100))).status, 201);
  const other = h.browser(); await other.signIn(); assert.equal((await other.join(42)).status, 201);
  const names = (await h.owner.send('/v1/hub/browsers')).body.browsers.map((entry: any) => entry.name);
  assert.deepEqual(names, ['Chrome onMac' + 'x'.repeat(48), 'Browser']);
  // The key alone, copied out of the browser, is no use without the Google session.
  const copied = { authorization: 'Bearer ' + safari.token, 'x-fort-installation-id': h.hub.installation.installationId };
  assert.equal((await h.call('/v1/agents', { headers: copied })).body.error.code, 'signInRequired');
});

test('a hub without the Google gate adds no browsers', async t => {
  const h = await fixture(t, { gate: false });
  const device = { deviceId: randomUUID(), deviceToken: secret() };
  const claimed = await h.call('/v1/hub/setup', { body: { setupCode, ...device }, headers: { origin } });
  assert.equal(claimed.status, 201);
  assert.equal((await h.call('/v1/hub/browsers', { body: { name: 'x' }, headers: { origin } })).status, 404);
  const listed = await h.call('/v1/hub/browsers', { headers: { origin, authorization: 'Bearer ' + device.deviceToken, 'x-fort-installation-id': claimed.body.installationId } });
  assert.deepEqual([listed.status, listed.body.google, listed.body.you, listed.body.browsers], [200, false, 'owner', []]);
});

test('removing a browser ends its key, and getting back in takes a new Google sign-in', async t => {
  const h = await fixture(t);
  await h.claim();
  const safari = h.browser(); await safari.signIn(); await safari.join();
  const firefox = h.browser(); await firefox.signIn(); await firefox.join('Firefox on Mac');
  const listed = (await h.owner.send('/v1/hub/browsers')).body.browsers as { deviceId: string; name: string }[];
  const target = listed.find(entry => entry.name === 'Safari on iPhone')!;
  assert.equal((await h.owner.send('/v1/hub/browsers/remove', { deviceId: target.deviceId })).status, 200);
  assert.equal((await safari.send('/v1/agents')).status, 401, 'its key is gone');
  assert.equal((await safari.send('/v1/hub/browsers')).status, 401);
  assert.equal((await firefox.send('/v1/agents')).status, 200, 'other browsers are untouched');
  assert.equal((await h.owner.send('/v1/agents')).status, 200);
  // Its Google session is from before the removal, so it cannot mint a new key with it.
  assert.equal((await safari.join()).body.error.code, 'signInAgain');
  await delay(5); await safari.signIn();
  assert.equal((await safari.join()).status, 201);
  assert.equal((await safari.send('/v1/agents')).status, 200);
  // A browser can forget itself; nobody can remove the owner or an unknown browser this way.
  const self = (await firefox.send('/v1/hub/browsers')).body.deviceId as string;
  assert.equal((await firefox.send('/v1/hub/browsers/remove', { deviceId: self })).status, 200);
  assert.equal((await firefox.send('/v1/agents')).status, 401);
  assert.equal((await safari.send('/v1/hub/browsers/remove', { deviceId: randomUUID() })).status, 404);
  assert.equal((await safari.send('/v1/hub/browsers/remove', { deviceId: 'constructor' })).status, 404);
  assert.equal((await h.owner.send('/v1/agents')).status, 200, 'the owner key is never touched');
});

test('a hub holds at most ten added browsers', async t => {
  const h = await fixture(t);
  await h.claim();
  for (let index = 0; index < 10; index++) { const next = h.browser(); await next.signIn(); assert.equal((await next.join('B' + index)).status, 201); }
  const extra = h.browser(); await extra.signIn();
  assert.equal((await extra.join()).body.error.code, 'tooManyBrowsers');
});

function call(h: Awaited<ReturnType<typeof fixture>>, path: string, body: unknown, headers: Record<string, string>) {
  return h.call(path, { body, headers });
}
