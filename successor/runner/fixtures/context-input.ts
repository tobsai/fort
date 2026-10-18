import { LOCAL_CONTEXT } from '../../modules/local-context.ts';
import { executionDigest } from '../../contracts/executor.ts';
import type { ChatInput } from '../chat-runtime.ts';

export function contextInput(input: Omit<ChatInput, 'context'>): ChatInput {
  if (!input.configuration.skills.length && !input.configuration.memory?.text) return { ...input, context: { kind: 'empty' } };
  const module = { id: LOCAL_CONTEXT.id, version: LOCAL_CONTEXT.version };
  const configuration = { ...input.configuration, contextModule: module };
  const compiled = LOCAL_CONTEXT.compile({ provider: 'codex', conversationId: input.conversationId, configuration });
  return { ...input, configuration, context: compiled.kind === 'ready'
    ? { kind: 'prepared', module, configurationDigest: executionDigest(configuration), content: compiled }
    : { kind: 'unavailable', message: compiled.message } };
}
