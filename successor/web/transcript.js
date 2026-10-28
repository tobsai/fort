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

// ---------- The Quest log home: what needs you, beside every chat ----------

/** Pips filled out of three as a run moves, the same steps as FortKit's RunMeter, with the run's status tone. */
export function runMeter(run) {
  const filled = { queued: 1, running: 2, cancelRequested: 2, waitingForApproval: 2, outcomeUnknown: 2, completed: 3, failed: 3, canceled: 3, closedUnknown: 3 }[run.state] ?? 0;
  return { filled, tone: runStatus(run).tone };
}

/** The Agent a run belongs to: the one the request was sent to, or the chat's only Agent. */
function runAgent(run, snapshot, conversation, agentsById) {
  const asked = snapshot.messages.find(message => message.requestId === run.requestId && (message.recipientAgentId || message.agentId));
  return agentsById.get(asked?.recipientAgentId ?? asked?.agentId) ?? (conversation.participantAgentIds.length === 1 ? agentsById.get(conversation.participantAgentIds[0]) : undefined);
}

/**
 * Every quest across your chats, most urgent first: approvals you can decide now, then a chat whose
 * latest turn failed, then runs waiting on an approval only the chat can show. A failed turn stops
 * being a quest as soon as a later turn in the same chat starts.
 */
export function questLog(conversations, snapshots, agents, now = Date.now()) {
  const agentsById = new Map(agents.map(agent => [agent.agentId, agent]));
  const decide = [], failed = [], waiting = [];
  for (const conversation of conversations) {
    const snapshot = snapshots.get(conversation.conversationId);
    if (!snapshot) continue;
    const base = run => ({ conversationId: conversation.conversationId, chat: conversation.name, agent: run ? runAgent(run, snapshot, conversation, agentsById) ?? null : null });
    const decidable = new Set();
    for (const approval of snapshot.approvals ?? []) {
      if (!canDecide(approval, now)) continue;
      const run = snapshot.runs.find(item => item.runId === approval.runId);
      decidable.add(approval.runId);
      decide.push({ ...base(run), id: 'approval:' + approval.approvalId, kind: 'approval', title: approval.actionDescription, detail: run?.statusDetail && run.statusDetail !== approval.actionDescription ? run.statusDetail : null, approval });
    }
    for (const run of snapshot.runs) {
      if (run.state === 'waitingForApproval' && !decidable.has(run.runId))
        waiting.push({ ...base(run), id: 'run:' + run.runId, kind: 'waiting', title: run.statusDetail ?? runStatus(run).text, detail: 'Open the chat to answer.', run });
    }
    const latest = snapshot.runs.at(-1);
    if (latest && runStatus(latest).tone === 'failed')
      failed.push({ ...base(latest), id: 'run:' + latest.runId, kind: 'failed', title: runStatus(latest).text, detail: latest.statusDetail ?? null, run: latest });
  }
  return [...decide, ...failed, ...waiting];
}

/** One panel per chat: its crew, the runs still going and how many quests it holds. */
export function chatPanels(conversations, snapshots, agents, quests) {
  const agentsById = new Map(agents.map(agent => [agent.agentId, agent]));
  return conversations.map(conversation => {
    const snapshot = snapshots.get(conversation.conversationId);
    const mine = quests.filter(quest => quest.conversationId === conversation.conversationId);
    const live = (snapshot?.runs ?? []).filter(run => ['working', 'needsYou'].includes(runStatus(run).tone)).map(run => {
      const agent = runAgent(run, snapshot, conversation, agentsById);
      return { runId: run.runId, agentName: agent?.name ?? null, text: runStatus(run).text, ...runMeter(run) };
    });
    const tone = mine.some(quest => quest.kind === 'failed') ? 'failed' : mine.length ? 'needsYou' : live.length ? 'working' : 'quiet';
    return {
      conversationId: conversation.conversationId, name: conversation.name, kind: conversation.kind, tone, quests: mine.length, live,
      crew: conversation.participantAgentIds.map(id => agentsById.get(id)).filter(Boolean).map(agent => ({ agentId: agent.agentId, name: agent.name, initial: [...agent.name.trim()][0]?.toUpperCase() ?? '?', slot: crewSlot(agent.agentId) }))
    };
  });
}

/** The one line under the header: how many quests, and how many runs are going. */
export function deskSummary(quests, panels) {
  const running = panels.reduce((sum, panel) => sum + panel.live.filter(run => run.tone === 'working').length, 0);
  const parts = [quests.length ? `${quests.length} ${quests.length === 1 ? 'quest' : 'quests'}` : 'No quests'];
  if (running) parts.push(`${running} running`);
  return parts.join(' · ');
}
