import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { lstatSync, readdirSync, realpathSync } from 'node:fs';
import { isAbsolute } from 'node:path';
import { performance } from 'node:perf_hooks';
import { SyntheticTurn, type SyntheticAction, type SyntheticDeadlines } from './synthetic-transport.ts';
import { FIXTURE_NODE } from './process.ts';
import type { SyntheticDispatch, RunnerReceipt } from '../contracts/runner.ts';
import type { RuntimeEvidence } from '../contracts/runtime.ts';

const SCENARIOS=['echo','progress-terminal-descendant','early-burst','fragmented','blocked-input','handshake-stall','exit-zero','partial-eof','stdout-flood','stderr-flood','hold','ignore-term','interruptible','descendant','terminal-descendant'] as const;
export type ProcessScenario=typeof SCENARIOS[number];
type Limits=SyntheticDeadlines & { writeMs:number;deliveryMs:number;cleanupMs:number };
const MAX:Limits={handshakeMs:5000,turnMs:30_000,interruptMs:2000,teardownMs:2000,writeMs:2000,deliveryMs:2000,cleanupMs:2000};
type Options={directory:string;scenario:ProcessScenario;limits?:Partial<Limits>;maxWriteChunkBytes?:number;deliver(event:RuntimeEvidence,signal:AbortSignal):Promise<RunnerReceipt>;signal?:AbortSignal};
export type ProcessReport={pid:number|null;exit:{code:number|null;signal:NodeJS.Signals|null}|null;cleanup:'observed'|'unverified';evidence:'confirmed'|'unconfirmed';terminal:'completed'|'interrupted'|'failed'|'unknown';failure:string|null;forced:boolean;stdoutBytes:number;stderrBytes:number;writtenBytes:number;writeChunks:number;pendingWriteBytes:number;startWritesAttempted:number;deliveryAttempts:number;peakQueuedEvidence:number;acknowledgedThrough:number;rootExitedBeforePipesClosed:boolean;trace:string[]};

