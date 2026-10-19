import assert from 'node:assert/strict';
import {test,type TestContext} from 'node:test';
import {mkdtemp,realpath,chmod,rm,writeFile} from 'node:fs/promises';
import {join} from 'node:path';
import {createServer} from 'node:http';
import {request} from 'node:https';
import {Server} from 'node:net';
import {setTimeout as delay} from 'node:timers/promises';
import {createSecureNetworkControl} from './network-control.ts';
import {networkTestIdentity} from './network-test-identity.ts';

const identity=networkTestIdentity();
const owner={installationId:'synthetic-installation',deviceId:'synthetic-primary'};
async function fixture(t:TestContext,authorize:Parameters<typeof createSecureNetworkControl>[0]['authorize']=input=>input.installationId===owner.installationId && input.deviceId===owner.deviceId) {
  const directory=await realpath(await mkdtemp('/tmp/fort-network-control-'));
  const server=createServer((_request,response)=>response.end('{"available":true}'));
  await new Promise<void>((resolve,reject)=>{server.once('error',reject);server.listen(join(directory,'core.sock'),resolve);});
  await chmod(join(directory,'core.sock'),0o600);
  t.after(async()=>{server.closeAllConnections();await new Promise<void>(resolve=>server.close(()=>resolve()));await rm(directory,{recursive:true,force:true});});
  const endpoint={kind:'privateSocket' as const,directory,name:'core.sock'};
  const control=createSecureNetworkControl({endpoint,authorize});
  let unconfirmed=false;
  t.after(async()=>{if(unconfirmed)await assert.rejects(control.close());else await control.close();});
  const configuration={bindAddress:'127.0.0.1',prefixLength:32,port:0,serverName:'127.0.0.1',certificate:identity.certificate.toString('base64'),privateKey:identity.privateKey.toString('base64')};
  return {control,configuration,endpoint,expectUnconfirmedCleanup(){unconfirmed=true;}};
}
function call(url:string) {
  return new Promise<number>((resolve,reject)=>{
    const req=request(new URL('/v1/agents',url),{agent:false,ca:identity.ca,headers:{'X-Fort-Installation-Id':owner.installationId}},res=>{res.resume();res.once('end',()=>resolve(res.statusCode!));});
    req.once('error',reject);req.end();
  });
}

test('explicit owner listener control starts once, reports the actual endpoint and confirms exact stop',async t=>{
  const f=await fixture(t),initial=await f.control.execute({operation:'inspect',owner});
  assert.equal(initial.generation,0);assert.deepEqual(initial.listener,{state:'stopped',cause:'initial'});
  const command={operation:'start',owner,sessionId:initial.sessionId,generation:0,configuration:f.configuration};
  const active=await f.control.execute(command);
  assert.equal(active.generation,1);assert.equal(active.listener.state,'active');
  if(active.listener.state!=='active')throw new Error('Missing active fixture listener');
  assert.equal(await call(active.listener.url),200);
  assert.deepEqual(await f.control.execute(command),active);
  const stop={operation:'stop',owner,sessionId:active.sessionId,generation:active.generation};
  const stopped=await f.control.execute(stop);
  assert.deepEqual(stopped.listener,{state:'stopped',cause:'requested'});
  assert.deepEqual(await f.control.execute(stop),stopped);
  assert.deepEqual(await f.control.execute(command),stopped);
  await assert.rejects(call(active.listener.url));
});

test('a confirmed opening failure consumes one generation and permits a new explicit attempt',async t=>{
  const f=await fixture(t),initial=await f.control.execute({operation:'inspect',owner});
  const failed=await f.control.execute({operation:'start',owner,sessionId:initial.sessionId,generation:0,configuration:{...f.configuration,serverName:'wrong.invalid'}});
  assert.deepEqual(failed.listener,{state:'stopped',cause:'startFailed'});
  assert.equal(failed.generation,1);
  const recovered=await f.control.execute({operation:'start',owner,sessionId:initial.sessionId,generation:1,configuration:f.configuration});
  assert.equal(recovered.listener.state,'active');assert.equal(recovered.generation,2);
});

