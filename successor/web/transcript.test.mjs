import { test } from 'node:test';
import assert from 'node:assert/strict';
import { approvalCopy, canDecide, chatPanels, deskSummary, questLog, resolution, runMeter, runStatus, transcriptRows } from './transcript.js';

// The web chat shows runs and approvals the way the Mac and iPhone apps do (FortKit TranscriptRows).
const hour = 3_600_000, now = Date.parse('2026-10-28T12:00:00Z');
const message = (messageId, requestId, role) => ({ messageId, requestId, role, conversationId: 'c', text: role, sequence: 1, createdAt: '' });
const run = (runId, requestId, state, extra = {}) => ({ runId, requestId, state, conversationId: 'c', bindingRevisionId: 'b', selectedModel: 'm', effectiveModel: null, modelEvidence: 'unreported', attemptId: 'a', ...extra });
const gate = (approvalId, runId, extra = {}) => ({ approvalId, runId, installationId: 'i', recoveryEpoch: 'e', attemptId: 'a', bindingRevisionId: 'b', actionDigest: 'd',
  expiresAt: new Date(now + hour).toISOString(), state: 'pending', decidedByDeviceId: null, decidedAt: null, kind: 'routineGate', routineRevisionId: 'r', stepId: 's',
  resourceScope: ['routine:continue'], actionDescription: 'Publish the post', ...extra });

test('a run waiting for approval reads as needing you, not as a failure', () => {
  assert.deepEqual(runStatus(run('r1', 'q1', 'waitingForApproval', { statusDetail: 'Publish the post?' })), { tone: 'needsYou', text: 'Waiting for your approval', detail: 'Publish the post?' });
  assert.equal(runStatus(run('r1', 'q1', 'queued')).tone, 'working');
  assert.equal(runStatus(run('r1', 'q1', 'failed')).tone, 'failed');
  assert.equal(runStatus(run('r1', 'q1', 'canceled')).tone, 'off');
  assert.equal(runStatus(run('r1', 'q1', 'somethingNew')).text, 'Unsupported state · refresh or update Fort');
});

test('each request shows its run and approval after its last message; finished replies get no run line', () => {
  const snapshot = {
    messages: [message('m1', 'q1', 'user'), message('m2', 'q1', 'assistant'), message('m3', 'q2', 'user')],
    runs: [run('r1', 'q1', 'completed'), run('r2', 'q2', 'waitingForApproval'), run('r3', 'q3', 'queued')],
    approvals: [gate('g2', 'r2'), gate('g9', 'gone')],
  };
  assert.deepEqual(transcriptRows(snapshot).map(row => row.kind + ':' + (row.message?.messageId ?? row.run?.runId ?? row.approval.approvalId)),
    ['message:m1', 'message:m2', 'message:m3', 'run:r2', 'approval:g2', 'run:r3', 'approval:g9']);
});

test('only a recognised, pending, unexpired approval takes a decision', () => {
  assert.equal(canDecide(gate('g', 'r'), now), true);
  assert.equal(canDecide(gate('g', 'r', { state: 'approved' }), now), false);
  assert.equal(canDecide(gate('g', 'r', { expiresAt: new Date(now - 1).toISOString() }), now), false);
  assert.equal(canDecide(gate('g', 'r', { resourceScope: ['shell:run'] }), now), false);
  assert.equal(canDecide(gate('g', 'r', { stepId: '' }), now), false);
});

test('approval buttons and records use the same words as the apps', () => {
  assert.deepEqual(approvalCopy(gate('g', 'r')), { approve: 'Continue routine', reject: 'Reject', decided: null });
  assert.equal(approvalCopy(gate('g', 'r', { state: 'rejected' })).decided, 'Rejected');
  assert.equal(approvalCopy(gate('g', 'r', { state: 'expired' })).decided, 'Expired before a decision');
});

test('a decision sends back exactly what the service checks', () => {
  assert.deepEqual(resolution(gate('g', 'r'), false), { recoveryEpoch: 'e', runId: 'r', attemptId: 'a', bindingRevisionId: 'b', actionDigest: 'd', resourceScope: ['routine:continue'], decision: 'reject' });
  assert.equal(resolution(gate('g', 'r'), true).decision, 'approve');
});

