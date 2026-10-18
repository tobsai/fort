import assert from 'node:assert/strict';
import { test, type TestContext } from 'node:test';
import { fork, spawn } from 'node:child_process';
import { once } from 'node:events';
import { request } from 'node:http';
import { mkdtemp, mkdir, rm, stat, writeFile, readFile, chmod, symlink, lstat } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import type { Agent, Acceptance, Snapshot } from '../contracts/v1.ts';
import { DatabaseSync } from 'node:sqlite';

function object(value: unknown): Record<string, unknown> {
  assert.ok(value && typeof value === 'object' && !Array.isArray(value));
  return value as Record<string, unknown>;
}
async function root(t: TestContext, longDirectory = false) {
  const base = await mkdtemp(join(tmpdir(), 'fort-socket-'));
  const directory = longDirectory ? join(base, 'long-private-directory-'.repeat(5)) : base;
  if (longDirectory) await mkdir(directory, { mode: 0o700 });
  const closers: (() => Promise<void>)[] = [];
  t.after(async () => { for (const close of closers) await close(); await rm(base, { recursive: true, force: true }); });
  return { directory, start: (name = 'core.sock', cwd = directory, mode = '') => host(directory, name, cwd, close => closers.push(close), mode) };
}
async function host(directory: string, name: string, cwd: string, registerClose: (close: () => Promise<void>) => void, mode: string) {
  const child = fork(fileURLToPath(new URL('./fixtures/socket-host.ts', import.meta.url)), [directory, name, mode], {
    execPath: process.execPath, execArgv: [], cwd, detached: true, stdio: ['ignore', 'ignore', 'pipe', 'ipc'], env: { PATH: '/usr/bin:/bin', LANG: 'C' }
  });
  const exited = once(child, 'exit');
  let diagnostic = '', expectedExit: number | null = 0, expectedSignal: NodeJS.Signals | null = null;
  child.stderr?.on('data', bytes => { diagnostic = (diagnostic + bytes.toString()).slice(-2000); });
  const close = async () => {
    if (child.connected) child.send('close', () => {});
    await Promise.race([exited, delay(2000, undefined, { ref: false })]);
    if (child.exitCode === null && child.signalCode === null && child.pid) process.kill(-child.pid, 'SIGKILL');
    const [code, signal] = await exited;
    assert.equal(signal, expectedSignal, diagnostic); assert.equal(code, expectedExit, diagnostic);
  };
  registerClose(close);
  const value: unknown = await Promise.race([
    once(child, 'message', { signal: AbortSignal.timeout(5000) }).then(([value]) => value),
    exited.then(() => { throw new Error('Synthetic host exited before readiness: ' + diagnostic); })
  ]);
  const ready = object(value); expectedExit = ready.kind === 'failed' ? 1 : 0;
  return { ready, close, diagnostic: () => diagnostic, async crash() {
    expectedExit = null; expectedSignal = 'SIGKILL';
    child.kill('SIGKILL'); await exited;
  } };
}
async function api<T>(socketPath: string, path: string, headers: Record<string, string> = {}, body?: unknown) {
  return new Promise<{ status: number; body: T }>((resolve, reject) => {
    const encoded = body === undefined ? undefined : JSON.stringify(body);
    const call = request({ socketPath, path, method: encoded === undefined ? 'GET' : 'POST', agent: false,
      headers: { 'Content-Type': 'application/json', ...(encoded ? { 'Content-Length': String(Buffer.byteLength(encoded)) } : {}), ...headers } }, response => {
      const parts: Buffer[] = [];
      response.on('data', (bytes: Buffer) => parts.push(bytes));
      response.once('end', () => { try { resolve({ status: response.statusCode ?? 0, body: JSON.parse(Buffer.concat(parts).toString()) as T }); } catch (error) { reject(error); } });
      response.once('error', reject);
    });
    call.setTimeout(3000, () => call.destroy(new Error('Synthetic HTTP timeout')));
    call.once('error', reject); call.end(encoded);
  });
}

