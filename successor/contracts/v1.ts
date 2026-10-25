export type RuntimeReadiness = { state: 'ready' | 'busy' | 'restartRequired' | 'stopped' | 'unavailable'; canRestart: boolean; detail: string; connectionsUsed?: number; maximumConnections?: number };
export type AccountChallenge = { kind: 'deviceCode'; verificationURL: string; userCode: string } | { kind: 'browserAuthorization'; authorizationURL: string };
export type AccountIdentity = { accountId: string; userLabel: string };
export type AccountSignInResult = { kind: 'signedIn'; account?: AccountIdentity } | { kind: 'canceled' | 'failed' | 'unconfirmed' };
export type AccountSignInOptions = { durationMs: number; onChallenge: (challenge: AccountChallenge) => void };
export type AccountSignInRequest = { operationId: string; expectedRevision: number; recoveryEpoch: string; expiresAt: number };
export type AccountSignInCancellation = { operationId: string; revision: number; recoveryEpoch: string };
export type AccountSignInStatus = { revision: number } & (
  | { state: 'idle' | 'unavailable' }
  | ({ operationId: string; expiresAt: number } & (
    | { state: 'starting' | 'canceling' }
    | { state: 'challenge'; challenge: AccountChallenge }
    | { state: 'finished'; result: AccountSignInResult; selected?: boolean }
  ))
);
export type RuntimeCapability = 'plainText' | 'modelCatalog' | 'cancel' | 'skills' | 'memory';
export type RuntimeDescriptor = { provider: string; name: string; runtimeVersion: string; hostId: string; session: 'stateless' | 'sessionful'; capabilities: RuntimeCapability[] };
export type Installation = { contractVersion: '1.0'; installationId: string; recoveryEpoch: string; mode: 'fixture' | 'openai' | 'local'; serviceName: 'Fort Fixture' | 'Fort Prototype' | 'Fort'; capabilities?: string[]; runtime?: RuntimeDescriptor; runtimes?: RuntimeDescriptor[]; eventCursor?: number };
export type Binding = { bindingRevisionId: string; provider: string; runtimeVersion: string; selectedModel: string; hostId: string; enabled: boolean };
export type AgentAvatar = 'orbit' | 'ember' | 'sprout';
export type ModelOption = { id:string; name:string; availability:'available'|'unavailable'; detail?:string };
export type ModelCatalog = {provider:string;state:'ready';models:ModelOption[];checkedAt:string}|{provider:string;state:'unavailable'|'error';models:[];detail:string};
// Spec 065: a portrait Fort drew on the ChatGPT plan. The PNG is fetched separately by revision.
export type AgentPortrait = { revision: string; createdAt: string; size: number };
export type PortraitDraw = { state: 'drawing' | 'failed'; detail: string; updatedAt: string };
export type PortraitImage = { agentId: string; revision: string; png: string };
// Spec 066: an image Fort drew in a conversation. The PNG is fetched separately by imageId.
export type MessageImage = { imageId: string; width: number; height: number };
export type ConversationImage = { imageId: string; png: string };
export type Agent = { agentId: string; name: string; homeConversationId: string; binding: Binding; readiness: 'fixtureReady' | 'requiresLiveReview'; instructions?: string; avatar?: AgentAvatar; portrait?: AgentPortrait; portraitDraw?: PortraitDraw };
export type Message = { messageId: string; conversationId: string; requestId: string; role: 'user' | 'assistant'; text: string; sequence: number; createdAt: string; agentId?: string; agentName?: string; recipientAgentId?: string; recipientAgentName?: string; drawsImage?: true; image?: MessageImage };
export type Run = { runId: string; requestId: string; conversationId: string; bindingRevisionId: string; state: 'queued' | 'running' | 'waitingForApproval' | 'cancelRequested' | 'canceled' | 'closedUnknown' | 'completed' | 'failed' | 'outcomeUnknown'; selectedModel: string; effectiveModel: string | null; modelEvidence: 'fixture' | 'unreported'; attemptId: string | null; attemptGeneration?: number; configuration?: TurnConfiguration; routine?: RoutineExecution; statusDetail?: string };
export type ConversationMemory = { conversationId: string; revisionId: string; text: string };
export type Snapshot = { window?: { before: number | null; olderBefore: number | null }; memory?: ConversationMemory; installationId: string; recoveryEpoch: string; conversationId: string; cursor: number; messages: Message[]; runs: Run[]; approvals: Approval[]; lastReadMessageSequence: number };
export type Submission = { requestId: string; bindingRevisionId: string; recoveryEpoch: string; text: string };
export type Receipt = { requestId: string; conversationId: string; messageId: string; runId: string; bindingRevisionId: string; inputDigest: string; acceptedAt: string };
export type Acceptance = { receipt: Receipt; snapshot: Snapshot; message?: Message; run?: Run };
export type RequestCancellation = { requestId: string; recoveryEpoch: string; conversationId: string; bindingRevisionId: string } & ({ status: 'notAccepted' } | { status: 'accepted'; receipt: Receipt; run: Run; snapshot: Snapshot; message?: Message });
export type DurableEvent = { schemaVersion: 1; installationId: string; recoveryEpoch: string; eventId: string; sequence: number; entityId: string; conversationId: string; kind: string; createdAt: string; data: unknown };
export type InvalidationPage = { installationId:string; recoveryEpoch:string; semantics:'invalidation'; cursor:number; events:DurableEvent[] };
export const MAX_TEXT_BYTES = 65_536;

