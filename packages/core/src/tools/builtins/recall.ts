/**
 * recall — Tier 1 (auto)
 *
 * Searches the agent's persistent memory for relevant information.
 * Uses FTS5 keyword search with LIKE fallback.
 */

import type { FortTool, ToolResult } from '../types.js';
import type { AgentMemoryStore, MemoryCategory } from '../../memory/agent-memory-store.js';

export interface RecallInput {
  query: string;
  category?: MemoryCategory;
  limit?: number;
}

export function createRecallTool(store: AgentMemoryStore, agentId: string): FortTool {
  return {
    name: 'recall',
    description: 'Search your persistent memory for relevant information.',
    inputSchema: {
      type: 'object',
      properties: {
        query: { type: 'string', description: 'Keyword query to search memory.' },
        category: {
          type: 'string',
          enum: ['fact', 'decision', 'preference', 'observation'],
          description: 'Optional category filter.',
        },
        limit: { type: 'number', description: 'Max results (default 10).' },
      },
      required: ['query'],
    },
    tier: 1,

    async execute(input: unknown): Promise<ToolResult> {
      const { query, category, limit } = input as RecallInput;

      if (!query || typeof query !== 'string') {
        return { success: false, output: '', error: 'recall requires a "query" string.' };
      }

      const memories = store.recall(agentId, query, { category, limit });

      if (memories.length === 0) {
        return { success: true, output: 'No matching memories found.', artifacts: [] };
      }

      const lines = memories.map((m) => {
        const tags = m.tags.length > 0 ? ` [${m.tags.join(', ')}]` : '';
        return `- [${m.category}]${tags} ${m.content} (${m.createdAt.slice(0, 10)})`;
      });

      return {
        success: true,
        output: `Found ${memories.length} memory entry(ies):\n${lines.join('\n')}`,
        artifacts: memories,
      };
    },
  };
}
