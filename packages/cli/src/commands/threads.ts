import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, cyan, yellow, green, statusColor, table, timeAgo } from '../utils/format.js';

export function createThreadsCommand(): Command {
  const cmd = new Command('threads')
    .description('Conversation thread management');

  // ── List threads (default action) ──────────────────────────────────

  cmd
    .option('--all', 'Include paused and resolved threads')
    .option('--project <tag>', 'Filter by project tag')
    .option('--agent <id>', 'Filter by assigned agent')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const filterOpts: Record<string, unknown> = {};
        if (!opts.all) filterOpts.status = 'active';
        if (opts.project) filterOpts.projectTag = opts.project;
        if (opts.agent) filterOpts.agentId = opts.agent;

        const list = fort.threads.listThreads(filterOpts as any);

        if (opts.json) {
          console.log(JSON.stringify(list, null, 2));
          return;
        }

        if (list.length === 0) {
          console.log(dim('\n  No threads found.\n'));
          return;
        }

        console.log(bold('\n  Threads\n'));

        const rows = [
          [dim('ID'), dim('Name'), dim('Status'), dim('Project'), dim('Last Active')],
        ];

        for (const t of list) {
          rows.push([
            cyan(t.id.slice(0, 8)),
            t.name,
            statusColor(t.status),
            t.projectTag ?? dim('—'),
            timeAgo(t.lastActiveAt),
          ]);
        }

        console.log('  ' + table(rows).split('\n').join('\n  '));
        console.log();
      });
    });

  // ── Create thread ──────────────────────────────────────────────────

  cmd
    .command('create')
    .description('Create a new thread')
    .requiredOption('--name <name>', 'Thread name')
    .option('--description <desc>', 'Thread description')
    .option('--project <tag>', 'Project tag')
    .option('--agent <id>', 'Assigned agent')
    .option('--parent <threadId>', 'Parent thread ID')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const thread = fort.threads.createThread({
          name: opts.name,
          description: opts.description,
          projectTag: opts.project,
          assignedAgent: opts.agent,
          parentThreadId: opts.parent,
        });

        console.log(bold('\n  Thread Created\n'));
        console.log(`  ID:      ${cyan(thread.id)}`);
        console.log(`  Name:    ${thread.name}`);
        console.log(`  Task:    ${dim(thread.taskId)}`);
        console.log(`  Status:  ${statusColor(thread.status)}`);
        if (thread.projectTag) console.log(`  Project: ${thread.projectTag}`);
        if (thread.assignedAgent) console.log(`  Agent:   ${thread.assignedAgent}`);
        console.log();
      });
    });

  // ── Show thread messages ───────────────────────────────────────────

  cmd
    .command('show <id>')
    .description('Show thread messages')
    .option('--limit <n>', 'Limit messages', parseInt)
    .option('--json', 'Output as JSON')
    .action(async (id, opts) => {
      await withFort(async (fort) => {
        const thread = fort.threads.getThread(id);
        const messages = fort.threads.getMessages(id, { limit: opts.limit });

        if (opts.json) {
          console.log(JSON.stringify({ thread, messages }, null, 2));
          return;
        }

        console.log(bold(`\n  Thread: ${thread.name}\n`));
        console.log(`  Status: ${statusColor(thread.status)}  |  Task: ${dim(thread.taskId.slice(0, 8))}`);
        if (thread.projectTag) console.log(`  Project: ${thread.projectTag}`);
        console.log();

        if (messages.length === 0) {
          console.log(dim('  No messages.\n'));
          return;
        }

        for (const msg of messages) {
          const roleLabel = msg.role === 'user' ? green('user')
            : msg.role === 'agent' ? cyan(msg.agentId ?? 'agent')
            : yellow('system');
          console.log(`  ${dim(timeAgo(msg.createdAt))}  ${roleLabel}`);
          console.log(`  ${msg.content}\n`);
        }
      });
    });

  // ── Pause thread ───────────────────────────────────────────────────

  cmd
    .command('pause <id>')
    .description('Pause a thread')
    .action(async (id) => {
      await withFort(async (fort) => {
        const thread = fort.threads.pauseThread(id);
        console.log(bold('\n  Thread Paused\n'));
        console.log(`  ${cyan(thread.id.slice(0, 8))}  ${thread.name}  ${statusColor(thread.status)}\n`);
      });
    });

  // ── Resume thread ──────────────────────────────────────────────────

  cmd
    .command('resume <id>')
    .description('Resume a paused thread')
    .action(async (id) => {
      await withFort(async (fort) => {
        const thread = fort.threads.resumeThread(id);
        console.log(bold('\n  Thread Resumed\n'));
        console.log(`  ${cyan(thread.id.slice(0, 8))}  ${thread.name}  ${statusColor(thread.status)}\n`);
      });
    });

  // ── Resolve thread ─────────────────────────────────────────────────

  cmd
    .command('resolve <id>')
    .description('Resolve/complete a thread')
    .action(async (id) => {
      await withFort(async (fort) => {
        const thread = fort.threads.resolveThread(id);
        console.log(bold('\n  Thread Resolved\n'));
        console.log(`  ${cyan(thread.id.slice(0, 8))}  ${thread.name}  ${statusColor(thread.status)}\n`);
      });
    });

  // ── Fork thread ────────────────────────────────────────────────────

  cmd
    .command('fork <id>')
    .description('Fork a thread into a new branch')
    .requiredOption('--name <name>', 'Name for the forked thread')
    .option('--from-message <messageId>', 'Fork from a specific message')
    .action(async (id, opts) => {
      await withFort(async (fort) => {
        const forked = fort.threads.forkThread(id, {
          name: opts.name,
          fromMessageId: opts.fromMessage,
        });

        console.log(bold('\n  Thread Forked\n'));
        console.log(`  New ID:  ${cyan(forked.id)}`);
        console.log(`  Name:    ${forked.name}`);
        console.log(`  Parent:  ${dim(id.slice(0, 8))}`);
        console.log(`  Task:    ${dim(forked.taskId)}`);
        console.log();
      });
    });

  // ── Search threads ─────────────────────────────────────────────────

  cmd
    .command('search <query>')
    .description('Search across thread names and messages')
    .option('--json', 'Output as JSON')
    .action(async (query, opts) => {
      await withFort(async (fort) => {
        const results = fort.threads.searchThreads(query);

        if (opts.json) {
          console.log(JSON.stringify(results, null, 2));
          return;
        }

        if (results.length === 0) {
          console.log(dim(`\n  No threads matching "${query}".\n`));
          return;
        }

        console.log(bold(`\n  Search: "${query}"  (${results.length} results)\n`));

        const rows = [
          [dim('ID'), dim('Name'), dim('Status'), dim('Last Active')],
        ];

        for (const t of results) {
          rows.push([
            cyan(t.id.slice(0, 8)),
            t.name,
            statusColor(t.status),
            timeAgo(t.lastActiveAt),
          ]);
        }

        console.log('  ' + table(rows).split('\n').join('\n  '));
        console.log();
      });
    });

  return cmd;
}
