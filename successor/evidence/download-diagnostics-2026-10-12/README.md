# Download diagnostics correction — held for independent review

The downloader now retains bounded failure evidence instead of discarding the
observed digest and response. This is source preparation only. No public download,
provisioning retry, downloaded-code execution, VM, key generation or synthetic
session occurred during this correction.

The original owner approval remains attached to commit
`95ab4a250dc4b8dce0452d8054e2f4a7dc09a509` and seal
`bc6aa4b100bb6792030d359fd71290f6a824c5a48f9da4ddcff3e3d4be4904c2`.
The closed failed run and its cleanup evidence remain unchanged at `28f76cf`.
The parent's new delegation authorizes this diagnostic repair, resealing and
review handoff, with **no execution retry yet**. See the packet's
[recovery record](../../ops/isolation-probe-v1/RECOVERY.md).

Revision 3 seal, covering 30 files:

`bc79f94b93ba163eae3662ea4091d50ffcb9140a6bfc94d39e2aaca0e3adfc38`

Worktree: `/tmp/fort-core-openmaus-20261011` (canonical `/private/tmp/...`).
Branch: `codex/fort-core-openmaus-20261011`. Only the nine packet files listed in
the scope comparison and this new evidence directory changed. Parent-owned
untracked `reviews/`, historical execution evidence, accepted product code,
native clients, donor quarantine and existing specs were not edited.

## Established problem and correction

The [independent integrity review](../../../reviews/ubuntu-key-28f76cf/REVIEW.md)
reports fifteen fresh Ubuntu exports matching the **original** 3,325-byte hash and
official primary fingerprint. The historical mismatch remains unexplained because
the failed response was discarded. No key rotation, compromise or publisher
verification result is inferred. [review-reference.json](review-reference.json)
pins the review document read for this correction; no review response was reused
as a download input.

The unchanged downloader first failed the new behavior test: a changed body
stopped the request sequence but left zero receipts. The correction now:

- Persists each completed/failed input receipt before another request. Receipts
  contain expected/observed digests, observed byte count, complete/incomplete state,
  input identity, HTTP status, bounded type/length headers and safe final URL.
  Redirect query strings are omitted unless the response URL is exactly the
  original approved public URL. No cookies or authentication headers are captured.
- Keeps at most a 64 KiB failed public-key prefix, also capped by that input's
  original bound, as a private, non-executable `*.untrusted.bin`. Other payloads
  retain metadata only. No response is rendered, imported, normalized or promoted
  after a failure.
- Writes exclusive files through held directory descriptors, refuses existing
  downloads/partials/diagnostic directories and symlinks, fsyncs new directory
  entries and receipts, and publishes verified downloads without replacing an
  existing file or making a second payload copy.
- Stops on mismatch, HTTP error, truncation, disallowed redirect, size overflow,
  receipt failure or cleanup failure. A receipt-write failure attempts exact owned
  input removal and emits bounded metadata to stderr; cleanup errors remain visible
  in the failure receipt. Remaining partials require reconciliation, never reuse.

At most twelve 16 KiB receipts plus one 64 KiB failed key prefix occupy 256 KiB in
`FortIsolation/evidence/download-diagnostics`, within the already approved 128 MiB
raw/copied-evidence allowance. A streamed overflow observes at most one extra byte
but retains neither an oversized partial nor an oversized prefix. Declared
oversized bodies are rejected without reading them. Existing network and payload
caps, TLS/redirect policy and every cryptographic gate remain in force.

## Verification

| Check | Result |
| --- | --- |
| Original diagnostic-loss regression | Failed first: expected one receipt, found zero; now passed |
| Overflow-prefix and truncation regressions | Two failures recorded, then fixed |
| Diagnostic bound / automatic-retry policy regressions | Two failures recorded, then fixed |
| Cleanup-error diagnostic regression | Failed first, then fixed; failure metadata survives cleanup rejection |
| Downloader behavior | **18 passed** |
| Packet policy | **16 passed** |
| Readiness/controller/SSH parser regressions | **9 passed** |
| Verifier regressions | **13 passed** |
| Total | **56 passed, zero failures** |
| Final seal, embedded sources/config and source syntax | Passed, 30 sealed files |
| Shell and Node syntax; independent YAML/JSON agreement | Passed |
| Execution guard | Intended blocked exit **2** |
| Original pins and authority/trust/resource scope | Exact equality passed |
| Host task roots | Both remain absent; read-only `lstat` check |

The HTTP tests invoke the downloader entrypoint with a synthetic scope and an
in-memory urllib HTTPS transport. They exercise real urllib redirect/error handling
and temporary-file behavior without binding a listener or contacting any server.
Coverage includes successful SHA-256/SHA-512 inputs, later failure preserving prior
receipts, changed armor bytes, HTTP-200 HTML, truncation, HTTP errors, declared and
streamed overflow, rejected/allowed redirects, query redaction, existing files and
directories, symlink refusal, receipt-write failure and cleanup failure. The retained
test bodies are synthetic and removed with their temporary directories.

The existing verifier tests still use real built-in crypto for previously retained
public registry metadata and a labeled contract double for detached OpenPGP. They
do not load the private OpenPGP.js package. No publisher signature or OS-isolation
acceptance is claimed.

Commands and logs are in [commands.json](commands.json), with the final affected
Python/seal/guard rerun in [final-checks.json](final-checks.json). Initial red evidence
is `red.*`, `boundary-red.*`, `policy-red.*` and `cleanup-red.*`.
[scope-comparison.json](scope-comparison.json) verifies every unchanged original
scope field and byte-identical approval, verifier and guest sources.
[roots-absent.json](roots-absent.json) records the read-only root check.
[validation.json](validation.json) binds the final evidence hashes and packet seal.

## Review handoff

Independently review this exact corrected commit and seal before recording recovery
under the preserved owner authority. The failed run stays closed; no original pin
changes, response-body reuse, retry-until-success loop or replacement synthetic
attempt is authorized. A repeat mismatch must stop with the new diagnostic receipt.

Public download recovery, archive/registry/publisher checks, negative controls,
manager signing/Gatekeeper checks, VM/guest validation and the single synthetic
session remain unrun. T07 and official providers remain gated. No push, external
deployment, UI session, new dependency, permission expansion or provider cleanup
was performed.
