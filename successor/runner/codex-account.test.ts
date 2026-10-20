import assert from 'node:assert/strict';
import test from 'node:test';
import { PassThrough, Writable } from 'node:stream';
import { writeFileSync } from 'node:fs';
import { performance } from 'node:perf_hooks';
import { CodexConnection } from './codex-rpc.ts';
import { syntheticConfig, syntheticFeatures, syntheticAccount, syntheticAccountIdentity } from './codex-preflight-fixture.ts';
import { signInCodexAccount, type CodexDeviceChallenge } from './codex-account.ts';

const loginId = 'e5b96847-e3c1-48b5-ab3d-5ec8ee92878f';
const challenge = { type: 'chatgptDeviceCode', loginId, verificationUrl: 'https://auth.openai.com/codex/device', userCode: 'TEST-ONLY' };
function fixture(options: {
  early?: boolean; account?: unknown; routing?: unknown; requiresAuth?: boolean; config?: unknown; challenge?: unknown;
  cancelStatus?: unknown; cancelCompletes?: boolean; silent?: string; cleanupFailure?: boolean; unsolicited?: boolean;
  requestAfterAccountRead?: number;
} = {}) {
  const output = new PassThrough();
  const calls: { method?: string; params?: Record<string, unknown>; id?: unknown }[] = [];
  let account: unknown = options.account ?? null, closed = 0, accountReads = 0;
  const emit = (value: unknown) => output.write(JSON.stringify(value) + '\n');
  const complete = (success = true, id = loginId, saveAccount = success) => {
    if (saveAccount) account = syntheticAccount().account;
    emit({ method: 'account/login/completed', params: { loginId: id, success, error: success ? null : 'synthetic-private-error' } });
  };
  const input = new Writable({ write(chunk, _encoding, done) {
    const frame = JSON.parse(chunk.toString()); calls.push(frame);
    queueMicrotask(() => {
      if (frame.method === options.silent) return;
      const reply = (result: unknown) => emit({ id: frame.id, result });
      switch (frame.method) {
        case 'initialize': if (options.unsolicited) complete(); reply({ userAgent: 'synthetic' }); break;
        case 'config/read': reply({ config: options.config ?? syntheticConfig() }); break;
        case 'permissionProfile/list': reply({ data: [{ id: 'fort_candidate_b_no_files', description: null, allowed: true }], nextCursor: null }); break;
        case 'experimentalFeature/list': reply({ data: syntheticFeatures(), nextCursor: null }); break;
        case 'mcpServerStatus/list': reply({ data: [], nextCursor: null }); break;
        case 'account/read':
          reply({ workspaceRouting: options.routing === undefined ? syntheticAccount().workspaceRouting : options.routing, account, requiresOpenaiAuth: options.requiresAuth ?? true });
          if (++accountReads === options.requestAfterAccountRead) emit({ id: 'forbidden', method: 'item/tool/call', params: {} });
          break;
        case 'account/login/start': if (options.early) complete(); reply(options.challenge ?? challenge); break;
        case 'account/login/cancel': if (options.cancelCompletes) complete(); reply({ status: options.cancelStatus ?? 'canceled' }); break;
      }
    }); done();
  } });
  const connection = new CodexConnection({ input, output, close: () => {
    closed++; input.destroy(); output.destroy();
    if (options.cleanupFailure) throw new Error('synthetic-private-cleanup-error');
  } });
  return { connection, calls, complete, emit, get closed() { return closed; } };
}

test('explicit device sign-in validates the challenge, confirms the account and disposes its connection', async () => {
  const f = fixture(), seen: CodexDeviceChallenge[] = [];
  assert.equal(f.calls.length, 0);
  const result = await signInCodexAccount({ connection: f.connection, workspace: '/synthetic/empty', runtimeVersion: '0.163.0',
    signal: new AbortController().signal, timeoutMs: 1000, onChallenge: value => { seen.push(value); f.complete(); } });
  assert.deepEqual(result, { kind: 'signedIn', account: syntheticAccountIdentity });
  assert.deepEqual(seen, [{ loginId, verificationURL: challenge.verificationUrl, userCode: challenge.userCode }]);
  assert.deepEqual(f.calls.filter(call => call.method === 'account/login/start').map(call => call.params), [{ type: 'chatgptDeviceCode' }]);
  assert.deepEqual(f.calls.filter(call => call.method === 'account/read').map(call => call.params), [{ refreshToken: false }, { refreshToken: false }]);
  assert.equal(f.calls.some(call => /thread|turn|logout|refresh|model\/list/.test(call.method ?? '')), false);
  assert.equal(f.closed, 1);
  assert.doesNotMatch(JSON.stringify({ result, seen }), /synthetic-private/);
});

