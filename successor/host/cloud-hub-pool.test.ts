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
import { startCloudHub, type QueueAlert } from './cloud-hub.ts';

// Spec 063, revision 2026-10-28: the Mac mini and the laptop serve one node. Runs go to the mini, to the
// laptop when the mini is away, and wait queued when neither is connected.
const secret = () => randomBytes(32).toString('base64url');
const provider: RuntimeDescriptor = { provider: 'remote-fixture', name: 'Remote fixture', runtimeVersion: 'fixture-1', hostId: 'home', session: 'stateless', capabilities: ['plainText'] };
const setupCode = secret(), setupCodeDigest = createHash('sha256').update(setupCode).digest('hex');
type Pool = { node: string; runners: string[]; queue: { waiting: number; waitingSince?: string; alert: boolean } };

async function fixture(t: TestContext, queueAlertMs = 60_000) {
  const dataDirectory = await mkdtemp(join(tmpdir(), 'fort-cloud-pool-')), alerts: QueueAlert[] = [];
  const hub = await startCloudHub({ dataDirectory, host: '127.0.0.1', port: 0, provider, setupCodeDigest, pool: ['mini', 'laptop'], queueAlertMs, onQueueAlert: alert => alerts.push(alert) });
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
  const tokens = new Map<string, string>(), seen: string[] = [];
  async function enroll(runnerId: string) {
    const invitation = { invitationId: randomUUID(), code: secret() }, runnerToken = secret();
    assert.equal((await call('/v1/runners/invitation', invitation)).status, 201);
    const joined = await call<{ runnerId: string; node: string }>('/v1/runners/join', { ...invitation, runnerId, runnerToken, runnerName: runnerId === 'mini' ? 'Mac mini' : 'MacBook' }, '');
    assert.equal(joined.status, 201);
    tokens.set(runnerId, runnerToken);
    return joined.body;
  }
  // Every Mac in the pool offers its lanes under the pool's node and reports the same account.
  function connect(runnerId: string) {
    const stop = new AbortController();
    const runtime: ChatRuntime = { async execute(input) { seen.push(runnerId + ':' + input.text); return { kind: 'completed', text: input.text, effectiveModel: null }; },
      signIn: async () => ({ kind: 'signedIn', account: { accountId: 'account', userLabel: 'owner@example.com' } }), async dispose() {} };
    void connectToHub({ url: hub.url, token: tokens.get(runnerId)!, binding: { provider: provider.provider, runtimeVersion: provider.runtimeVersion, hostId: 'home' }, createRuntime: () => runtime, signal: stop.signal }).catch(() => {});
    t.after(() => stop.abort());
    return stop;
  }
  const pool = async () => (await call<{ runners: { runnerId: string; online: boolean }[]; pool: Pool }>('/v1/runners')).body;
  async function online(runnerId: string, expected = true) {
    for (let index = 0; index < 200 && (await pool()).runners.find(runner => runner.runnerId === runnerId)?.online !== expected; index++) await delay(10);
  }
  async function confirmAccount() {
    const idle = (await call<AccountSignInStatus>('/v1/account/sign-in')).body;
    assert.equal((await call('/v1/account/sign-in/start', { operationId: randomUUID(), expectedRevision: idle.revision, recoveryEpoch: hub.installation.recoveryEpoch, expiresAt: Date.now() + 60_000 })).status, 200);
    for (let index = 0; index < 200 && (await call<AccountSignInStatus>('/v1/account/sign-in')).body.state !== 'finished'; index++) await delay(10);
  }
  let agent: Agent | undefined;
  async function send(text: string) {
    agent ??= (await call<Agent>('/v1/agents', { name: 'Remote', model: 'exact-model', instructions: 'Echo.' })).body;
    const accepted = await call<Acceptance>(`/v1/conversations/${agent.homeConversationId}/messages`, { requestId: randomUUID(), text, recipientAgentId: agent.agentId,
      bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: hub.installation.recoveryEpoch, skills: [] });
    assert.equal(accepted.status, 201);
    return accepted.body.receipt.requestId;
  }
  const run = async (requestId: string) => (await call<Acceptance>('/v1/requests/' + requestId)).body.snapshot.runs[0];
  async function settled(requestId: string) {
    for (let index = 0; index < 300; index++) { const current = await run(requestId); if (current && !['queued', 'running'].includes(current.state)) return current; await delay(20); }
    assert.fail('The run did not settle');
  }
  return { call, enroll, connect, pool, online, confirmAccount, send, run, settled, seen, alerts };
}

