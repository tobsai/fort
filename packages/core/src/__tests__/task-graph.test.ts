import { describe, it, expect } from 'vitest';
import { ModuleBus } from '../module-bus/index.js';
import { TaskGraph } from '../task-graph/index.js';

describe('TaskGraph', () => {
  function setup() {
    const bus = new ModuleBus();
    const graph = new TaskGraph(bus);
    return { bus, graph };
  }

  it('should create tasks with unique IDs', () => {
    const { graph } = setup();
    const t1 = graph.createTask({ title: 'Task 1', source: 'user_chat' });
    const t2 = graph.createTask({ title: 'Task 2', source: 'user_chat' });

    expect(t1.id).toBeTruthy();
    expect(t2.id).toBeTruthy();
    expect(t1.id).not.toBe(t2.id);
    expect(t1.status).toBe('created');
  });

  it('should update task status', () => {
    const { graph } = setup();
    const task = graph.createTask({ title: 'Test', source: 'user_chat' });

    graph.updateStatus(task.id, 'in_progress');
    expect(graph.getTask(task.id).status).toBe('in_progress');

    graph.updateStatus(task.id, 'completed');
    expect(graph.getTask(task.id).status).toBe('completed');
    expect(graph.getTask(task.id).completedAt).toBeTruthy();
  });

  it('should decompose tasks into subtasks', () => {
    const { graph } = setup();
    const parent = graph.createTask({ title: 'Big task', source: 'user_chat' });

    const subtasks = graph.decompose(parent.id, [
      { title: 'Sub A', assignedAgent: 'agent-1' },
      { title: 'Sub B', assignedAgent: 'agent-2' },
    ]);

    expect(subtasks).toHaveLength(2);
    expect(subtasks[0].parentId).toBe(parent.id);
    expect(subtasks[1].parentId).toBe(parent.id);
    expect(graph.getSubtasks(parent.id)).toHaveLength(2);
    expect(graph.getTask(parent.id).status).toBe('in_progress');
  });

  it('should auto-complete parent when all subtasks complete', () => {
    const { graph } = setup();
    const parent = graph.createTask({ title: 'Parent', source: 'user_chat' });
    const subs = graph.decompose(parent.id, [
      { title: 'A' },
      { title: 'B' },
    ]);

    graph.updateStatus(subs[0].id, 'completed');
    expect(graph.getTask(parent.id).status).toBe('in_progress');

    graph.updateStatus(subs[1].id, 'completed');
    expect(graph.getTask(parent.id).status).toBe('completed');
  });

  it('should mark parent as needs_review when subtask fails', () => {
    const { graph } = setup();
    const parent = graph.createTask({ title: 'Parent', source: 'user_chat' });
    const subs = graph.decompose(parent.id, [
      { title: 'A' },
      { title: 'B' },
    ]);

    graph.updateStatus(subs[0].id, 'completed');
    graph.updateStatus(subs[1].id, 'failed', 'Something went wrong');
    expect(graph.getTask(parent.id).status).toBe('needs_review');
  });

  it('should query tasks by status', () => {
    const { graph } = setup();
    graph.createTask({ title: 'A', source: 'user_chat' });
    const b = graph.createTask({ title: 'B', source: 'user_chat' });
    graph.updateStatus(b.id, 'in_progress');

    expect(graph.queryTasks({ status: 'created' })).toHaveLength(1);
    expect(graph.queryTasks({ status: 'in_progress' })).toHaveLength(1);
  });

  it('should search tasks by text', () => {
    const { graph } = setup();
    graph.createTask({ title: 'Schedule dinner with Sarah', source: 'user_chat' });
    graph.createTask({ title: 'Check email', source: 'user_chat' });

    expect(graph.queryTasks({ search: 'dinner' })).toHaveLength(1);
    expect(graph.queryTasks({ search: 'Sarah' })).toHaveLength(1);
    expect(graph.queryTasks({ search: 'nonexistent' })).toHaveLength(0);
  });

  it('should create and manage threads', () => {
    const { graph } = setup();
    const task = graph.createTask({ title: 'Discussion', source: 'user_chat' });
    const thread = graph.createThread({ name: 'Voice Mode', taskId: task.id });

    expect(thread.status).toBe('active');
    expect(graph.getTask(task.id).threadId).toBe(thread.id);

    graph.updateThreadStatus(thread.id, 'paused');
    expect(graph.getThread(thread.id).status).toBe('paused');
  });

  it('should throw when task not found', () => {
    const { graph } = setup();
    expect(() => graph.getTask('nonexistent')).toThrow('Task not found');
  });

  it('should assign agents to tasks', () => {
    const { graph } = setup();
    const task = graph.createTask({ title: 'Test', source: 'user_chat' });
    graph.assignAgent(task.id, 'memory-agent');
    expect(graph.getTask(task.id).assignedAgent).toBe('memory-agent');
  });

  it('should detect stale tasks', () => {
    const { graph } = setup();
    const task = graph.createTask({ title: 'Old task', source: 'user_chat' });
    graph.updateStatus(task.id, 'in_progress');

    // Manually backdate
    const t = graph.getTask(task.id);
    t.updatedAt = new Date(Date.now() - 60 * 60 * 1000); // 1 hour ago

    expect(graph.getStaleTasks(30 * 60 * 1000)).toHaveLength(1);
  });
});
