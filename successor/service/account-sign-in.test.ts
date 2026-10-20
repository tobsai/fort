import test, { type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, realpath, rm, readFile, writeFile } from 'node:fs/promises';
import { DatabaseSync } from 'node:sqlite';
import { join } from 'node:path';
import { randomBytes, randomUUID } from 'node:crypto';
import { createApplication } from './openai.ts';
import type { ChatRuntime } from '../runner/chat-runtime.ts';
import type { Agent, RuntimeDescriptor } from '../contracts/v1.ts';
type Reply = { state?: string; operationId?: string; revision?: number; challenge?: { userCode: string }; selected?: boolean; result?: { kind: string; account?: { accountId: string; userLabel: string } } };

async function fixture(t: TestContext, behavior: 'pending' | 'race' | 'badChallenge' | 'badResult' | 'unsupported' | 'failCleanup' | 'busyCatalog' | 'lyingReadiness' | 'lateAccount' = 'pending') {
  const directory = await realpath(await mkdtemp('/tmp/fort059-owner-account-'));
  const catalogStarted = Promise.withResolvers<void>(), catalogDone = Promise.withResolvers<void>();
  let restarts = 0, offset = 0;
  let starts = 0, executions = 0, catalogs = 0, cleanupFails = behavior === 'failCleanup', finish: ((value: { kind: 'signedIn' | 'canceled'; account?: { accountId: string; userLabel: string } }) => void) | undefined;
  const runtime: ChatRuntime = { execute: async () => { executions++; return { kind: 'notStarted', message: 'Synthetic' }; },
    models: async (_signal, account) => { assert.deepEqual(account, selectedAccount); catalogs++; catalogStarted.resolve(); if (behavior === 'busyCatalog') await catalogDone.promise; return { provider: 'independent-account', state: 'unavailable', models: [], detail: 'Synthetic' }; },
    dispose: async () => { catalogDone.resolve(); finish?.({ kind: 'canceled' }); if (cleanupFails) throw new Error('Synthetic cleanup failure'); },
    async signIn(options, signal) {
      starts++;
      if (behavior === 'badResult') return { kind: 'signedIn', token: 'synthetic-private' };
      options.onChallenge({ kind: 'deviceCode', verificationURL: 'https://device.fixture.invalid/verify', userCode: 'ONLY-IN-MEMORY', ...(behavior === 'badChallenge' ? { token: 'synthetic-private' } : {}) });
      return new Promise(resolve => { finish = resolve; if (signal.aborted) resolve({ kind: 'canceled' }); else signal.addEventListener('abort', () => resolve(behavior === 'lateAccount' ? { kind: 'signedIn', account: selectedAccount } : { kind: behavior === 'race' ? 'signedIn' : 'canceled' }), { once: true }); });
    } };
  if (behavior === 'lyingReadiness') { runtime.readiness = () => ({ state: 'ready', canRestart: true, detail: 'Synthetic' }); runtime.restart = async () => { restarts++; }; }
  if (behavior === 'unsupported') delete runtime.signIn;
  const provider: RuntimeDescriptor = { provider: 'independent-account', name: 'Independent fixture', runtimeVersion: 'fixture-7', hostId: 'fixture-host', session: 'stateless', capabilities: ['plainText', 'modelCatalog'] };
  const options = { provider, dataDirectory: directory, ownerPairingPolicy: { expiresAfterDays: null }, runtime, now: () => Date.now() + offset };
  let service = createApplication(options); await service.start();
  t.after(async () => { cleanupFails = false; await service.close(); await rm(directory, { recursive: true, force: true }); });
  const host = { deviceId: randomUUID(), deviceToken: randomBytes(32).toString('base64url') };
  const setup = service.beginOwnerSetup(); assert.equal(setup.kind, 'available');
  if (setup.kind !== 'available') throw new Error('Synthetic setup missing');
  await service.completeOwnerSetup({ ...host, installationId: setup.installationId, setupId: setup.setupId });
  async function api<T = Reply>(path: string, body?: unknown, token = host.deviceToken) {
    const response = await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers: { 'Content-Type': 'application/json', 'X-Fort-Installation-Id': service.installation.installationId, Authorization: 'Bearer ' + token }, body: body === undefined ? undefined : JSON.stringify(body) });
    return { status: response.status, body: await response.json() as T };
  }
  return { directory, api, host, runtime, advance(ms: number) { offset += ms; }, catalogStarted: catalogStarted.promise, releaseCatalog: () => catalogDone.resolve(), get restarts() { return restarts; }, get service() { return service; }, get starts() { return starts; }, get executions() { return executions; }, get catalogs() { return catalogs; }, cleanupSucceeds() { cleanupFails = false; },
    request(revision = 0) { return { operationId: randomUUID(), expectedRevision: revision, recoveryEpoch: service.installation.recoveryEpoch, expiresAt: Date.now() + offset + 30_000 }; },
    finish(account?: { accountId: string; userLabel: string }) { finish?.({ kind: 'signedIn', ...(account ? { account } : {}) }); }, async reopen() { await service.close(); service = createApplication(options); await service.start(); } };
}

