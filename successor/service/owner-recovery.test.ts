import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { createApplication } from './openai.ts';
import { OwnerRecoveryIdentityRejected } from '../contracts/owner-recovery.ts';
import { CODEX_RUNTIME } from '../runner/codex-descriptor.ts';
import type { Agent, RoutineRevision, RuntimeReadiness, ScheduleRevision, SchedulePreview, Snapshot } from '../contracts/v1.ts';

type Options = Partial<Omit<Parameters<typeof createApplication>[0], 'dataDirectory'>>;
async function fixture(t: TestContext, options: Options = {}) {
  const directory = await mkdtemp(join(tmpdir(), 'fort-owner-recovery-'));
  const create = () => createApplication({ provider: CODEX_RUNTIME, dataDirectory: directory, ownerPairingPolicy: { expiresAfterDays: null }, ...options });
  let service = create();
  await service.start();
  t.after(async () => { await service.close(); await rm(directory, { recursive: true, force: true }); });
  let token = '';
  async function enroll() {
    const response = await fetch(service.baseURL + '/v1/devices/enroll', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ...service.issueEnrollment(), deviceName: 'Synthetic recovery device' }),
    });
    assert.equal(response.status, 201);
    const device = await response.json() as { deviceId: string; deviceToken: string };
    token = device.deviceToken;
    return { installationId: service.installation.installationId, deviceId: device.deviceId };
  }
  const identity = await enroll();
  async function api<T>(path: string, body?: unknown) {
    const response = await fetch(service.baseURL + path, {
      method: body === undefined ? 'GET' : 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}`, 'X-Fort-Installation-Id': service.installation.installationId },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    return { status: response.status, body: await response.json() as T };
  }
  return { api, enroll, identity, directory, get service() { return service; }, async restart(start: Parameters<typeof service.start>[0] = {}) {
    await service.close(); service = create(); await service.start(start);
  } };
}

test('local start and recovery cannot overlap and erase a pending stop', async t => {
  const h = await fixture(t);
  assert.equal((await h.api('/v1/host/stop', {})).status, 200);
  await h.service.close();
  const reopened = createApplication({ provider: CODEX_RUNTIME, dataDirectory: h.directory });
  t.after(async () => { await reopened.close(); });
  const starting = reopened.start({ resumeOwnerHost: true });
  await assert.rejects(reopened.recoverOwnerPairing(h.identity), /starting/);
  await starting;
  assert.deepEqual(await reopened.recoverOwnerPairing(h.identity), { ...h.identity, runtimeStopped: true, revoked: true });
  await reopened.close();
});

test('an owner stop keeps queued routines and new work paused across core restart', async t => {
  let calls = 0;
  const h = await fixture(t, { runtime: {
    async execute() { calls++; return { kind: 'completed', text: 'Synthetic routine result', effectiveModel: null }; },
    async dispose() {},
  } });
  const agent = (await h.api<Agent>('/v1/agents', { name: 'Recovery fixture', instructions: 'Synthetic work only.', model: 'exact' })).body;
  const routine = (await h.api<RoutineRevision>('/v1/routines', { definition: {
    schemaVersion: 1, name: 'Queued fixture', agentId: agent.agentId, conversationId: agent.homeConversationId,
    bindingRevisionId: agent.binding.bindingRevisionId, instructions: 'Do the declared task.', inputs: [], skills: [],
    steps: [{ stepId: 'task', name: 'Task', type: 'task', instructions: 'Write.', inputNames: [], outputStepIds: [] }],
  } })).body;
  assert.equal((await h.api(`/v1/routines/${routine.routineId}/run`, {
    requestId: 'queued-before-stop', routineRevisionId: routine.revisionId, recoveryEpoch: h.service.installation.recoveryEpoch, inputs: {},
  })).status, 201);
  assert.equal((await h.api('/v1/host/stop', {})).status, 200);
  await h.restart();
  assert.equal(await h.service.dispatchNext(), false);
  assert.equal(calls, 0);
  assert.equal((await h.api<RuntimeReadiness>('/v1/runtime')).body.state, 'unavailable');
  assert.equal((await h.api('/v1/agents', { name: 'Blocked', instructions: 'Fixture', model: 'exact' })).status, 503);
  assert.throws(() => h.service.issueEnrollment(), /stopped/);
  await h.restart({ resumeOwnerHost: true });
  assert.equal((await h.api('/v1/pairing')).status, 200);
  assert.equal(await h.service.dispatchNext(), true);
  assert.equal(calls, 1);
});

test('owner stop prevents schedule catch-up until an explicit local start', async t => {
  let now = Date.parse('2026-10-18T09:00:00Z');
  const h = await fixture(t, { now: () => now });
  const agent = (await h.api<Agent>('/v1/agents', { name: 'Clock fixture', instructions: 'Synthetic work only.', model: 'exact' })).body;
  const routine = (await h.api<RoutineRevision>('/v1/routines', { definition: {
    schemaVersion: 1, name: 'Scheduled fixture', agentId: agent.agentId, conversationId: agent.homeConversationId,
    bindingRevisionId: agent.binding.bindingRevisionId, instructions: 'Only declared work.', inputs: [], skills: [],
    steps: [{ stepId: 'task', name: 'Task', type: 'task', instructions: 'Write.', inputNames: [], outputStepIds: [] }],
  } })).body;
  const schedule = (await h.api<ScheduleRevision>('/v1/schedules', { definition: {
    schemaVersion: 1, routineId: routine.routineId, routineRevisionId: routine.revisionId,
    kind: 'once', date: '2026-10-18', timezone: 'UTC', time: '09:01', inputs: {}, missed: 'catchUpOne',
  } })).body;
  const path = `/v1/schedules/${schedule.scheduleId}`;
  const preview = (await h.api<SchedulePreview>(path + '/preview')).body;
  assert.equal((await h.api(path + '/state', {
    revisionId: schedule.revisionId, recoveryEpoch: h.service.installation.recoveryEpoch, previewDigest: preview.digest, enable: true,
  })).status, 200);
  assert.equal((await h.api('/v1/host/stop', {})).status, 200);
  now += 120_000;
  await h.restart();
  assert.equal(h.service.tickSchedules(), 0);
  assert.equal(h.service.inspectSession().receipts.length, 0);
  assert.deepEqual((await h.api<{ occurrences: unknown[] }>(path + '/occurrences')).body.occurrences, []);
  await h.restart({ resumeOwnerHost: true });
  assert.equal(h.service.inspectSession().receipts.length, 1);
});

test('trusted local recovery verifies expired and already-revoked pairing without deleting history', async t => {
  let now = Date.parse('2026-10-18T00:00:00Z'), disposals = 0;
  const h = await fixture(t, { now: () => now, ownerPairingPolicy: { expiresAfterDays: 1 }, runtime: {
    async execute() { return { kind: 'completed', text: 'Retained synthetic reply', effectiveModel: null }; },
    async dispose() { disposals++; },
  } });
  const agent = (await h.api<Agent>('/v1/agents', { name: 'History fixture', instructions: 'Synthetic only.', model: 'exact' })).body;
  assert.equal((await h.api(`/v1/conversations/${agent.homeConversationId}/messages`, {
    requestId: 'retained-recovery', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId,
    recoveryEpoch: h.service.installation.recoveryEpoch, text: 'Retain this synthetic history.', skills: [],
  })).status, 201);
  await h.service.dispatchNext();
  const before = (await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body;
  now += 2 * 86_400_000;
  assert.equal((await h.api('/v1/pairing')).status, 401);
  await assert.rejects(h.service.recoverOwnerPairing({ ...h.identity, installationId: 'other-installation' }), OwnerRecoveryIdentityRejected);
  await assert.rejects(h.service.recoverOwnerPairing({ ...h.identity, deviceId: 'other-device' }), OwnerRecoveryIdentityRejected);
  assert.equal(disposals, 0);
  const proof = { ...h.identity, runtimeStopped: true, revoked: true };
  assert.deepEqual(await h.service.recoverOwnerPairing(h.identity), proof);
  assert.equal(disposals, 1);
  assert.deepEqual(h.service.inspectSession().deviceIDs, []);
  assert.throws(() => h.service.issueEnrollment(), /stopped/);
  await h.restart();
  assert.deepEqual(await h.service.recoverOwnerPairing(h.identity), proof);
  assert.equal(disposals, 2);
  await h.restart({ resumeOwnerHost: true });
  assert.equal((await h.api('/v1/pairing')).status, 401);
  const fresh = await h.enroll();
  assert.notEqual(fresh.deviceId, h.identity.deviceId);
  const beforeRejectedRecovery = disposals;
  await assert.rejects(h.service.recoverOwnerPairing(h.identity), /exact owner pairing/);
  assert.equal(disposals, beforeRejectedRecovery);
  assert.deepEqual((await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body, before);
});

test('pending disconnect survives failed disposal and cannot be downgraded or resumed before recovery', async t => {
  let failDisposal = true;
  const h = await fixture(t, { runtime: {
    async execute() { return { kind: 'notStarted', message: 'Synthetic only.' }; },
    async dispose() { if (failDisposal) throw new Error('SECRET runtime detail'); },
  } });
  t.after(() => { failDisposal = false; });
  const failed = await h.api('/v1/pairing/disconnect', {});
  assert.equal(failed.status, 503);
  assert.equal(JSON.stringify(failed).includes('SECRET'), false);
  assert.deepEqual(h.service.inspectSession().deviceIDs, [h.identity.deviceId]);
  assert.equal((await h.api('/v1/host/stop', {})).status, 409);
  failDisposal = false;
  await assert.rejects(h.restart({ resumeOwnerHost: true }), /pending disconnect/);
  await h.service.start();
  assert.equal((await h.api('/v1/pairing')).status, 200);
  assert.equal((await h.api('/v1/agents', { name: 'Blocked', instructions: 'Fixture', model: 'exact' })).status, 503);
  assert.deepEqual(await h.service.recoverOwnerPairing(h.identity), { ...h.identity, runtimeStopped: true, revoked: true });
  assert.deepEqual(h.service.inspectSession().deviceIDs, []);
});

test('recovery retains pairing during disposal and permits an explicit retry after failure', async t => {
  let rejectDisposal!: (error: Error) => void, started!: () => void, attempts = 0;
  const stopping = new Promise<void>(resolve => { started = resolve; });
  const disposal = new Promise<void>((_, reject) => { rejectDisposal = reject; });
  const h = await fixture(t, { runtime: {
    async execute() { return { kind: 'notStarted', message: 'Synthetic only.' }; },
    async dispose() { if (++attempts === 1) { started(); await disposal; } },
  } });
  const pending = h.service.recoverOwnerPairing(h.identity);
  const rejected = assert.rejects(pending, error => error instanceof Error && /not confirmed/.test(error.message) && !error.message.includes('SECRET'));
  await stopping;
  assert.deepEqual(h.service.inspectSession().deviceIDs, [h.identity.deviceId]);
  await assert.rejects(h.service.recoverOwnerPairing(h.identity), /still waiting/);
  rejectDisposal(new Error('SECRET runtime detail'));
  await rejected;
  assert.deepEqual(h.service.inspectSession().deviceIDs, [h.identity.deviceId]);
  assert.deepEqual(await h.service.recoverOwnerPairing(h.identity), { ...h.identity, runtimeStopped: true, revoked: true });
  assert.equal(attempts, 2);
});

test('a lost HTTP disconnect result is recoverable only through trusted composition', async t => {
  const h = await fixture(t);
  assert.equal((await h.api('/v1/pairing/recover', h.identity)).status, 404);
  assert.equal((await h.api('/v1/pairing/disconnect', {})).status, 200);
  await h.restart();
  assert.equal((await h.api('/v1/pairing')).status, 401);
  assert.deepEqual(await h.service.recoverOwnerPairing(h.identity), { ...h.identity, runtimeStopped: true, revoked: true });
});

test('explicit start never renews an expired credential and temporary sessions cannot use owner recovery', async t => {
  let now = Date.parse('2026-10-18T00:00:00Z');
  const h = await fixture(t, { now: () => now, ownerPairingPolicy: { expiresAfterDays: 1 } });
  assert.equal((await h.api('/v1/host/stop', {})).status, 200);
  now += 2 * 86_400_000;
  await h.restart({ resumeOwnerHost: true });
  assert.equal((await h.api('/v1/pairing')).status, 401);
  for (const credentialDeadline of [undefined, Date.now() + 60_000]) {
    const temporary = await fixture(t, { ownerPairingPolicy: undefined, credentialDeadline });
    await assert.rejects(temporary.service.recoverOwnerPairing(temporary.identity), /temporary session/);
    assert.deepEqual(temporary.service.inspectSession().deviceIDs, [temporary.identity.deviceId]);
    await assert.rejects(temporary.restart({ resumeOwnerHost: true }), /owner-local/);
  }
});

test('a runtime failure cannot masquerade as a harmless recovery identity rejection', async t => {
  let fails = true;
  const h = await fixture(t, { runtime: {
    async execute() { return { kind: 'notStarted', message: 'Synthetic only.' }; },
    async dispose() { if (fails) throw new OwnerRecoveryIdentityRejected('device'); },
  } });
  await assert.rejects(h.service.recoverOwnerPairing(h.identity), error => error instanceof Error && !(error instanceof OwnerRecoveryIdentityRejected) && error.message.includes('shutdown was not confirmed'));
  assert.deepEqual(h.service.inspectSession().deviceIDs, [h.identity.deviceId]);
  assert.equal((await h.api('/v1/agents', { name: 'Blocked', instructions: 'Fixture', model: 'exact' })).status, 503);
  fails = false;
  assert.deepEqual(await h.service.recoverOwnerPairing(h.identity), { ...h.identity, runtimeStopped: true, revoked: true });
});
