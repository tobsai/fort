import { spawn } from 'node:child_process';
import { createServer, type Server } from 'node:net';
import { mkdtemp, mkdir, realpath, writeFile, symlink, open, rm } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { FIXTURE_NODE } from './process.ts';

type Probe=Record<string,boolean>;
const execute=(path:string,args:string[],environment:Record<string,string>,descriptor?:number)=>new Promise<string>((resolve,reject)=>{
  const child=spawn(path,args,{env:environment,stdio:['ignore','pipe','pipe',descriptor??'ignore'],timeout:5000,killSignal:'SIGKILL'});
  let stdout='';let stderr='';
  child.stdout!.on('data',chunk=>{stdout+=chunk;if(stdout.length>8192)child.kill('SIGKILL');});
  child.stderr!.on('data',chunk=>{stderr+=chunk;if(stderr.length>8192)child.kill('SIGKILL');});
  child.once('error',reject);child.once('close',(code,signal)=>{if(code!==0)reject(new Error(`Native synthetic probe failed (${code??signal}): ${stderr.slice(0,2048)}`));else resolve(stdout);});
});
const listen=(server:Server,address:string|{host:string;port:number})=>new Promise<void>((resolve,reject)=>{server.once('error',reject);server.listen(address,resolve);});
const close=(server:Server)=>new Promise<void>(resolve=>server.close(()=>resolve()));

/** Temporary experiment with an installed deprecated mechanism, never a production launcher. */
export async function runConfinementCanary() {
  if(process.platform!=='darwin'||process.version!=='v26.5.0')throw new Error('Canary requires the reviewed Mac and pinned Node');
  const root=await realpath(await mkdtemp('/tmp/fort-canary-'));
  const tcp=createServer(socket=>socket.destroy());const unix=createServer(socket=>socket.destroy());
  const sentinel=spawn(FIXTURE_NODE,['-e','setInterval(()=>{},1000)'],{env:{LANG:'en_US.UTF-8',TZ:'UTC'},stdio:'ignore'});
  const sentinelExit=new Promise<void>(resolve=>sentinel.once('close',()=>resolve()));
  let report:{restrictiveStartup:string;platform:string;mechanism:string;policy:string;productionIsolationProven:boolean;control:Probe;tool:Probe;host:Probe;inheritedDescriptor:boolean;closedDescriptor:boolean;cleaned:boolean}|undefined;
  try {
    await mkdir(root+'/workspace');await mkdir(root+'/home');
    for(const file of ['workspace/allowed','owner','ledger','sibling','provider','home/secret'])await writeFile(root+'/'+file,'fort-synthetic-canary',{mode:0o600});
    await symlink(root+'/owner',root+'/workspace/escape');
    await execute('/usr/bin/clang',['-Wall','-Wextra','-Werror',fileURLToPath(new URL('./confinement-probe.c',import.meta.url)),'-o',root+'/probe'],{PATH:'/usr/bin:/bin',LANG:'en_US.UTF-8'});
    await listen(tcp,{host:'127.0.0.1',port:0});await listen(unix,root+'/ipc');
    const address=tcp.address();if(!address||typeof address==='string'||!sentinel.pid)throw new Error('Synthetic endpoints unavailable');
    const args=[root,String(address.port),String(sentinel.pid)];
    const cleanEnvironment={LANG:'en_US.UTF-8',TZ:'UTC',HOME:root+'/home'};
    const run=async(profile:string|null,inherit=false,secret=false):Promise<Probe>=>{
      const descriptor=inherit?await open(root+'/owner','r'):undefined;
      try {
        const output=await execute(profile?'/usr/bin/sandbox-exec':root+'/probe',profile?['-f',profile,root+'/probe',...args]:args,secret?{...cleanEnvironment,FORT_SYNTHETIC_SECRET:'synthetic'}:cleanEnvironment,descriptor?.fd);
        const parsed=JSON.parse(output);if(!parsed||Object.keys(parsed).length!==13||Object.values(parsed).some(value=>typeof value!=='boolean'))throw new Error('Invalid native probe report');return parsed;
      }finally{await descriptor?.close();}
    };
    const profile=(provider:boolean)=>`(version 1)\n(deny default)\n(allow file-read-metadata)\n(allow sysctl-read)\n(allow file-read* (literal "/dev/random") (literal "/dev/urandom"))\n(allow file-read* file-write* (literal "/dev/null"))\n(allow process-exec (literal ${JSON.stringify(root+'/probe')}))\n(allow file-read* (subpath "/System/Library") (subpath "/usr/lib") (literal ${JSON.stringify(root+'/probe')}))\n(allow file-read* file-write* (subpath ${JSON.stringify(root+'/workspace')}))\n${provider?`(allow file-read* (literal ${JSON.stringify(root+'/provider')}))\n`:''}`;
    await writeFile(root+'/tool.sb',profile(false));await writeFile(root+'/host.sb',profile(true));
    const control=await run(null,true,true);
    let restrictiveStartup='ran';
    try { await run(root+'/tool.sb'); } catch(error) { restrictiveStartup=error instanceof Error?error.message:'failed'; }
    // This diagnostic policy tests named denials only, not confinement of arbitrary code.
    const diagnostic=(provider:boolean)=>`(version 1)\n(allow default)\n(deny network*)\n(deny process-fork)\n(deny signal)\n(deny file-read* file-write* (subpath ${JSON.stringify(root)}))\n(allow file-read* (literal ${JSON.stringify(root+'/probe')}))\n(allow file-read* file-write* (subpath ${JSON.stringify(root+'/workspace')}))\n${provider?`(allow file-read* (literal ${JSON.stringify(root+'/provider')}))\n`:''}`;
    await writeFile(root+'/tool-diagnostic.sb',diagnostic(false));await writeFile(root+'/host-diagnostic.sb',diagnostic(true));
    const tool=await run(root+'/tool-diagnostic.sb');const host=await run(root+'/host-diagnostic.sb');
    const inherited=await run(root+'/tool-diagnostic.sb',true);const closed=await run(null);
    report={restrictiveStartup,platform:'darwin',mechanism:'sandbox-exec-deprecated',policy:'diagnostic-targeted-denials',productionIsolationProven:false,control,tool,host,inheritedDescriptor:inherited.descriptor,closedDescriptor:closed.descriptor,cleaned:false};
    return report;
  }finally{
    sentinel.kill('SIGKILL');await sentinelExit;await Promise.all([close(tcp),close(unix)]);await rm(root,{recursive:true,force:true});if(report)report.cleaned=true;
  }
}
