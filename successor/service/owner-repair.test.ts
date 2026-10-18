import assert from 'node:assert/strict';
import { test, type TestContext } from 'node:test';
import { randomBytes, randomUUID } from 'node:crypto';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createApplication } from './openai.ts';
import type { ChatInput, ChatRuntime } from '../runner/chat-runtime.ts';
import type { Agent, RuntimeReadiness, Snapshot } from '../contracts/v1.ts';

class PairingRuntime implements ChatRuntime {
  stopped = false;
  restarts = 0;
  disposals = 0;
  failStop = false;
  failRestart = false;
  restartSupported = true;
  hold: Promise<void> | undefined;
  async execute(input: ChatInput) { return { kind: 'completed' as const, text: input.text, effectiveModel: null }; }
  async dispose() {
    this.stopped = true; this.disposals++;
    if (this.failStop) throw new Error('Private synthetic stop failure');
  }
  readiness(): RuntimeReadiness { return { state: this.stopped ? 'stopped' : 'ready', canRestart: this.stopped && this.restartSupported, detail: 'Synthetic re-pair runtime.' }; }
  async restart(signal: AbortSignal) {
    signal.throwIfAborted(); this.restarts++; await this.hold; signal.throwIfAborted();
    if (this.failRestart) throw new Error('Private synthetic resume failure');
    this.stopped = false;
  }
}

async function fixture(t: TestContext) {
  const directory = await mkdtemp(join(tmpdir(), 'fort-owner-repair-'));
  let runtime = new PairingRuntime(), now = Date.now();
  const create = () => createApplication({ dataDirectory: directory, runtime, now: () => now, ownerPairingPolicy: { expiresAfterDays: null },
    provider: { provider: 'repair-fixture', name: 'Synthetic re-pair', runtimeVersion: '1', hostId: 'exact-local', session: 'stateless', capabilities: ['plainText'] } });
  let service = create(); await service.start();
  t.after(async () => { runtime.failStop = false; await service.close(); await rm(directory, { recursive: true, force: true }); });
  function prepare() {
    const offer = service.beginOwnerSetup(); assert.equal(offer.kind, 'available');
    return { installationId: offer.installationId, setupId: offer.setupId, deviceId: randomUUID(), deviceToken: randomBytes(32).toString('base64url') };
  }
  const original = prepare(); await service.completeOwnerSetup(original);
  async function api<T>(token: string, path: string, body?: unknown) {
    const response = await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers: {
      Authorization: 'Bearer ' + token, 'X-Fort-Installation-Id': service.installation.installationId, 'Content-Type': 'application/json',
    }, body: body === undefined ? undefined : JSON.stringify(body) });
    return { status: response.status, body: await response.json() as T };
  }
  return { original, prepare, api, get service() { return service; }, get runtime() { return runtime; },
    advance(milliseconds: number) { now += milliseconds; },
    async disconnect() { assert.equal((await api(original.deviceToken, '/v1/pairing/disconnect', {})).status, 200); },
    async reopen() { await service.close(); runtime = new PairingRuntime(); service = create(); await service.start(); } };
}

for (const reopened of [false, true]) test(`explicit local re-pair after disconnect preserves history and rejects the old credential (reopened=${reopened})`, async t => {
  const h = await fixture(t), old = h.original;
  const agent = (await h.api<Agent>(old.deviceToken, '/v1/agents', { name: 'Retained', instructions: 'Echo.', model: 'exact' })).body;
  const path = '/v1/conversations/' + agent.homeConversationId;
  const send = (token: string, requestId: string) => h.api(token, path + '/messages', { requestId, text: ' exact retained 🌲\n', recipientAgentId: agent.agentId,
    bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] });
  assert.equal((await send(old.deviceToken, 'before')).status, 201); await h.service.dispatchNext();
  const history = (await h.api<Snapshot>(old.deviceToken, path + '/snapshot')).body.messages;
  await h.disconnect(); if (reopened) await h.reopen();
  const replacement = h.prepare();
  assert.notEqual(replacement.setupId, old.setupId);
  assert.equal(h.runtime.restarts, 0); assert.equal(await h.service.dispatchNext(), false);
  assert.equal((await h.api(old.deviceToken, '/v1/installation')).status, 401);
  assert.deepEqual(h.service.inspectSession().deviceIDs, []);
  const receipt = await h.service.completeOwnerSetup(replacement);
  assert.deepEqual(await h.service.completeOwnerSetup(replacement), receipt);
  assert.equal(h.runtime.restarts, reopened ? 0 : 1);
  assert.equal((await h.api(old.deviceToken, '/v1/installation')).status, 401);
  assert.deepEqual((await h.api<Snapshot>(replacement.deviceToken, path + '/snapshot')).body.messages, history);
  assert.equal((await send(replacement.deviceToken, 'after')).status, 201); await h.service.dispatchNext();
  assert.equal((await h.api<Snapshot>(replacement.deviceToken, path + '/snapshot')).body.messages.length, history.length + 2);
});

