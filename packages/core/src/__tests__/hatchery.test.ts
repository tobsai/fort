import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync, mkdirSync, writeFileSync, existsSync, readFileSync } from 'node:fs';
import { stringify as stringifyYaml } from 'yaml';
import { Fort } from '../fort.js';

describe('AgentFactory', () => {
  let tmpDir: string;
  let fort: Fort;

  function setup() {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-agent-factory-'));
    const specsDir = join(tmpDir, 'specs');
    mkdirSync(specsDir, { recursive: true });

    fort = new Fort({
      dataDir: join(tmpDir, 'data'),
      specsDir,
      agentsDir: join(tmpDir, 'agents'),
    });
    return fort;
  }

  afterEach(async () => {
    if (fort) await fort.stop();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should create with only a name', async () => {
    setup();
    await fort.start();

    const agent = fort.agentFactory.create({ name: 'Research Agent' });

    expect(agent.config.id).toBe('research-agent');
    expect(agent.config.type).toBe('specialist');
    expect(agent.config.description).toBe('A Fort specialist agent.');
    expect(fort.agents.get('research-agent')).toBeDefined();
  });

  it('should create with name and description', async () => {
    setup();
    await fort.start();

    const agent = fort.agentFactory.create({
      name: 'Email Agent',
      description: 'Handles email drafting and monitoring',
    });

    expect(agent.config.description).toBe('Handles email drafting and monitoring');
  });

  it('should create agent directory with SOUL.md', async () => {
    setup();
    await fort.start();

    const agent = fort.agentFactory.create({ name: 'Test Agent' });

    // Check directory structure
    expect(existsSync(agent.agentDir)).toBe(true);
    expect(existsSync(join(agent.agentDir, 'identity.yaml'))).toBe(true);
    expect(existsSync(join(agent.agentDir, 'SOUL.md'))).toBe(true);
    expect(existsSync(join(agent.agentDir, 'tools'))).toBe(true);
  });

  it('should generate SOUL.md with agent name and description', async () => {
    setup();
    await fort.start();

    const agent = fort.agentFactory.create({
      name: 'Research Agent',
      description: 'Deep web research and source synthesis',
    });

    const soul = readFileSync(join(agent.agentDir, 'SOUL.md'), 'utf-8');
    expect(soul).toContain('# Research Agent');
    expect(soul).toContain('Deep web research and source synthesis');
    expect(soul).toContain('## Personality');
    expect(soul).toContain('## Rules');
    expect(soul).toContain('## Voice');
    expect(soul).toContain('## Boundaries');
    expect(soul).toContain('## Knowledge');
  });

  it('should read SOUL.md from agent via getSoul()', async () => {
    setup();
    await fort.start();

    const agent = fort.agentFactory.create({ name: 'Test Agent' });
    await agent.start();

    const soul = agent.getSoul();
    expect(soul).not.toBeNull();
    expect(soul).toContain('# Test Agent');
  });

  it('should refresh soul when file changes', async () => {
    setup();
    await fort.start();

    const agent = fort.agentFactory.create({ name: 'Evolving Agent' });
    await agent.start();

    // Verify initial soul
    expect(agent.getSoul()).toContain('# Evolving Agent');

    // Edit SOUL.md
    writeFileSync(join(agent.agentDir, 'SOUL.md'), '# Evolved Agent\n\nI am now different.', 'utf-8');

    // Should still have old cached version
    expect(agent.getSoul()).toContain('# Evolving Agent');

    // Refresh and verify new version
    agent.refreshSoul();
    expect(agent.getSoul()).toContain('# Evolved Agent');
    expect(agent.getSoul()).toContain('I am now different.');
  });

  it('should start created agent and handle tasks', async () => {
    setup();
    await fort.start();

    const agent = fort.agentFactory.create({ name: 'Test Agent' });
    await agent.start();

    expect(agent.status).toBe('running');

    const task = fort.taskGraph.createTask({
      title: 'Test task',
      source: 'user_chat',
      assignedAgent: 'test-agent',
    });
    await agent.handleTask(task.id);
    expect(fort.taskGraph.getTask(task.id).status).toBe('completed');
  });

  it('should retire an agent', async () => {
    setup();
    await fort.start();

    const agent = fort.agentFactory.create({ name: 'Temp Agent' });
    await agent.start();

    fort.agentFactory.retire('temp-agent', 'No longer needed');

    expect(fort.agents.get('temp-agent')).toBeUndefined();

    const identities = fort.agentFactory.listIdentities();
    const identity = identities.find((i) => i.id === 'temp-agent');
    expect(identity?.status).toBe('retired');

    // SOUL.md should still exist
    expect(existsSync(join(agent.agentDir, 'SOUL.md'))).toBe(true);
  });

  it('should fork an agent and copy SOUL.md', async () => {
    setup();
    await fort.start();

    const parent = fort.agentFactory.create({
      name: 'Research Agent',
      description: 'General research',
    });

    // Customize the parent's SOUL.md
    writeFileSync(
      join(parent.agentDir, 'SOUL.md'),
      '# Research Agent\n\nI am thorough and cite sources.\n\n## Rules\n- Always cite sources',
      'utf-8',
    );

    const forked = fort.agentFactory.fork('research-agent', {
      name: 'Academic Researcher',
    });

    expect(forked.config.id).toBe('academic-researcher');
    expect(forked.identity.createdBy).toBe('forked:research-agent');

    // Fork should have a copy of parent's SOUL.md
    const forkedSoul = readFileSync(join(forked.agentDir, 'SOUL.md'), 'utf-8');
    expect(forkedSoul).toContain('cite sources');
  });

  it('should revive a retired agent', async () => {
    setup();
    await fort.start();

    fort.agentFactory.create({ name: 'Revivable Agent' });
    fort.agentFactory.retire('revivable-agent');

    expect(fort.agents.get('revivable-agent')).toBeUndefined();

    const revived = fort.agentFactory.revive('revivable-agent');
    await revived.start();

    expect(fort.agents.get('revivable-agent')).toBeDefined();
    expect(revived.status).toBe('running');
    expect(revived.getSoul()).toContain('# Revivable Agent');
  });

  it('should create from a YAML file', async () => {
    setup();
    await fort.start();

    const identityFile = join(tmpDir, 'finance-agent.yaml');
    writeFileSync(identityFile, stringifyYaml({
      id: 'finance-agent',
      name: 'Finance Agent',
      description: 'Budget tracking and financial analysis',
    }));

    const agent = fort.agentFactory.createFromFile(identityFile);
    expect(agent.config.id).toBe('finance-agent');

    // Should have created directory and SOUL.md
    expect(existsSync(join(agent.agentDir, 'SOUL.md'))).toBe(true);
    expect(existsSync(join(agent.agentDir, 'identity.yaml'))).toBe(true);
  });

  it('should prevent duplicate agent IDs', async () => {
    setup();
    await fort.start();

    fort.agentFactory.create({ name: 'Unique Agent' });
    expect(() => {
      fort.agentFactory.create({ name: 'Unique Agent' });
    }).toThrow('already exists');
  });

  it('should prevent retiring nonexistent agents', async () => {
    setup();
    await fort.start();

    expect(() => {
      fort.agentFactory.retire('nonexistent-agent');
    }).toThrow('Agent not found');
  });

  it('should load persisted agents on restart', async () => {
    setup();
    await fort.start();

    fort.agentFactory.create({
      name: 'Persistent Agent',
      description: 'Survives restarts',
    });

    await fort.stop();

    const fort2 = new Fort({
      dataDir: join(tmpDir, 'data'),
      specsDir: join(tmpDir, 'specs'),
      agentsDir: join(tmpDir, 'agents'),
    });
    await fort2.start();

    const agent = fort2.agents.get('persistent-agent');
    expect(agent).toBeDefined();
    expect(agent!.config.name).toBe('Persistent Agent');

    await fort2.stop();
  });

  it('should load legacy flat-file agents', async () => {
    setup();
    await fort.start();

    // Manually create a legacy flat-file agent
    const agentsDir = join(tmpDir, 'agents');
    writeFileSync(join(agentsDir, 'legacy-agent.yaml'), stringifyYaml({
      id: 'legacy-agent',
      name: 'Legacy Agent',
      description: 'Created before directory layout',
      capabilities: [],
      memoryPartition: 'legacy-agent',
      behaviors: ['Be old school'],
      eventSubscriptions: [],
      createdAt: new Date().toISOString(),
      createdBy: 'test',
      status: 'active',
    }));

    await fort.stop();

    // Restart and verify legacy agent loads
    const fort2 = new Fort({
      dataDir: join(tmpDir, 'data'),
      specsDir: join(tmpDir, 'specs'),
      agentsDir: join(tmpDir, 'agents'),
    });
    await fort2.start();

    const agent = fort2.agents.get('legacy-agent');
    expect(agent).toBeDefined();
    expect(agent!.config.name).toBe('Legacy Agent');

    await fort2.stop();
  });

  it('should get soul via agentFactory', async () => {
    setup();
    await fort.start();

    fort.agentFactory.create({ name: 'Soul Agent' });

    const soul = fort.agentFactory.getSoul('soul-agent');
    expect(soul).not.toBeNull();
    expect(soul).toContain('# Soul Agent');
  });

  it('should return null soul for agent without SOUL.md', async () => {
    setup();
    await fort.start();

    const soul = fort.agentFactory.getSoul('nonexistent-agent');
    expect(soul).toBeNull();
  });
});
