import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, stat } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createHash, generateKeyPairSync, randomBytes, randomUUID, sign } from 'node:crypto';
import type { Agent, InstructionSkill, RoutineRevision, RuntimeDescriptor, SchedulePreview, ScheduleRevision, Snapshot } from '../contracts/v1.ts';
import type { ChatRuntime } from '../runner/chat-runtime.ts';
import { createApplication } from '../service/openai.ts';
import { LOCAL_CONTEXT } from '../modules/local-context.ts';
import { enrollWithHub } from '../runner/hub-credential.ts';
import { readHubClient, syncAgentsToHub } from '../runner/agent-handoff.ts';
import { uploadLedger } from '../runner/upload-ledger.ts';
import { startCloudHub } from './cloud-hub.ts';

// Spec 063, revision 2026-10-31: once a Mac syncs, the hub owns its Agents. The Mac hands over definitions (never history),
// its own copies go read-only, and it keeps a client credential so the Mac app can use the hub's Agents.
const secret = () => randomBytes(32).toString('base64url');
const setupCode = secret(), setupCodeDigest = createHash('sha256').update(setupCode).digest('hex');
const echo: ChatRuntime = { async execute(input) { return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() {} };
const lane = (provider: string, hostId: string): RuntimeDescriptor => ({ provider, name: provider, runtimeVersion: provider + '-1', hostId, session: 'stateless', capabilities: ['plainText'] });

// The Mac's local core: ChatGPT-like lane the hub serves, and a second lane it doesn't.
async function mac(t: TestContext) {
  let clock = Date.UTC(2026, 9, 31, 9, 0);
  const directory = await mkdtemp(join(tmpdir(), 'fort-mac-handoff-'));
  const service = createApplication({ provider: lane('fixture', 'mac-local'), runtime: echo, runtimes: [{ provider: lane('elsewhere', 'mac-local'), runtime: echo }], context: LOCAL_CONTEXT, dataDirectory: directory, now: () => clock });
  await service.start();
  let closed = false;
  const close = async () => { if (!closed) { closed = true; await service.close(); } };
  t.after(async () => { await close(); await rm(directory, { recursive: true, force: true }); });
  const enrolled = await (await fetch(service.baseURL + '/v1/devices/enroll', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ ...service.issueEnrollment(), deviceName: 'Mac app' }) })).json() as { deviceToken: string };
  const headers = { 'content-type': 'application/json', authorization: 'Bearer ' + enrolled.deviceToken, 'x-fort-installation-id': service.installation.installationId };
  async function api<T = Record<string, any>>(path: string, body?: unknown) {
    const response = await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers, body: body === undefined ? undefined : JSON.stringify(body) });
    return { status: response.status, body: await response.json() as T };
  }
  const epoch = service.installation.recoveryEpoch;
  const send = (agent: Agent, text: string) => api(`/v1/conversations/${agent.homeConversationId}/messages`, { requestId: randomUUID(), text, recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: epoch, skills: [] });
  const scout = (await api<Agent>('/v1/agents', { name: 'Scout', model: 'exact-model', instructions: 'Echo.' })).body;
  const loner = (await api<Agent>('/v1/agents', { name: 'Loner', model: 'other-model', instructions: 'Echo.', provider: 'elsewhere' })).body;
  assert.equal((await send(scout, 'remember the cedar')).status, 201);
  await service.dispatchNext();
  const skill = (await api<InstructionSkill>('/v1/skills', { name: 'Brief', instructions: 'Be brief.' })).body;
  await api('/v1/skills/' + skill.skillId + '/review', { revisionId: skill.revisionId, digest: skill.bundle!.digest, enable: true });
  const routine = (await api<RoutineRevision>('/v1/routines', { definition: { schemaVersion: 1, name: 'Morning', agentId: scout.agentId, conversationId: scout.homeConversationId, bindingRevisionId: scout.binding.bindingRevisionId,
    instructions: 'Run it.', inputs: [], skills: [{ skillId: skill.skillId, revisionId: skill.revisionId, digest: skill.bundle!.digest }], steps: [{ stepId: 'task', name: 'Task', type: 'task', instructions: 'Reply.', inputNames: [], outputStepIds: [] }] } })).body;
  const schedule = (await api<ScheduleRevision>('/v1/schedules', { definition: { schemaVersion: 1, routineId: routine.routineId, routineRevisionId: routine.revisionId, timezone: 'UTC', time: '09:30', kind: 'daily', inputs: {}, missed: 'skip' } })).body;
  const preview = (await api<SchedulePreview>('/v1/schedules/' + schedule.scheduleId + '/preview')).body;
  assert.equal((await api('/v1/schedules/' + schedule.scheduleId + '/state', { recoveryEpoch: epoch, revisionId: schedule.revisionId, previewDigest: preview.digest, enable: true })).status, 200);
  return { service, api, send, scout, loner, routine, schedule, epoch, directory, close, tick: (ms: number) => { clock += ms; } };
}

