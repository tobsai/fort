import type { RuntimeAttempt } from '../contracts/attempt.ts';
import type { ExecutionDispatch } from '../contracts/executor.ts';
import type { PreparedContext } from '../contracts/context.ts';
import type { Message, TurnConfiguration, RuntimeReadiness, ModelCatalog, AccountSignInOptions, AccountSignInResult, AccountIdentity } from '../contracts/v1.ts';

export type ChatInput = {
  context: PreparedContext;
  attempt: RuntimeAttempt;
  runId: string; requestId: string; conversationId: string;
  configuration: TurnConfiguration; history: Message[]; text: string;
};
export type ChatResult =
  | { kind: 'completed'; text: string; effectiveModel: null }
  | { kind: 'notStarted' | 'failed' | 'outcomeUnknown' | 'canceled'; message: string };
export type ChatRuntime = { execute(input: ChatInput, signal: AbortSignal, dispatch: ExecutionDispatch): Promise<ChatResult>; dispose(): Promise<void>; models?(signal:AbortSignal, account?: AccountIdentity):Promise<ModelCatalog>; readiness?(): RuntimeReadiness; restart?(signal: AbortSignal): Promise<void>; signIn?(options: AccountSignInOptions, signal: AbortSignal): Promise<AccountSignInResult> };
