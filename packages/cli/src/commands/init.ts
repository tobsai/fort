import { Command } from 'commander';
import { existsSync, mkdirSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { homedir } from 'node:os';
import { createInterface } from 'node:readline';
import { spawnSync, execSync, spawn } from 'node:child_process';
import { bold, dim, green, cyan, yellow, magenta, blue } from '../utils/format.js';

// Fort-shaped castle ASCII art — detailed and colorful
function getBanner(): string {
  const m = (s: string) => magenta(s);
  const c = (s: string) => bold(cyan(s));
  const y = (s: string) => yellow(s);
  const b = (s: string) => blue(s);
  const g = (s: string) => green(s);
  const d = (s: string) => dim(s);
  const w = (s: string) => bold(s);

  return `
${d('     ▄▀▀▀▄')}    ${y('⚑')}    ${d('▄▀▀▀▄')}           ${d('▄▀▀▀▄')}    ${y('⚑')}    ${d('▄▀▀▀▄')}
${d('     █   █')}  ${m('▄███▄')}  ${d('█   █')}           ${d('█   █')}  ${m('▄███▄')}  ${d('█   █')}
${d('     █   █')}  ${m('█████')}  ${d('█   █')}           ${d('█   █')}  ${m('█████')}  ${d('█   █')}
${d('     █▄▄▄█▄▄')}${m('█████')}${d('▄▄█▄▄▄█')}${d('▄▄▄▄▄▄▄▄▄')}${d('█▄▄▄█▄▄')}${m('█████')}${d('▄▄█▄▄▄█')}
${d('     ███████████████████████████████████████████')}
${d('     ██')}${b('╔═════════════════════════════════════╗')}${d('██')}
${d('     ██')}${b('║')}                                     ${b('║')}${d('██')}
${d('     ██')}${b('║')}  ${c('███████╗ ██████╗ ██████╗ ████████╗')} ${b('║')}${d('██')}
${d('     ██')}${b('║')}  ${c('██╔════╝██╔═══██╗██╔══██╗╚══██╔══╝')} ${b('║')}${d('██')}
${d('     ██')}${b('║')}  ${c('█████╗  ██║   ██║██████╔╝   ██║')}    ${b('║')}${d('██')}
${d('     ██')}${b('║')}  ${c('██╔══╝  ██║   ██║██╔══██╗   ██║')}    ${b('║')}${d('██')}
${d('     ██')}${b('║')}  ${c('██║     ╚██████╔╝██║  ██║   ██║')}    ${b('║')}${d('██')}
${d('     ██')}${b('║')}  ${c('╚═╝      ╚═════╝ ╚═╝  ╚═╝   ╚═╝')}    ${b('║')}${d('██')}
${d('     ██')}${b('║')}                                     ${b('║')}${d('██')}
${d('     ██')}${b('║')}   ${g('A self-improving AI agent platform')} ${b('║')}${d('██')}
${d('     ██')}${b('║')}                                     ${b('║')}${d('██')}
${d('     ██')}${b('╚═════════════════════════════════════╝')}${d('██')}
${d('     ███████████████████████████████████████████')}
${d('     ██')}   ${m('██')}   ${d('██')}  ${y('▓▓▓▓')}  ${d('██')}    ${d('██')}  ${y('▓▓▓▓')}  ${d('██')}   ${m('██')}   ${d('██')}
${d('     ██')}   ${m('██')}   ${d('██')}  ${y('▓▓▓▓')}  ${d('██')}    ${d('██')}  ${y('▓▓▓▓')}  ${d('██')}   ${m('██')}   ${d('██')}
${d('     ██')}   ${m('██')}   ${d('██')}  ${y('▓▓▓▓')}  ${d('██')}    ${d('██')}  ${y('▓▓▓▓')}  ${d('██')}   ${m('██')}   ${d('██')}
${d('   ▄▄██▄▄▄██▄▄▄██▄▄▓▓▓▓▄▄██▄▄▄▄██▄▄▓▓▓▓▄▄██▄▄▄██▄▄██▄▄')}
${d('   ████████████████████████████████████████████████████████')}
${d('   ▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀')}
`;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function promptUser(question: string): Promise<string> {
  const rl = createInterface({ input: process.stdin, output: process.stdout });
  return new Promise((resolve) => {
    rl.question(question, (answer) => {
      rl.close();
      resolve(answer.trim());
    });
  });
}


export function createInitCommand(): Command {
  return new Command('init')
    .description('Initialize Fort and run first-time setup')
    .option('--skip-risks', 'Skip the risk acknowledgment prompt')
    .action(async (opts) => {
      console.log(getBanner());
      await sleep(2000);

      // ─── Risk Acknowledgment ──────────────────────────────────
      const fortDir = join(homedir(), '.fort');
      const alreadyAcknowledged = existsSync(join(fortDir, '.risks-acknowledged'));

      if (!alreadyAcknowledged && !opts.skipRisks) {
        console.log(bold(yellow('  ⚠  Important: Please read before continuing\n')));
        console.log(`  Fort is an ${bold('autonomous AI agent platform')} that can:`);
        console.log();
        console.log(`    ${yellow('•')} Execute actions on your behalf (email, calendar, messages)`);
        console.log(`    ${yellow('•')} Read and write files on your local machine`);
        console.log(`    ${yellow('•')} Make API calls using your credentials`);
        console.log(`    ${yellow('•')} Create, modify, and run scheduled tasks`);
        console.log(`    ${yellow('•')} Interact with external services you connect`);
        console.log();
        console.log(`  Fort uses a ${bold('tiered permission model')} to gate dangerous actions,`);
        console.log(`  but you should understand that:`);
        console.log();
        console.log(`    ${dim('1.')} AI agents can make mistakes — always review before approving`);
        console.log(`    ${dim('2.')} Your API key grants Fort access to Claude on your behalf`);
        console.log(`    ${dim('3.')} Data is stored locally in ${cyan('~/.fort/')} — you own it`);
        console.log(`    ${dim('4.')} Fort is open source — audit the code at any time`);
        console.log(`    ${dim('5.')} You can reset everything with ${cyan('fort reset --hard')}`);
        console.log();

        const answer = await promptUser(`  ${bold('Do you acknowledge these risks and want to continue?')} ${dim('[y/N]')} `);

        if (answer.toLowerCase() !== 'y' && answer.toLowerCase() !== 'yes') {
          console.log(`\n  ${dim('Setup cancelled. Run')} ${cyan('fort init')} ${dim('when you\'re ready.')}\n`);
          process.exit(0);
        }

        // Persist acknowledgment so we don't ask again
        if (!existsSync(fortDir)) mkdirSync(fortDir, { recursive: true });
        const { writeFileSync } = await import('node:fs');
        writeFileSync(
          join(fortDir, '.risks-acknowledged'),
          `Acknowledged: ${new Date().toISOString()}\n`,
          'utf-8',
        );
        console.log(`\n  ${green('✓')} Acknowledged. Let's get started.\n`);
      }

      // ─── Setup ────────────────────────────────────────────────
      const dataDir = join(fortDir, 'data');
      const agentsDir = join(fortDir, 'agents');

      // Step 1: Create directories
      console.log(bold('  Step 1: Create Fort directories\n'));
      for (const dir of [fortDir, dataDir, agentsDir]) {
        if (!existsSync(dir)) {
          mkdirSync(dir, { recursive: true });
          console.log(`    ${green('✓')} Created ${dim(dir)}`);
        } else {
          console.log(`    ${green('✓')} ${dim(dir)} ${dim('(exists)')}`);
        }
      }
      console.log();

      // Step 2: LLM setup
      console.log(bold('  Step 2: Connect to Claude\n'));

      const { LLMClient } = await import('@fort/core');
      const testClient = new LLMClient(
        {},
        { publish: () => {}, subscribe: () => () => {}, clear: () => {} } as any,
      );

      // Helper: validate a configured client and report
      const validateAndReport = async () => {
        const client = new LLMClient(
          {},
          { publish: () => {}, subscribe: () => () => {}, clear: () => {} } as any,
        );
        if (!client.isConfigured) return false;
        console.log(`    ${dim('Validating...')}`);
        const error = await client.validateAuth();
        if (error) {
          console.log(`    ${yellow('⚠')} ${error}`);
        } else {
          console.log(`    ${green('✓')} Authentication validated!`);
          console.log(dim(`    Token stored in: ${LLMClient.envFilePath}`));
        }
        return true;
      };

      // Helper: extract keychain token → .env
      const extractKeychain = (): boolean => {
        const token = LLMClient.readKeychainToken();
        if (token) {
          LLMClient.writeEnvFile(token);
          return true;
        }
        return false;
      };

      if (testClient.isConfigured) {
        // .env already has a token (or ANTHROPIC_API_KEY env var is set)
        console.log(`    ${green('✓')} Found credentials in ${dim(LLMClient.envFilePath)}`);
        await validateAndReport();
      } else {
        // No .env token — check if keychain has one from a previous `claude setup-token`
        if (extractKeychain()) {
          console.log(`    ${green('✓')} Extracted token from Claude Code keychain to .env`);
          await validateAndReport();
        } else {
          // No keychain token either — need to set one up
          console.log(`    ${yellow('○')} No API credentials found.\n`);

          let hasClaude = false;
          try {
            execSync('which claude', { stdio: 'ignore' });
            hasClaude = true;
          } catch {}

          if (hasClaude) {
            console.log(`    Run ${cyan('claude setup-token')} to authenticate via your Claude subscription.`);
            console.log(`    Or paste an API key from ${cyan('https://console.anthropic.com/settings/keys')}\n`);

            const choice = await promptUser(`    ${bold('Run claude setup-token?')} ${dim('[Y/n]')} `);

            if (choice.toLowerCase() !== 'n') {
              console.log();
              const result = spawnSync('claude', ['setup-token'], {
                stdio: 'inherit',
                env: { ...process.env },
              });

              if (result.status === 0 && extractKeychain()) {
                console.log();
                await validateAndReport();
              } else {
                console.log(`\n    ${yellow('⚠')} Token not detected. Try ${cyan('fort llm setup')} later.`);
              }
            } else {
              const apiKeyInput = await promptUser(`\n    ${bold('Paste your API key (sk-ant-...):')} `);
              if (apiKeyInput && apiKeyInput.startsWith('sk-ant-')) {
                LLMClient.writeEnvFile(apiKeyInput);
                await validateAndReport();
              } else {
                console.log(`\n    ${dim('Skipped. Run')} ${cyan('fort llm setup')} ${dim('later.')}`);
              }
            }
          } else {
            console.log(`    Fort requires an API key to power your agents.\n`);
            console.log(`    Option 1: Install Claude Code (${cyan('npm i -g @anthropic-ai/claude-code')})`);
            console.log(`             then run ${cyan('fort llm setup')} to authenticate`);
            console.log(`    Option 2: Paste an API key from ${cyan('https://console.anthropic.com/settings/keys')}\n`);

            const apiKeyInput = await promptUser(`    ${bold('Paste your API key (sk-ant-...):')} `);
            if (apiKeyInput && apiKeyInput.startsWith('sk-ant-')) {
              LLMClient.writeEnvFile(apiKeyInput);
              await validateAndReport();
            } else {
              console.log(`\n    ${dim('Skipped. Run')} ${cyan('fort llm setup')} ${dim('later.')}`);
            }
          }
        }
      }
      console.log();

      // Step 3: Open portal for agent creation
      console.log(bold('  Step 3: Create your first agent\n'));

      const agentEntries = existsSync(agentsDir)
        ? readdirSync(agentsDir).filter((e) => !e.startsWith('.'))
        : [];

      if (agentEntries.length > 0) {
        console.log(`    ${green('✓')} ${agentEntries.length} agent${agentEntries.length > 1 ? 's' : ''} found`);
      } else {
        console.log(`    ${dim('Create your first agent in the Fort portal.')}`);
      }
      console.log();

      // Summary
      console.log(bold('  Ready!\n'));
      console.log(`    ${cyan('fort portal')}              ${dim('— Open the web portal')}`);
      console.log(`    ${cyan('fort doctor')}              ${dim('— Health check across all modules')}`);
      console.log(`    ${cyan('fort status')}              ${dim('— System overview')}`);
      console.log(`    ${cyan('fort ps')}                  ${dim('— Check running services')}`);
      console.log(`    ${cyan('fort agents create')}       ${dim('— Create another specialist agent')}`);
      console.log(`    ${cyan('fort llm ask "prompt"')}    ${dim('— Ask Claude a question')}`);
      console.log(`    ${cyan('fort stop')}                ${dim('— Stop all Fort services')}`);
      console.log(`    ${cyan('fort reset')}               ${dim('— Reset Fort data')}`);
      console.log();

      // Launch portal in the background
      console.log(`  ${dim('Opening Fort portal...')}\n`);
      try {
        const child = spawn('fort', ['portal'], {
          detached: true,
          stdio: 'ignore',
        });
        child.unref();
      } catch {
        // If fort isn't in PATH yet, just print instructions
        console.log(`    ${dim('Run')} ${cyan('fort portal')} ${dim('to open the web interface')}\n`);
      }
    });
}

/**
 * Check if this is the first time Fort has been run.
 */
export function isFirstRun(): boolean {
  return !existsSync(join(homedir(), '.fort', 'data'));
}

export { getBanner as BANNER };
