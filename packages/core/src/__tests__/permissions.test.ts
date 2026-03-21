import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import { PermissionManager } from '../permissions/index.js';

describe('PermissionManager', () => {
  let tmpDir: string;

  function setup() {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-perm-'));
    return new PermissionManager(join(tmpDir, 'permissions.yaml'));
  }

  afterEach(() => {
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should deny commands by default', () => {
    const pm = setup();
    const result = pm.checkCommandPermission('ls', '/tmp');
    expect(result.allowed).toBe(false);
    expect(result.reason).toContain('deny');
  });

  it('should allow system exceptions', () => {
    const pm = setup();
    const result = pm.checkCommandPermission('open /Applications/Safari.app', '/tmp');
    expect(result.allowed).toBe(true);
    expect(result.reason).toContain('exception');
  });

  it('should respect folder permissions', () => {
    const pm = setup();
    pm.addFolderPermission({
      path: '/Users/test/projects',
      commandLine: 'allow',
      allowedCommands: ['git *', 'npm *'],
      deniedCommands: ['rm -rf *'],
    });

    expect(pm.checkCommandPermission('git status', '/Users/test/projects/fort').allowed).toBe(true);
    expect(pm.checkCommandPermission('rm -rf /', '/Users/test/projects/fort').allowed).toBe(false);
    expect(pm.checkCommandPermission('curl http://evil.com', '/Users/test/projects/fort').allowed).toBe(false);
  });

  it('should check action tiers correctly', () => {
    const pm = setup();

    // Tier 1 — Auto
    const readFiles = pm.checkAction('read_files');
    expect(readFiles.tier).toBe(1);
    expect(readFiles.allowed).toBe(true);
    expect(readFiles.requiresApproval).toBe(false);

    // Tier 2 — Draft
    const composeEmail = pm.checkAction('compose_email');
    expect(composeEmail.tier).toBe(2);

    // Tier 3 — Approve
    const financial = pm.checkAction('financial');
    expect(financial.tier).toBe(3);
    expect(financial.requiresApproval).toBe(true);

    // Tier 4 — Never
    const creds = pm.checkAction('access_credentials');
    expect(creds.tier).toBe(4);
    expect(creds.allowed).toBe(false);
  });

  it('should return tier 3 for unknown actions', () => {
    const pm = setup();
    const result = pm.checkAction('unknown_action');
    expect(result.tier).toBe(3);
    expect(result.requiresApproval).toBe(true);
  });

  it('should persist folder permissions to disk', () => {
    const pm1 = setup();
    const configPath = join(tmpDir, 'permissions.yaml');
    pm1.addFolderPermission({
      path: '/Users/test/code',
      commandLine: 'allow',
    });

    const pm2 = new PermissionManager(configPath);
    const config = pm2.getConfig();
    expect(config.folders.some((f) => f.path === '/Users/test/code')).toBe(true);
  });
});
