import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync, mkdirSync, writeFileSync } from 'node:fs';
import { stringify as stringifyYaml } from 'yaml';
import { Fort } from '../fort.js';
import { LLMClient } from '../llm/index.js';

describe('OSIntegrationManager', () => {
  let tmpDir: string;
  let fort: Fort;

  beforeEach(() => {
    vi.spyOn(LLMClient, 'readKeychainToken').mockReturnValue(null);
    vi.spyOn(LLMClient, 'readEnvFile').mockReturnValue(null);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  function setup(): Fort {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-os-integration-'));
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

  it('should search tasks via Spotlight query and return results', async () => {
    setup();
    await fort.start();

    // Create a task that matches
    fort.taskGraph.createTask({
      title: 'Deploy the dashboard',
      description: 'Ship the Tauri app',
      source: 'user_chat',
    });

    const results = fort.osIntegration.handleSpotlightQuery('dashboard');

    expect(results.length).toBeGreaterThanOrEqual(1);
    const taskResult = results.find((r) => r.type === 'task');
    expect(taskResult).toBeDefined();
    expect(taskResult!.title).toBe('Deploy the dashboard');
    expect(taskResult!.subtitle).toContain('Task');
    expect(taskResult!.relevance).toBeGreaterThan(0);
  });

  it('should route shortcut action to the correct module', async () => {
    setup();
    await fort.start();

    // create_task should create a task via taskGraph
    const result = await fort.osIntegration.handleShortcutAction('create_task', {
      title: 'Test from Shortcuts',
      description: 'Created via Siri Shortcut',
    });

    expect(result.success).toBe(true);
    expect(result.intent).toBe('create_task');
    expect(result.target).toBe('taskGraph');
    expect(result.data).toHaveProperty('taskId');
    expect(result.data).toHaveProperty('title', 'Test from Shortcuts');
  });

  it('should create a task for file action', async () => {
    setup();
    await fort.start();

    const filePaths = ['/Users/test/document.pdf', '/Users/test/image.png'];
    const result = fort.osIntegration.handleFileAction(filePaths);

    expect(result.taskId).toBeTruthy();
    expect(result.fileCount).toBe(2);

    // Verify task was created in the graph
    const task = fort.taskGraph.getTask(result.taskId);
    expect(task.title).toContain('2 file(s)');
    expect(task.description).toContain('document.pdf');
    expect(task.metadata.filePaths).toEqual(filePaths);
  });

  it('should create a task for voice input', async () => {
    setup();
    await fort.start();

    // Voice input routes through fort.chat(), which requires a default agent
    const agent = fort.agentFactory.create({ name: 'Voice Agent' });
    agent.identity.isDefault = true;
    writeFileSync(join(agent.agentDir, 'identity.yaml'), stringifyYaml(agent.identity), 'utf-8');
    await agent.start();

    const result = await fort.osIntegration.handleVoiceInput('What is the status of my tasks?');

    expect(result.taskId).toBeTruthy();
    expect(typeof result.response).toBe('string');

    // handleVoiceInput creates a voice tracking task and a chat task
    // It returns the chat task's id
    const task = fort.taskGraph.getTask(result.taskId);
    expect(task).toBeDefined();
  });

  it('should suppress non-critical notifications during DND', () => {
    setup();

    const policy = fort.osIntegration.getNotificationPolicy('task', 'dnd');

    expect(policy.shouldSend).toBe(false);
    expect(policy.reason).toContain('Suppressed');
    expect(policy.category).toBe('task');
    expect(policy.focusMode).toBe('dnd');
  });

  it('should allow critical notifications during DND', () => {
    setup();

    const policy = fort.osIntegration.getNotificationPolicy('critical', 'dnd');

    expect(policy.shouldSend).toBe(true);
    expect(policy.reason).toContain('Critical');
    expect(policy.category).toBe('critical');
    expect(policy.focusMode).toBe('dnd');
  });

  it('should return error for unknown shortcut action', async () => {
    setup();
    await fort.start();

    const result = await fort.osIntegration.handleShortcutAction('nonexistent_intent', {});

    expect(result.success).toBe(false);
    expect(result.error).toContain('Unknown shortcut intent');
    expect(result.error).toContain('nonexistent_intent');
  });

  it('should return healthy diagnostics', async () => {
    setup();
    await fort.start();

    // Trigger some operations to populate counters
    fort.osIntegration.handleSpotlightQuery('test');
    fort.osIntegration.handleFileAction(['/tmp/test.txt']);

    const diag = fort.osIntegration.diagnose();

    expect(diag.module).toBe('os-integration');
    expect(diag.status).toBe('healthy');
    expect(diag.checks.length).toBe(5);
    expect(diag.checks.every((c) => c.passed)).toBe(true);

    const spotlightCheck = diag.checks.find((c) => c.name === 'Spotlight handler');
    expect(spotlightCheck!.message).toContain('1');

    const fileCheck = diag.checks.find((c) => c.name === 'File action handler');
    expect(fileCheck!.message).toContain('1');
  });
});