test('primary owner explicitly starts, reconciles and cancels one account attempt without persisting its code', async t => {
  const h = await fixture(t);
  const initial = await h.api('/v1/account/sign-in');
  assert.equal(initial.status, 200); assert.equal(initial.body.state, 'idle'); assert.equal(h.starts, 0);
  const request = h.request(), started = await h.api('/v1/account/sign-in/start', request);
  assert.equal(started.status, 200); assert.equal(started.body.operationId, request.operationId);
  const status = await h.api('/v1/account/sign-in');
  assert.equal(status.body.state, 'challenge'); assert.equal(status.body.challenge?.userCode, 'ONLY-IN-MEMORY');
  assert.equal((await h.api('/v1/account/sign-in/start', request)).body.operationId, request.operationId);
  assert.equal(h.starts, 1);
  const canceled = await h.api('/v1/account/sign-in/cancel', { operationId: request.operationId, revision: 1, recoveryEpoch: request.recoveryEpoch });
  assert.equal(canceled.status, 200); assert.equal(canceled.body.result?.kind, 'canceled');
  await h.reopen();
  assert.equal((await h.api('/v1/account/sign-in/start', request)).body.result?.kind, 'canceled'); assert.equal(h.starts, 1);
  const contents = await readFile(join(h.directory, 'openai-candidate-b.sqlite'));
  assert.equal(contents.includes(Buffer.from('ONLY-IN-MEMORY')), false);
});

test('account request scope, immutable deadline and revision exclude duplicates and stale starts', async t => {
  const h = await fixture(t), first = h.request();
  for (const change of [{ expectedRevision: -1 }, { expectedRevision: 1 }, { recoveryEpoch: 'foreign' }, { operationId: '' },
    { expiresAt: Date.now() - 1 }, { expiresAt: Date.now() + 116_000 }, { expiresAt: 'later' }, { token: 'synthetic-private' }]) {
    assert.ok([400, 409].includes((await h.api('/v1/account/sign-in/start', { ...first, ...change })).status));
  }
  assert.equal(h.starts, 0);
  assert.equal((await h.api('/v1/account/sign-in/start', first)).status, 200);
  for (const request of [h.request(1), { ...first, expectedRevision: 1 }, { ...first, expiresAt: first.expiresAt + 1 }]) {
    assert.equal((await h.api('/v1/account/sign-in/start', request)).status, 409);
  }
  const cancel = { operationId: first.operationId, revision: 1, recoveryEpoch: first.recoveryEpoch };
  assert.equal((await h.api('/v1/account/sign-in/cancel', { ...cancel, operationId: 'foreign' })).status, 409);
  assert.equal((await h.api('/v1/account/sign-in/cancel', cancel)).status, 200);
  assert.equal((await h.api('/v1/account/sign-in/cancel', cancel)).status, 200);
  assert.equal((await h.api('/v1/account/sign-in/start', h.request())).status, 409);
  assert.equal((await h.api('/v1/account/sign-in/start', h.request(1))).status, 200);
  assert.equal((await h.api('/v1/account/sign-in/start', first)).status, 409); assert.equal(h.starts, 2);
});

