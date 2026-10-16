import { mkdirSync,readFileSync,writeFileSync,renameSync,lstatSync,existsSync,unlinkSync } from 'node:fs';
import { join,isAbsolute } from 'node:path';
import { randomUUID } from 'node:crypto';
import { createOpenAIService,type SubmissionAdmission } from '../service/openai.ts';
import type { ChatRuntime } from '../runner/chat-runtime.ts';
import type { Acceptance,Run } from '../contracts/v1.ts';

export const SAMPLES = [
  { text:'Remember the sample word cedar. Reply with that word only.',agent:'Writer' },
  { text:'In one sentence, explain why a checklist is useful.',agent:'Reviewer' },
  { text:'Suggest one short title for a sample packing list.',agent:'Writer' },
  { text:'Our sample plan is to pack a red notebook. Repeat the item.',agent:'Reviewer' },
  { text:'What sample word did I ask you to remember earlier? Reply with that word only.',agent:'Writer' },
] as const;
type PreparedRuntime = ChatRuntime & { preflight(signal:AbortSignal):Promise<{model:string;runtimeVersion:string}> };
export type NativeSession = {
  assertFresh():void;
  inspect(installationId:string):{ appExited:boolean; pendingCount:number };
  cleanup(installationId:string,issuedDeviceIDs?:readonly string[]):Promise<boolean>;
};
type Receipt=Acceptance['receipt'];
type State={ version:1; starts:number; phase:'running'|'between'|'finished'|'blocked'; credentialDeadline:number; installationId?:string; port?:number; receipts:Receipt[]; credentialCleanup?:'retained-for-restart'|'deleted'|'unconfirmed' };
type Options={ root:string; native:NativeSession; launchRuntime:(remainingWorkMs:number)=>PreparedRuntime; now?:()=>number; scheduleStop?:(stop:()=>void,delayMs:number)=>()=>void };

function allowed(submission:SubmissionAdmission,runs:readonly Run[],session:number):boolean {
  const index=runs.length,sample=SAMPLES[index],config=submission.configuration,conversation=submission.conversation;
  if (!sample || index>=(session===1?4:5) || (session===2&&index!==4) || runs.some(run=>run.state!=='completed')) return false;
  if (submission.text!==sample.text || config.agentName!==sample.agent || config.selectedModel!=='gpt-6-astra') return false;
  if (config.instructions!==(sample.agent==='Writer'?'Reply briefly using supplied context.':'Check the sample claim.')) return false;
  const prior=runs.find(run=>run.configuration?.agentName===sample.agent);
  if (prior && (prior.configuration?.agentId!==config.agentId || prior.bindingRevisionId!==config.bindingRevisionId)) return false;
  if (index===3) {
    const writer=runs.find(run=>run.configuration?.agentName==='Writer')?.configuration?.agentId;
    if (conversation.kind!=='shared' || conversation.name!=='Sample planning' || conversation.participantAgentIds.length!==2 || !writer || !conversation.participantAgentIds.includes(writer) || !conversation.participantAgentIds.includes(config.agentId)) return false;
  } else if (conversation.kind!=='home' || conversation.participantAgentIds.length!==1 || conversation.participantAgentIds[0]!==config.agentId || (prior && prior.conversationId!==conversation.conversationId)) return false;
  return index===2
    ? config.skills.length===1 && config.skills[0]?.name==='Check marker' && config.skills[0]?.instructions==='End with [CHECKED].'
    : config.skills.length===0;
}

