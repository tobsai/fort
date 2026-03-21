/**
 * Introspector — Machine-Readable Documentation System
 *
 * Lets Fort introspect itself: understand its own modules, capabilities,
 * event topology, and architecture. Reads from existing registries —
 * no additional storage needed.
 */

import type {
  DiagnosticResult,
  ModuleManifest,
  ModuleManifestEntry,
  CapabilityEntry,
  EventCatalogEntry,
  SystemProfile,
} from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { AgentRegistry } from '../agents/index.js';
import type { ToolRegistry } from '../tools/index.js';
import { FortDoctor } from '../diagnostics/index.js';

export interface IntrospectorDeps {
  bus: ModuleBus;
  agents: AgentRegistry;
  tools: ToolRegistry;
  doctor: FortDoctor;
  version?: string;
  startedAt?: Date;
}

export class Introspector {
  private bus: ModuleBus;
  private agents: AgentRegistry;
  private tools: ToolRegistry;
  private doctor: FortDoctor;
  private version: string;
  private startedAt: Date;

  constructor(deps: IntrospectorDeps) {
    this.bus = deps.bus;
    this.agents = deps.agents;
    this.tools = deps.tools;
    this.doctor = deps.doctor;
    this.version = deps.version ?? '0.1.0';
    this.startedAt = deps.startedAt ?? new Date();
  }

  /**
   * Generate a complete manifest of all modules, their capabilities,
   * event subscriptions, event emissions, and health status.
   */
  generateModuleManifest(): ModuleManifest {
    const modules: ModuleManifestEntry[] = [];

    // Core infrastructure modules
    const registeredModules = this.doctor.getRegisteredModules();
    for (const moduleName of registeredModules) {
      const entry: ModuleManifestEntry = {
        name: moduleName,
        capabilities: this.getModuleCapabilities(moduleName),
        eventSubscriptions: [],
        eventEmissions: [],
        healthStatus: 'unknown',
      };
      modules.push(entry);
    }

    // Agent modules
    const agentInfos = this.agents.listInfo();
    for (const agent of agentInfos) {
      const existing = modules.find((m) => m.name === 'agents');
      if (existing) {
        existing.capabilities.push(
          ...agent.config.capabilities.map((c) => `agent:${agent.config.name}:${c}`),
        );
      }
    }

    return {
      name: 'fort',
      modules,
      generatedAt: new Date().toISOString(),
    };
  }

  /**
   * Return a flat map of every capability the system has
   * (from tools, agents, integrations) with what provides it.
   */
  generateCapabilityMap(): CapabilityEntry[] {
    const capabilities: CapabilityEntry[] = [];

    // Tools
    const tools = this.tools.list();
    for (const tool of tools) {
      for (const cap of tool.capabilities) {
        capabilities.push({
          capability: cap,
          provider: tool.name,
          providerType: 'tool',
          description: tool.description,
        });
      }
    }

    // Agents
    const agents = this.agents.listInfo();
    for (const agent of agents) {
      for (const cap of agent.config.capabilities) {
        capabilities.push({
          capability: cap,
          provider: agent.config.name,
          providerType: 'agent',
          description: agent.config.description,
        });
      }
    }

    // Doctor-registered modules as module capabilities
    const registeredModules = this.doctor.getRegisteredModules();
    for (const moduleName of registeredModules) {
      capabilities.push({
        capability: `module:${moduleName}`,
        provider: moduleName,
        providerType: 'module',
        description: `${moduleName} module`,
      });
    }

    return capabilities;
  }

