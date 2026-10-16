import { DatabaseSync } from 'node:sqlite';
import { createServer, type IncomingMessage } from 'node:http';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { mkdirSync, lstatSync, chmodSync } from 'node:fs';
import { isAbsolute, join } from 'node:path';
import type { Acceptance, Agent, Conversation, InstructionSkill, Installation, Message, Run, Snapshot, TurnConfiguration } from '../contracts/v1.ts';
import type { ChatInput, ChatRuntime } from '../runner/chat-runtime.ts';

type Entity = 'agents' | 'skills' | 'conversations' | 'messages' | 'runs' | 'requests' | 'contexts' | 'reads';
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
function id(value: unknown): string {
  const result = text(value, 'identifier', 128);
  if (!/^[A-Za-z0-9_-]+$/.test(result)) fail(400, 'invalidCommand', 'Invalid identifier.');
  return result;
}
function decodeAgent(value: unknown): Agent {
  const v = record(value), b = record(v.binding);
  if (b.provider !== 'codex' || b.enabled !== true || v.readiness !== 'requiresLiveReview') throw new Error('Invalid stored Agent');
  return { agentId: id(v.agentId), name: text(v.name, 'name'), homeConversationId: id(v.homeConversationId), instructions: text(v.instructions, 'instructions'), readiness: v.readiness,
    binding: { bindingRevisionId: id(b.bindingRevisionId), provider: b.provider, runtimeVersion: text(b.runtimeVersion, 'runtime'), selectedModel: text(b.selectedModel, 'model'), hostId: id(b.hostId), enabled: b.enabled } };
}
function decodeSkill(value: unknown): InstructionSkill {
  const v = record(value);
  return { skillId: id(v.skillId), revisionId: id(v.revisionId), name: text(v.name, 'name'), instructions: text(v.instructions, 'instructions') };
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
  return { messageId: id(v.messageId), conversationId: id(v.conversationId), requestId: id(v.requestId), role: v.role, text: text(v.text, 'message', 65_536), sequence: v.sequence, createdAt: text(v.createdAt, 'date'),
    ...(v.agentId === undefined ? {} : { agentId: id(v.agentId), agentName: text(v.agentName, 'name') }),
    ...(v.recipientAgentId === undefined ? {} : { recipientAgentId: id(v.recipientAgentId), recipientAgentName: text(v.recipientAgentName, 'name') }) };
}
function decodeRun(value: unknown): Run {
  const v = record(value);
  if (v.state !== 'queued' && v.state !== 'running' && v.state !== 'completed' && v.state !== 'failed' && v.state !== 'outcomeUnknown' && v.state !== 'canceled' && v.state !== 'closedUnknown') throw new Error('Invalid stored Run');
  return { runId: id(v.runId), requestId: id(v.requestId), conversationId: id(v.conversationId), bindingRevisionId: id(v.bindingRevisionId), state: v.state, selectedModel: text(v.selectedModel, 'model'), effectiveModel: null, modelEvidence: 'unreported', attemptId: v.attemptId === null ? null : id(v.attemptId), configuration: decodeConfiguration(v.configuration), ...(v.statusDetail === undefined ? {} : { statusDetail: text(v.statusDetail, 'status') }) };
}
function json(row: Record<string, unknown> | undefined): unknown {
  if (!row || typeof row.data !== 'string') fail(404, 'notFound', 'Record was not found.');
  return JSON.parse(row.data);
}