export class CandidatePacket {
  private readonly options:Options;
  private readonly statePath:string;
  constructor(options:Options) {
    if (!isAbsolute(options.root) || options.root==='/' || options.root.includes('\0')) throw new Error('Use an explicit packet directory');
    this.options=options;this.statePath=join(options.root,'session-state.json');
  }
  private read():State|undefined {
    if (!existsSync(this.statePath)) return;
    if (lstatSync(this.statePath).isSymbolicLink()) throw new Error('Packet state cannot be a symlink');
    const state=JSON.parse(readFileSync(this.statePath,'utf8')) as State;
    if (state.version!==1 || ![1,2].includes(state.starts) || !['running','between','finished','blocked'].includes(state.phase) || !Number.isSafeInteger(state.credentialDeadline) || !Array.isArray(state.receipts)) throw new Error('Invalid packet state');
    return state;
  }
  private save(state:State) {
    const temporary=this.statePath+'.'+randomUUID();
    writeFileSync(temporary,JSON.stringify(state,null,2)+'\n',{ mode:0o600,flag:'wx' });renameSync(temporary,this.statePath);
  }
  async cancelBetweenSessions() {
    const state=this.read();
    if(!state||state.phase!=='between')return;
    const lock=join(this.options.root,'active.lock');
    try{writeFileSync(lock,'local cleanup',{flag:'wx',mode:0o600});}catch{throw new Error('Packet ownership is unresolved');}
    try{await this.discardHandoff(state);}finally{unlinkSync(lock);}
  }
  private async discardHandoff(state:State) {
    state.phase='blocked';state.credentialCleanup='unconfirmed';this.save(state);
    try {
      const service=createOpenAIService({dataDirectory:join(this.options.root,'service'),now:this.options.now});
      const report=service.inspectSession();await service.close();
      if(await this.options.native.cleanup(report.installation.installationId,report.deviceIDs))state.credentialCleanup='deleted';
    } finally {this.save(state);}
  }
  async start() {
    const root=this.options.root;
    mkdirSync(root,{recursive:true,mode:0o700});
    if(lstatSync(root).isSymbolicLink())throw new Error('Packet directory cannot be a symlink');
    const lock=join(root,'active.lock'),owner=randomUUID();
    try {writeFileSync(lock,owner,{flag:'wx',mode:0o600});} catch {throw new Error('Packet session is already active or its ownership is unresolved');}
    let released=false;
    const release=()=>{if(!released&&readFileSync(lock,'utf8')===owner){unlinkSync(lock);released=true;}};
    try{return await this.startLocked(release);}catch(error){release();throw error;}
  }
  private async startLocked(release:()=>void) {
    const { root,native,launchRuntime }=this.options,now=this.options.now??Date.now;
    mkdirSync(root,{ recursive:true,mode:0o700 });
    if (lstatSync(root).isSymbolicLink()) throw new Error('Packet directory cannot be a symlink');
    const previous=this.read();
    if (previous && (previous.phase!=='between' || previous.starts!==1)) throw new Error('Packet cannot start another session');
    if (previous) {
      try {
        const observation=native.inspect(previous.installationId!);
        if (!observation.appExited || observation.pendingCount!==0 || previous.receipts.length!==4 || previous.credentialDeadline<=now()) throw new Error('Restart gate is not confirmed');
      } catch {await this.discardHandoff(previous);throw new Error('Restart gate failed; the packet is blocked.');}
    } else {
      native.assertFresh();
      if (existsSync(join(root,'service'))) throw new Error('First session requires a fresh ledger');
    }
    const began=now();
    const state:State={ version:1,starts:previous?2:1,phase:'running',credentialDeadline:previous?.credentialDeadline??began+600_000,receipts:previous?.receipts??[],...(previous?.installationId?{installationId:previous.installationId}:{}),...(previous?.port?{port:previous.port}:{}) };
    const workDeadline=Math.min(began+292_000,state.credentialDeadline-8_000);
    this.save(state);
    const abort=new AbortController();let runtime:PreparedRuntime|undefined,accepting=false,closing:Promise<State>|undefined;
    let resolveClosed!:(state:State)=>void;
    const closed=new Promise<State>(resolve=>{resolveClosed=resolve;});
    const runtimeProxy:ChatRuntime={
      execute:async(input,signal)=> !accepting || now()>=workDeadline || !runtime ? {kind:'notStarted',message:'Session dispatch is closed.'} : runtime.execute(input,AbortSignal.any([signal,abort.signal])),
      dispose:async()=>{ await runtime?.dispose(); },
    };
    const service=createOpenAIService({ dataDirectory:join(root,'service'),runtime:runtimeProxy,autoRun:true,credentialDeadline:state.credentialDeadline,now,
      admitSubmission:(submission,runs)=>accepting && now()<workDeadline && allowed(submission,runs,state.starts),
    });
    const finish=(requestCompletion=true):Promise<State>=>{
      if (closing) return closing;
      accepting=false;abort.abort();cancelTimer();
      closing=(async()=>{
        const report=service.inspectSession(),expected=state.starts===1?4:5;
        let passed=requestCompletion && now()<=workDeadline && report.receipts.length===expected && report.runs.length===expected && report.runs.every(run=>run.state==='completed');
        if (passed) {
          try { const observed=native.inspect(service.installation.installationId);passed=observed.appExited&&observed.pendingCount===0; } catch { passed=false; }
        }
        try { await service.close({ preserveSessionCredential:passed&&state.starts===1 }); } catch { passed=false; }
        if (passed) {
          try { const observed=native.inspect(service.installation.installationId);passed=observed.appExited&&observed.pendingCount===0&&now()<=began+300_000; } catch { passed=false; }
        }
        state.receipts=report.receipts;state.installationId=service.installation.installationId;
        state.phase=passed ? state.starts===1?'between':'finished' : 'blocked';
        state.credentialCleanup=state.phase==='between'?'retained-for-restart':'unconfirmed';
        if (state.phase!=='between') {
          try { const cleanup=createOpenAIService({ dataDirectory:join(root,'service'),now });await cleanup.close(); } catch { state.phase='blocked'; }
          try { if (!await native.cleanup(service.installation.installationId,report.deviceIDs)) state.phase='blocked';else state.credentialCleanup='deleted'; } catch { state.phase='blocked'; }
        }
        this.save(state);release();const result=structuredClone(state);resolveClosed(result);return result;
      })();return closing;
    };
    const schedule=this.options.scheduleStop??((stop,delay)=>{const timer=setTimeout(stop,delay);return ()=>clearTimeout(timer);});
    const cancelTimer=schedule(()=>{ void finish(false).catch(()=>{}); },Math.max(1,workDeadline-now()));
    try {
      if(now()>=workDeadline)throw new Error('No session work budget remains');
      if (previous) {
        const report=service.inspectSession();
        if (report.installation.installationId!==previous.installationId || report.runs.length!==4 || report.receipts.length!==4 || report.runs.some(run=>run.state!=='completed') || JSON.stringify(report.receipts)!==JSON.stringify(previous.receipts)) throw new Error('Stored first-session completions changed');
      }
      runtime=launchRuntime(Math.min(workDeadline,state.credentialDeadline)-now());
      const preflight=await runtime.preflight(abort.signal);
      if (preflight.model!=='gpt-6-astra' || preflight.runtimeVersion!=='0.157.1' || now()>=workDeadline || abort.signal.aborted) throw new Error('Session preflight did not complete within its budget');
      await service.start({ port:previous?.port??0 });
      state.installationId=service.installation.installationId;state.port=Number(new URL(service.baseURL).port);this.save(state);
      accepting=true;
      return { baseURL:service.baseURL,installationId:service.installation.installationId,enrollment:state.starts===1?service.issueEnrollment():undefined,
        finish:()=>finish(true),abort:()=>finish(false),status:()=>service.inspectSession(),closed,workDeadline,credentialDeadline:state.credentialDeadline };
    } catch {
      await finish(false);throw new Error('Session preflight failed; no retry or extra start is authorized.');
    }
  }
}
