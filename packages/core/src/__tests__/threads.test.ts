import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import { ThreadManager } from '../threads/index.js';
import { ModuleBus } from '../module-bus/index.js';
import { TaskGraph } from '../task-graph/index.js';

describe('ThreadManager', () => {
  let tmpDir: string;
  let bus: ModuleBus;
  let taskGraph: TaskGraph;
  let threads: ThreadManager;

  function setup() {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-threads-'));
    bus = new ModuleBus();
    taskGraph = new TaskGraph(bus);
    threads = new ThreadManager(join(tmpDir, 'threads.db'), bus, taskGraph);
    return threads;
  }

  afterEach(() => {
    threads?.close();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should create a thread backed by a task', () => {
    setup();

    const thread = threads.createThread({ name: 'Test Thread', description: 'A test' });

    expect(thread.id).toBeDefined();
    expect(thread.name).toBe('Test Thread');
    expect(thread.description).toBe('A test');
    expect(thread.status).toBe('active');
    expect(thread.taskId).toBeDefined();

    // Verify backing task exists and is in_progress
    const task = taskGraph.getTask(thread.taskId);
    expect(task).toBeDefined();
    expect(task.status).toBe('in_progress');
    expect(task.title).toBe('Thread: Test Thread');
  });

  it('should add messages and retrieve history', () => {
    setup();

    const thread = threads.createThread({ name: 'Chat' });

    threads.addMessage(thread.id, { role: 'user', content: 'Hello' });
    threads.addMessage(thread.id, { role: 'agent', content: 'Hi there', agentId: 'orchestrator' });
    threads.addMessage(thread.id, { role: 'system', content: 'Thread started' });

    const messages = threads.getMessages(thread.id);
    expect(messages).toHaveLength(3);
    expect(messages[0].role).toBe('user');
    expect(messages[0].content).toBe('Hello');
    expect(messages[1].role).toBe('agent');
    expect(messages[1].agentId).toBe('orchestrator');
    expect(messages[2].role).toBe('system');
  });

  it('should pause thread and set task to blocked', () => {
    setup();

    const thread = threads.createThread({ name: 'Pausable' });
    const paused = threads.pauseThread(thread.id);

    expect(paused.status).toBe('paused');

    const task = taskGraph.getTask(thread.taskId);
    expect(task.status).toBe('blocked');
  });

  it('should resume thread and set task to in_progress', () => {
    setup();

    const thread = threads.createThread({ name: 'Resumable' });
    threads.pauseThread(thread.id);
    const resumed = threads.resumeThread(thread.id);

    expect(resumed.status).toBe('active');

    const task = taskGraph.getTask(thread.taskId);
    expect(task.status).toBe('in_progress');
  });

  it('should resolve thread and complete the task', () => {
    setup();

    const thread = threads.createThread({ name: 'Resolvable' });
    const resolved = threads.resolveThread(thread.id);

    expect(resolved.status).toBe('resolved');

    const task = taskGraph.getTask(thread.taskId);
    expect(task.status).toBe('completed');
  });

  it('should fork a thread with lineage', () => {
    setup();

    const parent = threads.createThread({ name: 'Parent', projectTag: 'fort' });
    threads.addMessage(parent.id, { role: 'user', content: 'msg 1' });
    threads.addMessage(parent.id, { role: 'agent', content: 'msg 2' });

    const forked = threads.forkThread(parent.id, { name: 'Fork' });

    expect(forked.parentThreadId).toBe(parent.id);
    expect(forked.name).toBe('Fork');
    expect(forked.projectTag).toBe('fort');
    expect(forked.status).toBe('active');

    // Forked thread has its own task
    expect(forked.taskId).not.toBe(parent.taskId);
  });

  it('should fork a thread and copy messages up to fromMessageId', () => {
    setup();

    const parent = threads.createThread({ name: 'Parent' });
    const msg1 = threads.addMessage(parent.id, { role: 'user', content: 'first' });
    const msg2 = threads.addMessage(parent.id, { role: 'agent', content: 'second' });
    threads.addMessage(parent.id, { role: 'user', content: 'third' });

    const forked = threads.forkThread(parent.id, { name: 'Fork at msg2', fromMessageId: msg2.id });

    const forkedMessages = threads.getMessages(forked.id);
    expect(forkedMessages).toHaveLength(2);
    expect(forkedMessages[0].content).toBe('first');
    expect(forkedMessages[1].content).toBe('second');
  });

  it('should list threads with status filter', () => {
    setup();

    threads.createThread({ name: 'Active 1' });
    threads.createThread({ name: 'Active 2' });
    const t3 = threads.createThread({ name: 'To Pause' });
    threads.pauseThread(t3.id);

    const active = threads.listThreads({ status: 'active' });
    expect(active).toHaveLength(2);

    const paused = threads.listThreads({ status: 'paused' });
    expect(paused).toHaveLength(1);
    expect(paused[0].name).toBe('To Pause');
  });

  it('should list threads with project tag filter', () => {
    setup();

    threads.createThread({ name: 'Fort thread', projectTag: 'fort' });
    threads.createThread({ name: 'Other thread', projectTag: 'other' });
    threads.createThread({ name: 'No tag' });

    const fortThreads = threads.listThreads({ projectTag: 'fort' });
    expect(fortThreads).toHaveLength(1);
    expect(fortThreads[0].name).toBe('Fort thread');
  });

  it('should list threads with agent filter', () => {
    setup();

    threads.createThread({ name: 'Agent thread', assignedAgent: 'orchestrator' });
    threads.createThread({ name: 'Other thread', assignedAgent: 'memory-agent' });

    const agentThreads = threads.listThreads({ agentId: 'orchestrator' });
    expect(agentThreads).toHaveLength(1);
    expect(agentThreads[0].name).toBe('Agent thread');
  });

  it('should search across thread names and messages', () => {
    setup();

    const t1 = threads.createThread({ name: 'Debugging issue' });
    const t2 = threads.createThread({ name: 'Feature work' });
    threads.addMessage(t2.id, { role: 'user', content: 'We need to debug this too' });

    const results = threads.searchThreads('debug');
    expect(results.length).toBeGreaterThanOrEqual(2);

    const ids = results.map((r) => r.id);
    expect(ids).toContain(t1.id);
    expect(ids).toContain(t2.id);
  });

  it('should create cross-references between threads', () => {
    setup();

    const t1 = threads.createThread({ name: 'Thread A' });
    const t2 = threads.createThread({ name: 'Thread B' });

    const ref = threads.crossReference(t1.id, t2.id, 'Related discussion');

    expect(ref.fromThreadId).toBe(t1.id);
    expect(ref.toThreadId).toBe(t2.id);
    expect(ref.note).toBe('Related discussion');
  });

  it('should get all references for a thread', () => {
    setup();

    const t1 = threads.createThread({ name: 'Thread A' });
    const t2 = threads.createThread({ name: 'Thread B' });
    const t3 = threads.createThread({ name: 'Thread C' });

    threads.crossReference(t1.id, t2.id, 'ref 1');
    threads.crossReference(t3.id, t1.id, 'ref 2');

    const refs = threads.getReferences(t1.id);
    expect(refs).toHaveLength(2);
  });

  it('should support parent thread relationships', () => {
    setup();

    const parent = threads.createThread({ name: 'Parent' });
    const child = threads.createThread({ name: 'Child', parentThreadId: parent.id });

    expect(child.parentThreadId).toBe(parent.id);

    const retrieved = threads.getThread(child.id);
    expect(retrieved.parentThreadId).toBe(parent.id);
  });

  it('should get messages with limit', () => {
    setup();

    const thread = threads.createThread({ name: 'Long thread' });
    for (let i = 0; i < 10; i++) {
      threads.addMessage(thread.id, { role: 'user', content: `Message ${i}` });
    }

    const limited = threads.getMessages(thread.id, { limit: 3 });
    expect(limited).toHaveLength(3);
    expect(limited[0].content).toBe('Message 0');
  });

  it('should publish bus events for thread lifecycle', () => {
    setup();

    const events: string[] = [];
    bus.subscribe('thread.created', () => { events.push('created'); });
    bus.subscribe('thread.message', () => { events.push('message'); });
    bus.subscribe('thread.paused', () => { events.push('paused'); });
    bus.subscribe('thread.resumed', () => { events.push('resumed'); });
    bus.subscribe('thread.resolved', () => { events.push('resolved'); });
    bus.subscribe('thread.forked', () => { events.push('forked'); });

    const thread = threads.createThread({ name: 'Events test' });
    threads.addMessage(thread.id, { role: 'user', content: 'test' });
    threads.pauseThread(thread.id);
    threads.resumeThread(thread.id);
    threads.forkThread(thread.id, { name: 'Fork' });
    threads.resolveThread(thread.id);

    // thread.created fires for both the main thread and the forked thread
    expect(events).toContain('created');
    expect(events).toContain('message');
    expect(events).toContain('paused');
    expect(events).toContain('resumed');
    expect(events).toContain('forked');
    expect(events).toContain('resolved');
  });

  it('should return correct diagnostics', () => {
    setup();

    threads.createThread({ name: 'Diag thread' });
    threads.addMessage(
      threads.listThreads()[0].id,
      { role: 'user', content: 'hello' },
    );

    const diag = threads.diagnose();
    expect(diag.module).toBe('threads');
    expect(diag.status).toBe('healthy');
    expect(diag.checks.length).toBeGreaterThanOrEqual(3);
    expect(diag.checks[0].name).toBe('Thread database');
    expect(diag.checks[0].passed).toBe(true);
    expect(diag.checks[0].message).toContain('1 total threads');
  });

  it('should throw when getting non-existent thread', () => {
    setup();

    expect(() => threads.getThread('nonexistent')).toThrow('Thread not found');
  });
});
