import { DatabaseSync } from 'node:sqlite';
import { createServer, type IncomingMessage } from 'node:http';
import { randomUUID, randomBytes, createHash } from 'node:crypto';
import { mkdirSync, chmodSync, lstatSync, readdirSync, existsSync } from 'node:fs';
import { isAbsolute, join } from 'node:path';
import type { Installation, Agent, Message, Run, Snapshot, Acceptance, DurableEvent, Submission, Approval } from '../contracts/v1.ts';
import { MAX_TEXT_BYTES } from '../contracts/v1.ts';
import { executeFixture, FixtureStopped, type FixtureDispatch, type FixtureEvent } from '../runner/fixture.ts';

type Options = { dataDirectory: string; autoRun?: boolean; now?: () => number; checkpoint?: (point: string) => void; fixtureBehavior?: 'echo' | 'approval' | 'hold' };
class APIError extends Error {
  status: number; code: string;
  constructor(status: number, code: string, message: string) { super(message); this.status = status; this.code = code; }
}
function fail(status: number, code: string, message: string): never { throw new APIError(status, code, message); }
const digest = (value: string) => createHash('sha256').update(value, 'utf8').digest('hex');
const opaque = () => randomBytes(32).toString('base64url');
const identifier = (v: unknown): v is string => typeof v === 'string' && /^[A-Za-z0-9_-]{1,128}$/.test(v);
const object = (value: unknown): Record<string, unknown> => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) fail(400, 'invalidCommand', 'Expected a JSON object.');
  return value as Record<string, unknown>;
};

