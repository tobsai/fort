import {mkdirSync,readdirSync,readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {join} from 'node:path';
import {CodexChat} from '../runner/codex-chat.ts';
import {launchOwnedProcess,type OwnedProcessOptions} from '../runner/owned-process.ts';
import {codexLaunchPlan,CODEX_EXECUTABLE,CODEX_SHA256} from './owned-codex.ts';
import type {RuntimeBudget,PreparedRuntime} from './mvp-session.ts';

export function createMvpRuntime(root:string,budget:RuntimeBudget,launch:(plan:OwnedProcessOptions)=>ReturnType<typeof launchOwnedProcess>=launchOwnedProcess,verifyRuntime=()=>{if(createHash('sha256').update(readFileSync(CODEX_EXECUTABLE)).digest('hex')!==CODEX_SHA256)throw new Error('Official runtime pin changed');}):PreparedRuntime {
  const workspace=join(root,'empty'),temporaryDirectory=join(root,'tmp');
  for(const path of [workspace,temporaryDirectory])mkdirSync(path,{recursive:true,mode:0o700});
  const maximumConnections=budget.remainingConnections();
  const connect=()=>{
    budget.signal.throwIfAborted();verifyRuntime();
    if(readdirSync(workspace).length!==0)throw new Error('Runtime workspace is no longer empty');
    const remainingWorkMs=budget.claimLaunch();
    return launch(codexLaunchPlan({workspace,temporaryDirectory,remainingWorkMs})).connection;
  };
  const connection=connect();
  try{return new CodexChat({connection,workspace,runtimeVersion:'0.157.1',replacement:{maximumConnections,deadline:budget.workDeadline,connect}});}
  catch(error){
    connection.close();
    return {async preflight(){throw error;},async execute(){return {kind:'notStarted',message:'Runtime construction was refused. Nothing was dispatched.'};},dispose:()=>connection.dispose()};
  }
}