// The hub, serving the 'fixture' runtime under the node 'toby-node' with the Mac in its pool.
async function hub(t: TestContext, extra: Partial<Parameters<typeof startCloudHub>[0]> = {}) {
  const dataDirectory = await mkdtemp(join(tmpdir(), 'fort-hub-handoff-'));
  const open = () => startCloudHub({ dataDirectory, host: '127.0.0.1', port: 0, provider: lane('fixture', 'toby-node'), setupCodeDigest, pool: ['tobys-mac'], ...extra });
  let started = await open();
  t.after(async () => { await started.close(); await rm(dataDirectory, { recursive: true, force: true }); });
  const owner = { deviceId: randomUUID(), deviceToken: secret() };
  const installationId = started.installation.installationId;
  async function call<T = Record<string, any>>(path: string, body?: unknown, headers: Record<string, string> = { authorization: 'Bearer ' + owner.deviceToken }) {
    const response = await fetch(started.url + path, { method: body === undefined ? 'GET' : 'POST', headers: { 'content-type': 'application/json', 'x-fort-installation-id': installationId, ...headers }, body: body === undefined ? undefined : JSON.stringify(body) });
    const text = await response.text();
    return { status: response.status, body: (text ? JSON.parse(text) : {}) as T };
  }
  return { get started() { return started; }, call, owner, installationId, async restart() { await started.close(); started = await open(); } };
}

async function enroll(t: TestContext, h: Awaited<ReturnType<typeof hub>>, setup: (path: string, body: unknown) => Promise<{ status: number }>) {
  const invitation = { invitationId: randomUUID(), code: secret() };
  assert.equal((await setup('/v1/runners/invitation', invitation)).status, 201);
  const state = await mkdtemp(join(tmpdir(), 'fort-mac-state-'));
  t.after(() => rm(state, { recursive: true, force: true }));
  assert.equal(((await stat(state)).mode & 0o777), 0o700);
  await enrollWithHub({ stateDirectory: state, hubURL: h.started.url, invitationId: invitation.invitationId, code: invitation.code, runnerId: 'tobys-mac', runnerName: 'Toby’s Mac' });
  return state;
}