test('companion cannot observe or operate the primary account challenge', async t => {
  const h = await fixture(t), offer = { invitationId: randomUUID(), code: randomBytes(32).toString('base64url') };
  assert.equal((await h.api('/v1/pairing/invitation', offer)).status, 201);
  const phone = { ...offer, installationId: h.service.installation.installationId, deviceId: randomUUID(), deviceToken: randomBytes(32).toString('base64url'), deviceName: 'Synthetic phone' };
  assert.equal((await h.api('/v1/devices/join', phone)).status, 201);
  const request = h.request(); await h.api('/v1/account/sign-in/start', request);
  for (const [path, body] of [['/v1/account/sign-in', undefined], ['/v1/account/sign-in/start', request],
    ['/v1/account/sign-in/select', { operationId: request.operationId, revision: 1, recoveryEpoch: request.recoveryEpoch, account: selectedAccount }],
    ['/v1/account/sign-in/cancel', { operationId: request.operationId, revision: 1, recoveryEpoch: request.recoveryEpoch }]] as const) {
    const reply = await h.api(path, body, phone.deviceToken); assert.equal(reply.status, 403);
    assert.doesNotMatch(JSON.stringify(reply), /ONLY-IN-MEMORY|device.fixture/);
  }
  assert.equal(h.starts, 1);
});

test('pending account work excludes dispatch and catalog calls without relying on runtime readiness', async t => {
  const h = await fixture(t); await h.api('/v1/account/sign-in/start', h.request());
  assert.equal((await h.api('/v1/models')).body.state, 'unavailable'); assert.equal(h.catalogs, 0);
  const agent = (await h.api<Agent>('/v1/agents', { name: 'Synthetic agent', model: 'exact', instructions: 'Synthetic' })).body;
  assert.equal((await h.api('/v1/conversations/' + agent.homeConversationId + '/messages', { requestId: randomUUID(), text: 'Synthetic queue', recipientAgentId: agent.agentId,
    bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] })).status, 409);
  assert.equal(await h.service.dispatchNext(), false); assert.equal(h.executions, 0);
});

test('runtime account completion is validated and cancellation preserves a completed login race', async t => {
  for (const behavior of ['race', 'badChallenge', 'badResult'] as const) await t.test(behavior, async t => {
    const h = await fixture(t, behavior), request = h.request(); await h.api('/v1/account/sign-in/start', request);
    const result = await h.api('/v1/account/sign-in/cancel', { operationId: request.operationId, revision: 1, recoveryEpoch: request.recoveryEpoch });
    assert.equal(result.status, 200); assert.equal(result.body.result?.kind, behavior === 'race' ? 'signedIn' : 'unconfirmed');
    assert.doesNotMatch(JSON.stringify(result), /synthetic-private|ONLY-IN-MEMORY/);
  });
});

test('host stop and disconnect await the account operation and retain failed cleanup for retry', async t => {
  for (const action of ['stop', 'disconnect', 'failCleanup'] as const) await t.test(action, async t => {
    const h = await fixture(t, action === 'failCleanup' ? action : 'pending'), request = h.request();
    await h.api('/v1/account/sign-in/start', request);
    const route = action === 'disconnect' ? '/v1/pairing/disconnect' : '/v1/host/stop';
    assert.equal((await h.api(route, {})).status, action === 'failCleanup' ? 503 : 200);
    if (action === 'disconnect') assert.equal((await h.api('/v1/account/sign-in')).status, 401);
    else {
      assert.equal((await h.api('/v1/account/sign-in')).body.result?.kind, 'canceled');
      assert.equal((await h.api('/v1/account/sign-in/start', h.request(1))).status, 409);
      h.cleanupSucceeds(); assert.equal((await h.api(route, {})).status, 200);
    }
  });
});

test('an interrupted persisted attempt becomes unconfirmed on restart without invoking a provider', async t => {
  const h = await fixture(t), request = h.request(); await h.api('/v1/account/sign-in/start', request); await h.service.close();
  const db = new DatabaseSync(join(h.directory, 'openai-candidate-b.sqlite'));
  db.prepare("UPDATE metadata SET data=? WHERE key='accountSignIn'").run(JSON.stringify({ revision: 1, operationId: request.operationId, deviceId: h.host.deviceId,
    recoveryEpoch: request.recoveryEpoch, expiresAt: request.expiresAt, state: 'active' })); db.close();
  await h.reopen();
  const status = await h.api('/v1/account/sign-in'); assert.equal(status.body.result?.kind, 'unconfirmed');
  assert.equal((await h.api('/v1/account/sign-in/start', request)).body.result?.kind, 'unconfirmed'); assert.equal(h.starts, 1);
  const events = await h.api('/v1/events?after=0&recoveryEpoch=' + request.recoveryEpoch);
  assert.doesNotMatch(JSON.stringify(events), /ONLY-IN-MEMORY|device.fixture/);
});

