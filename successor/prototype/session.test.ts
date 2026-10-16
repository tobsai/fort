import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp,rm,writeFile,readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { CandidatePacket, SAMPLES } from './session.ts';
import type { Agent,InstructionSkill,Conversation,Acceptance,Snapshot } from '../contracts/v1.ts';
import type { ChatInput } from '../runner/chat-runtime.ts';
import {StartupCheckError} from '../runner/startup-diagnostic.ts';
import {codexLaunchPlan} from './owned-codex.ts';
import {validateOwnedProcessOptions} from '../runner/owned-process.ts';

for(const elapsedMs of [0,120_000,539_999])test(`controller remaining budget reaches actual launcher validation with the complete cleanup reserve: ${elapsedMs}`,async t=>{
  const root=await mkdtemp(join(tmpdir(),'fort-owner-budget-contract-'));t.after(()=>rm(root,{recursive:true,force:true}));
  const began=Date.now();let now=began,checked=false;
  const packet=new CandidatePacket({root,attempt:'owner-trial-replacement-20261016',now:()=>now,scheduleStop:()=>()=>{},
    native:{assertFresh(){now+=elapsedMs;},async stop(){return true;},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},
    launchRuntime:remainingWorkMs=>{
      const plan=codexLaunchPlan({workspace:root,temporaryDirectory:'/synthetic/tmp',packet:'owner-trial-replacement-20261016',remainingWorkMs});
      const cleanup=validateOwnedProcessOptions(plan);
      assert.equal(plan.deadlineMs,540_000-elapsedMs);
      assert.equal(began+600_000-(now+plan.deadlineMs),60_000);
      assert.ok(now+plan.deadlineMs+cleanup.terminationGraceMs+cleanup.killGraceMs<=began+600_000);
      checked=true;return {async preflight(){return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},async execute(){throw Error('No message expected');},async dispose(){}};
    }});
  const session=await packet.start();t.after(()=>session.abort());
  assert.equal(checked,true);assert.equal(session.credentialDeadline-session.workDeadline,60_000);
  assert.equal(session.workDeadline,began+540_000);assert.equal(session.credentialDeadline,began+600_000);
  assert.equal((await session.finish()).phase,'finished');
});

