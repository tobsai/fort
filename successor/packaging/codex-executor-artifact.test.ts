import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, realpathSync, rmSync, readFileSync, readdirSync, mkdirSync, writeFileSync, symlinkSync, existsSync, lstatSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { buildCodexExecutor } from './codex-executor-artifact.ts';

test('the optional executor artifact contains complete compiled dependencies and usable public declarations', async t => {
  const root = realpathSync(mkdtempSync('/tmp/fort059-executor-artifact-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const artifact = join(root, 'executor'), manifest = buildCodexExecutor(artifact);
  const files = readdirSync(artifact, { recursive: true, encoding: 'utf8' });
  assert.ok(files.includes('runner/codex-executor.js') && files.includes('runner/codex-executor.d.ts'));
  assert.ok(files.includes('runner/executor-worker.js') && files.includes('runner/codex-chat.js'));
  assert.ok(files.includes('runner/private-codex-plan.js') && files.includes('runner/private-codex-plan.d.ts'));
  assert.ok(!files.some(path => /(^|\/)(service|host|modules|prototype|foundation|node_modules|fixtures)(\/|$)|\.test\.|\.ts$/.test(path) && !path.endsWith('.d.ts')));
  assert.deepEqual(JSON.parse(readFileSync(join(artifact, 'manifest.json'), 'utf8')), manifest);
  assert.deepEqual(manifest.files.map(file => file.path).sort(), files.filter(path => path !== 'manifest.json' && lstatSync(join(artifact, path)).isFile()).sort());
  for (const file of manifest.files) {
    const bytes = readFileSync(join(artifact, file.path));
    assert.equal(bytes.length, file.bytes); assert.equal(createHash('sha256').update(bytes).digest('hex'), file.sha256);
  }
  assert.equal(readFileSync(join(artifact, 'node-openssl.cnf'), 'utf8'), 'nodejs_conf = fort_node\n[fort_node]\n');
  assert.match(readFileSync(join(artifact, 'LICENSE'), 'utf8'), /Copyright \(c\) 2026 Tobias Gunn/);
  const packageJSON = JSON.parse(readFileSync(join(artifact, 'package.json'), 'utf8'));
  assert.equal(packageJSON.private, true); assert.equal(packageJSON.scripts, undefined); assert.equal(packageJSON.dependencies, undefined);
  assert.equal(packageJSON.exports['.'].import, './runner/codex-executor.js');
  const imports = spawnSync(process.execPath, ['--no-experimental-strip-types', '--input-type=module', '-e',
    `const m=await import(${JSON.stringify(join(artifact, 'runner/codex-executor.js'))});if(typeof m.serveCodexExecutor!=='function'||typeof m.createCodexExecutorProcess!=='function'||typeof m.privateCodexLaunchPlan!=='function'||typeof m.createPrivateCodexExecutorProcess!=='function')process.exit(1);`],
  { cwd: root, env: {}, encoding: 'utf8', timeout: 5000 });
  assert.equal(imports.status, 0, imports.stderr);
  for (const args of [[], ['--unapproved', 'synthetic-private-marker'], ['--executable', '/synthetic/missing']]) {
    const rejected = spawnSync(process.execPath, ['--no-experimental-strip-types', join(artifact, 'runner/codex-executor.js'), ...args],
      { cwd: root, env: {}, encoding: 'utf8', timeout: 3000 });
    assert.equal(rejected.status, 1); assert.equal(rejected.stdout, '');
    assert.equal(rejected.stderr, 'Fort executor could not start or finish safely.\n');
  }
  writeFileSync(join(root, 'consumer.mts'), `import { createCodexExecutorProcess, serveCodexExecutor, privateCodexLaunchPlan, createPrivateCodexExecutorProcess } from './executor/runner/codex-executor.js';
const config: Parameters<typeof createCodexExecutorProcess>[0] = { executable:'/exact/node', args:[], cwd:'/exact/empty', env:{}, guardian:'/exact/guardian', hostId:'explicit-host' };
const worker: Parameters<typeof serveCodexExecutor>[0] = { input:process.stdin, output:process.stdout, hostId:'explicit-host', runtime:privateCodexLaunchPlan({ executable:'/exact/codex', directory:'/exact/private', guardian:'/exact/guardian' }) };
const privateProcess: Parameters<typeof createPrivateCodexExecutorProcess>[0] = { executable:'/exact/codex', directory:'/exact/private', guardian:'/exact/guardian', hostId:'explicit-host' };
void config; void worker; void privateProcess;
`);
  const compiler = fileURLToPath(new URL('../node_modules/typescript/bin/tsc', import.meta.url));
  const types = fileURLToPath(new URL('../node_modules/@types', import.meta.url));
  const checked = spawnSync(process.execPath, [compiler, '--noEmit', '--strict', '--skipLibCheck', 'false', '--module', 'NodeNext', '--moduleResolution', 'NodeNext', '--target', 'ES2025', '--types', 'node', '--typeRoots', types, join(root, 'consumer.mts')],
    { cwd: root, env: {}, encoding: 'utf8', timeout: 20_000 });
  assert.equal(checked.status, 0, checked.stdout + checked.stderr);
  const saved = readFileSync(join(artifact, 'manifest.json'));
  assert.throws(() => buildCodexExecutor(artifact)); assert.deepEqual(readFileSync(join(artifact, 'manifest.json')), saved);
});

test('executor packaging refuses relative or existing destinations and preserves unrelated contents', t => {
  const root = realpathSync(mkdtempSync('/tmp/fort059-executor-destination-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const existing = join(root, 'existing'); mkdirSync(existing); writeFileSync(join(existing, 'keep'), 'unrelated synthetic');
  const file = join(root, 'file'), link = join(root, 'link'); writeFileSync(file, 'synthetic file'); symlinkSync(existing, link);
  for (const path of ['relative-artifact', existing, file, link]) assert.throws(() => buildCodexExecutor(path));
  assert.equal(readFileSync(join(existing, 'keep'), 'utf8'), 'unrelated synthetic'); assert.equal(readFileSync(file, 'utf8'), 'synthetic file');
});

test('executor compilation does not inherit ambient Node preload hooks', t => {
  const root = realpathSync(mkdtempSync('/tmp/fort059-executor-build-env-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const artifact = join(root, 'executor'), preload = join(root, 'preload.cjs'), marker = join(root, 'unexpected-hook');
  writeFileSync(preload, `require('node:fs').writeFileSync(${JSON.stringify(marker)},'synthetic');process.exit(42);`);
  const prior = process.env.NODE_OPTIONS;
  try {
    process.env.NODE_OPTIONS = '--require=' + preload;
    try { buildCodexExecutor(artifact); }
    catch (error) { assert.equal(existsSync(artifact), false, 'Failed build must remove only its new artifact'); throw error; }
    assert.equal(existsSync(marker), false); assert.equal(existsSync(join(artifact, 'manifest.json')), true);
  } finally {
    if (prior === undefined) delete process.env.NODE_OPTIONS;
    else process.env.NODE_OPTIONS = prior;
  }
});

test('executor packaging rejects ill-formed Unicode before creating any output', t => {
  const root = realpathSync(mkdtempSync('/tmp/fort059-executor-unicode-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  assert.throws(() => buildCodexExecutor(join(root, 'executor-\ud800')), /absolute, new artifact directory/);
  assert.deepEqual(readdirSync(root), []);
});

test('five isolated fresh executor builds meet the packaging budget', { skip: !process.env.FORT_EXECUTOR_BUILD_BENCHMARK }, t => {
  const destination = process.env.FORT_EXECUTOR_BUILD_BENCHMARK; assert.ok(destination);
  const root = realpathSync(mkdtempSync('/tmp/fort059-executor-build-budget-')), samples: number[] = [];
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (let index = 0; index < 5; index++) {
    const started = performance.now(); buildCodexExecutor(join(root, String(index))); samples.push(performance.now() - started);
  }
  writeFileSync(destination, JSON.stringify(samples), { flag: 'wx' });
  assert.ok(Math.max(...samples) <= 10_000);
});
