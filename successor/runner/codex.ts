import { posix } from 'node:path';
import {CODEX_PERMISSION_PROFILE,verifyPermissionSelection} from './codex-permissions.ts';
import type { RuntimeAttempt, RuntimeEvidence, RuntimeEvidenceBody, RuntimeFailure, RuntimeMessagePhase } from '../contracts/runtime.ts';

export type CodexTurnInput = { attempt: RuntimeAttempt; threadId: string; requestId: string; workspace: string; text: string };
export type CodexOutput = { writes: string[]; events: RuntimeEvidence[]; closeTransport: boolean };
const MAX_TEXT = 65_536;
const MAX_WIRE = 1_000_000;
const MAX_FRAME = 262_144;
const wire = (value: unknown) => JSON.stringify(value)+'\n';
const object = (value: unknown): Record<string,unknown> | undefined => value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string,unknown> : undefined;
const id = (value: unknown): value is string => typeof value === 'string' && /^[A-Za-z0-9_.:/-]{1,128}$/.test(value);
const exactKeys = (value: Record<string,unknown>, keys: string[]) => Object.keys(value).length === keys.length && keys.every(key=>Object.hasOwn(value,key));
const validText = (value: unknown): value is string => typeof value === 'string' && value.isWellFormed();
const requestID = (value: unknown): value is string | number => (typeof value === 'string' && /^[A-Za-z0-9_.:/-]{1,256}$/.test(value)) || (typeof value === 'number' && Number.isSafeInteger(value));
type Item = { final?: { text: string; phase: RuntimeMessagePhase } };

export class CodexTurn {
  private readonly input: CodexTurnInput;
  private sent = false;
  private terminal = false;
  private turnId: string | undefined;
  private sequence = 0;
  private interruptRequested = false;
  private interruptSent = false;
  private readonly decoder = new TextDecoder('utf-8',{fatal:true});
  private buffer = '';
  private wireBytes = 0;
  private deltaBytes = 0;
  private finalBytes = 0;
  private readonly items = new Map<string,Item>();
  private pending: Record<string,unknown>[] = [];

