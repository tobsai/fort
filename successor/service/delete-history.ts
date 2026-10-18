import { randomUUID } from 'node:crypto';
import { closeSync, constants, fstatSync, lstatSync, openSync, readSync, realpathSync, type Stats } from 'node:fs';
import { isAbsolute, join } from 'node:path';
import { DatabaseSync } from 'node:sqlite';

export type HistoryDeletion = Readonly<{
  directory: string;
  installationId: string;
  recoveryEpoch: string;
  deviceId: string;
  deletionId: string;
}>;

type DeletionReceipt = Omit<HistoryDeletion, 'directory'> & { version: 1; nextRecoveryEpoch: string; deletedRecords: number };
const history = ['messages', 'runs', 'requests', 'contexts', 'reads', 'routineApprovals', 'requestCancellations', 'scheduleOccurrences'];
const retained = ['agents', 'agentBindings', 'disabledBindings', 'conversations', 'conversationMemory', 'skills', 'skillRevisions', 'routines', 'routineRevisions', 'schedules', 'scheduleRevisions'];
const metadata = ['installation', 'runtimeIdentity', 'outboxVersion', 'ownerPairingPolicy', 'ownerControl', 'ownerSetup', 'historyDeletion'];
const columns: Record<string, string[]> = {
  metadata: ['key', 'data'], enrollment: ['hash', 'expires'], devices: ['id', 'hash', 'expires'],
  entities: ['kind', 'id', 'data'], events: ['sequence', 'data'],
  dispatch_outbox: ['position', 'run_id', 'conversation_id', 'state', 'held', 'attempt_id', 'generation'],
};
const maximumBytes = 256 * 1024 * 1024;
function refuse(): never { throw new Error('History deletion refused: verify the exact disconnected installation and stopped host.'); }
function identifier(value: unknown): string {
  if (typeof value !== 'string' || !/^[A-Za-z0-9_-]{1,128}$/.test(value)) refuse();
  return value;
}
function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) refuse();
  return value as Record<string, unknown>;
}
function sameFile(a: Stats, b: Stats) { return a.dev === b.dev && a.ino === b.ino && a.uid === b.uid && a.mode === b.mode; }
function ownedFile(value: Stats) {
  if (!value.isFile() || value.nlink !== 1 || value.uid !== process.getuid?.() || (value.mode & 0o077) !== 0 || value.size > maximumBytes) refuse();
}

