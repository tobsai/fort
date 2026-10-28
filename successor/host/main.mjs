import { renameSync, rmSync, writeFileSync } from 'node:fs';
import { hostname } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { serveOwnedCore } from './core/host/owned-core.js';
import { createPrivateCodexExecutorProcess, CODEX_RUNTIME } from './executor/runner/codex-executor.js';
import { createClaudeCodeHost, selectedClaudeCode, watchClaudeCodeSelection } from './executor/runner/claude-code-host.js';
import { macHubRunner, sharedRuntime } from './executor/runner/mac-hub-runner.js';
import { LOCAL_CONTEXT } from './context/modules/local-context.js';

// The helper discards stderr, so a startup problem leaves its reason here for the app to show. Only errors raised before
// serving begins are recorded: those come from Fort's own checks, never from channel input or message bodies.
const FAILURE = 'host-start-failure.txt';
function recordFailure(error, prefix = '') {
  const reason = (prefix + (error instanceof Error ? error.message : 'Unknown error')).replace(/[^\x20-\x7e]/g, ' ').slice(0, 300);
  try { writeFileSync(FAILURE + '.partial', reason + '\n', { mode: 0o600 }); renameSync(FAILURE + '.partial', FAILURE); } catch {}
}
// Diagnostic lines on stderr: the Mac helper puts them in the unified log (subsystem io.mtree.fort). Only Fort's own
// lifecycle messages and reasons, never message bodies or credentials.
const note = line => { try { process.stderr.write('Fort host: ' + line.replace(/[^\x20-\x7e]/g, ' ').slice(0, 280) + '\n'); } catch {} };
let serving = false;
const controller = new AbortController();
const stop = () => controller.abort();
process.once('SIGTERM', stop);
try {
  rmSync(FAILURE, { force: true });
  if (process.argv.length !== 2) throw new Error('The bundled host accepts no arguments');
  const guardian = fileURLToPath(new URL('../../MacOS/FortProcessGuardian', import.meta.url));
  // ChatGPT (Codex) is always the primary runtime. When the owner has turned Claude on in the app, Claude Code runs beside it
  // (Spec 064), and each Agent's binding decides which one serves its turns. If Claude can't start, ChatGPT still does.
  const codex = createPrivateCodexExecutorProcess({
    executable: fileURLToPath(new URL('../../MacOS/codex', import.meta.url)), guardian,
    directory: join(process.cwd(), 'Runtime'), hostId: CODEX_RUNTIME.hostId,
  });
  const runtimes = [];
  const claude = selectedClaudeCode(process.cwd());
  if (claude) {
    try {
      const lane = createClaudeCodeHost({ directory: claude, guardian, temporary: join(process.env.TMPDIR ?? '', 'c'), report: note });
      runtimes.push(lane); note('Claude Code ' + lane.provider.runtimeVersion + ' is on beside ChatGPT');
    } catch (error) {
      recordFailure(error, 'Claude Code could not start, so only ChatGPT is available: ');
      note('Claude Code could not start: ' + (error instanceof Error ? error.message : 'unknown error'));
    }
  }
  // Turning Claude on or off in the app ends this host cleanly; the helper exits with it, and macOS starts a new one with
  // the new choice on the app's next request.
  watchClaudeCodeSelection(process.cwd(), () => { note('Claude was turned ' + (claude ? 'off' : 'on') + ', restarting'); stop(); });
  // Spec 063: once the owner enrolls this Mac with a cloud hub, the same runtimes also take the hub's turns, under the
  // node the hub named (this Mac's runner ID, or the pool's node). ChatGPT gets its own executor for that ID; Claude's
  // runtime is shared with the local core.
  const hub = macHubRunner({ stateDirectory: process.cwd(), runnerName: hostname().replace(/\.local$/, '').slice(0, 128) || 'Mac',
    lanes: hostId => [
      { binding: { provider: CODEX_RUNTIME.provider, runtimeVersion: CODEX_RUNTIME.runtimeVersion, hostId },
        createRuntime: sharedRuntime(createPrivateCodexExecutorProcess({ executable: fileURLToPath(new URL('../../MacOS/codex', import.meta.url)), guardian,
          directory: join(process.cwd(), 'Runtime'), hostId })) },
      ...runtimes.map(lane => ({ binding: { provider: lane.provider.provider, runtimeVersion: lane.provider.runtimeVersion, hostId }, createRuntime: sharedRuntime(lane.runtime) })),
    ] });
  try {
    await serveOwnedCore({ dataDirectory: process.cwd(), provider: CODEX_RUNTIME, runtime: codex, runtimes, context: LOCAL_CONTEXT, network: 'explicit', hubRunner: hub,
      input: process.stdin, output: process.stdout, signal: controller.signal, ready: () => {
        serving = true; note('serving ' + (runtimes.length ? 'ChatGPT and Claude' : 'ChatGPT only'));
        void hub.start().then(status => { if (status.state !== 'notEnrolled') note('connecting to the cloud hub as ' + status.runnerId); }, error => note('cloud hub runner could not start: ' + (error instanceof Error ? error.message : 'unknown error')));
      } });
  } finally { await hub.stop(); }
} catch (error) {
  process.stderr.write('Fort host could not start or finish safely.\n');
  note((serving ? 'stopped after an error: ' : 'could not start: ') + (error instanceof Error ? error.message : 'unknown error'));
  if (!serving) recordFailure(error);
  process.exitCode = 1;
} finally { process.removeListener('SIGTERM', stop); }
