import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { PassThrough } from 'node:stream';
import { mkdtemp, realpath, rm } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { setTimeout as delay } from 'node:timers/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { executionDigest } from '../contracts/executor.ts';
import type { ModelCatalog } from '../contracts/v1.ts';
import { serveExecutorWorker } from './executor-worker.ts';
import { ExecutorProcess } from './executor-process.ts';
import type { ChatRuntime } from './chat-runtime.ts';

const binding = { provider: 'catalog-fixture', runtimeVersion: 'fixture-1', hostId: 'fixture-host' };
const catalog: ModelCatalog = { provider: binding.provider, state: 'ready', checkedAt: '2026-10-18T00:00:00.000Z',
  models: [{ id: 'exact-text', name: 'Text model', availability: 'available' },
    { id: 'exact-audio', name: 'Audio model', availability: 'unavailable', detail: 'Text input unavailable.' }] };
function request(account?: { accountId: string; userLabel: string }) {
  const body = { ...(account ? { account } : {}), protocolVersion: 1, kind: 'models', requestId: randomUUID(), binding: { ...binding }, deadline: Date.now() + 5000 };
  return { ...body, requestDigest: executionDigest(body) };
}

test('the owned worker returns the exact catalog only after runtime disposal and never starts a turn', async () => {
  const input = new PassThrough(), output = new PassThrough(), chunks: Buffer[] = [], events: string[] = [];
  output.on('data', (chunk: Buffer) => { events.push('output'); chunks.push(chunk); });
  const done = serveExecutorWorker({ input, output, binding,
    createRuntime() { throw new Error('Discovery must not use the turn factory'); },
    createDiscovery() {
      return {
        async execute() { throw new Error('Discovery must not start a turn'); },
        async models(_signal, account) { assert.deepEqual(account, { accountId: 'workspace-fixture', userLabel: 'owner@fixture.invalid' }); events.push('models'); return catalog; },
        async dispose() { events.push('disposed'); },
      };
    },
  });
  const command = request({ accountId: 'workspace-fixture', userLabel: 'owner@fixture.invalid' }); input.write(JSON.stringify(command) + '\n');
  await done;
  assert.deepEqual(events, ['models', 'disposed', 'output']);
  assert.deepEqual(JSON.parse(Buffer.concat(chunks).toString()), { protocolVersion: 1, kind: 'catalog', sequence: 1,
    requestId: command.requestId, requestDigest: command.requestDigest, result: catalog });
  assert.equal(input.destroyed, true); assert.equal(output.writableFinished, true);
});

function streams(runtime: ChatRuntime, configured = true) {
  const input = new PassThrough(), output = new PassThrough(), chunks: Buffer[] = [], counts = { created: 0, turns: 0 };
  output.on('data', (chunk: Buffer) => chunks.push(chunk));
  const done = serveExecutorWorker({ input, output, binding,
    createRuntime() { counts.turns++; throw new Error('Unexpected turn'); },
    createDiscovery: configured ? () => { counts.created++; return runtime; } : undefined,
  });
  void done.catch(() => {});
  return { input, output, done, counts, bytes: () => Buffer.concat(chunks) };
}
function unusedRuntime(): ChatRuntime {
  return { async execute() { throw new Error('Unexpected turn'); }, async models() { return catalog; }, async dispose() {} };
}

