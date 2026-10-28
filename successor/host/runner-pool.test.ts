import { test } from 'node:test';
import assert from 'node:assert/strict';
import { DatabaseSync } from 'node:sqlite';
import { createServer } from 'node:http';
import { setTimeout as delay } from 'node:timers/promises';
import { EXECUTION_LIMITS, TEXT_POLICY, executionDigest, sealDispatch } from '../contracts/executor.ts';
import type { ChatInput, ChatRuntime } from '../runner/chat-runtime.ts';
import { connectToHub } from '../runner/hub-runner.ts';
import { chooseRunner, parseRunnerPool, runPins } from './runner-pool.ts';
import { RunnerHub, routePool } from './runner-hub.ts';

const pool = parseRunnerPool('home', 'mini,laptop');

test('the pool reads its Macs in order of preference and refuses a muddled list', () => {
  assert.deepEqual(pool, { node: 'home', runners: ['mini', 'laptop'] });
  assert.deepEqual(parseRunnerPool('home', ' mini , laptop ,').runners, ['mini', 'laptop']);
  for (const value of ['', ',', 'mini,mini', 'mini,lap top']) assert.throws(() => parseRunnerPool('home', value), /FORT_RUNNER_POOL/);
  assert.throws(() => parseRunnerPool('ho me', 'mini'), /node ID/);
});

test('a run goes to the mini first, the laptop when the mini is away, and nowhere when neither is connected', () => {
  const online = (ids: string[]) => (id: string) => ids.includes(id);
  assert.equal(chooseRunner(pool, online(['mini', 'laptop'])), 'mini');
  assert.equal(chooseRunner(pool, online(['laptop'])), 'laptop');
  assert.equal(chooseRunner(pool, online([])), undefined);
  assert.equal(chooseRunner(pool, online(['outsider'])), undefined);
});

test('a pinned run waits for its own Mac and never moves', () => {
  const online = (ids: string[]) => (id: string) => ids.includes(id);
  assert.equal(chooseRunner(pool, online(['mini', 'laptop']), 'laptop'), 'laptop');
  assert.equal(chooseRunner(pool, online(['mini']), 'laptop'), undefined);
  assert.equal(chooseRunner(pool, online(['mini', 'outsider']), 'outsider'), undefined);
});

test('pins keep the first Mac for a run, survive reopening, and expire after 30 days', () => {
  const db = new DatabaseSync(':memory:');
  let now = Date.UTC(2026, 9, 28);
  const pins = runPins(db, { now: () => now });
  assert.equal(pins.get('run'), undefined);
  assert.equal(pins.pin('run', 'laptop'), 'laptop');
  assert.equal(pins.pin('run', 'mini'), 'laptop');
  assert.equal(runPins(db, { now: () => now }).get('run'), 'laptop');
  now += 31 * 86_400_000;
  assert.equal(runPins(db, { now: () => now }).get('run'), undefined);
});

// The hub side over a real runner channel: two Macs serve the pool's node under their own tokens.
const binding = { provider: 'remote-fixture', runtimeVersion: 'fixture-1', hostId: 'home' };
function turn(runId = 'run', deadlineMs = 5000) {
  const input: ChatInput = { context: { kind: 'empty' },
    attempt: { runId, attemptId: runId + '-attempt', generation: 1, bindingRevisionId: 'binding', runtimeVersion: binding.runtimeVersion, selectedModel: 'exact-model' },
    runId, requestId: 'request', conversationId: 'conversation',
    configuration: { agentId: 'agent', agentName: 'Fixture', bindingRevisionId: 'binding', selectedModel: 'exact-model', instructions: '', skills: [] }, history: [], text: 'exact cedar' };
  const dispatch = sealDispatch({ protocolVersion: 1, installationId: 'installation', recoveryEpoch: 'epoch',
    binding: { ...binding, bindingRevisionId: 'binding', selectedModel: 'exact-model', enabled: true }, attempt: input.attempt,
    inputDigest: executionDigest(input), configurationDigest: executionDigest(input.configuration), historyDigest: executionDigest(input.history),
    policy: TEXT_POLICY, policyDigest: executionDigest(TEXT_POLICY), deadline: Date.now() + deadlineMs, limits: EXECUTION_LIMITS });
  return { input, dispatch };
}