export function startSyntheticProcess(input:SyntheticDispatch,options:Options) {
  if(process.version!=='v26.5.0'||!['darwin','linux'].includes(process.platform)||!SCENARIOS.includes(options.scenario))throw new Error('Unsupported synthetic process configuration');
  if(!isAbsolute(options.directory)||lstatSync(options.directory).isSymbolicLink()||!lstatSync(options.directory).isDirectory()||readdirSync(options.directory).length||(lstatSync(options.directory).mode&0o077)!==0)throw new Error('Use a fresh private synthetic process directory');
  const limits={...MAX,...options.limits};
  for(const [key,value] of Object.entries(limits))if(!(key in MAX)||!Number.isSafeInteger(value)||value<1||value>MAX[key as keyof Limits])throw new Error('Invalid synthetic process limits');
  const chunkBytes=options.maxWriteChunkBytes??16_384;
  if(!Number.isSafeInteger(chunkBytes)||chunkBytes<1||chunkBytes>65_536)throw new Error('Invalid synthetic write chunk size');
  const dispatch=structuredClone(input);const began=performance.now();const now=()=>Math.floor(performance.now()-began);
  const driver=new SyntheticTurn(dispatch,now,{handshakeMs:limits.handshakeMs,turnMs:limits.turnMs,interruptMs:limits.interruptMs,teardownMs:limits.teardownMs});
  const child=spawn(FIXTURE_NODE,[realpathSync(fileURLToPath(new URL('./process-fixture.mjs',import.meta.url))),options.scenario,dispatch.threadId],{cwd:realpathSync(options.directory),env:{LANG:'en_US.UTF-8',TZ:'UTC'},stdio:['pipe','pipe','pipe'],detached:true});
  const report:ProcessReport={pid:child.pid??null,exit:null,cleanup:'unverified',evidence:'confirmed',terminal:'unknown',failure:null,forced:false,stdoutBytes:0,stderrBytes:0,writtenBytes:0,writeChunks:0,pendingWriteBytes:0,startWritesAttempted:0,deliveryAttempts:0,peakQueuedEvidence:0,acknowledgedThrough:0,rootExitedBeforePipesClosed:false,trace:[]};
  const trace=(value:string)=>{if(report.trace.length<128)report.trace.push(value);};
  let resolveResult!:(report:ProcessReport)=>void;const result=new Promise<ProcessReport>(resolve=>{resolveResult=resolve;});
  let resolveReady!:(ready:boolean)=>void;const readiness=new Promise<boolean>(resolve=>{resolveReady=resolve;});
  let finished=false;let closing=false;let closeAt=0;let killedAt:number|undefined;let groupGone=false;let stdioClosed=false;let stdoutEnded=false;let ready=false;let hello=Buffer.alloc(0);let drainUntil:number|undefined;
  let pendingTerminal:Extract<RuntimeEvidence,{kind:'terminal'}>|undefined;
  let writing=false;const writes:Buffer[]=[];
  let delivering=false;let deliveryFailed=false;const events:RuntimeEvidence[]=[];
  let politeSignaled=false;
  const unknown=(sequence:number):RuntimeEvidence=>({protocolVersion:1,provider:'codex',...dispatch.attempt,sequence,kind:'outcomeUnknown',reason:'connectionLost'});
  const groupExists=()=>{
    if(groupGone||!child.pid)return false;
    try{process.kill(-child.pid,0);return true;}catch(error){if((error as NodeJS.ErrnoException).code==='ESRCH'){groupGone=true;trace('groupAbsent');return false;}return true;}
  };
  const signalGroup=(signal:NodeJS.Signals)=>{
    if(!child.pid||groupGone)return;
    try{process.kill(-child.pid,signal);trace(signal==='SIGTERM'?'signalTerm':'signalKill');}catch(error){if((error as NodeJS.ErrnoException).code==='ESRCH'){groupGone=true;trace('groupAbsent');}}
  };
  const forceClose=()=>{
    if(killedAt!==undefined)return;
    report.forced=true;killedAt=now();signalGroup('SIGKILL');
    writes.length=0;report.pendingWriteBytes=0;child.stdin.destroy();child.stdout.destroy();child.stderr.destroy();
  };
  const politeClose=()=>{
    if(!closing||writing||writes.length||killedAt!==undefined||politeSignaled)return;
    politeSignaled=true;child.stdin.end();signalGroup('SIGTERM');
  };
  const requestClose=(force=false)=>{
    if(!closing){closing=true;closeAt=now();trace('closeRequested');}
    if(force)forceClose();else politeClose();
  };
  const fail=(reason:string)=>{
    report.failure??=reason;
    if(pendingTerminal){if(!deliveryFailed)events.push(unknown(pendingTerminal.sequence));pendingTerminal=undefined;void pumpEvidence();}
    apply(driver.disconnected());requestClose();
  };
  const bounded=async<T>(operation:(signal:AbortSignal)=>Promise<T>,ms:number):Promise<T>=>{
    const controller=new AbortController();let timeout:ReturnType<typeof setTimeout>|undefined;
    try{return await Promise.race([operation(controller.signal),new Promise<T>((_,reject)=>{timeout=setTimeout(()=>{controller.abort();reject(new Error('Synthetic operation timed out'));},ms);})]);}
    finally{if(timeout)clearTimeout(timeout);controller.abort();}
  };
  const pumpEvidence=async()=>{
    if(delivering||deliveryFailed)return;delivering=true;
    try{
      while(events.length&&!deliveryFailed) {
        const event=events.shift()!;let confirmed=false;
        for(let attempt=0;attempt<2&&!confirmed;attempt++) {
          report.deliveryAttempts++;
          try{const receipt=await bounded(signal=>options.deliver(structuredClone(event),signal),limits.deliveryMs);if(receipt.attemptId!==event.attemptId||receipt.sequence!==event.sequence||!Number.isSafeInteger(receipt.cursor)||receipt.cursor<0)throw new Error('Uncorrelated receipt');confirmed=true;}
          catch{trace('evidenceRetry');}
        }
        if(!confirmed){deliveryFailed=true;report.evidence='unconfirmed';events.length=0;pendingTerminal=undefined;fail('evidenceUnconfirmed');break;}
        report.acknowledgedThrough=event.sequence;trace('evidenceAck');
        if(event.kind==='terminal')report.terminal=event.status;
      }
    }finally{delivering=false;maybeFinish();}
  };
  const pumpWrites=async()=>{
    if(writing||finished||killedAt!==undefined)return;writing=true;
    try{
      while(writes.length&&killedAt===undefined) {
        const bytes=writes.shift()!;const deadline=now()+limits.writeMs;
        if(JSON.parse(bytes.toString('utf8')).method==='turn/start')report.startWritesAttempted++;
        for(let offset=0;offset<bytes.length;offset+=chunkBytes) {
          if(killedAt!==undefined||now()>=deadline)throw new Error('Synthetic write deadline');
          const part=bytes.subarray(offset,offset+chunkBytes);
          await bounded(()=>new Promise<void>((resolve,reject)=>{child.stdin.write(part,error=>error?reject(error):resolve());}),Math.max(1,deadline-now()));
          report.writtenBytes+=part.length;report.writeChunks++;report.pendingWriteBytes=Math.max(0,report.pendingWriteBytes-part.length);
        }
        trace('writeComplete');
      }
    }catch{trace('writeTimedOut');fail('writeUnavailable');writes.length=0;report.pendingWriteBytes=0;child.stdin.destroy();}
    finally{writing=false;politeClose();maybeFinish();}
  };
  function apply(actions:SyntheticAction[]) {
    if(finished)return;
    for(const action of actions) {
      if(action.kind==='handshake'||action.kind==='write') {
        const bytes=Buffer.from(action.kind==='handshake'?JSON.stringify({kind:'handshake',threadId:action.threadId})+'\n':action.data);
        if(report.pendingWriteBytes+bytes.length>524_288){fail('inputLimit');break;}
        writes.push(bytes);report.pendingWriteBytes+=bytes.length;trace('writeQueued');void pumpWrites();
      } else if(action.kind==='evidence') {
        if(deliveryFailed)continue;
        if(action.event.kind==='terminal')pendingTerminal=action.event;
        else {
          if(events.length>=2){deliveryFailed=true;report.evidence='unconfirmed';events.length=0;pendingTerminal=undefined;fail('evidenceBacklog');requestClose(true);break;}
          events.push(action.event);report.peakQueuedEvidence=Math.max(report.peakQueuedEvidence,events.length);void pumpEvidence();
        }
      } else if(action.kind==='close')requestClose();
      else requestClose(true);
    }
  }
  function maybeFinish() {
    if(finished||!closing)return;
    const absent=!groupExists();
    const observed=report.exit!==null&&stdioClosed&&absent;
    const exhausted=killedAt!==undefined&&now()-killedAt>=limits.cleanupMs;
    if(!observed&&!exhausted)return;
    report.cleanup=observed?'observed':'unverified';
    if(pendingTerminal&&!deliveryFailed){const event=pendingTerminal;pendingTerminal=undefined;events.push(observed?event:unknown(event.sequence));void pumpEvidence();return;}
    if(delivering||events.length)return;
    if(!observed){report.terminal='unknown';report.failure??='cleanupUnverified';}
    finished=true;resolveReady(false);clearInterval(timer);options.signal?.removeEventListener('abort',abort);
    child.stdin.destroy();child.stdout.destroy();child.stderr.destroy();child.unref();report.pendingWriteBytes=0;resolveResult(structuredClone(report));
  }
  const read=(chunk:Buffer)=>{
    try{
      report.stdoutBytes=Math.min(1_000_001,report.stdoutBytes+chunk.length);
      if(report.stdoutBytes>1_000_000){fail('stdoutLimit');requestClose(true);return;}
      for(let offset=0;offset<chunk.length&&!finished;) {
        const newline=chunk.indexOf(10,offset);const end=Math.min(chunk.length,offset+4096,newline<0?Infinity:newline+1);
        let part=chunk.subarray(offset,end);offset=end;
        if(!ready) {
          hello=Buffer.concat([hello,part]);const newline=hello.indexOf(10);
          if(newline<0){if(hello.length>4096){fail('handshakeLimit');requestClose(true);}continue;}
          if(newline>4096){fail('handshakeLimit');requestClose(true);return;}
          let message:unknown;try{message=JSON.parse(new TextDecoder('utf-8',{fatal:true}).decode(hello.subarray(0,newline)));}catch{fail('invalidHandshake');return;}
          if(!message||typeof message!=='object'||Array.isArray(message)||Object.keys(message).sort().join(',')!=='kind,threadId'||(message as {kind:unknown}).kind!=='ready'||(message as {threadId:unknown}).threadId!==dispatch.threadId){fail('invalidHandshake');return;}
          part=hello.subarray(newline+1);hello=Buffer.alloc(0);ready=true;apply(driver.ready(dispatch.threadId));resolveReady(true);
        }
        if(part.length)apply(driver.receive(part));
      }
    }finally{maybeFinish();}
  };
  child.stdin.on('error',()=>{});
  child.stdout.on('data',chunk=>{try{read(chunk);}catch{fail('readUnavailable');}});
  child.stderr.on('data',chunk=>{report.stderrBytes=Math.min(65_537,report.stderrBytes+chunk.length);if(report.stderrBytes>65_536){fail('stderrLimit');requestClose(true);}});
  child.stdout.once('end',()=>{stdoutEnded=true;apply(driver.disconnected());requestClose();});
  child.once('error',()=>{report.exit={code:null,signal:null};fail('spawnUnavailable');});
  child.once('exit',(code,signal)=>{report.exit={code,signal};report.rootExitedBeforePipesClosed=!stdoutEnded;drainUntil=now()+limits.teardownMs;trace('exit');});
  child.once('close',()=>{stdioClosed=true;trace('stdioClosed');maybeFinish();});
  const abort=()=>{fail('supervisorAborted');requestClose(true);};
  const timer=setInterval(()=>{
    apply(driver.tick());
    if(!closing&&report.exit!==null&&!stdoutEnded&&drainUntil!==undefined&&now()>=drainUntil)fail('pipeHeldOpen');
    if(closing&&killedAt===undefined&&now()-closeAt>=limits.teardownMs)forceClose();
    maybeFinish();
  },5);
  options.signal?.addEventListener('abort',abort,{once:true});
  if(options.signal?.aborted)abort();else apply(driver.begin());
  return {pid:child.pid??null,ready:readiness,result,cancel(){apply(driver.cancel());}};
}