test('private socket transport preserves authentication, exact durable messages, replay and restart without a TCP URL', async t => {
  const fixture = await root(t), { directory } = fixture, first = await fixture.start();
  assert.equal(first.ready.kind, 'ready', first.diagnostic());
  const endpoint = object(first.ready.endpoint);
  assert.equal(endpoint.kind, 'privateSocket');
  assert.equal(endpoint.name, 'core.sock');
  assert.equal(first.ready.tcpURLAvailable, false);
  const socket = join(directory, 'core.sock');
  assert.equal((await stat(socket)).mode & 0o777, 0o600);
  assert.equal((await api(socket, '/v1/installation')).status, 401);
  const enrollment = await api<{ deviceToken: string }>(socket, '/v1/devices/enroll', {}, { ...object(first.ready.enrollment), deviceName: 'Socket fixture' });
  assert.equal(enrollment.status, 201);
  const installation = object(first.ready.installation);
  assert.ok(typeof installation.installationId === 'string');
  const headers = { Authorization: 'Bearer ' + enrollment.body.deviceToken, 'X-Fort-Installation-Id': installation.installationId };
  assert.equal((await api(socket, '/v1/installation', { ...headers, 'X-Fort-Installation-Id': 'foreign' })).status, 401);
  assert.equal((await api(socket, '/v1/installation', { ...headers, Origin: 'https://untrusted.invalid' })).status, 403);
  const agent = (await api<Agent>(socket, '/v1/agents', headers, { name: 'Local', model: 'exact-synthetic', instructions: 'Echo.' })).body;
  const path = '/v1/conversations/' + agent.homeConversationId;
  const command = { requestId: 'private-message', text: '  Cedar 🌲\r\n', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: installation.recoveryEpoch, skills: [] };
  const accepted = await api<Acceptance>(socket, path + '/messages', headers, command);
  assert.equal(accepted.status, 201);
  let snapshot = (await api<Snapshot>(socket, path + '/snapshot', headers)).body;
  for (let i = 0; i < 30 && snapshot.runs[0]?.state !== 'completed'; i++) { await delay(10); snapshot = (await api<Snapshot>(socket, path + '/snapshot', headers)).body; }
  assert.equal(snapshot.runs[0]?.state, 'completed');
  assert.deepEqual(snapshot.messages.map(message => message.text), [command.text, command.text]);
  const events = await api<{ events: unknown[] }>(socket, `/v1/events?after=0&recoveryEpoch=${installation.recoveryEpoch}`, headers);
  assert.ok(events.body.events.length > 0);
  await first.close();
  await assert.rejects(stat(socket), { code: 'ENOENT' });
  const second = await fixture.start();
  assert.equal(second.ready.kind, 'ready', second.diagnostic());
  const recovered = await api<Acceptance>(socket, path + '/messages', headers, command);
  assert.deepEqual(recovered.body.receipt, accepted.body.receipt);
  assert.equal(recovered.body.snapshot.messages.length, 2);
});

test('private socket startup refuses occupied paths, traversal, wrong working directory and shared directory permissions', async t => {
  const fixture = await root(t), { directory } = fixture, socket = join(directory, 'core.sock');
  await writeFile(socket, 'preserved marker');
  const occupied = await fixture.start();
  assert.equal(occupied.ready.kind, 'failed'); await occupied.close();
  assert.equal(await readFile(socket, 'utf8'), 'preserved marker');
  await rm(socket);
  for (const [name, cwd] of [['../escape.sock', directory], ['core.sock', tmpdir()]]) {
    assert.ok(name && cwd);
    const invalid = await fixture.start(name, cwd);
    assert.equal(invalid.ready.kind, 'failed'); await invalid.close();
    await assert.rejects(stat(socket), { code: 'ENOENT' });
  }
  await chmod(directory, 0o755);
  const shared = await fixture.start();
  assert.equal(shared.ready.kind, 'failed'); await shared.close();
  await assert.rejects(stat(socket), { code: 'ENOENT' });
});