function start(f: ReturnType<typeof fixture>, onChallenge: (value: CodexDeviceChallenge) => void = () => f.complete(), signal = new AbortController().signal, timeoutMs = 1000) {
  return signInCodexAccount({ connection: f.connection, workspace: '/synthetic/empty', runtimeVersion: '0.163.0', signal, timeoutMs, onChallenge });
}

test('an existing ChatGPT account is retained without another login and reports only its nonsecret identity', async () => {
  const f = fixture({ account: syntheticAccount().account });
  assert.deepEqual(await start(f, () => assert.fail('No challenge expected')), { kind: 'signedIn', account: syntheticAccountIdentity });
  assert.equal(f.calls.some(call => call.method?.includes('login')), false);
  assert.equal(f.closed, 1);
});

for (const phase of ['existing', 'new']) test('a forbidden server request after the ' + phase + ' account reply prevents success', async () => {
  const f = fixture({ ...(phase === 'existing' ? { account: { type: 'chatgpt' } } : {}), requestAfterAccountRead: phase === 'existing' ? 1 : 2 });
  assert.deepEqual(await start(f), { kind: 'unconfirmed' });
  assert.equal(f.closed, 1);
});

for (const scenario of ['foreign-account', 'auth-disabled', 'unsafe-controls', 'unsolicited-completion'] as const) {
  test(`device sign-in refuses ${scenario} before a login request`, async () => {
    const f = fixture(scenario === 'foreign-account' ? { account: { type: 'apiKey' } }
      : scenario === 'auth-disabled' ? { requiresAuth: false }
      : scenario === 'unsafe-controls' ? { config: { ...syntheticConfig(), analytics: { enabled: true } } }
      : { unsolicited: true });
    assert.deepEqual(await start(f, () => assert.fail('No challenge expected')), { kind: 'unconfirmed' });
    assert.equal(f.calls.some(call => call.method === 'account/login/start'), false);
    if (scenario === 'unsafe-controls') assert.equal(f.calls.some(call => call.method === 'account/read'), false);
    assert.equal(f.closed, 1);
  });
}

for (const changed of [
  { type: 'apiKey' }, { loginId: 'other' }, { verificationUrl: 'http://auth.openai.com/codex/device' },
  { verificationUrl: 'https://auth.openai.com.attacker.invalid/codex/device' },
  { verificationUrl: 'https://auth.openai.com/codex/device?token=synthetic-private' },
  { verificationUrl: 'https://synthetic-private@auth.openai.com/codex/device' },
  { userCode: '' }, { userCode: 'A'.repeat(33) }, { userCode: 'TEST\nPRIVATE' }, { userCode: 'open-this://link' },
]) test('unsafe device challenge is never presented: ' + Object.keys(changed)[0] + '-' + JSON.stringify(changed).length, async () => {
  const f = fixture({ challenge: { ...challenge, ...changed } }); let shown = false;
  assert.deepEqual(await start(f, () => { shown = true; }), { kind: 'unconfirmed' });
  assert.equal(shown, false); assert.equal(f.closed, 1);
});

test('completion arriving before the login reply is correlated and account-checked', async () => {
  const f = fixture({ early: true });
  assert.deepEqual(await start(f, () => {}), { kind: 'signedIn', account: syntheticAccountIdentity });
  assert.equal(f.calls.filter(call => call.method === 'account/read').length, 2);
});

test('a successful notification without a confirmed account stays unconfirmed', async () => {
  const f = fixture();
  assert.deepEqual(await start(f, () => f.complete(true, loginId, false)), { kind: 'unconfirmed' });
  assert.equal(f.closed, 1);
});

