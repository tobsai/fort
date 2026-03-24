import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, green, red, yellow } from '../utils/format.js';

export function createBehaviorsCommand(): Command {
  const cmd = new Command('behaviors')
    .description('Behavior management');

  cmd
    .command('list')
    .description('List all active behaviors')
    .option('--context <context>', 'Filter by context')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const list = fort.behaviors.listBehaviors(opts.context);

        if (opts.json) {
          console.log(JSON.stringify(list, null, 2));
          return;
        }

        console.log(bold('\n  Behaviors\n'));

        if (list.length === 0) {
          console.log(dim('  No active behaviors.\n'));
          return;
        }

        for (const b of list) {
          const priorityColor = b.priority >= 7 ? red : b.priority >= 4 ? yellow : dim;
          const priorityStr = priorityColor(`P${b.priority}`);

          console.log(`  ${priorityStr} ${bold(b.rule)}`);
          console.log(`    ${dim(`Context: ${b.context} | Source: ${b.source} | ID: ${b.id.slice(0, 8)}...`)}`);
          if (b.examples && b.examples.length > 0) {
            console.log(`    ${dim(`Examples: ${b.examples.join('; ')}`)}`);
          }
          console.log();
        }
      });
    });

  cmd
    .command('add')
    .description('Add a new behavior')
    .requiredOption('--rule <rule>', 'The behavioral rule text')
    .requiredOption('--context <context>', 'When this applies (e.g., email, scheduling, all)')
    .option('--priority <n>', 'Priority 1-10 (default: 5)', '5')
    .option('--source <source>', 'Source of behavior', 'user')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const behavior = fort.behaviors.addBehavior(
          opts.rule,
          opts.context,
          parseInt(opts.priority, 10),
          opts.source,
        );

        console.log(green(`\n  Behavior added: ${behavior.id.slice(0, 8)}...`));
        console.log(`  Rule: ${bold(behavior.rule)}`);
        console.log(`  Context: ${behavior.context} | Priority: ${behavior.priority}\n`);
      });
    });

  cmd
    .command('remove')
    .description('Remove (disable) a behavior')
    .argument('<id>', 'Behavior ID (or prefix)')
    .action(async (id) => {
      await withFort(async (fort) => {
        // Support prefix matching
        const allBehaviors = fort.behaviors.listBehaviors();
        const match = allBehaviors.find((b: any) => b.id === id || b.id.startsWith(id));

        if (!match) {
          console.log(red(`\n  Behavior not found: ${id}\n`));
          return;
        }

        const removed = fort.behaviors.removeBehavior(match.id);
        if (removed) {
          console.log(green(`\n  Behavior disabled: ${match.id.slice(0, 8)}...\n`));
        } else {
          console.log(red(`\n  Failed to remove behavior: ${id}\n`));
        }
      });
    });

  return cmd;
}
