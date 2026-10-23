import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createHash, createSign, generateKeyPairSync } from 'node:crypto';
import { prepareSignIn, completeSignIn, refreshPlanCredential, listenForCallback, discoveredSigningKeys, fetchTokenTransport, SignInRequired, PlanUsageNotGranted, type TokenTransport } from './openai-plan-account.ts';

const { privateKey, publicKey } = generateKeyPairSync('rsa', { modulusLength: 2048 });
const jwk = { ...publicKey.export({ format: 'jwk' }), kid: 'key-1', alg: 'RS256', use: 'sig' };
const keys = async (kid: string) => kid === 'key-1' ? jwk : undefined;
const now = () => 1_800_000_000_000;
const b64 = (value: unknown) => Buffer.from(JSON.stringify(value)).toString('base64url');
const idToken = (claims: Record<string, unknown>, kid = 'key-1') => {
  const head = b64({ alg: 'RS256', kid, typ: 'JWT' }) + '.' + b64(claims);
  return head + '.' + createSign('RSA-SHA256').update(head).sign(privateKey).toString('base64url');
};
const hostId = 'urn:uuid:00000000-0000-4000-8000-000000000000';
const scope = 'chatgpt.tokens.use.direct email offline_access openid profile resource.invoke';
const tokenTransport = (respond: (form: URLSearchParams) => { status: number; body: unknown }) => {
  const calls: { url: string; form: URLSearchParams }[] = [];
  const transport: TokenTransport = { async postForm(url, form) { calls.push({ url, form }); return respond(form); } };
  return { transport, calls };
};

test('first sign-in registers through the dynamic client with PKCE, state, nonce and the plan-usage scopes', () => {
  const pending = prepareSignIn({ hostId, agentName: 'Fort', redirectURI: 'http://127.0.0.1:51234/auth/callback' });
  const url = new URL(pending.authorizationURL);
  assert.equal(url.origin + url.pathname, 'https://auth.openai.com/api/accounts/authorize');
  const q = url.searchParams;
  assert.equal(q.get('client_id'), 'dynamic_agent_client');
  assert.equal(q.get('agent_name_hint'), 'Fort');
  assert.equal(q.get('ext_agent_host_id'), hostId);
  assert.equal(q.get('response_type'), 'code');
  assert.equal(q.get('scope'), 'openid profile email offline_access resource.invoke chatgpt.tokens.use.direct');
  assert.equal(q.get('resource'), 'https://api.openai.com/v1');
  assert.equal(q.get('code_challenge_method'), 'S256');
  assert.equal(q.get('code_challenge'), createHash('sha256').update(pending.verifier).digest('base64url'));
  assert.equal(q.get('state'), pending.state);
  assert.equal(q.get('nonce'), pending.nonce);
  assert.notEqual(prepareSignIn({ hostId, agentName: 'Fort', redirectURI: pending.redirectURI }).state, pending.state);
  const again = new URL(prepareSignIn({ hostId, agentName: 'Fort', redirectURI: pending.redirectURI, clientId: 'oaiapp_issued' }).authorizationURL).searchParams;
  assert.equal(again.get('client_id'), 'oaiapp_issued');
  assert.equal(again.has('agent_name_hint'), false);
  assert.throws(() => prepareSignIn({ hostId, agentName: 'Fort', redirectURI: 'http://localhost:1455/auth/callback' }), /loopback/);
  assert.throws(() => prepareSignIn({ hostId, agentName: 'Fort', redirectURI: 'http://127.0.0.1:1455/callback' }), /loopback/);
  assert.throws(() => prepareSignIn({ hostId: 'user@example.com', agentName: 'Fort', redirectURI: pending.redirectURI }), /host/);
});