/** Offline SQLite maintenance only. The native caller must separately confirm removal authority and lifecycle absence. */
export function deleteOwnerHistory(request: HistoryDeletion): { recoveryEpoch: string; deletedRecords: number } {
  const { directory } = request;
  for (const key of ['installationId', 'recoveryEpoch', 'deviceId', 'deletionId'] as const) identifier(request[key]);
  if (!isAbsolute(directory) || realpathSync(directory) !== directory) refuse();
  const root = lstatSync(directory), path = join(directory, 'openai-candidate-b.sqlite'), file = lstatSync(path);
  if (!root.isDirectory() || root.uid !== process.getuid?.() || (root.mode & 0o077) !== 0) refuse();
  ownedFile(file);
  const verifyFiles = () => {
    const current = lstatSync(path); ownedFile(current);
    if (!sameFile(root, lstatSync(directory)) || realpathSync(directory) !== directory || !sameFile(file, current)) refuse();
    let bytes = current.size;
    for (const suffix of ['-wal', '-shm', '-journal']) {
      try { const sidecar = lstatSync(path + suffix); ownedFile(sidecar); bytes += sidecar.size; }
      catch (error) { if (!(error instanceof Error && 'code' in error && error.code === 'ENOENT')) throw error; }
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
    db = new DatabaseSync(path, { enableForeignKeyConstraints: true });
    db.exec('PRAGMA trusted_schema=OFF; PRAGMA locking_mode=EXCLUSIVE; PRAGMA busy_timeout=0; BEGIN EXCLUSIVE');
    transaction = true; verifyFiles();
    if (db.prepare('PRAGMA integrity_check').get()?.integrity_check !== 'ok') refuse();
    const tables = db.prepare("SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name").all().map(row => row.name);
    if (JSON.stringify(tables) !== JSON.stringify(Object.keys(columns).sort()) || db.prepare("SELECT 1 FROM sqlite_schema WHERE type IN ('trigger','view') OR upper(sql) LIKE '%VIRTUAL TABLE%'").get()) refuse();
    for (const [table, names] of Object.entries(columns)) {
      if (JSON.stringify(db.prepare(`PRAGMA table_info(${table})`).all().map(row => row.name)) !== JSON.stringify(names)) refuse();
    }
    for (const row of db.prepare('SELECT DISTINCT kind FROM entities').iterate()) if (![...history, ...retained].includes(String(row.kind))) refuse();
    for (const row of db.prepare('SELECT key FROM metadata').iterate()) if (!metadata.includes(String(row.key))) refuse();
    const saved = (key: string) => {
      const row = db!.prepare('SELECT data FROM metadata WHERE key=?').get(key);
      return row ? object(JSON.parse(String(row.data))) : undefined;
    };
    const installation = saved('installation'), control = saved('ownerControl'), policy = saved('ownerPairingPolicy'), setup = saved('ownerSetup');
    if (!installation || installation.installationId !== request.installationId || installation.contractVersion !== '1.0'
      || !['local', 'openai'].includes(String(installation.mode)) || !['Fort', 'Fort Prototype'].includes(String(installation.serviceName))
      || control?.deviceId !== request.deviceId || control.intent !== 'disconnect' || !policy
      || !(policy.expiresAfterDays === null || Number.isInteger(policy.expiresAfterDays) && Number(policy.expiresAfterDays) >= 1 && Number(policy.expiresAfterDays) <= 365)
      || !saved('runtimeIdentity') || db.prepare("SELECT data FROM metadata WHERE key='outboxVersion'").get()?.data !== '1'
      || db.prepare('SELECT 1 FROM devices').get() || db.prepare('SELECT 1 FROM enrollment').get()
      || setup && (setup.version !== 1 || !['completed', 'canceled'].includes(String(setup.state)) || setup.deviceId !== request.deviceId)) refuse();
    for (const row of db.prepare("SELECT data FROM entities WHERE kind='schedules'").iterate()) {
      const schedule = object(JSON.parse(String(row.data)));
      if (schedule.state !== 'paused') throw new Error('Pause all schedules before confirming history deletion.');
      if (!Number.isSafeInteger(schedule.checkedThrough) || Number(schedule.checkedThrough) < 0) refuse();
    }
    const previous = saved('historyDeletion');
    if (previous && previous.version !== 1) refuse();
    let receipt: DeletionReceipt;
    if (previous?.deletionId === request.deletionId) {
      if (previous.version !== 1 || previous.installationId !== request.installationId || previous.deviceId !== request.deviceId
        || previous.recoveryEpoch !== request.recoveryEpoch || previous.nextRecoveryEpoch !== installation.recoveryEpoch
        || !Number.isSafeInteger(previous.deletedRecords) || Number(previous.deletedRecords) < 0) refuse();
      receipt = { version: 1, installationId: request.installationId, deviceId: request.deviceId, deletionId: request.deletionId,
        recoveryEpoch: request.recoveryEpoch, nextRecoveryEpoch: identifier(previous.nextRecoveryEpoch), deletedRecords: Number(previous.deletedRecords) };
      if (db.prepare(`SELECT 1 FROM entities WHERE kind IN (${history.map(() => '?').join(',')})`).get(...history)
        || db.prepare('SELECT 1 FROM events').get() || db.prepare('SELECT 1 FROM dispatch_outbox').get()) refuse();
    } else {
      if (installation.recoveryEpoch !== request.recoveryEpoch) refuse();
      db.exec('PRAGMA secure_delete=ON');
      const deletedRecords = Number(db.prepare(`DELETE FROM entities WHERE kind IN (${history.map(() => '?').join(',')})`).run(...history).changes)
        + Number(db.prepare('DELETE FROM events').run().changes) + Number(db.prepare('DELETE FROM dispatch_outbox').run().changes);
      receipt = { version: 1, installationId: request.installationId, deviceId: request.deviceId, deletionId: request.deletionId,
        recoveryEpoch: request.recoveryEpoch, nextRecoveryEpoch: randomUUID(), deletedRecords };
      db.prepare("UPDATE metadata SET data=? WHERE key='installation'").run(JSON.stringify({ ...installation, recoveryEpoch: receipt.nextRecoveryEpoch }));
      db.prepare("INSERT OR REPLACE INTO metadata VALUES ('historyDeletion',?)").run(JSON.stringify(receipt));
    }
    verifyFiles(); db.exec('COMMIT'); transaction = false;
    try {
      const checkpoint = () => { if (db!.prepare('PRAGMA wal_checkpoint(TRUNCATE)').get()?.busy !== 0) throw new Error('Checkpoint busy'); };
      checkpoint(); db.exec('PRAGMA secure_delete=ON; VACUUM'); checkpoint(); verifyFiles();
      if (db.prepare('PRAGMA integrity_check').get()?.integrity_check !== 'ok') throw new Error('Integrity check failed');
    } catch { throw new Error('History records were deleted; SQLite cleanup is unconfirmed. Retry the exact deletion request.'); }
    return { recoveryEpoch: receipt.nextRecoveryEpoch, deletedRecords: receipt.deletedRecords };
  } finally {
    try { if (transaction) db?.exec('ROLLBACK'); }
    finally { try { db?.close(); } finally { closeSync(fd); } }
  }
}
