import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, realpathSync, rmSync, mkdirSync, writeFileSync, readFileSync, readdirSync, chmodSync, symlinkSync, linkSync, lstatSync, truncateSync } from 'node:fs';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { privateCodexLaunchPlan } from './private-codex-plan.ts';

function fixture(t: TestContext) {
  const root = realpathSync(mkdtempSync('/tmp/fort059-private-codex-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const directory = join(root, 'owned'); mkdirSync(directory, { mode: 0o700 });
  for (const name of ['home', 'state', 'workspace', 'tmp']) mkdirSync(join(directory, name), { mode: 0o700 });
  const executable = join(root, 'synthetic-runtime'), guardian = join(root, 'guardian');
  writeFileSync(executable, `#!${process.execPath}\nprocess.stdout.write(JSON.stringify({args:process.argv.slice(2),env:process.env,cwd:process.cwd()}));`, { mode: 0o700 });
  writeFileSync(guardian, 'synthetic executable identifier only', { mode: 0o700 });
  return { root, directory, executable, guardian };
}

test('the private plan starts only the exact synthetic program with fixed controls and a private environment', t => {
  const f = fixture(t);
  const prior = process.env.FORT_SYNTHETIC_AMBIENT;
  process.env.FORT_SYNTHETIC_AMBIENT = 'must-not-inherit';
  try {
    const plan = privateCodexLaunchPlan(f);
    assert.equal(plan.executable, f.executable); assert.equal(plan.guardian, f.guardian);
    assert.equal(plan.cwd, join(f.directory, 'workspace'));
    assert.deepEqual(plan.env, { HOME: join(f.directory, 'home'), CODEX_HOME: join(f.directory, 'state'), TMPDIR: join(f.directory, 'tmp'), PATH: '/usr/bin:/bin', LANG: 'en_US.UTF8', TZ: 'UTC' });
    assert.equal(Object.isFrozen(plan), true); assert.equal(Object.isFrozen(plan.args), true); assert.equal(Object.isFrozen(plan.env), true);
    const child = spawnSync(plan.executable, [...plan.args], { cwd: plan.cwd, env: plan.env, encoding: 'utf8', timeout: 3000 });
    assert.equal(child.status, 0, child.stderr);
    const observed = JSON.parse(child.stdout);
    if (process.platform === 'darwin' && Object.hasOwn(observed.env, '__CF_USER_TEXT_ENCODING')) {
      assert.match(observed.env.__CF_USER_TEXT_ENCODING, /^0x[\da-f]+:0x[\da-f]+:0x[\da-f]+$/i);
      delete observed.env.__CF_USER_TEXT_ENCODING;
    }
    assert.deepEqual(observed, { args: plan.args, env: plan.env, cwd: plan.cwd });
    assert.deepEqual(observed.args.slice(0, 3), ['app-server', '--listen', 'stdio://']);
    const settings = new Map<string, string>();
    for (let index = 3; index < plan.args.length; index += 2) {
      if (plan.args[index] !== '-c') continue;
      const setting = plan.args[index + 1]; assert.ok(setting);
      const separator = setting.indexOf('='); settings.set(setting.slice(0, separator), setting.slice(separator + 1));
    }
    for (const [name, expected] of Object.entries({ 'analytics.enabled': false, check_for_update_on_startup: false,
      forced_login_method: 'chatgpt', cli_auth_credentials_store: 'file', mcp_oauth_credentials_store: 'file',
      'otel.exporter': 'none', 'otel.trace_exporter': 'none', 'otel.metrics_exporter': 'none', 'otel.log_user_prompt': false,
      'skills.include_instructions': false, 'cloud.skills.enabled': false, 'orchestrator.mcp.enabled': false,
      default_permissions: 'fort_candidate_b_no_files' })) assert.equal(settings.get(name), JSON.stringify(expected));
    assert.equal(settings.get('permissions.fort_candidate_b_no_files'), '{filesystem={"/**"="deny"},network={enabled=false}}');
    assert.ok(plan.args.includes('skip_host_skill_discovery'));
    for (const feature of ['apps', 'plugins', 'hooks', 'remote_control', 'shell_tool', 'computer_use', 'browser_use', 'image_generation']) {
      const index = plan.args.indexOf(feature); assert.ok(index > 0); assert.equal(plan.args[index - 1], '--disable');
    }
    assert.deepEqual(readdirSync(join(f.directory, 'state')), []);
  } finally {
    if (prior === undefined) delete process.env.FORT_SYNTHETIC_AMBIENT; else process.env.FORT_SYNTHETIC_AMBIENT = prior;
  }
});

test('private launch validation rejects unsafe paths without repairing or creating locations', async t => {
  const f = fixture(t);
  const paths = { relative: 'relative', root: '/', traversal: f.root + '/child/../owned', NUL: f.root + '\0secret', surrogate: f.root + '/\ud800', oversized: '/' + 'x'.repeat(4096) };
  for (const key of ['directory', 'guardian', 'executable'] as const) for (const [label, value] of Object.entries(paths)) {
    await t.test(key + ':' + label, () => {
      assert.throws(() => privateCodexLaunchPlan({ ...f, [key]: value }));
    });
  }
  assert.deepEqual(readdirSync(f.directory).sort(), ['home', 'state', 'tmp', 'workspace']);
});

test('a directory belonging to a different reported user is refused', t => {
  const f = fixture(t); assert.ok(process.getuid);
  const original = process.getuid, current = original();
  process.getuid = () => current + 1; t.after(() => { process.getuid = original; });
  assert.throws(() => privateCodexLaunchPlan(f));
  assert.deepEqual(readdirSync(join(f.directory, 'state')), []);
});

test('each required directory must exist, be canonical and remain owner-only', async t => {
  for (const name of ['', 'home', 'state', 'workspace', 'tmp']) for (const fault of ['missing', 'file', 'symlink', 'nonprivate', 'readonly']) await t.test((name || 'root') + ':' + fault, t => {
    const f = fixture(t), path = join(f.directory, name);
    if (fault === 'nonprivate' || fault === 'readonly') chmodSync(path, fault === 'nonprivate' ? 0o750 : 0o500);
    else {
      rmSync(path, { recursive: true });
      if (fault === 'file') writeFileSync(path, 'synthetic unrelated', { mode: 0o600 });
      if (fault === 'symlink') { const target = join(f.root, 'other'); mkdirSync(target, { mode: 0o700 }); symlinkSync(target, path); }
    }
    const before = lstatSync(path, { throwIfNoEntry: false });
    assert.throws(() => privateCodexLaunchPlan(f));
    const after = lstatSync(path, { throwIfNoEntry: false });
    assert.equal(after?.ino, before?.ino); assert.equal(after?.mode, before?.mode);
    if (fault === 'file') assert.equal(readFileSync(path, 'utf8'), 'synthetic unrelated');
    if (fault === 'readonly') chmodSync(path, 0o700);
  });
});

test('a symlink ancestor and nonempty runtime home or workspace are refused', async t => {
  const f = fixture(t), alias = join(f.root, 'alias'); symlinkSync(f.directory, alias);
  assert.throws(() => privateCodexLaunchPlan({ ...f, directory: alias }));
  for (const name of ['home', 'workspace']) await t.test(name, t => {
    const h = fixture(t), file = join(h.directory, name, 'keep'); writeFileSync(file, 'synthetic unrelated');
    assert.throws(() => privateCodexLaunchPlan(h)); assert.equal(readFileSync(file, 'utf8'), 'synthetic unrelated');
  });
});

test('dotenv and configuration entries are refused without reading or removing their contents', async t => {
  for (const name of ['.env', 'config.toml']) for (const kind of ['file', 'directory', 'dangling-link']) await t.test(name + ':' + kind, t => {
    const f = fixture(t), path = join(f.directory, 'state', name);
    if (kind === 'file') writeFileSync(path, 'synthetic private marker', { mode: 0o600 });
    else if (kind === 'directory') mkdirSync(path, { mode: 0o700 });
    else symlinkSync(join(f.root, 'missing'), path);
    const before = lstatSync(path);
    assert.throws(() => privateCodexLaunchPlan(f), error => error instanceof Error && !error.message.includes('synthetic private marker'));
    assert.equal(lstatSync(path).ino, before.ino);
    if (kind === 'file') assert.equal(readFileSync(path, 'utf8'), 'synthetic private marker');
  });
});

test('synthetic credential metadata is inspected without parsing, copying or repairing it', async t => {
  const f = fixture(t), file = join(f.directory, 'state', 'auth.json');
  writeFileSync(file, 'not a credential or JSON; opaque synthetic bytes', { mode: 0o600 });
  const original = readFileSync(file), inode = lstatSync(file).ino;
  privateCodexLaunchPlan(f);
  assert.deepEqual(readFileSync(file), original); assert.equal(lstatSync(file).ino, inode);
  assert.deepEqual(readdirSync(join(f.directory, 'state')), ['auth.json']);
  for (const kind of ['directory', 'symlink', 'hardlink', 'nonprivate', 'empty', 'oversized']) await t.test(kind, t => {
    const h = fixture(t), path = join(h.directory, 'state', 'auth.json');
    if (kind === 'directory') mkdirSync(path, { mode: 0o700 });
    else if (kind === 'symlink' || kind === 'hardlink') {
      const target = join(h.root, 'unrelated'); writeFileSync(target, 'synthetic unrelated', { mode: 0o600 });
      if (kind === 'symlink') symlinkSync(target, path); else linkSync(target, path);
    } else {
      writeFileSync(path, kind === 'empty' ? '' : 'synthetic opaque', { mode: kind === 'nonprivate' ? 0o640 : 0o600 });
      if (kind === 'oversized') truncateSync(path, 128 * 1024 + 1);
    }
    const before = lstatSync(path); assert.throws(() => privateCodexLaunchPlan(h));
    const after = lstatSync(path); assert.equal(after.ino, before.ino); assert.equal(after.mode, before.mode); assert.equal(after.size, before.size);
  });
});

test('twenty isolated private plan inspections meet the metadata budget', { skip: !process.env.FORT_PRIVATE_CODEX_PLAN_BENCHMARK }, t => {
  const destination = process.env.FORT_PRIVATE_CODEX_PLAN_BENCHMARK; assert.ok(destination);
  const samples: number[] = [];
  for (let index = 0; index < 20; index++) {
    const f = fixture(t), start = performance.now(); privateCodexLaunchPlan(f); samples.push(performance.now() - start);
  }
  writeFileSync(destination, JSON.stringify(samples), { flag: 'wx' });
  assert.ok(samples.toSorted((a, b) => a - b)[18]! <= 5);
});
