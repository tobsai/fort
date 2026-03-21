import { Command } from 'commander';
import { existsSync } from 'node:fs';
import { withFort } from '../utils/fort-instance.js';
import { bold, statusIcon, statusColor, dim, cyan, green, yellow, timeAgo } from '../utils/format.js';

export function createAgentsCommand(): Command {
  const cmd = new Command('agents')
    .description('Agent management');

  cmd
    .command('list')
    .description('List all registered agents')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const agents = fort.agents.listInfo();

        if (opts.json) {
          console.log(JSON.stringify(agents, null, 2));
          return;
        }

        console.log(bold('\n  Agent Registry\n'));

        if (agents.length === 0) {
          console.log(dim('  No agents registered.\n'));
          return;
        }

        for (const agent of agents) {
          console.log(`  ${statusIcon(agent.status)} ${bold(agent.config.name)} ${dim(`(${agent.config.type})`)}`);
          console.log(`    Status: ${statusColor(agent.status)} | Tasks: ${agent.taskCount} | Errors: ${agent.errorCount}`);
          console.log(`    ${dim(agent.config.description)}`);
          if (agent.config.capabilities.length > 0) {
            console.log(`    ${dim(`Capabilities: ${agent.config.capabilities.join(', ')}`)}`);
          }
          console.log();
        }
      });
    });

  cmd
    .command('inspect <agentId>')
    .description('Show detailed agent information')
    .option('--json', 'Output as JSON')
    .action(async (agentId, opts) => {
      await withFort(async (fort) => {
        const agent = fort.agents.get(agentId);
        if (!agent) {
          console.error(`  Agent not found: ${agentId}`);
          return;
        }

        const info = agent.info;
        if (opts.json) {
          console.log(JSON.stringify(info, null, 2));
          return;
        }

        console.log(bold(`\n  Agent: ${info.config.name}\n`));
        console.log(`  ID:           ${info.config.id}`);
        console.log(`  Type:         ${info.config.type}`);
        console.log(`  Status:       ${statusColor(info.status)}`);
        console.log(`  Description:  ${info.config.description}`);
        console.log(`  Started:      ${info.startedAt.toISOString()} ${dim(`(${timeAgo(info.startedAt)})`)}`);
        console.log(`  Tasks:        ${info.taskCount}`);
        console.log(`  Errors:       ${info.errorCount}`);
        if (info.currentTaskId) {
          console.log(`  Current Task: ${info.currentTaskId}`);
        }
        console.log(`  Capabilities: ${info.config.capabilities.join(', ')}`);
        console.log();
      });
    });

  cmd
    .command('hatch')
    .description('Hatch a new specialist agent')
    .requiredOption('--name <name>', 'Agent name')
    .requiredOption('--description <desc>', 'What this agent does')
    .option('--capabilities <caps>', 'Comma-separated capabilities', '')
    .option('--behaviors <rules>', 'Comma-separated behavioral rules', '')
    .option('--events <events>', 'Comma-separated event subscriptions', '')
    .option('--from <file>', 'Hatch from a YAML identity file')
    .action(async (opts) => {
      await withFort(async (fort) => {
        try {
          let agent;

          if (opts.from) {
            if (!existsSync(opts.from)) {
              console.error(`  File not found: ${opts.from}`);
              return;
            }
            agent = fort.hatchery.hatchFromFile(opts.from);
          } else {
            agent = fort.hatchery.hatch({
              name: opts.name,
              description: opts.description,
              capabilities: opts.capabilities ? opts.capabilities.split(',').map((s: string) => s.trim()) : [],
              behaviors: opts.behaviors ? opts.behaviors.split(',').map((s: string) => s.trim()) : [],
              eventSubscriptions: opts.events ? opts.events.split(',').map((s: string) => s.trim()) : [],
            });
          }

          await agent.start();

          console.log(bold('\n  Agent Hatched!\n'));
          console.log(`  ${green('✓')} ${bold(agent.config.name)} ${dim(`(${agent.config.id})`)}`);
          console.log(`    ${agent.config.description}`);
          if (agent.identity.capabilities.length > 0) {
            console.log(`    ${dim(`Capabilities: ${agent.identity.capabilities.join(', ')}`)}`);
          }
          if (agent.identity.behaviors.length > 0) {
            console.log(`    ${dim('Behaviors:')}`);
            for (const b of agent.identity.behaviors) {
              console.log(`      ${dim(`- ${b}`)}`);
            }
          }
          console.log();
        } catch (err) {
          console.error(`  Hatch failed: ${err instanceof Error ? err.message : err}`);
        }
      });
    });

  cmd
    .command('retire <agentId>')
    .description('Retire a specialist agent (preserves memory)')
    .option('--reason <reason>', 'Reason for retirement')
    .action(async (agentId, opts) => {
      await withFort(async (fort) => {
        try {
          fort.hatchery.retire(agentId, opts.reason);
          console.log(`\n  ${yellow('⚠')} Agent ${bold(agentId)} retired.`);
          console.log(dim('  Memory and history preserved.\n'));
        } catch (err) {
          console.error(`  Retire failed: ${err instanceof Error ? err.message : err}`);
        }
      });
    });

  cmd
    .command('fork <sourceAgentId>')
    .description('Fork an agent into a new specialist')
    .requiredOption('--name <name>', 'Name for the forked agent')
    .option('--description <desc>', 'Override description')
    .action(async (sourceAgentId, opts) => {
      await withFort(async (fort) => {
        try {
          const agent = fort.hatchery.fork(sourceAgentId, {
            name: opts.name,
            description: opts.description,
          });
          await agent.start();
          console.log(`\n  ${green('✓')} Forked ${bold(sourceAgentId)} → ${bold(agent.config.name)} ${dim(`(${agent.config.id})`)}\n`);
        } catch (err) {
          console.error(`  Fork failed: ${err instanceof Error ? err.message : err}`);
        }
      });
    });

  cmd
    .command('revive <agentId>')
    .description('Revive a retired specialist agent')
    .action(async (agentId) => {
      await withFort(async (fort) => {
        try {
          const agent = fort.hatchery.revive(agentId);
          await agent.start();
          console.log(`\n  ${green('✓')} Agent ${bold(agent.config.name)} revived and running.\n`);
        } catch (err) {
          console.error(`  Revive failed: ${err instanceof Error ? err.message : err}`);
        }
      });
    });

  cmd
    .command('identities')
    .description('List all specialist identities (active and retired)')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const identities = fort.hatchery.listIdentities();

        if (opts.json) {
          console.log(JSON.stringify(identities, null, 2));
          return;
        }

        console.log(bold('\n  Specialist Identities\n'));

        if (identities.length === 0) {
          console.log(dim('  No specialists hatched yet. Use `fort agents hatch` to create one.\n'));
          return;
        }

        for (const id of identities) {
          const statusStr = id.status === 'active' ? green('active') : dim('retired');
          console.log(`  ${id.status === 'active' ? green('●') : dim('○')} ${bold(id.name)} ${dim(`(${id.id})`)} — ${statusStr}`);
          console.log(`    ${id.description}`);
          if (id.behaviors.length > 0) {
            console.log(`    ${dim(`Behaviors: ${id.behaviors.join(' | ')}`)}`);
          }
          if (id.parentId) {
            console.log(`    ${dim(`Forked from: ${id.parentId}`)}`);
          }
          console.log();
        }
      });
    });

  return cmd;
}
