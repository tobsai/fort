import assert from 'node:assert/strict';
import test, { type TestContext } from 'node:test';
import { mkdtempSync, realpathSync, rmSync, readFileSync, existsSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ExecutorProcess } from './executor-process.ts';
import { performance } from 'node:perf_hooks';
import { PassThrough } from 'node:stream';
import { serveExecutorWorker } from './executor-worker.ts';
import { executionDigest } from '../contracts/executor.ts';

const binding = { provider: 'independent-device-fixture', runtimeVersion: 'fixture-7', hostId: 'account-fixture-host' };
function setup(t: TestContext, scenario = 'complete') {
  const directory = realpathSync(mkdtempSync('/tmp/fort059-account-process-'));
  const runtime = new ExecutorProcess({ executable: process.execPath,
    args: [fileURLToPath(new URL('./fixtures/executor-account-worker.mjs', import.meta.url)), scenario],
    cwd: directory, env: {}, lifecycle: { kind: 'processGroup' }, discovery: binding, signIn: binding });
  t.after(async () => { await runtime.dispose(); rmSync(directory, { recursive: true, force: true }); });
  return { runtime, directory };
}

test('owned executor presents an independent provider challenge and confirms cleanup before success', async t => {
  const h = setup(t), shown: unknown[] = [];
  assert.equal(Boolean(h.runtime.lastExit), false);
  assert.ok(h.runtime.signIn, 'The explicit account capability must be available');
  const result = await h.runtime.signIn({ durationMs: 1000, onChallenge: value => { shown.push(value); } }, new AbortController().signal);
  assert.deepEqual(result, { kind: 'signedIn' });
  assert.deepEqual(shown, [{ kind: 'deviceCode', verificationURL: 'https://device.fixture.invalid/verify', userCode: 'FORT-TEST' }]);
  assert.equal(readFileSync(join(h.directory, 'disposed'), 'utf8'), 'synthetic cleanup confirmed');
  assert.equal(h.runtime.lastExit?.forced, false); assert.equal(h.runtime.lastExit?.groupAbsent, true);
  const pid: unknown = JSON.parse(readFileSync(join(h.directory, 'identity.json'), 'utf8')).pid;
  assert.ok(typeof pid === 'number' && Number.isSafeInteger(pid) && pid > 1);
  assert.throws(() => process.kill(pid, 0), { code: 'ESRCH' });
  assert.throws(() => process.kill(-pid, 0), { code: 'ESRCH' });
});

test('account work deadline cancels before the cleanup reserve is consumed', async t => {
  const h = setup(t, 'pending'); assert.ok(h.runtime.signIn);
  const before = performance.now();
  assert.deepEqual(await h.runtime.signIn({ durationMs: 1000, onChallenge: () => {} }, new AbortController().signal), { kind: 'canceled' });
  assert.ok(performance.now() - before < 3000);
  assert.equal(h.runtime.lastExit?.forced, false); assert.equal(h.runtime.lastExit?.groupAbsent, true);
});

test('account sign-in owns the exclusive executor slot and explicit cancellation releases it', async t => {
  const h = setup(t, 'pending'), shown = Promise.withResolvers<void>(), abort = new AbortController();
  assert.ok(h.runtime.signIn); assert.ok(h.runtime.models);
  const pending = h.runtime.signIn({ durationMs: 5000, onChallenge: () => { shown.resolve(); } }, abort.signal);
  await shown.promise;
  assert.equal(h.runtime.readiness().state, 'busy');
  assert.deepEqual(await h.runtime.signIn({ durationMs: 1000, onChallenge: () => assert.fail('No second challenge') }, new AbortController().signal), { kind: 'unconfirmed' });
  assert.equal((await h.runtime.models(new AbortController().signal)).state, 'unavailable');
  abort.abort();
  assert.deepEqual(await pending, { kind: 'canceled' });
  assert.equal(h.runtime.readiness().state, 'ready');
  assert.equal(h.runtime.lastExit?.groupAbsent, true);
});

