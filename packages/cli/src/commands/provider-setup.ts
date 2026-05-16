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

/**
 * Interactive provider picker. Lists all 7 providers with their detected
 * state and asks the user to pick one. If the picked provider is already
 * configured, sets it as the global default and returns its id. If it's
 * not configured, runs the setup flow first, then re-shows the menu.
 *
 * No Enter default — the user must explicitly type a number. This matches
 * the user's preference: "detect the provider, but not preselect one".
 *
 * Returns the chosen provider id (which is now also the global default).
 */
export async function pickProvider(fort: any): Promise<ProviderId | null> {
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

    // Already configured — just set as default and return.
    if (chosen.usable) {
      try {
        if (fort.llmProviders.getProvider(chosen.id)) {
          fort.llmProviders.setDefault(chosen.id);
        }
      } catch { /* row may not exist for env-only providers — that's fine */ }
      console.log(`    ${green('✓')} Selected ${bold(chosen.name)} as your default provider.`);
      return chosen.id;
    }

    // Not configured — run setup, then loop back to let the user confirm.
    await setupProvider(chosen.id);
    console.log(`\n    ${dim('Re-checking provider state...')}`);
    // Loop continues — refreshed `providers` will reflect the new state and
    // user can pick the now-configured provider (or a different one).
  }
}

/**
 * Backwards-compat wrapper: setup-only menu (no return value), used by
 * places that just want to walk a user through configuring credentials
 * without selecting a default. Returns when at least one provider is
 * configured and the user presses Enter.
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
