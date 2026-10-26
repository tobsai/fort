import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, stat, readFile, readdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import type { Acceptance, AccountSignInStatus, Agent, RuntimeDescriptor } from '../contracts/v1.ts';
import { startCloudHub } from '../host/cloud-hub.ts';
import { deleteHubCredential, enrollWithHub, readHubCredential, runAsRunner } from './hub-credential.ts';
import type { RunnerStatus } from './hub-runner.ts';

const secret = () => randomBytes(32).toString('base64url');
const provider: RuntimeDescriptor = { provider: 'remote-fixture', name: 'Remote fixture', runtimeVersion: 'fixture-1', hostId: 'mac-mini', session: 'stateless', capabilities: ['plainText'] };
const binding = { provider: provider.provider, runtimeVersion: provider.runtimeVersion, hostId: 'mac-mini' };

async function fixture(t: TestContext) {
  const root = await mkdtemp(join(tmpdir(), 'fort-hub-credential-')), state = join(root, 'state');
  const { mkdir } = await import('node:fs/promises'); await mkdir(state, { mode: 0o700 });
  const setupCode = secret();
  const hub = await startCloudHub({ dataDirectory: join(root, 'hub'), host: '127.0.0.1', port: 0, provider, setupCodeDigest: createHash('sha256').update(setupCode).digest('hex') });
  t.after(async () => { await hub.close(); await rm(root, { recursive: true, force: true }); });
  const owner = secret();
  const setup = await fetch(hub.url + '/v1/hub/setup', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ setupCode, deviceId: randomUUID(), deviceToken: owner }) });
  assert.equal(setup.status, 201);
  const call = async <T = Record<string, unknown>>(path: string, body?: unknown) => {
    const response = await fetch(hub.url + path, { method: body === undefined ? 'GET' : 'POST', body: body === undefined ? undefined : JSON.stringify(body),
      headers: { 'content-type': 'application/json', authorization: 'Bearer ' + owner, 'x-fort-installation-id': hub.installation.installationId } });
    return { status: response.status, body: await response.json() as T };
  };
  const invite = async () => { const invitation = { invitationId: randomUUID(), code: secret() }; assert.equal((await call('/v1/runners/invitation', invitation)).status, 201); return invitation; };
  return { hub, state, call, invite };
}

test('a Mac enrolls, keeps its token in an owner-only file, and takes turns from the hub', async t => {
  const h = await fixture(t), invitation = await h.invite(), seen: string[] = [];
  assert.deepEqual(await enrollWithHub({ stateDirectory: h.state, hubURL: h.hub.url, ...invitation, runnerId: 'mac-mini', runnerName: 'Mac mini' }), { runnerId: 'mac-mini' });
  const file = join(h.state, 'hub-runner.json');
  assert.equal((await stat(file)).mode & 0o777, 0o600);
  const saved = await readHubCredential(h.state);
  assert.equal(saved?.runnerId, 'mac-mini'); assert.equal(saved?.hubURL, h.hub.url);
  assert.ok(!(await readFile(file, 'utf8')).includes(invitation.code));
  assert.ok(!JSON.stringify((await h.call('/v1/runners')).body).includes(saved!.token));

  const stop = new AbortController(), statuses: RunnerStatus['state'][] = [];
  const running = runAsRunner({ stateDirectory: h.state, binding, signal: stop.signal, onStatus: status => statuses.push(status.state),
    createRuntime: () => ({ async execute(input) { seen.push(input.text); return { kind: 'completed', text: input.text, effectiveModel: null }; },
      async signIn() { return { kind: 'signedIn', account: { accountId: 'account', userLabel: 'owner@example.com' } }; }, async dispose() {} }) });
  t.after(async () => { stop.abort(); await running.catch(() => {}); });
  for (let index = 0; index < 200 && !statuses.includes('connected'); index++) await delay(10);
  // The hub pins turns to the account the Mac reports, once the owner confirms it.
  assert.equal((await h.call('/v1/account/sign-in/start', { operationId: randomUUID(), expectedRevision: 0, recoveryEpoch: h.hub.installation.recoveryEpoch, expiresAt: Date.now() + 60_000 })).status, 200);
  for (let index = 0; index < 200 && (await h.call<AccountSignInStatus>('/v1/account/sign-in')).body.state !== 'finished'; index++) await delay(10);
  const agent = (await h.call<Agent>('/v1/agents', { name: 'Remote', model: 'exact-model', instructions: 'Echo.' })).body;
  const requestId = randomUUID();
  await h.call(`/v1/conversations/${agent.homeConversationId}/messages`, { requestId, text: 'from the cloud', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.hub.installation.recoveryEpoch, skills: [] });
  let state = '';
  for (let index = 0; index < 300 && state !== 'completed'; index++) { state = (await h.call<Acceptance>('/v1/requests/' + requestId)).body.snapshot.runs[0]?.state ?? ''; await delay(10); }
  assert.equal(state, 'completed');
  assert.deepEqual(seen, ['from the cloud']);
});

