import assert from 'node:assert/strict';
import {test,type TestContext} from 'node:test';
import {mkdtemp,rm,realpath,rename,writeFile,chmod} from 'node:fs/promises';
import {join} from 'node:path';
import {request,type RequestOptions} from 'node:https';
import {createServer,type RequestListener} from 'node:http';
import {connect} from 'node:tls';
import {Server} from 'node:net';
import {createApplication} from '../service/openai.ts';
import {CODEX_RUNTIME} from '../runner/codex-descriptor.ts';
import {networkTestIdentity} from './network-test-identity.ts';
import {openSecureNetwork,type SecureNetworkOptions} from './secure-network.ts';

const identity=networkTestIdentity();
async function fixture(t:TestContext) {
  const directory=await realpath(await mkdtemp('/tmp/fort-net-')),previous=process.cwd();process.chdir(directory);
  const service=createApplication({dataDirectory:directory,provider:CODEX_RUNTIME,ownerPairingPolicy:{expiresAfterDays:null},runtime:{async execute(){return {kind:'notStarted',message:'Synthetic transport test'};},async dispose(){}}});
  t.after(async()=>{await service.close();process.chdir(previous);await rm(directory,{recursive:true,force:true});});
  await service.start({listener:{kind:'privateSocket',name:'core.sock'}});
  const endpoint=service.endpoint;assert.ok(endpoint?.kind==='privateSocket');
  const options:SecureNetworkOptions={bindAddress:'127.0.0.1',prefixLength:32,port:0,serverName:'127.0.0.1',certificate:identity.certificate,privateKey:identity.privateKey,endpoint};
  return {service,options};
}
function call(url:string,path:string,headers:Record<string,string>={},body?:unknown,options:RequestOptions={}) {
  return new Promise<{status:number;body:Record<string,unknown>}>((resolve,reject)=>{
    const data=body===undefined?undefined:Buffer.from(JSON.stringify(body));
    const req=request(new URL(path,url),{agent:false,ca:identity.ca,servername:'fort-fixture.local',method:data?'POST':'GET',headers:{'Content-Type':'application/json',...headers,...(data?{'Content-Length':String(data.length)}:{})},...options},res=>{
      const parts:Buffer[]=[];res.on('data',chunk=>parts.push(chunk));res.once('error',reject);res.once('end',()=>{try{resolve({status:res.statusCode!,body:JSON.parse(Buffer.concat(parts).toString())});}catch(error){reject(error);}});
    });req.once('error',reject);req.end(data);
  });
}
async function upstreamFixture(t:TestContext,handler:RequestListener) {
  const directory=await realpath(await mkdtemp('/tmp/fort-tls-'));
  const server=createServer(handler),path=join(directory,'core.sock');
  t.after(async()=>{server.closeAllConnections();await new Promise<void>(resolve=>server.close(()=>resolve()));await rm(directory,{recursive:true,force:true});});
  await new Promise<void>((resolve,reject)=>{server.once('error',reject);server.listen(path,resolve);});
  await chmod(path,0o600);
  const options:SecureNetworkOptions={bindAddress:'127.0.0.1',prefixLength:32,port:0,serverName:'127.0.0.1',certificate:identity.certificate,privateKey:identity.privateKey,endpoint:{kind:'privateSocket',directory,name:'core.sock'}};
  return options;
}
test('explicit TLS transport retains core enrollment authentication and revocation',async t=>{
  const f=await fixture(t),network=await openSecureNetwork(f.options);t.after(()=>network.close());
  const headers={'X-Fort-Installation-Id':f.service.installation.installationId};
  assert.equal((await call(network.url,'/v1/agents',headers)).status,401);
  const enrolled=await call(network.url,'/v1/devices/enroll',headers,{...f.service.issueEnrollment(),deviceName:'Synthetic phone'});
  assert.equal(enrolled.status,201);
  const authenticated={...headers,Authorization:'Bearer '+enrolled.body.deviceToken};
  assert.equal((await call(network.url,'/v1/agents',authenticated)).status,200);
  assert.equal((await call(network.url,'/v1/pairing/disconnect',authenticated,{})).status,200);
  assert.equal((await call(network.url,'/v1/agents',authenticated)).status,401);
  await network.close();await assert.rejects(call(network.url,'/v1/agents',authenticated));
});