type ApprovalIdentity = { approvalId: string; installationId: string; recoveryEpoch: string; runId: string; attemptId: string; bindingRevisionId: string; actionDigest: string; expiresAt: string; state: 'pending' | 'approved' | 'rejected' | 'expired' | 'canceled'; decidedByDeviceId: string | null; decidedAt: string | null };
export type RoutineApproval = ApprovalIdentity & { kind: 'routineGate'; routineRevisionId: string; stepId: string; resourceScope: ['routine:continue']; actionDescription: string };
export type Approval = (ApprovalIdentity & { resourceScope: ['fixture:reply']; actionDescription: 'Produce the fixture reply'; kind?: never }) | RoutineApproval;

export type SkillReference = { skillId: string; revisionId: string };
export type SkillBundleFile = { path: string; kind: 'file'; executable: boolean; contentBase64: string; bytes: number; sha256: string };
export type SkillBundle = { schemaVersion: 1; name: string; version: string; source: string; license: string; entrypoint: string; supportedRuntimes: string[]; requestedTools: string[]; scripts: { path: string; interpreter: string; resourceScope: string[] }[]; files: SkillBundleFile[]; digest: string };
export type SkillReview = { state: 'reviewRequired' | 'enabled' | 'disabled'; digest: string };
export type InstructionSkill = SkillReference & { name: string; instructions: string } & ({ bundle?: never; review?: never } | { bundle: SkillBundle; review: SkillReview; previousRevisionId?: string });
export type Conversation = { conversationId: string; name: string; kind: 'home' | 'shared'; participantAgentIds: string[] };
export type TurnConfiguration = { account?: AccountIdentity; agentId: string; agentName: string; bindingRevisionId: string; selectedModel: string; instructions: string; skills: InstructionSkill[]; memory?: ConversationMemory; contextModule?: import('./context.ts').ContextModuleRef };

export type RoutineValue = string | number | boolean;
export type RoutineInput = { name: string; type: 'text' | 'number' | 'boolean'; required: boolean };
export type RoutineTask = { stepId: string; name: string; type: 'task'; instructions: string; inputNames: string[]; outputStepIds: string[] };
export type RoutineGate = { stepId: string; name: string; type: 'approval'; prompt: string };
export type RoutineStep = RoutineTask | RoutineGate;
export type RoutineDefinition = { schemaVersion: 1; name: string; agentId: string; conversationId: string; bindingRevisionId: string; instructions: string; inputs: RoutineInput[]; skills: (SkillReference & { digest: string })[]; steps: RoutineStep[] };
export type RoutineRevision = { routineId: string; revisionId: string; digest: string; definition: RoutineDefinition; agentSnapshot: Agent; skills: InstructionSkill[]; contextModule?: import('./context.ts').ContextModuleRef };
export type RoutineStepProgress = { stepId: string; stepRevisionId: string } & (
  | { state: 'pending' }
  | { state: 'running'; attemptId: string }
  | { state: 'completed'; attemptId: string; output: { messageId: string; text: string; digest: string } }
  | { state: 'waitingForApproval' | 'approved' | 'rejected'; attemptId: string; approvalId: string }
  | { state: 'failed' | 'outcomeUnknown' | 'canceled'; attemptId?: string; detail: string }
);
export type RoutineExecution = { routineId: string; revisionId: string; definition: RoutineDefinition; inputs: Record<string,RoutineValue>; steps: RoutineStepProgress[] };

