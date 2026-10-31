// Hands a Mac's Agents to the cloud hub it is enrolled with (Spec 063, revision 2026-10-31). Once a Mac syncs, the hub
// owns every Agent it can run: the hub defines, schedules and coordinates them, and Macs only run turns and show them.
// A handoff moves definitions, never history: the Agent, its binding revisions, its skills, its routines and schedules,
// its portrait, and the empty shells of the chats it needs (its home chat and its routines' chats) with their memory.
// Past messages and runs stay on the Mac, read-only, until the owner uploads history separately.
// Selection is pure so the Mac and the tests agree on exactly what moves.

import { HANDOFF_KINDS, HANDOFF_VERSION, type HandoffKind, type HandoffPayload, type HandoffRow, type HandoffRuntime, type HandoffSelection, type Staying } from '../runner/handoff-contract.ts';
export { HANDOFF_KINDS, HANDOFF_VERSION };
export type { HandoffKind, HandoffPayload, HandoffReport, HandoffRow, HandoffRuntime, HandoffSelection, Staying } from '../runner/handoff-contract.ts';

type Raw = { kind: string; id: string; data: string };
type Json = Record<string, unknown>;
const parse = (row: Raw): Json => JSON.parse(row.data) as Json;
const str = (value: unknown): string | undefined => typeof value === 'string' ? value : undefined;

// Picks what moves. An Agent moves when the hub serves its exact provider and version, and every chat it needs
// (its home chat and its routines' chats) has only moving Agents in it. Everything else stays and keeps running here.
export function selectHandoff(rows: readonly Raw[], options: { sourceInstallationId: string; runtimes: readonly HandoffRuntime[] }): HandoffSelection {
  const by = (kind: string) => rows.filter(row => row.kind === kind);
  const agents = by('agents').map(row => ({ row, value: parse(row) }));
  const conversations = new Map(by('conversations').map(row => [row.id, { row, value: parse(row) }]));
  const routines = by('routines').map(row => ({ row, value: parse(row) }));
  const routineAgent = (value: Json) => str((value.agentSnapshot as Json | undefined)?.agentId);
  const routineChat = (value: Json) => str((value.definition as Json | undefined)?.conversationId);
  const serves = (binding: Json) => options.runtimes.some(runtime => runtime.provider === binding.provider && runtime.runtimeVersion === binding.runtimeVersion);
  const staying = new Map<string, Staying>();
  const moving = new Set<string>();
  for (const { value } of agents) {
    const agentId = str(value.agentId)!;
    if (serves((value.binding ?? {}) as Json)) moving.add(agentId);
    else staying.set(agentId, { agentId, name: str(value.name) ?? '', reason: 'runtime' });
  }
  // A chat moves only with all of its Agents, so drop Agents whose chats can't move until nothing changes.
  const chatsOf = (agentId: string, home: string | undefined) => [home, ...routines.filter(routine => routineAgent(routine.value) === agentId).map(routine => routineChat(routine.value))];
  for (let changed = true; changed;) {
    changed = false;
    for (const { value } of agents) {
      const agentId = str(value.agentId)!;
      if (!moving.has(agentId)) continue;
      const blocked = chatsOf(agentId, str(value.homeConversationId)).some(chatId => {
        const chat = chatId === undefined ? undefined : conversations.get(chatId);
        const participants = Array.isArray(chat?.value.participantAgentIds) ? chat.value.participantAgentIds as unknown[] : [];
        return !chat || participants.some(participant => typeof participant !== 'string' || !moving.has(participant));
      });
      if (blocked) { moving.delete(agentId); staying.set(agentId, { agentId, name: str(value.name) ?? '', reason: 'sharedChat' }); changed = true; }
    }
  }
  const movingRoutines = new Set(routines.filter(routine => moving.has(routineAgent(routine.value) ?? '')).map(routine => routine.row.id));
  const chats = new Set<string>();
  for (const { value } of agents) if (moving.has(str(value.agentId)!)) chats.add(str(value.homeConversationId)!);
  for (const routine of routines) if (movingRoutines.has(routine.row.id)) chats.add(routineChat(routine.value)!);
  const picked: HandoffRow[] = [];
  const take = (kind: HandoffKind, keep: (row: Raw, value: Json) => boolean) => { for (const row of by(kind)) if (keep(row, parse(row))) picked.push({ kind, id: row.id, data: row.data }); };
  take('agents', (_row, value) => moving.has(str(value.agentId)!));
  take('agentBindings', (_row, value) => moving.has(str(value.agentId) ?? ''));
  take('disabledBindings', (_row, value) => moving.has(str(value.agentId) ?? ''));
  // Skills are a shared library and small; routines pin exact revisions, so the whole library moves.
  take('skills', () => true);
  take('skillRevisions', () => true);
  take('routines', row => movingRoutines.has(row.id));
  take('routineRevisions', (_row, value) => moving.has(routineAgent(value) ?? ''));
  take('schedules', (_row, value) => movingRoutines.has(str((value.definition as Json | undefined)?.routineId) ?? ''));
  take('scheduleRevisions', (_row, value) => movingRoutines.has(str((value.definition as Json | undefined)?.routineId) ?? ''));
  take('conversations', row => chats.has(row.id));
  take('conversationMemory', row => chats.has(row.id));
  take('portraits', row => moving.has(row.id));
  return { payload: { version: HANDOFF_VERSION, sourceInstallationId: options.sourceInstallationId, agentIds: [...moving].sort(), rows: picked }, staying: [...staying.values()] };
}

