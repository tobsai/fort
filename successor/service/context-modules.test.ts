import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { DatabaseSync } from 'node:sqlite';
import { createApplication } from './openai.ts';
import { CODEX_RUNTIME } from '../runner/codex-descriptor.ts';
import type { ContextCompiler } from '../contracts/context.ts';
import type { Acceptance, Agent, ConversationMemory, InstructionSkill, RoutineRevision, RuntimeDescriptor, Snapshot } from '../contracts/v1.ts';
import type { ChatInput } from '../runner/chat-runtime.ts';
import { LOCAL_CONTEXT } from '../modules/local-context.ts';

async function setup(t: TestContext, context?: ContextCompiler | null, reply?: (input: ChatInput) => string, provider: RuntimeDescriptor = CODEX_RUNTIME) {
  const directory = await mkdtemp(join(tmpdir(), 'fort-context-modules-'));
  const runtime = { async execute(input: ChatInput) { return { kind: 'completed' as const, text: reply ? reply(input) : input.text, effectiveModel: null }; }, async dispose() {} };
  let service = createApplication({ dataDirectory: directory, provider, context, runtime });
  await service.start();
  t.after(async () => { await service.close(); await rm(directory, { recursive: true, force: true }); });
  const response = await fetch(service.baseURL + '/v1/devices/enroll', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...service.issueEnrollment(), deviceName: 'Context fixture' }) });
  assert.equal(response.status, 201);
  const device = await response.json() as { deviceToken: string };
  async function api<T>(path: string, body?: unknown) {
    const response = await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + device.deviceToken, 'X-Fort-Installation-Id': service.installation.installationId }, body: body === undefined ? undefined : JSON.stringify(body) });
    return { status: response.status, body: await response.json() as T };
  }
  const agent = (await api<Agent>('/v1/agents', { name: 'Context', model: 'exact-model', instructions: 'Use exact context.' })).body;
  const path = '/v1/conversations/' + agent.homeConversationId;
  return { api, path, agent, directory, get service() { return service; }, async restart(context?: ContextCompiler | null) {
    await service.close(); service = createApplication({ dataDirectory: directory, provider, context, runtime }); await service.start();
  }, send(requestId: string, memoryRevisionId?: string) {
    return api(path + '/messages', { requestId, text: 'Exact plain text', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: service.installation.recoveryEpoch, skills: [], ...(memoryRevisionId ? { memoryRevisionId } : {}) });
  } };
}

test('an absent or disabled context module keeps text and history usable while refusing required memory', async t => {
  for (const context of [undefined, null]) await t.test(context === null ? 'disabled' : 'absent', async t => {
    const h = await setup(t, context);
    const status = await h.api<{ state: string }>('/v1/modules/context');
    assert.equal(status.status, 200); assert.equal(status.body.state, context === null ? 'disabled' : 'missing');
    assert.equal((await h.send('plain')).status, 201); await h.service.dispatchNext();
    const memory = (await h.api<ConversationMemory>(h.path + '/memory', { recoveryEpoch: h.service.installation.recoveryEpoch, expectedRevisionId: null, text: 'Retained note without an active compiler' })).body;
    assert.equal((await h.send('needs-context', memory.revisionId)).status, 409);
    const snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
    assert.deepEqual(snapshot.messages.map(message => message.text), ['Exact plain text', 'Exact plain text']);
    assert.deepEqual(snapshot.memory, memory);
    assert.equal(h.service.inspectSession().receipts.length, 1);
  });
});

