import {mkdirSync,existsSync,lstatSync,readFileSync,writeFileSync,renameSync,unlinkSync} from 'node:fs';
import {isAbsolute,join,normalize} from 'node:path';
import {randomUUID} from 'node:crypto';
import {DatabaseSync} from 'node:sqlite';
import {createOpenAIService,type SubmissionAdmission} from '../service/openai.ts';
import type {Acceptance,Run,RuntimeReadiness} from '../contracts/v1.ts';
import type {ChatRuntime} from '../runner/chat-runtime.ts';
import type {NativeSession} from './session.ts';

export const MVP_PROMPTS:Readonly<Record<1|2|3,readonly string[]>>={
  1:['What sample packing item is saved in this conversation\'s memory?','Our sample code word is cedar. Reply with that word only.','What sample code word did I give you earlier?','What sample packing item and code word have we used?','Reply with the single word ready.'],
  2:['Suggest one short title for a sample packing list.','Suggest another short title for a sample packing list.'],
  3:['Our sample plan is to pack a red notebook. Repeat the item.','Check that sample plan and name the packing item.','What departure is saved in this shared conversation?','Reply with the single word ready.','Write a short synthetic packing checklist for a red notebook.'],
};
export type RuntimeBudget={workDeadline:number;signal:AbortSignal;remainingConnections:()=>number;claimLaunch:()=>number};
export type PreparedRuntime=ChatRuntime&{preflight(signal:AbortSignal):Promise<{model:string;runtimeVersion:string}>};
type State={version:1;session:1|2|3;phase:'running'|'finished'|'blocked';began:number;workDeadline:number;credentialDeadline:number;launches:number;restarts:number;installationId?:string;port?:number;receipts:Acceptance['receipt'][];credentialCleanup:'pending'|'deleted'|'unconfirmed';runtimeCleanup:'pending'|'confirmed'|'unconfirmed';nativeCleanup:'pending'|'confirmed'|'unconfirmed'};
type Options={batchRoot:string;session:1|2|3;native:NativeSession;prepareNative?:(sessionRoot:string)=>Promise<void>;launchRuntime:(budget:RuntimeBudget)=>PreparedRuntime;now?:()=>number;scheduleStop?:(stop:()=>void,delay:number)=>()=>void};
function safeDirectory(path:string){
  if(!isAbsolute(path)||normalize(path)!==path||path==='/'||path.includes('\0'))throw new Error('Use an exact session directory');
  let current=path;while(current!=='/'){if(existsSync(current)&&lstatSync(current).isSymbolicLink())throw new Error('Session directories cannot contain symlinks');current=join(current,'..');}
}
function permitted(submission:SubmissionAdmission,runs:readonly Run[],session:1|2|3):boolean {
  const c=submission.configuration,room=submission.conversation;
  if(runs.length>=12||!MVP_PROMPTS[session].includes(submission.text)||c.selectedModel!=='gpt-6-astra')return false;
  if(c.instructions!==(c.agentName==='Writer'?'Reply briefly using supplied context.':c.agentName==='Reviewer'?'Check the sample claim.':null))return false;
  if(room.kind==='shared'&&(session!==3||room.participantAgentIds.length!==2))return false;
  if(room.kind==='home'&&(room.participantAgentIds.length!==1||room.participantAgentIds[0]!==c.agentId))return false;
  const note=c.memory?.text??'';
  if(!['',...(session===1?['The sample packing item is a red notebook.']:session===3?['The shared sample departure is Monday.']:[])].includes(note))return false;
  if(c.skills.length>1||(c.skills.length>0&&session!==2))return false;
  return c.skills.every(skill=>skill.name==='Check marker'&&['End with [CHECKED].','End with [REVIEWED].'].includes(skill.instructions)&&skill.bundle?.requestedTools.length===0&&skill.bundle.scripts.length===0);
}

