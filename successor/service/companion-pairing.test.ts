import {test,type TestContext} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm,realpath,writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {randomBytes,randomUUID} from 'node:crypto';
import {DatabaseSync} from 'node:sqlite';
import {createApplication} from './openai.ts';
import {CODEX_RUNTIME} from '../runner/codex-descriptor.ts';
import {deleteOwnerHistory} from './delete-history.ts';
import {migrateLedger} from './migrate-ledger.ts';

const secret=()=>randomBytes(32).toString('base64url');
async function fixture(t:TestContext) {
  const directory=await realpath(await mkdtemp(join(tmpdir(),'fort-companion-')));
  let now=Date.now(),failStop=false,stops=0,restarts=0;
  const options={provider:CODEX_RUNTIME,dataDirectory:directory,ownerPairingPolicy:{expiresAfterDays:null},now:()=>now,
    runtime:{async execute(){return {kind:'notStarted' as const,message:'Synthetic pairing test'};},async dispose(){stops++;if(failStop)throw new Error('Synthetic stop failure');},readiness(){return {state:'ready' as const,canRestart:true,detail:'Synthetic runtime'};},async restart(){restarts++;}}};
  let service=createApplication(options);await service.start();
  t.after(async()=>{failStop=false;await service.close();await rm(directory,{recursive:true,force:true});});
  const host={deviceId:randomUUID(),deviceToken:secret()};
  const setup=service.beginOwnerSetup();assert.equal(setup.kind,'available');
  if(setup.kind!=='available')throw new Error('Missing synthetic setup');
  await service.completeOwnerSetup({...host,installationId:setup.installationId,setupId:setup.setupId});
  async function api<T=Record<string,unknown>>(path:string,body?:unknown,token:string|undefined=host.deviceToken,installationId=service.installation.installationId) {
    const response=await fetch(service.baseURL+path,{method:body===undefined?'GET':'POST',headers:{'Content-Type':'application/json','X-Fort-Installation-Id':installationId,...(token?{Authorization:'Bearer '+token}:{})},body:body===undefined?undefined:JSON.stringify(body)});
    return {status:response.status,body:await response.json() as T};
  }
  function invitation(){return {invitationId:randomUUID(),code:secret()};}
  function phone(offer:ReturnType<typeof invitation>){return {...offer,installationId:service.installation.installationId,deviceId:randomUUID(),deviceToken:secret(),deviceName:'Synthetic phone'};}
  return {host,api,invitation,phone,directory,get service(){return service;},get stops(){return stops;},get restarts(){return restarts;},advance(ms:number){now+=ms;},failStop(value:boolean){failStop=value;},async reopen(){await service.close();service=createApplication(options);await service.start();}};
}

test('an explicitly invited phone shares the owner ledger without replacing the Mac control identity',async t=>{
  const h=await fixture(t),offer=h.invitation();
  assert.equal((await h.api('/v1/pairing/invitation',offer)).status,201);
  const phone=h.phone(offer),joined=await h.api('/v1/devices/join',phone,'');
  assert.equal(joined.status,201);assert.equal(joined.body.deviceId,phone.deviceId);
  assert.equal(joined.body.expiresAt,null);assert.equal(joined.body.deviceToken,undefined);
  const agent=await h.api('/v1/agents',{name:'Shared synthetic agent',model:'exact',instructions:'Synthetic identity'});
  assert.equal(agent.status,201);
  assert.deepEqual((await h.api('/v1/agents',undefined,phone.deviceToken)).body.agents,[agent.body]);
  assert.equal((await h.api('/v1/pairing',undefined,phone.deviceToken)).body.role,'companion');
  const restored=h.service.beginOwnerSetup();assert.equal(restored.kind,'paired');
  if(restored.kind!=='paired')throw new Error('Missing host pairing');
  assert.equal(restored.deviceId,h.host.deviceId);
  assert.deepEqual(new Set(h.service.inspectSession().deviceIDs),new Set([h.host.deviceId,phone.deviceId]));
});

