import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { SyntheticTurn } from './synthetic-transport.ts';
import type { SyntheticDispatch } from '../contracts/runner.ts';
const dispatch:SyntheticDispatch={protocolVersion:1,mode:'synthetic',protocol:'codex-turn',installationId:'installation',recoveryEpoch:'epoch',hostId:'fixture-local',sessionId:'session',dispatchToken:'synthetic-token',inputDigest:'synthetic-digest',threadId:'thread-one',requestId:'start-one',workspace:'/synthetic/workspace',text:'  /exact\r\ne\u0301 🦊  ',binding:{bindingRevisionId:'binding',provider:'fixture',runtimeVersion:'1',selectedModel:'fixture-echo-v1',hostId:'fixture-local',enabled:true},attempt:{runId:'run',attemptId:'attempt',generation:1,bindingRevisionId:'binding',runtimeVersion:'1',selectedModel:'fixture-echo-v1'}};
dispatch.inputDigest=createHash('sha256').update(dispatch.text,'utf8').digest('hex');
const frame=(data:unknown)=>Buffer.from(JSON.stringify(data)+'\n');
const started=()=>frame({id:'start-one',result:{turn:{id:'turn-one',status:'inProgress'}}});
const interrupted=()=>frame({method:'turn/completed',params:{threadId:'thread-one',turn:{id:'turn-one',status:'interrupted',error:null}}});

test('synthetic handshake must bind the exact thread before one immutable turn start',()=>{
  let time=0;const input=structuredClone(dispatch);const turn=new SyntheticTurn(input,()=>time);
  input.text='changed';assert.deepEqual(turn.begin(),[{kind:'handshake',threadId:'thread-one'}]);assert.deepEqual(turn.begin(),[]);
  const ready=turn.ready('thread-one');assert.equal(ready.length,1);assert.equal(ready[0].kind,'write');
  if(ready[0].kind==='write')assert.equal(JSON.parse(ready[0].data).params.input[0].text,dispatch.text);
  assert.deepEqual(turn.ready('thread-one'),[]);time=29_999;assert.deepEqual(turn.tick(),[]);
});

test('handshake and turn deadlines produce unknown evidence and never repeat a start',()=>{
  for(const phase of ['handshake','turn']) {
    let time=0;const turn=new SyntheticTurn(dispatch,()=>time);turn.begin();
    if(phase==='turn'){turn.ready('thread-one');turn.receive(started());}
    time=phase==='handshake'?5000:30_000;const actions=turn.tick();
    assert(actions.some(action=>action.kind==='evidence'&&action.event.kind==='outcomeUnknown'));
    assert.equal(actions.at(-1)?.kind,'close');
    if(phase==='turn'){assert.equal(actions[0].kind,'write');if(actions[0].kind==='write')assert.equal(JSON.parse(actions[0].data).method,'turn/interrupt');}
    assert.deepEqual(turn.ready('thread-one'),[]);assert.deepEqual(turn.receive(interrupted()),[]);assert.deepEqual(turn.begin(),[]);
    time+=2000;assert.deepEqual(turn.tick(),[{kind:'forceClose'}]);assert.deepEqual(turn.tick(),[]);
  }
});

test('cancellation ACK and connection closure are not terminal evidence; the stop deadline remains bounded',()=>{
  let time=0;const turn=new SyntheticTurn(dispatch,()=>time);turn.begin();turn.ready('thread-one');turn.receive(started());
  const cancel=turn.cancel();assert.equal(cancel[0].kind,'write');assert.deepEqual(turn.cancel(),[]);
  assert.deepEqual(turn.receive(frame({id:'start-one:interrupt',result:{}})),[]);
  time=2000;const stopped=turn.tick();assert(stopped.some(action=>action.kind==='evidence'&&action.event.kind==='outcomeUnknown'));
  assert(!stopped.some(action=>action.kind==='evidence'&&action.event.kind==='terminal'));assert.equal(stopped.at(-1)?.kind,'forceClose');
});

