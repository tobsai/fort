import type { DatabaseSync } from 'node:sqlite';
import { createHash } from 'node:crypto';

// Runner enrollment for the cloud hub (Spec 063). The owner prepares a one-time invitation; a Mac
// joins with it and its own private token. The hub keeps only digests. Revoking removes the token
// at once and tells the hub to drop that Mac's connection.
export class RunnerEnrollmentError extends Error {
  readonly status: number; readonly code: string;
  constructor(status: number, code: string, message: string) { super(message); this.status = status; this.code = code; }
}
export type RunnerSummary = { runnerId: string; runnerName: string; enrolledAt: string };
type Options = { now?: () => number; revoked?: (runnerId: string) => void };

const INVITATION_MS = 300_000;
const hash = (value: string) => createHash('sha256').update(value).digest('hex');
function fail(status: number, code: string, message: string): never { throw new RunnerEnrollmentError(status, code, message); }
function object(value: unknown, keys: readonly string[]): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).some(key => !keys.includes(key))) fail(400, 'invalidEnrollment', 'Use the exact enrollment fields.');
  return value as Record<string, unknown>;
}
function identifier(value: unknown): string {
  if (typeof value !== 'string' || !/^[A-Za-z0-9_-]{1,128}$/.test(value)) fail(400, 'invalidEnrollment', 'Use an exact enrollment identifier.');
  return value;
}
function secret(value: unknown): string {
  if (typeof value !== 'string' || !/^[A-Za-z0-9_-]{43}$/.test(value) || Buffer.from(value, 'base64url').toString('base64url') !== value) fail(400, 'invalidEnrollment', 'Use a newly prepared 32-byte value.');
  return value;
}
function name(value: unknown): string {
  if (typeof value !== 'string' || !value.isWellFormed() || !value.trim() || Buffer.byteLength(value) > 128) fail(400, 'invalidEnrollment', 'Use a Mac name within 128 bytes.');
  return value;
}

export function runnerEnrollment(db: DatabaseSync, options: Options = {}) {
  const now = options.now ?? Date.now;
  db.exec(`
    CREATE TABLE IF NOT EXISTS runner_invitations (id TEXT PRIMARY KEY, code_hash TEXT UNIQUE NOT NULL, expires INTEGER NOT NULL, runner_id TEXT, token_hash TEXT);
    CREATE TABLE IF NOT EXISTS runners (id TEXT PRIMARY KEY, name TEXT NOT NULL, token_hash TEXT UNIQUE NOT NULL, invitation_id TEXT UNIQUE NOT NULL, enrolled INTEGER NOT NULL);
  `);
  const transaction = <T>(work: () => T): T => {
    db.exec('BEGIN IMMEDIATE');
    try { const result = work(); db.exec('COMMIT'); return result; } catch (error) { db.exec('ROLLBACK'); throw error; }
  };
  return {
    invite(input: unknown): { invitationId: string; expiresAt: string } {
      const value = object(input, ['invitationId', 'code']), invitationId = identifier(value.invitationId), codeHash = hash(secret(value.code));
      return transaction(() => {
        const prior = db.prepare('SELECT code_hash, expires, runner_id FROM runner_invitations WHERE id=?').get(invitationId);
        if (prior) {
          if (prior.code_hash !== codeHash || prior.runner_id !== null) fail(409, 'invitationChanged', 'This invitation cannot be replaced by a retry.');
          return { invitationId, expiresAt: new Date(Number(prior.expires)).toISOString() };
        }
        if (db.prepare('SELECT 1 FROM runner_invitations WHERE code_hash=?').get(codeHash)) fail(409, 'invitationReused', 'Prepare a new invitation code.');
        const expires = now() + INVITATION_MS;
        db.prepare('INSERT INTO runner_invitations VALUES (?,?,?,NULL,NULL)').run(invitationId, codeHash, expires);
        return { invitationId, expiresAt: new Date(expires).toISOString() };
      });
    },
    join(input: unknown): { runnerId: string; runnerName: string } {
      const value = object(input, ['invitationId', 'code', 'runnerId', 'runnerToken', 'runnerName']);
      const invitationId = identifier(value.invitationId), codeHash = hash(secret(value.code)), runnerId = identifier(value.runnerId);
      const tokenHash = hash(secret(value.runnerToken)), runnerName = name(value.runnerName);
      if (codeHash === tokenHash) fail(400, 'credentialReused', 'Prepare a separate private runner token.');
      return transaction(() => {
        const invitation = db.prepare('SELECT code_hash, expires, runner_id, token_hash FROM runner_invitations WHERE id=?').get(invitationId);
        if (!invitation || invitation.code_hash !== codeHash) fail(401, 'invitationDenied', 'This invitation does not match.');
        if (invitation.runner_id !== null) {
          const enrolled = db.prepare('SELECT name FROM runners WHERE id=? AND token_hash=? AND invitation_id=?').get(runnerId, tokenHash, invitationId);
          if (invitation.runner_id !== runnerId || invitation.token_hash !== tokenHash || !enrolled || enrolled.name !== runnerName) fail(401, 'invitationDenied', 'This invitation is already used.');
          return { runnerId, runnerName };
        }
        if (Number(invitation.expires) <= now()) fail(401, 'invitationDenied', 'This invitation is expired.');
        if (db.prepare('SELECT 1 FROM runners WHERE id=?').get(runnerId)) fail(409, 'runnerEnrolled', 'Revoke the enrolled Mac with this ID before enrolling it again.');
        if (db.prepare('SELECT 1 FROM runners WHERE token_hash=?').get(tokenHash)) fail(409, 'credentialReused', 'Prepare a separate private runner token.');
        db.prepare('UPDATE runner_invitations SET runner_id=?, token_hash=? WHERE id=?').run(runnerId, tokenHash, invitationId);
        db.prepare('INSERT INTO runners VALUES (?,?,?,?,?)').run(runnerId, runnerName, tokenHash, invitationId, now());
        return { runnerId, runnerName };
      });
    },
    authenticate(token: string): string | undefined {
      if (!/^[A-Za-z0-9_-]{43}$/.test(token)) return undefined;
      const row = db.prepare('SELECT id FROM runners WHERE token_hash=?').get(hash(token));
      return typeof row?.id === 'string' ? row.id : undefined;
    },
    list(): RunnerSummary[] {
      return db.prepare('SELECT id, name, enrolled FROM runners ORDER BY enrolled, id').all()
        .map(row => ({ runnerId: String(row.id), runnerName: String(row.name), enrolledAt: new Date(Number(row.enrolled)).toISOString() }));
    },
    revoke(runnerId: string): boolean {
      const removed = db.prepare('DELETE FROM runners WHERE id=?').run(identifier(runnerId)).changes > 0;
      if (removed) options.revoked?.(runnerId);
      return removed;
    },
  };
}
