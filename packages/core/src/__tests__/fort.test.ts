import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync, mkdirSync } from 'node:fs';
import { Fort } from '../fort.js';

describe('Fort Integration', () => {
  let tmpDir: string;
  let fort: Fort;

  function setup() {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-'));
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

  it('should start with all core agents running', async () => {
    setup();
    await fort.start();

    const agents = fort.agents.listInfo();
    expect(agents).toHaveLength(4);
    expect(agents.every((a) => a.status === 'running')).toBe(true);

    const names = agents.map((a) => a.config.name);
    expect(names).toContain('Orchestrator');
    expect(names).toContain('Memory Agent');
    expect(names).toContain('Scheduler Agent');
    expect(names).toContain('Reflection Agent');
  });

  it('should create a task for every user input', async () => {
    setup();
    await fort.start();

    const taskId = await fort.chat('What time is it?');
    expect(taskId).toBeTruthy();

    const task = fort.taskGraph.getTask(taskId);
    expect(task.source).toBe('user_chat');
    expect(task.title).toContain('What time is it?');
  });

  it('should run doctor and report healthy', async () => {
    setup();
    await fort.start();

    const results = await fort.runDoctor();
    expect(results.length).toBeGreaterThan(0);

    // All modules should be at least healthy or degraded (not unhealthy)
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

  it('should create and retrieve specs', async () => {
    setup();
    await fort.start();

    const spec = fort.specs.create({
      title: 'Add Gmail Integration',
      goal: 'Integrate Gmail for read and draft-only write',
      approach: 'Use Gmail API with OAuth2',
      affectedFiles: ['packages/core/src/integrations/gmail.ts'],
      testCriteria: ['Can read emails', 'Can create drafts', 'Cannot send directly'],
      rollbackPlan: 'Remove gmail module, revert config',
    });

    expect(spec.id).toBeTruthy();
    expect(spec.status).toBe('draft');

    const retrieved = fort.specs.get(spec.id);
    expect(retrieved).not.toBeNull();
    expect(retrieved!.title).toBe('Add Gmail Integration');
  });

  it('should track multiple tasks and query by status', async () => {
    setup();
    await fort.start();

    await fort.chat('Task one');
    await fort.chat('Task two');
    await fort.chat('Task three');

    const allTasks = fort.taskGraph.getAllTasks();
    expect(allTasks.length).toBeGreaterThanOrEqual(3);
  });

  it('should report task and memory stats in doctor', async () => {
    setup();
    await fort.start();

    await fort.chat('Hello Fort');
    fort.memory.createNode({ type: 'fact', label: 'Test fact' });

    const results = await fort.runDoctor();
    const taskDiag = results.find((r) => r.module === 'tasks');
    const memDiag = results.find((r) => r.module === 'memory');

    expect(taskDiag).toBeDefined();
    expect(memDiag).toBeDefined();
  });
});
