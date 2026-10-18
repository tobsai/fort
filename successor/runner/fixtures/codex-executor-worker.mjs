import { writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { serveCodexExecutor } from '../codex-executor.ts';

const [directory, guardian, scenario] = process.argv.slice(2);
writeFileSync(join(directory, 'worker.json'), JSON.stringify({ worker: process.pid, outerGuardian: process.ppid }));
try {
  await serveCodexExecutor({ input: process.stdin, output: process.stdout, hostId: 'fixture-host',
    runtime: { guardian, executable: process.execPath, args: [fileURLToPath(new URL('./codex-executor-runtime.mjs', import.meta.url)), scenario, directory],
      cwd: join(directory, 'workspace'), env: {} },
  });
} catch { process.exitCode = 1; }
