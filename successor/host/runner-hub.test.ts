import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { setTimeout as delay } from 'node:timers/promises';
import { EXECUTION_LIMITS, TEXT_POLICY, executionDigest, sealDispatch } from '../contracts/executor.ts';
import type { ChatInput, ChatRuntime } from '../runner/chat-runtime.ts';
import { RunnerHub } from './runner-hub.ts';
import { connectToHub } from '../runner/hub-runner.ts';

const binding = { provider: 'remote-fixture', runtimeVersion: 'fixture-1', hostId: 'mac-mini' };
function turn(text = 'exact cedar 🌲', deadlineMs = 5000) {
  const input: ChatInput = { context: { kind: 'empty' },
    attempt: { runId: 'run', attemptId: 'attempt', generation: 1, bindingRevisionId: 'binding', runtimeVersion: binding.runtimeVersion, selectedModel: 'exact-model' },
    runId: 'run', requestId: 'request', conversationId: 'conversation',
    configuration: { agentId: 'agent', agentName: 'Fixture', bindingRevisionId: 'binding', selectedModel: 'exact-model', instructions: '', skills: [] }, history: [], text };
  const dispatch = sealDispatch({ protocolVersion: 1, installationId: 'installation', recoveryEpoch: 'epoch',
    binding: { ...binding, bindingRevisionId: 'binding', selectedModel: 'exact-model', enabled: true }, attempt: input.attempt,
    inputDigest: executionDigest(input), configurationDigest: executionDigest(input.configuration), historyDigest: executionDigest(input.history),
    policy: TEXT_POLICY, policyDigest: executionDigest(TEXT_POLICY), deadline: Date.now() + deadlineMs, limits: EXECUTION_LIMITS });
  return { input, dispatch };
}

async function hub(t: TestContext) {
  const tokens = new Map([['token-mini', 'mac-mini'], ['token-studio', 'mac-studio']]);
  const runners = new RunnerHub({ authenticate: token => tokens.get(token), pingMs: 50 });
  const server = createServer((request, response) => { if (!runners.handle(request, response)) response.writeHead(404).end(); });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const address = server.address(); assert.ok(address && typeof address === 'object');
  t.after(async () => { runners.close(); server.closeAllConnections(); await new Promise(resolve => server.close(resolve)); });
  return { runners, url: `http://127.0.0.1:${address.port}`, revoke: (token: string) => tokens.delete(token) };
}

function runner(t: TestContext, url: string, runtime: () => ChatRuntime, token = 'token-mini') {
  const stop = new AbortController();
  const connection = connectToHub({ url, token, binding, createRuntime: runtime, signal: stop.signal });
  void connection.catch(() => {});
  t.after(() => stop.abort());
  return { stop, connection };
}

async function online(runners: RunnerHub, id: string, expected = true) {
  for (let index = 0; index < 200 && runners.online(id) !== expected; index++) await delay(10);
  assert.equal(runners.online(id), expected);
}

const echo = (seen: string[] = []): ChatRuntime => ({
  async execute(input) { seen.push(input.text); return { kind: 'completed', text: input.text, effectiveModel: null }; },
  async dispose() {},
});

test('a turn dispatched to a connected runner completes with the exact reply', async t => {
  const h = await hub(t), seen: string[] = [];
  runner(t, h.url, () => echo(seen));
  await online(h.runners, 'mac-mini');
  const { input, dispatch } = turn();
  assert.deepEqual(await h.runners.executor('mac-mini').execute(input, new AbortController().signal, dispatch), { kind: 'completed', text: 'exact cedar 🌲', effectiveModel: null });
  assert.deepEqual(seen, ['exact cedar 🌲']);
});

test('an offline runner refuses the turn without sending anything or moving it elsewhere', async t => {
  const h = await hub(t), seen: string[] = [];
  runner(t, h.url, () => echo(seen), 'token-studio');
  await online(h.runners, 'mac-studio');
  const { input, dispatch } = turn();
  const result = await h.runners.executor('mac-mini').execute(input, new AbortController().signal, dispatch);
  assert.equal(result.kind, 'notStarted');
  assert.deepEqual(seen, []);
});

