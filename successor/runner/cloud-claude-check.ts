import { mkdtempSync, readdirSync, realpathSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { controlProblems } from './claude-code-chat.ts';
import { cloudClaudeSignedIn, createCloudClaude, installedCloudClaude } from './cloud-claude.ts';

// Image check for Claude on the hub (Spec 063, revision 2026-11-03), run by CI inside the hub image: the shipped Claude Code
// starts under setpriv with Fort's exact switches, confirms its controls, reports no account, offers the manual sign-in page
// (the one whose code the owner pastes), and leaves its workspace empty. It completes no sign-in and sends no turn.
export async function checkCloudClaude(root?: string): Promise<string> {
  const installed = installedCloudClaude(root);
  if (!installed) throw new Error('The image has no Claude Code');
  const scratch = realpathSync(mkdtempSync(join(tmpdir(), 'fcl-'))), directory = join(scratch, 'c');
  try {
    const claude = createCloudClaude({ executable: installed.executable, version: installed.version, directory, hostId: 'check', report: line => process.stderr.write(line + '\n') });
    const problems = controlProblems(await claude.runtime.probe(AbortSignal.timeout(30_000)) as Record<string, unknown>, installed.version);
    if (problems.length) throw new Error('Claude Code controls were not confirmed: ' + problems.join(', '));
    if (claude.signedIn()) throw new Error('A fresh hub reported a Claude sign-in');
    let challenge: { kind: string; authorizationURL?: string } | undefined;
    const result = await claude.runtime.signIn({ durationMs: 20_000, onChallenge: value => { challenge = value as typeof challenge; },
      code: () => Promise.reject(new Error('The check pastes no code')) }, AbortSignal.timeout(25_000));
    if (challenge?.kind !== 'browserCode' || new URL(challenge.authorizationURL!).searchParams.get('redirect_uri') === null) throw new Error('No manual Claude sign-in page was offered');
    if (result.kind === 'signedIn' || cloudClaudeSignedIn(directory)) throw new Error('The check signed in');
    if (readdirSync(join(directory, 'workspace')).length) throw new Error('Claude Code wrote into its workspace');
    await claude.runtime.dispose();
    return `Claude Code ${installed.version} controls confirmed; signed out; manual sign-in page offered`;
  } finally { rmSync(scratch, { recursive: true, force: true }); }
}

if (import.meta.main) {
  try { process.stdout.write(await checkCloudClaude(process.argv[2]) + '\n'); }
  catch (error) { process.stderr.write('Cloud Claude check failed: ' + (error instanceof Error ? error.message : 'unknown error') + '\n'); process.exitCode = 1; }
}
