import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp,rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { CandidatePacket } from './session.ts';
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
