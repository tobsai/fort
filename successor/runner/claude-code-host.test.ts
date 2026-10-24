import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { chmodSync, copyFileSync, lstatSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
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
if (args[0] === 'auth') { process.stdout.write(JSON.stringify({ loggedIn: false, authMethod: 'none', apiProvider: 'firstParty' })); process.exit(1); }
process.stdin.on('data', () => {}); setInterval(() => {}, 1000);
`;

function setup(t: TestContext, version = '2.1.299 (Claude Code)\n') {
  const root = realpathSync(mkdtempSync('/tmp/fcch-')); t.after(() => rmSync(root, { recursive: true, force: true }));
  const data = join(root, 'data'), directory = join(data, CLAUDE_CODE_DIRECTORY), temporary = join(root, 't'), guardian = join(root, 'guardian');
  mkdirSync(data, { mode: 0o700 }); mkdirSync(directory, { mode: 0o700 });
  writeFileSync(join(directory, 'claude'), claudeSource(version), { mode: 0o700 });
  writeFileSync(guardian, guardianSource, { mode: 0o700 });
  return { root, data, directory, temporary, guardian };
}

test('the Claude-only mode is selected only by a copied binary in the host data directory', t => {
  const f = setup(t);
  assert.equal(selectedClaudeCode(f.data), f.directory);
  rmSync(join(f.directory, 'claude'));
  assert.equal(selectedClaudeCode(f.data), undefined);
});

test('the host pins the copied binary\'s exact version and keeps its own private ledger, home and workspace', t => {
  const f = setup(t), host = createClaudeCodeHost({ directory: f.directory, guardian: f.guardian, temporary: f.temporary });
  assert.deepEqual(host.provider, { provider: 'claude-code', name: 'Claude', runtimeVersion: '2.1.299', hostId: 'local-claude-code', session: 'stateless', capabilities: ['plainText', 'modelCatalog', 'cancel', 'skills', 'memory'] });
  assert.equal(host.dataDirectory, join(f.directory, 'data'));
  for (const path of ['data', 'home', 'workspace']) assert.equal(lstatSync(join(f.directory, path)).mode & 0o777, 0o700, path);
  assert.equal(lstatSync(f.temporary).mode & 0o777, 0o700);
});

test('the host refuses links, shared or drifted binaries, a non-empty workspace and an over-long runtime directory', t => {
  const cases: [string, (f: ReturnType<typeof setup>) => void, Partial<{ temporary: string }>?][] = [
    ['symlink', f => { copyFileSync(join(f.directory, 'claude'), join(f.root, 'real')); rmSync(join(f.directory, 'claude')); symlinkSync(join(f.root, 'real'), join(f.directory, 'claude')); }],
    ['group writable', f => chmodSync(join(f.directory, 'claude'), 0o720)],
    ['not executable', f => chmodSync(join(f.directory, 'claude'), 0o600)],
    ['shared directory', f => chmodSync(f.directory, 0o755)],
    ['workspace used', f => { mkdirSync(join(f.directory, 'workspace'), { mode: 0o700 }); writeFileSync(join(f.directory, 'workspace', 'x'), ''); }],
    ['other program', f => writeFileSync(join(f.directory, 'claude'), claudeSource('codex-cli 0.163.0\n'), { mode: 0o700 })],
    ['long runtime directory', () => {}, { temporary: '/tmp/' + 'x'.repeat(80) }],
  ];
  for (const [name, change, override] of cases) {
    const f = setup(t); change(f);
    assert.throws(() => createClaudeCodeHost({ directory: f.directory, guardian: f.guardian, temporary: override?.temporary ?? f.temporary }), Error, name);
  }
});

test('every Claude Code session runs under the guardian with the isolated environment, and kill ends its process group', async t => {
  const f = setup(t); mkdirSync(f.temporary, { mode: 0o700 });
  const launch = guardedClaudeLaunch(f.guardian), executable = join(f.directory, 'claude');
  const child = launch({ executable, args: ['-p', '--verbose'], cwd: f.directory, env: { PATH: '/usr/bin:/bin', TMPDIR: f.temporary } });
  const closed = new Promise(resolve => child.stdout.on('close', resolve));
  for (let i = 0; i < 100 && !lstatSync(join(f.temporary, 'guardian.json'), { throwIfNoEntry: false }); i++) await new Promise(resolve => setTimeout(resolve, 20));
  assert.deepEqual(JSON.parse(readFileSync(join(f.temporary, 'guardian.json'), 'utf8')), [String(process.pid), executable, '-p', '--verbose']);
  child.kill();
  await closed;
});
