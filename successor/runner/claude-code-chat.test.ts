import { test } from 'node:test';
import assert from 'node:assert/strict';
import { PassThrough } from 'node:stream';
import { contextInput } from './fixtures/context-input.ts';
import type { ChatInput } from './chat-runtime.ts';
import type { ExecutionDispatch } from '../contracts/executor.ts';
import { ClaudeCodeChat, claudeCodeLaunch, initializeProblems, parseClaudeAuthStatus, type ClaudeCodeLaunch, type ClaudeCodeProcess } from './claude-code-chat.ts';

const VERSION = '2.1.310', MODEL = 'claude-sonnet-5-5', WORKSPACE = '/synthetic/empty', RUNTIME_DIR = '/synthetic/private';
const status = { loggedIn: true, authMethod: 'claude.ai', apiProvider: 'firstParty', email: 'owner@example.com', orgId: '0f6e7c1a-1111-4222-8333-944445555666', orgName: 'Owner', subscriptionType: 'max' };
const account = parseClaudeAuthStatus(status)!;
const base: Omit<ChatInput, 'context'> = { attempt: { runId: 'run-1', attemptId: 'attempt-1', generation: 1, bindingRevisionId: 'binding', runtimeVersion: VERSION, selectedModel: MODEL }, runId: 'run-1', requestId: 'request-1', conversationId: 'home', text: 'What was the word?',
  configuration: { account, agentId: 'writer', agentName: 'Writer', bindingRevisionId: 'binding', selectedModel: MODEL, instructions: 'Use only the supplied context.', skills: [] },
  history: [{ messageId: 'm1', requestId: 'earlier', conversationId: 'home', role: 'user', text: 'Remember cedar.', sequence: 1, createdAt: '2026-10-24T00:00:00Z' }] };
const dispatch = { deadline: Date.now() + 120_000 } as ExecutionDispatch;
const signedOutStatus = { loggedIn: false, authMethod: 'none', apiProvider: 'firstParty' };
const LOGIN = 'https://claude.com/cai/oauth/authorize?code=true&client_id=fixture&response_type=code&redirect_uri=http%3A%2F%2Flocalhost%3A54545%2Fcallback&state=fixture';

const models = [
  { value: 'default', resolvedModel: 'claude-opus-5-5', displayName: 'Default (recommended)' },
  { value: 'opus', resolvedModel: 'claude-opus-5-5', displayName: 'Opus' },
  { value: 'sonnet', resolvedModel: MODEL, displayName: 'Sonnet' },
];
const initialize = (overrides: Record<string, unknown> = {}) => ({ commands: [], models, claude_code_version: VERSION, current_permission_mode: 'default', analytics_disabled: true,
  remote_control_auto_enable: false, remote_control_auto_connect_default: false, remote_control_auto_on_by_default: false,
  // Shape observed from a signed-in Pro account on Claude Code 2.1.299: no tokenSource.
  account: { email: status.email, organization: 'Owner', subscriptionType: 'pro', apiProvider: 'firstParty' }, ...overrides });
const init = (overrides: Record<string, unknown> = {}) => ({ type: 'system', subtype: 'init', session_id: 'session-1', cwd: WORKSPACE, model: MODEL, tools: [], mcp_servers: [], slash_commands: [], skills: [],
  permissionMode: 'default', apiKeySource: 'none', claude_code_version: VERSION, messaging_socket_path: RUNTIME_DIR + '/cc-socks/1.sock', ...overrides });
const assistant = (text: string, overrides: Record<string, unknown> = {}) => ({ type: 'assistant', uuid: 'u-' + text, parent_tool_use_id: null, session_id: 'session-1', message: { id: 'msg-1', role: 'assistant', model: MODEL, content: [{ type: 'text', text }] }, ...overrides });
const result = (text: string, overrides: Record<string, unknown> = {}) => ({ type: 'result', subtype: 'success', is_error: false, result: text, session_id: 'session-1', permission_denials: [], ...overrides });
// rate_limit_event appears in real Claude Code output before the result.
const reply = (text: string) => [init(), { type: 'stream_event', event: { type: 'content_block_delta', index: 0, delta: { type: 'text_delta', text: text.slice(0, 2) } } }, assistant(text), { type: 'rate_limit_event', rate_limit_info: { status: 'allowed' } }, result(text)];

