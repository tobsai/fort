import { test } from 'node:test';
import assert from 'node:assert/strict';
import { PassThrough } from 'node:stream';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';
import { writeFileSync } from 'node:fs';
import { EXECUTION_LIMITS, TEXT_POLICY, executionDigest, sealDispatch } from '../contracts/executor.ts';
import type { ChatInput, ChatRuntime } from './chat-runtime.ts';
import { serveExecutorWorker } from './executor-worker.ts';
import { ExecutorProcess } from './executor-process.ts';
import { parseSkillBundle } from '../contracts/skill-bundle.ts';

const binding = { provider: 'worker-fixture', runtimeVersion: 'fixture-1', hostId: 'fixture-host' };
function request(text = '  exact cedar 🌲\n') {
  const input: ChatInput = { context: { kind: 'empty' },
    attempt: { runId: 'run', attemptId: 'attempt', generation: 1, bindingRevisionId: 'binding', runtimeVersion: binding.runtimeVersion, selectedModel: 'exact-model' },
    runId: 'run', requestId: 'request', conversationId: 'conversation',
    configuration: { agentId: 'agent', agentName: 'Fixture', bindingRevisionId: 'binding', selectedModel: 'exact-model', instructions: '', skills: [] }, history: [], text };
  const dispatch = sealDispatch({ protocolVersion: 1, installationId: 'installation', recoveryEpoch: 'epoch',
    binding: { ...binding, bindingRevisionId: 'binding', selectedModel: 'exact-model', enabled: true }, attempt: input.attempt,
    inputDigest: executionDigest(input), configurationDigest: executionDigest(input.configuration), historyDigest: executionDigest(input.history),
    policy: TEXT_POLICY, policyDigest: executionDigest(TEXT_POLICY), deadline: Date.now() + 5000, limits: EXECUTION_LIMITS });
  return { protocolVersion: 1, kind: 'dispatch', input, dispatch };
}

test('the owned worker preserves exact text and waits for runtime disposal before its terminal', async () => {
  const input = new PassThrough(), output = new PassThrough(), chunks: Buffer[] = [];
  output.on('data', (chunk: Buffer) => chunks.push(chunk));
  const disposing = Promise.withResolvers<void>(), release = Promise.withResolvers<void>();
  let calls = 0;
  const runtime: ChatRuntime = {
    async execute(value) { calls++; return { kind: 'completed', text: value.text, effectiveModel: null }; },
    async dispose() { disposing.resolve(); await release.promise; },
  };
  const done = serveExecutorWorker({ input, output, binding, createRuntime: () => runtime });
  const command = request();
  input.write(JSON.stringify(command) + '\n');
  await disposing.promise;
  assert.equal(chunks.length, 0);
  release.resolve(); await done;
  assert.equal(calls, 1);
  assert.deepEqual(JSON.parse(Buffer.concat(chunks).toString()), { protocolVersion: 1, kind: 'terminal', sequence: 1,
    dispatchDigest: command.dispatch.dispatchDigest, attemptId: 'attempt', generation: 1,
    result: { kind: 'completed', text: command.input.text, effectiveModel: null } });
  assert.equal(input.destroyed, true);
  assert.equal(output.writableFinished, true);
});

function streams(createRuntime: Parameters<typeof serveExecutorWorker>[0]['createRuntime']) {
  const input = new PassThrough(), output = new PassThrough(), chunks: Buffer[] = [];
  output.on('data', (chunk: Buffer) => chunks.push(chunk));
  const done = serveExecutorWorker({ input, output, binding, createRuntime });
  void done.catch(() => {});
  return { input, output, done, bytes: () => Buffer.concat(chunks) };
}

function resealedInput(input: object, changes: Record<string, unknown> = {}) {
  const good = request(), { dispatchDigest: _digest, ...prior } = good.dispatch;
  const body = { ...prior, inputDigest: executionDigest(input), configurationDigest: executionDigest(Reflect.get(input, 'configuration')),
    historyDigest: executionDigest(Reflect.get(input, 'history')), ...changes };
  return { ...good, input, dispatch: { ...body, dispatchDigest: executionDigest(body) } };
}