test('failure details stay private and the account is checked before reporting failure', async () => {
  const f = fixture();
  assert.deepEqual(await start(f, () => f.complete(false)), { kind: 'failed' });
  assert.equal(f.calls.filter(call => call.method === 'account/read').length, 2);
});

test('foreign completion is refused; identical duplicate completion is harmless', async () => {
  const foreign = fixture();
  assert.deepEqual(await start(foreign, () => foreign.complete(true, 'b1772565-580e-4b0d-9d37-bf3d375a5ae8')), { kind: 'unconfirmed' });
  const duplicate = fixture();
  assert.deepEqual(await start(duplicate, () => { duplicate.complete(); duplicate.complete(); }), { kind: 'signedIn', account: syntheticAccountIdentity });
  const conflicting = fixture();
  assert.deepEqual(await start(conflicting, () => { conflicting.complete(); conflicting.complete(false); }), { kind: 'unconfirmed' });
});

for (const changed of [{ loginId: null }, { success: 'true' }, { error: 'synthetic-private' }]) {
  test('malformed completion cannot establish account success: ' + Object.keys(changed)[0], async () => {
    const f = fixture();
    assert.deepEqual(await start(f, () => f.emit({ method: 'account/login/completed',
      params: { loginId, success: true, error: null, ...changed } })), { kind: 'unconfirmed' });
    assert.equal(f.closed, 1);
  });
}

test('foreign early completion cannot borrow a later login identifier', async () => {
  const f = fixture({ early: true, challenge: { ...challenge, loginId: 'b1772565-580e-4b0d-9d37-bf3d375a5ae8' } });
  assert.deepEqual(await start(f, () => assert.fail('No challenge')), { kind: 'unconfirmed' });
  assert.equal(f.closed, 1);
});

for (const status of ['canceled', 'notFound', 'unknown']) test(`cancel receipt ${status} is interpreted with account confirmation`, async () => {
  const f = fixture({ cancelStatus: status }), abort = new AbortController();
  assert.deepEqual(await start(f, () => abort.abort(), abort.signal), { kind: status === 'canceled' ? 'canceled' : 'unconfirmed' });
  assert.deepEqual(f.calls.filter(call => call.method === 'account/login/cancel').map(call => call.params), [{ loginId }]);
  assert.equal(f.closed, 1);
});

test('cancellation racing completed login reports the retained account instead of claiming sign-out', async () => {
  const f = fixture({ cancelStatus: 'notFound', cancelCompletes: true }), abort = new AbortController();
  assert.deepEqual(await start(f, () => abort.abort(), abort.signal), { kind: 'signedIn', account: syntheticAccountIdentity });
  assert.equal(f.calls.some(call => call.method === 'account/logout'), false);
});

test('already canceled input has no protocol effects and disposes the owned connection', async () => {
  const f = fixture();
  assert.deepEqual(await start(f, () => {}, AbortSignal.abort()), { kind: 'canceled' });
  assert.deepEqual(f.calls, []); assert.equal(f.closed, 1);
});

test('attempt deadline cancels a pending login and lost start replies stay unconfirmed', async () => {
  const waiting = fixture();
  assert.deepEqual(await start(waiting, () => {}, new AbortController().signal, 20), { kind: 'canceled' });
  const lost = fixture({ silent: 'account/login/start' });
  assert.deepEqual(await start(lost, () => assert.fail('No challenge'), new AbortController().signal, 20), { kind: 'unconfirmed' });
  assert.equal(lost.calls.filter(call => call.method === 'account/login/start').length, 1);
  assert.equal(lost.closed, 1);
});

test('a failed cancel response remains unconfirmed and closes its owned connection', async () => {
  const f = fixture(), abort = new AbortController();
  assert.deepEqual(await start(f, () => {
    abort.abort(); queueMicrotask(() => f.connection.close());
  }, abort.signal), { kind: 'unconfirmed' });
  assert.equal(f.closed, 1);
});

for (const timeoutMs of [0, -1, 600_001, Number.NaN, 0.5]) test('invalid attempt budget performs no protocol work: ' + timeoutMs, async () => {
  const f = fixture();
  assert.deepEqual(await start(f, () => assert.fail('No challenge'), new AbortController().signal, timeoutMs), { kind: 'unconfirmed' });
  assert.equal(f.calls.length, 0); assert.equal(f.closed, 1);
});