type Script = { initialize?: unknown; onUser?: () => unknown[]; onInterrupt?: () => unknown[]; exitAfterUser?: boolean;
  authenticate?: unknown; completion?: 'success' | 'error' | 'never'; onCompletion?: () => void; beforeInitialize?: unknown[]; initializeError?: string };
function fixture(script: Script = {}, accountStatus: unknown = status) {
  const readStatus = typeof accountStatus === 'function' ? accountStatus as () => unknown : () => accountStatus;
  const launches: ClaudeCodeLaunch[] = [], writes: Record<string, unknown>[] = [], statusCalls: number[] = [];
  let killed = 0;
  const launch = (plan: ClaudeCodeLaunch): ClaudeCodeProcess => {
    launches.push(plan);
    const stdin = new PassThrough(), stdout = new PassThrough();
    const emit = (frames: unknown[]) => { for (const frame of frames) stdout.write(JSON.stringify(frame) + '\n'); };
    let buffer = '';
    stdin.on('data', (chunk: Buffer) => {
      buffer += chunk.toString();
      for (let end = buffer.indexOf('\n'); end >= 0; end = buffer.indexOf('\n')) {
        const frame = JSON.parse(buffer.slice(0, end)); buffer = buffer.slice(end + 1); writes.push(frame);
        queueMicrotask(() => {
          if (frame.type === 'control_request' && frame.request.subtype === 'initialize')
            emit([...script.beforeInitialize ?? [], { type: 'control_response', response: script.initializeError !== undefined ? { subtype: 'error', request_id: frame.request_id, error: script.initializeError }
              : { subtype: 'success', request_id: frame.request_id, response: script.initialize ?? initialize() } }]);
          else if (frame.type === 'control_request' && frame.request.subtype === 'claude_authenticate')
            emit([{ type: 'control_response', response: { subtype: 'success', request_id: frame.request_id, response: script.authenticate ?? { manualUrl: LOGIN.replace('code=true', 'code=manual'), automaticUrl: LOGIN } } }]);
          else if (frame.type === 'control_request' && frame.request.subtype === 'claude_oauth_wait_for_completion') {
            if (script.completion === 'never') return;
            script.onCompletion?.();
            emit([{ type: 'control_response', response: script.completion === 'error' ? { subtype: 'error', request_id: frame.request_id, error: 'denied' }
              : { subtype: 'success', request_id: frame.request_id, response: { account: { email: status.email, subscriptionType: 'max' } } } }]);
          } else if (frame.type === 'control_request' && frame.request.subtype === 'interrupt') {
            emit([{ type: 'control_response', response: { subtype: 'success', request_id: frame.request_id, response: {} } }]);
            emit(script.onInterrupt?.() ?? []);
          } else if (frame.type === 'user') { emit(script.onUser?.() ?? reply('cedar')); if (script.exitAfterUser) stdout.end(); }
        });
      }
    });
    return { stdin, stdout, kill: () => { killed++; stdout.end(); } };
  };
  const runtime = new ClaudeCodeChat({ executable: '/synthetic/bin/claude', workspace: WORKSPACE, home: '/synthetic/home', runtimeDirectory: RUNTIME_DIR, runtimeVersion: VERSION, launch,
    authStatus: async () => { statusCalls.push(1); return readStatus(); } });
  return { runtime, launches, writes, statusCalls, get killed() { return killed; } };
}

