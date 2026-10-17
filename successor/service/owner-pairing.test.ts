import {test,type TestContext} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {createOpenAIService} from './openai.ts';
import type {PairingStatus,Agent,Snapshot} from '../contracts/v1.ts';
async function setup(t: TestContext, options: Omit<Parameters<typeof createOpenAIService>[0],'dataDirectory'> = {}) {
  const directory = await mkdtemp(join(tmpdir(), 'fort-rebuilt-unit-'));
  let service = createOpenAIService({ dataDirectory: directory, ...options });
  await service.start();
  t.after(async () => { await service.close(); await rm(directory, { recursive: true, force: true }); });
  const enrollment = service.issueEnrollment();
  const response = await fetch(service.baseURL + '/v1/devices/enroll', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...enrollment, deviceName: 'Synthetic test device' }) });
  assert.equal(response.status, 201);
  const device = await response.json() as { deviceToken: string };
  const headers = { 'Content-Type': 'application/json', Authorization: `Bearer ${device.deviceToken}`, 'X-Fort-Installation-Id': service.installation.installationId };
  async function api<T>(path: string, body?: unknown) {
    const response = await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers, body: body === undefined ? undefined : JSON.stringify(body) });
    return { status: response.status, body: await response.json() as T };
  }
  async function reenroll() {
    const response=await fetch(service.baseURL+'/v1/devices/enroll',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...service.issueEnrollment(),deviceName:'Fresh synthetic test device'})});
    assert.equal(response.status,201);const device=await response.json() as {deviceToken:string};headers.Authorization=`Bearer ${device.deviceToken}`;
  }
  return { api, reenroll, directory, headers, get service() { return service; }, async restart() { await service.close(); service = createOpenAIService({ dataDirectory: directory, ...options }); await service.start(); } };
}



