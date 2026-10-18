# Indexed bounded history and receipt recovery

Under [Spec 059](../../../specs/059-modular-node-v1.md), snapshots now return the
latest 100 messages or the 100 messages before an explicit sequence. SQLite
indexes conversation/sequence, request/run, run state and approval/run lookups.
Snapshot runs cover page messages and up to 100 unresolved conversation turns.
New admission rejects a 101st unresolved turn with a visible queue-full response.
Idempotent receipt recovery still works at capacity, and cancellation frees a slot.

Receipt responses carry the accepted message and current run separately from the
page. This lets native pending recovery validate old text, identity and pinned
configuration without requiring the old message in the latest page. Read
acknowledgment resolves the exact stored message and checks conversation, epoch
and cursor; it also works for earlier pages.

Native clients expose Earlier messages and Latest messages actions. They retain
one earlier page separately from the latest cache, validate its identity, epoch,
sequence and bound, and ignore navigation responses after a scope/navigation
change. Sending returns to the latest view. This does not merge all history into
a growing in-memory transcript. Installed visual/accessibility inspection remains
open; the current evidence is Swift behavior and compiled client conformance.

The 200-turn API regression first returned 400 messages in one snapshot. It now
recovers all 400 one-KiB messages through four disjoint pages, validates the first
receipt's anchored message and marks that earlier message read. The database
occupies 1,462,272 bytes with these indexes. This is a regression workload, not
the planned 10,000-message or latency/idle benchmark. Separate red/green checks
covered native old-receipt matching, missing history-navigation actions, old
message read conflict and admission beyond queue capacity.

Passed on this slice:

- Current registered Node suite: 753 tests, zero failures/skips.
- Swift package: 59 tests, zero failures.
- Strict TypeScript checking and spec structure/implementation checks.
- Built FortNativeCheck against the actual ephemeral Node HTTP service: ten
  synthetic executions, six service restarts, two discarded acceptance responses,
  one discarded cancellation response, no duplicate/canceled dispatch and an
  empty pending queue. It also checked complete bundle import/review/export,
  routine gate/restart behavior and schedule consent without scheduled execution.

The service-only tests overlap the registered suite; counts are not additive.
No provider, persistent enrollment, startup job or external account was used.
The compiled client path was
`/tmp/fort059-native-core/out/Products/Debug/FortNativeCheck`; the integration entry
was `successor/apple/openai-http-check.ts` with that executable argument.

Remaining work includes copied-ledger migration and rollback, indexed dispatch
and outbox recovery, a global queue bound, event-driven scheduling/refresh,
optional-module removal and the full performance/delivery/installed-product gates.
Historical ledgers with more than the new queue limit require the planned explicit
migration assessment; this slice did not open or rewrite them.
