import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {connect,type Socket} from 'node:net';
import {DatabaseSync} from 'node:sqlite';
import {createOpenAIService} from './openai.ts';

async function connected(port:number){
  const socket=connect(port,'127.0.0.1');
  await new Promise<void>((resolve,reject)=>{socket.once('connect',resolve);socket.once('error',reject);});
  socket.resume();return socket;
}
async function within<T>(operation:Promise<T>,milliseconds:number):Promise<T>{
  let timer:ReturnType<typeof setTimeout>|undefined;
  try{return await Promise.race([operation,new Promise<never>((_,reject)=>{timer=setTimeout(()=>reject(new Error('Socket cleanup exceeded its bound')),milliseconds);})]);}
  finally{clearTimeout(timer);}
}
function assertNoEnrollment(directory:string){
  const db=new DatabaseSync(join(directory,'openai-candidate-b.sqlite'),{readOnly:true});
  try{assert.equal(db.prepare('SELECT COUNT(*) AS n FROM enrollment').get()?.n,0);assert.equal(db.prepare('SELECT COUNT(*) AS n FROM devices').get()?.n,0);}
  finally{db.close();}
}
for(const abortFirst of [true,false])test(`service closes every accepted peer after ${abortFirst?'listener abort':'normal shutdown'} without enrollment`,async()=>{
  const directory=await mkdtemp(join(tmpdir(),'fort-shutdown-no-enrollment-')),abort=new AbortController();
  const service=createOpenAIService({dataDirectory:directory});const sockets:Socket[]=[];
  try{
    await service.start({signal:abort.signal});const port=Number(new URL(service.baseURL).port);
    const idle=await connected(port),headers=await connected(port),body=await connected(port);sockets.push(idle,headers,body);
    const closed=sockets.map(socket=>new Promise<void>(resolve=>socket.once('close',()=>resolve())));
    headers.write('POST /synthetic-incomplete HTTP/1.1\r\nHost: localhost\r\nContent-Length: 500\r\n');
    body.write('POST /v1/conversations/synthetic/memory HTTP/1.1\r\nHost: localhost\r\nContent-Length: 500\r\n\r\n{');
    await new Promise(resolve=>setTimeout(resolve,25));if(abortFirst)abort.abort();
    await within(service.close(),500);await within(Promise.all(closed),250);
    assert.equal(sockets.every(socket=>socket.destroyed),true);
    const probe=connect(port,'127.0.0.1');try{await assert.rejects(()=>new Promise<void>((resolve,reject)=>{probe.once('connect',resolve);probe.once('error',reject);}));}finally{probe.destroy();}
    await service.close();assertNoEnrollment(directory);
  }finally{for(const socket of sockets)socket.destroy();await service.close();await rm(directory,{recursive:true,force:true});}
});
for(const mode of ['never-started','already-aborted','abort-while-starting','close-while-starting'] as const)test(`service shutdown is idempotent without listener readiness or enrollment: ${mode}`,async()=>{
  const directory=await mkdtemp(join(tmpdir(),'fort-startup-no-enrollment-')),abort=new AbortController();const service=createOpenAIService({dataDirectory:directory});
  try{
    if(mode==='already-aborted'){abort.abort();await assert.rejects(()=>service.start({signal:abort.signal}));}
    if(mode==='abort-while-starting'||mode==='close-while-starting'){
      const starting=service.start({signal:abort.signal});void starting.catch(()=>{});
      if(mode==='abort-while-starting')abort.abort();else await within(service.close(),500);
      await assert.rejects(()=>within(starting,500));
    }
    await within(service.close(),500);await within(service.close(),500);assertNoEnrollment(directory);
  }finally{await service.close();await rm(directory,{recursive:true,force:true});}
});
