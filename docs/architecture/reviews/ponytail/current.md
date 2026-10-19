What this repo does: Fort is a single-owner native AI messenger with a Node core
and optional official runtimes. This review covers release evidence for the existing
one-owner design, not new behavior or shared hosting.

Reviewer: Codex primary implementation agent, October 19, 2026. Base
`ce3f2936a645a9bc528caf6bec98dd7f33a73a10`. Used the pinned Ponytail skill and
preserved whole-tree baseline. This is not an independent-reviewer claim.

Reviewed all eight changed evidence/spec/draft files and the actual 31-object
inventory. Traced native file imports, private cache/setup/removal, App Group and
bundle metadata checks, Mac shutdown timers and runtime configuration validation.
Compared the focused test results, package hashes, current Apple page, CSR public
metadata and preserved signing identities. The revised spec adds only declared
evidence paths and reuses the existing release-preparation design authority.

## Must fix before installed or release acceptance

1. **A signed package still lacks installed proof.**
   - What this is: The existing Mac artifact has valid development signatures.
   - Problem: It has not run; its helper lacks outgoing-network entitlement.
   - Fix: Resolve the concrete installed/network/provider scope, then test actual
     App Group/XPC, UI, lifecycle and provider behavior on the exact artifact.
   - If we skip it: Signing could be mistaken for working or delivered Fort.
   - Disposition: Open; no new execution grant inferred from this evidence.

2. **Runtime privacy declarations cannot yet be chosen from this inventory.**
   - What this is: Thirty-one binary objects and their native callers were inspected.
   - Problem: Runtime file/disk/clock imports and separate preferences signals have
     unresolved reachable uses; no artifact contains a privacy manifest.
   - Fix: Resolve exact SDK uses/platform responsibilities, add supported manifest
     entries and inspect the archive report alongside actual provider data flows.
   - If we skip it: Empty declarations or broad privacy claims would lack evidence.
   - Disposition: Open. The completed static inventory narrows the remaining audit;
     407 synthetic control passes do not prove provider retention or compliance.

## Blocked external continuation

3. **The G2 request is local and its upload is unapproved by the execution gate.**
   - What this is: Apple Assistant generated one local RSA-2048 key/public CSR.
   - Problem: Automatic approval review rejected sending its public key/identity
     metadata to Apple because the specific disclosure authorization was unverified.
   - Fix: Await the precise supported approval, reuse the CSR/key, then verify the
     issued leaf's G2 issuer, team, key match, expiry and paired signing identity.
   - If we skip it: A duplicate key, unauthorized upload or false completion claim
     could result. No G2 leaf has been issued, installed or used.
   - Disposition: Open. Picker canceled; portal confirms no selected file. Existing
     four signing identities remain. No private-key export or certificate revocation.

Verdict: Static inventory and local-CSR evidence are accurate and reviewable.
Installed acceptance, complete privacy decisions and the blocked G2 upload remain
open. The signed package and product source are unchanged.

Not checked: All reachable upstream runtime/dependency privacy paths, actual
provider retention or traffic, installed App Group/XPC/provider/UI, physical iPhone,
archive privacy report, distribution upload/processing/tester access or remote CI.
One optional timing test was skipped; existing complete suites/builds were not
repeated. No privacy manifest or compliance answer was fabricated.

```fort-review
{
  "format": 1,
  "reviewedCommit": "ce3f2936a645a9bc528caf6bec98dd7f33a73a10",
  "reviewer": "Codex primary implementation agent",
  "scope": "Static native/runtime privacy inventory, focused control verification, separate approved local G2 CSR and unperformed-upload evidence, release handoffs; no product source changes",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "All 31 actual Mach-O objects enumerated, hashed and read with nm -u; all exited zero. Independent second hash/size/source reconciliation passed; zero privacy manifests in both artifacts. Exact C symbol matching replaces false-positive substring scratch results.",
    "407 existing private-launch/startup-control synthetic tests passed, zero failed, one optional timing-budget test skipped. No installed app or real provider runtime executed.",
    "Apple Assistant local RSA-2048 CSR signature verified; public CSR/SPKI hashes recorded. Four existing signing identities remain. Portal G2 selection/no-file/disabled-Continue confirmed after canceled picker. No G2 leaf created or installed.",
    "Full spec and accepted implementation gates passed; exact staged spec/review and diff checks required before commit. Full runtime suites/builds not repeated for unchanged product source."
  ],
  "exceptions": [
    "Review is scoped implementation-agent semantic review, not independent reviewer signoff. Prior whole-tree baseline is preserved. No SDK-wide privacy audit, uploaded archive report or legal/compliance determination claimed.",
    "Automatic approval review rejected public CSR upload for unverified disclosure authorization. No workaround/retry; source-thread lookup returned no turns, precise user input is pending. A local key/CSR exists, not a paired G2 certificate.",
    "The auxiliary AppleScript lookup triggered a new automation permission prompt; the lookup was stopped and permission declined. No additional automation permission granted, private key exported, trust changed, certificate revoked or Xcode component installed.",
    "Signed development app remains unchanged and unlaunched. No provider, startup, app installation, TestFlight or publication operation occurred."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "The signed app is unlaunched and the helper has no outgoing-network entitlement. Effective App Group/XPC, installed UI/provider behavior and TestFlight acceptance remain open under their separate action-specific gates.",
      "disposition": "open"
    },
    {
      "id": "2",
      "detail": "The completed static inventory finds file/disk/clock imports and no privacy manifests. Exact runtime uses, native platform/manifest responsibilities, provider retention and final privacy/encryption/contact/URL declarations remain open; symbol presence is insufficient to choose reasons.",
      "disposition": "open"
    },
    {
      "id": "3",
      "detail": "The local G2 key/CSR is created, but upload was rejected by automatic approval review for unverified disclosure authorization. Canceling the picker and recording a precise pending approval prevents a false completion claim or duplicate key/certificate.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "b332be6b299ccc9a3bd7df50f5f5496d9f63fe41cb1b87362fd7ed40bcdeec3d",
    "successor/APPLE-HOST-ACTIVATION.md": "ada5f7a2af283469021e217580121fecbb9a3015a916e2183c727439d2b3d29a",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "d628c135a7035177efd04968ddcfc108dd19d80a6409a3a9c440481dcf222ab7",
    "successor/V1-ACCEPTANCE.md": "c74a20b753614d5c41ac2f7cabeec8329a30bb3709bab78007d868040f995ea5",
    "successor/release/SUBMISSION-DRAFT.md": "1e51bf2f6eeda7891c6de6ce539773767f6155e069a57a40b7a9c0dc71836624",
    "successor/evidence/2026-10-17-node-foundation/privacy-runtime-inventory.md": "d46df362a93ae86b292d5570e5f25e3931f1d8eb552d863c95d6396232b5faf8",
    "successor/evidence/2026-10-17-node-foundation/privacy-runtime-inventory.json": "ca0caf47661065556e5629e95452694c9d3a59e260e69e0eaf66d3c9162d7eef",
    "successor/evidence/2026-10-17-node-foundation/g2-certificate.md": "0816529c0c6be0c7bbf327b414b2c7059fe2b71598a95b81b91b94289677bb33"
  }
}
```
