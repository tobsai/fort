import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import type { SyntheticDispatch } from '../contracts/runner.ts';
import type { RuntimeEvidence } from '../contracts/runtime.ts';
import { startSyntheticProcess } from './synthetic-process.ts';

const make=(text='  /exact\r\ne\u0301 🦊  '):SyntheticDispatch=>({protocolVersion:1,mode:'synthetic',protocol:'codex-turn',installationId:'installation',recoveryEpoch:'epoch',hostId:'fixture-local',sessionId:'session',dispatchToken:'synthetic-only',inputDigest:createHash('sha256').update(text).digest('hex'),threadId:'thread-one',requestId:'start-one',workspace:'/synthetic/workspace',text,binding:{bindingRevisionId:'binding',provider:'fixture',runtimeVersion:'1',selectedModel:'fixture-echo-v1',hostId:'fixture-local',enabled:true},attempt:{runId:'run',attemptId:'attempt',generation:1,bindingRevisionId:'binding',runtimeVersion:'1',selectedModel:'fixture-echo-v1'}});
async function directory(t:TestContext){const path=await mkdtemp('/tmp/fort-process-test-');t.after(()=>rm(path,{recursive:true,force:true}));return path;}
const limits={handshakeMs:1000,turnMs:1500,interruptMs:100,teardownMs:100,writeMs:250,deliveryMs:250,cleanupMs:1000};

test('owned synthetic process preserves exact fragmented UTF-8 and waits for observed cleanup before terminal delivery',async t=>{
  const events:RuntimeEvidence[]=[];const dispatch=make();
  const run=startSyntheticProcess(dispatch,{directory:await directory(t),scenario:'fragmented',limits,maxWriteChunkBytes:7,deliver:async event=>{events.push(event);return {attemptId:event.attemptId,sequence:event.sequence,cursor:event.sequence,duplicate:false};}});
  const result=await run.result;
  assert.equal(result.cleanup,'observed');assert.equal(result.evidence,'confirmed');assert.equal(result.terminal,'completed');
  assert.equal(result.startWritesAttempted,1);assert(result.writeChunks>2);assert.equal(result.pendingWriteBytes,0);
  assert.deepEqual(events.map(event=>event.kind),['started','terminal']);const terminal=events.at(-1);assert(terminal?.kind==='terminal');assert.equal(terminal.messages[0].text,dispatch.text);
  assert(result.trace.indexOf('groupAbsent')<result.trace.lastIndexOf('evidenceAck'));
  assert(!JSON.stringify(result).includes(dispatch.text));
});

const receipt=(event:RuntimeEvidence)=>({attemptId:event.attemptId,sequence:event.sequence,cursor:event.sequence,duplicate:false});
const absent=(pid:number|null)=>{assert(pid);assert.throws(()=>process.kill(-pid,0),{code:'ESRCH'});};

for(const scenario of ['exit-zero','partial-eof','handshake-stall','stdout-flood','stderr-flood'] as const)test(`${scenario} cannot manufacture successful terminal evidence`,async t=>{
  const events:RuntimeEvidence[]=[];const began=performance.now();
  const run=startSyntheticProcess(make(),{directory:await directory(t),scenario,limits:{...limits,handshakeMs:150},deliver:async event=>{events.push(event);return receipt(event);}});
  const result=await run.result;
  assert(performance.now()-began<3000);assert.equal(result.cleanup,'observed');absent(result.pid);
  assert.equal(result.terminal,'unknown');assert.equal(events.at(-1)?.kind,'outcomeUnknown');assert(result.stdoutBytes<=1_000_001);assert(result.stderrBytes<=65_537);assert(result.trace.length<=128);
  if(scenario==='exit-zero'||scenario==='partial-eof')assert.equal(result.exit?.code,0);
});

for(const cancel of [false,true])test(`never-reading stdin ${cancel?'cancellation':'write timeout'} remains bounded independently of backpressure`,async t=>{
  const events:RuntimeEvidence[]=[];let evidenceAt=0;const began=performance.now();
  const run=startSyntheticProcess(make('\0'.repeat(65_536)),{directory:await directory(t),scenario:'blocked-input',limits:{...limits,writeMs:cancel?2000:75},deliver:async event=>{events.push(event);evidenceAt=performance.now();return receipt(event);}});
  const timer=cancel?setTimeout(()=>run.cancel(),100):undefined;t.after(()=>{if(timer)clearTimeout(timer);});
  const result=await run.result;
  assert.equal(result.startWritesAttempted,1);assert.equal(result.cleanup,'observed');absent(result.pid);
  assert.equal(result.terminal,'unknown');assert.equal(result.pendingWriteBytes,0);assert(result.writtenBytes<393_000);
  assert.equal(events.at(-1)?.kind,'outcomeUnknown');assert(evidenceAt-began<1000);assert(performance.now()-began<2000);
});

for(const scenario of ['ignore-term','interruptible'] as const)test(`cancel ${scenario} requires correlated terminal evidence and observed cleanup`,async t=>{
  const events:RuntimeEvidence[]=[];
  const run=startSyntheticProcess(make(),{directory:await directory(t),scenario,limits,deliver:async event=>{events.push(event);if(event.kind==='started')queueMicrotask(()=>run.cancel());return receipt(event);}});
  const result=await run.result;assert.equal(result.cleanup,'observed');absent(result.pid);assert.equal(result.startWritesAttempted,1);
  assert.equal(result.terminal,scenario==='interruptible'?'interrupted':'unknown');
  assert.equal(events.at(-1)?.kind,scenario==='interruptible'?'terminal':'outcomeUnknown');
});

