# Indexed durable outbox

Spec 059 slice under the recorded Node decision. The SQLite ledger now owns a
bounded outbox of unresolved work. Acceptance, claim, routine gates, cancellation,
unknown resolution and completion update the Run and outbox in one transaction.
Dispatch uses the ready index and earlier unresolved rows in the same conversation;
completed history is absent from that queue. Restart holds queued plain-text work
and preserves uncertain barriers while allowing another Home to proceed.

Admission caps unresolved work at 100 per conversation and 1,000 per installation.
Duplicate receipt recovery remains available when full; cancel/terminal resolution
releases capacity. The outbox has its own persistent integer sequence because
SQLite VACUUM may renumber implicit row IDs. Legacy backfill orders original Run
rows under the exclusive ledger lock and commits a version marker atomically.
The offline migration tool recognizes and preserves this table.

History assembly uses indexed user/request queries and one reply map. It no longer
searches the full message list separately for every earlier user message. Exact
message text, ordering, pinned configuration and service attempt identity remain.

- Red: the 1,001st unresolved acceptance returned 201 instead of 429.
- Green: 79 affected service/migration/settings/pairing/shutdown tests passed.
  Current, pre-outbox and compacted ledgers preserved held/unknown barriers after
  restart, allowed an independent Home and dispatched no uncertain work twice.
- Full registered Node suite: 762 passed, zero skips/failures.
- Strict TypeScript passed. Compiled Swift/HTTP conformance passed against this
  service with ten synthetic executions, six restarts, discarded responses,
  routine gates, cancellation and no duplicate/canceled execution.
- Structural/implementation gates passed. No real ledger/provider was opened.

Logs: `/tmp/fort059-outbox-red.log`, `/tmp/fort059-outbox-final.log`,
`/tmp/fort059-outbox-node-all.log`, `/tmp/fort059-outbox-typecheck.log`,
`/tmp/fort059-outbox-native-http.log`. Controlled 10,000-message performance and
process-boundary dispatch/evidence remain separate gates.
