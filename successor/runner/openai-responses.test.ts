import { test } from 'node:test';
import assert from 'node:assert/strict';
import { buildResponsesRequest, ResponsesStream, parseResponsesModels } from './openai-responses.ts';

const sse = (type: string, data: Record<string, unknown> = {}) => `event: ${type}\ndata: ${JSON.stringify({ type, ...data })}\n\n`;
const completed = (text = 'Final reply 🦊') => sse('response.completed', { response: { id: 'resp-1', status: 'completed', output: [{ type: 'message', role: 'assistant', content: [{ type: 'output_text', text }] }] } });

test('a plan-usage request is stateless, streamed and carries instructions outside the input roles', () => {
  const body = buildResponsesRequest({ model: 'gpt-6.1-sol', instructions: 'Reply briefly.', history: [{ role: 'user', text: 'Hi' }, { role: 'assistant', text: 'Hello' }], text: 'Next' });
  assert.equal(body.store, false);
  assert.equal(body.stream, true);
  assert.equal(body.model, 'gpt-6.1-sol');
  assert.equal(body.instructions, 'Reply briefly.');
  assert.deepEqual(body.input, [
    { type: 'message', role: 'user', content: [{ type: 'input_text', text: 'Hi' }] },
    { type: 'message', role: 'assistant', content: [{ type: 'output_text', text: 'Hello' }] },
    { type: 'message', role: 'user', content: [{ type: 'input_text', text: 'Next' }] },
  ]);
  for (const unsupported of ['previous_response_id', 'temperature', 'max_output_tokens', 'metadata', 'user', 'background']) assert.equal(Object.hasOwn(body, unsupported), false);
  assert.equal(JSON.stringify(body).includes('"system"'), false);
});

test('request building refuses empty models, oversized input and malformed text', () => {
  assert.throws(() => buildResponsesRequest({ model: '', instructions: '', history: [], text: 'x' }), /model/);
  assert.throws(() => buildResponsesRequest({ model: 'm', instructions: '', history: [], text: 'x'.repeat(1024 * 1024 + 1) }), /limit/);
  assert.throws(() => buildResponsesRequest({ model: 'm', instructions: '', history: [], text: '\ud800' }), /text/);
});

test('only response.completed yields a reply; deltas alone are provisional', () => {
  const stream = new ResponsesStream();
  assert.equal(stream.receive(sse('response.created') + sse('response.output_text.delta', { delta: 'provisional' })), undefined);
  assert.deepEqual(stream.receive(completed()), { kind: 'completed', text: 'Final reply 🦊' });
  assert.deepEqual(stream.finish(), { kind: 'completed', text: 'Final reply 🦊' });
});

test('fragmented and coalesced frames, including split UTF-8, parse identically', () => {
  const bytes = Buffer.from(sse('response.created') + completed('Café 🦊'));
  const stream = new ResponsesStream();
  let result;
  for (let i = 0; i < bytes.length; i += 3) result = stream.receive(bytes.subarray(i, i + 3)) ?? result;
  assert.deepEqual(result, { kind: 'completed', text: 'Café 🦊' });
});

test('failed, incomplete and usage-limit terminals are visible failures, never replies', () => {
  const usage = new ResponsesStream().receive(sse('response.failed', { response: { status: 'failed', error: { code: 'subscription_sharing_usage_limit_exceeded', message: 'limit' } } }));
  assert.deepEqual(usage, { kind: 'failed', reason: 'usageLimit', message: 'Your ChatGPT plan usage limit was reached. Nothing else was tried.' });
  const unavailable = new ResponsesStream().receive(sse('response.failed', { response: { status: 'failed', error: { code: 'subscription_sharing_usage_unavailable' } } }));
  assert.equal(unavailable?.kind === 'failed' && unavailable.reason, 'usageUnavailable');
  const failed = new ResponsesStream().receive(sse('response.failed', { response: { status: 'failed', error: { code: 'server_error' } } }));
  assert.equal(failed?.kind === 'failed' && failed.reason, 'providerFailed');
  const incomplete = new ResponsesStream().receive(sse('response.incomplete', { response: { status: 'incomplete' } }));
  assert.equal(incomplete?.kind === 'failed' && incomplete.reason, 'incomplete');
  const error = new ResponsesStream().receive(sse('error', { code: 'subscription_sharing_usage_limit_exceeded' }));
  assert.equal(error?.kind === 'failed' && error.reason, 'usageLimit');
});

test('a stream that ends or breaks before a terminal leaves the outcome unknown', () => {
  const ended = new ResponsesStream();
  ended.receive(sse('response.created') + sse('response.output_text.delta', { delta: 'partial' }));
  assert.deepEqual(ended.finish(), { kind: 'outcomeUnknown', reason: 'connectionLost' });
  const malformed = new ResponsesStream();
  assert.deepEqual(malformed.receive('event: x\ndata: {not json\n\n'), { kind: 'outcomeUnknown', reason: 'invalidProtocol' });
  assert.deepEqual(malformed.receive(completed()), { kind: 'outcomeUnknown', reason: 'invalidProtocol' });
});

test('oversized frames and replies are refused without committing text', () => {
  assert.deepEqual(new ResponsesStream().receive('data: ' + 'x'.repeat(256 * 1024 + 1)), { kind: 'outcomeUnknown', reason: 'outputLimit' });
  assert.deepEqual(new ResponsesStream().receive(completed('y'.repeat(64 * 1024 + 1))), { kind: 'outcomeUnknown', reason: 'outputLimit' });
  assert.deepEqual(new ResponsesStream().receive(completed('')), { kind: 'outcomeUnknown', reason: 'invalidProtocol' });
});

test('a terminal is final: later frames cannot change the outcome', () => {
  const stream = new ResponsesStream();
  stream.receive(completed('first'));
  assert.deepEqual(stream.receive(completed('second')), { kind: 'completed', text: 'first' });
  assert.deepEqual(stream.finish(), { kind: 'completed', text: 'first' });
});

test('the model picker keeps only listed models with exact slugs', () => {
  const models = parseResponsesModels({ data: [
    { slug: 'gpt-6.1-sol', display_name: 'GPT-6.1 Sol', visibility: 'list' },
    { slug: 'hidden-model', display_name: 'Hidden', visibility: 'hide' },
    { slug: 'gpt-6-astra', visibility: 'list' },
  ] });
  assert.deepEqual(models, [{ id: 'gpt-6.1-sol', name: 'GPT-6.1 Sol', availability: 'available' }, { id: 'gpt-6-astra', name: 'gpt-6-astra', availability: 'available' }]);
  assert.throws(() => parseResponsesModels({ data: [{ slug: 'bad slug', visibility: 'list' }] }), /catalog/);
  assert.throws(() => parseResponsesModels({ data: [{ slug: 'a', visibility: 'list' }, { slug: 'a', visibility: 'list' }] }), /catalog/);
  assert.throws(() => parseResponsesModels({}), /catalog/);
});
