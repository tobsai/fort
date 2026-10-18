import assert from 'node:assert/strict';
import { test, type TestContext } from 'node:test';
import { mkdtemp, rm } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { createApplication } from './openai.ts';
import type { ChatInput, ChatRuntime } from '../runner/chat-runtime.ts';
import type { Agent, RuntimeReadiness, Snapshot } from '../contracts/v1.ts';

class ResumeRuntime implements ChatRuntime {
  stopped = false;
  confirmed = false;
  disposals = 0;
  restarts = 0;
  failStop = false;
  failRestart = false;
  waitForRestart: Promise<void> | undefined;
  async execute(input: ChatInput) {
    return this.stopped ? { kind: 'notStarted' as const, message: 'Synthetic runtime stopped.' }
      : { kind: 'completed' as const, text: input.text, effectiveModel: null };
  }
  async dispose() {
    this.disposals++; this.stopped = true;
    if (this.failStop) throw new Error('Synthetic private disposal detail');
    this.confirmed = true;
  }
  readiness(): RuntimeReadiness {
    return { state: this.stopped ? 'stopped' : 'ready', canRestart: this.stopped && this.confirmed, detail: 'Synthetic lifecycle.' };
  }
  async restart(signal: AbortSignal) {
    signal.throwIfAborted(); this.restarts++;
    await this.waitForRestart;
    signal.throwIfAborted();
    if (this.failRestart) throw new Error('Synthetic private restart detail');
    this.stopped = false; this.confirmed = false;
  }
}

async function fixture(t: TestContext, owner = true, now = Date.now) {
  const directory = await mkdtemp(join(tmpdir(), 'fort-owner-resume-'));
  let runtime = new ResumeRuntime();
  const create = () => createApplication({ dataDirectory: directory, runtime, now,
    provider: { provider: 'resume-fixture', name: 'Resume fixture', runtimeVersion: '1', hostId: 'synthetic-host', session: 'stateless', capabilities: ['plainText'] },
    ...(owner ? { ownerPairingPolicy: { expiresAfterDays: null } } : { credentialDeadline: Date.now() + 60_000 }) });
  let service = create(); await service.start();
  t.after(async () => { runtime.failStop = false; await service.close(); await rm(directory, { recursive: true, force: true }); });
  const enrolled = await fetch(service.baseURL + '/v1/devices/enroll', { method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...service.issueEnrollment(), deviceName: 'Synthetic resume device' }) });
  assert.equal(enrolled.status, 201);
  const device = await enrolled.json() as { deviceToken: string; deviceId: string };
  const headers = { 'Content-Type': 'application/json', Authorization: 'Bearer ' + device.deviceToken, 'X-Fort-Installation-Id': service.installation.installationId };
  async function api<T>(path: string, body?: unknown, override: Record<string, string> = {}) {
    const response = await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers: { ...headers, ...override }, body: body === undefined ? undefined : JSON.stringify(body) });
    return { status: response.status, body: await response.json() as T };
  }
  return { api, device, headers, get service() { return service; }, get runtime() { return runtime; }, async reopen() {
    await service.close(); runtime = new ResumeRuntime(); service = create(); await service.start();
  } };
}

