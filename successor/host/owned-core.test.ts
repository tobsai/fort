import assert from 'node:assert/strict';
import { fork, spawnSync } from 'node:child_process';
import { chmod, mkdir, mkdtemp, readFile, readdir, rm, stat, symlink, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { randomBytes } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import { test, type TestContext } from 'node:test';
import { PassThrough } from 'node:stream';
import { serveOwnedCore } from './owned-core.ts';

function object(value: unknown): Record<string, unknown> {
  assert.ok(value && typeof value === 'object' && !Array.isArray(value));
  return value as Record<string, unknown>;
}
function string(value: unknown): string { assert.equal(typeof value, 'string'); return value as string; }
function body(frame: Record<string, unknown>, status = 200) {
  assert.equal(frame.kind, 'response');
  const response = object(frame.response); assert.equal(response.status, status);
  return object(JSON.parse(Buffer.from(string(response.body), 'base64').toString()));
}
async function fixture(t: TestContext) {
  const directory = await mkdtemp('/tmp/fort-owned-core-');
  const closers: (() => Promise<void>)[] = [];
  let completedDisposals = 0;
  t.after(async () => { for (const close of closers) await close(); await rm(directory, { recursive: true, force: true }); });
  return { directory, disposals: () => completedDisposals, async start(options: { start?: 'restore' | 'resume'; behavior?: string; modulePath?: string; expectFailure?: boolean } = {}) {
    const { start = 'restore', behavior = 'ordinary', modulePath = fileURLToPath(new URL('./owned-core.ts', import.meta.url)) } = options;
    const starting = performance.now();
    const child = fork(fileURLToPath(new URL('./fixtures/owned-core.mjs', import.meta.url)), [modulePath, directory, start, behavior], {
      execPath: process.execPath, execArgv: modulePath.endsWith('.js') ? ['--jitless', '--no-experimental-strip-types'] : [],
      cwd: directory, detached: true, stdio: ['pipe', 'pipe', 'pipe', 'ipc'], env: { PATH: '/usr/bin:/bin', LANG: 'C' },
    });
    const ready = Promise.withResolvers<void>(), exited = Promise.withResolvers<{ code: number | null; signal: NodeJS.Signals | null }>();
    const responses = new Map<number, ReturnType<typeof Promise.withResolvers<Record<string, unknown>>>>();
    let pending = '', diagnostics = '', nextID = 0, executions = 0, disposals = 0, expectedExit = options.expectFailure ? 1 : 0;
    child.stdin?.on('error', () => {});
    child.stderr?.on('data', chunk => { diagnostics = (diagnostics + chunk.toString()).slice(-2000); });
    child.once('error', error => ready.reject(error));
    child.once('exit', (code, signal) => exited.resolve({ code, signal }));
    child.on('message', value => { const message = object(value); if (message.kind === 'executed') executions++; if (message.kind === 'disposed') { disposals++; completedDisposals++; } });
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
      const pid = child.pid; assert.ok(pid); assert.throws(() => process.kill(-pid, 0), { code: 'ESRCH' });
    };
    closers.push(close);
    await Promise.race([ready.promise, exited.promise.then(() => { throw new Error('Owned fixture exited before ready: ' + diagnostics); }), delay(5000, undefined, { ref: false }).then(() => { throw new Error('Owned fixture startup timed out'); })]);
    const readyMs = performance.now() - starting;
    async function command(kind: string, value: unknown) {
      const id = ++nextID, result = Promise.withResolvers<Record<string, unknown>>(); responses.set(id, result);
      const key = kind === 'request' ? 'request' : kind === 'ownerSetup' ? 'setup' : 'recovery';
      child.stdin?.write(JSON.stringify({ protocolVersion: 1, kind, id, [key]: value }) + '\n');
      return Promise.race([result.promise, exited.promise.then(() => { throw new Error('Owned host exited'); }), delay(5000, undefined, { ref: false }).then(() => { throw new Error('Owned command timed out'); })]).finally(() => responses.delete(id));
    }
    return { close, command, readyMs, executions: () => executions, disposals: () => disposals,
      async api(installationID: string, token: string | undefined, path: string, value?: unknown, status = 200) {
        return body(await command('request', { path, installationID, token, method: value === undefined ? 'GET' : 'POST', timeout: 3, body: value === undefined ? undefined : Buffer.from(JSON.stringify(value)).toString('base64') }), status);
      },
      abort() { child.send('abort'); },
      raw(bytes: string) { expectedExit = 1; child.stdin?.write(bytes); },
    };
  } };
}

