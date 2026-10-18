import assert from 'node:assert/strict';
import { test, type TestContext } from 'node:test';
import { fork } from 'node:child_process';
import { once } from 'node:events';
import { request } from 'node:http';
import { mkdtemp, rm, stat, writeFile, readFile, chmod } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import type { Agent, Acceptance, Snapshot } from '../contracts/v1.ts';

function object(value: unknown): Record<string, unknown> {
  assert.ok(value && typeof value === 'object' && !Array.isArray(value));
  return value as Record<string, unknown>;
}
async function root(t: TestContext) {
  const directory = await mkdtemp(join(tmpdir(), 'fort-socket-'));
  const closers: (() => Promise<void>)[] = [];
  t.after(async () => { for (const close of closers) await close(); await rm(directory, { recursive: true, force: true }); });
  return { directory, start: (name = 'core.sock', cwd = directory) => host(directory, name, cwd, close => closers.push(close)) };
}
async function host(directory: string, name: string, cwd: string, registerClose: (close: () => Promise<void>) => void) {
  const child = fork(fileURLToPath(new URL('./fixtures/socket-host.ts', import.meta.url)), [directory, name], {
    execPath: process.execPath, execArgv: [], cwd, detached: true, stdio: ['ignore', 'ignore', 'pipe', 'ipc'], env: { PATH: '/usr/bin:/bin', LANG: 'C' }
  });
  const exited = once(child, 'exit');
  let diagnostic = '', expectedExit = 0;
  child.stderr?.on('data', bytes => { diagnostic = (diagnostic + bytes.toString()).slice(-2000); });
  const close = async () => {
    if (child.connected) child.send('close', () => {});
    await Promise.race([exited, delay(2000, undefined, { ref: false })]);
    if (child.exitCode === null && child.signalCode === null && child.pid) process.kill(-child.pid, 'SIGKILL');
    const [code, signal] = await exited;
    assert.equal(signal, null, diagnostic); assert.equal(code, expectedExit, diagnostic);
  };
  registerClose(close);
  const value: unknown = (await once(child, 'message', { signal: AbortSignal.timeout(5000) }))[0];
  const ready = object(value); expectedExit = ready.kind === 'failed' ? 1 : 0;
  return { ready, close, diagnostic: () => diagnostic };
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
