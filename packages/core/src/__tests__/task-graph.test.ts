import { describe, it, expect, vi } from 'vitest';
import { ModuleBus } from '../module-bus/index.js';
import { TaskGraph } from '../task-graph/index.js';
import type { LLMClient } from '../llm/index.js';

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

  // ─── reviewCompletion ─────────────────────────────────────────

  function mockLLM(response: string): LLMClient {
    return {
      isConfigured: true,
      ask: vi.fn().mockResolvedValue(response),
    } as unknown as LLMClient;
  }

  it('should complete task when review approves', async () => {
    const { graph } = setup();
    graph.setLLM(mockLLM(JSON.stringify({ approved: true, reason: 'Task addressed' })));

    const task = graph.createTask({ title: 'Say hello', description: 'Greet the user', source: 'user_chat' });
    await graph.reviewCompletion(task.id, 'Hello there!');

    expect(graph.getTask(task.id).status).toBe('completed');
    expect(graph.getTask(task.id).result).toBe('Hello there!');
  });

  it('should set needs_review when review rejects', async () => {
    const { graph } = setup();
    graph.setLLM(mockLLM(JSON.stringify({ approved: false, reason: 'Response is a placeholder' })));

    const task = graph.createTask({ title: 'Analyze data', description: 'Run analysis on Q1 sales', source: 'user_chat' });
    await graph.reviewCompletion(task.id, 'I will do that later.');

    const updated = graph.getTask(task.id);
    expect(updated.status).toBe('needs_review');
    expect(updated.result).toBe('I will do that later.');
    expect(updated.metadata.statusReason).toBe('Response is a placeholder');
  });

  it('should fall through to completeTask when no LLM configured', async () => {
    const { graph } = setup();
    // No setLLM call — llm is null

    const task = graph.createTask({ title: 'Quick task', source: 'user_chat' });
    await graph.reviewCompletion(task.id, 'Done');

    expect(graph.getTask(task.id).status).toBe('completed');
    expect(graph.getTask(task.id).result).toBe('Done');
  });

  it('should fall through to completeTask on LLM error when no inability detected', async () => {
    const { graph } = setup();
    const llm = {
      isConfigured: true,
      ask: vi.fn().mockRejectedValue(new Error('API timeout')),
    } as unknown as LLMClient;
    graph.setLLM(llm);

    const task = graph.createTask({ title: 'Test task', source: 'user_chat' });
    await graph.reviewCompletion(task.id, 'Result here');

    // Deterministic check passed (no inability signals), so LLM error falls through to complete
    expect(graph.getTask(task.id).status).toBe('completed');
    expect(graph.getTask(task.id).result).toBe('Result here');
  });

  // ─── Deterministic inability detection ────────────────────────

  it('should reject when agent says it cannot do something', async () => {
    const { graph } = setup();
    // No LLM needed — deterministic check catches this before LLM call
    const task = graph.createTask({ title: 'Check my X.com feed', description: 'Check my X.com feed', source: 'user_chat' });
    const result = `I'm unable to access external websites or social media platforms, including X.com. What I *can* do instead:\n- Summarize recent posts if you paste them to me`;
    await graph.reviewCompletion(task.id, result);

    expect(graph.getTask(task.id).status).toBe('needs_review');
  });

  it('should reject when agent lacks capability and offers alternatives', async () => {
    const { graph } = setup();
    const task = graph.createTask({ title: 'Pull my feed', description: 'Pull my X feed', source: 'user_chat' });
    const result = `I currently lack direct integration with X.com's API. What I can do instead: summarize trends.`;
    await graph.reviewCompletion(task.id, result);

    expect(graph.getTask(task.id).status).toBe('needs_review');
  });

  it('should reject when agent says it cannot perform the action', async () => {
    const { graph } = setup();
    const task = graph.createTask({ title: 'Send email', description: 'Send an email to Bob', source: 'user_chat' });
    const result = `Unfortunately, I can't send emails directly. I don't have access to email services.`;
    await graph.reviewCompletion(task.id, result);

    expect(graph.getTask(task.id).status).toBe('needs_review');
  });

  it('should approve when agent actually completes the task', async () => {
    const { graph } = setup();
    graph.setLLM(mockLLM(JSON.stringify({ approved: true, reason: 'Task completed' })));
    const task = graph.createTask({ title: 'Say hello', description: 'Greet the user', source: 'user_chat' });
    await graph.reviewCompletion(task.id, 'Hello! Great to meet you. How can I help today?');

    expect(graph.getTask(task.id).status).toBe('completed');
  });

  it('should publish task.review_completed event', async () => {
    const { bus, graph } = setup();
    graph.setLLM(mockLLM(JSON.stringify({ approved: true, reason: 'Looks good' })));

    const events: unknown[] = [];
    bus.subscribe('task.review_completed', (event) => {
      events.push(event.payload);
    });

    const task = graph.createTask({ title: 'Test', source: 'user_chat' });
    await graph.reviewCompletion(task.id, 'Done');

    expect(events).toHaveLength(1);
    expect((events[0] as any).approved).toBe(true);
    expect((events[0] as any).reason).toBe('Looks good');
  });

  it('should skip inability detection when response contains a valid tool proposal', async () => {
    const { graph } = setup();
    const task = graph.createTask({ title: 'Check X feed', description: 'Check my X.com feed', source: 'user_chat' });
    const result = `I don't have direct access to X.com yet, but I can build a tool for this.\n\`\`\`json\n{"needsTool": true, "toolName": "x-feed-reader", "toolDescription": "Read X.com feeds via API", "architecture": "Use X API v2"}\n\`\`\``;
    await graph.reviewCompletion(task.id, result);

    // Valid tool proposal bypasses inability detection; no LLM → completeTask
    expect(graph.getTask(task.id).status).toBe('completed');
  });

  it('should NOT skip inability detection for malformed tool proposals', async () => {
    const { graph } = setup();
    const task = graph.createTask({ title: 'Check X feed', description: 'Check my X.com feed', source: 'user_chat' });
    // Has needsTool text but no toolName — invalid proposal
    const result = `I don't have direct access to X.com. {"needsTool": true} but I can suggest alternatives.`;
    await graph.reviewCompletion(task.id, result);

    // Malformed proposal should NOT bypass inability detection
    expect(graph.getTask(task.id).status).toBe('needs_review');
  });

  it('should reject when agent says capabilities are limited', async () => {
    const { graph } = setup();
    const task = graph.createTask({ title: 'Check feed', description: 'Check my feed', source: 'user_chat' });
    const result = `My capabilities are limited to text processing. I cannot browse websites.`;
    await graph.reviewCompletion(task.id, result);

    expect(graph.getTask(task.id).status).toBe('needs_review');
  });

  it('should reject the exact X.com response from the agent', async () => {
    const { graph } = setup();
    const task = graph.createTask({ title: 'Can you check my x.com feed?', description: 'Can you check my x.com feed?', source: 'user_chat' });
    const result = `I must be transparent: I don\u2019t have direct access to browse X.com or retrieve your live feed in real-time. My capabilities are limited to information I can access through available integrations or APIs.`;
    await graph.reviewCompletion(task.id, result);

    expect(graph.getTask(task.id).status).toBe('needs_review');
  });

  it('should reject "don\'t currently have the ability" phrasing', async () => {
    const { graph } = setup();
    const task = graph.createTask({ title: 'Check my x.com feed', description: 'Check my x.com feed', source: 'user_chat' });
    const result = `Acknowledged. I\u2019m afraid I must be direct with you here\u2014I don\u2019t currently have the ability to access X.com or pull your social media feed.`;
    await graph.reviewCompletion(task.id, result);

    expect(graph.getTask(task.id).status).toBe('needs_review');
  });

  it('should reject "I can\'t access" phrasing', async () => {
    const { graph } = setup();
    const task = graph.createTask({ title: 'Check my x.com feed', description: 'Check my x.com feed', source: 'user_chat' });
    const result = `I can't access external websites or social media platforms including X.com.`;
    await graph.reviewCompletion(task.id, result);

    expect(graph.getTask(task.id).status).toBe('needs_review');
  });

  it('should NOT false-positive on "can\'t believe" in successful responses', async () => {
    const { graph } = setup();
    graph.setLLM(mockLLM(JSON.stringify({ approved: true, reason: 'Task completed' })));
    const task = graph.createTask({ title: 'Process data', description: 'Process data', source: 'user_chat' });
    const result = `Done! I can't believe how fast that was. Your report is ready.`;
    await graph.reviewCompletion(task.id, result);

    // "can't believe" should NOT trigger inability detection
    expect(graph.getTask(task.id).status).toBe('completed');
  });
});
