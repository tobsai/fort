import { test } from 'node:test';
import assert from 'node:assert/strict';
import { PassThrough, Writable } from 'node:stream';
import { CodexConnection } from './codex-rpc.ts';
import { CodexChat } from './codex-chat.ts';
import type { ChatInput } from './chat-runtime.ts';

const input: ChatInput = { runId:'run-1', requestId:'request-1', conversationId:'home', text:'What was the sample word?', configuration:{ agentId:'writer', agentName:'Writer', bindingRevisionId:'binding', selectedModel:'gpt-6-astra', instructions:'Use only the supplied context.', skills:[{ skillId:'marker', revisionId:'r1', name:'Marker', instructions:'End with [CHECKED].' }] }, history:[{ messageId:'m1', requestId:'earlier', conversationId:'home', role:'user', text:'Remember cedar.', sequence:1, createdAt:'2026-10-16T00:00:00Z', recipientAgentId:'writer', recipientAgentName:'Writer' }] };
function transport(options: { model?: string; tool?: boolean; wrongModel?: boolean } = {}) {
  const output = new PassThrough(); const calls: { method?: string; params?: Record<string, unknown>; id?: unknown }[] = []; let closed = false;
  const emit = (message: unknown) => output.write(JSON.stringify(message)+'\n');
  const stdin = new Writable({ write(chunk, _encoding, done) {
    const frame = JSON.parse(chunk.toString()); calls.push(frame);
    queueMicrotask(() => {
      const result = (value: unknown) => emit({ id:frame.id, result:value });
      switch (frame.method) {
        case 'initialize': result({ userAgent:'synthetic' }); break;
        case 'account/read': result({ account:{ type:'chatgpt' }, requiresOpenaiAuth:true }); break;
        case 'model/list': result({ data:[{ id:options.model ?? 'gpt-6-astra' }], nextCursor:null }); break;
        case 'thread/start': result({ thread:{ id:'thread', ephemeral:true }, model:options.wrongModel ? 'other' : frame.params.model, modelProvider:'openai', cwd:frame.params.cwd, approvalPolicy:'untrusted', sandbox:{ type:'readOnly', networkAccess:false }, instructionSources:[] }); break;
        case 'turn/start':
          result({ turn:{ id:'turn', status:'inProgress' } });
          if (options.tool) { emit({ id:93, method:'item/tool/call', params:{ threadId:'thread', turnId:'turn' } }); break; }
          emit({ method:'item/completed', params:{ threadId:'thread', turnId:'turn', item:{ id:'commentary', type:'agentMessage', phase:'commentary', text:'Do not save this provisional thought.' } } });
          emit({ method:'item/completed', params:{ threadId:'thread', turnId:'turn', item:{ id:'reply', type:'agentMessage', phase:'final_answer', text:'cedar [CHECKED]' } } });
          emit({ method:'turn/completed', params:{ threadId:'thread', turn:{ id:'turn', status:'completed', error:null } } }); break;
        case 'turn/interrupt': result({}); break;
      }
    }); done();
  } });
  const connection = new CodexConnection({ input:stdin, output, close:() => { closed = true; output.destroy(); stdin.destroy(); } });
  return { connection, calls, get closed() { return closed; } };
}

test('synthetic Codex connection preserves selected instructions and conversation context without login or model substitution', async () => {
  const fixture = transport(); const runtime = new CodexChat({ connection:fixture.connection, workspace:'/synthetic/empty', runtimeVersion:'0.157.1' });
  const result = await runtime.execute(input, new AbortController().signal);
  assert.deepEqual(result, { kind:'completed', text:'cedar [CHECKED]', effectiveModel:null });
  const start = fixture.calls.find(c => c.method === 'thread/start')!;
  assert.equal(start.params?.model, 'gpt-6-astra');
  assert.deepEqual(start.params?.environments, []);
  assert.deepEqual(start.params?.dynamicTools, []);
  assert.equal(start.params?.ephemeral, true);
  assert.match(String(start.params?.developerInstructions), /End with \[CHECKED\]/);
  const turn = fixture.calls.find(c => c.method === 'turn/start')!;
  assert.deepEqual(turn.params?.environments, []);
  assert.match(JSON.stringify(turn.params?.input), /Remember cedar/);
  assert.equal(fixture.calls.some(c => c.method?.includes('login')), false);
  fixture.connection.close();
});

test('unavailable exact model, changed thread model, oversized context and tool requests fail without fallback', async () => {
  for (const kind of ['missing','changed','oversized','tool'] as const) {
    const fixture = transport({ model:kind === 'missing' ? 'other' : undefined, wrongModel:kind === 'changed', tool:kind === 'tool' });
    const runtime = new CodexChat({ connection:fixture.connection, workspace:'/synthetic/empty', runtimeVersion:'0.157.1' });
    const result = await runtime.execute(kind === 'oversized' ? { ...input, text:'x'.repeat(65_537) } : input, new AbortController().signal);
    assert.notEqual(result.kind, 'completed');
    if (kind !== 'tool') assert.equal(fixture.calls.some(c => c.method === 'turn/start'), false);
    if (kind === 'oversized') assert.equal(fixture.calls.length, 0);
    if (kind === 'tool' || kind === 'changed') assert.equal(fixture.closed, true);
    fixture.connection.close();
  }
});