test('a valid callback exchanges the code and yields a verified plan credential', async () => {
  const pending = prepareSignIn({ hostId, agentName: 'Fort', redirectURI: 'http://127.0.0.1:51234/auth/callback' });
  const { transport, calls } = tokenTransport(() => ({ status: 200, body: { access_token: 'access-1', refresh_token: 'refresh-1', token_type: 'Bearer', expires_in: 3600, scope, id_token: idToken({ iss: 'https://auth.openai.com', aud: 'oaiapp_issued', sub: 'user-1', exp: now() / 1000 + 600, nonce: pending.nonce, email: 'owner@example.com' }) } }));
  const callback = new URLSearchParams({ code: 'code-1', state: pending.state, scope, client_id: 'oaiapp_issued' });
  const credential = await completeSignIn({ pending, callback, transport, keys, now });
  assert.equal(calls[0]?.url, 'https://auth.openai.com/api/accounts/oauth/token');
  assert.deepEqual(Object.fromEntries(calls[0]!.form), { grant_type: 'authorization_code', client_id: 'oaiapp_issued', code: 'code-1', code_verifier: pending.verifier, redirect_uri: pending.redirectURI, resource: 'https://api.openai.com/v1' });
  assert.deepEqual(credential, { issuer: 'https://auth.openai.com', subject: 'user-1', clientId: 'oaiapp_issued', hostId, idToken: credential.idToken, accessToken: 'access-1', refreshToken: 'refresh-1', expiresAt: now() + 3_600_000, scopes: scope.split(' '), email: 'owner@example.com' });
});

test('state, nonce, signature, issuer, audience, expiry and scope failures never yield a credential', async () => {
  const pending = prepareSignIn({ hostId, agentName: 'Fort', redirectURI: 'http://127.0.0.1:51234/auth/callback' });
  const claims = { iss: 'https://auth.openai.com', aud: 'oaiapp_issued', sub: 'user-1', exp: now() / 1000 + 600, nonce: pending.nonce };
  const attempt = (token: string, extra: Record<string, string> = {}, grantedScope = scope) => completeSignIn({ pending, now, keys,
    callback: new URLSearchParams({ code: 'code-1', state: pending.state, client_id: 'oaiapp_issued', ...extra }),
    transport: tokenTransport(() => ({ status: 200, body: { access_token: 'a', refresh_token: 'r', token_type: 'Bearer', expires_in: 3600, scope: grantedScope, id_token: token } })).transport });
  await assert.rejects(attempt(idToken(claims), { state: 'other' }), /state/);
  await assert.rejects(attempt(idToken(claims), { error: 'access_denied' }), /declined/);
  await assert.rejects(attempt(idToken({ ...claims, nonce: 'other' })), /identity/);
  await assert.rejects(attempt(idToken({ ...claims, iss: 'https://example.com' })), /identity/);
  await assert.rejects(attempt(idToken({ ...claims, aud: 'someone-else' })), /identity/);
  await assert.rejects(attempt(idToken({ ...claims, exp: now() / 1000 - 1 })), /identity/);
  await assert.rejects(attempt(idToken(claims, 'unknown-key')), /identity/);
  const forged = idToken(claims).split('.'); forged[1] = b64({ ...claims, sub: 'attacker' });
  await assert.rejects(attempt(forged.join('.')), /identity/);
  await assert.rejects(attempt(idToken(claims), {}, 'openid profile email offline_access'), PlanUsageNotGranted);
});

