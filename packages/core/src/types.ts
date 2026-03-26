/**
 * Fort Core Types
 *
 * Foundational types used across all Fort modules.
 * Every conversation is a task. Every task is tracked.
 */

// ─── Task Graph Types ───────────────────────────────────────────────

export type TaskStatus =
  | 'created'
  | 'in_progress'
  | 'blocked'
  | 'needs_review'
  | 'completed'
  | 'failed';

export type TaskSource =
  | 'user_chat'
  | 'imessage'
  | 'scheduled_routine'
  | 'agent_delegation'
  | 'reflection'
  | 'self_coding'
  | 'background';

export interface Task {
  id: string;
  shortId: string;
  parentId: string | null;
  title: string;
  description: string;
  status: TaskStatus;
  source: TaskSource;
  assignedAgent: string | null;
  sourceAgentId: string | null;
  createdAt: Date;
  updatedAt: Date;
  completedAt: Date | null;
  result: string | null;
  assignedTo: 'agent' | 'user' | null;
  metadata: Record<string, unknown>;
  subtaskIds: string[];
  threadId: string | null;
}

export interface Thread {
  id: string;
  name: string;
  description: string;
  taskId: string;
  parentThreadId: string | null;
  projectTag: string | null;
  assignedAgent: string | null;
  status: 'active' | 'paused' | 'resolved';
  createdAt: Date;
  lastActiveAt: Date;
}

export interface ThreadMessage {
  id: string;
  threadId: string;
  role: 'user' | 'agent' | 'system';
  content: string;
  agentId: string | null;
  createdAt: Date;
}

export interface ThreadReference {
  id: string;
  fromThreadId: string;
  toThreadId: string;
  note: string;
  createdAt: Date;
}

// ─── Module Bus Types ───────────────────────────────────────────────

export interface FortEvent<T = unknown> {
  id: string;
  type: string;
  source: string;
  timestamp: Date;
  payload: T;
}

export type EventHandler<T = unknown> = (event: FortEvent<T>) => void | Promise<void>;

export interface EventSubscription {
  eventType: string;
  handler: EventHandler;
}

// ─── Agent Types ────────────────────────────────────────────────────

export type AgentType = 'specialist';
export type AgentStatus = 'running' | 'paused' | 'stopped' | 'error';

export interface AgentConfig {
  id: string;
  name: string;
  type: AgentType;
  description: string;
  capabilities: string[];
  memoryPartition?: string;
}

export interface SpecialistIdentity {
  id: string;
  name: string;
  description: string;
  capabilities: string[];
  memoryPartition: string;
  behaviors: string[];
  eventSubscriptions: string[];
  createdAt: string;
  createdBy: string;
  parentId?: string;
  status: 'active' | 'retired';
  soulPath?: string;
  isDefault?: boolean;
  emoji?: string;
  avatar?: string;
  defaultModelTier?: 'fast' | 'standard' | 'powerful';
}

export interface AgentInfo {
  config: AgentConfig;
  status: AgentStatus;
  currentTaskId: string | null;
  startedAt: Date;
  taskCount: number;
  errorCount: number;
}

// ─── Memory Types ───────────────────────────────────────────────────

export interface MemoryNode {
  id: string;
  type: 'person' | 'project' | 'preference' | 'fact' | 'decision' | 'behavior' | 'routine' | 'entity';
  label: string;
  properties: Record<string, unknown>;
  createdAt: Date;
  updatedAt: Date;
  source: string;
}

export interface MemoryEdge {
  id: string;
  sourceId: string;
  targetId: string;
  type: string;
  properties: Record<string, unknown>;
  createdAt: Date;
}

export interface MemoryQuery {
  text?: string;
  nodeType?: MemoryNode['type'];
  limit?: number;
}

export interface MemorySearchResult {
  nodes: MemoryNode[];
  edges: MemoryEdge[];
  score?: number;
}

// ─── Permission Types ───────────────────────────────────────────────

