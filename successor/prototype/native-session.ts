import {existsSync,lstatSync,readFileSync,readdirSync,unlinkSync} from 'node:fs';
import {isAbsolute,join,normalize} from 'node:path';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import type {NativeSession} from './session.ts';

function appHasExited(executable:string):boolean {
  const pattern='^'+executable.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')+'($| )';
  try { execFileSync('/usr/bin/pgrep',['-f',pattern],{stdio:'ignore',timeout:1_000});return false; }
  catch(error) { if(error instanceof Error && 'status' in error && error.status===1)return true;throw new Error('Native process status is unknown'); }
}
async function eraseKeychainCredential(account:string):Promise<boolean> {
  try { execFileSync('/usr/bin/security',['delete-generic-password','-s','Fort.Spec058.Device','-a',account],{stdio:'ignore',timeout:1_500});return true; }
  catch(error) { return error instanceof Error && 'status' in error && error.status===44; }
}
export function createNativeSession(options:{directory:string;appExecutable:string;appExited?:()=>boolean;eraseCredential?:(account:string)=>Promise<boolean>}):NativeSession {
  const {directory,appExecutable}=options;
  if([directory,appExecutable].some(path=>!isAbsolute(path)||normalize(path)!==path||path==='/'||path.includes('\0')))throw new Error('Use exact native paths');
  const appExited=options.appExited??(()=>appHasExited(appExecutable)),erase=options.eraseCredential??eraseKeychainCredential;
  const connectionPath=join(directory,'connection.json');
  const read=(path:string):unknown=>{
    if(lstatSync(directory).isSymbolicLink()||!lstatSync(path).isFile()||lstatSync(path).size>1_000_000)throw new Error('Invalid native state');
    return JSON.parse(readFileSync(path,'utf8'));
  };
  const identity=(installationId:string)=>{
    const value=read(connectionPath);
    if(!value||typeof value!=='object'||!('installationID' in value)||value.installationID!==installationId||!('deviceID' in value)||typeof value.deviceID!=='string'||!/^[A-Za-z0-9_-]{1,128}$/.test(value.deviceID))throw new Error('Invalid native state');
    return {deviceId:value.deviceID};
  };
  return {
    assertFresh(){ if(!appExited()||(existsSync(directory)&&(lstatSync(directory).isSymbolicLink()||readdirSync(directory).length!==0)))throw new Error('Native candidate state is not fresh'); },
    inspect(installationId){
      try {
        identity(installationId);
        const scope=join(directory,createHash('sha256').update(installationId).digest('hex'));
        if(lstatSync(scope).isSymbolicLink())throw new Error('Invalid native state');
        const pending=read(join(scope,'pending.json'));
        if(!Array.isArray(pending))throw new Error('Invalid native state');
        return {appExited:appExited(),pendingCount:pending.length};
      }catch{throw new Error('Actual native state could not be confirmed');}
    },
    async cleanup(installationId,issuedDeviceIDs){
      if(!/^[A-Za-z0-9_-]{1,128}$/.test(installationId))throw new Error('Invalid native state');
      const connection=existsSync(connectionPath)?identity(installationId):undefined;
      const deviceIDs=issuedDeviceIDs??(connection?[connection.deviceId]:[]);
      if(deviceIDs.some(id=>!/^[A-Za-z0-9_-]{1,128}$/.test(id))||(connection&&!deviceIDs.includes(connection.deviceId)))throw new Error('Invalid native state');
      let erased=true;
      for(const deviceId of deviceIDs)if(!await erase(`${installationId}/${deviceId}`))erased=false;
      if(erased&&connection)unlinkSync(connectionPath);
      return erased;
    },
  };
}
