import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, realpathSync, rmSync, readFileSync, mkdirSync, writeFileSync, copyFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
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
  for (const scenario of ['complete', 'foreign-host']) await t.test(scenario, () => {
    const directory = join(root, scenario); mkdirSync(directory, { mode: 0o700 }); mkdirSync(join(directory, 'workspace'), { mode: 0o700 });
    const input: ChatInput = { context: { kind: 'empty' }, attempt: { runId: 'run', attemptId: 'attempt', generation: 1, bindingRevisionId: 'binding', runtimeVersion: '0.157.1', selectedModel: 'fixture-exact-model' },
      runId: 'run', requestId: 'request', conversationId: 'conversation', configuration: { agentId: 'agent', agentName: 'Fixture', bindingRevisionId: 'binding', selectedModel: 'fixture-exact-model', instructions: '', skills: [] }, history: [], text: '  compiled synthetic cedar 🌲\n' };
    const dispatch = sealDispatch({ protocolVersion: 1, installationId: 'installation', recoveryEpoch: 'epoch',
      binding: { bindingRevisionId: 'binding', provider: 'codex', runtimeVersion: '0.157.1', selectedModel: 'fixture-exact-model', hostId: 'fixture-host', enabled: true }, attempt: input.attempt,
      inputDigest: executionDigest(input), configurationDigest: executionDigest(input.configuration), historyDigest: executionDigest(input.history),
      policy: TEXT_POLICY, policyDigest: executionDigest(TEXT_POLICY), deadline: Date.now() + 10_000, limits: EXECUTION_LIMITS });
    writeFileSync(join(directory, 'command.json'), JSON.stringify({ input, dispatch }), { mode: 0o600 });
    const proof = spawnSync(process.execPath, ['--no-experimental-strip-types', script, scenario, artifact, directory, guardian],
      { cwd: root, env: {}, encoding: 'utf8', timeout: 15_000 });
    assert.equal(proof.status, 0, proof.stderr);
    const report = JSON.parse(proof.stdout);
    assert.equal(report.groupsAbsent, true); assert.equal(report.scenario, scenario);
    assert.equal(report.runtimeStarted, scenario === 'complete');
    if (scenario === 'complete') { assert.equal(report.exactText, true); assert.equal(report.models, 1); }
    else assert.equal(report.models, 0);
    t.diagnostic(JSON.stringify(report));
  });
});
