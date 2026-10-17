import type { RuntimeReadiness } from '../contracts/v1.ts';
import { randomUUID } from 'node:crypto';
import { prepareSkillContext } from './skill-context.ts';
import { isAbsolute, normalize } from 'node:path';
import { CodexTurn, type CodexOutput } from './codex.ts';
import { CodexConnection, object } from './codex-rpc.ts';
import type { ChatInput, ChatResult, ChatRuntime } from './chat-runtime.ts';
import { readRuntimePages, verifyRuntimeControls } from './codex-controls.ts';
import {StartupCheckError,redactedStartupFailure,type StartupDiagnostic} from './startup-diagnostic.ts';
import {CODEX_PERMISSION_PROFILE,verifyPermissionSelection} from './codex-permissions.ts';

type CodexChatOptions={ connection:CodexConnection; workspace:string; runtimeVersion:string; replacement?:{ maximumConnections:number; deadline:number; now?:()=>number; connect:()=>CodexConnection } };
export class CodexChat implements ChatRuntime {
  private readonly options: CodexChatOptions;
  private initialized = false;
  private busy = false;
  private disposed = false;
  private recoveryBlocked = false;
  private connectionsUsed = 1;
  private detail = 'Ready for the exact selected model.';
  constructor(options: CodexChatOptions) {
    if (options.runtimeVersion !== '0.157.1') throw new Error('Unreviewed runtime version');
    if (!isAbsolute(options.workspace) || options.workspace === '/' || normalize(options.workspace) !== options.workspace) throw new Error('Use an explicit empty workspace');
    if(options.replacement){const r=options.replacement,now=(r.now??Date.now)();if(!Number.isInteger(r.maximumConnections)||r.maximumConnections<1||r.maximumConnections>3||!Number.isSafeInteger(r.deadline)||r.deadline<=now||r.deadline>now+540_000)throw new Error('Use an explicit bounded replacement budget');}
    this.options = options;
  }
  async dispose(): Promise<void> { this.disposed=true;await this.options.connection.dispose(); }
  readiness():RuntimeReadiness {
    const replacement=this.options.replacement,closed=this.options.connection.isClosed;
    return {state:this.disposed?'stopped':this.busy?'busy':closed?'restartRequired':'ready',canRestart:!this.disposed&&!this.busy&&!this.recoveryBlocked&&closed&&replacement!==undefined&&this.connectionsUsed<replacement.maximumConnections&&(replacement.now??Date.now)()<replacement.deadline,detail:this.disposed?'Runtime stopped.':this.detail,connectionsUsed:this.connectionsUsed,...(replacement?{maximumConnections:replacement.maximumConnections}:{})};
  }
  async restart(signal:AbortSignal):Promise<void> {
    const replacement=this.options.replacement;
    if(!replacement||!this.readiness().canRestart)throw new Error('Runtime replacement is unavailable or its budget is exhausted');
    signal=AbortSignal.any([signal,AbortSignal.timeout(Math.max(1,replacement.deadline-(replacement.now??Date.now)()))]);
    signal.throwIfAborted();this.busy=true;this.detail='Stopping the previous runtime before replacement.';
    let removeAbort=()=>{};
    try {
      try {
        await Promise.race([this.options.connection.dispose(),new Promise<never>((_,reject)=>{const abort=()=>reject(new Error('Runtime cleanup was not confirmed before cancellation'));signal.addEventListener('abort',abort,{once:true});removeAbort=()=>signal.removeEventListener('abort',abort);if(signal.aborted)abort();})]);
      } catch(error){this.recoveryBlocked=true;this.detail='Previous runtime cleanup is unconfirmed. Replacement is blocked.';throw error;}
      signal.throwIfAborted();if((replacement.now??Date.now)()>=replacement.deadline)throw new Error('Runtime replacement deadline expired');
      this.connectionsUsed++;
      this.options.connection=replacement.connect();this.initialized=false;
      await this.preflight(signal);this.detail='Replacement verified. Earlier requests were not retried.';
    } finally {removeAbort();this.busy=false;}
  }
  async preflight(signal: AbortSignal, model = 'gpt-6-astra'): Promise<{ model:string; runtimeVersion:string }> {
    const { connection,workspace,runtimeVersion }=this.options;
    let checkpoint:StartupDiagnostic={stage:'protocol',check:'initialize'};
    try {
      if (!this.initialized) {
        await connection.request('initialize',{ clientInfo:{ name:'fort_candidate_b',title:'Fort',version:'0.1.0' },capabilities:{ experimentalApi:true,requestAttestation:false } },signal);
        connection.notify('initialized',{}); this.initialized=true;
      }
      await verifyRuntimeControls(connection,workspace,signal);
      checkpoint={stage:'protocol',check:'account/read'};
      const account=object(await connection.request('account/read',{ refreshToken:false },signal));
      if (object(account.account).type!=='chatgpt' || account.requiresOpenaiAuth!==true) throw new StartupCheckError({stage:'account',check:'chatgpt'});
      checkpoint={stage:'protocol',check:'model/list'};
      const models=await readRuntimePages(connection,'model/list',{},signal);
      if (!models.some(value => object(value).model===model)) throw new StartupCheckError({stage:'model',check:'availability'});
      return { model,runtimeVersion };
    } catch(error) {
      const failure=redactedStartupFailure(error,checkpoint);this.detail=failure.stage==='model'?'The exact selected model is unavailable. Runtime restart is required before a new request.':'Runtime setup stopped. An explicit restart is required before a new request.';
      connection.close();
      throw new StartupCheckError(redactedStartupFailure(error,checkpoint));
    }
  }
  async execute(input: ChatInput, signal: AbortSignal): Promise<ChatResult> {
    if(this.disposed||this.options.connection.isClosed)return {kind:'notStarted',message:'Runtime restart is required before a new request. This request was not dispatched.'};
    if (this.busy) return { kind:'failed', message:'This runtime already has an active turn.' };
    const memory=input.configuration.memory;
    if(memory&&(memory.conversationId!==input.conversationId||!memory.revisionId||typeof memory.text!=='string'||!memory.text.isWellFormed()||Buffer.byteLength(memory.text)>8192))return {kind:'notStarted',message:'Conversation memory identity or size was not confirmed. Nothing was sent.'};
    const skills = prepareSkillContext(input.configuration.skills);
    if (skills.kind === 'unsupported') return {kind:'notStarted',message:skills.message};
    const developerInstructions = input.configuration.instructions + (skills.instructions ? '\n\n' + skills.instructions : '');
    const text = JSON.stringify({ conversationId:input.conversationId,conversationMemory:memory, history:input.history.map(message => ({ role:message.role, text:message.text, agentId:message.agentId, agentName:message.agentName, recipientAgentId:message.recipientAgentId, recipientAgentName:message.recipientAgentName })), currentMessage:{ recipientAgentId:input.configuration.agentId, text:input.text },selectedSkillBundles:skills.bundles });
    if (input.history.length > 256 || input.history.some(m => m.conversationId !== input.conversationId) || !text.isWellFormed() || Buffer.byteLength(text)+Buffer.byteLength(developerInstructions) > 65_536) return { kind:'failed', message:'Conversation context exceeds this candidate’s limit or identity. Nothing was sent.' };
    if (signal.aborted) return { kind:'notStarted', message:'Canceled before runtime dispatch.' };
    this.busy = true;
    const { connection, workspace } = this.options;
    try {
      await this.preflight(signal,input.configuration.selectedModel);
      const started = object(await connection.request('thread/start', { model:input.configuration.selectedModel, modelProvider:'openai', cwd:workspace, approvalPolicy:'untrusted', approvalsReviewer:'user', permissions:CODEX_PERMISSION_PROFILE, runtimeWorkspaceRoots:[], ephemeral:true, environments:[], dynamicTools:[], selectedCapabilityRoots:[], developerInstructions }, signal));
      const thread = object(started.thread);
      verifyPermissionSelection(started.activePermissionProfile);
      if (typeof thread.id !== 'string' || thread.ephemeral !== true || started.model !== input.configuration.selectedModel || started.modelProvider !== 'openai' || started.cwd !== workspace || started.approvalPolicy !== 'untrusted' || started.approvalsReviewer!=='user' || !Array.isArray(started.runtimeWorkspaceRoots) || started.runtimeWorkspaceRoots.length!==0 || object(started.sandbox).type !== 'readOnly' || object(started.sandbox).networkAccess !== false || !Array.isArray(started.instructionSources) || started.instructionSources.length !== 0) throw new Error('Thread configuration was not confirmed');
      await verifyRuntimeControls(connection,workspace,signal,thread.id);
      return await this.turn(input, thread.id, text, signal);
    } catch { connection.close(); return { kind:'notStarted', message:'Runtime preflight or thread configuration was not confirmed. Nothing was dispatched.' }; }
    finally { this.busy = false; }
  }
  private turn(input: ChatInput, threadId: string, text: string, signal: AbortSignal): Promise<ChatResult> {
    const { connection, workspace, runtimeVersion } = this.options;
    const turn = new CodexTurn({ attempt:{ runId:input.runId, attemptId:randomUUID(), generation:1, bindingRevisionId:input.configuration.bindingRevisionId, runtimeVersion, selectedModel:input.configuration.selectedModel }, threadId, requestId:randomUUID(), workspace, text });
    return new Promise(resolve => {
      let done = false; let interruptTimer: ReturnType<typeof setTimeout> | undefined;
      const finish = (result: ChatResult, close: boolean) => {
        if (done) return; done = true; clearTimeout(deadline); clearTimeout(interruptTimer); signal.removeEventListener('abort', interrupt); unsubscribe();
        resolve(result); if (close) {this.detail='Runtime connection ended. Earlier outcomes remain recorded; restart before a new request.';connection.close();}
      };
      const handle = (output: CodexOutput) => {
        try { for (const line of output.writes) connection.write(line); }
        catch { finish({ kind:'outcomeUnknown', message:'Codex transport failed.' }, true); return; }
        for (const event of output.events) {
          if (event.kind === 'outcomeUnknown') { finish({ kind:'outcomeUnknown', message:`Codex stopped: ${event.reason}.` }, true); return; }
          if (event.kind === 'terminal') {
            const reply = event.messages.filter(m => m.phase !== 'commentary').map(m => m.text).join('\n');
            if (event.status === 'completed' && reply.trim()) finish({ kind:'completed', text:reply, effectiveModel:null }, false);
            else if(event.status==='interrupted')finish({kind:'canceled',message:'Codex confirmed interruption. Earlier effects, if any, remain.'},false);
            else finish({ kind:'outcomeUnknown', message:'Codex ended without a confirmed final text reply.' }, true);
          }
        }
      };
      const interrupt = () => {
        if (done || interruptTimer) return;
        handle(turn.interrupt()); interruptTimer = setTimeout(() => finish({ kind:'outcomeUnknown', message:'Codex interruption was not confirmed within the deadline.' }, true), 2_000);
      };
      const deadline = setTimeout(interrupt, 60_000);
      const unsubscribe = connection.subscribe({ frame:bytes => handle(turn.receive(bytes)), closed:() => handle(turn.disconnected()) });
      signal.addEventListener('abort', interrupt, { once:true });
      if (signal.aborted) { finish({ kind:'notStarted', message:'Canceled before turn dispatch.' }, true); return; }
      handle(turn.start());
    });
  }
}
