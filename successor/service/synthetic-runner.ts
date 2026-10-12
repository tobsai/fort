import type { DatabaseSync } from 'node:sqlite';
import type { IncomingMessage } from 'node:http';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import type { Installation, Agent, Run, Message, Binding } from '../contracts/v1.ts';
import type { RuntimeEvidence } from '../contracts/runtime.ts';
import type { RunnerScope, SyntheticDispatch, RunnerReceipt } from '../contracts/runner.ts';

type Context = {
  db: DatabaseSync; installation: Installation; now(): number; timestamp(): string;
  checkpoint(point: string): void; transaction<T>(work: () => T): T;
  agentFor(conversation: string): Agent; getRun(id: string): Run; updateRun(run: Run): unknown;
  cursor(): number; appendEvent(conversation: string, entity: string, kind: string, data: unknown): number;
};
export class RunnerError extends Error {
  readonly status: number; readonly code: string;
  constructor(status: number, code: string) { super(code); this.status=status;this.code=code; }
}
function deny(status:number,code:string):never { throw new RunnerError(status,code); }
const hash=(value:string)=>createHash('sha256').update(value,'utf8').digest('hex');
const token=()=>randomBytes(32).toString('base64url');

const record=(value:unknown):Record<string,unknown>=> {
  if(!value||typeof value!=='object'||Array.isArray(value))deny(400,'invalidRunnerEvidence');
  return value as Record<string,unknown>;
};
const exact=(value:Record<string,unknown>,keys:string[])=>{
  if(Object.keys(value).length!==keys.length||!keys.every(key=>Object.hasOwn(value,key)))deny(400,'invalidRunnerEvidence');
};
const id=(value:unknown):value is string=>typeof value==='string'&&/^[A-Za-z0-9_.:/-]{1,128}$/.test(value);
const text=(value:unknown):value is string=>typeof value==='string'&&value.isWellFormed()&&Buffer.byteLength(value)<=65_536;
const canonical=(value:unknown):string=>{
  if(Array.isArray(value))return '['+value.map(canonical).join(',')+']';
  if(value&&typeof value==='object')return '{'+Object.entries(value).sort(([a],[b])=>a<b?-1:a>b?1:0).map(([key,item])=>JSON.stringify(key)+':'+canonical(item)).join(',')+'}';
  return JSON.stringify(value);
};
function evidence(value:unknown):RuntimeEvidence {
  const event=record(value);
  const base=['protocolVersion','provider','sequence','runId','attemptId','generation','bindingRevisionId','runtimeVersion','selectedModel','kind'];
  if(event.protocolVersion!==1||event.provider!=='codex'||!['runId','attemptId','bindingRevisionId','runtimeVersion','selectedModel'].every(key=>id(event[key]))||
    !Number.isSafeInteger(event.sequence)||Number(event.sequence)<1||Number(event.sequence)>4096||!Number.isSafeInteger(event.generation)||Number(event.generation)<1)deny(400,'invalidRunnerEvidence');
  if(event.kind==='started'||event.kind==='textDelta'||event.kind==='terminal') {
    if(!id(event.threadId)||!id(event.turnId))deny(400,'invalidRunnerEvidence');
    base.push('threadId','turnId');
  }
  switch(event.kind) {
    case 'started': exact(event,base);break;
    case 'textDelta':
      exact(event,[...base,'itemId','text']);if(!id(event.itemId)||!text(event.text))deny(400,'invalidRunnerEvidence');break;
    case 'terminal': {
      exact(event,[...base,'status','messages','effectiveModel','modelEvidence']);
      if(!['completed','interrupted','failed'].includes(event.status as string)||event.effectiveModel!==null||event.modelEvidence!=='unreported'||!Array.isArray(event.messages)||event.messages.length>128)deny(400,'invalidRunnerEvidence');
      let bytes=0;const seen=new Set<string>();
      for(const item of event.messages) {
        const message=record(item);exact(message,['itemId','text','phase']);
        if(!id(message.itemId)||seen.has(message.itemId)||!text(message.text)||!['commentary','finalAnswer','unreported'].includes(message.phase as string))deny(400,'invalidRunnerEvidence');
        seen.add(message.itemId);bytes+=Buffer.byteLength(message.text);
      }
      if(bytes>65_536||(event.status==='completed'?!event.messages.some(item=>item.phase!=='commentary'):event.messages.length!==0))deny(400,'invalidRunnerEvidence');
      break;
    }
    case 'outcomeUnknown':
      exact(event,[...base,'reason']);if(!['invalidProtocol','outputLimit','connectionLost','startRejected','unsupportedAction','modelChanged'].includes(event.reason as string))deny(400,'invalidRunnerEvidence');break;
    case 'notStarted':exact(event,[...base,'reason']);if(event.reason!=='canceledBeforeDispatch')deny(400,'invalidRunnerEvidence');break;
    default:deny(400,'invalidRunnerEvidence');
  }
  return event as RuntimeEvidence;
}

