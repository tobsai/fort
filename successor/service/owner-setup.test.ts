import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes, randomUUID } from 'node:crypto';
import { mkdtemp, rm } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { DatabaseSync } from 'node:sqlite';
import { createHash } from 'node:crypto';
import { createApplication } from './openai.ts';
import { CODEX_RUNTIME } from '../runner/codex-descriptor.ts';
import type { OwnerSetupState } from '../contracts/owner-setup.ts';
import type { Agent, Snapshot } from '../contracts/v1.ts';

function prepare(offer: OwnerSetupState) {
  assert.equal(offer.kind, 'available');
  return { installationId: offer.installationId, setupId: offer.setupId, deviceId: randomUUID(), deviceToken: randomBytes(32).toString('base64url') };
}

function setupIdentity(request: ReturnType<typeof prepare>) {
  const { installationId, setupId, deviceId } = request;
  return { installationId, setupId, deviceId };
}

async function fixture(t: TestContext, options: Partial<Omit<Parameters<typeof createApplication>[0], 'dataDirectory'>> = {}) {
  const directory = await mkdtemp(join(tmpdir(), 'fort-owner-setup-'));
  const create = () => createApplication({ dataDirectory: directory, provider: CODEX_RUNTIME, ownerPairingPolicy: { expiresAfterDays: null }, ...options });
  let service = create();
  await service.start();
  t.after(async () => { await service.close(); await rm(directory, { recursive: true, force: true }); });
  return {
    directory,
    get service() { return service; },
    async restart(start: Parameters<typeof service.start>[0] = {}) { await service.close(); service = create(); await service.start(start); },
    async api(token: string, path = '/v1/pairing', body?: unknown) {
      return fetch(service.baseURL + path, {
        method: body === undefined ? 'GET' : 'POST',
        headers: { Authorization: `Bearer ${token}`, 'X-Fort-Installation-Id': service.installation.installationId, 'Content-Type': 'application/json' },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
    },
  };
}

test('a lost owner setup reply can be confirmed after restart with the exact prepared credential', async t => {
  const h = await fixture(t);
  const offer = h.service.beginOwnerSetup();
  assert.equal(offer.kind, 'available');
  assert.deepEqual(h.service.beginOwnerSetup(), offer);
  const request = prepare(offer);
  const receipt = await h.service.completeOwnerSetup(request);
  assert.deepEqual(receipt, { installationId: request.installationId, deviceId: request.deviceId, expiresAt: null });
  await h.restart();
  assert.deepEqual(await h.service.completeOwnerSetup(request), receipt);
  assert.deepEqual(h.service.inspectSession().deviceIDs, [request.deviceId]);
  assert.deepEqual(h.service.beginOwnerSetup(), { kind: 'paired', ...receipt });
  assert.equal((await h.api(request.deviceToken)).status, 200);
  assert.equal((await h.api(randomBytes(32).toString('base64url'))).status, 401);
});

test('canceling a pending setup durably prevents delayed completion without stopping the host', async t => {
  const h = await fixture(t);
  const request = prepare(h.service.beginOwnerSetup()), identity = setupIdentity(request);
  const expected = { ...identity, outcome: 'notPaired' };
  assert.deepEqual(await h.service.cancelOwnerSetup(identity), expected);
  assert.deepEqual(await h.service.cancelOwnerSetup(identity), expected);
  await assert.rejects(() => h.service.completeOwnerSetup(request), /canceled/);
  await h.restart();
  assert.deepEqual(await h.service.cancelOwnerSetup(identity), expected);
  await assert.rejects(() => h.service.completeOwnerSetup(request), /canceled/);
  assert.deepEqual(h.service.inspectSession().deviceIDs, []);
  const fresh = prepare(h.service.beginOwnerSetup());
  assert.notEqual(fresh.setupId, request.setupId);
  await h.service.completeOwnerSetup(fresh);
  assert.deepEqual(await h.service.cancelOwnerSetup(identity), expected);
  assert.deepEqual(h.service.inspectSession().deviceIDs, [fresh.deviceId]);
  assert.equal((await h.api(fresh.deviceToken)).status, 200);
});

test('canceling completed setup retains stop intent and pairing after failed disposal and restart', async t => {
  let fails = true, disposals = 0;
  const h = await fixture(t, { runtime: {
    async execute() { return { kind: 'notStarted', message: 'Synthetic only' }; },
    async dispose() { disposals++; if (fails) throw new Error('SECRET synthetic runtime detail'); },
  } });
  const request = prepare(h.service.beginOwnerSetup()), identity = setupIdentity(request);
  await h.service.completeOwnerSetup(request);
  try {
    await assert.rejects(h.service.cancelOwnerSetup(identity), error => error instanceof Error && /not confirmed/.test(error.message) && !error.message.includes('SECRET'));
    assert.deepEqual(h.service.inspectSession().deviceIDs, [request.deviceId]);
    await assert.rejects(() => h.service.completeOwnerSetup(request), /running local host/);
    assert.equal((await h.api(request.deviceToken, '/v1/agents', { name: 'Blocked', instructions: 'Synthetic', model: 'exact' })).status, 503);
  } finally { fails = false; }
  await assert.rejects(h.restart({ resumeOwnerHost: true }), /pending disconnect|pending setup cancellation/);
  await h.service.start();
  assert.equal((await h.api(request.deviceToken)).status, 200);
  assert.equal((await h.api(request.deviceToken, '/v1/agents', { name: 'Still blocked', instructions: 'Synthetic', model: 'exact' })).status, 503);
  const proof = { ...identity, outcome: 'revoked', runtimeStopped: true };
  assert.deepEqual(await h.service.cancelOwnerSetup(identity), proof);
  assert.equal((await h.api(request.deviceToken)).status, 401);
  assert.deepEqual(h.service.inspectSession().deviceIDs, []);
  await h.restart();
  const beforeRetry = disposals;
  assert.deepEqual(await h.service.cancelOwnerSetup(identity), proof);
  assert.equal(disposals, beforeRetry + 1);
  assert.throws(() => h.service.beginOwnerSetup(), /running local host/);
});

test('cancellation waits for runtime disposal and refuses concurrent cancellation or recovery', async t => {
  let release!: () => void, entered!: () => void;
  const disposal = new Promise<void>(resolve => { release = resolve; });
  const enteredDisposal = new Promise<void>(resolve => { entered = resolve; });
  const h = await fixture(t, { runtime: {
    async execute() { return { kind: 'notStarted', message: 'Synthetic only' }; },
    async dispose() { entered(); await disposal; },
  } });
  const request = prepare(h.service.beginOwnerSetup()), identity = setupIdentity(request);
  await h.service.completeOwnerSetup(request);
  let confirmed = false;
  const cancellation = h.service.cancelOwnerSetup(identity).then(proof => { confirmed = true; return proof; });
  try {
    await Promise.race([enteredDisposal, cancellation.then(() => assert.fail('Cancellation confirmed before disposal'))]);
    assert.equal(confirmed, false);
    assert.deepEqual(h.service.inspectSession().deviceIDs, [request.deviceId]);
    await assert.rejects(h.service.cancelOwnerSetup(identity), /available local host/);
    await assert.rejects(h.service.recoverOwnerPairing(identity), /still waiting/);
    await assert.rejects(() => h.service.completeOwnerSetup(request), /running local host/);
  } finally { release(); }
  assert.deepEqual(await cancellation, { ...identity, outcome: 'revoked', runtimeStopped: true });
  assert.deepEqual(h.service.inspectSession().deviceIDs, []);
});

test('an expired completed setup can be canceled without its token and history survives a new pairing', async t => {
  let now = Date.parse('2026-10-18T00:00:00Z'), disposals = 0;
  const h = await fixture(t, { now: () => now, ownerPairingPolicy: { expiresAfterDays: 1 }, runtime: {
    async execute() { return { kind: 'completed', text: 'Retained synthetic reply', effectiveModel: null }; },
    async dispose() { disposals++; },
  } });
  const request = prepare(h.service.beginOwnerSetup()), identity = setupIdentity(request);
  await h.service.completeOwnerSetup(request);
  const agentResponse = await h.api(request.deviceToken, '/v1/agents', { name: 'History', instructions: 'Synthetic only', model: 'exact' });
  assert.equal(agentResponse.status, 201);
  const agent = await agentResponse.json() as Agent;
  assert.equal((await h.api(request.deviceToken, `/v1/conversations/${agent.homeConversationId}/messages`, {
    requestId: 'cancel-setup-history', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId,
    recoveryEpoch: h.service.installation.recoveryEpoch, text: 'Keep synthetic history', skills: [],
  })).status, 201);
  await h.service.dispatchNext();
  const path = `/v1/conversations/${agent.homeConversationId}/snapshot`;
  const before = await (await h.api(request.deviceToken, path)).json() as Snapshot;
  now += 2 * 86_400_000;
  assert.equal((await h.api(request.deviceToken)).status, 401);
  assert.deepEqual(await h.service.cancelOwnerSetup(identity), { ...identity, outcome: 'revoked', runtimeStopped: true });
  assert.equal(disposals, 1);
  await h.restart({ resumeOwnerHost: true });
  const fresh = prepare(h.service.beginOwnerSetup());
  await h.service.completeOwnerSetup(fresh);
  const beforeObsolete = disposals;
  assert.deepEqual(await h.service.cancelOwnerSetup(identity), { ...identity, outcome: 'notPaired' });
  await assert.rejects(h.service.cancelOwnerSetup({ ...identity, deviceId: fresh.deviceId }), /exact owner setup/);
  assert.equal(disposals, beforeObsolete);
  assert.deepEqual(await (await h.api(fresh.deviceToken, path)).json(), before);
  assert.equal((await h.api(request.deviceToken)).status, 401);
});

test('cancellation validates exact identities without stopping or revoking a different pairing', async t => {
  let disposals = 0;
  const h = await fixture(t, { runtime: {
    async execute() { return { kind: 'notStarted', message: 'Synthetic only' }; },
    async dispose() { disposals++; },
  } });
  const request = prepare(h.service.beginOwnerSetup()), identity = setupIdentity(request);
  await h.service.completeOwnerSetup(request);
  for (const changed of [{ installationId: randomUUID() }, { setupId: randomUUID() }, { deviceId: randomUUID() }, { deviceId: '../foreign' }, { setupId: 'x'.repeat(129) }]) {
    await assert.rejects(h.service.cancelOwnerSetup({ ...identity, ...changed }));
  }
  assert.equal(disposals, 0);
  assert.deepEqual(h.service.inspectSession().deviceIDs, [request.deviceId]);
  assert.equal((await h.api(request.deviceToken)).status, 200);
  const absent = { ...identity, setupId: randomUUID(), deviceId: randomUUID() };
  assert.deepEqual(await h.service.cancelOwnerSetup(absent), { ...absent, outcome: 'notPaired' });
  assert.equal(disposals, 0);
});

test('cancellation of an expired pending offer survives restart and never revokes legacy enrollment', async t => {
  let now = Date.parse('2026-10-18T00:00:00Z');
  const h = await fixture(t, { now: () => now });
  const request = prepare(h.service.beginOwnerSetup()), identity = setupIdentity(request);
  now += 300_000;
  assert.deepEqual(await h.service.cancelOwnerSetup(identity), { ...identity, outcome: 'notPaired' });
  await assert.rejects(h.service.cancelOwnerSetup({ ...identity, deviceId: randomUUID() }), /exact owner setup/);
  await h.restart();
  await assert.rejects(() => h.service.completeOwnerSetup(request), /canceled/);
  const enrollment = h.service.issueEnrollment();
  const response = await fetch(h.service.baseURL + '/v1/devices/enroll', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...enrollment, deviceName: 'Synthetic legacy device' }),
  });
  assert.equal(response.status, 201);
  const legacy = await response.json() as { deviceId: string; deviceToken: string };
  assert.deepEqual(await h.service.cancelOwnerSetup(identity), { ...identity, outcome: 'notPaired' });
  await assert.rejects(h.service.cancelOwnerSetup({ ...identity, deviceId: legacy.deviceId }), /exact owner setup/);
  assert.deepEqual(h.service.inspectSession().deviceIDs, [legacy.deviceId]);
  assert.equal((await h.api(legacy.deviceToken)).status, 200);
});

