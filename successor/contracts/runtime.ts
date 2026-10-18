import type { RuntimeAttempt } from './attempt.ts';
export type RuntimeFailure = 'invalidProtocol' | 'outputLimit' | 'connectionLost' | 'startRejected' | 'unsupportedAction' | 'modelChanged';
type Correlation = RuntimeAttempt & { protocolVersion: 1; provider: 'codex'; sequence: number };
type ProviderTurn = { threadId: string; turnId: string };
export type RuntimeMessagePhase = 'commentary' | 'finalAnswer' | 'unreported';
export type RuntimeEvidenceBody =
  | { kind: 'notStarted'; reason: 'canceledBeforeDispatch' }
  | (ProviderTurn & { kind: 'started' })
  | (ProviderTurn & { kind: 'textDelta'; itemId: string; text: string })
  | (ProviderTurn & { kind: 'terminal'; status: 'completed' | 'interrupted' | 'failed'; messages: { itemId: string; text: string; phase: RuntimeMessagePhase }[]; effectiveModel: null; modelEvidence: 'unreported' })
  | { kind: 'outcomeUnknown'; reason: RuntimeFailure }
;
export type RuntimeEvidence = Correlation & RuntimeEvidenceBody;
