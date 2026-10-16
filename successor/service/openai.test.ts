import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createOpenAIService } from './openai.ts';
import type { Agent, Conversation, InstructionSkill, Installation } from '../contracts/v1.ts';

async function setup(t: TestContext) {
  const directory = await mkdtemp(join(tmpdir(), 'fort-rebuilt-unit-'));
  let service = createOpenAIService({ dataDirectory: directory });
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
  return { api, get service() { return service; }, async restart() { await service.close(); service = createOpenAIService({ dataDirectory: directory }); await service.start(); } };
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
