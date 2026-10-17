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
  if(args.length!==1||!['--check','--record'].includes(args[0]!))throw new Error('Use --check or --record');
  const prepared=verifyMvpPreparation();requireMvpApproval(prepared);
  const native=createMvpNative(prepared.appPath);
  const result=reconcileMvpCleanup({batchRoot:MVP_ROOT,scopeSHA256:prepared.scopeSHA256,expectedStateSHA256:'a4d8a619767ddcb06472bf79404a3e93eca38311cfa07cda773a06ec3cecdc4f',expectedLockSHA256:'a1855f3fad32884fc39e165c251e446177b5d20aab4c78cf5bed0f1cf2c1391a',nativeDirectory:native.directory,assertNativeExited:native.assertExited,assertRuntimeExited,assertListenerClosed,readLedger:()=>readMvpCleanupLedger(join(MVP_ROOT,'session-1/service/openai-candidate-b.sqlite')),inspectOnly:args[0]==='--check'});
  process.stdout.write(JSON.stringify({...result,liveActionsPerformed:false})+'\n');
}
if(import.meta.main){try{runMvpReconciliation();}catch{process.stderr.write('Cleanup reconciliation refused; preserve the state and retained lock.\n');process.exitCode=1;}}
