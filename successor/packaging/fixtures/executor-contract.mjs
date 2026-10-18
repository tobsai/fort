import assert from 'node:assert/strict';
import { readFileSync, writeFileSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const [scenario, artifact, directory, guardian] = process.argv.slice(2);
const script = fileURLToPath(import.meta.url);
const { createCodexExecutorProcess, serveCodexExecutor } = await import(pathToFileURL(join(artifact, 'runner/codex-executor.js')).href);

if (scenario === 'worker') {
  writeFileSync(join(directory, 'worker.json'), JSON.stringify({ worker: process.pid, outerGuardian: process.ppid }));
  try {
    await serveCodexExecutor({ input: process.stdin, output: process.stdout, hostId: 'fixture-host',
      runtime: { guardian, executable: process.execPath, args: ['--no-experimental-strip-types', join(dirname(script), 'runtime.mjs'), 'echo', directory],
        cwd: join(directory, 'workspace'), env: {} } });
  } catch { process.exitCode = 1; }
} else {
  const runtime = createCodexExecutorProcess({ executable: process.execPath,
    args: ['--no-experimental-strip-types', script, 'worker', artifact, directory, guardian], cwd: directory, env: {}, guardian,
    hostId: scenario === 'foreign-host' ? 'foreign-host' : 'fixture-host' });
  const observations = [];
  function confirmAbsent(includeRuntime) {
    const owned = JSON.parse(readFileSync(join(directory, 'worker.json'), 'utf8'));
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
    const catalog = await runtime.models(new AbortController().signal);
    if (scenario === 'foreign-host') {
      assert.notEqual(catalog.state, 'ready'); assert.equal(catalog.models.length, 0);
      assert.equal(existsSync(join(directory, 'runtime.json')), false); confirmAbsent(false);
      process.stdout.write(JSON.stringify({ scenario, models: 0, runtimeStarted: false, groupsAbsent: true, observations }) + '\n');
    } else {
      assert.equal(catalog.state, 'ready'); assert.deepEqual(catalog.models, [{ id: 'fixture-exact-model', name: 'fixture-exact-model', availability: 'available' }]);
      assert.equal(runtime.lastExit.forced, false); confirmAbsent(true);
      const { input, dispatch } = JSON.parse(readFileSync(join(directory, 'command.json'), 'utf8'));
      const result = await runtime.execute(input, new AbortController().signal, dispatch);
      assert.deepEqual(result, { kind: 'completed', text: input.text, effectiveModel: null });
      assert.equal(runtime.lastExit.forced, false); confirmAbsent(true);
      const methods = readFileSync(join(directory, 'methods.jsonl'), 'utf8').trim().split('\n').map(line => JSON.parse(line));
      assert.equal(methods.filter(value => value.method === 'thread/start').length, 1);
      assert.equal(methods.filter(value => value.method === 'turn/start').length, 1);
      assert.ok(methods.filter(value => value.method === 'account/read').every(value => value.refreshToken === false));
      process.stdout.write(JSON.stringify({ scenario, models: 1, exactText: true, runtimeStarted: true, groupsAbsent: true, observations }) + '\n');
    }
  } finally { await runtime.dispose(); }
}
