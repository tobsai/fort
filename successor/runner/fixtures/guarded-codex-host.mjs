import { writeFileSync, renameSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { launchGuardedCodexProcess } from '../guarded-codex-process.ts';

const [directory, guardian] = process.argv.slice(2);
setTimeout(() => process.exit(2), 20_000);
const owned = launchGuardedCodexProcess({ guardian, executable: process.execPath,
  args: [fileURLToPath(new URL('./guarded-codex-runtime.mjs', import.meta.url)), 'parent-death'], cwd: directory, env: {}, deadlineMs: 15_000 });
try {
  const result = await owned.connection.request('ping', {}, new AbortController().signal);
  const temporary = join(directory, 'ready.json.tmp');
  writeFileSync(temporary, JSON.stringify({ ...result, host: process.pid }), { mode: 0o600 });
  renameSync(temporary, join(directory, 'ready.json'));
  await new Promise(() => {});
} finally { await owned.dispose(); }
