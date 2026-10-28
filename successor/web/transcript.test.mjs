import { test } from 'node:test';
import assert from 'node:assert/strict';
import { approvalCopy, canDecide, resolution, runStatus, transcriptRows } from './transcript.js';

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
