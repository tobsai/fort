// The shapes a Mac and its cloud hub exchange when the Mac hands its Agents over (Spec 063, revision 2026-10-31).
// Kept in runner/ with no service imports, so the runner side (and the packaged executor) never pulls in the core.
export const HANDOFF_VERSION = 1;
export const HANDOFF_KINDS = ['agents', 'agentBindings', 'disabledBindings', 'skills', 'skillRevisions', 'routines', 'routineRevisions',
  'schedules', 'scheduleRevisions', 'conversations', 'conversationMemory', 'portraits'] as const;
export type HandoffKind = typeof HANDOFF_KINDS[number];
export type HandoffRow = { kind: HandoffKind; id: string; data: string };
export type HandoffRuntime = { provider: string; runtimeVersion: string };
export type HandoffPayload = { version: 1; sourceInstallationId: string; agentIds: string[]; rows: HandoffRow[] };
// What the hub holds after a handoff: every listed Agent is now the hub's, whether it was just added or already there.
// kept counts rows the hub already had, which it keeps as they are (the hub's copy wins).
export type HandoffReport = { version: 1; agentIds: string[]; added: Record<string, number>; kept: Record<string, number>; rebound: number };
export type Staying = { agentId: string; name: string; reason: 'runtime' | 'sharedChat' };
export type HandoffSelection = { payload: HandoffPayload; staying: Staying[] };

export type HubHandoff = { hubURL: string; agentIds: string[]; at: string };
// What a Mac's core lends its hub runner: export what can move, record what the hub confirmed, and read it back.
export type HubLedger = { exportHandoff(runtimes: readonly HandoffRuntime[]): HandoffSelection; recordHandoff(handoff: { hubURL: string; agentIds: readonly string[] }): HubHandoff; handoff(): HubHandoff | undefined };
