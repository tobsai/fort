import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const cli = fileURLToPath(new URL('./validate-specs.mjs', import.meta.url));
const principles = ['Vendor independence', 'Modularity', 'Speed', 'Installation and removal'];
const hash = text => createHash('sha256').update(text).digest('hex');
const digest = text => hash(text.replace(/^Reviewed content SHA256:.*$/m, 'Reviewed content SHA256: <excluded>'));
const review = (status = 'draft') => `# Example requirement\n\n## Architecture review
Review status: ${status}
Review owner: Toby
Decision reference: ${status === 'accepted' ? 'owner decision in fixture-record.md' : 'pending'}
Reviewed content SHA256: pending
${principles.map(principle => `
### ${principle}
Status: planned
Affected boundaries: Public commands and adapter composition.
Dependency/removal impact: Remove the adapter package; keep core persistence.
Acceptance criteria: Run the shared behavior suite with the adapter absent.
Evidence: Test plan is specified; implementation has not begun.
Tradeoffs: Adapter-specific capabilities remain explicit.
Unresolved judgment: ${status === 'accepted' ? 'none' : 'Owner review of acceptance plan.'}
`).join('')}
## Approach
Keep the public command contract stable.
`;

function fixture(t, text = review()) {
  const root = mkdtempSync(join(tmpdir(), 'fort-spec-gate-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  mkdirSync(join(root, 'specs'), { recursive: true });
  mkdirSync(join(root, 'ops/architecture'), { recursive: true });
  writeFileSync(join(root, 'ops/architecture/historical-specs.json'), JSON.stringify({ schemaVersion: 1, sourceCommit: 'a'.repeat(40), files: {} }));
  const spec = 'specs/059-example.md';
  const write = content => writeFileSync(join(root, spec), content);
  write(text);
  const run = (...args) => {
    const result = spawnSync(process.execPath, [cli, '--root', root, ...args], { encoding: 'utf8' });
    return { code: result.status, output: result.stdout + result.stderr };
  };
  return { root, spec, write, run };
}

test('draft passes structural review but cannot authorize implementation', t => {
  const f = fixture(t);
  assert.equal(f.run().code, 0);
  assert.match(f.run().output, /structure only/i);
  const result = f.run('--implementation', f.spec);
  assert.equal(result.code, 1);
  assert.match(result.output, /requires accepted/);
});

test('all four principles and their evidence fields are required', t => {
  const f = fixture(t, review().replace('### Modularity', '### Extra topic'));
  assert.match(f.run().output, /Modularity/);
  f.write(review().replace('Evidence: Test plan is specified; implementation has not begun.', 'Evidence: TBD'));
  assert.match(f.run().output, /Evidence/);
  assert.equal(f.run().code, 1);
});

test('duplicate status fields and invalid progress states fail', t => {
  const f = fixture(t, review().replace('Status: planned', 'Status: planned\nStatus: verified'));
  assert.match(f.run().output, /exactly one Status/);
  f.write(review().replace('Status: planned', 'Status: compliant'));
  assert.match(f.run().output, /invalid Status/);
});

test('accepted design requires a decision record and a matching whole-document digest', t => {
  const text = review('accepted');
  const f = fixture(t, text.replace('Reviewed content SHA256: pending', `Reviewed content SHA256: ${digest(text)}`));
  assert.equal(f.run('--implementation', f.spec).code, 0);
  assert.equal(f.run('--digest', f.spec).output.trim(), digest(text));
  f.write(readFileSync(join(f.root, f.spec), 'utf8') + '\nChanged requirement.\n');
  assert.match(f.run().output, /content changed/);
  assert.equal(f.run('--implementation', f.spec).code, 1);
  f.write(text.replace('owner decision in fixture-record.md', 'pending'));
  assert.match(f.run().output, /Decision reference/);
});

test('blocked progress or unresolved judgment cannot be accepted', t => {
  const f = fixture(t, review('accepted').replace('Status: planned', 'Status: blocked'));
  assert.match(f.run().output, /blocked/);
  f.write(review('accepted').replace('Unresolved judgment: none', 'Unresolved judgment: Choose a storage design.'));
  assert.match(f.run().output, /unresolved judgment/i);
});

test('not-affected still needs evidence and removal rationale', t => {
  const f = fixture(t, review().replaceAll('Status: planned', 'Status: not-affected'));
  assert.equal(f.run().code, 0);
  f.write(review().replace('Dependency/removal impact: Remove the adapter package; keep core persistence.', 'Dependency/removal impact: N/A'));
  assert.equal(f.run().code, 1);
});

test('unchanged historical specs are preserved, never implementation authority', t => {
  const old = '# Old spec\nStatus: Approved in an earlier architecture.\n';
  const f = fixture(t, old);
  writeFileSync(join(f.root, 'ops/architecture/historical-specs.json'), JSON.stringify({ schemaVersion: 1, sourceCommit: 'a'.repeat(40), files: { [f.spec]: hash(old) } }));
  assert.equal(f.run().code, 0);
  assert.match(f.run().output, /1 historical.*preservation only/);
  assert.equal(f.run('--implementation', f.spec).code, 1);
  f.write(old + 'A new requirement.\n');
  assert.match(f.run().output, /Architecture review/);
  rmSync(join(f.root, f.spec));
  assert.match(f.run().output, /historical file missing/);
});

test('new nested specs are discovered without registration', t => {
  const f = fixture(t);
  mkdirSync(join(f.root, 'specs/decisions'));
  writeFileSync(join(f.root, 'specs/decisions/060-new.md'), '# Unreviewed requirement\n');
  assert.equal(f.run().code, 1);
  assert.match(f.run().output, /060-new.md/);
});

test('headings in fenced examples cannot supply the review', t => {
  const f = fixture(t, '```markdown\n' + review() + '```\n');
  assert.equal(f.run().code, 1);
  assert.match(f.run().output, /Architecture review/);
});

test('invalid CLI paths and options cannot skip review', t => {
  const f = fixture(t);
  assert.equal(f.run('--unknown').code, 1);
  assert.equal(f.run('--implementation', '../outside.md').code, 1);
  assert.equal(f.run('--implementation', 'specs/README.md').code, 1);
  assert.equal(f.run('--implementation').code, 1);
});
