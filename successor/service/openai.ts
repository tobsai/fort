import { DatabaseSync } from 'node:sqlite';
import { createServer, type IncomingMessage } from 'node:http';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { mkdirSync, lstatSync, chmodSync } from 'node:fs';
import { isAbsolute, join } from 'node:path';
import type { Agent, Conversation, InstructionSkill, Installation } from '../contracts/v1.ts';

type Entity = 'agents' | 'skills' | 'conversations';
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
function json(row: Record<string, unknown> | undefined): unknown {
  if (!row || typeof row.data !== 'string') fail(404, 'notFound', 'Record was not found.');
  return JSON.parse(row.data);
}

export function createOpenAIService(options: { dataDirectory: string }) {
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
    CREATE TABLE IF NOT EXISTS entities (kind TEXT NOT NULL, id TEXT NOT NULL, data TEXT NOT NULL, PRIMARY KEY(kind,id));`);
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
  return { installation, issueEnrollment, get baseURL() { return baseURL; }, async start({ port = 0 } = {}) {
    await new Promise<void>((resolve, reject) => { server.once('error', reject); server.listen(port, '127.0.0.1', () => { server.off('error', reject); resolve(); }); });
    const address = server.address(); if (!address || typeof address === 'string') throw new Error('Missing listener'); baseURL = `http://127.0.0.1:${address.port}`;
  }, async close() { if (closed) return; closed = true; if (server.listening) await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve())); db.close(); } };
}
