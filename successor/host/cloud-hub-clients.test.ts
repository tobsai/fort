import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import type { Acceptance, AccountSignInStatus, Agent, Installation, ModelCatalog, RuntimeDescriptor } from '../contracts/v1.ts';
import type { ChatInput, ChatRuntime } from '../runner/chat-runtime.ts';
import { connectToHub } from '../runner/hub-runner.ts';
import { startCloudHub } from './cloud-hub.ts';

// Spec 063 revision: one Mac serves several providers through one runner connection, and the hub
// serves its own web client, which is the only browser origin allowed to call the API.
const secret = () => randomBytes(32).toString('base64url');
const node = 'tobys-mac';
const codex: RuntimeDescriptor = { provider: 'codex', name: 'ChatGPT', runtimeVersion: '0.163.0', hostId: node, session: 'stateless', capabilities: ['plainText', 'modelCatalog', 'cancel'] };
const claude: RuntimeDescriptor = { provider: 'claude-code', name: 'Claude', runtimeVersion: '2.1.0', hostId: node, session: 'stateless', capabilities: ['plainText', 'modelCatalog', 'cancel'] };
const setupCode = secret(), setupCodeDigest = createHash('sha256').update(setupCode).digest('hex');
const account = (provider: string) => ({ accountId: provider + '-account', userLabel: 'toby@example.com' });

// A Mac-side runtime that reports a signed-in account and its models, and refuses a turn pinned to
// any other account, as the Claude Code runtime does.
function macRuntime(provider: string, seen: ChatInput[]): ChatRuntime {
  return {
    async execute(input) {
      seen.push(input);
      if (input.configuration.account?.accountId !== account(provider).accountId) return { kind: 'notStarted', message: 'The selected account is not the signed-in account. Nothing was sent.' };
      return { kind: 'completed', text: `${provider}: ${input.text}`, effectiveModel: null };
    },
    async models() { return { provider, state: 'ready', models: [{ id: provider + '-model', name: provider + ' model', availability: 'available' }], checkedAt: new Date().toISOString() }; },
    async signIn() { return { kind: 'signedIn', account: account(provider) }; },
    async dispose() {},
  };
}

