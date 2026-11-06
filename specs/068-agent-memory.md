# 068: Agent memory, swappable per Agent

**Decision:** Toby asked on 2026-11-06 ("start working on an agentic memory system... modular (in the sense that it is
swappable for a different system). I should be able to see the system I've chosen for each agent and be able to swap
between agents", Fort channel). Merging is his.

## What and why

Agents forget everything between conversations; the only memory today is a note the owner types per chat. After this
change each Agent has a **memory system**. The Agent decides what to keep: it adds
`<fort-memory>one sentence</fort-memory>` to a reply (or `kind="event"` for something that happened), and Fort strips
the tag, so the owner sees only the answer. Before each turn Fort asks the memory system what it recalls for that
message and hands it to the runtime as data. The Agent can drop a stale note it was shown with `<fort-forget id="…"/>`.

The memory system sits behind a Fort-owned contract (`contracts/memory.ts`), so another one can replace it. Each Agent
names its own; the owner can see every Agent's choice, see and search what it holds, add or forget a note, and switch.

- **Fort Notes** (`modules/memory/notes.ts`), the default on the hub: short notes in one SQLite file of its own
  (`memory/fort-notes.sqlite` beside the ledger), recalled by word overlap, facts before events, newest first. No
  embeddings, no network, no paid service. At most 1000 notes an Agent; past that the oldest events go first.
- **No memory**: the Agent starts every conversation fresh. Needs no module.

## Risk

- **Billing/provider path.** None. Recall is local; the recalled notes ride inside the existing turn (at most 12 notes,
  4 KB), on the Agent's own runtime and account.
- **Credentials.** None. The instructions tell Agents not to store secrets; Fort doesn't check note contents.
- **Persistent data.** Notes outlive conversations by design. They live with the memory module, not the ledger:
  deleting history leaves them, and removing the module is deleting its file. Switching an Agent to another system
  keeps the old system's notes where they are, so switching back finds them; "bring notes along" copies them only
  when both systems can export and import.
- **Destructive.** Forgetting a note is the owner's explicit action, or the Agent's for a note it was shown that turn.
- **Removal.** A missing module shows as missing on the Agent and stops that Agent's sends with a reason
  ("pick another memory system") instead of quietly turning memory off. History stays readable; re-adding the module
  brings the notes back. A module that fails to recall fails the turn before anything is sent; one that fails to save
  never loses the reply.
- **Mac runners** need this build: a hub turn now carries a new `agentMemory` field that older Mac executors refuse.
  Merging builds both apps for TestFlight.

## Plan

Contract: `MemoryProvider` (recall, remember, forget, list; optional export/import) with declared capabilities, and the
tag protocol, which works with every runtime that returns text. The service composes modules it is given (the hub gives
Fort Notes); each Agent's choice is a `memoryChoices` row, saved for every Agent at start so a later removal is visible.

API: `GET /v1/memory` (systems on offer, every Agent's choice and note count), `GET /v1/agents/{id}/memory[?query=]`,
`POST /v1/agents/{id}/memory/select {expectedRevisionId, providerId, carry?}`, `.../remember {text, kind?}`,
`.../forget {entryIds}`. The picker goes in each Agent's page on web, Mac and iPhone after the Setup rework (PR 90).

Rejected: embeddings or a vector store (a paid or heavy dependency before word overlap is shown to fall short);
letting the runtime write memory through tools (runtime-specific, and the tools are off by policy); storing notes in
the ledger (the module wouldn't be removable). Tests run the same behaviour suite against Fort Notes and an
independently shaped in-process fixture, and cover write, recall, switch, switch back, carry, no memory, a removed
module and a failing one.
