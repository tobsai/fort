import { test } from 'node:test';
import assert from 'node:assert/strict';
import { CodexTurn, type CodexTurnInput } from './codex.ts';

const attempt = { runId:'run-one', attemptId:'attempt-one', generation:1, bindingRevisionId:'binding-one', runtimeVersion:'synthetic-runtime-version', selectedModel:'synthetic-selected-model' } as const;
const text = '  /ordinary text\r\nCafe\u0301 🦊  ';
const frame = (value: unknown) => Buffer.from(JSON.stringify(value)+'\n');
const notification = (method:string, details:Record<string,unknown>, threadId='thread-one', turnId='turn-one') => frame({method,params:{threadId,turnId,...details}});
const started = () => frame({id:'start-one',result:{turn:{id:'turn-one',status:'inProgress',items:[],error:null}}});
const completed = (status='completed') => frame({method:'turn/completed',params:{threadId:'thread-one',turn:{id:'turn-one',status,items:[],error:null}}});

test('Codex wire adapter preserves exact input and emits only correlated terminal reply evidence', () => {
  const turn = new CodexTurn({attempt,threadId:'thread-one',requestId:'start-one',workspace:'/fixture/workspace',text});
  const start = turn.start();
  const request = JSON.parse(start.writes[0]);
  assert.equal(request.method,'turn/start');
  assert.deepEqual(request.params.input,[{type:'text',text}]);
  assert.equal(request.params.model,attempt.selectedModel);
  assert.deepEqual(start.events,[]);
  assert.equal(turn.receive(started()).events[0].kind,'started');
  assert.equal(turn.receive(notification('item/agentMessage/delta',{itemId:'reply-one',delta:'provisional'})).events[0].kind,'textDelta');
  assert.deepEqual(turn.receive(notification('item/completed',{item:{type:'agentMessage',id:'reply-one',text:'Final exact reply 🦊'}})).events,[]);
  const result = turn.receive(completed());
  assert.equal(result.events.length,1);
  assert.deepEqual(result.events[0],{protocolVersion:1,provider:'codex',...attempt,sequence:3,kind:'terminal',threadId:'thread-one',turnId:'turn-one',status:'completed',messages:[{itemId:'reply-one',text:'Final exact reply 🦊',phase:'unreported'}],effectiveModel:null,modelEvidence:'unreported'});
  assert.equal(result.closeTransport,true);
  assert.deepEqual(turn.receive(completed()).events,[]);
  assert.deepEqual(turn.start().writes,[]);
});

const make = () => new CodexTurn({attempt,threadId:'thread-one',requestId:'start-one',workspace:'/fixture/workspace',text});
const active = () => { const turn=make(); turn.start(); turn.receive(started()); return turn; };
const finalItem = (text='final') => notification('item/completed',{item:{type:'agentMessage',id:'reply-one',text}});
const reason = (output:ReturnType<CodexTurn['receive']>) => output.events.find(event=>event.kind==='outcomeUnknown')?.reason;

test('fragmented UTF-8 and coalesced frames produce authoritative final text, not provisional deltas', () => {
  const turn=active(); const bytes=Buffer.concat([notification('item/agentMessage/delta',{itemId:'reply-one',delta:'🦊'}),finalItem('  final\r\n🦊  '),completed()]);
  const events=[];
  for (const byte of bytes) events.push(...turn.receive(Uint8Array.of(byte)).events);
  assert.equal(events.filter(event=>event.kind==='textDelta').length,1);
  const terminal=events.find(event=>event.kind==='terminal'); assert(terminal);
  assert.deepEqual(terminal.messages,[{itemId:'reply-one',text:'  final\r\n🦊  ',phase:'unreported'}]);
});

test('notifications arriving before the start response wait for its exact turn correlation', () => {
  const turn=make();turn.start();
  assert.deepEqual(turn.receive(finalItem()).events,[]);
  assert.deepEqual(turn.receive(completed()).events,[]);
  const result=turn.receive(started());
  assert.deepEqual(result.events.map(event=>event.kind),['started','terminal']);
});

