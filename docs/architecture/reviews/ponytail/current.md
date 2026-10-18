What this repo does: Fort is a single-owner native AI messenger with a local Node
core and optional runtime adapters. This slice composes an inert Mac review bundle
from explicit local build inputs. Expected load is one owner assembling a package.

## Semantic review

Codex self-review, October 18, 2026, base `e352ffa`. Used the pinned
[Ponytail skill](https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md)
and preserved baseline. Read the new CLI/test, test discovery, changed native test,
MacHostBundle/Program/AgentService, source app config, host-resource and executor
packagers, launch paths and recorded sandbox evidence. Traced source reads,
hash/identity checks, copies, native metadata subprocesses, destination cleanup,
file/mode manifest and native signature/profile refusal. No independent attestation.

## Must fix

1. **Production launch omitted the proven OpenSSL configuration — open.**
   - What this is: The sandbox probe supplies a fixed bundled Node configuration.
   - Problem: Current core and executor-worker launch paths omit it; the earlier
     sandbox run already recorded a denied global Homebrew configuration read.
   - Fix: Carry the same bounded configuration through both artifacts and launch
     plans, then test synthetic runtime behavior before installed acceptance.
   - If skipped: The final helper can retain an unwanted machine configuration
     prerequisite. Owner: implementation agent; next local slice, no release exception.

2. **Client builds lacked a composed host — fixed as review preparation.**
   - What this is: The ordinary client build did not contain the host/runtime layout.
   - Problem: A build pass alone could not establish even the package composition.
   - Fix: Explicit CLI checks hashes/identity, retains attribution and produces a
     review-only artifact. Fifteen tests and read-only native refusal/layout checks pass.
   - If skipped: Signing/provisioning review has no concrete package to assess.
     This still does not establish usable installed operation.

## Should fix

3. **Codex third-party notice closure remains incomplete — open.**
   - What this is: Exact root LICENSE/NOTICE are present in the composed artifact.
   - Problem: They do not enumerate all statically linked dependency obligations.
   - Fix: Finish the pinned source/binary inventory before distribution.
   - If skipped: A shipping package could omit attribution. Owner: implementation agent;
     Apache permission itself does not require a generic new owner approval.

Verdict: Inert package preparation is verified; fix the two open runtime/notice
issues and continue deletion/installed-product work. No release acceptance.
Not checked: product process launch, actual signatures/profiles/App Groups, provider
or account access, startup activation, native UI/accessibility/screenshots, iPhone
transport, full history erasure, TestFlight, remote CI and branch protection.
No hook/dependency was added. Native/service behavior is unchanged by this slice;
prior 240 native and 1104 portable passes remain separate evidence. No speculative
framework or unused-code reduction is claimed. The deliberate unsigned refusal is
an acceptance check, not a defect to bypass.


```fort-review
{
  "format": 1,
  "reviewedCommit": "e352ffa4934dbd7973ebc99429b7305296869d93",
  "reviewer": "Codex",
  "scope": "Inert Mac assembly CLI, changed native inspection test and test discovery, source artifact builders and launch/configuration boundaries. Self-review; baseline preserved.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "findings": [
    {
      "id": "OPENSSL-BOOTSTRAP",
      "disposition": "open",
      "detail": "Both production Node launches omit the fixed configuration already required by the sandbox probe; implement in the next local slice before runtime acceptance."
    },
    {
      "id": "MAC-PACKAGE-COMPOSITION",
      "disposition": "fixed",
      "detail": "Explicit local-only assembly produces a hashed unsigned review bundle with source preservation and native refusal checks."
    },
    {
      "id": "CODEX-NOTICE-CLOSURE",
      "disposition": "open",
      "detail": "Pinned root LICENSE/NOTICE retained; complete statically linked third-party inventory remains before distribution."
    }
  ],
  "tests": [
    "15 packaging functions passed with explicitly selected local Node/Codex bytes; neither runtime invoked.",
    "One test-discovery function and strict TypeScript passed.",
    "Unsigned Release client/helper and guardian builds passed.",
    "One read-only native layout/signature/profile refusal function passed.",
    "171 output hashes/modes and 169 input hashes verified; three warm assemblies p95 1880.101542 ms.",
    "Installed runtime/UI, account/signing/provisioning, provider, TestFlight and remote CI unrun."
  ],
  "exceptions": [],
  "files": {
    "specs/059-modular-node-v1.md": "6803099f70fa97202cd5a5aa150c72912176f103bd573fd51e29cdf151ec7483",
    "successor/V1-ACCEPTANCE.md": "a39aabe0c7d9f302e15260966ec1b20c52150a7c113e8bffde391adb6061b9e9",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostBundleTests.swift": "6d804f78335d10db0a11ef9dadd6475e224849194467a0f2d52f2f3262c8d7a1",
    "successor/evidence/2026-10-17-node-foundation/mac-review-package.md": "ea044830d1bf731d35a0311aa9de7a4b5cbb268a9820c2bd429019fe4bbec875",
    "successor/packaging/mac-application.test.ts": "e9b7598fe0794cf91e8427ec093f149117a644e0d25f453b54b439db7ee0f67d",
    "successor/packaging/mac-application.ts": "ea7614a749d8e1dab31e6e622fea830c34c60be7ac3c40a425b65c934a9d3a43",
    "successor/verification/test-suite.mjs": "df04f4e0a878a5db50dffcb3f5dd8c792f1d0f699efc60a8cb67b2396f4a799a",
    "successor/verification/test-suite.test.mjs": "529321c3abaa91a5693e09c5d50c04810bd586f7cc173d3881a7297f8ae1845d"
  }
}
```