for (const method of ['account/chatgptAuthTokens/refresh', 'item/tool/call', 'item/permissions/requestApproval']) {
  test(`sign-in refuses server request ${method}`, async () => {
    const f = fixture();
    assert.deepEqual(await start(f, () => f.emit({ id: 'request', method, params: { secret: 'synthetic-private' } })), { kind: 'unconfirmed' });
    assert.equal(f.closed, 1);
    assert.ok(f.calls.some(call => call.id === 'request' && !Object.hasOwn(call, 'result')));
  });
}

test('disconnection, callback failure and failed cleanup cannot claim successful completion', async () => {
  const disconnected = fixture();
  assert.deepEqual(await start(disconnected, () => disconnected.connection.close()), { kind: 'unconfirmed' });
  const throwing = fixture();
  assert.deepEqual(await start(throwing, () => { throw new Error('synthetic-private'); }), { kind: 'unconfirmed' });
  const dirty = fixture({ cleanupFailure: true });
  await assert.rejects(start(dirty), { message: 'Account connection cleanup was not confirmed' });
  assert.equal(dirty.closed, 1);
});

test('one connection cannot start competing sign-in attempts or mutate the captured challenge', async () => {
  const f = fixture(), shown = Promise.withResolvers<void>();
  const first = start(f, value => { value.loginId = 'changed'; shown.resolve(); });
  await shown.promise;
  await assert.rejects(start(f), /already in use/);
  assert.equal(f.closed, 0); f.complete();
  assert.deepEqual(await first, { kind: 'signedIn', account: syntheticAccountIdentity });
  assert.equal(f.calls.filter(call => call.method === 'account/login/start').length, 1);
});

test('account protocol retains the original callback when its options are later changed', async () => {
  const f = fixture(); let shown = 0;
  const options: Parameters<typeof signInCodexAccount>[0] = { connection: f.connection, workspace: '/synthetic/empty', runtimeVersion: '0.163.0',
    signal: new AbortController().signal, timeoutMs: 1000, onChallenge: () => { shown++; f.complete(); } };
  const pending = signInCodexAccount(options);
  options.onChallenge = () => { throw new Error('Changed callback must not receive the code'); };
  assert.deepEqual(await pending, { kind: 'signedIn', account: syntheticAccountIdentity }); assert.equal(shown, 1);
});

test('device sign-in synthetic protocol budget', { skip: !process.env.FORT_ACCOUNT_BENCHMARK }, async () => {
  const samples: number[] = [];
  for (let i = 0; i < 25; i++) {
    const f = fixture(), before = performance.now();
    assert.deepEqual(await start(f), { kind: 'signedIn', account: syntheticAccountIdentity });
    const elapsed = performance.now() - before;
    if (i >= 5) samples.push(elapsed);
    assert.equal(f.closed, 1);
  }
  samples.sort((a, b) => a - b);
  const result = { samples, p50: samples[9], p95: samples[18], max: samples[19], budgetMs: 25, warmups: 5 };
  assert.ok(result.p95 !== undefined && result.p95 < result.budgetMs);
  writeFileSync(process.env.FORT_ACCOUNT_BENCHMARK!, JSON.stringify(result) + '\n', { flag: 'wx', mode: 0o600 });
});

for (const account of [{ type: 'chatgpt', email: null }, { type: 'chatgpt' }, { type: 'chatgpt', email: 'unsafe\n@fixture.invalid' }])
  test('missing or invalid reported email cannot establish account identity ' + JSON.stringify(account), async () => {
    const f = fixture({ account }); assert.deepEqual(await start(f), { kind: 'unconfirmed' });
    assert.equal(f.calls.some(c => c.method === 'account/login/start'), false);
  });
for (const routing of [null, {}, { chatgptAccountId: null }, { chatgptAccountId: '' }, { chatgptAccountId: 'unsafe/account' }])
  test('missing or invalid workspace cannot establish account identity ' + JSON.stringify(routing), async () => {
    const f = fixture({ account: syntheticAccount().account, routing }); assert.deepEqual(await start(f), { kind: 'unconfirmed' });
    assert.equal(f.calls.some(c => c.method === 'account/login/start'), false);
  });