test('unsupported, foreign, expired or overbroad discovery requests never construct a runtime', async t => {
  const { requestDigest: _digest, ...body } = request();
  const changed = [
    { ...body, protocolVersion: 2 }, { ...body, kind: 'dispatch' }, { ...body, requestId: 'not-a-request-id' },
    ...['provider', 'runtimeVersion', 'hostId'].map(key => ({ ...body, binding: { ...body.binding, [key]: 'foreign' } })),
    { ...body, deadline: Date.now() - 1 }, { ...body, deadline: Date.now() + 6000 }, { ...body, deadline: Date.now() + 0.5 },
    { ...body, executable: '/foreign' }, { ...body, grants: ['full-access'] }, { ...body, history: [] },
  ];
  for (const [index, value] of changed.entries()) await t.test(String(index), async () => {
    const h = streams(unusedRuntime());
    h.input.write(JSON.stringify({ ...value, requestDigest: executionDigest(value) }) + '\n');
    await assert.rejects(h.done); assert.deepEqual(h.counts, { created: 0, turns: 0 }); assert.equal(h.bytes().length, 0);
  });
  for (const scenario of ['unsupported', 'digest', 'wire-size']) await t.test(scenario, async () => {
    const h = streams(unusedRuntime(), scenario !== 'unsupported');
    const value = request(); if (scenario === 'digest') value.requestDigest = '0'.repeat(64);
    h.input.write((scenario === 'wire-size' ? ' '.repeat(4096) : '') + JSON.stringify(value) + '\n');
    await assert.rejects(h.done); assert.deepEqual(h.counts, { created: 0, turns: 0 }); assert.equal(h.bytes().length, 0);
  });
});

test('a canceled catalog cannot escape while runtime disposal is pending', async () => {
  const disposing = Promise.withResolvers<void>(), released = Promise.withResolvers<void>();
  const h = streams({ ...unusedRuntime(), async dispose() { disposing.resolve(); await released.promise; } });
  const command = request(); h.input.write(JSON.stringify(command) + '\n');
  await disposing.promise; assert.equal(h.bytes().length, 0);
  h.input.write(JSON.stringify({ protocolVersion: 1, kind: 'cancelModels', requestId: command.requestId, requestDigest: command.requestDigest }) + '\n');
  released.resolve(); await assert.rejects(h.done); assert.equal(h.bytes().length, 0);
});

test('a foreign catalog or failed disposal never produces terminal evidence', async t => {
  for (const scenario of ['foreign-provider', 'cleanup-fail', 'unsupported-method']) await t.test(scenario, async () => {
    let disposed = 0;
    const runtime = unusedRuntime();
    if (scenario === 'foreign-provider') runtime.models = async () => ({ ...catalog, provider: 'foreign' });
    if (scenario === 'unsupported-method') delete runtime.models;
    runtime.dispose = async () => { disposed++; if (scenario === 'cleanup-fail') throw new Error('synthetic cleanup failure'); };
    const h = streams(runtime); h.input.write(JSON.stringify(request()) + '\n');
    await assert.rejects(h.done); assert.equal(disposed, 1); assert.equal(h.bytes().length, 0); assert.equal(h.counts.turns, 0);
  });
});

test('channel loss, a wrong cancel or a second operation abort discovery without dispatching a turn', async t => {
  for (const scenario of ['eof', 'closed', 'wrong-cancel', 'second-operation', 'partial-frame']) await t.test(scenario, async () => {
    let disposed = 0, aborted = false;
    const started = Promise.withResolvers<void>();
    const runtime = unusedRuntime();
    runtime.models = async signal => {
      started.resolve();
      await new Promise<void>(resolve => signal.addEventListener('abort', () => { aborted = true; resolve(); }, { once: true }));
      return catalog;
    };
    runtime.dispose = async () => { disposed++; };
    const h = streams(runtime), command = request(); h.input.write(JSON.stringify(command) + '\n'); await started.promise;
    if (scenario === 'eof') h.input.end();
    else if (scenario === 'closed') h.input.destroy();
    else if (scenario === 'wrong-cancel') h.input.write(JSON.stringify({ protocolVersion: 1, kind: 'cancelModels', requestId: randomUUID(), requestDigest: command.requestDigest }) + '\n');
    else if (scenario === 'second-operation') h.input.write(JSON.stringify(request()) + '\n');
    else { h.input.write('{'); h.input.end(); }
    await assert.rejects(h.done); assert.ok(aborted); assert.equal(disposed, 1); assert.equal(h.bytes().length, 0);
    assert.deepEqual(h.counts, { created: 1, turns: 0 });
  });
});

