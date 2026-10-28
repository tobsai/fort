// What the web chat shows for a conversation snapshot: messages, the runs behind them and any approvals
// they wait on, in the same order and with the same words as the Mac and iPhone apps (FortKit's
// TranscriptRows and Run.displayState). Pure functions, so they are tested without a browser.

// The status grammar shared by every Fort surface: working, needs you, done, failed, off.
const RUNS = {
  queued: ['working', 'Accepted · waiting for host'],
  running: ['working', 'Agent is responding'],
  cancelRequested: ['working', 'Cancellation requested · waiting for confirmation'],
  waitingForApproval: ['needsYou', 'Waiting for your approval'],
  completed: ['done', 'Reply saved'],
  failed: ['failed', 'Could not complete this turn'],
  outcomeUnknown: ['failed', 'Outcome unknown · review required'],
  canceled: ['off', 'Canceled'],
  closedUnknown: ['off', 'Closed without retry · original outcome remains unknown'],
};

/** The run's tone and one plain sentence, plus the service's own detail when it gave one. */
export function runStatus(run) {
  const [tone, text] = RUNS[run.state] ?? ['failed', 'Unsupported state · refresh or update Fort'];
  return { tone, text, detail: run.statusDetail ?? null };
}

/** A finished reply speaks for itself, so only runs still going or stopped short get a line. */
export const showsRun = run => run.state !== 'completed';

/** Messages in order; after the last message of each request, its runs and their approvals. */
export function transcriptRows(snapshot) {
  const lastMessage = new Map(snapshot.messages.map(message => [message.requestId, message.messageId]));
  const approvalsByRun = new Map();
  for (const approval of snapshot.approvals ?? []) approvalsByRun.set(approval.runId, [...(approvalsByRun.get(approval.runId) ?? []), approval]);
  const rows = [];
  const addRun = run => {
    if (showsRun(run)) rows.push({ kind: 'run', run });
    for (const approval of approvalsByRun.get(run.runId) ?? []) rows.push({ kind: 'approval', approval });
  };
  for (const message of snapshot.messages) {
    rows.push({ kind: 'message', message });
    if (lastMessage.get(message.requestId) === message.messageId) for (const run of snapshot.runs) if (run.requestId === message.requestId) addRun(run);
  }
  for (const run of snapshot.runs) if (!lastMessage.has(run.requestId)) addRun(run);
  const runIds = new Set(snapshot.runs.map(run => run.runId));
  for (const approval of snapshot.approvals ?? []) if (!runIds.has(approval.runId)) rows.push({ kind: 'approval', approval });
  return rows;
}

/** Only an approval Fort recognises, still pending and not yet expired, takes a decision. */
export function canDecide(approval, now = Date.now()) {
  const scope = JSON.stringify(approval.resourceScope);
  const recognized = (approval.kind === undefined && scope === '["fixture:reply"]')
    || (approval.kind === 'routineGate' && scope === '["routine:continue"]' && Boolean(approval.routineRevisionId) && Boolean(approval.stepId));
  return approval.state === 'pending' && recognized && Date.parse(approval.expiresAt) > now;
}

/** The buttons' words, and what a decided approval says instead. */
export function approvalCopy(approval) {
  const routine = approval.kind === 'routineGate';
  const decided = { approved: routine ? 'Continued' : 'Approved', rejected: 'Rejected', expired: 'Expired before a decision', canceled: 'Canceled' }[approval.state] ?? null;
  return { approve: routine ? 'Continue routine' : 'Approve', reject: 'Reject', decided };
}

/** The body for POST /v1/approvals/{approvalId}/resolve. The service checks every field against the approval. */
export function resolution(approval, approve) {
  return { recoveryEpoch: approval.recoveryEpoch, runId: approval.runId, attemptId: approval.attemptId, bindingRevisionId: approval.bindingRevisionId,
    actionDigest: approval.actionDigest, resourceScope: approval.resourceScope, decision: approve ? 'approve' : 'reject' };
}

/** An Agent's crew colour slot (0-5, the --crew-N tokens): FNV-1a over its id, the same as FortTheme.crewSlot in the apps. */
export function crewSlot(agentId, count = 6) {
  let hash = 0x811c9dc5;
  for (const byte of new TextEncoder().encode(agentId)) hash = Math.imul(hash ^ byte, 0x01000193) >>> 0;
  return hash % count;
}
