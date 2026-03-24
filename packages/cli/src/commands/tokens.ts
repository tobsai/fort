import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, cyan, yellow, green, red, table } from '../utils/format.js';

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(2)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

function formatCost(n: number): string {
  return `$${n.toFixed(4)}`;
}

export function createTokensCommand(): Command {
  const cmd = new Command('tokens')
    .description('Token usage tracking and budget management');

  cmd
    .command('stats')
    .description('Show current usage stats')
    .option('--by-agent', 'Breakdown by agent')
    .option('--by-model', 'Breakdown by model')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const stats = fort.tokens.getStats();

        if (opts.json) {
          console.log(JSON.stringify(stats, null, 2));
          return;
        }

        console.log(bold('\n  Token Usage\n'));

        const rows = [
          [dim('Period'), dim('Tokens'), dim('Cost'), dim('Calls')],
          ['Today', formatTokens(stats.today.tokens), formatCost(stats.today.cost), String(stats.today.calls)],
          ['This Month', formatTokens(stats.thisMonth.tokens), formatCost(stats.thisMonth.cost), String(stats.thisMonth.calls)],
          ['All Time', formatTokens(stats.allTime.tokens), formatCost(stats.allTime.cost), String(stats.allTime.calls)],
        ];
        console.log('  ' + table(rows).split('\n').join('\n  '));
        console.log();

        if (opts.byModel) {
          console.log(bold('  By Model\n'));
          const modelRows = [
            [dim('Model'), dim('Tokens'), dim('Cost'), dim('Calls')],
          ];
          for (const [model, data] of Object.entries(stats.byModel as Record<string, any>)) {
            modelRows.push([cyan(model), formatTokens(data.tokens), formatCost(data.cost), String(data.calls)]);
          }
          if (modelRows.length === 1) {
            console.log(dim('  No model data.\n'));
          } else {
            console.log('  ' + table(modelRows).split('\n').join('\n  '));
            console.log();
          }
        }

        if (opts.byAgent) {
          console.log(bold('  By Agent\n'));
          const agentRows = [
            [dim('Agent'), dim('Tokens'), dim('Cost'), dim('Calls')],
          ];
          for (const [agent, data] of Object.entries(stats.byAgent as Record<string, any>)) {
            agentRows.push([cyan(agent), formatTokens(data.tokens), formatCost(data.cost), String(data.calls)]);
          }
          if (agentRows.length === 1) {
            console.log(dim('  No agent data.\n'));
          } else {
            console.log('  ' + table(agentRows).split('\n').join('\n  '));
            console.log();
          }
        }
      });
    });

  cmd
    .command('budget')
    .description('Show budget status and remaining allowance')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const budget = fort.tokens.getBudget();
        const status = fort.tokens.checkBudget();

        if (opts.json) {
          console.log(JSON.stringify({ budget, status }, null, 2));
          return;
        }

        console.log(bold('\n  Token Budget\n'));

        console.log(`  Daily limit:     ${formatTokens(budget.dailyLimit)}`);
        console.log(`  Monthly limit:   ${formatTokens(budget.monthlyLimit)}`);
        console.log(`  Per-task limit:  ${formatTokens(budget.perTaskLimit)}`);
        console.log(`  Warning at:      ${(budget.warningThreshold * 100).toFixed(0)}%`);
        console.log();

        const statusLabel = status.withinBudget ? green('Within Budget') : red('EXCEEDED');
        console.log(`  Status:            ${statusLabel}`);
        console.log(`  Daily remaining:   ${formatTokens(status.dailyRemaining)}`);
        console.log(`  Monthly remaining: ${formatTokens(status.monthlyRemaining)}`);

        if (status.warnings.length > 0) {
          console.log();
          console.log(yellow('  Warnings:'));
          for (const w of status.warnings) {
            console.log(`    ${yellow('!')} ${w}`);
          }
        }

        console.log();
      });
    });

  const budgetSet = new Command('budget-set')
    .description('Set budget limits')
    .option('--daily <tokens>', 'Daily token limit', parseInt)
    .option('--monthly <tokens>', 'Monthly token limit', parseInt)
    .option('--per-task <tokens>', 'Per-task token limit', parseInt)
    .option('--warning <threshold>', 'Warning threshold (0-1)', parseFloat)
    .action(async (opts) => {
      await withFort(async (fort) => {
        const updates: Record<string, number> = {};
        if (opts.daily !== undefined) updates.dailyLimit = opts.daily;
        if (opts.monthly !== undefined) updates.monthlyLimit = opts.monthly;
        if (opts.perTask !== undefined) updates.perTaskLimit = opts.perTask;
        if (opts.warning !== undefined) updates.warningThreshold = opts.warning;

        if (Object.keys(updates).length === 0) {
          console.log(dim('\n  No budget options specified. Use --daily, --monthly, --per-task, or --warning.\n'));
          return;
        }

        fort.tokens.setBudget(updates);

        const budget = fort.tokens.getBudget();
        console.log(bold('\n  Budget Updated\n'));
        console.log(`  Daily limit:     ${formatTokens(budget.dailyLimit)}`);
        console.log(`  Monthly limit:   ${formatTokens(budget.monthlyLimit)}`);
        console.log(`  Per-task limit:  ${formatTokens(budget.perTaskLimit)}`);
        console.log(`  Warning at:      ${(budget.warningThreshold * 100).toFixed(0)}%`);
        console.log();
      });
    });

  cmd.addCommand(budgetSet);

  // Default action: show stats
  cmd.action(async () => {
    await withFort(async (fort) => {
      const stats = fort.tokens.getStats();

      console.log(bold('\n  Token Usage\n'));

      const rows = [
        [dim('Period'), dim('Tokens'), dim('Cost'), dim('Calls')],
        ['Today', formatTokens(stats.today.tokens), formatCost(stats.today.cost), String(stats.today.calls)],
        ['This Month', formatTokens(stats.thisMonth.tokens), formatCost(stats.thisMonth.cost), String(stats.thisMonth.calls)],
        ['All Time', formatTokens(stats.allTime.tokens), formatCost(stats.allTime.cost), String(stats.allTime.calls)],
      ];
      console.log('  ' + table(rows).split('\n').join('\n  '));
      console.log();
    });
  });

  return cmd;
}