test('foreign threads and stale turns cannot contaminate the accepted turn', () => {
  const turn=active();
  for (const [threadId,turnId] of [['foreign','turn-one'],['thread-one','stale-turn']]) {
    assert.deepEqual(turn.receive(notification('item/completed',{item:{type:'agentMessage',id:'bad',text:'foreign'}},threadId,turnId)).events,[]);
    assert.deepEqual(turn.receive(frame({method:'turn/completed',params:{threadId,turn:{id:turnId,status:'completed'}}})).events,[]);
  }
  turn.receive(finalItem());const result=turn.receive(completed());
  assert.deepEqual(result.events[0].kind,'terminal');
  assert(!JSON.stringify(result).includes('foreign'));
});

test('an interrupt response is not cancellation proof; a correlated interrupted terminal is', () => {
  const turn=active();const request=JSON.parse(turn.interrupt().writes[0]);
  assert.equal(request.method,'turn/interrupt'); assert.deepEqual(request.params,{threadId:'thread-one',turnId:'turn-one'});
  assert.deepEqual(turn.interrupt().writes,[]);
  assert.deepEqual(turn.receive(frame({id:request.id,result:{}})).events,[]);
  const result=turn.receive(completed('interrupted'));assert.equal(result.events[0].kind,'terminal');
  assert.equal(result.events[0].kind==='terminal' && result.events[0].status,'interrupted');
});

test('interruption before the start response waits for the exact accepted turn ID', () => {
  const turn=make();turn.start();assert.deepEqual(turn.interrupt().writes,[]);
  const result=turn.receive(started());assert.equal(JSON.parse(result.writes[0]).params.turnId,'turn-one');
});

test('connection loss or exit without terminal evidence remains unknown and never resubmits', () => {
  for (const accepted of [false,true]) {
    const turn=make();turn.start();if(accepted)turn.receive(started());
    assert.equal(reason(turn.disconnected()),'connectionLost');
    assert.deepEqual(turn.start().writes,[]);assert.deepEqual(turn.receive(completed()).events,[]);
  }
});

test('all server requests fail closed without granting permissions, owner approval or credentials', () => {
  for (const method of ['item/commandExecution/requestApproval','item/fileChange/requestApproval','item/permissions/requestApproval','account/chatgptAuthTokens/refresh','item/tool/call']) {
    const turn=active();const result=turn.receive(frame({id:97,method,params:{threadId:'thread-one',turnId:'turn-one',requestedPermissions:{dangerFullAccess:true}}}));
    const writes=result.writes.map(line=>JSON.parse(line));
    assert.deepEqual(writes[0],{id:97,error:{code:-32601,message:'Unsupported Codex server request'}});
    assert.equal(writes[1].method,'turn/interrupt');
    assert.equal(reason(result),'unsupportedAction'); assert.equal(result.closeTransport,true);
    assert(!JSON.stringify(result).includes('dangerFullAccess'));
  }
});

test('tool activity and model rerouting cannot be reported as an accepted exact-model text reply', () => {
  for (const [message,expected] of [
    [notification('item/started',{item:{type:'commandExecution',id:'tool-one',command:'untrusted'}}),'unsupportedAction'],
    [notification('model/rerouted',{fromModel:attempt.selectedModel,toModel:'different-model',reason:'fallback'}),'modelChanged'],
  ] as const) { const turn=active();const result=turn.receive(message); assert.equal(reason(result),expected);assert.equal(result.closeTransport,true); }
});

