import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { ChatInput, ChatRuntime } from '../runner/chat-runtime.ts';
import { createOpenAIService } from './openai.ts';
import type { Acceptance, Agent, Conversation, InstructionSkill, Installation, Snapshot } from '../contracts/v1.ts';

async function setup(t: TestContext, options: { runtime?: ChatRuntime; holdExistingQueue?: boolean } = {}) {
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
  return { api, get service() { return service; }, async restart() { await service.close(); service = createOpenAIService({ dataDirectory: directory, ...options }); await service.start(); } };
}

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
  assert.deepEqual((await h.api<Installation>('/v1/installation')).body, identity);
  assert.deepEqual((await h.api<{ agents: Agent[] }>('/v1/agents')).body.agents, [writer.body, reviewer.body]);
  assert.deepEqual((await h.api<{ skills: InstructionSkill[] }>('/v1/skills')).body.skills, [skill.body]);
  const rooms = (await h.api<{ conversations: Conversation[] }>('/v1/conversations')).body.conversations;
  assert.equal(rooms.length, 3);
  assert.deepEqual(rooms.find(r => r.kind === 'shared'), room.body);
  assert.equal((await fetch(h.service.baseURL + '/v1/agents')).status, 401);
});


test('shared submission pins its recipient and selected skill, saves one attributed reply and replays the original receipt', async t => {
  const inputs: ChatInput[] = [];
  const runtime: ChatRuntime = { async execute(input) { inputs.push(structuredClone(input)); return { kind: 'completed', text: 'red notebook [CHECKED]', effectiveModel: null }; } };
  const h = await setup(t, { runtime });
  const writer = (await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Write briefly.' })).body;
  const reviewer = (await h.api<Agent>('/v1/agents', { name: 'Reviewer', model: 'gpt-6-astra', instructions: 'Check the claim.' })).body;
  const skill = (await h.api<InstructionSkill>('/v1/skills', { name: 'Check marker', instructions: 'End with [CHECKED].' })).body;
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
  assert.deepEqual((await h.api<Acceptance>(path, command)).body, accepted.body);
  assert.equal((await h.api(path, { ...command, text: 'changed' })).status, 409);
  assert.equal(await h.service.dispatchNext(), false);
});

test('queued turns receive completed prior context in their own conversation, including after restart', async t => {
  const inputs: ChatInput[] = [];
  const runtime: ChatRuntime = { async execute(input) { inputs.push(structuredClone(input)); return { kind: 'completed', text: 'cedar', effectiveModel: null }; } };
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
  const runtime: ChatRuntime = { async execute(input, signal) {
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
  const h = await setup(t, { runtime: { async execute(input) { calls.push(input); return { kind: 'completed', text: 'Done.', effectiveModel: null }; } } });
  const agent = (await h.api<Agent>('/v1/agents', { name: 'Writer', model: 'gpt-6-astra', instructions: 'Original instructions.' })).body;
  const skill = (await h.api<InstructionSkill>('/v1/skills', { name: 'Marker', instructions: 'Use [ONE].' })).body;
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
