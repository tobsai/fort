// A memory module shaped unlike Fort Notes, for tests only: process memory, newest-only recall, no search, no export or
// import. It proves the service works through the contract and not through Fort Notes' particulars (ARCHITECTURE.md §1).
import type { MemoryEntry, MemoryProvider } from '../../../contracts/memory.ts';

export function createScratchpadMemory(options: { id?: string; failRecall?: () => boolean; beforeRecall?: () => Promise<void> } = {}): MemoryProvider & { entries: Map<string, MemoryEntry[]> } {
  const entries = new Map<string, MemoryEntry[]>();
  let next = 0;
  const of = (agentId: string) => entries.get(agentId) ?? [];
  return {
    protocolVersion: 1, id: options.id ?? 'scratchpad', version: '0.1', name: 'Scratchpad', summary: 'Keeps the last few notes in process memory.', storage: 'Nothing on disk.',
    capabilities: ['remember', 'recall', 'forget'], entries,
    async recall({ agentId, limit }) { await options.beforeRecall?.(); if (options.failRecall?.()) throw new Error('down'); return of(agentId).slice(-limit).reverse(); },
    async remember({ agentId, writes }) {
      const added = writes.map(write => ({ entryId: 'pad-' + ++next, agentId, kind: write.kind, text: write.text, createdAt: new Date(0).toISOString(), ...(write.source ? { source: write.source } : {}) }));
      entries.set(agentId, [...of(agentId), ...added].slice(-3)); return added;
    },
    async forget({ agentId, entryIds }) { const before = of(agentId).length; entries.set(agentId, of(agentId).filter(entry => !entryIds.includes(entry.entryId))); return before - of(agentId).length; },
    async list({ agentId, limit }) { return { entries: of(agentId).slice().reverse().slice(0, limit), total: of(agentId).length }; },
  };
}
