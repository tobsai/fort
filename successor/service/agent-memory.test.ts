import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createApplication } from './openai.ts';
import { CODEX_RUNTIME } from '../runner/codex-descriptor.ts';
import type { Agent, Snapshot } from '../contracts/v1.ts';
import type { AgentMemoryStatus, MemoryEntry, MemoryProvider, MemoryProviderInfo } from '../contracts/memory.ts';
import type { ChatInput } from '../runner/chat-runtime.ts';
import { createNotesMemory } from '../modules/memory/notes.ts';
import { createScratchpadMemory } from '../modules/memory/fixtures/scratchpad.ts';

type Overview = { providers: MemoryProviderInfo[]; defaultProviderId: string; agents: AgentMemoryStatus[] };
type Detail = AgentMemoryStatus & { entries: MemoryEntry[]; carried?: number; carryDetail?: string };

async function setup(t: TestContext, reply: (input: ChatInput) => string) {
  const directory = await mkdtemp(join(tmpdir(), 'fort-agent-memory-'));
  const notes = createNotesMemory({ directory: join(directory, 'memory') }), pad = createScratchpadMemory();
  const inputs: ChatInput[] = [];
  const runtime = { async execute(input: ChatInput) { inputs.push(input); return { kind: 'completed' as const, text: reply(input), effectiveModel: null }; }, async dispose() {} };
  const make = (memory?: { providers: MemoryProvider[]; defaultProviderId?: string }) => createApplication({ dataDirectory: join(directory, 'ledger'), provider: CODEX_RUNTIME, runtime, ...(memory ? { memory } : {}) });
  let service = make({ providers: [notes, pad], defaultProviderId: 'fort-notes' });
  await service.start();
  t.after(async () => { await service.close(); notes.close?.(); await rm(directory, { recursive: true, force: true }); });
  let token = '';
  const enroll = async () => {
    const response = await fetch(service.baseURL + '/v1/devices/enroll', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...service.issueEnrollment(), deviceName: 'Memory fixture' }) });
    token = ((await response.json()) as { deviceToken: string }).deviceToken;
  };
  await enroll();
  async function api<T>(path: string, body?: unknown) {
    const response = await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token, 'X-Fort-Installation-Id': service.installation.installationId }, body: body === undefined ? undefined : JSON.stringify(body) });
    return { status: response.status, body: await response.json() as T };
  }
  const agent = (await api<Agent>('/v1/agents', { name: 'Keeper', model: 'exact-model', instructions: 'Remember things.' })).body;
  const memory = () => api<Detail>(`/v1/agents/${agent.agentId}/memory`);
  return { api, agent, inputs, notes, pad, memory,
    async send(requestId: string, text: string) {
      const sent = await api(`/v1/conversations/${agent.homeConversationId}/messages`, { requestId, text, recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: service.installation.recoveryEpoch, skills: [] });
      assert.equal(sent.status, 201); await service.dispatchNext();
      return (await api<Snapshot>(`/v1/conversations/${agent.homeConversationId}/snapshot`)).body;
    },
    async select(providerId: string, carry?: boolean) {
      const current = (await memory()).body.choice.revisionId;
      return api<Detail>(`/v1/agents/${agent.agentId}/memory/select`, { expectedRevisionId: current, providerId, ...(carry === undefined ? {} : { carry }) });
    },
    async restart(memory?: { providers: MemoryProvider[]; defaultProviderId?: string }) { await service.close(); service = make(memory); await service.start(); },
  };
}
const last = (snapshot: Snapshot) => snapshot.messages.at(-1)!;