export type PermissionLevel = 'allow' | 'read_only' | 'deny';
export type ActionTier = 1 | 2 | 3 | 4;

export interface FolderPermission {
  path: string;
  commandLine: PermissionLevel;
  allowedCommands?: string[];
  deniedCommands?: string[];
}

export interface PermissionConfig {
  defaults: {
    commandLine: PermissionLevel;
  };
  folders: FolderPermission[];
  system: {
    commandLine: PermissionLevel;
    exceptions: string[];
  };
}

export interface ActionGate {
  tier: ActionTier;
  action: string;
  description: string;
  requiresApproval: boolean;
}

// ─── Tool Registry Types ────────────────────────────────────────────

export type { FortTool, ToolResult, ToolCallLog, ToolExecutionContext } from './tools/types.js';

export interface ToolDefinition {
  id: string;
  name: string;
  description: string;
  capabilities: string[];
  inputTypes: string[];
  outputTypes: string[];
  tags: string[];
  module: string;
  version: string;
  usageCount: number;
  lastUsedAt: Date | null;
}

// ─── Diagnostic Types ───────────────────────────────────────────────

export interface Capability {
  name: string;
  description: string;
}

export interface DiagnosticResult {
  module: string;
  status: 'healthy' | 'degraded' | 'unhealthy';
  checks: DiagnosticCheck[];
}

export interface DiagnosticCheck {
  name: string;
  passed: boolean;
  message: string;
  details?: Record<string, unknown>;
}

// ─── Plugin Types ───────────────────────────────────────────────────

export interface PluginCapability {
  name: string;
  description: string;
  inputSchema?: Record<string, unknown>;
  outputSchema?: Record<string, unknown>;
}

export interface PluginPermission {
  type: 'filesystem' | 'network' | 'command' | 'memory' | 'bus';
  scope: string;
  reason: string;
}

export interface PluginManifest {
  name: string;
  version: string;
  description: string;
  author?: string;
  capabilities: PluginCapability[];
  subscriptions: string[];
  emissions: string[];
  permissions: PluginPermission[];
  entryPoint: string;
}

export interface PluginSecurityReport {
  pluginName: string;
  timestamp: string;
  passed: boolean;
  checks: SecurityCheck[];
}

export interface SecurityCheck {
  name: string;
  passed: boolean;
  severity: 'info' | 'warning' | 'critical';
  message: string;
}

export interface LoadedPlugin {
  manifest: PluginManifest;
  status: 'loaded' | 'active' | 'disabled' | 'error' | 'blocked';
  loadedAt: string;
  securityReport?: PluginSecurityReport;
  error?: string;
}

export interface PluginContext {
  bus: import('./module-bus/index.js').ModuleBus;
  memory: { createNode: Function; search: Function };
  tools: { search: Function; register: Function };
  log: (message: string) => void;
}

export interface FortPlugin {
  name: string;
  version: string;
  description: string;
  initialize(context: PluginContext): Promise<void>;
  shutdown(): Promise<void>;
  capabilities: PluginCapability[];
  diagnose(): DiagnosticResult;
}

// ─── Token Tracking Types ───────────────────────────────────────────

export interface TokenUsage {
  id: string;
  timestamp: Date;
  model: string;
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
  costUsd: number;
  taskId?: string;
  agentId?: string;
  source: string;
}

export interface TokenBudget {
  dailyLimit: number;
  monthlyLimit: number;
  perTaskLimit: number;
  warningThreshold: number; // 0-1, e.g. 0.8 = warn at 80%
}

export interface TokenStats {
  today: { tokens: number; cost: number; calls: number };
  thisMonth: { tokens: number; cost: number; calls: number };
  allTime: { tokens: number; cost: number; calls: number };
  byModel: Record<string, { tokens: number; cost: number; calls: number }>;
  byAgent: Record<string, { tokens: number; cost: number; calls: number }>;
}

// ─── Behavior Types ─────────────────────────────────────────────────

