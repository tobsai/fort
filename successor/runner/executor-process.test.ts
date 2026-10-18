import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, realpath } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createApplication } from '../service/openai.ts';
import type { Acceptance, Agent, Snapshot } from '../contracts/v1.ts';
import { ExecutorProcess } from './executor-process.ts';
import type { ChatInput } from './chat-runtime.ts';
import { sealDispatch, type ExecutionDispatch } from '../contracts/executor.ts';

test('executor composition rejects relative guardians and unsupported Mac supervision', () => {
  const options = { executable: process.execPath, args: [], cwd: process.cwd(), env: {} };
  for (const guardian of ['relative', '/invalid\0guardian']) {
    assert.throws(() => new ExecutorProcess({ ...options, lifecycle: { kind: 'macParentExit', guardian } }), /exact Mac guardian/);
  }
  if (process.platform !== 'darwin') assert.throws(() => new ExecutorProcess({ ...options, lifecycle: { kind: 'macParentExit', guardian: '/exact/guardian' } }), /exact Mac guardian/);
  for (const cooperativeCancellationMs of [0, 249, 3501, 5000, NaN, Infinity, 250.5]) {
    assert.throws(() => new ExecutorProcess({ ...options, lifecycle: { kind: 'processGroup' }, cooperativeCancellationMs }), /cancellation allowance/);
  }
});

async function setup(t: TestContext, scenario = 'echo', attemptTimeoutMs = 5_000, change?: (input: ChatInput, dispatch: ExecutionDispatch) => { input: ChatInput; dispatch: ExecutionDispatch }, cooperativeCancellationMs?: number) {
  const directory = await realpath(await mkdtemp(join(tmpdir(), 'fort-executor-contract-')));
  const worker = fileURLToPath(new URL('./fixtures/executor-worker.mjs', import.meta.url));
  const runtime = new ExecutorProcess({ executable: process.execPath,
    args: ['--permission', '--allow-fs-read=' + worker, worker, scenario], cwd: directory, env: {}, lifecycle: { kind: 'processGroup' }, cooperativeCancellationMs });
  const provider = { provider: 'process-fixture', name: 'Process fixture', runtimeVersion: 'fixture-1', hostId: 'fixture-host', session: 'stateless', capabilities: ['plainText', 'cancel'] } as const;
  const service = createApplication({ dataDirectory: directory, provider: { ...provider, capabilities: [...provider.capabilities] },
    runtime: change ? { execute(input, signal, dispatch) { const changed = change(input, dispatch); return runtime.execute(changed.input, signal, changed.dispatch); }, dispose: () => runtime.dispose() } : runtime, attemptTimeoutMs });
  await service.start();
  t.after(async () => { await service.close(); await rm(directory, { recursive: true, force: true }); });
  const enrollment = await fetch(service.baseURL + '/v1/devices/enroll', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...service.issueEnrollment(), deviceName: 'Executor contract fixture' }) });
  assert.equal(enrollment.status, 201);
  const device = await enrollment.json() as { deviceToken: string };
  async function api<T>(path: string, body?: unknown) {
    const response = await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + device.deviceToken, 'X-Fort-Installation-Id': service.installation.installationId }, body: body === undefined ? undefined : JSON.stringify(body) });
    return { status: response.status, body: await response.json() as T };
  }
  const created = await api<Agent>('/v1/agents', { name: 'Fixture', model: 'exact-fixture-model', instructions: 'Echo the supplied text.' });
  assert.equal(created.status, 201);
  const agent = created.body, path = '/v1/conversations/' + agent.homeConversationId;
  return { runtime, service, api, path, async send(requestId: string, text: string) {
    const result = await api<Acceptance>(path + '/messages', { requestId, text, recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: service.installation.recoveryEpoch, skills: [] });
    assert.equal(result.status, 201); return result.body;
  } };
}

