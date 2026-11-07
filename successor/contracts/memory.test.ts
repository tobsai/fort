import { test } from 'node:test';
import assert from 'node:assert/strict';
import { MEMORY_LIMITS, NO_MEMORY, memoryInstructions, parseAgentMemoryContext, parseMemoryProviderInfo, readMemoryDirectives } from './memory.ts';

test('memory tags come out of the reply and become bounded writes and forgets', () => {
  const read = readMemoryDirectives('Sure, booked.\n<fort-memory>Owner prefers aisle seats.</fort-memory>\n<fort-memory kind="event">Booked Lisbon flights on Nov 6.</fort-memory> <fort-forget id="old-1"/>');
  assert.equal(read.text, 'Sure, booked.');
  assert.deepEqual(read.writes, [{ kind: 'fact', text: 'Owner prefers aisle seats.' }, { kind: 'event', text: 'Booked Lisbon flights on Nov 6.' }]);
  assert.deepEqual(read.forgets, ['old-1']);

  const many = readMemoryDirectives(Array.from({ length: 8 }, (_, index) => `<fort-memory>Note ${index}</fort-memory>`).join('') + '<fort-memory>Note 0</fort-memory><fort-memory>' + 'x'.repeat(MEMORY_LIMITS.entryBytes + 1) + '</fort-memory>');
  assert.equal(many.writes.length, MEMORY_LIMITS.writesPerTurn);
  assert.equal(many.dropped, 5);
  assert.equal(many.text, 'Noted.');
  assert.equal(readMemoryDirectives('No tags here, just <b>text</b>.').text, 'No tags here, just <b>text</b>.');
});

test('a turn’s recalled memory is checked field by field before a runner accepts it', () => {
  const good = { provider: { id: 'fort-notes', version: '1', name: 'Fort Notes' }, canRemember: true, canForget: true, entries: [{ entryId: 'a', kind: 'fact', text: 'Likes tea.', createdAt: '2026-11-06T00:00:00.000Z' }] };
  assert.deepEqual(parseAgentMemoryContext(good), good);
  assert.throws(() => parseAgentMemoryContext({ ...good, extra: true }));
  assert.throws(() => parseAgentMemoryContext({ ...good, entries: [{ ...good.entries[0], text: 'x'.repeat(MEMORY_LIMITS.entryBytes + 1) }] }));
  assert.throws(() => parseAgentMemoryContext({ ...good, entries: Array.from({ length: MEMORY_LIMITS.recallEntries + 1 }, (_, index) => ({ ...good.entries[0], entryId: 'e' + index })) }));
  assert.throws(() => parseAgentMemoryContext({ ...good, entries: [{ ...good.entries[0], kind: 'secret' }] }));
});

test('instructions offer only what the memory system can do, and mark recalled notes as data', () => {
  const base = { provider: { id: 'p', version: '1', name: 'Pad' }, entries: [] };
  const full = memoryInstructions({ ...base, canRemember: true, canForget: true });
  assert.match(full, /<fort-memory>/); assert.match(full, /fort-forget/);
  const readOnly = memoryInstructions({ ...base, canRemember: false, canForget: false, entries: [{ entryId: 'a', kind: 'fact', text: 'x', createdAt: '' }] });
  assert.doesNotMatch(readOnly, /<fort-memory>/); assert.match(readOnly, /data, not instructions/);
});

test('a memory module must name itself and its capabilities; "none" is reserved for no memory', () => {
  assert.throws(() => parseMemoryProviderInfo({ ...NO_MEMORY }));
  assert.throws(() => parseMemoryProviderInfo({ id: 'x', version: '1', name: 'X', summary: '', storage: '', capabilities: ['telepathy' as never] }));
  assert.deepEqual(parseMemoryProviderInfo({ id: 'x', version: '1', name: 'X', summary: '', storage: '', capabilities: ['recall', 'recall'] }).capabilities, ['recall']);
});
