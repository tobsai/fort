import { createHash } from 'node:crypto';
import { CODEX_RUNTIME } from '../runner/codex-descriptor.ts';
import { DatabaseSync } from 'node:sqlite';
import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, stat, readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import {createServer} from 'node:net';
import { launchOwnedProcess } from '../runner/owned-process.ts';
import { CodexChat } from '../runner/codex-chat.ts';
import type { ChatInput, ChatRuntime } from '../runner/chat-runtime.ts';
import { createApplication } from './openai.ts';
import type { Acceptance, Agent, Conversation, InstructionSkill, Installation, Snapshot, Run, RoutineRevision, Approval, ScheduleRevision, SchedulePreview, ScheduleOccurrence, ScheduleDefinition, RequestCancellation, ConversationMemory, RuntimeReadiness } from '../contracts/v1.ts';

async function setup(t: TestContext, options: Partial<Omit<Parameters<typeof createApplication>[0],'dataDirectory'>> = {}) {
  const directory = await mkdtemp(join(tmpdir(), 'fort-rebuilt-unit-'));
  let service = createApplication({ provider: CODEX_RUNTIME, dataDirectory: directory, ...options });
  await service.start();
  t.after(async () => { await service.close(); await rm(directory, { recursive: true, force: true }); });
  const enrollment = service.issueEnrollment();
  const response = await fetch(service.baseURL + '/v1/devices/enroll', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...enrollment, deviceName: 'Synthetic test device' }) });
  assert.equal(response.status, 201);
  const device = await response.json() as { deviceToken: string };
  const headers = { 'Content-Type': 'application/json', Authorization: `Bearer ${device.deviceToken}`, 'X-Fort-Installation-Id': service.installation.installationId };
  async function api<T>(path: string, body?: unknown) {
    const response = await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers, body: body === undefined ? undefined : JSON.stringify(body) });
    return { status: response.status, body: await response.json() as T };
  }
  async function reenroll() {
    const response=await fetch(service.baseURL+'/v1/devices/enroll',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...service.issueEnrollment(),deviceName:'Fresh synthetic test device'})});
    assert.equal(response.status,201);const device=await response.json() as {deviceToken:string};headers.Authorization=`Bearer ${device.deviceToken}`;
  }
  return { api, reenroll, directory, get service() { return service; }, async restart() { await service.close(); service = createApplication({ provider: CODEX_RUNTIME, dataDirectory: directory, ...options }); await service.start(); } };
}

test('an explicitly selected stateless runtime owns its binding and exposes unsupported discovery without fallback', async t => {
  const h = await setup(t, { provider: {
    provider: 'local-stateless', name: 'Local stateless fixture', runtimeVersion: 'fixture-1',
    hostId: 'synthetic-host', session: 'stateless', capabilities: ['plainText']
  }, runtime: { async execute(input) { return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() {} } });
  const agent = (await h.api<Agent>('/v1/agents', { name: 'Local', model: 'exact-local-model', instructions: 'Echo.' })).body;
  assert.equal(agent.binding.provider, 'local-stateless');
  assert.equal(agent.binding.runtimeVersion, 'fixture-1');
  assert.equal(agent.binding.hostId, 'synthetic-host');
  const catalog = (await h.api<{ provider: string; state: string }>('/v1/models')).body;
  assert.equal(catalog.provider, 'local-stateless');
  assert.equal(catalog.state, 'unavailable');
  const path = `/v1/conversations/${agent.homeConversationId}`;
  const accepted = (await h.api<Acceptance>(path + '/messages', { requestId: 'neutral-runtime', text: '  Exact Unicode: cedar 🌲\n', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] })).body;
  await h.service.dispatchNext();
  await h.restart();
  const recovered = (await h.api<Acceptance>('/v1/requests/neutral-runtime')).body;
  assert.deepEqual(recovered.receipt, accepted.receipt);
  assert.equal(recovered.snapshot.runs[0]?.state, 'completed');
  assert.equal(recovered.snapshot.messages[1]?.text, '  Exact Unicode: cedar 🌲\n');
});

test('model discovery rejects evidence from a different provider', async t => {
  const h = await setup(t, { runtime: {
    async execute() { throw new Error('No turn should be started'); }, async dispose() {},
    async models() { return { provider: 'foreign-provider', state: 'ready', checkedAt: '2026-10-17T00:00:00Z', models: [{ id: 'foreign-model', name: 'Foreign model', availability: 'available' }] }; }
  } });
  const result = (await h.api<{ provider: string; state: string; models: unknown[] }>('/v1/models')).body;
  assert.equal(result.provider, 'codex');
  assert.equal(result.state, 'error');
  assert.deepEqual(result.models, []);
});

test('bounded invalidation replay waits for committed changes and resumes at the exact page cursor', async t => {
  const h = await setup(t);
  const installation = (await h.api<Installation & { eventCursor: number }>('/v1/installation')).body;
  assert.ok(installation.capabilities?.includes('eventReplay'));
  type Page = { semantics: string; cursor: number; events: { sequence: number; kind: string }[] };
  const query = `/v1/events?recoveryEpoch=${installation.recoveryEpoch}&after=`;
  const waiting = h.api<Page>(query + installation.eventCursor + '&wait=25000');
  await new Promise(resolve => setTimeout(resolve, 20));
  for (let i = 0; i < 90; i++) await h.api('/v1/agents', { name: 'Fixture ' + i, model: 'exact-model', instructions: 'Synthetic.' });
  const awakened = await waiting;
  assert.equal(awakened.status, 200);
  assert.equal(awakened.body.semantics, 'invalidation');
  assert.ok(awakened.body.events.length > 0);
  const first = (await h.api<Page>(query + installation.eventCursor)).body;
  assert.equal(first.events.length, 256);
  assert.equal(first.cursor, first.events.at(-1)?.sequence);
  const second = (await h.api<Page>(query + first.cursor)).body;
  assert.ok(second.events.length > 0 && second.events.length <= 256);
  assert.ok(second.events.every(event => event.sequence > first.cursor));
  assert.equal(new Set([...first.events, ...second.events].map(event => event.sequence)).size, first.events.length + second.events.length);
  const idle = (await h.api<Page>(query + second.cursor + '&wait=10')).body;
  assert.deepEqual(idle.events, []);
  assert.equal(idle.cursor, second.cursor);
  assert.equal((await h.api(query + second.cursor + '&wait=25001')).status, 400);
  assert.equal((await h.api(query + (second.cursor + 1))).status, 409);
  assert.equal((await h.api(`/v1/events?after=0&recoveryEpoch=foreign`)).status, 409);
});

test('event retention requires snapshot recovery and concurrent waits apply backpressure', async t => {
  const h = await setup(t);
  await h.service.close();
  const fixture = new DatabaseSync(join(h.directory, 'openai-candidate-b.sqlite'));
  fixture.exec('BEGIN');
  const insert = fixture.prepare('INSERT INTO events(data) VALUES (?)');
  for (let i = 0; i < 10_050; i++) insert.run(JSON.stringify({ schemaVersion: 1, installationId: h.service.installation.installationId, recoveryEpoch: h.service.installation.recoveryEpoch, eventId: 'fixture-' + i, entityId: 'workspace', conversationId: '_workspace', kind: 'workspaceChanged', createdAt: '2026-10-18T00:00:00Z', data: {} }));
  fixture.exec('COMMIT'); fixture.close();
  await h.restart();
  await h.api('/v1/agents', { name: 'Trim fixture', model: 'exact-model', instructions: 'Synthetic.' });
  const installation = (await h.api<Installation>('/v1/installation')).body;
  const query = `/v1/events?recoveryEpoch=${installation.recoveryEpoch}&after=`;
  const expired = await h.api<{ error: { code: string } }>(query + '0');
  assert.equal(expired.status, 409); assert.equal(expired.body.error.code, 'cursorExpired');
  assert.equal((await h.api(query + installation.eventCursor)).status, 200);
  const waits = Array.from({ length: 8 }, () => h.api(query + installation.eventCursor + '&wait=25000'));
  await new Promise(resolve => setTimeout(resolve, 50));
  assert.equal((await h.api(query + installation.eventCursor + '&wait=1')).status, 429);
  await h.api('/v1/agents', { name: 'Wake fixture', model: 'exact-model', instructions: 'Synthetic.' });
  assert.ok((await Promise.all(waits)).every(response => response.status === 200));
  const latest = (await h.api<Installation>('/v1/installation')).body;
  assert.equal((await h.api(query + latest.eventCursor + '&wait=1')).status, 200);
});

test('reopening an installation with another runtime identity fails before dispatch', async t => {
  const h = await setup(t);
  await h.api('/v1/agents', { name: 'Writer', model: 'exact-model', instructions: 'Reply.' });
  await h.service.close();
  let replacement: ReturnType<typeof createApplication> | undefined;
  try {
    assert.throws(() => { replacement = createApplication({ dataDirectory: h.directory, provider: { ...CODEX_RUNTIME, provider: 'different-provider' } }); }, /runtime identity/i);
  } finally { await replacement?.close(); }
});

test('two hundred durable turns page exact replay within a four MiB ledger budget', async t => {
  let lastInput: ChatInput | undefined;
  const h = await setup(t, { runtime: {
    async execute(input) { lastInput = input; return { kind: 'completed', text: 'r'.repeat(1024), effectiveModel: null }; },
    async dispose() {}
  } });
  const agent = (await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Reply.' })).body;
  const path = `/v1/conversations/${agent.homeConversationId}`;
  let first: Acceptance | undefined;
  for (let index = 0; index < 200; index++) {
    const accepted = await h.api<Acceptance>(path + '/messages', { requestId: 'compact-' + index, text: String(index).padStart(4, '0') + 'u'.repeat(1020), recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] });
    assert.equal(accepted.status, 201);
    first ??= accepted.body;
    assert.equal(await h.service.dispatchNext(), true);
  }
  assert.ok(first);
  assert.equal(lastInput?.history.length, 398);
  assert.equal(lastInput?.history[0]?.text, '0000' + 'u'.repeat(1020));
  await h.restart();
  const recovered = (await h.api<Acceptance>('/v1/requests/compact-0')).body;
  assert.deepEqual(recovered.receipt, first.receipt);
  assert.equal(recovered.snapshot.messages.length, 100);
  assert.equal(recovered.snapshot.messages.at(-1)?.text, 'r'.repeat(1024));
  assert.equal(recovered.message?.text, '0000' + 'u'.repeat(1020));
  const messages = [...recovered.snapshot.messages];
  let before = recovered.snapshot.window?.olderBefore;
  let pages = 1;
  while (before !== null && before !== undefined) {
    const boundary = before;
    const page = (await h.api<Snapshot>(path + '/snapshot?before=' + boundary)).body;
    assert.equal(page.messages.length, 100);
    assert.equal(page.window?.before, before);
    assert.ok(page.messages.every(message => message.sequence < boundary));
    messages.unshift(...page.messages);
    before = page.window?.olderBefore;
    assert.ok(++pages <= 4);
  }
  assert.equal(pages, 4);
  assert.equal(messages.length, 400);
  assert.equal(new Set(messages.map(message => message.messageId)).size, 400);
  assert.equal(messages[0]?.text, '0000' + 'u'.repeat(1020));
  const read = await h.api(path + '/read', { messageId: messages[0]?.messageId, cursor: recovered.snapshot.cursor, recoveryEpoch: h.service.installation.recoveryEpoch });
  assert.equal(read.status, 200);
  await h.service.close();
  const bytes = (await stat(join(h.directory, 'openai-candidate-b.sqlite'))).size;
  assert.ok(bytes <= 4 * 1024 * 1024, `ledger used ${bytes} bytes for 200 synthetic turns`);
  t.diagnostic(`200 turns, 400 one-KiB messages: ${bytes} database bytes after close`);
});

test('conversation queue admission stays bounded and cancellation releases capacity without changing receipts', async t => {
  const h = await setup(t);
  const agent = (await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'exact-model', instructions: 'Reply.' })).body;
  const path = `/v1/conversations/${agent.homeConversationId}`;
  const command = (index: number) => ({ requestId: 'queued-' + index, text: 'Synthetic queue item', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] });
  let first: Acceptance | undefined;
  for (let index = 0; index < 100; index++) {
    const accepted = await h.api<Acceptance>(path + '/messages', command(index));
    assert.equal(accepted.status, 201);
    first ??= accepted.body;
  }
  assert.ok(first);
  assert.equal((await h.api(path + '/messages', command(100))).status, 429);
  assert.deepEqual((await h.api<Acceptance>(path + '/messages', command(0))).body.receipt, first.receipt);
  assert.equal((await h.api(`/v1/runs/${first.receipt.runId}/cancel`, { recoveryEpoch: h.service.installation.recoveryEpoch })).status, 200);
  assert.equal((await h.api(path + '/messages', command(100))).status, 201);
  const snapshot = (await h.api<Snapshot>(path + '/snapshot')).body;
  assert.equal(snapshot.messages.length, 100);
  assert.equal(snapshot.runs.filter(run => run.state === 'queued').length, 100);
});