test('an Agent writes its own memory, gets it back on a later turn, and the owner sees and edits it', async t => {
  const h = await setup(t, input => {
    if (input.text.startsWith('My dog')) return 'Lovely name.\n<fort-memory>The owner’s dog is called Biscuit.</fort-memory>';
    const dog = input.agentMemory?.entries.find(entry => entry.text.includes('Biscuit'));
    if (input.text.startsWith('Forget')) return `Done. <fort-forget id="${dog?.entryId}"/>`;
    return dog ? 'Your dog is Biscuit.' : 'I don’t know.';
  });
  const overview = (await h.api<Overview>('/v1/memory')).body;
  assert.deepEqual(overview.providers.map(provider => provider.id), ['none', 'fort-notes', 'scratchpad']);
  assert.equal(overview.defaultProviderId, 'fort-notes');
  assert.deepEqual(overview.agents.map(agent => [agent.agentName, agent.choice.providerId, agent.state, agent.total]), [['Keeper', 'fort-notes', 'ready', 0]]);

  const first = await h.send('first', 'My dog is called Biscuit.');
  assert.equal(last(first).text, 'Lovely name.', 'the tag never reaches the transcript');
  assert.ok(h.inputs[0]!.agentMemory?.canRemember);
  const kept = (await h.memory()).body;
  assert.equal(kept.total, 1);
  assert.equal(kept.entries[0]!.text, 'The owner’s dog is called Biscuit.');
  assert.equal(kept.entries[0]!.source?.messageId, last(first).messageId);

  assert.equal(last(await h.send('second', 'What is my dog called?')).text, 'Your dog is Biscuit.');
  assert.equal(h.inputs[1]!.agentMemory?.provider.id, 'fort-notes');

  const added = await h.api<{ entry: MemoryEntry }>(`/v1/agents/${h.agent.agentId}/memory/remember`, { text: 'The owner lives in Trophy Club.' });
  assert.equal(added.status, 201);
  assert.equal((await h.api<{ forgotten: number }>(`/v1/agents/${h.agent.agentId}/memory/forget`, { entryIds: [added.body.entry.entryId] })).body.forgotten, 1);

  // The Agent can forget only a note it was shown this turn.
  assert.equal(last(await h.send('third', 'Forget my dog’s name, please.')).text, 'Done.');
  assert.equal((await h.memory()).body.total, 0);
});

test('switching memory systems keeps each one’s notes, so switching back finds them; "No memory" sends nothing', async t => {
  const h = await setup(t, input => input.text === 'remember' ? 'ok <fort-memory>Owner likes jazz.</fort-memory>' : `recalled ${input.agentMemory?.entries.length ?? 'nothing'}`);
  await h.send('a', 'remember');
  assert.equal((await h.memory()).body.total, 1);

  const stale = await h.api(`/v1/agents/${h.agent.agentId}/memory/select`, { expectedRevisionId: 'not-current', providerId: 'scratchpad' });
  assert.equal(stale.status, 409);
  const toPad = await h.select('scratchpad');
  assert.equal(toPad.status, 200);
  assert.deepEqual([toPad.body.choice.providerId, toPad.body.provider?.name, toPad.body.total], ['scratchpad', 'Scratchpad', 0]);
  assert.equal(last(await h.send('b', 'jazz?')).text, 'recalled 0');
  await h.send('c', 'remember');
  assert.equal(h.pad.entries.get(h.agent.agentId)?.length, 1, 'the new system took the new note');
  assert.equal((await h.notes.list({ agentId: h.agent.agentId, limit: 10 })).total, 1, 'the old system kept its own');

  const off = await h.select('none');
  assert.equal(off.body.state, 'off');
  const plain = await h.send('d', 'remember');
  assert.equal(h.inputs.at(-1)!.agentMemory, undefined, 'no memory means nothing recalled and no memory instructions');
  assert.equal(last(plain).text, 'ok <fort-memory>Owner likes jazz.</fort-memory>', 'with no memory the reply is kept as written');

  assert.equal((await h.select('fort-notes')).body.total, 1);
  assert.equal(last(await h.send('e', 'jazz?')).text, 'recalled 1');

  const carry = await h.select('scratchpad', true);
  assert.equal(carry.body.carried, 0);
  assert.match(carry.body.carryDetail ?? '', /can’t copy/);
  assert.equal((await h.api(`/v1/agents/${h.agent.agentId}/memory/select`, { expectedRevisionId: carry.body.choice.revisionId, providerId: 'mystery' })).status, 400);
});

test('notes carry across a switch between systems that can export and import', async t => {
  const h = await setup(t, () => 'ok <fort-memory>Owner takes the 7:15 train.</fort-memory>');
  await h.send('a', 'hi');
  // A second Fort Notes under another id stands in for any system that can import.
  const directory = await mkdtemp(join(tmpdir(), 'fort-notes-b-'));
  const other = { ...createNotesMemory({ directory }), id: 'notes-b', name: 'Notes B' };
  t.after(async () => { other.close?.(); await rm(directory, { recursive: true, force: true }); });
  await h.restart({ providers: [h.notes, other], defaultProviderId: 'fort-notes' });
  const moved = await h.select('notes-b', true);
  assert.equal(moved.body.carried, 1);
  assert.equal(moved.body.total, 1);
});