test('the active service commits one exact process reply and preserves its receipt on retry', async t => {
  const h = await setup(t);
  const text = '  Exact cedar 🌲\n';
  const first = await h.send('first', text);
  assert.equal(await h.service.dispatchNext(), true);
  const snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
  assert.equal(snapshot.runs.find(run => run.runId === first.receipt.runId)?.state, 'completed');
  assert.deepEqual(snapshot.messages.map(message => message.text), [text, text]);
  const retry = await h.send('first', text);
  assert.deepEqual(retry.receipt, first.receipt);
  assert.equal(await h.service.dispatchNext(), false);
  assert.equal(h.runtime.readiness().state, 'ready');
  assert.equal(h.runtime.lastExit?.groupAbsent, true);
  assert.equal(h.runtime.lastExit?.forced, false);
});

test('an explicitly stopped executor resumes the same selected process only after confirmed cleanup', async t => {
  const h = await setup(t);
  await h.send('before-resume', 'Retained before stop'); await h.service.dispatchNext();
  const previous = h.runtime.lastExit;
  await h.runtime.dispose();
  assert.equal(h.runtime.readiness().state, 'stopped');
  assert.equal(h.runtime.readiness().canRestart, true);
  await assert.rejects(h.runtime.restart(AbortSignal.abort()), /abort/i);
  assert.equal(h.runtime.readiness().state, 'stopped');
  await h.runtime.restart(new AbortController().signal);
  assert.equal(h.runtime.readiness().state, 'ready');
  assert.deepEqual(h.runtime.lastExit, previous, 'Resume must not launch a child');
  await assert.rejects(h.runtime.restart(new AbortController().signal));
  await h.send('after-resume', 'Exact selection retained'); await h.service.dispatchNext();
  assert.deepEqual((await h.api<Snapshot>(h.path + '/snapshot')).body.messages.map(message => message.text),
    ['Retained before stop', 'Retained before stop', 'Exact selection retained', 'Exact selection retained']);
  assert.equal(h.runtime.lastExit?.groupAbsent, true);
});

test('a stale process generation cannot commit a reply or release later work in that conversation', async t => {
  const h = await setup(t, 'stale-generation');
  const first = await h.send('stale', 'First');
  await h.service.dispatchNext();
  const next = await h.send('later', 'Later');
  assert.equal(await h.service.dispatchNext(), false);
  const snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
  assert.equal(snapshot.runs.find(run => run.runId === first.receipt.runId)?.state, 'outcomeUnknown');
  assert.equal(snapshot.runs.find(run => run.runId === next.receipt.runId)?.state, 'queued');
  assert.deepEqual(snapshot.messages.map(message => message.role), ['user', 'user']);
  assert.equal(h.runtime.lastExit?.groupAbsent, true);
});

test('owner cancellation reaches only the active process attempt and leaves no reply', async t => {
  const h = await setup(t, 'await-cancel');
  const accepted = await h.send('cancel', 'Wait for cancellation');
  const work = h.service.dispatchNext();
  const canceled = await h.api('/v1/runs/' + accepted.receipt.runId + '/cancel', { recoveryEpoch: h.service.installation.recoveryEpoch });
  assert.equal(canceled.status, 200);
  await work;
  const snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
  assert.equal(snapshot.runs[0].state, 'canceled');
  assert.equal(snapshot.messages.length, 1);
  assert.equal(h.runtime.lastExit?.groupAbsent, true);
  assert.equal(h.runtime.lastExit?.forced, false);
});

test('foreign, contradictory, oversized or absent process evidence stays unknown', async t => {
  for (const scenario of ['foreign-attempt', 'foreign-dispatch', 'conflicting-terminals', 'oversized-reply', 'stdout-flood', 'stderr-flood', 'invalid-utf8', 'no-terminal', 'nonzero-exit']) await t.test(scenario, async t => {
    const h = await setup(t, scenario);
    await h.send('bad-evidence', 'No invented success');
    await h.service.dispatchNext();
    const snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
    assert.equal(snapshot.runs[0].state, 'outcomeUnknown');
    assert.equal(snapshot.messages.length, 1);
    assert.equal(h.runtime.lastExit?.groupAbsent, true);
  });
});

