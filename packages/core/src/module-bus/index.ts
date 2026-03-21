/**
 * Module Bus — Event-driven backbone for Fort
 *
 * All modules publish and subscribe to typed events through this bus.
 * Replaces OpenClaw's brittle heartbeat/cron conflicts with clear,
 * inspectable event streams.
 */

import { v4 as uuid } from 'uuid';
import type { FortEvent, EventHandler } from '../types.js';

export class ModuleBus {
  private handlers: Map<string, Set<EventHandler>> = new Map();
  private history: FortEvent[] = [];
  private maxHistory: number;

  constructor(options: { maxHistory?: number } = {}) {
    this.maxHistory = options.maxHistory ?? 1000;
  }

  subscribe<T = unknown>(eventType: string, handler: EventHandler<T>): () => void {
    if (!this.handlers.has(eventType)) {
      this.handlers.set(eventType, new Set());
    }
    const handlerSet = this.handlers.get(eventType)!;
    handlerSet.add(handler as EventHandler);

    // Return unsubscribe function
    return () => {
      handlerSet.delete(handler as EventHandler);
      if (handlerSet.size === 0) {
        this.handlers.delete(eventType);
      }
    };
  }

  async publish<T = unknown>(eventType: string, source: string, payload: T): Promise<void> {
    const event: FortEvent<T> = {
      id: uuid(),
      type: eventType,
      source,
      timestamp: new Date(),
      payload,
    };

    this.history.push(event as FortEvent);
    if (this.history.length > this.maxHistory) {
      this.history = this.history.slice(-this.maxHistory);
    }

    const handlers = this.handlers.get(eventType);
    if (!handlers || handlers.size === 0) return;

    const errors: Error[] = [];
    for (const handler of handlers) {
      try {
        await handler(event as FortEvent);
      } catch (err) {
        errors.push(err instanceof Error ? err : new Error(String(err)));
      }
    }

    if (errors.length > 0) {
      // Publish error event (avoid infinite loop by not re-throwing)
      if (eventType !== 'bus.error') {
        await this.publish('bus.error', 'module-bus', {
          originalEvent: eventType,
          errors: errors.map((e) => e.message),
        });
      }
    }
  }

  getHistory(eventType?: string, limit?: number): FortEvent[] {
    let events = eventType
      ? this.history.filter((e) => e.type === eventType)
      : this.history;
    if (limit) {
      events = events.slice(-limit);
    }
    return events;
  }

  getSubscriptionCount(eventType?: string): number {
    if (eventType) {
      return this.handlers.get(eventType)?.size ?? 0;
    }
    let total = 0;
    for (const handlers of this.handlers.values()) {
      total += handlers.size;
    }
    return total;
  }

  getEventTypes(): string[] {
    return Array.from(this.handlers.keys());
  }

  clear(): void {
    this.handlers.clear();
    this.history = [];
  }
}
