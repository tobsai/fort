/**
 * HatchService — orchestrates the conversational "hatch" between a
 * newly-created agent and its user.
 *
 * What's a hatch: a free-form first session where the agent gets to
 * know the user and proposes 2–4 working goals at the end. The agent
 * is marked `hatchedAt: <timestamp>` on completion; the proposed goals
 * are persisted as structured Goal rows.
 *
 * What this service does:
 *   - Tells the caller whether an agent still needs to hatch
 *     (`isHatching(agentId)`) and provides the opening message
 *     (`openingMessage(agentId)`).
 *   - Provides the system-prompt addendum that drives the conversation
 *     (`promptAddendum()`).
 *   - Detects the `[HATCH_COMPLETE: 1,2,3]` marker in agent output and
 *     completes the hatch: persists goals, sets `hatchedAt`, and writes
 *     anything captured during the chat to the memory graph.
 *
 * What this service does NOT do:
 *   - Drive the chat loop itself. The existing chat path (Specialist /
 *     Orchestrator) already does that; we just contribute extra system
 *     prompt content and a completion hook.
 */

import type { ModuleBus } from '../module-bus/index.js';
import type { AgentFactory } from '../agents/hatchery.js';
import type { MemoryManager } from '../memory/index.js';
import type { GoalsService } from './goals.js';
import { HATCH_SYSTEM_ADDENDUM, HATCH_OPENING_MESSAGE } from './hatch-prompt.js';

export interface ProposedGoal {
  title: string;
  description?: string | null;
}

export interface HatchCompletionResult {
  hatchedAt: string;
  goalIds: string[];
}

export class HatchService {
  constructor(
    private factory: AgentFactory,
    private goals: GoalsService,
    private memory: MemoryManager,
    private bus: ModuleBus,
  ) {}

  /**
   * Returns true if this agent has not yet completed its hatch
   * conversation. Treats missing `hatchedAt` and explicit `null` the
   * same — both mean "not yet."
   */
  isHatching(agentId: string): boolean {
    const identity = this.factory.getIdentity?.(agentId);
    if (!identity) return false;
    return identity.hatchedAt == null;
  }

  /**
   * Opening message the agent sends on first chat. Uses the agent's
   * own name so it feels personal.
   */
  openingMessage(agentId: string): string | null {
    const identity = this.factory.getIdentity?.(agentId);
    if (!identity) return null;
    return HATCH_OPENING_MESSAGE(identity.name);
  }

  /** System-prompt addendum to append while the hatch is in progress. */
  promptAddendum(): string {
    return HATCH_SYSTEM_ADDENDUM;
  }

  /**
   * Persist confirmed goals and mark the agent hatched. Returns the
   * timestamp set and the IDs of created goals.
   */
  complete(agentId: string, goals: ProposedGoal[]): HatchCompletionResult {
    const hatchedAt = new Date().toISOString();
    const goalIds: string[] = [];
    for (const g of goals) {
      if (!g.title || !g.title.trim()) continue;
      const created = this.goals.create({
        agentId,
        title: g.title,
        description: g.description ?? null,
        source: 'hatch',
      });
      goalIds.push(created.id);
    }

    // Persist hatchedAt on the agent's identity
    this.factory.updateIdentity?.(agentId, { hatchedAt });

    void this.bus.publish('hatch:completed', 'hatch', { agentId, hatchedAt, goalIds });
    return { hatchedAt, goalIds };
  }

  /**
   * Write a profile fact captured during the hatch to the memory graph.
   * Stored as a `profile`-typed memory node so it gets injected into
   * future system prompts ("About the User") across chats.
   */
  captureProfileFact(label: string, properties: Record<string, unknown> = {}): void {
    this.memory.createNode({
      type: 'profile',
      label: label.trim(),
      properties,
      source: 'hatch',
    });
  }

  /**
   * Parse the marker the agent emits to signal it's done.
   * Format: `[HATCH_COMPLETE: 1, 2, 3]` — references the goals the
   * agent proposed earlier in the conversation.
   *
   * Returns the parsed indices (1-based as the model emits them) or
   * null when no marker is present. Callers pair this with the agent's
   * most recent proposed-goals list to drive `complete()`.
   */
  static parseCompletionMarker(text: string): number[] | null {
    const m = text.match(/\[HATCH_COMPLETE:\s*([0-9,\s]+)\]/);
    if (!m) return null;
    const nums = m[1]
      .split(',')
      .map((s) => parseInt(s.trim(), 10))
      .filter((n) => Number.isFinite(n) && n > 0);
    return nums.length > 0 ? nums : null;
  }
}
