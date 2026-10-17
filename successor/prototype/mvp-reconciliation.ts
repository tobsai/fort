import {existsSync,lstatSync,readFileSync,writeFileSync,renameSync} from 'node:fs';
import {join,isAbsolute} from 'node:path';
import {createHash} from 'node:crypto';

export const MVP_CLEANUP_SCOPE_SHA256='24232f4c96da97a3fa2b64620bedcc4752332d17e111bc8c7d583eac26bdf4f6';
const firstKeychainEvidence={operatorThreadId:'01a1451b-81d7-76cf-8e5c-a85d7164201c',turnId:'01a14a81-8c2b-7271-b508-e739b6832403',result:'operator verified installation-matched scoped Keychain item already absent',deleteStatus:44,independentlyRechecked:false};
const cases={1:{receipts:3,remainingSessions:[2,3],keychainEvidence:firstKeychainEvidence},2:{receipts:1,remainingSessions:[3],keychainEvidence:{operatorThreadId:'01a1451b-81d7-76cf-8e5c-a85d7164201c',turnId:'01a14aa5-a8ff-70af-8478-2b279d766afc',result:'bounded controller reported scoped credential cleanup deleted; operator confirmed connection and pairing metadata absent',independentlyRechecked:false}}};
type State={version:number;session:number;phase:string;began:number;workDeadline:number;credentialDeadline:number;launches:number;restarts:number;receipts:{runId:string}[];credentialCleanup:string;runtimeCleanup:string;nativeCleanup:string;installationId:string;port:number};
export type CleanupLedger={installationId:string;credentialDeadline:number;devices:number;enrollments:number;runs:{runId:string;state:string}[]};
export type ReconciliationOptions={batchRoot:string;session?:1|2;scopeSHA256:string;expectedStateSHA256:string;expectedLockSHA256:string;nativeDirectory:string;assertNativeExited:()=>void;assertRuntimeExited:()=>void;assertListenerClosed:(port:number)=>void;readLedger:()=>CleanupLedger;now?:()=>number;inspectOnly?:boolean};
const hash=(bytes:Uint8Array)=>createHash('sha256').update(bytes).digest('hex');
function read(path:string):Buffer{
  if(!isAbsolute(path))throw new Error('Use exact cleanup evidence paths');
  let parent=path;while(parent!=='/'){if(existsSync(parent)&&lstatSync(parent).isSymbolicLink())throw new Error('Cleanup evidence cannot contain symlinks');parent=join(parent,'..');}
  if(!lstatSync(path).isFile()||lstatSync(path).size>1_000_000)throw new Error('Invalid cleanup evidence');return readFileSync(path);
}
function consumed(state:State,session:1|2):boolean{
  return state.version===1&&state.session===session&&state.phase==='blocked'&&state.launches===1&&state.restarts===1&&state.receipts.length===cases[session].receipts&&state.credentialCleanup==='deleted'&&state.runtimeCleanup==='confirmed'&&state.nativeCleanup==='unconfirmed'&&Number.isSafeInteger(state.began)&&state.workDeadline===state.began+540_000&&state.credentialDeadline===state.began+600_000&&/^[A-Za-z0-9_-]{1,128}$/.test(state.installationId)&&Number.isInteger(state.port)&&state.port>0&&state.port<65536;
}
function validRecord(batchRoot:string,session:1|2,archivedLock=true):boolean{
  try{
    const details=cases[session],root=join(batchRoot,`session-${session}`),bytes=read(join(root,'session-state.json')),state=JSON.parse(bytes.toString()) as State;
    const record=JSON.parse(read(join(root,'cleanup-reconciliation.json')).toString());
    const checks=record.checks;
    return consumed(state,session)&&record.schemaVersion===1&&record.session===session&&record.scopeSHA256===MVP_CLEANUP_SCOPE_SHA256&&record.originalStateSHA256===hash(bytes)&&record.installationId===state.installationId&&record.credentialDeadline===state.credentialDeadline&&Date.parse(record.checkedAtUTC)>=state.credentialDeadline&&Date.parse(record.checkedAtUTC)<=Date.now()&&record.consumed.launches===1&&record.consumed.restarts===1&&record.consumed.receipts===details.receipts&&JSON.stringify(record.remainingSessions)===JSON.stringify(details.remainingSessions)&&JSON.stringify(record.keychainEvidence)===JSON.stringify(details.keychainEvidence)&&checks.appExited===true&&checks.runtimeExited===true&&checks.listenerClosed===true&&checks.pendingCount===0&&checks.connectionAbsent===true&&checks.pairingAbsent===true&&checks.devices===0&&checks.enrollments===0&&checks.completedRuns===details.receipts&&record.retainedLockSHA256===hash(read(join(archivedLock?root:batchRoot,archivedLock?'reconciled-active.lock':'active.lock')));
  }catch{return false;}
}
export function assertMvpSessionAvailable(batchRoot:string,session:1|2|3):void{
  if(existsSync(join(batchRoot,`session-${session}`)))throw new Error('This session start is consumed; never reset or reuse it');
  if(existsSync(join(batchRoot,'active.lock')))throw new Error('The batch lock requires confirmed cleanup');
  for(let prior=1;prior<session;prior++){
    try{
      const state=JSON.parse(read(join(batchRoot,`session-${prior}`,'session-state.json')).toString()) as State;
      const hasRecord=existsSync(join(batchRoot,`session-${prior}`,'cleanup-reconciliation.json'));
      const nativeConfirmed=hasRecord?(prior===1||prior===2)&&validRecord(batchRoot,prior):state.nativeCleanup==='confirmed';
      if(state.session!==prior||!['finished','blocked'].includes(state.phase)||state.credentialCleanup!=='deleted'||state.runtimeCleanup!=='confirmed'||!nativeConfirmed)throw new Error();
    }catch{throw new Error('Previous session cleanup must be confirmed');}
  }
}
export function reconcileMvpCleanup(options:ReconciliationOptions){
  const {batchRoot,nativeDirectory}=options,session=options.session??1,details=cases[session],root=join(batchRoot,`session-${session}`),statePath=join(root,'session-state.json'),recordPath=join(root,'cleanup-reconciliation.json'),lock=join(batchRoot,'active.lock'),archive=join(root,'reconciled-active.lock');
  if(!details||options.scopeSHA256!==MVP_CLEANUP_SCOPE_SHA256||details.remainingSessions.some(n=>existsSync(join(batchRoot,`session-${n}`)))||(session===2&&!validRecord(batchRoot,1)))throw new Error('Reconciliation requires unchanged scope, prior evidence and unused remaining sessions');
  const bytes=read(statePath),state=JSON.parse(bytes.toString()) as State,now=(options.now??Date.now)();
  if(hash(bytes)!==options.expectedStateSHA256||!consumed(state,session)||now<state.credentialDeadline)throw new Error('Consumed session identity, counts or expiry changed');
  const lockPath=existsSync(lock)?lock:archive;
  if(hash(read(lockPath))!==options.expectedLockSHA256)throw new Error('Retained lock identity changed');
  if(lockPath===archive){if(!validRecord(batchRoot,session))throw new Error('Cleanup reconciliation is unconfirmed');return {reconciled:true,alreadyRecorded:true,remainingSessions:details.remainingSessions};}
  if(existsSync(archive))throw new Error('Retained lock archive already exists');
  options.assertNativeExited();options.assertRuntimeExited();options.assertListenerClosed(state.port);
  const pendingPath=join(nativeDirectory,createHash('sha256').update(state.installationId).digest('hex'),'pending.json'),pending=JSON.parse(read(pendingPath).toString());
  if(!Array.isArray(pending)||pending.length!==0||existsSync(join(nativeDirectory,'connection.json'))||existsSync(join(root,'local-enrollment.json')))throw new Error('Native queue or connection cleanup is unconfirmed');
  const ledger=options.readLedger();
  if(ledger.installationId!==state.installationId||ledger.credentialDeadline!==state.credentialDeadline||ledger.devices!==0||ledger.enrollments!==0||ledger.runs.length!==details.receipts||ledger.runs.some(run=>run.state!=='completed')||JSON.stringify(ledger.runs.map(r=>r.runId).sort())!==JSON.stringify(state.receipts.map(r=>r.runId).sort()))throw new Error('Exact ledger cleanup is unconfirmed');
  options.assertNativeExited();options.assertRuntimeExited();options.assertListenerClosed(state.port);
  if(hash(read(statePath))!==options.expectedStateSHA256||hash(read(lock))!==options.expectedLockSHA256)throw new Error('Cleanup evidence changed during verification');
  const record={schemaVersion:1,session,scopeSHA256:options.scopeSHA256,originalStateSHA256:options.expectedStateSHA256,retainedLockSHA256:options.expectedLockSHA256,installationId:state.installationId,credentialDeadline:state.credentialDeadline,checkedAtUTC:new Date(now).toISOString(),consumed:{launches:1,restarts:1,receipts:details.receipts},remainingSessions:details.remainingSessions,nativeDirectory,checks:{appExited:true,runtimeExited:true,listenerClosed:true,pendingCount:0,connectionAbsent:true,pairingAbsent:true,devices:0,enrollments:0,completedRuns:details.receipts},keychainEvidence:details.keychainEvidence};
  if(options.inspectOnly)return {reconciled:false,ready:true,record};
  if(existsSync(recordPath)){if(!validRecord(batchRoot,session,false))throw new Error('Existing reconciliation record differs');}
  else writeFileSync(recordPath,JSON.stringify(record,null,2)+'\n',{flag:'wx',mode:0o600});
  renameSync(lock,archive);
  if(!validRecord(batchRoot,session))throw new Error('Cleanup reconciliation could not be verified');
  return {reconciled:true,alreadyRecorded:false,remainingSessions:details.remainingSessions};
}
