/**
 * OpenClaw → Fort importer
 *
 * Reads ~/.openclaw/openclaw.json and migrates agents + LLM providers
 * into Fort's data directory.
 */

import { existsSync, readFileSync, writeFileSync, mkdirSync, copyFileSync } from 'node:fs';
import { join } from 'node:path';
import { homedir } from 'node:os';
import { stringify as stringifyYaml } from 'yaml';
import type { Fort } from '../fort.js';

// ─── OpenClaw config types (what we actually need) ────────────────────────────

interface OpenClawIdentity {
  name?: string;
  emoji?: string;
  avatar?: string;
}

interface OpenClawModelRef {
  primary?: string;
  fallbacks?: string[];
}

interface OpenClawAgent {
  id: string;
  name?: string;
  workspace?: string;
  model?: string | OpenClawModelRef;
  identity?: OpenClawIdentity;
  default?: boolean;
}

interface OpenClawProvider {
  apiKey?: string;
  baseUrl?: string;
  models?: Array<{ id: string }>;
}

interface OpenClawConfig {
  agents?: {
    defaults?: { workspace?: string };
    list?: OpenClawAgent[];
  };
  models?: {
    providers?: Record<string, OpenClawProvider>;
  };
  env?: Record<string, string>;
}

// ─── Preview / result types ───────────────────────────────────────────────────

export interface OpenClawAgentPreview {
  id: string;           // Proposed Fort id (slugified)
  name: string;
  emoji?: string;
  workspace: string;
  hasSoul: boolean;
  hasMemory: boolean;
  alreadyExists: boolean;
}

export interface OpenClawProviderPreview {
  id: string;           // 'anthropic' | 'openai' | 'groq'
  hasApiKey: boolean;
  alreadyExists: boolean;
}

export interface OpenClawPreview {
  found: boolean;
  configPath?: string;
  agents: OpenClawAgentPreview[];
  providers: OpenClawProviderPreview[];
  warnings: string[];
}

export interface OpenClawImportResult {
  agentsCreated: string[];
  agentsSkipped: string[];
  providersAdded: string[];
  providersSkipped: string[];
  errors: string[];
}

// ─── Provider ID mapping ──────────────────────────────────────────────────────

// OpenClaw provider IDs that map to Fort-supported providers
const SUPPORTED_PROVIDERS: Record<string, { name: string; defaultModel: string; baseUrl?: string }> = {
  anthropic: { name: 'Anthropic', defaultModel: 'claude-sonnet-4-5-20250929' },
  openai:    { name: 'OpenAI',    defaultModel: 'gpt-4o', baseUrl: 'https://api.openai.com/v1' },
  groq:      { name: 'Groq',      defaultModel: 'llama-3.3-70b-versatile', baseUrl: 'https://api.groq.com/openai/v1' },
};

// Env var names for API keys per provider
const PROVIDER_ENV_KEYS: Record<string, string[]> = {
  anthropic: ['ANTHROPIC_API_KEY', 'ANTHROPIC_API_KEY_1'],
  openai:    ['OPENAI_API_KEY'],
  groq:      ['GROQ_API_KEY'],
};

// ─── Helpers ──────────────────────────────────────────────────────────────────

function slugify(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
}

function defaultOpenClawDir(): string {
  return join(homedir(), '.openclaw');
}

function readOpenClawConfig(openClawDir: string): OpenClawConfig | null {
  const configPath = join(openClawDir, 'openclaw.json');
  if (!existsSync(configPath)) return null;
  try {
    return JSON.parse(readFileSync(configPath, 'utf-8')) as OpenClawConfig;
  } catch {
    return null;
  }
}

function resolveWorkspace(agent: OpenClawAgent, cfg: OpenClawConfig, openClawDir: string): string {
  const raw = agent.workspace ?? cfg.agents?.defaults?.workspace ?? join(openClawDir, 'workspace');
  return raw.replace(/^~/, homedir());
}

