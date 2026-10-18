import assert from 'node:assert/strict';
import { test } from 'node:test';
import { mkdtempSync, realpathSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { DatabaseSync } from 'node:sqlite';
import { PassThrough } from 'node:stream';
import { createApplication } from '../service/openai.ts';
import { serveHistoryMaintenance } from './history-maintenance.ts';

const identity = { installationId: 'installation', recoveryEpoch: 'old-epoch', deviceId: 'device', deletionId: 'confirmed-deletion', timeout: 5 };
function fixture(t: { after(fn: () => void): void }) {
  const directory = realpathSync(mkdtempSync('/tmp/fort059-maintenance-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const service = createApplication({ dataDirectory: directory, ownerPairingPolicy: { expiresAfterDays: null },
    provider: { provider: 'maintenance-fixture', name: 'Synthetic', runtimeVersion: '1', hostId: 'exact', session: 'stateless', capabilities: ['plainText'] } });
  return { directory, async prepare() {
    await service.close();
    const db = new DatabaseSync(join(directory, 'openai-candidate-b.sqlite'));
    db.prepare("UPDATE metadata SET data=? WHERE key='installation'").run(JSON.stringify({ contractVersion: '1.0', mode: 'local', serviceName: 'Fort', installationId: identity.installationId, recoveryEpoch: identity.recoveryEpoch }));
    db.prepare("INSERT INTO metadata VALUES ('ownerControl',?)").run(JSON.stringify({ deviceId: identity.deviceId, intent: 'disconnect' }));
    db.prepare("INSERT INTO entities VALUES ('messages','synthetic',?)").run(JSON.stringify({ text: 'Erase this synthetic history' }));
    db.close();
  }, read() {
    const db = new DatabaseSync(join(directory, 'openai-candidate-b.sqlite'), { readOnly: true });
    try { return db.prepare("SELECT data FROM entities WHERE kind='messages'").all(); } finally { db.close(); }
  } };
}
type Frame = { protocolVersion: number; kind: string; id?: number; response: { status: number; body: string } };
function channel(directory: string, signal?: AbortSignal) {
  const input = new PassThrough(), output = new PassThrough(); let pending = ''; const values: Frame[] = [];
  const waiters: Array<(value: Frame) => void> = [];
  output.on('data', (chunk: Buffer) => {
    pending += chunk.toString('utf8');
    for (let newline; (newline = pending.indexOf('\n')) >= 0;) {
      const frame = JSON.parse(pending.slice(0, newline)); pending = pending.slice(newline + 1);
      const waiter = waiters.shift(); if (waiter) waiter(frame); else values.push(frame);
    }
  });
  const done = serveHistoryMaintenance({ directory, input, output, signal });
  return { input, output, done, next: () => Promise.race([values.length ? Promise.resolve(values.shift()!) : new Promise<Frame>(resolve => waiters.push(resolve)), done.then(() => { throw new Error('Channel ended before response'); })]) };
}
test('private maintenance accepts one exact request, emits a correlated receipt and exits on parent EOF without starting a service', async t => {
  const h = fixture(t); await h.prepare(); const c = channel(h.directory);
  c.done.catch(() => {});
  assert.deepEqual(await c.next(), { protocolVersion: 1, kind: 'ready' });
  c.input.write(JSON.stringify({ protocolVersion: 1, kind: 'historyDeletion', id: 1, history: identity }) + '\n');
  const frame = await c.next(); assert.equal(frame.id, 1); assert.equal(frame.response.status, 200);
  const result = JSON.parse(Buffer.from(frame.response.body, 'base64').toString('utf8'));
  assert.equal(result.installationId, identity.installationId); assert.equal(result.deletionId, identity.deletionId);
  assert.notEqual(result.recoveryEpoch, identity.recoveryEpoch); assert.deepEqual(h.read(), []);
  c.input.end(); await c.done;
});

const command = { protocolVersion: 1, kind: 'historyDeletion', id: 1, history: identity };
for (const [name, value] of Object.entries({
  version: { ...command, protocolVersion: 2 }, id: { ...command, id: 2 }, http: { ...command, kind: 'request' },
  path: { ...command, directory: '/foreign' }, extra: { ...command, history: { ...identity, directory: '/foreign' } },
  missing: { ...command, history: { installationId: 'installation' } }, identifier: { ...command, history: { ...identity, deviceId: 'invalid/path' } },
  timeout: { ...command, history: { ...identity, timeout: 31 } }, zeroTimeout: { ...command, history: { ...identity, timeout: 0 } },
  null: null, array: [], oversized: 'x'.repeat(5000),
})) test(`maintenance refuses ${name} before deletion`, async t => {
  const h = fixture(t); await h.prepare(); const c = channel(h.directory), rejected = assert.rejects(c.done);
  await c.next(); c.input.end(JSON.stringify(value) + '\n'); await rejected; assert.equal(h.read().length, 1);
});

test('partial, invalid UTF-8 and two commands in one frame cannot delete', async t => {
  for (const bytes of [Buffer.from('{'), Buffer.from([0xff, 10]), Buffer.from(JSON.stringify(command) + '\n' + JSON.stringify(command) + '\n')]) {
    const h = fixture(t); await h.prepare(); const c = channel(h.directory), rejected = assert.rejects(c.done);
    await c.next(); c.input.end(bytes); await rejected; assert.equal(h.read().length, 1);
  }
});

test('foreign identity returns unconfirmed and exact receipt retry does not rotate the epoch twice', async t => {
  const h = fixture(t); await h.prepare();
  const invoke = async (history: typeof identity) => {
    const c = channel(h.directory); await c.next(); c.input.write(JSON.stringify({ ...command, history }) + '\n');
    const result = await c.next(); c.input.end(); await c.done; return result;
  };
  const refused = await invoke({ ...identity, installationId: 'foreign' });
  assert.equal(refused.response.status, 409); assert.deepEqual(JSON.parse(Buffer.from(refused.response.body, 'base64').toString()), { outcome: 'unconfirmed' });
  assert.equal(h.read().length, 1);
  assert.deepEqual(await invoke(identity), await invoke(identity)); assert.deepEqual(h.read(), []);
});

test('parent cancellation or empty EOF before a command preserves history', async t => {
  const h = fixture(t); await h.prepare(); const controller = new AbortController(), c = channel(h.directory, controller.signal);
  const rejected = assert.rejects(c.done); await c.next(); controller.abort(); await rejected; assert.equal(h.read().length, 1);
  const empty = channel(h.directory); await empty.next(); empty.input.end(); await empty.done; assert.equal(h.read().length, 1);
});
