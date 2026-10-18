import { backup, DatabaseSync } from 'node:sqlite';
import { createHash } from 'node:crypto';
import { lstat, mkdir, open, readFile, rm, stat } from 'node:fs/promises';
import { isAbsolute, join } from 'node:path';
import { isDeepStrictEqual, parseArgs } from 'node:util';

const filename = 'openai-candidate-b.sqlite';
const hash = (value: string | Buffer) => createHash('sha256').update(value).digest('hex');
function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Invalid migration record');
  return value as Record<string, unknown>;
}
function string(value: unknown): string {
  if (typeof value !== 'string' || !value.length) throw new Error('Missing migration field');
  return value;
}
function integer(value: unknown): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 1) throw new Error('Invalid migration sequence');
  return value;
}

export async function migrateLedger(options: { source: string; destination: string }) {
  if (!isAbsolute(options.source) || !isAbsolute(options.destination)) throw new Error('Use absolute source and destination paths');
  if (!(await lstat(options.source)).isFile()) throw new Error('Source must be a regular ledger file');
  const source = new DatabaseSync(options.source, { readOnly: true });
  let destination: DatabaseSync | undefined;
  let ownsDestination = false;
  try {
    await mkdir(options.destination, { mode: 0o700 });
    ownsDestination = true;
    const path = join(options.destination, filename);
    await (await open(path, 'wx', 0o600)).close();
    await backup(source, path);
    const snapshot = await readFile(path);
    destination = new DatabaseSync(path);
    const db = destination;
    if (db.prepare('PRAGMA integrity_check').get()?.integrity_check !== 'ok') throw new Error('Ledger integrity check failed');
    const tables = db.prepare("SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name").all().map(row => row.name);
    if (!isDeepStrictEqual(tables.filter(name => name !== 'dispatch_outbox'), ['devices', 'enrollment', 'entities', 'events', 'metadata']) || db.prepare("SELECT 1 FROM sqlite_schema WHERE type IN ('trigger','view')").get()) throw new Error('Unsupported ledger schema');
    const read = (kind: string, id: unknown) => {
      const row = db.prepare('SELECT data FROM entities WHERE kind=? AND id=?').get(kind, string(id));
      return object(JSON.parse(string(row?.data)));
    };
    const installation = object(JSON.parse(string(db.prepare("SELECT data FROM metadata WHERE key='installation'").get()?.data)));
    if (installation.contractVersion !== '1.0' || !['openai', 'local'].includes(string(installation.mode)) || !['Fort Prototype', 'Fort'].includes(string(installation.serviceName))) throw new Error('Unsupported installation');
    let receiptsCompacted = 0, contextsCompacted = 0;
    db.exec('BEGIN IMMEDIATE');
    for (const row of db.prepare("SELECT id,data FROM entities WHERE kind='requests'").iterate()) {
      const stored = object(JSON.parse(string(row.data)));
      const receipt = object(stored.receipt ?? object(stored.acceptance).receipt);
      const message = read('messages', receipt.messageId), run = read('runs', receipt.runId);
      if (receipt.requestId !== row.id || message.requestId !== row.id || run.requestId !== row.id || message.role !== 'user' || receipt.conversationId !== message.conversationId || receipt.conversationId !== run.conversationId || receipt.bindingRevisionId !== run.bindingRevisionId || receipt.inputDigest !== hash(string(message.text))) throw new Error('Receipt identity or input digest mismatch');
      string(receipt.acceptedAt); string(stored.fingerprint);
      if (stored.acceptance !== undefined) {
        db.prepare("UPDATE entities SET data=? WHERE kind='requests' AND id=?").run(JSON.stringify({ fingerprint: stored.fingerprint, receipt }), string(row.id));
        receiptsCompacted++;
      }
    }
    for (const row of db.prepare("SELECT id,data FROM entities WHERE kind='contexts'").iterate()) {
      const context = object(JSON.parse(string(row.data)));
      if (context.history === undefined) {
        for (const key of ['inputDigest', 'configurationDigest', 'historyDigest']) if (!/^[a-f0-9]{64}$/.test(string(context[key]))) throw new Error('Unsupported compact context');
        continue;
      }
      const run = read('runs', context.runId);
      const receipt = object(object(JSON.parse(string(db.prepare("SELECT data FROM entities WHERE kind='requests' AND id=?").get(string(run.requestId))?.data))).receipt);
      const message = read('messages', receipt.messageId);
      if (context.runId !== row.id || context.requestId !== run.requestId || context.conversationId !== run.conversationId || !isDeepStrictEqual(context.configuration, run.configuration) || !Array.isArray(context.history)) throw new Error('Legacy context identity mismatch');
      const history = context.history.map(object);
      const through = history.reduce((maximum, item) => Math.max(maximum, integer(item.sequence)), integer(message.sequence));
      const messages = db.prepare("SELECT data FROM entities WHERE kind='messages' AND json_extract(data,'$.conversationId')=? AND json_extract(data,'$.sequence')<=? ORDER BY json_extract(data,'$.sequence')").all(string(run.conversationId), through).map(item => object(JSON.parse(string(item.data))));
      const expected = messages.filter(item => item.role === 'user' && integer(item.sequence) < integer(message.sequence)).flatMap(item => [item, ...messages.filter(reply => reply.role === 'assistant' && reply.requestId === item.requestId)]);
      if (!isDeepStrictEqual(history, expected)) throw new Error('Legacy history cannot be reconstructed from canonical messages');
      db.prepare("UPDATE entities SET data=? WHERE kind='contexts' AND id=?").run(JSON.stringify({
        sourceFormat: 'legacy-context-v1', runId: context.runId, inputMessageId: message.messageId,
        inputDigest: hash(string(context.text)), configurationDigest: hash(JSON.stringify(context.configuration)), historyDigest: hash(JSON.stringify(history)),
        historyWindow: { beforeUserSequence: message.sequence, throughEventSequence: through }
      }), string(row.id));
      contextsCompacted++;
    }
    db.exec('COMMIT; VACUUM');
    if (db.prepare('PRAGMA integrity_check').get()?.integrity_check !== 'ok') throw new Error('Migrated ledger integrity check failed');
    db.close(); destination = undefined;
    return { version: 1, sourceSnapshotSHA256: hash(snapshot), sourceSnapshotBytes: snapshot.length, destinationBytes: (await stat(path)).size, receiptsCompacted, contextsCompacted };
  } catch (error) {
    destination?.close(); destination = undefined;
    if (ownsDestination) await rm(options.destination, { recursive: true, force: true });
    throw error;
  } finally { destination?.close(); source.close(); }
}

if (import.meta.main) {
  try {
    const { values } = parseArgs({ options: { source: { type: 'string' }, destination: { type: 'string' } }, strict: true });
    const report = await migrateLedger({ source: string(values.source), destination: string(values.destination) });
    process.stdout.write(JSON.stringify(report) + '\n');
  } catch {
    process.stderr.write('Migration refused or failed. The source is unchanged; an owned incomplete destination is removed. Stop the source service and use a new private destination.\n');
    process.exitCode = 1;
  }
}
