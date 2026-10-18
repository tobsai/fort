import type {DatabaseSync} from 'node:sqlite';
import {createHash} from 'node:crypto';
import type {CompanionInvitation,CompanionReceipt,CompanionRevocation,CompanionOverview} from '../contracts/v1.ts';

export class CompanionPairingError extends Error {
  readonly status:number;readonly code:string;
  constructor(status:number,code:string,message:string){super(message);this.status=status;this.code=code;}
}
function fail(status:number,code:string,message:string):never {throw new CompanionPairingError(status,code,message);}
const hash=(value:string)=>createHash('sha256').update(value).digest('hex');
function object(value:unknown,keys:readonly string[]):Record<string,unknown> {
  if(!value || typeof value!=='object' || Array.isArray(value) || Object.keys(value).some(key=>!keys.includes(key)))fail(400,'invalidPairing','Use the exact pairing fields.');
  return value as Record<string,unknown>;
}
function identifier(value:unknown):string {
  if(typeof value!=='string' || !/^[A-Za-z0-9_-]{1,128}$/.test(value))fail(400,'invalidPairing','Use an exact pairing identifier.');
  return value;
}
function token(value:unknown):string {
  if(typeof value!=='string' || !/^[A-Za-z0-9_-]{43}$/.test(value) || Buffer.from(value,'base64url').toString('base64url')!==value)fail(400,'invalidPairing','Use a newly prepared 32-byte pairing value.');
  return value;
}
type Host={deviceId:string;expiresAfterDays:number|null;running:boolean};
type Base={version:1;invitationId:string;codeHash:string;hostId:string;expires:number};
type State=Base & ({state:'offered'}|{state:'paired'|'revoked';deviceId:string;tokenHash:string;deviceName:string});