test('cancellation can recover a paused pairing and a lost cancellation reply without silently resuming', async t => {
  let disposals = 0;
  const h = await fixture(t, { runtime: {
    async execute() { return { kind: 'notStarted', message: 'Synthetic only' }; },
    async dispose() { disposals++; },
  } });
  const request = prepare(h.service.beginOwnerSetup()), identity = setupIdentity(request);
  await h.service.completeOwnerSetup(request);
  assert.equal((await h.api(request.deviceToken, '/v1/host/stop', {})).status, 200);
  await h.restart();
  const proof = { ...identity, outcome: 'revoked', runtimeStopped: true };
  assert.deepEqual(await h.service.cancelOwnerSetup(identity), proof);
  await h.restart({ resumeOwnerHost: true });
  const before = disposals;
  assert.deepEqual(await h.service.cancelOwnerSetup(identity), proof);
  assert.equal(disposals, before + 1);
  assert.throws(() => h.service.beginOwnerSetup(), /running local host/);
  await h.service.close();
  await assert.rejects(h.service.cancelOwnerSetup(identity), /available local host/);
});

test('separate pairing recovery cannot resume an unfinished setup cancellation after restart', async t => {
  let fails = true;
  const h = await fixture(t, { runtime: {
    async execute() { return { kind: 'notStarted', message: 'Synthetic only' }; },
    async dispose() { if (fails) throw new Error('Synthetic disposal failure'); },
  } });
  const request = prepare(h.service.beginOwnerSetup()), identity = setupIdentity(request);
  await h.service.completeOwnerSetup(request);
  try { await assert.rejects(h.service.cancelOwnerSetup(identity), /not confirmed/); }
  finally { fails = false; }
  await h.service.recoverOwnerPairing(identity);
  assert.deepEqual(h.service.inspectSession().deviceIDs, []);
  await assert.rejects(h.restart({ resumeOwnerHost: true }), /pending setup cancellation/);
  await h.service.start();
  assert.throws(() => h.service.beginOwnerSetup(), /running local host/);
  assert.deepEqual(await h.service.cancelOwnerSetup(identity), { ...identity, outcome: 'revoked', runtimeStopped: true });
  await h.restart({ resumeOwnerHost: true });
  const next = prepare(h.service.beginOwnerSetup());
  assert.notEqual(next.setupId, request.setupId);
});

