import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, green, red, yellow, timeAgo } from '../utils/format.js';

export function createRoutinesCommand(): Command {
  const cmd = new Command('routines')
    .description('Routine management');

  cmd
    .command('list')
    .description('List all routines with schedule and last run info')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const list = fort.routines.listRoutines();

        if (opts.json) {
          console.log(JSON.stringify(list, null, 2));
          return;
        }

        console.log(bold('\n  Routines\n'));

        if (list.length === 0) {
          console.log(dim('  No routines configured.\n'));
          return;
        }

        for (const r of list) {
          const enabledIcon = r.enabled ? green('ON') : red('OFF');
          const lastRun = r.lastRun ? timeAgo(new Date(r.lastRun)) : 'never';
          const lastStatus = r.lastResult === 'success'
            ? green('OK')
            : r.lastResult === 'failure'
              ? red('FAIL')
              : dim('n/a');

          console.log(`  ${enabledIcon} ${bold(r.name)} ${dim(`[${r.schedule}]`)}`);
          console.log(`    ${r.description || dim('no description')}`);
          console.log(`    ${dim(`Last: ${lastRun} (${lastStatus}) | Steps: ${r.steps.length} | ID: ${r.id.slice(0, 8)}...`)}`);
          if (r.behaviors && r.behaviors.length > 0) {
            console.log(`    ${dim(`Behaviors: ${r.behaviors.join(', ')}`)}`);
          }
          console.log();
        }
      });
    });

  cmd
    .command('add')
    .description('Add a new routine')
    .requiredOption('--name <name>', 'Routine name')
    .requiredOption('--schedule <cron>', 'Cron expression (e.g., "0 7 * * *")')
    .option('--description <desc>', 'Description', '')
    .option('--steps <json>', 'Steps as JSON array', '[]')
    .option('--source <source>', 'Source', 'user')
    .action(async (opts) => {
      await withFort(async (fort) => {
        let steps;
        try {
          steps = JSON.parse(opts.steps);
        } catch {
          console.log(red('\n  Invalid JSON for --steps\n'));
          return;
        }

        const routine = fort.routines.addRoutine({
          name: opts.name,
          description: opts.description,
          schedule: opts.schedule,
          steps,
          source: opts.source,
        });

        console.log(green(`\n  Routine added: ${routine.id.slice(0, 8)}...`));
        console.log(`  Name: ${bold(routine.name)}`);
        console.log(`  Schedule: ${routine.schedule}`);
        console.log(`  Steps: ${routine.steps.length}\n`);
      });
    });

  cmd
    .command('run')
    .description('Manually trigger a routine')
    .argument('<id>', 'Routine ID (or prefix)')
    .action(async (id) => {
      await withFort(async (fort) => {
        const allRoutines = fort.routines.listRoutines();
        const match = allRoutines.find((r) => r.id === id || r.id.startsWith(id));

        if (!match) {
          console.log(red(`\n  Routine not found: ${id}\n`));
          return;
        }

        console.log(dim(`\n  Executing routine: ${match.name}...`));
        const execution = await fort.routines.executeRoutine(match.id);

        if (execution.result === 'success') {
          console.log(green(`  Completed successfully.`));
        } else {
          console.log(red(`  Failed: ${execution.error}`));
        }
        console.log(`  ${dim(`Task: ${execution.taskId.slice(0, 8)}... | Duration: ${execution.completedAt}`)}\n`);
      });
    });

  cmd
    .command('history')
    .description('Show execution history for a routine')
    .argument('<id>', 'Routine ID (or prefix)')
    .option('--limit <n>', 'Max entries', '20')
    .action(async (id, opts) => {
      await withFort(async (fort) => {
        const allRoutines = fort.routines.listRoutines();
        const match = allRoutines.find((r) => r.id === id || r.id.startsWith(id));

        if (!match) {
          console.log(red(`\n  Routine not found: ${id}\n`));
          return;
        }

        const history = fort.routines.getHistory(match.id, parseInt(opts.limit, 10));

        console.log(bold(`\n  Execution History: ${match.name}\n`));

        if (history.length === 0) {
          console.log(dim('  No executions recorded.\n'));
          return;
        }

        for (const exec of history) {
          const resultIcon = exec.result === 'success' ? green('OK') : red('FAIL');
          const started = new Date(exec.startedAt).toLocaleString();

          console.log(`  ${resultIcon} ${dim(started)} ${dim(`Task: ${exec.taskId.slice(0, 8)}...`)}`);
          if (exec.error) {
            console.log(`    ${red(exec.error)}`);
          }
        }
        console.log();
      });
    });

  cmd
    .command('enable')
    .description('Enable a routine')
    .argument('<id>', 'Routine ID (or prefix)')
    .action(async (id) => {
      await withFort(async (fort) => {
        const allRoutines = fort.routines.listRoutines();
        const match = allRoutines.find((r) => r.id === id || r.id.startsWith(id));

        if (!match) {
          console.log(red(`\n  Routine not found: ${id}\n`));
          return;
        }

        fort.routines.enableRoutine(match.id);
        console.log(green(`\n  Routine enabled: ${match.name}\n`));
      });
    });

  cmd
    .command('disable')
    .description('Disable a routine')
    .argument('<id>', 'Routine ID (or prefix)')
    .action(async (id) => {
      await withFort(async (fort) => {
        const allRoutines = fort.routines.listRoutines();
        const match = allRoutines.find((r) => r.id === id || r.id.startsWith(id));

        if (!match) {
          console.log(red(`\n  Routine not found: ${id}\n`));
          return;
        }

        fort.routines.disableRoutine(match.id);
        console.log(yellow(`\n  Routine disabled: ${match.name}\n`));
      });
    });

  return cmd;
}
