import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm,readFile,realpath,rename} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {MvpPacket,MVP_PROMPTS,type RuntimeBudget} from './mvp-session.ts';
import type {Agent,Acceptance,Installation} from '../contracts/v1.ts';
import type {ChatInput} from '../runner/chat-runtime.ts';

test('bounded MVP session preserves enrollment across one restart and rejects a thirteenth turn or reused start',async t=>{
  const base=await realpath(await mkdtemp(join(tmpdir(),'fort-mvp-packet-')));t.after(()=>rm(base,{recursive:true,force:true}));
  const began=Date.now();let now=began,stop=()=>{},launches=0,disposals=0,cleanups=0;
  const calls:ChatInput[]=[],budgets:number[]=[];
  const packet=new MvpPacket({batchRoot:base,session:1,now:()=>now,scheduleStop:(callback,delay)=>{assert.equal(delay,540_000);stop=callback;return ()=>{};},
    native:{assertFresh(){},async stop(){return true;},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){cleanups++;return true;}},
    launchRuntime:(budget:RuntimeBudget)=>{budgets.push(budget.claimLaunch());launches++;return {async preflight(){return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},async execute(input){calls.push(input);return {kind:'completed',text:'synthetic reply',effectiveModel:null};},async dispose(){disposals++;}};}});
  const session=await packet.start();t.after(()=>session.abort());
  const pairing=JSON.parse(await readFile(session.pairingFile,'utf8'));
  const enrolled=await fetch(session.baseURL+'/v1/devices/enroll',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...pairing,deviceName:'Synthetic MVP'})});
  const device=await enrolled.json() as {deviceToken:string;expiresAt:string};assert.equal(Date.parse(device.expiresAt),began+600_000);
  const headers={'Content-Type':'application/json',Connection:'close',Authorization:`Bearer ${device.deviceToken}`,'X-Fort-Installation-Id':session.installationId};
  const api=async<T>(path:string,body?:unknown)=>{const r=await fetch(session.baseURL+path,{method:body===undefined?'GET':'POST',headers,body:body===undefined?undefined:JSON.stringify(body)});return {status:r.status,body:await r.json() as T};};
  const agent=(await api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply briefly using supplied context.'})).body;
  const installation=(await api<Installation>('/v1/installation')).body;
  const body={requestId:'sample-0',recoveryEpoch:installation.recoveryEpoch,bindingRevisionId:agent.binding.bindingRevisionId,recipientAgentId:agent.agentId,text:MVP_PROMPTS[1][0],skills:[]};
  const path=`/v1/conversations/${agent.homeConversationId}/messages`;
  for(let i=0;i<12;i++){
    const command={...body,requestId:`sample-${i}`};const response=await api<Acceptance>(path,command);assert.equal(response.status,201);
    for(let tries=0;tries<100&&session.status().runs[i]?.state!=='completed';tries++)await new Promise(resolve=>setTimeout(resolve,2));
    assert.equal(session.status().runs[i]?.state,'completed');
    assert.deepEqual((await api<Acceptance>(path,command)).body.receipt,response.body.receipt);
    if(i===5){now=began+120_000;await session.restartService();assert.equal(session.installationId,installation.installationId);assert.equal(session.credentialDeadline,began+600_000);assert.equal((await api('/v1/installation')).status,200);}
  }
  assert.equal((await api(path,{...body,requestId:'thirteen'})).status,403);
  assert.equal((await api(path,{...body,requestId:'non-synthetic',text:'Anything not in the reviewed samples'})).status,403);
  await assert.rejects(()=>session.restartService(),/restart/);
  now=began+540_000;assert.equal((await api(path,{...body,requestId:'expired'})).status,403);stop();
  const result=await session.closed;assert.equal(result.phase,'finished');assert.equal(result.receipts.length,12);assert.equal(result.credentialCleanup,'deleted');
  assert.deepEqual({launches,disposals,cleanups},{launches:2,disposals:2,cleanups:1});assert.deepEqual(budgets,[296_000,296_000]);assert.equal(calls.length,12);
  const {DatabaseSync}=await import('node:sqlite');const ledger=new DatabaseSync(join(base,'session-1/service/openai-candidate-b.sqlite'),{readOnly:true});
  try{assert.equal(ledger.prepare('SELECT COUNT(*) AS n FROM devices').get()?.n,0);assert.equal(ledger.prepare('SELECT COUNT(*) AS n FROM enrollment').get()?.n,0);}finally{ledger.close();}
  await assert.rejects(()=>readFile(session.pairingFile));await assert.rejects(()=>fetch(session.baseURL+'/v1/installation'));
  await assert.rejects(()=>packet.start(),/consumed/);assert.equal(launches,2);
});

