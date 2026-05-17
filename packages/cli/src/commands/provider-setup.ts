/**
 * Provider credential setup helpers, shared by `fort init` (interactive multi-
 * provider menu) and `fort llm setup --<provider>` flags.
 *
 * Each setup function persists its credential and returns whether it
 * succeeded. The orchestrator (`runProviderSetupMenu`) shows the live state
 * of all 7 providers as a numbered list and dispatches to the right helper.
 */
import { createInterface } from 'node:readline';
import { spawnSync } from 'node:child_process';
import { LLMClient } from '@fort-ai/core';
import { bold, cyan, dim, green, yellow } from '../utils/format.js';

const PROVIDER_ICONS: Record<string, string> = {
  anthropic:  '🟣',
  openai:     '🟢',
  grok:       '⚪',
  groq:       '⚡',
  google:     '🔵',
  ollama:     '🦙',
  openrouter: '🛣️',
};

type ProviderId = 'anthropic' | 'openai' | 'grok' | 'groq' | 'google' | 'ollama' | 'openrouter';

function prompt(question: string): Promise<string> {
  const rl = createInterface({ input: process.stdin, output: process.stdout });
  return new Promise((resolve) => {
    rl.question(question, (answer) => { rl.close(); resolve(answer.trim()); });
  });
}

/** True iff a binary is reachable via PATH. Uses spawnSync (no shell injection). */
function hasBinary(name: string): boolean {
  return spawnSync('which', [name], { stdio: 'ignore' }).status === 0;
}

export interface ProviderPickResult {
  /** The provider chosen as primary/default. Null if the user configured nothing. */
  primary: ProviderId | null;
  /** All provider ids configured (or already configured) in this session. */
  configured: ProviderId[];
}

/**
 * Detect what's already set up so callers can short-circuit when re-running init.
 */
export async function detectExistingSetup(fort: any): Promise<{ hasDefault: boolean; providers: Array<{ id: ProviderId; name: string; authMethod?: string }> }> {
  const available = await fort.llm.getAvailableProviders();
  const usable = available.filter((p: any) => p.usable);
  let hasDefault = false;
  try {
    hasDefault = !!fort.llmProviders.getDefaultProvider();
  } catch { /* store may be empty — ambient creds count as configured but not "default" */ }
  // Treat ambient-only configuration as "has default" too, so the wizard skips correctly.
  if (!hasDefault && usable.length > 0) hasDefault = true;
  return {
    hasDefault,
    providers: usable.map((p: any) => ({ id: p.id, name: p.name, authMethod: p.authMethod })),
  };
}

/**
 * Interactive multi-provider picker. Lists all 7 providers with detected state;
 * picking a configured one selects it, picking an unconfigured one runs setup.
 * After each successful setup, asks "Configure another?". When the user is done,
 * if more than one was configured this session, asks which should be primary.
 *
 * No Enter default — the user must explicitly type a number.
 *
 * Returns the primary provider id and the full list of providers configured.
 */
