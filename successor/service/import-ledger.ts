import { DatabaseSync } from 'node:sqlite';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { HANDOFF_KINDS } from './hub-handoff.ts';

// Copies a Mac's agents and chat history into the hub's ledger (Spec 063, step 4). The source is a
// snapshot taken with migrate-ledger while the Mac's core was stopped; the Mac keeps its own copy.
// Only entities move: the hub keeps its own installation, owner devices, sign-ins and event log.
// Each agent's current binding is pointed at the hub's runner ID for that Mac, so its turns run there
// on the same provider, version and model. Past runs keep the binding they ran with, unchanged.
// rebound counts the agents now pointed at the hub's runner ID.
export type LedgerImportReport = { version: 1; sourceSHA256: string; entities: Record<string, number>; rebound: number };

const TABLES = ['devices', 'dispatch_outbox', 'enrollment', 'entities', 'events', 'metadata'];
const SETTLED = ['completed', 'failed', 'canceled', 'closedUnknown', 'outcomeUnknown', 'notStarted'];
const BOUND = ['agents', 'agentBindings'];
export const IMPORT_RECORD = 'ledgerImport';

function refuse(detail: string): never { throw new Error('Import refused. ' + detail); }

// Checks the snapshot on its own: an intact Fort ledger with no work still pending.
export function inspectLedgerSnapshot(path: string): { sourceSHA256: string; entities: Record<string, number> } {
  const bytes = readFileSync(path);
  if (bytes.subarray(0, 16).toString('binary') !== 'SQLite format 3\0') refuse('Send a Fort ledger snapshot.');
  const db = new DatabaseSync(path, { readOnly: true });
  try {
    db.exec('PRAGMA trusted_schema=OFF');
    if (db.prepare('PRAGMA integrity_check').get()?.integrity_check !== 'ok') refuse('The snapshot is damaged.');
    const tables = db.prepare("SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name").all().map(row => String(row.name));
    // A runtime-migration journal may sit beside the Fort tables; it belongs to the Mac and is not copied.
    if (JSON.stringify(tables.filter(name => name !== 'runtime_migrations')) !== JSON.stringify(TABLES) || db.prepare("SELECT 1 FROM sqlite_schema WHERE type IN ('trigger','view')").get()) refuse('The snapshot is not a Fort ledger this hub understands.');
    const installation = JSON.parse(String(db.prepare("SELECT data FROM metadata WHERE key='installation'").get()?.data ?? '{}')) as { contractVersion?: unknown };
    if (installation.contractVersion !== '1.0') refuse('The snapshot comes from an unsupported Fort version.');
    if (db.prepare('SELECT 1 FROM dispatch_outbox').get() || db.prepare(`SELECT 1 FROM entities WHERE kind='runs' AND coalesce(json_extract(data,'$.state'),'') NOT IN (${SETTLED.map(() => '?').join(',')})`).get(...SETTLED))
      refuse('Let the Mac finish or cancel its running turns, then take a new snapshot.');
    const entities = Object.fromEntries(db.prepare('SELECT kind, count(*) AS n FROM entities GROUP BY kind ORDER BY kind').all().map(row => [String(row.kind), Number(row.n)]));
    return { sourceSHA256: createHash('sha256').update(bytes).digest('hex'), entities };
  } finally { db.close(); }
}

// Applies the snapshot to the hub's ledger file, which must not be open elsewhere. All or nothing.
export function importLedger(options: { source: string; ledger: string; hostId: string }): LedgerImportReport {
  if (!/^[A-Za-z0-9_-]{1,128}$/.test(options.hostId)) refuse('Use the hub’s runner ID.');
  const { sourceSHA256, entities } = inspectLedgerSnapshot(options.source);
  const db = new DatabaseSync(options.ledger);
  try {
    db.exec('PRAGMA trusted_schema=OFF; PRAGMA busy_timeout=0');
    db.prepare('ATTACH DATABASE ? AS source').run(options.source);
    db.exec('BEGIN IMMEDIATE');
    try {
      const prior = db.prepare('SELECT data FROM metadata WHERE key=?').get(IMPORT_RECORD);
      if (prior) {
        const report = JSON.parse(String(prior.data)) as LedgerImportReport;
        if (report.sourceSHA256 !== sourceSHA256) refuse('This hub already holds an imported ledger.');
        db.exec('ROLLBACK');
        return report;
      }
      // Records this Mac already handed to the hub (its Agents' definitions) stay as the hub has them; history fills in around them.
      const source = (JSON.parse(String(db.prepare("SELECT data FROM source.metadata WHERE key='installation'").get()?.data ?? '{}')) as { installationId?: unknown }).installationId;
      const handedOff = typeof source === 'string' && db.prepare('SELECT 1 FROM main.metadata WHERE key=?').get('agentHandoff:' + source) !== undefined;
      const kinds = handedOff ? HANDOFF_KINDS.map(kind => `'${kind}'`).join(',') : "''";
      if (db.prepare(`SELECT 1 FROM main.entities m JOIN source.entities s USING (kind, id) WHERE m.kind NOT IN (${kinds})`).get()) refuse('The hub already has some of these records.');
      if (db.prepare("SELECT 1 FROM main.entities WHERE kind='weeks'").get() && db.prepare("SELECT 1 FROM source.entities WHERE kind='weeks'").get()) refuse('Both ledgers have weeks; close the hub’s weeks first.');
      let rebound = 0;
      const insert = db.prepare('INSERT INTO main.entities(kind, id, data) VALUES (?, ?, ?) ON CONFLICT(kind, id) DO NOTHING');
      for (const row of db.prepare('SELECT kind, id, data FROM source.entities ORDER BY rowid').iterate()) {
        let data = String(row.data), moved = false;
        if (BOUND.includes(String(row.kind))) {
          const value = JSON.parse(data) as { binding?: { hostId?: unknown } };
          if (!value.binding || typeof value.binding.hostId !== 'string') refuse('An agent in the snapshot has no binding.');
          if (value.binding.hostId !== options.hostId) { value.binding.hostId = options.hostId; if (row.kind === 'agents') moved = true; }
          data = JSON.stringify(value);
        }
        if (insert.run(String(row.kind), String(row.id), data).changes && moved) rebound++;
      }
      const report: LedgerImportReport = { version: 1, sourceSHA256, entities, rebound };
      db.prepare('INSERT INTO main.metadata(key, data) VALUES (?, ?)').run(IMPORT_RECORD, JSON.stringify(report));
      db.exec('COMMIT');
      if (db.prepare('PRAGMA integrity_check').get()?.integrity_check !== 'ok') throw new Error('The hub ledger failed its integrity check after import');
      return report;
    } catch (error) {
      if (db.isTransaction) db.exec('ROLLBACK');
      throw error;
    }
  } finally { db.close(); }
}