for(const scenario of ['descendant','terminal-descendant'] as const)test(`${scenario} cannot leave an owned pipe-holding descendant behind`,async t=>{
  const events:RuntimeEvidence[]=[];const run=startSyntheticProcess(make(),{directory:await directory(t),scenario,limits,deliver:async event=>{events.push(event);return receipt(event);}});
  const result=await run.result;assert.equal(result.cleanup,'observed');absent(result.pid);assert.equal(result.forced,true);assert.equal(result.rootExitedBeforePipesClosed,true);
  assert.equal(result.terminal,scenario==='terminal-descendant'?'completed':'unknown');assert(result.trace.includes('signalKill'));
  if(scenario==='terminal-descendant')assert(result.trace.indexOf('groupAbsent')<result.trace.lastIndexOf('evidenceAck'));
});

test('lost acknowledgement retries identical evidence without repeating process start',async t=>{
  const events:RuntimeEvidence[]=[];const run=startSyntheticProcess(make(),{directory:await directory(t),scenario:'echo',limits,deliver:async event=>{events.push(event);if(events.length===1)throw new Error('synthetic lost acknowledgement');return receipt(event);}});
  const result=await run.result;assert.equal(result.terminal,'completed');assert.equal(result.startWritesAttempted,1);assert.equal(result.deliveryAttempts,3);assert.deepEqual(events[0],events[1]);absent(result.pid);
});

test('a hung evidence receiver cannot prevent bounded process teardown',async t=>{
  const signals:AbortSignal[]=[];const began=performance.now();
  const run=startSyntheticProcess(make(),{directory:await directory(t),scenario:'ignore-term',limits:{...limits,deliveryMs:50},deliver:async(_event,signal)=>{signals.push(signal);return new Promise(()=>{});}});
  const result=await run.result;assert.equal(result.evidence,'unconfirmed');assert.equal(result.terminal,'unknown');assert.equal(result.cleanup,'observed');assert.equal(result.deliveryAttempts,2);assert(signals.every(signal=>signal.aborted));assert(performance.now()-began<2000);absent(result.pid);
});

test('a slowly completed frame has one write deadline rather than a renewed deadline per chunk',async t=>{
  const run=startSyntheticProcess(make('x'.repeat(65_536)),{directory:await directory(t),scenario:'echo',maxWriteChunkBytes:1,limits:{...limits,writeMs:10},deliver:async event=>receipt(event)});
  const result=await run.result;assert.equal(result.failure,'writeUnavailable');assert.equal(result.terminal,'unknown');assert.equal(result.cleanup,'observed');assert(result.writtenBytes<65_536);absent(result.pid);
});

test('supervisor SIGKILL leaves uncertainty and requires an outer owner to reap its reported process group',{timeout:5000},async t=>{
  const {spawn}=await import('node:child_process');const {once}=await import('node:events');const {fileURLToPath}=await import('node:url');
  const host=spawn(process.execPath,[fileURLToPath(new URL('./synthetic-process-host.ts',import.meta.url)),await directory(t)],{env:{LANG:'en_US.UTF-8',TZ:'UTC'},stdio:['ignore','ignore','pipe','ipc']});
  let group:number|undefined;const exited=once(host,'exit');
  t.after(async()=>{host.kill('SIGKILL');if(group){try{process.kill(-group,'SIGKILL');}catch{}}await exited;});
  host.send(make('\0'.repeat(65_536)));
  const nextMessage=()=>Promise.race([once(host,'message'),exited.then(()=>{throw new Error('Synthetic host exited before its message');})]);
  const owned=await nextMessage();assert.equal(owned[0].kind,'ownedProcess');group=owned[0].pid;assert(Number.isSafeInteger(group));
  const ready=await nextMessage();assert.equal(ready[0].kind,'writeWindow');
  host.kill('SIGKILL');await exited;
  assert.doesNotThrow(()=>process.kill(-group!,0),'the outer owner must not confuse host death with descendant death');
  process.kill(-group!,'SIGKILL');
  const deadline=performance.now()+2000;
  for(;;){try{process.kill(-group!,0);}catch(error){assert.equal((error as NodeJS.ErrnoException).code,'ESRCH');break;}assert(performance.now()<deadline,'owned group did not disappear');await new Promise(resolve=>setTimeout(resolve,10));}
});


test('a burst released before start confirmation cannot accumulate an unbounded evidence backlog',async t=>{
  const began=performance.now();const run=startSyntheticProcess(make(),{directory:await directory(t),scenario:'early-burst',limits:{...limits,deliveryMs:250},deliver:async event=>{await new Promise(resolve=>setTimeout(resolve,100));return receipt(event);}});
  const result=await run.result;assert.equal(result.evidence,'unconfirmed');assert.equal(result.failure,'evidenceBacklog');assert(result.peakQueuedEvidence<=2);assert.equal(result.terminal,'unknown');assert.equal(result.cleanup,'observed');assert(performance.now()-began<2000);absent(result.pid);
});
