/**
 * Fort — Main Application Class
 *
 * Wires together all modules and provides the top-level API.
 * Deterministic by default — core services (orchestrator, reflection)
 * are infrastructure, not agents. Only user-created specialists are agents.
 */

import { join } from 'node:path';
import { existsSync, mkdirSync } from 'node:fs';
import Database from 'better-sqlite3';
import { ModuleBus } from './module-bus/index.js';
import { TaskGraph, TaskStore } from './task-graph/index.js';
import { AgentRegistry } from './agents/index.js';
import { AgentFactory } from './agents/hatchery.js';
import { AgentStore } from './agents/store.js';
import { OrchestratorService } from './services/orchestrator.js';
import { ReflectionService } from './services/reflection.js';
import { MemoryManager } from './memory/index.js';
import { PermissionManager } from './permissions/index.js';
import { ToolRegistry, ToolExecutor } from './tools/index.js';
import { Scheduler, SchedulerStore } from './scheduler/index.js';
import { SpecManager } from './specs/index.js';
import { TokenTracker } from './tokens/index.js';
import { BehaviorManager } from './behaviors/index.js';
import { RoutineManager } from './routines/index.js';
import { FeatureFlagManager } from './feature-flags/index.js';
import { PluginManager } from './plugins/index.js';
import { Harness } from './harness/index.js';
import { GarbageCollector } from './harness/garbage-collector.js';
import { RewindManager } from './rewind/index.js';
import { ThreadManager } from './threads/index.js';
import { NotificationStore } from './notifications/store.js';
import { NotificationService } from './notifications/service.js';
import { FortDoctor } from './diagnostics/index.js';
import { Introspector } from './introspect/index.js';
import { IPCServer } from './ipc/index.js';
import { OSIntegrationManager } from './os-integration/index.js';
import { LLMClient } from './llm/index.js';
import type { LLMClientConfig } from './llm/index.js';
import { UsageStore, UsageTracker } from './usage/index.js';
import { LLMProviderStore } from './llm/provider-store.js';

import type { DiagnosticResult, Task } from './types.js';

export interface FortConfig {
  dataDir: string;
  specsDir: string;
  repoRoot?: string;
  memuUrl?: string;
  permissionsPath?: string;
  agentsDir?: string;
  llm?: LLMClientConfig;
}

export class Fort {
  // Core infrastructure
  readonly bus: ModuleBus;
  readonly taskGraph: TaskGraph;

  // Agent registry (specialist agents only — no core agents)
  readonly agents: AgentRegistry;
  readonly agentFactory: AgentFactory;
  readonly agentStore: AgentStore;

  // Deterministic services (not agents)
  readonly orchestrator: OrchestratorService;
  readonly reflection: ReflectionService;

  // Modules
  readonly memory: MemoryManager;
  readonly permissions: PermissionManager;
  readonly tools: ToolRegistry;
  readonly toolExecutor: ToolExecutor;
  readonly scheduler: Scheduler;
  readonly specs: SpecManager;
  readonly tokens: TokenTracker;
  readonly behaviors: BehaviorManager;
  readonly routines: RoutineManager;
  readonly flags: FeatureFlagManager;
  readonly plugins: PluginManager;
  readonly harness: Harness;
  readonly gc: GarbageCollector;
  readonly rewind: RewindManager;
  readonly threads: ThreadManager;
  readonly notifications: NotificationService;
  readonly llm: LLMClient;
  readonly usageStore: UsageStore;
  readonly usageTracker: UsageTracker;
  readonly llmProviders: LLMProviderStore;

  readonly introspect: Introspector;
  readonly osIntegration: OSIntegrationManager;
  readonly ipc: IPCServer;
  readonly doctor: FortDoctor;

  private config: FortConfig;
  private taskDb: InstanceType<typeof Database> | null = null;

