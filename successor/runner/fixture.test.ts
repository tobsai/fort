import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, writeFile, symlink, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createServer } from 'node:http';
import { spawnFixtureProcess } from './process.ts';

test('fixture worker receives no ambient secrets and cannot read canaries, launch tools or reach the owner API', async t => {
  const directory = await mkdtemp(join(tmpdir(), 'fort-fixture-canaries-'));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const paths = ['owner-token', 'ledger', 'sibling-run', 'provider-scope', 'home-canary'].map(name => join(directory, name));
  for (const path of paths) await writeFile(path, 'CANARY ONLY', { mode: 0o600 });
  const linked = join(directory, 'symlink'); await symlink(paths[0], linked); paths.push(linked);
  let reached = false;
  const service = createServer((_, response) => { reached = true; response.end('denied'); });
  await new Promise<void>(resolve => service.listen(0, '127.0.0.1', resolve));
  t.after(() => new Promise<void>(resolve => service.close(() => resolve())));
  const address = service.address() as { port: number };
  process.env.FORT_UNKNOWN_SECRET_CANARY = 'must not inherit';
  let child;
  try { child = spawnFixtureProcess(new URL('./isolation-probe.mjs', import.meta.url)); }
  finally { delete process.env.FORT_UNKNOWN_SECRET_CANARY; }
  let output = ''; child.stdout.on('data', chunk => output += chunk); child.stderr.resume();
  child.stdin.end(JSON.stringify({ paths, serviceURL: `http://127.0.0.1:${address.port}/v1/agents` }));
  const code = await new Promise<number | null>((resolve, reject) => { child.once('error', reject); child.once('close', resolve); });
  assert.equal(code, 0);
  const result = JSON.parse(output);
  assert.deepEqual(result.reads, paths.map(() => true));
  assert.equal(result.environmentCanaryAbsent, true);
  assert.equal(result.networkDenied, true);
  assert.equal(result.childDenied, true);
  assert.equal(result.inheritedDescriptorDenied, true);
  assert.equal(reached, false);
});
