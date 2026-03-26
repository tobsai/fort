/**
 * remember — Tier 1 (auto)
 *
 * Stores a piece of information in the agent's persistent memory.
 * Requires agentId to be passed via the closure (set at registration time).
 */

import type { FortTool, ToolResult } from '../types.js';
import type { AgentMemoryStore, MemoryCategory } from '../../memory/agent-memory-store.js';

export interface RememberInput {
  content: string;
  category: MemoryCategory;
  tags?: string[];
}

export function createRememberTool(store: AgentMemoryStore, agentId: string): FortTool {
  return {
    name: 'remember',
    description: 'Store a piece of information in your persistent memory across sessions.',
    inputSchema: {
      type: 'object',
      properties: {
        content: { type: 'string', description: 'The information to remember.' },
        category: {
          type: 'string',
          enum: ['fact', 'decision', 'preference', 'observation'],
          description: 'Category of the memory.',
        },
        tags: {
          type: 'array',
          items: { type: 'string' },
          description: 'Optional tags for easier retrieval.',
        },
      },
      required: ['content', 'category'],
    },
    tier: 1,

    async execute(input: unknown): Promise<ToolResult> {
      const { content, category, tags } = input as RememberInput;

      if (!content || typeof content !== 'string') {
        return { success: false, output: '', error: 'remember requires a "content" string.' };
      }
      const validCategories: MemoryCategory[] = ['fact', 'decision', 'preference', 'observation'];
      if (!validCategories.includes(category)) {
        return { success: false, output: '', error: `category must be one of: ${validCategories.join(', ')}` };
      }

      const memory = store.remember(agentId, { content, category, tags });
      return {
        success: true,
        output: `Memory stored (${category}): "${content}"`,
        artifacts: [memory],
      };
    },
  };
}
