What this repo does: Fort is a single-owner native AI messenger with a local Node
core and optional runtime adapters. This slice records a read-only attribution
audit for the selected Codex binary. Expected load is one bounded packaging review.

## Semantic review

Codex self-review, October 18, 2026, base `36fc884`. Used the pinned
[Ponytail skill](https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md)
and preserved baseline. Reviewed the evidence note, manifest structure, collector
bounds/checksums, lock graph, selected notices/source headers, missing cases and
later-source comparisons. Verified the review archive's full membership and bytes.
No independent attestation, runtime implementation or dependency change is claimed.

## Must fix

1. **Published attribution is incomplete for the shipping Codex payload — open.**
   - What this is: The package retains root LICENSE/NOTICE; dependencies carry further terms.
   - Problem: This audit found many source notices outside those two root files and
     unresolved original notice/source correspondence for three exact crate versions.
   - Fix: Complete the documented mapping and covered-source availability, then include
     and verify the application notice payload. The collected candidates reduce that work.
   - If skipped: Distribution obligations remain unverified. Owner: implementation agent;
     this is not a reason to invent copyright text or demand generic owner approval.

## Should fix

2. **A conservative source graph can be mistaken for a binary inventory — addressed.**
   - What this is: Cargo.lock traversal includes tests, build tools and other platforms.
   - Problem: Treating all 1,337 entries as shipped would misstate enabled/runtime scope.
   - Fix: Both note and machine-readable manifest explicitly label their scope and limits.
   - If skipped: Readers could mistake preserved notices for activated product features.

Verdict: The audit is a concrete, reproducible research deliverable; final attribution
closure remains open. Continue local deletion/packaging work without shipping this
review artifact as an accepted application.
Not checked: exact binary build reproducibility, complete target/feature membership,
all source-header/legal nuances, final license payload, provider/account/signing or
installed/TestFlight acceptance. No source test rerun was necessary for evidence-only
changes; prior passed and failed source checks remain in their separate evidence.

```fort-review
{
  "format": 1,
  "reviewedCommit": "36fc884a0e4ae4a8ee08e5bfb706b7f16ae728f4",
  "reviewer": "Codex",
  "scope": "Read-only pinned Codex source-attribution research, exact audit manifest/archive, unresolved original notices, source-header and later-source evidence. No runtime/dependency implementation changes.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "findings": [
    {
      "id": "CODEX-NOTICE-CLOSURE",
      "disposition": "open",
      "detail": "Conservative source notices collected; original notice/source correspondence, covered-source availability and final application payload remain."
    },
    {
      "id": "SOURCE-GRAPH-SCOPE",
      "disposition": "fixed",
      "detail": "Both human and machine records distinguish conservative development/build/other-target source graph from a binary SBOM or enabled feature set."
    }
  ],
  "tests": [
    "1177 distinct published registry archives matched exact Cargo checksums; no download/checksum failures.",
    "1902 notice hashes and all 3257 review archive entries plus archive digest independently verified.",
    "Six difflib Rust files exactly match later upstream source; fax sources differ and are explicitly not treated as the same artifact.",
    "Read-only evidence slice: no downloaded code execution, application build/test rerun, account action or distribution. Earlier source checks remain separate."
  ],
  "exceptions": [],
  "files": {
    "specs/059-modular-node-v1.md": "cc0aae5376e46457624486b5ea5a624ceed278d86c00f7ce93de10a67f570aa1",
    "successor/V1-ACCEPTANCE.md": "a01db7babf648b9f9b575251948cf4bec90f80f59c388d7f07075fe5d9db6e47",
    "successor/evidence/2026-10-17-node-foundation/codex-third-party-audit.json": "aaa60fff6e3084a287e8f8f925b5a3cf3ad6297bfe18a02c4313b174a1473c22",
    "successor/evidence/2026-10-17-node-foundation/codex-third-party-audit.md": "7ffeaef31443653989c0f447d7189a9cc7416f83efe4fa71317175e3a37ed169"
  }
}
```
