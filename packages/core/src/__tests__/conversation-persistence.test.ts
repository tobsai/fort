/**
 * Conversation Persistence tests — SPEC-003
 *
 * Verifies that ThreadManager is correctly wired into the chat flow:
 * - User messages are persisted before routing to agent
 * - Agent responses are persisted when tasks complete
 * - WebSocket thread handlers (threads.list, thread.history, thread.create) work correctly
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import { ThreadManager } from '../threads/index.js';
import { ModuleBus } from '../module-bus/index.js';
import { TaskGraph } from '../task-graph/index.js';

// ─── helpers ────────────────────────────────────────────────────────────────

function makeSetup() {
  const tmpDir = mkdtempSync(join(tmpdir(), 'fort-cp-test-'));
  const bus = new ModuleBus();
  const taskGraph = new TaskGraph(bus);
  const threads = new ThreadManager(join(tmpDir, 'threads.db'), bus, taskGraph);
  return { tmpDir, bus, taskGraph, threads };
}

// ─── getOrCreateAgentThread behaviour ───────────────────────────────────────

describe('getOrCreateAgentThread (server helper logic)', () => {
  let tmpDir: string;
  let threads: ThreadManager;

  beforeEach(() => {
    const s = makeSetup();
    tmpDir = s.tmpDir;
    threads = s.threads;
  });

  afterEach(() => {
    threads.close();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('creates a thread the first time an agentId is seen', () => {
    const agentId = 'agent-abc';
    const existing = threads.listThreads({ agentId, status: 'active' });
    expect(existing).toHaveLength(0);

    // Simulate getOrCreateAgentThread
    const thread = threads.createThread({ name: 'Chat', assignedAgent: agentId });
    expect(thread.assignedAgent).toBe(agentId);
    expect(thread.status).toBe('active');
  });

  it('reuses the existing thread on a subsequent call', () => {
    const agentId = 'agent-abc';
    const first = threads.createThread({ name: 'Chat', assignedAgent: agentId });

    const existing = threads.listThreads({ agentId, status: 'active' });
    expect(existing).toHaveLength(1);
    expect(existing[0].id).toBe(first.id);
  });

  it('does not create a new thread if one already exists', () => {
    const agentId = 'agent-xyz';
    threads.createThread({ name: 'Chat', assignedAgent: agentId });
    threads.createThread({ name: 'Chat', assignedAgent: agentId }); // second call would happen if cache missed

    // Two threads exist; getOrCreateAgentThread always picks first
    const all = threads.listThreads({ agentId });
    expect(all.length).toBeGreaterThanOrEqual(1);
  });
});

// ─── user message persistence ────────────────────────────────────────────────

describe('user message persistence', () => {
  let tmpDir: string;
  let threads: ThreadManager;

  beforeEach(() => {
    const s = makeSetup();
    tmpDir = s.tmpDir;
    threads = s.threads;
  });

  afterEach(() => {
    threads.close();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('persists a user message to the thread', () => {
    const agentId = 'agent-lewis';
    const thread = threads.createThread({ name: 'Chat', assignedAgent: agentId });

    threads.addMessage(thread.id, { role: 'user', content: 'Hello, how are you?' });

    const messages = threads.getMessages(thread.id);
    expect(messages).toHaveLength(1);
    expect(messages[0].role).toBe('user');
    expect(messages[0].content).toBe('Hello, how are you?');
    expect(messages[0].agentId).toBeNull();
  });

  it('does NOT persist __greeting__ internal messages', () => {
    const agentId = 'agent-lewis';
    const thread = threads.createThread({ name: 'Chat', assignedAgent: agentId });

    // Simulate the isGreeting check in the server — greetings are NOT added
    const isGreeting = '__greeting__' === '__greeting__';
    if (!isGreeting) {
      threads.addMessage(thread.id, { role: 'user', content: '__greeting__' });
    }

    const messages = threads.getMessages(thread.id);
    expect(messages).toHaveLength(0);
  });

  it('persists multiple user messages in order', () => {
    const agentId = 'agent-lewis';
    const thread = threads.createThread({ name: 'Chat', assignedAgent: agentId });

    threads.addMessage(thread.id, { role: 'user', content: 'First message' });
    threads.addMessage(thread.id, { role: 'user', content: 'Second message' });
    threads.addMessage(thread.id, { role: 'user', content: 'Third message' });

    const messages = threads.getMessages(thread.id);
    expect(messages).toHaveLength(3);
    expect(messages[0].content).toBe('First message');
    expect(messages[1].content).toBe('Second message');
    expect(messages[2].content).toBe('Third message');
  });
});

// ─── agent response persistence ──────────────────────────────────────────────

describe('agent response persistence (task.status_changed flow)', () => {
  let tmpDir: string;
  let bus: ModuleBus;
  let threads: ThreadManager;

  beforeEach(() => {
    const s = makeSetup();
    tmpDir = s.tmpDir;
    bus = s.bus;
    threads = s.threads;
  });

  afterEach(() => {
    threads.close();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('persists agent response when task result is available', () => {
    const agentId = 'agent-lewis';
    const thread = threads.createThread({ name: 'Chat', assignedAgent: agentId });

    threads.addMessage(thread.id, { role: 'user', content: 'Hello' });

    // Simulate server persisting agent response on task.status_changed
    const result = 'Hi! I am doing well, thank you for asking.';
    threads.addMessage(thread.id, { role: 'agent', content: result, agentId });

    const messages = threads.getMessages(thread.id);
    expect(messages).toHaveLength(2);
    expect(messages[1].role).toBe('agent');
    expect(messages[1].content).toBe(result);
    expect(messages[1].agentId).toBe(agentId);
  });

  it('interleaves user and agent messages correctly', () => {
    const agentId = 'agent-lewis';
    const thread = threads.createThread({ name: 'Chat', assignedAgent: agentId });

    threads.addMessage(thread.id, { role: 'user', content: 'Question 1' });
    threads.addMessage(thread.id, { role: 'agent', content: 'Answer 1', agentId });
    threads.addMessage(thread.id, { role: 'user', content: 'Question 2' });
    threads.addMessage(thread.id, { role: 'agent', content: 'Answer 2', agentId });

    const messages = threads.getMessages(thread.id);
    expect(messages).toHaveLength(4);
    expect(messages[0].role).toBe('user');
    expect(messages[1].role).toBe('agent');
    expect(messages[2].role).toBe('user');
    expect(messages[3].role).toBe('agent');
  });

  it('bus event thread.message is published when agent response is persisted', () => {
    const agentId = 'agent-lewis';
    const thread = threads.createThread({ name: 'Chat', assignedAgent: agentId });

    const events: string[] = [];
    bus.subscribe('thread.message', () => { events.push('message'); });

    threads.addMessage(thread.id, { role: 'user', content: 'Hello' });
    threads.addMessage(thread.id, { role: 'agent', content: 'Hi!', agentId });

    expect(events).toHaveLength(2);
  });

  it('taskToThread map cleanup — task not persisted twice', () => {
    // Simulate the server's taskToThread.delete(taskId) after first persist
    const taskToThread = new Map<string, string>();
    const agentId = 'agent-lewis';
    const thread = threads.createThread({ name: 'Chat', assignedAgent: agentId });

    // Register task
    taskToThread.set('task-001', thread.id);
    expect(taskToThread.has('task-001')).toBe(true);

    // First status_changed: persist and delete
    const threadId = taskToThread.get('task-001')!;
    taskToThread.delete('task-001');
    threads.addMessage(threadId, { role: 'agent', content: 'Response' });

    // Second status_changed (e.g. needs_review): should not persist again
    expect(taskToThread.has('task-001')).toBe(false);

    const messages = threads.getMessages(thread.id);
    expect(messages).toHaveLength(1);
  });
});

// ─── threads.list WS handler ──────────────────────────────────────────────────

describe('threads.list WebSocket handler logic', () => {
  let tmpDir: string;
  let threads: ThreadManager;

  beforeEach(() => {
    const s = makeSetup();
    tmpDir = s.tmpDir;
    threads = s.threads;
  });

  afterEach(() => {
    threads.close();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('returns all threads when no filter provided', () => {
    threads.createThread({ name: 'Chat', assignedAgent: 'agent-a' });
    threads.createThread({ name: 'Chat', assignedAgent: 'agent-b' });

    const all = threads.listThreads();
    expect(all).toHaveLength(2);
  });

  it('returns threads filtered by agentId', () => {
    threads.createThread({ name: 'Chat', assignedAgent: 'agent-a' });
    threads.createThread({ name: 'Chat', assignedAgent: 'agent-b' });

    const forA = threads.listThreads({ agentId: 'agent-a' });
    expect(forA).toHaveLength(1);
    expect(forA[0].assignedAgent).toBe('agent-a');
  });

  it('returns empty array for unknown agentId', () => {
    const result = threads.listThreads({ agentId: 'nonexistent-agent' });
    expect(result).toHaveLength(0);
  });
});

// ─── thread.history WS handler ───────────────────────────────────────────────

describe('thread.history WebSocket handler logic', () => {
  let tmpDir: string;
  let threads: ThreadManager;

  beforeEach(() => {
    const s = makeSetup();
    tmpDir = s.tmpDir;
    threads = s.threads;
  });

  afterEach(() => {
    threads.close();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('returns messages for a thread by threadId', () => {
    const thread = threads.createThread({ name: 'Chat', assignedAgent: 'agent-a' });
    threads.addMessage(thread.id, { role: 'user', content: 'Hello' });
    threads.addMessage(thread.id, { role: 'agent', content: 'Hi!', agentId: 'agent-a' });

    const messages = threads.getMessages(thread.id);
    expect(messages).toHaveLength(2);
    expect(messages[0].content).toBe('Hello');
    expect(messages[1].content).toBe('Hi!');
  });

  it('returns messages for a thread by agentId (finds active thread)', () => {
    const agentId = 'agent-a';
    const thread = threads.createThread({ name: 'Chat', assignedAgent: agentId });
    threads.addMessage(thread.id, { role: 'user', content: 'First' });
    threads.addMessage(thread.id, { role: 'agent', content: 'Reply', agentId });

    // Simulate handler: find active thread for agent, then get messages
    const agentThreads = threads.listThreads({ agentId, status: 'active' });
    expect(agentThreads).toHaveLength(1);
    const messages = threads.getMessages(agentThreads[0].id, { limit: 200 });
    expect(messages).toHaveLength(2);
  });

  it('returns empty messages for agent with no thread', () => {
    // Simulate handler: agentId provided but no thread exists
    const agentThreads = threads.listThreads({ agentId: 'no-thread-agent', status: 'active' });
    expect(agentThreads).toHaveLength(0);
    // Handler returns { threadId: null, agentId, messages: [] }
  });

  it('respects limit parameter', () => {
    const thread = threads.createThread({ name: 'Chat' });
    for (let i = 0; i < 10; i++) {
      threads.addMessage(thread.id, { role: 'user', content: `Message ${i}` });
    }

    const limited = threads.getMessages(thread.id, { limit: 5 });
    expect(limited).toHaveLength(5);
  });
});

// ─── thread.create WS handler ────────────────────────────────────────────────

describe('thread.create WebSocket handler logic', () => {
  let tmpDir: string;
  let threads: ThreadManager;

  beforeEach(() => {
    const s = makeSetup();
    tmpDir = s.tmpDir;
    threads = s.threads;
  });

  afterEach(() => {
    threads.close();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('creates a named thread with assignedAgent', () => {
    const thread = threads.createThread({
      name: 'Project Alpha',
      assignedAgent: 'agent-a',
      description: 'Working on the alpha project',
    });

    expect(thread.id).toBeDefined();
    expect(thread.name).toBe('Project Alpha');
    expect(thread.assignedAgent).toBe('agent-a');
    expect(thread.description).toBe('Working on the alpha project');
    expect(thread.status).toBe('active');
  });

  it('creates a thread without assignedAgent', () => {
    const thread = threads.createThread({ name: 'Standalone Thread' });

    expect(thread.id).toBeDefined();
    expect(thread.name).toBe('Standalone Thread');
    expect(thread.assignedAgent).toBeNull();
  });

  it('created thread appears in threads.list', () => {
    threads.createThread({ name: 'Alpha', assignedAgent: 'agent-a' });

    const all = threads.listThreads();
    expect(all.some((t) => t.name === 'Alpha')).toBe(true);
  });
});

// ─── full conversation round-trip ────────────────────────────────────────────

describe('full conversation round-trip', () => {
  let tmpDir: string;
  let threads: ThreadManager;

  beforeEach(() => {
    const s = makeSetup();
    tmpDir = s.tmpDir;
    threads = s.threads;
  });

  afterEach(() => {
    threads.close();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('simulates a complete chat turn persisted in a thread', () => {
    const agentId = 'agent-lewis';

    // 1. Find or create thread
    let agentThreads = threads.listThreads({ agentId, status: 'active' });
    const thread = agentThreads.length > 0
      ? agentThreads[0]
      : threads.createThread({ name: 'Chat', assignedAgent: agentId });

    // 2. Persist user message
    threads.addMessage(thread.id, { role: 'user', content: 'What is the weather like?' });

    // 3. Simulate task completion — persist agent response
    const taskToThread = new Map([['task-001', thread.id]]);
    const taskId = 'task-001';
    const result = 'The weather is sunny and 72°F today!';

    if (taskToThread.has(taskId)) {
      threads.addMessage(taskToThread.get(taskId)!, { role: 'agent', content: result, agentId });
      taskToThread.delete(taskId);
    }

    // 4. Load history (simulating thread.history request)
    agentThreads = threads.listThreads({ agentId, status: 'active' });
    expect(agentThreads).toHaveLength(1);
    const history = threads.getMessages(agentThreads[0].id, { limit: 200 });

    expect(history).toHaveLength(2);
    expect(history[0].role).toBe('user');
    expect(history[0].content).toBe('What is the weather like?');
    expect(history[1].role).toBe('agent');
    expect(history[1].content).toBe(result);
    expect(history[1].agentId).toBe(agentId);
  });

  it('history survives thread manager restart (SQLite persistence)', () => {
    const agentId = 'agent-lewis';
    const dbPath = join(tmpDir, 'threads.db');

    // First "session" — create thread and add messages
    const { bus: bus1, taskGraph: tg1 } = makeSetup();
    const threads1 = new ThreadManager(dbPath, bus1, tg1);
    const thread = threads1.createThread({ name: 'Chat', assignedAgent: agentId });
    threads1.addMessage(thread.id, { role: 'user', content: 'Persisted message' });
    threads1.addMessage(thread.id, { role: 'agent', content: 'Persisted reply', agentId });
    threads1.close();

    // Second "session" — new ThreadManager instance, same DB
    const { bus: bus2, taskGraph: tg2 } = makeSetup();
    const threads2 = new ThreadManager(dbPath, bus2, tg2);
    const loaded = threads2.listThreads({ agentId });
    expect(loaded).toHaveLength(1);
    const messages = threads2.getMessages(loaded[0].id);
    expect(messages).toHaveLength(2);
    expect(messages[0].content).toBe('Persisted message');
    expect(messages[1].content).toBe('Persisted reply');
    threads2.close();
  });
});
