/**
 * NotificationService — Subscribes to bus events and creates notifications.
 * Passive subscriber: if not initialized, no notifications are generated.
 * Feature flag: FORT_NOTIFICATIONS_ENABLED=false disables the service.
 */

import type { ModuleBus } from '../module-bus/index.js';
import type { FortEvent } from '../types.js';
import { NotificationStore, type Notification } from './store.js';

type PushCallback = (notification: Notification) => void;

export class NotificationService {
  private store: NotificationStore;
  private bus: ModuleBus;
  private pushCallbacks: PushCallback[] = [];
  private unsubscribers: Array<() => void> = [];
  private enabled: boolean;

  constructor(store: NotificationStore, bus: ModuleBus) {
    this.store = store;
    this.bus = bus;
    this.enabled = process.env.FORT_NOTIFICATIONS_ENABLED !== 'false';
  }

  start(): void {
    if (!this.enabled) return;

    this.unsubscribers.push(
      this.bus.subscribe('task.completed', (event: FortEvent) => {
        const payload = event.payload as Record<string, unknown>;
        const task = (payload?.task ?? payload) as Record<string, unknown>;
        const title = (task?.title as string) ?? 'Unknown task';
        const taskId = (task?.id as string) ?? undefined;
        const n = this.store.create({
          type: 'task.completed',
          title: `Task completed: ${title}`,
          entityType: 'task',
          entityId: taskId,
        });
        this.push(n);
      }),
    );

    this.unsubscribers.push(
      this.bus.subscribe('task.failed', (event: FortEvent) => {
        const payload = event.payload as Record<string, unknown>;
        const task = (payload?.task ?? payload) as Record<string, unknown>;
        const title = (task?.title as string) ?? 'Unknown task';
        const taskId = (task?.id as string) ?? undefined;
        const error = (task?.error as string) ?? (payload?.error as string) ?? undefined;
        const n = this.store.create({
          type: 'task.failed',
          title: `Task failed: ${title}`,
          body: error,
          entityType: 'task',
          entityId: taskId,
        });
        this.push(n);
      }),
    );

    this.unsubscribers.push(
      this.bus.subscribe('task.status_changed', (event: FortEvent) => {
        const payload = event.payload as Record<string, unknown>;
        const task = (payload?.task ?? payload) as Record<string, unknown>;
        const newStatus = (payload?.newStatus as string) ?? (task?.status as string);
        if (newStatus !== 'completed' && newStatus !== 'failed') return;
        const title = (task?.title as string) ?? 'Unknown task';
        const taskId = (task?.id as string) ?? undefined;
        if (newStatus === 'completed') {
          const n = this.store.create({
            type: 'task.completed',
            title: `Task completed: ${title}`,
            entityType: 'task',
            entityId: taskId,
          });
          this.push(n);
        } else {
          const error = (task?.error as string) ?? undefined;
          const n = this.store.create({
            type: 'task.failed',
            title: `Task failed: ${title}`,
            body: error,
            entityType: 'task',
            entityId: taskId,
          });
          this.push(n);
        }
      }),
    );

    this.unsubscribers.push(
      this.bus.subscribe('approval.required', (event: FortEvent) => {
        const payload = event.payload as Record<string, unknown>;
        const tool = (payload?.tool as string) ?? 'unknown tool';
        const taskTitle = (payload?.taskTitle as string) ?? (payload?.task as string) ?? 'task';
        const approvalId = (payload?.id as string) ?? undefined;
        const n = this.store.create({
          type: 'approval.required',
          title: `Approval needed: ${tool} in ${taskTitle}`,
          entityType: 'approval',
          entityId: approvalId,
        });
        this.push(n);
      }),
    );

    this.unsubscribers.push(
      this.bus.subscribe('agent.started', (event: FortEvent) => {
        const payload = event.payload as Record<string, unknown>;
        const name = (payload?.name as string) ?? (payload?.agentName as string) ?? 'Agent';
        const agentId = (payload?.id as string) ?? (payload?.agentId as string) ?? undefined;
        const n = this.store.create({
          type: 'agent.started',
          title: `Agent ${name} started`,
          entityType: 'agent',
          entityId: agentId,
        });
        this.push(n);
      }),
    );

    this.unsubscribers.push(
      this.bus.subscribe('agent.stopped', (event: FortEvent) => {
        const payload = event.payload as Record<string, unknown>;
        const name = (payload?.name as string) ?? (payload?.agentName as string) ?? 'Agent';
        const agentId = (payload?.id as string) ?? (payload?.agentId as string) ?? undefined;
        const n = this.store.create({
          type: 'agent.stopped',
          title: `Agent ${name} stopped`,
          entityType: 'agent',
          entityId: agentId,
        });
        this.push(n);
      }),
    );
  }

  stop(): void {
    for (const unsub of this.unsubscribers) {
      unsub();
    }
    this.unsubscribers = [];
  }

  /**
   * Register a callback to be invoked when a new notification is created.
   * Used by Fort to broadcast 'notification.new' over WebSocket.
   */
  onNotification(callback: PushCallback): void {
    this.pushCallbacks.push(callback);
  }

  get notificationStore(): NotificationStore {
    return this.store;
  }

  private push(notification: Notification): void {
    for (const cb of this.pushCallbacks) {
      cb(notification);
    }
  }
}
