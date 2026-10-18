import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtempSync, mkdirSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { execFileSync, spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const cli = fileURLToPath(new URL('./validate-reviews.mjs', import.meta.url));
const report = 'docs/architecture/reviews/ponytail/current.md';
const baseline = 'docs/architecture/reviews/ponytail/baseline-20261018.md';
const skill = {
  url: 'https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md',
  sha256: '2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28',
};
const hash = bytes => createHash('sha256').update('100644\0').update(bytes).digest('hex');

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'fort-review-gate-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const git = (...args) => execFileSync('git', ['-c', 'core.hooksPath=/dev/null', '-c', 'commit.gpgsign=false', ...args], { cwd: root, encoding: 'utf8' }).trim();
  const write = (path, text) => { mkdirSync(join(root, path, '..'), { recursive: true }); writeFileSync(join(root, path), text); };
  git('init', '-q'); git('config', 'user.name', 'Synthetic reviewer'); git('config', 'user.email', 'fixture@example.invalid');
  write('source.txt', 'before');
  write(baseline, `What this repo does: A synthetic gate fixture.\n${skill.url}\n${skill.sha256}\nNot checked: No real product.\n`);
  git('add', '.'); git('commit', '-qm', 'Synthetic base');
  const base = git('rev-parse', 'HEAD');
  const metadata = {
    format: 1, reviewedCommit: base, reviewer: 'Synthetic reviewer', skill,
    scope: 'The changed public text and its callers.', findings: [],
    tests: ['Synthetic fixture only.'], exceptions: ['No real semantic review is claimed by this test.'],
    files: { 'source.txt': hash('after') },
  };
  const review = () => write(report, 'What this repo does: A synthetic gate fixture.\n\n```fort-review\n' + JSON.stringify(metadata, null, 2) + '\n```\n\nVerdict: Synthetic only.\nNot checked: Product behavior.\n');
  write('source.txt', 'after'); review(); git('add', '.');
  const run = (...args) => {
    const result = spawnSync(process.execPath, [cli, '--root', root, ...args], { encoding: 'utf8', timeout: 10_000 });
    return { code: result.status, output: result.stdout + result.stderr };
  };
  return { root, git, write, base, metadata, review, run };
}

test('the public CLI accepts a reviewed staged slice and rejects changed contents', t => {
  const f = fixture(t);
  const accepted = f.run('--staged');
  assert.equal(accepted.code, 0, accepted.output);
  assert.match(accepted.output, /evidence only.*semantic review/i);
  f.write('source.txt', 'unreviewed'); f.git('add', 'source.txt');
  const stale = f.run('--staged');
  assert.equal(stale.code, 1); assert.match(stale.output, /source.txt.*stale/i);
});

test('review binds staged content, including the report, rather than unstaged replacements', t => {
  const f = fixture(t);
  f.write('source.txt', 'not in this commit'); f.write(report, 'not staged');
  const checked = f.run('--staged'); assert.equal(checked.code, 0, checked.output);
});

test('a changed executable permission invalidates prior file coverage', t => {
  const f = fixture(t);
  f.git('update-index', '--chmod=+x', 'source.txt');
  const checked = f.run('--staged'); assert.equal(checked.code, 1); assert.match(checked.output, /stale/);
});

test('renames require both removed and new paths, and deletions require null', t => {
  const f = fixture(t);
  f.git('mv', 'source.txt', 'renamed.txt');
  f.metadata.files['renamed.txt'] = hash('after'); f.review(); f.git('add', report);
  assert.equal(f.run('--staged').code, 1);
  f.metadata.files['source.txt'] = null; f.review(); f.git('add', report);
  const checked = f.run('--staged'); assert.equal(checked.code, 0, checked.output);
});

test('all changed paths including documentation need review coverage', t => {
  const f = fixture(t); f.write('notes.md', 'New requirement'); f.git('add', 'notes.md');
  const missing = f.run('--staged'); assert.equal(missing.code, 1); assert.match(missing.output, /notes.md.*missing/);
});

test('missing baseline, missing report, or symlink content cannot supply evidence', t => {
  for (const path of [baseline, report]) {
    const f = fixture(t); f.git('rm', '-f', path);
    const missing = f.run('--staged'); assert.equal(missing.code, 1); assert.match(missing.output, /regular file/);
  }
  const f = fixture(t);
  rmSync(join(f.root, 'source.txt')); symlinkSync('unrelated', join(f.root, 'source.txt')); f.git('add', 'source.txt');
  assert.match(f.run('--staged').output, /regular file/);
});

test('skill provenance, meaningful evidence fields and finding dispositions are required', t => {
  const mutations = [
    review => { review.skill = { ...skill, sha256: '0'.repeat(64) }; },
    review => { review.tests = []; },
    review => { delete review.exceptions; },
    review => { review.findings = [{ id: 'F1', disposition: 'ignored', detail: 'No reason' }]; },
    review => { review.scope = ''; },
    review => { review.files['../outside'] = hash('after'); },
  ];
  for (const mutate of mutations) {
    const f = fixture(t); mutate(f.metadata); f.review(); f.git('add', report);
    assert.equal(f.run('--staged').code, 1);
  }
});

test('committed review checks the whole trusted range and refuses a dirty index', t => {
  const f = fixture(t); f.git('commit', '-qm', 'Reviewed synthetic change');
  const accepted = f.run('--base', f.base); assert.equal(accepted.code, 0, accepted.output);
  f.write('source.txt', 'later'); f.git('add', 'source.txt');
  assert.equal(f.run('--base', f.base).code, 1);
  assert.match(f.run('--staged').output, /commit must match/);
});

test('empty ranges and invalid or missing comparison modes do not claim a review', t => {
  const f = fixture(t); f.git('commit', '-qm', 'Reviewed synthetic change');
  assert.match(f.run('--base', f.git('rev-parse', 'HEAD')).output, /empty comparison/);
  assert.equal(f.run().code, 1); assert.equal(f.run('--base', 'HEAD~1').code, 1);
  assert.equal(f.run('--staged', '--base', f.base).code, 1);
  assert.equal(f.run('--base', '0'.repeat(40)).code, 1);
});