async function hub(t: import('node:test').TestContext, waitMs = 0) {
  const tokens = new Map([['token-mini', 'mini'], ['token-laptop', 'laptop'], ['token-outsider', 'outsider']]);
  const runners = new RunnerHub({ authenticate: token => tokens.get(token), pingMs: 50, nodeOf: id => pool.runners.includes(id) ? pool.node : id });
  const server = createServer((request, response) => { if (!runners.handle(request, response)) response.writeHead(404).end(); });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const address = server.address(); assert.ok(address && typeof address === 'object');
  t.after(async () => { runners.close(); server.closeAllConnections(); await new Promise(resolve => server.close(resolve)); });
  const pins = runPins(new DatabaseSync(':memory:'));
  const lane = routePool(runners, pool, { binding, pins, waitMs: () => waitMs, names: id => ({ mini: 'Mac mini', laptop: 'MacBook' })[id] });
  const url = `http://127.0.0.1:${address.port}`;
  const seen: string[] = [];
  function mac(token: string, hostId = 'home') {
    const stop = new AbortController(), id = tokens.get(token)!;
    const runtime: ChatRuntime = { async execute(input) { seen.push(id + ':' + input.runId); return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() {} };
    void connectToHub({ url, token, binding: { ...binding, hostId }, createRuntime: () => runtime, signal: stop.signal }).catch(() => {});
    t.after(() => stop.abort());
    return stop;
  }
  async function online(id: string, expected = true) {
    for (let index = 0; index < 200 && runners.online(id) !== expected; index++) await delay(10);
    assert.equal(runners.online(id), expected);
  }
  return { runners, lane, pins, seen, mac, online };
}

test('the pooled lane prefers the mini, falls back to the laptop, and pins each run', async t => {
  const h = await hub(t);
  assert.equal(h.lane.readiness?.().state, 'unavailable');
  assert.match(h.lane.readiness?.().detail ?? '', /Waiting for Mac mini or MacBook/);
  const laptop = h.mac('token-laptop'); h.mac('token-mini');
  await h.online('mini'); await h.online('laptop');
  assert.equal(h.lane.readiness?.().state, 'ready');
  const first = turn('first');
  assert.equal((await h.lane.execute(first.input, new AbortController().signal, first.dispatch)).kind, 'completed');
  assert.equal(h.pins.get('first'), 'mini');
  h.runners.disconnect('mini');
  await h.online('mini', false);
  const second = turn('second');
  assert.equal((await h.lane.execute(second.input, new AbortController().signal, second.dispatch)).kind, 'completed');
  assert.equal(h.pins.get('second'), 'laptop');
  assert.deepEqual(h.seen, ['mini:first', 'laptop:second']);
  laptop.abort();
});

test('a later step of a pinned run waits for its Mac and is refused rather than moved', async t => {
  const h = await hub(t, 100);
  h.mac('token-laptop'); h.mac('token-mini');
  await h.online('mini'); await h.online('laptop');
  h.pins.pin('routine', 'laptop');
  h.runners.disconnect('laptop');
  await h.online('laptop', false);
  const step = turn('routine');
  const result = await h.lane.execute(step.input, new AbortController().signal, step.dispatch);
  assert.equal(result.kind, 'notStarted');
  assert.match('message' in result ? result.message : '', /started on MacBook, which is not connected\. Nothing was sent, and the run was not moved/);
  assert.deepEqual(h.seen, []);
});

test('a queued run waits within its window and starts on the first Mac to connect', async t => {
  const h = await hub(t, 3000);
  const waiting = turn('late');
  const result = h.lane.execute(waiting.input, new AbortController().signal, waiting.dispatch);
  await delay(100);
  h.mac('token-laptop');
  assert.equal((await result).kind, 'completed');
  assert.deepEqual(h.seen, ['laptop:late']);
});

test('a Mac outside the pool never takes the pool’s turns', async t => {
  const h = await hub(t);
  h.mac('token-outsider', 'outsider');
  await h.online('outsider');
  const { input, dispatch } = turn('stray');
  assert.equal((await h.lane.execute(input, new AbortController().signal, dispatch)).kind, 'notStarted');
  assert.equal((await h.runners.executor('outsider').execute(input, new AbortController().signal, dispatch)).kind, 'notStarted');
  assert.deepEqual(h.seen, []);
});