async function fixture(t: TestContext) {
  const dataDirectory = await mkdtemp(join(tmpdir(), 'fort-cloud-clients-'));
  const hub = await startCloudHub({ dataDirectory, host: '127.0.0.1', port: 0, provider: codex, runtimes: [claude], setupCodeDigest, publicOrigin: 'https://fort.example.com' });
  t.after(async () => { await hub.close(); await rm(dataDirectory, { recursive: true, force: true }); });
  const owner = { deviceId: randomUUID(), deviceToken: secret() };
  let installationId = '';
  async function call<T = Record<string, unknown>>(path: string, body?: unknown, extra: Record<string, string> = {}) {
    const response = await fetch(hub.url + path, { method: body === undefined ? 'GET' : 'POST',
      headers: { 'content-type': 'application/json', 'x-fort-installation-id': installationId, authorization: 'Bearer ' + owner.deviceToken, ...extra },
      body: body === undefined ? undefined : JSON.stringify(body) });
    const text = await response.text();
    return { status: response.status, body: (text ? JSON.parse(text) : {}) as T, headers: response.headers };
  }
  async function setup(extra: Record<string, string> = {}) {
    const result = await call<{ installationId: string }>('/v1/hub/setup', { setupCode, ...owner }, extra);
    assert.equal(result.status, 201);
    installationId = result.body.installationId;
  }
  async function connectMac(seen: ChatInput[]) {
    const invitation = { invitationId: randomUUID(), code: secret() }, runnerToken = secret();
    assert.equal((await call('/v1/runners/invitation', invitation)).status, 201);
    assert.equal((await call('/v1/runners/join', { ...invitation, runnerId: node, runnerToken, runnerName: 'Toby’s Mac' })).status, 201);
    const stop = new AbortController(), lane = (descriptor: RuntimeDescriptor) => ({ binding: { provider: descriptor.provider, runtimeVersion: descriptor.runtimeVersion, hostId: node },
      createRuntime: () => macRuntime(descriptor.provider, seen) });
    const [first, second] = [lane(codex), lane(claude)];
    void connectToHub({ url: hub.url, token: runnerToken, ...first, lanes: [second], signal: stop.signal }).catch(() => {});
    t.after(() => stop.abort());
    for (let index = 0; index < 200 && !(await call<{ runners: { online: boolean }[] }>('/v1/runners')).body.runners[0]?.online; index++) await delay(10);
  }
  async function confirmAccount(provider: string) {
    const query = '?provider=' + provider;
    const idle = (await call<AccountSignInStatus>('/v1/account/sign-in' + query)).body;
    assert.equal((await call('/v1/account/sign-in/start' + query, { operationId: randomUUID(), expectedRevision: idle.revision, recoveryEpoch: hub.installation.recoveryEpoch, expiresAt: Date.now() + 60_000 })).status, 200);
    for (let index = 0; index < 200; index++) {
      const status = (await call<AccountSignInStatus>('/v1/account/sign-in' + query)).body;
      if (status.state === 'finished') {
        if (status.result.kind === 'signedIn' && status.result.account && !status.selected)
          return (await call<AccountSignInStatus>('/v1/account/sign-in/select' + query, { operationId: status.operationId, revision: status.revision, recoveryEpoch: hub.installation.recoveryEpoch, account: status.result.account })).body;
        return status;
      }
      await delay(10);
    }
    assert.fail('The account check did not finish');
  }
  async function talk(provider: string, model: string, text: string) {
    const created = await call<Agent>('/v1/agents', { name: provider, provider, model, instructions: 'Be brief.' }), agent = created.body;
    assert.equal(created.status, 201, JSON.stringify(created.body));
    assert.equal(agent.binding.hostId, node);
    const accepted = await call<Acceptance>(`/v1/conversations/${agent.homeConversationId}/messages`, { requestId: randomUUID(), text, recipientAgentId: agent.agentId,
      bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: hub.installation.recoveryEpoch, skills: [] });
    assert.equal(accepted.status, 201, JSON.stringify(accepted.body));
    for (let index = 0; index < 300; index++) {
      const snapshot = (await call<Acceptance>('/v1/requests/' + accepted.body.receipt.requestId)).body.snapshot;
      const run = snapshot.runs[0];
      if (run && !['queued', 'running'].includes(run.state)) return { run, snapshot };
      await delay(20);
    }
    assert.fail('The run did not settle');
  }
  return { hub, call, setup, connectMac, confirmAccount, talk };
}

test('one Mac connection serves ChatGPT and Claude, each on the account the Mac reports', async t => {
  const h = await fixture(t), seen: ChatInput[] = [];
  await h.setup();
  const installation = (await h.call<Installation>('/v1/installation')).body;
  assert.deepEqual(installation.runtimes?.map(runtime => [runtime.provider, runtime.hostId]), [['codex', node], ['claude-code', node]]);
  assert.ok(installation.capabilities?.includes('accountSignIn'));
  await h.connectMac(seen);

  assert.deepEqual((await h.call<ModelCatalog>('/v1/models?provider=claude-code')).body.models, [], 'models wait for a confirmed account');
  for (const provider of ['codex', 'claude-code']) {
    const status = await h.confirmAccount(provider);
    assert.equal(status.state, 'finished');
    assert.ok(status.state === 'finished' && status.selected);
  }
  const catalog = (await h.call<ModelCatalog>('/v1/models?provider=claude-code')).body;
  assert.equal(catalog.state, 'ready');
  assert.deepEqual(catalog.models.map(model => model.id), ['claude-code-model']);

  const chatgpt = await h.talk('codex', 'codex-model', 'exact cedar 🌲');
  assert.equal(chatgpt.run.state, 'completed', chatgpt.run.statusDetail ?? '');
  assert.equal(chatgpt.snapshot.messages[1]?.text, 'codex: exact cedar 🌲');
  const reply = await h.talk('claude-code', 'claude-code-model', 'hello');
  assert.equal(reply.run.state, 'completed', reply.run.statusDetail ?? '');
  assert.equal(reply.snapshot.messages[1]?.text, 'claude-code: hello');
  assert.deepEqual(seen.map(input => input.configuration.account?.accountId), ['codex-account', 'claude-code-account']);
});

