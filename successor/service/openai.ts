import { DatabaseSync } from 'node:sqlite';
import { createServer, type IncomingMessage } from 'node:http';
import type { Socket } from 'node:net';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { mkdirSync, lstatSync, chmodSync, realpathSync } from 'node:fs';
import { isAbsolute, join } from 'node:path';
import type { Week, Acceptance, Receipt, RuntimeDescriptor, Agent, Conversation, InstructionSkill, Installation, Message, Run, Snapshot, TurnConfiguration, RoutineRevision, RoutineExecution, RoutineApproval, ScheduleRevision, SchedulerStatus, SchedulePreview, ScheduleOccurrence, RequestCancellation, ConversationMemory, RuntimeReadiness, AgentAvatar, AgentPortrait, PortraitDraw, PortraitImage, OwnerPairingPolicy, PairingStatus } from '../contracts/v1.ts';
import type { ChatInput, ChatRuntime } from '../runner/chat-runtime.ts';
import { EXECUTION_LIMITS, TEXT_POLICY, executionDigest, immutable, sealDispatch } from '../contracts/executor.ts';
import { parseContextModuleRef, type ContextCompiler, type ContextModuleStatus, type PreparedContext } from '../contracts/context.ts';
import { parseSchedule, calendarBetween, previewCalendar } from './schedule.ts';
import { InvalidWeek, WeekConflict, addPlanItem, closeWeek, decodeWeek, foundWeek, placeBuild, viewWeek } from './week.ts';
import { InvalidRoutine, parseRoutineDefinition, parseRoutineInputs, parseRoutineExecution, startRoutine, stopRoutine, taskInput, routineDigest, outputDigest } from './routine.ts';
import { InvalidSkillBundle, parseSkillBundle, skillInstructions } from '../contracts/skill-bundle.ts';
import { PrivateSocketLease } from './private-socket.ts';
import type { OwnerSetupCancellation, OwnerSetupIdentity, OwnerSetupReceipt, OwnerSetupRequest, OwnerSetupState } from '../contracts/owner-setup.ts';
import { OwnerRecoveryIdentityRejected } from '../contracts/owner-recovery.ts';
import { companionPairing, CompanionPairingError } from './companion-pairing.ts';
import { parseAccountIdentity, sameAccount } from '../contracts/account.ts';
import { accountSignIn, AccountSignInError } from './account-sign-in.ts';

const unresolved = new Set<Run['state']>(['queued','running','cancelRequested','waitingForApproval','outcomeUnknown']);
export type ServiceListener = { kind: 'loopback'; port?: number } | { kind: 'privateSocket'; name: string };
export type ServiceEndpoint = Readonly<{ kind: 'loopback'; url: string } | { kind: 'privateSocket'; directory: string; name: string }>;
type OwnerControl = { deviceId: string; intent: 'running' | 'stop' | 'disconnect' };
type OwnerSetup = { version: 1; setupId: string; expires: number } & (
  | { state: 'pending' }
  | { state: 'completed' | 'canceling'; deviceId: string }
  | { state: 'canceled'; deviceId: string; outcome: 'notPaired' | 'revoked' }
);
type Entity = 'portraits' | 'weeks' | 'conversationMemory' | 'requestCancellations' | 'schedules' | 'scheduleRevisions' | 'scheduleOccurrences' | 'agentBindings' | 'disabledBindings' | 'routineApprovals' | 'routines' | 'routineRevisions' | 'agents' | 'skillRevisions' | 'skills' | 'conversations' | 'messages' | 'runs' | 'requests' | 'contexts' | 'reads';
class APIError extends Error {
  readonly status: number; readonly code: string;
  constructor(status: number, code: string, message: string) { super(message); this.status = status; this.code = code; }
}
function fail(status: number, code: string, message: string): never { throw new APIError(status, code, message); }
const digest = (text: string) => createHash('sha256').update(text).digest('hex');
function record(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) fail(400, 'invalidCommand', 'Expected an object.');
  return value as Record<string, unknown>;
}
function text(value: unknown, label: string, max = 16_384): string {
  if (typeof value !== 'string' || !value.isWellFormed() || value.trim().length === 0 || Buffer.byteLength(value) > max) fail(400, 'invalidCommand', `Invalid ${label}.`);
  return value;
}
function messageText(value: unknown): string {
  if (typeof value !== 'string' || !value.isWellFormed() || value.length === 0 || Buffer.byteLength(value) > 65_536) fail(400, 'invalidCommand', 'Use nonempty valid Unicode text within the message limit.');
  return value;
}
function id(value: unknown): string {
  const result = text(value, 'identifier', 128);
  if (!/^[A-Za-z0-9_-]+$/.test(result)) fail(400, 'invalidCommand', 'Invalid identifier.');
  return result;
}
function avatar(value:unknown):{avatar?:AgentAvatar} {
  if(value===undefined)return {};
  if(value!=='orbit'&&value!=='ember'&&value!=='sprout')fail(400,'invalidAvatar','Choose a bundled Fort avatar.');
  return {avatar:value};
}
function decodePortrait(value: unknown): { portrait?: AgentPortrait } {
  if (value === undefined) return {};
  const v = record(value);
  if (typeof v.size !== 'number' || !Number.isSafeInteger(v.size) || v.size < 256 || v.size > 2048 || typeof v.createdAt !== 'string') throw new Error('Invalid stored portrait');
  return { portrait: { revision: id(v.revision), createdAt: v.createdAt, size: v.size } };
}
function decodePortraitDraw(value: unknown): { portraitDraw?: PortraitDraw } {
  if (value === undefined) return {};
  const v = record(value);
  if ((v.state !== 'drawing' && v.state !== 'failed') || typeof v.updatedAt !== 'string') throw new Error('Invalid stored portrait draw');
  return { portraitDraw: { state: v.state, detail: text(v.detail, 'detail', 16_384), updatedAt: v.updatedAt } };
}
function decodeAgent(value: unknown): Agent {
  const v = record(value), b = record(v.binding);
  if (typeof b.provider !== 'string' || typeof b.enabled !== 'boolean' || v.readiness !== 'requiresLiveReview') throw new Error('Invalid stored Agent');
  return { ...avatar(v.avatar),...decodePortrait(v.portrait),...decodePortraitDraw(v.portraitDraw),agentId: id(v.agentId), name: text(v.name, 'name'), homeConversationId: id(v.homeConversationId), instructions: text(v.instructions, 'instructions'), readiness: v.readiness,
    binding: { bindingRevisionId: id(b.bindingRevisionId), provider: b.provider, runtimeVersion: text(b.runtimeVersion, 'runtime'), selectedModel: text(b.selectedModel, 'model'), hostId: id(b.hostId), enabled: b.enabled } };
}
function decodeSkill(value: unknown): InstructionSkill {
  const v = record(value);
  const identity = { skillId:id(v.skillId),revisionId:id(v.revisionId),name:text(v.name,'name'),instructions:text(v.instructions,'instructions') };
  if (v.bundle === undefined && v.review === undefined) return identity;
  const bundle = parseSkillBundle(v.bundle), review = record(v.review);
  if ((review.state !== 'reviewRequired' && review.state !== 'enabled' && review.state !== 'disabled') || review.digest !== bundle.digest || identity.name !== bundle.name || identity.instructions !== skillInstructions(bundle)) throw new Error('Invalid stored skill review');
  return { ...identity,bundle,review:{state:review.state,digest:bundle.digest},...(v.previousRevisionId===undefined?{}:{previousRevisionId:id(v.previousRevisionId)}) };
}
function decodeConversation(value: unknown): Conversation {
  const v = record(value);
  if ((v.kind !== 'home' && v.kind !== 'shared') || !Array.isArray(v.participantAgentIds)) throw new Error('Invalid stored Conversation');
  return { conversationId: id(v.conversationId), name: text(v.name, 'name'), kind: v.kind, participantAgentIds: v.participantAgentIds.map(id) };
}
function memoryText(value:unknown):string {
  if(typeof value!=='string'||!value.isWellFormed()||Buffer.byteLength(value)>8192)fail(400,'invalidMemory','Use plain Unicode notes within 8192 bytes.');return value;
}
function decodeMemory(value:unknown):ConversationMemory {
  const v=record(value);return {conversationId:id(v.conversationId),revisionId:id(v.revisionId),text:memoryText(v.text)};
}
function decodeConfiguration(value: unknown): TurnConfiguration {
  const v = record(value);
  if (!Array.isArray(v.skills)) throw new Error('Invalid stored skills');
  return { ...(v.account === undefined ? {} : { account: parseAccountIdentity(v.account) }), agentId: id(v.agentId), agentName: text(v.agentName, 'name'), bindingRevisionId: id(v.bindingRevisionId), selectedModel: text(v.selectedModel, 'model'), instructions: text(v.instructions, 'instructions'), skills: v.skills.map(decodeSkill),...(v.memory===undefined?{}:{memory:decodeMemory(v.memory)}),...(v.contextModule===undefined?{}:{contextModule:parseContextModuleRef(v.contextModule)}) };
}
function decodeMessage(value: unknown): Message {
  const v = record(value);
  if ((v.role !== 'user' && v.role !== 'assistant') || typeof v.sequence !== 'number' || !Number.isSafeInteger(v.sequence)) throw new Error('Invalid stored Message');
  return { messageId: id(v.messageId), conversationId: id(v.conversationId), requestId: id(v.requestId), role: v.role, text: messageText(v.text), sequence: v.sequence, createdAt: text(v.createdAt, 'date'),
    ...(v.agentId === undefined ? {} : { agentId: id(v.agentId), agentName: text(v.agentName, 'name') }),
    ...(v.recipientAgentId === undefined ? {} : { recipientAgentId: id(v.recipientAgentId), recipientAgentName: text(v.recipientAgentName, 'name') }) };
}
function storedReceipt(value: unknown): Receipt {
  const stored = record(value);
  const v = record(stored.receipt ?? record(stored.acceptance).receipt);
  return { requestId: id(v.requestId), conversationId: id(v.conversationId), messageId: id(v.messageId), runId: id(v.runId), bindingRevisionId: id(v.bindingRevisionId), inputDigest: text(v.inputDigest, 'input digest', 64), acceptedAt: text(v.acceptedAt, 'acceptance time') };
}
function decodeRun(value: unknown): Run {
  const v = record(value);
  if (v.state !== 'queued' && v.state !== 'running' && v.state !== 'waitingForApproval' && v.state !== 'cancelRequested' && v.state !== 'completed' && v.state !== 'failed' && v.state !== 'outcomeUnknown' && v.state !== 'canceled' && v.state !== 'closedUnknown') throw new Error('Invalid stored Run');
  if (v.attemptGeneration !== undefined && (typeof v.attemptGeneration !== 'number' || !Number.isSafeInteger(v.attemptGeneration) || v.attemptGeneration < 1)) throw new Error('Invalid stored attempt generation');
  return { ...(v.attemptGeneration === undefined ? {} : { attemptGeneration: v.attemptGeneration }), runId: id(v.runId), requestId: id(v.requestId), conversationId: id(v.conversationId), bindingRevisionId: id(v.bindingRevisionId), state: v.state, selectedModel: text(v.selectedModel, 'model'), effectiveModel: null, modelEvidence: 'unreported', attemptId: v.attemptId === null ? null : id(v.attemptId), configuration: decodeConfiguration(v.configuration), ...(v.routine===undefined?{}:{routine:parseRoutineExecution(v.routine)}), ...(v.statusDetail === undefined ? {} : { statusDetail: text(v.statusDetail, 'status') }) };
}
function decodeRoutineApproval(value:unknown):RoutineApproval {
  const v=record(value);
  if(v.kind!=='routineGate'||JSON.stringify(v.resourceScope)!==JSON.stringify(['routine:continue'])||(v.state!=='pending'&&v.state!=='approved'&&v.state!=='rejected'&&v.state!=='expired'&&v.state!=='canceled'))throw new Error('Invalid routine approval');
  return {approvalId:id(v.approvalId),installationId:id(v.installationId),recoveryEpoch:id(v.recoveryEpoch),runId:id(v.runId),attemptId:id(v.attemptId),bindingRevisionId:id(v.bindingRevisionId),actionDigest:text(v.actionDigest,'digest',64),expiresAt:text(v.expiresAt,'expiry'),state:v.state,decidedByDeviceId:v.decidedByDeviceId===null?null:id(v.decidedByDeviceId),decidedAt:v.decidedAt===null?null:text(v.decidedAt,'decision time'),kind:'routineGate',routineRevisionId:id(v.routineRevisionId),stepId:id(v.stepId),resourceScope:['routine:continue'],actionDescription:text(v.actionDescription,'gate description')};
}
function decodeRoutine(value:unknown):RoutineRevision {
  const v=record(value),definition=parseRoutineDefinition(v.definition),agentSnapshot=decodeAgent(v.agentSnapshot);
  if(!Array.isArray(v.skills))throw new Error('Invalid routine skill snapshot');
  const skills=v.skills.map(decodeSkill),contextModule=v.contextModule===undefined?{}:{contextModule:parseContextModuleRef(v.contextModule)},digest=routineDigest({definition,agentSnapshot,skills,...contextModule});
  if(v.digest!==digest)throw new Error('Routine revision digest changed');
  return {routineId:id(v.routineId),revisionId:id(v.revisionId),digest,definition,agentSnapshot,skills,...contextModule};
}
function decodeSchedule(value:unknown):ScheduleRevision {
  const v=record(value),definition=parseSchedule(v.definition),hash=routineDigest(definition);
  if(v.digest!==hash||(v.state!=='paused'&&v.state!=='enabled')||typeof v.checkedThrough!=='number'||!Number.isSafeInteger(v.checkedThrough))throw new Error('Invalid schedule revision');
  const result:ScheduleRevision={scheduleId:id(v.scheduleId),revisionId:id(v.revisionId),digest:hash,definition,state:v.state,checkedThrough:v.checkedThrough};
  if(v.consent!==undefined){const consent=record(v.consent);result.consent={deviceId:id(consent.deviceId),at:text(consent.at,'consent time'),previewDigest:text(consent.previewDigest,'preview digest')};}
  if(v.preview!==undefined){
    const preview=record(v.preview);if(!Array.isArray(preview.occurrences)||preview.overlap!=='skip'||(preview.missed!=='skip'&&preview.missed!=='catchUpOne'))throw new Error('Invalid schedule preview');
    result.preview={revisionId:id(preview.revisionId),digest:text(preview.digest,'preview digest'),expiresAt:text(preview.expiresAt,'preview expiry'),timezone:text(preview.timezone,'timezone'),overlap:'skip',missed:preview.missed,occurrences:preview.occurrences.map(value=>{const occurrence=record(value);return {localDate:text(occurrence.localDate,'date'),localTime:text(occurrence.localTime,'time'),intendedAt:occurrence.intendedAt===null?null:text(occurrence.intendedAt,'instant'),decisionAt:text(occurrence.decisionAt,'decision instant')};})};
  }
  return result;
}
function json(row: Record<string, unknown> | undefined): unknown {
  if (!row || typeof row.data !== 'string') fail(404, 'notFound', 'Record was not found.');
  return JSON.parse(row.data);
}