test('disposing the executor cancels account work before reporting the runtime stopped', async t => {
  const h = setup(t, 'pending'), shown = Promise.withResolvers<void>(); assert.ok(h.runtime.signIn);
  const pending = h.runtime.signIn({ durationMs: 5000, onChallenge: () => { shown.resolve(); } }, new AbortController().signal);
  await shown.promise; await h.runtime.dispose();
  assert.deepEqual(await pending, { kind: 'canceled' });
  assert.equal(h.runtime.readiness().state, 'stopped'); assert.equal(h.runtime.lastExit?.groupAbsent, true);
});

test('an existing account needs no challenge, while invalid budgets and cancellation create no process', async t => {
  const existing = setup(t, 'existing'); assert.ok(existing.runtime.signIn);
  assert.deepEqual(await existing.runtime.signIn({ durationMs: 1000, onChallenge: () => assert.fail('No challenge') }, new AbortController().signal), { kind: 'signedIn' });
  for (const durationMs of [0, -1, 115_001, Number.NaN]) {
    const h = setup(t); assert.ok(h.runtime.signIn);
    assert.deepEqual(await h.runtime.signIn({ durationMs, onChallenge: () => assert.fail('No challenge') }, new AbortController().signal), { kind: 'unconfirmed' });
    assert.equal(existsSync(join(h.directory, 'identity.json')), false);
  }
  const aborted = setup(t); assert.ok(aborted.runtime.signIn);
  assert.deepEqual(await aborted.runtime.signIn({ durationMs: 1000, onChallenge: () => assert.fail('No challenge') }, AbortSignal.abort()), { kind: 'unconfirmed' });
  assert.equal(existsSync(join(aborted.directory, 'identity.json')), false);
});

for (const scenario of ['wire-foreign', 'wire-digest', 'wire-sequence', 'wire-changed', 'wire-reordered', 'wire-oversized',
  'wire-insecure', 'wire-credential-url', 'wire-secret-field', 'wire-conflicting-result', 'wire-flood', 'wire-no-terminal', 'wire-nonzero', 'cleanup-fails']) {
  test('invalid executor account evidence cannot establish success: ' + scenario, async t => {
    const h = setup(t, scenario), shown: unknown[] = []; assert.ok(h.runtime.signIn);
    assert.deepEqual(await h.runtime.signIn({ durationMs: 1000, onChallenge: value => { shown.push(value); } }, new AbortController().signal), { kind: 'unconfirmed' });
    assert.ok(shown.length <= 1); assert.equal(h.runtime.lastExit?.groupAbsent, true);
    if (['wire-foreign', 'wire-digest', 'wire-sequence', 'wire-reordered', 'wire-oversized', 'wire-insecure', 'wire-credential-url'].includes(scenario)) assert.equal(shown.length, 0);
    assert.doesNotMatch(JSON.stringify(shown), /synthetic-private/);
  });
}

test('identical progress is presented once and callback failure terminates the owned process', async t => {
  const duplicate = setup(t, 'wire-duplicate'), seen: unknown[] = []; assert.ok(duplicate.runtime.signIn);
  assert.deepEqual(await duplicate.runtime.signIn({ durationMs: 1000, onChallenge: value => { seen.push(value); } }, new AbortController().signal), { kind: 'signedIn' });
  assert.equal(seen.length, 1);
  const failure = setup(t, 'pending'); assert.ok(failure.runtime.signIn);
  assert.deepEqual(await failure.runtime.signIn({ durationMs: 1000, onChallenge: () => { throw new Error('Synthetic presentation failed'); } }, new AbortController().signal), { kind: 'unconfirmed' });
  assert.equal(failure.runtime.lastExit?.groupAbsent, true);
});