export async function pickProvider(fort: any): Promise<ProviderPickResult> {
  const configured = new Set<ProviderId>();
  let primary: ProviderId | null = null;

  while (true) {
    const providers = await fort.llm.getAvailableProviders();
    console.log(bold('\n  Detected providers:\n'));
    providers.forEach((p: any, i: number) => {
      const num = cyan(`[${i + 1}]`);
      const icon = PROVIDER_ICONS[p.id] ?? '🔌';
      const status = p.usable
        ? green(`✓ ${p.authMethod ?? 'configured'}`)
        : yellow('○ not configured');
      console.log(`    ${num} ${icon}  ${bold(p.name.padEnd(14))} ${status}`);
    });

    const answer = await prompt(`\n    ${bold('Pick a provider')} ${dim(`[1-${providers.length}]`)}: `);
    if (!answer) {
      console.log(`    ${yellow('Type a number to pick a provider.')}`);
      continue;
    }

    const idx = parseInt(answer, 10) - 1;
    if (Number.isNaN(idx) || idx < 0 || idx >= providers.length) {
      console.log(`    ${yellow('Invalid selection.')}`);
      continue;
    }

    const chosen = providers[idx] as { id: ProviderId; name: string; usable: boolean };

    if (chosen.usable) {
      configured.add(chosen.id);
      if (!primary) primary = chosen.id;
      console.log(`    ${green('✓')} ${bold(chosen.name)} ready.`);
    } else {
      await setupProvider(chosen.id);
      console.log(`\n    ${dim('Re-checking provider state...')}`);
      // Re-fetch to see if setup succeeded.
      const refreshed = await fort.llm.getAvailableProviders();
      const refreshedChosen = refreshed.find((p: any) => p.id === chosen.id);
      if (refreshedChosen?.usable) {
        configured.add(chosen.id);
        if (!primary) primary = chosen.id;
      } else {
        console.log(`    ${yellow('⚠')} ${chosen.name} not configured — try again or pick another.`);
        continue;
      }
    }

    if (configured.size === 0) continue;

    const more = await prompt(`\n    ${bold('Configure another provider?')} ${dim('[y/N]')}: `);
    if (more.toLowerCase() !== 'y' && more.toLowerCase() !== 'yes') break;
  }

  // Choose primary when multiple were configured this session.
  const configuredList = Array.from(configured);
  if (configuredList.length > 1) {
    console.log(bold('\n  Which provider should be primary?\n'));
    configuredList.forEach((id, i) => {
      const icon = PROVIDER_ICONS[id] ?? '🔌';
      console.log(`    ${cyan(`[${i + 1}]`)} ${icon}  ${bold(id)}`);
    });
    while (true) {
      const ans = await prompt(`\n    ${bold('Pick primary')} ${dim(`[1-${configuredList.length}]`)}: `);
      const i = parseInt(ans, 10) - 1;
      if (!Number.isNaN(i) && i >= 0 && i < configuredList.length) {
        primary = configuredList[i];
        break;
      }
      console.log(`    ${yellow('Invalid selection.')}`);
    }
  }

  // Persist the primary as the global default (best-effort — env-only providers
  // may not have a provider-store row, and that's fine).
  if (primary) {
    try {
      if (fort.llmProviders.getProvider(primary)) {
        fort.llmProviders.setDefault(primary);
      }
    } catch { /* env-only — ambient detection still works */ }
    const name = primary;
    console.log(`\n    ${green('✓')} Primary provider: ${bold(name)}.`);
  }

  return { primary, configured: configuredList };
}

/**
 * Backwards-compat wrapper: setup-only menu used by `fort llm setup` (no flags).
 * Returns when the user finishes the multi-provider flow.
 */
export async function runProviderSetupMenu(fort: any): Promise<void> {
  await pickProvider(fort);
}

/**
 * Dispatch to the per-provider setup flow. Each flow handles its own prompts
 * and writes credentials to the right location (Claude keychain → .env,
 * Codex CLI auth file, or ~/.fort/.env for API keys).
 */
export async function setupProvider(id: ProviderId): Promise<void> {
  switch (id) {
    case 'anthropic':  return setupAnthropic();
    case 'openai':     return setupOpenAI();
    case 'grok':       return setupApiKeyProvider('grok');
    case 'groq':       return setupApiKeyProvider('groq');
    case 'google':     return setupApiKeyProvider('google');
    case 'openrouter': return setupApiKeyProvider('openrouter');
    case 'ollama':     return setupOllamaInstructions();
  }
}

async function setupAnthropic(): Promise<void> {
  console.log(bold('\n  Anthropic Setup\n'));

  const hasClaude = hasBinary('claude');

  if (hasClaude) {
    console.log(`    Run ${cyan('claude setup-token')} to authenticate via your Claude subscription.`);
    console.log(`    Or paste an API key from ${cyan('https://console.anthropic.com/settings/keys')}\n`);
    const choice = await prompt(`    ${bold('Run claude setup-token?')} ${dim('[Y/n]')}: `);
    if (choice.toLowerCase() !== 'n' && choice.toLowerCase() !== 'no') {
      const result = spawnSync('claude', ['setup-token'], { stdio: 'inherit', env: { ...process.env } });
      if (result.status === 0) {
        const token = LLMClient.readKeychainToken();
        if (token) {
          LLMClient.writeEnvFile(token);
          console.log(`\n    ${green('✓')} Authenticated via Claude Code keychain.`);
        } else {
          console.log(`\n    ${yellow('⚠')} Token not detected. Try ${cyan('fort llm setup')} later.`);
        }
        return;
      }
      console.log(`\n    ${yellow('⚠')} Authentication did not complete.`);
      return;
    }
  } else {
    console.log(`    ${dim('Claude CLI not found.')} Install with: ${cyan('npm i -g @anthropic-ai/claude-code')}\n`);
    console.log(`    Or paste an API key from ${cyan('https://console.anthropic.com/settings/keys')}\n`);
  }

  const key = await prompt(`    ${bold('Paste your Anthropic API key (sk-ant-...):')} `);
  if (!key) {
    console.log(`    ${dim('Skipped.')}\n`);
    return;
  }
  if (!key.startsWith('sk-ant-')) {
    console.log(`    ${yellow('⚠')} Key does not start with "sk-ant-". Saving anyway.`);
  }
  LLMClient.writeEnvFile(key);
  console.log(`    ${green('✓')} Saved to ${cyan(LLMClient.envFilePath)}`);
}