test('the launch plan uses the official binary with every ambient feature off and no API key in the environment', () => {
  const saved = { ...process.env };
  process.env.ANTHROPIC_API_KEY = 'synthetic-key-must-not-pass'; process.env.CLAUDE_CODE_OAUTH_TOKEN = 'synthetic-token-must-not-pass';
  try {
    const plan = claudeCodeLaunch({ executable: '/synthetic/bin/claude', workspace: WORKSPACE, home: '/synthetic/home', runtimeDirectory: RUNTIME_DIR, model: MODEL });
    assert.equal(plan.executable, '/synthetic/bin/claude'); assert.equal(plan.cwd, WORKSPACE);
    assert.ok(!plan.args.includes('--bare'), '--bare forces API-key billing');
    for (const flag of ['--strict-mcp-config', '--disable-slash-commands', '--no-session-persistence']) assert.ok(plan.args.includes(flag), flag);
    assert.deepEqual(plan.args.slice(plan.args.indexOf('--tools'), plan.args.indexOf('--tools') + 2), ['--tools', '']);
    assert.deepEqual(plan.args.slice(plan.args.indexOf('--setting-sources'), plan.args.indexOf('--setting-sources') + 2), ['--setting-sources', '']);
    assert.deepEqual(plan.args.slice(plan.args.indexOf('--model'), plan.args.indexOf('--model') + 2), ['--model', MODEL]);
    assert.ok(Object.keys(plan.env).every(key => !/^ANTHROPIC_|OAUTH|API_KEY|^AWS_|^GOOGLE_|^CLAUDE_CODE_USE_/.test(key)), Object.keys(plan.env).join(','));
    assert.equal(plan.env.PATH, '/synthetic/bin:/usr/bin:/bin', 'an npm-installed claude finds node beside it');
    assert.equal(plan.env.HOME, '/synthetic/home'); assert.equal(plan.env.TMPDIR, RUNTIME_DIR); assert.equal(plan.env.XDG_RUNTIME_DIR, RUNTIME_DIR);
    assert.equal(plan.env.CLAUDE_CODE_TMPDIR, RUNTIME_DIR, 'on macOS Claude Code ignores TMPDIR and uses /tmp/claude-<uid>, which the App Sandbox denies');
    assert.equal(plan.env.CLAUDE_CODE_DISABLE_AUTO_MEMORY, '1'); assert.equal(plan.env.CLAUDE_CODE_DISABLE_MODEL_ACCESS_FALLBACK, '1');
    assert.equal(claudeCodeLaunch({ executable: '/synthetic/bin/claude', workspace: WORKSPACE, home: '/synthetic/home', runtimeDirectory: RUNTIME_DIR }).args.includes('--model'), false);
    assert.throws(() => claudeCodeLaunch({ executable: 'claude', workspace: WORKSPACE, home: '/synthetic/home', runtimeDirectory: RUNTIME_DIR }));
    assert.throws(() => claudeCodeLaunch({ executable: '/synthetic/bin/claude', workspace: WORKSPACE, home: '/synthetic/home', runtimeDirectory: '/synthetic/' + 'x'.repeat(80) }));
  } finally { process.env = saved; }
});

test('only a Claude subscription sign-in is an account; Console and API-key sign-ins are refused', () => {
  assert.deepEqual(account, { accountId: status.orgId, userLabel: status.email });
  assert.equal(parseClaudeAuthStatus({ loggedIn: false, authMethod: 'none', apiProvider: 'firstParty' }), null);
  for (const other of [{ ...status, authMethod: 'api_key' }, { ...status, apiKeySource: 'ANTHROPIC_API_KEY' }, { ...status, apiProvider: 'bedrock' }, { ...status, authMethod: 'oauth_token' }])
    assert.throws(() => parseClaudeAuthStatus(other));
});

// Claude Code 2.1.x prints this when it finds a claude.ai credential but no account record in this HOME, which is
// what Fort's private HOME holds after an earlier in-sandbox login: the credential is in the Keychain, not the HOME.
const unidentified = { loggedIn: true, authMethod: 'claude.ai', apiProvider: 'firstParty', analyticsDisabled: false, projectsDirectory: '/synthetic/home/.claude/projects',
  configDirectory: '/synthetic/home/.claude', email: null, orgId: null, orgName: null, subscriptionType: null };

