import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const identity = process.env.FORT_SIGNING_IDENTITY;
const codex = process.env.FORT_TEST_CODEX;
test('Codex packaging refuses absent, wrong and implicit payloads before creating an artifact', t => {
  const root = mkdtempSync('/tmp/fort059-codex-preflight-');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const legal = fileURLToPath(new URL('./vendor/codex-0.157.1/LICENSE', import.meta.url));
  for (const [name, args] of [
    ['absent', ['--scenario', 'codex-version']],
    ['wrong', ['--scenario', 'codex-version', '--codex-runtime', legal]],
    ['implicit', ['--scenario', 'normal', '--codex-runtime', legal]]
  ] as const) {
    const destination = join(root, name);
    const result = spawnSync(process.execPath, [fileURLToPath(new URL('./sandbox-feasibility.ts', import.meta.url)), '--runtime', process.execPath, '--identity', '-', '--destination', destination, ...args], { encoding: 'utf8', timeout: 5000 });
    assert.equal(result.status, 1, result.stderr);
    assert.match(result.stderr, /exact Codex executable|not the reviewed 0\.157\.1 payload/);
    assert.equal(existsSync(destination), false);
  }
});

test('the exact Codex executable answers only its version inside the signed private sandbox', { skip: process.platform !== 'darwin' ? 'requires macOS' : !identity || !codex ? 'requires explicitly selected existing signing identity and reviewed Codex executable' : false }, t => {
  assert.ok(identity && codex);
  const root = mkdtempSync('/tmp/fort059-codex-version-test-');
  const result = spawnSync(process.execPath, [fileURLToPath(new URL('./sandbox-feasibility.ts', import.meta.url)), '--runtime', process.execPath, '--identity', identity, '--codex-runtime', codex, '--destination', join(root, 'proof'), '--scenario', 'codex-version'], { encoding: 'utf8', timeout: 90_000 });
  assert.equal(result.status, 0, result.stderr + result.stdout + '\nRetained artifact: ' + root);
  const proof: unknown = JSON.parse(result.stdout);
  assert.ok(proof && typeof proof === 'object');
  for (const key of ['codexVersionConfirmed', 'codexProcessAbsent', 'codexSourceUnchanged', 'outsideReadDenied', 'outsideWriteDenied', 'loopbackListenDenied', 'ownedProcessesAbsent', 'syntheticContainerAbsent']) {
    assert.ok(key in proof && Reflect.get(proof, key) === true, key + ' was not confirmed');
  }
  assert.equal('codexVersion' in proof && proof.codexVersion, 'codex-cli 0.157.1');
  assert.equal('codexSourceSHA256' in proof && proof.codexSourceSHA256, '27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d');
  assert.ok('codexVersionMs' in proof && typeof proof.codexVersionMs === 'number' && proof.codexVersionMs > 0 && proof.codexVersionMs < 5000);
  const codexPid = 'codexPid' in proof ? proof.codexPid : undefined;
  assert.ok(typeof codexPid === 'number' && Number.isSafeInteger(codexPid) && codexPid > 1);
  assert.throws(() => process.kill(codexPid, 0), { code: 'ESRCH' });
  assert.equal('providerUsed' in proof && proof.providerUsed, false);
  assert.equal('startupRegistered' in proof && proof.startupRegistered, false);
  assert.equal('codexForcedCleanup' in proof && proof.codexForcedCleanup, false);
  t.after(() => rmSync(root, { recursive: true, force: true }));
});

