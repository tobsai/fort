import { Command } from 'commander';
import { execSync, spawnSync } from 'node:child_process';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, green, yellow, cyan } from '../utils/format.js';

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
            fort.llm.authMethod === 'claude_code_oauth' ? 'Claude Code session token' :
            fort.llm.authMethod === 'claude_code_keychain' ? 'Claude Code subscription (keychain)' :
            fort.llm.authMethod === 'api_key_config' ? 'config file API key' :
            'ANTHROPIC_API_KEY environment variable';
          console.log(bold('\n  LLM Already Configured\n'));
          console.log(`  ${green('✓')} Authenticated via ${authLabel}.`);
          console.log(dim('  Run `fort llm status` to see model details and usage.\n'));
          return;
        }

        if (opts.apiKey) {
          // API key flow
          console.log(bold('\n  API Key Setup\n'));
          console.log('  Create an API key at:\n');
          console.log(`    ${cyan('https://console.anthropic.com/settings/keys')}\n`);
          console.log('  ' + yellow('Note:') + ' API usage is billed separately from any Claude subscription.');
          console.log('  Set up billing at: ' + cyan('https://console.anthropic.com/settings/billing') + '\n');
          console.log('  Then add to your shell profile ' + dim('(~/.zshrc, ~/.bashrc)') + ':\n');
          console.log(`    ${cyan('export ANTHROPIC_API_KEY="sk-ant-your-key-here"')}\n`);
          console.log('  Or set it in ' + dim('.fort/config.yaml') + ':\n');
          console.log(`    ${dim('llm:')}`);
          console.log(`    ${dim('  apiKey: "sk-ant-your-key-here"')}\n`);
          return;
        }

        // Check if Claude CLI is available
        let hasClaude = false;
        try {
          execSync('which claude', { stdio: 'ignore' });
          hasClaude = true;
        } catch {
          // not found
        }

        if (!hasClaude) {
          console.log(bold('\n  Claude CLI not found\n'));
          console.log('  Fort authenticates through the Claude CLI. Install it first:\n');
          console.log(`    ${cyan('npm install -g @anthropic-ai/claude-code')}\n`);
          console.log('  Then run ' + cyan('fort llm setup') + ' again.\n');
          console.log(dim('  Alternatively, use an API key: ' + cyan('fort llm setup --api-key')) + '\n');
          return;
        }

        // Run claude setup-token, inheriting stdio so the user can interact
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

        // Verify the token landed in the keychain by reading it back
        let tokenFound = false;
        try {
          const keychainResult = execSync(
            'security find-generic-password -s "Claude Code-credentials" -w 2>/dev/null',
            { encoding: 'utf-8', stdio: ['pipe', 'pipe', 'pipe'] },
          ).trim();
          tokenFound = keychainResult.length > 0;
        } catch {
          // Keychain read failed — might need different service name
        }

        if (tokenFound) {
          console.log(`\n  ${green('✓')} Authentication successful!\n`);
          console.log('  Fort will automatically use your Claude subscription token.');
          console.log(`  Verify with: ${cyan('fort llm status')}\n`);
        } else {
          console.log(`\n  ${green('✓')} Setup complete.`);
          console.log(`  Run ${cyan('fort llm status')} to verify authentication.\n`);
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
            stats.authMethod === 'claude_code_oauth' ? 'Claude Code session token' :
            stats.authMethod === 'claude_code_keychain' ? 'Claude Code subscription (keychain)' :
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
          console.log(`  ${yellow('Not authenticated.')} Run ${cyan('claude setup-token')} or see ${cyan('fort llm setup')} for options.\n`);
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
          console.error(`\n  ${yellow('LLM not configured.')} Run ${cyan('claude setup-token')} or see ${cyan('fort llm setup')} for options.\n`);
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
