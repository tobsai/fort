import type { RuntimeAttempt } from '../contracts/runtime.ts';
import type { Message, TurnConfiguration, RuntimeReadiness, ModelCatalog } from '../contracts/v1.ts';

export type ChatInput = {
  attempt: RuntimeAttempt;
  runId: string; requestId: string; conversationId: string;
  configuration: TurnConfiguration; history: Message[]; text: string;
};
export type ChatResult =
  | { kind: 'completed'; text: string; effectiveModel: null }
  | { kind: 'notStarted' | 'failed' | 'outcomeUnknown' | 'canceled'; message: string };
export type ChatRuntime = { execute(input: ChatInput, signal: AbortSignal): Promise<ChatResult>; dispose(): Promise<void>; models?(signal:AbortSignal):Promise<ModelCatalog>; readiness?(): RuntimeReadiness; restart?(signal: AbortSignal): Promise<void> };