test('each Mac in the pool learns the node it serves when it joins', async t => {
  const h = await fixture(t);
  assert.deepEqual(await h.enroll('mini'), { runnerId: 'mini', runnerName: 'Mac mini', node: 'home' });
  assert.deepEqual(await h.enroll('laptop'), { runnerId: 'laptop', runnerName: 'MacBook', node: 'home' });
  const listed = await h.pool();
  assert.deepEqual(listed.pool, { node: 'home', runners: ['mini', 'laptop'], queue: { waiting: 0, alert: false } });
});

test('runs go to the mini, and to the laptop while the mini is away', async t => {
  const h = await fixture(t);
  await h.enroll('mini'); await h.enroll('laptop');
  const mini = h.connect('mini'); h.connect('laptop');
  await h.online('mini'); await h.online('laptop');
  await h.confirmAccount();
  assert.equal((await h.settled(await h.send('to the mini'))).state, 'completed');
  mini.abort();
  await h.online('mini', false);
  assert.equal((await h.settled(await h.send('to the laptop'))).state, 'completed');
  assert.deepEqual(h.seen, ['mini:to the mini', 'laptop:to the laptop']);
  const listed = (await h.call<{ runners: { runnerId: string; online: boolean; turns: number; lastSeen?: string; connectedAt?: string }[] }>('/v1/runners')).body.runners;
  const away = listed.find(runner => runner.runnerId === 'mini')!, here = listed.find(runner => runner.runnerId === 'laptop')!;
  assert.deepEqual([away.online, away.turns, typeof away.lastSeen, away.connectedAt], [false, 0, 'string', undefined]);
  assert.deepEqual([here.online, here.turns, typeof here.connectedAt], [true, 0, 'string']);
});

test('with no Mac connected, work waits queued and starts when one connects', async t => {
  const h = await fixture(t);
  await h.enroll('mini'); await h.enroll('laptop');
  const first = h.connect('mini');
  await h.online('mini');
  await h.confirmAccount();
  first.abort();
  await h.online('mini', false);
  const requestId = await h.send('while away');
  await delay(200);
  assert.equal((await h.run(requestId))?.state, 'queued');
  const readiness = (await h.call<RuntimeReadiness>('/v1/runtime')).body;
  assert.equal(readiness.state, 'unavailable');
  assert.match(readiness.detail, /Waiting for Mac mini or MacBook to connect/);
  const waiting = (await h.pool()).pool.queue;
  assert.equal(waiting.waiting, 1); assert.equal(waiting.alert, false); assert.ok(waiting.waitingSince);
  h.connect('laptop');
  assert.equal((await h.settled(requestId)).state, 'completed');
  assert.deepEqual(h.seen, ['laptop:while away']);
  assert.deepEqual((await h.pool()).pool.queue, { waiting: 0, alert: false });
});

test('queued work that has waited past the alert time raises one alert', async t => {
  const h = await fixture(t, 1000);
  await h.enroll('mini'); await h.enroll('laptop');
  const first = h.connect('mini');
  await h.online('mini');
  await h.confirmAccount();
  first.abort();
  await h.online('mini', false);
  await h.send('still waiting');
  await h.pool();
  for (let index = 0; index < 300 && !h.alerts.length; index++) await delay(10);
  assert.equal(h.alerts.length, 1);
  assert.equal(h.alerts[0]!.waiting, 1);
  assert.deepEqual(h.alerts[0]!.runners, ['mini', 'laptop']);
  assert.equal((await h.pool()).pool.queue.alert, true);
  assert.match((await h.call<RuntimeReadiness>('/v1/runtime')).body.detail, /One run has waited since/);
  await delay(1200);
  assert.equal(h.alerts.length, 1);
});

test('a pool must name distinct Macs', async t => {
  const dataDirectory = await mkdtemp(join(tmpdir(), 'fort-cloud-pool-'));
  t.after(() => rm(dataDirectory, { recursive: true, force: true }));
  await assert.rejects(startCloudHub({ dataDirectory, host: '127.0.0.1', port: 0, provider, pool: ['mini', 'mini'] }), /distinct pool runners/);
  await assert.rejects(startCloudHub({ dataDirectory, host: '127.0.0.1', port: 0, provider, pool: [] }), /distinct pool runners/);
});
