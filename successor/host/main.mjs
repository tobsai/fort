import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { serveOwnedCore } from './core/host/owned-core.js';
import { createPrivateCodexExecutorProcess, CODEX_RUNTIME } from './executor/runner/codex-executor.js';
import { LOCAL_CONTEXT } from './context/modules/local-context.js';

const controller = new AbortController();
const stop = () => controller.abort();
process.once('SIGTERM', stop);
try {
  if (process.argv.length !== 2) throw new Error('The bundled host accepts no arguments');
  const runtime = createPrivateCodexExecutorProcess({
    executable: fileURLToPath(new URL('../../MacOS/codex', import.meta.url)),
    guardian: fileURLToPath(new URL('../../MacOS/FortProcessGuardian', import.meta.url)),
    directory: join(process.cwd(), 'Runtime'), hostId: CODEX_RUNTIME.hostId,
  });
  await serveOwnedCore({ dataDirectory: process.cwd(), provider: CODEX_RUNTIME, runtime, context: LOCAL_CONTEXT, network: 'explicit',
    input: process.stdin, output: process.stdout, signal: controller.signal });
} catch {
  process.stderr.write('Fort host could not start or finish safely.\n');
  process.exitCode = 1;
} finally { process.removeListener('SIGTERM', stop); }
