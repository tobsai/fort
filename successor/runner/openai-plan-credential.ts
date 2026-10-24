import { constants } from 'node:fs';
import { open } from 'node:fs/promises';
import { randomUUID } from 'node:crypto';
import { join } from 'node:path';
import { deletePrivate, privateDirectory, readPrivate, syncDirectory, writePrivate } from './private-file.ts';

// The only Fort-held provider credential (Spec 059, October 22 revision). It stays on the executing host,
// outside the ledger, events, evidence and clients, and is never shared with the Codex credential.
export type PlanCredential = { issuer: string; subject: string; clientId: string; hostId: string; idToken: string; accessToken: string; refreshToken: string; expiresAt: number; scopes: string[]; email?: string };

const CREDENTIAL = 'openai-plan.json', HOST_ID = 'openai-host-id';
const HOST_PATTERN = /^urn:uuid:[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const text = (value: unknown, limit = 16 * 1024): value is string => typeof value === 'string' && value.length > 0 && value.length <= limit && value.isWellFormed();

function parseCredential(value: unknown): PlanCredential {
  const row = value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
  const fields = ['issuer', 'subject', 'clientId', 'hostId', 'idToken', 'accessToken', 'refreshToken'] as const;
  if (!fields.every(field => text(row[field])) || !HOST_PATTERN.test(row.hostId as string) || !Number.isSafeInteger(row.expiresAt)
    || !Array.isArray(row.scopes) || row.scopes.length > 32 || !row.scopes.every(scope => text(scope, 128)) || (row.email !== undefined && !text(row.email, 320))) throw new Error('The saved credential is invalid');
  const credential: PlanCredential = { ...Object.fromEntries(fields.map(field => [field, row[field]])) as Omit<PlanCredential, 'expiresAt' | 'scopes' | 'email'>, expiresAt: row.expiresAt as number, scopes: [...row.scopes as string[]] };
  return row.email === undefined ? credential : { ...credential, email: row.email as string };
}

export async function readPlanCredential(directory: string): Promise<PlanCredential | undefined> {
  await privateDirectory(directory);
  const contents = await readPrivate(join(directory, CREDENTIAL));
  if (contents === undefined) return undefined;
  let value: unknown;
  try { value = JSON.parse(contents); } catch { throw new Error('The saved credential is invalid'); }
  return parseCredential(value);
}

export async function writePlanCredential(directory: string, credential: PlanCredential): Promise<void> {
  await privateDirectory(directory);
  await writePrivate(directory, CREDENTIAL, JSON.stringify(parseCredential(credential)));
}

export async function deletePlanCredential(directory: string): Promise<'deleted' | 'absent'> {
  return deletePrivate(directory, CREDENTIAL);
}

// One opaque identifier per Fort host, created before first sign-in. It is not a credential.
export async function ensureHostId(directory: string): Promise<string> {
  await privateDirectory(directory);
  const existing = await readPrivate(join(directory, HOST_ID));
  if (existing !== undefined) { if (!HOST_PATTERN.test(existing)) throw new Error('The saved host identifier is invalid'); return existing; }
  const hostId = `urn:uuid:${randomUUID()}`;
  let handle;
  try { handle = await open(join(directory, HOST_ID), constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL | constants.O_NOFOLLOW, 0o600); }
  catch (error) { if ((error as NodeJS.ErrnoException).code === 'EEXIST') return ensureHostId(directory); throw error; }
  try { await handle.writeFile(hostId); await handle.sync(); } finally { await handle.close(); }
  await syncDirectory(directory);
  return hostId;
}
