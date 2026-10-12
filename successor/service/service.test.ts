import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createFixtureService } from './service.ts';

async function setup(t: any, options: Record<string, unknown> = {}) {
  const dataDirectory = await mkdtemp(join(tmpdir(), 'fort-058-test-'));
  const service = createFixtureService({ dataDirectory, ...options });
  await service.start();
  t.after(async () => { await service.close(); await rm(dataDirectory, { recursive: true, force: true }); });
  const call = async (path: string, body?: unknown, headers: Record<string, string> = {}) => {
    const response = await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST',
      headers: { 'Content-Type': 'application/json', ...headers }, body: body === undefined ? undefined : JSON.stringify(body) });
    return { status: response.status, body: await response.json() as any };
  };
  const enrollment = service.issueEnrollment();
  const enrolled = await call('/v1/devices/enroll', { installationId: service.installation.installationId, code: enrollment.code, deviceName: 'Test Mac' });
  assert.equal(enrolled.status, 201);
  const headers = { Authorization: `Bearer ${enrolled.body.deviceToken}`, 'X-Fort-Installation-Id': service.installation.installationId };
  const api = (path: string, body?: unknown) => call(path, body, headers);
  const agent = (await api('/v1/agents')).body.agents[0];
  const submit = (text: string, requestId = crypto.randomUUID()) => api(`/v1/conversations/${agent.homeConversationId}/messages`, {
    requestId, text, recoveryEpoch: service.installation.recoveryEpoch, bindingRevisionId: agent.binding.bindingRevisionId });
  return { service, dataDirectory, call, api, headers, agent, submit, enrolled: enrolled.body };
}

test('authenticated native submission preserves exact text and one Agent/Home across durable receipt replay', async t => {
  const h = await setup(t);
  const text = '  /hello\r\nCafé e\u0301 👋  ';
  const accepted = await h.submit(text, 'stable-request');
  assert.equal(accepted.status, 201);
  assert.equal(accepted.body.snapshot.messages[0].text, text);
  assert.equal(accepted.body.snapshot.runs[0].state, 'queued');
  const repeated = await h.submit(text, 'stable-request');
  assert.equal(repeated.status, 200);
  assert.deepEqual(repeated.body, accepted.body);
  assert.equal((await h.submit(text + 'changed', 'stable-request')).status, 409);
  assert.deepEqual((await h.api('/v1/requests/stable-request')).body, accepted.body);
  assert.equal((await h.api(`/v1/conversations/${h.agent.homeConversationId}/snapshot`)).body.messages.length, 1);
});

test('fixture reply and ordered replay survive service restart without repeating accepted work', async t => {
  const h = await setup(t);
  const accepted = await h.submit('exact restart text', 'restart-request');
  assert.equal(await h.service.dispatchNextFixture(), true);
  const before = (await h.api(`/v1/conversations/${h.agent.homeConversationId}/snapshot`)).body;
  assert.equal(before.messages.length, 2);
  assert.equal(before.messages[1].text, '[Fixture reply — not a real agent]\nexact restart text');
  assert.equal(before.runs[0].state, 'completed');
  await h.service.close();
  const restarted = createFixtureService({ dataDirectory: h.dataDirectory });
  t.after(() => restarted.close());
  await restarted.start();
  const get = async (path: string) => (await fetch(restarted.baseURL + path, { headers: h.headers })).json() as any;
  assert.deepEqual(await get(`/v1/conversations/${h.agent.homeConversationId}/snapshot`), before);
  assert.deepEqual(await get('/v1/requests/restart-request'), accepted.body);
  assert.equal(await restarted.dispatchNextFixture(), false);
  const replay = await get(`/v1/events?after=0&recoveryEpoch=${restarted.installation.recoveryEpoch}`);
  assert.deepEqual(replay.events.map((e: any) => e.sequence), [1, 2, 3, 4]);
  assert.deepEqual(replay.events.map((e: any) => e.kind), ['message.accepted', 'run.updated', 'message.created', 'run.updated']);
  assert.equal(replay.cursor, before.cursor);
});

