import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { contextInput } from './fixtures/context-input.ts';
import type { ChatInput } from './chat-runtime.ts';
import { OpenAIPlanChat, OPENAI_PLAN_RUNTIME, planAccountIdentity } from './openai-plan-chat.ts';
import { writePlanCredential, readPlanCredential, type PlanCredential } from './openai-plan-credential.ts';
import type { ExecutionDispatch } from '../contracts/executor.ts';

const now = () => 1_800_000_000_000;
const credential: PlanCredential = { issuer: 'https://auth.openai.com', subject: 'user-synthetic', clientId: 'oaiapp_synthetic', hostId: 'urn:uuid:00000000-0000-4000-8000-000000000000', idToken: 'id', accessToken: 'access-synthetic', refreshToken: 'refresh-synthetic', expiresAt: now() + 3_600_000, scopes: ['chatgpt.tokens.use.direct', 'offline_access', 'openid'], email: 'owner@example.com' };
const account = planAccountIdentity(credential);
const base: Omit<ChatInput, 'context'> = { attempt: { runId: 'run-1', attemptId: 'attempt-1', generation: 1, bindingRevisionId: 'binding', runtimeVersion: OPENAI_PLAN_RUNTIME.runtimeVersion, selectedModel: 'gpt-6.1-sol' }, runId: 'run-1', requestId: 'request-1', conversationId: 'home', text: 'What was the word?',
  configuration: { account, agentId: 'writer', agentName: 'Writer', bindingRevisionId: 'binding', selectedModel: 'gpt-6.1-sol', instructions: 'Use only the supplied context.', skills: [] },
  history: [{ messageId: 'm1', requestId: 'earlier', conversationId: 'home', role: 'user', text: 'Remember cedar.', sequence: 1, createdAt: '2026-10-22T00:00:00Z' }, { messageId: 'm2', requestId: 'earlier', conversationId: 'home', role: 'assistant', text: 'Noted.', sequence: 2, createdAt: '2026-10-22T00:00:01Z' }] };
const dispatch = { deadline: now() + 120_000 } as ExecutionDispatch;
const sse = (type: string, data: Record<string, unknown> = {}) => `event: ${type}\ndata: ${JSON.stringify({ type, ...data })}\n\n`;
const completed = (text: string) => sse('response.completed', { response: { status: 'completed', output: [{ type: 'message', role: 'assistant', content: [{ type: 'output_text', text }] }] } });
const streamed = (...chunks: string[]) => new Response(new ReadableStream({ start(controller) { for (const chunk of chunks) controller.enqueue(new TextEncoder().encode(chunk)); controller.close(); } }), { status: 200, headers: { 'content-type': 'text/event-stream' } });

async function fixture(t: { after(fn: () => unknown): void }, respond: (url: string, init: RequestInit) => Response | Promise<Response>, saved: PlanCredential | null = credential) {
  const root = await mkdtemp(join(tmpdir(), 'fort-plan-chat-')); t.after(() => rm(root, { recursive: true, force: true }));
  const state = join(root, 'state'); await mkdir(state, { mode: 0o700 });
  if (saved) await writePlanCredential(state, saved);
  const calls: { url: string; init: RequestInit }[] = [];
  const tokenCalls: URLSearchParams[] = [];
  const runtime = new OpenAIPlanChat({ stateDirectory: state, now,
    fetch: async (url, init) => { calls.push({ url: String(url), init: init ?? {} }); return respond(String(url), init ?? {}); },
    tokenTransport: { async postForm(_url, form) { tokenCalls.push(form); return { status: 200, body: { access_token: 'access-new', refresh_token: 'refresh-new', token_type: 'Bearer', expires_in: 3600, scope: 'chatgpt.tokens.use.direct offline_access openid' } }; } } });
  t.after(() => runtime.dispose());
  return { runtime, calls, tokenCalls, state };
}

test('a turn sends one stateless streamed request with the bearer token and returns only the completed reply', async t => {
  const { runtime, calls } = await fixture(t, () => streamed(sse('response.created'), sse('response.output_text.delta', { delta: 'ce' }), completed('cedar')));
  const result = await runtime.execute(contextInput(base), new AbortController().signal, dispatch);
  assert.deepEqual(result, { kind: 'completed', text: 'cedar', effectiveModel: null });
  assert.equal(calls.length, 1);
  assert.equal(calls[0]!.url, 'https://api.openai.com/v1/responses');
  assert.equal(new Headers(calls[0]!.init.headers).get('authorization'), 'Bearer access-synthetic');
  const body = JSON.parse(String(calls[0]!.init.body));
  assert.equal(body.store, false); assert.equal(body.stream, true); assert.equal(body.model, 'gpt-6.1-sol');
  assert.match(body.instructions, /^Use only the supplied context\./);
  assert.deepEqual(body.input.map((item: { role: string }) => item.role), ['user', 'assistant', 'user']);
  assert.equal(body.input[2].content[0].text, 'What was the word?');
});

