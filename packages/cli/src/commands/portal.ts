import { Command } from 'commander';
import { exec } from 'node:child_process';
import { getFort } from '../utils/fort-instance.js';
import { bold, cyan, dim } from '../utils/format.js';

export function createPortalCommand(): Command {
  return new Command('portal')
    .description('Start the Fort web portal')
    .option('-p, --port <port>', 'Port number', '4077')
    .option('--no-open', 'Do not open browser automatically')
    .action(async (opts) => {
      const port = parseInt(opts.port, 10);
      const fort = getFort();

      await fort.start();

      const { FortServer } = await import('@fort/core');
      const server = new FortServer(fort, port);
      await server.start();

      const url = `http://localhost:${port}`;
      console.log(bold('\n  Fort Portal\n'));
      console.log(`    ${cyan(url)}`);
      console.log(`    ${dim('Press Ctrl+C to stop')}\n`);

      if (opts.open !== false) {
        exec(`open ${url}`);
      }

      // Keep process alive and handle shutdown
      const shutdown = async () => {
        console.log(dim('\n  Shutting down...'));
        await server.stop();
        await fort.stop();
        process.exit(0);
      };

      process.on('SIGINT', shutdown);
      process.on('SIGTERM', shutdown);
    });
}
