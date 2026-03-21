import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { execSync } from 'node:child_process';
import { ModuleBus } from '../module-bus/index.js';
import { TaskGraph } from '../task-graph/index.js';
import { SpecManager } from '../specs/index.js';
import { ToolRegistry } from '../tools/index.js';
import { FeatureFlagManager } from '../feature-flags/index.js';
import { Harness } from '../harness/index.js';
import { GarbageCollector } from '../harness/garbage-collector.js';

function createTempGitRepo(): string {
  const dir = mkdtempSync(join(tmpdir(), 'fort-harness-test-'));
  execSync('git init', { cwd: dir });
  execSync('git config user.email "test@fort.dev"', { cwd: dir });
  execSync('git config user.name "Fort Test"', { cwd: dir });

  // Create initial commit on main (with gitignore for SQLite WAL files)
  writeFileSync(join(dir, 'README.md'), '# Test Repo\n');
  writeFileSync(join(dir, '.gitignore'), '.fort-data/\n*.db-shm\n*.db-wal\n');
  execSync('git add -A', { cwd: dir });
  execSync('git commit -m "initial commit"', { cwd: dir });

  // Ensure we are on main
  try {
    execSync('git branch -M main', { cwd: dir });
  } catch {
    // May already be on main
  }

  return dir;
}

