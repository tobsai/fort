import {test,type TestContext} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {createOpenAIService} from './openai.ts';
import type {Agent,Snapshot,ModelCatalog} from '../contracts/v1.ts';
import type {ChatInput} from '../runner/chat-runtime.ts';
async function setup(t: TestContext, options: Omit<Parameters<typeof createOpenAIService>[0],'dataDirectory'> = {}) {
  const directory = await mkdtemp(join(tmpdir(), 'fort-rebuilt-unit-'));
  let service = createOpenAIService({ dataDirectory: directory, ...options });
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
  return { api, reenroll, directory, get service() { return service; }, async restart() { await service.close(); service = createOpenAIService({ dataDirectory: directory, ...options }); await service.start(); } };
}


test('agent identity, model and avatar persist without changing an accepted turn or its Home',async t=>{
 const calls:ChatInput[]=[];const h=await setup(t,{runtime:{async execute(input){calls.push(input);return {kind:'completed',text:'Synthetic reply',effectiveModel:null};},async dispose(){}}});
 const first=(await h.api<Agent>('/v1/agents',{name:'Writer',instructions:'  You are my careful editor.\nKeep my voice.  ',model:'gpt-6-astra',avatar:'sprout'})).body;
 assert.equal(first.avatar,'sprout');
 await h.api(`/v1/conversations/${first.homeConversationId}/messages`,{requestId:'before',recipientAgentId:first.agentId,bindingRevisionId:first.binding.bindingRevisionId,recoveryEpoch:h.service.installation.recoveryEpoch,text:'Please review this.',skills:[]});
 const updated=(await h.api<Agent>(`/v1/agents/${first.agentId}`,{name:'Editor',instructions:'Be concise.',model:'exact-other-model',avatar:'ember',expectedBindingRevisionId:first.binding.bindingRevisionId})).body;
 assert.equal(updated.homeConversationId,first.homeConversationId);assert.equal(updated.agentId,first.agentId);assert.equal(updated.avatar,'ember');
 await h.service.dispatchNext();assert.equal(calls[0]?.configuration.instructions,first.instructions);assert.equal(calls[0]?.configuration.selectedModel,'gpt-6-astra');
 await h.restart();const agents=(await h.api<{agents:Agent[]}>('/v1/agents')).body.agents;assert.deepEqual(agents,[updated]);
 assert.equal((await h.api<Snapshot>(`/v1/conversations/${first.homeConversationId}/snapshot`)).body.messages.length,2);
 assert.equal((await h.api(`/v1/agents/${first.agentId}`,{name:'Stale',instructions:'Overwrite',model:'other',avatar:'orbit',expectedBindingRevisionId:first.binding.bindingRevisionId})).status,409);
 assert.equal((await h.api('/v1/agents',{name:'Bad',instructions:'x',model:'gpt-6-astra',avatar:'https://untrusted.invalid/avatar.gif'})).status,400);
 assert.equal((await h.api('/v1/agents',{name:'Bad',instructions:'x'.repeat(16385),model:'gpt-6-astra'})).status,400);
});
test('older callers preserve an existing avatar and legacy agents load without one',async t=>{
 const h=await setup(t);const old=(await h.api<Agent>('/v1/agents',{name:'Legacy',instructions:'Existing identity',model:'gpt-6-astra'})).body;
 assert.equal(old.avatar,undefined);
 const a=(await h.api<Agent>(`/v1/agents/${old.agentId}`,{name:old.name,instructions:old.instructions,model:old.binding.selectedModel,avatar:'orbit',expectedBindingRevisionId:old.binding.bindingRevisionId})).body;
 const b=(await h.api<Agent>(`/v1/agents/${a.agentId}`,{name:a.name,instructions:a.instructions,model:a.binding.selectedModel,expectedBindingRevisionId:a.binding.bindingRevisionId})).body;
 assert.equal(b.avatar,'orbit');
});
test('model choices require owner authentication and report unavailable runtime or discovery failure honestly',async t=>{
 const h=await setup(t);assert.equal((await fetch(h.service.baseURL+'/v1/models')).status,401);
 assert.equal((await h.api<ModelCatalog>('/v1/models')).body.state,'unavailable');
 const failed=await setup(t,{runtime:{async models(){throw new Error('SECRET');},async execute(){return {kind:'notStarted',message:'fixture'};},async dispose(){}}});
 const result=await failed.api<ModelCatalog>('/v1/models');assert.equal(result.body.state,'error');assert.equal(JSON.stringify(result).includes('SECRET'),false);
 const expected:ModelCatalog={provider:'codex',state:'ready',checkedAt:'2026-10-17T00:00:00Z',models:[{id:'exact-model',name:'Exact model',availability:'available'}]};
 const ready=await setup(t,{runtime:{async models(){return expected;},async execute(){return {kind:'notStarted',message:'fixture'};},async dispose(){}}});
 assert.deepEqual((await ready.api<ModelCatalog>('/v1/models')).body,expected);
});