test('the selected context compiler prepares sealed runtime input and its version is retained with the accepted work', async t => {
  let calls = 0;
  const compiler: ContextCompiler = { protocolVersion: 1, id: 'fixture-context', version: '1', compile({ configuration }) {
    calls++; return { kind: 'ready', instructions: 'Prepared note', bundles: [], memory: configuration.memory };
  } };
  const h = await setup(t, compiler, input => {
    assert.equal(input.context.kind, 'prepared');
    if (input.context.kind !== 'prepared') throw new Error('Expected prepared context');
    assert.deepEqual(input.context.module, { id: 'fixture-context', version: '1' });
    assert.ok(Object.isFrozen(input.context));
    return input.context.content.instructions + ': ' + input.context.content.memory?.text;
  });
  const memory = (await h.api<ConversationMemory>(h.path + '/memory', { recoveryEpoch: h.service.installation.recoveryEpoch, expectedRevisionId: null, text: 'Cedar' })).body;
  assert.equal((await h.send('prepared', memory.revisionId)).status, 201);
  assert.equal(calls, 0);
  await h.service.dispatchNext();
  const snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
  assert.equal(snapshot.runs[0].state, 'completed');
  assert.deepEqual(snapshot.runs[0].configuration?.contextModule, { id: 'fixture-context', version: '1' });
  assert.equal(snapshot.messages[1].text, 'Prepared note: Cedar');
  assert.equal(calls, 1);
});

test('removing and re-adding the local compiler retains notes and completed history without replay', async t => {
  const h = await setup(t, LOCAL_CONTEXT, input => input.context.kind === 'prepared' ? input.context.content.memory?.text ?? input.text : input.text);
  const memory = (await h.api<ConversationMemory>(h.path + '/memory', { recoveryEpoch: h.service.installation.recoveryEpoch, expectedRevisionId: null, text: 'Retained Cedar' })).body;
  assert.equal((await h.send('first', memory.revisionId)).status, 201); await h.service.dispatchNext();
  await h.restart();
  assert.equal((await h.api<{ state: string }>('/v1/modules/context')).body.state, 'missing');
  assert.equal((await h.send('removed', memory.revisionId)).status, 409);
  let snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
  assert.deepEqual(snapshot.messages.map(message => message.text), ['Exact plain text', 'Retained Cedar']);
  assert.deepEqual(snapshot.memory, memory);
  await h.restart(LOCAL_CONTEXT);
  assert.equal(await h.service.dispatchNext(), false);
  assert.equal((await h.send('readded', memory.revisionId)).status, 201); await h.service.dispatchNext();
  snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
  assert.deepEqual(snapshot.messages.map(message => message.text), ['Exact plain text', 'Retained Cedar', 'Exact plain text', 'Retained Cedar']);
});

test('a context failure is visible, never calls the runtime and leaves explicit plain text usable', async t => {
  let calls = 0;
  const h = await setup(t, { ...LOCAL_CONTEXT, compile() { throw new Error('SYNTHETIC_PRIVATE_FAILURE_DETAIL'); } }, input => { calls++; return input.text; });
  const epoch = h.service.installation.recoveryEpoch;
  const memory = (await h.api<ConversationMemory>(h.path + '/memory', { recoveryEpoch: epoch, expectedRevisionId: null, text: 'Keep this note in history' })).body;
  assert.equal((await h.send('failed-context', memory.revisionId)).status, 201);
  await h.service.dispatchNext();
  const snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
  assert.equal(snapshot.runs[0].state, 'failed'); assert.equal(calls, 0);
  assert.equal(JSON.stringify(snapshot).includes('SYNTHETIC_PRIVATE_FAILURE_DETAIL'), false);
  assert.equal((await h.api<{ state: string }>('/v1/modules/context')).body.state, 'failed');
  assert.equal((await h.send('still-needs-context', memory.revisionId)).status, 409);
  const cleared = (await h.api<ConversationMemory>(h.path + '/memory', { recoveryEpoch: epoch, expectedRevisionId: memory.revisionId, text: '' })).body;
  assert.equal((await h.send('explicit-plain', cleared.revisionId)).status, 201);
  await h.service.dispatchNext(); assert.equal(calls, 1);
  await h.restart(LOCAL_CONTEXT);
  assert.equal((await h.api<{ state: string }>('/v1/modules/context')).body.state, 'ready');
  assert.equal(await h.service.dispatchNext(), false);
});