test('embedded XPC peers authenticate exact identities and reject mismatches without expanding the sandbox', { skip: process.platform !== 'darwin' ? 'requires macOS' : !identity ? 'requires explicitly selected existing signing identity' : false }, t => {
  assert.ok(identity);
  const root = mkdtempSync('/tmp/fort059-sandbox-xpc-test-');
  const result = spawnSync(process.execPath, [fileURLToPath(new URL('./sandbox-feasibility.ts', import.meta.url)), '--runtime', process.execPath, '--identity', identity, '--destination', join(root, 'proof'), '--scenario', 'xpc'], { encoding: 'utf8', timeout: 90_000 });
  if (result.status === 0) t.after(() => rmSync(root, { recursive: true, force: true }));
  assert.equal(result.status, 0, result.stderr + result.stdout + '\nRetained fixture: ' + root);
  const proof: unknown = JSON.parse(result.stdout);
  assert.ok(proof && typeof proof === 'object');
  for (const key of ['parentStartupMs', 'parentWorkMs', 'executionMs']) {
    assert.ok(key in proof && typeof Reflect.get(proof, key) === 'number' && Reflect.get(proof, key) > 0, key + ' was not measured');
  }
  for (const key of ['xpcRoundTrip', 'xpcClientRejected', 'xpcServiceRejected', 'xpcCancellation', 'xpcTimeout', 'xpcMalformedResponseRejected', 'xpcInvalidStatusRejected', 'xpcOversizedRequestRejected', 'xpcOversizedResponseRejected', 'xpcProcessesAbsent', 'coreExactMessages', 'coreRestart', 'outsideReadDenied', 'outsideWriteDenied', 'loopbackListenDenied', 'ownedProcessesAbsent', 'syntheticContainerAbsent']) {
    assert.ok(key in proof && Reflect.get(proof, key) === true, key + ' was not confirmed');
  }
  assert.equal('xpcClientError' in proof && proof.xpcClientError, 4097);
  assert.equal('xpcServiceError' in proof && proof.xpcServiceError, 4102);
  assert.equal('xpcForcedCleanup' in proof && proof.xpcForcedCleanup, false);
  for (const key of ['xpcObservedPeer', 'xpcObservedTimeout', 'xpcObservedInvalidProofRejected', 'xpcObservedCorePeer', 'xpcObservedCoreExit', 'xpcOwnerRecovery', 'xpcOwnerForeignRejected', 'xpcOwnerNoHTTPRecovery', 'xpcOwnerClientRejected', 'xpcOwnerServiceRejected', 'xpcOwnerDisabled']) {
    assert.ok(key in proof && Reflect.get(proof, key) === true, key + ' was not confirmed');
  }
  for (const key of ['xpcOwnerSetup', 'xpcOwnerSetupRetry', 'xpcOwnerSetupForeignRejected', 'xpcOwnerSetupNoHTTP', 'xpcSetupClientRejected', 'xpcSetupServiceRejected', 'xpcSetupDisabled']) {
    assert.ok(key in proof && Reflect.get(proof, key) === true, key + ' was not confirmed');
  }
  for (const key of ['xpcSetupCancelPending', 'xpcSetupCancellation', 'xpcSetupCancelRetry', 'xpcSetupCancelForeignRejected', 'xpcSetupCancelNoHTTP', 'xpcSetupCancelRetainsReceipt', 'xpcSetupCancelClientRejected', 'xpcSetupCancelServiceRejected', 'xpcSetupCancelDisabled']) {
    assert.ok(key in proof && Reflect.get(proof, key) === true, key + ' was not confirmed');
  }
  for (const key of ['xpcCoreGuarded', 'xpcCoreAuthentication', 'xpcCoreExactMessages', 'xpcCoreReplay', 'xpcCoreCancellation', 'xpcCoreRestart', 'xpcCoreNoDuplicate', 'xpcCoreStopped', 'xpcCoreProcessesAbsent', 'xpcCoreOuterExecutableDenied', 'xpcCoreOutsideReadDenied', 'xpcCoreOutsideWriteDenied', 'xpcCoreTCPDenied']) {
    assert.ok(key in proof && Reflect.get(proof, key) === true, key + ' was not confirmed');
  }
  assert.ok('xpcServicePid' in proof && typeof proof.xpcServicePid === 'number');
  assert.ok('xpcCoreAbruptRecovery' in proof && proof.xpcCoreAbruptRecovery === true, 'signed helper crash recovery was not confirmed');
  assert.ok('xpcCoreCrashCleanupMs' in proof && typeof proof.xpcCoreCrashCleanupMs === 'number' && proof.xpcCoreCrashCleanupMs < 3000, 'signed crash cleanup exceeded its bound');
  assert.ok('xpcCoreCrashRestartMs' in proof && typeof proof.xpcCoreCrashRestartMs === 'number' && proof.xpcCoreCrashRestartMs < 12000, 'signed crash restart exceeded its separate bound');
  assert.ok('xpcObservedCoreExitMs' in proof && typeof proof.xpcObservedCoreExitMs === 'number' && proof.xpcObservedCoreExitMs < 3000);
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
