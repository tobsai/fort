import type { ServiceEndpoint } from '../service/openai.ts';
import {lstatSync,realpathSync} from 'node:fs';
import {join} from 'node:path';
import {isIP,type Socket} from 'node:net';
import {X509Certificate,createPrivateKey} from 'node:crypto';
import {createServer} from 'node:https';
import {request,type IncomingMessage} from 'node:http';

export type SecureNetworkOptions = {
  bindAddress:string;
  prefixLength:number;
  port:number;
  serverName:string;
  certificate:Buffer;
  privateKey:Buffer;
  endpoint:Extract<ServiceEndpoint,{kind:'privateSocket'}>;
  signal?:AbortSignal;
};
const failure=()=>new Error('Secure network transport could not be verified');
const requestLimit=1024*1024,responseLimit=8*1024*1024;
function ipv4(value:string):number {
  if(isIP(value)!==4)throw failure();
  return value.split('.').reduce((result,part)=>result*256+Number(part),0);
}
function privateAddress(value:string,prefixLength:number):boolean {
  const parts=value.split('.').map(Number);
  return parts[0]===10 && prefixLength>=8 || parts[0]===172 && parts[1]>=16 && parts[1]<=31 && prefixLength>=12
    || parts[0]===192 && parts[1]===168 && prefixLength>=16
    || parts[0]===100 && parts[1]>=64 && parts[1]<=127 && prefixLength>=10;
}
function route(value:string|undefined):string {
  if(!value || value.length>8192 || !value.startsWith('/v1/') || /[\x00-\x20\x7f#\\]/.test(value))throw failure();
  for(const segment of value.split('?')[0].split('/')) {
    const decoded=decodeURIComponent(segment);
    if(decoded==='.' || decoded==='..' || /[/\\\x00-\x20\x7f]/.test(decoded))throw failure();
  }
  return value;
}
function headers(input:IncomingMessage,authority:string) {
  const seen=new Set<string>();
  for(let i=0;i<input.rawHeaders.length;i+=2) {
    const name=input.rawHeaders[i].toLowerCase();
    if(seen.has(name))throw failure();seen.add(name);
    if(name==='origin' || name==='cookie' || name==='forwarded' || name.startsWith('x-forwarded-') || name==='transfer-encoding' || name==='upgrade' || name==='expect')throw failure();
  }
  if(input.headers.host!==authority || !['GET','POST'].includes(input.method??''))throw failure();
  const installation=input.headers['x-fort-installation-id'],authorization=input.headers.authorization;
  if(typeof installation!=='string' || !/^[A-Za-z0-9_-]{1,128}$/.test(installation) || authorization!==undefined && !/^Bearer [A-Za-z0-9_-]{1,4096}$/.test(authorization))throw failure();
  const length=input.headers['content-length'];
  if(length!==undefined && (!/^(0|[1-9][0-9]*)$/.test(length) || Number(length)>requestLimit))throw failure();
  if(input.method==='GET' && length!==undefined && length!=='0')throw failure();
  if(input.method==='POST' && (length===undefined || input.headers['content-type']!=='application/json'))throw failure();
  return {'Content-Type':'application/json','X-Fort-Installation-Id':installation,...(authorization?{Authorization:authorization}:{})};
}
type SecureNetwork = {readonly url:string;readonly closed:Promise<void>;close():Promise<void>};
export class SecureNetworkOpenError extends Error {
  readonly cleanupConfirmed:boolean;
  constructor(cleanupConfirmed:boolean){super('Secure network transport could not be verified');this.cleanupConfirmed=cleanupConfirmed;}
}
class ShutdownUnconfirmed extends Error {
  constructor(){super('Secure network transport could not be verified');}
}
export async function openSecureNetwork(options:SecureNetworkOptions):Promise<SecureNetwork> {
  try {return await startSecureNetwork(options);}
  catch(error){throw new SecureNetworkOpenError(!(error instanceof ShutdownUnconfirmed));}
}
async function startSecureNetwork(options:SecureNetworkOptions):Promise<SecureNetwork> {
  const {bindAddress,prefixLength,port,serverName,signal}=options,endpoint={...options.endpoint};
  signal?.throwIfAborted();
  const address=ipv4(bindAddress),loopback=bindAddress==='127.0.0.1';
  if(!loopback && !privateAddress(bindAddress,prefixLength) || !Number.isInteger(prefixLength) || (loopback?prefixLength!==32:prefixLength<8 || prefixLength>30)
    || !Number.isInteger(port) || (port===0?!loopback:port<1024 || port>65535)
    || !/^[a-z0-9][a-z0-9.-]{0,252}$/.test(serverName) || serverName.includes('..'))throw failure();
  const subnetSize=2**(32-prefixLength),subnet=Math.floor(address/subnetSize);
  if(!loopback && (address%subnetSize===0 || address%subnetSize===subnetSize-1))throw failure();
  if(!Buffer.isBuffer(options.certificate) || !Buffer.isBuffer(options.privateKey) || !options.certificate.length || !options.privateKey.length
    || options.certificate.length>65536 || options.privateKey.length>65536)throw failure();
  const certificate=Buffer.from(options.certificate),privateKey=Buffer.from(options.privateKey);
  try {
    const cert=new X509Certificate(certificate),key=createPrivateKey(privateKey),now=Date.now();
    const validName=isIP(serverName)?cert.checkIP(serverName):cert.checkHost(serverName,{subject:'never'});
    const details=cert.publicKey.asymmetricKeyDetails;
    const strong=cert.publicKey.asymmetricKeyType==='rsa' && (details?.modulusLength??0)>=2048
      || cert.publicKey.asymmetricKeyType==='ec' && ['prime256v1','secp384r1','secp521r1'].includes(details?.namedCurve??'');
    if(cert.ca || !validName || !strong || Date.parse(cert.validFrom)>now || Date.parse(cert.validTo)<=now || !cert.checkPrivateKey(key))throw failure();
  }catch{throw failure();}
  if(endpoint.kind!=='privateSocket' || !/^[A-Za-z0-9][A-Za-z0-9_-]{0,47}\.sock$/.test(endpoint.name) || realpathSync(endpoint.directory)!==endpoint.directory)throw failure();
  const socketPath=join(endpoint.directory,endpoint.name),directory=lstatSync(endpoint.directory),socket=lstatSync(socketPath);
  const verify=()=>{
    const currentDirectory=lstatSync(endpoint.directory),currentSocket=lstatSync(socketPath);
    if(!process.getuid || !currentDirectory.isDirectory() || currentDirectory.uid!==process.getuid() || currentDirectory.mode&0o077
      || !currentSocket.isSocket() || currentSocket.uid!==process.getuid() || currentSocket.mode&0o077
      || currentDirectory.dev!==directory.dev || currentDirectory.ino!==directory.ino || currentSocket.dev!==socket.dev || currentSocket.ino!==socket.ino)throw failure();
  };
  verify();
  const server=createServer({key:privateKey,cert:certificate,minVersion:'TLSv1.3',maxHeaderSize:16*1024,handshakeTimeout:5000});
  const sockets=new Set<Socket>(),controllers=new Set<AbortController>(),lifecycle=new AbortController();
  const completion=Promise.withResolvers<void>();
  void completion.promise.catch(()=>{});
  let authority='',closing:Promise<void>|undefined;
  const close=()=>{
    if(closing)return closing;
    closing=completion.promise;
    const timer=setTimeout(()=>completion.reject(new ShutdownUnconfirmed()),5000);
    try {
      lifecycle.abort();
      for(const controller of controllers)controller.abort();
      server.close(error=>{clearTimeout(timer);signal?.removeEventListener('abort',aborted);if(error && (!('code' in error) || error.code!=='ERR_SERVER_NOT_RUNNING'))completion.reject(new ShutdownUnconfirmed());else completion.resolve();});
      for(const connection of sockets)connection.destroy();
    }catch{clearTimeout(timer);completion.reject(new ShutdownUnconfirmed());}
    return closing;
  };
  const aborted=()=>{void close().catch(()=>{});};
  server.maxConnections=8;server.maxHeadersCount=32;server.maxRequestsPerSocket=1;
  server.headersTimeout=5000;server.requestTimeout=30000;server.keepAliveTimeout=1000;server.setTimeout(30000,connection=>connection.destroy());
  server.on('connection',connection=>{sockets.add(connection);connection.once('close',()=>sockets.delete(connection));});
  server.on('upgrade',(_request,connection)=>connection.destroy());
  server.on('connect',(_request,connection)=>connection.destroy());
  server.on('checkContinue',(_request,response)=>{response.writeHead(417,{'Connection':'close'});response.end();});
  server.on('clientError',(_error,connection)=>connection.destroy());
  server.on('request',(incoming,outgoing)=>{
    const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),30000);controllers.add(controller);
    const canceled=()=>controller.abort();incoming.once('aborted',canceled);outgoing.once('close',canceled);
    const stopInput=()=>incoming.destroy();controller.signal.addEventListener('abort',stopInput,{once:true});
    const respond=(status:number,body:Buffer)=>{if(!outgoing.destroyed){outgoing.writeHead(status,{'Content-Type':'application/json','Cache-Control':'no-store','Connection':'close'});outgoing.end(body);}};
    void (async()=>{
      try {
        if(closing || controllers.size>8 || Math.floor(ipv4(incoming.socket.remoteAddress??'')/subnetSize)!==subnet)throw failure();
        const path=route(incoming.url),forwardHeaders=headers(incoming,authority);
        verify();
        const chunks:Buffer[]=[];let size=0;
        for await(const chunk of incoming){size+=chunk.length;if(size>requestLimit)throw failure();chunks.push(chunk);}
        controller.signal.throwIfAborted();verify();
        const body=Buffer.concat(chunks);
        const result=await new Promise<{status:number;body:Buffer}>((resolve,reject)=>{
          const call=request({socketPath,path,method:incoming.method,agent:false,signal:controller.signal,headers:{...forwardHeaders,...(body.length?{'Content-Length':String(body.length)}:{})}},response=>{
            const parts:Buffer[]=[];let length=0;
            response.on('data',(part:Buffer)=>{length+=part.length;if(length>responseLimit)call.destroy(failure());else parts.push(part);});
            response.once('error',reject);response.once('end',()=>{
              if(!response.complete || !response.statusCode || response.statusCode<200 || response.statusCode>599 || length>responseLimit)reject(failure());
              else resolve({status:response.statusCode,body:Buffer.concat(parts)});
            });
          });call.once('error',reject);call.end(body);
        });
        verify();respond(result.status,result.body);
      }catch{respond(controller.signal.aborted?503:400,Buffer.from('{"error":{"code":"networkUnconfirmed","message":"The network request could not be confirmed."}}'));}
      finally{clearTimeout(timer);controllers.delete(controller);incoming.off('aborted',canceled);outgoing.off('close',canceled);controller.signal.removeEventListener('abort',stopInput);}
    })();
  });
  signal?.addEventListener('abort',aborted,{once:true});
  try {
    signal?.throwIfAborted();
    await new Promise<void>((resolve,reject)=>{
      const cleanup=()=>{server.off('error',failed);lifecycle.signal.removeEventListener('abort',failed);};
      const failed=()=>{cleanup();reject(failure());};
      server.once('error',failed);lifecycle.signal.addEventListener('abort',failed,{once:true});
      server.listen({host:bindAddress,port,signal:lifecycle.signal},()=>{cleanup();resolve();});
    });
    signal?.throwIfAborted();
    const bound=server.address();if(!bound || typeof bound==='string' || bound.address!==bindAddress)throw failure();
    authority=serverName+':'+bound.port;
    server.on('error',aborted);
    return {url:'https://'+authority,closed:completion.promise,close};
  }catch{await close();throw failure();}
}