test('missing capability is explicit and status does not start account work', async t => {
  const h = await fixture(t, 'unsupported');
  assert.equal((await h.api('/v1/account/sign-in')).body.state, 'unavailable');
  assert.equal((await h.api('/v1/account/sign-in/start', h.request())).status, 409); assert.equal(h.starts, 0);
});

test('account start cannot overlap an already running catalog request', async t => {
  const h = await fixture(t, 'busyCatalog'), first = h.request();
  await h.api('/v1/account/sign-in/start', first); h.finish(selectedAccount); await h.api('/v1/account/sign-in');
  await h.api('/v1/account/sign-in/select', { operationId: first.operationId, revision: 1, recoveryEpoch: first.recoveryEpoch, account: selectedAccount });
  const catalog = h.api('/v1/models');
  await h.catalogStarted;
  try { assert.equal((await h.api('/v1/account/sign-in/start', h.request(1))).status, 409); assert.equal(h.starts, 1); }
  finally { h.releaseCatalog(); await catalog; }
});

test('service account ownership overrides a runtime that still reports ready to restart', async t => {
  const h = await fixture(t, 'lyingReadiness'); await h.api('/v1/account/sign-in/start', h.request());
  assert.equal((await h.api('/v1/runtime')).body.state, 'busy');
  assert.equal((await h.api('/v1/runtime/restart', { recoveryEpoch: h.service.installation.recoveryEpoch })).status, 409);
  assert.equal(h.restarts, 0);
});

test('owner account HTTP journeys meet their isolated budget', { skip: !process.env.FORT_OWNER_ACCOUNT_BENCHMARK }, async t => {
  const h = await fixture(t), samples: number[] = [];
  for (let i = 0; i < 25; i++) {
    const request = h.request(i), before = performance.now();
    assert.equal((await h.api('/v1/account/sign-in/start', request)).status, 200);
    assert.equal((await h.api('/v1/account/sign-in/cancel', { operationId: request.operationId, revision: i + 1, recoveryEpoch: request.recoveryEpoch })).status, 200);
    if (i >= 5) samples.push(performance.now() - before);
  }
  samples.sort((a, b) => a - b); const result = { samples, warmups: 5, p50: samples[9], p95: samples[18], maximum: samples[19], budgetMs: 100 };
  assert.ok(result.p95 < 100); await writeFile(process.env.FORT_OWNER_ACCOUNT_BENCHMARK!, JSON.stringify(result) + '\n', { flag: 'wx', mode: 0o600 });
});

const selectedAccount = { accountId: 'workspace-fixture', userLabel: 'owner@fixture.invalid' };
test('owner explicitly selects the reported account and restart retains the exact choice', async t => {
  const h = await fixture(t), request = h.request();
  await h.api('/v1/account/sign-in/start', request); h.finish(selectedAccount);
  const reported = await h.api('/v1/account/sign-in');
  assert.deepEqual(reported.body.result?.account, selectedAccount);
  assert.equal(reported.body.selected, false);
  await h.api('/v1/models'); assert.equal(h.catalogs, 0);
  const confirmation = { operationId: request.operationId, revision: 1, recoveryEpoch: request.recoveryEpoch, account: selectedAccount };
  for (const change of [{ revision: 2 }, { recoveryEpoch: 'foreign' }, { account: { ...selectedAccount, accountId: 'foreign' } }])
    assert.equal((await h.api('/v1/account/sign-in/select', { ...confirmation, ...change })).status, 409);
  const selected = await h.api('/v1/account/sign-in/select', confirmation);
  assert.equal(selected.status, 200); assert.equal(selected.body.selected, true);
  assert.equal((await h.api('/v1/account/sign-in/select', confirmation)).body.selected, true);
  await h.reopen();
  assert.equal((await h.api('/v1/account/sign-in')).body.selected, true);
  await h.api('/v1/models'); assert.equal(h.catalogs, 1);
  await h.api('/v1/account/sign-in/start', h.request(1));
  await h.api('/v1/models'); assert.equal(h.catalogs, 1);
});

