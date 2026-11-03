import { test } from 'node:test';
import assert from 'node:assert/strict';
import { DatabaseSync } from 'node:sqlite';
import type { ExecutionDispatch } from '../contracts/executor.ts';
import type { ChatInput, ChatRuntime } from '../runner/chat-runtime.ts';
import { runPins } from './runner-pool.ts';
import { CLOUD_RUNNER_ID, routeCloudFirst } from './cloud-runner.ts';

// Spec 063, revision 2026-11-03: the hub first, Macs for what it can't take, and a started run never moves.
function fake(name: string, seen: string[], ready = true): ChatRuntime {
  return {
    async execute(input) { seen.push(name + ':' + input.text); return { kind: 'completed', text: input.text, effectiveModel: null }; },
    readiness: () => ({ state: ready ? 'ready' : 'unavailable', canRestart: false, detail: name + (ready ? ' ready' : ' away') }),
    models: async () => ({ provider: 'codex', state: 'ready', models: [{ id: name, name, availability: 'available' }], checkedAt: new Date(0).toISOString() }),
    signIn: async () => ({ kind: 'signedIn', account: { accountId: 'account', userLabel: 'owner@example.com' } }),
    async dispose() {},
  };
}
const turn = (runId: string, text: string) => [{ text } as ChatInput, new AbortController().signal, { attempt: { runId } } as ExecutionDispatch] as const;

function router(options: { signedIn: boolean; macs?: boolean; macsReady?: boolean }) {
  const seen: string[] = [], state = { signedIn: options.signedIn, woke: 0 };
  const pins = runPins(new DatabaseSync(':memory:'));
  const cloud = fake('hub', seen), macs = options.macs === false ? undefined : fake('mac', seen, options.macsReady ?? true);
  cloud.signIn = async () => { state.signedIn = true; return { kind: 'signedIn', account: { accountId: 'account', userLabel: 'owner@example.com' } }; };
  const route = routeCloudFirst({ cloud: { runtime: cloud, signedIn: () => state.signedIn }, ...(macs ? { macs } : {}), pins, onSignedIn: () => { state.woke++; } });
  return { route, seen, state, pins };
}

test('a signed-in hub takes every new run, with or without a Mac', async () => {
  for (const macs of [true, false]) {
    const r = router({ signedIn: true, macs });
    assert.equal((await r.route.execute(...turn('run-1', 'hello'))).kind, 'completed');
    assert.deepEqual(r.seen, ['hub:hello']);
    assert.equal(r.pins.get('run-1'), CLOUD_RUNNER_ID);
    assert.equal(r.route.readiness!().state, 'ready');
    assert.equal(r.route.readiness!().detail, 'Runs on the hub.');
  }
});

test('until the hub signs in, runs go to the Macs as before', async () => {
  const r = router({ signedIn: false });
  await r.route.execute(...turn('run-1', 'early'));
  assert.deepEqual(r.seen, ['mac:early']);
  assert.equal(r.route.readiness!().detail, 'mac ready');
  assert.equal((await r.route.models!(new AbortController().signal)).models[0]!.id, 'mac');
});

test('with no Mac and no hub sign-in, nothing is sent and the lane says how to fix it', async () => {
  const r = router({ signedIn: false, macs: false });
  const result = await r.route.execute(...turn('run-1', 'nowhere'));
  assert.equal(result.kind, 'notStarted');
  assert.deepEqual(r.seen, []);
  const readiness = r.route.readiness!();
  assert.equal(readiness.state, 'unavailable');
  assert.match(readiness.detail, /Sign in to ChatGPT on the hub/);
});

test('a Mac that is away and a hub not signed in keep work queued', () => {
  const r = router({ signedIn: false, macsReady: false });
  const readiness = r.route.readiness!();
  assert.equal(readiness.state, 'unavailable');
  assert.match(readiness.detail, /Sign in to ChatGPT on the hub.*mac away/);
});

test('sign-in always happens on the hub, and wakes queued work', async () => {
  const r = router({ signedIn: false });
  const result = await r.route.signIn!({ durationMs: 1000, onChallenge() {} }, new AbortController().signal);
  assert.equal(result.kind, 'signedIn');
  assert.equal(r.state.signedIn, true);
  assert.equal(r.state.woke, 1);
  await r.route.execute(...turn('run-2', 'after'));
  assert.deepEqual(r.seen, ['hub:after']);
  assert.equal((await r.route.models!(new AbortController().signal)).models[0]!.id, 'hub');
});

test('a run that started on a Mac finishes on the Mac, even after the hub signs in', async () => {
  const r = router({ signedIn: false });
  r.pins.pin('routine', 'mini');
  r.state.signedIn = true;
  await r.route.execute(...turn('routine', 'step two'));
  assert.deepEqual(r.seen, ['mac:step two']);
});

test('a run that started on the hub never moves to a Mac', async () => {
  const r = router({ signedIn: true });
  await r.route.execute(...turn('routine', 'step one'));
  r.state.signedIn = false;
  const result = await r.route.execute(...turn('routine', 'step two'));
  assert.equal(result.kind, 'notStarted');
  assert.deepEqual(r.seen, ['hub:step one']);
});
