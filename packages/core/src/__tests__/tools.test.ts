import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import { ToolRegistry } from '../tools/index.js';

describe('ToolRegistry', () => {
  let tmpDir: string;
  let registry: ToolRegistry;

  function setup() {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-'));
    registry = new ToolRegistry(join(tmpDir, 'tools.db'));
    return registry;
  }

  afterEach(() => {
    registry?.close();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should register and retrieve tools', () => {
    setup();
    const tool = registry.register({
      name: 'gmail-reader',
      description: 'Read and search Gmail messages',
      capabilities: ['email_read', 'email_search'],
      inputTypes: ['string'],
      outputTypes: ['email[]'],
      tags: ['email', 'gmail', 'communication'],
      module: 'gmail',
      version: '1.0.0',
    });

    expect(tool.id).toBeTruthy();
    const retrieved = registry.get(tool.id);
    expect(retrieved).not.toBeNull();
    expect(retrieved!.name).toBe('gmail-reader');
  });

  it('should search tools by query', () => {
    setup();
    registry.register({
      name: 'gmail-reader',
      description: 'Read Gmail messages',
      capabilities: ['email_read'],
      inputTypes: [],
      outputTypes: [],
      tags: ['email'],
      module: 'gmail',
      version: '1.0.0',
    });
    registry.register({
      name: 'calendar-query',
      description: 'Query Google Calendar events',
      capabilities: ['calendar_read'],
      inputTypes: [],
      outputTypes: [],
      tags: ['calendar'],
      module: 'calendar',
      version: '1.0.0',
    });

    expect(registry.search('email')).toHaveLength(1);
    expect(registry.search('calendar')).toHaveLength(1);
    expect(registry.search('nonexistent')).toHaveLength(0);
  });

  it('should find tools by capability', () => {
    setup();
    registry.register({
      name: 'budget-tracker',
      description: 'Track budget and expenses',
      capabilities: ['budget_tracking', 'expense_reporting'],
      inputTypes: [],
      outputTypes: [],
      tags: ['finance'],
      module: 'finance',
      version: '1.0.0',
    });

    const results = registry.findByCapability('budget');
    expect(results).toHaveLength(1);
    expect(results[0].name).toBe('budget-tracker');
  });

  it('should track usage', () => {
    setup();
    const tool = registry.register({
      name: 'test-tool',
      description: 'Test',
      capabilities: [],
      inputTypes: [],
      outputTypes: [],
      tags: [],
      module: 'test',
      version: '1.0.0',
    });

    registry.recordUsage(tool.id);
    registry.recordUsage(tool.id);

    expect(registry.get(tool.id)!.usageCount).toBe(2);
  });

  it('should unregister tools', () => {
    setup();
    const tool = registry.register({
      name: 'temp',
      description: 'Temporary',
      capabilities: [],
      inputTypes: [],
      outputTypes: [],
      tags: [],
      module: 'test',
      version: '1.0.0',
    });

    expect(registry.unregister(tool.id)).toBe(true);
    expect(registry.get(tool.id)).toBeNull();
  });

  it('should report stats', () => {
    setup();
    registry.register({
      name: 'a',
      description: 'A',
      capabilities: [],
      inputTypes: [],
      outputTypes: [],
      tags: [],
      module: 'mod-a',
      version: '1.0.0',
    });
    registry.register({
      name: 'b',
      description: 'B',
      capabilities: [],
      inputTypes: [],
      outputTypes: [],
      tags: [],
      module: 'mod-b',
      version: '1.0.0',
    });

    const stats = registry.stats();
    expect(stats.totalTools).toBe(2);
    expect(stats.byModule['mod-a']).toBe(1);
    expect(stats.byModule['mod-b']).toBe(1);
  });
});