  constructor(input: CodexTurnInput) {
    const value=object(input); const attempt=object(value?.attempt);
    if (!value || !attempt || !exactKeys(value,['attempt','threadId','requestId','workspace','text']) ||
        !exactKeys(attempt,['runId','attemptId','generation','bindingRevisionId','runtimeVersion','selectedModel']) ||
        !['runId','attemptId','bindingRevisionId','runtimeVersion','selectedModel'].every(key=>id(attempt[key])) ||
        !Number.isSafeInteger(attempt.generation) || Number(attempt.generation)<1 || !id(value.threadId) || !id(value.requestId) ||
        !validText(value.text) || value.text.length===0 || Buffer.byteLength(value.text)>MAX_TEXT ||
        !validText(value.workspace) || !posix.isAbsolute(value.workspace) || value.workspace==='/' || value.workspace.includes('\0') ||
        posix.normalize(value.workspace)!==value.workspace || Buffer.byteLength(value.workspace)>4096) throw new Error('Invalid Codex turn input');
    this.input=structuredClone(input);
  }
  private output(): CodexOutput { return {writes:[],events:[],closeTransport:this.terminal}; }
  private emit(output:CodexOutput, body:RuntimeEvidenceBody) {
    output.events.push({protocolVersion:1,provider:'codex',...this.input.attempt,sequence:++this.sequence,...body});
  }
  private sendInterrupt(output:CodexOutput) {
    if (!this.turnId || this.interruptSent) return;
    this.interruptSent=true;
    output.writes.push(wire({id:this.input.requestId+':interrupt',method:'turn/interrupt',params:{threadId:this.input.threadId,turnId:this.turnId}}));
  }
  private stop(output:CodexOutput, reason:RuntimeFailure, transportAvailable=true) {
    if (this.terminal) return;
    if (transportAvailable) this.sendInterrupt(output);
    this.emit(output,{kind:'outcomeUnknown',reason});
    this.terminal=true; output.closeTransport=true; this.buffer=''; this.pending=[]; this.items.clear();
  }
  start(): CodexOutput {
    const output=this.output();
    if (this.sent || this.terminal) return output;
    this.sent=true;
    output.writes.push(wire({id:this.input.requestId,method:'turn/start',params:{threadId:this.input.threadId,input:[{type:'text',text:this.input.text,text_elements:[]}],model:this.input.attempt.selectedModel,cwd:this.input.workspace,environments:[],runtimeWorkspaceRoots:[],approvalPolicy:'untrusted',approvalsReviewer:'user',permissions:CODEX_PERMISSION_PROFILE}}));
    return output;
  }
  interrupt(): CodexOutput {
    const output=this.output();
    if (this.terminal) return output;
    if (!this.sent) { this.terminal=true; output.closeTransport=true; this.emit(output,{kind:'notStarted',reason:'canceledBeforeDispatch'}); return output; }
    this.interruptRequested=true; this.sendInterrupt(output); return output;
  }
  disconnected(): CodexOutput {
    const output=this.output(); this.stop(output,'connectionLost',false); return output;
  }
  receive(bytes: Uint8Array): CodexOutput {
    const output=this.output();
    if (this.terminal) return output;
    if (!this.sent) { this.stop(output,'invalidProtocol'); return output; }
    this.wireBytes+=bytes.byteLength;
    if (this.wireBytes>MAX_WIRE) { this.stop(output,'outputLimit'); return output; }
    try {
      this.buffer+=this.decoder.decode(bytes,{stream:true});
      for (;;) {
        const end=this.buffer.indexOf('\n'); if (end<0) break;
        const line=this.buffer.slice(0,end); this.buffer=this.buffer.slice(end+1);
        if (Buffer.byteLength(line)>MAX_FRAME) { this.stop(output,'outputLimit'); break; }
        if (line.trim()==='') continue;
        const message=object(JSON.parse(line));
        if (!message) { this.stop(output,'invalidProtocol'); break; }
        this.message(message,output);
        if (this.terminal) break;
      }
      if (!this.terminal && Buffer.byteLength(this.buffer)>MAX_FRAME) this.stop(output,'outputLimit');
    } catch { this.stop(output,'invalidProtocol'); }
    return output;
  }
  private message(message:Record<string,unknown>, output:CodexOutput) {
    if (Object.hasOwn(message,'method')) {
      if (typeof message.method!=='string' || Object.hasOwn(message,'result') || Object.hasOwn(message,'error')) { this.stop(output,'invalidProtocol'); return; }
      if (Object.hasOwn(message,'id')) {
        if (!requestID(message.id)) { this.stop(output,'invalidProtocol'); return; }
        output.writes.push(wire({id:message.id,error:{code:-32601,message:'Unsupported Codex server request'}}));
        this.stop(output,'unsupportedAction'); return;
      }
      this.notification(message,output); return;
    }
    if (!requestID(message.id) || Object.hasOwn(message,'result')===Object.hasOwn(message,'error')) { this.stop(output,'invalidProtocol'); return; }
    if (message.id===this.input.requestId+':interrupt' && this.interruptSent) {
      if (Object.hasOwn(message,'error')) this.stop(output,'invalidProtocol');
      return;
    }
    if (message.id!==this.input.requestId) return;
    if (Object.hasOwn(message,'error')) { this.stop(output,'startRejected'); return; }
    const turn=object(object(message.result)?.turn);
    if (!turn || !id(turn.id) || turn.status!=='inProgress') { this.stop(output,'invalidProtocol'); return; }
    if (this.turnId) { if (this.turnId!==turn.id) this.stop(output,'invalidProtocol'); return; }
    this.turnId=turn.id;
    this.emit(output,{kind:'started',threadId:this.input.threadId,turnId:this.turnId});
    if (this.interruptRequested) this.sendInterrupt(output);
    const pending=this.pending;this.pending=[];
    for(const notification of pending) { this.notification(notification,output); if(this.terminal)break; }
  }
  private notification(message:Record<string,unknown>, output:CodexOutput) {
    const method=message.method;
    if(method==='thread/settings/updated'){
      const params=object(message.params);
      if(!params||!id(params.threadId)){this.stop(output,'invalidProtocol');return;}
      if(params.threadId!==this.input.threadId)return;
      const settings=object(params.threadSettings),sandbox=object(settings?.sandboxPolicy);
      try{verifyPermissionSelection(settings?.activePermissionProfile);}
      catch{this.stop(output,'unsupportedAction');return;}
      if(settings?.cwd!==this.input.workspace||settings.model!==this.input.attempt.selectedModel||settings.modelProvider!=='openai'||
          settings.approvalPolicy!=='untrusted'||settings.approvalsReviewer!=='user'||sandbox?.type!=='readOnly'||sandbox.networkAccess!==false)this.stop(output,'unsupportedAction');
      return;
    }
    if (!['turn/started','turn/completed','item/agentMessage/delta','item/started','item/completed','model/rerouted'].includes(String(method))) return;
    const params=object(message.params);
    if (!params || !id(params.threadId)) { this.stop(output,'invalidProtocol'); return; }
    if (params.threadId!==this.input.threadId) return;
    const turn=object(params.turn);
    const turnID=(method==='turn/started' || method==='turn/completed') ? turn?.id : params.turnId;
    if (!id(turnID)) { this.stop(output,'invalidProtocol'); return; }
    if (turn && Object.hasOwn(params,'turnId') && params.turnId!==turnID) { this.stop(output,'invalidProtocol'); return; }
    if (!this.turnId) {
      if (this.pending.length===32) { this.stop(output,'outputLimit'); return; }
      this.pending.push(message);return;
    }
    if (turnID!==this.turnId) return;
    const scope={threadId:this.input.threadId,turnId:this.turnId};
    if (method==='model/rerouted') { this.stop(output,'modelChanged'); return; }
    if (method==='turn/started') { if(turn?.status!=='inProgress')this.stop(output,'invalidProtocol'); return; }
    if (method==='turn/completed') {
      const status=turn?.status;
      if (status!=='completed' && status!=='interrupted' && status!=='failed') { this.stop(output,'invalidProtocol'); return; }
      if (status==='completed' && (turn?.error!=null || this.items.size===0 || [...this.items.values()].some(item=>item.final===undefined))) { this.stop(output,'invalidProtocol'); return; }
      const messages=status==='completed' ? [...this.items].map(([itemId,item])=>({itemId,...item.final!})) : [];
      this.emit(output,{kind:'terminal',...scope,status,messages,effectiveModel:null,modelEvidence:'unreported'});
      this.terminal=true; output.closeTransport=true; this.buffer='';this.pending=[];this.items.clear(); return;
    }
    if (method==='item/agentMessage/delta') {
      if (!id(params.itemId) || !validText(params.delta)) { this.stop(output,'invalidProtocol'); return; }
      const item=this.item(params.itemId,output); if(!item)return;
      if(item.final!==undefined) { this.stop(output,'invalidProtocol'); return; }
      this.deltaBytes+=Buffer.byteLength(params.delta);
      if(this.deltaBytes>MAX_TEXT) { this.stop(output,'outputLimit'); return; }
      this.emit(output,{kind:'textDelta',...scope,itemId:params.itemId,text:params.delta});return;
    }
    const raw=object(params.item);
    if(!raw || !id(raw.id) || typeof raw.type!=='string') { this.stop(output,'invalidProtocol'); return; }
    if(['userMessage','reasoning','plan','contextCompaction'].includes(raw.type))return;
    if(raw.type!=='agentMessage') { this.stop(output,'unsupportedAction'); return; }
    const item=this.item(raw.id,output); if(!item)return;
    if(method==='item/started')return;
    if(!validText(raw.text) || (raw.phase!=null && raw.phase!=='commentary' && raw.phase!=='final_answer')) { this.stop(output,'invalidProtocol'); return; }
    const phase=raw.phase==='final_answer' ? 'finalAnswer' : raw.phase==='commentary' ? 'commentary' : 'unreported';
    if(item.final!==undefined) { if(item.final.text!==raw.text || item.final.phase!==phase)this.stop(output,'invalidProtocol'); return; }
    this.finalBytes+=Buffer.byteLength(raw.text);
    if(this.finalBytes>MAX_TEXT) { this.stop(output,'outputLimit'); return; }
    item.final={text:raw.text,phase};
  }
  private item(itemId:string, output:CodexOutput): Item | undefined {
    let item=this.items.get(itemId);
    if(!item) {
      if(this.items.size===128) { this.stop(output,'outputLimit'); return; }
      item={};this.items.set(itemId,item);
    }
    return item;
  }
}
