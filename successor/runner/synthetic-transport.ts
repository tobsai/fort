import { createHash } from 'node:crypto';
import { CodexTurn, type CodexOutput } from './codex.ts';
import type { SyntheticDispatch } from '../contracts/runner.ts';
import type { RuntimeEvidence } from '../contracts/runtime.ts';

export type SyntheticAction =
  | { kind:'handshake'; threadId:string }
  | { kind:'write'; data:string }
  | { kind:'evidence'; event:RuntimeEvidence }
  | { kind:'close' }
  | { kind:'forceClose' };

// A deterministic fixture state machine: these actions perform no I/O themselves.
export class SyntheticTurn {
  private readonly turn:CodexTurn;
  private readonly threadId:string;
  private readonly clock:()=>number;
  private phase:'idle'|'handshake'|'turn'|'interrupt'|'closing'|'closed'='idle';
  private deadline=0;
  private observedTime=0;
  constructor(input:SyntheticDispatch,clock:()=>number) {
    const dispatch=structuredClone(input);const binding=dispatch.binding;
    if(dispatch.mode!=='synthetic'||dispatch.protocol!=='codex-turn'||dispatch.workspace!=='/synthetic/workspace'||!binding?.enabled||
      binding.provider!=='fixture'||binding.runtimeVersion!=='1'||binding.selectedModel!=='fixture-echo-v1'||binding.hostId!=='fixture-local'||
      dispatch.hostId!==binding.hostId||dispatch.attempt.bindingRevisionId!==binding.bindingRevisionId||
      dispatch.attempt.selectedModel!==binding.selectedModel||dispatch.attempt.runtimeVersion!==binding.runtimeVersion||
      typeof dispatch.text!=='string'||dispatch.inputDigest!==createHash('sha256').update(dispatch.text,'utf8').digest('hex'))throw new Error('Invalid synthetic dispatch');
    this.turn=new CodexTurn({attempt:dispatch.attempt,threadId:dispatch.threadId,requestId:dispatch.requestId,workspace:dispatch.workspace,text:dispatch.text});
    this.threadId=dispatch.threadId;this.clock=clock;this.time();
  }
  private time() {
    const now=this.clock();if(!Number.isSafeInteger(now)||now<this.observedTime)throw new Error('Synthetic clock must be monotonic.');
    this.observedTime=now;return now;
  }
  private apply(output:CodexOutput):SyntheticAction[] {
    const actions:SyntheticAction[]=[...output.writes.map(data=>({kind:'write' as const,data})),...output.events.map(event=>({kind:'evidence' as const,event}))];
    if(output.closeTransport&&this.phase!=='closing'&&this.phase!=='closed') {
      this.phase='closing';this.deadline=this.time()+2000;actions.push({kind:'close'});
    }
    return actions;
  }
  begin():SyntheticAction[] {
    if(this.phase!=='idle')return [];
    this.phase='handshake';this.deadline=this.time()+5000;return [{kind:'handshake',threadId:this.threadId}];
  }
  ready(threadId:string):SyntheticAction[] {
    if(this.phase!=='handshake')return [];
    if(this.time()>=this.deadline)return this.tick();
    if(threadId!==this.threadId)return this.apply(this.turn.disconnected());
    this.phase='turn';this.deadline=this.time()+30_000;return this.apply(this.turn.start());
  }
  receive(bytes:Uint8Array):SyntheticAction[] {
    if(this.phase==='closing'||this.phase==='closed')return [];
    if(this.time()>=this.deadline&&this.phase!=='idle')return this.tick();
    return this.apply(this.turn.receive(bytes));
  }
  cancel():SyntheticAction[] {
    if(this.phase==='closing'||this.phase==='closed')return [];
    if(this.phase!=='idle'&&this.time()>=this.deadline)return this.tick();
    if(this.phase==='interrupt')return [];
    const output=this.turn.interrupt();const actions=this.apply(output);
    if(!output.closeTransport){this.phase='interrupt';this.deadline=this.time()+2000;}
    return actions;
  }
  disconnected():SyntheticAction[] {
    if(this.phase==='closing'||this.phase==='closed')return [];
    return this.apply(this.turn.disconnected());
  }
  teardownComplete():SyntheticAction[] {
    const actions=this.disconnected();this.phase='closed';return actions;
  }
  tick():SyntheticAction[] {
    if(this.phase==='idle'||this.phase==='closed'||this.time()<this.deadline)return [];
    if(this.phase==='closing'){this.phase='closed';return [{kind:'forceClose'}];}
    const stopping=this.phase==='interrupt';
    const actions=[...(this.phase==='handshake'?[]:this.apply(this.turn.interrupt())),...this.apply(this.turn.disconnected())];
    if(stopping){this.phase='closed';actions.push({kind:'forceClose'});}
    return actions;
  }
}
