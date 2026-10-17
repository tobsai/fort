import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import {createServer} from 'node:net';
import { launchOwnedProcess } from '../runner/owned-process.ts';
import { CodexChat } from '../runner/codex-chat.ts';
import type { ChatInput, ChatRuntime } from '../runner/chat-runtime.ts';
import { createOpenAIService } from './openai.ts';
import type { Acceptance, Agent, Conversation, InstructionSkill, Installation, Snapshot, Run } from '../contracts/v1.ts';

async function setup(t: TestContext, options: Omit<Parameters<typeof createOpenAIService>[0],'dataDirectory'> = {}) {
  const directory = await mkdtemp(join(tmpdir(), 'fort-rebuilt-unit-'));
  let service = createOpenAIService({ dataDirectory: directory, ...options });
  await service.start();
  t.after(async () => { await service.close(); await rm(directory, { recursive: true, force: true }); });
  const enrollment = service.issueEnrollment();
  const response = await fetch(service.baseURL + '/v1/devices/enroll', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...enrollment, deviceName: 'Synthetic test device' }) });
  assert.equal(response.status, 201);
  const device = await response.json() as { deviceToken: string };
  const headers = { 'Content-Type': 'application/json', Authorization: `Bearer ${device.deviceToken}`, 'X-Fort-Installation-Id': service.installation.installationId };
  async function api<T>(path: string, body?: unknown) {
    const response = await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers, body: body === undefined ? undefined : JSON.stringify(body) });
    return { status: response.status, body: await response.json() as T };
  }
  return { api, get service() { return service; }, async restart() { await service.close(); service = createOpenAIService({ dataDirectory: directory, ...options }); await service.start(); } };
}

function syntheticBundle() {
  return {schemaVersion:1,name:'Synthetic bundle',version:'1',source:'owner-local:synthetic',license:'CC0-1.0',entrypoint:'SKILL.md',supportedRuntimes:['codex'],requestedTools:[],scripts:[],files:[{path:'SKILL.md',kind:'file',executable:false,contentBase64:Buffer.from('Synthetic reviewed instructions').toString('base64')}]};
}

test('bundle import rejects unsafe paths, links, undeclared executables and malformed or oversized content without staging work',async t=>{
  const h=await setup(t),base=syntheticBundle(),file=base.files[0]!;
  const invalid:unknown[]=[
    ...['../outside','/absolute','nested/../escape','C:\\file','a\u0000b','a//b','a/./b','a/b.'].map(path=>({...base,entrypoint:path,files:[{...file,path}]})),
    {...base,files:[file,{...file,path:'skill.md'}]},
    {...base,files:[file,{...file,path:'assets'},{...file,path:'assets/nested.txt'}]},
    {...base,files:[file,{...file,path:'link',kind:'symlink',target:'outside'}]},
    {...base,files:[file,{...file,path:'run.sh',executable:true}]},
    {...base,scripts:[{path:'missing.sh',interpreter:'bash',resourceScope:[]}]},
    {...base,files:[{...file,contentBase64:'not base64'}]},
    {...base,files:[{...file,contentBase64:Buffer.from([255]).toString('base64')}]},
    {...base,files:[{...file,sha256:'forged'}]},
    {...base,files:[{...file,bytes:999}]},
    {...base,digest:'forged'},
    {...base,files:Array.from({length:65},(_,index)=>({...file,path:index?'asset-'+index:'SKILL.md'}))},
    {...base,files:[file,{...file,path:'large',contentBase64:Buffer.alloc(262_145).toString('base64')}]},
    {...base,files:[file,...[1,2,3].map(index=>({...file,path:'large-'+index,contentBase64:Buffer.alloc(200_000).toString('base64')}))]},
    {...base,unexpectedAuthority:'full-access'},
  ];
  for(const bundle of invalid){const result=await h.api<{error:{code:string}}>('/v1/skill-bundles',{bundle});assert.equal(result.status,400);assert.equal(result.body.error.code,'invalidSkillBundle');}
  assert.deepEqual((await h.api<{skills:InstructionSkill[]}>('/v1/skills')).body.skills,[]);
  assert.equal(await h.service.dispatchNext(),false);
});

