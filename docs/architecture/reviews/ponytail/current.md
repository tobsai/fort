What this repo does: Fort is a single-owner native messenger with a Node host,
a durable ledger and optional runtime modules. This review assumes one owner and
covers the offline Codex 0.163.0 update after Quit repair commit 3aba02fd.

Reviewer: primary implementation agent, using the pinned Ponytail skill and the
preserved whole-repository baseline. Read the adapter/control/version changes,
launch plan, account/thread checks, startup fixtures and public regressions; traced
the executor and host manifests into the Mac package. Reviewed the unchanged ledger
runtime-identity refusal and its new history-preservation regression. Inspected the
upstream protocol/features/preferences sources, exact dependency closure, collected
license delta, original mixed-license notice, legal archive records and readback.
Historical prototypes and donor source remain untouched.

## Fixed

1. **Package provenance could name the wrong source.** The changed binary hash was
   correct but two reports retained the old source SHA. Assertions against the public
   assembled and signed-probe artifacts failed first. Both labels now match the
   reviewed source. Without this correction, recipients could not trace the binary
   to the revision the report claimed.

2. **Compiled fixtures omitted the selected account.** Their catalog call and
   dispatch predated the explicit-account contract. Supplying the existing invented
   account restores the meaningful compiled executor checks. No production account
   guard or provider identity was relaxed.

## Must fix before installed update

3. **Existing history cannot open under the new runtime identity.** The current
   ledger records 0.157.1. The new descriptor intentionally refuses it before service
   startup. A new public regression verifies refusal and exact old-version recovery.
   Design/test an explicit history-preserving binding migration, including queued
   work and rollback, before installing. Do not delete data or rewrite past versions.

4. **Offline results do not establish installed/provider acceptance.** The real
   unauthenticated catalog and empty thread accept exact 6.1 selection, but no turn
   ran. The current disconnected app also cannot reach paired Settings. Live exact
   account/model behavior, paired Quit, physical iPhone and TestFlight remain open.
   Use a fresh bounded scope after the migration and exact signed package are ready.

## Accepted diagnostic limit

5. **Preferences IPC is wider than file isolation.** Upstream refuses configuration
   without managed-preference synchronization. The temporary probe checked absence
   of both forced Codex keys without reading values, then allowed only the required
   preferences services. This is documented diagnostic scope, not a production
   permission change or complete host confinement. Required admin settings are not
   bypassed and no inference or credential access was performed.

Correctness/security: exact version, source and binary pins move together. The
four new opt-outs must be false in both global and thread inventories. Existing
provider routing, telemetry/update, permission, account, MCP, tool and instruction
checks remain intact. The upstream forced execution-backend flag does not grant
shell authority; empty environments and shell_tool=false remain required. No new
runtime module, account route, service or message-time scan was introduced.

Attribution: 1,177 registry records match the exact closure checksums; 1,158 are
unchanged and 19 are newly collected. Preserve original LICENSE/NOTICE, mixed
rmcp MIT/Apache transition terms and 15 covered-source archives. All 3,248 notice
and 19 source archive entries were read back against their manifests. This is
conservative source attribution, not a target binary SBOM or service license grant.

Validation: portable 1,363 pass/16 skips; typecheck passes; compiled executor
package 4 pass/1 benchmark skip; Mac executor/guard 29 pass/2 benchmark skips;
assembly 21 pass; signed offline version/provenance and payload refusal 2 pass.
Fresh helper build and independent 202-file package verification pass. The
installed Quit-fixed 206-file app remains unchanged/disconnected at 04:06 UTC.
Relevant earlier failures and corrected fixtures are retained in the evidence note.

Verdict: Ready as an offline source and review-package checkpoint. Installation
remains blocked on explicit history-preserving migration and fresh live acceptance.
Not checked: Full authenticated 0.163.0 application journey, managed account
entitlement/effective 6.1 inference, paired installed Quit, complete signed XPC
suite for this candidate, performance benchmarks skipped above, physical iPhone,
TestFlight, G2 upload, provider closure or an independent whole-repository audit.

