import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, readdir, rm, stat, writeFile, chmod } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { launchLinuxCodexProcess, linuxCodexCommand, SETPRIV } from './linux-codex-process.ts';
import { cloudCodexSignedIn, prepareCloudCodexDirectory } from './cloud-codex-executor.ts';

// Spec 063, revision 2026-11-03: the hub's Codex starts under setpriv, which kills it if its executor dies.
const linux = process.platform === 'linux';

test('Codex is started through setpriv with the parent-death signal set', () => {
  assert.deepEqual(linuxCodexCommand({ executable: '/opt/fort/codex/codex', args: ['app-server', '--listen', 'stdio://'] }),
    { executable: SETPRIV, args: ['--pdeathsig', 'KILL', '--', '/opt/fort/codex/codex', 'app-server', '--listen', 'stdio://'] });
});

test('the cloud launch keeps the Mac launch bounds', { skip: !linux }, () => {
  const options = { executable: '/opt/fort/codex/codex', args: [], cwd: '/tmp', env: {}, deadlineMs: 1000 };
  for (const field of ['executable', 'cwd'] as const)
    for (const invalid of ['relative', '/', '/a/../b', 'bad\0path']) assert.throws(() => launchLinuxCodexProcess({ ...options, [field]: invalid }), /exact normalized/);
  for (const deadlineMs of [0, -1, 1.5, 120_001]) assert.throws(() => launchLinuxCodexProcess({ ...options, deadlineMs }), /explicit runtime deadline/);
  assert.throws(() => launchLinuxCodexProcess({ ...options, args: Array(257).fill('x') }), /arguments exceed/);
  assert.throws(() => launchLinuxCodexProcess({ ...options, env: { 'A=B': 'c' } }), /environment exceeds/);
});

test('a launched process gets only the environment it is given and is ended on dispose', { skip: !linux }, async t => {
  const root = await mkdtemp(join(tmpdir(), 'fort-linux-codex-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  // A stand-in runtime that answers one JSON-RPC request with its environment, then waits to be ended.
  const script = join(root, 'runtime.mjs');
  await writeFile(script, "process.stdin.once('data',d=>{const r=JSON.parse(d);process.stdout.write(JSON.stringify({id:r.id,result:{env:Object.keys(process.env).sort()}})+'\\n')});setInterval(()=>{},1000);\n");
  const owned = launchLinuxCodexProcess({ executable: process.execPath, args: [script], cwd: root, env: { FORT_EXPLICIT: 'yes' }, deadlineMs: 10_000 });
  const reply = await owned.connection.request('probe', {}, new AbortController().signal) as { env: string[] };
  assert.deepEqual(reply.env.filter(key => key !== 'PWD' && key !== 'SHLVL' && key !== '_'), ['FORT_EXPLICIT']);
  await owned.dispose();
  assert.equal(owned.cleanupConfirmed, true);
});

test('the cloud runner directories are private, and sign-in is read from the credential file alone', { skip: !linux }, async t => {
  const root = await mkdtemp(join(tmpdir(), 'fort-cloud-dirs-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const directory = prepareCloudCodexDirectory(join(root, 'codex'));
  assert.deepEqual((await readdir(directory)).sort(), ['home', 'state', 'tmp', 'workspace']);
  for (const part of ['', 'home', 'state', 'tmp', 'workspace']) assert.equal((await stat(join(directory, part))).mode & 0o777, 0o700);
  assert.equal(cloudCodexSignedIn(directory), false);
  const auth = join(directory, 'state', 'auth.json');
  await writeFile(auth, '{"synthetic":true}', { mode: 0o644 });
  await chmod(auth, 0o644);
  assert.equal(cloudCodexSignedIn(directory), false, 'a credential others can read is not trusted');
  await chmod(auth, 0o600);
  assert.equal(cloudCodexSignedIn(directory), true);
  assert.equal(prepareCloudCodexDirectory(directory), directory, 'preparing again keeps what is there');
  assert.throws(() => prepareCloudCodexDirectory('relative/path'), /exact absolute/);
});