test('a pending offer survives restart and expires without silently rotating on completion', async t => {
  let now = Date.parse('2026-10-18T00:00:00Z');
  const h = await fixture(t, { now: () => now });
  const offer = h.service.beginOwnerSetup(), request = prepare(offer);
  assert.equal(offer.expiresAt, new Date(now + 300_000).toISOString());
  now += 299_999;
  await h.restart();
  assert.deepEqual(h.service.beginOwnerSetup(), offer);
  now++;
  await assert.rejects(() => h.service.completeOwnerSetup(request), /expired/);
  assert.deepEqual(h.service.inspectSession().deviceIDs, []);
  const replacement = prepare(h.service.beginOwnerSetup());
  assert.notEqual(replacement.setupId, request.setupId);
  await assert.rejects(() => h.service.completeOwnerSetup(request), /current exact offer/);
  await h.service.completeOwnerSetup(replacement);
  assert.equal((await h.api(replacement.deviceToken)).status, 200);
});

test('completed retries retain the original pairing expiry beyond the setup offer lifetime', async t => {
  let now = Date.parse('2026-10-18T00:00:00Z');
  const h = await fixture(t, { now: () => now, ownerPairingPolicy: { expiresAfterDays: 1 } });
  const request = prepare(h.service.beginOwnerSetup()), receipt = await h.service.completeOwnerSetup(request);
  assert.equal(receipt.expiresAt, new Date(now + 86_400_000).toISOString());
  now += 86_399_999;
  await h.restart();
  assert.deepEqual(await h.service.completeOwnerSetup(request), receipt);
  assert.equal((await h.api(request.deviceToken)).status, 200);
  now++;
  await assert.rejects(() => h.service.completeOwnerSetup(request), /active pairing/);
  assert.equal((await h.api(request.deviceToken)).status, 401);
  const replacement = prepare(h.service.beginOwnerSetup());
  await h.service.completeOwnerSetup(replacement);
  await assert.rejects(() => h.service.completeOwnerSetup(request), /current exact offer/);
  assert.deepEqual(h.service.inspectSession().deviceIDs, [replacement.deviceId]);
});

