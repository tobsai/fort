import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import type { Acceptance, AccountSignInStatus, Agent, RuntimeDescriptor, RuntimeReadiness } from '../contracts/v1.ts';
import type { ChatRuntime } from '../runner/chat-runtime.ts';
import { connectToHub } from '../runner/hub-runner.ts';
import { startCloudHub } from './cloud-hub.ts';

// Spec 063, revision 2026-11-03: the hub runs Agents itself once it is signed in. Macs are extensions: they take the
// runtimes the hub can't serve, and the hub's runtimes only while the hub is not signed in.
const secret = () => randomBytes(32).toString('base64url');
const chatgpt: RuntimeDescriptor = { provider: 'remote-fixture', name: 'ChatGPT fixture', runtimeVersion: 'fixture-1', hostId: 'home', session: 'stateless', capabilities: ['plainText'] };
const claude: RuntimeDescriptor = { provider: 'claude-fixture', name: 'Claude fixture', runtimeVersion: 'fixture-2', hostId: 'home', session: 'stateless', capabilities: ['plainText'] };
const setupCode = secret(), setupCodeDigest = createHash('sha256').update(setupCode).digest('hex');
const account = { accountId: 'account', userLabel: 'owner@example.com' };

async function fixture(t: TestContext, pool: string[] | undefined = ['mini']) {
  const dataDirectory = await mkdtemp(join(tmpdir(), 'fort-cloud-runner-')), seen: string[] = [], hubState = { signedIn: false, signIns: 0 };
  const hubRuntime: ChatRuntime = {
    async execute(input) { seen.push('hub:' + input.text); return { kind: 'completed', text: input.text, effectiveModel: null }; },
    readiness: () => ({ state: 'ready', canRestart: false, detail: 'ready' }),
    signIn: async () => { hubState.signIns++; hubState.signedIn = true; return { kind: 'signedIn', account }; },
    async dispose() {},
  };
  const hub = await startCloudHub({ dataDirectory, host: '127.0.0.1', port: 0, provider: chatgpt, runtimes: [claude], setupCodeDigest, ...(pool ? { pool } : {}),
    cloud: descriptor => descriptor === chatgpt ? { runtime: hubRuntime, signedIn: () => hubState.signedIn } : undefined });
  t.after(async () => { await hub.close(); await rm(dataDirectory, { recursive: true, force: true }); });
  const owner = { deviceId: randomUUID(), deviceToken: secret() };
  let installationId = '';
  async function call<T = Record<string, unknown>>(path: string, body?: unknown, token: string = owner.deviceToken) {
    const response = await fetch(hub.url + path, { method: body === undefined ? 'GET' : 'POST',
      headers: { 'content-type': 'application/json', 'x-fort-installation-id': installationId, ...(token ? { authorization: 'Bearer ' + token } : {}) },
      body: body === undefined ? undefined : JSON.stringify(body) });
    const text = await response.text();
    return { status: response.status, body: (text ? JSON.parse(text) : {}) as T };
  }
  const setup = await call<{ installationId: string }>('/v1/hub/setup', { setupCode, ...owner }, '');
  assert.equal(setup.status, 201); installationId = setup.body.installationId;
  async function connectMac(runnerId: string) {
    const invitation = { invitationId: randomUUID(), code: secret() }, runnerToken = secret();
    assert.equal((await call('/v1/runners/invitation', invitation)).status, 201);
    assert.equal((await call('/v1/runners/join', { ...invitation, runnerId, runnerToken, runnerName: runnerId }, '')).status, 201);
    const stop = new AbortController();
    const runtime = (name: string): ChatRuntime => ({ async execute(input) { seen.push(name + ':' + input.text); return { kind: 'completed', text: input.text, effectiveModel: null }; },
      signIn: async () => ({ kind: 'signedIn', account }), async dispose() {} });
    const node = pool ? 'home' : runnerId;
    void connectToHub({ url: hub.url, token: runnerToken, binding: { provider: chatgpt.provider, runtimeVersion: chatgpt.runtimeVersion, hostId: node }, createRuntime: () => runtime(runnerId),
      lanes: [{ binding: { provider: claude.provider, runtimeVersion: claude.runtimeVersion, hostId: node }, createRuntime: () => runtime(runnerId + '-claude') }], signal: stop.signal }).catch(() => {});
    t.after(() => stop.abort());
    for (let index = 0; index < 200 && !(await call<{ runners: { runnerId: string; online: boolean }[] }>('/v1/runners')).body.runners.find(entry => entry.runnerId === runnerId)?.online; index++) await delay(10);
    return stop;
  }
  async function confirmAccount(provider: string) {
    const query = '?provider=' + provider;
    const idle = (await call<AccountSignInStatus>('/v1/account/sign-in' + query)).body;
    assert.equal((await call('/v1/account/sign-in/start' + query, { operationId: randomUUID(), expectedRevision: idle.revision, recoveryEpoch: hub.installation.recoveryEpoch, expiresAt: Date.now() + 60_000 })).status, 200);
    let status = (await call<AccountSignInStatus>('/v1/account/sign-in' + query)).body;
    for (let index = 0; index < 200 && status.state !== 'finished'; index++) { await delay(10); status = (await call<AccountSignInStatus>('/v1/account/sign-in' + query)).body; }
    // A sign-in that reports an account the hub has not chosen yet is chosen, as the web page does.
    if (status.state === 'finished' && status.result.kind === 'signedIn' && status.result.account && !status.selected)
      assert.equal((await call('/v1/account/sign-in/select' + query, { operationId: status.operationId, revision: status.revision, recoveryEpoch: hub.installation.recoveryEpoch, account: status.result.account })).status, 200);
  }
  const agents = new Map<string, Agent>();
  async function send(text: string, provider = chatgpt.provider) {
    let agent = agents.get(provider);
    if (!agent) { agent = (await call<Agent>('/v1/agents', { name: provider, model: 'exact-model', instructions: 'Echo.', provider })).body; agents.set(provider, agent); }
    const accepted = await call<Acceptance>(`/v1/conversations/${agent.homeConversationId}/messages`, { requestId: randomUUID(), text, recipientAgentId: agent.agentId,
      bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: hub.installation.recoveryEpoch, skills: [] });
    assert.equal(accepted.status, 201, JSON.stringify(accepted.body));
    return accepted.body.receipt.requestId;
  }
  const run = async (requestId: string) => (await call<Acceptance>('/v1/requests/' + requestId)).body.snapshot.runs.find(entry => entry.requestId === requestId);
  async function settled(requestId: string) {
    for (let index = 0; index < 300; index++) { const current = await run(requestId); if (current && !['queued', 'running'].includes(current.state)) return current; await delay(20); }
    assert.fail('The run did not settle');
  }
  return { call, connectMac, confirmAccount, send, run, settled, seen, hubState };
}

