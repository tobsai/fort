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
  const submit = (text: string, requestId: string = crypto.randomUUID()) => api(`/v1/conversations/${agent.homeConversationId}/messages`, {
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

test('two owner devices race a bound fixture approval; exactly one decision is durable and controls the reply', async t => {
  const h = await setup(t, { fixtureBehavior: 'approval' });
  const accepted = await h.submit('approval text', 'approval-request');
  await h.service.dispatchNextFixture();
  const path = `/v1/conversations/${h.agent.homeConversationId}/snapshot`;
  const parked = (await h.api(path)).body;
  assert.equal(parked.runs[0].state, 'waitingForApproval');
  assert.equal(parked.messages.length, 1);
  assert.equal(parked.approvals.length, 1);
  const approval = parked.approvals[0];
  const command = { recoveryEpoch: approval.recoveryEpoch, runId: approval.runId, attemptId: approval.attemptId,
    bindingRevisionId: approval.bindingRevisionId, actionDigest: approval.actionDigest, resourceScope: approval.resourceScope, decision: 'approve' };
  const resolve = `/v1/approvals/${approval.approvalId}/resolve`;
  assert.equal((await h.call(resolve, command)).status, 401);
  assert.equal((await h.api(resolve, { ...command, actionDigest: 'forged' })).status, 409);
  assert.equal((await h.api(resolve, { ...command, attemptId: 'other-attempt' })).status, 409);
  assert.equal((await h.api(resolve, { ...command, resourceScope: ['filesystem:all'] })).status, 409);
  const code = h.service.issueEnrollment();
  const enrolled = await h.call('/v1/devices/enroll', { installationId: h.service.installation.installationId, code: code.code, deviceName: 'Second owner device' });
  const responses = await Promise.all([h.api(resolve, command), h.call(resolve, { ...command, decision: 'reject' }, { ...h.headers, Authorization: `Bearer ${enrolled.body.deviceToken}` })]);
  for (const response of responses) assert.equal(response.status, 200);
  assert.equal(responses[0].body.approval.state, responses[1].body.approval.state);
  assert.equal(responses[0].body.approval.decidedByDeviceId, responses[1].body.approval.decidedByDeviceId);
  const won = responses[0].body.approval.state;
  await h.service.dispatchNextFixture();
  const final = (await h.api(path)).body;
  assert.equal(final.runs[0].state, won === 'approved' ? 'completed' : 'canceled');
  assert.equal(final.messages.length, won === 'approved' ? 2 : 1);
  assert.equal(final.approvals[0].runId, accepted.body.receipt.runId);
});

test('queued cancellation never dispatches; active cancellation remains requested until fixture termination is observed', async t => {
  await t.test('queued', async t => {
    const h = await setup(t); const accepted = await h.submit('never execute');
    const cancel = `/v1/runs/${accepted.body.receipt.runId}/cancel`;
    const command = { recoveryEpoch: h.service.installation.recoveryEpoch };
    assert.equal((await h.call(cancel, command)).status, 401);
    assert.equal((await h.api(cancel, { recoveryEpoch: 'wrong-epoch' })).status, 409);
    assert.equal((await h.api(cancel, command)).body.run.state, 'canceled');
    assert.equal((await h.api(cancel, command)).body.run.state, 'canceled');
    assert.equal(await h.service.dispatchNextFixture(), false);
    const snapshot = (await h.api(`/v1/conversations/${h.agent.homeConversationId}/snapshot`)).body;
    assert.equal(snapshot.messages.length, 1); assert.equal(snapshot.runs[0].attemptId, null);
  });
  await t.test('active fixture', async t => {
    let signalStarted: () => void = () => {};
    const started = new Promise<void>(resolve => { signalStarted = resolve; });
    const h = await setup(t, { fixtureBehavior: 'hold', checkpoint: (point: string) => { if (point === 'afterStarted') signalStarted(); } });
    const accepted = await h.submit('stop active fixture');
    const work = h.service.dispatchNextFixture(); await started;
    const response = await h.api(`/v1/runs/${accepted.body.receipt.runId}/cancel`, { recoveryEpoch: h.service.installation.recoveryEpoch });
    assert.equal(response.status, 200); assert.equal(response.body.run.state, 'cancelRequested');
    await work;
    const snapshot = (await h.api(`/v1/conversations/${h.agent.homeConversationId}/snapshot`)).body;
    assert.equal(snapshot.runs[0].state, 'canceled'); assert.equal(snapshot.messages.length, 1);
  });
});

test('a parked fixture approval survives restart, blocks later work, and resumes only its exact approved step', async t => {
  const h = await setup(t, { fixtureBehavior: 'approval' });
  await h.submit('first gated turn', 'gated-first'); await h.service.dispatchNextFixture();
  await h.submit('later queued turn', 'gated-later');
  const path = `/v1/conversations/${h.agent.homeConversationId}/snapshot`;
  const before = (await h.api(path)).body;
  await h.service.close();
  const restarted = createFixtureService({ dataDirectory: h.dataDirectory }); t.after(() => restarted.close()); await restarted.start();
  const api = async (path: string, body?: unknown) => {
    const response = await fetch(restarted.baseURL + path, { method: body ? 'POST' : 'GET', headers: { ...h.headers, 'Content-Type': 'application/json' }, body: body ? JSON.stringify(body) : undefined });
    return { status: response.status, body: await response.json() as any };
  };
  assert.deepEqual((await api(path)).body, before);
  assert.equal(await restarted.dispatchNextFixture(), false);
  const approval = before.approvals[0];
  const decision = { recoveryEpoch: approval.recoveryEpoch, runId: approval.runId, attemptId: approval.attemptId,
    bindingRevisionId: approval.bindingRevisionId, actionDigest: approval.actionDigest, resourceScope: approval.resourceScope, decision: 'approve' };
  assert.equal((await api(`/v1/approvals/${approval.approvalId}/resolve`, decision)).body.approval.state, 'approved');
  assert.equal(await restarted.dispatchNextFixture(), true);
  const after = (await api(path)).body;
  assert.equal(after.runs[0].state, 'completed');
  assert.equal(after.runs[0].attemptId, approval.attemptId);
  assert.equal(after.runs[1].state, 'queued');
  assert.equal(after.messages.filter((m: any) => m.role === 'assistant').length, 1);
  assert.equal(after.messages.at(-1).text, '[Fixture reply — not a real agent]\nfirst gated turn');
  assert.equal(await restarted.dispatchNextFixture(), true);
  assert.equal((await api(path)).body.runs[1].state, 'completed');
});

test('expired and canceled fixture approvals cannot authorize a reply', async t => {
  for (const action of ['expire', 'cancel', 'reject']) await t.test(action, async t => {
    let clock = Date.now();
    const h = await setup(t, { fixtureBehavior: 'approval', now: () => clock });
    const accepted = await h.submit('do not produce reply'); await h.service.dispatchNextFixture();
    const approval = (await h.api(`/v1/conversations/${h.agent.homeConversationId}/snapshot`)).body.approvals[0];
    if (action === 'expire') clock += 300_001;
    if (action === 'cancel') assert.equal((await h.api(`/v1/runs/${accepted.body.receipt.runId}/cancel`, { recoveryEpoch: h.service.installation.recoveryEpoch })).body.run.state, 'canceled');
    const result = await h.api(`/v1/approvals/${approval.approvalId}/resolve`, {
      recoveryEpoch: approval.recoveryEpoch, runId: approval.runId, attemptId: approval.attemptId,
      bindingRevisionId: approval.bindingRevisionId, actionDigest: approval.actionDigest, resourceScope: approval.resourceScope, decision: action === 'reject' ? 'reject' : 'approve',
    });
    assert.equal(result.status, action === 'expire' ? 409 : 200);
    assert.equal(await h.service.dispatchNextFixture(), false);
    const after = (await h.api(`/v1/conversations/${h.agent.homeConversationId}/snapshot`)).body;
    assert.equal(after.runs[0].state, 'canceled'); assert.equal(after.messages.length, 1);
    assert.equal(after.approvals[0].state, action === 'expire' ? 'expired' : action === 'reject' ? 'rejected' : 'canceled');
  });
});

test('owner can close an uncertain outcome without retrying it or claiming cancellation', async t => {
  let injected = false;
  const h = await setup(t, { checkpoint: (point: string) => { if (point === 'afterStarted' && !injected) { injected = true; throw new Error('lost fixture contact'); } } });
  const first = await h.submit('uncertain first', 'unknown-first');
  await h.submit('later work', 'unknown-later'); await h.service.dispatchNextFixture();
  const path = `/v1/conversations/${h.agent.homeConversationId}/snapshot`;
  assert.equal((await h.api(path)).body.runs[0].state, 'outcomeUnknown');
  assert.equal(await h.service.dispatchNextFixture(), false);
  const runPath = `/v1/runs/${first.body.receipt.runId}`;
  const epoch = h.service.installation.recoveryEpoch;
  assert.equal((await h.api(runPath + '/cancel', { recoveryEpoch: epoch })).status, 409);
  const close = { recoveryEpoch: epoch, acknowledgeUncertainty: true };
  assert.equal((await h.call(runPath + '/close-unknown', close)).status, 401);
  assert.equal((await h.api(runPath + '/close-unknown', { ...close, acknowledgeUncertainty: false })).status, 400);
  assert.equal((await h.api(runPath + '/close-unknown', close)).body.run.state, 'closedUnknown');
  assert.equal((await h.api(runPath + '/close-unknown', close)).body.run.state, 'closedUnknown');
  assert.equal(await h.service.dispatchNextFixture(), true);
  assert.equal(await h.service.dispatchNextFixture(), false);
  const final = (await h.api(path)).body;
  assert.equal(final.runs[0].state, 'closedUnknown'); assert.equal(final.runs[1].state, 'completed');
  assert.equal(final.messages.filter((m: any) => m.role === 'assistant' && m.requestId === 'unknown-first').length, 0);
  assert.deepEqual((await h.api('/v1/requests/unknown-first')).body, first.body);
});

test('revocation and expiry during upload deny every owner mutation before commit', async t => {
  const { request } = await import('node:http');
  for (const mode of ['revoke', 'expire']) for (const mutation of ['submit', 'read', 'approval', 'cancel', 'closeUnknown', 'revoke']) {
    await t.test(`${mode}: ${mutation}`, async t => {
      let clock = Date.now();
      const h = await setup(t, { now: () => clock });
      const accepted = await h.submit('existing message', 'existing-request');
      const epoch = h.service.installation.recoveryEpoch;
      const cases: Record<string, { path: string; payload: object }> = {
        submit: { path: `/v1/conversations/${h.agent.homeConversationId}/messages`, payload: { requestId: 'delayed-request', text: 'must not accept', bindingRevisionId: h.agent.binding.bindingRevisionId, recoveryEpoch: epoch } },
        read: { path: `/v1/conversations/${h.agent.homeConversationId}/read`, payload: { messageId: accepted.body.receipt.messageId, cursor: accepted.body.snapshot.cursor, recoveryEpoch: epoch } },
        approval: { path: '/v1/approvals/missing-approval/resolve', payload: {} },
        cancel: { path: `/v1/runs/${accepted.body.receipt.runId}/cancel`, payload: { recoveryEpoch: epoch } },
        closeUnknown: { path: `/v1/runs/${accepted.body.receipt.runId}/close-unknown`, payload: { recoveryEpoch: epoch, acknowledgeUncertainty: true } },
        revoke: { path: `/v1/devices/${h.enrolled.deviceId}/revoke`, payload: {} },
      };
      const secondCode = h.service.issueEnrollment();
      const second = await h.call('/v1/devices/enroll', { ...secondCode, deviceName: 'Revoking device' });
      const secondHeaders = { ...h.headers, Authorization: `Bearer ${second.body.deviceToken}` };
      const target = cases[mutation]; const payload = JSON.stringify(target.payload);
      const arrived = new Promise<void>(resolve => h.service.server.once('request', () => resolve()));
      let finish: ReturnType<typeof request>;
      const result = new Promise<number | undefined>((resolve, reject) => {
        finish = request(h.service.baseURL + target.path, { method: 'POST', headers: { ...h.headers, 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(payload) } }, response => {
          response.resume(); response.on('end', () => resolve(response.statusCode));
        });
        finish.on('error', reject); finish.write(payload.slice(0, 1));
      });
      await arrived;
      if (mode === 'revoke') assert.equal((await h.call(`/v1/devices/${h.enrolled.deviceId}/revoke`, {}, secondHeaders)).status, 200);
      else clock += 31 * 86_400_000;
      finish!.end(payload.slice(1));
      assert.equal(await result, 401);
      const enrollment = h.service.issueEnrollment();
      const device = await h.call('/v1/devices/enroll', { ...enrollment, deviceName: 'Inspection device' });
      const headers = { ...h.headers, Authorization: `Bearer ${device.body.deviceToken}` };
      const snapshot = await h.call(`/v1/conversations/${h.agent.homeConversationId}/snapshot`, undefined, headers);
      assert.equal(snapshot.body.messages.length, 1);
      assert.equal(snapshot.body.runs[0].state, 'queued');
      assert.equal(snapshot.body.lastReadMessageSequence, 0);
    });
  }
});

test('approval expiry before dispatch releases only that fixture step and continues later queued work', async t => {
  let clock = Date.now();
  const h = await setup(t, { fixtureBehavior: 'approval', now: () => clock });
  await h.submit('expired approved step', 'expired-approved'); await h.service.dispatchNextFixture();
  await h.submit('later gated step', 'later-gate');
  const path = `/v1/conversations/${h.agent.homeConversationId}/snapshot`;
  const approval = (await h.api(path)).body.approvals[0];
  assert.equal((await h.api(`/v1/approvals/${approval.approvalId}/resolve`, {
    recoveryEpoch: approval.recoveryEpoch, runId: approval.runId, attemptId: approval.attemptId,
    bindingRevisionId: approval.bindingRevisionId, actionDigest: approval.actionDigest, resourceScope: approval.resourceScope, decision: 'approve',
  })).status, 200);
  clock += 300_001;
  assert.equal(await h.service.dispatchNextFixture(), true);
  const snapshot = (await h.api(path)).body;
  assert.equal(snapshot.runs[0].state, 'canceled');
  assert.equal(snapshot.runs[1].state, 'waitingForApproval');
  assert.equal(snapshot.messages.length, 2);
  assert.equal(snapshot.approvals[0].state, 'expired');
});
