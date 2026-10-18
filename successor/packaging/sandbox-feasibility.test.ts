import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const identity = process.env.FORT_SIGNING_IDENTITY;
test('embedded XPC peers authenticate exact identities and reject mismatches without expanding the sandbox', { skip: process.platform !== 'darwin' ? 'requires macOS' : !identity ? 'requires explicitly selected existing signing identity' : false }, t => {
  assert.ok(identity);
  const root = mkdtempSync('/tmp/fort059-sandbox-xpc-test-');
  const result = spawnSync(process.execPath, [fileURLToPath(new URL('./sandbox-feasibility.ts', import.meta.url)), '--runtime', process.execPath, '--identity', identity, '--destination', join(root, 'proof'), '--scenario', 'xpc'], { encoding: 'utf8', timeout: 90_000 });
  if (result.status === 0) t.after(() => rmSync(root, { recursive: true, force: true }));
  assert.equal(result.status, 0, result.stderr + result.stdout + '\nRetained fixture: ' + root);
  const proof: unknown = JSON.parse(result.stdout);
  assert.ok(proof && typeof proof === 'object');
  for (const key of ['xpcRoundTrip', 'xpcClientRejected', 'xpcServiceRejected', 'xpcProcessesAbsent', 'coreExactMessages', 'coreRestart', 'outsideReadDenied', 'outsideWriteDenied', 'loopbackListenDenied', 'ownedProcessesAbsent', 'syntheticContainerAbsent']) {
    assert.ok(key in proof && Reflect.get(proof, key) === true, key + ' was not confirmed');
  }
  assert.equal('xpcClientError' in proof && proof.xpcClientError, 4097);
  assert.equal('xpcServiceError' in proof && proof.xpcServiceError, 4102);
  assert.equal('xpcForcedCleanup' in proof && proof.xpcForcedCleanup, false);
  assert.ok('xpcServicePid' in proof && typeof proof.xpcServicePid === 'number');
  const servicePid = proof.xpcServicePid;
  assert.throws(() => process.kill(servicePid, 0), { code: 'ESRCH' });
});

test('the compiled core accepts and replays exact synthetic messages inside the signed sandbox bundle', { skip: process.platform !== 'darwin' ? 'requires macOS' : !identity ? 'requires explicitly selected existing signing identity' : false }, t => {
  assert.ok(identity);
  const root = mkdtempSync('/tmp/fort059-sandbox-core-test-');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const result = spawnSync(process.execPath, [fileURLToPath(new URL('./sandbox-feasibility.ts', import.meta.url)), '--runtime', process.execPath, '--identity', identity, '--destination', join(root, 'proof'), '--scenario', 'core'], { encoding: 'utf8', timeout: 90_000 });
  assert.equal(result.status, 0, result.stderr);
  const proof: unknown = JSON.parse(result.stdout);
  assert.ok(proof && typeof proof === 'object');
  for (const key of ['coreAuthentication', 'coreExactMessages', 'coreReplay', 'coreRestart', 'coreNoDuplicate', 'coreSocketAbsent', 'privateSocketRoundTrip', 'outsideReadDenied', 'outsideWriteDenied', 'loopbackListenDenied', 'ownedProcessesAbsent', 'syntheticContainerAbsent']) {
    assert.ok(key in proof && Reflect.get(proof, key) === true, key + ' was not confirmed');
  }
});

test('a sandboxed child retaining open pipes is killed with its parent within the cleanup bound', { skip: process.platform !== 'darwin' ? 'requires macOS' : !identity ? 'requires explicitly selected existing signing identity' : false }, t => {
  assert.ok(identity);
  const root = mkdtempSync('/tmp/fort059-sandbox-hang-');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const result = spawnSync(process.execPath, [fileURLToPath(new URL('./sandbox-feasibility.ts', import.meta.url)), '--runtime', process.execPath, '--identity', identity, '--destination', join(root, 'proof'), '--scenario', 'hang', '--timeout-ms', '1000'], { encoding: 'utf8', timeout: 90_000 });
  assert.equal(result.status, 1, result.stderr);
  const proof: unknown = JSON.parse(result.stdout);
  assert.ok(proof && typeof proof === 'object');
  assert.equal('outcome' in proof && proof.outcome, 'timedOut');
  assert.equal('ownedProcessesAbsent' in proof && proof.ownedProcessesAbsent, true, JSON.stringify(proof));
  assert.equal('syntheticContainerAbsent' in proof && proof.syntheticContainerAbsent, true);
  assert.equal('hungChildObserved' in proof && proof.hungChildObserved, true);
  assert.ok('elapsedMs' in proof && typeof proof.elapsedMs === 'number' && proof.elapsedMs < 20000);
  assert.ok('executionMs' in proof && typeof proof.executionMs === 'number' && proof.executionMs < 4000);
  assert.ok('parentPid' in proof && 'childPid' in proof);
  for (const pid of [proof.parentPid, proof.childPid]) {
    assert.ok(typeof pid === 'number' && Number.isSafeInteger(pid) && pid > 1);
    assert.throws(() => process.kill(pid, 0), { code: 'ESRCH' });
  }
});

test('sandboxed Swift parent runs bundled Node with private SQLite and denied outside capabilities', { skip: process.platform !== 'darwin' ? 'requires macOS' : !identity ? 'requires explicitly selected existing signing identity' : false }, t => {
  assert.ok(identity);
  const root = mkdtempSync('/tmp/fort059-sandbox-test-');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const result = spawnSync(process.execPath, [fileURLToPath(new URL('./sandbox-feasibility.ts', import.meta.url)), '--runtime', process.execPath, '--identity', identity, '--destination', join(root, 'proof')], { encoding: 'utf8', timeout: 90_000 });
  assert.equal(result.status, 0, result.stderr);
  const proof: unknown = JSON.parse(result.stdout);
  assert.ok(proof && typeof proof === 'object');
  assert.equal('sqlite' in proof && proof.sqlite, 'ready');
  assert.equal('outsideReadDenied' in proof && proof.outsideReadDenied, true);
  assert.equal('outsideWriteDenied' in proof && proof.outsideWriteDenied, true);
  assert.equal('loopbackListenDenied' in proof && proof.loopbackListenDenied, true);
  assert.equal('privateSocketRoundTrip' in proof && proof.privateSocketRoundTrip, true);
  assert.equal('ownedProcessesAbsent' in proof && proof.ownedProcessesAbsent, true);
  assert.equal('syntheticContainerAbsent' in proof && proof.syntheticContainerAbsent, true);
});