export function createFixtureService(options: Options) {
  if (!isAbsolute(options.dataDirectory)) throw new Error('An explicit absolute fixture data directory is required.');
  mkdirSync(options.dataDirectory, { recursive: true, mode: 0o700 });
  if (lstatSync(options.dataDirectory).isSymbolicLink()) throw new Error('The fixture data directory cannot be a symlink.');
  const allowedFiles = new Set(['fixture.sqlite', 'fixture.sqlite-wal', 'fixture.sqlite-shm', 'fixture.sqlite-journal']);
  for (const name of readdirSync(options.dataDirectory)) {
    if (!allowedFiles.has(name)) throw new Error('Use a fresh fixture directory or the existing dedicated fixture ledger.');
    const info = lstatSync(join(options.dataDirectory, name));
    if (info.isSymbolicLink() || !info.isFile()) throw new Error('Fixture ledger files cannot be symlinks or non-files.');
  }
  chmodSync(options.dataDirectory, 0o700);
  const databasePath = join(options.dataDirectory, 'fixture.sqlite');
  const existing = existsSync(databasePath);
  const db = new DatabaseSync(databasePath);
  try {
    db.exec('PRAGMA locking_mode=EXCLUSIVE; BEGIN EXCLUSIVE; COMMIT;');
  } catch {
    db.close(); throw new Error('The ledger already has an active service writer or cannot be locked.');
  }
  if (existing) {
    try {
      const metadata = db.prepare("SELECT value FROM metadata WHERE key='installation'").get();
      const previous = JSON.parse(String(metadata?.value));
      if (previous.mode !== 'fixture' || previous.contractVersion !== '1.0') throw new Error('Wrong fixture identity');
    } catch {
      db.close(); throw new Error('Existing file is not a compatible Fort fixture ledger.');
    }
  }
  db.exec(`PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL; PRAGMA foreign_keys=ON;
    CREATE TABLE IF NOT EXISTS metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);
    CREATE TABLE IF NOT EXISTS enrollment (code_hash TEXT PRIMARY KEY, expires INTEGER NOT NULL, used INTEGER NOT NULL DEFAULT 0);
    CREATE TABLE IF NOT EXISTS devices (id TEXT PRIMARY KEY, name TEXT NOT NULL, token_hash TEXT UNIQUE NOT NULL, expires INTEGER NOT NULL, revoked INTEGER NOT NULL DEFAULT 0);
    CREATE TABLE IF NOT EXISTS agents (id TEXT PRIMARY KEY, home TEXT UNIQUE NOT NULL, data TEXT NOT NULL);
    CREATE TABLE IF NOT EXISTS requests (id TEXT PRIMARY KEY, fingerprint TEXT NOT NULL, acceptance TEXT);
    CREATE TABLE IF NOT EXISTS messages (id TEXT PRIMARY KEY, conversation TEXT NOT NULL REFERENCES agents(home), request TEXT NOT NULL REFERENCES requests(id), sequence INTEGER NOT NULL, data TEXT NOT NULL);
    CREATE TABLE IF NOT EXISTS runs (id TEXT PRIMARY KEY, conversation TEXT NOT NULL REFERENCES agents(home), request TEXT UNIQUE NOT NULL REFERENCES requests(id), state TEXT NOT NULL, data TEXT NOT NULL);
    CREATE TABLE IF NOT EXISTS outbox (run TEXT PRIMARY KEY REFERENCES runs(id), state TEXT NOT NULL DEFAULT 'queued');
    CREATE TABLE IF NOT EXISTS attempts (id TEXT PRIMARY KEY, run TEXT NOT NULL REFERENCES runs(id), generation INTEGER NOT NULL, token_hash TEXT NOT NULL, lease_expires INTEGER NOT NULL);
    CREATE TABLE IF NOT EXISTS approvals (id TEXT PRIMARY KEY, run TEXT UNIQUE NOT NULL REFERENCES runs(id), state TEXT NOT NULL, data TEXT NOT NULL);
    CREATE TABLE IF NOT EXISTS events (sequence INTEGER PRIMARY KEY AUTOINCREMENT, data TEXT NOT NULL);
    CREATE TABLE IF NOT EXISTS reads (conversation TEXT PRIMARY KEY REFERENCES agents(home), sequence INTEGER NOT NULL);`);
  const now = options.now ?? Date.now;
  const checkpoint = options.checkpoint ?? (() => {});
  const timestamp = () => new Date(now()).toISOString();
  const transaction = <T>(f: () => T): T => {
    db.exec('BEGIN IMMEDIATE');
    try { const result = f(); db.exec('COMMIT'); return result; }
    catch (error) { db.exec('ROLLBACK'); throw error; }
  };
  const getMeta = (key: string) => (db.prepare('SELECT value FROM metadata WHERE key=?').get(key) as { value: string } | undefined)?.value;
  if (!getMeta('installation')) transaction(() => {
    const installation: Installation = { contractVersion: '1.0', installationId: randomUUID(), recoveryEpoch: randomUUID(), mode: 'fixture', serviceName: 'Fort Fixture' };
    db.prepare('INSERT INTO metadata VALUES (?,?)').run('installation', JSON.stringify(installation));
    const agent: Agent = { agentId: randomUUID(), name: 'Fixture Agent', homeConversationId: randomUUID(), binding: {
      bindingRevisionId: randomUUID(), provider: 'fixture', runtimeVersion: '1', selectedModel: 'fixture-echo-v1', hostId: 'fixture-local', enabled: true }, readiness: 'fixtureReady' };
    db.prepare('INSERT INTO agents VALUES (?,?,?)').run(agent.agentId, agent.homeConversationId, JSON.stringify(agent));
  });
  const installation: Installation = JSON.parse(getMeta('installation')!);
  const cursor = () => Number((db.prepare('SELECT COALESCE(MAX(sequence),0) AS n FROM events').get() as { n: number }).n);
  const agentFor = (conversation: string): Agent => {
    const row = db.prepare('SELECT data FROM agents WHERE home=?').get(conversation) as { data: string } | undefined;
    if (!row) fail(404, 'notFound', 'Conversation was not found.');
    return JSON.parse(row.data);
  };
  const snapshot = (conversation: string): Snapshot => {
    agentFor(conversation);
    return { installationId: installation.installationId, recoveryEpoch: installation.recoveryEpoch, conversationId: conversation, cursor: cursor(),
      messages: db.prepare('SELECT data FROM messages WHERE conversation=? ORDER BY sequence').all(conversation).map(r => JSON.parse(r.data as string)),
      runs: db.prepare('SELECT data FROM runs WHERE conversation=? ORDER BY rowid').all(conversation).map(r => JSON.parse(r.data as string)),
      approvals: db.prepare('SELECT a.data FROM approvals a JOIN runs r ON r.id=a.run WHERE r.conversation=? ORDER BY a.rowid').all(conversation).map(r => JSON.parse(r.data as string)),
      lastReadMessageSequence: Number(db.prepare('SELECT sequence FROM reads WHERE conversation=?').get(conversation)?.sequence ?? 0) };
  };
  const appendEvent = (conversationId: string, entityId: string, kind: string, data: unknown): number => {
    const inserted = db.prepare('INSERT INTO events(data) VALUES (?)').run('{}');
    const sequence = Number(inserted.lastInsertRowid);
    const event: DurableEvent = { schemaVersion: 1, ...installation, eventId: randomUUID(), sequence, entityId, conversationId, kind, createdAt: timestamp(), data };
    db.prepare('UPDATE events SET data=? WHERE sequence=?').run(JSON.stringify(event), sequence);
    return sequence;
  };
  const issueEnrollment = (settings: { ttlMs?: number } = {}) => {
    const code = opaque(); const expiresAt = now() + (settings.ttlMs ?? 300_000);
    db.prepare('INSERT INTO enrollment(code_hash,expires) VALUES (?,?)').run(digest(code), expiresAt);
    return { installationId: installation.installationId, code, expiresAt: new Date(expiresAt).toISOString() };
  };
  const authenticate = (request: IncomingMessage) => {
    if (request.headers.origin !== undefined) fail(403, 'originDenied', 'Browser origins are not enabled for this fixture.');
    if (request.headers['x-fort-installation-id'] !== installation.installationId) fail(401, 'unauthorized', 'Connect with this installation’s device credential.');
    const match = /^Bearer ([A-Za-z0-9_-]+)$/.exec(request.headers.authorization ?? '');
    const device = match && db.prepare('SELECT id,expires,revoked FROM devices WHERE token_hash=?').get(digest(match[1]));
    if (!device || device.revoked || Number(device.expires) <= now()) fail(401, 'unauthorized', 'Device authorization is missing or expired.');
    return String(device.id);
  };
  const ownerTransaction = <T>(request: IncomingMessage, command: (deviceId: string) => T): T => transaction(() => command(authenticate(request)));
  const body = async (request: IncomingMessage, allowEmpty = false) => {
    const chunks: Buffer[] = []; let length = 0;
    for await (const chunk of request) { length += chunk.length; if (length > 1_000_000) fail(413, 'tooLarge', 'The command is too large.'); chunks.push(chunk); }
    if (allowEmpty && length === 0) return {};
    try { return object(JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(Buffer.concat(chunks)))); }
    catch (error) { if (error instanceof APIError) throw error; fail(400, 'invalidJSON', 'Use valid UTF-8 JSON.'); }
  };
  const accept = (request: IncomingMessage, conversationId: string, input: Record<string, unknown>) => {
    if (!identifier(input.requestId) || !identifier(input.bindingRevisionId) || !identifier(input.recoveryEpoch) || typeof input.text !== 'string' || !input.text.isWellFormed()) fail(400, 'invalidCommand', 'A request ID, binding revision, recovery epoch, and valid text are required.');
    if (Buffer.byteLength(input.text as string, 'utf8') > MAX_TEXT_BYTES) fail(413, 'textTooLarge', 'Text must be at most 65,536 UTF-8 bytes.');
    const command = input as Submission;
    if (command.recoveryEpoch !== installation.recoveryEpoch) fail(409, 'recoveryEpochChanged', 'Reconcile this request before sending in a new recovery epoch.');
    const fingerprint = digest(JSON.stringify([conversationId, command.bindingRevisionId, command.recoveryEpoch, command.text]));
    return ownerTransaction(request, () => {
      const previous = db.prepare('SELECT fingerprint,acceptance FROM requests WHERE id=?').get(command.requestId);
      if (previous) {
        if (previous.fingerprint !== fingerprint) fail(409, 'requestConflict', 'This request ID was already used with different content.');
        return { duplicate: true, acceptance: JSON.parse(previous.acceptance as string) as Acceptance };
      }
      const agent = agentFor(conversationId);
      if (!agent.binding.enabled || command.bindingRevisionId !== agent.binding.bindingRevisionId) fail(409, 'bindingConflict', 'The intended binding is no longer the enabled current revision.');
      const acceptedAt = timestamp(); const messageId = randomUUID(); const runId = randomUUID();
      const message: Message = { messageId, conversationId, requestId: command.requestId, role: 'user', text: command.text, sequence: cursor() + 1, createdAt: acceptedAt };
      const run: Run = { runId, conversationId, requestId: command.requestId, bindingRevisionId: command.bindingRevisionId, state: 'queued', selectedModel: agent.binding.selectedModel, effectiveModel: null, modelEvidence: 'unreported', attemptId: null };
      db.prepare('INSERT INTO requests(id,fingerprint) VALUES (?,?)').run(command.requestId, fingerprint);
      checkpoint('afterRequest');
      db.prepare('INSERT INTO messages VALUES (?,?,?,?,?)').run(messageId, conversationId, command.requestId, message.sequence, JSON.stringify(message));
      checkpoint('afterMessage');
      db.prepare('INSERT INTO runs VALUES (?,?,?,?,?)').run(runId, conversationId, command.requestId, run.state, JSON.stringify(run));
      checkpoint('afterRun');
      db.prepare('INSERT INTO outbox(run) VALUES (?)').run(runId);
      checkpoint('afterOutbox');
      appendEvent(conversationId, runId, 'message.accepted', { message, run });
      checkpoint('afterEvent');
      const acceptance: Acceptance = { receipt: { requestId: command.requestId, conversationId, messageId, runId, bindingRevisionId: command.bindingRevisionId, inputDigest: digest(command.text), acceptedAt }, snapshot: snapshot(conversationId) };
      db.prepare('UPDATE requests SET acceptance=? WHERE id=?').run(JSON.stringify(acceptance), command.requestId);
      checkpoint('beforeAcceptanceCommit');
      return { duplicate: false, acceptance };
    });
  };
  const getRun = (runId: string): Run => {
    const row = db.prepare('SELECT data FROM runs WHERE id=?').get(runId);
    if (!row) fail(404, 'notFound', 'Run was not found.');
    return JSON.parse(row.data as string);
  };
  const getApproval = (approvalId: string): Approval => {
    const row = db.prepare('SELECT data FROM approvals WHERE id=?').get(approvalId);
    if (!row) fail(404, 'notFound', 'Approval was not found.');
    return JSON.parse(row.data as string);
  };
  const updateApproval = (approval: Approval) => db.prepare('UPDATE approvals SET state=?,data=? WHERE id=?').run(approval.state, JSON.stringify(approval), approval.approvalId);
  const updateRun = (run: Run) => db.prepare('UPDATE runs SET state=?,data=? WHERE id=?').run(run.state, JSON.stringify(run), run.runId);
  const markUnknown = (runId: string) => transaction(() => {
    const row = db.prepare('SELECT data,state FROM runs WHERE id=?').get(runId);
    if (!row || row.state === 'completed' || row.state === 'canceled' || row.state === 'closedUnknown' || row.state === 'outcomeUnknown') return;
    const run: Run = JSON.parse(row.data as string); run.state = 'outcomeUnknown'; updateRun(run);
    db.prepare("UPDATE outbox SET state='outcomeUnknown' WHERE run=?").run(runId);
    appendEvent(run.conversationId, runId, 'run.updated', { run });
  });
  // A claimed attempt may have acted before a crash. Restart never requeues it.
  for (const row of db.prepare("SELECT run FROM outbox WHERE state='claimed'").all()) markUnknown(String(row.run));
  const ingest = (runId: string, event: FixtureEvent) => transaction(() => {
    const attempt = db.prepare('SELECT * FROM attempts WHERE id=? AND run=?').get(event.attemptId, runId);
    const row = db.prepare('SELECT data FROM runs WHERE id=?').get(runId);
    if (!attempt || !row || attempt.generation !== event.generation || attempt.token_hash !== digest(event.dispatchToken)) throw new Error('Uncorrelated runner event');
    const run: Run = JSON.parse(row.data as string);
    if (run.attemptId !== event.attemptId || !['queued', 'running', 'cancelRequested'].includes(run.state)) throw new Error('Attempt is fenced');
    if (run.state === 'cancelRequested' && event.kind !== 'completed') return;
    if (event.kind === 'started') {
      if (run.state !== 'queued') throw new Error('Duplicate start evidence');
      run.state = 'running'; updateRun(run); appendEvent(run.conversationId, runId, 'run.updated', { run }); return;
    }
    if (event.kind === 'approvalRequested') {
      if (run.state !== 'running' || options.fixtureBehavior !== 'approval') throw new Error('Unsupported approval request');
      const approval: Approval = { approvalId: randomUUID(), installationId: installation.installationId, recoveryEpoch: installation.recoveryEpoch,
        runId, attemptId: event.attemptId, bindingRevisionId: run.bindingRevisionId,
        actionDigest: digest(JSON.stringify(['fixture.reply', runId, event.attemptId, run.bindingRevisionId])),
        resourceScope: ['fixture:reply'], actionDescription: 'Produce the fixture reply', expiresAt: new Date(now() + 300_000).toISOString(),
        state: 'pending', decidedByDeviceId: null, decidedAt: null };
      db.prepare('INSERT INTO approvals VALUES (?,?,?,?)').run(approval.approvalId, runId, approval.state, JSON.stringify(approval));
      run.state = 'waitingForApproval'; updateRun(run);
      db.prepare("UPDATE outbox SET state='waitingForApproval' WHERE run=?").run(runId);
      appendEvent(run.conversationId, approval.approvalId, 'approval.requested', { approval });
      appendEvent(run.conversationId, runId, 'run.updated', { run }); return;
    }
    if (!['running', 'cancelRequested'].includes(run.state) || typeof event.text !== 'string' || event.effectiveModel !== run.selectedModel) throw new Error('Invalid terminal evidence');
    const message: Message = { messageId: randomUUID(), conversationId: run.conversationId, requestId: run.requestId, role: 'assistant', text: event.text, sequence: cursor() + 1, createdAt: timestamp() };
    db.prepare('INSERT INTO messages VALUES (?,?,?,?,?)').run(message.messageId, message.conversationId, message.requestId, message.sequence, JSON.stringify(message));
    appendEvent(run.conversationId, message.messageId, 'message.created', { message });
    run.state = 'completed'; run.effectiveModel = event.effectiveModel; run.modelEvidence = 'fixture'; updateRun(run);
    db.prepare("UPDATE outbox SET state='completed' WHERE run=?").run(runId);
    appendEvent(run.conversationId, runId, 'run.updated', { run });
    checkpoint('beforeTerminalCommit');
  });
  let activeRun: { runId: string; controller: AbortController } | undefined;
  let dispatching: Promise<boolean> | undefined;
  let closed = false;
  const dispatchNextFixture = (): Promise<boolean> => {
    if (closed) return Promise.resolve(false);
    if (dispatching) return dispatching;
    const claimed = transaction(() => {
      const row = db.prepare(`SELECT r.data,o.state AS dispatch_state FROM runs r JOIN outbox o ON o.run=r.id
        WHERE o.state IN ('queued','resume') AND NOT EXISTS (SELECT 1 FROM runs prior WHERE prior.conversation=r.conversation
        AND prior.rowid<r.rowid AND prior.state NOT IN ('completed','canceled','closedUnknown')) ORDER BY r.rowid LIMIT 1`).get();
      if (!row) return undefined;
      const run: Run = JSON.parse(row.data as string);
      const agent = agentFor(run.conversationId);
      if (!agent.binding.enabled || agent.binding.bindingRevisionId !== run.bindingRevisionId) return undefined;
      const resuming = row.dispatch_state === 'resume';
      const priorAttempt = run.attemptId ? db.prepare('SELECT * FROM attempts WHERE id=?').get(run.attemptId) : undefined;
      if (resuming) {
        const approvalRow = db.prepare('SELECT data FROM approvals WHERE run=?').get(run.runId);
        const approval: Approval | undefined = approvalRow ? JSON.parse(approvalRow.data as string) : undefined;
        if (!approval || approval.state !== 'approved' || approval.attemptId !== run.attemptId || !priorAttempt) throw new Error('No exact approved fixture continuation');
        if (Date.parse(approval.expiresAt) <= now()) {
          approval.state = 'expired'; updateApproval(approval); run.state = 'canceled'; updateRun(run);
          db.prepare("UPDATE outbox SET state='canceled' WHERE run=?").run(run.runId);
          appendEvent(run.conversationId, approval.approvalId, 'approval.updated', { approval });
          appendEvent(run.conversationId, run.runId, 'run.updated', { run }); return { expired: true as const };
        }
      }
      const attemptId = resuming ? run.attemptId! : randomUUID(); const dispatchToken = opaque(); const generation = resuming ? Number(priorAttempt!.generation) + 1 : 1;
      db.prepare('INSERT INTO attempts VALUES (?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET generation=excluded.generation, token_hash=excluded.token_hash, lease_expires=excluded.lease_expires').run(attemptId, run.runId, generation, digest(dispatchToken), now() + 30_000);
      db.prepare("UPDATE outbox SET state='claimed' WHERE run=?").run(run.runId);
      run.attemptId = attemptId; updateRun(run);
      const message = db.prepare("SELECT data FROM messages WHERE request=? AND json_extract(data,'$.role')='user'").get(run.requestId)!;
      const dispatch: FixtureDispatch = { protocolVersion: 1, provider: 'fixture', selectedModel: 'fixture-echo-v1', attemptId, generation, dispatchToken, text: JSON.parse(message.data as string).text, behavior: resuming ? 'resume' : (options.fixtureBehavior ?? 'echo') };
      return { runId: run.runId, dispatch };
    });
    if (!claimed) return Promise.resolve(false);
    if ('expired' in claimed) return dispatchNextFixture();
    checkpoint('afterClaim');
    const controller = new AbortController(); activeRun = { runId: claimed.runId, controller };
    dispatching = executeFixture(claimed.dispatch, event => {
      ingest(claimed.runId, event);
      checkpoint(event.kind === 'started' ? 'afterStarted' : event.kind === 'completed' ? 'afterTerminalCommit' : 'afterApprovalCommit');
    }, controller.signal)
      .then(() => true, error => {
        if (error instanceof FixtureStopped && getRun(claimed.runId).state === 'cancelRequested') transaction(() => {
          const run = getRun(claimed.runId); run.state = 'canceled'; updateRun(run);
          db.prepare("UPDATE outbox SET state='canceled' WHERE run=?").run(run.runId);
          appendEvent(run.conversationId, run.runId, 'run.updated', { run });
        });
        else markUnknown(claimed.runId);
        return true;
      }).finally(() => { dispatching = undefined; activeRun = undefined; });
    return dispatching;
  };
  const pump = async () => { while (!closed && await dispatchNextFixture()) {} };
  const server = createServer(async (request, response) => {
    const send = (status: number, data: unknown) => { response.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' }); response.end(JSON.stringify(data)); };
    try {
      const url = new URL(request.url!, 'http://fixture.invalid'); const path = url.pathname;
      if (request.headers.origin !== undefined) fail(403, 'originDenied', 'Browser origins are not enabled for this fixture.');
      if (path === '/v1/devices/enroll' && request.method === 'POST') {
        const input = await body(request);
        const result = transaction(() => {
          const code = typeof input.code === 'string' ? db.prepare('SELECT * FROM enrollment WHERE code_hash=?').get(digest(input.code)) : undefined;
          if (input.installationId !== installation.installationId || !code || code.used || Number(code.expires) <= now()) fail(401, 'enrollmentDenied', 'The enrollment code is invalid, expired, or already used.');
          if (typeof input.deviceName !== 'string' || input.deviceName.length < 1 || input.deviceName.length > 100) fail(400, 'invalidCommand', 'Enter a device name.');
          const deviceId = randomUUID(); const deviceToken = opaque(); const expires = now() + 30 * 86400_000;
          db.prepare('UPDATE enrollment SET used=1 WHERE code_hash=?').run(digest(input.code as string));
          db.prepare('INSERT INTO devices(id,name,token_hash,expires) VALUES (?,?,?,?)').run(deviceId, input.deviceName, digest(deviceToken), expires);
          return { installation, deviceId, deviceToken, expiresAt: new Date(expires).toISOString() };
        });
        return send(201, result);
      }
      authenticate(request);
      const closeUnknown = /^\/v1\/runs\/([A-Za-z0-9_-]+)\/close-unknown$/.exec(path);
      if (closeUnknown && request.method === 'POST') {
        const input = await body(request);
        const run = ownerTransaction(request, deviceId => {
          if (input.recoveryEpoch !== installation.recoveryEpoch) fail(409, 'recoveryEpochChanged', 'Refresh the uncertain run before deciding.');
          if (input.acknowledgeUncertainty !== true) fail(400, 'uncertaintyAcknowledgementRequired', 'Acknowledge that the original outcome remains unknown and will not be retried.');
          const run = getRun(closeUnknown[1]);
          if (run.state === 'closedUnknown') return run;
          if (run.state !== 'outcomeUnknown') fail(409, 'runConflict', 'Only an uncertain outcome can be closed without retry.');
          run.state = 'closedUnknown'; updateRun(run);
          db.prepare("UPDATE outbox SET state='closedUnknown' WHERE run=?").run(run.runId);
          appendEvent(run.conversationId, run.runId, 'run.updated', { run, resolution: { decision: 'closeWithoutRetry', deviceId, decidedAt: timestamp() } }); return run;
        });
        send(200, { run }); if (options.autoRun) setImmediate(() => { void pump(); }); return;
      }
      const cancelRoute = /^\/v1\/runs\/([A-Za-z0-9_-]+)\/cancel$/.exec(path);
      if (cancelRoute && request.method === 'POST') {
        const input = await body(request);
        const run = ownerTransaction(request, deviceId => {
          if (input.recoveryEpoch !== installation.recoveryEpoch) fail(409, 'recoveryEpochChanged', 'Refresh this run before requesting cancellation.');
          const run = getRun(cancelRoute[1]);
          if (['completed', 'canceled', 'closedUnknown', 'cancelRequested'].includes(run.state)) return run;
          if (run.state === 'outcomeUnknown') fail(409, 'outcomeUnknown', 'Execution cannot be confirmed stopped. Review the uncertain outcome first.');
          const outbox = db.prepare('SELECT state FROM outbox WHERE run=?').get(run.runId)!;
          const stopped = ['queued', 'resume', 'waitingForApproval'].includes(String(outbox.state));
          run.state = stopped ? 'canceled' : 'cancelRequested'; updateRun(run);
          if (stopped) db.prepare("UPDATE outbox SET state='canceled' WHERE run=?").run(run.runId);
          const pending = db.prepare("SELECT data FROM approvals WHERE run=? AND state='pending'").get(run.runId);
          if (pending) {
            const approval: Approval = JSON.parse(pending.data as string); approval.state = 'canceled'; approval.decidedAt = timestamp(); approval.decidedByDeviceId = deviceId; updateApproval(approval);
            appendEvent(run.conversationId, approval.approvalId, 'approval.updated', { approval });
          }
          appendEvent(run.conversationId, run.runId, 'run.updated', { run }); return run;
        });
        send(200, { run });
        if (run.state === 'cancelRequested' && activeRun?.runId === run.runId) activeRun.controller.abort();
        if (options.autoRun) setImmediate(() => { void pump(); }); return;
      }
      const approvalRoute = /^\/v1\/approvals\/([A-Za-z0-9_-]+)\/resolve$/.exec(path);
      if (approvalRoute && request.method === 'POST') {
        const input = await body(request);
        const result = ownerTransaction(request, deviceId => {
          const approval = getApproval(approvalRoute[1]); const run = getRun(approval.runId);
          if (input.recoveryEpoch !== installation.recoveryEpoch || input.runId !== approval.runId || input.attemptId !== approval.attemptId || input.bindingRevisionId !== approval.bindingRevisionId || input.actionDigest !== approval.actionDigest || JSON.stringify(input.resourceScope) !== JSON.stringify(approval.resourceScope)) fail(409, 'approvalConflict', 'This decision does not match the exact pending action.');
          if (input.decision !== 'approve' && input.decision !== 'reject') fail(400, 'invalidCommand', 'Choose approve or reject.');
          if (approval.state !== 'pending') return { approval, run, expired: approval.state === 'expired' };
          if (run.state !== 'waitingForApproval' || run.attemptId !== approval.attemptId) fail(409, 'approvalConflict', 'This attempt can no longer accept that decision.');
          const expired = Date.parse(approval.expiresAt) <= now();
          approval.state = expired ? 'expired' : input.decision === 'approve' ? 'approved' : 'rejected';
          approval.decidedByDeviceId = expired ? null : deviceId; approval.decidedAt = timestamp(); updateApproval(approval);
          run.state = approval.state === 'approved' ? 'queued' : 'canceled'; updateRun(run);
          db.prepare('UPDATE outbox SET state=? WHERE run=?').run(approval.state === 'approved' ? 'resume' : 'canceled', run.runId);
          appendEvent(run.conversationId, approval.approvalId, 'approval.updated', { approval });
          appendEvent(run.conversationId, run.runId, 'run.updated', { run });
          return { approval, run, expired };
        });
        if (options.autoRun) setImmediate(() => { void pump(); });
        if (result.expired) fail(409, 'approvalExpired', 'The action expired without authorizing execution.');
        send(200, { approval: result.approval, run: result.run }); return;
      }
      const revoke = /^\/v1\/devices\/([A-Za-z0-9_-]+)\/revoke$/.exec(path);
      if (revoke && request.method === 'POST') {
        await body(request, true);
        const result = ownerTransaction(request, () => db.prepare('UPDATE devices SET revoked=1 WHERE id=?').run(revoke[1]));
        if (!result.changes) fail(404, 'notFound', 'Device was not found.');
        return send(200, { deviceId: revoke[1], revoked: true });
      }
      const read = /^\/v1\/conversations\/([A-Za-z0-9_-]+)\/read$/.exec(path);
      if (read && request.method === 'POST') {
        const input = await body(request);
        const lastReadMessageSequence = ownerTransaction(request, () => {
          agentFor(read[1]);
          if (input.recoveryEpoch !== installation.recoveryEpoch) fail(409, 'recoveryEpochChanged', 'Refresh the snapshot before marking read.');
          if (!identifier(input.messageId) || !Number.isSafeInteger(input.cursor)) fail(400, 'invalidCommand', 'Use a message ID and observed cursor.');
          const message = db.prepare('SELECT sequence FROM messages WHERE id=? AND conversation=?').get(input.messageId as string, read[1]);
          if (!message) fail(404, 'notFound', 'This message was not found in the Conversation.');
          if (Number(input.cursor) > cursor() || Number(input.cursor) < Number(message.sequence)) fail(409, 'invalidCursor', 'The message must exist at the observed cursor.');
          const previous = Number(db.prepare('SELECT sequence FROM reads WHERE conversation=?').get(read[1])?.sequence ?? 0);
          const next = Math.max(previous, Number(message.sequence));
          db.prepare('INSERT INTO reads VALUES (?,?) ON CONFLICT(conversation) DO UPDATE SET sequence=excluded.sequence').run(read[1], next);
          if (next > previous) appendEvent(read[1], read[1], 'read.updated', { lastReadMessageSequence: next });
          return next;
        });
        return send(200, { lastReadMessageSequence });
      }
      if (path === '/v1/installation' && request.method === 'GET') return send(200, installation);
      if (path === '/v1/agents' && request.method === 'GET') return send(200, { agents: db.prepare('SELECT data FROM agents ORDER BY rowid').all().map(r => JSON.parse(r.data as string)) });
      if (path === '/v1/events' && request.method === 'GET') {
        if (url.searchParams.get('recoveryEpoch') !== installation.recoveryEpoch) fail(409, 'recoveryEpochChanged', 'Refresh your snapshot before replay.');
        const raw = url.searchParams.get('after'); const after = Number(raw);
        if (raw === null || !/^\d+$/.test(raw) || !Number.isSafeInteger(after) || after > cursor()) fail(400, 'invalidCursor', 'Use an existing replay cursor.');
        const events = db.prepare('SELECT data FROM events WHERE sequence>? ORDER BY sequence LIMIT 256').all(after).map(r => JSON.parse(r.data as string));
        return send(200, { installationId: installation.installationId, recoveryEpoch: installation.recoveryEpoch, cursor: events.length ? events.at(-1).sequence : after, events });
      }
      const conversation = /^\/v1\/conversations\/([A-Za-z0-9_-]+)\/(snapshot|messages)$/.exec(path);
      if (conversation?.[2] === 'snapshot' && request.method === 'GET') return send(200, transaction(() => snapshot(conversation[1])));
      if (conversation?.[2] === 'messages' && request.method === 'POST') {
        const result = accept(request, conversation[1], await body(request));
        checkpoint('afterAcceptanceCommit');
        send(result.duplicate ? 200 : 201, result.acceptance);
        if (options.autoRun) setImmediate(() => { void pump(); });
        return;
      }
      const receipt = /^\/v1\/requests\/([A-Za-z0-9_-]+)$/.exec(path);
      if (receipt && request.method === 'GET') {
        const found = db.prepare('SELECT acceptance FROM requests WHERE id=?').get(receipt[1]);
        if (!found) fail(404, 'notFound', 'Request was not found in this recovery epoch.');
        return send(200, JSON.parse(found.acceptance as string));
      }
      fail(404, 'notFound', 'This fixture route is not available.');
    } catch (error) { send(error instanceof APIError ? error.status : 500, { error: { code: error instanceof APIError ? error.code : 'internalError', message: error instanceof APIError ? error.message : 'Fort could not complete this operation.' } }); }
  });
  const service = {
    server, installation, issueEnrollment, dispatchNextFixture, baseURL: '',
    async start() { await new Promise<void>((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', () => { server.off('error', reject); resolve(); }); }); const address = server.address(); service.baseURL = `http://127.0.0.1:${typeof address === 'object' && address ? address.port : 0}`; if (options.autoRun) setImmediate(() => { void pump(); }); return service.baseURL; },
    async close() { if (closed) return; closed = true; activeRun?.controller.abort(); if (dispatching) await dispatching; if (server.listening) await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve())); db.close(); },
  };
  return service;
}