for(const count of [0,1,4,5])test(`owner trial admits up to five chosen messages and automatically closes: ${count}`,async t=>{
  const root=await mkdtemp(join(tmpdir(),'fort-owner-trial-'));t.after(()=>rm(root,{recursive:true,force:true}));
  const began=Date.now();let now=began,stop!:()=>void,scheduled=0,stops=0,disposals=0,cleanups=0,budget=0,exited=false;
  const calls:ChatInput[]=[];
  const options={root,attempt:'owner-trial-replacement-20261016' as const,now:()=>now,scheduleStop:(callback:()=>void,delay:number)=>{stop=callback;scheduled=delay;return ()=>{};},
    native:{assertFresh(){},async stop(){stops++;exited=true;return true;},inspect(){return {appExited:exited,pendingCount:0};},async cleanup(){cleanups++;return true;}},
    launchRuntime:(remaining:number)=>{budget=remaining;return {async preflight(){return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},async execute(input:ChatInput){calls.push(input);return {kind:'completed' as const,text:'Synthetic reply',effectiveModel:null};},async dispose(){disposals++;}};}};
  const packet=new CandidatePacket(options),session=await packet.start();t.after(()=>session.abort());
  assert.equal(session.workDeadline,began+540_000);assert.equal(session.credentialDeadline,began+600_000);assert.equal(scheduled,540_000);assert.equal(budget,540_000);
  const enrollment=await fetch(session.baseURL+'/v1/devices/enroll',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...session.enrollment,deviceName:'Synthetic owner trial'})});
  const {deviceToken}=await enrollment.json() as {deviceToken:string};
  const headers={'Content-Type':'application/json',Connection:'close',Authorization:`Bearer ${deviceToken}`,'X-Fort-Installation-Id':session.installationId};
  async function api<T>(path:string,body?:unknown){const result=await fetch(session.baseURL+path,{method:body===undefined?'GET':'POST',headers,body:body===undefined?undefined:JSON.stringify(body)});return {status:result.status,body:await result.json() as T};}
  const writer=(await api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply briefly using supplied context.'})).body;
  const epoch=(await api<{recoveryEpoch:string}>('/v1/installation')).body.recoveryEpoch;
  const path=`/v1/conversations/${writer.homeConversationId}/messages`;
  const submission={requestId:'owner-0',recoveryEpoch:epoch,bindingRevisionId:writer.binding.bindingRevisionId,recipientAgentId:writer.agentId,text:'A personally chosen question.',skills:[]};
  const reviewer=(await api<Agent>('/v1/agents',{name:'Reviewer',model:'gpt-6-astra',instructions:'Check the sample claim.'})).body;
  const skill=(await api<InstructionSkill>('/v1/skills',{name:'Check marker',instructions:'End with [CHECKED].'})).body;
  const shared=(await api<Conversation>('/v1/conversations',{name:'Denied shared',participantAgentIds:[writer.agentId,reviewer.agentId]})).body;
  assert.equal((await api(path,{...submission,requestId:'with-skill',skills:[{skillId:skill.skillId,revisionId:skill.revisionId}]})).status,403);
  assert.equal((await api(`/v1/conversations/${reviewer.homeConversationId}/messages`,{...submission,requestId:'other-agent',bindingRevisionId:reviewer.binding.bindingRevisionId,recipientAgentId:reviewer.agentId})).status,403);
  assert.equal((await api(`/v1/conversations/${shared.conversationId}/messages`,{...submission,requestId:'shared'})).status,403);
  for(let index=0;index<count;index++){
    const message={...submission,requestId:`owner-${index}`,text:`  Owner question ${index}: café\nPlease answer briefly.  `};
    const accepted=await api<Acceptance>(path,message);assert.equal(accepted.status,201);
    for(let count=0;count<50&&session.status().runs[index]?.state!=='completed';count++)await new Promise(resolve=>setTimeout(resolve,2));
    assert.equal(session.status().runs[index]?.state,'completed');
    const replay=await api<Acceptance>(path,message);assert.equal(replay.status,201);assert.deepEqual(replay.body.receipt,accepted.body.receipt);
    assert.equal(calls[index]?.text,message.text);
  }
  if(count===5)assert.equal((await api(path,{...submission,requestId:'sixth'})).status,403);
  now=began+540_000;
  assert.equal((await api(path,{...submission,requestId:'after-cutoff'})).status,403);
  now=began+540_001;stop();
  const result=await session.closed;
  assert.equal(result.phase,'finished');assert.equal(result.receipts.length,count);assert.equal(result.credentialCleanup,'deleted');
  assert.deepEqual({stops,disposals,cleanups},{stops:1,disposals:1,cleanups:1});assert.equal(calls.length,count);
  await assert.rejects(()=>fetch(session.baseURL+'/v1/installation'));
  await assert.rejects(()=>packet.start(),/cannot start/);await assert.rejects(()=>new CandidatePacket(options).start(),/cannot start/);
});

