import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync, mkdirSync, writeFileSync } from 'node:fs';
import { stringify as stringifyYaml } from 'yaml';
import { AgentStore } from '../agents/store.js';

describe('AgentStore', () => {
  let tmpDir: string;
  let store: AgentStore;

  function setup(agentsDir?: string) {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-agent-store-'));
    store = new AgentStore(join(tmpDir, 'agents.db'), agentsDir);
    return store;
  }

  afterEach(() => {
    store?.close();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('creates an agent and retrieves it', () => {
    setup();
    const agent = store.create({ id: 'test-agent', name: 'Test Agent', description: 'A test' });

    expect(agent.id).toBe('test-agent');
    expect(agent.name).toBe('Test Agent');
    expect(agent.description).toBe('A test');
    expect(agent.status).toBe('active');
    expect(agent.deletedAt).toBeNull();
    expect(agent.capabilities).toEqual([]);
    expect(agent.tools).toEqual([]);
    expect(agent.eventSubscriptions).toEqual([]);
    expect(agent.createdAt).toBeDefined();
    expect(agent.updatedAt).toBeDefined();
  });

  it('stores optional fields correctly', () => {
    setup();
    const agent = store.create({
      id: 'agent-with-opts',
      name: 'Agent With Options',
      description: 'Has extras',
      modelPreference: 'powerful',
      capabilities: ['write', 'search'],
      tools: ['web-search'],
      eventSubscriptions: ['task.created'],
      emoji: '🤖',
      isDefault: true,
      soul: '# Soul\nBe helpful',
    });

    expect(agent.modelPreference).toBe('powerful');
    expect(agent.capabilities).toEqual(['write', 'search']);
    expect(agent.tools).toEqual(['web-search']);
    expect(agent.eventSubscriptions).toEqual(['task.created']);
    expect(agent.emoji).toBe('🤖');
    expect(agent.isDefault).toBe(true);
    expect(agent.soul).toBe('# Soul\nBe helpful');
  });

  it('lists agents', () => {
    setup();
    store.create({ id: 'a1', name: 'Alpha' });
    store.create({ id: 'a2', name: 'Beta' });
    store.create({ id: 'a3', name: 'Gamma' });

    const list = store.list();
    expect(list).toHaveLength(3);
  });

  it('soft-deletes an agent (not in default list but in includeDeleted)', () => {
    setup();
    store.create({ id: 'del-me', name: 'To Delete' });
    store.softDelete('del-me');

    const list = store.list();
    expect(list).toHaveLength(0);

    const withDeleted = store.list({ includeDeleted: true });
    expect(withDeleted).toHaveLength(1);
    expect(withDeleted[0].deletedAt).not.toBeNull();
    expect(withDeleted[0].status).toBe('retired');
  });

  it('get returns null for missing agent', () => {
    setup();
    expect(store.get('nonexistent')).toBeNull();
  });

  it('updates agent fields', () => {
    setup();
    store.create({ id: 'upd', name: 'Original', description: 'Old desc' });

    const updated = store.update('upd', {
      name: 'Updated Name',
      description: 'New desc',
      emoji: '✨',
      modelPreference: 'fast',
    });

    expect(updated.name).toBe('Updated Name');
    expect(updated.description).toBe('New desc');
    expect(updated.emoji).toBe('✨');
    expect(updated.modelPreference).toBe('fast');
  });

  it('throws when updating non-existent agent', () => {
    setup();
    expect(() => store.update('missing', { name: 'X' })).toThrow('Agent not found');
  });

  it('update marks updated_at as newer than created_at', async () => {
    setup();
    store.create({ id: 'ts-test', name: 'Timestamp' });

    // Small delay to ensure different timestamps
    await new Promise((r) => setTimeout(r, 10));

    const updated = store.update('ts-test', { name: 'Updated' });
    expect(updated.updatedAt >= updated.createdAt).toBe(true);
  });

  it('migrates from YAML agent directory on first boot', () => {
    const agentsDir = join(mkdtempSync(join(tmpdir(), 'fort-test-agents-')));

    // Create a directory-based agent
    const agentDir = join(agentsDir, 'my-agent');
    mkdirSync(agentDir, { recursive: true });
    writeFileSync(join(agentDir, 'identity.yaml'), stringifyYaml({
      id: 'my-agent',
      name: 'My Agent',
      description: 'Migrated from YAML',
      capabilities: ['read'],
      status: 'active',
      memoryPartition: 'my-agent',
      behaviors: [],
      eventSubscriptions: [],
      createdAt: new Date().toISOString(),
      createdBy: 'user',
    }), 'utf-8');
    writeFileSync(join(agentDir, 'SOUL.md'), '# My Agent\nBe helpful.', 'utf-8');

    setup(agentsDir);

    const agents = store.list();
    expect(agents).toHaveLength(1);
    expect(agents[0].id).toBe('my-agent');
    expect(agents[0].name).toBe('My Agent');
    expect(agents[0].soul).toBe('# My Agent\nBe helpful.');

    // Clean up agents dir
    rmSync(agentsDir, { recursive: true, force: true });
  });

  it('skips migration if DB already has agents', () => {
    const agentsDir = join(mkdtempSync(join(tmpdir(), 'fort-test-agents-')));

    // Pre-populate the DB
    setup();
    store.create({ id: 'pre-existing', name: 'Pre-existing' });
    store.close();

    // Create a YAML agent that would normally be migrated
    const agentDir = join(agentsDir, 'yaml-agent');
    mkdirSync(agentDir, { recursive: true });
    writeFileSync(join(agentDir, 'identity.yaml'), stringifyYaml({
      id: 'yaml-agent',
      name: 'YAML Agent',
      status: 'active',
      memoryPartition: 'yaml-agent',
      behaviors: [],
      eventSubscriptions: [],
      capabilities: [],
      createdAt: new Date().toISOString(),
      createdBy: 'user',
    }), 'utf-8');

    // Re-open same DB with agents dir — should not migrate since DB has entries
    store = new AgentStore(join(tmpDir, 'agents.db'), agentsDir);
    const agents = store.list();
    expect(agents).toHaveLength(1);
    expect(agents[0].id).toBe('pre-existing');

    rmSync(agentsDir, { recursive: true, force: true });
  });

  it('returns correct diagnostics', () => {
    setup();
    store.create({ id: 'diag1', name: 'Agent 1' });
    store.create({ id: 'diag2', name: 'Agent 2' });
    store.softDelete('diag2');

    const diag = store.diagnose();
    expect(diag.module).toBe('agent-store');
    expect(diag.status).toBe('healthy');
    expect(diag.checks[0].passed).toBe(true);
    expect(diag.checks[0].message).toContain('1 agents total');
    expect(diag.checks[0].message).toContain('1 active');
  });

  it('update status to retired', () => {
    setup();
    store.create({ id: 'retire-me', name: 'Retiring' });
    const updated = store.update('retire-me', { status: 'retired' });
    expect(updated.status).toBe('retired');
  });
});
