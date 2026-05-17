import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, green, red, yellow } from '../utils/format.js';

export function createGoalsCommand(): Command {
  const cmd = new Command('goals').description('Goal management — the things the agent works toward');

  cmd
    .command('list')
    .description('List goals (defaults to active)')
    .option('--agent <agentId>', 'Filter by agent (defaults to the default agent)')
    .option('--all', 'Include paused/achieved/abandoned')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const agentId = resolveAgentId(fort, opts.agent);
        if (!agentId) {
          console.log(red('\n  No agent found. Run `fort init` first.\n'));
          return;
        }

        const goals = opts.all
          ? fort.goals.listAll(agentId)
          : fort.goals.listForAgent(agentId, 'active');

        if (opts.json) {
          console.log(JSON.stringify(goals, null, 2));
          return;
        }

        console.log(bold(`\n  Goals (${opts.all ? 'all' : 'active'})\n`));
        if (goals.length === 0) {
          console.log(dim('  No goals yet. Add one with `fort goals add --title "…"`\n'));
          return;
        }

        for (const g of goals) {
          const statusColor =
            g.status === 'active' ? green : g.status === 'achieved' ? dim : yellow;
          console.log(`  ${statusColor(g.status.padEnd(9))} ${bold(g.title)}  ${dim(`#${g.id.slice(0, 8)}`)}`);
          if (g.description) console.log(`    ${dim(g.description)}`);
          if (g.lastActivityAt) {
            const days = Math.floor((Date.now() - g.lastActivityAt.getTime()) / 86400000);
            console.log(`    ${dim(`last activity: ${days}d ago`)}`);
          }
          console.log();
        }
      });
    });

  cmd
    .command('add')
    .description('Add a new goal')
    .requiredOption('--title <title>', 'Short goal title')
    .option('--description <text>', 'Longer description')
    .option('--agent <agentId>', 'Agent this goal belongs to (defaults to the default agent)')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const agentId = resolveAgentId(fort, opts.agent);
        if (!agentId) {
          console.log(red('\n  No agent found. Run `fort init` first.\n'));
          return;
        }
        const goal = fort.goals.create({
          agentId,
          title: opts.title,
          description: opts.description ?? null,
          source: 'user',
        });
        console.log(green(`\n  Goal added: ${bold(goal.title)}`));
        console.log(`  ${dim(`id: ${goal.id} | status: ${goal.status}`)}\n`);
      });
    });

  cmd
    .command('done')
    .description('Mark a goal achieved')
    .argument('<id>', 'Goal ID (or prefix)')
    .action(async (idArg) => {
      await withFort(async (fort) => {
        const goal = findGoalByIdPrefix(fort, idArg);
        if (!goal) {
          console.log(red(`\n  Goal not found: ${idArg}\n`));
          return;
        }
        const updated = fort.goals.achieve(goal.id);
        console.log(green(`\n  Marked achieved: ${bold(updated?.title ?? '')}\n`));
      });
    });

  cmd
    .command('pause')
    .description('Pause a goal (will not be flagged by reflection)')
    .argument('<id>', 'Goal ID (or prefix)')
    .action(async (idArg) => {
      await withFort(async (fort) => {
        const goal = findGoalByIdPrefix(fort, idArg);
        if (!goal) {
          console.log(red(`\n  Goal not found: ${idArg}\n`));
          return;
        }
        fort.goals.update(goal.id, { status: 'paused' });
        console.log(yellow(`\n  Paused: ${bold(goal.title)}\n`));
      });
    });

  cmd
    .command('resume')
    .description('Mark a paused goal active again')
    .argument('<id>', 'Goal ID (or prefix)')
    .action(async (idArg) => {
      await withFort(async (fort) => {
        const goal = findGoalByIdPrefix(fort, idArg);
        if (!goal) {
          console.log(red(`\n  Goal not found: ${idArg}\n`));
          return;
        }
        fort.goals.update(goal.id, { status: 'active' });
        console.log(green(`\n  Resumed: ${bold(goal.title)}\n`));
      });
    });

  cmd
    .command('remove')
    .description('Delete a goal entirely')
    .argument('<id>', 'Goal ID (or prefix)')
    .action(async (idArg) => {
      await withFort(async (fort) => {
        const goal = findGoalByIdPrefix(fort, idArg);
        if (!goal) {
          console.log(red(`\n  Goal not found: ${idArg}\n`));
          return;
        }
        fort.goals.delete(goal.id);
        console.log(red(`\n  Removed: ${goal.title}\n`));
      });
    });

  return cmd;
}

function resolveAgentId(fort: any, explicitId?: string): string | null {
  if (explicitId) return explicitId;
  const records = fort.agentStore.list();
  if (records.length === 0) return null;
  const def = records.find((r: any) => r.isDefault) ?? records[0];
  return def.id;
}

function findGoalByIdPrefix(fort: any, idArg: string): any | null {
  // Direct match
  const direct = fort.goals.get(idArg);
  if (direct) return direct;
  // Prefix match across all agents
  const records = fort.agentStore.list();
  for (const r of records) {
    const candidates = fort.goals.listAll(r.id);
    const match = candidates.find((g: any) => g.id.startsWith(idArg));
    if (match) return match;
  }
  return null;
}