export interface Behavior {
  id: string;
  rule: string;
  context: string;
  priority: number;
  enabled: boolean;
  createdAt: string;
  source: string;
  examples?: string[];
}

// ─── Routine Types ──────────────────────────────────────────────────

export interface RoutineStep {
  id: string;
  action: string;
  params: Record<string, unknown>;
  onError?: 'abort' | 'skip' | 'retry';
}

export interface Routine {
  id: string;
  name: string;
  description: string;
  schedule: string;
  flowId?: string;
  steps: RoutineStep[];
  enabled: boolean;
  lastRun?: string;
  lastResult?: 'success' | 'failure' | 'skipped';
  createdAt: string;
  source: string;
  behaviors?: string[];
}

export interface RoutineExecution {
  id: string;
  routineId: string;
  startedAt: string;
  completedAt?: string;
  result: 'success' | 'failure' | 'skipped';
  error?: string;
  taskId: string;
}

// ─── Spec Types ─────────────────────────────────────────────────────

// ─── Flow Types ─────────────────────────────────────────────────────

export type FlowStepType = 'action' | 'condition' | 'transform' | 'llm' | 'parallel' | 'notify';
export type FlowErrorPolicy = 'abort' | 'skip' | 'retry';
export type FlowExecutionStatus = 'running' | 'completed' | 'failed' | 'aborted';

export interface FlowTrigger {
  type: 'cron' | 'event' | 'manual';
  value?: string; // cron expression or event type
}

export interface FlowStepBase {
  id: string;
  name: string;
  type: FlowStepType;
}

export interface ActionStep extends FlowStepBase {
  type: 'action';
  tool: string;
  params?: Record<string, unknown>;
}

export interface ConditionStep extends FlowStepBase {
  type: 'condition';
  expression: string; // JavaScript expression evaluated against context
  thenSteps: FlowStep[];
  elseSteps?: FlowStep[];
}

export interface TransformStep extends FlowStepBase {
  type: 'transform';
  expression: string; // JavaScript expression for data transformation
}

export interface LlmStep extends FlowStepBase {
  type: 'llm';
  prompt: string;
  model?: string;
}

export interface ParallelStep extends FlowStepBase {
  type: 'parallel';
  branches: FlowStep[][];
}

export interface NotifyStep extends FlowStepBase {
  type: 'notify';
  eventType: string;
  payload?: Record<string, unknown>;
}

export type FlowStep = ActionStep | ConditionStep | TransformStep | LlmStep | ParallelStep | NotifyStep;

export interface FlowDefinition {
  id: string;
  name: string;
  description: string;
  steps: FlowStep[];
  triggers?: FlowTrigger[];
  onError: FlowErrorPolicy;
}

export interface FlowStepResult {
  stepId: string;
  stepName: string;
  status: 'completed' | 'failed' | 'skipped';
  output: unknown;
  error?: string;
  startedAt: Date;
  completedAt: Date;
}

export interface FlowExecution {
  id: string;
  flowId: string;
  status: FlowExecutionStatus;
  startedAt: Date;
  completedAt: Date | null;
  stepResults: FlowStepResult[];
  context: Record<string, unknown>;
  error?: string;
  taskId: string;
}

// ─── Spec Types ─────────────────────────────────────────────────────

// ─── Feature Flag Types ─────────────────────────────────────────────

export interface FeatureFlag {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
  status: 'baking' | 'stable' | 'rolled_back' | 'disabled';
  createdAt: string;
  enabledAt?: string;
  bakePeriodMs: number;       // how long to bake before promoting to stable (default 24h)
  bakeStartedAt?: string;
  rollbackReason?: string;
  specId?: string;            // link to the spec that created this flag
  metadata: Record<string, unknown>;
}

export interface FlagCheck {
  flagId: string;
  timestamp: string;
  healthy: boolean;
  message: string;
}

export type SpecStatus = 'draft' | 'approved' | 'implementing' | 'verifying' | 'merged' | 'rolled_back';