async function journey(t: TestContext, modulePath?: string) {
  const local = await fixture(t), first = await local.start({ modulePath });
  const offer = body(await first.command('ownerSetup', { operation: 'begin', timeout: 3 }));
  assert.equal(offer.kind, 'available');
  const installationId = string(offer.installationId), deviceToken = randomBytes(32).toString('base64url');
  const setup = { installationId, setupId: string(offer.setupId), deviceId: 'synthetic-owner', deviceToken };
  const receipt = body(await first.command('ownerSetup', { operation: 'complete', ...setup, timeout: 3 }));
  assert.deepEqual(receipt, { installationId, deviceId: setup.deviceId, expiresAt: null });
  const installation = await first.api(installationId, deviceToken, '/v1/installation');
  assert.equal(installation.installationId, installationId);
  await first.api(installationId, deviceToken, '/v1/fixture-bootstrap', undefined, 404);
  await first.api(installationId, undefined, '/v1/installation', undefined, 401);
  const agent = await first.api(installationId, deviceToken, '/v1/agents', { name: 'Owned host Home', model: 'exact-synthetic', instructions: 'Echo the supplied text.' }, 201);
  const path = '/v1/conversations/' + string(agent.homeConversationId);
  const request = { requestId: 'owned-exact-turn', text: '  Cedar 🌲 Café e\u0301\r\n', recipientAgentId: agent.agentId,
    bindingRevisionId: object(agent.binding).bindingRevisionId, recoveryEpoch: installation.recoveryEpoch, skills: [] };
  const accepted = await first.api(installationId, deviceToken, path + '/messages', request, 201);
  let snapshot = await first.api(installationId, deviceToken, path + '/snapshot');
  for (let count = 0; count < 50 && (!Array.isArray(snapshot.messages) || snapshot.messages.length !== 2); count++) {
    await delay(10); snapshot = await first.api(installationId, deviceToken, path + '/snapshot');
  }
  assert.ok(Array.isArray(snapshot.messages)); assert.deepEqual(snapshot.messages.map(message => object(message).text), [request.text, request.text]);
  assert.equal(first.executions(), 1);
  await first.close(); assert.equal(first.disposals(), 1);
  await assert.rejects(stat(join(local.directory, 'core.sock')), { code: 'ENOENT' });
  const second = await local.start({ modulePath });
  assert.equal((await second.api(installationId, deviceToken, '/v1/installation')).installationId, installationId);
  assert.deepEqual(body(await second.command('ownerSetup', { operation: 'begin', timeout: 3 })), { kind: 'paired', ...receipt });
  assert.deepEqual((await second.api(installationId, deviceToken, path + '/messages', request, 201)).receipt, accepted.receipt);
  assert.equal(second.executions(), 0);
  assert.deepEqual(await second.api(installationId, deviceToken, '/v1/host/stop', {}), { runtimeStopped: true, revoked: false });
  await second.close(); assert.equal(second.disposals(), 1);
  const paused = await local.start({ modulePath });
  assert.equal((await paused.api(installationId, deviceToken, '/v1/runtime')).state, 'unavailable');
  await paused.api(installationId, deviceToken, '/v1/agents', { name: 'Blocked', model: 'exact-synthetic', instructions: 'Do not execute.' }, 503);
  assert.equal(paused.executions(), 0);
  await paused.close();
  const resumed = await local.start({ modulePath, start: 'resume' });
  const retained = await resumed.api(installationId, deviceToken, path + '/snapshot');
  assert.deepEqual(retained.messages, snapshot.messages);
  await resumed.api(installationId, deviceToken, '/v1/agents', { name: 'Resumed', model: 'exact-synthetic', instructions: 'Explicit start.' }, 201);
  await resumed.close();
  if (process.env.FORT_OWNED_CORE_BENCHMARK === '1') {
    const samples = { freshMs: first.readyMs, reopenedMs: [second.readyMs, paused.readyMs, resumed.readyMs] };
    assert.ok(samples.freshMs <= 2000); assert.ok(samples.reopenedMs.every(value => value <= 500));
    process.stdout.write('OWNED_CORE_STARTUPS=' + JSON.stringify(samples) + '\n');
  }
}

test('owned core preserves exact send/replay, owner pairing and stopped intent until explicit resume', t => journey(t));

test('the compiled artifact runs the owned core journey with vendor implementations absent', async t => {
  const directory = await mkdtemp('/tmp/fort-owned-artifact-');
  t.after(() => rm(directory, { recursive: true, force: true }));
  const artifact = join(directory, 'Core');
  const result = spawnSync(process.execPath, [fileURLToPath(new URL('../packaging/core-artifact.ts', import.meta.url)), '--destination', artifact], { encoding: 'utf8', timeout: 30_000 });
  assert.equal(result.status, 0, result.stderr);
  await journey(t, join(artifact, 'host/owned-core.js'));
});

test('owned core refuses a missing runtime before creating a ledger or becoming ready', async t => {
  const local = await fixture(t);
  await assert.rejects(local.start({ behavior: 'missing-runtime', expectFailure: true }), /before ready/);
  assert.deepEqual(await readdir(local.directory), ['core-pid.json']);
});

