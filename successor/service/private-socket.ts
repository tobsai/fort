import { lstatSync, unlinkSync, type BigIntStats } from 'node:fs';
import { join } from 'node:path';
import { createConnection } from 'node:net';
import type { DatabaseSync } from 'node:sqlite';

type Ownership = { version: 1; name: string; directory: string; socket: string };
const unavailable = () => new Error('Private socket ownership or absence was not confirmed; existing paths are preserved');
const missing = (error: unknown) => error instanceof Error && 'code' in error && error.code === 'ENOENT';
function inspect(path: string): BigIntStats | undefined {
  try { return lstatSync(path, { bigint: true }); }
  catch (error) { if (!missing(error)) throw error; }
}
function identity(file: BigIntStats, socket: boolean): string {
  return [file.dev, file.ino, file.uid, ...(socket ? [file.ctimeNs] : [])].join(':');
}

export class PrivateSocketLease {
  private readonly db: DatabaseSync;
  private readonly directory: string;
  private readonly name: string;
  private readonly path: string;
  private recorded: string | undefined;
  constructor(db: DatabaseSync, directory: string, name: string) {
    this.db = db; this.directory = directory; this.name = name;
    this.path = join(directory, name);
  }
  private stored(): Ownership | undefined {
    const row = this.db.prepare("SELECT data FROM metadata WHERE key='privateSocketOwnership'").get();
    if (!row) return;
    if (typeof row.data !== 'string') throw unavailable();
    let value: unknown;
    try { value = JSON.parse(row.data); } catch { throw unavailable(); }
    if (!value || typeof value !== 'object' || !('version' in value) || value.version !== 1
      || !('name' in value) || typeof value.name !== 'string' || !/^[A-Za-z0-9][A-Za-z0-9_-]{0,47}\.sock$/.test(value.name)
      || !('directory' in value) || typeof value.directory !== 'string' || !/^\d+:\d+:\d+$/.test(value.directory)
      || !('socket' in value) || typeof value.socket !== 'string' || !/^\d+:\d+:\d+:\d+$/.test(value.socket)
      || Object.keys(value).some(key => !['version', 'name', 'directory', 'socket'].includes(key))) throw unavailable();
    return { version: 1, name: value.name, directory: value.directory, socket: value.socket };
  }
  private capture(file: BigIntStats): Ownership {
    const directory = lstatSync(this.directory, { bigint: true });
    if (!directory.isDirectory() || directory.uid !== BigInt(process.getuid?.() ?? -1) || (directory.mode & 0o077n) !== 0n
      || !file.isSocket() || file.uid !== directory.uid || (file.mode & 0o077n) !== 0n) throw unavailable();
    return { version: 1, name: this.name, directory: identity(directory, false), socket: identity(file, true) };
  }
  async prepare(signal: AbortSignal): Promise<void> {
    signal.throwIfAborted();
    const prior = this.stored();
    if (prior && prior.name !== this.name && inspect(join(this.directory, prior.name))) throw unavailable();
    const file = inspect(this.path);
    if (!file) return;
    const expected = JSON.stringify(this.capture(file));
    if (!prior || JSON.stringify(prior) !== expected) throw unavailable();
    await new Promise<void>((resolve, reject) => {
      const connection = createConnection({ path: this.path, signal });
      let finished = false;
      const finish = (refused: boolean) => {
        if (finished) return;
        finished = true; clearTimeout(timer); connection.destroy();
        if (refused) resolve(); else reject(unavailable());
      };
      const timer = setTimeout(() => finish(false), 250);
      connection.once('connect', () => finish(false));
      connection.once('error', error => finish('code' in error && error.code === 'ECONNREFUSED'));
    });
    signal.throwIfAborted();
    const current = inspect(this.path);
    if (!current || JSON.stringify(this.capture(current)) !== expected) throw unavailable();
    unlinkSync(this.path);
  }
  bound(): void {
    const file = inspect(this.path);
    if (!file) throw unavailable();
    const ownership = JSON.stringify(this.capture(file));
    this.db.prepare("INSERT OR REPLACE INTO metadata VALUES ('privateSocketOwnership',?)").run(ownership);
    this.recorded = ownership;
  }
  release(): void {
    if (!this.recorded) return;
    if (inspect(this.path) || JSON.stringify(this.stored()) !== this.recorded) throw unavailable();
    this.db.prepare("DELETE FROM metadata WHERE key='privateSocketOwnership'").run();
    this.recorded = undefined;
  }
}
