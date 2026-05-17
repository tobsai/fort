import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, green, red, yellow } from '../utils/format.js';

export function createReflectionCommand(): Command {
  const cmd = new Command('reflection').description(
    'The Reflection service: periodic goal-review and nudges',
  );

  cmd
    .command('status')
    .description('Show whether reflection is enabled and config')
    .action(async () => {
      await withFort(async (fort) => {
        const config = fort.reflection.getConfig();
        console.log(bold('\n  Reflection\n'));
        console.log(
          `    ${config.enabled ? green('● on ') : red('● off')}  ${dim('(toggle with')} ${green('fort reflection on/off')}${dim(')')}`,
        );
        console.log(
          `    ${dim('stale threshold:')} ${config.scoring.staleThresholdDays}d`,
        );
        console.log(
          `    ${dim('cooldown:')}        ${config.scoring.cooldownHours}h between nudges per goal`,
        );
        console.log();
      });
    });

  cmd
    .command('on')
    .description('Enable the reflection loop')
    .action(async () => {
      await withFort(async (fort) => {
        fort.reflection.setEnabled(true);
        console.log(green('\n  Reflection enabled.\n'));
      });
    });

  cmd
    .command('off')
    .description('Disable the reflection loop (no nudges or drafts)')
    .action(async () => {
      await withFort(async (fort) => {
        fort.reflection.setEnabled(false);
        console.log(yellow('\n  Reflection disabled. No nudges will fire.\n'));
      });
    });

  cmd
    .command('run')
    .description('Run a goal review pass now')
    .option('--agent <agentId>', 'Limit to a single agent')
    .option('--json', 'Output JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const result = await fort.reflection.reviewGoals(
          opts.agent ? [opts.agent] : undefined,
        );
        if (opts.json) {
          console.log(JSON.stringify(result, null, 2));
          return;
        }
        console.log(bold('\n  Goal review\n'));
        console.log(`    ${dim('reviewed:')} ${result.reviewedGoals}`);
        for (const action of result.actions) {
          if (action.type === 'skip') continue;
          if (action.type === 'nudge') {
            console.log(`    ${yellow('•')} ${bold('nudge')}  ${dim(`(goal ${action.goalId.slice(0, 8)})`)}`);
            console.log(`      ${dim(action.message)}`);
          } else {
            console.log(`    ${green('•')} ${bold('draft task')}  ${dim(`(goal ${action.goalId.slice(0, 8)})`)}`);
            console.log(`      ${bold(action.title)}`);
            if (action.description && action.description !== action.title) {
              console.log(`      ${dim(action.description)}`);
            }
          }
        }
        console.log(`\n  ${dim(result.summary)}\n`);
      });
    });

  return cmd;
}
