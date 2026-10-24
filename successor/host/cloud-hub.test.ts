import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import type { Acceptance, Agent, RuntimeDescriptor } from '../contracts/v1.ts';
import type { ChatRuntime } from '../runner/chat-runtime.ts';
import { connectToHub } from '../runner/hub-runner.ts';
import { startCloudHub } from './cloud-hub.ts';

const secret = () => randomBytes(32).toString('base64url');
const provider: RuntimeDescriptor = { provider: 'remote-fixture', name: 'Remote fixture', runtimeVersion: 'fixture-1', hostId: 'mac-mini', session: 'stateless', capabilities: ['plainText'] };
const setupCode = secret(), setupCodeDigest = createHash('sha256').update(setupCode).digest('hex');

async function fixture(t: TestContext) {
  const dataDirectory = await mkdtemp(join(tmpdir(), 'fort-cloud-hub-'));
  const open = () => startCloudHub({ dataDirectory, host: '127.0.0.1', port: 0, provider, setupCodeDigest });
  let hub = await open();
  t.after(async () => { await hub.close(); await rm(dataDirectory, { recursive: true, force: true }); });
  const owner = { deviceId: randomUUID(), deviceToken: secret() };
  let installationId = '';
  async function call<T = Record<string, unknown>>(path: string, body?: unknown, token: string | undefined = owner.deviceToken, extra: Record<string, string> = {}) {
    const response = await fetch(hub.url + path, { method: body === undefined ? 'GET' : 'POST',
      headers: { 'content-type': 'application/json', 'x-fort-installation-id': installationId, ...(token ? { authorization: 'Bearer ' + token } : {}), ...extra },
      body: body === undefined ? undefined : JSON.stringify(body) });
    const text = await response.text();
    return { status: response.status, body: (text ? JSON.parse(text) : {}) as T };
  }
  async function setup() {
    const result = await call<{ installationId: string }>('/v1/hub/setup', { setupCode, ...owner }, '');
    assert.equal(result.status, 201);
    installationId = result.body.installationId;
  }
  async function enroll(runnerId = 'mac-mini') {
    const invitation = { invitationId: randomUUID(), code: secret() }, runnerToken = secret();
    assert.equal((await call('/v1/runners/invitation', invitation)).status, 201);
    assert.equal((await call('/v1/runners/join', { ...invitation, runnerId, runnerToken, runnerName: 'Mac mini' }, '')).status, 201);
    return runnerToken;
  }
  function runner(token: string, runtime: ChatRuntime) {
    const stop = new AbortController();
    void connectToHub({ url: hub.url, token, binding: { provider: provider.provider, runtimeVersion: provider.runtimeVersion, hostId: 'mac-mini' }, createRuntime: () => runtime, signal: stop.signal }).catch(() => {});
    t.after(() => stop.abort());
    return stop;
  }
  async function send(text: string) {
    const agent = (await call<Agent>('/v1/agents', { name: 'Remote', model: 'exact-model', instructions: 'Echo.' })).body;
    const accepted = await call<Acceptance>(`/v1/conversations/${agent.homeConversationId}/messages`, { requestId: randomUUID(), text, recipientAgentId: agent.agentId,
      bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: hub.installation.recoveryEpoch, skills: [] });
    assert.equal(accepted.status, 201);
    return accepted.body.receipt.requestId;
  }
  async function settled(requestId: string, states = ['completed', 'failed', 'outcomeUnknown', 'canceled', 'notStarted']) {
    for (let index = 0; index < 300; index++) {
      const run = (await call<Acceptance>('/v1/requests/' + requestId)).body.snapshot.runs[0];
      if (run && states.includes(run.state)) return run;
      await delay(20);
    }
    assert.fail('The run did not settle');
  }
  return { call, setup, enroll, runner, send, settled, dataDirectory, open,
    get hub() { return hub; }, async restart() { await hub.close(); hub = await open(); } };
}
const echo = (seen: string[] = []): ChatRuntime => ({ async execute(input) { seen.push(input.text); return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() {} });

test('the owner sets up the hub with the setup code, and only once', async t => {
  const h = await fixture(t);
  assert.equal((await h.call('/v1/hub/setup', { setupCode: secret(), deviceId: randomUUID(), deviceToken: secret() }, '')).status, 401);
  await h.setup();
  assert.equal((await h.call('/v1/hub/setup', { setupCode, deviceId: randomUUID(), deviceToken: secret() }, '')).status, 409);
  assert.equal((await h.call('/v1/agents')).status, 200);
});

test('a turn sent to the hub runs on the enrolled Mac and comes back exact', async t => {
  const h = await fixture(t), seen: string[] = [];
  await h.setup();
  h.runner(await h.enroll(), echo(seen));
  for (let index = 0; index < 200 && !(await h.call<{ runners: { online: boolean }[] }>('/v1/runners')).body.runners[0]?.online; index++) await delay(10);
  const requestId = await h.send('exact cedar 🌲');
  assert.equal((await h.settled(requestId)).state, 'completed');
  const snapshot = (await h.call<Acceptance>('/v1/requests/' + requestId)).body.snapshot;
  assert.equal(snapshot.messages[1]?.text, 'exact cedar 🌲');
  assert.deepEqual(seen, ['exact cedar 🌲']);
});

test('runner management is owner-only and never shows a token', async t => {
  const h = await fixture(t);
  await h.setup();
  const token = await h.enroll();
  const listed = await h.call<{ runners: unknown[] }>('/v1/runners');
  assert.deepEqual(listed.body.runners, [{ runnerId: 'mac-mini', runnerName: 'Mac mini', enrolledAt: (listed.body.runners[0] as { enrolledAt: string }).enrolledAt, online: false }]);
  assert.ok(!JSON.stringify(listed.body).includes(token));
  assert.equal((await h.call('/v1/runners', undefined, secret())).status, 401);
  assert.equal((await h.call('/v1/runners', undefined, token)).status, 401);
  assert.equal((await h.call('/v1/runners/invitation', { invitationId: randomUUID(), code: secret() }, '')).status, 401);
  assert.equal((await h.call('/v1/runners/revoke', { runnerId: 'mac-mini' })).status, 200);
  assert.deepEqual((await h.call<{ runners: unknown[] }>('/v1/runners')).body.runners, []);
});

test('an offline Mac fails the turn visibly instead of moving it', async t => {
  const h = await fixture(t);
  await h.setup();
  await h.enroll();
  const run = await h.settled(await h.send('nobody home'));
  assert.notEqual(run.state, 'completed');
  assert.match(run.statusDetail ?? '', /not connected/);
});

test('forwarded headers never become identity', async t => {
  const h = await fixture(t);
  await h.setup();
  assert.equal((await h.call('/v1/agents', undefined, undefined, { 'x-forwarded-for': '127.0.0.1', forwarded: 'for=127.0.0.1' })).status, 200);
  assert.equal((await h.call('/v1/agents', undefined, '', { 'x-forwarded-for': '127.0.0.1' })).status, 401);
});

test('a hub restart mid-turn leaves the turn unknown and does not replay it', async t => {
  const h = await fixture(t), seen: string[] = [], started = Promise.withResolvers<void>();
  await h.setup();
  const token = await h.enroll();
  h.runner(token, { async execute(input, signal) { seen.push(input.text); started.resolve(); await new Promise(resolve => signal.addEventListener('abort', resolve, { once: true })); return { kind: 'canceled', message: 'gone' }; }, async dispose() {} });
  const requestId = await h.send('in flight');
  await started.promise;
  await h.restart();
  h.runner(token, echo(seen));
  const run = await h.settled(requestId, ['outcomeUnknown', 'completed', 'failed', 'notStarted']);
  assert.equal(run.state, 'outcomeUnknown');
  await delay(200);
  assert.deepEqual(seen, ['in flight']);
});

test('a second hub cannot open the same data', async t => {
  const h = await fixture(t);
  await assert.rejects(h.open());
});
