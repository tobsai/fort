import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import { FeatureFlagManager } from '../feature-flags/index.js';
import { ModuleBus } from '../module-bus/index.js';

describe('FeatureFlagManager', () => {
  let tmpDir: string;
  let manager: FeatureFlagManager;
  let bus: ModuleBus;

  function setup() {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-flags-'));
    bus = new ModuleBus();
    manager = new FeatureFlagManager(join(tmpDir, 'flags.db'), bus);
    return manager;
  }

  afterEach(() => {
    manager?.close();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should create and query flags', () => {
    setup();

    const flag = manager.createFlag('dark-mode', 'Enable dark mode UI');

    expect(flag.id).toBeTruthy();
    expect(flag.name).toBe('dark-mode');
    expect(flag.description).toBe('Enable dark mode UI');
    expect(flag.enabled).toBe(false);
    expect(flag.status).toBe('disabled');
    expect(flag.bakePeriodMs).toBe(24 * 60 * 60 * 1000);

    const retrieved = manager.getFlag(flag.id);
    expect(retrieved).not.toBeNull();
    expect(retrieved!.name).toBe('dark-mode');

    const all = manager.listFlags();
    expect(all.length).toBe(1);
    expect(all[0].id).toBe(flag.id);
  });

  it('should create flag with custom options', () => {
    setup();

    const flag = manager.createFlag('new-parser', 'New config parser', {
      bakePeriodMs: 3600000,
      specId: 'spec-123',
      metadata: { version: 2 },
    });

    expect(flag.bakePeriodMs).toBe(3600000);
    expect(flag.specId).toBe('spec-123');
    expect(flag.metadata).toEqual({ version: 2 });
  });

  it('should enable flag and start bake period', () => {
    setup();

    const flag = manager.createFlag('new-feature', 'A new feature');
    expect(manager.isEnabled(flag.id)).toBe(false);

    const enabled = manager.enableFlag(flag.id);
    expect(enabled.enabled).toBe(true);
    expect(enabled.status).toBe('baking');
    expect(enabled.enabledAt).toBeTruthy();
    expect(enabled.bakeStartedAt).toBeTruthy();
    expect(manager.isEnabled(flag.id)).toBe(true);
  });

  it('should manually rollback a flag', () => {
    setup();

    const flag = manager.createFlag('risky-change', 'A risky change');
    manager.enableFlag(flag.id);

    const rolled = manager.rollback(flag.id, 'Caused errors');
    expect(rolled.enabled).toBe(false);
    expect(rolled.status).toBe('rolled_back');
    expect(rolled.rollbackReason).toBe('Caused errors');
    expect(manager.isEnabled(flag.id)).toBe(false);
  });

  it('should manually disable a flag', () => {
    setup();

    const flag = manager.createFlag('temp-feature', 'Temporary');
    manager.enableFlag(flag.id);

    const disabled = manager.disableFlag(flag.id, 'No longer needed');
    expect(disabled.enabled).toBe(false);
    expect(disabled.status).toBe('disabled');
    expect(disabled.rollbackReason).toBe('No longer needed');
  });

  it('should record health checks', () => {
    setup();

    const flag = manager.createFlag('monitored', 'Monitored feature');
    manager.enableFlag(flag.id);

    const check1 = manager.recordCheck(flag.id, true, 'All systems nominal');
    expect(check1.healthy).toBe(true);
    expect(check1.flagId).toBe(flag.id);

    const check2 = manager.recordCheck(flag.id, false, 'Error rate increased');
    expect(check2.healthy).toBe(false);

    const checks = manager.getChecks(flag.id);
    expect(checks.length).toBe(2);
    expect(checks[0].healthy).toBe(true);
    expect(checks[1].healthy).toBe(false);
  });

  it('should auto-promote when bake period passes with healthy checks', () => {
    setup();

    // Create flag with a very short bake period (0ms = already elapsed)
    const flag = manager.createFlag('quick-bake', 'Quick bake feature', {
      bakePeriodMs: 0,
    });
    manager.enableFlag(flag.id);

    // Record a healthy check
    manager.recordCheck(flag.id, true, 'Looking good');

    const result = manager.checkBakePeriods();
    expect(result.promoted.length).toBe(1);
    expect(result.promoted[0].id).toBe(flag.id);
    expect(result.promoted[0].status).toBe('stable');
    expect(result.rolledBack.length).toBe(0);

    // Verify the flag is now stable
    const updated = manager.getFlag(flag.id);
    expect(updated!.status).toBe('stable');
  });

  it('should auto-rollback when unhealthy check during bake', () => {
    setup();

    const flag = manager.createFlag('bad-feature', 'This will fail', {
      bakePeriodMs: 24 * 60 * 60 * 1000,
    });
    manager.enableFlag(flag.id);

    // Record a healthy check then an unhealthy one
    manager.recordCheck(flag.id, true, 'Initially fine');
    manager.recordCheck(flag.id, false, 'Error rate spiked');

    const result = manager.checkBakePeriods();
    expect(result.rolledBack.length).toBe(1);
    expect(result.rolledBack[0].id).toBe(flag.id);
    expect(result.rolledBack[0].status).toBe('rolled_back');
    expect(result.promoted.length).toBe(0);

    // Verify the flag is rolled back
    const updated = manager.getFlag(flag.id);
    expect(updated!.status).toBe('rolled_back');
    expect(updated!.enabled).toBe(false);
  });

  it('should not promote when bake period has not elapsed', () => {
    setup();

    const flag = manager.createFlag('still-baking', 'Not done yet', {
      bakePeriodMs: 24 * 60 * 60 * 1000,
    });
    manager.enableFlag(flag.id);
    manager.recordCheck(flag.id, true, 'Looking good');

    const result = manager.checkBakePeriods();
    expect(result.promoted.length).toBe(0);
    expect(result.rolledBack.length).toBe(0);

    const updated = manager.getFlag(flag.id);
    expect(updated!.status).toBe('baking');
  });

  it('should filter flags by status', () => {
    setup();

    const f1 = manager.createFlag('flag-1', 'First');
    const f2 = manager.createFlag('flag-2', 'Second');
    const f3 = manager.createFlag('flag-3', 'Third');

    manager.enableFlag(f1.id);
    manager.enableFlag(f2.id);
    manager.rollback(f2.id, 'Bad');

    const baking = manager.listFlags('baking');
    expect(baking.length).toBe(1);
    expect(baking[0].id).toBe(f1.id);

    const rolledBack = manager.listFlags('rolled_back');
    expect(rolledBack.length).toBe(1);
    expect(rolledBack[0].id).toBe(f2.id);

    const disabled = manager.listFlags('disabled');
    expect(disabled.length).toBe(1);
    expect(disabled[0].id).toBe(f3.id);
  });

  it('should publish events on flag operations', async () => {
    setup();

    const events: string[] = [];
    bus.subscribe('flag.created', () => { events.push('created'); });
    bus.subscribe('flag.enabled', () => { events.push('enabled'); });
    bus.subscribe('flag.disabled', () => { events.push('disabled'); });
    bus.subscribe('flag.rolled_back', () => { events.push('rolled_back'); });
    bus.subscribe('flag.promoted', () => { events.push('promoted'); });

    const flag = manager.createFlag('evented', 'Evented flag');
    // Wait for async event publishing
    await new Promise((r) => setTimeout(r, 10));
    expect(events).toContain('created');

    manager.enableFlag(flag.id);
    await new Promise((r) => setTimeout(r, 10));
    expect(events).toContain('enabled');

    manager.disableFlag(flag.id, 'test');
    await new Promise((r) => setTimeout(r, 10));
    expect(events).toContain('disabled');

    const f2 = manager.createFlag('rollback-test', 'For rollback');
    manager.enableFlag(f2.id);
    manager.rollback(f2.id, 'reason');
    await new Promise((r) => setTimeout(r, 10));
    expect(events).toContain('rolled_back');

    const f3 = manager.createFlag('promote-test', 'For promote');
    manager.enableFlag(f3.id);
    manager.promote(f3.id);
    await new Promise((r) => setTimeout(r, 10));
    expect(events).toContain('promoted');
  });

  it('should return diagnose output', () => {
    setup();

    const f1 = manager.createFlag('healthy-flag', 'Healthy');
    manager.enableFlag(f1.id);
    manager.promote(f1.id);

    const f2 = manager.createFlag('bad-flag', 'Bad');
    manager.enableFlag(f2.id);
    manager.rollback(f2.id, 'broke stuff');

    const result = manager.diagnose();
    expect(result.module).toBe('feature-flags');
    expect(result.status).toBe('degraded'); // has rolled back flags
    expect(result.checks.length).toBeGreaterThanOrEqual(4);

    const flagCountCheck = result.checks.find((c) => c.name === 'Flag count');
    expect(flagCountCheck).toBeTruthy();
    expect(flagCountCheck!.passed).toBe(true);

    const rolledBackCheck = result.checks.find((c) => c.name === 'Rolled back flags');
    expect(rolledBackCheck).toBeTruthy();
    expect(rolledBackCheck!.passed).toBe(false);
  });

  it('should report healthy diagnose when no rolled back flags', () => {
    setup();

    manager.createFlag('simple-flag', 'Simple');

    const result = manager.diagnose();
    expect(result.status).toBe('healthy');
  });

  it('should return false for isEnabled on nonexistent flag', () => {
    setup();
    expect(manager.isEnabled('nonexistent')).toBe(false);
  });

  it('should throw on operations with nonexistent flag id', () => {
    setup();
    expect(() => manager.enableFlag('nonexistent')).toThrow('Flag not found');
    expect(() => manager.disableFlag('nonexistent')).toThrow('Flag not found');
    expect(() => manager.rollback('nonexistent', 'reason')).toThrow('Flag not found');
    expect(() => manager.promote('nonexistent')).toThrow('Flag not found');
    expect(() => manager.recordCheck('nonexistent', true, 'msg')).toThrow('Flag not found');
  });
});
