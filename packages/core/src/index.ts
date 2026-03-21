/**
 * @fort/core — Public API
 */

// Main
export { Fort } from './fort.js';
export type { FortConfig } from './fort.js';

// Types
export * from './types.js';

// Module Bus
export { ModuleBus } from './module-bus/index.js';

// Task Graph
export { TaskGraph } from './task-graph/index.js';

// Agents
export { BaseAgent, AgentRegistry } from './agents/index.js';
export { OrchestratorAgent } from './agents/orchestrator.js';
export { MemoryAgent } from './agents/memory-agent.js';
export { SchedulerAgent } from './agents/scheduler-agent.js';
export { ReflectionAgent } from './agents/reflection-agent.js';
export { SpecialistAgent } from './agents/specialist.js';
export { AgentHatchery } from './agents/hatchery.js';

// Memory
export { MemoryManager } from './memory/index.js';
export { MemUClient } from './memory/memu-client.js';

// Permissions
export { PermissionManager } from './permissions/index.js';

// Tools
export { ToolRegistry } from './tools/index.js';

// Scheduler
export { Scheduler } from './scheduler/index.js';

// Flows
export { FlowEngine } from './flows/index.js';
export type { ActionHandler } from './flows/index.js';

// Behaviors
export { BehaviorManager } from './behaviors/index.js';

// Routines
export { RoutineManager } from './routines/index.js';

// Specs
export { SpecManager } from './specs/index.js';

// Tokens
export { TokenTracker } from './tokens/index.js';

// Feature Flags
export { FeatureFlagManager } from './feature-flags/index.js';

// Plugins
export { PluginManager } from './plugins/index.js';

// Harness
export { Harness } from './harness/index.js';
export { GarbageCollector } from './harness/garbage-collector.js';

// Rewind
export { RewindManager } from './rewind/index.js';

// Threads
export { ThreadManager } from './threads/index.js';

// Diagnostics
export { FortDoctor } from './diagnostics/index.js';

// Introspection
export { Introspector } from './introspect/index.js';
export type { IntrospectorDeps } from './introspect/index.js';

// Integrations
export {
  IntegrationRegistry,
  BaseIntegration,
  GmailIntegration,
  CalendarIntegration,
  IMessageIntegration,
  BraveSearchIntegration,
  BrowserIntegration,
} from './integrations/index.js';
export type {
  Integration,
  IntegrationStatus,
  IntegrationConfig,
  GmailConfig,
  GmailMessage,
  GmailDraft,
  GmailLabel,
  CalendarConfig,
  CalendarEvent,
  FreeTimeSlot,
  IMessageConfig,
  IMessage,
  IMessageConversation,
  BraveSearchConfig,
  SearchResult,
  SearchResponse,
  SearchSummary,
  BrowserConfig,
  PageContent,
  BrowserAction,
  ActionRisk,
} from './integrations/index.js';

// LLM
export { LLMClient } from './llm/index.js';
export type { LLMClientConfig, LLMRequest, LLMResponse, LLMMessage, LLMStreamEvent, ModelTier, ModelConfig } from './llm/index.js';

// OS Integration
export { OSIntegrationManager } from './os-integration/index.js';

// IPC
export { IPCServer } from './ipc/index.js';

// Server
export { FortServer } from './server/index.js';