test('a signed-out or unidentified Claude sign-in needs Sign in, and a refused one names the field, not its value', () => {
  assert.equal(parseClaudeAuthStatus(unidentified), null);
  assert.equal(parseClaudeAuthStatus({ ...status, orgId: 'not a uuid' }), null);
  assert.equal(parseClaudeAuthStatus({ ...status, email: undefined }), null);
  assert.equal(parseClaudeAuthStatus({ loggedIn: false, authMethod: 'none', apiProvider: 'firstParty', analyticsDisabled: false, configDirectory: '/synthetic/home/.claude' }), null);
  assert.equal(parseClaudeAuthStatus({ loggedIn: false, apiProvider: 'firstParty' }), null);
  assert.throws(() => parseClaudeAuthStatus({ ...status, authMethod: 'oauth_token' }), /authMethod oauth_token/);
  assert.throws(() => parseClaudeAuthStatus({ ...status, apiProvider: 'bedrock' }), /apiProvider bedrock/);
  assert.throws(() => parseClaudeAuthStatus({ ...status, apiKeySource: 'ANTHROPIC_API_KEY' }), /apiKeySource/);
  assert.throws(() => parseClaudeAuthStatus({ ...status, authMethod: 'Bad Value\nwith text' }), (error: Error) => !/Bad Value|\n/.test(error.message));
  assert.throws(() => parseClaudeAuthStatus({ loggedIn: false, apiKeySource: 'ANTHROPIC_API_KEY' }), /apiKeySource/, 'a key is refused even when signed out');
});

test('an unidentified Claude sign-in opens Claude\'s login page instead of failing silently', async () => {
  let signedIn = false;
  const lines: string[] = [];
  const f = fixture({ onCompletion: () => { signedIn = true; } }, () => signedIn ? status : unidentified), shown: unknown[] = [];
  const runtime = new ClaudeCodeChat({ ...(f.runtime as any).options, report: (line: string) => lines.push(line) });
  assert.deepEqual(await runtime.signIn({ durationMs: 60_000, onChallenge: challenge => { shown.push(challenge); } }, new AbortController().signal), { kind: 'signedIn', account });
  assert.deepEqual(shown, [{ kind: 'browserAuthorization', authorizationURL: LOGIN }]);
  assert.ok(lines.some(line => /without an account/.test(line)), lines.join('|'));
  assert.ok(!lines.some(line => line.includes('example.com') || line.includes('/synthetic/')), 'no account or path values in the log');
  const models = await fixture({}, unidentified).runtime.models(new AbortController().signal);
  assert.equal(models.state, 'unavailable', 'models ask for Sign in rather than reporting an error');
});

test('a turn sends the context with the system prompt off the command line and returns only the confirmed result', async () => {
  const f = fixture();
  const outcome = await f.runtime.execute(contextInput(base), new AbortController().signal, dispatch);
  assert.deepEqual(outcome, { kind: 'completed', text: 'cedar', effectiveModel: null });
  assert.equal(f.launches.length, 1); assert.equal(f.statusCalls.length, 1); assert.equal(f.killed, 1);
  assert.ok(!f.launches[0]!.args.some(arg => arg.includes('supplied context')));
  const [initRequest, user] = f.writes as [any, any];
  assert.equal(initRequest.request.subtype, 'initialize');
  assert.match(initRequest.request.systemPrompt, /^Use only the supplied context\./);
  assert.equal(user.type, 'user');
  const sent = JSON.parse(user.message.content[0].text);
  assert.equal(sent.currentMessage.text, 'What was the word?'); assert.equal(sent.history[0].text, 'Remember cedar.');
});

