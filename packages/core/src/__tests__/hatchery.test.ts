import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync, mkdirSync, writeFileSync } from 'node:fs';
import { stringify as stringifyYaml } from 'yaml';
import { Fort } from '../fort.js';

describe('AgentHatchery', () => {
  let tmpDir: string;
  let fort: Fort;

  function setup() {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-hatch-'));
    const specsDir = join(tmpDir, 'specs');
    mkdirSync(specsDir, { recursive: true });

    fort = new Fort({
      dataDir: join(tmpDir, 'data'),
      specsDir,
    });
    return fort;
  }

  afterEach(async () => {
    if (fort) await fort.stop();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should hatch a new specialist agent', async () => {
    setup();
    await fort.start();

    const agent = fort.hatchery.hatch({
      name: 'Research Agent',
      description: 'Deep web research and source synthesis',
      capabilities: ['web_search', 'summarization'],
      behaviors: ['Always cite sources', 'Prefer academic sources'],
    });

    expect(agent.config.id).toBe('research-agent');
    expect(agent.config.type).toBe('specialist');
    expect(agent.identity.behaviors).toContain('Always cite sources');

    // Should be in the registry
    expect(fort.agents.get('research-agent')).toBeDefined();
  });

  it('should persist identity to disk and reload', async () => {
    setup();
    await fort.start();

    fort.hatchery.hatch({
      name: 'Email Agent',
      description: 'Handles email drafting and monitoring',
      capabilities: ['email_read', 'email_draft'],
      behaviors: ['Always address Sarah as ma\'am'],
    });

    // Check it's in the identities list
    const identities = fort.hatchery.listIdentities();
    expect(identities.some((i) => i.id === 'email-agent')).toBe(true);
  });

  it('should start hatched agent and handle tasks', async () => {
    setup();
    await fort.start();

    const agent = fort.hatchery.hatch({
      name: 'Test Agent',
      description: 'Test specialist',
    });
    await agent.start();

    expect(agent.status).toBe('running');

    // Create and handle a task
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

    const agent = fort.hatchery.hatch({
      name: 'Temp Agent',
      description: 'Temporary agent',
    });
    await agent.start();

    fort.hatchery.retire('temp-agent', 'No longer needed');

    // Should be removed from registry
    expect(fort.agents.get('temp-agent')).toBeUndefined();

    // Identity should show as retired
    const identities = fort.hatchery.listIdentities();
    const identity = identities.find((i) => i.id === 'temp-agent');
    expect(identity?.status).toBe('retired');
  });

  it('should fork an agent', async () => {
    setup();
    await fort.start();

    fort.hatchery.hatch({
      name: 'Research Agent',
      description: 'General research',
      capabilities: ['web_search'],
      behaviors: ['Cite sources'],
    });

    const forked = fort.hatchery.fork('research-agent', {
      name: 'Academic Researcher',
      description: 'Academic-focused research',
    });

    expect(forked.config.id).toBe('academic-researcher');
    expect(forked.identity.capabilities).toContain('web_search');
    expect(forked.identity.behaviors).toContain('Cite sources');
    expect(forked.identity.createdBy).toBe('forked:research-agent');
  });

  it('should revive a retired agent', async () => {
    setup();
    await fort.start();

    fort.hatchery.hatch({
      name: 'Revivable Agent',
      description: 'Can be revived',
    });
    fort.hatchery.retire('revivable-agent');

    expect(fort.agents.get('revivable-agent')).toBeUndefined();

    const revived = fort.hatchery.revive('revivable-agent');
    await revived.start();

    expect(fort.agents.get('revivable-agent')).toBeDefined();
    expect(revived.status).toBe('running');
  });

  it('should hatch from a YAML file', async () => {
    setup();
    await fort.start();

    const identityFile = join(tmpDir, 'finance-agent.yaml');
    writeFileSync(identityFile, stringifyYaml({
      id: 'finance-agent',
      name: 'Finance Agent',
      description: 'Budget tracking and financial analysis',
      capabilities: ['budget_tracking', 'expense_reporting'],
      behaviors: ['Never make purchases without approval', 'Use conservative estimates'],
      eventSubscriptions: ['finance.request'],
    }));

    const agent = fort.hatchery.hatchFromFile(identityFile);
    expect(agent.config.id).toBe('finance-agent');
    expect(agent.identity.behaviors).toHaveLength(2);
  });

  it('should prevent duplicate agent IDs', async () => {
    setup();
    await fort.start();

    fort.hatchery.hatch({ name: 'Unique Agent', description: 'First' });
    expect(() => {
      fort.hatchery.hatch({ name: 'Unique Agent', description: 'Second' });
    }).toThrow('already exists');
  });

  it('should prevent retiring core agents', async () => {
    setup();
    await fort.start();

    expect(() => {
      fort.hatchery.retire('orchestrator');
    }).toThrow('Cannot retire core agent');
  });

  it('should load persisted agents on restart', async () => {
    setup();
    await fort.start();

    fort.hatchery.hatch({
      name: 'Persistent Agent',
      description: 'Survives restarts',
      capabilities: ['persistence'],
    });

    await fort.stop();

    // Create a fresh Fort instance pointing at same data
    const fort2 = new Fort({
      dataDir: join(tmpDir, 'data'),
      specsDir: join(tmpDir, 'specs'),
    });
    await fort2.start();

    // Should have loaded the persisted specialist
    const agent = fort2.agents.get('persistent-agent');
    expect(agent).toBeDefined();
    expect(agent!.config.name).toBe('Persistent Agent');

    await fort2.stop();
  });
});
