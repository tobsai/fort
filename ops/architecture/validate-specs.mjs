import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const principles = ['Vendor independence', 'Modularity', 'Speed', 'Installation and removal'];
const fields = ['Status', 'Affected boundaries', 'Dependency/removal impact', 'Acceptance criteria', 'Evidence', 'Tradeoffs', 'Unresolved judgment'];
const reviewStates = ['draft', 'needs-reassessment', 'ready-for-review', 'accepted', 'rejected'];
const progressStates = ['planned', 'implemented-unverified', 'verified', 'blocked', 'not-affected'];
const excluded = new Set(['specs/README.md', 'specs/TEMPLATE.md']);
const sha256 = text => createHash('sha256').update(text).digest('hex');
const digest = text => sha256(text.replace(/^Reviewed content SHA256:.*$/m, 'Reviewed content SHA256: <excluded>'));
const substantive = value => value && !/^(pending|todo|tbd|n\/a|none|<[^>]+>|\[[^\]]+\])\.?$/i.test(value.trim());

function visibleLines(text) {
  let fence = null;
  return text.split('\n').map(line => {
    const mark = line.match(/^\s{0,3}(`{3,}|~{3,})/);
    if (mark && !fence) { fence = mark[1]; return ''; }
    if (mark && fence && mark[1][0] === fence[0] && mark[1].length >= fence.length) { fence = null; return ''; }
    return fence ? '' : line;
  });
}

function section(lines, title, level) {
  const heading = '#'.repeat(level) + ' ' + title;
  const starts = lines.flatMap((line, index) => line.trimEnd() === heading ? [index] : []);
  if (starts.length !== 1) throw new Error(`expected exactly one ${heading}`);
  const rest = lines.slice(starts[0] + 1);
  const end = rest.findIndex(line => new RegExp(`^#{1,${level}} `).test(line));
  return rest.slice(0, end === -1 ? rest.length : end);
}

function field(lines, name) {
  const prefix = name + ':';
  const starts = lines.flatMap((line, index) => line.startsWith(prefix) ? [index] : []);
  if (starts.length !== 1) throw new Error(`expected exactly one ${name} field`);
  const rest = lines.slice(starts[0] + 1);
  const end = rest.findIndex(line => /^[A-Z][A-Za-z /0-9-]*:|^#{1,6} /.test(line));
  return [lines[starts[0]].slice(prefix.length), ...rest.slice(0, end === -1 ? rest.length : end)].join('\n').trim();
}

function validate(text, implementation) {
  const lines = section(visibleLines(text), 'Architecture review', 2);
  const metadata = lines.slice(0, lines.findIndex(line => line.startsWith('### ')));
  const status = field(metadata, 'Review status');
  if (!reviewStates.includes(status)) throw new Error('invalid Review status');
  if (!substantive(field(metadata, 'Review owner'))) throw new Error('Review owner is required');
  const reference = field(metadata, 'Decision reference');
  const reviewedDigest = field(metadata, 'Reviewed content SHA256');
  if (!reference || !reviewedDigest) throw new Error('decision metadata is required; use pending before acceptance');
  for (const principle of principles) {
    const block = section(lines, principle, 3);
    const values = Object.fromEntries(fields.map(name => [name, field(block, name)]));
    if (!progressStates.includes(values.Status)) throw new Error(`${principle}: invalid Status`);
    for (const name of fields.filter(name => name !== 'Status' && name !== 'Unresolved judgment')) {
      if (!substantive(values[name])) throw new Error(`${principle}: ${name} needs substantive content`);
    }
    if (!values['Unresolved judgment']) throw new Error(`${principle}: Unresolved judgment is required`);
    if (status === 'accepted' && values.Status === 'blocked') throw new Error(`${principle}: blocked work cannot be accepted`);
    if (status === 'accepted' && values['Unresolved judgment'] !== 'none') throw new Error(`${principle}: unresolved judgment prevents acceptance`);
  }
  if (status === 'accepted') {
    if (!substantive(reference)) throw new Error('accepted review needs a Decision reference');
    if (!/^[a-f0-9]{64}$/.test(reviewedDigest) || digest(text) !== reviewedDigest) {
      throw new Error('reviewed content changed or digest missing; reassess before acceptance');
    }
  }
  if (implementation && status !== 'accepted') throw new Error(`implementation requires accepted review; found ${status}`);
  return status;
}

function specFiles(root, directory = 'specs') {
  return readdirSync(resolve(root, directory), { withFileTypes: true }).flatMap(entry => {
    const path = `${directory}/${entry.name}`;
    if (entry.isSymbolicLink()) throw new Error(`spec symlinks are not supported: ${path}`);
    if (entry.isDirectory()) return specFiles(root, path);
    return entry.isFile() && path.endsWith('.md') && !excluded.has(path) ? [path] : [];
  }).sort();
}

const git = (root, ...args) => execFileSync('git', ['--literal-pathspecs', ...args], { cwd: root, encoding: 'utf8', maxBuffer: 10 * 1024 * 1024 });

function checkStagedPaths(root, file, text, base) {
  const entries = section(visibleLines(text), 'Implementation paths', 2).filter(line => line.trim());
  const declared = new Set([file]);
  if (!entries.length) throw new Error('Implementation paths requires exact repository-relative file paths');
  for (const line of entries) {
    const match = line.match(/^- `([^`]+)`$/);
    const path = match?.[1];
    if (!path || /[\\*?\[\]\x00-\x1f]/.test(path) || path.split('/').some(part => ['', '.', '..'].includes(part))) {
      throw new Error('Implementation paths requires exact repository-relative file paths; no globs or traversal');
    }
    if (declared.has(path)) throw new Error(`duplicate implementation path: ${path}`);
    declared.add(path);
  }
  const baseCommit = git(root, 'rev-parse', '--verify', '--end-of-options', `${base}^{commit}`).trim();
  const changed = git(root, 'diff', '--cached', '--no-ext-diff', '--no-renames', '--name-only', '-z', baseCommit, '--').split('\0').filter(Boolean);
  const missing = changed.filter(path => !declared.has(path));
  if (missing.length) throw new Error(missing.map(path => `${path}: not declared by ${file}`).join('\n'));
  console.log(`${changed.length} staged path(s) declared by ${file}, relative to ${baseCommit}. Unstaged and untracked files are excluded; approval is not authenticated.`);
}

function options(args) {
  let root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
  let mode = 'check';
  let target;
  let staged = false;
  let base = 'HEAD';
  const seen = new Set();
  while (args.length) {
    const flag = args.shift();
    if (!['--root', '--implementation', '--digest', '--staged', '--base'].includes(flag) || seen.has(flag)) throw new Error(`invalid option: ${flag}`);
    seen.add(flag);
    if (flag === '--staged') { staged = true; continue; }
    const value = args.shift();
    if (!value || value.startsWith('--')) throw new Error(`missing value for ${flag}`);
    if (flag === '--root') root = resolve(value);
    else if (flag === '--base') {
      if (!/^[a-f0-9]{40}$/.test(value)) throw new Error('--base requires a full commit SHA');
      base = value;
    }
    else {
      if (target) throw new Error('choose either --implementation or --digest');
      mode = flag.slice(2);
      target = value;
    }
  }
  if (staged && mode !== 'implementation') throw new Error('--staged requires --implementation');
  if (seen.has('--base') && !staged) throw new Error('--base requires --staged');
  return { root, mode, target, staged, base };
}

try {
  const { root, mode, target, staged, base } = options(process.argv.slice(2));
  const files = specFiles(root);
  if (target && !files.includes(target)) throw new Error('target must be a discovered spec Markdown file');
  if (mode === 'digest') {
    const text = readFileSync(resolve(root, target), 'utf8');
    field(section(visibleLines(text), 'Architecture review', 2), 'Reviewed content SHA256');
    console.log(digest(text));
  } else {
    const manifest = JSON.parse(readFileSync(resolve(root, 'ops/architecture/historical-specs.json'), 'utf8'));
    if (manifest.schemaVersion !== 1 || !/^[a-f0-9]{40}$/.test(manifest.sourceCommit) || !manifest.files || typeof manifest.files !== 'object' || Array.isArray(manifest.files)) {
      throw new Error('invalid historical manifest');
    }
    const errors = [];
    for (const [file, hash] of Object.entries(manifest.files)) {
      if (!files.includes(file)) errors.push(`${file}: historical file missing`);
      if (!/^[a-f0-9]{64}$/.test(hash)) errors.push(`${file}: invalid historical digest`);
    }
    let historical = 0;
    let reviewed = 0;
    for (const file of target ? [target] : files) {
      try {
        const text = readFileSync(resolve(root, file), 'utf8');
        if (staged && text !== git(root, 'show', `:${file}`)) throw new Error('spec differs from the index; stage the reviewed spec before checking staged implementation');
        if (sha256(text) === manifest.files[file]) {
          if (mode === 'implementation') throw new Error('historical preservation is not implementation authority; add an architecture review');
          historical++;
        } else {
          const status = validate(text, mode === 'implementation');
          if (staged) checkStagedPaths(root, file, text, base);
          console.log(`${file}: ${status}`);
          reviewed++;
        }
      } catch (error) { errors.push(`${file}: ${error.message}`); }
    }
    if (errors.length) throw new Error(errors.join('\n'));
    console.log(`PASS: ${reviewed} review record(s), ${historical} historical file(s) (preservation only). Structure only; human architectural judgment and behavioral verification remain required.`);
  }
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
}
