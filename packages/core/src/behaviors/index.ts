/**
 * Behavior Module — Persistent behavioral patterns for Fort
 *
 * Behaviors are rules woven into how Fort operates. They are stored
 * as first-class memory graph nodes and connected to context nodes
 * via edges. Examples:
 *   - "Always address Sarah as 'ma'am' in emails"
 *   - "Prefer concise responses unless asked for detail"
 *   - "When scheduling meetings, always check for conflicts first"
 */

import { v4 as uuid } from 'uuid';
import type { Behavior, DiagnosticResult } from '../types.js';
import type { MemoryManager } from '../memory/index.js';
import type { ModuleBus } from '../module-bus/index.js';

export class BehaviorManager {
  private memory: MemoryManager;
  private bus: ModuleBus;

  constructor(memory: MemoryManager, bus: ModuleBus) {
    this.memory = memory;
    this.bus = bus;
  }

  /**
   * Create a new behavior and store it as a memory graph node.
   * Also creates a context node (if needed) and links the behavior to it.
   */
  addBehavior(
    rule: string,
    context: string,
    priority: number = 5,
    source: string = 'user',
    examples?: string[],
  ): Behavior {
    const id = uuid();
    const now = new Date().toISOString();

    const behavior: Behavior = {
      id,
      rule,
      context,
      priority: Math.max(1, Math.min(10, priority)),
      enabled: true,
      createdAt: now,
      source,
      examples,
    };

    // Store as memory graph node
    this.memory.createNode({
      type: 'behavior',
      label: rule,
      properties: {
        behaviorId: id,
        rule,
        context,
        priority: behavior.priority,
        enabled: true,
        source,
        examples: examples ?? [],
        createdAt: now,
      },
      source,
    });

    // Find or create a context node and link
    this.ensureContextEdge(id, context);

    this.bus.publish('behavior.added', 'behaviors', { behavior });
    return behavior;
  }

  /**
   * Soft-delete: mark disabled rather than removing from graph.
   */
  removeBehavior(id: string): boolean {
    const node = this.findBehaviorNode(id);
    if (!node) return false;

    this.memory.updateNode(node.id, {
      properties: { ...node.properties, enabled: false },
    });

    this.bus.publish('behavior.removed', 'behaviors', { behaviorId: id });
    return true;
  }

  getBehavior(id: string): Behavior | null {
    const node = this.findBehaviorNode(id);
    if (!node) return null;
    return this.nodeToBehavior(node);
  }

  /**
   * List all active behaviors, optionally filtered by context.
   */
  listBehaviors(context?: string): Behavior[] {
    const result = this.memory.search({ nodeType: 'behavior' });
    let behaviors = result.nodes
      .map((n) => this.nodeToBehavior(n))
      .filter((b) => b.enabled);

    if (context) {
      behaviors = behaviors.filter(
        (b) => b.context === context || b.context === 'all',
      );
    }

    return behaviors.sort((a, b) => b.priority - a.priority);
  }

  /**
   * Get behaviors matching any of the given contexts, sorted by priority.
   */
  getRelevantBehaviors(contexts: string[]): Behavior[] {
    const contextSet = new Set([...contexts, 'all']);
    const result = this.memory.search({ nodeType: 'behavior' });

    return result.nodes
      .map((n) => this.nodeToBehavior(n))
      .filter((b) => b.enabled && contextSet.has(b.context))
      .sort((a, b) => b.priority - a.priority);
  }

  /**
   * Update a behavior's fields.
   */
  updateBehavior(
    id: string,
    updates: Partial<Pick<Behavior, 'rule' | 'context' | 'priority' | 'enabled' | 'examples'>>,
  ): Behavior | null {
    const node = this.findBehaviorNode(id);
    if (!node) return null;

    const props = { ...node.properties };
    if (updates.rule !== undefined) props.rule = updates.rule;
    if (updates.context !== undefined) props.context = updates.context;
    if (updates.priority !== undefined) props.priority = Math.max(1, Math.min(10, updates.priority));
    if (updates.enabled !== undefined) props.enabled = updates.enabled;
    if (updates.examples !== undefined) props.examples = updates.examples;

    const label = (updates.rule as string) ?? node.label;
    this.memory.updateNode(node.id, { label, properties: props });

    // Re-link context if changed
    if (updates.context !== undefined) {
      this.ensureContextEdge(id, updates.context);
    }

    this.bus.publish('behavior.updated', 'behaviors', { behaviorId: id, updates });
    return this.getBehavior(id);
  }

  diagnose(): DiagnosticResult {
    const allBehaviors = this.listBehaviors();
    const checks = [
      {
        name: 'Behavior count',
        passed: true,
        message: `${allBehaviors.length} active behaviors`,
      },
    ];

    const contexts = new Set(allBehaviors.map((b) => b.context));
    checks.push({
      name: 'Context coverage',
      passed: true,
      message: `Contexts: ${Array.from(contexts).join(', ') || 'none'}`,
    });

    return {
      module: 'behaviors',
      status: 'healthy',
      checks,
    };
  }

  // ─── Internal Helpers ───────────────────────────────────────────

  private findBehaviorNode(behaviorId: string): ReturnType<MemoryManager['getNode']> {
    const result = this.memory.search({ nodeType: 'behavior' });
    return result.nodes.find((n) => n.properties.behaviorId === behaviorId) ?? null;
  }

  private nodeToBehavior(node: NonNullable<ReturnType<MemoryManager['getNode']>>): Behavior {
    return {
      id: (node.properties.behaviorId as string) ?? node.id,
      rule: (node.properties.rule as string) ?? node.label,
      context: (node.properties.context as string) ?? 'all',
      priority: (node.properties.priority as number) ?? 5,
      enabled: (node.properties.enabled as boolean) ?? true,
      createdAt: (node.properties.createdAt as string) ?? node.createdAt.toISOString(),
      source: node.source || (node.properties.source as string) || 'unknown',
      examples: (node.properties.examples as string[]) ?? undefined,
    };
  }

  private ensureContextEdge(behaviorId: string, context: string): void {
    // Find or create context node
    const contextSearch = this.memory.search({ nodeType: 'entity', text: `context:${context}` });
    let contextNode = contextSearch.nodes[0];

    if (!contextNode) {
      contextNode = this.memory.createNode({
        type: 'entity',
        label: `context:${context}`,
        properties: { contextName: context },
        source: 'behaviors',
      });
    }

    // Find the behavior node and link
    const behaviorNode = this.findBehaviorNode(behaviorId);
    if (behaviorNode) {
      this.memory.createEdge({
        sourceId: behaviorNode.id,
        targetId: contextNode.id,
        type: 'applies_to_context',
        properties: { context },
      });
    }
  }
}
