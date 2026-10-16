import { test } from 'node:test';
import assert from 'node:assert/strict';
import { codexLaunchPlan, launchCandidateCodex, CODEX_EXECUTABLE } from './owned-codex.ts';
import {launchOwnedProcess,validateOwnedProcessOptions} from '../runner/owned-process.ts';
import {fileURLToPath} from 'node:url';

test('the full owner-trial plan passes the actual owned launcher and preserves its cleanup reserve',async()=>{
  const plan=codexLaunchPlan({workspace:fileURLToPath(new URL('../runner',import.meta.url)),temporaryDirectory:'/synthetic/tmp',packet:'owner-trial-replacement-20261016',remainingWorkMs:540_000});
  assert.deepEqual(validateOwnedProcessOptions(plan),{terminationGraceMs:2_000,killGraceMs:2_000});
  assert.equal(plan.deadlineMs,540_000);
  assert.ok(plan.deadlineMs+(plan.terminationGraceMs??2_000)+(plan.killGraceMs??2_000)<=600_000);
  const owned=launchOwnedProcess({...plan,executable:process.execPath,args:[fileURLToPath(new URL('../runner/owned-process-fixture.mjs',import.meta.url))]});
  try{await owned.connection.request('ping',{},new AbortController().signal);}finally{await owned.dispose();}
  assert.equal(owned.cleanupConfirmed,true);
});

for(const remainingWorkMs of [1,296_000,300_000,300_001,539_999,540_000])test(`every field of the generated owner plan crosses actual launcher validation: ${remainingWorkMs}`,()=>{
  const plan=codexLaunchPlan({workspace:'/synthetic/empty',temporaryDirectory:'/synthetic/tmp',packet:'owner-trial-replacement-20261016',remainingWorkMs});
  const cleanup=validateOwnedProcessOptions(plan);
  assert.equal(plan.deadlineMs,remainingWorkMs);
  assert.deepEqual(cleanup,{terminationGraceMs:2_000,killGraceMs:2_000});
  assert.deepEqual(plan.packetBudget,{packet:'owner-trial-replacement-20261016',workMs:540_000,totalMs:600_000,cleanupReserveMs:60_000});
  assert.equal(plan.packetBudget!.totalMs-plan.packetBudget!.workMs,60_000);
  assert.ok(cleanup.terminationGraceMs+cleanup.killGraceMs<=60_000);
  assert.ok(plan.deadlineMs+cleanup.terminationGraceMs+cleanup.killGraceMs<=600_000);
});

test('only the named owner trial permits nine minutes and preserves every other runtime setting',()=>{
  const paths={workspace:'/synthetic/empty',temporaryDirectory:'/synthetic/tmp'};
  const prior=codexLaunchPlan(paths);
  const {packetBudget,...trial}=codexLaunchPlan({...paths,packet:'owner-trial-replacement-20261016',remainingWorkMs:540_000});
  assert.deepEqual(packetBudget,{packet:'owner-trial-replacement-20261016',workMs:540_000,totalMs:600_000,cleanupReserveMs:60_000});
  assert.deepEqual({...trial,deadlineMs:prior.deadlineMs},prior);
  assert.equal(trial.deadlineMs,540_000);
  assert.throws(()=>codexLaunchPlan({...paths,remainingWorkMs:540_000}),/budget/);
  assert.throws(()=>codexLaunchPlan({...paths,packet:'owner-trial-replacement-20261016',remainingWorkMs:540_001}),/budget/);
});

test('candidate provider entry refuses use outside the approved manual packet before path inspection or launch', () => {
  assert.throws(() => launchCandidateCodex({ workspace:'/does-not-exist',temporaryDirectory:'/does-not-exist' }),/approved only through the pinned manual packet/);
  const plan = codexLaunchPlan({ workspace:'/synthetic/empty',temporaryDirectory:'/synthetic/tmp' });
  assert.equal(plan.executable,CODEX_EXECUTABLE);
  assert.deepEqual(Object.keys(plan.env).sort(),['HOME','LANG','PATH','TMPDIR','TZ']);
  assert.equal(plan.deadlineMs + (plan.terminationGraceMs ?? 0) + (plan.killGraceMs ?? 0),300_000);
  assert.equal(plan.args.includes('analytics.enabled=false'),true);
  assert.equal(plan.args.includes('skip_host_skill_discovery'),true);
});

test('the inert launch plan supplies a process-scoped no-files profile without a settings file or legacy sandbox override',()=>{
  const plan=codexLaunchPlan({workspace:'/synthetic/empty',temporaryDirectory:'/synthetic/tmp'});
  const overrides=plan.args.filter((_value,index)=>plan.args[index-1]==='-c');
  assert.equal(overrides.includes('default_permissions="fort_candidate_b_no_files"'),true);
  assert.equal(overrides.includes('permissions.fort_candidate_b_no_files={filesystem={"/**"="deny"},network={enabled=false}}'),true);
  assert.equal(overrides.some(value=>value.startsWith('sandbox_mode=')||value.startsWith('sandbox_workspace_write=')),false);
  assert.equal(plan.args.includes('--config-file'),false);
});
