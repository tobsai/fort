import { Command } from 'commander';
import { existsSync, mkdirSync, readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { homedir } from 'node:os';
import { createInterface } from 'node:readline';
import { bold, dim, green, cyan, yellow, magenta } from '../utils/format.js';
import { withFort } from '../utils/fort-instance.js';
import { runAgentWizard } from './wizard.js';
import { pickProvider } from './provider-setup.js';

const LOGO_PATH = join(__dirname, '..', '..', 'assets', 'fort-logo.png');

/**
 * Detect whether the current terminal supports inline images, and which protocol.
 * Returns 'iterm' (OSC 1337), 'kitty' (Kitty graphics protocol), or null (use ASCII).
 */
function detectImageProtocol(): 'iterm' | 'kitty' | null {
  if (!process.stdout.isTTY) return null;
  const term = process.env.TERM ?? '';
  const termProgram = process.env.TERM_PROGRAM ?? '';

  // Kitty + Ghostty use the Kitty graphics protocol
  if (term === 'xterm-kitty' || term.startsWith('xterm-kitty')) return 'kitty';
  if (term === 'xterm-ghostty' || termProgram === 'ghostty') return 'kitty';

  // iTerm2 and WezTerm understand the iTerm2 inline-image protocol
  if (termProgram === 'iTerm.app') return 'iterm';
  if (termProgram === 'WezTerm') return 'iterm';

  return null;
}

/**
 * Emit an iTerm2 OSC 1337 inline-image sequence.
 * Width is in terminal cells; aspect ratio is preserved.
 */
function emitIterm2Image(buf: Buffer, widthCells: number): void {
  const b64 = buf.toString('base64');
  const args = [
    'inline=1',
    `width=${widthCells}`,
    'preserveAspectRatio=1',
    `size=${buf.length}`,
  ].join(';');
  process.stdout.write(`\x1b]1337;File=${args}:${b64}\x07\n`);
}

/**
 * Emit a Kitty graphics protocol inline image.
 * Splits the base64 payload into 4KB chunks per the protocol spec.
 */
function emitKittyImage(buf: Buffer, widthCells: number): void {
  const b64 = buf.toString('base64');
  const CHUNK = 4096;
  if (b64.length <= CHUNK) {
    process.stdout.write(`\x1b_Gf=100,a=T,c=${widthCells};${b64}\x1b\\`);
  } else {
    for (let i = 0; i < b64.length; i += CHUNK) {
      const chunk = b64.slice(i, i + CHUNK);
      const isLast = i + CHUNK >= b64.length;
      const headers = i === 0
        ? `f=100,a=T,c=${widthCells},m=${isLast ? 0 : 1}`
        : `m=${isLast ? 0 : 1}`;
      process.stdout.write(`\x1b_G${headers};${chunk}\x1b\\`);
    }
  }
  process.stdout.write('\n');
}

/**
 * Render the Fort banner. On terminals with inline-image support, prints the
 * PNG logo at ~52 cells wide. Elsewhere, prints a clean ANSI wordmark.
 */
function printBanner(): void {
  const protocol = detectImageProtocol();
  if (protocol && existsSync(LOGO_PATH)) {
    try {
      const buf = readFileSync(LOGO_PATH);
      process.stdout.write('\n');
      if (protocol === 'iterm') emitIterm2Image(buf, 52);
      else emitKittyImage(buf, 52);
      process.stdout.write('\n');
      return;
    } catch {
      // Fall through to ASCII
    }
  }
  process.stdout.write(asciiBanner());
}

/**
 * Sharp ASCII fallback — ANSI Shadow wordmark, no decorations.
 */
function asciiBanner(): string {
  const p = (s: string) => bold(magenta(s));
  const c = (s: string) => cyan(s);
  const d = (s: string) => dim(s);
  return `
   ${p('███████╗ ██████╗ ██████╗ ████████╗')}
   ${p('██╔════╝██╔═══██╗██╔══██╗╚══██╔══╝')}
   ${p('█████╗  ██║   ██║██████╔╝   ██║   ')}
   ${p('██╔══╝  ██║   ██║██╔══██╗   ██║   ')}
   ${p('██║     ╚██████╔╝██║  ██║   ██║   ')}
   ${p('╚═╝      ╚═════╝ ╚═╝  ╚═╝   ╚═╝   ')}

   ${c('A self-improving AI agent platform')}
   ${d('https://github.com/tobsai/fort')}

`;
}

// Kept for backward compatibility — re-exported as BANNER.
function getBanner(): string {
  return asciiBanner();
}

/**
 * Print the running CLI version + the binary path. Useful for diagnosing
 * stale installs (e.g. an nvm-linked dev copy shadowing the brew install).
 */
function printVersionLine(): void {
  let version = 'unknown';
  try {
    const pkgPath = join(__dirname, '..', '..', 'package.json');
    version = JSON.parse(readFileSync(pkgPath, 'utf-8')).version ?? 'unknown';
  } catch {
    // fall through with 'unknown'
  }
  const binPath = process.argv[1] ?? 'unknown';
  console.log(`   ${dim(`v${version}`)}  ${dim(`(${binPath})`)}\n`);
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
      printBanner();
      printVersionLine();
      await sleep(1500);

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

      // Steps 2 + 3 share a Fort instance for provider detection + agent creation.
      try {
        await withFort(async (fort) => {
          // Step 2: Pick a Provider
          console.log(bold('  Step 2: Pick a Provider\n'));
          console.log(`    ${dim('Fort detects each provider and shows its state. Pick one to use as default.')}`);
          console.log(`    ${dim('Picking an unconfigured provider runs its setup flow inline.')}\n`);
          const chosenProvider = await pickProvider(fort);

          console.log();

          // Step 3: Create the first agent — inherits the provider from Step 2
          console.log(bold('  Step 3: Create your first agent\n'));
          const agentEntries = existsSync(agentsDir)
            ? readdirSync(agentsDir).filter((e) => !e.startsWith('.'))
            : [];

          if (agentEntries.length > 0) {
            console.log(`    ${green('✓')} ${agentEntries.length} agent${agentEntries.length > 1 ? 's' : ''} already configured.`);
          } else {
            await runAgentWizard(fort, { providerId: chosenProvider ?? undefined });
          }

          // Step 4: Bootstrap the Triager agent (idempotent — does nothing if it exists).
          const triagerAssets = join(__dirname, '..', '..', 'assets', 'triager');
          const seeded = fort.agentFactory.seedTriagerIfMissing(triagerAssets);
          if (seeded) {
            console.log(`    ${green('✓')} Triager agent installed at ${dim('~/.fort/agents/triager/')}`);
            console.log(`    ${dim('  Edit SOUL.md to tune how it classifies chats as tasks vs questions.')}`);
          }
        });
      } catch (err) {
        console.log(`    ${yellow('⚠')} Setup interrupted: ${err instanceof Error ? err.message : err}`);
        console.log(`    ${dim('Run')} ${cyan('fort llm setup')} ${dim('and')} ${cyan('fort agents create')} ${dim('to finish manually.')}`);
      }
      console.log();

      // Summary
      console.log(bold('  Ready!\n'));
      console.log(`    ${green('→')} ${bold('Run')} ${cyan('fort portal')} ${bold('to meet your agent.')}`);
      console.log(`    ${dim('  Your agent will introduce itself and spend a few minutes getting to know')}`);
      console.log(`    ${dim('  you. From that conversation it sets up the goals it works toward.')}\n`);
      console.log(`    ${dim('Other useful commands:')}`);
      console.log(`    ${cyan('fort doctor')}              ${dim('— Health check across all modules')}`);
      console.log(`    ${cyan('fort status')}              ${dim('— System overview')}`);
      console.log(`    ${cyan('fort goals list')}          ${dim('— See what you are working toward')}`);
      console.log(`    ${cyan('fort agents create')}       ${dim('— Create another specialist agent')}`);
      console.log(`    ${cyan('fort stop')}                ${dim('— Stop all Fort services')}`);
      console.log();
    });
}

/**
 * Check if this is the first time Fort has been run.
 */
export function isFirstRun(): boolean {
  return !existsSync(join(homedir(), '.fort', 'data'));
}

export { getBanner as BANNER };
