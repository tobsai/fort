import assert from 'node:assert/strict';
import test, { type TestContext } from 'node:test';
import { createServer } from 'node:http';
import { createSign, generateKeyPairSync } from 'node:crypto';
import { mkdtempSync, mkdirSync, realpathSync, rmSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createOpenAIPlanExecutorProcess, serveOpenAIPlanExecutor } from './openai-plan-executor.ts';
import { readPlanCredential } from './openai-plan-credential.ts';
import { PassThrough } from 'node:stream';

const { privateKey, publicKey } = generateKeyPairSync('rsa', { modulusLength: 2048 });
const jwk = { ...publicKey.export({ format: 'jwk' }), kid: 'key-1', alg: 'RS256', use: 'sig' };
const b64 = (value: unknown) => Buffer.from(JSON.stringify(value)).toString('base64url');
const signed = (claims: Record<string, unknown>) => { const head = b64({ alg: 'RS256', kid: 'key-1' }) + '.' + b64(claims); return head + '.' + createSign('RSA-SHA256').update(head).sign(privateKey).toString('base64url'); };

async function setup(t: TestContext) {
  const directory = realpathSync(mkdtempSync('/tmp/fort059-plan-executor-')), state = join(directory, 'state');
  mkdirSync(state, { mode: 0o700 });
  let nonce = '';
  const seen: string[] = [];
  const server = createServer((request, response) => {
    seen.push(request.method + ' ' + request.url);
    const json = (value: unknown) => response.writeHead(200, { 'content-type': 'application/json' }).end(JSON.stringify(value));
    if (request.url === '/auth.openai.com/.well-known/openid-configuration') return json({ issuer: 'https://auth.openai.com', jwks_uri: 'https://auth.openai.com/jwks' });
    if (request.url === '/auth.openai.com/jwks') return json({ keys: [jwk] });
    if (request.url === '/auth.openai.com/api/accounts/oauth/token') return json({ access_token: 'access-issued', refresh_token: 'refresh-issued', token_type: 'Bearer', expires_in: 3600, scope: 'chatgpt.tokens.use.direct offline_access openid',
      id_token: signed({ iss: 'https://auth.openai.com', aud: 'oaiapp_issued', sub: 'user-synthetic', email: 'owner@example.com', nonce, exp: Math.floor(Date.now() / 1000) + 3600 }) });
    if (request.url === '/api.openai.com/v1/models') return json({ data: [{ slug: 'gpt-6.1-sol', display_name: 'GPT-6.1 Sol', visibility: 'list' }] });
    response.writeHead(404).end();
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const address = server.address(); assert.ok(address && typeof address === 'object');
  const runtime = createOpenAIPlanExecutorProcess({ executable: process.execPath, hostId: 'plan-fixture-host',
    args: [fileURLToPath(new URL('./fixtures/openai-plan-executor-worker.mjs', import.meta.url)), state, `http://127.0.0.1:${address.port}`],
    cwd: directory, env: {}, lifecycle: { kind: 'processGroup' } });
  t.after(async () => { await runtime.dispose(); server.close(); rmSync(directory, { recursive: true, force: true }); });
  return { runtime, state, seen, setNonce: (value: string) => { nonce = value; } };
}

test('the owned plan executor signs in through the browser link, saves the credential and lists the account models', async t => {
  const h = await setup(t); assert.ok(h.runtime.signIn && h.runtime.models);
  const result = await h.runtime.signIn({ durationMs: 10_000, onChallenge: challenge => {
    assert.equal(challenge.kind, 'browserAuthorization');
    const query = new URL(challenge.kind === 'browserAuthorization' ? challenge.authorizationURL : '').searchParams;
    h.setNonce(query.get('nonce')!);
    const back = new URL(query.get('redirect_uri')!);
    back.search = new URLSearchParams({ code: 'code-synthetic', state: query.get('state')!, client_id: 'oaiapp_issued' }).toString();
    void fetch(back);
  } }, new AbortController().signal);
  assert.deepEqual(result, { kind: 'signedIn', account: { accountId: 'user-synthetic', userLabel: 'owner@example.com' } });
  assert.equal(h.runtime.lastExit?.groupAbsent, true);
  assert.equal((await readPlanCredential(h.state))?.clientId, 'oaiapp_issued');
  assert.equal(statSync(join(h.state, 'openai-plan.json')).mode & 0o777, 0o600);
  const catalog = await h.runtime.models(new AbortController().signal, result.kind === 'signedIn' ? result.account : undefined);
  assert.equal(catalog.state, 'ready'); assert.deepEqual(catalog.models.map(model => model.id), ['gpt-6.1-sol']);
  assert.ok(h.seen.includes('POST /auth.openai.com/api/accounts/oauth/token'));
});

test('the plan executor refuses an inexact state directory', () => {
  for (const stateDirectory of ['relative/state', '/', '/tmp/../tmp/state'])
    assert.throws(() => serveOpenAIPlanExecutor({ input: new PassThrough(), output: new PassThrough(), hostId: 'plan-fixture-host', stateDirectory }), /exact private state/);
});
