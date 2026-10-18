import assert from 'node:assert/strict';
import { test, type TestContext } from 'node:test';
import { randomBytes, randomUUID } from 'node:crypto';
import { chmod, link, mkdir, mkdtemp, readFile, realpath, rename, rm, symlink, truncate, writeFile } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import { DatabaseSync } from 'node:sqlite';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createApplication } from './openai.ts';
import { deleteOwnerHistory } from './delete-history.ts';
import type { Agent, RoutineRevision, RuntimeDescriptor, SchedulePreview, ScheduleRevision, Snapshot } from '../contracts/v1.ts';

const provider: RuntimeDescriptor = { provider: 'history-fixture', name: 'Synthetic history deletion', runtimeVersion: '1', hostId: 'exact-local', session: 'stateless', capabilities: ['plainText'] };
const filename = 'openai-candidate-b.sqlite';

async function fixture(t: TestContext) {
  const directory = await realpath(await mkdtemp(join(tmpdir(), 'fort059-history-')));
  let executions = 0, now = Date.parse('2026-10-18T09:00:00Z');
  const create = () => createApplication({ dataDirectory: directory, provider, now: () => now, ownerPairingPolicy: { expiresAfterDays: null }, runtime: {
    async execute() { executions++; return { kind: 'completed' as const, text: 'Synthetic reply to erase', effectiveModel: null }; }, async dispose() {},
  } });
  let service = create(); await service.start();
  t.after(async () => { await service.close(); await rm(directory, { recursive: true, force: true }); });
  async function pair() {
    const offer = service.beginOwnerSetup(); assert.equal(offer.kind, 'available');
    const device = { installationId: offer.installationId, setupId: offer.setupId, deviceId: randomUUID(), deviceToken: randomBytes(32).toString('base64url') };
    await service.completeOwnerSetup(device); return device;
  }
  let device = await pair();
  async function api<T>(path: string, body?: unknown) {
    const response = await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers: {
      Authorization: 'Bearer ' + device.deviceToken, 'X-Fort-Installation-Id': service.installation.installationId, 'Content-Type': 'application/json',
    }, body: body === undefined ? undefined : JSON.stringify(body) });
    return { status: response.status, body: await response.json() as T };
  }
  const agent = (await api<Agent>('/v1/agents', { name: 'Retain this agent', instructions: 'Keep these settings.', model: 'exact' })).body;
  const conversation = '/v1/conversations/' + agent.homeConversationId;
  const command = { requestId: 'old-pending', text: 'Synthetic message to erase', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: service.installation.recoveryEpoch, skills: [] };
  assert.equal((await api(conversation + '/messages', command)).status, 201); await service.dispatchNext();
  const request = { directory, installationId: service.installation.installationId, recoveryEpoch: service.installation.recoveryEpoch, deviceId: device.deviceId, deletionId: randomUUID() };
  return { directory, path: join(directory, filename), request, agent, command, conversation, api,
    get service() { return service; }, get executions() { return executions; },
    advance(milliseconds: number) { now += milliseconds; },
    async disconnect() { assert.equal((await api('/v1/pairing/disconnect', {})).status, 200); },
    async close() { await service.close(); },
    async reopen() { service = create(); await service.start(); device = await pair(); },
  };
}

