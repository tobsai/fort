import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import { SubscriptionQuotaStore } from '../llm/quota-store.js';

describe('SubscriptionQuotaStore', () => {
  let tmpDir: string;
  let store: SubscriptionQuotaStore;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-quota-'));
    store = new SubscriptionQuotaStore(join(tmpDir, 'quota.db'));
  });

  afterEach(() => {
    store.close();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('returns null for unknown provider', () => {
    expect(store.get('openai')).toBeNull();
  });

  it('upserts a snapshot and reads it back', () => {
    const written = store.set({
      providerId: 'openai',
      remaining: 142,
      used: 58,
      limit: 200,
      windowLabel: '3h',
      resetAt: '2026-05-11T18:00:00.000Z',
      rawHeaders: { 'x-ratelimit-remaining-requests': '142' },
    });

    expect(written.providerId).toBe('openai');
    expect(written.updatedAt).toBeTruthy();

    const read = store.get('openai');
    expect(read).not.toBeNull();
    expect(read!.remaining).toBe(142);
    expect(read!.limit).toBe(200);
    expect(read!.windowLabel).toBe('3h');
    expect(read!.resetAt).toBe('2026-05-11T18:00:00.000Z');
    expect(read!.rawHeaders['x-ratelimit-remaining-requests']).toBe('142');
  });

  it('overwrites the existing snapshot on second set', () => {
    store.set({
      providerId: 'openai',
      remaining: 142,
      used: 58,
      limit: 200,
      windowLabel: '3h',
      resetAt: null,
      rawHeaders: {},
    });
    store.set({
      providerId: 'openai',
      remaining: 100,
      used: 100,
      limit: 200,
      windowLabel: '3h',
      resetAt: null,
      rawHeaders: {},
    });

    const read = store.get('openai');
    expect(read!.remaining).toBe(100);
    expect(read!.used).toBe(100);
  });

  it('lists multiple providers in descending update order', () => {
    store.set({
      providerId: 'openai',
      remaining: 50,
      used: 50,
      limit: 100,
      windowLabel: null,
      resetAt: null,
      rawHeaders: {},
    });

    const all = store.list();
    expect(all.length).toBe(1);
    expect(all[0].providerId).toBe('openai');
  });

  it('persists snapshots across reopens', () => {
    store.set({
      providerId: 'openai',
      remaining: 42,
      used: 8,
      limit: 50,
      windowLabel: null,
      resetAt: null,
      rawHeaders: { 'x-ratelimit-remaining': '42' },
    });

    store.close();
    store = new SubscriptionQuotaStore(join(tmpDir, 'quota.db'));

    const read = store.get('openai');
    expect(read).not.toBeNull();
    expect(read!.remaining).toBe(42);
    expect(read!.rawHeaders['x-ratelimit-remaining']).toBe('42');
  });
});
