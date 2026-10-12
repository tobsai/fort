# Diagnostic correction and held recovery

The owner approved one temporary synthetic isolation experiment at 21:20 UTC on
2026-10-12, message identifier `Sentinel_7f07a5939a408191a8e7036c9ec04dbb`, against
commit `95ab4a250dc4b8dce0452d8054e2f4a7dc09a509` and seal
`bc6aa4b100bb6792030d359fd71290f6a824c5a48f9da4ddcff3e3d4be4904c2`.
The original [approval scope](APPROVAL-SCOPE.md) stays byte-identical. Authority is
recorded separately in [the execution evidence](../../evidence/isolation-execution-2026-10-12/authority.json).
Its historical draft wording is not a new permission request or a claim that
execution was never approved.

That run stopped before code execution when the eighth download, `ubuntu.pub`,
failed its exact-byte pin. Commit `28f76cf49bdf686177204c9599a178caa55d188a` preserves
the failure and verified cleanup. No manager, VM, keys or synthetic grant started.
Do not amend or reset that evidence.

The independent [integrity review](../../../reviews/ubuntu-key-28f76cf/REVIEW.md)
reports fifteen fresh public exports matching the original 3,325-byte body and
SHA-256 `927138de292fac4d861105a052cf56e9c5921987da385c844d88f34d19b39ab0`, with the
unchanged official primary fingerprint
`D2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81`. It cannot classify the historical response
because that body and digest were discarded. It does not cryptographically verify
the publisher. Preserve the original pin; no rotation or compromise is established.

The parent delegated only the diagnostic repair, tests, resealing and review
handoff for this increment. Revision 4 retains all original download entries,
domains, primary fingerprints, dependency integrity, offline verifier controls,
publisher negative controls, VM resource/network/identity limits and single-session
restriction. `scope.json` remains blocked with `execution_authorized: false`.
There is no execution retry in this preparation.

Revision 3 (`e1b7ec8be8ddc323bef159a647a9c8eab4f73b9c`, seal
`bc79f94b93ba163eae3662ea4091d50ffcb9140a6bfc94d39e2aaca0e3adfc38`) was rejected
by [the ordering review](../../../reviews/download-diagnostics-e1b7ec8/REVIEW.md).
It removed raw mismatch responses before persisting diagnostics and recorded
acceptance before fallible partial cleanup and target-directory synchronization.
Three review fault tests first reproduced these failures. Revision 4 holds the
response until failure evidence is durable and commits acceptance last.

The correction writes exclusive, bounded, fsynced failure observations and terminal
receipts plus an optional inert failed public-key prefix under the approved root.
Failure observations always veto acceptance; terminal failed receipts add cleanup
results without overwriting observations. Accepted terminal receipts follow all
download-file cleanup and directory synchronization. Caught receipt-publication
errors roll back only their own files. Failed evidence persistence preserves the
raw response as a partial, revoking any promoted final name when possible; any
unreconciled state stays blocked. Interruption tests model process stops, not power
loss. See the exact ordering and authority rules in [RUNBOOK.md](RUNBOOK.md).
The 256 KiB maximum is part of the existing 128 MiB evidence allowance, not a new
resource grant. Up to thirteen 8 KiB records replace the prior twelve 16 KiB
records; the optional 64 KiB body and bounded pending publications still fit the
same cap. A failure still stops; no receipt/body is a trusted input, no body is
executed, and receipt-write failure never permits the next request.

Before a later recovery:

1. Independently review the corrected commit and its exact new seal. Verify that
   original pins and trust/resource/credential scope remain unchanged.
2. Record the reviewed recovery separately against the preserved original owner
   authority and closed failure record. Repeat fresh ownership/resource/port
   preflight; do not reuse review-body copies, failed partials or old evidence roots.
3. Run every original download, registry, archive, publisher and negative-control
   gate before its corresponding code execution. A repeat mismatch stops for
   diagnosis with the new bounded evidence; it is not an automatic retry loop.
4. Permit only the originally approved single synthetic session if all remaining
   gates pass. No replacement grant or broader fault matrix is included. Preserve
   evidence, verify termination and remove only inventoried owned resources.

A different publisher identity, verifier, payload version, trust source/domain,
weaker verification policy, expanded privilege/path/resource/credential scope or
additional synthetic attempt is outside this correction. Existing unchanged
authority should not be requested again; material changes still require a decision.