```fort-review
{
  "format": 1,
  "reviewedCommit": "3aba02fd3a694c36ee052639cd0a0275cfbb0799",
  "reviewer": "Codex primary implementation agent",
  "scope": "Offline Codex 0.163.0 adapter/control upgrade, exact attribution and review-only Mac package; preserve installed 0.157.1 ledger and activation boundary",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Public exact 6.1 selection/runtime refusal and four new feature controls observed red then green; malformed/missing controls covered in global and thread scope.",
    "Portable 1363 pass, 16 optional skips; typecheck passes. Compiled executor package 4 pass/1 benchmark skip; compiled Mac executor/guard 29 pass/2 benchmark skips.",
    "Final assembly 21 pass; signed offline Codex version/provenance and wrong/missing payload 2 pass. Both source-SHA assertions failed before correction.",
    "Real unauthenticated network-denied capture: 175 features, empty global/thread MCP, exact 6.1 empty ephemeral thread, no turn/login, exit zero and process/credential absence. Public control replay passes ten calls.",
    "Existing-ledger regression refuses 0.163.0 and reopens original 0.157.1 with exact agent binding, receipt, messages and completed run intact.",
    "Fresh FortHost Release build; review-only 202-file manifest and all legal archive entries independently read back. No installed runtime update.",
    "04:06 UTC installed check matches all 206 Quit-fixed files and confirms process/job/credential/connection absence. Previous native slice has 342 pass/16 skips and Mac/iPhone Simulator builds; not rerun as new-runtime native acceptance.",
    "Spec, staged implementation, exact review and whitespace gates required before this commit. Live account/model, paired installed flow, physical/TestFlight/G2 acceptance unrun."
  ],
  "exceptions": [
    "Primary-agent scoped semantic review, not independent or repeated whole-repository audit; prior baseline retained.",
    "Preferences IPC diagnostic exception follows forced-key absence check; no general host-isolation claim and no production permission change.",
    "Conservative CLI legal inventory includes development/build/other-target packages; not a binary SBOM. Original notices and covered sources preserved.",
    "Review app is unsigned, unprovisioned, uninstalled and unlaunched. An explicit history-preserving migration is required before installing over retained data.",
    "No provider login, inference, credential extraction, activation, pairing, upload, push or merge. Consumed prior grant remains consumed."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Assembly and signed-probe reports retained the old source SHA although the binary hash was new. Public artifact assertions failed first; both reports now identify exact 0.163.0 source.",
      "disposition": "fixed"
    },
    {
      "id": "2",
      "detail": "Compiled executor fixtures lacked the selected-account input required by the existing contract. Supply their invented account through discovery and dispatch; production account checks remain unchanged.",
      "disposition": "fixed"
    },
    {
      "id": "3",
      "detail": "Retained installations bind runtimeIdentity to 0.157.1 and refuse the new version. Preserve refusal and exact old-history recovery; implement and verify explicit binding migration before installation.",
      "disposition": "open"
    },
    {
      "id": "4",
      "detail": "Unauthenticated bundled catalog and empty thread do not verify live ChatGPT entitlement/effective 6.1 inference, paired installed Quit or physical/TestFlight delivery. Fresh exact scope and real acceptance remain required.",
      "disposition": "open"
    },
    {
      "id": "5",
      "detail": "The offline app-server needs cfprefsd synchronization; bounded diagnostic IPC after presence-only absence check is documented separately from production and cannot establish complete agent-host confinement.",
      "disposition": "accepted-exception"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "fe8d0a42b5264746cfa1287fe35b5fde9e4617cc7d2f525a389a3d930aeacb57",
    "successor/APPLE-HOST-ACTIVATION.md": "97d40322be3a82cfb1e06adf75b409d0a51d1d73fb6e8c245aa69236cb920df1",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "444161278200ccde818c4f0907ff2f8201099abf2605105aac1df948a311468d",
    "successor/V1-ACCEPTANCE.md": "e616fbd6734fff5aad7a3fb5b5878dadabf18cc326b344abe4994fae2b9b6c55",
    "successor/evidence/2026-10-17-node-foundation/codex-0163-offline-update.md": "d7aecb8b280f9233d571dee6e5e5692cf5be688df7052168504206ea05688f6b",
    "successor/packaging/codex-executor-artifact.macos.test.ts": "026825f49fe2372bf57268449de7feacad354dc80af5de20e8fce8b289ec1f4a",
    "successor/packaging/codex-executor-artifact.test.ts": "d9432234da325236ef2d935023e47e89dc49e8c70384357beab821fe2f7c3648",
    "successor/packaging/codex-executor-artifact.ts": "c40a41b06ec178b1ed44573d10b0d13382bdcf19b62e4262ef0ddc3dc6e29efc",
    "successor/packaging/fixtures/executor-contract.mjs": "50590f95bb82ce696b916b2f737b261e76ec5f29dfb7eb3535157b76e4a6d9c2",
    "successor/packaging/host-resources.ts": "053f1d8b5d86360cf09ab00c0378962bfeb5157702baa8aa5bf33e3e97755878",
    "successor/packaging/mac-application.test.ts": "0c3d4f41d8f9611fc42274478a3e24fde1a304e65d98efc1558f9c8ef2328959",
    "successor/packaging/mac-application.ts": "ba68b3d69727e26c25b4b9218af30246a3eb7c7484e26d00f6e12d89a830a058",
    "successor/packaging/runtime-fixture.mjs": "0731861d6b0ca0071099358ae952bcc1e2f27115b1c83097429fb5434a45ff78",
    "successor/packaging/sandbox-feasibility.test.ts": "e9f75f0c86d9ba9c70874f9624aff5c6e36ed49de088a581681e12c3440923c0",
    "successor/packaging/sandbox-feasibility.ts": "40d7a11ffd958c0bed83ff61931bada1f230b3125e1e92a2fcf5e84c75666fc3",
    "successor/packaging/vendor/codex-0.163.0/LICENSE": "e2c797ff7ec1a3de9ff898e7f132bce0c64f4e931de3975d73767b2b591f7170",
    "successor/packaging/vendor/codex-0.163.0/NOTICE": "a2cff80d35c4c779d65f17fc2fa820d673759221794cdf6483255e6bc982ba69",
    "successor/packaging/vendor/codex-0.163.0/SOURCE-AVAILABILITY.txt": "fcacacf3ae82a0cebb0239366cee87b7531b09e8a789b6b710e77b4f9e1d671f",
    "successor/packaging/vendor/codex-0.163.0/THIRD-PARTY-NOTICES.zip": "b01171ea7c564ee1560c4e054ae3b1830023eb0217f93e73bf220123056819bf",
    "successor/packaging/vendor/codex-0.163.0/THIRD-PARTY-SOURCES.zip": "dd2817a90dd7a06fea627f3311eb5b5b62fcc00f2d3d197c7d6bcd801cd11be2",
    "successor/packaging/vendor/codex-0.163.0/THIRD-PARTY.json": "8db43684fec10b49fa51d4e9e89a127115fa1b3d7e7d421e9070a521040dd413",
    "successor/runner/PERMISSION-PROFILE.md": "f61d4b947ba3d21944716fdf5dfc08cf97a1e7fea8218b521c3bf07e345edbec",
    "successor/runner/README.md": "b89880721a650e66f2dd7b1f42ec4e7bb84c00416ec71a182da3f2c2e1ad2119",
    "successor/runner/STARTUP-CONTRACT.md": "a0b63fab83ae955079a7ccd224c3af02c932effccd69f8b8f362621b30de1be4",
    "successor/runner/codex-account.test.ts": "655885454c3507314c2c18956500c5dad9c84aa46cc413fa9bf70350f0832c59",
    "successor/runner/codex-account.ts": "297a027d1b8e90f2da8e8c0dc43a070d0a97d4739e7627e7a72ffab5dc884a8d",
    "successor/runner/codex-chat.test.ts": "af0aac67fe266e6314126f692f67cd85369a1e96d527ac3c343fb1336c8855d5",
    "successor/runner/codex-chat.ts": "6b15d4b45fd7fc68dd04daff73b6813f0700df245ce87bd52ba87ef20fa9c980",
    "successor/runner/codex-controls.ts": "2810f8b0490585c0560604d8ad2e9c5cee94bd63dd3796f6d9f16cb9b88da290",
    "successor/runner/codex-descriptor.ts": "7a76497f6be818f38c5289ee0356a51b173316d661cc49aa337dd7d249be6b51",
    "successor/runner/codex-executor.test.ts": "682d8d28119bf873bb5d6e610e423b5705576fd182eca60977541cd36daec81d",
    "successor/runner/codex-executor.ts": "6b3e84aff19d2ac912dbe614022eb06d47c04e9a89265e822ce3b39bc37f2bfc",
    "successor/runner/codex-preflight-fixture.ts": "27b2ff6365db6454f961447c0dddf0377b10f7d14fee4848c5377a3545f5a540",
    "successor/runner/codex-startup-contract.test.ts": "3e7011c202f2bd1b9994e0e4917e8f303114f01e36d83501d1056d3d7983c78d",
    "successor/runner/fixtures/codex-0.163.0-startup.json": "af2256ad106cabaf36d79fbd790dc7282210990335e2dd4ee14b40522132485c",
    "successor/runner/startup-diagnostic.ts": "f53d78c20116db0127b2ae2ae10f43c63a66ce729046232bc6a50f5556e3f853",
    "successor/service/openai.test.ts": "3f5f42db217c9c88ae9f8dbe8bf12c5125b42fa36138961bcf988fd6cfe5fb88"
  }
}
```
