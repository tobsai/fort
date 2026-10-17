import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,writeFileSync,readFileSync,statSync,existsSync,rmSync,realpathSync} from 'node:fs';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {createHash} from 'node:crypto';
import {reconcileMvpCleanup,assertMvpSessionAvailable,MVP_CLEANUP_SCOPE_SHA256,type ReconciliationOptions} from './mvp-reconciliation.ts';
import {readMvpCleanupLedger} from './mvp-reconcile.ts';
import {DatabaseSync} from 'node:sqlite';

const digest=(bytes:Uint8Array)=>createHash('sha256').update(bytes).digest('hex');
function fixture(t:{after:(fn:()=>void)=>void}){
  const root=realpathSync(mkdtempSync(join(tmpdir(),'fort-mvp-reconcile-')));t.after(()=>rmSync(root,{recursive:true,force:true}));
  const session=join(root,'session-1'),native=join(root,'native'),installationId='synthetic-installation';mkdirSync(session);mkdirSync(native);
  const state={version:1,session:1,phase:'blocked',began:1_000,workDeadline:541_000,credentialDeadline:601_000,launches:1,restarts:1,receipts:[{runId:'a'},{runId:'b'},{runId:'c'}],credentialCleanup:'deleted',runtimeCleanup:'confirmed',nativeCleanup:'unconfirmed',installationId,port:54275};
  const statePath=join(session,'session-state.json');writeFileSync(statePath,JSON.stringify(state));writeFileSync(join(root,'active.lock'),'synthetic-retained-owner');
  const pending=join(native,createHash('sha256').update(installationId).digest('hex'));mkdirSync(pending);writeFileSync(join(pending,'pending.json'),'[]');
  const options:ReconciliationOptions={batchRoot:root,scopeSHA256:MVP_CLEANUP_SCOPE_SHA256,expectedStateSHA256:digest(readFileSync(statePath)),expectedLockSHA256:digest(readFileSync(join(root,'active.lock'))),nativeDirectory:native,assertNativeExited(){},assertRuntimeExited(){},assertListenerClosed(){},readLedger:()=>({installationId,credentialDeadline:601_000,devices:0,enrollments:0,runs:[{runId:'a',state:'completed'},{runId:'b',state:'completed'},{runId:'c',state:'completed'}]}),now:()=>700_000};
  return {root,session,native,pending,statePath,state,options};
}

test('verified reconciliation preserves the consumed state and lock bytes and releases only the unused session gate',t=>{
  const f=fixture(t),before=readFileSync(f.statePath),modified=statSync(f.statePath).mtimeMs,lock=readFileSync(join(f.root,'active.lock'));
  assert.throws(()=>assertMvpSessionAvailable(f.root,2),/lock|cleanup/);
  reconcileMvpCleanup(f.options);
  assert.deepEqual(readFileSync(f.statePath),before);assert.equal(statSync(f.statePath).mtimeMs,modified);
  assert.deepEqual(readFileSync(join(f.session,'reconciled-active.lock')),lock);assert.equal(existsSync(join(f.root,'active.lock')),false);
  assertMvpSessionAvailable(f.root,2);assert.throws(()=>assertMvpSessionAvailable(f.root,1),/consumed/);assert.throws(()=>assertMvpSessionAvailable(f.root,3),/cleanup/);
  reconcileMvpCleanup(f.options);assert.deepEqual(readFileSync(f.statePath),before);
});

for(const problem of ['state','lock','app','runtime','listener','pending','connection','pairing','devices','enrollment','ledger identity','deadline','active run','missing run','before expiry','next session'] as const)test(`reconciliation refuses ${problem} without changing the state or retained lock`,t=>{
  const f=fixture(t);
  if(problem==='state')writeFileSync(f.statePath,JSON.stringify({...f.state,restarts:0}));
  if(problem==='lock')writeFileSync(join(f.root,'active.lock'),'other-owner');
  if(problem==='app')f.options.assertNativeExited=()=>{throw new Error('running app');};
  if(problem==='runtime')f.options.assertRuntimeExited=()=>{throw new Error('running runtime');};
  if(problem==='listener')f.options.assertListenerClosed=()=>{throw new Error('open listener');};
  if(problem==='pending')writeFileSync(join(f.pending,'pending.json'),'[{}]');
  if(problem==='connection')writeFileSync(join(f.native,'connection.json'),'{}');
  if(problem==='pairing')writeFileSync(join(f.session,'local-enrollment.json'),'{}');
  const ledger=f.options.readLedger();
  if(problem==='devices')ledger.devices=1;
  if(problem==='enrollment')ledger.enrollments=1;
  if(problem==='ledger identity')ledger.installationId='other';
  if(problem==='deadline')ledger.credentialDeadline++;
  if(problem==='active run')ledger.runs[0]!.state='running';
  if(problem==='missing run')ledger.runs.pop();
  f.options.readLedger=()=>ledger;
  if(problem==='before expiry')f.options.now=()=>600_000;
  if(problem==='next session')mkdirSync(join(f.root,'session-2'));
  const before=readFileSync(f.statePath),lock=readFileSync(join(f.root,'active.lock'));
  assert.throws(()=>reconcileMvpCleanup(f.options));
  assert.deepEqual(readFileSync(f.statePath),before);assert.deepEqual(readFileSync(join(f.root,'active.lock')),lock);
  assert.equal(existsSync(join(f.session,'cleanup-reconciliation.json')),false);
});

