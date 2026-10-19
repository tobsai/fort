import {test,type TestContext} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm} from 'node:fs/promises';
import {randomBytes} from 'node:crypto';
import {createApplication} from './openai.ts';

const provider={provider:'owner-access-fixture',name:'Synthetic owner access',runtimeVersion:'1',hostId:'synthetic',session:'stateless' as const,capabilities:['plainText' as const]};
async function fixture(t:TestContext,options:Partial<Parameters<typeof createApplication>[0]>={}) {
  const directory=await mkdtemp('/tmp/fort-owner-access-');
  const service=createApplication({dataDirectory:directory,provider,runtime:{async execute(){return {kind:'notStarted',message:'Synthetic owner access'};},async dispose(){},readiness(){return {state:'ready',canRestart:true,detail:'Synthetic'};},async restart(){}},ownerPairingPolicy:{expiresAfterDays:null},...options});
  t.after(async()=>{await service.close();await rm(directory,{recursive:true,force:true});});
  const offer=service.beginOwnerSetup();assert.equal(offer.kind,'available');
  if(offer.kind!=='available')throw new Error('Missing synthetic offer');
  const owner={installationId:offer.installationId,deviceId:'synthetic-primary'},token=randomBytes(32).toString('base64url');
  return {service,owner,token,async pair(){return service.completeOwnerSetup({...owner,setupId:offer.setupId,deviceToken:token});}};
}

test('local owner access distinguishes the current active, expired, removed and foreign primary',async t=>{
  let now=Date.now();const f=await fixture(t,{ownerPairingPolicy:{expiresAfterDays:1},now:()=>now});
  assert.equal(f.service.localOwnerAccess(f.owner),'foreign');await f.pair();
  assert.equal(f.service.localOwnerAccess(f.owner),'active');
  for(const owner of [{...f.owner,installationId:'foreign'},{...f.owner,deviceId:'foreign'}])assert.equal(f.service.localOwnerAccess(owner),'foreign');
  now+=86_400_000;assert.equal(f.service.localOwnerAccess(f.owner),'inactive');
  await f.service.recoverOwnerPairing(f.owner);assert.equal(f.service.localOwnerAccess(f.owner),'inactive');
  const next=f.service.beginOwnerSetup();assert.equal(next.kind,'available');
  if(next.kind!=='available')throw new Error('Missing replacement offer');
  const replacement={...f.owner,deviceId:'new-primary'};
  await f.service.completeOwnerSetup({...replacement,setupId:next.setupId,deviceToken:randomBytes(32).toString('base64url')});
  assert.equal(f.service.localOwnerAccess(f.owner),'foreign');assert.equal(f.service.localOwnerAccess(replacement),'active');
  await f.service.close();assert.equal(f.service.localOwnerAccess(replacement),'foreign');
});

test('owner access notifications follow committed pairing, expiry and close without polling',async t=>{
  let changes=0,now=Date.now();
  t.mock.timers.enable({apis:['setTimeout']});
  const f=await fixture(t,{ownerPairingPolicy:{expiresAfterDays:1},now:()=>now,ownerAccessChanged:()=>{changes++;}});
  await f.pair();await Promise.resolve();const paired=changes;assert.ok(paired>0);
  assert.equal(f.service.localOwnerAccess(f.owner),'active');
  now+=86_399_999;t.mock.timers.tick(86_399_999);await Promise.resolve();assert.equal(changes,paired);
  now++;t.mock.timers.tick(1);await Promise.resolve();assert.equal(changes,paired+1);assert.equal(f.service.localOwnerAccess(f.owner),'inactive');
  t.mock.timers.tick(86_400_000);await Promise.resolve();assert.equal(changes,paired+1);
  await f.service.close();await Promise.resolve();assert.equal(changes,paired+2);
});
