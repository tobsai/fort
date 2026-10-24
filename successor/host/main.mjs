import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { serveOwnedCore } from './core/host/owned-core.js';
import { createPrivateCodexExecutorProcess, CODEX_RUNTIME } from './executor/runner/codex-executor.js';
import { createClaudeCodeHost, selectedClaudeCode } from './executor/runner/claude-code-host.js';
import { LOCAL_CONTEXT } from './context/modules/local-context.js';

const controller = new AbortController();
const stop = () => controller.abort();
process.once('SIGTERM', stop);
try {
  if (process.argv.length !== 2) throw new Error('The bundled host accepts no arguments');
  const guardian = fileURLToPath(new URL('../../MacOS/FortProcessGuardian', import.meta.url));
  // Claude-only mode while the multi-provider service is built: chosen by the owner in the app, with its own ledger (Spec 062).
  const claude = selectedClaudeCode(process.cwd());
  const host = claude
    ? createClaudeCodeHost({ directory: claude, guardian, temporary: join(process.env.TMPDIR ?? '', 'claude') })
    : { dataDirectory: process.cwd(), provider: CODEX_RUNTIME, runtime: createPrivateCodexExecutorProcess({
      executable: fileURLToPath(new URL('../../MacOS/codex', import.meta.url)), guardian,
      directory: join(process.cwd(), 'Runtime'), hostId: CODEX_RUNTIME.hostId,
    }) };
  await serveOwnedCore({ dataDirectory: host.dataDirectory, provider: host.provider, runtime: host.runtime, context: LOCAL_CONTEXT, network: 'explicit',
    input: process.stdin, output: process.stdout, signal: controller.signal });
} catch {
  process.stderr.write('Fort host could not start or finish safely.\n');
  process.exitCode = 1;
} finally { process.removeListener('SIGTERM', stop); }