test('authorization denies absent, forged, expired, revoked, wrong-installation and cross-origin credentials', async t => {
  let time = Date.now();
  const h = await setup(t, { now: () => time });
  for (const path of ['/v1/installation', '/v1/agents', `/v1/conversations/${h.agent.homeConversationId}/snapshot`, '/v1/events?after=0', '/v1/requests/absent']) {
    assert.equal((await h.call(path)).status, 401);
    assert.equal((await h.call(path, undefined, { ...h.headers, Authorization: 'Bearer forged-runner-token' })).status, 401);
    assert.equal((await h.call(path, undefined, { ...h.headers, 'X-Fort-Installation-Id': 'another-installation' })).status, 401);
    assert.equal((await h.call(path, undefined, { ...h.headers, Origin: 'https://untrusted.example' })).status, 403);
  }
  const code = h.service.issueEnrollment();
  const enrollment = { installationId: h.service.installation.installationId, code: code.code, deviceName: 'Second device' };
  const second = await h.call('/v1/devices/enroll', enrollment);
  assert.equal(second.status, 201);
  assert.equal((await h.call('/v1/devices/enroll', enrollment)).status, 401);
  assert.equal((await h.api(`/v1/devices/${second.body.deviceId}/revoke`, {})).status, 200);
  assert.equal((await h.call('/v1/agents', undefined, { ...h.headers, Authorization: `Bearer ${second.body.deviceToken}` })).status, 401);
  time += 31 * 86_400_000;
  assert.equal((await h.api('/v1/agents')).status, 401);
  const expired = h.service.issueEnrollment({ ttlMs: 1 }); time += 2;
  assert.equal((await h.call('/v1/devices/enroll', { ...enrollment, code: expired.code })).status, 401);
});

test('read position is monotonic and bound to an existing message, Conversation, epoch and observed cursor', async t => {
  const h = await setup(t);
  const first = await h.submit('first'); await h.service.dispatchNextFixture();
  const snap = (await h.api(`/v1/conversations/${h.agent.homeConversationId}/snapshot`)).body;
  const path = `/v1/conversations/${h.agent.homeConversationId}/read`;
  const command = { recoveryEpoch: h.service.installation.recoveryEpoch, messageId: snap.messages[1].messageId, cursor: snap.cursor };
  assert.equal((await h.api(path, command)).body.lastReadMessageSequence, 3);
  assert.equal((await h.api(path, { ...command, messageId: first.body.receipt.messageId })).body.lastReadMessageSequence, 3);
  assert.equal((await h.api(path, { ...command, cursor: 0 })).status, 409);
  assert.equal((await h.api(path, { ...command, cursor: 999 })).status, 409);
  assert.equal((await h.api(path, { ...command, recoveryEpoch: 'wrong-epoch' })).status, 409);
  assert.equal((await h.api(path, { ...command, messageId: 'wrong-message' })).status, 404);
});

test('acceptance failure boundaries roll back together; an acknowledgement loss recovers one immutable receipt', async t => {
  for (const point of ['afterRequest', 'afterMessage', 'afterRun', 'afterOutbox', 'afterEvent', 'beforeAcceptanceCommit', 'afterAcceptanceCommit']) {
    await t.test(point, async t => {
      let injected = false;
      const h = await setup(t, { checkpoint: (actual: string) => { if (actual === point && !injected) { injected = true; throw new Error('injected failure'); } } });
      assert.equal((await h.submit('persist this exact text', 'lost-ack')).status, 500);
      const found = await h.api('/v1/requests/lost-ack');
      const snap = (await h.api(`/v1/conversations/${h.agent.homeConversationId}/snapshot`)).body;
      const committed = point === 'afterAcceptanceCommit';
      assert.equal(found.status, committed ? 200 : 404);
      assert.equal(snap.messages.length, committed ? 1 : 0);
      assert.equal(snap.runs.length, committed ? 1 : 0);
      const retry = await h.submit('persist this exact text', 'lost-ack');
      assert.equal(retry.status, committed ? 200 : 201);
      assert.equal(await h.service.dispatchNextFixture(), true);
      assert.equal(await h.service.dispatchNextFixture(), false);
      assert.equal((await h.api(`/v1/conversations/${h.agent.homeConversationId}/snapshot`)).body.messages.length, 2);
    });
  }
});