async function setupOpenAI(): Promise<void> {
  console.log(bold('\n  OpenAI Setup\n'));

  const existing = LLMClient.readCodexOpenAIToken();
  if (existing) {
    console.log(`    ${green('✓')} Codex/OpenAI subscription already authenticated.`);
    if (existing.accountId) console.log(`    Account ID: ${dim(existing.accountId)}`);
    console.log();
    return;
  }

  const hasCodex = hasBinary('codex');

  if (hasCodex) {
    console.log(`    Run ${cyan('codex login')} to authenticate via your ChatGPT subscription.\n`);
    const choice = await prompt(`    ${bold('Run codex login?')} ${dim('[Y/n]')}: `);
    if (choice.toLowerCase() !== 'n' && choice.toLowerCase() !== 'no') {
      const result = spawnSync('codex', ['login'], { stdio: 'inherit', env: { ...process.env } });
      if (result.status === 0) {
        const fresh = LLMClient.readCodexOpenAIToken();
        if (fresh) {
          console.log(`\n    ${green('✓')} Codex/OpenAI subscription authenticated.`);
          if (fresh.accountId) console.log(`    Account ID: ${dim(fresh.accountId)}`);
        } else {
          console.log(`\n    ${yellow('⚠')} Token not detected. Try ${cyan('codex login')} again.`);
        }
        console.log();
        return;
      }
      console.log(`\n    ${yellow('⚠')} Codex login did not complete.\n`);
      return;
    }
  } else {
    console.log(`    ${dim('Codex CLI not found.')} Install with: ${cyan('npm i -g @openai/codex')}\n`);
  }

  console.log(`    Or paste an API key from ${cyan('https://platform.openai.com/api-keys')}\n`);
  const key = await prompt(`    ${bold('Paste your OpenAI API key (sk-...):')} `);
  if (!key) {
    console.log(`    ${dim('Skipped.')}\n`);
    return;
  }
  LLMClient.writeEnvFileValue('OPENAI_API_KEY', key);
  console.log(`    ${green('✓')} Saved to ${cyan(LLMClient.envFilePath)}`);
}

async function setupApiKeyProvider(id: 'grok' | 'groq' | 'google' | 'openrouter'): Promise<void> {
  const spec = id === 'grok'
    ? { label: 'Grok (xAI)',     envVar: 'XAI_API_KEY',         prefix: 'xai-',   url: 'https://console.x.ai' }
    : id === 'groq'
    ? { label: 'Groq',           envVar: 'GROQ_API_KEY',        prefix: 'gsk_',   url: 'https://console.groq.com/keys' }
    : id === 'google'
    ? { label: 'Google Gemini',  envVar: 'GEMINI_API_KEY',      prefix: 'AIza',   url: 'https://aistudio.google.com/apikey' }
    : { label: 'OpenRouter',     envVar: 'OPENROUTER_API_KEY',  prefix: 'sk-or-', url: 'https://openrouter.ai/keys' };

  console.log(bold(`\n  ${spec.label} Setup\n`));
  console.log(`    Get an API key at: ${cyan(spec.url)}\n`);

  const key = await prompt(`    ${bold(spec.envVar + ':')} `);
  if (!key) {
    console.log(`    ${dim('Skipped.')}\n`);
    return;
  }
  if (!key.startsWith(spec.prefix)) {
    console.log(`    ${yellow('⚠')} Key does not start with "${spec.prefix}". Saving anyway.`);
  }
  LLMClient.writeEnvFileValue(spec.envVar, key);
  console.log(`    ${green('✓')} ${spec.label} key saved to ${cyan(LLMClient.envFilePath)}`);
}

async function setupOllamaInstructions(): Promise<void> {
  console.log(bold('\n  Ollama Setup\n'));
  console.log(`    Ollama runs locally. Make sure the server is running:`);
  console.log(`      ${cyan('ollama serve')}    ${dim('(starts the local server on :11434)')}`);
  console.log(`      ${cyan('ollama pull llama3.2')}   ${dim('(downloads a model)')}\n`);
  await prompt(`    ${dim('Press Enter when the server is running...')}`);
}
