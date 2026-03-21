import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, cyan, green, red, yellow } from '../utils/format.js';

export function createHarnessCommand(): Command {
  const cmd = new Command('harness')
    .description('Self-coding harness — Fort writes its own code');

  cmd
    .command('start')
    .description('Start a new self-coding cycle')
    .requiredOption('--goal <goal>', 'Goal for the self-coding cycle')
    .option('--approach <approach>', 'Approach to take')
    .option('--files <files>', 'Comma-separated affected files')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        try {
          const affectedFiles = opts.files
            ? opts.files.split(',').map((f: string) => f.trim())
            : undefined;

          const cycle = fort.harness.startCycle(
            opts.goal,
            opts.approach,
            affectedFiles,
          );

          if (opts.json) {
            console.log(JSON.stringify(cycle, null, 2));
            return;
          }

          console.log(bold('\n  Self-Coding Cycle Started\n'));
          console.log(`  Cycle ID: ${cyan(cycle.id)}`);
          console.log(`  Spec ID:  ${cyan(cycle.specId)}`);
          console.log(`  Branch:   ${cyan(cycle.branch)}`);
          console.log(`  Status:   ${yellow(cycle.status)}`);
          console.log();
        } catch (err) {
          console.error(red(`  Error: ${err instanceof Error ? err.message : String(err)}`));
          process.exitCode = 1;
        }
      });
    });

  cmd
    .command('approve <cycleId>')
    .description('Approve spec to proceed to implementation')
    .action(async (cycleId) => {
      await withFort(async (fort) => {
        try {
          const cycle = fort.harness.approveSpec(cycleId);
          console.log(green(`  Spec approved for cycle ${cycle.id}`));
          console.log(`  Status: ${yellow(cycle.status)}`);
        } catch (err) {
          console.error(red(`  Error: ${err instanceof Error ? err.message : String(err)}`));
          process.exitCode = 1;
        }
      });
    });

  cmd
    .command('status [cycleId]')
    .description('Show cycle status or list all cycles')
    .option('--json', 'Output as JSON')
    .action(async (cycleId, opts) => {
      await withFort(async (fort) => {
        if (cycleId) {
          const cycle = fort.harness.getCycle(cycleId);
          if (!cycle) {
            console.error(red(`  Cycle not found: ${cycleId}`));
            return;
          }

          if (opts.json) {
            console.log(JSON.stringify(cycle, null, 2));
            return;
          }

          console.log(bold(`\n  Cycle: ${cycle.id}\n`));
          console.log(`  Spec ID:  ${cycle.specId}`);
          console.log(`  Branch:   ${cycle.branch}`);
          console.log(`  Status:   ${statusColor(cycle.status)}`);
          console.log(`  Started:  ${cycle.startedAt}`);
          if (cycle.completedAt) {
            console.log(`  Completed: ${cycle.completedAt}`);
          }
          if (cycle.error) {
            console.log(`  Error:    ${red(cycle.error)}`);
          }
          console.log();
          console.log(bold('  Steps:'));
          for (const step of cycle.steps) {
            const icon = stepIcon(step.status);
            console.log(`    ${icon} ${step.phase} ${dim(`(${step.status})`)}`);
            if (step.error) {
              console.log(`      ${red(step.error)}`);
            }
          }
          console.log();
        } else {
          const cycles = fort.harness.listCycles();

          if (opts.json) {
            console.log(JSON.stringify(cycles, null, 2));
            return;
          }

          console.log(bold('\n  Self-Coding Cycles\n'));

          if (cycles.length === 0) {
            console.log(dim('  No cycles found.\n'));
            return;
          }

          for (const cycle of cycles) {
            console.log(
              `  ${cyan(cycle.id.slice(0, 8))} ${statusColor(cycle.status)} ${dim(cycle.branch)}`,
            );
            if (cycle.error) {
              console.log(`    ${red(cycle.error)}`);
            }
          }
          console.log();
        }
      });
    });

  cmd
    .command('rollback <cycleId>')
    .description('Rollback a self-coding cycle')
    .requiredOption('--reason <reason>', 'Reason for rollback')
    .action(async (cycleId, opts) => {
      await withFort(async (fort) => {
        try {
          const cycle = fort.harness.rollback(cycleId, opts.reason);
          console.log(yellow(`  Cycle ${cycle.id} rolled back`));
          console.log(`  Reason: ${opts.reason}`);
        } catch (err) {
          console.error(red(`  Error: ${err instanceof Error ? err.message : String(err)}`));
          process.exitCode = 1;
        }
      });
    });

  cmd
    .command('gc')
    .description('Garbage collection report')
    .option('--clean', 'Actually clean up (with confirmation)')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const report = fort.gc.generateReport();

        if (opts.json) {
          console.log(JSON.stringify(report, null, 2));
          return;
        }

        console.log(bold('\n  Garbage Collection Report\n'));

        const totalIssues =
          report.staleSpecs.length +
          report.unusedTools.length +
          report.deadFlags.length +
          report.orphanedTasks.length;

        if (totalIssues === 0) {
          console.log(green('  Everything is clean. No issues found.\n'));
          return;
        }

        if (report.staleSpecs.length > 0) {
          console.log(bold('  Stale Specs:'));
          for (const spec of report.staleSpecs) {
            console.log(
              `    ${yellow(spec.id.slice(0, 8))} ${spec.title} ${dim(`(${spec.status}, ${spec.ageDays} days old)`)}`,
            );
          }
          console.log();
        }

        if (report.unusedTools.length > 0) {
          console.log(bold('  Unused Tools:'));
          for (const tool of report.unusedTools) {
            const age = tool.daysSinceUse !== null ? `${tool.daysSinceUse} days` : 'never used';
            console.log(`    ${yellow(tool.name)} ${dim(`(${age})`)}`);
          }
          console.log();
        }

        if (report.deadFlags.length > 0) {
          console.log(bold('  Dead Feature Flags:'));
          for (const flag of report.deadFlags) {
            console.log(
              `    ${yellow(flag.cycleId.slice(0, 8))} ${flag.branch} ${dim(`(${flag.status})`)}`,
            );
          }
          console.log();
        }

        if (report.orphanedTasks.length > 0) {
          console.log(bold('  Orphaned Tasks:'));
          for (const task of report.orphanedTasks) {
            console.log(
              `    ${yellow(task.id.slice(0, 8))} ${task.title} ${dim(`(${task.ageDays} days)`)}`,
            );
          }
          console.log();
        }

        console.log(dim(`  Total issues: ${totalIssues}\n`));

        if (opts.clean) {
          console.log(yellow('  --clean flag acknowledged. Cleanup not yet implemented.\n'));
        }
      });
    });

  return cmd;
}

function statusColor(status: string): string {
  switch (status) {
    case 'complete':
      return green(status);
    case 'failed':
    case 'rolled_back':
      return red(status);
    default:
      return yellow(status);
  }
}

function stepIcon(status: string): string {
  switch (status) {
    case 'passed':
      return green('*');
    case 'failed':
      return red('x');
    case 'running':
      return yellow('>');
    case 'skipped':
      return dim('-');
    default:
      return dim('o');
  }
}