test('a revoked Mac stops connecting and keeps its file until the owner removes it', async t => {
  const h = await fixture(t);
  await enrollWithHub({ stateDirectory: h.state, hubURL: h.hub.url, ...await h.invite(), runnerId: 'mac-mini', runnerName: 'Mac mini' });
  assert.equal((await h.call('/v1/runners/revoke', { runnerId: 'mac-mini' })).status, 200);
  const status = await runAsRunner({ stateDirectory: h.state, binding, signal: new AbortController().signal, createRuntime: () => { throw new Error('No turn'); } });
  assert.equal(status.state, 'refused');
  assert.equal(await deleteHubCredential(h.state), 'deleted');
  assert.equal(await readHubCredential(h.state), undefined);
});

test('enrollment refuses a second hub, plain HTTP off this machine, and leaves nothing on failure', async t => {
  const h = await fixture(t);
  await assert.rejects(enrollWithHub({ stateDirectory: h.state, hubURL: 'http://hub.example.com', ...await h.invite(), runnerId: 'mac-mini', runnerName: 'Mac mini' }), /HTTPS/);
  await assert.rejects(enrollWithHub({ stateDirectory: h.state, hubURL: h.hub.url, invitationId: randomUUID(), code: secret(), runnerId: 'mac-mini', runnerName: 'Mac mini' }), /refused/);
  assert.deepEqual(await readdir(h.state), []);
  await enrollWithHub({ stateDirectory: h.state, hubURL: h.hub.url, ...await h.invite(), runnerId: 'mac-mini', runnerName: 'Mac mini' });
  await assert.rejects(enrollWithHub({ stateDirectory: h.state, hubURL: h.hub.url, ...await h.invite(), runnerId: 'mac-mini', runnerName: 'Mac mini' }), /already enrolled/);
});

test('a Mac that cannot reach the hub says why, not just "fetch failed"', async t => {
  const h = await fixture(t);
  const failing = (code: string) => async () => { throw new TypeError('fetch failed', { cause: Object.assign(new Error('unable to get local issuer certificate'), { code }) }); };
  await assert.rejects(enrollWithHub({ stateDirectory: h.state, hubURL: 'https://fort.example.com', ...await h.invite(), runnerId: 'mac-mini', runnerName: 'Mac mini', fetch: failing('UNABLE_TO_GET_ISSUER_CERT_LOCALLY') }),
    /couldn’t check the hub’s certificate.*UNABLE_TO_GET_ISSUER_CERT_LOCALLY/);
  await assert.rejects(enrollWithHub({ stateDirectory: h.state, hubURL: 'https://fort.example.com', ...await h.invite(), runnerId: 'mac-mini', runnerName: 'Mac mini', fetch: failing('ENOTFOUND') }),
    /couldn’t reach fort\.example\.com.*ENOTFOUND/);
  assert.deepEqual(await readdir(h.state), []);
});

test('runner mode refuses a binding for a different Mac than the one enrolled', async t => {
  const h = await fixture(t);
  await enrollWithHub({ stateDirectory: h.state, hubURL: h.hub.url, ...await h.invite(), runnerId: 'mac-mini', runnerName: 'Mac mini' });
  await assert.rejects(runAsRunner({ stateDirectory: h.state, binding: { ...binding, hostId: 'mac-studio' }, signal: new AbortController().signal, createRuntime: () => { throw new Error('No turn'); } }), /different Mac/);
  await assert.rejects(runAsRunner({ stateDirectory: join(h.state, '..'), binding, signal: new AbortController().signal, createRuntime: () => { throw new Error('No turn'); } }));
});
