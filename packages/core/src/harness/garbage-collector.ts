/**
 * Garbage Collector — Periodic Cleanup for Fort
 *
 * Finds stale specs, unused tools, dead feature flags, and orphaned tasks.
 * Generates cleanup reports and optionally performs cleanup.
 */

import type { SpecManager } from '../specs/index.js';
import type { ToolRegistry } from '../tools/index.js';
import type { FeatureFlagManager } from '../feature-flags/index.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { Harness } from './index.js';
import type { GCReport, DiagnosticResult } from '../types.js';

const MS_PER_DAY = 24 * 60 * 60 * 1000;

export class GarbageCollector {
  private specs: SpecManager;
  private tools: ToolRegistry;
  private flags: FeatureFlagManager;
  private taskGraph: TaskGraph;
  private harness: Harness;

  constructor(
    specs: SpecManager,
    tools: ToolRegistry,
    flags: FeatureFlagManager,
    taskGraph: TaskGraph,
    harness: Harness,
  ) {
    this.specs = specs;
    this.tools = tools;
    this.flags = flags;
    this.taskGraph = taskGraph;
    this.harness = harness;
  }

  /**
   * Find specs stuck in draft or implementing for too long.
   */
  findStaleSpecs(maxAgeDays: number = 7): GCReport['staleSpecs'] {
    const now = Date.now();
    const stale: GCReport['staleSpecs'] = [];

    for (const status of ['draft', 'implementing'] as const) {
      const specs = this.specs.list(status);
      for (const spec of specs) {
        const ageDays = (now - spec.createdAt.getTime()) / MS_PER_DAY;
        if (ageDays >= maxAgeDays) {
          stale.push({
            id: spec.id,
            title: spec.title,
            status: spec.status,
            ageDays: Math.round(ageDays),
          });
        }
      }
    }

    return stale;
  }

  /**
   * Find tools with zero recent usage.
   */
  findUnusedTools(minDaysSinceUse: number = 30): GCReport['unusedTools'] {
    const now = Date.now();
    const unused: GCReport['unusedTools'] = [];

    const tools = this.tools.list();
    for (const tool of tools) {
      if (tool.lastUsedAt === null) {
        unused.push({
          id: tool.id,
          name: tool.name,
          daysSinceUse: null,
        });
      } else {
        const daysSinceUse = (now - tool.lastUsedAt.getTime()) / MS_PER_DAY;
        if (daysSinceUse > minDaysSinceUse) {
          unused.push({
            id: tool.id,
            name: tool.name,
            daysSinceUse: Math.round(daysSinceUse),
          });
        }
      }
    }

    return unused;
  }

  /**
   * Find feature flags that are rolled_back or disabled for too long.
   */
  findDeadFlags(): GCReport['deadFlags'] {
    const dead: GCReport['deadFlags'] = [];

    const rolledBack = this.flags.listFlags('rolled_back');
    const disabled = this.flags.listFlags('disabled');

    for (const flag of [...rolledBack, ...disabled]) {
      // Find the harness cycle associated with this flag
      const cycles = this.harness.listCycles();
      const cycle = cycles.find((c) => c.specId === flag.specId);

      dead.push({
        cycleId: cycle?.id ?? 'unknown',
        branch: cycle?.branch ?? `flag:${flag.name}`,
        status: flag.status,
      });
    }

    return dead;
  }

  /**
   * Find tasks stuck in 'in_progress' for too long.
   */
  findOrphanedTasks(maxAgeDays: number = 3): GCReport['orphanedTasks'] {
    const now = Date.now();
    const orphaned: GCReport['orphanedTasks'] = [];

    const active = this.taskGraph.getActiveTasks();
    for (const task of active) {
      const ageDays = (now - task.createdAt.getTime()) / MS_PER_DAY;
      if (ageDays > maxAgeDays) {
        orphaned.push({
          id: task.id,
          title: task.title,
          ageDays: Math.round(ageDays),
        });
      }
    }

    return orphaned;
  }

  /**
   * Generate a comprehensive cleanup report.
   */
  generateReport(options?: {
    staleSpecDays?: number;
    unusedToolDays?: number;
    orphanedTaskDays?: number;
  }): GCReport {
    return {
      staleSpecs: this.findStaleSpecs(options?.staleSpecDays),
      unusedTools: this.findUnusedTools(options?.unusedToolDays),
      deadFlags: this.findDeadFlags(),
      orphanedTasks: this.findOrphanedTasks(options?.orphanedTaskDays),
      generatedAt: new Date().toISOString(),
    };
  }

  diagnose(): DiagnosticResult {
    const report = this.generateReport();
    const totalIssues =
      report.staleSpecs.length +
      report.unusedTools.length +
      report.deadFlags.length +
      report.orphanedTasks.length;

    const checks = [
      {
        name: 'Stale specs',
        passed: report.staleSpecs.length === 0,
        message: report.staleSpecs.length > 0
          ? `${report.staleSpecs.length} specs stuck in draft/implementing`
          : 'No stale specs',
      },
      {
        name: 'Unused tools',
        passed: report.unusedTools.length === 0,
        message: report.unusedTools.length > 0
          ? `${report.unusedTools.length} tools with no recent usage`
          : 'All tools recently used',
      },
      {
        name: 'Dead flags',
        passed: report.deadFlags.length === 0,
        message: report.deadFlags.length > 0
          ? `${report.deadFlags.length} dead feature flags`
          : 'No dead feature flags',
      },
      {
        name: 'Orphaned tasks',
        passed: report.orphanedTasks.length === 0,
        message: report.orphanedTasks.length > 0
          ? `${report.orphanedTasks.length} tasks stuck in progress`
          : 'No orphaned tasks',
      },
    ];

    return {
      module: 'garbage-collector',
      status: totalIssues > 0 ? 'degraded' : 'healthy',
      checks,
    };
  }
}
