import { before, after, test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, realpathSync, rmSync, readFileSync, existsSync, writeFileSync } from 'node:fs';
import { setTimeout as delay } from 'node:timers/promises';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ExecutorProcess } from './executor-process.ts';
import { createCodexExecutorProcess } from './codex-executor.ts';
import { EXECUTION_LIMITS, TEXT_POLICY, executionDigest, sealDispatch } from '../contracts/executor.ts';
import type { ChatInput } from './chat-runtime.ts';

const root = realpathSync(mkdtempSync('/tmp/fort059-codex-executor-')), guardian = join(root, 'guardian');
const worker = fileURLToPath(new URL('./fixtures/codex-executor-worker.mjs', import.meta.url));
before(() => {
  if (process.platform !== 'darwin') return;
  const built = spawnSync('/usr/bin/xcrun', ['swiftc', fileURLToPath(new URL('../host/mac-process-guardian.swift', import.meta.url)), '-o', guardian], { encoding: 'utf8', timeout: 60_000 });
  assert.equal(built.status, 0, built.stderr);
});
after(() => rmSync(root, { recursive: true, force: true }));
function command() {
  const input: ChatInput = { context: { kind: 'empty' }, attempt: { runId: 'run', attemptId: 'attempt', generation: 1, bindingRevisionId: 'binding', runtimeVersion: '0.157.1', selectedModel: 'fixture-exact-model' },
    runId: 'run', requestId: 'request', conversationId: 'conversation', configuration: { agentId: 'agent', agentName: 'Fixture', bindingRevisionId: 'binding', selectedModel: 'fixture-exact-model', instructions: '', skills: [] },
    history: [], text: '  synthetic exact cedar 🌲\n' };
  const dispatch = sealDispatch({ protocolVersion: 1, installationId: 'installation', recoveryEpoch: 'epoch',
    binding: { bindingRevisionId: 'binding', provider: 'codex', runtimeVersion: '0.157.1', selectedModel: 'fixture-exact-model', hostId: 'fixture-host', enabled: true }, attempt: input.attempt,
    inputDigest: executionDigest(input), configurationDigest: executionDigest(input.configuration), historyDigest: executionDigest(input.history),
    policy: TEXT_POLICY, policyDigest: executionDigest(TEXT_POLICY), deadline: Date.now() + 10_000, limits: EXECUTION_LIMITS });
  return { input, dispatch };
}
function setup(t: TestContext, scenario: string, legacy = false) {
  const directory = mkdtempSync(join(root, 'journey-')); mkdirSync(join(directory, 'workspace'), { mode: 0o700 });
  const options = { executable: process.execPath, args: [worker, directory, guardian, scenario], cwd: directory, env: {} };
  const runtime = legacy ? new ExecutorProcess({ ...options, lifecycle: { kind: 'macParentExit', guardian } }) : createCodexExecutorProcess({ ...options, guardian, hostId: 'fixture-host' });
  t.after(() => runtime.dispose());
  return { directory, runtime };
}
function processIDs(path: string): number[] {
  const value: unknown = JSON.parse(readFileSync(path, 'utf8'));
  assert.ok(value && typeof value === 'object' && !Array.isArray(value));
  return Object.values(value).map((pid: unknown) => { assert.ok(typeof pid === 'number' && Number.isSafeInteger(pid) && pid > 1); return pid; });
}
function proveAbsent(pids: number[]) {
  for (const pid of pids) assert.throws(() => process.kill(pid, 0), { code: 'ESRCH' });
}
function absent(pid: number) {
  try { process.kill(pid, 0); return false; }
  catch (error) { return error instanceof Error && 'code' in error && error.code === 'ESRCH'; }
}
async function until(predicate: () => boolean, milliseconds: number) {
  const deadline = performance.now() + milliseconds;
  while (!predicate() && performance.now() < deadline) await delay(10);
  return predicate();
}
function allAbsent(directory: string) {
  const outer = processIDs(join(directory, 'worker.json')), inner = processIDs(join(directory, 'runtime.json'));
  const outerGroup = outer[1], innerGroup = inner[1]; assert.ok(outerGroup && innerGroup);
  proveAbsent([...outer, ...inner, -outerGroup, -innerGroup]);
  return { outer, inner };
}

test('the guarded Codex adapter completes through the real worker protocol and leaves both groups absent', { skip: process.platform !== 'darwin' }, async t => {
  const h = setup(t, 'echo'), { input, dispatch } = command();
  const result = await h.runtime.execute(input, new AbortController().signal, dispatch);
  assert.deepEqual(result, { kind: 'completed', text: input.text, effectiveModel: null });
  await h.runtime.dispose();
  assert.equal(h.runtime.lastExit?.forced, false); assert.equal(h.runtime.lastExit?.groupAbsent, true);
  t.diagnostic(JSON.stringify(allAbsent(h.directory)));
});