test('a changed original state, record or archived lock invalidates the next-session gate',t=>{
  for(const target of ['session-state.json','cleanup-reconciliation.json','reconciled-active.lock']){
    const f=fixture(t);reconcileMvpCleanup(f.options);const path=join(f.session,target),bytes=readFileSync(path);
    writeFileSync(path,target==='cleanup-reconciliation.json'?JSON.stringify({...JSON.parse(bytes.toString()),scopeSHA256:'different'}):Buffer.concat([bytes,Buffer.from(' ')]));
    assert.throws(()=>assertMvpSessionAvailable(f.root,2),/cleanup/);
  }
});

test('editing the original cleanup flag cannot bypass an existing reconciliation record',t=>{
  const f=fixture(t);reconcileMvpCleanup(f.options);
  writeFileSync(f.statePath,JSON.stringify({...f.state,nativeCleanup:'confirmed'}));
  assert.throws(()=>assertMvpSessionAvailable(f.root,2),/cleanup/);
});

test('inspection does not release the lock; interrupted recording resumes without replacing evidence',t=>{
  const f=fixture(t);reconcileMvpCleanup({...f.options,inspectOnly:true});
  assert.equal(existsSync(join(f.root,'active.lock')),true);assert.equal(existsSync(join(f.session,'cleanup-reconciliation.json')),false);
  const result=reconcileMvpCleanup({...f.options,inspectOnly:true});
  assert.ok('record' in result);writeFileSync(join(f.session,'cleanup-reconciliation.json'),JSON.stringify(result.record));
  const before=readFileSync(join(f.session,'cleanup-reconciliation.json'));reconcileMvpCleanup(f.options);
  assert.deepEqual(readFileSync(join(f.session,'cleanup-reconciliation.json')),before);assertMvpSessionAvailable(f.root,2);
});

test('changed scope, reset counts and mutation during inspection cannot release the lock',t=>{
  for(const problem of ['scope','reset','race']){
    const f=fixture(t);
    if(problem==='scope')f.options.scopeSHA256='other';
    if(problem==='reset'){writeFileSync(f.statePath,JSON.stringify({...f.state,restarts:0}));f.options.expectedStateSHA256=digest(readFileSync(f.statePath));}
    if(problem==='race')f.options.assertNativeExited=()=>{writeFileSync(f.statePath,JSON.stringify({...f.state,restarts:0}));};
    assert.throws(()=>reconcileMvpCleanup(f.options));assert.equal(existsSync(join(f.root,'active.lock')),true);
  }
});

test('ledger inspection reads cleanup metadata without creating credentials or changing SQLite files',t=>{
  const f=fixture(t),path=join(f.session,'fixture.sqlite'),db=new DatabaseSync(path);
  db.exec('CREATE TABLE metadata (key TEXT PRIMARY KEY,data TEXT); CREATE TABLE devices (id TEXT); CREATE TABLE enrollment (id TEXT); CREATE TABLE entities (kind TEXT,id TEXT,data TEXT)');
  db.prepare('INSERT INTO metadata VALUES (?,?)').run('installation',JSON.stringify({installationId:f.state.installationId}));
  db.prepare('INSERT INTO metadata VALUES (?,?)').run('credentialDeadline',JSON.stringify(f.state.credentialDeadline));
  db.prepare('INSERT INTO entities VALUES (?,?,?)').run('runs','fixture-run',JSON.stringify({runId:'fixture-run',state:'completed',text:'must not appear in metadata output'}));db.close();
  const before=readFileSync(path),mtime=statSync(path).mtimeMs;
  assert.deepEqual(readMvpCleanupLedger(path),{installationId:f.state.installationId,credentialDeadline:f.state.credentialDeadline,devices:0,enrollments:0,runs:[{runId:'fixture-run',state:'completed'}]});
  assert.deepEqual(readFileSync(path),before);assert.equal(statSync(path).mtimeMs,mtime);assert.equal(existsSync(path+'-shm'),false);
  writeFileSync(path+'-wal','uncheckpointed');assert.throws(()=>readMvpCleanupLedger(path));assert.deepEqual(readFileSync(path),before);
});
