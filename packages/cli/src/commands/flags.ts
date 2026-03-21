import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, cyan, yellow, green, red, table } from '../utils/format.js';

function statusColor(status: string): string {
  switch (status) {
    case 'baking': return yellow(status);
    case 'stable': return green(status);
    case 'rolled_back': return red(status);
    case 'disabled': return dim(status);
    default: return status;
  }
}

export function createFlagsCommand(): Command {
  const cmd = new Command('flags')
    .description('Feature flag management');

  cmd
    .command('list')
    .description('List all feature flags')
    .option('--status <status>', 'Filter by status (baking, stable, rolled_back, disabled)')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const flags = fort.flags.listFlags(opts.status);

        if (opts.json) {
          console.log(JSON.stringify(flags, null, 2));
          return;
        }

        if (flags.length === 0) {
          console.log(dim('\n  No feature flags found.\n'));
          return;
        }

        console.log(bold('\n  Feature Flags\n'));

        const rows = [
          [dim('ID'), dim('Name'), dim('Status'), dim('Enabled'), dim('Created')],
        ];

        for (const flag of flags) {
          rows.push([
            flag.id.slice(0, 8),
            cyan(flag.name),
            statusColor(flag.status),
            flag.enabled ? green('yes') : dim('no'),
            flag.createdAt.slice(0, 10),
          ]);
        }

        console.log('  ' + table(rows).split('\n').join('\n  '));
        console.log();
      });
    });

  cmd
    .command('create')
    .description('Create a new feature flag')
    .requiredOption('--name <name>', 'Flag name')
    .requiredOption('--description <description>', 'Flag description')
    .option('--bake-period <ms>', 'Bake period in milliseconds', parseInt)
    .option('--spec-id <id>', 'Linked spec ID')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const flag = fort.flags.createFlag(opts.name, opts.description, {
          bakePeriodMs: opts.bakePeriod,
          specId: opts.specId,
        });

        console.log(bold('\n  Flag Created\n'));
        console.log(`  ID:          ${cyan(flag.id)}`);
        console.log(`  Name:        ${flag.name}`);
        console.log(`  Description: ${flag.description}`);
        console.log(`  Status:      ${statusColor(flag.status)}`);
        console.log(`  Bake Period: ${flag.bakePeriodMs}ms`);
        console.log();
      });
    });

  cmd
    .command('enable <id>')
    .description('Enable a flag and start its bake period')
    .action(async (id) => {
      await withFort(async (fort) => {
        const flag = fort.flags.enableFlag(id);
        console.log(bold('\n  Flag Enabled\n'));
        console.log(`  ${cyan(flag.name)} is now ${statusColor(flag.status)}`);
        console.log(`  Bake period: ${flag.bakePeriodMs}ms`);
        console.log();
      });
    });

  cmd
    .command('disable <id>')
    .description('Disable a flag')
    .option('--reason <reason>', 'Reason for disabling')
    .action(async (id, opts) => {
      await withFort(async (fort) => {
        const flag = fort.flags.disableFlag(id, opts.reason);
        console.log(bold('\n  Flag Disabled\n'));
        console.log(`  ${cyan(flag.name)} is now ${statusColor(flag.status)}`);
        if (opts.reason) {
          console.log(`  Reason: ${opts.reason}`);
        }
        console.log();
      });
    });

  cmd
    .command('promote <id>')
    .description('Manually promote a flag to stable')
    .action(async (id) => {
      await withFort(async (fort) => {
        const flag = fort.flags.promote(id);
        console.log(bold('\n  Flag Promoted\n'));
        console.log(`  ${cyan(flag.name)} is now ${green('stable')}`);
        console.log();
      });
    });

  cmd
    .command('rollback <id>')
    .description('Rollback a flag')
    .requiredOption('--reason <reason>', 'Reason for rollback')
    .action(async (id, opts) => {
      await withFort(async (fort) => {
        const flag = fort.flags.rollback(id, opts.reason);
        console.log(bold('\n  Flag Rolled Back\n'));
        console.log(`  ${cyan(flag.name)} is now ${red('rolled_back')}`);
        console.log(`  Reason: ${opts.reason}`);
        console.log();
      });
    });

  cmd
    .command('check')
    .description('Run bake period checks on all baking flags')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const result = fort.flags.checkBakePeriods();

        if (opts.json) {
          console.log(JSON.stringify(result, null, 2));
          return;
        }

        console.log(bold('\n  Bake Period Check\n'));

        if (result.promoted.length === 0 && result.rolledBack.length === 0) {
          console.log(dim('  No flags changed status.\n'));
          return;
        }

        if (result.promoted.length > 0) {
          console.log(green('  Promoted:'));
          for (const f of result.promoted) {
            console.log(`    ${cyan(f.name)} → ${green('stable')}`);
          }
          console.log();
        }

        if (result.rolledBack.length > 0) {
          console.log(red('  Rolled Back:'));
          for (const f of result.rolledBack) {
            console.log(`    ${cyan(f.name)} → ${red('rolled_back')} (${f.rollbackReason})`);
          }
          console.log();
        }
      });
    });

  // Default action: list all flags
  cmd.action(async () => {
    await withFort(async (fort) => {
      const flags = fort.flags.listFlags();

      if (flags.length === 0) {
        console.log(dim('\n  No feature flags found.\n'));
        return;
      }

      console.log(bold('\n  Feature Flags\n'));

      const rows = [
        [dim('ID'), dim('Name'), dim('Status'), dim('Enabled'), dim('Created')],
      ];

      for (const flag of flags) {
        rows.push([
          flag.id.slice(0, 8),
          cyan(flag.name),
          statusColor(flag.status),
          flag.enabled ? green('yes') : dim('no'),
          flag.createdAt.slice(0, 10),
        ]);
      }

      console.log('  ' + table(rows).split('\n').join('\n  '));
      console.log();
    });
  });

  return cmd;
}
