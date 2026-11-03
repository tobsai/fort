import { test } from 'node:test';
import assert from 'node:assert/strict';
import { claudeVersionFrom, untarFile } from './install-claude.ts';

// Spec 063, revision 2026-11-03: the hub image installs the Claude version its Claude Agents are bound to.
test('the Claude version comes from the Claude entry in FORT_RUNTIMES, and nothing else', () => {
  const codex = { provider: 'codex', runtimeVersion: '0.163.0', capabilities: [] };
  assert.equal(claudeVersionFrom(undefined), undefined);
  assert.equal(claudeVersionFrom(JSON.stringify([codex])), undefined);
  assert.equal(claudeVersionFrom(JSON.stringify([codex, { provider: 'claude-code', runtimeVersion: '2.1.305', capabilities: [] }])), '2.1.305');
  for (const version of ['latest', '2.1', '2.1.305; rm -rf /', 2])
    assert.throws(() => claudeVersionFrom(JSON.stringify([{ provider: 'claude-code', runtimeVersion: version }])));
  assert.throws(() => claudeVersionFrom('{"provider":"claude-code"}'));
});

function tar(entries: { name: string; body: string; type?: string }[]): Buffer {
  const blocks: Buffer[] = [];
  for (const entry of entries) {
    const header = Buffer.alloc(512), body = Buffer.from(entry.body);
    header.write(entry.name, 0); header.write(body.length.toString(8).padStart(11, '0') + '\0', 124); header.write(entry.type ?? '0', 156);
    blocks.push(header, body, Buffer.alloc((512 - body.length % 512) % 512));
  }
  return Buffer.concat([...blocks, Buffer.alloc(1024)]);
}

test('only the named regular file is read out of the package', () => {
  const archive = tar([{ name: 'package/README.md', body: 'readme' }, { name: 'package/claude', body: 'binary', type: '2' }, { name: 'package/claude', body: 'the binary' }]);
  assert.equal(untarFile(archive, 'package/claude').toString(), 'the binary');
  assert.throws(() => untarFile(archive, 'package/missing'), /has no package\/missing/);
});
