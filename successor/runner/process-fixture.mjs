import { createInterface } from 'node:readline';
import { spawn } from 'node:child_process';
const [scenario,threadId]=process.argv.slice(2);
const write=bytes=>new Promise((resolve,reject)=>process.stdout.write(bytes,error=>error?reject(error):resolve()));
const frame=async value=>{
  const bytes=Buffer.from(JSON.stringify(value)+'\n');
  if(scenario==='fragmented')for(const byte of bytes)await write(Buffer.from([byte]));else await write(bytes);
};
if(scenario==='descendant-child') {
  process.on('SIGTERM',()=>{});process.send?.({ready:true});setInterval(()=>{},1000);
} else if(scenario==='blocked-input') {
  process.on('SIGTERM',()=>{});await frame({kind:'ready',threadId});setInterval(()=>{},1000);
} else {
  if(['ignore-term','terminal-descendant','descendant'].includes(scenario))process.on('SIGTERM',()=>{});
  let busy=Promise.resolve();
  const lines=createInterface({input:process.stdin});
  lines.on('line',line=>{busy=busy.then(async()=>{
    const request=JSON.parse(line);
    if(request.kind==='handshake'){if(scenario!=='handshake-stall')await frame({kind:'ready',threadId});return;}
    if(request.method==='turn/interrupt'){await frame({id:request.id,result:{}});if(scenario==='interruptible')await frame({method:'turn/completed',params:{threadId,turn:{id:'fixture-turn',status:'interrupted',error:null}}});return;}
    if(request.method!=='turn/start')return;
    if(scenario==='early-burst')for(let i=0;i<32;i++)await frame({method:'item/agentMessage/delta',params:{threadId,turnId:'fixture-turn',itemId:'reply-one',delta:'x'}});
    await frame({id:request.id,result:{turn:{id:'fixture-turn',status:'inProgress'}}});
    if(scenario==='exit-zero')process.exit(0);
    if(scenario==='partial-eof'){await write(Buffer.from('{"method":"item/completed","params":'));process.exit(0);}
    if(scenario==='stdout-flood'||scenario==='stderr-flood') {
      const stream=scenario==='stdout-flood'?process.stdout:process.stderr;
      const block=Buffer.alloc(16_384,32);const count=scenario==='stdout-flood'?128:16;
      for(let i=0;i<count;i++)await new Promise(resolve=>stream.write(block,()=>resolve()));
      return;
    }
    if(['hold','ignore-term','interruptible'].includes(scenario))return;
    if(scenario==='descendant'||scenario==='terminal-descendant') {
      const child=spawn(process.execPath,[import.meta.filename,'descendant-child'],{env:{LANG:'en_US.UTF-8',TZ:'UTC'},stdio:['ignore','inherit','inherit','ipc']});
      await new Promise((resolve,reject)=>{child.once('message',resolve);child.once('error',reject);});
      if(scenario==='descendant')process.exit(0);
    }
    const text=request.params.input[0].text;
    await frame({method:'item/completed',params:{threadId,turnId:'fixture-turn',item:{id:'reply-one',type:'agentMessage',phase:'final_answer',text}}});
    await frame({method:'turn/completed',params:{threadId,turn:{id:'fixture-turn',status:'completed',error:null}}});
    if(scenario==='terminal-descendant')process.exit(0);
  }).catch(()=>process.exit(2));});
  lines.on('close',()=>{void busy.then(()=>process.exit(0));});
}
