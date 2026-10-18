import assert from 'node:assert/strict';
import { fork, spawnSync } from 'node:child_process';
import { mkdtemp, rm, stat } from 'node:fs/promises';
import { join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import { test, type TestContext } from 'node:test';

function object(value: unknown): Record<string, unknown> {
  assert.ok(value && typeof value === 'object' && !Array.isArray(value)); return value as Record<string, unknown>;
}
function string(value: unknown): string { assert.equal(typeof value, 'string'); return value as string; }
function array(value: unknown): unknown[] { assert.ok(Array.isArray(value)); return value; }

async function fixture(t: TestContext, artifact?: string) {
  const directory = await mkdtemp('/tmp/fort-private-channel-');
  const closers: (() => Promise<void>)[] = [];
  t.after(async () => { for (const close of closers) await close(); await rm(directory, { recursive: true, force: true }); });
  return { directory, start: async () => {
    const coreModule = artifact ? pathToFileURL(join(artifact, 'service/openai.js')).href : new URL('../service/openai.ts', import.meta.url).href;
    const channelModule = artifact ? pathToFileURL(join(artifact, 'host/private-channel.js')).href : new URL('./private-channel.ts', import.meta.url).href;
    const child = fork(fileURLToPath(new URL('./fixtures/private-core.mjs', import.meta.url)), [directory, coreModule, channelModule], {
      execPath: process.execPath, execArgv: artifact ? ['--jitless', '--no-experimental-strip-types'] : [], cwd: directory, detached: true,
      stdio: ['pipe', 'pipe', 'pipe', 'ipc'], env: { PATH: '/usr/bin:/bin', LANG: 'C' }
    });
    const setup = Promise.withResolvers<Record<string, unknown>>(), ready = Promise.withResolvers<void>();
    const exited = Promise.withResolvers<{ code: number | null; signal: NodeJS.Signals | null }>();
    const responses = new Map<number, ReturnType<typeof Promise.withResolvers<Record<string, unknown>>>>();
    let pending = '', diagnostics = '', nextID = 0, executions = 0, expectedExit = 0;
    child.stdin?.on('error', () => {});
    child.stderr?.on('data', chunk => { diagnostics = (diagnostics + chunk.toString()).slice(-2000); });
    child.on('message', value => { const message = object(value); if (message.kind === 'setup') setup.resolve(message); if (message.kind === 'executed') executions++; });
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
    return { installed, setup: await setup.promise, api, send, executions: () => executions, close,
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

test('the compiled vendor-free artifact supports the same private pipe journey', async t => {
  const root = await mkdtemp('/tmp/fort-private-channel-artifact-');
  t.after(() => rm(root, { recursive: true, force: true }));
  const artifact = join(root, 'core');
  const build = spawnSync(process.execPath, [fileURLToPath(new URL('../packaging/core-artifact.ts', import.meta.url)), '--destination', artifact], { encoding: 'utf8', timeout: 30_000 });
  assert.equal(build.status, 0, build.stderr);
  await journey(t, artifact);
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
