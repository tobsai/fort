export type Installation = { contractVersion: '1.0'; installationId: string; recoveryEpoch: string; mode: 'fixture' | 'openai'; serviceName: 'Fort Fixture' | 'Fort Prototype' };
export type Binding = { bindingRevisionId: string; provider: 'fixture' | 'codex'; runtimeVersion: string; selectedModel: string; hostId: string; enabled: boolean };
export type Agent = { agentId: string; name: string; homeConversationId: string; binding: Binding; readiness: 'fixtureReady' | 'requiresLiveReview'; instructions?: string };
export type Message = { messageId: string; conversationId: string; requestId: string; role: 'user' | 'assistant'; text: string; sequence: number; createdAt: string; agentId?: string; agentName?: string; recipientAgentId?: string; recipientAgentName?: string };
export type Run = { runId: string; requestId: string; conversationId: string; bindingRevisionId: string; state: 'queued' | 'running' | 'waitingForApproval' | 'cancelRequested' | 'canceled' | 'closedUnknown' | 'completed' | 'failed' | 'outcomeUnknown'; selectedModel: string; effectiveModel: string | null; modelEvidence: 'fixture' | 'unreported'; attemptId: string | null; configuration?: TurnConfiguration; statusDetail?: string };
export type Snapshot = { installationId: string; recoveryEpoch: string; conversationId: string; cursor: number; messages: Message[]; runs: Run[]; approvals: Approval[]; lastReadMessageSequence: number };
export type Submission = { requestId: string; bindingRevisionId: string; recoveryEpoch: string; text: string };
export type Receipt = { requestId: string; conversationId: string; messageId: string; runId: string; bindingRevisionId: string; inputDigest: string; acceptedAt: string };
export type Acceptance = { receipt: Receipt; snapshot: Snapshot };
export type DurableEvent = { schemaVersion: 1; installationId: string; recoveryEpoch: string; eventId: string; sequence: number; entityId: string; conversationId: string; kind: string; createdAt: string; data: unknown };
export const MAX_TEXT_BYTES = 65_536;

export type Approval = { approvalId: string; installationId: string; recoveryEpoch: string; runId: string; attemptId: string; bindingRevisionId: string; actionDigest: string; resourceScope: ['fixture:reply']; actionDescription: 'Produce the fixture reply'; expiresAt: string; state: 'pending' | 'approved' | 'rejected' | 'expired' | 'canceled'; decidedByDeviceId: string | null; decidedAt: string | null };

export type SkillReference = { skillId: string; revisionId: string };
export type SkillBundleFile = { path: string; kind: 'file'; executable: boolean; contentBase64: string; bytes: number; sha256: string };
export type SkillBundle = { schemaVersion: 1; name: string; version: string; source: string; license: string; entrypoint: string; supportedRuntimes: string[]; requestedTools: string[]; scripts: { path: string; interpreter: string; resourceScope: string[] }[]; files: SkillBundleFile[]; digest: string };
export type SkillReview = { state: 'reviewRequired' | 'enabled' | 'disabled'; digest: string };
export type InstructionSkill = SkillReference & { name: string; instructions: string } & ({ bundle?: never; review?: never } | { bundle: SkillBundle; review: SkillReview; previousRevisionId?: string });
export type Conversation = { conversationId: string; name: string; kind: 'home' | 'shared'; participantAgentIds: string[] };
export type TurnConfiguration = { agentId: string; agentName: string; bindingRevisionId: string; selectedModel: string; instructions: string; skills: InstructionSkill[] };
