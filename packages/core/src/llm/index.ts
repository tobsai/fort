/**
 * LLM Client — Anthropic Claude API with Model Routing
 *
 * Provides a unified interface for all LLM interactions in Fort.
 * Supports model routing (cheap models for simple tasks, powerful models
 * for complex reasoning), conversation context management, token tracking,
 * and behavioral system prompt injection.
 *
 * Design principle: "Deterministic by default, generative when needed."
 * This client is only invoked for steps that genuinely require reasoning.
 */

import { execSync } from 'node:child_process';
import Anthropic from '@anthropic-ai/sdk';
import type { ModuleBus } from '../module-bus/index.js';
import type { TokenTracker } from '../tokens/index.js';
import type { BehaviorManager } from '../behaviors/index.js';
import type { MemoryManager } from '../memory/index.js';
import type { DiagnosticResult } from '../types.js';

// ─── Types ──────────────────────────────────────────────────────────

export type ModelTier = 'fast' | 'standard' | 'powerful';

export interface ModelConfig {
  tier: ModelTier;
  model: string;
  maxTokens: number;
  description: string;
}

export interface LLMMessage {
  role: 'user' | 'assistant';
  content: string;
}

export interface LLMRequest {
  messages: LLMMessage[];
  system?: string;
  model?: ModelTier | string;
  maxTokens?: number;
  temperature?: number;
  taskId?: string;
  agentId?: string;
  context?: string[];
  injectBehaviors?: boolean;
  injectMemory?: string;
  stream?: boolean;
}

export interface LLMResponse {
  content: string;
  model: string;
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
  costUsd: number;
  stopReason: string | null;
  durationMs: number;
}

export interface LLMStreamEvent {
  type: 'text' | 'done' | 'error';
  text?: string;
  response?: LLMResponse;
  error?: string;
}

export interface LLMClientConfig {
  apiKey?: string;
  defaultModel?: ModelTier;
  models?: Partial<Record<ModelTier, ModelConfig>>;
  maxRetries?: number;
  systemPrompt?: string;
}

// ─── Default Model Routing ──────────────────────────────────────────

const DEFAULT_MODELS: Record<ModelTier, ModelConfig> = {
  fast: {
    tier: 'fast',
    model: 'claude-haiku-4-5-20250315',
    maxTokens: 2048,
    description: 'Fast and cheap — simple tasks, classification, extraction',
  },
  standard: {
    tier: 'standard',
    model: 'claude-sonnet-4-6-20250311',
    maxTokens: 4096,
    description: 'Balanced — most tasks, coding, analysis',
  },
  powerful: {
    tier: 'powerful',
    model: 'claude-opus-4-6-20250311',
    maxTokens: 8192,
    description: 'Maximum reasoning — complex planning, architecture, nuanced decisions',
  },
};

const DEFAULT_SYSTEM_PROMPT = `You are Fort, a personal AI agent platform. You are helpful, concise, and action-oriented. You prefer to take action rather than ask unnecessary clarifying questions. When given a task, you execute it and report results.`;

// ─── Pricing (per 1M tokens) ────────────────────────────────────────

const PRICING: Record<string, { input: number; output: number }> = {
  'claude-haiku-4-5-20250315': { input: 0.80, output: 4.00 },
  'claude-sonnet-4-6-20250311': { input: 3.00, output: 15.00 },
  'claude-opus-4-6-20250311': { input: 15.00, output: 75.00 },
};

// ─── LLM Client ─────────────────────────────────────────────────────

export class LLMClient {
  private client: Anthropic | null = null;
  private models: Record<ModelTier, ModelConfig>;
  private defaultTier: ModelTier;
  private systemPrompt: string;
  private maxRetries: number;
  private bus: ModuleBus;
  private tokenTracker: TokenTracker | null;
  private behaviors: BehaviorManager | null;
  private memory: MemoryManager | null;