test('guarded Codex model discovery returns exact models without a thread or turn and leaves both groups absent', { skip: process.platform !== 'darwin' }, async t => {
  const h = setup(t, 'catalog');
  assert.ok(h.runtime.models);
  const catalog = await h.runtime.models(new AbortController().signal);
  assert.equal(catalog.provider, 'codex'); assert.equal(catalog.state, 'ready');
  assert.deepEqual(catalog.models, [
    { id: 'fixture-exact-model', name: 'Text fixture', availability: 'available' },
    { id: 'fixture-audio-model', name: 'Audio fixture', availability: 'unavailable', detail: 'This model does not list text input support.' },
  ]);
  const methods = readFileSync(join(h.directory, 'methods.jsonl'), 'utf8').trim().split('\n').map(line => JSON.parse(line));
  assert.ok(methods.some(value => value.method === 'model/list'));
  assert.ok(methods.every(value => value.method !== 'thread/start' && value.method !== 'turn/start'));
  assert.deepEqual(methods.filter(value => value.method === 'account/read'), [{ method: 'account/read', refreshToken: false }]);
  assert.equal(h.runtime.lastExit?.forced, false); assert.equal(h.runtime.lastExit?.groupAbsent, true);
  t.diagnostic(JSON.stringify(allAbsent(h.directory)));
});

test('guarded model discovery refuses unsafe configuration and a foreign account before listing models', { skip: process.platform !== 'darwin' }, async t => {
  for (const scenario of ['unsafe-config', 'foreign-account']) await t.test(scenario, async t => {
    const h = setup(t, scenario); assert.ok(h.runtime.models);
    const catalog = await h.runtime.models(new AbortController().signal);
    assert.equal(catalog.state, 'error'); assert.deepEqual(catalog.models, []);
    const methods = readFileSync(join(h.directory, 'methods.jsonl'), 'utf8').trim().split('\n').map(line => JSON.parse(line));
    assert.ok(methods.every(value => !['model/list', 'thread/start', 'turn/start', 'account/login/start'].includes(value.method)));
    if (scenario === 'unsafe-config') assert.ok(methods.every(value => value.method !== 'account/read'));
    else assert.deepEqual(methods.filter(value => value.method === 'account/read'), [{ method: 'account/read', refreshToken: false }]);
    assert.equal(h.runtime.lastExit?.forced, false); allAbsent(h.directory);
  });
});

test('guarded account sign-in forwards a validated challenge without a chat turn and proves both groups absent', { skip: process.platform !== 'darwin' }, async t => {
  const h = setup(t, 'account-success'), shown: unknown[] = [];
  assert.ok(h.runtime.signIn);
  const result = await h.runtime.signIn({ durationMs: 5000, onChallenge: value => { shown.push(value); } }, new AbortController().signal);
  assert.deepEqual(result, { kind: 'signedIn' });
  assert.deepEqual(shown, [{ kind: 'deviceCode', verificationURL: 'https://auth.openai.com/codex/device', userCode: 'FORT-TEST' }]);
  const methods = readFileSync(join(h.directory, 'methods.jsonl'), 'utf8').trim().split('\n').map(line => JSON.parse(line));
  assert.deepEqual(methods.filter(value => value.method === 'account/login/start'), [{ method: 'account/login/start', loginType: 'chatgptDeviceCode' }]);
  assert.equal(methods.some(value => value.method === 'thread/start' || value.method === 'turn/start'), false);
  assert.equal(h.runtime.lastExit?.forced, false); assert.equal(h.runtime.lastExit?.groupAbsent, true);
  allAbsent(h.directory);
});

test('guarded account cancellation retains a completed account race and disposes every process', { skip: process.platform !== 'darwin' }, async t => {
  for (const scenario of ['account-cancel', 'account-race']) await t.test(scenario, async t => {
    const h = setup(t, scenario), abort = new AbortController();
    assert.ok(h.runtime.signIn);
    const result = await h.runtime.signIn({ durationMs: 5000, onChallenge: () => abort.abort() }, abort.signal);
    assert.deepEqual(result, { kind: scenario === 'account-race' ? 'signedIn' : 'canceled' });
    assert.equal(h.runtime.lastExit?.forced, false); assert.equal(h.runtime.lastExit?.groupAbsent, true);
    allAbsent(h.directory);
  });
});