test('a queued turn pins the selected identity and refuses a later account choice', async t => {
  const h = await fixture(t), first = h.request();
  await h.api('/v1/account/sign-in/start', first); h.finish(selectedAccount); await h.api('/v1/account/sign-in');
  await h.api('/v1/account/sign-in/select', { operationId: first.operationId, revision: 1, recoveryEpoch: first.recoveryEpoch, account: selectedAccount });
  const agent = (await h.api<Agent>('/v1/agents', { name: 'Synthetic agent', model: 'exact', instructions: 'Synthetic' })).body;
  const receipt = await h.api<{ run: { configuration: { account: unknown } } }>('/v1/conversations/' + agent.homeConversationId + '/messages', {
    requestId: randomUUID(), text: 'Synthetic pending', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId,
    recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] });
  assert.equal(receipt.status, 201); assert.deepEqual(receipt.body.run.configuration.account, selectedAccount);
  const next = h.request(1), changed = { ...selectedAccount, accountId: 'new-workspace' };
  await h.api('/v1/account/sign-in/start', next); h.finish(changed); await h.api('/v1/account/sign-in');
  await h.api('/v1/account/sign-in/select', { operationId: next.operationId, revision: 2, recoveryEpoch: next.recoveryEpoch, account: changed });
  assert.equal(await h.service.dispatchNext(), true); assert.equal(h.executions, 0);
});

test('account selection meets its isolated local budget', { skip: !process.env.FORT_ACCOUNT_SELECTION_BENCHMARK }, async t => {
  const h = await fixture(t), samples: number[] = [];
  for (let i = 0; i < 25; i++) {
    const request = h.request(i); await h.api('/v1/account/sign-in/start', request); h.finish(selectedAccount); await h.api('/v1/account/sign-in');
    const before = performance.now();
    assert.equal((await h.api('/v1/account/sign-in/select', { operationId: request.operationId, revision: i + 1, recoveryEpoch: request.recoveryEpoch, account: selectedAccount })).body.selected, true);
    if (i >= 5) samples.push(performance.now() - before);
  }
  samples.sort((a,b) => a-b); const result = { warmups: 5, samples, p50: samples[9], p95: samples[18], maximum: samples[19], budgetMs: 5 };
  assert.ok(result.p95 < 5); await writeFile(process.env.FORT_ACCOUNT_SELECTION_BENCHMARK!, JSON.stringify(result) + '\n', { flag: 'wx', mode: 0o600 });
});


test('fresh exact account reauthentication reuses the last explicit choice across restart', async t => {
  const h = await fixture(t), first = h.request();
  await h.api('/v1/account/sign-in/start', first); h.finish(selectedAccount); await h.api('/v1/account/sign-in');
  await h.api('/v1/account/sign-in/select', { operationId: first.operationId, revision: 1, recoveryEpoch: first.recoveryEpoch, account: selectedAccount });
  await h.reopen();
  const next = h.request(1); await h.api('/v1/account/sign-in/start', next);
  await h.api('/v1/models'); assert.equal(h.catalogs, 0);
  h.finish(selectedAccount);
  assert.equal((await h.api('/v1/account/sign-in')).body.selected, true);
  await h.api('/v1/models'); assert.equal(h.catalogs, 1);
});


async function confirmAccount(h: Awaited<ReturnType<typeof fixture>>, revision = 0, account = selectedAccount) {
  const request = h.request(revision); await h.api('/v1/account/sign-in/start', request); h.finish(account);
  await h.api('/v1/account/sign-in');
  assert.equal((await h.api('/v1/account/sign-in/select', { operationId: request.operationId, revision: revision + 1, recoveryEpoch: request.recoveryEpoch, account })).body.selected, true);
}

test('reauthentication never implicitly selects changed, missing or ambiguous identities', async t => {
  const cases = [undefined, { ...selectedAccount, accountId: 'another-workspace' }, { ...selectedAccount, userLabel: 'another@fixture.invalid' },
    { ...selectedAccount, accounts: [selectedAccount, { ...selectedAccount, accountId: 'another-workspace' }] }];
  for (const account of cases) await t.test(account === undefined ? 'missing' : 'different or ambiguous', async t => {
    const h = await fixture(t); await confirmAccount(h);
    await h.api('/v1/account/sign-in/start', h.request(1)); h.finish(account);
    assert.notEqual((await h.api('/v1/account/sign-in')).body.selected, true);
    await h.api('/v1/models'); assert.equal(h.catalogs, 0);
  });
});

test('an explicit replacement account supersedes the old choice instead of accumulating trusted accounts', async t => {
  const h = await fixture(t); await confirmAccount(h);
  const replacement = { ...selectedAccount, accountId: 'another-workspace' };
  await confirmAccount(h, 1, replacement); await h.reopen();
  await h.api('/v1/account/sign-in/start', h.request(2)); h.finish(selectedAccount);
  assert.equal((await h.api('/v1/account/sign-in')).body.selected, false);
  await h.api('/v1/account/sign-in/start', h.request(3)); h.finish(replacement);
  assert.equal((await h.api('/v1/account/sign-in')).body.selected, true);
});