const IDENTIFIER = /^[A-Za-z0-9_-]{1,128}$/;
// Checks a payload's shape before the hub decodes each row with its own ledger decoders.
export function parseHandoffPayload(value: unknown): HandoffPayload {
  const input = value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Json : {};
  if (Object.keys(input).some(key => !['version', 'sourceInstallationId', 'agentIds', 'rows'].includes(key)) || input.version !== HANDOFF_VERSION
    || typeof input.sourceInstallationId !== 'string' || !IDENTIFIER.test(input.sourceInstallationId)
    || !Array.isArray(input.agentIds) || input.agentIds.length > 1000 || input.agentIds.some(agentId => typeof agentId !== 'string' || !IDENTIFIER.test(agentId))
    || !Array.isArray(input.rows) || input.rows.length > 20_000) throw new Error('Send a Fort agent handoff.');
  const rows = input.rows.map((row: unknown): HandoffRow => {
    const value = row !== null && typeof row === 'object' && !Array.isArray(row) ? row as Json : {};
    if (Object.keys(value).some(key => !['kind', 'id', 'data'].includes(key)) || !(HANDOFF_KINDS as readonly unknown[]).includes(value.kind)
      || typeof value.id !== 'string' || !IDENTIFIER.test(value.id) || typeof value.data !== 'string') throw new Error('Send a Fort agent handoff.');
    return { kind: value.kind as HandoffKind, id: value.id, data: value.data };
  });
  const listed = new Set(rows.filter(row => row.kind === 'agents').map(row => row.id));
  if (input.agentIds.length !== listed.size || input.agentIds.some(agentId => !listed.has(agentId as string))) throw new Error('The handoff lists Agents it does not carry.');
  return { version: HANDOFF_VERSION, sourceInstallationId: input.sourceInstallationId, agentIds: input.agentIds as string[], rows };
}

// Rewrites a row for the hub: bindings point at the hub's node, and enabled schedules continue from now, so an occurrence
// the Mac already ran (or skipped) never runs again on the hub.
export function rebaseRow(row: HandoffRow, options: { node: string; now: number }): { data: string; rebound: boolean } {
  if (row.kind === 'agents' || row.kind === 'agentBindings') {
    const value = JSON.parse(row.data) as { binding?: { hostId?: unknown } };
    if (!value.binding || typeof value.binding.hostId !== 'string') throw new Error('An Agent in the handoff has no binding.');
    const rebound = value.binding.hostId !== options.node;
    value.binding.hostId = options.node;
    return { data: JSON.stringify(value), rebound: rebound && row.kind === 'agents' };
  }
  if (row.kind === 'schedules' || row.kind === 'scheduleRevisions') {
    const value = JSON.parse(row.data) as { state?: unknown; checkedThrough?: unknown };
    if (value.state === 'enabled' && typeof value.checkedThrough === 'number' && value.checkedThrough < options.now) value.checkedThrough = options.now;
    return { data: JSON.stringify(value), rebound: false };
  }
  return { data: row.data, rebound: false };
}
