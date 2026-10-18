import { test } from 'node:test';
import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { mkdtemp, rm, readFile, stat, mkdir, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { DatabaseSync } from 'node:sqlite';
import { createApplication } from './openai.ts';
import type { Acceptance, Agent, RuntimeDescriptor, Snapshot } from '../contracts/v1.ts';
import type { ChatInput } from '../runner/chat-runtime.ts';

const run = promisify(execFile);
const migration = new URL('./migrate-ledger.ts', import.meta.url).pathname;
const filename = 'openai-candidate-b.sqlite';
const provider: RuntimeDescriptor = { provider: 'migration-fixture', name: 'Migration fixture', runtimeVersion: '1', hostId: 'synthetic', session: 'stateless', capabilities: ['plainText'] };

test('offline copy compacts legacy duplicates, replays exact receipts, and leaves rollback source unchanged', async t => {
  const root = await mkdtemp(join(tmpdir(), 'fort059-migration-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const source = join(root, 'source'), destination = join(root, 'destination');
  let lastInput: ChatInput | undefined;
  let executions = 0;
  const runtime = { async execute(input: ChatInput) { lastInput = input; executions++; return { kind: 'completed' as const, text: 'r'.repeat(2048), effectiveModel: null }; }, async dispose() {} };
  let app = createApplication({ dataDirectory: source, provider, runtime });
  t.after(() => app.close());
  await app.start();
  const enrolled = await fetch(app.baseURL + '/v1/devices/enroll', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...app.issueEnrollment(), deviceName: 'Synthetic migration device' }) });
  const device = await enrolled.json() as { deviceToken: string };
  const headers = { 'Content-Type': 'application/json', Authorization: `Bearer ${device.deviceToken}`, 'X-Fort-Installation-Id': app.installation.installationId };
  async function api<T>(path: string, body?: unknown): Promise<T> {
    const response = await fetch(app.baseURL + path, { headers, method: body === undefined ? 'GET' : 'POST', body: body === undefined ? undefined : JSON.stringify(body) });
    assert.ok(response.ok, `HTTP ${response.status}`);
    return await response.json() as T;
  }
  const agent = await api<Agent>('/v1/agents', { name: 'Writer', model: 'exact-fixture-model', instructions: 'Reply.' });
  const path = `/v1/conversations/${agent.homeConversationId}`;
  const commands = Array.from({ length: 30 }, (_, index) => ({ requestId: 'copy-' + index, text: `${index}:` + 'u'.repeat(2048), recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: app.installation.recoveryEpoch, skills: [] }));
  const receipts = [];
  for (const command of commands) {
    receipts.push((await api<Acceptance>(path + '/messages', command)).receipt);
    await app.dispatchNext();
  }
  const snapshot = await api<Snapshot>(path + '/snapshot');
  await app.close();
  assert.ok(lastInput);
  const fixture = new DatabaseSync(join(source, filename));
  for (const row of fixture.prepare("SELECT id,data FROM entities WHERE kind='requests'").all()) {
    const stored = JSON.parse(String(row.data));
    fixture.prepare("UPDATE entities SET data=? WHERE kind='requests' AND id=?").run(JSON.stringify({ fingerprint: stored.fingerprint, acceptance: { receipt: stored.receipt, snapshot } }), row.id!);
  }
  const { attempt: _attempt, ...legacyContext } = lastInput;
  fixture.exec("DELETE FROM entities WHERE kind='contexts'");
  fixture.prepare('INSERT INTO entities VALUES (?,?,?)').run('contexts', lastInput.runId, JSON.stringify(legacyContext));
  fixture.close();
  const original = await readFile(join(source, filename));
  const copied = await run(process.execPath, [migration, '--source', join(source, filename), '--destination', destination]);
  const report = JSON.parse(copied.stdout);
  assert.equal(report.receiptsCompacted, 30);
  assert.equal(report.contextsCompacted, 1);
  assert.ok(report.destinationBytes < original.length / 3);
  assert.equal((await stat(destination)).mode & 0o777, 0o700);
  assert.equal((await stat(join(destination, filename))).mode & 0o777, 0o600);
  assert.deepEqual(await readFile(join(source, filename)), original);
  app = createApplication({ dataDirectory: destination, provider, runtime });
  await app.start();
  assert.deepEqual(await api<Snapshot>(path + '/snapshot'), snapshot);
  assert.deepEqual((await api<Acceptance>('/v1/requests/copy-0')).receipt, receipts[0]);
  assert.deepEqual((await api<Acceptance>(path + '/messages', commands[0])).receipt, receipts[0]);
  assert.equal(await app.dispatchNext(), false);
  assert.equal(executions, 30);
  await app.close();
  assert.deepEqual(await readFile(join(source, filename)), original);
  app = createApplication({ dataDirectory: source, provider, runtime });
  await app.start();
  assert.deepEqual((await api<Acceptance>('/v1/requests/copy-0')).receipt, receipts[0]);
  assert.deepEqual(await api<Snapshot>(path + '/snapshot'), snapshot);
  assert.equal(await app.dispatchNext(), false);
  await app.close();
  await assert.rejects(run(process.execPath, [migration, '--source', join(source, filename), '--destination', destination]));
  const second = join(root, 'second');
  const repeated = JSON.parse((await run(process.execPath, [migration, '--source', join(destination, filename), '--destination', second])).stdout);
  assert.equal(repeated.receiptsCompacted, 0);
  assert.equal(repeated.contextsCompacted, 0);
  const corrupt = new DatabaseSync(join(source, filename));
  corrupt.prepare("UPDATE entities SET data=json_set(data,'$.acceptance.receipt.inputDigest',?) WHERE kind='requests' AND id='copy-0'").run('0'.repeat(64));
  corrupt.close();
  const corruptedSource = await readFile(join(source, filename));
  const rejected = join(root, 'rejected');
  await assert.rejects(run(process.execPath, [migration, '--source', join(source, filename), '--destination', rejected]));
  await assert.rejects(stat(rejected), { code: 'ENOENT' });
  assert.deepEqual(await readFile(join(source, filename)), corruptedSource);
});

test('unsupported data fails without changing the source or an existing destination', async t => {
  const root = await mkdtemp(join(tmpdir(), 'fort059-migration-reject-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const source = join(root, 'unsupported.sqlite'), destination = join(root, 'copy');
  const db = new DatabaseSync(source); db.exec('CREATE TABLE unrelated (id INTEGER)'); db.close();
  const original = await readFile(source);
  await assert.rejects(run(process.execPath, [migration, '--source', source, '--destination', destination]));
  await assert.rejects(stat(destination), { code: 'ENOENT' });
  assert.deepEqual(await readFile(source), original);
  await mkdir(destination); await writeFile(join(destination, 'keep'), 'untouched');
  await assert.rejects(run(process.execPath, [migration, '--source', source, '--destination', destination]));
  assert.equal(await readFile(join(destination, 'keep'), 'utf8'), 'untouched');
});
