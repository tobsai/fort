import { randomUUID } from 'node:crypto';
import type { Readable, Writable } from 'node:stream';

export function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Invalid Codex response');
  return value as Record<string, unknown>;
}
type Pending = { resolve(value: unknown): void; reject(error: Error): void; cleanup(): void };
type Subscriber = { frame(bytes: Uint8Array): void; closed(): void };
export class CodexConnection {
  private readonly io: { input: Writable; output: Readable; close(): void | Promise<void> };
  private disposal: Promise<void> | undefined;
  private readonly pending = new Map<string, Pending>();
  private readonly subscribers = new Set<Subscriber>();
  private readonly decoder = new TextDecoder('utf-8', { fatal:true });
  private buffer = '';
  private stopped = false;
  constructor(io: { input: Writable; output: Readable; close(): void | Promise<void> }) {
    this.io = io;
    io.output.on('data', (bytes: Buffer) => this.receive(bytes));
    io.output.on('end', () => this.close()); io.output.on('error', () => this.close()); io.input.on('error', () => this.close());
  }
  get isClosed():boolean { return this.stopped; }
  request(method: string, params: unknown, signal: AbortSignal): Promise<unknown> {
    if (this.stopped || signal.aborted) return Promise.reject(new Error('Codex connection unavailable'));
    const id = randomUUID();
    return new Promise((resolve, reject) => {
      const abort = () => this.close();
      const timer = setTimeout(abort, 10_000);
      signal.addEventListener('abort', abort, { once:true });
      this.pending.set(id, { resolve, reject, cleanup:() => { clearTimeout(timer); signal.removeEventListener('abort', abort); } });
      try { this.write(JSON.stringify({ id, method, params })+'\n'); } catch { this.close(); }
    });
  }
  notify(method: string, params: unknown) { this.write(JSON.stringify({ method, params })+'\n'); }
  write(line: string) {
    if (this.stopped) throw new Error('Codex connection closed');
    if (Buffer.byteLength(line) > 262_144 || this.io.input.writableLength > 1_000_000) { this.close(); throw new Error('Codex input limit'); }
    this.io.input.write(line);
  }
  subscribe(subscriber: Subscriber): () => void {
    if (this.stopped) throw new Error('Codex connection closed');
    this.subscribers.add(subscriber); return () => { this.subscribers.delete(subscriber); };
  }
  close() {
    if (this.stopped) return; this.stopped = true;
    for (const pending of this.pending.values()) { pending.cleanup(); pending.reject(new Error('Codex connection ended without confirmation')); }
    this.pending.clear();
    for (const subscriber of [...this.subscribers]) subscriber.closed();
    this.subscribers.clear();
    try { this.disposal = Promise.resolve(this.io.close()); } catch { this.disposal = Promise.reject(new Error('Owned transport cleanup failed')); }
    void this.disposal.catch(() => {});
  }
  async dispose(): Promise<void> { this.close(); await this.disposal; }
  private receive(bytes: Uint8Array) {
    if (this.stopped) return;
    try {
      if (bytes.byteLength > 1_000_000) throw new Error('Frame limit');
      this.buffer += this.decoder.decode(bytes, { stream:true });
      for (;;) {
        const end = this.buffer.indexOf('\n'); if (end < 0) break;
        const line = this.buffer.slice(0,end); this.buffer = this.buffer.slice(end+1);
        if (Buffer.byteLength(line) > 262_144) throw new Error('Frame limit');
        if (!line.trim()) continue;
        const frame = object(JSON.parse(line));
        if (typeof frame.method === 'string' && frame.id !== undefined && this.subscribers.size === 0) {
          this.write(JSON.stringify({ id:frame.id, error:{ code:-32601, message:'Unsupported server request' } })+'\n'); this.close(); return;
        }
        const pending = typeof frame.id === 'string' ? this.pending.get(frame.id) : undefined;
        if (pending) {
          this.pending.delete(String(frame.id)); pending.cleanup();
          if (frame.error !== undefined || !Object.hasOwn(frame,'result') || frame.method !== undefined) pending.reject(new Error('Codex request rejected'));
          else pending.resolve(frame.result);
        } else for (const subscriber of [...this.subscribers]) subscriber.frame(Buffer.from(line+'\n'));
        if (this.stopped) return;
      }
      if (Buffer.byteLength(this.buffer) > 262_144) throw new Error('Frame limit');
    } catch { this.close(); }
  }
}
