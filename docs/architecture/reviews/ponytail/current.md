What this repo does: Fort is a single-owner native AI messenger with a local Node
core and optional runtime adapters. This slice lets the owner explicitly prepare
setup again after completed removal, keeping saved conversations.

## Semantic review

Reviewer: Codex, October 18, 2026. Reviewed base:
`dcdc013789333bc01f5af14a6231ee671f50726c`. This is a self-review using the immutable
[original Ponytail skill](https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md)
and its recorded hash. The whole-tree baseline remains unchanged.

I reviewed all changed source, tests and spec/evidence paths, native host-controller
construction, exact removal proof, owner-file lease/sync behavior, credential lookup,
setup eligibility and UI entry. The load is one owner. The new action neither
registers startup nor pairs. It verifies recorded cleanup and current absence before
clearing one completed setup block. The old revoked identity and history stay intact.
Inspection reads no credential; only the explicit action checks that exact entry.

## Must fix

1. **Completed removal blocked later setup — fixed in source.**
   - What this is: A completed host-removal record deliberately excludes new setup.
   - Problem: The record persisted after successful cleanup, so reinstall with retained
     owner files had no ordinary path back to setup. Public/model red tests reproduced it.
   - Fix: Add an explicit action that verifies the exact bundle, confirmed records,
     startup/helper absence and credential cleanup before durably removing that block.
   - If we skip it: Retained-history reinstalls can stay unusable. The source, refusal,
     retry and unsigned-build checks now pass; installed acceptance remains open.

2. **History deletion scope remains undecided — open owner decision.**
   - What this is: The production removal path currently preserves all history.
   - Problem: Deleting client caches, host ledger data or both has materially different
     effects; the requested deletion capability has no settled exact scope here.
   - Fix: Resolve the pending scope, then implement and verify that bounded optional
     action. Keep deletion unavailable until then.
   - If we skip it: V1 cannot claim its optional deletion behavior. This is not a release
     exception, and no actual history deletion was authorized or performed by this slice.

The final action refuses requested/recovered/corrupt or changed progress, live helper,
re-enabled startup, pending/active connections, foreign identity, unconfirmed local
cleanup, missing credential access and changed bundle. It rechecks records after
waits and checks cancellation before unlink. Repeated successful reconciliation is
inert. Native wording reports recorded completion without asserting the current
startup state, and only announces setup readiness if final metadata inspection passes.
No further defect was found in these changed paths.

## Tests and limits

- Four focused functions passed, including 11 refusal variants, three native model
  cases, successful retry and cancellation/retry. Initial public/model reds are saved.
- Full native behavior suite: 236 functions passed; budget, retained-signature and
  complete-host journey functions were excluded from that parallel run.
- Twenty isolated synthetic reconciliations: p95 2.111875 ms against 500 ms.
- Strict TypeScript and fresh unsigned Mac/iPhone Simulator Debug builds passed.
- Core/executor source is unchanged from the preceding slice's 1,104 portable passes
  and complete compiled-host journey in 1,668.438 ms. Those were not rerun here.
- Foundation source remains identical to its quarantined fork checkpoint. The original
  checkout and unrelated dirty files remain intact; no remote mutation occurred.
- [Evidence](../../../../successor/evidence/2026-10-17-node-foundation/owner-reinstall.md)
  contains exact logs, all timing samples, artifact hashes, failure history and limits.
- Full/accepted spec checks passed; exact staged spec/review coverage is required before
  commit. Metadata coverage does not perform or authenticate this semantic review.

Verdict: The reviewed local lifecycle gaps are resolved in source and fixtures.
History-deletion scope and runtime/package, Apple/provider and installed-product
acceptance still prevent v1 signoff.

Not checked: Real credentials/Keychain, persistent startup, official provider/account
execution, signing/profile provisioning, installed accessibility/screenshots, physical
iPhone or TestFlight. No authenticated Apple inspection or desktop UI automation tool
is exposed here. Remote CI, unchanged signature/sandbox and Mac adapter checks were
not rerun. Historical/quarantined source was preserved, not re-audited.

## Snapshot evidence

This binds the reviewed staged Git contents and modes, not external authorization.

```fort-review
{
  "format": 1,
  "reviewedCommit": "dcdc013789333bc01f5af14a6231ee671f50726c",
  "reviewer": "Codex",
  "scope": "All nine changed content paths for retained-history reinstall reconciliation; native controller, removal records, lease, credential lookup, setup guards and UI callers reviewed.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "findings": [
    {
      "id": "REINSTALL-REMOVAL-RECORD",
      "disposition": "fixed",
      "detail": "Explicit native action revalidates exact completed removal and absence before clearing only the setup block. Source, fixture and unsigned build proof passed; installed acceptance remains open."
    },
    {
      "id": "HISTORY-DELETION-SCOPE",
      "disposition": "open",
      "detail": "Exact client/host deletion scope remains an owner decision. Deletion stays unavailable; no release exception or real deletion authority is claimed."
    }
  ],
  "tests": [
    "Four focused functions passed, including 11 refusal variants and three model cases.",
    "236 native behavior functions passed; budget, signature and complete journey checks excluded.",
    "20 isolated synthetic reconciliations p95 2.111875 ms against 500 ms.",
    "Strict TypeScript and fresh unsigned Mac/iPhone Simulator builds passed.",
    "Unchanged core/executor retains preceding 1104 portable passes and 1668.438 ms compiled-host journey; not rerun in this slice.",
    "Foundation checkpoint comparison and primary-checkout preservation verified."
  ],
  "exceptions": [
    "History deletion remains disabled pending exact scope; no v1 exception.",
    "Runtime/package legal closure, Apple account/profiles, activation, provider, Keychain, installed UI, physical devices and TestFlight remain unverified.",
    "Remote CI, unchanged signature/sandbox and Mac adapter checks not rerun; historical/quarantined source preserved rather than re-audited."
  ],
  "files": {
    "specs/059-modular-node-v1.md": "df252b5e9d712dd70a1b322ad188820e5977194b1fa62dbc231dc0f7de590ed9",
    "successor/V1-ACCEPTANCE.md": "8e1fab2736b7054a0264573cb3bf160e635f3ca70aa76cf7784917ca09e1bdc3",
    "successor/apple/FortKit/Sources/FortKit/MacHostRemoval.swift": "26371a49a9454435241cbff4c8073d80f299f8c15c0759d197d777e6e67d0a08",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "5d4e1f1e919b6cace067def0feae93fa64b535f12173ec61a08d19b6858b010d",
    "successor/apple/FortKit/Sources/FortKit/OwnerSetupFiles.swift": "ed92b0a206aff498c9d7db92f7a2cbad00db751148d470032557afb480da1e1c",
    "successor/apple/FortKit/Sources/FortKit/OwnerSetupView.swift": "0b2f2c390c5970d27442d59cec9238c9951f4a5be180a3defd4910c4449b67b0",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostRemovalTests.swift": "47cafd20e49493ca5ae117bd844b605950259cfd5ae3f33ce7fc9c23eab175d1",
    "successor/apple/FortKit/Tests/FortKitTests/OwnerSetupModelTests.swift": "4e2fe88ef90158d25208e5940b8eeb85734517feba2e7357cfca35a9af697c96",
    "successor/evidence/2026-10-17-node-foundation/owner-reinstall.md": "e31ce2f5cf32097be243b803ce31dcfc615fa9c9fb6db71f0c9d434fa3beac8e"
  }
}
```
