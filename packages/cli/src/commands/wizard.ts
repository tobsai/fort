/**
 * Interactive agent-creation wizard for the CLI.
 *
 * Mirrors the dashboard SetupWizard's flow:
 *   1. Name
 *   2. Goals
 *   3. Provider (numbered list, usable/disabled state)
 *   4. Model tier (Fast / Standard / Powerful with provider's model names)
 *   5. Emoji
 *   6. Submit
 *
 * Used by `fort init` (when no agents exist) and `fort agents create` (when
 * called without --name).
 */
import { createInterface } from 'node:readline';
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

const TIER_LABELS = {
  fast:     'Quick responses, simple tasks. Lowest cost.',
  standard: 'Balanced quality and speed. Best for most tasks.',
  powerful: 'Maximum reasoning. Complex planning and analysis.',
};

type ModelTier = 'fast' | 'standard' | 'powerful';
type ProviderId = 'anthropic' | 'openai' | 'grok' | 'groq' | 'google' | 'ollama' | 'openrouter';

interface AvailableProvider {
  id: ProviderId;
  name: string;
  usable: boolean;
  authMethod: string | null;
  models: { fast: string; standard: string; powerful: string };
  hint?: string;
}

function prompt(question: string): Promise<string> {
  const rl = createInterface({ input: process.stdin, output: process.stdout });
  return new Promise((resolve) => {
    rl.question(question, (answer) => { rl.close(); resolve(answer.trim()); });
  });
}

/**
 * Run the interactive agent-creation wizard. The `fort` argument is the live
 * Fort instance (used both to query available providers and to submit the
 * agent — either directly via AgentFactory, or via HTTP POST to the portal).
 *
 * When `opts.providerId` is supplied, the wizard skips the provider-picker
 * step and uses the given provider directly. This is used from `fort init`
 * where Step 2 already chose a provider, so Step 3 doesn't ask again.
 */