test('identity, account and model mismatches stop before any message reaches Claude', async () => {
  const cases: [string, Partial<Script>, unknown, Omit<ChatInput, 'context'>][] = [
    ['foreign run', {}, status, { ...base, attempt: { ...base.attempt, runId: 'other' } }],
    ['other account', {}, { ...status, email: 'someone@example.com' }, base],
    ['signed out', {}, { loggedIn: false, authMethod: 'none', apiProvider: 'firstParty' }, base],
    ['runtime drifted', { initialize: initialize({ claude_code_version: '2.1.311' }) }, status, base],
    ['token from environment', { initialize: initialize({ account: { email: status.email, subscriptionType: 'pro', tokenSource: 'CLAUDE_CODE_OAUTH_TOKEN', apiProvider: 'firstParty' } }) }, status, base],
    ['signed-out token source', { initialize: initialize({ account: { email: status.email, subscriptionType: 'pro', tokenSource: 'none', apiProvider: 'firstParty' } }) }, status, base],
    ['no subscription', { initialize: initialize({ account: { email: status.email, apiProvider: 'firstParty' } }) }, status, base],
    ['API key present', { initialize: initialize({ account: { email: status.email, subscriptionType: 'pro', apiKeySource: 'ANTHROPIC_API_KEY', apiProvider: 'firstParty' } }) }, status, base],
    ['model not offered', { initialize: initialize({ models: models.slice(0, 2) }) }, status, base],
    ['ambient command', { initialize: initialize({ commands: [{ name: 'deploy' }] }) }, status, base],
    ['remote control auto-on', { initialize: initialize({ remote_control_auto_enable: true }) }, status, base],
  ];
  for (const [name, script, accountStatus, input] of cases) {
    const f = fixture(script, accountStatus);
    const outcome = await f.runtime.execute(contextInput(input), new AbortController().signal, dispatch);
    assert.equal(outcome.kind, 'notStarted', name);
    assert.ok(!f.writes.some(frame => frame.type === 'user'), name);
  }
});

test('tools, permission prompts, model changes and unconfirmed results never produce a reply', async () => {
  const cases: [string, unknown[], string][] = [
    ['tool offered', [init({ tools: ['Bash'] }), result('x')], 'outcomeUnknown'],
    ['MCP server present', [init({ mcp_servers: [{ name: 'x', status: 'connected' }] }), result('x')], 'outcomeUnknown'],
    ['shared messaging socket', [init({ messaging_socket_path: '/tmp/cc-socks/1.sock' }), result('x')], 'outcomeUnknown'],
    ['permission prompt', [init(), { type: 'control_request', request_id: 'p1', request: { subtype: 'can_use_tool', tool_name: 'Bash' } }], 'outcomeUnknown'],
    ['tool use', [init(), assistant('x', { message: { id: 'msg-1', role: 'assistant', model: MODEL, content: [{ type: 'tool_use', id: 't', name: 'Bash', input: {} }] } }), result('x')], 'outcomeUnknown'],
    ['fallback model', [init(), assistant('x', { message: { id: 'msg-1', role: 'assistant', model: 'claude-haiku-5-5', content: [{ type: 'text', text: 'x' }] } }), result('x')], 'outcomeUnknown'],
    ['fallback notice', [init(), { type: 'system', subtype: 'model_fallback' }, assistant('x'), result('x')], 'outcomeUnknown'],
    ['session model differs', [init({ model: 'claude-opus-5-5' }), assistant('x'), result('x')], 'outcomeUnknown'],
    ['error marked success', [init(), assistant('Not logged in', { error: 'authentication_failed', message: { id: 'm', role: 'assistant', model: '<synthetic>', content: [{ type: 'text', text: 'Not logged in' }] } }), result('Not logged in', { is_error: true })], 'failed'],
    ['result without assistant text', [init(), result('cedar')], 'outcomeUnknown'],
    ['result text disagrees', [init(), assistant('cedar'), result('pine')], 'outcomeUnknown'],
    ['stream ended', [init(), assistant('cedar')], 'outcomeUnknown'],
    ['unknown frame', [init(), { type: 'tool_progress' }, assistant('cedar'), result('cedar')], 'outcomeUnknown'],
  ];
  for (const [name, frames, kind] of cases) {
    const f = fixture({ onUser: () => frames, exitAfterUser: name === 'stream ended' });
    const outcome = await f.runtime.execute(contextInput(base), new AbortController().signal, dispatch);
    assert.equal(outcome.kind, kind, name);
    assert.equal(f.killed, 1, name);
  }
});

