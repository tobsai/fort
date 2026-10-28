import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { parseArgs } from 'node:util';
import { setTimeout as delay } from 'node:timers/promises';
import { DatabaseSync } from 'node:sqlite';
import { migrateLedger } from '../service/migrate-ledger.ts';
import { importLedger, inspectLedgerSnapshot } from '../service/import-ledger.ts';
import { readHubCredential } from './hub-credential.ts';

// Sends this Mac's ledger to the hub it is enrolled with (Spec 063, step 4). Take a copy of the ledger
// while Fort is quit, then run this on the Mac. It never prints message text, only counts.
//   node runner/upload-ledger.ts --ledger <copied openai-candidate-b.sqlite> --state <helper state dir> [--rehearse]
// --rehearse applies the snapshot to an empty local ledger instead and sends nothing.
type Fetch = (url: string, init?: RequestInit) => Promise<Response>;
export type UploadResult = { state: string; entities?: Record<string, number>; rebound?: number; message?: string; rehearsal?: Record<string, number>; bindings?: Record<string, number> };

export async function uploadLedger(options: { ledger: string; stateDirectory: string; rehearse?: boolean; fetch?: Fetch; waitMs?: number }): Promise<UploadResult> {
  const credential = await readHubCredential(options.stateDirectory);
  if (!credential) throw new Error('This Mac is not enrolled with a hub');
  const work = await mkdtemp(join(tmpdir(), 'fort-ledger-upload-'));
  try {
    await migrateLedger({ source: options.ledger, destination: join(work, 'snapshot') });
    const snapshot = join(work, 'snapshot', 'openai-candidate-b.sqlite');
    const inspected = inspectLedgerSnapshot(snapshot);
    if (options.rehearse) {
      // An empty Fort ledger, as a fresh hub has, receives the snapshot exactly as the hub would.
      const ledger = join(work, 'hub.sqlite'), empty = new DatabaseSync(ledger);
      empty.exec("CREATE TABLE metadata (key TEXT PRIMARY KEY, data TEXT NOT NULL); CREATE TABLE entities (kind TEXT NOT NULL, id TEXT NOT NULL, data TEXT NOT NULL, PRIMARY KEY(kind,id));");
      empty.close();
      const report = importLedger({ source: snapshot, ledger, hostId: credential.runnerId });
      const check = new DatabaseSync(ledger, { readOnly: true });
      try {
        const rehearsal = Object.fromEntries(check.prepare('SELECT kind, count(*) AS n FROM entities GROUP BY kind ORDER BY kind').all().map(row => [String(row.kind), Number(row.n)]));
        // Each agent runs only where the hub serves its exact provider and version.
        const bindings = Object.fromEntries(check.prepare("SELECT json_extract(data,'$.binding.provider') || ' ' || json_extract(data,'$.binding.runtimeVersion') AS runtime, count(*) AS n FROM entities WHERE kind='agents' GROUP BY 1 ORDER BY 1").all().map(row => [String(row.runtime), Number(row.n)]));
        return { state: 'rehearsed', entities: report.entities, rebound: report.rebound, rehearsal, bindings };
      } finally { check.close(); }
    }
    const send = options.fetch ?? fetch, url = credential.hubURL + '/v1/runners/import', authorization = 'Bearer ' + credential.token;
    const staged = await send(url, { method: 'POST', redirect: 'error', headers: { authorization, 'content-type': 'application/vnd.sqlite3' }, body: new Blob([new Uint8Array(await readFile(snapshot))]) });
    const reply = await staged.json().catch(() => ({})) as { error?: { message?: string } };
    if (staged.status !== 202) return { state: 'refused', message: reply.error?.message ?? `The hub answered ${staged.status}.`, entities: inspected.entities };
    // The hub restarts to apply it; ask until it says how it went.
    const until = Date.now() + (options.waitMs ?? 60_000);
    while (Date.now() < until) {
      await delay(1000);
      const status = await send(url, { headers: { authorization } }).then(response => response.ok ? response.json() as Promise<UploadResult> : undefined, () => undefined);
      if (status && status.state !== 'staged') return status;
    }
    return { state: 'staged', message: 'The hub has the snapshot but has not reported applying it yet.', entities: inspected.entities };
  } finally { await rm(work, { recursive: true, force: true }); }
}

if (import.meta.main) {
  const { values } = parseArgs({ options: { ledger: { type: 'string' }, state: { type: 'string' }, rehearse: { type: 'boolean' } }, strict: true });
  if (!values.ledger || !values.state) throw new Error('Use --ledger <copied ledger> --state <helper state directory>');
  const result = await uploadLedger({ ledger: values.ledger, stateDirectory: values.state, rehearse: values.rehearse === true });
  process.stdout.write(JSON.stringify(result) + "\n");
  if (result.state === 'refused') process.exitCode = 1;
}
