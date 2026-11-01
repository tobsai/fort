import assert from 'node:assert/strict';
import { lstatSync, mkdirSync, mkdtempSync, rmSync, statSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { makeReadable } from './mac-sign.ts';

// App Store Connect refused a Fort.pkg whose files only root could read (ITMS-90255).
test('opens an assembled app to every reader before signing, keeping executables executable', () => {
  const root = mkdtempSync(join(tmpdir(), 'fort-readable-'));
  try {
    const app = join(root, 'Fort.app'), macos = join(app, 'Contents/MacOS');
    mkdirSync(macos, { recursive: true, mode: 0o700 });
    writeFileSync(join(app, 'Contents/Info.plist'), 'plist', { mode: 0o600 });
    writeFileSync(join(macos, 'Fort'), 'code', { mode: 0o700 });
    writeFileSync(join(root, 'outside'), 'not in the app', { mode: 0o600 });
    symlinkSync('../Info.plist', join(macos, 'link'));
    const mode = (path: string) => statSync(path).mode & 0o7777;
    assert.equal(makeReadable(app), 5);
    assert.equal(mode(app), 0o755);
    assert.equal(mode(join(app, 'Contents')), 0o755);
    assert.equal(mode(macos), 0o755);
    assert.equal(mode(join(app, 'Contents/Info.plist')), 0o644);
    assert.equal(mode(join(macos, 'Fort')), 0o755);
    assert.ok(lstatSync(join(macos, 'link')).isSymbolicLink());
    assert.equal(mode(join(root, 'outside')), 0o600);
    assert.equal(makeReadable(app), 0);
  } finally { rmSync(root, { recursive: true, force: true }); }
});