export interface Spec {
  id: string;
  title: string;
  status: SpecStatus;
  goal: string;
  approach: string;
  affectedFiles: string[];
  testCriteria: string[];
  rollbackPlan: string;
  createdAt: Date;
  updatedAt: Date;
  author: string;
}

// ─── Harness Types ──────────────────────────────────────────────────

export interface HarnessConfig {
  repoRoot: string;
  autoApprove?: boolean;
  testCommand?: string;
  buildCommand?: string;
  lintCommand?: string;
}

export type HarnessCycleStatus =
  | 'spec_draft'
  | 'spec_review'
  | 'implementing'
  | 'testing'
  | 'verifying'
  | 'merging'
  | 'complete'
  | 'rolled_back'
  | 'failed';

export interface HarnessStep {
  phase: string;
  status: 'pending' | 'running' | 'passed' | 'failed' | 'skipped';
  startedAt?: string;
  completedAt?: string;
  output?: string;
  error?: string;
}

export interface HarnessCycle {
  id: string;
  specId: string;
  status: HarnessCycleStatus;
  branch: string;
  startedAt: string;
  completedAt?: string;
  steps: HarnessStep[];
  error?: string;
}

// ─── Rewind / Snapshot Types ────────────────────────────────────

export interface Snapshot {
  id: string;
  label?: string;
  trigger: string;
  createdAt: string;
  fileCount: number;
  totalBytes: number;
  filesHash: string;
  metadata: Record<string, unknown>;
}

export interface SnapshotStats {
  totalSnapshots: number;
  totalBytes: number;
  oldestSnapshot: string | null;
  newestSnapshot: string | null;
}

export interface RewindPreviewChange {
  file: string;
  type: 'added' | 'modified' | 'removed';
  details: string;
}

export interface RewindPreview {
  snapshotId: string;
  snapshot: Snapshot;
  changes: RewindPreviewChange[];
  hasChanges: boolean;
}

// ─── Introspection Types ────────────────────────────────────────────

export interface ModuleManifest {
  name: string;
  modules: ModuleManifestEntry[];
  generatedAt: string;
}

export interface ModuleManifestEntry {
  name: string;
  capabilities: string[];
  eventSubscriptions: string[];
  eventEmissions: string[];
  healthStatus: 'healthy' | 'degraded' | 'unhealthy' | 'unknown';
}

export interface CapabilityEntry {
  capability: string;
  provider: string;
  providerType: 'tool' | 'agent' | 'integration' | 'module';
  description: string;
}

export interface EventCatalogEntry {
  eventType: string;
  publishers: string[];
  subscriberCount: number;
}

export interface SystemProfile {
  version: string;
  uptime: number;
  moduleCount: number;
  agentCount: number;
  toolCount: number;
  integrationCount: number;
  taskCount: number;
  diagnosticSummary: {
    overall: 'healthy' | 'degraded' | 'unhealthy';
    totalChecks: number;
    passed: number;
    failed: number;
  } | null;
  generatedAt: string;
}

// ─── Garbage Collector Types ────────────────────────────────────────

// ─── OS Integration Types ───────────────────────────────────────────

export interface SpotlightResult {
  type: 'task' | 'thread' | 'memory';
  id: string;
  title: string;
  subtitle: string;
  relevance: number;
}

export interface ShortcutActionResult {
  success: boolean;
  intent?: string;
  target?: string;
  data?: unknown;
  error?: string;
}

export interface NotificationPolicy {
  shouldSend: boolean;
  reason: string;
  category: string;
  focusMode: string | null;
}

// ─── Garbage Collector Types ────────────────────────────────────────

export interface GCReport {
  staleSpecs: { id: string; title: string; status: string; ageDays: number }[];
  unusedTools: { id: string; name: string; daysSinceUse: number | null }[];
  deadFlags: { cycleId: string; branch: string; status: string }[];
  orphanedTasks: { id: string; title: string; ageDays: number }[];
  generatedAt: string;
}
