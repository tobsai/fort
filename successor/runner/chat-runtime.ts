import type { Message, TurnConfiguration } from '../contracts/v1.ts';

export type ChatInput = {
  runId: string; requestId: string; conversationId: string;
  configuration: TurnConfiguration; history: Message[]; text: string;
};
export type ChatResult =
  | { kind: 'completed'; text: string; effectiveModel: null }
  | { kind: 'failed' | 'outcomeUnknown'; message: string };
export type ChatRuntime = { execute(input: ChatInput, signal: AbortSignal): Promise<ChatResult> };
