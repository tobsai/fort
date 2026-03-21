import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import { ModuleBus } from '../module-bus/index.js';
import { MemoryManager } from '../memory/index.js';
import { TaskGraph } from '../task-graph/index.js';
import { Scheduler } from '../scheduler/index.js';
import { RoutineManager } from '../routines/index.js';

describe('RoutineManager', () => {
  let tmpDir: string;
  let memory: MemoryManager;
  let bus: ModuleBus;
  let taskGraph: TaskGraph;
  let scheduler: Scheduler;
  let routines: RoutineManager;

  function setup() {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-'));
    bus = new ModuleBus();
    memory = new MemoryManager(join(tmpDir, 'memory.db'), bus);
    taskGraph = new TaskGraph(bus);
    scheduler = new Scheduler(bus, taskGraph);
    routines = new RoutineManager(memory, bus, scheduler, taskGraph);
    return { routines, memory, bus, taskGraph, scheduler };
  }

  afterEach(() => {
    scheduler?.shutdown();
    memory?.close();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should add a routine and store in memory graph', () => {
    setup();
    const r = routines.addRoutine({
      name: 'Daily Briefing',
      description: 'Morning briefing at 7 AM',
      schedule: '0 7 * * *',
      steps: [
        { id: 'step1', action: 'fetch_news', params: { count: 5 }, onError: 'skip' },
        { id: 'step2', action: 'summarize', params: {} },
      ],
    });

    expect(r.id).toBeTruthy();
    expect(r.name).toBe('Daily Briefing');
    expect(r.schedule).toBe('0 7 * * *');
    expect(r.steps).toHaveLength(2);
    expect(r.enabled).toBe(true);

    // Verify stored in memory graph
    const result = memory.search({ nodeType: 'routine' });
    expect(result.nodes).toHaveLength(1);
    expect(result.nodes[0].label).toBe('Daily Briefing');
  });

  it('should register routine with the scheduler', () => {
    setup();
    routines.addRoutine({
      name: 'Test Routine',
      description: 'Test',
      schedule: '0 7 * * *',
      steps: [],
    });

    // Scheduler should have a routine registered
    const schedulerRoutines = scheduler.listRoutines();
    expect(schedulerRoutines.length).toBeGreaterThanOrEqual(1);
    expect(schedulerRoutines.some((r) => r.name === 'Test Routine')).toBe(true);
  });

  it('should list all routines', () => {
    setup();
    routines.addRoutine({
      name: 'Routine A',
      description: 'A',
      schedule: '0 7 * * *',
      steps: [],
    });
    routines.addRoutine({
      name: 'Routine B',
      description: 'B',
      schedule: '0 12 * * *',
      steps: [],
    });

    const list = routines.listRoutines();
    expect(list).toHaveLength(2);
  });

  it('should get a single routine by ID', () => {
    setup();
    const r = routines.addRoutine({
      name: 'Fetch Routine',
      description: 'Test',
      schedule: '0 7 * * *',
      steps: [],
    });

    const fetched = routines.getRoutine(r.id);
    expect(fetched).not.toBeNull();
    expect(fetched!.name).toBe('Fetch Routine');
  });

  it('should manually execute a routine and create a task', async () => {
    setup();
    const r = routines.addRoutine({
      name: 'Manual Run',
      description: 'Run manually',
      schedule: '0 7 * * *',
      steps: [
        { id: 's1', action: 'test_action', params: { key: 'value' } },
      ],
    });

    const execution = await routines.executeRoutine(r.id);

    expect(execution.routineId).toBe(r.id);
    expect(execution.result).toBe('success');
    expect(execution.taskId).toBeTruthy();
    expect(execution.completedAt).toBeTruthy();

    // Verify task was created in TaskGraph
    const task = taskGraph.getTask(execution.taskId);
    expect(task.title).toBe('Routine: Manual Run');
    expect(task.source).toBe('scheduled_routine');
    expect(task.status).toBe('completed');
  });

  it('should record execution history', async () => {
    setup();
    const r = routines.addRoutine({
      name: 'History Test',
      description: 'Test history',
      schedule: '0 7 * * *',
      steps: [],
    });

    await routines.executeRoutine(r.id);
    await routines.executeRoutine(r.id);

    const history = routines.getHistory(r.id);
    expect(history).toHaveLength(2);
    expect(history[0].result).toBe('success');
  });

  it('should disable and enable a routine', () => {
    setup();
    const r = routines.addRoutine({
      name: 'Toggle Routine',
      description: 'Test',
      schedule: '0 7 * * *',
      steps: [],
    });

    routines.disableRoutine(r.id);
    const disabled = routines.getRoutine(r.id);
    expect(disabled!.enabled).toBe(false);

    routines.enableRoutine(r.id);
    const enabled = routines.getRoutine(r.id);
    expect(enabled!.enabled).toBe(true);
  });

  it('should remove a routine', () => {
    setup();
    const r = routines.addRoutine({
      name: 'Remove Me',
      description: 'Test',
      schedule: '0 7 * * *',
      steps: [],
    });

    const removed = routines.removeRoutine(r.id);
    expect(removed).toBe(true);

    const list = routines.listRoutines();
    expect(list).toHaveLength(0);
  });

  it('should support behaviors attached to routines', () => {
    setup();
    const r = routines.addRoutine({
      name: 'With Behaviors',
      description: 'Routine with behaviors',
      schedule: '0 7 * * *',
      steps: [],
      behaviors: ['behavior-1', 'behavior-2'],
    });

    expect(r.behaviors).toEqual(['behavior-1', 'behavior-2']);

    const fetched = routines.getRoutine(r.id);
    expect(fetched!.behaviors).toEqual(['behavior-1', 'behavior-2']);
  });

  it('should update lastRun after execution', async () => {
    setup();
    const r = routines.addRoutine({
      name: 'Last Run Test',
      description: 'Test',
      schedule: '0 7 * * *',
      steps: [],
    });

    expect(routines.getRoutine(r.id)!.lastRun).toBeUndefined();

    await routines.executeRoutine(r.id);

    const updated = routines.getRoutine(r.id);
    expect(updated!.lastRun).toBeTruthy();
    expect(updated!.lastResult).toBe('success');
  });

  it('should emit events on bus', async () => {
    setup();
    const events: string[] = [];
    bus.subscribe('routine.added', () => { events.push('added'); });
    bus.subscribe('routine.executed', () => { events.push('executed'); });

    const r = routines.addRoutine({
      name: 'Event Test',
      description: 'Test',
      schedule: '0 7 * * *',
      steps: [],
    });

    await routines.executeRoutine(r.id);

    expect(events).toContain('added');
    expect(events).toContain('executed');
  });

  it('should return a healthy diagnostic', () => {
    setup();
    routines.addRoutine({
      name: 'Diag Test',
      description: 'Test',
      schedule: '0 7 * * *',
      steps: [],
    });

    const diag = routines.diagnose();
    expect(diag.module).toBe('routines');
    expect(diag.status).toBe('healthy');
    expect(diag.checks.length).toBeGreaterThanOrEqual(1);
  });

  it('should limit execution history', async () => {
    setup();
    const r = routines.addRoutine({
      name: 'Limit Test',
      description: 'Test',
      schedule: '0 7 * * *',
      steps: [],
    });

    for (let i = 0; i < 5; i++) {
      await routines.executeRoutine(r.id);
    }

    const limited = routines.getHistory(r.id, 3);
    expect(limited).toHaveLength(3);
  });
});