test('a killed private core recovers its exact socket and durable receipt without reenrollment', async t => {
  for (const longDirectory of [false, true]) await t.test(longDirectory ? 'long private directory' : 'ordinary directory', async t => {
    const fixture = await root(t, longDirectory), { directory } = fixture, first = await fixture.start();
    assert.equal(first.ready.kind, 'ready', first.diagnostic());
    const socket = join(directory, 'core.sock'), installation = object(first.ready.installation);
    const alias = await mkdtemp('/tmp/fort-socket-client-');
    t.after(() => rm(alias, { recursive: true, force: true }));
    await symlink(directory, join(alias, 'data'));
    const address = join(alias, 'data/core.sock');
    assert.ok(typeof installation.installationId === 'string');
    const enrolled = await api<{ deviceToken: string }>(address, '/v1/devices/enroll', {}, { ...object(first.ready.enrollment), deviceName: 'Crash fixture' });
    assert.equal(enrolled.status, 201);
    const headers = { Authorization: 'Bearer ' + enrolled.body.deviceToken, 'X-Fort-Installation-Id': installation.installationId };
    const agent = (await api<Agent>(address, '/v1/agents', headers, { name: 'Crash Home', model: 'exact-synthetic', instructions: 'Echo.' })).body;
    const path = '/v1/conversations/' + agent.homeConversationId;
    const command = { requestId: 'socket-crash', text: 'Exact retained cedar 🌲', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: installation.recoveryEpoch, skills: [] };
    const accepted = await api<Acceptance>(address, path + '/messages', headers, command);
    assert.equal(accepted.status, 201);
    let snapshot = (await api<Snapshot>(address, path + '/snapshot', headers)).body;
    for (let index = 0; index < 30 && snapshot.runs[0]?.state !== 'completed'; index++) { await delay(10); snapshot = (await api<Snapshot>(address, path + '/snapshot', headers)).body; }
    assert.deepEqual(snapshot.messages.map(message => message.text), [command.text, command.text]);
    await first.crash();
    assert.ok((await stat(socket)).isSocket());
    const second = await fixture.start();
    assert.equal(second.ready.kind, 'ready', second.diagnostic());
    assert.equal(object(second.ready.installation).installationId, installation.installationId);
    const duplicate = await api<Acceptance>(address, path + '/messages', headers, command);
    assert.equal(duplicate.status, 201);
    assert.deepEqual(duplicate.body.receipt, accepted.body.receipt);
    assert.deepEqual(duplicate.body.snapshot.messages.map(message => message.text), [command.text, command.text]);
    await second.close();
    await assert.rejects(stat(socket), { code: 'ENOENT' });
  });
});

test('a live owner keeps its socket and ledger when another core attempts startup', async t => {
  const fixture = await root(t), first = await fixture.start();
  assert.equal(first.ready.kind, 'ready', first.diagnostic());
  const socket = join(fixture.directory, 'core.sock'), before = await lstat(socket, { bigint: true });
  const second = await fixture.start();
  assert.equal(second.ready.kind, 'failed'); await second.close();
  assert.equal((await lstat(socket, { bigint: true })).ino, before.ino);
  assert.equal((await api(socket, '/v1/installation')).status, 401);
});

test('cancellation during stale-socket recovery preserves the recorded endpoint for a later retry', async t => {
  const fixture = await root(t), first = await fixture.start();
  assert.equal(first.ready.kind, 'ready', first.diagnostic());
  await first.crash();
  const socket = join(fixture.directory, 'core.sock'), before = await lstat(socket, { bigint: true });
  const canceled = await fixture.start('core.sock', fixture.directory, 'abort-recovery');
  assert.equal(canceled.ready.kind, 'failed'); await canceled.close();
  const after = await lstat(socket, { bigint: true });
  assert.equal(after.ino, before.ino); assert.equal(after.ctimeNs, before.ctimeNs);
  const recovered = await fixture.start();
  assert.equal(recovered.ready.kind, 'ready', recovered.diagnostic());
});

