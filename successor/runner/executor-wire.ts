import { EXECUTION_LIMITS, TEXT_POLICY, executionDigest, immutable, type ExecutionDispatch } from '../contracts/executor.ts';
import { parseContextModuleRef } from '../contracts/context.ts';
import { parseSkillBundle } from '../contracts/skill-bundle.ts';
import type { Binding, ModelCatalog, ModelOption } from '../contracts/v1.ts';
import type { ChatInput, ChatResult } from './chat-runtime.ts';

export type ExecutorBinding = Pick<Binding, 'provider' | 'runtimeVersion' | 'hostId'>;
export type ExecutorRequest = { input: ChatInput; dispatch: ExecutionDispatch };
export const DISPATCH_FRAME_BYTES = EXECUTION_LIMITS.inputBytes + 16_384;
export const MODEL_FRAME_BYTES = 4096;
export const MODEL_EVIDENCE_BYTES = 1_048_576;
export type ExecutorModelRequest = {
  protocolVersion: 1; kind: 'models'; requestId: string; binding: ExecutorBinding; deadline: number; requestDigest: string;
};

function reject(): never { throw new Error('Executor input was not confirmed'); }
function object(value: unknown, keys: readonly string[]): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) reject();
  const fields: Record<string, unknown> = {};
  for (const [key, item] of Object.entries(value)) {
    if (!keys.includes(key)) reject();
    fields[key] = item;
  }
  return fields;
}
function text(value: unknown, limit = 256, empty = false): asserts value is string {
  if (typeof value !== 'string' || !value.isWellFormed() || (!empty && !value.trim()) || Buffer.byteLength(value) > limit) reject();
}
function strings(value: Record<string, unknown>, keys: readonly string[], limit = 256) {
  for (const key of keys) { const field = value[key]; text(field, limit); if (field.includes('\0')) reject(); }
}
function list(value: unknown, limit: number): unknown[] {
  if (!Array.isArray(value) || value.length > limit) reject();
  return value;
}
function positive(value: unknown) {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 1) reject();
}
function attempt(value: unknown) {
  const v = object(value, ['runId', 'attemptId', 'generation', 'bindingRevisionId', 'runtimeVersion', 'selectedModel']);
  strings(v, ['runId', 'attemptId', 'bindingRevisionId', 'runtimeVersion', 'selectedModel']); positive(v.generation);
  return v;
}
function memory(value: unknown, conversationId: unknown) {
  const v = object(value, ['conversationId', 'revisionId', 'text']);
  strings(v, ['conversationId', 'revisionId']); text(v.text, 8192, true);
  if (v.conversationId !== conversationId) reject();
}
function moduleRef(value: unknown) {
  object(value, ['id', 'version']); return parseContextModuleRef(value);
}
function skillBundle(value: unknown) {
  const v = object(value, ['schemaVersion', 'name', 'version', 'source', 'license', 'entrypoint', 'supportedRuntimes', 'requestedTools', 'scripts', 'files', 'digest']);
  const parsed = parseSkillBundle(value);
  if (v.digest !== parsed.digest) reject();
  for (const item of list(v.files, 64)) {
    const file = object(item, ['path', 'kind', 'executable', 'contentBase64', 'bytes', 'sha256']);
    if (typeof file.bytes !== 'number' || typeof file.sha256 !== 'string') reject();
  }
  return parsed;
}
function skill(value: unknown) {
  const v = object(value, ['skillId', 'revisionId', 'name', 'instructions', 'bundle', 'review', 'previousRevisionId']);
  strings(v, ['skillId', 'revisionId', 'name']); text(v.instructions, 16_384, true);
  if (v.bundle === undefined) {
    if (v.review !== undefined || v.previousRevisionId !== undefined) reject();
    return;
  }
  const bundle = skillBundle(v.bundle), review = object(v.review, ['state', 'digest']);
  if ((review.state !== 'reviewRequired' && review.state !== 'enabled' && review.state !== 'disabled') || review.digest !== bundle.digest) reject();
  if (v.previousRevisionId !== undefined) text(v.previousRevisionId);
}
function configuration(value: unknown, conversationId: unknown) {
  const v = object(value, ['agentId', 'agentName', 'bindingRevisionId', 'selectedModel', 'instructions', 'skills', 'memory', 'contextModule']);
  strings(v, ['agentId', 'agentName', 'bindingRevisionId', 'selectedModel']); text(v.instructions, 65_536, true);
  for (const item of list(v.skills, 64)) skill(item);
  if (v.memory !== undefined) memory(v.memory, conversationId);
  if (v.contextModule !== undefined) moduleRef(v.contextModule);
  return v;
}
function context(value: unknown, config: Record<string, unknown>, conversationId: unknown) {
  const v = object(value, ['kind', 'module', 'configurationDigest', 'content', 'message']);
  if (v.kind === 'empty') { object(value, ['kind']); return; }
  if (v.kind === 'unavailable') { object(value, ['kind', 'message']); text(v.message, 16_384); return; }
  if (v.kind !== 'prepared') reject();
  object(value, ['kind', 'module', 'configurationDigest', 'content']);
  const actual = moduleRef(v.module), selected = moduleRef(config.contextModule);
  if (actual.id !== selected.id || actual.version !== selected.version || v.configurationDigest !== executionDigest(config)) reject();
  const content = object(v.content, ['kind', 'instructions', 'bundles', 'memory']);
  if (content.kind !== 'ready') reject();
  text(content.instructions, 65_536, true);
  if (content.memory !== undefined) memory(content.memory, conversationId);
  for (const item of list(content.bundles, 64)) {
    const bundle = object(item, ['skillId', 'revisionId', 'bundle', 'textFiles']);
    strings(bundle, ['skillId', 'revisionId']); skillBundle(bundle.bundle);
    for (const file of list(bundle.textFiles, 64)) {
      const fields = object(file, ['path', 'text']); text(fields.path, 512); text(fields.text, 262_144, true);
    }
  }
}
function inputShape(value: unknown) {
  const v = object(value, ['context', 'attempt', 'runId', 'requestId', 'conversationId', 'configuration', 'history', 'text']);
  strings(v, ['runId', 'requestId', 'conversationId']); text(v.text, 65_536);
  const a = attempt(v.attempt), config = configuration(v.configuration, v.conversationId);
  if (a.runId !== v.runId || a.bindingRevisionId !== config.bindingRevisionId || a.selectedModel !== config.selectedModel) reject();
  context(v.context, config, v.conversationId);
  for (const item of list(v.history, 1000)) {
    const m = object(item, ['messageId', 'conversationId', 'requestId', 'role', 'text', 'sequence', 'createdAt', 'agentId', 'agentName', 'recipientAgentId', 'recipientAgentName']);
    strings(m, ['messageId', 'conversationId', 'requestId', 'createdAt']); text(m.text, 65_536); positive(m.sequence);
    if (m.conversationId !== v.conversationId || (m.role !== 'user' && m.role !== 'assistant')) reject();
    for (const key of ['agentId', 'agentName', 'recipientAgentId', 'recipientAgentName']) if (m[key] !== undefined) text(m[key]);
  }
  return { value: v, attempt: a, configuration: config };
}