test('explicit offline deletion preserves identity/settings, fences old pending input and permits new messages after re-pair', async t => {
  const h = await fixture(t);
  assert.equal((await h.api<Snapshot>(h.conversation + '/snapshot')).body.messages.length, 2);
  await h.disconnect(); await h.close();
  const result = deleteOwnerHistory(h.request);
  assert.notEqual(result.recoveryEpoch, h.request.recoveryEpoch);
  await h.reopen();
  assert.equal(h.service.installation.installationId, h.request.installationId);
  assert.equal(h.service.installation.recoveryEpoch, result.recoveryEpoch);
  assert.deepEqual((await h.api<{ agents: Agent[] }>('/v1/agents')).body.agents, [h.agent]);
  const empty = (await h.api<Snapshot>(h.conversation + '/snapshot')).body;
  assert.deepEqual([empty.messages, empty.runs, empty.approvals], [[], [], []]);
  assert.equal((await h.api(h.conversation + '/messages', h.command)).status, 409);
  assert.equal(await h.service.dispatchNext(), false); assert.equal(h.executions, 1);
  assert.equal((await h.api(h.conversation + '/messages', { ...h.command, requestId: 'new-message', recoveryEpoch: result.recoveryEpoch })).status, 201);
  await h.service.dispatchNext();
  assert.equal((await h.api<Snapshot>(h.conversation + '/snapshot')).body.messages.length, 2);
});

function edit(path: string, action: (db: DatabaseSync) => void) {
  const db = new DatabaseSync(path); try { action(db); } finally { db.close(); }
}
function contents(path: string) {
  const db = new DatabaseSync(path, { readOnly: true });
  try { return ['metadata', 'entities', 'events', 'devices', 'enrollment', 'dispatch_outbox'].map(table => db.prepare(`SELECT * FROM ${table} ORDER BY rowid`).all()); }
  finally { db.close(); }
}
async function stopped(t: TestContext) { const h = await fixture(t); await h.disconnect(); await h.close(); return h; }

test('all execution records and live SQLite text are erased; configuration, owner notes, external files and exact retry survive', async t => {
  const h = await stopped(t), marker = 'SYNTHETIC-HISTORY-REMOVE-' + randomUUID();
  const kept = ['conversationMemory', 'skills', 'skillRevisions', 'routines', 'routineRevisions', 'scheduleRevisions', 'disabledBindings'];
  const erased = ['reads', 'routineApprovals', 'requestCancellations', 'scheduleOccurrences'];
  edit(h.path, db => {
    for (const kind of kept) db.prepare('INSERT INTO entities VALUES (?,?,?)').run(kind, 'retained', JSON.stringify({ fixture: 'Retain exact configured bytes' }));
    db.prepare('INSERT INTO entities VALUES (?,?,?)').run('schedules', 'paused', JSON.stringify({ state: 'paused', checkedThrough: 12345, definition: { fixture: 'Keep' } }));
    for (const kind of erased) db.prepare('INSERT INTO entities VALUES (?,?,?)').run(kind, 'erase', JSON.stringify({ fixture: marker }));
    db.prepare("UPDATE entities SET data=json_set(data,'$.text',?) WHERE kind='messages'").run(marker);
    db.prepare("INSERT INTO dispatch_outbox(run_id,conversation_id,state,generation) VALUES ('queued','fixture','queued',0)").run();
  });
  await mkdir(join(h.directory, 'Runtime'), { mode: 0o700 });
  const outside = join(h.directory, 'Runtime', 'synthetic-provider-state'); await writeFile(outside, 'Retain unrelated synthetic state');
  const before = contents(h.path)[1]!.filter(row => !['messages', 'runs', 'requests', 'contexts', ...erased].includes(String(row.kind)));
  const result = deleteOwnerHistory(h.request); assert.ok(result.deletedRecords > 10);
  assert.deepEqual(contents(h.path)[1], before);
  assert.deepEqual(contents(h.path).slice(2), [[], [], [], []]);
  assert.equal((await readFile(h.path)).includes(Buffer.from(marker)), false);
  for (const suffix of ['-wal', '-shm', '-journal']) {
    try { assert.equal((await readFile(h.path + suffix)).includes(Buffer.from(marker)), false); }
    catch (error) { if (!(error instanceof Error && 'code' in error && error.code === 'ENOENT')) throw error; }
  }
  assert.equal(await readFile(outside, 'utf8'), 'Retain unrelated synthetic state');
  const after = contents(h.path); assert.deepEqual(deleteOwnerHistory(h.request), result); assert.deepEqual(contents(h.path), after);
  assert.throws(() => deleteOwnerHistory({ ...h.request, deletionId: randomUUID() }));
  assert.throws(() => deleteOwnerHistory({ ...h.request, recoveryEpoch: result.recoveryEpoch }));
  assert.deepEqual(contents(h.path), after);
});