test('correlated interrupted terminal wins before timeout and teardown cannot invent another result',()=>{
  let time=0;const turn=new SyntheticTurn(dispatch,()=>time);turn.begin();turn.ready('thread-one');turn.receive(started());turn.cancel();
  const actions=turn.receive(interrupted());assert(actions.some(action=>action.kind==='evidence'&&action.event.kind==='terminal'&&action.event.status==='interrupted'));
  assert.equal(actions.at(-1)?.kind,'close');time=2000;assert.deepEqual(turn.tick(),[{kind:'forceClose'}]);assert.deepEqual(turn.disconnected(),[]);
});

test('unsupported requests are rejected and interrupted before the ordered close action',()=>{
  let time=0;const turn=new SyntheticTurn(dispatch,()=>time);turn.begin();turn.ready('thread-one');turn.receive(started());
  const actions=turn.receive(frame({id:3,method:'item/permissions/requestApproval',params:{}}));
  assert.deepEqual(actions.map(action=>action.kind),['write','write','evidence','close']);
  assert.equal(actions[0].kind==='write'&&JSON.parse(actions[0].data).error.code,-32601);
  assert.equal(actions[1].kind==='write'&&JSON.parse(actions[1].data).method,'turn/interrupt');
  time=2000;assert.deepEqual(turn.tick(),[{kind:'forceClose'}]);
});

test('a wrong handshake or transport loss remains unknown, while pre-dispatch cancellation is explicit',()=>{
  const wrong=new SyntheticTurn(dispatch,()=>0);wrong.begin();assert(wrong.ready('wrong-thread').some(action=>action.kind==='evidence'&&action.event.kind==='outcomeUnknown'));
  const lost=new SyntheticTurn(dispatch,()=>0);lost.begin();lost.ready('thread-one');lost.receive(started());
  assert(lost.disconnected().some(action=>action.kind==='evidence'&&action.event.kind==='outcomeUnknown'));
  const canceled=new SyntheticTurn(dispatch,()=>0);canceled.begin();
  assert(canceled.cancel().some(action=>action.kind==='evidence'&&action.event.kind==='notStarted'));
  assert.deepEqual(canceled.ready('thread-one'),[]);
});

test('synthetic transport refuses changed runtime/model bindings and leaks no dispatch credential into actions',()=>{
  for(const changes of [{text:'altered accepted input'},{mode:'live'},{binding:{...dispatch.binding,enabled:false}},{attempt:{...dispatch.attempt,selectedModel:'substituted'}},{attempt:{...dispatch.attempt,runtimeVersion:'new-version'}},{hostId:'foreign'},{workspace:'/host/home'}])assert.throws(()=>new SyntheticTurn({...dispatch,...changes} as SyntheticDispatch,()=>0),/Invalid synthetic dispatch/);
  const turn=new SyntheticTurn(dispatch,()=>0);assert(!JSON.stringify([...turn.begin(),...turn.ready('thread-one')]).includes(dispatch.dispatchToken));
});

test('a late cancellation cannot extend an already expired handshake or turn deadline',()=>{
  for(const phase of ['handshake','turn']) {
    let time=0;const turn=new SyntheticTurn(dispatch,()=>time);turn.begin();
    if(phase==='turn'){turn.ready('thread-one');turn.receive(started());}
    time=phase==='handshake'?5000:30_000;
    const actions=turn.cancel();assert(actions.some(action=>action.kind==='evidence'&&action.event.kind==='outcomeUnknown'));
    assert(!actions.some(action=>action.kind==='evidence'&&action.event.kind==='notStarted'));
  }
});

test('fixture I/O tests may shorten deadlines but cannot widen the accepted lifecycle policy',()=>{
  let time=0;const turn=new SyntheticTurn(dispatch,()=>time,{handshakeMs:10,turnMs:20,interruptMs:10,teardownMs:10});
  turn.begin();time=10;assert(turn.tick().some(action=>action.kind==='evidence'&&action.event.kind==='outcomeUnknown'));
  for(const limits of [{handshakeMs:5001},{turnMs:30001},{interruptMs:2001},{teardownMs:2001},{handshakeMs:0},{turnMs:1.5}])assert.throws(()=>new SyntheticTurn(dispatch,()=>0,limits),/Invalid synthetic deadlines/);
});
