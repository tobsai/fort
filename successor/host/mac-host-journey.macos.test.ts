import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { spawn, spawnSync, type ChildProcess } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { randomBytes, randomUUID } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import { buildHostResources } from '../packaging/host-resources.ts';

// The whole local-host journey Toby walks through on his Mac (Specs 062, 064), run against the real bundled host
// (main.mjs, the compiled core and the real Mac guardian) with a stand-in Claude Code: turn Claude on, the host
// restarts by itself, Claude appears beside ChatGPT, sign-in shows Claude's login page, models load, a Claude Agent
// replies, and turning Claude off restarts the host without it. Nothing reaches the network or a real account.
//
// The host looks for Claude Code in the user's real ~/.local/share/claude/versions, so the stand-in goes there. It
// runs only where that folder doesn't exist yet (a CI runner), never beside a real install.
const source = fileURLToPath(new URL('../', import.meta.url));
const installation = join(homedir(), '.local/share/claude');
const skip = process.platform !== 'darwin' ? 'needs macOS' : existsSync(installation) ? 'a real Claude Code install is present' : false;
const VERSION = '9.9.9';

function layout(t: TestContext) {
  const root = realpathSync(mkdtempSync('/tmp/fort-journey-'));
  t.after(() => { rmSync(root, { recursive: true, force: true }); rmSync(installation, { recursive: true, force: true }); });
  const contents = join(root, 'Fort.app/Contents'), macos = join(contents, 'MacOS'), host = join(contents, 'Resources/host');
  mkdirSync(macos, { recursive: true }); mkdirSync(join(contents, 'Resources'), { recursive: true });
  buildHostResources(host);
  const compiled = spawnSync('/usr/bin/xcrun', ['swiftc', join(source, 'host/mac-process-guardian.swift'), '-o', join(macos, 'FortProcessGuardian')], { encoding: 'utf8', timeout: 120_000 });
  assert.equal(compiled.status, 0, compiled.stderr);
  // Codex is never started in this journey; ChatGPT stays the primary runtime as on Toby's Mac.
  writeFileSync(join(macos, 'codex'), '#!/bin/sh\nexit 1\n', { mode: 0o755 });
  const data = join(root, 'data'), temporary = join(root, 'tmp');
  for (const path of [data, join(data, 'Runtime'), ...['home', 'state', 'workspace', 'tmp'].map(name => join(data, 'Runtime', name)), temporary]) mkdirSync(path, { mode: 0o700 });
  const versions = join(installation, 'versions');
  mkdirSync(versions, { recursive: true, mode: 0o755 });
  writeFileSync(join(versions, VERSION), `#!${process.execPath}\nrequire(${JSON.stringify(join(source, 'host/fixtures/fake-claude-code.cjs'))});\n`, { mode: 0o755 });
  return { main: join(host, 'main.mjs'), openssl: join(host, 'node-openssl.cnf'), data, temporary };
}

function decode(frame: Record<string, any>, status = 200) {
  assert.equal(frame.kind, 'response', JSON.stringify(frame));
  assert.equal(frame.response.status, status, Buffer.from(frame.response.body, 'base64').toString());
  return JSON.parse(Buffer.from(frame.response.body, 'base64').toString());
}

// Starts the host the way the Mac helper does: the same Node flags, the data directory as its working directory and
// only TMPDIR in its environment.
async function start(paths: ReturnType<typeof layout>) {
  const child: ChildProcess = spawn(process.execPath, ['--jitless', '--no-experimental-strip-types', '--no-warnings', '--openssl-config=' + paths.openssl, paths.main],
    { cwd: paths.data, env: { TMPDIR: paths.temporary }, detached: true, stdio: ['pipe', 'pipe', 'pipe'] });
  const exited = new Promise<number | null>(resolve => child.once('exit', code => resolve(code)));
  const pending = new Map<number, (frame: Record<string, any>) => void>();
  let buffer = '', diagnostics = '', next = 0;
  const ready = Promise.withResolvers<void>();
  child.stderr!.on('data', chunk => { diagnostics = (diagnostics + chunk).slice(-4000); });
  child.stdout!.setEncoding('utf8');
  child.stdout!.on('data', chunk => {
    buffer += chunk;
    for (let end = buffer.indexOf('\n'); end >= 0; end = buffer.indexOf('\n')) {
      const frame = JSON.parse(buffer.slice(0, end)); buffer = buffer.slice(end + 1);
      if (frame.kind === 'ready') ready.resolve(); else pending.get(frame.id)?.(frame);
    }
  });
  await Promise.race([ready.promise, exited.then(code => { throw new Error(`Host exited (${code}) before ready: ${diagnostics}`); }), delay(20_000).then(() => { throw new Error('Host startup timed out'); })]);
  function send(kind: string, key: string, value: unknown) {
    const id = ++next;
    const reply = new Promise<Record<string, any>>(resolve => pending.set(id, resolve));
    child.stdin!.write(JSON.stringify({ protocolVersion: 1, kind, id, [key]: value }) + '\n');
    return Promise.race([reply, exited.then(() => { throw new Error('Host exited: ' + diagnostics); }), delay(30_000).then(() => { throw new Error('Host request timed out'); })]);
  }
  return {
    exited,
    setup: (value: Record<string, unknown>) => send('ownerSetup', 'setup', { timeout: 10, ...value }).then(frame => decode(frame)),
    api: async (owner: { installationId: string; token: string }, path: string, body?: unknown, status = 200) => decode(await send('request', 'request', {
      path, method: body === undefined ? 'GET' : 'POST', installationID: owner.installationId, token: owner.token, timeout: 25,
      body: body === undefined ? undefined : Buffer.from(JSON.stringify(body)).toString('base64') }), status),
    async close() {
      child.stdin!.end();
      const code = await Promise.race([exited, delay(10_000).then(() => { if (child.pid) try { process.kill(-child.pid, 'SIGKILL'); } catch {} return 'killed'; })]);
      assert.equal(code, 0, diagnostics);
    },
  };
}