test('explicit owner setup has no default expiry, optional expiry persists, and demo limits cannot be expanded',async t=>{
 let now=Date.parse('2026-10-18T00:00:00Z');
 const h=await setup(t,{now:()=>now,ownerPairingPolicy:{expiresAfterDays:null}});
 assert.equal((await h.api<PairingStatus>('/v1/pairing')).body.expiresAt,null);
 now+=400*86400000;await h.restart();assert.equal((await h.api('/v1/agents')).status,200);
 const changed=await h.api<PairingStatus>('/v1/pairing/expiry',{expiresAfterDays:7});assert.equal(changed.status,200);assert.equal(changed.body.expiresAt,new Date(now+7*86400000).toISOString());
 await h.restart();assert.deepEqual((await h.api<PairingStatus>('/v1/pairing')).body,changed.body);
 assert.equal((await h.api('/v1/pairing/expiry',{expiresAfterDays:0})).status,400);
 now+=8*86400000;assert.equal((await h.api('/v1/agents')).status,401);
 const demo=await setup(t,{credentialDeadline:Date.now()+600000});assert.equal((await demo.api('/v1/pairing/expiry',{expiresAfterDays:null})).status,403);
});
test('disconnect confirms runtime stop then revokes pairing while retaining agents and history',async t=>{
 let stopped=false;const h=await setup(t,{ownerPairingPolicy:{expiresAfterDays:null},runtime:{async execute(){return {kind:'completed',text:'Saved fixture reply',effectiveModel:null};},async dispose(){stopped=true;}}});
 const agent=(await h.api<Agent>('/v1/agents',{name:'Keep me',instructions:'Saved identity',model:'exact'})).body;
 await h.api(`/v1/conversations/${agent.homeConversationId}/messages`,{requestId:'retained',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,text:'Saved fixture message',skills:[]});
 await h.service.dispatchNext();
 const before=(await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body;
 const response=await h.api<{revoked:boolean;runtimeStopped:boolean}>('/v1/pairing/disconnect',{});
 assert.equal(response.status,200);assert.equal(stopped,true);assert.equal(response.body.revoked,true);assert.equal(response.body.runtimeStopped,true);
 await h.service.close({preserveSessionCredential:true});await h.restart();assert.equal((await h.api('/v1/agents')).status,401);
 await h.reenroll();assert.deepEqual((await h.api<{agents:Agent[]}>('/v1/agents')).body.agents,[agent]);
 assert.deepEqual((await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body,before);
});
test('stop preserves pairing and does not claim success when runtime shutdown is unconfirmed',async t=>{
 const h=await setup(t,{ownerPairingPolicy:{expiresAfterDays:null}});assert.equal((await h.api('/v1/host/stop',{})).status,200);
 await h.service.close({preserveSessionCredential:true});await h.restart();assert.equal((await h.api('/v1/agents')).status,200);
 let fail=true;const failed=await setup(t,{ownerPairingPolicy:{expiresAfterDays:null},runtime:{async execute(){return {kind:'notStarted',message:'fixture'};},async dispose(){if(fail)throw new Error('SECRET');}}});
 const response=await failed.api<{error:{message:string}}>('/v1/pairing/disconnect',{});assert.equal(response.status,503);assert.equal(JSON.stringify(response).includes('SECRET'),false);assert.equal(failed.service.inspectSession().deviceIDs.length,1);fail=false;
});

test('owner pairing cannot adopt existing or bounded ledgers and admits only one paired device',async t=>{
 const h=await setup(t);await h.service.close();
 assert.throws(()=>createOpenAIService({dataDirectory:h.directory,ownerPairingPolicy:{expiresAfterDays:null}}),/fresh explicitly/);
 await h.restart();assert.equal((await h.api('/v1/agents')).status,200);
 const demo=await setup(t,{credentialDeadline:Date.now()+600000});await demo.service.close();
 assert.throws(()=>createOpenAIService({dataDirectory:demo.directory,ownerPairingPolicy:{expiresAfterDays:null}}),/fresh explicitly/);
 const owner=await setup(t,{ownerPairingPolicy:{expiresAfterDays:null}});
 assert.throws(()=>owner.service.issueEnrollment(),/Disconnect/);
 for(const path of ['/v1/pairing/expiry','/v1/pairing/disconnect','/v1/host/stop']){
   const response=await fetch(owner.service.baseURL+path,{method:'POST',headers:{...owner.headers,Origin:'https://untrusted.invalid'},body:'{}'});
   assert.equal(response.status,403);
 }
 assert.equal((await owner.api('/v1/pairing')).status,200);
});
test('disconnect retains pairing until the owned runtime confirms shutdown',async t=>{
 let confirm!:()=>void;let stopping!:()=>void;
 const stoppingSignal=new Promise<void>(resolve=>{stopping=resolve;});
 const stopped=new Promise<void>(resolve=>{confirm=resolve;});
 const h=await setup(t,{ownerPairingPolicy:{expiresAfterDays:null},runtime:{async execute(){return {kind:'notStarted',message:'fixture'};},async dispose(){stopping();await stopped;}}});
 const response=h.api('/v1/pairing/disconnect',{});await stoppingSignal;
 assert.equal(h.service.inspectSession().deviceIDs.length,1);
 assert.equal((await h.api('/v1/pairing/disconnect',{})).status,409);
 confirm();assert.equal((await response).status,200);
});
test('failed disposal leaves management reachable for an explicit disconnect retry but blocks new work',async t=>{
 let attempts=0;const h=await setup(t,{ownerPairingPolicy:{expiresAfterDays:null},runtime:{async execute(){return {kind:'notStarted',message:'fixture'};},async dispose(){if(++attempts===1)throw new Error('synthetic disposal failure');}}});
 assert.equal((await h.api('/v1/pairing/disconnect',{})).status,503);
 assert.equal((await h.api('/v1/pairing')).status,200);
 assert.equal((await h.api('/v1/agents',{name:'Must not create',instructions:'Fixture',model:'exact'})).status,503);
 assert.equal((await h.api('/v1/pairing/disconnect',{})).status,200);
 assert.equal(attempts,2);
});