  // Stats
  private requestCount = 0;
  private totalInputTokens = 0;
  private totalOutputTokens = 0;
  private totalCostUsd = 0;
  private errorCount = 0;
  private startedAt = new Date();

  constructor(
    config: LLMClientConfig,
    bus: ModuleBus,
    tokenTracker?: TokenTracker,
    behaviors?: BehaviorManager,
    memory?: MemoryManager,
  ) {
    this.bus = bus;
    this.tokenTracker = tokenTracker ?? null;
    this.behaviors = behaviors ?? null;
    this.memory = memory ?? null;
    this.defaultTier = config.defaultModel ?? 'standard';
    this.maxRetries = config.maxRetries ?? 2;
    this.systemPrompt = config.systemPrompt ?? DEFAULT_SYSTEM_PROMPT;

    // Merge custom models with defaults
    this.models = { ...DEFAULT_MODELS };
    if (config.models) {
      for (const [tier, modelConfig] of Object.entries(config.models)) {
        if (modelConfig) {
          this.models[tier as ModelTier] = modelConfig;
        }
      }
    }

    // Initialize Anthropic client
    // Priority: explicit config > Claude Code env token > keychain token > ANTHROPIC_API_KEY
    const oauthToken = process.env.CLAUDE_CODE_OAUTH_TOKEN;
    const apiKey = config.apiKey ?? process.env.ANTHROPIC_API_KEY;

    if (config.apiKey) {
      this.client = new Anthropic({ apiKey: config.apiKey });
      this._authMethod = 'api_key_config';
    } else if (oauthToken) {
      this.client = new Anthropic({ apiKey: oauthToken });
      this._authMethod = 'claude_code_oauth';
    } else if (apiKey) {
      this.client = new Anthropic({ apiKey });
      this._authMethod = 'api_key_env';
    } else {
      // Try reading from macOS Keychain (set by `claude setup-token`)
      const keychainToken = LLMClient.readKeychainToken();
      if (keychainToken) {
        this.client = new Anthropic({ apiKey: keychainToken });
        this._authMethod = 'claude_code_keychain';
      }
    }
    // If nothing found, client stays null — requests will return helpful errors
  }

  private _authMethod: 'api_key_config' | 'claude_code_oauth' | 'claude_code_keychain' | 'api_key_env' | null = null;

  /**
   * Try to read the Claude Code token from the macOS Keychain.
   * Returns null on non-macOS or if no token is stored.
   */
  static readKeychainToken(): string | null {
    if (process.platform !== 'darwin') return null;
    try {
      const token = execSync(
        'security find-generic-password -s "Claude Code-credentials" -w 2>/dev/null',
        { encoding: 'utf-8', stdio: ['pipe', 'pipe', 'pipe'] },
      ).trim();
      return token.length > 0 ? token : null;
    } catch {
      return null;
    }
  }

  /**
   * How the client authenticated. Null if not configured.
   */
  get authMethod(): string | null {
    return this._authMethod;
  }

  /**
   * Check if the LLM client is configured and ready.
   */
  get isConfigured(): boolean {
    return this.client !== null;
  }