// The Quest log home gathers what needs you from every chat, and shows each chat as a panel of its crew and live runs.
const agents = [{ agentId: 'lewis', name: 'Lewis' }, { agentId: 'marty', name: 'Marty' }];
const conversations = [
  { conversationId: 'home-lewis', name: 'Lewis', kind: 'home', participantAgentIds: ['lewis'] },
  { conversationId: 'room', name: 'Fort build', kind: 'shared', participantAgentIds: ['lewis', 'marty'] },
  { conversationId: 'home-marty', name: 'Marty', kind: 'home', participantAgentIds: ['marty'] },
];
const ask = (messageId, requestId, recipientAgentId) => ({ ...message(messageId, requestId, 'user'), recipientAgentId });
const snapshots = new Map([
  ['home-lewis', { messages: [message('l1', 'q1', 'user')], runs: [run('lr1', 'q1', 'completed'), run('lr2', 'q2', 'failed', { statusDetail: 'The Mac mini did not answer.' })], approvals: [] }],
  ['room', { messages: [ask('f1', 'q3', 'marty'), ask('f2', 'q4', 'lewis')], runs: [run('fr1', 'q3', 'waitingForApproval', { statusDetail: 'Publish the post?' }), run('fr2', 'q4', 'running')], approvals: [gate('g1', 'fr1')] }],
  ['home-marty', { messages: [], runs: [run('mr1', 'q5', 'failed'), run('mr2', 'q6', 'queued')], approvals: [] }],
]);

test('the quest log puts decisions first, then chats whose latest turn failed', () => {
  const quests = questLog(conversations, snapshots, agents, now);
  assert.deepEqual(quests.map(quest => [quest.kind, quest.conversationId, quest.agent?.name ?? null, quest.title]), [
    ['approval', 'room', 'Marty', 'Publish the post'],
    ['failed', 'home-lewis', 'Lewis', 'Could not complete this turn'],
  ]);
  assert.equal(quests[0].detail, 'Publish the post?');
  assert.equal(quests[1].detail, 'The Mac mini did not answer.');
});

test('a run waiting on an approval the web cannot decide still shows, pointing at its chat', () => {
  const expired = new Map(snapshots).set('room', { ...snapshots.get('room'), approvals: [gate('g1', 'fr1', { expiresAt: new Date(now - 1).toISOString() })] });
  const quests = questLog(conversations, expired, agents, now);
  assert.deepEqual(quests.map(quest => quest.kind), ['failed', 'waiting']);
  assert.equal(quests[1].title, 'Publish the post?');
});

test('each chat is a panel with its crew, its live runs as meters and its state', () => {
  const quests = questLog(conversations, snapshots, agents, now);
  const panels = chatPanels(conversations, snapshots, agents, quests);
  assert.deepEqual(panels.map(panel => [panel.name, panel.tone, panel.quests, panel.crew.map(member => member.initial).join('')]),
    [['Lewis', 'failed', 1, 'L'], ['Fort build', 'needsYou', 1, 'LM'], ['Marty', 'working', 0, 'M']]);
  assert.deepEqual(panels[1].live.map(item => [item.agentName, item.filled, item.tone]), [['Marty', 2, 'needsYou'], ['Lewis', 2, 'working']]);
  assert.deepEqual(panels[2].live.map(item => [item.agentName, item.filled]), [['Marty', 1]]);
  assert.equal(deskSummary(quests, panels), '2 quests · 2 running');
  assert.equal(deskSummary([], []), 'No quests');
});

test('run meters fill in the same three steps as the apps', () => {
  assert.deepEqual(['queued', 'running', 'waitingForApproval', 'completed', 'failed', 'canceled'].map(state => runMeter(run('r', 'q', state))),
    [{ filled: 1, tone: 'working' }, { filled: 2, tone: 'working' }, { filled: 2, tone: 'needsYou' }, { filled: 3, tone: 'done' }, { filled: 3, tone: 'failed' }, { filled: 3, tone: 'off' }]);
});