test('transport lifetime observes cancellation without initiating shutdown and permits exact port reuse',async t=>{
  const options=await upstreamFixture(t,(_request,response)=>response.end('{}'));
  const signal=new AbortController(),network=await openSecureNetwork({...options,signal:signal.signal});
  t.after(()=>network.close());
  let completed=false;
  const observed=network.closed.then(()=>{completed=true;});
  assert.equal((await call(network.url,'/v1/agents',{'X-Fort-Installation-Id':'fixture'})).status,200);
  assert.equal(completed,false);
  signal.abort();await observed;assert.equal(completed,true);
  await assert.rejects(call(network.url,'/v1/agents',{'X-Fort-Installation-Id':'fixture'}));
  const replacement=await openSecureNetwork({...options,port:Number(new URL(network.url).port)});
  t.after(()=>replacement.close());
  assert.equal((await call(replacement.url,'/v1/agents',{'X-Fort-Installation-Id':'fixture'})).status,200);
});

test('a server error closes the observable transport without an explicit stop',async t=>{
  const options=await upstreamFixture(t,(_request,response)=>response.end('{}'));
  let listener:Server|undefined;
  const listen=Server.prototype.listen;
  const intercepted=t.mock.method(Server.prototype,'listen',function(this:Server,...args:Parameters<Server['listen']>){
    listener=this;return Reflect.apply(listen,this,args);
  });
  const network=await openSecureNetwork(options);t.after(()=>network.close());
  intercepted.mock.restore();assert.ok(listener);
  listener.emit('error',new Error('Synthetic server boundary failure'));
  await network.closed;
  await assert.rejects(call(network.url,'/v1/agents',{'X-Fort-Installation-Id':'fixture'}));
});

test('unconfirmed transport shutdown rejects every observer and cannot later report success',{timeout:8000},async t=>{
  const options=await upstreamFixture(t,(_request,response)=>response.end('{}'));
  const network=await openSecureNetwork(options);
  const close=Server.prototype.close;
  const intercepted=t.mock.method(Server.prototype,'close',function(this:Server){return close.call(this);});
  t.after(()=>intercepted.mock.restore());
  const observed=assert.rejects(network.closed,{message:'Secure network transport could not be verified'});
  const started=performance.now(),stopping=network.close();
  assert.equal(stopping,network.closed);
  await assert.rejects(stopping,{message:'Secure network transport could not be verified'});
  await observed;assert.ok(performance.now()-started<6500);
  intercepted.mock.restore();
  await assert.rejects(network.close(),{message:'Secure network transport could not be verified'});
  const replacement=await openSecureNetwork({...options,port:Number(new URL(network.url).port)});
  t.after(()=>replacement.close());
});

test('secure transport refuses unselected or mismatched bind, TLS and socket identities before listening',async t=>{
  const f=await fixture(t);
  const overrides:Partial<SecureNetworkOptions>[]=[
    {bindAddress:'0.0.0.0'},{bindAddress:'::'},{bindAddress:'192.0.2.1'},{bindAddress:'localhost'},
    {bindAddress:'192.168.4.0',prefixLength:24,port:4443},{bindAddress:'192.168.4.255',prefixLength:24,port:4443},
    {bindAddress:'192.168.4.2',prefixLength:24},{prefixLength:24},{prefixLength:NaN},{port:443},{port:65536},
    {serverName:'another.invalid'},{serverName:'../escape'},{serverName:'127.0.0.2'},
    {certificate:identity.ca},{certificate:Buffer.from('not a certificate')},{privateKey:Buffer.from('not a key')},
    {certificate:Buffer.alloc(65537)},{privateKey:Buffer.alloc(65537)},
    {endpoint:{...f.options.endpoint,name:'../core.sock'}},{endpoint:{...f.options.endpoint,directory:'/tmp'}},
    {signal:AbortSignal.abort()}
  ];
  for(const change of overrides)await assert.rejects(openSecureNetwork({...f.options,...change}));
  const network=await openSecureNetwork(f.options);await network.close();
});

