import { mkdtempSync, readdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { privateCodexLaunchPlan } from './private-codex-plan.ts';
import { launchLinuxCodexProcess, SETPRIV } from './linux-codex-process.ts';
import { prepareCloudCodexDirectory } from './cloud-codex-executor.ts';
import { verifyRuntimeControls } from './codex-controls.ts';
import { readCodexAccount } from './codex-account.ts';

// Image check for the cloud runner (Spec 063, revision 2026-11-03), run by CI inside the hub image: the shipped Codex
// starts under setpriv with Fort's exact controls, confirms them, reports no account (nothing is signed in), and leaves
// its home and workspace empty. It makes no sign-in and sends no turn.
export async function checkCloudCodex(executable: string): Promise<string> {
  const root = mkdtempSync(join(tmpdir(), 'fort-cloud-check-')), directory = prepareCloudCodexDirectory(join(root, 'codex'));
  const plan = privateCodexLaunchPlan({ executable, guardian: SETPRIV, directory });
  const owned = launchLinuxCodexProcess({ ...plan, deadlineMs: 30_000 });
  const signal = AbortSignal.timeout(25_000);
  try {
    const started = await owned.connection.request('initialize', { clientInfo: { name: 'fort_candidate_b', title: 'Fort', version: '0.1.0' }, capabilities: { experimentalApi: true, requestAttestation: false } }, signal) as { platformOs?: unknown };
    owned.connection.notify('initialized', {});
    await verifyRuntimeControls(owned.connection, plan.cwd, signal);
    if (await readCodexAccount(owned.connection, signal) !== null) throw new Error('A fresh cloud runner reported an account');
    for (const part of ['home', 'workspace']) if (readdirSync(join(directory, part)).length) throw new Error('Codex wrote into its ' + part + ' directory');
    return `Codex controls confirmed on ${String(started.platformOs)}; signed out`;
  } finally { await owned.dispose(); rmSync(root, { recursive: true, force: true }); }
}

if (import.meta.main) {
  try { process.stdout.write(await checkCloudCodex(process.argv[2] ?? '/opt/fort/codex/codex') + '\n'); }
  catch (error) { process.stderr.write('Cloud Codex check failed: ' + (error instanceof Error ? error.message : 'unknown error') + '\n'); process.exitCode = 1; }
}