async function foreignSocket(path: string, t: TestContext) {
  const child = spawn(process.execPath, ['--input-type=module', '-e', 'import {createServer} from "node:net"; createServer().listen(process.argv[1],()=>process.stdout.write("ready")); setTimeout(()=>process.exit(3),10000);', path], { env: {}, stdio: ['ignore', 'pipe', 'pipe'] });
  const exited = once(child, 'exit');
  t.after(async () => { child.kill('SIGKILL'); await exited; });
  await once(child.stdout, 'data', { signal: AbortSignal.timeout(3000) });
  child.kill('SIGKILL'); await exited;
  await chmod(path, 0o600);
}

test('socket recovery preserves replaced files, symlinks and foreign sockets', async t => {
  for (const replacement of ['file', 'symlink', 'socket']) await t.test(replacement, async t => {
    const fixture = await root(t), first = await fixture.start();
    assert.equal(first.ready.kind, 'ready', first.diagnostic());
    await first.crash();
    const socket = join(fixture.directory, 'core.sock');
    await rm(socket);
    const marker = join(fixture.directory, 'preserved-marker');
    await writeFile(marker, 'Unrelated fixture marker');
    if (replacement === 'file') await writeFile(socket, 'Preserved file');
    else if (replacement === 'symlink') await symlink(marker, socket);
    else await foreignSocket(socket, t);
    const before = await lstat(socket, { bigint: true });
    const refused = await fixture.start();
    assert.equal(refused.ready.kind, 'failed'); await refused.close();
    const after = await lstat(socket, { bigint: true });
    assert.equal(after.ino, before.ino); assert.equal(after.ctimeNs, before.ctimeNs);
    assert.equal(await readFile(marker, 'utf8'), 'Unrelated fixture marker');
    if (replacement === 'file') assert.equal(await readFile(socket, 'utf8'), 'Preserved file');
  });
});

test('an unrecorded stale socket is preserved even in an owner-only directory', async t => {
  const fixture = await root(t), socket = join(fixture.directory, 'core.sock');
  await foreignSocket(socket, t);
  const before = await lstat(socket, { bigint: true });
  const refused = await fixture.start();
  assert.equal(refused.ready.kind, 'failed'); await refused.close();
  const after = await lstat(socket, { bigint: true });
  assert.equal(after.ino, before.ino); assert.equal(after.ctimeNs, before.ctimeNs);
});

test('malformed or future socket ownership records fail closed without exposing their contents', async t => {
  for (const data of ['synthetic-private-marker', '{"version":999,"name":"core.sock"}']) await t.test(data.startsWith('{') ? 'future version' : 'malformed record', async t => {
    const fixture = await root(t), first = await fixture.start();
    assert.equal(first.ready.kind, 'ready', first.diagnostic());
    await first.crash();
    const socket = join(fixture.directory, 'core.sock'), before = await lstat(socket, { bigint: true });
    const db = new DatabaseSync(join(fixture.directory, 'openai-candidate-b.sqlite'));
    try { db.prepare("UPDATE metadata SET data=? WHERE key='privateSocketOwnership'").run(data); }
    finally { db.close(); }
    const refused = await fixture.start();
    assert.equal(refused.ready.kind, 'failed'); await refused.close();
    assert.equal(typeof refused.ready.message, 'string');
    assert.ok(typeof refused.ready.message === 'string' && !refused.ready.message.includes(data));
    const after = await lstat(socket, { bigint: true });
    assert.equal(after.ino, before.ino); assert.equal(after.ctimeNs, before.ctimeNs);
  });
});