test('stopping an opening listener confirms absence and rejects a stale stop after replacement',async t=>{
  const f=await fixture(t),initial=await f.control.execute({operation:'inspect',owner});
  const opening=f.control.execute({operation:'start',owner,sessionId:initial.sessionId,generation:0,configuration:f.configuration});
  assert.equal((await f.control.execute({operation:'inspect',owner})).listener.state,'starting');
  const stopped=await f.control.execute({operation:'stop',owner,sessionId:initial.sessionId,generation:1});
  await opening;
  assert.deepEqual(stopped.listener,{state:'stopped',cause:'requested'});
  const replacement=await f.control.execute({operation:'start',owner,sessionId:initial.sessionId,generation:1,configuration:f.configuration});
  assert.equal(replacement.listener.state,'active');assert.equal(replacement.generation,2);
  await assert.rejects(f.control.execute({operation:'stop',owner,sessionId:initial.sessionId,generation:1}));
  assert.deepEqual(await f.control.execute({operation:'inspect',owner}),replacement);
});

test('a newly authorized owner cannot silently adopt an existing listener generation',async t=>{
  let selected=owner;
  const f=await fixture(t,input=>input.installationId===selected.installationId && input.deviceId===selected.deviceId);
  const initial=await f.control.execute({operation:'inspect',owner});
  const active=await f.control.execute({operation:'start',owner,sessionId:initial.sessionId,generation:0,configuration:f.configuration});
  selected={...owner,deviceId:'different-primary'};
  await assert.rejects(f.control.execute({operation:'inspect',owner:selected}));
  await assert.rejects(f.control.execute({operation:'stop',owner:selected,sessionId:initial.sessionId,generation:1}));
  selected=owner;assert.deepEqual(await f.control.execute({operation:'inspect',owner}),active);
});

test('lost owner authority during opening cannot publish or retain an active listener',async t=>{
  let allowed=true;
  const f=await fixture(t,()=>allowed),initial=await f.control.execute({operation:'inspect',owner});
  const opening=f.control.execute({operation:'start',owner,sessionId:initial.sessionId,generation:0,configuration:f.configuration});
  allowed=false;
  const result=await opening;
  assert.equal(result.listener.state,'stopped');
  allowed=true;assert.equal((await f.control.execute({operation:'inspect',owner})).listener.state,'stopped');
});

test('malformed commands are rejected before authorization and never disclose configuration',async t=>{
  let checks=0;
  const f=await fixture(t,()=>{checks++;return true;});
  const initial=await f.control.execute({operation:'inspect',owner});checks=0;
  const start={operation:'start',owner,sessionId:initial.sessionId,generation:0,configuration:f.configuration};
  const invalid:unknown[]=[null,[],{},'inspect',{operation:'inspect',owner,extra:true},{operation:'restart',owner},
    {...start,owner:{...owner,extra:'ignored'}},{...start,owner:{...owner,deviceId:'bad id'}},
    {...start,sessionId:''},{...start,generation:-1},{...start,generation:0.5},{...start,generation:NaN},
    {...start,generation:Number.MAX_SAFE_INTEGER+1},{...start,configuration:{...f.configuration,port:'4443'}},
    {...start,configuration:{...f.configuration,certificate:'YQ'}},{...start,configuration:{...f.configuration,privateKey:''}},
    {...start,configuration:{...f.configuration,privateKey:Buffer.alloc(65537).toString('base64')}},
    {...start,configuration:{...f.configuration,extra:'ignored'}}];
  for(const input of invalid)await assert.rejects(f.control.execute(input),{message:'The exact local listener operation could not be confirmed'});
  assert.equal(checks,0);
  const status=await f.control.execute({operation:'inspect',owner});assert.deepEqual(status,initial);
  assert.ok(!JSON.stringify(status).includes(f.configuration.privateKey));assert.ok(!JSON.stringify(status).includes(f.configuration.certificate));
});

