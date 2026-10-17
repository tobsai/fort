import {createInterface} from 'node:readline';
import {join} from 'node:path';
import {MvpPacket} from './mvp-session.ts';
import {createMvpRuntime} from './mvp-runtime.ts';
import {archiveMvpNativeCache,createMvpNative} from './mvp-native.ts';
import {MVP_ROOT,verifyMvpPreparation,requireMvpApproval} from './mvp-activation.ts';
import {assertMvpSessionAvailable} from './mvp-reconciliation.ts';

export async function runMvpManual(args=process.argv.slice(2)){
  const prepared=verifyMvpPreparation(),native=createMvpNative(prepared.appPath);
  if(args.length===1&&args[0]==='--check'){
    let activation=prepared.approvalPresent?'blocked: owner approval could not be validated':'blocked: owner approval pending';try{requireMvpApproval(prepared);activation='approved scope; operator must separately start one unused session';}catch{}
    native.assertExited();
    const sessions=([1,2,3] as const).map(session=>{try{assertMvpSessionAvailable(MVP_ROOT,session);return {session,available:true};}catch{return {session,available:false};}});
    process.stdout.write(JSON.stringify({preparation:'passed',sourceCommit:prepared.sourceCommit,appPath:native.appPath,nativeDirectory:native.directory,activation,sessions,liveActionsPerformed:false})+'\n');return;
  }
  if(args.length!==2||args[0]!=='--session'||!['1','2','3'].includes(args[1]!))throw new Error('Use --check or --session 1, 2 or 3');
  requireMvpApproval(prepared);
  const session=Number(args[1]) as 1|2|3,root=join(MVP_ROOT,`session-${session}`);
  const packet=new MvpPacket({batchRoot:MVP_ROOT,session,native:native.session,prepareNative:async root=>archiveMvpNativeCache(root,native.directory,native.assertExited),launchRuntime:budget=>{verifyMvpPreparation();return createMvpRuntime(root,budget);}});
  const terminal=createInterface({input:process.stdin,output:process.stdout}),abort=new AbortController();
  let active:Awaited<ReturnType<MvpPacket['start']>>|undefined,consumed=false;
  const stop=()=>{abort.abort();terminal.close();};process.on('SIGINT',stop);process.on('SIGTERM',stop);terminal.once('close',()=>abort.abort());
  process.stdout.write('Attended synthetic Mac acceptance. Commands: start, status, native-status, restart, done, abort. Quit Fort through its UI and check native-status before restart or done. No message text is accepted here. One explicit restart is shared with the native runtime button.\n');
  try{
    for await(const line of terminal){
      const command=line.trim();
      if(command==='start'){
        if(consumed)throw new Error('This session start is consumed');consumed=true;
        verifyMvpPreparation();requireMvpApproval(prepared);active=await packet.start(abort.signal);
        process.stdout.write(JSON.stringify({phase:'READY',appPath:prepared.appPath,pairingFile:active.pairingFile,installationId:active.installationId,workCutoff:new Date(active.workDeadline).toISOString(),credentialExpiry:new Date(active.credentialDeadline).toISOString(),maximumTurns:12})+'\n');
        process.stdout.write('Pair from the local file without displaying its values. Use only the checklist samples. Report readiness and both deadlines to the parent.\n');
        void active.closed.then(state=>{process.stdout.write(JSON.stringify({phase:state.phase,launches:state.launches,restarts:state.restarts,receipts:state.receipts,credentialCleanup:state.credentialCleanup,runtimeCleanup:state.runtimeCleanup,nativeCleanup:state.nativeCleanup})+'\n');terminal.close();});
      }else if(command==='status'){
        if(!active)process.stdout.write('No active session.\n');else process.stdout.write(JSON.stringify({runs:active.status().runs.map(run=>({requestId:run.requestId,runId:run.runId,state:run.state,selectedModel:run.selectedModel,effectiveModel:run.effectiveModel}))})+'\n');
      }else if(command==='native-status'){
        try{native.assertExited();process.stdout.write('Native app exit confirmed.\n');}catch{process.stdout.write('Native app exit is unconfirmed. Quit the exact Fort app through its UI before restart or done.\n');}
      }else if(command==='restart'){
        if(!active)throw new Error('No active session');await active.restartService();process.stdout.write('Service/runtime restarted with the same installation and credential expiry. Relaunch the same app; do not enroll again.\n');
      }else if(command==='done'){if(active)await active.finish();terminal.close();}
      else if(command==='abort'){await active?.abort();terminal.close();}
      else process.stdout.write('Use start, status, native-status, restart, done or abort.\n');
    }
  }finally{abort.abort();await active?.abort();terminal.close();process.off('SIGINT',stop);process.off('SIGTERM',stop);}
}
if(import.meta.main){try{await runMvpManual();}catch{process.stderr.write('MVP session refused or stopped. Check approval, prepared pins and redacted session state. Never reset a consumed start.\n');process.exitCode=1;}}
