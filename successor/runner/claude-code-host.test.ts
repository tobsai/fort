import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { chmodSync, lstatSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { createClaudeCodeHost, guardedClaudeLaunch, selectedClaudeCode, CLAUDE_CODE_DIRECTORY } from './claude-code-host.ts';

// A stand-in for the Mac guardian: records its arguments, then runs the child with the same pipes.
const guardianSource = `#!${process.execPath}
const { spawn } = require('node:child_process');
require('node:fs').writeFileSync(process.env.TMPDIR + '/guardian.json', JSON.stringify(process.argv.slice(2)));
const child = spawn(process.argv[3], process.argv.slice(4), { stdio: 'inherit' });
child.on('exit', code => process.exit(code ?? 1));
`;
const claudeSource = (version: string) => `#!${process.execPath}
const args = process.argv.slice(2);
if (args[0] === '--version') { process.stdout.write(${JSON.stringify(version)}); process.exit(0); }
process.stdin.on('data', () => {}); setInterval(() => {}, 1000);
`;

function setup(t: TestContext, versions = ['2.1.299', '2.1.310', '2.1.99']) {
  const root = realpathSync(mkdtempSync('/tmp/fcch-')); t.after(() => rmSync(root, { recursive: true, force: true }));
  const data = join(root, 'data'), directory = join(data, CLAUDE_CODE_DIRECTORY), temporary = join(root, 't'), guardian = join(root, 'guardian'), installation = join(root, 'versions');
  mkdirSync(data, { mode: 0o700 }); mkdirSync(directory, { mode: 0o700 }); mkdirSync(installation, { mode: 0o755 });
  writeFileSync(join(directory, 'selection.json'), '{}\n', { mode: 0o600 });
  for (const version of versions) writeFileSync(join(installation, version), claudeSource(version + ' (Claude Code)\n'), { mode: 0o755 });
  writeFileSync(guardian, guardianSource, { mode: 0o700 });
  return { root, data, directory, temporary, guardian, installation };
}
const host = (f: ReturnType<typeof setup>) => createClaudeCodeHost({ directory: f.directory, guardian: f.guardian, temporary: f.temporary, installation: f.installation });

test('the Claude-only mode is selected only by the owner\'s choice in the host data directory', t => {
  const f = setup(t);
  assert.equal(selectedClaudeCode(f.data), f.directory);
  rmSync(join(f.directory, 'selection.json'));
  assert.equal(selectedClaudeCode(f.data), undefined);
});

test('the host runs the newest installed version in place and keeps that pin across restarts', t => {
  const f = setup(t), first = host(f);
  assert.deepEqual(first.provider, { provider: 'claude-code', name: 'Claude', runtimeVersion: '2.1.310', hostId: 'local-claude-code', session: 'stateless', capabilities: ['plainText', 'modelCatalog', 'cancel', 'skills', 'memory'] });
  assert.deepEqual(JSON.parse(readFileSync(join(f.directory, 'pinned.json'), 'utf8')), { version: '2.1.310' });
  for (const path of ['home', 'workspace']) assert.equal(lstatSync(join(f.directory, path)).mode & 0o777, 0o700, path);
  writeFileSync(join(f.installation, '2.1.311'), claudeSource('2.1.311 (Claude Code)\n'), { mode: 0o755 });
  assert.equal(host(f).provider.runtimeVersion, '2.1.310', 'an update does not silently change the runtime');
  rmSync(join(f.installation, '2.1.310'));
  assert.throws(() => host(f), /no longer installed/);
});

test('the host refuses links, shared or mislabelled binaries, a non-empty workspace and an over-long runtime directory', t => {
  const cases: [string, (f: ReturnType<typeof setup>) => void, Partial<{ temporary: string }>?][] = [
    ['symlink', f => { rmSync(join(f.installation, '2.1.310')); symlinkSync(join(f.installation, '2.1.299'), join(f.installation, '2.1.310')); writeFileSync(join(f.directory, 'pinned.json'), '{"version":"2.1.310"}'); }],
    ['group writable', f => chmodSync(join(f.installation, '2.1.310'), 0o775)],
    ['not executable', f => chmodSync(join(f.installation, '2.1.310'), 0o644)],
    ['shared directory', f => chmodSync(f.directory, 0o755)],
    ['workspace used', f => { mkdirSync(join(f.directory, 'workspace'), { mode: 0o700 }); writeFileSync(join(f.directory, 'workspace', 'x'), ''); }],
    ['other program', f => writeFileSync(join(f.installation, '2.1.310'), claudeSource('codex-cli 0.163.0\n'), { mode: 0o755 })],
    ['version mismatch', f => writeFileSync(join(f.installation, '2.1.310'), claudeSource('2.1.299 (Claude Code)\n'), { mode: 0o755 })],
    ['bad pin', f => writeFileSync(join(f.directory, 'pinned.json'), '{"version":"../../bin/sh"}')],
    ['nothing installed', f => { for (const v of ['2.1.299', '2.1.310', '2.1.99']) rmSync(join(f.installation, v)); }],
    ['long runtime directory', () => {}, { temporary: '/tmp/' + 'x'.repeat(80) }],
  ];
  for (const [name, change, override] of cases) {
    const f = setup(t); change(f);
    assert.throws(() => createClaudeCodeHost({ directory: f.directory, guardian: f.guardian, temporary: override?.temporary ?? f.temporary, installation: f.installation }), Error, name);
  }
});

test('every Claude Code session runs under the guardian with the isolated environment, and kill ends its process group', async t => {
  const f = setup(t); mkdirSync(f.temporary, { mode: 0o700 });
  const launch = guardedClaudeLaunch(f.guardian), executable = join(f.installation, '2.1.310');
  const child = launch({ executable, args: ['-p', '--verbose'], cwd: f.directory, env: { PATH: '/usr/bin:/bin', TMPDIR: f.temporary } });
  const closed = new Promise(resolve => child.stdout.on('close', resolve));
  for (let i = 0; i < 100 && !lstatSync(join(f.temporary, 'guardian.json'), { throwIfNoEntry: false }); i++) await new Promise(resolve => setTimeout(resolve, 20));
  assert.deepEqual(JSON.parse(readFileSync(join(f.temporary, 'guardian.json'), 'utf8')), [String(process.pid), executable, '-p', '--verbose']);
  child.kill();
  await closed;
});