  /**
   * Send a completion request to Claude.
   */
  async complete(request: LLMRequest): Promise<LLMResponse> {
    if (!this.client) {
      throw new Error(
        'LLM client not configured. Set ANTHROPIC_API_KEY environment variable or pass apiKey in config.',
      );
    }

    const modelConfig = this.resolveModel(request.model);
    const system = await this.buildSystemPrompt(request);
    const maxTokens = request.maxTokens ?? modelConfig.maxTokens;

    const start = Date.now();
    let lastError: Error | null = null;

    for (let attempt = 0; attempt <= this.maxRetries; attempt++) {
      try {
        const response = await this.client.messages.create({
          model: modelConfig.model,
          max_tokens: maxTokens,
          system,
          messages: request.messages.map((m) => ({
            role: m.role,
            content: m.content,
          })),
          temperature: request.temperature,
        });

        const durationMs = Date.now() - start;
        const inputTokens = response.usage.input_tokens;
        const outputTokens = response.usage.output_tokens;
        const totalTokens = inputTokens + outputTokens;
        const costUsd = this.calculateCost(modelConfig.model, inputTokens, outputTokens);

        const content =
          response.content[0]?.type === 'text' ? response.content[0].text : '';

        const result: LLMResponse = {
          content,
          model: modelConfig.model,
          inputTokens,
          outputTokens,
          totalTokens,
          costUsd,
          stopReason: response.stop_reason,
          durationMs,
        };

        // Track usage
        this.requestCount++;
        this.totalInputTokens += inputTokens;
        this.totalOutputTokens += outputTokens;
        this.totalCostUsd += costUsd;

        // Record in token tracker
        if (this.tokenTracker) {
          this.tokenTracker.record({
            timestamp: new Date(),
            model: modelConfig.model,
            inputTokens,
            outputTokens,
            totalTokens,
            costUsd,
            taskId: request.taskId,
            agentId: request.agentId,
            source: 'llm_client',
          });
        }

        // Publish usage event
        this.bus.publish('llm.completed', 'llm-client', {
          model: modelConfig.model,
          tier: modelConfig.tier,
          inputTokens,
          outputTokens,
          costUsd,
          durationMs,
          taskId: request.taskId,
          agentId: request.agentId,
        });

        return result;
      } catch (err) {
        lastError = err instanceof Error ? err : new Error(String(err));
        this.errorCount++;

        if (attempt < this.maxRetries) {
          // Exponential backoff
          const delay = Math.min(1000 * Math.pow(2, attempt), 10000);
          await new Promise((resolve) => setTimeout(resolve, delay));

          this.bus.publish('llm.retry', 'llm-client', {
            attempt: attempt + 1,
            error: lastError.message,
            model: modelConfig.model,
          });
        }
      }
    }

    this.bus.publish('llm.error', 'llm-client', {
      error: lastError?.message,
      model: modelConfig.model,
      taskId: request.taskId,
    });

    throw lastError;
  }

  /**
   * Stream a completion response.
   * Returns an async generator of stream events.
   */
  async *stream(request: LLMRequest): AsyncGenerator<LLMStreamEvent> {
    if (!this.client) {
      yield {
        type: 'error',
        error: 'LLM client not configured. Set ANTHROPIC_API_KEY.',
      };
      return;
    }

    const modelConfig = this.resolveModel(request.model);
    const system = await this.buildSystemPrompt(request);
    const maxTokens = request.maxTokens ?? modelConfig.maxTokens;
    const start = Date.now();

    try {
      const stream = this.client.messages.stream({
        model: modelConfig.model,
        max_tokens: maxTokens,
        system,
        messages: request.messages.map((m) => ({
          role: m.role,
          content: m.content,
        })),
        temperature: request.temperature,
      });

      let fullContent = '';

      for await (const event of stream) {
        if (
          event.type === 'content_block_delta' &&
          event.delta.type === 'text_delta'
        ) {
          fullContent += event.delta.text;
          yield { type: 'text', text: event.delta.text };
        }
      }

      const finalMessage = await stream.finalMessage();
      const durationMs = Date.now() - start;
      const inputTokens = finalMessage.usage.input_tokens;
      const outputTokens = finalMessage.usage.output_tokens;
      const costUsd = this.calculateCost(modelConfig.model, inputTokens, outputTokens);

      this.requestCount++;
      this.totalInputTokens += inputTokens;
      this.totalOutputTokens += outputTokens;
      this.totalCostUsd += costUsd;

      if (this.tokenTracker) {
        this.tokenTracker.record({
          timestamp: new Date(),
          model: modelConfig.model,
          inputTokens,
          outputTokens,
          totalTokens: inputTokens + outputTokens,
          costUsd,
          taskId: request.taskId,
          agentId: request.agentId,
          source: 'llm_client_stream',
        });
      }

      yield {
        type: 'done',
        response: {
          content: fullContent,
          model: modelConfig.model,
          inputTokens,
          outputTokens,
          totalTokens: inputTokens + outputTokens,
          costUsd,
          stopReason: finalMessage.stop_reason,
          durationMs,
        },
      };
    } catch (err) {
      this.errorCount++;
      const message = err instanceof Error ? err.message : String(err);
      yield { type: 'error', error: message };
    }
  }