test('runtime dispatch carries the exact durable service attempt and generation', async t => {
  const calls: ChatInput[] = [];
  const observedRuns: Run[] = [];
  const h = await setup(t, { runtime: {
    async execute(input) {
      calls.push(input);
      const snapshot = (await h.api<Snapshot>(`/v1/conversations/${input.conversationId}/snapshot`)).body;
      const running = snapshot.runs.find(run => run.runId === input.runId);
      assert.ok(running);
      observedRuns.push(running);
      return { kind: 'completed', text: 'Correlated synthetic result', effectiveModel: null };
    },
    async dispose() {}
  } });
  const agent = (await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Reply.' })).body;
  const path = `/v1/conversations/${agent.homeConversationId}`;
  const command = { requestId: 'correlated-attempt', text: 'Synthetic request', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] };
  assert.equal((await h.api(path + '/messages', command)).status, 201);
  await h.service.dispatchNext();
  const completed = (await h.api<Snapshot>(path + '/snapshot')).body;
  assert.equal(completed.runs[0]?.state, 'completed');
  const running = observedRuns[0];
  assert.ok(running);
  assert.equal(running.state, 'running');
  assert.deepEqual(calls[0]?.attempt, {
    runId: running.runId, attemptId: running.attemptId, generation: 1,
    bindingRevisionId: running.bindingRevisionId, selectedModel: running.selectedModel,
    runtimeVersion: '0.157.1'
  });
  assert.equal(completed.runs[0]?.attemptId, calls[0]?.attempt.attemptId);
  await h.api(path + '/messages', command);
  assert.equal(await h.service.dispatchNext(), false);
  assert.equal(calls.length, 1);
  await h.restart();
  assert.equal((await h.api<Snapshot>(path + '/snapshot')).body.runs[0]?.attemptId, completed.runs[0]?.attemptId);
});

test('explicit authenticated runtime replacement recovers a new queued turn without retrying the rejected model request',async t=>{
  const workspace=fileURLToPath(new URL('../runner',import.meta.url)),processes:ReturnType<typeof launchOwnedProcess>[]=[];
  const launch=()=>{const child=launchOwnedProcess({executable:process.execPath,args:[fileURLToPath(new URL('../runner/owned-process-fixture.mjs',import.meta.url))],cwd:workspace,env:{PATH:'/usr/bin:/bin',FORT_SYNTHETIC_MODE:'complete'},deadlineMs:15_000,terminationGraceMs:50,killGraceMs:500});processes.push(child);return child.connection;};
  t.after(async()=>{for(const child of processes)await child.dispose();});
  const runtime=new CodexChat({connection:launch(),workspace,runtimeVersion:'0.157.1',replacement:{maximumConnections:2,deadline:Date.now()+20_000,connect:()=>{assert.equal(processes[0]?.cleanupConfirmed,true);return launch();}}});
  const h=await setup(t,{runtime});let agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'unavailable-exact-model',instructions:'Reply.'})).body;
  const epoch=h.service.installation.recoveryEpoch,path=`/v1/conversations/${agent.homeConversationId}`;
  const send=(requestId:string)=>h.api(path+'/messages',{requestId,text:'Synthetic '+requestId,recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:epoch,skills:[]});
  await send('rejected-model');await h.service.dispatchNext();
  const readiness=await h.api<RuntimeReadiness>('/v1/runtime');assert.equal(readiness.status,200);assert.equal(readiness.body.state,'restartRequired');assert.equal(readiness.body.canRestart,true);
  agent=(await h.api<Agent>(`/v1/agents/${agent.agentId}`,{expectedBindingRevisionId:agent.binding.bindingRevisionId,name:agent.name,model:'gpt-6-astra',instructions:'Reply.'})).body;
  await send('new-valid-request');assert.equal(await h.service.dispatchNext(),false);
  assert.equal((await h.api('/v1/runtime/restart',{recoveryEpoch:'wrong'})).status,409);assert.equal(processes.length,1);
  assert.equal((await h.api<RuntimeReadiness>('/v1/runtime/restart',{recoveryEpoch:epoch})).body.state,'ready');
  await h.service.dispatchNext();const snapshot=(await h.api<Snapshot>(path+'/snapshot')).body;
  assert.deepEqual(snapshot.runs.map(run=>[run.requestId,run.state]),[['rejected-model','failed'],['new-valid-request','completed']]);
  assert.equal(snapshot.messages.filter(message=>message.role==='assistant').length,1);assert.equal(h.service.inspectSession().receipts.length,2);assert.equal(processes.length,2);
  assert.equal((await h.api('/v1/runtime/restart',{recoveryEpoch:epoch})).status,409);await h.service.close();assert.ok(processes.every(child=>child.cleanupConfirmed));
});