test('a turn needs the Mac’s account confirmed first, and nothing is sent before that', async t => {
  const h = await fixture(t), seen: ChatInput[] = [];
  await h.setup();
  await h.connectMac(seen);
  const agent = (await h.call<Agent>('/v1/agents', { name: 'Claude', provider: 'claude-code', model: 'claude-code-model', instructions: 'Be brief.' })).body;
  const refused = await h.call<{ error: { code: string } }>(`/v1/conversations/${agent.homeConversationId}/messages`, { requestId: randomUUID(), text: 'too soon', recipientAgentId: agent.agentId,
    bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.hub.installation.recoveryEpoch, skills: [] });
  assert.equal(refused.status, 409);
  assert.equal(refused.body.error.code, 'accountSelectionRequired');
  assert.deepEqual(seen, []);
});

test('an account check with the Mac offline is unconfirmed, not invented', async t => {
  const h = await fixture(t);
  await h.setup();
  const status = await h.confirmAccount('codex');
  assert.ok(status.state === 'finished' && status.result.kind === 'unconfirmed');
  assert.equal((await h.call<ModelCatalog>('/v1/models?provider=codex')).body.state, 'unavailable');
});

test('the hub serves its web client with a policy that allows only its own scripts', async t => {
  const h = await fixture(t);
  for (const [path, type] of [['/', 'text/html'], ['/app.js', 'text/javascript'], ['/transcript.js', 'text/javascript'], ['/app.css', 'text/css'], ['/manifest.webmanifest', 'application/manifest+json']] as const) {
    const response = await fetch(h.hub.url + path);
    assert.equal(response.status, 200, path);
    assert.ok(response.headers.get('content-type')?.startsWith(type), path);
    assert.match(response.headers.get('content-security-policy') ?? '', /script-src 'self'/);
    assert.match(response.headers.get('content-security-policy') ?? '', /frame-ancestors 'none'/);
    assert.equal(response.headers.get('x-content-type-options'), 'nosniff');
    await response.arrayBuffer();
  }
  const html = await (await fetch(h.hub.url + '/')).text();
  assert.doesNotMatch(html, /<script>(?!<\/script>)/, 'no inline script');
  assert.notEqual((await fetch(h.hub.url + '/package.json')).status, 200);
  assert.notEqual((await fetch(h.hub.url + '/host/cloud-hub.ts')).status, 200);
});

test('only the hub’s own origin may call the API from a browser', async t => {
  const h = await fixture(t);
  assert.equal((await h.call('/v1/hub/setup', { setupCode, deviceId: randomUUID(), deviceToken: secret() }, { origin: 'https://evil.example' })).status, 403);
  await h.setup({ origin: 'https://fort.example.com' });
  assert.equal((await h.call('/v1/agents', undefined, { origin: 'https://fort.example.com' })).status, 200);
  assert.equal((await h.call('/v1/agents', undefined, { origin: 'https://evil.example' })).status, 403);
  assert.equal((await h.call('/v1/agents', undefined, { origin: 'null' })).status, 403);
  assert.equal((await h.call('/v1/runner/connect', {}, { origin: 'https://fort.example.com' })).status, 403);
  assert.equal((await h.call('/v1/runners/invitation', { invitationId: randomUUID(), code: secret() }, { origin: 'https://fort.example.com' })).status, 201);
});

test('without a public origin the hub refuses every browser origin, as before', async t => {
  const dataDirectory = await mkdtemp(join(tmpdir(), 'fort-cloud-clients-'));
  const hub = await startCloudHub({ dataDirectory, host: '127.0.0.1', port: 0, provider: codex, setupCodeDigest });
  t.after(async () => { await hub.close(); await rm(dataDirectory, { recursive: true, force: true }); });
  const response = await fetch(hub.url + '/v1/installation', { headers: { origin: hub.url } });
  assert.equal(response.status, 403);
  await response.arrayBuffer();
  await assert.rejects(startCloudHub({ dataDirectory: join(dataDirectory, 'other'), host: '127.0.0.1', port: 0, provider: codex, publicOrigin: 'http://fort.example.com' }), /https origin/);
});