  /**
   * Quick helper — single-turn completion with just a prompt string.
   */
  async ask(
    prompt: string,
    opts?: {
      model?: ModelTier | string;
      taskId?: string;
      agentId?: string;
      system?: string;
      injectBehaviors?: boolean;
      injectMemory?: string;
    },
  ): Promise<string> {
    const response = await this.complete({
      messages: [{ role: 'user', content: prompt }],
      system: opts?.system,
      model: opts?.model,
      taskId: opts?.taskId,
      agentId: opts?.agentId,
      injectBehaviors: opts?.injectBehaviors,
      injectMemory: opts?.injectMemory,
    });
    return response.content;
  }

  /**
   * Route a prompt to the appropriate model tier based on complexity.
   * Uses the fast model to classify, then routes to the right tier.
   */
  async routedComplete(request: LLMRequest): Promise<LLMResponse> {
    // If model already specified, use it directly
    if (request.model) {
      return this.complete(request);
    }

    // Estimate complexity from message length and content
    const tier = this.estimateComplexity(request.messages);
    return this.complete({ ...request, model: tier });
  }

  /**
   * Get current LLM client stats.
   */
  getStats() {
    return {
      configured: this.isConfigured,
      authMethod: this._authMethod,
      defaultTier: this.defaultTier,
      requestCount: this.requestCount,
      totalInputTokens: this.totalInputTokens,
      totalOutputTokens: this.totalOutputTokens,
      totalCostUsd: this.totalCostUsd,
      errorCount: this.errorCount,
      uptime: Date.now() - this.startedAt.getTime(),
      models: Object.fromEntries(
        Object.entries(this.models).map(([tier, config]) => [
          tier,
          { model: config.model, description: config.description },
        ]),
      ),
    };
  }

  /**
   * Get available model configurations.
   */
  getModels(): Record<ModelTier, ModelConfig> {
    return { ...this.models };
  }

  diagnose(): DiagnosticResult {
    const checks = [
      {
        name: 'Authentication',
        passed: this.isConfigured,
        message: this.isConfigured
          ? this._authMethod === 'claude_code_oauth'
            ? 'Authenticated via Claude Code session token'
            : this._authMethod === 'claude_code_keychain'
              ? 'Authenticated via Claude Code subscription (keychain)'
              : this._authMethod === 'api_key_config'
                ? 'Authenticated via config file API key'
                : 'Authenticated via ANTHROPIC_API_KEY environment variable'
          : 'Not configured — run `fort llm setup` for instructions',
      },
      {
        name: 'Request count',
        passed: true,
        message: `${this.requestCount} requests made`,
      },
      {
        name: 'Error rate',
        passed: this.requestCount === 0 || this.errorCount / this.requestCount < 0.1,
        message:
          this.requestCount === 0
            ? 'No requests yet'
            : `${this.errorCount}/${this.requestCount} errors (${((this.errorCount / this.requestCount) * 100).toFixed(1)}%)`,
      },
      {
        name: 'Total cost',
        passed: true,
        message: `$${this.totalCostUsd.toFixed(4)} spent`,
      },
      {
        name: 'Default model',
        passed: true,
        message: `${this.defaultTier} → ${this.models[this.defaultTier].model}`,
      },
    ];

    return {
      module: 'llm',
      status: !this.isConfigured
        ? 'degraded'
        : this.errorCount > 0 && this.requestCount > 0 && this.errorCount / this.requestCount > 0.5
          ? 'degraded'
          : 'healthy',
      checks,
    };
  }

