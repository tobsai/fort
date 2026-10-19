import { createInterface } from 'node:readline';
import { writeFileSync, appendFileSync } from 'node:fs';
import { join } from 'node:path';
import { spawn } from 'node:child_process';
import { syntheticConfig, syntheticFeatures, syntheticAccount } from '../codex-preflight-fixture.ts';

const [scenario, directory] = process.argv.slice(2);
const watchdog = setTimeout(() => process.exit(2), 20_000); watchdog.unref();
let descendant;
if (scenario === 'parent-death') {
  process.on('SIGTERM', () => {});
  descendant = spawn(process.execPath, ['-e', 'process.on("SIGTERM",()=>{});setTimeout(()=>process.exit(2),20000);process.stdout.write("ready\\n")'], { env: {}, stdio: ['ignore', 'pipe', 'ignore'] });
  await new Promise((resolve, reject) => { descendant.once('error', reject); descendant.stdout.once('data', resolve); });
}
writeFileSync(join(directory, 'runtime.json'), JSON.stringify({ runtime: process.pid, innerGuardian: process.ppid, descendant: descendant?.pid }));
const send = frame => process.stdout.write(JSON.stringify(frame) + '\n');
const reply = (id, result) => send({ id, result });
const completed = status => send({ method: 'turn/completed', params: { threadId: 'fixture-thread', turn: { id: 'fixture-turn', status, error: null } } });
let accountAuthenticated = !scenario.startsWith('account-');
const accountLoginId = '2a190387-aee4-4d5f-ab17-63210a74f348';
const accountCompleted = success => send({ method: 'account/login/completed', params: { loginId: accountLoginId, success, error: null } });
for await (const line of createInterface({ input: process.stdin })) {
  const frame = JSON.parse(line), { id, method, params } = frame;
  appendFileSync(join(directory, 'methods.jsonl'), JSON.stringify({ method, model: params?.model, refreshToken: params?.refreshToken,
    loginType: method === 'account/login/start' ? params?.type : undefined }) + '\n');
  if (method === 'initialized') continue;
  if (method === 'initialize') reply(id, {});
  else if (method === 'config/read') {
    const config = syntheticConfig(); if (scenario === 'unsafe-config') config.analytics.enabled = true;
    reply(id, { config });
  } else if (method === 'permissionProfile/list') reply(id, { data: [{ id: 'fort_candidate_b_no_files', allowed: true }], nextCursor: null });
  else if (method === 'experimentalFeature/list') reply(id, { data: syntheticFeatures(), nextCursor: null });
  else if (method === 'mcpServerStatus/list') reply(id, { data: [], nextCursor: null });
  else if (method === 'account/read') reply(id, { ...syntheticAccount(), account: accountAuthenticated ? { ...syntheticAccount().account, type: scenario === 'foreign-account' ? 'apiKey' : 'chatgpt' } : null });
  else if (method === 'account/login/start' && scenario.startsWith('account-')) {
    if (params.type !== 'chatgptDeviceCode') process.exit(3);
    reply(id, { type: 'chatgptDeviceCode', loginId: accountLoginId, verificationUrl: 'https://auth.openai.com/codex/device', userCode: 'FORT-TEST' });
    if (scenario === 'account-success') setTimeout(() => { accountAuthenticated = true; accountCompleted(true); }, 20);
  }
  else if (method === 'account/login/cancel' && scenario.startsWith('account-')) {
    if (params.loginId !== accountLoginId) process.exit(3);
    if (scenario === 'account-race') { accountAuthenticated = true; accountCompleted(true); reply(id, { status: 'notFound' }); }
    else { accountCompleted(false); reply(id, { status: 'canceled' }); }
  }
  else if (method === 'model/list') {
    writeFileSync(join(directory, 'model-list-started'), 'synthetic');
    if (scenario !== 'catalog-cancel') reply(id, { data: scenario === 'catalog' ? [
      { model: 'fixture-exact-model', displayName: 'Text fixture', inputModalities: ['text'] },
      { model: 'fixture-audio-model', displayName: 'Audio fixture', inputModalities: ['audio'] },
    ] : [{ model: scenario === 'missing-model' ? 'foreign-model' : 'fixture-exact-model' }], nextCursor: null });
  }
  else if (method === 'thread/start') reply(id, { thread: { id: 'fixture-thread', ephemeral: true }, model: params.model, modelProvider: 'openai', cwd: params.cwd,
    approvalPolicy: 'untrusted', approvalsReviewer: 'user', activePermissionProfile: { id: 'fort_candidate_b_no_files', extends: null },
    runtimeWorkspaceRoots: [], sandbox: { type: 'readOnly', networkAccess: false }, instructionSources: [] });
  else if (method === 'turn/start') {
    reply(id, { turn: { id: 'fixture-turn', status: 'inProgress' } });
    writeFileSync(join(directory, 'turn-started'), 'synthetic');
    if (scenario === 'echo') {
      const text = JSON.parse(params.input[0].text).currentMessage.text;
      send({ method: 'item/completed', params: { threadId: 'fixture-thread', turnId: 'fixture-turn', item: { id: 'reply', type: 'agentMessage', phase: 'final_answer', text } } });
      completed('completed');
    }
  } else if (method === 'turn/interrupt') {
    reply(id, {});
    if (scenario === 'delayed-cancel') setTimeout(() => completed('interrupted'), 450);
  } else process.exit(3);
}
