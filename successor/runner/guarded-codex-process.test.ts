import { before, after, test } from 'node:test';
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, writeFileSync, rmSync, realpathSync } from 'node:fs';
import { setTimeout as delay } from 'node:timers/promises';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { launchGuardedCodexProcess } from './guarded-codex-process.ts';

const root = realpathSync(mkdtempSync('/tmp/fort059-guarded-codex-'));
const guardian = join(root, 'guardian');
const runtime = fileURLToPath(new URL('./fixtures/guarded-codex-runtime.mjs', import.meta.url));
before(() => {
  if (process.platform !== 'darwin') return;
  const result = spawnSync('/usr/bin/xcrun', ['swiftc', fileURLToPath(new URL('../host/mac-process-guardian.swift', import.meta.url)), '-o', guardian], { encoding: 'utf8', timeout: 60_000 });
  assert.equal(result.status, 0, result.stderr);
});
after(() => rmSync(root, { recursive: true, force: true }));

function resultPID(value: unknown, key: string): number {
  assert.ok(value && typeof value === 'object' && key in value);
  const pid: unknown = Reflect.get(value, key);
  assert.ok(typeof pid === 'number' && Number.isSafeInteger(pid) && pid > 1);
  return pid;
}
function absent(pid: number) {
  try { process.kill(pid, 0); return false; }
  catch (error) { return error instanceof Error && 'code' in error && error.code === 'ESRCH'; }
}

test('guarded Codex transport preserves RPC and confirms the complete synthetic runtime group absent', { skip: process.platform !== 'darwin' }, async t => {
  const owned = launchGuardedCodexProcess({ guardian, executable: process.execPath, args: [runtime], cwd: root, env: {}, deadlineMs: 5000 });
  t.after(() => owned.dispose());
  const result = await owned.connection.request('ping', { text: '  exact synthetic 🌲\n' }, new AbortController().signal);
  const runtimePID = resultPID(result, 'pid'), guardianPID = resultPID(result, 'guardian');
  assert.notEqual(guardianPID, process.pid);
  assert.notEqual(guardianPID, runtimePID);
  assert.equal(absent(-guardianPID), false);
  assert.ok(result && typeof result === 'object' && 'echo' in result);
  assert.deepEqual(result.echo, { text: '  exact synthetic 🌲\n' });
  await owned.dispose(); await owned.dispose();
  assert.equal(owned.cleanupConfirmed, true);
  assert.ok([guardianPID, runtimePID, -guardianPID].every(absent));
});

async function until(predicate: () => boolean, milliseconds: number) {
  const deadline = performance.now() + milliseconds;
  while (!predicate() && performance.now() < deadline) await delay(10);
  return predicate();
}
function options(scenario = 'normal') {
  return { guardian, executable: process.execPath, args: [runtime, scenario], cwd: root, env: {}, deadlineMs: 5000 };
}

test('guarded runtime launch refuses invalid exact scope before returning a process', { skip: process.platform !== 'darwin' }, () => {
  for (const field of ['guardian', 'executable', 'cwd'] as const) {
    for (const invalid of ['relative', '/', '/tmp/../runtime', '/invalid\0runtime', '/invalid' + String.fromCharCode(0xd800), '/' + 'x'.repeat(4096)]) {
      assert.throws(() => launchGuardedCodexProcess({ ...options(), [field]: invalid }), /exact normalized/);
    }
  }
  for (const deadlineMs of [0, -1, 1.5, NaN, Infinity, 120001]) assert.throws(() => launchGuardedCodexProcess({ ...options(), deadlineMs }), /explicit runtime deadline/);
  for (const args of [Array.from({ length: 257 }, () => 'x'), ['x'.repeat(65536)], ['invalid\0argument'], [String.fromCharCode(0xd800)]]) {
    assert.throws(() => launchGuardedCodexProcess({ ...options(), args }), /arguments exceed/);
  }
  for (const env of [{ '': 'value' }, { 'bad=key': 'value' }, { valid: 'bad\0value' }, { valid: String.fromCharCode(0xd800) }, { valid: 'x'.repeat(65536) }, Object.fromEntries(Array.from({ length: 129 }, (_, index) => ['KEY_' + index, 'x']))]) {
    assert.throws(() => launchGuardedCodexProcess({ ...options(), env }), /environment exceeds/);
  }
});

test('guarded runtime launch uses only its explicit environment and never falls back from a missing executable', { skip: process.platform !== 'darwin' }, async t => {
  const prior = process.env.FORT_GUARDED_INHERITANCE_PROBE;
  process.env.FORT_GUARDED_INHERITANCE_PROBE = 'synthetic';
  t.after(() => { if (prior === undefined) delete process.env.FORT_GUARDED_INHERITANCE_PROBE; else process.env.FORT_GUARDED_INHERITANCE_PROBE = prior; });
  const owned = launchGuardedCodexProcess({ ...options(), env: { FORT_GUARDED_EXPLICIT: 'synthetic' } });
  t.after(() => owned.dispose());
  const result = await owned.connection.request('ping', {}, new AbortController().signal);
  assert.ok(result && typeof result === 'object' && 'environment' in result && Array.isArray(result.environment));
  assert.ok(result.environment.includes('FORT_GUARDED_EXPLICIT'));
  assert.ok(result.environment.every(key => key === 'FORT_GUARDED_EXPLICIT' || key === '__CF_USER_TEXT_ENCODING'));
  await owned.dispose();
  for (const field of ['guardian', 'executable'] as const) {
    const missing = launchGuardedCodexProcess({ ...options(), [field]: join(root, 'missing-' + field) });
    try { await assert.rejects(missing.connection.request('ping', {}, new AbortController().signal)); }
    finally { await missing.dispose(); }
    assert.equal(missing.cleanupConfirmed, true);
    assert.notEqual(missing.exit?.code, 0);
  }
});

