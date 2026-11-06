// Fort Notes (Spec 068): the built-in memory module. Short notes an Agent writes for itself, kept on the host that runs
// the Agent (the hub, by default) in one SQLite file of its own, so removing the module is deleting one file.
// Recall is plain word overlap, ranked deterministically: no embeddings, no paid service, no network.
import { DatabaseSync } from 'node:sqlite';
import { chmodSync, lstatSync, mkdirSync } from 'node:fs';
import { isAbsolute, join } from 'node:path';
import { randomUUID } from 'node:crypto';
import { MEMORY_LIMITS, memoryText, type MemoryEntry, type MemoryKind, type MemoryProvider } from '../../contracts/memory.ts';

const STOP = new Set('the and for are but not you your with this that have has had was were will would can could should from they them their there what when where which who whom why how all any each few more most other some such only own same than too very just into over also about been being does did doing its our ours out off once here then these those him his her hers she himself herself myself yourself i me my we us it is am be do of to in on at by or an a as if so no up'.split(' '));
// Lower-case word stems: "Planning", "planned" and "plans" all become "plan".
export function memoryTerms(text: string): Set<string> {
  const terms = new Set<string>();
  for (const word of text.toLowerCase().normalize('NFKC').match(/[\p{L}\p{N}]+/gu) ?? []) {
    if (word.length < 3 || STOP.has(word)) continue;
    terms.add(word.replace(/(?:ing|ed|s)$/u, '').replace(/(.)\1$/u, '$1').slice(0, 12) || word);
  }
  return terms;
}

type Row = { entry_id: string; agent_id: string; kind: string; text: string; created_at: string; conversation_id: string | null; message_id: string | null; seq: number };
const entry = (row: Row): MemoryEntry => ({ entryId: row.entry_id, agentId: row.agent_id, kind: row.kind === 'event' ? 'event' : 'fact', text: row.text, createdAt: row.created_at,
  ...(row.conversation_id ? { source: { conversationId: row.conversation_id, ...(row.message_id ? { messageId: row.message_id } : {}) } } : {}) });

