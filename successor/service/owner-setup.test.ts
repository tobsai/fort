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

function prepare(offer: OwnerSetupState) {
  assert.equal(offer.kind, 'available');
  return { installationId: offer.installationId, setupId: offer.setupId, deviceId: randomUUID(), deviceToken: randomBytes(32).toString('base64url') };
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
  const receipt = h.service.completeOwnerSetup(request);
  assert.deepEqual(receipt, { installationId: request.installationId, deviceId: request.deviceId, expiresAt: null });
  await h.restart();
  assert.deepEqual(h.service.completeOwnerSetup(request), receipt);
  assert.deepEqual(h.service.inspectSession().deviceIDs, [request.deviceId]);
  assert.deepEqual(h.service.beginOwnerSetup(), { kind: 'paired', ...receipt });
  assert.equal((await h.api(request.deviceToken)).status, 200);
  assert.equal((await h.api(randomBytes(32).toString('base64url'))).status, 401);
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
  assert.throws(() => h.service.completeOwnerSetup(request), /expired/);
  assert.deepEqual(h.service.inspectSession().deviceIDs, []);
  const replacement = prepare(h.service.beginOwnerSetup());
  assert.notEqual(replacement.setupId, request.setupId);
  assert.throws(() => h.service.completeOwnerSetup(request), /current exact offer/);
  h.service.completeOwnerSetup(replacement);
  assert.equal((await h.api(replacement.deviceToken)).status, 200);
});

test('completed retries retain the original pairing expiry beyond the setup offer lifetime', async t => {
  let now = Date.parse('2026-10-18T00:00:00Z');
  const h = await fixture(t, { now: () => now, ownerPairingPolicy: { expiresAfterDays: 1 } });
  const request = prepare(h.service.beginOwnerSetup()), receipt = h.service.completeOwnerSetup(request);
  assert.equal(receipt.expiresAt, new Date(now + 86_400_000).toISOString());
  now += 86_399_999;
  await h.restart();
  assert.deepEqual(h.service.completeOwnerSetup(request), receipt);
  assert.equal((await h.api(request.deviceToken)).status, 200);
  now++;
  assert.throws(() => h.service.completeOwnerSetup(request), /active pairing/);
  assert.equal((await h.api(request.deviceToken)).status, 401);
  const replacement = prepare(h.service.beginOwnerSetup());
  h.service.completeOwnerSetup(replacement);
  assert.throws(() => h.service.completeOwnerSetup(request), /current exact offer/);
  assert.deepEqual(h.service.inspectSession().deviceIDs, [replacement.deviceId]);
});

test('conflicting identity or credential cannot confirm or replace completed setup', async t => {
  const h = await fixture(t);
  const request = prepare(h.service.beginOwnerSetup());
  assert.throws(() => h.service.completeOwnerSetup({ ...request, installationId: randomUUID() }), /exact installation/);
  assert.throws(() => h.service.completeOwnerSetup({ ...request, setupId: randomUUID() }), /exact offer/);
  assert.throws(() => h.service.completeOwnerSetup({ ...request, deviceId: '../other' }), /identifier/);
  for (const deviceToken of ['x', '+'.repeat(43), 'A'.repeat(42) + 'B']) {
    assert.throws(() => h.service.completeOwnerSetup({ ...request, deviceToken }), /32-byte/);
  }
  assert.deepEqual(h.service.inspectSession().deviceIDs, []);
  const receipt = h.service.completeOwnerSetup(request);
  for (const changed of [{ deviceId: randomUUID() }, { deviceToken: randomBytes(32).toString('base64url') }]) {
    assert.throws(() => h.service.completeOwnerSetup({ ...request, ...changed }), /active pairing/);
  }
  assert.deepEqual(h.service.completeOwnerSetup(request), receipt);
  assert.deepEqual(h.service.inspectSession().deviceIDs, [request.deviceId]);
});

test('stop and closed-core states block setup without resuming work or losing pairing', async t => {
  const h = await fixture(t);
  const request = prepare(h.service.beginOwnerSetup()), receipt = h.service.completeOwnerSetup(request);
  assert.equal((await h.api(request.deviceToken, '/v1/host/stop', {})).status, 200);
  await h.restart();
  for (let attempt = 0; attempt < 2; attempt++) {
    assert.throws(() => h.service.beginOwnerSetup(), /running local host/);
    assert.throws(() => h.service.completeOwnerSetup(request), /running local host/);
    assert.equal((await h.api(request.deviceToken, '/v1/agents', { name: 'Must remain stopped', instructions: 'Synthetic', model: 'exact' })).status, 503);
    await h.restart();
  }
  await h.service.close();
  assert.throws(() => h.service.beginOwnerSetup(), /running local host/);
  assert.throws(() => h.service.completeOwnerSetup(request), /running local host/);
  await h.restart({ resumeOwnerHost: true });
  assert.deepEqual(h.service.completeOwnerSetup(request), receipt);
});

