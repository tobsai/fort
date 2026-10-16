import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp,rm,writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { CandidatePacket, SAMPLES } from './session.ts';
import type { Agent,InstructionSkill,Conversation,Acceptance,Snapshot } from '../contracts/v1.ts';
import type { ChatInput } from '../runner/chat-runtime.ts';

for (const mode of ['success','pending','app','dispose','late-pending'] as const) test(`bounded packet honors exact samples and restart gate: ${mode}`,async t=>{
  const root=await mkdtemp(join(tmpdir(),'fort-packet-unit-'));
  t.after(()=>rm(root,{ recursive:true,force:true }));
  const calls:ChatInput[]=[];let starts=0,disposals=0,cleanups=0,pendingCount=0,appExited=true,failDisposal=false;
  const native={ assertFresh(){},inspect(){return { appExited,pendingCount };},async cleanup(){cleanups++;return true;} };
  const packet=new CandidatePacket({ root,native,launchRuntime:()=>{ starts++;return { async preflight(){return { model:'gpt-6-astra',runtimeVersion:'0.157.1' };},async execute(input:ChatInput){calls.push(input);return { kind:'completed' as const,text:'Synthetic reply',effectiveModel:null };},async dispose(){disposals++;if(failDisposal)throw new Error('Synthetic cleanup failure');} };} });
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

test('failed or over-budget preflight consumes the first start without opening enrollment or allowing retry',async t=>{
  for (const mode of ['failed','late']) {
    const root=await mkdtemp(join(tmpdir(),'fort-packet-preflight-'));
    t.after(()=>rm(root,{recursive:true,force:true}));
    let now=Date.now(),disposed=0,executed=0;
    const options={root,now:()=>now,native:{assertFresh(){},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:()=>({
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

test('the outer deadline closes an idle session and its listener without relying on another user command',async t=>{
  const root=await mkdtemp(join(tmpdir(),'fort-packet-deadline-'));t.after(()=>rm(root,{recursive:true,force:true}));
  let now=Date.now(),stop!:()=>void,scheduled=0,disposed=0;
  const packet=new CandidatePacket({root,now:()=>now,scheduleStop:(callback,delay)=>{stop=callback;scheduled=delay;return ()=>{};},native:{assertFresh(){},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:()=>({async preflight(){return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},async execute(){return {kind:'completed',text:'unused',effectiveModel:null};},async dispose(){disposed++;}})});
  const session=await packet.start();t.after(()=>session.abort());assert.equal(scheduled,292_000);
  now+=292_000;stop();
  assert.equal((await session.closed).phase,'blocked');assert.equal(disposed,1);
  await assert.rejects(()=>fetch(session.baseURL+'/v1/installation'));
  await assert.rejects(()=>packet.start(),/cannot start/);
});