test('cancel sends an interrupt and reports cancellation only when Claude confirms it', async () => {
  for (const [confirm, kind] of [[true, 'canceled'], [false, 'outcomeUnknown']] as const) {
    const controller = new AbortController();
    const f = fixture({ onUser: () => { queueMicrotask(() => controller.abort()); return [init()]; },
      onInterrupt: () => confirm ? [result('', { subtype: 'error_during_execution', is_error: true })] : [] });
    const outcome = await f.runtime.execute(contextInput(base), controller.signal, { deadline: Date.now() + 120_000 } as ExecutionDispatch);
    assert.equal(outcome.kind, kind);
    assert.ok(f.writes.some((frame: any) => frame.request?.subtype === 'interrupt'));
    assert.equal(f.killed, 1);
  }
});

test('models come from the signed-in runtime, deduplicated to exact model IDs', async () => {
  const f = fixture();
  const catalog = await f.runtime.models(new AbortController().signal, account);
  assert.equal(catalog.state, 'ready');
  assert.deepEqual(catalog.models.map(model => [model.id, model.name]), [['claude-opus-5-5', 'Opus'], [MODEL, 'Sonnet']]);
  assert.equal(f.launches[0]!.args.includes('--model'), false); assert.equal(f.killed, 1);
  const signedOut = await fixture({}, { loggedIn: false, authMethod: 'none', apiProvider: 'firstParty' }).runtime.models(new AbortController().signal);
  assert.equal(signedOut.state, 'unavailable'); assert.match(signedOut.state === 'unavailable' ? signedOut.detail : '', /Sign in/);
  const other = await fixture().runtime.models(new AbortController().signal, { accountId: status.orgId, userLabel: 'someone@example.com' });
  assert.equal(other.state, 'error');
});

test('without an injected reader, the account comes from `claude auth status --json` run with the isolated environment', async t => {
  const { mkdtemp, writeFile, rm, readFile } = await import('node:fs/promises');
  const { tmpdir } = await import('node:os');
  const { join } = await import('node:path');
  const root = await mkdtemp(join(tmpdir(), 'fcc-')); t.after(() => rm(root, { recursive: true, force: true }));
  const fake = join(root, 'claude'), seen = join(root, 'seen.json');
  const saved = process.env.ANTHROPIC_API_KEY; process.env.ANTHROPIC_API_KEY = 'synthetic-key-must-not-pass';
  t.after(() => { if (saved === undefined) delete process.env.ANTHROPIC_API_KEY; else process.env.ANTHROPIC_API_KEY = saved; });
  const script = (body: string) => writeFile(fake, `#!${process.execPath}\nrequire('node:fs').writeFileSync(${JSON.stringify(seen)}, JSON.stringify({ args: process.argv.slice(2), env: process.env }));\n${body}\n`, { mode: 0o755 });
  const chat = () => new ClaudeCodeChat({ executable: fake, workspace: root, home: root, runtimeDirectory: root, runtimeVersion: VERSION, launch: () => { throw new Error('no session expected'); } });

  await script(`process.stdout.write(JSON.stringify({ loggedIn: false, authMethod: 'none', apiProvider: 'firstParty' })); process.exitCode = 1;`);
  const signedOut = await chat().models(new AbortController().signal);
  assert.equal(signedOut.state, 'unavailable');
  const observed = JSON.parse(await readFile(seen, 'utf8'));
  assert.deepEqual(observed.args, ['auth', 'status', '--json']);
  assert.equal(observed.env.ANTHROPIC_API_KEY, undefined); assert.equal(observed.env.HOME, root);

  await script(`process.stdout.write('x'.repeat(100000));`);
  assert.equal((await chat().models(new AbortController().signal)).state, 'error');
  await script(`setTimeout(() => {}, 60000);`);
  const controller = new AbortController(); setTimeout(() => controller.abort(), 200);
  assert.equal((await chat().models(controller.signal)).state, 'error');
});

