# Offline compact-ledger migration

Spec 059 implementation under the recorded Node decision. No real ledger,
provider, credential store or consumed demo was opened. All inputs were synthetic.

The CLI `node successor/service/migrate-ledger.ts --source /absolute/ledger.sqlite
--destination /absolute/new-directory` uses Node SQLite backup for a consistent
copy. Run it with the source service stopped. It never starts either service.
The source remains the rollback artifact; use only one selected installation.
It creates a private directory/file, refuses existing destinations and symlink
source files, validates schema/integrity and canonical receipt/history references,
then compacts duplicates and vacuums the copy. A failure removes only the directory
created by that attempt. Output contains counts, sizes and a snapshot digest.

Legacy contexts retain input/configuration/history digests and a reconstructable
message window. They are explicitly labeled historical; the tool does not invent
an attempt identity or generation absent from that record. Runtime bindings,
outcomes, authority/expiry records and provider identity are preserved. This is
not a provider migration, credential renewal or a safe way to run two copies.
Actual private-data migration requires its applicable access authority.

Validation on Node 26.5.0:

- Red: migration CLI absent; the real copy/replay test failed at that command.
- Green: 2 CLI/public-service tests passed, zero skips. Thirty synthetic turns
  replayed identical receipts/messages from the compacted copy and original.
  Duplicate submission dispatched no work. The source remained byte-identical
  throughout copying and destination operation. Re-copy compacted zero records.
- The copy was less than one third of the seeded legacy ledger size. This is a
  regression assertion, not a controlled performance benchmark.
- Existing destination, unknown schema and corrupt receipt digest were rejected;
  incomplete owned copies were absent and the source bytes were unchanged.
- Strict TypeScript check and structural/implementation spec gates passed.

Commands: `node --test successor/service/migrate-ledger.test.ts` and
`npm --prefix successor run typecheck`. Logs are in
`/tmp/fort059-migration-final.log` and `/tmp/fort059-migration-typecheck.log`.
Concurrent writers, interrupted OS termination, real-data adoption and provider
binding upgrades were not exercised. Copying does not itself authorize activation.