test('an expired, future or mismatched TLS identity cannot activate the transport',async t=>{
  const f=await fixture(t),other=networkTestIdentity();
  await assert.rejects(openSecureNetwork({...f.options,privateKey:other.privateKey}));
  for(const offset of [-86400000,3*86400000]) {
    const now=Date.now(),clock=t.mock.method(Date,'now',()=>now+offset);
    try {await assert.rejects(openSecureNetwork(f.options));}finally{clock.mock.restore();}
  }
});

test('a private bind cannot select a subnet containing public addresses before any bind attempt',async t=>{
  const f=await fixture(t);let attempts=0;
  t.mock.method(Server.prototype,'listen',()=>{attempts++;throw new Error('Synthetic listener boundary');});
  for(const change of [{bindAddress:'192.168.1.2',prefixLength:15},{bindAddress:'172.16.1.2',prefixLength:11}]) {
    await assert.rejects(openSecureNetwork({...f.options,port:4443,...change}));
    assert.equal(attempts,0);
  }
  await assert.rejects(openSecureNetwork(f.options));assert.equal(attempts,1);
});

test('TLS trust and selected subnet are enforced independently of bearer authentication',async t=>{
  const f=await fixture(t),network=await openSecureNetwork(f.options);t.after(()=>network.close());
  const headers={'X-Fort-Installation-Id':f.service.installation.installationId};
  await assert.rejects(call(network.url,'/v1/agents',headers,undefined,{ca:undefined}),/certificate|issuer/i);
  await assert.rejects(call(network.url,'/v1/agents',headers,undefined,{maxVersion:'TLSv1.2'}));
  await t.test('outside-subnet loopback peer is refused when the OS exposes that source address',async t=>{
    try {assert.equal((await call(network.url,'/v1/agents',headers,undefined,{localAddress:'127.0.0.2'})).status,400);}
    catch(error){if(error instanceof Error && 'code' in error && error.code==='EADDRNOTAVAIL')t.skip('The OS has no 127.0.0.2 source address; no interface setting was changed');else throw error;}
  });
  assert.equal((await call(network.url,'/v1/agents',headers)).status,401);
});

test('the network boundary refuses browser authority, duplicate headers and invalid routes or methods',async t=>{
  const f=await fixture(t),network=await openSecureNetwork(f.options);t.after(()=>network.close());
  const headers={'X-Fort-Installation-Id':f.service.installation.installationId};
  const forgedHeaders:Record<string,string>[]=[{Host:'another.invalid'}, {Origin:'https://another.invalid'}, {Cookie:'owner=true'}, {'X-Forwarded-For':'127.0.0.1'}, {Forwarded:'for=127.0.0.1'}, {Authorization:'Basic owner'}];
  for(const change of forgedHeaders) {
    assert.equal((await call(network.url,'/v1/agents',{...headers,...change})).status,400);
  }
  assert.equal((await call(network.url,'/v1/agents',headers,undefined,{method:'DELETE'})).status,400);
  for(const path of ['/v1/%2Fadmin','/v1/%00admin','/v1/%2E%2E/agents','/not-fort']) {
    assert.equal((await call(network.url,'/v1/agents',headers,undefined,{path})).status,400);
  }
  const duplicate=['Host',new URL(network.url).host,'X-Fort-Installation-Id',f.service.installation.installationId,'X-Fort-Installation-Id',f.service.installation.installationId];
  assert.equal((await call(network.url,'/v1/agents',headers,undefined,{headers:duplicate})).status,400);
  assert.equal((await call(network.url,'/v1/agents',headers,undefined,{headers:{...headers,'Content-Length':String(1024*1024+1)}})).status,400);
  assert.equal(f.service.inspectSession().deviceIDs.length,0);
});

