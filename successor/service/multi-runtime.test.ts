import test, { type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, realpath, rm } from 'node:fs/promises';
import { randomBytes, randomUUID } from 'node:crypto';
import { createApplication } from './openai.ts';
import type { ChatInput, ChatRuntime } from '../runner/chat-runtime.ts';
import type { Agent, Installation, RuntimeDescriptor } from '../contracts/v1.ts';

// Spec 064: one host serves several runtimes. Each Agent's binding names its runtime, and its turns reach only that one.
const descriptor = (name: string): RuntimeDescriptor => ({ provider: name, name: name + ' fixture', runtimeVersion: name + '-1', hostId: name + '-host', session: 'stateless', capabilities: ['plainText', 'modelCatalog'] });
const account = { accountId: 'beta-account', userLabel: 'owner@beta.invalid' };

function runtime(name: string, signIn = false) {
  const seen: ChatInput[] = [];
  const value: ChatRuntime = {
    async execute(input) { seen.push(input); return { kind: 'completed', text: name + ' reply', effectiveModel: null }; },
    async models() { return { provider: name, state: 'available', models: [{ id: name + '-model', name: name + ' model', availability: 'available' }] } as never; },
    async dispose() {},
    ...(signIn ? { async signIn() { return { kind: 'signedIn' as const, account }; } } : {}),
  };
  return { value, seen };
}

async function fixture(t: TestContext, extra: { beta?: boolean; betaSignIn?: boolean } = { beta: true }) {
  const directory = await realpath(await mkdtemp('/tmp/fort064-runtimes-'));
  const alpha = runtime('alpha'), beta = runtime('beta', extra.betaSignIn);
  const options = (withBeta: boolean) => ({ provider: descriptor('alpha'), runtime: alpha.value, dataDirectory: directory, ownerPairingPolicy: { expiresAfterDays: null },
    runtimes: withBeta ? [{ provider: descriptor('beta'), runtime: beta.value }] : [] });
  let service = createApplication(options(extra.beta !== false)); await service.start();
  t.after(async () => { await service.close(); await rm(directory, { recursive: true, force: true }); });
  const host = { deviceId: randomUUID(), deviceToken: randomBytes(32).toString('base64url') };
  const setup = service.beginOwnerSetup(); if (setup.kind !== 'available') throw new Error('Synthetic setup missing');
  await service.completeOwnerSetup({ ...host, installationId: setup.installationId, setupId: setup.setupId });
  async function api<T = Record<string, unknown>>(path: string, body?: unknown) {
    const response = await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers: { 'Content-Type': 'application/json', 'X-Fort-Installation-Id': service.installation.installationId, Authorization: 'Bearer ' + host.deviceToken }, body: body === undefined ? undefined : JSON.stringify(body) });
    return { status: response.status, body: await response.json() as T };
  }
  const send = (agent: Agent, text: string) => api('/v1/conversations/' + agent.homeConversationId + '/messages', { requestId: randomUUID(), text, recipientAgentId: agent.agentId,
    bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: service.installation.recoveryEpoch, skills: [] });
  const snapshot = async (agent: Agent) => (await api<{ messages: { text: string }[]; runs?: { state: string; statusDetail?: string }[] }>('/v1/conversations/' + agent.homeConversationId + '/snapshot')).body;
  return { api, send, snapshot, alpha, beta, get service() { return service; },
    async reopen(withBeta: boolean) { await service.close(); service = createApplication(options(withBeta)); await service.start(); } };
}

test('each Agent runs only on the runtime its binding names', async t => {
  const h = await fixture(t);
  const installation = (await h.api<Installation & { runtimes?: RuntimeDescriptor[] }>('/v1/installation')).body;
  assert.equal(installation.runtime?.provider, 'alpha');
  assert.deepEqual(installation.runtimes?.map(runtime => runtime.provider), ['alpha', 'beta']);
  const first = (await h.api<Agent>('/v1/agents', { name: 'Alpha agent', model: 'alpha-model', instructions: 'Synthetic' })).body;
  const second = (await h.api<Agent>('/v1/agents', { name: 'Beta agent', model: 'beta-model', instructions: 'Synthetic', provider: 'beta' })).body;
  assert.deepEqual([first.binding.provider, first.binding.hostId], ['alpha', 'alpha-host']);
  assert.deepEqual([second.binding.provider, second.binding.runtimeVersion, second.binding.hostId], ['beta', 'beta-1', 'beta-host']);
  assert.equal((await h.api('/v1/agents', { name: 'Nowhere', model: 'x', instructions: 'Synthetic', provider: 'gamma' })).status, 400);
  assert.equal((await h.send(first, 'to alpha')).status, 201); assert.equal((await h.send(second, 'to beta')).status, 201);
  while (await h.service.dispatchNext());
  assert.deepEqual(h.alpha.seen.map(input => input.text), ['to alpha']);
  assert.deepEqual(h.beta.seen.map(input => input.text), ['to beta']);
  assert.deepEqual((await h.snapshot(second)).messages.map(message => message.text), ['to beta', 'beta reply']);
  // Editing an Agent keeps its provider.
  const edited = (await h.api<Agent>('/v1/agents/' + second.agentId, { expectedBindingRevisionId: second.binding.bindingRevisionId, name: 'Beta agent', model: 'beta-model-2', instructions: 'Synthetic' })).body;
  assert.equal(edited.binding.provider, 'beta');
  assert.equal((await h.api<{ provider: string }>('/v1/models?provider=beta')).body.provider, 'beta');
  assert.equal((await h.api<{ provider: string }>('/v1/models')).body.provider, 'alpha');
  assert.equal((await h.api('/v1/models?provider=gamma')).status, 400);
});