test('a removed memory module shows as missing and stops sends with a clear reason; re-adding it brings the notes back', async t => {
  const h = await setup(t, input => input.text === 'remember' ? '<fort-memory>Owner is called Toby.</fort-memory>Hi.' : 'plain');
  await h.send('a', 'remember');
  await h.restart();
  const missing = (await h.api<Overview>('/v1/memory')).body;
  assert.deepEqual(missing.providers.map(provider => provider.id), ['none']);
  assert.equal(missing.agents[0]!.state, 'missing');
  const refused = await h.send('b', 'hello');
  assert.equal(refused.runs.at(-1)!.state, 'failed');
  assert.match(refused.runs.at(-1)!.statusDetail ?? '', /memory system \(fort-notes\) isn’t installed/);
  assert.deepEqual(refused.messages.map(message => message.text), ['remember', 'Hi.', 'hello'], 'history stays readable');

  await h.restart({ providers: [h.notes] });
  assert.equal((await h.memory()).body.total, 1);
  assert.equal(last(await h.send('c', 'hello again')).text, 'plain');
});

test('a module that fails to recall fails the turn before anything is sent', async t => {
  let down = false;
  const directory = await mkdtemp(join(tmpdir(), 'fort-agent-memory-down-'));
  const pad = createScratchpadMemory({ failRecall: () => down });
  let sent = 0;
  const service = createApplication({ dataDirectory: directory, provider: CODEX_RUNTIME, memory: { providers: [pad] },
    runtime: { async execute() { sent++; return { kind: 'completed' as const, text: 'ok', effectiveModel: null }; }, async dispose() {} } });
  await service.start();
  t.after(async () => { await service.close(); await rm(directory, { recursive: true, force: true }); });
  const enrolled = await (await fetch(service.baseURL + '/v1/devices/enroll', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...service.issueEnrollment(), deviceName: 'x' }) })).json() as { deviceToken: string };
  const call = async (path: string, body?: unknown) => (await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + enrolled.deviceToken, 'X-Fort-Installation-Id': service.installation.installationId }, body: body === undefined ? undefined : JSON.stringify(body) })).json() as Promise<any>;
  const agent = await call('/v1/agents', { name: 'Pad', model: 'm', instructions: 'i' }) as Agent;
  down = true;
  await call(`/v1/conversations/${agent.homeConversationId}/messages`, { requestId: 'r', text: 'hi', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: service.installation.recoveryEpoch, skills: [] });
  await service.dispatchNext();
  const snapshot = await call(`/v1/conversations/${agent.homeConversationId}/snapshot`) as Snapshot;
  assert.equal(snapshot.runs[0]!.state, 'failed');
  assert.match(snapshot.runs[0]!.statusDetail ?? '', /Scratchpad couldn’t recall/);
  assert.equal(sent, 0);
});

test('work canceled while memory is recalled stays canceled and is never sent', async t => {
  let release = () => {}, reached = () => {};
  const atRecall = new Promise<void>(resolve => { reached = resolve; });
  const directory = await mkdtemp(join(tmpdir(), 'fort-agent-memory-cancel-'));
  const pad = createScratchpadMemory({ beforeRecall: () => { reached(); return new Promise(resolve => { release = resolve; }); } });
  let sent = 0;
  const service = createApplication({ dataDirectory: directory, provider: CODEX_RUNTIME, memory: { providers: [pad] },
    runtime: { async execute() { sent++; return { kind: 'completed' as const, text: 'ok', effectiveModel: null }; }, async dispose() {} } });
  await service.start();
  t.after(async () => { await service.close(); await rm(directory, { recursive: true, force: true }); });
  const enrolled = await (await fetch(service.baseURL + '/v1/devices/enroll', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...service.issueEnrollment(), deviceName: 'x' }) })).json() as { deviceToken: string };
  const call = async (path: string, body?: unknown) => (await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + enrolled.deviceToken, 'X-Fort-Installation-Id': service.installation.installationId }, body: body === undefined ? undefined : JSON.stringify(body) })).json() as Promise<any>;
  const agent = await call('/v1/agents', { name: 'Pad', model: 'm', instructions: 'i' }) as Agent;
  await call(`/v1/conversations/${agent.homeConversationId}/messages`, { requestId: 'r', text: 'hi', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: service.installation.recoveryEpoch, skills: [] });
  const dispatched = service.dispatchNext();
  await atRecall;
  await call('/v1/requests/r/cancel', { recoveryEpoch: service.installation.recoveryEpoch, conversationId: agent.homeConversationId, bindingRevisionId: agent.binding.bindingRevisionId });
  release(); await dispatched; await service.dispatchNext();
  const snapshot = await call(`/v1/conversations/${agent.homeConversationId}/snapshot`) as Snapshot;
  assert.equal(snapshot.runs[0]!.state, 'canceled');
  assert.equal(sent, 0);
});
