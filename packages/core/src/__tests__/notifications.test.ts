import { describe, it, expect, afterEach, vi } from 'vitest';
import Database from 'better-sqlite3';
import { NotificationStore } from '../notifications/store.js';
import { NotificationService } from '../notifications/service.js';
import { ModuleBus } from '../module-bus/index.js';

describe('NotificationStore', () => {
  let db: Database.Database;
  let store: NotificationStore;

  function setup() {
    db = new Database(':memory:');
    store = new NotificationStore(db);
    store.initSchema();
  }

  afterEach(() => {
    db?.close();
  });

  it('create() writes to DB and returns Notification', () => {
    setup();
    const n = store.create({ type: 'task.completed', title: 'Task done', body: 'result ok' });
    expect(n.id).toBeDefined();
    expect(n.type).toBe('task.completed');
    expect(n.title).toBe('Task done');
    expect(n.body).toBe('result ok');
    expect(n.read).toBe(false);
    expect(n.createdAt).toBeInstanceOf(Date);

    const list = store.list();
    expect(list).toHaveLength(1);
    expect(list[0].id).toBe(n.id);
  });

  it('markRead() sets read flag on specific notification', () => {
    setup();
    const n1 = store.create({ type: 'task.completed', title: 'A' });
    const n2 = store.create({ type: 'task.failed', title: 'B' });

    store.markRead(n1.id);

    const list = store.list();
    const found1 = list.find((n) => n.id === n1.id)!;
    const found2 = list.find((n) => n.id === n2.id)!;
    expect(found1.read).toBe(true);
    expect(found2.read).toBe(false);
  });

  it('markAllRead() marks all notifications as read', () => {
    setup();
    store.create({ type: 'task.completed', title: 'A' });
    store.create({ type: 'task.failed', title: 'B' });
    store.create({ type: 'approval.required', title: 'C' });

    store.markAllRead();

    const list = store.list();
    expect(list.every((n) => n.read)).toBe(true);
  });

  it('getUnreadCount() returns correct count', () => {
    setup();
    expect(store.getUnreadCount()).toBe(0);

    const n1 = store.create({ type: 'task.completed', title: 'A' });
    store.create({ type: 'task.failed', title: 'B' });
    store.create({ type: 'agent.started', title: 'C' });
    expect(store.getUnreadCount()).toBe(3);

    store.markRead(n1.id);
    expect(store.getUnreadCount()).toBe(2);

    store.markAllRead();
    expect(store.getUnreadCount()).toBe(0);
  });

  it('list({ unreadOnly: true }) filters correctly', () => {
    setup();
    const n1 = store.create({ type: 'task.completed', title: 'A' });
    store.create({ type: 'task.failed', title: 'B' });
    store.markRead(n1.id);

    const unread = store.list({ unreadOnly: true });
    expect(unread).toHaveLength(1);
    expect(unread[0].title).toBe('B');
  });

  it('list({ limit }) returns at most N items', () => {
    setup();
    for (let i = 0; i < 5; i++) {
      store.create({ type: 'task.completed', title: `Task ${i}` });
    }
    const limited = store.list({ limit: 3 });
    expect(limited).toHaveLength(3);
  });
});

describe('NotificationService', () => {
  let db: Database.Database;
  let store: NotificationStore;
  let bus: ModuleBus;
  let service: NotificationService;

  function setup() {
    db = new Database(':memory:');
    store = new NotificationStore(db);
    store.initSchema();
    bus = new ModuleBus();
    service = new NotificationService(store, bus);
    service.start();
  }

  afterEach(() => {
    service?.stop();
    db?.close();
  });

  it('creates notification on task.completed bus event', async () => {
    setup();
    await bus.publish('task.completed', 'test', { id: 'task-1', title: 'Build app' });
    const list = store.list();
    expect(list.length).toBeGreaterThanOrEqual(1);
    const n = list.find((x) => x.type === 'task.completed')!;
    expect(n).toBeDefined();
    expect(n.title).toContain('Build app');
    expect(n.entityId).toBe('task-1');
  });

  it('creates notification on task.failed bus event', async () => {
    setup();
    await bus.publish('task.failed', 'test', { id: 'task-2', title: 'Deploy', error: 'timeout' });
    const list = store.list();
    const n = list.find((x) => x.type === 'task.failed')!;
    expect(n).toBeDefined();
    expect(n.title).toContain('Deploy');
    expect(n.body).toBe('timeout');
  });

  it('creates notification on task.status_changed to completed', async () => {
    setup();
    await bus.publish('task.status_changed', 'test', {
      newStatus: 'completed',
      task: { id: 'task-3', title: 'Run tests' },
    });
    const list = store.list();
    const n = list.find((x) => x.type === 'task.completed')!;
    expect(n).toBeDefined();
    expect(n.title).toContain('Run tests');
  });

  it('does not create notification on non-terminal task.status_changed', async () => {
    setup();
    await bus.publish('task.status_changed', 'test', {
      newStatus: 'in_progress',
      task: { id: 'task-4', title: 'Running' },
    });
    expect(store.list()).toHaveLength(0);
  });

  it('invokes push callback on new notification', async () => {
    setup();
    const received: unknown[] = [];
    service.onNotification((n) => { received.push(n); });
    await bus.publish('task.completed', 'test', { id: 'task-5', title: 'Done' });
    expect(received).toHaveLength(1);
  });

  it('multiple push callbacks are all invoked', async () => {
    setup();
    const calls: string[] = [];
    service.onNotification(() => { calls.push('a'); });
    service.onNotification(() => { calls.push('b'); });
    await bus.publish('approval.required', 'test', { tool: 'web-browse', taskTitle: 'Research' });
    expect(calls).toContain('a');
    expect(calls).toContain('b');
  });

  it('stop() unsubscribes from bus events', async () => {
    setup();
    service.stop();
    await bus.publish('task.completed', 'test', { id: 't', title: 'Silent' });
    expect(store.list()).toHaveLength(0);
  });
});
