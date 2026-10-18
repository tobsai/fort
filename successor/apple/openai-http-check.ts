import { LOCAL_CONTEXT } from '../modules/local-context.ts';
import { CODEX_RUNTIME } from '../runner/codex-descriptor.ts';
import assert from 'node:assert/strict';
import { mkdtemp, writeFile, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { execFile } from 'node:child_process';
import { createServer } from 'node:http';
import { promisify } from 'node:util';
import { createApplication } from '../service/openai.ts';
import type { ChatInput, ChatRuntime } from '../runner/chat-runtime.ts';

const executable = process.argv[2];
if (!executable) throw new Error('Pass the built FortNativeCheck executable path');
const directory = await mkdtemp(join(tmpdir(), 'fort-candidate-native-test-'));
const inputs: ChatInput[] = [];
let cleanupConfirmations = 0;
const runtime: ChatRuntime = { async dispose() { cleanupConfirmations++; }, async execute(input) {
  inputs.push(structuredClone(input));
  if(input.text==='Synthetic uncertain turn')return {kind:'outcomeUnknown',message:'Synthetic missing terminal evidence'};
  return { kind:'completed', text:input.configuration.skills.length ? 'Packing list [CHECKED]' : input.conversationId === inputs[0]?.conversationId ? 'cedar' : 'Synthetic sample reply', effectiveModel:null };
} };
let service = createApplication({ context: LOCAL_CONTEXT, provider: CODEX_RUNTIME, dataDirectory:join(directory,'ledger'), runtime, autoRun:true });
let discardedAcceptances = 0, discardedPendingAcceptances = 0, discardedCancellations = 0;
const gateway = createServer((request,response)=>{void(async()=>{
  let body='';for await(const chunk of request)body+=String(chunk);
  const headers:Record<string,string>={};
  for(const name of ['content-type','authorization','x-fort-installation-id'])if(typeof request.headers[name]==='string')headers[name]=request.headers[name];
  const upstream=await fetch(service.baseURL+request.url,{method:request.method,headers,body:body||undefined});
  const result=await upstream.text();
  if(request.method==='POST'&&request.url?.endsWith('/messages')&&body.includes('Synthetic acceptance response lost')&&discardedAcceptances===0){
    assert.equal(upstream.status,201);discardedAcceptances++;
    response.writeHead(503,{'Content-Type':'application/json'});
    response.end(JSON.stringify({error:{code:'syntheticLostAcceptance',message:'Synthetic gateway discarded the committed acceptance response.'}}));
  }else if(request.method==='POST'&&request.url?.endsWith('/messages')&&body.includes('Synthetic pending cancellation acceptance lost')&&discardedPendingAcceptances===0){
    assert.equal(upstream.status,201);discardedPendingAcceptances++;
    response.writeHead(503,{'Content-Type':'application/json'});response.end(JSON.stringify({error:{code:'syntheticLostAcceptance',message:'Synthetic discarded acceptance.'}}));
  }else if(request.method==='POST'&&request.url?.startsWith('/v1/requests/')&&request.url.endsWith('/cancel')&&discardedCancellations===0){
    assert.equal(upstream.status,200);discardedCancellations++;
    response.writeHead(503,{'Content-Type':'application/json'});response.end(JSON.stringify({error:{code:'syntheticLostCancellation',message:'Synthetic discarded cancellation.'}}));
  }else{response.writeHead(upstream.status,{'Content-Type':'application/json'});response.end(result);}
})().catch(()=>{response.writeHead(500);response.end();});});
try {
  await service.start();
  await new Promise<void>(resolve=>gateway.listen(0,'127.0.0.1',resolve));
  const address=gateway.address();assert(address&&typeof address==='object');
  const config = { baseURL:`http://127.0.0.1:${address.port}`, installationId:service.installation.installationId, codeA:service.issueEnrollment().code, codeB:service.issueEnrollment().code, stateDirectory:pathToFileURL(join(directory,'native')).href };
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
  service = createApplication({ context: LOCAL_CONTEXT, provider: CODEX_RUNTIME, dataDirectory:join(directory,'ledger'),runtime,autoRun:true });
  await service.start();
  await writeFile(configPath,JSON.stringify(config),{ mode:0o600 });
  await run('workspace-replay');
  assert.equal(inputs.length,5);
  assert.equal(inputs[4]?.configuration.agentName,'Writer');
  assert.equal(inputs[4]?.history.length,4);
  assert.match(inputs[4]!.history[0]!.text,/cedar/);
  assert.equal(inputs[4]?.history.every(m => m.conversationId === inputs[0]?.conversationId),true);
  await run('workspace-recovery-prepare');
  assert.equal(inputs.length,6);
  await service.close();
  service=createApplication({ context: LOCAL_CONTEXT,provider: CODEX_RUNTIME, dataDirectory:join(directory,'ledger'),runtime,autoRun:true});await service.start();
  await run('workspace-recovery-close');
  assert.equal(discardedAcceptances,1);
  assert.equal(inputs.length,7);
  await service.close();
  service=createApplication({ context: LOCAL_CONTEXT,provider: CODEX_RUNTIME, dataDirectory:join(directory,'ledger'),runtime,autoRun:true});await service.start();
  await run('workspace-recovery-replay');
  assert.equal(inputs.length,8);
  assert.equal(inputs.filter(input=>input.text==='Synthetic uncertain turn').length,1);
  assert.equal(inputs.filter(input=>input.text==='Synthetic acceptance response lost').length,1);
  assert.equal(inputs.some(input=>input.text==='Synthetic queued turn to cancel after restart'),false);
  assert.equal(inputs[7]?.history.length,10);
  assert.equal(inputs[7]?.history.every(message=>message.conversationId===inputs[0]?.conversationId),true);
  assert.equal(cleanupConfirmations,3);
  await run('workspace-bundles');
  assert.equal(inputs.length,8);
  await run('workspace-routine-prepare');assert.equal(inputs.length,9);
  const firstTask=JSON.parse(inputs[8]!.text);assert.equal(firstTask.stepId,'draft');assert.equal(firstTask.inputs.topic,'  Synthetic Café e\u0301\r\n');
  await service.close();service=createApplication({ context: LOCAL_CONTEXT,provider: CODEX_RUNTIME, dataDirectory:join(directory,'ledger'),runtime,autoRun:true});await service.start();
  await run('workspace-routine-replay');assert.equal(inputs.length,10);
  const finalTask=JSON.parse(inputs[9]!.text);assert.equal(finalTask.stepId,'finish');assert.equal(finalTask.previousOutputs.length,1);assert.equal(finalTask.previousOutputs[0]?.text,'cedar');
  assert.equal(cleanupConfirmations,4);
  process.stdout.write('PASS routine HTTP conformance: two synthetic task executions separated by a durable gate and restart, schedule consent and pause without execution\n');
  await service.close();service=createApplication({ context: LOCAL_CONTEXT,provider: CODEX_RUNTIME, dataDirectory:join(directory,'ledger'),runtime,autoRun:false});await service.start();
  await run('workspace-pending-prepare');assert.equal(inputs.length,10);assert.equal(discardedPendingAcceptances,1);assert.equal(discardedCancellations,1);
  await service.close();service=createApplication({ context: LOCAL_CONTEXT,provider: CODEX_RUNTIME, dataDirectory:join(directory,'ledger'),runtime,autoRun:false});await service.start();
  await run('workspace-pending-replay');assert.equal(await service.dispatchNext(),false);assert.equal(inputs.length,10);assert.equal(cleanupConfirmations,6);
  process.stdout.write('PASS FortClient/PendingStore HTTP conformance: ten synthetic executions, six service restarts, two discarded acceptances, one discarded cancellation, no duplicate or canceled dispatch and empty pending queue\n');
} finally { await new Promise<void>((resolve,reject)=>gateway.close(error=>error?reject(error):resolve())); await service.close(); await rm(directory,{ recursive:true,force:true }); }
