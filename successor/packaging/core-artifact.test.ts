import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, copyFile, readFile, readdir, mkdir, realpath } from 'node:fs/promises';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

test('the compiled core runs independent runtime contracts with provider implementations absent', async t => {
  const root = await realpath(await mkdtemp('/tmp/fort059-core-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const artifact = join(root, 'core'), data = join(root, 'data');
  const build = spawnSync(process.execPath, [fileURLToPath(new URL('./core-artifact.ts', import.meta.url)), '--destination', artifact], { encoding: 'utf8', timeout: 30_000 });
  assert.equal(build.status, 0, build.stderr);
  const files = await readdir(artifact, { recursive: true });
  assert.ok(files.includes('service/openai.js') && files.includes('service/openai.d.ts'));
  assert.ok(files.includes('contracts/attempt.d.ts'));
  assert.ok(!files.some(file => /codex|prototype|node_modules|foundation|contracts\/runtime/.test(file)));
  assert.match(await readFile(join(artifact, 'LICENSE'), 'utf8'), /Copyright \(c\) 2026 Tobias Gunn/);
  const fixture = join(root, 'provider-contract.mjs');
  await copyFile(new URL('./fixtures/provider-contract.mjs', import.meta.url), fixture);
  await mkdir(data);
  const proof = spawnSync(process.execPath, ['--jitless', '--no-experimental-strip-types', '--permission', '--allow-net', `--allow-fs-read=${root}`, `--allow-fs-write=${data}`, fixture, artifact, data], {
    cwd: root, env: { PATH: '/usr/bin:/bin', HOME: root, TMPDIR: root, LANG: 'en_US.UTF-8' }, encoding: 'utf8', timeout: 15_000
  });
  assert.equal(proof.status, 0, proof.stderr);
  const report = JSON.parse(proof.stdout);
  assert.deepEqual(report.providers, ['batch-fixture', 'session-fixture']);
  assert.equal(report.contractsPassed, 2);
  assert.equal(report.providerImplementationsPresent, false);
  const duplicate = spawnSync(process.execPath, [fileURLToPath(new URL('./core-artifact.ts', import.meta.url)), '--destination', artifact], { encoding: 'utf8', timeout: 30_000 });
  assert.notEqual(duplicate.status, 0);
});
