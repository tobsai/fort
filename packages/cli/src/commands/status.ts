import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, statusIcon, statusColor, dim, timeAgo } from '../utils/format.js';

export function createStatusCommand(): Command {
  const cmd = new Command('status')
    .description('Show Fort system status')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const agents = fort.agents.listInfo();
        const activeTasks = fort.taskGraph.getActiveTasks();
        const blockedTasks = fort.taskGraph.getBlockedTasks();
        const totalTasks = fort.taskGraph.getTaskCount();
        const memStats = fort.memory.stats();
        const toolStats = fort.tools.stats();
        const routines = fort.scheduler.listRoutines();

        if (opts.json) {
          console.log(JSON.stringify({
            agents: agents.map((a: any) => ({ ...a.config, status: a.status })),
            tasks: { total: totalTasks, active: activeTasks.length, blocked: blockedTasks.length },
            memory: memStats,
            tools: toolStats,
            routines: routines.length,
          }, null, 2));
          return;
        }

        console.log(bold('\n  Fort Status\n'));

        // Agents
        console.log(bold('  Agents:'));
        for (const agent of agents) {
          console.log(`    ${statusIcon(agent.status)} ${agent.config.name} ${dim(`(${agent.config.type})`)} — ${statusColor(agent.status)}`);
        }
        console.log();

        // Tasks
        console.log(bold('  Tasks:'));
        console.log(`    Total: ${totalTasks}`);
        console.log(`    Active: ${activeTasks.length}`);
        console.log(`    Blocked: ${blockedTasks.length}`);
        console.log();

        // Memory
        console.log(bold('  Memory:'));
        console.log(`    Nodes: ${memStats.nodeCount}`);
        console.log(`    Edges: ${memStats.edgeCount}`);
        console.log();

        // Tools
        console.log(bold('  Tools:'));
        console.log(`    Registered: ${toolStats.totalTools}`);
        console.log(`    Total usage: ${toolStats.totalUsage}`);
        console.log();

        // Scheduler
        console.log(bold('  Scheduler:'));
        console.log(`    Routines: ${routines.length} (${routines.filter((r: any) => r.enabled).length} enabled)`);
        console.log();
      });
    });

  return cmd;
}