test('the Mac can inspect its bounded companion status without disclosing credentials',async t=>{
  const h=await fixture(t),offer=h.invitation();
  assert.deepEqual((await h.api('/v1/pairing/companion')).body,{installationId:h.service.installation.installationId,hostDeviceId:h.host.deviceId,pairing:{state:'available'}});
  const invited=await h.api('/v1/pairing/invitation',offer);
  assert.deepEqual((await h.api('/v1/pairing/companion')).body.pairing,{state:'invited',invitationId:offer.invitationId,expiresAt:invited.body.expiresAt});
  const phone=h.phone(offer);await h.api('/v1/devices/join',phone,'');await h.reopen();
  const current=await h.api('/v1/pairing/companion');assert.equal(current.status,200);
  assert.deepEqual(current.body.pairing,{state:'paired',deviceId:phone.deviceId,deviceName:phone.deviceName,expiresAt:null,active:true});
  assert.ok(!JSON.stringify(current.body).includes(offer.code));assert.ok(!JSON.stringify(current.body).includes(phone.deviceToken));
  assert.equal((await h.api('/v1/pairing/companion',undefined,phone.deviceToken)).status,403);
  assert.equal((await h.api('/v1/pairing/revoke-companion',{deviceId:phone.deviceId})).status,200);
  assert.deepEqual((await h.api('/v1/pairing/companion')).body.pairing,{state:'available'});
});

test('pairing changes wake existing replay clients once and exact retries add no invalidation',async t=>{
  const h=await fixture(t),offer=h.invitation(),cursor=async()=>Number((await h.api('/v1/installation')).body.eventCursor);
  const initial=await cursor();await h.api('/v1/pairing/invitation',offer);assert.equal(await cursor(),initial+1);
  await h.api('/v1/pairing/invitation',offer);assert.equal(await cursor(),initial+1);
  const phone=h.phone(offer);await h.api('/v1/devices/join',phone,'');assert.equal(await cursor(),initial+2);
  await h.api('/v1/devices/join',phone,'');assert.equal(await cursor(),initial+2);
  await h.api('/v1/pairing/leave',{deviceId:phone.deviceId},phone.deviceToken);assert.equal(await cursor(),initial+3);
  await h.api('/v1/pairing/leave',{deviceId:phone.deviceId},phone.deviceToken);assert.equal(await cursor(),initial+3);
  const replay=await h.api<{events:{kind:string;conversationId:string}[]}>('/v1/events?after='+initial+'&recoveryEpoch='+h.service.installation.recoveryEpoch);
  assert.deepEqual(replay.body.events.map(event=>[event.kind,event.conversationId]),Array.from({length:3},()=>['pairingChanged','_workspace']));
});

test('expiry changes wake other devices without writing again for an unchanged no-expiry choice',async t=>{
  const h=await fixture(t),offer=h.invitation();await h.api('/v1/pairing/invitation',offer);
  const phone=h.phone(offer);await h.api('/v1/devices/join',phone,'');
  const cursor=async()=>Number((await h.api('/v1/installation')).body.eventCursor),before=await cursor();
  await h.api('/v1/pairing/expiry',{expiresAfterDays:7},phone.deviceToken);assert.equal(await cursor(),before+1);
  await h.api('/v1/pairing/expiry',{expiresAfterDays:null},phone.deviceToken);assert.equal(await cursor(),before+2);
  await h.api('/v1/pairing/expiry',{expiresAfterDays:null},phone.deviceToken);assert.equal(await cursor(),before+2);
});

test('a committed phone pairing can confirm its exact receipt while the host is stopped or expired',async t=>{
  const h=await fixture(t),offer=h.invitation();await h.api('/v1/pairing/invitation',offer);
  const phone=h.phone(offer),joined=await h.api('/v1/devices/join',phone,'');
  assert.equal((await h.api('/v1/pairing/expiry',{expiresAfterDays:1})).status,200);
  assert.equal((await h.api('/v1/host/stop',{})).status,200);h.advance(2*86_400_000);
  assert.deepEqual(await h.api('/v1/devices/join',phone,''),joined);
  assert.equal(h.stops,1);assert.equal(h.restarts,0);
});

