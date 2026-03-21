import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync, mkdirSync } from 'node:fs';
import { LLMClient } from '../llm/index.js';
import type { LLMClientConfig, ModelTier } from '../llm/index.js';
import { ModuleBus } from '../module-bus/index.js';
import { TokenTracker } from '../tokens/index.js';
import { BehaviorManager } from '../behaviors/index.js';
import { MemoryManager } from '../memory/index.js';

describe('LLMClient', () => {
  let tmpDir: string;
  let bus: ModuleBus;
  let tokens: TokenTracker;
  let behaviors: BehaviorManager;
  let memory: MemoryManager;
  let savedOAuthToken: string | undefined;
  let savedApiKey: string | undefined;

  function setup(config: LLMClientConfig = {}) {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-llm-'));
    bus = new ModuleBus();
    tokens = new TokenTracker(join(tmpDir, 'tokens.db'), bus);
    memory = new MemoryManager(join(tmpDir, 'memory.db'), bus);
    behaviors = new BehaviorManager(memory, bus);
    return new LLMClient(config, bus, tokens, behaviors, memory);
  }

  // Isolate tests from ambient auth (e.g. running inside Claude Code)
  beforeEach(() => {
    savedOAuthToken = process.env.CLAUDE_CODE_OAUTH_TOKEN;
    savedApiKey = process.env.ANTHROPIC_API_KEY;
    delete process.env.CLAUDE_CODE_OAUTH_TOKEN;
    delete process.env.ANTHROPIC_API_KEY;
    // Suppress keychain reads so tests run in a clean auth state
    vi.spyOn(LLMClient, 'readKeychainToken').mockReturnValue(null);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    // Restore env
    if (savedOAuthToken !== undefined) process.env.CLAUDE_CODE_OAUTH_TOKEN = savedOAuthToken;
    else delete process.env.CLAUDE_CODE_OAUTH_TOKEN;
    if (savedApiKey !== undefined) process.env.ANTHROPIC_API_KEY = savedApiKey;
    else delete process.env.ANTHROPIC_API_KEY;

    if (tokens) tokens.close();
    if (memory) memory.close();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  describe('Configuration', () => {
    it('should report not configured when no API key or token', () => {
      const client = setup();
      expect(client.isConfigured).toBe(false);
      expect(client.authMethod).toBeNull();
    });

    it('should report configured when API key provided', () => {
      const client = setup({ apiKey: 'test-key-123' });
      expect(client.isConfigured).toBe(true);
      expect(client.authMethod).toBe('api_key_config');
    });

    it('should use default model tier', () => {
      const client = setup({ defaultModel: 'fast' });
      const stats = client.getStats();
      expect(stats.defaultTier).toBe('fast');
    });

    it('should allow custom model configs', () => {
      const client = setup({
        models: {
          fast: {
            tier: 'fast',
            model: 'custom-fast-model',
            maxTokens: 1024,
            description: 'Custom fast',
          },
        },
      });
      const models = client.getModels();
      expect(models.fast.model).toBe('custom-fast-model');
      // Other tiers should have defaults
      expect(models.standard.model).toBe('claude-sonnet-4-6-20250311');
    });
  });

  describe('Error handling', () => {
    it('should throw when completing without API key', async () => {
      const client = setup();
      await expect(
        client.complete({
          messages: [{ role: 'user', content: 'hello' }],
        }),
      ).rejects.toThrow('LLM client not configured');
    });

    it('should yield error when streaming without API key', async () => {
      const client = setup();
      const events = [];
      for await (const event of client.stream({
        messages: [{ role: 'user', content: 'hello' }],
      })) {
        events.push(event);
      }
      expect(events).toHaveLength(1);
      expect(events[0].type).toBe('error');
      expect(events[0].error).toContain('not configured');
    });
  });

  describe('Model routing', () => {
    it('should return all three model tiers', () => {
      const client = setup();
      const models = client.getModels();
      expect(models.fast).toBeDefined();
      expect(models.standard).toBeDefined();
      expect(models.powerful).toBeDefined();
    });

    it('should have correct model names', () => {
      const client = setup();
      const models = client.getModels();
      expect(models.fast.model).toBe('claude-haiku-4-5-20250315');
      expect(models.standard.model).toBe('claude-sonnet-4-6-20250311');
      expect(models.powerful.model).toBe('claude-opus-4-6-20250311');
    });

    it('should have increasing max tokens per tier', () => {
      const client = setup();
      const models = client.getModels();
      expect(models.fast.maxTokens).toBeLessThan(models.standard.maxTokens);
      expect(models.standard.maxTokens).toBeLessThan(models.powerful.maxTokens);
    });
  });

  describe('Stats tracking', () => {
    it('should start with zero stats', () => {
      const client = setup();
      const stats = client.getStats();
      expect(stats.requestCount).toBe(0);
      expect(stats.totalInputTokens).toBe(0);
      expect(stats.totalOutputTokens).toBe(0);
      expect(stats.totalCostUsd).toBe(0);
      expect(stats.errorCount).toBe(0);
    });

    it('should report configured status', () => {
      const unconfigured = setup();
      expect(unconfigured.getStats().configured).toBe(false);

      const configured = setup({ apiKey: 'test' });
      expect(configured.getStats().configured).toBe(true);
    });
  });

  describe('Diagnostics', () => {
    it('should return degraded when not configured', () => {
      const client = setup();
      const diag = client.diagnose();
      expect(diag.module).toBe('llm');
      expect(diag.status).toBe('degraded');
      expect(diag.checks.some((c) => c.name === 'Authentication' && !c.passed)).toBe(true);
    });

    it('should return healthy when configured with no errors', () => {
      const client = setup({ apiKey: 'test-key' });
      const diag = client.diagnose();
      expect(diag.status).toBe('healthy');
      expect(diag.checks.some((c) => c.name === 'Authentication' && c.passed)).toBe(true);
    });

    it('should include model info in diagnostics', () => {
      const client = setup();
      const diag = client.diagnose();
      expect(diag.checks.some((c) => c.name === 'Default model')).toBe(true);
    });
  });

  describe('Behavior injection', () => {
    it('should have behaviors manager available', () => {
      const client = setup();
      // Behaviors are wired in — they'll be injected into system prompts
      // when complete() is called with injectBehaviors: true
      expect(client).toBeDefined();

      // Add a behavior and verify it's stored
      behaviors.addBehavior('Always be concise', 'general', 10);

      const relevant = behaviors.getRelevantBehaviors(['general']);
      expect(relevant.length).toBeGreaterThan(0);
    });
  });

  describe('Fort integration', () => {
    it('should be accessible from Fort instance', async () => {
      const Fort = (await import('../fort.js')).Fort;
      const specsDir = join(tmpDir, 'specs');
      mkdirSync(specsDir, { recursive: true });

      const fort = new Fort({
        dataDir: join(tmpDir, 'data'),
        specsDir,
      });

      expect(fort.llm).toBeDefined();
      expect(fort.llm).toBeInstanceOf(LLMClient);
      expect(fort.llm.isConfigured).toBe(false); // No API key in test

      await fort.stop();
    });

    it('should accept LLM config via Fort config', async () => {
      const Fort = (await import('../fort.js')).Fort;
      const specsDir = join(tmpDir, 'specs');
      mkdirSync(specsDir, { recursive: true });

      const fort = new Fort({
        dataDir: join(tmpDir, 'data'),
        specsDir,
        llm: {
          apiKey: 'test-key-from-config',
          defaultModel: 'fast',
        },
      });

      expect(fort.llm.isConfigured).toBe(true);
      expect(fort.llm.getStats().defaultTier).toBe('fast');

      await fort.stop();
    });

    it('should be registered with fort doctor', async () => {
      const Fort = (await import('../fort.js')).Fort;
      const specsDir = join(tmpDir, 'specs');
      mkdirSync(specsDir, { recursive: true });

      const fort = new Fort({
        dataDir: join(tmpDir, 'data'),
        specsDir,
      });

      const results = await fort.runDoctor();
      const llmResult = results.find((r) => r.module === 'llm');
      expect(llmResult).toBeDefined();
      expect(llmResult!.module).toBe('llm');

      await fort.stop();
    });
  });
});
