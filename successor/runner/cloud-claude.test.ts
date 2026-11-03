import { test } from 'node:test';
import assert from 'node:assert/strict';
import { chmodSync, mkdirSync, mkdtempSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { cloudClaudeSignedIn, installedCloudClaude, prepareCloudClaudeDirectory } from './cloud-claude.ts';

// Spec 063, revision 2026-11-03: Claude on the hub, in private folders on the volume, at the image's one exact version.
const scratch = () => realpathSync(mkdtempSync(join(tmpdir(), 'fort-cloud-claude-')));

test('the image names one exact Claude Code version beside a binary only the image can change', () => {
  const root = scratch();
  try {
    assert.equal(installedCloudClaude(root), undefined);
    writeFileSync(join(root, 'version'), 'latest\n');
    assert.throws(() => installedCloudClaude(root), /no exact Claude Code version/);
    writeFileSync(join(root, 'version'), '2.1.311\n');
    assert.throws(() => installedCloudClaude(root), /no usable Claude Code binary/);
    writeFileSync(join(root, '2.1.311'), '#!/bin/sh\n', { mode: 0o777 }); chmodSync(join(root, '2.1.311'), 0o777);
    assert.throws(() => installedCloudClaude(root), /no usable Claude Code binary/, 'a binary anyone can change is refused');
    chmodSync(join(root, '2.1.311'), 0o755);
    assert.deepEqual(installedCloudClaude(root), { version: '2.1.311', executable: join(root, '2.1.311') });
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('the sign-in counts only as a private file in Claude\'s own home, and the workspace stays empty', () => {
  const root = scratch(), directory = join(root, 'claude');
  try {
    prepareCloudClaudeDirectory(directory);
    assert.equal(cloudClaudeSignedIn(directory), false);
    mkdirSync(join(directory, 'home', '.claude'), { mode: 0o700 });
    writeFileSync(join(directory, 'home', '.claude', '.credentials.json'), '{}', { mode: 0o644 });
    assert.equal(cloudClaudeSignedIn(directory), false, 'a readable file is not a sign-in');
    chmodSync(join(directory, 'home', '.claude', '.credentials.json'), 0o600);
    assert.equal(cloudClaudeSignedIn(directory), true);
    writeFileSync(join(directory, 'workspace', 'note.txt'), 'x');
    assert.throws(() => prepareCloudClaudeDirectory(directory), /workspace on the hub must be empty/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});