test('explicitly disabling a reviewed skill before dispatch blocks accepted work without a runtime call',async t=>{
  let calls=0;
  const h=await setup(t,{runtime:{async execute(){calls++;return {kind:'completed',text:'Must not run',effectiveModel:null};},async dispose(){}}});
  const imported=(await h.api<InstructionSkill>('/v1/skill-bundles',{bundle:syntheticBundle()})).body;assert.ok(imported.bundle);
  const review={revisionId:imported.revisionId,digest:imported.bundle.digest,enable:true};
  await h.api(`/v1/skills/${imported.skillId}/review`,review);
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply.'})).body,prefix=`/v1/conversations/${agent.homeConversationId}`;
  const acceptance=await h.api<Acceptance>(prefix+'/messages',{requestId:'disabled-skill',text:'Synthetic question',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,skills:[{skillId:imported.skillId,revisionId:imported.revisionId}]});
  assert.equal(acceptance.status,201);
  await h.api(`/v1/skills/${imported.skillId}/review`,{...review,enable:false});
  await h.service.dispatchNext();assert.equal(calls,0);
  const snapshot=(await h.api<Snapshot>(prefix+'/snapshot')).body;
  assert.equal(snapshot.runs[0]?.state,'failed');assert.match(snapshot.runs[0]?.statusDetail??'',/disabled/);assert.equal(snapshot.messages.length,1);
});

test('complete skill bundle import is inert, requires exact review and preserves accepted revision after an edit',async t=>{
  const calls:ChatInput[]=[];
  const h=await setup(t,{runtime:{async execute(input){calls.push(structuredClone(input));return {kind:'completed',text:'Synthetic reviewed reply',effectiveModel:null};},async dispose(){}}});
  const bundle={schemaVersion:1,name:'Packing helper',version:'1.0',source:'owner-local:synthetic-packing',license:'CC0-1.0',entrypoint:'SKILL.md',supportedRuntimes:['codex'],requestedTools:[],scripts:[],files:[{path:'SKILL.md',kind:'file',executable:false,contentBase64:Buffer.from('Use the reviewed packing helper.').toString('base64')},{path:'assets/checklist.txt',kind:'file',executable:false,contentBase64:Buffer.from('Notebook\nPencil\n').toString('base64')}]};
  type BundleSkill=InstructionSkill&{bundle:typeof bundle&{digest:string};review:{state:string;digest:string}};
  const imported=await h.api<BundleSkill>('/v1/skill-bundles',{bundle});
  assert.equal(imported.status,201);assert.equal(imported.body.review.state,'reviewRequired');
  assert.equal(imported.body.bundle.files.length,2);assert.equal(imported.body.bundle.files[1]?.contentBase64,bundle.files[1]?.contentBase64);
  assert.match(imported.body.bundle.digest,/^[0-9a-f]{64}$/);assert.equal(calls.length,0);
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply.'})).body;
  const prefix=`/v1/conversations/${agent.homeConversationId}`;
  const command={requestId:'reviewed-bundle',text:'Synthetic question',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,skills:[{skillId:imported.body.skillId,revisionId:imported.body.revisionId}]};
  assert.equal((await h.api(prefix+'/messages',command)).status,409);
  assert.deepEqual((await h.api<Snapshot>(prefix+'/snapshot')).body.messages,[]);
  const review=`/v1/skills/${imported.body.skillId}/review`;
  assert.equal((await h.api(review,{revisionId:imported.body.revisionId,digest:'wrong',enable:true})).status,409);
  const enabled=await h.api<BundleSkill>(review,{revisionId:imported.body.revisionId,digest:imported.body.bundle.digest,enable:true});
  assert.equal(enabled.status,200);assert.equal(enabled.body.review.state,'enabled');
  const accepted=await h.api<Acceptance>(prefix+'/messages',command);assert.equal(accepted.status,201);
  bundle.files[1]!.contentBase64=Buffer.from('Changed asset').toString('base64');bundle.version='2.0';
  const changed=await h.api<BundleSkill>('/v1/skill-bundles',{bundle,skillId:imported.body.skillId,expectedRevisionId:imported.body.revisionId});
  assert.equal(changed.status,201);assert.notEqual(changed.body.revisionId,imported.body.revisionId);assert.equal(changed.body.review.state,'reviewRequired');assert.notEqual(changed.body.bundle.digest,imported.body.bundle.digest);
  assert.equal((await h.api(review,{revisionId:imported.body.revisionId,digest:imported.body.bundle.digest,enable:true})).status,409);
  await h.service.dispatchNext();assert.equal(calls.length,1);
  assert.deepEqual(calls[0]?.configuration.skills,[enabled.body]);
  const replay=await h.api<Acceptance>(prefix+'/messages',command);assert.deepEqual(replay.body.receipt,accepted.body.receipt);
  await h.restart();
  const exported=await h.api<BundleSkill>(`/v1/skills/${imported.body.skillId}/revisions/${imported.body.revisionId}`);
  assert.equal(exported.status,200);assert.deepEqual(exported.body.bundle,imported.body.bundle);
});

