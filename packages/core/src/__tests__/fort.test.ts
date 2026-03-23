import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync, mkdirSync, writeFileSync } from 'node:fs';
import { stringify as stringifyYaml } from 'yaml';
import { Fort } from '../fort.js';
import { LLMClient } from '../llm/index.js';

describe('Fort Integration', () => {
  let tmpDir: string;
  let fort: Fort;

  beforeEach(() => {
    vi.spyOn(LLMClient, 'readKeychainToken').mockReturnValue(null);
    vi.spyOn(LLMClient, 'readEnvFile').mockReturnValue(null);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  function setup() {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-'));
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

  it('should start with no agents (core agents are now services)', async () => {
    setup();
    await fort.start();

    const agents = fort.agents.listInfo();
    expect(agents).toHaveLength(0);
  });

  it('should have orchestrator and reflection as services', async () => {
    setup();
    await fort.start();

    expect(fort.orchestrator).toBeDefined();
    expect(fort.reflection).toBeDefined();
  });

  it('should create a task for every user input via chat', async () => {
    setup();
    await fort.start();

    // Create a default agent for chat to route to
    const agent = fort.agentFactory.create({ name: 'Test Agent' });
    agent.identity.isDefault = true;
    // Persist isDefault to disk so orchestrator can find it
    writeFileSync(join(agent.agentDir, 'identity.yaml'), stringifyYaml(agent.identity), 'utf-8');
    await agent.start();

    const task = await fort.chat('What time is it?');
    expect(task).toBeTruthy();
    expect(task.source).toBe('user_chat');
    expect(task.title).toContain('What time is it?');
    expect(task.assignedAgent).toBe('test-agent');
  });

  it('should route chat to specified agent', async () => {
    setup();
    await fort.start();

    const agent1 = fort.agentFactory.create({ name: 'Agent One' });
    const agent2 = fort.agentFactory.create({ name: 'Agent Two' });
    await agent1.start();
    await agent2.start();

    const task = await fort.chat('Hello', 'user_chat', 'agent-two');
    expect(task.assignedAgent).toBe('agent-two');
  });

  it('should report setup not complete when no default agent', async () => {
    setup();
    await fort.start();

    expect(fort.isSetupComplete()).toBe(false);
  });

  it('should report setup complete when default agent exists', async () => {
    setup();
    await fort.start();

    const agent = fort.agentFactory.create({ name: 'Fort' });
    agent.identity.isDefault = true;
    // Persist isDefault to disk so listIdentities() can read it
    writeFileSync(join(agent.agentDir, 'identity.yaml'), stringifyYaml(agent.identity), 'utf-8');
    await agent.start();

    expect(fort.isSetupComplete()).toBe(true);
  });

  it('should run doctor and report healthy', async () => {
    setup();
    await fort.start();

    const results = await fort.runDoctor();
    expect(results.length).toBeGreaterThan(0);

    for (const result of results) {
      expect(result.status).not.toBe('unhealthy');
    }
  });

  it('should store and search memory', async () => {
    setup();
    await fort.start();

    fort.memory.createNode({
      type: 'person',
      label: 'Sarah',
      properties: { relationship: 'wife' },
      source: 'test',
    });

    const results = fort.memory.search({ text: 'Sarah' });
    expect(results.nodes).toHaveLength(1);
    expect(results.nodes[0].label).toBe('Sarah');
  });

  it('should register and search tools', async () => {
    setup();
    await fort.start();

    fort.tools.register({
      name: 'web-search',
      description: 'Search the web via Brave Search API',
      capabilities: ['web_search'],
      inputTypes: ['string'],
      outputTypes: ['search_results'],
      tags: ['search', 'web'],
      module: 'browser',
      version: '1.0.0',
    });

    const results = fort.tools.search('web search');
    expect(results).toHaveLength(1);
    expect(results[0].name).toBe('web-search');
  });

  it('should track tasks created by chat', async () => {
    setup();
    await fort.start();

    const agent = fort.agentFactory.create({ name: 'Task Agent' });
    agent.identity.isDefault = true;
    // Persist isDefault to disk so orchestrator can find it
    writeFileSync(join(agent.agentDir, 'identity.yaml'), stringifyYaml(agent.identity), 'utf-8');
    await agent.start();

    await fort.chat('Task one');
    await fort.chat('Task two');
    await fort.chat('Task three');

    const allTasks = fort.taskGraph.getAllTasks();
    expect(allTasks.length).toBeGreaterThanOrEqual(3);
  }, 30_000);

  it('should complete tasks with results', async () => {
    setup();
    await fort.start();

    const task = fort.taskGraph.createTask({
      title: 'Test task',
      source: 'user_chat',
    });

    fort.taskGraph.completeTask(task.id, 'Task completed successfully');

    const completed = fort.taskGraph.getTask(task.id);
    expect(completed.status).toBe('completed');
    expect(completed.result).toBe('Task completed successfully');
  });
});
