import { createHash } from 'node:crypto';
import { DatabaseSync } from 'node:sqlite';
import { closeSync, constants, fstatSync, lstatSync, openSync, readFileSync, readSync, realpathSync, type Stats } from 'node:fs';
import { isAbsolute, join, normalize } from 'node:path';
import { parseArgs } from 'node:util';

export type RuntimeMigrationRequest = Readonly<{
  action: 'apply' | 'rollback'; migrationId: string; installationId: string; recoveryEpoch: string;
  deviceId: string; provider: string; hostId: string; fromVersion: string; toVersion: string;
}>;
type Receipt = { state: 'applied' | 'rolledBack' | 'notApplied'; migrationId: string; agents: number; disabledBindings: number };
type Change = { kind: 'agents' | 'agentBindings' | 'disabledBindings'; id: string; before: string | null; after: string };
const columns: Record<string, string[]> = {
  metadata: ['key', 'data'], enrollment: ['hash', 'expires'], devices: ['id', 'hash', 'expires'],
  entities: ['kind', 'id', 'data'], events: ['sequence', 'data'],
  dispatch_outbox: ['position', 'run_id', 'conversation_id', 'state', 'held', 'attempt_id', 'generation'],
};
const maximumBytes = 256 * 1024 * 1024;
const digest = (bytes: string) => createHash('sha256').update(bytes).digest('hex');
function refuse(detail = 'Verify the exact disconnected installation, stopped host and unchanged migration request.'): never {
  throw new Error('Runtime migration refused. ' + detail);
}
function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) refuse();
  return value as Record<string, unknown>;
}
function identifier(value: unknown): string {
  if (typeof value !== 'string' || !/^[A-Za-z0-9_-]{1,128}$/.test(value)) refuse();
  return value;
}
function version(value: unknown): string {
  if (typeof value !== 'string' || !/^[A-Za-z0-9._-]{1,128}$/.test(value)) refuse();
  return value;
}
function parseRequest(value: unknown): RuntimeMigrationRequest {
  const row = object(value);
  if (Object.keys(row).sort().join(',') !== 'action,deviceId,fromVersion,hostId,installationId,migrationId,provider,recoveryEpoch,toVersion'
    || row.action !== 'apply' && row.action !== 'rollback') refuse();
  const request: RuntimeMigrationRequest = { action: row.action, migrationId: identifier(row.migrationId), installationId: identifier(row.installationId),
    recoveryEpoch: identifier(row.recoveryEpoch), deviceId: identifier(row.deviceId), provider: identifier(row.provider),
    hostId: identifier(row.hostId), fromVersion: version(row.fromVersion), toVersion: version(row.toVersion) };
  if (request.fromVersion === request.toVersion) refuse();
  return request;
}
function sameFile(a: Stats, b: Stats) { return a.dev === b.dev && a.ino === b.ino && a.uid === b.uid && a.mode === b.mode; }
function ownedFile(file: Stats) {
  if (!file.isFile() || file.nlink !== 1 || file.uid !== process.getuid?.() || file.mode & 0o077 || file.size > maximumBytes) refuse();
}
function data(row: Record<string, unknown> | undefined): string {
  if (!row || typeof row.data !== 'string') refuse();
  return row.data;
}
function sameRuntime(value: unknown, request: RuntimeMigrationRequest, runtimeVersion: string): boolean {
  const row = object(value);
  return row.provider === request.provider && row.hostId === request.hostId && row.runtimeVersion === runtimeVersion;
}
function logicalDigest(db: DatabaseSync): string {
  const hash = createHash('sha256');
  const add = (row: unknown) => hash.update(JSON.stringify(row) + '\n');
  add(db.prepare('PRAGMA user_version').get());
  for (const row of db.prepare("SELECT type,name,tbl_name,sql FROM sqlite_schema WHERE name NOT LIKE 'sqlite_autoindex_%' AND tbl_name<>'runtime_migrations' ORDER BY type,name").iterate()) add(row);
  for (const [table, names] of Object.entries({ ...columns, sqlite_sequence: ['name', 'seq'] })) {
    add(table);
    for (const row of db.prepare(`SELECT ${names.join(',')} FROM ${table} ORDER BY ${names.join(',')}`).iterate()) add(row);
  }
  return hash.digest('hex');
}
function parseChanges(value: unknown): Change[] {
  if (!Array.isArray(value) || value.length > 5000) refuse();
  const seen = new Set<string>();
  return value.map(value => {
    const row = object(value), kind = row.kind, id = identifier(row.id);
    if (kind !== 'agents' && kind !== 'agentBindings' && kind !== 'disabledBindings' || typeof row.after !== 'string'
      || row.before !== null && typeof row.before !== 'string' || seen.has(kind + '/' + id)) refuse();
    seen.add(kind + '/' + id);
    return { kind, id, before: row.before, after: row.after };
  });
}

