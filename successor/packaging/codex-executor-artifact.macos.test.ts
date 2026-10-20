import { syntheticAccountIdentity } from '../runner/codex-preflight-fixture.ts';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, realpathSync, rmSync, readFileSync, mkdirSync, writeFileSync, copyFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { spawnSync } from 'node:child_process';
import { buildCodexExecutor } from './codex-executor-artifact.ts';
import { EXECUTION_LIMITS, TEXT_POLICY, executionDigest, sealDispatch } from '../contracts/executor.ts';
import type { ChatInput } from '../runner/chat-runtime.ts';

test('the isolated compiled executor discovers and executes without TypeScript or source checkout imports', { skip: process.platform !== 'darwin' }, async t => {
  const root = realpathSync(mkdtempSync('/tmp/fort059-compiled-executor-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const guardian = join(root, 'guardian'), artifact = join(root, 'executor'), script = join(root, 'contract.mjs');
  const compiled = spawnSync('/usr/bin/xcrun', ['swiftc', fileURLToPath(new URL('../host/mac-process-guardian.swift', import.meta.url)), '-o', guardian], { encoding: 'utf8', timeout: 60_000 });
  assert.equal(compiled.status, 0, compiled.stderr);
  buildCodexExecutor(artifact);
  copyFileSync(new URL('./fixtures/executor-contract.mjs', import.meta.url), script);
  copyFileSync(new URL('../runner/codex-preflight-fixture.ts', import.meta.url), join(root, 'preflight.mjs'));
  writeFileSync(join(root, 'runtime.mjs'), readFileSync(new URL('../runner/fixtures/codex-executor-runtime.mjs', import.meta.url), 'utf8').replace("'../codex-preflight-fixture.ts'", "'./preflight.mjs'"));
  const samples: number[] = [];
  const privateScenarios = process.env.FORT_PRIVATE_EXECUTOR_BENCHMARK ? Array.from({ length: 20 }, (_, index) => 'private-' + index) : ['private'];
  for (const scenario of ['complete', 'foreign-host', ...privateScenarios, 'private-changed', 'private-foreign', 'private-openssl-changed', 'private-openssl-missing', 'private-openssl-altered', 'private-openssl-linked']) await t.test(scenario, () => {
    rmSync(join(artifact, 'node-openssl.cnf'), { force:true });
    writeFileSync(join(artifact, 'node-openssl.cnf'), 'nodejs_conf = fort_node\n[fort_node]\n', { mode:0o600 });
    const directory = join(root, scenario); mkdirSync(directory, { mode: 0o700 }); mkdirSync(join(directory, 'workspace'), { mode: 0o700 });
    if (scenario.startsWith('private')) {
      mkdirSync(join(directory, 'private'), { mode: 0o700 });
      for (const name of ['home', 'state', 'workspace', 'tmp']) mkdirSync(join(directory, 'private', name), { mode: 0o700 });
      writeFileSync(join(directory, 'synthetic-codex.mjs'), `#!${process.execPath}\nimport { writeFileSync } from 'node:fs';
writeFileSync(${JSON.stringify(join(directory, 'launch.json'))}, JSON.stringify({ args:process.argv.slice(2), cwd:process.cwd(), env:process.env }));
process.argv = [process.execPath, import.meta.filename, 'echo', ${JSON.stringify(directory)}];
await import(${JSON.stringify(pathToFileURL(join(root, 'runtime.mjs')).href)});
`, { mode: 0o700 });
    }
    const input: ChatInput = { context: { kind: 'empty' }, attempt: { runId: 'run', attemptId: 'attempt', generation: 1, bindingRevisionId: 'binding', runtimeVersion: '0.163.0', selectedModel: 'fixture-exact-model' },
      runId: 'run', requestId: 'request', conversationId: 'conversation', configuration: { account: syntheticAccountIdentity, agentId: 'agent', agentName: 'Fixture', bindingRevisionId: 'binding', selectedModel: 'fixture-exact-model', instructions: '', skills: [] }, history: [], text: '  compiled synthetic cedar 🌲\n' };
    const dispatch = sealDispatch({ protocolVersion: 1, installationId: 'installation', recoveryEpoch: 'epoch',
      binding: { bindingRevisionId: 'binding', provider: 'codex', runtimeVersion: '0.163.0', selectedModel: 'fixture-exact-model', hostId: 'fixture-host', enabled: true }, attempt: input.attempt,
      inputDigest: executionDigest(input), configurationDigest: executionDigest(input.configuration), historyDigest: executionDigest(input.history),
      policy: TEXT_POLICY, policyDigest: executionDigest(TEXT_POLICY), deadline: Date.now() + 10_000, limits: EXECUTION_LIMITS });
    writeFileSync(join(directory, 'command.json'), JSON.stringify({ input, dispatch }), { mode: 0o600 });
    const started = performance.now();
    const proof = spawnSync(process.execPath, ['--jitless', '--no-experimental-strip-types', script, scenario, artifact, directory, guardian],
      { cwd: root, env: { FORT_SYNTHETIC_AMBIENT: 'must-not-inherit' }, encoding: 'utf8', timeout: 15_000 });
    const elapsed = performance.now() - started;
    assert.equal(proof.status, 0, proof.stderr);
    const report = JSON.parse(proof.stdout);
    assert.equal(report.groupsAbsent, true); assert.equal(report.scenario, scenario);
    const refused = scenario === 'foreign-host' || scenario === 'private-changed' || scenario.startsWith('private-openssl-');
    assert.equal(report.runtimeStarted, !refused);
    if (refused) assert.equal(report.models, 0);
    else { assert.equal(report.models, 1); assert.equal(report.exactText, scenario !== 'private-foreign'); }
    if (scenario === 'private-foreign') assert.equal(report.refusedForeignDispatch, true);
    if (scenario === 'private' || /^private-\d+$/.test(scenario)) samples.push(elapsed);
    t.diagnostic(JSON.stringify(report));
  });
  if (process.env.FORT_PRIVATE_EXECUTOR_BENCHMARK) {
    assert.equal(samples.length, 20);
    writeFileSync(process.env.FORT_PRIVATE_EXECUTOR_BENCHMARK, JSON.stringify(samples), { flag: 'wx' });
    assert.ok(samples.toSorted((a, b) => a - b)[18]! <= 2000);
  }
});
