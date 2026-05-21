/**
 * LLM Client — LLM API with Model Routing
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
import { spawnSync } from 'node:child_process';
import Anthropic from '@anthropic-ai/sdk';
import {
  RateLimitError,
  AuthenticationError,
  BadRequestError,
  APIError,
  APIConnectionError,
} from '@anthropic-ai/sdk';
import type { ModuleBus } from '../module-bus/index.js';
import type { TokenTracker } from '../tokens/index.js';
import type { BehaviorManager } from '../behaviors/index.js';
import type { MemoryManager } from '../memory/index.js';
import type { DiagnosticResult } from '../types.js';
import type { FortTool, ToolCallLog } from '../tools/types.js';
import type { ToolExecutor } from '../tools/executor.js';
import type { LLMProviderStore, LLMProviderRuntime } from './provider-store.js';
import type { SubscriptionQuotaStore, QuotaSnapshot } from './quota-store.js';
import type { SpecialistIdentity } from '../types.js';

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
  /** When true, a gated 429 throws ModelGatedError instead of auto-falling-back.
   *  Set by the Specialist for interactive (user_chat) tasks. */
  interactive?: boolean;
  /** Forces a specific provider id for this one request, above identity/global
   *  resolution. Used to retry after the user picks a different provider. */
  providerOverride?: string;
  context?: string[];
  injectBehaviors?: boolean;
  injectMemory?: string;
  soul?: string;
  stream?: boolean;
  /** Optional tools to expose to the LLM for this request */
  tools?: FortTool[];
  /**
   * Whether to inject the agent's active goals and user profile facts into
   * the system prompt. Defaults to true when `agentId` is set and `system`
   * is not overridden — the "regular agent chat" path. Triager and Hatch
   * pass their own `system` so they opt out automatically.
   */
  injectAgentContext?: boolean;
  /**
   * When true, append the hatch-mode addendum to the system prompt. Set
   * by Specialist when the agent's `hatchedAt` is null — drives the
   * conversational onboarding flow. Composes WITH the base prompt and
   * SOUL, doesn't replace them.
   */
  hatchMode?: boolean;
}

/**
 * Minimal interface for goal lookup. Implemented by GoalsService — kept as
 * a structural type so LLMClient doesn't need to import the service module.
 */
export interface GoalsLookup {
  listForAgent(agentId: string, status?: 'active'): Array<{ id: string; title: string; status: string }>;
}

/**
 * Callback that resolves an agentId to its identity. Injected after construction
 * by Fort so the LLM client can read `identity.provider` for per-agent routing
 * without importing AgentFactory directly.
 */
export type IdentityResolver = (agentId: string) => SpecialistIdentity | null;

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
  quotaStore?: SubscriptionQuotaStore;
}

// ─── Rate Limit & Cooldown Types ────────────────────────────────────

export interface ModelCooldown {
  until: number;
  reason: string;
}

type ErrorClassification = {
  type: 'rate_limit' | 'auth' | 'bad_request' | 'overloaded' | 'connection' | 'unknown';
  retryAfterMs: number | null;
  retryable: boolean;
};

type RuntimeProvider =
  | { id: 'anthropic'; client: Anthropic; authMethod: string; isOAuth: boolean }
  | { id: 'openai'; token: string; baseUrl: string; authMethod: string; accountId?: string }
  // OpenAI-compatible chat-completions providers (grok / groq / openrouter / ollama)
  | { id: 'grok' | 'groq' | 'openrouter' | 'ollama'; token: string; baseUrl: string; authMethod: string }
  // Gemini-native (uses generateContent endpoint, different request/response shape)
  | { id: 'google'; token: string; baseUrl: string; authMethod: string };

type OpenAIResponse = {
  id?: string;
  output_text?: string;
  output?: Array<Record<string, any>>;
  usage?: {
    input_tokens?: number;
    output_tokens?: number;
    total_tokens?: number;
  };
  error?: { message?: string };
};

/** Public base URLs for OpenAI-compatible providers (also exposed for testing). */
function defaultBaseUrlFor(id: 'openai' | 'grok' | 'groq' | 'openrouter' | 'ollama' | 'google'): string {
  switch (id) {
    case 'openai':     return 'https://api.openai.com/v1';
    case 'grok':       return 'https://api.x.ai/v1';
    case 'groq':       return 'https://api.groq.com/openai/v1';
    case 'openrouter': return 'https://openrouter.ai/api/v1';
    case 'ollama':     return 'http://localhost:11434';
    case 'google':     return 'https://generativelanguage.googleapis.com';
  }
}

/** Typed HTTP error thrown by callOpenAIResponses so the retry layer can classify it. */
class OpenAIHttpError extends Error {
  constructor(
    public readonly status: number,
    public readonly headers: Headers,
    public readonly body: OpenAIResponse | undefined,
    message: string,
  ) {
    super(message);
    this.name = 'OpenAIHttpError';
  }
}

/** Result of a single OpenAI HTTP call — body plus the raw headers (used for quota tracking). */
interface OpenAICallResult {
  body: OpenAIResponse;
  headers: Headers;
}

// Graduated backoff for 429s (inspired by OpenClaw's circuit breaker)
const RATE_LIMIT_BACKOFFS_MS = [30_000, 60_000, 300_000]; // 30s, 1min, 5min
const RATE_LIMIT_MAX_RETRIES = 3;
const TIER_FALLBACK: ModelTier[] = ['powerful', 'standard', 'fast'];
const TOKEN_REFRESH_TTL_MS = 5 * 60 * 1000; // 5 minutes
const MAX_FALLBACK_DEPTH = 3;

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

const DEFAULT_OPENAI_MODELS: Record<ModelTier, ModelConfig> = {
  fast:     { tier: 'fast',     model: 'gpt-5.4-mini',     maxTokens: 2048, description: 'Fast OpenAI model for simple tasks' },
  standard: { tier: 'standard', model: 'gpt-5.4',          maxTokens: 4096, description: 'Balanced OpenAI model' },
  powerful: { tier: 'powerful', model: 'gpt-5.5',          maxTokens: 8192, description: 'Frontier OpenAI model' },
};

const DEFAULT_GROK_MODELS: Record<ModelTier, ModelConfig> = {
  fast:     { tier: 'fast',     model: 'grok-3-mini',      maxTokens: 2048, description: 'Fast xAI Grok model' },
  standard: { tier: 'standard', model: 'grok-4',           maxTokens: 4096, description: 'Standard xAI Grok model' },
  powerful: { tier: 'powerful', model: 'grok-4-heavy',     maxTokens: 8192, description: 'Frontier xAI Grok model' },
};

const DEFAULT_GROQ_MODELS: Record<ModelTier, ModelConfig> = {
  fast:     { tier: 'fast',     model: 'llama-3.1-8b-instant',     maxTokens: 2048, description: 'Fast Llama via Groq inference' },
  standard: { tier: 'standard', model: 'llama-3.3-70b-versatile',  maxTokens: 4096, description: 'Standard Llama via Groq inference' },
  powerful: { tier: 'powerful', model: 'llama-3.3-70b-versatile',  maxTokens: 8192, description: 'Powerful Llama via Groq inference' },
};

const DEFAULT_GOOGLE_MODELS: Record<ModelTier, ModelConfig> = {
  fast:     { tier: 'fast',     model: 'gemini-2.5-flash',  maxTokens: 2048, description: 'Fast Gemini model' },
  standard: { tier: 'standard', model: 'gemini-2.5-pro',    maxTokens: 4096, description: 'Standard Gemini model' },
  powerful: { tier: 'powerful', model: 'gemini-2.5-pro',    maxTokens: 8192, description: 'Frontier Gemini model' },
};

const DEFAULT_OLLAMA_MODELS: Record<ModelTier, ModelConfig> = {
  fast:     { tier: 'fast',     model: 'llama3.2:1b',       maxTokens: 2048, description: 'Fast local model via Ollama' },
  standard: { tier: 'standard', model: 'llama3.2',          maxTokens: 4096, description: 'Standard local model via Ollama' },
  powerful: { tier: 'powerful', model: 'qwen2.5-coder',     maxTokens: 8192, description: 'Coding-tuned local model via Ollama' },
};

const DEFAULT_OPENROUTER_MODELS: Record<ModelTier, ModelConfig> = {
  fast:     { tier: 'fast',     model: 'openai/gpt-5.4-mini',           maxTokens: 2048, description: 'Fast via OpenRouter' },
  standard: { tier: 'standard', model: 'anthropic/claude-sonnet-4-5',   maxTokens: 4096, description: 'Standard via OpenRouter' },
  powerful: { tier: 'powerful', model: 'anthropic/claude-opus-4-6',     maxTokens: 8192, description: 'Powerful via OpenRouter' },
};

const DEFAULT_SYSTEM_PROMPT = `You are Fort, a personal AI agent platform.

You operate in two modes and pick between them per message:

- Quick mode: terse, direct, no preamble. Answer the question or do the thing. Use it for lookups, well-formed asks, and anything where the right answer is unambiguous.
- Curious mode: propose options with brief reasoning, address the user personally using what you know about them, and ask one clarifying question when one would change the answer. Use it for decisions, design choices, planning, and anything load-bearing on the user's goals.

Pick mode from the shape and intent of each message. When genuinely in doubt, prefer curious — but don't pad a quick question with curious framing.

Override your default when a short answer would mislead. In that case, lean in: ask the one question whose answer changes everything, then answer once you have it. An extra question beats a wrong answer.

Three constraints, always:
1. No inner monologue. Don't narrate your reasoning or think out loud — state results.
2. Never stack questions. One clarifying question at a time, in your own voice, in the chat.
3. No recap. Don't restate the user's message before answering.

Speak personally. When you have context about the user (active goals, profile facts, prior work), let it shape the answer so it feels addressed to them, not generic. Don't perform that context — just use it.`;

// ─── Pricing (per 1M tokens) ────────────────────────────────────────

