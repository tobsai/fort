import { randomUUID } from 'node:crypto';
import { isAbsolute, normalize } from 'node:path';
import { CodexTurn, type CodexOutput } from './codex.ts';
import { CodexConnection, object } from './codex-rpc.ts';
import type { ChatInput, ChatResult, ChatRuntime } from './chat-runtime.ts';
import { readRuntimePages, verifyRuntimeControls } from './codex-controls.ts';
import {StartupCheckError,redactedStartupFailure,type StartupDiagnostic} from './startup-diagnostic.ts';
import {CODEX_PERMISSION_PROFILE,verifyPermissionSelection} from './codex-permissions.ts';

export class CodexChat implements ChatRuntime {
  private readonly options: { connection: CodexConnection; workspace: string; runtimeVersion: string };
  private initialized = false;
  private busy = false;
  constructor(options: { connection: CodexConnection; workspace: string; runtimeVersion: string }) {
    if (options.runtimeVersion !== '0.157.1') throw new Error('Unreviewed runtime version');
    if (!isAbsolute(options.workspace) || options.workspace === '/' || normalize(options.workspace) !== options.workspace) throw new Error('Use an explicit empty workspace');
    this.options = options;
  }
  dispose(): Promise<void> { return this.options.connection.dispose(); }
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
      connection.close();
      throw new StartupCheckError(redactedStartupFailure(error,checkpoint));
    }
  }
  async execute(input: ChatInput, signal: AbortSignal): Promise<ChatResult> {
    if (this.busy) return { kind:'failed', message:'This runtime already has an active turn.' };
    for(const skill of input.configuration.skills){
      const bundle=skill.bundle;
      if(bundle&&(skill.review.state!=='enabled'||!bundle.supportedRuntimes.includes('codex')||bundle.requestedTools.length!==0||bundle.scripts.length!==0||bundle.files.length!==1||bundle.files[0]?.path!==bundle.entrypoint||bundle.files[0].executable)){
        return {kind:'notStarted',message:'This no-tool Codex runtime cannot preserve the selected bundle files, scripts, tools or declared runtime. Nothing was dispatched.'};
      }
    }
    const developerInstructions = input.configuration.instructions + input.configuration.skills.map(skill => `\n\nSelected instruction skill: ${skill.name}\n${skill.instructions}`).join('');
    const text = JSON.stringify({ conversationId:input.conversationId, history:input.history.map(message => ({ role:message.role, text:message.text, agentId:message.agentId, agentName:message.agentName, recipientAgentId:message.recipientAgentId, recipientAgentName:message.recipientAgentName })), currentMessage:{ recipientAgentId:input.configuration.agentId, text:input.text } });
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
        resolve(result); if (close) connection.close();
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