test('the original challenge destination is captured when the operation starts', async t => {
  const h = setup(t); assert.ok(h.runtime.signIn); let shown = 0;
  const options = { durationMs: 1000, onChallenge: () => { shown++; } };
  const pending = h.runtime.signIn(options, new AbortController().signal);
  options.onChallenge = () => { throw new Error('Changed callback must not receive the code'); };
  assert.deepEqual(await pending, { kind: 'signedIn' }); assert.equal(shown, 1);
});

test('progress arriving after cancellation is not presented', async t => {
  const h = setup(t, 'wire-duplicate'), abort = new AbortController(); assert.ok(h.runtime.signIn);
  const pending = h.runtime.signIn({ durationMs: 1000, onChallenge: () => assert.fail('Canceled challenge must not be shown') }, abort.signal);
  abort.abort();
  assert.deepEqual(await pending, { kind: 'signedIn' });
  assert.equal(h.runtime.lastExit?.groupAbsent, true);
});

test('forced account cancellation stays unconfirmed after owned group absence is verified', async t => {
  const h = setup(t, 'wire-hang'), abort = new AbortController(); assert.ok(h.runtime.signIn);
  assert.deepEqual(await h.runtime.signIn({ durationMs: 1000, onChallenge: () => abort.abort() }, abort.signal), { kind: 'unconfirmed' });
  assert.equal(h.runtime.lastExit?.forced, true); assert.equal(h.runtime.lastExit?.groupAbsent, true);
});

test('worker validates account scope before creating any runtime', async () => {
  const now = Date.now();
  const body = { protocolVersion: 1, kind: 'accountSignIn', requestId: 'fixture-request', binding,
    workDeadline: now + 1000, deadline: now + 6000 };
  for (const changed of [
    { protocolVersion: 2 }, { requestId: '' }, { binding: { ...binding, provider: 'foreign' } },
    { binding: { ...binding, runtimeVersion: 'foreign' } }, { binding: { ...binding, hostId: 'foreign' } },
    { workDeadline: now - 1, deadline: now + 4999 }, { workDeadline: now + 125_000, deadline: now + 130_000 },
    { deadline: now + 1000 }, { workDeadline: 'later' }, { token: 'synthetic-private' }, { requestDigest: '0'.repeat(64) },
  ]) {
    const value = { ...body, ...changed }, request = { ...value, requestDigest: 'requestDigest' in changed ? changed.requestDigest : executionDigest(value) };
    const input = new PassThrough(), output = new PassThrough(); let created = 0;
    const create = () => { created++; throw new Error('Invalid request reached runtime'); };
    const serving = serveExecutorWorker({ input, output, binding, createRuntime: create, createSignIn: create });
    input.write(JSON.stringify(request) + '\n');
    await assert.rejects(serving); assert.equal(created, 0);
  }
});

test('account process journeys meet their isolated budget', { skip: !process.env.FORT_OWNED_ACCOUNT_BENCHMARK }, async t => {
  const samples: number[] = [];
  for (let i = 0; i < 25; i++) {
    const cancel = i % 2 === 1, h = setup(t, cancel ? 'pending' : 'complete'), abort = new AbortController();
    assert.ok(h.runtime.signIn); const before = performance.now();
    assert.deepEqual(await h.runtime.signIn({ durationMs: 5000, onChallenge: () => { if (cancel) abort.abort(); } }, abort.signal), { kind: cancel ? 'canceled' : 'signedIn' });
    await h.runtime.dispose(); const elapsed = performance.now() - before;
    if (i >= 5) samples.push(elapsed);
    assert.equal(h.runtime.lastExit?.forced, false); assert.equal(h.runtime.lastExit?.groupAbsent, true);
  }
  samples.sort((a, b) => a - b);
  const result = { samples, p50: samples[9], p95: samples[18], max: samples[19], budgetMs: 2000, warmups: 5 };
  assert.ok(result.p95 !== undefined && result.p95 < result.budgetMs);
  writeFileSync(process.env.FORT_OWNED_ACCOUNT_BENCHMARK!, JSON.stringify(result) + '\n', { flag: 'wx', mode: 0o600 });
});