  constructor(config: FortConfig) {
    this.config = config;

    if (!existsSync(config.dataDir)) {
      mkdirSync(config.dataDir, { recursive: true });
    }

    // Core infrastructure
    this.bus = new ModuleBus();

    // Task persistence (shared SQLite DB for tasks)
    const taskDbPath = join(config.dataDir, 'tasks.db');
    this.taskDb = new (Database as any)(taskDbPath) as InstanceType<typeof Database>;
    (this.taskDb as any).pragma('journal_mode = WAL');
    const taskStore = new TaskStore(this.taskDb);
    taskStore.initSchema();

    this.taskGraph = new TaskGraph(this.bus, taskStore);
    this.agents = new AgentRegistry(this.bus);

    // Scheduler DB (shares the tasks.db — separate table)
    const schedulerStore = new SchedulerStore(this.taskDb);
    schedulerStore.initSchema();

    // Modules
    this.memory = new MemoryManager(
      join(config.dataDir, 'memory.db'),
      this.bus,
      config.memuUrl
    );
    this.permissions = new PermissionManager(
      config.permissionsPath ?? join(config.dataDir, 'permissions.yaml')
    );
    this.tools = new ToolRegistry(join(config.dataDir, 'tools.db'));
    this.tokens = new TokenTracker(join(config.dataDir, 'tokens.db'), this.bus);
    this.toolExecutor = new ToolExecutor(this.permissions, this.bus, this.tokens);
    this.scheduler = new Scheduler(this.bus, this.taskGraph, schedulerStore);
    this.specs = new SpecManager(config.specsDir);
    this.behaviors = new BehaviorManager(this.memory, this.bus);
    this.routines = new RoutineManager(this.memory, this.bus, this.scheduler, this.taskGraph);
    this.flags = new FeatureFlagManager(join(config.dataDir, 'flags.db'), this.bus);
    this.plugins = new PluginManager(
      join(config.dataDir, 'plugins'),
      this.bus,
      this.permissions,
    );
    this.harness = new Harness(
      {
        repoRoot: config.repoRoot ?? process.cwd(),
        testCommand: 'npm test',
        buildCommand: 'npm run build',
      },
      this.bus,
      this.taskGraph,
      this.specs,
      this.tools,
      this.flags,
    );
    this.gc = new GarbageCollector(
      this.specs,
      this.tools,
      this.flags,
      this.taskGraph,
      this.harness,
    );
    this.rewind = new RewindManager(
      join(config.dataDir, 'rewind.db'),
      config.dataDir,
      this.bus,
    );
    this.threads = new ThreadManager(
      join(config.dataDir, 'threads.db'),
      this.bus,
      this.taskGraph,
    );

    // Notifications — shared task DB
    const notificationStore = new NotificationStore(this.taskDb as InstanceType<typeof Database>);
    notificationStore.initSchema();
    this.notifications = new NotificationService(notificationStore, this.bus);
    // LLM provider store — encryption key derived from SESSION_SECRET env var
    const encryptionKey = process.env.SESSION_SECRET ?? 'fort-default-llm-encryption-key';
    this.llmProviders = new LLMProviderStore(join(config.dataDir, 'llm-providers.db'), encryptionKey);

    this.llm = new LLMClient(
      { ...(config.llm ?? {}), providerStore: this.llmProviders },
      this.bus,
      this.tokens,
      this.behaviors,
      this.memory,
    );

    // Usage tracking
    this.usageStore = new UsageStore(this.taskDb);
    this.usageStore.initSchema();
    this.usageStore.seedDefaultPricing();
    this.usageTracker = new UsageTracker(this.usageStore, this.bus);
    this.usageTracker.start();

    // Wire LLM into task graph for completion review
    this.taskGraph.setLLM(this.llm);

    // Deterministic services
    this.orchestrator = new OrchestratorService(this.taskGraph, this.agents, this.bus);
    this.reflection = new ReflectionService(this.taskGraph, this.bus, this.llm);

    // Agent store (SQLite persistence for agent metadata)
    this.agentStore = new AgentStore(
      join(config.dataDir, 'agents-store.db'),
      config.agentsDir ?? join(config.dataDir, 'agents'),
    );

    // Agent factory (specialist agents only)
    this.agentFactory = new AgentFactory(
      config.agentsDir ?? join(config.dataDir, 'agents'),
      this.bus,
      this.taskGraph,
      this.memory,
      this.agents,
    );
    this.agentFactory.setLLM(this.llm);
    this.agentFactory.setToolRegistry(this.tools);
    this.agentFactory.setToolExecutor(this.toolExecutor);

    // Diagnostics and introspection
    this.doctor = new FortDoctor();
    this.introspect = new Introspector({
      bus: this.bus,
      agents: this.agents,
      tools: this.tools,
      doctor: this.doctor,
    });
    this.osIntegration = new OSIntegrationManager(this);
    this.ipc = new IPCServer(this);

    // Register diagnostic providers
    this.doctor.register('memory', this.memory);
    this.doctor.register('tools', this.tools);
    this.doctor.register('tokens', this.tokens);
    this.doctor.register('feature-flags', this.flags);
    this.doctor.register('plugins', this.plugins);
    this.doctor.register('harness', this.harness);
    this.doctor.register('garbage-collector', this.gc);
    this.doctor.register('rewind', this.rewind);
    this.doctor.register('threads', this.threads);
    this.doctor.register('behaviors', this.behaviors);
    this.doctor.register('routines', this.routines);
    this.doctor.register('scheduler', this.scheduler);
    this.doctor.register('llm', this.llm);
    this.doctor.register('usage-tracker', this.usageTracker);
    this.doctor.register('reflection', this.reflection);
    this.doctor.register('os-integration', this.osIntegration);
    this.doctor.register('ipc', this.ipc);
    this.doctor.register('introspect', this.introspect);
    this.doctor.register('agents', {
      diagnose: () => this.agentDiagnostics(),
    });
    this.doctor.register('tasks', {
      diagnose: () => this.taskDiagnostics(),
    });
  }

