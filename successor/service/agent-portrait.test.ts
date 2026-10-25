import test, { type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, realpath, rm } from 'node:fs/promises';
import { randomBytes, randomUUID } from 'node:crypto';
import { createApplication } from './openai.ts';
import type { ChatRuntime } from '../runner/chat-runtime.ts';
import type { PortraitRequest, PortraitResult } from '../contracts/portrait.ts';
import type { Agent, PortraitImage, RuntimeDescriptor } from '../contracts/v1.ts';
import { portraitPng } from '../runner/fixtures/portrait-png.ts';

// Spec 065: Fort draws an Agent's portrait on that Agent's own runtime and signed-in account, then serves it.
const descriptor: RuntimeDescriptor = { provider: 'codex', name: 'Codex fixture', runtimeVersion: '0.163.0', hostId: 'fixture-host', session: 'stateless', capabilities: ['plainText', 'modelCatalog'] };
const account = { accountId: 'workspace-fixture', userLabel: 'owner@fixture.invalid' };

async function fixture(t: TestContext, options: { draw?: (request: PortraitRequest) => Promise<PortraitResult>; portraits?: boolean; signedIn?: boolean } = {}) {
  const directory = await realpath(await mkdtemp('/tmp/fort065-portrait-'));
  const requests: PortraitRequest[] = [], chats: string[] = [];
  const runtime: ChatRuntime = {
    async execute(input) { chats.push(input.text); return { kind: 'completed', text: 'reply', effectiveModel: null }; },
    async dispose() {},
    async signIn() { return { kind: 'signedIn' as const, account }; },
    ...(options.portraits === false ? {} : { async portrait(request: PortraitRequest) { requests.push(request); return options.draw ? options.draw(request) : { kind: 'drawn' as const, png: portraitPng(), width: 256, height: 256 }; } }),
  };
  const make = () => createApplication({ provider: descriptor, runtime, dataDirectory: directory, ownerPairingPolicy: { expiresAfterDays: null } });
  let service = make(); await service.start();
  t.after(async () => { await service.close(); await rm(directory, { recursive: true, force: true }); });
  const host = { deviceId: randomUUID(), deviceToken: randomBytes(32).toString('base64url') };
  const setup = service.beginOwnerSetup(); if (setup.kind !== 'available') throw new Error('Synthetic setup missing');
  await service.completeOwnerSetup({ ...host, installationId: setup.installationId, setupId: setup.setupId });
  async function api<T = Record<string, unknown>>(path: string, body?: unknown, method = body === undefined ? 'GET' : 'POST') {
    const response = await fetch(service.baseURL + path, { method, headers: { 'Content-Type': 'application/json', 'X-Fort-Installation-Id': service.installation.installationId, Authorization: 'Bearer ' + host.deviceToken }, body: body === undefined ? undefined : JSON.stringify(body) });
    return { status: response.status, body: await response.json() as T };
  }
  if (options.signedIn !== false) {
    const request = { operationId: randomUUID(), expectedRevision: 0, recoveryEpoch: service.installation.recoveryEpoch, expiresAt: Date.now() + 30_000 };
    assert.equal((await api('/v1/account/sign-in/start', request)).status, 200);
    for (let i = 0; i < 50 && (await api<{ state: string }>('/v1/account/sign-in')).body.state !== 'finished'; i++) await new Promise(resolve => setTimeout(resolve, 10));
    assert.equal((await api('/v1/account/sign-in/select', { operationId: request.operationId, revision: 1, recoveryEpoch: request.recoveryEpoch, account })).status, 200);
  }
  const agent = (await api<Agent>('/v1/agents', { name: 'Pascal', model: 'gpt-6-astra', instructions: 'You ship Fort.' })).body;
  const settled = async () => {
    for (let i = 0; i < 100; i++) {
      const current = (await api<{ agents: Agent[] }>('/v1/agents')).body.agents.find(item => item.agentId === agent.agentId)!;
      if (current.portraitDraw?.state !== 'drawing') return current;
      await new Promise(resolve => setTimeout(resolve, 10));
    }
    throw new Error('Portrait never settled');
  };
  return { api, agent, requests, chats, settled, get service() { return service; }, async reopen() { await service.close(); service = make(); await service.start(); } };
}