export function createOpenAIService(options: { dataDirectory: string; runtime?: ChatRuntime; autoRun?: boolean }) {
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
  const put = (kind: Entity, entityId: string, data: unknown) => db.prepare('INSERT INTO entities VALUES (?,?,?) ON CONFLICT(kind,id) DO UPDATE SET data=excluded.data').run(kind, entityId, JSON.stringify(data));
  const list = <T>(kind: Entity, decode: (value: unknown) => T): T[] => db.prepare('SELECT data FROM entities WHERE kind=? ORDER BY rowid').all(kind).map(row => decode(json(row)));
  const load = <T>(kind: Entity, entityId: string, decode: (value: unknown) => T): T => decode(json(db.prepare('SELECT data FROM entities WHERE kind=? AND id=?').get(kind, entityId)));
  const authenticate = (request: IncomingMessage) => {
    if (request.headers.origin !== undefined) fail(403, 'originDenied', 'Browser origins are not enabled.');
    if (request.headers['x-fort-installation-id'] !== installation.installationId) fail(401, 'unauthorized', 'Use this installation’s device credential.');
    const token = /^Bearer ([A-Za-z0-9_-]+)$/.exec(request.headers.authorization ?? '')?.[1];
    const device = token && db.prepare('SELECT id,expires FROM devices WHERE hash=?').get(digest(token));
    if (!device || Number(device.expires) <= Date.now()) fail(401, 'unauthorized', 'Device authorization is missing or expired.');
  };
  const owner = <T>(request: IncomingMessage, operation: () => T): T => transaction(() => { authenticate(request); return operation(); });
  const cursor = () => Number(db.prepare('SELECT coalesce(max(sequence),0) AS cursor FROM events').get()?.cursor);
  const append = (conversationId: string, kind: string, entityId: string) => {
    const event = { schemaVersion: 1, installationId: installation.installationId, recoveryEpoch: installation.recoveryEpoch, eventId: randomUUID(), conversationId, entityId, kind, createdAt: new Date().toISOString(), data: {} };
    return Number(db.prepare('INSERT INTO events(data) VALUES (?)').run(JSON.stringify(event)).lastInsertRowid);
  };
  const held = new Set<string>();
  transaction(() => {
    for (const run of list('runs', decodeRun)) {
      if (run.state === 'running') { put('runs', run.runId, { ...run, state: 'outcomeUnknown', statusDetail: 'Service restarted before a confirmed result.' }); append(run.conversationId, 'runStopped', run.runId); }
      if (run.state === 'queued') { held.add(run.runId); put('runs', run.runId, { ...run, statusDetail: 'Held after restart. This candidate does not automatically resume old queued work.' }); }
    }
  });
  const snapshot = (conversationId: string): Snapshot => {
    load('conversations', conversationId, decodeConversation);
    const read = db.prepare("SELECT data FROM entities WHERE kind='reads' AND id=?").get(conversationId);
    return { installationId: installation.installationId, recoveryEpoch: installation.recoveryEpoch, conversationId, cursor: cursor(), messages: list('messages', decodeMessage).filter(m => m.conversationId === conversationId), runs: list('runs', decodeRun).filter(r => r.conversationId === conversationId), approvals: [], lastReadMessageSequence: read ? Number(record(json(read)).sequence) : 0 };
  };
  const accept = (request: IncomingMessage, conversationId: string, body: Record<string, unknown>): Acceptance => {
    const requestId = id(body.requestId), bindingRevisionId = id(body.bindingRevisionId), recipientAgentId = id(body.recipientAgentId), recoveryEpoch = id(body.recoveryEpoch), inputText = text(body.text, 'message', 65_536);
    if (!Array.isArray(body.skills) || body.skills.length > 8) fail(400, 'invalidSkills', 'Select at most eight instruction skills.');
    const references = body.skills.map(value => { const v = record(value); return { skillId: id(v.skillId), revisionId: id(v.revisionId) }; });
    if (new Set(references.map(s => s.skillId)).size !== references.length) fail(400, 'invalidSkills', 'Select each skill once.');
    const fingerprint = digest(JSON.stringify({ conversationId, bindingRevisionId, recipientAgentId, recoveryEpoch, text: inputText, skills: references }));
    return owner(request, () => {
      if (recoveryEpoch !== installation.recoveryEpoch) fail(409, 'recoveryEpochChanged', 'Review this pending message after recovery.');
      const prior = db.prepare("SELECT data FROM entities WHERE kind='requests' AND id=?").get(requestId);
      if (prior) { const stored = record(json(prior)); if (stored.fingerprint !== fingerprint) fail(409, 'requestConflict', 'This request already has different contents.'); return stored.acceptance as Acceptance; }
      const conversation = load('conversations', conversationId, decodeConversation), agent = load('agents', recipientAgentId, decodeAgent);
      if (!conversation.participantAgentIds.includes(recipientAgentId)) fail(409, 'recipientMismatch', 'Select a participant in this conversation.');
      if (agent.binding.bindingRevisionId !== bindingRevisionId || !agent.binding.enabled) fail(409, 'bindingChanged', 'Review the changed Agent before submitting.');
      const skills = references.map(ref => { const skill = load('skills', ref.skillId, decodeSkill); if (skill.revisionId !== ref.revisionId) fail(409, 'skillChanged', 'Review the changed skill before submitting.'); return skill; });
      const configuration: TurnConfiguration = { agentId: agent.agentId, agentName: agent.name, bindingRevisionId, selectedModel: agent.binding.selectedModel, instructions: agent.instructions ?? '', skills };
      const messageId = randomUUID(), runId = randomUUID(), acceptedAt = new Date().toISOString();
      const sequence = append(conversationId, 'messageAccepted', messageId);
      const message: Message = { messageId, conversationId, requestId, role: 'user', text: inputText, sequence, createdAt: acceptedAt, recipientAgentId, recipientAgentName: agent.name };
      const run: Run = { runId, requestId, conversationId, bindingRevisionId, state: 'queued', selectedModel: configuration.selectedModel, effectiveModel: null, modelEvidence: 'unreported', attemptId: null, configuration };
      put('messages', messageId, message); put('runs', runId, run);
      const acceptance: Acceptance = { receipt: { requestId, conversationId, messageId, runId, bindingRevisionId, inputDigest: digest(inputText), acceptedAt }, snapshot: snapshot(conversationId) };
      put('requests', requestId, { fingerprint, acceptance }); return acceptance;
    });
  };
  let dispatching: Promise<boolean> | undefined; let active: AbortController | undefined;
  const executeNext = async (): Promise<boolean> => {
    if (closed || !options.runtime) return false;
    const runs = list('runs', decodeRun);
    const queued = runs.find((r, index) => r.state === 'queued' && !held.has(r.runId) && !runs.slice(0, index).some(prior => prior.conversationId === r.conversationId && (prior.state === 'queued' || prior.state === 'running' || prior.state === 'outcomeUnknown')));
    if (!queued?.configuration) return false;
    const input = transaction(() => {
      const messages = list('messages', decodeMessage).filter(m => m.conversationId === queued.conversationId);
      const current = messages.find(m => m.requestId === queued.requestId && m.role === 'user');
      if (!current || !queued.configuration) throw new Error('Queued run has no accepted input');
      put('runs', queued.runId, { ...queued, state: 'running', attemptId: randomUUID() }); append(queued.conversationId, 'runStarted', queued.runId);
      const history = messages.filter(m => m.role === 'user' && m.sequence < current.sequence).flatMap(m => [m, ...messages.filter(reply => reply.role === 'assistant' && reply.requestId === m.requestId)]);
      const next: ChatInput = { runId: queued.runId, requestId: queued.requestId, conversationId: queued.conversationId, configuration: queued.configuration, text: current.text, history };
      put('contexts', queued.runId, next); return next;
    });
    active = new AbortController();
    try {
      const result = await options.runtime.execute(input, active.signal);
      transaction(() => {
        const run = load('runs', queued.runId, decodeRun);
        if (result.kind === 'completed') {
          const messageId = randomUUID(), sequence = append(run.conversationId, 'messageCompleted', messageId);
          const reply: Message = { messageId, conversationId: run.conversationId, requestId: run.requestId, role: 'assistant', text: text(result.text, 'reply', 65_536), sequence, createdAt: new Date().toISOString(), agentId: input.configuration.agentId, agentName: input.configuration.agentName };
          put('messages', messageId, reply); put('runs', run.runId, { ...run, state: 'completed' });
        } else { put('runs', run.runId, { ...run, state: result.kind, statusDetail: result.message }); append(run.conversationId, 'runStopped', run.runId); }
      });
    } catch {
      transaction(() => { const run = load('runs', queued.runId, decodeRun); put('runs', run.runId, { ...run, state: 'outcomeUnknown', statusDetail: 'The runtime ended without a confirmed result.' }); append(run.conversationId, 'runStopped', run.runId); });
    } finally { active = undefined; }
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
    const code = randomBytes(32).toString('base64url');
    db.prepare('INSERT INTO enrollment VALUES (?,?)').run(digest(code), Date.now() + 300_000);
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
          if (!code || Number(code.expires) <= Date.now()) fail(401, 'unauthorized', 'Enrollment code is invalid or expired.');
          db.prepare('DELETE FROM enrollment WHERE hash=?').run(digest(body.code));
          const deviceId = randomUUID(), deviceToken = randomBytes(32).toString('base64url'), expires = Date.now() + 30 * 86_400_000;
          db.prepare('INSERT INTO devices VALUES (?,?,?)').run(deviceId, digest(deviceToken), expires);
          return { installation, deviceId, deviceToken, expiresAt: new Date(expires).toISOString() };
        }); status = 201;
      } else {
        authenticate(request);
        if (request.method === 'GET' && url.pathname === '/v1/installation') result = installation;
        else if (request.method === 'GET' && url.pathname === '/v1/agents') result = { agents: list('agents', decodeAgent) };
        else if (request.method === 'GET' && url.pathname === '/v1/skills') result = { skills: list('skills', decodeSkill) };
        else if (request.method === 'GET' && url.pathname === '/v1/conversations') result = { conversations: list('conversations', decodeConversation) };
        else if (request.method === 'GET' && /^\/v1\/conversations\/[^/]+\/snapshot$/.test(url.pathname)) result = snapshot(id(url.pathname.split('/')[3]));
        else if (request.method === 'GET' && /^\/v1\/requests\/[^/]+$/.test(url.pathname)) result = load('requests', id(url.pathname.split('/')[3]), record).acceptance;
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
            const agent: Agent = { ...old, name: text(body.name, 'name', 128), instructions: text(body.instructions, 'instructions'), binding: { ...old.binding, bindingRevisionId: randomUUID(), selectedModel: text(body.model, 'exact model', 128) } };
            put('agents', agentId, agent);
            const home = load('conversations', old.homeConversationId, decodeConversation); put('conversations', home.conversationId, { ...home, name: agent.name }); return agent;
          });
        } else if (request.method === 'POST' && /^\/v1\/skills\/[^/]+$/.test(url.pathname)) {
          const body = await readBody(request), skillId = id(url.pathname.split('/')[3]);
          result = owner(request, () => {
            const old = load('skills', skillId, decodeSkill);
            if (body.expectedRevisionId !== old.revisionId) fail(409, 'skillChanged', 'Reload the skill before saving.');
            const skill: InstructionSkill = { skillId, revisionId: randomUUID(), name: text(body.name, 'name', 128), instructions: text(body.instructions, 'instructions') }; put('skills', skillId, skill); return skill;
          });
        }
        else if (request.method === 'POST' && url.pathname === '/v1/agents') {
          const body = await readBody(request);
          result = owner(request, () => {
            const agent: Agent = { agentId: randomUUID(), name: text(body.name, 'name', 128), instructions: text(body.instructions, 'instructions'), homeConversationId: randomUUID(), readiness: 'requiresLiveReview',
              binding: { bindingRevisionId: randomUUID(), provider: 'codex', runtimeVersion: '0.157.1', selectedModel: text(body.model, 'exact model', 128), hostId: 'local-codex', enabled: true } };
            put('agents', agent.agentId, agent);
            const home: Conversation = { conversationId: agent.homeConversationId, name: agent.name, kind: 'home', participantAgentIds: [agent.agentId] };
            put('conversations', home.conversationId, home); return agent;
          }); status = 201;
        } else if (request.method === 'POST' && url.pathname === '/v1/skills') {
          const body = await readBody(request);
          result = owner(request, () => { const skill: InstructionSkill = { skillId: randomUUID(), revisionId: randomUUID(), name: text(body.name, 'name', 128), instructions: text(body.instructions, 'instructions') }; put('skills', skill.skillId, skill); return skill; }); status = 201;
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
      status = error instanceof APIError ? error.status : 500;
      result = { error: { code: error instanceof APIError ? error.code : 'internalError', message: error instanceof APIError ? error.message : 'Fort could not complete the request.' } };
    }
    response.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' }); response.end(JSON.stringify(result));
  });
  let baseURL = ''; let closed = false;
  return { installation, issueEnrollment, dispatchNext, get baseURL() { return baseURL; }, async start({ port = 0 } = {}) {
    await new Promise<void>((resolve, reject) => { server.once('error', reject); server.listen(port, '127.0.0.1', () => { server.off('error', reject); resolve(); }); });
    const address = server.address(); if (!address || typeof address === 'string') throw new Error('Missing listener'); baseURL = `http://127.0.0.1:${address.port}`;
  }, async close() { if (closed) return; closed = true; active?.abort(); await dispatching; if (server.listening) await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve())); db.close(); } };
}