test('conflicting identity or credential cannot confirm or replace completed setup', async t => {
  const h = await fixture(t);
  const request = prepare(h.service.beginOwnerSetup());
  await assert.rejects(() => h.service.completeOwnerSetup({ ...request, installationId: randomUUID() }), /exact installation/);
  await assert.rejects(() => h.service.completeOwnerSetup({ ...request, setupId: randomUUID() }), /exact offer/);
  await assert.rejects(() => h.service.completeOwnerSetup({ ...request, deviceId: '../other' }), /identifier/);
  for (const deviceToken of ['x', '+'.repeat(43), 'A'.repeat(42) + 'B']) {
    await assert.rejects(() => h.service.completeOwnerSetup({ ...request, deviceToken }), /32-byte/);
  }
  assert.deepEqual(h.service.inspectSession().deviceIDs, []);
  const receipt = await h.service.completeOwnerSetup(request);
  for (const changed of [{ deviceId: randomUUID() }, { deviceToken: randomBytes(32).toString('base64url') }]) {
    await assert.rejects(() => h.service.completeOwnerSetup({ ...request, ...changed }), /active pairing/);
  }
  assert.deepEqual(await h.service.completeOwnerSetup(request), receipt);
  assert.deepEqual(h.service.inspectSession().deviceIDs, [request.deviceId]);
});

