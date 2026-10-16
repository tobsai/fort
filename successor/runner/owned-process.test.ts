import { test } from 'node:test';
import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';
import { launchOwnedProcess,validateOwnedProcessOptions,OWNER_TRIAL_PROCESS_BUDGET,type OwnedProcessOptions } from './owned-process.ts';

test('the default process ceiling stays five minutes and arbitrary packet budget extensions fail before spawn',()=>{
  const base={executable:'/synthetic/unused',args:[],cwd:'/synthetic',env:{},deadlineMs:300_000};
  assert.deepEqual(validateOwnedProcessOptions(base),{terminationGraceMs:2_000,killGraceMs:2_000});
  assert.throws(()=>launchOwnedProcess({...base,deadlineMs:300_001}),/bounded deadline/);
  const approved={...base,deadlineMs:540_000,packetBudget:OWNER_TRIAL_PROCESS_BUDGET};
  for(const patch of [{packet:'owner-trial-20261016'},{packet:'other'},{workMs:540_001},{totalMs:600_001},{cleanupReserveMs:59_999},{workMs:600_000,totalMs:660_000},{extra:true}]){
    const malformed={...approved,packetBudget:{...OWNER_TRIAL_PROCESS_BUDGET,...patch}} as unknown as OwnedProcessOptions;
    assert.throws(()=>launchOwnedProcess(malformed),/packet-scoped process budget/);
  }
  for(const deadlineMs of [0,-1,1.5,540_001,600_000,NaN,Infinity])assert.throws(()=>launchOwnedProcess({...approved,deadlineMs}),/bounded deadline/);
  for(const field of ['terminationGraceMs','killGraceMs'] as const)for(const invalid of [0,-1,2_001,60_000,NaN,Infinity])assert.throws(()=>launchOwnedProcess({...approved,[field]:invalid}),/cleanup deadline/);
  for(const field of ['executable','cwd'] as const)assert.throws(()=>launchOwnedProcess({...approved,[field]:'relative'}),/explicit process paths/);
});

test('owned runtime disposal confirms exit even when the child ignores graceful termination', async () => {
  const owned = launchOwnedProcess({ executable:process.execPath, args:[fileURLToPath(new URL('./owned-process-fixture.mjs',import.meta.url))], cwd:fileURLToPath(new URL('.',import.meta.url)), env:{ PATH:'/usr/bin:/bin', FORT_SYNTHETIC_MODE:'ignore-term' }, deadlineMs:5_000, terminationGraceMs:50, killGraceMs:500 });
  await owned.connection.request('ping',{},new AbortController().signal);
  let lateFrames = 0;
  owned.connection.subscribe({ frame:() => { lateFrames++; }, closed:() => {} });
  await owned.dispose(); await owned.dispose();
  assert.equal(owned.cleanupConfirmed,true);
  assert.equal(owned.exit?.signal,'SIGKILL');
  assert.equal(lateFrames,0);
});

test('owned session deadline stops an idle child and disposal confirms its exit', async () => {
  const owned = launchOwnedProcess({ executable:process.execPath,args:[fileURLToPath(new URL('./owned-process-fixture.mjs',import.meta.url))],cwd:fileURLToPath(new URL('.',import.meta.url)),env:{ PATH:'/usr/bin:/bin' },deadlineMs:250,terminationGraceMs:100,killGraceMs:500 });
  await owned.connection.request('ping',{},new AbortController().signal);
  await new Promise(resolve => setTimeout(resolve,400));
  assert.equal(owned.cleanupConfirmed,true);
  await owned.dispose();
  assert.equal(owned.exit?.signal,'SIGTERM');
  await assert.rejects(owned.connection.request('ping',{},new AbortController().signal));
});
