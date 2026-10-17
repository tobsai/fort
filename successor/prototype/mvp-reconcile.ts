import {execFileSync} from 'node:child_process';
import {existsSync,realpathSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import {join} from 'node:path';
import {MVP_ROOT,verifyMvpPreparation,requireMvpApproval} from './mvp-activation.ts';
import {createMvpNative} from './mvp-native.ts';
import {reconcileMvpCleanup,type CleanupLedger} from './mvp-reconciliation.ts';
import {CODEX_EXECUTABLE} from './owned-codex.ts';

export function readMvpCleanupLedger(path:string):CleanupLedger{
  return JSON.parse(execFileSync('/usr/bin/python3',['-B',fileURLToPath(new URL('./mvp-reconcile-ledger.py',import.meta.url)),path],{encoding:'utf8',timeout:2_000,stdio:['ignore','pipe','pipe']})) as CleanupLedger;
}
function assertRuntimeExited(){
  const expected=realpathSync(CODEX_EXECUTABLE);
  const processes=execFileSync('/bin/ps',['-ww','-axo','pid=,comm='],{encoding:'utf8',timeout:1_000,stdio:['ignore','pipe','pipe']});
  for(const line of processes.split('\n')){
    const executable=/^\s*\d+\s+(.+)$/.exec(line)?.[1];
    if(executable&&(executable===expected||(existsSync(executable)&&realpathSync(executable)===expected)))throw new Error('Official runtime absence is unconfirmed');
  }
}
function assertListenerClosed(port:number){
  try{execFileSync('/usr/sbin/lsof',['-nP',`-iTCP:${port}`,'-sTCP:LISTEN'],{stdio:'ignore',timeout:1_000});}
  catch(error){if(error instanceof Error&&'status' in error&&error.status===1)return;throw new Error('Listener status is unknown');}
  throw new Error('Session listener is still open');
}
export function runMvpReconciliation(args=process.argv.slice(2)){
  if(!['--check','--record'].includes(args[0]!)||!(args.length===1||(args.length===3&&args[1]==='--session'&&['1','2'].includes(args[2]!))))throw new Error('Use --check or --record, optionally followed by --session 1 or 2');
  const session=args[2]==='2'?2:1,pins=session===1?{state:'a4d8a619767ddcb06472bf79404a3e93eca38311cfa07cda773a06ec3cecdc4f',lock:'a1855f3fad32884fc39e165c251e446177b5d20aab4c78cf5bed0f1cf2c1391a'}:{state:'712a53c78a9b16dd643793ae88812e57a7485a015e6cd5db265f540e8fabfc3b',lock:'84eb1d851805b97b154a0024b75935a8c03b6244bba002981655a602df0d95f5'};
  const prepared=verifyMvpPreparation();requireMvpApproval(prepared);
  const native=createMvpNative(prepared.appPath);
  const result=reconcileMvpCleanup({batchRoot:MVP_ROOT,session,scopeSHA256:prepared.scopeSHA256,expectedStateSHA256:pins.state,expectedLockSHA256:pins.lock,nativeDirectory:native.directory,assertNativeExited:native.assertExited,assertRuntimeExited,assertListenerClosed,readLedger:()=>readMvpCleanupLedger(join(MVP_ROOT,`session-${session}/service/openai-candidate-b.sqlite`)),inspectOnly:args[0]==='--check'});
  process.stdout.write(JSON.stringify({...result,liveActionsPerformed:false})+'\n');
}
if(import.meta.main){try{runMvpReconciliation();}catch{process.stderr.write('Cleanup reconciliation refused; preserve the state and retained lock.\n');process.exitCode=1;}}