for(const mode of ['preflight','launch','native-stop','pending','runtime-cleanup','credential-cleanup','late-cleanup','cancel-start','ledger-moved'] as const)test(`MVP start stays consumed and cleanup failure cannot authorize another session: ${mode}`,async t=>{
  const base=await realpath(await mkdtemp(join(tmpdir(),'fort-mvp-failure-')));t.after(()=>rm(base,{recursive:true,force:true}));
  const began=Date.now();let now=began,launches=0,disposals=0,cleanups=0;const abort=new AbortController();
  const packet=new MvpPacket({batchRoot:base,session:1,now:()=>now,native:{assertFresh(){},async stop(){return mode!=='native-stop';},inspect(){return {appExited:true,pendingCount:mode==='pending'?1:0};},async cleanup(){cleanups++;if(mode==='late-cleanup')now=began+600_001;return mode!=='credential-cleanup';}},launchRuntime:budget=>{
    budget.claimLaunch();launches++;if(mode==='launch')throw new Error('Synthetic launch error');
    return {async preflight(){if(mode==='preflight')throw new Error('Synthetic setup refusal');if(mode==='cancel-start'){abort.abort();throw new Error('Synthetic setup canceled');}return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},async execute(){return {kind:'completed',text:'synthetic',effectiveModel:null};},async dispose(){disposals++;if(mode==='ledger-moved')await rename(join(base,'session-1/service'),join(base,'session-1/preserved-service'));if(mode==='runtime-cleanup')throw new Error('Synthetic cleanup refusal');}};
  }});
  if(['preflight','launch','cancel-start'].includes(mode))await assert.rejects(()=>packet.start(abort.signal));
  else{
    const session=await packet.start();
    if(mode==='pending'){
      const pairing=JSON.parse(await readFile(session.pairingFile,'utf8'));
      assert.equal((await fetch(session.baseURL+'/v1/devices/enroll',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...pairing,deviceName:'Synthetic'})})).status,201);
    }
    const finished=await session.finish();assert.equal(finished.phase,'blocked');
  }
  const state=JSON.parse(await readFile(join(base,'session-1/session-state.json'),'utf8'));assert.equal(state.phase,'blocked');
  await assert.rejects(()=>packet.start(),/consumed/);assert.equal(launches,1);assert.equal(cleanups,1);
  assert.equal(disposals,mode==='launch'?0:1);
  if(['native-stop','pending','runtime-cleanup','credential-cleanup','late-cleanup','ledger-moved'].includes(mode)){
    const next=new MvpPacket({batchRoot:base,session:2,native:{assertFresh(){},async stop(){return true;},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime(){throw Error('Must not launch');}});
    await assert.rejects(()=>next.start(),/cleanup/);
  }
});

