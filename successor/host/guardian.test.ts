import { before, after, test } from 'node:test';
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';

const source = fileURLToPath(new URL('../', import.meta.url));
const build = mkdtempSync('/tmp/fort059-guardian-build-');
const supervisor = join(build, 'supervisor');
const guardian = join(build, 'guardian');
before(() => {
  if (process.platform !== 'darwin') return;
  const sources = ['FortTransport.swift', 'MacOwnerControl.swift', 'OwnerSetup.swift', 'MacCoreProcessTransport.swift']
    .map(file => join(source, 'apple/FortKit/Sources/FortKit', file));
  const compiled = spawnSync('/usr/bin/xcrun', ['swiftc', ...sources, join(source, 'host/fixtures/core-supervisor.swift'), '-o', supervisor], { encoding: 'utf8', timeout: 60_000 });
  assert.equal(compiled.status, 0, compiled.stderr);
  const guarded = spawnSync('/usr/bin/xcrun', ['swiftc', join(source, 'host/mac-process-guardian.swift'), '-o', guardian], { encoding: 'utf8', timeout: 60_000 });
  assert.equal(guarded.status, 0, guarded.stderr);
});
after(() => rmSync(build, { recursive: true, force: true }));

const absent = (pid: number) => {
  try { process.kill(pid, 0); return false; }
  catch (error) { return error instanceof Error && 'code' in error && error.code === 'ESRCH'; }
};
async function until(predicate: () => boolean, milliseconds: number) {
  const deadline = performance.now() + milliseconds;
  while (!predicate() && performance.now() < deadline) await delay(10);
  return predicate();
}
function readPid(path: string): number {
  const value: unknown = JSON.parse(readFileSync(path, 'utf8'));
  assert.ok(typeof value === 'number' && Number.isSafeInteger(value) && value > 1);
  return value;
}
function readGroup(directory: string): number {
  const value: unknown = JSON.parse(readFileSync(join(directory, 'supervisor-ready.json'), 'utf8'));
  assert.ok(value && typeof value === 'object' && 'group' in value && typeof value.group === 'number' && value.group > 1);
  return value.group;
}

test('abrupt Swift supervisor death removes its noncooperative runtime and descendant group', { skip: process.platform !== 'darwin', timeout: 15_000 }, async t => {
  const directory = mkdtempSync('/tmp/fort059-guardian-kill-');
  const child = spawn(supervisor, [process.execPath, join(source, 'host/fixtures/native-channel.mjs'), directory, 'hang', guardian], { env: { PATH: '/usr/bin:/bin' }, stdio: 'ignore' });
  const pid = child.pid;
  assert.ok(pid && pid > 1);
  let group: number | undefined;
  t.after(async () => {
    for (const target of [pid, ...(group ? [-group] : [])]) { try { process.kill(target, 'SIGKILL'); } catch {} }
    assert.ok(await until(() => absent(pid) && (!group || absent(-group)), 2000), 'fixture cleanup was not confirmed');
    rmSync(directory, { recursive: true, force: true });
  });
  assert.ok(await until(() => existsSync(join(directory, 'supervisor-ready.json')) && existsSync(join(directory, 'descendant-pid.json')), 6000));
  group = readGroup(directory);
  const runtime = readPid(join(directory, 'fixture-pid.json'));
  const descendant = readPid(join(directory, 'descendant-pid.json'));
  assert.equal(absent(runtime), false);
  assert.equal(absent(descendant), false);
  process.kill(pid, 'SIGKILL');
  const started = performance.now();
  assert.ok(await until(() => [pid, group, runtime, descendant, -group].every(absent), 3000), 'owned processes survived abrupt supervisor exit');
  t.diagnostic(JSON.stringify({ supervisor: pid, group, runtime, descendant, cleanupMs: performance.now() - started }));
});

test('guarded protocol requests close cleanly without held pipe descriptors or residual processes', { skip: process.platform !== 'darwin', timeout: 15_000 }, () => {
  const directory = mkdtempSync('/tmp/fort059-guardian-close-');
  try {
    const result = spawnSync(supervisor, [process.execPath, join(source, 'host/fixtures/native-channel.mjs'), directory, 'normal', guardian], { env: { PATH: '/usr/bin:/bin' }, encoding: 'utf8', timeout: 10_000 });
    assert.equal(result.status, 0, result.stderr);
    assert.deepEqual(JSON.parse(readFileSync(join(directory, 'closed.json'), 'utf8')), { text: 'exact', clean: true, forced: false });
    assert.ok(absent(readPid(join(directory, 'fixture-pid.json'))));
    assert.ok(absent(readGroup(directory)));
    assert.ok(absent(-readGroup(directory)));
  } finally { rmSync(directory, { recursive: true, force: true }); }
});