test('an unknown or revoked token cannot connect', async t => {
  const h = await hub(t);
  const refused = await fetch(h.url + '/v1/runner/connect', { method: 'POST', headers: { authorization: 'Bearer nope' } });
  assert.equal(refused.status, 401);
  h.revoke('token-mini');
  const late = runner(t, h.url, () => echo());
  await assert.rejects(late.connection);
  assert.equal(h.runners.online('mac-mini'), false);
});

test('cancelling a turn reaches the runner and returns its cancellation', async t => {
  const h = await hub(t), started = Promise.withResolvers<void>();
  runner(t, h.url, () => ({
    async execute(_input, signal) { started.resolve(); await new Promise(resolve => signal.addEventListener('abort', resolve, { once: true })); return { kind: 'canceled', message: 'Stopped by owner.' }; },
    async dispose() {},
  }));
  await online(h.runners, 'mac-mini');
  const { input, dispatch } = turn(), abort = new AbortController();
  const pending = h.runners.executor('mac-mini').execute(input, abort.signal, dispatch);
  await started.promise; abort.abort();
  assert.deepEqual(await pending, { kind: 'canceled', message: 'Stopped by owner.' });
});

test('a runner that drops mid-turn leaves the outcome unknown, and reconnecting does not replay it', async t => {
  const h = await hub(t), seen: string[] = [], started = Promise.withResolvers<void>();
  const first = runner(t, h.url, () => ({
    async execute(input, signal) { seen.push(input.text); started.resolve(); await new Promise(resolve => signal.addEventListener('abort', resolve, { once: true })); return { kind: 'canceled', message: 'gone' }; },
    async dispose() {},
  }));
  await online(h.runners, 'mac-mini');
  const lost = turn('first');
  const pending = h.runners.executor('mac-mini').execute(lost.input, new AbortController().signal, lost.dispatch);
  await started.promise; first.stop.abort();
  assert.equal((await pending).kind, 'outcomeUnknown');
  await online(h.runners, 'mac-mini', false);
  runner(t, h.url, () => echo(seen));
  await online(h.runners, 'mac-mini');
  const next = turn('second');
  assert.equal((await h.runners.executor('mac-mini').execute(next.input, new AbortController().signal, next.dispatch)).kind, 'completed');
  assert.deepEqual(seen, ['first', 'second']);
});

test('frames for another runner, an unknown lease or past the size bound are refused', async t => {
  const h = await hub(t);
  runner(t, h.url, () => echo());
  await online(h.runners, 'mac-mini');
  const post = (token: string, body: string) => fetch(h.url + '/v1/runner/frames', { method: 'POST', headers: { authorization: `Bearer ${token}`, 'content-type': 'application/x-ndjson' }, body });
  assert.equal((await post('token-mini', JSON.stringify({ leaseId: 'not-a-lease', frame: {} }) + '\n')).status, 409);
  assert.equal((await post('token-studio', JSON.stringify({ leaseId: 'not-a-lease', frame: {} }) + '\n')).status, 409);
  assert.equal((await post('token-mini', 'x'.repeat(2 * 1024 * 1024))).status, 413);
  assert.equal((await post('nope', '{}\n')).status, 401);
});

test('a dispatch that fails its seal is refused before reaching the runner', async t => {
  const h = await hub(t), seen: string[] = [];
  runner(t, h.url, () => echo(seen));
  await online(h.runners, 'mac-mini');
  const { input, dispatch } = turn();
  const result = await h.runners.executor('mac-mini').execute({ ...input, text: 'changed' }, new AbortController().signal, dispatch);
  assert.equal(result.kind, 'notStarted');
  assert.deepEqual(seen, []);
});

test('a runner refuses to send its token to a hub over plain HTTP off this machine', async () => {
  await assert.rejects(connectToHub({ url: 'http://hub.example.com', token: 'token-mini', binding, createRuntime: () => echo(), signal: new AbortController().signal,
    fetch: async () => { throw new Error('No request may be made'); } }), /HTTPS/);
});