test('a synced Mac hands its Agents to the hub once, keeps history, and its copies go read-only', async t => {
  const m = await mac(t), h = await hub(t);
  assert.equal((await h.call('/v1/hub/setup', { setupCode, ...h.owner }, {})).status, 201);
  const state = await enroll(t, h, (path, body) => h.call(path, body));
  const first = await syncAgentsToHub({ stateDirectory: state, ledger: m.service.hubLedger });
  assert.equal(first.state, 'synced');
  if (first.state !== 'synced') return;
  assert.deepEqual(first.agentIds, [m.scout.agentId]);
  assert.deepEqual(first.staying, [{ agentId: m.loner.agentId, name: 'Loner', reason: 'runtime' }], 'an Agent on a runtime the hub lacks stays and keeps running here');

  // The hub now owns Scout, bound to its node, with its routine and schedule, and no history.
  const agents = (await h.call<{ agents: Agent[] }>('/v1/agents')).body.agents;
  assert.deepEqual(agents.map(agent => [agent.name, agent.binding.hostId]), [['Scout', 'toby-node']]);
  const routines = (await h.call<{ routines: RoutineRevision[] }>('/v1/routines')).body.routines;
  assert.deepEqual(routines.map(routine => routine.revisionId), [m.routine.revisionId], 'routine revisions move unchanged');
  const schedules = (await h.call<{ schedules: ScheduleRevision[] }>('/v1/schedules')).body.schedules;
  assert.equal(schedules[0]?.state, 'enabled', 'the schedule keeps running, now on the hub');
  assert.ok(schedules[0]!.checkedThrough >= m.schedule.checkedThrough);
  assert.equal((await h.call<Snapshot>(`/v1/conversations/${m.scout.homeConversationId}/snapshot`)).body.messages.length, 0, 'history stays on the Mac');
  assert.equal((await h.call<{ state: string }>('/v1/modules/context')).body.state, 'ready', 'skills and memory work on the hub');

  // The Mac keeps everything, but Scout is read-only and its schedule paused; Loner still runs here.
  const local = (await m.api<{ agents: (Agent & { movedToHub?: true })[] }>('/v1/agents')).body.agents;
  assert.deepEqual(local.map(agent => [agent.name, agent.movedToHub ?? false]), [['Scout', true], ['Loner', false]]);
  assert.equal((await m.api<Snapshot>(`/v1/conversations/${m.scout.homeConversationId}/snapshot`)).body.messages.length, 2);
  const refused = await m.send(m.scout, 'still here?');
  assert.equal(refused.status, 409); assert.equal(refused.body.error.code, 'movedToHub');
  assert.equal((await m.api(`/v1/agents/${m.scout.agentId}`, { name: 'Renamed', instructions: 'x', model: 'm', expectedBindingRevisionId: m.scout.binding.bindingRevisionId })).status, 409);
  assert.equal((await m.api('/v1/routines/' + m.routine.routineId + '/run', { requestId: randomUUID(), routineRevisionId: m.routine.revisionId, recoveryEpoch: m.epoch, inputs: {} })).status, 409);
  assert.equal((await m.api<{ schedules: ScheduleRevision[] }>('/v1/schedules')).body.schedules[0]?.state, 'paused');
  m.tick(3_600_000);
  assert.equal(m.service.tickSchedules(), 0, 'the Mac never runs the moved schedule');
  assert.equal((await m.send(m.loner, 'hello')).status, 201);

  // A second sync changes nothing on either side.
  const again = await syncAgentsToHub({ stateDirectory: state, ledger: m.service.hubLedger });
  assert.equal(again.state, 'synced');
  assert.equal((await h.call<{ agents: Agent[] }>('/v1/agents')).body.agents.length, 1);
  assert.equal((await h.call<{ schedules: ScheduleRevision[] }>('/v1/schedules')).body.schedules.length, 1);
  assert.deepEqual(m.service.hubLedger.handoff()?.agentIds, [m.scout.agentId]);

  // A new Agent made on the Mac moves on the next sync.
  const later = (await m.api<Agent>('/v1/agents', { name: 'Later', model: 'exact-model', instructions: 'Echo.' })).body;
  const third = await syncAgentsToHub({ stateDirectory: state, ledger: m.service.hubLedger });
  assert.deepEqual(third.state === 'synced' && third.agentIds, [m.scout.agentId, later.agentId].sort());
  assert.equal((await h.call<{ agents: Agent[] }>('/v1/agents')).body.agents.length, 2);
});