test('guarded runtime disposal reaps noncooperative children and their inherited descendants', { skip: process.platform !== 'darwin' }, async t => {
  const owned = launchGuardedCodexProcess(options('ignore-term'));
  t.after(() => owned.dispose());
  const result = await owned.connection.request('ping', {}, new AbortController().signal);
  const pids = ['pid', 'guardian', 'descendant'].map(key => resultPID(result, key));
  const group = resultPID(result, 'guardian');
  assert.ok(pids.every(pid => !absent(pid)));
  const started = performance.now();
  await owned.dispose();
  const elapsed = performance.now() - started;
  assert.ok(elapsed < 1500, 'noncooperative runtime cleanup exceeded its TERM/KILL reserve');
  assert.equal(owned.cleanupConfirmed, true);
  assert.ok([...pids, -group].every(absent));
  t.diagnostic(JSON.stringify({ pids, group, cleanupMs: elapsed }));
});

test('the explicit runtime deadline closes idle RPC and confirms its group absent', { skip: process.platform !== 'darwin' }, async t => {
  const owned = launchGuardedCodexProcess({ ...options(), deadlineMs: 350 });
  t.after(() => owned.dispose());
  const result = await owned.connection.request('ping', {}, new AbortController().signal);
  assert.ok(await until(() => owned.cleanupConfirmed, 2000));
  assert.equal(owned.connection.isClosed, true);
  const group = resultPID(result, 'guardian');
  assert.ok([resultPID(result, 'pid'), group, -group].every(absent));
  await assert.rejects(owned.connection.request('ping', {}, new AbortController().signal));
});

test('abrupt direct-parent death removes the separate guarded runtime group without a JavaScript cleanup handler', { skip: process.platform !== 'darwin', timeout: 12000 }, async t => {
  const directory = realpathSync(mkdtempSync('/tmp/fort059-runtime-parent-death-'));
  const host = spawn(process.execPath, [fileURLToPath(new URL('./fixtures/guarded-codex-host.mjs', import.meta.url)), directory, guardian], { cwd: directory, env: {}, detached: true, stdio: 'ignore' });
  const hostPID = host.pid;
  assert.ok(hostPID && hostPID > 1);
  let group: number | undefined, complete = false;
  t.after(async () => {
    if (!complete) {
      if (host.exitCode === null && host.signalCode === null) host.kill('SIGKILL');
      if (group && !absent(-group)) { try { process.kill(-group, 'SIGKILL'); } catch {} }
      assert.ok(await until(() => absent(hostPID) && (!group || absent(-group)), 2000), 'synthetic cleanup remains unconfirmed');
    }
    if (complete) rmSync(directory, { recursive: true, force: true });
    else t.diagnostic('Retained synthetic failure directory: ' + directory);
  });
  assert.ok(await until(() => existsSync(join(directory, 'ready.json')), 5000), 'synthetic host did not become ready');
  const ready: unknown = JSON.parse(readFileSync(join(directory, 'ready.json'), 'utf8'));
  assert.equal(resultPID(ready, 'host'), hostPID);
  const observedGroup = resultPID(ready, 'guardian');
  group = observedGroup;
  assert.notEqual(group, hostPID);
  const pids = [hostPID, group, resultPID(ready, 'pid'), resultPID(ready, 'descendant')];
  assert.ok(pids.every(pid => !absent(pid)) && !absent(-group));
  const started = performance.now();
  host.kill('SIGKILL');
  assert.ok(await until(() => [...pids, -observedGroup, -hostPID].every(absent), 3000), 'a guarded runtime process survived direct-parent death');
  complete = true;
  t.diagnostic(JSON.stringify({ pids, group, cleanupMs: performance.now() - started }));
});

test('guarded runtime synthetic launch ping and disposal meet their isolated budget', { skip: process.platform !== 'darwin' || !process.env.FORT_GUARDED_CODEX_BENCHMARK }, async () => {
  const destination = process.env.FORT_GUARDED_CODEX_BENCHMARK;
  assert.ok(destination);
  const samples: number[] = [];
  for (let index = 0; index < 20; index++) {
    const started = performance.now();
    const owned = launchGuardedCodexProcess(options());
    try {
      const result = await owned.connection.request('ping', { index }, new AbortController().signal);
      await owned.dispose();
      const elapsed = performance.now() - started;
      const group = resultPID(result, 'guardian');
      assert.ok(owned.cleanupConfirmed && [resultPID(result, 'pid'), group, -group].every(absent));
      samples.push(elapsed);
    } finally { await owned.dispose(); }
  }
  writeFileSync(destination, JSON.stringify(samples));
  const ordered = [...samples].sort((a, b) => a - b);
  const p95 = ordered[18];
  assert.ok(p95 !== undefined && p95 <= 2000);
});
