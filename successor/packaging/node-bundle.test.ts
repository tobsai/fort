import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import { mkdtempSync, readdirSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const identity = process.env.FORT_SIGNING_IDENTITY;
test('packaged Node runs SQLite without Homebrew load paths and retains attribution', { skip: process.platform !== 'darwin' ? 'requires macOS' : !identity ? 'requires explicitly selected existing signing identity' : false }, t => {
  assert.ok(identity);
  const root = mkdtempSync('/tmp/fort059-node-bundle-');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const destination = join(root, 'runtime');
  const build = spawnSync(process.execPath, [fileURLToPath(new URL('./node-bundle.ts', import.meta.url)), '--runtime', process.execPath, '--destination', destination, '--identity', identity], { encoding: 'utf8', timeout: 60_000 });
  assert.equal(build.status, 0, build.stderr);
  const executable = join(destination, 'bin/node');
  const result = spawnSync(executable, ['--jitless', '--no-experimental-strip-types', '--no-warnings', '-e', "const {DatabaseSync}=require('node:sqlite');const db=new DatabaseSync(':memory:');db.exec('create table proof(value text)');db.prepare('insert into proof values (?)').run('ready');process.stdout.write(process.version+'|'+db.prepare('select value from proof').get().value);db.close();"], { encoding: 'utf8', timeout: 10_000, cwd: root, env: { PATH: '/usr/bin:/bin', HOME: root, TMPDIR: root } });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, 'v26.5.0|ready');
  const libraries = readdirSync(join(destination, 'lib'));
  assert.ok(libraries.length > 0);
  for (const binary of [executable, ...libraries.map(name => join(destination, 'lib', name))]) {
    const dependencies = execFileSync('/usr/bin/otool', ['-L', binary], { encoding: 'utf8' });
    assert.doesNotMatch(dependencies, /\/opt\/homebrew/);
    execFileSync('/usr/bin/codesign', ['--verify', '--strict', binary]);
  }
  assert.ok(readdirSync(join(destination, 'licenses')).length > 1);
});
