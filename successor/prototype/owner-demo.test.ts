import {test} from 'node:test';
import assert from 'node:assert/strict';
import {admitOwnerDemoSubmission} from './owner-demo-policy.ts';
import {codexLaunchPlan} from './owned-codex.ts';
import {validateOwnedProcessOptions} from '../runner/owned-process.ts';
import type {SubmissionAdmission} from '../service/openai.ts';
import type {Run} from '../contracts/v1.ts';
const submission=():SubmissionAdmission=>({text:'Hello, help me think through my day.',conversation:{conversationId:'home',name:'Writer',kind:'home',participantAgentIds:['writer']},configuration:{agentId:'writer',agentName:'Writer',bindingRevisionId:'binding',selectedModel:'gpt-6-astra',instructions:'Reply briefly using supplied context.',skills:[]}});
const run:Run={runId:'run',requestId:'request',conversationId:'home',bindingRevisionId:'binding',state:'completed',selectedModel:'gpt-6-astra',effectiveModel:null,modelEvidence:'unreported',attemptId:null};
test('owner demo admits ordinary exact text through twelve requests in the same Writer Home',()=>{
 const s=submission();s.text='  私の今日\n/hello 🌲  ';
 assert.equal(admitOwnerDemoSubmission(s,[]),true);
 assert.equal(admitOwnerDemoSubmission(s,Array(11).fill(run)),true);
 assert.equal(admitOwnerDemoSubmission(s,Array(12).fill(run)),false);
 assert.equal(admitOwnerDemoSubmission(s,[{...run,conversationId:'other'}]),false);
});
for(const change of ['model','agent','instructions','memory','skills','room','participant'] as const)test(`owner demo rejects scope expansion: ${change}`,()=>{
 const s=submission();
 if(change==='model')s.configuration.selectedModel='other';
 if(change==='agent')s.configuration.agentName='Reviewer';
 if(change==='instructions')s.configuration.instructions+='\n\nRoutine instructions:\nDo work';
 if(change==='memory')s.configuration.memory={conversationId:'home',revisionId:'memory',text:'memory'};
 if(change==='skills')s.configuration.skills=[{skillId:'skill',revisionId:'revision',name:'Skill',instructions:'Do work'}];
 if(change==='room')s.conversation.kind='shared';
 if(change==='participant')s.conversation.participantAgentIds.push('other');
 assert.equal(admitOwnerDemoSubmission(s,[]),false);
});
test('owner demo has one named nine-minute work budget without changing generic process ceiling',()=>{
 const plan=codexLaunchPlan({workspace:'/tmp/fort-test-empty',temporaryDirectory:'/tmp/fort-test-tmp',remainingWorkMs:536000,packet:'owner-demo-20261017'});
 validateOwnedProcessOptions(plan);assert.equal(plan.packetBudget?.totalMs,600000);assert.equal(plan.packetBudget?.cleanupReserveMs,60000);
 assert.throws(()=>validateOwnedProcessOptions({...plan,packetBudget:undefined}),/bounded deadline/);
 assert.throws(()=>codexLaunchPlan({workspace:'/tmp/a',temporaryDirectory:'/tmp/b',remainingWorkMs:540001,packet:'owner-demo-20261017'}),/budget/);
});

test('owner demo approval cannot be borrowed from the consumed batch or another operator',async()=>{
 const {validateOwnerDemoApproval}=await import('./owner-demo-activation.ts');
 const value={schemaVersion:1,actor:'owner',decision:'approved',scopeId:'owner-demo-20261017',scopeSHA256:'scope',operatorThreadId:'01a14bac-6acb-70be-926a-23ab4584a0f1',sourceThreadId:'01a0f307-3940-7333-96b0-a3b0b854e0e8',approvalMessageId:'Sentinel_f671f601a3148191bae54d2de3b281c1',approvedAtUTC:'2026-10-17T21:00:19Z'};
 validateOwnerDemoApproval(value,'scope',Date.parse('2026-10-17T22:00:00Z'));
 for(const patch of [{scopeId:'mvp-native-tests-20261017'},{operatorThreadId:'01a1451b-81d7-76cf-8e5c-a85d7164201c'},{approvalMessageId:'old'},{decision:'pending'},{scopeSHA256:'changed'}])assert.throws(()=>validateOwnerDemoApproval({...value,...patch},'scope',Date.parse('2026-10-17T22:00:00Z')));
});
test('owner demo claims one long bounded process and refuses a second before any enrollment',async t=>{
 const {mkdtemp,rm,readFile,realpath}=await import('node:fs/promises');const {tmpdir}=await import('node:os');const {join}=await import('node:path');const {MvpPacket}=await import('./mvp-session.ts');
 const root=await realpath(await mkdtemp(join(tmpdir(),'fort-demo-budget-')));t.after(()=>rm(root,{recursive:true,force:true}));
 const packet=new MvpPacket({profile:'owner-demo-20261017',batchRoot:root,session:1,now:()=>1000,native:{assertFresh(){},async stop(){return true;},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:budget=>{
   assert.equal(budget.packet,'owner-demo-20261017');assert.equal(budget.remainingConnections(),1);assert.equal(budget.claimLaunch(),536000);assert.equal(budget.remainingConnections(),0);assert.throws(()=>budget.claimLaunch(),/exhausted/);
   return {async preflight(){throw new Error('Synthetic stop before enrollment');},async execute(){return {kind:'notStarted',message:'fixture'};},async dispose(){}};
 }});
 await assert.rejects(packet.start(),/Synthetic stop before enrollment/);
 const state=JSON.parse(await readFile(join(root,'session-1/session-state.json'),'utf8'));assert.equal(state.launches,1);assert.equal(state.restarts,0);assert.equal(state.credentialDeadline-state.began,600000);assert.equal(state.credentialCleanup,'deleted');
 await assert.rejects(packet.start(),/consumed|exists/);
});