test('denied, foreign or throwing authorization cannot start a listener',async t=>{
  let allowed=false,throws=false;
  const f=await fixture(t,input=>{if(throws)throw new Error('Synthetic private diagnostic');return allowed && input.installationId===owner.installationId && input.deviceId===owner.deviceId;});
  await assert.rejects(f.control.execute({operation:'inspect',owner}));
  allowed=true;const initial=await f.control.execute({operation:'inspect',owner});
  const start={operation:'start',owner,sessionId:initial.sessionId,generation:0,configuration:f.configuration};
  for(const foreign of [{...owner,installationId:'foreign'},{...owner,deviceId:'phone'}])await assert.rejects(f.control.execute({...start,owner:foreign}));
  allowed=false;await assert.rejects(f.control.execute(start));
  throws=true;await assert.rejects(f.control.execute(start),{message:'The exact local listener operation could not be confirmed'});
  throws=false;allowed=true;assert.deepEqual(await f.control.execute({operation:'inspect',owner}),initial);
});

test('concurrent exact starts share one generation while changed retries and stale sessions fail',async t=>{
  const f=await fixture(t),initial=await f.control.execute({operation:'inspect',owner});
  const start={operation:'start',owner,sessionId:initial.sessionId,generation:0,configuration:f.configuration};
  const opening=f.control.execute(start),retry=f.control.execute(start);
  await assert.rejects(f.control.execute({...start,configuration:{...f.configuration,serverName:'changed.invalid'}}));
  const [active,repeated]=await Promise.all([opening,retry]);assert.deepEqual(active,repeated);assert.equal(active.generation,1);
  await assert.rejects(f.control.execute({...start,sessionId:'old-process'}));
  await assert.rejects(f.control.execute({...start,generation:1}));
  await assert.rejects(f.control.execute({...start,generation:0,configuration:{...f.configuration,port:4443}}));
  const fresh=createSecureNetworkControl({endpoint:f.endpoint,authorize:()=>true});t.after(()=>fresh.close());
  const other=await fresh.execute({operation:'inspect',owner});assert.notEqual(other.sessionId,initial.sessionId);
  await assert.rejects(fresh.execute(start));assert.equal(other.generation,0);
});

test('caller mutation cannot change an admitted configuration, captured endpoint or returned status',async t=>{
  const f=await fixture(t),initial=await f.control.execute({operation:'inspect',owner});
  const command={operation:'start',owner:{...owner},sessionId:initial.sessionId,generation:0,configuration:{...f.configuration}};
  const opening=f.control.execute(command);
  command.owner.deviceId='foreign';command.configuration.privateKey='broken';command.configuration.serverName='foreign.invalid';f.endpoint.name='other.sock';
  const active=await opening;assert.equal(active.listener.state,'active');
  if(active.listener.state!=='active')throw new Error('Missing active fixture listener');
  assert.equal(await call(active.listener.url),200);
  Object.assign(active.owner,{deviceId:'mutated'});Object.assign(active.listener,{url:'https://wrong.invalid:443'});
  const restored=await f.control.execute({operation:'inspect',owner});assert.deepEqual(restored.owner,owner);
  assert.equal(restored.listener.state,'active');
  if(restored.listener.state==='active')assert.equal(await call(restored.listener.url),200);
});

test('final close aborts opening, is idempotent and refuses every later command',async t=>{
  const f=await fixture(t),initial=await f.control.execute({operation:'inspect',owner});
  const opening=f.control.execute({operation:'start',owner,sessionId:initial.sessionId,generation:0,configuration:f.configuration});
  const closing=f.control.close();assert.equal(f.control.close(),closing);await closing;
  assert.equal((await opening).listener.state,'stopped');
  for(const command of [{operation:'inspect',owner},{operation:'stop',owner,sessionId:initial.sessionId,generation:1},{operation:'start',owner,sessionId:initial.sessionId,generation:1,configuration:f.configuration}])await assert.rejects(f.control.execute(command));
});

