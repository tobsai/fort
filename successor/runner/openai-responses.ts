import type { ModelOption } from '../contracts/v1.ts';

// OpenAI "Sign in with ChatGPT" plan usage: POST /v1/responses with store:false and stream:true (Spec 059, October 22 revision).
const INPUT_LIMIT = 1024 * 1024, FRAME_LIMIT = 256 * 1024, REPLY_LIMIT = 64 * 1024;
const MODEL = /^[a-zA-Z0-9_.:/-]{1,128}$/;

export type ResponsesTurnInput = { model: string; instructions: string; history: { role: 'user' | 'assistant'; text: string }[]; text: string };
export type ResponsesOutcome =
  | { kind: 'completed'; text: string }
  | { kind: 'failed'; reason: 'usageLimit' | 'usageUnavailable' | 'providerFailed' | 'incomplete'; message: string }
  | { kind: 'outcomeUnknown'; reason: 'invalidProtocol' | 'outputLimit' | 'connectionLost' };

const record = (value: unknown): Record<string, unknown> => value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
const wellFormed = (text: unknown): text is string => typeof text === 'string' && text.isWellFormed();

export function buildResponsesRequest(input: ResponsesTurnInput) {
  if (!MODEL.test(input.model)) throw new Error('Select an exact model');
  const turns = [...input.history, { role: 'user' as const, text: input.text }];
  if (![input.instructions, ...turns.map(turn => turn.text)].every(wellFormed) || turns.some(turn => turn.role !== 'user' && turn.role !== 'assistant')) throw new Error('Invalid conversation text');
  const body = {
    model: input.model, instructions: input.instructions, store: false, stream: true,
    input: turns.map(turn => ({ type: 'message', role: turn.role, content: [{ type: turn.role === 'user' ? 'input_text' : 'output_text', text: turn.text }] })),
  };
  if (Buffer.byteLength(JSON.stringify(body)) > INPUT_LIMIT) throw new Error('Conversation context exceeds the input limit');
  return body;
}

function failure(code: unknown, incomplete = false): ResponsesOutcome {
  if (code === 'subscription_sharing_usage_limit_exceeded') return { kind: 'failed', reason: 'usageLimit', message: 'Your ChatGPT plan usage limit was reached. Nothing else was tried.' };
  if (code === 'subscription_sharing_usage_unavailable') return { kind: 'failed', reason: 'usageUnavailable', message: 'ChatGPT plan usage is temporarily unavailable. Try again later.' };
  if (incomplete) return { kind: 'failed', reason: 'incomplete', message: 'The model stopped before finishing a reply.' };
  return { kind: 'failed', reason: 'providerFailed', message: 'OpenAI reported that the reply failed.' };
}

function completedText(response: Record<string, unknown>): string | undefined {
  if (response.status !== 'completed' || !Array.isArray(response.output)) return undefined;
  let text = '';
  for (const item of response.output.map(record)) {
    if (item.type !== 'message' || item.role !== 'assistant' || !Array.isArray(item.content)) continue;
    for (const part of item.content.map(record)) if (part.type === 'output_text') { if (!wellFormed(part.text)) return undefined; text += part.text; }
  }
  return text.length > 0 ? text : undefined;
}

// Parses one server-sent event stream. The first terminal outcome is final; later bytes are ignored.
export class ResponsesStream {
  private readonly decoder = new TextDecoder('utf-8', { fatal: true });
  private buffer = '';
  private outcome: ResponsesOutcome | undefined;

  receive(chunk: string | Uint8Array): ResponsesOutcome | undefined {
    if (this.outcome) return this.outcome;
    try { this.buffer += typeof chunk === 'string' ? chunk : this.decoder.decode(chunk, { stream: true }); }
    catch { return this.outcome = { kind: 'outcomeUnknown', reason: 'invalidProtocol' }; }
    let end;
    while (!this.outcome && (end = this.buffer.indexOf('\n\n')) >= 0) {
      const frame = this.buffer.slice(0, end);
      this.buffer = this.buffer.slice(end + 2);
      this.outcome = this.frame(frame);
    }
    if (!this.outcome && Buffer.byteLength(this.buffer) > FRAME_LIMIT) this.outcome = { kind: 'outcomeUnknown', reason: 'outputLimit' };
    return this.outcome;
  }

  finish(): ResponsesOutcome {
    return this.outcome ??= { kind: 'outcomeUnknown', reason: 'connectionLost' };
  }

  private frame(frame: string): ResponsesOutcome | undefined {
    if (Buffer.byteLength(frame) > FRAME_LIMIT) return { kind: 'outcomeUnknown', reason: 'outputLimit' };
    const data = frame.split('\n').filter(line => line.startsWith('data:')).map(line => line.slice(line.startsWith('data: ') ? 6 : 5)).join('\n');
    if (data.length === 0) return undefined;
    let event: Record<string, unknown>;
    try { event = record(JSON.parse(data)); } catch { return { kind: 'outcomeUnknown', reason: 'invalidProtocol' }; }
    const response = record(event.response);
    switch (event.type) {
      case 'response.completed': {
        const text = completedText(response);
        if (text === undefined) return { kind: 'outcomeUnknown', reason: 'invalidProtocol' };
        return Buffer.byteLength(text) > REPLY_LIMIT ? { kind: 'outcomeUnknown', reason: 'outputLimit' } : { kind: 'completed', text };
      }
      case 'response.failed': return failure(record(response.error).code);
      case 'response.incomplete': return failure(record(response.error).code, true);
      case 'error': return failure(event.code ?? record(event.error).code);
      default: return typeof event.type === 'string' ? undefined : { kind: 'outcomeUnknown', reason: 'invalidProtocol' };
    }
  }
}

// GET /v1/models with the same token: keep entries whose visibility is "list" and pin the exact slug.
export function parseResponsesModels(value: unknown): ModelOption[] {
  const rows = record(value).data;
  if (!Array.isArray(rows)) throw new Error('Invalid model catalog');
  const seen = new Set<string>(), models: ModelOption[] = [];
  for (const row of rows.map(record)) {
    if (typeof row.slug !== 'string' || !MODEL.test(row.slug) || seen.has(row.slug)) throw new Error('Invalid model catalog');
    seen.add(row.slug);
    if (row.visibility !== 'list') continue;
    const name = typeof row.display_name === 'string' && row.display_name.length > 0 && row.display_name.length <= 128 ? row.display_name : row.slug;
    models.push({ id: row.slug, name, availability: 'available' });
  }
  return models;
}
