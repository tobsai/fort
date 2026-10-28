import type { DatabaseSync } from 'node:sqlite';

// Several Macs serving one node (Spec 063, revision 2026-10-28). Toby's Mac mini is the primary and his
// laptop takes the work when the mini is away. Every Mac in the pool serves the same lanes under the
// pool's node ID, so bindings, digests and the executor wire are unchanged: the node is still the
// hostId. The hub picks one Mac for a run, pins the run to it, and never moves a started run.
export type RunnerPool = { node: string; runners: readonly string[] };

const IDENTIFIER = /^[A-Za-z0-9_-]{1,128}$/;
const PIN_DAYS = 30;

// FORT_RUNNER_POOL is a comma-separated list of runner IDs in order of preference: primary first.
export function parseRunnerPool(node: string, value: string): RunnerPool {
  const runners = value.split(',').map(entry => entry.trim()).filter(Boolean);
  if (!IDENTIFIER.test(node)) throw new Error('Use an exact node ID for the runner pool');
  if (!runners.length || runners.length > 8 || runners.some(id => !IDENTIFIER.test(id)) || new Set(runners).size !== runners.length) throw new Error('Set FORT_RUNNER_POOL to distinct runner IDs, primary first');
  return Object.freeze({ node, runners: Object.freeze(runners) });
}

// The first connected Mac in order of preference, or the Mac a run is already pinned to.
// A pinned run waits for its own Mac; it never moves to another one.
export function chooseRunner(pool: RunnerPool, online: (runnerId: string) => boolean, pinned?: string): string | undefined {
  if (pinned !== undefined) return pool.runners.includes(pinned) && online(pinned) ? pinned : undefined;
  return pool.runners.find(online);
}

// Which Mac each run went to. Kept beside runner enrollment so a hub restart keeps a routine's later
// steps on the Mac that ran its first ones.
export function runPins(db: DatabaseSync, options: { now?: () => number } = {}) {
  const now = options.now ?? Date.now;
  db.exec('CREATE TABLE IF NOT EXISTS run_pins (run_id TEXT PRIMARY KEY, runner_id TEXT NOT NULL, pinned INTEGER NOT NULL)');
  db.prepare('DELETE FROM run_pins WHERE pinned < ?').run(now() - PIN_DAYS * 86_400_000);
  return {
    get(runId: string): string | undefined {
      const row = db.prepare('SELECT runner_id FROM run_pins WHERE run_id=?').get(runId);
      return typeof row?.runner_id === 'string' ? row.runner_id : undefined;
    },
    // Keeps the first pin; a later call for the same run never re-pins it.
    pin(runId: string, runnerId: string): string {
      db.prepare('INSERT OR IGNORE INTO run_pins VALUES (?,?,?)').run(runId, runnerId, now());
      return this.get(runId)!;
    },
  };
}
export type RunPins = ReturnType<typeof runPins>;
