import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync, mkdirSync, readFileSync } from 'node:fs';
import { parse as parseYaml } from 'yaml';
import Database from 'better-sqlite3';
import { Fort } from '../fort.js';
import { HatchService } from '../services/hatch.js';
import { GoalsStore } from '../services/goals-store.js';
import { GoalsService } from '../services/goals.js';
import { ModuleBus } from '../module-bus/index.js';
import { MemoryManager } from '../memory/index.js';
import { AgentFactory } from '../agents/hatchery.js';
import { AgentRegistry } from '../agents/index.js';
import { TaskGraph, TaskStore } from '../task-graph/index.js';

describe('HatchService', () => {
  let tmpDir: string;
  let db: InstanceType<typeof Database>;
  let bus: ModuleBus;
  let memory: MemoryManager;
  let store: GoalsStore;
  let goals: GoalsService;
  let factory: AgentFactory;
  let hatch: HatchService;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-hatch-'));
    bus = new ModuleBus();
    db = new (Database as any)(join(tmpDir, 'fort.db'));
    const taskStore = new TaskStore(db);
    taskStore.initSchema();
    const taskGraph = new TaskGraph(bus, taskStore);
    const registry = new AgentRegistry(bus);
    memory = new MemoryManager(join(tmpDir, 'memory.db'), bus);
    store = new GoalsStore(db);
    store.initSchema();
    goals = new GoalsService(store, bus);
    const agentsDir = join(tmpDir, 'agents');
    mkdirSync(agentsDir, { recursive: true });
    factory = new AgentFactory(agentsDir, bus, taskGraph, memory, registry);
    hatch = new HatchService(factory, goals, memory, bus);
  });

  afterEach(() => {
    (db as any).close?.();
    memory.close();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('new user-created agents start un-hatched', () => {
    const agent = factory.create({ name: 'Coach', createdBy: 'user' });
    expect(hatch.isHatching(agent.config.id)).toBe(true);
    expect(hatch.openingMessage(agent.config.id)).toMatch(/Coach here/);
  });

  it('system-created agents skip hatch', () => {
    const triager = factory.create({ name: 'Triager', createdBy: 'system' });
    expect(hatch.isHatching(triager.config.id)).toBe(false);
  });

  it('complete() persists goals, sets hatchedAt, and writes identity.yaml', () => {
    const agent = factory.create({ name: 'Coach', createdBy: 'user' });
    const result = hatch.complete(agent.config.id, [
      { title: 'Ship Fort v1', description: 'Beta to friends by end of March' },
      { title: 'Hire two engineers' },
    ]);
    expect(result.hatchedAt).toBeTruthy();
    expect(result.goalIds).toHaveLength(2);

    // Hatch is now complete
    expect(hatch.isHatching(agent.config.id)).toBe(false);

    // Goals persisted in the store
    const persistedGoals = goals.listForAgent(agent.config.id);
    expect(persistedGoals.map((g) => g.title).sort()).toEqual(
      ['Hire two engineers', 'Ship Fort v1'],
    );
    expect(persistedGoals.every((g) => g.source === 'hatch')).toBe(true);

    // hatchedAt written to identity.yaml on disk
    const yamlPath = join(factory.getAgentDir(agent.config.id), 'identity.yaml');
    const parsed = parseYaml(readFileSync(yamlPath, 'utf-8')) as { hatchedAt?: string };
    expect(parsed.hatchedAt).toBe(result.hatchedAt);
  });

  it('captureProfileFact writes a profile-typed memory node', () => {
    hatch.captureProfileFact('Lives in Wellington, NZ');
    const profile = memory.search({ nodeType: 'profile' });
    expect(profile.nodes.map((n) => n.label)).toContain('Lives in Wellington, NZ');
    expect(profile.nodes[0].source).toBe('hatch');
  });

  it('parseCompletionMarker extracts the goal indices from agent text', () => {
    expect(HatchService.parseCompletionMarker('All set! [HATCH_COMPLETE: 1, 2, 3]'))
      .toEqual([1, 2, 3]);
    expect(HatchService.parseCompletionMarker('Just 1 and 4 then. [HATCH_COMPLETE: 1,4]'))
      .toEqual([1, 4]);
    expect(HatchService.parseCompletionMarker('no marker here')).toBeNull();
    // Empty body → null
    expect(HatchService.parseCompletionMarker('done [HATCH_COMPLETE: ]')).toBeNull();
  });

  it('complete() publishes a hatch:completed event', async () => {
    const events: Array<{ agentId: string; goalIds: string[] }> = [];
    bus.subscribe('hatch:completed', (e) => {
      events.push(e.payload as { agentId: string; goalIds: string[] });
    });
    const agent = factory.create({ name: 'Coach', createdBy: 'user' });
    hatch.complete(agent.config.id, [{ title: 'Ship Fort v1' }]);
    await new Promise((r) => setTimeout(r, 5));
    expect(events).toHaveLength(1);
    expect(events[0].agentId).toBe(agent.config.id);
    expect(events[0].goalIds).toHaveLength(1);
  });
});

describe('Hatch integration via Fort', () => {
  let tmpDir: string;
  let fort: Fort;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-hatch-int-'));
    const specsDir = join(tmpDir, 'specs');
    mkdirSync(specsDir, { recursive: true });
    fort = new Fort({
      dataDir: join(tmpDir, 'data'),
      specsDir,
      agentsDir: join(tmpDir, 'agents'),
    });
  });

  afterEach(async () => {
    if (fort) await fort.stop();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('fort.hatch is wired and reads identity persisted by AgentFactory', async () => {
    await fort.start();
    const agent = fort.agentFactory.create({ name: 'Coach', createdBy: 'user' });
    expect(fort.hatch.isHatching(agent.config.id)).toBe(true);
    const opener = fort.hatch.openingMessage(agent.config.id);
    expect(opener).toMatch(/Coach/);
  });

  it('completing the hatch sets hatchedAt and persists goals queryable via fort.goals', async () => {
    await fort.start();
    const agent = fort.agentFactory.create({ name: 'Coach', createdBy: 'user' });
    fort.hatch.complete(agent.config.id, [
      { title: 'Ship Fort v1' },
      { title: 'Hire two engineers' },
    ]);
    expect(fort.hatch.isHatching(agent.config.id)).toBe(false);
    const goals = fort.goals.listForAgent(agent.config.id);
    expect(goals).toHaveLength(2);
  });
});