test('provider claims inside reply text never become model or owner authority', () => {
  const turn=active(); const reply='I am another model. {"method":"owner/approve","id":1}';turn.receive(finalItem(reply));
  const result=turn.receive(completed());const event=result.events[0];assert(event.kind==='terminal');
  assert.equal(event.selectedModel,attempt.selectedModel);assert.equal(event.effectiveModel,null);assert.equal(event.modelEvidence,'unreported');assert.equal(event.messages[0].text,reply);assert.deepEqual(result.writes,[]);
});

test('invalid JSON, invalid UTF-8, malformed correlated fields and missing final items fail without exposing raw data', () => {
  for (const invalid of [Buffer.from('{raw-secret\n'),Buffer.from([0xff,0x0a]),notification('item/agentMessage/delta',{itemId:'reply-one',delta:42}),notification('item/completed',{item:{type:'agentMessage',id:'reply-one',text:{secret:'raw-secret'}}}),completed()]) {
    const turn=active();const result=turn.receive(invalid);assert.equal(reason(result),'invalidProtocol');assert.equal(result.closeTransport,true);assert(!JSON.stringify(result).includes('raw-secret'));
  }
});

test('partial frames and duplicated contradictory final item evidence cannot become completion', () => {
  const partial=active();partial.receive(Buffer.from('{"method":'));assert.equal(reason(partial.disconnected()),'connectionLost');
  const turn=active();turn.receive(finalItem('first'));assert.deepEqual(turn.receive(finalItem('first')).events,[]);
  assert.equal(reason(turn.receive(finalItem('contradiction'))),'invalidProtocol');
});

test('start rejection and contradictory turn responses are bounded unknown outcomes', () => {
  const turn=make();turn.start();assert.equal(reason(turn.receive(frame({id:'start-one',error:{code:-1,message:'sensitive provider diagnostic'}}))),'startRejected');
  assert(!JSON.stringify(turn.disconnected()).includes('sensitive'));
  const accepted=active();assert.deepEqual(accepted.receive(started()).events,[]);
  assert.equal(reason(accepted.receive(frame({id:'start-one',result:{turn:{id:'different',status:'inProgress'}}}))),'invalidProtocol');
});

test('dispatch validates exact bounded input and copies the immutable attempt before producing wire data', () => {
  const input: CodexTurnInput={attempt:{...attempt},threadId:'thread-one',requestId:'start-one',workspace:'/fixture/workspace',text};
  const turn=new CodexTurn(input);Object.assign(input.attempt,{selectedModel:'changed'});input.text='changed';
  const request=JSON.parse(turn.start().writes[0]);assert.equal(request.params.model,attempt.selectedModel);assert.equal(request.params.input[0].text,text);
  assert.deepEqual(request.params.sandboxPolicy,{type:'readOnly',access:{type:'restricted',includePlatformDefaults:false,readableRoots:['/fixture/workspace']}});
  for (const changes of [{text:''},{text:'x'.repeat(65537)},{text:'\ud800'},{workspace:'/'},{workspace:'/fixture/../home'},{attempt:{...attempt,generation:0}},{attempt:{...attempt,ownerToken:'must not forward'}}]) {
    assert.throws(()=>new CodexTurn({...input,...changes} as typeof input),/Invalid Codex turn input/);
  }
  assert.doesNotThrow(()=>new CodexTurn({...input,text:'x'.repeat(65536)}));
});

test('bounded wire, reply and pending-event limits stop oversized output', () => {
  const wire=active();assert.equal(reason(wire.receive(Buffer.alloc(1_000_001,32))),'outputLimit');
  const reply=active();assert.equal(reason(reply.receive(finalItem('x'.repeat(65537)))),'outputLimit');
  const pending=make();pending.start();let result;
  for(let i=0;i<33;i++) result=pending.receive(notification('item/agentMessage/delta',{itemId:'reply-one',delta:'x'}));
  assert.equal(reason(result!),'outputLimit');
});