test('oversized composed routine configuration cannot partially commit manual or scheduled acceptance',async t=>{
  let now=Date.parse('2026-10-17T08:59:30Z'),calls=0;
  const h=await setup(t,{now:()=>now,runtime:{async execute(){calls++;return {kind:'completed',text:'Healthy subsequent work',effectiveModel:null};},async dispose(){}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Calendar',model:'gpt-6-astra',instructions:'a'.repeat(9000)})).body,epoch=h.service.installation.recoveryEpoch;
  const definition={schemaVersion:1,name:'Combined limit',agentId:agent.agentId,conversationId:agent.homeConversationId,bindingRevisionId:agent.binding.bindingRevisionId,instructions:'r'.repeat(9000),inputs:[],skills:[],steps:[{stepId:'task',name:'Task',type:'task',instructions:'Write.',inputNames:[],outputStepIds:[]}]};
  const routine=(await h.api<RoutineRevision>('/v1/routines',{definition})).body;
  const invalid=await h.api<{error:{code:string}}>(`/v1/routines/${routine.routineId}/run`,{requestId:'invalid-combined',routineRevisionId:routine.revisionId,recoveryEpoch:epoch,inputs:{}});
  assert.equal(invalid.status,400);
  const saved=(await h.api<ScheduleRevision>('/v1/schedules',{definition:{schemaVersion:1,routineId:routine.routineId,routineRevisionId:routine.revisionId,kind:'daily',timezone:'UTC',time:'09:00',inputs:{},missed:'skip'}})).body;
  const path=`/v1/schedules/${saved.scheduleId}`,preview=(await h.api<SchedulePreview>(path+'/preview')).body;
  await h.api(path+'/state',{recoveryEpoch:epoch,revisionId:saved.revisionId,previewDigest:preview.digest,enable:true});
  const before=(await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body;
  now+=60_000;assert.equal(h.service.tickSchedules(),0);
  const after=await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`);assert.equal(after.status,200);assert.deepEqual({...after.body,cursor:before.cursor},before);assert.ok(after.body.cursor > before.cursor);
  assert.equal(invalid.body.error.code,'invalidConfiguration');
  const inspection=h.service.inspectSession();assert.equal(inspection.runs.length,0);assert.equal(inspection.receipts.length,0);assert.equal(calls,0);
  const occurrence=(await h.api<{occurrences:ScheduleOccurrence[]}>(path+'/occurrences')).body.occurrences;assert.equal(occurrence.length,1);assert.equal(occurrence[0]?.state,'blocked');
  await h.restart();assert.equal(h.service.inspectSession().runs.length,0);
  const healthy=(await h.api<RoutineRevision>(`/v1/routines/${routine.routineId}`,{expectedRevisionId:routine.revisionId,definition:{...definition,instructions:'Use declared inputs.'}})).body;
  const revised=(await h.api<ScheduleRevision>(path,{expectedRevisionId:saved.revisionId,definition:{...saved.definition,routineRevisionId:healthy.revisionId,time:'09:01'}})).body;
  const nextPreview=(await h.api<SchedulePreview>(path+'/preview')).body;
  await h.api(path+'/state',{recoveryEpoch:epoch,revisionId:revised.revisionId,previewDigest:nextPreview.digest,enable:true});
  now+=60_000;assert.equal(h.service.tickSchedules(),1);await h.service.dispatchNext();assert.equal(calls,1);assert.equal(h.service.inspectSession().receipts.length,1);
});

test('owner memory is conversation-scoped, revision-pinned at acceptance and durable across restart',async t=>{
  const inputs:ChatInput[]=[];const h=await setup(t,{runtime:{async execute(input){inputs.push(structuredClone(input));return {kind:'completed',text:'Synthetic reply',effectiveModel:null};},async dispose(){}}});
  const writer=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Use supplied context.'})).body;
  const reviewer=(await h.api<Agent>('/v1/agents',{name:'Reviewer',model:'gpt-6-astra',instructions:'Review supplied context.'})).body;
  const room=(await h.api<Conversation>('/v1/conversations',{name:'Sample planning',participantAgentIds:[writer.agentId,reviewer.agentId]})).body,epoch=h.service.installation.recoveryEpoch;
  const home=`/v1/conversations/${writer.homeConversationId}`,shared=`/v1/conversations/${room.conversationId}`;
  const first=await h.api<ConversationMemory>(home+'/memory',{expectedRevisionId:null,recoveryEpoch:epoch,text:'  Private sample word: cedar.\r\n'});assert.equal(first.status,200);
  const sharedMemory=(await h.api<ConversationMemory>(shared+'/memory',{expectedRevisionId:null,recoveryEpoch:epoch,text:'Shared sample item: red notebook.'})).body;
  const command={requestId:'memory-before-edit',text:'Use my saved note.',recipientAgentId:writer.agentId,bindingRevisionId:writer.binding.bindingRevisionId,recoveryEpoch:epoch,skills:[],memoryRevisionId:first.body.revisionId};
  const accepted=(await h.api<Acceptance>(home+'/messages',command)).body;
  const changed=await h.api<ConversationMemory>(home+'/memory',{expectedRevisionId:first.body.revisionId,recoveryEpoch:epoch,text:'New sample word: birch.'});assert.equal(changed.status,200);
  assert.equal((await h.api(home+'/memory',{expectedRevisionId:first.body.revisionId,recoveryEpoch:epoch,text:'Stale replacement.'})).status,409);
  assert.equal((await h.api(home+'/messages',{...command,requestId:'stale-memory'})).status,409);
  assert.equal((await h.api(home+'/messages',{...command,requestId:'missing-memory',memoryRevisionId:undefined})).status,409);
  const repeated = (await h.api<Acceptance>(home+'/messages',command)).body;
  assert.deepEqual(repeated.receipt,accepted.receipt);
  assert.deepEqual(repeated.snapshot.memory,changed.body);
  await h.service.dispatchNext();assert.deepEqual(inputs[0]?.configuration.memory,first.body);
  for(const [agent,requestId] of [[writer,'shared-writer'],[reviewer,'shared-reviewer']] as const){
    assert.equal((await h.api(shared+'/messages',{...command,requestId,recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,memoryRevisionId:sharedMemory.revisionId})).status,201);await h.service.dispatchNext();
  }
  assert.ok(inputs.slice(1).every(input=>input.configuration.memory?.text==='Shared sample item: red notebook.'));
  assert.equal(JSON.stringify(inputs.slice(1)).includes('Private sample word'),false);
  assert.equal((await h.api(`/v1/conversations/${reviewer.homeConversationId}/messages`,{...command,requestId:'other-home',recipientAgentId:reviewer.agentId,bindingRevisionId:reviewer.binding.bindingRevisionId,memoryRevisionId:undefined})).status,201);await h.service.dispatchNext();assert.equal(inputs[3]?.configuration.memory,undefined);
  await h.restart();assert.deepEqual((await h.api<Snapshot>(home+'/snapshot')).body.memory,changed.body);
  const cleared=await h.api<ConversationMemory>(home+'/memory',{expectedRevisionId:changed.body.revisionId,recoveryEpoch:epoch,text:''});assert.equal(cleared.status,200);assert.equal(cleared.body.text,'');
  assert.equal((await h.api(home+'/messages',{...command,requestId:'cleared-memory',memoryRevisionId:cleared.body.revisionId})).status,201);await h.service.dispatchNext();assert.equal(inputs[4]?.configuration.memory?.text,'');
  const snapshot=(await h.api<Snapshot>(home+'/snapshot')).body;assert.equal(snapshot.runs[0]?.configuration?.memory?.text,first.body.text);assert.equal(snapshot.messages.length,4);
  assert.equal((await h.api(home+'/memory',{expectedRevisionId:cleared.body.revisionId,recoveryEpoch:'old',text:'Wrong epoch.'})).status,409);
  assert.equal((await h.api(home+'/memory',{expectedRevisionId:cleared.body.revisionId,recoveryEpoch:epoch,text:'x'.repeat(8193)})).status,400);
});

test('request cancellation persists before acceptance and fences late chat or routine submission after restart',async t=>{
  const h=await setup(t),agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Write.'})).body,epoch=h.service.installation.recoveryEpoch;
  const command={requestId:'cancel-before-accept',recoveryEpoch:epoch,recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,text:'Late original message',skills:[]};
  const scope={recoveryEpoch:epoch,conversationId:agent.homeConversationId,bindingRevisionId:agent.binding.bindingRevisionId},path=`/v1/requests/${command.requestId}/cancel`;
  const canceled=await h.api<RequestCancellation>(path,scope);assert.equal(canceled.status,200);assert.equal(canceled.body.status,'notAccepted');assert.equal(canceled.body.requestId,command.requestId);
  await h.restart();assert.deepEqual((await h.api<RequestCancellation>(path,scope)).body,canceled.body);
  const late=await h.api<{error:{code:string}}>(`/v1/conversations/${agent.homeConversationId}/messages`,{...command,recipientAgentId:agent.agentId});assert.equal(late.status,409);assert.equal(late.body.error.code,'requestCanceled');
  assert.equal((await h.api(path,{...scope,bindingRevisionId:'different'})).status,409);
  assert.equal((await h.api(path,{...scope,recoveryEpoch:'old-epoch'})).status,409);
  const routine=(await h.api<RoutineRevision>('/v1/routines',{definition:{schemaVersion:1,name:'Canceled task',agentId:agent.agentId,conversationId:agent.homeConversationId,bindingRevisionId:agent.binding.bindingRevisionId,instructions:'Use declared inputs.',inputs:[],skills:[],steps:[{stepId:'task',name:'Task',type:'task',instructions:'Write.',inputNames:[],outputStepIds:[]}]}})).body;
  assert.equal((await h.api(`/v1/routines/${routine.routineId}/run`,{requestId:command.requestId,routineRevisionId:routine.revisionId,recoveryEpoch:epoch,inputs:{}})).status,409);
  assert.equal((await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body.runs.length,0);
});

test('request cancellation reconciles a lost acceptance and a submit race without duplicate or late dispatch',async t=>{
  let calls=0;const h=await setup(t,{runtime:{async execute(){calls++;return {kind:'completed',text:'Synthetic',effectiveModel:null};},async dispose(){}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Write.'})).body,epoch=h.service.installation.recoveryEpoch;
  const scope={recoveryEpoch:epoch,conversationId:agent.homeConversationId,bindingRevisionId:agent.binding.bindingRevisionId};
  const command={requestId:'lost-acceptance-cancel',text:'Synthetic sample',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:epoch,skills:[]},path=`/v1/conversations/${agent.homeConversationId}/messages`;
  const accepted=(await h.api<Acceptance>(path,command)).body;
  assert.equal((await h.api(`/v1/requests/${command.requestId}/cancel`,{...scope,conversationId:'wrong-home'})).status,409);
  const canceled=await h.api<RequestCancellation>(`/v1/requests/${command.requestId}/cancel`,scope);assert.equal(canceled.status,200);assert.equal(canceled.body.status,'accepted');
  if(canceled.body.status!=='accepted')throw new Error('Expected accepted cancellation');
  assert.deepEqual(canceled.body.receipt,accepted.receipt);assert.equal(canceled.body.run.state,'canceled');
  await h.restart();assert.equal((await h.api<Acceptance>(path,command)).body.receipt.runId,accepted.receipt.runId);
  for(let index=0;index<12;index++){
    const requestId='cancel-race-'+index;
    const [submission,cancellation]=await Promise.all([h.api<Acceptance>(path,{...command,requestId}),h.api<RequestCancellation>(`/v1/requests/${requestId}/cancel`,scope)]);
    assert.equal(cancellation.status,200);assert.ok(submission.status===201||submission.status===409);
    if(submission.status===201){assert.equal(cancellation.body.status,'accepted');if(cancellation.body.status==='accepted')assert.equal(cancellation.body.run.state,'canceled');}
    else assert.equal(cancellation.body.status,'notAccepted');
  }
  assert.equal(await h.service.dispatchNext(),false);assert.equal(calls,0);
  assert.ok((await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body.runs.every(run=>run.state==='canceled'));
});

test('an old instruction record migrates only by an explicit save and review, retaining its original revision',async t=>{
  const h=await setup(t),agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Write.'})).body;
  const legacy={skillId:'legacy',revisionId:'legacy-revision',name:'Historical skill',instructions:'  Legacy exact instructions.\r\n'};
  await h.service.close();
  const db=new DatabaseSync(join(h.directory,'openai-candidate-b.sqlite'));db.prepare('INSERT INTO entities VALUES (?,?,?)').run('skills',legacy.skillId,JSON.stringify(legacy));db.close();
  await h.restart();assert.deepEqual((await h.api<{skills:InstructionSkill[]}>('/v1/skills')).body.skills,[legacy]);
  const command={requestId:'legacy-new-use',text:'Synthetic',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,skills:[{skillId:legacy.skillId,revisionId:legacy.revisionId}]},route=`/v1/conversations/${agent.homeConversationId}/messages`;
  assert.equal((await h.api(route,command)).status,409);
  const migrated=await h.api<InstructionSkill>('/v1/skills/legacy',{expectedRevisionId:legacy.revisionId,name:legacy.name,instructions:legacy.instructions});assert.equal(migrated.status,200);assert.equal(migrated.body.review?.state,'reviewRequired');
  assert.ok(migrated.body.bundle);assert.equal(migrated.body.instructions,legacy.instructions);assert.equal(migrated.body.previousRevisionId,legacy.revisionId);
  assert.deepEqual((await h.api<InstructionSkill>('/v1/skills/legacy/revisions/legacy-revision')).body,legacy);
  assert.ok(migrated.body.bundle);await h.api('/v1/skills/legacy/review',{revisionId:migrated.body.revisionId,digest:migrated.body.bundle.digest,enable:true});
  assert.equal((await h.api(route,{...command,skills:[{skillId:legacy.skillId,revisionId:migrated.body.revisionId}]})).status,201);
});

test('native instruction authoring creates a complete inert revision and every edit requires exact review',async t=>{
  const h=await setup(t),agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Write.'})).body;
  const created=await h.api<InstructionSkill>('/v1/skills',{name:'My instructions',instructions:'  Preserve Café e\u0301\r\n'});assert.equal(created.status,201);
  const skill=created.body;assert.equal(skill.review?.state,'reviewRequired');assert.equal(skill.bundle?.files.length,1);
  assert.equal(Buffer.from(skill.bundle!.files[0]!.contentBase64,'base64').toString('utf8'),skill.instructions);
  assert.equal(skill.bundle?.license,'LicenseRef-Owner-Private');assert.deepEqual(skill.bundle?.requestedTools,[]);
  const command={requestId:'new-instructions',text:'Synthetic sample',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,skills:[{skillId:skill.skillId,revisionId:skill.revisionId}]},path=`/v1/conversations/${agent.homeConversationId}/messages`;
  assert.equal((await h.api(path,command)).status,409);
  const enabled=(await h.api<InstructionSkill>(`/v1/skills/${skill.skillId}/review`,{revisionId:skill.revisionId,digest:skill.bundle!.digest,enable:true})).body;
  assert.equal((await h.api(path,command)).status,201);
  const changed=await h.api<InstructionSkill>(`/v1/skills/${skill.skillId}`,{expectedRevisionId:skill.revisionId,name:'My updated instructions',instructions:'  Changed exact text.  '});assert.equal(changed.status,200);
  assert.ok(changed.body.bundle);assert.equal(changed.body.review?.state,'reviewRequired');assert.equal(changed.body.previousRevisionId,skill.revisionId);
  assert.equal(changed.body.bundle?.license,skill.bundle?.license);assert.equal(changed.body.bundle?.source,skill.bundle?.source);
  assert.deepEqual((await h.api<InstructionSkill>(`/v1/skills/${skill.skillId}/revisions/${skill.revisionId}`)).body,enabled);
  assert.equal((await h.api(path,{...command,requestId:'new-revision',skills:[{skillId:skill.skillId,revisionId:changed.body.revisionId}]})).status,409);
  assert.equal((await h.api(path,command)).status,201);
});

async function scheduleFixture(t:TestContext,clock:()=>number,calendar:Omit<ScheduleDefinition,'schemaVersion'|'routineId'|'routineRevisionId'|'inputs'>,gate=false,autoRun=false){
  const h=await setup(t,{now:clock,autoRun}),agent=(await h.api<Agent>('/v1/agents',{name:'Calendar',model:'gpt-6-astra',instructions:'Use synthetic inputs.'})).body;
  const routine=(await h.api<RoutineRevision>('/v1/routines',{definition:{schemaVersion:1,name:'Calendar routine',agentId:agent.agentId,conversationId:agent.homeConversationId,bindingRevisionId:agent.binding.bindingRevisionId,instructions:'Only declared steps.',inputs:[],skills:[],steps:gate?[{stepId:'gate',name:'Review',type:'approval',prompt:'Continue?'}]:[{stepId:'task',name:'Task',type:'task',instructions:'Return a summary.',inputNames:[],outputStepIds:[]}]}})).body;
  const definition={schemaVersion:1,routineId:routine.routineId,routineRevisionId:routine.revisionId,inputs:{},...calendar};
  const saved=await h.api<ScheduleRevision>('/v1/schedules',{definition});assert.equal(saved.status,201);
  const path=`/v1/schedules/${saved.body.scheduleId}`,preview=await h.api<SchedulePreview>(path+'/preview');assert.equal(preview.status,200);
  assert.equal((await h.api(path+'/state',{recoveryEpoch:h.service.installation.recoveryEpoch,revisionId:saved.body.revisionId,previewDigest:preview.body.digest,enable:true})).status,200);
  return {h,agent,routine,schedule:saved.body,path,definition,preview:preview.body,async occurrences(){return (await h.api<{occurrences:ScheduleOccurrence[]}>(path+'/occurrences')).body.occurrences;}};
}

test('idle scheduling makes no writes and an on-time occurrence is not missed after idle time', async t => {
  let now = Date.parse('2026-10-17T08:00:00Z');
  const f = await scheduleFixture(t, () => now, { kind: 'daily', timezone: 'UTC', time: '09:00', missed: 'skip' });
  const wal = join(f.h.directory, 'openai-candidate-b.sqlite-wal');
  const walDigest = async () => createHash('sha256').update(await readFile(wal)).digest('hex');
  const before = await walDigest();
  for (let second = 0; second < 60; second++) { now += 1000; assert.equal(f.h.service.tickSchedules(), 0); }
  assert.equal(await walDigest(), before);
  assert.deepEqual(await f.occurrences(), []);
  now = Date.parse('2026-10-17T09:00:00Z');
  assert.equal(f.h.service.tickSchedules(), 1);
  assert.equal((await f.occurrences())[0]?.state, 'accepted');
  assert.equal(f.h.service.tickSchedules(), 0);
});

test('automatic scheduling exposes an idle state without a recurring wakeup', async t => {
  const h = await setup(t, { autoRun: true });
  const status = await h.api('/v1/scheduler');
  assert.equal(status.status, 200);
  assert.deepEqual(status.body, { state: 'idle' });
});

test('enabling a schedule arms its due time, claims once automatically, and removes the completed timer', async t => {
  const start = performance.now();
  const clock = () => Date.parse('2026-10-17T08:59:58Z') + Math.floor(performance.now() - start);
  const f = await scheduleFixture(t, clock, { kind: 'once', date: '2026-10-17', timezone: 'UTC', time: '09:00', missed: 'skip' }, false, true);
  const status = (await f.h.api<{ state: string; nextWakeAt: string }>('/v1/scheduler')).body;
  assert.equal(status.state, 'waiting');
  assert.ok(Math.abs(Date.parse(status.nextWakeAt) - Date.parse('2026-10-17T09:00:00Z')) < 20);
  let occurrences = await f.occurrences();
  while (!occurrences.length && performance.now() - start < 5000) {
    await new Promise(resolve => setTimeout(resolve, 20));
    occurrences = await f.occurrences();
  }
  assert.deepEqual(occurrences.map(row => row.state), ['accepted']);
  assert.deepEqual((await f.h.api('/v1/scheduler')).body, { state: 'idle' });
  assert.equal(f.h.service.tickSchedules(), 0);
});

test('a persisted schedule fault remains visible with a bounded retry instead of stopping silently', async t => {
  const now = Date.parse('2026-10-17T08:00:00Z');
  const f = await scheduleFixture(t, () => now, { kind: 'daily', timezone: 'UTC', time: '09:00', missed: 'skip' }, false, true);
  await f.h.service.close();
  const fixture = new DatabaseSync(join(f.h.directory, 'openai-candidate-b.sqlite'));
  fixture.prepare("UPDATE entities SET data=json_set(data,'$.digest','invalid') WHERE kind='schedules' AND id=?").run(f.schedule.scheduleId);
  fixture.close();
  await f.h.restart();
  const status = (await f.h.api<{ state: string; retryAt: string; detail: string }>('/v1/scheduler')).body;
  assert.equal(status.state, 'fault');
  assert.equal(Date.parse(status.retryAt) - now, 60_000);
  assert.match(status.detail, /evaluation failed/);
  assert.equal(f.h.service.inspectSession().receipts.length, 0);
});

test('daily DST gaps are recorded without execution and folds use only the first instant',async t=>{
  let now=Date.parse('2026-03-07T12:00:00Z');
  const spring=await scheduleFixture(t,()=>now,{kind:'daily',timezone:'America/New_York',time:'02:30',missed:'catchUpOne'});
  assert.equal(spring.preview.occurrences[0]?.intendedAt,null);assert.equal(spring.preview.occurrences[0]?.localDate,'2026-03-08');
  now=Date.parse('2026-03-08T07:00:00Z');await spring.h.service.tickSchedules();assert.deepEqual((await spring.occurrences()).map(item=>item.state),['skippedGap']);
  now=Date.parse('2026-03-09T06:30:00Z');await spring.h.service.tickSchedules();assert.deepEqual((await spring.occurrences()).map(item=>item.state),['skippedGap','accepted']);
  let fallTime=Date.parse('2026-11-01T05:29:30Z');
  const fall=await scheduleFixture(t,()=>fallTime,{kind:'weekly',weekdays:[7],timezone:'America/New_York',time:'01:30',missed:'skip'});
  assert.equal(fall.preview.occurrences[0]?.intendedAt,'2026-11-01T05:30:00.000Z');
  fallTime+=60_000;await fall.h.service.tickSchedules();fallTime=Date.parse('2026-11-01T06:30:00Z');await fall.h.service.tickSchedules();
  const rows=await fall.occurrences();assert.equal(rows.length,1);assert.equal(rows[0]?.state,'accepted');assert.equal(rows[0]?.intendedAt,'2026-11-01T05:30:00.000Z');
});

test('downtime skips missed work by default, allows one catch-up, and an approval keeps overlap protection',async t=>{
  for(const missed of ['skip','catchUpOne'] as const){
    let now=Date.parse('2026-10-17T08:00:00Z');const fixture=await scheduleFixture(t,()=>now,{kind:'daily',timezone:'UTC',time:'09:00',missed},true);
    now=Date.parse('2026-10-19T10:00:00Z');await fixture.h.restart();
    const rows=await fixture.occurrences();assert.equal(rows.length,3);assert.deepEqual(rows.map(row=>row.state),missed==='skip'?['skippedMissed','skippedMissed','skippedMissed']:['skippedMissed','skippedMissed','accepted']);
    if(missed==='catchUpOne'){
      await fixture.h.service.dispatchNext();const snapshot=(await fixture.h.api<Snapshot>(`/v1/conversations/${fixture.agent.homeConversationId}/snapshot`)).body;assert.equal(snapshot.runs[0]?.state,'waitingForApproval');
      now=Date.parse('2026-10-20T09:00:00Z');await fixture.h.service.tickSchedules();assert.equal((await fixture.occurrences()).at(-1)?.state,'skippedOverlap');
      const paused=await fixture.h.api<ScheduleRevision>(fixture.path+'/state',{recoveryEpoch:fixture.h.service.installation.recoveryEpoch,revisionId:fixture.schedule.revisionId,enable:false});assert.equal(paused.body.state,'paused');
      now+=86_400_000;await fixture.h.service.tickSchedules();assert.equal((await fixture.occurrences()).length,4);
      assert.equal((await fixture.h.api<Snapshot>(`/v1/conversations/${fixture.agent.homeConversationId}/snapshot`)).body.runs[0]?.state,'waitingForApproval');
    }
  }
});

test('multi-year recurring downtime records a bounded summary and admits at most one catch-up without replay',async t=>{
  for(const missed of ['skip','catchUpOne'] as const){
    let now=Date.parse('2026-10-17T08:00:00Z');const f=await scheduleFixture(t,()=>now,{kind:'daily',timezone:'UTC',time:'09:00',missed});
    now=Date.parse('2030-10-17T10:00:00Z');await f.h.restart();
    assert.equal((await f.h.api(f.path+'/occurrences')).status,401);await f.h.reenroll();
    const rows=await f.occurrences();assert.equal(rows.length,367);assert.equal(rows[0]?.state,'skippedMissedRange');
    assert.match(rows[0]?.detail??'',/No backlog was dispatched/);
    assert.equal(rows.filter(row=>row.state==='accepted').length,missed==='catchUpOne'?1:0);
    if(missed==='catchUpOne')assert.equal(rows.at(-1)?.intendedAt,'2030-10-17T09:00:00.000Z');
    assert.equal((await f.h.api<Snapshot>(`/v1/conversations/${f.agent.homeConversationId}/snapshot`)).body.runs.length,missed==='catchUpOne'?1:0);
    await f.h.restart();await f.h.service.tickSchedules();assert.deepEqual(await f.occurrences(),rows);
  }
});

test('one-time schedules retain their exact missed occurrence and owner catch-up policy after multi-year downtime',async t=>{
  for(const missed of ['skip','catchUpOne'] as const){
    let now=Date.parse('2026-10-17T08:00:00Z');const f=await scheduleFixture(t,()=>now,{kind:'once',date:'2026-10-18',timezone:'UTC',time:'09:00',missed});
    now=Date.parse('2030-10-17T10:00:00Z');await f.h.restart();await f.h.reenroll();
    const rows=await f.occurrences();assert.equal(rows.length,1);assert.equal(rows[0]?.intendedAt,'2026-10-18T09:00:00.000Z');
    assert.equal(rows[0]?.state,missed==='skip'?'skippedMissed':'accepted');
    const snapshot=(await f.h.api<Snapshot>(`/v1/conversations/${f.agent.homeConversationId}/snapshot`)).body;
    assert.equal(snapshot.runs.length,missed==='catchUpOne'?1:0);
    if(missed==='catchUpOne')assert.equal(snapshot.runs[0]?.requestId,'occurrence-'+rows[0]?.occurrenceId);
    await f.h.restart();await f.h.service.tickSchedules();assert.deepEqual(await f.occurrences(),rows);
  }
});

test('one-time schedules claim once across duplicate schedules and restarts and reject unsupported calendar authority',async t=>{
  let now=Date.parse('2026-10-17T08:59:30Z');const f=await scheduleFixture(t,()=>now,{kind:'once',date:'2026-10-17',timezone:'UTC',time:'09:00',missed:'skip'});
  const duplicate=(await f.h.api<ScheduleRevision>('/v1/schedules',{definition:f.definition})).body,preview=(await f.h.api<SchedulePreview>(`/v1/schedules/${duplicate.scheduleId}/preview`)).body;
  await f.h.api(`/v1/schedules/${duplicate.scheduleId}/state`,{recoveryEpoch:f.h.service.installation.recoveryEpoch,revisionId:duplicate.revisionId,previewDigest:preview.digest,enable:true});
  now+=60_000;await f.h.service.tickSchedules();await f.h.restart();assert.equal((await f.occurrences()).length,1);
  assert.equal((await f.h.api<Snapshot>(`/v1/conversations/${f.agent.homeConversationId}/snapshot`)).body.runs.length,1);
  for(const patch of [{timezone:'+01:00'},{timezone:'Invalid/Nowhere'},{time:'25:00'},{date:'2026-02-31'},{kind:'cron',expression:'* * * * *'},{enable:true},{missed:'all'}])assert.equal((await f.h.api('/v1/schedules',{definition:{...f.definition,...patch}})).status,400);
});

test('schedules require exact owner preview consent and atomically deduplicate occurrences through routine acceptance',async t=>{
  let now=Date.parse('2026-10-17T09:00:00Z'),calls=0;
  const h=await setup(t,{now:()=>now,runtime:{async execute(){calls++;return {kind:'completed',text:'Scheduled evidence',effectiveModel:null};},async dispose(){}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Scheduler',model:'gpt-6-astra',instructions:'Only declared work.'})).body,epoch=h.service.installation.recoveryEpoch;
  const routine=(await h.api<RoutineRevision>('/v1/routines',{definition:{schemaVersion:1,name:'Daily review',agentId:agent.agentId,conversationId:agent.homeConversationId,bindingRevisionId:agent.binding.bindingRevisionId,instructions:'Review the supplied topic.',inputs:[],skills:[],steps:[{stepId:'task',name:'Review',type:'task',instructions:'Review.',inputNames:[],outputStepIds:[]}]}})).body;
  const definition={schemaVersion:1,routineId:routine.routineId,routineRevisionId:routine.revisionId,kind:'daily',timezone:'UTC',time:'09:01',inputs:{},missed:'skip'};
  const saved=await h.api<ScheduleRevision>('/v1/schedules',{definition});assert.equal(saved.status,201);assert.equal(saved.body.state,'paused');
  assert.equal(calls,0);assert.equal((await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body.runs.length,0);
  const path=`/v1/schedules/${saved.body.scheduleId}`,preview=(await h.api<SchedulePreview>(path+'/preview')).body;
  assert.equal(preview.occurrences[0]?.intendedAt,'2026-10-17T09:01:00.000Z');
  const command={recoveryEpoch:epoch,revisionId:saved.body.revisionId,previewDigest:preview.digest,enable:true};
  assert.equal((await h.api(path+'/state',{...command,previewDigest:'wrong'})).status,409);
  assert.equal((await h.api<ScheduleRevision>(path+'/state',command)).body.state,'enabled');
  now+=60_000;await Promise.all([h.service.tickSchedules(),h.service.tickSchedules()]);
  let occurrences=(await h.api<{occurrences:ScheduleOccurrence[]}>(path+'/occurrences')).body.occurrences;
  assert.equal(occurrences.length,1);assert.equal(occurrences[0]?.state,'accepted');
  let snapshot=(await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body;
  assert.equal(snapshot.runs.length,1);assert.equal(snapshot.runs[0]?.routine?.revisionId,routine.revisionId);assert.equal(calls,0);
  await h.service.dispatchNext();assert.equal(calls,1);
  await h.restart();await h.service.tickSchedules();assert.deepEqual((await h.api<{occurrences:ScheduleOccurrence[]}>(path+'/occurrences')).body.occurrences,occurrences);
  const edited=await h.api<ScheduleRevision>(path,{expectedRevisionId:saved.body.revisionId,definition:{...definition,timezone:'America/New_York'}});assert.equal(edited.status,200);assert.equal(edited.body.state,'paused');assert.notEqual(edited.body.revisionId,saved.body.revisionId);
  assert.equal((await h.api(path+'/state',command)).status,409);
  now+=86_400_000;await h.service.tickSchedules();snapshot=(await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body;assert.equal(snapshot.runs.length,1);
});

test('routine cancellation and uncertainty preserve completed evidence without admitting successors',async t=>{
  for(const terminal of ['outcomeUnknown','completed'] as const){
    let finish!:(value:{kind:'outcomeUnknown';message:string}|{kind:'completed';text:string;effectiveModel:null})=>void;
    let began!:()=>void;const started=new Promise<void>(resolve=>{began=resolve;});let calls=0;
    const h=await setup(t,{runtime:{async execute(){calls++;began();return await new Promise(resolve=>{finish=resolve;});},async dispose(){}}});
    const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Use synthetic inputs.'})).body,epoch=h.service.installation.recoveryEpoch;
    const definition={schemaVersion:1,name:'Two tasks',agentId:agent.agentId,conversationId:agent.homeConversationId,bindingRevisionId:agent.binding.bindingRevisionId,instructions:'Do the declared steps only.',inputs:[],skills:[],steps:['one','two'].map(stepId=>({stepId,name:stepId,type:'task',instructions:'Return synthetic text.',inputNames:[],outputStepIds:[]}))};
    const saved=(await h.api<RoutineRevision>('/v1/routines',{definition})).body;
    const accepted=(await h.api<Acceptance>(`/v1/routines/${saved.routineId}/run`,{requestId:'routine-'+terminal,routineRevisionId:saved.revisionId,recoveryEpoch:epoch,inputs:{}})).body;
    const runId=accepted.snapshot.runs[0]!.runId,dispatch=h.service.dispatchNext();await started;
    if(terminal==='completed')await h.api(`/v1/runs/${runId}/cancel`,{recoveryEpoch:epoch});
    finish(terminal==='completed'?{kind:'completed',text:'Preserved output',effectiveModel:null}:{kind:'outcomeUnknown',message:'Synthetic terminal loss'});await dispatch;
    let snapshot=(await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body;
    assert.equal(snapshot.runs[0]?.state,terminal==='completed'?'canceled':'outcomeUnknown');
    assert.deepEqual(snapshot.runs[0]?.routine?.steps.map(step=>step.state),terminal==='completed'?['completed','canceled']:['outcomeUnknown','pending']);
    await h.restart();assert.equal(await h.service.dispatchNext(),false);assert.equal(calls,1);
    if(terminal==='outcomeUnknown'){
      await h.api(`/v1/runs/${runId}/close-unknown`,{recoveryEpoch:epoch,acknowledgeUncertainty:true});
      snapshot=(await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body;
      assert.equal(snapshot.runs[0]?.state,'closedUnknown');assert.deepEqual(snapshot.runs[0]?.routine?.steps.map(step=>step.state),['outcomeUnknown','canceled']);
    }else assert.equal(snapshot.messages.find(message=>message.role==='assistant')?.text,'Preserved output');
    assert.equal(await h.service.dispatchNext(),false);assert.equal(calls,1);
  }
});

test('routine gates expire without authority and cancellation prevents a late decision from resuming work',async t=>{
  let now=Date.now(),calls=0;const h=await setup(t,{now:()=>now,runtime:{async execute(){calls++;return {kind:'completed',text:'Synthetic',effectiveModel:null};},async dispose(){}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Write.'})).body,epoch=h.service.installation.recoveryEpoch;
  const saved=(await h.api<RoutineRevision>('/v1/routines',{definition:{schemaVersion:1,name:'Gate first',agentId:agent.agentId,conversationId:agent.homeConversationId,bindingRevisionId:agent.binding.bindingRevisionId,instructions:'Wait for the owner.',inputs:[],skills:[],steps:[{stepId:'gate',name:'Review',type:'approval',prompt:'Continue?'},{stepId:'task',name:'Write',type:'task',instructions:'Write.',inputNames:[],outputStepIds:[]}]}})).body;
  await h.api(`/v1/routines/${saved.routineId}/run`,{requestId:'expiry',routineRevisionId:saved.revisionId,recoveryEpoch:epoch,inputs:{}});await h.service.dispatchNext();
  const approval=(await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body.approvals[0]!;
  const command={recoveryEpoch:epoch,runId:approval.runId,attemptId:approval.attemptId,bindingRevisionId:approval.bindingRevisionId,actionDigest:approval.actionDigest,resourceScope:approval.resourceScope,decision:'approve'};
  now+=86_400_001;
  const expired=await h.api<{error:{code:string}}>(`/v1/approvals/${approval.approvalId}/resolve`,command);assert.equal(expired.status,409);assert.equal(expired.body.error.code,'approvalExpired');
  await h.api(`/v1/runs/${approval.runId}/cancel`,{recoveryEpoch:epoch});
  const late=await h.api<{approval:Approval;run:Run}>(`/v1/approvals/${approval.approvalId}/resolve`,command);assert.equal(late.body.approval.state,'canceled');assert.equal(late.body.run.state,'canceled');
  await h.restart();assert.equal(await h.service.dispatchNext(),false);assert.equal(calls,0);
});

test('installation advertises routine capabilities without changing its durable identity',async t=>{
  const h=await setup(t),initial=(await h.api<Installation>('/v1/installation')).body;
  assert.ok(initial.capabilities?.includes('routines'));assert.ok(initial.capabilities?.includes('routineGates'));
  await h.restart();assert.deepEqual((await h.api<Installation>('/v1/installation')).body,initial);
});

test('routine edits preserve pinned configuration and disabling its binding blocks queued and future work',async t=>{
  const calls:ChatInput[]=[];
  const h=await setup(t,{runtime:{async execute(input){calls.push(structuredClone(input));return {kind:'completed',text:'Synthetic revision reply',effectiveModel:null};},async dispose(){}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Original instructions.'})).body,epoch=h.service.installation.recoveryEpoch;
  const definition={schemaVersion:1,name:'Draft',agentId:agent.agentId,conversationId:agent.homeConversationId,bindingRevisionId:agent.binding.bindingRevisionId,instructions:'Use the pinned configuration.',inputs:[],skills:[],steps:[{stepId:'one',name:'Draft',type:'task',instructions:'Draft something concise.',inputNames:[],outputStepIds:[]}]};
  const saved=(await h.api<RoutineRevision>('/v1/routines',{definition})).body;
  const command={requestId:'before-disable',routineRevisionId:saved.revisionId,recoveryEpoch:epoch,inputs:{}};
  const path=`/v1/routines/${saved.routineId}`,accepted=(await h.api<Acceptance>(path+'/run',command)).body;
  const changedAgent=(await h.api<Agent>(`/v1/agents/${agent.agentId}`,{expectedBindingRevisionId:agent.binding.bindingRevisionId,name:'New Writer',model:'gpt-6-sol',instructions:'New instructions.'})).body;
  const changed=await h.api<RoutineRevision>(path,{expectedRevisionId:saved.revisionId,definition:{...definition,name:'Revised routine'}});assert.equal(changed.status,200);
  assert.equal(changed.body.agentSnapshot.binding.selectedModel,'gpt-6-astra');
  assert.equal((await h.api(path,{expectedRevisionId:saved.revisionId,definition})).status,409);
  assert.deepEqual((await h.api<RoutineRevision>(path+'/revisions/'+saved.revisionId)).body,saved);
  const disabled=await h.api(`/v1/agents/${agent.agentId}/bindings/${agent.binding.bindingRevisionId}/disable`,{recoveryEpoch:epoch});assert.equal(disabled.status,200);
  await h.service.dispatchNext();assert.equal(calls.length,0);
  const snapshot=(await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body;assert.equal(snapshot.runs[0]?.state,'failed');assert.match(snapshot.runs[0]?.statusDetail??'',/disabled/);
  assert.deepEqual((await h.api<Acceptance>(path+'/run',command)).body.receipt,accepted.receipt);
  assert.equal((await h.api(path+'/run',{...command,requestId:'after-disable',routineRevisionId:changed.body.revisionId})).status,409);
  const rebound=await h.api<RoutineRevision>(path,{expectedRevisionId:changed.body.revisionId,definition:{...definition,bindingRevisionId:changedAgent.binding.bindingRevisionId}});assert.equal(rebound.status,200);
  assert.equal((await h.api(path+'/run',{...command,requestId:'explicit-new-binding',routineRevisionId:rebound.body.revisionId})).status,201);
  await h.service.dispatchNext();assert.equal(calls.length,1);assert.equal(calls[0]?.configuration.selectedModel,'gpt-6-sol');
});

test('routine validation rejects unsupported authority, missing inputs and forward or cyclic output references before saving',async t=>{
  const h=await setup(t),agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Write.'})).body;
  const step={stepId:'one',name:'Task',type:'task',instructions:'Use the declared input.',inputNames:[],outputStepIds:[]};
  const definition={schemaVersion:1,name:'Invalid plan',agentId:agent.agentId,conversationId:agent.homeConversationId,bindingRevisionId:agent.binding.bindingRevisionId,instructions:'Bounded plan.',inputs:[],skills:[],steps:[step]};
  const invalid=[{...definition,steps:[]},{...definition,steps:Array.from({length:17},(_,index)=>({...step,stepId:'step-'+index}))},{...definition,steps:[{...step,outputStepIds:['one']}]},{...definition,steps:[{...step,outputStepIds:['later']},{...step,stepId:'later'}]},{...definition,steps:[{...step,inputNames:['undeclared']}]},{...definition,steps:[{...step,model:'unapproved'}]},{...definition,steps:[{...step,type:'script',command:'echo synthetic'}]},{...definition,inputs:[{name:'x',type:'text',required:true,tools:['network']}]}];
  for(const value of invalid)assert.equal((await h.api('/v1/routines',{definition:value})).status,400);
  assert.deepEqual((await h.api<{routines:RoutineRevision[]}>('/v1/routines')).body.routines,[]);
});

test('routine checkpoints survive restart, park at an exact owner gate and keep later Home work behind the barrier',async t=>{
  const calls:ChatInput[]=[];
  const h=await setup(t,{runtime:{async execute(input){calls.push(structuredClone(input));return {kind:'completed',text:input.text==='Later chat'?'Later reply':calls.length===1?'Draft evidence':'Final evidence',effectiveModel:null};},async dispose(){}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Use only supplied evidence.'})).body,prefix=`/v1/conversations/${agent.homeConversationId}`,epoch=h.service.installation.recoveryEpoch;
  const definition={schemaVersion:1,name:'Review a summary',agentId:agent.agentId,conversationId:agent.homeConversationId,bindingRevisionId:agent.binding.bindingRevisionId,instructions:'Stay with supplied material.',inputs:[],skills:[],steps:[{stepId:'draft',name:'Draft',type:'task',instructions:'Draft the summary.',inputNames:[],outputStepIds:[]},{stepId:'review',name:'Review',type:'approval',prompt:'Continue with the saved draft?'},{stepId:'finish',name:'Finish',type:'task',instructions:'Polish the approved draft.',inputNames:[],outputStepIds:['draft']}]};
  const saved=await h.api<RoutineRevision>('/v1/routines',{definition});assert.equal(saved.status,201);
  const accepted=await h.api<Acceptance>(`/v1/routines/${saved.body.routineId}/run`,{requestId:'gated-routine',routineRevisionId:saved.body.revisionId,recoveryEpoch:epoch,inputs:{}});assert.equal(accepted.status,201);
  await h.service.dispatchNext();assert.equal(calls.length,1);
  await h.restart();await h.service.dispatchNext();assert.equal(calls.length,1);
  const parked=await h.api<Snapshot>(prefix+'/snapshot');assert.equal(parked.status,200);let snapshot=parked.body;
  assert.equal(snapshot.runs[0]?.state,'waitingForApproval');assert.deepEqual(snapshot.runs[0]?.routine?.steps.map(step=>step.state),['completed','waitingForApproval','pending']);
  const approval=snapshot.approvals[0];assert.ok(approval);assert.deepEqual(approval.resourceScope,['routine:continue']);
  await h.api(prefix+'/messages',{requestId:'later-chat',text:'Later chat',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:epoch,skills:[]});
  assert.equal(await h.service.dispatchNext(),false);
  await h.restart();snapshot=(await h.api<Snapshot>(prefix+'/snapshot')).body;assert.deepEqual(snapshot.approvals[0],approval);assert.equal(await h.service.dispatchNext(),false);
  const decision={recoveryEpoch:epoch,runId:approval.runId,attemptId:approval.attemptId,bindingRevisionId:approval.bindingRevisionId,actionDigest:approval.actionDigest,resourceScope:approval.resourceScope,decision:'approve'};
  const route=`/v1/approvals/${approval.approvalId}/resolve`;
  assert.equal((await h.api(route,{...decision,actionDigest:'wrong'})).status,409);
  assert.equal((await h.api(route,{...decision,resourceScope:['all-tools']})).status,409);
  const approved=await h.api<{approval:Approval}>(route,decision);assert.equal(approved.status,200);assert.equal(approved.body.approval.state,'approved');
  assert.deepEqual((await h.api<{approval:Approval}>(route,{...decision,decision:'reject'})).body.approval,approved.body.approval);
  await h.restart();await h.service.dispatchNext();assert.equal(calls.length,2);
  const finalInput=JSON.parse(calls[1]!.text);assert.equal(finalInput.stepId,'finish');assert.equal(finalInput.previousOutputs.length,1);assert.equal(finalInput.previousOutputs[0]?.text,'Draft evidence');
  snapshot=(await h.api<Snapshot>(prefix+'/snapshot')).body;assert.equal(snapshot.runs[0]?.state,'completed');assert.deepEqual(snapshot.runs[0]?.routine?.steps.map(step=>step.state),['completed','approved','completed']);
  const heldChat=snapshot.runs[1]!;await h.api(`/v1/runs/${heldChat.runId}/cancel`,{recoveryEpoch:epoch});
  assert.equal(await h.service.dispatchNext(),false);assert.equal(calls.length,2);
  assert.deepEqual((await h.api<Acceptance>('/v1/requests/gated-routine')).body.receipt,accepted.body.receipt);
});

test('Run now pins its reviewed routine and older Agent revision through the same durable Home and receipt path',async t=>{
  const calls:ChatInput[]=[];
  const h=await setup(t,{runtime:{async execute(input){calls.push(structuredClone(input));return {kind:'completed',text:'Synthetic routine output',effectiveModel:null};},async dispose(){}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Original Agent instructions.'})).body;
  const definition={schemaVersion:1,name:'Draft summary',agentId:agent.agentId,conversationId:agent.homeConversationId,bindingRevisionId:agent.binding.bindingRevisionId,instructions:'Keep this routine concise.',inputs:[{name:'topic',type:'text',required:true}],skills:[],steps:[{stepId:'draft',name:'Draft',type:'task',instructions:'Draft a summary of the supplied topic.',inputNames:['topic'],outputStepIds:[]}]};
  type SavedRoutine={routineId:string;revisionId:string;definition:typeof definition};
  const saved=await h.api<SavedRoutine>('/v1/routines',{definition});assert.equal(saved.status,201);
  await h.api(`/v1/agents/${agent.agentId}`,{expectedBindingRevisionId:agent.binding.bindingRevisionId,name:'Writer changed',model:'gpt-6-sol',instructions:'Later instructions.'});
  const command={requestId:'routine-first',routineRevisionId:saved.body.revisionId,recoveryEpoch:h.service.installation.recoveryEpoch,inputs:{topic:'  Cedar\r\n'}};
  const path=`/v1/routines/${saved.body.routineId}/run`;
  assert.equal((await h.api(path,{...command,inputs:{topic:42}})).status,400);
  const accepted=await h.api<Acceptance>(path,command);assert.equal(accepted.status,201);
  assert.equal(accepted.body.receipt.conversationId,agent.homeConversationId);assert.equal(accepted.body.receipt.bindingRevisionId,agent.binding.bindingRevisionId);
  assert.equal(calls.length,0);await h.service.dispatchNext();assert.equal(calls.length,1);
  assert.equal(calls[0]?.configuration.agentName,'Writer');assert.equal(calls[0]?.configuration.selectedModel,'gpt-6-astra');
  assert.match(calls[0]?.configuration.instructions??'',/Original Agent instructions/);assert.match(calls[0]?.configuration.instructions??'',/Keep this routine concise/);
  const task=JSON.parse(calls[0]!.text);assert.deepEqual(task.inputs,{topic:'  Cedar\r\n'});assert.deepEqual(task.previousOutputs,[]);assert.equal(task.instructions,definition.steps[0]?.instructions);
  const repeated=await h.api<Acceptance>(path,command);assert.deepEqual(repeated.body.receipt,accepted.body.receipt);
  await h.restart();
  const snapshot=(await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body;
  assert.equal(snapshot.messages.length,2);assert.equal(snapshot.messages[1]?.text,'Synthetic routine output');assert.equal(snapshot.runs[0]?.state,'completed');
  assert.equal(await h.service.dispatchNext(),false);assert.equal(calls.length,1);
  assert.deepEqual((await h.api<Acceptance>('/v1/requests/routine-first')).body.receipt,accepted.body.receipt);
});

function syntheticBundle() {
  return {schemaVersion:1,name:'Synthetic bundle',version:'1',source:'owner-local:synthetic',license:'CC0-1.0',entrypoint:'SKILL.md',supportedRuntimes:['codex'],requestedTools:[],scripts:[],files:[{path:'SKILL.md',kind:'file',executable:false,contentBase64:Buffer.from('Synthetic reviewed instructions').toString('base64')}]};
}

test('bundle import rejects unsafe paths, links, undeclared executables and malformed or oversized content without staging work',async t=>{
  const h=await setup(t),base=syntheticBundle(),file=base.files[0]!;
  const invalid:unknown[]=[
    ...['../outside','/absolute','nested/../escape','C:\\file','a\u0000b','a//b','a/./b','a/b.'].map(path=>({...base,entrypoint:path,files:[{...file,path}]})),
    {...base,files:[file,{...file,path:'skill.md'}]},
    {...base,files:[file,{...file,path:'assets'},{...file,path:'assets/nested.txt'}]},
    {...base,files:[file,{...file,path:'link',kind:'symlink',target:'outside'}]},
    {...base,files:[file,{...file,path:'run.sh',executable:true}]},
    {...base,scripts:[{path:'missing.sh',interpreter:'bash',resourceScope:[]}]},
    {...base,files:[{...file,contentBase64:'not base64'}]},
    {...base,files:[{...file,contentBase64:Buffer.from([255]).toString('base64')}]},
    {...base,files:[{...file,sha256:'forged'}]},
    {...base,files:[{...file,bytes:999}]},
    {...base,digest:'forged'},
    {...base,files:Array.from({length:65},(_,index)=>({...file,path:index?'asset-'+index:'SKILL.md'}))},
    {...base,files:[file,{...file,path:'large',contentBase64:Buffer.alloc(262_145).toString('base64')}]},
    {...base,files:[file,...[1,2,3].map(index=>({...file,path:'large-'+index,contentBase64:Buffer.alloc(200_000).toString('base64')}))]},
    {...base,unexpectedAuthority:'full-access'},
  ];
  for(const bundle of invalid){const result=await h.api<{error:{code:string}}>('/v1/skill-bundles',{bundle});assert.equal(result.status,400);assert.equal(result.body.error.code,'invalidSkillBundle');}
  assert.deepEqual((await h.api<{skills:InstructionSkill[]}>('/v1/skills')).body.skills,[]);
  assert.equal(await h.service.dispatchNext(),false);
});

test('revoking a historical pinned skill blocks queued and new routine invocation while retaining the current revision',async t=>{
  let calls=0;const h=await setup(t,{runtime:{async execute(){calls++;return {kind:'completed',text:'Must not dispatch',effectiveModel:null};},async dispose(){}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Write.'})).body,epoch=h.service.installation.recoveryEpoch;
  const a=(await h.api<InstructionSkill>('/v1/skills',{name:'Revision A',instructions:'Original reviewed instructions.'})).body;assert.ok(a.bundle);
  await h.api(`/v1/skills/${a.skillId}/review`,{revisionId:a.revisionId,digest:a.bundle.digest,enable:true});
  const routine=(await h.api<RoutineRevision>('/v1/routines',{definition:{schemaVersion:1,name:'Pinned A',agentId:agent.agentId,conversationId:agent.homeConversationId,bindingRevisionId:agent.binding.bindingRevisionId,instructions:'Use this pinned revision.',inputs:[],skills:[{skillId:a.skillId,revisionId:a.revisionId,digest:a.bundle.digest}],steps:[{stepId:'task',name:'Task',type:'task',instructions:'Write.',inputNames:[],outputStepIds:[]}]}})).body;
  const command={requestId:'queued-with-a',routineRevisionId:routine.revisionId,recoveryEpoch:epoch,inputs:{}},path=`/v1/routines/${routine.routineId}/run`;
  assert.equal((await h.api(path,command)).status,201);
  const b=(await h.api<InstructionSkill>(`/v1/skills/${a.skillId}`,{expectedRevisionId:a.revisionId,name:'Revision B',instructions:'New instructions.'})).body;assert.ok(b.bundle);
  await h.api(`/v1/skills/${b.skillId}/review`,{revisionId:b.revisionId,digest:b.bundle.digest,enable:true});
  const pinned=(await h.api<InstructionSkill>(`/v1/skills/${a.skillId}/revisions/${a.revisionId}`)).body;assert.equal(pinned.review?.state,'enabled');
  assert.equal((await h.api(`/v1/skills/${a.skillId}/review`,{revisionId:a.revisionId,digest:a.bundle.digest,enable:false})).status,200);
  assert.equal((await h.api<{skills:InstructionSkill[]}>('/v1/skills')).body.skills[0]?.revisionId,b.revisionId);
  assert.equal((await h.api(path,{...command,requestId:'new-with-a'})).status,409);
  await h.service.dispatchNext();assert.equal(calls,0);assert.equal(h.service.inspectSession().runs[0]?.state,'failed');
  await h.restart();assert.equal((await h.api<InstructionSkill>(`/v1/skills/${a.skillId}/revisions/${a.revisionId}`)).body.review?.state,'disabled');
});

test('explicitly disabling a reviewed skill before dispatch blocks accepted work without a runtime call',async t=>{
  let calls=0;
  const h=await setup(t,{runtime:{async execute(){calls++;return {kind:'completed',text:'Must not run',effectiveModel:null};},async dispose(){}}});
  const imported=(await h.api<InstructionSkill>('/v1/skill-bundles',{bundle:syntheticBundle()})).body;assert.ok(imported.bundle);
  const review={revisionId:imported.revisionId,digest:imported.bundle.digest,enable:true};
  await h.api(`/v1/skills/${imported.skillId}/review`,review);
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply.'})).body,prefix=`/v1/conversations/${agent.homeConversationId}`;
  const acceptance=await h.api<Acceptance>(prefix+'/messages',{requestId:'disabled-skill',text:'Synthetic question',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,skills:[{skillId:imported.skillId,revisionId:imported.revisionId}]});
  assert.equal(acceptance.status,201);
  await h.api(`/v1/skills/${imported.skillId}/review`,{...review,enable:false});
  await h.service.dispatchNext();assert.equal(calls,0);
  const snapshot=(await h.api<Snapshot>(prefix+'/snapshot')).body;
  assert.equal(snapshot.runs[0]?.state,'failed');assert.match(snapshot.runs[0]?.statusDetail??'',/disabled/);assert.equal(snapshot.messages.length,1);
});

test('complete skill bundle import is inert, requires exact review and preserves accepted revision after an edit',async t=>{
  const calls:ChatInput[]=[];
  const h=await setup(t,{runtime:{async execute(input){calls.push(structuredClone(input));return {kind:'completed',text:'Synthetic reviewed reply',effectiveModel:null};},async dispose(){}}});
  const bundle={schemaVersion:1,name:'Packing helper',version:'1.0',source:'owner-local:synthetic-packing',license:'CC0-1.0',entrypoint:'SKILL.md',supportedRuntimes:['codex'],requestedTools:[],scripts:[],files:[{path:'SKILL.md',kind:'file',executable:false,contentBase64:Buffer.from('Use the reviewed packing helper.').toString('base64')},{path:'assets/checklist.txt',kind:'file',executable:false,contentBase64:Buffer.from('Notebook\nPencil\n').toString('base64')}]};
  type BundleSkill=InstructionSkill&{bundle:typeof bundle&{digest:string};review:{state:string;digest:string}};
  const imported=await h.api<BundleSkill>('/v1/skill-bundles',{bundle});
  assert.equal(imported.status,201);assert.equal(imported.body.review.state,'reviewRequired');
  assert.equal(imported.body.bundle.files.length,2);assert.equal(imported.body.bundle.files[1]?.contentBase64,bundle.files[1]?.contentBase64);
  assert.match(imported.body.bundle.digest,/^[0-9a-f]{64}$/);assert.equal(calls.length,0);
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply.'})).body;
  const prefix=`/v1/conversations/${agent.homeConversationId}`;
  const command={requestId:'reviewed-bundle',text:'Synthetic question',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,skills:[{skillId:imported.body.skillId,revisionId:imported.body.revisionId}]};
  assert.equal((await h.api(prefix+'/messages',command)).status,409);
  assert.deepEqual((await h.api<Snapshot>(prefix+'/snapshot')).body.messages,[]);
  const review=`/v1/skills/${imported.body.skillId}/review`;
  assert.equal((await h.api(review,{revisionId:imported.body.revisionId,digest:'wrong',enable:true})).status,409);
  const enabled=await h.api<BundleSkill>(review,{revisionId:imported.body.revisionId,digest:imported.body.bundle.digest,enable:true});
  assert.equal(enabled.status,200);assert.equal(enabled.body.review.state,'enabled');
  const accepted=await h.api<Acceptance>(prefix+'/messages',command);assert.equal(accepted.status,201);
  bundle.files[1]!.contentBase64=Buffer.from('Changed asset').toString('base64');bundle.version='2.0';
  const changed=await h.api<BundleSkill>('/v1/skill-bundles',{bundle,skillId:imported.body.skillId,expectedRevisionId:imported.body.revisionId});
  assert.equal(changed.status,201);assert.notEqual(changed.body.revisionId,imported.body.revisionId);assert.equal(changed.body.review.state,'reviewRequired');assert.notEqual(changed.body.bundle.digest,imported.body.bundle.digest);
  assert.equal((await h.api(review,{revisionId:imported.body.revisionId,digest:imported.body.bundle.digest,enable:true})).status,409);
  await h.service.dispatchNext();assert.equal(calls.length,1);
  assert.deepEqual(calls[0]?.configuration.skills,[enabled.body]);
  const replay=await h.api<Acceptance>(prefix+'/messages',command);assert.deepEqual(replay.body.receipt,accepted.body.receipt);
  await h.restart();
  const exported=await h.api<BundleSkill>(`/v1/skills/${imported.body.skillId}/revisions/${imported.body.revisionId}`);
  assert.equal(exported.status,200);assert.deepEqual(exported.body.bundle,imported.body.bundle);
});

test('owner cancels queued work without executing it and the same Home can continue',async t=>{
  const calls:string[]=[];
  const h=await setup(t,{runtime:{async execute(input){calls.push(input.requestId);return {kind:'completed',text:'Synthetic next reply',effectiveModel:null};},async dispose(){}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply briefly.'})).body;
  const path=`/v1/conversations/${agent.homeConversationId}`,epoch=h.service.installation.recoveryEpoch;
  const send=(requestId:string)=>h.api<Acceptance>(path+'/messages',{requestId,text:requestId,recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:epoch,skills:[]});
  const first=(await send('cancel-this')).body;
  await h.restart();assert.equal(await h.service.dispatchNext(),false);
  const canceled=await h.api<{run:Run}>(`/v1/runs/${first.receipt.runId}/cancel`,{recoveryEpoch:epoch});
  assert.equal(canceled.status,200);assert.equal(canceled.body.run.state,'canceled');
  assert.equal(await h.service.dispatchNext(),false);assert.deepEqual(calls,[]);
  const duplicate=await h.api<{run:Run}>(`/v1/runs/${first.receipt.runId}/cancel`,{recoveryEpoch:epoch});
  assert.deepEqual(duplicate.body.run,canceled.body.run);
  await send('continue-here');assert.equal(await h.service.dispatchNext(),true);
  const snapshot=(await h.api<Snapshot>(path+'/snapshot')).body;
  assert.deepEqual(snapshot.runs.map(run=>run.state),['canceled','completed']);
  assert.deepEqual(calls,['continue-here']);
  await h.restart();assert.deepEqual((await h.api<Snapshot>(path+'/snapshot')).body.runs.map(run=>run.state),['canceled','completed']);
});

for(const target of ['run','request'])for(const outcome of ['canceled','completed','outcomeUnknown'] as const)test(`active ${target} cancellation preserves runtime evidence and the Conversation barrier: ${outcome}`,async t=>{
  const started=Promise.withResolvers<AbortSignal>(),terminal=Promise.withResolvers<Awaited<ReturnType<ChatRuntime['execute']>>>();
  const calls:string[]=[];
  const h=await setup(t,{runtime:{async execute(input,signal){calls.push(input.requestId);if(input.requestId==='active'){started.resolve(signal);return terminal.promise;}return {kind:'completed',text:'Synthetic later reply',effectiveModel:null};},async dispose(){terminal.resolve({kind:'canceled',message:'Synthetic confirmed interruption'});}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply briefly.'})).body,epoch=h.service.installation.recoveryEpoch;
  const path=`/v1/conversations/${agent.homeConversationId}`;
  const send=(requestId:string)=>h.api<Acceptance>(path+'/messages',{requestId,text:requestId,recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:epoch,skills:[]});
  const first=(await send('active')).body;const dispatch=h.service.dispatchNext();const signal=await started.promise;
  await send('later');
  assert.equal((await h.api(`/v1/runs/${first.receipt.runId}/close-unknown`,{recoveryEpoch:epoch,acknowledgeUncertainty:true})).status,409);
  assert.equal((await h.api(`/v1/runs/${first.receipt.runId}/cancel`,{recoveryEpoch:'wrong'})).status,409);
  const cancelPath=target==='run'?`/v1/runs/${first.receipt.runId}/cancel`:'/v1/requests/active/cancel';
  const scope={recoveryEpoch:epoch,conversationId:agent.homeConversationId,bindingRevisionId:agent.binding.bindingRevisionId};
  const response=await h.api<{run:Run}>(cancelPath,scope);
  assert.equal(response.status,200);assert.equal(response.body.run.state,'cancelRequested');assert.equal(signal.aborted,true);
  assert.deepEqual((await h.api<Snapshot>(path+'/snapshot')).body.runs.map(run=>run.state),['cancelRequested','queued']);assert.deepEqual(calls,['active']);
  terminal.resolve(outcome==='completed'?{kind:'completed',text:'Synthetic completion won the race',effectiveModel:null}:{kind:outcome,message:'Synthetic terminal evidence'});await dispatch;
  const final=(await h.api<Snapshot>(path+'/snapshot')).body;
  assert.equal(final.runs[0]?.state,outcome);
  assert.equal((await h.api<{run:Run}>(cancelPath,scope)).body.run.state,outcome);
  assert.equal(final.messages.filter(message=>message.role==='assistant').length,outcome==='completed'?1:0);
  assert.equal(await h.service.dispatchNext(),outcome!=='outcomeUnknown');
  assert.deepEqual(calls,outcome==='outcomeUnknown'?['active']:['active','later']);
});

test('owner explicitly closes uncertainty without retrying it, preserving history and releasing later work',async t=>{
  const calls:string[]=[];
  const h=await setup(t,{runtime:{async execute(input){calls.push(input.requestId);return input.requestId==='uncertain'?{kind:'outcomeUnknown',message:'Synthetic lost terminal evidence'}:{kind:'completed',text:'Synthetic next reply',effectiveModel:null};},async dispose(){}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply briefly.'})).body,epoch=h.service.installation.recoveryEpoch,path=`/v1/conversations/${agent.homeConversationId}`;
  const send=(requestId:string)=>h.api<Acceptance>(path+'/messages',{requestId,text:requestId,recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:epoch,skills:[]});
  const first=(await send('uncertain')).body;await h.service.dispatchNext();await send('later');
  assert.equal(await h.service.dispatchNext(),false);assert.deepEqual(calls,['uncertain']);
  const close=`/v1/runs/${first.receipt.runId}/close-unknown`;
  assert.equal((await h.api(close,{recoveryEpoch:epoch})).status,400);
  assert.equal((await h.api(close,{recoveryEpoch:'wrong',acknowledgeUncertainty:true})).status,409);
  const result=await h.api<{run:Run}>(close,{recoveryEpoch:epoch,acknowledgeUncertainty:true});
  assert.equal(result.status,200);assert.equal(result.body.run.state,'closedUnknown');
  assert.match(result.body.run.statusDetail??'',/not retried/);
  assert.deepEqual((await h.api<{run:Run}>(close,{recoveryEpoch:epoch,acknowledgeUncertainty:true})).body,result.body);
  await h.service.dispatchNext();assert.deepEqual(calls,['uncertain','later']);
  await h.restart();const snapshot=(await h.api<Snapshot>(path+'/snapshot')).body;
  assert.deepEqual(snapshot.runs.map(run=>run.state),['closedUnknown','completed']);assert.equal(snapshot.messages.length,3);
  assert.deepEqual((await h.api<Acceptance>('/v1/requests/uncertain')).body.receipt,first.receipt);
});

test('runtime refusal before dispatch is a visible failure, not an owner cancellation',async t=>{
  const h=await setup(t,{runtime:{async execute(){return {kind:'notStarted',message:'Exact selected model is unavailable. Nothing was dispatched.'};},async dispose(){}}});
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply briefly.'})).body;
  const path=`/v1/conversations/${agent.homeConversationId}`;
  await h.api(path+'/messages',{requestId:'unavailable',text:'Synthetic question',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,skills:[]});
  await h.service.dispatchNext();
  const snapshot=(await h.api<Snapshot>(path+'/snapshot')).body;
  assert.equal(snapshot.runs[0]?.state,'failed');assert.match(snapshot.runs[0]?.statusDetail??'',/Nothing was dispatched/);assert.equal(snapshot.messages.length,1);
});

for(const when of ['before','during'] as const)test(`canceled listener startup exposes no address or surviving listener: ${when}`,async t=>{
  const directory=await mkdtemp(join(tmpdir(),'fort-listener-cancel-'));
  const service=createApplication({provider: CODEX_RUNTIME, dataDirectory:directory});
  t.after(async()=>{await service.close();await rm(directory,{recursive:true,force:true});});
  const reservation=createServer();t.after(()=>{reservation.close();});
  await new Promise<void>(resolve=>reservation.listen(0,'127.0.0.1',resolve));
  const address=reservation.address();assert.ok(address&&typeof address==='object');
  const port=address.port;
  await new Promise<void>((resolve,reject)=>reservation.close(error=>error?reject(error):resolve()));
  const cancellation=new AbortController();if(when==='before')cancellation.abort();
  const starting=service.start({port,signal:cancellation.signal});
  const result=starting.then(()=> 'unexpected success',()=> 'rejected');
  cancellation.abort();
  assert.equal(await result,'rejected');
  assert.equal(service.baseURL,'');
  await new Promise(resolve=>setImmediate(resolve));
  await assert.rejects(()=>fetch(`http://127.0.0.1:${port}/v1/installation`));
});

test('owner creates two exact Agent Homes, an instruction skill and a shared conversation that survive restart', async t => {
  const h = await setup(t);
  const writer = await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Reply briefly.' });
  assert.equal(writer.status, 201);
  const reviewer = await h.api<Agent>('/v1/agents', { name: 'Reviewer', model: 'gpt-6-astra', instructions: 'Check the claim.' });
  assert.equal(reviewer.status, 201);
  assert.notEqual(writer.body.homeConversationId, reviewer.body.homeConversationId);
  const skill = await h.api<InstructionSkill>('/v1/skills', { name: 'Check marker', instructions: 'End with [CHECKED].' });
  assert.equal(skill.status, 201);
  const room = await h.api<Conversation>('/v1/conversations', { name: 'Sample planning', participantAgentIds: [writer.body.agentId, reviewer.body.agentId] });
  assert.equal(room.status, 201);
  const identity = h.service.installation;
  await h.restart();
  const { eventCursor, ...reopened } = (await h.api<Installation>('/v1/installation')).body;
  assert.deepEqual(reopened, identity); assert.ok(eventCursor !== undefined && eventCursor > 0);
  assert.deepEqual((await h.api<{ agents: Agent[] }>('/v1/agents')).body.agents, [writer.body, reviewer.body]);
  assert.deepEqual((await h.api<{ skills: InstructionSkill[] }>('/v1/skills')).body.skills, [skill.body]);
  const rooms = (await h.api<{ conversations: Conversation[] }>('/v1/conversations')).body.conversations;
  assert.equal(rooms.length, 3);
  assert.deepEqual(rooms.find(r => r.kind === 'shared'), room.body);
  assert.equal((await fetch(h.service.baseURL + '/v1/agents')).status, 401);
});

test('bounded enrollment expires within ten minutes and final close invalidates its device without a new enrollment', async t => {
  const directory=await mkdtemp(join(tmpdir(),'fort-bounded-enrollment-'));
  let now=Date.now(); const deadline=now+600_000;
  let service=createApplication({ provider: CODEX_RUNTIME, dataDirectory:directory,credentialDeadline:deadline,now:()=>now });
  t.after(async()=>{ await service.close(); await rm(directory,{ recursive:true,force:true }); });
  await service.start();
  const enrollment=service.issueEnrollment();
  const response=await fetch(service.baseURL+'/v1/devices/enroll',{ method:'POST',headers:{ 'Content-Type':'application/json' },body:JSON.stringify({ ...enrollment,deviceName:'Synthetic bounded device' }) });
  const device=await response.json() as { deviceToken:string; expiresAt:string };
  assert.equal(Date.parse(device.expiresAt),deadline);
  const headers={ Authorization:`Bearer ${device.deviceToken}`,'X-Fort-Installation-Id':service.installation.installationId };
  assert.throws(()=>service.issueEnrollment(),/already issued/);
  await service.close({ preserveSessionCredential:true });
  service=createApplication({ provider: CODEX_RUNTIME, dataDirectory:directory,credentialDeadline:deadline,now:()=>now });
  await service.start();
  assert.equal((await fetch(service.baseURL+'/v1/installation',{ headers })).status,200);
  await service.close();
  service=createApplication({ provider: CODEX_RUNTIME, dataDirectory:directory,credentialDeadline:deadline,now:()=>now });
  await service.start();
  assert.equal((await fetch(service.baseURL+'/v1/installation',{ headers })).status,401);
  assert.throws(()=>service.issueEnrollment(),/closed/);
});

test('closed sample admission rejects before durable acceptance or runtime work', async t => {
  let executions=0;
  const h=await setup(t,{ autoRun:true,admitSubmission:()=>false,runtime:{ async dispose(){},async execute(){ executions++;return { kind:'completed',text:'unexpected',effectiveModel:null }; } } });
  const agent=(await h.api<Agent>('/v1/agents',{ name:'Writer',model:'gpt-6-astra',instructions:'Reply.' })).body;
  const prefix=`/v1/conversations/${agent.homeConversationId}`;
  const result=await h.api(`${prefix}/messages`,{ requestId:'blocked',text:'extra sample',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,skills:[] });
  assert.equal(result.status,403);
  assert.deepEqual((await h.api<Snapshot>(`${prefix}/snapshot`)).body.runs,[]);
  assert.equal(executions,0);
});

test('bounded device authorization expires even if the service remains open',async t=>{
  let now=Date.now();
  const h=await setup(t,{credentialDeadline:now+600_000,now:()=>now});
  assert.equal((await h.api('/v1/installation')).status,200);
  const agent=(await h.api<Agent>('/v1/agents',{name:'Writer',model:'gpt-6-astra',instructions:'Reply.'})).body;
  const accepted=(await h.api<Acceptance>(`/v1/conversations/${agent.homeConversationId}/messages`,{requestId:'expiry-check',text:'Synthetic sample',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,skills:[]})).body;
  now+=600_000;
  for(const action of ['cancel','close-unknown'])assert.equal((await h.api(`/v1/runs/${accepted.receipt.runId}/${action}`,{recoveryEpoch:h.service.installation.recoveryEpoch,acknowledgeUncertainty:true})).status,401);
  assert.equal((await h.api('/v1/installation')).status,401);
  assert.throws(()=>h.service.issueEnrollment(),/closed/);
});

test('explicit automatic dispatch runs new accepted work and read acknowledgement persists monotonically', async t => {
  const h = await setup(t, { autoRun: true, runtime: { async dispose() {}, async execute() { return { kind: 'completed', text: 'Saved reply', effectiveModel: null }; } } });
  const agent = (await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Reply.' })).body;
  const prefix = `/v1/conversations/${agent.homeConversationId}`;
  await h.api(`${prefix}/messages`, { requestId: 'automatic', text: 'Sample', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] });
  let snapshot = (await h.api<Snapshot>(`${prefix}/snapshot`)).body;
  for (let count = 0; count < 20 && snapshot.runs[0]?.state !== 'completed'; count++) { await new Promise(resolve => setTimeout(resolve, 5)); snapshot = (await h.api<Snapshot>(`${prefix}/snapshot`)).body; }
  assert.equal(snapshot.runs[0]?.state, 'completed');
  const message = snapshot.messages[1]!;
  assert.equal((await h.api(`${prefix}/read`, { recoveryEpoch: h.service.installation.recoveryEpoch, cursor: snapshot.cursor, messageId: message.messageId })).status, 200);
  await h.api(`${prefix}/read`, { recoveryEpoch: h.service.installation.recoveryEpoch, cursor: snapshot.cursor, messageId: snapshot.messages[0]!.messageId });
  await h.restart();
  assert.equal((await h.api<Snapshot>(`${prefix}/snapshot`)).body.lastReadMessageSequence, message.sequence);
});


test('shared submission pins its recipient and selected skill, saves one attributed reply and replays the original receipt', async t => {
  const inputs: ChatInput[] = [];
  const runtime: ChatRuntime = { async dispose() {}, async execute(input) { inputs.push(structuredClone(input)); return { kind: 'completed', text: 'red notebook [CHECKED]', effectiveModel: null }; } };
  const h = await setup(t, { runtime });
  const writer = (await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Write briefly.' })).body;
  const reviewer = (await h.api<Agent>('/v1/agents', { name: 'Reviewer', model: 'gpt-6-astra', instructions: 'Check the claim.' })).body;
  let skill = (await h.api<InstructionSkill>('/v1/skills', { name: 'Check marker', instructions: 'End with [CHECKED].' })).body;
  assert.ok(skill.bundle);skill=(await h.api<InstructionSkill>(`/v1/skills/${skill.skillId}/review`,{revisionId:skill.revisionId,digest:skill.bundle.digest,enable:true})).body;
  const room = (await h.api<Conversation>('/v1/conversations', { name: 'Sample planning', participantAgentIds: [writer.agentId, reviewer.agentId] })).body;
  const path = `/v1/conversations/${room.conversationId}/messages`;
  const command = { requestId: 'sample-shared', recoveryEpoch: h.service.installation.recoveryEpoch, bindingRevisionId: reviewer.binding.bindingRevisionId, recipientAgentId: reviewer.agentId, skills: [{ skillId: skill.skillId, revisionId: skill.revisionId }], text: '  red notebook\n' };
  assert.equal((await h.api(path, { ...command, recipientAgentId: undefined })).status, 400);
  const accepted = await h.api<Acceptance>(path, command);
  assert.equal(accepted.status, 201);
  assert.equal(inputs.length, 0);
  assert.equal(await h.service.dispatchNext(), true);
  assert.equal(inputs.length, 1);
  assert.equal(inputs[0]?.configuration.agentId, reviewer.agentId);
  assert.equal(inputs[0]?.configuration.instructions, 'Check the claim.');
  assert.deepEqual(inputs[0]?.configuration.skills, [skill]);
  assert.equal(inputs[0]?.text, command.text);
  assert.deepEqual(inputs[0]?.history, []);
  const snapshot = (await h.api<Snapshot>(`/v1/conversations/${room.conversationId}/snapshot`)).body;
  assert.equal(snapshot.runs[0]?.state, 'completed');
  assert.equal(snapshot.messages[0]?.recipientAgentId, reviewer.agentId);
  assert.equal(snapshot.messages[1]?.agentId, reviewer.agentId);
  assert.equal(snapshot.messages[1]?.agentName, 'Reviewer');
  assert.equal(snapshot.messages[1]?.text, 'red notebook [CHECKED]');
  const repeated = (await h.api<Acceptance>(path, command)).body;
  const recovered = (await h.api<Acceptance>('/v1/requests/sample-shared')).body;
  assert.deepEqual(repeated.receipt, accepted.body.receipt);
  assert.deepEqual(recovered.receipt, accepted.body.receipt);
  assert.deepEqual(repeated.snapshot, snapshot);
  assert.deepEqual(recovered.snapshot, snapshot);
  assert.equal((await h.api(path, { ...command, text: 'changed' })).status, 409);
  assert.equal(await h.service.dispatchNext(), false);
});

test('queued turns receive completed prior context in their own conversation, including after restart', async t => {
  const inputs: ChatInput[] = [];
  const runtime: ChatRuntime = { async dispose() {}, async execute(input) { inputs.push(structuredClone(input)); return { kind: 'completed', text: 'cedar', effectiveModel: null }; } };
  const h = await setup(t, { runtime });
  const writer = (await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Remember the sample word.' })).body;
  const reviewer = (await h.api<Agent>('/v1/agents', { name: 'Reviewer', model: 'gpt-6-astra', instructions: 'Check the sample.' })).body;
  const send = (agent: Agent, requestId: string, text: string) => h.api<Acceptance>(`/v1/conversations/${agent.homeConversationId}/messages`, { requestId, text, recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] });
  assert.equal((await send(writer, 'remember', 'Remember cedar.')).status, 201);
  assert.equal((await send(writer, 'recall-queued', 'What was the word?')).status, 201);
  assert.equal((await send(reviewer, 'unrelated', 'Explain checklists.')).status, 201);
  await h.service.dispatchNext(); await h.service.dispatchNext(); await h.service.dispatchNext();
  assert.deepEqual(inputs[0]?.history, []);
  assert.deepEqual(inputs[1]?.history.map(m => [m.role, m.text]), [['user', 'Remember cedar.'], ['assistant', 'cedar']]);
  assert.deepEqual(inputs[2]?.history, []);
  await h.restart();
  assert.equal((await send(writer, 'recall-restarted', 'What was the sample word?')).status, 201);
  await h.service.dispatchNext();
  assert.deepEqual(inputs[3]?.history.map(m => [m.role, m.text]), [['user', 'Remember cedar.'], ['assistant', 'cedar'], ['user', 'What was the word?'], ['assistant', 'cedar']]);
  assert.equal(inputs[3]?.history.every(m => m.conversationId === writer.homeConversationId), true);
});

test('restart holds old queued work and unknown outcomes without blocking a different Home', async t => {
  const calls: ChatInput[] = [];
  let started!: () => void;
  const entered = new Promise<void>(resolve => { started = resolve; });
  const runtime: ChatRuntime = { async dispose() {}, async execute(input, signal) {
    calls.push(input);
    if (input.requestId === 'uncertain') { started(); await new Promise<void>(resolve => signal.addEventListener('abort', () => resolve(), { once: true })); return { kind: 'outcomeUnknown', message: 'Stopped before confirmation.' }; }
    return { kind: 'completed', text: 'Synthetic reply', effectiveModel: null };
  } };
  const h = await setup(t, { runtime });
  const a = (await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Write.' })).body;
  const b = (await h.api<Agent>('/v1/agents', { name: 'Reviewer', model: 'gpt-6-astra', instructions: 'Review.' })).body;
  const send = (agent: Agent, requestId: string) => h.api(`/v1/conversations/${agent.homeConversationId}/messages`, { requestId, text: requestId, recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [] });
  await send(a, 'uncertain'); await send(a, 'old-queued');
  const dispatch = h.service.dispatchNext(); await entered;
  await h.restart(); await dispatch;
  assert.equal(await h.service.dispatchNext(), false);
  await send(a, 'new-behind-barrier'); await send(b, 'other-home');
  assert.equal(await h.service.dispatchNext(), true);
  assert.deepEqual(calls.map(c => c.requestId), ['uncertain', 'other-home']);
  assert.equal(await h.service.dispatchNext(), false);
  const snapshot = (await h.api<Snapshot>(`/v1/conversations/${a.homeConversationId}/snapshot`)).body;
  assert.deepEqual(snapshot.runs.map(r => r.state), ['outcomeUnknown', 'queued', 'queued']);
});

test('profile edits keep stable identity, fence stale submissions and preserve already accepted configuration', async t => {
  const calls: ChatInput[] = [];
  const h = await setup(t, { runtime: { async dispose() {}, async execute(input) { calls.push(input); return { kind: 'completed', text: 'Done.', effectiveModel: null }; } } });
  const agent = (await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Original instructions.' })).body;
  let skill = (await h.api<InstructionSkill>('/v1/skills', { name: 'Marker', instructions: 'Use [ONE].' })).body;
  assert.ok(skill.bundle);skill=(await h.api<InstructionSkill>(`/v1/skills/${skill.skillId}/review`,{revisionId:skill.revisionId,digest:skill.bundle.digest,enable:true})).body;
  const command = { requestId: 'before-edit', text: 'Sample.', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [{ skillId: skill.skillId, revisionId: skill.revisionId }] };
  const path = `/v1/conversations/${agent.homeConversationId}/messages`;
  assert.equal((await h.api(path, command)).status, 201);
  const changed = await h.api<Agent>(`/v1/agents/${agent.agentId}`, { expectedBindingRevisionId: agent.binding.bindingRevisionId, name: 'Author', model: 'another-exact-model', instructions: 'Changed instructions.' });
  assert.equal(changed.status, 200);
  assert.equal(changed.body.homeConversationId, agent.homeConversationId);
  assert.notEqual(changed.body.binding.bindingRevisionId, agent.binding.bindingRevisionId);
  const changedSkill = await h.api<InstructionSkill>(`/v1/skills/${skill.skillId}`, { expectedRevisionId: skill.revisionId, name: 'Marker', instructions: 'Use [TWO].' });
  assert.equal(changedSkill.status, 200);
  assert.equal((await h.api(path, { ...command, requestId: 'stale-agent' })).status, 409);
  assert.equal((await h.api(path, { ...command, requestId: 'stale-skill', bindingRevisionId: changed.body.binding.bindingRevisionId })).status, 409);
  await h.service.dispatchNext();
  assert.equal(calls[0]?.configuration.instructions, agent.instructions);
  assert.equal(calls[0]?.configuration.selectedModel, agent.binding.selectedModel);
  assert.deepEqual(calls[0]?.configuration.skills, [skill]);
  assert.equal((await h.api(path, command)).status, 201);
});

test('message acceptance preserves whitespace-only plain text and rejects malformed Unicode before dispatch', async t => {
  const h = await setup(t);
  const agent = (await h.api<Agent>('/v1/agents', { name:'Writer', model:'gpt-6-astra', instructions:'Reply.' })).body;
  const path = `/v1/conversations/${agent.homeConversationId}/messages`;
  const command = { requestId:'whitespace', text:' \t\r\n ', recipientAgentId:agent.agentId, bindingRevisionId:agent.binding.bindingRevisionId, recoveryEpoch:h.service.installation.recoveryEpoch, skills:[] };
  const result = await h.api<Acceptance>(path,command);
  assert.equal(result.status,201);
  assert.equal(result.body.snapshot.messages[0]?.text,command.text);
  assert.equal((await h.api(path,{ ...command,requestId:'invalid-unicode',text:'\ud800' })).status,400);
});

test('service shutdown disposes its idle runtime exactly once and awaits cleanup confirmation', async t => {
  let disposalCalls = 0, confirm!: () => void;
  const cleanup = new Promise<void>(resolve => { confirm = resolve; });
  const runtime = { async execute() { return { kind:'completed' as const, text:'unused', effectiveModel:null }; }, async dispose() { disposalCalls++; await cleanup; } };
  const h = await setup(t,{ runtime });
  let closed = false;
  const closing = h.service.close().then(() => { closed = true; });
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(disposalCalls,1);
  assert.equal(closed,false);
  confirm(); await closing; await h.service.close();
  assert.equal(disposalCalls,1);
  assert.equal(closed,true);
});

test('active service shutdown confirms owned child exit and leaves no late reply or automatic retry', async t => {
  const workspace = fileURLToPath(new URL('../runner',import.meta.url));
  const owned = launchOwnedProcess({ executable:process.execPath,args:[fileURLToPath(new URL('../runner/owned-process-fixture.mjs',import.meta.url))],cwd:workspace,env:{ PATH:'/usr/bin:/bin',FORT_SYNTHETIC_MODE:'ignore-term' },deadlineMs:5_000,terminationGraceMs:50,killGraceMs:500 });
  t.after(() => owned.dispose());
  let started!: () => void;
  const entered = new Promise<void>(resolve => { started=resolve; });
  owned.connection.subscribe({ frame:bytes => { const frame=JSON.parse(Buffer.from(bytes).toString()); if(frame.result?.turn?.status==='inProgress')started(); },closed:()=>{} });
  const runtime = new CodexChat({ connection:owned.connection,workspace,runtimeVersion:'0.157.1' });
  const h = await setup(t,{ runtime });
  const agent = (await h.api<Agent>('/v1/agents',{ name:'Writer',model:'gpt-6-astra',instructions:'Reply.' })).body;
  await h.api(`/v1/conversations/${agent.homeConversationId}/messages`,{ requestId:'active-shutdown',text:'Synthetic active turn',recipientAgentId:agent.agentId,bindingRevisionId:agent.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,skills:[] });
  const turn = h.service.dispatchNext(); await entered;
  await h.service.close(); await turn;
  assert.equal(owned.cleanupConfirmed,true);
  assert.equal(owned.exit?.signal,'SIGKILL');
  await h.restart();
  const snapshot=(await h.api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body;
  assert.equal(snapshot.runs[0]?.state,'outcomeUnknown');
  assert.equal(snapshot.messages.length,1);
  assert.equal(await h.service.dispatchNext(),false);
});
