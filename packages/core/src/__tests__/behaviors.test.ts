import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import { ModuleBus } from '../module-bus/index.js';
import { MemoryManager } from '../memory/index.js';
import { BehaviorManager } from '../behaviors/index.js';

describe('BehaviorManager', () => {
  let tmpDir: string;
  let memory: MemoryManager;
  let bus: ModuleBus;
  let behaviors: BehaviorManager;

  function setup() {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-'));
    bus = new ModuleBus();
    memory = new MemoryManager(join(tmpDir, 'memory.db'), bus);
    behaviors = new BehaviorManager(memory, bus);
    return { behaviors, memory, bus };
  }

  afterEach(() => {
    memory?.close();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should add and retrieve a behavior', () => {
    setup();
    const b = behaviors.addBehavior(
      "Always address Sarah as 'ma'am' in emails",
      'email',
      8,
      'user',
    );

    expect(b.id).toBeTruthy();
    expect(b.rule).toBe("Always address Sarah as 'ma'am' in emails");
    expect(b.context).toBe('email');
    expect(b.priority).toBe(8);
    expect(b.enabled).toBe(true);

    const retrieved = behaviors.getBehavior(b.id);
    expect(retrieved).not.toBeNull();
    expect(retrieved!.rule).toBe(b.rule);
  });

  it('should list all active behaviors', () => {
    setup();
    behaviors.addBehavior('Rule A', 'email', 5, 'user');
    behaviors.addBehavior('Rule B', 'scheduling', 7, 'user');
    behaviors.addBehavior('Rule C', 'all', 3, 'reflection');

    const list = behaviors.listBehaviors();
    expect(list).toHaveLength(3);
  });

  it('should filter behaviors by context', () => {
    setup();
    behaviors.addBehavior('Email rule', 'email', 5);
    behaviors.addBehavior('Schedule rule', 'scheduling', 7);
    behaviors.addBehavior('Global rule', 'all', 3);

    const emailBehaviors = behaviors.listBehaviors('email');
    expect(emailBehaviors).toHaveLength(2); // email + all
    expect(emailBehaviors.some((b) => b.context === 'email')).toBe(true);
    expect(emailBehaviors.some((b) => b.context === 'all')).toBe(true);
  });

  it('should sort behaviors by priority (highest first)', () => {
    setup();
    behaviors.addBehavior('Low priority', 'all', 2);
    behaviors.addBehavior('High priority', 'all', 9);
    behaviors.addBehavior('Medium priority', 'all', 5);

    const list = behaviors.listBehaviors();
    expect(list[0].priority).toBe(9);
    expect(list[1].priority).toBe(5);
    expect(list[2].priority).toBe(2);
  });

  it('should get relevant behaviors for multiple contexts', () => {
    setup();
    behaviors.addBehavior('Email rule', 'email', 5);
    behaviors.addBehavior('Schedule rule', 'scheduling', 7);
    behaviors.addBehavior('Global rule', 'all', 3);
    behaviors.addBehavior('Chat rule', 'chat', 4);

    const relevant = behaviors.getRelevantBehaviors(['email', 'chat']);
    expect(relevant).toHaveLength(3); // email + chat + all
    // Sorted by priority: scheduling(7) is excluded, email(5), chat(4), all(3)
    expect(relevant[0].priority).toBe(5);
    expect(relevant[1].priority).toBe(4);
    expect(relevant[2].priority).toBe(3);
  });

  it('should soft-delete (disable) a behavior', () => {
    setup();
    const b = behaviors.addBehavior('To be removed', 'all', 5);

    const removed = behaviors.removeBehavior(b.id);
    expect(removed).toBe(true);

    // Should no longer appear in active list
    const list = behaviors.listBehaviors();
    expect(list).toHaveLength(0);

    // But the node still exists in memory graph (soft-delete)
    const result = memory.search({ nodeType: 'behavior' });
    expect(result.nodes).toHaveLength(1);
    expect(result.nodes[0].properties.enabled).toBe(false);
  });

  it('should update a behavior', () => {
    setup();
    const b = behaviors.addBehavior('Original rule', 'email', 5);

    const updated = behaviors.updateBehavior(b.id, {
      rule: 'Updated rule',
      priority: 9,
      context: 'scheduling',
    });

    expect(updated).not.toBeNull();
    expect(updated!.rule).toBe('Updated rule');
    expect(updated!.priority).toBe(9);
    expect(updated!.context).toBe('scheduling');
  });

  it('should clamp priority to 1-10 range', () => {
    setup();
    const low = behaviors.addBehavior('Low', 'all', -5);
    expect(low.priority).toBe(1);

    const high = behaviors.addBehavior('High', 'all', 99);
    expect(high.priority).toBe(10);
  });

  it('should enable/disable via update', () => {
    setup();
    const b = behaviors.addBehavior('Toggle me', 'all', 5);

    behaviors.updateBehavior(b.id, { enabled: false });
    expect(behaviors.listBehaviors()).toHaveLength(0);

    behaviors.updateBehavior(b.id, { enabled: true });
    expect(behaviors.listBehaviors()).toHaveLength(1);
  });

  it('should store examples', () => {
    setup();
    const b = behaviors.addBehavior(
      'Be concise',
      'all',
      5,
      'user',
      ['Short answer example', 'Brief summary example'],
    );

    const retrieved = behaviors.getBehavior(b.id);
    expect(retrieved!.examples).toHaveLength(2);
    expect(retrieved!.examples![0]).toBe('Short answer example');
  });

  it('should store behaviors as memory graph nodes', () => {
    setup();
    behaviors.addBehavior('Graph test', 'email', 5);

    const result = memory.search({ nodeType: 'behavior' });
    expect(result.nodes).toHaveLength(1);
    expect(result.nodes[0].label).toBe('Graph test');
    expect(result.nodes[0].properties.context).toBe('email');
  });

  it('should create context edges in memory graph', () => {
    setup();
    const b = behaviors.addBehavior('Edge test', 'email', 5);

    const behaviorNodes = memory.search({ nodeType: 'behavior' });
    const behaviorNode = behaviorNodes.nodes[0];
    const edges = memory.getEdgesFrom(behaviorNode.id);

    expect(edges).toHaveLength(1);
    expect(edges[0].type).toBe('applies_to_context');
  });

  it('should return a healthy diagnostic', () => {
    setup();
    behaviors.addBehavior('Test', 'all', 5);

    const diag = behaviors.diagnose();
    expect(diag.module).toBe('behaviors');
    expect(diag.status).toBe('healthy');
    expect(diag.checks.length).toBeGreaterThanOrEqual(1);
  });
});