export type SubmissionAdmission = { text:string; conversation:Conversation; configuration:TurnConfiguration };
export function createApplication(options: { dataDirectory: string; provider: RuntimeDescriptor; runtime?: ChatRuntime; runtimes?: readonly { provider: RuntimeDescriptor; runtime?: ChatRuntime }[]; context?: ContextCompiler | null; autoRun?: boolean; attemptTimeoutMs?: number; credentialDeadline?: number; ownerPairingPolicy?: OwnerPairingPolicy; now?: () => number; ownerAccessChanged?: () => void; admitSubmission?: (submission:SubmissionAdmission,priorRuns:readonly Run[]) => boolean }) {
  const ownerAccessChanged=options.ownerAccessChanged;
  if(ownerAccessChanged!==undefined && typeof ownerAccessChanged!=='function')throw new Error('Select an explicit owner access notification');
  let ownerAccessWake=()=>{},ownerWakeQueued=false;
  const queueOwnerAccess=()=>{
    if(!ownerAccessChanged || ownerWakeQueued)return;
    ownerWakeQueued=true;queueMicrotask(()=>{ownerWakeQueued=false;ownerAccessWake();});
  };
  const attemptTimeoutMs = options.attemptTimeoutMs ?? 120_000;
  if (!Number.isSafeInteger(attemptTimeoutMs) || attemptTimeoutMs < 50 || attemptTimeoutMs > 120_000) throw new Error('Use a bounded execution deadline');
  const provider = structuredClone(options.provider);
  const validDescriptor = (value: RuntimeDescriptor) => Boolean(value && /^[A-Za-z0-9_-]{1,128}$/.test(value.provider) && value.name && value.runtimeVersion && value.hostId && value.capabilities.includes('plainText'));
  if (!validDescriptor(provider)) throw new Error('Use an explicit plain-text runtime descriptor');
  // Spec 064: the primary runtime plus any others this host serves. A binding names its runtime by provider, version and
  // host id, and its turns run only there. The ledger's runtime identity stays the primary's.
  type Lane = { provider: RuntimeDescriptor; runtime?: ChatRuntime };
  const lanes: Lane[] = [{ provider, runtime: options.runtime }, ...(options.runtimes ?? []).map(lane => ({ provider: structuredClone(lane.provider), runtime: lane.runtime }))];
  if (!lanes.every(lane => validDescriptor(lane.provider)) || new Set(lanes.map(lane => lane.provider.provider)).size !== lanes.length) throw new Error('Use distinct explicit plain-text runtime descriptors');
  const sameRuntime = (binding: { provider: string; runtimeVersion: string; hostId: string }, lane: Lane) =>
    binding.provider === lane.provider.provider && binding.runtimeVersion === lane.provider.runtimeVersion && binding.hostId === lane.provider.hostId;
  const laneFor = (binding: { provider: string; runtimeVersion: string; hostId: string }) => lanes.find(lane => sameRuntime(binding, lane));
  const RUNTIME_ABSENT = 'This Agent\u2019s runtime is not available on this host. Turn it on in the Mac app, then send again. Nothing was sent.';
  const context = options.context;
  if (context && (context.protocolVersion !== 1 || !/^[A-Za-z0-9_-]{1,128}$/.test(context.id) || !context.version || context.version.length > 128 || typeof context.compile !== 'function')) throw new Error('Select a versioned context compiler');
  const contextRef = context ? immutable(parseContextModuleRef(context)) : undefined;
  let contextFailed = false;
  const contextStatus = (): ContextModuleStatus => contextRef ? { state: contextFailed ? 'failed' : 'ready', module: contextRef } : { state: context === null ? 'disabled' : 'missing' };
  const now=options.now ?? Date.now;
  const expiryDays = (value:unknown):number|null => {
    if(value===null)return null;
    if(typeof value!=='number'||!Number.isSafeInteger(value)||value<1||value>365)fail(400,'invalidExpiry','Choose no expiry or a whole number of days from 1 to 365.');
    return value;
  };
  if(options.ownerPairingPolicy){
    expiryDays(options.ownerPairingPolicy.expiresAfterDays);
    if(options.credentialDeadline!==undefined)throw new Error('Owner pairing cannot extend a bounded session');
  }
  if (options.credentialDeadline!==undefined && (!Number.isSafeInteger(options.credentialDeadline) || options.credentialDeadline>now()+600_000)) throw new Error('Bounded credential deadline exceeds ten minutes');
  if (!isAbsolute(options.dataDirectory)) throw new Error('Use an explicit absolute data directory');
  mkdirSync(options.dataDirectory, { recursive: true, mode: 0o700 });
  if (lstatSync(options.dataDirectory).isSymbolicLink()) throw new Error('Data directory cannot be a symlink');
  const path = join(options.dataDirectory, 'openai-candidate-b.sqlite');
  try { if (lstatSync(path).isSymbolicLink()) throw new Error('Ledger cannot be a symlink'); }
  catch (error) { if (!(error instanceof Error && 'code' in error && error.code === 'ENOENT')) throw error; }
  const db = new DatabaseSync(path);
  try { db.exec('PRAGMA locking_mode=EXCLUSIVE; BEGIN EXCLUSIVE; COMMIT;'); }
  catch (error) { db.close(); throw error; }
  chmodSync(path, 0o600);
  db.exec(`PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL;
    CREATE TABLE IF NOT EXISTS metadata (key TEXT PRIMARY KEY, data TEXT NOT NULL);
    CREATE TABLE IF NOT EXISTS enrollment (hash TEXT PRIMARY KEY, expires INTEGER NOT NULL);
    CREATE TABLE IF NOT EXISTS devices (id TEXT PRIMARY KEY, hash TEXT UNIQUE NOT NULL, expires INTEGER NOT NULL);
    CREATE TABLE IF NOT EXISTS entities (kind TEXT NOT NULL, id TEXT NOT NULL, data TEXT NOT NULL, PRIMARY KEY(kind,id));
    CREATE TABLE IF NOT EXISTS events (sequence INTEGER PRIMARY KEY AUTOINCREMENT, data TEXT NOT NULL);
    CREATE TABLE IF NOT EXISTS dispatch_outbox (position INTEGER PRIMARY KEY AUTOINCREMENT, run_id TEXT NOT NULL UNIQUE, conversation_id TEXT NOT NULL, state TEXT NOT NULL CHECK(state IN ('queued','running','cancelRequested','waitingForApproval','outcomeUnknown')), held INTEGER NOT NULL DEFAULT 0 CHECK(held IN (0,1)), attempt_id TEXT, generation INTEGER NOT NULL CHECK(generation>=0));
    CREATE INDEX IF NOT EXISTS outbox_ready ON dispatch_outbox(state,held,position);
    CREATE INDEX IF NOT EXISTS outbox_conversation ON dispatch_outbox(conversation_id,position);
    CREATE INDEX IF NOT EXISTS message_request ON entities(json_extract(data,'$.requestId'),json_extract(data,'$.role')) WHERE kind='messages';
    CREATE INDEX IF NOT EXISTS message_page ON entities(json_extract(data,'$.conversationId'),json_extract(data,'$.sequence')) WHERE kind='messages';
    CREATE INDEX IF NOT EXISTS run_request ON entities(json_extract(data,'$.requestId')) WHERE kind='runs';
    CREATE INDEX IF NOT EXISTS run_state ON entities(json_extract(data,'$.conversationId'),json_extract(data,'$.state')) WHERE kind='runs';
    CREATE INDEX IF NOT EXISTS approval_run ON entities(json_extract(data,'$.runId')) WHERE kind='routineApprovals';`);
  const outboxVersion = db.prepare("SELECT data FROM metadata WHERE key='outboxVersion'").get();
  if (outboxVersion && outboxVersion.data !== '1') { db.close(); throw new Error('Unsupported outbox version'); }
  if (!outboxVersion) {
    db.exec(`BEGIN IMMEDIATE;
      INSERT INTO dispatch_outbox(run_id,conversation_id,state,attempt_id,generation)
      SELECT id,json_extract(data,'$.conversationId'),json_extract(data,'$.state'),json_extract(data,'$.attemptId'),coalesce(json_extract(data,'$.attemptGeneration'),0)
      FROM entities WHERE kind='runs' AND json_extract(data,'$.state') IN ('queued','running','cancelRequested','waitingForApproval','outcomeUnknown') ORDER BY rowid;
      INSERT INTO metadata VALUES ('outboxVersion','1'); COMMIT;`);
  }
  const runtimeIdentity = JSON.stringify({ provider: provider.provider, runtimeVersion: provider.runtimeVersion, hostId: provider.hostId });
  const savedRuntime = db.prepare("SELECT data FROM metadata WHERE key='runtimeIdentity'").get();
  try {
    if (savedRuntime && savedRuntime.data !== runtimeIdentity) throw new Error('Installation runtime identity changed; explicit binding migration is required');
    if (!savedRuntime) {
      for (const row of db.prepare("SELECT data FROM entities WHERE kind IN ('agents','agentBindings')").all()) {
        const binding = decodeAgent(json(row)).binding;
        if (!laneFor(binding)) throw new Error('Stored binding runtime identity differs from the selected runtime');
      }
      db.prepare("INSERT INTO metadata VALUES ('runtimeIdentity',?)").run(runtimeIdentity);
    }
  } catch (error) { db.close(); throw error; }
  const savedDeadline=db.prepare("SELECT data FROM metadata WHERE key='credentialDeadline'").get();
  const credentialDeadline=savedDeadline ? Number(json(savedDeadline)) : options.credentialDeadline;
  if (savedDeadline && options.credentialDeadline!==undefined && credentialDeadline!==options.credentialDeadline) { db.close(); throw new Error('Credential deadline cannot be changed'); }
  if (!savedDeadline && credentialDeadline!==undefined) {
    if (credentialDeadline<=now() || db.prepare("SELECT 1 FROM metadata WHERE key='installation'").get()) { db.close(); throw new Error('Bounded enrollment requires a fresh installation'); }
    db.prepare('INSERT INTO metadata VALUES (?,?)').run('credentialDeadline',JSON.stringify(credentialDeadline));
  }
  const savedPairing=db.prepare("SELECT data FROM metadata WHERE key='ownerPairingPolicy'").get();
  let ownerPairingPolicy:OwnerPairingPolicy|undefined;
  try {
    if(savedPairing){
      if(credentialDeadline!==undefined)throw new Error('Conflicting pairing policy');
      ownerPairingPolicy={expiresAfterDays:expiryDays(record(json(savedPairing)).expiresAfterDays)};
    }else if(options.ownerPairingPolicy){
      if(credentialDeadline!==undefined||db.prepare("SELECT 1 FROM metadata WHERE key='installation'").get())throw new Error('Owner pairing requires a fresh explicitly configured installation');
      ownerPairingPolicy={expiresAfterDays:options.ownerPairingPolicy.expiresAfterDays};
      db.prepare('INSERT INTO metadata VALUES (?,?)').run('ownerPairingPolicy',JSON.stringify(ownerPairingPolicy));
    }
  }catch(error){db.close();throw error;}
  let ownerControl: OwnerControl | undefined;
  try {
    const savedControl = db.prepare("SELECT data FROM metadata WHERE key='ownerControl'").get();
    if (savedControl) {
      const value = record(json(savedControl));
      if (!ownerPairingPolicy) throw new Error('Invalid owner control state');
      const intent = value.intent;
      if (intent !== 'running' && intent !== 'stop' && intent !== 'disconnect') throw new Error('Invalid owner control intent');
      ownerControl = { deviceId: id(value.deviceId), intent };
    }
  } catch (error) { db.close(); throw error; }
  const saveOwnerControl = (control: OwnerControl) => {
    db.prepare("INSERT OR REPLACE INTO metadata VALUES ('ownerControl',?)").run(JSON.stringify(control));
    ownerControl = control;
  };
  const transaction = <T>(operation: () => T): T => {
    db.exec('BEGIN IMMEDIATE');
    try { const result = operation(); db.exec('COMMIT'); queueOwnerAccess(); return result; }
    catch (error) { db.exec('ROLLBACK'); throw error; }
  };
  const acceptanceTransaction=<T>(operation:()=>T):T=>{
    db.exec('SAVEPOINT acceptance');
    try { const result=operation();db.exec('RELEASE acceptance');return result; }
    catch(error){db.exec('ROLLBACK TO acceptance; RELEASE acceptance');throw error;}
  };
  let installation: Installation;
  const saved = db.prepare("SELECT data FROM metadata WHERE key='installation'").get();
  if (saved) {
    const v = record(json(saved));
    if (v.contractVersion !== '1.0' || (v.mode !== 'openai' && v.mode !== 'local') || (v.serviceName !== 'Fort Prototype' && v.serviceName !== 'Fort')) { db.close(); throw new Error('Wrong ledger identity'); }
    installation = { contractVersion: v.contractVersion, mode: v.mode, serviceName: v.serviceName, installationId: id(v.installationId), recoveryEpoch: id(v.recoveryEpoch) };
  } else {
    installation = { contractVersion: '1.0', mode: 'local', serviceName: 'Fort', installationId: randomUUID(), recoveryEpoch: randomUUID() };
    db.prepare('INSERT INTO metadata VALUES (?,?)').run('installation', JSON.stringify(installation));
  }
  installation = {...installation,runtime:provider,runtimes:lanes.map(lane=>lane.provider),capabilities:[...(ownerPairingPolicy?['ownerPairing','companionPairing']:[]),...(ownerPairingPolicy&&lanes.some(lane=>lane.runtime?.signIn)?['accountSignIn']:[]),...(lanes.length>1?['multipleRuntimes']:[]),...(options.runtime?.readiness&&options.runtime.restart?['runtimeRecovery']:[]),'eventReplay','schedulerStatus','agentSettings','modelCatalog','conversationMemory','requestCancellation','skillBundles','routines','routineGates','schedules','weeks']};
  const put = (kind: Entity, entityId: string, data: unknown) => {
    const result = db.prepare('INSERT INTO entities VALUES (?,?,?) ON CONFLICT(kind,id) DO UPDATE SET data=excluded.data').run(kind, entityId, JSON.stringify(data));
    if (kind === 'runs') {
      const run = decodeRun(data);
      if (unresolved.has(run.state)) db.prepare(`INSERT INTO dispatch_outbox(run_id,conversation_id,state,attempt_id,generation) VALUES (?,?,?,?,?)
        ON CONFLICT(run_id) DO UPDATE SET state=excluded.state,attempt_id=excluded.attempt_id,generation=excluded.generation`).run(run.runId,run.conversationId,run.state,run.attemptId,run.attemptGeneration ?? 0);
      else db.prepare('DELETE FROM dispatch_outbox WHERE run_id=?').run(run.runId);
    }
    if (['agents', 'agentBindings', 'disabledBindings', 'conversations', 'skills', 'skillRevisions', 'routines', 'schedules', 'weeks'].includes(kind)) append('_workspace', 'workspaceChanged', entityId);
    return result;
  };
  const list = <T>(kind: Entity, decode: (value: unknown) => T): T[] => db.prepare('SELECT data FROM entities WHERE kind=? ORDER BY rowid').all(kind).map(row => decode(json(row)));
  const load = <T>(kind: Entity, entityId: string, decode: (value: unknown) => T): T => decode(json(db.prepare('SELECT data FROM entities WHERE kind=? AND id=?').get(kind, entityId)));
  const authenticate = (request: IncomingMessage) => {
    if (closed) fail(503,'closed','This session has stopped.');
    if (request.headers.origin !== undefined) fail(403, 'originDenied', 'Browser origins are not enabled.');
    if (request.headers['x-fort-installation-id'] !== installation.installationId) fail(401, 'unauthorized', 'Use this installation’s device credential.');
    const token = /^Bearer ([A-Za-z0-9_-]+)$/.exec(request.headers.authorization ?? '')?.[1];
    const device = token && db.prepare('SELECT id,expires FROM devices WHERE hash=?').get(digest(token));
    if (!device || (!(ownerPairingPolicy && device.expires===0) && Number(device.expires) <= now())) fail(401, 'unauthorized', 'Device authorization is missing or expired.');
    if(ownerPairingPolicy && ownerControl?.deviceId!==device.id && !companions.isCompanion(id(device.id)))fail(401,'unauthorized','This device has no current membership.');
    return id(device.id);
  };
  const pairingStatus=(deviceId:string):PairingStatus=>{
    const device=db.prepare('SELECT expires FROM devices WHERE id=?').get(deviceId);
    if(!device)fail(401,'unauthorized','This pairing has been removed.');
    return {deviceId,mode:ownerPairingPolicy?'owner-local':'temporary',expiresAt:ownerPairingPolicy&&device.expires===0?null:new Date(Number(device.expires)).toISOString(),...(ownerPairingPolicy && ownerControl?.deviceId!==deviceId && companions.isCompanion(deviceId)?{role:'companion' as const}:{})};
  };
  const owner = <T>(request: IncomingMessage, operation: (deviceId:string) => T): T => transaction(() => {
    const deviceId=authenticate(request);
    if(dispatchStopped&&request.method!=='GET'&&!['/v1/host/stop','/v1/pairing/disconnect','/v1/runtime/restart'].includes(request.url??''))fail(503,'dispatchStopped','Work is stopped. Resume explicitly, retry disconnect or finish local host recovery.');
    return operation(deviceId);
  });
  const cursor = () => Number(db.prepare('SELECT coalesce(max(sequence),0) AS cursor FROM events').get()?.cursor);
  const eventWaiters = new Set<() => void>();
  const append = (conversationId: string, kind: string, entityId: string) => {
    const event = { schemaVersion: 1, installationId: installation.installationId, recoveryEpoch: installation.recoveryEpoch, eventId: randomUUID(), conversationId, entityId, kind, createdAt: new Date().toISOString(), data: {} };
    const sequence = Number(db.prepare('INSERT INTO events(data) VALUES (?)').run(JSON.stringify(event)).lastInsertRowid);
    db.prepare('DELETE FROM events WHERE sequence<=?').run(sequence - 10_000);
    queueMicrotask(() => { for (const wake of [...eventWaiters]) wake(); });
    return sequence;
  };
  // A portrait draw doesn't survive a restart (Spec 065); say so instead of showing it as still drawing.
  transaction(() => {
    for (const agent of list('agents', decodeAgent)) if (agent.portraitDraw?.state === 'drawing')
      put('agents', agent.agentId, { ...agent, portraitDraw: { state: 'failed', detail: 'Fort restarted before the portrait finished. Nothing was saved.', updatedAt: new Date(now()).toISOString() } });
  });
  transaction(() => {
    for (const run of db.prepare("SELECT e.data FROM dispatch_outbox o JOIN entities e ON e.kind='runs' AND e.id=o.run_id ORDER BY o.position").all().map(row=>decodeRun(json(row)))) {
      if (run.state === 'running' || run.state === 'cancelRequested') { put('runs', run.runId, { ...run, state: 'outcomeUnknown', statusDetail: 'Service restarted before a confirmed result.',...(run.routine?{routine:stopRoutine(run.routine,'outcomeUnknown','Service restarted before a confirmed result.')}: {}) }); append(run.conversationId, 'runStopped', run.runId); }
      if (run.state === 'queued'&&!run.routine) { put('runs', run.runId, { ...run, statusDetail: 'Held after restart. Review or cancel this queued work before continuing.' }); db.prepare('UPDATE dispatch_outbox SET held=1 WHERE run_id=?').run(run.runId); }
    }
  });
  const conversationMemory=(conversationId:string):ConversationMemory|undefined=>{
    const row=db.prepare("SELECT data FROM entities WHERE kind='conversationMemory' AND id=?").get(conversationId);return row?decodeMemory(json(row)):undefined;
  };
  const snapshot = (conversationId: string, before: number | null = null): Snapshot => {
    load('conversations', conversationId, decodeConversation);
    const read = db.prepare("SELECT data FROM entities WHERE kind='reads' AND id=?").get(conversationId);
    const memory=conversationMemory(conversationId);
    const rows = db.prepare("SELECT data FROM entities WHERE kind='messages' AND json_extract(data,'$.conversationId')=? AND json_extract(data,'$.sequence')<? ORDER BY json_extract(data,'$.sequence') DESC LIMIT 101").all(conversationId, before ?? Number.MAX_SAFE_INTEGER);
    const messages = rows.slice(0, 100).map(row => decodeMessage(json(row))).reverse();
    const requestIds = JSON.stringify([...new Set(messages.map(message => message.requestId))]);
    const related = db.prepare("SELECT rowid,data FROM entities WHERE kind='runs' AND json_extract(data,'$.requestId') IN (SELECT value FROM json_each(?))").all(requestIds);
    const active = db.prepare("SELECT rowid,data FROM entities WHERE kind='runs' AND json_extract(data,'$.conversationId')=? AND json_extract(data,'$.state') IN ('queued','running','cancelRequested','waitingForApproval','outcomeUnknown') ORDER BY rowid LIMIT 100").all(conversationId);
    const runs = [...new Map([...related, ...active].map(row => [Number(row.rowid), row])).entries()].sort(([a], [b]) => a-b).map(([,row]) => decodeRun(json(row)));
    const approvals = db.prepare("SELECT data FROM entities WHERE kind='routineApprovals' AND json_extract(data,'$.runId') IN (SELECT value FROM json_each(?)) ORDER BY rowid").all(JSON.stringify(runs.map(run => run.runId))).map(row => decodeRoutineApproval(json(row)));
    return { ...(memory?{memory}:{}),installationId: installation.installationId, recoveryEpoch: installation.recoveryEpoch, conversationId, cursor: cursor(), messages, runs, approvals, window: { before, olderBefore: rows.length > 100 ? messages[0]?.sequence ?? null : null }, lastReadMessageSequence: read ? Number(record(json(read)).sequence) : 0 };
  };
  const acceptance = (receipt: Receipt): Acceptance => ({ receipt, snapshot: snapshot(receipt.conversationId), message: load('messages', receipt.messageId, decodeMessage), run: load('runs', receipt.runId, decodeRun) });

  const cancelRun=(runId:string):Run=>{
    const run=load('runs',runId,decodeRun);
    if(run.state!=='queued'&&run.state!=='running'&&run.state!=='waitingForApproval')return run;
    const immediate=run.state!=='running';
    const canceled:Run={...run,state:immediate?'canceled':'cancelRequested',statusDetail:immediate?'Canceled before runtime dispatch.':'Stop requested; waiting for runtime confirmation.',...(immediate&&run.routine?{routine:stopRoutine(run.routine,'canceled','Owner canceled the routine.')}:{})};
    if(immediate)for(const approval of list('routineApprovals',decodeRoutineApproval)){if(approval.runId===runId&&approval.state==='pending')put('routineApprovals',approval.approvalId,{...approval,state:'canceled'});}
    put('runs',runId,canceled);append(run.conversationId,'runStopped',runId);return canceled;
  };
  // The runtime a binding revision names: its saved snapshot, or the Agent whose current binding it is.
  const laneOf=(bindingRevisionId:string)=>{
    const row=db.prepare("SELECT data FROM entities WHERE kind='agentBindings' AND id=?").get(bindingRevisionId);
    const binding=row?decodeAgent(json(row)).binding:list('agents',decodeAgent).find(agent=>agent.binding.bindingRevisionId===bindingRevisionId)?.binding;
    return binding?laneFor(binding):undefined;
  };
  const bindingDisabled=(bindingRevisionId:string)=>Boolean(db.prepare("SELECT 1 FROM entities WHERE kind='disabledBindings' AND id=?").get(bindingRevisionId));
  const saveRoutine=(request:IncomingMessage,body:Record<string,unknown>,routineId?:string):RoutineRevision=>{
    const definition=parseRoutineDefinition(body.definition);
    return owner(request,()=>{
      if(routineId&&load('routines',routineId,decodeRoutine).revisionId!==body.expectedRevisionId)fail(409,'routineChanged','Reload the routine before saving a revision.');
      const current=load('agents',definition.agentId,decodeAgent),agentSnapshot=definition.bindingRevisionId===current.binding.bindingRevisionId?current:load('agentBindings',definition.bindingRevisionId,decodeAgent);
      if(agentSnapshot.agentId!==definition.agentId||definition.conversationId!==agentSnapshot.homeConversationId||!agentSnapshot.binding.enabled||bindingDisabled(definition.bindingRevisionId))fail(409,'bindingChanged','Select an enabled binding of this Agent and its exact Home.');
      put('agentBindings',agentSnapshot.binding.bindingRevisionId,agentSnapshot);
      const skills=definition.skills.map(ref=>{const skill=load('skillRevisions',ref.revisionId,decodeSkill);if(skill.skillId!==ref.skillId||skill.bundle?.digest!==ref.digest||skill.review?.state!=='enabled')fail(409,'skillReviewRequired','Select exact enabled skill bundles.');return skill;});
      const contextModule = skills.length && contextRef ? { contextModule: contextRef } : {};
      const routine:RoutineRevision={routineId:routineId??randomUUID(),revisionId:randomUUID(),digest:routineDigest({definition,agentSnapshot,skills,...contextModule}),definition,agentSnapshot,skills,...contextModule};
      put('routines',routine.routineId,routine);put('routineRevisions',routine.revisionId,routine);return routine;
    });
  };
  const priorAcceptance=(requestId:string,fingerprint:string):Acceptance|undefined=>{
    const prior=db.prepare("SELECT data FROM entities WHERE kind='requests' AND id=?").get(requestId);
    if(!prior)return undefined;
    const stored=record(json(prior));if(stored.fingerprint!==fingerprint)fail(409,'requestConflict','This request already has different contents.');const receipt=storedReceipt(stored);return acceptance(receipt);
  };
  const requiresContext = (configuration: TurnConfiguration) => Boolean(configuration.skills.length || configuration.memory?.text);
  const unsupportedContext = (configuration: TurnConfiguration, runtime: RuntimeDescriptor = provider): string | undefined => {
    if (configuration.skills.length && !runtime.capabilities.includes('skills')) return 'The selected runtime does not support skills. Restore support or send without selected skills.';
    if (configuration.memory?.text && !runtime.capabilities.includes('memory')) return 'The selected runtime does not support saved memory. Restore support or explicitly clear the note before sending.';
    if ((configuration.skills.length || configuration.memory?.text) && !context) return 'Context preparation is unavailable. Restore the selected module or explicitly remove the context requirement before sending.';
    if (requiresContext(configuration) && contextFailed) return 'Context preparation failed. Restore the module before sending work that requires it.';
    if (configuration.contextModule && (configuration.contextModule.id !== contextRef?.id || configuration.contextModule.version !== contextRef.version)) return 'The pinned context compiler changed. Review a new submission before running this work.';
    return undefined;
  };
  const recordSubmission=(command:{conversationId:string;requestId:string;bindingRevisionId:string;inputText:string;fingerprint:string;configuration:TurnConfiguration;routine?:RoutineExecution}):Acceptance=>acceptanceTransaction(()=>{
    const {conversationId,requestId,bindingRevisionId,inputText,fingerprint,configuration:requestedConfiguration,routine}=command;
    const lane = laneOf(bindingRevisionId);
    if (!lane) fail(409, 'runtimeUnavailable', RUNTIME_ABSENT);
    const account = laneAccounts(lane).selected();
    if (ownerPairingPolicy && lane.runtime?.signIn && !account) fail(409, 'accountSelectionRequired', 'Select the reported provider account on the owning Mac before sending.');
    const pinnedConfiguration = { ...requestedConfiguration, ...(account ? { account } : {}) };
    const configuration = requiresContext(pinnedConfiguration) && contextRef && !pinnedConfiguration.contextModule ? { ...pinnedConfiguration, contextModule: contextRef } : pinnedConfiguration;
    const unavailable = unsupportedContext(configuration, lane.provider);
    if (unavailable) fail(409, 'capabilityUnavailable', unavailable);
    if(db.prepare("SELECT 1 FROM entities WHERE kind='requestCancellations' AND id=?").get(requestId))fail(409,'requestCanceled','This request was canceled before acceptance and will not run.');
    if (Number(db.prepare('SELECT count(*) AS count FROM dispatch_outbox').get()?.count) >= 1000) fail(429,'queueFull','This installation has 1000 unresolved turns. Complete, cancel or resolve earlier work before adding another.');
    const queued = db.prepare('SELECT count(*) AS count FROM dispatch_outbox WHERE conversation_id=?').get(conversationId);
    if (Number(queued?.count) >= 100) fail(429,'queueFull','This conversation has 100 unresolved turns. Complete, cancel or resolve earlier work before adding another.');
    try { decodeConfiguration(configuration); }
    catch(error){if(error instanceof APIError&&error.status===400)fail(400,'invalidConfiguration','Combined Agent and routine instructions must fit within 16384 bytes. Shorten the saved configuration before running it.');throw error;}
    const messageId=randomUUID(),runId=randomUUID(),acceptedAt=new Date().toISOString(),sequence=append(conversationId,'messageAccepted',messageId);
    const message:Message={messageId,conversationId,requestId,role:'user',text:inputText,sequence,createdAt:acceptedAt,recipientAgentId:configuration.agentId,recipientAgentName:configuration.agentName};
    const run:Run={runId,requestId,conversationId,bindingRevisionId,state:'queued',selectedModel:configuration.selectedModel,effectiveModel:null,modelEvidence:'unreported',attemptId:null,configuration,...(routine?{routine}:{})};
    put('messages',messageId,message);put('runs',runId,run);
    const receipt:Receipt={requestId,conversationId,messageId,runId,bindingRevisionId,inputDigest:digest(inputText),acceptedAt};
    put('requests',requestId,{fingerprint,receipt});return acceptance(receipt);
  });
  const saveInstructions=(request:IncomingMessage,body:Record<string,unknown>,skillId?:string)=>owner(request,()=>{
    if(Object.keys(body).some(key=>!['name','instructions','expectedRevisionId'].includes(key)))fail(400,'invalidCommand','Use only the instruction authoring fields. Review authority separately.');
    const old=skillId?load('skills',skillId,decodeSkill):undefined;
    if(old&&body.expectedRevisionId!==old.revisionId)fail(409,'skillChanged','Reload the skill before saving.');
    if(old?.bundle&&(old.bundle.source!=='owner-local:Fort-instructions'||old.bundle.files.length!==1||old.bundle.requestedTools.length!==0||old.bundle.scripts.length!==0||old.bundle.files[0]?.executable||JSON.stringify(old.bundle.supportedRuntimes)!==JSON.stringify([provider.provider])))fail(409,'bundleEditRequired','Import the complete revised bundle so its files and authority can be reviewed.');
    const name=text(body.name,'name',128),instructions=text(body.instructions,'instructions'),version=Number(old?.bundle?.version??'0');
    const bundle=parseSkillBundle({schemaVersion:1,name,version:Number.isSafeInteger(version)&&version>=0&&version<Number.MAX_SAFE_INTEGER?String(version+1):randomUUID(),source:old?.bundle?.source??'owner-local:Fort-instructions',license:old?.bundle?.license??'LicenseRef-Owner-Private',entrypoint:old?.bundle?.entrypoint??'SKILL.md',supportedRuntimes:[provider.provider],requestedTools:[],scripts:[],files:[{path:old?.bundle?.entrypoint??'SKILL.md',kind:'file',executable:false,contentBase64:Buffer.from(instructions).toString('base64')}]});
    const skill:InstructionSkill={skillId:skillId??randomUUID(),revisionId:randomUUID(),name,instructions,bundle,review:{state:'reviewRequired',digest:bundle.digest},...(old?{previousRevisionId:old.revisionId}:{})};
    if(old)put('skillRevisions',old.revisionId,old);
    put('skillRevisions',skill.revisionId,skill);put('skills',skill.skillId,skill);return skill;
  });
  const accept = (request: IncomingMessage, conversationId: string, body: Record<string, unknown>): Acceptance => {
    const requestId = id(body.requestId), bindingRevisionId = id(body.bindingRevisionId), recipientAgentId = id(body.recipientAgentId), recoveryEpoch = id(body.recoveryEpoch), inputText = messageText(body.text);
    if (!Array.isArray(body.skills) || body.skills.length > 8) fail(400, 'invalidSkills', 'Select at most eight instruction skills.');
    const references = body.skills.map(value => { const v = record(value); return { skillId: id(v.skillId), revisionId: id(v.revisionId) }; });
    if (new Set(references.map(s => s.skillId)).size !== references.length) fail(400, 'invalidSkills', 'Select each skill once.');
    const memoryRevisionId=body.memoryRevisionId===undefined?undefined:id(body.memoryRevisionId);
    const fingerprint = digest(JSON.stringify({ conversationId, bindingRevisionId, recipientAgentId, recoveryEpoch, text: inputText, skills: references, memoryRevisionId }));
    return owner(request, () => {
      if (recoveryEpoch !== installation.recoveryEpoch) fail(409, 'recoveryEpochChanged', 'Review this pending message after recovery.');
      const prior=priorAcceptance(requestId,fingerprint);if(prior)return prior;
      const conversation = load('conversations', conversationId, decodeConversation), agent = load('agents', recipientAgentId, decodeAgent);
      if (!conversation.participantAgentIds.includes(recipientAgentId)) fail(409, 'recipientMismatch', 'Select a participant in this conversation.');
      if (agent.binding.bindingRevisionId !== bindingRevisionId || !agent.binding.enabled) fail(409, 'bindingChanged', 'Review the changed Agent before submitting.');
      const skills = references.map(ref => { const skill = load('skills', ref.skillId, decodeSkill); if (skill.revisionId !== ref.revisionId) fail(409, 'skillChanged', 'Review the changed skill before submitting.'); if(!skill.bundle||skill.review?.state!=='enabled')fail(409,'skillReviewRequired','Migrate, review and enable the exact complete skill bundle before submitting.'); return skill; });
      const memory=conversationMemory(conversationId);
      if(memory?.revisionId!==memoryRevisionId)fail(409,'memoryChanged','Review the changed conversation memory before submitting.');
      const configuration: TurnConfiguration = { ...(memory?{memory}:{}),agentId: agent.agentId, agentName: agent.name, bindingRevisionId, selectedModel: agent.binding.selectedModel, instructions: agent.instructions ?? '', skills };
      if (options.admitSubmission && !options.admitSubmission({ text:inputText,conversation,configuration },list('runs',decodeRun))) fail(403,'sampleNotAuthorized','This message is outside the active sample or session.');
      return recordSubmission({conversationId,requestId,bindingRevisionId,inputText,fingerprint,configuration});
    });
  };
  const acceptRoutine=(routineId:string,body:Record<string,unknown>):Acceptance=>{
    const requestId=id(body.requestId),revisionId=id(body.routineRevisionId);
    if(body.recoveryEpoch!==installation.recoveryEpoch)fail(409,'recoveryEpochChanged','Review this routine after installation recovery.');
    const saved=load('routineRevisions',revisionId,decodeRoutine);if(saved.routineId!==routineId)fail(409,'routineChanged','Select the exact saved routine revision.');
    const inputs=parseRoutineInputs(saved.definition,body.inputs),fingerprint=digest(JSON.stringify({command:'routine',routineId,revisionId,recoveryEpoch:body.recoveryEpoch,inputs}));
    const prior=priorAcceptance(requestId,fingerprint);if(prior)return prior;
    if (saved.skills.length && !saved.contextModule) fail(409, 'contextChanged', 'This routine has no context compiler pin. Review and save a new revision before running it.');
    if(bindingDisabled(saved.definition.bindingRevisionId))fail(409,'bindingDisabled','The routine’s pinned binding revision is disabled.');
    for(const ref of saved.definition.skills){const skill=load('skillRevisions',ref.revisionId,decodeSkill);if(skill.review?.state!=='enabled'||skill.bundle?.digest!==ref.digest)fail(409,'skillReviewRequired','A pinned skill revision is no longer enabled.');}
    const agent=saved.agentSnapshot,definition=saved.definition,configuration:TurnConfiguration={agentId:agent.agentId,agentName:agent.name,bindingRevisionId:definition.bindingRevisionId,selectedModel:agent.binding.selectedModel,instructions:agent.instructions+'\n\nRoutine instructions:\n'+definition.instructions,skills:saved.skills,...(saved.contextModule?{contextModule:saved.contextModule}:{})};
    const inputText=messageText('Run routine: '+definition.name+'\nInputs: '+JSON.stringify(inputs)),conversation=load('conversations',definition.conversationId,decodeConversation);
    if(options.admitSubmission&&!options.admitSubmission({text:inputText,conversation,configuration},list('runs',decodeRun)))fail(403,'sampleNotAuthorized','This routine is outside the active session.');
    return recordSubmission({conversationId:definition.conversationId,requestId,bindingRevisionId:definition.bindingRevisionId,inputText,fingerprint,configuration,routine:startRoutine(routineId,revisionId,definition,inputs)});
  };
  const agentIds=()=>new Set(list('agents',decodeAgent).map(agent=>agent.agentId));
  const weeks=()=>list('weeks',decodeWeek).sort((a,b)=>b.number-a.number);
  const foundNextWeek=(request:IncomingMessage,body:Record<string,unknown>)=>owner(request,()=>{
    const existing=weeks();if(existing.some(week=>week.state==='live'))fail(409,'weekLive','Close the live week before founding the next one.');
    const week=foundWeek(body,agentIds(),(existing[0]?.number??0)+1);put('weeks',week.weekId,week);return viewWeek(week);
  });
  const changeWeek=(request:IncomingMessage,weekId:string,body:Record<string,unknown>,change:(week:Week)=>Week)=>owner(request,()=>{
    const week=load('weeks',weekId,decodeWeek);
    if(body.expectedRevisionId!==week.revisionId)fail(409,'weekChanged','Reload the week before changing it.');
    if(week.state!=='live')fail(409,'weekClosed','This week is closed.');
    const next=change(week);put('weeks',next.weekId,next);return viewWeek(next);
  });
  const saveSchedule=(request:IncomingMessage,body:Record<string,unknown>,scheduleId?:string)=>owner(request,()=>{
    const definition=parseSchedule(body.definition),routine=load('routineRevisions',definition.routineRevisionId,decodeRoutine);
    if(routine.routineId!==definition.routineId)fail(409,'routineChanged','Select the exact saved routine revision.');
    definition.inputs=parseRoutineInputs(routine.definition,definition.inputs);
    if(scheduleId&&load('schedules',scheduleId,decodeSchedule).revisionId!==body.expectedRevisionId)fail(409,'scheduleChanged','Reload the schedule before saving.');
    const saved:ScheduleRevision={scheduleId:scheduleId??randomUUID(),revisionId:randomUUID(),digest:routineDigest(definition),definition,state:'paused',checkedThrough:now()};
    put('schedules',saved.scheduleId,saved);put('scheduleRevisions',saved.revisionId,saved);return saved;
  });
  const schedulePreview=(scheduleId:string)=>transaction(()=>{
    const schedule=load('schedules',scheduleId,decodeSchedule),occurrences=previewCalendar(schedule.definition,now());
    const preview:SchedulePreview={revisionId:schedule.revisionId,digest:routineDigest({revisionId:schedule.revisionId,at:now(),occurrences}),expiresAt:new Date(now()+600_000).toISOString(),timezone:schedule.definition.timezone,occurrences,overlap:'skip',missed:schedule.definition.missed};
    put('schedules',scheduleId,{...schedule,preview});return preview;
  });
  const tickSchedules=(downtime=false):number=>{
    if(closed||dispatchStopped||(credentialDeadline!==undefined&&now()>=credentialDeadline))return 0;
    let admitted=0;
    for(const saved of list('schedules',decodeSchedule)){
      if(saved.state!=='enabled'||!previewCalendar(saved.definition,saved.checkedThrough).some(occurrence=>Date.parse(occurrence.decisionAt)<=now()))continue;
      transaction(()=>{
        const schedule=load('schedules',saved.scheduleId,decodeSchedule),through=now();
        if(schedule.state!=='enabled'||through<=schedule.checkedThrough)return;
        let after=schedule.checkedThrough;
        if(schedule.definition.kind!=='once'&&through-after>366*86_400_000){
          const cutoff=through-366*86_400_000,occurrenceId=routineDigest({scheduleRevisionId:schedule.revisionId,from:after,through:cutoff});
          const summary:ScheduleOccurrence={occurrenceId,scheduleId:schedule.scheduleId,scheduleRevisionId:schedule.revisionId,routineId:schedule.definition.routineId,routineRevisionId:schedule.definition.routineRevisionId,intendedAt:null,localDate:'',localTime:schedule.definition.time,state:'skippedMissedRange',detail:'Skipped the downtime range '+new Date(after).toISOString()+' through '+new Date(cutoff).toISOString()+'. No backlog was dispatched.'};
          put('scheduleOccurrences',occurrenceId,summary);after=cutoff;
        }
        const due=schedule.definition.kind==='once'?previewCalendar(schedule.definition,after).filter(occurrence=>Date.parse(occurrence.decisionAt)<=through):calendarBetween(schedule.definition,after,through);
        const missed=downtime||due.some(occurrence=>through-Date.parse(occurrence.decisionAt)>60_000);
        const catchUp=missed&&schedule.definition.missed==='catchUpOne'?due.findLast(occurrence=>occurrence.intendedAt!==null):undefined;
        for(const occurrence of due){
          const occurrenceId=routineDigest(occurrence.intendedAt===null?{routineRevisionId:schedule.definition.routineRevisionId,gap:occurrence.localDate+'T'+occurrence.localTime,timezone:schedule.definition.timezone}:{routineRevisionId:schedule.definition.routineRevisionId,intendedAt:occurrence.intendedAt});
          if(db.prepare("SELECT 1 FROM entities WHERE kind='scheduleOccurrences' AND id=?").get(occurrenceId))continue;
          let record:ScheduleOccurrence={occurrenceId,scheduleId:schedule.scheduleId,scheduleRevisionId:schedule.revisionId,routineId:schedule.definition.routineId,routineRevisionId:schedule.definition.routineRevisionId,intendedAt:occurrence.intendedAt,localDate:occurrence.localDate,localTime:occurrence.localTime,state:'accepted'};
          if(occurrence.intendedAt===null)record.state='skippedGap';
          else if(missed&&occurrence!==catchUp)record.state='skippedMissed';
          else if(db.prepare("SELECT 1 FROM dispatch_outbox o JOIN entities e ON e.kind='runs' AND e.id=o.run_id WHERE json_extract(e.data,'$.routine.routineId')=? LIMIT 1").get(schedule.definition.routineId))record.state='skippedOverlap';
          else {
            try {
              const acceptance=acceptRoutine(schedule.definition.routineId,{requestId:'occurrence-'+occurrenceId,routineRevisionId:schedule.definition.routineRevisionId,recoveryEpoch:installation.recoveryEpoch,inputs:schedule.definition.inputs});
              record.runId=acceptance.receipt.runId;admitted++;
            }catch(error){if(error instanceof APIError&&error.status<500){record.state='blocked';record.detail=error.message;}else throw error;}
          }
          db.prepare('INSERT INTO entities VALUES (?,?,?)').run('scheduleOccurrences',occurrenceId,JSON.stringify(record));
        }
        put('schedules',schedule.scheduleId,{...schedule,checkedThrough:through});
      });
    }
    if(admitted)queueMicrotask(wake);return admitted;
  };
  let scheduleStatus: SchedulerStatus = { state: options.autoRun ? 'idle' : 'manual' };
  let schedulerFault = false;
  const armScheduleClock = () => {
    clearTimeout(scheduleTimer);
    if (closed || dispatchStopped || (credentialDeadline !== undefined && now() >= credentialDeadline)) { scheduleStatus = { state: 'stopped' }; return; }
    if (!options.autoRun) { scheduleStatus = { state: 'manual' }; return; }
    let delay = 60_000;
    try {
      if (!schedulerFault) {
        let due = Infinity;
        for (const schedule of list('schedules', decodeSchedule)) {
          if (schedule.state !== 'enabled') continue;
          const occurrence = previewCalendar(schedule.definition, schedule.checkedThrough)[0];
          if (occurrence) due = Math.min(due, Date.parse(occurrence.decisionAt));
        }
        if (due === Infinity) { scheduleStatus = { state: 'idle' }; return; }
        delay = Math.max(1, Math.min(delay, due - now()));
        scheduleStatus = { state: 'waiting', nextWakeAt: new Date(now() + delay).toISOString() };
      }
    } catch { schedulerFault = true; }
    if (schedulerFault) scheduleStatus = { state: 'fault', retryAt: new Date(now() + delay).toISOString(), detail: 'Schedule evaluation failed. No uncertain work is retried; the next calendar check is in one minute.' };
    scheduleTimer = setTimeout(() => runScheduleClock(), delay);
    scheduleTimer.unref();
  };
  const runScheduleClock = (downtime = false) => {
    const wasFault = schedulerFault;
    try { tickSchedules(downtime); schedulerFault = false; }
    catch { schedulerFault = true; }
    armScheduleClock();
    if (wasFault !== schedulerFault) transaction(() => append('_workspace', 'schedulerChanged', installation.installationId));
  };
  let runtimeOperations = 0, portraitDrawing = false;
  const canPrepareRuntime = () => Boolean(!closed && !starting && !stopCommandActive && !accountsBusy() && runtimeOperations === 0 && options.runtime
    && (runtimeStop === undefined ? (!options.runtime.readiness || options.runtime.readiness().state === 'ready')
      : runtimeStop.state === 'confirmed' && options.runtime.restart && options.runtime.readiness?.().canRestart));
  const canResumeOwner = () => Boolean(ownerPairingPolicy && ownerControl?.intent === 'stop' && canPrepareRuntime());
  const runtimeReadiness=():RuntimeReadiness=>dispatchStopped
    ? {state:'stopped',canRestart:canResumeOwner(),detail:'Local work is stopped. History and owner controls remain available. Resume explicitly or finish disconnect.'}
    : accountsBusy() || runtimeOperations > 0 ? {state:'busy',canRestart:false,detail:'An explicit runtime operation is in progress.'}
    : options.runtime?.readiness?.()??{state:options.runtime?'ready':'unavailable',canRestart:false,detail:options.runtime?'Runtime is available.':'No runtime is connected.'};
  let dispatching: Promise<boolean> | undefined; let active: AbortController | undefined; let activeRunId:string|undefined;
  const executeNext = async (): Promise<boolean> => {
    if (closed || dispatchStopped || accountsBusy() || runtimeOperations > 0 || (options.runtime?.readiness && runtimeReadiness().state!=='ready')) return false;
    const nextRow = db.prepare(`SELECT e.data FROM dispatch_outbox o JOIN entities e ON e.kind='runs' AND e.id=o.run_id
      WHERE o.state='queued' AND o.held=0 AND NOT EXISTS
        (SELECT 1 FROM dispatch_outbox prior WHERE prior.conversation_id=o.conversation_id AND prior.position<o.position)
      ORDER BY o.position LIMIT 1`).get();
    const queued = nextRow ? decodeRun(json(nextRow)) : undefined;
    if (!queued?.configuration) return false;
    const lane = laneOf(queued.bindingRevisionId);
    if (lane?.runtime?.readiness && lane.runtime !== options.runtime && lane.runtime.readiness().state !== 'ready') return false;
    const contextFailure = !lane ? RUNTIME_ABSENT : ownerPairingPolicy && lane.runtime?.signIn && !sameAccount(queued.configuration.account, laneAccounts(lane).selected())
      ? 'The selected provider account changed or is unconfirmed. Nothing was sent. Review a new submission.' : unsupportedContext(queued.configuration, lane.provider);
    if (contextFailure) {
      transaction(() => { put('runs', queued.runId, { ...queued, state: 'failed', statusDetail: contextFailure,
        ...(queued.routine ? { routine: stopRoutine(queued.routine, 'failed', contextFailure) } : {}) }); append(queued.conversationId, 'runStopped', queued.runId); });
      return true;
    }
    if(bindingDisabled(queued.bindingRevisionId)){
      const detail='The pinned binding revision was disabled before dispatch. Nothing was sent.';
      transaction(()=>{put('runs',queued.runId,{...queued,state:'failed',statusDetail:detail,...(queued.routine?{routine:stopRoutine(queued.routine,'failed',detail)}:{})});append(queued.conversationId,'runStopped',queued.runId);});return true;
    }
    if(queued.configuration.skills.some(skill=>skill.bundle&&load('skillRevisions',skill.revisionId,decodeSkill).review?.state!=='enabled')){
      transaction(()=>{put('runs',queued.runId,{...queued,state:'failed',statusDetail:'A selected skill revision was disabled before dispatch. Nothing was sent.',...(queued.routine?{routine:stopRoutine(queued.routine,'failed','A selected skill revision was disabled before dispatch. Nothing was sent.')}: {})});append(queued.conversationId,'runStopped',queued.runId);});
      return true;
    }
    const stepIndex=queued.routine?.steps.findIndex(step=>step.state!=='completed'&&step.state!=='approved')??-1;
    const nextStep=stepIndex>=0?queued.routine?.definition.steps[stepIndex]:undefined;
    if(nextStep?.type==='approval'&&queued.routine){
      transaction(()=>{
        const attemptId=randomUUID(),approvalId=randomUUID(),routine=queued.routine!;
        const approval:RoutineApproval={kind:'routineGate',approvalId,installationId:installation.installationId,recoveryEpoch:installation.recoveryEpoch,runId:queued.runId,attemptId,bindingRevisionId:queued.bindingRevisionId,routineRevisionId:routine.revisionId,stepId:nextStep.stepId,resourceScope:['routine:continue'],actionDescription:nextStep.prompt,actionDigest:routineDigest({installationId:installation.installationId,recoveryEpoch:installation.recoveryEpoch,runId:queued.runId,attemptId,routineRevisionId:routine.revisionId,step:nextStep,inputs:routine.inputs,previous:routine.steps}),expiresAt:new Date(now()+86_400_000).toISOString(),state:'pending',decidedByDeviceId:null,decidedAt:null};
        const parked:RoutineExecution={...routine,steps:routine.steps.map((step,index)=>index===stepIndex?{stepId:step.stepId,stepRevisionId:step.stepRevisionId,state:'waitingForApproval',attemptId,approvalId}:step)};
        put('routineApprovals',approvalId,approval);put('runs',queued.runId,{...queued,state:'waitingForApproval',attemptId,routine:parked,statusDetail:nextStep.prompt});append(queued.conversationId,'approvalRequested',approvalId);
      });return true;
    }
    if(!lane?.runtime)return false;
    const runtime = lane.runtime;
    let preparedContext: PreparedContext = { kind: 'empty' };
    if (requiresContext(queued.configuration)) {
      let failure: string | undefined;
      if (!queued.configuration.contextModule || !context || !contextRef) failure = 'This work has no confirmed context compiler pin. Review a new submission before running it.';
      else {
        try {
          const compiled = context.compile({ provider: lane.provider.provider, conversationId: queued.conversationId, configuration: immutable(queued.configuration) });
          if (compiled.kind === 'unsupported') failure = compiled.message;
          else preparedContext = { kind: 'prepared', module: contextRef, configurationDigest: executionDigest(queued.configuration), content: compiled };
        } catch { contextFailed = true; failure = 'Context preparation failed. Nothing was sent to the runtime.'; }
      }
      if (failure) {
        const detail = failure;
        transaction(() => { put('runs', queued.runId, { ...queued, state: 'failed', statusDetail: detail,
          ...(queued.routine ? { routine: stopRoutine(queued.routine, 'failed', detail) } : {}) }); append(queued.conversationId, 'runStopped', queued.runId); if (contextFailed) append('_workspace', 'moduleStatusChanged', 'context'); });
        return true;
      }
    }
    const { input, dispatch } = transaction(() => {
      const inputRow = db.prepare("SELECT data FROM entities WHERE kind='messages' AND json_extract(data,'$.requestId')=? AND json_extract(data,'$.role')='user'").get(queued.requestId);
      const current = inputRow ? decodeMessage(json(inputRow)) : undefined;
      if (!current || !queued.configuration) throw new Error('Queued run has no accepted input');
      const attemptId=randomUUID(),routine=queued.routine?{...queued.routine,steps:queued.routine.steps.map((step,index)=>index===stepIndex?{stepId:step.stepId,stepRevisionId:step.stepRevisionId,state:'running' as const,attemptId}:step)}:undefined;
      const attemptGeneration = (queued.attemptGeneration ?? 0) + 1;
      const binding = load('agentBindings', queued.bindingRevisionId, decodeAgent).binding;
      put('runs', queued.runId, { ...queued, state: 'running', attemptId, attemptGeneration,...(routine?{routine}:{}) }); append(queued.conversationId, 'runStarted', queued.runId);
      const users = db.prepare("SELECT data FROM entities WHERE kind='messages' AND json_extract(data,'$.conversationId')=? AND json_extract(data,'$.sequence')<? AND json_extract(data,'$.role')='user' ORDER BY json_extract(data,'$.sequence')").all(queued.conversationId,current.sequence).map(row=>decodeMessage(json(row)));
      const replies = db.prepare("SELECT data FROM entities WHERE kind='messages' AND json_extract(data,'$.requestId') IN (SELECT value FROM json_each(?)) AND json_extract(data,'$.role')='assistant' ORDER BY json_extract(data,'$.sequence')").all(JSON.stringify(users.map(message=>message.requestId))).map(row=>decodeMessage(json(row)));
      const repliesByRequest = new Map<string,Message[]>();
      for (const reply of replies) { const group=repliesByRequest.get(reply.requestId) ?? []; group.push(reply); repliesByRequest.set(reply.requestId,group); }
      const history = users.flatMap(message=>[message,...(repliesByRequest.get(message.requestId) ?? [])]);
      const next: ChatInput = { context: preparedContext, attempt: { runId: queued.runId, attemptId, generation: attemptGeneration, bindingRevisionId: queued.bindingRevisionId, runtimeVersion: binding.runtimeVersion, selectedModel: queued.selectedModel }, runId: queued.runId, requestId: queued.requestId, conversationId: queued.conversationId, configuration: queued.configuration, text: routine?taskInput(routine,routine.steps[stepIndex]!.stepId):current.text, history };
      const dispatch = sealDispatch({ protocolVersion: 1, installationId: installation.installationId, recoveryEpoch: installation.recoveryEpoch,
        binding, attempt: next.attempt, inputDigest: executionDigest(next), configurationDigest: executionDigest(next.configuration),
        historyDigest: executionDigest(history), policy: TEXT_POLICY, policyDigest: executionDigest(TEXT_POLICY),
        deadline: now() + attemptTimeoutMs, limits: EXECUTION_LIMITS });
      put('contexts', attemptId, {
        runId: queued.runId, attempt: next.attempt, inputMessageId: current.messageId,
        inputDigest: digest(next.text), configurationDigest: digest(JSON.stringify(next.configuration)),
        historyDigest: digest(JSON.stringify(history)),
        historyWindow: { beforeUserSequence: current.sequence, throughEventSequence: cursor() }, dispatch
      });
      return { input: immutable(next), dispatch };
    });
    active = new AbortController(); activeRunId=queued.runId;
    try {
      const result = Buffer.byteLength(JSON.stringify(input)) > EXECUTION_LIMITS.inputBytes
        ? { kind: 'notStarted' as const, message: 'Conversation context exceeds the execution limit. Nothing was sent.' }
        : await runtime.execute(input, active.signal, dispatch);
      transaction(() => {
        const run = load('runs', queued.runId, decodeRun);
        if (run.attemptId !== dispatch.attempt.attemptId || run.attemptGeneration !== dispatch.attempt.generation || (run.state !== 'running' && run.state !== 'cancelRequested')) return;
        if (result.kind === 'completed') {
          const messageId = randomUUID(), sequence = append(run.conversationId, 'messageCompleted', messageId);
          const reply: Message = { messageId, conversationId: run.conversationId, requestId: run.requestId, role: 'assistant', text: messageText(result.text), sequence, createdAt: new Date().toISOString(), agentId: input.configuration.agentId, agentName: input.configuration.agentName };
          const routine=run.routine?{...run.routine,steps:run.routine.steps.map(step=>step.state==='running'?{stepId:step.stepId,stepRevisionId:step.stepRevisionId,state:'completed' as const,attemptId:step.attemptId,output:{messageId,text:reply.text,digest:outputDigest(reply.text)}}:step)}:undefined;
          const remaining=routine?.steps.some(step=>step.state==='pending')??false;
          const canceled=remaining&&run.state==='cancelRequested';
          put('messages',messageId,reply);put('runs',run.runId,{...run,state:canceled?'canceled':remaining?'queued':'completed',...(routine?{routine:canceled?stopRoutine(routine,'canceled','Owner cancellation stopped remaining steps.'):routine}:{}),statusDetail:canceled?'Current task completed; remaining routine steps were canceled.':undefined});
        } else {
          const state=result.kind==='notStarted'?(run.state==='cancelRequested'||closed||dispatchStopped?'canceled':'failed'):result.kind;
          put('runs',run.runId,{...run,state,statusDetail:result.message,...(run.routine?{routine:stopRoutine(run.routine,state,result.message)}:{})});append(run.conversationId,'runStopped',run.runId);
        }
      });
    } catch {
      transaction(() => { const run = load('runs', queued.runId, decodeRun); if (run.attemptId !== dispatch.attempt.attemptId || run.attemptGeneration !== dispatch.attempt.generation || (run.state !== 'running' && run.state !== 'cancelRequested')) return; put('runs', run.runId, { ...run, state: 'outcomeUnknown', statusDetail: 'The runtime ended without a confirmed result.',...(run.routine?{routine:stopRoutine(run.routine,'outcomeUnknown','The runtime ended without a confirmed result.')}: {}) }); append(run.conversationId, 'runStopped', run.runId); });
    } finally { active = undefined; activeRunId=undefined; }
    return true;
  };
  const dispatchNext = (): Promise<boolean> => {
    if (dispatching) return dispatching;
    dispatching = executeNext().finally(() => { dispatching = undefined; }); return dispatching;
  };
  let pumping = false;
  const wake = () => {
    if (!options.autoRun || pumping || closed || dispatchStopped) return;
    pumping = true;
    void (async () => { try { while (await dispatchNext()) { /* One serialized durable claim per turn. */ } } finally { pumping = false; } })().catch(() => { active?.abort(); });
  };
  const readBody = async (request: IncomingMessage) => {
    const chunks: Buffer[] = []; let size = 0;
    for await (const chunk of request) { size += chunk.length; if (size > 1_000_000) fail(413, 'tooLarge', 'Command is too large.'); chunks.push(chunk); }
    try { return record(JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(Buffer.concat(chunks)))); }
    catch (error) { if (error instanceof APIError) throw error; fail(400, 'invalidJSON', 'Use valid UTF-8 JSON.'); }
  };
  const issueEnrollment = () => {
    if(closed||dispatchStopped)throw new Error('Service is stopped');
    if(ownerPairingPolicy&&db.prepare('SELECT 1 FROM devices WHERE expires=0 OR expires>?').get(now()))throw new Error('Disconnect the paired device before enrolling another');
    if (credentialDeadline!==undefined) {
      if (db.prepare("SELECT 1 FROM metadata WHERE key='credentialClosed'").get() || credentialDeadline<=now()) throw new Error('Bounded enrollment is closed');
      if (db.prepare("SELECT 1 FROM metadata WHERE key='enrollmentIssued'").get()) throw new Error('Bounded enrollment already issued');
    }
    const code = randomBytes(32).toString('base64url');
    transaction(()=>{
      if(ownerPairingPolicy)db.prepare('DELETE FROM enrollment').run();
      db.prepare('INSERT INTO enrollment VALUES (?,?)').run(digest(code), Math.min(now()+300_000,credentialDeadline ?? Infinity));
      if (credentialDeadline!==undefined) db.prepare('INSERT INTO metadata VALUES (?,?)').run('enrollmentIssued','true');
    });
    return { installationId: installation.installationId, code };
  };
  const canPairAfterDisconnect = () => ownerControl?.intent === 'disconnect' && canPrepareRuntime()
    && !db.prepare('SELECT id FROM devices WHERE id=? OR expires=0 OR expires>?').get(ownerControl.deviceId,now())
    && readOwnerSetup()?.state !== 'canceling';
  const setupPolicy = () => {
    if (closed || starting || stopCommandActive || dispatchStopped && !canPairAfterDisconnect()) fail(409, 'hostUnavailable', 'Owner setup requires a running local host or a confirmed disconnect.');
    if (!ownerPairingPolicy) fail(403, 'temporaryPairing', 'Owner setup requires an explicitly configured owner-local installation.');
    return ownerPairingPolicy;
  };
  const readOwnerSetup = (): OwnerSetup | undefined => {
    const row = db.prepare("SELECT data FROM metadata WHERE key='ownerSetup'").get();
    if (!row) return undefined;
    try {
      const value = record(json(row));
      if (value.version !== 1 || typeof value.expires !== 'number' || !Number.isSafeInteger(value.expires) || value.expires <= 0) throw new Error();
      const base = { version: 1 as const, setupId: id(value.setupId), expires: value.expires };
      if (value.state === 'pending') return { ...base, state: 'pending' };
      if (value.state === 'completed' || value.state === 'canceling') return { ...base, state: value.state, deviceId: id(value.deviceId) };
      if (value.state === 'canceled' && (value.outcome === 'notPaired' || value.outcome === 'revoked')) return { ...base, state: 'canceled', deviceId: id(value.deviceId), outcome: value.outcome };
      throw new Error();
    } catch { fail(409, 'invalidSetupState', 'Stored owner setup cannot be recovered.'); }
  };
  const saveOwnerSetup = (setup: OwnerSetup) => db.prepare("INSERT OR REPLACE INTO metadata VALUES ('ownerSetup',?)").run(JSON.stringify(setup));
  const setupReceipt = (deviceId: string): OwnerSetupReceipt => ({ installationId: installation.installationId, deviceId, expiresAt: pairingStatus(deviceId).expiresAt });
  const beginOwnerSetup = (): OwnerSetupState => {
    setupPolicy();
    return transaction(() => {
      const paired = ownerControl
        ? db.prepare('SELECT id FROM devices WHERE id=? AND (expires=0 OR expires>?)').get(ownerControl.deviceId,now())
        : db.prepare('SELECT id FROM devices WHERE expires=0 OR expires>?').get(now());
      if (paired) return { kind: 'paired', ...setupReceipt(id(paired.id)) };
      if(db.prepare('SELECT 1 FROM devices WHERE expires=0 OR expires>?').get(now()))fail(409,'ownerRecoveryRequired','Complete exact local host recovery before replacing the Mac pairing.');
      let setup = readOwnerSetup();
      if (setup?.state === 'canceling') fail(409, 'setupCanceling', 'Complete the pending setup cancellation before starting a new setup.');
      if (!setup || setup.state !== 'pending' || setup.expires <= now()) {
        setup = { version: 1, state: 'pending', setupId: randomUUID(), expires: now() + 300_000 };
        saveOwnerSetup(setup);
      }
      return { kind: 'available', installationId: installation.installationId, setupId: setup.setupId, expiresAt: new Date(setup.expires).toISOString() };
    });
  };
  const completeOwnerSetup = async (request: OwnerSetupRequest): Promise<OwnerSetupReceipt> => {
    const policy = setupPolicy(), value = record(request);
    if (value.installationId !== installation.installationId) fail(409, 'installationMismatch', 'Setup requires this exact installation.');
    const setupId = id(value.setupId), deviceId = id(value.deviceId), token = value.deviceToken;
    if (typeof token !== 'string' || !/^[A-Za-z0-9_-]{43}$/.test(token) || Buffer.from(token, 'base64url').toString('base64url') !== token) fail(400, 'invalidSetupToken', 'Use a newly generated 32-byte device token.');
    const validate = () => {
      const setup = readOwnerSetup();
      if (!setup || setup.setupId !== setupId) fail(409, 'setupMismatch', 'Setup requires the current exact offer.');
      if (setup.state === 'completed') {
        const device = db.prepare('SELECT id FROM devices WHERE id=? AND hash=? AND (expires=0 OR expires>?)').get(deviceId, digest(token), now());
        if (setup.deviceId !== deviceId || ownerControl?.deviceId !== deviceId || !device) fail(409, 'pairingMismatch', 'The completed setup no longer matches an active pairing.');
        return setup;
      }
      if (setup.state !== 'pending') fail(409, 'setupCanceled', 'This owner setup has been canceled.');
      if (setup.expires <= now()) fail(409, 'setupExpired', 'Owner setup has expired. Begin a new setup.');
      if (db.prepare('SELECT 1 FROM devices WHERE expires=0 OR expires>?').get(now())) fail(409, 'alreadyPaired', 'Disconnect the paired device before enrolling another.');
      if (ownerControl?.deviceId === deviceId) fail(409, 'deviceReused', 'Use a new device identity after a prior pairing.');
      return setup;
    };
    const commit = () => {
      const setup = validate();
      if (setup.state === 'completed') return setupReceipt(deviceId);
      const expires = policy.expiresAfterDays === null ? 0 : now() + policy.expiresAfterDays * 86_400_000;
      db.prepare('DELETE FROM devices WHERE expires>0 AND expires<=?').run(now());
      db.prepare('DELETE FROM enrollment').run();
      companions.clear();
      db.prepare('INSERT INTO devices VALUES (?,?,?)').run(deviceId, digest(token), expires);
      saveOwnerSetup({ ...setup, state: 'completed', deviceId });
      saveOwnerControl({ deviceId, intent: 'running' });
      return setupReceipt(deviceId);
    };
    validate();
    return dispatchStopped ? resumeOwnerRuntime(commit) : transaction(commit);
  };
  const listenerAbort = new AbortController();
  const companions=companionPairing(db,installation.installationId,now,()=>ownerPairingPolicy && ownerControl
    ? {deviceId:ownerControl.deviceId,expiresAfterDays:ownerPairingPolicy.expiresAfterDays,running:!closed&&!dispatchStopped&&!stopCommandActive&&!starting}:undefined,
    ()=>append('_workspace','pairingChanged',installation.installationId));
  // One account sign-in and choice per runtime (Spec 064). The primary keeps the original metadata keys.
  const laneAccountList: ReturnType<typeof accountSignIn>[] = [];
  try { for (const [index, lane] of lanes.entries()) laneAccountList.push(accountSignIn(db, { runtime: lane.runtime, now, identity: { installationId: installation.installationId, provider: lane.provider.provider, hostId: lane.provider.hostId },
    ...(index === 0 ? {} : { keys: { attempt: 'accountSignIn:' + lane.provider.provider, choice: 'accountChoice:' + lane.provider.provider } }),
    scope: () => ownerPairingPolicy && ownerControl ? { deviceId: ownerControl.deviceId, recoveryEpoch: installation.recoveryEpoch,
      running: !closed && !dispatchStopped && !stopCommandActive && !starting && !dispatching && runtimeOperations === 0
        && (!lane.runtime?.readiness || lane.runtime.readiness().state === 'ready') } : undefined,
    commit: transaction, changed: () => append('_workspace', 'accountChanged', installation.installationId), settled: () => queueMicrotask(wake) })); } catch (error) { db.close(); throw error; }
  const laneAccounts = (lane: Lane) => laneAccountList[lanes.indexOf(lane)]!;
  const accountsBusy = () => laneAccountList.some(item => item.busy);
  const requestedLane = (url: URL): Lane => {
    const name = url.searchParams.get('provider');
    if (name === null) return lanes[0]!;
    const lane = lanes.find(item => item.provider.provider === name);
    if (!lane) fail(400, 'runtimeUnavailable', 'That provider is not available on this host.');
    return lane;
  };
  const acceptedConnections = new Map<Socket,Promise<void>>();
  const server = createServer(async (request, response) => {
    let status = 200; let result: unknown;
    try {
      const url = new URL(request.url ?? '/', 'http://127.0.0.1');
      if(request.method==='POST' && url.pathname==='/v1/pairing/leave'){
        const credential=/^Bearer ([A-Za-z0-9_-]{43})$/.exec(request.headers.authorization??'')?.[1];
        if(request.headers.origin!==undefined || request.headers['x-fort-installation-id']!==installation.installationId || !credential)fail(401,'unauthorized','Use the exact companion credential.');
        const body=await readBody(request);result=transaction(()=>companions.leave(body,credential));
      } else if(request.method==='POST' && url.pathname==='/v1/devices/join'){
        if(request.headers.origin!==undefined || request.headers['x-fort-installation-id']!==installation.installationId)fail(401,'unauthorized','Use this installation.');
        const body=await readBody(request);result=transaction(()=>companions.join(body));status=201;
      } else if (request.method === 'POST' && url.pathname === '/v1/devices/enroll') {
        const body = await readBody(request);
        result = transaction(() => {
          if (request.headers.origin !== undefined || body.installationId !== installation.installationId || typeof body.code !== 'string') fail(401, 'unauthorized', 'Enrollment is not authorized.');
          text(body.deviceName, 'device name', 128);
          const code = db.prepare('SELECT expires FROM enrollment WHERE hash=?').get(digest(body.code));
          if (closed || dispatchStopped || !code || Number(code.expires) <= now()) fail(401, 'unauthorized', 'Enrollment code is invalid or expired.');
          if(ownerPairingPolicy&&db.prepare('SELECT 1 FROM devices WHERE expires=0 OR expires>?').get(now()))fail(409,'alreadyPaired','Disconnect the paired device before enrolling another.');
          if(ownerPairingPolicy)db.prepare('DELETE FROM devices WHERE expires>0 AND expires<=?').run(now());
          db.prepare('DELETE FROM enrollment WHERE hash=?').run(digest(body.code));
          const deviceId = randomUUID(), deviceToken = randomBytes(32).toString('base64url'), expires = ownerPairingPolicy ? (ownerPairingPolicy.expiresAfterDays===null?0:now()+ownerPairingPolicy.expiresAfterDays*86_400_000) : Math.min(now()+30*86_400_000,credentialDeadline ?? Infinity);
          db.prepare('INSERT INTO devices VALUES (?,?,?)').run(deviceId, digest(deviceToken), expires);
          if (ownerPairingPolicy) {
            db.prepare("DELETE FROM metadata WHERE key='ownerSetup'").run();companions.clear();
            saveOwnerControl({ deviceId, intent: 'running' });
          }
          return { installation, deviceId, deviceToken, expiresAt: expires===0?null:new Date(expires).toISOString() };
        }); status = 201;
      } else {
        authenticate(request);
        if(request.method==='GET' && url.pathname==='/v1/account/sign-in'){
          result=laneAccounts(requestedLane(url)).status(authenticate(request));
        } else if(request.method==='POST' && url.pathname==='/v1/account/sign-in/start'){
          const lane=requestedLane(url),body=await readBody(request); result=laneAccounts(lane).start(authenticate(request),body);
        } else if(request.method==='POST' && url.pathname==='/v1/account/sign-in/select'){
          const lane = requestedLane(url), body = await readBody(request); result = laneAccounts(lane).select(authenticate(request), body);
        } else if(request.method==='POST' && url.pathname==='/v1/account/sign-in/cancel'){
          const body=await readBody(request),deviceId=authenticate(request);
          result=await laneAccounts(requestedLane(url)).cancel(deviceId,body);
          if(authenticate(request)!==deviceId)fail(401,'unauthorized','The owning Mac pairing changed.');
        } else if(request.method==='POST' && url.pathname==='/v1/pairing/invitation'){
          const body=await readBody(request);result=owner(request,deviceId=>companions.invite(deviceId,body));status=201;
        } else if(request.method==='GET' && url.pathname==='/v1/pairing/companion'){
          result=transaction(()=>companions.overview(authenticate(request)));
        } else if(request.method==='POST' && url.pathname==='/v1/pairing/revoke-companion'){
          const body=await readBody(request);result=transaction(()=>companions.revokeCompanion(authenticate(request),body));
        } else if (request.method === 'GET' && url.pathname === '/v1/installation') result = { ...installation, eventCursor: cursor() };
        else if (request.method === 'GET' && url.pathname === '/v1/events') {
          const after = Number(url.searchParams.get('after')), wait = Number(url.searchParams.get('wait') ?? 0);
          if (!url.searchParams.has('after') || !Number.isSafeInteger(after) || after < 0 || !Number.isSafeInteger(wait) || wait < 0 || wait > 25_000) fail(400, 'invalidCursor', 'Use a nonnegative cursor and a wait of at most 25000 milliseconds.');
          const checkCursor = () => {
            if (url.searchParams.get('recoveryEpoch') !== installation.recoveryEpoch) fail(409, 'recoveryEpochChanged', 'Refresh this installation after recovery.');
            const first = Number(db.prepare('SELECT coalesce(min(sequence),1) AS first FROM events').get()?.first);
            if (after < first - 1 || after > cursor()) fail(409, 'cursorExpired', 'Refresh durable state before resuming event replay.');
          };
          checkCursor();
          if (wait && after === cursor()) {
            if (eventWaiters.size >= 8) fail(429, 'tooManyWaits', 'At most eight event waits may be open.');
            await new Promise<void>(resolve => {
              const finish = () => { clearTimeout(timer); eventWaiters.delete(changed); response.off('close', finish); listenerAbort.signal.removeEventListener('abort', finish); resolve(); };
              const changed = () => { if (closed || cursor() > after) finish(); };
              const timer = setTimeout(finish, wait); timer.unref();
              eventWaiters.add(changed); response.once('close', finish); listenerAbort.signal.addEventListener('abort', finish, { once: true });
            });
          }
          if (response.destroyed) return;
          authenticate(request); checkCursor();
          const events = db.prepare('SELECT sequence,data FROM events WHERE sequence>? ORDER BY sequence LIMIT 256').all(after).map(row => ({ ...record(json(row)), sequence: Number(row.sequence) }));
          result = { installationId: installation.installationId, recoveryEpoch: installation.recoveryEpoch, semantics: 'invalidation', cursor: events.at(-1)?.sequence ?? after, events };
        }
        else if(request.method==='GET'&&url.pathname==='/v1/pairing')result=owner(request,pairingStatus);
        else if(request.method==='POST'&&url.pathname==='/v1/pairing/expiry'){
          const body=await readBody(request);
          result=owner(request,deviceId=>{
            if(!ownerPairingPolicy)fail(403,'temporaryPairing','This temporary session cannot be extended.');
            if(Object.keys(body).some(key=>key!=='expiresAfterDays'))fail(400,'invalidExpiry','Use only the pairing expiry field.');
            const days=expiryDays(body.expiresAfterDays);
            const policy:OwnerPairingPolicy={expiresAfterDays:days};
            const expires=days===null?0:now()+days*86_400_000;
            const expiryChanged=db.prepare('SELECT expires FROM devices WHERE id=?').get(deviceId)?.expires!==expires;
            const policyChanged=ownerControl?.deviceId===deviceId && ownerPairingPolicy.expiresAfterDays!==days;
            if(expiryChanged)db.prepare('UPDATE devices SET expires=? WHERE id=?').run(expires,deviceId);
            if(policyChanged){
              db.prepare("UPDATE metadata SET data=? WHERE key='ownerPairingPolicy'").run(JSON.stringify(policy));ownerPairingPolicy=policy;
            }
            if(expiryChanged || policyChanged)append('_workspace','pairingChanged',installation.installationId);
            return pairingStatus(deviceId);
          });
        }
        else if(request.method==='POST'&&(url.pathname==='/v1/host/stop'||url.pathname==='/v1/pairing/disconnect')){
          const body=await readBody(request);
          const deviceId=owner(request,deviceId=>{
            if(ownerPairingPolicy && ownerControl?.deviceId!==deviceId)fail(403,'hostOnly','Use the owning Mac to stop or disconnect the host.');
            if(!ownerPairingPolicy)fail(403,'temporaryPairing','Use the temporary session cleanup procedure.');
            if(stopCommandActive)fail(409,'stopInProgress','A stop request is still waiting for runtime confirmation.');
            if(Object.keys(body).length)fail(400,'invalidStop','This command accepts no additional fields.');
            return deviceId;
          });
          const revoked=url.pathname==='/v1/pairing/disconnect';
          await stopOwner(deviceId, revoked);
          result={runtimeStopped:true,revoked};
        }
        else if (request.method === 'GET' && url.pathname === '/v1/models') {
          const lane = requestedLane(url), runtime = lane.runtime, laneProvider = lane.provider, laneAccount = laneAccounts(lane);
          if (ownerPairingPolicy && runtime?.signIn && !laneAccount.selected()) result = { provider: laneProvider.provider, state: 'unavailable', models: [], detail: 'Sign in on the owning Mac. If the reported account differs, review Advanced account management before checking models.' };
          else if(dispatchStopped || accountsBusy())result={provider:laneProvider.provider,state:'unavailable',models:[],detail:'The runtime is stopped or busy; model discovery is unavailable.'};
          else if(!laneProvider.capabilities.includes('modelCatalog') || !runtime?.models)result={provider:laneProvider.provider,state:'unavailable',models:[],detail:'This runtime does not provide model discovery.'};
          else try{
            runtimeOperations++;
            const catalog=await runtime.models(AbortSignal.any([listenerAbort.signal,AbortSignal.timeout(5_000)]), laneAccount.selected());
            if(catalog.provider!==laneProvider.provider)throw new Error('Model catalog identity mismatch');
            result=catalog;
          }
          catch{result={provider:laneProvider.provider,state:'error',models:[],detail:'Model discovery failed. Your selected model has not changed.'};}
          finally { runtimeOperations--; queueMicrotask(wake); }
        }
        else if (request.method === 'GET' && url.pathname === '/v1/agents') result = { agents: list('agents', decodeAgent) };
        else if(request.method==='POST'&&/^\/v1\/approvals\/[^/]+\/resolve$/.test(url.pathname)){
          const body=await readBody(request),approvalId=id(url.pathname.split('/')[3]);
          result=owner(request,deviceId=>{
            const approval=load('routineApprovals',approvalId,decodeRoutineApproval),run=load('runs',approval.runId,decodeRun);
            if(body.recoveryEpoch!==installation.recoveryEpoch||body.recoveryEpoch!==approval.recoveryEpoch||body.runId!==approval.runId||body.attemptId!==approval.attemptId||body.bindingRevisionId!==approval.bindingRevisionId||body.actionDigest!==approval.actionDigest||JSON.stringify(body.resourceScope)!==JSON.stringify(approval.resourceScope)||(body.decision!=='approve'&&body.decision!=='reject'))fail(409,'approvalConflict','This decision does not match the exact pending action.');
            if(approval.state!=='pending')return {approval,run};
            if(bindingDisabled(run.bindingRevisionId))fail(409,'bindingDisabled','This routine binding was disabled. Cancel the held run and review a new binding.');
            if(Date.parse(approval.expiresAt)<=now())fail(409,'approvalExpired','This approval expired. Cancel the held routine and review a new run.');
            const current=run.routine?.steps.find(step=>step.stepId===approval.stepId);
            if(run.state!=='waitingForApproval'||!run.routine||current?.state!=='waitingForApproval'||current.attemptId!==approval.attemptId||current.approvalId!==approvalId||run.routine.revisionId!==approval.routineRevisionId)fail(409,'approvalConflict','This routine is no longer waiting for this action.');
            const state=body.decision==='approve'?'approved':'rejected';
            const decided:RoutineApproval={...approval,state,decidedByDeviceId:deviceId,decidedAt:new Date(now()).toISOString()};
            const resolved:RoutineExecution={...run.routine,steps:run.routine.steps.map(step=>step.stepId===approval.stepId?{stepId:step.stepId,stepRevisionId:step.stepRevisionId,state,attemptId:approval.attemptId,approvalId}:step)};
            const updated:Run={...run,state:state==='rejected'?'canceled':resolved.steps.some(step=>step.state==='pending')?'queued':'completed',routine:state==='rejected'?stopRoutine(resolved,'canceled','The owner rejected this routine gate.'):resolved,statusDetail:state==='rejected'?'The owner rejected this routine gate.':undefined};
            put('routineApprovals',approvalId,decided);put('runs',run.runId,updated);append(run.conversationId,'approvalResolved',approvalId);return {approval:decided,run:updated};
          });queueMicrotask(wake);
        }
        else if(request.method==='GET'&&url.pathname==='/v1/weeks')result={weeks:weeks().map(viewWeek)};
        else if(request.method==='POST'&&url.pathname==='/v1/weeks'){result=foundNextWeek(request,await readBody(request));status=201;}
        else if(request.method==='POST'&&/^\/v1\/weeks\/[^/]+\/(plan|builds|close)$/.test(url.pathname)){
          const body=await readBody(request),[,,,weekId,action]=url.pathname.split('/'),at=new Date(now()).toISOString();
          result=changeWeek(request,id(weekId),body,week=>action==='plan'?addPlanItem(week,body,agentIds()):action==='builds'?placeBuild(week,body,agentIds(),at):closeWeek(week,body,at));
        }
        else if(request.method==='GET'&&url.pathname==='/v1/schedules')result={schedules:list('schedules',decodeSchedule)};
        else if(request.method==='POST'&&url.pathname==='/v1/schedules'){result=saveSchedule(request,await readBody(request));status=201;}
        else if(request.method==='POST'&&/^\/v1\/schedules\/[^/]+$/.test(url.pathname))result=saveSchedule(request,await readBody(request),id(url.pathname.split('/')[3]));
        else if(request.method==='GET'&&/^\/v1\/schedules\/[^/]+\/preview$/.test(url.pathname))result=schedulePreview(id(url.pathname.split('/')[3]));
        else if(request.method==='GET'&&/^\/v1\/schedules\/[^/]+\/occurrences$/.test(url.pathname)){
          const schedule=load('schedules',id(url.pathname.split('/')[3]),decodeSchedule);
          result={occurrences:list('scheduleOccurrences',record).filter(occurrence=>occurrence.routineId===schedule.definition.routineId)};
        }
        else if(request.method==='POST'&&/^\/v1\/schedules\/[^/]+\/state$/.test(url.pathname)){
          const body=await readBody(request),scheduleId=id(url.pathname.split('/')[3]);
          result=owner(request,deviceId=>{
            const schedule=load('schedules',scheduleId,decodeSchedule);
            if(body.recoveryEpoch!==installation.recoveryEpoch||body.revisionId!==schedule.revisionId||typeof body.enable!=='boolean')fail(409,'scheduleChanged','Review the current schedule before changing its state.');
            if(!body.enable){const paused={...schedule,state:'paused' as const};put('schedules',scheduleId,paused);return paused;}
            if(schedule.state==='enabled')return schedule;
            const preview=schedule.preview;
            if(!preview||preview.revisionId!==schedule.revisionId||preview.digest!==body.previewDigest||Date.parse(preview.expiresAt)<=now()||!preview.occurrences.length||Date.parse(preview.occurrences[0]!.decisionAt)<=now())fail(409,'previewChanged','Preview the next occurrences before enabling this schedule.');
            const enabled:ScheduleRevision={...schedule,state:'enabled',checkedThrough:now(),consent:{deviceId,at:new Date(now()).toISOString(),previewDigest:preview.digest}};
            put('schedules',scheduleId,enabled);return enabled;
          });
        }
        else if(request.method==='GET'&&url.pathname==='/v1/routines')result={routines:list('routines',decodeRoutine)};
        else if(request.method==='POST'&&url.pathname==='/v1/routines'){result=saveRoutine(request,await readBody(request));status=201;}
        else if(request.method==='POST'&&/^\/v1\/routines\/[^/]+$/.test(url.pathname))result=saveRoutine(request,await readBody(request),id(url.pathname.split('/')[3]));
        else if(request.method==='GET'&&/^\/v1\/routines\/[^/]+\/revisions\/[^/]+$/.test(url.pathname)){
          const saved=load('routineRevisions',id(url.pathname.split('/')[5]),decodeRoutine);if(saved.routineId!==id(url.pathname.split('/')[3]))fail(404,'notFound','Routine revision was not found.');result=saved;
        }
        else if(request.method==='POST'&&/^\/v1\/agents\/[^/]+\/bindings\/[^/]+\/disable$/.test(url.pathname)){
          const body=await readBody(request),agentId=id(url.pathname.split('/')[3]),bindingRevisionId=id(url.pathname.split('/')[5]);
          result=owner(request,()=>{
            if(body.recoveryEpoch!==installation.recoveryEpoch)fail(409,'recoveryEpochChanged','Reload this installation before changing authority.');
            const current=load('agents',agentId,decodeAgent),saved=current.binding.bindingRevisionId===bindingRevisionId?current:load('agentBindings',bindingRevisionId,decodeAgent);
            if(saved.agentId!==agentId)fail(409,'bindingChanged','This binding belongs to a different Agent.');
            put('disabledBindings',bindingRevisionId,{agentId,bindingRevisionId,disabledAt:new Date(now()).toISOString()});
            if(current.binding.bindingRevisionId===bindingRevisionId)put('agents',agentId,{...current,binding:{...current.binding,enabled:false}});
            return {binding:{...saved.binding,enabled:false}};
          });queueMicrotask(wake);
        }
        else if(request.method==='POST'&&/^\/v1\/routines\/[^/]+\/run$/.test(url.pathname)){
          const body=await readBody(request),routineId=id(url.pathname.split('/')[3]);
          result=owner(request,()=>acceptRoutine(routineId,body));status=201;queueMicrotask(wake);
        }
        else if (request.method === 'GET' && url.pathname === '/v1/skills') result = { skills: list('skills', decodeSkill) };
        else if (request.method === 'GET' && /^\/v1\/skills\/[^/]+\/revisions\/[^/]+$/.test(url.pathname)) {
          const skill=load('skillRevisions',id(url.pathname.split('/')[5]),decodeSkill);
          if(skill.skillId!==id(url.pathname.split('/')[3]))fail(404,'notFound','Skill revision was not found.');result=skill;
        }
        else if (request.method === 'POST' && url.pathname === '/v1/skill-bundles') {
          const body=await readBody(request),bundle=parseSkillBundle(body.bundle);
          result=owner(request,()=>{
            const skillId=body.skillId===undefined?randomUUID():id(body.skillId);
            const old=body.skillId===undefined?undefined:load('skills',skillId,decodeSkill);
            if(old){
              if(body.expectedRevisionId!==old.revisionId)fail(409,'skillChanged','Reload the skill before importing a revision.');
              put('skillRevisions',old.revisionId,old);
            }
            const skill:InstructionSkill={skillId,revisionId:randomUUID(),name:bundle.name,instructions:skillInstructions(bundle),bundle,review:{state:'reviewRequired',digest:bundle.digest},...(old?{previousRevisionId:old.revisionId}:{})};
            put('skillRevisions',skill.revisionId,skill);put('skills',skillId,skill);return skill;
          });status=201;
        }
        else if (request.method === 'POST' && /^\/v1\/skills\/[^/]+\/review$/.test(url.pathname)) {
          const body=await readBody(request),skillId=id(url.pathname.split('/')[3]);
          result=owner(request,()=>{
            if(typeof body.enable!=='boolean')fail(400,'invalidCommand','Choose explicitly whether to enable this reviewed bundle.');
            const current=load('skills',skillId,decodeSkill);
            if(body.enable&&body.revisionId!==current.revisionId)fail(409,'skillChanged','Review the exact current skill revision before enabling it.');
            const skill=body.revisionId===current.revisionId?current:load('skillRevisions',id(body.revisionId),decodeSkill);
            if(skill.skillId!==skillId||!skill.bundle||body.digest!==skill.bundle.digest)fail(409,'skillChanged','Review the exact skill revision and digest.');
            const reviewed:InstructionSkill={...skill,bundle:skill.bundle,review:{state:body.enable?'enabled':'disabled',digest:skill.bundle.digest}};
            put('skillRevisions',skill.revisionId,reviewed);if(current.revisionId===skill.revisionId)put('skills',skillId,reviewed);return reviewed;
          });
        }
        else if (request.method === 'GET' && url.pathname === '/v1/conversations') result = { conversations: list('conversations', decodeConversation) };
        else if (request.method === 'GET' && /^\/v1\/conversations\/[^/]+\/snapshot$/.test(url.pathname)) {
          const before = url.searchParams.has('before') ? Number(url.searchParams.get('before')) : null;
          if(before !== null && (!Number.isSafeInteger(before) || before < 1)) fail(400,'invalidCursor','Use a positive message sequence.');
          result = snapshot(id(url.pathname.split('/')[3]), before);
        }
        else if (request.method === 'GET' && /^\/v1\/requests\/[^/]+$/.test(url.pathname)) { const receipt = load('requests', id(url.pathname.split('/')[3]), storedReceipt); result = acceptance(receipt); }
        else if (request.method === 'POST' && /^\/v1\/runs\/[^/]+\/close-unknown$/.test(url.pathname)) {
          const body=await readBody(request),runId=id(url.pathname.split('/')[3]);
          result=owner(request,()=>{
            if(body.recoveryEpoch!==installation.recoveryEpoch)fail(409,'recoveryEpochChanged','Reload this installation before changing work.');
            if(body.acknowledgeUncertainty!==true)fail(400,'uncertaintyAcknowledgmentRequired','Acknowledge that earlier effects remain unknown and this turn will not be retried.');
            const run=load('runs',runId,decodeRun);
            if(run.state==='closedUnknown')return {run};
            if(run.state!=='outcomeUnknown')fail(409,'runNotUnknown','Only an unknown outcome can be closed without retry.');
            const resolved:Run={...run,state:'closedUnknown',statusDetail:'Closed by owner acknowledgment; earlier effects remain unknown. This turn was not retried.',...(run.routine?{routine:stopRoutine(run.routine,'canceled','Routine closed without retry; earlier effects remain unknown.')}: {})};
            put('runs',runId,resolved);append(run.conversationId,'runClosed',runId);return {run:resolved};
          });queueMicrotask(wake);
        }
        else if(request.method==='GET'&&url.pathname==='/v1/scheduler')result=scheduleStatus;
        else if(request.method==='GET'&&url.pathname==='/v1/runtime')result=owner(request,()=>runtimeReadiness());
        else if(request.method==='GET'&&url.pathname==='/v1/modules/context')result=contextStatus();
        else if(request.method==='POST'&&url.pathname==='/v1/runtime/restart'){
          const body=await readBody(request);
          if(Object.keys(body).some(key=>key!=='recoveryEpoch'))fail(400,'invalidRestart','Use only the current recovery epoch.');
          if(dispatchStopped){
            const deviceId=owner(request,deviceId=>{
              if(ownerPairingPolicy && ownerControl?.deviceId!==deviceId)fail(403,'hostOnly','Use the owning Mac to restart its runtime.');
              if(body.recoveryEpoch!==installation.recoveryEpoch)fail(409,'recoveryEpochChanged','Reload this installation before resuming local work.');
              return deviceId;
            });
            await resumeOwner(deviceId,request);
            result=runtimeReadiness();
          } else {
          const restart=owner(request,deviceId=>{
            if(ownerPairingPolicy && ownerControl?.deviceId!==deviceId)fail(403,'hostOnly','Use the owning Mac to restart its runtime.');
            if(body.recoveryEpoch!==installation.recoveryEpoch)fail(409,'recoveryEpochChanged','Reload this installation before restarting the runtime.');
            const operation=options.runtime?.restart?.bind(options.runtime);
            if(!operation||!runtimeReadiness().canRestart||dispatching)fail(409,'runtimeRestartUnavailable','Runtime replacement is unavailable, busy or outside its existing budget.');
            runtimeOperations++; return operation;
          });
          try { await restart(AbortSignal.timeout(Math.max(1,Math.min(10_000,(credentialDeadline??now()+10_000)-now())))); } finally { runtimeOperations--; }
          result=runtimeReadiness();queueMicrotask(wake);
          }
        }
        else if (request.method === 'POST' && /^\/v1\/conversations\/[^/]+\/memory$/.test(url.pathname)) {
          const body=await readBody(request),conversationId=id(url.pathname.split('/')[3]);
          result=owner(request,()=>{
            if(body.recoveryEpoch!==installation.recoveryEpoch)fail(409,'recoveryEpochChanged','Reload this installation before editing memory.');
            if(Object.keys(body).some(key=>!['recoveryEpoch','expectedRevisionId','text'].includes(key)))fail(400,'invalidMemory','Use only the owner memory fields.');
            load('conversations',conversationId,decodeConversation);
            if((conversationMemory(conversationId)?.revisionId??null)!==body.expectedRevisionId)fail(409,'memoryChanged','Reload conversation memory before saving.');
            const memory:ConversationMemory={conversationId,revisionId:randomUUID(),text:memoryText(body.text)};
            put('conversationMemory',conversationId,memory);append(conversationId,'memoryChanged',memory.revisionId);return memory;
          });
        }
        else if (request.method === 'POST' && /^\/v1\/requests\/[^/]+\/cancel$/.test(url.pathname)) {
          const body=await readBody(request),requestId=id(url.pathname.split('/')[3]);
          const cancellation:RequestCancellation=owner(request,()=>{
            if(body.recoveryEpoch!==installation.recoveryEpoch)fail(409,'recoveryEpochChanged','Reload this installation before changing work.');
            const scope={requestId,recoveryEpoch:installation.recoveryEpoch,conversationId:id(body.conversationId),bindingRevisionId:id(body.bindingRevisionId)};
            const prior=db.prepare("SELECT data FROM entities WHERE kind='requestCancellations' AND id=?").get(requestId);
            if(prior){const stored=record(json(prior));if(stored.conversationId!==scope.conversationId||stored.bindingRevisionId!==scope.bindingRevisionId||stored.recoveryEpoch!==scope.recoveryEpoch)fail(409,'requestConflict','The cancellation belongs to a different request scope.');}
            const accepted=db.prepare("SELECT data FROM entities WHERE kind='requests' AND id=?").get(requestId);
            if(accepted){
              const receipt=storedReceipt(json(accepted));
              if(receipt.conversationId!==scope.conversationId||receipt.bindingRevisionId!==scope.bindingRevisionId)fail(409,'requestConflict','The accepted request belongs to a different scope.');
              const run=cancelRun(receipt.runId);put('requestCancellations',requestId,scope);
              return {...scope,status:'accepted',receipt,run,message:load('messages',receipt.messageId,decodeMessage),snapshot:snapshot(scope.conversationId)};
            }
            load('conversations',scope.conversationId,decodeConversation);
            put('requestCancellations',requestId,scope);return {...scope,status:'notAccepted'};
          });
          result=cancellation;
          if(cancellation.status==='accepted'&&activeRunId===cancellation.run.runId)active?.abort();
          queueMicrotask(wake);
        }
        else if (request.method === 'POST' && /^\/v1\/runs\/[^/]+\/cancel$/.test(url.pathname)) {
          const body=await readBody(request),runId=id(url.pathname.split('/')[3]);
          result=owner(request,()=>{
            if(body.recoveryEpoch!==installation.recoveryEpoch)fail(409,'recoveryEpochChanged','Reload this installation before changing work.');
            return {run:cancelRun(runId)};
          });
          if(activeRunId===runId)active?.abort();
          queueMicrotask(wake);
        }
        else if (request.method === 'POST' && /^\/v1\/conversations\/[^/]+\/messages$/.test(url.pathname)) { result = accept(request, id(url.pathname.split('/')[3]), await readBody(request)); status = 201; queueMicrotask(wake); }
        else if (request.method === 'POST' && /^\/v1\/conversations\/[^/]+\/read$/.test(url.pathname)) {
          const body = await readBody(request), conversationId = id(url.pathname.split('/')[3]);
          result = owner(request, () => {
            load('conversations', conversationId, decodeConversation);
            const row = typeof body.messageId === 'string' ? db.prepare("SELECT data FROM entities WHERE kind='messages' AND id=?").get(body.messageId) : undefined;
            const message = row ? decodeMessage(json(row)) : undefined;
            if (body.recoveryEpoch !== installation.recoveryEpoch || typeof body.cursor !== 'number' || !Number.isSafeInteger(body.cursor) || body.cursor > cursor() || !message || message.conversationId !== conversationId || message.sequence > body.cursor) fail(409, 'readConflict', 'Read acknowledgement does not match this conversation.');
            const prior = db.prepare("SELECT data FROM entities WHERE kind='reads' AND id=?").get(conversationId);
            const previous = prior ? Number(record(json(prior)).sequence) : 0;
            const sequence = Math.max(previous, message.sequence);
            if (sequence > previous) {
              put('reads', conversationId, { sequence });
              append(conversationId, 'readChanged', message.messageId);
            }
            return { lastReadMessageSequence: sequence };
          });
        }
        else if ((request.method === 'GET' || request.method === 'POST') && /^\/v1\/agents\/[^/]+\/portrait(\/remove)?$/.test(url.pathname)) {
          // Spec 065: one portrait draw at a time, on the Agent's own runtime and signed-in account.
          const agentId = id(url.pathname.split('/')[3]);
          if (url.pathname.endsWith('/remove') && request.method !== 'POST') fail(404, 'notFound', 'Route was not found.');
          if (request.method === 'GET') {
            authenticate(request);
            const agent = load('agents', agentId, decodeAgent), row = db.prepare("SELECT data FROM entities WHERE kind='portraits' AND id=?").get(agentId);
            if (!agent.portrait || !row) fail(404, 'noPortrait', 'This Agent has no portrait.');
            const image = record(json(row));
            if (image.revision !== agent.portrait.revision || typeof image.png !== 'string') fail(404, 'noPortrait', 'This Agent has no portrait.');
            result = { agentId, revision: agent.portrait.revision, png: image.png } satisfies PortraitImage;
          } else if (url.pathname.endsWith('/remove')) {
            const body = await readBody(request);
            if (Object.keys(body).length) fail(400, 'invalidPortrait', 'This command accepts no fields.');
            result = owner(request, () => {
              const agent = load('agents', agentId, decodeAgent);
              if (agent.portraitDraw?.state === 'drawing') fail(409, 'portraitDrawing', 'Wait for the portrait to finish drawing.');
              const { portrait: _portrait, portraitDraw: _draw, ...rest } = agent;
              db.prepare("DELETE FROM entities WHERE kind='portraits' AND id=?").run(agentId);
              put('agents', agentId, rest); return rest;
            });
          } else {
            const body = await readBody(request);
            if (Object.keys(body).some(key => key !== 'note') || (body.note !== undefined && (typeof body.note !== 'string' || !body.note.isWellFormed() || Buffer.byteLength(body.note) > 500))) fail(400, 'invalidPortrait', 'Use an optional note of at most 500 bytes.');
            const note = typeof body.note === 'string' && body.note.trim() ? body.note.trim() : undefined;
            const started = owner(request, () => {
              const agent = load('agents', agentId, decodeAgent), lane = laneFor(agent.binding), account = lane && laneAccounts(lane).selected();
              if (!lane?.runtime?.portrait) fail(409, 'portraitUnavailable', 'Portraits need a ChatGPT Agent on this Mac. Nothing was drawn.');
              if (agent.portraitDraw?.state === 'drawing' || portraitDrawing) fail(409, 'portraitDrawing', 'A portrait is already being drawn.');
              if (dispatchStopped || accountsBusy() || dispatching || runtimeOperations > 0) fail(409, 'runtimeBusy', 'Fort is busy with another reply. Try again when it finishes.');
              if (!account) fail(409, 'signInRequired', 'Sign in to ChatGPT on this Mac first. Nothing was drawn.');
              const drawing: Agent = { ...agent, portraitDraw: { state: 'drawing', detail: 'Drawing on your ChatGPT plan.', updatedAt: new Date(now()).toISOString() } };
              put('agents', agentId, drawing);
              return { agent: drawing, runtime: lane.runtime, account };
            });
            portraitDrawing = true; runtimeOperations++;
            void (async () => {
              let outcome: { kind: 'drawn'; png: Buffer; width: number } | { kind: 'failed'; message: string };
              try {
                const drawn = await started.runtime.portrait!({ account: started.account, model: started.agent.binding.selectedModel, subject: { name: started.agent.name, instructions: started.agent.instructions ?? '', ...(note ? { note } : {}) } }, listenerAbort.signal);
                outcome = drawn.kind === 'drawn' ? { kind: 'drawn', png: drawn.png, width: drawn.width } : { kind: 'failed', message: drawn.message };
              } catch { outcome = { kind: 'failed', message: 'The portrait run failed. Nothing was saved.' }; }
              finally { portraitDrawing = false; runtimeOperations--; queueMicrotask(wake); }
              if (closed) return;
              try {
                transaction(() => {
                  const current = load('agents', agentId, decodeAgent), { portraitDraw: _draw, ...rest } = current;
                  if (outcome.kind === 'drawn') {
                    const portrait: AgentPortrait = { revision: randomUUID(), createdAt: new Date(now()).toISOString(), size: outcome.width };
                    db.prepare("INSERT INTO entities(kind,id,data) VALUES ('portraits',?,?) ON CONFLICT(kind,id) DO UPDATE SET data=excluded.data").run(agentId, JSON.stringify({ revision: portrait.revision, png: outcome.png.toString('base64') }));
                    put('agents', agentId, { ...rest, portrait });
                  } else put('agents', agentId, { ...rest, portraitDraw: { state: 'failed', detail: outcome.message, updatedAt: new Date(now()).toISOString() } });
                });
              } catch {}
            })();
            result = started.agent; status = 202;
          }
        }
        else if (request.method === 'POST' && /^\/v1\/agents\/[^/]+$/.test(url.pathname)) {
          const body = await readBody(request), agentId = id(url.pathname.split('/')[3]);
          result = owner(request, () => {
            const old = load('agents', agentId, decodeAgent);
            if (body.expectedBindingRevisionId !== old.binding.bindingRevisionId) fail(409, 'bindingChanged', 'Reload the Agent before saving.');
            const agent: Agent = { ...old, ...avatar(body.avatar), name: text(body.name, 'name', 128), instructions: text(body.instructions, 'instructions'), binding: { ...old.binding, bindingRevisionId: randomUUID(), selectedModel: text(body.model, 'exact model', 128),enabled:true } };
            put('agentBindings',old.binding.bindingRevisionId,old);put('agentBindings',agent.binding.bindingRevisionId,agent);put('agents', agentId, agent);
            const home = load('conversations', old.homeConversationId, decodeConversation); put('conversations', home.conversationId, { ...home, name: agent.name }); return agent;
          });
        } else if (request.method === 'POST' && /^\/v1\/skills\/[^/]+$/.test(url.pathname)) {
          const body = await readBody(request), skillId = id(url.pathname.split('/')[3]);
          result = saveInstructions(request,body,skillId);
        }
        else if (request.method === 'POST' && url.pathname === '/v1/agents') {
          const body = await readBody(request);
          result = owner(request, () => {
            // The Agent's provider is fixed for good when it is created (Spec 064). The default is the primary runtime.
            const chosen = body.provider === undefined ? lanes[0]!.provider : lanes.find(lane => lane.provider.provider === body.provider)?.provider;
            if (!chosen) fail(400, 'runtimeUnavailable', 'That provider is not available on this host.');
            const agent: Agent = { ...avatar(body.avatar),agentId: randomUUID(), name: text(body.name, 'name', 128), instructions: text(body.instructions, 'instructions'), homeConversationId: randomUUID(), readiness: 'requiresLiveReview',
              binding: { bindingRevisionId: randomUUID(), provider: chosen.provider, runtimeVersion: chosen.runtimeVersion, selectedModel: text(body.model, 'exact model', 128), hostId: chosen.hostId, enabled: true } };
            put('agents', agent.agentId, agent);put('agentBindings',agent.binding.bindingRevisionId,agent);
            const home: Conversation = { conversationId: agent.homeConversationId, name: agent.name, kind: 'home', participantAgentIds: [agent.agentId] };
            put('conversations', home.conversationId, home); return agent;
          }); status = 201;
        } else if (request.method === 'POST' && url.pathname === '/v1/skills') {
          const body = await readBody(request);
          result = saveInstructions(request,body); status = 201;
        } else if (request.method === 'POST' && url.pathname === '/v1/conversations') {
          const body = await readBody(request);
          result = owner(request, () => {
            if (!Array.isArray(body.participantAgentIds) || body.participantAgentIds.length < 2 || body.participantAgentIds.length > 8) fail(400, 'invalidParticipants', 'Select two to eight Agents.');
            const participants = body.participantAgentIds.map(id);
            if (new Set(participants).size !== participants.length) fail(400, 'invalidParticipants', 'Select each Agent once.');
            for (const participant of participants) load('agents', participant, decodeAgent);
            const conversation: Conversation = { conversationId: randomUUID(), name: text(body.name, 'name', 128), kind: 'shared', participantAgentIds: participants };
            put('conversations', conversation.conversationId, conversation); return conversation;
          }); status = 201;
        } else fail(404, 'notFound', 'Route was not found.');
      }
      if (request.method === 'POST' && url.pathname.startsWith('/v1/schedules')) armScheduleClock();
    } catch (error) {
      status = error instanceof APIError || error instanceof CompanionPairingError || error instanceof AccountSignInError ? error.status : error instanceof InvalidSkillBundle || error instanceof InvalidRoutine || error instanceof InvalidWeek ? 400 : error instanceof WeekConflict ? 409 : 500;
      result = { error: { code: error instanceof APIError || error instanceof CompanionPairingError || error instanceof AccountSignInError ? error.code : error instanceof InvalidSkillBundle ? 'invalidSkillBundle' : error instanceof InvalidRoutine ? 'invalidRoutine' : error instanceof InvalidWeek ? 'invalidWeek' : error instanceof WeekConflict ? 'weekConflict' : 'internalError', message: error instanceof APIError || error instanceof CompanionPairingError || error instanceof AccountSignInError || error instanceof InvalidSkillBundle || error instanceof InvalidRoutine || error instanceof InvalidWeek || error instanceof WeekConflict ? error.message : 'Fort could not complete the request.' } };
    }
    response.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' }); response.end(JSON.stringify(result));
  });
  let baseURL = ''; let endpoint: ServiceEndpoint | undefined; let closed = false; let starting = false; let dispatchStopped = ownerControl !== undefined && ownerControl.intent !== 'running'; let stopCommandActive = false; let closing: Promise<void> | undefined; let scheduleTimer:ReturnType<typeof setTimeout>|undefined;
  let privateSocket: PrivateSocketLease | undefined;
  server.on('connection',socket=>{
    let resolveClosed!:()=>void;
    acceptedConnections.set(socket,new Promise<void>(resolve=>{resolveClosed=resolve;}));
    socket.once('close',()=>{acceptedConnections.delete(socket);resolveClosed();});
    if(closed)socket.destroy();
  });
  const closeHTTP=async()=>{
    listenerAbort.abort();
    const stopped=new Promise<void>((resolve,reject)=>server.close(error=>{
      if(error&&(!('code' in error)||error.code!=='ERR_SERVER_NOT_RUNNING'))reject(error);else resolve();
    }));
    for(const socket of acceptedConnections.keys())socket.destroy();
    await stopped;
    while(acceptedConnections.size){
      for(const socket of acceptedConnections.keys())socket.destroy();
      await Promise.all([...acceptedConnections.values()]);
    }
    privateSocket?.release();
  };
  type RuntimeStop = { state: 'pending' | 'confirmed' | 'unconfirmed'; completion: Promise<void> };
  let runtimeStop:RuntimeStop|undefined;
  let ownerResuming:Promise<unknown>|undefined;
  const stopRuntime=()=>{
    if(runtimeStop && runtimeStop.state!=='unconfirmed')return runtimeStop.completion;
    const progress:RuntimeStop={state:'pending',completion:Promise.resolve().then(async()=>{
      const outcomes=await Promise.allSettled([...lanes.map(lane=>Promise.resolve().then(()=>lane.runtime?.dispose())),dispatching,...laneAccountList.map(item=>item.stop())]);
      if(outcomes.some(outcome=>outcome.status==='rejected'))throw new Error('Runtime shutdown unconfirmed');
      progress.state='confirmed';
    }).catch(error=>{progress.state='unconfirmed';throw error;})};
    runtimeStop=progress;
    return progress.completion;
  };
  const resumeOwnerRuntime = async <T>(commit: () => T): Promise<T> => {
    const restart=runtimeStop===undefined?undefined:options.runtime?.restart?.bind(options.runtime);
    stopCommandActive=true;
    const operation=Promise.resolve().then(async()=>{
      const signal=AbortSignal.any([listenerAbort.signal,AbortSignal.timeout(10_000)]);
      try {
        signal.throwIfAborted();
        if(restart){runtimeStop=undefined;await restart(signal);}
        signal.throwIfAborted();
        if(closed || options.runtime?.readiness && options.runtime.readiness().state!=='ready')throw new Error('Runtime did not become ready');
        const result=transaction(commit);
        dispatchStopped=false;runScheduleClock(true);queueMicrotask(wake);
        return result;
      } catch {
        await stopRuntime().catch(()=>{});
        fail(503,'resumeUnconfirmed','Local work remains paused. Runtime resume or cleanup was not confirmed.');
      } finally {stopCommandActive=false;}
    });
    ownerResuming=operation;
    try {return await operation;}
    finally {if(ownerResuming===operation)ownerResuming=undefined;}
  };
  const resumeOwner = async (deviceId: string, request: IncomingMessage) => {
    if(ownerControl?.deviceId!==deviceId || !canResumeOwner() || readOwnerSetup()?.state==='canceling')fail(409,'runtimeRestartUnavailable','This exact owner pairing cannot resume until shutdown and recovery are confirmed.');
    await resumeOwnerRuntime(()=>{
      if(authenticate(request)!==deviceId)throw new Error('Owner pairing changed');
      saveOwnerControl({deviceId,intent:'running'});
    });
  };
  const stopOwner = async (deviceId: string, disconnect: boolean) => {
    if (closed) fail(503, 'closed', 'This session has stopped.');
    if (!ownerPairingPolicy) fail(403, 'temporaryPairing', 'Use the temporary session cleanup procedure.');
    if (starting) fail(409, 'startInProgress', 'The local host is starting. Retry recovery after startup completes.');
    if (stopCommandActive) fail(409, 'stopInProgress', 'A stop request is still waiting for runtime confirmation.');
    const storedDevice = db.prepare('SELECT id FROM devices WHERE id=?').get(deviceId);
    if (ownerControl ? ownerControl.deviceId !== deviceId : !storedDevice) fail(409, 'pairingMismatch', 'Recovery requires the latest exact owner pairing.');
    if (!disconnect && ownerControl?.intent === 'disconnect') fail(409, 'disconnectPending', 'Complete the pending disconnect before starting this host.');
    transaction(() => saveOwnerControl({ deviceId, intent: disconnect ? 'disconnect' : 'stop' }));
    stopCommandActive = true; dispatchStopped = true; clearTimeout(scheduleTimer); scheduleStatus = { state: 'stopped' }; active?.abort();
    try {
      await stopRuntime();
      if (closed) fail(503, 'closed', 'This session has stopped. Retry recovery with the local host.');
      if (disconnect) transaction(() => { db.prepare('DELETE FROM devices').run(); db.prepare('DELETE FROM enrollment').run(); companions.clear(); });
    } catch {
      fail(503, 'stopUnconfirmed', 'Runtime shutdown was not confirmed. Pairing is retained for recovery.');
    } finally { stopCommandActive = false; }
  };
  const cancelOwnerSetup = async (request: OwnerSetupIdentity): Promise<OwnerSetupCancellation> => {
    if (closed || starting || stopCommandActive) fail(409, 'hostUnavailable', 'Owner setup cancellation requires an available local host.');
    if (!ownerPairingPolicy) fail(403, 'temporaryPairing', 'Owner setup requires an explicitly configured owner-local installation.');
    const value = record(request);
    if (value.installationId !== installation.installationId) fail(409, 'installationMismatch', 'Setup requires this exact installation.');
    const identity = { installationId: installation.installationId, setupId: id(value.setupId), deviceId: id(value.deviceId) };
    const canceledSetup = transaction(() => {
      const setup = readOwnerSetup();
      const paired = db.prepare('SELECT 1 FROM devices WHERE id=?').get(identity.deviceId);
      if (!setup || setup.setupId !== identity.setupId) {
        if (paired) fail(409, 'pairingMismatch', 'Cancellation requires the exact owner setup and device.');
        return;
      }
      if (setup.state === 'pending' || (setup.state === 'canceled' && setup.outcome === 'notPaired')) {
        if (paired || (setup.state === 'canceled' && setup.deviceId !== identity.deviceId)) fail(409, 'pairingMismatch', 'Cancellation requires the exact owner setup and device.');
        if (setup.state === 'pending') saveOwnerSetup({ ...setup, state: 'canceled', deviceId: identity.deviceId, outcome: 'notPaired' });
        return;
      }
      if (setup.deviceId !== identity.deviceId || ownerControl?.deviceId !== identity.deviceId || (setup.state === 'canceled' && paired)) fail(409, 'pairingMismatch', 'Cancellation requires the exact owner setup and device.');
      saveOwnerSetup({ version: 1, setupId: setup.setupId, expires: setup.expires, state: 'canceling', deviceId: identity.deviceId });
      saveOwnerControl({ deviceId: identity.deviceId, intent: 'disconnect' });
      return setup;
    });
    if (!canceledSetup) return { ...identity, outcome: 'notPaired' };
    await stopOwner(identity.deviceId, true);
    transaction(() => saveOwnerSetup({ version: 1, setupId: canceledSetup.setupId, expires: canceledSetup.expires, state: 'canceled', deviceId: identity.deviceId, outcome: 'revoked' }));
    return { ...identity, outcome: 'revoked', runtimeStopped: true };
  };
  const localOwnerAccess = (identity: {installationId:string;deviceId:string}): 'active' | 'inactive' | 'foreign' => {
    if(closed || !ownerPairingPolicy || identity.installationId!==installation.installationId || identity.deviceId!==ownerControl?.deviceId)return 'foreign';
    const device=db.prepare('SELECT expires FROM devices WHERE id=?').get(identity.deviceId);
    return device && (device.expires===0 || Number(device.expires)>now()) && ownerControl.intent==='running' && !dispatchStopped && !stopCommandActive && !starting ? 'active' : 'inactive';
  };
  let ownerAccessTimer:ReturnType<typeof setTimeout>|undefined,lastOwnerAccess='';
  ownerAccessWake=()=>{
    if(!ownerAccessChanged)return;
    clearTimeout(ownerAccessTimer);ownerAccessTimer=undefined;
    const device=closed || !ownerControl ? undefined : db.prepare('SELECT expires FROM devices WHERE id=?').get(ownerControl.deviceId);
    const access=ownerControl?localOwnerAccess({installationId:installation.installationId,deviceId:ownerControl.deviceId}):'foreign';
    const state=JSON.stringify([access,ownerControl?.deviceId,device?.expires]);
    if(access==='active' && device && Number(device.expires)>now()) {
      ownerAccessTimer=setTimeout(()=>ownerAccessWake(),Math.min(2_147_483_647,Number(device.expires)-now()));ownerAccessTimer.unref();
    }
    if(state!==lastOwnerAccess){lastOwnerAccess=state;ownerAccessChanged();}
  };
  // The owning Mac's device ID when this bearer token is its current credential; for host-only routes outside this service.
  const ownerDevice = (token: string): string | undefined => {
    if (closed || !ownerControl || !/^[A-Za-z0-9_-]{43}$/.test(token)) return undefined;
    const device = db.prepare('SELECT id,expires FROM devices WHERE id=? AND hash=?').get(ownerControl.deviceId, digest(token));
    return device && (device.expires === 0 || Number(device.expires) > now()) ? ownerControl.deviceId : undefined;
  };
  const api = { installation, issueEnrollment, beginOwnerSetup, completeOwnerSetup, cancelOwnerSetup, dispatchNext, tickSchedules, localOwnerAccess, ownerDevice,
    async recoverOwnerPairing(identity: { installationId: string; deviceId: string }) {
      if (identity.installationId !== installation.installationId) throw new OwnerRecoveryIdentityRejected('installation');
      const deviceId = id(identity.deviceId);
      try { await stopOwner(deviceId, true); }
      catch (error) {
        if (error instanceof APIError && error.code === 'pairingMismatch') throw new OwnerRecoveryIdentityRejected('device');
        throw error;
      }
      return { installationId: installation.installationId, deviceId, runtimeStopped: true, revoked: true };
    },
    inspectSession() { return { installation,runs:list('runs',decodeRun),receipts:list('requests',storedReceipt),deviceIDs:db.prepare('SELECT id FROM devices').all().map(row=>id(row.id)) }; },
    get baseURL() { if (endpoint?.kind === 'privateSocket') throw new Error('Private socket transport has no TCP URL'); return baseURL; },
    get endpoint() { return endpoint; },
    async start({ listener = { kind: 'loopback' }, signal, resumeOwnerHost = false }: { listener?: ServiceListener; signal?: AbortSignal; resumeOwnerHost?: boolean } = {}) {
    if(closed)throw new Error('Service is closed');
    if (server.listening || starting) throw new Error('Service is already listening or starting');
    if (resumeOwnerHost) {
      if (!ownerPairingPolicy) throw new Error('Explicit resume requires an owner-local installation');
      if (runtimeStop || stopCommandActive) throw new Error('Open a fresh core before resuming the owner host');
      if (readOwnerSetup()?.state === 'canceling') throw new Error('Complete the pending setup cancellation before starting this host');
      if (ownerControl?.intent === 'disconnect' && db.prepare('SELECT id FROM devices WHERE id=?').get(ownerControl.deviceId)) throw new Error('Complete the pending disconnect before starting this host');
    }
    starting = true;
    try {
    listener = { ...listener };
    signal=signal?AbortSignal.any([signal,listenerAbort.signal]):listenerAbort.signal;
    signal.throwIfAborted();
    if (listener.kind === 'privateSocket') {
      const directory = lstatSync(options.dataDirectory);
      if (!/^[A-Za-z0-9][A-Za-z0-9_-]{0,47}\.sock$/.test(listener.name) || realpathSync(options.dataDirectory) !== realpathSync(process.cwd())
        || !process.getuid || directory.uid !== process.getuid() || (directory.mode & 0o077) !== 0) throw new Error('Private socket requires a short filename and an owner-only ledger working directory');
      const lease = new PrivateSocketLease(db, options.dataDirectory, listener.name);
      await lease.prepare(signal);
      privateSocket = lease;
    } else if (listener.kind !== 'loopback') throw new Error('Unsupported service transport');
    signal.throwIfAborted();
    await new Promise<void>((resolve, reject) => {
      const cleanup=()=>{server.off('error',failed);server.off('close',canceled);};
      const failed=(error:unknown)=>{cleanup();reject(error);};
      const canceled=()=>failed(new Error('Listener startup canceled'));
      server.once('error',failed);server.once('close',canceled);
      try {
        const binding = listener.kind === 'privateSocket' ? { path: listener.name, signal } : { port: listener.port ?? 0, host: '127.0.0.1', signal };
        server.listen(binding, () => {
          try {
            if (listener.kind === 'privateSocket') {
              chmodSync(join(options.dataDirectory, listener.name), 0o600);
              privateSocket?.bound();
            }
          }
          catch (error) { failed(error); server.close(); return; }
          cleanup(); resolve();
        });
      } catch(error) { failed(error); }
    });
    signal?.throwIfAborted();
    const address = server.address();
    if (listener.kind === 'privateSocket') {
      if (address !== listener.name) throw new Error('Private listener identity changed');
      endpoint = Object.freeze({ kind: 'privateSocket', directory: realpathSync(options.dataDirectory), name: listener.name });
    } else {
      if (!address || typeof address === 'string') throw new Error('Missing listener');
      baseURL = `http://127.0.0.1:${address.port}`; endpoint = Object.freeze({ kind: 'loopback', url: baseURL });
    }
    const ownerDeviceId = ownerControl?.deviceId;
    if (resumeOwnerHost && ownerDeviceId !== undefined) {
      transaction(() => saveOwnerControl({ deviceId: ownerDeviceId, intent: 'running' }));
      dispatchStopped = false;
    }
    runScheduleClock(true);queueMicrotask(wake);
    } finally { starting = false; queueOwnerAccess(); }
  }, close({ preserveSessionCredential=false }: { preserveSessionCredential?: boolean } = {}): Promise<void> {
    if (closing) return closing;
    closed = true; clearTimeout(scheduleTimer); active?.abort(); ownerAccessWake();
    if (credentialDeadline!==undefined && !preserveSessionCredential) transaction(()=>{
      db.prepare('DELETE FROM enrollment').run(); db.prepare('DELETE FROM devices').run();
      db.prepare("INSERT OR REPLACE INTO metadata VALUES ('credentialClosed','true')").run();
    });
    closing = (async () => {
      const outcomes = await Promise.allSettled([closeHTTP(),(async()=>{await ownerResuming?.catch(()=>{});await stopRuntime();})()]);
      db.close();
      if (outcomes.some(result => result.status === 'rejected')) throw new Error('Runtime or HTTP cleanup was not confirmed; restart is blocked.');
    })();
    return closing;
  } };
  return api;
}