test('canceling guarded model discovery disposes its runtime and leaves both groups absent', { skip: process.platform !== 'darwin' }, async t => {
  const h = setup(t, 'catalog-cancel'), controller = new AbortController(); assert.ok(h.runtime.models);
  const pending = h.runtime.models(controller.signal);
  assert.ok(await until(() => existsSync(join(h.directory, 'model-list-started')), 4000));
  controller.abort(); const result = await pending;
  assert.notEqual(result.state, 'ready'); assert.deepEqual(result.models, []);
  assert.equal(h.runtime.lastExit?.groupAbsent, true); allAbsent(h.directory);
  const methods = readFileSync(join(h.directory, 'methods.jsonl'), 'utf8').trim().split('\n').map(line => JSON.parse(line));
  assert.ok(methods.every(value => value.method !== 'thread/start' && value.method !== 'turn/start'));
});

test('guarded execution and model discovery share one active worker', { skip: process.platform !== 'darwin' }, async t => {
  const h = setup(t, 'delayed-cancel'), { input, dispatch } = command(), controller = new AbortController();
  const executing = h.runtime.execute(input, controller.signal, dispatch);
  assert.ok(await until(() => existsSync(join(h.directory, 'turn-started')), 4000));
  const before = readFileSync(join(h.directory, 'worker.json'), 'utf8'); assert.ok(h.runtime.models);
  assert.equal((await h.runtime.models(new AbortController().signal)).state, 'unavailable');
  assert.equal(readFileSync(join(h.directory, 'worker.json'), 'utf8'), before);
  controller.abort(); assert.equal((await executing).kind, 'canceled'); allAbsent(h.directory);
});

test('the exact Codex interruption can finish before the parent forces cleanup', { skip: process.platform !== 'darwin' }, async t => {
  const h = setup(t, 'delayed-cancel'), { input, dispatch } = command(), controller = new AbortController();
  const executing = h.runtime.execute(input, controller.signal, dispatch);
  assert.ok(await until(() => existsSync(join(h.directory, 'turn-started')), 5000));
  const started = performance.now(); controller.abort();
  const result = await executing;
  t.diagnostic(JSON.stringify({ result: result.kind, exit: h.runtime.lastExit, cancellationMs: performance.now() - started }));
  assert.equal(result.kind, 'canceled'); assert.equal(h.runtime.lastExit?.forced, false);
  assert.ok(performance.now() - started < 4500); allAbsent(h.directory);
});

test('unconfirmed Codex interruption remains unknown after bounded child cleanup', { skip: process.platform !== 'darwin' }, async t => {
  const h = setup(t, 'unconfirmed-cancel'), { input, dispatch } = command(), controller = new AbortController();
  const executing = h.runtime.execute(input, controller.signal, dispatch);
  assert.ok(await until(() => existsSync(join(h.directory, 'turn-started')), 5000));
  const started = performance.now(); controller.abort();
  const result = await executing;
  assert.equal(result.kind, 'outcomeUnknown'); assert.equal(h.runtime.lastExit?.forced, false);
  assert.ok(performance.now() - started < 4500); allAbsent(h.directory);
  t.diagnostic(JSON.stringify({ result: result.kind, cleanupMs: performance.now() - started }));
});

test('unsafe runtime configuration and missing exact models refuse turns and leave both groups absent', { skip: process.platform !== 'darwin' }, async t => {
  for (const scenario of ['unsafe-config', 'missing-model']) await t.test(scenario, async t => {
    const h = setup(t, scenario), { input, dispatch } = command();
    const result = await h.runtime.execute(input, new AbortController().signal, dispatch);
    assert.equal(result.kind, 'notStarted'); assert.equal(h.runtime.lastExit?.forced, false);
    const methods = readFileSync(join(h.directory, 'methods.jsonl'), 'utf8').trim().split('\n').map(line => JSON.parse(line));
    assert.ok(methods.every(value => value.method !== 'turn/start' && value.method !== 'thread/start'));
    if (scenario === 'unsafe-config') assert.ok(methods.every(value => value.method !== 'account/read'));
    allAbsent(h.directory);
  });
});

test('a nonempty workspace is refused before any runtime child starts', { skip: process.platform !== 'darwin' }, async t => {
  const h = setup(t, 'echo'), { input, dispatch } = command();
  writeFileSync(join(h.directory, 'workspace', 'unrelated.txt'), 'synthetic existing contents');
  const result = await h.runtime.execute(input, new AbortController().signal, dispatch);
  assert.equal(result.kind, 'outcomeUnknown');
  assert.equal(existsSync(join(h.directory, 'runtime.json')), false);
  assert.equal(readFileSync(join(h.directory, 'workspace', 'unrelated.txt'), 'utf8'), 'synthetic existing contents');
});