test('owner cancels queued work without executing it and the same Home can continue',async t=>{
  const calls:string[]=[];
  const h=await setup(t,{runtime:{async execute(input){calls.push(input.requestId);return {kind:'completed',text:'Synthetic next reply',effectiveModel:null};},async dispose(){}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply briefly.'})).body;
  const path=`/v1/conversations/${agent.homeConversationId}`,epoch=h.service.installation.recoveryEpoch;
  const send=(requestId:string)=>h.api<Acceptance>(path+'/messages',{requestId,text:requestId,recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:epoch,skills:[]});
  const first=(await send('cancel-this')).body;
  await h.restart();assert.equal(await h.service.dispatchNext(),false);
  const canceled=await h.api<{run:Run}>(`/v1/runs/${first.receipt.runId}/cancel`,{recoveryEpoch:epoch});
  assert.equal(canceled.status,200);assert.equal(canceled.body.run.state,'canceled');
  assert.equal(await h.service.dispatchNext(),false);assert.deepEqual(calls,[]);
  const duplicate=await h.api<{run:Run}>(`/v1/runs/${first.receipt.runId}/cancel`,{recoveryEpoch:epoch});
  assert.deepEqual(duplicate.body.run,canceled.body.run);
  await send('continue-here');assert.equal(await h.service.dispatchNext(),true);
  const snapshot=(await h.api<Snapshot>(path+'/snapshot')).body;
  assert.deepEqual(snapshot.runs.map(run=>run.state),['canceled','completed']);
  assert.deepEqual(calls,['continue-here']);
  await h.restart();assert.deepEqual((await h.api<Snapshot>(path+'/snapshot')).body.runs.map(run=>run.state),['canceled','completed']);
});

for(const outcome of ['canceled','completed','outcomeUnknown'] as const)test(`active cancellation preserves runtime evidence and the Conversation barrier: ${outcome}`,async t=>{
  const started=Promise.withResolvers<AbortSignal>(),terminal=Promise.withResolvers<Awaited<ReturnType<ChatRuntime['execute']>>>();
  const calls:string[]=[];
  const h=await setup(t,{runtime:{async execute(input,signal){calls.push(input.requestId);if(input.requestId==='active'){started.resolve(signal);return terminal.promise;}return {kind:'completed',text:'Synthetic later reply',effectiveModel:null};},async dispose(){terminal.resolve({kind:'canceled',message:'Synthetic confirmed interruption'});}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply briefly.'})).body,epoch=h.service.installation.recoveryEpoch;
  const path=`/v1/conversations/${agent.homeConversationId}`;
  const send=(requestId:string)=>h.api<Acceptance>(path+'/messages',{requestId,text:requestId,recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:epoch,skills:[]});
  const first=(await send('active')).body;const dispatch=h.service.dispatchNext();const signal=await started.promise;
  await send('later');
  assert.equal((await h.api(`/v1/runs/${first.receipt.runId}/close-unknown`,{recoveryEpoch:epoch,acknowledgeUncertainty:true})).status,409);
  assert.equal((await h.api(`/v1/runs/${first.receipt.runId}/cancel`,{recoveryEpoch:'wrong'})).status,409);
  const response=await h.api<{run:Run}>(`/v1/runs/${first.receipt.runId}/cancel`,{recoveryEpoch:epoch});
  assert.equal(response.status,200);assert.equal(response.body.run.state,'cancelRequested');assert.equal(signal.aborted,true);
  assert.deepEqual((await h.api<Snapshot>(path+'/snapshot')).body.runs.map(run=>run.state),['cancelRequested','queued']);assert.deepEqual(calls,['active']);
  terminal.resolve(outcome==='completed'?{kind:'completed',text:'Synthetic completion won the race',effectiveModel:null}:{kind:outcome,message:'Synthetic terminal evidence'});await dispatch;
  const final=(await h.api<Snapshot>(path+'/snapshot')).body;
  assert.equal(final.runs[0]?.state,outcome);
  assert.equal(final.messages.filter(message=>message.role==='assistant').length,outcome==='completed'?1:0);
  assert.equal(await h.service.dispatchNext(),outcome!=='outcomeUnknown');
  assert.deepEqual(calls,outcome==='outcomeUnknown'?['active']:['active','later']);
});

test('owner explicitly closes uncertainty without retrying it, preserving history and releasing later work',async t=>{
  const calls:string[]=[];
  const h=await setup(t,{runtime:{async execute(input){calls.push(input.requestId);return input.requestId==='uncertain'?{kind:'outcomeUnknown',message:'Synthetic lost terminal evidence'}:{kind:'completed',text:'Synthetic next reply',effectiveModel:null};},async dispose(){}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply briefly.'})).body,epoch=h.service.installation.recoveryEpoch,path=`/v1/conversations/${agent.homeConversationId}`;
  const send=(requestId:string)=>h.api<Acceptance>(path+'/messages',{requestId,text:requestId,recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:epoch,skills:[]});
  const first=(await send('uncertain')).body;await h.service.dispatchNext();await send('later');
  assert.equal(await h.service.dispatchNext(),false);assert.deepEqual(calls,['uncertain']);
  const close=`/v1/runs/${first.receipt.runId}/close-unknown`;
  assert.equal((await h.api(close,{recoveryEpoch:epoch})).status,400);
  assert.equal((await h.api(close,{recoveryEpoch:'wrong',acknowledgeUncertainty:true})).status,409);
  const result=await h.api<{run:Run}>(close,{recoveryEpoch:epoch,acknowledgeUncertainty:true});
  assert.equal(result.status,200);assert.equal(result.body.run.state,'closedUnknown');
  assert.match(result.body.run.statusDetail??'',/not retried/);
  assert.deepEqual((await h.api<{run:Run}>(close,{recoveryEpoch:epoch,acknowledgeUncertainty:true})).body,result.body);
  await h.service.dispatchNext();assert.deepEqual(calls,['uncertain','later']);
  await h.restart();const snapshot=(await h.api<Snapshot>(path+'/snapshot')).body;
  assert.deepEqual(snapshot.runs.map(run=>run.state),['closedUnknown','completed']);assert.equal(snapshot.messages.length,3);
  assert.deepEqual((await h.api<Acceptance>('/v1/requests/uncertain')).body.receipt,first.receipt);
});

test('runtime refusal before dispatch is a visible failure, not an owner cancellation',async t=>{
  const h=await setup(t,{runtime:{async execute(){return {kind:'notStarted',message:'Exact selected model is unavailable. Nothing was dispatched.'};},async dispose(){}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply briefly.'})).body;
  const path=`/v1/conversations/${agent.homeConversationId}`;
  await h.api(path+'/messages',{requestId:'unavailable',text:'Synthetic question',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,skills:[]});
  await h.service.dispatchNext();
  const snapshot=(await h.api<Snapshot>(path+'/snapshot')).body;
  assert.equal(snapshot.runs[0]?.state,'failed');assert.match(snapshot.runs[0]?.statusDetail??'',/Nothing was dispatched/);assert.equal(snapshot.messages.length,1);
});

for(const when of ['before','during'] as const)test(`canceled listener startup exposes no address or surviving listener: ${when}`,async t=>{
  const directory=await mkdtemp(join(tmpdir(),'fort-listener-cancel-'));
  const service=createOpenAIService({dataDirectory:directory});
  t.after(async()=>{await service.close();await rm(directory,{recursive:true,force:true});});
  const reservation=createServer();t.after(()=>{reservation.close();});
  await new Promise<void>(resolve=>reservation.listen(0,'127.0.0.1',resolve));
  const address=reservation.address();assert.ok(address&&typeof address==='object');
  const port=address.port;
  await new Promise<void>((resolve,reject)=>reservation.close(error=>error?reject(error):resolve()));
  const cancellation=new AbortController();if(when==='before')cancellation.abort();
  const starting=service.start({port,signal:cancellation.signal});
  const result=starting.then(()=> 'unexpected success',()=> 'rejected');
  cancellation.abort();
  assert.equal(await result,'rejected');
  assert.equal(service.baseURL,'');
  await new Promise(resolve=>setImmediate(resolve));
  await assert.rejects(()=>fetch(`http://127.0.0.1:${port}/v1/installation`));
});

test('owner creates two exact Agent Homes, an instruction skill and a shared conversation that survive restart', async t => {
  const h = await setup(t);
  const writer = await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Reply briefly.' });
  assert.equal(writer.status, 201);
  const reviewer = await h.api<Agent>('/v1/agents', { name: 'Reviewer', model: 'gpt-6-astra', instructions: 'Check the claim.' });
  assert.equal(reviewer.status, 201);
  assert.notEqual(writer.body.homeConversationId, reviewer.body.homeConversationId);
  const skill = await h.api<InstructionSkill>('/v1/skills', { name: 'Check marker', instructions: 'End with [CHECKED].' });
  assert.equal(skill.status, 201);
  const room = await h.api<Conversation>('/v1/conversations', { name: 'Sample planning', participantAgentIds: [writer.body.agentId, reviewer.body.agentId] });
  assert.equal(room.status, 201);
  const identity = h.service.installation;
  await h.restart();
  assert.deepEqual((await h.api<Installation>('/v1/installation')).body, identity);
  assert.deepEqual((await h.api<{ agents: Agent[] }>('/v1/agents')).body.agents, [writer.body, reviewer.body]);
  assert.deepEqual((await h.api<{ skills: InstructionSkill[] }>('/v1/skills')).body.skills, [skill.body]);
  const rooms = (await h.api<{ conversations: Conversation[] }>('/v1/conversations')).body.conversations;
  assert.equal(rooms.length, 3);
  assert.deepEqual(rooms.find(r => r.kind === 'shared'), room.body);
  assert.equal((await fetch(h.service.baseURL + '/v1/agents')).status, 401);
});

test('bounded enrollment expires within ten minutes and final close invalidates its device without a new enrollment', async t => {
  const directory=await mkdtemp(join(tmpdir(),'fort-bounded-enrollment-'));
  let now=Date.now(); const deadline=now+600_000;
  let service=createOpenAIService({ dataDirectory:directory,credentialDeadline:deadline,now:()=>now });
  t.after(async()=>{ await service.close(); await rm(directory,{ recursive:true,force:true }); });
  await service.start();
  const enrollment=service.issueEnrollment();
  const response=await fetch(service.baseURL+'/v1/devices/enroll',{ method:'POST',headers:{ 'Content-Type':'application/json' },body:JSON.stringify({ ...enrollment,deviceName:'Synthetic bounded device' }) });
  const device=await response.json() as { deviceToken:string; expiresAt:string };
  assert.equal(Date.parse(device.expiresAt),deadline);
  const headers={ Authorization:`Bearer ${device.deviceToken}`,'X-Fort-Installation-Id':service.installation.installationId };
  assert.throws(()=>service.issueEnrollment(),/already issued/);
  await service.close({ preserveSessionCredential:true });
  service=createOpenAIService({ dataDirectory:directory,credentialDeadline:deadline,now:()=>now });
  await service.start();
  assert.equal((await fetch(service.baseURL+'/v1/installation',{ headers })).status,200);
  await service.close();
  service=createOpenAIService({ dataDirectory:directory,credentialDeadline:deadline,now:()=>now });
  await service.start();
  assert.equal((await fetch(service.baseURL+'/v1/installation',{ headers })).status,401);
  assert.throws(()=>service.issueEnrollment(),/closed/);
});

test('closed sample admission rejects before durable acceptance or runtime work', async t => {
  let executions=0;
  const h=await setup(t,{ autoRun:true,admitSubmission:()=>false,runtime:{ async dispose(){},async execute(){ executions++;return { kind:'completed',text:'unexpected',effectiveModel:null }; } } });
  const agent=(await h.api<Agent>('/v1/agents',{ name:'Writer',model:'gpt-6-astra',instructions:'Reply.' })).body;
  const prefix=`/v1/conversations/${agent.homeConversationId}`;
  const result=await h.api(`${prefix}/messages`,{ requestId:'blocked',text:'extra sample',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,skills:[] });
  assert.equal(result.status,403);
  assert.deepEqual((await h.api<Snapshot>(`${prefix}/snapshot`)).body.runs,[]);
  assert.equal(executions,0);
});

test('bounded device authorization expires even if the service remains open',async t=>{
  let now=Date.now();
  const h=await setup(t,{credentialDeadline:now+600_000,now:()=>now});
  assert.equal((await h.api('/v1/installation')).status,200);
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply.'})).body;
  const accepted=(await h.api<Acceptance>(`/v1/conversations/${agent.homeConversationId}/messages`,{requestId:'expiry-check',text:'Synthetic sample',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,skills:[]})).body;
  now+=600_000;
  for(const action of ['cancel','close-unknown'])assert.equal((await h.api(`/v1/runs/${accepted.receipt.runId}/${action}`,{recoveryEpoch:h.service.installation.recoveryEpoch,acknowledgeUncertainty:true})).status,401);
  assert.equal((await h.api('/v1/installation')).status,401);
  assert.throws(()=>h.service.issueEnrollment(),/closed/);
});

test('explicit automatic dispatch runs new accepted work and read acknowledgement persists monotonically', async t => {
  const h = await setup(t, { autoRun: true, runtime: { async dispose() {}, async execute() { return { kind: 'completed', text: 'Saved reply', effectiveModel: null }; } } });
  const agent = (await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Reply.' })).body;
  const prefix = `/v1/conversations/${agent.homeConversationId}`;
  await h.api(`${prefix}/messages`, { requestId: 'automatic', text: 'Sample', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] });
  let snapshot = (await h.api<Snapshot>(`${prefix}/snapshot`)).body;
  for (let count = 0; count < 20 && snapshot.runs[0]?.state !== 'completed'; count++) { await new Promise(resolve => setTimeout(resolve, 5)); snapshot = (await h.api<Snapshot>(`${prefix}/snapshot`)).body; }
  assert.equal(snapshot.runs[0]?.state, 'completed');
  const message = snapshot.messages[1]!;
  assert.equal((await h.api(`${prefix}/read`, { recoveryEpoch: h.service.installation.recoveryEpoch, cursor: snapshot.cursor, messageId: message.messageId })).status, 200);
  await h.api(`${prefix}/read`, { recoveryEpoch: h.service.installation.recoveryEpoch, cursor: snapshot.cursor, messageId: snapshot.messages[0]!.messageId });
  await h.restart();
  assert.equal((await h.api<Snapshot>(`${prefix}/snapshot`)).body.lastReadMessageSequence, message.sequence);
});


test('shared submission pins its recipient and selected skill, saves one attributed reply and replays the original receipt', async t => {
  const inputs: ChatInput[] = [];
  const runtime: ChatRuntime = { async dispose() {}, async execute(input) { inputs.push(structuredClone(input)); return { kind: 'completed', text: 'red notebook [CHECKED]', effectiveModel: null }; } };
  const h = await setup(t, { runtime });
  const writer = (await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Write briefly.' })).body;
  const reviewer = (await h.api<Agent>('/v1/agents', { name: 'Reviewer', model: 'gpt-6-astra', instructions: 'Check the claim.' })).body;
  const skill = (await h.api<InstructionSkill>('/v1/skills', { name: 'Check marker', instructions: 'End with [CHECKED].' })).body;
  const room = (await h.api<Conversation>('/v1/conversations', { name: 'Sample planning', participantAgentIds: [writer.agentId, reviewer.agentId] })).body;
  const path = `/v1/conversations/${room.conversationId}/messages`;
  const command = { requestId: 'sample-shared', recoveryEpoch: h.service.installation.recoveryEpoch, bindingRevisionId: reviewer.binding.bindingRevisionId, recipientAgentId: reviewer.agentId, skills: [{ skillId: skill.skillId, revisionId: skill.revisionId }], text: '  red notebook\n' };
  assert.equal((await h.api(path, { ...command, recipientAgentId: undefined })).status, 400);
  const accepted = await h.api<Acceptance>(path, command);
  assert.equal(accepted.status, 201);
  assert.equal(inputs.length, 0);
  assert.equal(await h.service.dispatchNext(), true);
  assert.equal(inputs.length, 1);
  assert.equal(inputs[0]?.configuration.agentId, reviewer.agentId);
  assert.equal(inputs[0]?.configuration.instructions, 'Check the claim.');
  assert.deepEqual(inputs[0]?.configuration.skills, [skill]);
  assert.equal(inputs[0]?.text, command.text);
  assert.deepEqual(inputs[0]?.history, []);
  const snapshot = (await h.api<Snapshot>(`/v1/conversations/${room.conversationId}/snapshot`)).body;
  assert.equal(snapshot.runs[0]?.state, 'completed');
  assert.equal(snapshot.messages[0]?.recipientAgentId, reviewer.agentId);
  assert.equal(snapshot.messages[1]?.agentId, reviewer.agentId);
  assert.equal(snapshot.messages[1]?.agentName, 'Reviewer');
  assert.equal(snapshot.messages[1]?.text, 'red notebook [CHECKED]');
  assert.deepEqual((await h.api<Acceptance>(path, command)).body, accepted.body);
  assert.deepEqual((await h.api<Acceptance>('/v1/requests/sample-shared')).body, accepted.body);
  assert.equal((await h.api(path, { ...command, text: 'changed' })).status, 409);
  assert.equal(await h.service.dispatchNext(), false);
});

test('queued turns receive completed prior context in their own conversation, including after restart', async t => {
  const inputs: ChatInput[] = [];
  const runtime: ChatRuntime = { async dispose() {}, async execute(input) { inputs.push(structuredClone(input)); return { kind: 'completed', text: 'cedar', effectiveModel: null }; } };
  const h = await setup(t, { runtime });
  const writer = (await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Remember the sample word.' })).body;
  const reviewer = (await h.api<Agent>('/v1/agents', { name: 'Reviewer', model: 'gpt-6-astra', instructions: 'Check the sample.' })).body;
  const send = (agent: Agent, requestId: string, text: string) => h.api<Acceptance>(`/v1/conversations/${agent.homeConversationId}/messages`, { requestId, text, recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] });
  assert.equal((await send(writer, 'remember', 'Remember cedar.')).status, 201);
  assert.equal((await send(writer, 'recall-queued', 'What was the word?')).status, 201);
  assert.equal((await send(reviewer, 'unrelated', 'Explain checklists.')).status, 201);
  await h.service.dispatchNext(); await h.service.dispatchNext(); await h.service.dispatchNext();
  assert.deepEqual(inputs[0]?.history, []);
  assert.deepEqual(inputs[1]?.history.map(m => [m.role, m.text]), [['user', 'Remember cedar.'], ['assistant', 'cedar']]);
  assert.deepEqual(inputs[2]?.history, []);
  await h.restart();
  assert.equal((await send(writer, 'recall-restarted', 'What was the sample word?')).status, 201);
  await h.service.dispatchNext();
  assert.deepEqual(inputs[3]?.history.map(m => [m.role, m.text]), [['user', 'Remember cedar.'], ['assistant', 'cedar'], ['user', 'What was the word?'], ['assistant', 'cedar']]);
  assert.equal(inputs[3]?.history.every(m => m.conversationId === writer.homeConversationId), true);
});

test('restart holds old queued work and unknown outcomes without blocking a different Home', async t => {
  const calls: ChatInput[] = [];
  let started!: () => void;
  const entered = new Promise<void>(resolve => { started = resolve; });
  const runtime: ChatRuntime = { async dispose() {}, async execute(input, signal) {
    calls.push(input);
    if (input.requestId === 'uncertain') { started(); await new Promise<void>(resolve => signal.addEventListener('abort', () => resolve(), { once: true })); return { kind: 'outcomeUnknown', message: 'Stopped before confirmation.' }; }
    return { kind: 'completed', text: 'Synthetic reply', effectiveModel: null };
  } };
  const h = await setup(t, { runtime });
  const a = (await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Write.' })).body;
  const b = (await h.api<Agent>('/v1/agents', { name: 'Reviewer', model: 'gpt-6-astra', instructions: 'Review.' })).body;
  const send = (agent: Agent, requestId: string) => h.api(`/v1/conversations/${agent.homeConversationId}/messages`, { requestId, text: requestId, recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] });
  await send(a, 'uncertain'); await send(a, 'old-queued');
  const dispatch = h.service.dispatchNext(); await entered;
  await h.restart(); await dispatch;
  assert.equal(await h.service.dispatchNext(), false);
  await send(a, 'new-behind-barrier'); await send(b, 'other-home');
  assert.equal(await h.service.dispatchNext(), true);
  assert.deepEqual(calls.map(c => c.requestId), ['uncertain', 'other-home']);
  assert.equal(await h.service.dispatchNext(), false);
  const snapshot = (await h.api<Snapshot>(`/v1/conversations/${a.homeConversationId}/snapshot`)).body;
  assert.deepEqual(snapshot.runs.map(r => r.state), ['outcomeUnknown', 'queued', 'queued']);
});

test('profile edits keep stable identity, fence stale submissions and preserve already accepted configuration', async t => {
  const calls: ChatInput[] = [];
  const h = await setup(t, { runtime: { async dispose() {}, async execute(input) { calls.push(input); return { kind: 'completed', text: 'Done.', effectiveModel: null }; } } });
  const agent = (await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Original instructions.' })).body;
  const skill = (await h.api<InstructionSkill>('/v1/skills', { name: 'Marker', instructions: 'Use [ONE].' })).body;
  const command = { requestId: 'before-edit', text: 'Sample.', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [{ skillId: skill.skillId, revisionId: skill.revisionId }] };
  const path = `/v1/conversations/${agent.homeConversationId}/messages`;
  assert.equal((await h.api(path, command)).status, 201);
  const changed = await h.api<Agent>(`/v1/agents/${agent.agentId}`, { expectedBindingRevisionId: agent.binding.bindingRevisionId, name: 'Author', model: 'another-exact-model', instructions: 'Changed instructions.' });
  assert.equal(changed.status, 200);
  assert.equal(changed.body.homeConversationId, agent.homeConversationId);
  assert.notEqual(changed.body.binding.bindingRevisionId, agent.binding.bindingRevisionId);
  const changedSkill = await h.api<InstructionSkill>(`/v1/skills/${skill.skillId}`, { expectedRevisionId: skill.revisionId, name: 'Marker', instructions: 'Use [TWO].' });
  assert.equal(changedSkill.status, 200);
  assert.equal((await h.api(path, { ...command, requestId: 'stale-agent' })).status, 409);
  assert.equal((await h.api(path, { ...command, requestId: 'stale-skill', bindingRevisionId: changed.body.binding.bindingRevisionId })).status, 409);
  await h.service.dispatchNext();
  assert.equal(calls[0]?.configuration.instructions, agent.instructions);
  assert.equal(calls[0]?.configuration.selectedModel, agent.binding.selectedModel);
  assert.deepEqual(calls[0]?.configuration.skills, [skill]);
  assert.equal((await h.api(path, command)).status, 201);
});

test('message acceptance preserves whitespace-only plain text and rejects malformed Unicode before dispatch', async t => {
  const h = await setup(t);
  const agent = (await h.api<Agent>('/v1/agents', { name:'Writer', model:'gpt-6-astra', instructions:'Reply.' })).body;
  const path = `/v1/conversations/${agent.homeConversationId}/messages`;
  const command = { requestId:'whitespace', text:' \t\r\n ', recipientAgentId:agent.agentId, bindingRevisionId:agent.binding.bindingRevisionId, recoveryEpoch:h.service.installation.recoveryEpoch, skills:[] };
  const result = await h.api<Acceptance>(path,command);
  assert.equal(result.status,201);
  assert.equal(result.body.snapshot.messages[0]?.text,command.text);
  assert.equal((await h.api(path,{ ...command,requestId:'invalid-unicode',text:'\ud800' })).status,400);
});

test('service shutdown disposes its idle runtime exactly once and awaits cleanup confirmation', async t => {
  let disposalCalls = 0, confirm!: () => void;
  const cleanup = new Promise<void>(resolve => { confirm = resolve; });
  const runtime = { async execute() { return { kind:'completed' as const, text:'unused', effectiveModel:null }; }, async dispose() { disposalCalls++; await cleanup; } };
  const h = await setup(t,{ runtime });
  let closed = false;
  const closing = h.service.close().then(() => { closed = true; });
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(disposalCalls,1);
  assert.equal(closed,false);
  confirm(); await closing; await h.service.close();
  assert.equal(disposalCalls,1);
  assert.equal(closed,true);
});

test('active service shutdown confirms owned child exit and leaves no late reply or automatic retry', async t => {
  const workspace = fileURLToPath(new URL('../runner',import.meta.url));
  const owned = launchOwnedProcess({ executable:process.execPath,args:[fileURLToPath(new URL('../runner/owned-process-fixture.mjs',import.meta.url))],cwd:workspace,env:{ PATH:'/usr/bin:/bin',FORT_SYNTHETIC_MODE:'ignore-term' },deadlineMs:5_000,terminationGraceMs:50,killGraceMs:500 });
  t.after(() => owned.dispose());
  let started!: () => void;
  const entered = new Promise<void>(resolve => { started=resolve; });
  owned.connection.subscribe({ frame:bytes => { const frame=JSON.parse(Buffer.from(bytes).toString()); if(frame.result?.turn?.status==='inProgress')started(); },closed:()=>{} });
  const runtime = new CodexChat({ connection:owned.connection,workspace,runtimeVersion:'0.157.1' });
  const h = await setup(t,{ runtime });
  const agent = (await h.api<Agent>('/v1/agents',{ name:'Writer',model:'gpt-6-astra',instructions:'Reply.' })).body;
  await h.api(`/v1/conversations/${agent.homeConversationId}/messages`,{ requestId:'active-shutdown',text:'Synthetic active turn',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,skills:[] });
  const turn = h.service.dispatchNext(); await entered;
  await h.service.close(); await turn;
  assert.equal(owned.cleanupConfirmed,true);
  assert.equal(owned.exit?.signal,'SIGKILL');
  await h.restart();
  const snapshot=(await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body;
  assert.equal(snapshot.runs[0]?.state,'outcomeUnknown');
  assert.equal(snapshot.messages.length,1);
  assert.equal(await h.service.dispatchNext(),false);
});
