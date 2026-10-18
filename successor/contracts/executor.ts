import { createHash } from 'node:crypto';
import type { RuntimeAttempt } from './attempt.ts';
import type { Binding } from './v1.ts';

export const EXECUTION_LIMITS = Object.freeze({ inputBytes: 1_048_576, replyBytes: 65_536, evidenceBytes: 262_144 });
export const TEXT_POLICY = Object.freeze({ kind: 'plainText', grants: Object.freeze([]) } as const);
export type ExecutionDispatch = Readonly<{
  protocolVersion: 1;
  installationId: string;
  recoveryEpoch: string;
  binding: Binding;
  attempt: RuntimeAttempt;
  inputDigest: string;
  configurationDigest: string;
  historyDigest: string;
  policy: typeof TEXT_POLICY;
  policyDigest: string;
  deadline: number;
  limits: typeof EXECUTION_LIMITS;
  dispatchDigest: string;
}>;

export function executionDigest(value: unknown): string {
  return createHash('sha256').update(JSON.stringify(value)).digest('hex');
}

export function sealDispatch(value: Omit<ExecutionDispatch, 'dispatchDigest'>): ExecutionDispatch {
  return immutable({ ...value, dispatchDigest: executionDigest(value) });
}

export function immutable<T>(value: T): T {
  if (typeof value === 'object' && value !== null) {
    for (const child of Object.values(value)) immutable(child);
    Object.freeze(value);
  }
  return value;
}