test('revoked and superseded setup requests cannot recreate pairing across later generations', async t => {
  const h = await fixture(t);
  const prior: ReturnType<typeof prepare>[] = [];
  for (let generation = 0; generation < 3; generation++) {
    const request = prepare(h.service.beginOwnerSetup());
    for (const old of prior) assert.throws(() => h.service.completeOwnerSetup(old), /current exact offer/);
    h.service.completeOwnerSetup(request);
    assert.equal((await h.api(request.deviceToken, '/v1/pairing/disconnect', {})).status, 200);
    assert.throws(() => h.service.completeOwnerSetup(request), /running local host/);
    await h.restart({ resumeOwnerHost: true });
    assert.throws(() => h.service.completeOwnerSetup(request), /active pairing/);
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
  assert.throws(() => h.service.completeOwnerSetup(request));
  assert.equal((await h.api(legacy.deviceToken, '/v1/pairing/disconnect', {})).status, 200);
  await h.restart({ resumeOwnerHost: true });
  assert.throws(() => h.service.completeOwnerSetup(request), /current exact offer/);
  const fresh = prepare(h.service.beginOwnerSetup());
  assert.notEqual(fresh.setupId, request.setupId);
  h.service.completeOwnerSetup(fresh);
});

test('temporary and bounded sessions cannot use owner setup', async t => {
  for (const credentialDeadline of [undefined, Date.now() + 60_000]) {
    const h = await fixture(t, { ownerPairingPolicy: undefined, credentialDeadline });
    assert.throws(() => h.service.beginOwnerSetup(), /owner-local installation/);
    assert.throws(() => h.service.completeOwnerSetup({ installationId: h.service.installation.installationId, setupId: randomUUID(), deviceId: randomUUID(), deviceToken: randomBytes(32).toString('base64url') }), /owner-local installation/);
    assert.deepEqual(h.service.inspectSession().deviceIDs, []);
  }
});

test('local setup adds no unauthenticated HTTP route and consumes legacy enrollment on completion', async t => {
  const h = await fixture(t);
  const enrollment = h.service.issueEnrollment(), request = prepare(h.service.beginOwnerSetup());
  for (const path of ['/v1/owner/setup', '/v1/owner/setup/complete']) {
    const response = await fetch(h.service.baseURL + path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(request) });
    assert.equal(response.status, 401);
  }
  h.service.completeOwnerSetup(request);
  assert.equal((await h.api(request.deviceToken, '/v1/owner/setup', {})).status, 404);
  const response = await fetch(h.service.baseURL + '/v1/devices/enroll', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...enrollment, deviceName: 'Stale legacy device' }) });
  assert.equal(response.status, 401);
  assert.deepEqual(h.service.inspectSession().deviceIDs, [request.deviceId]);
});

test('the persisted setup and pairing contain only identity, expiry and a credential hash', async t => {
  const h = await fixture(t);
  const request = prepare(h.service.beginOwnerSetup());
  h.service.completeOwnerSetup(request);
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
  for (const data of ['{"secret":"synthetic-secret"', JSON.stringify({ version: 2, state: 'pending', setupId: request.setupId, expires: Date.now() + 60_000 }), JSON.stringify({ version: 1, state: 'completed', setupId: request.setupId, expires: Date.now() + 60_000 })]) {
    await h.service.close();
    const db = new DatabaseSync(join(h.directory, 'openai-candidate-b.sqlite'));
    try { db.prepare("UPDATE metadata SET data=? WHERE key='ownerSetup'").run(data); }
    finally { db.close(); }
    await h.restart();
    for (const operation of [() => h.service.beginOwnerSetup(), () => h.service.completeOwnerSetup(request)]) {
      assert.throws(operation, error => error instanceof Error && error.message === 'Stored owner setup cannot be recovered.');
    }
    assert.deepEqual(h.service.inspectSession().deviceIDs, []);
  }
});
