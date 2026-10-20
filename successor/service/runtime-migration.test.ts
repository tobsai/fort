import assert from 'node:assert/strict';
import { test, type TestContext } from 'node:test';
import { DatabaseSync } from 'node:sqlite';
import { randomUUID, randomBytes } from 'node:crypto';
import { mkdtempSync, realpathSync, rmSync, writeFileSync, readFileSync, chmodSync, symlinkSync, linkSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { cpus, release } from 'node:os';
import { join } from 'node:path';
import { createApplication } from './openai.ts';
import { migrateRuntime, type RuntimeMigrationRequest } from './runtime-migration.ts';
import type { Agent, Acceptance, RuntimeDescriptor } from '../contracts/v1.ts';

const oldRuntime: RuntimeDescriptor = { provider: 'migration-fixture', hostId: 'same-host', name: 'Synthetic runtime', runtimeVersion: 'old', session: 'stateless', capabilities: ['plainText'] };
const newRuntime: RuntimeDescriptor = { ...oldRuntime, runtimeVersion: 'new' };
async function fixture(t: TestContext) {
  const directory = realpathSync(mkdtempSync('/tmp/fort059-runtime-migration-'));
  let service = createApplication({ dataDirectory: directory, provider: oldRuntime, ownerPairingPolicy: { expiresAfterDays: null },
    runtime: { async execute(input) { return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() {} } });
  t.after(async () => { await service.close(); rmSync(directory, { recursive: true, force: true }); });
  let deviceId = randomUUID(), token = randomBytes(32).toString('base64url');
  async function pair() {
    const setup = service.beginOwnerSetup(); assert.equal(setup.kind, 'available');
    if (setup.kind !== 'available') throw new Error('Fixture setup failed');
    await service.completeOwnerSetup({ installationId: setup.installationId, setupId: setup.setupId, deviceId, deviceToken: token });
  }
  await service.start(); await pair();
  async function api<T>(path: string, body?: unknown): Promise<{ status: number; body: T }> {
    const response = await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}`, 'X-Fort-Installation-Id': service.installation.installationId },
      body: body === undefined ? undefined : JSON.stringify(body) });
    return { status: response.status, body: await response.json() as T };
  }
  const created = await api<Agent>('/v1/agents', { name: 'Retained synthetic agent', model: 'same-exact-model', instructions: 'Retain these synthetic instructions.' });
  assert.equal(created.status, 201); const agent = created.body;
  const command = { requestId: 'retained', text: 'Retained synthetic message 🌲', recipientAgentId: agent.agentId,
    bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: service.installation.recoveryEpoch, skills: [] };
  const path = `/v1/conversations/${agent.homeConversationId}`;
  assert.equal((await api(path + '/messages', command)).status, 201);
  await service.dispatchNext();
  const before = (await api<Acceptance>('/v1/requests/retained')).body;
  const request: RuntimeMigrationRequest = { action: 'apply', migrationId: randomUUID(), installationId: service.installation.installationId,
    recoveryEpoch: service.installation.recoveryEpoch, deviceId, provider: oldRuntime.provider, hostId: oldRuntime.hostId, fromVersion: 'old', toVersion: 'new' };
  async function disconnect() { assert.equal((await api('/v1/pairing/disconnect', {})).status, 200); await service.close(); }
  function rows() {
    const db = new DatabaseSync(join(directory, 'openai-candidate-b.sqlite'), { readOnly: true });
    try { return db.prepare('SELECT kind,id,data FROM entities ORDER BY kind,id').all(); } finally { db.close(); }
  }
  function edit(operation: (db: DatabaseSync) => void) {
    const db = new DatabaseSync(join(directory, 'openai-candidate-b.sqlite'));
    try { operation(db); } finally { db.close(); }
  }
  function dump() {
    let result = '';
    edit(db => {
      const tables = ['metadata','enrollment','devices','entities','events','dispatch_outbox','sqlite_sequence'];
      result = JSON.stringify(tables.map(table => [table, db.prepare(`SELECT * FROM ${table} ORDER BY rowid`).all()]));
    });
    return result;
  }
  return { directory, request, agent, before, command, api, path, rows, disconnect, edit, dump, get service() { return service; },
    async reopen(provider = newRuntime) { service = createApplication({ dataDirectory: directory, provider, runtime: { async execute(input) { return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() {} } }); await service.start(); },
    async rePair() { deviceId = randomUUID(); token = randomBytes(32).toString('base64url'); await pair(); } };
}

test('offline migration preserves exact historical rows, creates current revisions without selecting another model, and reopens through the public service', async t => {
  const f = await fixture(t); await f.disconnect(); const oldRows = f.rows();
  const receipt = migrateRuntime(f.directory, f.request);
  assert.deepEqual(receipt, { state: 'applied', migrationId: f.request.migrationId, agents: 1, disabledBindings: 1 });
  const migratedRows = f.rows();
  for (const row of oldRows.filter(row => row.kind !== 'agents')) assert.deepEqual(migratedRows.find(next => next.kind === row.kind && next.id === row.id), row);
  assert.deepEqual(migrateRuntime(f.directory, f.request), receipt); assert.deepEqual(f.rows(), migratedRows);
  assert.throws(() => createApplication({ dataDirectory: f.directory, provider: oldRuntime }), /explicit binding migration/);
  await f.reopen(); await f.rePair();
  const agents = (await f.api<{ agents: Agent[] }>('/v1/agents')).body.agents;
  assert.equal(agents.length, 1); const current = agents[0]; assert.ok(current);
  assert.notEqual(current.binding.bindingRevisionId, f.agent.binding.bindingRevisionId);
  assert.deepEqual(current, { ...f.agent, binding: { ...f.agent.binding, runtimeVersion: 'new', bindingRevisionId: current.binding.bindingRevisionId } });
  const after = (await f.api<Acceptance>('/v1/requests/retained')).body;
  assert.deepEqual(after.receipt, f.before.receipt); assert.deepEqual(after.snapshot.messages, f.before.snapshot.messages);
  assert.deepEqual(after.snapshot.runs, f.before.snapshot.runs);
  assert.equal((await f.api(f.path + '/messages', { ...f.command, requestId: 'stale-binding' })).status, 409);
  assert.equal((await f.api(f.path + '/messages', { ...f.command, requestId: 'new-version', bindingRevisionId: current.binding.bindingRevisionId })).status, 201);
});

test('rollback restores the original ledger and schema, retries harmlessly and allows the old runtime to reopen', async t => {
  const f = await fixture(t); await f.disconnect(); const before = f.dump();
  migrateRuntime(f.directory, f.request);
  const rollback = { ...f.request, action: 'rollback' as const };
  assert.equal(migrateRuntime(f.directory, rollback).state, 'rolledBack');
  assert.equal(f.dump(), before);
  assert.equal(migrateRuntime(f.directory, rollback).state, 'notApplied');
  f.edit(db => assert.equal(db.prepare("SELECT 1 FROM sqlite_schema WHERE name='runtime_migrations'").get(), undefined));
  await f.reopen(oldRuntime); await f.rePair();
  assert.deepEqual((await f.api<{agents: Agent[]}>('/v1/agents')).body.agents, [f.agent]);
  assert.deepEqual((await f.api<Acceptance>('/v1/requests/retained')).body.snapshot.messages, f.before.snapshot.messages);
});

test('rollback refuses newer ledger writes and preserves them', async t => {
  const f = await fixture(t); await f.disconnect(); migrateRuntime(f.directory, f.request);
  f.edit(db => db.prepare("UPDATE entities SET data=json_set(data,'$.text','Newer synthetic history') WHERE kind='messages' AND json_extract(data,'$.role')='assistant'").run());
  const after = f.dump();
  assert.throws(() => migrateRuntime(f.directory, { ...f.request, action: 'rollback' }), /ledger changed/);
  assert.equal(f.dump(), after);
});

for (const key of ['installationId','recoveryEpoch','deviceId','provider','hostId','fromVersion'] as const)
  test('migration refuses foreign ' + key + ' without changing the ledger', async t => {
    const f = await fixture(t); await f.disconnect(); const before = f.dump();
    assert.throws(() => migrateRuntime(f.directory, { ...f.request, [key]: 'foreign' }), /refused/);
    assert.equal(f.dump(), before);
  });

test('same operation with changed target or identity cannot be mistaken for a retry', async t => {
  const f = await fixture(t); await f.disconnect(); migrateRuntime(f.directory, f.request); const after = f.dump();
  for (const change of [{ toVersion: 'other' }, { deviceId: 'foreign' }])
    assert.throws(() => migrateRuntime(f.directory, { ...f.request, ...change }), /refused/);
  assert.equal(f.dump(), after);
});

for (const state of ['queued','running','waitingForApproval','cancelRequested','outcomeUnknown'])
  test('migration refuses unresolved ' + state + ' without retargeting or canceling it', async t => {
    const f = await fixture(t); await f.disconnect();
    f.edit(db => db.prepare("UPDATE entities SET data=json_set(data,'$.state',?) WHERE kind='runs'").run(state));
    const before = f.dump();
    assert.throws(() => migrateRuntime(f.directory, f.request), /pending or unknown work/);
    assert.equal(f.dump(), before);
  });

test('migration refuses an outbox record even if its run incorrectly appears complete', async t => {
  const f = await fixture(t); await f.disconnect();
  f.edit(db => db.prepare("INSERT INTO dispatch_outbox(run_id,conversation_id,state,held,generation) VALUES ('orphan','synthetic','queued',1,0)").run());
  const before = f.dump(); assert.throws(() => migrateRuntime(f.directory, f.request), /pending or unknown work/); assert.equal(f.dump(), before);
});

test('migration refuses enabled schedules and an active host before modifying data', async t => {
  const f = await fixture(t);
  assert.throws(() => migrateRuntime(f.directory, f.request), /locked|refused/);
  await f.disconnect();
  f.edit(db => db.prepare("INSERT INTO entities VALUES ('schedules','schedule',?)").run(JSON.stringify({ state: 'enabled' })));
  const before = f.dump(); assert.throws(() => migrateRuntime(f.directory, f.request), /Pause schedules/); assert.equal(f.dump(), before);
});

for (const bad of ['directory-mode','file-mode','symlink','hardlink','trigger','extra-table','invalid-request','same-version'])
  test('migration refuses ' + bad + ' before changes', async t => {
    const f = await fixture(t); await f.disconnect();
    const ledger = join(f.directory, 'openai-candidate-b.sqlite'); let directory = f.directory;
    if (bad === 'directory-mode') chmodSync(directory, 0o755);
    if (bad === 'file-mode') chmodSync(ledger, 0o644);
    if (bad === 'symlink') { directory = f.directory + '-link'; symlinkSync(f.directory, directory); t.after(() => rmSync(directory)); }
    if (bad === 'hardlink') linkSync(ledger, join(f.directory, 'linked.sqlite'));
    if (bad === 'trigger') f.edit(db => db.exec('CREATE TRIGGER unexpected AFTER UPDATE ON metadata BEGIN SELECT 1; END;'));
    if (bad === 'extra-table') f.edit(db => db.exec('CREATE TABLE unexpected (data TEXT)'));
    const before = f.dump();
    const request = bad === 'invalid-request' ? { ...f.request, unexpected: true } : bad === 'same-version' ? { ...f.request, toVersion: 'old' } : f.request;
    assert.throws(() => migrateRuntime(directory, request), /refused/); assert.equal(f.dump(), before);
  });

for (const action of ['apply','rollback'] as const) for (const point of ['before','after'])
  test(`public CLI ${action} recovers a process crash ${point} COMMIT without duplicate revisions or lost history`, async t => {
    const f = await fixture(t); await f.disconnect(); const before = f.dump();
    if (action === 'rollback') migrateRuntime(f.directory, f.request);
    const request = { ...f.request, action }, requestPath = join(f.directory, 'request.json');
    writeFileSync(requestPath, JSON.stringify(request), { mode: 0o600 });
    const preload = join(f.directory, 'crash.mjs'), marker = join(f.directory, 'reached');
    writeFileSync(preload, `import { DatabaseSync } from 'node:sqlite';
import { writeFileSync } from 'node:fs';
const original = DatabaseSync.prototype.exec;
DatabaseSync.prototype.exec = function (sql) {
  if (sql !== 'COMMIT') return original.call(this, sql);
  ${point === 'after' ? 'const value = original.call(this, sql);' : ''}
  writeFileSync(${JSON.stringify(marker)}, ${JSON.stringify(point)}, { mode: 0o600 });
  process.kill(process.pid, 'SIGKILL');
};\n`, { mode: 0o600 });
    const result = spawnSync(process.execPath, ['--import', preload, fileURLToPath(new URL('./runtime-migration.ts', import.meta.url)), '--directory', f.directory, '--request', requestPath], { env: {}, encoding: 'utf8', timeout: 10_000 });
    assert.equal(result.signal, 'SIGKILL', result.stderr); assert.equal(readFileSync(marker, 'utf8'), point);
    const recovered = migrateRuntime(f.directory, request);
    if (action === 'apply') {
      assert.equal(recovered.state, 'applied'); const after = f.dump();
      assert.deepEqual(migrateRuntime(f.directory, request), recovered); assert.equal(f.dump(), after);
      assert.equal(migrateRuntime(f.directory, { ...request, action: 'rollback' }).state, 'rolledBack');
    } else assert.equal(recovered.state, point === 'after' ? 'notApplied' : 'rolledBack');
    assert.equal(f.dump(), before);
  });

test('10,000 synthetic messages migrate within the five-second maintenance budget and roll back exactly', async t => {
  const f = await fixture(t); await f.disconnect();
  f.edit(db => {
    db.exec('BEGIN'); const put = db.prepare("INSERT INTO entities VALUES ('messages',?,?)");
    for (let i = 0; i < 10_000; i++) put.run('budget-' + i, JSON.stringify({ messageId: 'budget-' + i, text: 'Synthetic history '.repeat(10), sequence: i }));
    db.exec('COMMIT');
  });
  const before = f.dump(), start = performance.now();
  migrateRuntime(f.directory, f.request); const elapsedMs = performance.now() - start;
  assert.ok(elapsedMs < 5000, `Migration took ${elapsedMs} ms`);
  migrateRuntime(f.directory, { ...f.request, action: 'rollback' }); assert.equal(f.dump(), before);
  t.diagnostic(JSON.stringify({ maintenanceMs: elapsedMs, messages: 10002, node: process.version, arch: process.arch, os: release(), cpu: cpus()[0]?.model, samples: 1, cache: 'warm fixture freshly written', providerLatency: 'none' }));
});

test('explicit history deletion still works after migration without losing the current agent or reviving old bindings', async t => {
  const { deleteOwnerHistory } = await import('./delete-history.ts');
  const f = await fixture(t); await f.disconnect(); migrateRuntime(f.directory, f.request);
  const current = f.rows().filter(row => ['agents', 'agentBindings', 'disabledBindings'].includes(String(row.kind)));
  const result = deleteOwnerHistory({ directory: f.directory, installationId: f.request.installationId, recoveryEpoch: f.request.recoveryEpoch,
    deviceId: f.request.deviceId, deletionId: 'synthetic-explicit-deletion' });
  assert.ok(result.deletedRecords > 0); assert.notEqual(result.recoveryEpoch, f.request.recoveryEpoch);
  assert.deepEqual(f.rows().filter(row => ['agents', 'agentBindings', 'disabledBindings'].includes(String(row.kind))), current);
  assert.deepEqual(f.rows().filter(row => ['messages', 'runs', 'requests'].includes(String(row.kind))), []);
  assert.throws(() => migrateRuntime(f.directory, { ...f.request, action: 'rollback' }), /refused/);
});

test('migration does not revive a disabled current agent', async t => {
  const f = await fixture(t);
  assert.equal((await f.api(`/v1/agents/${f.agent.agentId}/bindings/${f.agent.binding.bindingRevisionId}/disable`, { recoveryEpoch: f.request.recoveryEpoch })).status, 200);
  await f.disconnect(); const before = f.dump();
  assert.equal(migrateRuntime(f.directory, f.request).disabledBindings, 0);
  await f.reopen(); await f.rePair();
  const current = (await f.api<{agents: Agent[]}>('/v1/agents')).body.agents[0]; assert.ok(current);
  assert.equal(current.binding.enabled, false);
  assert.equal((await f.api(f.path + '/messages', { ...f.command, requestId: 'disabled-new-version', bindingRevisionId: current.binding.bindingRevisionId })).status, 409);
  assert.ok(before.length > 0);
});

for (const bad of ['unfinished-setup','missing-run-state'])
  test('migration refuses ' + bad + ' without changing data', async t => {
    const f = await fixture(t); await f.disconnect();
    f.edit(db => {
      if (bad === 'unfinished-setup') db.prepare("UPDATE metadata SET data=json_set(data,'$.state','canceling') WHERE key='ownerSetup'").run();
      else db.prepare("UPDATE entities SET data=json_remove(data,'$.state') WHERE kind='runs'").run();
    });
    const before = f.dump(); assert.throws(() => migrateRuntime(f.directory, f.request), /refused/); assert.equal(f.dump(), before);
  });