for(const mode of ['stop-refused','stop-threw','app','pending','cleanup','late-cleanup','abort','preflight','no-stop'] as const)test(`owner trial fails closed and consumes its start: ${mode}`,async t=>{
  const root=await mkdtemp(join(tmpdir(),'fort-owner-failure-'));t.after(()=>rm(root,{recursive:true,force:true}));
  const began=Date.now();let now=began,stops=0,launches=0,disposals=0,cleanups=0;
  const native={assertFresh(){},...(mode==='no-stop'?{}:{async stop(){stops++;if(mode==='stop-threw')throw Error('Synthetic stop failure');return mode!=='stop-refused';}}),inspect(){return {appExited:mode!=='app',pendingCount:mode==='pending'?1:0};},async cleanup(){cleanups++;if(mode==='late-cleanup')now=began+600_001;return mode!=='cleanup';}};
  const options={root,attempt:'owner-trial-replacement-20261016' as const,now:()=>now,native,launchRuntime:()=>{launches++;return {async preflight(){if(mode==='preflight')throw Error('Synthetic preflight failure');return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},async execute(){throw Error('No fixture message expected');},async dispose(){disposals++;}};}};
  const packet=new CandidatePacket(options);
  if(mode==='preflight'||mode==='no-stop')await assert.rejects(()=>packet.start());
  else{const session=await packet.start();t.after(()=>session.abort());const result=await (mode==='abort'?session.abort():session.finish());assert.equal(result.phase,'blocked');assert.equal(result.receipts.length,0);assert.equal(result.credentialCleanup,mode==='cleanup'?'unconfirmed':'deleted');}
  await assert.rejects(()=>packet.start(),/cannot start/);
  assert.deepEqual({launches,stops,disposals,cleanups},mode==='no-stop'?{launches:0,stops:0,disposals:0,cleanups:0}:{launches:1,stops:1,disposals:1,cleanups:1});
});

for(const mode of ['success','pending','app','cleanup','late-cleanup'] as const)test(`fourth packet admits one exact Writer Home sample and cannot restart: ${mode}`,async t=>{
  const root=await mkdtemp(join(tmpdir(),'fort-packet-fourth-'));t.after(()=>rm(root,{recursive:true,force:true}));
  const began=Date.now();let now=began,starts=0,disposals=0,cleanups=0;
  const calls:ChatInput[]=[];
  const native={assertFresh(){},inspect(){return {appExited:mode!=='app',pendingCount:mode==='pending'?1:0};},async cleanup(){cleanups++;if(mode==='late-cleanup')now=began+300_001;return mode!=='cleanup';}};
  const options={root,attempt:'fourth-20261016' as const,now:()=>now,native,launchRuntime:()=>{
    starts++;return {async preflight(){return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},async execute(input:ChatInput){calls.push(input);return {kind:'completed' as const,text:'cedar',effectiveModel:null};},async dispose(){disposals++;}};
  }};
  const packet=new CandidatePacket(options),session=await packet.start();t.after(()=>session.abort());
  assert.equal(session.workDeadline,began+292_000);
  assert.equal(session.credentialDeadline,began+300_000);
  const response=await fetch(session.baseURL+'/v1/devices/enroll',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...session.enrollment,deviceName:'Synthetic fourth device'})});
  const {deviceToken}=await response.json() as {deviceToken:string};
  const headers={'Content-Type':'application/json',Connection:'close',Authorization:`Bearer ${deviceToken}`,'X-Fort-Installation-Id':session.installationId};
  async function api<T>(path:string,body?:unknown){const result=await fetch(session.baseURL+path,{method:body===undefined?'GET':'POST',headers,body:body===undefined?undefined:JSON.stringify(body)});return {status:result.status,body:await result.json() as T};}
  const writer=(await api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply briefly using supplied context.'})).body;
  const epoch=(await api<{recoveryEpoch:string}>('/v1/installation')).body.recoveryEpoch;
  const text='Remember the sample word cedar. Reply with that word only.';
  const submission={requestId:'sole-sample',recoveryEpoch:epoch,bindingRevisionId:writer.binding.bindingRevisionId,recipientAgentId:writer.agentId,text,skills:[]};
  const path=`/v1/conversations/${writer.homeConversationId}/messages`;
  const skill=(await api<InstructionSkill>('/v1/skills',{name:'Check marker',instructions:'End with [CHECKED].'})).body;
  const reviewer=(await api<Agent>('/v1/agents',{name:'Reviewer',model:'gpt-6-astra',instructions:'Check the sample claim.'})).body;
  const shared=(await api<Conversation>('/v1/conversations',{name:'Synthetic denied shared',participantAgentIds:[writer.agentId,reviewer.agentId]})).body;
  assert.equal((await api(path,{...submission,requestId:'wrong-text',text:'Corrective prompt'})).status,403);
  assert.equal((await api(path,{...submission,requestId:'with-skill',skills:[{skillId:skill.skillId,revisionId:skill.revisionId}]})).status,403);
  assert.equal((await api(`/v1/conversations/${reviewer.homeConversationId}/messages`,{...submission,requestId:'wrong-agent',bindingRevisionId:reviewer.binding.bindingRevisionId,recipientAgentId:reviewer.agentId})).status,403);
  assert.equal((await api(`/v1/conversations/${shared.conversationId}/messages`,{...submission,requestId:'wrong-conversation'})).status,403);
  assert.equal(calls.length,0);
  const accepted=await api<Acceptance>(path,submission);assert.equal(accepted.status,201);
  for(let count=0;count<50&&session.status().runs[0]?.state!=='completed';count++)await new Promise(resolve=>setTimeout(resolve,2));
  assert.equal(session.status().runs[0]?.state,'completed');
  assert.equal((await api(path,{...submission,requestId:'extra-sample'})).status,403);
  const replay=await api<Acceptance>(path,submission);assert.equal(replay.status,201);assert.deepEqual(replay.body.receipt,accepted.body.receipt);
  const result=await session.finish();
  assert.equal(result.phase,mode==='success'?'finished':'blocked');assert.equal(result.attempt,'fourth-20261016');assert.equal(result.starts,1);
  assert.equal(result.receipts.length,1);assert.equal(result.credentialCleanup,mode==='cleanup'?'unconfirmed':'deleted');
  assert.equal(starts,1);assert.equal(disposals,1);assert.equal(cleanups,1);assert.deepEqual(calls.map(input=>input.text),[text]);
  await assert.rejects(()=>packet.start(),/cannot start/);
  await assert.rejects(()=>new CandidatePacket(options).start(),/cannot start/);
  assert.equal(starts,1);
});

