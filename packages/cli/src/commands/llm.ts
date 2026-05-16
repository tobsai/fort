import { Command } from 'commander';
import { execSync, spawnSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { createInterface } from 'node:readline';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, green, yellow, cyan } from '../utils/format.js';
import { LLMClient } from '@fort-ai/core';

export function createLLMCommand(): Command {
  const cmd = new Command('llm')
    .description('LLM client management and stats');

  cmd
    .command('setup')
    .description('Authenticate Fort with an LLM provider (Claude, OpenAI, Grok, Groq, Google, OpenRouter)')
    .option('--api-key', 'Set up with an Anthropic API key instead of Claude subscription')
    .option('--openai', 'Authenticate with OpenAI via the Codex CLI subscription')
    .option('--grok', 'Set up Grok (xAI) — prompts for XAI_API_KEY')
    .option('--groq', 'Set up Groq (Llama inference) — prompts for GROQ_API_KEY')
    .option('--google', 'Set up Google Gemini — prompts for GEMINI_API_KEY')
    .option('--openrouter', 'Set up OpenRouter — prompts for OPENROUTER_API_KEY')
    .action(async (opts) => {
      await withFort(async (fort) => {
        if (opts.openai) {
          await setupOpenAI(fort);
          return;
        }
        if (opts.grok || opts.groq || opts.google || opts.openrouter) {
          await setupApiKeyProvider(opts);
          return;
        }
        if (fort.llm.isConfigured) {
          const authLabel =
            fort.llm.authMethod === 'dotenv' ? `~/.fort/.env` :
            fort.llm.authMethod === 'api_key_config' ? 'config file API key' :
            fort.llm.authMethod === 'openai_dotenv' ? `OPENAI_API_KEY in ~/.fort/.env` :
            fort.llm.authMethod === 'openai_api_key_env' ? 'OPENAI_API_KEY environment variable' :
            fort.llm.authMethod === 'codex_subscription' ? 'Codex/OpenAI subscription' :
            'ANTHROPIC_API_KEY environment variable';
          console.log(bold('\n  LLM Already Configured\n'));
          console.log(`  ${green('✓')} Authenticated via ${authLabel}.`);
          console.log(dim(`  Token stored at: ${LLMClient.envFilePath}`));
          console.log(dim('  Run `fort llm status` to see model details and usage.\n'));
          return;
        }

        if (opts.apiKey) {
          // Direct API key flow — show instructions
          console.log(bold('\n  API Key Setup\n'));
          console.log('  Create an API key at:\n');
          console.log(`    ${cyan('https://console.anthropic.com/settings/keys')}\n`);
          console.log('  ' + yellow('Note:') + ' API usage is billed separately from any Claude subscription.');
          console.log('  Set up billing at: ' + cyan('https://console.anthropic.com/settings/billing') + '\n');
          console.log('  Then run:\n');
          console.log(`    ${cyan('fort llm setup')} and paste your key when prompted.\n`);
          return;
        }

        // Check if Claude CLI is available for OAuth flow
        let hasClaude = false;
        try {
          execSync('which claude', { stdio: 'ignore' });
          hasClaude = true;
        } catch {
          // not found
        }

        if (!hasClaude) {
          console.log(bold('\n  Claude CLI not found\n'));
          console.log('  Fort can authenticate through the Claude CLI. Install it:\n');
          console.log(`    ${cyan('npm install -g @anthropic-ai/claude-code')}\n`);
          console.log('  Then run ' + cyan('fort llm setup') + ' again.\n');
          console.log(dim('  Or use an API key: ' + cyan('fort llm setup --api-key')));
          console.log(dim('  Or set up OpenAI:  ' + cyan('fort llm setup --openai')) + '\n');
          return;
        }

        // Run claude setup-token to get OAuth token
        console.log(bold('\n  Authenticating with Claude...\n'));
        console.log('  This will open your browser to sign in with your Anthropic account.');
        console.log('  Your Claude Pro/Team/Max subscription covers Fort usage.\n');

        const result = spawnSync('claude', ['setup-token'], {
          stdio: 'inherit',
          env: { ...process.env },
        });

        if (result.status !== 0) {
          console.log(`\n  ${yellow('⚠')} Authentication did not complete.`);
          console.log('  Try running ' + cyan('claude setup-token') + ' directly to debug.\n');
          return;
        }

        // After Claude CLI auth, read the token from keychain and save to .env
        let token: string | null = null;
        try {
          const raw = execSync(
            'security find-generic-password -s "Claude Code-credentials" -w 2>/dev/null',
            { encoding: 'utf-8', stdio: ['pipe', 'pipe', 'pipe'] },
          ).trim();
          if (raw) {
            try {
              const parsed = JSON.parse(raw);
              if (parsed.claudeAiOauth?.accessToken) {
                token = parsed.claudeAiOauth.accessToken;
              }
            } catch {
              if (raw.startsWith('sk-ant-')) token = raw;
            }
          }
        } catch {
          // Keychain read failed
        }

        if (token) {
          // Write to .env file for persistent, inspectable storage
          LLMClient.writeEnvFile(token);
          console.log(`\n  ${green('✓')} Authentication successful!\n`);
          console.log(`  Token saved to: ${cyan(LLMClient.envFilePath)}`);
          console.log(dim('  You can inspect or edit this file anytime.'));
          console.log(`  Verify with: ${cyan('fort llm status')}\n`);
        } else {
          console.log(`\n  ${yellow('⚠')} Could not extract token from keychain.`);
          console.log('  You can manually add your API key:\n');
          console.log(`    Edit ${cyan(LLMClient.envFilePath)} and add:`);
          console.log(`    ${dim('ANTHROPIC_API_KEY=sk-ant-your-key-here')}\n`);
        }
      });
    });

  cmd
    .command('status')
    .description('Show LLM client status and configuration')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const stats = fort.llm.getStats();

        if (opts.json) {
          console.log(JSON.stringify(stats, null, 2));
          return;
        }

        console.log(bold('\n  LLM Client Status\n'));

        const statusStr = stats.configured ? green('● Configured') : yellow('○ Not Configured');
        console.log(`  Status:         ${statusStr}`);
        if (stats.activeProvider) {
          const providerLabel =
            stats.activeProvider === 'anthropic' ? 'Anthropic' :
            stats.activeProvider === 'openai' ? 'OpenAI' :
            stats.activeProvider === 'groq' ? 'Groq' :
            stats.activeProvider === 'ollama' ? 'Ollama' :
            stats.activeProvider;
          console.log(`  Provider:       ${providerLabel}`);
        }
        if (stats.authMethod) {
          const authLabel =
            stats.authMethod === 'codex_subscription' ? 'Codex/OpenAI subscription' :
            stats.authMethod === 'openai_dotenv' ? 'OPENAI_API_KEY in ~/.fort/.env' :
            stats.authMethod === 'openai_api_key_env' ? 'OPENAI_API_KEY environment variable' :
            stats.authMethod === 'provider_store' ? 'Stored provider key' :
            stats.authMethod === 'dotenv' ? '~/.fort/.env' :
            stats.authMethod === 'api_key_config' ? 'Config file API key' :
            stats.authMethod === 'api_key_env' ? 'ANTHROPIC_API_KEY environment variable' :
            stats.authMethod;
          console.log(`  Auth:           ${authLabel}`);
        }
        console.log(`  Default Model:  ${stats.defaultTier}`);
        console.log();

        console.log(bold('  Models:'));
        for (const [tier, model] of Object.entries(stats.models)) {
          console.log(`    ${cyan(tier.padEnd(10))} ${(model as any).model}`);
          console.log(`    ${" ".repeat(10)} ${dim((model as any).description)}`);
        }
        console.log();

        if (stats.subscriptionQuota) {
          const q = stats.subscriptionQuota as any;
          console.log(bold('  Subscription:'));
          if (q.planType) {
            console.log(`    Plan:          ChatGPT ${q.planType[0].toUpperCase() + q.planType.slice(1)}`);
          }
          // Percent-based (ChatGPT backend): limit=100, used=percent. Render as "%".
          const isPercent = q.limit === 100 && q.used !== null;
          if (isPercent) {
            console.log(`    Used:          ${q.used}% (${q.remaining}% remaining)`);
          } else if (q.remaining !== null && q.limit !== null) {
            const used = q.limit - q.remaining;
            const pct = q.limit > 0 ? Math.round((used / q.limit) * 100) : 0;
            console.log(`    Remaining:     ${q.remaining.toLocaleString()} / ${q.limit.toLocaleString()} (${pct}% used)`);
          } else if (q.remaining !== null) {
            console.log(`    Remaining:     ${q.remaining.toLocaleString()}`);
          }
          if (q.resetAt) {
            const resetDate = new Date(q.resetAt);
            const deltaMs = resetDate.getTime() - Date.now();
            if (deltaMs > 0) {
              const mins = Math.round(deltaMs / 60_000);
              const human = mins >= 60 ? `${Math.floor(mins / 60)}h ${mins % 60}m` : `${mins}m`;
              console.log(`    Resets in:     ${human} (${resetDate.toLocaleString()})`);
            }
          }
          if (q.windowLabel) console.log(`    Window:        ${q.windowLabel}`);
          console.log();
        }

        console.log(bold('  Usage:'));
        console.log(`    Requests:      ${stats.requestCount}`);
        console.log(`    Input tokens:  ${stats.totalInputTokens.toLocaleString()}`);
        console.log(`    Output tokens: ${stats.totalOutputTokens.toLocaleString()}`);
        console.log(`    Total cost:    $${stats.totalCostUsd.toFixed(4)}`);
        console.log(`    Errors:        ${stats.errorCount}`);
        console.log();

        if (!stats.configured) {
          console.log(`  ${yellow('Not authenticated.')} Run ${cyan('fort llm setup')} to authenticate.\n`);
          console.log(dim(`  Or add your key directly to: ${LLMClient.envFilePath}\n`));
        }
      });
    });

  cmd
    .command('models')
    .description('List available model configurations for the active provider')
    .action(async () => {
      await withFort(async (fort) => {
        const models = fort.llm.getActiveModels();
        const stats = fort.llm.getStats();

        console.log(bold('\n  Model Routing Configuration\n'));
        if (stats.activeProvider) {
          console.log(dim(`  Active provider: ${stats.activeProvider}\n`));
        }

        for (const [tier, config] of Object.entries(models as Record<string, any>)) {
          console.log(`  ${bold(tier.toUpperCase().padEnd(12))} ${cyan(config.model)}`);
          console.log(`  ${''.padEnd(12)} Max tokens: ${config.maxTokens}`);
          console.log(`  ${''.padEnd(12)} ${dim(config.description)}`);
          console.log();
        }

        console.log(dim('  Fort automatically routes to the appropriate model based on task complexity.'));
        console.log(dim('  Override with --model fast|standard|powerful on any command.\n'));
      });
    });

  cmd
    .command('ask <prompt...>')
    .description('Send a one-off prompt to the LLM')
    .option('--model <tier>', 'Model tier: fast, standard, powerful', 'standard')
    .option('--no-behaviors', 'Do not inject behavioral rules')
    .option('--memory <query>', 'Inject relevant memories matching this query')
    .action(async (promptParts, opts) => {
      await withFort(async (fort) => {
        const prompt = promptParts.join(' ');

        if (!fort.llm.isConfigured) {
          console.error(`\n  ${yellow('LLM not configured.')} Run ${cyan('fort llm setup')} or add your key to ${cyan(LLMClient.envFilePath)}.\n`);
          return;
        }

        try {
          console.log(dim(`\n  Sending to ${opts.model}...\n`));

          const response = await fort.llm.complete({
            messages: [{ role: 'user', content: prompt }],
            model: opts.model,
            injectBehaviors: opts.behaviors !== false,
            injectMemory: opts.memory,
          });

          console.log(response.content);
          console.log();
          console.log(
            dim(
              `  ${response.model} | ${response.inputTokens}+${response.outputTokens} tokens | $${response.costUsd.toFixed(4)} | ${response.durationMs}ms`,
            ),
          );
          console.log();
        } catch (err) {
          console.error(`  Error: ${err instanceof Error ? err.message : err}`);
        }
      });
    });

  cmd
    .command('diagnose')
    .description('Run LLM health check')
    .action(async () => {
      await withFort(async (fort) => {
        const diag = fort.llm.diagnose();

        console.log(bold(`\n  LLM Diagnostics — ${diag.status}\n`));

        for (const check of diag.checks) {
          const icon = check.passed ? green('✓') : yellow('✗');
          console.log(`  ${icon} ${check.name}: ${check.message}`);
        }
        console.log();
      });
    });

  const providersCmd = new Command('providers').description('Manage configured LLM providers');

  providersCmd
    .command('list', { isDefault: true })
    .description('List configured LLM providers')
    .action(async () => {
      await withFort(async (fort) => {
        const providers = fort.llmProviders.listProviders();
        if (providers.length === 0) {
          console.log(dim('\n  No providers configured.'));
          console.log(dim('  Run `fort llm setup` (Claude) or `fort llm setup --openai` (OpenAI) to get started.\n'));
          return;
        }
        console.log(bold('\n  LLM Providers\n'));
        for (const p of providers) {
          const flag = p.isDefault ? green('●') : ' ';
          const status = p.enabled ? '' : dim(' (disabled)');
          console.log(`  ${flag} ${bold(p.name.padEnd(12))} ${cyan(p.id)}${status}`);
          console.log(`    ${dim('Model:')} ${p.defaultModel}`);
          console.log(`    ${dim('Key:  ')} ${p.apiKeyEncrypted ? 'configured' : (p.id === 'openai' ? 'using Codex subscription' : 'none')}`);
          console.log();
        }
      });
    });

  providersCmd
    .command('set-default <id>')
    .description('Set the default LLM provider')
    .action(async (id: string) => {
      await withFort(async (fort) => {
        try {
          fort.llmProviders.setDefault(id);
          console.log(`  ${green('✓')} Default provider set to ${cyan(id)}.\n`);
        } catch (err) {
          console.error(`  ${yellow('⚠')} ${err instanceof Error ? err.message : err}\n`);
        }
      });
    });

  providersCmd
    .command('remove <id>')
    .description('Remove an LLM provider from the store')
    .action(async (id: string) => {
      await withFort(async (fort) => {
        fort.llmProviders.deleteProvider(id);
        console.log(`  ${green('✓')} Removed provider ${cyan(id)}.\n`);
      });
    });

  providersCmd
    .command('test <id>')
    .description('Test connectivity to a configured provider')
    .action(async (id: string) => {
      await withFort(async (fort) => {
        const err = await fort.llm.testConnection(id);
        if (err) {
          console.log(`  ${yellow('✗')} ${id}: ${err}\n`);
        } else {
          console.log(`  ${green('✓')} ${id}: connection OK\n`);
        }
      });
    });

  cmd.addCommand(providersCmd);

  return cmd;
}