export async function runAgentWizard(
  fort: any,
  opts: { providerId?: ProviderId } = {},
): Promise<void> {
  console.log(bold('\n  New Agent Setup\n'));

  // Step 1: name
  const nameInput = await prompt(`    ${bold('Agent name')} ${dim('[Fort]')}: `);
  const name = nameInput || 'Fort';

  // Step 2: goals
  const goals = await prompt(`    ${bold('What should this agent help you with?')}\n      ${dim('(short description)')}: `);

  // Step 3: provider — inherited from caller (init Step 2) or pick interactively.
  const providers: AvailableProvider[] = await fort.llm.getAvailableProviders();
  let providerChoice: AvailableProvider | null = null;

  if (opts.providerId) {
    providerChoice = providers.find((p) => p.id === opts.providerId) ?? null;
    if (providerChoice) {
      console.log(`\n    ${dim('Provider:')} ${PROVIDER_ICONS[providerChoice.id]} ${bold(providerChoice.name)} ${dim('(inherited)')}`);
    }
  }

  if (!providerChoice) {
    console.log(bold('\n    Detected providers:\n'));
    providers.forEach((p, i) => {
      const num = `${i + 1}`.padStart(2, ' ');
      const icon = PROVIDER_ICONS[p.id] ?? '🔌';
      const status = p.usable ? green('✓ configured') : yellow('○ not configured');
      console.log(`      ${cyan(num + '.')} ${icon}  ${bold(p.name.padEnd(14))} ${status}`);
      if (!p.usable && p.hint) {
        console.log(`         ${dim(p.hint)}`);
      }
    });

    while (!providerChoice) {
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
      const candidate = providers[idx];
      if (!candidate.usable) {
        console.log(`    ${yellow('That provider is not configured.')} ${dim(candidate.hint ?? '')}`);
        continue;
      }
      providerChoice = candidate;
    }
  }

  // Step 4: model tier
  console.log(bold('\n    Model tiers:\n'));
  const tiers: ModelTier[] = ['fast', 'standard', 'powerful'];
  tiers.forEach((t, i) => {
    const num = cyan(`${i + 1}.`);
    console.log(`      ${num} ${bold(t.padEnd(10))} ${cyan(providerChoice!.models[t])}`);
    console.log(`         ${dim(TIER_LABELS[t])}`);
  });

  let modelTier: ModelTier | null = null;
  while (modelTier === null) {
    const answer = await prompt(`\n    ${bold('Pick a tier')} ${dim('[1-3]')}: `);
    if (!answer) {
      console.log(`    ${yellow('Type a number to pick a tier.')}`);
      continue;
    }
    const idx = parseInt(answer, 10) - 1;
    if (Number.isNaN(idx) || idx < 0 || idx > 2) {
      console.log(`    ${yellow('Invalid selection.')}`);
      continue;
    }
    modelTier = tiers[idx];
  }

  // Step 5: emoji
  const emojiInput = await prompt(`\n    ${bold('Emoji')} ${dim('[🏰]')}: `);
  const emoji = emojiInput || '🏰';

  // Confirm
  console.log(bold('\n  Summary:\n'));
  console.log(`    ${dim('Name:    ')} ${name}`);
  console.log(`    ${dim('Goals:   ')} ${goals || dim('(none)')}`);
  console.log(`    ${dim('Provider:')} ${PROVIDER_ICONS[providerChoice.id]} ${providerChoice.name}`);
  console.log(`    ${dim('Model:   ')} ${modelTier} (${providerChoice.models[modelTier]})`);
  console.log(`    ${dim('Emoji:   ')} ${emoji}\n`);

  const confirm = await prompt(`    ${bold('Create this agent?')} ${dim('[Y/n]')}: `);
  if (confirm.toLowerCase() === 'n' || confirm.toLowerCase() === 'no') {
    console.log(dim('\n    Cancelled.\n'));
    return;
  }

  // Submit — talk to portal if reachable, else create directly
  const payload = {
    name,
    goals,
    emoji,
    personality: '',
    avatarDataUrl: null,
    modelTier,
    provider: providerChoice.id,
  };

  let created = false;
  try {
    const res = await fetch('http://localhost:4077/api/agents/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const body = await res.json().catch(() => ({})) as { id?: string; error?: string };
      if (body.error) throw new Error(body.error);
      console.log(`\n    ${green('✓')} Agent created: ${cyan(body.id ?? name)}\n`);
      created = true;
    }
  } catch {
    // Portal unreachable — fall through to direct creation
  }

  if (!created) {
    try {
      const agent = fort.agentFactory.create({
        name,
        description: goals || 'A Fort specialist agent.',
        emoji,
      });
      const identity = agent.identity;
      (identity as any).isDefault = true;
      (identity as any).defaultModelTier = modelTier;
      (identity as any).provider = providerChoice.id;

      // Mirror the server's persistence: write identity.yaml + SOUL.md.
      const { join } = await import('node:path');
      const { writeFileSync, existsSync, mkdirSync } = await import('node:fs');
      const { stringify: stringifyYaml } = await import('yaml');
      const agentDir = join((fort.agentFactory as any).agentsDir, identity.id);
      if (!existsSync(agentDir)) mkdirSync(agentDir, { recursive: true });
      writeFileSync(join(agentDir, 'identity.yaml'), stringifyYaml(identity), 'utf-8');
      const soul = `# ${name}\n\n${goals || 'General-purpose assistant.'}\n\n## Goals\n${goals || 'General-purpose assistant.'}\n\n## Personality\nHelpful, concise, and action-oriented.\n\n## Rules\n- Every request should result in a clear action or response\n- Be transparent about limitations\n`;
      writeFileSync(join(agentDir, 'SOUL.md'), soul, 'utf-8');
      agent.refreshSoul();
      await agent.start();

      // Set provider as global default if its row exists.
      try {
        if (fort.llmProviders.getProvider(providerChoice.id)) {
          fort.llmProviders.setDefault(providerChoice.id);
        }
      } catch { /* noop */ }

      console.log(`\n    ${green('✓')} Agent created: ${cyan(identity.id)}\n`);
    } catch (err) {
      console.log(`\n    ${yellow('⚠')} Agent creation failed: ${err instanceof Error ? err.message : err}\n`);
    }
  }
}