test('owner stop keeps control and history available until explicit authenticated resume', async t => {
  const h = await fixture(t);
  const agent = (await h.api<Agent>('/v1/agents', { name: 'Retained', instructions: 'Echo.', model: 'exact' })).body;
  const path = '/v1/conversations/' + agent.homeConversationId;
  const send = (requestId: string) => h.api(path + '/messages', { requestId, text: ' exact 🌲\n', recipientAgentId: agent.agentId,
    bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] });
  assert.equal((await send('before')).status, 201); await h.service.dispatchNext();
  const history = (await h.api<Snapshot>(path + '/snapshot')).body.messages;
  assert.equal((await h.api('/v1/host/stop', {})).status, 200);
  assert.equal((await h.api('/v1/agents')).status, 200);
  assert.deepEqual((await h.api<Snapshot>(path + '/snapshot')).body.messages, history);
  assert.equal((await send('blocked')).status, 503);
  const paused = (await h.api<RuntimeReadiness>('/v1/runtime')).body;
  assert.equal(paused.state, 'stopped'); assert.equal(paused.canRestart, true);
  assert.equal((await h.api('/v1/runtime/restart', { recoveryEpoch: 'foreign' })).status, 409);
  assert.equal(h.runtime.restarts, 0);
  const resumed = await h.api<RuntimeReadiness>('/v1/runtime/restart', { recoveryEpoch: h.service.installation.recoveryEpoch });
  assert.equal(resumed.status, 200); assert.equal(resumed.body.state, 'ready');
  assert.equal(h.runtime.restarts, 1); assert.equal(h.runtime.disposals, 1);
  assert.equal((await send('after')).status, 201); await h.service.dispatchNext();
  assert.equal((await h.api<Snapshot>(path + '/snapshot')).body.messages.length, history.length + 2);
});

test('reopening a stopped owner host stays paused until the exact explicit resume', async t => {
  const h = await fixture(t);
  assert.equal((await h.api('/v1/host/stop', {})).status, 200);
  await h.reopen();
  assert.equal(h.runtime.stopped, false, 'Fresh runtime is inert until dispatch');
  const paused = (await h.api<RuntimeReadiness>('/v1/runtime')).body;
  assert.equal(paused.state, 'stopped'); assert.equal(paused.canRestart, true);
  assert.equal((await h.api('/v1/agents', { name: 'Blocked', model: 'exact', instructions: '' })).status, 503);
  assert.equal((await h.api('/v1/runtime/restart', { recoveryEpoch: h.service.installation.recoveryEpoch })).status, 200);
  assert.equal(h.runtime.restarts, 0, 'A fresh selected runtime needs no replacement');
  assert.equal((await h.api('/v1/agents', { name: 'Allowed', model: 'exact', instructions: 'Synthetic.' })).status, 201);
});

test('auth, exact installation, browser origin and disconnect remain resume barriers', async t => {
  const h = await fixture(t), body = { recoveryEpoch: h.service.installation.recoveryEpoch };
  assert.equal((await h.api('/v1/host/stop', {})).status, 200);
  const rejectedHeaders: Array<Record<string, string>> = [{ Authorization: 'Bearer foreign' }, { 'X-Fort-Installation-Id': 'foreign' }, { Origin: 'https://untrusted.invalid' }];
  for (const headers of rejectedHeaders) {
    const response = await h.api('/v1/runtime/restart', body, headers);
    assert.ok([401, 403].includes(response.status));
  }
  assert.equal((await h.api('/v1/runtime/restart', { ...body, unexpected: true })).status, 400);
  assert.equal(h.runtime.restarts, 0);
  assert.equal((await h.api('/v1/pairing/disconnect', {})).status, 200);
  assert.equal((await h.api('/v1/runtime/restart', body)).status, 401);
  assert.equal(h.runtime.restarts, 0);
  const demo = await fixture(t, false);
  assert.equal((await demo.api('/v1/host/stop', {})).status, 403);
  assert.equal(demo.runtime.disposals, 0);
});

test('failed shutdown cannot be mistaken for a fresh paused runtime', async t => {
  const h = await fixture(t); h.runtime.failStop = true;
  assert.equal((await h.api('/v1/host/stop', {})).status, 503);
  assert.equal((await h.api<RuntimeReadiness>('/v1/runtime')).body.canRestart, false);
  assert.equal((await h.api('/v1/runtime/restart', { recoveryEpoch: h.service.installation.recoveryEpoch })).status, 409);
  assert.equal(h.runtime.restarts, 0); assert.equal(h.runtime.disposals, 1);
  h.runtime.failStop = false;
  assert.equal((await h.api('/v1/host/stop', {})).status, 200);
  assert.equal((await h.api('/v1/runtime/restart', { recoveryEpoch: h.service.installation.recoveryEpoch })).status, 200);
});