export class MvpPacket {
  private readonly options:Options;
  constructor(options:Options){safeDirectory(options.batchRoot);if(![1,2,3].includes(options.session))throw new Error('Only three sessions are proposed');this.options=options;}
  async start(signal?:AbortSignal){
    const {batchRoot,session,native}=this.options,now=this.options.now??Date.now,root=join(batchRoot,`session-${session}`),lock=join(batchRoot,'active.lock');
    safeDirectory(batchRoot);mkdirSync(batchRoot,{recursive:true,mode:0o700});
    if(existsSync(root))throw new Error('This session start is consumed; never reset or reuse it');
    for(let prior=1;prior<session;prior++){
      const path=join(batchRoot,`session-${prior}`,'session-state.json');
      if(!existsSync(path)||lstatSync(path).isSymbolicLink())throw new Error('Previous session cleanup must be confirmed');
      const state=JSON.parse(readFileSync(path,'utf8')) as State;
      if(state.session!==prior||state.phase==='running'||state.credentialCleanup!=='deleted'||state.runtimeCleanup!=='confirmed'||state.nativeCleanup!=='confirmed')throw new Error('Previous session cleanup must be confirmed');
    }
    const owner=randomUUID();writeFileSync(lock,owner,{flag:'wx',mode:0o600});
    try{mkdirSync(root,{mode:0o700});}catch(error){unlinkSync(lock);throw error;}
    const began=now(),state:State={version:1,session,phase:'running',began,workDeadline:began+540_000,credentialDeadline:began+600_000,launches:0,restarts:0,receipts:[],credentialCleanup:'pending',runtimeCleanup:'pending',nativeCleanup:'pending'};
    const save=()=>{const temporary=join(root,`state-${randomUUID()}.tmp`);writeFileSync(temporary,JSON.stringify(state,null,2)+'\n',{flag:'wx',mode:0o600});renameSync(temporary,join(root,'session-state.json'));};
    save();
    const abort=new AbortController(),pairingFile=join(root,'local-enrollment.json');
    let accepting=false,restarting=false,runtime:PreparedRuntime|undefined,service:ReturnType<typeof createOpenAIService>|undefined,closing:Promise<State>|undefined,cancelTimer=()=>{},issuedPairing=false;
    let resolveClosed!:(state:State)=>void;const closed=new Promise<State>(resolve=>{resolveClosed=resolve;});
    const active=()=>!abort.signal.aborted&&now()<state.workDeadline;
    const budget:RuntimeBudget={workDeadline:state.workDeadline,signal:abort.signal,remainingConnections:()=>3-state.launches,claimLaunch:()=>{
      if(!active()||state.launches>=3)throw new Error('Runtime launch budget is exhausted');
      const remaining=Math.min(296_000,state.workDeadline-now()-4_000);if(remaining<1)throw new Error('No runtime cleanup reserve remains');
      state.launches++;save();return remaining;
    }};
    const readiness=():RuntimeReadiness=>{
      const current=runtime?.readiness?.()??{state:runtime?'ready':'unavailable',canRestart:false,detail:runtime?'Runtime available.':'Runtime unavailable.'};
      return {...current,canRestart:current.canRestart&&active()&&!restarting&&state.restarts<1&&state.launches<3,connectionsUsed:state.launches,maximumConnections:3};
    };
    const claimRestart=()=>{if(!active()||closing||restarting||state.restarts>=1||state.launches>=3)throw new Error('Session restart is unavailable or consumed');restarting=true;accepting=false;state.restarts++;save();};
    const proxy:ChatRuntime={
      execute:(input,signal)=>accepting&&active()&&runtime?runtime.execute(input,AbortSignal.any([signal,abort.signal])):Promise.resolve({kind:'notStarted',message:'Session dispatch is closed.'}),
      dispose:async()=>{await runtime?.dispose();},readiness,
      restart:async signal=>{
        if(!runtime?.restart||!readiness().canRestart)throw new Error('Runtime restart is unavailable');
        claimRestart();try{await runtime.restart(AbortSignal.any([signal,abort.signal]));if(!active())throw new Error('Session expired during restart');accepting=true;}
        catch(error){queueMicrotask(()=>{void finish(false);});throw error;}finally{restarting=false;}
      },
    };
    const openService=()=>createOpenAIService({dataDirectory:join(root,'service'),runtime:proxy,autoRun:true,credentialDeadline:state.credentialDeadline,now,admitSubmission:(submission,runs)=>accepting&&active()&&permitted(submission,runs,session)});
    const finish=(success=true):Promise<State>=>{
      if(closing)return closing;
      accepting=false;abort.abort();cancelTimer();signal?.removeEventListener('abort',cancel);
      closing=(async()=>{
        let passed=success,devices:string[]=[];
        try{if(service){const report=service.inspectSession();state.receipts=report.receipts;devices=report.deviceIDs;state.installationId=report.installation.installationId;if(report.runs.some(run=>!['completed','failed','canceled','closedUnknown'].includes(run.state)))passed=false;}}catch{passed=false;}
        try{
          if(!native.stop||!await native.stop())throw new Error('Native shutdown is unconfirmed');
          if(state.installationId&&devices.length>0){const observed=native.inspect(state.installationId,{allowUncreatedQueue:state.receipts.length===0});if(!observed.appExited||observed.pendingCount!==0)throw new Error('Native pending work is unresolved');}
          state.nativeCleanup='confirmed';
        }catch{state.nativeCleanup='unconfirmed';passed=false;}
        try{await service?.close();if(!service)await runtime?.dispose();state.runtimeCleanup='confirmed';}catch{state.runtimeCleanup='unconfirmed';passed=false;}
        if(state.installationId){
          let revoked=false;
          const inspectLedger=()=>{
            const path=join(root,'service','openai-candidate-b.sqlite');if(!lstatSync(path).isFile())throw new Error('Exact session ledger is missing');
            const db=new DatabaseSync(path,{readOnly:true});
            try{
              const row=db.prepare("SELECT data FROM metadata WHERE key='installation'").get(),deadline=db.prepare("SELECT data FROM metadata WHERE key='credentialDeadline'").get();
              if(typeof row?.data!=='string'||JSON.parse(row.data).installationId!==state.installationId||typeof deadline?.data!=='string'||JSON.parse(deadline.data)!==state.credentialDeadline)throw new Error('Cleanup ledger identity changed');
              return {devices:Number(db.prepare('SELECT COUNT(*) AS n FROM devices').get()?.n),enrollments:Number(db.prepare('SELECT COUNT(*) AS n FROM enrollment').get()?.n)};
            }finally{db.close();}
          };
          try{
            inspectLedger();const cleanup=createOpenAIService({dataDirectory:join(root,'service'),credentialDeadline:state.credentialDeadline,now});const report=cleanup.inspectSession();devices=[...new Set([...devices,...report.deviceIDs])];await cleanup.close();
            const remaining=inspectLedger();revoked=remaining.devices===0&&remaining.enrollments===0;
          }catch{passed=false;}
          try{const erased=await native.cleanup(state.installationId,devices);if(revoked&&erased)state.credentialCleanup='deleted';else passed=false;}catch{passed=false;}
        }else state.credentialCleanup='deleted';
        try{if(issuedPairing){unlinkSync(pairingFile);issuedPairing=false;}}catch{passed=false;}
        if(state.credentialCleanup!=='deleted')state.credentialCleanup='unconfirmed';
        if(now()>state.credentialDeadline){passed=false;state.nativeCleanup='unconfirmed';}
        state.phase=passed?'finished':'blocked';save();
        if(state.runtimeCleanup==='confirmed'&&state.nativeCleanup==='confirmed'&&state.credentialCleanup==='deleted'&&readFileSync(lock,'utf8')===owner)unlinkSync(lock);
        const result=structuredClone(state);resolveClosed(result);return result;
      })();return closing;
    };
    const cancel=()=>{void finish(false);};signal?.addEventListener('abort',cancel,{once:true});
    const schedule=this.options.scheduleStop??((stop,delay)=>{const timer=setTimeout(stop,delay);return ()=>clearTimeout(timer);});
    cancelTimer=schedule(()=>{void finish();},Math.max(1,state.workDeadline-now()));
    const launch=async()=>{
      if(!active())throw new Error('Session work deadline passed');
      const before=state.launches;runtime=this.options.launchRuntime(budget);
      if(state.launches!==before+1)throw new Error('Runtime must claim exactly one initial process launch');
      const preflight=await Promise.race([runtime.preflight(abort.signal),closed.then(()=>{throw new Error('Session closed during preflight');})]);
      if(preflight.model!=='gpt-6-astra'||preflight.runtimeVersion!=='0.157.1'||!active())throw new Error('Exact runtime preflight was not confirmed');
    };
    const restartService=async()=>{
      if(!service)throw new Error('Service restart is unavailable');
      const report=service.inspectSession();
      if(report.runs.some(run=>['queued','running','cancelRequested','waitingForApproval'].includes(run.state)))throw new Error('Resolve active or queued work before service restart');
      claimRestart();
      try{
        if(!native.stop||!await native.stop())throw new Error('Native shutdown is unconfirmed');
        const observed=native.inspect(service.installation.installationId,{allowUncreatedQueue:report.receipts.length===0});
        if(!observed.appExited||observed.pendingCount!==0)throw new Error('Native pending work prevents restart');
        await service.close({preserveSessionCredential:true});service=undefined;
        await launch();service=openService();
        if(service.installation.installationId!==state.installationId)throw new Error('Installation changed during restart');
        await service.start({port:state.port,signal:abort.signal});if(!active())throw new Error('Restart deadline passed');accepting=true;
      }catch(error){await finish(false);throw error;}finally{restarting=false;}
    };
    try{
      signal?.throwIfAborted();if(!native.stop)throw new Error('Automatic native shutdown is required');
      await this.options.prepareNative?.(root);native.assertFresh();
      service=openService();state.installationId=service.installation.installationId;save();
      await launch();await service.start({signal:abort.signal});if(!active())throw new Error('Session expired before readiness');
      state.port=Number(new URL(service.baseURL).port);save();
      const enrollment=service.issueEnrollment();writeFileSync(pairingFile,JSON.stringify({baseURL:service.baseURL,...enrollment})+'\n',{flag:'wx',mode:0o600});issuedPairing=true;accepting=true;
      return {get baseURL(){return `http://127.0.0.1:${state.port}`;},installationId:service.installation.installationId,pairingFile,workDeadline:state.workDeadline,credentialDeadline:state.credentialDeadline,closed,
        status:()=>{if(!service||closing)throw new Error('Session is closed');return service.inspectSession();},restartService,finish:()=>finish(),abort:()=>finish(false)};
    }catch(error){await finish(false);throw error;}
  }
}