test('recomputed hashes cannot bypass input shape, policy or cross-identity validation', async t => {
  const good = request().input;
  const cases = [
    resealedInput({ ...good, extra: 'unsupported' }), resealedInput({ ...good, requestId: '' }),
    resealedInput({ ...good, text: 'x'.repeat(65537) }), resealedInput({ ...good, text: String.fromCharCode(0xd800) }),
    resealedInput({ ...good, attempt: { ...good.attempt, generation: 0 } }),
    resealedInput({ ...good, configuration: { ...good.configuration, selectedModel: 'foreign' } }),
    resealedInput({ ...good, configuration: { ...good.configuration, skills: [{}] } }),
    resealedInput({ ...good, configuration: { ...good.configuration, memory: { conversationId: 'foreign', revisionId: 'memory', text: 'foreign' } } }),
    resealedInput({ ...good, context: { kind: 'prepared' } }),
    resealedInput({ ...good, context: { kind: 'empty', extra: 'unsupported' } }),
    ...['system', 'user'].map(role => resealedInput({ ...good, history: [{ messageId: 'message', requestId: 'request', conversationId: 'foreign', sequence: 1, createdAt: '2026-10-18', text: 'synthetic', role }] })),
    resealedInput(good, { policy: { kind: 'plainText', grants: ['unapproved'] }, policyDigest: executionDigest({ kind: 'plainText', grants: ['unapproved'] }) }),
    resealedInput(good, { limits: { ...EXECUTION_LIMITS, replyBytes: 1_000_000 } }),
  ];
  for (const [index, value] of cases.entries()) await t.test(String(index), async () => {
    let created = false;
    const h = streams(() => { created = true; throw new Error('must not start'); });
    h.input.write(JSON.stringify(value) + '\n'); await assert.rejects(h.done);
    assert.equal(created, false); assert.equal(h.bytes().length, 0);
  });
});