test('oversized upstream responses are refused without forwarding their contents',async t=>{
  const options=await upstreamFixture(t,(_request,response)=>response.end('x'.repeat(8*1024*1024+1)));
  const network=await openSecureNetwork(options);t.after(()=>network.close());
  const result=await call(network.url,'/v1/agents',{'X-Fort-Installation-Id':'fixture'});
  assert.equal(result.status,400);assert.deepEqual(Object.keys(result.body),['error']);
});

test('stop aborts active upstream work, closes idle TLS connections and is idempotent',{timeout:5000},async t=>{
  const received=Promise.withResolvers<void>(),upstreamClosed=Promise.withResolvers<void>();
  const options=await upstreamFixture(t,(_request,response)=>{response.once('close',()=>upstreamClosed.resolve());received.resolve();});
  const network=await openSecureNetwork(options);t.after(()=>network.close());
  const pending=call(network.url,'/v1/agents',{'X-Fort-Installation-Id':'fixture'});
  const rejected=assert.rejects(pending);
  await received.promise;
  const started=performance.now(),stopping=network.close();
  assert.equal(stopping,network.closed);assert.equal(network.close(),stopping);
  await Promise.all([stopping,network.closed]);
  await rejected;await upstreamClosed.promise;
  assert.ok(performance.now()-started<1000);
  await assert.rejects(call(network.url,'/v1/agents',{'X-Fort-Installation-Id':'fixture'}));
});

test('at most eight TLS connections are retained and stop closes every connection',{timeout:5000},async t=>{
  const options=await upstreamFixture(t,(_request,response)=>response.end('{}'));
  const network=await openSecureNetwork(options);t.after(()=>network.close());
  const target=new URL(network.url),connections:ReturnType<typeof connect>[]=[];
  t.after(()=>{for(const socket of connections)socket.destroy();});
  async function open() {
    const socket=connect({host:target.hostname,port:Number(target.port),ca:identity.ca,servername:'fort-fixture.local'});
    connections.push(socket);socket.on('error',()=>{});
    await new Promise<void>((resolve,reject)=>{socket.once('secureConnect',resolve);socket.once('error',reject);});
    return socket;
  }
  for(let index=0;index<8;index++)await open();
  await assert.rejects(open());
  const closed=connections.filter(socket=>!socket.closed).map(socket=>new Promise<void>(resolve=>socket.once('close',()=>resolve())));
  await network.close();await Promise.all(closed);
  assert.ok(connections.every(socket=>socket.destroyed));
});

test('abort during bind leaves the exact port available and later abort closes the listener',{timeout:5000},async t=>{
  const options=await upstreamFixture(t,(_request,response)=>response.end('{}'));
  const first=await openSecureNetwork(options),port=Number(new URL(first.url).port);await first.close();
  const early=new AbortController(),opening=openSecureNetwork({...options,port,signal:early.signal});early.abort();
  await assert.rejects(opening);
  const later=new AbortController(),network=await openSecureNetwork({...options,port,signal:later.signal});t.after(()=>network.close());
  await assert.rejects(openSecureNetwork({...options,port}));
  assert.equal((await call(network.url,'/v1/agents',{'X-Fort-Installation-Id':'fixture'})).status,200);
  later.abort();await network.close();
  await assert.rejects(call(network.url,'/v1/agents',{'X-Fort-Installation-Id':'fixture'}));
});

test('changed private socket identity refuses forwarding and preserves the replacement',async t=>{
  const f=await fixture(t),network=await openSecureNetwork(f.options);t.after(()=>network.close());
  const path=join(f.options.endpoint.directory,'core.sock'),moved=path+'.original';
  await rename(path,moved);await writeFile(path,'synthetic replacement',{mode:0o600});
  try {assert.equal((await call(network.url,'/v1/agents',{'X-Fort-Installation-Id':f.service.installation.installationId})).status,400);}
  finally{await rm(path);await rename(moved,path);}
  await chmod(path,0o666);
  try {assert.equal((await call(network.url,'/v1/agents',{'X-Fort-Installation-Id':f.service.installation.installationId})).status,400);}
  finally{await chmod(path,0o600);}
});