test('exact prepared invitation and join retries survive lost responses, expiry and restart',async t=>{
  const h=await fixture(t),offer=h.invitation();
  const first=await h.api('/v1/pairing/invitation',offer);h.advance(1000);
  assert.deepEqual(await h.api('/v1/pairing/invitation',offer),first);
  const phone=h.phone(offer),joined=await h.api('/v1/devices/join',phone,'');assert.equal(joined.status,201);
  h.advance(300_000);await h.reopen();
  assert.deepEqual(await h.api('/v1/devices/join',phone,''),joined);
  for(const change of [{deviceToken:secret()},{deviceId:randomUUID()},{deviceName:'Different'},{invitationId:randomUUID()},{code:secret()}]) {
    assert.ok([401,409].includes((await h.api('/v1/devices/join',{...phone,...change},'')).status));
  }
  assert.equal((await h.api('/v1/agents',undefined,phone.deviceToken)).status,200);
  assert.equal((await h.api('/v1/pairing/invitation',h.invitation())).status,409);
  assert.equal(h.service.inspectSession().deviceIDs.length,2);
});

test('companion cannot grant access, change the runtime or stop the owning host',async t=>{
  const h=await fixture(t),offer=h.invitation();await h.api('/v1/pairing/invitation',offer);
  const phone=h.phone(offer);await h.api('/v1/devices/join',phone,'');
  for(const [path,body] of [['/v1/pairing/invitation',h.invitation()],['/v1/runtime/restart',{recoveryEpoch:h.service.installation.recoveryEpoch}],['/v1/host/stop',{}],['/v1/pairing/disconnect',{}]] as const) {
    assert.equal((await h.api(path,body,phone.deviceToken)).status,403);
  }
  assert.equal(h.stops,0);assert.equal(h.restarts,0);
  assert.equal((await h.api('/v1/agents',undefined,h.host.deviceToken)).status,200);
});

test('companion leave revokes only its exact credential and returns only its same receipt on retry',async t=>{
  const h=await fixture(t),offer=h.invitation();await h.api('/v1/pairing/invitation',offer);
  const phone=h.phone(offer);await h.api('/v1/devices/join',phone,'');
  const body={deviceId:phone.deviceId},receipt=await h.api('/v1/pairing/leave',body,phone.deviceToken);
  assert.equal(receipt.status,200);assert.deepEqual(receipt.body,{installationId:h.service.installation.installationId,deviceId:phone.deviceId,revoked:true,scope:'companion-only'});
  assert.equal(h.stops,0);assert.equal((await h.api('/v1/agents',undefined,phone.deviceToken)).status,401);
  assert.equal((await h.api('/v1/agents')).status,200);
  assert.deepEqual(await h.api('/v1/pairing/leave',body,phone.deviceToken),receipt);
  await h.reopen();assert.deepEqual(await h.api('/v1/pairing/leave',body,phone.deviceToken),receipt);
  assert.equal((await h.api('/v1/pairing/leave',body,secret())).status,401);
  assert.equal((await h.api('/v1/pairing/leave',{deviceId:h.host.deviceId},phone.deviceToken)).status,401);
  assert.equal((await h.api('/v1/pairing/leave',body,phone.deviceToken,'another-installation')).status,401);
  assert.equal((await h.api('/v1/devices/join',phone,'')).status,401);
});

test('failed whole-host disconnect retains both credentials until stop is confirmed',async t=>{
  const h=await fixture(t),offer=h.invitation();await h.api('/v1/pairing/invitation',offer);
  const phone=h.phone(offer);await h.api('/v1/devices/join',phone,'');
  h.failStop(true);assert.equal((await h.api('/v1/pairing/disconnect',{})).status,503);
  assert.equal(h.service.inspectSession().deviceIDs.length,2);
  assert.equal((await h.api('/v1/agents',undefined,phone.deviceToken)).status,200);
  h.failStop(false);assert.equal((await h.api('/v1/pairing/disconnect',{})).status,200);
  assert.deepEqual(h.service.inspectSession().deviceIDs,[]);
  assert.equal((await h.api('/v1/agents',undefined,phone.deviceToken)).status,401);
  assert.equal((await h.api('/v1/agents')).status,401);
});

test('an expired Mac is never replaced by its still-active companion during local setup',async t=>{
  const h=await fixture(t),offer=h.invitation();await h.api('/v1/pairing/invitation',offer);
  const phone=h.phone(offer);await h.api('/v1/devices/join',phone,'');
  assert.equal((await h.api('/v1/pairing/expiry',{expiresAfterDays:1})).status,200);h.advance(2*86_400_000);
  assert.equal((await h.api('/v1/agents')).status,401);
  assert.equal((await h.api('/v1/agents',undefined,phone.deviceToken)).status,200);
  assert.throws(()=>h.service.beginOwnerSetup(),/recovery/);
});

