import { Command } from 'commander';
import { execSync, spawnSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, green, yellow, cyan } from '../utils/format.js';
import { LLMClient } from '@fort/core';

export function createLLMCommand(): Command {
  const cmd = new Command('llm')
    .description('LLM client management and stats');

  cmd
    .command('setup')
    .description('Authenticate Fort with your Claude subscription')
    .option('--api-key', 'Set up with an API key instead of Claude subscription')
    .action(async (opts) => {
      await withFort(async (fort) => {
        if (fort.llm.isConfigured) {
          const authLabel =
            fort.llm.authMethod === 'dotenv' ? `~/.fort/.env` :
            fort.llm.authMethod === 'api_key_config' ? 'config file API key' :
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
          console.log(dim('  Or use an API key: ' + cyan('fort llm setup --api-key')) + '\n');
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
        if (stats.authMethod) {
          const authLabel =
            stats.authMethod === 'dotenv' ? `~/.fort/.env` :
            stats.authMethod === 'api_key_config' ? 'Config file API key' :
            'ANTHROPIC_API_KEY environment variable';
          console.log(`  Auth:           ${authLabel}`);
        }
        console.log(`  Default Model:  ${stats.defaultTier}`);
        console.log();

        console.log(bold('  Models:'));
        for (const [tier, model] of Object.entries(stats.models)) {
          console.log(`    ${cyan(tier.padEnd(10))} ${model.model}`);
          console.log(`    ${' '.repeat(10)} ${dim(model.description)}`);
        }
        console.log();

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
    .description('List available model configurations')
    .action(async () => {
      await withFort(async (fort) => {
        const models = fort.llm.getModels();

        console.log(bold('\n  Model Routing Configuration\n'));

        for (const [tier, config] of Object.entries(models)) {
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

  return cmd;
}