const PRICING: Record<string, { input: number; output: number }> = {
  // Anthropic — public pricing (per 1M tokens)
  'claude-haiku-4-5-20251001':   { input: 0.80,  output: 4.00 },
  'claude-sonnet-4-5-20250929':  { input: 3.00,  output: 15.00 },
  'claude-opus-4-6':             { input: 15.00, output: 75.00 },
  // OpenAI ChatGPT/Codex subscription — flat-rate, tracked via quota.
  'gpt-5.5':         { input: 0, output: 0 },
  'gpt-5.4':         { input: 0, output: 0 },
  'gpt-5.4-mini':    { input: 0, output: 0 },
  'gpt-5.3-codex':   { input: 0, output: 0 },
  'gpt-5.2':         { input: 0, output: 0 },
  // xAI Grok — public pricing approximations
  'grok-4-heavy':       { input: 5.00, output: 25.00 },
  'grok-4':             { input: 3.00, output: 15.00 },
  'grok-3':             { input: 1.00, output: 5.00 },
  'grok-3-mini':        { input: 0.30, output: 1.50 },
  'grok-code-fast':     { input: 0.20, output: 1.00 },
  // Groq (Llama inference) — very cheap
  'llama-3.3-70b-versatile':  { input: 0.59, output: 0.79 },
  'llama-3.1-8b-instant':     { input: 0.05, output: 0.08 },
  'mixtral-8x7b-32768':       { input: 0.24, output: 0.24 },
  // Google Gemini — public pricing
  'gemini-2.5-pro':     { input: 1.25, output: 10.00 },
  'gemini-2.5-flash':   { input: 0.10, output: 0.40 },
  'gemini-2.0-flash':   { input: 0.10, output: 0.40 },
  // Ollama — local, free
  'llama3.2':           { input: 0, output: 0 },
  'llama3.2:1b':        { input: 0, output: 0 },
  'qwen2.5-coder':      { input: 0, output: 0 },
  // OpenRouter — proxy pricing varies by routed model; approximations
  'openai/gpt-5.4-mini':            { input: 0.25, output: 2.00 },
  'openai/gpt-5.5':                 { input: 2.50, output: 10.00 },
  'anthropic/claude-sonnet-4-5':    { input: 3.00, output: 15.00 },
  'anthropic/claude-opus-4-6':      { input: 15.00, output: 75.00 },
  'google/gemini-2.5-pro':          { input: 1.25, output: 10.00 },
  'x-ai/grok-4':                    { input: 3.00, output: 15.00 },
  'meta-llama/llama-3.3-70b-instruct': { input: 0.50, output: 0.75 },
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
  private goals: GoalsLookup | null = null;
  private identityResolver: IdentityResolver | null = null;
  private providerStore: LLMProviderStore | null;
  private quotaStore: SubscriptionQuotaStore | null;

  // Stats
  private requestCount = 0;
  private totalInputTokens = 0;
  private totalOutputTokens = 0;
  private totalCostUsd = 0;
  private errorCount = 0;
  private rateLimitCount = 0;
  private startedAt = new Date();

  // Rate limit & cooldown state
  private cooldowns = new Map<string, ModelCooldown>();
  private lastTokenRefresh = 0;
  private cachedToken: string | null = null;

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
    this.quotaStore = config.quotaStore ?? null;
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
      this.client = LLMClient.createAnthropicClient(resolvedToken, this._isOAuthToken);
      this.cachedToken = resolvedToken;
      this._authMethod = config.apiKey ? 'api_key_config'
        : envFileToken ? 'dotenv'
        : 'api_key_env';
    }
    // If nothing found, client stays null — requests will return helpful errors
  }

  /**
   * Inject the goals lookup so user-facing chats see active goals as
   * system-prompt context. Called once by Fort during wiring.
   */
  setGoals(goals: GoalsLookup): void {
    this.goals = goals;
  }

  /**
   * Inject the identity resolver so per-agent provider routing can read
   * `identity.provider`. Called once by Fort during wiring, after AgentFactory exists.
   */
  setIdentityResolver(resolver: IdentityResolver): void {
    this.identityResolver = resolver;
  }

  /**
   * Create an Anthropic client with maxRetries: 0 (Fort manages retries).
   * Without this, the SDK's built-in retry (default: 2) layers on top of
   * Fort's retry loop, creating up to 9 HTTP attempts with unpredictable timing.
   */
  private static createAnthropicClient(token: string, isOAuth: boolean): Anthropic {
    if (isOAuth) {
      return new Anthropic({
        authToken: token,
        defaultHeaders: { 'anthropic-beta': 'oauth-2025-04-20' },
        maxRetries: 0,
      });
    }
    return new Anthropic({ apiKey: token, maxRetries: 0 });
  }

  private _authMethod: 'api_key_config' | 'dotenv' | 'api_key_env' | null = null;
  private _isOAuthToken = false;

  /**
   * Read the API key from ~/.fort/.env
   * The file is a simple KEY=VALUE format, one per line.
   */
  static readEnvFile(): string | null {
    return LLMClient.readEnvFileValue('ANTHROPIC_API_KEY');
  }

  static readOpenAIEnvFile(): string | null {
    return LLMClient.readEnvFileValue('OPENAI_API_KEY');
  }

  /**
   * Read any KEY=VALUE entry from ~/.fort/.env. Public so provider-specific
   * resolvers (grok/groq/google/openrouter) can share the same parser.
   */
  static readEnvFileValue(key: string): string | null {
    const envPath = join(homedir(), '.fort', '.env');
    if (!existsSync(envPath)) return null;
    try {
      const content = readFileSync(envPath, 'utf-8');
      for (const line of content.split('\n')) {
        const trimmed = line.trim();
        if (trimmed.startsWith('#') || !trimmed) continue;
        const escapedKey = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        const match = trimmed.match(new RegExp(`^${escapedKey}\\s*=\\s*["']?(.+?)["']?\\s*$`));
        if (match) return match[1];
      }
      return null;
    } catch {
      return null;
    }
  }

  /**
   * Read an OpenAI access token from Codex auth. This enables Fort to reuse an
   * active ChatGPT/Codex subscription without requiring a separate API key.
   */
  static readCodexOpenAIToken(): { accessToken: string; accountId?: string } | null {
    const authPath = join(homedir(), '.codex', 'auth.json');
    if (!existsSync(authPath)) return null;
    try {
      const parsed = JSON.parse(readFileSync(authPath, 'utf-8'));
      const accessToken = parsed?.tokens?.access_token;
      if (typeof accessToken !== 'string' || accessToken.length === 0) return null;
      const accountId = parsed?.tokens?.account_id;
      return {
        accessToken,
        accountId: typeof accountId === 'string' ? accountId : undefined,
      };
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
   * Write an Anthropic API key to ~/.fort/.env (backwards-compatible wrapper).
   */
  static writeEnvFile(apiKey: string): void {
    LLMClient.writeEnvFileValue('ANTHROPIC_API_KEY', apiKey);
  }

  /**
   * Upsert any KEY=VALUE entry in ~/.fort/.env. Creates the file with the
   * standard header if missing; replaces the line in-place if the key already
   * exists; otherwise appends. Used by `fort llm setup --<provider>` flows.
   */
  static writeEnvFileValue(key: string, value: string): void {
    const fortDir = join(homedir(), '.fort');
    const envPath = join(fortDir, '.env');
    const { mkdirSync, writeFileSync: writeFile } = require('node:fs');
    if (!existsSync(fortDir)) mkdirSync(fortDir, { recursive: true });

    const escapedKey = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const matcher = new RegExp(`^${escapedKey}\\s*=.*$`, 'm');

    let content = '';
    if (existsSync(envPath)) {
      content = readFileSync(envPath, 'utf-8');
      if (matcher.test(content)) {
        content = content.replace(matcher, `${key}=${value}`);
      } else {
        content = content.trimEnd() + `\n${key}=${value}\n`;
      }
    } else {
      content = `# Fort API Configuration\n# Generated by \`fort llm setup\`\n\n${key}=${value}\n`;
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
    // Report the auth method actually being used at runtime, not the one set
    // during constructor. resolveRuntimeProvider() picks the active provider
    // and its authMethod is the source of truth for `fort llm status`.
    const active = this.resolveRuntimeProvider();
    if (active) return active.authMethod;
    return this._authMethod ?? LLMClient.resolveOpenAIToken()?.authMethod ?? null;
  }

  /**
   * Check if the LLM client is configured and ready.
   */
  get isConfigured(): boolean {
    return this.client !== null || this.getActiveProvider() !== null || LLMClient.resolveOpenAIToken() !== null;
  }

  /**
   * Validate that the configured API key actually works.
   * Makes a minimal API call to verify authentication.
   * Returns null on success, or an error message string on failure.
   */
  async validateAuth(): Promise<string | null> {
    const active = this.resolveRuntimeProvider();
    if (active?.id === 'openai') {
      try {
        await this.callOpenAIResponses(active, {
          model: this.resolveModelForProvider(active, this.defaultTier).model,
          instructions: 'You are a connectivity check. Reply with one word.',
          input: 'hi',
          max_output_tokens: 1,
        });
        return null;
      } catch (err) {
        return `API connection error: ${err instanceof Error ? err.message : String(err)}`;
      }
    }
    if (active && (active.id === 'grok' || active.id === 'groq' || active.id === 'openrouter' || active.id === 'ollama')) {
      try {
        await this.callOpenAICompatibleChat(active, {
          model: this.resolveModelForProvider(active, this.defaultTier).model,
          messages: [{ role: 'user', content: 'hi' }],
          max_tokens: 1,
        });
        return null;
      } catch (err) {
        return `${active.id} connection error: ${err instanceof Error ? err.message : String(err)}`;
      }
    }
    if (active?.id === 'google') {
      try {
        await this.callGoogleGemini(active, {
          model: this.resolveModelForProvider(active, this.defaultTier).model,
          contents: [{ role: 'user', parts: [{ text: 'hi' }] }],
          generationConfig: { maxOutputTokens: 1 },
        });
        return null;
      } catch (err) {
        return `Google connection error: ${err instanceof Error ? err.message : String(err)}`;
      }
    }

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
   * Uses callApi() for retry-aware error handling with rate limit cooldowns
   * and automatic tier fallback.
   */
  async complete(request: LLMRequest, _fallbackDepth = 0): Promise<LLMResponse> {
    const runtime = this.resolveRuntimeProvider(request.agentId, request.providerOverride);
    if (!runtime) {
      throw new Error(
        'LLM client not configured. Set ANTHROPIC_API_KEY or OPENAI_API_KEY, add a provider, or sign in with Codex.',
      );
    }

    if (runtime.id === 'openai') {
      return this.completeOpenAI(runtime, request);
    }
    if (runtime.id === 'grok' || runtime.id === 'groq' || runtime.id === 'openrouter' || runtime.id === 'ollama') {
      return this.completeOpenAICompatible(runtime, request);
    }
    if (runtime.id === 'google') {
      return this.completeGoogle(runtime, request);
    }

    if (runtime.id !== 'anthropic') {
      throw new Error(`${(runtime as RuntimeProvider).id} generation is not implemented yet.`);
    }

    const client = runtime.client;
    const modelConfig = this.resolveModelForProvider(runtime, request.model);
    const system = await this.buildSystemPrompt(request);
    const maxTokens = request.maxTokens ?? modelConfig.maxTokens;

    const start = Date.now();

    const claudeTools: Anthropic.Tool[] | undefined =
      request.tools && request.tools.length > 0
        ? request.tools.map((t) => ({
            name: t.name,
            description: t.description,
            input_schema: t.inputSchema as Anthropic.Tool['input_schema'],
          }))
        : undefined;

    const params: Anthropic.MessageCreateParams = {
      model: modelConfig.model,
      max_tokens: maxTokens,
      system,
      messages: request.messages.map((m) => ({
        role: m.role,
        content: m.content,
      })),
      temperature: request.temperature,
      ...(claudeTools ? { tools: claudeTools } : {}),
    };

    let response: Anthropic.Message;
    try {
      response = await this.callApi(client, params, {
        modelConfig,
        request,
        fallbackDepth: _fallbackDepth,
      });
    } catch (err: any) {
      // Handle tier fallback signal from callApi
      if (err?.message === '__TIER_FALLBACK__' && err._fallbackTier) {
        return this.complete(
          { ...request, model: err._fallbackTier },
          (err._fallbackDepth ?? 0) + 1,
        );
      }
      throw err;
    }

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

    // Publish cost-tracking event
    if (request.taskId && request.agentId) {
      this.bus.publish('usage.recorded', request.agentId, {
        taskId: request.taskId,
        agentId: request.agentId,
        model: modelConfig.model,
        inputTokens,
        outputTokens,
        cacheReadTokens: (response.usage as any).cache_read_input_tokens ?? 0,
        cacheWriteTokens: (response.usage as any).cache_creation_input_tokens ?? 0,
      });
    }

    return result;
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
    const runtime = this.resolveRuntimeProvider(request.agentId, request.providerOverride);
    if (!runtime) {
      throw new Error(
        'LLM client not configured. Set ANTHROPIC_API_KEY or OPENAI_API_KEY, add a provider, or sign in with Codex.',
      );
    }

    if (runtime.id === 'openai') {
      return this.completeOpenAIWithTools(runtime, request, executor, opts);
    }
    // Tool-calling for grok/groq/google/openrouter/ollama is not yet wired.
    // Surface a clear error rather than silently falling through.
    if (runtime.id !== 'anthropic') {
      throw new Error(`Tool calling via ${(runtime as RuntimeProvider).id} is not implemented yet — use Anthropic or OpenAI for tools.`);
    }

    const client = runtime.client;

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
      let modelConfig = this.resolveModelForProvider(runtime, request.model);
      const system = await this.buildSystemPrompt(request);
      let maxTokens = request.maxTokens ?? modelConfig.maxTokens;

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

        let response: Anthropic.Message;
        try {
          response = await this.callApi(client, {
            model: modelConfig.model,
            max_tokens: maxTokens,
            system,
            messages,
            tools: claudeTools,
            temperature: request.temperature,
          }, { modelConfig, request });
        } catch (err: any) {
          // Rate limited with a lower tier available: switch models and retry
          // this same turn. The conversation + tool results so far are kept in
          // `messages`, so no progress is lost. Without this the agent's first
          // run dies on an opus rate limit even though haiku/sonnet are free.
          if (err?.message === '__TIER_FALLBACK__' && err._fallbackTier) {
            const fallback = this.resolveModelForProvider(runtime, err._fallbackTier);
            if (fallback.model !== modelConfig.model) {
              this.bus.publish('llm.tier_fallback', 'llm-client', {
                from: modelConfig.model,
                to: fallback.model,
                reason: 'rate_limit_tool_loop',
                taskId: request.taskId,
                agentId: request.agentId,
              });
              modelConfig = fallback;
              maxTokens = request.maxTokens ?? fallback.maxTokens;
              iteration--; // this turn didn't count — retry it on the new model
              continue;
            }
          }
          throw err;
        }

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

          // Publish cost-tracking event (aggregate for all tool-loop turns)
          if (request.taskId && request.agentId) {
            this.bus.publish('usage.recorded', request.agentId, {
              taskId: request.taskId,
              agentId: request.agentId,
              model: modelConfig.model,
              inputTokens: totalInputTokens,
              outputTokens: totalOutputTokens,
              cacheReadTokens: 0,
              cacheWriteTokens: 0,
            });
          }

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

      // Publish cost-tracking event (max iterations path)
      if (request.taskId && request.agentId) {
        this.bus.publish('usage.recorded', request.agentId, {
          taskId: request.taskId,
          agentId: request.agentId,
          model: modelConfig.model,
          inputTokens: totalInputTokens,
          outputTokens: totalOutputTokens,
          cacheReadTokens: 0,
          cacheWriteTokens: 0,
        });
      }

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
    const runtime = this.resolveRuntimeProvider(request.agentId, request.providerOverride);
    if (!runtime) {
      yield {
        type: 'error',
        error: 'LLM client not configured. Set ANTHROPIC_API_KEY or OPENAI_API_KEY, add a provider, or sign in with Codex.',
      };
      return;
    }

    if (runtime.id === 'openai' || runtime.id === 'grok' || runtime.id === 'groq' || runtime.id === 'openrouter' || runtime.id === 'ollama' || runtime.id === 'google') {
      try {
        const response = runtime.id === 'openai'
          ? await this.completeOpenAI(runtime, request)
          : runtime.id === 'google'
            ? await this.completeGoogle(runtime, request)
            : await this.completeOpenAICompatible(runtime, request);
        yield { type: 'text', text: response.content };
        yield { type: 'done', response };
      } catch (err) {
        this.errorCount++;
        yield { type: 'error', error: err instanceof Error ? err.message : String(err) };
      }
      return;
    }

    if (runtime.id !== 'anthropic') {
      yield { type: 'error', error: `${runtime.id} streaming is not implemented yet.` };
      return;
    }

    const client = runtime.client;

    const modelConfig = this.resolveModelForProvider(runtime, request.model);
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

      // Publish cost-tracking event
      if (request.taskId && request.agentId) {
        this.bus.publish('usage.recorded', request.agentId, {
          taskId: request.taskId,
          agentId: request.agentId,
          model: modelConfig.model,
          inputTokens,
          outputTokens,
          cacheReadTokens: (finalMessage.usage as any).cache_read_input_tokens ?? 0,
          cacheWriteTokens: (finalMessage.usage as any).cache_creation_input_tokens ?? 0,
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
      injectAgentContext?: boolean;
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
      injectAgentContext: opts?.injectAgentContext,
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
    const active = this.resolveRuntimeProvider();
    return {
      configured: this.isConfigured,
      authMethod: this.authMethod,
      defaultTier: this.defaultTier,
      requestCount: this.requestCount,
      totalInputTokens: this.totalInputTokens,
      totalOutputTokens: this.totalOutputTokens,
      totalCostUsd: this.totalCostUsd,
      errorCount: this.errorCount,
      rateLimitCount: this.rateLimitCount,
      cooldowns: this.getActiveCooldowns(),
      uptime: Date.now() - this.startedAt.getTime(),
      activeProvider: active?.id ?? null,
      models: Object.fromEntries(
        Object.entries(this.getActiveModels()).map(([tier, config]) => [
          tier,
          { model: config.model, description: config.description },
        ]),
      ),
      subscriptionQuota: this.quotaStore?.get('openai') ?? null,
    };
  }

  /**
   * Enumerate every supported provider with its current usability state.
   * Used by the dashboard SetupWizard and `fort agents create` to render
   * the Provider step.
   *
   * `usable: true` means the provider has a credential source resolvable
   * right now (env var, ~/.fort/.env, provider-store row with apiKey,
   * Codex subscription, Claude keychain, or reachable Ollama server).
   */
  async getAvailableProviders(): Promise<Array<{
    id: 'anthropic' | 'openai' | 'grok' | 'groq' | 'google' | 'ollama' | 'openrouter';
    name: string;
    usable: boolean;
    authMethod: string | null;
    models: { fast: string; standard: string; powerful: string };
    hint?: string;
  }>> {
    // Pick up any tokens written since construction (fort init's claude
    // setup-token / paste-API-key flows write to ~/.fort/.env or keychain).
    // Without this, anthropicUsable stays false until Fort restarts and the
    // CLI loops on the provider menu after a successful setup.
    this.refreshAuth();

    const tierMap = (id: 'anthropic' | 'openai' | 'grok' | 'groq' | 'google' | 'ollama' | 'openrouter') => {
      const m = id === 'anthropic' ? this.models : LLMClient.tierMapFor(id);
      return {
        fast: m!.fast.model,
        standard: m!.standard.model,
        powerful: m!.powerful.model,
      };
    };

    // Anthropic — usable if constructor wired a client OR provider-store
    // has an anthropic row with apiKey.
    const anthropicStoreRow = this.providerStore?.getProviderRuntime('anthropic') ?? null;
    const anthropicUsable = this.client !== null || !!(anthropicStoreRow && anthropicStoreRow.apiKey);
    const anthropicAuth = this.client
      ? (this._isOAuthToken ? 'claude_subscription' : this._authMethod ?? 'unknown')
      : anthropicStoreRow?.apiKey ? 'provider_store' : null;

    const openai = LLMClient.resolveOpenAIToken();
    const grok = LLMClient.resolveGrokToken();
    const groq = LLMClient.resolveGroqToken();
    const google = LLMClient.resolveGoogleToken();
    const openrouter = LLMClient.resolveOpenRouterToken();

    // Ollama — usable if local server responds (short timeout)
    let ollamaUsable = false;
    try {
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), 500);
      const res = await fetch('http://localhost:11434/api/tags', { signal: controller.signal });
      clearTimeout(timer);
      ollamaUsable = res.ok;
    } catch {
      ollamaUsable = false;
    }

    return [
      {
        id: 'anthropic',
        name: 'Anthropic',
        usable: anthropicUsable,
        authMethod: anthropicAuth,
        models: tierMap('anthropic'),
        ...(anthropicUsable ? {} : { hint: 'Run `fort llm setup` to authenticate with Claude.' }),
      },
      {
        id: 'openai',
        name: 'OpenAI',
        usable: openai !== null,
        authMethod: openai?.authMethod ?? null,
        models: tierMap('openai'),
        ...(openai ? {} : { hint: 'Run `fort llm setup --openai` to sign in with Codex.' }),
      },
      {
        id: 'grok',
        name: 'Grok (xAI)',
        usable: grok !== null,
        authMethod: grok?.authMethod ?? null,
        models: tierMap('grok'),
        ...(grok ? {} : { hint: 'Run `fort llm setup --grok` or set XAI_API_KEY in ~/.fort/.env.' }),
      },
      {
        id: 'groq',
        name: 'Groq',
        usable: groq !== null,
        authMethod: groq?.authMethod ?? null,
        models: tierMap('groq'),
        ...(groq ? {} : { hint: 'Run `fort llm setup --groq` or set GROQ_API_KEY in ~/.fort/.env.' }),
      },
      {
        id: 'google',
        name: 'Google',
        usable: google !== null,
        authMethod: google?.authMethod ?? null,
        models: tierMap('google'),
        ...(google ? {} : { hint: 'Run `fort llm setup --google` or set GEMINI_API_KEY in ~/.fort/.env.' }),
      },
      {
        id: 'ollama',
        name: 'Ollama',
        usable: ollamaUsable,
        authMethod: ollamaUsable ? 'ollama_local' : null,
        models: tierMap('ollama'),
        ...(ollamaUsable ? {} : { hint: 'Start the Ollama server: `ollama serve` (default localhost:11434).' }),
      },
      {
        id: 'openrouter',
        name: 'OpenRouter',
        usable: openrouter !== null,
        authMethod: openrouter?.authMethod ?? null,
        models: tierMap('openrouter'),
        ...(openrouter ? {} : { hint: 'Run `fort llm setup --openrouter` or set OPENROUTER_API_KEY in ~/.fort/.env.' }),
      },
    ];
  }

  /**
   * Get the model tier mapping for the currently active provider. Falls back
   * to the Anthropic defaults when no provider is resolved.
   */
  getActiveModels(): Record<ModelTier, ModelConfig> {
    const active = this.resolveRuntimeProvider();
    if (active) {
      const tierMap = LLMClient.tierMapFor(active.id);
      if (tierMap) return { ...tierMap };
    }
    return { ...this.models };
  }

  /**
   * Get available model configurations.
   */
  getModels(): Record<ModelTier, ModelConfig> {
    return { ...this.models };
  }

  /**
   * Get the active provider for a given agent (or the global default).
   * Resolution order: agent's `identity.provider` → global default → null.
   * The runtime caller falls through to ambient env credentials if this returns null.
   */
  getActiveProvider(agentId?: string): LLMProviderRuntime | null {
    if (!this.providerStore) return null;
    if (agentId && this.identityResolver) {
      const identity = this.identityResolver(agentId);
      if (identity?.provider) {
        const rt = this.providerStore.getProviderRuntime(identity.provider);
        if (rt) return rt;
      }
    }
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
        const testClient = LLMClient.createAnthropicClient(key, isOAuth);
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
      const key = runtime.apiKey ?? (providerId === 'openai' ? LLMClient.resolveOpenAIToken()?.token : null);
      if (!key) return `No API key or Codex OpenAI subscription token configured for ${runtime.name}`;
      if (!baseUrl) return `No base URL configured for ${runtime.name}`;
      if (providerId === 'openai') {
        const codex = !runtime.apiKey ? LLMClient.resolveOpenAIToken() : null;
        await this.callOpenAIResponses({
          id: 'openai',
          token: key,
          baseUrl,
          authMethod: runtime.apiKey ? 'provider_store' : (codex?.authMethod ?? 'openai'),
          accountId: codex?.accountId,
        }, {
          model: runtime.defaultModel || 'gpt-5.4-mini',
          input: 'hi',
          max_output_tokens: 1,
        });
        return null;
      }
      const res = await fetch(`${baseUrl.replace(/\/$/, '')}/models`, {
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
          ? this.authMethod === 'codex_subscription'
            ? 'Authenticated via active Codex/OpenAI subscription'
            : this.authMethod === 'openai_dotenv'
              ? `Authenticated via OPENAI_API_KEY in ${LLMClient.envFilePath}`
              : this.authMethod === 'openai_api_key_env'
                ? 'Authenticated via OPENAI_API_KEY environment variable'
                : this.authMethod === 'provider_store'
                  ? 'Authenticated via stored provider key'
                  : this.authMethod === 'dotenv'
                    ? `Authenticated via ${LLMClient.envFilePath}`
                    : this._isOAuthToken
                      ? 'Authenticated via Claude Code session token'
                      : this.authMethod === 'api_key_config'
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
        message: `${this.defaultTier} → ${this.getActiveModels()[this.defaultTier].model}`,
      },
      {
        name: 'Rate limiting',
        passed: this.rateLimitCount === 0 || Object.keys(this.getActiveCooldowns()).length === 0,
        message: this.rateLimitCount === 0
          ? 'No rate limits encountered'
          : `${this.rateLimitCount} rate limit(s), ${Object.keys(this.getActiveCooldowns()).length} model(s) in cooldown`,
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

  // ── Error Classification ──────────────────────────────────────────

  /**
   * Classify an API error to determine retry strategy.
   * Parses Retry-After headers for rate limits, distinguishes auth errors
   * (potentially fixable via token refresh) from permanent failures.
   */
  private classifyError(err: unknown): ErrorClassification {
    if (err instanceof RateLimitError) {
      let retryAfterMs: number | null = null;
      try {
        // SDK exposes headers on the error object
        const headers = (err as any).headers;
        if (headers) {
          // Prefer retry-after-ms (milliseconds), then retry-after (seconds)
          // Headers may be a plain object or a Headers-like class with .get()
          const getHeader = (name: string): string | null | undefined => {
            if (typeof headers.get === 'function') return headers.get(name);
            return headers[name];
          };
          const msHeader = getHeader('retry-after-ms');
          const secHeader = getHeader('retry-after');
          if (msHeader) {
            retryAfterMs = parseInt(String(msHeader), 10);
          } else if (secHeader) {
            const secs = parseFloat(String(secHeader));
            if (!isNaN(secs)) retryAfterMs = secs * 1000;
          }
        }
      } catch {
        // Header parsing failed — use default backoff
      }
      return { type: 'rate_limit', retryAfterMs, retryable: true };
    }

    if (err instanceof AuthenticationError) {
      // OAuth tokens can be refreshed; API keys cannot
      return { type: 'auth', retryAfterMs: null, retryable: this._isOAuthToken };
    }

    if (err instanceof BadRequestError) {
      return { type: 'bad_request', retryAfterMs: null, retryable: false };
    }

    if (err instanceof APIConnectionError) {
      return { type: 'connection', retryAfterMs: null, retryable: true };
    }

    if (err instanceof APIError && (err as any).status >= 500) {
      return { type: 'overloaded', retryAfterMs: null, retryable: true };
    }

    return { type: 'unknown', retryAfterMs: null, retryable: false };
  }

  // ── Model Cooldown Management ─────────────────────────────────────

  /**
   * Put a model into cooldown. Publishes llm.cooldown event.
   */
  private setCooldown(model: string, durationMs: number, reason: string): void {
    const until = Date.now() + durationMs;
    this.cooldowns.set(model, { until, reason });
    this.bus.publish('llm.cooldown', 'llm-client', {
      model,
      durationMs,
      reason,
      until,
    });
  }

  /**
   * Check if a model is currently in cooldown. Cleans up expired entries.
   */
  private isInCooldown(model: string): boolean {
    const entry = this.cooldowns.get(model);
    if (!entry) return false;
    if (Date.now() >= entry.until) {
      this.cooldowns.delete(model);
      return false;
    }
    return true;
  }

  /**
   * Walk the tier fallback chain downward from the given tier.
   * Returns the first available tier whose model is NOT in cooldown, or null.
   */
  private getNextAvailableTier(currentTier: ModelTier): ModelTier | null {
    const idx = TIER_FALLBACK.indexOf(currentTier);
    if (idx < 0) return null;
    // Only fall DOWN (never up to more expensive models)
    for (let i = idx + 1; i < TIER_FALLBACK.length; i++) {
      const tier = TIER_FALLBACK[i];
      if (!this.isInCooldown(this.models[tier].model)) {
        return tier;
      }
    }
    return null;
  }

  /**
   * Get active cooldowns (for stats/diagnostics).
   */
  private getActiveCooldowns(): Record<string, ModelCooldown> {
    const now = Date.now();
    const active: Record<string, ModelCooldown> = {};
    for (const [model, cd] of this.cooldowns) {
      if (cd.until > now) {
        active[model] = cd;
      } else {
        this.cooldowns.delete(model);
      }
    }
    return active;
  }

  // ── Token Refresh ─────────────────────────────────────────────────

  /**
   * Re-read token from disk if TTL expired. Reinitializes client if token changed.
   * Returns true if the token was refreshed.
   */
  private maybeRefreshToken(): boolean {
    if (Date.now() - this.lastTokenRefresh < TOKEN_REFRESH_TTL_MS) {
      return false;
    }
    this.lastTokenRefresh = Date.now();

    const freshToken = LLMClient.readEnvFile() || LLMClient.readKeychainToken();
    if (!freshToken || freshToken === this.cachedToken) {
      return false;
    }

    this.cachedToken = freshToken;
    this._isOAuthToken = freshToken.startsWith('sk-ant-oat');
    this.client = LLMClient.createAnthropicClient(freshToken, this._isOAuthToken);
    return true;
  }

  /**
   * Force-refresh the Anthropic client from ambient credentials, bypassing the
   * TTL guard. Called from the CLI/portal after `fort init`'s provider setup
   * writes a fresh token to ~/.fort/.env so `getAvailableProviders()` sees it
   * without restarting Fort.
   */
  refreshAuth(): boolean {
    this.lastTokenRefresh = 0;
    return this.maybeRefreshToken();
  }

  // ── Retry-Aware API Call ──────────────────────────────────────────

  /**
   * Provider-agnostic retry wrapper.
   * - 429 (rate limit): graduated backoff (30s/1min/5min) + model cooldown + tier fallback
   * - 401 (auth): caller-provided refresh, then retry
   * - 400 (bad request) with Anthropic OAuth: fall back to fast tier
   * - 5xx (overloaded): short backoff (1s/2s/10s)
   * - Other: throw immediately
   *
   * `doCall` is the actual API request. `classify` maps thrown errors to
   * an ErrorClassification. `onAuthRefresh` is called on retryable 401s
   * and should return true if it refreshed credentials (caller's closure
   * is responsible for picking them up on the next doCall).
   */
  private async callWithRetry<T>(
    doCall: () => Promise<T>,
    classify: (err: unknown) => ErrorClassification,
    context: {
      modelConfig: ModelConfig;
      request: LLMRequest;
      fallbackDepth?: number;
      onAuthRefresh?: () => boolean;
    },
  ): Promise<T> {
    const { modelConfig, request } = context;
    const fallbackDepth = context.fallbackDepth ?? 0;

    // Check cooldown before attempting the call
    if (this.isInCooldown(modelConfig.model)) {
      const fallbackTier = this.getNextAvailableTier(modelConfig.tier);
      if (fallbackTier && fallbackDepth < MAX_FALLBACK_DEPTH) {
        this.bus.publish('llm.retry', 'llm-client', {
          attempt: 'cooldown-fallback',
          error: `Model "${modelConfig.model}" in cooldown, falling back to ${fallbackTier}`,
          model: this.models[fallbackTier].model,
        });
        throw Object.assign(new Error('__TIER_FALLBACK__'), {
          _fallbackTier: fallbackTier,
          _fallbackDepth: fallbackDepth,
        });
      }
      const cd = this.cooldowns.get(modelConfig.model);
      const waitSec = cd ? Math.ceil((cd.until - Date.now()) / 1000) : '?';
      throw new Error(
        `All models are rate-limited. "${modelConfig.model}" in cooldown for ~${waitSec}s. Try again later.`,
      );
    }

    let rateLimitAttempts = 0;
    let generalAttempts = 0;
    let authRefreshAttempted = false;

    for (;;) {
      try {
        return await doCall();
      } catch (err) {
        const classified = classify(err);
        this.errorCount++;

        // ── Rate limit (429) ──
        if (classified.type === 'rate_limit') {
          rateLimitAttempts++;
          this.rateLimitCount++;

          const backoffMs = classified.retryAfterMs
            ?? RATE_LIMIT_BACKOFFS_MS[Math.min(rateLimitAttempts - 1, RATE_LIMIT_BACKOFFS_MS.length - 1)];

          this.setCooldown(modelConfig.model, backoffMs, 'rate_limit');

          this.bus.publish('llm.rate_limited', 'llm-client', {
            model: modelConfig.model,
            tier: modelConfig.tier,
            backoffMs,
            retryAfterMs: classified.retryAfterMs,
            attempt: rateLimitAttempts,
          });

          // Try tier fallback immediately (no sleep needed)
          const fallbackTier = this.getNextAvailableTier(modelConfig.tier);
          if (fallbackTier && fallbackDepth < MAX_FALLBACK_DEPTH) {
            this.bus.publish('llm.retry', 'llm-client', {
              attempt: 'rate-limit-fallback',
              error: `Rate limited on "${modelConfig.model}", falling back to ${fallbackTier}`,
              model: this.models[fallbackTier].model,
            });
            throw Object.assign(new Error('__TIER_FALLBACK__'), {
              _fallbackTier: fallbackTier,
              _fallbackDepth: fallbackDepth,
            });
          }

          if (rateLimitAttempts < RATE_LIMIT_MAX_RETRIES) {
            await new Promise((resolve) => setTimeout(resolve, backoffMs));
            this.bus.publish('llm.retry', 'llm-client', {
              attempt: rateLimitAttempts,
              error: `Rate limited, retrying after ${backoffMs}ms`,
              model: modelConfig.model,
            });
            continue;
          }

          this.bus.publish('llm.error', 'llm-client', {
            error: `Rate limited after ${rateLimitAttempts} retries`,
            model: modelConfig.model,
            taskId: request.taskId,
          });
          throw err;
        }

        // ── Auth error (401) — caller may attempt a refresh ──
        if (classified.type === 'auth' && classified.retryable && !authRefreshAttempted) {
          authRefreshAttempted = true;
          if (context.onAuthRefresh?.()) {
            this.bus.publish('llm.retry', 'llm-client', {
              attempt: 'token-refresh',
              error: 'Auth failed, retrying with refreshed token',
              model: modelConfig.model,
            });
            continue;
          }
          throw err;
        }

        // ── Bad request (400) with OAuth — Anthropic-specific tier fallback ──
        if (classified.type === 'bad_request' && this._isOAuthToken) {
          if (modelConfig.model !== this.models.fast.model && fallbackDepth < MAX_FALLBACK_DEPTH) {
            this.bus.publish('llm.retry', 'llm-client', {
              attempt: 'oauth-model-fallback',
              error: `Model "${modelConfig.model}" unavailable on subscription, falling back to fast`,
              model: this.models.fast.model,
            });
            throw Object.assign(new Error('__TIER_FALLBACK__'), {
              _fallbackTier: 'fast' as ModelTier,
              _fallbackDepth: fallbackDepth,
            });
          }
          throw err;
        }

        // ── Overloaded / connection — short backoff ──
        if (classified.retryable && generalAttempts < this.maxRetries) {
          generalAttempts++;
          const delay = Math.min(1000 * Math.pow(2, generalAttempts - 1), 10000);
          await new Promise((resolve) => setTimeout(resolve, delay));

          this.bus.publish('llm.retry', 'llm-client', {
            attempt: generalAttempts,
            error: err instanceof Error ? err.message : String(err),
            model: modelConfig.model,
          });
          continue;
        }

        // ── Non-retryable or retries exhausted ──
        this.bus.publish('llm.error', 'llm-client', {
          error: err instanceof Error ? err.message : String(err),
          model: modelConfig.model,
          taskId: request.taskId,
        });
        throw err;
      }
    }
  }

  /**
   * Anthropic-specific wrapper around callWithRetry. Maintains a mutable
   * `currentClient` reference so a 401 refresh swaps in the fresh client
   * for the next attempt without leaving callWithRetry.
   */
  private async callApi(
    client: Anthropic,
    params: Anthropic.MessageCreateParams,
    context: { modelConfig: ModelConfig; request: LLMRequest; fallbackDepth?: number },
  ): Promise<Anthropic.Message> {
    let currentClient = client;
    return this.callWithRetry<Anthropic.Message>(
      () => currentClient.messages.create(params) as Promise<Anthropic.Message>,
      (err) => this.classifyError(err),
      {
        ...context,
        onAuthRefresh: () => {
          if (this.maybeRefreshToken()) {
            const fresh = this.resolveClient(context.request.agentId, context.request.providerOverride);
            if (fresh) {
              currentClient = fresh;
              return true;
            }
          }
          return false;
        },
      },
    );
  }

  /**
   * Classify an OpenAI HTTP error (from OpenAIHttpError) for the retry layer.
   * Mirrors classifyError() but operates on raw HTTP status + headers, since the
   * OpenAI path uses fetch() rather than an SDK with typed exceptions.
   */
  private classifyOpenAIError(err: unknown, currentAuthMethod?: string): ErrorClassification {
    if (!(err instanceof OpenAIHttpError)) {
      // Network failures (fetch threw) — treat as retryable connection error
      return { type: 'connection', retryAfterMs: null, retryable: true };
    }

    if (err.status === 429) {
      let retryAfterMs: number | null = null;
      const msHeader = err.headers.get('retry-after-ms');
      const secHeader = err.headers.get('retry-after');
      if (msHeader) {
        const ms = parseInt(msHeader, 10);
        if (!isNaN(ms)) retryAfterMs = ms;
      } else if (secHeader) {
        const secs = parseFloat(secHeader);
        if (!isNaN(secs)) retryAfterMs = secs * 1000;
      }
      return { type: 'rate_limit', retryAfterMs, retryable: true };
    }

    if (err.status === 401) {
      // Only Codex subscription tokens can be refreshed; raw API keys cannot.
      return { type: 'auth', retryAfterMs: null, retryable: currentAuthMethod === 'codex_subscription' };
    }

    if (err.status >= 500) {
      return { type: 'overloaded', retryAfterMs: null, retryable: true };
    }

    if (err.status >= 400) {
      return { type: 'bad_request', retryAfterMs: null, retryable: false };
    }

    return { type: 'unknown', retryAfterMs: null, retryable: false };
  }

  /**
   * Resolve the Anthropic client to use for a request.
   * Priority: DB default provider key → constructor-configured client.
   * Returns null only when neither is available.
   */
  private resolveClient(agentId?: string, providerOverride?: string): Anthropic | null {
    const runtime = this.resolveRuntimeProvider(agentId, providerOverride);
    return runtime?.id === 'anthropic' ? runtime.client : null;
  }

  private resolveRuntimeProvider(agentId?: string, providerOverride?: string): RuntimeProvider | null {
    // Explicit per-request override wins (user picked a provider in the choice gate).
    if (providerOverride && this.providerStore) {
      const rt = this.providerStore.getProviderRuntime(providerOverride);
      if (rt) {
        const built = this.buildRuntimeProviderFromStore(rt);
        if (built) return built;
      }
    }

    if (this.providerStore) {
      const provider = this.getActiveProvider(agentId);
      if (provider) {
        const fromStore = this.buildRuntimeProviderFromStore(provider);
        if (fromStore) return fromStore;
      }
    }

    // Per-agent ambient fallback: if the agent's identity asks for a specific
    // provider and the provider store has no row, honor the preference using
    // ambient credentials for THAT provider. Without this, the user picks
    // "Anthropic" in `fort init` but routing falls through to ambient OpenAI
    // (which lands below) — the wrong subscription gets charged.
    if (agentId && this.identityResolver) {
      const identity = this.identityResolver(agentId);
      if (identity?.provider === 'anthropic' && this.client) {
        return { id: 'anthropic', client: this.client, authMethod: this._authMethod ?? 'unknown', isOAuth: this._isOAuthToken };
      }
      if (identity?.provider === 'openai') {
        const openai = LLMClient.resolveOpenAIToken();
        if (openai) {
          return { id: 'openai', token: openai.token, baseUrl: 'https://api.openai.com/v1', authMethod: openai.authMethod, accountId: openai.accountId };
        }
      }
    }

    // Fallback to ambient credentials: prefer OpenAI Codex subscription /
    // OPENAI_API_KEY first (the most common Fort default), then Anthropic.
    const openai = LLMClient.resolveOpenAIToken();
    if (openai) {
      return {
        id: 'openai',
        token: openai.token,
        baseUrl: 'https://api.openai.com/v1',
        authMethod: openai.authMethod,
        accountId: openai.accountId,
      };
    }

    if (this.client) {
      return {
        id: 'anthropic',
        client: this.client,
        authMethod: this._authMethod ?? 'unknown',
        isOAuth: this._isOAuthToken,
      };
    }

    return null;
  }

  /**
   * Map an LLMProviderRuntime row to a RuntimeProvider. The row's apiKey takes
   * precedence; if absent, falls back to per-provider ambient credentials
   * (Codex subscription for OpenAI, env / .env for the OpenAI-compatible
   * providers and Google).
   */
  private buildRuntimeProviderFromStore(provider: LLMProviderRuntime): RuntimeProvider | null {
    if (provider.id === 'anthropic') {
      if (!provider.apiKey) return null;
      const isOAuth = provider.apiKey.startsWith('sk-ant-oat');
      return {
        id: 'anthropic',
        client: LLMClient.createAnthropicClient(provider.apiKey, isOAuth),
        authMethod: 'provider_store',
        isOAuth,
      };
    }

    if (provider.id === 'openai') {
      const resolved = provider.apiKey
        ? { token: provider.apiKey, authMethod: 'provider_store', accountId: undefined as string | undefined }
        : LLMClient.resolveOpenAIToken();
      if (!resolved) return null;
      return {
        id: 'openai',
        token: resolved.token,
        baseUrl: provider.baseUrl ?? 'https://api.openai.com/v1',
        authMethod: resolved.authMethod,
        accountId: resolved.accountId,
      };
    }

    // OpenAI-compatible chat-completions providers
    if (provider.id === 'grok' || provider.id === 'groq' || provider.id === 'openrouter') {
      const ambient = provider.id === 'grok'       ? LLMClient.resolveGrokToken()
                    : provider.id === 'groq'       ? LLMClient.resolveGroqToken()
                    :                                 LLMClient.resolveOpenRouterToken();
      const resolved = provider.apiKey
        ? { token: provider.apiKey, authMethod: 'provider_store' }
        : ambient;
      if (!resolved) return null;
      return {
        id: provider.id,
        token: resolved.token,
        baseUrl: provider.baseUrl ?? defaultBaseUrlFor(provider.id),
        authMethod: resolved.authMethod,
      };
    }

    if (provider.id === 'google') {
      const ambient = LLMClient.resolveGoogleToken();
      const resolved = provider.apiKey
        ? { token: provider.apiKey, authMethod: 'provider_store' }
        : ambient;
      if (!resolved) return null;
      return {
        id: 'google',
        token: resolved.token,
        baseUrl: provider.baseUrl ?? 'https://generativelanguage.googleapis.com',
        authMethod: resolved.authMethod,
      };
    }

    if (provider.id === 'ollama') {
      return {
        id: 'ollama',
        token: '', // Ollama has no auth — placeholder for the union shape
        baseUrl: provider.baseUrl ?? 'http://localhost:11434',
        authMethod: 'ollama_local',
      };
    }

    return null;
  }

  private static resolveOpenAIToken(): { token: string; authMethod: string; accountId?: string } | null {
    if (process.env.OPENAI_API_KEY) {
      return { token: process.env.OPENAI_API_KEY, authMethod: 'openai_api_key_env' };
    }
    const envFileToken = LLMClient.readOpenAIEnvFile();
    if (envFileToken) {
      return { token: envFileToken, authMethod: 'openai_dotenv' };
    }
    const codex = LLMClient.readCodexOpenAIToken();
    if (codex) {
      return { token: codex.accessToken, authMethod: 'codex_subscription', accountId: codex.accountId };
    }
    return null;
  }

  /**
   * Generic credential resolver for OpenAI-compatible providers. Walks
   * process.env[envVar] → ~/.fort/.env → null. Used by Grok, Groq, Google,
   * and OpenRouter.
   */
  static resolveSimpleProviderToken(
    envVar: string,
    authMethodLabel: string,
  ): { token: string; authMethod: string } | null {
    const fromEnv = process.env[envVar];
    if (fromEnv) return { token: fromEnv, authMethod: `${authMethodLabel}_env` };
    const fromFile = LLMClient.readEnvFileValue(envVar);
    if (fromFile) return { token: fromFile, authMethod: `${authMethodLabel}_dotenv` };
    return null;
  }

  static resolveGrokToken()       { return LLMClient.resolveSimpleProviderToken('XAI_API_KEY', 'grok'); }
  static resolveGroqToken()       { return LLMClient.resolveSimpleProviderToken('GROQ_API_KEY', 'groq'); }
  static resolveOpenRouterToken() { return LLMClient.resolveSimpleProviderToken('OPENROUTER_API_KEY', 'openrouter'); }
  /** Google accepts GEMINI_API_KEY or GOOGLE_API_KEY. Checks both env vars. */
  static resolveGoogleToken(): { token: string; authMethod: string } | null {
    if (process.env.GEMINI_API_KEY) return { token: process.env.GEMINI_API_KEY, authMethod: 'google_env' };
    if (process.env.GOOGLE_API_KEY) return { token: process.env.GOOGLE_API_KEY, authMethod: 'google_env' };
    const fromFile = LLMClient.readEnvFileValue('GEMINI_API_KEY') ?? LLMClient.readEnvFileValue('GOOGLE_API_KEY');
    if (fromFile) return { token: fromFile, authMethod: 'google_dotenv' };
    return null;
  }

  /** Tier-model map for a given provider id. */
  private static tierMapFor(id: RuntimeProvider['id']): Record<ModelTier, ModelConfig> | null {
    switch (id) {
      case 'openai':     return DEFAULT_OPENAI_MODELS;
      case 'grok':       return DEFAULT_GROK_MODELS;
      case 'groq':       return DEFAULT_GROQ_MODELS;
      case 'google':     return DEFAULT_GOOGLE_MODELS;
      case 'ollama':     return DEFAULT_OLLAMA_MODELS;
      case 'openrouter': return DEFAULT_OPENROUTER_MODELS;
      default:           return null; // anthropic uses this.models
    }
  }

  private resolveModelForProvider(provider: RuntimeProvider, modelSpec?: ModelTier | string): ModelConfig {
    const tierMap = LLMClient.tierMapFor(provider.id);
    if (tierMap && (!modelSpec || modelSpec in tierMap)) {
      const tier = (modelSpec ?? this.defaultTier) as ModelTier;
      return tierMap[tier];
    }
    return this.resolveModel(modelSpec);
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

    // Inject the agent's active goals and user profile facts on the
    // regular agent-chat path. The model uses these to address the user
    // personally — answers feel "shaped by who you are" instead of generic.
    // Triager / Hatch / classification calls pass `system` themselves;
    // those don't get this context. Tests / generic calls without agentId
    // also skip it. Callers can force via `injectAgentContext`.
    const wantAgentContext =
      request.injectAgentContext ?? (!!request.agentId && !request.system);
    if (wantAgentContext) {
      if (this.goals && request.agentId) {
        const active = this.goals.listForAgent(request.agentId, 'active');
        if (active.length > 0) {
          parts.push(
            '\n\n## Active Goals\nThe user is working toward:\n' +
              active.map((g) => `- ${g.title}`).join('\n'),
          );
        }
      }
      if (this.memory) {
        const profile = this.memory.search({ nodeType: 'profile', limit: 10 });
        if (profile.nodes.length > 0) {
          parts.push(
            '\n\n## About the User\n' +
              profile.nodes.map((n) => `- ${n.label}`).join('\n'),
          );
        }
      }
    }

    // Hatch addendum — drives the first conversation with a new agent.
    // Composed AFTER goals/profile so the model sees them while
    // having the getting-to-know-you conversation (it may already have
    // started capturing facts).
    if (request.hatchMode) {
      // Imported lazily so this file doesn't depend on the services dir.
      const { HATCH_SYSTEM_ADDENDUM } = await import('../services/hatch-prompt.js');
      parts.push('\n\n' + HATCH_SYSTEM_ADDENDUM);
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

  /**
   * Parse subscription rate-limit headers into a QuotaSnapshot.
   *
   * ChatGPT's Codex backend reports quota as percent-used in two windows
   * (primary ~5h, secondary ~1 week). We surface the primary window as the
   * standard remaining/limit/used fields (limit=100, used=percent).
   *
   * Falls back to OpenAI API-style `x-ratelimit-*` headers for non-Codex
   * paths. Returns null if no recognised quota header is present.
   */
  private static parseSubscriptionQuota(headers: Headers, providerId: string): QuotaSnapshot | null {
    const get = (name: string): string | null => headers.get(name);
    const firstNumber = (...names: string[]): number | null => {
      for (const n of names) {
        const v = get(n);
        if (v === null || v === undefined) continue;
        const num = parseFloat(v);
        if (!isNaN(num)) return num;
      }
      return null;
    };
    const parseReset = (raw: string | null): string | null => {
      if (!raw) return null;
      const asNumber = parseFloat(raw);
      if (!isNaN(asNumber)) {
        const ms = asNumber > 1_000_000_000 ? asNumber * 1000 : Date.now() + asNumber * 1000;
        return new Date(ms).toISOString();
      }
      try { return new Date(raw).toISOString(); } catch { return null; }
    };

    let planType: string | null = get('x-codex-plan-type') ?? get('x-codex-active-limit');
    let remaining: number | null = null;
    let used: number | null = null;
    let limit: number | null = null;
    let windowLabel: string | null = null;
    let resetAt: string | null = null;

    // ChatGPT/Codex backend: percent-based primary window.
    const primaryUsedPct = firstNumber('x-codex-primary-used-percent');
    if (primaryUsedPct !== null) {
      used = primaryUsedPct;
      limit = 100;
      remaining = Math.max(0, 100 - primaryUsedPct);
      resetAt =
        parseReset(get('x-codex-primary-reset-at')) ??
        parseReset(get('x-codex-primary-reset-after-seconds'));
      const windowMins = firstNumber('x-codex-primary-window-minutes');
      if (windowMins !== null) {
        windowLabel = windowMins >= 60 && windowMins % 60 === 0
          ? `${windowMins / 60}h`
          : `${windowMins}m`;
      }
    } else {
      // OpenAI API style: count-based with separate remaining/limit.
      remaining = firstNumber(
        'x-codex-remaining-queries',
        'x-ratelimit-remaining-requests',
        'x-ratelimit-remaining',
      );
      limit = firstNumber(
        'x-codex-limit-queries',
        'x-ratelimit-limit-requests',
        'x-ratelimit-limit',
      );
      used = firstNumber('x-codex-used-queries', 'x-ratelimit-used-requests');
      resetAt = parseReset(
        get('x-codex-reset') ??
          get('x-ratelimit-reset-requests') ??
          get('x-ratelimit-reset'),
      );
      windowLabel = get('x-codex-window') ?? get('x-ratelimit-window');
    }

    // Capture every x-codex-* / x-ratelimit-* header for debugging.
    const rawHeaders: Record<string, string> = {};
    headers.forEach((value, key) => {
      const k = key.toLowerCase();
      if (k.startsWith('x-ratelimit-') || k.startsWith('x-codex-') || k === 'retry-after') {
        rawHeaders[k] = value;
      }
    });

    if (
      remaining === null && limit === null && used === null &&
      resetAt === null && planType === null
    ) {
      return null;
    }

    return {
      providerId,
      planType,
      remaining,
      used,
      limit,
      windowLabel,
      resetAt,
      rawHeaders,
      updatedAt: '',
    };
  }

  /**
   * Capture subscription quota from a successful OpenAI response and publish
   * `llm.subscription_quota` for any dashboard subscribers. Skipped when no
   * quota store is configured (tests, headless usage).
   */
  private observeOpenAIHeaders(
    provider: RuntimeProvider & { id: 'openai' },
    headers: Headers,
  ): void {
    if (!this.quotaStore) return;
    const snapshot = LLMClient.parseSubscriptionQuota(headers, provider.id);
    if (!snapshot) return;
    const stored = this.quotaStore.set(snapshot);
    this.bus.publish('llm.subscription_quota', 'llm-client', stored);
  }

  /**
   * Refresh a Codex CLI access token (when the subscription token has expired).
   * Best-effort: asks the codex CLI to refresh, then re-reads ~/.codex/auth.json.
   * Returns true when the cached token actually changed.
   */
  private maybeRefreshCodexToken(): boolean {
    if (Date.now() - this.lastTokenRefresh < TOKEN_REFRESH_TTL_MS) return false;
    this.lastTokenRefresh = Date.now();

    // Ask Codex CLI to refresh — uses separate args, no shell interpolation.
    try {
      spawnSync('codex', ['auth', 'refresh'], { stdio: 'pipe', timeout: 10_000 });
    } catch {
      // Codex CLI may auto-refresh in the background; re-reading the file is enough.
    }

    const fresh = LLMClient.readCodexOpenAIToken();
    if (!fresh) return false;
    if (fresh.accessToken === this.cachedToken) return false;
    this.cachedToken = fresh.accessToken;
    return true;
  }

  private async completeOpenAI(
    provider: RuntimeProvider & { id: 'openai' },
    request: LLMRequest,
    fallbackDepth = 0,
  ): Promise<LLMResponse> {
    const modelConfig = this.resolveModelForProvider(provider, request.model);
    const system = await this.buildSystemPrompt(request);
    const maxTokens = request.maxTokens ?? modelConfig.maxTokens;
    const start = Date.now();

    let currentProvider = provider;
    let result: OpenAICallResult;
    try {
      result = await this.callWithRetry<OpenAICallResult>(
        () => this.callOpenAIResponses(currentProvider, {
          model: modelConfig.model,
          instructions: system,
          input: request.messages.map((m) => ({ role: m.role, content: m.content })),
          max_output_tokens: maxTokens,
          temperature: request.temperature,
        }),
        (err) => this.classifyOpenAIError(err, currentProvider.authMethod),
        {
          modelConfig,
          request,
          fallbackDepth,
          onAuthRefresh: () => {
            if (currentProvider.authMethod === 'codex_subscription' && this.maybeRefreshCodexToken()) {
              const fresh = this.resolveRuntimeProvider(request.agentId, request.providerOverride);
              if (fresh && (fresh.id === 'openai')) {
                currentProvider = fresh;
                return true;
              }
            }
            return false;
          },
        },
      );
    } catch (err: any) {
      if (err?.message === '__TIER_FALLBACK__' && err._fallbackTier) {
        return this.completeOpenAI(
          provider,
          { ...request, model: err._fallbackTier },
          (err._fallbackDepth ?? 0) + 1,
        );
      }
      throw err;
    }

    this.observeOpenAIHeaders(provider, result.headers);
    return this.recordOpenAIResult(result.body, modelConfig, request, Date.now() - start, 'llm_client');
  }

  private async completeOpenAIWithTools(
    provider: RuntimeProvider & { id: 'openai' },
    request: LLMRequest & { tools: FortTool[] },
    executor: ToolExecutor,
    opts: { maxIterations?: number } = {},
  ): Promise<LLMToolsResponse> {
    const MAX_ITERATIONS = opts.maxIterations ?? 10;
    const toolCallLog: ToolCallLog[] = [];
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
      let modelConfig = this.resolveModelForProvider(provider, request.model);
      const system = await this.buildSystemPrompt(request);
      let maxTokens = request.maxTokens ?? modelConfig.maxTokens;
      const toolMap = new Map<string, FortTool>(request.tools.map((t) => [t.name, t]));
      const input: any[] = request.messages.map((m) => ({ role: m.role, content: m.content }));
      const openaiTools = request.tools.map((t) => ({
        type: 'function',
        name: t.name,
        description: t.description,
        parameters: t.inputSchema,
      }));

      let totalInputTokens = 0;
      let totalOutputTokens = 0;
      let totalCostUsd = 0;
      const start = Date.now();

      let currentProvider = provider;
      for (let iteration = 1; iteration <= MAX_ITERATIONS; iteration++) {
        let iterCall: OpenAICallResult;
        try {
          iterCall = await this.callWithRetry<OpenAICallResult>(
            () => this.callOpenAIResponses(currentProvider, {
              model: modelConfig.model,
              instructions: system,
              input,
              tools: openaiTools,
              max_output_tokens: maxTokens,
              temperature: request.temperature,
            }),
            (err) => this.classifyOpenAIError(err, currentProvider.authMethod),
            {
              modelConfig,
              request,
              onAuthRefresh: () => {
                if (currentProvider.authMethod === 'codex_subscription' && this.maybeRefreshCodexToken()) {
                  const fresh = this.resolveRuntimeProvider(request.agentId, request.providerOverride);
                  if (fresh && (fresh.id === 'openai')) {
                    currentProvider = fresh;
                    return true;
                  }
                }
                return false;
              },
            },
          );
        } catch (err: any) {
          // Rate limited with a lower tier available: switch models and retry.
          // `input` isn't mutated for this turn until after a successful call,
          // so re-sending it on the fallback model loses no progress.
          if (err?.message === '__TIER_FALLBACK__' && err._fallbackTier) {
            const fallback = this.resolveModelForProvider(currentProvider, err._fallbackTier);
            if (fallback.model !== modelConfig.model) {
              this.bus.publish('llm.tier_fallback', 'llm-client', {
                from: modelConfig.model,
                to: fallback.model,
                reason: 'rate_limit_tool_loop',
                taskId: request.taskId,
                agentId: request.agentId,
              });
              modelConfig = fallback;
              maxTokens = request.maxTokens ?? fallback.maxTokens;
              continue;
            }
          }
          throw err;
        }
        this.observeOpenAIHeaders(currentProvider, iterCall.headers);
        const response = iterCall.body;

        const inputTokens = response.usage?.input_tokens ?? 0;
        const outputTokens = response.usage?.output_tokens ?? 0;
        totalInputTokens += inputTokens;
        totalOutputTokens += outputTokens;
        totalCostUsd += this.calculateCost(modelConfig.model, inputTokens, outputTokens);

        this.requestCount++;
        this.totalInputTokens += inputTokens;
        this.totalOutputTokens += outputTokens;
        this.totalCostUsd += this.calculateCost(modelConfig.model, inputTokens, outputTokens);

        const functionCalls = (response.output ?? []).filter((item) => item.type === 'function_call');
        if (functionCalls.length === 0) {
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
          });

          if (request.taskId && request.agentId) {
            this.bus.publish('usage.recorded', request.agentId, {
              taskId: request.taskId,
              agentId: request.agentId,
              model: modelConfig.model,
              inputTokens: totalInputTokens,
              outputTokens: totalOutputTokens,
              cacheReadTokens: 0,
              cacheWriteTokens: 0,
            });
          }

          return {
            content: this.extractOpenAIText(response),
            model: modelConfig.model,
            inputTokens: totalInputTokens,
            outputTokens: totalOutputTokens,
            totalTokens: totalInputTokens + totalOutputTokens,
            costUsd: totalCostUsd,
            stopReason: 'end_turn',
            durationMs,
            toolCallLog,
            iterations: iteration,
          };
        }

        input.push(...(response.output ?? []));
        for (const call of functionCalls) {
          const tool = toolMap.get(String(call.name));
          if (!tool) {
            input.push({
              type: 'function_call_output',
              call_id: call.call_id,
              output: `Error: Tool "${call.name}" not found in registry`,
            });
            continue;
          }
          let args: unknown = {};
          try {
            args = call.arguments ? JSON.parse(String(call.arguments)) : {};
          } catch {
            args = {};
          }
          const toolResult = await executor.execute(tool, args, {
            taskId: request.taskId,
            agentId: request.agentId,
          });
          input.push({
            type: 'function_call_output',
            call_id: call.call_id,
            output: toolResult.output || toolResult.error || '',
          });
        }
      }

      if (request.taskId && request.agentId) {
        this.bus.publish('usage.recorded', request.agentId, {
          taskId: request.taskId,
          agentId: request.agentId,
          model: modelConfig.model,
          inputTokens: totalInputTokens,
          outputTokens: totalOutputTokens,
          cacheReadTokens: 0,
          cacheWriteTokens: 0,
        });
      }

      return {
        content: '',
        model: modelConfig.model,
        inputTokens: totalInputTokens,
        outputTokens: totalOutputTokens,
        totalTokens: totalInputTokens + totalOutputTokens,
        costUsd: totalCostUsd,
        stopReason: 'max_iterations',
        durationMs: Date.now() - start,
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
   * Make an OpenAI Responses-API call. For ChatGPT/Codex subscription tokens,
   * targets `https://chatgpt.com/backend-api/codex/responses` with the
   * `chatgpt-account-id` and `OpenAI-Beta` headers, and consumes the mandated
   * SSE response stream (the subscription endpoint rejects non-streaming).
   * For API keys, hits the provider's configured base URL (api.openai.com by
   * default) and parses a single JSON body.
   *
   * On HTTP error throws OpenAIHttpError so the retry layer can classify by
   * status. On success returns a normalised body AND the raw Headers (used by
   * the quota tracker).
   */
  private async callOpenAIResponses(
    provider: RuntimeProvider & { id: 'openai' },
    body: Record<string, unknown>,
  ): Promise<OpenAICallResult> {
    const isCodexSub = provider.authMethod === 'codex_subscription';
    const url = isCodexSub
      ? 'https://chatgpt.com/backend-api/codex/responses'
      : `${provider.baseUrl.replace(/\/$/, '')}/responses`;

    const headers: Record<string, string> = {
      Authorization: `Bearer ${provider.token}`,
      'Content-Type': 'application/json',
    };
    const reqBody: Record<string, unknown> = { ...body };
    if (isCodexSub) {
      headers['OpenAI-Beta'] = 'responses=experimental';
      headers['Accept'] = 'text/event-stream';
      if (provider.accountId) headers['chatgpt-account-id'] = provider.accountId;
      // ChatGPT subscription endpoint mandates streaming + store=false and
      // rejects max_output_tokens.
      reqBody.store = false;
      reqBody.stream = true;
      delete reqBody.max_output_tokens;
    }

    const res = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(reqBody),
    });

    if (!res.ok) {
      const errJson = await res.json().catch(() => ({})) as any;
      // ChatGPT backend uses {detail: "..."}; OpenAI API uses {error: {message: "..."}}.
      const message = errJson.error?.message ?? errJson.detail ?? `OpenAI request failed: HTTP ${res.status}`;
      throw new OpenAIHttpError(res.status, res.headers, errJson, message);
    }

    if (isCodexSub) {
      return this.consumeOpenAISSE(res);
    }

    const json = await res.json().catch(() => ({})) as OpenAIResponse;
    return { body: json, headers: res.headers };
  }

  /**
   * Consume a Server-Sent Events response from the ChatGPT subscription
   * `/responses` endpoint and reassemble it into the same shape as the
   * non-streaming JSON response, so the rest of LLMClient doesn't need to
   * care which transport was used.
   *
   * Relevant events:
   *   - response.output_text.delta — incremental text chunks
   *   - response.output_item.done  — completed item (message or function_call)
   *   - response.completed         — terminal event with usage + final response
   */
  private async consumeOpenAISSE(res: Response): Promise<OpenAICallResult> {
    if (!res.body) {
      throw new OpenAIHttpError(res.status, res.headers, undefined, 'SSE response missing body');
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    let outputText = '';
    const outputItems: Record<string, any>[] = [];
    let usage: OpenAIResponse['usage'] | undefined;
    let responseId: string | undefined;

    try {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });

        // SSE events are separated by a blank line.
        let sepIdx: number;
        while ((sepIdx = buffer.indexOf('\n\n')) >= 0) {
          const rawEvent = buffer.slice(0, sepIdx);
          buffer = buffer.slice(sepIdx + 2);
          let dataLine = '';
          for (const line of rawEvent.split('\n')) {
            if (line.startsWith('data: ')) dataLine += line.slice(6);
            else if (line.startsWith('data:')) dataLine += line.slice(5);
          }
          if (!dataLine || dataLine === '[DONE]') continue;
          let evt: any;
          try { evt = JSON.parse(dataLine); } catch { continue; }

          switch (evt.type) {
            case 'response.output_text.delta':
              if (typeof evt.delta === 'string') outputText += evt.delta;
              break;
            case 'response.output_item.done':
              if (evt.item) outputItems.push(evt.item);
              break;
            case 'response.completed':
              responseId = evt.response?.id;
              usage = evt.response?.usage;
              break;
            case 'response.error':
            case 'error':
              throw new OpenAIHttpError(
                res.status,
                res.headers,
                evt,
                evt.error?.message ?? evt.message ?? 'SSE stream error',
              );
          }
        }
      }
    } finally {
      try { reader.releaseLock(); } catch { /* noop */ }
    }

    return {
      body: {
        id: responseId,
        output_text: outputText,
        output: outputItems,
        usage,
      },
      headers: res.headers,
    };
  }

  private recordOpenAIResult(
    response: OpenAIResponse,
    modelConfig: ModelConfig,
    request: LLMRequest,
    durationMs: number,
    source: string,
  ): LLMResponse {
    const inputTokens = response.usage?.input_tokens ?? 0;
    const outputTokens = response.usage?.output_tokens ?? 0;
    const totalTokens = response.usage?.total_tokens ?? inputTokens + outputTokens;
    const costUsd = this.calculateCost(modelConfig.model, inputTokens, outputTokens);
    const content = this.extractOpenAIText(response);

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
        totalTokens,
        costUsd,
        taskId: request.taskId,
        agentId: request.agentId,
        source,
      });
    }

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

    if (request.taskId && request.agentId) {
      this.bus.publish('usage.recorded', request.agentId, {
        taskId: request.taskId,
        agentId: request.agentId,
        model: modelConfig.model,
        inputTokens,
        outputTokens,
        cacheReadTokens: 0,
        cacheWriteTokens: 0,
      });
    }

    return {
      content,
      model: modelConfig.model,
      inputTokens,
      outputTokens,
      totalTokens,
      costUsd,
      stopReason: 'end_turn',
      durationMs,
    };
  }

  private extractOpenAIText(response: OpenAIResponse): string {
    if (response.output_text) return response.output_text;
    const parts: string[] = [];
    for (const item of response.output ?? []) {
      if (item.type !== 'message' || !Array.isArray(item.content)) continue;
      for (const content of item.content) {
        if (typeof content.text === 'string') parts.push(content.text);
      }
    }
    return parts.join('');
  }

  // ─── OpenAI-Compatible Chat Completions ───────────────────────────────
  // Handles Grok (xAI), Groq, OpenRouter, and Ollama — all of which speak
  // the standard /chat/completions wire protocol.

  private async completeOpenAICompatible(
    provider: RuntimeProvider & { id: 'grok' | 'groq' | 'openrouter' | 'ollama' },
    request: LLMRequest,
    fallbackDepth = 0,
  ): Promise<LLMResponse> {
    const modelConfig = this.resolveModelForProvider(provider, request.model);
    const system = await this.buildSystemPrompt(request);
    const maxTokens = request.maxTokens ?? modelConfig.maxTokens;
    const start = Date.now();

    // Build standard ChatCompletion messages with the system prompt as the
    // first system-role message (works for all four backends).
    const messages: Array<{ role: 'system' | 'user' | 'assistant'; content: string }> = [];
    if (system) messages.push({ role: 'system', content: system });
    for (const m of request.messages) messages.push({ role: m.role, content: m.content });

    let body: OpenAIChatBody;
    try {
      body = await this.callWithRetry<OpenAIChatBody>(
        () => this.callOpenAICompatibleChat(provider, {
          model: modelConfig.model,
          messages,
          max_tokens: maxTokens,
          temperature: request.temperature,
        }),
        (err) => this.classifyOpenAIError(err, provider.authMethod),
        { modelConfig, request, fallbackDepth },
      );
    } catch (err: any) {
      if (err?.message === '__TIER_FALLBACK__' && err._fallbackTier) {
        return this.completeOpenAICompatible(
          provider,
          { ...request, model: err._fallbackTier },
          (err._fallbackDepth ?? 0) + 1,
        );
      }
      throw err;
    }

    const inputTokens = body.usage?.prompt_tokens ?? 0;
    const outputTokens = body.usage?.completion_tokens ?? 0;
    const totalTokens = body.usage?.total_tokens ?? inputTokens + outputTokens;
    const costUsd = this.calculateCost(modelConfig.model, inputTokens, outputTokens);
    const content = body.choices?.[0]?.message?.content ?? '';
    const stopReason = body.choices?.[0]?.finish_reason ?? null;

    this.requestCount++;
    this.totalInputTokens += inputTokens;
    this.totalOutputTokens += outputTokens;
    this.totalCostUsd += costUsd;

    if (this.tokenTracker) {
      this.tokenTracker.record({
        timestamp: new Date(),
        model: modelConfig.model,
        inputTokens, outputTokens, totalTokens, costUsd,
        taskId: request.taskId, agentId: request.agentId,
        source: `llm_client_${provider.id}`,
      });
    }

    const durationMs = Date.now() - start;
    this.bus.publish('llm.completed', 'llm-client', {
      model: modelConfig.model, tier: modelConfig.tier,
      inputTokens, outputTokens, costUsd, durationMs,
      taskId: request.taskId, agentId: request.agentId,
    });
    if (request.taskId && request.agentId) {
      this.bus.publish('usage.recorded', request.agentId, {
        taskId: request.taskId, agentId: request.agentId,
        model: modelConfig.model, inputTokens, outputTokens,
        cacheReadTokens: 0, cacheWriteTokens: 0,
      });
    }

    return { content, model: modelConfig.model, inputTokens, outputTokens, totalTokens, costUsd, stopReason, durationMs };
  }

  /**
   * Raw POST to `${baseUrl}/chat/completions`. Throws OpenAIHttpError on
   * non-2xx so the retry layer can classify by status. Used by Grok / Groq /
   * OpenRouter / Ollama.
   */
  private async callOpenAICompatibleChat(
    provider: RuntimeProvider & { id: 'grok' | 'groq' | 'openrouter' | 'ollama' },
    body: Record<string, unknown>,
  ): Promise<OpenAIChatBody> {
    const url = `${provider.baseUrl.replace(/\/$/, '')}/chat/completions`;
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    // Ollama runs unauthenticated locally; everyone else uses bearer auth.
    if (provider.token) headers['Authorization'] = `Bearer ${provider.token}`;
    if (provider.id === 'openrouter') {
      headers['HTTP-Referer'] = 'https://github.com/tobsai/fort';
      headers['X-Title'] = 'Fort';
    }

    const res = await fetch(url, {
      method: 'POST', headers, body: JSON.stringify(body),
    });

    if (!res.ok) {
      const errJson = await res.json().catch(() => ({})) as any;
      const message = errJson.error?.message ?? errJson.detail ?? errJson.message ?? `${provider.id} request failed: HTTP ${res.status}`;
      throw new OpenAIHttpError(res.status, res.headers, errJson, message);
    }
    return await res.json() as OpenAIChatBody;
  }

  // ─── Google Gemini ────────────────────────────────────────────────────

  private async completeGoogle(
    provider: RuntimeProvider & { id: 'google' },
    request: LLMRequest,
    fallbackDepth = 0,
  ): Promise<LLMResponse> {
    const modelConfig = this.resolveModelForProvider(provider, request.model);
    const system = await this.buildSystemPrompt(request);
    const maxTokens = request.maxTokens ?? modelConfig.maxTokens;
    const start = Date.now();

    // Gemini uses contents[] with role/parts. 'assistant' becomes 'model'.
    const contents = request.messages.map((m) => ({
      role: m.role === 'assistant' ? 'model' : 'user',
      parts: [{ text: m.content }],
    }));

    let body: GeminiResponseBody;
    try {
      body = await this.callWithRetry<GeminiResponseBody>(
        () => this.callGoogleGemini(provider, {
          model: modelConfig.model,
          contents,
          systemInstruction: system ? { parts: [{ text: system }] } : undefined,
          generationConfig: { maxOutputTokens: maxTokens, temperature: request.temperature },
        }),
        (err) => this.classifyOpenAIError(err, provider.authMethod),
        { modelConfig, request, fallbackDepth },
      );
    } catch (err: any) {
      if (err?.message === '__TIER_FALLBACK__' && err._fallbackTier) {
        return this.completeGoogle(provider, { ...request, model: err._fallbackTier }, (err._fallbackDepth ?? 0) + 1);
      }
      throw err;
    }

    const inputTokens = body.usageMetadata?.promptTokenCount ?? 0;
    const outputTokens = body.usageMetadata?.candidatesTokenCount ?? 0;
    const totalTokens = body.usageMetadata?.totalTokenCount ?? inputTokens + outputTokens;
    const costUsd = this.calculateCost(modelConfig.model, inputTokens, outputTokens);
    const content = (body.candidates?.[0]?.content?.parts ?? [])
      .map((p) => p.text ?? '')
      .join('');
    const stopReason = body.candidates?.[0]?.finishReason ?? null;

    this.requestCount++;
    this.totalInputTokens += inputTokens;
    this.totalOutputTokens += outputTokens;
    this.totalCostUsd += costUsd;

    if (this.tokenTracker) {
      this.tokenTracker.record({
        timestamp: new Date(),
        model: modelConfig.model,
        inputTokens, outputTokens, totalTokens, costUsd,
        taskId: request.taskId, agentId: request.agentId,
        source: 'llm_client_google',
      });
    }

    const durationMs = Date.now() - start;
    this.bus.publish('llm.completed', 'llm-client', {
      model: modelConfig.model, tier: modelConfig.tier,
      inputTokens, outputTokens, costUsd, durationMs,
      taskId: request.taskId, agentId: request.agentId,
    });
    if (request.taskId && request.agentId) {
      this.bus.publish('usage.recorded', request.agentId, {
        taskId: request.taskId, agentId: request.agentId,
        model: modelConfig.model, inputTokens, outputTokens,
        cacheReadTokens: 0, cacheWriteTokens: 0,
      });
    }

    return { content, model: modelConfig.model, inputTokens, outputTokens, totalTokens, costUsd, stopReason, durationMs };
  }

  /**
   * Raw POST to Google Gemini `generateContent`. The model is part of the URL
   * path; auth is via `?key=` URL parameter.
   */
  private async callGoogleGemini(
    provider: RuntimeProvider & { id: 'google' },
    params: {
      model: string;
      contents: Array<{ role: string; parts: Array<{ text: string }> }>;
      systemInstruction?: { parts: Array<{ text: string }> };
      generationConfig?: { maxOutputTokens?: number; temperature?: number };
    },
  ): Promise<GeminiResponseBody> {
    const base = provider.baseUrl.replace(/\/$/, '');
    const url = `${base}/v1beta/models/${encodeURIComponent(params.model)}:generateContent?key=${encodeURIComponent(provider.token)}`;
    const body: Record<string, unknown> = { contents: params.contents };
    if (params.systemInstruction) body.systemInstruction = params.systemInstruction;
    if (params.generationConfig) body.generationConfig = params.generationConfig;

    const res = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });

    if (!res.ok) {
      const errJson = await res.json().catch(() => ({})) as any;
      const message = errJson.error?.message ?? `Gemini request failed: HTTP ${res.status}`;
      throw new OpenAIHttpError(res.status, res.headers, errJson, message);
    }
    return await res.json() as GeminiResponseBody;
  }
}

// ─── Response body shapes ────────────────────────────────────────────────

interface OpenAIChatBody {
  id?: string;
  choices?: Array<{ message?: { role?: string; content?: string }; finish_reason?: string }>;
  usage?: { prompt_tokens?: number; completion_tokens?: number; total_tokens?: number };
  error?: { message?: string };
}

interface GeminiResponseBody {
  candidates?: Array<{
    content?: { parts?: Array<{ text?: string }> };
    finishReason?: string;
  }>;
  usageMetadata?: {
    promptTokenCount?: number;
    candidatesTokenCount?: number;
    totalTokenCount?: number;
  };
  error?: { message?: string };
}