test('concurrent native retries converge and exact UTF-8 limits reject before acceptance', async t => {
  const h = await setup(t);
  const attempts = await Promise.all(Array.from({ length: 12 }, () => h.submit('/literal\r\ne\u0301  ', 'concurrent-id')));
  assert.equal(attempts.filter(r => r.status === 201).length, 1);
  assert.equal(attempts.filter(r => r.status === 200).length, 11);
  for (const attempt of attempts) assert.deepEqual(attempt.body, attempts[0].body);
  const atLimit = '🐈'.repeat(16_384);
  assert.equal((await h.submit(atLimit, 'exact-limit')).status, 201);
  assert.equal((await h.submit(atLimit + 'a', 'over-limit')).status, 413);
  assert.equal((await h.api('/v1/requests/over-limit')).status, 404);
  assert.equal((await h.submit('\ud800', 'invalid-unicode')).status, 400);
  assert.equal((await h.api(`/v1/conversations/${h.agent.homeConversationId}/messages`, {
    requestId: 'stale-binding', text: 'text', recoveryEpoch: h.service.installation.recoveryEpoch, bindingRevisionId: 'stale-binding',
  })).status, 409);
  assert.equal((await h.api(`/v1/conversations/${h.agent.homeConversationId}/messages`, {
    requestId: 'old-epoch', text: 'text', recoveryEpoch: 'old-epoch', bindingRevisionId: h.agent.binding.bindingRevisionId,
  })).status, 409);
});

test('a second service cannot open the same authoritative ledger concurrently', async t => {
  const h = await setup(t);
  let second: ReturnType<typeof createFixtureService> | undefined;
  try { assert.throws(() => { second = createFixtureService({ dataDirectory: h.dataDirectory }); }, /active service writer/); }
  finally { await second?.close(); }
});

test('killed service fences claimed work and preserves the Conversation barrier without blind resend', async t => {
  const { fork } = await import('node:child_process');
  const { fileURLToPath } = await import('node:url');
  for (const point of ['afterClaim', 'afterStarted', 'beforeTerminalCommit', 'afterTerminalCommit']) {
    await t.test(point, async t => {
      const h = await setup(t);
      await h.submit('first turn', 'crash-first');
      await h.submit('later turn', 'crash-later');
      await h.service.close();
      const child = fork(fileURLToPath(new URL('./crash-host.ts', import.meta.url)), [h.dataDirectory, point], {
        execPath: '/opt/homebrew/opt/node@26/bin/node', env: { LANG: 'en_US.UTF-8', TZ: 'UTC' }, stdio: ['ignore', 'ignore', 'pipe', 'ipc'],
      });
      t.after(() => { child.kill('SIGKILL'); });
      await new Promise<void>((resolve, reject) => {
        child.once('error', reject); child.once('message', () => resolve());
      });
      const exited = new Promise<string | null>(resolve => child.once('exit', (_, signal) => resolve(signal)));
      child.on('message', message => { if ((message as any).unexpectedlySurvived) child.kill('SIGTERM'); });
      child.send('dispatch');
      assert.equal(await exited, 'SIGKILL', `checkpoint ${point} must actually kill the service`);
      const restarted = createFixtureService({ dataDirectory: h.dataDirectory }); t.after(() => restarted.close());
      await restarted.start();
      const response = await fetch(`${restarted.baseURL}/v1/conversations/${h.agent.homeConversationId}/snapshot`, { headers: h.headers });
      const snap = await response.json() as any;
      const completed = point === 'afterTerminalCommit';
      assert.equal(snap.runs[0].state, completed ? 'completed' : 'outcomeUnknown');
      assert.equal(snap.runs[1].state, 'queued');
      assert.equal(snap.messages.length, completed ? 3 : 2);
      assert.equal(await restarted.dispatchNextFixture(), completed);
      if (!completed) {
        const replay = await (await fetch(`${restarted.baseURL}/v1/events?after=0&recoveryEpoch=${restarted.installation.recoveryEpoch}`, { headers: h.headers })).json() as any;
        assert.equal(replay.events.at(-1).data.run.state, 'outcomeUnknown');
      }
    });
  }
});

test('fixture startup refuses unrelated data roots and ledger symlinks', async t => {
  const { writeFile, symlink } = await import('node:fs/promises');
  const root = await mkdtemp(join(tmpdir(), 'fort-fresh-root-')); t.after(() => rm(root, { recursive: true, force: true }));
  await writeFile(join(root, 'unrelated-history'), 'must stay untouched');
  let service: ReturnType<typeof createFixtureService> | undefined;
  try { assert.throws(() => { service = createFixtureService({ dataDirectory: root }); }, /fresh fixture directory/); }
  finally { await service?.close(); }
  const target = await mkdtemp(join(tmpdir(), 'fort-link-target-')); t.after(() => rm(target, { recursive: true, force: true }));
  await symlink(join(root, 'unrelated-history'), join(target, 'fixture.sqlite'));
  assert.throws(() => createFixtureService({ dataDirectory: target }), /symlink/);
});
