import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, statusIcon, statusColor, dim, timeAgo, table } from '../utils/format.js';

export function createTasksCommand(): Command {
  const cmd = new Command('tasks')
    .description('Show and manage tasks')
    .option('--all', 'Show all tasks including completed')
    .option('--agent <name>', 'Filter by agent')
    .option('--status <status>', 'Filter by status')
    .option('--search <query>', 'Full-text search across tasks')
    .option('--json', 'Output as JSON')
    .argument('[taskId]', 'Inspect a specific task')
    .action(async (taskId, opts) => {
      await withFort(async (fort) => {
        if (taskId) {
          // Inspect specific task
          try {
            const task = fort.taskGraph.getTask(taskId);
            const subtasks = fort.taskGraph.getSubtasks(taskId);

            if (opts.json) {
              console.log(JSON.stringify({ task, subtasks }, null, 2));
              return;
            }

            console.log(bold(`\n  Task #${task.id.slice(0, 8)}\n`));
            console.log(`  Title:    ${task.title}`);
            console.log(`  Status:   ${statusColor(task.status)}`);
            console.log(`  Source:   ${task.source}`);
            console.log(`  Agent:    ${task.assignedAgent ?? dim('unassigned')}`);
            console.log(`  Created:  ${task.createdAt.toISOString()} ${dim(`(${timeAgo(task.createdAt)})`)}`);
            if (task.description && task.description !== task.title) {
              console.log(`  Description: ${task.description}`);
            }

            if (subtasks.length > 0) {
              console.log(bold('\n  Sub-tasks:'));
              for (const sub of subtasks) {
                console.log(`    ${statusIcon(sub.status)} ${sub.title} ${dim(`(${sub.assignedAgent ?? 'unassigned'})`)}`);
              }
            }
            console.log();
          } catch (err) {
            console.error(`  Task not found: ${taskId}`);
          }
          return;
        }

        // List tasks
        const filter: Record<string, any> = {};
        if (!opts.all) {
          // By default, show non-completed tasks
        }
        if (opts.agent) filter.assignedAgent = opts.agent;
        if (opts.status) filter.status = opts.status;
        if (opts.search) filter.search = opts.search;

        let tasks = fort.taskGraph.queryTasks(filter);

        if (!opts.all) {
          tasks = tasks.filter((t) => t.status !== 'completed' && t.status !== 'failed');
        }

        if (opts.json) {
          console.log(JSON.stringify(tasks, null, 2));
          return;
        }

        console.log(bold('\n  Tasks\n'));

        if (tasks.length === 0) {
          console.log(dim('  No tasks found.\n'));
          return;
        }

        for (const task of tasks.slice(0, 50)) {
          const id = task.id.slice(0, 8);
          const agent = task.assignedAgent ? dim(` [${task.assignedAgent}]`) : '';
          console.log(`  ${statusIcon(task.status)} ${dim(id)} ${task.title}${agent} ${dim(timeAgo(task.createdAt))}`);
        }

        if (tasks.length > 50) {
          console.log(dim(`\n  ... and ${tasks.length - 50} more. Use --all to see everything.`));
        }
        console.log();
      });
    });

  return cmd;
}
