import { createFixtureService } from './service.ts';
import { CodexTurn } from '../runner/codex.ts';
import type { SyntheticDispatch, RunnerScope } from '../contracts/runner.ts';
const [mode,directory,point]=process.argv.slice(2);
const checkpoint=(actual:string)=>{if(actual===point)process.kill(process.pid,'SIGKILL');};
if(mode==='service') {
  const service=createFixtureService({dataDirectory:directory,syntheticRunner:true,checkpoint});
  await service.start();process.send?.({baseURL:service.baseURL});
} else if(mode==='runner') {
  process.send?.({ready:true});
  process.once('message',async(value:unknown)=>{
    const {baseURL,scope,dispatch,headers}=value as {baseURL:string;scope:RunnerScope;dispatch:SyntheticDispatch;headers:Record<string,string>};
    const ingest=async(event:unknown)=>{
      const response=await fetch(baseURL+'/fixture/runner/events',{method:'POST',headers:{...headers,'Content-Type':'application/json'},body:JSON.stringify({...scope,dispatchToken:dispatch.dispatchToken,event})});
      await response.text();if(response.status!==200)throw new Error('Synthetic evidence was not accepted.');
    };
    const frame=(message:unknown)=>Buffer.from(JSON.stringify(message)+'\n');
    const turn=new CodexTurn({attempt:dispatch.attempt,threadId:dispatch.threadId,requestId:dispatch.requestId,workspace:dispatch.workspace,text:dispatch.text});
    checkpoint('runnerBeforeStart');turn.start();
    const started=turn.receive(frame({id:dispatch.requestId,result:{turn:{id:'crash-fixture-turn',status:'inProgress'}}}));
    await ingest(started.events[0]);checkpoint('runnerAfterStarted');
    if(point==='runnerExitZero')process.exit(0);
    turn.receive(frame({method:'item/completed',params:{threadId:dispatch.threadId,turnId:'crash-fixture-turn',item:{id:'reply-one',type:'agentMessage',text:'exact synthetic crash reply',phase:'final_answer'}}}));
    const terminal=turn.receive(frame({method:'turn/completed',params:{threadId:dispatch.threadId,turn:{id:'crash-fixture-turn',status:'completed',error:null}}}));
    checkpoint('runnerBeforeTerminal');await ingest(terminal.events[0]);checkpoint('runnerAfterTerminal');
    process.exit(0);
  });
} else throw new Error('Choose the synthetic service or runner crash fixture.');