for(const path of ['local-setup','legacy-enrollment'] as const)test(`replacing two expired devices clears the retired companion slot through ${path}`,async t=>{
  const h=await fixture(t),offer=h.invitation();await h.api('/v1/pairing/expiry',{expiresAfterDays:1});await h.api('/v1/pairing/invitation',offer);
  const phone=h.phone(offer);await h.api('/v1/devices/join',phone,'');h.advance(2*86_400_000);
  let replacement:{deviceId:string;deviceToken:string};
  if(path==='local-setup'){
    const setup=h.service.beginOwnerSetup();assert.equal(setup.kind,'available');if(setup.kind!=='available')throw new Error('Expected new owner setup');
    replacement={deviceId:randomUUID(),deviceToken:secret()};
    await h.service.completeOwnerSetup({...replacement,installationId:setup.installationId,setupId:setup.setupId});
  }else{
    const result=await h.api<{deviceId:string;deviceToken:string}>('/v1/devices/enroll',{...h.service.issueEnrollment(),deviceName:'Replacement Mac'},'');
    assert.equal(result.status,201);replacement=result.body;
  }
  assert.equal((await h.api('/v1/agents',undefined,phone.deviceToken)).status,401);
  assert.equal((await h.api('/v1/agents')).status,401);
  assert.equal((await h.api('/v1/pairing/invitation',h.invitation(),replacement.deviceToken)).status,201);
});

test('the host can revoke the exact companion while stopped without changing its own pairing',async t=>{
  const h=await fixture(t),offer=h.invitation();await h.api('/v1/pairing/invitation',offer);
  const phone=h.phone(offer);await h.api('/v1/devices/join',phone,'');
  assert.equal((await h.api('/v1/host/stop',{})).status,200);
  const result=await h.api('/v1/pairing/revoke-companion',{deviceId:phone.deviceId});
  assert.equal(result.status,200);assert.equal(result.body.deviceId,phone.deviceId);
  assert.equal(result.body.scope,'companion-only');
  assert.deepEqual(await h.api('/v1/pairing/revoke-companion',{deviceId:phone.deviceId}),result);
  assert.equal((await h.api('/v1/agents',undefined,phone.deviceToken)).status,401);
  assert.equal((await h.api('/v1/agents')).status,200);
  assert.equal((await h.api('/v1/pairing/revoke-companion',{deviceId:h.host.deviceId})).status,409);
});

test('phone expiry is independent of the host and the next explicitly invited phone',async t=>{
  const h=await fixture(t),offer=h.invitation();await h.api('/v1/pairing/invitation',offer);
  const phone=h.phone(offer);await h.api('/v1/devices/join',phone,'');
  assert.equal((await h.api('/v1/pairing/expiry',{expiresAfterDays:1},phone.deviceToken)).status,200);
  assert.equal((await h.api('/v1/pairing')).body.expiresAt,null);h.advance(2*86_400_000);
  assert.equal((await h.api('/v1/agents',undefined,phone.deviceToken)).status,401);
  assert.equal((await h.api('/v1/pairing/invitation',h.invitation())).status,409);
  assert.equal((await h.api('/v1/pairing/revoke-companion',{deviceId:phone.deviceId})).status,200);
  const next=h.invitation();assert.equal((await h.api('/v1/pairing/invitation',next)).status,201);
  const joined=await h.api('/v1/devices/join',h.phone(next),'');assert.equal(joined.status,201);assert.equal(joined.body.expiresAt,null);
});