test('the Mac gets its own client credential for the hub, which ends when the Mac is removed', async t => {
  const m = await mac(t), h = await hub(t);
  assert.equal((await h.call('/v1/hub/setup', { setupCode, ...h.owner }, {})).status, 201);
  const state = await enroll(t, h, (path, body) => h.call(path, body));
  assert.equal((await syncAgentsToHub({ stateDirectory: state, ledger: m.service.hubLedger })).state, 'synced');
  const client = await readHubClient(state);
  assert.ok(client);
  assert.equal(client.installationId, h.installationId);
  const asMac = { authorization: 'Bearer ' + client.token };
  const agents = (await h.call<{ agents: Agent[] }>('/v1/agents', undefined, asMac)).body.agents;
  assert.deepEqual(agents.map(agent => agent.name), ['Scout']);
  const scout = agents[0]!;
  const sent = await h.call(`/v1/conversations/${scout.homeConversationId}/messages`, { requestId: randomUUID(), text: 'from the Mac app', recipientAgentId: scout.agentId, bindingRevisionId: scout.binding.bindingRevisionId, recoveryEpoch: h.started.installation.recoveryEpoch, skills: [] }, asMac);
  // The Mac app reaches the hub's own checks, like any client: here the hub has no provider account picked yet.
  assert.equal((sent.body as { error?: { code?: string } }).error?.code, 'accountSelectionRequired');
  assert.equal((await h.call('/v1/runners', undefined, asMac)).status, 401, 'the client credential is not the owner');
  assert.equal((await h.call<{ role?: string }>('/v1/pairing', undefined, asMac)).body.role, 'companion');
  // An iPhone can still pair beside it.
  assert.equal((await h.call('/v1/pairing/invitation', { invitationId: randomUUID(), code: secret() })).status, 201);
  assert.equal((await h.call('/v1/runners/revoke', { runnerId: 'tobys-mac' })).status, 200);
  assert.equal((await h.call('/v1/agents', undefined, asMac)).status, 401);
});

test('an older hub without handoff leaves the Mac’s Agents running where they are', async t => {
  const m = await mac(t), h = await hub(t);
  assert.equal((await h.call('/v1/hub/setup', { setupCode, ...h.owner }, {})).status, 201);
  const state = await enroll(t, h, (path, body) => h.call(path, body));
  const old = (url: string, init?: RequestInit) => url.endsWith('/v1/runners/agents') ? Promise.resolve(new Response('{}', { status: 404 })) : fetch(url, init);
  assert.equal((await syncAgentsToHub({ stateDirectory: state, ledger: m.service.hubLedger, fetch: old })).state, 'unsupported');
  assert.equal(m.service.hubLedger.handoff(), undefined);
  assert.equal((await m.send(m.scout, 'still mine')).status, 201);
});

test('the hub refuses handoffs from strangers and Agents it cannot run', async t => {
  const m = await mac(t), h = await hub(t);
  assert.equal((await h.call('/v1/hub/setup', { setupCode, ...h.owner }, {})).status, 201);
  const { payload } = m.service.hubLedger.exportHandoff([{ provider: 'fixture', runtimeVersion: 'fixture-1' }]);
  assert.equal((await h.call('/v1/runners/agents', payload, { authorization: 'Bearer ' + secret() })).status, 401);
  assert.equal((await h.call('/v1/runners/agents', payload)).status, 401, 'the owner’s credential is not a Mac');
  const state = await enroll(t, h, (path, body) => h.call(path, body));
  const token = JSON.parse(await (await import('node:fs/promises')).readFile(join(state, 'hub-runner.json'), 'utf8')).token as string;
  const everything = m.service.hubLedger.exportHandoff([{ provider: 'fixture', runtimeVersion: 'fixture-1' }, { provider: 'elsewhere', runtimeVersion: 'elsewhere-1' }]).payload;
  const refused = await h.call('/v1/runners/agents', everything, { authorization: 'Bearer ' + token });
  assert.equal(refused.status, 422, 'an Agent whose runtime the hub lacks is refused, and nothing is added');
  assert.equal((await h.call<{ agents: Agent[] }>('/v1/agents')).body.agents.length, 0);
  const tampered = { ...payload, rows: payload.rows.map(row => row.kind === 'agents' ? { ...row, data: '{"agentId":"x"}' } : row) };
  assert.equal((await h.call('/v1/runners/agents', tampered, { authorization: 'Bearer ' + token })).status, 422);
  assert.equal((await h.call('/v1/runners/agents', payload, { authorization: 'Bearer ' + token })).status, 200);
});