test('failed resume cleans its runtime and retains stopped intent without exposing private details', async t => {
  const h = await fixture(t); await h.api('/v1/host/stop', {}); h.runtime.failRestart = true;
  const result = await h.api('/v1/runtime/restart', { recoveryEpoch: h.service.installation.recoveryEpoch });
  assert.equal(result.status, 503); assert.ok(!JSON.stringify(result).includes('Synthetic private'));
  assert.equal(h.runtime.stopped, true); assert.equal(h.runtime.disposals, 2);
  await h.reopen(); assert.equal((await h.api<RuntimeReadiness>('/v1/runtime')).body.state, 'stopped');
});

test('overlapping resume, stop and disconnect cannot duplicate or supersede an in-flight resume', async t => {
  const hold = Promise.withResolvers<void>();
  t.after(() => hold.resolve());
  const h = await fixture(t); await h.api('/v1/host/stop', {}); h.runtime.waitForRestart = hold.promise;
  const body = { recoveryEpoch: h.service.installation.recoveryEpoch };
  const resumed = h.api('/v1/runtime/restart', body);
  for (let i = 0; h.runtime.restarts === 0 && i < 100; i++) await new Promise(resolve => setTimeout(resolve, 5));
  assert.equal(h.runtime.restarts, 1);
  assert.equal((await h.api('/v1/runtime/restart', body)).status, 409);
  assert.equal((await h.api('/v1/host/stop', {})).status, 409);
  assert.equal((await h.api('/v1/pairing/disconnect', {})).status, 409);
  assert.equal((await h.api<RuntimeReadiness>('/v1/runtime')).body.canRestart, false);
  hold.resolve(); assert.equal((await resumed).status, 200);
  assert.equal((await h.api('/v1/runtime/restart', body)).status, 409);
  assert.equal(h.runtime.restarts, 1);
});

test('close waits for an in-flight resume and leaves persisted owner intent paused', async t => {
  const hold = Promise.withResolvers<void>();
  t.after(() => hold.resolve());
  const h = await fixture(t); await h.api('/v1/host/stop', {}); h.runtime.waitForRestart = hold.promise;
  const resumed = h.api('/v1/runtime/restart', { recoveryEpoch: h.service.installation.recoveryEpoch }).catch(() => undefined);
  for (let i = 0; h.runtime.restarts === 0 && i < 100; i++) await new Promise(resolve => setTimeout(resolve, 5));
  assert.equal(h.runtime.restarts, 1);
  let closed = false;
  const closing = h.service.close().then(() => { closed = true; });
  await new Promise(resolve => setTimeout(resolve, 20)); assert.equal(closed, false);
  hold.resolve(); await closing; await resumed;
  assert.equal(h.runtime.stopped, true); assert.equal(h.runtime.disposals, 2);
  await h.reopen(); assert.equal((await h.api<RuntimeReadiness>('/v1/runtime')).body.state, 'stopped');
});

test('pairing expiry during resume prevents running intent and disposes the restarted runtime', async t => {
  const hold = Promise.withResolvers<void>();
  t.after(() => hold.resolve());
  let now = Date.now();
  const h = await fixture(t, true, () => now);
  assert.equal((await h.api('/v1/pairing/expiry', { expiresAfterDays: 1 })).status, 200);
  await h.api('/v1/host/stop', {}); h.runtime.waitForRestart = hold.promise;
  const resumed = h.api('/v1/runtime/restart', { recoveryEpoch: h.service.installation.recoveryEpoch });
  for (let i = 0; h.runtime.restarts === 0 && i < 100; i++) await new Promise(resolve => setTimeout(resolve, 5));
  assert.equal(h.runtime.restarts, 1);
  now += 86_400_001; hold.resolve();
  assert.equal((await resumed).status, 503);
  assert.equal(h.runtime.stopped, true); assert.equal(h.runtime.disposals, 2);
  assert.equal((await h.api('/v1/installation')).status, 401);
  assert.throws(() => h.service.issueEnrollment());
});