export function createSyntheticRunnerBridge(c:Context) {
  const {db,installation}=c;
  db.exec(`CREATE TABLE IF NOT EXISTS runner_hosts (id TEXT PRIMARY KEY, enabled INTEGER NOT NULL);
    CREATE TABLE IF NOT EXISTS runner_sessions (id TEXT PRIMARY KEY, host TEXT NOT NULL REFERENCES runner_hosts(id), token_hash TEXT UNIQUE NOT NULL, installation TEXT NOT NULL, epoch TEXT NOT NULL, expires INTEGER NOT NULL, revoked INTEGER NOT NULL DEFAULT 0);
    CREATE TABLE IF NOT EXISTS synthetic_dispatches (attempt TEXT PRIMARY KEY REFERENCES attempts(id), session TEXT NOT NULL REFERENCES runner_sessions(id), data TEXT NOT NULL, turn_id TEXT, last_sequence INTEGER NOT NULL DEFAULT 0);
    CREATE TABLE IF NOT EXISTS runner_evidence (attempt TEXT NOT NULL REFERENCES synthetic_dispatches(attempt), sequence INTEGER NOT NULL, fingerprint TEXT NOT NULL, data TEXT NOT NULL, cursor INTEGER NOT NULL, PRIMARY KEY(attempt,sequence));`);
  const enroll=(hostId:string)=>c.transaction(()=>{
    if(hostId!=='fixture-local')deny(400,'unsupportedSyntheticHost');
    db.prepare('INSERT INTO runner_hosts VALUES (?,1) ON CONFLICT(id) DO UPDATE SET enabled=1').run(hostId);
    const sessionId=randomUUID();const credential=token();
    db.prepare('INSERT INTO runner_sessions(id,host,token_hash,installation,epoch,expires) VALUES (?,?,?,?,?,?)').run(sessionId,hostId,hash(credential),installation.installationId,installation.recoveryEpoch,c.now()+300_000);
    return {sessionId,token:credential};
  });
  const authenticate=(request:IncomingMessage,input:Record<string,unknown>):RunnerScope=>{
    if(request.headers.origin!==undefined)deny(403,'runnerOriginDenied');
    const bearer=/^Bearer ([A-Za-z0-9_-]{43})$/.exec(request.headers.authorization??'');
    const session=bearer&&db.prepare('SELECT s.*,h.enabled FROM runner_sessions s JOIN runner_hosts h ON h.id=s.host WHERE token_hash=?').get(hash(bearer[1]));
    if(!session||session.installation!==installation.installationId||session.epoch!==installation.recoveryEpoch||session.revoked||!session.enabled||Number(session.expires)<=c.now()||input.protocolVersion!==1||
      request.headers['x-fort-installation-id']!==installation.installationId||input.installationId!==installation.installationId||
      input.recoveryEpoch!==installation.recoveryEpoch||input.sessionId!==session.id||input.hostId!==session.host)deny(401,'runnerUnauthorized');
    return {protocolVersion:1,installationId:installation.installationId,recoveryEpoch:installation.recoveryEpoch,sessionId:String(session.id),hostId:String(session.host)};
  };
  const expireDue=()=>{
    for(const row of db.prepare("SELECT r.data FROM runs r JOIN attempts a ON a.id=json_extract(r.data,'$.attemptId') JOIN outbox o ON o.run=r.id JOIN synthetic_dispatches s ON s.attempt=a.id WHERE o.state='claimed' AND a.lease_expires<=?").all(c.now())) {
      const run:Run=JSON.parse(String(row.data));run.state='outcomeUnknown';c.updateRun(run);
      db.prepare("UPDATE outbox SET state='outcomeUnknown' WHERE run=?").run(run.runId);
      c.appendEvent(run.conversationId,run.runId,'run.updated',{run,reason:'syntheticRunnerLeaseExpired'});
    }
  };
  const claim=(scope:RunnerScope):SyntheticDispatch|null=>{
    const row=db.prepare(`SELECT r.data FROM runs r JOIN outbox o ON o.run=r.id WHERE o.state='queued'
      AND NOT EXISTS (SELECT 1 FROM runs prior WHERE prior.conversation=r.conversation AND prior.rowid<r.rowid AND prior.state NOT IN ('completed','canceled','closedUnknown')) ORDER BY r.rowid LIMIT 1`).get();
    if(!row)return null;
    const run:Run=JSON.parse(String(row.data));
    const binding:Binding=JSON.parse(String(db.prepare('SELECT data FROM run_bindings WHERE run=?').get(run.runId)?.data));
    const current=c.agentFor(run.conversationId).binding;
    if(binding.hostId!==scope.hostId||!current.enabled||JSON.stringify(current)!==JSON.stringify(binding))deny(409,'runnerBindingConflict');
    const user=db.prepare("SELECT data FROM messages WHERE request=? AND json_extract(data,'$.role')='user'").get(run.requestId)!;
    const text=String(JSON.parse(String(user.data)).text);const attemptId=randomUUID();const credential=token();
    const dispatch:SyntheticDispatch={...scope,mode:'synthetic',protocol:'codex-turn',dispatchToken:credential,
      attempt:{runId:run.runId,attemptId,generation:1,bindingRevisionId:binding.bindingRevisionId,runtimeVersion:binding.runtimeVersion,selectedModel:binding.selectedModel},
      binding,inputDigest:hash(text),threadId:'synthetic-'+run.runId,requestId:'start-'+attemptId,workspace:'/synthetic/workspace',text};
    db.prepare('INSERT INTO attempts VALUES (?,?,?,?,?)').run(attemptId,run.runId,1,hash(credential),c.now()+40_000);
    const {dispatchToken:_,...stored}=dispatch;
    db.prepare('INSERT INTO synthetic_dispatches(attempt,session,data) VALUES (?,?,?)').run(attemptId,scope.sessionId,JSON.stringify(stored));
    run.attemptId=attemptId;c.updateRun(run);db.prepare("UPDATE outbox SET state='claimed' WHERE run=?").run(run.runId);
    c.checkpoint('syntheticBeforeClaimCommit');return dispatch;
  };
  const correlate=(scope:RunnerScope,input:Record<string,unknown>,event:RuntimeEvidence)=>{
    const row=db.prepare('SELECT s.*,a.run,a.generation,a.token_hash,a.lease_expires FROM synthetic_dispatches s JOIN attempts a ON a.id=s.attempt WHERE s.attempt=?').get(event.attemptId);
    if(!row||row.session!==scope.sessionId||typeof input.dispatchToken!=='string'||hash(input.dispatchToken)!==row.token_hash)deny(401,'dispatchUnauthorized');
    const dispatch=JSON.parse(String(row.data)) as Omit<SyntheticDispatch,'dispatchToken'>;
    for(const key of ['runId','attemptId','generation','bindingRevisionId','runtimeVersion','selectedModel'] as const)if(event[key]!==dispatch.attempt[key])deny(409,'dispatchFenced');
    if(event.protocolVersion!==1||event.provider!=='codex'||event.generation!==row.generation)deny(409,'dispatchFenced');
    return {row,dispatch,run:c.getRun(String(row.run))};
  };
  const ingest=(scope:RunnerScope,input:Record<string,unknown>):RunnerReceipt=>{
    const event=evidence(input.event);
    const {row,dispatch,run}=correlate(scope,input,event);
    const encoded=canonical(event);const fingerprint=hash(encoded);
    const old=db.prepare('SELECT fingerprint,cursor FROM runner_evidence WHERE attempt=? AND sequence=?').get(event.attemptId,event.sequence);
    if(old){if(old.fingerprint!==fingerprint)deny(409,'evidenceConflict');return {attemptId:event.attemptId,sequence:event.sequence,cursor:Number(old.cursor),duplicate:true};}
    if(Number(row.lease_expires)<=c.now()||run.attemptId!==event.attemptId||!['queued','running','cancelRequested'].includes(run.state)||event.sequence!==Number(row.last_sequence)+1)deny(409,'dispatchFenced');
    c.checkpoint('syntheticAfterReceipt');
    const totals=db.prepare("SELECT COALESCE(SUM(length(CAST(data AS BLOB))),0) AS bytes, COALESCE(SUM(CASE WHEN json_extract(data,'$.kind')='textDelta' THEN length(CAST(json_extract(data,'$.text') AS BLOB)) ELSE 0 END),0) AS text_bytes FROM runner_evidence WHERE attempt=?").get(event.attemptId)!;
    if(Number(totals.bytes)+Buffer.byteLength(encoded)>1_000_000||(event.kind==='textDelta'&&Number(totals.text_bytes)+Buffer.byteLength(event.text)>65_536))deny(413,'runnerEvidenceLimit');
    if(event.kind==='started') {
      if(row.turn_id!==null||event.threadId!==dispatch.threadId||typeof event.turnId!=='string')deny(409,'turnConflict');
      db.prepare('UPDATE synthetic_dispatches SET turn_id=? WHERE attempt=?').run(event.turnId,event.attemptId);
      if(run.state!=='cancelRequested')run.state='running';c.updateRun(run);c.appendEvent(run.conversationId,run.runId,'run.updated',{run});
    } else if(event.kind==='textDelta') {
      if(event.threadId!==dispatch.threadId||event.turnId!==row.turn_id)deny(409,'turnConflict');
    } else if(event.kind==='notStarted') {
      if(row.turn_id!==null||event.sequence!==1||run.state!=='cancelRequested')deny(409,'turnConflict');
      run.state='canceled';c.updateRun(run);db.prepare("UPDATE outbox SET state='canceled' WHERE run=?").run(run.runId);c.appendEvent(run.conversationId,run.runId,'run.updated',{run});
    } else if(event.kind==='terminal') {
      if(event.threadId!==dispatch.threadId||event.turnId!==row.turn_id||event.effectiveModel!==null||event.modelEvidence!=='unreported')deny(409,'turnConflict');
      if(event.status==='completed') {
        for(const item of event.messages.filter(item=>item.phase!=='commentary')) {
          const message:Message={messageId:randomUUID(),conversationId:run.conversationId,requestId:run.requestId,role:'assistant',text:item.text,sequence:c.cursor()+1,createdAt:c.timestamp()};
          db.prepare('INSERT INTO messages VALUES (?,?,?,?,?)').run(message.messageId,message.conversationId,message.requestId,message.sequence,JSON.stringify(message));
          c.appendEvent(run.conversationId,message.messageId,'message.created',{message});
          c.checkpoint('syntheticAfterMessage');
        }
        run.state='completed';
      } else run.state=event.status==='interrupted'?'canceled':'outcomeUnknown';
      c.updateRun(run);db.prepare('UPDATE outbox SET state=? WHERE run=?').run(run.state,run.runId);c.appendEvent(run.conversationId,run.runId,'run.updated',{run});
    } else if(event.kind==='outcomeUnknown') {
      run.state='outcomeUnknown';c.updateRun(run);db.prepare("UPDATE outbox SET state='outcomeUnknown' WHERE run=?").run(run.runId);c.appendEvent(run.conversationId,run.runId,'run.updated',{run});
    } else deny(400,'unsupportedRunnerEvidence');
    db.prepare('UPDATE synthetic_dispatches SET last_sequence=? WHERE attempt=?').run(event.sequence,event.attemptId);
    const cursor=c.cursor();db.prepare('INSERT INTO runner_evidence VALUES (?,?,?,?,?)').run(event.attemptId,event.sequence,fingerprint,encoded,cursor);
    c.checkpoint(event.kind==='terminal'?'syntheticBeforeTerminalCommit':'syntheticBeforeEvidenceCommit');
    return {attemptId:event.attemptId,sequence:event.sequence,cursor,duplicate:false};
  };
  return {
    enroll,
    expire() { c.transaction(expireDue); },
    revoke(sessionId:string) { db.prepare('UPDATE runner_sessions SET revoked=1 WHERE id=?').run(sessionId); },
    command(action:string,request:IncomingMessage,input:Record<string,unknown>) {
      const result=c.transaction(()=>{
        const scope=authenticate(request,input);
        const scopeKeys=['protocolVersion','installationId','recoveryEpoch','hostId','sessionId'];
        exact(input,[...scopeKeys,...(action==='events'?['dispatchToken','event']:action==='status'?['dispatchToken','attemptId']:[])]);
        if(action==='claim'){expireDue();return {dispatch:claim(scope)};}
        if(action==='events')return ingest(scope,input);
        if(action==='status') {
          if(!id(input.attemptId)||typeof input.dispatchToken!=='string')deny(401,'dispatchUnauthorized');
          const row=db.prepare('SELECT s.session,a.run,a.token_hash FROM synthetic_dispatches s JOIN attempts a ON a.id=s.attempt WHERE s.attempt=?').get(input.attemptId);
          if(!row||row.session!==scope.sessionId||row.token_hash!==hash(input.dispatchToken))deny(401,'dispatchUnauthorized');
          expireDue();return {state:c.getRun(String(row.run)).state};
        }
        deny(404,'runnerRouteUnavailable');
      });
      const point=action==='claim'?'syntheticAfterClaimCommit':action==='events'&&record(input.event).kind==='terminal'?'syntheticAfterTerminalCommit':'syntheticAfterEvidenceCommit';
      c.checkpoint(point);return result;
    },
  };
}