test('live or merely stopped host cannot grant offline deletion', async t => {
  const h = await fixture(t);
  assert.throws(() => deleteOwnerHistory(h.request), /locked/);
  assert.equal((await h.api<Snapshot>(h.conversation + '/snapshot')).body.messages.length, 2);
  assert.equal((await h.api('/v1/host/stop', {})).status, 200);
  await h.close(); const before = contents(h.path);
  assert.throws(() => deleteOwnerHistory(h.request)); assert.deepEqual(contents(h.path), before);
});

test('retained paused schedules need fresh consent and cannot replay erased occurrences after re-pair', async t => {
  const h = await fixture(t), agent = h.agent;
  const routine = (await h.api<RoutineRevision>('/v1/routines', { definition: {
    schemaVersion: 1, name: 'Retain this routine', agentId: agent.agentId, conversationId: agent.homeConversationId,
    bindingRevisionId: agent.binding.bindingRevisionId, instructions: 'Synthetic task.', inputs: [], skills: [],
    steps: [{ stepId: 'task', name: 'Task', type: 'task', instructions: 'Write.', inputNames: [], outputStepIds: [] }],
  } })).body;
  const schedule = (await h.api<ScheduleRevision>('/v1/schedules', { definition: {
    schemaVersion: 1, routineId: routine.routineId, routineRevisionId: routine.revisionId,
    kind: 'daily', timezone: 'UTC', time: '09:01', inputs: {}, missed: 'catchUpOne',
  } })).body;
  const path = '/v1/schedules/' + schedule.scheduleId;
  const oldPreview = (await h.api<SchedulePreview>(path + '/preview')).body;
  const state = { revisionId: schedule.revisionId, recoveryEpoch: h.request.recoveryEpoch };
  assert.equal((await h.api(path + '/state', { ...state, previewDigest: oldPreview.digest, enable: true })).status, 200);
  h.advance(60_001); assert.equal(h.service.tickSchedules(), 1); await h.service.dispatchNext();
  const paused = (await h.api<ScheduleRevision>(path + '/state', { ...state, enable: false })).body;
  const memory = (await h.api(h.conversation + '/memory', { recoveryEpoch: h.request.recoveryEpoch, expectedRevisionId: null, text: 'Retain these owner notes.' })).body;
  await h.disconnect(); await h.close(); deleteOwnerHistory(h.request); h.advance(3 * 86_400_000); await h.reopen();
  assert.deepEqual((await h.api<{ routines: RoutineRevision[] }>('/v1/routines')).body.routines, [routine]);
  assert.deepEqual((await h.api<{ schedules: ScheduleRevision[] }>('/v1/schedules')).body.schedules, [paused]);
  assert.deepEqual((await h.api<Snapshot>(h.conversation + '/snapshot')).body.memory, memory);
  assert.deepEqual((await h.api<{ occurrences: unknown[] }>(path + '/occurrences')).body.occurrences, []);
  assert.equal(h.service.tickSchedules(true), 0); assert.equal(await h.service.dispatchNext(), false); assert.equal(h.executions, 2);
  const currentState = { ...state, recoveryEpoch: h.service.installation.recoveryEpoch, enable: true };
  assert.equal((await h.api(path + '/state', { ...currentState, previewDigest: oldPreview.digest })).status, 409);
  const fresh = (await h.api<SchedulePreview>(path + '/preview')).body;
  assert.equal((await h.api(path + '/state', { ...currentState, previewDigest: fresh.digest })).status, 200);
  assert.equal(h.service.tickSchedules(true), 0); assert.equal(await h.service.dispatchNext(), false); assert.equal(h.executions, 2);
});