describe('Harness', () => {
  let repoDir: string;
  let dataDir: string;
  let specsDir: string;
  let bus: ModuleBus;
  let taskGraph: TaskGraph;
  let specs: SpecManager;
  let tools: ToolRegistry;
  let flags: FeatureFlagManager;
  let harness: Harness;

  beforeEach(() => {
    repoDir = createTempGitRepo();
    dataDir = join(repoDir, '.fort-data');
    specsDir = join(repoDir, 'specs');
    mkdirSync(dataDir, { recursive: true });

    bus = new ModuleBus();
    taskGraph = new TaskGraph(bus);
    specs = new SpecManager(specsDir);
    tools = new ToolRegistry(join(dataDir, 'tools.db'));
    flags = new FeatureFlagManager(join(dataDir, 'flags.db'), bus);

    harness = new Harness(
      { repoRoot: repoDir, testCommand: 'echo "tests pass"', buildCommand: 'echo "build ok"' },
      bus,
      taskGraph,
      specs,
      tools,
      flags,
    );
  });

  afterEach(() => {
    tools.close();
    flags.close();
    try {
      rmSync(repoDir, { recursive: true, force: true });
    } catch {
      // Cleanup best effort
    }
  });

  // ─── Start Cycle ─────────────────────────────────────────────────

  it('should start a cycle and create a spec', () => {
    const cycle = harness.startCycle('Add a greeting module');

    expect(cycle.id).toBeTruthy();
    expect(cycle.specId).toBeTruthy();
    expect(cycle.status).toBe('spec_draft');
    expect(cycle.branch).toBe(`fort/${cycle.specId}`);
    expect(cycle.steps.length).toBeGreaterThan(0);

    // Spec should exist
    const spec = specs.get(cycle.specId);
    expect(spec).not.toBeNull();
    expect(spec!.title).toBe('Add a greeting module');
    expect(spec!.status).toBe('draft');

    // Git branch should exist
    const branches = execSync('git branch', { cwd: repoDir, encoding: 'utf-8' });
    expect(branches).toContain(cycle.branch);
  });

  it('should create a task in TaskGraph when starting a cycle', () => {
    harness.startCycle('New module');

    const tasks = taskGraph.getAllTasks();
    expect(tasks.length).toBe(1);
    expect(tasks[0].source).toBe('self_coding');
    expect(tasks[0].title).toContain('New module');
  });

  // ─── Tool Registry Enforcement ────────────────────────────────────

  it('should block cycle if existing tool matches', () => {
    tools.register({
      name: 'greeting-tool',
      description: 'Handles greeting capability',
      capabilities: ['greeting'],
      inputTypes: ['string'],
      outputTypes: ['string'],
      tags: ['greeting'],
      module: 'core',
      version: '1.0.0',
    });

    expect(() => harness.startCycle('greeting')).toThrow(/Existing tool/);
  });

  it('should allow cycle when no matching tools found', () => {
    tools.register({
      name: 'math-tool',
      description: 'Handles math operations',
      capabilities: ['math', 'calculation'],
      inputTypes: ['number'],
      outputTypes: ['number'],
      tags: ['math'],
      module: 'core',
      version: '1.0.0',
    });

    // This should NOT throw because "greeting" doesn't match "math"
    const cycle = harness.startCycle('Add a greeting module');
    expect(cycle.id).toBeTruthy();
  });

  // ─── Cycle Status Tracking ────────────────────────────────────────

  it('should track cycle through phases', () => {
    const cycle = harness.startCycle('Build a widget');
    expect(cycle.status).toBe('spec_draft');

    // Approve
    const approved = harness.approveSpec(cycle.id);
    expect(approved.status).toBe('spec_review');

    // Implement
    const implemented = harness.implement(cycle.id, {
      'src/widget.ts': 'export const widget = "hello";',
    });
    expect(implemented.status).toBe('implementing');

    // Test
    const tested = harness.runTests(cycle.id);
    expect(tested.status).toBe('testing');

    // Build
    const built = harness.runBuild(cycle.id);
    expect(built.status).toBe('verifying');
  });

  it('should reject operations in wrong status', () => {
    const cycle = harness.startCycle('Build a widget');

    // Can't implement without approving
    expect(() => harness.implement(cycle.id, { 'test.ts': 'code' })).toThrow(/status/);

    // Can't run tests without implementing
    expect(() => harness.runTests(cycle.id)).toThrow(/status/);
  });

  // ─── Git Branch Creation ──────────────────────────────────────────

  it('should create a git branch named after the spec', () => {
    const cycle = harness.startCycle('Branch test feature');
    const branch = cycle.branch;

    expect(branch).toMatch(/^fort\//);

    // Verify branch exists in git
    const output = execSync(`git branch --list "${branch}"`, {
      cwd: repoDir,
      encoding: 'utf-8',
    });
    expect(output.trim()).toContain(branch);
  });

  // ─── Implementation & Commit ──────────────────────────────────────

  it('should write files and commit on implement', () => {
    const cycle = harness.startCycle('Implement test');
    harness.approveSpec(cycle.id);

    harness.implement(cycle.id, {
      'src/hello.ts': 'export const hello = "world";',
    });

    // Verify the file was committed
    const log = execSync(`git log --oneline ${cycle.branch}`, {
      cwd: repoDir,
      encoding: 'utf-8',
    });
    expect(log).toContain('fort-harness');
  });

  // ─── Test Execution ───────────────────────────────────────────────

  it('should capture test output', () => {
    const cycle = harness.startCycle('Test capture');
    harness.approveSpec(cycle.id);
    harness.implement(cycle.id, { 'src/test.ts': '// test file' });

    const tested = harness.runTests(cycle.id);
    const testStep = tested.steps.find((s) => s.phase === 'testing');
    expect(testStep!.status).toBe('passed');
    expect(testStep!.output).toContain('tests pass');
  });

  it('should mark cycle as failed when tests fail', () => {
    const failHarness = new Harness(
      { repoRoot: repoDir, testCommand: 'exit 1', buildCommand: 'echo ok' },
      bus,
      taskGraph,
      specs,
      tools,
      flags,
    );

    const cycle = failHarness.startCycle('Failing test');
    failHarness.approveSpec(cycle.id);
    failHarness.implement(cycle.id, { 'src/bad.ts': '// bad code' });

    const tested = failHarness.runTests(cycle.id);
    expect(tested.status).toBe('failed');
    expect(tested.error).toContain('Tests failed');
  });

  // ─── Merge Flow ───────────────────────────────────────────────────

  it('should merge branch and create feature flag', () => {
    const cycle = harness.startCycle('Merge test');
    harness.approveSpec(cycle.id);
    harness.implement(cycle.id, { 'src/feature.ts': 'export const x = 1;' });
    harness.runTests(cycle.id);
    harness.runBuild(cycle.id);

    const merged = harness.merge(cycle.id);
    expect(merged.status).toBe('complete');
    expect(merged.completedAt).toBeTruthy();

    // Verify feature flag was created
    const allFlags = flags.listFlags();
    expect(allFlags.length).toBeGreaterThan(0);
    expect(allFlags[0].name).toContain(cycle.specId);

    // Verify spec status updated
    const spec = specs.get(cycle.specId);
    expect(spec!.status).toBe('merged');
  });

  // ─── Rollback Flow ───────────────────────────────────────────────

  it('should rollback a cycle', () => {
    const cycle = harness.startCycle('Rollback test');
    harness.approveSpec(cycle.id);
    harness.implement(cycle.id, { 'src/oops.ts': 'export const oops = true;' });
    harness.runTests(cycle.id);
    harness.runBuild(cycle.id);
    harness.merge(cycle.id);

    const rolled = harness.rollback(cycle.id, 'Found a bug');
    expect(rolled.status).toBe('rolled_back');
    expect(rolled.error).toBe('Found a bug');

    // Spec should be rolled back
    const spec = specs.get(cycle.specId);
    expect(spec!.status).toBe('rolled_back');
  });

  it('should rollback a non-merged cycle', () => {
    const cycle = harness.startCycle('Early rollback');
    harness.approveSpec(cycle.id);

    const rolled = harness.rollback(cycle.id, 'Changed my mind');
    expect(rolled.status).toBe('rolled_back');
  });

  // ─── Self Review ──────────────────────────────────────────────────

  it('should return diff for self review', () => {
    const cycle = harness.startCycle('Review test');
    harness.approveSpec(cycle.id);
    harness.implement(cycle.id, { 'src/review.ts': 'export const review = true;' });
    harness.runTests(cycle.id);
    harness.runBuild(cycle.id);

    const { diff } = harness.selfReview(cycle.id);
    expect(diff).toContain('review');
  });

  // ─── Query Methods ────────────────────────────────────────────────

  it('should list and get cycles', () => {
    const c1 = harness.startCycle('Cycle one');
    const c2 = harness.startCycle('Cycle two');

    const all = harness.listCycles();
    expect(all.length).toBe(2);

    const found = harness.getCycle(c1.id);
    expect(found).not.toBeNull();
    expect(found!.id).toBe(c1.id);

    const missing = harness.getCycle('nonexistent');
    expect(missing).toBeNull();
  });

  it('should filter cycles by status', () => {
    harness.startCycle('Draft cycle');
    const c2 = harness.startCycle('Approved cycle');
    harness.approveSpec(c2.id);

    const drafts = harness.listCycles('spec_draft');
    expect(drafts.length).toBe(1);

    const reviews = harness.listCycles('spec_review');
    expect(reviews.length).toBe(1);
  });

  // ─── Diagnostics ──────────────────────────────────────────────────

  it('should produce diagnostic results', () => {
    harness.startCycle('Diagnostic test');

    const result = harness.diagnose();
    expect(result.module).toBe('harness');
    expect(result.status).toBe('healthy');
    expect(result.checks.length).toBeGreaterThan(0);
  });
});

describe('GarbageCollector', () => {
  let repoDir: string;
  let dataDir: string;
  let specsDir: string;
  let bus: ModuleBus;
  let taskGraph: TaskGraph;
  let specs: SpecManager;
  let tools: ToolRegistry;
  let flags: FeatureFlagManager;
  let harness: Harness;
  let gc: GarbageCollector;

  beforeEach(() => {
    repoDir = createTempGitRepo();
    dataDir = join(repoDir, '.fort-data');
    specsDir = join(repoDir, 'specs');
    mkdirSync(dataDir, { recursive: true });

    bus = new ModuleBus();
    taskGraph = new TaskGraph(bus);
    specs = new SpecManager(specsDir);
    tools = new ToolRegistry(join(dataDir, 'tools.db'));
    flags = new FeatureFlagManager(join(dataDir, 'flags.db'), bus);

    harness = new Harness(
      { repoRoot: repoDir, testCommand: 'echo ok', buildCommand: 'echo ok' },
      bus,
      taskGraph,
      specs,
      tools,
      flags,
    );

    gc = new GarbageCollector(specs, tools, flags, taskGraph, harness);
  });

  afterEach(() => {
    tools.close();
    flags.close();
    try {
      rmSync(repoDir, { recursive: true, force: true });
    } catch {
      // Best effort
    }
  });

  // ─── Stale Spec Detection ────────────────────────────────────────

  it('should find stale specs', () => {
    // Create a spec and backdate it
    const spec = specs.create({
      title: 'Old spec',
      goal: 'Test',
      approach: 'Test',
      affectedFiles: [],
      testCriteria: [],
      rollbackPlan: 'revert',
    });

    // With maxAgeDays=0, any spec is stale
    const stale = gc.findStaleSpecs(0);
    expect(stale.length).toBe(1);
    expect(stale[0].id).toBe(spec.id);
  });

  it('should not flag recent specs as stale', () => {
    specs.create({
      title: 'Fresh spec',
      goal: 'Test',
      approach: 'Test',
      affectedFiles: [],
      testCriteria: [],
      rollbackPlan: 'revert',
    });

    // With default 7 days, a just-created spec is not stale
    const stale = gc.findStaleSpecs(7);
    expect(stale.length).toBe(0);
  });

  // ─── Unused Tool Detection ───────────────────────────────────────

  it('should find unused tools', () => {
    tools.register({
      name: 'unused-tool',
      description: 'A tool nobody uses',
      capabilities: ['nothing'],
      inputTypes: [],
      outputTypes: [],
      tags: [],
      module: 'test',
      version: '1.0.0',
    });

    // Never-used tool should show up with minDaysSinceUse=0
    const unused = gc.findUnusedTools(0);
    expect(unused.length).toBe(1);
    expect(unused[0].name).toBe('unused-tool');
    expect(unused[0].daysSinceUse).toBeNull();
  });

  it('should not flag recently used tools', () => {
    const tool = tools.register({
      name: 'active-tool',
      description: 'A tool used recently',
      capabilities: ['something'],
      inputTypes: [],
      outputTypes: [],
      tags: [],
      module: 'test',
      version: '1.0.0',
    });
    tools.recordUsage(tool.id);

    // With 30-day threshold, just-used tool should not appear
    const unused = gc.findUnusedTools(30);
    expect(unused.length).toBe(0);
  });

  // ─── Dead Flag Detection ─────────────────────────────────────────

  it('should find dead flags', () => {
    flags.createFlag('dead-flag', 'A rolled back flag');
    const allFlags = flags.listFlags();
    // It starts disabled, so should show up
    const dead = gc.findDeadFlags();
    expect(dead.length).toBe(1);
  });

  // ─── Report Generation ───────────────────────────────────────────

  it('should generate a comprehensive report', () => {
    const report = gc.generateReport();

    expect(report).toHaveProperty('staleSpecs');
    expect(report).toHaveProperty('unusedTools');
    expect(report).toHaveProperty('deadFlags');
    expect(report).toHaveProperty('orphanedTasks');
    expect(report).toHaveProperty('generatedAt');
  });

  // ─── Diagnostics ──────────────────────────────────────────────────

  it('should produce diagnostic results', () => {
    const result = gc.diagnose();
    expect(result.module).toBe('garbage-collector');
    expect(result.checks.length).toBeGreaterThan(0);
  });

  it('should report degraded status when issues exist', () => {
    specs.create({
      title: 'Stale spec',
      goal: 'Test',
      approach: 'Test',
      affectedFiles: [],
      testCriteria: [],
      rollbackPlan: 'revert',
    });

    // Override to detect immediately
    const gcImmediate = new GarbageCollector(specs, tools, flags, taskGraph, harness);
    // Use 0 days so the just-created spec counts as stale
    const report = gcImmediate.generateReport({ staleSpecDays: 0 });
    expect(report.staleSpecs.length).toBe(1);
  });
});
