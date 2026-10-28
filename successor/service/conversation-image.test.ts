import test, { type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, realpath, rm } from 'node:fs/promises';
import { randomBytes, randomUUID } from 'node:crypto';
import { createApplication } from './openai.ts';
import type { ChatRuntime } from '../runner/chat-runtime.ts';
import type { PortraitRequest, PortraitResult } from '../contracts/portrait.ts';
import type { Agent, ConversationImage, Message, RuntimeDescriptor, Snapshot } from '../contracts/v1.ts';
import { portraitPng } from '../runner/fixtures/portrait-png.ts';

// Spec 067: the owner asks an Agent for an image in a conversation; Fort draws it on the portrait run and posts it as the Agent's reply.
const descriptor: RuntimeDescriptor = { provider: 'codex', name: 'Codex fixture', runtimeVersion: '0.163.0', hostId: 'fixture-host', session: 'stateless', capabilities: ['plainText', 'modelCatalog'] };
const account = { accountId: 'workspace-fixture', userLabel: 'owner@fixture.invalid' };

async function fixture(t: TestContext, options: { draw?: (request: PortraitRequest) => Promise<PortraitResult>; portraits?: boolean; signedIn?: boolean } = {}) {
  const directory = await realpath(await mkdtemp('/tmp/fort067-image-'));
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
  const messages = async () => (await api<Snapshot>('/v1/conversations/' + agent.homeConversationId + '/snapshot')).body.messages;
  const settled = async (requestId: string): Promise<Message> => {
    for (let i = 0; i < 100; i++) {
      const reply = (await messages()).find(message => message.requestId === requestId && message.role === 'assistant');
      if (reply) return reply;
      await new Promise(resolve => setTimeout(resolve, 10));
    }
    throw new Error('Image never settled');
  };
  const ask = (prompt: unknown, requestId: string = randomUUID(), agentId = agent.agentId) => api<{ message: Message; error?: { code: string } }>('/v1/conversations/' + agent.homeConversationId + '/images', { requestId, agentId, prompt });
  return { api, agent, requests, chats, settled, ask, messages, get service() { return service; }, async reopen() { await service.close(); service = make(); await service.start(); } };
}

test('asking for an image draws it on the Agent’s model and account and posts it as the reply', async t => {
  const h = await fixture(t);
  assert.ok((await h.api<{ capabilities: string[] }>('/v1/installation')).body.capabilities.includes('conversationImages'));
  const requestId = randomUUID(), asked = await h.ask('  A lighthouse at dusk, watercolour  ', requestId);
  assert.equal(asked.status, 202);
  assert.deepEqual([asked.body.message.role, asked.body.message.text, asked.body.message.drawsImage, asked.body.message.recipientAgentId], ['user', '/image A lighthouse at dusk, watercolour', true, h.agent.agentId]);
  const reply = await h.settled(requestId);
  assert.deepEqual([reply.agentId, reply.image?.width, reply.image?.height], [h.agent.agentId, 256, 256]);
  assert.deepEqual(h.requests, [{ account, model: 'gpt-6-astra', prompt: 'A lighthouse at dusk, watercolour' }]);
  const image = await h.api<ConversationImage>('/v1/images/' + reply.image!.imageId);
  assert.equal(image.status, 200);
  assert.ok(Buffer.from(image.body.png, 'base64').equals(portraitPng()));
  // Retrying the same request doesn't draw again.
  assert.equal((await h.ask('A lighthouse at dusk, watercolour', requestId)).status, 202);
  assert.equal(h.requests.length, 1);
  assert.equal((await h.api('/v1/images/' + randomUUID())).status, 404);
});

test('a failed image is the Agent’s reply, and nothing is stored', async t => {
  const h = await fixture(t, { draw: async () => ({ kind: 'failed', reason: 'usageLimit', message: 'Your ChatGPT plan image limit was reached. Nothing else was tried.' }) });
  const requestId = randomUUID();
  await h.ask('A red bicycle', requestId);
  const reply = await h.settled(requestId);
  assert.equal(reply.image, undefined);
  assert.equal(reply.text, 'I couldn’t draw that image. Your ChatGPT plan image limit was reached. Nothing else was tried.');
  assert.equal(h.requests.length, 1);
});

test('one draw at a time, shared with portraits, and chat waits until it finishes', async t => {
  const release = Promise.withResolvers<void>();
  const h = await fixture(t, { draw: async () => { await release.promise; return { kind: 'drawn', png: portraitPng(), width: 256, height: 256 }; } });
  const first = randomUUID();
  assert.equal((await h.ask('A red bicycle', first)).status, 202);
  assert.equal((await h.ask('A blue bicycle')).body.error?.code, 'imageDrawing');
  assert.equal((await h.api<{ error: { code: string } }>('/v1/agents/' + h.agent.agentId + '/portrait', {})).body.error.code, 'portraitDrawing');
  assert.equal((await h.api('/v1/conversations/' + h.agent.homeConversationId + '/messages', { requestId: randomUUID(), text: 'hello', recipientAgentId: h.agent.agentId,
    bindingRevisionId: h.agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] })).status, 201);
  assert.equal(await h.service.dispatchNext(), false);
  release.resolve(); await h.settled(first);
  while (await h.service.dispatchNext());
  assert.deepEqual(h.chats, ['hello']);
  assert.equal(h.requests.length, 1);
});

test('nothing is drawn without a signed-in account, an image runtime, a participant Agent or a usable prompt', async t => {
  const unsigned = await fixture(t, { signedIn: false });
  assert.equal((await unsigned.ask('A red bicycle')).body.error?.code, 'signInRequired');
  const claude = await fixture(t, { portraits: false });
  assert.ok(!(await claude.api<{ capabilities: string[] }>('/v1/installation')).body.capabilities.includes('conversationImages'));
  assert.equal((await claude.ask('A red bicycle')).body.error?.code, 'imagesUnavailable');
  const h = await fixture(t);
  for (const prompt of ['', '   ', 'x'.repeat(2001), 42]) assert.equal((await h.ask(prompt)).status, 400);
  assert.equal((await h.ask('A red bicycle', randomUUID(), randomUUID())).status, 409);
  assert.equal((await h.api('/v1/conversations/' + h.agent.homeConversationId + '/images', { requestId: randomUUID(), agentId: h.agent.agentId, prompt: 'x', model: 'other' })).status, 400);
  assert.equal(unsigned.requests.length + claude.requests.length + h.requests.length, 0);
  assert.deepEqual(await h.messages(), []);
});

test('a restart mid-draw posts a reply saying the image did not finish', async t => {
  const stuck = await fixture(t, { draw: () => new Promise(() => {}) });
  const requestId = randomUUID();
  await stuck.ask('A red bicycle', requestId);
  await stuck.reopen();
  const reply = await stuck.settled(requestId);
  assert.equal(reply.image, undefined);
  assert.match(reply.text, /restarted before the image finished/);
  assert.equal((await stuck.ask('Another', randomUUID())).status, 202);
});
