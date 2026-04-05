import { Command } from 'commander';
import { FortDoctor } from '@fort-ai/core';
import { withFort } from '../utils/fort-instance.js';
import { bold, statusIcon, statusColor, table, green, red, yellow } from '../utils/format.js';

export function createDoctorCommand(): Command {
  const cmd = new Command('doctor')
    .description('Run comprehensive health checks')
    .option('--json', 'Output as JSON')
    .option('--module <name>', 'Check a specific module only')
    .action(async (opts) => {
      await withFort(async (fort) => {
        console.log(bold('\n  Fort Doctor\n'));

        const results = opts.module
          ? [await fort.doctor.runModule(opts.module)].filter(Boolean)
          : await fort.runDoctor();

        if (opts.json) {
          console.log(JSON.stringify(results, null, 2));
          return;
        }

        for (const result of results) {
          if (!result) continue;
          console.log(`  ${statusIcon(result.status)} ${bold(result.module)} — ${statusColor(result.status)}`);
          for (const check of result.checks) {
            console.log(`    ${statusIcon(check.passed ? 'healthy' : 'unhealthy')} ${check.name}: ${check.message}`);
          }
          console.log();
        }

        const summary = FortDoctor.summarize(results.filter(Boolean) as any[]);
        console.log(`  ${bold('Overall:')} ${statusColor(summary.overall)} (${summary.passed}/${summary.totalChecks} checks passed)\n`);
      });
    });

  return cmd;
}
