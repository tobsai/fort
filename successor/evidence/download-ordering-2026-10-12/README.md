# Diagnostic ordering correction — held for independent review

Revision 4 corrects the two P2 ordering defects reproduced by
[the review of e1b7ec8](../../../reviews/download-diagnostics-e1b7ec8/REVIEW.md).
All three review fault cases failed before the change and now pass. No public
download, downloaded-code execution, provisioning retry, VM, key generation or
synthetic session occurred. Recovery remains held for independent review.

Final 31-file packet seal:

`31e22d8e99e3141e9083f7a2246c15469d32362279cf537c56aa35911936e406`

Base commit: `e1b7ec8be8ddc323bef159a647a9c8eab4f73b9c`.
Worktree: `/tmp/fort-core-openmaus-20261011` (canonical `/private/tmp/...`).
Branch: `codex/fort-core-openmaus-20261011`.

The original owner approval remains the one recorded against `95ab4a2` / seal
`bc6aa4b100bb6792030d359fd71290f6a824c5a48f9da4ddcff3e3d4be4904c2`.
The parent delegated this correction, tests, resealing and review handoff only.
The failed run at `28f76cf` and rejected correction at `e1b7ec8` remain intact in
history. Unchanged infrastructure authority is preserved; no replacement synthetic
attempt or execution retry is authorized here. See [RECOVERY.md](../../ops/isolation-probe-v1/RECOVERY.md).

## Ordering changes

On failure, the downloader now fsyncs the bounded untrusted body and an exclusive
`NN-name.failure.json` observation **before deleting the raw response**. It then
attempts owned cleanup and writes a separate failed terminal `NN-name.json` with
the cleanup result. A stop between these phases retains the earlier evidence;
missing terminal data means cleanup is unresolved. A failure observation always
vetoes acceptance. No observation is overwritten by a later outcome.

On success, the downloader finishes payload fsync, exclusive final publication,
partial removal and downloads-directory fsync **before recording acceptance**.
It retains the owned-file identity until the accepted receipt has been published
and synced. Diagnostic records first use an exclusive pending file, fsync its
contents, publish without replacement, remove the pending alias and fsync the
diagnostic directory. Caught publication errors roll back only their owned files.

If failure-evidence persistence itself fails, the raw response is preserved. A
previously promoted matching input is restored to a partial hard link before its
final name is revoked, where those operations succeed. Failed rollback or cleanup
stays unreconciled and blocked. No failure starts another request. Acceptance
requires a clean stage exit, all twelve accepted input/receipt pairs, and no
failure observations, pending publications or contradictory records. File presence
alone is not authorization for verifier or manager execution.

The diagnostics allocation changes from twelve 16 KiB records to at most thirteen
8 KiB records: up to eleven successful receipts plus one failure observation and
one failed terminal record. There is only one failed pair because failure stops
the sequence. The optional public-key prefix remains at most 64 KiB. Temporary
publication aliases are hard links; their bounded logical/allocated sizes remain
within the unchanged **256 KiB diagnostic cap** and **128 MiB evidence allowance**.
No original payload pin, URL, fingerprint, dependency, host path, resource,
network policy, credential class, verifier restriction or guest source changed.

## Red and green evidence

The three adapted review tests run through the downloader entrypoint with the
existing in-memory urllib HTTPS fixture. They write no parent review artifact:

| Review fault | Before | After |
| --- | --- | --- |
| Mismatch interrupted before first evidence write | Raw response absent; no retained diagnostic | Raw partial remains |
| Matching input followed by partial-unlink failure | Durable accepted receipt despite failed cleanup | Failed observation and terminal; cleanup error retained; final input absent |
| Matching input followed by target-directory fsync failure | Accepted receipt and final input remain | Failed receipts; no accepted JSON or final input |

[review-red.log](review-red.log) and [review-red.json](review-red.json) retain the
three observed failures against the rejected source. The two success-path tests
also inspect every raw JSON record to reject an accepted receipt; they do not rely
only on the fixture's projected outcome.

Eight additional boundaries cover acceptance-file and acceptance-directory fsync
failure, persistent evidence-storage failure for matching and mismatching inputs,
interruption between body and observation writes, interruption before failure
cleanup, interruption before the failed terminal write, and failure at the twelfth
input with thirteen receipts within the unchanged cap. SystemExit injection models
a process stop at a boundary; no physical power-loss test is claimed.

| Check | Result |
| --- | --- |
| Downloader behavior retained from revision 3 | 18 passed |
| Review faults and adjacent ordering boundaries | 11 passed |
| Packet policy | 16 passed |
| Readiness/controller/SSH parser | 9 passed |
| Verifier regressions | 13 passed |
| Total | **67 passed, zero failures** |
| Seal, embedded config/source, syntax | Passed, 31 sealed files |
| Shell/Node syntax and independent YAML/JSON agreement | Passed |
| Execution guard | Intended blocked exit **2** |
| Original security/resource scope | Exact equality / protected source bytes unchanged |
| Host task roots | Both remain absent; read-only lstat check |

Full commands/logs are in [commands.json](commands.json); tightened final raw-receipt
assertions and final seal/guard checks are in [final-checks.json](final-checks.json).
The tests bind no listener and make no public HTTP request. Existing Node verifier
tests use built-in crypto for retained public registry metadata and a labeled
OpenPGP contract double; the private OpenPGP.js package is not loaded.

[scope-comparison.json](scope-comparison.json) records the unchanged original scope
and diagnostic allocation difference. [review-reference.json](review-reference.json)
pins the review and reproductions read. [roots-absent.json](roots-absent.json) records
the host-root check. [validation.json](validation.json) binds the final evidence,
packet and preserved historical artifacts.

## Remaining gate

Independently review the exact corrected commit and seal before recording a recovery
under the existing owner approval. Fresh preflight and all original integrity,
registry, publisher, negative-control, code-signature and guest-readiness checks
remain required. Any repeat mismatch stops with evidence; no automatic retry,
pin change, review-body reuse or replacement grant is included.

Public download recovery, actual publisher verification, VM/guest execution, the
single synthetic session and T07 remain unrun. Official providers stay disabled.
No push, deployment, UI session, new dependency, provider cleanup or permission
expansion occurred. Parent-owned `reviews/`, historical evidence and accepted
product/native/guest implementation were not edited.
