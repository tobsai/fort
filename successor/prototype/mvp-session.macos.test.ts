import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm,readFile,realpath} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {MvpPacket} from './mvp-session.ts';

test('the retired dedicated runtime packet refuses unselected account preflight and reaps its owned child',async t=>{
  const {createMvpRuntime}=await import('./mvp-runtime.ts');
  const {launchOwnedProcess}=await import('../runner/owned-process.ts');
  const {fileURLToPath}=await import('node:url');
  const base=await realpath(await mkdtemp(join(tmpdir(),'fort-mvp-owned-')));t.after(()=>rm(base,{recursive:true,force:true}));
  const children:ReturnType<typeof launchOwnedProcess>[]=[];t.after(async()=>{await Promise.all(children.map(child=>child.dispose()));});
  const packet=new MvpPacket({batchRoot:base,session:1,native:{assertFresh(){},async stop(){return true;},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:budget=>createMvpRuntime(join(base,'session-1'),budget,plan=>{
    assert.equal(plan.executable,'/opt/homebrew/Caskroom/codex/0.157.1/bin/codex');assert.ok(plan.deadlineMs+4_000<=300_000);assert.deepEqual(Object.keys(plan.env).sort(),['HOME','LANG','PATH','TMPDIR','TZ']);
    const child=launchOwnedProcess({...plan,executable:process.execPath,args:[fileURLToPath(new URL('../runner/owned-process-fixture.mjs',import.meta.url))],env:{PATH:'/usr/bin:/bin',FORT_SYNTHETIC_MODE:'complete'}});children.push(child);return child;
  },()=>{})});
  await assert.rejects(packet.start());
  assert.equal(children.length, 1);
  assert.equal(children[0]?.cleanupConfirmed, true);

});

test('a runtime constructor refusal after spawning still joins owned-process cleanup before closing the packet',async t=>{
  const {createMvpRuntime}=await import('./mvp-runtime.ts'),{launchOwnedProcess}=await import('../runner/owned-process.ts'),{fileURLToPath}=await import('node:url');
  const base=await realpath(await mkdtemp(join(tmpdir(),'fort-mvp-construction-')));t.after(()=>rm(base,{recursive:true,force:true}));
  let child:ReturnType<typeof launchOwnedProcess>|undefined;t.after(()=>child?.dispose());
  const packet=new MvpPacket({batchRoot:base,session:1,native:{assertFresh(){},async stop(){return true;},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:budget=>createMvpRuntime(join(base,'session-1'),{...budget,workDeadline:Date.now()-1},plan=>{
    child=launchOwnedProcess({...plan,executable:process.execPath,args:[fileURLToPath(new URL('../runner/owned-process-fixture.mjs',import.meta.url))],env:{PATH:'/usr/bin:/bin',FORT_SYNTHETIC_MODE:'complete'}});return child;
  },()=>{})});
  await assert.rejects(()=>packet.start());assert.ok(child);assert.equal(child.cleanupConfirmed,true);
  const state=JSON.parse(await readFile(join(base,'session-1/session-state.json'),'utf8'));assert.equal(state.phase,'blocked');assert.equal(state.runtimeCleanup,'confirmed');
});