test('the worker preserves reviewed bundles and refuses incomplete wire manifests', async t => {
  const instructions = 'Synthetic bundle instructions';
  const bundle = parseSkillBundle({ schemaVersion: 1, name: 'Fixture', version: '1', source: 'synthetic', license: 'MIT', entrypoint: 'SKILL.md',
    supportedRuntimes: ['fixture'], requestedTools: [], scripts: [], files: [{ path: 'SKILL.md', kind: 'file', executable: false, contentBase64: Buffer.from(instructions).toString('base64') }] });
  for (const scenario of ['complete', 'missing-bytes', 'missing-digest']) await t.test(scenario, async () => {
    const raw = scenario === 'missing-bytes' ? { ...bundle, files: bundle.files.map(({ bytes: _bytes, ...file }) => file) }
      : scenario === 'missing-digest' ? { ...bundle, digest: undefined } : bundle;
    const original = request().input, configuration = { ...original.configuration, contextModule: { id: 'fixture_context', version: '1' },
      skills: [{ skillId: 'skill', revisionId: 'revision', name: 'Fixture', instructions, bundle: raw, review: { state: 'enabled', digest: bundle.digest } }] };
    const value = { ...original, configuration, context: { kind: 'prepared', module: configuration.contextModule, configurationDigest: executionDigest(configuration),
      content: { kind: 'ready', instructions, bundles: [{ skillId: 'skill', revisionId: 'revision', bundle: raw, textFiles: [{ path: 'SKILL.md', text: instructions }] }] } } };
    let created = false;
    const h = streams(() => { created = true; return { async execute(input) { assert.deepEqual(input, value); return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() {} }; });
    h.input.write(JSON.stringify(resealedInput(value)) + '\n');
    if (scenario === 'complete') { await h.done; assert.equal(created, true); }
    else { await assert.rejects(h.done); assert.equal(created, false); assert.equal(h.bytes().length, 0); }
  });
});

test('an expired active deadline aborts execution and disposes without emitting a terminal', async () => {
  let disposed = false, aborted = false;
  const h = streams(() => ({ async execute(_input, signal) {
    await new Promise<void>(resolve => signal.addEventListener('abort', () => { aborted = true; resolve(); }, { once: true }));
    return { kind: 'canceled', message: 'Synthetic cancellation' };
  }, async dispose() { disposed = true; } }));
  h.input.write(JSON.stringify(resealedInput(request().input, { deadline: Date.now() + 100 })) + '\n');
  await assert.rejects(h.done); assert.equal(disposed, true); assert.equal(aborted, true); assert.equal(h.bytes().length, 0);
});

test('fragmented UTF-8 input retains prepared optional context and exact text', async () => {
  const command = request('  exact\0text 🌲\n');
  command.input.configuration = { ...command.input.configuration, skills: [{ skillId: 'skill', revisionId: 'revision', name: 'Fixture', instructions: 'Synthetic instructions' }],
    contextModule: { id: 'fixture_context', version: '1' }, memory: { conversationId: 'conversation', revisionId: 'memory', text: 'Synthetic memory' } };
  command.input.context = { kind: 'prepared', module: { id: 'fixture_context', version: '1' }, configurationDigest: executionDigest(command.input.configuration),
    content: { kind: 'ready', instructions: 'Synthetic compiled instructions', bundles: [], memory: command.input.configuration.memory } };
  const { dispatchDigest: _oldDigest, ...body } = command.dispatch;
  command.dispatch = sealDispatch({ ...body, inputDigest: executionDigest(command.input), configurationDigest: executionDigest(command.input.configuration) });
  let calls = 0;
  const h = streams(() => ({ async execute(value) { calls++; assert.deepEqual(value, command.input); return { kind: 'completed', text: value.text, effectiveModel: null }; }, async dispose() {} }));
  const bytes = Buffer.from(JSON.stringify(command) + '\n');
  for (let offset = 0; offset < bytes.length; offset += 7) h.input.write(bytes.subarray(offset, offset + 7));
  await h.done;
  assert.equal(calls, 1);
  assert.equal(JSON.parse(h.bytes().toString()).result.text, command.input.text);
});

test('malformed, foreign and unsealed requests are rejected before constructing a runtime', async t => {
  const good = request();
  const { dispatchDigest: _digest, ...body } = good.dispatch;
  const cases: unknown[] = [null, [], {}, { ...good, extra: true }, { ...good, protocolVersion: 2 },
    { ...good, input: { ...good.input, text: 'tampered' } },
    { ...good, dispatch: { ...good.dispatch, inputDigest: '0'.repeat(64) } },
    ...['provider', 'runtimeVersion', 'hostId', 'selectedModel', 'bindingRevisionId'].map(key => ({ ...good, dispatch: sealDispatch({ ...body, binding: { ...good.dispatch.binding, [key]: 'foreign' } }) })),
    { ...good, dispatch: sealDispatch({ ...body, binding: { ...good.dispatch.binding, enabled: false } }) },
    { ...good, dispatch: sealDispatch({ ...body, deadline: Date.now() - 1 }) },
    { ...good, dispatch: sealDispatch({ ...body, deadline: Date.now() + 121_000 }) },
    { ...good, dispatch: sealDispatch({ ...body, attempt: { ...good.dispatch.attempt, generation: 2 } }) },
    { ...good, input: { ...good.input, history: [{ role: 'system', text: 'foreign' }] } },
    { ...good, input: { ...good.input, configuration: { ...good.input.configuration, skills: [{}] } } },
    { ...good, input: { ...good.input, context: { kind: 'prepared' } } },
    { ...good, input: { ...good.input, text: String.fromCharCode(0xd800) } },
  ];
  for (const [index, value] of cases.entries()) await t.test(String(index), async () => {
    let created = false;
    const h = streams(() => { created = true; throw new Error('must not start'); });
    h.input.write(JSON.stringify(value) + '\n');
    await assert.rejects(h.done, /not confirmed/);
    assert.equal(created, false); assert.equal(h.bytes().length, 0);
  });
  for (const bytes of [Buffer.from([255, 10]), Buffer.alloc(1_048_576 + 16_385, 120)]) {
    let created = false;
    const h = streams(() => { created = true; throw new Error('must not start'); });
    h.input.write(bytes);
    await assert.rejects(h.done); assert.equal(created, false); assert.equal(h.bytes().length, 0);
  }
});

test('only an exact cancellation reaches the active attempt', async t => {
  for (const exact of [true, false]) await t.test(String(exact), async () => {
    const command = request(), started = Promise.withResolvers<void>();
    let disposed = 0, aborted = false;
    const h = streams(() => ({
      async execute(_input, signal) {
        started.resolve();
        await new Promise<void>(resolve => signal.addEventListener('abort', () => { aborted = true; resolve(); }, { once: true }));
        return { kind: 'canceled', message: 'Synthetic runtime confirmed cancellation' };
      }, async dispose() { disposed++; },
    }));
    h.input.write(JSON.stringify(command) + '\n'); await started.promise;
    h.input.write(JSON.stringify({ protocolVersion: 1, kind: 'cancel', dispatchDigest: command.dispatch.dispatchDigest, attemptId: exact ? 'attempt' : 'foreign', generation: 1 }) + '\n');
    if (exact) { await h.done; assert.equal(JSON.parse(h.bytes().toString()).result.kind, 'canceled'); }
    else { await assert.rejects(h.done); assert.equal(h.bytes().length, 0); }
    assert.equal(disposed, 1); assert.equal(aborted, true);
  });
});

test('EOF, channel closure, extra dispatches and trailing partial data cannot produce a successful terminal', async t => {
  for (const scenario of ['eof', 'close', 'duplicate', 'partial']) await t.test(scenario, async () => {
    const release = Promise.withResolvers<void>(), started = Promise.withResolvers<void>();
    let disposed = 0;
    const h = streams(() => ({ async execute(value) { started.resolve(); await release.promise; return { kind: 'completed', text: value.text, effectiveModel: null }; }, async dispose() { disposed++; } }));
    const line = JSON.stringify(request()) + '\n';
    h.input.write(line); await started.promise;
    if (scenario === 'eof') h.input.end();
    else if (scenario === 'close') h.input.destroy();
    else h.input.write(scenario === 'duplicate' ? line : '{');
    await delay(1); release.resolve();
    await assert.rejects(h.done); assert.equal(h.bytes().length, 0); assert.equal(disposed, 1);
  });
});

test('execution failures, invalid results and unconfirmed disposal emit no terminal', async t => {
  for (const scenario of ['execute', 'cleanup', 'late-cleanup', 'invalid-result']) await t.test(scenario, async () => {
    let disposed = 0;
    const h = streams(() => ({
      async execute(value) {
        if (scenario === 'execute') throw new Error('Synthetic private provider detail');
        return { kind: 'completed', text: scenario === 'invalid-result' ? '' : value.text, effectiveModel: null };
      },
      async dispose() { disposed++; if (scenario === 'cleanup') throw new Error('Synthetic cleanup failure'); if (scenario === 'late-cleanup') await new Promise(() => {}); },
    }));
    h.input.write(JSON.stringify(request()) + '\n');
    await assert.rejects(h.done, /completion was not confirmed/);
    assert.equal(disposed, 1); assert.equal(h.bytes().length, 0);
  });
});

test('the production worker speaks to the real executor process without a provider', async t => {
  const worker = fileURLToPath(new URL('./fixtures/owned-executor.mjs', import.meta.url));
  for (const scenario of ['echo', 'cleanup-fail']) {
    const runtime = new ExecutorProcess({ executable: process.execPath, args: [worker, scenario], cwd: process.cwd(), env: {}, lifecycle: { kind: 'processGroup' } });
    const command = request();
    try {
      const result = await runtime.execute(command.input, new AbortController().signal, command.dispatch);
      assert.equal(result.kind, scenario === 'echo' ? 'completed' : 'outcomeUnknown');
      if (result.kind === 'completed') assert.equal(result.text, command.input.text);
      assert.equal(runtime.lastExit?.groupAbsent, true);
      assert.equal(runtime.lastExit?.forced, false);
      const pid = runtime.lastExit?.pid; assert.ok(pid && pid > 1);
      for (const target of [pid, -pid]) assert.throws(() => process.kill(target, 0), { code: 'ESRCH' });
      t.diagnostic(JSON.stringify({ scenario, pid, groupAbsent: true, exitCode: runtime.lastExit?.code }));
    } finally { await runtime.dispose(); }
  }
});

test('the production worker synthetic process journey meets its isolated budget', { skip: !process.env.FORT_EXECUTOR_WORKER_BENCHMARK }, async () => {
  const destination = process.env.FORT_EXECUTOR_WORKER_BENCHMARK; assert.ok(destination);
  const worker = fileURLToPath(new URL('./fixtures/owned-executor.mjs', import.meta.url)), samples: number[] = [];
  for (let index = 0; index < 20; index++) {
    const command = request(), started = performance.now();
    const runtime = new ExecutorProcess({ executable: process.execPath, args: [worker, 'echo'], cwd: process.cwd(), env: {}, lifecycle: { kind: 'processGroup' } });
    try {
      const result = await runtime.execute(command.input, new AbortController().signal, command.dispatch);
      await runtime.dispose(); assert.equal(result.kind, 'completed'); assert.equal(runtime.lastExit?.groupAbsent, true);
      const pid = runtime.lastExit?.pid; assert.ok(pid && pid > 1);
      for (const target of [pid, -pid]) assert.throws(() => process.kill(target, 0), { code: 'ESRCH' });
      samples.push(performance.now() - started);
    } finally { await runtime.dispose(); }
  }
  writeFileSync(destination, JSON.stringify(samples));
  const sorted = [...samples].sort((a, b) => a - b), p95 = sorted[18]; assert.ok(p95 !== undefined && p95 <= 2000);
});