function runGuardian(args: string[], detached: boolean, cwd?: string): Promise<number | null> {
  return new Promise((resolve, reject) => {
    const child = spawn(guardian, args, { cwd, detached, env: {}, stdio: 'ignore', timeout: 3000, killSignal: 'SIGKILL' });
    child.once('error', reject);
    child.once('exit', code => {
      if (detached && child.pid) { try { process.kill(-child.pid, 'SIGKILL'); } catch {} }
      resolve(code);
    });
  });
}

test('guardian rejects incorrect parent and shared group before starting a runtime', { skip: process.platform !== 'darwin' }, async () => {
  const directory = mkdtempSync('/tmp/fort059-guardian-reject-');
  try {
    for (const [parent, detached] of [[1, true], [process.pid + 1, true], [process.pid, false]] as const) {
      const status = await runGuardian([String(parent), process.execPath, join(source, 'host/fixtures/native-channel.mjs'), 'normal'], detached, directory);
      assert.equal(status, 125);
      assert.equal(existsSync(join(directory, 'fixture-pid.json')), false);
    }
  } finally { rmSync(directory, { recursive: true, force: true }); }
});

test('guardian preserves the exact child exit status and rejects an unavailable executable', { skip: process.platform !== 'darwin' }, async () => {
  for (const [command, args, expected] of [
    [process.execPath, ['-e', 'process.exit(42)'], 42],
    [join(build, 'missing-executable'), [], 125]
  ] as const) {
    const status = await runGuardian([String(process.pid), command, ...args], true);
    assert.equal(status, expected);
  }
});

test('guardian stays within the added warm launch budget over ten equivalent launches', { skip: process.platform !== 'darwin', timeout: 30_000 }, t => {
  const samples: { baselineMs: number; guardedMs: number }[] = [];
  for (let index = 0; index < 10; index++) {
    const elapsed: number[] = [];
    for (const guard of index % 2 === 0 ? ['unguarded', guardian] : [guardian, 'unguarded']) {
      const directory = mkdtempSync('/tmp/fort059-guardian-launch-');
      try {
        const result = spawnSync(supervisor, [process.execPath, join(source, 'host/fixtures/native-channel.mjs'), directory, 'normal', guard], { env: { PATH: '/usr/bin:/bin' }, encoding: 'utf8', timeout: 8000 });
        assert.equal(result.status, 0, result.stderr);
        const proof: unknown = JSON.parse(readFileSync(join(directory, 'supervisor-ready.json'), 'utf8'));
        assert.ok(proof && typeof proof === 'object' && 'startupMs' in proof && typeof proof.startupMs === 'number');
        elapsed.push(proof.startupMs);
        assert.ok(absent(readGroup(directory)) && absent(-readGroup(directory)));
      } finally { rmSync(directory, { recursive: true, force: true }); }
    }
    samples.push({ baselineMs: elapsed[index % 2], guardedMs: elapsed[1 - index % 2] });
  }
  const added = samples.map(sample => sample.guardedMs - sample.baselineMs).sort((a, b) => a - b);
  assert.ok(added[9] <= 100, 'added warm startup p95 exceeds 100 ms');
  t.diagnostic(JSON.stringify({ samples, addedP95Ms: added[9] }));
});

test('the parent-exit guard waits without idle polling within its resource budgets', { skip: process.platform !== 'darwin', timeout: 20_000 }, async t => {
  const directory = mkdtempSync('/tmp/fort059-guardian-idle-');
  const child = spawn(supervisor, [process.execPath, join(source, 'host/fixtures/native-channel.mjs'), directory, 'hang', guardian], { env: { PATH: '/usr/bin:/bin' }, stdio: 'ignore' });
  const pid = child.pid;
  assert.ok(pid && pid > 1);
  let group: number | undefined;
  t.after(async () => {
    for (const target of [pid, ...(group ? [-group] : [])]) { try { process.kill(target, 'SIGKILL'); } catch {} }
    assert.ok(await until(() => absent(pid) && (!group || absent(-group)), 2000), 'fixture cleanup was not confirmed');
    rmSync(directory, { recursive: true, force: true });
  });
  assert.ok(await until(() => existsSync(join(directory, 'supervisor-ready.json')), 6000));
  group = readGroup(directory);
  const sample = () => {
    const result = spawnSync('/bin/ps', ['-p', String(group), '-o', 'rss=,time='], { encoding: 'utf8', timeout: 1000 });
    assert.equal(result.status, 0, result.stderr);
    const parts = result.stdout.trim().split(/\s+/);
    const rssMiB = Number(parts[0]) / 1024;
    const cpuSeconds = parts[1].split(':').reduce((seconds, value) => seconds * 60 + Number(value), 0);
    assert.ok(Number.isFinite(rssMiB) && Number.isFinite(cpuSeconds));
    return { rssMiB, cpuSeconds };
  };
  const first = sample(), started = performance.now();
  await delay(12_000);
  const last = sample(), elapsedSeconds = (performance.now() - started) / 1000;
  const cpuPercentUpperBound = (last.cpuSeconds - first.cpuSeconds + 0.01) / elapsedSeconds * 100;
  assert.ok(Math.max(first.rssMiB, last.rssMiB) <= 25, 'guardian idle RSS exceeds 25 MiB');
  assert.ok(cpuPercentUpperBound <= 0.1, 'guardian idle CPU exceeds 0.1% including ps rounding');
  t.diagnostic(JSON.stringify({ first, last, elapsedSeconds, cpuPercentUpperBound, scope: 'guardian only; ps CPU resolution 10 ms' }));
});