test('refresh replaces the whole token set and terminal refresh errors require sign-in again', async () => {
  const saved = { issuer: 'https://auth.openai.com', subject: 'user-1', clientId: 'oaiapp_issued', hostId, idToken: 'id-old', accessToken: 'access-old', refreshToken: 'refresh-old', expiresAt: now() - 1, scopes: scope.split(' ') };
  const { transport, calls } = tokenTransport(() => ({ status: 200, body: { access_token: 'access-new', refresh_token: 'refresh-new', token_type: 'Bearer', expires_in: 3600, scope } }));
  const next = await refreshPlanCredential({ credential: saved, transport, now });
  assert.deepEqual(Object.fromEntries(calls[0]!.form), { grant_type: 'refresh_token', client_id: 'oaiapp_issued', refresh_token: 'refresh-old', resource: 'https://api.openai.com/v1' });
  assert.deepEqual(next, { ...saved, accessToken: 'access-new', refreshToken: 'refresh-new', expiresAt: now() + 3_600_000 });
  for (const code of ['invalid_grant', 'refresh_token_reused', 'refresh_token_expired'])
    await assert.rejects(refreshPlanCredential({ credential: saved, now, transport: tokenTransport(() => ({ status: 400, body: { error: code } })).transport }), SignInRequired);
  await assert.rejects(refreshPlanCredential({ credential: saved, now, transport: tokenTransport(() => ({ status: 503, body: {} })).transport }), error => !(error instanceof SignInRequired));
  await assert.rejects(refreshPlanCredential({ credential: saved, now, transport: tokenTransport(() => ({ status: 200, body: { access_token: 'a', token_type: 'Bearer', expires_in: 3600, scope: 'openid' } })).transport }), PlanUsageNotGranted);
});

test('the loopback listener accepts one callback on 127.0.0.1 at /auth/callback and then closes', async () => {
  const listener = await listenForCallback(AbortSignal.timeout(5000));
  assert.match(listener.redirectURI, /^http:\/\/127\.0\.0\.1:\d+\/auth\/callback$/);
  const wrong = await fetch(listener.redirectURI.replace('/auth/callback', '/callback'));
  assert.equal(wrong.status, 404);
  const response = await fetch(listener.redirectURI + '?code=c&state=s');
  assert.equal(response.status, 200);
  const params = await listener.callback;
  assert.equal(params.get('code'), 'c');
  await assert.rejects(fetch(listener.redirectURI));
  const aborted = new AbortController();
  const second = await listenForCallback(aborted.signal);
  aborted.abort();
  await assert.rejects(second.callback);
});

test('signing keys come from the issuer discovery document and stay on the issuer origin', async () => {
  const requested: string[] = [];
  const serve = (jwksURI: string) => async (url: string) => {
    requested.push(url);
    if (url === 'https://auth.openai.com/.well-known/openid-configuration') return new Response(JSON.stringify({ issuer: 'https://auth.openai.com', jwks_uri: jwksURI }));
    if (url === jwksURI) return new Response(JSON.stringify({ keys: [jwk, { kty: 'EC', kid: 'ec-1' }] }));
    return new Response('', { status: 404 });
  };
  const keysFound = discoveredSigningKeys(serve('https://auth.openai.com/jwks'));
  assert.deepEqual(await keysFound('key-1'), jwk);
  assert.deepEqual(await keysFound('key-1'), jwk);
  assert.equal(requested.length, 2);
  assert.equal(await keysFound('ec-1'), undefined);
  await assert.rejects(discoveredSigningKeys(serve('https://keys.example.invalid/jwks'))('key-1'));
});

test('the token transport posts one form to the exact token endpoint', async () => {
  const seen: { url: string; init?: RequestInit }[] = [];
  const transport = fetchTokenTransport(async (url, init) => { seen.push({ url, ...(init ? { init } : {}) }); return new Response(JSON.stringify({ error: 'invalid_grant' }), { status: 400 }); });
  assert.deepEqual(await transport.postForm('https://auth.openai.com/api/accounts/oauth/token', new URLSearchParams({ grant_type: 'refresh_token' })), { status: 400, body: { error: 'invalid_grant' } });
  assert.equal(seen[0]?.init?.method, 'POST'); assert.equal(seen[0]?.init?.body, 'grant_type=refresh_token');
  assert.equal(new Headers(seen[0]?.init?.headers).get('content-type'), 'application/x-www-form-urlencoded');
  await assert.rejects(transport.postForm('https://elsewhere.example.invalid/token', new URLSearchParams()));
  assert.equal(seen.length, 1);
});
