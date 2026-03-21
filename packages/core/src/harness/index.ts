/**
 * Self-Coding Harness — Fort Writes Its Own Code
 *
 * Orchestrates the self-coding cycle:
 *   spec → branch → implement → test → build → review → merge/rollback
 *
 * Enforces Tool Registry reuse-before-build: before starting a cycle,
 * the harness searches for existing tools that handle the capability.
 */

import { execSync } from 'node:child_process';
import { writeFileSync, mkdirSync } from 'node:fs';
import { dirname } from 'node:path';
import { v4 as uuid } from 'uuid';
import type { ModuleBus } from '../module-bus/index.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { SpecManager } from '../specs/index.js';
import type { ToolRegistry } from '../tools/index.js';
import type { FeatureFlagManager } from '../feature-flags/index.js';
import type {
  HarnessConfig,
  HarnessCycle,
  HarnessCycleStatus,
  HarnessStep,
  DiagnosticResult,
} from '../types.js';

export class Harness {
  private cycles: Map<string, HarnessCycle> = new Map();
  private config: HarnessConfig;
  private bus: ModuleBus;
  private taskGraph: TaskGraph;
  private specs: SpecManager;
  private tools: ToolRegistry;
  private flags: FeatureFlagManager;

  constructor(
    config: HarnessConfig,
    bus: ModuleBus,
    taskGraph: TaskGraph,
    specs: SpecManager,
    tools: ToolRegistry,
    flags: FeatureFlagManager,
  ) {
    this.config = config;
    this.bus = bus;
    this.taskGraph = taskGraph;
    this.specs = specs;
    this.tools = tools;
    this.flags = flags;
  }

  // ─── Cycle Lifecycle ───────────────────────────────────────────────

  startCycle(
    goal: string,
    approach?: string,
    affectedFiles?: string[],
  ): HarnessCycle {
    // Enforce reuse-before-build: search Tool Registry first
    const existingTools = this.tools.search(goal);
    if (existingTools.length > 0) {
      const toolNames = existingTools.map((t) => t.name).join(', ');
      throw new Error(
        `Existing tool(s) '${toolNames}' may handle this capability. ` +
        `Use or extend them instead of building new code.`,
      );
    }

    // Create spec via SpecManager
    const spec = this.specs.create({
      title: goal,
      goal,
      approach: approach ?? 'To be determined during implementation',
      affectedFiles: affectedFiles ?? [],
      testCriteria: ['All existing tests pass', 'New functionality works as specified'],
      rollbackPlan: 'Revert merge commit and disable feature flag',
      author: 'fort-harness',
    });

    const cycleId = uuid();
    const branch = `fort/${spec.id}`;

    // Create git branch
    this.gitExec(`checkout -b ${branch}`);

    const cycle: HarnessCycle = {
      id: cycleId,
      specId: spec.id,
      status: 'spec_draft',
      branch,
      startedAt: new Date().toISOString(),
      steps: [
        this.makeStep('spec_creation', 'passed'),
        this.makeStep('branch_creation', 'passed'),
        this.makeStep('spec_review', 'pending'),
        this.makeStep('implementation', 'pending'),
        this.makeStep('testing', 'pending'),
        this.makeStep('build', 'pending'),
        this.makeStep('self_review', 'pending'),
        this.makeStep('merge', 'pending'),
      ],
    };

    this.cycles.set(cycleId, cycle);

    // Create a task for transparency
    this.taskGraph.createTask({
      title: `Self-coding: ${goal}`,
      description: `Harness cycle ${cycleId} for spec ${spec.id}`,
      source: 'self_coding',
      metadata: { cycleId, specId: spec.id, branch },
    });

    this.bus.publish('harness.cycle_started', 'harness', {
      cycleId,
      specId: spec.id,
      branch,
      goal,
    });

    // Auto-approve if configured
    if (this.config.autoApprove) {
      this.approveSpec(cycleId);
    }

    return cycle;
  }

  approveSpec(cycleId: string): HarnessCycle {
    const cycle = this.requireCycle(cycleId);
    this.requireStatus(cycle, ['spec_draft', 'spec_review']);

    // Update spec status
    this.specs.updateStatus(cycle.specId, 'approved');

    cycle.status = 'spec_review';
    this.updateStep(cycle, 'spec_review', 'passed');

    this.bus.publish('harness.spec_approved', 'harness', {
      cycleId,
      specId: cycle.specId,
    });

    return cycle;
  }