test('signed in on the hub, Agents run with no Mac at all', async t => {
  const h = await fixture(t);
  await h.confirmAccount(chatgpt.provider);
  assert.equal(h.hubState.signIns, 1, 'the sign-in ran on the hub');
  assert.equal((await h.settled(await h.send('no mac needed'))).state, 'completed');
  assert.deepEqual(h.seen, ['hub:no mac needed']);
  const readiness = (await h.call<RuntimeReadiness>('/v1/runtime')).body;
  assert.equal(readiness.state, 'ready');
  const listed = (await h.call<{ cloud: { runnerId: string; lanes: { provider: string; signedIn: boolean }[] } }>('/v1/runners')).body;
  assert.deepEqual(listed.cloud, { runnerId: 'cloud', lanes: [{ provider: chatgpt.provider, name: chatgpt.name, runtimeVersion: chatgpt.runtimeVersion, signedIn: true }] });
});

test('a connected Mac gets no ChatGPT turns once the hub is signed in', async t => {
  const h = await fixture(t);
  await h.connectMac('mini');
  await h.confirmAccount(chatgpt.provider);
  await h.settled(await h.send('first'));
  await h.settled(await h.send('second'));
  assert.deepEqual(h.seen, ['hub:first', 'hub:second']);
});

test('the Mac is the extension for a runtime the hub does not serve', async t => {
  const h = await fixture(t);
  await h.connectMac('mini');
  await h.confirmAccount(chatgpt.provider);
  await h.confirmAccount(claude.provider);
  assert.equal(h.hubState.signIns, 1, 'Claude was confirmed on the Mac, not the hub');
  assert.equal((await h.settled(await h.send('chatgpt'))).state, 'completed');
  assert.equal((await h.settled(await h.send('claude', claude.provider))).state, 'completed');
  assert.deepEqual(h.seen, ['hub:chatgpt', 'mini-claude:claude']);
});

test('without a pool, the one enrolled Mac is still the extension', async t => {
  const h = await fixture(t, undefined);
  await h.confirmAccount(chatgpt.provider);
  assert.equal((await h.settled(await h.send('pool-free'))).state, 'completed');
  assert.deepEqual(h.seen, ['hub:pool-free']);
});

test('while the hub is signed out, work waits, and runs once it signs in again to the same account', async t => {
  const h = await fixture(t);
  await h.confirmAccount(chatgpt.provider);
  h.hubState.signedIn = false;
  const requestId = await h.send('waiting for sign-in');
  await delay(200);
  assert.equal((await h.run(requestId))?.state, 'queued');
  const readiness = (await h.call<RuntimeReadiness>('/v1/runtime')).body;
  assert.equal(readiness.state, 'unavailable');
  assert.match(readiness.detail, /Sign in to ChatGPT on the hub/);
  await h.confirmAccount(chatgpt.provider);
  assert.equal(h.hubState.signIns, 2, 'the second sign-in also ran on the hub');
  assert.equal((await h.settled(requestId)).state, 'completed');
  assert.deepEqual(h.seen, ['hub:waiting for sign-in']);
});