async function until<T>(read: () => Promise<T>, done: (value: T) => boolean, what: string): Promise<T> {
  let value = await read();
  for (let index = 0; index < 150 && !done(value); index++) { await delay(100); value = await read(); }
  assert.ok(done(value), what + ': ' + JSON.stringify(value));
  return value;
}

test('turning Claude on and off restarts the bundled host by itself, and a Claude Agent signs in and replies', { skip, timeout: 240_000 }, async t => {
  const paths = layout(t);
  let host = await start(paths);
  const setup = await host.setup({ operation: 'begin' });
  assert.equal(setup.kind, 'available');
  const owner = { installationId: setup.installationId as string, token: randomBytes(32).toString('base64url') };
  await host.setup({ operation: 'complete', installationId: setup.installationId, setupId: setup.setupId, deviceId: 'journey-mac', deviceToken: owner.token });
  const providers = async () => ((await host.api(owner, '/v1/installation')).runtimes ?? []).map((runtime: { provider: string }) => runtime.provider);
  assert.deepEqual(await providers(), ['codex']);

  // "Turn on Claude": the app writes the selection file. The host ends by itself, and the next start serves Claude.
  const claude = join(paths.data, 'ClaudeCode'); mkdirSync(claude, { mode: 0o700 });
  writeFileSync(join(claude, 'selection.json'), '{}\n', { mode: 0o600 });
  assert.equal(await Promise.race([host.exited, delay(10_000).then(() => 'still running')]), 0, 'the host ends cleanly when Claude is turned on');
  host = await start(paths);
  assert.deepEqual(await providers(), ['codex', 'claude-code']);
  const installed = await host.api(owner, '/v1/installation');
  assert.equal(installed.runtimes[1].runtimeVersion, VERSION);

  // Sign in to Claude: the host shows Claude's own login page, then reports the account once the login completes.
  const signIn = (suffix = '', body?: unknown) => host.api(owner, '/v1/account/sign-in' + suffix + '?provider=claude-code', body);
  assert.equal((await signIn()).state, 'idle');
  const request = { operationId: randomUUID(), expectedRevision: (await signIn()).revision, recoveryEpoch: installed.recoveryEpoch, expiresAt: Date.now() + 100_000 };
  await signIn('/start', request);
  const challenged = await until(() => signIn(), status => status.state !== 'idle' && status.state !== 'starting', 'sign-in progressed');
  if (challenged.state === 'challenge') assert.match(JSON.stringify(challenged.challenge), /claude\.com\/cai\/oauth\/authorize/);
  const finished = await until(() => signIn(), status => status.state === 'finished', 'sign-in finished');
  assert.equal(finished.result?.kind, 'signedIn', JSON.stringify(finished));
  if (!finished.selected) await signIn('/select', { operationId: request.operationId, revision: finished.revision, recoveryEpoch: installed.recoveryEpoch, account: finished.result.account });
  assert.equal((await signIn()).selected, true);
  // ChatGPT's own sign-in is untouched.
  assert.notEqual((await host.api(owner, '/v1/account/sign-in')).selected, true);

  const catalog = await host.api(owner, '/v1/models?provider=claude-code');
  assert.equal(catalog.state, 'ready', JSON.stringify(catalog));
  assert.ok(catalog.models.some((model: { id: string }) => model.id === 'claude-sonnet-5-5'));

  // A new Claude Agent replies through Claude, not ChatGPT.
  const agent = await host.api(owner, '/v1/agents', { name: 'Claude journey', model: 'claude-sonnet-5-5', instructions: 'Synthetic', provider: 'claude-code' });
  assert.equal(agent.binding.provider, 'claude-code');
  await host.api(owner, '/v1/conversations/' + agent.homeConversationId + '/messages', { requestId: randomUUID(), text: 'Synthetic journey prompt', recipientAgentId: agent.agentId,
    bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: installed.recoveryEpoch, skills: [] }, 201);
  const snapshot = await until(() => host.api(owner, '/v1/conversations/' + agent.homeConversationId + '/snapshot'),
    value => value.messages.some((message: { text: string }) => message.text === 'Synthetic Claude reply') || value.runs?.some((run: { state: string }) => ['failed', 'notStarted', 'outcomeUnknown'].includes(run.state)), 'Claude replied');
  assert.ok(snapshot.messages.some((message: { text: string }) => message.text === 'Synthetic Claude reply'), JSON.stringify(snapshot.runs));

  // "Turn off Claude": the host restarts without it, and the Claude Agent is kept.
  rmSync(join(claude, 'selection.json'));
  assert.equal(await Promise.race([host.exited, delay(10_000).then(() => 'still running')]), 0, 'the host ends cleanly when Claude is turned off');
  host = await start(paths);
  assert.deepEqual(await providers(), ['codex']);
  assert.ok((await host.api(owner, '/v1/agents')).agents.some((saved: { agentId: string }) => saved.agentId === agent.agentId));
  await host.close();
});
