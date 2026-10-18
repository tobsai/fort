import { serveHistoryMaintenance } from './core/host/history-maintenance.js';

const controller = new AbortController();
const stop = () => controller.abort();
process.once('SIGTERM', stop);
try {
  if (process.version !== 'v26.5.0' || process.argv.length !== 2) throw new Error('Unexpected maintenance launch');
  await serveHistoryMaintenance({ directory: process.cwd(), input: process.stdin, output: process.stdout, signal: controller.signal });
} catch {
  process.stderr.write('History deletion or cleanup could not be confirmed. Retry only the exact saved request.\n');
  process.exitCode = 1;
} finally { process.stdin.destroy(); process.removeListener('SIGTERM', stop); }