test('the historical cancellation allowance still forces an uncertain result without leaking child groups', { skip: process.platform !== 'darwin' }, async t => {
  const h = setup(t, 'delayed-cancel', true), { input, dispatch } = command(), controller = new AbortController();
  const executing = h.runtime.execute(input, controller.signal, dispatch);
  assert.ok(await until(() => existsSync(join(h.directory, 'turn-started')), 5000));
  controller.abort(); const result = await executing;
  assert.equal(result.kind, 'outcomeUnknown'); assert.equal(h.runtime.lastExit?.forced, true);
  const inner = processIDs(join(h.directory, 'runtime.json'));
  assert.ok(await until(() => inner.every(absent), 3000)); allAbsent(h.directory);
});

test('abrupt launcher death removes the outer worker and separate guarded runtime chain', { skip: process.platform !== 'darwin', timeout: 12000 }, async t => {
  const directory = mkdtempSync(join(root, 'parent-death-')); mkdirSync(join(directory, 'workspace'), { mode: 0o700 });
  writeFileSync(join(directory, 'command.json'), JSON.stringify(command()), { mode: 0o600 });
  const host = spawn(process.execPath, [fileURLToPath(new URL('./fixtures/codex-executor-host.mjs', import.meta.url)), directory, guardian], { cwd: directory, env: {}, detached: true, stdio: 'ignore' });
  const hostPID = host.pid; assert.ok(hostPID && hostPID > 1);
  let targets = [hostPID, -hostPID], complete = false;
  t.after(async () => {
    if (!complete) {
      if (host.exitCode === null && host.signalCode === null) host.kill('SIGKILL');
      for (const pid of targets) if (!absent(pid)) { try { process.kill(pid, 'SIGKILL'); } catch {} }
      assert.ok(await until(() => targets.every(absent), 2000), 'Synthetic nested process cleanup is unconfirmed');
      t.diagnostic(JSON.stringify({ forcedFixtureCleanup: true, targets }));
    }
  });
  assert.ok(await until(() => existsSync(join(directory, 'turn-started')), 5000));
  const outer = processIDs(join(directory, 'worker.json')), inner = processIDs(join(directory, 'runtime.json'));
  const outerGroup = outer[1], innerGroup = inner[1]; assert.ok(outerGroup && innerGroup);
  targets = [...targets, ...outer, ...inner, -outerGroup, -innerGroup];
  assert.ok(targets.every(pid => !absent(pid)));
  const started = performance.now(); host.kill('SIGKILL');
  assert.ok(await until(() => targets.every(absent), 3000), 'A process survived nested launcher death');
  complete = true; t.diagnostic(JSON.stringify({ targets, cleanupMs: performance.now() - started }));
});

test('the full synthetic Codex executor journey meets its isolated budget', { skip: process.platform !== 'darwin' || !process.env.FORT_CODEX_EXECUTOR_BENCHMARK }, async t => {
  const destination = process.env.FORT_CODEX_EXECUTOR_BENCHMARK; assert.ok(destination);
  const samples: number[] = [];
  for (let index = 0; index < 20; index++) {
    const h = setup(t, 'echo'), { input, dispatch } = command(), started = performance.now();
    const result = await h.runtime.execute(input, new AbortController().signal, dispatch);
    await h.runtime.dispose(); assert.deepEqual(result, { kind: 'completed', text: input.text, effectiveModel: null });
    assert.equal(h.runtime.lastExit?.forced, false); allAbsent(h.directory);
    samples.push(performance.now() - started);
  }
  writeFileSync(destination, JSON.stringify(samples));
  const sorted = [...samples].sort((a, b) => a - b), p95 = sorted[18]; assert.ok(p95 !== undefined && p95 <= 2000);
});

test('the full synthetic Codex catalog journey meets its isolated budget', { skip: process.platform !== 'darwin' || !process.env.FORT_CODEX_CATALOG_BENCHMARK }, async t => {
  const destination = process.env.FORT_CODEX_CATALOG_BENCHMARK; assert.ok(destination);
  const samples: number[] = [];
  for (let index = 0; index < 20; index++) {
    const h = setup(t, 'catalog'), started = performance.now(); assert.ok(h.runtime.models);
    const result = await h.runtime.models(new AbortController().signal);
    await h.runtime.dispose(); assert.equal(result.state, 'ready'); assert.equal(result.models.length, 2);
    assert.equal(h.runtime.lastExit?.forced, false); allAbsent(h.directory);
    samples.push(performance.now() - started);
  }
  writeFileSync(destination, JSON.stringify(samples));
  const sorted = [...samples].sort((a, b) => a - b), p95 = sorted[18]; assert.ok(p95 !== undefined && p95 <= 2000);
});