test('the parent returns a catalog only after the real worker exits cleanly with its group absent', async t => {
  const directory = await realpath(await mkdtemp(join(tmpdir(), 'fort-model-catalog-')));
  const worker = fileURLToPath(new URL('./fixtures/owned-executor.mjs', import.meta.url));
  const selected = { ...binding };
  const runtime = new ExecutorProcess({ executable: process.execPath, args: [worker, 'models'], cwd: directory, env: {},
    lifecycle: { kind: 'processGroup' }, discovery: selected });
  selected.provider = 'foreign'; selected.hostId = 'foreign'; selected.runtimeVersion = 'foreign';
  t.after(async () => { await runtime.dispose(); await rm(directory, { recursive: true, force: true }); });
  assert.equal(typeof runtime.models, 'function'); assert.ok(runtime.models);
  assert.deepEqual(await runtime.models(new AbortController().signal), catalog);
  assert.equal(runtime.readiness().state, 'ready');
  assert.equal(runtime.lastExit?.code, 0); assert.equal(runtime.lastExit?.forced, false); assert.equal(runtime.lastExit?.groupAbsent, true);
  const pid = runtime.lastExit?.pid; assert.ok(pid);
  assert.throws(() => process.kill(pid, 0), { code: 'ESRCH' });
  assert.throws(() => process.kill(-pid, 0), { code: 'ESRCH' });
});

async function rawProcess(t: TestContext, scenario: string) {
  const directory = await realpath(await mkdtemp(join(tmpdir(), 'fort-model-wire-')));
  const worker = fileURLToPath(new URL('./fixtures/executor-worker.mjs', import.meta.url));
  const runtime = new ExecutorProcess({ executable: process.execPath, args: [worker, scenario], cwd: directory, env: {},
    lifecycle: { kind: 'processGroup' }, discovery: binding });
  t.after(async () => { await runtime.dispose(); await rm(directory, { recursive: true, force: true }); });
  assert.ok(runtime.models);
  return { directory, runtime, models: runtime.models };
}
async function waitForCatalog(directory: string) {
  const deadline = performance.now() + 4000;
  while (!existsSync(join(directory, 'catalog-output')) && performance.now() < deadline) await delay(5);
  assert.ok(existsSync(join(directory, 'catalog-output')));
}

test('canceling after catalog output but before worker exit cannot return ready models', async t => {
  const h = await rawProcess(t, 'models-result-then-cancel'), controller = new AbortController();
  const result = h.models(controller.signal);
  await waitForCatalog(h.directory); await delay(50);
  controller.abort();
  const returned = await result;
  assert.notEqual(returned.state, 'ready'); assert.deepEqual(returned.models, []);
  assert.equal(h.runtime.lastExit?.groupAbsent, true);
});

test('disposing after catalog output also cancels that discovery without an external abort', async t => {
  const h = await rawProcess(t, 'models-result-then-cancel');
  const result = h.models(new AbortController().signal);
  await waitForCatalog(h.directory); await delay(50);
  await h.runtime.dispose();
  const returned = await result;
  assert.notEqual(returned.state, 'ready'); assert.deepEqual(returned.models, []);
  assert.equal(h.runtime.lastExit?.groupAbsent, true); assert.equal(h.runtime.readiness().state, 'stopped');
});

test('the parent refuses foreign, malformed, oversized, partial and nonzero-exit catalog evidence', async t => {
  const cases = ['foreign-provider', 'foreign-request', 'foreign-digest', 'duplicates', 'empty-id', 'invalid-state', 'bad-date',
    'extra-field', 'too-many', 'big-name', 'invalid-name', 'oversized-frame', 'no-terminal', 'invalid-utf8', 'trailing', 'nonzero'];
  for (const scenario of cases) await t.test(scenario, async t => {
    const h = await rawProcess(t, 'models-' + scenario), result = await h.models(new AbortController().signal);
    assert.notEqual(result.state, 'ready'); assert.deepEqual(result.models, []); assert.equal(result.provider, binding.provider);
    assert.equal(h.runtime.lastExit?.groupAbsent, true);
    const pid = h.runtime.lastExit?.pid; assert.ok(pid);
    assert.throws(() => process.kill(pid, 0), { code: 'ESRCH' });
    assert.throws(() => process.kill(-pid, 0), { code: 'ESRCH' });
  });
});

