import { test } from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

test('test discovery includes prior omissions and partitions privileged checks explicitly', () => {
  const script = fileURLToPath(new URL('./test-suite.mjs', import.meta.url));
  const suites = ['portable', 'macos', 'signed'].map(suite => {
    const result = spawnSync(process.execPath, [script, '--suite', suite, '--list'], { encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
    return JSON.parse(result.stdout);
  });
  for (const path of ['service/agent-settings.test.ts', 'service/owner-pairing.test.ts', 'service/migrate-ledger.test.ts', 'packaging/core-artifact.test.ts', 'prototype/owner-demo.test.ts', 'runner/startup-diagnostic.test.ts']) assert.ok(suites[0].files.includes(path));
  assert.deepEqual(suites[1].files, ['host/guardian.test.ts', 'prototype/mvp-app.test.ts', 'runner/confinement-canary.test.ts', 'runner/guarded-codex-process.test.ts']);
  assert.deepEqual(suites[2].files, ['packaging/node-bundle.test.ts', 'packaging/sandbox-feasibility.test.ts']);
  const all = suites.flatMap(suite => suite.files);
  assert.equal(new Set(all).size, all.length);
  assert.equal(all.length, suites[0].discovered);
  const unknown = spawnSync(process.execPath, [script, '--suite', 'unknown', '--list'], { encoding: 'utf8' });
  assert.notEqual(unknown.status, 0);
  const signed = spawnSync(process.execPath, [script, '--suite', 'signed'], { encoding: 'utf8', env: { PATH: '/usr/bin:/bin' } });
  assert.notEqual(signed.status, 0);
});
