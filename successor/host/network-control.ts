import {createHash,randomUUID} from 'node:crypto';
import type {NetworkCommand,NetworkConfiguration,NetworkListenerState,NetworkOwner,NetworkStatus,SecureNetworkControl} from '../contracts/secure-network.ts';
import {openSecureNetwork,SecureNetworkOpenError,type SecureNetworkOptions} from './secure-network.ts';

type Network = Awaited<ReturnType<typeof openSecureNetwork>>;
type Phase =
  | Extract<NetworkListenerState,{state:'stopped'}>
  | {state:'starting';abort:AbortController;opening:Promise<Network>}
  | {state:'active';network:Network}
  | {state:'stopping';completion:Promise<void>}
  | {state:'unconfirmed'};
const failure=()=>new Error('The exact local listener operation could not be confirmed');
function object(value:unknown,keys:readonly string[]):Record<string,unknown> {
  if(!value || typeof value!=='object' || Array.isArray(value) || Object.keys(value).length!==keys.length || Object.keys(value).some(key=>!keys.includes(key)))throw failure();
  return value as Record<string,unknown>;
}
function identifier(value:unknown):string {
  if(typeof value!=='string' || !/^[A-Za-z0-9_-]{1,128}$/.test(value))throw failure();
  return value;
}
function integer(value:unknown,minimum:number,maximum:number):number {
  if(typeof value!=='number' || !Number.isSafeInteger(value) || value<minimum || value>maximum)throw failure();
  return value;
}
function text(value:unknown,maximum:number):string {
  if(typeof value!=='string' || !value.length || value.length>maximum || !/^[\x20-\x7e]+$/.test(value))throw failure();
  return value;
}
function encoded(value:unknown):string {
  const input=text(value,87384),bytes=Buffer.from(input,'base64');
  if(!bytes.length || bytes.length>65536 || bytes.toString('base64')!==input)throw failure();
  return input;
}
function configuration(value:unknown):NetworkConfiguration {
  const input=object(value,['bindAddress','prefixLength','port','serverName','certificate','privateKey']);
  return {bindAddress:text(input.bindAddress,15),prefixLength:integer(input.prefixLength,0,32),port:integer(input.port,0,65535),serverName:text(input.serverName,253),certificate:encoded(input.certificate),privateKey:encoded(input.privateKey)};
}
function command(value:unknown):NetworkCommand {
  if(!value || typeof value!=='object' || !('operation' in value))throw failure();
  const keys=value.operation==='inspect'?['operation','owner']:value.operation==='start'?['operation','owner','sessionId','generation','configuration']:['operation','owner','sessionId','generation'];
  const input=object(value,keys),identity=object(input.owner,['installationId','deviceId']);
  const owner={installationId:identifier(identity.installationId),deviceId:identifier(identity.deviceId)};
  if(input.operation==='inspect')return {operation:'inspect',owner};
  const sessionId=identifier(input.sessionId),generation=integer(input.generation,0,Number.MAX_SAFE_INTEGER);
  if(input.operation==='stop')return {operation:'stop',owner,sessionId,generation};
  if(input.operation!=='start')throw failure();
  return {operation:'start',owner,sessionId,generation,configuration:configuration(input.configuration)};
}

export function createSecureNetworkControl(options:{
  endpoint:SecureNetworkOptions['endpoint'];
  authorize:(owner:NetworkOwner,operation:NetworkCommand['operation'])=>boolean;
}):SecureNetworkControl {
  const endpoint={...options.endpoint},authorize=options.authorize,sessionId=randomUUID();
  if(typeof authorize!=='function')throw failure();
  let generation=0,phase:Phase={state:'stopped',cause:'initial'},closed=false;
  let lastStart:{generation:number;fingerprint:string;owner:NetworkOwner}|undefined,closing:Promise<void>|undefined;
  const permitted=(input:NetworkCommand)=>{try{return authorize({...input.owner},input.operation)===true;}catch{return false;}};
  const snapshot=(owner:NetworkOwner):NetworkStatus=>{
    const listener:NetworkListenerState=phase.state==='active'?{state:'active',url:phase.network.url}
      :phase.state==='stopped'?{state:'stopped',cause:phase.cause}:{state:phase.state};
    return {owner:{...owner},sessionId,generation,listener};
  };
  const stop=(cause:'requested'|'startFailed'='requested'):Promise<void>=>{
    if(phase.state==='stopping')return phase.completion;
    if(phase.state==='stopped')return Promise.resolve();
    if(phase.state==='unconfirmed')return Promise.reject(failure());
    let stopped:Promise<void>;
    if(phase.state==='starting') {
      const current=phase;current.abort.abort();
      stopped=current.opening.then(network=>network.close(),error=>{if(!(error instanceof SecureNetworkOpenError) || !error.cleanupConfirmed)throw failure();});
    }else stopped=phase.network.close();
    const completion=stopped.then(()=>{phase={state:'stopped',cause};},()=>{phase={state:'unconfirmed'};throw failure();});
    phase={state:'stopping',completion};
    return completion;
  };
  const settled=async()=>{
    if(phase.state==='starting')await phase.opening.catch(()=>{});
    if(phase.state==='stopping')await phase.completion.catch(()=>{});
  };
  return {
    async execute(value:unknown):Promise<NetworkStatus> {
      if(closed)throw failure();
      const input=command(value);
      if(!permitted(input))throw failure();
      if(phase.state!=='stopped' && lastStart && (lastStart.owner.installationId!==input.owner.installationId || lastStart.owner.deviceId!==input.owner.deviceId))throw failure();
      if(input.operation==='inspect')return snapshot(input.owner);
      if(input.sessionId!==sessionId)throw failure();
      if(input.operation==='stop') {
        if(input.generation!==generation)throw failure();
        await stop().catch(()=>{});return snapshot(input.owner);
      }
      const fingerprint=createHash('sha256').update(JSON.stringify([input.owner,input.configuration])).digest('hex');
      if(lastStart?.generation===input.generation) {
        if(lastStart.fingerprint!==fingerprint)throw failure();
        await settled();return snapshot(input.owner);
      }
      if(input.generation!==generation || generation===Number.MAX_SAFE_INTEGER || phase.state!=='stopped')throw failure();
      generation++;lastStart={generation:input.generation,fingerprint,owner:{...input.owner}};
      const abort=new AbortController();
      const opening=openSecureNetwork({...input.configuration,certificate:Buffer.from(input.configuration.certificate,'base64'),privateKey:Buffer.from(input.configuration.privateKey,'base64'),endpoint,signal:abort.signal});
      phase={state:'starting',abort,opening};
      try {
        const network=await opening;
        if(phase.state==='starting' && phase.opening===opening) {
          phase={state:'active',network};
          void network.closed.then(()=>{
            if(phase.state==='active' && phase.network===network)phase={state:'stopped',cause:'interrupted'};
          },()=>{if(phase.state==='active' && phase.network===network)phase={state:'unconfirmed'};});
          if(!permitted(input))await stop('startFailed').catch(()=>{});
        }
      }catch(error){if(phase.state==='starting' && phase.opening===opening)phase=error instanceof SecureNetworkOpenError && error.cleanupConfirmed?{state:'stopped',cause:'startFailed'}:{state:'unconfirmed'};}
      await settled();return snapshot(input.owner);
    },
    close():Promise<void> {
      if(closing)return closing;
      closed=true;closing=stop();return closing;
    }
  };
}
