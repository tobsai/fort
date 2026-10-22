import { constants } from 'node:fs';
import { lstat, open, rename, unlink } from 'node:fs/promises';
import { randomUUID } from 'node:crypto';
import { isAbsolute, join, normalize } from 'node:path';

// The only Fort-held provider credential (Spec 059, October 22 revision). It stays on the executing host,
// outside the ledger, events, evidence and clients, and is never shared with the Codex credential.
export type PlanCredential = { issuer: string; subject: string; clientId: string; hostId: string; idToken: string; accessToken: string; refreshToken: string; expiresAt: number; scopes: string[]; email?: string };

const CREDENTIAL = 'openai-plan.json', HOST_ID = 'openai-host-id', LIMIT = 64 * 1024;
const HOST_PATTERN = /^urn:uuid:[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const uid = () => process.getuid?.() ?? -1;
const missing = (error: unknown) => (error as NodeJS.ErrnoException).code === 'ENOENT';
const text = (value: unknown, limit = 16 * 1024): value is string => typeof value === 'string' && value.length > 0 && value.length <= limit && value.isWellFormed();

async function privateDirectory(directory: string): Promise<void> {
  if (!isAbsolute(directory) || normalize(directory) !== directory) throw new Error('Use an exact private state directory');
  const info = await lstat(directory);
  if (!info.isDirectory() || (info.mode & 0o077) !== 0 || info.uid !== uid()) throw new Error('Use an exact private state directory');
}

async function readPrivate(path: string): Promise<string | undefined> {
  let handle;
  try { handle = await open(path, constants.O_RDONLY | constants.O_NOFOLLOW); }
  catch (error) { if (missing(error)) return undefined; throw new Error('The credential file is not a private regular file'); }
  try {
    const info = await handle.stat();
    if (!info.isFile() || info.nlink !== 1 || (info.mode & 0o777) !== 0o600 || info.uid !== uid() || info.size > LIMIT) throw new Error('The credential file is not a private regular file');
    return await handle.readFile('utf8');
  } finally { await handle.close(); }
}

async function writePrivate(directory: string, name: string, contents: string): Promise<void> {
  const temporary = join(directory, `.${name}.${randomUUID()}`);
  const handle = await open(temporary, constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL | constants.O_NOFOLLOW, 0o600);
  try { await handle.writeFile(contents); await handle.sync(); } finally { await handle.close(); }
  try { await rename(temporary, join(directory, name)); } catch (error) { await unlink(temporary).catch(() => {}); throw error; }
  await syncDirectory(directory);
}

async function syncDirectory(directory: string): Promise<void> {
  const handle = await open(directory, constants.O_RDONLY | constants.O_DIRECTORY);
  try { await handle.sync(); } finally { await handle.close(); }
}

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
  await privateDirectory(directory);
  const path = join(directory, CREDENTIAL);
  try {
    const info = await lstat(path);
    if (!info.isFile() || info.nlink !== 1) throw new Error('The credential file is not a private regular file');
    await unlink(path);
  } catch (error) { if (missing(error)) return 'absent'; throw error; }
  await syncDirectory(directory);
  try { await lstat(path); } catch (error) { if (missing(error)) return 'deleted'; throw error; }
  throw new Error('Credential deletion was not confirmed');
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
