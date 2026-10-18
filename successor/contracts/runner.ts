import type { RuntimeAttempt } from './attempt.ts';
import type { Binding } from './v1.ts';
import type { RuntimeEvidence } from './runtime.ts';

export type RunnerScope = { protocolVersion: 1; installationId: string; recoveryEpoch: string; hostId: string; sessionId: string };
export type SyntheticDispatch = RunnerScope & {
  mode: 'synthetic'; protocol: 'codex-turn'; dispatchToken: string;
  attempt: RuntimeAttempt; binding: Binding; inputDigest: string;
  threadId: string; requestId: string; workspace: '/synthetic/workspace'; text: string;
};
export type RunnerEvidenceCommand = RunnerScope & { dispatchToken: string; event: RuntimeEvidence };
export type RunnerReceipt = { attemptId: string; sequence: number; cursor: number; duplicate: boolean };