test('expired or canceled sign-in completion cannot reuse a remembered identity', async t => {
  for (const kind of ['expired', 'canceled'] as const) await t.test(kind, async t => {
    const h = await fixture(t, 'lateAccount'); await confirmAccount(h);
    const next = h.request(1); await h.api('/v1/account/sign-in/start', next);
    if (kind === 'expired') { h.advance(40_000); h.finish(selectedAccount); }
    else await h.api('/v1/account/sign-in/cancel', { operationId: next.operationId, revision: 2, recoveryEpoch: next.recoveryEpoch });
    assert.equal((await h.api('/v1/account/sign-in')).body.selected, false);
    await h.api('/v1/models'); assert.equal(h.catalogs, 0);
  });
});

test('legacy explicit choice is retained but stale or malformed remembered scopes cannot select', async t => {
  for (const change of ['legacy', 'installationId', 'recoveryEpoch', 'provider', 'hostId', 'malformed'] as const) await t.test(change, async t => {
    const h = await fixture(t); await confirmAccount(h); await h.service.close();
    const db = new DatabaseSync(join(h.directory, 'openai-candidate-b.sqlite'));
    if (change === 'legacy') db.prepare("DELETE FROM metadata WHERE key='accountChoice'").run();
    else if (change === 'malformed') db.prepare("UPDATE metadata SET data='{}' WHERE key='accountChoice'").run();
    else db.prepare("UPDATE metadata SET data=json_set(data,?,?) WHERE key='accountChoice'").run('$.' + change, 'different-scope');
    db.close(); await h.reopen();
    await h.api('/v1/account/sign-in/start', h.request(1)); h.finish(selectedAccount);
    assert.equal((await h.api('/v1/account/sign-in')).body.selected, change === 'legacy');
    await h.api('/v1/models'); assert.equal(h.catalogs, change === 'legacy' ? 1 : 0);
  });
});

test('interrupted reauthentication retains the choice but waits for a fresh matching provider result', async t => {
  const h = await fixture(t); await confirmAccount(h);
  const next = h.request(1); await h.api('/v1/account/sign-in/start', next); await h.service.close();
  const db = new DatabaseSync(join(h.directory, 'openai-candidate-b.sqlite'));
  db.prepare("UPDATE metadata SET data=? WHERE key='accountSignIn'").run(JSON.stringify({ revision: 2, operationId: next.operationId, deviceId: h.host.deviceId,
    recoveryEpoch: next.recoveryEpoch, expiresAt: next.expiresAt, state: 'active' })); db.close(); await h.reopen();
  assert.equal((await h.api('/v1/account/sign-in')).body.result?.kind, 'unconfirmed');
  await h.api('/v1/models'); assert.equal(h.catalogs, 0);
  await h.api('/v1/account/sign-in/start', h.request(2)); h.finish(selectedAccount);
  assert.equal((await h.api('/v1/account/sign-in')).body.selected, true);
  await h.api('/v1/models'); assert.equal(h.catalogs, 1);
});


test('legacy confirmed identity survives explicit owner re-pairing but still requires fresh runtime evidence', async t => {
  const h = await fixture(t, 'lyingReadiness'); await confirmAccount(h); await h.service.close();
  const db = new DatabaseSync(join(h.directory, 'openai-candidate-b.sqlite'));
  db.prepare("DELETE FROM metadata WHERE key='accountChoice'").run(); db.close(); await h.reopen();
  assert.equal((await h.api('/v1/pairing/disconnect', {})).status, 200);
  h.host.deviceId = randomUUID(); h.host.deviceToken = randomBytes(32).toString('base64url');
  const setup = h.service.beginOwnerSetup(); assert.equal(setup.kind, 'available');
  if (setup.kind !== 'available') throw new Error('Synthetic re-pair missing');
  await h.service.completeOwnerSetup({ ...h.host, installationId: setup.installationId, setupId: setup.setupId });
  await h.api('/v1/models'); assert.equal(h.catalogs, 0);
  await h.api('/v1/account/sign-in/start', h.request(1)); h.finish(selectedAccount);
  assert.equal((await h.api('/v1/account/sign-in')).body.selected, true);
  await h.api('/v1/models'); assert.equal(h.catalogs, 1);
});