  async start(): Promise<void> {
    this.taskGraph.loadFromStore();
    await this.memory.initialize();
    await this.agentFactory.loadAll();
    await this.agents.startAll();
    this.notifications.start();
    if (process.env['FORT_SCHEDULER_ENABLED'] !== 'false') {
      this.scheduler.start();
    }
    try {
      await this.ipc.start();
    } catch {
      // Port may be in use
    }

    this.bus.publish('fort.started', 'fort', {
      timestamp: new Date(),
      agents: this.agents.listInfo().length,
    });
  }

  async stop(): Promise<void> {
    await this.ipc.stop();
    this.notifications.stop();
    await this.plugins.shutdownAll();
    this.scheduler.shutdown();
    await this.agents.stopAll();
    this.memory.close();
    this.tools.close();
    this.tokens.close();
    this.flags.close();
    this.usageTracker.stop();
    this.rewind.close();
    this.threads.close();
    this.agentStore.close();

    this.taskDb?.close();

    this.llmProviders.close();


    this.bus.publish('fort.stopped', 'fort', { timestamp: new Date() });
    this.bus.clear();
  }

  /**
   * Send a chat message to an agent. Creates a task and routes it.
   * Returns the task (which will contain the result when complete).
   */
  async chat(message: string, source: string = 'user_chat', agentId?: string, modelTier?: string): Promise<Task> {
    return this.orchestrator.routeChat(message, source as any, agentId, modelTier);
  }

  async runDoctor(): Promise<DiagnosticResult[]> {
    return this.doctor.runAll();
  }

  /**
   * Check if initial setup is complete (at least one default agent exists).
   */
  isSetupComplete(): boolean {
    const identities = this.agentFactory.listIdentities();
    return identities.some((i) => i.isDefault && i.status === 'active');
  }

  private agentDiagnostics(): DiagnosticResult {
    const agents = this.agents.listInfo();
    const running = agents.filter((a) => a.status === 'running');
    const errored = agents.filter((a) => a.status === 'error');

    const checks = [
      {
        name: 'Specialist agents',
        passed: true,
        message: `${agents.length} agents registered (${running.length} running)`,
      },
    ];

    if (errored.length > 0) {
      checks.push({
        name: 'Agent errors',
        passed: false,
        message: `${errored.length} agents in error state: ${errored.map((a) => a.config.name).join(', ')}`,
      });
    }

    for (const agent of agents) {
      checks.push({
        name: `Agent: ${agent.config.name}`,
        passed: agent.status === 'running',
        message: `${agent.status} | ${agent.taskCount} tasks | ${agent.errorCount} errors`,
      });
    }

    return {
      module: 'agents',
      status: errored.length > 0 ? 'degraded' : 'healthy',
      checks,
    };
  }

  private taskDiagnostics(): DiagnosticResult {
    const allTasks = this.taskGraph.getAllTasks();
    const active = this.taskGraph.getActiveTasks();
    const blocked = this.taskGraph.getBlockedTasks();
    const stale = this.taskGraph.getStaleTasks();

    const checks = [
      { name: 'Task count', passed: true, message: `${allTasks.length} total tasks` },
      { name: 'Active tasks', passed: true, message: `${active.length} in progress` },
      {
        name: 'Blocked tasks',
        passed: blocked.length === 0,
        message: blocked.length > 0 ? `${blocked.length} tasks blocked` : 'No blocked tasks',
      },
      {
        name: 'Stale tasks',
        passed: stale.length === 0,
        message: stale.length > 0 ? `${stale.length} stale tasks need attention` : 'No stale tasks',
      },
    ];

    return {
      module: 'tasks',
      status: stale.length > 0 ? 'degraded' : 'healthy',
      checks,
    };
  }
}