for(const remaining of [540_000,80_000,4_001,4_000])test(`MVP owned process work fits the original deadline and four-second reap reserve: ${remaining}`,async t=>{
  const base=await realpath(await mkdtemp(join(tmpdir(),'fort-mvp-budget-')));t.after(()=>rm(base,{recursive:true,force:true}));
  let now=Date.now(),claimed=0;const began=now;
  const packet=new MvpPacket({batchRoot:base,session:1,now:()=>now,native:{assertFresh(){now=began+540_000-remaining;},async stop(){return true;},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:budget=>{
    claimed=budget.claimLaunch();assert.ok(claimed<=296_000);assert.ok(now+claimed+4_000<=began+540_000);
    return {async preflight(){return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},async execute(){throw Error('No input');},async dispose(){}};
  }});
  if(remaining===4_000){await assert.rejects(()=>packet.start(),/reserve/);assert.equal(claimed,0);}else{const session=await packet.start();assert.equal(claimed,Math.min(296_000,remaining-4_000));await session.finish();}
});

test('MVP concurrent restart has one durable winner and expiry during cleanup launches no replacement',async t=>{
  const base=await realpath(await mkdtemp(join(tmpdir(),'fort-mvp-restart-')));t.after(()=>rm(base,{recursive:true,force:true}));
  let now=Date.now(),launches=0,release:(()=>void)|undefined;const began=now;let disposal:Promise<void>|undefined;
  const packet=new MvpPacket({batchRoot:base,session:1,now:()=>now,native:{assertFresh(){},async stop(){return true;},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:budget=>{
    budget.claimLaunch();launches++;return {async preflight(){return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},async execute(){throw Error('No input');},async dispose(){disposal??=new Promise<void>(resolve=>{release=resolve;});await disposal;}};
  }});
  const session=await packet.start();const restarting=session.restartService();await assert.rejects(()=>session.restartService(),/restart/);
  for(let i=0;i<30&&!release;i++)await new Promise(resolve=>setTimeout(resolve,1));
  now=began+540_000;assert.ok(release);release();await assert.rejects(()=>restarting,/deadline|budget/);
  assert.equal(launches,1);assert.equal((await session.closed).phase,'blocked');
});

test('MVP shutdown closes a stalled HTTP body instead of spending its cleanup reserve waiting for the client',async t=>{
  const {connect}=await import('node:net');
  const base=await realpath(await mkdtemp(join(tmpdir(),'fort-mvp-stalled-')));t.after(()=>rm(base,{recursive:true,force:true}));
  const packet=new MvpPacket({batchRoot:base,session:1,native:{assertFresh(){},async stop(){return true;},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:budget=>{budget.claimLaunch();return {async preflight(){return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},async execute(){throw Error('No input');},async dispose(){}};}});
  const session=await packet.start(),socket=connect(Number(new URL(session.baseURL).port),'127.0.0.1');
  const peerClosed=new Promise<void>(resolve=>socket.once('close',()=>resolve()));
  socket.on('error',()=>{});socket.resume();
  await new Promise<void>(resolve=>socket.once('connect',resolve));socket.write('POST /v1/devices/enroll HTTP/1.1\r\nHost: localhost\r\nContent-Length: 500\r\n\r\n{');
  await new Promise(resolve=>setTimeout(resolve,10));const closing=session.finish();
  try{
    assert.notEqual(await Promise.race([closing,new Promise(resolve=>setTimeout(()=>resolve('hung'),250))]),'hung');
    assert.notEqual(await Promise.race([peerClosed.then(()=>'closed'),new Promise(resolve=>setTimeout(()=>resolve('still-open'),250))]),'still-open');
    assert.equal(socket.destroyed,true);
  }
  finally{socket.destroy();await closing;}
});

test('exactly three distinct sessions can be consumed and a later session requires prior cleanup',async t=>{
  const base=await realpath(await mkdtemp(join(tmpdir(),'fort-mvp-three-')));t.after(()=>rm(base,{recursive:true,force:true}));let launches=0;
  const options={batchRoot:base,native:{assertFresh(){},async stop(){return true;},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:(budget:RuntimeBudget)=>{budget.claimLaunch();launches++;return {async preflight(){return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},async execute(){throw Error('No input');},async dispose(){}};}};
  await assert.rejects(()=>new MvpPacket({...options,session:2}).start(),/Previous session cleanup/);
  for(const n of [1,2,3] as const){const session=await new MvpPacket({...options,session:n}).start();assert.equal((await session.finish()).phase,'finished');await assert.rejects(()=>new MvpPacket({...options,session:n}).start(),/consumed/);}
  assert.equal(launches,3);assert.throws(()=>new MvpPacket({...options,session:4 as 1}),/three/);
});

test('session abort while service restart waits for old cleanup never launches a later runtime',async t=>{
  const base=await realpath(await mkdtemp(join(tmpdir(),'fort-mvp-stop-restart-')));t.after(()=>rm(base,{recursive:true,force:true}));
  let release:(()=>void)|undefined,disposal:Promise<void>|undefined,launches=0;
  const packet=new MvpPacket({batchRoot:base,session:1,native:{assertFresh(){},async stop(){return true;},inspect(){return {appExited:true,pendingCount:0};},async cleanup(){return true;}},launchRuntime:budget=>{budget.claimLaunch();launches++;return {async preflight(){return {model:'gpt-6-astra',runtimeVersion:'0.157.1'};},async execute(){throw Error('No input');},async dispose(){disposal??=new Promise<void>(resolve=>{release=resolve;});await disposal;}};}});
  const session=await packet.start(),restart=session.restartService();void restart.catch(()=>{});
  for(let i=0;i<30&&!release;i++)await new Promise(resolve=>setTimeout(resolve,1));assert.ok(release);
  const stopped=session.abort();release();await assert.rejects(()=>restart);
  const state=await stopped;assert.equal(state.phase,'blocked');assert.equal(state.launches,1);assert.equal(launches,1);assert.equal(state.credentialCleanup,'deleted');assert.equal(state.runtimeCleanup,'confirmed');
});
