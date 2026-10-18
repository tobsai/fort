import type { Readable, Writable } from 'node:stream';
import { deleteOwnerHistory } from '../service/delete-history.ts';

export async function serveHistoryMaintenance({ directory, input, output, signal }: { directory: string; input: Readable; output: Writable; signal?: AbortSignal }): Promise<void> {
  const failure = () => new Error('History maintenance protocol or cleanup is unconfirmed');
  let canceled = false;
  const abort = () => { canceled = true; input.destroy(); };
  signal?.throwIfAborted();
  signal?.addEventListener('abort', abort, { once: true });
  output.on('error', abort);
  const lifetime = setTimeout(abort, 30_000);
  const write = (frame: unknown) => new Promise<void>((resolve, reject) => {
    const timeout = setTimeout(() => { abort(); reject(failure()); }, 5000);
    output.write(JSON.stringify(frame) + '\n', error => { clearTimeout(timeout); error ? reject(failure()) : resolve(); });
  });
  let pending = Buffer.alloc(0), consumed = false;
  try {
    await write({ protocolVersion: 1, kind: 'ready' });
    for await (const chunk of input) {
      if (!Buffer.isBuffer(chunk) || consumed || pending.length + chunk.length > 4096) throw failure();
      pending = Buffer.concat([pending, chunk]);
      const newline = pending.indexOf(10);
      if (newline < 0) continue;
      if (newline !== pending.length - 1) throw failure();
      const bytes = pending.subarray(0, newline), text = bytes.toString('utf8');
      if (!Buffer.from(text).equals(bytes)) throw failure();
      const frame = JSON.parse(text);
      if (!frame || frame.protocolVersion !== 1 || frame.kind !== 'historyDeletion' || frame.id !== 1
        || Object.keys(frame).sort().join(',') !== 'history,id,kind,protocolVersion') throw failure();
      const request = frame.history;
      if (!request || Array.isArray(request) || Object.keys(request).sort().join(',') !== 'deletionId,deviceId,installationId,recoveryEpoch,timeout'
        || typeof request.timeout !== 'number' || !Number.isFinite(request.timeout) || request.timeout <= 0 || request.timeout > 30
        || !['installationId', 'deviceId', 'recoveryEpoch', 'deletionId'].every(key => typeof request[key] === 'string' && /^[A-Za-z0-9_-]{1,128}$/.test(request[key]))) throw failure();
      signal?.throwIfAborted(); consumed = true; pending = Buffer.alloc(0);
      let status = 200, body: unknown;
      try {
        const result = deleteOwnerHistory({ directory, installationId: request.installationId, deviceId: request.deviceId, recoveryEpoch: request.recoveryEpoch, deletionId: request.deletionId });
        body = { installationId: request.installationId, deviceId: request.deviceId, deletionId: request.deletionId, previousRecoveryEpoch: request.recoveryEpoch, ...result };
      } catch { status = 409; body = { outcome: 'unconfirmed' }; }
      await write({ protocolVersion: 1, kind: 'response', id: 1, response: { status, body: Buffer.from(JSON.stringify(body)).toString('base64') } });
    }
    if (pending.length || canceled) throw failure();
  } catch { throw failure(); }
  finally { clearTimeout(lifetime); signal?.removeEventListener('abort', abort); output.off('error', abort); }
}