test('queued context-bearing work cannot silently adopt a changed or previously unpinned compiler', async t => {
  let calls = 0;
  const h = await setup(t, LOCAL_CONTEXT, input => { calls++; return input.text; });
  const skill = (await h.api<InstructionSkill>('/v1/skills', { name: 'Pinned skill', instructions: 'Use the exact reviewed text.' })).body;
  assert.ok(skill.bundle);
  await h.api('/v1/skills/' + skill.skillId + '/review', { revisionId: skill.revisionId, digest: skill.bundle.digest, enable: true });
  const routine = (await h.api<RoutineRevision>('/v1/routines', { definition: { schemaVersion: 1, name: 'Pinned compiler', agentId: h.agent.agentId, conversationId: h.agent.homeConversationId, bindingRevisionId: h.agent.binding.bindingRevisionId, instructions: 'Run the task.', inputs: [], skills: [{ skillId: skill.skillId, revisionId: skill.revisionId, digest: skill.bundle.digest }], steps: [{ stepId: 'task', name: 'Task', type: 'task', instructions: 'Produce a reply.', inputNames: [], outputStepIds: [] }] } })).body;
  const path = '/v1/routines/' + routine.routineId + '/run';
  const command = { requestId: 'pinned-compiler', routineRevisionId: routine.revisionId, recoveryEpoch: h.service.installation.recoveryEpoch, inputs: {} };
  const accepted = (await h.api<Acceptance>(path, command)).body;
  await h.restart({ ...LOCAL_CONTEXT, version: '2' });
  await h.service.dispatchNext();
  const snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
  assert.equal(snapshot.runs[0].state, 'failed'); assert.equal(calls, 0);
  assert.deepEqual(snapshot.runs[0].configuration?.contextModule, { id: 'fort-local-context', version: '1' });
  assert.deepEqual((await h.api<Acceptance>(path, command)).body.receipt, accepted.receipt);
  await h.restart(LOCAL_CONTEXT);
  const legacyCommand = { ...command, requestId: 'legacy-without-compiler-pin' };
  const legacy = (await h.api<Acceptance>(path, legacyCommand)).body;
  await h.service.close();
  const db = new DatabaseSync(join(h.directory, 'openai-candidate-b.sqlite'));
  try { db.prepare("UPDATE entities SET data=json_remove(data,'$.configuration.contextModule') WHERE kind='runs' AND id=?").run(legacy.receipt.runId); }
  finally { db.close(); }
  await h.restart(LOCAL_CONTEXT); await h.service.dispatchNext();
  const legacySnapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
  assert.equal(legacySnapshot.runs.find(run => run.runId === legacy.receipt.runId)?.state, 'failed');
  assert.equal(calls, 0);
  assert.deepEqual((await h.api<Acceptance>(path, legacyCommand)).body.receipt, legacy.receipt);
});

test('the local compiler prepares complete reviewed bundles for an explicitly selected non-Codex runtime', async t => {
  const provider: RuntimeDescriptor = { provider: 'independent-fixture', name: 'Independent fixture', runtimeVersion: '1', hostId: 'fixture-host', session: 'sessionful', capabilities: ['plainText', 'skills'] };
  const h = await setup(t, LOCAL_CONTEXT, input => {
    assert.equal(input.context.kind, 'prepared');
    if (input.context.kind !== 'prepared') throw new Error('Expected complete prepared context');
    const bundle = input.context.content.bundles[0];
    assert.deepEqual(bundle.bundle.supportedRuntimes, ['independent-fixture']);
    return bundle.textFiles[0].text;
  }, provider);
  const skill = (await h.api<InstructionSkill>('/v1/skills', { name: 'Independent bundle', instructions: 'Exact non-Codex instructions' })).body;
  assert.ok(skill.bundle);
  await h.api('/v1/skills/' + skill.skillId + '/review', { revisionId: skill.revisionId, digest: skill.bundle.digest, enable: true });
  const accepted = await h.api(h.path + '/messages', { requestId: 'independent-context', text: 'Use the reviewed bundle', recipientAgentId: h.agent.agentId, bindingRevisionId: h.agent.binding.bindingRevisionId, recoveryEpoch: h.service.installation.recoveryEpoch, skills: [{ skillId: skill.skillId, revisionId: skill.revisionId }] });
  assert.equal(accepted.status, 201); await h.service.dispatchNext();
  const snapshot = (await h.api<Snapshot>(h.path + '/snapshot')).body;
  assert.equal(snapshot.runs[0].state, 'completed');
  assert.equal(snapshot.messages[1].text, 'Exact non-Codex instructions');
});