test('initialize problems are reported by check name, without values', () => {
  assert.deepEqual(initializeProblems(initialize(), VERSION, account), []);
  assert.deepEqual(initializeProblems(initialize({ analytics_disabled: false, remote_control_auto_on_by_default: undefined, models: 'x' }), VERSION, account),
    ['analytics_disabled', 'remote_control_auto_on_by_default', 'models']);
  assert.deepEqual(initializeProblems(null, VERSION, account), ['shape']);
});

test('sign-in runs Claude Code\'s own claude.ai login and shows only its browser page', async () => {
  let signedIn = false;
  const f = fixture({ onCompletion: () => { signedIn = true; } }, () => signedIn ? status : signedOutStatus), shown: unknown[] = [];
  const outcome = await f.runtime.signIn({ durationMs: 60_000, onChallenge: challenge => { shown.push(challenge); } }, new AbortController().signal);
  assert.deepEqual(outcome, { kind: 'signedIn', account });
  assert.deepEqual(shown, [{ kind: 'browserAuthorization', authorizationURL: LOGIN }]);
  assert.deepEqual(f.writes.map((frame: any) => frame.request?.subtype), ['initialize', 'claude_authenticate', 'claude_oauth_wait_for_completion']);
  assert.equal((f.writes[1] as any).request.loginWithClaudeAi, true, 'never a Console or API-key login');
  assert.equal(f.launches[0]!.args.includes('--model'), false); assert.equal(f.killed, 1);
  assert.equal(f.statusCalls.length, 2, 'the account is read back from the runtime, not from the login reply');
});

test('an existing Claude sign-in is reported without starting a login', async () => {
  const f = fixture();
  assert.deepEqual(await f.runtime.signIn({ durationMs: 60_000, onChallenge: () => assert.fail('no challenge') }, new AbortController().signal), { kind: 'signedIn', account });
  assert.equal(f.launches.length, 0);
});

test('sign-in refuses foreign login pages, drifted runtimes and non-subscription results', async () => {
  const cases: [string, Script, () => unknown][] = [
    ['foreign page', { authenticate: { automaticUrl: 'https://example.com/cai/oauth/authorize?x=1', manualUrl: LOGIN } }, () => signedOutStatus],
    ['plain http', { authenticate: { automaticUrl: LOGIN.replace('https:', 'http:'), manualUrl: LOGIN } }, () => signedOutStatus],
    ['runtime drifted', { initialize: initialize({ claude_code_version: '2.1.311', account: {} }) }, () => signedOutStatus],
    ['remote control auto-on', { initialize: initialize({ remote_control_auto_enable: true, account: {} }) }, () => signedOutStatus],
  ];
  for (const [name, script, read] of cases) {
    const f = fixture(script, read), shown: unknown[] = [];
    assert.deepEqual(await f.runtime.signIn({ durationMs: 60_000, onChallenge: value => { shown.push(value); } }, new AbortController().signal), { kind: 'unconfirmed' }, name);
    assert.deepEqual(shown, [], name); assert.equal(f.killed, 1, name);
    assert.ok(!f.writes.some((frame: any) => frame.request?.subtype === 'claude_oauth_wait_for_completion'), name);
  }
  let calls = 0;
  const apiKey = fixture({}, () => ++calls === 1 ? signedOutStatus : { ...status, authMethod: 'api_key' });
  assert.deepEqual(await apiKey.runtime.signIn({ durationMs: 60_000, onChallenge: () => {} }, new AbortController().signal), { kind: 'unconfirmed' });
  const denied = fixture({ completion: 'error' }, () => signedOutStatus);
  assert.deepEqual(await denied.runtime.signIn({ durationMs: 60_000, onChallenge: () => {} }, new AbortController().signal), { kind: 'unconfirmed' });
});

