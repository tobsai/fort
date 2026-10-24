import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { setTimeout as delay } from 'node:timers/promises';
import { EXECUTION_LIMITS, TEXT_POLICY, executionDigest, sealDispatch } from '../contracts/executor.ts';
import type { ChatInput, ChatRuntime } from './chat-runtime.ts';
import { RunnerHub } from '../host/runner-hub.ts';
import { keepConnected, type RunnerStatus } from './hub-runner.ts';

const binding = { provider: 'remote-fixture', runtimeVersion: 'fixture-1', hostId: 'mac-mini' };
function turn(text: string) {
  const input: ChatInput = { context: { kind: 'empty' },
    attempt: { runId: 'run', attemptId: 'attempt-' + text, generation: 1, bindingRevisionId: 'binding', runtimeVersion: binding.runtimeVersion, selectedModel: 'exact-model' },
    runId: 'run', requestId: 'request', conversationId: 'conversation',
    configuration: { agentId: 'agent', agentName: 'Fixture', bindingRevisionId: 'binding', selectedModel: 'exact-model', instructions: '', skills: [] }, history: [], text };
  const dispatch = sealDispatch({ protocolVersion: 1, installationId: 'installation', recoveryEpoch: 'epoch',
    binding: { ...binding, bindingRevisionId: 'binding', selectedModel: 'exact-model', enabled: true }, attempt: input.attempt,
    inputDigest: executionDigest(input), configurationDigest: executionDigest(input.configuration), historyDigest: executionDigest(input.history),
    policy: TEXT_POLICY, policyDigest: executionDigest(TEXT_POLICY), deadline: Date.now() + 5000, limits: EXECUTION_LIMITS });
  return { input, dispatch };
}
const echo = (seen: string[]): ChatRuntime => ({ async execute(input) { seen.push(input.text); return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() {} });

async function serve(t: TestContext, handler: Parameters<typeof createServer>[1]) {
  const server = createServer(handler);
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const address = server.address(); assert.ok(address && typeof address === 'object');
  t.after(async () => { server.closeAllConnections(); await new Promise(resolve => server.close(resolve)); });
  return `http://127.0.0.1:${address.port}`;
}
function run(t: TestContext, url: string, runtime: ChatRuntime, extra: { idleMs?: number } = {}) {
  const stop = new AbortController(), statuses: RunnerStatus['state'][] = [];
  const done = keepConnected({ url, token: 'token-mini', binding, createRuntime: () => runtime, signal: stop.signal, minRetryMs: 20, maxRetryMs: 100, onStatus: status => statuses.push(status.state), ...extra });
  t.after(async () => { stop.abort(); await done; });
  return { stop, done, statuses };
}

test('a dropped runner reconnects by itself and takes the next turn', async t => {
  const hub = new RunnerHub({ authenticate: token => token === 'token-mini' ? 'mac-mini' : undefined, pingMs: 50 }), seen: string[] = [];
  t.after(() => hub.close());
  const url = await serve(t, (request, response) => { if (!hub.handle(request, response)) response.writeHead(404).end(); });
  run(t, url, echo(seen));
  for (let index = 0; index < 200 && !hub.online('mac-mini'); index++) await delay(10);
  hub.disconnect('mac-mini');
  for (let index = 0; index < 200 && !hub.online('mac-mini'); index++) await delay(10);
  const { input, dispatch } = turn('after reconnect');
  assert.equal((await hub.executor('mac-mini').execute(input, new AbortController().signal, dispatch)).kind, 'completed');
  assert.deepEqual(seen, ['after reconnect']);
});

test('a refused runner stops instead of retrying', async t => {
  let attempts = 0;
  const url = await serve(t, (_request, response) => { attempts++; response.writeHead(401).end(); });
  const runner = run(t, url, echo([]));
  assert.equal((await runner.done).state, 'refused');
  await delay(100);
  assert.equal(attempts, 1);
});

test('a hub that goes silent is treated as gone and the runner reconnects', async t => {
  let attempts = 0;
  const url = await serve(t, (request, response) => { attempts++; request.resume(); response.writeHead(200, { 'content-type': 'application/x-ndjson' }); response.flushHeaders(); });
  run(t, url, echo([]), { idleMs: 50 });
  for (let index = 0; index < 200 && attempts < 3; index++) await delay(10);
  assert.ok(attempts >= 3);
});

test('stopping the runner ends the loop and reports it', async t => {
  const url = await serve(t, (_request, response) => { response.writeHead(503).end(); });
  const runner = run(t, url, echo([]));
  await delay(60); runner.stop.abort();
  assert.equal((await runner.done).state, 'stopped');
  assert.ok(runner.statuses.includes('waiting'));
});
