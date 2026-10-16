import assert from 'node:assert/strict';
import { mkdtemp, writeFile, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { createOpenAIService } from '../service/openai.ts';
import type { ChatInput, ChatRuntime } from '../runner/chat-runtime.ts';

const executable = process.argv[2];
if (!executable) throw new Error('Pass the built FortNativeCheck executable path');
const directory = await mkdtemp(join(tmpdir(), 'fort-candidate-native-test-'));
const inputs: ChatInput[] = [];
let cleanupConfirmations = 0;
const runtime: ChatRuntime = { async dispose() { cleanupConfirmations++; }, async execute(input) {
  inputs.push(structuredClone(input));
  return { kind:'completed', text:input.configuration.skills.length ? 'Packing list [CHECKED]' : input.conversationId === inputs[0]?.conversationId ? 'cedar' : 'Synthetic sample reply', effectiveModel:null };
} };
let service = createOpenAIService({ dataDirectory:join(directory,'ledger'), runtime, autoRun:true });
try {
  await service.start();
  const config = { baseURL:service.baseURL, installationId:service.installation.installationId, codeA:service.issueEnrollment().code, codeB:service.issueEnrollment().code, stateDirectory:pathToFileURL(join(directory,'native')).href };
  const configPath = join(directory,'synthetic-config.json');
  await writeFile(configPath,JSON.stringify(config),{ mode:0o600 });
  const run = async (phase: string) => {
    const result = await promisify(execFile)(resolve(executable),[phase,configPath],{ env:{ PATH:'/usr/bin:/bin' },timeout:20_000 });
    process.stdout.write(result.stdout);
  };
  await run('workspace-prepare');
  const saved: unknown = JSON.parse(await readFile(join(directory,'native','native-check.json'),'utf8'));
  assert(saved !== null && typeof saved === 'object' && 'receipts' in saved && Array.isArray(saved.receipts));
  assert.equal(saved.receipts.length,4);
  assert('pendingRequestCount' in saved && saved.pendingRequestCount === 0);
  assert.equal(inputs.length,4);
  assert.equal(inputs[2]?.configuration.skills[0]?.name,'Check marker');
  assert.equal(inputs[3]?.configuration.agentName,'Reviewer');
  assert.deepEqual(inputs[1]?.history,[]);
  assert.deepEqual(inputs[3]?.history,[]);
  await service.close();
  assert.equal(cleanupConfirmations,1);
  service = createOpenAIService({ dataDirectory:join(directory,'ledger'),runtime,autoRun:true });
  await service.start(); config.baseURL = service.baseURL;
  await writeFile(configPath,JSON.stringify(config),{ mode:0o600 });
  await run('workspace-replay');
  assert.equal(inputs.length,5);
  assert.equal(inputs[4]?.configuration.agentName,'Writer');
  assert.equal(inputs[4]?.history.length,4);
  assert.match(inputs[4]!.history[0]!.text,/cedar/);
  assert.equal(inputs[4]?.history.every(m => m.conversationId === inputs[0]?.conversationId),true);
  process.stdout.write('PASS FortClient/PendingStore HTTP conformance: five synthetic turns, four retained completed receipts, empty pending queue, confirmed resource-free runtime cleanup before restart\n');
} finally { await service.close(); await rm(directory,{ recursive:true,force:true }); }