function extractApiKey(providerId: string, providerCfg: OpenClawProvider | undefined, env: Record<string, string>): string | undefined {
  // Prefer inline key in provider config
  if (providerCfg?.apiKey) return providerCfg.apiKey;
  // Fall back to env section
  for (const envKey of PROVIDER_ENV_KEYS[providerId] ?? []) {
    if (env[envKey]) return env[envKey];
  }
  return undefined;
}

function extractModelId(model: string | OpenClawModelRef | undefined, providerId: string): string | undefined {
  if (!model) return undefined;
  if (typeof model === 'string') {
    // Format: "anthropic/claude-opus-4-5" or just "claude-opus-4-5"
    const parts = model.split('/');
    return parts.length > 1 ? parts[1] : parts[0];
  }
  const primary = model.primary ?? '';
  const parts = primary.split('/');
  const modelId = parts.length > 1 ? parts[1] : parts[0];
  return modelId || undefined;
}

// ─── Public API ───────────────────────────────────────────────────────────────

/**
 * Scan for an OpenClaw installation and return a preview of what would be imported.
 * Does not modify any Fort data.
 */
export function scanOpenClaw(fort: Fort, openClawDir?: string): OpenClawPreview {
  const dir = openClawDir ?? defaultOpenClawDir();
  const cfg = readOpenClawConfig(dir);

  if (!cfg) {
    return { found: false, agents: [], providers: [], warnings: [] };
  }

  const warnings: string[] = [];
  const agentPreviews: OpenClawAgentPreview[] = [];
  const providerPreviews: OpenClawProviderPreview[] = [];

  // ── Agents ──
  const agentList = cfg.agents?.list ?? [];
  if (agentList.length === 0) {
    warnings.push('No agents found in openclaw.json');
  }

  for (const agent of agentList) {
    const rawName = agent.identity?.name ?? agent.name ?? agent.id;
    if (!rawName) {
      warnings.push(`Skipping agent with missing name (id: ${agent.id})`);
      continue;
    }
    const fortId = slugify(rawName);
    const workspace = resolveWorkspace(agent, cfg, dir);
    const hasSoul = existsSync(join(workspace, 'SOUL.md'));
    const hasMemory = existsSync(join(workspace, 'MEMORY.md'));
    const agentDir = fort.agentFactory.getAgentDir(fortId);
    const alreadyExists = existsSync(join(agentDir, 'identity.yaml'));

    agentPreviews.push({
      id: fortId,
      name: rawName,
      emoji: agent.identity?.emoji,
      workspace,
      hasSoul,
      hasMemory,
      alreadyExists,
    });
  }

  // ── Providers ──
  const providers = cfg.models?.providers ?? {};
  const env = cfg.env ?? {};

  for (const [pid, meta] of Object.entries(SUPPORTED_PROVIDERS)) {
    const providerCfg = providers[pid];
    const apiKey = extractApiKey(pid, providerCfg, env);
    const alreadyExists = !!fort.llmProviders.getProvider(pid);

    providerPreviews.push({
      id: pid,
      hasApiKey: !!apiKey,
      alreadyExists,
    });
  }

  return {
    found: true,
    configPath: join(dir, 'openclaw.json'),
    agents: agentPreviews,
    providers: providerPreviews,
    warnings,
  };
}

/**
 * Import agents and LLM providers from OpenClaw into Fort.
 * Skips items that already exist.
 */
