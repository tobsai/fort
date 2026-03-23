/**
 * Reflection Service — Deterministic chat review
 *
 * Not an agent. Periodically reviews recent chat tasks to ensure
 * no action items were missed. Creates tasks for anything that
 * slipped through.
 */

import type { Task } from '../types.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { LLMClient } from '../llm/index.js';
import type { DiagnosticResult } from '../types.js';

export interface ReflectionResult {
  reviewedTasks: number;
  newTasks: Task[];
  summary: string;
}

export class ReflectionService {
  private taskGraph: TaskGraph;
  private bus: ModuleBus;
  private llm: LLMClient;
  private lastReviewAt: Date | null = null;

  constructor(taskGraph: TaskGraph, bus: ModuleBus, llm: LLMClient) {
    this.taskGraph = taskGraph;
    this.bus = bus;
    this.llm = llm;
  }

  /**
   * Review recent chat tasks for missed action items.
   * This is a deterministic pass — it scans completed tasks and
   * checks if any mentioned actions weren't turned into tasks.
   */
  async reviewChats(since?: Date): Promise<ReflectionResult> {
    const cutoff = since ?? this.lastReviewAt ?? new Date(Date.now() - 30 * 60 * 1000);
    this.lastReviewAt = new Date();

    // Get all completed chat tasks since cutoff
    const allTasks = this.taskGraph.getAllTasks();
    const chatTasks = allTasks.filter((t) =>
      t.metadata.type === 'chat' &&
      t.status === 'completed' &&
      t.result &&
      t.completedAt &&
      t.completedAt >= cutoff
    );

    if (chatTasks.length === 0) {
      return {
        reviewedTasks: 0,
        newTasks: [],
        summary: 'No recent chat tasks to review.',
      };
    }

    // If LLM is not configured, do a basic pattern-based review
    if (!this.llm.isConfigured) {
      return this.basicReview(chatTasks);
    }

    // Use LLM to analyze chats for missed tasks
    return this.llmReview(chatTasks);
  }

  /**
   * Basic pattern-based review (no LLM needed).
   * Looks for common action indicators in chat results.
   */
  private basicReview(chatTasks: Task[]): ReflectionResult {
    const newTasks: Task[] = [];
    const actionPatterns = [
      /(?:i(?:'ll| will)|let me|going to|need to|should|must|have to|don't forget to)\s+(.+?)(?:\.|$)/gi,
      /(?:remind|schedule|follow up|check back|circle back)\s+(.+?)(?:\.|$)/gi,
      /(?:todo|action item|next step)[:\s]+(.+?)(?:\.|$)/gi,
    ];

    for (const task of chatTasks) {
      const text = `${task.description} ${task.result ?? ''}`;

      for (const pattern of actionPatterns) {
        pattern.lastIndex = 0;
        let match;
        while ((match = pattern.exec(text)) !== null) {
          const actionText = match[1].trim();
          if (actionText.length > 10 && actionText.length < 200) {
            // Check if a similar task already exists
            const existing = this.taskGraph.getAllTasks().find((t) =>
              t.title.toLowerCase().includes(actionText.toLowerCase().slice(0, 30))
            );
            if (!existing) {
              const newTask = this.taskGraph.createTask({
                title: actionText.slice(0, 100),
                description: `Detected from chat: "${task.title}"`,
                source: 'reflection',
                metadata: { detectedFrom: task.id, type: 'action_item' },
              });
              newTask.assignedTo = 'user';
              newTasks.push(newTask);
            }
          }
        }
      }
    }

    const summary = newTasks.length > 0
      ? `Found ${newTasks.length} potential action item${newTasks.length > 1 ? 's' : ''} in ${chatTasks.length} recent chat${chatTasks.length > 1 ? 's' : ''}.`
      : `Reviewed ${chatTasks.length} recent chat${chatTasks.length > 1 ? 's' : ''}. No missed action items detected.`;

    this.bus.publish('reflection.completed', 'reflection', {
      reviewedTasks: chatTasks.length,
      newTaskCount: newTasks.length,
      summary,
    });

    return { reviewedTasks: chatTasks.length, newTasks, summary };
  }

  /**
   * LLM-powered review — asks Claude to find missed tasks.
   */
  private async llmReview(chatTasks: Task[]): Promise<ReflectionResult> {
    // Build a summary of recent chats
    const chatSummary = chatTasks.map((t) =>
      `[${t.assignedAgent ?? 'unknown'}] User: ${t.description}\nAgent: ${t.result ?? '(no response)'}`
    ).join('\n---\n');

    const existingTasks = this.taskGraph.getAllTasks()
      .filter((t) => t.status !== 'completed' && t.status !== 'failed')
      .map((t) => `- ${t.title} (${t.status}, assigned to ${t.assignedAgent ?? 'user'})`)
      .join('\n');

    try {
      const response = await this.llm.ask(
        `Review these recent conversations and identify any action items, commitments, or follow-ups that were mentioned but do NOT already exist as tasks.

RECENT CHATS:
${chatSummary}

EXISTING OPEN TASKS:
${existingTasks || '(none)'}

For each missed item, output a JSON array of objects with "title" and "assignedTo" ("agent" or "user") fields. If nothing was missed, output an empty array [].
Only output the JSON array, nothing else.`,
        {
          model: 'fast',
          system: 'You are a task extraction system. Identify action items from conversations. Be conservative — only flag clear commitments or requests, not vague mentions.',
        },
      );

      // Parse LLM response
      const newTasks: Task[] = [];
      try {
        const items = JSON.parse(response);
        if (Array.isArray(items)) {
          for (const item of items) {
            if (item.title && typeof item.title === 'string') {
              const task = this.taskGraph.createTask({
                title: item.title.slice(0, 100),
                description: 'Detected by reflection review',
                source: 'reflection',
                metadata: { type: 'action_item', detectedBy: 'llm' },
              });
              task.assignedTo = item.assignedTo === 'agent' ? 'agent' : 'user';
              newTasks.push(task);
            }
          }
        }
      } catch {
        // LLM response wasn't valid JSON — no tasks extracted
      }

      const summary = newTasks.length > 0
        ? `Found ${newTasks.length} missed action item${newTasks.length > 1 ? 's' : ''} in ${chatTasks.length} recent chat${chatTasks.length > 1 ? 's' : ''}.`
        : `Reviewed ${chatTasks.length} recent chat${chatTasks.length > 1 ? 's' : ''}. No missed items.`;

      this.bus.publish('reflection.completed', 'reflection', {
        reviewedTasks: chatTasks.length,
        newTaskCount: newTasks.length,
        summary,
      });

      return { reviewedTasks: chatTasks.length, newTasks, summary };
    } catch (err) {
      // LLM call failed — fall back to basic review
      return this.basicReview(chatTasks);
    }
  }

  diagnose(): DiagnosticResult {
    return {
      module: 'reflection',
      status: 'healthy',
      checks: [
        {
          name: 'Last review',
          passed: true,
          message: this.lastReviewAt
            ? `Last reviewed at ${this.lastReviewAt.toISOString()}`
            : 'No reviews yet',
        },
        {
          name: 'LLM available',
          passed: this.llm.isConfigured,
          message: this.llm.isConfigured
            ? 'LLM-powered review enabled'
            : 'Using basic pattern matching (no LLM)',
        },
      ],
    };
  }
}