test('identical repeated terminal evidence commits exactly one reply', async t => {
  const h = await setup(t, 'duplicate');
  await h.send('duplicate', 'One reply');
  await h.service.dispatchNext();
  const snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
  assert.equal(snapshot.runs[0].state, 'completed');
  assert.deepEqual(snapshot.messages.map(message => message.text), ['One reply', 'One reply']);
});

test('a hung or unread executor cannot block the service beyond its deadline and cleanup reserve', async t => {
  for (const scenario of ['hang', 'unread-input', 'terminal-then-hang']) await t.test(scenario, async t => {
    const h = await setup(t, scenario, 200);
    await h.send('deadline', 'x'.repeat(65_536));
    const started = performance.now();
    await h.service.dispatchNext();
    assert.ok(performance.now() - started < 2_500);
    const snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
    assert.equal(snapshot.runs[0].state, 'outcomeUnknown');
    assert.equal(snapshot.messages.length, 1);
    assert.equal(h.runtime.lastExit?.forced, true);
    assert.equal(h.runtime.lastExit?.groupAbsent, true);
    const pid = h.runtime.lastExit?.pid;
    assert.ok(pid);
    assert.throws(() => process.kill(pid, 0), { code: 'ESRCH' });
    assert.throws(() => process.kill(-pid, 0), { code: 'ESRCH' });
  });
});

test('a successful result arriving after the service deadline cannot complete the turn', async t => {
  const h = await setup(t, 'late-terminal', 200);
  await h.send('late', 'Do not accept late success');
  await h.service.dispatchNext();
  const snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
  assert.equal(snapshot.runs[0].state, 'outcomeUnknown');
  assert.equal(snapshot.messages.length, 1);
  assert.equal(h.runtime.lastExit?.groupAbsent, true);
});

test('a longer cooperative allowance cannot extend the dispatch deadline cleanup bound', async t => {
  const h = await setup(t, 'hang', 200, undefined, 3500);
  await h.send('bounded-cancel', 'Synthetic deadline');
  const started = performance.now(); await h.service.dispatchNext();
  assert.ok(performance.now() - started < 2500);
  assert.equal(h.runtime.lastExit?.forced, true); assert.equal(h.runtime.lastExit?.groupAbsent, true);
  const pid = h.runtime.lastExit?.pid; assert.ok(pid && pid > 1);
  for (const target of [pid, -pid]) assert.throws(() => process.kill(target, 0), { code: 'ESRCH' });
});

test('changed input, policy digest or expired authority is refused before process launch', async t => {
  const changes: { name: string; change: NonNullable<Parameters<typeof setup>[3]> }[] = [
    { name: 'input', change: (input, dispatch) => ({ input: { ...input, text: 'Tampered input' }, dispatch }) },
    { name: 'policy', change: (input, dispatch) => ({ input, dispatch: { ...dispatch, policyDigest: '0'.repeat(64) } }) },
    { name: 'expiry', change: (input, dispatch) => { const { dispatchDigest, ...body } = dispatch; return { input, dispatch: sealDispatch({ ...body, deadline: Date.now() - 1 }) }; } }
  ];
  for (const { name, change } of changes) await t.test(name, async t => {
    const h = await setup(t, 'echo', 5_000, change);
    await h.send('altered', 'Original input');
    await h.service.dispatchNext();
    const snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
    assert.equal(snapshot.runs[0].state, 'failed');
    assert.equal(snapshot.messages.length, 1);
    assert.equal(h.runtime.lastExit, undefined);
  });
});

test('the synthetic child has no ledger, network, child-process or inherited environment access', async t => {
  const h = await setup(t, 'permission-controls');
  await h.send('permissions', 'Permission controls completed');
  await h.service.dispatchNext();
  const snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
  assert.equal(snapshot.runs[0].state, 'completed', JSON.stringify(h.runtime.lastExit));
  assert.equal(snapshot.messages[1].text, 'Permission controls completed');
  assert.equal(h.runtime.lastExit?.groupAbsent, true);
});