/**
 * Authenticate Fort with an OpenAI/ChatGPT subscription via the Codex CLI.
 * Mirrors the Claude OAuth flow: probe for the codex binary, run `codex login`,
 * then read ~/.codex/auth.json to confirm.
 */
async function setupOpenAI(fort: any): Promise<void> {
  const tokenInfo = LLMClient.readCodexOpenAIToken();
  if (tokenInfo) {
    console.log(bold('\n  Codex Subscription Detected\n'));
    console.log(`  ${green('✓')} Authenticated via active Codex/OpenAI subscription.`);
    if (tokenInfo.accountId) {
      console.log(`  Account ID: ${dim(tokenInfo.accountId)}`);
    }
    console.log(dim('  Token file: ~/.codex/auth.json (managed by Codex CLI)\n'));
    registerOpenAIProvider(fort, tokenInfo);
    return;
  }

  let hasCodex = false;
  const probe = spawnSync('which', ['codex'], { stdio: 'pipe' });
  if (probe.status === 0) hasCodex = true;

  if (!hasCodex) {
    console.log(bold('\n  Codex CLI not found\n'));
    console.log('  Fort authenticates with OpenAI through the Codex CLI. Install it:\n');
    console.log(`    ${cyan('npm install -g @openai/codex')}\n`);
    console.log(dim('  Install docs: https://github.com/openai/codex\n'));
    console.log('  Then run ' + cyan('fort llm setup --openai') + ' again.\n');
    return;
  }

  console.log(bold('\n  Authenticating with OpenAI...\n'));
  console.log('  This will open your browser to sign in with your OpenAI/ChatGPT account.');
  console.log('  Your ChatGPT Plus/Pro/Team subscription covers Fort usage.\n');

  const result = spawnSync('codex', ['login'], {
    stdio: 'inherit',
    env: { ...process.env },
  });

  if (result.status !== 0) {
    console.log(`\n  ${yellow('⚠')} Authentication did not complete.`);
    console.log('  Try running ' + cyan('codex login') + ' directly to debug.\n');
    return;
  }

  const fresh = LLMClient.readCodexOpenAIToken();
  if (!fresh) {
    console.log(`\n  ${yellow('⚠')} Could not read credentials from ~/.codex/auth.json.`);
    console.log('  The login flow exited, but Fort cannot find the access token.\n');
    return;
  }

  console.log(`\n  ${green('✓')} Authentication successful!\n`);
  if (fresh.accountId) {
    console.log(`  Account ID: ${dim(fresh.accountId)}`);
  }
  console.log(dim('  Token file: ~/.codex/auth.json (managed by Codex CLI)'));
  console.log(`  Verify with: ${cyan('fort llm status')}\n`);
  registerOpenAIProvider(fort, fresh);
}

