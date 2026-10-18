import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createFixtureService } from './service.ts';
import type { SyntheticAction } from '../runner/synthetic-transport.ts';
import { CodexTurn } from '../runner/codex.ts';

async function setup(t: TestContext, options: Record<string, unknown> = {}) {
  const directory=await mkdtemp(join(tmpdir(),'fort-synthetic-bridge-'));
  let service=createFixtureService({dataDirectory:directory,syntheticRunner:true,...options});
  await service.start();
  t.after(async()=>{await service.close();await rm(directory,{recursive:true,force:true});});
  let endpoint='';
  const call=async(path:string,body?:unknown,headers:Record<string,string>={})=>{
    const response=await fetch((endpoint||service.baseURL)+path,{method:body===undefined?'GET':'POST',headers:{'Content-Type':'application/json',...headers},body:body===undefined?undefined:JSON.stringify(body)});
    return {status:response.status,body:await response.json() as any};
  };
  const enrollment=service.issueEnrollment();
  const enrolled=await call('/v1/devices/enroll',{installationId:service.installation.installationId,code:enrollment.code,deviceName:'Synthetic bridge test'});
  const ownerHeaders={Authorization:`Bearer ${enrolled.body.deviceToken}`,'X-Fort-Installation-Id':service.installation.installationId};
  const owner=(path:string,body?:unknown)=>call(path,body,ownerHeaders);
  const agent=(await owner('/v1/agents')).body.agents[0];
  const auth=service.enrollSyntheticRunner('fixture-local');
  const runnerHeaders={Authorization:`Bearer ${auth.token}`,'X-Fort-Installation-Id':service.installation.installationId};
  const scope={protocolVersion:1,installationId:service.installation.installationId,recoveryEpoch:service.installation.recoveryEpoch,hostId:'fixture-local',sessionId:auth.sessionId};
  const runner=(path:string,body:unknown)=>call('/fixture/runner/'+path,body,runnerHeaders);
  const submit=(text:string,requestId:string=crypto.randomUUID())=>owner(`/v1/conversations/${agent.homeConversationId}/messages`,{requestId,text,recoveryEpoch:scope.recoveryEpoch,bindingRevisionId:agent.binding.bindingRevisionId});
  const snap=async()=> (await owner(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body;
  const claim=async()=>{const result=await runner('claim',scope);assert.equal(result.status,200);return result.body.dispatch;};
  const ingest=(dispatch:any,event:any)=>runner('events',{...scope,dispatchToken:dispatch.dispatchToken,event});
  const restart=async()=>{endpoint='';await service.close();service=createFixtureService({dataDirectory:directory,syntheticRunner:true,...options});await service.start();};
  return {connect(url:string){endpoint=url;},get service(){return service;},directory,call,owner,ownerHeaders,runnerHeaders,scope,runner,submit,snap,claim,ingest,restart,agent,auth};
}
const frame=(message:unknown)=>Buffer.from(JSON.stringify(message)+'\n');
function wireEvents(dispatch:any,reply='  exact synthetic reply\r\n🦊  ') {
  const turn=new CodexTurn({attempt:dispatch.attempt,threadId:dispatch.threadId,requestId:dispatch.requestId,workspace:dispatch.workspace,text:dispatch.text});
  const start=turn.start();
  const started=turn.receive(frame({id:dispatch.requestId,result:{turn:{id:'synthetic-turn',status:'inProgress'}}})).events[0];
  turn.receive(frame({method:'item/completed',params:{threadId:dispatch.threadId,turnId:'synthetic-turn',item:{id:'reply-one',type:'agentMessage',text:reply,phase:'final_answer'}}}));
  const terminal=turn.receive(frame({method:'turn/completed',params:{threadId:dispatch.threadId,turn:{id:'synthetic-turn',status:'completed',error:null}}})).events[0];
  return {start,started,terminal,reply};
}

test('synthetic authenticated dispatch commits exact correlated terminal output once and replays durably',async t=>{
  const h=await setup(t);const text='  /exact\r\ne\u0301 🦊  ';
  const accepted=await h.submit(text);assert.equal(accepted.status,201);
  const dispatch=await h.claim();assert.equal(dispatch.mode,'synthetic');assert.equal(dispatch.text,text);
  assert.deepEqual(dispatch.binding,h.agent.binding);assert.equal(dispatch.attempt.runId,accepted.body.receipt.runId);
  assert.equal(dispatch.attempt.selectedModel,h.agent.binding.selectedModel);
  const wire=wireEvents(dispatch);assert.equal(JSON.parse(wire.start.writes[0]).params.input[0].text,text);
  assert.equal((await h.ingest(dispatch,wire.started)).status,200);
  const terminal=await h.ingest(dispatch,wire.terminal);assert.equal(terminal.status,200);
  const before=await h.snap();assert.equal(before.runs[0].state,'completed');assert.equal(before.messages[1].text,wire.reply);
  assert.equal(before.runs[0].effectiveModel,null);assert.equal(before.runs[0].modelEvidence,'unreported');
  assert.equal((await h.ingest(dispatch,wire.terminal)).body.duplicate,true);
  assert.deepEqual(await h.snap(),before);
  await h.restart();assert.deepEqual(await h.snap(),before);
  assert.equal((await h.ingest(dispatch,wire.terminal)).body.duplicate,true);
  assert.equal(await h.claim(),null);
});

test('runner and owner credentials have disjoint scopes; every correlation mismatch has no ledger effect',async t=>{
  const h=await setup(t);await h.submit('authority test');const dispatch=await h.claim();const wire=wireEvents(dispatch);const before=await h.snap();
  const command={...h.scope,dispatchToken:dispatch.dispatchToken,event:wire.started};
  const badScopes=[{installationId:'foreign'},{recoveryEpoch:'old-epoch'},{hostId:'other-host'},{sessionId:'forged-session'},{protocolVersion:2},{dispatchToken:'forged'}];
  for(const change of badScopes){assert.equal((await h.runner('events',{...command,...change})).status,401);assert.deepEqual(await h.snap(),before);}
  for(const change of [{runId:'foreign-run'},{attemptId:'foreign-attempt'},{generation:2},{bindingRevisionId:'foreign-binding'},{runtimeVersion:'changed-runtime'},{selectedModel:'changed-model'},{provider:'fixture'}]) {
    const response=await h.ingest(dispatch,{...wire.started,...change});assert([400,401,409].includes(response.status));assert.deepEqual(await h.snap(),before);
  }
  for(const headers of [{},{...h.runnerHeaders,Authorization:'Bearer forged'},h.ownerHeaders,{...h.runnerHeaders,Origin:'https://untrusted.example'}])assert([401,403].includes((await h.call('/fixture/runner/events',command,headers)).status));
  assert.equal((await h.call('/v1/agents',undefined,h.runnerHeaders)).status,401);
  assert.equal((await h.call(`/v1/runs/${dispatch.attempt.runId}/cancel`,{recoveryEpoch:h.scope.recoveryEpoch},h.runnerHeaders)).status,401);
  const second=h.service.enrollSyntheticRunner('fixture-local');
  assert.equal((await h.call('/fixture/runner/events',{...command,sessionId:second.sessionId},{...h.runnerHeaders,Authorization:`Bearer ${second.token}`})).status,401);
  assert.equal((await h.ingest(dispatch,wire.started)).status,200);
});

test('evidence replay is canonical and gaps, conflicts and late attempts cannot mutate the ledger',async t=>{
  const h=await setup(t);await h.submit('sequence test');const dispatch=await h.claim();const wire=wireEvents(dispatch);
  assert.equal((await h.ingest(dispatch,wire.terminal)).status,409);
  assert.equal((await h.ingest(dispatch,wire.started)).status,200);const before=await h.snap();
  const reordered=Object.fromEntries(Object.entries(wire.started).reverse());
  assert.equal((await h.ingest(dispatch,reordered)).body.duplicate,true);
  assert.equal((await h.ingest(dispatch,{...wire.started,turnId:'changed-turn'})).status,409);
  assert.deepEqual(await h.snap(),before);
  await h.restart();const unknown=await h.snap();assert.equal(unknown.runs[0].state,'outcomeUnknown');
  assert.equal((await h.ingest(dispatch,wire.terminal)).status,409);assert.deepEqual(await h.snap(),unknown);assert.equal(await h.claim(),null);
});

test('malformed or oversized normalized events and unknown fields are rejected before persistence',async t=>{
  const h=await setup(t);await h.submit('validate runner output');const dispatch=await h.claim();const wire=wireEvents(dispatch);
  for(const event of [null,[],{...wire.started,sequence:0},{...wire.started,sequence:1.5},{...wire.started,turnId:''},{...wire.started,turnId:'x'.repeat(129)},{...wire.started,ownerToken:'synthetic-forgery'},{...wire.started,kind:'ownerApproved'}]) {
    const before=await h.snap();assert.equal((await h.ingest(dispatch,event)).status,400);assert.deepEqual(await h.snap(),before);
  }
  assert.equal((await h.ingest(dispatch,wire.started)).status,200);
  for(const event of [
    {...wire.terminal,status:'invented'}, {...wire.terminal,status:['interrupted'],messages:[]}, {...wire.terminal,status:{toString:'completed'},messages:[]}, {...wire.terminal,messages:[]}, {...wire.terminal,effectiveModel:'self-claimed'},
    {...wire.terminal,messages:[{itemId:'one',text:'x'.repeat(65_537),phase:'unreported'}]},
    {...wire.terminal,messages:[{itemId:'one',text:'\ud800',phase:'unreported'}]},
    {...wire.terminal,messages:[{itemId:'one',text:'body',phase:'unsupported'}]},
    {...wire.terminal,messages:[{itemId:'one',text:'private commentary',phase:['commentary']}]},
    {...wire.terminal,messages:[{itemId:'one',text:'body',phase:'finalAnswer'},{itemId:'one',text:'different',phase:'finalAnswer'}]},
  ]){const before=await h.snap();assert.equal((await h.ingest(dispatch,event)).status,400);assert.deepEqual(await h.snap(),before);}
});

test('deltas remain provisional, exact final messages commit together and phases stay private',async t=>{
  const h=await setup(t);await h.submit('phases');const dispatch=await h.claim();const wire=wireEvents(dispatch);
  await h.ingest(dispatch,wire.started);
  const delta={...wire.started,sequence:2,kind:'textDelta',itemId:'reply-one',text:'provisional'};
  assert.equal((await h.ingest(dispatch,delta)).status,200);assert.equal((await h.snap()).messages.length,1);
  const terminal={...wire.terminal,sequence:3,messages:[{itemId:'comment',text:'Working',phase:'commentary'},{itemId:'one',text:'first\r\n ',phase:'finalAnswer'},{itemId:'two',text:' e\u0301 🦊',phase:'unreported'}]};
  assert.equal((await h.ingest(dispatch,terminal)).status,200);
  const snap=await h.snap();assert.deepEqual(snap.messages.slice(1).map((message:any)=>message.text),['first\r\n ',' e\u0301 🦊']);
  assert.equal(snap.runs[0].state,'completed');
  assert.equal((await h.ingest(dispatch,{...terminal,sequence:4})).status,409);
});

test('cancellation and completion races have one durable outcome; no ACK or exit can confirm cancellation',async t=>{
  for(const terminalStatus of ['completed','interrupted'])await t.test(terminalStatus,async t=>{
    const h=await setup(t);await h.submit('race');const dispatch=await h.claim();const wire=wireEvents(dispatch);await h.ingest(dispatch,wire.started);
    const cancel=()=>h.owner(`/v1/runs/${dispatch.attempt.runId}/cancel`,{recoveryEpoch:h.scope.recoveryEpoch});
    const terminal={...wire.terminal,status:terminalStatus,messages:terminalStatus==='completed'?wire.terminal.kind==='terminal'?wire.terminal.messages:[]:[]};
    const results=await Promise.all([h.ingest(dispatch,terminal),cancel()]);assert(results.every(result=>result.status===200));
    const snapshot=await h.snap();assert.equal(snapshot.runs[0].state,terminalStatus==='completed'?'completed':'canceled');
    assert.equal(snapshot.messages.length,terminalStatus==='completed'?2:1);
    const contradictory={...terminal,status:terminalStatus==='completed'?'interrupted':'completed',messages:terminalStatus==='completed'?[]:wire.terminal.kind==='terminal'?wire.terminal.messages:[]};
    assert.equal((await h.ingest(dispatch,contradictory)).status,409);assert.deepEqual(await h.snap(),snapshot);
  });
});

test('runner expiry or revocation during a partial upload cannot commit evidence',async t=>{
  const {request}=await import('node:http');
  for(const action of ['expiry','revocation'])await t.test(action,async t=>{
    let now=1000;const h=await setup(t,{now:()=>now});await h.submit('delayed upload');const dispatch=await h.claim();const wire=wireEvents(dispatch);const before=await h.snap();
    const payload=JSON.stringify({...h.scope,dispatchToken:dispatch.dispatchToken,event:wire.started});
    let finish!:()=>void;
    const result=new Promise<number>((resolve,reject)=>{
      const req=request(h.service.baseURL+'/fixture/runner/events',{method:'POST',headers:{...h.runnerHeaders,'Content-Type':'application/json','Content-Length':Buffer.byteLength(payload)}},res=>{res.resume();res.on('end',()=>resolve(res.statusCode!));});
      req.on('error',reject);req.write(payload.slice(0,-1));finish=()=>req.end(payload.slice(-1));
    });
    if(action==='expiry')now+=300_001;else h.service.revokeSyntheticRunner(h.auth.sessionId);
    finish();assert.equal(await result,401);const after=await h.snap();assert.deepEqual(after.messages,before.messages);if(action==='expiry')assert.equal(after.runs[0].state,'outcomeUnknown');else assert.deepEqual(after,before);
  });
});

test('synthetic selection is persisted and cannot activate the ordinary fixture process path',async t=>{
  const h=await setup(t);await h.submit('held');assert.equal(await h.service.dispatchNextFixture(),false);
  await h.service.close();assert.throws(()=>createFixtureService({dataDirectory:h.directory}),/execution mode/);
  await h.restart();assert.equal((await h.snap()).runs[0].state,'queued');
});

test('a lost runner and expired dispatch become unknown on observation, fence late output and retain the barrier',async t=>{
  let now=1000;const h=await setup(t,{now:()=>now});await h.submit('first');await h.submit('successor');const dispatch=await h.claim();const wire=wireEvents(dispatch);
  await h.ingest(dispatch,wire.started);
  const command={...h.scope,attemptId:dispatch.attempt.attemptId,dispatchToken:dispatch.dispatchToken};
  assert.equal((await h.runner('status',command)).body.state,'running');
  now+=40_001;assert.equal((await h.snap()).runs[0].state,'outcomeUnknown');
  assert.equal((await h.ingest(dispatch,wire.terminal)).status,409);assert.equal(await h.claim(),null);
  await h.restart();assert.equal((await h.snap()).runs[0].state,'outcomeUnknown');assert.equal(await h.claim(),null);
});

test('cancel before turn dispatch requires an owner cancellation and never-started evidence',async t=>{
  const h=await setup(t);await h.submit('cancel before handshake');const dispatch=await h.claim();
  const event={protocolVersion:1,provider:'codex',...dispatch.attempt,sequence:1,kind:'notStarted',reason:'canceledBeforeDispatch'};
  assert.equal((await h.ingest(dispatch,event)).status,409);
  const command={...h.scope,attemptId:dispatch.attempt.attemptId,dispatchToken:dispatch.dispatchToken};
  const cancelled=await h.owner(`/v1/runs/${dispatch.attempt.runId}/cancel`,{recoveryEpoch:h.scope.recoveryEpoch});assert.equal(cancelled.body.run.state,'cancelRequested');
  assert.equal((await h.runner('status',command)).body.state,'cancelRequested');
  assert.equal((await h.ingest(dispatch,event)).status,200);assert.equal((await h.snap()).runs[0].state,'canceled');
  assert.equal((await h.snap()).messages.length,1);
});

test('valid wrong-turn progress, cumulative deltas and failed terminal output cannot become successful messages',async t=>{
  const h=await setup(t);await h.submit('bounded');const dispatch=await h.claim();const wire=wireEvents(dispatch);await h.ingest(dispatch,wire.started);
  const delta={...wire.started,sequence:2,kind:'textDelta',itemId:'one',text:'x'.repeat(65_536)};
  const before=await h.snap();assert.equal((await h.ingest(dispatch,{...delta,turnId:'wrong-turn'})).status,409);assert.deepEqual(await h.snap(),before);
  assert.equal((await h.ingest(dispatch,delta)).status,200);
  assert.equal((await h.ingest(dispatch,{...delta,sequence:3,text:'x'})).status,413);
  const failed={...wire.terminal,sequence:3,status:'failed',messages:[]};assert.equal((await h.ingest(dispatch,failed)).status,200);
  const snap=await h.snap();assert.equal(snap.runs[0].state,'outcomeUnknown');assert.equal(snap.messages.length,1);
});

async function crashChild(t:TestContext,mode:string,directory:string,point:string) {
  const {fork}=await import('node:child_process');const {fileURLToPath}=await import('node:url');
  const child=fork(fileURLToPath(new URL('./synthetic-crash-host.ts',import.meta.url)),[mode,directory,point],{execPath:process.execPath,env:{LANG:'en_US.UTF-8',TZ:'UTC'},stdio:['ignore','ignore','pipe','ipc']});
  child.stderr?.resume();t.after(()=>{if(child.exitCode===null&&child.signalCode===null)child.kill('SIGKILL');});
  const exited=new Promise<{code:number|null;signal:NodeJS.Signals|null}>(resolve=>child.once('exit',(code,signal)=>resolve({code,signal})));
  const ready=await new Promise<any>((resolve,reject)=>{child.once('error',reject);child.once('message',resolve);child.once('exit',()=>reject(new Error('Synthetic crash fixture exited before ready.')));});
  return {child,exited,ready};
}

test('actual service SIGKILL around dispatch, receipt and terminal commit leaves an atomic replayable ledger', {timeout:15_000},async t=>{
  for(const point of ['syntheticBeforeClaimCommit','syntheticAfterClaimCommit','syntheticAfterReceipt','syntheticAfterEvidenceCommit','syntheticAfterMessage','syntheticBeforeTerminalCommit','syntheticAfterTerminalCommit'])await t.test(point,async t=>{
    const h=await setup(t);await h.submit('first crash turn');await h.submit('later crash turn');await h.service.close();
    const child=await crashChild(t,'service',h.directory,point);h.connect(child.ready.baseURL);
    let dispatch:any;let terminal:any;
    const action=async()=>{
      dispatch=await h.claim();const wire=wireEvents(dispatch);terminal=wire.terminal;
      await h.ingest(dispatch,wire.started);
      await h.ingest(dispatch,wire.terminal);
    };
    await assert.rejects(action(),/fetch failed|other side closed/);
    assert.equal((await child.exited).signal,'SIGKILL');
    await h.restart();const snap=await h.snap();
    const completed=point==='syntheticAfterTerminalCommit';const neverClaimed=point==='syntheticBeforeClaimCommit';
    assert.equal(snap.runs[0].state,completed?'completed':neverClaimed?'queued':'outcomeUnknown');
    assert.equal(snap.messages.length,completed?3:2);assert.equal(snap.runs[1].state,'queued');
    if(completed){assert.equal((await h.ingest(dispatch,terminal)).body.duplicate,true);assert.deepEqual(await h.snap(),snap);}
    else if(dispatch&&terminal)assert.equal((await h.ingest(dispatch,terminal)).status,409);
    const next=await h.claim();assert.equal(next===null,!completed&&!neverClaimed);
  });
});

test('actual synthetic runner kill or exit-zero cannot become success; committed terminal evidence survives', {timeout:15_000},async t=>{
  for(const point of ['runnerBeforeStart','runnerAfterStarted','runnerBeforeTerminal','runnerAfterTerminal','runnerExitZero'])await t.test(point,async t=>{
    let now=1000;const h=await setup(t,{now:()=>now});await h.submit('runner crash');await h.submit('later work');const dispatch=await h.claim();
    const child=await crashChild(t,'runner',h.directory,point);
    child.child.send({baseURL:h.service.baseURL,scope:h.scope,dispatch,headers:h.runnerHeaders});
    const ended=await child.exited;if(point==='runnerExitZero')assert.equal(ended.code,0);else assert.equal(ended.signal,'SIGKILL');
    now+=40_001;const snap=await h.snap();const completed=point==='runnerAfterTerminal';
    assert.equal(snap.runs[0].state,completed?'completed':'outcomeUnknown');assert.equal(snap.messages.length,completed?3:2);
    await h.restart();assert.deepEqual(await h.snap(),snap);assert.equal((await h.claim())===null,!completed);
  });
});

test('synthetic runtime input compatibility is checked before durable acceptance',async t=>{
  const h=await setup(t);const result=await h.submit('','empty-synthetic-input');assert.equal(result.status,400);
  assert.equal((await h.owner('/v1/requests/empty-synthetic-input')).status,404);assert.equal((await h.snap()).messages.length,0);
});

test('synthetic transport actions cross the authenticated bridge with ordered writes and durable deadline uncertainty',async t=>{
  const {SyntheticTurn}=await import('../runner/synthetic-transport.ts');
  for(const failure of ['handshake','turn','stop','unsupported'])await t.test(failure,async t=>{
    let now=1000;const h=await setup(t,{now:()=>now});await h.submit('transport lifecycle');await h.submit('blocked successor');
    const dispatch=await h.claim();const driver=new SyntheticTurn(dispatch,()=>now);const actionsSeen:string[]=[];const writes:string[]=[];
    const apply=async(actions:SyntheticAction[])=>{
      for(const action of actions){actionsSeen.push(action.kind);if(action.kind==='write')writes.push(action.data);if(action.kind==='evidence')assert.equal((await h.ingest(dispatch,action.event)).status,200);}
    };
    await apply(driver.begin());
    if(failure!=='handshake'){
      await apply(driver.ready(dispatch.threadId));
      await apply(driver.receive(frame({id:dispatch.requestId,result:{turn:{id:'turn-one',status:'inProgress'}}})));
    }
    if(failure==='stop') {
      await h.owner(`/v1/runs/${dispatch.attempt.runId}/cancel`,{recoveryEpoch:h.scope.recoveryEpoch});await apply(driver.cancel());
      await apply(driver.receive(frame({id:dispatch.requestId+':interrupt',result:{}})));
      assert.equal((await h.snap()).runs[0].state,'cancelRequested');now+=2000;
    } else if(failure==='unsupported')await apply(driver.receive(frame({id:81,method:'item/permissions/requestApproval',params:{}})));
    else now+=failure==='handshake'?5000:30_000;
    await apply(driver.tick());const unknown=await h.snap();assert.equal(unknown.runs[0].state,'outcomeUnknown');assert.equal(unknown.messages.length,2);assert.equal(await h.claim(),null);
    const closeIndex=actionsSeen.indexOf('close');assert(closeIndex>=0);assert.equal(actionsSeen.slice(closeIndex).includes('write'),false);
    if(failure==='unsupported'){const methods=writes.map(line=>JSON.parse(line));assert.equal(methods.at(-2).error.code,-32601);assert.equal(methods.at(-1).method,'turn/interrupt');}
    now+=2000;await apply(driver.tick());assert.equal(actionsSeen.at(-1),'forceClose');assert.deepEqual(driver.begin(),[]);
    await h.restart();assert.deepEqual(await h.snap(),unknown);assert.equal(await h.claim(),null);
  });
});

test('closing an unknown attempt releases only later work and permanently fences its late terminal',async t=>{
  const h=await setup(t);await h.submit('uncertain original');await h.submit('eligible later turn');const first=await h.claim();const old=wireEvents(first);
  await h.ingest(first,old.started);
  await h.ingest(first,{protocolVersion:1,provider:'codex',...first.attempt,sequence:2,kind:'outcomeUnknown',reason:'connectionLost'});
  assert.equal(await h.claim(),null);
  assert.equal((await h.owner(`/v1/runs/${first.attempt.runId}/close-unknown`,{recoveryEpoch:h.scope.recoveryEpoch,acknowledgeUncertainty:true})).status,200);
  const later=await h.claim();assert.notEqual(later.attempt.runId,first.attempt.runId);const current=wireEvents(later,'later result');
  assert.equal((await h.ingest(first,old.terminal)).status,409);await h.ingest(later,current.started);await h.ingest(later,current.terminal);
  const snap=await h.snap();assert.deepEqual(snap.runs.map((run:any)=>run.state),['closedUnknown','completed']);assert.deepEqual(snap.messages.filter((message:any)=>message.role==='assistant').map((message:any)=>message.text),['later result']);
});

test('multi-message terminal rollback and lost acknowledgement preserve one complete result',async t=>{
  for(const point of ['syntheticAfterMessage','syntheticAfterTerminalCommit'])await t.test(point,async t=>{
    let hit=0;let armed=false;const h=await setup(t,{checkpoint:(actual:string)=>{if(armed&&actual===point&&++hit===(point==='syntheticAfterMessage'?2:1))throw new Error('synthetic atomicity fault');}});
    await h.submit('atomic terminal');const dispatch=await h.claim();const wire=wireEvents(dispatch);await h.ingest(dispatch,wire.started);
    const terminal={...wire.terminal,messages:[{itemId:'one',text:'first',phase:'finalAnswer'},{itemId:'two',text:'second',phase:'finalAnswer'}]};
    const before=await h.snap();armed=true;assert.equal((await h.ingest(dispatch,terminal)).status,500);armed=false;
    const after=await h.snap();const committed=point==='syntheticAfterTerminalCommit';
    if(!committed)assert.deepEqual(after,before);else {assert.equal(after.messages.length,3);assert.equal(after.runs[0].state,'completed');}
    const retry=await h.ingest(dispatch,terminal);assert.equal(retry.status,200);assert.equal(retry.body.duplicate,committed);
    const final=await h.snap();assert.deepEqual(final.messages.slice(1).map((message:any)=>message.text),['first','second']);
    await h.restart();assert.deepEqual(await h.snap(),final);assert.equal((await h.ingest(dispatch,terminal)).body.duplicate,true);
  });
});

test('a synthetic recovery-epoch change invalidates runner sessions even if current scope IDs are copied',async t=>{
  const {DatabaseSync}=await import('node:sqlite');
  const h=await setup(t);await h.submit('epoch-fenced turn');const dispatch=await h.claim();const wire=wireEvents(dispatch);await h.service.close();
  const db=new DatabaseSync(join(h.directory,'fixture.sqlite'));const install=JSON.parse(String(db.prepare("SELECT value FROM metadata WHERE key='installation'").get()!.value));
  install.recoveryEpoch=crypto.randomUUID();db.prepare("UPDATE metadata SET value=? WHERE key='installation'").run(JSON.stringify(install));db.close();await h.restart();
  const command={...h.scope,recoveryEpoch:install.recoveryEpoch,dispatchToken:dispatch.dispatchToken,event:wire.started};
  const before=await h.snap();assert.equal((await h.runner('events',command)).status,401);assert.equal((await h.runner('claim',{...h.scope,recoveryEpoch:install.recoveryEpoch})).status,401);assert.deepEqual(await h.snap(),before);
});

test('concurrent contradictory terminals have exactly one committed winner',async t=>{
  const h=await setup(t);await h.submit('terminal race');const dispatch=await h.claim();const wire=wireEvents(dispatch);await h.ingest(dispatch,wire.started);
  const interrupted={...wire.terminal,status:'interrupted',messages:[]};
  const results=await Promise.all([h.ingest(dispatch,wire.terminal),h.ingest(dispatch,interrupted)]);
  assert.deepEqual(results.map(result=>result.status).sort(),[200,409]);
  const completed=results[0].status===200;const snap=await h.snap();assert.equal(snap.runs[0].state,completed?'completed':'canceled');assert.equal(snap.messages.length,completed?2:1);
  await h.restart();assert.deepEqual(await h.snap(),snap);
  assert.equal((await h.ingest(dispatch,completed?wire.terminal:interrupted)).body.duplicate,true);
  assert.equal((await h.ingest(dispatch,completed?interrupted:wire.terminal)).status,409);
});

// Real synthetic pipes feed the same authenticated private HTTP boundary.
test('supervised fragmented process output survives a lost committed acknowledgement without a second start',async t=>{
  const {startSyntheticProcess}=await import('../runner/synthetic-process.ts');
  const h=await setup(t);const text='  /supervised\r\ne\u0301 🦊  ';await h.submit(text);const dispatch=await h.claim();
  const directory=await mkdtemp('/tmp/fort-bridge-process-');t.after(()=>rm(directory,{recursive:true,force:true}));
  let lost=false;let duplicate=false;
  const run=startSyntheticProcess(dispatch,{directory,scenario:'fragmented',maxWriteChunkBytes:7,deliver:async(event,signal)=>{
    const response=await fetch(h.service.baseURL+'/fixture/runner/events',{method:'POST',headers:{'Content-Type':'application/json',...h.runnerHeaders},body:JSON.stringify({...h.scope,dispatchToken:dispatch.dispatchToken,event}),signal});
    assert.equal(response.status,200);const receipt=await response.json() as any;
    if(event.kind==='terminal'&&!lost){lost=true;throw new Error('synthetic acknowledgement loss after commit');}
    duplicate ||= receipt.duplicate;return receipt;
  }});
  const result=await run.result;assert.equal(result.cleanup,'observed');assert.equal(result.terminal,'completed');assert.equal(result.startWritesAttempted,1);assert.equal(duplicate,true);
  const before=await h.snap();assert.equal(before.messages[1].text,text);assert.equal(before.runs[0].state,'completed');assert.equal(before.messages.length,2);await h.restart();assert.deepEqual(await h.snap(),before);assert.equal(await h.claim(),null);
});

test('blocked process input cannot delay committed owner cancellation or release uncertain queued work',async t=>{
  const {startSyntheticProcess}=await import('../runner/synthetic-process.ts');const h=await setup(t);
  await h.submit('\0'.repeat(65_536));const dispatch=await h.claim();await h.submit('queued behind the blocked process');
  const directory=await mkdtemp('/tmp/fort-bridge-process-');t.after(()=>rm(directory,{recursive:true,force:true}));
  const run=startSyntheticProcess(dispatch,{directory,scenario:'blocked-input',limits:{interruptMs:100,teardownMs:100},deliver:async event=>{
    const response=await h.ingest(dispatch,event);assert.equal(response.status,200);return response.body;
  }});
  await new Promise(resolve=>setTimeout(resolve,100));
  const canceled=await h.owner(`/v1/runs/${dispatch.attempt.runId}/cancel`,{recoveryEpoch:h.scope.recoveryEpoch});assert.equal(canceled.status,200);run.cancel();
  const result=await run.result;assert.equal(result.cleanup,'observed');assert.equal(result.terminal,'unknown');assert.equal(result.startWritesAttempted,1);
  const before=await h.snap();assert.equal(before.runs[0].state,'outcomeUnknown');assert.equal(before.messages.length,2);assert.equal(await h.claim(),null);await h.restart();assert.deepEqual(await h.snap(),before);assert.equal(await h.claim(),null);
});

test('service restart fences a still-running supervised process instead of accepting late output or repeating work',async t=>{
  const {startSyntheticProcess}=await import('../runner/synthetic-process.ts');const h=await setup(t);await h.submit('restart while process is running');const dispatch=await h.claim();
  const directory=await mkdtemp('/tmp/fort-bridge-process-');t.after(()=>rm(directory,{recursive:true,force:true}));
  let restarted=false;
  const run=startSyntheticProcess(dispatch,{directory,scenario:'echo',limits:{teardownMs:100,deliveryMs:250},deliver:async event=>{
    const response=await h.ingest(dispatch,event);
    if(event.kind==='started'){assert.equal(response.status,200);await h.restart();restarted=true;return response.body;}
    assert.equal(response.status,409);throw new Error('old attempt fenced');
  }});
  const result=await run.result;assert.equal(restarted,true);assert.equal(result.cleanup,'observed');assert.equal(result.evidence,'unconfirmed');assert.equal(result.terminal,'unknown');assert.equal(result.startWritesAttempted,1);
  const snapshot=await h.snap();assert.equal(snapshot.runs[0].state,'outcomeUnknown');assert.equal(snapshot.messages.length,1);assert.equal(await h.claim(),null);
});

test('slow committed start acknowledgement cannot discard supervised terminal output or block later work',async t=>{
  const {startSyntheticProcess}=await import('../runner/synthetic-process.ts');const h=await setup(t);const text='  /slow upload\r\ne\u0301 🦊  ';
  await h.submit(text);const dispatch=await h.claim();await h.submit('later accepted work');
  const directory=await mkdtemp('/tmp/fort-bridge-process-');t.after(()=>rm(directory,{recursive:true,force:true}));
  const sequences:number[]=[];let lost=false;let duplicate=false;
  const run=startSyntheticProcess(dispatch,{directory,scenario:'progress-terminal-descendant',limits:{teardownMs:40,deliveryMs:250},deliver:async(event,signal)=>{
    const response=await fetch(h.service.baseURL+'/fixture/runner/events',{method:'POST',headers:{'Content-Type':'application/json',...h.runnerHeaders},body:JSON.stringify({...h.scope,dispatchToken:dispatch.dispatchToken,event}),signal});
    assert.equal(response.status,200);const receipt=await response.json() as any;sequences.push(event.sequence);duplicate ||= receipt.duplicate;
    await new Promise(resolve=>setTimeout(resolve,100));
    if(event.kind==='started'&&!lost){lost=true;throw new Error('Synthetic acknowledgement lost after commit');}
    return receipt;
  }});
  const report=await run.result;assert.equal(report.terminal,'completed');assert.equal(report.cleanup,'observed');assert.equal(report.evidence,'confirmed');assert.equal(report.failure,null);assert.equal(report.startWritesAttempted,1);assert.equal(duplicate,true);assert.deepEqual(sequences,[1,1,2,3,4]);
  const snapshot=await h.snap();assert.equal(snapshot.messages.filter((message:any)=>message.text===text).length,2);assert.equal(snapshot.runs[0].state,'completed');assert.equal(snapshot.runs[1].state,'queued');assert.equal(snapshot.messages.length,3);
  await h.restart();assert.deepEqual(await h.snap(),snapshot);const later=await h.claim();assert.equal(later.text,'later accepted work');assert.notEqual(later.attempt.runId,dispatch.attempt.runId);
});