test('invalid, replaced and expired invitations never consume a valid prepared pairing',async t=>{
  const h=await fixture(t),old=h.invitation();assert.equal((await h.api('/v1/pairing/invitation',old)).status,201);
  assert.equal((await h.api('/v1/pairing/invitation',{...old,code:secret()})).status,409);
  const offer=h.invitation();assert.equal((await h.api('/v1/pairing/invitation',offer)).status,201);
  assert.equal((await h.api('/v1/devices/join',h.phone(old),'')).status,401);
  const phone=h.phone(offer);
  for(const change of [{installationId:'wrong'},{invitationId:'wrong'},{code:secret()},{deviceId:h.host.deviceId},{deviceToken:h.host.deviceToken},{deviceToken:offer.code},{deviceToken:'short'},{deviceName:''},{deviceName:'x'.repeat(129)},{unexpected:true}]) {
    assert.ok([400,401,409].includes((await h.api('/v1/devices/join',{...phone,...change},'')).status));
  }
  assert.equal(h.service.inspectSession().deviceIDs.length,1);
  const browser=await fetch(h.service.baseURL+'/v1/devices/join',{method:'POST',headers:{'Content-Type':'application/json','X-Fort-Installation-Id':phone.installationId,Origin:'https://untrusted.invalid'},body:JSON.stringify(phone)});
  assert.equal(browser.status,401);await browser.arrayBuffer();
  h.advance(300_001);assert.equal((await h.api('/v1/devices/join',phone,'')).status,401);
  assert.equal((await h.api('/v1/pairing/invitation',offer)).status,409);
  const fresh=h.invitation();assert.equal((await h.api('/v1/pairing/invitation',fresh)).status,201);
  assert.equal((await h.api('/v1/devices/join',h.phone(fresh),'')).status,201);
});

test('a replaced invitation cannot revive or revoke a previously disconnected phone',async t=>{
  const h=await fixture(t),first=h.invitation();await h.api('/v1/pairing/invitation',first);
  const old=h.phone(first);await h.api('/v1/devices/join',old,'');await h.api('/v1/pairing/leave',{deviceId:old.deviceId},old.deviceToken);
  const second=h.invitation();await h.api('/v1/pairing/invitation',second);
  const current=h.phone(second);assert.equal((await h.api('/v1/devices/join',current,'')).status,201);
  assert.equal((await h.api('/v1/devices/join',old,'')).status,401);
  assert.equal((await h.api('/v1/pairing/leave',{deviceId:old.deviceId},old.deviceToken)).status,401);
  assert.equal((await h.api('/v1/pairing/revoke-companion',{deviceId:old.deviceId})).status,409);
  assert.equal((await h.api('/v1/agents',undefined,current.deviceToken)).status,200);
});

test('the persisted grant contains digests only and conflicting concurrent joins admit one phone',async t=>{
  const h=await fixture(t),offer=h.invitation();await h.api('/v1/pairing/invitation',offer);
  const first=h.phone(offer),second=h.phone(offer);
  const results=await Promise.all([h.api('/v1/devices/join',first,''),h.api('/v1/devices/join',second,'')]);
  assert.deepEqual(results.map(value=>value.status).sort(),[201,409]);
  assert.equal(h.service.inspectSession().deviceIDs.length,2);await h.service.close();
  const db=new DatabaseSync(join(h.directory,'openai-candidate-b.sqlite'),{readOnly:true});
  try {
    const persisted=JSON.stringify([db.prepare('SELECT * FROM metadata').all(),db.prepare('SELECT * FROM devices').all()]);
    for(const secret of [offer.code,h.host.deviceToken,first.deviceToken,second.deviceToken])assert.equal(persisted.includes(secret),false);
  }finally{db.close();}
});

test('stopped hosts and legacy temporary sessions cannot issue companion grants',async t=>{
  const h=await fixture(t),offer=h.invitation();await h.api('/v1/pairing/invitation',offer);
  assert.equal((await h.api('/v1/host/stop',{})).status,200);
  assert.equal((await h.api('/v1/devices/join',h.phone(offer),'')).status,409);
  assert.equal((await h.api('/v1/pairing/invitation',h.invitation())).status,503);
  const directory=await realpath(await mkdtemp(join(tmpdir(),'fort-no-companion-'))),service=createApplication({provider:CODEX_RUNTIME,dataDirectory:directory});
  await service.start();t.after(async()=>{await service.close();await rm(directory,{recursive:true,force:true});});
  const response=await fetch(service.baseURL+'/v1/devices/enroll',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...service.issueEnrollment(),deviceName:'Temporary fixture'})});
  const enrolled=await response.json() as {deviceToken:string};
  const denied=await fetch(service.baseURL+'/v1/pairing/invitation',{method:'POST',headers:{'Content-Type':'application/json','X-Fort-Installation-Id':service.installation.installationId,Authorization:'Bearer '+enrolled.deviceToken},body:JSON.stringify(offer)});
  assert.equal(denied.status,403);await denied.arrayBuffer();
  assert.equal(service.installation.capabilities?.includes('companionPairing'),false);
});

