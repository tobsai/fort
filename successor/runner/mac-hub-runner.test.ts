import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdir, mkdtemp, rm, stat } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import type { RuntimeDescriptor } from '../contracts/v1.ts';
import type { ChatRuntime } from './chat-runtime.ts';
import { startCloudHub } from '../host/cloud-hub.ts';
import { macHubRunner, parseRunnerInvitation, sharedRuntime, type MacHubStatus } from './mac-hub-runner.ts';

const secret = () => randomBytes(32).toString('base64url');
const node = 'toby-mac';
const codex: RuntimeDescriptor = { provider: 'codex', name: 'ChatGPT', runtimeVersion: '0.163.0', hostId: node, session: 'stateless', capabilities: ['plainText'] };
const encode = (value: object) => 'fort-runner:' + Buffer.from(JSON.stringify(value)).toString('base64url');

async function fixture(t: TestContext) {
  const awake: string[] = [];
  const root = await mkdtemp(join(tmpdir(), 'fort-mac-hub-')), state = join(root, 'state');
  await mkdir(state, { mode: 0o700 });
  const setupCode = secret();
  const hub = await startCloudHub({ dataDirectory: join(root, 'hub'), host: '127.0.0.1', port: 0, provider: codex, setupCodeDigest: createHash('sha256').update(setupCode).digest('hex') });
  const owner = secret();
  assert.equal((await fetch(hub.url + '/v1/hub/setup', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ setupCode, deviceId: randomUUID(), deviceToken: owner }) })).status, 201);
  const call = async <T>(path: string, body?: unknown) => (await fetch(hub.url + path, { method: body === undefined ? 'GET' : 'POST', body: body === undefined ? undefined : JSON.stringify(body),
    headers: { 'content-type': 'application/json', authorization: 'Bearer ' + owner, 'x-fort-installation-id': hub.installation.installationId } })).json() as Promise<T>;
  const invitation = async () => {
    const value = { invitationId: randomUUID(), code: secret() };
    await call('/v1/runners/invitation', value);
    return encode({ v: 1, hub: hub.url, ...value, runnerId: node });
  };
  const disposed: string[] = [];
  const runtime: ChatRuntime = { async execute(input) { return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() { disposed.push('local'); } };
  const runner = macHubRunner({ stateDirectory: state, runnerName: 'Toby’s Mac', lanes: runnerId => [{ binding: { provider: 'codex', runtimeVersion: '0.163.0', hostId: runnerId }, createRuntime: sharedRuntime(runtime) }],
    keepAwake: () => { awake.push('held'); return { release: () => awake.push('released') }; } });
  t.after(async () => { await runner.stop(); await hub.close(); await rm(root, { recursive: true, force: true }); });
  const until = async (state: MacHubStatus['state']) => { for (let index = 0; index < 300 && runner.status().state !== state; index++) await delay(10); assert.equal(runner.status().state, state); };
  return { hub, call, invitation, runner, state, until, disposed, awake };
}

test('pasting the hub’s invitation enrolls this Mac and connects it under the hub’s runner ID', async t => {
  const h = await fixture(t);
  assert.deepEqual(await h.runner.start(), { state: 'notEnrolled' });
  const status = await h.runner.enroll(await h.invitation());
  assert.equal(status.state === 'notEnrolled' ? undefined : status.runnerId, node);
  await h.until('connected');
  const listed = await h.call<{ runners: { runnerId: string; runnerName: string; online: boolean }[] }>('/v1/runners');
  assert.deepEqual(listed.runners.map(runner => [runner.runnerId, runner.runnerName, runner.online]), [[node, 'Toby’s Mac', true]]);
  assert.equal((await stat(join(h.state, 'hub-runner.json'))).mode & 0o777, 0o600);
});

test('a restarted helper reconnects from its saved enrollment', async t => {
  const h = await fixture(t);
  await h.runner.enroll(await h.invitation());
  await h.until('connected');
  await h.runner.stop();
  assert.equal((await h.runner.start()).state, 'connecting');
  await h.until('connected');
});

