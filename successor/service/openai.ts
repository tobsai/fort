import { DatabaseSync } from 'node:sqlite';
import { createServer, type IncomingMessage } from 'node:http';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { mkdirSync, lstatSync, chmodSync } from 'node:fs';
import { isAbsolute, join } from 'node:path';
import type { Acceptance, Agent, Conversation, InstructionSkill, Installation, Message, Run, Snapshot, TurnConfiguration, RoutineRevision, RoutineExecution, RoutineApproval, ScheduleRevision, SchedulePreview, ScheduleOccurrence, RequestCancellation } from '../contracts/v1.ts';
import type { ChatInput, ChatRuntime } from '../runner/chat-runtime.ts';
import { parseSchedule, calendarBetween, previewCalendar } from './schedule.ts';
import { InvalidRoutine, parseRoutineDefinition, parseRoutineInputs, parseRoutineExecution, startRoutine, stopRoutine, taskInput, routineDigest, outputDigest } from './routine.ts';
import { InvalidSkillBundle, parseSkillBundle, skillInstructions } from '../contracts/skill-bundle.ts';

type Entity = 'requestCancellations' | 'schedules' | 'scheduleRevisions' | 'scheduleOccurrences' | 'agentBindings' | 'disabledBindings' | 'routineApprovals' | 'routines' | 'routineRevisions' | 'agents' | 'skillRevisions' | 'skills' | 'conversations' | 'messages' | 'runs' | 'requests' | 'contexts' | 'reads';
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
function decodeAgent(value: unknown): Agent {
  const v = record(value), b = record(v.binding);
  if (b.provider !== 'codex' || typeof b.enabled !== 'boolean' || v.readiness !== 'requiresLiveReview') throw new Error('Invalid stored Agent');
  return { agentId: id(v.agentId), name: text(v.name, 'name'), homeConversationId: id(v.homeConversationId), instructions: text(v.instructions, 'instructions'), readiness: v.readiness,
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
function decodeConfiguration(value: unknown): TurnConfiguration {
  const v = record(value);
  if (!Array.isArray(v.skills)) throw new Error('Invalid stored skills');
  return { agentId: id(v.agentId), agentName: text(v.agentName, 'name'), bindingRevisionId: id(v.bindingRevisionId), selectedModel: text(v.selectedModel, 'model'), instructions: text(v.instructions, 'instructions'), skills: v.skills.map(decodeSkill) };
}
function decodeMessage(value: unknown): Message {
  const v = record(value);
  if ((v.role !== 'user' && v.role !== 'assistant') || typeof v.sequence !== 'number' || !Number.isSafeInteger(v.sequence)) throw new Error('Invalid stored Message');
  return { messageId: id(v.messageId), conversationId: id(v.conversationId), requestId: id(v.requestId), role: v.role, text: messageText(v.text), sequence: v.sequence, createdAt: text(v.createdAt, 'date'),
    ...(v.agentId === undefined ? {} : { agentId: id(v.agentId), agentName: text(v.agentName, 'name') }),
    ...(v.recipientAgentId === undefined ? {} : { recipientAgentId: id(v.recipientAgentId), recipientAgentName: text(v.recipientAgentName, 'name') }) };
}
function decodeRun(value: unknown): Run {
  const v = record(value);
  if (v.state !== 'queued' && v.state !== 'running' && v.state !== 'waitingForApproval' && v.state !== 'cancelRequested' && v.state !== 'completed' && v.state !== 'failed' && v.state !== 'outcomeUnknown' && v.state !== 'canceled' && v.state !== 'closedUnknown') throw new Error('Invalid stored Run');
  return { runId: id(v.runId), requestId: id(v.requestId), conversationId: id(v.conversationId), bindingRevisionId: id(v.bindingRevisionId), state: v.state, selectedModel: text(v.selectedModel, 'model'), effectiveModel: null, modelEvidence: 'unreported', attemptId: v.attemptId === null ? null : id(v.attemptId), configuration: decodeConfiguration(v.configuration), ...(v.routine===undefined?{}:{routine:parseRoutineExecution(v.routine)}), ...(v.statusDetail === undefined ? {} : { statusDetail: text(v.statusDetail, 'status') }) };
}
function decodeRoutineApproval(value:unknown):RoutineApproval {
  const v=record(value);
  if(v.kind!=='routineGate'||JSON.stringify(v.resourceScope)!==JSON.stringify(['routine:continue'])||(v.state!=='pending'&&v.state!=='approved'&&v.state!=='rejected'&&v.state!=='expired'&&v.state!=='canceled'))throw new Error('Invalid routine approval');
  return {approvalId:id(v.approvalId),installationId:id(v.installationId),recoveryEpoch:id(v.recoveryEpoch),runId:id(v.runId),attemptId:id(v.attemptId),bindingRevisionId:id(v.bindingRevisionId),actionDigest:text(v.actionDigest,'digest',64),expiresAt:text(v.expiresAt,'expiry'),state:v.state,decidedByDeviceId:v.decidedByDeviceId===null?null:id(v.decidedByDeviceId),decidedAt:v.decidedAt===null?null:text(v.decidedAt,'decision time'),kind:'routineGate',routineRevisionId:id(v.routineRevisionId),stepId:id(v.stepId),resourceScope:['routine:continue'],actionDescription:text(v.actionDescription,'gate description')};
}
function decodeRoutine(value:unknown):RoutineRevision {
  const v=record(value),definition=parseRoutineDefinition(v.definition),agentSnapshot=decodeAgent(v.agentSnapshot);
  if(!Array.isArray(v.skills))throw new Error('Invalid routine skill snapshot');
  const skills=v.skills.map(decodeSkill),digest=routineDigest({definition,agentSnapshot,skills});
  if(v.digest!==digest)throw new Error('Routine revision digest changed');
  return {routineId:id(v.routineId),revisionId:id(v.revisionId),digest,definition,agentSnapshot,skills};
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
export function createOpenAIService(options: { dataDirectory: string; runtime?: ChatRuntime; autoRun?: boolean; credentialDeadline?: number; now?: () => number; admitSubmission?: (submission:SubmissionAdmission,priorRuns:readonly Run[]) => boolean }) {
  const now=options.now ?? Date.now;
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
    CREATE TABLE IF NOT EXISTS events (sequence INTEGER PRIMARY KEY AUTOINCREMENT, data TEXT NOT NULL);`);
  const savedDeadline=db.prepare("SELECT data FROM metadata WHERE key='credentialDeadline'").get();
  const credentialDeadline=savedDeadline ? Number(json(savedDeadline)) : options.credentialDeadline;
  if (savedDeadline && options.credentialDeadline!==undefined && credentialDeadline!==options.credentialDeadline) { db.close(); throw new Error('Credential deadline cannot be changed'); }
  if (!savedDeadline && credentialDeadline!==undefined) {
    if (credentialDeadline<=now() || db.prepare("SELECT 1 FROM metadata WHERE key='installation'").get()) { db.close(); throw new Error('Bounded enrollment requires a fresh installation'); }
    db.prepare('INSERT INTO metadata VALUES (?,?)').run('credentialDeadline',JSON.stringify(credentialDeadline));
  }
  const transaction = <T>(operation: () => T): T => {
    db.exec('BEGIN IMMEDIATE');
    try { const result = operation(); db.exec('COMMIT'); return result; }
    catch (error) { db.exec('ROLLBACK'); throw error; }
  };
  let installation: Installation;
  const saved = db.prepare("SELECT data FROM metadata WHERE key='installation'").get();
  if (saved) {
    const v = record(json(saved));
    if (v.contractVersion !== '1.0' || v.mode !== 'openai' || v.serviceName !== 'Fort Prototype') { db.close(); throw new Error('Wrong ledger identity'); }
    installation = { contractVersion: v.contractVersion, mode: v.mode, serviceName: v.serviceName, installationId: id(v.installationId), recoveryEpoch: id(v.recoveryEpoch) };
  } else {
    installation = { contractVersion: '1.0', mode: 'openai', serviceName: 'Fort Prototype', installationId: randomUUID(), recoveryEpoch: randomUUID() };
    db.prepare('INSERT INTO metadata VALUES (?,?)').run('installation', JSON.stringify(installation));
  }
  installation = {...installation,capabilities:['requestCancellation','skillBundles','routines','routineGates','schedules']};
  const put = (kind: Entity, entityId: string, data: unknown) => db.prepare('INSERT INTO entities VALUES (?,?,?) ON CONFLICT(kind,id) DO UPDATE SET data=excluded.data').run(kind, entityId, JSON.stringify(data));
  const list = <T>(kind: Entity, decode: (value: unknown) => T): T[] => db.prepare('SELECT data FROM entities WHERE kind=? ORDER BY rowid').all(kind).map(row => decode(json(row)));
  const load = <T>(kind: Entity, entityId: string, decode: (value: unknown) => T): T => decode(json(db.prepare('SELECT data FROM entities WHERE kind=? AND id=?').get(kind, entityId)));
  const authenticate = (request: IncomingMessage) => {
    if (closed) fail(503,'closed','This session has stopped.');
    if (request.headers.origin !== undefined) fail(403, 'originDenied', 'Browser origins are not enabled.');
    if (request.headers['x-fort-installation-id'] !== installation.installationId) fail(401, 'unauthorized', 'Use this installation’s device credential.');
    const token = /^Bearer ([A-Za-z0-9_-]+)$/.exec(request.headers.authorization ?? '')?.[1];
    const device = token && db.prepare('SELECT id,expires FROM devices WHERE hash=?').get(digest(token));
    if (!device || Number(device.expires) <= now()) fail(401, 'unauthorized', 'Device authorization is missing or expired.');
    return id(device.id);
  };
  const owner = <T>(request: IncomingMessage, operation: (deviceId:string) => T): T => transaction(() => operation(authenticate(request)));
  const cursor = () => Number(db.prepare('SELECT coalesce(max(sequence),0) AS cursor FROM events').get()?.cursor);
  const append = (conversationId: string, kind: string, entityId: string) => {
    const event = { schemaVersion: 1, installationId: installation.installationId, recoveryEpoch: installation.recoveryEpoch, eventId: randomUUID(), conversationId, entityId, kind, createdAt: new Date().toISOString(), data: {} };
    return Number(db.prepare('INSERT INTO events(data) VALUES (?)').run(JSON.stringify(event)).lastInsertRowid);
  };
  const held = new Set<string>();
  transaction(() => {
    for (const run of list('runs', decodeRun)) {
      if (run.state === 'running' || run.state === 'cancelRequested') { put('runs', run.runId, { ...run, state: 'outcomeUnknown', statusDetail: 'Service restarted before a confirmed result.',...(run.routine?{routine:stopRoutine(run.routine,'outcomeUnknown','Service restarted before a confirmed result.')}: {}) }); append(run.conversationId, 'runStopped', run.runId); }
      if (run.state === 'queued'&&!run.routine) { held.add(run.runId); put('runs', run.runId, { ...run, statusDetail: 'Held after restart. This candidate does not automatically resume old queued work.' }); }
    }
  });
  const snapshot = (conversationId: string): Snapshot => {
    load('conversations', conversationId, decodeConversation);
    const read = db.prepare("SELECT data FROM entities WHERE kind='reads' AND id=?").get(conversationId);
    return { installationId: installation.installationId, recoveryEpoch: installation.recoveryEpoch, conversationId, cursor: cursor(), messages: list('messages', decodeMessage).filter(m => m.conversationId === conversationId), runs: list('runs', decodeRun).filter(r => r.conversationId === conversationId), approvals: list('routineApprovals',decodeRoutineApproval).filter(approval=>load('runs',approval.runId,decodeRun).conversationId===conversationId), lastReadMessageSequence: read ? Number(record(json(read)).sequence) : 0 };
  };
  const cancelRun=(runId:string):Run=>{
    const run=load('runs',runId,decodeRun);
    if(run.state!=='queued'&&run.state!=='running'&&run.state!=='waitingForApproval')return run;
    const immediate=run.state!=='running';
    const canceled:Run={...run,state:immediate?'canceled':'cancelRequested',statusDetail:immediate?'Canceled before runtime dispatch.':'Stop requested; waiting for runtime confirmation.',...(immediate&&run.routine?{routine:stopRoutine(run.routine,'canceled','Owner canceled the routine.')}:{})};
    if(immediate)for(const approval of list('routineApprovals',decodeRoutineApproval)){if(approval.runId===runId&&approval.state==='pending')put('routineApprovals',approval.approvalId,{...approval,state:'canceled'});}
    put('runs',runId,canceled);append(run.conversationId,'runStopped',runId);return canceled;
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
      const routine:RoutineRevision={routineId:routineId??randomUUID(),revisionId:randomUUID(),digest:routineDigest({definition,agentSnapshot,skills}),definition,agentSnapshot,skills};
      put('routines',routine.routineId,routine);put('routineRevisions',routine.revisionId,routine);return routine;
    });
  };
  const priorAcceptance=(requestId:string,fingerprint:string):Acceptance|undefined=>{
    const prior=db.prepare("SELECT data FROM entities WHERE kind='requests' AND id=?").get(requestId);
    if(!prior)return undefined;
    const stored=record(json(prior));if(stored.fingerprint!==fingerprint)fail(409,'requestConflict','This request already has different contents.');return stored.acceptance as Acceptance;
  };
  const recordSubmission=(command:{conversationId:string;requestId:string;bindingRevisionId:string;inputText:string;fingerprint:string;configuration:TurnConfiguration;routine?:RoutineExecution}):Acceptance=>{
    const {conversationId,requestId,bindingRevisionId,inputText,fingerprint,configuration,routine}=command;
    if(db.prepare("SELECT 1 FROM entities WHERE kind='requestCancellations' AND id=?").get(requestId))fail(409,'requestCanceled','This request was canceled before acceptance and will not run.');
    const messageId=randomUUID(),runId=randomUUID(),acceptedAt=new Date().toISOString(),sequence=append(conversationId,'messageAccepted',messageId);
    const message:Message={messageId,conversationId,requestId,role:'user',text:inputText,sequence,createdAt:acceptedAt,recipientAgentId:configuration.agentId,recipientAgentName:configuration.agentName};
    const run:Run={runId,requestId,conversationId,bindingRevisionId,state:'queued',selectedModel:configuration.selectedModel,effectiveModel:null,modelEvidence:'unreported',attemptId:null,configuration,...(routine?{routine}:{})};
    put('messages',messageId,message);put('runs',runId,run);
    const acceptance:Acceptance={receipt:{requestId,conversationId,messageId,runId,bindingRevisionId,inputDigest:digest(inputText),acceptedAt},snapshot:snapshot(conversationId)};
    put('requests',requestId,{fingerprint,acceptance});return acceptance;
  };
  const saveInstructions=(request:IncomingMessage,body:Record<string,unknown>,skillId?:string)=>owner(request,()=>{
    if(Object.keys(body).some(key=>!['name','instructions','expectedRevisionId'].includes(key)))fail(400,'invalidCommand','Use only the instruction authoring fields. Review authority separately.');
    const old=skillId?load('skills',skillId,decodeSkill):undefined;
    if(old&&body.expectedRevisionId!==old.revisionId)fail(409,'skillChanged','Reload the skill before saving.');
    if(old?.bundle&&(old.bundle.source!=='owner-local:Fort-instructions'||old.bundle.files.length!==1||old.bundle.requestedTools.length!==0||old.bundle.scripts.length!==0||old.bundle.files[0]?.executable||JSON.stringify(old.bundle.supportedRuntimes)!==JSON.stringify(['codex'])))fail(409,'bundleEditRequired','Import the complete revised bundle so its files and authority can be reviewed.');
    const name=text(body.name,'name',128),instructions=text(body.instructions,'instructions'),version=Number(old?.bundle?.version??'0');
    const bundle=parseSkillBundle({schemaVersion:1,name,version:Number.isSafeInteger(version)&&version>=0&&version<Number.MAX_SAFE_INTEGER?String(version+1):randomUUID(),source:old?.bundle?.source??'owner-local:Fort-instructions',license:old?.bundle?.license??'LicenseRef-Owner-Private',entrypoint:old?.bundle?.entrypoint??'SKILL.md',supportedRuntimes:['codex'],requestedTools:[],scripts:[],files:[{path:old?.bundle?.entrypoint??'SKILL.md',kind:'file',executable:false,contentBase64:Buffer.from(instructions).toString('base64')}]});
    const skill:InstructionSkill={skillId:skillId??randomUUID(),revisionId:randomUUID(),name,instructions,bundle,review:{state:'reviewRequired',digest:bundle.digest},...(old?{previousRevisionId:old.revisionId}:{})};
    if(old)put('skillRevisions',old.revisionId,old);
    put('skillRevisions',skill.revisionId,skill);put('skills',skill.skillId,skill);return skill;
  });
  const accept = (request: IncomingMessage, conversationId: string, body: Record<string, unknown>): Acceptance => {
    const requestId = id(body.requestId), bindingRevisionId = id(body.bindingRevisionId), recipientAgentId = id(body.recipientAgentId), recoveryEpoch = id(body.recoveryEpoch), inputText = messageText(body.text);
    if (!Array.isArray(body.skills) || body.skills.length > 8) fail(400, 'invalidSkills', 'Select at most eight instruction skills.');
    const references = body.skills.map(value => { const v = record(value); return { skillId: id(v.skillId), revisionId: id(v.revisionId) }; });
    if (new Set(references.map(s => s.skillId)).size !== references.length) fail(400, 'invalidSkills', 'Select each skill once.');
    const fingerprint = digest(JSON.stringify({ conversationId, bindingRevisionId, recipientAgentId, recoveryEpoch, text: inputText, skills: references }));
    return owner(request, () => {
      if (recoveryEpoch !== installation.recoveryEpoch) fail(409, 'recoveryEpochChanged', 'Review this pending message after recovery.');
      const prior=priorAcceptance(requestId,fingerprint);if(prior)return prior;
      const conversation = load('conversations', conversationId, decodeConversation), agent = load('agents', recipientAgentId, decodeAgent);
      if (!conversation.participantAgentIds.includes(recipientAgentId)) fail(409, 'recipientMismatch', 'Select a participant in this conversation.');
      if (agent.binding.bindingRevisionId !== bindingRevisionId || !agent.binding.enabled) fail(409, 'bindingChanged', 'Review the changed Agent before submitting.');
      const skills = references.map(ref => { const skill = load('skills', ref.skillId, decodeSkill); if (skill.revisionId !== ref.revisionId) fail(409, 'skillChanged', 'Review the changed skill before submitting.'); if(!skill.bundle||skill.review?.state!=='enabled')fail(409,'skillReviewRequired','Migrate, review and enable the exact complete skill bundle before submitting.'); return skill; });
      const configuration: TurnConfiguration = { agentId: agent.agentId, agentName: agent.name, bindingRevisionId, selectedModel: agent.binding.selectedModel, instructions: agent.instructions ?? '', skills };
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
    if(bindingDisabled(saved.definition.bindingRevisionId))fail(409,'bindingDisabled','The routine’s pinned binding revision is disabled.');
    for(const ref of saved.definition.skills){const skill=load('skillRevisions',ref.revisionId,decodeSkill);if(skill.review?.state!=='enabled'||skill.bundle?.digest!==ref.digest)fail(409,'skillReviewRequired','A pinned skill revision is no longer enabled.');}
    const agent=saved.agentSnapshot,definition=saved.definition,configuration:TurnConfiguration={agentId:agent.agentId,agentName:agent.name,bindingRevisionId:definition.bindingRevisionId,selectedModel:agent.binding.selectedModel,instructions:agent.instructions+'\n\nRoutine instructions:\n'+definition.instructions,skills:saved.skills};
    const inputText=messageText('Run routine: '+definition.name+'\nInputs: '+JSON.stringify(inputs)),conversation=load('conversations',definition.conversationId,decodeConversation);
    if(options.admitSubmission&&!options.admitSubmission({text:inputText,conversation,configuration},list('runs',decodeRun)))fail(403,'sampleNotAuthorized','This routine is outside the active session.');
    return recordSubmission({conversationId:definition.conversationId,requestId,bindingRevisionId:definition.bindingRevisionId,inputText,fingerprint,configuration,routine:startRoutine(routineId,revisionId,definition,inputs)});
  };
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
    if(closed||(credentialDeadline!==undefined&&now()>=credentialDeadline))return 0;
    let admitted=0;
    for(const saved of list('schedules',decodeSchedule)){
      if(saved.state!=='enabled')continue;
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
        const missed=downtime||through-schedule.checkedThrough>60_000;
        const catchUp=missed&&schedule.definition.missed==='catchUpOne'?due.findLast(occurrence=>occurrence.intendedAt!==null):undefined;
        for(const occurrence of due){
          const occurrenceId=routineDigest(occurrence.intendedAt===null?{routineRevisionId:schedule.definition.routineRevisionId,gap:occurrence.localDate+'T'+occurrence.localTime,timezone:schedule.definition.timezone}:{routineRevisionId:schedule.definition.routineRevisionId,intendedAt:occurrence.intendedAt});
          if(db.prepare("SELECT 1 FROM entities WHERE kind='scheduleOccurrences' AND id=?").get(occurrenceId))continue;
          let record:ScheduleOccurrence={occurrenceId,scheduleId:schedule.scheduleId,scheduleRevisionId:schedule.revisionId,routineId:schedule.definition.routineId,routineRevisionId:schedule.definition.routineRevisionId,intendedAt:occurrence.intendedAt,localDate:occurrence.localDate,localTime:occurrence.localTime,state:'accepted'};
          if(occurrence.intendedAt===null)record.state='skippedGap';
          else if(missed&&occurrence!==catchUp)record.state='skippedMissed';
          else if(list('runs',decodeRun).some(run=>run.routine?.routineId===schedule.definition.routineId&&['queued','running','waitingForApproval','cancelRequested','outcomeUnknown'].includes(run.state)))record.state='skippedOverlap';
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
  let dispatching: Promise<boolean> | undefined; let active: AbortController | undefined; let activeRunId:string|undefined;
  const executeNext = async (): Promise<boolean> => {
    if (closed) return false;
    const runs = list('runs', decodeRun);
    const queued = runs.find((r, index) => r.state === 'queued' && !held.has(r.runId) && !runs.slice(0, index).some(prior => prior.conversationId === r.conversationId && (prior.state === 'queued' || prior.state === 'running' || prior.state === 'cancelRequested' || prior.state === 'waitingForApproval' || prior.state === 'outcomeUnknown')));
    if (!queued?.configuration) return false;
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
    if(!options.runtime)return false;
    const input = transaction(() => {
      const messages = list('messages', decodeMessage).filter(m => m.conversationId === queued.conversationId);
      const current = messages.find(m => m.requestId === queued.requestId && m.role === 'user');
      if (!current || !queued.configuration) throw new Error('Queued run has no accepted input');
      const attemptId=randomUUID(),routine=queued.routine?{...queued.routine,steps:queued.routine.steps.map((step,index)=>index===stepIndex?{stepId:step.stepId,stepRevisionId:step.stepRevisionId,state:'running' as const,attemptId}:step)}:undefined;
      put('runs', queued.runId, { ...queued, state: 'running', attemptId,...(routine?{routine}:{}) }); append(queued.conversationId, 'runStarted', queued.runId);
      const history = messages.filter(m => m.role === 'user' && m.sequence < current.sequence).flatMap(m => [m, ...messages.filter(reply => reply.role === 'assistant' && reply.requestId === m.requestId)]);
      const next: ChatInput = { runId: queued.runId, requestId: queued.requestId, conversationId: queued.conversationId, configuration: queued.configuration, text: routine?taskInput(routine,routine.steps[stepIndex]!.stepId):current.text, history };
      put('contexts', queued.runId, next); return next;
    });
    active = new AbortController(); activeRunId=queued.runId;
    try {
      const result = await options.runtime.execute(input, active.signal);
      transaction(() => {
        const run = load('runs', queued.runId, decodeRun);
        if (result.kind === 'completed') {
          const messageId = randomUUID(), sequence = append(run.conversationId, 'messageCompleted', messageId);
          const reply: Message = { messageId, conversationId: run.conversationId, requestId: run.requestId, role: 'assistant', text: messageText(result.text), sequence, createdAt: new Date().toISOString(), agentId: input.configuration.agentId, agentName: input.configuration.agentName };
          const routine=run.routine?{...run.routine,steps:run.routine.steps.map(step=>step.state==='running'?{stepId:step.stepId,stepRevisionId:step.stepRevisionId,state:'completed' as const,attemptId:step.attemptId,output:{messageId,text:reply.text,digest:outputDigest(reply.text)}}:step)}:undefined;
          const remaining=routine?.steps.some(step=>step.state==='pending')??false;
          const canceled=remaining&&run.state==='cancelRequested';
          put('messages',messageId,reply);put('runs',run.runId,{...run,state:canceled?'canceled':remaining?'queued':'completed',...(routine?{routine:canceled?stopRoutine(routine,'canceled','Owner cancellation stopped remaining steps.'):routine}:{}),statusDetail:canceled?'Current task completed; remaining routine steps were canceled.':undefined});
        } else {
          const state=result.kind==='notStarted'?(run.state==='cancelRequested'||closed?'canceled':'failed'):result.kind;
          put('runs',run.runId,{...run,state,statusDetail:result.message,...(run.routine?{routine:stopRoutine(run.routine,state,result.message)}:{})});append(run.conversationId,'runStopped',run.runId);
        }
      });
    } catch {
      transaction(() => { const run = load('runs', queued.runId, decodeRun); put('runs', run.runId, { ...run, state: 'outcomeUnknown', statusDetail: 'The runtime ended without a confirmed result.',...(run.routine?{routine:stopRoutine(run.routine,'outcomeUnknown','The runtime ended without a confirmed result.')}: {}) }); append(run.conversationId, 'runStopped', run.runId); });
    } finally { active = undefined; activeRunId=undefined; }
    return true;
  };
  const dispatchNext = (): Promise<boolean> => {
    if (dispatching) return dispatching;
    dispatching = executeNext().finally(() => { dispatching = undefined; }); return dispatching;
  };
  let pumping = false;
  const wake = () => {
    if (!options.autoRun || pumping || closed) return;
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
    if (credentialDeadline!==undefined) {
      if (db.prepare("SELECT 1 FROM metadata WHERE key='credentialClosed'").get() || credentialDeadline<=now()) throw new Error('Bounded enrollment is closed');
      if (db.prepare("SELECT 1 FROM metadata WHERE key='enrollmentIssued'").get()) throw new Error('Bounded enrollment already issued');
    }
    const code = randomBytes(32).toString('base64url');
    transaction(()=>{
      db.prepare('INSERT INTO enrollment VALUES (?,?)').run(digest(code), Math.min(now()+300_000,credentialDeadline ?? Infinity));
      if (credentialDeadline!==undefined) db.prepare('INSERT INTO metadata VALUES (?,?)').run('enrollmentIssued','true');
    });
    return { installationId: installation.installationId, code };
  };
  const server = createServer(async (request, response) => {
    let status = 200; let result: unknown;
    try {
      const url = new URL(request.url ?? '/', 'http://127.0.0.1');
      if (request.method === 'POST' && url.pathname === '/v1/devices/enroll') {
        const body = await readBody(request);
        result = transaction(() => {
          if (request.headers.origin !== undefined || body.installationId !== installation.installationId || typeof body.code !== 'string') fail(401, 'unauthorized', 'Enrollment is not authorized.');
          text(body.deviceName, 'device name', 128);
          const code = db.prepare('SELECT expires FROM enrollment WHERE hash=?').get(digest(body.code));
          if (closed || !code || Number(code.expires) <= now()) fail(401, 'unauthorized', 'Enrollment code is invalid or expired.');
          db.prepare('DELETE FROM enrollment WHERE hash=?').run(digest(body.code));
          const deviceId = randomUUID(), deviceToken = randomBytes(32).toString('base64url'), expires = Math.min(now()+30*86_400_000,credentialDeadline ?? Infinity);
          db.prepare('INSERT INTO devices VALUES (?,?,?)').run(deviceId, digest(deviceToken), expires);
          return { installation, deviceId, deviceToken, expiresAt: new Date(expires).toISOString() };
        }); status = 201;
      } else {
        authenticate(request);
        if (request.method === 'GET' && url.pathname === '/v1/installation') result = installation;
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
        else if (request.method === 'GET' && /^\/v1\/conversations\/[^/]+\/snapshot$/.test(url.pathname)) result = snapshot(id(url.pathname.split('/')[3]));
        else if (request.method === 'GET' && /^\/v1\/requests\/[^/]+$/.test(url.pathname)) result = load('requests', id(url.pathname.split('/')[3]), record).acceptance;
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
        else if (request.method === 'POST' && /^\/v1\/requests\/[^/]+\/cancel$/.test(url.pathname)) {
          const body=await readBody(request),requestId=id(url.pathname.split('/')[3]);
          const cancellation:RequestCancellation=owner(request,()=>{
            if(body.recoveryEpoch!==installation.recoveryEpoch)fail(409,'recoveryEpochChanged','Reload this installation before changing work.');
            const scope={requestId,recoveryEpoch:installation.recoveryEpoch,conversationId:id(body.conversationId),bindingRevisionId:id(body.bindingRevisionId)};
            const prior=db.prepare("SELECT data FROM entities WHERE kind='requestCancellations' AND id=?").get(requestId);
            if(prior){const stored=record(json(prior));if(stored.conversationId!==scope.conversationId||stored.bindingRevisionId!==scope.bindingRevisionId||stored.recoveryEpoch!==scope.recoveryEpoch)fail(409,'requestConflict','The cancellation belongs to a different request scope.');}
            const accepted=db.prepare("SELECT data FROM entities WHERE kind='requests' AND id=?").get(requestId);
            if(accepted){
              const {receipt}=record(json(accepted)).acceptance as Acceptance;
              if(receipt.conversationId!==scope.conversationId||receipt.bindingRevisionId!==scope.bindingRevisionId)fail(409,'requestConflict','The accepted request belongs to a different scope.');
              const run=cancelRun(receipt.runId);put('requestCancellations',requestId,scope);
              return {...scope,status:'accepted',receipt,run,snapshot:snapshot(scope.conversationId)};
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
            const current = snapshot(conversationId), message = current.messages.find(m => m.messageId === body.messageId);
            if (body.recoveryEpoch !== installation.recoveryEpoch || typeof body.cursor !== 'number' || !Number.isSafeInteger(body.cursor) || body.cursor > current.cursor || !message || message.sequence > body.cursor) fail(409, 'readConflict', 'Read acknowledgement does not match this conversation.');
            const sequence = Math.max(current.lastReadMessageSequence, message.sequence); put('reads', conversationId, { sequence }); return { lastReadMessageSequence: sequence };
          });
        }
        else if (request.method === 'POST' && /^\/v1\/agents\/[^/]+$/.test(url.pathname)) {
          const body = await readBody(request), agentId = id(url.pathname.split('/')[3]);
          result = owner(request, () => {
            const old = load('agents', agentId, decodeAgent);
            if (body.expectedBindingRevisionId !== old.binding.bindingRevisionId) fail(409, 'bindingChanged', 'Reload the Agent before saving.');
            const agent: Agent = { ...old, name: text(body.name, 'name', 128), instructions: text(body.instructions, 'instructions'), binding: { ...old.binding, bindingRevisionId: randomUUID(), selectedModel: text(body.model, 'exact model', 128),enabled:true } };
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
            const agent: Agent = { agentId: randomUUID(), name: text(body.name, 'name', 128), instructions: text(body.instructions, 'instructions'), homeConversationId: randomUUID(), readiness: 'requiresLiveReview',
              binding: { bindingRevisionId: randomUUID(), provider: 'codex', runtimeVersion: '0.157.1', selectedModel: text(body.model, 'exact model', 128), hostId: 'local-codex', enabled: true } };
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
    } catch (error) {
      status = error instanceof APIError ? error.status : error instanceof InvalidSkillBundle || error instanceof InvalidRoutine ? 400 : 500;
      result = { error: { code: error instanceof APIError ? error.code : error instanceof InvalidSkillBundle ? 'invalidSkillBundle' : error instanceof InvalidRoutine ? 'invalidRoutine' : 'internalError', message: error instanceof APIError || error instanceof InvalidSkillBundle || error instanceof InvalidRoutine ? error.message : 'Fort could not complete the request.' } };
    }
    response.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' }); response.end(JSON.stringify(result));
  });
  let baseURL = ''; let closed = false; let closing: Promise<void> | undefined; let scheduleTimer:ReturnType<typeof setInterval>|undefined;
  return { installation, issueEnrollment, dispatchNext, tickSchedules,
    inspectSession() { return { installation,runs:list('runs',decodeRun),receipts:list('requests',value=>(record(value).acceptance as Acceptance).receipt),deviceIDs:db.prepare('SELECT id FROM devices').all().map(row=>id(row.id)) }; },
    get baseURL() { return baseURL; }, async start({ port = 0,signal }: { port?:number;signal?:AbortSignal } = {}) {
    if(closed)throw new Error('Service is closed');
    signal?.throwIfAborted();
    await new Promise<void>((resolve, reject) => {
      const cleanup=()=>{server.off('error',failed);server.off('close',canceled);};
      const failed=(error:unknown)=>{cleanup();reject(error);};
      const canceled=()=>failed(new Error('Listener startup canceled'));
      server.once('error',failed);server.once('close',canceled);
      try {server.listen({port,host:'127.0.0.1',signal},()=>{cleanup();resolve();});}catch(error){failed(error);}
    });
    signal?.throwIfAborted();
    const address = server.address(); if (!address || typeof address === 'string') throw new Error('Missing listener'); baseURL = `http://127.0.0.1:${address.port}`;tickSchedules(true);queueMicrotask(wake);
    if(options.autoRun){scheduleTimer=setInterval(()=>{try{tickSchedules();}catch{clearInterval(scheduleTimer);}},1000);scheduleTimer.unref();}
  }, close({ preserveSessionCredential=false }: { preserveSessionCredential?: boolean } = {}): Promise<void> {
    if (closing) return closing;
    closed = true; clearInterval(scheduleTimer); active?.abort();
    if (credentialDeadline!==undefined && !preserveSessionCredential) transaction(()=>{
      db.prepare('DELETE FROM enrollment').run(); db.prepare('DELETE FROM devices').run();
      db.prepare("INSERT OR REPLACE INTO metadata VALUES ('credentialClosed','true')").run();
    });
    closing = (async () => {
      const outcomes = await Promise.allSettled([options.runtime?.dispose(), dispatching]);
      if (server.listening) await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
      db.close();
      if (outcomes.some(result => result.status === 'rejected')) throw new Error('Runtime cleanup was not confirmed; restart is blocked.');
    })();
    return closing;
  } };
}