test('identity, context and account mismatches stop before any provider request', async t => {
  const { runtime, calls } = await fixture(t, () => streamed(completed('no')));
  const signal = new AbortController().signal;
  assert.equal((await runtime.execute(contextInput({ ...base, attempt: { ...base.attempt, runId: 'other' } }), signal, dispatch)).kind, 'notStarted');
  assert.equal((await runtime.execute(contextInput({ ...base, attempt: { ...base.attempt, runtimeVersion: 'other' } }), signal, dispatch)).kind, 'notStarted');
  assert.equal((await runtime.execute(contextInput({ ...base, configuration: { ...base.configuration, account: { accountId: 'someone-else', userLabel: 'x@example.com' } } }), signal, dispatch)).kind, 'notStarted');
  assert.equal((await runtime.execute({ ...base, context: { kind: 'unavailable', message: 'Context module missing.' } }, signal, dispatch)).kind, 'notStarted');
  assert.equal(calls.length, 0);
});

test('a missing credential asks for sign-in without contacting OpenAI', async t => {
  const { runtime, calls } = await fixture(t, () => streamed(completed('no')), null);
  const result = await runtime.execute(contextInput(base), new AbortController().signal, dispatch);
  assert.equal(result.kind, 'notStarted');
  assert.match(result.kind === 'notStarted' ? result.message : '', /Sign in/);
  assert.equal(calls.length, 0);
});

test('an expiring token is refreshed and saved before the request', async t => {
  const { runtime, calls, tokenCalls, state } = await fixture(t, () => streamed(completed('ok')), { ...credential, expiresAt: now() + 30_000 });
  assert.equal((await runtime.execute(contextInput(base), new AbortController().signal, dispatch)).kind, 'completed');
  assert.equal(tokenCalls.length, 1);
  assert.equal(new Headers(calls[0]!.init.headers).get('authorization'), 'Bearer access-new');
  assert.equal((await readPlanCredential(state))?.refreshToken, 'refresh-new');
});

test('usage limits and provider failures are visible failures; a broken stream is unknown', async t => {
  const usage = await fixture(t, () => streamed(sse('response.failed', { response: { status: 'failed', error: { code: 'subscription_sharing_usage_limit_exceeded' } } })));
  assert.deepEqual(await usage.runtime.execute(contextInput(base), new AbortController().signal, dispatch), { kind: 'failed', message: 'Your ChatGPT plan usage limit was reached. Nothing else was tried.' });
  const refused = await fixture(t, () => new Response(JSON.stringify({ error: { code: 'subscription_sharing_usage_limit_exceeded' } }), { status: 429 }));
  assert.equal((await refused.runtime.execute(contextInput(base), new AbortController().signal, dispatch)).kind, 'failed');
  const broken = await fixture(t, () => streamed(sse('response.created'), sse('response.output_text.delta', { delta: 'part' })));
  assert.equal((await broken.runtime.execute(contextInput(base), new AbortController().signal, dispatch)).kind, 'outcomeUnknown');
  const network = await fixture(t, () => { throw new TypeError('fetch failed'); });
  assert.equal((await network.runtime.execute(contextInput(base), new AbortController().signal, dispatch)).kind, 'outcomeUnknown');
});

test('cancellation before dispatch sends nothing; during the stream it stops without a reply', async t => {
  const before = await fixture(t, () => streamed(completed('no')));
  const aborted = new AbortController(); aborted.abort();
  assert.equal((await before.runtime.execute(contextInput(base), aborted.signal, dispatch)).kind, 'notStarted');
  assert.equal(before.calls.length, 0);
  const during = new AbortController();
  const live = await fixture(t, (_url, init) => new Response(new ReadableStream({ start(controller) {
    controller.enqueue(new TextEncoder().encode(sse('response.created')));
    init.signal?.addEventListener('abort', () => controller.error(new DOMException('aborted', 'AbortError')));
    queueMicrotask(() => during.abort());
  } }), { status: 200 }));
  assert.equal((await live.runtime.execute(contextInput(base), during.signal, dispatch)).kind, 'canceled');
});

test('the model catalog lists the signed-in account models and refuses a different account', async t => {
  const { runtime, calls } = await fixture(t, url => url.endsWith('/v1/models') ? new Response(JSON.stringify({ data: [{ slug: 'gpt-6.1-sol', display_name: 'GPT-6.1 Sol', visibility: 'list' }, { slug: 'hidden', visibility: 'hide' }] })) : new Response('', { status: 404 }));
  const catalog = await runtime.models(new AbortController().signal, account);
  assert.equal(catalog.state, 'ready');
  assert.deepEqual(catalog.models, [{ id: 'gpt-6.1-sol', name: 'GPT-6.1 Sol', availability: 'available' }]);
  assert.equal(new Headers(calls[0]!.init.headers).get('authorization'), 'Bearer access-synthetic');
  assert.equal((await runtime.models(new AbortController().signal, { accountId: 'other', userLabel: 'other@example.com' })).state, 'error');
  assert.equal(calls.length, 1);
});
