import {createInterface} from 'node:readline';
import {join} from 'node:path';
import {execFileSync} from 'node:child_process';
import {MvpPacket} from './mvp-session.ts';
import {createMvpRuntime} from './mvp-runtime.ts';
import {archiveMvpNativeCache,createMvpNative} from './mvp-native.ts';
import {OWNER_DEMO_ROOT,verifyOwnerDemoPreparation,requireOwnerDemoApproval} from './owner-demo-activation.ts';
import {assertMvpSessionAvailable} from './mvp-reconciliation.ts';
import {checkOwnerDemoWrites,OwnerDemoStartupFailure,type OwnerDemoStage} from './owner-demo-startup.ts';

export async function runOwnerDemoManual(args=process.argv.slice(2)){
  let stage:OwnerDemoStage='preparation';
  try{
  const prepared=verifyOwnerDemoPreparation();stage='approval';requireOwnerDemoApproval(prepared);
  stage='native-identity';const native=createMvpNative(prepared.appPath);
  if(args.length===1&&args[0]==='--check'){
    assertMvpSessionAvailable(OWNER_DEMO_ROOT,1);
    process.stdout.write(JSON.stringify({preparation:'passed',sourceCommit:prepared.sourceCommit,appPath:native.appPath,nativeDirectory:native.directory,activation:'approved; one unused owner-demo start',liveActionsPerformed:false})+'\n');return;
  }
  if(args.length===1&&args[0]==='--check-writes'){
    stage='write-preflight';checkOwnerDemoWrites(OWNER_DEMO_ROOT);
    process.stdout.write(JSON.stringify({filesystemPreparation:'passed',sessionCreated:false,clockStarted:false,runtimeLaunched:false,enrollmentIssued:false})+'\n');return;
  }
  if(args.length!==0)throw new Error('Use --check, --check-writes or no arguments for the attended terminal');
  const session=1,root=join(OWNER_DEMO_ROOT,'session-1');
  const packet=new MvpPacket({profile:'owner-demo-20261017',batchRoot:OWNER_DEMO_ROOT,session,native:native.session,prepareNative:async root=>archiveMvpNativeCache(root,native.directory,native.assertExited),launchRuntime:budget=>{verifyOwnerDemoPreparation();return createMvpRuntime(root,budget);}});
  const terminal=createInterface({input:process.stdin,output:process.stdout}),abort=new AbortController();
  let active:Awaited<ReturnType<MvpPacket['start']>>|undefined,consumed=false;
  const stop=()=>{abort.abort();terminal.close();};process.on('SIGINT',stop);process.on('SIGTERM',stop);terminal.once('close',()=>abort.abort());
  process.stdout.write('Attended interactive owner demo. Commands: start, status, native-status, done, abort. First quit any Fort preview through its UI. Type start only when ready to pair and hand over the composer. One launch; no restarts. No message text is accepted here.\n');
  try{
    for await(const line of terminal){
      const command=line.trim();
      if(command==='start'){
        if(consumed)throw new Error('This session start is consumed');stage='native-exit';native.assertExited();
        stage='preparation';verifyOwnerDemoPreparation();stage='approval';requireOwnerDemoApproval(prepared);
        stage='write-preflight';checkOwnerDemoWrites(OWNER_DEMO_ROOT);
        stage='session-start';active=await packet.start(abort.signal);consumed=true;
        stage='native-open';
        execFileSync('/usr/bin/open',['-n',prepared.appPath],{timeout:5_000,stdio:'ignore'});
        process.stdout.write(JSON.stringify({phase:'READY',appPath:prepared.appPath,pairingFile:active.pairingFile,installationId:active.installationId,workCutoff:new Date(active.workDeadline).toISOString(),credentialExpiry:new Date(active.credentialDeadline).toISOString(),maximumTurns:12})+'\n');
        process.stdout.write('Normal Fort app opened. Privately pair once from the local file, create Writer with model gpt-6-astra and instructions Reply briefly using supplied context., then open its Home with no skill or memory. Hand the composer to the owner immediately and report both deadlines. At most 12 plain-text requests; no routines, schedules or restart.\n');
        void active.closed.then(state=>{process.stdout.write(JSON.stringify({phase:state.phase,launches:state.launches,restarts:state.restarts,receipts:state.receipts,credentialCleanup:state.credentialCleanup,runtimeCleanup:state.runtimeCleanup,nativeCleanup:state.nativeCleanup})+'\n');terminal.close();});
      }else if(command==='status'){
        stage='session-command';
        if(!active)process.stdout.write('No active session.\n');else process.stdout.write(JSON.stringify({runs:active.status().runs.map(run=>({requestId:run.requestId,runId:run.runId,state:run.state,selectedModel:run.selectedModel,effectiveModel:run.effectiveModel}))})+'\n');
      }else if(command==='native-status'){
        stage='native-exit';
        try{native.assertExited();process.stdout.write('Native app exit confirmed.\n');}catch{process.stdout.write('Native app exit is unconfirmed. Quit the exact Fort app through its UI before done.\n');}
      }else if(command==='done'){stage='session-command';if(active)await active.finish();terminal.close();}
      else if(command==='abort'){stage='session-command';await active?.abort();terminal.close();}
      else process.stdout.write('Use start, status, native-status, done or abort.\n');
    }
  }finally{abort.abort();await active?.abort();terminal.close();process.off('SIGINT',stop);process.off('SIGTERM',stop);}
  }catch(error){throw new OwnerDemoStartupFailure(error,stage,OWNER_DEMO_ROOT);}
}
if(import.meta.main){try{await runOwnerDemoManual();}catch(error){process.stderr.write(JSON.stringify(error instanceof OwnerDemoStartupFailure?error.report:{phase:'refused',stage:'entry',code:'unclassified',nextAction:'Inspect the launcher before retrying; no raw exception was logged.'})+'\n');process.exitCode=1;}}