test('abrupt Node service death removes the dispatched executor and noncooperative descendant', { skip: process.platform !== 'darwin', timeout: 15_000 }, async t => {
  const directory = mkdtempSync('/tmp/fort059-executor-owner-');
  const child = spawn(process.execPath, [join(source, 'host/fixtures/executor-supervisor.mjs'), directory, guardian, 'parent-death'], { cwd: directory, env: {}, stdio: 'ignore' });
  const pid = child.pid;
  assert.ok(pid && pid > 1);
  let group: number | undefined;
  t.after(async () => {
    for (const target of [pid, ...(group ? [-group] : [])]) { try { process.kill(target, 'SIGKILL'); } catch {} }
    assert.ok(await until(() => absent(pid) && (!group || absent(-group)), 2000), 'fixture cleanup was not confirmed');
    rmSync(directory, { recursive: true, force: true });
  });
  assert.ok(await until(() => existsSync(join(directory, 'executor-descendant-pid.json')), 6000));
  const executor = readPid(join(directory, 'executor-pid.json'));
  const descendant = readPid(join(directory, 'executor-descendant-pid.json'));
  const grouping = spawnSync('/bin/ps', ['-p', String(executor), '-o', 'pgid='], { encoding: 'utf8', timeout: 1000 });
  assert.equal(grouping.status, 0, grouping.stderr);
  group = Number(grouping.stdout.trim());
  assert.ok(Number.isSafeInteger(group) && group > 1 && group !== pid);
  assert.equal(absent(executor), false);
  assert.equal(absent(descendant), false);
  process.kill(pid, 'SIGKILL');
  const started = performance.now();
  assert.ok(await until(() => [pid, group, executor, descendant, -group].every(absent), 3000), 'executor group survived abrupt Node owner exit');
  t.diagnostic(JSON.stringify({ owner: pid, group, executor, descendant, cleanupMs: performance.now() - started }));
});

test('guarded executor preserves exact replies, cancellation, invalid evidence and permission denials', { skip: process.platform !== 'darwin', timeout: 15_000 }, async t => {
  for (const [scenario, expected] of [
    ['echo', 'completed'], ['await-cancel', 'canceled'], ['permission-controls', 'completed'],
    ['missing-guardian', 'failed'],
    ['stale-generation', 'outcomeUnknown'], ['terminal-then-hang', 'outcomeUnknown']
  ] as const) await t.test(scenario, () => {
    const directory = mkdtempSync('/tmp/fort059-guarded-executor-');
    try {
      const selected = scenario === 'missing-guardian' ? join(build, 'missing-guardian') : guardian;
      const result = spawnSync(process.execPath, [join(source, 'host/fixtures/executor-supervisor.mjs'), directory, selected, scenario], { cwd: directory, env: {}, encoding: 'utf8', timeout: 10_000 });
      assert.equal(result.status, 0, result.stderr);
      const proof: unknown = JSON.parse(readFileSync(join(directory, 'owner-done.json'), 'utf8'));
      assert.ok(proof && typeof proof === 'object' && 'state' in proof && 'messages' in proof && 'exit' in proof);
      assert.equal(proof.state, expected);
      assert.deepEqual(proof.messages, expected === 'completed' ? ['Exact synthetic reply', 'Exact synthetic reply'] : ['Exact synthetic reply']);
      const exit = proof.exit;
      assert.ok(exit && typeof exit === 'object' && 'groupAbsent' in exit && 'pid' in exit && 'forced' in exit);
      assert.equal(exit.groupAbsent, true);
      if (expected === 'failed') { assert.equal(exit.pid, null); return; }
      assert.equal(typeof exit.pid, 'number');
      assert.ok(typeof exit.pid === 'number');
      assert.ok(absent(exit.pid) && absent(-exit.pid));
      if (expected !== 'outcomeUnknown') assert.equal(exit.forced, false);
    } finally { rmSync(directory, { recursive: true, force: true }); }
  });
});