test('a failed sign-in step is reported by name for the host log, without runtime output', async () => {
  const lines: string[] = [];
  const f = fixture({ authenticate: { automaticUrl: 'https://example.com/cai/oauth/authorize?x=1', manualUrl: LOGIN } }, () => signedOutStatus);
  const runtime = new ClaudeCodeChat({ ...(f.runtime as any).options, report: (line: string) => lines.push(line) });
  assert.deepEqual(await runtime.signIn({ durationMs: 60_000, onChallenge: () => {} }, new AbortController().signal), { kind: 'unconfirmed' });
  assert.equal(lines.length, 1); assert.match(lines[0]!, /^Claude Code sign-in page failed: /);
  assert.ok(!lines[0]!.includes('example.com'), 'the refused page is not echoed');
});

test('when Claude Code ends before answering, the host log says how it ended', async () => {
  const lines: string[] = [];
  const runtime = new ClaudeCodeChat({ executable: '/synthetic/bin/claude', workspace: WORKSPACE, home: '/synthetic/home', runtimeDirectory: RUNTIME_DIR, runtimeVersion: VERSION,
    authStatus: async () => signedOutStatus, report: line => lines.push(line),
    launch: () => { const stdout = new PassThrough(); setImmediate(() => stdout.end()); return { stdin: new PassThrough(), stdout, kill() {}, ended: Promise.resolve('signal SIGTRAP') }; } });
  assert.deepEqual(await runtime.signIn({ durationMs: 60_000, onChallenge: () => assert.fail('no page') }, new AbortController().signal), { kind: 'unconfirmed' });
  assert.deepEqual(lines, ['Claude Code sign-in start failed: Claude Code control request was not confirmed (Claude Code signal SIGTRAP)']);
});

test('a status frame before Claude answers no longer ends sign-in, and a refusal is logged in Claude\'s words without account values', async () => {
  let signedIn = false;
  const f = fixture({ onCompletion: () => { signedIn = true; }, beforeInitialize: [{ type: 'system', subtype: 'status', status: 'starting' }, { type: 'auth_status', isAuthenticating: false, output: [] }] },
    () => signedIn ? status : signedOutStatus), shown: unknown[] = [];
  assert.deepEqual(await f.runtime.signIn({ durationMs: 60_000, onChallenge: challenge => { shown.push(challenge); } }, new AbortController().signal), { kind: 'signedIn', account });
  assert.deepEqual(shown, [{ kind: 'browserAuthorization', authorizationURL: LOGIN }]);
  const lines: string[] = [];
  const refused = fixture({ initializeError: 'Not logged in as owner@example.com \u00b7 run /login', beforeInitialize: [{ type: 'system', subtype: 'status' }] }, () => signedOutStatus);
  const runtime = new ClaudeCodeChat({ ...(refused.runtime as any).options, report: (line: string) => lines.push(line) });
  assert.deepEqual(await runtime.signIn({ durationMs: 60_000, onChallenge: () => assert.fail('no page') }, new AbortController().signal), { kind: 'unconfirmed' });
  assert.deepEqual(lines, ['Claude Code sign-in start failed: Claude Code control request was not confirmed (Claude Code skipped a system/status frame; refused a request: Not logged in as [email]   run /login)']);
});

test('canceling or timing out a pending sign-in stops Claude Code and reports cancellation', async () => {
  const controller = new AbortController();
  const f = fixture({ completion: 'never' }, () => signedOutStatus);
  const outcome = await f.runtime.signIn({ durationMs: 60_000, onChallenge: () => { setTimeout(() => controller.abort(), 10); } }, controller.signal);
  assert.deepEqual(outcome, { kind: 'canceled' }); assert.equal(f.killed, 1);
  const slow = fixture({ completion: 'never' }, () => signedOutStatus);
  assert.deepEqual(await slow.runtime.signIn({ durationMs: 50, onChallenge: () => {} }, new AbortController().signal), { kind: 'canceled' });
  assert.equal(slow.killed, 1);
});