export type ScheduleDefinition = { schemaVersion:1; routineId:string; routineRevisionId:string; timezone:string; time:string; inputs:Record<string,RoutineValue>; missed:'skip'|'catchUpOne' } & ({kind:'once';date:string;weekdays?:never}|{kind:'daily';date?:never;weekdays?:never}|{kind:'weekly';weekdays:number[];date?:never});
export type CalendarOccurrence = { localDate:string; localTime:string; intendedAt:string|null; decisionAt:string };
export type SchedulePreview = { revisionId:string; digest:string; expiresAt:string; timezone:string; occurrences:CalendarOccurrence[]; overlap:'skip'; missed:'skip'|'catchUpOne' };
export type ScheduleRevision = { scheduleId:string; revisionId:string; digest:string; definition:ScheduleDefinition; state:'paused'|'enabled'; checkedThrough:number; consent?:{deviceId:string;at:string;previewDigest:string}; preview?:SchedulePreview };
export type SchedulerStatus = { state:'idle'|'manual'|'stopped' } | { state:'waiting'; nextWakeAt:string } | { state:'fault'; retryAt:string; detail:string };
export type ScheduleOccurrence = { occurrenceId:string; scheduleId:string; scheduleRevisionId:string; routineId:string; routineRevisionId:string; intendedAt:string|null; localDate:string; localTime:string; state:'accepted'|'skippedOverlap'|'skippedMissed'|'skippedGap'|'blocked'|'skippedMissedRange'; runId?:string; detail?:string };

export type OwnerPairingPolicy = { expiresAfterDays: number | null };
export type PairingStatus = { deviceId: string; mode: 'temporary' | 'owner-local'; expiresAt: string | null; role?: 'companion' };
export type CompanionInvitation = { installationId: string; invitationId: string; expiresAt: string };
export type CompanionReceipt = { installationId: string; deviceId: string; expiresAt: string | null; role: 'companion' };
export type CompanionRevocation = { installationId: string; deviceId: string; revoked: true; scope: 'companion-only' };
export type CompanionOverview = { installationId: string; hostDeviceId: string; pairing:
  | { state: 'available' }
  | { state: 'invited'; invitationId: string; expiresAt: string }
  | { state: 'paired'; deviceId: string; deviceName: string; expiresAt: string | null; active: boolean }
};
export type HostStopReceipt = { runtimeStopped: true; revoked: boolean };
export type WeekZone = 'code' | 'money' | 'writing' | 'personal';
export type WeekSize = 'small' | 'moderate' | 'significant' | 'milestone';
export type WeekOwner = { kind: 'you' } | { kind: 'agent'; agentId: string };
export type WeekGoal = { goalId: string; text: string; zone: WeekZone; lead: WeekOwner; targetPoints: number };
export type WeekPlanItem = { itemId: string; goalId: string; task: string; owner: WeekOwner; size: WeekSize; unlocks?: string; doneBuildId: string | null };
export type WeekBuild = { buildId: string; goalId: string; itemId: string | null; task: string; size: WeekSize; by: WeekOwner; placedAt: string; evidence?: string };
export type Week = { weekId: string; revisionId: string; number: number; name: string; startsOn: string; endsOn: string; state: 'live' | 'closed'; mainGoalId: string | null; goals: WeekGoal[]; plan: WeekPlanItem[]; builds: WeekBuild[]; closedAt?: string };
export type WeekProgress = { points: number; goals: { goalId: string; points: number; targetPoints: number; met: boolean }[]; goalsMet: number; goalsCounted: number; grade: 'A' | 'B' | 'C' | 'D' | null };
export type WeekView = Week & { progress: WeekProgress };