export function migrateRuntime(directory: string, input: RuntimeMigrationRequest): Receipt {
  const request = parseRequest(input);
  if (!isAbsolute(directory) || normalize(directory) !== directory || !directory.isWellFormed() || directory.includes('\0') || realpathSync(directory) !== directory) refuse();
  const root = lstatSync(directory), path = join(directory, 'openai-candidate-b.sqlite'), file = lstatSync(path);
  if (!root.isDirectory() || root.uid !== process.getuid?.() || root.mode & 0o077) refuse();
  ownedFile(file);
  const verifyFiles = () => {
    const current = lstatSync(path); ownedFile(current);
    if (!sameFile(root, lstatSync(directory)) || realpathSync(directory) !== directory || !sameFile(file, current)) refuse();
    let bytes = current.size;
    for (const suffix of ['-wal', '-shm', '-journal']) {
      const sidecar = lstatSync(path + suffix, { throwIfNoEntry: false });
      if (sidecar) { ownedFile(sidecar); bytes += sidecar.size; }
    }
    if (bytes > maximumBytes) refuse();
  };
  verifyFiles();
  const fd = openSync(path, constants.O_RDWR | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  let db: DatabaseSync | undefined, transaction = false;
  try {
    if (!sameFile(file, fstatSync(fd))) refuse();
    const header = Buffer.alloc(16);
    if (readSync(fd, header, 0, 16, 0) !== 16 || header.toString('binary') !== 'SQLite format 3\0') refuse();
    db = new DatabaseSync(path);
    db.exec('PRAGMA trusted_schema=OFF; PRAGMA locking_mode=EXCLUSIVE; PRAGMA busy_timeout=0; PRAGMA synchronous=FULL; BEGIN EXCLUSIVE');
    transaction = true; verifyFiles();
    if (db.prepare('PRAGMA integrity_check').get()?.integrity_check !== 'ok') refuse();
    const tables = db.prepare("SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name").all().map(row => row.name);
    const hasJournal = tables.includes('runtime_migrations');
    if (JSON.stringify(tables) !== JSON.stringify([...Object.keys(columns), ...(hasJournal ? ['runtime_migrations'] : [])].sort())
      || db.prepare("SELECT 1 FROM sqlite_schema WHERE type IN ('trigger','view') OR upper(sql) LIKE '%VIRTUAL TABLE%'").get()) refuse();
    for (const [table, names] of Object.entries({ ...columns, ...(hasJournal ? { runtime_migrations: ['id', 'data'] } : {}) }))
      if (JSON.stringify(db.prepare(`PRAGMA table_info(${table})`).all().map(row => row.name)) !== JSON.stringify(names)) refuse();
    const database = db;
    const saved = (key: string) => object(JSON.parse(data(database.prepare('SELECT data FROM metadata WHERE key=?').get(key))));
    const installation = saved('installation'), control = saved('ownerControl'), policy = saved('ownerPairingPolicy');
    const setupRow = db.prepare("SELECT data FROM metadata WHERE key='ownerSetup'").get();
    if (setupRow) {
      const setup = object(JSON.parse(data(setupRow)));
      if (setup.version !== 1 || setup.state !== 'completed' && setup.state !== 'canceled' || setup.deviceId !== request.deviceId) refuse('Finish the pending owner setup or cleanup before migrating.');
    }
    if (installation.installationId !== request.installationId || installation.recoveryEpoch !== request.recoveryEpoch
      || installation.contractVersion !== '1.0' || !['local', 'openai'].includes(String(installation.mode))
      || !['Fort', 'Fort Prototype'].includes(String(installation.serviceName)) || control.deviceId !== request.deviceId || control.intent !== 'disconnect'
      || !(policy.expiresAfterDays === null || Number.isInteger(policy.expiresAfterDays) && Number(policy.expiresAfterDays) >= 1 && Number(policy.expiresAfterDays) <= 365)
      || db.prepare("SELECT data FROM metadata WHERE key='outboxVersion'").get()?.data !== '1'
      || db.prepare('SELECT 1 FROM devices').get() || db.prepare('SELECT 1 FROM enrollment').get()) refuse();
    if (db.prepare('SELECT 1 FROM dispatch_outbox').get() || db.prepare("SELECT 1 FROM entities WHERE kind='runs' AND (json_extract(data,'$.state') IS NULL OR json_extract(data,'$.state') NOT IN ('completed','failed','canceled','closedUnknown'))").get()) refuse('Resolve pending or unknown work before migrating. Nothing was changed.');
    if (db.prepare("SELECT 1 FROM entities WHERE kind='schedules' AND json_extract(data,'$.state') IS NOT 'paused'").get()) refuse('Pause schedules before migrating. Nothing was changed.');
    const identityData = data(db.prepare("SELECT data FROM metadata WHERE key='runtimeIdentity'").get());
    const identity = object(JSON.parse(identityData));
    const { action, ...operation } = request;
    const operationJSON = JSON.stringify(operation);
    const priorRow = hasJournal ? db.prepare('SELECT data FROM runtime_migrations WHERE id=?').get(request.migrationId) : undefined;
    let receipt: Receipt;
    if (priorRow) {
      const journal = object(JSON.parse(data(priorRow))), changes = parseChanges(journal.changes);
      if (journal.format !== 1 || journal.operation !== operationJSON || typeof journal.beforeRuntime !== 'string'
        || typeof journal.beforeDigest !== 'string' || !/^[a-f0-9]{64}$/.test(journal.beforeDigest)
        || typeof journal.afterDigest !== 'string' || !/^[a-f0-9]{64}$/.test(journal.afterDigest)
        || !sameRuntime(identity, request, request.toVersion)) refuse();
      receipt = { state: 'applied', migrationId: request.migrationId, agents: changes.filter(c => c.kind === 'agents').length, disabledBindings: changes.filter(c => c.kind === 'disabledBindings').length };
      if (action === 'rollback') {
        if (logicalDigest(db) !== journal.afterDigest) refuse('The ledger changed after migration. Rollback would discard later changes.');
        for (const change of changes) {
          if (data(db.prepare('SELECT data FROM entities WHERE kind=? AND id=?').get(change.kind, change.id)) !== change.after) refuse();
          if (change.before === null) db.prepare('DELETE FROM entities WHERE kind=? AND id=?').run(change.kind, change.id);
          else db.prepare('UPDATE entities SET data=? WHERE kind=? AND id=?').run(change.before, change.kind, change.id);
        }
        if (!sameRuntime(JSON.parse(journal.beforeRuntime), request, request.fromVersion)) refuse();
        db.prepare("UPDATE metadata SET data=? WHERE key='runtimeIdentity'").run(journal.beforeRuntime);
        db.prepare('DELETE FROM runtime_migrations WHERE id=?').run(request.migrationId);
        if (!db.prepare('SELECT 1 FROM runtime_migrations').get()) db.exec('DROP TABLE runtime_migrations');
        if (logicalDigest(db) !== journal.beforeDigest) refuse();
        receipt.state = 'rolledBack';
      }
    } else if (action === 'rollback') {
      if (!sameRuntime(identity, request, request.fromVersion)) refuse();
      receipt = { state: 'notApplied', migrationId: request.migrationId, agents: 0, disabledBindings: 0 };
    } else {
      if (!sameRuntime(identity, request, request.fromVersion)) refuse();
      const beforeDigest = logicalDigest(db), changes: Change[] = [];
      const agents = db.prepare("SELECT id,data FROM entities WHERE kind='agents' ORDER BY id LIMIT 1001").all();
      const bindings = db.prepare("SELECT id,data FROM entities WHERE kind='agentBindings' ORDER BY id LIMIT 3001").all();
      if (agents.length > 1000 || bindings.length > 3000) refuse('The migration exceeds its reviewed binding limit.');
      for (const row of bindings) {
        const agent = object(JSON.parse(data(row))), binding = object(agent.binding), bindingId = identifier(binding.bindingRevisionId);
        if (row.id !== bindingId || binding.provider !== request.provider || binding.hostId !== request.hostId) refuse();
        if (binding.runtimeVersion !== request.fromVersion) continue;
        if (!db.prepare("SELECT 1 FROM entities WHERE kind='disabledBindings' AND id=?").get(bindingId)) changes.push({ kind: 'disabledBindings', id: bindingId, before: null,
          after: JSON.stringify({ agentId: identifier(agent.agentId), bindingRevisionId: bindingId, disabledAt: new Date().toISOString() }) });
      }
      for (const row of agents) {
        const before = data(row), agent = object(JSON.parse(before)), binding = object(agent.binding), agentId = identifier(agent.agentId);
        if (row.id !== agentId || !sameRuntime(binding, request, request.fromVersion) || typeof binding.enabled !== 'boolean'
          || typeof binding.selectedModel !== 'string' || !binding.selectedModel || binding.selectedModel.length > 128
          || agent.readiness !== 'requiresLiveReview' || typeof agent.name !== 'string' || typeof agent.instructions !== 'string') refuse();
        identifier(agent.homeConversationId);
        const original = bindings.find(row => row.id === binding.bindingRevisionId);
        if (!original || !sameRuntime(object(JSON.parse(data(original))).binding, request, request.fromVersion)) refuse();
        const bindingRevisionId = 'upgrade_' + digest(request.migrationId + '\0' + agentId);
        if (db.prepare("SELECT 1 FROM entities WHERE kind='agentBindings' AND id=?").get(bindingRevisionId)) refuse();
        const after = JSON.stringify({ ...agent, binding: { ...binding, bindingRevisionId, runtimeVersion: request.toVersion } });
        changes.push({ kind: 'agentBindings', id: bindingRevisionId, before: null, after }, { kind: 'agents', id: agentId, before, after });
      }
      for (const change of changes) {
        if (change.before === null) db.prepare('INSERT INTO entities(kind,id,data) VALUES (?,?,?)').run(change.kind, change.id, change.after);
        else db.prepare('UPDATE entities SET data=? WHERE kind=? AND id=?').run(change.after, change.kind, change.id);
      }
      db.prepare("UPDATE metadata SET data=? WHERE key='runtimeIdentity'").run(JSON.stringify({ provider: request.provider, runtimeVersion: request.toVersion, hostId: request.hostId }));
      const afterDigest = logicalDigest(db);
      db.exec('CREATE TABLE IF NOT EXISTS runtime_migrations (id TEXT PRIMARY KEY, data TEXT NOT NULL)');
      db.prepare('INSERT INTO runtime_migrations(id,data) VALUES (?,?)').run(request.migrationId, JSON.stringify({ format: 1, operation: operationJSON, beforeRuntime: identityData, beforeDigest, afterDigest, changes }));
      receipt = { state: 'applied', migrationId: request.migrationId, agents: agents.length, disabledBindings: changes.filter(c => c.kind === 'disabledBindings').length };
    }
    verifyFiles(); db.exec('COMMIT'); transaction = false;
    return receipt;
  } finally {
    try { if (transaction) db?.exec('ROLLBACK'); }
    finally { try { db?.close(); } finally { closeSync(fd); } }
  }
}

if (import.meta.main) {
  try {
    if (process.version !== 'v26.5.0') refuse('Use the reviewed Node 26.5.0 runtime.');
    const { values } = parseArgs({ options: { directory: { type: 'string' }, request: { type: 'string' } }, strict: true, allowPositionals: false });
    if (!values.directory || !values.request) refuse('Select the exact directory and request file.');
    const file = lstatSync(values.request); ownedFile(file); if (file.size > 4096) refuse();
    const request: unknown = JSON.parse(readFileSync(values.request, 'utf8'));
    process.stdout.write(JSON.stringify(migrateRuntime(values.directory, parseRequest(request))) + '\n');
  } catch { process.stderr.write('Runtime migration or rollback was not confirmed. Preserve all files and inspect the exact request and ledger state.\n'); process.exitCode = 1; }
}
