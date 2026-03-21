import { describe, it, expect, vi } from 'vitest';
import { ModuleBus } from '../module-bus/index.js';

describe('ModuleBus', () => {
  it('should publish and subscribe to events', async () => {
    const bus = new ModuleBus();
    const handler = vi.fn();

    bus.subscribe('test.event', handler);
    await bus.publish('test.event', 'test-source', { data: 'hello' });

    expect(handler).toHaveBeenCalledTimes(1);
    expect(handler).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'test.event',
        source: 'test-source',
        payload: { data: 'hello' },
      })
    );
  });

  it('should support unsubscribe', async () => {
    const bus = new ModuleBus();
    const handler = vi.fn();

    const unsub = bus.subscribe('test.event', handler);
    unsub();
    await bus.publish('test.event', 'test-source', {});

    expect(handler).not.toHaveBeenCalled();
  });

  it('should maintain event history', async () => {
    const bus = new ModuleBus();

    await bus.publish('event.a', 'source', { a: 1 });
    await bus.publish('event.b', 'source', { b: 2 });
    await bus.publish('event.a', 'source', { a: 3 });

    expect(bus.getHistory()).toHaveLength(3);
    expect(bus.getHistory('event.a')).toHaveLength(2);
    expect(bus.getHistory('event.b', 1)).toHaveLength(1);
  });

  it('should handle multiple subscribers', async () => {
    const bus = new ModuleBus();
    const h1 = vi.fn();
    const h2 = vi.fn();

    bus.subscribe('test', h1);
    bus.subscribe('test', h2);
    await bus.publish('test', 'source', {});

    expect(h1).toHaveBeenCalledTimes(1);
    expect(h2).toHaveBeenCalledTimes(1);
  });

  it('should catch handler errors and publish bus.error', async () => {
    const bus = new ModuleBus();
    const errorHandler = vi.fn();

    bus.subscribe('test', () => { throw new Error('boom'); });
    bus.subscribe('bus.error', errorHandler);

    await bus.publish('test', 'source', {});

    expect(errorHandler).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'bus.error',
        payload: expect.objectContaining({
          originalEvent: 'test',
          errors: ['boom'],
        }),
      })
    );
  });

  it('should report subscription counts', () => {
    const bus = new ModuleBus();
    bus.subscribe('a', () => {});
    bus.subscribe('a', () => {});
    bus.subscribe('b', () => {});

    expect(bus.getSubscriptionCount('a')).toBe(2);
    expect(bus.getSubscriptionCount('b')).toBe(1);
    expect(bus.getSubscriptionCount()).toBe(3);
  });
});
