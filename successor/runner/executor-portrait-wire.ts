import { executionDigest, immutable } from '../contracts/executor.ts';
import { parseAccountIdentity } from '../contracts/account.ts';
import type { AccountIdentity } from '../contracts/v1.ts';
import { validateExecutorBinding, type ExecutorBinding } from './executor-wire.ts';
import { decodePortrait, PORTRAIT_MAX_BYTES, type PortraitResult, type PortraitSubject } from '../contracts/portrait.ts';

// Spec 065: one portrait draw per executor process. The result frame carries the PNG as base64.
export const PORTRAIT_FRAME_BYTES = 80 * 1024;
export const PORTRAIT_EVIDENCE_BYTES = Math.ceil(PORTRAIT_MAX_BYTES / 3) * 4 + 16_384;
// Fits the guarded runtime's 120-second ceiling.
export const PORTRAIT_DEADLINE_MS = 115_000;
export type ExecutorPortraitRequest = {
  protocolVersion: 1; kind: 'portrait'; requestId: string; binding: ExecutorBinding; account: AccountIdentity;
  model: string; subject: PortraitSubject; deadline: number; requestDigest: string;
};

function reject(): never { throw new Error('Portrait executor evidence was not confirmed'); }
function object(value: unknown, keys: readonly string[]): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) reject();
  const fields: Record<string, unknown> = {};
  for (const [key, item] of Object.entries(value)) { if (!keys.includes(key)) reject(); fields[key] = item; }
  return fields;
}
const identifier = (value: unknown): value is string => typeof value === 'string' && /^[A-Za-z0-9_-]{1,128}$/.test(value);
const text = (value: unknown, limit: number): value is string => typeof value === 'string' && value.isWellFormed() && !value.includes('\0') && Buffer.byteLength(value) <= limit;

export function portraitRequestBody(binding: ExecutorBinding, account: AccountIdentity, model: string, subject: PortraitSubject, requestId: string, now = Date.now()): ExecutorPortraitRequest {
  const body = { protocolVersion: 1, kind: 'portrait', requestId, binding, account: parseAccountIdentity(account), model, subject: { ...subject }, deadline: now + PORTRAIT_DEADLINE_MS } as const;
  return { ...body, requestDigest: executionDigest(body) };
}

export function parseExecutorPortraitRequest(value: unknown, selected: ExecutorBinding): ExecutorPortraitRequest {
  const row = object(value, ['protocolVersion', 'kind', 'requestId', 'binding', 'account', 'model', 'subject', 'deadline', 'requestDigest']);
  const binding = object(row.binding, ['provider', 'runtimeVersion', 'hostId']);
  const subject = object(row.subject, ['name', 'instructions', 'note']);
  if (row.protocolVersion !== 1 || row.kind !== 'portrait' || !identifier(row.requestId)
      || binding.provider !== selected.provider || binding.runtimeVersion !== selected.runtimeVersion || binding.hostId !== selected.hostId
      || !text(row.model, 128) || !/^[A-Za-z0-9_.:/-]{1,128}$/.test(row.model)
      || !text(subject.name, 128) || !subject.name.trim() || !text(subject.instructions, 65_536) || (subject.note !== undefined && !text(subject.note, 500))
      || typeof row.deadline !== 'number' || !Number.isSafeInteger(row.deadline) || row.deadline <= Date.now() || row.deadline > Date.now() + PORTRAIT_DEADLINE_MS
      || typeof row.requestDigest !== 'string' || !/^[a-f0-9]{64}$/.test(row.requestDigest) || Buffer.byteLength(JSON.stringify(value)) > PORTRAIT_FRAME_BYTES) reject();
  validateExecutorBinding(selected);
  const account = parseAccountIdentity(row.account);
  const { requestDigest, ...body } = row;
  if (executionDigest(body) !== requestDigest) reject();
  return immutable({ protocolVersion: 1, kind: 'portrait', requestId: row.requestId, binding: { ...selected }, account, model: row.model,
    subject: { name: subject.name, instructions: subject.instructions, ...(subject.note === undefined ? {} : { note: subject.note as string }) }, deadline: row.deadline, requestDigest: row.requestDigest });
}

export function validateExecutorPortraitCancel(value: unknown, request: ExecutorPortraitRequest): void {
  const row = object(value, ['protocolVersion', 'kind', 'requestId', 'requestDigest']);
  if (row.protocolVersion !== 1 || row.kind !== 'cancelPortrait' || row.requestId !== request.requestId || row.requestDigest !== request.requestDigest) reject();
}

export function encodePortraitResult(result: PortraitResult): object {
  return result.kind === 'drawn' ? { kind: 'drawn', png: result.png.toString('base64'), width: result.width, height: result.height } : { ...result };
}

export function parsePortraitResult(value: unknown): PortraitResult {
  const row = object(value, ['kind', 'png', 'width', 'height', 'reason', 'message']);
  if (row.kind === 'drawn') {
    object(value, ['kind', 'png', 'width', 'height']);
    const image = decodePortrait(row.png);
    if (image.width !== row.width || image.height !== row.height) reject();
    return { kind: 'drawn', ...image };
  }
  if (!text(row.message, 16_384) || !row.message.trim()) reject();
  if (row.kind === 'failed') {
    object(value, ['kind', 'reason', 'message']);
    if (row.reason !== 'usageLimit' && row.reason !== 'refused' && row.reason !== 'noImage' && row.reason !== 'invalidImage') reject();
    return { kind: 'failed', reason: row.reason, message: row.message };
  }
  object(value, ['kind', 'message']);
  if (row.kind !== 'notStarted' && row.kind !== 'canceled' && row.kind !== 'outcomeUnknown') reject();
  return { kind: row.kind, message: row.message };
}

export function parseExecutorPortraitFrame(value: unknown, request: ExecutorPortraitRequest): PortraitResult {
  const row = object(value, ['protocolVersion', 'kind', 'sequence', 'requestId', 'requestDigest', 'result']);
  if (row.protocolVersion !== 1 || row.kind !== 'portraitResult' || row.sequence !== 1 || row.requestId !== request.requestId
      || row.requestDigest !== request.requestDigest || Date.now() >= request.deadline) reject();
  return parsePortraitResult(row.result);
}
