import { test } from 'node:test';
import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';
import { mkdtempSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
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

test('failed launches and immediate child exits finish disposal after the OS reaps the child', async t => {
  const cwd = mkdtempSync(join(tmpdir(), 'fort059-owned-exit-'));
  t.after(() => rmSync(cwd, { recursive: true, force: true }));
  for (let index = 0; index < 10; index++) {
    for (const launch of [
      { executable: join(cwd, 'missing-runtime'), args: [], expectedCode: -2 },
      { executable: process.execPath, args: ['-e', 'process.exit(23)'], expectedCode: 23 },
    ]) {
      const owned = launchOwnedProcess({ ...launch, cwd, env: {}, deadlineMs: 2000, terminationGraceMs: 250, killGraceMs: 750 });
      await assert.rejects(owned.connection.request('ping', {}, new AbortController().signal));
      await owned.dispose(); await owned.dispose();
      assert.equal(owned.cleanupConfirmed, true);
      assert.equal(owned.exit?.code, launch.expectedCode);
    }
  }
});

test('unavailable group status blocks cleanup confirmation even after the owned child exits', async t => {
  const owned = launchOwnedProcess({ executable: process.execPath, args: ['-e', 'process.stdin.once("data",data=>{const request=JSON.parse(data);process.stdout.write(JSON.stringify({id:request.id,result:{pid:process.pid}})+"\\n")});setInterval(()=>{},1000)'], cwd: process.cwd(), env: {}, deadlineMs: 2000, terminationGraceMs: 25, killGraceMs: 50 });
  const result = await owned.connection.request('ping', {}, new AbortController().signal);
  assert.ok(result && typeof result === 'object' && 'pid' in result && typeof result.pid === 'number' && Number.isSafeInteger(result.pid) && result.pid > 1);
  const pid = result.pid, kill = process.kill.bind(process);
  const probe = t.mock.method(process, 'kill', (target: number, signal?: number | NodeJS.Signals) => {
    if (target === -pid && signal === 0) throw Object.assign(new Error('Synthetic unavailable group status'), { code: 'EPERM' });
    return kill(target, signal);
  });
  try {
    await assert.rejects(owned.dispose(), /exit was not confirmed/);
    assert.equal(owned.cleanupConfirmed, false);
    assert.ok(owned.exit);
    await assert.rejects(owned.dispose(), /exit was not confirmed/);
  } finally { probe.mock.restore(); }
  for (const target of [pid, -pid]) assert.throws(() => kill(target, 0), { code: 'ESRCH' });
});