test('wrong installation, device, epoch and deletion identifiers never remove records', async t => {
  const h = await stopped(t), before = contents(h.path);
  for (const key of ['installationId', 'deviceId', 'recoveryEpoch'] as const) {
    assert.throws(() => deleteOwnerHistory({ ...h.request, [key]: randomUUID() }));
    assert.deepEqual(contents(h.path), before);
  }
  for (const deletionId of ['', 'bad/path', 'x'.repeat(129)]) assert.throws(() => deleteOwnerHistory({ ...h.request, deletionId }));
  assert.deepEqual(contents(h.path), before);
});

const unsafeState: Record<string, string> = {
  'bounded historical grant': "INSERT INTO metadata VALUES ('credentialDeadline','0')",
  'stale private socket': "INSERT INTO metadata VALUES ('privateSocketOwnership','{}')",
  'unknown metadata': "INSERT INTO metadata VALUES ('futureHistory','{}')",
  'unknown entity': "INSERT INTO entities VALUES ('futureHistory','x','{}')",
  'unknown table': 'CREATE TABLE future_history(data TEXT)',
  'unknown column': 'ALTER TABLE events ADD COLUMN future_data TEXT',
  'view': 'CREATE VIEW future_history AS SELECT * FROM events',
  'trigger': 'CREATE TRIGGER future_history AFTER DELETE ON events BEGIN DELETE FROM metadata; END',
  'active device': "INSERT INTO devices VALUES ('other','synthetic',0)",
  'expired device': "INSERT INTO devices VALUES ('other','synthetic',1)",
  'pending enrollment': "INSERT INTO enrollment VALUES ('synthetic',1)",
  'pending setup': "UPDATE metadata SET data=json_set(data,'$.state','pending') WHERE key='ownerSetup'",
  'canceling setup': "UPDATE metadata SET data=json_set(data,'$.state','canceling') WHERE key='ownerSetup'",
  'foreign setup': "UPDATE metadata SET data=json_set(data,'$.deviceId','other') WHERE key='ownerSetup'",
  'future setup version': "UPDATE metadata SET data=json_set(data,'$.version',2) WHERE key='ownerSetup'",
  'future deletion receipt': `INSERT INTO metadata VALUES ('historyDeletion','{"version":2}')`,
  'wrong outbox version': "UPDATE metadata SET data='2' WHERE key='outboxVersion'",
  'missing pairing policy': "DELETE FROM metadata WHERE key='ownerPairingPolicy'",
  'invalid pairing policy': "UPDATE metadata SET data='{}' WHERE key='ownerPairingPolicy'",
  'enabled schedule': `INSERT INTO entities VALUES ('schedules','x','{"state":"enabled","checkedThrough":12345}')`,
  'invalid schedule progress': `INSERT INTO entities VALUES ('schedules','x','{"state":"paused","checkedThrough":null}')`,
};
for (const [name, sql] of Object.entries(unsafeState)) test(`deletion refuses ${name} without changing stored rows`, async t => {
  const h = await stopped(t); edit(h.path, db => db.exec(sql)); const before = contents(h.path);
  assert.throws(() => deleteOwnerHistory(h.request)); assert.deepEqual(contents(h.path), before);
});

test('transaction failure rolls back deletion and a post-commit cleanup failure permits exact retry', async t => {
  const h = await stopped(t), before = contents(h.path), original = DatabaseSync.prototype.exec;
  const commit = t.mock.method(DatabaseSync.prototype, 'exec', function (this: DatabaseSync, sql: string) {
    if (sql === 'COMMIT') throw new Error('Synthetic commit failure');
    return Reflect.apply(original, this, [sql]);
  });
  assert.throws(() => deleteOwnerHistory(h.request), /Synthetic commit/); commit.mock.restore();
  assert.deepEqual(contents(h.path), before);
  const vacuum = t.mock.method(DatabaseSync.prototype, 'exec', function (this: DatabaseSync, sql: string) {
    if (sql.includes('VACUUM')) throw new Error('Synthetic cleanup failure');
    return Reflect.apply(original, this, [sql]);
  });
  assert.throws(() => deleteOwnerHistory(h.request), /records were deleted; SQLite cleanup is unconfirmed/); vacuum.mock.restore();
  const committed = contents(h.path);
  assert.equal(committed[1]!.some(row => row.kind === 'messages'), false);
  const result = deleteOwnerHistory(h.request);
  assert.notEqual(result.recoveryEpoch, h.request.recoveryEpoch); assert.deepEqual(contents(h.path), committed);
});

