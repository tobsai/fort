# Preserve routine compiler identity across future runs

Design: [Spec 059](../../../specs/059-modular-node-v1.md). Follow-up to the
[optional context module](context-modules.md).

The first module slice pinned a compiler on each accepted Run. A saved routine
still chose the current compiler at each new acceptance. A failing public test
demonstrated a due occurrence being accepted after the compiler changed from
version 1 to version 2. That could change future standing work without revising
the saved routine.

Context-bearing routine revisions now store the compiler ID/version inside their
digest. Manual and scheduled admission preserve that pin instead of replacing it.
After a compiler change, manual invocation fails and a due occurrence records a
blocked decision without accepting or executing work. Saving a new routine
revision explicitly selects the current compiler and changes the digest. Existing
schedules keep their earlier routine revision and consent until separately edited
and reviewed.

Historical routine revisions without a pin remain readable with their original
digest format. A context-bearing unpinned routine requires a newly saved revision
before invocation. Context-free routines need no compiler. Library authoring
remains possible while the module is absent; it does not grant execution authority.
Prior accepted request receipts retain their original idempotency behavior.

On October 18, 2026:

- The public manual/scheduled regression failed before the correction and passed
  after it, using synthetic time and a fixture runtime.
- All 9 module tests passed, including two blocked daily occurrences, explicit
  routine revision, unchanged schedule revision, and readable unpinned history
  followed by refused invocation.
- All 827 discovered portable tests passed, with no failures or skips.
- Strict TypeScript passed.
- The compiled native HTTP journey passed through ten synthetic executions,
  six service restarts, lost responses and cancellation without duplicates.

No provider, credential, real schedule or persistent service was activated. Native
installed presentation and real scheduled provider execution remain separate
acceptance gates. The context artifact from the preceding slice remains preserved;
it predates this routine-pinning correction.
