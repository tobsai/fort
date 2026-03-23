import { Command } from 'commander';
import { execSync } from 'node:child_process';
import { bold, dim, green, yellow } from '../utils/format.js';

export function createStopCommand(): Command {
  return new Command('stop')
    .description('Stop any running Fort processes')
    .action(async () => {
      let stopped = 0;

      // Kill any process listening on Fort's IPC port (4001)
      stopped += killPort(4001, 'IPC server');

      // Kill any process listening on Fort's HTTP/WS port (4077)
      stopped += killPort(4077, 'WebSocket server');

      if (stopped === 0) {
        console.log(dim('\n  No running Fort processes found.\n'));
      } else {
        console.log(`\n  ${green('✓')} Stopped ${stopped} Fort process${stopped > 1 ? 'es' : ''}.\n`);
      }
    });
}

function killPort(port: number, label: string): number {
  try {
    const pids = execSync(
      `lsof -ti :${port} 2>/dev/null`,
      { encoding: 'utf-8' },
    ).trim();

    if (!pids) return 0;

    for (const pid of pids.split('\n').filter(Boolean)) {
      try {
        process.kill(parseInt(pid, 10), 'SIGTERM');
        console.log(`  ${yellow('●')} Stopped ${label} ${dim(`(pid ${pid}, port ${port})`)}`);
      } catch {
        // Process may have already exited
      }
    }
    return 1;
  } catch {
    return 0;
  }
}