test('an Agent whose runtime is not on this host is refused and its work never reaches another runtime', async t => {
  const h = await fixture(t);
  const second = (await h.api<Agent>('/v1/agents', { name: 'Beta agent', model: 'beta-model', instructions: 'Synthetic', provider: 'beta' })).body;
  await h.reopen(false);
  const installation = (await h.api<{ runtimes?: RuntimeDescriptor[] }>('/v1/installation')).body;
  assert.deepEqual(installation.runtimes?.map(runtime => runtime.provider), ['alpha']);
  assert.equal((await h.send(second, 'to beta')).status, 409);
  assert.equal((await h.api('/v1/agents', { name: 'Beta again', model: 'beta-model', instructions: 'Synthetic', provider: 'beta' })).status, 400);
  // Work queued while the runtime was present is held after restart, as all queued work is, and never reaches another runtime.
  await h.reopen(true);
  assert.equal((await h.send(second, 'queued for beta')).status, 201);
  await h.reopen(false);
  while (await h.service.dispatchNext());
  assert.equal(h.beta.seen.length, 0); assert.equal(h.alpha.seen.length, 0);
  assert.equal((await h.snapshot(second)).runs?.at(-1)?.state, 'queued');
});

test('each runtime keeps its own account sign-in and choice', async t => {
  const h = await fixture(t, { beta: true, betaSignIn: true });
  assert.equal((await h.api<{ state: string }>('/v1/account/sign-in')).body.state, 'unavailable');
  assert.equal((await h.api<{ state: string }>('/v1/account/sign-in?provider=beta')).body.state, 'idle');
  const first = (await h.api<Agent>('/v1/agents', { name: 'Alpha agent', model: 'alpha-model', instructions: 'Synthetic' })).body;
  const second = (await h.api<Agent>('/v1/agents', { name: 'Beta agent', model: 'beta-model', instructions: 'Synthetic', provider: 'beta' })).body;
  assert.equal((await h.send(first, 'alpha needs no account')).status, 201);
  assert.equal((await h.send(second, 'beta needs its account')).status, 409);
  assert.equal((await h.api<{ state: string }>('/v1/models?provider=beta')).body.state, 'unavailable');
  const request = { operationId: randomUUID(), expectedRevision: 0, recoveryEpoch: h.service.installation.recoveryEpoch, expiresAt: Date.now() + 30_000 };
  assert.equal((await h.api('/v1/account/sign-in/start?provider=beta', request)).status, 200);
  for (let i = 0; i < 50 && (await h.api<{ state: string }>('/v1/account/sign-in?provider=beta')).body.state !== 'finished'; i++) await new Promise(resolve => setTimeout(resolve, 10));
  assert.equal((await h.api('/v1/account/sign-in/select?provider=beta', { operationId: request.operationId, revision: 1, recoveryEpoch: request.recoveryEpoch, account })).status, 200);
  assert.equal((await h.api<{ state: string }>('/v1/account/sign-in')).body.state, 'unavailable');
  assert.equal((await h.send(second, 'beta with its account')).status, 201);
  while (await h.service.dispatchNext());
  assert.deepEqual(h.beta.seen.map(input => [input.text, input.configuration.account?.accountId]), [['beta with its account', 'beta-account']]);
  assert.deepEqual(h.alpha.seen.map(input => [input.text, input.configuration.account]), [['alpha needs no account', undefined]]);
  await h.reopen(true);
  assert.equal((await h.api<{ selected?: boolean }>('/v1/account/sign-in?provider=beta')).body.selected, true);
});

test('a host with one runtime behaves as before', async t => {
  const h = await fixture(t, { beta: false });
  assert.deepEqual((await h.api<{ runtimes?: RuntimeDescriptor[] }>('/v1/installation')).body.runtimes?.map(runtime => runtime.provider), ['alpha']);
  const agent = (await h.api<Agent>('/v1/agents', { name: 'Alpha agent', model: 'alpha-model', instructions: 'Synthetic', provider: 'alpha' })).body;
  assert.equal(agent.binding.provider, 'alpha');
  assert.equal((await h.send(agent, 'hello')).status, 201);
  while (await h.service.dispatchNext());
  assert.deepEqual(h.alpha.seen.map(input => input.text), ['hello']);
});