export function validateExecutorBinding(value: ExecutorBinding): void {
  strings(object(value, ['provider', 'runtimeVersion', 'hostId']), ['provider', 'runtimeVersion', 'hostId']);
}

export function parseExecutorModelRequest(value: unknown, selected: ExecutorBinding): ExecutorModelRequest {
  const v = object(value, ['protocolVersion', 'kind', 'requestId', 'binding', 'deadline', 'requestDigest']);
  const binding = object(v.binding, ['provider', 'runtimeVersion', 'hostId']);
  strings(binding, ['provider', 'runtimeVersion', 'hostId']);
  const requestId = v.requestId; text(requestId, 128);
  const requestDigest = v.requestDigest; text(requestDigest, 64);
  const { requestDigest: _digest, ...body } = v;
  if (v.protocolVersion !== 1 || v.kind !== 'models' || !/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(requestId)
    || binding.provider !== selected.provider || binding.runtimeVersion !== selected.runtimeVersion || binding.hostId !== selected.hostId
    || typeof v.deadline !== 'number' || !Number.isSafeInteger(v.deadline) || v.deadline <= Date.now() || v.deadline > Date.now() + 5000
    || executionDigest(body) !== requestDigest || Buffer.byteLength(JSON.stringify(value)) > MODEL_FRAME_BYTES) reject();
  return immutable({ protocolVersion: 1, kind: 'models', requestId, binding: { ...selected }, deadline: v.deadline, requestDigest });
}

export function validateExecutorModelCancel(value: unknown, request: ExecutorModelRequest): void {
  const v = object(value, ['protocolVersion', 'kind', 'requestId', 'requestDigest']);
  if (v.protocolVersion !== 1 || v.kind !== 'cancelModels' || v.requestId !== request.requestId || v.requestDigest !== request.requestDigest) reject();
}

export function parseExecutorModelCatalog(value: unknown, provider: string): ModelCatalog {
  const v = object(value, ['provider', 'state', 'models', 'checkedAt', 'detail']);
  if (v.provider !== provider || Buffer.byteLength(JSON.stringify(value)) > MODEL_EVIDENCE_BYTES) reject();
  if (v.state === 'ready') {
    object(value, ['provider', 'state', 'models', 'checkedAt']);
    const checkedAt = v.checkedAt; text(checkedAt, 24);
    if (!Number.isFinite(Date.parse(checkedAt)) || new Date(checkedAt).toISOString() !== checkedAt) reject();
    const seen = new Set<string>();
    const models: ModelOption[] = list(v.models, 1000).map(value => {
      const row = object(value, ['id', 'name', 'availability', 'detail']);
      const id = row.id, name = row.name; text(id, 128); text(name, 512);
      if (!/^[A-Za-z0-9_.:/-]+$/.test(id) || seen.has(id) || name.length > 128 || name.includes('\0')) reject();
      if (row.availability !== 'available' && row.availability !== 'unavailable') reject();
      seen.add(id);
      const detail = row.detail;
      if (detail !== undefined) { text(detail, 16_384); if (detail.includes('\0')) reject(); }
      return { id, name, availability: row.availability, ...(detail === undefined ? {} : { detail }) };
    });
    return { provider, state: 'ready', models, checkedAt };
  }
  object(value, ['provider', 'state', 'models', 'detail']);
  const detail = v.detail; text(detail, 16_384);
  if ((v.state !== 'unavailable' && v.state !== 'error') || list(v.models, 0).length !== 0 || detail.includes('\0')) reject();
  return { provider, state: v.state, models: [], detail };
}