export function createNotesMemory(options: { directory: string; now?: () => number; maxEntriesPerAgent?: number }): MemoryProvider {
  if (!isAbsolute(options.directory)) throw new Error('Use an explicit absolute memory directory');
  mkdirSync(options.directory, { recursive: true, mode: 0o700 });
  if (lstatSync(options.directory).isSymbolicLink()) throw new Error('Memory directory cannot be a symlink');
  const path = join(options.directory, 'fort-notes.sqlite');
  try { if (lstatSync(path).isSymbolicLink()) throw new Error('Memory store cannot be a symlink'); }
  catch (error) { if (!(error instanceof Error && 'code' in error && error.code === 'ENOENT')) throw error; }
  const db = new DatabaseSync(path);
  chmodSync(path, 0o600);
  db.exec(`PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL;
    CREATE TABLE IF NOT EXISTS entries (seq INTEGER PRIMARY KEY AUTOINCREMENT, entry_id TEXT UNIQUE NOT NULL, agent_id TEXT NOT NULL,
      kind TEXT NOT NULL CHECK(kind IN ('fact','event')), text TEXT NOT NULL, created_at TEXT NOT NULL, conversation_id TEXT, message_id TEXT);
    CREATE INDEX IF NOT EXISTS entries_agent ON entries(agent_id, seq);`);
  const now = options.now ?? Date.now, cap = options.maxEntriesPerAgent ?? 1000;
  let closed = false;
  const open = () => { if (closed) throw new Error('Fort Notes is closed'); };
  const all = (agentId: string) => db.prepare('SELECT * FROM entries WHERE agent_id=? ORDER BY seq').all(agentId) as unknown as Row[];
  const count = (agentId: string) => Number((db.prepare('SELECT count(*) AS n FROM entries WHERE agent_id=?').get(agentId) as { n: number }).n);
  const insert = (agentId: string, kind: MemoryKind, text: string, createdAt: string, source?: { conversationId: string; messageId?: string }) => {
    const entryId = randomUUID();
    db.prepare('INSERT INTO entries(entry_id,agent_id,kind,text,created_at,conversation_id,message_id) VALUES (?,?,?,?,?,?,?)').run(entryId, agentId, kind, text, createdAt, source?.conversationId ?? null, source?.messageId ?? null);
    return entryId;
  };
  // Over the cap, the oldest events go first, then the oldest facts.
  const trim = (agentId: string) => {
    let over = count(agentId) - cap;
    if (over <= 0) return;
    for (const kind of ['event', 'fact']) {
      if (over <= 0) break;
      const removed = db.prepare('DELETE FROM entries WHERE seq IN (SELECT seq FROM entries WHERE agent_id=? AND kind=? ORDER BY seq LIMIT ?)').run(agentId, kind, over);
      over -= Number(removed.changes);
    }
  };
  const tx = <T>(operation: () => T): T => { db.exec('BEGIN IMMEDIATE'); try { const result = operation(); db.exec('COMMIT'); return result; } catch (error) { db.exec('ROLLBACK'); throw error; } };
  const fits = (entries: Row[], limit: number, budget: number) => {
    const out: MemoryEntry[] = []; let bytes = 0;
    for (const row of entries) { const size = Buffer.byteLength(row.text); if (out.length >= limit || bytes + size > budget) continue; bytes += size; out.push(entry(row)); }
    return out;
  };

  const notes: MemoryProvider = {
    protocolVersion: 1 as const, id: 'fort-notes', version: '1', name: 'Fort Notes',
    summary: 'Short notes the agent writes for itself, recalled by matching words. Kept on your hub.',
    storage: 'One file on the host that runs the agent: memory/fort-notes.sqlite.',
    capabilities: ['remember', 'recall', 'search', 'forget', 'export', 'import'],
    async recall({ agentId, query, limit, budgetBytes }) {
      open();
      const terms = memoryTerms(query), rows = all(agentId), newest = rows.length ? rows[rows.length - 1]!.seq : 0;
      const scored = rows.map(row => {
        const own = memoryTerms(row.text); let hits = 0;
        for (const term of terms) if (own.has(term)) hits++;
        return { row, hits, score: hits ? hits + (row.kind === 'fact' ? 0.5 : 0) + (row.seq / Math.max(1, newest)) * 0.25 : 0 };
      });
      const matched = scored.filter(item => item.hits).sort((a, b) => b.score - a.score || b.row.seq - a.row.seq).map(item => item.row);
      // With room left, the newest facts ride along: they are what the Agent knows about the owner.
      const standing = rows.filter(row => row.kind === 'fact' && !matched.includes(row)).reverse().slice(0, 4);
      return fits([...matched, ...standing], Math.min(limit, MEMORY_LIMITS.recallEntries), Math.min(budgetBytes, MEMORY_LIMITS.recallBytes));
    },
    async remember({ agentId, writes }) {
      open();
      const created = new Date(now()).toISOString();
      return tx(() => {
        const existing = new Set(all(agentId).map(row => row.text.toLowerCase()));
        const ids: string[] = [];
        for (const write of writes.slice(0, MEMORY_LIMITS.writesPerTurn)) {
          const text = memoryText(write.text);
          if (!text || existing.has(text.toLowerCase())) continue;
          existing.add(text.toLowerCase());
          ids.push(insert(agentId, write.kind === 'event' ? 'event' : 'fact', text, created, write.source));
        }
        trim(agentId);
        return ids.flatMap(entryId => (db.prepare('SELECT * FROM entries WHERE entry_id=?').all(entryId) as unknown as Row[]).map(entry));
      });
    },
    async forget({ agentId, entryIds }) {
      open();
      return tx(() => entryIds.reduce((sum, entryId) => sum + Number(db.prepare('DELETE FROM entries WHERE agent_id=? AND entry_id=?').run(agentId, entryId).changes), 0));
    },
    async list({ agentId, limit, query }) {
      open();
      let rows = all(agentId).reverse();
      if (query?.trim()) { const terms = memoryTerms(query); rows = rows.filter(row => { const own = memoryTerms(row.text); return [...terms].some(term => own.has(term)); }); }
      return { entries: rows.slice(0, Math.min(limit, MEMORY_LIMITS.listEntries)).map(entry), total: count(agentId) };
    },
    async export({ agentId }) { open(); return all(agentId).map(entry); },
    async import({ agentId, entries }) {
      open();
      return tx(() => {
        const existing = new Set(all(agentId).map(row => row.text.toLowerCase())); let added = 0;
        for (const item of entries) {
          const text = memoryText(item.text);
          if (!text || existing.has(text.toLowerCase())) continue;
          existing.add(text.toLowerCase());
          insert(agentId, item.kind === 'event' ? 'event' : 'fact', text, typeof item.createdAt === 'string' && item.createdAt.length <= 64 ? item.createdAt : new Date(now()).toISOString(), item.source); added++;
        }
        trim(agentId);
        return added;
      });
    },
    close() { if (!closed) { closed = true; db.close(); } },
  };
  return Object.freeze(notes);
}