export function companionPairing(db:DatabaseSync,installationId:string,now:()=>number,currentHost:()=>Host|undefined,didChange:()=>void) {
  const read=():State|undefined=>{
    const row=db.prepare("SELECT data FROM metadata WHERE key='companionPairing'").get();
    if(!row)return;
    try {
      if(typeof row.data!=='string' || row.data.length>4096)throw new Error();
      const value=object(JSON.parse(row.data),['version','invitationId','codeHash','hostId','expires','state','deviceId','tokenHash','deviceName']);
      if(value.version!==1 || !Number.isSafeInteger(value.expires) || Number(value.expires)<=0 || typeof value.codeHash!=='string' || !/^[a-f0-9]{64}$/.test(value.codeHash))throw new Error();
      const base:Base={version:1,invitationId:identifier(value.invitationId),codeHash:value.codeHash,hostId:identifier(value.hostId),expires:Number(value.expires)};
      if(value.state==='offered')return {...base,state:'offered'};
      if((value.state!=='paired' && value.state!=='revoked') || typeof value.tokenHash!=='string' || !/^[a-f0-9]{64}$/.test(value.tokenHash) || typeof value.deviceName!=='string' || !value.deviceName.isWellFormed() || !value.deviceName.trim() || Buffer.byteLength(value.deviceName)>128)throw new Error();
      return {...base,state:value.state,deviceId:identifier(value.deviceId),tokenHash:value.tokenHash,deviceName:value.deviceName};
    } catch {fail(409,'pairingState','The saved companion pairing needs local recovery.');}
  };
  const save=(state:State)=>{db.prepare("INSERT OR REPLACE INTO metadata VALUES ('companionPairing',?)").run(JSON.stringify(state));didChange();};
  const host=(deviceId?:string)=>{
    const owner=currentHost();
    if(!owner || deviceId!==undefined && deviceId!==owner.deviceId)fail(403,'hostOnly','Use the owning Mac to manage companion access.');
    return owner;
  };
  const available=(owner:Host)=>{
    if(!owner.running || !db.prepare('SELECT 1 FROM devices WHERE id=? AND (expires=0 OR expires>?)').get(owner.deviceId,now()))fail(409,'hostUnavailable','Pairing requires the active owning Mac.');
  };
  const receipt=(state:Extract<State,{state:'paired'|'revoked'}>):CompanionReceipt=>{
    const device=db.prepare('SELECT expires FROM devices WHERE id=? AND hash=?').get(state.deviceId,state.tokenHash);
    if(!device || Number(device.expires)!==0 && Number(device.expires)<=now())fail(401,'pairingInactive','This companion pairing is no longer active.');
    return {installationId,deviceId:state.deviceId,expiresAt:device.expires===0?null:new Date(Number(device.expires)).toISOString(),role:'companion'};
  };
  const revoke=(state:Extract<State,{state:'paired'|'revoked'}>):CompanionRevocation=>{
    const device=db.prepare('SELECT hash FROM devices WHERE id=?').get(state.deviceId);
    if(device && (device.hash!==state.tokenHash || state.state==='revoked') || !device && state.state!=='revoked')fail(409,'pairingChanged','The saved companion identity changed.');
    if(state.state!=='revoked'){
      db.prepare('DELETE FROM devices WHERE id=? AND hash=?').run(state.deviceId,state.tokenHash);
      save({...state,state:'revoked'});
    }
    return {installationId,deviceId:state.deviceId,revoked:true,scope:'companion-only'};
  };
  return {
    clear(){if(db.prepare("DELETE FROM metadata WHERE key='companionPairing'").run().changes)didChange();},
    isCompanion(deviceId:string):boolean {const state=read();return state?.state==='paired' && state.deviceId===deviceId && state.hostId===currentHost()?.deviceId;},
    overview(deviceId:string):CompanionOverview {
      const owner=host(deviceId),state=read(),base={installationId,hostDeviceId:owner.deviceId};
      if(!state || state.hostId!==owner.deviceId || state.state==='revoked' || state.state==='offered' && state.expires<=now())return {...base,pairing:{state:'available'}};
      if(state.state==='offered')return {...base,pairing:{state:'invited',invitationId:state.invitationId,expiresAt:new Date(state.expires).toISOString()}};
      const device=db.prepare('SELECT expires FROM devices WHERE id=? AND hash=?').get(state.deviceId,state.tokenHash);
      if(!device)fail(409,'pairingChanged','The saved companion identity changed.');
      return {...base,pairing:{state:'paired',deviceId:state.deviceId,deviceName:state.deviceName,expiresAt:device.expires===0?null:new Date(Number(device.expires)).toISOString(),active:device.expires===0 || Number(device.expires)>now()}};
    },
    revokeCompanion(deviceId:string,input:unknown):CompanionRevocation {
      const owner=host(deviceId),value=object(input,['deviceId']),target=identifier(value.deviceId),state=read();
      if(!state || state.state==='offered' || state.hostId!==owner.deviceId || state.deviceId!==target)fail(409,'pairingChanged','Select the exact current companion.');
      return revoke(state);
    },
    invite(deviceId:string,input:unknown):CompanionInvitation {
      const owner=host(deviceId);available(owner);
      const value=object(input,['invitationId','code']),invitationId=identifier(value.invitationId),codeHash=hash(token(value.code)),prior=read();
      if(prior?.state==='paired' || db.prepare('SELECT 1 FROM devices WHERE id<>?').get(owner.deviceId))fail(409,'companionPaired','Revoke the existing companion before inviting another.');
      if(prior?.invitationId===invitationId){
        if(prior.hostId!==owner.deviceId || prior.codeHash!==codeHash || prior.state!=='offered')fail(409,'invitationChanged','This invitation cannot be replaced by a retry.');
        if(prior.expires<=now())fail(409,'invitationExpired','Prepare a new invitation.');
        return {installationId,invitationId,expiresAt:new Date(prior.expires).toISOString()};
      }
      if(prior?.codeHash===codeHash)fail(409,'invitationReused','Prepare a new invitation code.');
      const state:State={version:1,state:'offered',invitationId,codeHash,hostId:owner.deviceId,expires:now()+300_000};save(state);
      return {installationId,invitationId,expiresAt:new Date(state.expires).toISOString()};
    },
    leave(input:unknown,credential:string):CompanionRevocation {
      const value=object(input,['deviceId']),deviceId=identifier(value.deviceId),tokenHash=hash(token(credential)),state=read();
      if(!state || state.state==='offered' || state.hostId!==currentHost()?.deviceId || state.deviceId!==deviceId || state.tokenHash!==tokenHash)fail(401,'unauthorized','This exact companion credential is required.');
      return revoke(state);
    },
    join(input:unknown):CompanionReceipt {
      const value=object(input,['installationId','invitationId','code','deviceId','deviceToken','deviceName']),owner=host();
      if(value.installationId!==installationId)fail(401,'wrongInstallation','Use this exact installation.');
      const invitationId=identifier(value.invitationId),codeHash=hash(token(value.code)),deviceId=identifier(value.deviceId),tokenHash=hash(token(value.deviceToken));
      if(codeHash===tokenHash)fail(400,'credentialReused','Prepare a separate private device credential.');
      if(typeof value.deviceName!=='string' || !value.deviceName.isWellFormed() || !value.deviceName.trim() || Buffer.byteLength(value.deviceName)>128)fail(400,'invalidPairing','Use a device name within 128 bytes.');
      const state=read();
      if(!state || state.hostId!==owner.deviceId || state.invitationId!==invitationId || state.codeHash!==codeHash || deviceId===owner.deviceId)fail(401,'invitationDenied','This invitation does not match.');
      if(state.state==='paired'){
        if(state.deviceId!==deviceId || state.tokenHash!==tokenHash || state.deviceName!==value.deviceName)fail(409,'pairingChanged','Retry only the exact prepared companion pairing.');
        return receipt(state);
      }
      available(owner);
      if(state.state!=='offered' || state.expires<=now())fail(401,'invitationDenied','This invitation is expired or consumed.');
      if(db.prepare('SELECT 1 FROM devices WHERE id<>? OR hash=?').get(owner.deviceId,tokenHash))fail(409,'companionPaired','Revoke the existing companion before joining another.');
      const expires=owner.expiresAfterDays===null?0:now()+owner.expiresAfterDays*86_400_000;
      db.prepare('INSERT INTO devices VALUES (?,?,?)').run(deviceId,tokenHash,expires);
      const paired:State={...state,state:'paired',deviceId,tokenHash,deviceName:value.deviceName};save(paired);return receipt(paired);
    }
  };
}
