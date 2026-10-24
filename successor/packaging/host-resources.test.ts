import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, realpathSync, rmSync, readFileSync, readdirSync, writeFileSync, lstatSync, mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { buildHostResources } from './host-resources.ts';

test('host resources compose isolated compiled modules with complete attribution and no ambient preload', t => {
  const root = realpathSync(mkdtempSync('/tmp/fort059-host-resources-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const destination = join(root, 'host'), preload = join(root, 'preload.cjs'), marker = join(root, 'unexpected');
  writeFileSync(preload, `require('node:fs').writeFileSync(${JSON.stringify(marker)},'synthetic');process.exit(42);`);
  const prior = process.env.NODE_OPTIONS;
  let manifest;
  try { process.env.NODE_OPTIONS = '--require=' + preload; manifest = buildHostResources(destination); }
  finally { if (prior === undefined) delete process.env.NODE_OPTIONS; else process.env.NODE_OPTIONS = prior; }
  assert.equal(lstatSync(marker, { throwIfNoEntry: false }), undefined);
  const files = readdirSync(destination, { recursive: true, encoding: 'utf8' }).filter(path => lstatSync(join(destination, path)).isFile());
  assert.deepEqual(files.filter(path => path !== 'manifest.json').sort(), manifest.files.map(file => file.path).sort());
  for (const file of manifest.files) {
    const bytes = readFileSync(join(destination, file.path));
    assert.equal(bytes.length, file.bytes); assert.equal(createHash('sha256').update(bytes).digest('hex'), file.sha256);
  }
  for (const license of ['core/LICENSE', 'executor/LICENSE', 'context/LICENSE']) assert.ok(readFileSync(join(destination, license), 'utf8').includes('MIT License'));
  for (const path of ['node-openssl.cnf', 'executor/node-openssl.cnf']) assert.equal(readFileSync(join(destination, path), 'utf8'), 'nodejs_conf = fort_node\n[fort_node]\n');
  assert.ok(files.includes('context/modules/local-context.js'));
  for (const path of ['history-maintenance.mjs', 'core/host/history-maintenance.js', 'core/service/delete-history.js']) assert.ok(files.includes(path), path);
  assert.ok(!files.some(path => path.startsWith('core/') && /(?:codex|modules\/)/.test(path)));
  assert.ok(!files.some(path => path.startsWith('executor/modules/')));
  assert.ok(!files.some(path => path.endsWith('.ts') && !path.endsWith('.d.ts')));
  const invalid = spawnSync(process.execPath, ['--jitless', '--no-experimental-strip-types', join(destination, 'main.mjs'), '--directory', root], { cwd: root, env: {}, encoding: 'utf8', timeout: 5000 });
  assert.equal(invalid.status, 1); assert.match(invalid.stderr, /Fort host could not start/);
  assert.equal(readFileSync(join(root, 'host-start-failure.txt'), 'utf8'), 'The bundled host accepts no arguments\n');
  assert.equal(lstatSync(join(root, 'host-start-failure.txt')).mode & 0o777, 0o600);
  assert.deepEqual(readdirSync(root).sort(), ['host', 'host-start-failure.txt', 'preload.cjs']);
});

test('host resource assembly refuses existing destinations without touching their contents', t => {
  const root = realpathSync(mkdtempSync('/tmp/fort059-host-existing-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  mkdirSync(join(root, 'keep')); writeFileSync(join(root, 'keep', 'marker'), 'synthetic unchanged');
  for (const path of ['relative', '/', root + '/child/../new', root + '\0bad', root + '/\ud800', join(root, 'keep')]) assert.throws(() => buildHostResources(path));
  assert.equal(readFileSync(join(root, 'keep', 'marker'), 'utf8'), 'synthetic unchanged');
  assert.deepEqual(readdirSync(root), ['keep']);
});

test('five fresh host resource builds meet their separate assembly budget', { skip: !process.env.FORT_HOST_RESOURCE_BENCHMARK }, t => {
  const destination = process.env.FORT_HOST_RESOURCE_BENCHMARK; assert.ok(destination);
  const root = realpathSync(mkdtempSync('/tmp/fort059-host-build-budget-')), samples: number[] = [];
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (let index = 0; index < 5; index++) { const start = performance.now(); buildHostResources(join(root, String(index))); samples.push(performance.now() - start); }
  writeFileSync(destination, JSON.stringify(samples), { flag: 'wx' });
  assert.ok(Math.max(...samples) <= 10_000);
});