  /**
   * Return all event types flowing through the ModuleBus
   * with publishers and subscriber counts.
   */
  generateEventCatalog(): EventCatalogEntry[] {
    const catalog: EventCatalogEntry[] = [];

    // Get active subscription event types
    const eventTypes = this.bus.getEventTypes();
    for (const eventType of eventTypes) {
      const subscriberCount = this.bus.getSubscriptionCount(eventType);
      // Derive publishers from event history
      const history = this.bus.getHistory(eventType);
      const publishers = [...new Set(history.map((e) => e.source))];

      catalog.push({
        eventType,
        publishers,
        subscriberCount,
      });
    }

    // Also include event types only seen in history (no active subscribers)
    const allHistory = this.bus.getHistory();
    const historyTypes = [...new Set(allHistory.map((e) => e.type))];
    for (const eventType of historyTypes) {
      if (!eventTypes.includes(eventType)) {
        const history = this.bus.getHistory(eventType);
        const publishers = [...new Set(history.map((e) => e.source))];
        catalog.push({
          eventType,
          publishers,
          subscriberCount: 0,
        });
      }
    }

    return catalog;
  }

  /**
   * Return full system profile — version, uptime, module count,
   * agent count, tool count, task count, diagnostic summary.
   */
  async generateSystemProfile(): Promise<SystemProfile> {
    const agents = this.agents.listInfo();
    const tools = this.tools.list();
    const registeredModules = this.doctor.getRegisteredModules();
    const uptime = Date.now() - this.startedAt.getTime();

    let diagnosticSummary: SystemProfile['diagnosticSummary'] = null;
    try {
      const results = await this.doctor.runAll();
      const summary = FortDoctor.summarize(results);
      diagnosticSummary = {
        overall: summary.overall,
        totalChecks: summary.totalChecks,
        passed: summary.passed,
        failed: summary.failed,
      };
    } catch {
      // Doctor may fail; leave summary null
    }

    return {
      version: this.version,
      uptime,
      moduleCount: registeredModules.length,
      agentCount: agents.length,
      toolCount: tools.length,
      integrationCount: 0,
      taskCount: agents.reduce((sum, a) => sum + a.taskCount, 0),
      diagnosticSummary,
      generatedAt: new Date().toISOString(),
    };
  }

  /**
   * Search across all capabilities (tools, agents, module skills).
   * Used by the harness before building new things.
   */
  searchCapabilities(query: string): CapabilityEntry[] {
    const allCapabilities = this.generateCapabilityMap();
    const terms = query.toLowerCase().split(/\s+/).filter(Boolean);
    if (terms.length === 0) return [];

    return allCapabilities.filter((entry) => {
      const searchable = [
        entry.capability,
        entry.provider,
        entry.description,
        entry.providerType,
      ]
        .join(' ')
        .toLowerCase();

      return terms.every((term) => searchable.includes(term));
    });
  }

  /**
   * Export full system documentation in JSON or Markdown format.
   */
  async exportDocumentation(format: 'json' | 'markdown'): Promise<string> {
    const manifest = this.generateModuleManifest();
    const capabilities = this.generateCapabilityMap();
    const events = this.generateEventCatalog();
    const profile = await this.generateSystemProfile();

    const doc = { manifest, capabilities, events, profile };

    if (format === 'json') {
      return JSON.stringify(doc, null, 2);
    }

    return this.renderMarkdown(doc);
  }

  /**
   * Diagnostic check for the introspector itself.
   */
  diagnose(): DiagnosticResult {
    const checks = [];

    try {
      const manifest = this.generateModuleManifest();
      checks.push({
        name: 'Module manifest generation',
        passed: true,
        message: `${manifest.modules.length} modules discovered`,
      });
    } catch (err) {
      checks.push({
        name: 'Module manifest generation',
        passed: false,
        message: `Failed: ${err instanceof Error ? err.message : String(err)}`,
      });
    }

    try {
      const capabilities = this.generateCapabilityMap();
      checks.push({
        name: 'Capability map generation',
        passed: true,
        message: `${capabilities.length} capabilities mapped`,
      });
    } catch (err) {
      checks.push({
        name: 'Capability map generation',
        passed: false,
        message: `Failed: ${err instanceof Error ? err.message : String(err)}`,
      });
    }

    try {
      const events = this.generateEventCatalog();
      checks.push({
        name: 'Event catalog generation',
        passed: true,
        message: `${events.length} event types cataloged`,
      });
    } catch (err) {
      checks.push({
        name: 'Event catalog generation',
        passed: false,
        message: `Failed: ${err instanceof Error ? err.message : String(err)}`,
      });
    }

    return {
      module: 'introspect',
      status: checks.every((c) => c.passed) ? 'healthy' : 'unhealthy',
      checks,
    };
  }

