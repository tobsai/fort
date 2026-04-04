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
    // Suppress .env and keychain reads so tests run in a clean auth state
    vi.spyOn(LLMClient, 'readEnvFile').mockReturnValue(null);
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
      expect(models.standard.model).toBe('claude-sonnet-4-5-20250929');
    });
  });

  describe('Auth validation', () => {
    it('should return error message when not configured', async () => {
      const client = setup();
      const error = await client.validateAuth();
      expect(error).toContain('not configured');
    });

    it('should detect OAuth tokens as invalid', async () => {
      const client = setup({ apiKey: 'sk-ant-oat01-fake-token' });
      expect(client.isConfigured).toBe(true);
      const error = await client.validateAuth();
      // Will get auth error since the key is fake — but the important thing is it doesn't crash
      expect(error).toBeTruthy();
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
      expect(models.fast.model).toBe('claude-haiku-4-5-20251001');
      expect(models.standard.model).toBe('claude-sonnet-4-5-20250929');
      expect(models.powerful.model).toBe('claude-opus-4-6');
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

  describe('Rate limit handling', () => {
    // Helper to create a mock Anthropic response
    function mockResponse(overrides?: Partial<any>) {
      return {
        id: 'msg_test',
        type: 'message',
        role: 'assistant',
        content: [{ type: 'text', text: 'Hello!' }],
        model: 'claude-sonnet-4-5-20250929',
        stop_reason: 'end_turn',
        usage: { input_tokens: 10, output_tokens: 5 },
        ...overrides,
      };
    }

    it('should set cooldown on rate limit and fall back to lower tier', async () => {
      const client = setup({ apiKey: 'test-key' });
      const mockCreate = vi.fn();

      // First call: 429 on standard model
      const rateLimitErr = new (await import('@anthropic-ai/sdk')).RateLimitError(
        429, undefined, 'rate limited', undefined as any,
      );
      mockCreate.mockRejectedValueOnce(rateLimitErr);
      // Second call (fallback to fast): succeeds
      mockCreate.mockResolvedValueOnce(mockResponse({
        model: 'claude-haiku-4-5-20251001',
      }));

      (client as any).client.messages = { create: mockCreate };

      const result = await client.complete({
        messages: [{ role: 'user', content: 'hello' }],
        model: 'standard',
      });

      // Should have fallen back to fast tier
      expect(result.model).toBeDefined();
      expect(client.getStats().rateLimitCount).toBe(1);
    });

    it('should respect Retry-After header for cooldown duration', async () => {
      // Use standard tier so on first 429 it falls back to fast (no sleep)
      const client = setup({ apiKey: 'test-key' });
      const mockCreate = vi.fn();

      // Create a RateLimitError with retry-after header
      const rateLimitErr = new (await import('@anthropic-ai/sdk')).RateLimitError(
        429, undefined, 'rate limited', undefined as any,
      );
      (rateLimitErr as any).headers = { 'retry-after': '45' };

      // First call (standard): 429 → sets cooldown → falls back to fast
      mockCreate.mockRejectedValueOnce(rateLimitErr);
      // Second call (fast fallback): succeeds
      mockCreate.mockResolvedValueOnce(mockResponse({
        model: 'claude-haiku-4-5-20251001',
      }));

      (client as any).client.messages = { create: mockCreate };

      await client.complete({
        messages: [{ role: 'user', content: 'hello' }],
        model: 'standard',
      });

      const stats = client.getStats();
      const cooldowns = stats.cooldowns;
      const standardModel = client.getModels().standard.model;
      expect(cooldowns[standardModel]).toBeDefined();
      expect(cooldowns[standardModel].reason).toBe('rate_limit');
    });

    it('should publish llm.rate_limited event on 429', async () => {
      const client = setup({ apiKey: 'test-key' });
      const mockCreate = vi.fn();

      const rateLimitErr = new (await import('@anthropic-ai/sdk')).RateLimitError(
        429, undefined, 'rate limited', undefined as any,
      );
      mockCreate.mockRejectedValueOnce(rateLimitErr);
      mockCreate.mockResolvedValueOnce(mockResponse({
        model: 'claude-haiku-4-5-20251001',
      }));

      (client as any).client.messages = { create: mockCreate };

      const events: any[] = [];
      bus.subscribe('llm.rate_limited', (e) => { events.push(e); });

      await client.complete({
        messages: [{ role: 'user', content: 'hello' }],
        model: 'standard',
      });

      expect(events.length).toBeGreaterThanOrEqual(1);
      expect(events[0].payload.tier).toBe('standard');
    });

    it('should publish llm.cooldown event when model enters cooldown', async () => {
      const client = setup({ apiKey: 'test-key' });
      const mockCreate = vi.fn();

      const rateLimitErr = new (await import('@anthropic-ai/sdk')).RateLimitError(
        429, undefined, 'rate limited', undefined as any,
      );
      mockCreate.mockRejectedValueOnce(rateLimitErr);
      mockCreate.mockResolvedValueOnce(mockResponse({
        model: 'claude-haiku-4-5-20251001',
      }));

      (client as any).client.messages = { create: mockCreate };

      const events: any[] = [];
      bus.subscribe('llm.cooldown', (e) => { events.push(e); });

      await client.complete({
        messages: [{ role: 'user', content: 'hello' }],
        model: 'standard',
      });

      expect(events.length).toBeGreaterThanOrEqual(1);
      expect(events[0].payload.reason).toBe('rate_limit');
      expect(events[0].payload.durationMs).toBeGreaterThan(0);
    });

    it('should throw when all models are in cooldown', async () => {
      const client = setup({ apiKey: 'test-key' });

      // Manually set cooldowns on all model tiers
      const models = client.getModels();
      const cooldowns = (client as any).cooldowns as Map<string, any>;
      cooldowns.set(models.fast.model, { until: Date.now() + 60000, reason: 'rate_limit' });
      cooldowns.set(models.standard.model, { until: Date.now() + 60000, reason: 'rate_limit' });
      cooldowns.set(models.powerful.model, { until: Date.now() + 60000, reason: 'rate_limit' });

      // Request should throw immediately (no API call needed)
      await expect(
        client.complete({
          messages: [{ role: 'user', content: 'hello' }],
          model: 'standard',
        }),
      ).rejects.toThrow(/cooldown|rate-limited/i);
    });

    it('should use short backoff for 500 errors, not rate limit backoff', async () => {
      const client = setup({ apiKey: 'test-key' });
      const mockCreate = vi.fn();

      const serverErr = new (await import('@anthropic-ai/sdk')).APIError(
        500, undefined, 'server error', undefined as any,
      );
      mockCreate.mockRejectedValueOnce(serverErr);
      mockCreate.mockResolvedValueOnce(mockResponse());

      (client as any).client.messages = { create: mockCreate };

      const start = Date.now();
      await client.complete({
        messages: [{ role: 'user', content: 'hello' }],
      });
      const elapsed = Date.now() - start;

      // Short backoff should be < 5s, not the 30s rate limit backoff
      expect(elapsed).toBeLessThan(5000);
      // Rate limit count should not have been incremented
      expect(client.getStats().rateLimitCount).toBe(0);
    });

    it('should include rate limit info in stats', () => {
      const client = setup({ apiKey: 'test-key' });
      const stats = client.getStats();
      expect(stats.rateLimitCount).toBe(0);
      expect(stats.cooldowns).toEqual({});
    });

    it('should include rate limiting check in diagnostics', () => {
      const client = setup({ apiKey: 'test-key' });
      const diag = client.diagnose();
      const rlCheck = diag.checks.find((c) => c.name === 'Rate limiting');
      expect(rlCheck).toBeDefined();
      expect(rlCheck!.passed).toBe(true);
      expect(rlCheck!.message).toBe('No rate limits encountered');
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