test('stop and closed-core states block setup without resuming work or losing pairing', async t => {
  const h = await fixture(t);
  const request = prepare(h.service.beginOwnerSetup()), receipt = await h.service.completeOwnerSetup(request);
  assert.equal((await h.api(request.deviceToken, '/v1/host/stop', {})).status, 200);
  await h.restart();
  for (let attempt = 0; attempt < 2; attempt++) {
    assert.throws(() => h.service.beginOwnerSetup(), /running local host/);
    await assert.rejects(() => h.service.completeOwnerSetup(request), /running local host/);
    assert.equal((await h.api(request.deviceToken, '/v1/agents', { name: 'Must remain stopped', instructions: 'Synthetic', model: 'exact' })).status, 503);
    await h.restart();
  }
  await h.service.close();
  assert.throws(() => h.service.beginOwnerSetup(), /running local host/);
  await assert.rejects(() => h.service.completeOwnerSetup(request), /running local host/);
  await h.restart({ resumeOwnerHost: true });
  assert.deepEqual(await h.service.completeOwnerSetup(request), receipt);
});

test('revoked and superseded setup requests cannot recreate pairing across later generations', async t => {
  const h = await fixture(t);
  const prior: ReturnType<typeof prepare>[] = [];
  for (let generation = 0; generation < 3; generation++) {
    const request = prepare(h.service.beginOwnerSetup());
    for (const old of prior) await assert.rejects(() => h.service.completeOwnerSetup(old), /current exact offer/);
    await h.service.completeOwnerSetup(request);
    assert.equal((await h.api(request.deviceToken, '/v1/pairing/disconnect', {})).status, 200);
    await assert.rejects(() => h.service.completeOwnerSetup(request), /running local host/);
    await h.restart({ resumeOwnerHost: true });
    await assert.rejects(() => h.service.completeOwnerSetup(request), /active pairing/);
    assert.equal((await h.api(request.deviceToken)).status, 401);
    assert.deepEqual(h.service.inspectSession().deviceIDs, []);
    prior.push(request);
  }
});

test('legacy enrollment supersedes an outstanding setup even after disconnect and explicit resume', async t => {
  const h = await fixture(t);
  const request = prepare(h.service.beginOwnerSetup());
  const response = await fetch(h.service.baseURL + '/v1/devices/enroll', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...h.service.issueEnrollment(), deviceName: 'Synthetic legacy device' }),
  });
  assert.equal(response.status, 201);
  const legacy = await response.json() as { deviceId: string; deviceToken: string; expiresAt: null };
  assert.deepEqual(h.service.beginOwnerSetup(), { kind: 'paired', installationId: request.installationId, deviceId: legacy.deviceId, expiresAt: null });
  await assert.rejects(() => h.service.completeOwnerSetup(request));
  assert.equal((await h.api(legacy.deviceToken, '/v1/pairing/disconnect', {})).status, 200);
  await h.restart({ resumeOwnerHost: true });
  await assert.rejects(() => h.service.completeOwnerSetup(request), /current exact offer/);
  const fresh = prepare(h.service.beginOwnerSetup());
  assert.notEqual(fresh.setupId, request.setupId);
  await h.service.completeOwnerSetup(fresh);
});