test('removing the enrollment disconnects and forgets the token', async t => {
  const h = await fixture(t);
  await h.runner.enroll(await h.invitation());
  await h.until('connected');
  assert.deepEqual(await h.runner.remove(), { state: 'notEnrolled' });
  await assert.rejects(stat(join(h.state, 'hub-runner.json')));
  for (let index = 0; index < 200 && (await h.call<{ runners: { online: boolean }[] }>('/v1/runners')).runners[0]?.online; index++) await delay(10);
  assert.equal((await h.call<{ runners: { online: boolean }[] }>('/v1/runners')).runners[0]?.online, false);
  assert.deepEqual(h.disposed, [], 'the local core’s runtime is never disposed by the hub connection');
});

test('a used, foreign or garbled invitation is refused and nothing is saved', async t => {
  const h = await fixture(t);
  const used = await h.invitation();
  await h.runner.enroll(used);
  await h.runner.remove();
  await assert.rejects(h.runner.enroll(used), /refused/);
  await assert.rejects(h.runner.enroll('fort-invite:abc'), /fort-runner/);
  await assert.rejects(h.runner.enroll(encode({ v: 1, hub: 'http://hub.example.com', invitationId: 'a', code: 'b', runnerId: node })), /HTTPS/);
  assert.throws(() => parseRunnerInvitation(encode({ v: 1, hub: 'https://hub.example.com', invitationId: 'a', code: 'b', runnerId: node, extra: true })));
  await assert.rejects(stat(join(h.state, 'hub-runner.json')));
  assert.deepEqual(h.runner.status(), { state: 'notEnrolled' });
});

test('the owning Mac manages its hub connection through the local core; others cannot', async t => {
  const { createApplication } = await import('../service/openai.ts');
  const directory = await mkdtemp(join(tmpdir(), 'fort-mac-hub-core-')), seen: string[] = [];
  const hubRunner = { status: () => ({ state: 'notEnrolled' }), async enroll(invitation: string) { seen.push(invitation); return { state: 'connecting', hubURL: 'https://hub.example.com', runnerId: node }; },
    async remove() { seen.push('remove'); return { state: 'notEnrolled' }; } };
  const service = createApplication({ dataDirectory: directory, provider: { ...codex, hostId: 'local-codex' }, ownerPairingPolicy: { expiresAfterDays: null }, hubRunner });
  await service.start();
  t.after(async () => { await service.close(); await rm(directory, { recursive: true, force: true }); });
  const offer = service.beginOwnerSetup();
  assert.equal(offer.kind, 'available');
  const token = secret();
  service.completeOwnerSetup({ installationId: offer.installationId, setupId: offer.setupId, deviceId: randomUUID(), deviceToken: token });
  const api = (path: string, body?: unknown, credential = token) => fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST',
    headers: { authorization: 'Bearer ' + credential, 'x-fort-installation-id': service.installation.installationId, 'content-type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
  assert.deepEqual(await (await api('/v1/hub/runner')).json(), { state: 'notEnrolled' });
  assert.equal((await api('/v1/hub/runner/enroll', { invitation: 'fort-runner:abc' })).status, 200);
  assert.equal((await api('/v1/hub/runner/enroll', { invitation: 'fort-runner:abc', extra: 1 })).status, 400);
  assert.equal((await api('/v1/hub/runner/remove', {})).status, 200);
  assert.equal((await api('/v1/hub/runner', undefined, secret())).status, 401);
  assert.deepEqual(seen, ['fort-runner:abc', 'remove']);
});

test('the Mac holds itself awake only while its runner is up', async t => {
  const h = await fixture(t);
  await h.runner.start();
  assert.deepEqual(h.awake, [], 'an unenrolled Mac never holds the assertion');
  await h.runner.enroll(await h.invitation());
  await h.until('connected');
  await h.runner.start();
  assert.deepEqual(h.awake, ['held'], 'starting again keeps the one assertion');
  await h.runner.stop();
  assert.deepEqual(h.awake, ['held', 'released']);
  await h.runner.start(); await h.until('connected');
  await h.runner.remove();
  assert.deepEqual(h.awake, ['held', 'released', 'held', 'released']);
});
