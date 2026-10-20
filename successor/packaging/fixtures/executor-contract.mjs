import assert from 'node:assert/strict';
import { readFileSync, writeFileSync, existsSync, rmSync, symlinkSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const [scenario, artifact, directory, guardian] = process.argv.slice(2);
const script = fileURLToPath(import.meta.url);
const { syntheticAccountIdentity } = await import('./preflight.mjs');
const { createCodexExecutorProcess, serveCodexExecutor, createPrivateCodexExecutorProcess } = await import(pathToFileURL(join(artifact, 'runner/codex-executor.js')).href);

if (scenario === 'worker') {
  writeFileSync(join(directory, 'worker.json'), JSON.stringify({ worker: process.pid, outerGuardian: process.ppid }));
  try {
    await serveCodexExecutor({ input: process.stdin, output: process.stdout, hostId: 'fixture-host',
      runtime: { guardian, executable: process.execPath, args: ['--no-experimental-strip-types', join(dirname(script), 'runtime.mjs'), 'echo', directory],
        cwd: join(directory, 'workspace'), env: {} } });
  } catch { process.exitCode = 1; }
} else {
  const privateWorker = scenario.startsWith('private');
  if (privateWorker) {
    const preload = join(directory, 'unexpected-preload.cjs');
    writeFileSync(preload, `require('node:fs').writeFileSync(${JSON.stringify(join(directory, 'unexpected-preload'))},'synthetic');process.exit(42);`);
    process.env.NODE_OPTIONS = '--require=' + preload;
  }
  if (['private-openssl-missing', 'private-openssl-altered', 'private-openssl-linked'].includes(scenario)) {
    const configuration = join(artifact, 'node-openssl.cnf');
    if (scenario === 'private-openssl-altered') writeFileSync(configuration, 'nodejs_conf = foreign\n[foreign]\n');
    else {
      rmSync(configuration);
      if (scenario === 'private-openssl-linked') {
        const target = join(directory, 'linked.cnf'); writeFileSync(target, 'nodejs_conf = fort_node\n[fort_node]\n'); symlinkSync(target, configuration);
      }
    }
    assert.throws(() => createPrivateCodexExecutorProcess({ executable: join(directory, 'synthetic-codex.mjs'),
      directory: join(directory, 'private'), guardian, hostId: 'fixture-host' }));
    assert.equal(existsSync(join(directory, 'launch.json')), false);
    assert.equal(existsSync(join(directory, 'runtime.json')), false);
    process.stdout.write(JSON.stringify({scenario, models:0, runtimeStarted:false, groupsAbsent:true, observations:[]}) + '\n');
    process.exit(0);
  }
  const runtime = privateWorker ? createPrivateCodexExecutorProcess({ executable: join(directory, 'synthetic-codex.mjs'),
    directory: join(directory, 'private'), guardian, hostId: 'fixture-host' }) : createCodexExecutorProcess({ executable: process.execPath,
    args: ['--no-experimental-strip-types', script, 'worker', artifact, directory, guardian], cwd: directory, env: {}, guardian,
    hostId: scenario === 'foreign-host' ? 'foreign-host' : 'fixture-host' });
  assert.equal(runtime.lastExit, undefined); assert.equal(existsSync(join(directory, 'runtime.json')), false);
  if (scenario === 'private-changed') writeFileSync(join(directory, 'private/state/.env'), 'SYNTHETIC=must-not-load', { mode: 0o600 });
  if (scenario === 'private-openssl-changed') writeFileSync(join(artifact, 'node-openssl.cnf'), 'nodejs_conf = broken\n[broken\n');
  const observations = [];
  function confirmAbsent(includeRuntime) {
    const owned = privateWorker ? { outerGuardian: runtime.lastExit.pid } : JSON.parse(readFileSync(join(directory, 'worker.json'), 'utf8'));
    if (includeRuntime) Object.assign(owned, JSON.parse(readFileSync(join(directory, 'runtime.json'), 'utf8')));
    for (const pid of Object.values(owned)) {
      assert.ok(Number.isSafeInteger(pid) && pid > 1);
      assert.throws(() => process.kill(pid, 0), { code: 'ESRCH' });
    }
    for (const group of [owned.outerGuardian, owned.innerGuardian].filter(value => value !== undefined)) {
      assert.throws(() => process.kill(-group, 0), { code: 'ESRCH' });
    }
    assert.equal(runtime.lastExit.groupAbsent, true); observations.push(owned);
  }
  try {
    const catalog = await runtime.models(new AbortController().signal, syntheticAccountIdentity);
    if (scenario === 'foreign-host' || scenario === 'private-changed' || scenario === 'private-openssl-changed') {
      assert.notEqual(catalog.state, 'ready'); assert.equal(catalog.models.length, 0);
      assert.equal(existsSync(join(directory, 'runtime.json')), false); confirmAbsent(false);
      if (scenario === 'private-changed') assert.equal(readFileSync(join(directory, 'private/state/.env'), 'utf8'), 'SYNTHETIC=must-not-load');
      process.stdout.write(JSON.stringify({ scenario, models: 0, runtimeStarted: false, groupsAbsent: true, observations }) + '\n');
    } else {
      assert.equal(catalog.state, 'ready'); assert.deepEqual(catalog.models, [{ id: 'fixture-exact-model', name: 'fixture-exact-model', availability: 'available' }]);
      assert.equal(runtime.lastExit.forced, false); confirmAbsent(true);
      const command = JSON.parse(readFileSync(join(directory, 'command.json'), 'utf8'));
      const { input } = command;
      let { dispatch } = command;
      const prior = readFileSync(join(directory, 'runtime.json'));
      if (scenario === 'private-foreign') {
        const { sealDispatch } = await import(pathToFileURL(join(artifact, 'contracts/executor.js')).href);
        const { dispatchDigest, ...body } = dispatch;
        dispatch = sealDispatch({ ...body, binding: { ...body.binding, hostId: 'foreign-host' } });
      }
      const result = await runtime.execute(input, new AbortController().signal, dispatch);
      if (scenario === 'private-foreign') {
        assert.equal(result.kind, 'outcomeUnknown'); assert.deepEqual(readFileSync(join(directory, 'runtime.json')), prior);
      } else assert.deepEqual(result, { kind: 'completed', text: input.text, effectiveModel: null });
      assert.equal(runtime.lastExit.forced, false); confirmAbsent(scenario !== 'private-foreign');
      const methods = readFileSync(join(directory, 'methods.jsonl'), 'utf8').trim().split('\n').map(line => JSON.parse(line));
      assert.equal(methods.filter(value => value.method === 'thread/start').length, scenario === 'private-foreign' ? 0 : 1);
      assert.equal(methods.filter(value => value.method === 'turn/start').length, scenario === 'private-foreign' ? 0 : 1);
      assert.ok(methods.filter(value => value.method === 'account/read').every(value => value.refreshToken === false));
      if (privateWorker) {
        const launch = JSON.parse(readFileSync(join(directory, 'launch.json'), 'utf8'));
        assert.equal(launch.cwd, join(directory, 'private/workspace'));
        if (Object.hasOwn(launch.env, '__CF_USER_TEXT_ENCODING')) {
          assert.match(launch.env.__CF_USER_TEXT_ENCODING, /^0x[\da-f]+:0x[\da-f]+:0x[\da-f]+$/i); delete launch.env.__CF_USER_TEXT_ENCODING;
        }
        assert.deepEqual(launch.env, { HOME: join(directory, 'private/home'), CODEX_HOME: join(directory, 'private/state'), TMPDIR: join(directory, 'private/tmp'), PATH: '/usr/bin:/bin', LANG: 'en_US.UTF8', TZ: 'UTC' });
        assert.deepEqual(launch.args.slice(0, 3), ['app-server', '--listen', 'stdio://']);
        assert.ok(launch.args.includes('cli_auth_credentials_store="file"')); assert.ok(launch.args.includes('forced_login_method="chatgpt"'));
      }
      process.stdout.write(JSON.stringify({ scenario, models: 1, exactText: scenario !== 'private-foreign', refusedForeignDispatch: scenario === 'private-foreign', runtimeStarted: true, groupsAbsent: true, observations }) + '\n');
    }
  } finally {
    await runtime.dispose();
    assert.equal(existsSync(join(directory, 'unexpected-preload')), false);
  }
}
