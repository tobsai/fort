import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import type { Acceptance, AccountSignInStatus, Agent, RuntimeDescriptor } from '../contracts/v1.ts';
import type { ChatRuntime } from '../runner/chat-runtime.ts';
import { connectToHub } from '../runner/hub-runner.ts';
import { migrateLedger } from '../service/migrate-ledger.ts';
import { uploadLedger } from '../runner/upload-ledger.ts';
import { startCloudHub } from './cloud-hub.ts';

const secret = () => randomBytes(32).toString('base64url');
const setupCode = secret(), setupCodeDigest = createHash('sha256').update(setupCode).digest('hex');
const echo = (seen: string[] = []): ChatRuntime => ({ async execute(input) { seen.push(input.text); return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() {} });

// One Fort ledger served for the Mac named hostId: the hub fixture from cloud-hub.test.ts, for either side.
async function fixture(t: TestContext, hostId: string) {
  const provider: RuntimeDescriptor = { provider: 'remote-fixture', name: 'Remote fixture', runtimeVersion: 'fixture-1', hostId, session: 'stateless', capabilities: ['plainText'] };
  const dataDirectory = await mkdtemp(join(tmpdir(), 'fort-hub-import-'));
  const open = () => startCloudHub({ dataDirectory, host: '127.0.0.1', port: 0, provider, setupCodeDigest });
  let hub = await open(), closed = false;
  t.after(async () => { if (!closed) await hub.close(); await rm(dataDirectory, { recursive: true, force: true }); });
  const owner = { deviceId: randomUUID(), deviceToken: secret() };
  let installationId = '', runnerToken = '';
  async function call<T = Record<string, unknown>>(path: string, body?: unknown, headers: Record<string, string> = { authorization: 'Bearer ' + owner.deviceToken }) {
    const response = await fetch(hub.url + path, { method: body === undefined ? 'GET' : 'POST', headers: { 'content-type': 'application/json', 'x-fort-installation-id': installationId, ...headers },
      body: body === undefined ? undefined : body instanceof Uint8Array ? new Blob([new Uint8Array(body)]) : JSON.stringify(body) });
    const text = await response.text();
    return { status: response.status, body: (text ? JSON.parse(text) : {}) as T };
  }
  let runtime: ChatRuntime = echo();
  // The Mac's runner connection; a restarted hub needs it to connect again.
  async function connect() {
    const stop = new AbortController();
    void connectToHub({ url: hub.url, token: runnerToken, binding: { provider: 'remote-fixture', runtimeVersion: 'fixture-1', hostId }, createRuntime: () => runtime, signal: stop.signal }).catch(() => {});
    t.after(() => stop.abort());
    for (let index = 0; index < 200 && !(await call<{ runners: { online: boolean }[] }>('/v1/runners')).body.runners[0]?.online; index++) await delay(10);
  }
  async function ready(seen: string[] = []) {
    const setup = await call<{ installationId: string }>('/v1/hub/setup', { setupCode, ...owner }, {});
    assert.equal(setup.status, 201);
    installationId = setup.body.installationId;
    const invitation = { invitationId: randomUUID(), code: secret() };
    runnerToken = secret();
    assert.equal((await call('/v1/runners/invitation', invitation)).status, 201);
    assert.equal((await call('/v1/runners/join', { ...invitation, runnerId: hostId, runnerToken, runnerName: 'Mac' }, {})).status, 201);
    runtime = { ...echo(seen), signIn: async () => ({ kind: 'signedIn', account: { accountId: 'account', userLabel: 'owner@example.com' } }) };
    await connect();
    const idle = (await call<AccountSignInStatus>('/v1/account/sign-in')).body;
    assert.equal((await call('/v1/account/sign-in/start', { operationId: randomUUID(), expectedRevision: idle.revision, recoveryEpoch: hub.installation.recoveryEpoch, expiresAt: Date.now() + 60_000 })).status, 200);
    for (let index = 0; index < 200 && (await call<AccountSignInStatus>('/v1/account/sign-in')).body.state !== 'finished'; index++) await delay(10);
  }
  async function send(agent: Agent, text: string) {
    const accepted = await call<Acceptance>(`/v1/conversations/${agent.homeConversationId}/messages`, { requestId: randomUUID(), text, recipientAgentId: agent.agentId,
      bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: hub.installation.recoveryEpoch, skills: [] });
    assert.equal(accepted.status, 201);
    const requestId = accepted.body.receipt.requestId;
    for (let index = 0; index < 300; index++) {
      const run = (await call<Acceptance>('/v1/requests/' + requestId)).body.snapshot.runs.find(row => row.requestId === requestId);
      if (run && ['completed', 'failed', 'outcomeUnknown', 'notStarted'].includes(run.state)) return { requestId, state: run.state };
      await delay(20);
    }
    assert.fail('The run did not settle');
  }
  return { call, ready, send, connect, dataDirectory, get url() { return hub.url; }, get runnerToken() { return runnerToken; },
    async restart() { await hub.close(); hub = await open(); }, async close() { closed = true; await hub.close(); } };
}

// The Mac side: a ledger with one agent and a finished chat, snapshotted the way the Mac exports it.
async function macSnapshot(t: TestContext) {
  const mac = await fixture(t, 'old-mac');
  await mac.ready();
  const agent = (await mac.call<Agent>('/v1/agents', { name: 'Scout', model: 'exact-model', instructions: 'Echo.' })).body;
  const first = await mac.send(agent, 'remember the cedar 🌲');
  assert.equal(first.state, 'completed');
  await mac.close();
  const export_ = join(mac.dataDirectory, 'export');
  await migrateLedger({ source: join(mac.dataDirectory, 'openai-candidate-b.sqlite'), destination: export_ });
  return { agent, requestId: first.requestId, ledger: join(mac.dataDirectory, 'openai-candidate-b.sqlite'), bytes: await readFile(join(export_, 'openai-candidate-b.sqlite')) };
}

test('a Mac’s agents and chats move to the hub, keep their history, and run on that Mac', async t => {
  const source = await macSnapshot(t), seen: string[] = [];
  const hub = await fixture(t, 'mac-mini');
  await hub.ready(seen);
  const machine = { authorization: 'Bearer ' + hub.runnerToken, 'content-type': 'application/vnd.sqlite3' };
  const staged = await hub.call<{ state: string; entities: Record<string, number> }>('/v1/runners/import', new Uint8Array(source.bytes), machine);
  assert.equal(staged.status, 202);
  assert.equal(staged.body.state, 'staged');
  assert.equal(staged.body.entities.agents, 1);
  await hub.restart();
  const status = await hub.call<{ state: string; rebound: number }>('/v1/runners/import', undefined, machine);
  assert.equal(status.body.state, 'applied');
  assert.equal(status.body.rebound, 1);

  const agents = (await hub.call<{ agents: Agent[] }>('/v1/agents')).body.agents;
  const scout = agents.find(agent => agent.agentId === source.agent.agentId);
  assert.ok(scout, 'the Mac’s agent is on the hub');
  assert.equal(scout.binding.hostId, 'mac-mini');
  assert.equal(scout.binding.bindingRevisionId, source.agent.binding.bindingRevisionId);
  const history = (await hub.call<Acceptance>('/v1/requests/' + source.requestId)).body.snapshot;
  assert.deepEqual(history.messages.map(message => message.text), ['remember the cedar 🌲', 'remember the cedar 🌲']);

  await hub.connect();
  const next = await hub.send(scout, 'still here?');
  assert.equal(next.state, 'completed');
  assert.deepEqual(seen, ['still here?']);
  assert.equal((await hub.call('/v1/runners/import', new Uint8Array(source.bytes), machine)).status, 409);
});

test('only the hub’s own Mac can import, and a bad snapshot changes nothing', async t => {
  const source = await macSnapshot(t);
  const hub = await fixture(t, 'mac-mini');
  await hub.ready();
  const as = (token: string) => ({ authorization: 'Bearer ' + token, 'content-type': 'application/vnd.sqlite3' });
  assert.equal((await hub.call('/v1/runners/import', new Uint8Array(source.bytes), as(secret()))).status, 401);
  assert.equal((await hub.call('/v1/runners/import', new Uint8Array(source.bytes), { ...as(hub.runnerToken), origin: 'https://evil.example' })).status, 403);
  const broken = await hub.call<{ error: { code: string } }>('/v1/runners/import', new Uint8Array(Buffer.from('not a ledger')), as(hub.runnerToken));
  assert.equal(broken.status, 422);
  await hub.restart();
  assert.equal((await hub.call<{ state: string }>('/v1/runners/import', undefined, as(hub.runnerToken))).body.state, 'none');
  assert.deepEqual((await hub.call<{ agents: Agent[] }>('/v1/agents')).body.agents, []);
});

test('agents made on the web stay beside the imported ones', async t => {
  const source = await macSnapshot(t);
  const hub = await fixture(t, 'mac-mini');
  await hub.ready();
  const machine = { authorization: 'Bearer ' + hub.runnerToken, 'content-type': 'application/vnd.sqlite3' };
  const mine = (await hub.call<Agent>('/v1/agents', { name: 'Web agent', model: 'exact-model', instructions: 'Hi.' })).body;
  assert.equal((await hub.call('/v1/runners/import', new Uint8Array(source.bytes), machine)).status, 202);
  await hub.restart();
  assert.equal((await hub.call<{ state: string }>('/v1/runners/import', undefined, machine)).body.state, 'applied');
  // A hub agent made on the web stays beside the imported one.
  const names = (await hub.call<{ agents: Agent[] }>('/v1/agents')).body.agents.map(agent => agent.name).sort();
  assert.deepEqual(names, ['Scout', 'Web agent']);
  assert.ok(mine.agentId);
});

test('the Mac rehearses the import locally, then sends its ledger with its runner token', async t => {
  const source = await macSnapshot(t);
  const hub = await fixture(t, 'mac-mini');
  await hub.ready();
  const state = join(hub.dataDirectory, 'mac-state');
  await mkdir(state, { mode: 0o700 });
  await writeFile(join(state, 'hub-runner.json'), JSON.stringify({ hubURL: hub.url, runnerId: 'mac-mini', token: hub.runnerToken }), { mode: 0o600 });

  const rehearsal = await uploadLedger({ ledger: source.ledger, stateDirectory: state, rehearse: true });
  assert.equal(rehearsal.state, 'rehearsed');
  assert.equal(rehearsal.rebound, 1);
  assert.deepEqual(rehearsal.rehearsal, rehearsal.entities);
  assert.deepEqual(rehearsal.bindings, { 'remote-fixture fixture-1': 1 });
  assert.deepEqual((await hub.call<{ agents: Agent[] }>('/v1/agents')).body.agents, []);

  const sent = await uploadLedger({ ledger: source.ledger, stateDirectory: state, waitMs: 1 });
  assert.equal(sent.state, 'staged');
  await hub.restart();
  assert.equal((await hub.call<{ agents: Agent[] }>('/v1/agents')).body.agents[0]?.binding.hostId, 'mac-mini');
});