for (const mode of ['success','pending','app','dispose','late-pending','replacement','replacement-late-cleanup','third','third-late-cleanup'] as const) test(`bounded packet honors exact samples and restart gate: ${mode}`,async t=>{
  const root=await mkdtemp(join(tmpdir(),'fort-packet-unit-'));
  t.after(()=>rm(root,{ recursive:true,force:true }));
  const replacement=mode.startsWith('replacement')||mode.startsWith('third');
  const attempt=mode.startsWith('third')?'third-20261016' as const:'replacement-20261016' as const;let now=Date.now();
  const calls:ChatInput[]=[];let starts=0,disposals=0,cleanups=0,pendingCount=0,appExited=true,failDisposal=false;
  const native={ assertFresh(){},inspect(){return { appExited,pendingCount };},async cleanup(){cleanups++;if(mode.endsWith('late-cleanup'))now+=300_001;return true;} };
  const packet=new CandidatePacket({ root,native,...(replacement?{attempt,now:()=>now}:{}),launchRuntime:()=>{ starts++;return { async preflight(){return { model:'gpt-6-astra',runtimeVersion:'0.157.1' };},async execute(input:ChatInput){calls.push(input);return { kind:'completed' as const,text:'Synthetic reply',effectiveModel:null };},async dispose(){disposals++;if(failDisposal)throw new Error('Synthetic cleanup failure');} };} });
  const first=await packet.start();
  t.after(()=>first.abort());
  const enrolled=await fetch(first.baseURL+'/v1/devices/enroll',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...first.enrollment,deviceName:'Synthetic packet device'})});
  const device=await enrolled.json() as { deviceToken:string };
  let baseURL=first.baseURL;
  const headers={'Content-Type':'application/json',Connection:'close',Authorization:`Bearer ${device.deviceToken}`,'X-Fort-Installation-Id':first.installationId};
  async function api<T>(path:string,body?:unknown){ const response=await fetch(baseURL+path,{method:body===undefined?'GET':'POST',headers,body:body===undefined?undefined:JSON.stringify(body)});return {status:response.status,body:await response.json() as T}; }
  const writer=(await api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply briefly using supplied context.'})).body;
  const reviewer=(await api<Agent>('/v1/agents',{name:'Reviewer',model:'gpt-6-astra',instructions:'Check the sample claim.'})).body;
  const skill=(await api<InstructionSkill>('/v1/skills',{name:'Check marker',instructions:'End with [CHECKED].'})).body;
  const room=(await api<Conversation>('/v1/conversations',{name:'Sample planning',participantAgentIds:[writer.agentId,reviewer.agentId]})).body;
  const epoch=(await api<{recoveryEpoch:string}>('/v1/installation')).body.recoveryEpoch;
  async function send(index:number,text:string=SAMPLES[index]!.text){
    const agent=index===1 || index===3 ? reviewer:writer;
    return api<Acceptance>(`/v1/conversations/${index===3?room.conversationId:agent.homeConversationId}/messages`,{requestId:`sample-${index}`,recoveryEpoch:epoch,bindingRevisionId:agent.binding.bindingRevisionId,recipientAgentId:agent.agentId,text,skills:index===2?[{skillId:skill.skillId,revisionId:skill.revisionId}]:[]});
  }
  assert.equal((await send(0,'unauthorized extra prompt')).status,403);
  assert.equal(calls.length,0);
  for(let index=0;index<4;index++){
    const result=await send(index);assert.equal(result.status,201);
    for(let count=0;count<30;count++){
      const snapshot=(await api<Snapshot>(`/v1/conversations/${result.body.receipt.conversationId}/snapshot`)).body;
      if(snapshot.runs.some(r=>r.runId===result.body.receipt.runId&&r.state==='completed'))break;
      await new Promise(resolve=>setTimeout(resolve,2));
    }
  }
  if(replacement){
    assert.equal(first.credentialDeadline-first.workDeadline,8_000);
    assert.equal((await send(4)).status,201);
    for(let count=0;count<30;count++){
      if(first.status().runs.every(run=>run.state==='completed'))break;
      await new Promise(resolve=>setTimeout(resolve,2));
    }
    assert.equal((await send(5,'unapproved sixth sample')).status,403);
    const result=await first.finish();
    assert.equal(result.phase,mode.endsWith('late-cleanup')?'blocked':'finished');assert.equal(result.starts,1);assert.equal(result.attempt,attempt);
    assert.equal(result.credentialCleanup,'deleted');assert.equal(cleanups,1);
    assert.equal(disposals,1);assert.deepEqual(calls.map(input=>input.text),SAMPLES.map(sample=>sample.text));
    await assert.rejects(()=>packet.start(),/cannot start/);
    const reopened=new CandidatePacket({root,native,attempt,launchRuntime(){throw new Error('No second start');}});
    await assert.rejects(()=>reopened.start(),/cannot start/);
    assert.equal(starts,1);return;
  }
  assert.equal((await send(4)).status,403);
  if (mode!=='success'&&mode!=='late-pending') {
    pendingCount=mode==='pending'?1:0;appExited=mode!=='app';failDisposal=mode==='dispose';
    assert.equal((await first.finish()).phase,'blocked');
    assert.equal(starts,1);assert.equal(calls.length,4);assert.equal(cleanups,1);
    await assert.rejects(()=>packet.start(),/cannot start/);return;
  }
  assert.equal((await first.finish()).phase,'between');
  assert.equal(disposals,1);assert.equal(cleanups,0);
  if(mode==='late-pending'){
    pendingCount=1;await assert.rejects(()=>packet.start(),/Restart gate/);
    pendingCount=0;await assert.rejects(async()=>{const unexpected=await packet.start();t.after(()=>unexpected.abort());},/cannot start/);
    assert.equal(starts,1);assert.equal(cleanups,1);return;
  }
  const second=await packet.start();baseURL=second.baseURL;
  t.after(()=>second.abort());
  assert.equal(second.enrollment,undefined);
  assert.equal(second.installationId,first.installationId);
  const fifth=await send(4);assert.equal(fifth.status,201);
  for(let count=0;count<30;count++){
    const snapshot=(await api<Snapshot>(`/v1/conversations/${writer.homeConversationId}/snapshot`)).body;
    if(snapshot.runs.some(run=>run.runId===fifth.body.receipt.runId&&run.state==='completed'))break;
    await new Promise(resolve=>setTimeout(resolve,2));
  }
  assert.equal((await second.finish()).phase,'finished');
  assert.equal(starts,2);assert.equal(disposals,2);assert.equal(cleanups,1);
  assert.deepEqual(calls.map(input=>input.text),SAMPLES.map(sample=>sample.text));
  await assert.rejects(()=>packet.start(),/cannot start/);
});