test('temporary and bounded sessions cannot use owner setup', async t => {
  for (const credentialDeadline of [undefined, Date.now() + 60_000]) {
    const h = await fixture(t, { ownerPairingPolicy: undefined, credentialDeadline });
    assert.throws(() => h.service.beginOwnerSetup(), /owner-local installation/);
    await assert.rejects(() => h.service.completeOwnerSetup({ installationId: h.service.installation.installationId, setupId: randomUUID(), deviceId: randomUUID(), deviceToken: randomBytes(32).toString('base64url') }), /owner-local installation/);
    await assert.rejects(h.service.cancelOwnerSetup({ installationId: h.service.installation.installationId, setupId: randomUUID(), deviceId: randomUUID() }), /owner-local installation/);
    assert.deepEqual(h.service.inspectSession().deviceIDs, []);
  }
});

test('local setup adds no unauthenticated HTTP route and consumes legacy enrollment on completion', async t => {
  const h = await fixture(t);
  const enrollment = h.service.issueEnrollment(), request = prepare(h.service.beginOwnerSetup());
  for (const path of ['/v1/owner/setup', '/v1/owner/setup/complete', '/v1/owner/setup/cancel']) {
    const response = await fetch(h.service.baseURL + path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(request) });
    assert.equal(response.status, 401);
  }
  await h.service.completeOwnerSetup(request);
  assert.equal((await h.api(request.deviceToken, '/v1/owner/setup', {})).status, 404);
  assert.equal((await h.api(request.deviceToken, '/v1/owner/setup/cancel', setupIdentity(request))).status, 404);
  const response = await fetch(h.service.baseURL + '/v1/devices/enroll', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...enrollment, deviceName: 'Stale legacy device' }) });
  assert.equal(response.status, 401);
  assert.deepEqual(h.service.inspectSession().deviceIDs, [request.deviceId]);
});

test('the persisted setup and pairing contain only identity, expiry and a credential hash', async t => {
  const h = await fixture(t);
  const request = prepare(h.service.beginOwnerSetup());
  await h.service.completeOwnerSetup(request);
  await h.service.close();
  const db = new DatabaseSync(join(h.directory, 'openai-candidate-b.sqlite'), { readOnly: true });
  try {
    const pairing = db.prepare('SELECT id,hash,expires FROM devices').get();
    assert.deepEqual({ ...pairing }, { id: request.deviceId, hash: createHash('sha256').update(request.deviceToken).digest('hex'), expires: 0 });
    const metadata = db.prepare('SELECT data FROM metadata').all();
    assert.equal(JSON.stringify(metadata).includes(request.deviceToken), false);
  } finally { db.close(); }
});

test('corrupt or unsupported setup records fail closed without exposing stored contents', async t => {
  const h = await fixture(t);
  const request = prepare(h.service.beginOwnerSetup());
  for (const data of ['{"secret":"synthetic-secret"', JSON.stringify({ version: 2, state: 'pending', setupId: request.setupId, expires: Date.now() + 60_000 }), ...['completed', 'canceling', 'canceled'].map(state => JSON.stringify({ version: 1, state, setupId: request.setupId, expires: Date.now() + 60_000 }))]) {
    await h.service.close();
    const db = new DatabaseSync(join(h.directory, 'openai-candidate-b.sqlite'));
    try { db.prepare("UPDATE metadata SET data=? WHERE key='ownerSetup'").run(data); }
    finally { db.close(); }
    await h.restart();
    for (const operation of [async () => h.service.beginOwnerSetup(), () => h.service.completeOwnerSetup(request)]) {
      await assert.rejects(operation, error => error instanceof Error && error.message === 'Stored owner setup cannot be recovered.');
    }
    await assert.rejects(h.service.cancelOwnerSetup(setupIdentity(request)), error => error instanceof Error && error.message === 'Stored owner setup cannot be recovered.');
    assert.deepEqual(h.service.inspectSession().deviceIDs, []);
  }
});
