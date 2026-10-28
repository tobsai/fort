import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { chmod, mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

// The Mac runner scripts, run against stand-ins for the macOS tools so the test needs no Mac.
const check = fileURLToPath(new URL('./mac-runner-check.sh', import.meta.url));
const power = fileURLToPath(new URL('./mac-runner-power.sh', import.meta.url));
const MINI = `AC Power:
 sleep                0
 disksleep            0
 womp                 1
 autorestart          1
`;

async function mac(t: TestContext, tools: Record<string, string>) {
  const bin = await mkdtemp(join(tmpdir(), 'fort-setup-'));
  t.after(() => rm(bin, { recursive: true, force: true }));
  const all: Record<string, string> = { sw_vers: 'echo 26.1', scutil: 'echo "Mac mini"', fdesetup: 'echo "FileVault is On."',
    pmset: `case "$2" in custom) printf '%s' "${MINI}";; ps) echo "Now drawing from 'AC Power' -UPS";; esac`,
    pgrep: 'exit 0', curl: 'echo \'{"google":true}\'', sudo: 'echo "sudo $*" >> "$SUDO_LOG"', ...tools };
  for (const [name, body] of Object.entries(all)) { await writeFile(join(bin, name), '#!/bin/bash\n' + body + '\n'); await chmod(join(bin, name), 0o755); }
  return (script: string, args: string[], home = bin) => spawnSync('/bin/bash', [script, ...args], { encoding: 'utf8',
    env: { PATH: `${bin}:/usr/bin:/bin`, HOME: home, SUDO_LOG: join(bin, 'sudo.log') } });
}

test('both scripts parse as bash', () => {
  for (const script of [check, power]) assert.equal(spawnSync('/bin/bash', ['-n', script]).status, 0);
});

test('a ready mini passes, with the FileVault restart caveat spelled out', async t => {
  const run = await mac(t, {});
  const result = run(check, ['--role', 'primary'], '/');
  assert.match(result.stdout, /FIX   Fort isn't in Applications/);
  const home = await mkdtemp(join(tmpdir(), 'fort-home-'));
  t.after(() => rm(home, { recursive: true, force: true }));
  await mkdir(join(home, 'Applications/Fort.app/Contents/Helpers/FortHost.app'), { recursive: true });
  const ready = run(check, ['--role', 'primary'], home);
  assert.equal(ready.status, 0, ready.stdout);
  assert.match(ready.stdout, /ok    The Mac never sleeps on power\./);
  assert.match(ready.stdout, /ok    A UPS is connected\./);
  assert.match(ready.stdout, /waits at the unlock screen/);
  assert.match(ready.stdout, /Nothing to fix\./);
});

test('a mini that sleeps, has FileVault off and can’t reach the hub gets told what to fix', async t => {
  const run = await mac(t, { fdesetup: 'echo "FileVault is Off."', pmset: `printf 'AC Power:\\n sleep 10\\n autorestart 0\\n'`, curl: 'exit 7', pgrep: 'exit 1' });
  const result = run(check, ['--role', 'primary']);
  assert.equal(result.status, 1);
  for (const expected of [/FIX   FileVault is off/, /FIX   The Mac sleeps after 10 minutes/, /FIX   It won't start up after a power failure/, /FIX   The Fort host isn't running/, /FIX   https:\/\/fort\.mtree\.io didn't answer/])
    assert.match(result.stdout, expected);
});

test('the laptop is checked lightly and its power settings are left alone', async t => {
  const run = await mac(t, { fdesetup: 'echo "FileVault is Off."' });
  const result = run(check, ['--role', 'overflow']);
  assert.doesNotMatch(result.stdout, /Power \(on AC\)/);
  assert.match(result.stdout, /note  FileVault is off on this laptop\./);
  assert.equal(run(check, ['--role', 'cloud']).status, 2);
  assert.equal(run(check, ['--hub', 'http://fort.mtree.io']).status, 2);
});

test('the power script changes nothing unless asked, then sets the agreed values', async t => {
  const run = await mac(t, {});
  const preview = run(power, []);
  assert.equal(preview.status, 0);
  assert.match(preview.stdout, /sudo pmset -c sleep 0 disksleep 0 displaysleep 15 womp 1 autorestart 1 powernap 0 tcpkeepalive 1/);
  assert.match(preview.stdout, /sudo pmset -u haltlevel 20/);
  assert.match(preview.stdout, /Nothing changed\./);
  const applied = run(power, ['--apply', '--halt-at', '30']);
  assert.equal(applied.status, 0, applied.stderr);
  assert.match(applied.stdout, /Done\./);
  assert.equal(run(power, ['--halt-at', '2']).status, 2);
  assert.equal(run(power, ['--halt-at', 'lots']).status, 2);
});
