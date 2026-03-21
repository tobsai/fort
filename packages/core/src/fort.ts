/**
 * Fort — Main Application Class
 *
 * Wires together all modules and provides the top-level API.
 * This is the entry point for both the CLI and the Swift shell.
 */

import { join } from 'node:path';
import { existsSync, mkdirSync } from 'node:fs';
import { ModuleBus } from './module-bus/index.js';
import { TaskGraph } from './task-graph/index.js';
import { AgentRegistry } from './agents/index.js';
import { OrchestratorAgent } from './agents/orchestrator.js';
import { MemoryAgent } from './agents/memory-agent.js';
import { SchedulerAgent } from './agents/scheduler-agent.js';
import { ReflectionAgent } from './agents/reflection-agent.js';
import { AgentHatchery } from './agents/hatchery.js';
import { MemoryManager } from './memory/index.js';
import { PermissionManager } from './permissions/index.js';
import { ToolRegistry } from './tools/index.js';
import { Scheduler } from './scheduler/index.js';
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
import { FortDoctor } from './diagnostics/index.js';
import { Introspector } from './introspect/index.js';
import { IPCServer } from './ipc/index.js';
import { OSIntegrationManager } from './os-integration/index.js';
import { LLMClient } from './llm/index.js';
import type { LLMClientConfig } from './llm/index.js';
import type { DiagnosticResult } from './types.js';

export interface FortConfig {
  dataDir: string;
  specsDir: string;
  repoRoot?: string;
  memuUrl?: string;
  permissionsPath?: string;
  llm?: LLMClientConfig;
}

export class Fort {
  readonly bus: ModuleBus;
  readonly taskGraph: TaskGraph;
  readonly agents: AgentRegistry;
  readonly memory: MemoryManager;
  readonly permissions: PermissionManager;
  readonly tools: ToolRegistry;
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
  readonly introspect: Introspector;
  readonly llm: LLMClient;
  readonly osIntegration: OSIntegrationManager;
  readonly ipc: IPCServer;
  readonly doctor: FortDoctor;
  readonly hatchery: AgentHatchery;

  private orchestrator: OrchestratorAgent;
  private memoryAgent: MemoryAgent;
  private schedulerAgent: SchedulerAgent;
  private reflectionAgent: ReflectionAgent;
  private config: FortConfig;

  constructor(config: FortConfig) {
    this.config = config;

    // Ensure data directory exists
    if (!existsSync(config.dataDir)) {
      mkdirSync(config.dataDir, { recursive: true });
    }

    // Initialize core infrastructure
    this.bus = new ModuleBus();
    this.taskGraph = new TaskGraph(this.bus);
    this.agents = new AgentRegistry(this.bus);

    // Initialize modules
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
    this.scheduler = new Scheduler(this.bus, this.taskGraph);
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
    this.llm = new LLMClient(
      config.llm ?? {},
      this.bus,
      this.tokens,
      this.behaviors,
      this.memory,
    );
    this.doctor = new FortDoctor();
    this.introspect = new Introspector({
      bus: this.bus,
      agents: this.agents,
      tools: this.tools,
      doctor: this.doctor,
    });
    this.osIntegration = new OSIntegrationManager(this);
    this.ipc = new IPCServer(this);

    // Create core agents
    this.orchestrator = new OrchestratorAgent(this.bus, this.taskGraph);
    this.memoryAgent = new MemoryAgent(this.bus, this.taskGraph, this.memory);
    this.schedulerAgent = new SchedulerAgent(this.bus, this.taskGraph, this.scheduler);
    this.reflectionAgent = new ReflectionAgent(this.bus, this.taskGraph, this.memory);

    // Register agents
    this.agents.register(this.orchestrator);
    this.agents.register(this.memoryAgent);
    this.agents.register(this.schedulerAgent);
    this.agents.register(this.reflectionAgent);
    this.orchestrator.setRegistry(this.agents);

    // Hatchery for specialist agents
    this.hatchery = new AgentHatchery(
      join(config.dataDir, 'agents'),
      this.bus,
      this.taskGraph,
      this.memory,
      this.agents,
    );

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
    await this.memory.initialize();
    await this.hatchery.loadAll();
    await this.agents.startAll();
    // IPC is best-effort — don't block startup if port is unavailable
    try {
      await this.ipc.start();
    } catch {
      // Port may be in use (e.g. another Fort instance or test runner)
    }

    this.bus.publish('fort.started', 'fort', {
      timestamp: new Date(),
      agents: this.agents.listInfo().length,
    });
  }

  async stop(): Promise<void> {
    await this.ipc.stop();
    await this.plugins.shutdownAll();
    this.scheduler.shutdown();
    await this.agents.stopAll();
    this.memory.close();
    this.tools.close();
    this.tokens.close();
    this.flags.close();
    this.rewind.close();
    this.threads.close();

    this.bus.publish('fort.stopped', 'fort', { timestamp: new Date() });
    this.bus.clear();
  }

  async chat(message: string, source: string = 'user_chat'): Promise<string> {
    return this.orchestrator.handleUserInput(message, source);
  }

  async runDoctor(): Promise<DiagnosticResult[]> {
    return this.doctor.runAll();
  }

  private agentDiagnostics(): DiagnosticResult {
    const agents = this.agents.listInfo();
    const running = agents.filter((a) => a.status === 'running');
    const errored = agents.filter((a) => a.status === 'error');

    const checks = [
      {
        name: 'Agent count',
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
      {
        name: 'Task count',
        passed: true,
        message: `${allTasks.length} total tasks`,
      },
      {
        name: 'Active tasks',
        passed: true,
        message: `${active.length} in progress`,
      },
      {
        name: 'Blocked tasks',
        passed: blocked.length === 0,
        message: blocked.length > 0
          ? `${blocked.length} tasks blocked`
          : 'No blocked tasks',
      },
      {
        name: 'Stale tasks',
        passed: stale.length === 0,
        message: stale.length > 0
          ? `${stale.length} stale tasks need attention`
          : 'No stale tasks',
      },
    ];

    return {
      module: 'tasks',
      status: stale.length > 0 ? 'degraded' : 'healthy',
      checks,
    };
  }
}
