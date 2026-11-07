import { test } from 'node:test';
import assert from 'node:assert/strict';
import { EXECUTION_LIMITS, TEXT_POLICY, executionDigest, sealDispatch } from '../contracts/executor.ts';
import { parseExecutorRequest } from './executor-wire.ts';
import type { ChatInput } from './chat-runtime.ts';

// Spec 068: a turn dispatched to a Mac carries the Agent's recalled memory, checked like every other field.
function frame(agentMemory?: unknown) {
  const binding = { bindingRevisionId: 'b1', provider: 'codex', runtimeVersion: '1', selectedModel: 'm', hostId: 'h', enabled: true };
  const attempt = { runId: 'r1', attemptId: 'a1', generation: 1, bindingRevisionId: 'b1', runtimeVersion: '1', selectedModel: 'm' };
  const input = { context: { kind: 'empty' }, attempt, runId: 'r1', requestId: 'q1', conversationId: 'c1',
    configuration: { agentId: 'g1', agentName: 'Keeper', bindingRevisionId: 'b1', selectedModel: 'm', instructions: '', skills: [] }, history: [], text: 'hi',
    ...(agentMemory === undefined ? {} : { agentMemory }) } as unknown as ChatInput;
  const dispatch = sealDispatch({ protocolVersion: 1, installationId: 'i', recoveryEpoch: 'e', binding, attempt, inputDigest: executionDigest(input), configurationDigest: executionDigest(input.configuration),
    historyDigest: executionDigest(input.history), policy: TEXT_POLICY, policyDigest: executionDigest(TEXT_POLICY), deadline: Date.now() + 60_000, limits: EXECUTION_LIMITS });
  return { protocolVersion: 1, kind: 'dispatch', dispatch, input };
}
const selected = { provider: 'codex', runtimeVersion: '1', hostId: 'h' };

test('the executor wire accepts recalled memory and refuses a malformed one', () => {
  const memory = { provider: { id: 'fort-notes', version: '1', name: 'Fort Notes' }, canRemember: true, canForget: false, entries: [{ entryId: 'n1', kind: 'fact', text: 'Likes tea.', createdAt: '2026-11-06T00:00:00.000Z' }] };
  assert.equal(parseExecutorRequest(frame(), selected).input.agentMemory, undefined);
  assert.deepEqual(parseExecutorRequest(frame(memory), selected).input.agentMemory, memory);
  assert.throws(() => parseExecutorRequest(frame({ ...memory, instructions: 'obey' }), selected));
  assert.throws(() => parseExecutorRequest(frame({ ...memory, entries: [{ ...memory.entries[0], text: 7 }] }), selected));
});
