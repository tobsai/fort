import { renameSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { serveOwnedCore } from './core/host/owned-core.js';
import { createPrivateCodexExecutorProcess, CODEX_RUNTIME } from './executor/runner/codex-executor.js';
import { createClaudeCodeHost, selectedClaudeCode } from './executor/runner/claude-code-host.js';
import { LOCAL_CONTEXT } from './context/modules/local-context.js';

// The helper discards stderr, so a startup failure leaves its reason here for the app to show. Only errors raised before
// serving begins are recorded: those come from Fort's own checks, never from channel input or message bodies.
const FAILURE = 'host-start-failure.txt';
function recordFailure(error) {
  const reason = (error instanceof Error ? error.message : 'Unknown error').replace(/[^\x20-\x7e]/g, ' ').slice(0, 300);
  try { writeFileSync(FAILURE + '.partial', reason + '\n', { mode: 0o600 }); renameSync(FAILURE + '.partial', FAILURE); } catch {}
}
let serving = false;
const controller = new AbortController();
const stop = () => controller.abort();
process.once('SIGTERM', stop);
try {
  rmSync(FAILURE, { force: true });
  if (process.argv.length !== 2) throw new Error('The bundled host accepts no arguments');
  const guardian = fileURLToPath(new URL('../../MacOS/FortProcessGuardian', import.meta.url));
  // Claude-only mode while the multi-provider service is built (Spec 062): chosen by the owner in the app. It shares the
  // host's one ledger and pairing. The ledger records one runtime identity, so an existing host can't switch provider.
  const claude = selectedClaudeCode(process.cwd());
  const host = claude
    ? createClaudeCodeHost({ directory: claude, guardian, temporary: join(process.env.TMPDIR ?? '', 'claude') })
    : { provider: CODEX_RUNTIME, runtime: createPrivateCodexExecutorProcess({
      executable: fileURLToPath(new URL('../../MacOS/codex', import.meta.url)), guardian,
      directory: join(process.cwd(), 'Runtime'), hostId: CODEX_RUNTIME.hostId,
    }) };
  await serveOwnedCore({ dataDirectory: process.cwd(), provider: host.provider, runtime: host.runtime, context: LOCAL_CONTEXT, network: 'explicit',
    input: process.stdin, output: process.stdout, signal: controller.signal, ready: () => { serving = true; } });
} catch (error) {
  process.stderr.write('Fort host could not start or finish safely.\n');
  if (!serving) recordFailure(error);
  process.exitCode = 1;
} finally { process.removeListener('SIGTERM', stop); }