test('ordinary stop, unconfirmed disconnect and unsupported runtime remain setup barriers', async t => {
  const h = await fixture(t);
  assert.equal((await h.api(h.original.deviceToken, '/v1/host/stop', {})).status, 200);
  assert.throws(h.prepare, /running local host/);
  assert.equal((await h.api(h.original.deviceToken, '/v1/runtime/restart', { recoveryEpoch: h.service.installation.recoveryEpoch })).status, 200);
  h.runtime.failStop = true;
  assert.equal((await h.api(h.original.deviceToken, '/v1/pairing/disconnect', {})).status, 503);
  assert.throws(h.prepare, /running local host/);
  assert.deepEqual(h.service.inspectSession().deviceIDs, [h.original.deviceId]);
  h.runtime.failStop = false; await h.disconnect(); h.runtime.restartSupported = false;
  assert.throws(h.prepare, /running local host/);
  assert.equal(h.runtime.restarts, 1);
});

test('invalid new identity, old device and canceled offer cannot prepare runtime or pair', async t => {
  const h = await fixture(t); await h.disconnect();
  const next = h.prepare();
  for (const changed of [{ installationId: randomUUID() }, { setupId: randomUUID() }, { deviceId: h.original.deviceId }, { deviceToken: 'invalid' }]) {
    await assert.rejects(h.service.completeOwnerSetup({ ...next, ...changed }));
  }
  assert.equal(h.runtime.restarts, 0); assert.deepEqual(h.service.inspectSession().deviceIDs, []);
  const { deviceToken, ...identity } = next;
  assert.deepEqual(await h.service.cancelOwnerSetup(identity), { ...identity, outcome: 'notPaired' });
  await assert.rejects(h.service.completeOwnerSetup(next), /canceled/);
  assert.equal(await h.service.dispatchNext(), false); assert.equal(h.runtime.restarts, 0);
  assert.equal((await h.api(deviceToken, '/v1/installation')).status, 401);
});

test('failed re-pair runtime preparation keeps revocation and allows exact retry after cleanup', async t => {
  const h = await fixture(t); await h.disconnect(); const next = h.prepare(); h.runtime.failRestart = true;
  await assert.rejects(h.service.completeOwnerSetup(next), error => error instanceof Error && /remains paused/.test(error.message) && !error.message.includes('Private synthetic'));
  assert.equal(h.runtime.stopped, true); assert.equal(h.runtime.disposals, 2);
  assert.deepEqual(h.service.inspectSession().deviceIDs, []); assert.equal(await h.service.dispatchNext(), false);
  assert.equal((await h.api(h.original.deviceToken, '/v1/installation')).status, 401);
  h.runtime.failRestart = false;
  await h.service.completeOwnerSetup(next);
  assert.equal((await h.api(next.deviceToken, '/v1/installation')).status, 200);
});

test('offer expiry during runtime preparation cannot commit a replacement pairing', async t => {
  const hold = Promise.withResolvers<void>(); t.after(() => hold.resolve());
  const h = await fixture(t); await h.disconnect(); const next = h.prepare(); h.runtime.hold = hold.promise;
  const completion = h.service.completeOwnerSetup(next);
  const rejected = assert.rejects(completion, /remains paused/);
  for (let i = 0; h.runtime.restarts === 0 && i < 100; i++) await new Promise(resolve => setTimeout(resolve, 5));
  assert.equal(h.runtime.restarts, 1); h.advance(300_001); hold.resolve(); await rejected;
  assert.deepEqual(h.service.inspectSession().deviceIDs, []); assert.equal(h.runtime.stopped, true);
  assert.equal((await h.api(next.deviceToken, '/v1/installation')).status, 401);
  const replacement = h.prepare(); assert.notEqual(replacement.setupId, next.setupId);
  await h.service.completeOwnerSetup(replacement);
});

test('re-pair preparation fences overlap and close waits without committing a credential', async t => {
  const hold = Promise.withResolvers<void>(); t.after(() => hold.resolve());
  const h = await fixture(t); await h.disconnect(); const next = h.prepare(); h.runtime.hold = hold.promise;
  const completion = h.service.completeOwnerSetup(next);
  const rejected = assert.rejects(completion, /remains paused/);
  for (let i = 0; h.runtime.restarts === 0 && i < 100; i++) await new Promise(resolve => setTimeout(resolve, 5));
  assert.equal(h.runtime.restarts, 1);
  assert.throws(h.prepare, /running local host/);
  await assert.rejects(h.service.completeOwnerSetup(next), /running local host/);
  const { deviceToken, ...identity } = next;
  await assert.rejects(h.service.cancelOwnerSetup(identity), /available local host/);
  let closed = false; const closing = h.service.close().then(() => { closed = true; });
  await new Promise(resolve => setTimeout(resolve, 20)); assert.equal(closed, false);
  hold.resolve(); await rejected; await closing;
  assert.equal(h.runtime.stopped, true); assert.equal(h.runtime.disposals, 2);
  await h.reopen();
  assert.deepEqual(h.service.inspectSession().deviceIDs, []);
  assert.equal((await h.api(deviceToken, '/v1/installation')).status, 401);
  assert.equal(h.service.beginOwnerSetup().kind, 'available');
  await h.service.completeOwnerSetup(next);
  assert.equal((await h.api(deviceToken, '/v1/installation')).status, 200);
});
