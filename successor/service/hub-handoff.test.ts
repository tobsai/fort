import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseHandoffPayload, rebaseRow, selectHandoff } from './hub-handoff.ts';

// Spec 063, revision 2026-10-31: what a Mac hands its hub, decided without the network.
const row = (kind: string, id: string, data: object) => ({ kind, id, data: JSON.stringify(data) });
const agent = (agentId: string, provider = 'codex') => row('agents', agentId, { agentId, name: agentId, homeConversationId: 'home-' + agentId, binding: { provider, runtimeVersion: '1', hostId: 'mac' } });
const home = (agentId: string) => row('conversations', 'home-' + agentId, { conversationId: 'home-' + agentId, kind: 'home', participantAgentIds: [agentId] });
const runtimes = [{ provider: 'codex', runtimeVersion: '1' }];

test('only Agents the hub can run move, with their chats, routines, schedules and portraits, and never history', () => {
  const rows = [agent('a'), home('a'), agent('b', 'claude'), home('b'),
    row('routines', 'r', { agentSnapshot: { agentId: 'a' }, definition: { conversationId: 'home-a' } }), row('routineRevisions', 'r1', { agentSnapshot: { agentId: 'a' }, definition: { conversationId: 'home-a' } }),
    row('schedules', 's', { definition: { routineId: 'r' } }), row('portraits', 'a', { revision: 'p', png: '' }), row('portraits', 'b', { revision: 'p', png: '' }),
    row('messages', 'm', { conversationId: 'home-a' }), row('runs', 'x', { conversationId: 'home-a' })];
  const { payload, staying } = selectHandoff(rows, { sourceInstallationId: 'mac', runtimes });
  assert.deepEqual(payload.agentIds, ['a']);
  assert.deepEqual(staying, [{ agentId: 'b', name: 'b', reason: 'runtime' }]);
  assert.deepEqual(payload.rows.map(item => item.kind + ':' + item.id), ['agents:a', 'routines:r', 'routineRevisions:r1', 'schedules:s', 'conversations:home-a', 'portraits:a']);
  assert.deepEqual(parseHandoffPayload(JSON.parse(JSON.stringify(payload))), payload);
});

test('an Agent whose routine shares a chat with an Agent that stays, stays too', () => {
  const shared = row('conversations', 'shared', { conversationId: 'shared', participantAgentIds: ['a', 'b'] });
  const rows = [agent('a'), home('a'), agent('b', 'claude'), home('b'), shared, row('routines', 'r', { agentSnapshot: { agentId: 'a' }, definition: { conversationId: 'shared' } })];
  const { payload, staying } = selectHandoff(rows, { sourceInstallationId: 'mac', runtimes });
  assert.deepEqual(payload.agentIds, []);
  assert.deepEqual(staying.map(item => [item.agentId, item.reason]).sort(), [['a', 'sharedChat'], ['b', 'runtime']]);
});

test('bindings point at the hub’s node and enabled schedules continue from now', () => {
  assert.deepEqual(JSON.parse(rebaseRow({ kind: 'agents', id: 'a', data: agent('a').data }, { node: 'hub-node', now: 5 }).data).binding.hostId, 'hub-node');
  assert.equal(JSON.parse(rebaseRow({ kind: 'schedules', id: 's', data: JSON.stringify({ state: 'enabled', checkedThrough: 1 }) }, { node: 'n', now: 5 }).data).checkedThrough, 5);
  assert.equal(JSON.parse(rebaseRow({ kind: 'schedules', id: 's', data: JSON.stringify({ state: 'paused', checkedThrough: 1 }) }, { node: 'n', now: 5 }).data).checkedThrough, 1);
  assert.throws(() => parseHandoffPayload({ version: 1, sourceInstallationId: 'mac', agentIds: ['a'], rows: [] }), /does not carry/);
  assert.throws(() => parseHandoffPayload({ version: 1, sourceInstallationId: 'mac', agentIds: [], rows: [{ kind: 'messages', id: 'm', data: '{}' }] }), /handoff/);
});
