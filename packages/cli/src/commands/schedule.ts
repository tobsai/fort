import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, statusIcon, dim, green, red, timeAgo } from '../utils/format.js';

export function createScheduleCommand(): Command {
  const cmd = new Command('schedule')
    .description('Scheduler management');

  cmd
    .command('list')
    .description('List all scheduled routines')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const routines = fort.scheduler.listRoutines();
        const triggers = fort.scheduler.listTriggers();

        if (opts.json) {
          console.log(JSON.stringify({ routines: routines.map(({ handler: _h, ...r }: any) => r), triggers }, null, 2));
          return;
        }

        console.log(bold('\n  Scheduled Routines\n'));

        if (routines.length === 0) {
          console.log(dim('  No routines scheduled.\n'));
        } else {
          for (const routine of routines) {
            const enabledIcon = routine.enabled ? green('ON') : red('OFF');
            const lastRun = routine.lastRunAt ? timeAgo(routine.lastRunAt) : 'never';
            const lastStatus = routine.lastResult === 'success' ? green('OK') : routine.lastResult === 'failure' ? red('FAIL') : dim('n/a');

            console.log(`  ${enabledIcon} ${bold(routine.name)} ${dim(`[${routine.cronExpression}]`)}`);
            console.log(`    ${routine.description || dim('no description')}`);
            console.log(`    ${dim(`Last: ${lastRun} (${lastStatus}) | Runs: ${routine.runCount} | Fails: ${routine.failCount}`)}`);
            console.log();
          }
        }

        if (triggers.length > 0) {
          console.log(bold('  Event Triggers\n'));
          for (const trigger of triggers) {
            const enabledIcon = trigger.enabled ? green('ON') : red('OFF');
            console.log(`  ${enabledIcon} ${bold(trigger.name)} ${dim(`→ ${trigger.eventType}`)}`);
          }
          console.log();
        }
      });
    });

  return cmd;
}
