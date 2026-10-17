import { test } from 'node:test';
import assert from 'node:assert/strict';
import { PassThrough, Writable } from 'node:stream';
import { CodexConnection } from './codex-rpc.ts';
import { CodexChat } from './codex-chat.ts';
import type { ChatInput } from './chat-runtime.ts';
import { syntheticConfig, syntheticFeatures } from './codex-preflight-fixture.ts';
import {redactedStartupFailure} from './startup-diagnostic.ts';

const input: ChatInput = { runId:'run-1', requestId:'request-1', conversationId:'home', text:'What was the sample word?', configuration:{ agentId:'writer', agentName:'Writer', bindingRevisionId:'binding', selectedModel:'gpt-6-astra', instructions:'Use only the supplied context.', skills:[{ skillId:'marker', revisionId:'r1', name:'Marker', instructions:'End with [CHECKED].' }] }, history:[{ messageId:'m1', requestId:'earlier', conversationId:'home', role:'user', text:'Remember cedar.', sequence:1, createdAt:'2026-10-16T00:00:00Z', recipientAgentId:'writer', recipientAgentName:'Writer' }] };
function transport(options: { model?: string; tool?: boolean; wrongModel?: boolean; afterThread?: () => void; config?: unknown; mcpPages?: readonly (readonly unknown[])[]; threadMcp?:unknown[]; features?: unknown[]; featurePages?: readonly (readonly unknown[])[]; threadFeatures?:unknown[]; cycleCursor?: boolean; rejectMethod?:string; afterTurn?:()=>void; interruption?:'confirmed'|'disconnect' } = {}) {
  const output = new PassThrough(); const calls: { method?: string; params?: Record<string, unknown>; id?: unknown }[] = []; let closed = false;
  const emit = (message: unknown) => output.write(JSON.stringify(message)+'\n');
  const stdin = new Writable({ write(chunk, _encoding, done) {
    const frame = JSON.parse(chunk.toString()); calls.push(frame);
    queueMicrotask(() => {
      const result = (value: unknown) => emit({ id:frame.id, result:value });
      if(frame.method===options.rejectMethod){emit({id:frame.id,error:{code:-32602,message:'synthetic-secret-must-not-report',data:{token:'synthetic-secret-must-not-report'}}});return;}
      switch (frame.method) {
        case 'initialize': result({ userAgent:'synthetic' }); break;
        case 'config/read': result({ config:options.config ?? syntheticConfig() }); break;
        case 'experimentalFeature/list': {
          if(frame.params.threadId&&options.threadFeatures){result({data:options.threadFeatures,nextCursor:null});break;}
          const pages=options.featurePages??[options.features??syntheticFeatures()],page=Number(frame.params.cursor??0);
          result({data:pages[page]??[],nextCursor:page+1<pages.length?String(page+1):null});break;
        }
        case 'mcpServerStatus/list': {
          if (frame.params.threadId && options.threadMcp) { result({data:options.threadMcp,nextCursor:null});break; }
          const pages = options.mcpPages ?? [[]], page = Number(frame.params.cursor ?? 0);
          result({ data:pages[page] ?? [],nextCursor:options.cycleCursor ? '0' : page+1<pages.length ? String(page+1) : null }); break;
        }
        case 'account/read': result({ account:{ type:'chatgpt' }, requiresOpenaiAuth:true }); break;
        case 'model/list': result({ data:[{ id:options.model ?? 'gpt-6-astra',model:options.model ?? 'gpt-6-astra' }], nextCursor:null }); break;
        case 'permissionProfile/list': result({data:[{id:'fort_candidate_b_no_files',description:null,allowed:true}],nextCursor:null});break;
        case 'thread/start': result({ thread:{ id:'thread', ephemeral:true }, model:options.wrongModel ? 'other' : frame.params.model, modelProvider:'openai', cwd:frame.params.cwd, approvalPolicy:'untrusted', approvalsReviewer:'user',activePermissionProfile:{id:'fort_candidate_b_no_files',extends:null},runtimeWorkspaceRoots:[],sandbox:{ type:'readOnly', networkAccess:false }, instructionSources:[] }); options.afterThread?.(); break;
        case 'turn/start':
          result({ turn:{ id:'turn', status:'inProgress' } });
          if(options.afterTurn){options.afterTurn();break;}
          if (options.tool) { emit({ id:93, method:'item/tool/call', params:{ threadId:'thread', turnId:'turn' } }); break; }
          emit({ method:'item/completed', params:{ threadId:'thread', turnId:'turn', item:{ id:'commentary', type:'agentMessage', phase:'commentary', text:'Do not save this provisional thought.' } } });
          emit({ method:'item/completed', params:{ threadId:'thread', turnId:'turn', item:{ id:'reply', type:'agentMessage', phase:'final_answer', text:'cedar [CHECKED]' } } });
          emit({ method:'turn/completed', params:{ threadId:'thread', turn:{ id:'turn', status:'completed', error:null } } }); break;
        case 'turn/interrupt': result({}); if(options.interruption==='confirmed')emit({method:'turn/completed',params:{threadId:'thread',turn:{id:'turn',status:'interrupted',error:null}}});else if(options.interruption==='disconnect')output.destroy();break;
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

test('no-turn preflight reports only the exact failed check, never provider payloads',async()=>{
  for(const [options,expected] of [
    [{rejectMethod:'initialize'},{stage:'protocol',check:'initialize'}],
    [{rejectMethod:'config/read'},{stage:'protocol',check:'config/read'}],
    [{config:{...syntheticConfig(),analytics:{enabled:'synthetic-secret-must-not-report'}}},{stage:'config',check:'analytics.enabled'}],
    [{config:{...syntheticConfig(),check_for_update_on_startup:undefined}},{stage:'config',check:'check_for_update_on_startup'}],
    [{features:syntheticFeatures().filter(feature=>feature.name!=='shell_tool')},{stage:'features',check:'shell_tool'}],
    [{mcpPages:[[{name:'synthetic-secret-must-not-report'}]]},{stage:'mcp',check:'inventory'}],
    [{model:'synthetic-secret-must-not-report'},{stage:'model',check:'availability'}],
  ] as const){
    const fixture=transport(options),runtime=new CodexChat({connection:fixture.connection,workspace:'/synthetic/empty',runtimeVersion:'0.157.1'});
    let failure:unknown;
    try{await runtime.preflight(new AbortController().signal);}catch(error){failure=error;}
    await runtime.dispose();
    assert.deepEqual(redactedStartupFailure(failure,{stage:'session',check:'runtime-preflight'}),expected);
    assert.equal(JSON.stringify(failure).includes('synthetic-secret-must-not-report'),false);
    assert.equal(fixture.calls.some(call=>call.method==='thread/start'||call.method==='turn/start'),false);
    assert.equal(fixture.closed,true);
  }
});

test('ambient servers, missing features, repeated cursors and thread-only MCP capabilities fail closed',async()=>{
  for(const options of [
    {config:{...syntheticConfig(),mcp_servers:{...syntheticConfig().mcp_servers,hidden:{enabled:true,env:{SECRET:'synthetic-do-not-report'}}}}},
    {features:syntheticFeatures().filter(feature=>feature.name!=='shell_tool')},
    {cycleCursor:true},
    {threadMcp:[{name:'thread-only',tools:{dangerous:{}}}]},
    {config:{...syntheticConfig(),model_providers:{openai:{base_url:'https://unapproved.invalid',env_key:'synthetic'}}}},
  ]) {
    const fixture=transport(options),runtime=new CodexChat({connection:fixture.connection,workspace:'/synthetic/empty',runtimeVersion:'0.157.1'});
    const result=await runtime.execute(input,new AbortController().signal);
    assert.equal(result.kind,'notStarted');assert.equal(fixture.calls.some(c=>c.method==='turn/start'),false);
    assert.equal(JSON.stringify(result).includes('synthetic-do-not-report'),false);
    await runtime.dispose();
  }
});

test('unconfirmed effective configuration stops before thread creation, MCP discovery or turn dispatch', async () => {
  const fixture = transport({ config:{ model_provider:'unexpected' } });
  const runtime = new CodexChat({ connection:fixture.connection,workspace:'/synthetic/empty',runtimeVersion:'0.157.1' });
  const result = await runtime.execute(input,new AbortController().signal);
  assert.equal(result.kind,'notStarted');
  assert.equal(fixture.calls.some(c => ['thread/start','mcpServerStatus/list','turn/start'].includes(c.method ?? '')),false);
  await runtime.dispose();
});

test('MCP inventory is checked through the final page before a thread or turn is allowed', async () => {
  const fixture = transport({ mcpPages:[[],[{ name:'unapproved',tools:{ dangerous:{} } }]] });
  const runtime = new CodexChat({ connection:fixture.connection,workspace:'/synthetic/empty',runtimeVersion:'0.157.1' });
  const result = await runtime.execute(input,new AbortController().signal);
  assert.equal(result.kind,'notStarted');
  assert.equal(fixture.calls.filter(c => c.method === 'mcpServerStatus/list').length,2);
  assert.equal(fixture.calls.some(c => ['thread/start','turn/start'].includes(c.method ?? '')),false);
  await runtime.dispose();
});

test('explicit no-turn preflight and later dispatch share one initialized connection', async () => {
  const fixture=transport();
  const runtime=new CodexChat({ connection:fixture.connection,workspace:'/synthetic/empty',runtimeVersion:'0.157.1' });
  assert.deepEqual(await runtime.preflight(new AbortController().signal),{ model:'gpt-6-astra',runtimeVersion:'0.157.1' });
  assert.equal(fixture.calls.some(c => ['thread/start','turn/start'].includes(c.method ?? '')),false);
  assert.equal((await runtime.execute(input,new AbortController().signal)).kind,'completed');
  assert.equal(fixture.calls.filter(c => c.method === 'initialize').length,1);
  assert.equal(fixture.calls.some(c => c.method==='mcpServerStatus/list' && c.params?.threadId==='thread'),true);
  await runtime.dispose();
});

test('ambient ChatGPT routing and telemetry exporters cannot bypass the selected provider controls',async()=>{
  for(const config of [
    {...syntheticConfig(),chatgpt_base_url:'https://unapproved.invalid/backend-api'},
    {...syntheticConfig(),openai_base_url:'https://unapproved.invalid/v1'},
    {...syntheticConfig(),otel:{exporter:{'otlp-http':{endpoint:'https://unapproved.invalid'}}}},
  ]){
    const fixture=transport({config}),runtime=new CodexChat({connection:fixture.connection,workspace:'/synthetic/empty',runtimeVersion:'0.157.1'});
    const result=await runtime.execute(input,new AbortController().signal);
    assert.equal(result.kind,'notStarted');assert.equal(fixture.calls.some(c=>c.method==='turn/start'),false);
    await runtime.dispose();
  }
});

test('cancellation after thread creation but before continuation never dispatches a turn', async () => {
  const abort = new AbortController();
  const fixture = transport({ afterThread:() => abort.abort() });
  const runtime = new CodexChat({ connection:fixture.connection, workspace:'/synthetic/empty', runtimeVersion:'0.157.1' });
  const result = await runtime.execute(input,abort.signal);
  assert.equal(fixture.calls.some(c => c.method === 'turn/start'),false);
  assert.equal(result.kind,'notStarted');
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


test('Codex 0.157.1 canonical inventory confirms disabled memory and images without legacy alias entries',async()=>{
  const features=syntheticFeatures().filter(feature=>!['memory_tool','imagegenext'].includes(feature.name));
  const fixture=transport({featurePages:[features.slice(0,7),features.slice(7)]}),runtime=new CodexChat({connection:fixture.connection,workspace:'/synthetic/empty',runtimeVersion:'0.157.1'});
  try{
    assert.deepEqual(await runtime.preflight(new AbortController().signal),{model:'gpt-6-astra',runtimeVersion:'0.157.1'});
    assert.equal(fixture.calls.some(call=>['thread/start','turn/start'].includes(call.method??'')),false);
    assert.equal((await runtime.execute(input,new AbortController().signal)).kind,'completed');
    assert.equal(fixture.calls.some(call=>call.method==='experimentalFeature/list'&&call.params?.threadId==='thread'),true);
  }finally{await runtime.dispose();}
});


for(const [alias,canonical] of [['memory_tool','memories'],['imagegenext','image_generation']] as const){
  for(const mode of ['enabled','missing','alias-only','renamed','alias-enabled','nonboolean','duplicate'] as const){
    test(`${alias} compatibility still refuses ${mode} inventory before a thread or turn`,async()=>{
      let features:{name:string;enabled:unknown}[]=syntheticFeatures();
      if(mode==='enabled')features=features.map(feature=>feature.name===canonical?{...feature,enabled:true}:feature);
      if(mode==='missing'||mode==='alias-only')features=features.filter(feature=>feature.name!==canonical);
      if(mode==='alias-only')features.push({name:alias,enabled:false});
      if(mode==='renamed')features=features.map(feature=>feature.name===canonical?{...feature,name:'unreviewed_'+canonical}:feature);
      if(mode==='alias-enabled')features.push({name:alias,enabled:true});
      if(mode==='nonboolean')features.push({name:alias,enabled:'false'});
      if(mode==='duplicate')features.push({name:canonical,enabled:false});
      const fixture=transport({featurePages:[features.slice(0,7),features.slice(7)]}),runtime=new CodexChat({connection:fixture.connection,workspace:'/synthetic/empty',runtimeVersion:'0.157.1'});
      let failure:unknown;
      try{await runtime.preflight(new AbortController().signal);}catch(error){failure=error;}
      finally{await runtime.dispose();}
      const check=mode==='nonboolean'||mode==='duplicate'?'inventory':mode==='alias-enabled'?alias:canonical;
      assert.deepEqual(redactedStartupFailure(failure,{stage:'session',check:'runtime-preflight'}),{stage:'features',check});
      assert.equal(fixture.calls.some(call=>['thread/start','turn/start','mcpServerStatus/list','account/read'].includes(call.method??'')),false);
      assert.equal(fixture.closed,true);
    });
  }
  test(`${canonical} is rechecked in thread scope before turn dispatch`,async()=>{
    const fixture=transport({threadFeatures:syntheticFeatures().map(feature=>feature.name===canonical?{...feature,enabled:true}:feature)});
    const runtime=new CodexChat({connection:fixture.connection,workspace:'/synthetic/empty',runtimeVersion:'0.157.1'});
    try{
      assert.equal((await runtime.execute(input,new AbortController().signal)).kind,'notStarted');
      assert.equal(fixture.calls.some(call=>call.method==='thread/start'),true);
      assert.equal(fixture.calls.some(call=>call.method==='turn/start'),false);
    }finally{await runtime.dispose();}
  });
}

for(const interruption of ['confirmed','disconnect'] as const)test(`cancellation requires correlated terminal evidence: ${interruption}`,async()=>{
  const cancellation=new AbortController();
  const fixture=transport({afterTurn:()=>cancellation.abort(),interruption}),runtime=new CodexChat({connection:fixture.connection,workspace:'/synthetic/empty',runtimeVersion:'0.157.1'});
  try{
    const result=await runtime.execute(input,cancellation.signal);
    assert.equal(result.kind,interruption==='confirmed'?'canceled':'outcomeUnknown');
    assert.equal(fixture.calls.filter(call=>call.method==='turn/interrupt').length,1);
    assert.equal(fixture.calls.filter(call=>call.method==='turn/start').length,1);
  }finally{await runtime.dispose();}
});
