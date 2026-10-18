import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createCodexExecutorProcess } from '../codex-executor.ts';

const [directory, guardian] = process.argv.slice(2);
setTimeout(() => process.exit(2), 20_000);
const runtime = createCodexExecutorProcess({ guardian, hostId: 'fixture-host', executable: process.execPath,
  args: [fileURLToPath(new URL('./codex-executor-worker.mjs', import.meta.url)), directory, guardian, 'parent-death'], cwd: directory, env: {} });
try {
  const { input, dispatch } = JSON.parse(readFileSync(join(directory, 'command.json'), 'utf8'));
  await runtime.execute(input, new AbortController().signal, dispatch);
} finally { await runtime.dispose(); }
