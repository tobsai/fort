import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
import { parseArgs } from 'node:util';

const reportPath = 'docs/architecture/reviews/ponytail/current.md';
const baselinePath = 'docs/architecture/reviews/ponytail/baseline-20261018.md';
const skillURL = 'https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md';
const skillHash = '2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28';
const sha = /^[a-f0-9]{40}$/;
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const text = value => typeof value === 'string' && value.trim().length > 0;
const object = value => value !== null && typeof value === 'object' && !Array.isArray(value);
const pathOK = value => text(value) && !value.startsWith('/') && !value.includes('\\') && !/[\0\r\n]/.test(value)
  && !value.split('/').some(part => part === '' || part === '.' || part === '..');
const require = (condition, message) => { if (!condition) throw new Error(message); };

function validate() {
  const { values } = parseArgs({ options: { root: { type: 'string' }, staged: { type: 'boolean' }, base: { type: 'string' } }, strict: true });
  require(Boolean(values.staged) !== Boolean(values.base), 'Select --staged or --base <trusted full commit SHA>');
  const root = resolve(values.root ?? fileURLToPath(new URL('../../', import.meta.url)));
  const git = (...args) => execFileSync('git', args, { cwd: root, timeout: 5000, maxBuffer: 16 * 1024 * 1024, stdio: ['ignore', 'pipe', 'pipe'] });
  const head = git('rev-parse', 'HEAD').toString().trim();
  const base = values.staged ? head : values.base;
  require(sha.test(base), 'The comparison base must be a full commit SHA');
  git('merge-base', '--is-ancestor', base, head);
  if (!values.staged) git('diff', '--quiet', '--cached', 'HEAD', '--');
  const entries = new Map();
  const listing = values.staged ? git('ls-files', '--stage', '-z') : git('ls-tree', '-r', '-z', 'HEAD');
  for (const entry of listing.toString().split('\0').filter(Boolean)) {
    const match = values.staged ? /^(\d+) ([a-f0-9]+) (\d)\t([\s\S]+)$/.exec(entry) : /^(\d+) blob ([a-f0-9]+)\t([\s\S]+)$/.exec(entry);
    require(match, 'The snapshot contains an unsupported Git entry');
    if (values.staged) require(match[3] === '0', 'Resolve unmerged files before review');
    entries.set(match[values.staged ? 4 : 3], { mode: match[1], oid: match[2] });
  }
  const read = path => {
    require(pathOK(path), 'Review contains an invalid repository path');
    const entry = entries.get(path);
    require(entry && ['100644', '100755'].includes(entry.mode), path + ': expected a regular file in the selected snapshot');
    return git('cat-file', 'blob', entry.oid);
  };
  const baseline = read(baselinePath).toString();
  require(baseline.startsWith('What this repo does:') && baseline.includes(skillURL) && baseline.includes(skillHash)
    && baseline.includes('Not checked:'), 'The baseline needs scope, immutable skill provenance and omissions');
  const document = read(reportPath).toString();
  const blocks = [...document.matchAll(/^```fort-review\n([\s\S]*?)\n```$/gm)];
  require(blocks.length === 1, 'The current report needs exactly one fort-review metadata block');
  const review = JSON.parse(blocks[0][1]);
  require(object(review) && review.format === 1, 'Unsupported review format');
  require(text(review.reviewer) && text(review.scope), 'Review needs reviewer and scope');
  require(sha.test(review.reviewedCommit), 'Review needs a full reviewed commit SHA');
  if (values.staged) require(review.reviewedCommit === head, 'Review commit must match staged HEAD');
  else git('merge-base', '--is-ancestor', review.reviewedCommit, head);
  require(review.skill?.url === skillURL && review.skill?.sha256 === skillHash, 'Review skill pin does not match the required immutable source');
  require(Array.isArray(review.tests) && review.tests.length > 0 && review.tests.every(text), 'Review needs test evidence, including unrun checks');
  require(Array.isArray(review.exceptions) && review.exceptions.every(text), 'Review must list exceptions or an empty array');
  require(Array.isArray(review.findings) && review.findings.length <= 20 && review.findings.every(finding => object(finding)
    && text(finding.id) && text(finding.detail) && ['fixed', 'open', 'accepted-exception'].includes(finding.disposition)), 'Findings need identifiers, dispositions and details');
  require(document.startsWith('What this repo does:') && document.includes('Verdict:') && document.includes('Not checked:'), 'Review needs its semantic report and stated limits');
  require(object(review.files), 'Review needs changed-file hashes');
  for (const [path, digest] of Object.entries(review.files)) require(pathOK(path) && (digest === null || /^[a-f0-9]{64}$/.test(digest)), 'Review contains an invalid file hash');
  const args = ['diff', '--name-only', '-z', '--no-renames', ...(values.staged ? ['--cached'] : []), base, ...(values.staged ? [] : ['HEAD']), '--'];
  const changed = git(...args).toString().split('\0').filter(path => path && path !== reportPath);
  require(changed.length > 0, 'An empty comparison does not prove a change review');
  for (const path of changed) {
    require(Object.hasOwn(review.files, path), path + ': missing review coverage');
    const expected = entries.has(path) ? hash(Buffer.concat([Buffer.from(entries.get(path).mode + '\0'), read(path)])) : null;
    require(review.files[path] === expected, path + ': stale review coverage');
  }
  const open = review.findings.filter(finding => finding.disposition === 'open').length;
  process.stdout.write(`PASS: ${changed.length} changed paths covered; ${open} open findings. Evidence only; semantic review and completion judgment remain required.\n`);
}

try { validate(); }
catch (error) {
  process.stderr.write('FAIL: ' + (error instanceof Error ? error.message : 'Review validation failed') + '\n');
  process.exitCode = 1;
}