const unsafeFiles: Record<string, (path: string, directory: string) => Promise<void>> = {
  'ledger symlink': async path => { await rename(path, path + '.kept'); await symlink(path + '.kept', path); },
  'ledger hardlink': async path => { await link(path, path + '.kept'); },
  'ledger directory': async path => { await rename(path, path + '.kept'); await mkdir(path); },
  'ledger FIFO': async path => { await rename(path, path + '.kept'); execFileSync('/usr/bin/mkfifo', [path]); },
  'ledger shared mode': async path => { await chmod(path, 0o640); },
  'directory shared mode': async (_path, directory) => { await chmod(directory, 0o750); },
  'sidecar symlink': async path => { await symlink(path, path + '-wal'); },
  'sidecar hardlink': async path => { await writeFile(path + '.kept', 'fixture', { mode: 0o600 }); await link(path + '.kept', path + '-shm'); },
  'sidecar FIFO': async path => { execFileSync('/usr/bin/mkfifo', [path + '-wal']); },
  'oversized ledger': async path => { await truncate(path, 256 * 1024 * 1024 + 1); },
};
for (const [name, mutate] of Object.entries(unsafeFiles)) test(`deletion refuses ${name} before SQLite opens it`, async t => {
  const h = await stopped(t); await mutate(h.path, h.directory);
  assert.throws(() => deleteOwnerHistory(h.request));
});

test('canonical directory and existing SQLite header are required without creating a new ledger', async t => {
  const h = await stopped(t), original = await readFile(h.path);
  assert.throws(() => deleteOwnerHistory({ ...h.request, directory: '.' }));
  assert.throws(() => deleteOwnerHistory({ ...h.request, directory: h.directory + '/' }));
  const alias = h.directory + '-alias'; await symlink(h.directory, alias); t.after(() => rm(alias));
  assert.throws(() => deleteOwnerHistory({ ...h.request, directory: alias }));
  assert.deepEqual(await readFile(h.path), original);
  await writeFile(h.path, 'Not SQLite'); assert.throws(() => deleteOwnerHistory(h.request));
  assert.equal(await readFile(h.path, 'utf8'), 'Not SQLite');
});

test('10,000-message warm offline deletion stays within its two-second p95 budget', { skip: !process.env.FORT_HISTORY_SAMPLES }, async t => {
  const samples: number[] = [];
  for (let sample = 0; sample < 20; sample++) {
    const h = await stopped(t);
    edit(h.path, db => {
      db.exec('BEGIN');
      const insert = db.prepare('INSERT INTO entities VALUES (?,?,?)');
      for (let i = 0; i < 10_000; i++) insert.run('messages', 'bulk-' + i, JSON.stringify({ requestId: 'bulk-' + i, role: 'user', conversationId: 'fixture', sequence: i, text: 's'.repeat(1024) }));
      db.exec('COMMIT');
    });
    const started = performance.now(); deleteOwnerHistory(h.request); samples.push(performance.now() - started);
  }
  await writeFile(process.env.FORT_HISTORY_SAMPLES!, JSON.stringify({ unit: 'milliseconds', samples, p95: [...samples].sort((a, b) => a - b)[18] }, null, 2));
  assert.ok([...samples].sort((a, b) => a - b)[18]! <= 2000);
});