export function parseExecutorModelFrame(value: unknown, request: ExecutorModelRequest): ModelCatalog {
  const v = object(value, ['protocolVersion', 'kind', 'sequence', 'requestId', 'requestDigest', 'result']);
  if (v.protocolVersion !== 1 || v.kind !== 'catalog' || v.sequence !== 1 || v.requestId !== request.requestId
    || v.requestDigest !== request.requestDigest || Date.now() >= request.deadline) reject();
  return parseExecutorModelCatalog(v.result, request.binding.provider);
}

export function parseExecutorRequest(value: unknown, selected: ExecutorBinding): ExecutorRequest {
  const frame = object(value, ['protocolVersion', 'kind', 'dispatch', 'input']);
  if (frame.protocolVersion !== 1 || frame.kind !== 'dispatch') reject();
  const input = inputShape(frame.input);
  const d = object(frame.dispatch, ['protocolVersion', 'installationId', 'recoveryEpoch', 'binding', 'attempt', 'inputDigest', 'configurationDigest', 'historyDigest', 'policy', 'policyDigest', 'deadline', 'limits', 'dispatchDigest']);
  strings(d, ['installationId', 'recoveryEpoch']);
  const b = object(d.binding, ['bindingRevisionId', 'provider', 'runtimeVersion', 'selectedModel', 'hostId', 'enabled']);
  strings(b, ['bindingRevisionId', 'provider', 'runtimeVersion', 'selectedModel', 'hostId']); attempt(d.attempt);
  const { dispatchDigest, ...body } = d;
  if (d.protocolVersion !== 1 || executionDigest(body) !== dispatchDigest
    || b.enabled !== true || b.provider !== selected.provider || b.runtimeVersion !== selected.runtimeVersion || b.hostId !== selected.hostId
    || b.bindingRevisionId !== input.attempt.bindingRevisionId || b.runtimeVersion !== input.attempt.runtimeVersion || b.selectedModel !== input.attempt.selectedModel
    || executionDigest(d.attempt) !== executionDigest(input.value.attempt)
    || executionDigest(d.policy) !== executionDigest(TEXT_POLICY) || d.policyDigest !== executionDigest(TEXT_POLICY)
    || executionDigest(d.limits) !== executionDigest(EXECUTION_LIMITS)
    || executionDigest(frame.input) !== d.inputDigest || executionDigest(input.value.configuration) !== d.configurationDigest || executionDigest(input.value.history) !== d.historyDigest
    || typeof d.deadline !== 'number' || !Number.isSafeInteger(d.deadline) || d.deadline <= Date.now() || d.deadline > Date.now() + 120_000
    || Buffer.byteLength(JSON.stringify(frame.input)) > EXECUTION_LIMITS.inputBytes) reject();
  // Every input/dispatch variant is validated above; retain original key order for the sealed digests.
  return immutable({ input: frame.input as ChatInput, dispatch: frame.dispatch as ExecutionDispatch });
}

export function validateExecutorCancel(value: unknown, dispatch: ExecutionDispatch): void {
  const v = object(value, ['protocolVersion', 'kind', 'dispatchDigest', 'attemptId', 'generation']);
  if (v.protocolVersion !== 1 || v.kind !== 'cancel' || v.dispatchDigest !== dispatch.dispatchDigest || v.attemptId !== dispatch.attempt.attemptId || v.generation !== dispatch.attempt.generation) reject();
}

export function parseExecutorResult(value: unknown): ChatResult {
  const v = object(value, ['kind', 'text', 'effectiveModel', 'message']);
  if (v.kind === 'completed') {
    object(value, ['kind', 'text', 'effectiveModel']); text(v.text, EXECUTION_LIMITS.replyBytes);
    if (v.effectiveModel !== null) reject();
    return { kind: 'completed', text: v.text, effectiveModel: null };
  }
  object(value, ['kind', 'message']); text(v.message, 16_384);
  if (v.kind !== 'notStarted' && v.kind !== 'failed' && v.kind !== 'outcomeUnknown' && v.kind !== 'canceled') reject();
  return { kind: v.kind, message: v.message };
}
