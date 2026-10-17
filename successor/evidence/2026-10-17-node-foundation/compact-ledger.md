# Compact acceptance and dispatch records

[Spec 059](../../../specs/059-modular-node-v1.md) governs this change. Fresh
acceptance records persist the immutable receipt and request fingerprint. They
do not persist a conversation snapshot. Submission retry and receipt lookup keep
the original receipt and project current state. Pinned run configuration remains
unchanged when the owner edits conversation memory after acceptance.

Dispatch persists one record per service attempt with the input message identity,
input/configuration/history digests and historical sequence bounds. It does not
store another copy of message bodies or full history. Existing message and run
records remain the source of replay and dispatch input.

The public API regression ran 200 turns with 400 one-KiB synthetic messages.
Before removing copied dispatch history, the database occupied 54,120,448 bytes.
Afterwards it occupied 1,413,120 bytes and passed the four-MiB test limit. The test
restarted the service, replayed all messages and recovered the first immutable
receipt. These are single regression measurements, not the planned ten-thousand
message performance benchmark or latency/energy results.

All 64 affected service, settings, owner-pairing and shutdown tests passed with
zero failures/skips. Strict TypeScript checking passed. Two old assertions expected
stale snapshots; the revised tests require the immutable receipt plus current
snapshot and independently retain pinned dispatch configuration assertions.

Historical request records remain readable through their earlier receipt shape.
No historical ledger was opened or compacted. Existing duplicated historical
records still consume space until an explicit copied-ledger migration proves
backup, restart and rollback. Indexed bounded paging, outbox recovery and full
foundation acceptance remain open. No live provider or external account was used.