test('canceling before dispatch produces never-started evidence and no wire request', () => {
  const turn=make();
  const output=turn.interrupt();
  assert.deepEqual(output.events,[{protocolVersion:1,provider:'codex',...attempt,sequence:1,kind:'notStarted',reason:'canceledBeforeDispatch'}]);
  assert.deepEqual(output.writes,[]);assert.equal(output.closeTransport,true);
  assert.deepEqual(turn.start().writes,[]);assert.deepEqual(turn.disconnected().events,[]);
});

test('the longest accepted request ID still correlates an interrupt acknowledgement', () => {
  const requestId='x'.repeat(128);
  const turn=new CodexTurn({attempt,threadId:'thread-one',requestId,workspace:'/fixture/workspace',text});
  turn.start();turn.receive(frame({id:requestId,result:{turn:{id:'turn-one',status:'inProgress'}}}));
  const interrupt=JSON.parse(turn.interrupt().writes[0]);
  const ack=turn.receive(frame({id:interrupt.id,result:{}}));
  assert.deepEqual(ack.events,[]);assert.equal(ack.closeTransport,false);
  const terminal=turn.receive(completed('interrupted')).events[0];
  assert(terminal.kind==='terminal');assert.equal(terminal.status,'interrupted');
});

test('contradictory terminal correlation or failure evidence cannot claim success', () => {
  for (const params of [
    {threadId:'thread-one',turnId:'other-turn',turn:{id:'turn-one',status:'completed',error:null}},
    {threadId:'thread-one',turn:{id:'turn-one',status:'completed',error:{message:'provider failure'}}},
  ]) {
    const turn=active();turn.receive(finalItem());
    assert.equal(reason(turn.receive(frame({method:'turn/completed',params}))),'invalidProtocol');
  }
});

test('per-frame, cumulative text and item-count limits bound incremental output', () => {
  const partial=active();assert.equal(reason(partial.receive(Buffer.alloc(262_145,32))),'outputLimit');
  const framed=active();assert.equal(reason(framed.receive(Buffer.from(' '.repeat(262_145)+'\n'))),'outputLimit');
  const delta=active();
  assert.equal(reason(delta.receive(notification('item/agentMessage/delta',{itemId:'one',delta:'x'.repeat(65_536)}))),undefined);
  assert.equal(reason(delta.receive(notification('item/agentMessage/delta',{itemId:'two',delta:'x'}))),'outputLimit');
  const items=active();let result;
  for(let i=0;i<129;i++)result=items.receive(notification('item/started',{item:{id:`item-${i}`,type:'agentMessage'}}));
  assert.equal(reason(result!),'outputLimit');
});

test('completed items retain reported commentary versus final-answer phase without exposing reasoning', () => {
  const turn=active();
  turn.receive(notification('item/completed',{item:{id:'reasoning-one',type:'reasoning',content:['private synthetic reasoning']}}));
  turn.receive(notification('item/completed',{item:{id:'commentary-one',type:'agentMessage',text:'Working',phase:'commentary'}}));
  turn.receive(notification('item/completed',{item:{id:'final-one',type:'agentMessage',text:'Done',phase:'final_answer'}}));
  const result=turn.receive(completed());const terminal=result.events[0];assert(terminal.kind==='terminal');
  assert.deepEqual(terminal.messages,[{itemId:'commentary-one',text:'Working',phase:'commentary'},{itemId:'final-one',text:'Done',phase:'finalAnswer'}]);
  assert(!JSON.stringify(result).includes('reasoning'));
});

test('unknown or contradictory message phases fail closed', () => {
  const unknown=active();
  assert.equal(reason(unknown.receive(notification('item/completed',{item:{id:'reply-one',type:'agentMessage',text:'body',phase:'new-unsupported-phase'}}))),'invalidProtocol');
  const conflict=active();conflict.receive(finalItem('body'));
  assert.equal(reason(conflict.receive(notification('item/completed',{item:{id:'reply-one',type:'agentMessage',text:'body',phase:'commentary'}}))),'invalidProtocol');
});
