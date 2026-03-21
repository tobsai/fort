import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync, mkdirSync, writeFileSync } from 'node:fs';
import { PluginManager } from '../plugins/index.js';
import { ModuleBus } from '../module-bus/index.js';
import { PermissionManager } from '../permissions/index.js';

function createTmpDir(): string {
  return mkdtempSync(join(tmpdir(), 'fort-plugin-test-'));
}

function writeManifest(dir: string, manifest: Record<string, unknown>): void {
  writeFileSync(join(dir, 'manifest.json'), JSON.stringify(manifest, null, 2));
}

function writeSource(dir: string, filename: string, content: string): void {
  writeFileSync(join(dir, filename), content);
}

function createCleanPlugin(baseDir: string, name = 'test-plugin'): string {
  const pluginDir = join(baseDir, name);
  mkdirSync(pluginDir, { recursive: true });
  writeManifest(pluginDir, {
    name,
    version: '1.0.0',
    description: 'A test plugin',
    author: 'Test Author',
    capabilities: [{ name: 'test-cap', description: 'Test capability' }],
    subscriptions: ['task.created'],
    emissions: ['plugin.result'],
    permissions: [],
    entryPoint: 'index.js',
  });
  writeSource(pluginDir, 'index.js', `
    module.exports = function create() {
      return {
        name: '${name}',
        version: '1.0.0',
        description: 'A test plugin',
        capabilities: [{ name: 'test-cap', description: 'Test capability' }],
        initialize: async (ctx) => { ctx.log('initialized'); },
        shutdown: async () => {},
        diagnose: () => ({ module: '${name}', status: 'healthy', checks: [] }),
      };
    };
  `);
  return pluginDir;
}

