import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, copyFile, readFile, readdir, mkdir, realpath, writeFile, symlink, lstat } from 'node:fs/promises';
import { join } from 'node:path';
import childProcess, { spawnSync } from 'node:child_process';
import { mkdirSync, writeFileSync, renameSync, symlinkSync, linkSync } from 'node:fs';
import { syncBuiltinESMExports } from 'node:module';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';
import { buildCore } from './core-artifact.ts';

test('core compilation excludes ambient Node preload hooks', async t => {
  const root = await realpath(await mkdtemp('/tmp/fort059-core-build-env-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const destination = join(root, 'core'), preload = join(root, 'preload.cjs'), marker = join(root, 'unexpected-hook');
  await writeFile(preload, `require('node:fs').writeFileSync(${JSON.stringify(marker)},'synthetic');process.exit(42);`);
  const prior = process.env.NODE_OPTIONS;
  try {
    process.env.NODE_OPTIONS = '--require=' + preload;
    try { buildCore(destination); }
    catch (error) {
      t.diagnostic(JSON.stringify({ syntheticPreloadRan: await lstat(marker).then(() => true, () => false), failedArtifactRemoved: await lstat(destination).then(() => false, () => true) }));
      throw error;
    }
    await assert.rejects(lstat(marker), { code: 'ENOENT' });
    assert.equal((await lstat(join(destination, 'manifest.json'))).isFile(), true);
  } finally {
    if (prior === undefined) delete process.env.NODE_OPTIONS; else process.env.NODE_OPTIONS = prior;
  }
});

test('core packaging refuses unsafe and existing destinations without changing their contents', async t => {
  const root = await realpath(await mkdtemp('/tmp/fort059-core-build-paths-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const existing = join(root, 'existing'), file = join(root, 'file'), link = join(root, 'link');
  await mkdir(existing); await writeFile(join(existing, 'keep'), 'synthetic unrelated');
  await writeFile(file, 'synthetic file'); await symlink(existing, link);
  for (const destination of ['relative', '/', root + '/child/../new', root + '\0marker', root + '/\ud800', existing, file, link]) assert.throws(() => buildCore(destination));
  assert.deepEqual((await readdir(root)).sort(), ['existing', 'file', 'link']);
  assert.equal(await readFile(join(existing, 'keep'), 'utf8'), 'synthetic unrelated');
  assert.equal(await readFile(file, 'utf8'), 'synthetic file');
  assert.equal((await lstat(link)).isSymbolicLink(), true);
});

test('a failed build preserves a replacement destination directory', async t => {
  const root = await realpath(await mkdtemp('/tmp/fort059-core-build-replaced-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const destination = join(root, 'core'), original = childProcess.spawnSync;
  const override = t.mock.method(childProcess, 'spawnSync', () => {
    renameSync(destination, join(root, 'created-by-build')); mkdirSync(destination);
    writeFileSync(join(destination, 'keep'), 'synthetic replacement');
    return original(process.execPath, ['-e', 'process.exit(42)'], { env: {}, encoding: 'utf8' });
  });
  syncBuiltinESMExports();
  try {
    assert.throws(() => buildCore(destination), /Core compilation failed/);
    assert.equal(await readFile(join(destination, 'keep'), 'utf8'), 'synthetic replacement');
  } finally { override.mock.restore(); syncBuiltinESMExports(); }
});

test('unexpected compiled symlinks and hardlinks fail without modifying their target', async t => {
  for (const kind of ['symlink', 'hardlink']) await t.test(kind, async t => {
    const root = await realpath(await mkdtemp('/tmp/fort059-core-build-links-'));
    t.after(() => rm(root, { recursive: true, force: true }));
    const destination = join(root, 'core'), target = join(root, 'outside'), original = childProcess.spawnSync;
    await writeFile(target, 'synthetic unrelated');
    const override = t.mock.method(childProcess, 'spawnSync', () => {
      if (kind === 'symlink') symlinkSync(target, join(destination, 'unexpected.js'));
      else linkSync(target, join(destination, 'unexpected.js'));
      return original(process.execPath, ['-e', ''], { env: {}, encoding: 'utf8' });
    });
    syncBuiltinESMExports();
    try {
      assert.throws(() => buildCore(destination), /Unexpected core artifact file/);
      await assert.rejects(lstat(destination), { code: 'ENOENT' });
      assert.equal(await readFile(target, 'utf8'), 'synthetic unrelated');
    } finally { override.mock.restore(); syncBuiltinESMExports(); }
  });
});

test('five isolated fresh core artifacts meet the packaging budget', { skip: !process.env.FORT_CORE_BUILD_BENCHMARK }, async t => {
  const destination = process.env.FORT_CORE_BUILD_BENCHMARK; assert.ok(destination);
  const root = await realpath(await mkdtemp('/tmp/fort059-core-build-budget-')), samples: number[] = [];
  t.after(() => rm(root, { recursive: true, force: true }));
  for (let index = 0; index < 5; index++) {
    const started = performance.now(); buildCore(join(root, String(index))); samples.push(performance.now() - started);
  }
  await writeFile(destination, JSON.stringify(samples), { flag: 'wx' });
  assert.ok(Math.max(...samples) <= 10_000);
});

test('the compiled core runs independent runtime contracts with provider implementations absent', async t => {
  const root = await realpath(await mkdtemp('/tmp/fort059-core-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const artifact = join(root, 'core'), data = join(root, 'data');
  const build = spawnSync(process.execPath, [fileURLToPath(new URL('./core-artifact.ts', import.meta.url)), '--destination', artifact], { encoding: 'utf8', timeout: 30_000 });
  assert.equal(build.status, 0, build.stderr);
  const files = await readdir(artifact, { recursive: true });
  assert.ok(files.includes('service/openai.js') && files.includes('service/openai.d.ts'));
  assert.ok(files.includes('contracts/attempt.d.ts'));
  assert.ok(files.includes('host/secure-network.js') && files.includes('host/secure-network.d.ts'));
  assert.ok(files.includes('host/network-control.js') && files.includes('host/network-control.d.ts') && files.includes('contracts/secure-network.d.ts'));
  assert.ok(!files.includes('host/network-test-identity.js'));
  const packageInfo = JSON.parse(await readFile(join(artifact, 'package.json'), 'utf8'));
  assert.equal(packageInfo.exports['./secure-network'].import, './host/secure-network.js');
  assert.equal(packageInfo.exports['./network-control'].import, './host/network-control.js');
  const inert = spawnSync(process.execPath, ['--permission', `--allow-fs-read=${artifact}`, '--input-type=module', '-e', 'const transport = await import(process.argv[1]); if (typeof transport.openSecureNetwork !== "function") process.exit(2);', join(artifact, 'host/secure-network.js')], { env: {}, encoding: 'utf8', timeout: 5000 });
  assert.equal(inert.status, 0, inert.stderr);
  const inertControl = spawnSync(process.execPath, ['--permission', `--allow-fs-read=${artifact}`, '--input-type=module', '-e', 'const {createSecureNetworkControl}=await import(process.argv[1]); const control=createSecureNetworkControl({endpoint:{kind:"privateSocket",directory:"/nonexistent",name:"core.sock"},authorize:()=>true}); const status=await control.execute({operation:"inspect",owner:{installationId:"fixture",deviceId:"owner"}}); if(status.generation!==0 || status.listener.state!=="stopped")process.exit(2); await control.close();', join(artifact,'host/network-control.js')], {env:{},encoding:'utf8',timeout:5000});
  assert.equal(inertControl.status,0,inertControl.stderr);

  assert.ok(!files.some(file => /codex|prototype|node_modules|foundation|contracts\/runtime|modules\/|skill-context/.test(file)));
  assert.match(await readFile(join(artifact, 'LICENSE'), 'utf8'), /Copyright \(c\) 2026 Tobias Gunn/);
  const manifest = JSON.parse(await readFile(join(artifact, 'manifest.json'), 'utf8'));
  const emitted: string[] = [];
  for (const path of files) if (path !== 'manifest.json' && (await lstat(join(artifact, path))).isFile()) emitted.push(path);
  assert.deepEqual(manifest.files.map((entry: { path: string }) => entry.path).sort(), emitted.sort());
  for (const entry of manifest.files) {
    const bytes = await readFile(join(artifact, entry.path));
    assert.equal(bytes.length, entry.bytes); assert.equal(createHash('sha256').update(bytes).digest('hex'), entry.sha256);
  }
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
