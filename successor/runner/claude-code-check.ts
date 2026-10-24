// Manual check of the Claude Code runtime on the owner's own machine, outside the app sandbox.
//   node runner/claude-code-check.ts --executable "$(command -v claude)"            # sign-in state and models only
//   node runner/claude-code-check.ts --executable "$(command -v claude)" --prompt   # plus one fixed prompt on the subscription
// Prints no credential, no email and no reply text beyond the fixed check word.
import { execFileSync } from 'node:child_process';
import { mkdtempSync, rmSync, realpathSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { parseArgs } from 'node:util';
import { ClaudeCodeChat, parseClaudeAuthStatus, readAuthStatus } from './claude-code-chat.ts';
import { contextInput } from './fixtures/context-input.ts';
import type { ExecutionDispatch } from '../contracts/executor.ts';

const { values } = parseArgs({ options: { executable: { type: 'string' }, prompt: { type: 'boolean', default: false }, model: { type: 'string' } }, strict: true });
if (!values.executable) throw new Error('Pass --executable with the absolute path of the official claude binary');
const executable = values.executable;
const version = /^(\d+\.\d+\.\d+) \(Claude Code\)$/.exec(execFileSync(executable, ['--version'], { env: { PATH: '/usr/bin:/bin', HOME: homedir() }, encoding: 'utf8' }).trim())?.[1];
if (!version) throw new Error('Not an official Claude Code binary');
const root = realpathSync(mkdtempSync('/tmp/fcc-'));
const report: Record<string, unknown> = { claudeCode: version, executable: realpathSync(executable) };
try {
  const options = { executable, workspace: join(root, 'w'), home: homedir(), runtimeDirectory: root };
  execFileSync('/bin/mkdir', ['-m', '700', options.workspace]);
  const status = await readAuthStatus(options, AbortSignal.timeout(15_000));
  const row = status as Record<string, unknown>;
  report.authMethod = row.authMethod; report.apiProvider = row.apiProvider; report.subscriptionType = row.subscriptionType; report.apiKeyPresent = row.apiKeySource !== undefined;
  let account;
  try { account = parseClaudeAuthStatus(status); } catch { report.result = 'refused: not a Claude subscription sign-in'; }
  if (account === null) report.result = 'signed out: run `claude auth login`';
  if (account) {
    report.accountId = account.accountId.slice(0, 8) + '…';
    const chat = new ClaudeCodeChat({ ...options, runtimeVersion: version });
    const catalog = await chat.models(AbortSignal.timeout(30_000), account);
    report.models = catalog.state === 'ready' ? catalog.models.map(model => model.id) : catalog.state + ': ' + catalog.detail;
    if (values.prompt && catalog.state === 'ready') {
      const model = values.model ?? catalog.models.find(model => /sonnet/.test(model.id))?.id ?? catalog.models[0]!.id;
      const attempt = { runId: 'check', attemptId: 'check-1', generation: 1, bindingRevisionId: 'check', runtimeVersion: version, selectedModel: model };
      const outcome = await chat.execute(contextInput({ attempt, runId: 'check', requestId: 'check', conversationId: 'check', text: 'Reply with exactly one lowercase word: cedar',
        configuration: { account, agentId: 'check', agentName: 'Check', bindingRevisionId: 'check', selectedModel: model, instructions: 'Follow the instruction exactly.', skills: [] }, history: [] }),
        AbortSignal.timeout(120_000), { deadline: Date.now() + 110_000 } as ExecutionDispatch);
      report.prompt = { model, kind: outcome.kind, ...(outcome.kind === 'completed' ? { expectedWord: outcome.text.trim().toLowerCase().replace(/[^a-z]/g, '') === 'cedar' } : { message: outcome.message }) };
    }
    await chat.dispose();
  }
} finally { rmSync(root, { recursive: true, force: true }); }
process.stdout.write(JSON.stringify(report, null, 2) + '\n');
