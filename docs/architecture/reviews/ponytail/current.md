What this repo does: Fort is a single-owner native messenger with a durable Node
ledger and optional runtimes. This scoped review assumes a stopped, disconnected
installation, at most 256 MiB ledger/sidecars and 1,000 current Agents.

Read the pinned Ponytail skill and preserved baseline. Traced migration input/file
checks, exact schema and lifecycle guards, SQLite exclusivity, current/historical
binding reads, transaction and logical digest, journal retry/rollback, delete-history
schema acceptance, normal startup refusal, package exports and performance callers.
Read the complete new regression suite and final changes. No unused-code claim or
whole-repository re-audit is made. Donor and prior specs remain unchanged.

## Fixed

1. The migration journal broke explicit history deletion. Its exact optional schema
   is now accepted; a public test deletes history, preserves Agent settings and rotates
   the epoch. A stale rollback then refuses, preserving the newer state.
2. Unfinished owner setup and missing run state were not rejected initially. Tests
   exposed each gap before guards were added. No setup or pending work is silently
   retargeted, completed or canceled.
3. The measurement harness omitted its package’s ES-module setting. Copying the
   existing package descriptor fixes the real compile failure without changing build
   budgets, product compilation, permissions or executing package scripts.

## Must fix before claiming installed acceptance

4. Source tests do not prove an installed upgrade or provider inference. Rehearse on
   the closed real-ledger copy, verify the exact signed candidate, then use the approved
   ordinary sign-in/three-prompt scope and local cleanup. Retain history and backups;
   do not roll a stale snapshot over newer writes or call catalog presence inference.

Correctness/security: normal startup cannot migrate implicitly. Current binding
versions change in one transaction while historical records retain exact bytes.
Old revisions are disabled for future pinned use. Missing or changed operation
identity refuses; a lost reply returns the same result without another revision.
Rollback requires the full post-state digest and restores original rows/schema;
SIGKILL tests bracket COMMIT on both apply and rollback. File checks reject foreign,
nonprivate, aliased and unexpected schemas; exclusive lock fences the ordinary host.
CLI failures emit a generic diagnostic, never request contents or ledger values.
The host process boundary and tool sandbox are unchanged; no new external dependency.

Validation: focused 80 pass/two skips; final portable 1,398 pass/16 skips; strict
typecheck; fresh resources/unsigned assembly. One synthetic maintenance sample is
55.996 ms for 10,002 messages, below five seconds, without provider latency. Read-only
installed inventory and exact identity/counts confirm stopped/disconnected old state.
No real history mutation, installed change, credential read or turn occurred here.

Verdict: Source migration ready for the separately authorized installed verification.
Not checked: Actual migration/install/provider flow, paired installed Quit, physical
Apple devices, TestFlight/G2, managed entitlement/effective-model attestation, full
OS host isolation and repeated production performance. No independent review claim.

```fort-review
{
  "format": 1,
  "reviewedCommit": "3f8ea843f02a1ccdc7c47a18f0b11cfe90311409",
  "reviewer": "Codex primary implementation agent",
  "scope": "Explicit stopped-ledger runtime migration, retry/crash/rollback and deletion compatibility; compiled core export and synthetic build fixture; no installed mutation at this checkpoint",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Focused public migration/history/package: 80 pass, 2 optional skips; four actual SIGKILL COMMIT-boundary cases recover through public CLI.",
    "Full portable final: 1398 pass, 16 optional skips; strict typecheck passes. First run failed measurement temporary module classification; corrected and reverified in final full suite.",
    "Synthetic 10002-message migration: 55.996 ms, one warm sample, M1 Max/arm64 Darwin 27.2.0/Node26.5.0; no provider.",
    "Fresh resources and 204-file unsigned assembly pass; installed 206-file prior app and stopped/disconnected state read-only verified.",
    "Staged spec, review and whitespace gates required before commit. No real migration/install/sign-in/provider turn at this checkpoint."
  ],
  "exceptions": [
    "Primary-agent scoped review using retained whole-repository baseline, not independent review.",
    "Agent configuration remains in a bounded local journal; no messages/credentials logged. Ledger bounded to 256 MiB, 1000 current agents/3000 old bindings.",
    "No automatic rollback after later writes. Preserve current data and use a new reviewed reverse operation.",
    "Complete signed/installed/provider flow, physical iPhone, TestFlight/G2 and full performance percentiles are unrun in this source slice."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "New migration table made explicit history deletion reject the otherwise valid ledger. Accept only its exact reviewed columns; public deletion/recovery-epoch behavior passes.",
      "disposition": "fixed"
    },
    {
      "id": "2",
      "detail": "Unfinished owner setup and malformed run with missing state initially bypassed maintenance preconditions. Reject both with unchanged-data regressions.",
      "disposition": "fixed"
    },
    {
      "id": "3",
      "detail": "Incremental build smoke copied TypeScript files without the package module setting; new import.meta entry could not compile. Preserve package.json in the temporary source; full smoke and portable suite pass.",
      "disposition": "fixed"
    },
    {
      "id": "4",
      "detail": "Actual retained-ledger migration, signed installed update, personal-account exact 6.1 execution and paired Quit remain separate acceptance stages; this source checkpoint cannot establish them.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "57c18545b39041ed1605bb2f0a9489396b215b59c84ec4371d20d8cc6fa8d8d8",
    "successor/service/runtime-migration.ts": "d1035cc137ebc1c2b3b0635656f8c52d66daee8cdd887bb18d2bf27683ac59dd",
    "successor/service/runtime-migration.test.ts": "d722f43b4caa1f11e4e60c1017ef4abcd336ed6481885c6d11b367ff22bc4f73",
    "successor/service/delete-history.ts": "5ad82b4aa9bce65ae1ebb085405b02d323b5f528d55ea2a2ca2e883549525335",
    "successor/packaging/core-artifact.ts": "8b7f06c56b341fb6c54687365291240e49d978ae43c0c0ccd1ae48a7fac6610d",
    "successor/packaging/core-artifact.test.ts": "75a1927408b4ccd7f5d8e5f54eb3cc7f30fc6d7af8200b08bd037c2b431e2bce",
    "successor/tsconfig.core.json": "15f0eddf7738eec423e5ce43653668894e4a117f1aab0a48c29aa1d0d497b251",
    "successor/verification/performance.mjs": "99797fcb34bd7a0297a2bad3852e7d90bdae31b366ba5537283ab3e7f6203630",
    "successor/evidence/2026-10-17-node-foundation/runtime-migration-0163.md": "fd890dddbb34cb88fa6b06a6e095d7097801e30d5e9ff60ebbb86c2fd5b26305",
    "successor/V1-ACCEPTANCE.md": "e3c1d667998775ec7a04b90d2baa1792589113135c3ba96266231ea4c72be573",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "f824b263fe73559722078e9acec487b2e5665995bd78a441d4de68cc9514b0cd",
    "successor/APPLE-HOST-ACTIVATION.md": "90b1d87bbbce8cefb4999d106bacc7d08c7684ca05312801c58d57b15edbc1fa"
  }
}
```