describe('PluginManager', () => {
  let tmpDir: string;
  let pluginsDir: string;
  let bus: ModuleBus;
  let permissions: PermissionManager;
  let manager: PluginManager;

  beforeEach(() => {
    tmpDir = createTmpDir();
    pluginsDir = join(tmpDir, 'plugins');
    mkdirSync(pluginsDir, { recursive: true });
    bus = new ModuleBus();
    permissions = new PermissionManager(join(tmpDir, 'permissions.yaml'));
    manager = new PluginManager(pluginsDir, bus, permissions);
  });

  afterEach(() => {
    rmSync(tmpDir, { recursive: true, force: true });
  });

  // ── Manifest Scanning ──────────────────────────────────────────────

  it('should scan a valid manifest', () => {
    const pluginDir = createCleanPlugin(pluginsDir);
    const manifest = manager.scanPlugin(pluginDir);

    expect(manifest.name).toBe('test-plugin');
    expect(manifest.version).toBe('1.0.0');
    expect(manifest.description).toBe('A test plugin');
    expect(manifest.author).toBe('Test Author');
    expect(manifest.capabilities).toHaveLength(1);
    expect(manifest.subscriptions).toEqual(['task.created']);
    expect(manifest.emissions).toEqual(['plugin.result']);
    expect(manifest.entryPoint).toBe('index.js');
  });

  it('should throw on missing manifest', () => {
    const emptyDir = join(pluginsDir, 'empty');
    mkdirSync(emptyDir);
    expect(() => manager.scanPlugin(emptyDir)).toThrow('No manifest.json found');
  });

  it('should throw on invalid JSON in manifest', () => {
    const pluginDir = join(pluginsDir, 'bad-json');
    mkdirSync(pluginDir);
    writeFileSync(join(pluginDir, 'manifest.json'), '{invalid json');
    expect(() => manager.scanPlugin(pluginDir)).toThrow('Invalid JSON');
  });

  it('should throw on manifest missing required fields', () => {
    const pluginDir = join(pluginsDir, 'no-name');
    mkdirSync(pluginDir);
    writeManifest(pluginDir, { version: '1.0.0', description: 'test', entryPoint: 'index.js' });
    expect(() => manager.scanPlugin(pluginDir)).toThrow('name');
  });

  // ── Security Scanning ──────────────────────────────────────────────

  it('should catch eval() in security scan', () => {
    const pluginDir = join(pluginsDir, 'evil-eval');
    mkdirSync(pluginDir);
    writeManifest(pluginDir, {
      name: 'evil-eval',
      version: '1.0.0',
      description: 'Evil plugin',
      capabilities: [],
      subscriptions: [],
      emissions: [],
      permissions: [],
      entryPoint: 'index.js',
    });
    writeSource(pluginDir, 'index.js', 'const result = eval("1+1");');

    const manifest = manager.scanPlugin(pluginDir);
    const report = manager.securityScan(manifest, pluginDir);

    expect(report.passed).toBe(false);
    const evalCheck = report.checks.find((c) => c.name === 'eval-usage');
    expect(evalCheck).toBeDefined();
    expect(evalCheck!.passed).toBe(false);
    expect(evalCheck!.severity).toBe('critical');
  });

  it('should catch child_process in security scan', () => {
    const pluginDir = join(pluginsDir, 'evil-cp');
    mkdirSync(pluginDir);
    writeManifest(pluginDir, {
      name: 'evil-cp',
      version: '1.0.0',
      description: 'Evil plugin',
      capabilities: [],
      subscriptions: [],
      emissions: [],
      permissions: [],
      entryPoint: 'index.js',
    });
    writeSource(pluginDir, 'index.js', "const cp = require('child_process');");

    const manifest = manager.scanPlugin(pluginDir);
    const report = manager.securityScan(manifest, pluginDir);

    expect(report.passed).toBe(false);
    const cpCheck = report.checks.find((c) => c.name === 'child-process');
    expect(cpCheck).toBeDefined();
    expect(cpCheck!.passed).toBe(false);
  });

  it('should catch new Function() in security scan', () => {
    const pluginDir = join(pluginsDir, 'evil-func');
    mkdirSync(pluginDir);
    writeManifest(pluginDir, {
      name: 'evil-func',
      version: '1.0.0',
      description: 'Evil plugin',
      capabilities: [],
      subscriptions: [],
      emissions: [],
      permissions: [],
      entryPoint: 'index.js',
    });
    writeSource(pluginDir, 'index.js', 'const fn = new Function("return 42");');

    const manifest = manager.scanPlugin(pluginDir);
    const report = manager.securityScan(manifest, pluginDir);

    expect(report.passed).toBe(false);
    const funcCheck = report.checks.find((c) => c.name === 'function-constructor');
    expect(funcCheck).toBeDefined();
    expect(funcCheck!.passed).toBe(false);
    expect(funcCheck!.severity).toBe('critical');
  });

  it('should catch execSync and spawn in security scan', () => {
    const pluginDir = join(pluginsDir, 'evil-exec');
    mkdirSync(pluginDir);
    writeManifest(pluginDir, {
      name: 'evil-exec',
      version: '1.0.0',
      description: 'Evil plugin',
      capabilities: [],
      subscriptions: [],
      emissions: [],
      permissions: [],
      entryPoint: 'index.js',
    });
    writeSource(pluginDir, 'index.js', `
      const { execSync, spawn } = require('child_process');
      execSync('rm -rf /');
      spawn('ls');
    `);

    const manifest = manager.scanPlugin(pluginDir);
    const report = manager.securityScan(manifest, pluginDir);

    expect(report.passed).toBe(false);
    expect(report.checks.find((c) => c.name === 'exec-sync')?.passed).toBe(false);
    expect(report.checks.find((c) => c.name === 'spawn-usage')?.passed).toBe(false);
  });

  it('should catch destructive fs operations in security scan', () => {
    const pluginDir = join(pluginsDir, 'evil-fs');
    mkdirSync(pluginDir);
    writeManifest(pluginDir, {
      name: 'evil-fs',
      version: '1.0.0',
      description: 'Evil plugin',
      capabilities: [],
      subscriptions: [],
      emissions: [],
      permissions: [],
      entryPoint: 'index.js',
    });
    writeSource(pluginDir, 'index.js', `
      fs.rmSync('/tmp/data', { recursive: true });
      fs.unlinkSync('/tmp/file');
      fs.rmdirSync('/tmp/dir');
    `);

    const manifest = manager.scanPlugin(pluginDir);
    const report = manager.securityScan(manifest, pluginDir);

    expect(report.passed).toBe(false);
    expect(report.checks.find((c) => c.name === 'fs-rm-sync')?.passed).toBe(false);
    expect(report.checks.find((c) => c.name === 'fs-unlink-sync')?.passed).toBe(false);
    expect(report.checks.find((c) => c.name === 'fs-rmdir-sync')?.passed).toBe(false);
  });

  it('should catch process.env access in security scan', () => {
    const pluginDir = join(pluginsDir, 'env-snoop');
    mkdirSync(pluginDir);
    writeManifest(pluginDir, {
      name: 'env-snoop',
      version: '1.0.0',
      description: 'Env snooper',
      capabilities: [],
      subscriptions: [],
      emissions: [],
      permissions: [],
      entryPoint: 'index.js',
    });
    writeSource(pluginDir, 'index.js', 'const key = process.env.SECRET_KEY;');

    const manifest = manager.scanPlugin(pluginDir);
    const report = manager.securityScan(manifest, pluginDir);

    const envCheck = report.checks.find((c) => c.name === 'process-env');
    expect(envCheck).toBeDefined();
    expect(envCheck!.passed).toBe(false);
    expect(envCheck!.severity).toBe('warning');
  });

  it('should catch undeclared network access', () => {
    const pluginDir = join(pluginsDir, 'sneaky-net');
    mkdirSync(pluginDir);
    writeManifest(pluginDir, {
      name: 'sneaky-net',
      version: '1.0.0',
      description: 'Sneaky network plugin',
      capabilities: [],
      subscriptions: [],
      emissions: [],
      permissions: [],
      entryPoint: 'index.js',
    });
    writeSource(pluginDir, 'index.js', "fetch('https://evil.com/data');");

    const manifest = manager.scanPlugin(pluginDir);
    const report = manager.securityScan(manifest, pluginDir);

    const netCheck = report.checks.find((c) => c.name === 'undeclared-network');
    expect(netCheck).toBeDefined();
    expect(netCheck!.passed).toBe(false);
  });

  it('should pass security scan for clean code', () => {
    const pluginDir = createCleanPlugin(pluginsDir, 'clean-plugin');

    const manifest = manager.scanPlugin(pluginDir);
    const report = manager.securityScan(manifest, pluginDir);

    expect(report.passed).toBe(true);
    expect(report.pluginName).toBe('clean-plugin');
    const failedChecks = report.checks.filter((c) => !c.passed);
    expect(failedChecks).toHaveLength(0);
  });

  it('should allow declared network permissions', () => {
    const pluginDir = join(pluginsDir, 'legit-net');
    mkdirSync(pluginDir);
    writeManifest(pluginDir, {
      name: 'legit-net',
      version: '1.0.0',
      description: 'Legitimate network plugin',
      capabilities: [],
      subscriptions: [],
      emissions: [],
      permissions: [
        { type: 'network', scope: 'https://api.example.com/*', reason: 'API calls' },
      ],
      entryPoint: 'index.js',
    });
    writeSource(pluginDir, 'index.js', "const data = await fetch('https://api.example.com/data');");

    const manifest = manager.scanPlugin(pluginDir);
    const report = manager.securityScan(manifest, pluginDir);

    // fetch is declared so it should pass
    expect(report.passed).toBe(true);
    const fetchCheck = report.checks.find((c) => c.name === 'fetch-usage');
    expect(fetchCheck).toBeDefined();
    expect(fetchCheck!.passed).toBe(true);
  });

  it('should flag wildcard permissions', () => {
    const pluginDir = join(pluginsDir, 'greedy');
    mkdirSync(pluginDir);
    writeManifest(pluginDir, {
      name: 'greedy',
      version: '1.0.0',
      description: 'Greedy plugin',
      capabilities: [],
      subscriptions: [],
      emissions: [],
      permissions: [
        { type: 'filesystem', scope: '*', reason: 'I want everything' },
      ],
      entryPoint: 'index.js',
    });
    writeSource(pluginDir, 'index.js', 'module.exports = {};');

    const manifest = manager.scanPlugin(pluginDir);
    const report = manager.securityScan(manifest, pluginDir);

    const permCheck = report.checks.find((c) => c.name === 'excessive-permissions');
    expect(permCheck).toBeDefined();
    expect(permCheck!.passed).toBe(false);
    expect(permCheck!.severity).toBe('warning');
  });

  // ── Load / Unload Lifecycle ────────────────────────────────────────

  it('should load a clean plugin', async () => {
    const pluginDir = createCleanPlugin(pluginsDir, 'loadable');
    const loaded = await manager.loadPlugin(pluginDir);

    expect(loaded.status).toBe('active');
    expect(loaded.manifest.name).toBe('loadable');
    expect(loaded.securityReport?.passed).toBe(true);
  });

  it('should block a plugin that fails security scan', async () => {
    const pluginDir = join(pluginsDir, 'blocked');
    mkdirSync(pluginDir);
    writeManifest(pluginDir, {
      name: 'blocked',
      version: '1.0.0',
      description: 'Blocked plugin',
      capabilities: [],
      subscriptions: [],
      emissions: [],
      permissions: [],
      entryPoint: 'index.js',
    });
    writeSource(pluginDir, 'index.js', 'eval("danger");');

    const loaded = await manager.loadPlugin(pluginDir);

    expect(loaded.status).toBe('blocked');
    expect(loaded.error).toBe('Security scan failed');
  });

  it('should unload a loaded plugin', async () => {
    const pluginDir = createCleanPlugin(pluginsDir, 'unloadable');
    await manager.loadPlugin(pluginDir);

    expect(manager.getPlugin('unloadable')).toBeDefined();

    const success = await manager.unloadPlugin('unloadable');
    expect(success).toBe(true);
    expect(manager.getPlugin('unloadable')).toBeUndefined();
  });

  it('should return false when unloading non-existent plugin', async () => {
    const success = await manager.unloadPlugin('nonexistent');
    expect(success).toBe(false);
  });

  it('should prevent loading a plugin twice', async () => {
    const pluginDir = createCleanPlugin(pluginsDir, 'double-load');
    await manager.loadPlugin(pluginDir);
    await expect(manager.loadPlugin(pluginDir)).rejects.toThrow('already loaded');
  });

  // ── Enable / Disable ──────────────────────────────────────────────

  it('should disable an active plugin', async () => {
    const pluginDir = createCleanPlugin(pluginsDir, 'toggleable');
    await manager.loadPlugin(pluginDir);

    expect(manager.getPlugin('toggleable')?.status).toBe('active');

    const disabled = manager.disablePlugin('toggleable');
    expect(disabled).toBe(true);
    expect(manager.getPlugin('toggleable')?.status).toBe('disabled');
  });

  it('should enable a disabled plugin', async () => {
    const pluginDir = createCleanPlugin(pluginsDir, 'reenable');
    await manager.loadPlugin(pluginDir);
    manager.disablePlugin('reenable');

    const enabled = manager.enablePlugin('reenable');
    expect(enabled).toBe(true);
    expect(manager.getPlugin('reenable')?.status).toBe('active');
  });

  it('should not enable a blocked plugin', async () => {
    const pluginDir = join(pluginsDir, 'stay-blocked');
    mkdirSync(pluginDir);
    writeManifest(pluginDir, {
      name: 'stay-blocked',
      version: '1.0.0',
      description: 'Blocked',
      capabilities: [],
      subscriptions: [],
      emissions: [],
      permissions: [],
      entryPoint: 'index.js',
    });
    writeSource(pluginDir, 'index.js', 'eval("danger");');
    await manager.loadPlugin(pluginDir);

    const enabled = manager.enablePlugin('stay-blocked');
    expect(enabled).toBe(false);
    expect(manager.getPlugin('stay-blocked')?.status).toBe('blocked');
  });

  // ── Plugin Discovery ──────────────────────────────────────────────

  it('should discover plugins in the plugins directory', () => {
    createCleanPlugin(pluginsDir, 'plugin-a');
    createCleanPlugin(pluginsDir, 'plugin-b');

    const discovered = manager.discoverPlugins();
    expect(discovered).toHaveLength(2);
    const names = discovered.map((m) => m.name).sort();
    expect(names).toEqual(['plugin-a', 'plugin-b']);
  });

  it('should return empty array for non-existent plugins directory', () => {
    const noDir = new PluginManager(
      join(tmpDir, 'nonexistent'),
      bus,
      permissions,
    );
    expect(noDir.discoverPlugins()).toEqual([]);
  });

  it('should skip invalid plugin directories during discovery', () => {
    createCleanPlugin(pluginsDir, 'valid');
    // Create a directory without manifest
    const invalidDir = join(pluginsDir, 'invalid');
    mkdirSync(invalidDir);

    const discovered = manager.discoverPlugins();
    expect(discovered).toHaveLength(1);
    expect(discovered[0].name).toBe('valid');
  });

  // ── List Plugins ──────────────────────────────────────────────────

  it('should list all loaded plugins', async () => {
    createCleanPlugin(pluginsDir, 'list-a');
    createCleanPlugin(pluginsDir, 'list-b');
    await manager.loadPlugin(join(pluginsDir, 'list-a'));
    await manager.loadPlugin(join(pluginsDir, 'list-b'));

    const list = manager.listPlugins();
    expect(list).toHaveLength(2);
  });

  // ── Diagnostics ───────────────────────────────────────────────────

  it('should produce diagnostic output', () => {
    const result = manager.diagnose();

    expect(result.module).toBe('plugins');
    expect(result.status).toBe('healthy');
    expect(result.checks.length).toBeGreaterThan(0);
    expect(result.checks.find((c) => c.name === 'Plugin count')).toBeDefined();
  });

  it('should report degraded status with errored plugins', async () => {
    const pluginDir = join(pluginsDir, 'err-diag');
    mkdirSync(pluginDir);
    writeManifest(pluginDir, {
      name: 'err-diag',
      version: '1.0.0',
      description: 'Errors on load',
      capabilities: [],
      subscriptions: [],
      emissions: [],
      permissions: [],
      entryPoint: 'index.js',
    });
    writeSource(pluginDir, 'index.js', 'eval("bad");');
    await manager.loadPlugin(pluginDir);

    const result = manager.diagnose();
    expect(result.status).toBe('degraded');
  });

  // ── Shutdown All ──────────────────────────────────────────────────

  it('should shutdown all plugins', async () => {
    createCleanPlugin(pluginsDir, 'shut-a');
    createCleanPlugin(pluginsDir, 'shut-b');
    await manager.loadPlugin(join(pluginsDir, 'shut-a'));
    await manager.loadPlugin(join(pluginsDir, 'shut-b'));

    expect(manager.listPlugins()).toHaveLength(2);

    await manager.shutdownAll();
    expect(manager.listPlugins()).toHaveLength(0);
  });

  // ── Bus Events ────────────────────────────────────────────────────

  it('should emit plugin.loaded event on successful load', async () => {
    const events: string[] = [];
    bus.subscribe('plugin.loaded', (e) => { events.push(e.payload as string); });

    const pluginDir = createCleanPlugin(pluginsDir, 'event-test');
    await manager.loadPlugin(pluginDir);

    expect(events.length).toBeGreaterThanOrEqual(1);
  });

  it('should emit plugin.unloaded event on unload', async () => {
    const events: string[] = [];
    bus.subscribe('plugin.unloaded', (e) => { events.push(e.payload as string); });

    const pluginDir = createCleanPlugin(pluginsDir, 'unload-event');
    await manager.loadPlugin(pluginDir);
    await manager.unloadPlugin('unload-event');

    expect(events.length).toBeGreaterThanOrEqual(1);
  });
});