test('failed private socket startup disposes its runtime and preserves the unknown path', async t => {
  const local = await fixture(t), socket = join(local.directory, 'core.sock');
  await writeFile(socket, 'synthetic unrelated marker', { mode: 0o600 });
  await assert.rejects(local.start({ expectFailure: true }), /before ready/);
  assert.equal(local.disposals(), 1);
  assert.equal(await readFile(socket, 'utf8'), 'synthetic unrelated marker');
  await rm(socket);
  const reopened = await local.start();
  assert.equal(body(await reopened.command('ownerSetup', { operation: 'begin', timeout: 3 })).kind, 'available');
  await reopened.close(); assert.equal(local.disposals(), 2);
});

test('owned core disposes transferred runtime without mutating rejected storage or pre-aborted startup', async t => {
  const root = await mkdtemp('/tmp/fort-owned-preflight-');
  t.after(() => rm(root, { recursive: true, force: true }));
  const directory = join(root, 'private'), shared = join(root, 'shared'), linked = join(root, 'linked'), file = join(root, 'file');
  await mkdir(directory, { mode: 0o700 }); await mkdir(shared, { mode: 0o700 }); await chmod(shared, 0o755);
  await symlink(directory, linked); await writeFile(file, 'owned test marker');
  const cases = [directory, shared, linked, file, join(root, 'absent'), 'relative'];
  for (const dataDirectory of cases) {
    let disposed = 0;
    await assert.rejects(serveOwnedCore({ dataDirectory, input: new PassThrough(), output: new PassThrough(),
      provider: { provider: 'preflight-fixture', name: 'Preflight', runtimeVersion: '1', hostId: 'synthetic', session: 'stateless', capabilities: ['plainText'] },
      runtime: { async execute() { throw new Error('No execution permitted'); }, async dispose() { await delay(1); disposed++; } },
    }));
    assert.equal(disposed, 1);
  }
  let disposed = 0;
  await assert.rejects(serveOwnedCore({ dataDirectory: join(root, 'aborted'), input: new PassThrough(), output: new PassThrough(), signal: AbortSignal.abort(),
    provider: { provider: 'preflight-fixture', name: 'Preflight', runtimeVersion: '1', hostId: 'synthetic', session: 'stateless', capabilities: ['plainText'] },
    runtime: { async execute() { throw new Error('No execution permitted'); }, async dispose() { disposed++; } },
  }));
  assert.equal(disposed, 1);
  assert.deepEqual(await readdir(directory), []); assert.deepEqual(await readdir(shared), []);
  assert.equal((await stat(shared)).mode & 0o777, 0o755);
  assert.deepEqual((await readdir(root)).sort(), ['file', 'linked', 'private', 'shared']);
});

test('owned core confirms pending setup cancellation and exact recovery without a network control route', async t => {
  const local = await fixture(t), host = await local.start();
  const offer = body(await host.command('ownerSetup', { operation: 'begin', timeout: 3 }));
  const identity = { installationId: string(offer.installationId), setupId: string(offer.setupId), deviceId: 'synthetic-owner' };
  const canceled = body(await host.command('ownerSetup', { operation: 'cancel', ...identity, timeout: 3 }));
  assert.deepEqual(canceled, { ...identity, outcome: 'notPaired' });
  const replacement = body(await host.command('ownerSetup', { operation: 'begin', timeout: 3 }));
  assert.notEqual(replacement.setupId, offer.setupId);
  const token = randomBytes(32).toString('base64url');
  body(await host.command('ownerSetup', { operation: 'complete', ...identity, setupId: replacement.setupId, deviceToken: token, timeout: 3 }));
  const rejected = body(await host.command('ownerRecovery', { installationID: identity.installationId, deviceID: 'foreign', timeout: 3 }), 409);
  assert.deepEqual(rejected, { installationId: identity.installationId, deviceId: 'foreign', outcome: 'identityRejected' });
  await host.api(identity.installationId, token, '/v1/pairing/recover', {}, 404);
  const proof = body(await host.command('ownerRecovery', { installationID: identity.installationId, deviceID: identity.deviceId, timeout: 3 }));
  assert.deepEqual(proof, { installationId: identity.installationId, deviceId: identity.deviceId, runtimeStopped: true, revoked: true });
  await host.api(identity.installationId, token, '/v1/installation', undefined, 401);
  await host.close(); assert.equal(host.disposals(), 1);
});

test('abort, malformed input and failed cleanup cannot leave a ready owned host', async t => {
  for (const behavior of ['abort', 'malformed', 'cleanup-failure']) {
    const local = await fixture(t), host = await local.start({ behavior, expectFailure: behavior === 'cleanup-failure' });
    if (behavior === 'abort') host.abort();
    if (behavior === 'malformed') host.raw('not-json\n');
    await host.close(); assert.equal(host.disposals(), 1);
    await assert.rejects(stat(join(local.directory, 'core.sock')), { code: 'ENOENT' });
  }
});