export async function importOpenClaw(fort: Fort, openClawDir?: string): Promise<OpenClawImportResult> {
  const dir = openClawDir ?? defaultOpenClawDir();
  const cfg = readOpenClawConfig(dir);

  const result: OpenClawImportResult = {
    agentsCreated: [],
    agentsSkipped: [],
    providersAdded: [],
    providersSkipped: [],
    errors: [],
  };

  if (!cfg) {
    result.errors.push('OpenClaw config not found at ' + join(dir, 'openclaw.json'));
    return result;
  }

  const env = cfg.env ?? {};

  // ── Import agents ──
  for (const agent of cfg.agents?.list ?? []) {
    const rawName = agent.identity?.name ?? agent.name ?? agent.id;
    if (!rawName) {
      result.errors.push(`Skipping agent with missing name (id: ${agent.id})`);
      continue;
    }

    const fortId = slugify(rawName);

    const fortAgentDir = fort.agentFactory.getAgentDir(fortId);
    if (existsSync(join(fortAgentDir, 'identity.yaml'))) {
      result.agentsSkipped.push(fortId);
      continue;
    }

    try {
      const workspace = resolveWorkspace(agent, cfg, dir);

      // Read SOUL.md from OpenClaw workspace (optional)
      let soulContent: string | undefined;
      const soulPath = join(workspace, 'SOUL.md');
      if (existsSync(soulPath)) {
        soulContent = readFileSync(soulPath, 'utf-8');
      }

      // Determine model tier from model string
      const modelStr = typeof agent.model === 'string' ? agent.model :
        (agent.model as OpenClawModelRef | undefined)?.primary ?? '';
      let modelTier: 'fast' | 'standard' | 'powerful' = 'standard';
      if (modelStr.includes('opus')) modelTier = 'powerful';
      else if (modelStr.includes('haiku') || modelStr.includes('mini') || modelStr.includes('instant')) modelTier = 'fast';

      // Create the Fort agent
      const createdAgent = fort.agentFactory.create({
        name: rawName,
        description: `Imported from OpenClaw`,
        emoji: agent.identity?.emoji,
        createdBy: 'openclaw-import',
      });

      const agentDir = fort.agentFactory.getAgentDir(fortId);

      // Overwrite SOUL.md with OpenClaw content if available
      if (soulContent) {
        writeFileSync(join(agentDir, 'SOUL.md'), soulContent, 'utf-8');
        createdAgent.refreshSoul();
      }

      // Copy MEMORY.md if present
      const memoryPath = join(workspace, 'MEMORY.md');
      if (existsSync(memoryPath)) {
        copyFileSync(memoryPath, join(agentDir, 'MEMORY.md'));
      }

      // Patch identity with modelTier and isDefault
      const identity = createdAgent.identity;
      (identity as any).defaultModelTier = modelTier;
      if (agent.default) (identity as any).isDefault = true;
      writeFileSync(join(agentDir, 'identity.yaml'), stringifyYaml(identity), 'utf-8');

      // Copy avatar if referenced and exists
      if (agent.identity?.avatar) {
        const avatarSrc = agent.identity.avatar.startsWith('/')
          ? agent.identity.avatar
          : join(workspace, agent.identity.avatar);
        if (existsSync(avatarSrc)) {
          const ext = avatarSrc.split('.').pop() ?? 'png';
          copyFileSync(avatarSrc, join(agentDir, `avatar.${ext}`));
        }
      }

      await createdAgent.start();
      result.agentsCreated.push(fortId);
    } catch (err) {
      result.errors.push(`Agent "${rawName}": ${err instanceof Error ? err.message : String(err)}`);
    }
  }

  // ── Import LLM providers ──
  const providers = cfg.models?.providers ?? {};

  for (const [pid, meta] of Object.entries(SUPPORTED_PROVIDERS)) {
    if (fort.llmProviders.getProvider(pid)) {
      result.providersSkipped.push(pid);
      continue;
    }

    const providerCfg = providers[pid];
    const apiKey = extractApiKey(pid, providerCfg, env);

    // Only add if we have an API key (Ollama doesn't need one but isn't in OpenClaw)
    if (!apiKey) {
      // No key found — skip silently (user hasn't configured this provider in OpenClaw)
      continue;
    }

    try {
      // Find the best default model from OpenClaw's model list
      const openClawModels = providerCfg?.models?.map((m) => m.id) ?? [];
      const defaultModel = openClawModels[0] ?? meta.defaultModel;

      fort.llmProviders.addProvider({
        id: pid,
        name: meta.name,
        apiKey,
        baseUrl: providerCfg?.baseUrl ?? meta.baseUrl,
        defaultModel,
        isDefault: result.providersAdded.length === 0 && result.providersSkipped.length === 0,
      });

      result.providersAdded.push(pid);
    } catch (err) {
      result.errors.push(`Provider "${pid}": ${err instanceof Error ? err.message : String(err)}`);
    }
  }

  return result;
}