for(const attempt of ['third-20261016','fourth-20261016'] as const)test(`failed or over-budget preflight consumes the first start without enrollment or retry: ${attempt}`,async t=>{
  for (const mode of ['failed','late']) {
    const root=await mkdtemp(join(tmpdir(),'fort-packet-preflight-'));
    t.after(()=>rm(root,{recursive:true,force:true}));
    let now=Date.now(),disposed=0,executed=0;
    const options={root,attempt,now:()=>now,native:{assertFresh(){},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:()=>({
      async preflight(){if(mode==='failed')throw new Error('synthetic failure');now+=296_001;return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},
      async execute(){executed++;return {kind:'completed' as const,text:'unused',effectiveModel:null};},async dispose(){disposed++;},
    })};
    await assert.rejects(()=>new CandidatePacket(options).start(),/preflight failed/);
    await assert.rejects(()=>new CandidatePacket(options).start(),/cannot start/);
    assert.equal(disposed,1);assert.equal(executed,0);
  }
});

test('an existing session lock refuses a parallel start before launching a runtime',async t=>{
  const root=await mkdtemp(join(tmpdir(),'fort-packet-lock-'));t.after(()=>rm(root,{recursive:true,force:true}));
  await writeFile(join(root,'active.lock'),'synthetic ownership marker');
  let launches=0;
  const packet=new CandidatePacket({root,native:{assertFresh(){},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:()=>{launches++;return {async preflight(){return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},async execute(){return {kind:'completed',text:'unused',effectiveModel:null};},async dispose(){}};}});
  await assert.rejects(async()=>{const session=await packet.start();t.after(()=>session.abort());},/already active/);
  assert.equal(launches,0);
});

for(const attempt of ['third-20261016','fourth-20261016'] as const)test(`outer deadline closes an idle session and listener without another user command: ${attempt}`,async t=>{
  const root=await mkdtemp(join(tmpdir(),'fort-packet-deadline-'));t.after(()=>rm(root,{recursive:true,force:true}));
  const began=Date.now();let now=began,stop!:()=>void,scheduled=0,disposed=0;
  const packet=new CandidatePacket({root,attempt,now:()=>now,scheduleStop:(callback,delay)=>{stop=callback;scheduled=delay;return ()=>{};},native:{assertFresh(){now+=1_000;},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:()=>({async preflight(){return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},async execute(){return {kind:'completed',text:'unused',effectiveModel:null};},async dispose(){disposed++;}})});
  const session=await packet.start();t.after(()=>session.abort());assert.equal(scheduled,291_000);assert.equal(session.credentialDeadline,began+300_000);
  now+=291_000;stop();
  assert.equal((await session.closed).phase,'blocked');assert.equal(disposed,1);
  await assert.rejects(()=>fetch(session.baseURL+'/v1/installation'));
  await assert.rejects(()=>packet.start(),/cannot start/);
});

for(const attempt of [undefined,'fourth-20261016'] as const)for(const cooperative of [false,true])test(`cancellation during blocked preflight consumes the start: ${attempt??'original'} / ${cooperative}`,async t=>{
  const root=await mkdtemp(join(tmpdir(),'fort-packet-cancel-'));t.after(()=>rm(root,{recursive:true,force:true}));
  const cancellation=new AbortController(),entered=Promise.withResolvers<AbortSignal>(),preflight=Promise.withResolvers<{model:string;runtimeVersion:string}>();
  let launches=0,disposals=0,executions=0,cleanups=0;
  const packet=new CandidatePacket({root,...(attempt?{attempt}:{}),native:{assertFresh(){},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){cleanups++;return true;}},launchRuntime:()=>{
    launches++;return {preflight(signal:AbortSignal){entered.resolve(signal);if(cooperative)signal.addEventListener('abort',()=>preflight.reject(new StartupCheckError({stage:'protocol',check:'initialize'})),{once:true});return preflight.promise;},async execute(){executions++;return {kind:'completed' as const,text:'unused',effectiveModel:null};},async dispose(){disposals++;}};
  }});
  const starting=packet.start(cancellation.signal);
  let settled=false;
  const result=starting.then(async session=>{await session.abort();settled=true;return 'unexpected success';},()=>{settled=true;return 'rejected';});
  const signal=await entered.promise;cancellation.abort();
  await new Promise(resolve=>setImmediate(resolve));
  const observed={preflightCanceled:signal.aborted,disposals,settled};
  preflight.resolve({model:'gpt-6-astra',runtimeVersion:'0.157.1'});
  assert.equal(await result,'rejected');
  assert.deepEqual(observed,{preflightCanceled:true,disposals:1,settled:true});
  assert.equal(launches,1);assert.equal(disposals,1);assert.equal(executions,0);assert.equal(cleanups,1);
  const state=JSON.parse(await readFile(join(root,'session-state.json'),'utf8'));
  assert.equal(state.phase,'blocked');assert.equal(state.starts,1);assert.equal(state.port,undefined);
  assert.equal(state.credentialCleanup,'deleted');
  assert.deepEqual(state.failure,{stage:'session',check:'cancelled-or-expired'});
  await assert.rejects(()=>packet.start(),/cannot start/);
});

test('an already canceled start is blocked without launching a runtime or allowing another attempt',async t=>{
  const root=await mkdtemp(join(tmpdir(),'fort-packet-precanceled-'));t.after(()=>rm(root,{recursive:true,force:true}));
  let launches=0;
  const packet=new CandidatePacket({root,native:{assertFresh(){},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:()=>{launches++;throw new Error('Unexpected runtime launch');}});
  const cancellation=new AbortController();cancellation.abort();
  await assert.rejects(()=>packet.start(cancellation.signal),/preflight failed/);
  const state=JSON.parse(await readFile(join(root,'session-state.json'),'utf8'));
  assert.equal(state.phase,'blocked');assert.equal(state.starts,1);assert.equal(state.port,undefined);
  await assert.rejects(()=>packet.start(),/cannot start/);assert.equal(launches,0);
});

test('the same cancellation handle closes a ready session and revokes its listener',async t=>{
  const root=await mkdtemp(join(tmpdir(),'fort-packet-ready-cancel-'));t.after(()=>rm(root,{recursive:true,force:true}));
  let disposals=0;
  const packet=new CandidatePacket({root,native:{assertFresh(){},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:()=>({async preflight(){return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},async execute(){throw new Error('Unexpected turn');},async dispose(){disposals++;}})});
  const cancellation=new AbortController(),session=await packet.start(cancellation.signal);t.after(()=>session.abort());
  cancellation.abort();
  assert.equal((await session.closed).phase,'blocked');assert.equal(disposals,1);
  await assert.rejects(()=>fetch(session.baseURL+'/v1/installation'));
  await assert.rejects(()=>packet.start(),/cannot start/);
});

for(const recognized of [false,true])test(`a failed startup records its redacted check without allowing retry or copying the underlying error: ${recognized}`,async t=>{
  const root=await mkdtemp(join(tmpdir(),'fort-packet-diagnostic-'));t.after(()=>rm(root,{recursive:true,force:true}));
  let disposals=0,launches=0;
  const packet=new CandidatePacket({root,native:{assertFresh(){},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:()=>{
    launches++;return {async preflight(){throw recognized?Object.assign(new StartupCheckError({stage:'config',check:'analytics.enabled'}),{message:'synthetic-secret-must-not-persist'}):new Error('synthetic-secret-must-not-persist');},async execute(){throw new Error('Unexpected turn');},async dispose(){disposals++;}};
  }});
  await assert.rejects(()=>packet.start(),/preflight failed/);
  const raw=await readFile(join(root,'session-state.json'),'utf8'),state=JSON.parse(raw);
  assert.deepEqual(state.failure,recognized?{stage:'config',check:'analytics.enabled'}:{stage:'session',check:'runtime-preflight'});
  assert.equal(raw.includes('synthetic-secret-must-not-persist'),false);
  assert.equal(state.phase,'blocked');assert.equal(state.starts,1);assert.equal(state.port,undefined);
  assert.equal(state.receipts.length,0);assert.equal(state.credentialCleanup,'deleted');
  await assert.rejects(()=>packet.start(),/cannot start/);
  assert.equal(disposals,1);assert.equal(launches,1);
});

for(const attempt of ['third-20261016','fourth-20261016','owner-trial-20261016','owner-trial-replacement-20261016'] as const)for(const priorAttempt of [undefined,'replacement-20261016','third-20261016','fourth-20261016','owner-trial-20261016'] as const)if(attempt!==priorAttempt)test(`${attempt} cannot adopt a prior packet: ${priorAttempt??'original'}`,async t=>{
  const root=await mkdtemp(join(tmpdir(),'fort-packet-original-'));t.after(()=>rm(root,{recursive:true,force:true}));
  const path=join(root,'session-state.json');
  const original=JSON.stringify({version:1,attempt:priorAttempt,starts:1,phase:'between',credentialDeadline:Date.now()+600_000,receipts:[]});
  await writeFile(path,original);
  let nativeCalls=0,launches=0;
  const packet=new CandidatePacket({root,attempt,native:{assertFresh(){nativeCalls++;},inspect(){nativeCalls++;return {appExited:true,pendingCount:0};},async cleanup(){nativeCalls++;return true;}},launchRuntime(){launches++;throw new Error('Unexpected launch');}});
  await assert.rejects(()=>packet.start(),/another attempt/);
  await assert.rejects(()=>packet.cancelBetweenSessions(),/another attempt/);
  assert.equal(await readFile(path,'utf8'),original);
  assert.equal(nativeCalls,0);assert.equal(launches,0);
});

for(const attempt of ['third-20261016','fourth-20261016','owner-trial-20261016','owner-trial-replacement-20261016'] as const)test(`freshness refusal consumes the sole start before provider launch: ${attempt}`,async t=>{
  const root=await mkdtemp(join(tmpdir(),'fort-packet-freshness-'));t.after(()=>rm(root,{recursive:true,force:true}));
  let inspections=0,launches=0;
  const options={root,attempt,native:{assertFresh(){inspections++;throw new Error('Synthetic native state is not fresh');},inspect(){throw new Error('Unexpected inspection');},async cleanup(){throw new Error('Unexpected cleanup');}},launchRuntime(){launches++;throw new Error('Unexpected launch');}};
  await assert.rejects(()=>new CandidatePacket(options).start());
  await assert.rejects(()=>new CandidatePacket(options).start(),/cannot start/);
  const state=JSON.parse(await readFile(join(root,'session-state.json'),'utf8'));
  assert.equal(state.phase,'blocked');assert.equal(state.starts,1);
  assert.deepEqual(state.failure,{stage:'session',check:'entry'});
  assert.equal(inspections,1);assert.equal(launches,0);
});