test('busy, stopped and already canceled discovery never launches another worker', async t => {
  const h = await rawProcess(t, 'models-await-cancel'), controller = new AbortController();
  assert.equal((await h.models(AbortSignal.abort())).state, 'unavailable');
  const initialExit = h.runtime.lastExit; assert.equal(initialExit, undefined);
  const result = h.models(controller.signal); await waitForCatalog(h.directory);
  assert.equal(h.runtime.readiness().state, 'busy');
  assert.equal((await h.models(new AbortController().signal)).state, 'unavailable');
  const busyExit = h.runtime.lastExit; assert.equal(busyExit, undefined);
  controller.abort(); assert.notEqual((await result).state, 'ready');
  const report = h.runtime.lastExit; assert.equal(report?.groupAbsent, true);
  await h.runtime.dispose();
  assert.equal((await h.models(new AbortController().signal)).state, 'unavailable'); assert.equal(h.runtime.lastExit, report);
});

test('the catalog deadline bounds a worker that ignores cancellation and TERM', async t => {
  const h = await rawProcess(t, 'models-hang'), start = performance.now();
  const result = await h.models(new AbortController().signal);
  assert.notEqual(result.state, 'ready'); assert.deepEqual(result.models, []);
  assert.equal(h.runtime.lastExit?.forced, true); assert.equal(h.runtime.lastExit?.groupAbsent, true);
  assert.ok(performance.now() - start < 7500);
  const pid = h.runtime.lastExit?.pid; assert.ok(pid);
  assert.throws(() => process.kill(-pid, 0), { code: 'ESRCH' });
});

test('discovery requires an explicit immutable binding and is absent from ordinary executors', async () => {
  const options = { executable: process.execPath, args: [], cwd: process.cwd(), env: {}, lifecycle: { kind: 'processGroup' } } as const;
  const ordinary = new ExecutorProcess({ ...options, args: [] });
  assert.equal(ordinary.models, undefined); assert.equal(ordinary.lastExit, undefined); await ordinary.dispose();
  for (const key of ['provider', 'runtimeVersion', 'hostId']) {
    const invalid = { ...binding }; Reflect.set(invalid, key, '');
    assert.throws(() => new ExecutorProcess({ ...options, args: [], discovery: invalid }));
  }
});

test('unconfirmed group absence rejects the catalog and blocks future operations', async t => {
  const directory = await realpath(await mkdtemp(join(tmpdir(), 'fort-model-absence-')));
  const worker = fileURLToPath(new URL('./fixtures/owned-executor.mjs', import.meta.url));
  const runtime = new ExecutorProcess({ executable: process.execPath, args: [worker, 'models'], cwd: directory, env: {},
    lifecycle: { kind: 'processGroup' }, discovery: binding });
  const realKill = process.kill.bind(process);
  const mock = t.mock.method(process, 'kill', (pid: number, signal?: number | NodeJS.Signals) => {
    if (pid < 0) throw Object.assign(new Error('Synthetic process access denied'), { code: 'EPERM' });
    return realKill(pid, signal);
  });
  t.after(async () => { mock.mock.restore(); await runtime.dispose().catch(() => {}); await rm(directory, { recursive: true, force: true }); });
  assert.ok(runtime.models);
  const result = await runtime.models(new AbortController().signal);
  assert.equal(result.state, 'error'); assert.deepEqual(result.models, []);
  assert.equal(runtime.readiness().state, 'unavailable'); assert.equal(runtime.lastExit?.groupAbsent, false);
  const report = runtime.lastExit;
  assert.equal((await runtime.models(new AbortController().signal)).state, 'unavailable'); assert.equal(runtime.lastExit, report);
  await assert.rejects(runtime.dispose(), /cleanup was not confirmed/);
  mock.mock.restore();
  const pid = report?.pid; assert.ok(pid);
  assert.throws(() => process.kill(pid, 0), { code: 'ESRCH' });
  assert.throws(() => process.kill(-pid, 0), { code: 'ESRCH' });
});
