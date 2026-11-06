import type { RuntimeAttempt } from '../contracts/attempt.ts';
import type { ExecutionDispatch } from '../contracts/executor.ts';
import type { PreparedContext } from '../contracts/context.ts';
import type { AgentMemoryContext } from '../contracts/memory.ts';
import type { PortraitRequest, PortraitResult } from '../contracts/portrait.ts';
import type { Message, TurnConfiguration, RuntimeReadiness, ModelCatalog, AccountSignInOptions, AccountSignInResult, AccountIdentity } from '../contracts/v1.ts';

export type ChatInput = {
  context: PreparedContext;
  attempt: RuntimeAttempt;
  runId: string; requestId: string; conversationId: string;
  configuration: TurnConfiguration; history: Message[]; text: string;
  // Spec 068: what the Agent's memory system recalled for this turn, when it has one.
  agentMemory?: AgentMemoryContext;
};
export type ChatResult =
  | { kind: 'completed'; text: string; effectiveModel: null }
  | { kind: 'notStarted' | 'failed' | 'outcomeUnknown' | 'canceled'; message: string };
export type ChatRuntime = { execute(input: ChatInput, signal: AbortSignal, dispatch: ExecutionDispatch): Promise<ChatResult>; dispose(): Promise<void>; models?(signal:AbortSignal, account?: AccountIdentity):Promise<ModelCatalog>; readiness?(): RuntimeReadiness; restart?(signal: AbortSignal): Promise<void>; signIn?(options: AccountSignInOptions, signal: AbortSignal): Promise<AccountSignInResult>;
  // Whether a sign-in can start now, when that differs from turns being ready: a hub that is not signed in yet can't take turns, but can sign in (Spec 063, revision 2026-11-03).
  canSignIn?(): boolean; portrait?(request: PortraitRequest, signal: AbortSignal): Promise<PortraitResult> };