  // ─── Private Helpers ──────────────────────────────────────────────

  private getModuleCapabilities(moduleName: string): string[] {
    // Map known modules to their core capabilities
    const moduleCapabilityMap: Record<string, string[]> = {
      memory: ['node_storage', 'graph_query', 'search'],
      tools: ['tool_registration', 'tool_search', 'capability_lookup'],
      tokens: ['token_tracking', 'budget_management', 'cost_reporting'],
      'feature-flags': ['flag_management', 'bake_tracking', 'rollback'],
      plugins: ['plugin_loading', 'plugin_lifecycle', 'security_audit'],
      harness: ['spec_driven_development', 'build_test_cycle', 'code_generation'],
      'garbage-collector': ['stale_spec_detection', 'unused_tool_detection', 'orphan_cleanup'],
      behaviors: ['behavior_management', 'rule_application'],
      routines: ['routine_scheduling', 'routine_execution'],
      scheduler: ['task_scheduling', 'cron_management'],
      agents: ['agent_lifecycle', 'inter_agent_messaging', 'task_delegation'],
      tasks: ['task_tracking', 'task_graph', 'dependency_management'],
    };

    return moduleCapabilityMap[moduleName] ?? [];
  }

  private renderMarkdown(doc: {
    manifest: ModuleManifest;
    capabilities: CapabilityEntry[];
    events: EventCatalogEntry[];
    profile: SystemProfile;
  }): string {
    const lines: string[] = [];

    lines.push('# Fort System Documentation');
    lines.push('');
    lines.push(`Generated: ${doc.profile.generatedAt}`);
    lines.push(`Version: ${doc.profile.version}`);
    lines.push('');

    // System Profile
    lines.push('## System Profile');
    lines.push('');
    lines.push(`- **Version**: ${doc.profile.version}`);
    lines.push(`- **Uptime**: ${Math.floor(doc.profile.uptime / 1000)}s`);
    lines.push(`- **Modules**: ${doc.profile.moduleCount}`);
    lines.push(`- **Agents**: ${doc.profile.agentCount}`);
    lines.push(`- **Tools**: ${doc.profile.toolCount}`);
    lines.push(`- **Tasks processed**: ${doc.profile.taskCount}`);
    if (doc.profile.diagnosticSummary) {
      lines.push(`- **Health**: ${doc.profile.diagnosticSummary.overall} (${doc.profile.diagnosticSummary.passed}/${doc.profile.diagnosticSummary.totalChecks} checks passed)`);
    }
    lines.push('');

    // Modules
    lines.push('## Modules');
    lines.push('');
    for (const mod of doc.manifest.modules) {
      lines.push(`### ${mod.name}`);
      lines.push('');
      if (mod.capabilities.length > 0) {
        lines.push('Capabilities:');
        for (const cap of mod.capabilities) {
          lines.push(`- ${cap}`);
        }
        lines.push('');
      }
    }

    // Capabilities
    lines.push('## Capabilities');
    lines.push('');
    lines.push('| Capability | Provider | Type | Description |');
    lines.push('|------------|----------|------|-------------|');
    for (const cap of doc.capabilities) {
      lines.push(`| ${cap.capability} | ${cap.provider} | ${cap.providerType} | ${cap.description} |`);
    }
    lines.push('');

    // Events
    if (doc.events.length > 0) {
      lines.push('## Event Catalog');
      lines.push('');
      lines.push('| Event Type | Publishers | Subscribers |');
      lines.push('|------------|-----------|-------------|');
      for (const evt of doc.events) {
        lines.push(`| ${evt.eventType} | ${evt.publishers.join(', ') || 'none'} | ${evt.subscriberCount} |`);
      }
      lines.push('');
    }

    return lines.join('\n');
  }
}