// With the Google gate on, the Mac's client credential stands in for a Google session; browsers never can use it.
const origin = 'https://fort.example.com', clientId = '1234-abc.apps.googleusercontent.com', iosClientId = '1234-ios.apps.googleusercontent.com';
const google = generateKeyPairSync('rsa', { modulusLength: 2048 });
const jwk = { ...google.publicKey.export({ format: 'jwk' }), kid: 'k1', alg: 'RS256', use: 'sig' };
const b64 = (value: object) => Buffer.from(JSON.stringify(value)).toString('base64url');
function idToken(nonce: string) {
  const now = Math.floor(Date.now() / 1000);
  const body = b64({ alg: 'RS256', kid: 'k1', typ: 'JWT' }) + '.' + b64({ iss: 'https://accounts.google.com', aud: clientId, sub: '1001', email: 'toby@example.com', email_verified: true, iat: now, exp: now + 3600, nonce });
  return body + '.' + sign('RSA-SHA256', Buffer.from(body), google.privateKey).toString('base64url');
}

test('behind the Google gate the Mac uses its client credential without its own Google sign-in', async t => {
  const m = await mac(t), h = await hub(t, { publicOrigin: origin, google: { clientId, iosClientId, emails: ['toby@example.com'], keys: async () => [jwk] } });
  const nonce = (await h.call<{ nonce: string }>('/auth/session', undefined, {})).body.nonce;
  const signedIn = await fetch(h.started.url + '/auth/google', { method: 'POST', headers: { 'content-type': 'application/json', origin }, body: JSON.stringify({ credential: idToken(nonce) }) });
  const cookie = signedIn.headers.get('set-cookie')!.split(';')[0]!;
  const browser = (path: string, body?: unknown) => h.call(path, body, { origin, 'sec-fetch-site': 'same-origin', cookie, authorization: 'Bearer ' + h.owner.deviceToken });
  assert.equal((await browser('/v1/hub/setup', { setupCode, ...h.owner })).status, 201);
  const state = await enroll(t, h, browser);
  assert.equal((await syncAgentsToHub({ stateDirectory: state, ledger: m.service.hubLedger })).state, 'synced');
  const client = (await readHubClient(state))!;
  assert.equal((await h.call('/v1/agents', undefined, { authorization: 'Bearer ' + client.token })).status, 200);
  assert.equal((await h.call<{ error: { code: string } }>('/v1/agents', undefined, { authorization: 'Bearer ' + client.token, origin })).body.error.code, 'signInRequired', 'a browser can’t borrow it');
  assert.equal((await h.call<{ error: { code: string } }>('/v1/agents', undefined, { authorization: 'Bearer ' + secret() })).body.error.code, 'signInRequired');
});

test('history uploaded later fills in around the Agents the Mac already handed over', async t => {
  const m = await mac(t), h = await hub(t);
  assert.equal((await h.call('/v1/hub/setup', { setupCode, ...h.owner }, {})).status, 201);
  const state = await enroll(t, h, (path, body) => h.call(path, body));
  assert.equal((await syncAgentsToHub({ stateDirectory: state, ledger: m.service.hubLedger })).state, 'synced');
  // The owner's separate history upload, with the Mac's Fort quit.
  await m.close();
  const sent = await uploadLedger({ ledger: join(m.directory, 'openai-candidate-b.sqlite'), stateDirectory: state, waitMs: 1 });
  assert.equal(sent.state, 'staged');
  await h.restart();
  const machine = JSON.parse(await (await import('node:fs/promises')).readFile(join(state, 'hub-runner.json'), 'utf8')).token as string;
  assert.equal((await h.call<{ state: string }>('/v1/runners/import', undefined, { authorization: 'Bearer ' + machine })).body.state, 'applied');
  const agents = (await h.call<{ agents: Agent[] }>('/v1/agents')).body.agents;
  assert.deepEqual(agents.map(agent => [agent.name, agent.binding.hostId]).sort(), [['Loner', 'toby-node'], ['Scout', 'toby-node']], 'no duplicate Scout');
  assert.deepEqual((await h.call<Snapshot>(`/v1/conversations/${m.scout.homeConversationId}/snapshot`)).body.messages.map(message => message.text), ['remember the cedar', 'remember the cedar']);
  assert.equal((await h.call<{ schedules: ScheduleRevision[] }>('/v1/schedules')).body.schedules[0]?.state, 'enabled', 'the hub keeps its own copy, not the Mac’s paused one');
});