/**
 * Register the OpenAI provider in Fort's provider store so it shows up in the
 * dashboard Settings page. Idempotent — does nothing if already registered.
 */
/**
 * One-flag setup flow for API-key providers (Grok, Groq, Google, OpenRouter).
 * Prompts for the API key, writes it to ~/.fort/.env under the right env var.
 */
async function setupApiKeyProvider(opts: {
  grok?: boolean; groq?: boolean; google?: boolean; openrouter?: boolean;
}): Promise<void> {
  const spec = opts.grok
    ? { id: 'grok',       label: 'Grok (xAI)',  envVar: 'XAI_API_KEY',        prefix: 'xai-',    url: 'https://console.x.ai' }
    : opts.groq
    ? { id: 'groq',       label: 'Groq',        envVar: 'GROQ_API_KEY',        prefix: 'gsk_',    url: 'https://console.groq.com/keys' }
    : opts.google
    ? { id: 'google',     label: 'Google Gemini', envVar: 'GEMINI_API_KEY',    prefix: 'AIza',    url: 'https://aistudio.google.com/apikey' }
    : { id: 'openrouter', label: 'OpenRouter',  envVar: 'OPENROUTER_API_KEY', prefix: 'sk-or-',  url: 'https://openrouter.ai/keys' };

  console.log(bold(`\n  ${spec.label} Setup\n`));
  console.log(`  Get an API key at: ${cyan(spec.url)}\n`);

  const key = await new Promise<string>((resolve) => {
    const rl = createInterface({ input: process.stdin, output: process.stdout });
    rl.question(`  ${bold(spec.envVar + ':')} `, (answer) => { rl.close(); resolve(answer.trim()); });
  });

  if (!key) {
    console.log(`\n  ${yellow('⚠')} No key entered. Aborted.\n`);
    return;
  }
  if (!key.startsWith(spec.prefix)) {
    console.log(`\n  ${yellow('⚠')} Key does not start with expected prefix "${spec.prefix}". Saving anyway.`);
  }

  LLMClient.writeEnvFileValue(spec.envVar, key);
  console.log(`\n  ${green('✓')} ${spec.label} key saved to ${cyan(LLMClient.envFilePath)}.`);
  console.log(dim(`  Verify with: ${cyan('fort llm status')}\n`));
}

function registerOpenAIProvider(fort: any, _tokenInfo: { accountId?: string }): void {
  try {
    const existing = fort.llmProviders.getProvider('openai');
    if (existing) return;
    fort.llmProviders.addProvider({
      id: 'openai',
      name: 'OpenAI',
      baseUrl: 'https://api.openai.com/v1',
      defaultModel: 'gpt-5.4',
      enabled: true,
      isDefault: false,
    });
    console.log(dim('  Registered OpenAI provider in Fort.'));
    console.log(dim('  Make it the default with: ' + cyan('fort llm providers set-default openai') + '\n'));
  } catch (err) {
    console.log(dim('  Could not register provider in store: ' + (err instanceof Error ? err.message : String(err))));
  }
}