  // ─── Private Methods ──────────────────────────────────────────────

  private resolveModel(modelSpec?: ModelTier | string): ModelConfig {
    if (!modelSpec) {
      return this.models[this.defaultTier];
    }

    // Check if it's a tier name
    if (modelSpec in this.models) {
      return this.models[modelSpec as ModelTier];
    }

    // It's a raw model name — wrap it in a config
    return {
      tier: 'standard',
      model: modelSpec,
      maxTokens: 4096,
      description: `Custom model: ${modelSpec}`,
    };
  }

  private async buildSystemPrompt(request: LLMRequest): Promise<string> {
    const parts: string[] = [request.system ?? this.systemPrompt];

    // Inject relevant behaviors
    if (request.injectBehaviors !== false && this.behaviors) {
      const contexts = this.extractContexts(request.messages);
      const allBehaviors: string[] = [];
      const behaviors = this.behaviors.getRelevantBehaviors(contexts);
      for (const b of behaviors) {
        if (!allBehaviors.includes(b.rule)) {
          allBehaviors.push(b.rule);
        }
      }
      if (allBehaviors.length > 0) {
        parts.push(
          '\n\n## Active Behaviors\nFollow these behavioral rules:\n' +
            allBehaviors.map((b) => `- ${b}`).join('\n'),
        );
      }
    }

    // Inject relevant memories
    if (request.injectMemory && this.memory) {
      const results = this.memory.search({ text: request.injectMemory, limit: 10 });
      if (results.nodes.length > 0) {
        const memoryLines = results.nodes.map((n) => `- ${n.label}: ${JSON.stringify(n.properties)}`);
        parts.push(
          '\n\n## Relevant Memories\n' + memoryLines.join('\n'),
        );
      }
    }

    // Inject additional context
    if (request.context && request.context.length > 0) {
      parts.push('\n\n## Additional Context\n' + request.context.join('\n'));
    }

    return parts.join('');
  }

  private extractContexts(messages: LLMMessage[]): string[] {
    // Extract likely contexts from message content for behavior lookup
    const contexts = new Set<string>();
    const lastMessage = messages[messages.length - 1];
    if (!lastMessage) return [];

    const text = lastMessage.content.toLowerCase();
    if (text.includes('email') || text.includes('mail')) contexts.add('email');
    if (text.includes('calendar') || text.includes('meeting') || text.includes('schedule'))
      contexts.add('calendar');
    if (text.includes('code') || text.includes('implement') || text.includes('build'))
      contexts.add('coding');
    if (text.includes('research') || text.includes('search') || text.includes('find'))
      contexts.add('research');
    if (text.includes('message') || text.includes('text') || text.includes('imessage'))
      contexts.add('messaging');

    // Always include 'general'
    contexts.add('general');
    return Array.from(contexts);
  }

  private estimateComplexity(messages: LLMMessage[]): ModelTier {
    const totalLength = messages.reduce((sum, m) => sum + m.content.length, 0);
    const lastMessage = messages[messages.length - 1]?.content ?? '';

    // Long conversations or complex prompts → powerful
    if (totalLength > 4000 || messages.length > 10) return 'powerful';

    // Keywords suggesting complex reasoning
    const complexKeywords = [
      'analyze', 'architect', 'design', 'plan', 'compare',
      'trade-off', 'strategy', 'complex', 'nuanced', 'comprehensive',
      'review', 'debug', 'refactor',
    ];
    if (complexKeywords.some((k) => lastMessage.toLowerCase().includes(k))) {
      return 'standard';
    }

    // Short, simple queries → fast
    if (totalLength < 200 && messages.length <= 2) return 'fast';

    return 'standard';
  }

  private calculateCost(model: string, inputTokens: number, outputTokens: number): number {
    const pricing = PRICING[model];
    if (!pricing) return 0;
    return (inputTokens * pricing.input + outputTokens * pricing.output) / 1_000_000;
  }
}