test('a slow request cannot extend the thirty-second total lifetime by trickling bytes',{timeout:36000},async t=>{
  const f=await fixture(t),network=await openSecureNetwork(f.options);t.after(()=>network.close());
  await new Promise<void>((resolve,reject)=>{
    const started=performance.now();
    const req=request(new URL('/v1/devices/enroll',network.url),{agent:false,ca:identity.ca,method:'POST',headers:{'Content-Type':'application/json','Content-Length':'100','X-Fort-Installation-Id':f.service.installation.installationId}},res=>{res.resume();res.once('end',done);});
    const interval=setInterval(()=>req.write(' '),2000);
    const deadline=setTimeout(()=>{reject(new Error('A trickling request outlived its total deadline'));req.destroy();},33000);
    function done(){clearInterval(interval);clearTimeout(deadline);assert.ok(performance.now()-started<32500);resolve();}
    req.once('error',done);req.write('{');
    t.after(()=>{clearInterval(interval);clearTimeout(deadline);req.destroy();});
  });
});

test('twenty isolated warm TLS journeys meet the transport budget',{skip:!process.env.FORT_SECURE_NETWORK_BENCHMARK},async t=>{
  const destination=process.env.FORT_SECURE_NETWORK_BENCHMARK;assert.ok(destination);
  const payload=JSON.stringify({data:'x'.repeat(1013)});assert.equal(Buffer.byteLength(payload),1024);
  const options=await upstreamFixture(t,(_request,response)=>response.end(payload));
  const network=await openSecureNetwork(options);t.after(()=>network.close());
  const samples:number[]=[];
  for(let index=0;index<25;index++) {
    const started=performance.now(),result=await call(network.url,'/v1/agents',{'X-Fort-Installation-Id':'fixture'});
    const elapsed=performance.now()-started;
    assert.equal(result.status,200);assert.equal(JSON.stringify(result.body),payload);
    if(index>=5)samples.push(elapsed);
  }
  const started=performance.now();await network.close();const closeMs=performance.now()-started;
  const sorted=[...samples].sort((a,b)=>a-b),p95=sorted[18];
  await writeFile(destination,JSON.stringify({node:process.version,platform:process.platform,architecture:process.arch,responseBytes:1024,warmups:5,samplesMs:samples,p50Ms:sorted[9],p95Ms:p95,maximumMs:sorted[19],closeMs},null,2)+'\n',{flag:'wx'});
  assert.ok(p95<100);assert.ok(closeMs<5000);
});

test('twenty isolated transport lifetimes meet the shutdown observation budget',{skip:!process.env.FORT_SECURE_NETWORK_LIFETIME_BENCHMARK},async t=>{
  const destination=process.env.FORT_SECURE_NETWORK_LIFETIME_BENCHMARK;assert.ok(destination);
  const options=await upstreamFixture(t,(_request,response)=>response.end('{}'));
  const samples:number[]=[];
  for(let index=0;index<25;index++) {
    const started=performance.now(),signal=new AbortController();
    const network=await openSecureNetwork({...options,signal:signal.signal});
    const port=Number(new URL(network.url).port);
    assert.equal((await call(network.url,'/v1/agents',{'X-Fort-Installation-Id':'fixture'})).status,200);
    signal.abort();await network.closed;
    const replacement=await openSecureNetwork({...options,port});
    await replacement.close();await replacement.closed;
    if(index>=5)samples.push(performance.now()-started);
  }
  const sorted=[...samples].sort((a,b)=>a-b),p95=sorted[18];
  await writeFile(destination,JSON.stringify({node:process.version,platform:process.platform,architecture:process.arch,warmups:5,samplesMs:samples,p50Ms:sorted[9],p95Ms:p95,maximumMs:sorted[19]},null,2)+'\n',{flag:'wx'});
  assert.ok(p95<100);
});