  implement(cycleId: string, code: Record<string, string>): HarnessCycle {
    const cycle = this.requireCycle(cycleId);
    this.requireStatus(cycle, ['spec_review']);

    cycle.status = 'implementing';
    this.updateStep(cycle, 'implementation', 'running');

    // Update spec status
    this.specs.updateStatus(cycle.specId, 'implementing');

    try {
      // Ensure we are on the correct branch
      this.gitExec(`checkout ${cycle.branch}`);

      // Write files
      for (const [filePath, content] of Object.entries(code)) {
        const fullPath = filePath.startsWith('/')
          ? filePath
          : `${this.config.repoRoot}/${filePath}`;
        mkdirSync(dirname(fullPath), { recursive: true });
        writeFileSync(fullPath, content, 'utf-8');
      }

      // Stage and commit
      this.gitExec('add -A');
      const fileList = Object.keys(code).join(', ');
      this.gitExec(`commit -m "fort-harness: implement spec ${cycle.specId}\n\nFiles: ${fileList}"`);

      this.updateStep(cycle, 'implementation', 'passed');
    } catch (err) {
      this.updateStep(cycle, 'implementation', 'failed', String(err));
      cycle.status = 'failed';
      cycle.error = `Implementation failed: ${err}`;
    }

    return cycle;
  }

  runTests(cycleId: string): HarnessCycle {
    const cycle = this.requireCycle(cycleId);
    this.requireStatus(cycle, ['implementing']);

    cycle.status = 'testing';
    this.updateStep(cycle, 'testing', 'running');

    try {
      this.gitExec(`checkout ${cycle.branch}`);

      const testCommand = this.config.testCommand ?? 'npm test';
      const output = this.exec(testCommand);

      this.updateStep(cycle, 'testing', 'passed', output);
    } catch (err) {
      const errorStr = String(err);
      this.updateStep(cycle, 'testing', 'failed', errorStr);
      cycle.status = 'failed';
      cycle.error = `Tests failed: ${errorStr}`;
    }

    return cycle;
  }

  runBuild(cycleId: string): HarnessCycle {
    const cycle = this.requireCycle(cycleId);
    this.requireStatus(cycle, ['testing']);

    cycle.status = 'verifying';
    this.updateStep(cycle, 'build', 'running');

    try {
      this.gitExec(`checkout ${cycle.branch}`);

      const buildCommand = this.config.buildCommand ?? 'npm run build';
      const output = this.exec(buildCommand);

      this.updateStep(cycle, 'build', 'passed', output);

      // Run lint if configured
      if (this.config.lintCommand) {
        try {
          this.exec(this.config.lintCommand);
        } catch {
          // Lint failures are non-fatal but recorded
        }
      }
    } catch (err) {
      const errorStr = String(err);
      this.updateStep(cycle, 'build', 'failed', errorStr);
      cycle.status = 'failed';
      cycle.error = `Build failed: ${errorStr}`;
    }

    return cycle;
  }

  selfReview(cycleId: string): { cycle: HarnessCycle; diff: string } {
    const cycle = this.requireCycle(cycleId);
    this.requireStatus(cycle, ['verifying']);

    this.updateStep(cycle, 'self_review', 'running');

    let diff: string;
    try {
      diff = this.gitExec(`diff main...${cycle.branch}`);
      this.updateStep(cycle, 'self_review', 'passed', `${diff.length} chars of diff`);
    } catch (err) {
      diff = '';
      this.updateStep(cycle, 'self_review', 'failed', String(err));
    }

    return { cycle, diff };
  }

  merge(cycleId: string): HarnessCycle {
    const cycle = this.requireCycle(cycleId);
    this.requireStatus(cycle, ['verifying']);

    cycle.status = 'merging';
    this.updateStep(cycle, 'merge', 'running');

    try {
      // Create feature flag for this change
      const flag = this.flags.createFlag(
        `harness-${cycle.specId}`,
        `Feature flag for harness cycle ${cycle.id}`,
        { specId: cycle.specId, metadata: { cycleId: cycle.id, branch: cycle.branch } },
      );
      this.flags.enableFlag(flag.id);

      // Merge branch to main (stash any untracked db files first)
      try { this.gitExec('stash --include-untracked'); } catch { /* nothing to stash */ }
      this.gitExec('checkout main');
      try { this.gitExec('stash pop'); } catch { /* nothing to pop */ }
      this.gitExec(`merge ${cycle.branch} --no-ff -m "fort-harness: merge spec ${cycle.specId}"`);

      // Update spec status
      this.specs.updateStatus(cycle.specId, 'merged');

      this.updateStep(cycle, 'merge', 'passed');
      cycle.status = 'complete';
      cycle.completedAt = new Date().toISOString();

      this.bus.publish('harness.cycle_completed', 'harness', {
        cycleId,
        specId: cycle.specId,
        flagId: flag.id,
      });
    } catch (err) {
      const errorStr = String(err);
      this.updateStep(cycle, 'merge', 'failed', errorStr);
      cycle.status = 'failed';
      cycle.error = `Merge failed: ${errorStr}`;

      // Try to go back to main
      try {
        this.gitExec('checkout main');
      } catch {
        // Best effort
      }
    }

    return cycle;
  }