test('drawing a portrait uses the Agent’s model and account, then serves the PNG', async t => {
  const h = await fixture(t);
  assert.ok((await h.api<{ capabilities: string[] }>('/v1/installation')).body.capabilities.includes('agentPortraits'));
  const started = await h.api<Agent>('/v1/agents/' + h.agent.agentId + '/portrait', { note: 'Waistcoat, short dark hair.' });
  assert.equal(started.status, 202);
  assert.equal(started.body.portraitDraw?.state, 'drawing');
  const agent = await h.settled();
  assert.equal(agent.portraitDraw, undefined);
  assert.equal(agent.portrait?.size, 256);
  assert.deepEqual(h.requests, [{ account, model: 'gpt-6-astra', subject: { name: 'Pascal', instructions: 'You ship Fort.', note: 'Waistcoat, short dark hair.' } }]);
  const image = await h.api<PortraitImage>('/v1/agents/' + h.agent.agentId + '/portrait');
  assert.equal(image.status, 200);
  assert.equal(image.body.revision, agent.portrait?.revision);
  assert.ok(Buffer.from(image.body.png, 'base64').equals(portraitPng()));
  // Editing the Agent keeps its portrait.
  const edited = (await h.api<Agent>('/v1/agents/' + h.agent.agentId, { expectedBindingRevisionId: agent.binding.bindingRevisionId, name: 'Pascal', model: 'gpt-6-astra', instructions: 'Still ships Fort.' })).body;
  assert.equal(edited.portrait?.revision, agent.portrait?.revision);
});

test('a failed draw is recorded on the Agent and the old portrait stays', async t => {
  let fail = false;
  const h = await fixture(t, { draw: async () => fail ? { kind: 'failed', reason: 'usageLimit', message: 'Your ChatGPT plan image limit was reached. Nothing else was tried.' } : { kind: 'drawn', png: portraitPng(), width: 256, height: 256 } });
  await h.api('/v1/agents/' + h.agent.agentId + '/portrait', {});
  const first = await h.settled();
  fail = true;
  await h.api('/v1/agents/' + h.agent.agentId + '/portrait', {});
  const second = await h.settled();
  assert.equal(second.portrait?.revision, first.portrait?.revision);
  assert.deepEqual([second.portraitDraw?.state, second.portraitDraw?.detail], ['failed', 'Your ChatGPT plan image limit was reached. Nothing else was tried.']);
  assert.equal(h.requests.length, 2);
});

test('one draw at a time, and chat waits until it finishes', async t => {
  const release = Promise.withResolvers<void>();
  const h = await fixture(t, { draw: async () => { await release.promise; return { kind: 'drawn', png: portraitPng(), width: 256, height: 256 }; } });
  assert.equal((await h.api('/v1/agents/' + h.agent.agentId + '/portrait', {})).status, 202);
  assert.equal((await h.api<{ error: { code: string } }>('/v1/agents/' + h.agent.agentId + '/portrait', {})).body.error.code, 'portraitDrawing');
  assert.equal((await h.api('/v1/conversations/' + h.agent.homeConversationId + '/messages', { requestId: randomUUID(), text: 'hello', recipientAgentId: h.agent.agentId,
    bindingRevisionId: h.agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] })).status, 201);
  assert.equal(await h.service.dispatchNext(), false);
  assert.deepEqual(h.chats, []);
  release.resolve(); await h.settled();
  while (await h.service.dispatchNext());
  assert.deepEqual(h.chats, ['hello']);
  assert.equal(h.requests.length, 1);
});

test('nothing is drawn without a signed-in account, a portrait runtime or a valid note', async t => {
  const unsigned = await fixture(t, { signedIn: false });
  assert.equal((await unsigned.api<{ error: { code: string } }>('/v1/agents/' + unsigned.agent.agentId + '/portrait', {})).body.error.code, 'signInRequired');
  const claude = await fixture(t, { portraits: false });
  assert.ok(!(await claude.api<{ capabilities: string[] }>('/v1/installation')).body.capabilities.includes('agentPortraits'));
  assert.equal((await claude.api<{ error: { code: string } }>('/v1/agents/' + claude.agent.agentId + '/portrait', {})).body.error.code, 'portraitUnavailable');
  assert.equal((await claude.api('/v1/agents/' + claude.agent.agentId + '/portrait', { note: 'x'.repeat(501) })).status, 400);
  assert.equal((await claude.api('/v1/agents/' + claude.agent.agentId + '/portrait', { prompt: 'anything' })).status, 400);
  assert.equal(unsigned.requests.length + claude.requests.length, 0);
});

test('removing a portrait deletes it, and a restart mid-draw is reported', async t => {
  const h = await fixture(t);
  await h.api('/v1/agents/' + h.agent.agentId + '/portrait', {});
  await h.settled();
  const removed = await h.api<Agent>('/v1/agents/' + h.agent.agentId + '/portrait/remove', {});
  assert.equal(removed.status, 200);
  assert.equal(removed.body.portrait, undefined);
  assert.equal((await h.api('/v1/agents/' + h.agent.agentId + '/portrait')).status, 404);

  const stuck = await fixture(t, { draw: () => new Promise(() => {}) });
  await stuck.api('/v1/agents/' + stuck.agent.agentId + '/portrait', {});
  await stuck.reopen();
  const after = (await stuck.api<{ agents: Agent[] }>('/v1/agents')).body.agents[0]!;
  assert.equal(after.portraitDraw?.state, 'failed');
  assert.match(after.portraitDraw?.detail ?? '', /restarted/);
});
