import assert from 'node:assert/strict';
import { fork, spawnSync } from 'node:child_process';
import { mkdtemp, rm, stat } from 'node:fs/promises';
import { join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import { test, type TestContext } from 'node:test';
import { randomBytes, randomUUID } from 'node:crypto';

function object(value: unknown): Record<string, unknown> {
  assert.ok(value && typeof value === 'object' && !Array.isArray(value)); return value as Record<string, unknown>;
}
function string(value: unknown): string { assert.equal(typeof value, 'string'); return value as string; }
function array(value: unknown): unknown[] { assert.ok(Array.isArray(value)); return value; }

async function fixture(t: TestContext, artifact?: string, owner = false, recoveryBehavior = '') {
  const directory = await mkdtemp('/tmp/fort-private-channel-');
  const closers: (() => Promise<void>)[] = [];
  t.after(async () => { for (const close of closers) await close(); await rm(directory, { recursive: true, force: true }); });
  return { directory, start: async (issueEnrollment = true) => {
    const coreModule = artifact ? pathToFileURL(join(artifact, 'service/openai.js')).href : new URL('../service/openai.ts', import.meta.url).href;
    const channelModule = artifact ? pathToFileURL(join(artifact, 'host/private-channel.js')).href : new URL('./private-channel.ts', import.meta.url).href;
    const child = fork(fileURLToPath(new URL('./fixtures/private-core.mjs', import.meta.url)), [directory, coreModule, channelModule, owner ? 'owner' : 'ordinary', recoveryBehavior, issueEnrollment ? 'enroll' : 'no-enrollment'], {
      execPath: process.execPath, execArgv: artifact ? ['--jitless', '--no-experimental-strip-types'] : [], cwd: directory, detached: true,
      stdio: ['pipe', 'pipe', 'pipe', 'ipc'], env: { PATH: '/usr/bin:/bin', LANG: 'C' }
    });
    const setup = Promise.withResolvers<Record<string, unknown>>(), ready = Promise.withResolvers<void>();
    const exited = Promise.withResolvers<{ code: number | null; signal: NodeJS.Signals | null }>();
    const responses = new Map<number, ReturnType<typeof Promise.withResolvers<Record<string, unknown>>>>();
    let pending = '', diagnostics = '', nextID = 0, executions = 0, cancellations = 0, expectedExit = 0;
    child.stdin?.on('error', () => {});
    child.stderr?.on('data', chunk => { diagnostics = (diagnostics + chunk.toString()).slice(-2000); });
    child.on('message', value => { const message = object(value); if (message.kind === 'setup') setup.resolve(message); if (message.kind === 'executed') executions++; if (message.kind === 'cancelInvoked') cancellations++; });
    child.once('error', error => { setup.reject(error); ready.reject(error); });
    child.once('exit', (code, signal) => exited.resolve({ code, signal }));
    child.stdout?.setEncoding('utf8');
    child.stdout?.on('data', chunk => {
      pending += chunk;
      for (let newline; (newline = pending.indexOf('\n')) >= 0;) {
        const frame = object(JSON.parse(pending.slice(0, newline))); pending = pending.slice(newline + 1);
        if (frame.kind === 'ready') ready.resolve();
        else if (typeof frame.id === 'number') responses.get(frame.id)?.resolve(frame);
      }
    });
    const close = async () => {
      child.stdin?.end();
      await Promise.race([exited.promise, delay(2000, undefined, { ref: false })]);
      if (child.exitCode === null && child.signalCode === null && child.pid) process.kill(-child.pid, 'SIGKILL');
      const result = await exited.promise;
      child.stdin?.destroy(); child.stdout?.destroy(); child.stderr?.destroy();
      assert.equal(result.signal, null, diagnostics); assert.equal(result.code, expectedExit, diagnostics);
      assert.ok(child.pid); assert.throws(() => process.kill(-child.pid!, 0), { code: 'ESRCH' });
    };
    closers.push(close);
    await Promise.race([Promise.all([setup.promise, ready.promise]), exited.promise.then(() => { throw new Error('Fixture exited before ready: ' + diagnostics); }), delay(5000, undefined, { ref: false }).then(() => { throw new Error('Fixture startup timed out'); })]);
    const installed = object((await setup.promise).installation);
    const send = (path: string, body?: unknown, token?: string, timeout = 3, installationID = string(installed.installationId)) => {
      const id = ++nextID, response = Promise.withResolvers<Record<string, unknown>>(); responses.set(id, response);
      child.stdin?.write(JSON.stringify({ protocolVersion: 1, kind: 'request', id, request: { path, method: body === undefined ? 'GET' : 'POST', installationID, token, body: body === undefined ? undefined : Buffer.from(JSON.stringify(body)).toString('base64'), timeout } }) + '\n');
      return { id, result: Promise.race([response.promise, delay(5000, undefined, { ref: false }).then(() => { throw new Error('Missing channel response'); })]).finally(() => responses.delete(id)) };
    };
    const api = async (path: string, body?: unknown, token?: string, timeout = 3, installationID?: string) => {
      const frame = await send(path, body, token, timeout, installationID).result;
      assert.equal(frame.kind, 'response'); const response = object(frame.response);
      return { status: response.status, body: object(JSON.parse(Buffer.from(string(response.body), 'base64').toString())) };
    };
    const recovery = (installationID: string, deviceID: string, timeout = 3) => {
      const id = ++nextID, response = Promise.withResolvers<Record<string, unknown>>(); responses.set(id, response);
      child.stdin?.write(JSON.stringify({ protocolVersion: 1, kind: 'ownerRecovery', id, recovery: { installationID, deviceID, timeout } }) + '\n');
      return { id, result: Promise.race([response.promise, delay(5000, undefined, { ref: false }).then(() => { throw new Error('Missing recovery response'); })]).finally(() => responses.delete(id)) };
    };
    const ownerSetup = (setup: unknown) => {
      const id = ++nextID, response = Promise.withResolvers<Record<string, unknown>>(); responses.set(id, response);
      child.stdin?.write(JSON.stringify({ protocolVersion: 1, kind: 'ownerSetup', id, setup }) + '\n');
      return { id, result: Promise.race([response.promise, exited.promise.then(() => { throw new Error('Setup channel exited'); }), delay(5000, undefined, { ref: false }).then(() => { throw new Error('Missing setup response'); })]).finally(() => responses.delete(id)) };
    };
    return { installed, setup: await setup.promise, api, send, recovery, ownerSetup,
      recover: (installationID: string, deviceID: string, timeout = 3) => recovery(installationID, deviceID, timeout).result,
      executions: () => executions, cancellations: () => cancellations, close,
      cancel(id: number) { child.stdin?.write(JSON.stringify({ protocolVersion: 1, kind: 'cancel', id }) + '\n'); },
      raw(bytes: string) { expectedExit = 1; child.stdin?.write(bytes); },
      async failed() { const result = await Promise.race([exited.promise, delay(10_000, undefined, { ref: false }).then(() => { throw new Error('Failed fixture did not exit within its bound'); })]); assert.equal(result.code, 1); },
      expectFailure() { expectedExit = 1; },
      pauseOutput() { child.stdout?.pause(); },
      end() { child.stdin?.end(); }
    };
  } };
}

async function journey(t: TestContext, artifact?: string) {
  const local = await fixture(t, artifact), first = await local.start();
  assert.equal((await first.api('/v1/installation')).status, 401);
  const enrolled = await first.api('/v1/devices/enroll', { ...object(first.setup.enrollment), deviceName: 'Synthetic pipe peer' });
  assert.equal(enrolled.status, 201); const token = string(enrolled.body.deviceToken);
  assert.equal((await first.api('/v1/installation', undefined, token, 3, 'foreign')).status, 401);
  const agent = (await first.api('/v1/agents', { name: 'Pipe Home', model: 'exact-synthetic', instructions: 'Echo.' }, token)).body;
  const path = '/v1/conversations/' + string(agent.homeConversationId);
  const command = { requestId: 'pipe-turn', text: '  Exact cedar 🌲\r\n', recipientAgentId: agent.agentId, bindingRevisionId: object(agent.binding).bindingRevisionId, recoveryEpoch: first.installed.recoveryEpoch, skills: [] };
  const accepted = await first.api(path + '/messages', command, token); assert.equal(accepted.status, 201);
  let snapshot = (await first.api(path + '/snapshot', undefined, token)).body;
  for (let count = 0; count < 50 && object(array(snapshot.runs)[0]).state !== 'completed'; count++) { await delay(10); snapshot = (await first.api(path + '/snapshot', undefined, token)).body; }
  assert.deepEqual(array(snapshot.messages).map(value => object(value).text), [command.text, command.text]);
  const events = (await first.api('/v1/events?after=0&recoveryEpoch=' + first.installed.recoveryEpoch, undefined, token)).body;
  assert.equal(events.semantics, 'invalidation'); assert.equal(first.executions(), 1);
  const wait = first.send('/v1/events?after=' + events.cursor + '&recoveryEpoch=' + first.installed.recoveryEpoch + '&wait=25000', undefined, token, 30);
  first.cancel(wait.id); assert.equal((await wait.result).error, 'canceled');
  const timed = first.send('/v1/events?after=' + events.cursor + '&recoveryEpoch=' + first.installed.recoveryEpoch + '&wait=25000', undefined, token, 0.05);
  assert.equal((await timed.result).error, 'timeout');
  first.cancel(timed.id); assert.equal((await first.api('/v1/installation', undefined, token)).status, 200);
  await first.close(); await assert.rejects(stat(join(local.directory, 'core.sock')), { code: 'ENOENT' });
  const second = await local.start();
  assert.deepEqual(second.installed, first.installed);
  const duplicate = await second.api(path + '/messages', command, token);
  assert.deepEqual(duplicate.body.receipt, accepted.body.receipt);
  assert.equal(array(object(duplicate.body.snapshot).messages).length, 2);
  assert.equal(second.executions(), 0);
}

test('private pipe requests preserve core authorization, exact messages, canceled waits and restart receipts', t => journey(t));

function responseBody(frame: Record<string, unknown>) {
  assert.equal(frame.kind, 'response');
  const response = object(frame.response); assert.equal(response.status, 200);
  return object(JSON.parse(Buffer.from(string(response.body), 'base64').toString()));
}

async function ownerSetupJourney(t: TestContext, artifact?: string) {
  const local = await fixture(t, artifact, true), host = await local.start();
  const offer = responseBody(await host.ownerSetup({ operation: 'begin', timeout: 3 }).result);
  assert.equal(offer.kind, 'available');
  const setup = { operation: 'complete', timeout: 3, installationId: offer.installationId, setupId: offer.setupId, deviceId: randomUUID(), deviceToken: randomBytes(32).toString('base64url') };
  const receipt = responseBody(await host.ownerSetup(setup).result);
  assert.deepEqual(receipt, { installationId: offer.installationId, deviceId: setup.deviceId, expiresAt: null });
  assert.deepEqual(responseBody(await host.ownerSetup(setup).result), receipt);
  assert.equal((await host.api('/v1/pairing', undefined, setup.deviceToken)).status, 200);
  assert.equal((await host.api('/v1/owner/setup', {}, setup.deviceToken)).status, 404);
  assert.deepEqual(responseBody(await host.ownerSetup({ operation: 'begin', timeout: 3 }).result), { kind: 'paired', ...receipt });
  await host.close();
  const reopened = await local.start();
  assert.deepEqual(responseBody(await reopened.ownerSetup(setup).result), receipt);
  assert.equal((await reopened.api('/v1/pairing', undefined, setup.deviceToken)).status, 200);
  assert.equal(reopened.executions(), 0);
}

test('explicit owner setup on the private pipe confirms the prepared credential across restart', t => ownerSetupJourney(t));

async function ownerCancellationJourney(t: TestContext, artifact?: string) {
  const local = await fixture(t, artifact, true), host = await local.start();
  const offer = responseBody(await host.ownerSetup({ operation: 'begin', timeout: 3 }).result);
  const first = { installationId: offer.installationId, setupId: offer.setupId, deviceId: randomUUID() };
  const cancel = { operation: 'cancel', timeout: 3, ...first };
  assert.deepEqual(responseBody(await host.ownerSetup(cancel).result), { ...first, outcome: 'notPaired' });
  assert.equal((await host.ownerSetup({ ...first, operation: 'complete', timeout: 3, deviceToken: randomBytes(32).toString('base64url') }).result).error, 'unavailable');
  await host.close();
  const reopened = await local.start(false);
  assert.deepEqual(responseBody(await reopened.ownerSetup(cancel).result), { ...first, outcome: 'notPaired' });
  const next = responseBody(await reopened.ownerSetup({ operation: 'begin', timeout: 3 }).result);
  const identity = { installationId: next.installationId, setupId: next.setupId, deviceId: randomUUID() };
  const token = randomBytes(32).toString('base64url');
  responseBody(await reopened.ownerSetup({ ...identity, operation: 'complete', timeout: 3, deviceToken: token }).result);
  assert.deepEqual(responseBody(await reopened.ownerSetup(cancel).result), { ...first, outcome: 'notPaired' });
  assert.equal((await reopened.api('/v1/pairing', undefined, token)).status, 200);
  assert.equal((await reopened.api('/v1/owner/setup/cancel', identity, token)).status, 404);
  const completedCancel = { operation: 'cancel', timeout: 3, ...identity };
  const proof = { ...identity, outcome: 'revoked', runtimeStopped: true };
  assert.deepEqual(responseBody(await reopened.ownerSetup(completedCancel).result), proof);
  assert.equal((await reopened.api('/v1/pairing', undefined, token)).status, 401);
  await reopened.close();
  const final = await local.start(false);
  assert.deepEqual(responseBody(await final.ownerSetup(completedCancel).result), proof);
  assert.equal((await final.ownerSetup({ operation: 'begin', timeout: 3 }).result).error, 'unavailable');
  assert.equal(final.executions(), 0);
}

test('private pipe cancellation fences pending setup and revokes exact completed setup across restart', t => ownerCancellationJourney(t));

test('pipe cancellation rejects malformed identity, credential fields and deadlines before invoking core', async t => {
  const cancel = { operation: 'cancel', timeout: 1, installationId: 'synthetic', setupId: 'synthetic', deviceId: 'synthetic' };
  for (const changed of [{ setupId: '' }, { deviceId: '../other' }, { installationId: 'x'.repeat(129) }, { timeout: 31 }, { timeout: 0 }, { timeout: null }, { deviceToken: 'forbidden' }, { token: 'forbidden' }, { unexpected: true }, { operation: 'discard' }]) {
    const local = await fixture(t, undefined, true), host = await local.start();
    host.raw(JSON.stringify({ protocolVersion: 1, kind: 'ownerSetup', id: 1, setup: { ...cancel, ...changed } }) + '\n');
    await host.failed(); await host.close();
    assert.equal(host.cancellations(), 0);
  }
});

test('pipe cancellation stays unavailable when its composition callback is absent', async t => {
  const local = await fixture(t, undefined, true, 'cancel-disabled'), host = await local.start();
  const offer = responseBody(await host.ownerSetup({ operation: 'begin', timeout: 3 }).result);
  const identity = { installationId: offer.installationId, setupId: offer.setupId, deviceId: randomUUID() };
  assert.equal((await host.ownerSetup({ ...identity, operation: 'cancel', timeout: 3 }).result).error, 'unavailable');
  const token = randomBytes(32).toString('base64url');
  responseBody(await host.ownerSetup({ ...identity, operation: 'complete', timeout: 3, deviceToken: token }).result);
  assert.equal((await host.api('/v1/pairing', undefined, token)).status, 200);
  await host.close();
  assert.equal(host.cancellations(), 0);
});

test('pipe cancellation validates all receipt identities and confirmed shutdown without leaking callback details', async t => {
  for (const behavior of ['cancel-wrong-installation', 'cancel-wrong-setup', 'cancel-wrong-device', 'cancel-unconfirmed', 'cancel-bad-outcome', 'cancel-throws']) {
    const local = await fixture(t, undefined, true, behavior), host = await local.start();
    const offer = responseBody(await host.ownerSetup({ operation: 'begin', timeout: 3 }).result);
    const result = await host.ownerSetup({ operation: 'cancel', timeout: 3, installationId: offer.installationId, setupId: offer.setupId, deviceId: randomUUID() }).result;
    assert.equal(result.error, 'unavailable');
    assert.equal(JSON.stringify(result).includes('synthetic-secret'), false);
    await host.close();
    assert.equal(host.cancellations(), 1);
  }
});

test('timed out or canceled pipe cancellation remains uncertain until its exact durable result is retried', async t => {
  for (const completed of [false, true]) for (const canceledReply of [false, true]) {
    const local = await fixture(t, undefined, true, 'cancel-slow'), host = await local.start();
    const offer = responseBody(await host.ownerSetup({ operation: 'begin', timeout: 3 }).result);
    const identity = { installationId: offer.installationId, setupId: offer.setupId, deviceId: randomUUID() }, token = randomBytes(32).toString('base64url');
    const complete = { ...identity, operation: 'complete', timeout: 3, deviceToken: token };
    if (completed) responseBody(await host.ownerSetup(complete).result);
    const pending = host.ownerSetup({ ...identity, operation: 'cancel', timeout: canceledReply ? 3 : 0.01 });
    if (canceledReply) host.cancel(pending.id);
    assert.equal((await pending.result).error, canceledReply ? 'canceled' : 'timeout');
    const proof = completed ? { ...identity, outcome: 'revoked', runtimeStopped: true } : { ...identity, outcome: 'notPaired' };
    assert.deepEqual(responseBody(await host.ownerSetup({ ...identity, operation: 'cancel', timeout: 3 }).result), proof);
    assert.equal((await host.ownerSetup(complete).result).error, 'unavailable');
    assert.equal((await host.api('/v1/pairing', undefined, token)).status, 401);
    await host.close();
    const reopened = await local.start(false);
    assert.deepEqual(responseBody(await reopened.ownerSetup({ ...identity, operation: 'cancel', timeout: 3 }).result), proof);
    await reopened.close();
  }
});

test('pipe cancellation shares the eight-operation concurrency limit and never repeats completion', async t => {
  for (const overflow of [false, true]) {
    const local = await fixture(t, undefined, true, 'cancel-slow'), host = await local.start();
    const offer = responseBody(await host.ownerSetup({ operation: 'begin', timeout: 3 }).result);
    const identity = { installationId: offer.installationId, setupId: offer.setupId, deviceId: randomUUID() };
    const cancel = { ...identity, operation: 'cancel', timeout: 3 };
    if (overflow) {
      host.raw(Array.from({ length: 9 }, (_, index) => JSON.stringify({ protocolVersion: 1, kind: 'ownerSetup', id: index + 2, setup: cancel }) + '\n').join(''));
      await host.failed();
    } else {
      const proofs = await Promise.all(Array.from({ length: 8 }, () => host.ownerSetup(cancel).result.then(responseBody)));
      for (const proof of proofs) assert.deepEqual(proof, { ...identity, outcome: 'notPaired' });
    }
    await host.close();
    assert.equal(host.cancellations(), 8);
    assert.equal(host.executions(), 0);
  }
});

test('setup stays disabled without composition and rejects malformed operations before calling core', async t => {
  const begin = { operation: 'begin', timeout: 1 };
  const complete = { operation: 'complete', timeout: 1, installationId: 'synthetic', setupId: 'synthetic', deviceId: 'synthetic', deviceToken: randomBytes(32).toString('base64url') };
  for (const value of [begin, { ...begin, timeout: 31 }, { ...begin, timeout: 0 }, { ...begin, unexpected: 'field' },
    { ...complete, operation: 'replace' }, { ...complete, setupId: '' }, { ...complete, deviceId: '../wrong' },
    { ...complete, installationId: 'x'.repeat(129) }, { ...complete, deviceToken: 'x' }, { ...complete, deviceToken: 'A'.repeat(42) + 'B' }, { ...complete, token: 'unexpected' }]) {
    const local = await fixture(t, undefined, value !== begin), host = await local.start();
    host.raw(JSON.stringify({ protocolVersion: 1, kind: 'ownerSetup', id: 1, setup: value }) + '\n');
    await host.failed(); await host.close();
  }
});

test('a timed out or canceled setup reply stays uncertain until the same prepared credential is confirmed', async t => {
  for (const cancel of [false, true]) {
    const local = await fixture(t, undefined, true, 'setup-slow'), host = await local.start();
    const offer = responseBody(await host.ownerSetup({ operation: 'begin', timeout: 3 }).result);
    const setup = { operation: 'complete', timeout: cancel ? 3 : 0.01, installationId: offer.installationId, setupId: offer.setupId, deviceId: randomUUID(), deviceToken: randomBytes(32).toString('base64url') };
    const pending = host.ownerSetup(setup);
    if (cancel) host.cancel(pending.id);
    assert.equal((await pending.result).error, cancel ? 'canceled' : 'timeout');
    const receipt = responseBody(await host.ownerSetup({ ...setup, timeout: 3 }).result);
    assert.deepEqual(receipt, { installationId: offer.installationId, deviceId: setup.deviceId, expiresAt: null });
    assert.equal((await host.api('/v1/pairing', undefined, setup.deviceToken)).status, 200);
    await host.close();
  }
});

test('composition output is validated and only receipt fields cross the setup channel', async t => {
  for (const behavior of ['setup-bad-offer', 'setup-wrong', 'setup-throws']) {
    const local = await fixture(t, undefined, true, behavior), host = await local.start();
    const begun = await host.ownerSetup({ operation: 'begin', timeout: 3 }).result;
    if (behavior === 'setup-bad-offer') assert.equal(begun.error, 'unavailable');
    else {
      const offer = responseBody(begun);
      const result = await host.ownerSetup({ operation: 'complete', timeout: 3, installationId: offer.installationId, setupId: offer.setupId, deviceId: randomUUID(), deviceToken: randomBytes(32).toString('base64url') }).result;
      assert.equal(result.error, 'unavailable');
      assert.equal(JSON.stringify(result).includes('synthetic-secret'), false);
    }
    await host.close();
  }
});

test('concurrent matching setup retries use the same pairing and share the channel concurrency limit', async t => {
  for (const overflow of [false, true]) {
    const local = await fixture(t, undefined, true, 'setup-slow'), host = await local.start();
    const offer = responseBody(await host.ownerSetup({ operation: 'begin', timeout: 3 }).result);
    const setup = { operation: 'complete', timeout: 3, installationId: offer.installationId, setupId: offer.setupId, deviceId: randomUUID(), deviceToken: randomBytes(32).toString('base64url') };
    if (overflow) {
      host.raw(Array.from({ length: 9 }, (_, index) => JSON.stringify({ protocolVersion: 1, kind: 'ownerSetup', id: index + 2, setup }) + '\n').join(''));
      await host.failed();
    } else {
      const receipts = await Promise.all(Array.from({ length: 8 }, () => host.ownerSetup(setup).result.then(responseBody)));
      for (const receipt of receipts) assert.deepEqual(receipt, { installationId: offer.installationId, deviceId: setup.deviceId, expiresAt: null });
      assert.equal((await host.api('/v1/pairing', undefined, setup.deviceToken)).status, 200);
    }
    await host.close();
  }
});

test('explicit owner pipe recovery verifies exact identity and revokes without a bearer-token exception', async t => {
  const local = await fixture(t, undefined, true), host = await local.start();
  const enrolled = await host.api('/v1/devices/enroll', { ...object(host.setup.enrollment), deviceName: 'Synthetic owner' });
  const token = string(enrolled.body.deviceToken), deviceID = string(enrolled.body.deviceId), installationID = string(host.installed.installationId);
  assert.equal((await host.api('/v1/pairing/recover', { installationID, deviceID }, token)).status, 404);
  assert.equal((await host.recover('foreign', deviceID)).error, 'unavailable');
  assert.equal((await host.recover(installationID, 'foreign')).error, 'unavailable');
  assert.equal((await host.api('/v1/pairing', undefined, token)).status, 200);
  const result = await host.recover(installationID, deviceID);
  assert.equal(result.kind, 'response');
  assert.deepEqual(JSON.parse(Buffer.from(string(object(result.response).body), 'base64').toString()), { installationId: installationID, deviceId: deviceID, runtimeStopped: true, revoked: true });
  assert.equal((await host.api('/v1/pairing', undefined, token)).status, 401);
  assert.deepEqual(await host.recover(installationID, deviceID).then(frame => frame.response), result.response);
  await host.close();
});

test('owner pipe recovery stays disabled by default and rejects malformed control identity or limits', async t => {
  const recovery = { installationID: 'synthetic', deviceID: 'synthetic', timeout: 1 };
  for (const value of [recovery, { ...recovery, deviceID: '' }, { ...recovery, installationID: 'a'.repeat(129) },
    { ...recovery, deviceID: 'wrong/path' }, { ...recovery, timeout: 31 }, { ...recovery, token: 'unexpected' }]) {
    const local = await fixture(t, undefined, value !== recovery), host = await local.start();
    host.raw(JSON.stringify({ protocolVersion: 1, kind: 'ownerRecovery', id: 1, recovery: value }) + '\n');
    await host.failed(); await host.close();
  }
});

test('timeout and cancellation never claim recovery when disposal completes later', async t => {
  for (const cancel of [false, true]) {
    const local = await fixture(t, undefined, true, 'slow'), host = await local.start();
    const enrolled = await host.api('/v1/devices/enroll', { ...object(host.setup.enrollment), deviceName: 'Synthetic owner' });
    const installationID = string(host.installed.installationId), deviceID = string(enrolled.body.deviceId);
    const pending = host.recovery(installationID, deviceID, cancel ? 3 : 0.01);
    if (cancel) host.cancel(pending.id);
    assert.equal((await pending.result).error, cancel ? 'canceled' : 'timeout');
    assert.equal((await host.recover(installationID, deviceID)).kind, 'response');
    await host.close();
  }
});

test('a composition callback cannot return a recovery proof for another pairing', async t => {
  const local = await fixture(t, undefined, true, 'wrong-proof'), host = await local.start();
  const enrolled = await host.api('/v1/devices/enroll', { ...object(host.setup.enrollment), deviceName: 'Synthetic owner' });
  assert.equal((await host.recover(string(host.installed.installationId), string(enrolled.body.deviceId))).error, 'unavailable');
  await host.close();
});

test('the compiled vendor-free artifact supports the same private pipe journey', async t => {
  const root = await mkdtemp('/tmp/fort-private-channel-artifact-');
  t.after(() => rm(root, { recursive: true, force: true }));
  const artifact = join(root, 'core');
  const build = spawnSync(process.execPath, [fileURLToPath(new URL('../packaging/core-artifact.ts', import.meta.url)), '--destination', artifact], { encoding: 'utf8', timeout: 30_000 });
  assert.equal(build.status, 0, build.stderr);
  await journey(t, artifact);
  await ownerSetupJourney(t, artifact);
  await ownerCancellationJourney(t, artifact);
});

test('private pipe rejects malformed, oversized, foreign-route, invalid-body and stale request frames', async t => {
  const request = { protocolVersion: 1, kind: 'request', id: 1, request: { path: '/v1/installation', method: 'GET', installationID: 'fixture', timeout: 1 } };
  for (const frame of ['not-json\n', 'a'.repeat(2 * 1024 * 1024 + 1),
    JSON.stringify({ ...request, protocolVersion: 2 }) + '\n',
    JSON.stringify({ ...request, request: { ...request.request, path: 'https://elsewhere.invalid/v1/installation' } }) + '\n',
    JSON.stringify({ ...request, request: { ...request.request, method: 'POST', body: 'invalid!' } }) + '\n',
    JSON.stringify({ protocolVersion: 1, kind: 'cancel', id: 99 }) + '\n']) {
    const local = await fixture(t), host = await local.start(); host.raw(frame); await host.failed();
    await host.close(); await assert.rejects(stat(join(local.directory, 'core.sock')), { code: 'ENOENT' });
  }
  const local = await fixture(t), host = await local.start();
  await host.api('/v1/installation'); host.raw(JSON.stringify(request) + '\n'); await host.failed();
});

test('private pipe bounds concurrent waits and closes all requests on EOF', async t => {
  for (const overload of [false, true]) {
    const local = await fixture(t), host = await local.start();
    const enrollment = await host.api('/v1/devices/enroll', { ...object(host.setup.enrollment), deviceName: 'Synthetic wait peer' });
    const token = string(enrollment.body.deviceToken);
    const page = (await host.api('/v1/events?after=0&recoveryEpoch=' + host.installed.recoveryEpoch, undefined, token)).body;
    if (overload) host.expectFailure();
    for (let i = 0; i < (overload ? 9 : 1); i++) {
      void host.send('/v1/events?after=' + page.cursor + '&recoveryEpoch=' + host.installed.recoveryEpoch + '&wait=25000', undefined, token, 30).result.catch(() => {});
    }
    if (overload) await host.failed();
    else { await delay(50); host.end(); }
    await host.close(); await assert.rejects(stat(join(local.directory, 'core.sock')), { code: 'ENOENT' });
  }
});

test('private pipe fails within its output bound when the owner stops reading large responses', async t => {
  const local = await fixture(t), host = await local.start();
  const enrollment = await host.api('/v1/devices/enroll', { ...object(host.setup.enrollment), deviceName: 'Synthetic backpressure peer' });
  const token = string(enrollment.body.deviceToken);
  const agent = (await host.api('/v1/agents', { name: 'Backpressure Home', model: 'exact-synthetic', instructions: 'Echo.' }, token)).body;
  host.pauseOutput(); host.expectFailure(); const started = performance.now();
  for (let i = 0; i < 3; i++) {
    void host.send('/v1/conversations/' + string(agent.homeConversationId) + '/messages', {
      requestId: 'backpressure-' + i, text: 'a'.repeat(65_536), recipientAgentId: agent.agentId,
      bindingRevisionId: object(agent.binding).bindingRevisionId, recoveryEpoch: host.installed.recoveryEpoch, skills: []
    }, token).result.catch(() => {});
  }
  await host.failed(); assert.ok(performance.now() - started < 8000, 'Output drain did not fail within its bound');
  await host.close(); await assert.rejects(stat(join(local.directory, 'core.sock')), { code: 'ENOENT' });
});