test('unexpected server closure is visible without a control stop',async t=>{
  const f=await fixture(t),initial=await f.control.execute({operation:'inspect',owner});
  let listener:Server|undefined;const listen=Server.prototype.listen;
  const intercepted=t.mock.method(Server.prototype,'listen',function(this:Server,...args:Parameters<Server['listen']>){listener=this;return Reflect.apply(listen,this,args);});
  const active=await f.control.execute({operation:'start',owner,sessionId:initial.sessionId,generation:0,configuration:f.configuration});
  intercepted.mock.restore();assert.ok(listener);listener.emit('error',new Error('Synthetic server error'));
  let observed=await f.control.execute({operation:'inspect',owner});
  for(let count=0;count<100 && observed.listener.state==='active';count++){await delay(1);observed=await f.control.execute({operation:'inspect',owner});}
  assert.deepEqual(observed.listener,{state:'stopped',cause:'interrupted'});
  if(active.listener.state!=='active')throw new Error('Missing active fixture listener');
  await assert.rejects(call(active.listener.url));
});

test('unconfirmed shutdown remains visible and blocks replacement',{timeout:8000},async t=>{
  const f=await fixture(t),initial=await f.control.execute({operation:'inspect',owner});
  const active=await f.control.execute({operation:'start',owner,sessionId:initial.sessionId,generation:0,configuration:f.configuration});
  const close=Server.prototype.close;
  const intercepted=t.mock.method(Server.prototype,'close',function(this:Server){return close.call(this);});
  f.expectUnconfirmedCleanup();
  try {
    const result=await f.control.execute({operation:'stop',owner,sessionId:active.sessionId,generation:active.generation});
    assert.deepEqual(result.listener,{state:'unconfirmed'});
  }finally{intercepted.mock.restore();}
  await assert.rejects(f.control.execute({operation:'start',owner,sessionId:active.sessionId,generation:active.generation,configuration:f.configuration}));
  assert.deepEqual((await f.control.execute({operation:'stop',owner,sessionId:active.sessionId,generation:active.generation})).listener,{state:'unconfirmed'});
  await assert.rejects(f.control.close());
});

test('twenty isolated owner listener control journeys meet the budget',{skip:!process.env.FORT_NETWORK_CONTROL_BENCHMARK},async t=>{
  const destination=process.env.FORT_NETWORK_CONTROL_BENCHMARK;assert.ok(destination);
  const f=await fixture(t),samples:number[]=[];
  for(let index=0;index<25;index++) {
    const started=performance.now(),initial=await f.control.execute({operation:'inspect',owner});
    const command={operation:'start',owner,sessionId:initial.sessionId,generation:initial.generation,configuration:f.configuration};
    const active=await f.control.execute(command);assert.equal(active.listener.state,'active');
    assert.deepEqual(await f.control.execute(command),active);
    if(active.listener.state!=='active')throw new Error('Missing active fixture listener');
    assert.equal(await call(active.listener.url),200);
    assert.equal((await f.control.execute({operation:'stop',owner,sessionId:active.sessionId,generation:active.generation})).listener.state,'stopped');
    if(index>=5)samples.push(performance.now()-started);
  }
  const sorted=[...samples].sort((a,b)=>a-b);
  await writeFile(destination,JSON.stringify({node:process.version,platform:process.platform,architecture:process.arch,warmups:5,samplesMs:samples,p50Ms:sorted[9],p95Ms:sorted[18],maximumMs:sorted[19]},null,2)+'\n',{flag:'wx'});
  assert.ok(sorted[18]<100);
});


test('owner reconciliation closes admitted work and retains the explicit generation',async t=>{
  let allowed=true;const f=await fixture(t,(_owner,operation)=>operation!=='start'||allowed);
  const initial=await f.control.execute({operation:'inspect',owner});
  const start={operation:'start',owner,sessionId:initial.sessionId,generation:0,configuration:f.configuration};
  const active=await f.control.execute(start);allowed=false;await f.control.reconcileOwner();
  const stopped=await f.control.execute({operation:'inspect',owner});
  assert.deepEqual(stopped.listener,{state:'stopped',cause:'ownerUnavailable'});assert.equal(stopped.generation,1);
  if(active.listener.state!=='active')throw new Error('Missing active fixture listener');
  await assert.rejects(call(active.listener.url));await assert.rejects(f.control.execute({...start,generation:1}));
  allowed=true;assert.deepEqual(await f.control.execute(start),stopped);
  assert.equal((await f.control.execute({...start,generation:1})).listener.state,'active');
});
