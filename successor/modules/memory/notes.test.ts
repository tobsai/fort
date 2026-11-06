import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, stat } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { MemoryProvider } from '../../contracts/memory.ts';
import { createNotesMemory, memoryTerms } from './notes.ts';
import { createScratchpadMemory } from './fixtures/scratchpad.ts';

async function notes(t: TestContext, options: { maxEntriesPerAgent?: number } = {}) {
  const directory = await mkdtemp(join(tmpdir(), 'fort-notes-'));
  const make = () => createNotesMemory({ directory, ...options });
  const module = make();
  t.after(async () => { module.close?.(); await rm(directory, { recursive: true, force: true }); });
  return { module, directory, reopen: () => { module.close?.(); return make(); } };
}

// The behaviour every memory module owes Fort, run against two independently shaped modules.
const suite: [string, (t: TestContext) => Promise<MemoryProvider>][] = [
  ['Fort Notes', async t => (await notes(t)).module],
  ['Scratchpad fixture', async () => createScratchpadMemory()],
];
for (const [name, make] of suite) {
  test(`${name}: keeps each Agent's notes to that Agent, and forgets on request`, async t => {
    const module = await make(t);
    const [kept] = await module.remember({ agentId: 'ada', writes: [{ kind: 'fact', text: 'Ada drinks green tea.' }] });
    await module.remember({ agentId: 'bo', writes: [{ kind: 'fact', text: 'Bo drinks coffee.' }] });
    assert.ok(kept?.entryId);
    const recalled = await module.recall({ agentId: 'ada', query: 'what does Ada drink?', limit: 5, budgetBytes: 4096 });
    assert.deepEqual(recalled.map(entry => [entry.agentId, entry.text]), [['ada', 'Ada drinks green tea.']]);
    assert.equal((await module.list({ agentId: 'bo', limit: 10 })).total, 1);
    assert.equal(await module.forget({ agentId: 'bo', entryIds: [kept!.entryId] }), 0, 'one Agent cannot forget another’s note');
    assert.equal(await module.forget({ agentId: 'ada', entryIds: [kept!.entryId] }), 1);
    assert.deepEqual(await module.recall({ agentId: 'ada', query: 'tea', limit: 5, budgetBytes: 4096 }), []);
  });
}

test('Fort Notes recalls by matching words, facts first, within the byte budget', async t => {
  const { module } = await notes(t);
  await module.remember({ agentId: 'a', writes: [
    { kind: 'event', text: 'Planned the Lisbon trip with the owner.' },
    { kind: 'fact', text: 'The owner is planning a trip to Lisbon in March.' },
    { kind: 'fact', text: 'The owner’s daughter is called June.' },
    { kind: 'fact', text: 'The owner’s daughter is called June.' },
  ] });
  const recalled = await module.recall({ agentId: 'a', query: 'Any ideas for the Lisbon plans?', limit: 12, budgetBytes: 4096 });
  assert.deepEqual(recalled.map(entry => entry.text), ['The owner is planning a trip to Lisbon in March.', 'Planned the Lisbon trip with the owner.', 'The owner’s daughter is called June.']);
  assert.equal((await module.list({ agentId: 'a', limit: 10 })).total, 3, 'the same note is kept once');
  const tight = await module.recall({ agentId: 'a', query: 'Lisbon', limit: 12, budgetBytes: 60 });
  assert.equal(tight.length, 1);
  assert.deepEqual([...memoryTerms('Planning planned plans')], ['plan']);
});

test('Fort Notes lives in one private file that survives a restart, and trims old events before facts', async t => {
  const h = await notes(t, { maxEntriesPerAgent: 3 });
  await h.module.remember({ agentId: 'a', writes: [{ kind: 'fact', text: 'Fact one.' }, { kind: 'event', text: 'Event one.' }, { kind: 'event', text: 'Event two.' }] });
  await h.module.remember({ agentId: 'a', writes: [{ kind: 'fact', text: 'Fact two.' }] });
  const reopened = h.reopen(); t.after(() => reopened.close?.());
  assert.deepEqual((await reopened.list({ agentId: 'a', limit: 10 })).entries.map(entry => entry.text), ['Fact two.', 'Event two.', 'Fact one.']);
  assert.equal((await stat(join(h.directory, 'fort-notes.sqlite'))).mode & 0o777, 0o600);
  const exported = await reopened.export!({ agentId: 'a' });
  assert.equal(await reopened.import!({ agentId: 'b', entries: exported }), 3);
  assert.equal(await reopened.import!({ agentId: 'b', entries: exported }), 0, 'importing twice adds nothing');
});