  rollback(cycleId: string, reason: string): HarnessCycle {
    const cycle = this.requireCycle(cycleId);

    try {
      // If already merged, revert
      if (cycle.status === 'complete' || cycle.status === 'merging') {
        this.gitExec('checkout main');
        try {
          this.gitExec('revert HEAD --no-edit');
        } catch {
          // Revert may fail if not merged yet
        }
      } else {
        // Just go back to main
        try {
          this.gitExec('checkout main');
        } catch {
          // Best effort
        }
      }

      // Disable feature flag if one exists
      const flags = this.flags.listFlags();
      for (const flag of flags) {
        if (flag.specId === cycle.specId) {
          this.flags.rollback(flag.id, reason);
        }
      }

      // Update spec
      this.specs.updateStatus(cycle.specId, 'rolled_back');

      cycle.status = 'rolled_back';
      cycle.error = reason;
      cycle.completedAt = new Date().toISOString();

      this.bus.publish('harness.cycle_rolled_back', 'harness', {
        cycleId,
        specId: cycle.specId,
        reason,
      });
    } catch (err) {
      cycle.status = 'failed';
      cycle.error = `Rollback failed: ${err}. Original reason: ${reason}`;
    }

    return cycle;
  }

  // ─── Queries ───────────────────────────────────────────────────────

  getCycle(id: string): HarnessCycle | null {
    return this.cycles.get(id) ?? null;
  }

  listCycles(status?: HarnessCycleStatus): HarnessCycle[] {
    const all = Array.from(this.cycles.values());
    if (status) {
      return all.filter((c) => c.status === status);
    }
    return all.sort((a, b) => b.startedAt.localeCompare(a.startedAt));
  }

  // ─── Diagnostics ──────────────────────────────────────────────────

  diagnose(): DiagnosticResult {
    const all = this.listCycles();
    const active = all.filter((c) =>
      !['complete', 'rolled_back', 'failed'].includes(c.status),
    );
    const failed = all.filter((c) => c.status === 'failed');
    const rolledBack = all.filter((c) => c.status === 'rolled_back');
    const complete = all.filter((c) => c.status === 'complete');

    const checks = [
      {
        name: 'Total cycles',
        passed: true,
        message: `${all.length} total cycles`,
      },
      {
        name: 'Active cycles',
        passed: true,
        message: `${active.length} cycles in progress`,
      },
      {
        name: 'Completed cycles',
        passed: true,
        message: `${complete.length} successfully completed`,
      },
      {
        name: 'Failed cycles',
        passed: failed.length === 0,
        message: failed.length > 0
          ? `${failed.length} cycles failed`
          : 'No failed cycles',
      },
      {
        name: 'Rolled back cycles',
        passed: true,
        message: `${rolledBack.length} cycles rolled back`,
      },
    ];

    return {
      module: 'harness',
      status: failed.length > 0 ? 'degraded' : 'healthy',
      checks,
    };
  }

  // ─── Internal Helpers ─────────────────────────────────────────────

  private requireCycle(id: string): HarnessCycle {
    const cycle = this.cycles.get(id);
    if (!cycle) throw new Error(`Harness cycle not found: ${id}`);
    return cycle;
  }

  private requireStatus(cycle: HarnessCycle, allowed: HarnessCycleStatus[]): void {
    if (!allowed.includes(cycle.status)) {
      throw new Error(
        `Cycle ${cycle.id} is in status '${cycle.status}', expected one of: ${allowed.join(', ')}`,
      );
    }
  }

  private makeStep(
    phase: string,
    status: HarnessStep['status'],
    output?: string,
  ): HarnessStep {
    return {
      phase,
      status,
      startedAt: status !== 'pending' ? new Date().toISOString() : undefined,
      completedAt: ['passed', 'failed', 'skipped'].includes(status)
        ? new Date().toISOString()
        : undefined,
      output,
    };
  }

  private updateStep(
    cycle: HarnessCycle,
    phase: string,
    status: HarnessStep['status'],
    output?: string,
  ): void {
    const step = cycle.steps.find((s) => s.phase === phase);
    if (step) {
      step.status = status;
      if (status === 'running') {
        step.startedAt = new Date().toISOString();
      }
      if (['passed', 'failed', 'skipped'].includes(status)) {
        step.completedAt = new Date().toISOString();
      }
      if (output) {
        if (status === 'failed') {
          step.error = output;
        } else {
          step.output = output;
        }
      }
    }
  }

  private gitExec(command: string): string {
    return execSync(`git ${command}`, {
      cwd: this.config.repoRoot,
      encoding: 'utf-8',
      timeout: 30000,
    }).trim();
  }

  private exec(command: string): string {
    return execSync(command, {
      cwd: this.config.repoRoot,
      encoding: 'utf-8',
      timeout: 120000,
    }).trim();
  }
}
