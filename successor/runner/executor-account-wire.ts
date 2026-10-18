import { parseAccountChallenge, parseAccountSignInResult } from '../contracts/account.ts';
export { parseAccountChallenge, parseAccountSignInResult } from '../contracts/account.ts';
import { executionDigest, immutable } from '../contracts/executor.ts';
import type { AccountChallenge, AccountSignInResult } from '../contracts/v1.ts';
import { validateExecutorBinding, type ExecutorBinding } from './executor-wire.ts';

export const ACCOUNT_FRAME_BYTES = 4096;
export const ACCOUNT_EVIDENCE_BYTES = 16_384;
export type ExecutorAccountRequest = {
  protocolVersion: 1; kind: 'accountSignIn'; requestId: string; binding: ExecutorBinding;
  workDeadline: number; deadline: number; requestDigest: string;
};
function reject(): never { throw new Error('Account executor evidence was not confirmed'); }
function object(value: unknown, keys: readonly string[]): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) reject();
  const fields: Record<string, unknown> = {};
  for (const [key, item] of Object.entries(value)) { if (!keys.includes(key)) reject(); fields[key] = item; }
  return fields;
}
function identifier(value: unknown): value is string { return typeof value === 'string' && /^[A-Za-z0-9_-]{1,128}$/.test(value); }

export function parseExecutorAccountRequest(value: unknown, selected: ExecutorBinding): ExecutorAccountRequest {
  const row = object(value, ['protocolVersion', 'kind', 'requestId', 'binding', 'workDeadline', 'deadline', 'requestDigest']);
  const binding = object(row.binding, ['provider', 'runtimeVersion', 'hostId']);
  if (row.protocolVersion !== 1 || row.kind !== 'accountSignIn' || !identifier(row.requestId)
      || binding.provider !== selected.provider || binding.runtimeVersion !== selected.runtimeVersion || binding.hostId !== selected.hostId
      || typeof row.workDeadline !== 'number' || !Number.isSafeInteger(row.workDeadline) || row.workDeadline <= Date.now()
      || typeof row.deadline !== 'number' || row.deadline !== row.workDeadline + 5000 || row.deadline > Date.now() + 120_000
      || typeof row.requestDigest !== 'string' || !/^[a-f0-9]{64}$/.test(row.requestDigest)) reject();
  validateExecutorBinding(selected);
  const { requestDigest, ...body } = row;
  if (executionDigest(body) !== requestDigest) reject();
  return immutable({ protocolVersion: 1, kind: 'accountSignIn', requestId: row.requestId, binding: { ...selected },
    workDeadline: row.workDeadline, deadline: row.deadline, requestDigest: row.requestDigest });
}

export function parseExecutorAccountFrame(value: unknown, request: ExecutorAccountRequest):
  { kind: 'challenge'; value: AccountChallenge } | { kind: 'result'; value: AccountSignInResult } {
  const row = object(value, ['protocolVersion', 'kind', 'sequence', 'requestId', 'requestDigest', 'challenge', 'result']);
  if (row.protocolVersion !== 1 || row.requestId !== request.requestId || row.requestDigest !== request.requestDigest || Date.now() >= request.deadline) reject();
  if (row.kind === 'accountChallenge' && row.sequence === 0 && !Object.hasOwn(row, 'result')) return { kind: 'challenge', value: parseAccountChallenge(row.challenge) };
  if (row.kind === 'accountResult' && row.sequence === 1 && !Object.hasOwn(row, 'challenge')) return { kind: 'result', value: parseAccountSignInResult(row.result) };
  return reject();
}

export function validateExecutorAccountCancel(value: unknown, request: ExecutorAccountRequest): void {
  const row = object(value, ['protocolVersion', 'kind', 'requestId', 'requestDigest']);
  if (row.protocolVersion !== 1 || row.kind !== 'cancelAccount' || row.requestId !== request.requestId || row.requestDigest !== request.requestDigest) reject();
}
