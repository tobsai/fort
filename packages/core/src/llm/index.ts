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

import { readFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';
import { homedir } from 'node:os';
import Anthropic from '@anthropic-ai/sdk';
import type { ModuleBus } from '../module-bus/index.js';
import type { TokenTracker } from '../tokens/index.js';
import type { BehaviorManager } from '../behaviors/index.js';
import type { MemoryManager } from '../memory/index.js';
import type { DiagnosticResult } from '../types.js';
import type { FortTool, ToolCallLog } from '../tools/types.js';
import type { ToolExecutor } from '../tools/executor.js';
import type { LLMProviderStore, LLMProviderRuntime } from './provider-store.js';

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
  soul?: string;
  stream?: boolean;
  /** Optional tools to expose to the LLM for this request */
  tools?: FortTool[];
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

export interface LLMToolsResponse extends LLMResponse {
  /** Log of every tool call made during the multi-turn loop */
  toolCallLog: ToolCallLog[];
  /** Number of LLM turns used (1 = no tools called) */
  iterations: number;
}

export interface LLMClientConfig {
  apiKey?: string;
  defaultModel?: ModelTier;
  models?: Partial<Record<ModelTier, ModelConfig>>;
  maxRetries?: number;
  systemPrompt?: string;
  providerStore?: LLMProviderStore;
}

// ─── Default Model Routing ──────────────────────────────────────────

const DEFAULT_MODELS: Record<ModelTier, ModelConfig> = {
  fast: {
    tier: 'fast',
    model: 'claude-haiku-4-5-20251001',
    maxTokens: 2048,
    description: 'Fast and cheap — simple tasks, classification, extraction',
  },
  standard: {
    tier: 'standard',
    model: 'claude-sonnet-4-5-20250929',
    maxTokens: 4096,
    description: 'Balanced — most tasks, coding, analysis',
  },
  powerful: {
    tier: 'powerful',
    model: 'claude-opus-4-6',
    maxTokens: 8192,
    description: 'Maximum reasoning — complex planning, architecture, nuanced decisions',
  },
};

const DEFAULT_SYSTEM_PROMPT = `You are Fort, a personal AI agent platform. You are helpful, concise, and action-oriented. You prefer to take action rather than ask unnecessary clarifying questions. When given a task, you execute it and report results.`;

// ─── Pricing (per 1M tokens) ────────────────────────────────────────

const PRICING: Record<string, { input: number; output: number }> = {
  'claude-haiku-4-5-20251001': { input: 0.80, output: 4.00 },
  'claude-sonnet-4-5-20250929': { input: 3.00, output: 15.00 },
  'claude-opus-4-6': { input: 15.00, output: 75.00 },
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
  private providerStore: LLMProviderStore | null;

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
    this.providerStore = config.providerStore ?? null;
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
    // Priority: explicit config > ~/.fort/.env > ANTHROPIC_API_KEY env
    // Keychain is never read at runtime — `fort init` extracts to .env
    const envFileToken = LLMClient.readEnvFile();
    const apiKey = process.env.ANTHROPIC_API_KEY;

    const resolvedToken = config.apiKey || envFileToken || apiKey;
    if (resolvedToken) {
      // OAuth tokens (sk-ant-oat*) need authToken + beta header; API keys use apiKey
      this._isOAuthToken = resolvedToken.startsWith('sk-ant-oat');
      if (this._isOAuthToken) {
        this.client = new Anthropic({
          authToken: resolvedToken,
          defaultHeaders: { 'anthropic-beta': 'oauth-2025-04-20' },
        });
      } else {
        this.client = new Anthropic({ apiKey: resolvedToken });
      }
      this._authMethod = config.apiKey ? 'api_key_config'
        : envFileToken ? 'dotenv'
        : 'api_key_env';
    }
    // If nothing found, client stays null — requests will return helpful errors
  }

  private _authMethod: 'api_key_config' | 'dotenv' | 'api_key_env' | null = null;
  private _isOAuthToken = false;

  /**
   * Read the API key from ~/.fort/.env
   * The file is a simple KEY=VALUE format, one per line.
   */
  static readEnvFile(): string | null {
    const envPath = join(homedir(), '.fort', '.env');
    if (!existsSync(envPath)) return null;
    try {
      const content = readFileSync(envPath, 'utf-8');
      for (const line of content.split('\n')) {
        const trimmed = line.trim();
        if (trimmed.startsWith('#') || !trimmed) continue;
        const match = trimmed.match(/^ANTHROPIC_API_KEY\s*=\s*["']?(.+?)["']?\s*$/);
        if (match) return match[1];
      }
      return null;
    } catch {
      return null;
    }
  }

  /**
   * Read the OAuth token from the macOS keychain (set by `claude setup-token`).
   * Returns null on non-macOS or if no credential exists.
   */
  static readKeychainToken(): string | null {
    if (process.platform !== 'darwin') return null;
    try {
      const { execSync } = require('node:child_process');
      const raw = execSync(
        'security find-generic-password -s "Claude Code-credentials" -w 2>/dev/null',
        { encoding: 'utf-8', stdio: ['pipe', 'pipe', 'pipe'] },
      ).trim();
      if (!raw) return null;
      try {
        const parsed = JSON.parse(raw);
        return parsed?.claudeAiOauth?.accessToken ?? null;
      } catch {
        // Might be a raw token string
        return raw.startsWith('sk-ant-') ? raw : null;
      }
    } catch {
      return null;
    }
  }

  /**
   * Write an API key to ~/.fort/.env
   */
  static writeEnvFile(apiKey: string): void {
    const fortDir = join(homedir(), '.fort');
    const envPath = join(fortDir, '.env');
    const { mkdirSync, writeFileSync: writeFile } = require('node:fs');
    if (!existsSync(fortDir)) mkdirSync(fortDir, { recursive: true });

    // Read existing content, replace or append
    let content = '';
    if (existsSync(envPath)) {
      content = readFileSync(envPath, 'utf-8');
      // Replace existing key
      if (content.match(/^ANTHROPIC_API_KEY\s*=/m)) {
        content = content.replace(/^ANTHROPIC_API_KEY\s*=.*$/m, `ANTHROPIC_API_KEY=${apiKey}`);
      } else {
        content = content.trimEnd() + `\nANTHROPIC_API_KEY=${apiKey}\n`;
      }
    } else {
      content = `# Fort API Configuration\n# Generated by \`fort llm setup\`\n\nANTHROPIC_API_KEY=${apiKey}\n`;
    }

    writeFile(envPath, content, 'utf-8');
  }

  /**
   * Get the path to the .env file.
   */
  static get envFilePath(): string {
    return join(homedir(), '.fort', '.env');
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
    return this.client !== null || this.getActiveProvider() !== null;
  }

  /**
   * Validate that the configured API key actually works.
   * Makes a minimal API call to verify authentication.
   * Returns null on success, or an error message string on failure.
   */
  async validateAuth(): Promise<string | null> {
    if (!this.client) {
      return 'LLM client not configured. Run `fort llm setup` or set ANTHROPIC_API_KEY.';
    }
    // Test with the default tier model to catch subscription-level access issues
    const testModel = this.models[this.defaultTier]?.model ?? this.models.fast.model;
    try {
      await this.client.messages.create({
        model: testModel,
        max_tokens: 1,
        messages: [{ role: 'user', content: 'hi' }],
      });
      return null;
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg.includes('401') || msg.includes('authentication') || msg.includes('Invalid bearer')) {
        return 'API key is invalid or expired. Run `fort llm setup` to re-authenticate.';
      }
      // 400 errors with OAuth often mean the model isn't available on this subscription tier
      if (msg.includes('400') && this._isOAuthToken) {
        // Fall back to haiku if the default model isn't available
        if (testModel !== this.models.fast.model) {
          try {
            await this.client.messages.create({
              model: this.models.fast.model,
              max_tokens: 1,
              messages: [{ role: 'user', content: 'hi' }],
            });
            // Haiku works but the default model doesn't — switch default to fast
            this.defaultTier = 'fast';
            return null;
          } catch {
            // Even haiku failed
          }
        }
        return `Model "${testModel}" is not available on your subscription. Try switching to the Fast (Haiku) model tier.`;
      }
      return `API connection error: ${msg}`;
    }
  }

  /**
   * Send a completion request to Claude.
   */
  async complete(request: LLMRequest): Promise<LLMResponse> {
    const client = this.resolveClient(request.agentId);
    if (!client) {
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
        const claudeTools: Anthropic.Tool[] | undefined =
          request.tools && request.tools.length > 0
            ? request.tools.map((t) => ({
                name: t.name,
                description: t.description,
                input_schema: t.inputSchema as Anthropic.Tool['input_schema'],
              }))
            : undefined;

        const response = await client.messages.create({
          model: modelConfig.model,
          max_tokens: maxTokens,
          system,
          messages: request.messages.map((m) => ({
            role: m.role,
            content: m.content,
          })),
          temperature: request.temperature,
          ...(claudeTools ? { tools: claudeTools } : {}),
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

    // If using OAuth and a non-fast model got a 400, fall back to fast tier
    const errMsg = lastError?.message ?? '';
    if (
      this._isOAuthToken &&
      errMsg.includes('400') &&
      modelConfig.model !== this.models.fast.model
    ) {
      this.bus.publish('llm.retry', 'llm-client', {
        attempt: 'fallback',
        error: `Model "${modelConfig.model}" unavailable, falling back to ${this.models.fast.model}`,
        model: this.models.fast.model,
      });
      return this.complete({ ...request, model: 'fast' });
    }

    this.bus.publish('llm.error', 'llm-client', {
      error: lastError?.message,
      model: modelConfig.model,
      taskId: request.taskId,
    });

    throw lastError;
  }

  /**
   * Complete a request with tool use support.
   *
   * Handles the multi-turn loop:
   *   1. Send message with tools
   *   2. If response has tool_use → execute via ToolExecutor → append tool_result → re-send
   *   3. Repeat until pure text response (or max iterations hit)
   *   4. Return final text + tool call log
   *
   * Max iterations defaults to 10 to prevent runaway loops.
   */
  async completeWithTools(
    request: LLMRequest & { tools: FortTool[] },
    executor: ToolExecutor,
    opts: { maxIterations?: number } = {},
  ): Promise<LLMToolsResponse> {
    const client = this.resolveClient(request.agentId);
    if (!client) {
      throw new Error(
        'LLM client not configured. Set ANTHROPIC_API_KEY environment variable or pass apiKey in config.',
      );
    }

    const MAX_ITERATIONS = opts.maxIterations ?? 10;
    const toolCallLog: ToolCallLog[] = [];

    // Collect tool call log entries from bus events published by ToolExecutor
    const unsubExecuted = this.bus.subscribe('tool.executed', (event) => {
      toolCallLog.push(event.payload as ToolCallLog);
    });
    const unsubDenied = this.bus.subscribe('tool.denied', (event) => {
      toolCallLog.push(event.payload as ToolCallLog);
    });
    const unsubError = this.bus.subscribe('tool.error', (event) => {
      toolCallLog.push(event.payload as ToolCallLog);
    });

    try {
      const modelConfig = this.resolveModel(request.model);
      const system = await this.buildSystemPrompt(request);
      const maxTokens = request.maxTokens ?? modelConfig.maxTokens;

      // Convert FortTool[] to Claude's tools format
      const claudeTools: Anthropic.Tool[] = request.tools.map((t) => ({
        name: t.name,
        description: t.description,
        input_schema: t.inputSchema as Anthropic.Tool['input_schema'],
      }));

      // Build name → FortTool lookup for fast resolution during the loop
      const toolMap = new Map<string, FortTool>(request.tools.map((t) => [t.name, t]));

      // Maintain multi-turn message history with proper Anthropic types
      const messages: Anthropic.MessageParam[] = request.messages.map((m) => ({
        role: m.role,
        content: m.content,
      }));

      let iteration = 0;
      let totalInputTokens = 0;
      let totalOutputTokens = 0;
      let totalCostUsd = 0;
      const start = Date.now();

      while (iteration < MAX_ITERATIONS) {
        iteration++;

        const response = await client.messages.create({
          model: modelConfig.model,
          max_tokens: maxTokens,
          system,
          messages,
          tools: claudeTools,
          temperature: request.temperature,
        });

        const inputTokens = response.usage.input_tokens;
        const outputTokens = response.usage.output_tokens;
        const costUsd = this.calculateCost(modelConfig.model, inputTokens, outputTokens);
        totalInputTokens += inputTokens;
        totalOutputTokens += outputTokens;
        totalCostUsd += costUsd;

        // Update aggregate stats
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
            source: 'llm_client_tools',
          });
        }

        // Pure text response — loop is done
        if (response.stop_reason !== 'tool_use') {
          const durationMs = Date.now() - start;
          const textBlock = response.content.find(
            (b): b is Anthropic.TextBlock => b.type === 'text',
          );

          this.bus.publish('llm.completed', 'llm-client', {
            model: modelConfig.model,
            tier: modelConfig.tier,
            inputTokens: totalInputTokens,
            outputTokens: totalOutputTokens,
            costUsd: totalCostUsd,
            durationMs,
            taskId: request.taskId,
            agentId: request.agentId,
            toolCalls: toolCallLog.length,
          });

          return {
            content: textBlock?.text ?? '',
            model: modelConfig.model,
            inputTokens: totalInputTokens,
            outputTokens: totalOutputTokens,
            totalTokens: totalInputTokens + totalOutputTokens,
            costUsd: totalCostUsd,
            stopReason: response.stop_reason,
            durationMs,
            toolCallLog,
            iterations: iteration,
          };
        }

        // Tool use — collect tool_use blocks
        const toolUseBlocks = response.content.filter(
          (b): b is Anthropic.ToolUseBlock => b.type === 'tool_use',
        );

        // Append assistant turn (includes tool_use blocks) to history
        messages.push({ role: 'assistant', content: response.content });

        // Execute each tool and build tool_result blocks
        const toolResults: Anthropic.ToolResultBlockParam[] = [];

        for (const block of toolUseBlocks) {
          const tool = toolMap.get(block.name);
          if (!tool) {
            toolResults.push({
              type: 'tool_result',
              tool_use_id: block.id,
              content: `Error: Tool "${block.name}" not found in registry`,
              is_error: true,
            });
            continue;
          }

          const toolResult = await executor.execute(tool, block.input, {
            taskId: request.taskId,
            agentId: request.agentId,
          });

          toolResults.push({
            type: 'tool_result',
            tool_use_id: block.id,
            content: toolResult.output || toolResult.error || '',
            is_error: !toolResult.success,
          });
        }

        // Append tool results as the next user turn
        messages.push({ role: 'user', content: toolResults });
      }

      // Max iterations reached
      const durationMs = Date.now() - start;

      this.bus.publish('llm.completed', 'llm-client', {
        model: modelConfig.model,
        tier: modelConfig.tier,
        inputTokens: totalInputTokens,
        outputTokens: totalOutputTokens,
        costUsd: totalCostUsd,
        durationMs,
        taskId: request.taskId,
        agentId: request.agentId,
        toolCalls: toolCallLog.length,
        maxIterationsReached: true,
      });

      return {
        content: '',
        model: modelConfig.model,
        inputTokens: totalInputTokens,
        outputTokens: totalOutputTokens,
        totalTokens: totalInputTokens + totalOutputTokens,
        costUsd: totalCostUsd,
        stopReason: 'max_iterations',
        durationMs,
        toolCallLog,
        iterations: MAX_ITERATIONS,
      };
    } finally {
      unsubExecuted();
      unsubDenied();
      unsubError();
    }
  }

  /**
   * Stream a completion response.
   * Returns an async generator of stream events.
   */
  async *stream(request: LLMRequest): AsyncGenerator<LLMStreamEvent> {
    const client = this.resolveClient(request.agentId);
    if (!client) {
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
      const stream = client.messages.stream({
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

  /**
   * Get the active provider for a given agent (or the global default).
   * Resolution order: agent DB preference → global DB default → env var fallback (null).
   */
  getActiveProvider(_agentId?: string): LLMProviderRuntime | null {
    if (!this.providerStore) return null;
    return this.providerStore.getDefaultProviderRuntime();
  }

  /**
   * Test connectivity to a configured provider.
   * Sends a minimal prompt and returns null on success or an error string on failure.
   */
  async testConnection(providerId: string): Promise<string | null> {
    if (!this.providerStore) return 'Provider store not configured';
    const runtime = this.providerStore.getProviderRuntime(providerId);
    if (!runtime) return `Provider not found: ${providerId}`;

    try {
      if (providerId === 'anthropic') {
        const key = runtime.apiKey;
        if (!key) return 'No API key configured for Anthropic';
        const isOAuth = key.startsWith('sk-ant-oat');
        const testClient = isOAuth
          ? new Anthropic({ authToken: key, defaultHeaders: { 'anthropic-beta': 'oauth-2025-04-20' } })
          : new Anthropic({ apiKey: key });
        await testClient.messages.create({
          model: runtime.defaultModel || 'claude-haiku-4-5-20251001',
          max_tokens: 1,
          messages: [{ role: 'user', content: 'hi' }],
        });
        return null;
      }

      if (providerId === 'ollama') {
        const baseUrl = runtime.baseUrl ?? 'http://localhost:11434';
        const res = await fetch(`${baseUrl}/api/tags`);
        if (!res.ok) return `Ollama connection failed: HTTP ${res.status}`;
        return null;
      }

      // OpenAI-compatible providers (openai, groq)
      const baseUrl = runtime.baseUrl;
      const key = runtime.apiKey;
      if (!key) return `No API key configured for ${runtime.name}`;
      if (!baseUrl) return `No base URL configured for ${runtime.name}`;
      const res = await fetch(`${baseUrl}/models`, {
        headers: { Authorization: `Bearer ${key}` },
      });
      if (!res.ok) return `Connection failed: HTTP ${res.status}`;
      return null;
    } catch (err) {
      return err instanceof Error ? err.message : String(err);
    }
  }

  diagnose(): DiagnosticResult {
    const checks = [
      {
        name: 'Authentication',
        passed: this.isConfigured,
        message: this.isConfigured
          ? this._authMethod === 'dotenv'
            ? `Authenticated via ${LLMClient.envFilePath}`
            : this._isOAuthToken
              ? 'Authenticated via Claude Code session token'
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

  /**
   * Resolve the Anthropic client to use for a request.
   * Priority: DB default provider key → constructor-configured client.
   * Returns null only when neither is available.
   */
  private resolveClient(agentId?: string): Anthropic | null {
    if (this.providerStore) {
      const provider = this.getActiveProvider(agentId);
      if (provider && provider.id === 'anthropic' && provider.apiKey) {
        const isOAuth = provider.apiKey.startsWith('sk-ant-oat');
        if (isOAuth) {
          return new Anthropic({
            authToken: provider.apiKey,
            defaultHeaders: { 'anthropic-beta': 'oauth-2025-04-20' },
          });
        }
        return new Anthropic({ apiKey: provider.apiKey });
      }
    }
    return this.client;
  }

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

    // Inject agent soul (SOUL.md) — defines WHO the agent is
    if (request.soul) {
      parts.push('\n\n## Agent Identity\n' + request.soul);
    }

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

    // Inject current time so the agent is aware of time of day
    const now = new Date();
    parts.push(
      `\n\n## Current Time\n${now.toLocaleString()} (${Intl.DateTimeFormat().resolvedOptions().timeZone})`,
    );

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