test('corrupt companion metadata blocks phone authority while preserving exact Mac recovery',async t=>{
  const h=await fixture(t),offer=h.invitation();await h.api('/v1/pairing/invitation',offer);
  const phone=h.phone(offer);await h.api('/v1/devices/join',phone,'');
  await h.service.close();
  const db=new DatabaseSync(join(h.directory,'openai-candidate-b.sqlite'));
  try{db.prepare("UPDATE metadata SET data=? WHERE key='companionPairing'").run('unreadable synthetic PRIVATE_VALUE');}finally{db.close();}
  await h.reopen();
  const denied=await h.api('/v1/agents',undefined,phone.deviceToken);assert.equal(denied.status,409);
  assert.equal(JSON.stringify(denied).includes('PRIVATE_VALUE'),false);
  assert.equal((await h.api('/v1/pairing')).status,200);
  assert.equal(h.service.beginOwnerSetup().kind,'paired');
  assert.equal((await h.api('/v1/pairing/disconnect',{})).status,200);
  assert.deepEqual(h.service.inspectSession().deviceIDs,[]);
});

test('whole-host revocation clears companion metadata before separately confirmed offline history deletion',async t=>{
  const h=await fixture(t),offer=h.invitation();await h.api('/v1/pairing/invitation',offer);
  const phone=h.phone(offer);await h.api('/v1/devices/join',phone,'');
  const request={directory:h.directory,installationId:h.service.installation.installationId,recoveryEpoch:h.service.installation.recoveryEpoch,deviceId:h.host.deviceId,deletionId:randomUUID()};
  assert.equal((await h.api('/v1/pairing/disconnect',{})).status,200);await h.service.close();
  const erased=deleteOwnerHistory(request);assert.ok(erased.recoveryEpoch);assert.deepEqual(deleteOwnerHistory(request),erased);
});

test('an offline ledger copy preserves exact companion membership and retry identity',async t=>{
  const h=await fixture(t),offer=h.invitation();await h.api('/v1/pairing/invitation',offer);
  const phone=h.phone(offer),joined=await h.api('/v1/devices/join',phone,'');await h.service.close();
  const destination=join(h.directory,'migrated');await migrateLedger({source:join(h.directory,'openai-candidate-b.sqlite'),destination});
  const copy=createApplication({provider:CODEX_RUNTIME,dataDirectory:destination});await copy.start();t.after(()=>copy.close());
  const response=await fetch(copy.baseURL+'/v1/devices/join',{method:'POST',headers:{'Content-Type':'application/json','X-Fort-Installation-Id':phone.installationId},body:JSON.stringify(phone)});
  assert.equal(response.status,201);assert.deepEqual(await response.json(),joined.body);
  assert.equal(copy.beginOwnerSetup().kind,'paired');
  await copy.close();
});

test('twenty isolated companion pairing cycles meet the local budget',{skip:!process.env.FORT_COMPANION_BENCHMARK},async t=>{
  const destination=process.env.FORT_COMPANION_BENCHMARK;assert.ok(destination);
  const h=await fixture(t),samples:number[]=[];
  for(let index=0;index<25;index++) {
    const offer=h.invitation(),phone=h.phone(offer),started=performance.now();
    assert.equal((await h.api('/v1/pairing/invitation',offer)).status,201);
    assert.equal((await h.api('/v1/devices/join',phone,'')).status,201);
    assert.equal((await h.api('/v1/pairing/revoke-companion',{deviceId:phone.deviceId})).status,200);
    const elapsed=performance.now()-started;if(index>=5)samples.push(elapsed);
  }
  const sorted=[...samples].sort((a,b)=>a-b);
  await writeFile(destination,JSON.stringify({node:process.version,warmups:5,samplesMs:samples,p50Ms:sorted[9],p95Ms:sorted[18],maximumMs:sorted[19]},null,2)+'\n',{flag:'wx'});
  assert.ok(sorted[18]<25);
});
