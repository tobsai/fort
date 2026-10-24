import { constants } from 'node:fs';
import { lstat, open, rename, unlink } from 'node:fs/promises';
import { randomUUID } from 'node:crypto';
import { isAbsolute, join, normalize } from 'node:path';

// Owner-only files in a host's private state directory, for the few secrets Fort keeps on a Mac.
const LIMIT = 64 * 1024;
const uid = () => process.getuid?.() ?? -1;
const missing = (error: unknown) => (error as NodeJS.ErrnoException).code === 'ENOENT';

export async function privateDirectory(directory: string): Promise<void> {
  if (!isAbsolute(directory) || normalize(directory) !== directory) throw new Error('Use an exact private state directory');
  const info = await lstat(directory);
  if (!info.isDirectory() || (info.mode & 0o077) !== 0 || info.uid !== uid()) throw new Error('Use an exact private state directory');
}

export async function readPrivate(path: string): Promise<string | undefined> {
  let handle;
  try { handle = await open(path, constants.O_RDONLY | constants.O_NOFOLLOW); }
  catch (error) { if (missing(error)) return undefined; throw new Error('The credential file is not a private regular file'); }
  try {
    const info = await handle.stat();
    if (!info.isFile() || info.nlink !== 1 || (info.mode & 0o777) !== 0o600 || info.uid !== uid() || info.size > LIMIT) throw new Error('The credential file is not a private regular file');
    return await handle.readFile('utf8');
  } finally { await handle.close(); }
}

export async function writePrivate(directory: string, name: string, contents: string): Promise<void> {
  const temporary = join(directory, `.${name}.${randomUUID()}`);
  const handle = await open(temporary, constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL | constants.O_NOFOLLOW, 0o600);
  try { await handle.writeFile(contents); await handle.sync(); } finally { await handle.close(); }
  try { await rename(temporary, join(directory, name)); } catch (error) { await unlink(temporary).catch(() => {}); throw error; }
  await syncDirectory(directory);
}

export async function syncDirectory(directory: string): Promise<void> {
  const handle = await open(directory, constants.O_RDONLY | constants.O_DIRECTORY);
  try { await handle.sync(); } finally { await handle.close(); }
}

export async function deletePrivate(directory: string, name: string): Promise<'deleted' | 'absent'> {
  await privateDirectory(directory);
  const path = join(directory, name);
  try {
    const info = await lstat(path);
    if (!info.isFile() || info.nlink !== 1) throw new Error('The credential file is not a private regular file');
    await unlink(path);
  } catch (error) { if (missing(error)) return 'absent'; throw error; }
  await syncDirectory(directory);
  try { await lstat(path); } catch (error) { if (missing(error)) return 'deleted'; throw error; }
  throw new Error('Credential deletion was not confirmed');
}
