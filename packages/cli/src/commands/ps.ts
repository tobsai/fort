import { Command } from 'commander';
import { execSync } from 'node:child_process';
import { bold, dim, green, red, yellow, cyan } from '../utils/format.js';

interface ServiceStatus {
  name: string;
  port: number;
  pid: string | null;
  status: 'running' | 'stopped';
  info?: string;
}

export function createPsCommand(): Command {
  return new Command('ps')
    .description('Show running Fort services')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      const services = checkServices();

      if (opts.json) {
        console.log(JSON.stringify(services, null, 2));
        return;
      }

      console.log(bold('\n  Fort Services\n'));

      for (const svc of services) {
        const icon = svc.status === 'running' ? green('●') : dim('○');
        const statusStr = svc.status === 'running' ? green('running') : dim('stopped');
        const pidStr = svc.pid ? dim(` (pid ${svc.pid})`) : '';
        console.log(`  ${icon} ${svc.name.padEnd(22)} :${svc.port}  ${statusStr}${pidStr}`);
        if (svc.info) {
          console.log(`    ${dim(svc.info)}`);
        }
      }

      const running = services.filter((s) => s.status === 'running');
      console.log();

      if (running.length === 0) {
        console.log(dim('  No Fort services running.'));
        console.log(dim('  Services start automatically when you use Fort commands.\n'));
      } else {
        console.log(dim(`  ${running.length} service${running.length > 1 ? 's' : ''} running.`));
        console.log(dim(`  Use ${cyan('fort stop')} to stop all.\n`));
      }
    });
}

function checkServices(): ServiceStatus[] {
  return [
    checkPort('IPC Server', 4001),
    checkPort('WebSocket Server', 4077),
    checkHealthEndpoint('HTTP API', 4077),
  ].filter((s): s is ServiceStatus => s !== null);
}

function checkPort(name: string, port: number): ServiceStatus {
  try {
    const output = execSync(
      `lsof -ti :${port} 2>/dev/null`,
      { encoding: 'utf-8' },
    ).trim();

    if (output) {
      const pids = output.split('\n').filter(Boolean);
      return {
        name,
        port,
        pid: pids[0],
        status: 'running',
      };
    }
  } catch {
    // No process on port
  }

  return { name, port, pid: null, status: 'stopped' };
}

function checkHealthEndpoint(name: string, port: number): ServiceStatus | null {
  // Only check health if port is open
  try {
    const portCheck = execSync(`lsof -ti :${port} 2>/dev/null`, { encoding: 'utf-8' }).trim();
    if (!portCheck) return null;

    const response = execSync(
      `curl -s -o /dev/null -w '%{http_code}' http://localhost:${port}/health 2>/dev/null`,
      { encoding: 'utf-8', timeout: 2000 },
    ).trim();

    if (response === '200') {
      // Get the health response body
      try {
        const body = execSync(
          `curl -s http://localhost:${port}/health 2>/dev/null`,
          { encoding: 'utf-8', timeout: 2000 },
        ).trim();
        const data = JSON.parse(body);
        return {
          name: 'Health Endpoint',
          port,
          pid: null,
          status: 'running',
          info: `${data.agents ?? '?'} agents registered`,
        };
      } catch {
        return {
          name: 'Health Endpoint',
          port,
          pid: null,
          status: 'running',
        };
      }
    }
  } catch {
    // Health check failed or timed out
  }
  return null;
}
