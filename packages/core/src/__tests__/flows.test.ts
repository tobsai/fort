import { describe, it, expect, beforeEach } from 'vitest';
import { ModuleBus } from '../module-bus/index.js';
import { TaskGraph } from '../task-graph/index.js';
import { FlowEngine } from '../flows/index.js';
import type { FlowDefinition } from '../types.js';

describe('FlowEngine', () => {
  let bus: ModuleBus;
  let taskGraph: TaskGraph;
  let engine: FlowEngine;

  beforeEach(() => {
    bus = new ModuleBus();
    taskGraph = new TaskGraph(bus);
    engine = new FlowEngine(bus, taskGraph);
  });

  // ─── Registration ──────────────────────────────────────────────

  it('should register a flow definition', () => {
    const flow: FlowDefinition = {
      id: 'test-flow',
      name: 'Test Flow',
      description: 'A simple test flow',
      steps: [
        { id: 'step1', name: 'First Step', type: 'action', tool: 'greet', params: { name: 'world' } },
      ],
      onError: 'abort',
    };

    engine.registerFlow(flow);
    const listed = engine.listFlows();
    expect(listed).toHaveLength(1);
    expect(listed[0].id).toBe('test-flow');
  });

  it('should reject flow definitions missing required fields', () => {
    expect(() => engine.registerFlow({ id: '', name: 'X', description: '', steps: [], onError: 'abort' })).toThrow();
    expect(() => engine.registerFlow({ id: 'x', name: '', description: '', steps: [], onError: 'abort' })).toThrow();
  });

  // ─── Simple multi-step execution ──────────────────────────────

  it('should execute a multi-step flow with context passing', async () => {
    engine.registerAction('add', async (params, context) => {
      const a = (params.a ?? context.a) as number;
      const b = (params.b ?? context.b) as number;
      return a + b;
    });

    engine.registerAction('multiply', async (params, context) => {
      const value = context['step1'] as number;
      const factor = params.factor as number;
      return value * factor;
    });

    const flow: FlowDefinition = {
      id: 'math-flow',
      name: 'Math Flow',
      description: 'Two-step math',
      steps: [
        { id: 'step1', name: 'Add', type: 'action', tool: 'add', params: { a: 2, b: 3 } },
        { id: 'step2', name: 'Multiply', type: 'action', tool: 'multiply', params: { factor: 4 } },
      ],
      onError: 'abort',
    };

    engine.registerFlow(flow);
    const execution = await engine.executeFlow('math-flow');

    expect(execution.status).toBe('completed');
    expect(execution.stepResults).toHaveLength(2);
    expect(execution.stepResults[0].output).toBe(5);
    expect(execution.stepResults[1].output).toBe(20);
    expect(execution.context['step1']).toBe(5);
    expect(execution.context['step2']).toBe(20);
  });

  // ─── Creates task in TaskGraph ─────────────────────────────────

  it('should create a task in TaskGraph for each execution', async () => {
    engine.registerAction('noop', async () => 'done');

    engine.registerFlow({
      id: 'task-flow',
      name: 'Task Flow',
      description: 'Creates a task',
      steps: [{ id: 's1', name: 'Noop', type: 'action', tool: 'noop' }],
      onError: 'abort',
    });

    const execution = await engine.executeFlow('task-flow');
    const task = taskGraph.getTask(execution.taskId);

    expect(task).toBeDefined();
    expect(task.title).toBe('Flow: Task Flow');
    expect(task.status).toBe('completed');
    expect(task.source).toBe('background');
  });

  // ─── Condition branching ──────────────────────────────────────

  it('should follow the then branch when condition is true', async () => {
    engine.registerAction('result', async (params) => params.value);

    engine.registerFlow({
      id: 'cond-flow',
      name: 'Condition Flow',
      description: 'Tests condition branching',
      steps: [
        {
          id: 'check',
          name: 'Check Value',
          type: 'condition',
          expression: 'inputValue > 10',
          thenSteps: [
            { id: 'then-step', name: 'Then', type: 'action', tool: 'result', params: { value: 'big' } },
          ],
          elseSteps: [
            { id: 'else-step', name: 'Else', type: 'action', tool: 'result', params: { value: 'small' } },
          ],
        },
      ],
      onError: 'abort',
    });

    const exec1 = await engine.executeFlow('cond-flow', { inputValue: 20 });
    expect(exec1.status).toBe('completed');
    expect(exec1.context['then-step']).toBe('big');
    expect(exec1.context['else-step']).toBeUndefined();

    const exec2 = await engine.executeFlow('cond-flow', { inputValue: 5 });
    expect(exec2.status).toBe('completed');
    expect(exec2.context['else-step']).toBe('small');
    expect(exec2.context['then-step']).toBeUndefined();
  });

  // ─── Transform steps ──────────────────────────────────────────

  it('should transform data between steps', async () => {
    engine.registerAction('fetch', async () => [1, 2, 3, 4, 5]);

    engine.registerFlow({
      id: 'transform-flow',
      name: 'Transform Flow',
      description: 'Tests transform',
      steps: [
        { id: 'data', name: 'Fetch Data', type: 'action', tool: 'fetch' },
        { id: 'transformed', name: 'Double Values', type: 'transform', expression: 'data.map(x => x * 2)' },
      ],
      onError: 'abort',
    });

    const execution = await engine.executeFlow('transform-flow');
    expect(execution.status).toBe('completed');
    expect(execution.context['transformed']).toEqual([2, 4, 6, 8, 10]);
  });

  // ─── Error handling: abort ─────────────────────────────────────

  it('should abort flow on step failure when onError is abort', async () => {
    engine.registerAction('fail', async () => { throw new Error('boom'); });
    engine.registerAction('noop', async () => 'should not run');

    engine.registerFlow({
      id: 'abort-flow',
      name: 'Abort Flow',
      description: 'Tests abort',
      steps: [
        { id: 's1', name: 'Fail', type: 'action', tool: 'fail' },
        { id: 's2', name: 'After', type: 'action', tool: 'noop' },
      ],
      onError: 'abort',
    });

    const execution = await engine.executeFlow('abort-flow');
    expect(execution.status).toBe('aborted');
    expect(execution.error).toContain('boom');
    expect(execution.stepResults).toHaveLength(1);
    expect(execution.stepResults[0].status).toBe('failed');

    // Task should be marked failed
    const task = taskGraph.getTask(execution.taskId);
    expect(task.status).toBe('failed');
  });

  // ─── Error handling: skip ──────────────────────────────────────

  it('should skip failed step and continue when onError is skip', async () => {
    engine.registerAction('fail', async () => { throw new Error('oops'); });
    engine.registerAction('ok', async () => 'success');

    engine.registerFlow({
      id: 'skip-flow',
      name: 'Skip Flow',
      description: 'Tests skip',
      steps: [
        { id: 's1', name: 'Fail', type: 'action', tool: 'fail' },
        { id: 's2', name: 'Continue', type: 'action', tool: 'ok' },
      ],
      onError: 'skip',
    });

    const execution = await engine.executeFlow('skip-flow');
    expect(execution.status).toBe('completed');
    expect(execution.stepResults).toHaveLength(2);
    expect(execution.stepResults[0].status).toBe('skipped');
    expect(execution.stepResults[1].status).toBe('completed');
    expect(execution.stepResults[1].output).toBe('success');
  });

  // ─── Parallel execution ───────────────────────────────────────

  it('should execute parallel branches concurrently', async () => {
    const order: string[] = [];
    engine.registerAction('track', async (params) => {
      order.push(params.label as string);
      return params.label;
    });

    engine.registerFlow({
      id: 'parallel-flow',
      name: 'Parallel Flow',
      description: 'Tests parallel',
      steps: [
        {
          id: 'par',
          name: 'Parallel',
          type: 'parallel',
          branches: [
            [{ id: 'b1s1', name: 'Branch 1', type: 'action', tool: 'track', params: { label: 'a' } }],
            [{ id: 'b2s1', name: 'Branch 2', type: 'action', tool: 'track', params: { label: 'b' } }],
          ],
        },
      ],
      onError: 'abort',
    });

    const execution = await engine.executeFlow('parallel-flow');
    expect(execution.status).toBe('completed');
    // Both branches should have executed
    expect(order).toContain('a');
    expect(order).toContain('b');
    expect(execution.context['b1s1']).toBe('a');
    expect(execution.context['b2s1']).toBe('b');
  });

  // ─── Notify step ──────────────────────────────────────────────

  it('should publish events for notify steps', async () => {
    const events: unknown[] = [];
    bus.subscribe('custom.notification', (event) => {
      events.push(event.payload);
    });

    engine.registerFlow({
      id: 'notify-flow',
      name: 'Notify Flow',
      description: 'Tests notify',
      steps: [
        { id: 'n1', name: 'Notify', type: 'notify', eventType: 'custom.notification', payload: { msg: 'hello' } },
      ],
      onError: 'abort',
    });

    await engine.executeFlow('notify-flow');
    expect(events).toHaveLength(1);
    expect((events[0] as Record<string, unknown>).msg).toBe('hello');
  });

  // ─── LLM placeholder step ────────────────────────────────────

  it('should handle llm steps as a placeholder', async () => {
    const llmEvents: unknown[] = [];
    bus.subscribe('flow.llm_requested', (event) => {
      llmEvents.push(event.payload);
    });

    engine.registerFlow({
      id: 'llm-flow',
      name: 'LLM Flow',
      description: 'Tests LLM placeholder',
      steps: [
        { id: 'llm1', name: 'Think', type: 'llm', prompt: 'Summarize the data' },
      ],
      onError: 'abort',
    });

    const execution = await engine.executeFlow('llm-flow');
    expect(execution.status).toBe('completed');
    expect(execution.stepResults[0].output).toEqual({ placeholder: true, prompt: 'Summarize the data' });
    expect(llmEvents).toHaveLength(1);
  });

  // ─── Flow status query ────────────────────────────────────────

  it('should track and retrieve execution status', async () => {
    engine.registerAction('noop', async () => 'ok');

    engine.registerFlow({
      id: 'status-flow',
      name: 'Status Flow',
      description: 'Tests status',
      steps: [{ id: 's1', name: 'Step', type: 'action', tool: 'noop' }],
      onError: 'abort',
    });

    const execution = await engine.executeFlow('status-flow');
    const status = engine.getFlowStatus(execution.id);

    expect(status.id).toBe(execution.id);
    expect(status.status).toBe('completed');
    expect(status.completedAt).toBeDefined();
    expect(status.flowId).toBe('status-flow');
  });

  it('should throw when querying non-existent execution', () => {
    expect(() => engine.getFlowStatus('non-existent')).toThrow('Flow execution not found');
  });

  // ─── Flow listing ─────────────────────────────────────────────

  it('should list all registered flows', () => {
    engine.registerFlow({
      id: 'f1', name: 'Flow 1', description: 'First', steps: [
        { id: 's1', name: 'S', type: 'notify', eventType: 'test' },
      ], onError: 'abort',
    });
    engine.registerFlow({
      id: 'f2', name: 'Flow 2', description: 'Second', steps: [
        { id: 's1', name: 'S', type: 'notify', eventType: 'test' },
      ], onError: 'skip',
    });

    const flows = engine.listFlows();
    expect(flows).toHaveLength(2);
    expect(flows.map((f) => f.id)).toContain('f1');
    expect(flows.map((f) => f.id)).toContain('f2');
  });

  // ─── Execution listing ────────────────────────────────────────

  it('should list executions filtered by flowId', async () => {
    engine.registerAction('noop', async () => 'ok');

    engine.registerFlow({
      id: 'ea', name: 'A', description: '', steps: [
        { id: 's1', name: 'S', type: 'action', tool: 'noop' },
      ], onError: 'abort',
    });
    engine.registerFlow({
      id: 'eb', name: 'B', description: '', steps: [
        { id: 's1', name: 'S', type: 'action', tool: 'noop' },
      ], onError: 'abort',
    });

    await engine.executeFlow('ea');
    await engine.executeFlow('ea');
    await engine.executeFlow('eb');

    expect(engine.listExecutions()).toHaveLength(3);
    expect(engine.listExecutions('ea')).toHaveLength(2);
    expect(engine.listExecutions('eb')).toHaveLength(1);
  });

  // ─── Diagnostics ──────────────────────────────────────────────

  it('should return healthy diagnostic when no failures', () => {
    const result = engine.diagnose();
    expect(result.module).toBe('flow-engine');
    expect(result.status).toBe('healthy');
    expect(result.checks.every((c) => c.passed)).toBe(true);
  });

  it('should return degraded diagnostic when executions have failed', async () => {
    engine.registerAction('fail', async () => { throw new Error('fail'); });

    engine.registerFlow({
      id: 'bad', name: 'Bad Flow', description: '', steps: [
        { id: 's1', name: 'Fail', type: 'action', tool: 'fail' },
      ], onError: 'abort',
    });

    await engine.executeFlow('bad');

    const result = engine.diagnose();
    expect(result.status).toBe('degraded');
    expect(result.checks.some((c) => !c.passed)).toBe(true);
  });

  // ─── Context initial values ───────────────────────────────────

  it('should pass initial context to flow steps', async () => {
    engine.registerAction('echo', async (params, context) => {
      return context.greeting;
    });

    engine.registerFlow({
      id: 'ctx-flow',
      name: 'Context Flow',
      description: '',
      steps: [{ id: 's1', name: 'Echo', type: 'action', tool: 'echo' }],
      onError: 'abort',
    });

    const execution = await engine.executeFlow('ctx-flow', { greeting: 'hello' });
    expect(execution.stepResults[0].output).toBe('hello');
  });

  // ─── Error: unknown flow ──────────────────────────────────────

  it('should throw when executing a non-existent flow', async () => {
    await expect(engine.executeFlow('nope')).rejects.toThrow('Flow not found');
  });

  // ─── Error: missing action handler ────────────────────────────

  it('should fail when action handler is not registered', async () => {
    engine.registerFlow({
      id: 'missing-handler',
      name: 'Missing',
      description: '',
      steps: [{ id: 's1', name: 'Bad', type: 'action', tool: 'unregistered' }],
      onError: 'abort',
    });

    const execution = await engine.executeFlow('missing-handler');
    expect(execution.status).toBe('aborted');
    expect(execution.error).toContain('No action handler registered');
  });

  // ─── Bus events ───────────────────────────────────────────────

  it('should publish flow lifecycle events on the bus', async () => {
    const events: string[] = [];
    bus.subscribe('flow.registered', () => { events.push('registered'); });
    bus.subscribe('flow.started', () => { events.push('started'); });
    bus.subscribe('flow.completed', () => { events.push('completed'); });

    engine.registerAction('noop', async () => 'ok');
    engine.registerFlow({
      id: 'evt-flow',
      name: 'Event Flow',
      description: '',
      steps: [{ id: 's1', name: 'S', type: 'action', tool: 'noop' }],
      onError: 'abort',
    });

    await engine.executeFlow('evt-flow');

    expect(events).toEqual(['registered', 'started', 'completed']);
  });
});
